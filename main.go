package main

import (
	//"agent-cli/tui"
	"fmt"
	rag "gogentic-rag"
	"log"
	// "sync"
	//"os"
	//tea "github.com/charmbracelet/bubbletea"
)

func main() {
	const EMPTY = 0
	// p := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	// if _, err := p.Run(); err != nil {
	// 	fmt.Printf("Error: %v\n", err)
	// 	os.Exit(1)
	// }
	const COLLECTION_NAME = ""
	const OVERLAP = 150
	rag_service := rag.NewRagService(COLLECTION_NAME, 768, float32(0.653), 1000, OVERLAP)

	// var wg sync.WaitGroup
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()

	// 	if err := rag_service.ProcessFile(f); err != nil {
	// 		log.Printf("Error: %v", err)
	// 	}

	// }()

	question := "Your question"
	docs, err := rag_service.Retrieve(question, 5)
	if err != nil {
		log.Fatalf("Retrieve failed: %v", err)
	}

	for _, doc := range docs {
		fmt.Printf("Score of doc %s: %f\n", doc.ID, doc.Score)
		fmt.Printf("Text is %s\n", doc.Text)
	}
	if len(docs) == EMPTY {
		fmt.Printf("Not found document related with query: %s\n", question)
	}
	// wg.Wait()

}
