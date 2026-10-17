package rag

import (
	"os"
	"testing"
)

// readFileHelper is a helper function that reads chunks of a file and get them into a channel.
// it also handles the closing of the channel after the read is finished.
func readFileHelper(channel chan string, ingestor Ingestor, filename string) ([]string, error) {
	var readError error = nil
	go func() {
		defer close(channel)
		err := ingestor.ReadChunks(filename, channel)
		if err != nil {
			readError = err
		}
	}()
	chunks := make([]string, 0)
	for chunk := range channel {
		chunks = append(chunks, chunk)
	}
	return chunks, readError

}

// Tests the ReadChunks method from the Ingestor struct with Overlap disabled.
func TestIngestorReadChunksNoOverlap(t *testing.T) {
	// Failure is posible if this file is located in other places
	const FILENAME = "../README.md"

	channel := make(chan string)
	const STEP = 1000
	ingestor := Ingestor{
		Step:    STEP,
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

	chunks, err := readFileHelper(channel, ingestor, FILENAME)
	if err != nil {
		t.Fatalf("Test failed because %v", err)
	}
	for _, chunk := range chunks {
		numberOfBytes += len(chunk)
	}

	if int64(numberOfBytes) != size {
		t.Fatalf("Expected: {%d} Actual: {%d}", size, numberOfBytes)
	}

}

// Tests the ReadChunks method from the Ingestor struct with Overlap enabled.
func TestIngestorReadChunksWithOverlap(t *testing.T) {
	// Failure is posible if this file is located in other places
	const FILENAME = "../README.md"

	channel := make(chan string)
	const STEP = 1000
	const OVERLAP = 150
	ingestor := Ingestor{
		Step:    STEP,
		Overlap: OVERLAP,
	}

	file, err := os.Open(FILENAME)

	if err != nil {
		t.Fatalf("%s should have existed", FILENAME)
	}
	stats, _ := file.Stat()
	// Length in bytes
	size := stats.Size()
	chunks, err := readFileHelper(channel, ingestor, FILENAME)
	if err != nil {
		t.Fatalf("Test failed because %v", err)
	}
	numberOfChunks := len(chunks)
	numberOfBytes := 0

	for _, chunk := range chunks {
		numberOfBytes += len(chunk)
	}
	// the formula to compute the number of bytes read by the Ingestor when Overlap is active.
	//
	size += int64(OVERLAP * (numberOfChunks - 1))

	if int64(numberOfBytes) != size {
		t.Fatalf("Expected: {%d} Actual: {%d}", size, numberOfBytes)
	}
	
	

}

// Tests the situation when Overlap >= Step. In this case an error must be raised.
func TestOverlapBiggerOrEqualThanStep(t *testing.T) {
	const FILENAME = "../README.md"

	channel := make(chan string)
	
	const STEP = 1000
	const OVERLAP = 1001
	ingestor := Ingestor{
		Step:    STEP,
		Overlap: OVERLAP,

	}
	_, err := readFileHelper(channel, ingestor, FILENAME)
	expectedError := "Overlap must be lower than Step"
	if err != nil && err.Error() != expectedError {
		t.Fatalf("Expected error: %s; actual error: %s", expectedError, err.Error())
	}

}
