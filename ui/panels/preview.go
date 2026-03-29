package panels

import (
	"github.com/ozemin/lazycut/video"

	"github.com/charmbracelet/lipgloss"
)

type Preview struct {
	player *video.Player
}

func NewPreview(player *video.Player) *Preview {
	return &Preview{
		player: player,
	}
}

func (p *Preview) Render(width, height int) string {
	frame := p.player.CurrentFrame()

	if frame == "" {
		placeholder := "Press SPACE to play"
		if p.player.IsPlaying() {
			placeholder = "Loading..."
		}
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(placeholder)
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(frame)
}
