package ui

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// ffprobe's pts_time for frame idx.
func frameStart(idx int, fps float64) time.Duration {
	return time.Duration(math.Round(float64(idx) / fps * float64(time.Second)))
}

func frameNumber(d time.Duration, fps float64) int {
	return int(math.Floor(d.Seconds()*fps + 1e-6))
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		fps  float64
		want string
	}{
		{"29.97 frame 0", frameStart(0, 29.97), 29.97, "00:00.00"},
		{"29.97 frame 29 closes second 0", frameStart(29, 29.97), 29.97, "00:00.29"},
		{"29.97 frame 30 opens second 1", frameStart(30, 29.97), 29.97, "00:01.00"},
		{"29.97 frame 59 closes second 1", frameStart(59, 29.97), 29.97, "00:01.29"},
		{"29.97 frame 990 opens second 33", frameStart(990, 29.97), 29.97, "00:33.00"},
		{"29.97 frame 1018 closes the short second 33", frameStart(1018, 29.97), 29.97, "00:33.28"},
		{"29.97 frame 1019 opens second 34", frameStart(1019, 29.97), 29.97, "00:34.00"},
		{"30 frame 29", frameStart(29, 30), 30, "00:00.29"},
		{"30 frame 30", frameStart(30, 30), 30, "00:01.00"},
		{"25 frame 24", frameStart(24, 25), 25, "00:00.24"},
		{"23.976 frame 23", frameStart(23, 23.976), 23.976, "00:00.23"},
		{"23.976 frame 24", frameStart(24, 23.976), 23.976, "00:01.00"},
		{"minutes roll over", 61*time.Second + 500*time.Millisecond, 30, "01:01.15"},
		{"audio-only has no frame counter", 3 * time.Second, 0, "00:03.00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDuration(tc.d, tc.fps); got != tc.want {
				t.Errorf("formatDuration(%v, %v) = %q, want %q", tc.d, tc.fps, got, tc.want)
			}
		})
	}
}

func TestFormatDurationCountsFramesContiguously(t *testing.T) {
	for _, fps := range []float64{23.976, 25, 29.97, 30, 59.94} {
		prevSec, prevFrame := -1, -1
		for idx := range 3000 {
			out := formatDuration(frameStart(idx, fps), fps)
			var mins, secs, frame int
			if _, err := fmt.Sscanf(out, "%d:%d.%d", &mins, &secs, &frame); err != nil {
				t.Fatalf("fps %v frame %d: unparsable %q: %v", fps, idx, out, err)
			}
			sec := mins*60 + secs
			switch {
			case sec == prevSec && frame != prevFrame+1:
				t.Fatalf("fps %v frame %d: %q does not follow frame %02d", fps, idx, out, prevFrame)
			case sec == prevSec+1 && frame != 0:
				t.Fatalf("fps %v frame %d: %q should restart the frame counter", fps, idx, out)
			case sec != prevSec && sec != prevSec+1:
				t.Fatalf("fps %v frame %d: %q skipped a second after %02d", fps, idx, out, prevSec)
			}
			prevSec, prevFrame = sec, frame
		}
	}
}

func TestStepFrames(t *testing.T) {
	tests := []struct {
		name string
		pos  time.Duration
		fps  float64
		n    int
		want time.Duration
	}{
		{"29.97 one frame forward", frameStart(30, 29.97), 29.97, 1, frameStart(31, 29.97)},
		{"29.97 one frame back", frameStart(30, 29.97), 29.97, -1, frameStart(29, 29.97)},
		{"29.97 snaps a position that is not on a frame boundary", time.Second, 29.97, 1, frameStart(30, 29.97)},
		{"29.97 repeat count steps that many frames", frameStart(100, 29.97), 29.97, 10, frameStart(110, 29.97)},
		{"stops at the first frame", frameStart(0, 29.97), 29.97, -1, 0},
		{"25 one frame forward", frameStart(7, 25), 25, 1, frameStart(8, 25)},
		{"audio-only leaves the position alone", 5 * time.Second, 0, 1, 5 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stepFrames(tc.pos, tc.fps, tc.n); got != tc.want {
				t.Errorf("stepFrames(%v, %v, %d) = %v, want %v", tc.pos, tc.fps, tc.n, got, tc.want)
			}
		})
	}
}

func TestStepFramesAdvancesExactlyOneFrame(t *testing.T) {
	for _, fps := range []float64{23.976, 25, 29.97, 30, 59.94} {
		pos := time.Duration(0)
		for i := 1; i <= 500; i++ {
			pos = stepFrames(pos, fps, 1)
			if got := frameNumber(pos, fps); got != i {
				t.Fatalf("fps %v: step %d landed on frame %d (%v)", fps, i, got, pos)
			}
		}
		for i := 499; i >= 0; i-- {
			pos = stepFrames(pos, fps, -1)
			if got := frameNumber(pos, fps); got != i {
				t.Fatalf("fps %v: back step to %d landed on frame %d (%v)", fps, i, got, pos)
			}
		}
	}
}
