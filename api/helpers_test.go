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
