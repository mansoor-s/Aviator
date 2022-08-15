package aviator

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mansoor-s/aviator/builder"
	"github.com/mansoor-s/aviator/js"
	"net/http"
	"path/filepath"
	"strings"
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
		numVMs:        1,
		logger:        stdOutLogger{},
		htmlGenerator: defaultHTMLGenerator,
		htmlLang:      "en",
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

	//TODO: make this configurable
	//a.vm, err = js.NewV8VMPool(a.numVMs, svelteCompilerCode)
	a.vm, err = js.NewGojaVMPool(a.numVMs)
	//some vm instance initializations might have succeeded. Clean up if possible
	if err != nil {
		return err
	}

	/*_, err = a.vm.Eval(
		"svelte_compiler_init.js",
		svelteCompilerCode,
	)*/
	err = a.vm.InitializationScript(
		"svelte_compiler_init.js",
		svelteCompilerCode,
	)
	if err != nil {
		return err
	}

	if a.isDevMode {
		err := a.rebuildViews()
		if err != nil {
			return err
		}
	}

	//if !a.isDevMode {
	// add SSR JS to VM as compiled JS for faster execution
	//}

	a.isInitialized = true

	return nil
}

//startChangeWatcher requires a mutex lock. That's currently handled by the caller
func (a *Aviator) startChangeWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				var err error
				if !ok {
					return
				}

				err = a.rebuildViews()

				if err != nil {
					a.logger.Error(
						fmt.Errorf("failed to rebuild views on FS change: %w", err).Error(),
					)
				}
				//return early because we're about to rebuild everything from scratch
				err = watcher.Close()
				if err != nil {
					a.logger.Error(
						fmt.Errorf("error fswatch close: %w", err).Error(),
					)
				}
				return
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				a.logger.Error(fmt.Errorf("watcher error: %w", err).Error())
			}
		}
	}()

	//fsnotify doesn't currently support watching a whole directory tree, so we must
	//manually watch each child directory here
	for _, dirPath := range a.componentTree.GetAllDescendantPaths() {
		err = watcher.Add(dirPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// rebuildViews destroys all View objects and rescans the views directory
// to reconstruct the component tree.
// It will then re-build all components
func (a *Aviator) rebuildViews() error {
	a.viewLock.Lock()
	defer a.viewLock.Unlock()

	var err error
	a.componentTree, err = builder.CreateComponentTree(a.viewsPath)
	if err != nil {
		return err
	}

	a.viewManager = builder.NewViewManagerOld(a.componentTree)

	a.ssrBuilder = builder.NewSSRBuilder(a.vm, a.viewManager, a.viewsPath)

	compiledSSRResult, err := a.ssrBuilder.DevBuild(context.Background())
	if err != nil {
		//don't exit the update loop on esbuild errors
		a.logger.Error("error building SSR: " + err.Error())
		//watch for changes
		return a.startChangeWatcher()
	}

	browserBuilder := builder.NewBrowserBuilder(
		a.vm, a.viewManager, a.viewsPath, a.outputPath)

	err = browserBuilder.BuildDev(context.Background())
	if err != nil {
		//don't exit the update loop on esbuild errors
		a.logger.Error("error building browser bundle: " + err.Error())
		//watch for changes
		return a.startChangeWatcher()
	}

	a._devModeSSRCompiledJs = compiledSSRResult.JS
	a._devModeSSRCompiledCSS = compiledSSRResult.CSS
	//cssHash := fmt.Sprintf("%x", sha256.Sum256(a._devModeSSRCompiledCSS))
	//a._compiledCSSFileName = "bundled_css_" + cssHash[:10] + ".css"

	/*err = a.vm.InitializationScript(
		context.Background(),
		"aviator_ssr_router.js",
		string(compiledSSRResult.JS),
	)*/
	_, err = a.vm.Eval(
		"aviator_ssr_router.js",
		string(compiledSSRResult.JS),
	)
	if err != nil {
		return err
	}

	//watch for changes
	return a.startChangeWatcher()
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
	var view *builder.View
	if a.isDevMode {
		a.viewLock.RLock()
		defer a.viewLock.RUnlock()
		view = a.viewManager.ViewByRelPath(viewPath)
	} else {
		view = a.viewManager.ViewByRelPath(viewPath)
	}

	if view == nil {
		return "", fmt.Errorf("view does not exist in path %s", viewPath)
	}

	//TODO: Create a sanitized copy of the props object where
	// string objects are escaped to avoid script injections on the front end
	// Should users be able to bypass escaping using tags?
	jsonValue := "{}"
	if props != nil {
		jsonProps, err := json.Marshal(props)
		if err != nil {
			return "", fmt.Errorf("failed to json serialize props %w", err)
		}
		jsonValue = string(jsonProps)
	}

	expr := fmt.Sprintf(
		"; __aviator__.render(%q, %s, {})",
		view.WrappedUniqueName,
		jsonValue,
	)
	renderOutputStr, err := a.vm.Eval("runtime_renderer", expr)
	if err != nil {
		return renderOutputStr, err
	}

	ssrOutputData := &ssrData{}
	err = json.Unmarshal([]byte(renderOutputStr), ssrOutputData)
	if err != nil {
		return "", err
	}

	ssrOutputData.Head = ssrOutputData.Head + "\n" +
		a.createJSImportTags(view.JSImports) +
		a.createCSSImportTag(view.CSSImports) +
		a.createPropsScriptElem(jsonValue)

	ssrOutputData.Lang = a.htmlLang
	//cssPath := path.Join(a.assetListenPath, a._compiledCSSFileName)
	//ssrOutputData.BundledCSS = "<link href=\"" + cssPath + "\" rel=\"stylesheet\">"

	buf := new(bytes.Buffer)
	err = a.htmlGenerator.Execute(buf, ssrOutputData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (a *Aviator) createPropsScriptElem(props string) string {
	format := "<script id=\"__aviator_props\" type=\"text/template\" defer>%s</script>\n"
	return fmt.Sprintf(format, props)
}

func (a *Aviator) createJSImportTags(assetImports []string) string {
	output := ""
	format := "<script type=\"module\" src=\"%s\" defer></script>\n"
	for _, rawPath := range assetImports {
		output += fmt.Sprintf(format, filepath.Join(a.staticAssetRoute, rawPath))
	}

	return output
}

func (a *Aviator) createCSSImportTag(assetImports []string) string {
	output := ""
	format := "<link href=\"%s\" rel=\"stylesheet\">\n"
	for _, rawPath := range assetImports {
		output += fmt.Sprintf(format, filepath.Join(a.staticAssetRoute, rawPath))
	}

	return output
}

func (a *Aviator) Close() {
	//a.vm.Close()
}

func (a *Aviator) DynamicAssetHandler(listenPath string) http.HandlerFunc {
	a.assetListenPath = listenPath
	return func(resp http.ResponseWriter, req *http.Request) {
		a.viewLock.RLock()
		defer a.viewLock.RUnlock()

		header := resp.Header()

		var fileName string
		fileNameParts := strings.Split(req.URL.Path, "/")
		if len(fileNameParts) == 1 {
			fileName = fileNameParts[0]
		} else {
			fileName = fileNameParts[len(fileNameParts)-1]
		}

		pathParts := strings.Split(fileName, ".")
		if len(pathParts) <= 1 {
			resp.WriteHeader(400)
			return
		}

		if strings.ToLower(pathParts[len(pathParts)-1]) == "css" &&
			fileName == a._compiledCSSFileName {
			header["Content-Type"] = []string{"text/css"}
			resp.Write(a._devModeSSRCompiledCSS)

		} else if strings.ToLower(pathParts[len(pathParts)-1]) == "js" {
			//contentType = "text/javascript"
			resp.WriteHeader(400)
		} else {
			resp.WriteHeader(400)
		}

	}
}
