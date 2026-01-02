package progress

import (
	"errors"
	"testing"
)

// mockDetector is a test detector
type mockDetector struct {
	name       string
	hasProgress bool
	err        error
	details    string
}

func (m *mockDetector) Name() string         { return m.name }
func (m *mockDetector) Detect() (bool, error) { return m.hasProgress, m.err }
func (m *mockDetector) Reset() error         { return m.err }
func (m *mockDetector) Details() string      { return m.details }

func TestNewMultiDetector(t *testing.T) {
	d1 := &mockDetector{name: "d1"}
	d2 := &mockDetector{name: "d2"}

	md := NewMultiDetector(d1, d2)
	if md == nil {
		t.Fatal("expected non-nil MultiDetector")
	}
	if len(md.detectors) != 2 {
		t.Errorf("expected 2 detectors, got %d", len(md.detectors))
	}
}

func TestMultiDetector_DetectNoProgress(t *testing.T) {
	d1 := &mockDetector{name: "d1", hasProgress: false}
	d2 := &mockDetector{name: "d2", hasProgress: false}

	md := NewMultiDetector(d1, d2)
	result, err := md.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasProgress {
		t.Error("expected no progress")
	}
	if result.Source != "none" {
		t.Errorf("expected source 'none', got %q", result.Source)
	}
}

func TestMultiDetector_DetectWithProgress(t *testing.T) {
	d1 := &mockDetector{name: "d1", hasProgress: false}
	d2 := &mockDetector{name: "d2", hasProgress: true, details: "some changes"}

	md := NewMultiDetector(d1, d2)
	result, err := md.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasProgress {
		t.Error("expected progress")
	}
	if result.Source != "d2" {
		t.Errorf("expected source 'd2', got %q", result.Source)
	}
	if result.Details != "some changes" {
		t.Errorf("expected details 'some changes', got %q", result.Details)
	}
}

func TestMultiDetector_DetectFirstProgress(t *testing.T) {
	d1 := &mockDetector{name: "d1", hasProgress: true, details: "first"}
	d2 := &mockDetector{name: "d2", hasProgress: true, details: "second"}

	md := NewMultiDetector(d1, d2)
	result, err := md.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "d1" {
		t.Errorf("expected source 'd1' (first), got %q", result.Source)
	}
}

func TestMultiDetector_DetectSkipsErrors(t *testing.T) {
	d1 := &mockDetector{name: "d1", err: errors.New("fail")}
	d2 := &mockDetector{name: "d2", hasProgress: true, details: "ok"}

	md := NewMultiDetector(d1, d2)
	result, err := md.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasProgress {
		t.Error("expected progress despite d1 error")
	}
	if result.Source != "d2" {
		t.Errorf("expected source 'd2', got %q", result.Source)
	}
}

func TestMultiDetector_ResetSuccess(t *testing.T) {
	d1 := &mockDetector{name: "d1"}
	d2 := &mockDetector{name: "d2"}

	md := NewMultiDetector(d1, d2)
	err := md.Reset()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultiDetector_ResetError(t *testing.T) {
	d1 := &mockDetector{name: "d1"}
	d2 := &mockDetector{name: "d2", err: errors.New("reset fail")}

	md := NewMultiDetector(d1, d2)
	err := md.Reset()
	if err == nil {
		t.Error("expected error from reset")
	}
}

func TestMultiDetector_Empty(t *testing.T) {
	md := NewMultiDetector()
	result, err := md.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasProgress {
		t.Error("expected no progress with empty detector list")
	}
}
