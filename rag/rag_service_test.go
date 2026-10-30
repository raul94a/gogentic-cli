package rag

import (
	"fmt"
	"strings"
	"testing"
)

type RagServiceMock struct {
	ingestor Ingestor
}

// ProcessText uses a prompt to fetch information from a Vector Database.
func (r *RagServiceMock) ProcessText(prompt string) error { return nil }

// ProcessFile reads a file to store the embeddings into a Vector Database.
func (r *RagServiceMock) ProcessFile(path string) error {

	chunkChan := make(chan string)
	var readError error = nil
	go func() {
		defer close(chunkChan)
		if err := r.ingestor.ReadChunks(path, chunkChan); err != nil {
			readError = err
		}
	}()

	chunks := func(chann chan string) []string {
		chunks := make([]string, 0)
		for c := range chunkChan {
			c = strings.ToValidUTF8(c, "")
			chunks = append(chunks, c)
		}
		return chunks
	}(chunkChan)

	for _, c := range chunks {
		fmt.Printf("Process mock of length %d\n", len(c))
	}

	return readError

}

func createRagMock(step, overlap int64) Rag {

	return &RagServiceMock{
		ingestor: Ingestor{
			Step:    step,
			Overlap: overlap,
		},
	}
}

// Retrieve uses a prompt to retrieve close information to it from a Vector Database
func (r *RagServiceMock) Retrieve(prompt string, limit int) ([]RetrievedDocument, error) {
	return []RetrievedDocument{}, nil
}
func TestProcessFileOverlapBiggerThanStep(t *testing.T) {
	const FILENAME = "../README.md"
	svc := createRagMock(100, 200)

	err := svc.ProcessFile(FILENAME)

	if err == nil {
		t.Fatalf("Expected error because overlap >= step")
	}
	
	expectedError := "Overlap must be lower than Step"
	if err.Error() != expectedError {
		t.Fatalf("Expected error: %s; actual error: %s", expectedError, err.Error())

	}
}
