// Package config 负责读取、展开和校验 YAML 配置。
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是服务的完整运行配置。
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Auth    AuthConfig    `yaml:"auth"`
	LLM     LLMConfig     `yaml:"llm"`
	HTTP    HTTPConfig    `yaml:"http"`
	Worker  WorkerConfig  `yaml:"worker"`
	Storage StorageConfig `yaml:"storage"`
	Docker  DockerConfig  `yaml:"docker"`
	AUR     AURConfig     `yaml:"aur"`
}

// ServerConfig 定义 HTTP 服务和跨域来源。
type ServerConfig struct {
	Address        string `yaml:"address"`
	FrontendOrigin string `yaml:"frontend_origin"`
}

// AuthConfig 定义页面密码、签名会话和登录 Cookie 安全选项。
type AuthConfig struct {
	Disabled         bool          `yaml:"disabled"`
	Password         string        `yaml:"password"`
	PasswordEnv      string        `yaml:"password_env"`
	SessionSecret    string        `yaml:"session_secret"`
	SessionSecretEnv string        `yaml:"session_secret_env"`
	SessionTTL       time.Duration `yaml:"session_ttl"`
	CookieSecure     bool          `yaml:"cookie_secure"`
}

// LLMConfig 定义 Anthropic 兼容大模型接口和 Agent 轮次限制。
type LLMConfig struct {
	BaseURL   string        `yaml:"base_url"`
	APIKey    string        `yaml:"api_key"`
	APIKeyEnv string        `yaml:"api_key_env"`
	Model     string        `yaml:"model"`
	MaxTokens int           `yaml:"max_tokens"`
	MaxRounds int           `yaml:"max_rounds"`
	Timeout   time.Duration `yaml:"timeout"`
}

// HTTPConfig 定义上游网页抓取的代理、超时、大小和标识。
type HTTPConfig struct {
	ProxyURL         string        `yaml:"proxy_url"`
	NoProxy          string        `yaml:"no_proxy"`
	Timeout          time.Duration `yaml:"timeout"`
	MaxResponseBytes int64         `yaml:"max_response_bytes"`
	UserAgent        string        `yaml:"user_agent"`
}

// WorkerConfig 定义后台任务并发数。
type WorkerConfig struct {
	Concurrency int `yaml:"concurrency"`
}

// StorageConfig 定义任务持久化目录。
type StorageConfig struct {
	Directory string `yaml:"directory"`
}

// DockerConfig 定义 Arch 构建容器及资源限制。
type DockerConfig struct {
	Image    string        `yaml:"image"`
	CPUs     string        `yaml:"cpus"`
	Memory   string        `yaml:"memory"`
	PIDs     int           `yaml:"pids"`
	Timeout  time.Duration `yaml:"timeout"`
	Network  bool          `yaml:"network"`
	ProxyURL string        `yaml:"proxy_url"`
	NoProxy  string        `yaml:"no_proxy"`
}

// AURConfig 定义自建 aurweb 的 Git 发布参数。
type AURConfig struct {
	Enabled       bool   `yaml:"enabled"`
	WebURL        string `yaml:"web_url"`
	RepositoryURL string `yaml:"repository_url"`
	SSHUser       string `yaml:"ssh_user"`
	SSHHost       string `yaml:"ssh_host"`
	SSHPort       int    `yaml:"ssh_port"`
	PrivateKey    string `yaml:"private_key"`
	KnownHosts    string `yaml:"known_hosts"`
	Branch        string `yaml:"branch"`
	GitName       string `yaml:"git_name"`
	GitEmail      string `yaml:"git_email"`
	GitUsername   string `yaml:"git_username"`
	GitToken      string `yaml:"git_token"`
	GitTokenEnv   string `yaml:"git_token_env"`
}

