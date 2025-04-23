package internal

import "github.com/awesome-gocui/gocui"

// SetKeybindings configures the application's keybindings.
func (app *App) SetKeybindings(g *gocui.Gui) error {
	// --- Global ---
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, quit); err != nil { // Add 'q' for quit
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	// --- Files View Specific (or when focused) ---
	if err := g.SetKeybinding(FilesViewName, gocui.KeyArrowUp, gocui.ModNone, app.CursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'k', gocui.ModNone, app.CursorUp); err != nil { // Vim-like nav
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyArrowDown, gocui.ModNone, app.CursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'j', gocui.ModNone, app.CursorDown); err != nil { // Vim-like nav
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeySpace, gocui.ModNone, app.ToggleSelect); err != nil {
		return err
	}
	// 'a' selects/deselects all visible files
	if err := g.SetKeybinding(FilesViewName, 'a', gocui.ModNone, app.SelectAllFiles); err != nil {
		return err
	}
	// 'c' copies selected files content
	if err := g.SetKeybinding(FilesViewName, 'c', gocui.ModNone, app.CopyAllSelected); err != nil {
		return err
	}
	// 'f' switches filter mode (Include/Exclude) and refreshes
	if err := g.SetKeybinding(FilesViewName, 'f', gocui.ModNone, app.SwitchFilterModeAndRefresh); err != nil {
		return err
	}

	// --- Content View Scrolling (Global binding for simplicity) ---
	if err := g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlB, gocui.ModNone, app.ScrollContentUp); err != nil { // Vim-like page up
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, app.ScrollContentDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlF, gocui.ModNone, app.ScrollContentDown); err != nil { // Vim-like page down
		return err
	}
	// --- Help View Specific ---
	if err := g.SetKeybinding(HelpViewName, '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyEsc, gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'q', gocui.ModNone, app.ToggleHelp); err != nil { // Allow 'q' to close help
		return err
	}
	// --- Filter View Specific ---
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil {
		return err
	}

	// Global/FilesView: Focus filter input
	// Use Ctrl+E for "Edit filter"
	if err := g.SetKeybinding("", gocui.KeyCtrlE, gocui.ModNone, app.FocusFilterView); err != nil {
		return err
	}
	return nil
}

// quit is a standard gocui handler function to exit the application.
func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
