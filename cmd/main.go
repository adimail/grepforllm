package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/adimail/grepforllm/internal"
	"github.com/awesome-gocui/gocui"
)

func main() {
	// --- Argument Parsing ---
	rootDir := flag.String("dir", ".", "Root directory to scan")
	flag.Parse()

	absRootDir, err := filepath.Abs(*rootDir)
	if err != nil {
		log.Fatalf("Error getting absolute path for %s: %v", *rootDir, err)
	}

	// Check if directory exists
	info, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Error: Directory does not exist: %s", absRootDir)
		}
		log.Fatalf("Error accessing directory %s: %v", absRootDir, err)
	}
	if !info.IsDir() {
		log.Fatalf("Error: Path is not a directory: %s", absRootDir)
	}

	// --- Initialize App State ---
	app := internal.NewApp(absRootDir) // isLoading is true initially

	// --- Load Gitignore (Synchronous, relatively fast) ---
	matcher, err := internal.LoadGitignoreMatcher(app.RootDir())
	if err != nil {
		log.Printf("Warning: Failed to parse .gitignore: %v", err)
	} else if matcher != nil {
		app.SetGitignoreMatcher(matcher)
	} else {
		log.Printf("Info: No .gitignore file found or parsed in %s", app.RootDir())
	}

	// --- Initialize gocui ---
	g, err := gocui.NewGui(gocui.OutputNormal, true)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorMagenta
	g.SelBgColor = gocui.ColorDefault
	g.Cursor = true

	// --- Configure and Run App ---
	app.SetGui(g)
	g.SetManagerFunc(app.Layout)

	if err := app.SetKeybindings(g); err != nil {
		log.Panicln(err)
	}

	// --- Start Asynchronous File Loading ---
	go func() {
		err := app.ListFiles()

		app.SetLoadingComplete(err)

		// Trigger a UI refresh from the main GUI thread
		g.Update(func(g *gocui.Gui) error {
			// This empty function just ensures the ManagerFunc (app.Layout) runs again
			// after the loading state has been updated.
			return nil
		})
	}()

	// --- Main Loop ---
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(fmt.Sprintf("Error in main loop: %v", err))
	}
}
