// Package domain 定义 Agent、API 与前端共享的核心任务模型。
package domain

import "time"

// Status 表示一个 PKGBUILD 任务当前所处的生命周期阶段。
type Status string

const (
	StatusQueued      Status = "queued"
	StatusDiscovering Status = "discovering"
	StatusGenerating  Status = "generating"
	StatusBuilding    Status = "building"
	StatusSucceeded   Status = "succeeded"
	StatusFailed      Status = "failed"
	StatusCanceled    Status = "canceled"
	StatusPublishing  Status = "publishing"
	StatusPublished   Status = "published"
)

// LogEntry 保存一条可展示给用户的任务执行日志。
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Publication 保存一次自建 aurweb 仓库推送的结果。
type Publication struct {
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Commit     string    `json:"commit"`
	PackageURL string    `json:"package_url"`
	PushedAt   time.Time `json:"pushed_at"`
}

// Task 保存从源码发现到构建发布的全部可持久化状态。
type Task struct {
	ID               string       `json:"id"`
	SourceURL        string       `json:"source_url"`
	PackageName      string       `json:"package_name"`
	Version          string       `json:"version"`
	Description      string       `json:"description"`
	UpstreamURL      string       `json:"upstream_url"`
	SourceArchiveURL string       `json:"source_archive_url"`
	Status           Status       `json:"status"`
	PublishRequested bool         `json:"publish_requested"`
	Attempts         int          `json:"attempts"`
	PKGBUILD         string       `json:"pkgbuild"`
	SRCINFO          string       `json:"srcinfo"`
	BuildLog         string       `json:"build_log"`
	Error            string       `json:"error"`
	Logs             []LogEntry   `json:"logs"`
	Publication      *Publication `json:"publication,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	FinishedAt       *time.Time   `json:"finished_at,omitempty"`
}

// Terminal 判断任务是否已经进入不会自动继续执行的终态。
func (t *Task) Terminal() bool {
	switch t.Status {
	case StatusSucceeded, StatusFailed, StatusCanceled, StatusPublished:
		return true
	default:
		return false
	}
}
