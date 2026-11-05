package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"rendering-cms-platform/backend/internal/integrations/postbridge"
)

func registerPostBridgeAttempts(router chi.Router, service *postbridge.Service) {
	router.Get("/attempts", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, err := postBridgePageNumber(query.Get("page"), 1)
		if err != nil {
			writeAttemptError(w, err)
			return
		}
		size, err := postBridgePageNumber(query.Get("pageSize"), 20)
		if err != nil {
			writeAttemptError(w, err)
			return
		}
		result, err := service.List(r.Context(), query.Get("articleId"), query.Get("status"), page, size)
		if err != nil {
			writeAttemptError(w, err)
			return
		}
		writePostBridgeJSON(w, http.StatusOK, result)
	})
	router.Get("/attempts/{attemptId}", postBridgeAttemptAction(service.Get))
	router.Post("/attempts/{attemptId}/refresh", postBridgeAttemptAction(service.Refresh))
	router.Post("/attempts/{attemptId}/submit", postBridgeAttemptAction(service.Submit))
	router.Post("/attempts/{attemptId}/cancel", postBridgeAttemptAction(service.Cancel))
}

func postBridgePrepareHandler(service *postbridge.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input postbridge.CreateAttemptInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
			writeAttemptError(w, postbridge.ErrAttemptInput)
			return
		}
		attempt, err := service.Create(r.Context(), chi.URLParam(r, "articleId"), input)
		if err != nil {
			writeAttemptError(w, err)
			return
		}
		writePostBridgeJSON(w, http.StatusAccepted, attempt)
	}
}

func postBridgeAttemptAction(action func(context.Context, string) (postbridge.Attempt, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		attempt, err := action(r.Context(), chi.URLParam(r, "attemptId"))
		if err != nil {
			writeAttemptError(w, err)
			return
		}
		writePostBridgeJSON(w, http.StatusOK, attempt)
	}
}

func postBridgePageNumber(raw string, defaultValue int32) (int32, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 1 {
		return 0, postbridge.ErrAttemptInput
	}
	return int32(value), nil
}

func writeAttemptError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postbridge.ErrStoreNotFound):
		writeHTTPError(w, http.StatusNotFound, "文章或分发记录不存在")
	case errors.Is(err, postbridge.ErrStoreBusy):
		writeHTTPError(w, http.StatusConflict, "操作正在处理中，请刷新")
	case errors.Is(err, postbridge.ErrAttemptConflict):
		writeHTTPError(w, http.StatusConflict, "文章状态、修订或请求已变化，请重新准备")
	case errors.Is(err, postbridge.ErrCannotSubmit):
		writeHTTPError(w, http.StatusConflict, "平台稿尚未确认或授权已失效")
	case errors.Is(err, postbridge.ErrCoverRequiresChoice):
		writeHTTPError(w, http.StatusUnprocessableEntity, "请确认只发送文字，原文章封面会保留")
	case errors.Is(err, postbridge.ErrUnsupportedContent), errors.Is(err, postbridge.ErrInvalidRequest):
		writeHTTPError(w, http.StatusUnprocessableEntity, "文章内容不符合 PostBridge 的文字 Markdown 限制")
	case errors.Is(err, postbridge.ErrRequestTooLarge):
		writeHTTPError(w, http.StatusRequestEntityTooLarge, "文章快照超过 PostBridge 请求限制")
	case errors.Is(err, postbridge.ErrAttemptInput):
		writeHTTPError(w, http.StatusBadRequest, "分发请求格式或目标无效")
	default:
		var upstream *postbridge.UpstreamError
		if errors.As(err, &upstream) || errors.Is(err, postbridge.ErrNotConfigured) || errors.Is(err, postbridge.ErrUnavailable) {
			writePostBridgeError(w, err)
			return
		}
		writeHTTPError(w, http.StatusInternalServerError, "分发记录操作失败")
	}
}
