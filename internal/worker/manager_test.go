package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/store"
)

// fakeRunner 用可控阻塞和失败模拟真实 Agent，便于验证调度而非模型输出。
type fakeRunner struct {
	store    *store.Store
	started  chan string
	release  chan struct{}
	failNext atomic.Bool
	mu       sync.Mutex
	active   int
	maxSeen  int
}

// Run 记录并行峰值，并响应释放、取消和一次性失败。
func (r *fakeRunner) Run(ctx context.Context, taskID string) error {
	r.mu.Lock()
	r.active++
	if r.active > r.maxSeen {
		r.maxSeen = r.active
	}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.active--; r.mu.Unlock() }()
	r.started <- taskID
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if r.failNext.CompareAndSwap(true, false) {
		return errors.New("模拟构建失败")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := r.store.Update(taskID, func(task *domain.Task) error {
		task.Status = domain.StatusSucceeded
		return nil
	})
	return err
}

// fakePublisher 提供不访问外部仓库的测试发布器。
type fakePublisher struct{}

// Publish 在测试中始终模拟发布成功。
func (fakePublisher) Publish(context.Context, string) error { return nil }

// TestParallelLimit 验证两个 worker 能同时执行且第三个任务保持排队。
func TestParallelLimit(t *testing.T) {
	storage, _ := store.New(t.TempDir())
	runner := &fakeRunner{store: storage, started: make(chan string, 4), release: make(chan struct{})}
	manager := New(storage, runner, fakePublisher{}, config.AURConfig{}, 2)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	tasks := make([]*domain.Task, 3)
	for index := range tasks {
		task, _ := storage.Create("https://github.com/owner/project", "project", false)
		tasks[index] = task
		manager.Submit(task.ID)
	}
	<-runner.started
	<-runner.started
	stats := manager.Stats()
	if stats.Running != 2 || stats.Queued != 1 {
		t.Fatalf("并行状态不正确: %#v", stats)
	}
	close(runner.release)
	for _, task := range tasks {
		waitStatus(t, storage, task.ID, domain.StatusSucceeded)
	}
	runner.mu.Lock()
	maxSeen := runner.maxSeen
	runner.mu.Unlock()
	if maxSeen != 2 {
		t.Fatalf("实际并行峰值为 %d，期望 2", maxSeen)
	}
}

// TestCancelAndRetry 验证运行中取消以及取消后重试的生命周期。
func TestCancelAndRetry(t *testing.T) {
	storage, _ := store.New(t.TempDir())
	runner := &fakeRunner{store: storage, started: make(chan string, 2), release: make(chan struct{})}
	manager := New(storage, runner, fakePublisher{}, config.AURConfig{}, 1)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	task, _ := storage.Create("https://github.com/owner/project", "project", false)
	manager.Submit(task.ID)
	<-runner.started
	if err := manager.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, storage, task.ID, domain.StatusCanceled)
	close(runner.release)
	if err := manager.Retry(task.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, storage, task.ID, domain.StatusSucceeded)
}

// TestRetryFailureAndDelete 验证失败重试和终态删除约束。
func TestRetryFailureAndDelete(t *testing.T) {
	storage, _ := store.New(t.TempDir())
	runner := &fakeRunner{store: storage, started: make(chan string, 2)}
	runner.failNext.Store(true)
	manager := New(storage, runner, fakePublisher{}, config.AURConfig{}, 1)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	task, _ := storage.Create("https://github.com/owner/project", "project", false)
	manager.Submit(task.ID)
	waitStatus(t, storage, task.ID, domain.StatusFailed)
	if err := manager.Retry(task.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, storage, task.ID, domain.StatusSucceeded)
	if err := manager.Delete(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get(task.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("终态任务未被删除")
	}
}

// waitStatus 在有限时间内轮询任务状态，避免调度测试永久阻塞。
func waitStatus(t *testing.T, storage *store.Store, id string, expected domain.Status) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		task, err := storage.Get(id)
		if err == nil && task.Status == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, _ := storage.Get(id)
	t.Fatalf("等待状态 %s 超时，当前为 %s", expected, task.Status)
}
