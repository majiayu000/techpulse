package progress

import (
	"errors"
	"strings"
	"testing"
)

// mockDetector is a test detector
type mockDetector struct {
	name        string
	hasProgress bool
	err         error
	details     string
}

func (m *mockDetector) Name() string          { return m.name }
func (m *mockDetector) Detect() (bool, error) { return m.hasProgress, m.err }
func (m *mockDetector) Reset() error          { return m.err }
func (m *mockDetector) Details() string       { return m.details }

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

// TestMultiDetector_DetectErrors is table-driven coverage for Detect error
// handling: partial failures are recorded in Result.Failures, and an
// all-detectors failure is surfaced as a combined error (fail closed).
func TestMultiDetector_DetectErrors(t *testing.T) {
	errBoom := errors.New("boom")
	tests := []struct {
		name         string
		detectors    []*mockDetector
		wantErr      bool
		wantErrSubs  []string
		wantProgress bool
		wantFailures int
		wantSource   string
	}{
		{
			name:         "all detectors fail",
			detectors:    []*mockDetector{{name: "a", err: errBoom}, {name: "b", err: errBoom}},
			wantErr:      true,
			wantErrSubs:  []string{"a", "b", "boom"},
			wantProgress: false,
			wantFailures: 2,
			wantSource:   "none",
		},
		{
			name:         "partial failure with progress",
			detectors:    []*mockDetector{{name: "a", err: errBoom}, {name: "b", hasProgress: true, details: "ok"}},
			wantErr:      false,
			wantErrSubs:  nil,
			wantProgress: true,
			wantFailures: 1,
			wantSource:   "b",
		},
		{
			name:         "partial failure no progress",
			detectors:    []*mockDetector{{name: "a", err: errBoom}, {name: "b"}},
			wantErr:      false,
			wantErrSubs:  nil,
			wantProgress: false,
			wantFailures: 1,
			wantSource:   "none",
		},
		{
			name:         "no failures",
			detectors:    []*mockDetector{{name: "a"}, {name: "b"}},
			wantErr:      false,
			wantErrSubs:  nil,
			wantProgress: false,
			wantFailures: 0,
			wantSource:   "none",
		},
		{
			name:         "empty detector list",
			detectors:    nil,
			wantErr:      false,
			wantErrSubs:  nil,
			wantProgress: false,
			wantFailures: 0,
			wantSource:   "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dets := make([]Detector, len(tt.detectors))
			for i, d := range tt.detectors {
				dets[i] = d
			}
			md := NewMultiDetector(dets...)
			result, err := md.Detect()

			if tt.wantErr && err == nil {
				t.Fatal("expected combined error when all detectors fail")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.HasProgress != tt.wantProgress {
				t.Errorf("expected HasProgress %v, got %v", tt.wantProgress, result.HasProgress)
			}
			if result.Source != tt.wantSource {
				t.Errorf("expected source %q, got %q", tt.wantSource, result.Source)
			}
			if len(result.Failures) == tt.wantFailures {
				for _, f := range result.Failures {
					if f.Detector == "" || f.Err == nil {
						t.Errorf("expected failure entries to carry detector name and error, got %+v", f)
					}
					if !strings.Contains(f.Err.Error(), "boom") {
						t.Errorf("expected failure error to wrap underlying error, got %v", f.Err)
					}
				}
			} else {
				t.Errorf("expected %d failures, got %d (%v)", tt.wantFailures, len(result.Failures), result.Failures)
			}
			for _, sub := range tt.wantErrSubs {
				if err != nil && !strings.Contains(err.Error(), sub) {
					t.Errorf("expected combined error to contain %q, got %v", sub, err)
				}
			}
			if tt.wantErr {
				var de *DetectorError
				if !errors.As(err, &de) {
					t.Errorf("expected combined error to wrap *DetectorError, got %v", err)
				}
			}
		})
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
