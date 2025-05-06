package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"
)

// Layout decides which UI layout to render based on app state.
func (app *App) Layout(g *gocui.Gui) error {
	app.mutex.Lock()
	showCache := app.showCacheView
	showHelp := app.showHelp // Need help state for main layout too
	isLoading := app.isLoading
	loadingError := app.loadingError
	app.mutex.Unlock()

	// --- Loading State Handling ---
	// Display loading indicator or error *before* attempting main layout
	if isLoading {
		return app.layoutLoadingView(g)
	}
	if loadingError != nil {
		return app.layoutErrorView(g, loadingError)
	}
	// --- End Loading State Handling ---

	if showCache {
		return app.layoutCacheView(g) // Cache view takes precedence
	} else if showHelp {
		// Render main layout first, then overlay help
		_ = app.GrepApplicationView(g)
		return app.layoutHelpView(g) // Help view overlays main view
	} else {
		// Ensure help view is deleted if it exists and shouldn't be open
		// This needs to happen *before* setting focus back in GrepApplicationView
		if _, err := g.View(HelpViewName); err == nil {
			_ = g.DeleteView(HelpViewName)
		}
		return app.GrepApplicationView(g) // Normal view
	}
}

// GrepApplicationView renders the standard file browser UI.
func (app *App) GrepApplicationView(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// Ensure modal views (except help, handled in Layout) are gone
	_ = g.DeleteView(CacheViewName)
	_ = g.DeleteView("loading") // Ensure loading view is gone
	_ = g.DeleteView("error")   // Ensure error view is gone

	filesWidth := maxX / 3
	pathHeight := 2
	filterHeight := 3
	statusBarHeight := 2 // Status bar takes 2 lines now

	// --- Y coordinates ---
	pathY0 := 0
	pathY1 := pathY0 + pathHeight // Occupies lines 0, 1

	filesY0 := pathY1 + 1                                // Starts at line 2
	filesViewY1 := maxY - statusBarHeight - filterHeight // Ends before filter and status

	filterY0 := filesViewY1 + 1             // Starts below files
	filterY1 := filterY0 + filterHeight - 1 // Ends before status

	contentViewY1 := maxY - statusBarHeight // Content ends before status bar

	statusBarY0 := maxY - statusBarHeight // Status starts
	statusBarY1 := maxY                   // Status ends at bottom

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
		pv.FrameColor = gocui.ColorBlue // Path view never focused
		pv.FgColor = gocui.ColorMagenta
		pv.Clear() // Clear before writing
		fmt.Fprint(pv, app.rootDir)
	} else {
		// Update content if needed (e.g., if rootDir could change - not currently possible)
		pv.Clear()
		fmt.Fprint(pv, app.rootDir)
	}

	// --- Files View ---
	if v, err := g.SetView(FilesViewName, 0, filesY0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Files " // Title updated in refreshFilesView
		v.Highlight = true
		v.SelBgColor = gocui.ColorDefault               // Background for selected line
		v.SelFgColor = gocui.ColorCyan | gocui.AttrBold // Foreground for selected line
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false // We handle scrolling manually

		// Set initial focus if needed (e.g., on startup or returning from cache)
		// Help view focus is handled in Layout()
		if g.CurrentView() == nil || currentViewName == CacheViewName {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
			currentViewName = FilesViewName // Update local tracker
		}
	}
	// Always refresh files view content and update frame color based on focus
	app.refreshFilesView(g) // Refreshes content and title
	fv, _ := g.View(FilesViewName)
	if fv != nil { // Check if view exists before setting color
		if currentViewName == FilesViewName {
			fv.FrameColor = gocui.ColorGreen // Focused
		} else {
			fv.FrameColor = gocui.ColorBlue // Not focused
		}
	}

	// --- Filter View ---
	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false // Initially not editable
		v.Wrap = false
		v.Editor = gocui.DefaultEditor // Use default editor for input
		v.FgColor = gocui.ColorCyan    // Default text color
		// Title set below based on mode
	}
	// Always update filter view content/title and frame color
	app.mutex.Lock()
	modeStr := "Exclude"
	if app.filterMode == IncludeMode {
		modeStr = "Include"
	}
	app.mutex.Unlock()
	filterV, _ := g.View(FilterViewName)
	if filterV != nil {
		filterV.Title = fmt.Sprintf(" Filter: %s (Ctrl+F: Mode) ", modeStr)
		if currentViewName == FilterViewName {
			filterV.FrameColor = gocui.ColorGreen               // Focused
			filterV.FgColor = gocui.ColorWhite | gocui.AttrBold // Focused text color
		} else {
			filterV.FrameColor = gocui.ColorBlue // Not focused
			filterV.FgColor = gocui.ColorCyan    // Default text color
		}
		// Update content/cursor state (handles editable vs non-editable)
		app.updateFilterViewContent(g)
	}

	// --- Content View ---
	contentX0 := filesWidth + 1
	contentY0 := 0 // Content view starts at the top now
	if v, err := g.SetView(ContentViewName, contentX0, contentY0, maxX-1, contentViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content " // Title updated in refreshContentView
		v.Editable = false
		v.Wrap = true        // Enable wrapping for content
		v.Autoscroll = false // We handle scrolling
		v.Frame = true
	}
	// Always refresh content view and update frame color
	app.refreshContentView(g) // Refreshes content and title
	cv, _ := g.View(ContentViewName)
	if cv != nil {
		if currentViewName == ContentViewName {
			cv.FrameColor = gocui.ColorGreen // Focused
		} else {
			cv.FrameColor = gocui.ColorBlue // Not focused
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
		v.BgColor = gocui.ColorDefault // Or maybe ColorBlue? Default is usually fine.
	}
	// Always reset status unless awaiting cache confirmation
	app.mutex.Lock()
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()
	if !awaitingConfirm {
		app.resetStatus(g) // Update status bar text (NOW INCLUDES COUNTS)
	}

	return nil
}

