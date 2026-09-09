package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/SonChegg/PyMax/api"
	"github.com/SonChegg/PyMax/protocol"
)

// fixedPasswordProvider returns each password in order, then repeats the
// last one for any further calls, recording how many times it was asked.
type fixedPasswordProvider struct {
	passwords []string
	calls     int
}

func (p *fixedPasswordProvider) GetPassword(ctx context.Context, hint string) (string, error) {
	i := p.calls
	p.calls++
	if i < len(p.passwords) {
		return p.passwords[i], nil
	}
	return p.passwords[len(p.passwords)-1], nil
}

// checkPasswordInvoke fakes the server side of AuthService.CheckPassword: it
// answers AUTH_LOGIN_CHECK_PASSWORD with a login token when the submitted
// password matches correctPassword, and an error field otherwise.
func checkPasswordInvoke(correctPassword, token string) api.InvokeFunc {
	return func(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
		if opcode != protocol.OpcodeAuthLoginCheckPassword {
			return protocol.InboundFrame{}, errors.New("unexpected opcode")
		}
		if pw, _ := payload["password"].(string); pw == correctPassword {
			return protocol.InboundFrame{Payload: map[string]any{
				"tokenAttrs": map[string]any{"LOGIN": map[string]any{"token": token}},
			}}, nil
		}
		return protocol.InboundFrame{Payload: map[string]any{"error": "bad password"}}, nil
	}
}

// TestAuthenticateWithPasswordRespectsMaxAttempts covers the SMS flow's 2FA
// retry loop, a port of pymax's auth.sms.SmsAuthFlow._authenticate_with_password:
// it bounds retries by app.config.password_max_attempts and raises
// PasswordAttemptsExceededError (ErrPasswordAttemptsExceeded here) once
// exhausted.
func TestAuthenticateWithPasswordRespectsMaxAttempts(t *testing.T) {
	env := api.NewEnv()
	env.Invoke = checkPasswordInvoke("correct-pw", "tok-ok")
	authSvc := api.NewAuthService(env)

	max := 3
	provider := &fixedPasswordProvider{passwords: []string{"wrong1", "wrong2", "wrong3", "wrong4"}}
	deps := Deps{Auth: authSvc, PasswordMaxAttempts: &max}

	_, err := authenticateWithPassword(context.Background(), deps, provider, "track-1", "hint")
	if !errors.Is(err, ErrPasswordAttemptsExceeded) {
		t.Fatalf("expected ErrPasswordAttemptsExceeded, got %v", err)
	}
	if provider.calls != max {
		t.Fatalf("expected exactly %d password attempts, got %d", max, provider.calls)
	}
}

// TestAuthenticateWithPasswordUnboundedIgnoresMaxAttempts covers the QR
// flow's 2FA retry loop, a port of pymax's
// auth.qr.QrAuthFlow._authenticate_with_password: unlike the SMS flow, it
// retries unconditionally (`while True`) with no attempt cap and no
// PasswordAttemptsExceededError. Even with deps.PasswordMaxAttempts set, the
// unbounded variant used by the QR flow must keep retrying past it.
func TestAuthenticateWithPasswordUnboundedIgnoresMaxAttempts(t *testing.T) {
	env := api.NewEnv()
	env.Invoke = checkPasswordInvoke("correct-pw", "tok-ok")
	authSvc := api.NewAuthService(env)

	max := 2 // deliberately smaller than the number of attempts needed below
	provider := &fixedPasswordProvider{passwords: []string{"wrong1", "wrong2", "wrong3", "wrong4", "correct-pw"}}
	deps := Deps{Auth: authSvc, PasswordMaxAttempts: &max}

	token, err := authenticateWithPasswordUnbounded(context.Background(), deps, provider, "track-1", "hint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok-ok" {
		t.Fatalf("expected token %q, got %q", "tok-ok", token)
	}
	if provider.calls != len(provider.passwords) {
		t.Fatalf("expected exactly %d password attempts (past max=%d), got %d", len(provider.passwords), max, provider.calls)
	}
}
