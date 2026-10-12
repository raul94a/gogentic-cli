package tui

import (
	Config "agent-cli/config"
	"agent-cli/ollama"
	Ollama "agent-cli/ollama"
	"agent-cli/utils"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type TuiModel struct {
	streaming        bool
	thinking         bool
	showAutocomplete bool
	listIndex        int
	currentWordStart int
	currentWordEnd   int
	terminalWidth    int
	terminalHeight   int
	streamChan       chan string
	queue            []string
	matches          []string
	attachedFiles    map[string]string
	spinner          spinner.Model
	input            textinput.Model
	viewport         viewport.Model

	config         Config.AgentConfig
	ollama_service Ollama.Service
	history        []Ollama.Message
}

// This type is for the file search for the agent TUI
type SearchResultMsg []string

func New() TuiModel {
	vp := viewport.New(0, 0)
	ti := textinput.New()
	ti.Placeholder = "Escribe un prompt... (@ para buscar archivos, Enter para enviar)"
	ti.CharLimit = 0
	ti.Focus()
	ti.Width = 100
	s := spinner.New()
	s.Spinner = spinner.Pulse
	config := Config.AgentConfig(Config.InitConfig())
	// Formateamos el arte y el subtítulo
	welcomeText := fmt.Sprintf("%s\n%s\n",
		logoStyle.Render(asciiLogo),
		subtitleStyle.Render("                  ▶ Model in use: "+config.ModelName),
	)
	return TuiModel{
		config: config,
		history: []Ollama.Message{
			{Role: "logo", Content: welcomeText},
		}, 
		spinner: s,
		input:          ti,
		viewport:       vp,
		streamChan:     make(chan string),
		attachedFiles:  make(map[string]string),
		thinking:       false,
		ollama_service: ollama.CreateServiceWithConfig(&config),
	}
}

func (m TuiModel) getRecentMessages() []Ollama.Message {
	if len(m.history) < 2 {
		return nil
	}

	totalMsgs := len(m.history) - 1 // Excluir placeholder vacío del asistente

	if totalMsgs == 1 {
		return []Ollama.Message{m.history[0]}
	}

	var totalChars int
	startIdx := totalMsgs - 1
	for i := totalMsgs - 1; i >= 0; i-- {
		msgLen := len(m.history[i].Content)
		if totalChars+msgLen > int(m.config.MaxContextChars) && i < totalMsgs-1 {
			break
		}
		totalChars += msgLen
		startIdx = i
	}

	payload := make([]Ollama.Message, totalMsgs-startIdx)
	copy(payload, m.history[startIdx:totalMsgs])
	return payload
}

func (m *TuiModel) request(prompt string) (tea.Model, tea.Cmd) {
	m.streaming = true
	m.history = append(m.history, Ollama.Message{Role: "user", Content: prompt})
	m.history = append(m.history, Ollama.Message{Role: "assistant", Content: ""})

	recentContext := m.getRecentMessages()

	return m, tea.Batch(
		func() tea.Cmd {
			m.ollama_service.StreamFromOllamaWithContext(m.streamChan, recentContext)
			return nil

		}(),
		m.waitForNextChunk(m.streamChan),
	)
}

func (m *TuiModel) waitForNextChunk(sub chan string) tea.Cmd {
	return func() tea.Msg {
		chunk := <-sub
		if chunk == "" {
			return Ollama.StreamDoneMsg{}
		}
		return Ollama.StreamChunkMsg(chunk)
	}
}
func (m *TuiModel) checkAutocompletion() tea.Cmd {
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
		return m.searchFiles(searchTerm)
	}

	m.showAutocomplete = false
	return nil
}

func (m *TuiModel) searchFiles(term string) tea.Cmd {
	return func() tea.Msg {
		return SearchResultMsg(utils.SearchFiles(term))
	}
}
