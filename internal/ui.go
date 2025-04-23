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
	filterY0 := filesViewY1 + 1
	filterY1 := filterY0 + filterHeight - 1
	contentViewY1 := maxY - statusBarHeight - 1
	statusBarY0 := maxY - statusBarHeight
	statusBarY1 := maxY - 1

	currentView := g.CurrentView()
	currentViewName := ""
	if currentView != nil {
		currentViewName = currentView.Name()
	}

	// --- Files View ---
	if v, err := g.SetView(FilesViewName, 0, 0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Files "
		v.Highlight = true
		v.SelBgColor = gocui.ColorDefault
		v.SelFgColor = gocui.ColorYellow | gocui.AttrBold
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false

		app.refreshFilesView(g)

		if g.CurrentView() == nil {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
			currentViewName = FilesViewName
		}
	} else {
		if currentViewName == FilesViewName {
			v.FrameColor = gocui.ColorGreen
		} else {
			v.FrameColor = gocui.ColorBlue
		}
		app.refreshFilesView(g)
	}

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
	}

	if v, err := g.SetView(StatusViewName, 0, statusBarY0, maxX-1, statusBarY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			log.Printf("Error setting status view: %v", err)
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		v.FgColor = gocui.ColorWhite
		v.BgColor = gocui.ColorBlue
		app.resetStatus(g)
	}

	// --- Help View (Modal) ---
	if app.showHelp {
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
	selectedCount := len(app.selectedFiles)
	totalCount := len(app.fileList)

	if app.currentLine >= totalCount {
		app.currentLine = max(0, totalCount-1)
	}
	if app.currentLine < 0 && totalCount > 0 {
		app.currentLine = 0
	} else if totalCount == 0 {
		app.currentLine = 0
	}

	currentFileList := make([]string, totalCount)
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	currentLine := app.currentLine
	app.mutex.Unlock()

	title := fmt.Sprintf(" Files (%d/%d) %s [?] Help ", selectedCount, totalCount, modeStr)
	v.Title = title

	for i, file := range currentFileList {
		isSelected := currentSelectedFiles[file]
		isCurrent := (i == currentLine)
		prefix := "[ ]"
		if isSelected {
			prefix = "[*]"
		}
		line := fmt.Sprintf("%s %s", prefix, file)

		if isCurrent {
			fmt.Fprintln(v, line)
		} else if isSelected {
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line)
		} else {
			fmt.Fprintln(v, line)
		}
	}

	if totalCount > 0 {
		err = v.SetCursor(0, currentLine)
		if err != nil {
			log.Printf("Error setting cursor in files view: %v", err)
		}
	} else {
		err = v.SetCursor(0, 0)
		if err != nil {
			log.Printf("Error resetting cursor in empty files view: %v", err)
		}
	}

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
	selectedCount := len(app.selectedFiles)
	currentFileList := make([]string, len(app.fileList))
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	rootDir := app.rootDir
	app.mutex.Unlock()

	for _, relPath := range currentFileList {
		if currentSelectedFiles[relPath] {
			fullPath := filepath.Join(rootDir, relPath)
			fileContent, err := os.ReadFile(fullPath)
			separator := fmt.Sprintf("--- BEGIN FILE: %s ---\n", relPath)
			endSeparator := fmt.Sprintf("--- END FILE: %s ---\n\n", relPath)

			contentBuilder.WriteString(separator)
			if err != nil {
				log.Printf("Warning: Error reading file %s: %v", fullPath, err)
				contentBuilder.WriteString(fmt.Sprintf("\n!!! ERROR READING FILE: %v !!!\n", err))
			} else {
				if contentBuilder.Len() > len(separator) && !strings.HasSuffix(contentBuilder.String(), "\n") {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.Write(fileContent)
				if !strings.HasSuffix(string(fileContent), "\n") {
					contentBuilder.WriteString("\n")
				}
			}
			contentBuilder.WriteString(endSeparator)
			count++
		}
	}

	v.Title = fmt.Sprintf(" Content (%d files) - PgUp/PgDn Scroll ", count)

	if count == 0 {
		fmt.Fprintln(v, "Select files using [Space] to view content.")
		fmt.Fprintln(v, "\nUse [?] for help.")
	} else {
		fmt.Fprint(v, contentBuilder.String())
	}

	err = v.SetOrigin(currentOx, currentOy)
	if err != nil {
		_ = v.SetOrigin(0, 0)
		log.Printf("Warning: Could not restore content view origin (%d, %d), resetting. Error: %v", currentOx, currentOy, err)
	}
}

func (app *App) updateStatus(g *gocui.Gui, message string) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err == nil {
			v.Clear()
			fmt.Fprint(v, message)
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
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
			v.Rewind()
		} else if err != gocui.ErrUnknownView {
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
