package cmd

import (
	"fmt"

	"github.com/ozemin/lazycut/video"

	"github.com/spf13/cobra"
)

var probeCmd = &cobra.Command{
	Use:   "probe [video-file]",
	Short: "Print video properties and exit",
	Args:  validateVideoArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		videoPath := args[0]

		props, err := video.GetVideoProperties(videoPath)
		if err != nil {
			return fmt.Errorf("failed to get video properties: %v", err)
		}

		fmt.Print(props.Summary(videoPath))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(probeCmd)
}
