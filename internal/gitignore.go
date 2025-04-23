package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/denormal/go-gitignore"
)

// ListIgnoredFiles returns a slice of paths (relative to rootDir) that are
// ignored by the .gitignore file found in rootDir. If no .gitignore is present,
// it returns an empty slice without error.
func ListIgnoredFiles(rootDir string) ([]string, error) {
	// Locate .gitignore
	gitignorePath := filepath.Join(rootDir, ".gitignore")
	if stat, err := os.Stat(gitignorePath); err != nil {
		if os.IsNotExist(err) {
			// No .gitignore => no ignored files
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to stat .gitignore: %w", err)
	} else if stat.IsDir() {
		return nil, fmt.Errorf(".gitignore at %q is a directory, expected file", gitignorePath)
	}

	// Parse .gitignore
	ignore, err := gitignore.NewFromFile(gitignorePath)
	if err != nil {
		return nil, fmt.Errorf("error parsing .gitignore: %w", err)
	}

	var ignored []string

	// Walk the directory tree
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip .git directory entirely
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Compute relative path
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == ".gitignore" {
			return nil
		}

		// If ignored, collect
		if ignore.Ignore(rel) {
			ignored = append(ignored, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %q: %w", rootDir, err)
	}

	return ignored, nil
}
