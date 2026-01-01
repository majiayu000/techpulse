// Package memory 管理外部记忆文件
package memory

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Manager 记忆文件管理器
type Manager struct {
	dir string
}

// NewManager 创建记忆管理器
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建记忆目录失败: %w", err)
	}

	m := &Manager{dir: dir}
	if err := m.initFiles(); err != nil {
		return nil, err
	}

	return m, nil
}

// initFiles 初始化记忆文件（如果不存在）
func (m *Manager) initFiles() error {
	now := time.Now().Format("2006-01-02 15:04")

	files := map[string]string{
		"TASKS.md": fmt.Sprintf(`# 待办任务

## 高优先级
- [ ] 添加你的第一个任务

## 中优先级

## 低优先级

---
*创建时间: %s*
`, now),
		"CONTEXT.md": fmt.Sprintf(`# 项目上下文

## 项目概述
描述你的项目...

## 最近完成的工作
（暂无）

## 当前阻塞问题
（暂无）

## 下一步建议
查看 TASKS.md 中的任务列表

---
*最后更新: %s*
`, now),
		"DONE.md": `# 完成历史

| 时间 | 任务 | 备注 |
|------|------|------|

`,
	}

	for name, content := range files {
		path := filepath.Join(m.dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("创建 %s 失败: %w", name, err)
			}
		}
	}

	return nil
}

// ReadTasks 读取任务文件
func (m *Manager) ReadTasks() (string, error) {
	return m.readFile("TASKS.md")
}

// ReadContext 读取上下文文件
func (m *Manager) ReadContext() (string, error) {
	return m.readFile("CONTEXT.md")
}

// ReadDone 读取完成历史
func (m *Manager) ReadDone() (string, error) {
	return m.readFile("DONE.md")
}

func (m *Manager) readFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(m.dir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ContentHash 获取记忆文件内容哈希
func (m *Manager) ContentHash() (string, error) {
	tasks, err := m.ReadTasks()
	if err != nil {
		return "", err
	}
	context, err := m.ReadContext()
	if err != nil {
		return "", err
	}

	hash := md5.Sum([]byte(tasks + context))
	return hex.EncodeToString(hash[:])[:8], nil
}

// HasPendingTasks 检查是否有待办任务
func (m *Manager) HasPendingTasks() (bool, error) {
	tasks, err := m.ReadTasks()
	if err != nil {
		return false, err
	}
	return strings.Contains(tasks, "- [ ]"), nil
}

// CountCompletedTasks 统计已完成任务数
func (m *Manager) CountCompletedTasks() (int, error) {
	done, err := m.ReadDone()
	if err != nil {
		return 0, err
	}

	// 统计表格行数（排除表头）
	lines := strings.Split(done, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|") &&
			!strings.Contains(line, "时间") &&
			!strings.Contains(line, "---") {
			count++
		}
	}
	return count, nil
}

// Dir 返回记忆目录路径
func (m *Manager) Dir() string {
	return m.dir
}

// AbsDir 返回记忆目录绝对路径
func (m *Manager) AbsDir() (string, error) {
	return filepath.Abs(m.dir)
}

// ContextSize 返回上下文文件行数
func (m *Manager) ContextSize() (int, error) {
	context, err := m.ReadContext()
	if err != nil {
		return 0, err
	}
	return len(strings.Split(context, "\n")), nil
}

// ExtractPendingTasks 提取待办任务列表
func (m *Manager) ExtractPendingTasks() ([]string, error) {
	tasks, err := m.ReadTasks()
	if err != nil {
		return nil, err
	}

	var pending []string
	re := regexp.MustCompile(`- \[ \] (.+)`)
	matches := re.FindAllStringSubmatch(tasks, -1)
	for _, match := range matches {
		if len(match) > 1 {
			pending = append(pending, match[1])
		}
	}
	return pending, nil
}
