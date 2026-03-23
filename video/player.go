package video

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type TrimState struct {
	InPoint  *time.Duration
	OutPoint *time.Duration
}

func (t *TrimState) SetIn(pos time.Duration) {
	if t.OutPoint != nil && pos > *t.OutPoint {
		t.OutPoint = nil
	}
	t.InPoint = &pos
}

func (t *TrimState) SetOut(pos time.Duration) {
	if t.InPoint != nil && pos < *t.InPoint {
		t.InPoint = nil
	}
	t.OutPoint = &pos
}

func (t *TrimState) Clear() {
	t.InPoint = nil
	t.OutPoint = nil
}

func (t *TrimState) IsComplete() bool {
	return t.InPoint != nil && t.OutPoint != nil
}

func (t *TrimState) Duration() time.Duration {
	if !t.IsComplete() {
		return 0
	}
	return *t.OutPoint - *t.InPoint
}

type Player struct {
	path       string
	duration   time.Duration
	position   time.Duration
	playing    bool
	fps        int
	width      int
	height     int
	properties *VideoProperties
	quality    QualityPreset

	mu            sync.Mutex
	currentFrame  string
	stopChan      chan struct{}
	stream        *FrameStream
	frameInterval time.Duration

	// Optimization: Frame cache
	cache *FrameCache

	// Audio playback
	audioPlayer *AudioPlayer

	Trim TrimState
}

func NewPlayer(path string) (*Player, error) {
	props, err := GetVideoProperties(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	return &Player{
		path:        path,
		duration:    props.Duration,
		position:    0,
		playing:     false,
		fps:         int(props.FPS),
		properties:  props,
		quality:     QualityHigh,
		stopChan:    make(chan struct{}),
		cache:       NewFrameCache(DefaultCacheCapacity, props.FPS),
		audioPlayer: NewAudioPlayer(path),
	}, nil
}

func (p *Player) SetSize(width, height int) {
	p.mu.Lock()
	oldWidth, oldHeight := p.width, p.height
	p.width = width
	p.height = height
	pos := p.position
	quality := p.quality
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 && (width != oldWidth || height != oldHeight) {
		p.renderFrameCached(pos, width, height, quality)
	}
}

func (p *Player) Play() error {
	p.mu.Lock()
	if p.playing {
		p.mu.Unlock()
		return nil
	}
	p.playing = true
	p.stopChan = make(chan struct{})
	previewFPS := p.properties.PreviewFPS()
	if previewFPS > 0 {
		p.frameInterval = time.Second / time.Duration(previewFPS)
	} else {
		p.frameInterval = time.Second / 24
	}
	pos := p.position
	p.mu.Unlock()

	// Start audio playback
	p.audioPlayer.Start(pos.Seconds())

	go p.playbackLoop()
	return nil
}

func (p *Player) Pause() {
	p.mu.Lock()
	if !p.playing {
		p.mu.Unlock()
		return
	}
	p.playing = false
	close(p.stopChan)
	stream := p.stream
	p.stream = nil
	pos := p.position
	width, height := p.width, p.height
	quality := p.quality
	p.mu.Unlock()

	// Stop audio playback
	p.audioPlayer.Stop()

	if stream != nil {
		stream.Close()
	}

	if width > 0 && height > 0 {
		p.renderFrameCached(pos, width, height, quality)
	}
}

func (p *Player) Toggle() error {
	p.mu.Lock()
	playing := p.playing
	p.mu.Unlock()

	if playing {
		p.Pause()
		return nil
	}
	return p.Play()
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position
}

func (p *Player) Seek(position time.Duration) {
	p.mu.Lock()
	if position < 0 {
		position = 0
	}
	if position > p.duration {
		position = p.duration
	}
	p.position = position
	width, height := p.width, p.height
	quality := p.quality
	playing := p.playing
	stream := p.stream
	p.stream = nil
	p.mu.Unlock()

	// Stop audio during seek
	p.audioPlayer.Stop()

	if playing && stream != nil {
		stream.Close()
	}

	// Restart audio from new position if playing
	if playing {
		p.audioPlayer.Start(position.Seconds())
	}

	if !playing && width > 0 && height > 0 {
		p.renderFrameCached(position, width, height, quality)
	}
}

func (p *Player) FPS() int {
	return p.fps
}

func (p *Player) Path() string {
	return p.path
}

func (p *Player) Duration() time.Duration {
	return p.duration
}

func (p *Player) Properties() *VideoProperties {
	return p.properties
}

func (p *Player) CurrentFrame() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentFrame
}

func (p *Player) Quality() QualityPreset {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.quality
}

func (p *Player) CycleQuality() QualityPreset {
	p.mu.Lock()
	p.quality = p.quality.Next()
	newQuality := p.quality
	pos := p.position
	width, height := p.width, p.height
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 {
		p.renderFrameCached(pos, width, height, newQuality)
	}
	return newQuality
}

func (p *Player) Close() {
	p.Pause()
	p.audioPlayer.Stop()
}

func (p *Player) ToggleMute() {
	p.audioPlayer.ToggleMute()
}

func (p *Player) IsMuted() bool {
	return p.audioPlayer.IsMuted()
}

