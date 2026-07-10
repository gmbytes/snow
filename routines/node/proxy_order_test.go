package node

import (
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProxyPreservesCallOrderForSameSourceAndTarget(t *testing.T) {
	want := []string{"request-1", "request-2", "fence"}

	t.Run("local sender", func(t *testing.T) {
		target := &Service{}
		proxy := newOrderTestProxy(target)
		callNames(proxy, want)

		target.msgBufferLock.Lock()
		messages := append([]*message(nil), target.msgBuffer...)
		target.msgBufferLock.Unlock()
		require.Equal(t, want, requestNames(t, messages))
	})

	t.Run("loopback remote sender", func(t *testing.T) {
		client, server := loopbackTCPPair(t)
		handle := newRemoteHandle(&Node{}, Addr(101), client)
		handle.wg.Add(1)
		go handle.doSend()
		t.Cleanup(func() {
			handle.cancel()
			_ = client.Close()
			_ = server.Close()
			handle.wg.Wait()
		})

		proxy := newOrderTestProxy(handle)
		callNames(proxy, want)
		handle.onTick()

		require.NoError(t, server.SetReadDeadline(time.Now().Add(time.Second)))
		require.Equal(t, want, readRemoteRequestNames(t, server, len(want)))
	})
}

func TestProxyDoesNotReplayTimedOutPromiseAfterSenderReconnect(t *testing.T) {
	previousNode := gNode
	t.Cleanup(func() { gNode = previousNode })

	addr := Addr(101)
	testNode := &Node{
		services: make(map[int32]*Service),
		handle:   make(map[Addr]*remoteHandle),
	}
	oldHandle := newRemoteHandle(testNode, addr, nil)
	replacement := newRemoteHandle(testNode, addr, nil)
	testNode.handle[addr] = oldHandle
	gNode = testNode

	proxy := newOrderTestProxy(oldHandle)
	proxy.nAddr = addr
	proxy.Call("timed-out").Then(func() {}).Timeout(time.Millisecond).Done()
	require.Len(t, oldHandle.wBuffer, 1)

	request := oldHandle.wBuffer[0]
	callback, ok := oldHandle.sessCb.LoadAndDelete(request.sess)
	require.True(t, ok)
	callback.(*session).cb(&message{trace: request.trace, err: ErrRequestTimeoutLocal})

	atomic.StoreInt32(&oldHandle.status, 1)
	testNode.handle[addr] = replacement
	proxy.Call("next-request").Done()

	require.Equal(t, []string{"timed-out"}, requestNames(t, oldHandle.wBuffer))
	require.Equal(t, []string{"next-request"}, requestNames(t, replacement.wBuffer))
}

func newOrderTestProxy(sender iMessageSender) *serviceProxy {
	return &serviceProxy{
		srv:    &Service{sAddr: 1},
		nAddr:  Addr(101),
		sAddr:  2,
		sender: sender,
	}
}

func callNames(proxy IProxy, names []string) {
	for _, name := range names {
		proxy.Call(name).Done()
	}
}

func requestNames(t *testing.T, messages []*message) []string {
	t.Helper()
	names := make([]string, 0, len(messages))
	for _, msg := range messages {
		name, err := msg.getRequestFunc()
		require.NoError(t, err)
		names = append(names, name)
	}
	return names
}

func loopbackTCPPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		accepted <- acceptResult{conn: conn, err: acceptErr}
	}()

	client, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
	require.NoError(t, err)
	result := <-accepted
	require.NoError(t, result.err)
	return client, result.conn
}

func readRemoteRequestNames(t *testing.T, conn net.Conn, count int) []string {
	t.Helper()
	names := make([]string, 0, count)
	for range count {
		header := make([]byte, 4)
		_, err := io.ReadFull(conn, header)
		require.NoError(t, err)
		length := int(binary.LittleEndian.Uint32(header))
		require.GreaterOrEqual(t, length, messageHeaderLen)

		data := make([]byte, length)
		copy(data, header)
		_, err = io.ReadFull(conn, data[4:])
		require.NoError(t, err)
		msg := &message{}
		require.NoError(t, msg.unmarshal(data))
		name, err := msg.getRequestFunc()
		require.NoError(t, err)
		names = append(names, name)
	}
	return names
}
