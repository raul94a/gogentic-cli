package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	p "path"
	"strings"
)

// IMPLEMENTATION OF RAG
type RagService struct {
	db             *database
	ingestor       *Ingestor
	embeddingURL   string
	embeddingModel string
}

func NewRagService(collectionName string, vectorSize uint64, minScoreThreshold float32, chunkStep int64, overlap int64) Rag {
	return &RagService{
		db:             CreateDatabase(collectionName, vectorSize, minScoreThreshold),
		ingestor:       &Ingestor{Step: chunkStep, Overlap: overlap},
		embeddingURL:   os.Getenv("OLLAMA_EMBEDDING_ENDPOINT"), // e.g. "http://localhost:11434/api/embed" if running ollama in local
		embeddingModel: os.Getenv("EMBEDDING_MODEL"),           // e.g. "embeddinggemma-2:270m" for just text
	}
}
func (r *RagService) GetOverlap() int64 {
	return r.ingestor.Overlap
}

func (r *RagService) optimizeChunkForEmbbedingGemma2(chunk string, filename string) string {
	return fmt.Sprintf("title %s | text: %s", filename, chunk)
}

// getEmbedding calls the Ollama /api/embed endpoint to convert text into []float32.
func (r RagService) getEmbeddings(text []string) ([][]float32, error) {
	reqBody, err := json.Marshal(map[string]any{
		"model": r.embeddingModel,
		"input": text,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(r.embeddingURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status: %d", resp.StatusCode)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned from model")
	}

	return result.Embeddings, nil
}

// ProcessFile streams chunks from the file using Ingestor, generates embeddings, and stores them.
func (r *RagService) ProcessFile(path string) error {
	chunksChan := make(chan string)
	var readError error = nil
	
	go func() {
		defer close(chunksChan)
		if err := r.ingestor.ReadChunks(path, chunksChan); err != nil {
			readError = err
		}
	}()

	const BATCH_SIZE = 64
	filename := p.Base(path)

	chunks := make([]string, 0)

	for chunk := range chunksChan {
		if readError != nil {
			return readError
		}
		// we need to modify each chunk
		chunk = strings.ToValidUTF8(chunk, "")
		chunk = r.optimizeChunkForEmbbedingGemma2(chunk, filename)
		chunks = append(chunks, chunk)
		if len(chunks) != BATCH_SIZE {
			continue
		}
		if err := r.processBatch(filename, &chunks, int(BATCH_SIZE)); err != nil {
			return err
		}

	}

	if len(chunks) > 0 {
		if err := r.processBatch(filename, &chunks, len(chunks)); err != nil {
			return err
		}
	}
	

	return nil

}

func (r *RagService) processBatch(filename string, chunks *[]string, batchSize int) error {
	if r.ingestor.Debug {
		fmt.Printf("Calling processBatch with BATCH SIZE %d\n", len(*chunks))
	}
	embeddings, err := r.getEmbeddings(*chunks)
	if err != nil {
		return fmt.Errorf("getEmbeddings failed")
	}
	if len(*chunks) == batchSize {
		if err := r.db.UpsertMany(filename, *chunks, embeddings); err != nil {
			return err
		}
		clear(*chunks)
		*chunks = (*chunks)[:0]
	}
	return nil
}

// ProcessText stores a single direct string prompt/text into the vector database.
func (r *RagService) ProcessText(prompt string) error {
	embedding, err := r.getEmbeddings([]string{prompt})
	if err != nil {
		return err
	}
	return r.db.Upsert("direct_input", 0, prompt, embedding[0])
}

// Retrieve converts a prompt into an embedding and queries Qdrant for the top 5 closest chunks.
func (r *RagService) Retrieve(prompt string, limit int) ([]RetrievedDocument, error) {
	if len(strings.TrimSpace(prompt)) == 0 {
		return []RetrievedDocument{}, fmt.Errorf("Prompt cannot be empty")
	}
	prompt = r.optimizeRetrievePromptEmbeddingGemma2(prompt)
	queryEmbedding, err := r.getEmbeddings([]string{prompt})

	if err != nil {
		return []RetrievedDocument{}, err
	}

	docs, err := r.db.Query(queryEmbedding[0], uint64(limit))
	if err != nil {
		return []RetrievedDocument{}, err
	}

	for i := range docs {
		docs[i].Text = r.cleanRetrievedDocWithEmbbedingGemma2Prefix(docs[i].Text)
	}
	return docs, nil
}

// optimizeRetrievePromptEmbeddingGemma2 modifies the prompt
// to add the search prefix in order to optimize the query embeddings
func (r *RagService) optimizeRetrievePromptEmbeddingGemma2(prompt string) string {
	return fmt.Sprintf("task: search result | query: %s", prompt)
}

func (r *RagService) cleanRetrievedDocWithEmbbedingGemma2Prefix(text string) string {

	if idx := strings.Index(text, "| text:"); idx != -1 {
		return strings.TrimSpace(text[idx+len("| text:"):])
	}

	return strings.TrimSpace(text)
}
