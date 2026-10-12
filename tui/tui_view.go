package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	agentStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("43")).Bold(true)
	systemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Italic(true)
	chipStyle       = lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("231")).Bold(true)
)

const asciiLogo = `
  ____                        _   _        ____ _     ___ 
 / ___| ___   __ _  ___ _ __ | |_(_) ___  / ___| |   |_ _|
| |  _ / _ \ / _' |/ _ \ '_ \| __| |/ __|| |   | |    | | 
| |_| | (_) | (_| |  __/ | | | |_| | (__ | |___| |___ | | 
 \____|\___/ \__, |\___|_| |_|\__|_|\___(_)____|_____|___|
             |___/                                        
`

var (
	logoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Italic(true)
)

func (m TuiModel) footView() string {
	var sections []string

	// Añadimos un espacio en blanco para separar el chat del input
	sections = append(sections, "")

	if m.thinking {
		spin := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.spinner.View())
		sections = append(sections, fmt.Sprintf(" %s Pensando...", spin))
	}

	if len(m.queue) > 0 {
		sections = append(sections, systemStyle.Render(fmt.Sprintf("[Queue: %d items pending]", len(m.queue))))
	}

	sections = append(sections, m.input.View())

	if len(m.attachedFiles) > 0 {
		var chips string
		chips += "Archivos en contexto: "
		for tag := range m.attachedFiles {
			chips += chipStyle.Render(" "+tag+" ") + " "
		}
		sections = append(sections, chips)
	}

	if m.showAutocomplete {
		sections = append(sections, "--- Archivos disponibles (Tab para adjuntar) ---")
		for i, match := range m.matches {
			if i == m.listIndex {
				sections = append(sections, selectedStyle.Render("> "+match))
			} else {
				sections = append(sections, unselectedStyle.Render("  "+match))
			}
		}
	}

	// lipgloss une todas las secciones asegurando que no haya \n sobrantes al final
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *TuiModel) syncViewport() {
	var content strings.Builder

	wrapWidth := int(float64(m.terminalWidth) * 0.8)
	if wrapWidth < 40 {
		wrapWidth = 40
	}
	messageWrapStyle := lipgloss.NewStyle().Width(wrapWidth)

	for _, msg := range m.history {
		if msg.Role == "logo" {
			content.WriteString(msg.Content)
			content.WriteString("\n\n")
			continue
		}
		roleLabel := ""
		if msg.Role == "user" {
			roleLabel = userStyle.Render("[User]")
		} else {
			roleLabel = agentStyle.Render("[Agent]")
		}

		var formattedMessage string
		if strings.HasPrefix(msg.Content, "[ERROR]") || strings.HasPrefix(msg.Content, "[System") {
			formattedMessage = fmt.Sprintf("%s %s", roleLabel, systemStyle.Render(msg.Content))
		} else {
			formattedMessage = fmt.Sprintf("%s %s", roleLabel, msg.Content)
		}

		content.WriteString(messageWrapStyle.Render(formattedMessage))
		content.WriteString("\n\n")
	}

	m.viewport.SetContent(content.String())
}

func (m TuiModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), m.footView())
}
