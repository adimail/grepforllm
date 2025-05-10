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
	app.mutex.Lock() // Lock at the beginning

	var files []string
	// Walk the directory only once to find all potential files
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				// Log permission errors? For now, just skip.
				fmt.Fprintf(os.Stderr, "Warning: Skipping directory due to permission error: %s\n", path)
				return filepath.SkipDir // Skip directories we can't read
			}
			// Log other walk errors?
			fmt.Fprintf(os.Stderr, "Warning: Error accessing path %s: %v\n", path, err)
			return nil // Continue if possible, skip the problematic entry
		}

		// Skip root itself
		if path == app.rootDir {
			return nil
		}

		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			// Should not happen if path is within rootDir
			fmt.Fprintf(os.Stderr, "Warning: Could not get relative path for %s: %v\n", path, err)
			return nil
		}
		relPathSlash := filepath.ToSlash(relPath) // Use slash-separated path consistently

		// --- Directory Handling ---
		if d.IsDir() {
			// Check gitignore for directories FIRST. If ignored, skip the whole dir.
			// This is more efficient than checking every file inside.
			// Note: The matcher needs the path relative to the gitignore location (rootDir).
			// The go-gitignore library expects paths relative to the .gitignore file's location.
			// We also need to check if the directory *itself* matches a pattern.
			// Add a trailing slash for directory matching consistency with gitignore rules.
			if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash+"/") {
				return filepath.SkipDir
			}

			// Simple check for default excluded *directories* during walk
			// This prevents descending into large unwanted dirs like .git or node_modules
			dirPathWithSlash := relPathSlash + "/"
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" || !strings.HasSuffix(pattern, "/") {
					continue // Only check directory patterns here
				}
				pattern = filepath.ToSlash(pattern)
				if strings.HasPrefix(dirPathWithSlash, pattern) {
					return filepath.SkipDir
				}
			}
			// Also skip .git directory explicitly if not caught by DefaultExcludes
			// (Gitignore check above should handle this too if .git is in .gitignore)
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil // Continue walking in this directory
		}

		// --- File Handling ---

		// Skip .gitignore file itself (already handled by LoadGitignoreMatcher not walking)
		// but double-check here just in case.
		if relPathSlash == ".gitignore" {
			return nil
		}

		// Check gitignore for the file path *before* opening/reading it
		if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash) {
			return nil // Skip ignored file
		}

		// --- Binary File Check (only for files not ignored) ---
		// Optimization: Stat first to check size?
		// info, err := d.Info()
		// if err != nil {
		// 	// Error getting file info, skip
		// 	fmt.Fprintf(os.Stderr, "Warning: Could not get file info for %s: %v\n", path, err)
		// 	return nil
		// }
		// if info.Size() == 0 { // Skip empty files
		// 	return nil
		// }
		// Optional: Add a max size check here to avoid reading huge files
		// if info.Size() > MaxFileSizeBytes { return nil }

		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not open file %s: %v\n", path, err)
			return nil // Skip files we cannot open
		}
		defer file.Close()

		buffer := make([]byte, 512) // Read a small chunk to detect content type
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "Warning: Could not read file %s: %v\n", path, err)
			return nil // Skip files we cannot read
		}

		// Check if it's likely a text file
		contentType := http.DetectContentType(buffer[:n])
		// Be a bit more lenient? Allow application/json, etc.?
		// For now, stick to text/*
		if !strings.HasPrefix(contentType, "text/") {
			// Optionally log skipped binary files: log.Printf("Skipping binary file: %s (type: %s)", relPathSlash, contentType)
			return nil // Skip binary files
		}

		// If it's a text file and not ignored, add its relative path to the list
		files = append(files, relPathSlash)
		return nil
	})
	// Unlock should happen *before* calling applyFilters if applyFilters acquires lock
	// Or, applyFilters should assume lock is held. Let's assume applyFilters needs the lock.
	// app.mutex.Unlock() // Unlock before calling applyFilters
	if err != nil {
		app.mutex.Unlock() // Ensure unlock on error during walk
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Strings(files)  // Sort all discovered text files
	app.allFiles = files // Store the complete list

	// applyFilters will now use the gitignore info via shouldIncludeFile
	// It also unlocks the mutex.
	app.applyFilters() // This function now handles unlocking
	return nil
}

