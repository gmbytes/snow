package transport

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

// Config defines gateway listener settings.
// TCP and WebSocket can be enabled at the same time.
type Config struct {
	TCPHost string
	TCPPort int
	WSHost  string
	WSPort  int
	WSPath  string
}

// NewListeners creates listeners according to transport configuration.
func NewListeners(cfg *Config) ([]net.Listener, error) {
	var listeners []net.Listener

	if cfg.TCPPort > 0 {
		l, err := net.Listen("tcp", net.JoinHostPort(cfg.TCPHost, strconv.Itoa(cfg.TCPPort)))
		if err != nil {
			return nil, fmt.Errorf("tcp listen at %v:%v failed: %w", cfg.TCPHost, cfg.TCPPort, err)
		}
		listeners = append(listeners, l)
	}

	if cfg.WSPort > 0 {
		l, err := newWSListener(cfg.WSHost, cfg.WSPort, cfg.WSPath)
		if err != nil {
			for _, ln := range listeners {
				_ = ln.Close()
			}
			return nil, fmt.Errorf("ws listen at %v:%v failed: %w", cfg.WSHost, cfg.WSPort, err)
		}
		listeners = append(listeners, l)
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("no listener configured: set TcpListenPort or WsListenPort")
	}

	return listeners, nil
}

// IsListenerClosedError reports whether an Accept error means listener shutdown.
func IsListenerClosedError(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, errWSListenerClosed)
}
