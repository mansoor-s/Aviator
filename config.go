package aviator

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mansoor-s/aviator/builder"
	"github.com/mansoor-s/aviator/js"
	"sync"
	"text/template"
)

type Logger interface {
	Info(string)
	Error(string)
}

type Option func(config *Aviator)

// All options must start "With"

type Aviator struct {
	componentTree  builder.ComponentTree
	vm             js.VM
	ssrBuilder     *builder.SSRBuilder
	browserBuilder *builder.BrowserBuilder
	viewManager    *builder.ViewManagerOld
	watcher        *fsnotify.Watcher
	logger         Logger

	assetListenPath  string
	staticAssetRoute string

	htmlGenerator *template.Template

	isDevMode bool
	numVMs    int
	htmlLang  string

	isInitialized bool

	viewsPath  string
	outputPath string

	// TODO: optimize by removing this lock for non-dev environment
	viewLock sync.RWMutex

	_devModeSSRCompiledJs  []byte
	_devModeSSRCompiledCSS []byte
	_compiledCSSFileName   string
}

func WithDevMode(isDevMode bool) Option {
	return func(a *Aviator) {
		a.isDevMode = isDevMode
	}
}

func WithNumJsVMs(numVMs int) Option {
	return func(a *Aviator) {
		a.numVMs = numVMs
	}
}

func WithViewsPath(path string) Option {
	return func(a *Aviator) {
		a.viewsPath = path
	}
}

func WithAssetOutputPath(path string) Option {
	return func(a *Aviator) {
		a.outputPath = path
	}
}

func WithStaticAssetRoute(route string) Option {
	return func(a *Aviator) {
		a.staticAssetRoute = route
	}
}

func WithHTMLLang(lang string) Option {
	return func(a *Aviator) {
		a.htmlLang = lang
	}
}

func WithLogger(l Logger) Option {
	return func(a *Aviator) {
		a.logger = l
	}
}

func WithNullLogger() Option {
	return func(a *Aviator) {
		a.logger = nullLogger{}
	}
}

//stdOutLogger writes logs to STDOUT
type stdOutLogger struct {
}

func (l stdOutLogger) Info(str string) {
	fmt.Printf("info: %s\n", str)
}

func (l stdOutLogger) Error(str string) {
	fmt.Printf("error: %s\n", str)
}

//nullLogger is a no-op logger
type nullLogger struct {
}

func (l nullLogger) Info(_ string) {
}

func (l nullLogger) Error(_ string) {
}