// layoutHelpView renders the help overlay. Assumes GrepApplicationView was called first.
func (app *App) layoutHelpView(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	helpWidth := maxX * 2 / 3
	helpHeight := maxY * 2 / 3
	x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
	x1, y1 := x0+helpWidth-1, y0+helpHeight-1

	if v, err := g.SetView(HelpViewName, x0, y0, x1, y1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Help (? or Esc or q to Close) "
		v.Wrap = true
		v.Editable = false
		v.Frame = true
		v.FgColor = gocui.ColorWhite // Text color for help

		// --- Updated Help Text ---
		v.Clear()
		fmt.Fprintln(v, "grepforllm - Select & Copy File Contents")
		fmt.Fprintln(v, "----------------------------------------")
		fmt.Fprintln(v, "General:")
		fmt.Fprintln(v, "  Tab           : Switch focus Files <-> Filter <-> Content")
		fmt.Fprintln(v, "  ?             : Toggle this help message")
		fmt.Fprintln(v, "  Ctrl+C        : Show Cache View")
		fmt.Fprintln(v, "  q             : Quit / Close Help / Close Cache")
		fmt.Fprintln(v, "  Ctrl+Q        : Force Quit Application")
		fmt.Fprintln(v, "\nFiles View (Left):")
		fmt.Fprintln(v, "  ↑ / k         : Move cursor up")
		fmt.Fprintln(v, "  ↓ / j         : Move cursor down")
		fmt.Fprintln(v, "  Enter         : Focus Content View for scrolling")
		fmt.Fprintln(v, "  Space         : Toggle select file under cursor")
		fmt.Fprintln(v, "  a             : Select / Deselect all visible files")
		fmt.Fprintln(v, "  c / y         : Copy contents of selected files to clipboard")
		fmt.Fprintln(v, "\nContent View (Right):")
		fmt.Fprintln(v, "  ↑ / k         : Scroll content UP one line (when focused)")
		fmt.Fprintln(v, "  ↓ / j         : Scroll content DOWN one line (when focused)")
		fmt.Fprintln(v, "  PgUp / Ctrl+B : Scroll content UP one page (works globally)")
		fmt.Fprintln(v, "  PgDn          : Scroll content DOWN one page (works globally)")
		// fmt.Fprintln(v, "  Esc           : Return focus to Files View (Optional - Not bound by default)")
		fmt.Fprintln(v, "\nFilter View (Bottom-Left):")
		fmt.Fprintln(v, "  (Type patterns: *.go, cmd/, file.txt)")
		fmt.Fprintln(v, "  Enter         : Apply filter & return focus to Files")
		fmt.Fprintln(v, "  Esc           : Cancel input & return focus to Files")
		fmt.Fprintln(v, "  Ctrl+F        : Toggle filter mode (Include/Exclude)")
		fmt.Fprintln(v, "\nCache View (Ctrl+C):")
		fmt.Fprintln(v, "  ↑ / k / ↓ / j : Scroll Line")
		fmt.Fprintln(v, "  PgUp / PgDn   : Scroll Page")
		fmt.Fprintln(v, "  Ctrl+D        : Prompt to clear cache")
		fmt.Fprintln(v, "  y / n         : Confirm / Cancel cache clear")
		fmt.Fprintln(v, "  Esc / q       : Close Cache View")
		// --- End Updated Help Text ---

		// Set focus to Help view when it's created
		if _, err := g.SetCurrentView(HelpViewName); err != nil {
			return err
		}
		v.FrameColor = gocui.ColorGreen // Focused color
	} else {
		// If view exists, ensure it's focused and has the right color
		if g.CurrentView() != v {
			if _, err := g.SetCurrentView(HelpViewName); err != nil {
				return err
			}
		}
		v.FrameColor = gocui.ColorGreen // Focused color
	}
	return nil
}

