package host

import (
	"context"

	"github.com/gmbytes/snow/pkg/xsync"
)

type IHostApplication interface {
	OnStarted(listener func())
	OnStopped(listener func())
	OnStopping(listener func())

	StopApplication()
}

// Run 启动 Host 并阻塞等待应用停止信号。
// startCtx 仅用于启动阶段，启动完成后 Run 继续阻塞等待 OnStopping 信号，
// 不会因 startCtx 超时而自动退出。停止阶段使用 context.Background()。
// 如需独立控制停止阶段的超时预算，请使用 RunWithStopContext。
func Run(h IHost, ctx context.Context) {
	RunWithStopContext(h, ctx, context.Background())
}

// RunWithStopContext 启动 Host 并阻塞等待应用停止信号，允许分别控制启动和停止阶段的上下文。
// startCtx 仅用于启动阶段（Start 调用）；stopCtx 用于停止阶段（Stop 调用），控制优雅关闭的超时预算。
// 两个参数为 nil 时均回退到 context.Background()。
func RunWithStopContext(h IHost, startCtx, stopCtx context.Context) {
	if startCtx == nil {
		startCtx = context.Background()
	}
	if stopCtx == nil {
		stopCtx = context.Background()
	}

	app := GetRoutine[IHostApplication](h.GetRoutineProvider())
	runCtx, cancel := context.WithCancel(startCtx)

	started := false
	app.OnStopping(func() {
		cancel()
	})
	app.OnStarted(func() {
		started = true
	})

	wg := xsync.NewTimeoutWaitGroup()
	h.Start(runCtx, wg)
	wg.Wait()

	// startCtx 取消仅触发 OnStopping → cancel()，不阻塞这里
	select {
	case <-startCtx.Done():
		app.StopApplication()
	default:
	}

	<-runCtx.Done()

	if started {
		wg = xsync.NewTimeoutWaitGroup()
		h.Stop(stopCtx, wg)
		wg.Wait()
	}
}
