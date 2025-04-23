package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"
)

func (app *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	filesWidth := maxX / 3
	filterHeight := 3
	statusBarHeight := 1

	filesViewY1 := maxY - statusBarHeight - filterHeight - 1
	if v, err := g.SetView(FilesViewName, 0, 0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false

		app.refreshFilesView(g)

		if g.CurrentView() == nil {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
		}
	} else {
		app.refreshFilesView(g)
	}

	filterY0 := filesViewY1 + 1
	filterY1 := filterY0 + filterHeight - 1

	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false             // Start non-editable
		v.Wrap = false                 // Usually single line input
		v.Highlight = true             // Highlight frame when focused
		v.Editor = gocui.DefaultEditor // Need an editor even if not initially editable

		app.updateFilterViewContent(g)
	}

	if v, err := g.View(FilterViewName); err == nil {
		app.mutex.Lock()
		modeStr := "Exclude"
		if app.filterMode == IncludeMode {
			modeStr = "Include"
		}
		app.mutex.Unlock()
		v.Title = fmt.Sprintf(" Filter: %s (Tab: Focus, Enter: Apply, Ctrl+F: Mode) ", modeStr)

		if !v.Editable {
			app.updateFilterViewContent(g)
		}
	}

	contentViewY1 := maxY - statusBarHeight - 1
	if v, err := g.SetView(ContentViewName, filesWidth+1, 0, maxX-1, contentViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content - PgUp/PgDn Scroll "
		v.Editable = false
		v.Wrap = true
		v.Autoscroll = false
		app.refreshContentView(g)
	} else {
		app.refreshContentView(g)
	}

	statusY0 := maxY - statusBarHeight - 1
	if v, err := g.SetView(StatusViewName, 0, statusY0+1, maxX-1, statusY0+statusBarHeight+1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		fmt.Fprint(v, "↑/↓: Nav | Space: Sel | a: Sel All | c: Copy | Tab: Focus Filter | ?: Help | Ctrl+C: Quit")
	}

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
			fmt.Fprintln(v, "  Tab         : Switch focus between Files and Filter")
			fmt.Fprintln(v, "  PgUp / Ctrl+B: Scroll content view UP")
			fmt.Fprintln(v, "  PgDn / Ctrl+F: Scroll content view DOWN")
			fmt.Fprintln(v, "  ?           : Toggle this help message")
			fmt.Fprintln(v, "  Ctrl+C / q  : Quit the application")
			fmt.Fprintln(v, "\nIn Filter View (when focused):")
			fmt.Fprintln(v, "  Enter       : Apply filter and return to Files")
			fmt.Fprintln(v, "  Esc         : Cancel filter and return to Files")
			fmt.Fprintln(v, "  Ctrl+F      : Toggle filter mode (Include/Exclude)")
			fmt.Fprintln(v, "  (Use comma-separated patterns, e.g., *.go,cmd/,Makefile)")

			if _, err := g.SetCurrentView(HelpViewName); err != nil {
				return err
			}
		}
	} else {
		_ = g.DeleteView(HelpViewName)
		if cv := g.CurrentView(); cv != nil && cv.Name() == HelpViewName {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				log.Printf("Error setting current view to files after closing help: %v", err)
			}
		}
	}

	return nil
}

func (app *App) refreshFilesView(g *gocui.Gui) {
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
	title := fmt.Sprintf(" Files (%d/%d) %s [?] Help ", len(app.selectedFiles), len(app.fileList), modeStr)
	v.Title = title

	if app.currentLine >= len(app.fileList) {
		app.currentLine = max(0, len(app.fileList)-1)
	}

	currentFileList := app.fileList // Use local copy within lock
	currentSelectedFiles := app.selectedFiles
	currentLine := app.currentLine
	app.mutex.Unlock() // Unlock after reading shared state

	for _, file := range currentFileList {
		prefix := "[ ]"
		if currentSelectedFiles[file] {
			prefix = "[*]"
		}
		line := fmt.Sprintf("%s %s", prefix, file)
		fmt.Fprintln(v, line)
	}

	_, oy := v.Origin()
	cursorYInView := currentLine - oy
	_ = v.SetCursor(0, cursorYInView)

	app.adjustFilesViewScroll(g, v)
}

func (app *App) refreshContentView(g *gocui.Gui) {
	v, err := g.View(ContentViewName)
	if err != nil {
		return
	}
	currentOx, currentOy := v.Origin()
	v.Clear()

	var contentBuilder strings.Builder
	count := 0

	app.mutex.Lock()
	currentFileList := app.fileList // Use local copy within lock
	currentSelectedFiles := app.selectedFiles
	rootDir := app.rootDir
	app.mutex.Unlock() // Unlock after reading shared state

	for _, relPath := range currentFileList {
		if currentSelectedFiles[relPath] {
			fullPath := filepath.Join(rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("--- FILE: %s ---\n", relPath)

			contentBuilder.WriteString(separator)
			if err != nil {
				log.Printf("Warning: Error reading file %s: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n\n", err))
			} else {
				if contentBuilder.Len() > 0 && !strings.HasSuffix(contentBuilder.String(), "\n") {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.WriteString(string(fileContent))
				if !strings.HasSuffix(string(fileContent), "\n") {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.WriteString("\n")
			}
			count++
		}
	}

	if count == 0 {
		fmt.Fprintln(v, "Select files using [Space] to view content.")
		fmt.Fprintln(v, "\nUse [?] for help.")
	} else {
		fmt.Fprint(v, contentBuilder.String())
	}
	v.Title = fmt.Sprintf(" Content (%d files) - PgUp/PgDn Scroll ", count)

	_ = v.SetOrigin(currentOx, currentOy)
	app.scrollContent(g, 0)
}

func (app *App) updateStatus(g *gocui.Gui, message string) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, message)
		} else {
			log.Printf("Error finding status view to update: %v", err)
		}
		return nil
	})
}

func (app *App) resetStatus(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, "↑/↓: Nav | Space: Sel | a: Sel All | c: Copy | Tab: Focus Filter | ?: Help | Ctrl+C: Quit")
		} else {
			log.Printf("Error finding status view to reset: %v", err)
		}
		return nil
	})
}

func (app *App) refreshViews(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		app.refreshFilesView(g)
		app.refreshContentView(g)
		return nil
	})
}
