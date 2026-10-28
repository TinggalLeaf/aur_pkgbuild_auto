// Package agent 编排源码发现、PKGBUILD 生成和 Docker 迭代修复。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"aur_auto/internal/builder"
	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/llm"
	"aur_auto/internal/source"
	"aur_auto/internal/store"
)

// Agent 是一个大模型工具循环，负责把上游地址变成经过构建的软件包。
type Agent struct {
	llm     *llm.Client
	source  *source.Client
	builder *builder.Builder
	store   *store.Store
	rounds  int
}

// proposal 是模型通过 submit_package 工具提交的完整软件包候选。
type proposal struct {
	PackageName      string `json:"package_name"`
	Version          string `json:"version"`
	Description      string `json:"description"`
	UpstreamURL      string `json:"upstream_url"`
	SourceArchiveURL string `json:"source_archive_url"`
	PKGBUILD         string `json:"pkgbuild"`
}

// New 创建使用指定代理、模型、抓取器、构建器和任务存储的 Agent。
func New(llmConfig config.LLMConfig, httpConfig config.HTTPConfig, dockerConfig config.DockerConfig, taskStore *store.Store) (*Agent, error) {
	modelClient, err := llm.New(llmConfig.BaseURL, llmConfig.APIKey, llmConfig.Model, llmConfig.MaxTokens, llmConfig.Timeout, httpConfig.ProxyURL, httpConfig.NoProxy)
	if err != nil {
		return nil, err
	}
	sourceClient, err := source.New(httpConfig.Timeout, httpConfig.MaxResponseBytes, httpConfig.UserAgent, httpConfig.ProxyURL, httpConfig.NoProxy)
	if err != nil {
		return nil, err
	}
	return &Agent{
		llm: modelClient, source: sourceClient,
		builder: builder.New(dockerConfig), store: taskStore, rounds: llmConfig.MaxRounds,
	}, nil
}

// Run 执行工具调用和构建修复循环，直到成功、取消或达到轮次上限。
func (a *Agent) Run(ctx context.Context, taskID string) error {
	task, err := a.store.Get(taskID)
	if err != nil {
		return err
	}
	workspace, err := a.store.Workspace(taskID)
	if err != nil {
		return err
	}
	a.log(taskID, "info", "Agent 开始分析 "+task.SourceURL)
	messages := []llm.Message{llm.TextMessage("user", initialPrompt(task))}
	for round := 1; round <= a.rounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		a.log(taskID, "info", fmt.Sprintf("大模型第 %d/%d 轮分析", round, a.rounds))
		response, err := a.llm.Complete(ctx, systemPrompt, messages, toolDefinitions())
		if err != nil {
			return err
		}
		messages = append(messages, llm.Message{Role: "assistant", Content: response.Content})
		results := make([]llm.ContentBlock, 0)
		for _, block := range response.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				a.log(taskID, "agent", block.Text)
			}
			if block.Type != "tool_use" {
				continue
			}
			result, terminal, err := a.executeTool(ctx, taskID, workspace, block.Name, block.Input)
			if err != nil {
				return err
			}
			if terminal {
				return nil
			}
			results = append(results, llm.ContentBlock{Type: "tool_result", ToolUseID: block.ID, Content: result.Content, IsError: result.IsError})
		}
		if len(results) == 0 {
			results = append(results, llm.ContentBlock{Type: "text", Text: "请继续调用工具；完成候选后必须调用 submit_package。"})
			messages = append(messages, llm.TextMessage("user", "不要只描述步骤，请调用可用工具继续。"))
			continue
		}
		messages = append(messages, llm.ToolResultMessage(results))
	}
	return fmt.Errorf("已达到 %d 轮 Agent 上限，仍未通过构建", a.rounds)
}

