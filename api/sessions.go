package api

import (
	"context"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/session"
)

// DeviceSession is one of the account's active device sessions, a port of
// pymax's types.domain.session.Session.
type DeviceSession struct {
	ID           any     `json:"id,omitempty"`
	DeviceID     *string `json:"deviceId,omitempty"`
	Current      *bool   `json:"current,omitempty"`
	UserAgent    *string `json:"userAgent,omitempty"`
	AppVersion   *string `json:"appVersion,omitempty"`
	DeviceName   *string `json:"deviceName,omitempty"`
	DeviceType   *string `json:"deviceType,omitempty"`
	Platform     *string `json:"platform,omitempty"`
	IP           *string `json:"ip,omitempty"`
	Location     *string `json:"location,omitempty"`
	Created      *int64  `json:"created,omitempty"`
	Updated      *int64  `json:"updated,omitempty"`
	LastActivity *int64  `json:"lastActivity,omitempty"`
	Options      any     `json:"options,omitempty"`
}

// SessionsService implements handshake and device-session management, a
// port of pymax's api.session.service.SessionService plus the
// sessions-related bits of api.self.service.SelfService and
// api.users.service.UserService.
type SessionsService struct{ env *Env }

// NewSessionsService builds a sessions service bound to env.
func NewSessionsService(env *Env) *SessionsService { return &SessionsService{env: env} }

// Handshake performs the initial mobile or web handshake, a port of
// pymax's SessionService.handshake.
func (s *SessionsService) Handshake(ctx context.Context, isWeb bool, ua session.UserAgent, deviceID string) (HandshakeResponse, error) {
	var payload map[string]any
	if isWeb {
		payload = map[string]any{
			"userAgent": userAgentPayload(ua),
			"deviceId":  deviceID,
		}
	} else {
		payload = map[string]any{
			"mt_instanceid":   s.env.MtInstanceID,
			"userAgent":       userAgentPayload(ua),
			"deviceId":        deviceID,
			"clientSessionId": 1,
		}
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeSessionInit, payload)
	if err != nil {
		return HandshakeResponse{}, err
	}
	resp, err := requireModel[HandshakeResponse](frame)
	if err != nil {
		return HandshakeResponse{}, err
	}
	s.env.SetHandshakeResponse(&resp)
	return resp, nil
}

// GetSessions lists the account's active device sessions, a port of
// pymax's UserService.get_sessions.
func (s *SessionsService) GetSessions(ctx context.Context) ([]DeviceSession, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeSessionsInfo, map[string]any{})
	if err != nil {
		return nil, err
	}
	return decodeList[DeviceSession](frame, "sessions")
}

// CloseAllOtherSessions revokes every other device session, a port of
// pymax's SelfService.close_all_sessions.
func (s *SessionsService) CloseAllOtherSessions(ctx context.Context) (bool, error) {
	sess := s.env.Session()
	if sess == nil {
		return false, nil
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeSessionsClose, map[string]any{})
	if err != nil {
		return false, err
	}

	token, _ := payloadItem(frame, "token").(string)
	if token == "" {
		return false, nil
	}

	if s.env.Store != nil {
		if err := s.env.Store.UpdateToken(ctx, sess.Token, token); err != nil {
			return false, err
		}
	}
	updated := *sess
	updated.Token = token
	s.env.SetSession(&updated)
	return true, nil
}

// Logout ends the current session server-side, a port of pymax's
// SelfService.logout.
func (s *SessionsService) Logout(ctx context.Context) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeLogout, map[string]any{})
	return err
}
