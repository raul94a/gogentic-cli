package rag

import (
	"context"
	"github.com/qdrant/go-client/qdrant"
)

// We need to fetch two ENV vars
// OLLAMA_EMBEDDING_MODEL
// OLLAMA_EMBEDDING_ENDPOINT
// VECTOR_DATABASE_URL
// VECTOR_DATABASE_PORT


type Rag interface {

	// ProcessText uses a prompt to fetch information from a Vector Database.
	ProcessText(prompt string) error
	// ProcessFile reads a file to store the embeddings into a Vector Database.
	ProcessFile(path string) error
	// Retrieve uses a prompt to retrieve close information to it from a Vector Database
	Retrieve(prompt string) ([]RetrievedDocument, error)
}

// Ingestor will handle stream process of files to emit chunks of text.
// Overlap will manage the overlap among chunks to avoid sentence breaking
// so the overall score may be improved.
type Ingestor struct {
	Step    int64
	Overlap int64
}


// RetrievedDocument is a result of the Retrieve from Rag
type RetrievedDocument struct {
	ID         string  `json:"id"`
	Score      float32 `json:"score"`
	Source     string  `json:"source"`
	ChunkIndex int     `json:"chunk_index"`
	Text       string  `json:"text"`
}


type database struct {
	CollectionName    string
	context           context.Context
	client            *qdrant.Client
	vectorSize        uint64
	minScoreThreshold float32
}

