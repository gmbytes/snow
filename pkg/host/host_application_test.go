package host_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/injection"
	"github.com/gmbytes/snow/pkg/xsync"
	"github.com/stretchr/testify/assert"
)

type mockHostApp struct {
	mu                sync.Mutex
	startedListeners  []func()
	stoppedListeners  []func()
	stoppingListeners []func()
}

func (m *mockHostApp) OnStarted(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedListeners = append(m.startedListeners, fn)
}

func (m *mockHostApp) OnStopped(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stoppedListeners = append(m.stoppedListeners, fn)
}

func (m *mockHostApp) OnStopping(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stoppingListeners = append(m.stoppingListeners, fn)
}

func (m *mockHostApp) StopApplication() {
	m.mu.Lock()
	listeners := make([]func(), len(m.stoppingListeners))
	copy(listeners, m.stoppingListeners)
	m.mu.Unlock()
	for _, fn := range listeners {
		fn()
	}
}

func (m *mockHostApp) emitStarted() {
	m.mu.Lock()
	listeners := make([]func(), len(m.startedListeners))
	copy(listeners, m.startedListeners)
	m.mu.Unlock()
	for _, fn := range listeners {
		fn()
	}
}

var hostAppType = reflect.TypeOf((*host.IHostApplication)(nil)).Elem()

type mockProvider struct {
	app *mockHostApp
}

func (p *mockProvider) GetRoutine(ty reflect.Type) any {
	if ty == hostAppType {
		return host.IHostApplication(p.app)
	}
	return nil
}

func (p *mockProvider) GetKeyedRoutine(_ any, _ reflect.Type) any { return nil }
func (p *mockProvider) CreateScope() injection.IRoutineScope      { return nil }
func (p *mockProvider) GetRootScope() injection.IRoutineScope     { return nil }

type mockHost struct {
	provider    *mockProvider
	startCalled bool
	stopCalled  bool
	startCtx    context.Context
	stopCtx     context.Context
	mu          sync.Mutex
}

func (h *mockHost) Start(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	wg.Add(1)
	defer wg.Done()

	h.mu.Lock()
	h.startCalled = true
	h.startCtx = ctx
	h.mu.Unlock()

	h.provider.app.emitStarted()
}

func (h *mockHost) Stop(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	wg.Add(1)
	defer wg.Done()

	h.mu.Lock()
	h.stopCalled = true
	h.stopCtx = ctx
	h.mu.Unlock()
}

func (h *mockHost) GetRoutineProvider() injection.IRoutineProvider {
	return h.provider
}

func newMockHost() (*mockHost, *mockHostApp) {
	app := &mockHostApp{}
	provider := &mockProvider{app: app}
	return &mockHost{provider: provider}, app
}

func TestRun_WhenStopApplicationCalled_ExpectStartAndStopInvoked(t *testing.T) {
	h, app := newMockHost()

	done := make(chan struct{})
	go func() {
		host.Run(h, context.Background())
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	app.StopApplication()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run 未在 StopApplication 后退出")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	assert.True(t, h.startCalled)
	assert.True(t, h.stopCalled)
}

func TestRun_WhenStartCtxCancelled_ExpectStopInvoked(t *testing.T) {
	h, _ := newMockHost()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		host.Run(h, ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run 未在 startCtx 取消后退出")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	assert.True(t, h.startCalled)
	assert.True(t, h.stopCalled)
}

func TestRunWithStopContext_NilContexts_ExpectNoFault(t *testing.T) {
	h, app := newMockHost()

	done := make(chan struct{})
	go func() {
		host.RunWithStopContext(h, nil, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	app.StopApplication()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWithStopContext(nil,nil) 未完成")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	assert.True(t, h.startCalled)
	assert.True(t, h.stopCalled)
}

func TestRunWithStopContext_UsesStopCtxForStop(t *testing.T) {
	h, app := newMockHost()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	done := make(chan struct{})
	go func() {
		host.RunWithStopContext(h, context.Background(), stopCtx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	app.StopApplication()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWithStopContext 未完成")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	assert.True(t, h.stopCalled)
	assert.Equal(t, stopCtx, h.stopCtx)
}
