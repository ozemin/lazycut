package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ozemin/lazycut/ui/keymap"
	"github.com/ozemin/lazycut/ui/panels"
	"github.com/ozemin/lazycut/video"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const tickFPS = 30

var logo = []string{
	` ██╗      █████╗ ███████╗██╗   ██╗ ██████╗██╗   ██╗████████╗`,
	` ██║     ██╔══██╗╚══███╔╝╚██╗ ██╔╝██╔════╝██║   ██║╚══██╔══╝`,
	` ██║     ███████║  ███╔╝  ╚████╔╝ ██║     ██║   ██║   ██║   `,
	` ██║     ██╔══██║ ███╔╝    ╚██╔╝  ██║     ██║   ██║   ██║   `,
	` ███████╗██║  ██║███████╗   ██║   ╚██████╗╚██████╔╝   ██║   `,
	` ╚══════╝╚═╝  ╚═╝╚══════╝   ╚═╝    ╚═════╝ ╚═════╝    ╚═╝   `,
}

type splashDoneMsg struct{}

type TickMsg time.Time

type ExportDoneMsg struct {
	Output string
	Err    error
}

type ExportProgressMsg float64

type Model struct {
	width        int
	height       int
	splashDone   bool
	player       *video.Player
	preview      *panels.Preview
	properties   *panels.Properties
	timeline     *panels.Timeline
	previewMode  bool
	exportStatus string

	showExportModal    bool
	exportFilename     string
	exportAspectRatio  int // index into video.AspectRatioOptions
	exportFocusField   int // 0: filename, 1: aspect ratio, 2: mode (multi only)
	exportMode         int // 0: separate clips, 1: single clip
	exporting          bool
	exportProgress     float64
	exportProgressChan <-chan float64

	showHelpModal bool
	undoStack     []trimSnapshot

	km          *keymap.Keymap
	repeatCount int

	previewQueue    []video.Section
	previewQueueIdx int
}

type trimSnapshot struct {
	inPoint  *time.Duration
	outPoint *time.Duration
}

func NewModel(player *video.Player) Model {
	return Model{
		player:     player,
		preview:    panels.NewPreview(player),
		properties: panels.NewProperties(player),
		timeline:   panels.NewTimeline(player),
		km:         keymap.New(),
	}
}

func (m *Model) allSections() []video.Section {
	sections := make([]video.Section, len(m.player.Sections))
	copy(sections, m.player.Sections)
	return sections
}

