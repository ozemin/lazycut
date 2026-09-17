//go:build integration

package video

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderFailuresStopPlayback(t *testing.T) {
	src, err := os.ReadFile(synthVideo)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPlayer(t, path)
	p.SetSize(120, 30)

	// Properties are already read, so this leaves a playable-looking player whose every ffmpeg run fails.
	if err := os.WriteFile(path, []byte("not a video"), 0o644); err != nil {
		t.Fatal(err)
	}

	start := p.Position()
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, pid := range childProcs(t, "ffmpeg") {
			seen[pid] = true
		}
		if !p.IsPlaying() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("distinct ffmpeg pids observed: %d", len(seen))
	if p.IsPlaying() {
		t.Error("playback still running after repeated render failures")
	}
	if len(seen) > maxRenderFailures {
		t.Errorf("spawned %d ffmpeg processes, want at most %d", len(seen), maxRenderFailures)
	}
	if got := p.Position(); got != start {
		t.Errorf("position moved to %v, want it left at %v", got, start)
	}
}
