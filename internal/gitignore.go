// FILE: internal/gitignore.go
package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/denormal/go-gitignore"
)

// LoadGitignoreMatcher parses the .gitignore file found in rootDir and returns
// a matcher object. If no .gitignore is present or readable, it returns nil.
// If parsing fails, it returns an error.
func LoadGitignoreMatcher(rootDir string) (gitignore.GitIgnore, error) {
	// Locate .gitignore
	gitignorePath := filepath.Join(rootDir, ".gitignore")
	stat, err := os.Stat(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No .gitignore => no matcher needed, not an error
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat .gitignore: %w", err)
	}
	if stat.IsDir() {
		// Treat a directory named .gitignore as if it doesn't exist for matching purposes
		return nil, nil
		// Or return an error: return nil, fmt.Errorf(".gitignore at %q is a directory, expected file", gitignorePath)
	}

	// Parse .gitignore using the library
	// Pass the rootDir as the base path for the gitignore rules
	ignore, err := gitignore.NewFromFile(gitignorePath)
	if err != nil {
		// Check if the error is specifically about the file not existing, which we already handled
		// This might happen if the file disappears between Stat and NewFromFile
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error parsing .gitignore file %q: %w", gitignorePath, err)
	}

	// Successfully parsed, return the matcher
	return ignore, nil
}
