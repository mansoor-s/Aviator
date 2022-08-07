package builder

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/mansoor-s/aviator/js"
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
			wrappedComponentsPlugin(s.workingDir, s.viewManager, s.ssrCompile),
			svelteComponentsPlugin(s.workingDir, s.ssrCompile),
			npmJsPathPlugin(s.workingDir),
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
