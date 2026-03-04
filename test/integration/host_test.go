package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gmbytes/snow/pkg/configuration"
	"github.com/gmbytes/snow/pkg/configuration/sources"
	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/host/builder"
	"github.com/gmbytes/snow/pkg/injection"
	"github.com/gmbytes/snow/pkg/logging"
	"github.com/gmbytes/snow/pkg/logging/handler/compound"
	"github.com/gmbytes/snow/pkg/xsync"
)

type mockRoutine struct {
	startCalled atomic.Int32
	stopCalled  atomic.Int32
}

func (ss *mockRoutine) Start(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.startCalled.Add(1)
}

func (ss *mockRoutine) Stop(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.stopCalled.Add(1)
}

type mockLifecycleRoutine struct {
	mu    sync.Mutex
	calls []string
}

func (ss *mockLifecycleRoutine) record(name string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.calls = append(ss.calls, name)
}

func (ss *mockLifecycleRoutine) BeforeStart(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("BeforeStart")
}

func (ss *mockLifecycleRoutine) Start(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("Start")
}

func (ss *mockLifecycleRoutine) AfterStart(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("AfterStart")
}

func (ss *mockLifecycleRoutine) BeforeStop(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("BeforeStop")
}

func (ss *mockLifecycleRoutine) Stop(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("Stop")
}

func (ss *mockLifecycleRoutine) AfterStop(_ context.Context, _ *xsync.TimeoutWaitGroup) {
	ss.record("AfterStop")
}

func (ss *mockLifecycleRoutine) getCalls() []string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	result := make([]string, len(ss.calls))
	copy(result, ss.calls)
	return result
}

type testSingleton struct {
	Value string
}

func runHostAsync(h host.IHost, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		host.Run(h, ctx)
	}()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for: %s", msg)
	}
}

func TestHost_BuildAndStart_ExpectNoFault(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()
	if h == nil {
		t.Fatal("期望获得非 nil IHost")
	}

	ctx := context.Background()
	wg := xsync.NewTimeoutWaitGroup()
	h.Start(ctx, wg)
	wg.Wait()

	stopWg := xsync.NewTimeoutWaitGroup()
	h.Stop(ctx, stopWg)
	stopWg.Wait()
}

func TestHost_RunWithStopContext_ExpectGracefulShutdown(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	app := host.GetRoutine[host.IHostApplication](h.GetRoutineProvider())
	if app == nil {
		t.Fatal("期望从 provider 获得 IHostApplication")
	}

	startedCh := make(chan struct{})
	app.OnStarted(func() {
		close(startedCh)
	})

	startCtx := context.Background()
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		host.RunWithStopContext(h, startCtx, stopCtx)
	}()

	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Host 未在超时内完成启动")
	}

	app.StopApplication()

	waitDone(t, done, 10*time.Second, "RunWithStopContext 优雅关闭")
}

func TestHost_HostedRoutine_ExpectStartStopCalled(t *testing.T) {
	b := builder.NewDefaultBuilder()
	host.AddHostedRoutine[*mockRoutine](b)

	h := b.Build()
	routine := host.GetRoutine[*mockRoutine](h.GetRoutineProvider())
	if routine == nil {
		t.Fatal("期望 *mockRoutine 已注册并可解析")
	}

	app := host.GetRoutine[host.IHostApplication](h.GetRoutineProvider())

	startedCh := make(chan struct{}, 1)
	app.OnStarted(func() {
		select {
		case startedCh <- struct{}{}:
		default:
		}
	})

	done := runHostAsync(h, context.Background())

	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Host 未在超时内启动")
	}

	if routine.startCalled.Load() == 0 {
		t.Error("期望 Start 被调用")
	}

	app.StopApplication()
	waitDone(t, done, 10*time.Second, "Host 停止")

	if routine.stopCalled.Load() == 0 {
		t.Error("期望 Stop 被调用")
	}
}

func TestHost_HostedLifecycleRoutine_ExpectAllHooksCalled(t *testing.T) {
	b := builder.NewDefaultBuilder()

	lifecycle := &mockLifecycleRoutine{}

	container := host.GetRoutine[host.IHostedLifecycleRoutineContainer](b.GetRoutineProvider())
	container.AddHostedLifecycleRoutine(func() host.IHostedLifecycleRoutine {
		return lifecycle
	})

	h := b.Build()
	app := host.GetRoutine[host.IHostApplication](h.GetRoutineProvider())

	startedCh := make(chan struct{}, 1)
	app.OnStarted(func() {
		select {
		case startedCh <- struct{}{}:
		default:
		}
	})

	done := runHostAsync(h, context.Background())

	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Host 未在超时内启动")
	}

	startCalls := lifecycle.getCalls()
	expectStart := []string{"BeforeStart", "Start", "AfterStart"}
	for i, expected := range expectStart {
		if i >= len(startCalls) {
			t.Errorf("启动钩子缺失 [%d]: 期望 %q", i, expected)
			continue
		}
		if startCalls[i] != expected {
			t.Errorf("启动钩子 [%d]: 期望 %q，得到 %q", i, expected, startCalls[i])
		}
	}

	app.StopApplication()
	waitDone(t, done, 10*time.Second, "Host 停止")

	allCalls := lifecycle.getCalls()
	expectAll := []string{"BeforeStart", "Start", "AfterStart", "BeforeStop", "Stop", "AfterStop"}
	if len(allCalls) != len(expectAll) {
		t.Fatalf("期望 %d 个钩子调用，得到 %d: %v", len(expectAll), len(allCalls), allCalls)
	}
	for i, expected := range expectAll {
		if allCalls[i] != expected {
			t.Errorf("钩子 [%d]: 期望 %q，得到 %q", i, expected, allCalls[i])
		}
	}
}

