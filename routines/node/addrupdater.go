package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mogud/snow/core/task"
)

const addrResolveTimeout = 5 * time.Second

type AddrResolver func(context.Context) (Addr, error)

type AddrUpdater struct {
	addr           atomic.Int64
	resolver       AddrResolver
	resolveTimeout time.Duration
	signal         chan struct{}
	done           chan struct{}

	started atomic.Bool
	stopped atomic.Bool

	stateMu  sync.Mutex
	cancel   context.CancelFunc
	doneOnce sync.Once
}

func NewNodeAddrUpdater(initial Addr, resolver AddrResolver) *AddrUpdater {
	u := &AddrUpdater{
		resolver:       resolver,
		resolveTimeout: addrResolveTimeout,
		signal:         make(chan struct{}, 1),
		done:           make(chan struct{}),
	}
	u.addr.Store(int64(initial))
	return u
}

func (ss *AddrUpdater) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !ss.started.CompareAndSwap(false, true) {
		return
	}
	if ss.stopped.Load() {
		ss.closeDone()
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	ss.stateMu.Lock()
	if ss.stopped.Load() {
		ss.stateMu.Unlock()
		cancel()
		ss.closeDone()
		return
	}
	ss.cancel = cancel
	ss.stateMu.Unlock()

	task.Execute(func() {
		defer ss.closeDone()
		defer ss.stopped.Store(true)
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ss.signal:
				ss.resolve(runCtx)
			}
		}
	})
}

func (ss *AddrUpdater) GetNodeAddr() Addr {
	return Addr(ss.addr.Load())
}

func (ss *AddrUpdater) Stop() {
	if ss.stopped.CompareAndSwap(false, true) {
		ss.stateMu.Lock()
		cancel := ss.cancel
		ss.stateMu.Unlock()
		if cancel != nil {
			cancel()
		}
		if !ss.started.Load() {
			ss.closeDone()
		}
	}
	<-ss.done
}

func (ss *AddrUpdater) signalRefresh() {
	if ss.stopped.Load() {
		return
	}
	select {
	case ss.signal <- struct{}{}:
	default:
	}
}

func (ss *AddrUpdater) resolve(ctx context.Context) {
	if ss.resolver == nil {
		if ctx.Err() == nil && !ss.stopped.Load() {
			ss.addr.Store(int64(AddrInvalid))
		}
		return
	}

	resolveCtx, cancel := context.WithTimeout(ctx, ss.resolveTimeout)
	addr, err := ss.resolver(resolveCtx)
	resolveErr := resolveCtx.Err()
	cancel()

	if ctx.Err() != nil || ss.stopped.Load() {
		return
	}
	if err != nil || resolveErr != nil {
		addr = AddrInvalid
	}
	ss.addr.Store(int64(addr))
}

func (ss *AddrUpdater) closeDone() {
	ss.doneOnce.Do(func() {
		close(ss.done)
	})
}
