package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// --- Cache Helper Functions ---

// getCacheFilePath determines the path for the cache file.
func getCacheFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	cacheDir := filepath.Join(configDir, "grepforllm") // ~/.config/grepforllm
	err = os.MkdirAll(cacheDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	return filepath.Join(cacheDir, "cache.json"), nil // ~/.config/grepforllm/cache.json
}

// loadCache reads the cache file and unmarshals it.
func loadCache(filePath string) (AppCache, error) {
	cache := make(AppCache)
	if filePath == "" {
		return cache, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, fmt.Errorf("failed to read cache file %s: %w", filePath, err)
	}

	if len(data) == 0 {
		return cache, nil
	}

	err = json.Unmarshal(data, &cache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to unmarshal cache data from %s: %v. Treating as empty.\n", filePath, err)
		return make(AppCache), nil
	}

	return cache, nil
}

// saveCache marshals the cache map and writes it to the file atomically.
func saveCache(filePath string, cache AppCache) error {
	if filePath == "" {
		return fmt.Errorf("cache file path is empty, cannot save")
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	// Use temp file for atomic write
	tempFile := filePath + ".tmp"
	err = os.WriteFile(tempFile, data, 0o640)
	if err != nil {
		return fmt.Errorf("failed to write temporary cache file %s: %w", tempFile, err)
	}

	// Rename temporary file to the actual cache file path
	err = os.Rename(tempFile, filePath)
	if err != nil {
		// Clean up temp file if rename fails
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename temporary cache file to %s: %w", filePath, err)
	}

	return nil
}

// Helper function to pretty print JSON bytes
func prettyPrintJSON(data []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, data, "", "  ")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

// Helper function to check if content is likely text
func isLikelyText(data []byte) bool {
	// Simple check: presence of null byte often indicates binary
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	// Could add more checks here if needed (e.g., http.DetectContentType)
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