func (m *Model) saveTrimState() {
	snapshot := trimSnapshot{}
	if m.player.Trim.InPoint != nil {
		val := *m.player.Trim.InPoint
		snapshot.inPoint = &val
	}
	if m.player.Trim.OutPoint != nil {
		val := *m.player.Trim.OutPoint
		snapshot.outPoint = &val
	}
	m.undoStack = append(m.undoStack, snapshot)
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/tickFPS, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ExportProgressMsg:
		m.exportProgress = float64(msg)
		if m.exportProgressChan != nil {
			return m, listenProgress(m.exportProgressChan)
		}
		return m, nil

	case ExportDoneMsg:
		m.exporting = false
		m.showExportModal = false
		m.exportProgress = 0
		m.exportProgressChan = nil
		if msg.Err != nil {
			m.exportStatus = "Export failed: " + msg.Err.Error()
		} else {
			m.exportStatus = "Exported: " + msg.Output
		}
		return m, nil

	case splashDoneMsg:
		m.splashDone = true
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		dims := CalculatePanelDimensions(m.width, m.height)
		m.player.SetSize(dims.PreviewContentWidth, dims.PreviewContentHeight)
		if !m.splashDone {
			return m, tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
				return splashDoneMsg{}
			})
		}
		return m, nil

	case TickMsg:
		if m.previewMode && m.player.IsPlaying() && len(m.previewQueue) > 0 {
			current := m.previewQueue[m.previewQueueIdx]
			if m.player.Position() >= current.Out {
				next := m.previewQueueIdx + 1
				if next < len(m.previewQueue) {
					m.previewQueueIdx = next
					m.player.Seek(m.previewQueue[next].In)
				} else {
					m.player.Pause()
					m.previewMode = false
					m.previewQueue = nil
					m.previewQueueIdx = 0
				}
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		if m.showHelpModal {
			return m.handleHelpModalKey(msg)
		}
		if m.showExportModal {
			return m.handleExportModalKey(msg)
		}
		pos := m.player.Position()
		fps := m.player.FPS()
		frameDuration := time.Second / time.Duration(fps)

		key := msg.String()
		isDigit := len(key) == 1 && key[0] >= '1' && key[0] <= '9'
		isZero := key == "0" && m.repeatCount > 0
		pendingRepeat := m.repeatCount
		if !isDigit && !isZero {
			m.exportStatus = ""
			m.repeatCount = 0
		}

		if amount, ok := m.km.Seek(msg); ok {
			n := pendingRepeat
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n)*amount)
			return m, nil
		}

		switch key {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.repeatCount = pendingRepeat*10 + int(msg.Runes[0]-'0')
			return m, nil
		case "0":
			if pendingRepeat == 0 {
				m.player.Seek(0)
				return m, nil
			}
			m.repeatCount = pendingRepeat * 10
			return m, nil
		case "ctrl+c", "q":
			m.player.Close()
			return m, tea.Quit

		case " ":
			m.player.Toggle()
			return m, nil

		case ",":
			n := pendingRepeat
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n)*frameDuration)
			return m, nil

		case ".":
			n := pendingRepeat
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n)*frameDuration)
			return m, nil

		case "$", "G":
			m.player.Seek(m.player.Duration())
			return m, nil

		case "i":
			m.saveTrimState()
			m.player.Trim.SetIn(pos)
			return m, nil

		case "o":
			m.saveTrimState()
			m.player.Trim.SetOut(pos)
			if m.player.Trim.IsComplete() {
				m.player.AddSection(*m.player.Trim.InPoint, *m.player.Trim.OutPoint)
				m.player.Trim.Clear()
			}
			return m, nil

		case "x":
			m.player.RemoveLastSection()
			return m, nil

		case "X":
			m.player.ClearSections()
			return m, nil

		case "p":
			if n := len(m.player.Sections); n > 0 {
				last := m.player.Sections[n-1]
				m.previewQueue = []video.Section{last}
				m.previewQueueIdx = 0
				m.player.Seek(last.In)
				m.previewMode = true
				m.player.Play()
			}
			return m, nil

		case "P":
			var queue []video.Section
			if m.player.Trim.InPoint != nil && m.player.Trim.OutPoint != nil {
				queue = []video.Section{{In: *m.player.Trim.InPoint, Out: *m.player.Trim.OutPoint}}
			} else if len(m.player.Sections) > 0 {
				queue = make([]video.Section, len(m.player.Sections))
				copy(queue, m.player.Sections)
			}
			if len(queue) > 0 {
				m.previewQueue = queue
				m.previewQueueIdx = 0
				m.player.Seek(queue[0].In)
				m.previewMode = true
				m.player.Play()
			}
			return m, nil

		case "enter":
			if len(m.player.Sections) > 0 {
				m.showExportModal = true
				m.exportFilename = ""
				m.exportAspectRatio = 0
				m.exportMode = 0
				m.exportFocusField = 0
			}
			return m, nil

		case "esc", "d":
			if m.player.Trim.InPoint != nil || m.player.Trim.OutPoint != nil {
				m.saveTrimState()
			}
			m.player.Trim.Clear()
			m.previewMode = false
			return m, nil

		case "?":
			m.showHelpModal = true
			return m, nil

		case "u":
			if len(m.undoStack) > 0 {
				last := m.undoStack[len(m.undoStack)-1]
				m.undoStack = m.undoStack[:len(m.undoStack)-1]
				m.player.Trim.InPoint = last.inPoint
				m.player.Trim.OutPoint = last.outPoint
			}
			return m, nil

		case "m":
			m.player.ToggleMute()
			return m, nil
		}
	}

	return m, nil
}

