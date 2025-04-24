package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"
)

func (app *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	filesWidth := maxX / 3
	pathHeight := 2
	filterHeight := 3
	statusBarHeight := 2

	// --- Y coordinates ---
	pathY0 := 0
	pathY1 := pathY0 + pathHeight

	filesY0 := pathY1 + 1
	filesViewY1 := maxY - statusBarHeight - filterHeight

	filterY0 := filesViewY1 + 1
	filterY1 := filterY0 + filterHeight - 1

	contentViewY1 := maxY - statusBarHeight

	statusBarY0 := maxY - statusBarHeight
	statusBarY1 := maxY

	currentView := g.CurrentView()
	currentViewName := ""
	if currentView != nil {
		currentViewName = currentView.Name()
	}

	if pv, err := g.SetView(PathViewName, 0, pathY0, filesWidth, pathY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		pv.Title = " Directory "
		pv.Editable = false
		pv.Wrap = false
		pv.Frame = true
		pv.FrameColor = gocui.ColorBlue
		pv.FgColor = gocui.ColorMagenta
		fmt.Fprint(pv, app.rootDir)
	}
	// --- Files View ---
	if v, err := g.SetView(FilesViewName, 0, filesY0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Files "
		v.Highlight = true
		v.SelBgColor = gocui.ColorDefault
		v.SelFgColor = gocui.ColorCyan | gocui.AttrBold
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false

		// Initial population if view is just created
		app.refreshFilesView(g)

		// Set initial focus if no view is focused
		if g.CurrentView() == nil {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
			currentViewName = FilesViewName // Update currentViewName after setting focus
		}
	} else {
		app.refreshFilesView(g)
	}

	// --- Filter View ---
	// Adjusted Y coordinates: filterY0 and filterY1
	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Wrap = false
		v.Editor = gocui.DefaultEditor
		v.FgColor = gocui.ColorCyan

		// Initial population
		app.updateFilterViewContent(g)
	} else {
		app.mutex.Lock()
		modeStr := "Exclude"
		if app.filterMode == IncludeMode {
			modeStr = "Include"
		}
		app.mutex.Unlock()
		v.Title = fmt.Sprintf(" Filter: %s (Enter: Apply, Ctrl+F: Mode) ", modeStr)

		// Update frame and text color based on focus
		if currentViewName == FilterViewName {
			v.FrameColor = gocui.ColorGreen
			v.FgColor = gocui.ColorWhite | gocui.AttrBold
		} else {
			v.FrameColor = gocui.ColorBlue
			v.FgColor = gocui.ColorCyan
		}

		// Refresh content if not editable (i.e., not currently being edited)
		if !v.Editable {
			app.updateFilterViewContent(g)
		}
	}

	// --- Content View ---
	// Adjusted Y coordinate: contentViewY1
	if v, err := g.SetView(ContentViewName, filesWidth+1, 0, maxX-1, contentViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content - PgUp/PgDn Scroll "
		v.Editable = false
		v.Wrap = true
		v.Autoscroll = false
		v.FrameColor = gocui.ColorBlue
		// Initial population
		app.refreshContentView(g)
	} else {
		// Always refresh content
		app.refreshContentView(g)
	}

	// --- Status Bar ---
	// Adjusted Y coordinates: statusBarY0 and statusBarY1
	if v, err := g.SetView(StatusViewName, 0, statusBarY0, maxX-1, statusBarY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		v.FgColor = gocui.ColorWhite
		v.BgColor = gocui.ColorDefault
		// Initial population
		app.resetStatus(g)
	} // No else needed, status is updated via updateStatus/resetStatus

	// --- Preview View (Modal/Floating) ---
	app.mutex.Lock()
	isPreviewOpen := app.isPreviewOpen
	previewFile := app.previewFile
	previewContent := app.previewContent
	previewOriginY := app.previewOriginY
	app.mutex.Unlock()

	if isPreviewOpen {
		// Calculate dimensions (e.g., 80% width, 80% height, centered)
		previewWidth := maxX * 8 / 10
		previewHeight := maxY * 8 / 10
		x0 := (maxX - previewWidth) / 2
		y0 := (maxY - previewHeight) / 2
		x1 := x0 + previewWidth - 1
		y1 := y0 + previewHeight - 1

		if pv, err := g.SetView(PreviewViewName, x0, y0, x1, y1, gocui.TOP); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			// Initialize the preview view
			pv.Title = fmt.Sprintf(" Preview: %s  ", previewFile) // Updated title
			pv.Editable = false
			pv.Wrap = true // Enable wrapping for long lines
			pv.Autoscroll = false
			pv.Frame = true
			pv.FrameColor = gocui.ColorCyan
			pv.FgColor = gocui.ColorWhite

			// Set initial content and origin
			pv.Clear()
			fmt.Fprint(pv, previewContent)
			_ = pv.SetOrigin(0, previewOriginY) // Use stored origin

			// Set focus to the new preview window
			if _, err := g.SetCurrentView(PreviewViewName); err != nil {
				return err
			}
			currentViewName = PreviewViewName // Update currentViewName
		} else {
			// View already exists, ensure content, title and origin are up-to-date
			pv.Title = fmt.Sprintf(" Preview: %s ", previewFile)
			pv.Clear()
			fmt.Fprint(pv, previewContent)
			_ = pv.SetOrigin(0, previewOriginY) // Apply potentially updated origin

			// Ensure frame color reflects focus (important if focus changes rapidly)
			if currentViewName == PreviewViewName {
				pv.FrameColor = gocui.ColorGreen // Use green when focused
			} else {
				pv.FrameColor = gocui.ColorCyan // Use cyan when not focused (though it should be focused if open)
			}
		}
	} else {
		// Ensure preview view is deleted if it exists and shouldn't be open
		if _, err := g.View(PreviewViewName); err == nil {
			_ = g.DeleteView(PreviewViewName)
			// If the deleted view was the current view, reset focus (e.g., to FilesView)
			if currentViewName == PreviewViewName {
				if _, err := g.SetCurrentView(FilesViewName); err != nil {
					// Log or handle error if setting focus fails
				}
				// No need to update currentViewName here, it will be updated on the next Layout cycle
			}
		}
	}

	// --- Help View (Modal) ---
	app.mutex.Lock()
	showHelp := app.showHelp
	app.mutex.Unlock()

	if showHelp {
		helpWidth := maxX / 2
		helpHeight := maxY / 2
		x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
		x1, y1 := x0+helpWidth, y0+helpHeight

		if v, err := g.SetView(HelpViewName, x0, y0, x1, y1, gocui.TOP); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Help (? or Esc to Close) "
			v.Wrap = true
			v.FrameColor = gocui.ColorMagenta
			fmt.Fprintln(v, "grepforllm - Copy File Contents for LLM Input")
			fmt.Fprintln(v, "----------------------------------------")
			fmt.Fprintln(v, "Keyboard Shortcuts:")
			fmt.Fprintln(v, "  ↑ / k         : Move cursor up in file list")
			fmt.Fprintln(v, "  ↓ / j         : Move cursor down")
			fmt.Fprintln(v, "  Enter         : Preview selected file")
			fmt.Fprintln(v, "  Space         : Toggle select file")
			fmt.Fprintln(v, "  a             : Select / Deselect all visible files")
			fmt.Fprintln(v, "  c / y         : Copy contents of selected files")
			fmt.Fprintln(v, "  Tab           : Switch focus between Files and Filter")
			fmt.Fprintln(v, "  PgUp / Ctrl+B : Scroll main content view UP")
			fmt.Fprintln(v, "  PgDn          : Scroll main content view DOWN")
			fmt.Fprintln(v, "  ?             : Toggle this help message")
			fmt.Fprintln(v, "  Ctrl+C / q    : Quit the application (closes Preview if open)")
			fmt.Fprintln(v, "\nIn Filter View (when focused)  :")
			fmt.Fprintln(v, "  Enter         : Apply filter and return to Files")
			fmt.Fprintln(v, "  Esc           : Cancel filter and return to Files")
			fmt.Fprintln(v, "  Ctrl+F        : Toggle filter mode (Include/Exclude)")
			fmt.Fprintln(v, "  (Use comma-separated patterns, e.g., *.go,cmd/,Makefile)")
			fmt.Fprintln(v, "\nIn Preview View (when focused):")
			fmt.Fprintln(v, "  ↑ / k         : Scroll Up")
			fmt.Fprintln(v, "  ↓ / j         : Scroll Down")
			fmt.Fprintln(v, "  Esc / q       : Close Preview")

			// Set focus to help view if it's not already focused
			if currentViewName != HelpViewName {
				if _, err := g.SetCurrentView(HelpViewName); err != nil {
					return err
				}
				// currentViewName = HelpViewName // Update currentViewName
			}
		} else {
			// Ensure frame color reflects focus
			if currentViewName == HelpViewName {
				v.FrameColor = gocui.ColorGreen
			} else {
				v.FrameColor = gocui.ColorMagenta
			}
		}
	} else {
		// Ensure help view is deleted if it exists and shouldn't be open
		if _, err := g.View(HelpViewName); err == nil {
			_ = g.DeleteView(HelpViewName)
			// If the deleted view was the current view, reset focus (unless preview is open)
			if currentViewName == HelpViewName && !isPreviewOpen {
				if _, err := g.SetCurrentView(FilesViewName); err != nil {
					// Log or handle error if setting focus fails
				}
				// No need to update currentViewName here, it will be updated on the next Layout cycle
			}
		}
	}

	return nil
}

