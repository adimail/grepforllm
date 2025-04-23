package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"
)

const (
	filesViewName   = "files"
	contentViewName = "content"
	helpViewName    = "help"
	filterViewName  = "filter"
	defaultExcludes = ".git/,__pycache__/,node_modules/"
)

type FilterMode int

const (
	ExcludeMode FilterMode = iota
	IncludeMode
)

type App struct {
	g             *gocui.Gui
	rootDir       string
	fileList      []string // List of relative file paths
	allFiles      []string // List of all files before filtering
	selectedFiles map[string]bool
	currentLine   int
	showHelp      bool
	showFilter    bool
	filterMode    FilterMode
	excludes      string     // Files/patterns to exclude
	includes      string     // Files/patterns to include
	mutex         sync.Mutex // To protect concurrent access if needed later
}

func main() {
	// --- Argument Parsing ---
	rootDir := flag.String("dir", ".", "Root directory to scan")
	flag.Parse()

	absRootDir, err := filepath.Abs(*rootDir)
	if err != nil {
		log.Fatalf("Error getting absolute path for %s: %v", *rootDir, err)
	}

	// Check if directory exists
	info, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Error: Directory does not exist: %s", absRootDir)
		}
		log.Fatalf("Error accessing directory %s: %v", absRootDir, err)
	}
	if !info.IsDir() {
		log.Fatalf("Error: Path is not a directory: %s", absRootDir)
	}

	// --- Initialize App State ---
	app := &App{
		rootDir:       absRootDir,
		selectedFiles: make(map[string]bool),
		fileList:      []string{},
		allFiles:      []string{},
		currentLine:   0,
		showHelp:      false,
		showFilter:    false,
		filterMode:    ExcludeMode,
		excludes:      defaultExcludes,
		includes:      "",
	}

	// --- List Files ---
	err = app.listFiles()
	if err != nil {
		log.Fatalf("Error listing files in %s: %v", app.rootDir, err)
	}
	if len(app.fileList) == 0 {
		log.Printf("No files found in %s", app.rootDir)
		// Decide if you want to exit or show an empty UI
		// For now, let's proceed to show the empty UI
	}

	// --- Initialize gocui ---
	g, err := gocui.NewGui(gocui.OutputNormal, true) // OutputNormal, support mouse (optional)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	app.g = g
	g.Highlight = true

	// Remove all colors as per requirement
	g.SelFgColor = gocui.ColorDefault
	g.SelBgColor = gocui.ColorDefault
	g.Cursor = true // Show cursor in the files view

	g.SetManagerFunc(app.layout)

	if err := app.keybindings(g); err != nil {
		log.Panicln(err)
	}

	// --- Main Loop ---
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

// listFiles populates the app.fileList with relative paths
func (app *App) listFiles() error {
	var files []string
	err := filepath.WalkDir(app.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log permission errors but continue walking
			if os.IsPermission(err) {
				log.Printf("Warning: Permission denied accessing %s", path)
				return filepath.SkipDir // Skip this directory if we can't read it
			}
			return err // Propagate other errors
		}

		// Skip the root directory itself
		if path == app.rootDir {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			// Always skip certain directories
			name := d.Name()
			if strings.HasPrefix(name, ".git") ||
				strings.HasPrefix(name, "__pycache__") ||
				strings.HasPrefix(name, "node_modules") {
				return filepath.SkipDir
			}

			// Check for user-defined directory excludes
			if app.shouldSkipDirectory(path) {
				return filepath.SkipDir
			}

			return nil
		}

		relPath, err := filepath.Rel(app.rootDir, path)
		if err != nil {
			log.Printf("Warning: Could not get relative path for %s: %v", path, err)
			return nil // Skip if we can't get relative path
		}

		// Apply include/exclude filters
		if app.shouldIncludeFile(relPath) {
			files = append(files, relPath)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking directory %s: %w", app.rootDir, err)
	}

	sort.Strings(files) // Sort files alphabetically
	app.allFiles = files
	app.applyFilters() // Apply current filters
	return nil
}