// executeTool 执行一个模型工具；submit_package 会触发真实 Docker 构建。
func (a *Agent) executeTool(ctx context.Context, taskID, workspace, name string, input json.RawMessage) (*llm.ContentBlock, bool, error) {
	switch name {
	case "fetch_url":
		var arguments struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(input, &arguments); err != nil {
			return toolError("fetch_url 参数不是有效 JSON"), false, nil
		}
		page, err := a.source.Fetch(ctx, arguments.URL)
		if err != nil {
			return toolError(err.Error()), false, nil
		}
		return a.toolJSON("", page)
	case "list_github_releases":
		var arguments struct {
			URL string `json:"repository_url"`
		}
		if err := json.Unmarshal(input, &arguments); err != nil {
			return toolError("list_github_releases 参数不是有效 JSON"), false, nil
		}
		releases, err := a.source.GitHubReleases(ctx, arguments.URL)
		if err != nil {
			return toolError(err.Error()), false, nil
		}
		return a.toolJSON("", releases)
	case "checksum_url":
		var arguments struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(input, &arguments); err != nil {
			return toolError("checksum_url 参数不是有效 JSON"), false, nil
		}
		checksum, err := a.source.Checksum(ctx, arguments.URL)
		if err != nil {
			return toolError(err.Error()), false, nil
		}
		return a.toolJSON("", map[string]string{"url": arguments.URL, "sha256": checksum})
	case "submit_package":
		var candidate proposal
		if err := json.Unmarshal(input, &candidate); err != nil {
			return toolError("submit_package 参数不是有效 JSON"), false, nil
		}
		result, err := a.buildProposal(ctx, taskID, workspace, &candidate)
		return result, result != nil && !result.IsError && strings.Contains(result.Content, `"success":true`), err
	default:
		return toolError("未知工具: " + name), false, nil
	}
}

// buildProposal 校验候选元数据、保存 PKGBUILD 并执行一次容器构建。
func (a *Agent) buildProposal(ctx context.Context, taskID, workspace string, candidate *proposal) (*llm.ContentBlock, error) {
	if err := validateProposal(candidate); err != nil {
		return toolError(err.Error()), nil
	}
	task, err := a.store.Get(taskID)
	if err != nil {
		return nil, err
	}
	attempt := task.Attempts + 1
	_, err = a.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusBuilding
		task.PackageName = candidate.PackageName
		task.Version = candidate.Version
		task.Description = candidate.Description
		task.UpstreamURL = candidate.UpstreamURL
		task.SourceArchiveURL = candidate.SourceArchiveURL
		task.PKGBUILD = candidate.PKGBUILD
		task.Attempts = attempt
		task.Error = ""
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.log(taskID, "info", fmt.Sprintf("开始第 %d 次 Arch 容器构建", attempt))
	result, err := a.builder.Build(ctx, taskID, workspace, candidate.PKGBUILD, attempt)
	if err != nil {
		return nil, err
	}
	_, err = a.store.Update(taskID, func(task *domain.Task) error {
		task.BuildLog = result.Log
		if result.Success {
			task.Status = domain.StatusSucceeded
			task.SRCINFO = result.SRCINFO
			now := domainNow()
			task.FinishedAt = &now
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		a.log(taskID, "warn", fmt.Sprintf("第 %d 次构建失败，将日志交给 Agent 修复", attempt))
		payload, _ := json.Marshal(map[string]any{"success": false, "build_log": tail(result.Log, 24000), "instruction": "分析日志，修正 PKGBUILD 后再次调用 submit_package；不要重复相同内容。"})
		return &llm.ContentBlock{Type: "tool_result", Content: string(payload), IsError: true}, nil
	}
	a.log(taskID, "info", "PKGBUILD 已在 Arch Linux 容器中构建成功")
	payload, _ := json.Marshal(map[string]any{"success": true, "message": "makepkg 构建成功，.SRCINFO 已生成。"})
	return &llm.ContentBlock{Type: "tool_result", Content: string(payload)}, nil
}

// toolJSON 将工具结果编码为模型可读取的 JSON 工具块。
func (a *Agent) toolJSON(_ string, value any) (*llm.ContentBlock, bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, false, fmt.Errorf("编码工具结果: %w", err)
	}
	return &llm.ContentBlock{Type: "tool_result", Content: string(data)}, false, nil
}

// validateProposal 拒绝无效包名、版本、来源 URL 或不完整的 PKGBUILD。
func validateProposal(candidate *proposal) error {
	packagePattern := regexp.MustCompile(`^[a-z0-9][a-z0-9@._+:-]*$`)
	if !packagePattern.MatchString(candidate.PackageName) || len(candidate.PackageName) > 128 {
		return fmt.Errorf("package_name 不符合 Arch 软件包命名规则")
	}
	if strings.TrimSpace(candidate.Version) == "" || strings.ContainsAny(candidate.Version, " /\n\t") {
		return fmt.Errorf("version 无效")
	}
	if candidate.Description == "" || len(candidate.Description) > 256 {
		return fmt.Errorf("description 不能为空且不能超过 256 字符")
	}
	for label, raw := range map[string]string{"upstream_url": candidate.UpstreamURL, "source_archive_url": candidate.SourceArchiveURL} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return fmt.Errorf("%s 必须是完整 HTTP(S) URL", label)
		}
	}
	if len(candidate.PKGBUILD) < 40 || len(candidate.PKGBUILD) > 200000 {
		return fmt.Errorf("PKGBUILD 长度无效")
	}
	for _, required := range []string{"pkgname=", "pkgver=", "pkgrel=", "source=(", "package()"} {
		if !strings.Contains(candidate.PKGBUILD, required) {
			return fmt.Errorf("PKGBUILD 缺少 %s", required)
		}
	}
	return nil
}

