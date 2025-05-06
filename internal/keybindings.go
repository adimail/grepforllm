package internal

import "github.com/awesome-gocui/gocui"

func (app *App) SetKeybindings(g *gocui.Gui) error {
	// --- Global ---
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil { // Force Quit
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, app.ShowCacheView); err != nil { // Show Cache
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, app.QuitHandler); err != nil { // Normal Quit / Close View
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.ToggleHelp); err != nil { // Toggle Help
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, app.SwitchFocus); err != nil { // Switch Focus (Files <-> Filter <-> Content)
		return err
	}
	// Global scrolling for main content view (Page Up/Down) - Works regardless of focus (unless filter editable)
	if err := g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlB, gocui.ModNone, app.ScrollContentUp); err != nil { // Alternative Page Up
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, app.ScrollContentDown); err != nil {
		return err
	}
	// Note: Ctrl+F for filter mode toggle is bound to FilterViewName below

	// --- Files View (FilesViewName) ---
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
	if err := g.SetKeybinding(FilesViewName, 'y', gocui.ModNone, app.CopyAllSelected); err != nil { // Alternative copy
		return err
	}
	// ENTER KEY: Focus the content view for scrolling
	if err := g.SetKeybinding(FilesViewName, gocui.KeyEnter, gocui.ModNone, app.FocusContentView); err != nil {
		return err
	}

	// --- Content View (ContentViewName) ---
	// Line scrolling only when content view is focused
	if err := g.SetKeybinding(ContentViewName, gocui.KeyArrowUp, gocui.ModNone, app.ScrollContentLineUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, 'k', gocui.ModNone, app.ScrollContentLineUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, gocui.KeyArrowDown, gocui.ModNone, app.ScrollContentLineDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, 'j', gocui.ModNone, app.ScrollContentLineDown); err != nil {
		return err
	}
	// Page scrolling (PgUp/PgDn/Ctrl+B) is handled by global bindings already.
	// Optional: Add Esc binding to return focus to FilesView?
	// if err := g.SetKeybinding(ContentViewName, gocui.KeyEsc, gocui.ModNone, app.FocusFilesView); err != nil { // Requires FocusFilesView handler
	// 	return err
	// }

	// --- Help View (HelpViewName) ---
	if err := g.SetKeybinding(HelpViewName, '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyEsc, gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'q', gocui.ModNone, app.ToggleHelp); err != nil { // 'q' also closes help
		return err
	}

	// --- Filter View (FilterViewName) ---
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil { // Apply filter
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil { // Cancel filter input
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyCtrlF, gocui.ModNone, app.ToggleFilterMode); err != nil { // Toggle Include/Exclude
		return err
	}

	// --- Cache View (CacheViewName) ---
	if err := g.SetKeybinding(CacheViewName, gocui.KeyEsc, gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'q', gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyCtrlD, gocui.ModNone, app.PromptClearCache); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'y', gocui.ModNone, app.ConfirmClearCache); err != nil { // Confirm clear
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'n', gocui.ModNone, app.CancelClearCache); err != nil { // Cancel clear
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

// QuitHandler checks if cache view is open before quitting
func (app *App) QuitHandler(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	isCacheViewOpen := app.showCacheView
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if awaitingConfirm {
		// If waiting for cache clear confirmation, 'q' or 'Esc' should cancel
		return app.CancelClearCache(g, v) // Assuming Esc is also bound to CancelClearCache or QuitHandler
	}
	if isCacheViewOpen {
		return app.CloseCacheView(g, v)
	}

	// If nothing else is open/active, 'q' quits the app
	return quit(g, v)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
