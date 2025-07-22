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

// SwitchFocus handles Tab key presses to cycle focus between Files and Filter views.
func (app *App) SwitchFocus(g *gocui.Gui, v *gocui.View) error {
	currentView := g.CurrentView()
	nextViewName := FilesViewName // Default to Files view

	if currentView != nil {
		switch currentView.Name() {
		case FilesViewName:
			nextViewName = FilterViewName
		case FilterViewName:
			// If coming from Filter, go back to Files
			nextViewName = FilesViewName
		case ContentViewName:
			// If coming from Content, go to Filter
			nextViewName = FilterViewName
		default:
			// If somehow focus is elsewhere, reset to Files
			nextViewName = FilesViewName
		}
	}

	// Set the new current view
	if _, err := g.SetCurrentView(nextViewName); err != nil {
		return err
	}

	// Trigger layout update to reflect focus changes (frame colors)
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return nil
}

// FocusContentView switches focus to the content view. Triggered by Enter in FilesView.
func (app *App) FocusContentView(g *gocui.Gui, v *gocui.View) error {
	// Ensure the target view exists
	if _, err := g.View(ContentViewName); err != nil {
		// Content view doesn't exist? Should not happen in normal layout
		return err
	}

	// Set focus
	if _, err := g.SetCurrentView(ContentViewName); err != nil {
		return err
	}

	// Update layout to reflect focus change (frame color)
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
		// Show help: Layout will handle creation and focus setting
		g.Update(func(g *gocui.Gui) error {
			return app.Layout(g)
		})
	}
	return nil
}

func (app *App) adjustFilesViewScroll(g *gocui.Gui, v *gocui.View) {
	// This function remains the same - adjusts FilesView scroll based on app.currentLine
	currentLine := app.currentLine

	if v == nil || v.Name() != FilesViewName {
		return
	}
	ox, oy := v.Origin()
	_, vy := v.Size()

	// Scroll up if cursor is above origin
	if currentLine < oy {
		newOy := currentLine
		_ = v.SetOrigin(ox, newOy) // Ignore error for simplicity
	}

	// Scroll down if cursor is below visible area
	if currentLine >= oy+vy {
		newOy := currentLine - vy + 1
		_ = v.SetOrigin(ox, newOy) // Ignore error for simplicity
	}
}

func (app *App) updateFilterViewContent(g *gocui.Gui) {
	// This function remains the same - updates the FilterView content and cursor
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

	// Handle cursor position only if view is focused and editable
	if g.CurrentView() == v && v.Editable {
		cursorPos := len(value)
		_ = v.SetCursor(cursorPos, 0)

		// Adjust origin if cursor is out of view horizontally
		maxX, _ := v.Size()
		ox, _ := v.Origin()
		if cursorPos < ox {
			_ = v.SetOrigin(cursorPos, 0)
		} else if cursorPos >= ox+maxX {
			_ = v.SetOrigin(cursorPos-maxX+1, 0)
		}
	} else {
		// If not focused/editable, reset cursor and origin
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
		// Ensure entry exists before modifying
		if _, ok := app.cache[app.rootDir]; !ok {
			app.cache[app.rootDir] = DirectoryCache{} // Create if missing
		}
		currentEntry := app.cache[app.rootDir]
		currentEntry.Includes = app.includes
		currentEntry.Excludes = app.excludes
		currentEntry.LastOpened = time.Now()
		currentEntry.FilterMode = app.filterMode
		app.cache[app.rootDir] = currentEntry

		err := saveCache(app.cacheFilePath, app.cache)
		if err != nil {
			// Log or display error? For now, print to stderr
			fmt.Fprintf(os.Stderr, "Warning: Failed to save cache on ApplyFilter: %v\n", err)
			// Optionally update status bar here
		}
	}

	app.applyFilters()

	g.Update(func(g *gocui.Gui) error {
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

	_, err := g.SetCurrentView(FilesViewName)
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})
	return err
}

func (app *App) CursorUp(g *gocui.Gui, v *gocui.View) error {
	// This function remains the same - moves cursor up in FilesView, refreshes views
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
	app.refreshFilesView(g)   // Update file list display (cursor)
	app.refreshContentView(g) // Update content view with new file's content
	return nil
}

func (app *App) CursorDown(g *gocui.Gui, v *gocui.View) error {
	// This function remains the same - moves cursor down in FilesView, refreshes views
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
	app.refreshFilesView(g)   // Update file list display (cursor)
	app.refreshContentView(g) // Update content view with new file's content
	return nil
}

func (app *App) ToggleFilterMode(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		// This keybinding is intended for when the FilterView is active.
		return nil
	}

	app.mutex.Lock()
	if app.filterMode == ExcludeMode {
		app.filterMode = IncludeMode
	} else {
		app.filterMode = ExcludeMode
	}

	// Update cache with new mode
	if app.cacheFilePath != "" {
		if _, ok := app.cache[app.rootDir]; ok { // Ensure entry exists
			currentEntry := app.cache[app.rootDir]
			currentEntry.FilterMode = app.filterMode
			currentEntry.LastOpened = time.Now() // Update timestamp
			app.cache[app.rootDir] = currentEntry
			err := saveCache(app.cacheFilePath, app.cache)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save cache on ToggleFilterMode: %v\n", err)
			}
		}
		// If entry doesn't exist, it will be created on next ApplyFilter or app start.
		// For now, we only update if it exists.
	}
	app.mutex.Unlock()

	// REMOVED: app.updateFilterViewContent(g)
	// Rely entirely on the Layout refresh to update title and content.

	// Update the layout to refresh the title and content
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g) // Layout will now handle everything
	})

	return nil
}

