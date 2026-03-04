package file

import (
	"fmt"
	"maps"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gmbytes/snow/pkg/logging"
	"github.com/gmbytes/snow/pkg/option"
)

var _ logging.ILogHandler = (*Handler)(nil)

// BackpressureMode 日志 channel 满时的背压策略。
type BackpressureMode int

const (
	// BackpressureDrop channel 满时丢弃当前日志（默认行为，与历史兼容）。
	BackpressureDrop BackpressureMode = iota
	// BackpressureBlock channel 满时阻塞调用方，直到有空间写入。
	BackpressureBlock
	// BackpressureDropLow channel 满时仅保留 >= DropMinLevel 的日志，低级别日志丢弃。
	BackpressureDropLow
)

const (
	dropReporterIntervalSeconds   = 30
	minFileRollingIntervalSeconds = 60
)

type Option struct {
	LogPath                    string                   `snow:"LogPath"`
	MaxLogChanLength           int                      `snow:"MaxLogChanLength"`
	Formatter                  string                   `snow:"Formatter"`
	FileLineLevel              int                      `snow:"FileLineLevel"`
	FileLineSkip               int                      `snow:"FileLineSkip"`
	Filter                     map[string]logging.Level `snow:"Filter"`
	DefaultLevel               logging.Level            `snow:"DefaultLevel"`
	FileNameFormat             string                   `snow:"FileNameFormat"`
	FileRollingMegabytes       int                      `snow:"FileRollingMegabytes"`
	FileRollingIntervalSeconds int                      `snow:"FileRollingIntervalSeconds"`
	Compress                   bool                     `snow:"Compress"`

	// BackpressureMode channel 满时的背压策略，默认 BackpressureDrop。
	BackpressureMode BackpressureMode `snow:"BackpressureMode"`
	// DropMinLevel BackpressureDropLow 模式下保留的最低级别（默认 WARN）。
	DropMinLevel logging.Level `snow:"DropMinLevel"`
}

// dropStats 记录各级别丢弃计数，用于可观测性。
type dropStats struct {
	counts [6]atomic.Int64 // 索引对应 logging.Level (NONE=0..FATAL=5)
	total  atomic.Int64
}

func (d *dropStats) inc(level logging.Level) {
	if int(level) >= 0 && int(level) < len(d.counts) {
		d.counts[level].Add(1)
	}
	d.total.Add(1)
}

func (d *dropStats) swapTotal() int64 {
	return d.total.Swap(0)
}

func (d *dropStats) snapshot() [6]int64 {
	var s [6]int64
	for i := range d.counts {
		s[i] = d.counts[i].Swap(0)
	}
	return s
}

type Handler struct {
	lock sync.Mutex

	lastFileLogTime         time.Time
	lastFileNameRefreshTime time.Time
	fileName                string
	fileNameTemplate        string
	index                   int32
	logChan                 chan *writerElement
	fileWriter              *writer

	option            *Option
	sortedFilterKeys  []string
	cacheFilterKeyMap map[string]struct{}
	formatter         func(logData *logging.LogData) string

	dropped dropStats
}

func NewHandler() *Handler {
	handler := &Handler{
		option: &Option{
			LogPath:          "logs",
			MaxLogChanLength: 102400,
			Formatter:        "Default",
			FileLineLevel:    4,
			FileLineSkip:     6,
			Filter:           make(map[string]logging.Level),
		},
		cacheFilterKeyMap: make(map[string]struct{}),
		formatter:         logging.ColorLogFormatter,
		logChan:           make(chan *writerElement),
	}

	for sPath := range handler.option.Filter {
		handler.sortedFilterKeys = append(handler.sortedFilterKeys, sPath)
		handler.cacheFilterKeyMap[sPath] = struct{}{}
	}

	slices.Sort(handler.sortedFilterKeys)
	handler.startDropReporter()
	return handler
}

// DroppedTotal 返回自上次调用以来被丢弃的日志总数（原子交换，调用即清零）。
// 可用于外部 MetricCollector 上报。
func (ss *Handler) DroppedTotal() int64 {
	return ss.dropped.swapTotal()
}

// DroppedSnapshot 返回自上次调用以来各级别的丢弃计数（原子交换，调用即清零）。
func (ss *Handler) DroppedSnapshot() [6]int64 {
	return ss.dropped.snapshot()
}

