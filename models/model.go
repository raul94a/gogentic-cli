package models
/*
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	input         textinput.Model
	history       []Message
	queue         []string
	streaming     bool
	streamChan    chan string
	attachedFiles map[string]string

	showAutocomplete bool
	matches          []string
	listIndex        int
	currentWordStart int
	currentWordEnd   int
	terminalWidth    int
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
			m.terminalWidth = msg.Width
			// También limitamos la anchura de la caja de texto al 80%
			m.input.Width = int(float64(msg.Width) * 0.8)
			return m, nil

	case tea.KeyMsg:
		switch msg.Type {

		
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			if m.showAutocomplete && m.listIndex > 0 {
				m.listIndex--
				return m, nil
			}
		case tea.KeyDown:
			if m.showAutocomplete && m.listIndex < len(m.matches)-1 {
				m.listIndex++
				return m, nil
			}

		case tea.KeyTab:
			if m.showAutocomplete && len(m.matches) > 0 {
				selectedFile := m.matches[m.listIndex]
				content, err := os.ReadFile(selectedFile)
				if err == nil {
					tag := fmt.Sprintf("[@%s]", selectedFile)
					m.attachedFiles[tag] = string(content)

					val := m.input.Value()
					start := m.currentWordStart
					end := m.currentWordEnd

					if start < 0 {
						start = 0
					}
					if end > len(val) {
						end = len(val)
					}

					newVal := val[:start] + tag + " " + val[end:]
					m.input.SetValue(newVal)
					m.input.SetCursor(start + len(tag) + 1)
				}
				m.showAutocomplete = false
				m.matches = nil
				return m, nil
			}

		case tea.KeyEnter:
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}

			// Reemplazar etiquetas por el contenido íntegro del fichero
			finalPrompt := prompt
			for tag, content := range m.attachedFiles {
				if strings.Contains(finalPrompt, tag) {
					expanded := fmt.Sprintf("\n\n```%s\n%s\n```\n", tag, content)
					finalPrompt = strings.ReplaceAll(finalPrompt, tag, expanded)
				}
			}

			// Reiniciar input y archivos adjuntos
			m.attachedFiles = make(map[string]string)
			m.input.SetValue("")
			m.showAutocomplete = false

			if m.streaming {
				m.queue = append(m.queue, finalPrompt)
				return m, nil
			}

			return m.triggerLLM(finalPrompt)
		}

	case searchResultMsg:
		if len(msg) > 0 {
			m.matches = msg
			m.showAutocomplete = true
			m.listIndex = 0
		} else {
			m.showAutocomplete = false
		}

	case streamChunkMsg:
		lastIdx := len(m.history) - 1
		m.history[lastIdx].Content += string(msg)
		return m, waitForNextChunk(m.streamChan)

	case streamDoneMsg:
		m.streaming = false
		if len(m.queue) > 0 {
			nextPrompt := m.queue[0]
			m.queue = m.queue[1:]
			return m.triggerLLM(nextPrompt)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Sincronizar: si el usuario eliminó el tag [@archivo] con backspace, quitarlo del mapa
	currentInput := m.input.Value()
	for tag := range m.attachedFiles {
		if !strings.Contains(currentInput, tag) {
			delete(m.attachedFiles, tag)
		}
	}

	searchCmd := m.checkAutocompletion()
	if searchCmd != nil {
		cmds = append(cmds, searchCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) triggerLLM(prompt string) (tea.Model, tea.Cmd) {
	m.streaming = true
	m.history = append(m.history, Message{Role: "user", Content: prompt})
	m.history = append(m.history, Message{Role: "assistant", Content: ""})

	recentContext := m.getRecentMessages()

	return *m, tea.Batch(
		streamFromOllama(m.streamChan, recentContext),
		waitForNextChunk(m.streamChan),
	)
}

// Ventana deslizante para soportar hasta los 128k tokens sin descartar el último prompt
func (m Model) getRecentMessages() []Message {
	if len(m.history) < 2 {
		return nil
	}

	totalMsgs := len(m.history) - 1 // Excluir placeholder vacío del asistente

	if totalMsgs == 1 {
		return []Message{m.history[0]}
	}

	var totalChars int
	startIdx := totalMsgs - 1 // Al menos el prompt actual siempre debe entrar

	for i := totalMsgs - 1; i >= 0; i-- {
		msgLen := len(m.history[i].Content)
		if totalChars+msgLen > maxContextChars && i < totalMsgs-1 {
			break
		}
		totalChars += msgLen
		startIdx = i
	}

	payload := make([]Message, totalMsgs-startIdx)
	copy(payload, m.history[startIdx:totalMsgs])
	return payload
}

func (m *Model) checkAutocompletion() tea.Cmd {
	val := m.input.Value()
	cursor := m.input.Position()

	if cursor > len(val) {
		cursor = len(val)
	}

	start := strings.LastIndex(val[:cursor], " ")
	if start == -1 {
		start = 0
	} else {
		start++
	}

	word := val[start:cursor]

	if strings.HasPrefix(word, "@") {
		m.currentWordStart = start
		m.currentWordEnd = cursor
		searchTerm := word[1:]
		return searchFiles(searchTerm)
	}

	m.showAutocomplete = false
	return nil
}

func (m Model) View() string {
	var view strings.Builder

    wrapWidth := int(float64(m.terminalWidth) * 0.8)
	if wrapWidth < 40 {
		wrapWidth = 40 // fallback si terminalWidth aún no se ha inicializado
	}

	messageWrapStyle := lipgloss.NewStyle().Width(wrapWidth)

	// Render Historial
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
		view.WriteString(messageWrapStyle.Render(formattedMessage) + "\n\n")
	}

	// Render Cola
	if len(m.queue) > 0 {
		view.WriteString(systemStyle.Render(fmt.Sprintf("\n[Queue: %d items pending]\n", len(m.queue))))
	}

	// Render Input
	view.WriteString("\n" + m.input.View() + "\n")

	// Render chips de archivos cargados
	if len(m.attachedFiles) > 0 {
		view.WriteString("Archivos en contexto: ")
		for tag := range m.attachedFiles {
			view.WriteString(chipStyle.Render(" "+tag+" ") + " ")
		}
		view.WriteString("\n")
	}

	// Render Dropdown
	if m.showAutocomplete {
		view.WriteString("\n--- Archivos disponibles (Tab para adjuntar) ---\n")
		for i, match := range m.matches {
			if i == m.listIndex {
				view.WriteString(selectedStyle.Render("> "+match) + "\n")
			} else {
				view.WriteString(unselectedStyle.Render("  "+match) + "\n")
			}
		}
	}

	return view.String()
}
*/