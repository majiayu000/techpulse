package worker

import (
	"io"
	"slices"
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

func containsFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

func TestBuildClaudeArgs_DefaultOmitsSkipPermissions(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "")

	r := &Runner{}
	args := r.buildClaudeArgs("test prompt")

	if containsFlag(args, claudeSkipPermissionsFlag) {
		t.Fatalf("default args must omit %s; got %v", claudeSkipPermissionsFlag, args)
	}
	if !slices.Equal(args, []string{"-p", "test prompt", "--output-format", "json"}) {
		t.Fatalf("unexpected default args: %v", args)
	}
}

func TestBuildClaudeArgs_OptInViaField(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "")

	r := &Runner{SkipPermissions: true}
	args := r.buildClaudeArgs("test prompt")

	if !containsFlag(args, claudeSkipPermissionsFlag) {
		t.Fatalf("field opt-in must include %s; got %v", claudeSkipPermissionsFlag, args)
	}
	want := []string{"-p", "test prompt", claudeSkipPermissionsFlag, "--output-format", "json"}
	if !slices.Equal(args, want) {
		t.Fatalf("unexpected opt-in args:\n got %v\nwant %v", args, want)
	}
}

func TestBuildClaudeArgs_OptInViaTechpulseEnv(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "true")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "")

	r := &Runner{}
	args := r.buildClaudeArgs("hello")

	if !containsFlag(args, claudeSkipPermissionsFlag) {
		t.Fatalf("%s=true must include %s; got %v", EnvClaudeSkipPermissions, claudeSkipPermissionsFlag, args)
	}
}

func TestBuildClaudeArgs_OptInViaAutonomousRunnerEnv(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "1")

	r := &Runner{}
	args := r.buildClaudeArgs("hello")

	if !containsFlag(args, claudeSkipPermissionsFlag) {
		t.Fatalf("%s=1 must include %s; got %v", EnvAutonomousRunnerSkipPermissions, claudeSkipPermissionsFlag, args)
	}
}

func TestBuildClaudeArgs_EnvFalsyDoesNotEnable(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "false")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "0")

	r := &Runner{}
	args := r.buildClaudeArgs("hello")

	if containsFlag(args, claudeSkipPermissionsFlag) {
		t.Fatalf("falsy env must omit %s; got %v", claudeSkipPermissionsFlag, args)
	}
}

func TestSetSkipPermissions(t *testing.T) {
	t.Setenv(EnvClaudeSkipPermissions, "")
	t.Setenv(EnvAutonomousRunnerSkipPermissions, "")

	r := NewRunner(nil, ".", 0)
	if r.SkipPermissions {
		t.Fatal("NewRunner must default SkipPermissions to false")
	}

	r.SetSkipPermissions(true)
	if !containsFlag(r.buildClaudeArgs("p"), claudeSkipPermissionsFlag) {
		t.Fatal("SetSkipPermissions(true) must enable the flag")
	}

	r.SetSkipPermissions(false)
	if containsFlag(r.buildClaudeArgs("p"), claudeSkipPermissionsFlag) {
		t.Fatal("SetSkipPermissions(false) must disable the flag")
	}
}
