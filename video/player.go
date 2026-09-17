package video

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

const frameBufferSize = 15

type bufferedFrame struct {
	frame string
	pos   time.Duration
}

// session owns a single playback run. Play and Seek install a fresh one and
// cancel the previous, and the render/display goroutines only ever touch the
// session they were handed — so a retired run can never observe or close the
// live run's channel.
type session struct {
	ctx    context.Context
	cancel context.CancelFunc
	frames chan bufferedFrame
}

func newSession() *session {
	ctx, cancel := context.WithCancel(context.Background())
	return &session{
		ctx:    ctx,
		cancel: cancel,
		frames: make(chan bufferedFrame, frameBufferSize),
	}
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
	session       *session
	frameInterval time.Duration

	cache *FrameCache

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
	p.frameInterval = time.Second / time.Duration(p.previewFPS)
	pos := p.position
	prev := p.session
	s := newSession()
	p.session = s
	p.mu.Unlock()

	if prev != nil {
		prev.cancel()
	}

	p.audioPlayer.Start(pos.Seconds())

	go p.renderLoop(s)
	go p.displayLoop(s)
	return nil
}

func (p *Player) Pause() {
	p.mu.Lock()
	if !p.playing {
		p.mu.Unlock()
		return
	}
	p.playing = false
	s := p.session
	pos := p.position
	width, height := p.width, p.height
	p.mu.Unlock()

	if s != nil {
		s.cancel()
	}

	p.audioPlayer.Stop()

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
	position = max(position, 0)
	position = min(position, p.duration)
	p.position = position
	width, height := p.width, p.height
	playing := p.playing
	var prev, s *session
	if playing {
		prev, s = p.session, newSession()
		p.session = s
	}
	p.mu.Unlock()

	if prev != nil {
		prev.cancel()
	}

	p.audioPlayer.Stop()

	// s is non-nil only when playback was running, so it doubles as that check.
	if s != nil {
		p.audioPlayer.Start(position.Seconds())
		go p.renderLoop(s)
		go p.displayLoop(s)
		return
	}

	if width > 0 && height > 0 {
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
	p.mu.Lock()
	s := p.session
	p.session = nil
	p.mu.Unlock()
	if s != nil {
		s.cancel()
	}
	p.audioPlayer.Stop()
}

func (p *Player) ToggleMute() {
	p.audioPlayer.ToggleMute()
}

func (p *Player) IsMuted() bool {
	return p.audioPlayer.IsMuted()
}

// renderLoop is the sole closer of s.frames, which is how displayLoop learns
// the run is over.
func (p *Player) renderLoop(s *session) {
	var currentStream *FrameStream
	var renderPos time.Duration

	defer func() {
		if currentStream != nil {
			currentStream.Close()
		}
		close(s.frames)
	}()

	// A source with no video stream never yields a frame, and the loop below
	// would respawn ffmpeg as fast as it can fail.
	if p.properties.Width <= 0 || p.properties.Height <= 0 {
		<-s.ctx.Done()
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		p.mu.Lock()
		width := p.width
		height := p.height
		fps := p.fps
		frameInterval := p.frameInterval
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

		if currentStream == nil || currentStream.NeedsRestart(width, height, previewFPS, videoWidth) {
			if currentStream != nil {
				currentStream.Close()
			}
			p.mu.Lock()
			renderPos = p.position
			p.mu.Unlock()

			stream, err := NewFrameStream(s.ctx, p.path, renderPos, width, height, previewFPS, videoWidth, videoHeight)
			if err != nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			currentStream = stream
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
		case s.frames <- bufferedFrame{frame: frame, pos: renderPos}:
		case <-s.ctx.Done():
			return
		}

		renderPos += frameInterval
	}
}

func (p *Player) displayLoop(s *session) {
	p.mu.Lock()
	frameInterval := p.frameInterval
	p.mu.Unlock()

	for {
		var item bufferedFrame
		select {
		case f, ok := <-s.frames:
			if !ok {
				// Producer is gone. Only report end of video if this session is
				// still the live one — otherwise a Pause or Seek retired it.
				p.mu.Lock()
				ended := p.session == s && p.playing
				if ended {
					p.position = p.duration
					p.playing = false
				}
				p.mu.Unlock()
				if ended {
					p.audioPlayer.Stop()
				}
				return
			}
			item = f
		case <-s.ctx.Done():
			return
		}

		displayStart := time.Now()

		p.mu.Lock()
		p.currentFrame = item.frame
		p.position = item.pos
		p.mu.Unlock()

		elapsed := time.Since(displayStart)
		if sleep := frameInterval - elapsed; sleep > 0 {
			select {
			case <-time.After(sleep):
			case <-s.ctx.Done():
				return
			}
		}
	}
}

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
	return renderChafa(pixels, pixW, pixH, termW, termH)
}