// applyFilters filters app.allFiles into app.fileList based on current filter settings.
// It assumes the mutex is held when called and unlocks it upon completion.
func (app *App) applyFilters() {
	defer app.mutex.Unlock() // Unlock when done

	filteredList := []string{}
	newSelectedFiles := make(map[string]bool)

	// Read filter state under lock
	currentFilterMode := app.filterMode
	currentIncludes := app.includes
	currentExcludes := app.excludes
	// gitignoreMatcher is already checked during the ListFiles walk,
	// so allFiles should already exclude gitignored files.
	// However, shouldIncludeFile still needs to handle default/user filters.

	for _, file := range app.allFiles {
		// Pass the gitignoreMatcher to shouldIncludeFile or rely on allFiles being pre-filtered?
		// Let's modify shouldIncludeFile to *only* check default/user filters,
		// assuming gitignore filtering happened during ListFiles walk.
		if app.shouldIncludeFileByFilters(file, currentFilterMode, currentIncludes, currentExcludes) {
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
		// Run UI updates in the main GUI thread
		app.g.Update(func(g *gocui.Gui) error {
			app.refreshFilesView(g)
			app.refreshContentView(g) // Content view depends on selected files
			return nil
		})
	}
}

// shouldIncludeFileByFilters determines if a file should be included based *only* on
// filter mode, include/exclude patterns, and default excludes.
// Assumes gitignore filtering was already done during the initial file walk.
// Assumes mutex is held by caller (applyFilters).
func (app *App) shouldIncludeFileByFilters(relPath string, filterMode FilterMode, includes string, excludes string) bool {
	// relPath is already slash format from ListFiles

	// Prepare paths for pattern matching
	baseName := filepath.Base(relPath)
	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = "" // Represent root directory for matching
	} else {
		dirPath += "/" // Add trailing slash for directory matching
	}

	// 1. Check Default Excludes (applied regardless of include/exclude mode, unless overridden by include)
	// We apply default excludes *before* include mode checks, except when include mode specifically matches the file.
	isDefaultExcluded := false
	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") { // Directory pattern
			// Check if the file's directory path starts with the pattern
			// Example: pattern="node_modules/", dirPath="node_modules/some_lib/" -> match
			if strings.HasPrefix(dirPath, pattern) {
				isDefaultExcluded = true
				break
			}
		} else { // File pattern
			// Check against base name first (e.g., *.log)
			if matched, _ := filepath.Match(pattern, baseName); matched {
				isDefaultExcluded = true
				break
			}
			// Check against full relative path (e.g., specific/file.txt)
			if matched, _ := filepath.Match(pattern, relPath); matched {
				isDefaultExcluded = true
				break
			}
		}
	}

	// 2. Apply Filter Mode Logic
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
				// Example: pattern="cmd/", dirPath="cmd/" -> match
				// Example: pattern="cmd/", dirPath="cmd/subdir/" -> match
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
		// Note: If a file is default excluded (e.g. in node_modules/) but matches an include pattern (e.g. *.js),
		// should it be included? Current logic says NO (must match include AND NOT be default excluded).
		// This seems reasonable.
		return included && !isDefaultExcluded

	} else { // ExcludeMode (default)
		// If default excluded, definitely exclude
		if isDefaultExcluded {
			return false
		}

		// Check against user-defined excludes (these are additional to DefaultExcludes)
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

		// If not default excluded, and not user-excluded, include it.
		return true
	}
}

func (app *App) SetLoadingComplete(err error) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.isLoading = false
	app.loadingError = err
}
