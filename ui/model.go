package ui

import (
	"fmt"
	"github.com/emin-ozata/lazycut/ui/panels"
	"github.com/emin-ozata/lazycut/video"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TickMsg time.Time

type ExportDoneMsg struct {
	Output string
	Err    error
}

type ExportProgressMsg float64

type Model struct {
	width        int
	height       int
	player       *video.Player
	preview      *panels.Preview
	properties   *panels.Properties
	timeline     *panels.Timeline
	ready        bool
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

	aspectMode int // 0: maintain aspect ratio (default), 1: stretch
	fullscreen bool

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
		ready:      false,
	}
}

func (m *Model) allSections() []video.Section {
	sections := make([]video.Section, len(m.player.Sections))
	copy(sections, m.player.Sections)
	return sections
}

func (m *Model) updatePlayerSize() {
	var dims PanelDimensions
	if m.fullscreen {
		dims = CalculateFullscreenDimensions(m.width, m.height)
	} else {
		dims = CalculatePanelDimensions(m.width, m.height)
	}
	w, h := dims.PreviewContentWidth, dims.PreviewContentHeight
	if m.aspectMode == 0 {
		props := m.player.Properties()
		w, h = computeAspectFitDimensions(w, h, props.Width, props.Height)
	}
	m.player.SetSize(w, h)
}

