package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SonChegg/PyMax/api"
	"github.com/SonChegg/PyMax/protocol"
)

// fixedRegistrationProvider returns each (firstName, lastName) pair in
// order, then repeats the last one, recording how many times it was asked
// and what lastErr it was shown each time.
type fixedRegistrationProvider struct {
	names    [][2]string
	calls    int
	lastErrs []error
}

func (p *fixedRegistrationProvider) GetRegistration(ctx context.Context, lastErr error) (api.RegistrationConfig, error) {
	p.lastErrs = append(p.lastErrs, lastErr)
	i := p.calls
	p.calls++
	pair := p.names[len(p.names)-1]
	if i < len(p.names) {
		pair = p.names[i]
	}
	return api.RegistrationConfig{FirstName: pair[0], LastName: pair[1]}, nil
}

// confirmRegistrationInvoke fakes the server side of
// AuthService.ConfirmRegistration: it rejects any name containing a digit
// (mirroring MAX's real "only letters" validation) without consuming the
// registration token, and otherwise succeeds with a login token.
func confirmRegistrationInvoke(token string) api.InvokeFunc {
	return func(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
		if opcode != protocol.OpcodeAuthConfirm {
			return protocol.InboundFrame{}, errors.New("unexpected opcode")
		}
		first, _ := payload["firstName"].(string)
		last, _ := payload["lastName"].(string)
		if strings.ContainsAny(first, "0123456789") || strings.ContainsAny(last, "0123456789") {
			return protocol.InboundFrame{}, errors.New("validate.last_name.invalid_chars")
		}
		return protocol.InboundFrame{Payload: map[string]any{"token": token}}, nil
	}
}

// TestAuthenticateWithRegistrationRetriesSameToken guards against a
// regression where a rejected registration name (e.g. MAX's "only letters
// allowed" validation) forced restarting the whole SMS flow from scratch —
// wasting the SMS code and risking the phone number being rate-limited for
// requesting too many codes. The retry loop must reuse the same
// registration token and only re-prompt for the name.
func TestAuthenticateWithRegistrationRetriesSameToken(t *testing.T) {
	env := api.NewEnv()
	env.Invoke = confirmRegistrationInvoke("tok-registered")
	authSvc := api.NewAuthService(env)

	provider := &fixedRegistrationProvider{names: [][2]string{
		{"Ivan4", "Petrov"}, // rejected: digit in first name
		{"Ivan", "Petrov2"}, // rejected: digit in last name
		{"Ivan", "Petrov"},  // accepted
	}}
	deps := Deps{Auth: authSvc}

	token, err := authenticateWithRegistration(context.Background(), deps, provider, "reg-track-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok-registered" {
		t.Fatalf("expected token %q, got %q", "tok-registered", token)
	}
	if provider.calls != 3 {
		t.Fatalf("expected exactly 3 registration attempts, got %d", provider.calls)
	}
	if provider.lastErrs[0] != nil {
		t.Fatalf("expected the first attempt to see no prior error, got %v", provider.lastErrs[0])
	}
	if provider.lastErrs[1] == nil || provider.lastErrs[2] == nil {
		t.Fatalf("expected retries to be told about the prior rejection, got %v", provider.lastErrs)
	}
}

// TestSmsFlowUsesRegistrationProviderOverStaticConfig covers the
// SmsFlow.Authenticate wiring: when RegistrationProvider is set it takes
// precedence over a static Deps.RegistrationConfig, matching PasswordProvider's
// existing precedence pattern.
func TestSmsFlowUsesRegistrationProviderOverStaticConfig(t *testing.T) {
	env := api.NewEnv()
	calls := 0
	env.Invoke = func(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
		switch opcode {
		case protocol.OpcodeAuthRequest:
			return protocol.InboundFrame{Payload: map[string]any{"token": "start-tok"}}, nil
		case protocol.OpcodeAuth:
			calls++
			return protocol.InboundFrame{Payload: map[string]any{
				"tokenAttrs": map[string]any{"REGISTER": map[string]any{"token": "reg-track"}},
			}}, nil
		case protocol.OpcodeAuthConfirm:
			return protocol.InboundFrame{Payload: map[string]any{"token": "final-tok"}}, nil
		}
		return protocol.InboundFrame{}, errors.New("unexpected opcode")
	}
	authSvc := api.NewAuthService(env)

	provider := &fixedRegistrationProvider{names: [][2]string{{"Ivan", "Petrov"}}}
	flow := NewSmsFlow(&fixedCodeProvider{code: "123456"}, nil)
	flow.RegistrationProvider = provider

	deps := Deps{
		Auth:  authSvc,
		Phone: "+79990000000",
		// A static config is also set; the interactive provider must win.
		RegistrationConfig: &api.RegistrationConfig{FirstName: "Should", LastName: "NotBeUsed"},
	}

	result, err := flow.Authenticate(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "final-tok" {
		t.Fatalf("expected token %q, got %q", "final-tok", result.Token)
	}
	if provider.calls != 1 {
		t.Fatalf("expected the interactive provider to be used exactly once, got %d calls", provider.calls)
	}
}

type fixedCodeProvider struct{ code string }

func (p *fixedCodeProvider) GetCode(ctx context.Context, phone string, lastErr error) (string, error) {
	return p.code, nil
}
