package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/awesome-gocui/gocui"
)

func (app *App) ShowCacheView(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	cachePath := app.cacheFilePath
	app.mutex.Unlock()

	if cachePath == "" {
		app.mutex.Lock()
		app.cacheViewContent = "Error: Cache file path not determined."
		app.showCacheView = true
		app.cacheViewOriginY = 0
		app.awaitingCacheClearConfirmation = false // Ensure confirmation state is reset
		app.mutex.Unlock()
		// Trigger layout update
		g.Update(func(g *gocui.Gui) error { return app.Layout(g) })
		return nil
	}

	contentBytes, err := os.ReadFile(cachePath)
	var displayContent string

	if err != nil {
		if os.IsNotExist(err) {
			displayContent = "No cache data available (file does not exist)."
		} else {
			displayContent = fmt.Sprintf("Error reading cache file:\n%v", err)
		}
	} else if len(contentBytes) == 0 {
		displayContent = "No cache data available (file is empty)."
	} else {
		// Try to pretty-print the JSON
		prettyJSON, jsonErr := prettyPrintJSON(contentBytes)
		if jsonErr != nil {
			// If pretty-printing fails, show raw content with a warning
			displayContent = fmt.Sprintf("Cache file content (not valid JSON?):\n%s", string(contentBytes))
		} else {
			displayContent = prettyJSON
		}
	}

	app.mutex.Lock()
	app.cacheViewContent = displayContent
	app.showCacheView = true
	app.cacheViewOriginY = 0
	app.awaitingCacheClearConfirmation = false // Ensure confirmation state is reset
	app.mutex.Unlock()

	// Trigger layout update which will create/show the cache view
	g.Update(func(g *gocui.Gui) error {
		return app.Layout(g)
	})

	return nil
}

// CloseCacheView hides the cache view and returns to the normal file browser UI.
func (app *App) CloseCacheView(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	app.showCacheView = false
	app.cacheViewContent = ""                  // Clear content
	app.awaitingCacheClearConfirmation = false // Reset confirmation state
	app.mutex.Unlock()

	// Delete the cache view
	if err := g.DeleteView(CacheViewName); err != nil && err != gocui.ErrUnknownView {
		// Log or handle error if needed
	}

	// Trigger layout update to redraw the normal UI
	g.Update(func(g *gocui.Gui) error {
		// Layout will handle recreating normal views and setting focus
		return app.Layout(g)
	})

	// Explicitly set focus back to FilesView after layout update
	g.Update(func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(FilesViewName)
		return err
	})

	return nil
}

// PromptClearCache asks the user for confirmation before clearing the cache.
func (app *App) PromptClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	// Only proceed if the cache view is actually showing
	if !app.showCacheView {
		app.mutex.Unlock()
		return nil
	}
	app.awaitingCacheClearConfirmation = true
	app.mutex.Unlock()

	app.updateStatus(g, "CLEAR CACHE? (y/n) - Press 'y' to confirm, 'n' or Esc to cancel.")
	return nil
}

// ConfirmClearCache deletes the cache file and clears the in-memory cache.
func (app *App) ConfirmClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheClearConfirmation {
		app.mutex.Unlock()
		return nil // Only proceed if confirmation was requested
	}
	app.awaitingCacheClearConfirmation = false // Reset confirmation state
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
			// Report error only if it's not "file already gone"
			statusMsg = fmt.Sprintf("Error clearing cache file: %v", err)
			newCacheContent = fmt.Sprintf("Error clearing cache file:\n%v", err)
		} else {
			statusMsg = "Cache cleared successfully."
			newCacheContent = "Cache cleared."
			// Also clear the in-memory cache
			app.mutex.Lock()
			app.cache = make(AppCache)
			// Re-add entry for the current directory with current settings
			app.cache[app.rootDir] = DirectoryCache{
				Includes:   app.includes,
				Excludes:   app.excludes,
				LastOpened: time.Now(),
				FilterMode: app.filterMode,
			}
			// No need to save here, as the file is gone. It will be recreated on next save.
			app.mutex.Unlock()
		}
	}

	// Update the cache view content immediately
	app.mutex.Lock()
	app.cacheViewContent = newCacheContent
	app.cacheViewOriginY = 0 // Reset scroll
	app.mutex.Unlock()

	// Refresh the cache view in the UI thread
	g.Update(func(g *gocui.Gui) error {
		cv, err := g.View(CacheViewName)
		if err == nil {
			cv.Clear()
			fmt.Fprint(cv, newCacheContent)
			_ = cv.SetOrigin(0, 0)
		}
		return nil
	})

	// Update status bar and reset it after a delay
	app.updateStatus(g, statusMsg)
	go func(msg string) {
		time.Sleep(3 * time.Second)
		g.Update(func(g *gocui.Gui) error {
			sv, err := g.View(StatusViewName)
			// Only reset if the status message hasn't changed
			// And if we are still in cache view mode
			app.mutex.Lock()
			showCache := app.showCacheView
			awaitingConfirm := app.awaitingCacheClearConfirmation
			app.mutex.Unlock()
			if err == nil && showCache && !awaitingConfirm && strings.HasPrefix(sv.Buffer(), msg) {
				app.resetStatusForCacheView(g) // Use cache view specific reset
			}
			return nil
		})
	}(statusMsg)

	return nil
}

// CancelClearCache cancels the cache clearing process.
func (app *App) CancelClearCache(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	if !app.awaitingCacheClearConfirmation {
		app.mutex.Unlock()
		return nil // Only proceed if confirmation was requested
	}
	app.awaitingCacheClearConfirmation = false // Reset confirmation state
	app.mutex.Unlock()

	app.resetStatusForCacheView(g) // Reset status to cache view default
	return nil
}

// scrollCacheView handles scrolling within the cache view.
func (app *App) scrollCacheView(g *gocui.Gui, v *gocui.View, direction int) error {
	if v == nil || v.Name() != CacheViewName {
		return nil // Only scroll the cache view
	}
	ox, oy := v.Origin()
	_, vy := v.Size() // Get view height for page scrolling

	scrollAmount := 1    // Default scroll by 1 line
	if direction == -2 { // Page Up
		scrollAmount = max(1, vy-1)
		direction = -1
	} else if direction == 2 { // Page Down
		scrollAmount = max(1, vy-1)
		direction = 1
	}

	newOy := oy + (direction * scrollAmount)
	if newOy < 0 {
		newOy = 0
	}

	// Set the view's origin
	if err := v.SetOrigin(ox, newOy); err != nil {
		return err
	}

	// Update the stored origin in app state
	app.mutex.Lock()
	app.cacheViewOriginY = newOy
	app.mutex.Unlock()

	return nil
}

// ScrollCacheViewUp scrolls the cache view content up.
func (app *App) ScrollCacheViewUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollCacheView(g, v, -1) // Scroll up by 1 line
}

// ScrollCacheViewDown scrolls the cache view content down.
func (app *App) ScrollCacheViewDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollCacheView(g, v, 1) // Scroll down by 1 line
}

// ScrollCacheViewPageUp scrolls the cache view content up by a page.
func (app *App) ScrollCacheViewPageUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollCacheView(g, v, -2) // Use -2 for Page Up signal
}

// ScrollCacheViewPageDown scrolls the cache view content down by a page.
func (app *App) ScrollCacheViewPageDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollCacheView(g, v, 2) // Use 2 for Page Down signal
}
