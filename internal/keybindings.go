package internal

import "github.com/awesome-gocui/gocui"

func (app *App) SetKeybindings(g *gocui.Gui) error {
	// --- Global ---
	// if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil { // Original Ctrl+C quits
	// 	return err
	// }
	// Rebind original Ctrl+C quit to something else if needed, e.g., Ctrl+Q
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
		return err
	}
	// Add Ctrl+C binding to show cache view
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, app.ShowCacheView); err != nil {
		return err
	}

	if err := g.SetKeybinding("", 'q', gocui.ModNone, app.QuitHandler); err != nil { // Use a handler that checks preview/cache view
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, app.SwitchFocus); err != nil {
		return err
	}
	// Global scrolling for main content view
	if err := g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlB, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, app.ScrollContentDown); err != nil {
		return err
	}

	// --- Files View ---
	if err := g.SetKeybinding(FilesViewName, gocui.KeyArrowUp, gocui.ModNone, app.CursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'k', gocui.ModNone, app.CursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyArrowDown, gocui.ModNone, app.CursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'j', gocui.ModNone, app.CursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeySpace, gocui.ModNone, app.ToggleSelect); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'a', gocui.ModNone, app.SelectAllFiles); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'c', gocui.ModNone, app.CopyAllSelected); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'y', gocui.ModNone, app.CopyAllSelected); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyEnter, gocui.ModNone, app.ShowPreview); err != nil {
		return err
	}

	// --- Help View ---
	if err := g.SetKeybinding(HelpViewName, '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyEsc, gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'q', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}

	// --- Filter View ---
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyCtrlF, gocui.ModNone, app.ToggleFilterMode); err != nil {
		return err
	}

	// --- Preview View ---
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyEsc, gocui.ModNone, app.ClosePreview); err != nil {
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, 'q', gocui.ModNone, app.ClosePreview); err != nil {
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyArrowUp, gocui.ModNone, app.ScrollPreviewUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, 'k', gocui.ModNone, app.ScrollPreviewUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyArrowDown, gocui.ModNone, app.ScrollPreviewDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, 'j', gocui.ModNone, app.ScrollPreviewDown); err != nil {
		return err
	}
	// Add PgUp/PgDn for Preview? (Optional)
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyPgup, gocui.ModNone, app.ScrollPreviewPageUp); err != nil { // Assuming ScrollPreviewPageUp exists
		return err
	}
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyPgdn, gocui.ModNone, app.ScrollPreviewPageDown); err != nil { // Assuming ScrollPreviewPageDown exists
		return err
	}

	// --- Cache View ---
	if err := g.SetKeybinding(CacheViewName, gocui.KeyEsc, gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'q', gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyCtrlD, gocui.ModNone, app.PromptClearCache); err != nil {
		return err
	}
	// Bindings for confirmation (only active when awaiting confirmation)
	if err := g.SetKeybinding(CacheViewName, 'y', gocui.ModNone, app.ConfirmClearCache); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'n', gocui.ModNone, app.CancelClearCache); err != nil {
		return err
	}
	// Scrolling for Cache View
	if err := g.SetKeybinding(CacheViewName, gocui.KeyArrowUp, gocui.ModNone, app.ScrollCacheViewUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'k', gocui.ModNone, app.ScrollCacheViewUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyArrowDown, gocui.ModNone, app.ScrollCacheViewDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'j', gocui.ModNone, app.ScrollCacheViewDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyPgup, gocui.ModNone, app.ScrollCacheViewPageUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyPgdn, gocui.ModNone, app.ScrollCacheViewPageDown); err != nil {
		return err
	}

	return nil
}

// QuitHandler checks if preview or cache view is open before quitting
func (app *App) QuitHandler(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	isPreviewOpen := app.isPreviewOpen
	isCacheViewOpen := app.showCacheView
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if awaitingConfirm {
		// If waiting for confirmation, 'q' or 'Esc' should cancel
		return app.CancelClearCache(g, v)
	}
	if isCacheViewOpen {
		return app.CloseCacheView(g, v)
	}
	if isPreviewOpen {
		return app.ClosePreview(g, v)
	}
	// If nothing else is open, 'q' quits the app
	return quit(g, v)
}

// Add scroll handlers for Preview PageUp/PageDown if they don't exist
func (app *App) scrollPreviewPage(g *gocui.Gui, v *gocui.View, direction int) error {
	if v == nil || v.Name() != PreviewViewName {
		return nil
	}
	ox, oy := v.Origin()
	_, vy := v.Size()
	scrollAmount := max(1, vy-1) // Page scroll amount
	newOy := oy + (direction * scrollAmount)
	if newOy < 0 {
		newOy = 0
	}
	if err := v.SetOrigin(ox, newOy); err != nil {
		return err
	}
	app.mutex.Lock()
	app.previewOriginY = newOy
	app.mutex.Unlock()
	return nil
}

func (app *App) ScrollPreviewPageUp(g *gocui.Gui, v *gocui.View) error {
	return app.scrollPreviewPage(g, v, -1)
}

func (app *App) ScrollPreviewPageDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollPreviewPage(g, v, 1)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
