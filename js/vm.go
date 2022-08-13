package js

import (
	"context"
	"github.com/jackc/puddle"
)

// VM for evaluating javascript
type VM interface {
	//PreCompile(uniqueName string, source string) error
	RunScript(uniqueName string) (string, error)
	InitializationScript(path, source string) error
	Eval(path, expression string) (string, error)
	//Close()
}

/*
type v8VMPool struct {
	//v8   *V8VM
	poolSize int
	pool     *puddle.Pool
}

//var _ VM = &v8VMPool{}

// NewV8VMPool creates a new VM pool. it will run initScript script on every newly created script
// for our use-case, that means compiling an instance of the svelte compiler so that doesn't have
// to happen for every request
func NewV8VMPool(poolSize int, initScript string) (*v8VMPool, error) {
	constructorFn := func(ctx context.Context) (interface{}, error) {
		vm, err := newV8VM()
		if err != nil {
			return nil, err
		}

		return vm, nil
	}

	destructorFn := func(res interface{}) {
		res.(*V8VM).Close()
	}

	pool := puddle.NewPool(constructorFn, destructorFn, int32(poolSize))

	//allocate full pool size
	for i := 0; i < poolSize; i++ {
		err := pool.CreateResource(context.Background())
		if err != nil {
			return nil, err
		}
	}

	v8Pool := &v8VMPool{
		pool:     pool,
		poolSize: poolSize,
	}

	err := v8Pool.InitializationScript(
		context.Background(),
		"__aviator_vm_init_script",
		initScript,
	)
	if err != nil {
		return nil, newV8JSError(err)
	}

	return v8Pool, nil
}

//InitializationScript runs an initialization script on all VM instances
func (p *v8VMPool) InitializationScript(ctx context.Context, path, script string) error {
	//acquire all VMs, so they aren't released before initialization is completed
	var allVMResources []*puddle.Resource

	for i := 0; i < p.poolSize; i++ {
		res, err := p.pool.Acquire(ctx)
		if err != nil {
			return err
		}
		allVMResources = append(allVMResources, res)
	}

	for i := 0; i < p.poolSize; i++ {
		res := allVMResources[i]
		vm := res.Value().(*V8VM)
		err := vm.InitializationScript(ctx, path, script)
		if err != nil {
			return err
		}
	}

	for i := 0; i < p.poolSize; i++ {
		allVMResources[i].Release()
	}

	return nil
}

func (p *v8VMPool) Eval(ctx context.Context, path, expression string) (string, error) {
	res, err := p.pool.Acquire(ctx)
	defer res.Release()
	if err != nil {
		return "", err
	}

	vm := res.Value().(*V8VM)

	val, err := vm.Eval(ctx, path, expression)
	if err != nil {
		return "", newV8JSError(err)
	}

	return val, nil
}

func (p *v8VMPool) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}
*/
type gojaVMPool struct {
	poolSize int

	pool *puddle.Pool
}

var _ VM = &gojaVMPool{}

func NewGojaVMPool(poolSize int) (*gojaVMPool, error) {
	constructorFn := func(ctx context.Context) (interface{}, error) {
		vm, err := newGojaVM()
		if err != nil {
			return nil, err
		}

		return vm, nil
	}

	destructorFn := func(res interface{}) {
		//noop
	}

	pool := puddle.NewPool(constructorFn, destructorFn, int32(poolSize))

	return &gojaVMPool{
		poolSize: poolSize,
		pool:     pool,
	}, nil
}

func (g *gojaVMPool) RunScript(uniqueName string) (string, error) {
	res, err := g.pool.Acquire(context.Background())
	defer res.Release()
	if err != nil {
		return "", err
	}

	vm := res.Value().(*gojaVM)

	return vm.RunScript(uniqueName)
}

func (g *gojaVMPool) Eval(path, source string) (string, error) {
	res, err := g.pool.Acquire(context.Background())
	defer res.Release()
	if err != nil {
		return "", err
	}

	vm := res.Value().(*gojaVM)

	return vm.Eval(path, source)
}

//InitializationScript runs an initialization script on all VM instances
func (g *gojaVMPool) InitializationScript(path, source string) error {
	//acquire all VMs, so they aren't released before initialization is completed
	var allVMResources []*puddle.Resource

	for i := 0; i < g.poolSize; i++ {
		res, err := g.pool.Acquire(context.Background())
		if err != nil {
			return err
		}
		allVMResources = append(allVMResources, res)
	}

	for i := 0; i < g.poolSize; i++ {
		res := allVMResources[i]
		vm := res.Value().(*gojaVM)
		_, err := vm.Eval(path, source)

		if err != nil {
			return err
		}
	}

	for i := 0; i < g.poolSize; i++ {
		allVMResources[i].Release()
	}

	return nil
}

func (g *gojaVMPool) Close() {
	//noop
}
