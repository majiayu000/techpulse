package progress

import (
	"bytes"
	"strings"
	"testing"
)

// TestBarRenderZeroTotal verifies that rendering with total==0 (or negative)
// no longer divides by zero: no NaN output, no strings.Repeat panic,
// and the bar is always exactly b.width cells wide.
func TestBarRenderZeroTotal(t *testing.T) {
	tests := []struct {
		name   string
		total  int
		action func(*Bar)
		want   string
	}{
		{
			name:  "start with zero total",
			total: 0,
			action: func(b *Bar) {
				b.Start()
			},
			want: "(0/0)",
		},
		{
			name:  "complete with zero total",
			total: 0,
			action: func(b *Bar) {
				b.CompleteTask("t1", "")
			},
			want: "(1/0)",
		},
		{
			name:  "negative total",
			total: -1,
			// Negative totals render like zero: no NaN, no panic.
			action: func(b *Bar) {
				b.Start()
			},
			want: "(0/-1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			bar := New(tt.total)
			bar.SetOutput(&buf)
			tt.action(bar)

			out := buf.String()
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got %q", tt.want, out)
			}
			if strings.Contains(out, "NaN") {
				t.Errorf("expected no NaN in output, got %q", out)
			}
			idx := strings.LastIndex(out, "[")
			end := strings.LastIndex(out, "]")
			if idx < 0 || end < idx {
				t.Fatalf("expected a rendered bar in output, got %q", out)
			}
			cells := []rune(out[idx+1 : end])
			if len(cells) != bar.width {
				t.Errorf("expected bar width %d cells, got %d (%q)", bar.width, len(cells), string(cells))
			}
			for _, c := range cells {
				if c != '█' && c != '░' {
					t.Errorf("unexpected bar cell %q in %q", c, out)
				}
			}
		})
	}
}