func renderPanel(content string, width, height int) string {
	innerWidth := width - 2
	innerHeight := height - 2
	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	paddedContent := strings.Join(lines[:innerHeight], "\n")

	return BorderStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(paddedContent)
}

func (m Model) renderSplash() string {
	logoStr := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Render(strings.Join(logo, "\n"))
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(logoStr)
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if !m.splashDone {
		return m.renderSplash()
	}

	dims := CalculatePanelDimensions(m.width, m.height)

	if dims.PreviewContentWidth < minPanelWidth || dims.PreviewContentHeight < minPanelHeight {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Terminal too small")
	}

	previewContent := m.preview.Render(dims.PreviewContentWidth, dims.PreviewContentHeight)
	previewPanel := renderPanel(previewContent, dims.PreviewWidth, dims.PreviewHeight)

	propertiesLine := m.properties.RenderLine(dims.PropertiesLineWidth)

	m.timeline.SetExportStatus(m.exportStatus)
	repeatDisplay := ""
	if m.repeatCount > 0 {
		repeatDisplay = fmt.Sprintf("%dx", m.repeatCount)
	}
	m.timeline.SetRepeatDisplay(repeatDisplay)
	timelineContent := m.timeline.Render(dims.TimelineContentWidth, dims.TimelineContentHeight)
	timelinePanel := renderPanel(timelineContent, dims.TimelineWidth, dims.TimelineHeight)

	base := lipgloss.JoinVertical(lipgloss.Left, previewPanel, propertiesLine, timelinePanel)

	if m.showHelpModal {
		return m.renderHelpModal()
	}
	if m.showExportModal {
		return m.renderExportModal()
	}

	return base
}

func (m Model) handleExportModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sections := m.allSections()
	isMulti := len(sections) > 1
	maxField := 1
	if isMulti {
		maxField = 2
	}

	switch msg.Type {
	case tea.KeyEsc:
		if !m.exporting {
			m.showExportModal = false
		}
		return m, nil

	case tea.KeyEnter:
		if m.exporting {
			return m, nil
		}
		m.exporting = true
		m.exportProgress = 0
		progressChan := make(chan float64, 100)
		m.exportProgressChan = progressChan
		props := m.player.Properties()

		if !isMulti {
			opts := video.ExportOptions{
				Input:       m.player.Path(),
				Output:      m.exportFilename,
				InPoint:     sections[0].In,
				OutPoint:    sections[0].Out,
				AspectRatio: video.AspectRatioOptions[m.exportAspectRatio].Ratio,
				Width:       props.Width,
				Height:      props.Height,
			}
			return m, startExportWithChan(opts, progressChan)
		}

		multiOpts := video.MultiExportOptions{
			Input:       m.player.Path(),
			Output:      m.exportFilename,
			Sections:    sections,
			AspectRatio: video.AspectRatioOptions[m.exportAspectRatio].Ratio,
			Width:       props.Width,
			Height:      props.Height,
		}
		if m.exportMode == 0 {
			return m, startMultiExportSeparate(multiOpts, progressChan)
		}
		return m, startMultiExportConcatenated(multiOpts, progressChan)

	case tea.KeyUp, tea.KeyShiftTab:
		if m.exportFocusField > 0 {
			m.exportFocusField--
		}
		return m, nil

	case tea.KeyDown, tea.KeyTab:
		if m.exportFocusField < maxField {
			m.exportFocusField++
		}
		return m, nil

	case tea.KeyLeft:
		if m.exportFocusField == 1 {
			m.exportAspectRatio--
			if m.exportAspectRatio < 0 {
				m.exportAspectRatio = len(video.AspectRatioOptions) - 1
			}
		} else if m.exportFocusField == 2 {
			m.exportMode = (m.exportMode + 1) % 2
		}
		return m, nil

	case tea.KeyRight:
		if m.exportFocusField == 1 {
			m.exportAspectRatio = (m.exportAspectRatio + 1) % len(video.AspectRatioOptions)
		} else if m.exportFocusField == 2 {
			m.exportMode = (m.exportMode + 1) % 2
		}
		return m, nil

	case tea.KeyBackspace:
		if m.exportFocusField == 0 && len(m.exportFilename) > 0 {
			m.exportFilename = m.exportFilename[:len(m.exportFilename)-1]
		}
		return m, nil

	default:
		if m.exportFocusField == 0 && len(msg.Runes) > 0 {
			m.exportFilename += string(msg.Runes)
		}
		return m, nil
	}
}

