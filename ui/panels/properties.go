package panels

import (
	"fmt"
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

func (p *Properties) Render(width, height int) string {
	props := p.player.Properties()
	if props == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Render("No properties")
	}

	var lines []string

	labelStyle := lipgloss.NewStyle().Width(12).Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

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

	fps := p.player.FPS()

	for i, sec := range p.player.Sections {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render(fmt.Sprintf("Section %d", i+1)))
		addLine("In", formatTime(sec.In, fps))
		addLine("Out", formatTime(sec.Out, fps))
		addLine("Length", formatTime(sec.Duration(), fps))
	}

	trim := &p.player.Trim
	if trim.InPoint != nil || trim.OutPoint != nil {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Selection"))

		if trim.InPoint != nil {
			addLine("In", formatTime(*trim.InPoint, fps))
		}
		if trim.OutPoint != nil {
			addLine("Out", formatTime(*trim.OutPoint, fps))
		}
		if trim.IsComplete() {
			addLine("Length", formatTime(trim.Duration(), fps))
			addLine("Est. Size", props.EstimateOutputSize(trim.Duration()))
		}
	}

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

func formatTime(d interface{ Seconds() float64 }, fps int) string {
	s := d.Seconds()
	total := int(s)
	mins := total / 60
	secs := total % 60
	frame := 0
	if fps > 0 {
		frame = int(s*float64(fps)) % fps
	}
	return fmt.Sprintf("%02d:%02d.%02d", mins, secs, frame)
}
