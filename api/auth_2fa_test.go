package api

import (
	"context"
	"testing"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// recordedInvoke captures every opcode/payload passed to Env.Invoke, in
// order, and canned per-opcode responses, letting tests assert both the
// call sequence and the payload shape without a real connection.
type recordedInvoke struct {
	calls     []recordedCall
	responses map[protocol.Opcode]map[string]any
}

type recordedCall struct {
	Opcode  protocol.Opcode
	Payload map[string]any
}

func (r *recordedInvoke) invoke(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
	r.calls = append(r.calls, recordedCall{Opcode: opcode, Payload: payload})
	if resp, ok := r.responses[opcode]; ok {
		return protocol.InboundFrame{Payload: resp}, nil
	}
	return protocol.InboundFrame{Payload: map[string]any{}}, nil
}

func newRecordedInvoke() *recordedInvoke {
	return &recordedInvoke{
		responses: map[protocol.Opcode]map[string]any{
			protocol.OpcodeAuthCreateTrack: {"trackId": "track-xyz"},
		},
	}
}

// TestSet2FAMinimal covers pymax's AuthService.set_2fa with no email/hint:
// it should create a track, validate the password, then call AUTH_SET_2FA
// with expectedCapabilities=[SET_PASSWORD] and no hint key at all (pymax's
// CamelModel.to_payload excludes None fields).
func TestSet2FAMinimal(t *testing.T) {
	ri := newRecordedInvoke()
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewAuthService(env)

	ok, err := svc.Set2FA(context.Background(), "pw123", nil, nil, nil)
	if err != nil {
		t.Fatalf("Set2FA: %v", err)
	}
	if !ok {
		t.Fatalf("expected Set2FA to report success")
	}

	wantOpcodes := []protocol.Opcode{
		protocol.OpcodeAuthCreateTrack,
		protocol.OpcodeAuthValidatePassword,
		protocol.OpcodeAuthSet2FA,
	}
	if len(ri.calls) != len(wantOpcodes) {
		t.Fatalf("expected %d invoke calls, got %d: %+v", len(wantOpcodes), len(ri.calls), ri.calls)
	}
	for i, op := range wantOpcodes {
		if ri.calls[i].Opcode != op {
			t.Fatalf("call %d: expected opcode %v, got %v", i, op, ri.calls[i].Opcode)
		}
	}

	final := ri.calls[2].Payload
	if final["trackId"] != "track-xyz" || final["password"] != "pw123" {
		t.Fatalf("unexpected AUTH_SET_2FA payload: %+v", final)
	}
	if _, hasHint := final["hint"]; hasHint {
		t.Fatalf("expected no hint key in payload, got %+v", final)
	}
	caps, ok := final["expectedCapabilities"].([]TwoFactorAction)
	if !ok || len(caps) != 1 || caps[0] != TwoFactorSetPassword {
		t.Fatalf("expected expectedCapabilities=[SetPassword], got %+v", final["expectedCapabilities"])
	}
}

// TestSet2FAWithHint covers pymax's set_2fa with a hint but no email: the
// hint must be validated (AUTH_VALIDATE_HINT) before AUTH_SET_2FA, and the
// final payload's expectedCapabilities must include HINT after SET_PASSWORD
// (matching pymax's append order: SET_PASSWORD, then HINT, then EMAIL).
func TestSet2FAWithHint(t *testing.T) {
	ri := newRecordedInvoke()
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewAuthService(env)

	hint := "my hint"
	if _, err := svc.Set2FA(context.Background(), "pw123", nil, &hint, nil); err != nil {
		t.Fatalf("Set2FA: %v", err)
	}

	wantOpcodes := []protocol.Opcode{
		protocol.OpcodeAuthCreateTrack,
		protocol.OpcodeAuthValidatePassword,
		protocol.OpcodeAuthValidateHint,
		protocol.OpcodeAuthSet2FA,
	}
	if len(ri.calls) != len(wantOpcodes) {
		t.Fatalf("expected %d invoke calls, got %d: %+v", len(wantOpcodes), len(ri.calls), ri.calls)
	}
	for i, op := range wantOpcodes {
		if ri.calls[i].Opcode != op {
			t.Fatalf("call %d: expected opcode %v, got %v", i, op, ri.calls[i].Opcode)
		}
	}

	final := ri.calls[len(ri.calls)-1].Payload
	if final["hint"] != hint {
		t.Fatalf("expected hint %q in final payload, got %+v", hint, final)
	}
	caps, ok := final["expectedCapabilities"].([]TwoFactorAction)
	if !ok || len(caps) != 2 || caps[0] != TwoFactorSetPassword || caps[1] != TwoFactorHint {
		t.Fatalf("expected expectedCapabilities=[SetPassword, Hint], got %+v", final["expectedCapabilities"])
	}
}

// TestRemove2FA covers pymax's AuthService.remove_2fa: create a track,
// verify the current password via AUTH_CHECK_PASSWORD (not
// AUTH_LOGIN_CHECK_PASSWORD, which is only used during login), then call
// AUTH_SET_2FA with remove2fa=true and expectedCapabilities=[REMOVE_2FA].
func TestRemove2FA(t *testing.T) {
	ri := newRecordedInvoke()
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewAuthService(env)

	ok, err := svc.Remove2FA(context.Background(), "current-pw")
	if err != nil {
		t.Fatalf("Remove2FA: %v", err)
	}
	if !ok {
		t.Fatalf("expected Remove2FA to report success")
	}

	wantOpcodes := []protocol.Opcode{
		protocol.OpcodeAuthCreateTrack,
		protocol.OpcodeAuthCheckPassword,
		protocol.OpcodeAuthSet2FA,
	}
	if len(ri.calls) != len(wantOpcodes) {
		t.Fatalf("expected %d invoke calls, got %d: %+v", len(wantOpcodes), len(ri.calls), ri.calls)
	}
	for i, op := range wantOpcodes {
		if ri.calls[i].Opcode != op {
			t.Fatalf("call %d: expected opcode %v, got %v", i, op, ri.calls[i].Opcode)
		}
	}

	checkPasswordCall := ri.calls[1].Payload
	if checkPasswordCall["password"] != "current-pw" {
		t.Fatalf("expected AUTH_CHECK_PASSWORD to carry the current password, got %+v", checkPasswordCall)
	}

	final := ri.calls[2].Payload
	if final["remove2fa"] != true {
		t.Fatalf("expected remove2fa=true, got %+v", final)
	}
	caps, ok := final["expectedCapabilities"].([]TwoFactorAction)
	if !ok || len(caps) != 1 || caps[0] != TwoFactorRemove2FA {
		t.Fatalf("expected expectedCapabilities=[Remove2FA], got %+v", final["expectedCapabilities"])
	}
}

// TestChangePassword covers pymax's AuthService.change_password: verify the
// old password (AUTH_CHECK_PASSWORD), validate the new one
// (AUTH_VALIDATE_PASSWORD), then AUTH_SET_2FA with
// expectedCapabilities=[UPDATE_PASSWORD].
func TestChangePassword(t *testing.T) {
	ri := newRecordedInvoke()
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewAuthService(env)

	ok, err := svc.ChangePassword(context.Background(), "old-pw", "new-pw")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !ok {
		t.Fatalf("expected ChangePassword to report success")
	}

	wantOpcodes := []protocol.Opcode{
		protocol.OpcodeAuthCreateTrack,
		protocol.OpcodeAuthCheckPassword,
		protocol.OpcodeAuthValidatePassword,
		protocol.OpcodeAuthSet2FA,
	}
	if len(ri.calls) != len(wantOpcodes) {
		t.Fatalf("expected %d invoke calls, got %d: %+v", len(wantOpcodes), len(ri.calls), ri.calls)
	}
	for i, op := range wantOpcodes {
		if ri.calls[i].Opcode != op {
			t.Fatalf("call %d: expected opcode %v, got %v", i, op, ri.calls[i].Opcode)
		}
	}

	if ri.calls[1].Payload["password"] != "old-pw" {
		t.Fatalf("expected AUTH_CHECK_PASSWORD to carry the old password, got %+v", ri.calls[1].Payload)
	}
	if ri.calls[2].Payload["password"] != "new-pw" {
		t.Fatalf("expected AUTH_VALIDATE_PASSWORD to carry the new password, got %+v", ri.calls[2].Payload)
	}
	final := ri.calls[3].Payload
	if final["password"] != "new-pw" {
		t.Fatalf("expected AUTH_SET_2FA to carry the new password, got %+v", final)
	}
	caps, ok := final["expectedCapabilities"].([]TwoFactorAction)
	if !ok || len(caps) != 1 || caps[0] != TwoFactorUpdatePassword {
		t.Fatalf("expected expectedCapabilities=[UpdatePassword], got %+v", final["expectedCapabilities"])
	}
}

// TestCheck2FA covers pymax's AuthService.check_2fa: it inspects the cached
// profile's profileOptions for SECOND_FACTOR_PASSWORD_ENABLED (2), without
// calling the server.
func TestCheck2FA(t *testing.T) {
	env := NewEnv()
	svc := NewAuthService(env)

	if svc.Check2FA() {
		t.Fatalf("expected Check2FA to be false before any profile is cached")
	}

	env.SetMe(&types.Profile{ProfileOptions: []int{1, 4}})
	if svc.Check2FA() {
		t.Fatalf("expected Check2FA to be false without SECOND_FACTOR_PASSWORD_ENABLED")
	}

	env.SetMe(&types.Profile{ProfileOptions: []int{1, int(ProfileOptionSecondFactorPasswordEnabled), 4}})
	if !svc.Check2FA() {
		t.Fatalf("expected Check2FA to be true with SECOND_FACTOR_PASSWORD_ENABLED")
	}
}