func (app *App) refreshFilesView(g *gocui.Gui) {
	v, err := g.View(FilesViewName)
	if err != nil {
		return // View doesn't exist yet
	}
	v.Clear()

	app.mutex.Lock()
	modeStr := "[Exclude]"
	if app.filterMode == IncludeMode {
		modeStr = "[Include]"
	}
	selectedCount := len(app.selectedFiles)
	totalCount := len(app.fileList)

	// Ensure currentLine is valid
	if totalCount == 0 {
		app.currentLine = 0
	} else if app.currentLine >= totalCount {
		app.currentLine = totalCount - 1
	} else if app.currentLine < 0 {
		app.currentLine = 0
	}

	// Make copies under lock
	currentFileList := make([]string, totalCount)
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	currentLine := app.currentLine
	app.mutex.Unlock()

	// Update title
	title := fmt.Sprintf(" Files (%d/%d) %s [?] Help ", selectedCount, totalCount, modeStr)
	v.Title = title

	// Populate view content
	for i, file := range currentFileList {
		isSelected := currentSelectedFiles[file]
		isCurrent := (i == currentLine)
		prefix := "[ ]"
		if isSelected {
			prefix = "[*]"
		}
		line := fmt.Sprintf("%s %s", prefix, file)

		if isCurrent {
			// Let gocui handle highlighting the current line based on SelFgColor/SelBgColor
			fmt.Fprintln(v, line)
		} else if isSelected {
			// Use a different color (e.g., green) for selected but not current lines
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line) // Green text
		} else {
			fmt.Fprintln(v, line)
		}
	}

	// Set cursor position (relative to view buffer, not origin)
	if totalCount > 0 {
		// gocui expects cursor position relative to the buffer content
		err = v.SetCursor(0, currentLine)
		if err != nil {
			// Log or handle error setting cursor
		}
	} else {
		// No content, place cursor at 0,0
		err = v.SetCursor(0, 0)
		if err != nil {
			// Log or handle error
		}
	}

	// Adjust origin (scrolling) based on cursor position
	app.adjustFilesViewScroll(g, v)
}

