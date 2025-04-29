package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	// --- Preview State ---
	isPreviewOpen  bool
	previewFile    string
	previewContent string
	previewOriginY int

	// --- Cache State ---
	cache         AppCache
	cacheFilePath string
}

// NewApp creates a new application instance.
func NewApp(rootDir string) *App {
	tke, _ := tiktoken.GetEncoding("cl100k_base")

	app := &App{
		rootDir:          rootDir,
		selectedFiles:    make(map[string]bool),
		gitignoreMatcher: nil,
		fileList:         []string{},
		allFiles:         []string{},
		currentLine:      0,
		showHelp:         false,
		filterMode:       ExcludeMode,
		excludes:         DefaultExcludes,
		includes:         "",
		tokenizer:        tke,
		isPreviewOpen:    false,
		previewOriginY:   0,
		cache:            make(AppCache),
	}

	// --- Cache ---
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

		if entry, ok := app.cache[app.rootDir]; ok {
			app.includes = entry.Includes
			app.excludes = entry.Excludes
			app.filterMode = entry.FilterMode

			entry.LastOpened = time.Now()
			app.cache[app.rootDir] = entry
		} else {
			app.cache[app.rootDir] = DirectoryCache{
				Includes:   app.includes,
				Excludes:   app.excludes,
				LastOpened: time.Now(),
				FilterMode: app.filterMode,
			}
		}

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

// --- Cache Helper Functions ---

// getCacheFilePath determines the path for the cache file.
func getCacheFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	cacheDir := filepath.Join(configDir, "grepforllm") // ~/.config/grepforllm
	err = os.MkdirAll(cacheDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	return filepath.Join(cacheDir, "cache.json"), nil // ~/.config/grepforllm/cache.json
}

// loadCache reads the cache file and unmarshals it.
func loadCache(filePath string) (AppCache, error) {
	cache := make(AppCache)
	if filePath == "" {
		return cache, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, fmt.Errorf("failed to read cache file %s: %w", filePath, err)
	}

	if len(data) == 0 {
		return cache, nil
	}

	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data from %s: %w", filePath, err)
	}

	return cache, nil
}

// saveCache marshals the cache map and writes it to the file atomically.
func saveCache(filePath string, cache AppCache) error {
	if filePath == "" {
		return fmt.Errorf("cache file path is empty, cannot save")
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	tempFile := filePath + ".tmp"
	err = os.WriteFile(tempFile, data, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write temporary cache file %s: %w", tempFile, err)
	}

	err = os.Rename(tempFile, filePath)
	if err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename temporary cache file to %s: %w", filePath, err)
	}

	return nil
}
