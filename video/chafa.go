package video

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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

	args := []string{
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
	}
	if chafaProbesTerminal() {
		// Otherwise chafa opens /dev/tty and waits for a query reply that Bubble
		// Tea's raw-mode stdin reader swallows, hanging every render.
		args = append(args, "--probe", "off")
	}
	args = append(args, "-")

	cmd := exec.Command("chafa", args...)
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

// chafa has no PPM loader on Linux, where the raw PPM this replaced only ever
// decoded through macOS' CoreGraphics fallback. The PNG loader is bundled in
// every build, and uncompressed PNG also encodes 5-8x faster than PPM did.
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

// Probing arrived in chafa 1.16 and older builds abort on the unknown option,
// so --probe cannot be passed unconditionally. An unreadable version counts as
// probing: a rejected flag surfaces as a chafa error, a missed one hangs.
var chafaProbesTerminal = sync.OnceValue(func() bool {
	out, err := exec.Command("chafa", "--version").Output()
	if err != nil {
		return true
	}
	major, minor, ok := parseChafaVersion(out)
	if !ok {
		return true
	}
	return major > 1 || (major == 1 && minor >= 16)
})

func parseChafaVersion(versionOutput []byte) (major, minor int, ok bool) {
	line, _, _ := bytes.Cut(versionOutput, []byte("\n"))
	fields := bytes.Fields(line)
	if len(fields) < 3 {
		return 0, 0, false
	}
	parts := strings.SplitN(string(fields[2]), ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}
