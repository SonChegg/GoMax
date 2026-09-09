package dispatch

import (
	"context"
	"fmt"
	"sync"

	"github.com/SonChegg/PyMax/protocol"
)

// Dispatcher resolves inbound frames to typed events and fans them out to
// the root Router tree (plus an internal router used by the api package for
// upload-ready signals), a port of pymax's dispatch.dispatcher.Dispatcher.
type Dispatcher[C any] struct {
	RootRouter     *Router[C]
	InternalRouter *Router[C]

	client    C
	hasClient bool

	startCtx    context.Context
	startCancel context.CancelFunc
	startWG     sync.WaitGroup
}

// NewDispatcher builds a dispatcher around the given root router (or a
// fresh one if nil).
func NewDispatcher[C any](root *Router[C]) *Dispatcher[C] {
	if root == nil {
		root = NewRouter[C]()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher[C]{
		RootRouter:     root,
		InternalRouter: NewRouter[C](),
		startCtx:       ctx,
		startCancel:    cancel,
	}
}

// BindClient attaches the concrete client instance passed to every handler
// callback, a port of pymax's Dispatcher.bind_client.
func (d *Dispatcher[C]) BindClient(client C) {
	d.client = client
	d.hasClient = true
}

// IncludeRouter attaches a router to the root router's children.
func (d *Dispatcher[C]) IncludeRouter(router *Router[C]) {
	d.RootRouter.IncludeRouter(router)
}

// EmitStart runs every OnStart handler in the router tree concurrently, a
// port of pymax's Dispatcher.emit_start.
func (d *Dispatcher[C]) EmitStart(client C) {
	for _, r := range d.iterRouters(d.RootRouter) {
		for _, handler := range r.onStartHandlers {
			d.startWG.Add(1)
			go func(r *Router[C], handler StartHandler[C]) {
				defer d.startWG.Done()
				if err := handler(d.startCtx, client); err != nil {
					d.EmitError(err, EventOnStart, nil, r)
				}
			}(r, handler)
		}
	}
}

// StopStartupTasks cancels and waits for any still-running OnStart
// handlers, a port of pymax's Dispatcher.stop_startup_tasks.
func (d *Dispatcher[C]) StopStartupTasks() {
	d.startCancel()
	d.startWG.Wait()
}

// Dispatch resolves and routes one inbound frame, a port of pymax's
// Dispatcher.dispatch.
func (d *Dispatcher[C]) Dispatch(ctx context.Context, frame protocol.InboundFrame) {
	eventType, ok := Resolve(frame)
	if ok {
		event := Map(eventType, frame)
		d.dispatchToRouter(ctx, d.InternalRouter, eventType, event)
		d.dispatchToRouter(ctx, d.RootRouter, eventType, event)
	}

	d.dispatchToRouter(ctx, d.RootRouter, EventRaw, frame)
}

func (d *Dispatcher[C]) dispatchToRouter(ctx context.Context, router *Router[C], eventType EventType, event any) {
	for _, entry := range router.handlers[eventType] {
		if !matchesFilters(entry.filters, event) {
			continue
		}

		if err := d.call(ctx, entry.callback, event); err != nil {
			d.EmitError(err, eventType, event, router)
		}
	}

	for _, child := range router.children {
		d.dispatchToRouter(ctx, child, eventType, event)
	}
}

func matchesFilters(filters []Filter, event any) bool {
	for _, f := range filters {
		if !f(event) {
			return false
		}
	}
	return true
}

func (d *Dispatcher[C]) call(ctx context.Context, callback Handler[C], event any) (err error) {
	if !d.hasClient {
		return fmt.Errorf("dispatch: client is not bound")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("dispatch: handler panicked: %v", r)
		}
	}()
	return callback(ctx, event, d.client)
}

// EmitError runs every matching error handler in the router tree, a port of
// pymax's Dispatcher.emit_error. LOCAL-scoped handlers only see errors from
// their own router; GLOBAL-scoped handlers see the whole tree.
func (d *Dispatcher[C]) EmitError(err error, eventType EventType, event any, failedRouter *Router[C]) bool {
	if !d.hasClient {
		return false
	}

	handled := false
	ctx := ErrorContext[C]{Client: d.client, EventType: eventType, Event: event, Router: failedRouter}

	for _, r := range d.iterRouters(d.RootRouter) {
		for _, entry := range r.errorHandlers {
			if entry.scope == ErrorScopeLocal && r != failedRouter {
				continue
			}
			handled = true
			entry.callback(err, ctx)
		}
	}
	return handled
}

// EmitDisconnect runs every OnDisconnect handler in the router tree, a port
// of pymax's Dispatcher.emit_disconnect.
func (d *Dispatcher[C]) EmitDisconnect(err error, willReconnect bool, delay float64) {
	for _, r := range d.iterRouters(d.RootRouter) {
		for _, handler := range r.disconnectHandlers {
			handler(err, willReconnect, delay)
		}
	}
}

func (d *Dispatcher[C]) iterRouters(root *Router[C]) []*Router[C] {
	out := []*Router[C]{root}
	for _, child := range root.children {
		out = append(out, d.iterRouters(child)...)
	}
	return out
}
