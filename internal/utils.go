package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func getCacheFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	cacheDir := filepath.Join(configDir, "grepforllm")
	err = os.MkdirAll(cacheDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	return filepath.Join(cacheDir, "cache.json"), nil
}

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

func saveCache(filePath string, cache AppCache) error {
	if filePath == "" {
		return fmt.Errorf("cache file path is empty, cannot save")
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	tempFile := filePath + ".tmp"
	err = os.WriteFile(tempFile, data, 0o640)
	if err != nil {
		return fmt.Errorf("failed to write temporary cache file %s: %w", tempFile, err)
	}

	err = os.Rename(tempFile, filePath)
	if err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename temporary cache file to %s: %w", filePath, err)
	}

	return nil
}

func prettyPrintJSON(data []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, data, "", "  ")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

func isLikelyText(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatTokens(t int) string {
	if t < 1000 {
		return fmt.Sprintf("%d", t)
	}
	if t < 1000000 {
		return fmt.Sprintf("%.1fk", float64(t)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(t)/1000000)
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
