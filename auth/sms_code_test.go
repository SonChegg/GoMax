package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/SonChegg/PyMax/api"
	"github.com/SonChegg/PyMax/protocol"
)

// fixedCodeSequenceProvider returns each code in order, then repeats the
// last one, recording how many times it was asked and the lastErr shown
// each time.
type fixedCodeSequenceProvider struct {
	codes    []string
	calls    int
	lastErrs []error
}

func (p *fixedCodeSequenceProvider) GetCode(ctx context.Context, phone string, lastErr error) (string, error) {
	p.lastErrs = append(p.lastErrs, lastErr)
	i := p.calls
	p.calls++
	if i < len(p.codes) {
		return p.codes[i], nil
	}
	return p.codes[len(p.codes)-1], nil
}

// sendCodeInvoke fakes the server side of AuthService.SendCode: it rejects
// any code but correctCode with a business error (mirroring MAX's real
// verify.code.wrong, which does not invalidate the start token) and
// otherwise succeeds with a login token.
func sendCodeInvoke(correctCode, token string) api.InvokeFunc {
	return func(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
		if opcode != protocol.OpcodeAuth {
			return protocol.InboundFrame{}, errors.New("unexpected opcode")
		}
		if code, _ := payload["verifyCode"].(string); code == correctCode {
			return protocol.InboundFrame{Payload: map[string]any{
				"tokenAttrs": map[string]any{"LOGIN": map[string]any{"token": token}},
			}}, nil
		}
		return protocol.InboundFrame{}, errors.New("verify.code.wrong")
	}
}

// TestAuthenticateWithSmsCodeRetriesSameStartToken guards against a
// regression (matching a real live failure): a wrong SMS code used to kill
// the whole login attempt, forcing a brand-new RequestCode/SMS to retry —
// risking MAX rate-limiting the phone for requesting too many codes over a
// single mistyped digit. The retry loop must reuse the same start token.
func TestAuthenticateWithSmsCodeRetriesSameStartToken(t *testing.T) {
	env := api.NewEnv()
	env.Invoke = sendCodeInvoke("445566", "tok-logged-in")
	authSvc := api.NewAuthService(env)

	provider := &fixedCodeSequenceProvider{codes: []string{"111111", "222222", "445566"}}
	deps := Deps{Auth: authSvc, Phone: "+79990000000"}

	resp, err := authenticateWithSmsCode(context.Background(), deps, provider, "start-tok-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.LoginToken() != "tok-logged-in" {
		t.Fatalf("expected login token %q, got %q", "tok-logged-in", resp.LoginToken())
	}
	if provider.calls != 3 {
		t.Fatalf("expected exactly 3 code attempts, got %d", provider.calls)
	}
	if provider.lastErrs[0] != nil {
		t.Fatalf("expected the first attempt to see no prior error, got %v", provider.lastErrs[0])
	}
	if provider.lastErrs[1] == nil || provider.lastErrs[2] == nil {
		t.Fatalf("expected retries to be told about the prior rejection, got %v", provider.lastErrs)
	}
}
