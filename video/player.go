package video

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const frameBufferSize = 15

type bufferedFrame struct {
	frame   string
	pos     time.Duration
	version int
}

type Section struct {
	In  time.Duration
	Out time.Duration
}

func (s Section) Duration() time.Duration {
	return s.Out - s.In
}

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

	previewFPS int

	mu            sync.Mutex
	currentFrame  string
	stopChan      chan struct{}
	stream        *FrameStream
	frameInterval time.Duration
	frameBuffer   chan bufferedFrame
	seekVersion   int

	// Optimization: Frame cache
	cache *FrameCache

	// Audio playback
	audioPlayer *AudioPlayer

	Trim     TrimState
	Sections []Section
}

func (p *Player) AddSection(in, out time.Duration) {
	p.Sections = append(p.Sections, Section{In: in, Out: out})
}

func (p *Player) RemoveLastSection() {
	if len(p.Sections) > 0 {
		p.Sections = p.Sections[:len(p.Sections)-1]
	}
}

func (p *Player) ClearSections() {
	p.Sections = nil
}

func NewPlayer(path string, previewFPS int) (*Player, error) {
	props, err := GetVideoProperties(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	if previewFPS <= 0 {
		previewFPS = 24
	}

	return &Player{
		path:        path,
		duration:    props.Duration,
		position:    0,
		playing:     false,
		fps:         int(props.FPS),
		properties:  props,
		previewFPS:  previewFPS,
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
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 && (width != oldWidth || height != oldHeight) {
		p.renderFrameCached(pos, width, height)
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
	p.frameInterval = time.Second / time.Duration(p.previewFPS)
	pos := p.position
	p.frameBuffer = make(chan bufferedFrame, frameBufferSize)
	p.mu.Unlock()

	// Start audio playback
	p.audioPlayer.Start(pos.Seconds())

	go p.renderLoop()
	go p.displayLoop()
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
	p.mu.Unlock()

	// Stop audio playback
	p.audioPlayer.Stop()

	if stream != nil {
		stream.Close()
	}

	if width > 0 && height > 0 {
		p.renderFrameCached(pos, width, height)
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
	if p.playing {
		p.seekVersion++
	}
	width, height := p.width, p.height
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
		p.renderFrameCached(position, width, height)
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

// renderLoop is the producer goroutine: fetches and renders frames into frameBuffer ahead of playback.
func (p *Player) renderLoop() {
	var currentStream *FrameStream
	var renderPos time.Duration

	defer func() {
		if currentStream != nil {
			currentStream.Close()
		}
		close(p.frameBuffer)
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
		fps := p.fps
		frameInterval := p.frameInterval
		version := p.seekVersion
		streamWasReset := p.stream == nil && currentStream != nil
		p.mu.Unlock()

		if width <= 0 || height <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if fps <= 0 {
			fps = 24
		}

		previewFPS := p.previewFPS
		videoWidth := p.properties.Width
		videoHeight := p.properties.Height

		// Seek detected or first start
		if streamWasReset {
			currentStream.Close()
			currentStream = nil
		}

		if currentStream == nil || currentStream.NeedsRestart(width, height, previewFPS, videoWidth) {
			if currentStream != nil {
				currentStream.Close()
			}
			p.mu.Lock()
			renderPos = p.position
			p.mu.Unlock()

			stream, err := NewFrameStream(p.path, renderPos, width, height, previewFPS, videoWidth, videoHeight)
			if err != nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			currentStream = stream
			p.mu.Lock()
			p.stream = stream
			version = p.seekVersion
			p.mu.Unlock()
		}

		frameBytes, err := currentStream.NextFrame()
		if err != nil {
			currentStream.Close()
			currentStream = nil
			if renderPos >= p.duration-frameInterval {
				return // natural EOF
			}
			continue
		}

		pixW, pixH := currentStream.PixelDimensions()
		frame, err := p.renderFrameFromPixels(frameBytes, pixW, pixH, width, height)
		if err != nil {
			renderPos += frameInterval
			continue
		}

		p.cache.Put(renderPos, width, height, frame)

		select {
		case p.frameBuffer <- bufferedFrame{frame: frame, pos: renderPos, version: version}:
		case <-p.stopChan:
			return
		}

		renderPos += frameInterval
	}
}

// displayLoop is the consumer goroutine: reads pre-rendered frames and displays them at the correct rate.
func (p *Player) displayLoop() {
	p.mu.Lock()
	frameInterval := p.frameInterval
	p.mu.Unlock()

	for {
		var item bufferedFrame
		select {
		case f, ok := <-p.frameBuffer:
			if !ok {
				// Channel closed = end of video or stop
				p.mu.Lock()
				if p.playing {
					p.position = p.duration
					p.playing = false
				}
				p.mu.Unlock()
				p.audioPlayer.Stop()
				return
			}
			item = f
		case <-p.stopChan:
			return
		}

		displayStart := time.Now()

		p.mu.Lock()
		if item.version < p.seekVersion {
			// Stale frame from before a seek — discard
			p.mu.Unlock()
			continue
		}
		if !p.playing {
			p.mu.Unlock()
			return
		}
		p.currentFrame = item.frame
		p.position = item.pos
		p.mu.Unlock()

		// Sleep remaining time to maintain target frame rate
		elapsed := time.Since(displayStart)
		if sleep := frameInterval - elapsed; sleep > 0 {
			select {
			case <-time.After(sleep):
			case <-p.stopChan:
				return
			}
		}
	}
}

// renderFrameCached renders a frame using cache
func (p *Player) renderFrameCached(position time.Duration, width, height int) {
	if frame, ok := p.cache.Get(position, width, height); ok {
		p.mu.Lock()
		p.currentFrame = frame
		p.mu.Unlock()
		return
	}

	frame, err := p.renderFrame(position, width, height)
	if err != nil {
		return
	}
	p.cache.Put(position, width, height, frame)
	p.mu.Lock()
	p.currentFrame = frame
	p.mu.Unlock()
}

func (p *Player) renderFrame(position time.Duration, width, height int) (string, error) {
	pixW, pixH := computePixelDimensions(width, height, p.properties.Width, p.properties.Height)

	ffmpegCmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", position.Seconds()),
		"-i", p.path,
		"-vf", fmt.Sprintf("scale=%d:%d:flags=fast_bilinear", pixW, pixH),
		"-vframes", "1",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-loglevel", "error",
		"-",
	)

	var pixelData bytes.Buffer
	ffmpegCmd.Stdout = &pixelData
	if err := ffmpegCmd.Run(); err != nil {
		return "", err
	}

	return p.renderFrameFromPixels(pixelData.Bytes(), pixW, pixH, width, height)
}

func (p *Player) renderFrameFromPixels(pixels []byte, pixW, pixH, termW, termH int) (string, error) {
	return defaultChafaConfig.Render(pixels, pixW, pixH, termW, termH)
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
	return nil
}