func (m Model) handleHelpModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q", "enter", " ":
		m.showHelpModal = false
		return m, nil
	}
	return m, nil
}

func (m Model) renderHelpModal() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true)
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	kd := func(key, desc string) string {
		return keyStyle.Render(fmt.Sprintf("%-9s", key)) + descStyle.Render(desc)
	}

	playback := sectionStyle.Render("PLAYBACK") + "\n" +
		kd("Space", "Play/Pause") + "\n" +
		kd("h / l", "Seek ±1 second") + "\n" +
		kd("H / L", "Seek ±5 seconds") + "\n" +
		kd("← / →", "Seek ±5 seconds") + "\n" +
		kd("⇧← / ⇧→", "Seek ±1 second") + "\n" +
		kd("↑ / ↓", "Seek ±1 minute") + "\n" +
		kd(", / .", "Seek ±1 frame") + "\n" +
		kd("0", "Go to start") + "\n" +
		kd("G / $", "Go to end") + "\n" +
		kd("5l 10.", "Vim-style counts") + "\n" +
		kd("m", "Toggle mute")

	trim := sectionStyle.Render("TRIM") + "\n" +
		kd("i", "Set in-point") + "\n" +
		kd("o", "Set out-point") + "\n" +
		kd("x", "Remove last section") + "\n" +
		kd("X", "Remove all sections") + "\n" +
		kd("p", "Preview last section") + "\n" +
		kd("P", "Preview all sections") + "\n" +
		kd("d / Esc", "Clear selection") + "\n" +
		kd("Enter", "Export")

	other := sectionStyle.Render("OTHER") + "\n" +
		kd("u", "Undo") + "\n" +
		kd("?", "Toggle help") + "\n" +
		kd("q", "Quit")

	footer := dimStyle.Render("Press any key to close")

	content := titleStyle.Render("Keyboard Shortcuts") + "\n\n" +
		playback + "\n\n" +
		trim + "\n\n" +
		other + "\n\n" +
		footer

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 3).
		Width(45).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func startExportWithChan(opts video.ExportOptions, progressChan chan float64) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			output, err := video.ExportWithProgress(opts, progressChan)
			return ExportDoneMsg{Output: output, Err: err}
		},
		listenProgress(progressChan),
	)
}

func startMultiExportSeparate(opts video.MultiExportOptions, progressChan chan float64) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			outputs, err := video.ExportSeparate(opts, progressChan)
			output := strings.Join(outputs, ", ")
			return ExportDoneMsg{Output: output, Err: err}
		},
		listenProgress(progressChan),
	)
}

func startMultiExportConcatenated(opts video.MultiExportOptions, progressChan chan float64) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			output, err := video.ExportConcatenated(opts, progressChan)
			return ExportDoneMsg{Output: output, Err: err}
		},
		listenProgress(progressChan),
	)
}

func listenProgress(ch <-chan float64) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return ExportProgressMsg(p)
	}
}

