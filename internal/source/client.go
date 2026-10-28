// Package source 提供限制私网访问的网页与 GitHub 源码发现能力。
package source

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"aur_auto/internal/netutil"
)

// Client 使用受限 HTTP 客户端抓取公开上游信息。
type Client struct {
	httpClient *http.Client
	maxBytes   int64
	userAgent  string
	proxied    bool
}

// Page 是适合交给大模型阅读的纯文本网页快照。
type Page struct {
	URL         string   `json:"url"`
	ContentType string   `json:"content_type"`
	Text        string   `json:"text"`
	Links       []string `json:"links,omitempty"`
}

// Release 描述一个 GitHub release 及其可下载资产。
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Tarball string  `json:"tarball_url"`
	Zipball string  `json:"zipball_url"`
	Assets  []Asset `json:"assets"`
}

// Asset 描述 GitHub release 中一个可公开下载的文件。
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// New 创建带代理、超时、重定向检查、响应大小限制和私网保护的抓取客户端。
func New(timeout time.Duration, maxBytes int64, userAgent, proxyURL, noProxy string) (*Client, error) {
	transport, err := netutil.Transport(proxyURL, noProxy)
	if err != nil {
		return nil, err
	}
	client := &Client{maxBytes: maxBytes, userAgent: userAgent, proxied: proxyURL != ""}
	client.httpClient = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return validatePublicURL(request.Context(), request.URL, client.proxied)
		},
	}
	return client, nil
}

// Fetch 读取公开网页并返回去标签、解码实体后的有限长度文本。
func (c *Client) Fetch(ctx context.Context, rawURL string) (*Page, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("无效的 URL: %w", err)
	}
	if err := validatePublicURL(ctx, parsed, c.proxied); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "text/html,application/json,text/plain;q=0.9,*/*;q=0.1")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("抓取网页: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("上游返回 HTTP %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, c.maxBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取网页: %w", err)
	}
	if int64(len(data)) > c.maxBytes {
		return nil, fmt.Errorf("网页超过 %d 字节限制", c.maxBytes)
	}
	contentType := response.Header.Get("Content-Type")
	text := string(data)
	links := []string(nil)
	if strings.Contains(contentType, "html") || looksLikeHTML(text) {
		links = extractLinks(text, response.Request.URL)
		text = htmlToText(text)
	}
	return &Page{URL: response.Request.URL.String(), ContentType: contentType, Text: text, Links: links}, nil
}

// Checksum 下载公开源码归档并计算 SHA-256，拒绝超过 1 GiB 的文件。
func (c *Client) Checksum(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("无效的源码 URL: %w", err)
	}
	if err := validatePublicURL(ctx, parsed, c.proxied); err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("下载源码归档: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("源码服务器返回 HTTP %d", response.StatusCode)
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(response.Body, 1<<30+1))
	if err != nil {
		return "", fmt.Errorf("计算源码校验和: %w", err)
	}
	if written > 1<<30 {
		return "", fmt.Errorf("源码归档超过 1 GiB 限制")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GitHubReleases 查询 GitHub 仓库最近的 releases 和源码归档地址。
func (c *Client) GitHubReleases(ctx context.Context, repositoryURL string) ([]Release, error) {
	owner, repository, err := githubRepository(repositoryURL)
	if err != nil {
		return nil, err
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=5", url.PathEscape(owner), url.PathEscape(repository))
	page, err := c.Fetch(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	var releases []Release
	if err := json.Unmarshal([]byte(page.Text), &releases); err != nil {
		return nil, fmt.Errorf("解析 GitHub releases: %w", err)
	}
	return releases, nil
}

// githubRepository 从 GitHub 仓库 URL 提取所有者和仓库名。
func githubRepository(rawURL string) (string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "github.com" {
		return "", "", fmt.Errorf("仅支持 github.com 仓库 URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("GitHub URL 缺少所有者或仓库名")
	}
	repository := strings.TrimSuffix(parts[1], ".git")
	if parts[0] == "" || repository == "" {
		return "", "", fmt.Errorf("GitHub 仓库地址无效")
	}
	return parts[0], repository, nil
}

// validatePublicURL 阻止私网目标；显式代理模式允许代理解析暂时无法本地解析的公网域名。
func validatePublicURL(ctx context.Context, parsed *url.URL, proxied bool) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅允许 HTTP(S) URL")
	}
	hostname := parsed.Hostname()
	if hostname == "" || parsed.User != nil {
		return fmt.Errorf("URL 主机无效或包含凭据")
	}
	if strings.EqualFold(hostname, "localhost") || strings.HasSuffix(strings.ToLower(hostname), ".localhost") {
		return fmt.Errorf("禁止访问本地主机")
	}
	if address, err := netip.ParseAddr(hostname); err == nil {
		if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
			return fmt.Errorf("禁止访问非公网地址 %s", address.String())
		}
		return nil
	}
	if proxied {
		return nil
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", hostname)
	if err != nil {
		return fmt.Errorf("解析上游主机: %w", err)
	}
	for _, address := range addresses {
		if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
			return fmt.Errorf("禁止访问非公网地址 %s", address.String())
		}
	}
	return nil
}

// extractLinks 提取网页超链接并转为绝对 URL，让模型能继续追踪下载地址。
func extractLinks(input string, base *url.URL) []string {
	pattern := regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)
	matches := pattern.FindAllStringSubmatch(input, 100)
	links := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		reference, err := url.Parse(html.UnescapeString(strings.TrimSpace(match[1])))
		if err != nil || reference.Scheme == "javascript" || reference.Scheme == "mailto" {
			continue
		}
		absolute := base.ResolveReference(reference)
		if (absolute.Scheme == "http" || absolute.Scheme == "https") && !seen[absolute.String()] {
			seen[absolute.String()] = true
			links = append(links, absolute.String())
		}
	}
	return links
}

// htmlToText 移除脚本、样式和标签，保留便于模型分析的换行文本。
func htmlToText(input string) string {
	script := regexp.MustCompile(`(?is)<(script|style|svg)\b[^>]*>.*?</(script|style|svg)>`)
	tags := regexp.MustCompile(`(?s)<[^>]+>`)
	lines := regexp.MustCompile(`(?i)<(?:br|/p|/div|/li|/tr|/h[1-6])\s*/?>`)
	text := script.ReplaceAllString(input, " ")
	text = lines.ReplaceAllString(text, "\n")
	text = tags.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

// looksLikeHTML 判断缺少 Content-Type 的响应是否仍应按 HTML 处理。
func looksLikeHTML(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html")
}

// IsPrivateAddress 暴露公网地址判断，供其他网络边界复用。
func IsPrivateAddress(address string) bool {
	parsed, err := netip.ParseAddr(address)
	return err == nil && (!parsed.IsGlobalUnicast() || parsed.IsPrivate() || parsed.IsLoopback())
}
