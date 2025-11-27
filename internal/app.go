package internal

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/denormal/go-gitignore"
)

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

type FilterMode int

const (
	ExcludeMode FilterMode = iota
	IncludeMode
)

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
	fileList         []string
	allFiles         []string
	selectedFiles    map[string]bool
	gitignoreMatcher gitignore.GitIgnore
	currentLine      int
	showHelp         bool
	filterMode       FilterMode
	excludes         string
	includes         string
	mutex            sync.Mutex

	currentlyPreviewedFile string
	contentViewOriginY     int

	cache         AppCache
	cacheFilePath string

	showCacheView                  bool
	cacheViewContent               string
	cacheViewOriginY               int
	awaitingCacheClearConfirmation bool

	isLoading     bool
	loadingError  error
	loadStartTime time.Time

	isCopyHighlightActive bool
}

func NewApp(rootDir string) *App {
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
		currentlyPreviewedFile: "",
		contentViewOriginY:     0,
		cache:                  make(AppCache),

		showCacheView:                  false,
		cacheViewContent:               "",
		cacheViewOriginY:               0,
		awaitingCacheClearConfirmation: false,

		isLoading:     true,
		loadingError:  nil,
		loadStartTime: time.Now(),

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

func (app *App) SetGui(g *gocui.Gui) {
	app.g = g
}

func (app *App) RootDir() string {
	return app.rootDir
}

func (app *App) FileList() []string {
	return app.fileList
}

func (app *App) SetGitignoreMatcher(matcher gitignore.GitIgnore) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.gitignoreMatcher = matcher
}
