package video

import (
	"fmt"
	"os/exec"
	"sync"
)

type AudioPlayer struct {
	filePath string
	cmd      *exec.Cmd
	muted    bool
	mu       sync.Mutex
}

func NewAudioPlayer(filePath string) *AudioPlayer {
	return &AudioPlayer{
		filePath: filePath,
		muted:    false,
	}
}

func (a *AudioPlayer) Start(position float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.muted {
		return
	}

	a.stopLocked()

	a.cmd = exec.Command("ffplay",
		"-nodisp",
		"-autoexit",
		"-vn",
		"-ss", formatSeconds(position),
		"-loglevel", "quiet",
		a.filePath,
	)

	_ = a.cmd.Start()
}

func (a *AudioPlayer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}

func (a *AudioPlayer) stopLocked() {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
		a.cmd = nil
	}
}

func (a *AudioPlayer) ToggleMute() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.muted = !a.muted
	if a.muted {
		a.stopLocked()
	}
}

func (a *AudioPlayer) IsMuted() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.muted
}

func formatSeconds(seconds float64) string {
	return fmt.Sprintf("%.3f", seconds)
}
