package video

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

const rgbaChannels = 4

// FrameStream keeps a long-lived ffmpeg process that outputs raw RGBA frames.
type FrameStream struct {
	cmd         *exec.Cmd
	stdout      io.ReadCloser
	cancel      context.CancelFunc
	width       int // terminal char width
	height      int // terminal char height
	pixelWidth  int // ffmpeg output pixel width
	pixelHeight int // ffmpeg output pixel height
	videoWidth  int // source video width (for NeedsRestart)
	targetFPS   int
	mu          sync.Mutex
}

// computePixelDimensions returns the pixel output size that preserves the
// video's aspect ratio while fitting within termW*pixelScale × termH*pixelScale.
func computePixelDimensions(termW, termH, videoW, videoH int) (int, int) {
	const pixelScale = 4
	maxW := termW * pixelScale
	maxH := termH * pixelScale

	if videoW <= 0 || videoH <= 0 {
		return maxW, maxH
	}

	pixW := maxW
	pixH := pixW * videoH / videoW
	if pixH > maxH {
		pixH = maxH
		pixW = pixH * videoW / videoH
	}

	// Round down to even (ffmpeg requirement for many pixel formats)
	pixW = pixW &^ 1
	pixH = pixH &^ 1
	if pixW < 2 {
		pixW = 2
	}
	if pixH < 2 {
		pixH = 2
	}
	return pixW, pixH
}

func NewFrameStream(path string, start time.Duration, termW, termH, fps, videoW, videoH int) (*FrameStream, error) {
	if termW <= 0 || termH <= 0 || fps <= 0 {
		return nil, fmt.Errorf("invalid stream configuration")
	}

	pixW, pixH := computePixelDimensions(termW, termH, videoW, videoH)

	ctx, cancel := context.WithCancel(context.Background())

	filter := fmt.Sprintf("scale=%d:%d:flags=fast_bilinear,fps=%d", pixW, pixH, fps)

	args := []string{
		"-ss", fmt.Sprintf("%.3f", start.Seconds()),
		"-i", path,
		"-vf", filter,
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-loglevel", "error",
		"-",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	return &FrameStream{
		cmd:         cmd,
		stdout:      stdout,
		cancel:      cancel,
		width:       termW,
		height:      termH,
		pixelWidth:  pixW,
		pixelHeight: pixH,
		videoWidth:  videoW,
		targetFPS:   fps,
	}, nil
}

// Close stops the ffmpeg process.
func (s *FrameStream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	s.cancel = nil
	s.cmd = nil
	if s.stdout != nil {
		_ = s.stdout.Close()
		s.stdout = nil
	}
}

// NeedsRestart checks if the stream configuration matches the desired parameters.
func (s *FrameStream) NeedsRestart(termW, termH, fps, videoW int) bool {
	if s == nil {
		return true
	}
	return s.width != termW || s.height != termH ||
		s.targetFPS != fps || s.videoWidth != videoW
}

// PixelDimensions returns the pixel width and height of each frame.
func (s *FrameStream) PixelDimensions() (int, int) {
	return s.pixelWidth, s.pixelHeight
}

// NextFrame reads the next raw RGBA frame from the stream.
func (s *FrameStream) NextFrame() ([]byte, error) {
	s.mu.Lock()
	stdout := s.stdout
	pixW := s.pixelWidth
	pixH := s.pixelHeight
	s.mu.Unlock()
	if stdout == nil {
		return nil, io.EOF
	}

	frameSize := pixW * pixH * rgbaChannels
	frame := make([]byte, frameSize)
	if _, err := io.ReadFull(stdout, frame); err != nil {
		return nil, err
	}
	return frame, nil
}
