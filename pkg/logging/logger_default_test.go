package logging

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHandler 收集所有 Log 调用，线程安全。
type mockHandler struct {
	mu   sync.Mutex
	logs []*LogData
}

func (m *mockHandler) Log(data *LogData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, data)
}

func (m *mockHandler) last() *LogData {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logs) == 0 {
		return nil
	}
	return m.logs[len(m.logs)-1]
}

func (m *mockHandler) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.logs)
}

// TestNewDefaultLogger_WhenCreated_ExpectNonNil
func TestNewDefaultLogger_WhenCreated_ExpectNonNil(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("test/path", h, nil)
	assert.NotNil(t, logger)
}

// TestDefaultLogger_WhenTracef_ExpectLevelTrace
func TestDefaultLogger_WhenTracef_ExpectLevelTrace(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Tracef("trace %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, TRACE, h.last().Level)
}

// TestDefaultLogger_WhenDebugf_ExpectLevelDebug
func TestDefaultLogger_WhenDebugf_ExpectLevelDebug(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Debugf("debug %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, DEBUG, h.last().Level)
}

// TestDefaultLogger_WhenInfof_ExpectLevelInfo
func TestDefaultLogger_WhenInfof_ExpectLevelInfo(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Infof("info %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, INFO, h.last().Level)
}

// TestDefaultLogger_WhenWarnf_ExpectLevelWarn
func TestDefaultLogger_WhenWarnf_ExpectLevelWarn(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Warnf("warn %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, WARN, h.last().Level)
}

// TestDefaultLogger_WhenErrorf_ExpectLevelError
func TestDefaultLogger_WhenErrorf_ExpectLevelError(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Errorf("error %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, ERROR, h.last().Level)
}

// TestDefaultLogger_WhenFatalf_ExpectLevelFatal
func TestDefaultLogger_WhenFatalf_ExpectLevelFatal(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Fatalf("fatal %s", "msg")
	require.NotNil(t, h.last())
	assert.Equal(t, FATAL, h.last().Level)
}

// TestDefaultLogger_WhenPathSet_ExpectPathInLogData
func TestDefaultLogger_WhenPathSet_ExpectPathInLogData(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("myapp/service", h, nil)
	logger.Infof("hello")
	require.NotNil(t, h.last())
	assert.Equal(t, "myapp/service", h.last().Path)
}

// TestDefaultLogger_WhenMessageFormatted_ExpectCorrectMessage
func TestDefaultLogger_WhenMessageFormatted_ExpectCorrectMessage(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Infof("value=%d name=%s", 42, "foo")
	require.NotNil(t, h.last())
	assert.Equal(t, "value=42 name=foo", h.last().Message())
}

// TestDefaultLogger_WhenLogDataBuilderSet_ExpectBuilderCalled
func TestDefaultLogger_WhenLogDataBuilderSet_ExpectBuilderCalled(t *testing.T) {
	h := &mockHandler{}
	called := false
	builder := func(data *LogData) {
		called = true
		data.Name = "injected"
	}
	logger := NewDefaultLogger("p", h, builder)
	logger.Infof("msg")
	assert.True(t, called, "logDataBuilder 应被调用")
	require.NotNil(t, h.last())
	assert.Equal(t, "injected", h.last().Name)
}

// TestDefaultLogger_WhenGlobalLogDataBuilderSet_ExpectBuilderCalled
func TestDefaultLogger_WhenGlobalLogDataBuilderSet_ExpectBuilderCalled(t *testing.T) {
	// 保存并在测试后恢复全局 builder
	original := GlobalLogDataBuilder
	t.Cleanup(func() { GlobalLogDataBuilder = original })

	globalCalled := false
	GlobalLogDataBuilder = func(data *LogData) {
		globalCalled = true
		data.NodeName = "global-node"
	}

	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Infof("msg")
	assert.True(t, globalCalled, "GlobalLogDataBuilder 应被调用")
	require.NotNil(t, h.last())
	assert.Equal(t, "global-node", h.last().NodeName)
}

// TestDefaultLogger_WhenNilHandler_ExpectNoPanic
func TestDefaultLogger_WhenNilHandler_ExpectNoPanic(t *testing.T) {
	logger := NewDefaultLogger("p", nil, nil)
	assert.NotPanics(t, func() {
		logger.Infof("msg")
	})
}

// TestDefaultLogger_WhenMultipleLogs_ExpectAllReceived
func TestDefaultLogger_WhenMultipleLogs_ExpectAllReceived(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Tracef("t")
	logger.Debugf("d")
	logger.Infof("i")
	logger.Warnf("w")
	logger.Errorf("e")
	logger.Fatalf("f")
	assert.Equal(t, 6, h.count())
}

// TestDefaultLogger_WhenTimeSet_ExpectNonZeroTime
func TestDefaultLogger_WhenTimeSet_ExpectNonZeroTime(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.Infof("msg")
	require.NotNil(t, h.last())
	assert.False(t, h.last().Time.IsZero(), "LogData.Time 不应为零值")
}
