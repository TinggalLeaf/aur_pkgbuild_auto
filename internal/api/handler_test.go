package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/store"
	"aur_auto/internal/worker"
)

// TestAuthenticationFlow 验证公开状态、未授权拒绝、登录 Cookie、授权访问和退出。
func TestAuthenticationFlow(t *testing.T) {
	storage, _ := store.New(t.TempDir())
	manager := worker.New(storage, nil, nil, config.AURConfig{}, 1)
	handler := New(storage, manager, &config.Config{
		Server: config.ServerConfig{FrontendOrigin: "http://localhost:3000"},
		Auth:   config.AuthConfig{Password: "api-test-password", SessionSecret: "api-test-secret", SessionTTL: time.Hour},
	})
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/tasks", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登录响应为 %d", unauthorized.Code)
	}
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"api-test-password"}`))
	loginRequest.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("登录失败: %d %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK || authorized.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("授权请求失败: %d", authorized.Code)
	}
	logout := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", bytes.NewReader(nil))
	logoutRequest.AddCookie(cookie)
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusOK || logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("退出登录未清理 Cookie")
	}
}

// TestOriginAndAgentLog 验证不可信 Origin 被拦截且 Agent 日志可下载。
func TestOriginAndAgentLog(t *testing.T) {
	storage, _ := store.New(t.TempDir())
	task, _ := storage.Create("https://github.com/owner/project", "project", false)
	_ = storage.AppendLog(task.ID, "info", "日志下载测试")
	manager := worker.New(storage, nil, nil, config.AURConfig{}, 1)
	handler := New(storage, manager, &config.Config{
		Server: config.ServerConfig{FrontendOrigin: "http://localhost:3000"},
		Auth:   config.AuthConfig{Disabled: true, SessionTTL: time.Hour},
	})
	forbidden := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader("{}"))
	request.Header.Set("Origin", "https://attacker.example")
	handler.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("不可信 Origin 未被拦截: %d", forbidden.Code)
	}
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/tasks/"+task.ID+"/files/agent.log", nil))
	if download.Code != http.StatusOK || !strings.Contains(download.Body.String(), "日志下载测试") {
		t.Fatalf("Agent 日志下载失败: %d %s", download.Code, download.Body.String())
	}
	if _, err := storage.Update(task.ID, func(current *domain.Task) error {
		current.Status = domain.StatusFailed
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
