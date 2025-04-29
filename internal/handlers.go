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
	if currentView != nil && currentView.Name() == FilesViewName {
		nextViewName = FilterViewName
	}

	if nextViewName == FilterViewName {
		fv, err := g.View(FilterViewName)
		if err != nil {
			return err
		}
		if _, err := g.SetCurrentView(FilterViewName); err != nil {
			return err
		}
		fv.Editable = true
		app.updateFilterViewContent(g)
	} else {
		if fv, err := g.View(FilterViewName); err == nil {
			fv.Editable = false
			app.updateFilterViewContent(g)
		}
		if _, err := g.SetCurrentView(FilesViewName); err != nil {
			return err
		}
	}

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return nil
}

func (app *App) ToggleHelp(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showHelp = !app.showHelp
	currentViewName := ""
	if cv := g.CurrentView(); cv != nil {
		currentViewName = cv.Name()
	}
	app.mutex.Unlock()

	if !app.showHelp {
		_ = g.DeleteView(HelpViewName)
		previousView := FilesViewName
		if currentViewName != HelpViewName && currentViewName != "" {
			if fv, err := g.View(FilterViewName); err == nil {
				fv.Editable = false
			}
		}
		_, err := g.SetCurrentView(previousView)
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
		err := v.SetOrigin(ox, newOy)
		if err != nil {
		}
	}

	if currentLine >= oy+vy {
		newOy := currentLine - vy + 1
		err := v.SetOrigin(ox, newOy)
		if err != nil {
		}
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
		err = v.SetCursor(cursorPos, 0)
		if err != nil {
		}
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

	// --- Update Cache ---
	if app.cacheFilePath != "" {
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
		fv, err := g.View(FilterViewName)
		if err == nil {
			fv.Editable = false
			app.updateFilterViewContent(g)
		}

		if _, err := g.SetCurrentView(FilesViewName); err != nil {
			// Log or handle error
		}

		return app.Layout(g)
	})

	return nil
}

func (app *App) CancelFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}

	v.Editable = false

	app.updateFilterViewContent(g)

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
	app.mutex.Unlock()
	app.updateFilterViewContent(g)
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
	selectedFile := app.fileList[app.currentLine]
	if app.selectedFiles[selectedFile] {
		delete(app.selectedFiles, selectedFile)
	} else {
		app.selectedFiles[selectedFile] = true
	}
	app.mutex.Unlock()

	app.refreshViews(g)
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

	allSelected := true
	if len(app.selectedFiles) != len(app.fileList) {
		allSelected = false
	} else {
		for _, file := range app.fileList {
			if !app.selectedFiles[file] {
				allSelected = false
				break
			}
		}
	}

	statusMsg := ""
	if allSelected {
		app.selectedFiles = make(map[string]bool)
		statusMsg = "Deselected all visible files."
	} else {
		for _, file := range app.fileList {
			app.selectedFiles[file] = true
		}
		statusMsg = "Selected all visible files."
	}
	app.mutex.Unlock()

	app.updateStatus(g, statusMsg)
	app.refreshViews(g)

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
		go func() {
			g.Update(func(g *gocui.Gui) error {
				cv, err := g.View(ContentViewName)
				if err == nil {
					cv.BgColor = gocui.ColorYellow
					cv.FgColor = gocui.ColorBlack
				} else if err != gocui.ErrUnknownView {
				}
				return nil
			})

			time.Sleep(250 * time.Millisecond)

			g.Update(func(g *gocui.Gui) error {
				cv, err := g.View(ContentViewName)
				if err == nil {
					cv.BgColor = gocui.ColorDefault
					cv.FgColor = gocui.ColorDefault
				} else if err != gocui.ErrUnknownView {
				}
				return nil
			})
		}()
	}

	app.updateStatus(g, statusMsg)

	go func(msg string) {
		time.Sleep(3 * time.Second)
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

func (app *App) scrollContent(g *gocui.Gui, direction int) error {
	v, err := g.View(ContentViewName)
	if err != nil {
		return nil
	}
	ox, oy := v.Origin()
	_, vy := v.Size()

	scrollAmount := max(1, vy-1)

	newOy := oy + (direction * scrollAmount)

	if newOy < 0 {
		newOy = 0
	}

	return v.SetOrigin(ox, newOy)
}

func (app *App) ScrollContentUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollContent(g, -1)
}

func (app *App) ScrollContentDown(g *gocui.Gui, v *gocui.View) error {
	if cv := g.CurrentView(); cv != nil && cv.Name() == FilterViewName {
		return nil
	}
	return app.scrollContent(g, 1)
}

func (app *App) ShowPreview(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName {
		return nil // Should only trigger from FilesView
	}

	app.mutex.Lock()
	if len(app.fileList) == 0 || app.currentLine >= len(app.fileList) {
		app.mutex.Unlock()
		return nil // No file selected or list empty
	}
	selectedFileRelPath := app.fileList[app.currentLine]
	rootDir := app.rootDir
	app.mutex.Unlock() // Unlock before file I/O

	fullPath := filepath.Join(rootDir, selectedFileRelPath)
	contentBytes, err := os.ReadFile(fullPath)
	content := ""
	if err != nil {
		content = fmt.Sprintf("Error reading file:\n%v", err)
	} else {
		// Basic check for binary content (can be improved)
		if !isLikelyText(contentBytes) {
			content = fmt.Sprintf("File appears to be binary:\n%s", selectedFileRelPath)
		} else {
			content = string(contentBytes)
		}
	}

	app.mutex.Lock()
	app.previewFile = selectedFileRelPath
	app.previewContent = content
	app.isPreviewOpen = true
	app.previewOriginY = 0 // Reset scroll on new preview
	app.mutex.Unlock()

	// Trigger layout update which will create/show the view
	g.Update(func(g *gocui.Gui) error {
		// Layout will handle view creation and focus setting
		return app.Layout(g)
	})

	return nil
}

func (app *App) ClosePreview(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.isPreviewOpen = false
	app.previewContent = "" // Clear content
	app.previewFile = ""
	app.mutex.Unlock()

	if err := g.DeleteView(PreviewViewName); err != nil && err != gocui.ErrUnknownView {
		// Log or handle error if needed, but usually ErrUnknownView is fine
	}
	if _, err := g.SetCurrentView(FilesViewName); err != nil {
		return err // Should generally succeed
	}
	// Optional: Trigger layout if needed, but deleting view + setting focus might suffice
	// g.Update(func(g *gocui.Gui) error { return app.Layout(g) })
	return nil
}

func (app *App) scrollPreview(g *gocui.Gui, v *gocui.View, direction int) error {
	if v == nil || v.Name() != PreviewViewName {
		return nil // Only scroll the preview view
	}
	ox, oy := v.Origin()
	newOy := oy + direction
	if newOy < 0 {
		newOy = 0
	}

	// Set the view's origin
	if err := v.SetOrigin(ox, newOy); err != nil {
		return err
	}

	// Update the stored origin in app state
	app.mutex.Lock()
	app.previewOriginY = newOy
	app.mutex.Unlock()

	return nil
}

func (app *App) ScrollPreviewUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollPreview(g, v, -1) // Scroll up by 1 line
}

func (app *App) ScrollPreviewDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollPreview(g, v, 1) // Scroll down by 1 line
}

// Helper function (optional, can be refined)
func isLikelyText(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
