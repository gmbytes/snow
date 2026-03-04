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
	rc := rpcContextPool.Get().(*rpcContext)
	rc.ctx = ctx
	rc.cancel = cancel
	rc.reqSess = reqSess
	rc.reqSrc = reqSrc
	rc.reqNodeAddr = reqNodeAddr
	rc.reqCb = reqCb
	rc.mRsp = mRsp
	rc.srv = srv
	rc.flushed = false
	rc.flushCb = flushCb
	return rc
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
			mRsp = nil
		} else if reqNodeAddr != 0 { // must be remote message
			sender := nodeGetMessageSender(reqNodeAddr, reqSrc, false, nil)
			if sender != nil {
				sender.send(mRsp)
				// mRsp 交给 sender 管理，由 remoteHandle.onTick 在序列化后调用 m.clear() 归还
				mRsp = nil
			} else {
				ss.srv.Errorf("service at nAddr(%v) sAddr(%#8x) not found when rpc return", reqNodeAddr, reqSrc)
				releaseMessage(mRsp)
				mRsp = nil
			}
		} // else is a local post
	}

	if mRsp != nil {
		releaseMessage(mRsp)
	}

	ss.releaseToPool()
}

func (ss *rpcContext) releaseToPool() {
	ss.ctx = nil
	ss.cancel = nil
	ss.reqCb = nil
	ss.mRsp = nil
	ss.srv = nil
	ss.flushCb = nil
	rpcContextPool.Put(ss)
}