// applyFilters applies the current include/exclude filters to all files
func (app *App) applyFilters() {
	app.fileList = []string{}

	// Reset selection if needed
	newSelectedFiles := make(map[string]bool)

	for _, file := range app.allFiles {
		if app.shouldIncludeFile(file) {
			app.fileList = append(app.fileList, file)
			// Preserve selection for files that remain in the list
			if app.selectedFiles[file] {
				newSelectedFiles[file] = true
			}
		}
	}

	// Update selected files
	app.selectedFiles = newSelectedFiles

	// Ensure current line is within bounds
	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1)
	}
}

// shouldSkipDirectory determines if a directory should be skipped based on filters
func (app *App) shouldSkipDirectory(path string) bool {
	relPath, err := filepath.Rel(app.rootDir, path)
	if err != nil {
		return false // If we can't determine relative path, don't skip
	}

	dirPath := relPath + "/"

	// In include mode, skip directories not explicitly included
	if app.filterMode == IncludeMode {
		for _, pattern := range strings.Split(app.includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}

			// Handle directory pattern
			if strings.HasSuffix(pattern, "/") && strings.HasPrefix(dirPath, pattern) {
				return false // Don't skip this directory
			}
		}
		return true // Skip by default in include mode
	}

	// In exclude mode, check if directory matches any exclude pattern
	for _, pattern := range strings.Split(app.excludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Handle directory pattern
		if strings.HasSuffix(pattern, "/") && strings.HasPrefix(dirPath, pattern) {
			return true // Skip this directory
		}
	}

	return false
}

// shouldIncludeFile determines if a file should be included based on filters
func (app *App) shouldIncludeFile(relPath string) bool {
	// In include mode, only include files that match include patterns
	if app.filterMode == IncludeMode {
		if app.includes == "" {
			return true // If no includes specified, include everything
		}

		for _, pattern := range strings.Split(app.includes, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}

			// Handle glob patterns
			if strings.HasPrefix(pattern, "*.") {
				ext := pattern[1:] // Get the extension with the *
				if strings.HasSuffix(relPath, ext) {
					return true
				}
			} else if strings.HasSuffix(pattern, "/") {
				// Handle directory pattern
				if strings.HasPrefix(relPath, pattern) {
					return true
				}
			} else if pattern == relPath {
				// Exact match
				return true
			}
		}
		return false
	}

	// In exclude mode, exclude files that match exclude patterns
	for _, pattern := range strings.Split(app.excludes, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Handle glob patterns
		if strings.HasPrefix(pattern, "*.") {
			ext := pattern[1:] // Get the extension with the *
			if strings.HasSuffix(relPath, ext) {
				return false
			}
		} else if strings.HasSuffix(pattern, "/") {
			// Handle directory pattern
			if strings.HasPrefix(relPath, pattern) {
				return false
			}
		} else if pattern == relPath {
			// Exact match
			return false
		}
	}

	return true // Include by default in exclude mode
}

