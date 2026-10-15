package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"
)

func (app *App) SwitchFocus(g *gocui.Gui, v *gocui.View) error {
	currentView := g.CurrentView()
	nextViewName := FilesViewName

	if currentView != nil {
		switch currentView.Name() {
		case FilesViewName:
			nextViewName = FilterViewName
		case FilterViewName:
			nextViewName = FilesViewName
		case ContentViewName:
			nextViewName = FilterViewName
		default:
			nextViewName = FilesViewName
		}
	}

	if _, err := g.SetCurrentView(nextViewName); err != nil {
		return err
	}

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return nil
}

func (app *App) FocusContentView(g *gocui.Gui, v *gocui.View) error {
	if _, err := g.View(ContentViewName); err != nil {
		return err
	}

	if _, err := g.SetCurrentView(ContentViewName); err != nil {
		return err
	}

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return nil
}

func (app *App) ToggleHelp(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showHelp = !app.showHelp
	app.mutex.Unlock()

	if !app.showHelp {
		_ = g.DeleteView(HelpViewName)
		previousView := FilesViewName
		_, err := g.SetCurrentView(previousView)
		g.Update(func(g *gocui.Gui) error { return app.Layout(g) })
		return err
	} else {
		g.Update(func(g *gocui.Gui) error {
			return app.Layout(g)
		})
	}
	return nil
}

func (app *App) adjustFilesViewScroll(g *gocui.Gui, v *gocui.View) {
	currentLine := app.currentLine

	if v == nil || v.Name() != FilesViewName {
		return
	}
	ox, oy := v.Origin()
	_, vy := v.Size()

	if currentLine < oy {
		newOy := currentLine
		_ = v.SetOrigin(ox, newOy)
	}

	if currentLine >= oy+vy {
		newOy := currentLine - vy + 1
		_ = v.SetOrigin(ox, newOy)
	}
}

func (app *App) updateFilterViewContent(g *gocui.Gui) {
	v, err := g.View(FilterViewName)
	if err != nil {
		return
	}

	app.mutex.Lock()
	var value string
	currentMode := app.filterMode
	if currentMode == ExcludeMode {
		value = app.excludes
	} else {
		value = app.includes
	}
	app.mutex.Unlock()

	v.Clear()
	fmt.Fprint(v, value)

	if g.CurrentView() == v && v.Editable {
		cursorPos := len(value)
		_ = v.SetCursor(cursorPos, 0)

		maxX, _ := v.Size()
		ox, _ := v.Origin()
		if cursorPos < ox {
			_ = v.SetOrigin(cursorPos, 0)
		} else if cursorPos >= ox+maxX {
			_ = v.SetOrigin(cursorPos-maxX+1, 0)
		}
	} else {
		_ = v.SetCursor(0, 0)
		_ = v.SetOrigin(0, 0)
	}
}

func (app *App) ApplyFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}

	app.mutex.Lock()

	pattern := strings.TrimSpace(v.Buffer())
	if app.filterMode == ExcludeMode {
		app.excludes = pattern
	} else {
		app.includes = pattern
	}

	if app.cacheFilePath != "" {
		if _, ok := app.cache[app.rootDir]; !ok {
			app.cache[app.rootDir] = DirectoryCache{}
		}
		currentEntry := app.cache[app.rootDir]
		currentEntry.Includes = app.includes
		currentEntry.Excludes = app.excludes
		currentEntry.LastOpened = time.Now()
		currentEntry.FilterMode = app.filterMode
		app.cache[app.rootDir] = currentEntry

		err := saveCache(app.cacheFilePath, app.cache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save cache on ApplyFilter: %v\n", err)
		}
	}

	app.applyFilters()

	g.Update(func(g *gocui.Gui) error {
		if _, err := g.SetCurrentView(FilesViewName); err != nil {
		}
		return app.Layout(g)
	})

	return nil
}

