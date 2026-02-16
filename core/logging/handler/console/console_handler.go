package console

import (
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/gmbytes/snow/core/logging"
	"github.com/gmbytes/snow/core/option"
)

var _ logging.ILogHandler = (*Handler)(nil)

type Option struct {
	Formatter     string                   `snow:"Formatter"`
	FileLineLevel int                      `snow:"FileLineLevel"`
	FileLineSkip  int                      `snow:"FileLineSkip"`
	ErrorLevel    logging.Level            `snow:"ErrorLevel"`
	Filter        map[string]logging.Level `snow:"Filter"`
	DefaultLevel  logging.Level            `snow:"DefaultLevel"`
}

type Handler struct {
	lock             sync.Mutex
	option           *Option
	sortedFilterKeys []string
	formatter        func(logData *logging.LogData) string
}

func NewHandler() *Handler {
	handler := &Handler{
		option: &Option{
			Formatter:     "Color",
			FileLineLevel: 4,
			FileLineSkip:  6,
			ErrorLevel:    logging.ERROR,
			Filter:        make(map[string]logging.Level),
		},
		formatter: logging.ColorLogFormatter,
	}

	handler.sortedFilterKeys = slices.Sorted(maps.Keys(handler.option.Filter))
	return handler
}

func (ss *Handler) Construct(opt *option.Option[*Option], repo *logging.LogFormatterContainer) {
	ss.option = opt.Get()
	formatterName := ss.option.Formatter
	ss.formatter = repo.GetFormatter(formatterName)
	if ss.formatter == nil {
		ss.formatter = logging.ColorLogFormatter
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
	ss.lock.Unlock()

	if curOption == nil {
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

	if len(logData.File) == 0 && int(logData.Level) >= curOption.FileLineLevel {
		// 安全调用 runtime.Caller，防止 skip 值过大导致 panic
		skip := curOption.FileLineSkip
		if skip < 0 {
			skip =20
		}
		if skip > 20 { // 限制最大 skip 值，防止超出调用栈
			skip = 20
		}
		pc, fn, ln, ok := runtime.Caller(skip)
		if !ok {
			// 如果 Caller 失败，使用原始 logData
			fn = ""
			ln = 0
		}
		_ = pc // 避免未使用变量警告
		d := logData
		logData = &logging.LogData{
			Time:     d.Time,
			NodeID:   d.NodeID,
			NodeName: d.NodeName,
			Path:     d.Path,
			Name:     d.Name,
			ID:       d.ID,
			File:     fn,
			Line:     ln,
			Level:    d.Level,
			ErrorCode: d.ErrorCode,
			Custom:   d.Custom,
			Message:  d.Message,
		}
	}

	if formatter == nil {
		formatter = logging.ColorLogFormatter
	}
	message := formatter(logData)

	if logData.Level < curOption.ErrorLevel {
		_, _ = fmt.Fprintln(os.Stdout, message)
	} else {
		_, _ = fmt.Fprintln(os.Stderr, message)
	}
}
