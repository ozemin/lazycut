package video

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type ChafaConfig struct {
	WorkFactor float32
}

var defaultChafaConfig = ChafaConfig{
	WorkFactor: 0.25,
}

func (c ChafaConfig) Render(pixels []byte, pixW, pixH, termW, termH int) (string, error) {
	if len(pixels) != pixW*pixH*rgbaChannels {
		return "", fmt.Errorf("pixel buffer size mismatch: got %d, want %d", len(pixels), pixW*pixH*rgbaChannels)
	}

	var buf bytes.Buffer
	buf.Grow(len("P6\n") + 20 + pixW*pixH*3)
	fmt.Fprintf(&buf, "P6\n%d %d\n255\n", pixW, pixH)
	for i := 0; i < len(pixels); i += rgbaChannels {
		buf.WriteByte(pixels[i])
		buf.WriteByte(pixels[i+1])
		buf.WriteByte(pixels[i+2])
	}

	work := int(c.WorkFactor*8) + 1
	if work < 1 {
		work = 1
	} else if work > 9 {
		work = 9
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
		"--dither", "noise",
		"--color-extractor", "median",
		"--optimize", "5",
		"--format", "symbols",
		"--probe", "off",
		"--work", fmt.Sprintf("%d", work),
		"--animate", "off",
		"-",
	)
	cmd.Stdin = &buf

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("chafa: %w", err)
	}
	return string(out), nil
}
