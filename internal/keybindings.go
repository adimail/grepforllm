package internal

import "github.com/awesome-gocui/gocui"

func (app *App) SetKeybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, app.ShowCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, app.QuitHandler); err != nil {
		return err
	}
	if err := g.SetKeybinding("", '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, app.SwitchFocus); err != nil {
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
	if err := g.SetKeybinding("", gocui.KeyCtrlF, gocui.ModNone, app.ToggleFilterMode); err != nil {
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
	if err := g.SetKeybinding(FilesViewName, 'y', gocui.ModNone, app.CopyAllSelected); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyEnter, gocui.ModNone, app.FocusContentView); err != nil {
		return err
	}

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

	if err := g.SetKeybinding(HelpViewName, '?', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyEsc, gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'q', gocui.ModNone, app.ToggleHelp); err != nil {
		return err
	}

	for _, vName := range []string{FilterIncludeViewName, FilterExcludeViewName} {
		if err := g.SetKeybinding(vName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil {
			return err
		}
		if err := g.SetKeybinding(vName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil {
			return err
		}
	}

	if err := g.SetKeybinding(CacheViewName, gocui.KeyEsc, gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'q', gocui.ModNone, app.CloseCacheView); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, gocui.KeyCtrlD, gocui.ModNone, app.PromptClearCache); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'y', gocui.ModNone, app.ConfirmClearCache); err != nil {
		return err
	}
	if err := g.SetKeybinding(CacheViewName, 'n', gocui.ModNone, app.CancelClearCache); err != nil {
		return err
	}
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

func (app *App) QuitHandler(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	isCacheViewOpen := app.showCacheView
	awaitingConfirm := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if awaitingConfirm {
		return app.CancelClearCache(g, v)
	}
	if isCacheViewOpen {
		return app.CloseCacheView(g, v)
	}

	return quit(g, v)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