// layout defines the TUI layout
func (app *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	filesWidth := maxX / 3 // Left pane takes 1/3 of the width
	filterHeight := 3      // Height for filter input area

	// --- Files View (Left Pane) ---
	filesViewHeight := maxY - 2
	if app.showFilter {
		filesViewHeight -= filterHeight
	}

	if v, err := g.SetView(filesViewName, 0, 0, filesWidth, filesViewHeight, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Files (%d) - [?] Help ", len(app.fileList))
		v.Highlight = true
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false // We handle scrolling manually

		// Populate the view
		app.refreshFilesView(g)

		// Set as current view initially
		if _, err := g.SetCurrentView(filesViewName); err != nil {
			return err
		}
	}

	// --- Filter View (Below Files View) ---
	if app.showFilter {
		filterY0 := filesViewHeight
		filterY1 := filesViewHeight + filterHeight

		if v, err := g.SetView(filterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Filter "
			v.Editable = true
			v.Wrap = true

			// Clear and redraw content if needed
			v.Clear()
			if app.filterMode == ExcludeMode {
				fmt.Fprintf(v, "Exclude: %s", app.excludes)
			} else {
				fmt.Fprintf(v, "Include: %s", app.includes)
			}

			// Set cursor at the end of input
			if app.filterMode == ExcludeMode {
				_ = v.SetCursor(len("Exclude: ")+len(app.excludes), 0)
			} else {
				_ = v.SetCursor(len("Include: ")+len(app.includes), 0)
			}

			if _, err := g.SetCurrentView(filterViewName); err != nil {
				return err
			}
		}
	} else {
		// Remove filter view if not shown
		g.DeleteView(filterViewName)
	}

	// --- Content View (Right Pane) ---
	if v, err := g.SetView(contentViewName, filesWidth+1, 0, maxX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content - PgUp/PgDn Scroll "
		v.Editable = false
		v.Wrap = true             // Wrap long lines
		v.Autoscroll = false      // We handle scrolling manually
		app.refreshContentView(g) // Initial content update
	}

	// --- Status Bar ---
	if v, err := g.SetView("status", 0, maxY-2, maxX-1, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		fmt.Fprintln(v, " ↑/↓: Navigate | Space: Select | A: Copy All | F: Toggle Filter | ?: Help | Ctrl+C: Quit")
	}

	// --- Help Popup (Conditional) ---
	if app.showHelp {
		helpWidth, helpHeight := maxX/2, maxY/2
		x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
		x1, y1 := x0+helpWidth, y0+helpHeight

		if v, err := g.SetView(helpViewName, x0, y0, x1, y1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Help "
			v.Wrap = true
			fmt.Fprintln(v, "grepforllm - Copy File Contents for LLM Input")
			fmt.Fprintln(v, "----------------------------------------")
			fmt.Fprintln(v, "Keyboard Shortcuts:")
			fmt.Fprintln(v, "  ↑           : Move cursor up in file list")
			fmt.Fprintln(v, "  ↓           : Move cursor down in file list")
			fmt.Fprintln(v, "  Space       : Toggle selection for the current file")
			fmt.Fprintln(v, "  A           : Copy contents of all selected files")
			fmt.Fprintln(v, "  F           : Toggle filter view (Exclude/Include)")
			fmt.Fprintln(v, "  PgUp/PgDn   : Scroll content view")
			fmt.Fprintln(v, "  ?           : Toggle this help message")
			fmt.Fprintln(v, "  Ctrl+C      : Quit the application")
			fmt.Fprintln(v, "\nPress '?' again to close this window.")

			if _, err := g.SetCurrentView(helpViewName); err != nil {
				return err
			}
		}
	} else {
		// Ensure help view is deleted if not shown
		_ = g.DeleteView(helpViewName)
		// Ensure focus returns to files view if help was just closed
		if g.CurrentView() != nil && g.CurrentView().Name() == helpViewName {
			if _, err := g.SetCurrentView(filesViewName); err != nil {
				return err
			}
		}
	}

	return nil
}

// refreshFilesView updates the content of the files view
func (app *App) refreshFilesView(g *gocui.Gui) {
	v, err := g.View(filesViewName)
	if err != nil {
		return // View might not be ready yet
	}
	v.Clear()

	title := fmt.Sprintf(" Files (%d/%d) - [?] Help ", len(app.selectedFiles), len(app.fileList))
	if app.filterMode == ExcludeMode {
		title += "[Exclude Mode]"
	} else {
		title += "[Include Mode]"
	}
	v.Title = title

	for i, file := range app.fileList {
		prefix := "[ ]"
		if app.selectedFiles[file] {
			prefix = "[*]"
		}
		// Ensure cursor stays within bounds if list shrinks
		if app.currentLine >= len(app.fileList) {
			app.currentLine = max(0, len(app.fileList)-1)
		}
		// Set cursor position for the current line
		if i == app.currentLine {
			// The library handles drawing the selection highlight based on SelBgColor/SelFgColor
			// We just need to ensure the cursor is logically on this line
			_, err := v.Line(i) // Check if line exists before setting cursor (might not with filtering)
			if err == nil {
				_ = v.SetCursor(0, i) // Set internal cursor position
			} else {
				// If line doesn't exist (e.g., view empty), reset cursor
				_ = v.SetCursor(0, 0)
			}
		}
		fmt.Fprintf(v, "%s %s\n", prefix, file)
	}
}

