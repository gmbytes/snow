package ignore_input

import (
	"bufio"
	"context"
	"github.com/gmbytes/snow/core/host"
	"github.com/gmbytes/snow/core/xsync"
	"os"
	"sync/atomic"
	"time"
)

var _ host.IHostedRoutine = (*IgnoreInput)(nil)

type IgnoreInput struct {
	closed atomic.Bool
}

func (ss *IgnoreInput) Start(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for !ss.closed.Load() {
			if !scanner.Scan() {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (ss *IgnoreInput) Stop(ctx context.Context, wg *xsync.TimeoutWaitGroup) {
	ss.closed.Store(true)
}
