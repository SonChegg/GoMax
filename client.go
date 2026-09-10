// Package gomax is a Go port of PyMax (github.com/MaxApiTeam/PyMax), an
// unofficial API client library for the "MAX" messenger. Client is the raw
// TCP/SMS entry point; WebClient (see webclient.go) is the WebSocket/QR
// entry point.
package gomax

import (
	"context"
	"fmt"
	"time"

	"github.com/SonChegg/PyMax/auth"
	"github.com/SonChegg/PyMax/dispatch"
	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// Client is the TCP client with SMS-based authentication, a port of
// pymax's client.Client.
//
// Example:
//
//	client := gomax.NewClient("+79990000000", gomax.Config{})
//	client.OnMessage(func(ctx context.Context, msg *types.Message, c *gomax.Client) error {
//		_, err := c.API().Messages.SendMessage(ctx, *msg.ChatID, "hi", 0, nil, true, nil)
//		return err
//	})
//	if err := client.Start(context.Background()); err != nil {
//		log.Fatal(err)
//	}
type Client struct {
	phone  string
	cfg    Config
	router *dispatch.Router[*Client]
	rt     *runtime[*Client]
}

// NewClient builds a TCP client for phone. cfg may be the zero value to use
// every pymax default (host api2.oneme.ru:443, SMS auth via the console,
// SQLite session persisted to "./session.db", automatic reconnect).
func NewClient(phone string, cfg Config) *Client {
	return &Client{phone: phone, cfg: cfg, router: dispatch.NewRouter[*Client]()}
}

func (c *Client) resolveAuthFlow() auth.Flow {
	if c.cfg.AuthFlow != nil {
		return c.cfg.AuthFlow
	}
	codeProvider := c.cfg.SmsCodeProvider
	if codeProvider == nil {
		codeProvider = auth.ConsoleSmsCodeProvider{}
	}
	return auth.NewSmsFlow(codeProvider, c.cfg.PasswordProvider)
}

func (c *Client) ensureRuntime() error {
	if c.rt != nil {
		return nil
	}
	rt, err := newRuntime(c.cfg, false, c.phone, c.resolveAuthFlow(), c.router)
	if err != nil {
		return err
	}
	rt.dispatcher.BindClient(c)
	c.rt = rt
	return nil
}

// API exposes every typed request/response service (messages, chats,
// users, uploads, sessions, auth). Only valid after the client has started.
func (c *Client) API() *Facade {
	if c.rt == nil {
		return nil
	}
	return c.rt.api
}

// Me returns the authenticated account's profile, or nil before Start/Connect.
func (c *Client) Me() *types.Profile {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Me()
}

// Chats returns the chats Max returned on login/sync.
func (c *Client) Chats() []*types.Chat {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Chats()
}

// Contacts returns the contacts Max returned on login/sync.
func (c *Client) Contacts() []*types.User {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Contacts()
}

// Messages returns the messages Max returned on login/sync, keyed by chat ID.
func (c *Client) Messages() map[int64][]*types.Message {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Messages()
}

// IsConnected reports whether the underlying connection is currently open.
func (c *Client) IsConnected() bool {
	return c.rt != nil && c.rt.isOpen()
}

// Connect opens the connection once and returns after on_start handlers
// have been scheduled, leaving the connection open. Unlike Start, it does
// not block waiting for the connection to close or run a reconnect loop; a
// port of pymax's BaseClient.connect.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.ensureRuntime(); err != nil {
		return err
	}
	if err := c.rt.open(ctx); err != nil {
		_ = c.Close(ctx)
		return err
	}
	c.rt.dispatcher.EmitStart(c)
	return nil
}

