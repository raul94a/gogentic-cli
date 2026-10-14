package main

import (
	//"agent-cli/tui"
	rag "gogentic-rag"
	"log"
	"sync"
	//"os"
	//tea "github.com/charmbracelet/bubbletea"
)

func main() {

	// p := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	// if _, err := p.Run(); err != nil {
	// 	fmt.Printf("Error: %v\n", err)
	// 	os.Exit(1)
	// }
	rag_service := rag.NewRagService("collection_test", 768, 1000)

	var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := rag_service.ProcessFile("README.md"); err != nil {
            log.Printf("Error: %v", err)
        }
    }()

    wg.Wait()
	

}
