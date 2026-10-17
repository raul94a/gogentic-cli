package rag

import (
	"fmt"
	"io"
	"os"
)


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