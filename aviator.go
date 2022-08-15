package aviator

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
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
		a.isDevMode,
		a.cacheDir,
		a.viewsPath,
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
	view := a.viewManager.ViewByRelPath(viewPath)

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
