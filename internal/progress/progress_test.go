package progress

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestNewBar(t *testing.T) {
	bar := New(10)
	if bar.total != 10 {
		t.Errorf("expected total 10, got %d", bar.total)
	}
	if bar.current != 0 {
		t.Errorf("expected current 0, got %d", bar.current)
	}
}

func TestBarDisabled(t *testing.T) {
	var buf bytes.Buffer
	bar := New(2)
	bar.SetOutput(&buf)
	bar.SetDisabled(true)

	bar.Start()
	bar.StartTask("task1")
	bar.CompleteTask("task1", "done")
	bar.Finish()

	if buf.Len() != 0 {
		t.Errorf("expected no output when disabled, got %q", buf.String())
	}
}

func TestBarProgress(t *testing.T) {
	var buf bytes.Buffer
	bar := New(2)
	bar.SetOutput(&buf)

	bar.Start()
	bar.CompleteTask("task1", "5 articles")
	bar.CompleteTask("task2", "3 articles")
	bar.Finish()

	output := buf.String()
	if !strings.Contains(output, "50%") {
		t.Errorf("expected 50%% in output")
	}
	if !strings.Contains(output, "100%") {
		t.Errorf("expected 100%% in output")
	}
	if !strings.Contains(output, "Completed 2 tasks") {
		t.Errorf("expected completion message in output, got %q", output)
	}
}

func TestBarFailTask(t *testing.T) {
	var buf bytes.Buffer
	bar := New(1)
	bar.SetOutput(&buf)

	bar.Start()
	bar.FailTask("task1", "connection error")
	bar.Finish()

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error icon in output")
	}
	if !strings.Contains(output, "connection error") {
		t.Errorf("expected error message in output")
	}
}

func TestBarTaskTracking(t *testing.T) {
	bar := New(3)
	bar.SetDisabled(true) // No output, just track

	bar.StartTask("task1")
	if bar.tasks["task1"].Status != "running" {
		t.Errorf("expected task1 to be running")
	}

	bar.CompleteTask("task1", "")
	if bar.tasks["task1"].Status != "done" {
		t.Errorf("expected task1 to be done")
	}
	if bar.current != 1 {
		t.Errorf("expected current to be 1, got %d", bar.current)
	}
}

func TestBarConcurrentUpdates(t *testing.T) {
	bar := New(100)
	bar.SetDisabled(true) // Suppress output

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(i int) {
			bar.CompleteTask("task", "")
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	if bar.current != 100 {
		t.Errorf("expected current 100, got %d", bar.current)
	}
}

// TestBarFinishSummary verifies the Finish summary distinguishes succeeded
// from failed tasks instead of counting every finished task as completed.
func TestBarFinishSummary(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		actions     []string // "ok" or "fail"
		wantSubs    []string
		wantMissing []string
	}{
		{
			name:        "all tasks succeeded",
			total:       2,
			actions:     []string{"ok", "ok"},
			wantSubs:    []string{"✓ Completed 2 tasks"},
			wantMissing: []string{"failed"},
		},
		{
			name:        "mixed success and failure",
			total:       3,
			actions:     []string{"ok", "fail", "fail"},
			wantSubs:    []string{"⚠ Finished 3/3", "1 succeeded", "2 failed"},
			wantMissing: []string{"✓ Completed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			bar := New(tt.total)
			bar.SetOutput(&buf)
			bar.Start()

			for i, a := range tt.actions {
				name := fmt.Sprintf("task%d", i+1)
				if a == "ok" {
					bar.CompleteTask(name, "")
				} else {
					bar.FailTask(name, "boom")
				}
			}
			bar.Finish()

			out := buf.String()
			for _, sub := range tt.wantSubs {
				if !strings.Contains(out, sub) {
					t.Errorf("expected output to contain %q, got %q", sub, out)
				}
			}
			for _, sub := range tt.wantMissing {
				if strings.Contains(out, sub) {
					t.Errorf("expected output NOT to contain %q, got %q", sub, out)
				}
			}
		})
	}
}
