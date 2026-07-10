package node

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAddrUpdaterStopPreventsFurtherResolve(t *testing.T) {
	var calls atomic.Int32
	u := NewNodeAddrUpdater(Addr(1), func(context.Context) (Addr, error) {
		calls.Add(1)
		return Addr(2), nil
	})

	u.Start(context.Background())
	u.signalRefresh()
	require.Eventually(t, func() bool {
		return u.GetNodeAddr() == Addr(2)
	}, time.Second, time.Millisecond)

	u.Stop()
	u.signalRefresh()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(1), calls.Load())
}

func TestAddrUpdaterResolverFailureInvalidatesAddress(t *testing.T) {
	u := NewNodeAddrUpdater(Addr(1), func(context.Context) (Addr, error) {
		return AddrInvalid, errors.New("registry unavailable")
	})

	u.Start(context.Background())
	u.signalRefresh()
	require.Eventually(t, func() bool {
		return u.GetNodeAddr() == AddrInvalid
	}, time.Second, time.Millisecond)
	u.Stop()
}

func TestAddrUpdaterCoalescesConcurrentRefreshSignals(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	u := NewNodeAddrUpdater(Addr(1), func(ctx context.Context) (Addr, error) {
		calls.Add(1)
		enteredOnce.Do(func() { close(entered) })
		select {
		case <-release:
			return Addr(2), nil
		case <-ctx.Done():
			return AddrInvalid, ctx.Err()
		}
	})

	u.Start(context.Background())
	u.signalRefresh()
	<-entered
	for range 128 {
		u.signalRefresh()
	}
	close(release)

	require.Eventually(t, func() bool {
		return u.GetNodeAddr() == Addr(2) && calls.Load() == 2
	}, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(2), calls.Load())
	u.Stop()
}

func TestAddrUpdaterStartAndStopAreIdempotent(t *testing.T) {
	var calls atomic.Int32
	u := NewNodeAddrUpdater(Addr(1), func(context.Context) (Addr, error) {
		calls.Add(1)
		return Addr(2), nil
	})

	u.Start(context.Background())
	u.Start(context.Background())
	u.signalRefresh()
	require.Eventually(t, func() bool {
		return calls.Load() == 1
	}, time.Second, time.Millisecond)
	u.Stop()
	u.Stop()
}

func TestAddrUpdaterStopCancelsInFlightResolver(t *testing.T) {
	entered := make(chan struct{})
	u := NewNodeAddrUpdater(Addr(1), func(ctx context.Context) (Addr, error) {
		close(entered)
		<-ctx.Done()
		return AddrInvalid, ctx.Err()
	})

	u.Start(context.Background())
	u.signalRefresh()
	<-entered
	require.NotPanics(t, u.Stop)
	require.Equal(t, Addr(1), u.GetNodeAddr())
}

func TestAddrUpdaterResolverDeadlineInvalidatesAddress(t *testing.T) {
	u := NewNodeAddrUpdater(Addr(1), func(ctx context.Context) (Addr, error) {
		<-ctx.Done()
		return AddrInvalid, ctx.Err()
	})
	u.resolveTimeout = 10 * time.Millisecond

	u.Start(context.Background())
	u.signalRefresh()
	require.Eventually(t, func() bool {
		return u.GetNodeAddr() == AddrInvalid
	}, time.Second, time.Millisecond)
	u.Stop()
}
