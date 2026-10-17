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
	if err := g.SetKeybinding(FilesViewName, '/', gocui.ModNone, app.StartSearch); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 's', gocui.ModNone, app.CycleSortMode); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'f', gocui.ModNone, app.CycleGitFilterMode); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'J', gocui.ModNone, app.DirectScrollContentDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, 'K', gocui.ModNone, app.DirectScrollContentUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyCtrlJ, gocui.ModNone, app.DirectScrollContentDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.KeyCtrlK, gocui.ModNone, app.DirectScrollContentUp); err != nil {
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
	if err := g.SetKeybinding(FilesViewName, gocui.MouseLeft, gocui.ModNone, app.OnFilesMouseClick); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.MouseWheelUp, gocui.ModNone, app.OnFilesMouseWheelUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilesViewName, gocui.MouseWheelDown, gocui.ModNone, app.OnFilesMouseWheelDown); err != nil {
		return err
	}

	if err := g.SetKeybinding(SearchViewName, gocui.KeyEnter, gocui.ModNone, app.ConfirmSearch); err != nil {
		return err
	}
	if err := g.SetKeybinding(SearchViewName, gocui.KeyEsc, gocui.ModNone, app.CancelSearch); err != nil {
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
	if err := g.SetKeybinding(ContentViewName, gocui.KeyEnter, gocui.ModNone, app.FocusFilesView); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, gocui.KeyEsc, gocui.ModNone, app.FocusFilesView); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, 'h', gocui.ModNone, app.FocusFilesView); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, gocui.KeyArrowLeft, gocui.ModNone, app.FocusFilesView); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, gocui.MouseWheelUp, gocui.ModNone, app.OnContentMouseWheelUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(ContentViewName, gocui.MouseWheelDown, gocui.ModNone, app.OnContentMouseWheelDown); err != nil {
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
	if err := g.SetKeybinding(HelpViewName, gocui.KeyArrowUp, gocui.ModNone, app.ScrollHelpUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'k', gocui.ModNone, app.ScrollHelpUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyArrowDown, gocui.ModNone, app.ScrollHelpDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, 'j', gocui.ModNone, app.ScrollHelpDown); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyPgup, gocui.ModNone, app.ScrollHelpPageUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(HelpViewName, gocui.KeyPgdn, gocui.ModNone, app.ScrollHelpPageDown); err != nil {
		return err
	}

	if err := g.SetKeybinding(FilterViewName, gocui.KeyEnter, gocui.ModNone, app.ApplyFilter); err != nil {
		return err
	}
	if err := g.SetKeybinding(FilterViewName, gocui.KeyEsc, gocui.ModNone, app.CancelFilter); err != nil {
		return err
	}

	cacheViews := []string{CacheListViewName, CacheDetailViewName}
	for _, cvName := range cacheViews {
		if err := g.SetKeybinding(cvName, gocui.KeyEsc, gocui.ModNone, app.CloseCacheView); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'q', gocui.ModNone, app.CloseCacheView); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeySpace, gocui.ModNone, app.ToggleCacheSelect); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'a', gocui.ModNone, app.SelectAllCacheEntries); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'd', gocui.ModNone, app.PromptDeleteCacheEntries); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'x', gocui.ModNone, app.PromptDeleteCacheEntries); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeyCtrlD, gocui.ModNone, app.PromptClearCache); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'y', gocui.ModNone, app.ConfirmCacheAction); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'n', gocui.ModNone, app.CancelCacheAction); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeyArrowUp, gocui.ModNone, app.ScrollCacheViewUp); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'k', gocui.ModNone, app.ScrollCacheViewUp); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeyArrowDown, gocui.ModNone, app.ScrollCacheViewDown); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, 'j', gocui.ModNone, app.ScrollCacheViewDown); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeyPgup, gocui.ModNone, app.ScrollCacheViewPageUp); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.KeyPgdn, gocui.ModNone, app.ScrollCacheViewPageDown); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.MouseWheelUp, gocui.ModNone, app.OnCacheMouseWheelUp); err != nil {
			return err
		}
		if err := g.SetKeybinding(cvName, gocui.MouseWheelDown, gocui.ModNone, app.OnCacheMouseWheelDown); err != nil {
			return err
		}
	}

	if err := g.SetKeybinding(CacheListViewName, gocui.MouseLeft, gocui.ModNone, app.OnCacheListMouseClick); err != nil {
		return err
	}

	return nil
}

func (app *App) ConfirmCacheAction(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	awaitingDel := app.awaitingCacheDeleteConfirmation
	awaitingClr := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if awaitingDel {
		return app.ConfirmDeleteCacheEntries(g, v)
	}
	if awaitingClr {
		return app.ConfirmClearCache(g, v)
	}
	return nil
}

func (app *App) CancelCacheAction(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	awaitingDel := app.awaitingCacheDeleteConfirmation
	awaitingClr := app.awaitingCacheClearConfirmation
	app.mutex.Unlock()

	if awaitingDel {
		return app.CancelDeleteCacheEntries(g, v)
	}
	if awaitingClr {
		return app.CancelClearCache(g, v)
	}
	return nil
}

func (app *App) QuitHandler(g *gocui.Gui, v *gocui.View) error {
	app.mutex.Lock()
	isCacheViewOpen := app.showCacheView
	awaitingConfirm := app.awaitingCacheClearConfirmation || app.awaitingCacheDeleteConfirmation
	searchActive := app.isSearchActive
	app.saveCurrentStateToCache()
	app.mutex.Unlock()

	if searchActive {
		return app.CancelSearch(g, v)
	}
	if awaitingConfirm {
		return app.CancelCacheAction(g, v)
	}
	if isCacheViewOpen {
		return app.CloseCacheView(g, v)
	}

	return quit(g, v)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
