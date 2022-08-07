package js

import (
	"context"
	"github.com/jackc/puddle"
)

// VM for evaluating javascript
type VM interface {
	InitializationScript(ctx context.Context, path, script string) error
	Eval(ctx context.Context, path, expression string) (string, error)
	Close()
}

type v8VMPool struct {
	//v8   *V8VM
	poolSize int
	pool     *puddle.Pool
}

var _ VM = &v8VMPool{}

// NewV8VMPool creates a new VM pool. it will run initScript script on every newly created script
// for our use-case, that means compiling an instance of the svelte compiler so that doesn't have
// to happen for every request
func NewV8VMPool(poolSize int, initScript string) (*v8VMPool, error) {
	constructorFn := func(ctx context.Context) (interface{}, error) {
		vm, err := newV8VM()
		if err != nil {
			return nil, err
		}
		/*if len(initScript) > 0 {
			err := vm.InitializationScript(context.Background(), "__aviator_vm_init_script", initScript)

			if err != nil {
				return nil, err
			}
		}
		*/

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

	return v8Pool, err
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

	return vm.Eval(ctx, path, expression)
}

func (p *v8VMPool) Close() {
	p.pool.Close()
}
