package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"io"
	"net/http"
	"os"
	p "path"
	"strconv"
	"strings"
)

// We need to fetch two ENV vars
// OLLAMA_EMBEDDING_MODEL
// OLLAMA_EMBEDDING_ENDPOINT
// VECTOR_DATABASE_URL
// VECTOR_DATABASE_PORT

// Ingestor will handle stream process of files to emit chunks of text.
// Overlap will manage the overlap among chunks to avoid sentence breaking
// so the overall score may be improved.
type Ingestor struct {
	Step    int64
	Overlap int64
}

// ReadChunks handles the reading of a file
// each chunk will be emited through a channel.
func (i *Ingestor) ReadChunks(path string, content chan<- string) error {
	if i.Overlap >= i.Step {
		return fmt.Errorf("Overlap must be lower than Step")
	}
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
	// TODO: Remove
	fmt.Printf("Reading %s with size of %d\n", path, fileLength)

	var start int64 = 0

	buffer := make([]byte, i.Step)

	for start < fileLength {
		if start > 0 {
			start -= i.Overlap
		}
		fmt.Printf("Start has a value of %d\n", start)
		remaining := fileLength - start

		readSlice := buffer
		if remaining < i.Step {
			readSlice = buffer[:remaining]
		}

		n, err := file.ReadAt(readSlice, start)
		fmt.Printf("number of bytes read is: %d\n", n)
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
	Retrieve(prompt string) ([]RetrievedDocument, error)
}

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
		embeddingURL:   os.Getenv("OLLAMA_EMBEDDING_ENDPOINT"), // e.g. "http://localhost:11434/api/embed"
		embeddingModel: os.Getenv("EMBEDDING_MODEL"),           // e.g. "embeddinggemma"
	}
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
	// TODO: Need a channel of a custom type
	chunksChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunksChan)
		defer close(errChan)
		if err := r.ingestor.ReadChunks(path, chunksChan); err != nil {
			errChan <- err
		}
	}()

	const BATCH_SIZE = 64
	filename := p.Base(path)

	chunks := make([]string, 0)

	for chunk := range chunksChan {
		// we need to modify each chunk
		chunk = strings.ToValidUTF8(chunk,"")
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
	
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func (r *RagService) processBatch(filename string, chunks *[]string, batchSize int) error {
	fmt.Printf("Calling processBatch with BATCH SIZE %d", len(*chunks))
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
func (r *RagService) Retrieve(prompt string) ([]RetrievedDocument, error) {
	if len(strings.TrimSpace(prompt)) == 0 {
		return []RetrievedDocument{}, fmt.Errorf("Prompt cannot be empty")
	}
	prompt = r.optimizeRetrievePromptEmbeddingGemma2(prompt)
	queryEmbedding, err := r.getEmbeddings([]string{prompt})

	if err != nil {
		return []RetrievedDocument{}, err
	}

	docs, err := r.db.Query(queryEmbedding[0], 5)
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

// END IMPLEMENTATION OF RAG

type database struct {
	CollectionName    string
	context           context.Context
	client            *qdrant.Client
	vectorSize        uint64
	minScoreThreshold float32
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

// Upsert stores a single text chunk and its embedding vector into Qdrant.
func (d *database) UpsertMany(sourceID string, chunks []string, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("Error. Different size of embeddings vs chunks")
	}
	points := []*qdrant.PointStruct{}

	for i := 0; i < len(chunks); i++ {
		pointID := uuid.NewMD5(uuid.NameSpaceURL, []byte(fmt.Sprintf("%s_%d", sourceID, i))).String()
		c := chunks[i]
		e := embeddings[i]
		point := &qdrant.PointStruct{
			Id:      qdrant.NewID(pointID),
			Vectors: qdrant.NewVectors(e...),
			Payload: qdrant.NewValueMap(map[string]any{
				"text":        c,
				"source":      sourceID,
				"chunk_index": i,
			}),
		}
		points = append(points, point)
	}

	wait := true
	_, err := d.client.Upsert(d.context, &qdrant.UpsertPoints{
		CollectionName: d.CollectionName,
		Wait:           &wait,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

// Query searches the collection using a query embedding and returns the concatenated text
// of the top-K closest chunks.
func (d *database) Query(queryEmbedding []float32, limit uint64) ([]RetrievedDocument, error) {
	points, err := d.client.Query(d.context, &qdrant.QueryPoints{
		CollectionName: d.CollectionName,
		Query:          qdrant.NewQuery(queryEmbedding...),
		Limit:          qdrant.PtrOf(limit),
		ScoreThreshold: &d.minScoreThreshold,
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
		Params: &qdrant.SearchParams{
			HnswEf: qdrant.PtrOf(uint64(128)),
			Exact:  qdrant.PtrOf(false),
			Quantization: &qdrant.QuantizationSearchParams{
				Rescore:      qdrant.PtrOf(true),
				Oversampling: qdrant.PtrOf(2.0),
			},
		},
	})
	if err != nil {
		return []RetrievedDocument{}, fmt.Errorf("failed to query collection: %w", err)
	}

	return ParseQueryResults(points), nil
}

// CreateDatabase takes the collection name and the vector size
// Usually we follow this convention
//
//	 Target Dimension		Memory / Storage Footprint			Recommended Use Case
//		768 (Native)		100%						Full precision, maximum accuracy
func CreateDatabase(c string, vectorSize uint64, minScoreThreshold float32) *database {
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
		CollectionName:    c,
		context:           ctx,
		client:            client,
		minScoreThreshold: minScoreThreshold,
		vectorSize:        vectorSize,
	}

	db.MaybeCreateCollection()

	return db
}

// RetrievedDocument is a result of the Retrieve from Rag
type RetrievedDocument struct {
	ID         string  `json:"id"`
	Score      float32 `json:"score"`
	Source     string  `json:"source"`
	ChunkIndex int     `json:"chunk_index"`
	Text       string  `json:"text"`
}

// ParseQueryResults converts raw Qdrant scored points into your public API structs.
func ParseQueryResults(points []*qdrant.ScoredPoint) []RetrievedDocument {
	results := make([]RetrievedDocument, len(points))

	for i, p := range points {
		doc := RetrievedDocument{
			Score: p.Score,
		}

		if p.Id != nil {
			doc.ID = p.Id.GetUuid()
		}

		if p.Payload != nil {
			if src, ok := p.Payload["source"]; ok {
				doc.Source = src.GetStringValue()
			}
			if idx, ok := p.Payload["chunk_index"]; ok {
				doc.ChunkIndex = int(idx.GetIntegerValue())
			}
			if txt, ok := p.Payload["text"]; ok {
				doc.Text = txt.GetStringValue()
			}
		}

		results[i] = doc
	}

	return results
}
