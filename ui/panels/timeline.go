package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/ozemin/lazycut/video"

	"github.com/charmbracelet/lipgloss"
)

type Timeline struct {
	player        *video.Player
	exportStatus  string
	repeatDisplay string
}

func NewTimeline(player *video.Player) *Timeline {
	return &Timeline{
		player: player,
	}
}

func (t *Timeline) SetExportStatus(status string) {
	t.exportStatus = status
}

func (t *Timeline) SetRepeatDisplay(s string) {
	t.repeatDisplay = s
}

func (t *Timeline) Render(width, height int) string {
	pos := t.player.Position()
	dur := t.player.Duration()
	playing := t.player.IsPlaying()
	trim := &t.player.Trim

	playIcon := "▶ "
	if playing {
		playIcon = "❚❚"
	}

	muteIcon := "))"
	if t.player.IsMuted() {
		muteIcon = "×)"
	}

	barWidth := max(width-3, 10)

	fps := t.player.FPS()
	line1 := fmt.Sprintf(" %s %s / %s  %s", playIcon, formatDuration(pos, fps), formatDuration(dur, fps), muteIcon)
	line2 := " " + t.markerLine(barWidth, dur, trim)
	line3 := " " + t.progressBar(barWidth, pos, dur, trim)
	line4 := " " + t.cursorLine(barWidth, pos, dur)
	line5 := t.footerHelp(width)

	content := strings.Join([]string{line1, line2, line3, line4, line5}, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

func (t *Timeline) progressBar(barWidth int, pos, dur time.Duration, trim *video.TrimState) string {
	if dur <= 0 {
		return "[" + repeat("-", barWidth) + "]"
	}

	cursor := min(int(float64(pos)/float64(dur)*float64(barWidth)), barWidth)

	var in, out int = -1, -1
	if trim.InPoint != nil {
		in = min(int(float64(*trim.InPoint)/float64(dur)*float64(barWidth)), barWidth)
	}
	if trim.OutPoint != nil {
		out = min(int(float64(*trim.OutPoint)/float64(dur)*float64(barWidth)), barWidth)
	}

	// Build index ranges for committed sections
	type sectionRange struct{ in, out int }
	var committedRanges []sectionRange
	for _, sec := range t.player.Sections {
		si := min(int(float64(sec.In)/float64(dur)*float64(barWidth)), barWidth)
		so := min(int(float64(sec.Out)/float64(dur)*float64(barWidth)), barWidth)
		committedRanges = append(committedRanges, sectionRange{si, so})
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		inActive := in >= 0 && out >= 0 && i >= in && i <= out
		inCommitted := false
		for _, r := range committedRanges {
			if i >= r.in && i <= r.out {
				inCommitted = true
				break
			}
		}

		if inActive || inCommitted {
			bar.WriteString("▓")
		} else if i < cursor {
			bar.WriteString("=")
		} else {
			bar.WriteString("-")
		}
	}
	bar.WriteString("]")

	return bar.String()
}

func (t *Timeline) markerLine(barWidth int, dur time.Duration, trim *video.TrimState) string {
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
		si := min(int(float64(sec.In)/float64(dur)*float64(barWidth))+1, len(line)-1)
		so := min(int(float64(sec.Out)/float64(dur)*float64(barWidth))+1, len(line)-1)
		line[si] = inStyle.Render("▼")
		line[so] = outStyle.Render("▼")
	}

	if trim.InPoint != nil {
		in := min(int(float64(*trim.InPoint)/float64(dur)*float64(barWidth))+1, len(line)-1)
		line[in] = inStyle.Render("▼")
	}

	if trim.OutPoint != nil {
		out := min(int(float64(*trim.OutPoint)/float64(dur)*float64(barWidth))+1, len(line)-1)
		line[out] = outStyle.Render("▼")
	}

	return strings.Join(line, "")
}

func (t *Timeline) cursorLine(barWidth int, pos, dur time.Duration) string {
	if dur <= 0 {
		return repeat(" ", barWidth+2)
	}

	line := make([]rune, barWidth+2)
	for i := range line {
		line[i] = ' '
	}

	cursor := min(int(float64(pos)/float64(dur)*float64(barWidth))+1, len(line)-1)
	line[cursor] = '▲'

	return string(line)
}

func formatDuration(d time.Duration, fps int) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	frame := 0
	if fps > 0 {
		frame = int(d.Seconds()*float64(fps)) % fps
	}
	return fmt.Sprintf("%02d:%02d.%02d", mins, secs, frame)
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

func (t *Timeline) footerHelp(width int) string {
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

	badge := ""
	if len(sections) > 0 {
		badge = dimStyle.Render(fmt.Sprintf("%d section(s)", len(sections))) + "  "
	}

	var result string

	if t.exportStatus != "" {
		result = " " + t.exportStatus
	} else if len(sections) > 0 && trim.InPoint == nil {
		remove := "remove section"
		if len(sections) > 1 {
			remove = "remove last section"
		}
		preview := "preview"
		if len(sections) > 1 {
			preview = "preview all"
		}
		hints := " " + badge +
			kd("Enter", "export", true) + sep +
			kd("i", "in", false) + "  " + kd("o", "out", false) + sep +
			kd("X", remove, false) + sep +
			kd("p", preview, false)
		if len(sections) > 1 {
			hints += "  " + kd("P", "preview last", false)
		}
		result = hints + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + sep +
			kd("?", "help", false)
	} else if trim.InPoint != nil {
		result = " " + badge + dimStyle.Render("IN set") + "  " +
			kd("o", "set out", true) + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + sep +
			kd("d", "clear", false) + "  " + kd("?", "help", false)
	} else {
		result = " " + kd("i", "in", false) + "  " + kd("o", "out", false) + sep +
			kd("h/l", "±1s", false) + "  " + kd("H/L", "±5s", false) + "  " + kd(",/.", "±frame", false) + sep +
			kd("m", "mute", false) + sep +
			kd("?", "help", false)
	}

	if t.repeatDisplay != "" {
		result += dimStyle.Render("  ·  ") + accentStyle.Render(t.repeatDisplay)
	}

	return result
}
