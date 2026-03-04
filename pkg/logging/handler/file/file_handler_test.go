package file

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gmbytes/snow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ────────────────────────────────────────────────────────────
// BackpressureMode 常量测试
// ────────────────────────────────────────────────────────────

func TestBackpressureMode_WhenConstants_ExpectCorrectValues(t *testing.T) {
	assert.Equal(t, BackpressureMode(0), BackpressureDrop, "Drop 应为 0")
	assert.Equal(t, BackpressureMode(1), BackpressureBlock, "Block 应为 1")
	assert.Equal(t, BackpressureMode(2), BackpressureDropLow, "DropLow 应为 2")
}

// ────────────────────────────────────────────────────────────
// dropStats 单元测试
// ────────────────────────────────────────────────────────────

func TestDropStats_WhenInc_ExpectCountsIncrease(t *testing.T) {
	var ds dropStats
	ds.inc(logging.INFO)
	ds.inc(logging.INFO)
	ds.inc(logging.WARN)

	assert.Equal(t, int64(3), ds.total.Load(), "total 应为 3")
	assert.Equal(t, int64(2), ds.counts[logging.INFO].Load(), "INFO 计数应为 2")
	assert.Equal(t, int64(1), ds.counts[logging.WARN].Load(), "WARN 计数应为 1")
}

func TestDropStats_WhenSwapTotal_ExpectReturnsAndClears(t *testing.T) {
	var ds dropStats
	ds.inc(logging.DEBUG)
	ds.inc(logging.ERROR)

	total := ds.swapTotal()
	assert.Equal(t, int64(2), total, "swapTotal 应返回 2")
	assert.Equal(t, int64(0), ds.total.Load(), "交换后 total 应归零")
}

func TestDropStats_WhenSnapshot_ExpectReturnsAndClearsPerLevel(t *testing.T) {
	var ds dropStats
	ds.inc(logging.TRACE)
	ds.inc(logging.TRACE)
	ds.inc(logging.ERROR)

	snap := ds.snapshot()
	assert.Equal(t, int64(2), snap[logging.TRACE], "TRACE 快照应为 2")
	assert.Equal(t, int64(1), snap[logging.ERROR], "ERROR 快照应为 1")

	// 再次快照应全部归零
	snap2 := ds.snapshot()
	for i, v := range snap2 {
		assert.Equal(t, int64(0), v, "第二次快照 index=%d 应为 0", i)
	}
}

func TestDropStats_WhenConcurrentInc_ExpectCorrectTotal(t *testing.T) {
	var ds dropStats
	const n = 1000
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ds.inc(logging.INFO)
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(n), ds.total.Load())
}

// ────────────────────────────────────────────────────────────
// NewHandler 测试
// ────────────────────────────────────────────────────────────

func TestNewHandler_WhenCreated_ExpectNonNilWithDefaults(t *testing.T) {
	h := NewHandler()
	require.NotNil(t, h)
	assert.NotNil(t, h.option)
	assert.Equal(t, "logs", h.option.LogPath)
	assert.Equal(t, 102400, h.option.MaxLogChanLength)
}

// ────────────────────────────────────────────────────────────
// DroppedTotal / DroppedSnapshot
// ────────────────────────────────────────────────────────────

func TestHandler_WhenDroppedTotal_ExpectAccumulatedCount(t *testing.T) {
	h := NewHandler()
	h.dropped.inc(logging.INFO)
	h.dropped.inc(logging.WARN)

	total := h.DroppedTotal()
	assert.Equal(t, int64(2), total)
	// 调用后归零
	assert.Equal(t, int64(0), h.DroppedTotal())
}

func TestHandler_WhenDroppedSnapshot_ExpectPerLevelCountsAndReset(t *testing.T) {
	h := NewHandler()
	h.dropped.inc(logging.DEBUG)
	h.dropped.inc(logging.ERROR)
	h.dropped.inc(logging.ERROR)

	snap := h.DroppedSnapshot()
	assert.Equal(t, int64(1), snap[logging.DEBUG])
	assert.Equal(t, int64(2), snap[logging.ERROR])

	// 调用后归零
	snap2 := h.DroppedSnapshot()
	for i, v := range snap2 {
		assert.Equal(t, int64(0), v, "index=%d 应归零", i)
	}
}

// ────────────────────────────────────────────────────────────
// Log 过滤逻辑测试（绕过 refreshFileName 文件 I/O）
// 通过检查 dropped 计数来验证过滤行为
// ────────────────────────────────────────────────────────────

// newHandlerWithTinyBuf 创建一个使用极小 channel buffer 的 handler，用于测试背压。
func newHandlerWithTinyBuf(t *testing.T, bufSize int, mode BackpressureMode) *Handler {
	t.Helper()
	tmpDir := t.TempDir()

	logChan := make(chan *writerElement, bufSize)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			MaxLogChanLength:           bufSize,
			DefaultLevel:               logging.TRACE, // 允许所有级别
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			BackpressureMode:           mode,
			DropMinLevel:               logging.WARN,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	return h
}