func (m Model) renderExportModal() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true)
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("75")).
		Bold(true)
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	sections := m.allSections()
	isMulti := len(sections) > 1

	props := m.player.Properties()

	var ffmpegCmd string
	if len(sections) > 0 {
		opts := video.ExportOptions{
			Input:       m.player.Path(),
			Output:      m.exportFilename,
			InPoint:     sections[0].In,
			OutPoint:    sections[0].Out,
			AspectRatio: video.AspectRatioOptions[m.exportAspectRatio].Ratio,
			Width:       props.Width,
			Height:      props.Height,
		}
		ffmpegCmd = video.BuildFFmpegCommand(opts)
		if isMulti {
			ffmpegCmd = fmt.Sprintf("(%d sections) %s ...", len(sections), ffmpegCmd)
		}
	}

	var content string

	if m.exporting {
		title := titleStyle.Render("Exporting")

		w := 50
		filled := int(m.exportProgress * float64(w))
		empty := w - filled
		progressBar := dimStyle.Render("[") +
			accentStyle.Render(strings.Repeat("=", filled)) +
			dimStyle.Render(strings.Repeat("-", empty)+"]")
		percent := valueStyle.Render(fmt.Sprintf("%3.0f%%", m.exportProgress*100))

		content = title + "\n\n" +
			progressBar + " " + percent + "\n\n" +
			cmdStyle.Render(ffmpegCmd)
	} else {
		title := titleStyle.Render("Export Selection")
		if isMulti {
			title = titleStyle.Render(fmt.Sprintf("Export %d Sections", len(sections)))
		}

		var sectionList string
		if isMulti {
			fps := m.player.FPS()
			for i, sec := range sections {
				sectionList += dimStyle.Render(fmt.Sprintf("  #%d  %s → %s  (%s)",
					i+1,
					formatDuration(sec.In, fps),
					formatDuration(sec.Out, fps),
					formatDuration(sec.Duration(), fps),
				)) + "\n"
			}
			sectionList += "\n"
		}

		filename := m.exportFilename
		display := filename
		if m.exportFocusField == 0 {
			display = filename + dimStyle.Render("_")
		}
		if filename == "" && m.exportFocusField != 0 {
			display = dimStyle.Render("(auto)")
		}

		fn := "  "
		ar := "  "
		mode := "  "
		if m.exportFocusField == 0 {
			fn = accentStyle.Render("> ")
		} else if m.exportFocusField == 1 {
			ar = accentStyle.Render("> ")
		} else {
			mode = accentStyle.Render("> ")
		}

		var ratioLine string
		for i, opt := range video.AspectRatioOptions {
			if i == m.exportAspectRatio {
				ratioLine += accentStyle.Render("["+opt.Label+"]") + " "
			} else {
				ratioLine += dimStyle.Render(" "+opt.Label) + "  "
			}
		}

		modeOptions := []string{"Separate Clips", "Single Clip"}
		var modeLine string
		for i, opt := range modeOptions {
			if i == m.exportMode {
				modeLine += accentStyle.Render("["+opt+"]") + " "
			} else {
				modeLine += dimStyle.Render(" "+opt) + "  "
			}
		}

		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
		footer := keyStyle.Render("↑↓") + labelStyle.Render(" field  ") +
			keyStyle.Render("←→") + labelStyle.Render(" select  ") +
			keyStyle.Render("Enter") + labelStyle.Render(" export  ") +
			keyStyle.Render("Esc") + labelStyle.Render(" cancel")

		fields := fn + labelStyle.Render("Filename  ") + valueStyle.Render(display) + "\n\n" +
			ar + labelStyle.Render("Aspect    ") + ratioLine
		if isMulti {
			fields += "\n\n" + mode + labelStyle.Render("Mode      ") + modeLine
		}

		content = title + "\n\n" +
			sectionList +
			fields + "\n\n" +
			cmdStyle.Render(ffmpegCmd) + "\n\n" +
			footer
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 3).
		Width(75).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
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