func TestHost_DIRegistration_ExpectSingletonResolved(t *testing.T) {
	b := builder.NewDefaultBuilder()

	host.AddSingletonFactory[*testSingleton](b, func(_ injection.IRoutineScope) *testSingleton {
		return &testSingleton{Value: "hello-snow"}
	})

	h := b.Build()
	provider := h.GetRoutineProvider()

	singleton := host.GetRoutine[*testSingleton](provider)
	if singleton == nil {
		t.Fatal("期望解析到 *testSingleton，得到 nil")
	}
	if singleton.Value != "hello-snow" {
		t.Errorf("期望 Value=%q，得到 %q", "hello-snow", singleton.Value)
	}

	singleton2 := host.GetRoutine[*testSingleton](provider)
	if singleton != singleton2 {
		t.Error("单例应返回同一实例")
	}
}

func TestHost_Configuration_MemorySource_ExpectValueBound(t *testing.T) {
	b := builder.NewDefaultBuilder()

	b.GetConfigurationManager().AddSource(&sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"App:Name":    "SnowTest",
			"App:Version": "1.0.0",
		},
	})

	h := b.Build()

	config := host.GetRoutine[configuration.IConfiguration](h.GetRoutineProvider())
	if config == nil {
		t.Fatal("期望从 provider 获得 IConfiguration")
	}

	name := config.Get("App:Name")
	if name != "SnowTest" {
		t.Errorf("期望 App:Name=%q，得到 %q", "SnowTest", name)
	}

	version := config.Get("App:Version")
	if version != "1.0.0" {
		t.Errorf("期望 App:Version=%q，得到 %q", "1.0.0", version)
	}
}

func TestHost_LogFilter_ExpectFilterApplied(t *testing.T) {
	b := builder.NewDefaultBuilder()

	host.AddLogFilter[*logging.LevelFilter](b, func() *logging.LevelFilter {
		return &logging.LevelFilter{Min: logging.WARN}
	})

	_ = b.Build()

	compoundHandler := host.GetRoutine[*compound.Handler](b.GetRoutineProvider())
	if compoundHandler == nil {
		t.Fatal("期望获得 *compound.Handler")
	}
}

func TestHost_RunWithContext_WhenContextCancelled_ExpectStop(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	ctx, cancel := context.WithCancel(context.Background())

	done := runHostAsync(h, ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()

	waitDone(t, done, 10*time.Second, "context 取消后 Host 停止")
}

func TestHost_BuildTwice_ExpectIndependentHosts(t *testing.T) {
	b1 := builder.NewDefaultBuilder()
	b2 := builder.NewDefaultBuilder()

	h1 := b1.Build()
	h2 := b2.Build()

	if h1 == nil || h2 == nil {
		t.Fatal("期望两个非 nil Host")
	}
	if h1 == h2 {
		t.Error("期望两个独立的 Host 实例")
	}
	if h1.GetRoutineProvider() == h2.GetRoutineProvider() {
		t.Error("期望两个独立的 RoutineProvider")
	}
}

func TestHost_OnStarted_ExpectCallbackInvoked(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	app := host.GetRoutine[host.IHostApplication](h.GetRoutineProvider())
	var called atomic.Bool
	app.OnStarted(func() {
		called.Store(true)
	})

	startedCh := make(chan struct{})
	app.OnStarted(func() {
		select {
		case <-startedCh:
		default:
			close(startedCh)
		}
	})

	done := runHostAsync(h, context.Background())

	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("OnStarted 回调未在超时内触发")
	}

	if !called.Load() {
		t.Error("期望 OnStarted 回调被调用")
	}

	app.StopApplication()
	waitDone(t, done, 10*time.Second, "Host 停止")
}

func TestHost_OnStopped_ExpectCallbackInvoked(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	app := host.GetRoutine[host.IHostApplication](h.GetRoutineProvider())

	startedCh := make(chan struct{}, 1)
	app.OnStarted(func() {
		select {
		case startedCh <- struct{}{}:
		default:
		}
	})

	stoppedCh := make(chan struct{})
	app.OnStopped(func() {
		select {
		case <-stoppedCh:
		default:
			close(stoppedCh)
		}
	})

	done := runHostAsync(h, context.Background())

	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Host 未在超时内启动")
	}

	app.StopApplication()
	waitDone(t, done, 10*time.Second, "Host 停止")

	select {
	case <-stoppedCh:
	case <-time.After(2 * time.Second):
		t.Error("期望 OnStopped 回调在 Stop 后被触发")
	}
}

func TestHost_GetRoutineProvider_ExpectNonNil(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	provider := h.GetRoutineProvider()
	if provider == nil {
		t.Fatal("期望 GetRoutineProvider 返回非 nil")
	}
}

func TestHost_ConfigurationSource_ExpectGetReturnsEmpty_WhenKeyNotExist(t *testing.T) {
	b := builder.NewDefaultBuilder()
	h := b.Build()

	config := host.GetRoutine[configuration.IConfiguration](h.GetRoutineProvider())
	val := config.Get("NonExistent:Key:That:Does:Not:Exist")
	if val != "" {
		t.Errorf("期望不存在的键返回空字符串，得到 %q", val)
	}
}
