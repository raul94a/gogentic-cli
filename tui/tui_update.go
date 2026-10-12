package tui

import (
	"fmt"
	"os"
	"strings"

	Ollama "agent-cli/ollama"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// The spinner needs to animate to the next frame
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.input.Width = int(float64(msg.Width) * 0.8)
		m.viewport.Width = msg.Width
		m.input.Width = int(float64(msg.Width) * 0.8)

		// 2. Calcular altura fija para la parte inferior (Input + márgenes)
		// Reserva unas 4-5 líneas por defecto para el input y los márgenes.
		// Si está el menú de autocomplete abierto, necesitas reservar unas 10 líneas.
		footerHeight := 5 
		if m.showAutocomplete {
			footerHeight += len(m.matches) + 2 // Altura del menú
		}
		
		m.viewport.Height = msg.Height - footerHeight
		
		m.syncViewport()
		m.viewport.GotoBottom()
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
				// Protección de seguridad por si el índice se desincroniza
				if m.listIndex >= len(m.matches) {
					m.listIndex = len(m.matches) - 1
				}

				selectedFile := m.matches[m.listIndex]
				val := m.input.Value()
				start := m.currentWordStart
				end := m.currentWordEnd

				// Asegurar límites seguros del cursor
				if start < 0 {
					start = 0
				}
				if end > len(val) {
					end = len(val)
				}

				if strings.HasSuffix(selectedFile, "/") {
				
					newVal := val[:start] + "@" + selectedFile + val[end:]
					m.input.SetValue(newVal)
					m.input.SetCursor(start + 1 + len(selectedFile))

					
					searchCmd := m.checkAutocompletion()
					return m, searchCmd

				} else {
					
					content, err := os.ReadFile(selectedFile)
					if err == nil {
						tag := fmt.Sprintf("[@%s]", selectedFile)

						if m.attachedFiles == nil {
							m.attachedFiles = make(map[string]string)
						}
						m.attachedFiles[tag] = string(content)

						newVal := val[:start] + tag + " " + val[end:]
						m.input.SetValue(newVal)
						m.input.SetCursor(start + len(tag) + 1)
					}

					// Cerrar autocompletado
					m.showAutocomplete = false
					m.matches = nil
					return m, nil
				}
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

			m.attachedFiles = make(map[string]string)
			m.input.SetValue("")
			m.showAutocomplete = false

			if m.streaming {
				m.queue = append(m.queue, finalPrompt)
				return m, nil
			}

			m.thinking = true

			return m.request(finalPrompt)
		}

	case SearchResultMsg:
		if len(msg) > 0 {
			m.matches = msg
			m.showAutocomplete = true
			m.listIndex = 0
		} else {
			m.showAutocomplete = false
		}
		return m, nil

	case Ollama.StreamChunkMsg:
		m.thinking = false
		lastIdx := len(m.history) - 1
		m.history[lastIdx].Content += string(msg)
		m.syncViewport()
		m.viewport.GotoBottom()
		return m, m.waitForNextChunk(m.streamChan)

	case Ollama.StreamDoneMsg:
		m.thinking = false
		m.streaming = false
		if len(m.queue) > 0 {
			nextPrompt := m.queue[0]
			m.queue = m.queue[1:]
			return m.request(nextPrompt)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	var viewPortCmd tea.Cmd
	m.viewport, viewPortCmd = m.viewport.Update(msg)
	cmds = append(cmds, viewPortCmd)
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
