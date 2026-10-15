package internal

import (
	"fmt"
	"io"
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

	filesWidth := maxX / 3
	pathHeight := 2
	filterHeight := 3
	statusBarHeight := 2

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
		v.Highlight = true
		v.SelBgColor = gocui.ColorDefault
		v.SelFgColor = gocui.ColorCyan | gocui.AttrBold
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

	if v, err := g.SetView(FilterViewName, 0, filterY0, filesWidth, filterY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		v.Wrap = false
		v.Editor = gocui.DefaultEditor
		v.FgColor = gocui.ColorCyan
	}

	app.mutex.Lock()
	modeStr := "Exclude"
	currentFilterMode := app.filterMode
	currentExcludes := app.excludes
	currentIncludes := app.includes
	app.mutex.Unlock()

	filterV, _ := g.View(FilterViewName)
	if filterV != nil {
		filterV.Title = fmt.Sprintf(" Filter: %s (Ctrl+F: Mode) ", modeStr)
		isFilterViewFocused := (currentViewName == FilterViewName)

		if isFilterViewFocused {
			filterV.FrameColor = gocui.ColorGreen
			filterV.FgColor = gocui.ColorWhite | gocui.AttrBold

			if !filterV.Editable {
				filterV.Editable = true
				var currentValueText string
				if currentFilterMode == ExcludeMode {
					currentValueText = currentExcludes
				} else {
					currentValueText = currentIncludes
				}

				filterV.Clear()
				fmt.Fprint(filterV, currentValueText)
				cursorPos := len(currentValueText)
				_ = filterV.SetCursor(cursorPos, 0)

				maxXSize, _ := filterV.Size()
				ox, _ := filterV.Origin()
				if cursorPos < ox {
					_ = filterV.SetOrigin(cursorPos, 0)
				} else if cursorPos >= ox+maxXSize {
					_ = filterV.SetOrigin(cursorPos-maxXSize+1, 0)
				}
			}
		} else {
			filterV.FrameColor = gocui.ColorBlue
			filterV.FgColor = gocui.ColorCyan
			if filterV.Editable {
				filterV.Editable = false
			}
			app.updateFilterViewContent(g)
		}
	}

	contentX0 := filesWidth + 1
	contentY0 := 0
	if v, err := g.SetView(ContentViewName, contentX0, contentY0, maxX-1, contentViewY1, 0); err != nil {
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

	if v, err := g.SetView(StatusViewName, 0, statusBarY0, maxX-1, statusBarY1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		v.FgColor = gocui.ColorWhite
		v.BgColor = gocui.ColorDefault
	}
	app.mutex.Lock()
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()
	if !awaitingConfirm {
		app.resetStatus(g)
	}

	return nil
}

func (app *App) layoutHelpView(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	helpWidth := maxX * 3 / 4
	helpHeight := maxY * 3 / 4
	x0, y0 := (maxX-helpWidth)/2, (maxY-helpHeight)/2
	x1, y1 := x0+helpWidth-1, y0+helpHeight-1

	if v, err := g.SetView(HelpViewName, x0, y0, x1, y1, gocui.TOP); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Help (↑/↓/PgUp/PgDn to Scroll | ?/Esc/q to Close) "
		v.Wrap = true
		v.Editable = false
		v.Autoscroll = false
		v.Frame = true
		v.FgColor = gocui.ColorWhite

		v.Clear()
		fmt.Fprintln(v, "grepforllm - Select & Copy File Contents")
		fmt.Fprintln(v, "----------------------------------------------------")
		fmt.Fprintln(v, "General:")
		fmt.Fprintln(v, "  Tab                 : Switch focus Files <-> Filter <-> Content")
		fmt.Fprintln(v, "  ?                   : Toggle this help message")
		fmt.Fprintln(v, "  Ctrl+C              : Show Cache View")
		fmt.Fprintln(v, "  q                   : Quit / Close Help / Close Cache")
		fmt.Fprintln(v, "  Ctrl+Q              : Force Quit Application")
		fmt.Fprintln(v, "\nFiles View (Left):")
		fmt.Fprintln(v, "  ↑ / k               : Move cursor up")
		fmt.Fprintln(v, "  ↓ / j               : Move cursor down")
		fmt.Fprintln(v, "  J / K               : Scroll Content preview DOWN / UP directly")
		fmt.Fprintln(v, "  Ctrl+J / Ctrl+K     : Scroll Content preview DOWN / UP directly")
		fmt.Fprintln(v, "  Enter               : Focus Content View for line scrolling")
		fmt.Fprintln(v, "  Space               : Toggle select file under cursor")
		fmt.Fprintln(v, "  a                   : Select / Deselect all visible files")
		fmt.Fprintln(v, "  c / y               : Copy contents of selected files to clipboard")
		fmt.Fprintln(v, "\nContent View (Right):")
		fmt.Fprintln(v, "  ↑ / k               : Scroll content UP one line (when focused)")
		fmt.Fprintln(v, "  ↓ / j               : Scroll content DOWN one line (when focused)")
		fmt.Fprintln(v, "  PgUp / Ctrl+B       : Scroll content UP one page (works globally)")
		fmt.Fprintln(v, "  PgDn                : Scroll content DOWN one page (works globally)")
		fmt.Fprintln(v, "\nFilter View (Bottom-Left):")
		fmt.Fprintln(v, "  (Type patterns: *.go, cmd/, file.txt)")
		fmt.Fprintln(v, "  Enter               : Apply filter & return focus to Files")
		fmt.Fprintln(v, "  Esc                 : Cancel input & return focus to Files")
		fmt.Fprintln(v, "  Ctrl+F              : Toggle filter mode (Include/Exclude)")
		fmt.Fprintln(v, "\nCache View (Ctrl+C):")
		fmt.Fprintln(v, "  ↑ / k / ↓ / j       : Scroll Line")
		fmt.Fprintln(v, "  PgUp / PgDn         : Scroll Page")
		fmt.Fprintln(v, "  Ctrl+D              : Prompt to clear cache")
		fmt.Fprintln(v, "  y / n               : Confirm / Cancel cache clear")
		fmt.Fprintln(v, "  Esc / q             : Close Cache View")

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
		FilesViewName, ContentViewName, FilterViewName, PathViewName,
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
		cv.Title = " Project Cache History "
		cv.Editable = false
		cv.Wrap = false
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

	currentFileList := make([]FileItem, totalCount)
	copy(currentFileList, app.fileList)
	currentSelectedFiles := make(map[string]bool, selectedCount)
	for k, val := range app.selectedFiles {
		currentSelectedFiles[k] = val
	}
	currentLine := app.currentLine
	isCopyHighlightActive := app.isCopyHighlightActive
	app.mutex.Unlock()

	var selectedTokens int
	var selectedBytes int64
	for _, file := range currentFileList {
		if currentSelectedFiles[file.RelPath] {
			selectedTokens += file.EstimatedTokens
			selectedBytes += file.SizeBytes
		}
	}

	title := fmt.Sprintf(" Files (%d/%d Sel | ~%s tok | %s) %s [?] Help ",
		selectedCount, totalCount, formatTokens(selectedTokens), formatBytes(selectedBytes), modeStr)
	v.Title = title

	for i, file := range currentFileList {
		isSelected := currentSelectedFiles[file.RelPath]
		isCurrent := (i == currentLine)
		prefix := "[ ]"
		if isSelected {
			prefix = "[*]"
		}
		line := fmt.Sprintf("%s %s (%s)", prefix, file.RelPath, formatTokens(file.EstimatedTokens))

		switch {
		case isCopyHighlightActive && isSelected:
			fmt.Fprintf(v, "\x1b[7m%s\x1b[0m\n", line)
		case isCurrent:
			fmt.Fprintln(v, line)
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
	var currentFileItem FileItem
	hasFile := false
	if fileListLen > 0 && currentLine >= 0 && currentLine < fileListLen {
		currentFileItem = app.fileList[currentLine]
		hasFile = true
	}
	rootDir := app.rootDir
	previousPreviewedFile := app.currentlyPreviewedFile
	currentContentOriginY := app.contentViewOriginY
	app.mutex.Unlock()

	resetScroll := (hasFile && currentFileItem.RelPath != previousPreviewedFile)
	newOriginY := currentContentOriginY
	if resetScroll {
		newOriginY = 0
	}

	v.Clear()

	if !hasFile {
		v.Title = " Content - PgUp/PgDn Scroll "
		fmt.Fprintln(v, "\nNo file selected or list is empty.")
		fmt.Fprintln(v, "\nUse ↑ / ↓ to navigate files.")
		fmt.Fprintln(v, "[J] / [K] : Scroll preview directly")
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

	fullPath := filepath.Join(rootDir, currentFileItem.RelPath)
	const previewLimit = 256 * 1024

	file, readErr := os.Open(fullPath)
	var fileContentBytes []byte
	isTruncated := false

	if readErr == nil {
		buffer := make([]byte, previewLimit)
		n, err := io.ReadFull(file, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			readErr = err
		} else {
			fileContentBytes = buffer[:n]
			if currentFileItem.SizeBytes > int64(n) {
				isTruncated = true
			}
		}
		_ = file.Close()
	}

	v.Title = fmt.Sprintf(" Content: %s (%s, ~%s tok) - PgUp/PgDn / J/K Scroll ",
		currentFileItem.RelPath, formatBytes(currentFileItem.SizeBytes), formatTokens(currentFileItem.EstimatedTokens))

	if readErr != nil {
		fmt.Fprintf(v, "\n!!! ERROR READING FILE: %v !!!\n", readErr)
	} else if len(fileContentBytes) == 0 {
		fmt.Fprintln(v, "(Empty File)")
	} else if !isLikelyText(fileContentBytes) {
		fmt.Fprintf(v, "(Binary File: %s)", currentFileItem.RelPath)
	} else {
		fmt.Fprint(v, string(fileContentBytes))
		if isTruncated {
			fmt.Fprintf(v, "\n\n--- [Preview Truncated: File is %s, showing first %s] ---\n",
				formatBytes(currentFileItem.SizeBytes), formatBytes(previewLimit))
		}
	}

	app.mutex.Lock()
	app.currentlyPreviewedFile = currentFileItem.RelPath
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
