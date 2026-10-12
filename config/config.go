package Config

import (
	"os"
	"strconv"
)


type config struct {
	OllamaUrl        string
	ModelName        string
	MaxContextChars  int64
	NumContextTokens int64
}

type AgentConfig config

func InitConfig() config {
	ollama_url := os.Getenv("OLLAMA_URL")
	model_name := os.Getenv("MODEL_NAME")
	var max_ctx_chars string = os.Getenv("MAX_CONTEXT_CHARS")
	var max_ctx_tokens string = os.Getenv("MAX_CONTEXT_TOKENS")

	chars_i64, err := strconv.ParseInt(max_ctx_chars, 10, 64)
	tokens_i64, err := strconv.ParseInt(max_ctx_tokens, 10, 64)
	
	if err != nil {
		panic(err)
	}
	
	return config {
		OllamaUrl: ollama_url,
		ModelName: model_name,
		MaxContextChars: chars_i64,
		NumContextTokens: tokens_i64,
	}


}
