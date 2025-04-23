package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"
)

// Layout defines the TUI layout.
func (app *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	filesWidth := maxX / 3 // Left pane takes 1/3 of the width
	filterHeight := 3      // Height for filter input area
	statusBarHeight := 1   // Standard single line status bar

	// --- Files View (Left Pane) --- Always leaves space for filter and status
	filesViewY1 := maxY - statusBarHeight - filterHeight - 1
	if v, err := g.SetView(FilesViewName, 0, 0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Files " // Title updated in refreshFilesView
		v.Highlight = true
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false // We handle scrolling manually

		app.refreshFilesView(g) // Populate the view

		// Set as current view initially if nothing else is focused
		if g.CurrentView() == nil {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
		}
	}

	// --- Filter View (Below Files View, Always Visible) ---
	filterY0 := filesViewY1 + 1
	filterY1 := filterY0 + filterHeight - 1

	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil { // Don't use gocui.TOP here
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Filter (Enter: Apply, Esc: Cancel, Ctrl+E: Edit) "
		v.Editable = true
		v.Wrap = false // Usually single line input

		// Set initial content
		app.updateFilterViewContent(g)
	} else {
		// Ensure content is up-to-date on resize/refresh
		app.updateFilterViewContent(g)
	}

	// --- Content View (Right Pane) ---
	contentViewY1 := maxY - statusBarHeight - 1
	if v, err := g.SetView(ContentViewName, filesWidth+1, 0, maxX-1, contentViewY1, 0); err != nil {
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
	statusY0 := maxY - statusBarHeight - 1 // Positioned at the very bottom
	if v, err := g.SetView(StatusViewName, 0, statusY0+1, maxX-1, statusY0+statusBarHeight+1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		// Initial status message - might be overwritten by actions
		fmt.Fprint(v, "↑/↓: Nav | Space: Sel | a: Sel All | c: Copy | f: Filter Mode | Ctrl+E: Edit Filter | ?: Help | Ctrl+C: Quit")
	}

	// --- Help Popup (Conditional, on top) ---
	if app.showHelp {
		helpWidth := maxX / 2
		helpHeight := maxY / 2
		x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
		x1, y1 := x0+helpWidth, y0+helpHeight

		if v, err := g.SetView(HelpViewName, x0, y0, x1, y1, gocui.TOP); err != nil { // Use gocui.TOP
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Help (? or Esc to Close) "
			v.Wrap = true
			fmt.Fprintln(v, "grepforllm - Copy File Contents for LLM Input")
			fmt.Fprintln(v, "----------------------------------------")
			fmt.Fprintln(v, "Keyboard Shortcuts:")
			fmt.Fprintln(v, "  ↑ / k       : Move cursor up in file list")
			fmt.Fprintln(v, "  ↓ / j       : Move cursor down")
			fmt.Fprintln(v, "  Space       : Toggle select file")
			fmt.Fprintln(v, "  a           : Select / Deselect all visible files")
			fmt.Fprintln(v, "  c           : Copy contents of selected files")
			fmt.Fprintln(v, "  f           : Toggle filter mode (Include/Exclude)")
			fmt.Fprintln(v, "  Ctrl+E      : Focus filter input for editing")
			fmt.Fprintln(v, "  PgUp / Ctrl+B: Scroll content view UP")
			fmt.Fprintln(v, "  PgDn / Ctrl+F: Scroll content view DOWN")
			fmt.Fprintln(v, "  ?           : Toggle this help message")
			fmt.Fprintln(v, "  Ctrl+C / q  : Quit the application")
			fmt.Fprintln(v, "\nIn Filter View:")
			fmt.Fprintln(v, "  Enter       : Apply filter and close")
			fmt.Fprintln(v, "  Esc         : Cancel filter and close")
			fmt.Fprintln(v, "  (Use comma-separated patterns, e.g., *.go,cmd/,Makefile)")

			if _, err := g.SetCurrentView(HelpViewName); err != nil {
				return err
			}
		}
	} else {
		_ = g.DeleteView(HelpViewName)
		// Ensure focus returns to files view if help was just closed
		if g.CurrentView() != nil && g.CurrentView().Name() == HelpViewName {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				log.Printf("Error setting current view to files after closing help: %v", err)
			}
		}
	}

	return nil
}

