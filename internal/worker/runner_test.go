package worker

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamOutput_ConcurrentSafeCollection(t *testing.T) {
	r := &Runner{}
	var (
		stdoutBuf strings.Builder
		stderrBuf strings.Builder
		wg        sync.WaitGroup
	)

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	wg.Add(2)
	go func() {
		defer wg.Done()
		r.streamOutput(stdoutR, &stdoutBuf)
	}()
	go func() {
		defer wg.Done()
		r.streamOutput(stderrR, &stderrBuf)
	}()

	go func() {
		defer stdoutW.Close()
		for i := 0; i < 50; i++ {
			io.WriteString(stdoutW, `{"total_cost_usd":1.25}`+"\n")
		}
	}()
	go func() {
		defer stderrW.Close()
		for i := 0; i < 50; i++ {
			io.WriteString(stderrW, "diag line\n")
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream readers to finish")
	}

	out := stdoutBuf.String() + stderrBuf.String()
	if !strings.Contains(out, `"total_cost_usd":1.25`) {
		t.Fatalf("expected stdout JSON preserved, got %q", out)
	}
	if !strings.Contains(out, "diag line") {
		t.Fatalf("expected stderr lines preserved, got %q", out)
	}
}

func TestStreamOutput_LargeJSONLineBeyondScannerDefault(t *testing.T) {
	r := &Runner{}
	var buf strings.Builder
	pr, pw := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.streamOutput(pr, &buf)
	}()

	// Exceed bufio.Scanner's default 64 KiB token limit.
	payload := `{"total_cost_usd":2.5,"blob":"` + strings.Repeat("x", 70*1024) + `"}` + "\n"
	go func() {
		defer pw.Close()
		io.WriteString(pw, payload)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for large-line reader")
	}

	out := buf.String()
	if !strings.Contains(out, `"total_cost_usd":2.5`) {
		t.Fatalf("expected large JSON line preserved, got len=%d", len(out))
	}
	if strings.Contains(out, "[streamOutput error:") {
		t.Fatalf("unexpected stream error for large line: %q", out)
	}
}