// layoutCacheView renders the cache content view.
func (app *App) layoutCacheView(g *gocui.Gui) error {
	// This function remains the same - shows cache content, status bar for cache
	maxX, maxY := g.Size()

	// --- Delete normal views ---
	viewsToDelete := []string{
		FilesViewName, ContentViewName, FilterViewName, PathViewName,
		HelpViewName, // Also delete help if it was open
		"loading",    // Also delete loading/error views
		"error",
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
		cv.Title = " Cache Contents (cache.json) "
		cv.Editable = false
		cv.Wrap = true
		cv.Autoscroll = false
		cv.Frame = true
		cv.FgColor = gocui.ColorWhite

		cv.Clear()
		fmt.Fprint(cv, cacheContent)
		_ = cv.SetOrigin(0, cacheOriginY)

		if _, err := g.SetCurrentView(CacheViewName); err != nil {
			return err
		}
		cv.FrameColor = gocui.ColorGreen
	} else {
		cv.Clear()
		fmt.Fprint(cv, cacheContent)
		_ = cv.SetOrigin(0, cacheOriginY)

		if g.CurrentView() == cv {
			cv.FrameColor = gocui.ColorGreen
		} else {
			// This case shouldn't happen if focus logic is correct
			if _, err := g.SetCurrentView(CacheViewName); err == nil {
				cv.FrameColor = gocui.ColorGreen
			} else {
				cv.FrameColor = gocui.ColorCyan
			}
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
		// Set initial status for cache view
		if !awaitingConfirm {
			app.resetStatusForCacheView(g)
		} // else: PromptClearCache will set the status
	} else {
		// Update status if not awaiting confirmation
		if !awaitingConfirm {
			app.resetStatusForCacheView(g)
		}
	}

	return nil
}

// refreshFilesView updates the content and appearance of the Files view.
func (app *App) refreshFilesView(g *gocui.Gui) {
	// This function remains the same - displays files, selection, handles copy highlight
	v, err := g.View(FilesViewName)
	if err != nil {
		return
	}
	v.Clear()

	app.mutex.Lock()
	modeStr := "[Exclude]"
	if app.filterMode == IncludeMode {
		modeStr = "[Include]"
	}
	selectedCount := len(app.selectedFiles)
	totalCount := len(app.fileList)

	if totalCount == 0 {
		app.currentLine = 0
	} else if app.currentLine >= totalCount {
		app.currentLine = totalCount - 1
	} else if app.currentLine < 0 {
		app.currentLine = 0
	}

	currentFileList := make([]string, totalCount)
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	currentLine := app.currentLine
	isCopyHighlightActive := app.isCopyHighlightActive
	app.mutex.Unlock()

	title := fmt.Sprintf(" Files (%d/%d Sel) %s [?] Help ", selectedCount, totalCount, modeStr)
	v.Title = title

	for i, file := range currentFileList {
		isSelected := currentSelectedFiles[file]
		isCurrent := (i == currentLine)
		prefix := "[ ]"
		if isSelected {
			prefix = "[*]"
		}
		line := fmt.Sprintf("%s %s", prefix, file)

		switch {
		case isCopyHighlightActive && isSelected:
			fmt.Fprintf(v, "\x1b[7m%s\x1b[0m\n", line) // Inverse video for copy highlight
		case isCurrent:
			// Let gocui handle highlighting the current line via SelFgColor/SelBgColor
			fmt.Fprintln(v, line)
		case isSelected:
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line) // Green text for selected (not current)
		default:
			fmt.Fprintln(v, line)
		}
	}

	if totalCount > 0 {
		_ = v.SetCursor(0, currentLine)
	} else {
		_ = v.SetCursor(0, 0)
	}

	app.adjustFilesViewScroll(g, v) // Ensure cursor is visible

	// Refresh status bar whenever files view is refreshed, as selection count might change
	app.resetStatus(g)
}

