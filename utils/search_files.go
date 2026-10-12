package utils

import (
	"os"
	"strings"
)

func searchFiles(term string) []string {

	var matches []string
	entries, err := os.ReadDir(".")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), term) {
				matches = append(matches, entry.Name())
			}
		}
	}
	return matches

}
