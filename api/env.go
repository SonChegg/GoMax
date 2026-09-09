// Package api implements the typed request/response layer on top of the
// wire protocol: building payloads, invoking opcodes and parsing responses
// into domain types. It is a port of pymax's api package together with the
// runtime state pymax keeps on its App object.
package api

import (
	"context"
	"sync"
	"time"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/session"
	"github.com/SonChegg/PyMax/types"
)

// RegistrationConfig holds the profile data used to finish registering a
// brand-new account after SMS verification, a port of pymax's
// config.RegistrationConfig.
type RegistrationConfig struct {
	FirstName string
	LastName  string
}

// HandshakeResponse is returned by SessionsService.Handshake, a port of
// pymax's types.domain.handshake.HandshakeResponse.
type HandshakeResponse struct {
	CallsSeed     *int64 `json:"callsSeed"`
	AppUpdateType *int   `json:"app-update-type"`
}

// InvokeFunc sends a request frame for opcode/payload and returns the
// decoded response, a port of pymax's App.invoke. It is supplied by the
// root gomax package, which owns the connection.Manager and sequence
// numbering.
type InvokeFunc func(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error)

// FingerprintFunc generates the mode/chat-cache-fingerprint bytes pymax's
// mobile client sends on auth/login requests. It is nil for WEB sessions.
// Supplied by the root gomax package, backed by auth.FingerprintGenerator.
type FingerprintFunc func(deviceID string, callsSeed int64, arch string) ([]byte, error)

// Env is the shared runtime state and dependencies every api service needs,
// a port of pymax's App (the parts services reach into via self.app.*).
type Env struct {
	Invoke              InvokeFunc
	GenerateFingerprint FingerprintFunc
	Store               session.Store

	Phone               string
	DeviceID            string
	MtInstanceID        string
	Proxy               string
	UploadTimeout       time.Duration
	RegistrationConfig  *RegistrationConfig
	PasswordMaxAttempts *int
	Sync                session.SyncOverrides
	AppVersion          string

	mu                sync.Mutex
	interactive       bool
	userAgent         session.UserAgent
	sess              *session.Info
	handshakeResponse *HandshakeResponse
	me                *types.Profile
	chats             []*types.Chat
	users             map[int64]*types.User
	contacts          []*types.User
	messages          map[int64][]*types.Message
}

// NewEnv builds an Env with empty caches.
func NewEnv() *Env {
	return &Env{
		interactive: true,
		users:       make(map[int64]*types.User),
		messages:    make(map[int64][]*types.Message),
	}
}

func (e *Env) SetInteractive(v bool) { e.mu.Lock(); e.interactive = v; e.mu.Unlock() }
func (e *Env) Interactive() bool     { e.mu.Lock(); defer e.mu.Unlock(); return e.interactive }

func (e *Env) SetUserAgent(ua session.UserAgent) { e.mu.Lock(); e.userAgent = ua; e.mu.Unlock() }
func (e *Env) UserAgent() session.UserAgent      { e.mu.Lock(); defer e.mu.Unlock(); return e.userAgent }

func (e *Env) SetSession(s *session.Info) { e.mu.Lock(); e.sess = s; e.mu.Unlock() }
func (e *Env) Session() *session.Info     { e.mu.Lock(); defer e.mu.Unlock(); return e.sess }

func (e *Env) SetHandshakeResponse(h *HandshakeResponse) {
	e.mu.Lock()
	e.handshakeResponse = h
	e.mu.Unlock()
}
func (e *Env) HandshakeResponse() *HandshakeResponse {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.handshakeResponse
}

func (e *Env) SetMe(p *types.Profile) { e.mu.Lock(); e.me = p; e.mu.Unlock() }
func (e *Env) Me() *types.Profile     { e.mu.Lock(); defer e.mu.Unlock(); return e.me }

func (e *Env) SetChats(chats []*types.Chat) { e.mu.Lock(); e.chats = chats; e.mu.Unlock() }
func (e *Env) Chats() []*types.Chat {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.chats
}

func (e *Env) SetContacts(contacts []*types.User) { e.mu.Lock(); e.contacts = contacts; e.mu.Unlock() }
func (e *Env) Contacts() []*types.User {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.contacts
}

func (e *Env) SetMessages(messages map[int64][]*types.Message) {
	e.mu.Lock()
	e.messages = messages
	e.mu.Unlock()
}
func (e *Env) Messages() map[int64][]*types.Message {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.messages
}

func (e *Env) CacheUser(u *types.User) {
	e.mu.Lock()
	e.users[u.ID] = u
	e.mu.Unlock()
}

func (e *Env) CachedUser(id int64) *types.User {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.users[id]
}

func (e *Env) DropCachedUser(id int64) {
	e.mu.Lock()
	delete(e.users, id)
	e.mu.Unlock()
}

// CacheChat inserts or replaces chat in the chat cache by ID.
func (e *Env) CacheChat(chat *types.Chat) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, c := range e.chats {
		if c.ID == chat.ID {
			e.chats[i] = chat
			return
		}
	}
	e.chats = append(e.chats, chat)
}

func (e *Env) CachedChat(id int64) *types.Chat {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.chats {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (e *Env) DropCachedChat(id int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.chats[:0]
	for _, c := range e.chats {
		if c.ID != id {
			out = append(out, c)
		}
	}
	e.chats = out
}