// startDropReporter 每 30 秒检查丢弃计数，若有丢弃则输出到 stderr。
func (ss *Handler) startDropReporter() {
	go func() {
		ticker := time.NewTicker(dropReporterIntervalSeconds * time.Second)
		defer ticker.Stop()
		levelNames := [6]string{"NONE", "TRACE", "DEBUG", "INFO", "WARN", "ERROR"}
		for range ticker.C {
			snap := ss.dropped.snapshot()
			var total int64
			for _, v := range snap {
				total += v
			}
			if total == 0 {
				continue
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("file log dropped %d entries in last %ds:", total, dropReporterIntervalSeconds))
			for i, v := range snap {
				if v > 0 {
					sb.WriteString(fmt.Sprintf(" %s=%d", levelNames[i], v))
				}
			}
			_, _ = fmt.Fprintln(os.Stderr, sb.String())
		}
	}()
}

func (ss *Handler) Construct(opt *option.Option[*Option], repo *logging.LogFormatterContainer) {
	ss.option = opt.Get()

	formatterName := ss.option.Formatter
	ss.formatter = repo.GetFormatter(formatterName)
	if ss.formatter == nil {
		ss.formatter = logging.DefaultLogFormatter
	}

	ss.CheckOption()

	opt.OnChanged(func() {
		newOption := opt.Get()

		ss.lock.Lock()
		defer ss.lock.Unlock()

		ss.option = newOption
		ss.CheckOption()
	})
}

func (ss *Handler) CheckOption() {
	ss.sortedFilterKeys = slices.Sorted(maps.Keys(ss.option.Filter))

	if ss.option.MaxLogChanLength <= 0 {
		ss.option.MaxLogChanLength = 102400
	}

	if ss.option.FileRollingMegabytes <= 0 {
		ss.option.FileRollingMegabytes = 100
	}

	if ss.option.FileRollingIntervalSeconds <= 0 {
		ss.option.FileRollingIntervalSeconds = 3600
	} else if ss.option.FileRollingIntervalSeconds < minFileRollingIntervalSeconds {
		ss.option.FileRollingIntervalSeconds = minFileRollingIntervalSeconds
	}

	if ss.option.FileNameFormat == "" {
		ss.option.FileNameFormat = "%Y_%02M_%02D_%02h_%02m_%04i.log"
	}

	if ss.option.MaxLogChanLength != cap(ss.logChan) || ss.fileWriter == nil || ss.option.Compress != ss.fileWriter.compress {
		close(ss.logChan)
		ss.logChan = make(chan *writerElement, ss.option.MaxLogChanLength)
		ss.fileWriter = newWriter(ss.logChan, ss.option.Compress)
	}

	if ss.option.LogPath == "" {
		ss.option.LogPath = "logs"
	}

	if ss.option.DefaultLevel == logging.NONE {
		ss.option.DefaultLevel = logging.INFO
	}
}

func (ss *Handler) Log(logData *logging.LogData) {
	if logData.Level == logging.NONE {
		return
	}

	ss.lock.Lock()
	curOption := ss.option
	filterKeys := ss.sortedFilterKeys
	formatter := ss.formatter
	logCh := ss.logChan
	ss.lock.Unlock()

	if curOption == nil || logCh == nil {
		return
	}

	filterLevel := curOption.DefaultLevel
	for _, key := range filterKeys {
		if strings.HasPrefix(logData.Path, key) {
			filterLevel = curOption.Filter[key]
			break
		}
	}

	if logData.Level < filterLevel {
		return
	}

	if logData.File == "" && int(logData.Level) >= curOption.FileLineLevel {
		_, fn, ln, _ := runtime.Caller(curOption.FileLineSkip)
		d := logData
		logData = &logging.LogData{
			Time:      d.Time,
			NodeID:    d.NodeID,
			NodeName:  d.NodeName,
			Path:      d.Path,
			Name:      d.Name,
			ID:        d.ID,
			File:      fn,
			Line:      ln,
			Level:     d.Level,
			ErrorCode: d.ErrorCode,
			Custom:    d.Custom,
			Message:   d.Message,
		}
	}

	if formatter == nil {
		formatter = logging.DefaultLogFormatter
	}
	message := formatter(logData)

	fileName := ss.refreshFileName(logData.Time)
	if fileName == "" {
		return
	}

	unit := &writerElement{
		File:    fileName,
		Message: message,
	}

	ss.lock.Lock()
	mode := curOption.BackpressureMode
	dropMin := curOption.DropMinLevel
	ss.lock.Unlock()

	if dropMin == logging.NONE {
		dropMin = logging.WARN
	}

	switch mode {
	case BackpressureBlock:
		logCh <- unit
	case BackpressureDropLow:
		if logData.Level >= dropMin {
			// 高优先级日志：阻塞写入，保证不丢
			logCh <- unit
		} else {
			select {
			case logCh <- unit:
			default:
				ss.dropped.inc(logData.Level)
			}
		}
	default: // BackpressureDrop
		select {
		case logCh <- unit:
		default:
			ss.dropped.inc(logData.Level)
		}
	}
}