func (app *App) CancelFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}

	_, err := g.SetCurrentView(FilesViewName)
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return err
}

func (app *App) CursorUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName {
		return nil
	}
	app.mutex.Lock()
	if len(app.fileList) == 0 {
		app.mutex.Unlock()
		return nil
	}
	if app.currentLine > 0 {
		app.currentLine--
	}
	app.mutex.Unlock()
	app.refreshFilesView(g)
	app.refreshContentView(g)
	return nil
}

func (app *App) CursorDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName {
		return nil
	}
	app.mutex.Lock()
	if len(app.fileList) == 0 {
		app.mutex.Unlock()
		return nil
	}
	if app.currentLine < len(app.fileList)-1 {
		app.currentLine++
	}
	app.mutex.Unlock()
	app.refreshFilesView(g)
	app.refreshContentView(g)
	return nil
}

func (app *App) ToggleFilterMode(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}

	app.mutex.Lock()
	if app.filterMode == ExcludeMode {
		app.filterMode = IncludeMode
	} else {
		app.filterMode = ExcludeMode
	}

	if app.cacheFilePath != "" {
		if _, ok := app.cache[app.rootDir]; ok {
			currentEntry := app.cache[app.rootDir]
			currentEntry.FilterMode = app.filterMode
			currentEntry.LastOpened = time.Now()
			app.cache[app.rootDir] = currentEntry
			err := saveCache(app.cacheFilePath, app.cache)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save cache on ToggleFilterMode: %v\n", err)
			}
		}
	}
	app.mutex.Unlock()

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	return nil
}

func (app *App) ToggleSelect(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName {
		return nil
	}

	app.mutex.Lock()
	if len(app.fileList) == 0 || app.currentLine >= len(app.fileList) {
		app.mutex.Unlock()
		return nil
	}
	selectedFile := app.fileList[app.currentLine].RelPath
	if app.selectedFiles[selectedFile] {
		delete(app.selectedFiles, selectedFile)
	} else {
		app.selectedFiles[selectedFile] = true
	}
	app.mutex.Unlock()

	app.refreshFilesView(g)
	return nil
}

func (app *App) SelectAllFiles(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName {
		return nil
	}

	app.mutex.Lock()
	if len(app.fileList) == 0 {
		app.mutex.Unlock()
		return nil
	}

	allVisibleSelected := true
	if len(app.selectedFiles) < len(app.fileList) {
		allVisibleSelected = false
	} else {
		for _, file := range app.fileList {
			if !app.selectedFiles[file.RelPath] {
				allVisibleSelected = false
				break
			}
		}
	}

	statusMsg := ""
	if allVisibleSelected {
		for _, file := range app.fileList {
			delete(app.selectedFiles, file.RelPath)
		}
		statusMsg = "Deselected all visible files."
	} else {
		for _, file := range app.fileList {
			app.selectedFiles[file.RelPath] = true
		}
		statusMsg = "Selected all visible files."
	}
	app.mutex.Unlock()

	app.updateStatus(g, statusMsg)
	app.refreshFilesView(g)

	go func(msg string) {
		time.Sleep(2 * time.Second)
		g.Update(func(g *gocui.Gui) error {
			sv, err := g.View(StatusViewName)
			if err == nil && strings.HasPrefix(sv.Buffer(), msg) {
				app.resetStatus(g)
			}
			return nil
		})
	}(statusMsg)

	return nil
}

