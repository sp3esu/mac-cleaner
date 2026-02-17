package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/engine"
)

// DefaultIdleTimeout is the maximum time a connection can be idle before
// being closed. Reset on each received message.
const DefaultIdleTimeout = 5 * time.Minute

// Server is a Unix domain socket IPC server for mac-cleaner.
type Server struct {
	socketPath string
	listener   net.Listener
	version    string

	// IdleTimeout is the maximum time a connection can be idle before
	// being closed. Defaults to DefaultIdleTimeout if zero.
	IdleTimeout time.Duration

	// engine is the scan/cleanup engine instance.
	engine *engine.Engine

	// handler is the method dispatch table.
	handler *Handler

	// busy tracks whether a scan or cleanup operation is in progress.
	busy atomic.Bool

	// mu guards active connection state.
	mu     sync.Mutex
	active net.Conn

	// connCancel cancels the current connection's context when the client
	// disconnects, allowing long-running handlers to abort cleanly.
	connCancel context.CancelFunc

	// done is closed when the server shuts down.
	done chan struct{}
}

// New creates a new server that will listen on the given socket path.
// The engine is used for all scan and cleanup operations.
func New(socketPath, version string, eng *engine.Engine) *Server {
	s := &Server{
		socketPath:  socketPath,
		version:     version,
		engine:      eng,
		IdleTimeout: DefaultIdleTimeout,
		done:        make(chan struct{}),
	}
	s.handler = NewHandler(s)
	return s
}

// Serve starts the server, listening for connections until the context is
// cancelled or Shutdown is called. It removes stale socket files on startup
// and cleans up the socket file on shutdown.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.cleanStaleSocket(); err != nil {
		return fmt.Errorf("stale socket: %w", err)
	}

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	// Ensure socket file is removed on shutdown.
	defer s.cleanup()

	// Cancel the listener when context is done.
	go func() {
		select {
		case <-ctx.Done():
			ln.Close() // #nosec G104 -- best-effort listener close during shutdown
		case <-s.done:
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}

		// Handle one connection at a time.
		s.handleConnection(ctx, conn)
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() {
	select {
	case <-s.done:
		return // already shut down
	default:
	}
	close(s.done)
	if s.listener != nil {
		s.listener.Close() // #nosec G104 -- best-effort listener close during shutdown
	}
	s.mu.Lock()
	if s.connCancel != nil {
		s.connCancel()
	}
	if s.active != nil {
		s.active.Close() // #nosec G104 -- best-effort connection close during shutdown
	}
	s.mu.Unlock()
}

// handleConnection processes a single client connection. It creates a
// per-connection context that is cancelled when the client disconnects,
// allowing long-running handlers (scan, cleanup) to abort cleanly.
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	s.active = conn
	s.connCancel = cancel
	s.mu.Unlock()

	defer func() {
		conn.Close() // #nosec G104 -- best-effort connection close on handler exit
		s.mu.Lock()
		s.active = nil
		s.connCancel = nil
		s.mu.Unlock()
	}()

	reader := NewNDJSONReader(conn)
	writer := NewNDJSONWriter(conn)

	for {
		select {
		case <-connCtx.Done():
			return
		case <-s.done:
			return
		default:
		}

		// Set idle timeout — if no message arrives within IdleTimeout,
		// the connection is closed.
		_ = conn.SetReadDeadline(time.Now().Add(s.IdleTimeout))

		req, err := reader.Read()
		if err != nil {
			return // connection closed, timeout, or read error
		}

		// Reset deadline for next read.
		_ = conn.SetReadDeadline(time.Time{})

		if req.Method == MethodShutdown {
			_ = writer.WriteResult(req.ID, map[string]string{"status": "shutting_down"})
			s.Shutdown()
			return
		}

		s.handler.Dispatch(connCtx, req, writer)
	}
}

// cleanStaleSocket removes a leftover socket file if no process is listening
// on it. This handles the case where a previous server crashed without cleanup.
func (s *Server) cleanStaleSocket() error {
	info, err := os.Lstat(s.socketPath)
	if os.IsNotExist(err) {
		return nil // no socket file, nothing to do
	}
	if err != nil {
		return fmt.Errorf("stat socket: %w", err)
	}

	// Only remove if it's a socket (not a regular file or symlink).
	if info.Mode().Type()&os.ModeSocket == 0 {
		return fmt.Errorf("path %s exists but is not a socket", s.socketPath)
	}

	// Try connecting to see if a server is already running.
	conn, err := net.Dial("unix", s.socketPath)
	if err == nil {
		conn.Close() // #nosec G104 -- best-effort close of probe connection
		return fmt.Errorf("another server is already listening on %s", s.socketPath)
	}

	// Stale socket — remove it.
	if err := os.Remove(s.socketPath); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

// cleanup removes the socket file.
func (s *Server) cleanup() {
	_ = os.Remove(s.socketPath)
}
