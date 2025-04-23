package internal

import "github.com/awesome-gocui/gocui"

func (app *App) SetKeybindings(g *gocui.Gui) error {
	// --- Global ---
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, app.QuitHandler); err != nil { // Use a handler that checks preview
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
	if err := g.SetKeybinding(FilesViewName, gocui.KeyEnter, gocui.ModNone, app.ShowPreview); err != nil { // <-- Add Enter binding
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
	if err := g.SetKeybinding(PreviewViewName, gocui.KeyEsc, gocui.ModNone, app.ClosePreview); err != nil { // <-- Add Preview bindings
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

	return nil
}

// QuitHandler checks if preview is open before quitting
func (app *App) QuitHandler(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	isPreviewOpen := app.isPreviewOpen
	app.mutex.Unlock()

	if isPreviewOpen {
		return app.ClosePreview(g, v) // v might be nil here, ClosePreview handles it
	}
	// If preview is not open, 'q' quits the app
	return quit(g, v)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
