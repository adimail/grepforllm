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
	paths := make([]string, 0, len(app.cache))
	for p := range app.cache {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		return app.cache[paths[i]].LastOpened.After(app.cache[paths[j]].LastOpened)
	})

	app.cachePaths = paths
	app.selectedCacheEntries = make(map[string]bool)
	app.cacheCurrentLine = 0
	app.cacheViewOriginY = 0
	app.showCacheView = true
	app.awaitingCacheClearConfirmation = false
	app.awaitingCacheDeleteConfirmation = false
	app.mutex.Unlock()

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	return nil
}

func (app *App) CloseCacheView(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showCacheView = false
	app.selectedCacheEntries = make(map[string]bool)
	app.awaitingCacheClearConfirmation = false
	app.awaitingCacheDeleteConfirmation = false
	app.mutex.Unlock()

	_ = g.DeleteView(CacheHeaderViewName)
	_ = g.DeleteView(CacheListViewName)
	_ = g.DeleteView(CacheDetailViewName)

	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	g.Update(func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(FilesViewName)
		return err
	})

	return nil
}

func (app *App) refreshCacheList(g *gocui.Gui) {
	v, err := g.View(CacheListViewName)
	if err != nil {
		return
	}
	v.Clear()

	app.mutex.Lock()
	paths := app.cachePaths
	curLine := app.cacheCurrentLine
	selected := app.selectedCacheEntries
	originY := app.cacheViewOriginY
	cacheData := app.cache
	app.mutex.Unlock()

	if len(paths) == 0 {
		fmt.Fprintln(v, " (No saved projects in cache)")
		return
	}

	for i, path := range paths {
		prefix := "[ ]"
		if selected[path] {
			prefix = "[*]"
		}

		item := cacheData[path]
		relTime := formatRelativeTime(item.LastOpened)

		displayPath := path
		if len(displayPath) > 28 {
			displayPath = "..." + displayPath[len(displayPath)-25:]
		}

		cursorPrefix := "  "
		if i == curLine {
			cursorPrefix = "▶ "
		}

		line := fmt.Sprintf("%s%s %-28s %8s", cursorPrefix, prefix, displayPath, relTime)

		if i == curLine {
			fmt.Fprintf(v, "\x1b[7m%s\x1b[0m\n", line)
		} else if selected[path] {
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line)
		} else {
			fmt.Fprintln(v, line)
		}
	}

	_, vy := v.Size()
	localCursorY := curLine - originY
	if localCursorY < 0 {
		localCursorY = 0
	}
	if localCursorY >= vy && vy > 0 {
		localCursorY = vy - 1
	}
	_ = v.SetCursor(0, localCursorY)
}

func (app *App) refreshCacheDetail(g *gocui.Gui) {
	v, err := g.View(CacheDetailViewName)
	if err != nil {
		return
	}
	v.Clear()

	app.mutex.Lock()
	paths := app.cachePaths
	curLine := app.cacheCurrentLine
	var item DirectoryCache
	var path string
	hasItem := false
	if len(paths) > 0 && curLine >= 0 && curLine < len(paths) {
		path = paths[curLine]
		item = app.cache[path]
		hasItem = true
	}
	app.mutex.Unlock()

	if !hasItem {
		fmt.Fprintln(v, "No directory selected.")
		return
	}

	modeStr := "Exclude"
	if item.FilterMode == IncludeMode {
		modeStr = "Include"
	}

	sortStr := "Name"
	switch item.SortMode {
	case SortBySize:
		sortStr = "Size"
	case SortByTokens:
		sortStr = "Tokens"
	case SortByModTime:
		sortStr = "Modified Date"
	}

	gitStr := "All files"
	switch item.GitFilterMode {
	case GitFilterChanged:
		gitStr = "Changed (Untracked + Modified)"
	case GitFilterNew:
		gitStr = "New (Untracked only)"
	case GitFilterUpdated:
		gitStr = "Updated (Modified only)"
	}

	searchQuery := item.SearchQuery
	if searchQuery == "" {
		searchQuery = "(none)"
	}

	activeFilter := item.Excludes
	if item.FilterMode == IncludeMode {
		activeFilter = item.Includes
	}
	if activeFilter == "" {
		activeFilter = "(none)"
	}

	fmt.Fprintf(v, "\x1b[1mDirectory:\x1b[0m\n  %s\n\n", path)
	fmt.Fprintf(v, "\x1b[1mLast Accessed:\x1b[0m\n  %s (%s)\n\n", item.LastOpened.Format("2006-01-02 15:04:05"), formatRelativeTime(item.LastOpened))
	fmt.Fprintf(v, "\x1b[1mFilter Mode:\x1b[0m       %s\n", modeStr)
	fmt.Fprintf(v, "\x1b[1mActive Filter:\x1b[0m     %s\n", activeFilter)
	fmt.Fprintf(v, "\x1b[1mSearch Query:\x1b[0m      %s\n", searchQuery)
	fmt.Fprintf(v, "\x1b[1mSort Order:\x1b[0m        %s\n", sortStr)
	fmt.Fprintf(v, "\x1b[1mGit Filter:\x1b[0m        %s\n\n", gitStr)
	fmt.Fprintf(v, "\x1b[1mSaved Includes:\x1b[0m\n")
	if item.Includes == "" {
		fmt.Fprintf(v, "  (none)\n")
	} else {
		for _, inc := range strings.Split(item.Includes, ",") {
			inc = strings.TrimSpace(inc)
			if inc != "" {
				fmt.Fprintf(v, "  • %s\n", inc)
			}
		}
	}

	fmt.Fprintf(v, "\n\x1b[1mSaved Excludes:\x1b[0m\n")
	if item.Excludes == "" {
		fmt.Fprintf(v, "  (none)\n")
	} else {
		for _, exc := range strings.Split(item.Excludes, ",") {
			exc = strings.TrimSpace(exc)
			if exc != "" {
				fmt.Fprintf(v, "  • %s\n", exc)
			}
		}
	}
}

