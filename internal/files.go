package internal

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
)

// ListFiles scans the root directory and populates the app's file list,
// respecting initial default excludes but not user filters yet.
func (app *App) ListFiles() error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	var files []string
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Printf("Warning: Permission denied accessing %s", path)
				return filepath.SkipDir
			}
			return err // Propagate other errors
		}

		// Skip the root directory itself
		if path == app.rootDir {
			return nil
		}

		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			log.Printf("Warning: Could not get relative path for %s: %v", path, err)
			return nil // Skip if we can't get relative path
		}

		// Handle directories first
		if d.IsDir() {
			// Check against default excludes (these are always checked)
			name := d.Name() + "/" // Add slash for directory matching
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}
				// Simple prefix check for default directory excludes
				if strings.HasSuffix(pattern, "/") && strings.HasPrefix(name, pattern) {
					// log.Printf("Skipping dir %s due to default exclude %s", relPath, pattern)
					return filepath.SkipDir
				}
			}
			// We don't apply user filters during the initial walk, only defaults
			return nil // Continue walking into non-excluded directories
		}

		// If it's a file, add its relative path
		files = append(files, relPath)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Strings(files) // Sort all discovered files
	app.allFiles = files
	app.applyFilters() // Apply current (initially default) filters
	return nil
}

// applyFilters updates app.fileList based on app.allFiles and current filter settings.
func (app *App) applyFilters() {
	// No lock needed here if called from methods that already hold the lock (like ListFiles or ApplyFilter handler)
	// If called directly from elsewhere, a lock might be needed. For now, assume it's called internally.

	filteredList := []string{}
	newSelectedFiles := make(map[string]bool)

	for _, file := range app.allFiles {
		if app.shouldIncludeFile(file) { // Check against current include/exclude rules
			filteredList = append(filteredList, file)
			// Preserve selection only for files that remain in the list
			if app.selectedFiles[file] {
				newSelectedFiles[file] = true
			}
		}
	}

	app.fileList = filteredList
	app.selectedFiles = newSelectedFiles

	// Ensure current line is within bounds after filtering
	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1) // Use max from utils
	}

	// Refresh UI if GUI is available
	if app.g != nil {
		app.g.Update(func(g *gocui.Gui) error {
			app.refreshFilesView(g)
			app.refreshContentView(g) // Content might change if selected files are filtered out
			return nil
		})
	}
}

// shouldIncludeFile determines if a file should be included based on current filters.
// This checks *both* file patterns and directory patterns that might contain the file.
func (app *App) shouldIncludeFile(relPath string) bool {
	// Check directory exclusion/inclusion first
	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = "" // Root directory files have no prefix
	} else {
		dirPath += "/" // Ensure trailing slash for matching
	}

	// --- Include Mode Logic ---
	if app.filterMode == IncludeMode {
		if app.includes == "" {
			return true // Include everything if no include patterns are set
		}
		included := false
		for _, pattern := range strings.Split(app.includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}

			if strings.HasSuffix(pattern, "/") { // Directory pattern
				if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
					included = true // File is in an included directory path
					break
				}
			} else if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched { // Glob on filename
				included = true
				break
			} else if matched, _ := filepath.Match(pattern, relPath); matched { // Glob on full path
				included = true
				break
			}
		}
		if !included {
			return false // Not matched by any include pattern
		}
		// If included, still check against default excludes (like .git files)
		for _, pattern := range strings.Split(DefaultExcludes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if strings.HasSuffix(pattern, "/") { // Default dir exclude
				if strings.HasPrefix(relPath+"/", pattern) { // Check if file is *within* a default excluded dir
					return false
				}
			} else if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched { // Default file exclude
				return false
			} else if matched, _ := filepath.Match(pattern, relPath); matched { // Default full path exclude
				return false
			}
		}
		return true // Included and not default-excluded

	}

	// --- Exclude Mode Logic ---
	// First, check against default excludes
	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") { // Default dir exclude
			if strings.HasPrefix(relPath+"/", pattern) {
				return false
			}
		} else if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched { // Default file exclude
			return false
		} else if matched, _ := filepath.Match(pattern, relPath); matched { // Default full path exclude
			return false
		}
	}
	// Then, check against user excludes
	for _, pattern := range strings.Split(app.excludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		if strings.HasSuffix(pattern, "/") { // Directory pattern
			if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
				return false // File is in an excluded directory path
			}
		} else if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched { // Glob on filename
			return false
		} else if matched, _ := filepath.Match(pattern, relPath); matched { // Glob on full path
			return false
		}
	}

	return true // Not excluded by any pattern
}

// Note: shouldSkipDirectory is implicitly handled by shouldIncludeFile now.
// The initial WalkDir only skips default directories. Filtering happens on the full list.
