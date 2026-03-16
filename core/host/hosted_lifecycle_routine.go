package host

import (
	"context"
	"github.com/gmbytes/snow/core/xsync"
)

type IHostedLifecycleRoutine interface {
	IHostedRoutine

	BeforeStart(ctx context.Context, wg *xsync.TimeoutWaitGroup)
	AfterStart(ctx context.Context, wg *xsync.TimeoutWaitGroup)
	BeforeStop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
	AfterStop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
}

type IHostedLifecycleRoutineContainer interface {
	AddHostedLifecycleRoutine(factory func() IHostedLifecycleRoutine)
	BuildHostedLifecycleRoutines()
	GetHostedLifecycleRoutines() []IHostedLifecycleRoutine
}

func AddHostedLifecycleRoutine[U IHostedLifecycleRoutine](builder IBuilder) {
	provider := builder.GetRoutineProvider()
	container := GetRoutine[IHostedLifecycleRoutineContainer](provider)

	container.AddHostedLifecycleRoutine(func() IHostedLifecycleRoutine {
		s := NewStruct[U]()
		Inject(provider.GetRootScope(), s)
		return s
	})
}
