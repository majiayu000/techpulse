package progress

import (
	"bytes"
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
