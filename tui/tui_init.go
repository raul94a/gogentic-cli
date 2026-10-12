package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m TuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}