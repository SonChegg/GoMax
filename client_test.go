package gomax

import (
	"context"
	"testing"

	"github.com/SonChegg/PyMax/dispatch"
	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

func TestNewClientConstructsWithoutConnecting(t *testing.T) {
	client := NewClient("+79990000000", Config{})
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.IsConnected() {
		t.Fatal("a freshly constructed client should not report itself as connected")
	}
	if client.Me() != nil {
		t.Fatal("a freshly constructed client should not have a profile yet")
	}

	// Handler registration should not require a live runtime.
	client.OnMessage(func(ctx context.Context, msg *types.Message, c *Client) error { return nil })
	client.OnStart(func(ctx context.Context, c *Client) error { return nil })
}

func TestNewWebClientConstructsWithoutConnecting(t *testing.T) {
	client := NewWebClient(Config{})
	if client == nil {
		t.Fatal("NewWebClient returned nil")
	}
	if client.IsConnected() {
		t.Fatal("a freshly constructed client should not report itself as connected")
	}

	client.OnMessage(func(ctx context.Context, msg *types.Message, c *WebClient) error { return nil })
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{}
	cfg.withDefaults()

	if cfg.Host == "" || cfg.Port == 0 || cfg.URL == "" {
		t.Fatalf("expected non-zero connection defaults, got %+v", cfg)
	}
	if cfg.AppVersion == "" {
		t.Fatalf("expected a default app version, got %+v", cfg)
	}
}

// TestRuntimeWiresEnvInvoke guards against a regression where env.Invoke
// (the func field every api.*Service method calls) was left nil because
// newRuntime never bound it to the runtime's own invoke method — every
// live API call nil-panicked despite api package unit tests passing (they
// construct their own Env with Invoke stubbed directly, bypassing
// newRuntime entirely). This only surfaced against the real server.
func TestRuntimeWiresEnvInvoke(t *testing.T) {
	rt, err := newRuntime[*WebClient](Config{}, true, "", nil, dispatch.NewRouter[*WebClient]())
	if err != nil {
		t.Fatalf("newRuntime: %v", err)
	}
	if rt.env.Invoke == nil {
		t.Fatal("env.Invoke was not wired to the runtime's invoke method")
	}

	// Calling it before a connection is open should return a plain error,
	// not panic.
	_, err = rt.env.Invoke(context.Background(), protocol.OpcodePing, nil)
	if err == nil {
		t.Fatal("expected an error invoking before the runtime is connected")
	}
}