// refreshFilesView updates the content of the files view.
func (app *App) refreshFilesView(g *gocui.Gui) {
	v, err := g.View(FilesViewName)
	if err != nil {
		return // View might not exist yet
	}
	v.Clear()

	// Update title with counts and mode
	modeStr := "[Exclude]"
	if app.filterMode == IncludeMode {
		modeStr = "[Include]"
	}
	v.Title = fmt.Sprintf(" Files (%d/%d) %s [?] Help ", len(app.selectedFiles), len(app.fileList), modeStr)

	// Ensure cursor stays within bounds if list shrinks
	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1)
	}

	for _, file := range app.fileList {
		prefix := "[ ]"
		if app.selectedFiles[file] {
			prefix = "[*]"
		}
		// Let gocui handle the highlight based on the current view and cursor position
		fmt.Fprintf(v, "%s %s\n", prefix, file)
	}

	// Set gocui's internal cursor. It will handle drawing the highlight.
	// Need to calculate cursor relative to origin.
	_, oy := v.Origin()
	cursorYInView := app.currentLine - oy
	_ = v.SetCursor(0, cursorYInView) // X is always 0 for selection line

	// Adjust origin if cursor is out of view (handled in cursor movement, but good safety check)
	app.adjustFilesViewScroll(g, v)
}

// refreshContentView updates the content view based on selected files.
func (app *App) refreshContentView(g *gocui.Gui) {
	v, err := g.View(ContentViewName)
	if err != nil {
		return
	}
	v.Clear()
	_ = v.SetOrigin(0, 0) // Reset scroll position on refresh

	var contentBuilder strings.Builder
	count := 0

	// Iterate through fileList to maintain display order, check selection map
	for _, relPath := range app.fileList {
		if app.selectedFiles[relPath] {
			fullPath := filepath.Join(app.rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("--- FILE: %s ---\n", relPath) // Simpler separator

			contentBuilder.WriteString(separator)
			if err != nil {
				log.Printf("Warning: Error reading file %s: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				// Add newline before content only if builder is not empty and doesn't end with newline
				if contentBuilder.Len() > 0 && !strings.HasSuffix(contentBuilder.String(), "\n") {
					contentBuilder.WriteString("\n")
				}
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

	if count == 0 {
		fmt.Fprintln(v, "Select files using [Space] to view content.")
		fmt.Fprintln(v, "\nUse [?] for help.")
	} else {
		// Use Fprint to avoid extra newline at the end
		fmt.Fprint(v, contentBuilder.String())
	}
	v.Title = fmt.Sprintf(" Content (%d files) - PgUp/PgDn Scroll ", count)
}

// updateStatus displays a temporary message on the status bar.
func (app *App) updateStatus(g *gocui.Gui, message string) {
	v, err := g.View(StatusViewName)
	if err == nil {
		v.Clear()
		// Keep standard controls visible if possible, or just show the message
		fmt.Fprint(v, message) // Use Fprint
	} else {
		log.Printf("Error updating status view: %v", err)
	}
}

// resetStatus restores the default status bar message.
func (app *App) resetStatus(g *gocui.Gui) {
	v, err := g.View(StatusViewName)
	if err == nil {
		v.Clear()
		fmt.Fprint(v, "↑/↓: Nav | Space: Sel | a: Sel All | c: Copy | f: Filter Mode | Ctrl+E: Edit Filter | ?: Help | Ctrl+C: Quit")
	}
}

// refreshViews is a helper to refresh multiple common views
func (app *App) refreshViews(g *gocui.Gui) {
	app.refreshFilesView(g)
	app.refreshContentView(g)
	// Status might have been updated, don't reset it here unless needed
	// Filter view content updated by updateFilterViewContent when mode changes
}
