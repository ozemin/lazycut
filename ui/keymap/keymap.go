package keymap

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Keymap struct {
	seek map[string]time.Duration
}

func New() *Keymap {
	return &Keymap{seek: defaultSeekBindings()}
}

func (k *Keymap) Seek(msg tea.KeyMsg) (time.Duration, bool) {
	amount, ok := k.seek[msg.String()]
	return amount, ok
}
