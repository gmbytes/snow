package node

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"
)

const (
	defaultTCPTimeout = 30 * time.Second
)

var (
	ErrServiceNotExist      = fmt.Errorf("service not exist")
	ErrNodeMessageChanFull  = fmt.Errorf("note message chan full")
	ErrRequestTimeoutRemote = fmt.Errorf("session timeout from remote")
	ErrRequestTimeoutLocal  = fmt.Errorf("session timeout from local")
	ErrRequestCancelled     = fmt.Errorf("request cancelled by context")
)

type iProxy interface {
	IProxy

	doCall(*promise)
}

var _ = iProxy((*serviceProxy)(nil))

type serviceProxy struct {
	srv          *Service
	nAddr        Addr
	nAddrUpdater *AddrUpdater
	sAddr        int32
	sender       iMessageSender

	bufferFullCB func()
	buffer       []*promise
}

func (ss *serviceProxy) Call(fName string, args ...any) IPromise {
	if ss.sAddr == 0 {
		return (*dumbPromise)(nil)
	}

	return newPromise(ss, fName, args)
}

func (ss *serviceProxy) GetNodeAddr() INodeAddr {
	if ss.sAddr == 0 {
		return AddrInvalid
	}

	if ss.nAddrUpdater != nil {
		return ss.nAddrUpdater.GetNodeAddr()
	}

	return ss.nAddr
}

func (ss *serviceProxy) Avail() bool {
	return ss.sAddr != 0
}

func (ss *serviceProxy) doCall(p *promise) {
	if ss.sAddr == 0 {
		if ss.buffer != nil {
			if cap(ss.buffer) == len(ss.buffer) && ss.bufferFullCB != nil {
				ss.bufferFullCB()
				return
			}
			ss.buffer = append(ss.buffer, p)
		}
		return
	}

	srv := ss.srv

	// 确定父 Context：显式传入 > Service 生命周期 > Background
	parentCtx := p.ctx
	if parentCtx == nil {
		parentCtx = srv.ctx
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// 统一从 Context 派生超时：若调用方未设置 deadline，施加默认超时
	var callCtx context.Context
	var callCancel context.CancelFunc
	if _, ok := parentCtx.Deadline(); ok {
		callCtx, callCancel = context.WithCancel(parentCtx)
	} else {
		callCtx, callCancel = context.WithTimeout(parentCtx, defaultTCPTimeout)
	}

	m := &message{
		ctx:     callCtx,
		timeout: timeoutFromContext(callCtx),
		src:     srv.GetAddr(),
		dst:     ss.sAddr,
		// TODO trace id
	}
	m.writeRequest(p.fName, p.args)

	if ss.sender == nil || ss.sender.closed() {
		var ch chan<- bool
		if ss.nAddrUpdater != nil {
			ch = ss.nAddrUpdater.getSigChan()
		}
		ss.sender = nodeGetMessageSender(ss.GetNodeAddr().(Addr), ss.sAddr, true, ch)
	}
	if ss.sender == nil {
		callCancel()
		if p.errCb != nil {
			srv.Fork("proxy.err.cb", func() {
				p.errCb(ErrServiceNotExist)
			})
		} else {
			srv.Errorf("rpc(%s) uncatched error: %+v", p.fName, ErrServiceNotExist)
		}
		if p.finalCb != nil {
			srv.Fork("proxy.err.finalCb", func() {
				p.finalCb()
				p.clear()
			})
		} else {
			p.clear()
		}
		m.clear()
		return
	}

	if p.successCb == nil {
		callCancel()
		if p.finalCb != nil {
			srv.Fork("proxy.post.finalCb", func() {
				p.finalCb()
				p.clear()
			})
		} else {
			p.clear()
		}
	} else {
		sess := nodeGenSessionID()
		trace := m.trace

		// sync.Once 保证回调仅执行一次，消除正常响应与超时/取消竞态
		var cbOnce sync.Once
		cb := func(mm *message) {
			cbOnce.Do(func() {
				callCancel()
				ss.callThen(mm, srv, p, sess)
			})
		}
		m.cb = cb
		m.sess = sess

		// 唯一超时/取消监听：全部由 Context 驱动
		go func() {
			<-callCtx.Done()
			err := callCtx.Err()
			if err == context.DeadlineExceeded {
				err = ErrRequestTimeoutLocal
			} else {
				err = ErrRequestCancelled
			}
			om := &message{
				trace: trace,
				err:   err,
			}
			cb(om)
		}()
	}

	ss.sender.send(m)
}

func (ss *serviceProxy) callThen(mm *message, srv *Service, p *promise, sess int32) {
	srv.Fork("proxy.forkCb", func() {
		defer func() {
			if p.finalCb != nil {
				p.finalCb()
			}
			p.clear()
		}()

		if mm.src == 0 { // error occurs
			err := mm.getError()
			if p.errCb != nil {
				p.errCb(err)
			} else {
				srv.Errorf("rpc(%s:%v) uncatched error: %+v", p.fName, sess, err)
			}
			return
		}

		fv := reflect.ValueOf(p.successCb)
		if !fv.IsValid() {
			srv.Errorf("rpc(%s:%v) invalid success callback", p.fName, sess)
			return
		}

		ft := fv.Type()
		fArgs, err := mm.getResponse(ft)
		if err != nil {
			srv.Errorf("rpc(%s:%v) response error: %+v", p.fName, sess, err)
			return
		}

		panicked := true
		defer func() {
			if panicked {
				if e := recover(); e != nil {
					srv.Errorf("rpc(%s:%v) response got panic: %v => %v", p.fName, sess, e, string(debug.Stack()))
				} else {
					srv.Errorf("rpc(%s:%v) response got panic: %v", p.fName, sess, string(debug.Stack()))
				}
			}
		}()

		fRet := fv.Call(fArgs)

		for _, arg := range fArgs {
			if arg.CanAddr() {
				arg.SetZero()
			}
		}
		fArgs = nil
		for _, arg := range fRet {
			if arg.CanAddr() {
				arg.SetZero()
			}
		}
		fRet = nil

		panicked = false
	})
}

// timeoutFromContext 从 Context 的 deadline 计算剩余超时时长，无 deadline 返回 0。
func timeoutFromContext(ctx context.Context) time.Duration {
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 {
			return d
		}
	}
	return 0
}
