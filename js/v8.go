package js

/*
import (
	"context"
	"errors"
	"go.kuoruan.net/v8go-polyfills/base64"
	"go.kuoruan.net/v8go-polyfills/console"
	"go.kuoruan.net/v8go-polyfills/fetch"
	"go.kuoruan.net/v8go-polyfills/url"
	"os"
	"rogchap.com/v8go"
)

type V8VM struct {
	context *v8go.Context
}

type v8Error v8go.JSError

func (e v8Error) Error() string {
	return e.Message
}

func (e v8Error) ErrorStackTrace() string {
	return e.StackTrace
}

func newV8VM() (*V8VM, error) {
	var v8Ctx *v8go.Context

	isolate := v8go.NewIsolate()
	if isolate == nil {
		return nil, errors.New("unable to create a new V8 isolate")
	}

	var err error
	defer func() {
		if err != nil {
			//clean up on error
			isolate.TerminateExecution()
			isolate.Dispose()
			if v8Ctx != nil {
				v8Ctx.Close()
			}
		}
	}()

	//TODO: why is this needed?
	global := v8go.NewObjectTemplate(isolate)

	err = base64.InjectTo(isolate, global)
	if err != nil {
		return nil, err
	}

	// Fetch support
	err = fetch.InjectTo(isolate, global)
	if err != nil {
		return nil, err
	}

	v8Ctx = v8go.NewContext(isolate, global)
	if v8Ctx == nil {
		return nil, errors.New("unable to create a new V8 context")
	}

	// URL support
	err = url.InjectTo(v8Ctx)
	if err != nil {
		return nil, err
	}

	// Console support
	err = console.InjectMultipleTo(v8Ctx,
		console.NewConsole(console.WithOutput(os.Stderr), console.WithMethodName("error")),
		console.NewConsole(console.WithOutput(os.Stderr), console.WithMethodName("warn")),
		console.NewConsole(console.WithOutput(os.Stdout), console.WithMethodName("log")),
	)
	if err != nil {
		return nil, err
	}

	return &V8VM{
		context: v8Ctx,
	}, nil
}

//var _ VM = (*V8VM)(nil)

//InitializationScript compiles and runs a script into the context's isolate
func (vm *V8VM) InitializationScript(_ context.Context, path, code string) error {
	script, err := vm.context.Isolate().CompileUnboundScript(code, path, v8go.CompileOptions{})
	if err != nil {
		return err
	}
	// Bind to the context
	if _, err := script.Run(vm.context); err != nil {
		return err
	}
	return nil
}

//Eval runs the specified script. The script output MUST be a string.
// if the return value is a JS object, it should be return with the output of JSON.stringify()
func (vm *V8VM) Eval(_ context.Context, path, expr string) (string, error) {
	value, err := vm.context.RunScript(expr, path)
	if err != nil {
		return "", err
	}
	// Handle promises
	if value.IsPromise() {
		prom, err := value.AsPromise()
		if err != nil {
			return "", err
		}
		// TODO: this could run forever
		for prom.State() == v8go.Pending {
			continue
		}
		return prom.Result().String(), nil
	}
	return value.String(), nil
}

func (vm *V8VM) Close() {
	vm.context.Close()
	vm.context.Isolate().TerminateExecution()
	vm.context.Isolate().Dispose()
}
*/
