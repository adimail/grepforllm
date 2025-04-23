package internal

import (
	"sync"

	"github.com/awesome-gocui/gocui"
	"github.com/denormal/go-gitignore"
	"github.com/pkoukk/tiktoken-go"
)

// View names
const (
	FilesViewName    = "files"
	ContentViewName  = "content"
	HelpViewName     = "help"
	PreviewViewName  = "preview"
	MaxSelectedFiles = 50
	MaxFileSizeBytes = 100 * 1024
	FilterViewName   = "filter"
	StatusViewName   = "status"
	DefaultExcludes  = ".git/,node_modules/"
)

// FilterMode defines whether the filter includes or excludes patterns.
type FilterMode int

const (
	ExcludeMode FilterMode = iota // Filter excludes matching patterns (default)
	IncludeMode                   // Filter includes *only* matching patterns
)

// App holds the application state and logic.
type App struct {
	g                *gocui.Gui
	rootDir          string
	fileList         []string // Currently displayed list of relative file paths
	allFiles         []string // All discovered files before filtering
	selectedFiles    map[string]bool
	gitignoreMatcher gitignore.GitIgnore
	currentLine      int // Cursor position in the fileList view
	showHelp         bool
	filterMode       FilterMode
	excludes         string // Comma-separated patterns to exclude
	includes         string // Comma-separated patterns to include
	mutex            sync.Mutex
	tokenizer        *tiktoken.Tiktoken

	// --- Preview State ---
	isPreviewOpen  bool
	previewFile    string
	previewContent string
	previewOriginY int
}

// NewApp creates a new application instance.
func NewApp(rootDir string) *App {
	tke, _ := tiktoken.GetEncoding("cl100k_base")

	return &App{
		rootDir:          rootDir,
		selectedFiles:    make(map[string]bool),
		gitignoreMatcher: nil, // <-- Initialize to nil
		fileList:         []string{},
		allFiles:         []string{},
		currentLine:      0,
		showHelp:         false,
		filterMode:       ExcludeMode, // Default to exclude mode
		excludes:         DefaultExcludes,
		includes:         "",
		tokenizer:        tke,
		isPreviewOpen:    false,
		previewOriginY:   0,
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

func (app *App) SetGitignoreMatcher(matcher gitignore.GitIgnore) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.gitignoreMatcher = matcher
}
