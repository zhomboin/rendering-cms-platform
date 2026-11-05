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

type postBridgeConnection interface {
	SigningConfigured() bool
	Register(context.Context, string, string, postbridge.RegistrationGrant) (postbridge.Registration, error)
	Activate(context.Context) error
	Targets(context.Context) ([]postbridge.Target, error)
}

func PostBridgeRoutes(client postBridgeConnection, grant postbridge.RegistrationGrant, service *postbridge.Service) RouteRegistrar {
	return func(router chi.Router) {
		attemptsEnabled := service != nil && client.SigningConfigured()
		if attemptsEnabled {
			router.With(requirePostBridgeAdmin).Post("/articles/{articleId}/postbridge/preparations", postBridgePrepareHandler(service))
		}
		router.Route("/postbridge", func(bridge chi.Router) {
			bridge.Use(requirePostBridgeAdmin)
			if attemptsEnabled {
				registerPostBridgeAttempts(bridge, service)
			}
			bridge.Get("/connection", func(w http.ResponseWriter, r *http.Request) {
				result := map[string]any{"enabled": true, "signingConfigured": client.SigningConfigured()}
				if service != nil {
					registration, err := service.Registration(r.Context())
					if err == nil {
						result["registrationId"], result["registrationStatus"] = registration.ID, registration.Status
					} else if !errors.Is(err, postbridge.ErrStoreNotFound) {
						writeAttemptError(w, err)
						return
					}
				}
				writePostBridgeJSON(w, http.StatusOK, result)
			})
			bridge.Post("/registrations", func(w http.ResponseWriter, r *http.Request) {
				var input struct {
					InvitationToken string `json:"invitationToken"`
					Name            string `json:"name"`
				}
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&input); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
					writeHTTPError(w, http.StatusBadRequest, "注册请求格式无效")
					return
				}
				result, err := client.Register(r.Context(), input.InvitationToken, input.Name, grant)
				if err != nil {
					writePostBridgeError(w, err)
					return
				}
				recorded := service != nil && service.SaveRegistration(r.Context(), result) == nil
				writePostBridgeJSON(w, http.StatusCreated, map[string]any{
					"registrationId": result.ID, "status": result.Status, "recorded": recorded,
				})
			})
			bridge.Post("/activate", func(w http.ResponseWriter, r *http.Request) {
				if err := client.Activate(r.Context()); err != nil {
					writePostBridgeError(w, err)
					return
				}
				targets, err := client.Targets(r.Context())
				result := struct {
					Status          string               `json:"status"`
					TargetsVerified bool                 `json:"targetsVerified"`
					Items           *[]postbridge.Target `json:"items,omitempty"`
				}{Status: "active", TargetsVerified: err == nil}
				if err == nil {
					result.Items = &targets
				}
				writePostBridgeJSON(w, http.StatusOK, result)
			})
			bridge.Get("/targets", func(w http.ResponseWriter, r *http.Request) {
				targets, err := client.Targets(r.Context())
				if err != nil {
					writePostBridgeError(w, err)
					return
				}
				writePostBridgeJSON(w, http.StatusOK, map[string][]postbridge.Target{"items": targets})
			})
		})
	}
}

func requirePostBridgeAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		user, ok := UserFromContext(r.Context())
		if !ok || user.Role != "admin" {
			writeHTTPError(w, http.StatusForbidden, "仅管理员可以操作 PostBridge 接入")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writePostBridgeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writePostBridgeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postbridge.ErrNotConfigured):
		writeHTTPError(w, http.StatusServiceUnavailable, "PostBridge 签名凭据尚未配置")
		return
	case errors.Is(err, postbridge.ErrInvalidRegistration):
		writeHTTPError(w, http.StatusBadRequest, "请检查邀请、应用名称和服务端注册配置")
		return
	}
	var upstream *postbridge.UpstreamError
	if errors.As(err, &upstream) {
		switch upstream.Status {
		case http.StatusTooManyRequests:
			if seconds, err := strconv.ParseUint(upstream.RetryAfter, 10, 32); err == nil {
				w.Header().Set("Retry-After", strconv.FormatUint(seconds, 10))
			}
			writeHTTPError(w, http.StatusTooManyRequests, "PostBridge 请求过于频繁")
			return
		case http.StatusUnauthorized, http.StatusForbidden:
			writeHTTPError(w, http.StatusBadGateway, "PostBridge 拒绝请求，请检查邀请或应用授权")
			return
		case http.StatusServiceUnavailable:
			writeHTTPError(w, http.StatusServiceUnavailable, "PostBridge 接入暂不可用")
			return
		}
	}
	writeHTTPError(w, http.StatusBadGateway, "PostBridge 请求失败")
}
