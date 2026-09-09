// Package dispatch routes decoded inbound frames to registered handlers, a
// port of pymax's dispatch package (dispatcher.py, router.py, mapping.py,
// resolvers.py, enums.py).
package dispatch

import (
	"context"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// EventType identifies a decoded, dispatchable event kind, a port of
// pymax's dispatch.enums.EventType.
type EventType string

const (
	EventMessageNew     EventType = "message_new"
	EventMessageEdit    EventType = "message_edit"
	EventMessageDelete  EventType = "message_delete"
	EventMessageRead    EventType = "message_read"
	EventTyping         EventType = "typing"
	EventPresence       EventType = "presence"
	EventReactionUpdate EventType = "reaction_update"
	EventChatUpdate     EventType = "chat_update"
	EventVideoReady     EventType = "video_ready"
	EventFileReady      EventType = "file_ready"
	EventVoiceReady     EventType = "voice_ready"
	EventRaw            EventType = "raw"
	EventOnStart        EventType = "on_start"
)

// ErrorScope controls whether an error handler sees failures from its own
// Router only or from the whole tree, a port of pymax's
// dispatch.router.ErrorScope.
type ErrorScope string

const (
	ErrorScopeGlobal ErrorScope = "global"
	ErrorScopeLocal  ErrorScope = "local"
)

// Filter is a predicate evaluated against a decoded event before its
// handler runs, a port of pymax's dispatch.router.FilterCallback.
type Filter func(event any) bool

// Handler is a registered event callback, a port of pymax's
// dispatch.router.HandlerCallback. event's concrete type depends on the
// EventType it was registered for (see the typed On* helpers below).
type Handler[C any] func(ctx context.Context, event any, client C) error

// StartHandler runs once the client has finished its startup sequence, a
// port of pymax's dispatch.router.StartCallback.
type StartHandler[C any] func(ctx context.Context, client C) error

// DisconnectHandler runs when the connection is lost, before an optional
// reconnect, a port of pymax's dispatch.router.DisconnectCallback.
type DisconnectHandler func(err error, willReconnect bool, delay float64)

// ErrorContext carries the failure that triggered an error handler, a port
// of pymax's dispatch.router.ErrorContext.
type ErrorContext[C any] struct {
	Client    C
	EventType EventType
	Event     any
	Router    *Router[C]
}

// ErrorHandler observes a panic/error raised by another handler, a port of
// pymax's dispatch.router.ErrorCallback.
type ErrorHandler[C any] func(err error, ctx ErrorContext[C])

type handlerEntry[C any] struct {
	callback Handler[C]
	filters  []Filter
}

type errorEntry[C any] struct {
	callback ErrorHandler[C]
	scope    ErrorScope
}

// Router is a container of event handlers, a port of pymax's
// dispatch.router.Router. Routers can be nested via IncludeRouter; the
// Dispatcher walks the whole tree on every inbound frame.
type Router[C any] struct {
	handlers           map[EventType][]handlerEntry[C]
	children           []*Router[C]
	onStartHandlers    []StartHandler[C]
	errorHandlers      []errorEntry[C]
	disconnectHandlers []DisconnectHandler
}

// NewRouter builds an empty router.
func NewRouter[C any]() *Router[C] {
	return &Router[C]{handlers: make(map[EventType][]handlerEntry[C])}
}

// IncludeRouter attaches a child router whose handlers/filters are also
// consulted during dispatch.
func (r *Router[C]) IncludeRouter(child *Router[C]) {
	r.children = append(r.children, child)
}

// On registers a handler for the given event type, evaluated only if every
// filter returns true, a port of pymax's dispatch.router.Router.on.
func (r *Router[C]) On(event EventType, handler Handler[C], filters ...Filter) {
	r.handlers[event] = append(r.handlers[event], handlerEntry[C]{callback: handler, filters: filters})
}

// OnError registers an error handler, a port of pymax's
// dispatch.router.Router.on_error.
func (r *Router[C]) OnError(scope ErrorScope, handler ErrorHandler[C]) {
	if scope == "" {
		scope = ErrorScopeGlobal
	}
	r.errorHandlers = append(r.errorHandlers, errorEntry[C]{callback: handler, scope: scope})
}

// OnDisconnect registers a handler run when the connection is lost, a port
// of pymax's dispatch.router.Router.on_disconnect.
func (r *Router[C]) OnDisconnect(handler DisconnectHandler) {
	r.disconnectHandlers = append(r.disconnectHandlers, handler)
}

// OnStart registers a handler run once the client finishes starting, a
// port of pymax's dispatch.router.Router.on_start.
func (r *Router[C]) OnStart(handler StartHandler[C]) {
	r.onStartHandlers = append(r.onStartHandlers, handler)
}

// The following typed helpers adapt strongly-typed callbacks/filters into
// the generic Handler[C]/Filter machinery, matching pymax's
// on_message/on_message_edit/on_typing/... convenience methods.

func filtersOf[T any](filters []func(T) bool) []Filter {
	out := make([]Filter, len(filters))
	for i, f := range filters {
		f := f
		out[i] = func(event any) bool {
			v, ok := event.(T)
			if !ok {
				return false
			}
			return f(v)
		}
	}
	return out
}

func handlerOf[C, T any](handler func(ctx context.Context, event T, client C) error) Handler[C] {
	return func(ctx context.Context, event any, client C) error {
		v, ok := event.(T)
		if !ok {
			return nil
		}
		return handler(ctx, v, client)
	}
}

// OnMessage registers a handler for new messages.
func (r *Router[C]) OnMessage(handler func(ctx context.Context, msg *types.Message, client C) error, filters ...func(*types.Message) bool) {
	r.On(EventMessageNew, handlerOf[C, *types.Message](handler), filtersOf(filters)...)
}

// OnMessageEdit registers a handler for edited messages.
func (r *Router[C]) OnMessageEdit(handler func(ctx context.Context, msg *types.Message, client C) error, filters ...func(*types.Message) bool) {
	r.On(EventMessageEdit, handlerOf[C, *types.Message](handler), filtersOf(filters)...)
}

// OnMessageDelete registers a handler for deleted messages.
func (r *Router[C]) OnMessageDelete(handler func(ctx context.Context, event *types.MessageDeleteEvent, client C) error, filters ...func(*types.MessageDeleteEvent) bool) {
	r.On(EventMessageDelete, handlerOf[C, *types.MessageDeleteEvent](handler), filtersOf(filters)...)
}

// OnMessageRead registers a handler for read-marker updates.
func (r *Router[C]) OnMessageRead(handler func(ctx context.Context, event *types.MessageReadEvent, client C) error, filters ...func(*types.MessageReadEvent) bool) {
	r.On(EventMessageRead, handlerOf[C, *types.MessageReadEvent](handler), filtersOf(filters)...)
}

// OnTyping registers a handler for typing notifications.
func (r *Router[C]) OnTyping(handler func(ctx context.Context, event *types.TypingEvent, client C) error, filters ...func(*types.TypingEvent) bool) {
	r.On(EventTyping, handlerOf[C, *types.TypingEvent](handler), filtersOf(filters)...)
}

// OnPresence registers a handler for presence changes.
func (r *Router[C]) OnPresence(handler func(ctx context.Context, event *types.PresenceEvent, client C) error, filters ...func(*types.PresenceEvent) bool) {
	r.On(EventPresence, handlerOf[C, *types.PresenceEvent](handler), filtersOf(filters)...)
}

// OnReactionUpdate registers a handler for reaction changes.
func (r *Router[C]) OnReactionUpdate(handler func(ctx context.Context, event *types.ReactionUpdateEvent, client C) error, filters ...func(*types.ReactionUpdateEvent) bool) {
	r.On(EventReactionUpdate, handlerOf[C, *types.ReactionUpdateEvent](handler), filtersOf(filters)...)
}

// OnChatUpdate registers a handler for chat updates.
func (r *Router[C]) OnChatUpdate(handler func(ctx context.Context, chat *types.Chat, client C) error, filters ...func(*types.Chat) bool) {
	r.On(EventChatUpdate, handlerOf[C, *types.Chat](handler), filtersOf(filters)...)
}

// OnRaw registers a handler for every inbound frame, decoded or not.
func (r *Router[C]) OnRaw(handler func(ctx context.Context, frame protocol.InboundFrame, client C) error, filters ...func(protocol.InboundFrame) bool) {
	r.On(EventRaw, handlerOf[C, protocol.InboundFrame](handler), filtersOf(filters)...)
}