func (app *App) refreshContentView(g *gocui.Gui) {
	v, err := g.View(ContentViewName)
	if err != nil {
		return // View doesn't exist yet
	}
	// Preserve scroll position before clearing
	currentOx, currentOy := v.Origin()
	v.Clear()

	var contentBuilder strings.Builder
	count := 0
	totalCharCount := 0
	totalTokenCount := 0

	app.mutex.Lock()
	// Make copies under lock
	selectedCount := len(app.selectedFiles)
	currentFileList := make([]string, len(app.fileList))
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	rootDir := app.rootDir
	tokenizer := app.tokenizer // Copy tokenizer reference under lock
	app.mutex.Unlock()

	// Build content string outside lock
	sortedSelectedFiles := []string{}
	for _, file := range currentFileList { // Iterate in the display order
		if currentSelectedFiles[file] {
			sortedSelectedFiles = append(sortedSelectedFiles, file)
		}
	}

	for _, relPath := range sortedSelectedFiles {
		fullPath := filepath.Join(rootDir, relPath)
		fileContent, err := os.ReadFile(fullPath)
		separator := fmt.Sprintf("--- BEGIN FILE: %s ---\n", relPath)
		endSeparator := fmt.Sprintf("--- END FILE: %s ---\n\n", relPath)

		contentBuilder.WriteString(separator)
		if err != nil {
			contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n", err))
		} else {
			// Ensure newline before content if needed (shouldn't be necessary with separator)
			// contentBuilder.WriteString("\n") // Add newline before content?
			contentStr := string(fileContent)
			totalCharCount += utf8.RuneCountInString(contentStr) // Use RuneCountInString for UTF-8
			if tokenizer != nil {
				// Consider running tokenization in a goroutine if it becomes slow
				tokens := tokenizer.Encode(contentStr, nil, nil)
				totalTokenCount += len(tokens)
			}
			contentBuilder.WriteString(contentStr)
			if !strings.HasSuffix(contentStr, "\n") {
				contentBuilder.WriteString("\n") // Ensure trailing newline
			}
		}
		contentBuilder.WriteString(endSeparator)
		count++
	}

	// Update title
	v.Title = fmt.Sprintf(" Content (%d files | Chars: %d | Tokens: %d) - PgUp/PgDn Scroll ", count, totalCharCount, totalTokenCount)

	// Write content to view
	if count == 0 {
		fmt.Fprintln(v, "\nSelect files using [Space] to view content.\n")
		fmt.Fprintln(v, "[a]       : Select / Deselect all visible files")
		fmt.Fprintln(v, "[c] / [y] : Copy content of selected files")
		fmt.Fprintln(v, "[Tab]     : Focus views\n")
		fmt.Fprintln(v, "\nExample patterns: *.txt (text files), config/ (config folder), README* (files starting with README).\n")
		fmt.Fprintln(v, "\nUse [?] for full help.")
	} else {
		// Use Fprint to avoid extra newline added by Fprintln
		fmt.Fprint(v, contentBuilder.String())
	}

	// Restore scroll position
	// Check if the old origin is still valid given the new content height
	_, contentHeight := v.Size() // This gets view size, not buffer size
	bufferLines := strings.Count(v.ViewBuffer(), "\n")
	maxOy := max(0, bufferLines-contentHeight) // Calculate max possible origin Y
	if currentOy > maxOy {
		currentOy = maxOy // Adjust if previous origin is now out of bounds
	}

	err = v.SetOrigin(currentOx, currentOy)
	if err != nil {
		// If setting origin fails, reset to 0,0
		_ = v.SetOrigin(0, 0)
	}
}

