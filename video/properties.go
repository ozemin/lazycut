package video

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type VideoProperties struct {
	Width    int
	Height   int
	Codec    string
	FPS      float64
	Bitrate  int64
	FileSize int64
	Duration time.Duration
}

type ffprobeOutput struct {
	Streams []struct {
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		CodecName  string `json:"codec_name"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

func GetVideoProperties(path string) (*VideoProperties, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration,size,bit_rate",
		"-show_entries", "stream=width,height,codec_name,r_frame_rate",
		"-of", "json",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	props := &VideoProperties{}

	for _, stream := range probe.Streams {
		if stream.Width > 0 && stream.Height > 0 {
			props.Width = stream.Width
			props.Height = stream.Height
			props.Codec = stream.CodecName
			props.FPS = parseFrameRate(stream.RFrameRate)
			break
		}
	}

	if probe.Format.Duration != "" {
		seconds, _ := strconv.ParseFloat(probe.Format.Duration, 64)
		props.Duration = time.Duration(seconds * float64(time.Second))
	}

	if probe.Format.Size != "" {
		props.FileSize, _ = strconv.ParseInt(probe.Format.Size, 10, 64)
	}

	if probe.Format.BitRate != "" {
		props.Bitrate, _ = strconv.ParseInt(probe.Format.BitRate, 10, 64)
	}

	if props.FileSize == 0 {
		if info, err := os.Stat(path); err == nil {
			props.FileSize = info.Size()
		}
	}

	return props, nil
}

func parseFrameRate(s string) float64 {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 0
	}
	return num / den
}

func (p *VideoProperties) Resolution() string {
	return fmt.Sprintf("%dx%d", p.Width, p.Height)
}

func (p *VideoProperties) FormattedFPS() string {
	return fmt.Sprintf("%.2f fps", p.FPS)
}

func (p *VideoProperties) FormattedBitrate() string {
	if p.Bitrate == 0 {
		return "N/A"
	}
	mbps := float64(p.Bitrate) / 1_000_000
	return fmt.Sprintf("%.1f Mbps", mbps)
}

func (p *VideoProperties) FormattedFileSize() string {
	if p.FileSize == 0 {
		return "N/A"
	}
	mb := float64(p.FileSize) / (1024 * 1024)
	if mb >= 1024 {
		gb := mb / 1024
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.1f MB", mb)
}

func (p *VideoProperties) FormattedDuration() string {
	total := int(p.Duration.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func (p *VideoProperties) EstimateOutputSize(selectionDuration time.Duration) string {
	if p.Bitrate == 0 || p.Duration == 0 {
		return "N/A"
	}
	ratio := float64(selectionDuration) / float64(p.Duration)
	estimatedBytes := int64(float64(p.FileSize) * ratio)
	mb := float64(estimatedBytes) / (1024 * 1024)
	return fmt.Sprintf("~%.1f MB", mb)
}
