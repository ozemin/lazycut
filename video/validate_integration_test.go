//go:build integration

package video

import (
	"errors"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateRejectsAudioOnly(t *testing.T) {
	p := newTestPlayer(t, synthAudio)
	if err := p.Validate(); !errors.Is(err, ErrNoVideoStream) {
		t.Errorf("Validate() = %v, want %v", err, ErrNoVideoStream)
	}
}

func TestValidateAcceptsVideo(t *testing.T) {
	p := newTestPlayer(t, synthVideo)
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestPlayerKeepsFractionalFPS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ntsc.mp4")
	out, err := exec.Command("ffmpeg", "-y", "-v", "error", "-f", "lavfi",
		"-i", "testsrc2=size=320x180:rate=30000/1001", "-t", "2",
		"-c:v", "libx264", "-preset", "ultrafast", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}

	p, err := NewPlayer(path, 24)
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	defer p.Close()

	if want := 30000.0 / 1001.0; math.Abs(p.FPS()-want) > 1e-6 {
		t.Errorf("FPS() = %v, want %v", p.FPS(), want)
	}
}
