package internal

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
)

func (app *App) ListFiles() error {
	app.mutex.Lock()

	var files []string
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Printf("Warning: Permission denied accessing %s", path)
				return filepath.SkipDir
			}
			return err
		}
		if path == app.rootDir {
			return nil
		}
		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			log.Printf("Warning: Could not get relative path for %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			relDirPath := filepath.ToSlash(relPath) + "/"
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}
				if strings.HasSuffix(pattern, "/") {
					pattern = filepath.ToSlash(pattern)
					if strings.HasPrefix(relDirPath, pattern) {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}

		// --- Binary File Check ---
		file, err := os.Open(path)
		if err != nil {
			log.Printf("Warning: Could not open file %s to check type: %v", path, err)
			return nil
		}
		defer file.Close()

		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("Warning: Could not read from file %s to check type: %v", path, err)
			return nil
		}

		contentType := http.DetectContentType(buffer[:n])
		if !strings.HasPrefix(contentType, "text/") {
			return nil
		}

		files = append(files, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		app.mutex.Unlock()
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Strings(files)
	app.allFiles = files

	app.applyFilters()
	return nil
}

func (app *App) applyFilters() {
	defer app.mutex.Unlock()

	filteredList := []string{}
	newSelectedFiles := make(map[string]bool)

	currentFilterMode := app.filterMode
	currentIncludes := app.includes
	currentExcludes := app.excludes

	for _, file := range app.allFiles {
		if app.shouldIncludeFile(file, currentFilterMode, currentIncludes, currentExcludes) {
			filteredList = append(filteredList, file)
			if app.selectedFiles[file] {
				newSelectedFiles[file] = true
			}
		}
	}

	app.fileList = filteredList
	app.selectedFiles = newSelectedFiles

	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1)
	}

	if app.g != nil {
		app.g.Update(func(g *gocui.Gui) error {
			app.refreshFilesView(g)
			app.refreshContentView(g)
			return nil
		})
	}
}

func (app *App) shouldIncludeFile(relPath string, filterMode FilterMode, includes string, excludes string) bool {
	relPath = filepath.ToSlash(relPath)
	baseName := filepath.Base(relPath)

	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	} else {
		dirPath += "/"
	}

	if filterMode == IncludeMode {
		if includes == "" {
			return true
		}
		included := false
		for _, pattern := range strings.Split(includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			pattern = filepath.ToSlash(pattern)

			if strings.HasSuffix(pattern, "/") {
				if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
					included = true
					break
				}
			} else {
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
		if !included {
			return false
		}
		for _, pattern := range strings.Split(DefaultExcludes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			pattern = filepath.ToSlash(pattern)

			if strings.HasSuffix(pattern, "/") {
				if strings.HasPrefix(dirPath, pattern) {
					return false
				}
			} else {
				if matched, _ := filepath.Match(pattern, baseName); matched {
					return false
				}
				if matched, _ := filepath.Match(pattern, relPath); matched {
					return false
				}
			}
		}
		return true
	}

	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(dirPath, pattern) {
				return false
			}
		} else {
			if matched, _ := filepath.Match(pattern, baseName); matched {
				return false
			}
			if matched, _ := filepath.Match(pattern, relPath); matched {
				return false
			}
		}
	}
	for _, pattern := range strings.Split(excludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
				return false
			}
		} else {
			if matched, _ := filepath.Match(pattern, baseName); matched {
				return false
			}
			if matched, _ := filepath.Match(pattern, relPath); matched {
				return false
			}
		}
	}

	return true
}
