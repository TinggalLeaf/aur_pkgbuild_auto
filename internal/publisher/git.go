// Package publisher 将验证通过的软件包提交到自建 aurweb Git 仓库。
package publisher

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/store"
)

// Publisher 使用 Git CLI 和独立 SSH 配置发布 PKGBUILD。
type Publisher struct {
	config config.AURConfig
	store  *store.Store
}

// New 创建自建 aurweb 发布器。
func New(cfg config.AURConfig, taskStore *store.Store) *Publisher {
	return &Publisher{config: cfg, store: taskStore}
}

// Publish 克隆软件包仓库，写入已验证文件，提交并推送指定分支。
func (p *Publisher) Publish(ctx context.Context, taskID string) error {
	task, err := p.store.Get(taskID)
	if err != nil {
		return err
	}
	if !p.config.Enabled || !task.PublishRequested {
		return fmt.Errorf("AUR 总开关或任务发布请求未开启")
	}
	if task.Status != domain.StatusSucceeded || task.PKGBUILD == "" || task.SRCINFO == "" {
		return fmt.Errorf("只有构建成功的任务才能发布")
	}
	if _, err := p.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusPublishing
		task.Error = ""
		return nil
	}); err != nil {
		return err
	}
	if err := p.publish(ctx, task); err != nil {
		_, _ = p.store.Update(taskID, func(current *domain.Task) error {
			current.Status = domain.StatusSucceeded
			current.Error = "发布失败: " + err.Error()
			return nil
		})
		return err
	}
	return nil
}

// publish 执行实际 Git 操作，并把 commit 结果保存到任务。
func (p *Publisher) publish(ctx context.Context, task *domain.Task) error {
	workspace, err := p.store.Workspace(task.ID)
	if err != nil {
		return err
	}
	repositoryDirectory := filepath.Join(workspace, "aur-repository")
	repository := p.repositoryURL(task.PackageName)
	if _, err := os.Stat(filepath.Join(repositoryDirectory, ".git")); os.IsNotExist(err) {
		if err := os.RemoveAll(repositoryDirectory); err != nil {
			return err
		}
		if err := p.git(ctx, workspace, "clone", repository, repositoryDirectory); err != nil {
			return fmt.Errorf("克隆 AUR 仓库: %w", err)
		}
	} else {
		if err := p.git(ctx, repositoryDirectory, "fetch", "origin"); err != nil {
			return fmt.Errorf("更新 AUR 仓库: %w", err)
		}
		if err := p.git(ctx, repositoryDirectory, "pull", "--rebase", "origin", p.config.Branch); err != nil {
			return fmt.Errorf("同步 AUR 远端变化: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(repositoryDirectory, "PKGBUILD"), []byte(task.PKGBUILD), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repositoryDirectory, ".SRCINFO"), []byte(task.SRCINFO), 0o644); err != nil {
		return err
	}
	for _, arguments := range [][]string{
		{"config", "user.name", p.config.GitName},
		{"config", "user.email", p.config.GitEmail},
		{"add", "PKGBUILD", ".SRCINFO"},
	} {
		if err := p.git(ctx, repositoryDirectory, arguments...); err != nil {
			return err
		}
	}
	if err := p.git(ctx, repositoryDirectory, "diff", "--cached", "--quiet"); err == nil {
		return p.record(task.ID, repository, "无文件变化")
	}
	message := fmt.Sprintf("Update %s to %s", task.PackageName, task.Version)
	if err := p.git(ctx, repositoryDirectory, "commit", "-m", message); err != nil {
		return fmt.Errorf("提交软件包: %w", err)
	}
	if err := p.git(ctx, repositoryDirectory, "push", "origin", "HEAD:"+p.config.Branch); err != nil {
		return fmt.Errorf("推送 AUR 仓库: %w", err)
	}
	return p.record(task.ID, repository, "")
}

// record 读取当前 commit 并将发布结果标记为已完成。
func (p *Publisher) record(taskID, repository, note string) error {
	workspace, err := p.store.Workspace(taskID)
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", filepath.Join(workspace, "aur-repository"), "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("读取发布 commit: %w", err)
	}
	commit := strings.TrimSpace(string(output))
	_, err = p.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusPublished
		task.Publication = &domain.Publication{
			Repository: repository, Branch: p.config.Branch, Commit: commit,
			PackageURL: strings.TrimRight(p.config.WebURL, "/") + "/packages/" + task.PackageName,
			PushedAt:   time.Now().UTC(),
		}
		task.Error = note
		return nil
	})
	return err
}

// repositoryURL 优先展开自定义模板，否则按 aurweb 约定生成软件包 SSH 地址。
func (p *Publisher) repositoryURL(packageName string) string {
	if p.config.RepositoryURL != "" {
		return strings.ReplaceAll(p.config.RepositoryURL, "{pkg}", url.PathEscape(packageName))
	}
	host := p.config.SSHHost
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("ssh://%s@%s:%d/%s.git", url.PathEscape(p.config.SSHUser), host, p.config.SSHPort, url.PathEscape(packageName))
}

// git 在固定目录执行 Git，通过环境隔离 SSH 身份，并用临时 Askpass 保护 HTTP token。
func (p *Publisher) git(ctx context.Context, directory string, arguments ...string) error {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", directory}, arguments...)...)
	sshCommand := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", shellQuote(p.config.PrivateKey))
	if p.config.KnownHosts != "" {
		sshCommand = fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o UserKnownHostsFile=%s -o StrictHostKeyChecking=yes", shellQuote(p.config.PrivateKey), shellQuote(p.config.KnownHosts))
	}
	command.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand, "GIT_TERMINAL_PROMPT=0")
	if p.config.GitToken != "" {
		askpass := filepath.Join(directory, ".packpilot-askpass.sh")
		script := "#!/bin/sh\ncase \"$1\" in\n*Username*) printf '%s\\n' \"$PACKPILOT_GIT_USERNAME\" ;;\n*) printf '%s\\n' \"$PACKPILOT_GIT_PASSWORD\" ;;\nesac\n"
		if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
			return fmt.Errorf("创建 Git 凭据助手: %w", err)
		}
		defer os.Remove(askpass)
		command.Env = append(command.Env,
			"GIT_ASKPASS="+askpass,
			"PACKPILOT_GIT_USERNAME="+p.config.GitUsername,
			"PACKPILOT_GIT_PASSWORD="+p.config.GitToken,
		)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", arguments[0], strings.TrimSpace(string(output)))
	}
	return nil
}

// shellQuote 为 GIT_SSH_COMMAND 中的单个路径提供安全单引号转义。
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
