package panels

import (
	"github.com/emin-ozata/lazycut/video"

	"github.com/charmbracelet/lipgloss"
)

// Preview represents the video preview panel
type Preview struct {
	player *video.Player
}

// NewPreview creates a new Preview panel
func NewPreview(player *video.Player) *Preview {
	return &Preview{
		player: player,
	}
}

// Render renders the preview panel
func (p *Preview) Render(width, height int) string {
	frame := p.player.CurrentFrame()

	if frame == "" {
		// Show placeholder when no frame available
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
