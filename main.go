package main

import (
	"fmt"
	"github.com/arobase-che/lazycut/ui"
	"github.com/arobase-che/lazycut/video"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	// Check command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: lazycut <video.mp4>")
		os.Exit(1)
	}

	// Handle version flag
	if os.Args[1] == "-v" || os.Args[1] == "--version" {
		fmt.Printf("lazycut version %s\n", version)
		os.Exit(0)
	}

	videoPath := os.Args[1]

	// Check if video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		fmt.Printf("File not found: %s\n", videoPath)
		os.Exit(1)
	}

	// Check dependencies
	if err := video.CheckDependencies(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create video player
	player, err := video.NewPlayer(videoPath)
	if err != nil {
		fmt.Printf("Failed to open video: %v\n", err)
		os.Exit(1)
	}
	defer player.Close()

	// Create the UI model with video player
	m := ui.NewModel(player)

	// Create the bubbletea program with alternate screen
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
