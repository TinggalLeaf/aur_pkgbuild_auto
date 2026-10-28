// Package builder 在隔离的 Arch Linux Docker 容器中验证 PKGBUILD。
package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aur_auto/internal/config"
)

// Result 保存一次 makepkg 构建的日志与元数据。
type Result struct {
	Success bool
	Log     string
	SRCINFO string
}

// Builder 使用 Docker CLI 创建资源受限的一次性构建容器。
type Builder struct {
	config config.DockerConfig
}

// New 创建 Arch Linux PKGBUILD 构建器。
func New(cfg config.DockerConfig) *Builder {
	return &Builder{config: cfg}
}

// Build 写入候选 PKGBUILD，运行 makepkg，并在成功时读取 .SRCINFO。
func (b *Builder) Build(ctx context.Context, taskID, workspace, pkgbuild string, attempt int) (*Result, error) {
	absoluteWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("解析工作目录: %w", err)
	}
	if err := os.WriteFile(filepath.Join(absoluteWorkspace, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		return nil, fmt.Errorf("写入 PKGBUILD: %w", err)
	}
	name := fmt.Sprintf("aur-agent-%s-%d", taskID, attempt)
	buildCtx, cancel := context.WithTimeout(ctx, b.config.Timeout)
	defer cancel()
	defer exec.Command("docker", "rm", "-f", name).Run()

	network := "none"
	if b.config.Network {
		network = "bridge"
	}
	args := []string{
		"run", "--rm", "--name", name,
		"--cpus", b.config.CPUs, "--memory", b.config.Memory,
		"--pids-limit", fmt.Sprint(b.config.PIDs),
		"--cap-drop", "ALL",
		"--cap-add", "CHOWN", "--cap-add", "SETUID", "--cap-add", "SETGID",
		"--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER",
		"--network", network, "-v", absoluteWorkspace + ":/work:rw",
	}
	if b.config.ProxyURL != "" {
		args = append(args, "--add-host", "host.docker.internal:host-gateway")
		for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			args = append(args, "--env", key+"="+b.config.ProxyURL)
		}
	}
	if b.config.NoProxy != "" {
		args = append(args, "--env", "NO_PROXY="+b.config.NoProxy, "--env", "no_proxy="+b.config.NoProxy)
	}
	args = append(args, b.config.Image, "bash", "-c", buildScript)
	command := exec.CommandContext(buildCtx, "docker", args...)
	output := &cappedBuffer{limit: 2 << 20}
	command.Stdout = output
	command.Stderr = output
	err = command.Run()
	log := output.String()
	if buildCtx.Err() == context.DeadlineExceeded {
		log += "\n构建超过 " + b.config.Timeout.String() + "，容器已终止。"
	}
	if ctx.Err() != nil {
		return &Result{Log: log}, ctx.Err()
	}
	if err != nil {
		if log == "" {
			log = err.Error()
		} else {
			log += "\n" + err.Error()
		}
		return &Result{Success: false, Log: log}, nil
	}
	srcinfo, readErr := os.ReadFile(filepath.Join(absoluteWorkspace, ".SRCINFO"))
	if readErr != nil {
		return &Result{Success: false, Log: log + "\n无法读取 .SRCINFO: " + readErr.Error()}, nil
	}
	return &Result{Success: true, Log: log, SRCINFO: string(srcinfo)}, nil
}

// buildScript 初始化非 root 构建用户并执行真实的依赖解析、构建和元数据生成。
const buildScript = `set -o pipefail
pacman -Sy --noconfirm --needed sudo
id build >/dev/null 2>&1 || useradd -m build
printf 'build ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/build
chmod 0777 /work
cd /work
runuser -u build -- bash -c 'cd /work && makepkg --syncdeps --noconfirm --cleanbuild && makepkg --printsrcinfo > .SRCINFO'
`

// cappedBuffer 保留构建日志开头并限制内存占用。
type cappedBuffer struct {
	bytes.Buffer
	limit int
}

// Write 将未超出上限的日志写入缓冲区，同时始终向 Writer 调用者报告成功。
func (b *cappedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := b.limit - b.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		b.Buffer.Write(data)
	}
	return originalLength, nil
}

// String 返回日志，并在发生截断时附加明确提示。
func (b *cappedBuffer) String() string {
	text := b.Buffer.String()
	if b.Len() >= b.limit {
		text += "\n[日志已达到 2 MiB 上限，后续输出已省略]"
	}
	return strings.TrimSpace(text)
}

// Available 检查 Docker CLI 和守护进程是否可用于真实构建。
func (b *Builder) Available(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(checkCtx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Docker 不可用: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
