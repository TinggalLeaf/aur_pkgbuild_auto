// Command server 启动 AUR PKGBUILD Agent 的 API 与后台任务服务。
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aur_auto/internal/agent"
	"aur_auto/internal/api"
	"aur_auto/internal/config"
	"aur_auto/internal/publisher"
	"aur_auto/internal/store"
	"aur_auto/internal/worker"
)

// main 加载配置、启动 worker 和 HTTP 服务，并处理优雅退出。
func main() {
	configPath := flag.String("config", "config.yaml", "YAML 配置文件路径")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("配置错误: %v", err)
	}
	taskStore, err := store.New(cfg.Storage.Directory)
	if err != nil {
		log.Fatalf("存储初始化失败: %v", err)
	}
	taskAgent, err := agent.New(cfg.LLM, cfg.HTTP, cfg.Docker, taskStore)
	if err != nil {
		log.Fatalf("Agent 初始化失败: %v", err)
	}
	taskPublisher := publisher.New(cfg.AUR, taskStore)
	manager := worker.New(taskStore, taskAgent, taskPublisher, cfg.AUR, cfg.Worker.Concurrency)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := manager.Start(ctx); err != nil {
		log.Fatalf("任务恢复失败: %v", err)
	}
	server := &http.Server{
		Addr: cfg.Server.Address, Handler: api.New(taskStore, manager, cfg),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 0, IdleTimeout: 90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("AUR Agent API 正在监听 %s", cfg.Server.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP 服务失败: %v", err)
	}
	manager.Stop()
}
