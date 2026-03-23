package video

import (
	"fmt"
	"os"
	"strconv"
)

type QualityPreset int

const (
	QualityLow QualityPreset = iota
	QualityHigh
)

func (q QualityPreset) String() string {
	switch q {
	case QualityLow:
		return "LOW"
	case QualityHigh:
		return "HIGH"
	}
	return "UNKNOWN"
}

func (q QualityPreset) Next() QualityPreset {
	return (q + 1) % 2
}

type ChafaConfig struct {
	Colors         string
	Optimize       int
	Work           int
	ColorSpace     string
	Dither         string
	ColorExtractor string
}

var ChafaPresets = map[QualityPreset]ChafaConfig{
	QualityLow: {
		Colors: "256", Optimize: 9, Work: 1,
		ColorSpace: "rgb", Dither: "none", ColorExtractor: "average",
	},
	QualityHigh: {
		Colors: "full", Optimize: 1, Work: 9,
		ColorSpace: "din99d", Dither: "diffusion", ColorExtractor: "median",
	},
}

func (c ChafaConfig) BuildArgs(width, height int) []string {
	colors := c.Colors
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		colors = "none"
	}
	return []string{
		"--probe=off",
		"--format=symbols",
		"--size", fmt.Sprintf("%dx%d", width, height),
		"--colors", colors,
		"-O", strconv.Itoa(c.Optimize),
		"--work", strconv.Itoa(c.Work),
		"--color-space", c.ColorSpace,
		"--dither", c.Dither,
		"--color-extractor", c.ColorExtractor,
		"-",
	}
}
