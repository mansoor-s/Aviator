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
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

type SSRBuilder struct {
	vm          js.VM
	viewManager *ViewManager
	workingDir  string
}

type CompiledResult struct {
	JS  []byte
	CSS []byte
}

func NewSSRBuilder(vm js.VM, viewManager *ViewManager, workingDir string) *SSRBuilder {
	return &SSRBuilder{
		vm:          vm,
		viewManager: viewManager,
		workingDir:  workingDir,
	}
}

func (s *SSRBuilder) DevBuild(_ context.Context) (*CompiledResult, error) {
	result := esbuild.Build(esbuild.BuildOptions{
		//__aviator_ssr.js is a file created by ssrPlugin at build-time
		EntryPointsAdvanced: []esbuild.EntryPoint{
			{
				InputPath: "__aviator_ssr.js",
			},
		},
		AbsWorkingDir: s.workingDir,
		Outdir:        "./",
		Format:        esbuild.FormatIIFE,
		Platform:      esbuild.PlatformBrowser,
		GlobalName:    "__aviator__",
		Bundle:        true,
		Metafile:      true,
		LogLevel:      esbuild.LogLevelInfo,
		Plugins: []esbuild.Plugin{
			s.ssrPlugin(),
			s.wrappedComponentsPlugin(),
			s.svelteComponentsPlugin(),
			s.npmJsPathPlugin(),
		},
	})

	if len(result.Errors) > 0 {
		msgs := esbuild.FormatMessages(result.Errors, esbuild.FormatMessagesOptions{
			Color:         true,
			Kind:          esbuild.ErrorMessage,
			TerminalWidth: 80,
		})
		return nil, fmt.Errorf(strings.Join(msgs, "\n"))
	}

	compiledResult := &CompiledResult{
		JS: result.OutputFiles[0].Contents,
	}
	if len(result.OutputFiles) > 1 {
		compiledResult.CSS = result.OutputFiles[1].Contents
	}

	return compiledResult, nil
}

//go:embed ssrHelperTemplate.gotext
var ssrTemplate string

// ssrGenerator
var ssrGenerator = template.Must(template.New("ssrTemplate").Parse(ssrTemplate))

// Generate the virtual __aviator_ssr.js which includes a reference to all
// svelte components. __aviator_ssr.js serves as the entrypoint
// It will compile Go template file ssrHelperTemplate.gotext
func (s *SSRBuilder) ssrPlugin() esbuild.Plugin {
	viewList := s.viewManager.AllViews()
	return esbuild.Plugin{
		Name: "ssr",
		Setup: func(epb esbuild.PluginBuild) {
			epb.OnResolve(
				esbuild.OnResolveOptions{Filter: `^__aviator_ssr.js$`},
				func(args esbuild.OnResolveArgs) (result esbuild.OnResolveResult, err error) {
					result.Namespace = "ssr"
					result.Path = args.Path
					return result, nil
				},
			)
			epb.OnLoad(
				esbuild.OnLoadOptions{Filter: `.*`, Namespace: "ssr"},
				func(args esbuild.OnLoadArgs) (result esbuild.OnLoadResult, err error) {
					//this data is used to compile the .gotext template to get JS
					viewData := map[string]interface{}{
						"Views": viewList,
					}

					buf := bytes.Buffer{}
					err = ssrGenerator.Execute(&buf, viewData)
					if err != nil {
						return result, err
					}
					contents := buf.String()
					result.ResolveDir = s.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJS
					return result, nil
				},
			)
		},
	}
}

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

//wrappedComponentsPlugin creates a new virtual svelte component that
// composes all Svelte Components with all the layouts that apply to them
// i.e:  <RootLayout><FooLayout><MyComponent></MyComponent></FooLayout></RootLayout>
func (s *SSRBuilder) wrappedComponentsPlugin() esbuild.Plugin {
	//index views by their WrappedUniqueName for easier lookup in plugin
	viewsByWrappedName := make(map[string]*View)
	for _, view := range s.viewManager.AllViews() {
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

					compiledCode, err := s.ssrCompile(args.Path, []byte(rawVirtualCode))
					if err != nil {
						return result, err
					}

					contents := compiledCode.JS
					result.ResolveDir = s.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}

//svelteComponentsPlugin handles .svelte files both inside the project and node_modules
func (s *SSRBuilder) svelteComponentsPlugin() esbuild.Plugin {
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

					compiledCode, err := s.ssrCompile(newPath, rawCode)
					if err != nil {
						return result, err
					}

					contents := compiledCode.JS
					result.ResolveDir = s.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJSX
					return result, nil
				},
			)
		},
	}
}

func (s *SSRBuilder) npmJsPathPlugin() esbuild.Plugin {
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
					result.ResolveDir = s.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderJS
					return result, nil
				},
			)
		},
	}
}

type SvelteBuildOutput struct {
	JS  string
	CSS string
}

//ssrCompile compiles a compiled
func (s *SSRBuilder) ssrCompile(path string, code []byte) (*SvelteBuildOutput, error) {
	expr := fmt.Sprintf(
		`;__svelte__.compile({ "path": %q, "code": %q, "target": "ssr", "dev": %t, "css": false })`,
		path,
		code,
		true,
	)
	result, err := s.vm.Eval(context.Background(), path, expr)
	if err != nil {
		return nil, err
	}
	outputStruct := &SvelteBuildOutput{}
	if err := json.Unmarshal([]byte(result), outputStruct); err != nil {
		return nil, err
	}

	return outputStruct, nil
}
