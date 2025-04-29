package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"
)

// Layout decides which UI layout to render based on app state.
func (app *App) Layout(g *gocui.Gui) error {
	app.mutex.Lock()
	showCache := app.showCacheView
	app.mutex.Unlock()

	if showCache {
		return app.layoutCacheView(g)
	} else {
		return app.GrepApplicationView(g)
	}
}

// GrepApplicationView renders the standard file browser UI.
func (app *App) GrepApplicationView(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	_ = g.DeleteView(CacheViewName)

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

	// --- Path View ---
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

		app.refreshFilesView(g)

		// Set initial focus if no view is focused or if returning from cache/help/preview
		if g.CurrentView() == nil || currentViewName == CacheViewName || currentViewName == HelpViewName || currentViewName == PreviewViewName {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
			currentViewName = FilesViewName
		}
	} else {
		app.refreshFilesView(g)
		if currentViewName == FilesViewName {
			v.FrameColor = gocui.ColorGreen
		} else {
			v.FrameColor = gocui.ColorBlue
		}
	}

	// --- Filter View ---
	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Wrap = false
		v.Editor = gocui.DefaultEditor
		v.FgColor = gocui.ColorCyan
		app.updateFilterViewContent(g)
	} else {
		app.mutex.Lock()
		modeStr := "Exclude"
		if app.filterMode == IncludeMode {
			modeStr = "Include"
		}
		app.mutex.Unlock()
		v.Title = fmt.Sprintf(" Filter: %s (Enter: Apply, Ctrl+F: Mode) ", modeStr)

		if currentViewName == FilterViewName {
			v.FrameColor = gocui.ColorGreen
			v.FgColor = gocui.ColorWhite | gocui.AttrBold
		} else {
			v.FrameColor = gocui.ColorBlue
			v.FgColor = gocui.ColorCyan
		}
		if !v.Editable {
			app.updateFilterViewContent(g)
		}
	}

	// --- Content View ---
	if v, err := g.SetView(ContentViewName, filesWidth+1, 0, maxX-1, contentViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content - PgUp/PgDn Scroll "
		v.Editable = false
		v.Wrap = true
		v.Autoscroll = false
		v.FrameColor = gocui.ColorBlue
		app.refreshContentView(g)
	} else {
		app.refreshContentView(g)
		if currentViewName == ContentViewName {
			v.FrameColor = gocui.ColorGreen
		} else {
			v.FrameColor = gocui.ColorBlue
		}
	}

	// --- Status Bar ---
	if v, err := g.SetView(StatusViewName, 0, statusBarY0, maxX-1, statusBarY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		v.FgColor = gocui.ColorWhite
		v.BgColor = gocui.ColorDefault
		app.resetStatus(g)
	} else {
		app.mutex.Lock()
		awaitingConfirm := app.awaitingCacheClearConfirmation
		app.mutex.Unlock()
		if !awaitingConfirm {
			app.resetStatus(g)
		}
	}

	// --- Preview View (Modal/Floating) ---
	app.mutex.Lock()
	isPreviewOpen := app.isPreviewOpen
	previewFile := app.previewFile
	previewContent := app.previewContent
	previewOriginY := app.previewOriginY
	app.mutex.Unlock()

	if isPreviewOpen {
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
			pv.Title = fmt.Sprintf(" Preview: %s ", previewFile)
			pv.Editable = false
			pv.Wrap = true
			pv.Autoscroll = false
			pv.Frame = true
			pv.FgColor = gocui.ColorWhite

			pv.Clear()
			fmt.Fprint(pv, previewContent)
			_ = pv.SetOrigin(0, previewOriginY)

			if _, err := g.SetCurrentView(PreviewViewName); err != nil {
				return err
			}
			currentViewName = PreviewViewName
			pv.FrameColor = gocui.ColorGreen
		} else {
			// Update existing preview view
			pv.Title = fmt.Sprintf(" Preview: %s ", previewFile)
			pv.Clear()
			fmt.Fprint(pv, previewContent)
			_ = pv.SetOrigin(0, previewOriginY)

			if currentViewName == PreviewViewName {
				pv.FrameColor = gocui.ColorGreen
			} else {
				pv.FrameColor = gocui.ColorCyan
			}
		}
	} else {
		// Ensure preview view is deleted if it exists and shouldn't be open
		if _, err := g.View(PreviewViewName); err == nil {
			_ = g.DeleteView(PreviewViewName)
			if currentViewName == PreviewViewName {
				// Focus was on preview, return it to FilesView
				if _, err := g.SetCurrentView(FilesViewName); err != nil {
					// Log or handle error
				}
				// currentViewName will update on next Layout cycle
			}
		}
	}

	// --- Help View (Modal) ---
	app.mutex.Lock()
	showHelp := app.showHelp
	app.mutex.Unlock()

	if showHelp {
		helpWidth := maxX * 2 / 3  // Adjusted width
		helpHeight := maxY * 2 / 3 // Adjusted height
		x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
		x1, y1 := x0+helpWidth-1, y0+helpHeight-1 // Adjusted for 0-based index

		if v, err := g.SetView(HelpViewName, x0, y0, x1, y1, gocui.TOP); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Help (? or Esc to Close) "
			v.Wrap = true
			v.Editable = false // Help view should not be editable
			v.Frame = true
			// Populate help text (consider updating with Ctrl+C, Ctrl+D, Ctrl+Q)
			fmt.Fprintln(v, "grepforllm - Copy File Contents for LLM Input")
			fmt.Fprintln(v, "----------------------------------------")
			fmt.Fprintln(v, "Keyboard Shortcuts (Normal Mode):")
			fmt.Fprintln(v, "  ↑ / k         : Move cursor up")
			fmt.Fprintln(v, "  ↓ / j         : Move cursor down")
			fmt.Fprintln(v, "  Enter         : Preview selected file")
			fmt.Fprintln(v, "  Space         : Toggle select file")
			fmt.Fprintln(v, "  a             : Select / Deselect all visible files")
			fmt.Fprintln(v, "  c / y         : Copy contents of selected files")
			fmt.Fprintln(v, "  Tab           : Switch focus Files <-> Filter")
			fmt.Fprintln(v, "  PgUp / Ctrl+B : Scroll main content view UP")
			fmt.Fprintln(v, "  PgDn          : Scroll main content view DOWN")
			fmt.Fprintln(v, "  ?             : Toggle this help message")
			fmt.Fprintln(v, "  Ctrl+C        : Show Cache View")
			fmt.Fprintln(v, "  q             : Quit / Close Preview / Close Cache")
			fmt.Fprintln(v, "  Ctrl+Q        : Force Quit Application")
			fmt.Fprintln(v, "\nIn Filter View:")
			fmt.Fprintln(v, "  Enter         : Apply filter")
			fmt.Fprintln(v, "  Esc           : Cancel filter")
			fmt.Fprintln(v, "  Ctrl+F        : Toggle filter mode (Include/Exclude)")
			fmt.Fprintln(v, "  (Patterns: *.go, cmd/, Makefile)")
			fmt.Fprintln(v, "\nIn Preview View:")
			fmt.Fprintln(v, "  ↑ / k / ↓ / j : Scroll Line")
			fmt.Fprintln(v, "  PgUp / PgDn   : Scroll Page")
			fmt.Fprintln(v, "  Esc / q       : Close Preview")
			fmt.Fprintln(v, "\nIn Cache View:")
			fmt.Fprintln(v, "  ↑ / k / ↓ / j : Scroll Line")
			fmt.Fprintln(v, "  PgUp / PgDn   : Scroll Page")
			fmt.Fprintln(v, "  Ctrl+D        : Prompt to clear cache")
			fmt.Fprintln(v, "  y / n         : Confirm / Cancel cache clear")
			fmt.Fprintln(v, "  Esc / q       : Close Cache View")

			if _, err := g.SetCurrentView(HelpViewName); err != nil {
				return err
			}
			currentViewName = HelpViewName
			v.FrameColor = gocui.ColorGreen
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
			if currentViewName == HelpViewName && !isPreviewOpen {
				if _, err := g.SetCurrentView(FilesViewName); err != nil {
					// Log or handle error
				}
				// currentViewName will update on next Layout cycle
			}
		}
	}

	return nil
}

