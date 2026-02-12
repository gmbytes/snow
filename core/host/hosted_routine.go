package host

import (
	"context"
	"github.com/gmbytes/snow/core/injection"
	"github.com/gmbytes/snow/core/xsync"
)

type IHostedRoutine interface {
	Start(ctx context.Context, wg *xsync.TimeoutWaitGroup)
	Stop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
}

type IHostedRoutineContainer interface {
	AddHostedRoutine(factory func() IHostedRoutine)
	BuildHostedRoutines()
	GetHostedRoutines() []IHostedRoutine
}

func AddHostedRoutine[U IHostedRoutine](builder IBuilder) {
	provider := builder.GetRoutineProvider()
	container := GetRoutine[IHostedRoutineContainer](provider)

	AddSingleton[U](builder)
	container.AddHostedRoutine(func() IHostedRoutine {
		return injection.GetRoutine[U](provider)
	})
}
