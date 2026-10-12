package ollama

import (
	"agent-cli/config"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

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

type config struct {
	OllamaUrl        string
	ModelName        string
	NumContextTokens int64
}

type OllamaConfig config

func NewOllamaConfig(c Config.AgentConfig) OllamaConfig {
	return OllamaConfig{
		OllamaUrl:        c.OllamaUrl,
		ModelName:        c.ModelName,
		NumContextTokens: c.NumContextTokens,
	}
}

// --- Messages ---
type StreamChunkMsg string
type StreamDoneMsg struct{}


// Service is the struct used by the TuiModel to interact with LLMs
type Service interface {
	requestWithContext(sub chan string, messages []Message)
	StreamFromOllamaWithContext(sub chan string, messages []Message)
	Request(sub chan string, message Message)
}

type ServiceImpl struct {
	config config
}

func CreateServiceWithConfig(c *Config.AgentConfig) Service {

	return &ServiceImpl{
		config: config(NewOllamaConfig(*(c))),
	}
}

func (s *ServiceImpl) requestWithContext(sub chan string, messages []Message) {
	cnf := s.config
	reqBody := OllamaRequest{
		Model:    cnf.ModelName,
		Messages: messages,
		Stream:   true,
		Options: OllamaOptions{
			NumCtx: int(cnf.NumContextTokens),
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		sub <- fmt.Sprintf("\n[System Error: %v]", err)
		return
	}

	resp, err := http.Post(cnf.OllamaUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		sub <- "\n[ERROR] No se pudo conectar a Ollama. Revisa que esté activo en " + cnf.OllamaUrl
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
}

func (s *ServiceImpl) StreamFromOllamaWithContext(sub chan string, messages []Message){

	go func() {
		defer func() { sub <- "" }()
		s.requestWithContext(sub, messages)
	}()

}

func (s *ServiceImpl) Request(sub chan string, message Message) {
	panic("Not implemented")
}
