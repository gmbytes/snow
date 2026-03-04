package compound

import (
	"sync"
	"testing"
	"time"

	"github.com/gmbytes/snow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogHandler 测试用 handler，线程安全收集 LogData。
type mockLogHandler struct {
	mu   sync.Mutex
	logs []*logging.LogData
}

func (m *mockLogHandler) Log(data *logging.LogData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, data)
}

func (m *mockLogHandler) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.logs)
}

func (m *mockLogHandler) last() *logging.LogData {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logs) == 0 {
		return nil
	}
	return m.logs[len(m.logs)-1]
}

func makeData(level logging.Level, msg string) *logging.LogData {
	return &logging.LogData{
		Time:    time.Now(),
		Level:   level,
		Message: func() string { return msg },
	}
}

// TestNewHandler_WhenCreated_ExpectNonNil
func TestNewHandler_WhenCreated_ExpectNonNil(t *testing.T) {
	h := NewHandler()
	assert.NotNil(t, h)
}

// TestHandler_WhenAddHandler_ExpectDispatchToIt
func TestHandler_WhenAddHandler_ExpectDispatchToIt(t *testing.T) {
	h := NewHandler()
	m := &mockLogHandler{}
	h.AddHandler(m)

	data := makeData(logging.INFO, "hello")
	h.Log(data)
	assert.Equal(t, 1, m.count(), "handler 应收到 1 条日志")
}

// TestHandler_WhenMultipleHandlers_ExpectAllDispatched
func TestHandler_WhenMultipleHandlers_ExpectAllDispatched(t *testing.T) {
	h := NewHandler()
	m1 := &mockLogHandler{}
	m2 := &mockLogHandler{}
	m3 := &mockLogHandler{}
	h.AddHandler(m1)
	h.AddHandler(m2)
	h.AddHandler(m3)

	data := makeData(logging.WARN, "broadcast")
	h.Log(data)

	assert.Equal(t, 1, m1.count(), "handler1 应收到日志")
	assert.Equal(t, 1, m2.count(), "handler2 应收到日志")
	assert.Equal(t, 1, m3.count(), "handler3 应收到日志")
}

// TestHandler_WhenNoSubHandlers_ExpectNoPanic
func TestHandler_WhenNoSubHandlers_ExpectNoPanic(t *testing.T) {
	h := NewHandler()
	data := makeData(logging.INFO, "no subs")
	assert.NotPanics(t, func() {
		h.Log(data)
	})
}

// TestHandler_WhenNilOpt_ExpectDispatchWithoutNodeChange
func TestHandler_WhenNilOpt_ExpectDispatchWithoutNodeChange(t *testing.T) {
	h := NewHandler() // opt 默认为 nil
	m := &mockLogHandler{}
	h.AddHandler(m)

	data := &logging.LogData{
		Time:     time.Now(),
		Level:    logging.INFO,
		NodeID:   99,
		NodeName: "original",
		Message:  func() string { return "msg" },
	}
	h.Log(data)
	require.NotNil(t, m.last())
	// 无 opt 时不修改 NodeID/NodeName
	assert.Equal(t, 99, m.last().NodeID)
	assert.Equal(t, "original", m.last().NodeName)
}

// TestHandler_WhenOptSet_ExpectNodeIDAndNameOverridden
func TestHandler_WhenOptSet_ExpectNodeIDAndNameOverridden(t *testing.T) {
	h := NewHandler()
	h.opt = &Option{
		NodeId:   42,
		NodeName: "test-node",
	}
	m := &mockLogHandler{}
	h.AddHandler(m)

	data := &logging.LogData{
		Time:    time.Now(),
		Level:   logging.INFO,
		Message: func() string { return "msg" },
	}
	h.Log(data)
	require.NotNil(t, m.last())
	assert.Equal(t, 42, m.last().NodeID)
	assert.Equal(t, "test-node", m.last().NodeName)
}

// TestHandler_WhenOptSetWithZeroNodeID_ExpectNodeIDNotOverridden
func TestHandler_WhenOptSetWithZeroNodeID_ExpectNodeIDNotOverridden(t *testing.T) {
	h := NewHandler()
	h.opt = &Option{
		NodeId:   0, // 零值不覆盖
		NodeName: "override-name",
	}
	m := &mockLogHandler{}
	h.AddHandler(m)

	data := &logging.LogData{
		Time:    time.Now(),
		Level:   logging.INFO,
		NodeID:  55,
		Message: func() string { return "msg" },
	}
	h.Log(data)
	require.NotNil(t, m.last())
	assert.Equal(t, 55, m.last().NodeID, "NodeID 为 0 时不应覆盖")
	assert.Equal(t, "override-name", m.last().NodeName)
}

// TestHandler_WhenMultipleLogs_ExpectAllReceived
func TestHandler_WhenMultipleLogs_ExpectAllReceived(t *testing.T) {
	h := NewHandler()
	m := &mockLogHandler{}
	h.AddHandler(m)

	for i := 0; i < 5; i++ {
		h.Log(makeData(logging.DEBUG, "msg"))
	}
	assert.Equal(t, 5, m.count())
}
