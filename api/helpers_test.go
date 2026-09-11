package api

import (
	"encoding/json"
	"testing"
)

// TestLenientInt64AcceptsStringOrNumber guards against a regression hit
// live: MAX's ConfirmRegistrationResponse.userToken arrives as a JSON
// string for some accounts and a JSON number for others (pymax's `int`
// pydantic field accepts either automatically; Go's encoding/json does not
// without a custom UnmarshalJSON), which made a *successful* registration
// fail to parse and look like an error.
func TestLenientInt64AcceptsStringOrNumber(t *testing.T) {
	cases := []struct {
		name string
		json string
		want int64
	}{
		{"number", `123456789012`, 123456789012},
		{"string", `"123456789012"`, 123456789012},
		{"empty string", `""`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var n LenientInt64
			if err := json.Unmarshal([]byte(c.json), &n); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if int64(n) != c.want {
				t.Fatalf("got %d, want %d", int64(n), c.want)
			}
		})
	}
}

func TestLenientInt64RejectsNonNumericString(t *testing.T) {
	var n LenientInt64
	if err := json.Unmarshal([]byte(`"not-a-number"`), &n); err == nil {
		t.Fatal("expected an error for a non-numeric string")
	}
}

// TestConfirmRegistrationResponseAcceptsStringUserToken is the concrete
// regression case: decoding a real-shaped ConfirmRegistrationResponse
// payload where userToken is a string must not fail.
func TestConfirmRegistrationResponseAcceptsStringUserToken(t *testing.T) {
	raw := []byte(`{"userToken":"987654321098","tokenType":"REGISTER","token":"tok-abc"}`)
	var resp ConfirmRegistrationResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int64(resp.UserToken) != 987654321098 {
		t.Fatalf("got %d, want 987654321098", int64(resp.UserToken))
	}
	if resp.Token != "tok-abc" {
		t.Fatalf("got token %q, want %q", resp.Token, "tok-abc")
	}
}

// TestToPayloadPreservesByteSlices guards against a regression where
// toPayload's json.Marshal/Unmarshal round trip silently turned a []byte
// field into a base64 *string* (JSON has no binary type) before it ever
// reached gomax's msgpack encoder — which does emit []byte as a proper
// msgpack bin value, but only for a genuine []byte, not a string that
// happens to contain base64 text. Max's server rejected the resulting voice
// message attach payload outright ("Invalid attachment", [proto.payload]).
func TestToPayloadPreservesByteSlices(t *testing.T) {
	p := VideoAttachPayload{
		Type:     "AUDIO",
		AudioID:  42,
		Duration: 4200,
		Wave:     []byte{1, 2, 3, 4, 5},
	}
	m := toPayload(p)

	wave, ok := m["wave"].([]byte)
	if !ok {
		t.Fatalf("m[\"wave\"] is %T, want []byte", m["wave"])
	}
	if string(wave) != string([]byte{1, 2, 3, 4, 5}) {
		t.Fatalf("wave = %v, want [1 2 3 4 5]", wave)
	}

	// Zero-value fields tagged omitempty must still be dropped, and
	// ordinary scalar fields must still come through as themselves.
	if _, present := m["token"]; present {
		t.Fatalf("m[\"token\"] should be omitted (zero value, omitempty), got %v", m["token"])
	}
	if m["_type"] != "AUDIO" {
		t.Fatalf("m[\"_type\"] = %v, want AUDIO", m["_type"])
	}
	if m["audioId"] != int64(42) {
		t.Fatalf("m[\"audioId\"] = %v, want 42", m["audioId"])
	}
}
