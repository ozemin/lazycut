package video

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
)

func renderChafa(pixels []byte, pixW, pixH, termW, termH int) (string, error) {
	if len(pixels) != pixW*pixH*rgbaChannels {
		return "", fmt.Errorf("pixel buffer size mismatch: got %d, want %d", len(pixels), pixW*pixH*rgbaChannels)
	}

	encoded, err := encodeFrame(pixels, pixW, pixH)
	if err != nil {
		return "", err
	}

	colors := "full"
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		colors = "2"
	}

	cmd := exec.Command("chafa",
		"--size", fmt.Sprintf("%dx%d", termW, termH),
		"--symbols", "block+border+space",
		"--colors", colors,
		"--color-space", "din99d",
		"--dither", "ordered",
		"--color-extractor", "median",
		"--optimize", "5",
		"--format", "symbols",
		"--work", "3",
		"--animate", "off",
		"-",
	)
	cmd.Stdin = bytes.NewReader(encoded)

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("chafa: %w: %s", err, bytes.TrimSpace(exitErr.Stderr))
		}
		return "", fmt.Errorf("chafa: %w", err)
	}
	return string(out), nil
}

// Uncompressed PNG: 5-8x cheaper per frame than the raw PPM it replaced, and
// chafa's PNG loader is bundled (LodePNG) rather than an optional dependency.
func encodeFrame(pixels []byte, pixW, pixH int) ([]byte, error) {
	img := &image.NRGBA{
		Pix:    pixels,
		Stride: pixW * rgbaChannels,
		Rect:   image.Rect(0, 0, pixW, pixH),
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode frame: %w", err)
	}
	return buf.Bytes(), nil
}
