package keymap

import "time"

func defaultSeekBindings() map[string]time.Duration {
	return map[string]time.Duration{
		"h":           -time.Second,
		"l":           +time.Second,
		"H":           -5 * time.Second,
		"L":           +5 * time.Second,
		"left":        -5 * time.Second,
		"right":       +5 * time.Second,
		"shift+left":  -time.Second,
		"shift+right": +time.Second,
		"up":   +time.Minute,
		"down": -time.Minute,
	}
}
