package js

import (
	"errors"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

type gojaVM struct {
	runtime *goja.Runtime
	//pool     *puddle.Pool
	preCompiled map[string]*goja.Program
}

//var _ VM = &gojaVM{}

func newGojaVM() (*gojaVM, error) {
	runtime := goja.New()
	new(require.Registry).Enable(runtime)
	console.Enable(runtime)
	return &gojaVM{
		runtime:     runtime,
		preCompiled: make(map[string]*goja.Program),
	}, nil
}

func (g *gojaVM) PreCompile(uniqueName string, source string) error {
	compile, err := goja.Compile(uniqueName, source, false)
	if err != nil {
		return err
	}

	g.preCompiled[uniqueName] = compile

	return nil
}

func (g *gojaVM) RunScript(uniqueName string) (string, error) {
	prog, ok := g.preCompiled[uniqueName]
	if !ok {
		return "", errors.New("couldn't find compiled script with name : \"" + uniqueName + "\"")
	}

	outputVal, err := g.runtime.RunProgram(prog)
	if err != nil {
		return "", err
	}

	return outputVal.String(), nil
}

func (g *gojaVM) Eval(path, source string) (string, error) {
	val, err := g.runtime.RunScript(path, source)
	if err != nil {
		return "", err
	}

	if val == nil {
		return "", nil
	}

	return val.String(), nil

}

/*
func (g *gojaVM) InitializationScript(path, script string) error {
	return nil
}

func (g *gojaVM) Close() {

}
*/
