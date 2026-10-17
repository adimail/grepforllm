package internal

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/denormal/go-gitignore"
	"github.com/fsnotify/fsnotify"
)

const (
	PathViewName     = "path"
	FilesViewName    = "files"
	ContentViewName  = "content"
	HelpViewName     = "help"
	FilterViewName   = "filter"
	SearchViewName   = "search"
	StatusViewName   = "status"
	CacheViewName    = "cache"
	ConfirmViewName  = "confirm"
	DefaultExcludes  = ".git/,node_modules/,vendor/,.next/,.nuxt/,dist/,build/,target/,.venv/,__pycache__/,*.min.js,*.min.css,*.map,package-lock.json,pnpm-lock.yaml,yarn.lock,Cargo.lock,go.sum,poetry.lock,composer.lock,flake.lock"
	MaxSelectedFiles = 50
	MaxFileSizeBytes = 100 * 1024
)

type FilterMode int

const (
	ExcludeMode FilterMode = iota
	IncludeMode
)

type SortMode int

const (
	SortByName SortMode = iota
	SortBySize
	SortByTokens
	SortByModTime
)

type GitStatus int

const (
	GitClean GitStatus = iota
	GitModified
	GitUntracked
)

type GitFilterMode int

const (
	GitFilterAll GitFilterMode = iota
	GitFilterChanged
	GitFilterNew
	GitFilterUpdated
)

type FileItem struct {
	RelPath         string
	SizeBytes       int64
	ModTime         time.Time
	EstimatedTokens int
	GitStatus       GitStatus
}

type DirectoryCache struct {
	Includes      string        `json:"includes"`
	Excludes      string        `json:"excludes"`
	SearchQuery   string        `json:"searchQuery,omitempty"`
	LastOpened    time.Time     `json:"lastOpened"`
	FilterMode    FilterMode    `json:"filterMode"`
	SortMode      SortMode      `json:"sortMode,omitempty"`
	GitFilterMode GitFilterMode `json:"gitFilterMode,omitempty"`
}

type AppCache map[string]DirectoryCache

type App struct {
	g                *gocui.Gui
	rootDir          string
	fileList         []FileItem
	allFiles         []FileItem
	selectedFiles    map[string]bool
	gitignoreMatcher gitignore.GitIgnore
	currentLine      int
	showHelp         bool
	filterMode       FilterMode
	sortMode         SortMode
	gitFilterMode    GitFilterMode
	gitStatusMap     map[string]GitStatus
	excludes         string
	includes         string
	searchQuery      string
	isSearchActive   bool
	mutex            sync.Mutex
	scanMutex        sync.Mutex

	currentlyPreviewedFile string
	contentViewOriginY     int

	cache         AppCache
	cacheFilePath string

	showCacheView                  bool
	cacheViewContent               string
	cacheViewOriginY               int
	cacheCurrentLine               int
	cacheTotalLines                int
	awaitingCacheClearConfirmation bool

	isLoading     bool
	loadingError  error
	loadStartTime time.Time

	isCopyHighlightActive bool

	watcher     *fsnotify.Watcher
	watcherDone chan struct{}
	watchedDirs map[string]bool
}

func NewApp(rootDir string) *App {
	watcher, _ := fsnotify.NewWatcher()

	app := &App{
		rootDir:                rootDir,
		selectedFiles:          make(map[string]bool),
		gitignoreMatcher:       nil,
		fileList:               []FileItem{},
		allFiles:               []FileItem{},
		currentLine:            0,
		showHelp:               false,
		filterMode:             ExcludeMode,
		sortMode:               SortByName,
		gitFilterMode:          GitFilterAll,
		gitStatusMap:           make(map[string]GitStatus),
		excludes:               "",
		includes:               "",
		searchQuery:            "",
		isSearchActive:         false,
		currentlyPreviewedFile: "",
		contentViewOriginY:     0,
		cache:                  make(AppCache),

		showCacheView:                  false,
		cacheViewContent:               "",
		cacheViewOriginY:               0,
		cacheCurrentLine:               0,
		cacheTotalLines:                0,
		awaitingCacheClearConfirmation: false,

		isLoading:     true,
		loadingError:  nil,
		loadStartTime: time.Now(),

		isCopyHighlightActive: false,

		watcher:     watcher,
		watcherDone: make(chan struct{}),
		watchedDirs: make(map[string]bool),
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

		for k, v := range app.cache {
			if v.Excludes == DefaultExcludes {
				v.Excludes = ""
				app.cache[k] = v
			}
		}

		if entry, ok := app.cache[app.rootDir]; ok {
			app.includes = entry.Includes
			app.excludes = entry.Excludes
			app.searchQuery = entry.SearchQuery
			if app.searchQuery != "" {
				app.isSearchActive = true
			}
			app.filterMode = entry.FilterMode
			app.sortMode = entry.SortMode
			app.gitFilterMode = entry.GitFilterMode
			entry.LastOpened = time.Now()
			app.cache[app.rootDir] = entry
		} else {
			app.cache[app.rootDir] = DirectoryCache{
				Includes:      app.includes,
				Excludes:      app.excludes,
				SearchQuery:   app.searchQuery,
				LastOpened:    time.Now(),
				FilterMode:    app.filterMode,
				SortMode:      app.sortMode,
				GitFilterMode: app.gitFilterMode,
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

func (app *App) FileList() []FileItem {
	return app.fileList
}

func (app *App) SetGitignoreMatcher(matcher gitignore.GitIgnore) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.gitignoreMatcher = matcher
}

func (app *App) Close() {
	if app.watcher != nil {
		select {
		case <-app.watcherDone:
		default:
			close(app.watcherDone)
		}
		_ = app.watcher.Close()
	}
}

func (app *App) saveCurrentStateToCache() {
	if app.cacheFilePath == "" {
		return
	}
	entry := DirectoryCache{
		Includes:      app.includes,
		Excludes:      app.excludes,
		SearchQuery:   app.searchQuery,
		LastOpened:    time.Now(),
		FilterMode:    app.filterMode,
		SortMode:      app.sortMode,
		GitFilterMode: app.gitFilterMode,
	}
	app.cache[app.rootDir] = entry
	_ = saveCache(app.cacheFilePath, app.cache)
}
