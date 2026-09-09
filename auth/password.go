package auth

import (
	"context"
	"fmt"
)

// authenticateWithPassword runs the 2FA password retry loop used by the SMS
// flow, a port of pymax's auth.sms.SmsAuthFlow._authenticate_with_password.
// Unlike the QR flow, pymax's SMS flow bounds the retry loop by
// app.config.password_max_attempts (raising PasswordAttemptsExceededError
// once exhausted), which is why this variant honors deps.PasswordMaxAttempts
// while authenticateWithPasswordUnbounded (used by the QR flow) does not.
func authenticateWithPassword(ctx context.Context, deps Deps, provider PasswordProvider, trackID, hint string) (string, error) {
	attempt := 0
	for deps.PasswordMaxAttempts == nil || *deps.PasswordMaxAttempts > attempt {
		password, err := provider.GetPassword(ctx, hint)
		if err != nil {
			return "", err
		}
		if password == "" {
			attempt++
			continue
		}

		resp, err := deps.Auth.CheckPassword(ctx, trackID, password)
		if err != nil {
			attempt++
			continue
		}
		if resp.Error != nil {
			attempt++
			continue
		}
		if resp.LoginToken() != "" {
			return resp.LoginToken(), nil
		}
		attempt++
	}
	return "", ErrPasswordAttemptsExceeded
}

// authenticateWithPasswordUnbounded runs the 2FA password retry loop used by
// the QR flow, a port of pymax's auth.qr.QrAuthFlow._authenticate_with_password.
// Unlike the SMS flow, pymax's QR flow retries with an unconditional
// `while True` and has no attempt cap and no PasswordAttemptsExceededError;
// it only stops once the password provider returns a login token (or an
// error/ctx cancellation propagates out of the provider or API call).
func authenticateWithPasswordUnbounded(ctx context.Context, deps Deps, provider PasswordProvider, trackID, hint string) (string, error) {
	for {
		password, err := provider.GetPassword(ctx, hint)
		if err != nil {
			return "", err
		}
		if password == "" {
			continue
		}

		resp, err := deps.Auth.CheckPassword(ctx, trackID, password)
		if err != nil {
			continue
		}
		if resp.Error != nil {
			continue
		}
		if resp.LoginToken() != "" {
			return resp.LoginToken(), nil
		}
	}
}

// PasswordFlow re-authenticates an already-known account using only its
// 2FA password against an existing track/challenge (e.g. after
// AuthService.CheckPassword-style validation flows such as
// AUTH_VALIDATE_PASSWORD / AUTH_CHECK_PASSWORD). It is a thin, directly
// invokable wrapper around the same retry logic SmsFlow/QrFlow use
// internally for their password challenges, exposed standalone for
// integrations that already have a track ID (e.g. from a custom Flow).
type PasswordFlow struct {
	PasswordProvider PasswordProvider
}

// NewPasswordFlow builds a password-challenge flow. If passwordProvider is
// nil, ConsolePasswordProvider is used.
func NewPasswordFlow(passwordProvider PasswordProvider) *PasswordFlow {
	if passwordProvider == nil {
		passwordProvider = ConsolePasswordProvider{}
	}
	return &PasswordFlow{PasswordProvider: passwordProvider}
}

// Resolve runs the password retry loop against an existing 2FA challenge
// track ID (as returned in a PasswordChallenge by SendCode/ConfirmQR) and
// returns the resulting login token.
func (f *PasswordFlow) Resolve(ctx context.Context, deps Deps, trackID, hint string) (string, error) {
	if deps.Auth == nil {
		return "", fmt.Errorf("gomax: auth service is required to resolve a password challenge")
	}
	return authenticateWithPassword(ctx, deps, f.PasswordProvider, trackID, hint)
}
