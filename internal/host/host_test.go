package internal

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/injection"
	"github.com/gmbytes/snow/pkg/xsync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 测试用 reflect.Type 辅助
// ---------------------------------------------------------------------------

func hostTyOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// ---------------------------------------------------------------------------
// 测试用 IHostedRoutine Mock
// ---------------------------------------------------------------------------

type mockHostedRoutine struct {
	startCalled int32
	stopCalled  int32
}

func (m *mockHostedRoutine) Start(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.startCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockHostedRoutine) Stop(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.stopCalled, 1)
	wg.Add(1)
	wg.Done()
}

// ---------------------------------------------------------------------------
// 辅助：构建最小 provider，注入 Host 运行所需的服务
// ---------------------------------------------------------------------------

func buildMinimalProvider(mock *mockHostedRoutine) injection.IRoutineProvider {
	col := NewRoutineCollection()

	// IHostApplication
	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostApplication](),
		Factory: func(_ injection.IRoutineScope) any {
			return NewHostApplication()
		},
	})

	// IHostedRoutineContainer（可选注入 mock）
	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostedRoutineContainer](),
		Factory: func(_ injection.IRoutineScope) any {
			c := &HostedRoutineContainer{}
			if mock != nil {
				captured := mock
				c.AddHostedRoutine(func() host.IHostedRoutine { return captured })
			}
			return c
		},
	})

	// IHostedLifecycleRoutineContainer（空）
	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostedLifecycleRoutineContainer](),
		Factory: func(_ injection.IRoutineScope) any {
			return &HostedLifecycleRoutineContainer{}
		},
	})

	return NewProvider(col, nil)
}

func TestHost_GetRoutineProvider_ReturnsProvider(t *testing.T) {
	provider := buildMinimalProvider(nil)
	h := NewHost(provider)

	got := h.GetRoutineProvider()
	require.NotNil(t, got)
	assert.Equal(t, provider, got)
}

func TestHost_Start_CallsHostedRoutines_ExpectStartInvoked(t *testing.T) {
	mock := &mockHostedRoutine{}
	provider := buildMinimalProvider(mock)
	h := NewHost(provider)

	ctx := context.Background()
	wg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, wg)
	wg.Wait()

	assert.EqualValues(t, 1, atomic.LoadInt32(&mock.startCalled))
}

func TestHost_Stop_CallsHostedRoutines_ExpectStopInvoked(t *testing.T) {
	mock := &mockHostedRoutine{}
	provider := buildMinimalProvider(mock)
	h := NewHost(provider)

	ctx := context.Background()

	startWg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, startWg)
	startWg.Wait()

	stopWg := xsync.NewTimeoutWaitGroup()
	h.Stop(ctx, stopWg)
	stopWg.Wait()

	assert.EqualValues(t, 1, atomic.LoadInt32(&mock.stopCalled))
}

func TestHost_Start_WhenContextAlreadyCancelled_SkipsRunPhase(t *testing.T) {
	mock := &mockHostedRoutine{}
	provider := buildMinimalProvider(mock)
	h := NewHost(provider)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, wg)
	wg.Wait()

	assert.EqualValues(t, 0, atomic.LoadInt32(&mock.stopCalled))
}

func TestHostedRoutineContainer_AddAndBuild_ExpectRoutinesPresent(t *testing.T) {
	c := &HostedRoutineContainer{}
	m1 := &mockHostedRoutine{}
	m2 := &mockHostedRoutine{}

	c.AddHostedRoutine(func() host.IHostedRoutine { return m1 })
	c.AddHostedRoutine(func() host.IHostedRoutine { return m2 })

	assert.Empty(t, c.GetHostedRoutines())

	c.BuildHostedRoutines()

	routines := c.GetHostedRoutines()
	assert.Len(t, routines, 2)
}

type mockLifecycleRoutine struct {
	beforeStartCalled int32
	startCalled       int32
	afterStartCalled  int32
	beforeStopCalled  int32
	stopCalled        int32
	afterStopCalled   int32
}

func (m *mockLifecycleRoutine) BeforeStart(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.beforeStartCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockLifecycleRoutine) Start(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.startCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockLifecycleRoutine) AfterStart(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.afterStartCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockLifecycleRoutine) BeforeStop(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.beforeStopCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockLifecycleRoutine) Stop(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.stopCalled, 1)
	wg.Add(1)
	wg.Done()
}

func (m *mockLifecycleRoutine) AfterStop(_ context.Context, wg *xsync.TimeoutWaitGroup) {
	atomic.AddInt32(&m.afterStopCalled, 1)
	wg.Add(1)
	wg.Done()
}

func TestHostedLifecycleRoutineContainer_AddAndBuild_ExpectRoutinesPresent(t *testing.T) {
	c := &HostedLifecycleRoutineContainer{}
	m := &mockLifecycleRoutine{}

	c.AddHostedLifecycleRoutine(func() host.IHostedLifecycleRoutine { return m })
	assert.Empty(t, c.GetHostedLifecycleRoutines())

	c.BuildHostedLifecycleRoutines()

	routines := c.GetHostedLifecycleRoutines()
	assert.Len(t, routines, 1)
}

func buildProviderWithLifecycle(lm *mockLifecycleRoutine) injection.IRoutineProvider {
	col := NewRoutineCollection()

	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostApplication](),
		Factory:  func(_ injection.IRoutineScope) any { return NewHostApplication() },
	})

	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostedRoutineContainer](),
		Factory:  func(_ injection.IRoutineScope) any { return &HostedRoutineContainer{} },
	})

	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    hostTyOf[host.IHostedLifecycleRoutineContainer](),
		Factory: func(_ injection.IRoutineScope) any {
			c := &HostedLifecycleRoutineContainer{}
			captured := lm
			c.AddHostedLifecycleRoutine(func() host.IHostedLifecycleRoutine { return captured })
			return c
		},
	})

	return NewProvider(col, nil)
}

func TestHost_WithLifecycleRoutine_Start_CallsAllHooks(t *testing.T) {
	lm := &mockLifecycleRoutine{}
	provider := buildProviderWithLifecycle(lm)
	h := NewHost(provider)

	ctx := context.Background()
	wg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, wg)
	wg.Wait()

	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.beforeStartCalled))
	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.startCalled))
	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.afterStartCalled))
}

func TestHost_WithLifecycleRoutine_Stop_CallsAllHooks(t *testing.T) {
	lm := &mockLifecycleRoutine{}
	provider := buildProviderWithLifecycle(lm)
	h := NewHost(provider)

	ctx := context.Background()

	startWg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, startWg)
	startWg.Wait()

	stopWg := xsync.NewTimeoutWaitGroup()
	h.Stop(ctx, stopWg)
	stopWg.Wait()

	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.beforeStopCalled))
	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.stopCalled))
	assert.EqualValues(t, 1, atomic.LoadInt32(&lm.afterStopCalled))
}
