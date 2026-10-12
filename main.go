package main

import (
	"fmt"
	"os"
	tea "github.com/charmbracelet/bubbletea"
	"agent-cli/tui"
)

func main() {
	p := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
