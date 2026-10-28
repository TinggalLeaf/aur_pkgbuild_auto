package api

import (
	"errors"
	"net/http"
)

// security 校验写请求 Origin，并添加防止嵌入、嗅探和 referrer 泄露的响应头。
func (h *Handler) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		if request.Method != http.MethodGet && request.Method != http.MethodOptions {
			origin := request.Header.Get("Origin")
			if origin != "" && origin != h.origin {
				writeError(writer, http.StatusForbidden, errors.New("请求来源不受信任"))
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
