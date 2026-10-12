package utils

import (
	"os"
	"strings"
)

func SearchFiles(term string) []string {
	var matches []string

	dir := "."
	prefix := term
	
	if idx := strings.LastIndex(term, "/"); idx != -1 {
		dir = term[:idx]
		if dir == "" {
			dir = "/" 
		}
		prefix = term[idx+1:]
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				matchName := entry.Name()

				if entry.IsDir() {
					matchName += "/"
				}

				if dir != "." && dir != "/" {
					matchName = dir + "/" + matchName
				} else if dir == "/" {
					matchName = "/" + matchName
				}

				matches = append(matches, matchName)
			}
		}
	}
	
	return matches
}