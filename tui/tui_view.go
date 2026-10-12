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
	// 1. Construimos PRIMERO toda la parte de abajo (el footer)
	var footer strings.Builder

	if m.thinking {
		spin := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.spinner.View())
		footer.WriteString(fmt.Sprintf("\n  %s Pensando...\n", spin))
	}
	
	// Render Cola
	if len(m.queue) > 0 {
		footer.WriteString(systemStyle.Render(fmt.Sprintf("\n[Queue: %d items pending]\n", len(m.queue))))
	}

	// Render Input
	footer.WriteString("\n")
	footer.WriteString(m.input.View())
	footer.WriteString("\n")

	// Render chips de archivos cargados
	if len(m.attachedFiles) > 0 {
		footer.WriteString("Archivos en contexto: ")
		for tag := range m.attachedFiles {
			footer.WriteString(chipStyle.Render(" " + tag + " "))
			footer.WriteString(" ")
		}
		footer.WriteString("\n")
	}

	// Render Dropdown
	if m.showAutocomplete {
		footer.WriteString("\n--- Archivos disponibles (Tab para adjuntar) ---\n")
		for i, match := range m.matches {
			if i == m.listIndex {
				footer.WriteString(selectedStyle.Render("> " + match))
				footer.WriteString("\n")
			} else {
				footer.WriteString(unselectedStyle.Render("  " + match))
				footer.WriteString("\n")
			}
		}
	}

	// Convertimos el footer a string para medirlo
	footerStr := footer.String()

	// 2. LA SOLUCIÓN: Calculamos cuántas líneas ocupa todo ese texto
	// lipgloss.Height cuenta exactamente los saltos de línea reales de footerStr
	if m.terminalHeight > 0 {
		footerHeight := lipgloss.Height(footerStr)
		
		// Le restamos el footer a la pantalla total para dárselo al viewport
		m.viewport.Height = m.terminalHeight - footerHeight
	}

	// 3. Sincronizamos el viewport AHORA que tiene su tamaño exacto
	m.syncViewport()

	// 4. Juntamos el Viewport arriba y el Footer abajo
	var finalView strings.Builder
	finalView.WriteString(m.viewport.View())
	finalView.WriteString(footerStr)

	return finalView.String()
}