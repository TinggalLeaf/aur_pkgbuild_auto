package publisher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"aur_auto/internal/config"
	"aur_auto/internal/domain"
	"aur_auto/internal/store"
)

// TestLocalRepositoryPublish 验证自建本地裸仓库的完整提交、推送和结果记录。
func TestLocalRepositoryPublish(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "jo.git")
	if output, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("创建裸仓库: %s %v", output, err)
	}
	storage, _ := store.New(filepath.Join(root, "data"))
	task, _ := storage.Create("https://github.com/jpmens/jo", "jo", true)
	_, _ = storage.Update(task.ID, func(current *domain.Task) error {
		current.Status = domain.StatusSucceeded
		current.Version = "1.9"
		current.PKGBUILD = "pkgname=jo\npkgver=1.9\n"
		current.SRCINFO = "pkgbase = jo\n\tpkgver = 1.9\n"
		return nil
	})
	publisher := New(config.AURConfig{
		Enabled: true, RepositoryURL: remote, WebURL: "http://localhost:8000",
		Branch: "master", GitName: "Test", GitEmail: "test@localhost",
	}, storage)
	if err := publisher.Publish(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	updated, _ := storage.Get(task.ID)
	if updated.Status != domain.StatusPublished || updated.Publication == nil || updated.Publication.Commit == "" {
		t.Fatalf("发布结果不完整: %#v", updated)
	}
	output, err := exec.Command("git", "--git-dir="+remote, "show", "HEAD:PKGBUILD").Output()
	if err != nil || string(output) != "pkgname=jo\npkgver=1.9\n" {
		t.Fatalf("远端 PKGBUILD 不正确: %s %v", output, err)
	}
	if updated.Publication.PushedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("发布时间无效")
	}
	if _, err := os.Stat(filepath.Join(root, "data", "workspaces", task.ID, "aur-repository", ".packpilot-askpass.sh")); !os.IsNotExist(err) {
		t.Fatal("临时凭据助手未清理")
	}
}
