package video

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MultiExportOptions struct {
	Input       string
	Output      string
	Sections    []Section
	AspectRatio AspectRatio
	Width       int
	Height      int
}

// ExportSeparate exports each section as a numbered individual file.
func ExportSeparate(opts MultiExportOptions, progress chan<- float64) ([]string, error) {
	defer close(progress)

	total := len(opts.Sections)
	if total == 0 {
		return nil, fmt.Errorf("no sections to export")
	}

	base, ext := outputBaseAndExt(opts.Input, opts.Output)
	dir := filepath.Dir(opts.Input)
	if opts.Output != "" && filepath.IsAbs(opts.Output) {
		dir = filepath.Dir(opts.Output)
	}

	var outputs []string
	for i, sec := range opts.Sections {
		numberedOut := filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, i+1, ext))

		sectionProgress := make(chan float64, 100)
		done := make(chan struct{})
		go func(ch <-chan float64, idx int) {
			defer close(done)
			for p := range ch {
				combined := (float64(idx) + p) / float64(total)
				select {
				case progress <- combined:
				default:
				}
			}
		}(sectionProgress, i)

		singleOpts := ExportOptions{
			Input:       opts.Input,
			Output:      numberedOut,
			InPoint:     sec.In,
			OutPoint:    sec.Out,
			AspectRatio: opts.AspectRatio,
			Width:       opts.Width,
			Height:      opts.Height,
		}
		out, err := ExportWithProgress(singleOpts, sectionProgress)
		<-done // wait for goroutine to finish forwarding before continuing
		if err != nil {
			return outputs, fmt.Errorf("section %d: %w", i+1, err)
		}
		outputs = append(outputs, out)
	}

	return outputs, nil
}

// ExportConcatenated exports all sections into a single concatenated file.
// It uses a two-pass approach: export each section to a temp file, then concat.
func ExportConcatenated(opts MultiExportOptions, progress chan<- float64) (string, error) {
	defer close(progress)

	total := len(opts.Sections)
	if total == 0 {
		return "", fmt.Errorf("no sections to export")
	}

	ext := filepath.Ext(opts.Input)
	if opts.Output != "" && filepath.Ext(opts.Output) != "" {
		ext = filepath.Ext(opts.Output)
	}

	// Pass 1: export each section to a temp file (90% of progress)
	var tempFiles []string
	for i, sec := range opts.Sections {
		tmpFile, err := os.CreateTemp("", fmt.Sprintf("lazycut_section_%03d_*%s", i+1, ext))
		if err != nil {
			cleanupFiles(tempFiles)
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		tempFiles = append(tempFiles, tmpPath)

		sectionProgress := make(chan float64, 100)
		go func(ch <-chan float64, idx int) {
			for p := range ch {
				combined := (float64(idx) + p) / float64(total) * 0.9
				select {
				case progress <- combined:
				default:
				}
			}
		}(sectionProgress, i)

		singleOpts := ExportOptions{
			Input:       opts.Input,
			Output:      tmpPath,
			InPoint:     sec.In,
			OutPoint:    sec.Out,
			AspectRatio: opts.AspectRatio,
			Width:       opts.Width,
			Height:      opts.Height,
		}
		if _, err := ExportWithProgress(singleOpts, sectionProgress); err != nil {
			cleanupFiles(tempFiles)
			return "", fmt.Errorf("section %d: %w", i+1, err)
		}
	}

	// Pass 2: write concat list and merge (final 10% of progress)
	listFile, err := os.CreateTemp("", "lazycut_concat_*.txt")
	if err != nil {
		cleanupFiles(tempFiles)
		return "", fmt.Errorf("failed to create concat list: %w", err)
	}
	listPath := listFile.Name()
	defer os.Remove(listPath)

	for _, f := range tempFiles {
		fmt.Fprintf(listFile, "file '%s'\n", f)
	}
	listFile.Close()

	output := opts.Output
	if output == "" {
		output = generateOutputName(opts.Input)
	} else {
		dir := filepath.Dir(opts.Input)
		outExt := filepath.Ext(opts.Output)
		if outExt == "" {
			output = output + ext
		}
		if !filepath.IsAbs(output) {
			output = filepath.Join(dir, output)
		}
	}

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		output,
	)
	if err := cmd.Run(); err != nil {
		cleanupFiles(tempFiles)
		return "", fmt.Errorf("concat failed: %w", err)
	}

	cleanupFiles(tempFiles)
	progress <- 1.0
	return output, nil
}

func outputBaseAndExt(input, output string) (base, ext string) {
	ext = filepath.Ext(input)
	if output != "" {
		outExt := filepath.Ext(output)
		if outExt != "" {
			ext = outExt
			base = strings.TrimSuffix(filepath.Base(output), outExt)
		} else {
			base = filepath.Base(output)
		}
	} else {
		base = strings.TrimSuffix(filepath.Base(input), ext) + "_trimmed"
	}
	return base, ext
}

