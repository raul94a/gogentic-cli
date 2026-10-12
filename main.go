package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"agent-cli/config"
)

// // --- Configuration ---
// const (
// 	ollamaURL = "http://localhost:11434/api/chat"
// 	modelName = "gemma4:e4b"
// 	// ~128k tokens equivale aproximadamente a 450.000 - 500.000 caracteres
// 	maxContextChars = 400000
// 	numCtxTokens    = 131072 // 128 * 1024 para Ollama
// )

// --- Styling ---
var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	agentStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("43")).Bold(true)
	systemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Italic(true)
	chipStyle       = lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("231")).Bold(true)
)

// --- API Structs ---
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaOptions struct {
	NumCtx int `json:"num_ctx"`
}

type OllamaRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  OllamaOptions `json:"options"`
}

type OllamaResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// --- Messages ---
type streamChunkMsg string
type streamDoneMsg struct{}
type searchResultMsg []string

// --- App State ---
type model struct {
	config 		  Config.AgentConfig
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
	thinking         bool
	spinner          spinner.Model
}

func initialModel() model {
	



	ti := textinput.New()
	ti.Placeholder = "Escribe un prompt... (@ para buscar archivos, Enter para enviar)"
	ti.CharLimit = 0
	ti.Focus()
	ti.Width = 100
	s := spinner.New()
	s.Spinner = spinner.Pulse
	return model{
		config: Config.AgentConfig(Config.InitConfig()),
		spinner:       s,
		input:         ti,
		streamChan:    make(chan string),
		attachedFiles: make(map[string]string),
		thinking:      false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
		
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// The spinner needs to animate to the next frame
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
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

			m.attachedFiles = make(map[string]string)
			m.input.SetValue("")
			m.showAutocomplete = false

			if m.streaming {
				m.queue = append(m.queue, finalPrompt)
				return m, nil
			}

			m.thinking = true

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
		m.thinking = false
		lastIdx := len(m.history) - 1
		m.history[lastIdx].Content += string(msg)
		return m, waitForNextChunk(m.streamChan)

	case streamDoneMsg:
		m.thinking = false 
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

func (m model) triggerLLM(prompt string) (tea.Model, tea.Cmd) {
	m.streaming = true
	m.history = append(m.history, Message{Role: "user", Content: prompt})
	m.history = append(m.history, Message{Role: "assistant", Content: ""})

	recentContext := m.getRecentMessages()

	return m, tea.Batch(
		streamFromOllama(m.streamChan, recentContext),
		waitForNextChunk(m.streamChan),
	)
}

func (m model) getRecentMessages() []Message {
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
		if totalChars+msgLen > int(m.config.MaxContextChars) && i < totalMsgs-1 {
			break
		}
		totalChars += msgLen
		startIdx = i
	}

	payload := make([]Message, totalMsgs-startIdx)
	copy(payload, m.history[startIdx:totalMsgs])
	return payload
}

func (m *model) checkAutocompletion() tea.Cmd {
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

func (m model) View() string {
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
    if m.thinking {
        spin := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.spinner.View())
        view.WriteString(fmt.Sprintf("\n  %s Pensando...\n\n", spin))
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

func searchFiles(term string) tea.Cmd {
	return func() tea.Msg {
		var matches []string
		entries, err := os.ReadDir(".")
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasPrefix(entry.Name(), term) {
					matches = append(matches, entry.Name())
				}
			}
		}
		return searchResultMsg(matches)
	}
}

func streamFromOllama(sub chan string, messages []Message) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer func() { sub <- "" }()

			reqBody := OllamaRequest{
				Model:    modelName,
				Messages: messages,
				Stream:   true,
				Options: OllamaOptions{
					NumCtx: numCtxTokens, // Obliga a Ollama a asignar la ventana de 128k tokens
				},
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				sub <- fmt.Sprintf("\n[System Error: %v]", err)
				return
			}

			resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				sub <- "\n[ERROR] No se pudo conectar a Ollama. Revisa que esté activo en http://localhost:11434"
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				sub <- fmt.Sprintf("\n[ERROR de Ollama: Código %d - %s]", resp.StatusCode, resp.Status)
				return
			}

			decoder := json.NewDecoder(resp.Body)
			for {
				var chunk OllamaResponse
				if err := decoder.Decode(&chunk); err != nil {
					break
				}
				if chunk.Message.Content != "" {
					sub <- chunk.Message.Content
				}
				if chunk.Done {
					break
				}
			}
		}()
		return nil
	}
}

func waitForNextChunk(sub chan string) tea.Cmd {
	return func() tea.Msg {
		chunk := <-sub
		if chunk == "" {
			return streamDoneMsg{}
		}
		return streamChunkMsg(chunk)
	}
}

func listenFinishChunk(sub chan string, m *model) tea.Cmd {
	return func() tea.Msg {
		chunk := <-sub
		if chunk == "" {
			m.thinking = false
			return streamDoneMsg{}
		}
		return streamChunkMsg(chunk)
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
