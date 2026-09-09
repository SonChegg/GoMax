package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/SonChegg/PyMax/internal/data"
)

const closeTimeout = 5 * time.Second

// TCPTransport is a raw TLS/TCP transport to the Max mobile API, a port of
// pymax's transport.tcp.TCPTransport.
type TCPTransport struct {
	host   string
	port   int
	proxy  string
	useTLS bool

	tlsConfig *tls.Config

	mu   sync.Mutex
	conn net.Conn
}

// NewTCPTransport builds a TCP transport for the given host/port, optionally
// tunneled through proxyURL ("socks5://" or "http://"/"https://").
func NewTCPTransport(host string, port int, proxyURL string, useTLS bool) (*TCPTransport, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		// A port of pymax's transport.tcp.TCPTransport, which builds its SSL
		// context with ssl.create_default_context() (the OS trust store)
		// and then layers the embedded CA on top via
		// load_verify_locations() rather than replacing it. Falling back to
		// an empty pool here (e.g. on platforms without a system trust
		// store) still lets the embedded CA below be trusted.
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data.RootCACert) {
		return nil, fmt.Errorf("transport: failed to parse embedded root CA certificate")
	}

	return &TCPTransport{
		host:   host,
		port:   port,
		proxy:  proxyURL,
		useTLS: useTLS,
		tlsConfig: &tls.Config{
			RootCAs:    pool,
			ServerName: host,
		},
	}, nil
}

// Connect opens the TCP connection, optionally through a proxy, and
// optionally wraps it in TLS.
func (t *TCPTransport) Connect(ctx context.Context) error {
	addr := net.JoinHostPort(t.host, fmt.Sprintf("%d", t.port))

	rawConn, err := t.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("transport: tcp connect: %w", err)
	}

	if t.useTLS {
		tlsConn := tls.Client(rawConn, t.tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return fmt.Errorf("transport: tls handshake: %w", err)
		}
		rawConn = tlsConn
	}

	t.mu.Lock()
	t.conn = rawConn
	t.mu.Unlock()
	return nil
}

func (t *TCPTransport) dial(ctx context.Context, addr string) (net.Conn, error) {
	if t.proxy == "" {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}

	proxyURL, err := url.Parse(t.proxy)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}

	switch proxyURL.Scheme {
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, err
		}
		if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
			return ctxDialer.DialContext(ctx, "tcp", addr)
		}
		return dialer.Dial("tcp", addr)
	case "http", "https":
		return dialHTTPConnect(ctx, proxyURL, addr)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}
}

// dialHTTPConnect tunnels a TCP connection through an HTTP(S) proxy using
// the CONNECT method.
func dialHTTPConnect(ctx context.Context, proxyURL *url.URL, addr string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyURL.Host)
	if err != nil {
		return nil, err
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}
	if proxyURL.User != nil {
		if pw, ok := proxyURL.User.Password(); ok {
			connectReq.SetBasicAuth(proxyURL.User.Username(), pw)
		}
	}

	if err := connectReq.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), connectReq)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", resp.Status)
	}

	return conn, nil
}

// Close closes the underlying connection, aborting it if a clean close
// takes too long.
func (t *TCPTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	conn := t.conn
	t.conn = nil
	t.mu.Unlock()

	if conn == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() { done <- conn.Close() }()

	select {
	case err := <-done:
		return err
	case <-time.After(closeTimeout):
		return conn.Close()
	}
}

// Send writes data to the connection, blocking until fully written.
func (t *TCPTransport) Send(ctx context.Context, data []byte) error {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("transport: not connected to the server")
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(deadline)
		defer conn.SetWriteDeadline(time.Time{})
	}

	_, err := conn.Write(data)
	return err
}

// Recv reads exactly n bytes from the connection.
func (t *TCPTransport) Recv(ctx context.Context, n int) ([]byte, error) {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("transport: not connected to the server")
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// Connected reports whether the socket is currently open.
func (t *TCPTransport) Connected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}
