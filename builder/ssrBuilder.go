package builder

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"text/template"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/mansoor-s/aviator/js"
	"github.com/mansoor-s/aviator/utils"
)

type SSRBuilder struct {
	vm         js.VM
	logger     utils.Logger
	workingDir string
	cache      Cache
}

type CompiledResult struct {
	JS        []byte
	CSS       []byte
	SourceMap []byte
}

func NewSSRBuilder(
	logger utils.Logger,
	vm js.VM,
	cache Cache,
	workingDir string,
) *SSRBuilder {
	return &SSRBuilder{
		logger:     logger,
		vm:         vm,
		workingDir: workingDir,
		cache:      cache,
	}
}

func (s *SSRBuilder) DevBuild(allViews []*View) (*CompiledResult, error) {
	allEntryPointViews := []*View{}
	for _, view := range allViews {
		if view.IsEntrypoint {
			allEntryPointViews = append(allEntryPointViews, view)
		}
	}

	cssCache := &sync.Map{}

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
		Metafile:      false,
		LogLevel:      esbuild.LogLevelInfo,
		Sourcemap:     esbuild.SourceMapInline,
		Target:        esbuild.ES2015,
		Plugins: []esbuild.Plugin{
			s.ssrPlugin(allEntryPointViews),
			wrappedComponentsPlugin(s.cache, s.workingDir, allViews, s.ssrCompile),
			svelteComponentsPlugin(s.cache, s.workingDir, cssCache, s.ssrCompile),
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
	s.cache.Finished()

	compiledResult := &CompiledResult{
		//SourceMap: result.OutputFiles[0].Contents,
		JS: result.OutputFiles[0].Contents,
	}
	//css is generated in the browser builder
	/*
		if len(result.OutputFiles) > 1 {
			compiledResult.CSS = result.OutputFiles[1].Contents
		}*/

	return compiledResult, nil
}

//go:embed ssrHelperTemplate.gotext
var ssrTemplate string

// ssrGenerator
var ssrGenerator = template.Must(template.New("ssrTemplate").Parse(ssrTemplate))

// Generate the virtual __aviator_ssr.js which includes a reference to all
// svelte components. __aviator_ssr.js serves as the entrypoint
// It will compile Go template file ssrHelperTemplate.gotext
func (s *SSRBuilder) ssrPlugin(allEntryPointViews []*View) esbuild.Plugin {
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
						"Views": allEntryPointViews,
					}

					buf := bytes.Buffer{}
					err = ssrGenerator.Execute(&buf, viewData)
					if err != nil {
						return result, err
					}
					contents := buf.String()
					result.ResolveDir = s.workingDir
					result.Contents = &contents
					result.Loader = esbuild.LoaderTS
					return result, nil
				},
			)
		},
	}
}

type SvelteBuildOutput struct {
	CSSCode string
	JSCode  string

	JSSourceMap  string
	CSSSourceMap string
}

// ssrCompile compiles a compiled
func (s *SSRBuilder) ssrCompile(path string, code []byte) (*SvelteBuildOutput, error) {
	format := `__svelte__.compile({ "Path": %q, "code": %q, "target": "ssr", "dev": %t, "css": false, "enableSourcemap": %t, "isHydratable": %t })`
	expr := fmt.Sprintf(
		format,
		path,
		code,
		false,
		false,
		false,
	)
	result, err := s.vm.Eval(path, expr)
	if err != nil {
		return nil, err
	}
	outputStruct := &SvelteBuildOutput{}
	if err := json.Unmarshal([]byte(result), outputStruct); err != nil {
		return nil, err
	}

	return outputStruct, nil
}
