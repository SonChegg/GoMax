package auth

import (
	"context"

	"github.com/SonChegg/PyMax/api"
)

// authenticateWithSmsCode runs the SMS-code entry retry loop: on a
// rejected code (e.g. MAX's verify.code.wrong), it re-prompts and retries
// SendCode against the same start token instead of forcing a brand-new
// RequestCode/SMS — pymax's own SmsAuthFlow has no such retry, which risks
// MAX rate-limiting the phone number for requesting too many codes just
// because of a mistyped digit.
func authenticateWithSmsCode(ctx context.Context, deps Deps, provider SmsCodeProvider, startToken string) (api.CheckCodeResponse, error) {
	var lastErr error
	for {
		code, err := provider.GetCode(ctx, deps.Phone, lastErr)
		if err != nil {
			return api.CheckCodeResponse{}, err
		}
		if code == "" {
			lastErr = nil
			continue
		}

		resp, err := deps.Auth.SendCode(ctx, startToken, code)
		if err != nil {
			lastErr = err
			continue
		}
		return resp, nil
	}
}
