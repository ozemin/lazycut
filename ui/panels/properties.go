package panels

import (
	"strings"

	"github.com/ozemin/lazycut/video"

	"github.com/charmbracelet/lipgloss"
)

type Properties struct {
	player *video.Player
}

func NewProperties(player *video.Player) *Properties {
	return &Properties{
		player: player,
	}
}

func (p *Properties) RenderLine(width int) string {
	props := p.player.Properties()
	if props == nil {
		return ""
	}

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	sep := sepStyle.Render("  ·  ")
	kv := func(label, value string) string {
		return labelStyle.Render(label) + " " + valueStyle.Render(value)
	}

	parts := []string{
		kv("Resolution", props.Resolution()),
		kv("Codec", props.Codec),
		kv("FPS", props.FormattedFPS()),
		kv("Bitrate", props.FormattedBitrate()),
		kv("Size", props.FormattedFileSize()),
		kv("Duration", props.FormattedDuration()),
	}

	return " " + strings.Join(parts, sep)
}
