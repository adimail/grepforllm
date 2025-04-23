// FILE: cmd/main.go
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
	app := internal.NewApp(absRootDir)

	matcher, err := internal.LoadGitignoreMatcher(app.RootDir())
	if err != nil {
		log.Printf("Warning: Failed to parse .gitignore: %v", err)
	} else if matcher != nil {
		app.SetGitignoreMatcher(matcher)
	} else {
		log.Printf("Info: No .gitignore file found or parsed in %s", app.RootDir())
	}

	err = app.ListFiles()
	if err != nil {
		log.Fatalf("Error listing initial files in %s: %v", app.RootDir(), err)
	}
	if len(app.FileList()) == 0 {
		log.Printf("No files found in %s (or all excluded by .gitignore or default excludes)", app.RootDir())
	}

	// --- Initialize gocui ---
	g, err := gocui.NewGui(gocui.OutputNormal, true) // OutputNormal, support mouse
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorMagenta
	g.SelBgColor = gocui.ColorDefault
	g.Cursor = true

	// --- Configure and Run App ---
	app.SetGui(g) // Pass the gui instance to the app
	g.SetManagerFunc(app.Layout)

	if err := app.SetKeybindings(g); err != nil {
		log.Panicln(err)
	}

	// --- Main Loop ---
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(fmt.Sprintf("Error in main loop: %v", err)) // More specific error
	}
}
