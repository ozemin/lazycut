//go:build integration

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ozemin/lazycut/video"
)

func synth(t testing.TB, name string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	out := filepath.Join(t.TempDir(), name)
	full := append([]string{"-y", "-v", "error"}, args...)
	full = append(full, out)
	if b, err := exec.Command("ffmpeg", full...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, b)
	}
	return out
}

func TestModelSize(t *testing.T) {
	t.Logf("unsafe.Sizeof(Model{}) = %d bytes", unsafe.Sizeof(Model{}))
}

// fps==0 (audio-only or unparsable r_frame_rate) => integer division by zero in Update.
func TestUpdateWithZeroFPS(t *testing.T) {
	path := synth(t, "audio.m4a", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "2", "-c:a", "aac")
	player, err := video.NewPlayer(path, 24)
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	defer player.Close()
	t.Logf("FPS()=%.2f", player.FPS())
	m := NewModel(player)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Update panicked on first keypress: %v", r)
		}
	}()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
}

// ctrl+c inside the export modal is swallowed by the default branch.
func TestExportModalSwallowsCtrlC(t *testing.T) {
	path := synth(t, "v.mp4", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30", "-t", "2", "-c:v", "libx264", "-preset", "ultrafast")
	player, err := video.NewPlayer(path, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	m := NewModel(player)
	m.showExportModal = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	t.Logf("cmd returned for ctrl+c in export modal: %v (nil means ignored)", cmd != nil)
	if cmd != nil {
		t.Errorf("expected ctrl+c to be swallowed (documenting current behaviour)")
	}
}

func BenchmarkView(b *testing.B) {
	path := synth(b, "v.mp4", "-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=30", "-t", "2", "-c:v", "libx264", "-preset", "ultrafast")
	player, err := video.NewPlayer(path, 24)
	if err != nil {
		b.Fatal(err)
	}
	defer player.Close()
	for _, sz := range []struct{ w, h int }{{200, 50}, {300, 80}} {
		m := NewModel(player)
		m.splashDone = true
		m.width, m.height = sz.w, sz.h
		dims := CalculatePanelDimensions(sz.w, sz.h)
		player.SetSize(dims.PreviewContentWidth, dims.PreviewContentHeight)
		if player.CurrentFrame() == "" {
			b.Fatal("no frame rendered")
		}
		b.Run(fmt.Sprintf("term%dx%d", sz.w, sz.h), func(b *testing.B) {
			b.ReportAllocs()
			var n int
			for b.Loop() {
				n = len(m.View())
			}
			b.ReportMetric(float64(n), "view-bytes")
		})
	}
	_ = os.Stderr
}
