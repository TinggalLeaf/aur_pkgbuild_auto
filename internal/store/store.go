// Package store 提供基于原子文件替换的并发安全任务存储。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aur_auto/internal/domain"
)

// ErrNotFound 表示请求的任务 ID 不存在。
var ErrNotFound = errors.New("任务不存在")

// Store 将每个任务保存为独立 JSON 文件，并用进程内锁保护更新。
type Store struct {
	directory string
	mu        sync.RWMutex
}

// New 创建任务目录并返回文件存储实例。
func New(directory string) (*Store, error) {
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("解析存储目录: %w", err)
	}
	tasks := filepath.Join(absoluteDirectory, "tasks")
	if err := os.MkdirAll(tasks, 0o750); err != nil {
		return nil, fmt.Errorf("创建任务目录: %w", err)
	}
	return &Store{directory: tasks}, nil
}

// Create 为来源 URL 创建排队任务并持久化。
func (s *Store) Create(sourceURL, packageName string, publish bool) (*domain.Task, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	task := &domain.Task{
		ID: id, SourceURL: sourceURL, PackageName: packageName,
		Status: domain.StatusQueued, PublishRequested: publish,
		Logs: []domain.LogEntry{}, CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeLocked(task); err != nil {
		return nil, err
	}
	return task, nil
}

// Get 返回任务快照，调用者可以安全修改而不会影响存储中的值。
func (s *Store) Get(id string) (*domain.Task, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readLocked(id)
}

// List 按创建时间倒序返回全部任务快照。
func (s *Store) List() ([]*domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return nil, fmt.Errorf("读取任务列表: %w", err)
	}
	tasks := make([]*domain.Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		task, err := s.readLocked(id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.After(tasks[j].CreatedAt) })
	return tasks, nil
}

// Update 在写锁内读取、修改并原子保存同一个任务。
func (s *Store) Update(id string, mutate func(*domain.Task) error) (*domain.Task, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	if err := mutate(task); err != nil {
		return nil, err
	}
	task.UpdatedAt = time.Now().UTC()
	if len(task.Logs) > 3000 {
		task.Logs = task.Logs[len(task.Logs)-3000:]
	}
	if err := s.writeLocked(task); err != nil {
		return nil, err
	}
	return task, nil
}

// AppendLog 向任务追加一条带 UTC 时间的执行日志。
func (s *Store) AppendLog(id, level, message string) error {
	_, err := s.Update(id, func(task *domain.Task) error {
		task.Logs = append(task.Logs, domain.LogEntry{Time: time.Now().UTC(), Level: level, Message: message})
		return nil
	})
	return err
}

// Delete 删除任务 JSON 和对应的专用工作目录，不影响其他任务。
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readLocked(id); err != nil {
		return err
	}
	if err := os.Remove(s.path(id)); err != nil {
		return fmt.Errorf("删除任务: %w", err)
	}
	workspace := filepath.Join(filepath.Dir(s.directory), "workspaces", id)
	if err := os.RemoveAll(workspace); err != nil {
		return fmt.Errorf("清理工作目录: %w", err)
	}
	return nil
}

// Workspace 创建并返回任务专用的构建工作目录。
func (s *Store) Workspace(id string) (string, error) {
	if !validID(id) {
		return "", ErrNotFound
	}
	path := filepath.Join(filepath.Dir(s.directory), "workspaces", id)
	if err := os.MkdirAll(path, 0o750); err != nil {
		return "", fmt.Errorf("创建工作目录: %w", err)
	}
	return path, nil
}

// readLocked 在调用者持锁的前提下读取一个任务文件。
func (s *Store) readLocked(id string) (*domain.Task, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取任务: %w", err)
	}
	var task domain.Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("解析任务 %s: %w", id, err)
	}
	return &task, nil
}

// writeLocked 在调用者持锁的前提下以同目录临时文件原子替换任务。
func (s *Store) writeLocked(task *domain.Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("编码任务: %w", err)
	}
	temporary, err := os.CreateTemp(s.directory, ".task-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时任务文件: %w", err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("写入任务: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("同步任务: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path(task.ID)); err != nil {
		return fmt.Errorf("保存任务: %w", err)
	}
	return nil
}

// path 返回经过 ID 校验后的任务文件路径。
func (s *Store) path(id string) string {
	return filepath.Join(s.directory, id+".json")
}

// newID 生成适合 URL 和文件名的随机任务标识。
func newID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成任务 ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// validID 限制任务 ID 为固定长度的小写十六进制，防止路径穿越。
func validID(id string) bool {
	if len(id) != 16 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && id == strings.ToLower(id)
}
