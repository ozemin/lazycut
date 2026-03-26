package cmd

import (
	"fmt"
	"os"

	"github.com/ozemin/lazycut/ui"
	"github.com/ozemin/lazycut/video"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	fps     int
)

func SetVersion(v string) {
	version = v
}

var rootCmd = &cobra.Command{
	Use:   "lazycut [video-file]",
	Short: "Terminal-based video trimming TUI (ffmpeg + chafa)",
	Long: `lazycut trims videos from the terminal.

Keyboard Shortcuts:
  Space           Play/Pause
  h / l           Seek -/+1 second
  H / L           Seek -/+5 seconds
  , / .           Seek -/+1 frame
  0               Go to start
  G / $           Go to end
  i / o           Set in/out points
  p               Preview selection
  d / Esc         Clear selection
  Enter           Export selection
  u               Undo last trim change
  m               Toggle mute
  ?               Show keyboard shortcuts
  q               Quit`,
	Args:    validateVideoArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			lipgloss.SetColorProfile(termenv.Ascii)
		}

		videoPath := args[0]

		if err := video.CheckBinaries("ffmpeg", "ffprobe", "ffplay"); err != nil {
			return err
		}

		player, err := video.NewPlayer(videoPath, fps)
		if err != nil {
			return fmt.Errorf("failed to open video: %v", err)
		}
		defer player.Close()

		m := ui.NewModel(player)

		p := tea.NewProgram(
			m,
			tea.WithAltScreen(),
		)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().IntVar(&fps, "fps", 24, "preview frame rate (lower = faster rendering)")
}

func validateVideoArg(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected exactly one video file")
	}
	if _, err := os.Stat(args[0]); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", args[0])
	}
	return nil
}

func Execute() {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