// refreshContentView updates the content view with the file under the cursor.
func (app *App) refreshContentView(g *gocui.Gui) {
	// This function remains the same - shows content of file at app.currentLine
	v, err := g.View(ContentViewName)
	if err != nil {
		return
	}

	app.mutex.Lock()
	currentLine := app.currentLine
	fileListLen := len(app.fileList)
	var fileToPreviewRelPath string
	if fileListLen > 0 && currentLine >= 0 && currentLine < fileListLen {
		fileToPreviewRelPath = app.fileList[currentLine]
	}
	rootDir := app.rootDir
	previousPreviewedFile := app.currentlyPreviewedFile
	currentContentOriginY := app.contentViewOriginY
	app.mutex.Unlock()

	resetScroll := (fileToPreviewRelPath != previousPreviewedFile)
	newOriginY := currentContentOriginY
	if resetScroll {
		newOriginY = 0
	}

	v.Clear()

	if fileToPreviewRelPath == "" {
		v.Title = " Content - PgUp/PgDn Scroll "
		fmt.Fprintln(v, "\nNo file selected or list is empty.")
		fmt.Fprintln(v, "\nUse ↑ / ↓ to navigate files.")
		fmt.Fprintln(v, "[Enter]   : Focus this view for scrolling (j/k)")
		fmt.Fprintln(v, "[Space]   : Toggle select file")
		fmt.Fprintln(v, "[c] / [y] : Copy selected files")
		fmt.Fprintln(v, "[Tab]     : Switch focus")
		fmt.Fprintln(v, "[?]       : Help")
		_ = v.SetOrigin(0, 0)
		app.mutex.Lock()
		app.currentlyPreviewedFile = ""
		app.contentViewOriginY = 0
		app.mutex.Unlock()
		return
	}

	fullPath := filepath.Join(rootDir, fileToPreviewRelPath)
	fileContentBytes, readErr := os.ReadFile(fullPath)

	v.Title = fmt.Sprintf(" Content: %s - PgUp/PgDn Scroll ", fileToPreviewRelPath)

	if readErr != nil {
		fmt.Fprintf(v, "\n!!! ERROR READING FILE: %v !!!\n", readErr)
	} else if len(fileContentBytes) == 0 {
		fmt.Fprintln(v, "(Empty File)")
	} else if !isLikelyText(fileContentBytes) {
		fmt.Fprintf(v, "(Binary File: %s)", fileToPreviewRelPath)
	} else {
		fmt.Fprint(v, string(fileContentBytes))
	}

	app.mutex.Lock()
	app.currentlyPreviewedFile = fileToPreviewRelPath
	// Don't update contentViewOriginY here, it's updated by scroll handlers
	// If we reset scroll, newOriginY is 0, otherwise it's the old value.
	// The actual application of the origin happens below.
	app.mutex.Unlock()

	// Apply the calculated origin (either 0 for new file, or previous origin)
	// Need to calculate maxOy based on the *new* buffer content
	_, viewHeight := v.Size()
	bufferLines := strings.Count(v.ViewBuffer(), "\n") + 1
	maxOy := max(0, bufferLines-viewHeight)
	if newOriginY > maxOy {
		newOriginY = maxOy
	}

	err = v.SetOrigin(0, newOriginY) // Set horizontal origin to 0
	if err != nil {
		_ = v.SetOrigin(0, 0)
		// If origin setting failed, reset the stored state too
		if resetScroll { // Only reset state if we intended to reset scroll
			app.mutex.Lock()
			app.contentViewOriginY = 0
			app.mutex.Unlock()
		}
	} else {
		// If origin setting succeeded *and* we reset scroll, update state
		if resetScroll {
			app.mutex.Lock()
			app.contentViewOriginY = 0
			app.mutex.Unlock()
		}
		// If we didn't reset scroll, app.contentViewOriginY already holds the correct value
	}
}

// --- Status Bar Functions ---

func (app *App) updateStatus(g *gocui.Gui, message string) {
	// This function remains the same
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, message)
			v.Rewind() // Ensure message starts from the beginning if it's long
		} else if err != gocui.ErrUnknownView {
			return err
		}
		return nil
	})
}

