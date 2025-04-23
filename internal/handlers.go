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

func (app *App) ToggleHelp(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showHelp = !app.showHelp
	currentViewName := ""
	if cv := g.CurrentView(); cv != nil {
		currentViewName = cv.Name()
	}
	app.mutex.Unlock()

	if !app.showHelp {
		// If closing help, ensure focus returns to the previous view if it wasn't help itself
		// Or default to files view if something went wrong
		_ = g.DeleteView(HelpViewName) // Ensure deleted
		previousView := FilesViewName  // Default return view
		if currentViewName != HelpViewName && currentViewName != "" {
			// Ideally, store the view name before opening help, but this is simpler
			// For now, just go back to files view as it's the main interaction point
		}
		_, err := g.SetCurrentView(previousView)
		return err
	}
	// Layout function will handle creating/focusing help view when showHelp is true
	return nil
}

// ToggleFilterMode switches between Include and Exclude modes in the filter view.
func (app *App) ToggleFilterMode(g *gocui.Gui, v *gocui.View) error {
	// This handler is only active when FilterView is focused
	if v == nil || v.Name() != FilterViewName {
		return nil // Should not happen if keybinding is set correctly
	}

	app.mutex.Lock()
	if app.filterMode == ExcludeMode {
		app.filterMode = IncludeMode
	} else {
		app.filterMode = ExcludeMode
	}
	app.mutex.Unlock()

	app.updateFilterViewContent(g) // Update the view text
	app.refreshFilesView(g)        // Update files view title

	return nil
}

// updateFilterViewContent updates the filter view's text based on the current mode.
func (app *App) updateFilterViewContent(g *gocui.Gui) {
	v, err := g.View(FilterViewName)
	if err != nil {
		return // View doesn't exist yet or error
	}
	v.Clear()
	var prompt, value string
	if app.filterMode == ExcludeMode {
		prompt = "Exclude: "
		value = app.excludes
	} else {
		prompt = "Include: "
		value = app.includes
	}
	fmt.Fprint(v, prompt+value)

	// Only set cursor if this view is the current one
	if g.CurrentView() == v {
		_ = v.SetCursor(len(prompt)+len(value), 0)
		_ = v.SetOrigin(0, 0)
	}

	return
}

// ApplyFilter reads the filter input, applies it, and closes the filter view.
func (app *App) ApplyFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	buffer := strings.TrimSpace(v.Buffer()) // Get trimmed content

	// Parse the input based on the current mode
	if app.filterMode == ExcludeMode {
		prefix := "Exclude: "
		if strings.HasPrefix(buffer, prefix) {
			app.excludes = strings.TrimSpace(strings.TrimPrefix(buffer, prefix))
		} else {
			app.excludes = buffer // Allow entering just the pattern
		}
	} else { // IncludeMode
		prefix := "Include: "
		if strings.HasPrefix(buffer, prefix) {
			app.includes = strings.TrimSpace(strings.TrimPrefix(buffer, prefix))
		} else {
			app.includes = buffer // Allow entering just the pattern
		}
	}

	// Apply the filter logic (which refreshes views)
	app.applyFilters() // This function now handles UI refresh internally

	// Return focus to files view
	app.updateFilterViewContent(g) // Reset prompt/value in case it was only partially typed
	_, err := g.SetCurrentView(FilesViewName)
	return err
}

// CancelFilter closes the filter view without applying changes.
func (app *App) CancelFilter(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilterViewName {
		return nil
	}
	// No state change on cancel, just potentially reset view content and return focus
	app.updateFilterViewContent(g) // Reset content to reflect actual filter value

	// Return focus to files view
	_, err := g.SetCurrentView(FilesViewName)
	return err
}

// CursorUp moves the selection cursor up in the files view.
func (app *App) CursorUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName || len(app.fileList) == 0 {
		return nil
	}
	app.mutex.Lock()
	if app.currentLine > 0 {
		app.currentLine--
	}
	app.mutex.Unlock()

	// Refresh view to show new cursor position and potentially scroll
	app.refreshFilesView(g) // refresh handles scroll adjustment now
	return nil
}

// CursorDown moves the selection cursor down in the files view.
func (app *App) CursorDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName || len(app.fileList) == 0 {
		return nil
	}
	app.mutex.Lock()
	if app.currentLine < len(app.fileList)-1 {
		app.currentLine++
	}
	app.mutex.Unlock()

	// Refresh view to show new cursor position and potentially scroll
	app.refreshFilesView(g) // refresh handles scroll adjustment now
	return nil
}

// adjustFilesViewScroll ensures the current line is visible in the files view.
// Called by refreshFilesView.
func (app *App) adjustFilesViewScroll(g *gocui.Gui, v *gocui.View) {
	if v == nil || v.Name() != FilesViewName {
		return
	}
	ox, oy := v.Origin()
	_, vy := v.Size() // View height

	// If cursor is above the view
	if app.currentLine < oy {
		_ = v.SetOrigin(ox, app.currentLine)
	}
	// If cursor is below the view
	if app.currentLine >= oy+vy {
		_ = v.SetOrigin(ox, app.currentLine-vy+1)
	}
}

