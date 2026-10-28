package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"aur_auto/internal/auth"
)

// authStatus 返回认证开关及当前 Cookie 的登录状态，不泄露任何配置。
func (h *Handler) authStatus(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]bool{
		"enabled": h.auth.Enabled(), "authenticated": h.auth.Authenticated(request),
	})
}

// login 校验页面密码并写入签名会话 Cookie。
func (h *Handler) login(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, errors.New("登录请求格式无效"))
		return
	}
	token, expires, err := h.auth.Login(clientIP(request), input.Password)
	if errors.Is(err, auth.ErrTooManyAttempts) {
		writeError(writer, http.StatusTooManyRequests, err)
		return
	}
	if err != nil {
		writeError(writer, http.StatusUnauthorized, auth.ErrInvalidCredentials)
		return
	}
	h.auth.WriteCookie(writer, token, expires)
	writeJSON(writer, http.StatusOK, map[string]bool{"authenticated": true})
}

// logout 清除当前浏览器的会话 Cookie。
func (h *Handler) logout(writer http.ResponseWriter, _ *http.Request) {
	h.auth.ClearCookie(writer)
	writeJSON(writer, http.StatusOK, map[string]bool{"authenticated": false})
}

// requireAuth 保护任务、日志、健康和发布接口，未登录时返回统一 401。
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		public := request.Method == http.MethodOptions || request.URL.Path == "/api/auth/status" || request.URL.Path == "/api/auth/login" || request.URL.Path == "/api/openapi.yaml" || request.URL.Path == "/api/docs"
		if !public && !h.auth.Authenticated(request) {
			writeError(writer, http.StatusUnauthorized, errors.New("请先登录"))
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// clientIP 使用直接对端地址限流，避免伪造 X-Forwarded-For 绕过限制。
func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		return request.RemoteAddr
	}
	return host
}
