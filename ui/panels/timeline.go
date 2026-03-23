package panels

import (
	"fmt"
	"github.com/emin-ozata/lazycut/video"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Timeline struct {
	player       *video.Player
	exportStatus string
}

func NewTimeline(player *video.Player) *Timeline {
	return &Timeline{
		player: player,
	}
}

func (t *Timeline) SetExportStatus(status string) {
	t.exportStatus = status
}

func (t *Timeline) Render(width, height int) string {
	pos := t.player.Position()
	dur := t.player.Duration()
	playing := t.player.IsPlaying()
	trim := &t.player.Trim

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)

	playIcon := "▶ "
	if playing {
		playIcon = "❚❚"
	}

	muteIcon := "))"
	if t.player.IsMuted() {
		muteIcon = "×)"
	}

	barWidth := width - 3
	if barWidth < 10 {
		barWidth = 10
	}

	line1 := fmt.Sprintf(" %s %s / %s  %s", playIcon, posStr, durStr, muteIcon)
	line2 := " " + t.buildMarkerLine(barWidth, dur, trim)
	line3 := " " + t.buildProgressBar(barWidth, pos, dur, trim)
	line4 := " " + t.buildCursorLine(barWidth, pos, dur)
	line5 := t.buildFooterHelp(width)

	content := strings.Join([]string{line1, line2, line3, line4, line5}, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

func (t *Timeline) buildProgressBar(barWidth int, pos, dur time.Duration, trim *video.TrimState) string {
	if dur <= 0 {
		return "[" + repeat("-", barWidth) + "]"
	}

	posIdx := int(float64(pos) / float64(dur) * float64(barWidth))
	if posIdx > barWidth {
		posIdx = barWidth
	}

	var inIdx, outIdx int = -1, -1
	if trim.InPoint != nil {
		inIdx = int(float64(*trim.InPoint) / float64(dur) * float64(barWidth))
		if inIdx > barWidth {
			inIdx = barWidth
		}
	}
	if trim.OutPoint != nil {
		outIdx = int(float64(*trim.OutPoint) / float64(dur) * float64(barWidth))
		if outIdx > barWidth {
			outIdx = barWidth
		}
	}

	// Build index ranges for committed sections
	type sectionRange struct{ in, out int }
	var committedRanges []sectionRange
	for _, sec := range t.player.Sections {
		si := int(float64(sec.In) / float64(dur) * float64(barWidth))
		so := int(float64(sec.Out) / float64(dur) * float64(barWidth))
		if si > barWidth {
			si = barWidth
		}
		if so > barWidth {
			so = barWidth
		}
		committedRanges = append(committedRanges, sectionRange{si, so})
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		inActive := inIdx >= 0 && outIdx >= 0 && i >= inIdx && i <= outIdx
		inCommitted := false
		for _, r := range committedRanges {
			if i >= r.in && i <= r.out {
				inCommitted = true
				break
			}
		}

		if inActive || inCommitted {
			bar.WriteString("▓")
		} else if i < posIdx {
			bar.WriteString("=")
		} else {
			bar.WriteString("-")
		}
	}
	bar.WriteString("]")

	return bar.String()
}

func (t *Timeline) buildMarkerLine(barWidth int, dur time.Duration, trim *video.TrimState) string {
	if dur <= 0 {
		return repeat(" ", barWidth+2)
	}

	inStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	outStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("166")).Bold(true)

	line := make([]string, barWidth+2)
	for i := range line {
		line[i] = " "
	}

	// Draw committed section markers (active trim markers take priority)
	for _, sec := range t.player.Sections {
		si := int(float64(sec.In)/float64(dur)*float64(barWidth)) + 1
		so := int(float64(sec.Out)/float64(dur)*float64(barWidth)) + 1
		if si >= len(line) {
			si = len(line) - 1
		}
		if so >= len(line) {
			so = len(line) - 1
		}
		line[si] = inStyle.Render("▼")
		line[so] = outStyle.Render("▼")
	}

	if trim.InPoint != nil {
		inIdx := int(float64(*trim.InPoint)/float64(dur)*float64(barWidth)) + 1
		if inIdx >= len(line) {
			inIdx = len(line) - 1
		}
		line[inIdx] = inStyle.Render("▼")
	}

	if trim.OutPoint != nil {
		outIdx := int(float64(*trim.OutPoint)/float64(dur)*float64(barWidth)) + 1
		if outIdx >= len(line) {
			outIdx = len(line) - 1
		}
		line[outIdx] = outStyle.Render("▼")
	}

	return strings.Join(line, "")
}

func (t *Timeline) buildCursorLine(barWidth int, pos, dur time.Duration) string {
	if dur <= 0 {
		return repeat(" ", barWidth+2)
	}

	line := make([]rune, barWidth+2)
	for i := range line {
		line[i] = ' '
	}

	posIdx := int(float64(pos)/float64(dur)*float64(barWidth)) + 1
	if posIdx >= len(line) {
		posIdx = len(line) - 1
	}
	line[posIdx] = '▲'

	return string(line)
}

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func (t *Timeline) buildFooterHelp(width int) string {
	trim := &t.player.Trim
	sections := t.player.Sections

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	kd := func(key, desc string, accent bool) string {
		if accent {
			return accentStyle.Render(key) + descStyle.Render(" "+desc)
		}
		return keyStyle.Render(key) + descStyle.Render(" "+desc)
	}

	sep := dimStyle.Render("  ·  ")

	sectionBadge := ""
	if len(sections) > 0 {
		sectionBadge = dimStyle.Render(fmt.Sprintf("%d section(s)", len(sections))) + "  "
	}

	var result string

	if t.exportStatus != "" {
		result = " " + t.exportStatus
	} else if len(sections) > 0 && trim.InPoint == nil {
		removeLabel := "remove section"
		if len(sections) > 1 {
			removeLabel = "remove last section"
		}
		previewLabel := "preview"
		if len(sections) > 1 {
			previewLabel = "preview all"
		}
		hints := " " + sectionBadge +
			kd("Enter", "export", true) + sep +
			kd("i", "in", false) + "  " + kd("o", "out", false) + sep +
			kd("X", removeLabel, false) + sep +
			kd("p", previewLabel, false)
		if len(sections) > 1 {
			hints += "  " + kd("P", "preview last", false)
		}
		result = hints + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + sep +
			kd("?", "help", false)
	} else if trim.InPoint != nil {
		result = " " + sectionBadge + dimStyle.Render("IN set") + "  " +
			kd("o", "set out", true) + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + sep +
			kd("d", "clear", false) + "  " + kd("?", "help", false)
	} else {
		result = " " + kd("i", "in", false) + "  " + kd("o", "out", false) + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + "  " + kd(",/.", "±frame", false) + sep +
			kd("m", "mute", false) + sep +
			kd("?", "help", false)
	}

	return result
}
