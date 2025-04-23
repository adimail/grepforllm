package internal

import "github.com/awesome-gocui/gocui"

func (app *App) SetKeybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, app.SwitchFocus); err != nil {
		return err
	}

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
	if err := g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlB, gocui.ModNone, app.ScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, app.ScrollContentDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlF, gocui.ModNone, app.ScrollContentDown); err != nil {
		return err
	}

	if err := g.SetKeybinding(HelpViewName, '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyEsc, gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'q', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}

	if err := g.SetKeybinding(FilterViewName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyCtrlF, gocui.ModNone, app.ToggleFilterMode); err != nil {
		return err
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
