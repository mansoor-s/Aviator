package builder

import (
	"bytes"
	"context"
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

type BrowserImports struct {
	JS  []string
	CSS []string
}

type BrowserBuilder struct {
	vm          js.VM
	viewManager *ViewManager

	outputDir  string
	workingDir string
}

func NewBrowserBuilder(
	vm js.VM,
	viewManager *ViewManager,
	workingDir, outputDir string,
) *BrowserBuilder {
	return &BrowserBuilder{
		vm:          vm,
		workingDir:  workingDir,
		outputDir:   outputDir,
		viewManager: viewManager,
	}
}

// The entrypoints are the virtual files created for all Components in the
// browserRuntimePlugin func. This plugin will reference those virtual files
// and will bundle and persist the outputs

//BuildDev creates assets for embedding into the rendered view
// It persists the assets into the output directory
func (b *BrowserBuilder) BuildDev(_ context.Context) error {
	views := b.viewManager.AllViews()
	viewsByEntryPoint := make(map[string]*View, len(views))
	viewsByOutputName := make(map[string]*View, len(views))

	var entryPoints []esbuild.EntryPoint

	for _, view := range views {
		//skip layouts as entrypoints
		if view.IsLayout {
			continue
		}

		entryPath := view.WrappedUniqueName + "_Runtime.svelte"
		outputPrettyName := view.UniqueName + ".svelte"
		entryPoints = append(entryPoints, esbuild.EntryPoint{
			InputPath:  entryPath,
			OutputPath: filepath.Join(b.outputDir, outputPrettyName),
		})
		viewsByOutputName[outputPrettyName] = view
		viewsByEntryPoint[entryPath] = view
	}

	result := esbuild.Build(esbuild.BuildOptions{
		EntryPointsAdvanced: entryPoints,
		Outdir:              b.outputDir,
		AbsWorkingDir:       b.workingDir,
		ChunkNames:          "[name]-[hash]",
		Format:              esbuild.FormatESModule,
		Platform:            esbuild.PlatformBrowser,
		// Add "import" condition to support svelte/internal
		// https://esbuild.github.io/api/#how-conditions-work
		Conditions:        []string{"browser", "default", "import"},
		Metafile:          false,
		Bundle:            true,
		Splitting:         true,
		MinifyIdentifiers: false,
		MinifySyntax:      false,
		MinifyWhitespace:  false,
		LogLevel:          esbuild.LogLevelInfo,
		Plugins: append(
			[]esbuild.Plugin{
				b.browserRuntimePlugin(viewsByEntryPoint),
				wrappedComponentsPlugin(b.workingDir, b.viewManager, b.browserCompile),
				svelteComponentsPlugin(b.workingDir, b.browserCompile),
				npmJsPathPlugin(b.workingDir),
			},
		),
		Write: true,
	})
	if len(result.Errors) > 0 {
		msgs := esbuild.FormatMessages(result.Errors, esbuild.FormatMessagesOptions{
			Color:         true,
			Kind:          esbuild.ErrorMessage,
			TerminalWidth: 80,
		})
		return fmt.Errorf(strings.Join(msgs, "\n"))
	}

	//delete all old generated files
	err := utils.RemoveDirContents(b.outputDir)
	if err != nil {
		return err
	}

	for _, file := range result.OutputFiles {
		fileName := filepath.Base(file.Path)
		extension := utils.FileExtension(fileName)
		viewRefName := fileName[:len(fileName)-len(extension)-1]
		view := viewsByOutputName[viewRefName]

		//skip if no view is directly associated with this "chunk" file
		if view != nil {
			if extension == "js" {
				view.JSImports = append(view.JSImports, fileName)
			} else if extension == "css" {
				view.CSSImports = append(view.CSSImports, fileName)
			}
		}

		//save files to outputDir
		//err := os.WriteFile(filepath.Join(b.outputDir, fileName), file.Contents, 775)
		//if err != nil {
		//	return err
		//}
	}

	return nil
}

//go:embed browserHelperTemplate.gotext
var browserTemplate string

var browserGenerator = template.Must(template.New("browserTemplate").Parse(browserTemplate))

//browserRuntimePlugin renders the browserTemplate for each component
//The rendered content acts as the entrypoint that are used for the esbuild and
//also imported by each of the view in the final HTML
func (b *BrowserBuilder) browserRuntimePlugin(viewsByOutputName map[string]*View) esbuild.Plugin {
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
					view := viewsByOutputName[args.Path]

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
		`;__svelte__.compile({ "path": %q, "code": %q, "target": "dom", "dev": %t, "css": true })`,
		path,
		code,
		true,
	)
	result, err := b.vm.Eval(context.Background(), path, expr)
	if err != nil {
		return nil, err
	}
	out := &SvelteBuildOutput{}
	if err := json.Unmarshal([]byte(result), out); err != nil {
		return nil, err
	}

	return out, nil
}
