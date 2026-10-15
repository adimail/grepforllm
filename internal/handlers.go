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
			if app.filterMode == IncludeMode {
				nextViewName = FilterIncludeViewName
			} else {
				nextViewName = FilterExcludeViewName
			}
		case FilterIncludeViewName:
			nextViewName = FilterExcludeViewName
		case FilterExcludeViewName:
			nextViewName = FilesViewName
		case ContentViewName:
			if app.filterMode == IncludeMode {
				nextViewName = FilterIncludeViewName
			} else {
				nextViewName = FilterExcludeViewName
			}
		default:
			nextViewName = FilesViewName
		}
	}

	nv, err := g.SetCurrentView(nextViewName)
	if err != nil {
		return err
	}

	if nextViewName == FilterIncludeViewName || nextViewName == FilterExcludeViewName {
		content := strings.TrimSuffix(nv.Buffer(), "\n")
		cursorPos := len(content)
		_ = nv.SetCursor(cursorPos, 0)
		app.adjustFilterCursor(g, nv)
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
	if v == nil || v.Name() != FilesViewName {
		return
	}

	app.mutex.Lock()
	currentLine := app.currentLine
	totalLines := len(app.fileList)
	app.mutex.Unlock()

	ox, oy := v.Origin()
	_, vy := v.Size()
	margin := 5

	if totalLines <= vy {
		_ = v.SetOrigin(ox, 0)
		return
	}

	newOy := oy
	if currentLine < oy+margin {
		newOy = currentLine - margin
	} else if currentLine >= oy+vy-margin {
		newOy = currentLine - vy + 1 + margin
	}

	if newOy < 0 {
		newOy = 0
	}
	if newOy > totalLines-vy {
		newOy = totalLines - vy
	}

	if newOy != oy {
		_ = v.SetOrigin(ox, newOy)
	}
}

func (app *App) updateFilterViewContent(g *gocui.Gui) {
	app.mutex.Lock()
	includes := app.includes
	excludes := app.excludes
	app.mutex.Unlock()

	currentView := g.CurrentView()

	if v, err := g.View(FilterIncludeViewName); err == nil {
		if currentView == nil || currentView.Name() != FilterIncludeViewName {
			v.Clear()
			fmt.Fprint(v, includes)
			content := strings.TrimSuffix(v.Buffer(), "\n")
			_ = v.SetCursor(len(content), 0)
		}
		app.adjustFilterCursor(g, v)
	}

	if v, err := g.View(FilterExcludeViewName); err == nil {
		if currentView == nil || currentView.Name() != FilterExcludeViewName {
			v.Clear()
			fmt.Fprint(v, excludes)
			content := strings.TrimSuffix(v.Buffer(), "\n")
			_ = v.SetCursor(len(content), 0)
		}
		app.adjustFilterCursor(g, v)
	}
}

func (app *App) adjustFilterCursor(g *gocui.Gui, v *gocui.View) {
	cx, _ := v.Cursor()
	ox, _ := v.Origin()
	maxX, _ := v.Size()

	if cx < ox {
		_ = v.SetOrigin(cx, 0)
	} else if cx >= ox+maxX {
		_ = v.SetOrigin(cx-maxX+1, 0)
	}
}

func (app *App) ApplyFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}

	app.mutex.Lock()
	if v.Name() == FilterIncludeViewName {
		app.includes = strings.TrimSpace(v.Buffer())
	} else if v.Name() == FilterExcludeViewName {
		app.excludes = strings.TrimSpace(v.Buffer())
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
		_ = saveCache(app.cacheFilePath, app.cache)
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
	if v == nil {
		return nil
	}

	app.mutex.Lock()
	inc := app.includes
	exc := app.excludes
	app.mutex.Unlock()

	if v.Name() == FilterIncludeViewName {
		v.Clear()
		fmt.Fprint(v, inc)
	} else {
		v.Clear()
		fmt.Fprint(v, exc)
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
			_ = saveCache(app.cacheFilePath, app.cache)
		}
	}
	app.mutex.Unlock()

	g.Update(func(g *gocui.Gui) error {
		if app.filterMode == IncludeMode {
			nv, _ := g.SetCurrentView(FilterIncludeViewName)
			if nv != nil {
				content := strings.TrimSuffix(nv.Buffer(), "\n")
				_ = nv.SetCursor(len(content), 0)
			}
		} else {
			nv, _ := g.SetCurrentView(FilterExcludeViewName)
			if nv != nil {
				content := strings.TrimSuffix(nv.Buffer(), "\n")
				_ = nv.SetCursor(len(content), 0)
			}
		}
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
	selectedFile := app.fileList[app.currentLine]
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
			if !app.selectedFiles[file] {
				allVisibleSelected = false
				break
			}
		}
	}

	statusMsg := ""
	if allVisibleSelected {
		for _, file := range app.fileList {
			delete(app.selectedFiles, file)
		}
		statusMsg = "Deselected all visible files."
	} else {
		for _, file := range app.fileList {
			app.selectedFiles[file] = true
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
	fileListCopy := make([]string, len(app.fileList))
	copy(fileListCopy, app.fileList)
	rootDirCopy := app.rootDir

	app.mutex.Unlock()

	var contentBuilder strings.Builder
	count := 0

	for _, relPath := range fileListCopy {
		if selectedFileCopy[relPath] {
			fullPath := filepath.Join(rootDirCopy, relPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("==========================\nFILE: %s\n==========================\n", relPath)

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
		}
	}

	content := contentBuilder.String()
	err := clipboard.WriteAll(content)

	var statusMsg string
	if err != nil {
		statusMsg = "Error copying to clipboard!"
	} else {
		statusMsg = fmt.Sprintf("Copied content of %d file(s) to clipboard.", count)
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
