package auth

import (
	"context"
	"fmt"
)

// SmsFlow is the standard SMS-code login flow used by the TCP Client, a
// port of pymax's auth.sms.SmsAuthFlow.
type SmsFlow struct {
	CodeProvider     SmsCodeProvider
	PasswordProvider PasswordProvider
	// RegistrationProvider, if set, handles a brand-new-account signup
	// interactively (retrying ConfirmRegistration with the same token on a
	// rejected name, no fresh SMS needed) instead of requiring a
	// pre-supplied, non-retryable Deps.RegistrationConfig.
	RegistrationProvider RegistrationProvider
}

// NewSmsFlow builds an SMS auth flow. If passwordProvider is nil,
// ConsolePasswordProvider is used for any 2FA challenge. Set the returned
// flow's RegistrationProvider field directly for interactive, retryable
// new-account registration.
func NewSmsFlow(codeProvider SmsCodeProvider, passwordProvider PasswordProvider) *SmsFlow {
	if passwordProvider == nil {
		passwordProvider = ConsolePasswordProvider{}
	}
	return &SmsFlow{CodeProvider: codeProvider, PasswordProvider: passwordProvider}
}

// Authenticate runs the SMS-code (plus optional 2FA/registration) login
// sequence, a port of pymax's SmsAuthFlow.authenticate.
func (f *SmsFlow) Authenticate(ctx context.Context, deps Deps) (AuthResult, error) {
	if deps.Phone == "" {
		return AuthResult{}, fmt.Errorf("gomax: phone is required for sms authentication")
	}

	start, err := deps.Auth.RequestCode(ctx, deps.Phone)
	if err != nil {
		return AuthResult{}, err
	}

	result, err := authenticateWithSmsCode(ctx, deps, f.CodeProvider, start.Token)
	if err != nil {
		return AuthResult{}, err
	}

	var token string
	switch {
	case result.LoginToken() != "":
		token = result.LoginToken()
	case result.PasswordChallenge != nil:
		token, err = authenticateWithPassword(ctx, deps, f.PasswordProvider, result.PasswordChallenge.TrackID, hintOf(result.PasswordChallenge.Hint))
		if err != nil {
			return AuthResult{}, err
		}
	case result.RegisterToken() != "" && f.RegistrationProvider != nil:
		token, err = authenticateWithRegistration(ctx, deps, f.RegistrationProvider, result.RegisterToken())
		if err != nil {
			return AuthResult{}, err
		}
	case result.RegisterToken() != "":
		if deps.RegistrationConfig == nil {
			return AuthResult{}, fmt.Errorf("gomax: RegistrationConfig is required to register a new account")
		}
		resp, err := deps.Auth.ConfirmRegistration(ctx, deps.RegistrationConfig.FirstName, deps.RegistrationConfig.LastName, result.RegisterToken())
		if err != nil {
			return AuthResult{}, err
		}
		token = resp.Token
	default:
		return AuthResult{}, fmt.Errorf("gomax: authentication failed: server returned no login token, password challenge, or registration token")
	}

	return AuthResult{Token: token}, nil
}

func hintOf(hint *string) string {
	if hint == nil {
		return ""
	}
	return *hint
}