// Load 读取 YAML 文件、展开环境变量并返回经过校验的配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置: %w", err)
	}
	cfg.setDefaults()
	if cfg.LLM.APIKey == "" && cfg.LLM.APIKeyEnv != "" {
		cfg.LLM.APIKey = os.Getenv(cfg.LLM.APIKeyEnv)
	}
	if cfg.Auth.Password == "" && cfg.Auth.PasswordEnv != "" {
		cfg.Auth.Password = os.Getenv(cfg.Auth.PasswordEnv)
	}
	if cfg.Auth.SessionSecret == "" && cfg.Auth.SessionSecretEnv != "" {
		cfg.Auth.SessionSecret = os.Getenv(cfg.Auth.SessionSecretEnv)
	}
	if cfg.AUR.GitToken == "" && cfg.AUR.GitTokenEnv != "" {
		cfg.AUR.GitToken = os.Getenv(cfg.AUR.GitTokenEnv)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// setDefaults 为可选配置填充安全的默认值。
func (c *Config) setDefaults() {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}
	if c.Server.FrontendOrigin == "" {
		c.Server.FrontendOrigin = "http://localhost:3000"
	}
	if c.Auth.PasswordEnv == "" {
		c.Auth.PasswordEnv = "PACKPILOT_PASSWORD"
	}
	if c.Auth.SessionSecretEnv == "" {
		c.Auth.SessionSecretEnv = "PACKPILOT_SESSION_SECRET"
	}
	if c.Auth.SessionTTL == 0 {
		c.Auth.SessionTTL = 24 * time.Hour
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = "https://api.minimax.cn/anthropic"
	}
	if c.LLM.APIKeyEnv == "" {
		c.LLM.APIKeyEnv = "MINIMAX_API_KEY"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "MiniMax-M2.5"
	}
	if c.LLM.MaxTokens == 0 {
		c.LLM.MaxTokens = 8192
	}
	if c.LLM.MaxRounds == 0 {
		c.LLM.MaxRounds = 12
	}
	if c.LLM.Timeout == 0 {
		c.LLM.Timeout = 180 * time.Second
	}
	if c.HTTP.Timeout == 0 {
		c.HTTP.Timeout = 45 * time.Second
	}
	if c.HTTP.MaxResponseBytes == 0 {
		c.HTTP.MaxResponseBytes = 4 << 20
	}
	if c.HTTP.UserAgent == "" {
		c.HTTP.UserAgent = "AUR-PKGBUILD-Agent/1.0"
	}
	if c.Worker.Concurrency == 0 {
		c.Worker.Concurrency = 2
	}
	if c.Storage.Directory == "" {
		c.Storage.Directory = "./data"
	}
	if c.Docker.Image == "" {
		c.Docker.Image = "archlinux:base-devel"
	}
	if c.Docker.CPUs == "" {
		c.Docker.CPUs = "2"
	}
	if c.Docker.Memory == "" {
		c.Docker.Memory = "4g"
	}
	if c.Docker.PIDs == 0 {
		c.Docker.PIDs = 512
	}
	if c.Docker.Timeout == 0 {
		c.Docker.Timeout = 20 * time.Minute
	}
	if c.HTTP.ProxyURL != "" && c.HTTP.NoProxy == "" {
		c.HTTP.NoProxy = "localhost,127.0.0.1,::1"
	}
	if c.Docker.ProxyURL == "" {
		c.Docker.ProxyURL = c.HTTP.ProxyURL
	}
	if c.Docker.NoProxy == "" {
		c.Docker.NoProxy = c.HTTP.NoProxy
	}
	if c.AUR.SSHUser == "" {
		c.AUR.SSHUser = "aur"
	}
	if c.AUR.SSHPort == 0 {
		c.AUR.SSHPort = 22
	}
	if c.AUR.Branch == "" {
		c.AUR.Branch = "master"
	}
	if c.AUR.GitTokenEnv == "" {
		c.AUR.GitTokenEnv = "AUR_GIT_TOKEN"
	}
}

// validate 拒绝会导致服务无法安全运行的关键配置。
func (c *Config) validate() error {
	if !c.Auth.Disabled && len(c.Auth.Password) < 12 {
		return fmt.Errorf("页面密码至少需要 12 个字符，请设置 %s", c.Auth.PasswordEnv)
	}
	if c.Auth.SessionTTL < time.Minute || c.Auth.SessionTTL > 30*24*time.Hour {
		return fmt.Errorf("auth.session_ttl 必须在 1 分钟到 30 天之间")
	}
	if c.LLM.APIKey == "" {
		return fmt.Errorf("缺少大模型密钥，请设置 %s", c.LLM.APIKeyEnv)
	}
	for name, raw := range map[string]string{"llm.base_url": c.LLM.BaseURL, "server.frontend_origin": c.Server.FrontendOrigin} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("%s 必须是完整的 HTTP(S) URL", name)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("%s 仅支持 HTTP(S)", name)
		}
	}
	for name, raw := range map[string]string{"http.proxy_url": c.HTTP.ProxyURL, "docker.proxy_url": c.Docker.ProxyURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("%s 必须是完整的代理 URL", name)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
			return fmt.Errorf("%s 仅支持 http、https 和 socks5", name)
		}
	}
	if c.LLM.MaxTokens < 1024 || c.LLM.MaxRounds < 1 || c.LLM.MaxRounds > 50 {
		return fmt.Errorf("llm token 或轮次限制无效")
	}
	if c.Worker.Concurrency < 1 || c.Worker.Concurrency > 32 {
		return fmt.Errorf("worker.concurrency 必须在 1 到 32 之间")
	}
	if c.HTTP.MaxResponseBytes < 4096 || c.HTTP.Timeout <= 0 || c.Docker.Timeout <= 0 {
		return fmt.Errorf("HTTP 或 Docker 限制无效")
	}
	if strings.TrimSpace(c.Storage.Directory) == "" {
		return fmt.Errorf("storage.directory 不能为空")
	}
	if c.AUR.Enabled {
		parsed, _ := url.Parse(c.AUR.RepositoryURL)
		scheme := strings.ToLower(parsed.Scheme)
		switch scheme {
		case "http", "https":
			if c.AUR.GitToken == "" {
				return fmt.Errorf("HTTP(S) AUR 发布必须配置 git_token 或 %s", c.AUR.GitTokenEnv)
			}
		case "":
			if c.AUR.RepositoryURL == "" && (c.AUR.SSHHost == "" || c.AUR.PrivateKey == "") {
				return fmt.Errorf("标准 AUR SSH 发布必须配置 ssh_host 和 private_key")
			}
		default:
			if c.AUR.PrivateKey == "" {
				return fmt.Errorf("自定义 SSH AUR 发布必须配置 private_key")
			}
		}
	}
	return nil
}
