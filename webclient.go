package gomax

import (
	"context"
	"time"

	"github.com/SonChegg/PyMax/auth"
	"github.com/SonChegg/PyMax/dispatch"
	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// WebClient is the WebSocket client with QR-based authentication, a port
// of pymax's client_web.WebClient.
//
// Example:
//
//	client := gomax.NewWebClient(gomax.Config{})
//	client.OnMessage(func(ctx context.Context, msg *types.Message, c *gomax.WebClient) error {
//		return nil
//	})
//	if err := client.Start(context.Background()); err != nil {
//		log.Fatal(err)
//	}
type WebClient struct {
	cfg    Config
	router *dispatch.Router[*WebClient]
	rt     *runtime[*WebClient]
}

// NewWebClient builds a WebSocket client. cfg may be the zero value to use
// every pymax default (wss://api.oneme.ru/websocket, QR auth via the
// console, SQLite session persisted to "./session.db", automatic reconnect).
func NewWebClient(cfg Config) *WebClient {
	return &WebClient{cfg: cfg, router: dispatch.NewRouter[*WebClient]()}
}

func (c *WebClient) resolveAuthFlow() auth.Flow {
	if c.cfg.AuthFlow != nil {
		return c.cfg.AuthFlow
	}
	qrProvider := c.cfg.QrProvider
	if qrProvider == nil {
		qrProvider = auth.ConsoleQrHandler{}
	}
	return auth.NewQrFlow(qrProvider, c.cfg.PasswordProvider)
}

func (c *WebClient) ensureRuntime() error {
	if c.rt != nil {
		return nil
	}
	rt, err := newRuntime(c.cfg, true, "", c.resolveAuthFlow(), c.router)
	if err != nil {
		return err
	}
	rt.dispatcher.BindClient(c)
	c.rt = rt
	return nil
}

// API exposes every typed request/response service (messages, chats,
// users, uploads, sessions, auth). Only valid after the client has started.
func (c *WebClient) API() *Facade {
	if c.rt == nil {
		return nil
	}
	return c.rt.api
}

// Me returns the authenticated account's profile, or nil before Start/Connect.
func (c *WebClient) Me() *types.Profile {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Me()
}

// Chats returns the chats Max returned on login/sync.
func (c *WebClient) Chats() []*types.Chat {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Chats()
}

// Contacts returns the contacts Max returned on login/sync.
func (c *WebClient) Contacts() []*types.User {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Contacts()
}

// Messages returns the messages Max returned on login/sync, keyed by chat ID.
func (c *WebClient) Messages() map[int64][]*types.Message {
	if c.rt == nil {
		return nil
	}
	return c.rt.env.Messages()
}

// IsConnected reports whether the underlying connection is currently open.
func (c *WebClient) IsConnected() bool {
	return c.rt != nil && c.rt.isOpen()
}

// Connect opens the connection once and returns after on_start handlers
// have been scheduled, leaving the connection open. A port of pymax's
// BaseClient.connect.
func (c *WebClient) Connect(ctx context.Context) error {
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
// connection closes, reconnecting on network errors unless
// cfg.DisableReconnect is set. A port of pymax's BaseClient.start.
func (c *WebClient) Start(ctx context.Context) error {
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

// Close closes the connection, background tasks and session store.
func (c *WebClient) Close(ctx context.Context) error {
	if c.rt == nil {
		return nil
	}
	return c.rt.close(ctx)
}

// Stop is an alias for Close.
func (c *WebClient) Stop(ctx context.Context) error { return c.Close(ctx) }

// IncludeRouter attaches a child router to this client's root router.
func (c *WebClient) IncludeRouter(router *dispatch.Router[*WebClient]) {
	c.router.IncludeRouter(router)
}

// OnStart registers a handler run once the client finishes starting.
func (c *WebClient) OnStart(handler dispatch.StartHandler[*WebClient]) { c.router.OnStart(handler) }

// OnMessage registers a handler for new messages.
func (c *WebClient) OnMessage(handler func(ctx context.Context, msg *types.Message, client *WebClient) error, filters ...func(*types.Message) bool) {
	c.router.OnMessage(handler, filters...)
}

// OnMessageEdit registers a handler for edited messages.
func (c *WebClient) OnMessageEdit(handler func(ctx context.Context, msg *types.Message, client *WebClient) error, filters ...func(*types.Message) bool) {
	c.router.OnMessageEdit(handler, filters...)
}

// OnMessageDelete registers a handler for deleted messages.
func (c *WebClient) OnMessageDelete(handler func(ctx context.Context, event *types.MessageDeleteEvent, client *WebClient) error, filters ...func(*types.MessageDeleteEvent) bool) {
	c.router.OnMessageDelete(handler, filters...)
}

// OnMessageRead registers a handler for read-marker updates.
func (c *WebClient) OnMessageRead(handler func(ctx context.Context, event *types.MessageReadEvent, client *WebClient) error, filters ...func(*types.MessageReadEvent) bool) {
	c.router.OnMessageRead(handler, filters...)
}

// OnTyping registers a handler for typing notifications.
func (c *WebClient) OnTyping(handler func(ctx context.Context, event *types.TypingEvent, client *WebClient) error, filters ...func(*types.TypingEvent) bool) {
	c.router.OnTyping(handler, filters...)
}

// OnPresence registers a handler for presence changes.
func (c *WebClient) OnPresence(handler func(ctx context.Context, event *types.PresenceEvent, client *WebClient) error, filters ...func(*types.PresenceEvent) bool) {
	c.router.OnPresence(handler, filters...)
}

// OnReactionUpdate registers a handler for reaction changes.
func (c *WebClient) OnReactionUpdate(handler func(ctx context.Context, event *types.ReactionUpdateEvent, client *WebClient) error, filters ...func(*types.ReactionUpdateEvent) bool) {
	c.router.OnReactionUpdate(handler, filters...)
}

// OnChatUpdate registers a handler for chat updates.
func (c *WebClient) OnChatUpdate(handler func(ctx context.Context, chat *types.Chat, client *WebClient) error, filters ...func(*types.Chat) bool) {
	c.router.OnChatUpdate(handler, filters...)
}

// OnRaw registers a handler for every inbound frame, decoded or not.
func (c *WebClient) OnRaw(handler func(ctx context.Context, frame protocol.InboundFrame, client *WebClient) error) {
	c.router.OnRaw(handler)
}

// OnError registers a handler for errors raised by other handlers or by
// the startup sequence.
func (c *WebClient) OnError(scope dispatch.ErrorScope, handler dispatch.ErrorHandler[*WebClient]) {
	c.router.OnError(scope, handler)
}

// OnDisconnect registers a handler run when the connection is lost, before
// an optional reconnect.
func (c *WebClient) OnDisconnect(handler dispatch.DisconnectHandler) {
	c.router.OnDisconnect(handler)
}
