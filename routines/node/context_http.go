package node

import (
	"context"
	"net/http"

	"github.com/gmbytes/snow/core/xjson"
)

var _ IRpcContext = (*httpRpcContext)(nil)

type httpRpcContext struct {
	ctx    context.Context
	cancel context.CancelFunc

	ch   chan *httpResponse
	errF func(error)
}

func newHttpRpcContext(parentCtx context.Context, ch chan *httpResponse) *httpRpcContext {
	ctx, cancel := context.WithCancel(parentCtx)
	return &httpRpcContext{
		ctx:    ctx,
		cancel: cancel,
		ch:     ch,
	}
}

func (ss *httpRpcContext) Context() context.Context {
	return ss.ctx
}

func (ss *httpRpcContext) GetRemoteNodeAddr() INodeAddr {
	return Addr(0)
}

func (ss *httpRpcContext) GetRemoteServiceAddr() int32 {
	return 0
}

func (ss *httpRpcContext) Catch(f func(error)) IRpcContext {
	ss.errF = f
	return ss
}

func (ss *httpRpcContext) Return(args ...any) {
	if ss.cancel != nil {
		ss.cancel()
	}

	if ss.ch == nil {
		return
	}

	if args == nil {
		args = make([]any, 0)
	}
	argsStr, err := xjson.Marshal(args)
	if err != nil {
		ss.ch <- &httpResponse{
			StatusCode: http.StatusInternalServerError,
			Result:     xjson.RawMessage(err.Error()),
		}
		return
	}

	ss.ch <- &httpResponse{
		StatusCode: http.StatusOK,
		Result:     argsStr,
	}
}

func (ss *httpRpcContext) Error(err error) {
	if ss.cancel != nil {
		ss.cancel()
	}

	if ss.ch == nil {
		return
	}

	ss.ch <- &httpResponse{
		StatusCode: http.StatusBadRequest,
		Result:     xjson.RawMessage(err.Error()),
	}
}

func (ss *httpRpcContext) onError(err error) {
	ss.errF(err)
}
