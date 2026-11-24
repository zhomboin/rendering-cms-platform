package articlemcp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func reportedClient(req mcp.Request) string {
	var client *mcp.Implementation
	if init, ok := req.GetParams().(*mcp.InitializeParams); ok {
		client = init.ClientInfo
	} else if identified, ok := req.(interface{ ClientInfo() *mcp.Implementation }); ok {
		client = identified.ClientInfo()
	}
	if client == nil {
		return ""
	}
	name := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, client.Name)
	runes := []rune(strings.TrimSpace(name))
	return string(runes[:min(100, len(runes))])
}

func (s *Server) observe(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "initialize" && method != "tools/list" && method != "tools/call" {
			return next(ctx, method, req)
		}
		extra := req.GetExtra()
		if extra == nil || extra.TokenInfo == nil {
			return nil, errDenied
		}
		id, err := parseID(extra.TokenInfo.UserID)
		if err != nil {
			return nil, errDenied
		}
		q := dbgen.New(s.store.pool)
		if err := q.ObserveMCPClient(ctx, dbgen.ObserveMCPClientParams{ID: id, ClientName: reportedClient(req)}); err != nil {
			return nil, errors.New("无法记录客户端活动")
		}
		if method != "tools/call" {
			return next(ctx, method, req)
		}
		params, ok := req.GetParams().(*mcp.CallToolParamsRaw)
		if !ok || params == nil {
			return nil, errInput
		}
		name := params.Name
		switch name {
		case createTool, readTool, editTool, publishTool, statusTool:
		default:
			if !slices.Contains(albumTools, name) {
				name = "unknown"
			}
		}
		entry, err := q.StartMCPOperation(ctx, dbgen.StartMCPOperationParams{ConnectionID: id, Operation: name})
		if err != nil {
			return nil, errors.New("无法记录本次操作")
		}
		result, callErr := next(ctx, method, req)
		finish := dbgen.FinishMCPOperationParams{ID: entry, Outcome: "failed"}
		if res, ok := result.(*mcp.CallToolResult); ok && res != nil && !res.IsError && callErr == nil {
			finish.Outcome = "success"
			raw := params.Arguments
			if name == createTool || name == createAlbumTool {
				raw, _ = res.StructuredContent.(json.RawMessage)
			}
			var target struct {
				ArticleID string `json:"articleId"`
				AlbumID   string `json:"albumId"`
			}
			if json.Unmarshal(raw, &target) == nil {
				if slices.Contains(albumTools, name) {
					finish.AlbumID, _ = parseID(target.AlbumID)
				} else {
					finish.ArticleID, _ = parseID(target.ArticleID)
				}
			}
		}
		finishOperation(ctx, q, finish)
		return result, callErr
	}
}

// 客户端断开后仍在独立期限内写回审计结果。
func finishOperation(ctx context.Context, q *dbgen.Queries, finish dbgen.FinishMCPOperationParams) {
	final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := q.FinishMCPOperation(final, finish); err != nil {
		log.Printf("MCP 操作 %d 结果待核实", finish.ID)
	}
}

type operationItem struct {
	ID         string      `json:"id"`
	Connection string      `json:"connection"`
	Operation  string      `json:"operation"`
	Outcome    string      `json:"outcome"`
	ArticleID  pgtype.UUID `json:"articleId"`
	AlbumID    pgtype.UUID `json:"albumId"`
	CreatedAt  time.Time   `json:"createdAt"`
}

func (s *Server) operations(w http.ResponseWriter, r *http.Request) {
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		var err error
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			http.Error(w, "分页位置无效", 400)
			return
		}
	}
	rows, err := dbgen.New(s.store.pool).ListMCPOperations(r.Context(), dbgen.ListMCPOperationsParams{OwnerID: owner(r), BeforeID: before})
	if err != nil {
		http.Error(w, "读取操作记录失败", 500)
		return
	}
	page := rows[:min(50, len(rows))]
	items := make([]operationItem, 0, len(page))
	for _, row := range page {
		items = append(items, operationItem{ID: strconv.FormatInt(row.ID, 10), Connection: row.Label, Operation: row.Operation, Outcome: row.Outcome, ArticleID: row.ArticleID, AlbumID: row.AlbumID, CreatedAt: row.CreatedAt.Time})
	}
	var next string
	if len(rows) > 50 {
		next = items[len(items)-1].ID
	}
	respond(w, 200, map[string]any{"items": items, "next": next})
}
