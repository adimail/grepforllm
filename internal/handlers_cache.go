package internal

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/awesome-gocui/gocui"
)

func (app *App) ShowCacheView(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	cachePath := app.cacheFilePath
	cacheCopy := make(AppCache, len(app.cache))
	for k, val := range app.cache {
		cacheCopy[k] = val
	}
	app.mutex.Unlock()

	if cachePath == "" || len(cacheCopy) == 0 {
		app.mutex.Lock()
		app.cacheViewContent = "No cache data available."
		app.showCacheView = true
		app.cacheViewOriginY = 0
		app.cacheCurrentLine = 0
		app.cacheTotalLines = 1
		app.awaitingCacheClearConfirmation = false
		app.mutex.Unlock()
		g.Update(func(g *gocui.Gui) error { return app.Layout(g) })
		return nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%-38s  %-9s  %-24s  %-19s\n", "DIRECTORY", "MODE", "ACTIVE FILTER", "LAST ACCESSED"))
	builder.WriteString(strings.Repeat("-", 96) + "\n")

	paths := make([]string, 0, len(cacheCopy))
	for p := range cacheCopy {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		item := cacheCopy[p]
		modeStr := "Exclude"
		filterStr := item.Excludes
		if item.FilterMode == IncludeMode {
			modeStr = "Include"
			filterStr = item.Includes
		}
		if filterStr == "" {
			filterStr = "(none)"
		}
		if len(filterStr) > 24 {
			filterStr = filterStr[:21] + "..."
		}

		displayPath := p
		if len(displayPath) > 38 {
			displayPath = "..." + displayPath[len(displayPath)-35:]
		}

		timeStr := item.LastOpened.Format("2006-01-02 15:04:05")
		builder.WriteString(fmt.Sprintf("%-38s  %-9s  %-24s  %-19s\n", displayPath, modeStr, filterStr, timeStr))
	}

	contentStr := builder.String()
	totalLines := strings.Count(contentStr, "\n")

	app.mutex.Lock()
	app.cacheViewContent = contentStr
	app.showCacheView = true
	app.cacheViewOriginY = 0
	app.cacheCurrentLine = 2
	app.cacheTotalLines = totalLines
	app.awaitingCacheClearConfirmation = false
	app.mutex.Unlock()

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	return nil
}

func (app *App) CloseCacheView(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showCacheView = false
	app.cacheViewContent = ""
	app.awaitingCacheClearConfirmation = false
	app.mutex.Unlock()

	_ = g.DeleteView(CacheViewName)

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	g.Update(func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(FilesViewName)
		return err
	})

	return nil
}

func (app *App) PromptClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.showCacheView {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheClearConfirmation = true
	app.mutex.Unlock()

	app.updateStatus(g, "CLEAR CACHE? (y/n) - Press 'y' to confirm, 'n' or Esc to cancel.")
	return nil
}

