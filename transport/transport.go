// Package transport defines the low-level byte transports used by the Max
// protocol: a raw TLS/TCP socket transport and a WebSocket transport.
package transport

import "context"

// Transport is the shared contract implemented by TCPTransport and
// WebSocketTransport. It mirrors pymax's transport.base.Transport protocol.
type Transport interface {
	// Connect establishes the underlying connection.
	Connect(ctx context.Context) error
	// Close closes the underlying connection.
	Close(ctx context.Context) error
	// Send writes raw bytes (or a text frame, for WebSocket) to the peer.
	Send(ctx context.Context, data []byte) error
	// Recv reads exactly n bytes for TCP, or the next full message for
	// WebSocket (n is ignored in that case).
	Recv(ctx context.Context, n int) ([]byte, error)
	// Connected reports whether the transport currently has a live connection.
	Connected() bool
}
