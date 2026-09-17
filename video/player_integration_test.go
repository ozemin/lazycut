//go:build integration

package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	synthVideo string
	synthAudio string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		fmt.Println("ffmpeg not installed; skipping integration tests")
		os.Exit(0)
	}
	dir, err := os.MkdirTemp("", "lazycut-it-*")
	if err != nil {
		panic(err)
	}
	synthVideo = filepath.Join(dir, "synth.mp4")
	synthAudio = filepath.Join(dir, "audio.m4a")
	run := func(args ...string) {
		if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
			panic(fmt.Sprintf("ffmpeg %v: %v\n%s", args, err, out))
		}
	}
	run("-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=30",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "30",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "30", "-c:a", "aac", "-shortest", synthVideo)
	run("-y", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "10", "-c:a", "aac", synthAudio)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newTestPlayer(t *testing.T, path string) *Player {
	t.Helper()
	p, err := NewPlayer(path, 24)
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	p.ToggleMute() // keep ffplay out of the picture unless a test wants it
	t.Cleanup(p.Close)
	return p
}

func childProcs(t *testing.T, name string) []string {
	t.Helper()
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(os.Getpid()), "-x", name).Output()
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func waitGoroutines(base int, d time.Duration) int {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if n := runtime.NumGoroutine(); n <= base {
			return n
		}
		time.Sleep(50 * time.Millisecond)
	}
	return runtime.NumGoroutine()
}

func dumpLeakProfile(t *testing.T) {
	if p := pprof.Lookup("goroutineleak"); p != nil {
		var sb strings.Builder
		_ = p.WriteTo(&sb, 1)
		t.Logf("goroutineleak profile:\n%s", sb.String())
	}
}

// Rapid Play/Pause toggling: exercises stopChan/frameBuffer reassignment races.
func TestRapidToggle(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	p.SetSize(120, 30)
	base := runtime.NumGoroutine()
	for i := 0; i < 40; i++ {
		_ = p.Play()
		time.Sleep(time.Duration(i%7) * 3 * time.Millisecond)
		p.Pause()
	}
	time.Sleep(300 * time.Millisecond)
	n := waitGoroutines(base, 3*time.Second)
	t.Logf("goroutines base=%d after=%d ffmpeg-children=%d", base, n, len(childProcs(t, "ffmpeg")))
	dumpLeakProfile(t)
	if n > base {
		t.Errorf("goroutine leak: %d > %d", n, base)
	}
}

// Seek spam while playing: measures cost per Seek and process churn.
func TestSeekSpamWhilePlaying(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	p.ToggleMute() // unmute so ffplay churn is included
	p.SetSize(120, 30)
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	const n = 30
	seen := map[string]bool{}
	start := time.Now()
	for i := 0; i < n; i++ {
		p.Seek(time.Duration(1+i%20) * time.Second)
		for _, pid := range childProcs(t, "ffplay") {
			seen[pid] = true
		}
		time.Sleep(20 * time.Millisecond)
	}
	el := time.Since(start)
	t.Logf("%d seeks in %v => %.1f ms/seek (incl. 20ms sleep); distinct ffplay pids observed=%d", n, el, float64(el.Milliseconds())/n, len(seen))
	base := runtime.NumGoroutine()
	p.Pause()
	after := waitGoroutines(base-2, 3*time.Second)
	t.Logf("goroutines while playing=%d after pause=%d", base, after)
	dumpLeakProfile(t)
}

// Resize spam while paused: each SetSize spawns ffmpeg+chafa synchronously.
func TestResizeSpamWhilePaused(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	p.Seek(5 * time.Second)
	start := time.Now()
	const n = 20
	for i := 0; i < n; i++ {
		p.SetSize(100+i, 30+i%3)
	}
	el := time.Since(start)
	t.Logf("%d resizes in %v => %.1f ms/resize (blocking the UI goroutine)", n, el, float64(el.Milliseconds())/n)
}

// Video clock drift: position advance vs wall clock over a playback window.
func TestVideoClockDrift(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	p.SetSize(120, 30)
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	// let the pipeline warm up
	for p.Position() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	p0 := p.Position()
	w0 := time.Now()
	time.Sleep(10 * time.Second)
	p1 := p.Position()
	w1 := time.Now()
	posDelta := p1 - p0
	wallDelta := w1.Sub(w0)
	drift := posDelta - wallDelta
	t.Logf("video advanced %v in wall %v => drift %v (%.2f%%), extrapolated to 5min: %v",
		posDelta, wallDelta, drift, float64(drift)/float64(wallDelta)*100, time.Duration(float64(drift)*300/wallDelta.Seconds()))
	p.Pause()
}

// Audio-only input: video stream absent; Play must not spin spawning ffmpeg.
func TestAudioOnlyDoesNotSpin(t *testing.T) {
	p := newTestPlayer(t, synthAudio)
	p.SetSize(120, 30)
	t.Logf("props: %dx%d fps=%.2f dur=%v", p.properties.Width, p.properties.Height, p.properties.FPS, p.duration)
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		for _, pid := range childProcs(t, "ffmpeg") {
			seen[pid] = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Pause()
	t.Logf("distinct ffmpeg pids spawned in 1s while 'playing' audio-only file: %d (playing=%v)", len(seen), p.IsPlaying())
	if len(seen) > 5 {
		t.Errorf("ffmpeg respawn storm: %d processes in 1s", len(seen))
	}
}

// Natural EOF then Play again: stale p.stream and position=duration behaviour.
func TestPlayPastEOF(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	p.SetSize(120, 30)
	p.Seek(p.Duration() - 300*time.Millisecond)
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	t.Logf("after EOF: playing=%v pos=%v dur=%v", p.IsPlaying(), p.Position(), p.Duration())
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	t.Logf("Play() at EOF: playing=%v pos=%v (expected: rewinds or stays stopped, not a busy loop)", p.IsPlaying(), p.Position())
	p.Pause()
}