// ToggleSelect toggles the selection state of the currently highlighted file.
func (app *App) ToggleSelect(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName || len(app.fileList) == 0 || app.currentLine >= len(app.fileList) {
		return nil
	}

	app.mutex.Lock()
	selectedFile := app.fileList[app.currentLine]
	if app.selectedFiles[selectedFile] {
		delete(app.selectedFiles, selectedFile)
	} else {
		app.selectedFiles[selectedFile] = true
	}
	app.mutex.Unlock()

	// Refresh both views
	app.refreshFilesView(g)
	app.refreshContentView(g)
	return nil
}

// SelectAllFiles selects all files currently visible in the file list.
func (app *App) SelectAllFiles(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != FilesViewName || len(app.fileList) == 0 {
		return nil
	}
	app.mutex.Lock()
	// Check if all are already selected to maybe implement deselect all?
	// For now, just select all.
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

	if allSelected {
		// If all are selected, deselect all
		app.selectedFiles = make(map[string]bool)
		app.updateStatus(g, "Deselected all visible files.")
	} else {
		// Otherwise, select all
		for _, file := range app.fileList {
			app.selectedFiles[file] = true
		}
		app.updateStatus(g, "Selected all visible files.")
	}
	app.mutex.Unlock()

	app.refreshViews(g) // Refresh both file and content views
	return nil
}

// CopyAllSelected copies the formatted content of all selected files to the clipboard.
func (app *App) CopyAllSelected(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock() // Lock while reading files and selection map
	defer app.mutex.Unlock()

	if len(app.selectedFiles) == 0 {
		app.updateStatus(g, "No files selected to copy.")
		// Reset status after a delay
		go func() {
			time.Sleep(2 * time.Second)
			g.Update(func(g *gocui.Gui) error {
				// Check if status still shows the message before resetting
				sv, err := g.View(StatusViewName)
				if err == nil && strings.HasPrefix(sv.Buffer(), "No files") {
					app.resetStatus(g)
				}
				return nil
			})
		}()
		return nil
	}

	var contentBuilder strings.Builder
	count := 0

	// Iterate through fileList to maintain order, check selection map
	for _, relPath := range app.fileList {
		if app.selectedFiles[relPath] {
			fullPath := filepath.Join(app.rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("==========================\nFILE: %s\n==========================\n", relPath)

			contentBuilder.WriteString(separator)
			if err != nil {
				log.Printf("Warning: Error reading file %s for copy: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				contentBuilder.WriteString("\n") // Add newline before content
				contentBuilder.WriteString(string(fileContent))
				// Add newline after content only if content doesn't end with one
				if !strings.HasSuffix(string(fileContent), "\n") {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.WriteString("\n") // Extra newline between files
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
	// Reset status after a delay
	go func() {
		time.Sleep(3 * time.Second)
		g.Update(func(g *gocui.Gui) error {
			// Check if status still shows the copy message before resetting
			sv, err := g.View(StatusViewName)
			if err == nil && strings.HasPrefix(sv.Buffer(), statusMsg) {
				app.resetStatus(g)
			}
			return nil
		})
	}()

	return nil // Don't return clipboard error to main loop
}

// SwitchFilterModeAndRefresh toggles the filter mode (Include/Exclude)
// and immediately applies the filter based on the new mode.
// Intended to be called from the FilesView.
func (app *App) SwitchFilterModeAndRefresh(g *gocui.Gui, v *gocui.View) error {
	// Should only be called when FilesView is active, but check just in case
	if v != nil && v.Name() != FilesViewName {
		return nil
	}

	app.mutex.Lock()
	if app.filterMode == ExcludeMode {
		app.filterMode = IncludeMode
	} else {
		app.filterMode = ExcludeMode
	}
	// Apply filter immediately with the new mode
	app.applyFilters() // This refreshes files view too
	app.mutex.Unlock()

	// Update the filter view's displayed text/prompt AFTER applying filters
	app.updateFilterViewContent(g)

	return nil
}

// FocusFilterView sets the focus to the filter input view.
func (app *App) FocusFilterView(g *gocui.Gui, v *gocui.View) error {
	fv, err := g.View(FilterViewName)
	if err != nil {
		return err // Should not happen if layout is correct
	}
	_, err = g.SetCurrentView(FilterViewName)
	_ = fv.SetCursor(len(fv.Buffer()), 0) // Move cursor to end
	return err
}

// scrollContent scrolls the content view up/down by a fraction of the page height.
func (app *App) scrollContent(g *gocui.Gui, direction int) error {
	v, err := g.View(ContentViewName)
	if err != nil {
		return nil // View might not exist
	}
	ox, oy := v.Origin()
	_, vy := v.Size()

	// Calculate scroll amount (e.g., half page or full page)
	// scrollAmount := vy / 2 // Half page
	scrollAmount := max(1, vy-1) // Almost full page (like PgUp/PgDn)

	newOy := oy + (direction * scrollAmount)

	// Clamp scrolling within buffer limits
	if newOy < 0 {
		newOy = 0
	}

	// Check against the total number of lines to prevent scrolling past the end
	numLines := v.LinesHeight() // Use LinesHeight which considers wrapped lines
	if numLines <= vy {
		newOy = 0 // Don't scroll if content fits
	} else {
		maxOy := numLines - vy
		if newOy > maxOy {
			newOy = maxOy
		}
	}

	return v.SetOrigin(ox, newOy)
}

// ScrollContentUp scrolls the content view up.
func (app *App) ScrollContentUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollContent(g, -1) // -1 for up
}

// ScrollContentDown scrolls the content view down.
func (app *App) ScrollContentDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollContent(g, 1) // 1 for down
}