func (app *App) ConfirmClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheClearConfirmation {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheClearConfirmation = false
	cachePath := app.cacheFilePath
	app.mutex.Unlock()

	statusMsg := ""
	newCacheContent := ""

	if cachePath == "" {
		statusMsg = "Error: Cache file path not determined. Cannot clear."
		newCacheContent = "Error: Cache file path not determined."
	} else {
		err := os.Remove(cachePath)
		if err != nil && !os.IsNotExist(err) {
			statusMsg = fmt.Sprintf("Error clearing cache file: %v", err)
			newCacheContent = fmt.Sprintf("Error clearing cache file:\n%v", err)
		} else {
			statusMsg = "Cache cleared successfully."
			newCacheContent = "Cache cleared."
			app.mutex.Lock()
			app.cache = make(AppCache)
			app.cache[app.rootDir] = DirectoryCache{
				Includes:   app.includes,
				Excludes:   app.excludes,
				LastOpened: time.Now(),
				FilterMode: app.filterMode,
			}
			app.mutex.Unlock()
		}
	}

	app.mutex.Lock()
	app.cacheViewContent = newCacheContent
	app.cacheViewOriginY = 0
	app.cacheCurrentLine = 0
	app.cacheTotalLines = 1
	app.mutex.Unlock()

	g.Update(func(g *gocui.Gui) error {
		cv, err := g.View(CacheViewName)
		if err == nil {
			cv.Clear()
			fmt.Fprint(cv, newCacheContent)
			_ = cv.SetOrigin(0, 0)
			_ = cv.SetCursor(0, 0)
		}
		return nil
	})

	app.updateStatus(g, statusMsg)
	go func(msg string) {
		time.Sleep(3 * time.Second)
		g.Update(func(g *gocui.Gui) error {
			sv, err := g.View(StatusViewName)
			app.mutex.Lock()
			showCache := app.showCacheView
			awaitingConfirm := app.awaitingCacheClearConfirmation
			app.mutex.Unlock()
			if err == nil && showCache && !awaitingConfirm && strings.HasPrefix(sv.Buffer(), msg) {
				app.resetStatusForCacheView(g)
			}
			return nil
		})
	}(statusMsg)

	return nil
}

func (app *App) CancelClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheClearConfirmation {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheClearConfirmation = false
	app.mutex.Unlock()

	app.resetStatusForCacheView(g)
	return nil
}

func (app *App) ScrollCacheViewUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != CacheViewName {
		return nil
	}
	app.mutex.Lock()
	if app.cacheCurrentLine > 2 {
		app.cacheCurrentLine--
	}
	curLine := app.cacheCurrentLine
	app.mutex.Unlock()

	ox, oy := v.Origin()
	if curLine < oy {
		_ = v.SetOrigin(ox, curLine)
		app.mutex.Lock()
		app.cacheViewOriginY = curLine
		app.mutex.Unlock()
	}
	_ = v.SetCursor(0, curLine)
	return nil
}

func (app *App) ScrollCacheViewDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != CacheViewName {
		return nil
	}
	app.mutex.Lock()
	if app.cacheCurrentLine < app.cacheTotalLines-1 {
		app.cacheCurrentLine++
	}
	curLine := app.cacheCurrentLine
	app.mutex.Unlock()

	ox, oy := v.Origin()
	_, vy := v.Size()

	if curLine >= oy+vy {
		newOy := curLine - vy + 1
		_ = v.SetOrigin(ox, newOy)
		app.mutex.Lock()
		app.cacheViewOriginY = newOy
		app.mutex.Unlock()
	}
	_ = v.SetCursor(0, curLine)
	return nil
}

func (app *App) ScrollCacheViewPageUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != CacheViewName {
		return nil
	}
	_, vy := v.Size()
	step := max(1, vy-1)

	app.mutex.Lock()
	app.cacheCurrentLine = max(2, app.cacheCurrentLine-step)
	curLine := app.cacheCurrentLine
	app.mutex.Unlock()

	ox, oy := v.Origin()
	newOy := max(0, oy-step)
	_ = v.SetOrigin(ox, newOy)
	app.mutex.Lock()
	app.cacheViewOriginY = newOy
	app.mutex.Unlock()

	_ = v.SetCursor(0, curLine)
	return nil
}

func (app *App) ScrollCacheViewPageDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() != CacheViewName {
		return nil
	}
	_, vy := v.Size()
	step := max(1, vy-1)

	app.mutex.Lock()
	app.cacheCurrentLine = min(max(2, app.cacheTotalLines-1), app.cacheCurrentLine+step)
	curLine := app.cacheCurrentLine
	app.mutex.Unlock()

	ox, oy := v.Origin()
	newOy := oy + step
	_ = v.SetOrigin(ox, newOy)
	app.mutex.Lock()
	app.cacheViewOriginY = newOy
	app.mutex.Unlock()

	_ = v.SetCursor(0, curLine)
	return nil
}
