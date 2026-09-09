// Package session defines the persisted client session (login token, device
// identity and sync markers) and its storage contract, a port of pymax's
// session package (session/models.py, session/protocol.py, session/store.py).
package session

import "context"

// DefaultConfigHash is the placeholder config hash used before the server
// has returned a real one, a port of pymax's session.protocol.DEFAULT_CONFIG_HASH.
const DefaultConfigHash = "00000000-0000000000000000-00000000-" +
	"0000000000000000-0000000000000000-0-" +
	"0000000000000000-00000000"

// SyncState is the full persisted sync-marker state for login, a port of
// pymax's session.models... via types.domain.sync.SyncState.
type SyncState struct {
	ChatsSync    int64  `json:"chats_sync"`
	ContactsSync int64  `json:"contacts_sync"`
	DraftsSync   int64  `json:"drafts_sync"`
	PresenceSync int64  `json:"presence_sync"`
	ConfigHash   string `json:"config_hash"`
}

// NewSyncState returns a SyncState with all markers reset (-1) and the
// default config hash, matching pymax's SyncState() defaults.
func NewSyncState() SyncState {
	return SyncState{ChatsSync: -1, ContactsSync: -1, DraftsSync: -1, PresenceSync: -1, ConfigHash: DefaultConfigHash}
}

// SyncOverrides holds optional per-field overrides for SyncState, a port of
// pymax's types.domain.sync.SyncOverrides.
type SyncOverrides struct {
	ChatsSync    *int64
	ContactsSync *int64
	DraftsSync   *int64
	PresenceSync *int64
	ConfigHash   *string
}

// Resolve merges overrides on top of a saved SyncState.
func (o SyncOverrides) Resolve(saved SyncState) SyncState {
	resolved := saved
	if o.ChatsSync != nil {
		resolved.ChatsSync = *o.ChatsSync
	}
	if o.ContactsSync != nil {
		resolved.ContactsSync = *o.ContactsSync
	}
	if o.DraftsSync != nil {
		resolved.DraftsSync = *o.DraftsSync
	}
	if o.PresenceSync != nil {
		resolved.PresenceSync = *o.PresenceSync
	}
	if o.ConfigHash != nil {
		resolved.ConfigHash = *o.ConfigHash
	}
	return resolved
}

// UserAgent is the device/app identity pymax sends on handshake and login,
// a port of pymax's api.session.payloads.MobileUserAgentPayload.
type UserAgent struct {
	DeviceType      string `json:"deviceType"`
	AppVersion      string `json:"appVersion"`
	OsVersion       string `json:"osVersion"`
	Timezone        string `json:"timezone"`
	Screen          string `json:"screen"`
	PushDeviceType  string `json:"pushDeviceType,omitempty"`
	Arch            string `json:"arch,omitempty"`
	Locale          string `json:"locale"`
	BuildNumber     int    `json:"buildNumber,omitempty"`
	DeviceName      string `json:"deviceName"`
	DeviceLocale    string `json:"deviceLocale"`
	Release         int    `json:"release,omitempty"`
	HeaderUserAgent string `json:"headerUserAgent,omitempty"`
}

// Info is the state of an authorized session persisted through Store, a
// port of pymax's session.models.SessionInfo.
type Info struct {
	Token        string
	DeviceID     string
	Phone        string
	MtInstanceID string
	UserAgent    *UserAgent
	Sync         SyncState
}

// ResolveUserAgent mirrors pymax's session.models.resolve_session_user_agent:
// it keeps the stored device identity but refreshes the app version/build
// number to the ones the current runtime is using, unless the device type
// changed (e.g. after switching between Client and WebClient) or nothing
// was stored yet.
func ResolveUserAgent(current UserAgent, stored *UserAgent) UserAgent {
	if stored == nil || stored.DeviceType != current.DeviceType {
		return current
	}
	updated := *stored
	updated.AppVersion = current.AppVersion
	updated.BuildNumber = current.BuildNumber
	return updated
}

// Store is the persistence contract for session data, a port of pymax's
// session.protocol.StoreProtocol.
type Store interface {
	// SaveSession saves or fully replaces the session data.
	SaveSession(ctx context.Context, info Info) error
	// UpdateToken replaces oldToken with newToken on the matching session.
	UpdateToken(ctx context.Context, oldToken, newToken string) error
	// LoadSession returns the session to use on startup, or nil if none exists.
	LoadSession(ctx context.Context) (*Info, error)
	// LoadSessionByDeviceID looks up a session by device ID.
	LoadSessionByDeviceID(ctx context.Context, deviceID string) (*Info, error)
	// LoadSessionByPhone looks up a session by phone number.
	LoadSessionByPhone(ctx context.Context, phone string) (*Info, error)
	// DeleteSession removes the session with the given token, if any.
	DeleteSession(ctx context.Context, token string) error
	// Close releases any resources held by the store.
	Close(ctx context.Context) error
}
