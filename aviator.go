package aviator

import (
	"context"
	_ "embed"
	"errors"
	"github.com/mansoor-s/aviator/builder"
	"github.com/mansoor-s/aviator/js"
	"text/template"
)

//go:embed embedded_assets/svelte_compiler.js
var svelteCompilerCode string

//go:embed embedded_assets/defaultHTML.html
var defaultHTMLTemplate string

var defaultHTMLGenerator = template.Must(template.New("defaultHTML").Parse(defaultHTMLTemplate))

//JSError provides a JS runtime agnostic error
//type JSError = js.JSError

func NewAviator(configs ...Option) *Aviator {
	a := &Aviator{
		numVMs:        4,
		logger:        stdOutLogger{},
		htmlGenerator: defaultHTMLGenerator,
		htmlLang:      "en",
		cacheDir:      ".aviator_cache",
	}
	for _, config := range configs {
		config(a)
	}

	return a
}

//configCheck checks to see if the provided configs are sufficient to start
func (a *Aviator) configCheck() error {
	if len(a.outputPath) == 0 {
		return errors.New("asset output path not specified")
	}

	if len(a.viewsPath) == 0 {
		return errors.New("svelte views directory path not specified")
	}

	return nil
}

// Init scans the contents of the configured view directory and starts
// watching for changes in dev mode.
// starts JS VM pool
// compiles svelte compiler
func (a *Aviator) Init() error {
	err := a.configCheck()
	if err != nil {
		return err
	}

	a.vm, err = js.NewGojaVMPool(a.numVMs)
	//some vm instance initializations might have succeeded. Clean up if possible
	if err != nil {
		return err
	}

	err = a.vm.InitializationScript(
		"svelte_compiler_init.js",
		svelteCompilerCode,
	)
	if err != nil {
		return err
	}

	a.componentTree, err = builder.CreateComponentTree(a.viewsPath)
	if err != nil {
		return err
	}

	a.viewManager, err = builder.NewViewManager(
		a.logger,
		a.vm,
		a.componentTree,
		a.htmlGenerator,
		a.isDevMode,
		a.cacheDir,
		a.viewsPath,
		a.staticAssetRoute,
		a.htmlLang,
	)
	if err != nil {
		return err
	}

	err = a.viewManager.StartWatch()
	if err != nil {
		return err
	}

	a.isInitialized = true

	return nil
}

type ssrData struct {
	Head    string
	Body    string
	PageCSS string

	//this is created during bundling
	BundledCSS string

	//these pare provided by the user
	Lang string
}

func (a *Aviator) Render(
	ctx context.Context,
	viewPath string,
	props interface{},
) (string, error) {
	return a.viewManager.Render(ctx, viewPath, props)
}

//GetStaticAsset returns a byte array contents of the static asset and a boolean
//indicating whether the static asset was found
func (a *Aviator) GetStaticAsset(name string) ([]byte, string, bool) {
	staticAsset, found := a.viewManager.GetStaticAsset(name)

	return staticAsset.Content, staticAsset.MimeType, found
}
