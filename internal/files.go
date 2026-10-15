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

func (app *App) ListFiles() error {
	app.mutex.Lock()

	var files []string
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return nil
		}

		if path == app.rootDir {
			return nil
		}

		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			return nil
		}
		relPathSlash := filepath.ToSlash(relPath)

		if d.IsDir() {
			if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash+"/") {
				return filepath.SkipDir
			}

			dirPathWithSlash := relPathSlash + "/"
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" || !strings.HasSuffix(pattern, "/") {
					continue
				}
				pattern = filepath.ToSlash(pattern)
				if strings.HasPrefix(dirPathWithSlash, pattern) {
					return filepath.SkipDir
				}
			}
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if relPathSlash == ".gitignore" {
			return nil
		}

		if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil
		}

		contentType := http.DetectContentType(buffer[:n])
		if !strings.HasPrefix(contentType, "text/") {
			return nil
		}

		files = append(files, relPathSlash)
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
		if app.shouldIncludeFileByFilters(file, currentFilterMode, currentIncludes, currentExcludes) {
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

func (app *App) shouldIncludeFileByFilters(relPath string, filterMode FilterMode, includes string, excludes string) bool {
	baseName := filepath.Base(relPath)
	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	} else {
		dirPath += "/"
	}

	isDefaultExcluded := false
	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(dirPath, pattern) {
				isDefaultExcluded = true
				break
			}
		} else {
			if matched, _ := filepath.Match(pattern, baseName); matched {
				isDefaultExcluded = true
				break
			}
			if matched, _ := filepath.Match(pattern, relPath); matched {
				isDefaultExcluded = true
				break
			}
		}
	}

	if isDefaultExcluded {
		return false
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

	if filterMode == IncludeMode && includes != "" {
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
		return included
	}

	return true
}

func (app *App) SetLoadingComplete(err error) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.isLoading = false
	app.loadingError = err
}