func (app *App) updateStatus(g *gocui.Gui, message string) {
	// Run in g.Update to ensure thread safety with gocui
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, message) // Use Fprint to avoid extra newline
			v.Rewind()             // Ensure content starts from the beginning if it wraps
		} else if err != gocui.ErrUnknownView {
			// Log or handle error if view is unexpectedly missing
			return err
		}
		return nil
	})
}

func (app *App) resetStatus(g *gocui.Gui) {
	// Run in g.Update to ensure thread safety with gocui
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			// Updated status bar text to include Enter for preview
			fmt.Fprint(v, "↑↓: Nav | Space: Sel | Enter: Preview | a: Sel All | c: Copy | Tab: Filter | ?: Help | q: Quit/Close")
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
			// Log or handle error if view is unexpectedly missing
			return err
		}
		return nil
	})
}

// refreshViews is a convenience function to update multiple views at once.
func (app *App) refreshViews(g *gocui.Gui) {
	// Run in g.Update to ensure thread safety with gocui
	g.Update(func(g *gocui.Gui) error {
		// Refreshing views might change layout/content, potentially affecting focus or origin.
		// It's generally safe to call refresh functions within g.Update.
		app.refreshFilesView(g)
		app.refreshContentView(g)
		// No need to explicitly call Layout here, gocui handles redrawing after Update.
		return nil
	})
}