// resetStatus sets the default status bar text for the normal file browser view.
// NOW INCLUDES CHARACTER AND TOKEN COUNTS FOR SELECTED FILES.
func (app *App) resetStatus(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err != nil && err != gocui.ErrUnknownView {
			return err // Return actual error if not ErrUnknownView
		}
		if err == gocui.ErrUnknownView {
			return nil // View doesn't exist yet, nothing to do
		}

		// --- Calculate Character and Token Counts ---
		app.mutex.Lock()
		// Copy needed state under lock
		selectedFilesCopy := make(map[string]bool, len(app.selectedFiles))
		for k, v := range app.selectedFiles {
			selectedFilesCopy[k] = v
		}
		rootDirCopy := app.rootDir
		tokenizer := app.tokenizer // Assuming tokenizer is thread-safe or immutable after init
		app.mutex.Unlock()

		totalChars := 0
		totalTokens := 0
		readErrors := 0

		if tokenizer == nil {
			// Handle case where tokenizer might not be initialized (shouldn't happen)
			fmt.Fprintf(os.Stderr, "Warning: Tokenizer not initialized in resetStatus\n")
		} else {
			for relPath := range selectedFilesCopy {
				fullPath := filepath.Join(rootDirCopy, relPath)
				contentBytes, readErr := os.ReadFile(fullPath)
				if readErr != nil {
					// Log error or just count them? Let's count for now.
					// log.Printf("Warning: Failed to read file %s for status count: %v", fullPath, readErr)
					readErrors++
					continue // Skip this file
				}

				// Count characters (bytes)
				totalChars += len(contentBytes)

				// Count tokens
				// Use Encode with suppress_special_tokens=True, allowed_special="all" equivalent if needed
				// For basic counting, default Encode is usually fine.
				tokens := tokenizer.Encode(string(contentBytes), nil, nil)
				if err != nil {
					// Log tokenizer error?
					// log.Printf("Warning: Failed to tokenize file %s: %v", fullPath, err)
					readErrors++ // Count as error if tokenization fails
					continue
				}
				totalTokens += len(tokens)
			}
		}
		// --- End Calculation ---

		v.Clear()
		// Format the status string with counts and keybindings
		statusFormat := "Chars: %d | Tokens: %d%s || ?: Help | q: Quit"
		errorStr := ""
		if readErrors > 0 {
			errorStr = fmt.Sprintf(" (%d read err)", readErrors)
		}
		statusText := fmt.Sprintf(statusFormat, totalChars, totalTokens, errorStr)

		fmt.Fprint(v, statusText)
		v.Rewind()

		return nil
	})
}

// resetStatusForCacheView sets the default status bar text for the cache view.
func (app *App) resetStatusForCacheView(g *gocui.Gui) {
	// This function remains the same
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

// refreshViews is a convenience function
func (app *App) refreshViews(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		app.refreshFilesView(g)
		// Content view refreshes on cursor move now
		// Status bar is refreshed by refreshFilesView
		return nil
	})
}

// --- Loading and Error Layouts ---

func (app *App) layoutLoadingView(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	msg := "Scanning files..."
	x0, y0 := (maxX-len(msg))/2, maxY/2
	x1, y1 := x0+len(msg), y0+2

	if v, err := g.SetView("loading", x0, y0, x1, y1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.FgColor = gocui.ColorCyan
		fmt.Fprint(v, msg)
	}
	return nil
}

func (app *App) layoutErrorView(g *gocui.Gui, loadErr error) error {
	maxX, maxY := g.Size()
	msgLines := []string{
		"Error during file scan:",
		loadErr.Error(),
		"",
		"Press Ctrl+Q to quit.",
	}
	maxWidth := 0
	for _, line := range msgLines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	width := maxWidth + 4
	height := len(msgLines) + 2
	x0, y0 := (maxX-width)/2, (maxY-height)/2
	x1, y1 := x0+width, y0+height

	if v, err := g.SetView("error", x0, y0, x1, y1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Error "
		v.Frame = true
		v.Editable = false
		v.Wrap = true
		v.FgColor = gocui.ColorRed

		v.Clear()
		for _, line := range msgLines {
			fmt.Fprintln(v, line)
		}
		// Set focus to error view so Ctrl+Q works easily
		if _, err := g.SetCurrentView("error"); err != nil {
			return err
		}
	}
	return nil
}
