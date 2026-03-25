package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ozemin/lazycut/ui"
	"github.com/ozemin/lazycut/video"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var version = "dev"

func main() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	fs := flag.NewFlagSet("lazycut", flag.ContinueOnError)
	fps := fs.Int("fps", 24, "preview frame rate (default 24)")
	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")
	showVersion := fs.Bool("version", false, "show version")
	fs.BoolVar(showVersion, "v", false, "show version")

	if err := fs.Parse(os.Args[1:]); err != nil {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	if *showHelp {
		printHelp(os.Stdout)
		os.Exit(0)
	}
	if *showVersion {
		fmt.Printf("lazycut version %s\n", version)
		os.Exit(0)
	}

	args := fs.Args()
	if len(args) == 0 {
		printUsage(os.Stderr)
		os.Exit(1)
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: expected exactly one video file.")
		printUsage(os.Stderr)
		os.Exit(1)
	}

	videoPath := args[0]

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File not found: %s\n", videoPath)
		os.Exit(1)
	}

	if err := video.CheckDependencies(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	player, err := video.NewPlayer(videoPath, *fps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open video: %v\n", err)
		os.Exit(1)
	}
	defer player.Close()

	m := ui.NewModel(player)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
