package gomax

import (
	"context"
	"testing"

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
