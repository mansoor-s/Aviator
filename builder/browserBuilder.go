package builder

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/mansoor-s/aviator/js"
	"github.com/mansoor-s/aviator/utils"
	"path/filepath"
	"strings"
	"text/template"
)

/*
type BrowserImports struct {
	JS  []string
	CSS []string
}
*/

type BrowserBuilder struct {
	vm     js.VM
	cache  *cacheManager
	logger utils.Logger

	workingDir string
}

func NewBrowserBuilder(
	logger utils.Logger,
	vm js.VM,
	cache *cacheManager,
	workingDir string,
) *BrowserBuilder {
	return &BrowserBuilder{
		logger:     logger,
		vm:         vm,
		workingDir: workingDir,
		cache:      cache,
	}
}

// The entrypoints are the virtual files created for all Components in the
// browserRuntimePlugin func. This plugin will reference those virtual files
// and will bundle and persist the outputs

//BuildDev creates assets for embedding into the rendered view
// It persists the assets into the output directory
func (b *BrowserBuilder) BuildDev(allViews []*View) error {
	viewsByEntryPoint := make(map[string]*View, len(allViews))
	viewsByOutputName := make(map[string]*View, len(allViews))

	var entryPoints []esbuild.EntryPoint

	for _, view := range allViews {
		if !view.IsEntrypoint {
			continue
		}

		entryPath := view.WrappedUniqueName + "_Runtime.svelte"
		outputPrettyName := view.UniqueName + ".svelte"
		entryPoints = append(entryPoints, esbuild.EntryPoint{
			InputPath:  entryPath,
			OutputPath: outputPrettyName,
		})
		viewsByOutputName[outputPrettyName] = view
		viewsByEntryPoint[entryPath] = view
	}

	result := esbuild.Build(esbuild.BuildOptions{
		EntryPointsAdvanced: entryPoints,
		Outdir:              "./",
		AbsWorkingDir:       b.workingDir,
		Format:              esbuild.FormatESModule,
		Platform:            esbuild.PlatformBrowser,
		// Add "import" condition to support svelte/internal
		// https://esbuild.github.io/api/#how-conditions-work
		Conditions: []string{"browser", "default", "import"},
		Metafile:   false,
		Bundle:     true,
		Sourcemap:  esbuild.SourceMapInline,
		LogLevel:   esbuild.LogLevelInfo,
		Plugins: append(
			[]esbuild.Plugin{
				b.browserRuntimePlugin(viewsByEntryPoint),
				wrappedComponentsPlugin(b.cache, b.workingDir, allViews, b.browserCompile),
				svelteComponentsPlugin(b.cache, b.workingDir, b.browserCompile),
				npmJsPathPlugin(b.workingDir),
			},
		),
		Write: false,
	})
	if len(result.Errors) > 0 {
		msgs := esbuild.FormatMessages(result.Errors, esbuild.FormatMessagesOptions{
			Color:         true,
			Kind:          esbuild.ErrorMessage,
			TerminalWidth: 80,
		})
		return fmt.Errorf(strings.Join(msgs, "\n"))
	}

	for _, file := range result.OutputFiles {
		fileName := filepath.Base(file.Path)
		extension := utils.FileExtension(fileName)
		viewRefName := fileName[:len(fileName)-len(extension)-1]

		view := viewsByOutputName[viewRefName]
		view.JSImports = []string{}
		view.CSSImports = []string{}

		if extension == "js" {
			view.JSImports = append(view.JSImports, fileName)
		} else if extension == "css" {
			view.CSSImports = append(view.CSSImports, fileName)
		}
	}

	return nil
}

//go:embed browserHelperTemplate.gotext
var browserTemplate string

var browserGenerator = template.Must(template.New("browserTemplate").Parse(browserTemplate))

//browserRuntimePlugin renders the browserTemplate for each component
//The rendered content acts as the entrypoint that are used for the esbuild and
//also imported by each of the view in the final HTML
func (b *BrowserBuilder) browserRuntimePlugin(viewsByEntryPoint map[string]*View) esbuild.Plugin {
	return esbuild.Plugin{
		Name: "browserRuntimePlugin",
		Setup: func(epb esbuild.PluginBuild) {
			epb.OnResolve(
				//__AviatorWrapped{UniqueName}_Runtime.svelte
				esbuild.OnResolveOptions{Filter: `^__AviatorWrapped.*_Runtime\.svelte$`},
				func(args esbuild.OnResolveArgs) (result esbuild.OnResolveResult, err error) {
					result.Namespace = "browserRuntime"
					result.Path = args.Path
					return result, nil
				},
			)
			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "browserRuntime"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {
					view := viewsByEntryPoint[args.Path]

					buf := bytes.Buffer{}
					err = browserGenerator.Execute(&buf, view)

					contents := buf.String()

					result.ResolveDir = b.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderTS
					return result, nil
				},
			)
		},
	}

}

func (b *BrowserBuilder) browserCompile(path string, code []byte) (*SvelteBuildOutput, error) {
	expr := fmt.Sprintf(
		`;__svelte__.compile({ "Path": %q, "code": %q, "target": "dom", "dev": %t, "css": true, "enableSourcemap": %t })`,
		path,
		code,
		true,
		true,
	)
	result, err := b.vm.Eval(path, expr)
	if err != nil {
		return nil, err
	}
	out := &SvelteBuildOutput{}
	if err := json.Unmarshal([]byte(result), out); err != nil {
		return nil, err
	}

	return out, nil
}
