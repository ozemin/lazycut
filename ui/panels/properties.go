package panels

import (
	"fmt"
	"github.com/arobase-che/lazycut/video"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Properties represents the video properties panel
type Properties struct {
	player *video.Player
}

// NewProperties creates a new Properties panel
func NewProperties(player *video.Player) *Properties {
	return &Properties{
		player: player,
	}
}

// Render renders the properties panel
func (p *Properties) Render(width, height int) string {
	props := p.player.Properties()
	if props == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Render("No properties")
	}

	var lines []string

	// Static properties section
	labelStyle := lipgloss.NewStyle().Width(12)
	valueStyle := lipgloss.NewStyle()

	addLine := func(label, value string) {
		line := labelStyle.Render(label) + valueStyle.Render(value)
		lines = append(lines, line)
	}

	addLine("Resolution", props.Resolution())
	addLine("Codec", props.Codec)
	addLine("FPS", props.FormattedFPS())
	addLine("Bitrate", props.FormattedBitrate())
	addLine("Size", props.FormattedFileSize())
	addLine("Duration", props.FormattedDuration())

	// Quality indicator with color
	quality := p.player.Quality()
	qualityColor := "243" // gray for LOW
	if quality == video.QualityHigh {
		qualityColor = "46" // green
	}
	qualityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(qualityColor))
	addLine("Quality", qualityStyle.Render(quality.String()))

	// Selection section (only show if trim points are set)
	trim := &p.player.Trim
	if trim.InPoint != nil || trim.OutPoint != nil {
		lines = append(lines, "") // Empty line separator
		lines = append(lines, "Selection")

		if trim.InPoint != nil {
			addLine("In", formatTime(*trim.InPoint))
		}
		if trim.OutPoint != nil {
			addLine("Out", formatTime(*trim.OutPoint))
		}
		if trim.IsComplete() {
			addLine("Length", formatTime(trim.Duration()))
			addLine("Est. Size", props.EstimateOutputSize(trim.Duration()))
		}
	}

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

// formatTime formats a duration as MM:SS
func formatTime(d interface{ Seconds() float64 }) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}