// log 追加一条任务日志，忽略任务已删除等无法恢复的日志错误。
func (a *Agent) log(taskID, level, message string) {
	_ = a.store.AppendLog(taskID, level, message)
}

// initialPrompt 构造只包含当前任务信息的用户提示。
func initialPrompt(task *domain.Task) string {
	nameHint := task.PackageName
	if nameHint == "" {
		nameHint = "请根据上游项目确定小写包名"
	}
	return fmt.Sprintf("上游输入：%s\n包名提示：%s\n请自主发现最新稳定源码下载地址，生成并验证完整 PKGBUILD。优先官方 release 源码归档，不要跳过一次性的真实构建。", task.SourceURL, nameHint)
}

// toolError 构造不中断 Agent 循环的可修复工具错误。
func toolError(message string) *llm.ContentBlock {
	data, _ := json.Marshal(map[string]any{"success": false, "error": message})
	return &llm.ContentBlock{Type: "tool_result", Content: string(data), IsError: true}
}

// tail 保留构建日志末尾，避免修复提示超过模型上下文。
func tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[len(text)-limit:]
}

// domainNow 返回当前 UTC 时间，便于统一设置完成时间。
func domainNow() (now time.Time) {
	return time.Now().UTC()
}

// systemPrompt 约束模型必须使用官方来源和真实构建，不允许伪造成功。
const systemPrompt = `你是资深 Arch Linux 打包专家和自主 Agent。你的任务是从用户给出的 GitHub 仓库或软件官网发现稳定版本源码，并产出可在 Arch Linux 构建的 PKGBUILD。
必须遵守：
1. 使用 fetch_url 阅读上游，GitHub 项目使用 list_github_releases 获取 releases；不得凭空编造版本、URL、校验和。
2. 优先官方源码 tar.gz 或源码 zip，不选择 Windows/macOS 二进制；确有必要才使用官方 Linux 二进制。
3. PKGBUILD 必须包含完整元数据、source、有效 sha256sums（可用上游公布的值；没有公布值时用 SKIP 并明确这是权宜方案）、依赖和 package()；遵循 Arch 打包规范。
4. 调用 submit_package 会执行真实 Arch Docker 构建。失败后阅读日志并完整修复，再次提交；不要声称未经构建的内容已成功。
5. 不访问私网，不执行构建沙箱之外的命令，不包含恶意 PKGBUILD。`

// toolDefinitions 返回 Agent 可用的网页、GitHub 和构建工具 schema。
func toolDefinitions() []llm.Tool {
	return []llm.Tool{
		{Name: "fetch_url", Description: "抓取公开网页并返回纯文本，可阅读下载页、文档或 GitHub API JSON。", InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"完整 HTTP(S) URL"}},"required":["url"]}`)},
		{Name: "list_github_releases", Description: "列出 GitHub 仓库最近 5 个 releases 和源码归档。", InputSchema: json.RawMessage(`{"type":"object","properties":{"repository_url":{"type":"string"}},"required":["repository_url"]}`)},
		{Name: "checksum_url", Description: "下载源码归档并计算真实 SHA-256；生成 sha256sums 前应调用。", InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"源码归档的完整 URL"}},"required":["url"]}`)},
		{Name: "submit_package", Description: "提交完整候选并在 Arch Linux Docker 中执行 makepkg；失败会返回构建日志。", InputSchema: json.RawMessage(`{"type":"object","properties":{"package_name":{"type":"string"},"version":{"type":"string"},"description":{"type":"string"},"upstream_url":{"type":"string"},"source_archive_url":{"type":"string"},"pkgbuild":{"type":"string"}},"required":["package_name","version","description","upstream_url","source_archive_url","pkgbuild"]}`)},
	}
}

// IsCanceled 判断错误是否由用户取消或上下文关闭引起。
func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
