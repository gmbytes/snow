package node

import "context"

var _ = (IRpcContext)((*rpcContext)(nil))

type rpcContext struct {
	ctx    context.Context
	cancel context.CancelFunc

	reqSess     int32
	reqSrc      int32
	reqCb       func(m *message)
	reqNodeAddr Addr

	mRsp    *message
	srv     *Service
	flushed bool
	flushCb func()
}

func newRpcContext(parentCtx context.Context, srv *Service, mRsp *message, reqSess, reqSrc int32, reqNodeAddr Addr, reqCb func(m *message), flushCb func()) *rpcContext {
	ctx, cancel := context.WithCancel(parentCtx)
	return &rpcContext{
		ctx:    ctx,
		cancel: cancel,

		reqSess:     reqSess,
		reqSrc:      reqSrc,
		reqNodeAddr: reqNodeAddr,
		reqCb:       reqCb,

		mRsp:    mRsp,
		srv:     srv,
		flushCb: flushCb,
	}
}

func (ss *rpcContext) Context() context.Context {
	return ss.ctx
}

func (ss *rpcContext) GetRemoteNodeAddr() INodeAddr {
	return ss.reqNodeAddr
}

func (ss *rpcContext) GetRemoteServiceAddr() int32 {
	return ss.reqSrc
}

func (ss *rpcContext) Catch(f func(error)) IRpcContext {
	ss.mRsp.cb = func(m *message) {
		f(m.err)
	}
	return ss
}

func (ss *rpcContext) Return(args ...any) {
	ss.mRsp.writeResponse(args...)
	ss.flush()
}

func (ss *rpcContext) Error(err error) {
	ss.mRsp.err = err
	ss.mRsp.src = 0

	// 统计 RPC 错误次数，便于按服务维度聚合错误率
	if ss.srv != nil && ss.srv.node != nil && ss.srv.node.regOpt.MetricCollector != nil {
		ss.srv.node.regOpt.MetricCollector.Counter("[ServiceError] "+ss.srv.name, 1)
	}

	ss.flush()
}

func (ss *rpcContext) flush() {
	if ss.flushed {
		return
	}
	ss.flushed = true

	// 响应已发送，取消本次 RPC 关联的 Context，释放派生资源
	if ss.cancel != nil {
		ss.cancel()
	}

	if ss.flushCb != nil {
		ss.flushCb()
	}

	reqSess := ss.reqSess
	reqSrc := ss.reqSrc
	reqNodeAddr := ss.reqNodeAddr
	reqCb := ss.reqCb
	mRsp := ss.mRsp
	if reqSess > 0 {
		if reqCb != nil { // local service message
			reqCb(mRsp)
		} else if reqNodeAddr != 0 { // must be remote message
			sender := nodeGetMessageSender(reqNodeAddr, reqSrc, false, nil)
			if sender != nil {
				sender.send(mRsp)
			} else {
				ss.srv.Errorf("service at nAddr(%v) sAddr(%#8x) not found when rpc return", reqNodeAddr, reqSrc)
				mRsp.clear()
			}
		} // else is a local post
	}
}