// layoutCacheView renders the cache content view.
func (app *App) layoutCacheView(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// --- Delete normal views ---
	// Ensure a clean slate when showing the cache view
	viewsToDelete := []string{
		FilesViewName, ContentViewName, FilterViewName, PathViewName,
		PreviewViewName, HelpViewName, // Also delete preview/help if they were open
	}
	for _, viewName := range viewsToDelete {
		_ = g.DeleteView(viewName) // Ignore ErrUnknownView
	}

	// --- Cache View ---
	cacheViewY1 := maxY - 2 // Leave space for status bar

	app.mutex.Lock()
	cacheContent := app.cacheViewContent
	cacheOriginY := app.cacheViewOriginY
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if cv, err := g.SetView(CacheViewName, 0, 0, maxX-1, cacheViewY1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// Initialize cache view
		cv.Title = " Cache Contents (cache.json) "
		cv.Editable = false
		cv.Wrap = true // Enable wrapping
		cv.Autoscroll = false
		cv.Frame = true
		cv.FgColor = gocui.ColorWhite

		cv.Clear()
		fmt.Fprint(cv, cacheContent)
		_ = cv.SetOrigin(0, cacheOriginY)

		// Set focus to the cache view
		if _, err := g.SetCurrentView(CacheViewName); err != nil {
			return err
		}
		cv.FrameColor = gocui.ColorGreen // Focused color
	} else {
		// Update existing cache view (e.g., after clearing cache)
		cv.Clear()
		fmt.Fprint(cv, cacheContent)
		_ = cv.SetOrigin(0, cacheOriginY) // Apply potentially updated origin

		// Ensure frame color reflects focus
		if g.CurrentView() == cv {
			cv.FrameColor = gocui.ColorGreen
		} else {
			cv.FrameColor = gocui.ColorCyan // Should not happen if logic is correct
		}
	}

	// --- Status Bar ---
	statusBarY0 := maxY - 2
	statusBarY1 := maxY

	if sv, err := g.SetView(StatusViewName, 0, statusBarY0, maxX-1, statusBarY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		sv.Frame = false
		sv.Editable = false
		sv.Wrap = false
		sv.FgColor = gocui.ColorWhite
		sv.BgColor = gocui.ColorDefault
		if !awaitingConfirm {
			app.resetStatusForCacheView(g)
		}
	} else {
		if !awaitingConfirm {
			app.resetStatusForCacheView(g)
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
		err = v.SetCursor(0, currentLine) // gocui expects cursor relative to buffer
		if err != nil {
			// Log or handle error setting cursor
		}
	} else {
		err = v.SetCursor(0, 0) // No content, place cursor at 0,0
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
			contentStr := string(fileContent)
			totalCharCount += utf8.RuneCountInString(contentStr) // Use RuneCountInString for UTF-8
			if tokenizer != nil {
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
		fmt.Fprint(v, contentBuilder.String())
	}

	// Restore scroll position
	_, viewHeight := v.Size() // This gets view size, not buffer size
	bufferLines := strings.Count(v.ViewBuffer(), "\n")
	maxOy := max(0, bufferLines-viewHeight) // Calculate max possible origin Y
	if currentOy > maxOy {
		currentOy = maxOy // Adjust if previous origin is now out of bounds
	}

	err = v.SetOrigin(currentOx, currentOy)
	if err != nil {
		_ = v.SetOrigin(0, 0) // Reset to 0,0 on error
	}
}

// --- Status Bar Functions ---

func (app *App) updateStatus(g *gocui.Gui, message string) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, message)
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
			return err
		}
		return nil
	})
}

// resetStatus sets the default status bar text for the normal file browser view.
func (app *App) resetStatus(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			// Updated status bar text for normal mode
			fmt.Fprint(v, "↑↓: Nav | Space: Sel | Enter: Preview | a: Sel All | c: Copy | Tab: Filter | ?: Help | Ctrl+C: Cache | q: Quit/Close")
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
			return err
		}
		return nil
	})
}

// resetStatusForCacheView sets the default status bar text for the cache view.
func (app *App) resetStatusForCacheView(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, "↑↓ PgUp/Dn: Scroll | Ctrl+D: Clear Cache | Esc/q: Close Cache View")
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
			return err
		}
		return nil
	})
}

// refreshViews is a convenience function to update multiple views at once (for normal layout).
func (app *App) refreshViews(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		app.refreshFilesView(g)
		app.refreshContentView(g)
		return nil
	})
}
