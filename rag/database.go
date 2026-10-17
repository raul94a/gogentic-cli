package rag

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)



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
