package internal

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/injection"
	"github.com/gmbytes/snow/pkg/logging"
	"github.com/gmbytes/snow/pkg/option"
	"github.com/gmbytes/snow/pkg/xsync"
)

var _ host.IHost = (*Host)(nil)

type HostOption struct {
	StartWaitTimeoutSeconds int
	StopWaitTimeoutSeconds  int
}

type Host struct {
	option                          *HostOption
	logger                          logging.ILogger
	provider                        injection.IRoutineProvider
	app                             *HostApplication
	hostedRoutineContainer          host.IHostedRoutineContainer
	hostedRoutines                  []host.IHostedRoutine
	hostedLifecycleRoutineContainer host.IHostedLifecycleRoutineContainer
	hostedLifecycleRoutines         []host.IHostedLifecycleRoutine
}

func NewHost(provider injection.IRoutineProvider) *Host {
	return &Host{provider: provider}
}

func (ss *Host) Construct(opt *option.Option[*HostOption], logger *logging.Logger[Host]) {
	ss.option = opt.Get()
	if ss.option.StartWaitTimeoutSeconds == 0 {
		ss.option.StartWaitTimeoutSeconds = 5
	}
	if ss.option.StopWaitTimeoutSeconds == 0 {
		ss.option.StopWaitTimeoutSeconds = 8
	}

	ss.logger = logger.Get(func(data *logging.LogData) {
		data.Name = "Host"
		data.ID = fmt.Sprintf("%X", unsafe.Pointer(ss))
	})
}

func (ss *Host) startTimeout() time.Duration {
	timeout := 5 * time.Second
	if ss.option != nil {
		timeout = time.Duration(ss.option.StartWaitTimeoutSeconds) * time.Second
	}
	return timeout
}

func (ss *Host) stopTimeout() time.Duration {
	timeout := 8 * time.Second
	if ss.option != nil {
		timeout = time.Duration(ss.option.StopWaitTimeoutSeconds) * time.Second
	}
	return timeout
}

func (ss *Host) runLifecycleHook(
	ctx context.Context,
	routines []host.IHostedLifecycleRoutine,
	hookName string,
	timeout time.Duration,
	fn func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup),
) {
	if len(routines) == 0 {
		return
	}
	routineWg := xsync.NewTimeoutWaitGroup()
	routineWg.Add(len(routines))
	for _, routine := range routines {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					if ss.logger != nil {
						ss.logger.Errorf("panic in %s: %v", hookName, err)
					}
				}
				routineWg.Done()
			}()
			fn(routine, ctx, routineWg)
		}()
	}
	if !routineWg.WaitTimeout(timeout) {
		if ss.logger != nil {
			ss.logger.Warnf("'%s' wait timeout in hosted lifecycle routines", hookName)
		}
	}
}

type lifecycleRoutineAction func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup)
type hostedRoutineAction func(r host.IHostedRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup)

func (ss *Host) runPhase(
	ctx context.Context,
	timeout time.Duration,
	phaseName string,
	lifecycleAction lifecycleRoutineAction,
	routineAction hostedRoutineAction,
) {
	routineWg := xsync.NewTimeoutWaitGroup()

	if len(ss.hostedLifecycleRoutines) > 0 {
		routineWg.Add(len(ss.hostedLifecycleRoutines))
		for _, routine := range ss.hostedLifecycleRoutines {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						if ss.logger != nil {
							ss.logger.Errorf("panic in lifecycle routine %s: %v", phaseName, err)
						}
					}
					routineWg.Done()
				}()
				lifecycleAction(routine, ctx, routineWg)
			}()
		}
	}

	if len(ss.hostedRoutines) > 0 {
		routineWg.Add(len(ss.hostedRoutines))
		for _, routine := range ss.hostedRoutines {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						if ss.logger != nil {
							ss.logger.Errorf("panic in routine %s: %v", phaseName, err)
						}
					}
					routineWg.Done()
				}()
				routineAction(routine, ctx, routineWg)
			}()
		}
	}

	if !routineWg.WaitTimeout(timeout) {
		if ss.logger != nil {
			ss.logger.Warnf("'%s' wait timeout in hosted routines", phaseName)
		}
	}
}

func (ss *Host) Start(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	wg.Add(1)
	defer wg.Done()

	ss.initApp()

	defer func() {
		if ss.app != nil {
			select {
			case <-ctx.Done():
				ss.app.EmitRoutineStartedFailed()
				return
			default:
				ss.app.EmitRoutineStartedSuccess()
			}
		}
	}()

	ss.initRoutines()

	timeout := ss.startTimeout()
	ss.runLifecycleHook(ctx, ss.hostedLifecycleRoutines, "BeforeStart", timeout,
		func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) {
			r.BeforeStart(ctx, wg)
		},
	)

	select {
	case <-ctx.Done():
		return
	default:
	}

	if len(ss.hostedLifecycleRoutines) > 0 || len(ss.hostedRoutines) > 0 {
		ss.runPhase(ctx, timeout, "Start",
			func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) {
				r.Start(ctx, wg)
			},
			func(r host.IHostedRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) { r.Start(ctx, wg) },
		)
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	ss.runLifecycleHook(ctx, ss.hostedLifecycleRoutines, "AfterStart", timeout,
		func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) {
			r.AfterStart(ctx, wg)
		},
	)
}

func (ss *Host) Stop(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	wg.Add(1)
	defer wg.Done()

	timeout := ss.stopTimeout()

	ss.runLifecycleHook(ctx, ss.hostedLifecycleRoutines, "BeforeStop", timeout,
		func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) {
			r.BeforeStop(ctx, wg)
		},
	)

	if len(ss.hostedLifecycleRoutines) > 0 || len(ss.hostedRoutines) > 0 {
		ss.runPhase(ctx, timeout, "Stop",
			func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) { r.Stop(ctx, wg) },
			func(r host.IHostedRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) { r.Stop(ctx, wg) },
		)
	}

	ss.runLifecycleHook(ctx, ss.hostedLifecycleRoutines, "AfterStop", timeout,
		func(r host.IHostedLifecycleRoutine, ctx context.Context, wg *xsync.TimeoutWaitGroup) {
			r.AfterStop(ctx, wg)
		},
	)

	if ss.app != nil {
		ss.app.EmitRoutineStopped()
	}
}

func (ss *Host) initApp() {
	if ss.app != nil {
		return
	}
	appInstance := injection.GetRoutine[host.IHostApplication](ss.provider)
	if appInstance == nil {
		panic("IHostApplication not found in provider")
	}
	var ok bool
	ss.app, ok = appInstance.(*HostApplication)
	if !ok {
		panic("IHostApplication is not *HostApplication")
	}
}

func (ss *Host) initRoutines() {
	if ss.hostedRoutineContainer == nil {
		ss.hostedRoutineContainer = injection.GetRoutine[host.IHostedRoutineContainer](ss.provider)
		ss.hostedRoutineContainer.BuildHostedRoutines()
		ss.hostedRoutines = ss.hostedRoutineContainer.GetHostedRoutines()
	}
	if ss.hostedLifecycleRoutineContainer == nil {
		ss.hostedLifecycleRoutineContainer = injection.GetRoutine[host.IHostedLifecycleRoutineContainer](ss.provider)
		ss.hostedLifecycleRoutineContainer.BuildHostedLifecycleRoutines()
		ss.hostedLifecycleRoutines = ss.hostedLifecycleRoutineContainer.GetHostedLifecycleRoutines()
	}
}

func (ss *Host) GetRoutineProvider() injection.IRoutineProvider {
	return ss.provider
}
