package builder

import (
	"context"
	"encoding/json"
	"fmt"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/mansoor-s/aviator/js"
	"github.com/mansoor-s/aviator/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
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

//BuildDev creates assets for embedding into the rendered view
// It persists the assets into the output directory and adds the names
// to the relevant view
func (b *BrowserBuilder) BuildDev(ctx context.Context) error {
	views := b.viewManager.AllViews()
	viewsByEntryPoint := make(map[string]*View, len(views))

	var entryPoints []esbuild.EntryPoint

	for _, view := range views {
		//skip layouts as entrypoints
		if view.IsLayout {
			continue
		}
		entryPath := view.WrappedUniqueName + ".svelte"
		outputPrettyName := view.UniqueName + ".svelte"
		entryPoints = append(entryPoints, esbuild.EntryPoint{
			InputPath:  entryPath,
			OutputPath: filepath.Join(b.outputDir, outputPrettyName),
		})

		viewsByEntryPoint[outputPrettyName] = view
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
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		LogLevel:          esbuild.LogLevelInfo,
		Plugins: append(
			[]esbuild.Plugin{
				b.wrappedComponentsPlugin(),
				b.svelteComponentsPlugin(),
				b.npmJsPathPlugin(),
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

	//delete all old generated files
	err := utils.RemoveDirContents(b.outputDir)
	if err != nil {
		return err
	}

	for _, file := range result.OutputFiles {
		fileName := filepath.Base(file.Path)
		extension := utils.FileExtension(fileName)
		viewRefName := fileName[:len(fileName)-len(extension)-1]
		view := viewsByEntryPoint[viewRefName]

		//skip if no view is directly associated with this "chunk" file
		if view != nil {
			if extension == "js" {
				view.JSImports = append(view.JSImports, fileName)
			} else if extension == "css" {
				view.CSSImports = append(view.CSSImports, fileName)
			}
		}

		//save files to outputDir
		err := os.WriteFile(filepath.Join(b.outputDir, fileName), file.Contents, 775)
		if err != nil {
			return err
		}
	}

	return nil
}

//wrappedComponentsPlugin creates a new virtual svelte component that
// composes all Svelte Components with all the layouts that apply to them
// i.e:  <RootLayout><FooLayout><MyComponent></MyComponent></FooLayout></RootLayout>
func (b *BrowserBuilder) wrappedComponentsPlugin() esbuild.Plugin {
	//index views by their WrappedUniqueName for easier lookup in plugin
	viewsByWrappedName := make(map[string]*View)
	for _, view := range b.viewManager.AllViews() {
		viewsByWrappedName[view.WrappedUniqueName] = view
	}

	return esbuild.Plugin{
		Name: "wrappedComponents",
		Setup: func(epb esbuild.PluginBuild) {
			epb.OnResolve(
				//__AviatorWrapped{UniqueName}.svelte
				esbuild.OnResolveOptions{Filter: `^__AviatorWrapped_.*\.svelte$`},
				func(args esbuild.OnResolveArgs) (result esbuild.OnResolveResult, err error) {
					result.Namespace = "wrappedComponents"
					result.Path = args.Path
					return result, nil
				},
			)

			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "wrappedComponents"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {

					//get the wrapped unique name by removing the extension
					wrappedName := args.Path
					fileExt := filepath.Ext(args.Path)
					if len(fileExt) > 0 {
						wrappedName = wrappedName[:len(wrappedName)-len(fileExt)]
					}

					view, ok := viewsByWrappedName[wrappedName]
					if !ok {
						return result, fmt.Errorf(
							"unable to find wrapped component named: %s", wrappedName,
						)
					}

					rawVirtualCode := createLayoutWrappedView(view)

					compiledCode, err := b.browserCompile(args.Path, []byte(rawVirtualCode))
					if err != nil {
						return result, err
					}

					contents := compiledCode.JS
					result.ResolveDir = b.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}

//svelteComponentsPlugin handles .svelte files both inside the project and node_modules
func (b *BrowserBuilder) svelteComponentsPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "svelte",
		Setup: func(epb esbuild.PluginBuild) {
			epb.OnResolve(
				esbuild.OnResolveOptions{Filter: `^.*\.svelte$`},
				func(args esbuild.OnResolveArgs) (result esbuild.OnResolveResult, err error) {
					callerPath := filepath.Dir(args.Importer)
					var absPath string
					if callerPath == "." {
						absPath = path.Join(args.ResolveDir, args.Path)
					} else {
						absPath, err = filepath.Abs(path.Join(callerPath, args.Path))
						if err != nil {
							return result, err
						}
					}

					result.Path = absPath
					result.Namespace = "svelte"
					return result, nil
				})

			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "svelte"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {

					rawCode, err := os.ReadFile(args.Path)
					if err != nil {
						return result, err
					}

					newPath := utils.PathPascalCase(filepath.Base(args.Path))

					compiledCode, err := b.browserCompile(newPath, rawCode)
					if err != nil {
						return result, err
					}

					contents := compiledCode.JS
					result.ResolveDir = b.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}

func (b *BrowserBuilder) npmJsPathPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "js_path",
		Setup: func(epb esbuild.PluginBuild) {
			//handles imports that are JS files, but for some reason the import path doesn't
			//include the .js suffix
			epb.OnResolve(
				esbuild.OnResolveOptions{Filter: `\.+\/(.*\/)?[a-zA-Z0-9]+$`},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					var result esbuild.OnResolveResult

					importedFilePath := args.Path

					fileExt := filepath.Ext(args.Path)
					if len(fileExt) == 0 {
						importedFilePath += ".js"
					}

					callerPath := filepath.Dir(args.Importer)
					absPath, err := filepath.Abs(path.Join(callerPath, importedFilePath))
					if err != nil {
						return result, err
					}

					result.Path = absPath
					result.Namespace = "js_path"

					return result, nil
				},
			)

			epb.OnResolve(
				esbuild.OnResolveOptions{Filter: `^.*\.js$`},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					var result esbuild.OnResolveResult

					callerPath := filepath.Dir(args.Importer)
					absPath, err := filepath.Abs(path.Join(callerPath, args.Path))
					if err != nil {
						return result, err
					}

					result.Path = absPath
					result.Namespace = "js_path"

					return result, nil
				},
			)

			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "js_path"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {
					rawCode, err := os.ReadFile(args.Path)
					if err != nil {
						return result, err
					}

					contents := string(rawCode)
					result.ResolveDir = b.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJS
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