// refreshContentView updates the content of the content view based on selected files
func (app *App) refreshContentView(g *gocui.Gui) {
	v, err := g.View(contentViewName)
	if err != nil {
		return // View might not be ready yet
	}
	v.Clear()
	_ = v.SetOrigin(0, 0) // Reset scroll position

	var contentBuilder strings.Builder
	count := 0

	// Iterate through fileList to maintain order, but check selection map
	for _, relPath := range app.fileList {
		if app.selectedFiles[relPath] {
			fullPath := filepath.Join(app.rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			if err != nil {
				// Log error reading file but continue
				log.Printf("Warning: Error reading file %s: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("FILE: %s\n", relPath))
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("FILE: %s\n", relPath))
				contentBuilder.WriteString(fmt.Sprintf("==========================\n\n"))
				contentBuilder.WriteString(string(fileContent))
				contentBuilder.WriteString("\n\n") // Add space between files
			}
			count++
		}
	}

	if count == 0 {
		fmt.Fprintln(v, "Select files using Spacebar to view their content here.")
	} else {
		fmt.Fprint(v, contentBuilder.String())
	}
	v.Title = fmt.Sprintf(" Content (%d files) - PgUp/PgDn Scroll ", count)
}

// --- Keybinding Handlers ---

func (app *App) keybindings(g *gocui.Gui) error {
	// --- Global ---
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.toggleHelp); err != nil {
		return err
	}

	// --- Files View Specific (or when focused) ---
	if err := g.SetKeybinding(filesViewName, gocui.KeyArrowUp, gocui.ModNone, app.cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, gocui.KeyArrowDown, gocui.ModNone, app.cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, gocui.KeySpace, gocui.ModNone, app.toggleSelect); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, 'a', gocui.ModNone, app.copyAllSelected); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, 'A', gocui.ModNone, app.copyAllSelected); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, 'f', gocui.ModNone, app.toggleFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(filesViewName, 'F', gocui.ModNone, app.toggleFilter); err != nil {
		return err
	}

	// --- Content View Scrolling (Global binding for simplicity) ---
	if err := g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, app.scrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, app.scrollContentDown); err != nil {
		return err
	}

	// --- Help View Specific ---
	if err := g.SetKeybinding(helpViewName, '?', gocui.ModNone, app.toggleHelp); err != nil {
		return err
	}
	// Allow closing help with Esc as well
	if err := g.SetKeybinding(helpViewName, gocui.KeyEsc, gocui.ModNone, app.toggleHelp); err != nil {
		return err
	}

	// --- Filter View Specific ---
	if err := g.SetKeybinding(filterViewName, gocui.KeyEnter, gocui.ModNone, app.applyFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(filterViewName, gocui.KeyEsc, gocui.ModNone, app.cancelFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(filterViewName, 'f', gocui.ModNone, app.toggleFilterMode); err != nil {
		return err
	}
	if err := g.SetKeybinding(filterViewName, 'F', gocui.ModNone, app.toggleFilterMode); err != nil {
		return err
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (app *App) toggleHelp(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showHelp = !app.showHelp
	app.mutex.Unlock()

	if !app.showHelp {
		// If closing help, ensure focus returns to files view
		_ = g.DeleteView(helpViewName)
		_, err := g.SetCurrentView(filesViewName)
		return err
	}
	// Layout function will handle creating/focusing help view
	return nil
}

func (app *App) toggleFilter(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showFilter = !app.showFilter
	app.mutex.Unlock()

	// Layout will handle creating/removing the filter view
	g.Update(func(g *gocui.Gui) error {
		return nil // Force update
	})

	return nil
}

func (app *App) toggleFilterMode(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if app.filterMode == ExcludeMode {
		app.filterMode = IncludeMode
	} else {
		app.filterMode = ExcludeMode
	}
	app.mutex.Unlock()

	// Update the filter view content
	v.Clear()
	if app.filterMode == ExcludeMode {
		fmt.Fprintf(v, "Exclude: %s", app.excludes)
		_ = v.SetCursor(len("Exclude: ")+len(app.excludes), 0)
	} else {
		fmt.Fprintf(v, "Include: %s", app.includes)
		_ = v.SetCursor(len("Include: ")+len(app.includes), 0)
	}

	return nil
}

func (app *App) applyFilter(g *gocui.Gui, v *gocui.View) error {
	// Get the content from filter view
	buffer := v.Buffer()
	buffer = strings.TrimSpace(buffer)

	// Parse the input
	if app.filterMode == ExcludeMode {
		prefix := "Exclude: "
		if strings.HasPrefix(buffer, prefix) {
			app.excludes = strings.TrimPrefix(buffer, prefix)
		}
	} else {
		prefix := "Include: "
		if strings.HasPrefix(buffer, prefix) {
			app.includes = strings.TrimPrefix(buffer, prefix)
		}
	}

	// Apply the filter and refresh view
	app.applyFilters()

	// Close filter view and return to files view
	app.showFilter = false
	g.Update(func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(filesViewName)
		return err
	})

	return nil
}

func (app *App) cancelFilter(g *gocui.Gui, v *gocui.View) error {
	// Close filter view without applying changes
	app.showFilter = false
	g.Update(func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(filesViewName)
		return err
	})

	return nil
}

func (app *App) cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || len(app.fileList) == 0 {
		return nil
	}
	if app.currentLine > 0 {
		app.currentLine--
		// Ensure the view scrolls if needed
		if err := app.adjustFilesViewScroll(g, v); err != nil {
			log.Printf("Error adjusting scroll: %v", err)
		}
		app.refreshFilesView(g) // Refresh to update cursor position visually if needed
	}
	return nil
}

