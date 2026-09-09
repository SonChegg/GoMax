// Package connection implements the connection lifecycle shared by the TCP
// and WebSocket clients: opening/closing the transport, a receive loop that
// decodes frames and either resolves a pending request or dispatches an
// event, and sequence-number bookkeeping for outbound requests. It is a
// port of pymax's connection.connection.ConnectionManager.
package connection

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/transport"
)

// ErrNotOpen is returned when Send/Request is called on a closed connection.
var ErrNotOpen = errors.New("connection: connection is not open")

// ErrConnectionLost is returned by WaitClosed and to pending requests when
// the receive loop observes a transport error.
var ErrConnectionLost = errors.New("connection: connection lost")

// EventHandler is invoked for every inbound frame that is not a
// response/error to a pending request (or in addition to resolving one,
// exactly like pymax dispatches to both the pending future and on_event).
type EventHandler func(ctx context.Context, frame protocol.InboundFrame)

// CloseHandler is invoked once when the connection transitions to closed,
// carrying the error that caused it (nil for a clean, requested close).
type CloseHandler func(err error)

// Manager owns a Transport + Reader + Codec triple and drives the receive
// loop, a port of pymax's connection.connection.ConnectionManager.
type Manager struct {
	Reader    Reader
	Transport transport.Transport
	Codec     protocol.Codec

	OnEvent EventHandler
	OnClose CloseHandler

	requests *PendingRequests

	mu             sync.Mutex
	isOpen         bool
	connectionLost bool
	closeReported  bool
	seq            int
	cancelRecv     context.CancelFunc
	recvDone       chan struct{}
	recvErr        error
	eventWG        sync.WaitGroup
}

// NewManager builds a connection manager around the given reader/transport/codec.
func NewManager(reader Reader, t transport.Transport, codec protocol.Codec) *Manager {
	return &Manager{
		Reader:    reader,
		Transport: t,
		Codec:     codec,
		requests:  NewPendingRequests(),
		seq:       -1,
	}
}

// Open connects the transport and starts the receive loop. Calling Open on
// an already-open connection is a no-op.
func (m *Manager) Open(ctx context.Context) error {
	m.mu.Lock()
	if m.isOpen {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if err := m.Transport.Connect(ctx); err != nil {
		return err
	}

	recvCtx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.isOpen = true
	m.connectionLost = false
	m.closeReported = false
	m.cancelRecv = cancel
	m.recvDone = make(chan struct{})
	m.mu.Unlock()

	go m.recvLoop(recvCtx)
	return nil
}

// Close stops the receive loop, cancels all pending requests, and closes
// the transport.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	if !m.isOpen && m.recvDone == nil {
		m.mu.Unlock()
		return nil
	}
	m.isOpen = false
	cancel := m.cancelRecv
	done := m.recvDone
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	err := m.Transport.Close(ctx)

	if done != nil {
		<-done
	}
	m.eventWG.Wait()

	m.requests.CancelAll(nil)
	return err
}

// Fail marks the connection as failed and tears it down, used when a
// background loop (e.g. the ping loop) detects the connection is unusable.
func (m *Manager) Fail(ctx context.Context, err error) {
	m.mu.Lock()
	m.connectionLost = true
	m.mu.Unlock()

	m.requests.CancelAll(err)
	_ = m.Transport.Close(ctx)
	m.markClosed(err)
}

// Send encodes and writes frame without waiting for a response.
func (m *Manager) Send(ctx context.Context, frame protocol.OutboundFrame) error {
	m.mu.Lock()
	open := m.isOpen
	m.mu.Unlock()
	if !open {
		return ErrNotOpen
	}

	data, err := m.Codec.Encode(frame)
	if err != nil {
		return err
	}
	return m.Transport.Send(ctx, data)
}

// Request sends frame and waits for the response with matching sequence
// number, honoring ctx cancellation/deadline.
func (m *Manager) Request(ctx context.Context, frame protocol.OutboundFrame) (protocol.InboundFrame, error) {
	ch := m.requests.Create(frame.Seq)

	data, err := m.Codec.Encode(frame)
	if err != nil {
		m.requests.Discard(frame.Seq)
		return protocol.InboundFrame{}, err
	}

	if err := m.Transport.Send(ctx, data); err != nil {
		m.requests.Discard(frame.Seq)
		return protocol.InboundFrame{}, err
	}

	select {
	case result, ok := <-ch:
		if !ok {
			return protocol.InboundFrame{}, context.Canceled
		}
		if result.err != nil {
			return protocol.InboundFrame{}, result.err
		}
		return result.frame, nil
	case <-ctx.Done():
		m.requests.Discard(frame.Seq)
		return protocol.InboundFrame{}, ctx.Err()
	}
}

// WaitClosed blocks until the receive loop has exited, returning the error
// that caused it to stop (nil for a clean, requested close).
func (m *Manager) WaitClosed(ctx context.Context) error {
	m.mu.Lock()
	done := m.recvDone
	m.mu.Unlock()

	if done == nil {
		return nil
	}

	select {
	case <-done:
		m.mu.Lock()
		err := m.recvErr
		lost := m.connectionLost
		m.mu.Unlock()
		if lost && err != nil {
			return fmt.Errorf("%w: %v", ErrConnectionLost, err)
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) recvLoop(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		done := m.recvDone
		m.mu.Unlock()
		if done != nil {
			close(done)
		}
	}()

	for {
		raw, err := m.Reader.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Requested shutdown; not a connection failure.
				return
			}

			m.mu.Lock()
			m.recvErr = err
			m.connectionLost = true
			m.mu.Unlock()

			m.requests.CancelAll(fmt.Errorf("%w: %v", ErrConnectionLost, err))
			m.markClosed(err)
			return
		}

		frame := m.Codec.Decode(raw)
		m.handleInbound(ctx, frame)
	}
}

func (m *Manager) handleInbound(ctx context.Context, frame protocol.InboundFrame) {
	if (frame.Cmd == protocol.CommandResponse || frame.Cmd == protocol.CommandError) && frame.Seq != nil {
		m.requests.Resolve(*frame.Seq, frame)
	}

	if m.OnEvent == nil {
		return
	}

	m.eventWG.Add(1)
	go func() {
		defer m.eventWG.Done()
		m.OnEvent(ctx, frame)
	}()
}

// NextSeq returns the next outbound sequence number, wrapping at 0x10000.
func (m *Manager) NextSeq() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq = (m.seq + 1) % 0x10000
	return m.seq
}

// IsOpen reports whether the connection is currently open.
func (m *Manager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isOpen
}

func (m *Manager) markClosed(err error) {
	m.mu.Lock()
	m.isOpen = false
	if m.closeReported {
		m.mu.Unlock()
		return
	}
	m.closeReported = true
	handler := m.OnClose
	m.mu.Unlock()

	if handler != nil {
		handler(err)
	}
}
