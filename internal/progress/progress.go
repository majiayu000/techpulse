// Package progress provides terminal progress bar functionality.
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Bar represents a progress bar with concurrent update support.
type Bar struct {
	mu        sync.Mutex
	total     int
	current   int
	completed int
	failed    int
	width     int
	out       io.Writer
	disabled  bool
	tasks     map[string]TaskStatus
	startAt   time.Time
}

// TaskStatus represents the current status of a task.
type TaskStatus struct {
	Name   string
	Status string // "pending", "running", "done", "error"
}

// New creates a new progress bar.
func New(total int) *Bar {
	return &Bar{
		total:   total,
		width:   40,
		out:     os.Stdout,
		tasks:   make(map[string]TaskStatus),
		startAt: time.Now(),
	}
}

// SetOutput sets the output writer.
func (b *Bar) SetOutput(w io.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.out = w
}

// SetDisabled disables progress bar output.
func (b *Bar) SetDisabled(disabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disabled = disabled
}

// Start displays the initial progress bar.
func (b *Bar) Start() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled {
		return
	}
	b.render()
}

// StartTask marks a task as started.
func (b *Bar) StartTask(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tasks[name] = TaskStatus{Name: name, Status: "running"}
	if !b.disabled {
		b.renderTaskLine(name, "running", "")
	}
}

// CompleteTask marks a task as completed and increments progress.
func (b *Bar) CompleteTask(name string, info string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current++
	b.completed++
	b.tasks[name] = TaskStatus{Name: name, Status: "done"}
	if b.disabled {
		return
	}
	b.renderTaskLine(name, "done", info)
	b.render()
}

// FailTask marks a task as failed and increments progress.
func (b *Bar) FailTask(name string, errMsg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current++
	b.failed++
	b.tasks[name] = TaskStatus{Name: name, Status: "error"}
	if b.disabled {
		return
	}
	b.renderTaskLine(name, "error", errMsg)
	b.render()
}

// Finish completes the progress bar. The summary line distinguishes
// succeeded tasks from failed ones instead of counting every finished
// task as completed.
func (b *Bar) Finish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled {
		return
	}
	elapsed := time.Since(b.startAt)
	if b.failed > 0 {
		fmt.Fprintf(b.out, "\n⚠ Finished %d/%d tasks in %.1fs (%d succeeded, %d failed)\n",
			b.current, b.total, elapsed.Seconds(), b.completed, b.failed)
		return
	}
	fmt.Fprintf(b.out, "\n✓ Completed %d tasks in %.1fs\n", b.completed, elapsed.Seconds())
}
