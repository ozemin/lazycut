package main

import (
	"fmt"
	"io"
)

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: lazycut <video-file>")
	fmt.Fprintln(w, "Try 'lazycut --help' for more information.")
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `lazycut trims videos from the terminal.

Usage:
  lazycut <video-file>

Arguments:
  <video-file>    Path to the input video file

Flags:
  -h, --help      Show this help message
  -v, --version   Show version information

Examples:
  lazycut clip.mp4
  lazycut ~/Videos/interview.mov

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
  Tab             Cycle preview quality
  ?               Show keyboard shortcuts
  q               Quit`)
}
