// Package llm 实现 Anthropic Messages API 的工具调用客户端。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aur_auto/internal/netutil"
)

// Message 表示对话中的一条用户或助手消息。
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock 同时承载文本、工具调用和工具结果。
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// Tool 描述模型可以请求执行的一个本地工具。
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Response 是一次 Messages API 调用返回的内容。
type Response struct {
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

// Client 保存 Anthropic 兼容服务连接参数。
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

// New 创建支持代理且不会将密钥写入日志的 Anthropic 兼容客户端。
func New(baseURL, apiKey, model string, maxTokens int, timeout time.Duration, proxyURL, noProxy string) (*Client, error) {
	transport, err := netutil.Transport(proxyURL, noProxy)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model,
		maxTokens: maxTokens, httpClient: &http.Client{Timeout: timeout, Transport: transport},
	}, nil
}

// Complete 发送对话和工具定义，并返回模型的文本或工具调用。
func (c *Client) Complete(ctx context.Context, system string, messages []Message, tools []Tool) (*Response, error) {
	payload := struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system"`
		Messages  []Message `json:"messages"`
		Tools     []Tool    `json:"tools,omitempty"`
	}{Model: c.model, MaxTokens: c.maxTokens, System: system, Messages: messages, Tools: tools}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("编码模型请求: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", c.apiKey)
	request.Header.Set("anthropic-version", "2023-06-01")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求大模型: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("读取模型响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if len(message) > 2000 {
			message = message[:2000]
		}
		return nil, fmt.Errorf("大模型返回 HTTP %d: %s", response.StatusCode, message)
	}
	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("解析模型响应: %w", err)
	}
	if len(decoded.Content) == 0 {
		return nil, fmt.Errorf("大模型返回了空内容")
	}
	return &decoded, nil
}

// TextMessage 创建一条纯文本用户或助手消息。
func TextMessage(role, text string) Message {
	return Message{Role: role, Content: []ContentBlock{{Type: "text", Text: text}}}
}

// ToolResultMessage 将本地工具执行结果作为用户消息返回给模型。
func ToolResultMessage(results []ContentBlock) Message {
	return Message{Role: "user", Content: results}
}