func (app *App) adjustCacheListScroll(v *gocui.View) {
	app.mutex.Lock()
	curLine := app.cacheCurrentLine
	total := len(app.cachePaths)
	app.mutex.Unlock()

	if v == nil || total == 0 {
		return
	}

	ox, oy := v.Origin()
	_, vy := v.Size()

	if curLine < oy {
		oy = curLine
		_ = v.SetOrigin(ox, oy)
	} else if curLine >= oy+vy {
		oy = curLine - vy + 1
		_ = v.SetOrigin(ox, oy)
	}

	app.mutex.Lock()
	app.cacheViewOriginY = oy
	app.mutex.Unlock()
}

func (app *App) ScrollCacheViewUp(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if app.cacheCurrentLine > 0 {
		app.cacheCurrentLine--
	}
	app.mutex.Unlock()

	lv, err := g.View(CacheListViewName)
	if err == nil {
		app.adjustCacheListScroll(lv)
	}

	app.refreshCacheList(g)
	app.refreshCacheDetail(g)
	return nil
}

func (app *App) ScrollCacheViewDown(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if app.cacheCurrentLine < len(app.cachePaths)-1 {
		app.cacheCurrentLine++
	}
	app.mutex.Unlock()

	lv, err := g.View(CacheListViewName)
	if err == nil {
		app.adjustCacheListScroll(lv)
	}

	app.refreshCacheList(g)
	app.refreshCacheDetail(g)
	return nil
}

func (app *App) ScrollCacheViewPageUp(g *gocui.Gui, v *gocui.View) error {
	lv, _ := g.View(CacheListViewName)
	step := 5
	if lv != nil {
		_, vy := lv.Size()
		step = max(1, vy-1)
	}

	app.mutex.Lock()
	app.cacheCurrentLine = max(0, app.cacheCurrentLine-step)
	app.mutex.Unlock()

	if lv != nil {
		app.adjustCacheListScroll(lv)
	}

	app.refreshCacheList(g)
	app.refreshCacheDetail(g)
	return nil
}

func (app *App) ScrollCacheViewPageDown(g *gocui.Gui, v *gocui.View) error {
	lv, _ := g.View(CacheListViewName)
	step := 5
	if lv != nil {
		_, vy := lv.Size()
		step = max(1, vy-1)
	}

	app.mutex.Lock()
	if len(app.cachePaths) > 0 {
		app.cacheCurrentLine = min(len(app.cachePaths)-1, app.cacheCurrentLine+step)
	}
	app.mutex.Unlock()

	if lv != nil {
		app.adjustCacheListScroll(lv)
	}

	app.refreshCacheList(g)
	app.refreshCacheDetail(g)
	return nil
}

func (app *App) ToggleCacheSelect(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if len(app.cachePaths) > 0 && app.cacheCurrentLine >= 0 && app.cacheCurrentLine < len(app.cachePaths) {
		path := app.cachePaths[app.cacheCurrentLine]
		if app.selectedCacheEntries[path] {
			delete(app.selectedCacheEntries, path)
		} else {
			app.selectedCacheEntries[path] = true
		}
	}
	app.mutex.Unlock()

	app.refreshCacheList(g)
	return nil
}

func (app *App) SelectAllCacheEntries(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	allSelected := len(app.selectedCacheEntries) == len(app.cachePaths) && len(app.cachePaths) > 0
	if allSelected {
		app.selectedCacheEntries = make(map[string]bool)
	} else {
		for _, p := range app.cachePaths {
			app.selectedCacheEntries[p] = true
		}
	}
	app.mutex.Unlock()

	app.refreshCacheList(g)
	return nil
}