func (app *App) cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || len(app.fileList) == 0 {
		return nil
	}
	if app.currentLine < len(app.fileList)-1 {
		app.currentLine++
		// Ensure the view scrolls if needed
		if err := app.adjustFilesViewScroll(g, v); err != nil {
			log.Printf("Error adjusting scroll: %v", err)
		}
		app.refreshFilesView(g) // Refresh to update cursor position visually if needed
	}
	return nil
}

// adjustFilesViewScroll ensures the current line is visible
func (app *App) adjustFilesViewScroll(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	cx, cy := v.Cursor() // Cursor relative to view origin

	// Get view height
	_, vy := v.Size()
	viewHeight := vy

	// Calculate absolute cursor position (line number)
	absCursorY := oy + cy

	// If cursor moved out of view (above)
	if absCursorY < oy {
		// Scroll up to bring cursor into view
		if err := v.SetOrigin(ox, absCursorY); err != nil {
			return err
		}
	}

	// If cursor moved out of view (below)
	if absCursorY >= oy+viewHeight {
		// Scroll down to bring cursor into view
		if err := v.SetOrigin(ox, absCursorY-viewHeight+1); err != nil {
			return err
		}
	}

	// Ensure the internal cursor is set correctly relative to the (potentially new) origin
	_, newOy := v.Origin()
	return v.SetCursor(cx, app.currentLine-newOy)
}

func (app *App) toggleSelect(g *gocui.Gui, v *gocui.View) error {
	if v == nil || len(app.fileList) == 0 || app.currentLine >= len(app.fileList) {
		return nil
	}

	selectedFile := app.fileList[app.currentLine]
	app.selectedFiles[selectedFile] = !app.selectedFiles[selectedFile]

	// If deselected, remove from map
	if !app.selectedFiles[selectedFile] {
		delete(app.selectedFiles, selectedFile)
	}

	// Refresh both views
	app.refreshFilesView(g)
	app.refreshContentView(g)
	return nil
}

