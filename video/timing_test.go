package video

import (
	"testing"
	"time"
)

func TestRenderBackoff(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{1, 50 * time.Millisecond},
		{2, 100 * time.Millisecond},
		{3, 200 * time.Millisecond},
		{4, 400 * time.Millisecond},
		{5, 800 * time.Millisecond},
		{6, 1600 * time.Millisecond},
		{7, 2 * time.Second},
		{8, 2 * time.Second},
	}

	for _, tc := range tests {
		if got := renderBackoff(tc.failures); got != tc.want {
			t.Errorf("renderBackoff(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}
