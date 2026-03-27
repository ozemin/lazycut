package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ozemin/lazycut/video"

	"github.com/spf13/cobra"
)

var (
	trimIn     string
	trimOut    string
	trimOutput string
)

var trimCmd = &cobra.Command{
	Use:   "trim [video-file]",
	Short: "Trim a video non-interactively",
	Long: `Trim a video file using specified in/out timestamps.

Examples:
  lazycut trim --in 00:00:10 --out 00:00:30 clip.mp4
  lazycut trim --in 00:01:00 --out 00:02:00 -o output.mp4 clip.mp4`,
	Args: validateVideoArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		videoPath := args[0]

		inPoint, err := parseTimestamp(trimIn)
		if err != nil {
			return fmt.Errorf("invalid --in timestamp: %v", err)
		}

		outPoint, err := parseTimestamp(trimOut)
		if err != nil {
			return fmt.Errorf("invalid --out timestamp: %v", err)
		}

		if outPoint <= inPoint {
			return fmt.Errorf("--out must be after --in")
		}

		progress := make(chan float64, 100)
		done := make(chan error, 1)

		opts := video.ExportOptions{
			Input:    videoPath,
			Output:   trimOutput,
			InPoint:  inPoint,
			OutPoint: outPoint,
		}

		go func() {
			_, err := video.ExportWithProgress(opts, progress)
			done <- err
		}()

		for p := range progress {
			fmt.Fprintf(cmd.OutOrStdout(), "\rProgress: %.0f%%", p*100)
		}
		fmt.Fprintln(cmd.OutOrStdout())

		if err := <-done; err != nil {
			return fmt.Errorf("export failed: %v", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Done.")
		return nil
	},
}

func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3:
		h, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours: %s", parts[0])
		}
		m, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %s", parts[1])
		}
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %s", parts[2])
		}
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second)), nil
	case 2:
		m, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %s", parts[0])
		}
		sec, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %s", parts[1])
		}
		return time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second)), nil
	case 1:
		sec, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %s", parts[0])
		}
		return time.Duration(sec * float64(time.Second)), nil
	default:
		return 0, fmt.Errorf("expected HH:MM:SS, MM:SS, or seconds")
	}
}

func init() {
	trimCmd.Flags().StringVar(&trimIn, "in", "", "start timestamp (HH:MM:SS, MM:SS, or seconds)")
	trimCmd.Flags().StringVar(&trimOut, "out", "", "end timestamp (HH:MM:SS, MM:SS, or seconds)")
	trimCmd.Flags().StringVarP(&trimOutput, "output", "o", "", "output file path (optional)")
	trimCmd.MarkFlagRequired("in")
	trimCmd.MarkFlagRequired("out")
	rootCmd.AddCommand(trimCmd)
}
