package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDefaults 验证默认值、环境密钥和代理继承能够正确加载。
func TestLoadDefaults(t *testing.T) {
	t.Setenv("TEST_LLM_KEY", "test-key")
	t.Setenv("TEST_PASSWORD", "a-secure-test-password")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "auth:\n  password_env: TEST_PASSWORD\nllm:\n  api_key_env: TEST_LLM_KEY\nhttp:\n  proxy_url: http://127.0.0.1:7890\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.APIKey != "test-key" || cfg.Auth.Password != "a-secure-test-password" {
		t.Fatal("环境变量密钥未加载")
	}
	if cfg.Docker.ProxyURL != cfg.HTTP.ProxyURL || cfg.Worker.Concurrency != 2 {
		t.Fatal("代理继承或 worker 默认值不正确")
	}
}

// TestPasswordAndProxyValidation 验证弱密码和非法代理会在启动前失败。
func TestPasswordAndProxyValidation(t *testing.T) {
	for name, content := range map[string]string{
		"weak-password": "llm:\n  api_key: key\nauth:\n  password: short\n",
		"bad-proxy":     "llm:\n  api_key: key\nauth:\n  disabled: true\nhttp:\n  proxy_url: ftp://127.0.0.1:21\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("无效配置未被拒绝")
			}
		})
	}
}
