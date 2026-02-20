package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

const defaultAcceptedConnectionsBuffer = 1024

var _ net.Listener = (*wsListener)(nil)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

var errWSListenerClosed = errors.New("websocket listener closed")

// wsListener implements net.Listener by running an HTTP server that upgrades
// incoming connections to WebSocket.
type wsListener struct {
	addr    net.Addr
	server  *http.Server
	connCh  chan net.Conn
	closeCh chan struct{}
	once    sync.Once
}

func newWSListener(host string, port int, path string) (*wsListener, error) {
	if path == "" {
		path = "/ws"
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	l := &wsListener{
		connCh:  make(chan net.Conn, defaultAcceptedConnectionsBuffer),
		closeCh: make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, l.handleUpgrade)

	l.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l.addr = ln.Addr()

	go func() {
		if err := l.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = l.Close()
		}
	}()

	return l, nil
}

func (l *wsListener) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	ws, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := newWSConn(ws)
	select {
	case l.connCh <- conn:
	case <-l.closeCh:
		_ = ws.Close()
	}
}

func (l *wsListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.closeCh:
		return nil, errWSListenerClosed
	}
}

func (l *wsListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.closeCh)
		err = l.server.Shutdown(context.Background())
	})
	return err
}

func (l *wsListener) Addr() net.Addr {
	return l.addr
}