func makeLogData(level logging.Level) *logging.LogData {
	return &logging.LogData{
		Time:    time.Now(),
		Level:   level,
		Path:    "test/path",
		Message: func() string { return "test message" },
	}
}

// TestHandler_WhenLogNoneLevel_ExpectSkipped
func TestHandler_WhenLogNoneLevel_ExpectSkipped(t *testing.T) {
	h := newHandlerWithTinyBuf(t, 10, BackpressureDrop)
	data := makeLogData(logging.NONE)
	h.Log(data)
	// NONE 级别直接返回，不写入 channel，dropped 不增加
	assert.Equal(t, int64(0), h.dropped.total.Load())
}

// TestHandler_WhenDefaultLevelFilter_ExpectLowLevelDropped
func TestHandler_WhenDefaultLevelFilter_ExpectLowLevelDropped(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			DefaultLevel:               logging.WARN, // 只允许 WARN+
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}

	// INFO < WARN，应被过滤（不写 channel，不增加 dropped）
	h.Log(makeLogData(logging.INFO))
	h.Log(makeLogData(logging.DEBUG))

	// channel 应为空
	assert.Equal(t, 0, len(logChan), "低级别日志不应进入 channel")
}

// TestHandler_WhenFilterMap_ExpectPathBasedFiltering
func TestHandler_WhenFilterMap_ExpectPathBasedFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			DefaultLevel:               logging.TRACE,
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			Filter: map[string]logging.Level{
				"noisy/module": logging.ERROR, // noisy 模块只写 ERROR+
			},
		},
		sortedFilterKeys:  []string{"noisy/module"},
		cacheFilterKeyMap: map[string]struct{}{"noisy/module": {}},
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}

	// noisy 模块 WARN 级别 < ERROR，应被过滤
	noisyData := &logging.LogData{
		Time:    time.Now(),
		Level:   logging.WARN,
		Path:    "noisy/module/sub",
		Message: func() string { return "noisy warn" },
	}
	h.Log(noisyData)
	assert.Equal(t, 0, len(logChan), "noisy WARN 应被路径过滤器拦截")
}

func newHandlerWithFullChan(t *testing.T, mode BackpressureMode) *Handler {
	t.Helper()
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 1)
	logChan <- &writerElement{File: "dummy", Message: "pre-fill"}

	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			DefaultLevel:               logging.TRACE,
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			BackpressureMode:           mode,
			DropMinLevel:               logging.WARN,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
	}
	return h
}

func TestHandler_WhenDropMode_FullChannel_ExpectDropped(t *testing.T) {
	h := newHandlerWithFullChan(t, BackpressureDrop)
	h.Log(makeLogData(logging.INFO))
	assert.Equal(t, int64(1), h.dropped.total.Load(), "channel 满时 Drop 模式应记录 dropped")
}

func TestHandler_WhenDropLowMode_HighPriority_ExpectBlockNotDrop(t *testing.T) {
	logDir, err := os.MkdirTemp("", "snow-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(logDir) })

	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    logDir,
			DefaultLevel:               logging.TRACE,
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			BackpressureMode:           BackpressureDropLow,
			DropMinLevel:               logging.WARN,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}

	h.Log(makeLogData(logging.WARN))
	h.Log(makeLogData(logging.ERROR))
	assert.Equal(t, int64(0), h.dropped.total.Load(), "高优先级日志不应被 drop")
}

func TestHandler_WhenDropLowMode_LowPriorityFullChannel_ExpectDropped(t *testing.T) {
	h := newHandlerWithFullChan(t, BackpressureDropLow)
	h.Log(makeLogData(logging.DEBUG))
	assert.Equal(t, int64(1), h.dropped.total.Load(), "DropLow 模式下低优先级日志应被 drop")
}

// TestHandler_WhenBlockMode_SpaceAvailable_ExpectWritten
func TestHandler_WhenBlockMode_SpaceAvailable_ExpectWritten(t *testing.T) {
	logDir, err := os.MkdirTemp("", "snow-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(logDir) })

	logChan := make(chan *writerElement, 5)
	h := &Handler{
		option: &Option{
			LogPath:                    logDir,
			DefaultLevel:               logging.TRACE,
			FileNameFormat:             "test.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			BackpressureMode:           BackpressureBlock,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}

	var done atomic.Bool
	go func() {
		h.Log(makeLogData(logging.INFO))
		done.Store(true)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !done.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, done.Load(), "Block 模式在有空间时应不阻塞完成")
	assert.Equal(t, int64(0), h.dropped.total.Load())
}

// ────────────────────────────────────────────────────────────
// refreshFileName 测试
// ────────────────────────────────────────────────────────────

func TestRefreshFileName_WhenSimpleFormat_ExpectFileInLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			FileNameFormat:             "gs.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
	}

	now := time.Now()
	name := h.refreshFileName(now)
	assert.NotEmpty(t, name, "refreshFileName 应返回非空文件名")
	assert.Contains(t, name, tmpDir, "文件名应包含 log 目录路径")
}

func TestRefreshFileName_WhenDateFormat_ExpectDateInName(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			FileNameFormat:             "%Y_%02M_%02D.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
	}

	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	name := h.refreshFileName(now)
	assert.Contains(t, name, "2026", "文件名应包含年份")
	assert.Contains(t, name, "03", "文件名应包含月份")
	assert.Contains(t, name, "04", "文件名应包含日期")
}