// Start connects the client and blocks, dispatching events until the
// connection closes. On network errors it reconnects automatically unless
// cfg.DisableReconnect is set; a revoked login token triggers
// re-authentication unless cfg.DisableRelogin is set. A port of pymax's
// BaseClient.start.
func (c *Client) Start(ctx context.Context) error {
	for {
		if err := c.ensureRuntime(); err != nil {
			return err
		}

		if err := c.rt.open(ctx); err != nil {
			_ = c.Close(ctx)
			return err
		}
		c.rt.dispatcher.EmitStart(c)

		err := c.rt.waitClosed(ctx)
		if err == nil {
			_ = c.Close(ctx)
			return nil
		}

		if apiErr, ok := err.(*APIError); ok && isInvalidLoginTokenError(apiErr) {
			if c.cfg.DisableRelogin {
				_ = c.Close(ctx)
				return err
			}

			// Drop the revoked session (drop_config_token=True, start=False
			// in pymax's terms) and let the loop rebuild+re-authenticate.
			if sess := c.rt.env.Session(); sess != nil && c.rt.store != nil {
				_ = c.rt.store.DeleteSession(ctx, sess.Token)
			}
			_ = c.Close(ctx)
			c.cfg.Token = ""
			c.rt = nil
			continue
		}

		_ = c.Close(ctx)
		c.rt.dispatcher.EmitDisconnect(err, !c.cfg.DisableReconnect, c.cfg.ReconnectDelay.Seconds())

		if c.cfg.DisableReconnect {
			return err
		}

		select {
		case <-time.After(c.cfg.ReconnectDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
		c.rt = nil
	}
}

func isInvalidLoginTokenError(err *APIError) bool {
	if err.Code != "FAIL_LOGIN_TOKEN" && err.Message != "FAIL_LOGIN_TOKEN" &&
		err.Code != "FAIL_LOGOUT_ALL" && err.Message != "FAIL_LOGOUT_ALL" {
		return false
	}
	return true
}

// Close closes the connection, background tasks and session store.
func (c *Client) Close(ctx context.Context) error {
	if c.rt == nil {
		return nil
	}
	return c.rt.close(ctx)
}

// Stop is an alias for Close.
func (c *Client) Stop(ctx context.Context) error { return c.Close(ctx) }

// Relogin deletes the current local session and re-authenticates from
// scratch, a port of pymax's BaseClient.relogin.
func (c *Client) Relogin(ctx context.Context, dropConfigToken bool) error {
	if c.rt == nil {
		return fmt.Errorf("gomax: cannot relogin before the client has started")
	}
	sess := c.rt.env.Session()
	if sess == nil {
		return fmt.Errorf("gomax: cannot relogin before session is loaded")
	}
	if c.rt.store != nil {
		_ = c.rt.store.DeleteSession(ctx, sess.Token)
	}
	_ = c.Close(ctx)

	if dropConfigToken {
		c.cfg.Token = ""
	}
	c.rt = nil
	return c.Start(ctx)
}

// IncludeRouter attaches a child router to this client's root router.
func (c *Client) IncludeRouter(router *dispatch.Router[*Client]) {
	c.router.IncludeRouter(router)
}

// OnStart registers a handler run once the client finishes starting.
func (c *Client) OnStart(handler dispatch.StartHandler[*Client]) { c.router.OnStart(handler) }

// OnMessage registers a handler for new messages.
func (c *Client) OnMessage(handler func(ctx context.Context, msg *types.Message, client *Client) error, filters ...func(*types.Message) bool) {
	c.router.OnMessage(handler, filters...)
}

// OnMessageEdit registers a handler for edited messages.
func (c *Client) OnMessageEdit(handler func(ctx context.Context, msg *types.Message, client *Client) error, filters ...func(*types.Message) bool) {
	c.router.OnMessageEdit(handler, filters...)
}

// OnMessageDelete registers a handler for deleted messages.
func (c *Client) OnMessageDelete(handler func(ctx context.Context, event *types.MessageDeleteEvent, client *Client) error, filters ...func(*types.MessageDeleteEvent) bool) {
	c.router.OnMessageDelete(handler, filters...)
}

// OnMessageRead registers a handler for read-marker updates.
func (c *Client) OnMessageRead(handler func(ctx context.Context, event *types.MessageReadEvent, client *Client) error, filters ...func(*types.MessageReadEvent) bool) {
	c.router.OnMessageRead(handler, filters...)
}

// OnTyping registers a handler for typing notifications.
func (c *Client) OnTyping(handler func(ctx context.Context, event *types.TypingEvent, client *Client) error, filters ...func(*types.TypingEvent) bool) {
	c.router.OnTyping(handler, filters...)
}

// OnPresence registers a handler for presence changes.
func (c *Client) OnPresence(handler func(ctx context.Context, event *types.PresenceEvent, client *Client) error, filters ...func(*types.PresenceEvent) bool) {
	c.router.OnPresence(handler, filters...)
}

// OnReactionUpdate registers a handler for reaction changes.
func (c *Client) OnReactionUpdate(handler func(ctx context.Context, event *types.ReactionUpdateEvent, client *Client) error, filters ...func(*types.ReactionUpdateEvent) bool) {
	c.router.OnReactionUpdate(handler, filters...)
}

// OnChatUpdate registers a handler for chat updates.
func (c *Client) OnChatUpdate(handler func(ctx context.Context, chat *types.Chat, client *Client) error, filters ...func(*types.Chat) bool) {
	c.router.OnChatUpdate(handler, filters...)
}

// OnRaw registers a handler for every inbound frame, decoded or not.
func (c *Client) OnRaw(handler func(ctx context.Context, frame protocol.InboundFrame, client *Client) error) {
	c.router.OnRaw(handler)
}

// OnError registers a handler for errors raised by other handlers or by
// the startup sequence.
func (c *Client) OnError(scope dispatch.ErrorScope, handler dispatch.ErrorHandler[*Client]) {
	c.router.OnError(scope, handler)
}

// OnDisconnect registers a handler run when the connection is lost, before
// an optional reconnect.
func (c *Client) OnDisconnect(handler dispatch.DisconnectHandler) {
	c.router.OnDisconnect(handler)
}
