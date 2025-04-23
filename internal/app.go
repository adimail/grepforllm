package internal

import (
	"log"
	"sync"

	"github.com/awesome-gocui/gocui"
	"github.com/pkoukk/tiktoken-go"
)

// View names
const (
	FilesViewName   = "files"
	ContentViewName = "content"
	HelpViewName    = "help"
	FilterViewName  = "filter"
	StatusViewName  = "status" // Added for clarity
	DefaultExcludes = ".git/,__pycache__/,node_modules/"
)

// FilterMode defines whether the filter includes or excludes patterns.
type FilterMode int

const (
	ExcludeMode FilterMode = iota // Filter excludes matching patterns (default)
	IncludeMode                   // Filter includes *only* matching patterns
)

// App holds the application state and logic.
type App struct {
	g               *gocui.Gui
	rootDir         string
	fileList        []string // Currently displayed list of relative file paths
	allFiles        []string // All discovered files before filtering
	selectedFiles   map[string]bool
	gitIgnoredFiles map[string]bool // <<< Add this field
	currentLine     int             // Cursor position in the fileList view
	showHelp        bool
	filterMode      FilterMode
	excludes        string // Comma-separated patterns to exclude
	includes        string // Comma-separated patterns to include
	mutex           sync.Mutex
	tokenizer       *tiktoken.Tiktoken
}

// NewApp creates a new application instance.
func NewApp(rootDir string) *App {
	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		log.Fatalf("Error getting tiktoken encoding 'cl100k_base': %v", err)
	}

	return &App{
		rootDir:         rootDir,
		selectedFiles:   make(map[string]bool),
		gitIgnoredFiles: make(map[string]bool), // <<< Initialize the map
		fileList:        []string{},
		allFiles:        []string{},
		currentLine:     0,
		showHelp:        false,
		filterMode:      ExcludeMode, // Default to exclude mode
		excludes:        DefaultExcludes,
		includes:        "",
		tokenizer:       tke,
	}
}

// SetGui assigns the gocui Gui object to the App.
func (app *App) SetGui(g *gocui.Gui) {
	app.g = g
}

// RootDir returns the root directory being scanned.
func (app *App) RootDir() string {
	return app.rootDir
}

// FileList returns the currently filtered list of files.
func (app *App) FileList() []string {
	// Return a copy to prevent external modification? For now, return direct slice.
	return app.fileList
}

// <<< Add a method to populate gitIgnoredFiles >>>
// SetGitIgnoredFiles populates the internal map of gitignored files.
func (app *App) SetGitIgnoredFiles(ignoredPaths []string) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.gitIgnoredFiles = make(map[string]bool, len(ignoredPaths))
	for _, p := range ignoredPaths {
		app.gitIgnoredFiles[p] = true
	}
	// It might be good practice to re-apply filters if this could be called later,
	// but for initial setup, it's fine.
}