func TestRefreshFileName_WhenIndexFormat_ExpectIndexInName(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			FileNameFormat:             "%Y_%04i.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
	}

	now := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	name := h.refreshFileName(now)
	assert.NotEmpty(t, name)
	// 应包含 index 格式化结果（从 0 开始）
	base := filepath.Base(name)
	assert.Contains(t, base, "0000", "初始 index 为 0，格式 %04i 应产生 0000，实际: %s", base)
}

func TestRefreshFileName_WhenCalledTwiceWithinWindow_ExpectSameName(t *testing.T) {
	tmpDir := t.TempDir()
	logChan := make(chan *writerElement, 10)
	h := &Handler{
		option: &Option{
			LogPath:                    tmpDir,
			FileNameFormat:             "fixed.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
	}

	now := time.Now()
	name1 := h.refreshFileName(now)
	// 5 秒内再次调用（< 10 秒刷新阈值）
	name2 := h.refreshFileName(now.Add(5 * time.Second))
	assert.Equal(t, name1, name2, "10 秒内应返回相同文件名")
}

func TestHandler_WhenLogWritesToFile_ExpectFileCreated(t *testing.T) {
	logDir, err := os.MkdirTemp("", "snow-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(logDir) })

	logChan := make(chan *writerElement, 100)
	h := &Handler{
		option: &Option{
			LogPath:                    logDir,
			DefaultLevel:               logging.TRACE,
			FileNameFormat:             "output.log",
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			BackpressureMode:           BackpressureDrop,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.DefaultLogFormatter,
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}

	h.Log(makeLogData(logging.INFO))

	expectedFile := filepath.Join(logDir, "output.log")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(expectedFile); statErr == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_, statErr := os.Stat(expectedFile)
	require.NoError(t, statErr, "日志文件应已创建，路径: %s", expectedFile)

	content, readErr := os.ReadFile(expectedFile)
	require.NoError(t, readErr)
	assert.NotEmpty(t, content, "日志文件应有内容")
}

func TestCheckOption_WhenZeroMaxChanLength_ExpectDefault(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           0,
			FileRollingMegabytes:       0,
			FileRollingIntervalSeconds: 0,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	assert.Equal(t, 102400, h.option.MaxLogChanLength)
	assert.Equal(t, 100, h.option.FileRollingMegabytes)
	assert.Equal(t, 3600, h.option.FileRollingIntervalSeconds)
}

func TestCheckOption_WhenShortIntervalSeconds_ExpectClampedTo60(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           102400,
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 30,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	assert.Equal(t, 60, h.option.FileRollingIntervalSeconds)
}

func TestCheckOption_WhenEmptyFileNameFormat_ExpectDefault(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           102400,
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			FileNameFormat:             "",
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	assert.Equal(t, "%Y_%02M_%02D_%02h_%02m_%04i.log", h.option.FileNameFormat)
}

func TestCheckOption_WhenEmptyLogPath_ExpectDefault(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           102400,
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			FileNameFormat:             "test.log",
			LogPath:                    "",
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	assert.Equal(t, "logs", h.option.LogPath)
}

func TestCheckOption_WhenDefaultLevelNone_ExpectInfo(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           102400,
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			FileNameFormat:             "test.log",
			LogPath:                    "logs",
			DefaultLevel:               logging.NONE,
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	assert.Equal(t, logging.INFO, h.option.DefaultLevel)
}

func TestCheckOption_WhenFilterMap_ExpectSortedKeys(t *testing.T) {
	logChan := make(chan *writerElement, 102400)
	h := &Handler{
		option: &Option{
			MaxLogChanLength:           102400,
			FileRollingMegabytes:       100,
			FileRollingIntervalSeconds: 3600,
			FileNameFormat:             "test.log",
			Filter: map[string]logging.Level{
				"z/module": logging.ERROR,
				"a/module": logging.WARN,
				"m/module": logging.INFO,
			},
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		logChan:           logChan,
		fileWriter:        newWriter(logChan, false),
	}
	h.CheckOption()
	require.Len(t, h.sortedFilterKeys, 3)
	assert.Equal(t, "a/module", h.sortedFilterKeys[0])
	assert.Equal(t, "m/module", h.sortedFilterKeys[1])
	assert.Equal(t, "z/module", h.sortedFilterKeys[2])
}
