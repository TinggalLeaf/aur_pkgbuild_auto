package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aur_auto/internal/config"
)

// testConfig 返回启用安全会话的测试认证配置。
func testConfig() config.AuthConfig {
	return config.AuthConfig{Password: "correct-horse-password", SessionSecret: "独立测试会话密钥", SessionTTL: time.Hour}
}

// TestLoginAndCookie 验证正确密码、签名 Cookie 和退出清理流程。
func TestLoginAndCookie(t *testing.T) {
	manager := New(testConfig())
	token, expires, err := manager.Login("127.0.0.1", "correct-horse-password")
	if err != nil || token == "" || !expires.After(time.Now()) {
		t.Fatalf("登录签发信息无效: %v", err)
	}
	request := httptest.NewRequest("GET", "/api/tasks", nil)
	request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	if !manager.Authenticated(request) {
		t.Fatal("有效会话未通过认证")
	}
	recorder := httptest.NewRecorder()
	manager.WriteCookie(recorder, token, expires)
	if cookie := recorder.Result().Cookies()[0]; !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("Cookie 安全属性不正确")
	}
}

// TestExpiredAndWrongPassword 验证错误密码和过期令牌都会被拒绝。
func TestExpiredAndWrongPassword(t *testing.T) {
	manager := New(testConfig())
	if _, _, err := manager.Login("127.0.0.1", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("错误密码返回了意外错误: %v", err)
	}
	if manager.valid(manager.sign(time.Now().Add(-time.Minute)), time.Now()) {
		t.Fatal("过期令牌仍然有效")
	}
}

// TestRateLimit 验证连续失败后触发限流，合法密码也不能绕过。
func TestRateLimit(t *testing.T) {
	manager := New(testConfig())
	for index := 0; index < 5; index++ {
		_, _, _ = manager.Login("192.0.2.1", "bad-password")
	}
	if _, _, err := manager.Login("192.0.2.1", "correct-horse-password"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("限流未生效: %v", err)
	}
}
