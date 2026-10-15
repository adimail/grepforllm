package internal

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
)

var binaryExtensions = map[string]bool{
	".png":     true,
	".jpg":     true,
	".jpeg":    true,
	".gif":     true,
	".ico":     true,
	".webp":    true,
	".bmp":     true,
	".tiff":    true,
	".pdf":     true,
	".doc":     true,
	".docx":    true,
	".xls":     true,
	".xlsx":    true,
	".ppt":     true,
	".pptx":    true,
	".zip":     true,
	".tar":     true,
	".gz":      true,
	".bz2":     true,
	".xz":      true,
	".7z":      true,
	".rar":     true,
	".exe":     true,
	".bin":     true,
	".dll":     true,
	".so":      true,
	".dylib":   true,
	".o":       true,
	".a":       true,
	".lib":     true,
	".pyc":     true,
	".class":   true,
	".woff":    true,
	".woff2":   true,
	".ttf":     true,
	".eot":     true,
	".otf":     true,
	".mp3":     true,
	".mp4":     true,
	".wav":     true,
	".flac":    true,
	".ogg":     true,
	".mov":     true,
	".avi":     true,
	".mkv":     true,
	".webm":    true,
	".db":      true,
	".sqlite":  true,
	".sqlite3": true,
	".wasm":    true,
}

func (app *App) loadGitStatus() map[string]GitStatus {
	statusMap := make(map[string]GitStatus)
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = app.rootDir
	out, err := cmd.Output()
	if err != nil {
		return statusMap
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		pathPart := strings.TrimSpace(line[3:])
		if strings.HasPrefix(pathPart, "\"") && strings.HasSuffix(pathPart, "\"") && len(pathPart) >= 2 {
			pathPart = pathPart[1 : len(pathPart)-1]
		}
		if idx := strings.Index(pathPart, " -> "); idx != -1 {
			pathPart = pathPart[idx+4:]
		}
		pathPart = filepath.ToSlash(pathPart)

		var status GitStatus
		if code == "??" {
			status = GitUntracked
		} else if code[0] == 'M' || code[1] == 'M' || code[0] == 'D' || code[1] == 'D' {
			status = GitModified
		} else if code[0] == 'A' || code[0] == 'R' || code[0] == 'C' {
			status = GitStaged
		} else {
			status = GitModified
		}
		statusMap[pathPart] = status
	}
	return statusMap
}

func (app *App) ListFiles() error {
	gitStatusMap := app.loadGitStatus()

	app.mutex.Lock()
	app.gitStatusMap = gitStatusMap

	var files []FileItem
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
			dirName := d.Name()
			if dirName == ".git" || dirName == "node_modules" || dirName == "vendor" ||
				dirName == ".next" || dirName == ".nuxt" || dirName == "dist" ||
				dirName == "build" || dirName == "target" || dirName == ".venv" ||
				dirName == "__pycache__" {
				return filepath.SkipDir
			}

			if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash+"/") {
				return filepath.SkipDir
			}

			segments := strings.Split(relPathSlash, "/")
			for _, pattern := range strings.Split(DefaultExcludes, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" || !strings.HasSuffix(pattern, "/") {
					continue
				}
				trimmed := strings.Trim(filepath.ToSlash(pattern), "/")
				for _, seg := range segments {
					if seg == trimmed {
						return filepath.SkipDir
					}
				}
			}

			return nil
		}

		if relPathSlash == ".gitignore" {
			return nil
		}

		if app.gitignoreMatcher != nil && app.gitignoreMatcher.Ignore(relPathSlash) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(relPathSlash))
		if binaryExtensions[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		size := info.Size()
		tokens := int(size / 4)
		if size > 0 && tokens == 0 {
			tokens = 1
		}

		status := gitStatusMap[relPathSlash]

		files = append(files, FileItem{
			RelPath:         relPathSlash,
			SizeBytes:       size,
			ModTime:         info.ModTime(),
			EstimatedTokens: tokens,
			GitStatus:       status,
		})

		return nil
	})
	if err != nil {
		app.mutex.Unlock()
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})
	app.allFiles = files

	app.applyFilters()
	return nil
}

func (app *App) applyFilters() {
	defer app.mutex.Unlock()

	filteredList := []FileItem{}
	newSelectedFiles := make(map[string]bool)

	currentFilterMode := app.filterMode
	currentIncludes := app.includes
	currentExcludes := app.excludes
	query := strings.ToLower(strings.TrimSpace(app.searchQuery))

	for _, file := range app.allFiles {
		if !app.shouldIncludeFileByFilters(file.RelPath, currentFilterMode, currentIncludes, currentExcludes) {
			continue
		}

		if query != "" && !strings.Contains(strings.ToLower(file.RelPath), query) {
			continue
		}

		switch app.gitFilterMode {
		case GitFilterChanged:
			if file.GitStatus == GitClean {
				continue
			}
		case GitFilterModified:
			if file.GitStatus != GitModified {
				continue
			}
		case GitFilterUntracked:
			if file.GitStatus != GitUntracked {
				continue
			}
		case GitFilterStaged:
			if file.GitStatus != GitStaged {
				continue
			}
		}

		filteredList = append(filteredList, file)
		if app.selectedFiles[file.RelPath] {
			newSelectedFiles[file.RelPath] = true
		}
	}

	switch app.sortMode {
	case SortByName:
		sort.Slice(filteredList, func(i, j int) bool {
			return filteredList[i].RelPath < filteredList[j].RelPath
		})
	case SortBySize:
		sort.Slice(filteredList, func(i, j int) bool {
			return filteredList[i].SizeBytes > filteredList[j].SizeBytes
		})
	case SortByTokens:
		sort.Slice(filteredList, func(i, j int) bool {
			return filteredList[i].EstimatedTokens > filteredList[j].EstimatedTokens
		})
	case SortByModTime:
		sort.Slice(filteredList, func(i, j int) bool {
			return filteredList[i].ModTime.After(filteredList[j].ModTime)
		})
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

	segments := strings.Split(relPath, "/")

	isDefaultExcluded := false
	for _, pattern := range strings.Split(DefaultExcludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)

		if strings.HasSuffix(pattern, "/") {
			trimmed := strings.Trim(pattern, "/")
			for _, seg := range segments[:len(segments)-1] {
				if seg == trimmed {
					isDefaultExcluded = true
					break
				}
			}
			if isDefaultExcluded {
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

	if filterMode == IncludeMode {
		if includes == "" {
			return !isDefaultExcluded
		}

		included := false
		for _, pattern := range strings.Split(includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			pattern = filepath.ToSlash(pattern)

			if strings.HasSuffix(pattern, "/") {
				trimmed := strings.Trim(pattern, "/")
				for _, seg := range segments[:len(segments)-1] {
					if seg == trimmed {
						included = true
						break
					}
				}
				if strings.HasPrefix(dirPath, pattern) || (pattern == "/" && dirPath == "") {
					included = true
				}
				if included {
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

		return included && !isDefaultExcluded
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
			trimmed := strings.Trim(pattern, "/")
			for _, seg := range segments[:len(segments)-1] {
				if seg == trimmed {
					return false
				}
			}
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

func (app *App) SetLoadingComplete(err error) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.isLoading = false
	app.loadingError = err
}