func (ss *Handler) refreshFileName(now time.Time) string {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if ss.option == nil {
		return ""
	}

	if ss.fileName == "" || now.Sub(ss.lastFileNameRefreshTime) > 10*time.Second {
		ss.updateFileName(now)
	}
	return ss.fileName
}

func (ss *Handler) updateFileName(now time.Time) {
	ss.lastFileNameRefreshTime = now
	rollingFile := false
	if now.Sub(ss.lastFileLogTime) > time.Duration(ss.option.FileRollingIntervalSeconds)*time.Second {
		ss.lastFileLogTime = now.Truncate(time.Duration(ss.option.FileRollingIntervalSeconds) * time.Second)
		rollingFile = true
	}
	year, month, day := ss.lastFileLogTime.Date()

	newTemplate, indexFmt := ss.buildFileNameTemplate(year, int(month), day)
	if ss.fileNameTemplate != newTemplate {
		ss.applyNewTemplate(newTemplate, indexFmt)
	} else {
		ss.maybeRollFile(rollingFile, indexFmt)
	}
}

func (ss *Handler) buildFileNameTemplate(year, month, day int) (template, indexFmt string) {
	escape := false
	fileNameTemplateBuilder := &strings.Builder{}
	itemFormatBuilder := &strings.Builder{}

	for _, c := range ss.option.FileNameFormat {
		if escape {
			var done bool
			indexFmt, done = ss.handleEscapeChar(c, fileNameTemplateBuilder, itemFormatBuilder, year, month, day, indexFmt)
			if done {
				itemFormatBuilder.Reset()
				escape = false
			}
			continue
		}
		if c == '%' {
			escape = true
			itemFormatBuilder.WriteByte('%')
		} else {
			fileNameTemplateBuilder.WriteRune(c)
		}
	}
	return fileNameTemplateBuilder.String(), indexFmt
}

func (ss *Handler) handleEscapeChar(
	c rune,
	tmpl, item *strings.Builder,
	year, month, day int,
	indexFmt string,
) (newIndexFmt string, done bool) {
	switch {
	case c == '%':
		tmpl.WriteByte('%')
	case c >= '0' && c <= '9':
		item.WriteRune(c)
		return indexFmt, false
	case c == 'Y':
		item.WriteByte('d')
		fmt.Fprintf(tmpl, item.String(), year)
	case c == 'M':
		item.WriteByte('d')
		fmt.Fprintf(tmpl, item.String(), month)
	case c == 'D':
		item.WriteByte('d')
		fmt.Fprintf(tmpl, item.String(), day)
	case c == 'h':
		item.WriteByte('d')
		fmt.Fprintf(tmpl, item.String(), ss.lastFileLogTime.Hour())
	case c == 'm':
		item.WriteByte('d')
		fmt.Fprintf(tmpl, item.String(), ss.lastFileLogTime.Minute())
	case c == 'i':
		item.WriteByte('d')
		indexFmt = item.String()
		tmpl.WriteString("///__INDEX__///")
	default:
		tmpl.WriteRune(c)
	}
	return indexFmt, true
}

func (ss *Handler) applyNewTemplate(newTemplate, indexFmt string) {
	ss.fileNameTemplate = newTemplate
	ss.index = 0

	baseName := strings.ReplaceAll(ss.fileNameTemplate, "///__INDEX__///", fmt.Sprintf(indexFmt, ss.index))
	fullName := path.Join(ss.option.LogPath, baseName)
	if ss.fileName == "" && indexFmt != "" {
		for {
			if _, err := os.Stat(fullName); err != nil {
				if _, err = os.Stat(fullName + ".zst"); err != nil {
					break
				}
			}
			ss.index++
			baseName = strings.ReplaceAll(ss.fileNameTemplate, "///__INDEX__///", fmt.Sprintf(indexFmt, ss.index))
			fullName = path.Join(ss.option.LogPath, baseName)
		}
	}
	ss.fileName = fullName
}

func (ss *Handler) maybeRollFile(rollingFile bool, indexFmt string) {
	if !rollingFile && indexFmt != "" {
		if stat, err := os.Stat(ss.fileName); err == nil && stat.Size() > int64(ss.option.FileRollingMegabytes)*1024*1024 {
			rollingFile = true
		}
	}

	if rollingFile {
		ss.index++
		baseName := strings.ReplaceAll(ss.fileNameTemplate, "///__INDEX__///", fmt.Sprintf(indexFmt, ss.index))
		ss.fileName = path.Join(ss.option.LogPath, baseName)
	}
}
