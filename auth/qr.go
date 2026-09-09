package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/SonChegg/PyMax/api"
)

// QrFlow is the standard QR-code login flow used by the WebClient, a port
// of pymax's auth.qr.QrAuthFlow.
type QrFlow struct {
	QrProvider       QrHandler
	PasswordProvider PasswordProvider
}

// NewQrFlow builds a QR auth flow. If passwordProvider is nil,
// ConsolePasswordProvider is used for any 2FA challenge.
func NewQrFlow(qrProvider QrHandler, passwordProvider PasswordProvider) *QrFlow {
	if qrProvider == nil {
		qrProvider = ConsoleQrHandler{}
	}
	if passwordProvider == nil {
		passwordProvider = ConsolePasswordProvider{}
	}
	return &QrFlow{QrProvider: qrProvider, PasswordProvider: passwordProvider}
}

// Authenticate runs the QR-code (plus optional 2FA) login sequence, a port
// of pymax's QrAuthFlow.authenticate.
func (f *QrFlow) Authenticate(ctx context.Context, deps Deps) (AuthResult, error) {
	qrInfo, err := deps.Auth.RequestQR(ctx)
	if err != nil {
		return AuthResult{}, err
	}

	if err := f.QrProvider.ShowQR(ctx, qrInfo.QrLink); err != nil {
		return AuthResult{}, err
	}

	confirmed, err := f.pollQR(ctx, deps.Auth, qrInfo)
	if err != nil {
		return AuthResult{}, err
	}
	if !confirmed {
		return AuthResult{}, fmt.Errorf("gomax: qr authentication expired")
	}

	result, err := deps.Auth.ConfirmQR(ctx, qrInfo.TrackID)
	if err != nil {
		return AuthResult{}, err
	}

	token := result.LoginToken()
	if token == "" && result.PasswordChallenge != nil {
		token, err = authenticateWithPasswordUnbounded(ctx, deps, f.PasswordProvider, result.PasswordChallenge.TrackID, hintOf(result.PasswordChallenge.Hint))
		if err != nil {
			return AuthResult{}, err
		}
	}

	return AuthResult{Token: token}, nil
}

func (f *QrFlow) pollQR(ctx context.Context, authSvc *api.AuthService, qrInfo api.RequestQrResponse) (bool, error) {
	interval := time.Duration(qrInfo.PollingInterval) * time.Millisecond
	expiresAt := time.UnixMilli(qrInfo.ExpiresAt)

	for time.Now().Before(expiresAt) {
		resp, err := authSvc.CheckQR(ctx, qrInfo.TrackID)
		if err != nil {
			return false, err
		}
		if resp.Status.LoginAvailable != nil && *resp.Status.LoginAvailable {
			return true, nil
		}

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	return false, nil
}
