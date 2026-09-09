package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

const wsMaxMessageSize = 10 * 1024 * 1024 // 10 MB, matches pymax's max_size

// WebSocketTransport is a WebSocket transport to the Max web API, a port of
// pymax's transport.websocket.WebSocketTransport.
type WebSocketTransport struct {
	url   string
	proxy string

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewWebSocketTransport builds a WebSocket transport for the given URL,
// optionally tunneled through proxyURL.
func NewWebSocketTransport(wsURL string, proxyURL string) *WebSocketTransport {
	return &WebSocketTransport{url: wsURL, proxy: proxyURL}
}

// Connect dials the WebSocket endpoint, sending the Origin header pymax uses
// ("https://web.max.ru").
func (t *WebSocketTransport) Connect(ctx context.Context) error {
	dialer := *websocket.DefaultDialer
	dialer.ReadBufferSize = wsMaxMessageSize

	if t.proxy != "" {
		proxyURL, err := url.Parse(t.proxy)
		if err != nil {
			return fmt.Errorf("transport: invalid proxy url: %w", err)
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}

	header := http.Header{}
	header.Set("Origin", "https://web.max.ru")

	conn, _, err := dialer.DialContext(ctx, t.url, header)
	if err != nil {
		return fmt.Errorf("transport: websocket connect: %w", err)
	}
	conn.SetReadLimit(wsMaxMessageSize)

	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()
	return nil
}

// Close closes the WebSocket connection.
func (t *WebSocketTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	conn := t.conn
	t.conn = nil
	t.mu.Unlock()

	if conn == nil {
		return nil
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return conn.Close()
}

// Send writes a text frame to the WebSocket (Max's WebSocket protocol is
// JSON-over-text).
func (t *WebSocketTransport) Send(ctx context.Context, data []byte) error {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("transport: not connected to the server")
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// Recv reads the next full WebSocket message. n is ignored (WebSocket is
// message-oriented, unlike the raw TCP transport).
func (t *WebSocketTransport) Recv(ctx context.Context, n int) ([]byte, error) {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("transport: not connected to the server")
	}

	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Connected reports whether the WebSocket connection is open.
func (t *WebSocketTransport) Connected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}
