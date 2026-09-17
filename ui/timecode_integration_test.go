//go:build integration

package ui

import (
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ozemin/lazycut/video"
)

func ffprobePTS(t *testing.T, path string) []float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "frame=pts_time", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var pts []float64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSuffix(strings.TrimSpace(line), ",")
		if line == "" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err != nil {
			t.Fatalf("unparsable pts_time %q: %v", line, err)
		}
		pts = append(pts, v)
	}
	sort.Float64s(pts)
	return pts
}

// Expectations come from ffprobe's own pts_time list, not from our arithmetic.
func TestFormatDurationMatchesFfprobeFrameNumbers(t *testing.T) {
	path := synth(t, "ntsc.mp4", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30000/1001",
		"-t", "5", "-c:v", "libx264", "-preset", "ultrafast")

	player, err := video.NewPlayer(path, 24)
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	defer player.Close()

	fps := player.FPS()
	if math.Abs(fps-30000.0/1001.0) > 1e-6 {
		t.Fatalf("FPS() = %v, want %v", fps, 30000.0/1001.0)
	}

	pts := ffprobePTS(t, path)
	if len(pts) < 100 {
		t.Fatalf("expected a few seconds of frames, got %d", len(pts))
	}

	firstOfSecond := map[int]int{}
	for idx, p := range pts {
		sec := int(math.Floor(p))
		if _, ok := firstOfSecond[sec]; !ok {
			firstOfSecond[sec] = idx
		}
	}

	for idx, p := range pts {
		sec := int(math.Floor(p))
		want := fmt.Sprintf("%02d:%02d.%02d", sec/60, sec%60, idx-firstOfSecond[sec])
		got := formatDuration(time.Duration(p*float64(time.Second)), fps)
		if got != want {
			t.Fatalf("frame %d (pts %.6f): formatDuration = %q, want %q", idx, p, got, want)
		}
	}
}