func (app *App) PromptDeleteCacheEntries(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if len(app.cachePaths) == 0 {
		app.mutex.Unlock()
		return nil
	}

	count := len(app.selectedCacheEntries)
	if count == 0 {
		count = 1
	}
	app.awaitingCacheDeleteConfirmation = true
	app.awaitingCacheClearConfirmation = false
	app.mutex.Unlock()

	app.updateStatus(g, fmt.Sprintf("DELETE %d entry(ies) from cache? (y/n) - Press 'y' to confirm, 'n' to cancel.", count))
	return nil
}

func (app *App) ConfirmDeleteCacheEntries(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheDeleteConfirmation {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheDeleteConfirmation = false

	toDelete := make([]string, 0)
	if len(app.selectedCacheEntries) > 0 {
		for p := range app.selectedCacheEntries {
			toDelete = append(toDelete, p)
		}
	} else if len(app.cachePaths) > 0 && app.cacheCurrentLine < len(app.cachePaths) {
		toDelete = append(toDelete, app.cachePaths[app.cacheCurrentLine])
	}

	for _, p := range toDelete {
		delete(app.cache, p)
	}
	_ = saveCache(app.cacheFilePath, app.cache)

	newPaths := make([]string, 0, len(app.cache))
	for p := range app.cache {
		newPaths = append(newPaths, p)
	}
	sort.Slice(newPaths, func(i, j int) bool {
		return app.cache[newPaths[i]].LastOpened.After(app.cache[newPaths[j]].LastOpened)
	})

	app.cachePaths = newPaths
	app.selectedCacheEntries = make(map[string]bool)
	if app.cacheCurrentLine >= len(newPaths) {
		app.cacheCurrentLine = max(0, len(newPaths)-1)
	}
	app.cacheViewOriginY = 0
	app.mutex.Unlock()

	app.updateStatus(g, fmt.Sprintf("Removed %d project cache entry(ies).", len(toDelete)))
	app.refreshCacheList(g)
	app.refreshCacheDetail(g)

	time.AfterFunc(2*time.Second, func() {
		app.mutex.Lock()
		showCache := app.showCacheView
		awaitingDel := app.awaitingCacheDeleteConfirmation
		awaitingClr := app.awaitingCacheClearConfirmation
		app.mutex.Unlock()
		if showCache && !awaitingDel && !awaitingClr {
			app.resetStatusForCacheView(g)
		}
	})

	return nil
}

func (app *App) CancelDeleteCacheEntries(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheDeleteConfirmation {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheDeleteConfirmation = false
	app.mutex.Unlock()

	app.resetStatusForCacheView(g)
	return nil
}

func (app *App) PromptClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.showCacheView {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheClearConfirmation = true
	app.awaitingCacheDeleteConfirmation = false
	app.mutex.Unlock()

	app.updateStatus(g, "CLEAR ENTIRE CACHE HISTORY? (y/n) - Press 'y' to confirm, 'n' or Esc to cancel.")
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
	if cachePath != "" {
		_ = os.Remove(cachePath)
		statusMsg = "All cache history cleared."
		app.mutex.Lock()
		app.cache = make(AppCache)
		app.cache[app.rootDir] = DirectoryCache{
			Includes:      app.includes,
			Excludes:      app.excludes,
			SearchQuery:   app.searchQuery,
			LastOpened:    time.Now(),
			FilterMode:    app.filterMode,
			SortMode:      app.sortMode,
			GitFilterMode: app.gitFilterMode,
		}
		app.cachePaths = []string{app.rootDir}
		app.selectedCacheEntries = make(map[string]bool)
		app.cacheCurrentLine = 0
		app.cacheViewOriginY = 0
		app.mutex.Unlock()
	}

	app.updateStatus(g, statusMsg)
	app.refreshCacheList(g)
	app.refreshCacheDetail(g)

	time.AfterFunc(2*time.Second, func() {
		app.mutex.Lock()
		showCache := app.showCacheView
		awaitingClr := app.awaitingCacheClearConfirmation
		awaitingDel := app.awaitingCacheDeleteConfirmation
		app.mutex.Unlock()
		if showCache && !awaitingClr && !awaitingDel {
			app.resetStatusForCacheView(g)
		}
	})

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

func (app *App) OnCacheListMouseClick(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	_, cy := v.Cursor()
	_, oy := v.Origin()
	clickedIndex := oy + cy

	app.mutex.Lock()
	if clickedIndex >= 0 && clickedIndex < len(app.cachePaths) {
		app.cacheCurrentLine = clickedIndex
	}
	app.mutex.Unlock()

	app.refreshCacheList(g)
	app.refreshCacheDetail(g)
	return nil
}

func (app *App) OnCacheMouseWheelUp(g *gocui.Gui, v *gocui.View) error {
	return app.ScrollCacheViewUp(g, v)
}

func (app *App) OnCacheMouseWheelDown(g *gocui.Gui, v *gocui.View) error {
	return app.ScrollCacheViewDown(g, v)
}
