package main

import (
	"fmt"
	"github.com/emin-ozata/lazycut/ui"
	"github.com/emin-ozata/lazycut/video"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printHelp(os.Stdout)
		os.Exit(0)
	}

	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Printf("lazycut version %s\n", version)
		os.Exit(0)
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: expected exactly one video file.")
		printUsage(os.Stderr)
		os.Exit(1)
	}

	videoPath := args[0]

	// Check if video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File not found: %s\n", videoPath)
		os.Exit(1)
	}

	// Check dependencies
	if err := video.CheckDependencies(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Create video player
	player, err := video.NewPlayer(videoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open video: %v\n", err)
		os.Exit(1)
	}
	defer player.Close()

	// Create the UI model with video player
	m := ui.NewModel(player)

	// Create the bubbletea program with alternate screen
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