func (app *App) CopyAllSelected(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()

	if len(app.selectedFiles) == 0 {
		app.mutex.Unlock()
		app.updateStatus(g, "No files selected to copy.")
		go func() {
			time.Sleep(2 * time.Second)
			g.Update(func(g *gocui.Gui) error {
				sv, err := g.View(StatusViewName)
				if err == nil && strings.HasPrefix(sv.Buffer(), "No files") {
					app.resetStatus(g)
				}
				return nil
			})
		}()
		return nil
	}

	selectedFileCopy := make(map[string]bool, len(app.selectedFiles))
	for k, v := range app.selectedFiles {
		selectedFileCopy[k] = v
	}
	fileListCopy := make([]FileItem, len(app.fileList))
	copy(fileListCopy, app.fileList)
	rootDirCopy := app.rootDir

	app.mutex.Unlock()

	var contentBuilder strings.Builder
	count := 0
	var totalTokens int
	var totalBytes int64

	for _, item := range fileListCopy {
		if selectedFileCopy[item.RelPath] {
			fullPath := filepath.Join(rootDirCopy, item.RelPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("==========================\nFILE: %s\n==========================\n", item.RelPath)

			contentBuilder.WriteString(separator)
			if err != nil {
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				contentBuilder.WriteString("\n")
				contentBuilder.WriteString(string(fileContent))
				if !strings.HasSuffix(string(fileContent), "\n") {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.WriteString("\n")
			}
			count++
			totalTokens += item.EstimatedTokens
			totalBytes += item.SizeBytes
		}
	}

	content := contentBuilder.String()
	err := clipboard.WriteAll(content)

	var statusMsg string
	if err != nil {
		statusMsg = "Error copying to clipboard!"
	} else {
		statusMsg = fmt.Sprintf("Copied %d file(s) (~%s tokens, %s) to clipboard.", count, formatTokens(totalTokens), formatBytes(totalBytes))
	}

	if err == nil && count > 0 {
		app.mutex.Lock()
		app.isCopyHighlightActive = true
		app.mutex.Unlock()

		g.Update(func(g *gocui.Gui) error {
			app.refreshFilesView(g)
			return nil
		})

		time.AfterFunc(350*time.Millisecond, func() {
			app.mutex.Lock()
			app.isCopyHighlightActive = false
			app.mutex.Unlock()
			g.Update(func(g *gocui.Gui) error {
				app.refreshFilesView(g)
				return nil
			})
		})
	}

	app.updateStatus(g, statusMsg)

	return nil
}

func (app *App) scrollContent(g *gocui.Gui, amount int) error {
	v, err := g.View(ContentViewName)
	if err != nil {
		return nil
	}

	if cv := g.CurrentView(); cv != nil && cv.Name() == FilterViewName {
		fv, _ := g.View(FilterViewName)
		if fv != nil && fv.Editable {
			return nil
		}
	}

	ox, oy := v.Origin()
	newOy := oy + amount

	if newOy < 0 {
		newOy = 0
	}

	if err := v.SetOrigin(ox, newOy); err != nil {
		return err
	}

	_, actualNewOy := v.Origin()
	app.mutex.Lock()
	app.contentViewOriginY = actualNewOy
	app.mutex.Unlock()

	return nil
}

func (app *App) ScrollContentUp(g *gocui.Gui, v *gocui.View) error {
	cv, err := g.View(ContentViewName)
	if err != nil {
		return nil
	}
	_, vy := cv.Size()
	scrollAmount := max(1, vy-1)

	return app.scrollContent(g, -scrollAmount)
}

func (app *App) ScrollContentDown(g *gocui.Gui, v *gocui.View) error {
	cv, err := g.View(ContentViewName)
	if err != nil {
		return nil
	}
	_, vy := cv.Size()
	scrollAmount := max(1, vy-1)

	return app.scrollContent(g, scrollAmount)
}

func (app *App) ScrollContentLineUp(g *gocui.Gui, v *gocui.View) error {
	if cv := g.CurrentView(); cv == nil || cv.Name() != ContentViewName {
		return nil
	}
	return app.scrollContent(g, -1)
}

func (app *App) ScrollContentLineDown(g *gocui.Gui, v *gocui.View) error {
	if cv := g.CurrentView(); cv == nil || cv.Name() != ContentViewName {
		return nil
	}
	return app.scrollContent(g, 1)
}
