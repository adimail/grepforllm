package internal

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
)

// ListFiles walks the directory, identifies text files, populates app.allFiles,
// and then applies filters (which now include gitignore checks).
func (app *App) ListFiles() error {
	app.mutex.Lock()

	var files []string
	// Walk the directory only once to find all potential files
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return filepath.SkipDir // Skip directories we can't read
			}
			return nil
		}

		// Skip root itself
		if path == app.rootDir {
			return nil
		}

		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			return nil
		}
		relPathSlash := filepath.ToSlash(relPath) // Use slash-separated path consistently

		// --- Directory Handling ---
		if d.IsDir() {
			dirPathWithSlash := relPathSlash + "/"
			// Simple check for default excluded *directories* during walk
			// This prevents descending into large unwanted dirs like .git or node_modules
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" || !strings.HasSuffix(pattern, "/") {
					continue // Only check directory patterns here
				}
				// Use filepath.ToSlash for consistent matching
				pattern = filepath.ToSlash(pattern)
				if strings.HasPrefix(dirPathWithSlash, pattern) {
					return filepath.SkipDir
				}
			}
			// Also skip .git directory explicitly if not caught by DefaultExcludes
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil // Continue walking in this directory
		}

		// --- File Handling ---

		// Skip .gitignore file itself
		if relPathSlash == ".gitignore" {
			return nil
		}

		// --- Binary File Check (only for files) ---
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		buffer := make([]byte, 512) // Read a small chunk to detect content type
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil
		}

		// Check if it's likely a text file
		contentType := http.DetectContentType(buffer[:n])
		if !strings.HasPrefix(contentType, "text/") {
			return nil // Skip binary files
		}

		// If it's a text file, add its relative path to the list
		files = append(files, relPathSlash)
		return nil
	})
	if err != nil {
		app.mutex.Unlock() // Ensure unlock on error
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Strings(files)  // Sort all discovered text files
	app.allFiles = files // Store the complete list

	// applyFilters will now use the gitignore info via shouldIncludeFile
	// It also unlocks the mutex.
	app.applyFilters()
	return nil
}

// applyFilters filters app.allFiles into app.fileList based on current filter settings
// and gitignore rules. It assumes the mutex is held when called.
func (app *App) applyFilters() {
	defer app.mutex.Unlock() // Unlock when done

	filteredList := []string{}
	newSelectedFiles := make(map[string]bool)

	// Read filter state under lock
	currentFilterMode := app.filterMode
	currentIncludes := app.includes
	currentExcludes := app.excludes
	// No need to copy gitIgnoredFiles, it's read-only here

	for _, file := range app.allFiles {
		// shouldIncludeFile now checks gitignore first
		if app.shouldIncludeFile(file, currentFilterMode, currentIncludes, currentExcludes) {
			filteredList = append(filteredList, file)
			// Preserve selection state if the file remains visible
			if app.selectedFiles[file] {
				newSelectedFiles[file] = true
			}
		}
	}

	app.fileList = filteredList
	app.selectedFiles = newSelectedFiles

	// Adjust cursor if it's now out of bounds
	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1)
	}

	// Update UI if GUI is initialized
	if app.g != nil {
		app.g.Update(func(g *gocui.Gui) error {
			app.refreshFilesView(g)
			app.refreshContentView(g) // Content view depends on selected files
			return nil
		})
	}
}

// shouldIncludeFile determines if a file should be included based on gitignore,
// filter mode, include/exclude patterns, and default excludes.
// Assumes mutex is held by caller (applyFilters).
func (app *App) shouldIncludeFile(relPath string, filterMode FilterMode, includes string, excludes string) bool {
	relPath = filepath.ToSlash(relPath) // Ensure slash format

	// 1. Check Gitignore first (most specific exclusion)
	if app.gitIgnoredFiles[relPath] {
		return false
	}

	// Prepare paths for pattern matching
	baseName := filepath.Base(relPath)
	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = "" // Represent root directory for matching
	} else {
		dirPath += "/" // Add trailing slash for directory matching
	}

	// 2. Check Default Excludes (applied regardless of include/exclude mode, unless overridden by include)
	// We apply default excludes *before* include mode checks, except when include mode specifically matches the file.
	isDefaultExcluded := false
	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") { // Directory pattern
			if strings.HasPrefix(dirPath, pattern) {
				isDefaultExcluded = true
				break
			}
		} else { // File pattern
			// Check against base name first
			if matched, _ := filepath.Match(pattern, baseName); matched {
				isDefaultExcluded = true
				break
			}
			// Check against full relative path
			if matched, _ := filepath.Match(pattern, relPath); matched {
				isDefaultExcluded = true
				break
			}
		}
	}

	// 3. Apply Filter Mode Logic
	if filterMode == IncludeMode {
		// If no include patterns, include everything *not* default excluded
		if includes == "" {
			return !isDefaultExcluded
		}

		// Check if the file matches any include pattern
		included := false
		for _, pattern := range strings.Split(includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			pattern = filepath.ToSlash(pattern)

			if strings.HasSuffix(pattern, "/") { // Directory pattern
				// Match if file is within the specified directory path
				if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
					included = true
					break
				}
			} else { // File pattern
				if matched, _ := filepath.Match(pattern, baseName); matched {
					included = true
					break
				}
				if matched, _ := filepath.Match(pattern, relPath); matched {
					included = true
					break
				}
			}
		}

		// Must match an include pattern AND not be default excluded
		return included && !isDefaultExcluded

	} else { // ExcludeMode (default)
		// If default excluded, definitely exclude
		if isDefaultExcluded {
			return false
		}

		// Check against user-defined excludes
		for _, pattern := range strings.Split(excludes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			pattern = filepath.ToSlash(pattern)

			if strings.HasSuffix(pattern, "/") { // Directory pattern
				if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
					return false // Exclude if in this directory
				}
			} else { // File pattern
				if matched, _ := filepath.Match(pattern, baseName); matched {
					return false
				}
				if matched, _ := filepath.Match(pattern, relPath); matched {
					return false
				}
			}
		}

		// If not gitignored, not default excluded, and not user-excluded, include it.
		return true
	}
}
