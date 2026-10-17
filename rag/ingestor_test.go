package rag

import (
	"os"
	"testing"
)

func startReading(channel chan string, ingestor Ingestor, filename string) []string {
	go func (){
		defer close(channel)
		ingestor.ReadChunks(filename, channel)
	}()
	chunks := make([]string, 0)
	for chunk := range channel {
		chunks = append(chunks, chunk)
	}
	return chunks

}
// Tests the ReadChunks method from the Ingestor struct
func TestIngestorReadChunksNoOverlap(t *testing.T) {
	// Failure is posible if this file is located in other places
	const FILENAME = "../README.md"
	
	channel := make(chan string)
	const STEP = 1000
	ingestor := Ingestor {
		Step: STEP,
		Overlap: 0,
	}
	
	file, err := os.Open(FILENAME)
	
	if err != nil {
		t.Fatalf("%s should have existed", FILENAME)
	}
	defer file.Close()
	stats, _ := file.Stat()
	// Length in bytes
	size := stats.Size()
	
	numberOfBytes := 0

	chunks := startReading(channel, ingestor,FILENAME)

	for _, chunk := range chunks {
		numberOfBytes += len(chunk)
	}

	if int64(numberOfBytes) != size {
		t.Fatalf("Expected: {%d} Actual: {%d}", size, numberOfBytes)
	}

}


func TestIngestorReadChunksWithOverlap(t *testing.T) {
	// Failure is posible if this file is located in other places
	const FILENAME = "../README.md"
	
	channel := make(chan string)
	const STEP = 1000
	const OVERLAP = 150
	ingestor := Ingestor {
		Step: STEP,
		Overlap: OVERLAP,
	}
	
	file, err := os.Open(FILENAME)
	
	if err != nil {
		t.Fatalf("%s should have existed", FILENAME)
	}
	stats, _ := file.Stat()
	// Length in bytes
	size := stats.Size()
	chunks := startReading(channel, ingestor, FILENAME)
	numberOfChunks := len(chunks)
	numberOfBytes := 0

	for _, chunk := range chunks {
		numberOfBytes += len(chunk)
	}

	size += int64(OVERLAP * (numberOfChunks - 1))

	if int64(numberOfBytes) != size {
		t.Fatalf("Expected: {%d} Actual: {%d}", size, numberOfBytes)
	}

}