func (p *Player) playbackLoop() {
	var currentStream *FrameStream
	defer func() {
		if currentStream != nil {
			currentStream.Close()
		}
	}()

	for {
		select {
		case <-p.stopChan:
			return
		default:
		}

		p.mu.Lock()
		if !p.playing {
			p.mu.Unlock()
			return
		}
		width := p.width
		height := p.height
		quality := p.quality
		pos := p.position
		frameInterval := p.frameInterval
		fps := p.fps
		p.mu.Unlock()

		if width <= 0 || height <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if fps <= 0 {
			fps = 24
		}

		// Use preview parameters for smooth playback
		previewFPS := p.properties.PreviewFPS()
		videoWidth := p.properties.Width

		if currentStream == nil || currentStream.NeedsRestart(width, height, previewFPS, videoWidth) {
			if currentStream != nil {
				currentStream.Close()
			}
			stream, err := NewFrameStream(p.path, pos, width, height, previewFPS, videoWidth)
			if err != nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			currentStream = stream
			p.mu.Lock()
			p.stream = stream
			p.mu.Unlock()
		}

		frameBytes, err := currentStream.NextFrame()
		if err != nil {
			currentStream.Close()
			currentStream = nil
			continue
		}

		frame, err := p.renderFrameFromBytes(frameBytes, width, height, quality)
		if err != nil {
			continue
		}

		p.cache.Put(pos, width, height, quality, frame)
		p.mu.Lock()
		if !p.playing {
			p.mu.Unlock()
			return
		}
		p.currentFrame = frame
		p.position += frameInterval
		if p.position >= p.duration {
			p.position = p.duration
			p.playing = false
			if currentStream != nil {
				currentStream.Close()
				currentStream = nil
				p.stream = nil
			}
			p.mu.Unlock()
			// Stop audio when playback ends
			p.audioPlayer.Stop()
			return
		}
		p.mu.Unlock()
	}
}

// renderFrameCached renders a frame using cache
func (p *Player) renderFrameCached(position time.Duration, width, height int, quality QualityPreset) {
	// Check cache first
	if frame, ok := p.cache.Get(position, width, height, quality); ok {
		p.mu.Lock()
		p.currentFrame = frame
		p.mu.Unlock()
		return
	}

	// Cache miss - render
	frame, err := p.renderFrame(position, width, height)
	if err != nil {
		return
	}
	p.cache.Put(position, width, height, quality, frame)
	p.mu.Lock()
	p.currentFrame = frame
	p.mu.Unlock()
}

func (p *Player) renderFrame(position time.Duration, width, height int) (string, error) {
	p.mu.Lock()
	config := ChafaPresets[p.quality]
	p.mu.Unlock()

	// Build filter chain with preview parameters
	previewFPS := p.properties.PreviewFPS()
	var filters []string
	if p.properties.NeedsScaling() {
		filters = append(filters, "scale=1920:-1:flags=fast_bilinear")
	}
	filters = append(filters, fmt.Sprintf("fps=%d", previewFPS))

	ffmpegCmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", position.Seconds()),
		"-i", p.path,
		"-vf", strings.Join(filters, ","),
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "bmp",
		"-loglevel", "error",
		"-",
	)

	chafaArgs := config.BuildArgs(width, height)
	chafaCmd := exec.Command("chafa", chafaArgs...)

	pipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	chafaCmd.Stdin = pipe

	var chafaOut bytes.Buffer
	chafaCmd.Stdout = &chafaOut

	if err := chafaCmd.Start(); err != nil {
		return "", err
	}
	if err := ffmpegCmd.Run(); err != nil {
		return "", err
	}
	if err := chafaCmd.Wait(); err != nil {
		return "", err
	}

	return chafaOut.String(), nil
}

func (p *Player) renderFrameFromBytes(frame []byte, width, height int, quality QualityPreset) (string, error) {
	config := ChafaPresets[quality]
	chafaArgs := config.BuildArgs(width, height)
	chafaCmd := exec.Command("chafa", chafaArgs...)

	chafaCmd.Stdin = bytes.NewReader(frame)

	var chafaOut bytes.Buffer
	chafaCmd.Stdout = &chafaOut

	if err := chafaCmd.Run(); err != nil {
		return "", err
	}

	return chafaOut.String(), nil
}

func getInstallCommand(packageName string) string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("brew install %s", packageName)
	case "linux":
		// Detect Linux package manager
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			return fmt.Sprintf("sudo apt install %s", packageName)
		}
		if _, err := os.Stat("/etc/redhat-release"); err == nil {
			return fmt.Sprintf("sudo dnf install %s", packageName)
		}
		// Fallback for unknown Linux distro
		return fmt.Sprintf("sudo apt install %s (Debian/Ubuntu) or sudo dnf install %s (Fedora/RHEL)", packageName, packageName)
	case "windows":
		// Map package names for Windows winget
		wingetPackages := map[string]string{
			"ffmpeg": "Gyan.FFmpeg",
			"chafa":  "chafa",
		}
		if wingetName, ok := wingetPackages[packageName]; ok {
			return fmt.Sprintf("winget install %s", wingetName)
		}
		return fmt.Sprintf("winget install %s", packageName)
	default:
		// Unknown OS, show all options
		return fmt.Sprintf("brew install %s (macOS) or sudo apt install %s (Linux)", packageName, packageName)
	}
}

func CheckDependencies() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found. Install: %s", getInstallCommand("ffmpeg"))
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found. Install: %s", getInstallCommand("ffmpeg"))
	}
	if _, err := exec.LookPath("ffplay"); err != nil {
		return fmt.Errorf("ffplay not found. Install: %s", getInstallCommand("ffmpeg"))
	}
	if _, err := exec.LookPath("chafa"); err != nil {
		return fmt.Errorf("chafa not found. Install: %s", getInstallCommand("chafa"))
	}
	return nil
}