func computeAspectFitDimensions(termW, termH, videoW, videoH int) (int, int) {
	if videoW <= 0 || videoH <= 0 {
		return termW, termH
	}
	// Terminal cells are ~2x taller than wide
	cellRatio := 2.0
	videoAspect := float64(videoW) / float64(videoH) * cellRatio

	charW := termW
	charH := int(float64(charW) / videoAspect)

	if charH > termH {
		charH = termH
		charW = int(float64(charH) * videoAspect)
	}

	if charW < 1 {
		charW = 1
	}
	if charH < 1 {
		charH = 1
	}

	return charW, charH
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
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updatePlayerSize()
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
		m.exportStatus = ""

		pos := m.player.Position()
		fps := m.player.FPS()
		frameDuration := time.Second / time.Duration(fps)

		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.repeatCount = m.repeatCount*10 + int(msg.Runes[0]-'0')
			m.exportStatus = fmt.Sprintf("%dx", m.repeatCount)
			return m, nil
		case "0":
			if m.repeatCount == 0 {
				m.player.Seek(0)
				return m, nil
			}
			m.repeatCount *= 10
			m.exportStatus = fmt.Sprintf("%dx", m.repeatCount)
			return m, nil
		case "ctrl+c", "q":
			m.player.Close()
			return m, tea.Quit

		case " ":
			m.player.Toggle()
			return m, nil

		case "h":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "l":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "H":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n*5)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "L":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n*5)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "j":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n)*time.Minute)
			m.repeatCount = 0
			return m, nil

		case "k":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n)*time.Minute)
			m.repeatCount = 0
			return m, nil

		case "J":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n*5)*time.Minute)
			m.repeatCount = 0
			return m, nil

		case "K":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n*5)*time.Minute)
			m.repeatCount = 0
			return m, nil

		case ",":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos - time.Duration(n)*frameDuration)
			m.repeatCount = 0
			return m, nil

		case ".":
			n := m.repeatCount
			if n <= 0 {
				n = 1
			}
			m.player.Seek(pos + time.Duration(n)*frameDuration)
			m.repeatCount = 0
			return m, nil

		case "$", "G":
			m.player.Seek(m.player.Duration())
			m.repeatCount = 0
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

		case "X":
			m.player.RemoveLastSection()
			return m, nil

		case "p":
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

		case "P":
			if n := len(m.player.Sections); n > 0 {
				last := m.player.Sections[n-1]
				m.previewQueue = []video.Section{last}
				m.previewQueueIdx = 0
				m.player.Seek(last.In)
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

		case "a":
			m.aspectMode = (m.aspectMode + 1) % 2
			m.updatePlayerSize()
			return m, nil

		case "f":
			m.fullscreen = !m.fullscreen
			m.updatePlayerSize()
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

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.showHelpModal {
		return m.renderHelpModal()
	}
	if m.showExportModal {
		return m.renderExportModal()
	}

	if m.fullscreen {
		dims := CalculateFullscreenDimensions(m.width, m.height)
		if dims.PreviewContentWidth < minPanelWidth || dims.PreviewContentHeight < minPanelHeight {
			return lipgloss.NewStyle().
				Width(m.width).
				Height(m.height).
				Align(lipgloss.Center, lipgloss.Center).
				Render("Terminal too small")
		}
		previewContent := m.preview.Render(dims.PreviewContentWidth, dims.PreviewContentHeight)
		return renderPanel(previewContent, dims.PreviewWidth, dims.PreviewHeight)
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

	propertiesContent := m.properties.Render(dims.PropertiesContentWidth, dims.PropertiesContentHeight)
	propertiesPanel := renderPanel(propertiesContent, dims.PropertiesWidth, dims.PropertiesHeight)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, previewPanel, propertiesPanel)

	m.timeline.SetExportStatus(m.exportStatus)
	timelineContent := m.timeline.Render(dims.TimelineContentWidth, dims.TimelineContentHeight)
	timelinePanel := renderPanel(timelineContent, dims.TimelineWidth, dims.TimelineHeight)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, timelinePanel)
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
		if m.exportFocusField == 0 {
			if len(msg.Runes) > 0 {
				m.exportFilename += string(msg.Runes)
			}
			return m, nil
		}
		switch msg.String() {
		case "j":
			if m.exportFocusField < maxField {
				m.exportFocusField++
			}
		case "k":
			if m.exportFocusField > 0 {
				m.exportFocusField--
			}
		case "h":
			if m.exportFocusField == 1 {
				m.exportAspectRatio--
				if m.exportAspectRatio < 0 {
					m.exportAspectRatio = len(video.AspectRatioOptions) - 1
				}
			} else if m.exportFocusField == 2 {
				m.exportMode = (m.exportMode + 1) % 2
			}
		case "l":
			if m.exportFocusField == 1 {
				m.exportAspectRatio = (m.exportAspectRatio + 1) % len(video.AspectRatioOptions)
			} else if m.exportFocusField == 2 {
				m.exportMode = (m.exportMode + 1) % 2
			}
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
		kd("j / k", "Seek ±1 minute") + "\n" +
		kd("J / K", "Seek ±5 minutes") + "\n" +
		kd(", / .", "Seek ±1 frame") + "\n" +
		kd("0", "Go to start") + "\n" +
		kd("G / $", "Go to end") + "\n" +
		kd("5l 10.", "Vim-style counts") + "\n" +
		kd("m", "Toggle mute") + "\n" +
		kd("a", "Cycle aspect mode") + "\n" +
		kd("f", "Toggle fullscreen")

	trim := sectionStyle.Render("TRIM") + "\n" +
		kd("i", "Set in-point") + "\n" +
		kd("o", "Set out-point") + "\n" +
		kd("X", "Remove last section") + "\n" +
		kd("p", "Preview all sections") + "\n" +
		kd("P", "Preview last section") + "\n" +
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

		barWidth := 50
		filled := int(m.exportProgress * float64(barWidth))
		empty := barWidth - filled
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
			for i, sec := range sections {
				sectionList += dimStyle.Render(fmt.Sprintf("  #%d  %s → %s  (%s)",
					i+1,
					formatDuration(sec.In),
					formatDuration(sec.Out),
					formatDuration(sec.Duration()),
				)) + "\n"
			}
			sectionList += "\n"
		}

		filename := m.exportFilename
		filenameDisplay := filename
		if m.exportFocusField == 0 {
			filenameDisplay = filename + dimStyle.Render("_")
		}
		if filename == "" && m.exportFocusField != 0 {
			filenameDisplay = dimStyle.Render("(auto)")
		}

		fnIndicator := "  "
		arIndicator := "  "
		modeIndicator := "  "
		if m.exportFocusField == 0 {
			fnIndicator = accentStyle.Render("> ")
		} else if m.exportFocusField == 1 {
			arIndicator = accentStyle.Render("> ")
		} else {
			modeIndicator = accentStyle.Render("> ")
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

		fields := fnIndicator + labelStyle.Render("Filename  ") + valueStyle.Render(filenameDisplay) + "\n\n" +
			arIndicator + labelStyle.Render("Aspect    ") + ratioLine
		if isMulti {
			fields += "\n\n" + modeIndicator + labelStyle.Render("Mode      ") + modeLine
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

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}
