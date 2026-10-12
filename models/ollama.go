package models;

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