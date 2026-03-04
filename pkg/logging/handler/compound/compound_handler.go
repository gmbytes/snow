package compound

import (
	"github.com/gmbytes/snow/pkg/logging"
	"github.com/gmbytes/snow/pkg/option"
)

var _ logging.ILogHandler = (*Handler)(nil)

type Option struct {
	NodeId   int    `snow:"NodeId"`   // 日志节点 Id
	NodeName string `snow:"NodeName"` // 日志节点名
}

type Handler struct {
	proxy   []logging.ILogHandler
	filters []logging.ILogFilter
	opt     *Option
}

func NewHandler() *Handler {
	return &Handler{}
}

func (ss *Handler) Construct(opt *option.Option[*Option]) {
	ss.opt = opt.Get()
}

func (ss *Handler) Log(data *logging.LogData) {
	for _, f := range ss.filters {
		if f != nil && !f.ShouldLog(data.Level, data.Name, data.Path) {
			return
		}
	}

	if ss.opt != nil {
		if ss.opt.NodeId > 0 {
			data.NodeID = ss.opt.NodeId
		}
		if ss.opt.NodeName != "" {
			data.NodeName = ss.opt.NodeName
		}
	}

	for _, handler := range ss.proxy {
		if handler != nil {
			handler.Log(data)
		}
	}
}

func (ss *Handler) AddHandler(logger logging.ILogHandler) {
	ss.proxy = append(ss.proxy, logger)
}

func (ss *Handler) AddFilter(filter logging.ILogFilter) {
	ss.filters = append(ss.filters, filter)
}