func cleanupFiles(paths []string) {
	for _, p := range paths {
		os.Remove(p)
	}
}

type AspectRatio int

const (
	AspectOriginal AspectRatio = iota
	Aspect16x9                 // Landscape
	Aspect9x16                 // Portrait/Mobile
	Aspect1x1                  // Square
	Aspect4x5                  // Instagram Portrait
)

var AspectRatioOptions = []struct {
	Ratio AspectRatio
	Label string
	W, H  int // ratio components (0,0 means original)
}{
	{AspectOriginal, "Original", 0, 0},
	{Aspect16x9, "16:9", 16, 9},
	{Aspect9x16, "9:16", 9, 16},
	{Aspect1x1, "1:1", 1, 1},
	{Aspect4x5, "4:5", 4, 5},
}

type ExportOptions struct {
	Input       string
	Output      string
	InPoint     time.Duration
	OutPoint    time.Duration
	AspectRatio AspectRatio
	Width       int
	Height      int
}

func BuildFFmpegCommand(opts ExportOptions) string {
	output := opts.Output
	if output == "" {
		output = generateOutputName(opts.Input)
	}
	duration := opts.OutPoint - opts.InPoint

	args := []string{"ffmpeg", "-y",
		"-ss", fmt.Sprintf("%.3f", opts.InPoint.Seconds()),
		"-i", filepath.Base(opts.Input),
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
	}

	if opts.AspectRatio != AspectOriginal && opts.Width > 0 && opts.Height > 0 {
		cropFilter := buildCropFilter(opts.Width, opts.Height, opts.AspectRatio)
		if cropFilter != "" {
			args = append(args, "-vf", cropFilter)
		}
	} else {
		args = append(args, "-c", "copy")
	}

	args = append(args, filepath.Base(output))
	return strings.Join(args, " ")
}

func ExportWithProgress(opts ExportOptions, progress chan<- float64) (string, error) {
	defer close(progress)

	output := opts.Output
	if output == "" {
		output = generateOutputName(opts.Input)
	} else {
		dir := filepath.Dir(opts.Input)
		ext := filepath.Ext(opts.Input)
		if filepath.Ext(output) == "" {
			output = output + ext
		}
		if !filepath.IsAbs(output) {
			output = filepath.Join(dir, output)
		}
	}
	duration := opts.OutPoint - opts.InPoint
	totalMicros := float64(duration.Microseconds())

	args := []string{"-y",
		"-ss", fmt.Sprintf("%.3f", opts.InPoint.Seconds()),
		"-i", opts.Input,
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
		"-progress", "pipe:2",
	}

	if opts.AspectRatio != AspectOriginal && opts.Width > 0 && opts.Height > 0 {
		cropFilter := buildCropFilter(opts.Width, opts.Height, opts.AspectRatio)
		if cropFilter != "" {
			args = append(args, "-vf", cropFilter)
		}
	} else {
		args = append(args, "-c", "copy")
	}

	args = append(args, output)

	cmd := exec.Command("ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_us=") {
			timeStr := strings.TrimPrefix(line, "out_time_us=")
			if micros, err := strconv.ParseFloat(timeStr, 64); err == nil && totalMicros > 0 {
				p := micros / totalMicros
				if p > 1.0 {
					p = 1.0
				}
				select {
				case progress <- p:
				default:
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}

	progress <- 1.0
	return output, nil
}

func buildCropFilter(srcW, srcH int, ratio AspectRatio) string {
	var targetW, targetH int
	for _, opt := range AspectRatioOptions {
		if opt.Ratio == ratio {
			targetW, targetH = opt.W, opt.H
			break
		}
	}
	if targetW == 0 || targetH == 0 {
		return ""
	}

	srcRatio := float64(srcW) / float64(srcH)
	targetRatio := float64(targetW) / float64(targetH)

	var cropW, cropH int
	if srcRatio > targetRatio {
		cropH = srcH
		cropW = int(float64(srcH) * targetRatio)
	} else {
		cropW = srcW
		cropH = int(float64(srcW) / targetRatio)
	}

	// H.264 requires even dimensions
	cropW = cropW &^ 1
	cropH = cropH &^ 1

	return fmt.Sprintf("crop=%d:%d", cropW, cropH)
}

func generateOutputName(input string) string {
	dir := filepath.Dir(input)
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(filepath.Base(input), ext)

	trimmedPath := filepath.Join(dir, base+"_trimmed"+ext)
	if !fileExists(trimmedPath) {
		return trimmedPath
	}

	for i := 1; i <= 999; i++ {
		numberedPath := filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, i, ext))
		if !fileExists(numberedPath) {
			return numberedPath
		}
	}

	return filepath.Join(dir, base+"_trimmed_new"+ext)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
