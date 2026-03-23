package video

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FrameStream keeps a long-lived ffmpeg process that outputs scaled BMP frames.
type FrameStream struct {
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	cancel     context.CancelFunc
	width      int
	height     int
	videoWidth int
	targetFPS  int
	mu         sync.Mutex
}

func NewFrameStream(path string, start time.Duration, width, height, fps, videoWidth int) (*FrameStream, error) {
	if width <= 0 || height <= 0 || fps <= 0 {
		return nil, fmt.Errorf("invalid stream configuration")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build filter chain: scale (if needed) -> fps
	var filters []string
	if videoWidth > 1920 {
		filters = append(filters, "scale=1920:-1:flags=fast_bilinear")
	}
	filters = append(filters, fmt.Sprintf("fps=%d", fps))

	args := []string{
		"-re",
		"-ss", fmt.Sprintf("%.3f", start.Seconds()),
		"-i", path,
		"-vf", strings.Join(filters, ","),
		"-f", "image2pipe",
		"-vcodec", "bmp",
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
		cmd:        cmd,
		stdout:     stdout,
		cancel:     cancel,
		width:      width,
		height:     height,
		videoWidth: videoWidth,
		targetFPS:  fps,
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
func (s *FrameStream) NeedsRestart(width, height, fps, videoWidth int) bool {
	if s == nil {
		return true
	}
	return s.width != width || s.height != height ||
		s.targetFPS != fps || s.videoWidth != videoWidth
}

// NextFrame reads the next BMP frame from the stream.
func (s *FrameStream) NextFrame() ([]byte, error) {
	s.mu.Lock()
	stdout := s.stdout
	s.mu.Unlock()
	if stdout == nil {
		return nil, io.EOF
	}

	header := make([]byte, 14)
	if _, err := io.ReadFull(stdout, header); err != nil {
		return nil, err
	}
	if header[0] != 'B' || header[1] != 'M' {
		return nil, fmt.Errorf("invalid frame header")
	}
	frameSize := binary.LittleEndian.Uint32(header[2:6])
	if frameSize < 14 {
		return nil, fmt.Errorf("invalid frame size")
	}

	frame := make([]byte, frameSize)
	copy(frame, header)
	if _, err := io.ReadFull(stdout, frame[14:frameSize]); err != nil {
		return nil, err
	}
	return frame, nil
}