func (app *App) copyAllSelected(g *gocui.Gui, v *gocui.View) error {
	if len(app.selectedFiles) == 0 {
		// Optionally show a message if nothing is selected
		statusView, err := g.View("status")
		if err == nil {
			statusView.Clear()
			// Keep the original status text or show a specific message
			// For now, let's just revert to the default help text
			fmt.Fprintln(statusView, " ↑/↓: Navigate | Space: Select | A: Copy All | F: Toggle Filter | ?: Help | Ctrl+C: Quit")
			fmt.Fprintln(statusView, "No files selected to copy.") // Add a temporary message line
			// Consider adding a timer to clear this temporary message later if desired
		}
		return nil // Nothing to copy
	}

	var contentBuilder strings.Builder
	count := 0

	// Iterate through fileList to maintain order
	for _, relPath := range app.fileList {
		if app.selectedFiles[relPath] {
			fullPath := filepath.Join(app.rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			if err != nil {
				// Log the error for debugging, but still include info in clipboard
				log.Printf("Warning: Error reading file %s for copy: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("FILE: %s\n", relPath))
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("==========================\n"))
				contentBuilder.WriteString(fmt.Sprintf("FILE: %s\n", relPath))
				contentBuilder.WriteString(fmt.Sprintf("==========================\n\n"))
				contentBuilder.WriteString(string(fileContent))
				contentBuilder.WriteString("\n\n") // Add space between files
			}
			count++
		}
	}

	// Copy to clipboard
	content := contentBuilder.String()
	err := clipboard.WriteAll(content)

	// Update status bar regardless of success or failure
	statusView, statusErr := g.View("status")
	if statusErr == nil { // Only update if we can get the status view
		statusView.Clear()
		// Always show the standard controls first
		fmt.Fprintln(statusView, " ↑/↓: Navigate | Space: Select | A: Copy All | F: Toggle Filter | ?: Help | Ctrl+C: Quit")

		if err != nil {
			log.Printf("Error copying to clipboard: %v", err)
			// Show error message in the second line of the status bar
			fmt.Fprintln(statusView, "Error copying to clipboard!")
		} else {
			// Show success message in the second line
			successMsg := fmt.Sprintf("Copied content of %d file(s) to clipboard.", count)
			fmt.Fprintln(statusView, successMsg)
			// Optional: Clear the success message after a short delay?
			// This would require using timers and might complicate things.
			// For now, the message stays until the next status update.
		}
	} else {
		// Log if we couldn't even get the status view
		log.Printf("Error getting status view to show copy status: %v", statusErr)
	}

	// Even if clipboard write fails, it's not a fatal error for the TUI loop
	return nil
}

// --- Missing Utility Function ---

// max returns the larger of x or y.
// Used for ensuring indices/positions stay within valid bounds.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- Missing Keybinding Handlers ---

// scrollContentUp scrolls the content view up by approximately one page height.
func (app *App) scrollContentUp(g *gocui.Gui, v *gocui.View) error {
	contentView, err := g.View(contentViewName)
	if err != nil {
		// Content view might not exist yet, ignore
		return nil
	}
	ox, oy := contentView.Origin()
	_, vy := contentView.Size() // Get height for page calculation

	// Calculate new origin, scroll up by view height minus 1 (for overlap)
	newOy := oy - (vy - 1)
	if newOy < 0 {
		newOy = 0 // Don't scroll past the top
	}

	// Set the new origin for the content view
	return contentView.SetOrigin(ox, newOy)
}

// scrollContentDown scrolls the content view down by approximately one page height.
func (app *App) scrollContentDown(g *gocui.Gui, v *gocui.View) error {
	contentView, err := g.View(contentViewName)
	if err != nil {
		// Content view might not exist yet, ignore
		return nil
	}
	ox, oy := contentView.Origin()
	_, vy := contentView.Size() // Get height for page calculation

	// Calculate the potential new origin
	newOy := oy + (vy - 1) // Scroll down by view height minus 1 (for overlap)

	// Check against the total number of lines to prevent scrolling past the end
	// contentView.ViewLines() gets the lines *currently* rendered based on wrapping.
	// contentView.BufferLines() gets the raw lines before wrapping. Use BufferLines for accurate scrolling limits.
	bufferLines := contentView.BufferLines()
	numLines := len(bufferLines)
	if numLines == 0 {
		return nil // Nothing to scroll
	}

	// The maximum origin Y allows the last line to be visible at the bottom.
	// If the number of lines is less than the view height, max origin is 0.
	maxOy := 0
	if numLines > vy {
		maxOy = numLines - vy
	}

	// Clamp newOy to the maximum possible origin
	if newOy > maxOy {
		newOy = maxOy
	}

	// Set the new origin for the content view
	return contentView.SetOrigin(ox, newOy)
}