func (app *App) ToggleSelect(g *gocui.Gui, v *gocui.View) error {
	// This function remains the same - toggles selection state of the current file
	if v == nil || v.Name() != FilesViewName {
		return nil
	}

	app.mutex.Lock()
	if len(app.fileList) == 0 || app.currentLine >= len(app.fileList) {
		app.mutex.Unlock()
		return nil // No file selected or list empty
	}
	selectedFile := app.fileList[app.currentLine]
	if app.selectedFiles[selectedFile] {
		delete(app.selectedFiles, selectedFile)
	} else {
		// Optional: Check against MaxSelectedFiles limit?
		// if len(app.selectedFiles) >= MaxSelectedFiles {
		//     app.mutex.Unlock()
		//     app.updateStatus(g, fmt.Sprintf("Selection limit reached (%d files)", MaxSelectedFiles))
		//     // Schedule status reset
		//     return nil
		// }
		app.selectedFiles[selectedFile] = true
	}
	app.mutex.Unlock()

	// Refresh Files view immediately to show selection change
	app.refreshFilesView(g)
	// No need to refresh content view here, as selection doesn't affect it directly anymore
	return nil
}

func (app *App) SelectAllFiles(g *gocui.Gui, v *gocui.View) error {
	// This function remains the same - selects/deselects all *visible* files
	if v == nil || v.Name() != FilesViewName {
		return nil
	}

	app.mutex.Lock()
	if len(app.fileList) == 0 {
		app.mutex.Unlock()
		return nil
	}

	// Check if all *currently visible* files are selected
	allVisibleSelected := true
	if len(app.selectedFiles) < len(app.fileList) { // Optimization: if counts differ, not all selected
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
		// Deselect all visible files
		for _, file := range app.fileList {
			delete(app.selectedFiles, file)
		}
		statusMsg = "Deselected all visible files."
	} else {
		// Select all visible files
		// Optional: Check limit before selecting all
		// if len(app.fileList) > MaxSelectedFiles { ... handle limit ... }
		for _, file := range app.fileList {
			app.selectedFiles[file] = true
		}
		statusMsg = "Selected all visible files."
	}
	app.mutex.Unlock()

	app.updateStatus(g, statusMsg)
	app.refreshFilesView(g)

	// Schedule status reset
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
	// This function remains the same - copies selected files, highlights file list
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

	// --- File List Highlight ---
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

// scrollContent scrolls the ContentViewName by a given amount (positive=down, negative=up).
// It also updates the app.contentViewOriginY state.
func (app *App) scrollContent(g *gocui.Gui, amount int) error {
	v, err := g.View(ContentViewName)
	if err != nil {
		return nil // View doesn't exist
	}

	// Prevent scrolling if filter view is focused and editable
	if cv := g.CurrentView(); cv != nil && cv.Name() == FilterViewName {
		fv, _ := g.View(FilterViewName)
		if fv != nil && fv.Editable {
			return nil // Don't scroll content when editing filter
		}
	}

	ox, oy := v.Origin()
	newOy := oy + amount

	// Clamp newOy to be non-negative
	if newOy < 0 {
		newOy = 0
	}

	// Set the new origin. gocui's SetOrigin handles clamping at the bottom.
	if err := v.SetOrigin(ox, newOy); err != nil {
		// Log error? For now, just return it.
		return err
	}

	// If SetOrigin succeeded, update the stored origin state
	// Read the actual origin back from the view in case it was clamped
	_, actualNewOy := v.Origin()
	app.mutex.Lock()
	app.contentViewOriginY = actualNewOy
	app.mutex.Unlock()

	return nil
}

// ScrollContentUp handles page up scrolling for the content view.
func (app *App) ScrollContentUp(g *gocui.Gui, v *gocui.View) error {
	// Determine scroll amount (page size)
	cv, err := g.View(ContentViewName)
	if err != nil {
		return nil // Should not happen if called from global binding
	}
	_, vy := cv.Size()
	scrollAmount := max(1, vy-1) // Page size

	return app.scrollContent(g, -scrollAmount) // Negative amount for up
}

// ScrollContentDown handles page down scrolling for the content view.
func (app *App) ScrollContentDown(g *gocui.Gui, v *gocui.View) error {
	// Determine scroll amount (page size)
	cv, err := g.View(ContentViewName)
	if err != nil {
		return nil
	}
	_, vy := cv.Size()
	scrollAmount := max(1, vy-1) // Page size

	return app.scrollContent(g, scrollAmount) // Positive amount for down
}

// ScrollContentLineUp handles single line up scrolling (e.g., for 'k' or ArrowUp in ContentView).
func (app *App) ScrollContentLineUp(g *gocui.Gui, v *gocui.View) error {
	// Only scroll if the ContentView actually has focus
	if cv := g.CurrentView(); cv == nil || cv.Name() != ContentViewName {
		return nil
	}
	return app.scrollContent(g, -1) // Scroll up by 1 line
}

// ScrollContentLineDown handles single line down scrolling (e.g., for 'j' or ArrowDown in ContentView).
func (app *App) ScrollContentLineDown(g *gocui.Gui, v *gocui.View) error {
	// Only scroll if the ContentView actually has focus
	if cv := g.CurrentView(); cv == nil || cv.Name() != ContentViewName {
		return nil
	}
	return app.scrollContent(g, 1) // Scroll down by 1 line
}
