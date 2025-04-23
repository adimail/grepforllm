package internal

import (
	"fmt"
	"log"
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
			log.Printf("Error scrolling files view up: %v", err)
		}
	}

	if currentLine >= oy+vy {
		newOy := currentLine - vy + 1
		err := v.SetOrigin(ox, newOy)
		if err != nil {
			log.Printf("Error scrolling files view down: %v", err)
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
			log.Printf("Error setting cursor in filter view: %v", err)
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
	app.applyFilters()

	v.Editable = false
	app.updateFilterViewContent(g)
	_, err := g.SetCurrentView(FilesViewName)
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return err
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
				log.Printf("Warning: Error reading file %s for copy: %v", fullPath, err)
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
		log.Printf("Error copying to clipboard: %v", err)
		statusMsg = "Error copying to clipboard!"
	} else {
		statusMsg = fmt.Sprintf("Copied content of %d file(s) to clipboard.", count)
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
