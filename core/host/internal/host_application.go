package internal

import (
	"sync/atomic"

	"github.com/gmbytes/snow/core/host"
)

var _ host.IHostApplication = (*HostApplication)(nil)

type HostApplication struct {
	secondPass        atomic.Int32
	startedListeners  []func()
	stoppedListeners  []func()
	stoppingListeners []func()
}

func (ss *HostApplication) OnStarted(listener func()) {
	ss.startedListeners = append(ss.startedListeners, listener)
}

func (ss *HostApplication) OnStopped(listener func()) {
	ss.stoppedListeners = append(ss.stoppedListeners, listener)
}

func (ss *HostApplication) OnStopping(listener func()) {
	ss.stoppingListeners = append(ss.stoppingListeners, listener)
}

func NewHostApplication() *HostApplication {
	app := &HostApplication{}
	return app
}

func (ss *HostApplication) EmitRoutineStartedSuccess() {
	for _, listener := range ss.startedListeners {
		listener()
	}
	// 注意：启动成功不应该调用 StopApplication()，停止应该由外部信号或其他机制触发
}

func (ss *HostApplication) EmitRoutineStartedFailed() {
	ss.StopApplication()
}

func (ss *HostApplication) EmitRoutineStopped() {
	for _, listener := range ss.stoppedListeners {
		listener()
	}
}

func (ss *HostApplication) StopApplication() {
	// 第一次调用：0 -> 1，执行 stopping listeners
	if ss.secondPass.CompareAndSwap(0, 1) {
		for _, listener := range ss.stoppingListeners {
			listener()
		}
		return
	}

	// 第二次调用：1 -> 2，不再执行（避免重复调用）
	_ = ss.secondPass.CompareAndSwap(1, 2)
}
