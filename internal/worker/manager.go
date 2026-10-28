// Package worker 管理有界任务队列、并发执行、取消和自动发布。
package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/store"
)

// Runner 是后台 worker 所需的 Agent 执行接口。
type Runner interface {
	Run(ctx context.Context, taskID string) error
}

// Publisher 是后台 worker 所需的软件包发布接口。
type Publisher interface {
	Publish(ctx context.Context, taskID string) error
}

// Stats 描述 worker 池的并行容量和当前负载。
type Stats struct {
	Concurrency int `json:"concurrency"`
	Running     int `json:"running"`
	Queued      int `json:"queued"`
}

// Manager 将持久化任务分配给固定数量的后台 worker。
type Manager struct {
	store       *store.Store
	agent       Runner
	publisher   Publisher
	aur         config.AURConfig
	queue       chan string
	root        context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	cancels     map[string]context.CancelFunc
	running     int
	concurrency int
	wg          sync.WaitGroup
}

// New 创建任务管理器和有界队列。
func New(taskStore *store.Store, taskAgent Runner, taskPublisher Publisher, aurConfig config.AURConfig, concurrency int) *Manager {
	return &Manager{
		store: taskStore, agent: taskAgent, publisher: taskPublisher, aur: aurConfig,
		queue: make(chan string, concurrency*16), cancels: make(map[string]context.CancelFunc), concurrency: concurrency,
	}
}

// Start 恢复未完成任务并启动固定数量的后台 worker。
func (m *Manager) Start(parent context.Context) error {
	m.root, m.cancel = context.WithCancel(parent)
	tasks, err := m.store.List()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Terminal() || task.Status == domain.StatusPublishing {
			continue
		}
		if _, err := m.store.Update(task.ID, func(current *domain.Task) error {
			current.Status = domain.StatusQueued
			return nil
		}); err != nil {
			return err
		}
		m.Submit(task.ID)
	}
	concurrency := cap(m.queue) / 16
	for index := 0; index < concurrency; index++ {
		m.wg.Add(1)
		go m.runWorker()
	}
	return nil
}

// Submit 为新任务注册取消函数并放入执行队列。
func (m *Manager) Submit(taskID string) {
	m.mu.Lock()
	if _, exists := m.cancels[taskID]; !exists && m.root != nil {
		_, cancel := context.WithCancel(m.root)
		m.cancels[taskID] = cancel
	}
	m.mu.Unlock()
	m.queue <- taskID
}

// Cancel 取消排队或正在执行的任务，并立即反映到持久化状态。
func (m *Manager) Cancel(taskID string) error {
	m.mu.Lock()
	cancel := m.cancels[taskID]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	_, err := m.store.Update(taskID, func(task *domain.Task) error {
		if task.Terminal() {
			return fmt.Errorf("任务已经结束")
		}
		task.Status = domain.StatusCanceled
		now := time.Now().UTC()
		task.FinishedAt = &now
		return nil
	})
	return err
}

// Publish 为构建成功的任务开启发布请求，并在后台执行 Git 推送。
func (m *Manager) Publish(taskID string) error {
	if !m.aur.Enabled {
		return fmt.Errorf("服务端尚未启用 AUR 发布")
	}
	task, err := m.store.Get(taskID)
	if err != nil {
		return err
	}
	if task.Status != domain.StatusSucceeded {
		return fmt.Errorf("任务当前状态不能发布")
	}
	if _, err := m.store.Update(taskID, func(task *domain.Task) error {
		task.PublishRequested = true
		return nil
	}); err != nil {
		return err
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.publisher.Publish(m.root, taskID); err != nil {
			_ = m.store.AppendLog(taskID, "error", err.Error())
		}
	}()
	return nil
}

// Stats 返回并发上限、正在执行和排队等待的任务数。
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Stats{Concurrency: m.concurrency, Running: m.running, Queued: len(m.queue)}
}

// Retry 将失败或取消的任务重新放回队列，并保留历史日志。
func (m *Manager) Retry(taskID string) error {
	if err := m.waitIdle(taskID); err != nil {
		return err
	}
	task, err := m.store.Get(taskID)
	if err != nil {
		return err
	}
	if task.Status != domain.StatusFailed && task.Status != domain.StatusCanceled {
		return fmt.Errorf("只有失败或取消的任务可以重试")
	}
	if _, err := m.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusQueued
		task.Error = ""
		task.FinishedAt = nil
		task.Logs = append(task.Logs, domain.LogEntry{Time: time.Now().UTC(), Level: "info", Message: "用户请求重新执行任务"})
		return nil
	}); err != nil {
		return err
	}
	m.Submit(taskID)
	return nil
}

// Delete 删除已经结束的任务及其专用工作目录。
func (m *Manager) Delete(taskID string) error {
	if err := m.waitIdle(taskID); err != nil {
		return err
	}
	task, err := m.store.Get(taskID)
	if err != nil {
		return err
	}
	if !task.Terminal() {
		return fmt.Errorf("运行中的任务不能删除，请先取消")
	}
	return m.store.Delete(taskID)
}

// waitIdle 等待旧执行协程清理取消函数，防止取消后立即重试或删除发生竞态。
func (m *Manager) waitIdle(taskID string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		_, running := m.cancels[taskID]
		m.mu.Unlock()
		if !running {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待旧任务退出超时")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Stop 停止接收新工作，取消运行任务并等待 worker 退出。
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// runWorker 持续消费队列，并保证每个任务的取消资源被释放。
func (m *Manager) runWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.root.Done():
			return
		case taskID := <-m.queue:
			m.execute(taskID)
		}
	}
}

// execute 执行单个任务，统一处理跳过、失败、取消和自动发布状态。
func (m *Manager) execute(taskID string) {
	m.mu.Lock()
	cancel := m.cancels[taskID]
	m.mu.Unlock()
	if cancel == nil {
		return
	}
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.cancels, taskID)
		m.mu.Unlock()
	}()
	task, err := m.store.Get(taskID)
	if err != nil || task.Status != domain.StatusQueued {
		return
	}
	m.mu.Lock()
	m.running++
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.running--
		m.mu.Unlock()
	}()
	if _, err := m.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusDiscovering
		task.Error = ""
		task.FinishedAt = nil
		return nil
	}); err != nil {
		return
	}
	ctx, taskCancel := context.WithCancel(m.root)
	m.mu.Lock()
	m.cancels[taskID] = taskCancel
	m.mu.Unlock()
	defer taskCancel()
	if err := m.agent.Run(ctx, taskID); err != nil {
		status := domain.StatusFailed
		if errors.Is(err, context.Canceled) {
			status = domain.StatusCanceled
		}
		_, _ = m.store.Update(taskID, func(task *domain.Task) error {
			task.Status = status
			task.Error = err.Error()
			now := time.Now().UTC()
			task.FinishedAt = &now
			return nil
		})
		_ = m.store.AppendLog(taskID, "error", err.Error())
		return
	}
	if task.PublishRequested && m.aur.Enabled {
		if err := m.publisher.Publish(ctx, taskID); err != nil {
			_ = m.store.AppendLog(taskID, "error", err.Error())
		}
	}
}
