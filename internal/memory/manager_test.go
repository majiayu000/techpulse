package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// 检查文件是否创建
	files := []string{"TASKS.md", "CONTEXT.md", "DONE.md"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", f)
		}
	}

	// 检查目录路径
	if m.Dir() != dir {
		t.Errorf("Dir() = %s, want %s", m.Dir(), dir)
	}
}

func TestHasPendingTasks(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	// 默认应该有待办任务
	has, err := m.HasPendingTasks()
	if err != nil {
		t.Fatalf("HasPendingTasks failed: %v", err)
	}
	if !has {
		t.Error("Expected pending tasks in default TASKS.md")
	}

	// 写入没有待办任务的内容
	os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte("# Tasks\n- [x] Done"), 0644)
	has, _ = m.HasPendingTasks()
	if has {
		t.Error("Expected no pending tasks")
	}

	// 写入有待办任务的内容
	os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte("# Tasks\n- [ ] Todo\n- [x] Done"), 0644)
	has, _ = m.HasPendingTasks()
	if !has {
		t.Error("Expected pending tasks")
	}
}

func TestContentHash(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	hash1, err := m.ContentHash()
	if err != nil {
		t.Fatalf("ContentHash failed: %v", err)
	}
	if len(hash1) != 8 {
		t.Errorf("Expected 8 char hash, got %d", len(hash1))
	}

	// 修改内容后哈希应该改变
	os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte("Changed content"), 0644)
	hash2, _ := m.ContentHash()
	if hash1 == hash2 {
		t.Error("Expected hash to change after content modification")
	}
}

func TestCountCompletedTasks(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	// 写入完成记录
	done := `# 完成历史

| 时间 | 任务 | 备注 |
|------|------|------|
| 2024-01-01 | Task 1 | Done |
| 2024-01-02 | Task 2 | Done |
| 2024-01-03 | Task 3 | Done |
`
	os.WriteFile(filepath.Join(dir, "DONE.md"), []byte(done), 0644)

	count, err := m.CountCompletedTasks()
	if err != nil {
		t.Fatalf("CountCompletedTasks failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", count)
	}
}

func TestExtractPendingTasks(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	tasks := `# Tasks

## High Priority
- [ ] First task
- [x] Completed task
- [ ] Second task

## Low Priority
- [ ] Third task
`
	os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(tasks), 0644)

	pending, err := m.ExtractPendingTasks()
	if err != nil {
		t.Fatalf("ExtractPendingTasks failed: %v", err)
	}

	expected := []string{"First task", "Second task", "Third task"}
	if len(pending) != len(expected) {
		t.Fatalf("Expected %d tasks, got %d", len(expected), len(pending))
	}

	for i, task := range pending {
		if task != expected[i] {
			t.Errorf("Task %d: got %q, want %q", i, task, expected[i])
		}
	}
}
