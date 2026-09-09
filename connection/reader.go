package connection

import (
	"context"

	"github.com/SonChegg/PyMax/protocol/tcp"
	"github.com/SonChegg/PyMax/transport"
)

// Reader reads exactly one raw protocol frame's worth of bytes from a
// Transport, a port of pymax's connection.readers.base.BaseReader.
type Reader interface {
	Read(ctx context.Context) ([]byte, error)
}

// TCPReader reads a fixed-header-prefixed TCP packet: it first reads the
// header to learn the payload length, then reads exactly that many more
// bytes. A port of pymax's connection.readers.tcp.TCPReader.
type TCPReader struct {
	Transport transport.Transport
	Framer    tcp.Framer
}

// NewTCPReader builds a reader for the raw TCP framing.
func NewTCPReader(t transport.Transport) *TCPReader {
	return &TCPReader{Transport: t}
}

// Read reads one full TCP packet (header + payload).
func (r *TCPReader) Read(ctx context.Context) ([]byte, error) {
	header, err := r.Transport.Recv(ctx, tcp.HeaderSize)
	if err != nil {
		return nil, err
	}

	payloadLen, err := r.Framer.UnpackHeader(header)
	if err != nil {
		return nil, err
	}

	if payloadLen == 0 {
		return header, nil
	}

	payload, err := r.Transport.Recv(ctx, payloadLen)
	if err != nil {
		return nil, err
	}

	return append(header, payload...), nil
}

// WSReader reads one full WebSocket message, a port of pymax's
// connection.readers.ws.WSReader.
type WSReader struct {
	Transport transport.Transport
}

// NewWSReader builds a reader for the WebSocket framing.
func NewWSReader(t transport.Transport) *WSReader {
	return &WSReader{Transport: t}
}

// Read reads the next WebSocket message.
func (r *WSReader) Read(ctx context.Context) ([]byte, error) {
	return r.Transport.Recv(ctx, 0)
}
