package video

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os/exec"
	"testing"
)

func testPixels(w, h int) []byte {
	pixels := make([]byte, w*h*rgbaChannels)
	for i := 0; i < len(pixels); i += rgbaChannels {
		pixels[i] = byte(i)
		pixels[i+1] = byte(i >> 8)
		pixels[i+2] = byte(i >> 16)
		pixels[i+3] = 0xff
	}
	return pixels
}

func TestEncodeFramePreservesPixels(t *testing.T) {
	const w, h = 8, 4
	pixels := testPixels(w, h)

	data, err := encodeFrame(pixels, w, h)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := image.NewNRGBA(img.Bounds())
	draw.Draw(got, got.Bounds(), img, image.Point{}, draw.Src)
	if !bytes.Equal(got.Pix, pixels) {
		t.Errorf("round-trip pixels differ, got %dx%d", got.Rect.Dx(), got.Rect.Dy())
	}
}

func TestParseChafaVersion(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		major, minor int
		ok           bool
	}{
		{"1.18.2", "Chafa version 1.18.2\nLoaders:  AVIF GIF JPEG PNG QOI\n", 1, 18, true},
		{"1.12.4", "Chafa version 1.12.4\n", 1, 12, true},
		{"two components", "Chafa version 2.0\n", 2, 0, true},
		{"empty", "", 0, 0, false},
		{"unexpected prefix", "chafa\n", 0, 0, false},
		{"non-numeric", "Chafa version x.y\n", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, ok := parseChafaVersion([]byte(tt.output))
			if major != tt.major || minor != tt.minor || ok != tt.ok {
				t.Errorf("got (%d, %d, %t), want (%d, %d, %t)", major, minor, ok, tt.major, tt.minor, tt.ok)
			}
		})
	}
}

func TestRenderChafaRejectsSizeMismatch(t *testing.T) {
	if _, err := renderChafa(testPixels(8, 4), 8, 8, 20, 10); err == nil {
		t.Error("expected error for mismatched pixel buffer")
	}
}

func TestRenderChafaRendersFrame(t *testing.T) {
	if _, err := exec.LookPath("chafa"); err != nil {
		t.Skip("chafa not installed")
	}

	out, err := renderChafa(testPixels(64, 32), 64, 32, 20, 10)
	if err != nil {
		t.Fatalf("renderChafa: %v", err)
	}
	if out == "" {
		t.Error("renderChafa returned empty output")
	}
}
