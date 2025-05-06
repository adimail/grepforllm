package internal

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/denormal/go-gitignore"
	"github.com/pkoukk/tiktoken-go"
)

// View names
const (
	PathViewName     = "path"
	FilesViewName    = "files"
	ContentViewName  = "content"
	HelpViewName     = "help"
	FilterViewName   = "filter"
	StatusViewName   = "status"
	CacheViewName    = "cache"
	ConfirmViewName  = "confirm"
	DefaultExcludes  = ".git/,node_modules/"
	MaxSelectedFiles = 50
	MaxFileSizeBytes = 100 * 1024
)

// FilterMode defines whether the filter includes or excludes patterns.
type FilterMode int

const (
	ExcludeMode FilterMode = iota // Filter excludes matching patterns (default)
	IncludeMode                   // Filter includes *only* matching patterns
)

// --- Cache Structures ---

// DirectoryCache holds the cached settings for a specific directory.
type DirectoryCache struct {
	Includes   string     `json:"includes"`
	Excludes   string     `json:"excludes"`
	LastOpened time.Time  `json:"lastOpened"`
	FilterMode FilterMode `json:"filterMode"`
}

type AppCache map[string]DirectoryCache

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

	// --- Live Preview State (Content View) ---
	currentlyPreviewedFile string // File path for the live content view preview
	contentViewOriginY     int    // Scroll position for the content view

	// --- Cache State ---
	cache         AppCache
	cacheFilePath string

	// --- Cache View State ---
	showCacheView                  bool
	cacheViewContent               string
	cacheViewOriginY               int
	awaitingCacheClearConfirmation bool

	// --- Loading State ---
	isLoading     bool
	loadingError  error
	loadStartTime time.Time

	// --- Copy Highlight State ---
	isCopyHighlightActive bool
}

// NewApp creates a new application instance.
func NewApp(rootDir string) *App {
	tke, _ := tiktoken.GetEncoding("cl100k_base")

	app := &App{
		rootDir:                rootDir,
		selectedFiles:          make(map[string]bool),
		gitignoreMatcher:       nil,
		fileList:               []string{},
		allFiles:               []string{},
		currentLine:            0,
		showHelp:               false,
		filterMode:             ExcludeMode,
		excludes:               DefaultExcludes,
		includes:               "",
		tokenizer:              tke,
		currentlyPreviewedFile: "", // Initialize live preview field
		contentViewOriginY:     0,  // Initialize content view scroll
		cache:                  make(AppCache),

		// --- Initialize Cache View State ---
		showCacheView:                  false,
		cacheViewContent:               "",
		cacheViewOriginY:               0,
		awaitingCacheClearConfirmation: false,

		// --- Initialize Loading State ---
		isLoading:     true,
		loadingError:  nil,
		loadStartTime: time.Now(),

		// --- Initialize Copy Highlight State ---
		isCopyHighlightActive: false,
	}

	var err error
	app.cacheFilePath, err = getCacheFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not determine cache file path: %v\n", err)
	} else {
		app.cache, err = loadCache(app.cacheFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not load cache file %s: %v\n", app.cacheFilePath, err)
			app.cache = make(AppCache)
		}

		// Load settings for the current directory from cache if available
		if entry, ok := app.cache[app.rootDir]; ok {
			app.includes = entry.Includes
			app.excludes = entry.Excludes
			app.filterMode = entry.FilterMode
			entry.LastOpened = time.Now()
			app.cache[app.rootDir] = entry
		} else {
			// Only add if not found, keep existing defaults otherwise
			app.cache[app.rootDir] = DirectoryCache{
				Includes:   app.includes,
				Excludes:   app.excludes,
				LastOpened: time.Now(),
				FilterMode: app.filterMode,
			}
		}

		// Save cache immediately after potential update/add
		err = saveCache(app.cacheFilePath, app.cache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save cache file %s: %v\n", app.cacheFilePath, err)
		}
	}

	return app
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
