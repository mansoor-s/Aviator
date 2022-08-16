package builder

import (
	"fmt"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/mansoor-s/aviator/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const wrappedScriptFmt = "<script>\n%s\n</script>\n"
const wrappedImportStatementFmt = "import %s from \"%s\""

func createLayoutWrappedView(view *View) string {
	layouts := view.ApplicableLayoutViews

	var importStatements []string
	var startTags []string
	var endTags []string

	for _, layout := range layouts {
		importStatement := fmt.Sprintf(wrappedImportStatementFmt, layout.UniqueName, layout.RelPath)
		importStatements = append(importStatements, importStatement)

		startStr := `<` + layout.UniqueName + `>`
		startTags = append(startTags, startStr)

		endStr := `</` + layout.UniqueName + `>`
		endTags = append([]string{endStr}, endTags...)
	}
	importStatement := fmt.Sprintf(wrappedImportStatementFmt, view.UniqueName, view.RelPath)
	importStatements = append(importStatements, importStatement)

	componentStr := `<svelte:component this={` + view.UniqueName + `}  {...($$props || {})}/>`

	wrappedComponentStr := strings.Join(startTags, "") +
		componentStr +
		strings.Join(endTags, "")

	allImportStatements := strings.Join(importStatements, "\n")
	wrappedSvelteComponent := fmt.Sprintf(wrappedScriptFmt, allImportStatements)

	return wrappedSvelteComponent + wrappedComponentStr
}

func npmJsPathPlugin(workingDir string) esbuild.Plugin {
	return esbuild.Plugin{
		Name: "js_path",
		Setup: func(epb esbuild.PluginBuild) {
			//handles imports that are JS files, but for some reason the import Path doesn't
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
					result.ResolveDir = workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJS
					return result, nil
				},
			)
		},
	}
}

type SvelteCompilerFunc func(string, []byte) (*SvelteBuildOutput, error)

//wrappedComponentsPlugin creates a new virtual svelte component that
// composes all Svelte Components with all the layouts that apply to them
// i.e:  <RootLayout><FooLayout><MyComponent></MyComponent></FooLayout></RootLayout>
func wrappedComponentsPlugin(
	cache *cacheManager,
	workingDir string,
	allViews []*View,
	compilerFunc SvelteCompilerFunc,
) esbuild.Plugin {
	//index views by their WrappedUniqueName for easier lookup in plugin
	viewsByWrappedName := make(map[string]*View)
	for _, view := range allViews {
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

					/*
						err = cache.DependsOn(args.Importer, args.Path)
						if err != nil {
							return result, err
						}*/

					return result, nil
				},
			)

			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "wrappedComponents"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {
					var contents *string

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

					compiledCode, err := compilerFunc(args.Path, []byte(rawVirtualCode))
					if err != nil {
						return result, err
					}

					contents = &compiledCode.JSCode
					cache.AddCache(args.Path, contents)

					result.ResolveDir = workingDir
					result.Contents = contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}

//svelteComponentsPlugin handles .svelte files both inside the project and node_modules
func svelteComponentsPlugin(
	cache *cacheManager,
	workingDir string,
	compilerFunc SvelteCompilerFunc,
) esbuild.Plugin {
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

					err = cache.DependsOn(args.Importer, absPath)
					if err != nil {
						return result, err
					}

					result.Path = absPath
					result.Namespace = "svelte"
					return result, nil
				})

			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "svelte"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {
					var contents *string

					cachedContent := cache.GetContent(args.Path)
					//cache miss
					if cachedContent == nil {
						rawCode, err := os.ReadFile(args.Path)
						if err != nil {
							return result, err
						}

						newPath := utils.PathPascalCase(filepath.Base(args.Path))

						compiledCode, err := compilerFunc(newPath, rawCode)
						if err != nil {
							return result, err
						}
						//contents = &compiledCode.JSCode

						compiledContent := compiledCode.JSCode +
							`//# sourceMappingURL=` +
							compiledCode.JSSourceMap

						contents = &compiledContent

						cache.AddCache(args.Path, &compiledContent)
					} else {
						contents = cachedContent
					}

					result.ResolveDir = workingDir
					result.Contents = contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}
