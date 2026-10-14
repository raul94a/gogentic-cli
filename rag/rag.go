package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

// We need to fetch two ENV vars
// OLLAMA_EMBEDDING_MODEL
// OLLAMA_EMBEDDING_ENDPOINT
// VECTOR_DATABASE_URL
// VECTOR_DATABASE_PORT

// Ingestor will handle stream process of files to emit chunks of text.
type Ingestor struct {
	Step int64
}

// ReadChunks handles the reading of a file
// each chunk will be emited through a channel.
func (i *Ingestor) ReadChunks(path string, content chan<- string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	fileLength := fileInfo.Size()

	var start int64 = 0

	buffer := make([]byte, i.Step)

	for start < fileLength {
		remaining := fileLength - start
		readSlice := buffer
		if remaining < i.Step {
			readSlice = buffer[:remaining]
		}

		n, err := file.ReadAt(readSlice, start)
		if n > 0 {
			content <- string(readSlice[:n])
			start += int64(n)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if n == 0 {
			break
		}
	}

	return nil
}

type Rag interface {

	// ProcessText uses a prompt to fetch information from a Vector Database.
	ProcessText(prompt string) error
	// ProcessFile reads a file to store the embeddings into a Vector Database.
	ProcessFile(path string) error
	// Retrieve uses a prompt to retrieve close information to it from a Vector Database
	Retrieve(prompt string) (string, error)
}

// IMPLEMENTATION OF RAG

type RagService struct {
	db             *database
	ingestor       *Ingestor
	embeddingURL   string
	embeddingModel string
}

func NewRagService(collectionName string, vectorSize uint64, chunkStep int64) *RagService {
	return &RagService{
		db:             CreateDatabase(collectionName, vectorSize),
		ingestor:       &Ingestor{Step: chunkStep},
		embeddingURL:   os.Getenv("OLLAMA_EMBEDDING_ENDPOINT"),   // e.g. "http://localhost:11434/api/embed"
		embeddingModel: os.Getenv("EMBEDDING_MODEL"), // e.g. "embeddinggemma"
	}
}

// getEmbedding calls the Ollama /api/embed endpoint to convert text into []float32.
func (r *RagService) getEmbedding(text string) ([]float32, error) {
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

	return result.Embeddings[0], nil
}

// ProcessFile streams chunks from the file using Ingestor, generates embeddings, and stores them.
func (r *RagService) ProcessFile(path string) error {
	chunksChan := make(chan string)
	errChan := make(chan error, 1)

	// Start streaming chunks in a goroutine
	go func() {
		defer close(chunksChan)
	
		if err := r.ingestor.ReadChunks(path, chunksChan); err != nil {
			errChan <- err
		}
	}()

	chunkIndex := 0
	for chunk := range chunksChan {
		embedding, err := r.getEmbedding(chunk)
		if err != nil {
			return fmt.Errorf("embedding failed at chunk %d: %w", chunkIndex, err)
		}

		if err := r.db.Upsert(path, chunkIndex, chunk, embedding); err != nil {
			return fmt.Errorf("qdrant upsert failed at chunk %d: %w", chunkIndex, err)
		}
		chunkIndex++
	}

	// Check if ReadChunks exited with an error
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// ProcessText stores a single direct string prompt/text into the vector database.
func (r *RagService) ProcessText(prompt string) error {
	embedding, err := r.getEmbedding(prompt)
	if err != nil {
		return err
	}
	return r.db.Upsert("direct_input", 0, prompt, embedding)
}

// Retrieve converts a prompt into an embedding and queries Qdrant for the top 5 closest chunks.
func (r *RagService) Retrieve(prompt string) (string, error) {
	queryEmbedding, err := r.getEmbedding(prompt)
	if err != nil {
		return "", err
	}
	return r.db.Query(queryEmbedding, 5)
}

// END IMPLEMENTATION OF RAG

type database struct {
	CollectionName string
	context        context.Context
	client         *qdrant.Client
	vectorSize     uint64
}

func (d *database) MaybeCreateCollection() error {
	exists, err := d.client.CollectionExists(d.context, d.CollectionName)
	if err != nil {
		return fmt.Errorf("error checking collection existence: %w", err)
	}
	if exists {
		return nil
	}

	err = d.client.CreateCollection(d.context, &qdrant.CreateCollection{
		CollectionName: d.CollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     d.vectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// Upsert stores a single text chunk and its embedding vector into Qdrant.
func (d *database) Upsert(sourceID string, chunkIndex int, text string, embedding []float32) error {
	// Generate a deterministic UUID from the file path and chunk index
	pointID := uuid.NewMD5(uuid.NameSpaceURL, []byte(fmt.Sprintf("%s_%d", sourceID, chunkIndex))).String()

	wait := true
	_, err := d.client.Upsert(d.context, &qdrant.UpsertPoints{
		CollectionName: d.CollectionName,
		Wait:           &wait,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewID(pointID),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(map[string]any{
					"text":        text,
					"source":      sourceID,
					"chunk_index": chunkIndex,
				}),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

// Query searches the collection using a query embedding and returns the concatenated text
// of the top-K closest chunks.
func (d *database) Query(queryEmbedding []float32, limit uint64) (string, error) {
	points, err := d.client.Query(d.context, &qdrant.QueryPoints{
		CollectionName: d.CollectionName,
		Query:          qdrant.NewQuery(queryEmbedding...),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query collection: %w", err)
	}

	var builder strings.Builder
	for _, point := range points {
		if textVal, ok := point.Payload["text"]; ok {
			builder.WriteString(textVal.GetStringValue())
			builder.WriteString("\n---\n")
		}
	}

	return builder.String(), nil
}

// CreateDatabase takes the collection name and the vector size
// Usually we follow this convention 
//
//  Target Dimension		Memory / Storage Footprint			Recommended Use Case
//	768 (Native)		100%						Full precision, maximum accuracy
func CreateDatabase(c string, vectorSize uint64) *database {
	ctx := context.Background()
	port, err := strconv.ParseInt(os.Getenv("VECTOR_DATABASE_PORT"), 10, 32)
	if err != nil {
		panic(err)
	}
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:     os.Getenv("VECTOR_DATABASE_URL"),
		Port:     int(port),
		UseTLS:   false, // uses default config with minimum TLS version set to 1.3
		PoolSize: 3,
		// KeepAliveTime: 10,
		// KeepAliveTimeout: 2,
		// TLSConfig: &tls.Config{...},
		// GrpcOptions: []grpc.DialOption{},
	})
	if err != nil {
		panic(err)
	}
	db := &database{
		CollectionName: c,
		context:        ctx,
		client:         client,
		vectorSize:     vectorSize,
	}

	db.MaybeCreateCollection()

	return db
}

