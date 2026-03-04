package slog

import (
	"sync"
	"testing"

	"github.com/gmbytes/snow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger 用于测试的 ILogger 实现，收集所有调用。
type mockLogger struct {
	mu    sync.Mutex
	calls []logCall
}

type logCall struct {
	level  string
	format string
	args   []any
}

func (m *mockLogger) Tracef(format string, args ...any) {
	m.record("trace", format, args)
}
func (m *mockLogger) Debugf(format string, args ...any) {
	m.record("debug", format, args)
}
func (m *mockLogger) Infof(format string, args ...any) {
	m.record("info", format, args)
}
func (m *mockLogger) Warnf(format string, args ...any) {
	m.record("warn", format, args)
}
func (m *mockLogger) Errorf(format string, args ...any) {
	m.record("error", format, args)
}
func (m *mockLogger) Fatalf(format string, args ...any) {
	m.record("fatal", format, args)
}

func (m *mockLogger) record(level, format string, args []any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, logCall{level: level, format: format, args: args})
}

func (m *mockLogger) lastCall() *logCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	c := m.calls[len(m.calls)-1]
	return &c
}

func (m *mockLogger) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// mockHandler 测试用 handler。
type mockHandler struct {
	mu   sync.Mutex
	logs []*logging.LogData
}

func (m *mockHandler) Log(data *logging.LogData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, data)
}

func (m *mockHandler) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.logs)
}

// restoreGlobalLogger 在每个测试后恢复原始全局 logger。
func restoreGlobalLogger(t *testing.T) {
	t.Helper()
	// 记录当前的全局 logger，测试结束后还原
	current := globalLogger.Load()
	t.Cleanup(func() {
		if current != nil {
			globalLogger.Store(current)
		}
	})
}

// TestBindGlobalHandler_WhenCalled_ExpectGlobalLoggerChanged
func TestBindGlobalHandler_WhenCalled_ExpectGlobalLoggerChanged(t *testing.T) {
	restoreGlobalLogger(t)

	h := &mockHandler{}
	BindGlobalHandler(h)

	// 验证 global logger 改变
	logger := getLogger()
	require.NotNil(t, logger)

	// 向新 handler 发送日志
	logger.Infof("test message")
	assert.Equal(t, 1, h.count(), "BindGlobalHandler 后，日志应发送到新 handler")
}

// TestBindGlobalLogger_WhenValidLogger_ExpectGlobalChanged
func TestBindGlobalLogger_WhenValidLogger_ExpectGlobalChanged(t *testing.T) {
	restoreGlobalLogger(t)

	m := &mockLogger{}
	BindGlobalLogger(m)

	logger := getLogger()
	require.NotNil(t, logger)
	logger.Infof("hello")

	assert.Equal(t, 1, m.count())
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "info", m.lastCall().level)
}

// TestBindGlobalLogger_WhenNil_ExpectPanic
func TestBindGlobalLogger_WhenNil_ExpectPanic(t *testing.T) {
	assert.Panics(t, func() {
		BindGlobalLogger(nil)
	}, "传入 nil 应 panic")
}

// TestTracef_WhenCalled_ExpectDelegatedToGlobalLogger
func TestTracef_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Tracef("trace %s", "msg")
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "trace", m.lastCall().level)
}

// TestDebugf_WhenCalled_ExpectDelegatedToGlobalLogger
func TestDebugf_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Debugf("debug %d", 1)
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "debug", m.lastCall().level)
}

// TestInfof_WhenCalled_ExpectDelegatedToGlobalLogger
func TestInfof_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Infof("info %s", "test")
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "info", m.lastCall().level)
}

// TestWarnf_WhenCalled_ExpectDelegatedToGlobalLogger
func TestWarnf_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Warnf("warn")
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "warn", m.lastCall().level)
}

// TestErrorf_WhenCalled_ExpectDelegatedToGlobalLogger
func TestErrorf_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Errorf("error %v", "oops")
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "error", m.lastCall().level)
}

// TestFatalf_WhenCalled_ExpectDelegatedToGlobalLogger
func TestFatalf_WhenCalled_ExpectDelegatedToGlobalLogger(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	Fatalf("fatal %s", "crash")
	require.NotNil(t, m.lastCall())
	assert.Equal(t, "fatal", m.lastCall().level)
}

// TestGlobalLogger_WhenConcurrentAccess_ExpectNoDataRace
func TestGlobalLogger_WhenConcurrentAccess_ExpectNoDataRace(t *testing.T) {
	restoreGlobalLogger(t)

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				m := &mockLogger{}
				BindGlobalLogger(m)
			} else {
				Infof("concurrent %d", i)
			}
		}(i)
	}
	wg.Wait()
	// 只要没有 data race，测试就通过（-race 模式下会检测）
}

// TestGetLogger_WhenGlobalSet_ExpectReturnsIt
func TestGetLogger_WhenGlobalSet_ExpectReturnsIt(t *testing.T) {
	restoreGlobalLogger(t)
	m := &mockLogger{}
	BindGlobalLogger(m)

	got := getLogger()
	require.NotNil(t, got)
	got.Warnf("test")
	assert.Equal(t, 1, m.count())
}
