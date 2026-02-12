package host

import (
	"context"
	"github.com/gmbytes/snow/core/xsync"
)

type IHostApplication interface {
	OnStarted(listener func())
	OnStopped(listener func())
	OnStopping(listener func())

	StopApplication()
}

func Run(h IHost) {
	app := GetRoutine[IHostApplication](h.GetRoutineProvider())
	ctx, cancel := context.WithCancel(context.Background())

	started := false
	app.OnStopping(func() {
		cancel()
	})
	app.OnStarted(func() {
		started = true
	})

	wg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, wg)
	wg.Wait()

	<-ctx.Done()

	if started {
		wg = xsync.NewTimeoutWaitGroup()
		h.Stop(context.Background(), wg)
		wg.Wait()
	}
}
