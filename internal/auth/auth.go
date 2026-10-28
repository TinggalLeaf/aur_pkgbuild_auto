// Package auth 提供页面密码、签名会话 Cookie 和登录限流。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aur_auto/internal/config"
)

const cookieName = "packpilot_session"

// ErrInvalidCredentials 表示页面密码不正确。
var ErrInvalidCredentials = errors.New("密码不正确")

// ErrTooManyAttempts 表示客户端在短时间内登录失败过多。
var ErrTooManyAttempts = errors.New("登录失败次数过多，请稍后再试")

// Manager 校验密码并签发和验证无状态会话。
type Manager struct {
	enabled      bool
	passwordHash [32]byte
	secret       []byte
	ttl          time.Duration
	secure       bool
	mu           sync.Mutex
	failures     map[string]*failureWindow
}

// failureWindow 保存一个客户端在当前限流窗口内的失败次数。
type failureWindow struct {
	First time.Time
	Count int
}

// New 使用配置密码和独立会话密钥创建认证管理器。
func New(cfg config.AuthConfig) *Manager {
	secret := []byte(cfg.SessionSecret)
	if len(secret) == 0 {
		digest := sha256.Sum256([]byte("packpilot-session:" + cfg.Password))
		secret = digest[:]
	}
	return &Manager{
		enabled: !cfg.Disabled, passwordHash: sha256.Sum256([]byte(cfg.Password)),
		secret: secret, ttl: cfg.SessionTTL, secure: cfg.CookieSecure,
		failures: make(map[string]*failureWindow),
	}
}

// Enabled 返回是否启用页面和 API 认证。
func (m *Manager) Enabled() bool {
	return m.enabled
}

// Login 校验限流和密码，成功时返回签名 Cookie 值及过期时间。
func (m *Manager) Login(client, password string) (string, time.Time, error) {
	if !m.enabled {
		return "", time.Time{}, fmt.Errorf("认证未启用")
	}
	if err := m.checkRateLimit(client); err != nil {
		return "", time.Time{}, err
	}
	candidate := sha256.Sum256([]byte(password))
	if !hmac.Equal(candidate[:], m.passwordHash[:]) {
		m.recordFailure(client)
		return "", time.Time{}, ErrInvalidCredentials
	}
	m.mu.Lock()
	delete(m.failures, client)
	m.mu.Unlock()
	expires := time.Now().UTC().Add(m.ttl)
	return m.sign(expires), expires, nil
}

// Authenticated 判断请求是否带有有效且未过期的会话 Cookie。
func (m *Manager) Authenticated(request *http.Request) bool {
	if !m.enabled {
		return true
	}
	cookie, err := request.Cookie(cookieName)
	if err != nil {
		return false
	}
	return m.valid(cookie.Value, time.Now().UTC())
}

// WriteCookie 写入 HttpOnly、SameSite=Lax 的登录会话 Cookie。
func (m *Manager) WriteCookie(writer http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name: cookieName, Value: value, Path: "/", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true,
		Secure: m.secure, SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie 立即过期登录会话 Cookie。
func (m *Manager) ClearCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		Expires: time.Unix(0, 0), HttpOnly: true, Secure: m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// sign 对过期时间进行 HMAC 签名并生成 URL 安全令牌。
func (m *Manager) sign(expires time.Time) string {
	payload := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

// valid 使用常量时间比较验证令牌签名和有效期。
func (m *Manager) valid(token string, now time.Time) bool {
	payload, signature, found := strings.Cut(token, ".")
	if !found || payload == "" || signature == "" {
		return false
	}
	expires, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || now.Unix() >= expires {
		return false
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// checkRateLimit 在密码校验前拒绝仍处于限流窗口的客户端。
func (m *Manager) checkRateLimit(client string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	failure := m.failures[client]
	if failure == nil {
		return nil
	}
	if time.Since(failure.First) >= 5*time.Minute {
		delete(m.failures, client)
		return nil
	}
	if failure.Count >= 5 {
		return ErrTooManyAttempts
	}
	return nil
}

// recordFailure 记录一次密码失败，并按窗口起始时间限制后续尝试。
func (m *Manager) recordFailure(client string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	failure := m.failures[client]
	if failure == nil || time.Since(failure.First) >= 5*time.Minute {
		m.failures[client] = &failureWindow{First: time.Now(), Count: 1}
		return
	}
	failure.Count++
}
