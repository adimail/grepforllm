package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"
)

func (app *App) Layout(g *gocui.Gui) error {
	app.mutex.Lock()
	showCache := app.showCacheView
	showHelp := app.showHelp
	isLoading := app.isLoading
	loadingError := app.loadingError
	app.mutex.Unlock()

	if isLoading {
		return app.layoutLoadingView(g)
	}
	if loadingError != nil {
		return app.layoutErrorView(g, loadingError)
	}

	if showCache {
		return app.layoutCacheView(g)
	} else if showHelp {
		_ = app.GrepApplicationView(g)
		return app.layoutHelpView(g)
	} else {
		if _, err := g.View(HelpViewName); err == nil {
			_ = g.DeleteView(HelpViewName)
		}
		return app.GrepApplicationView(g)
	}
}

func (app *App) GrepApplicationView(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	_ = g.DeleteView(CacheViewName)
	_ = g.DeleteView("loading")
	_ = g.DeleteView("error")

	app.mutex.Lock()
	mode := app.filterMode
	app.mutex.Unlock()

	filesWidth := maxX / 3
	pathHeight := 2
	statusBarHeight := 2
	filterHeight := 3

	pathY0 := 0
	pathY1 := pathY0 + pathHeight

	var filesViewY1 int
	if mode == IncludeMode {
		filesViewY1 = maxY - statusBarHeight - (filterHeight * 2)
	} else {
		filesViewY1 = maxY - statusBarHeight - filterHeight
	}

	filesY0 := pathY1 + 1

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
		pv.Clear()
		fmt.Fprint(pv, app.rootDir)
	} else {
		pv.Clear()
		fmt.Fprint(pv, app.rootDir)
	}

	if v, err := g.SetView(FilesViewName, 0, filesY0, filesWidth, filesViewY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Files "
		v.Highlight = false
		v.Editable = false
		v.Wrap = false
		v.Autoscroll = false

		if g.CurrentView() == nil || currentViewName == CacheViewName {
			if _, err := g.SetCurrentView(FilesViewName); err != nil {
				return err
			}
			currentViewName = FilesViewName
		}
	}
	app.refreshFilesView(g)
	fv, _ := g.View(FilesViewName)
	if fv != nil {
		if currentViewName == FilesViewName {
			fv.FrameColor = gocui.ColorGreen
		} else {
			fv.FrameColor = gocui.ColorBlue
		}
	}

	if mode == IncludeMode {
		incY0 := filesViewY1 + 1
		incY1 := incY0 + filterHeight - 1
		if v, err := g.SetView(FilterIncludeViewName, 0, incY0, filesWidth, incY1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Filter: Include (Ctrl+F: Mode) "
			v.Editable = true
			v.Editor = gocui.DefaultEditor
			app.mutex.Lock()
			v.Clear()
			fmt.Fprint(v, app.includes)
			app.mutex.Unlock()
		}
		excY0 := incY1 + 1
		excY1 := excY0 + filterHeight - 1
		if v, err := g.SetView(FilterExcludeViewName, 0, excY0, filesWidth, excY1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Filter: Exclude "
			v.Editable = true
			v.Editor = gocui.DefaultEditor
			app.mutex.Lock()
			v.Clear()
			fmt.Fprint(v, app.excludes)
			app.mutex.Unlock()
		}
		_ = g.DeleteView("filter")
	} else {
		excY0 := filesViewY1 + 1
		excY1 := excY0 + filterHeight - 1
		if v, err := g.SetView(FilterExcludeViewName, 0, excY0, filesWidth, excY1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Filter: Exclude (Ctrl+F: Mode) "
			v.Editable = true
			v.Editor = gocui.DefaultEditor
			app.mutex.Lock()
			v.Clear()
			fmt.Fprint(v, app.excludes)
			app.mutex.Unlock()
		}
		_ = g.DeleteView(FilterIncludeViewName)
	}

	for _, vName := range []string{FilterIncludeViewName, FilterExcludeViewName} {
		if v, err := g.View(vName); err == nil {
			if currentViewName == vName {
				v.FrameColor = gocui.ColorGreen
				v.FgColor = gocui.ColorWhite | gocui.AttrBold
				v.Editable = true
			} else {
				v.FrameColor = gocui.ColorBlue
				v.FgColor = gocui.ColorCyan
				v.Editable = false
			}
		}
	}

	app.updateFilterViewContent(g)

	contentX0 := filesWidth + 1
	contentY0 := 0
	if v, err := g.SetView(ContentViewName, contentX0, contentY0, maxX-1, maxY-statusBarHeight, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Content "
		v.Editable = false
		v.Wrap = true
		v.Autoscroll = false
		v.Frame = true
	}
	app.refreshContentView(g)
	cv, _ := g.View(ContentViewName)
	if cv != nil {
		if currentViewName == ContentViewName {
			cv.FrameColor = gocui.ColorGreen
		} else {
			cv.FrameColor = gocui.ColorBlue
		}
	}

	if v, err := g.SetView(StatusViewName, 0, maxY-statusBarHeight, maxX-1, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		v.FgColor = gocui.ColorWhite
		v.BgColor = gocui.ColorDefault
	}
	app.resetStatus(g)

	return nil
}

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
		v.FgColor = gocui.ColorWhite

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

		if _, err := g.SetCurrentView(HelpViewName); err != nil {
			return err
		}
		v.FrameColor = gocui.ColorGreen
	} else {
		if g.CurrentView() != v {
			if _, err := g.SetCurrentView(HelpViewName); err != nil {
				return err
			}
		}
		v.FrameColor = gocui.ColorGreen
	}
	return nil
}

