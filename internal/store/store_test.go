package store

import (
	"errors"
	"os"
	"testing"

	"aur_auto/internal/domain"
)

// TestTaskLifecycle 验证创建、更新、日志、工作目录和删除的完整持久化流程。
func TestTaskLifecycle(t *testing.T) {
	storage, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := storage.Create("https://github.com/owner/project", "project", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(task.ID, func(current *domain.Task) error {
		current.Status = domain.StatusSucceeded
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.AppendLog(task.ID, "info", "测试日志"); err != nil {
		t.Fatal(err)
	}
	workspace, err := storage.Workspace(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := storage.Get(task.ID)
	if err != nil || loaded.Status != domain.StatusSucceeded || len(loaded.Logs) != 1 {
		t.Fatalf("任务快照不正确: %#v %v", loaded, err)
	}
	tasks, err := storage.List()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("任务列表不正确: %d %v", len(tasks), err)
	}
	if err := storage.Delete(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get(task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后仍能读取任务: %v", err)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatal("工作目录未被清理")
	}
}

// TestInvalidID 验证路径穿越和非法 ID 无法访问存储。
func TestInvalidID(t *testing.T) {
	storage, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get("../../secret"); !errors.Is(err, ErrNotFound) {
		t.Fatal("非法任务 ID 未被拒绝")
	}
}