func (app *App) layoutCacheView(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	viewsToDelete := []string{
		FilesViewName, ContentViewName, FilterIncludeViewName, FilterExcludeViewName, PathViewName,
		HelpViewName,
		"loading",
		"error",
	}
	for _, viewName := range viewsToDelete {
		_ = g.DeleteView(viewName)
	}

	cacheViewY1 := maxY - 2

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
			if _, err := g.SetCurrentView(CacheViewName); err == nil {
				cv.FrameColor = gocui.ColorGreen
			} else {
				cv.FrameColor = gocui.ColorCyan
			}
		}
	}

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
			fmt.Fprintf(v, "\x1b[7m%s\x1b[0m\n", line)
		case isCurrent:
			fmt.Fprintf(v, "\x1b[34;1m%s\x1b[0m\n", line)
		case isSelected:
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line)
		default:
			fmt.Fprintln(v, line)
		}
	}

	if totalCount > 0 {
		_ = v.SetCursor(0, currentLine)
	} else {
		_ = v.SetCursor(0, 0)
	}

	app.adjustFilesViewScroll(g, v)

	app.resetStatus(g)
}

func (app *App) refreshContentView(g *gocui.Gui) {
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
	app.mutex.Unlock()

	_, viewHeight := v.Size()
	bufferLines := strings.Count(v.ViewBuffer(), "\n") + 1
	maxOy := max(0, bufferLines-viewHeight)
	if newOriginY > maxOy {
		newOriginY = maxOy
	}

	err = v.SetOrigin(0, newOriginY)
	if err != nil {
		_ = v.SetOrigin(0, 0)
		if resetScroll {
			app.mutex.Lock()
			app.contentViewOriginY = 0
			app.mutex.Unlock()
		}
	} else {
		if resetScroll {
			app.mutex.Lock()
			app.contentViewOriginY = 0
			app.mutex.Unlock()
		}
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
			return err
		}
		return nil
	})
}

func (app *App) resetStatus(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(StatusViewName)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		if err == gocui.ErrUnknownView {
			return nil
		}

		v.Clear()
		statusText := "?: Help | q: Quit"

		fmt.Fprint(v, statusText)
		v.Rewind()

		return nil
	})
}

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

func (app *App) refreshViews(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		app.refreshFilesView(g)
		return nil
	})
}

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
		if _, err := g.SetCurrentView("error"); err != nil {
			return err
		}
	}
	return nil
}
