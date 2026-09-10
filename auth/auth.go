// Package auth implements the pluggable login flows (SMS code, QR, 2FA
// password) and the mobile fingerprint/version catalog used to authenticate
// with the Max protocol. It is a port of pymax's auth and fingerprint
// packages.
package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/SonChegg/PyMax/api"
)

// AuthResult is the outcome of a successful AuthFlow, a port of pymax's
// auth.models.AuthResult.
type AuthResult struct {
	Token string
}

// Deps are the dependencies an AuthFlow needs from the running client, a
// port of the parts of pymax's App an AuthFlow reaches into (app.api.auth,
// app.config.phone, app.config.registration_config, ...).
type Deps struct {
	Auth                *api.AuthService
	Phone               string
	RegistrationConfig  *api.RegistrationConfig
	PasswordMaxAttempts *int
}

// Flow is a full custom login scenario, a port of pymax's auth.base.AuthFlow.
// Most integrations only need to swap SmsCodeProvider, PasswordProvider or
// QrHandler instead of implementing Flow directly.
type Flow interface {
	Authenticate(ctx context.Context, deps Deps) (AuthResult, error)
}

// ErrPasswordAttemptsExceeded is returned once the 2FA password retry limit
// is reached, a port of pymax's auth.exceptions.PasswordAttemptsExceededError.
var ErrPasswordAttemptsExceeded = errors.New("gomax: 2fa password attempts exhausted")

// SmsCodeProvider supplies the SMS verification code. Called again (with
// lastErr set) if MAX rejects the code, so an implementation can let the
// caller retry against the same SMS instead of forcing a brand-new one —
// pymax's own SmsAuthFlow has no such retry (a wrong code kills the whole
// login attempt), which is a real usability gap given MAX can rate-limit a
// phone number for requesting too many codes.
type SmsCodeProvider interface {
	GetCode(ctx context.Context, phone string, lastErr error) (string, error)
}

// ConsoleSmsCodeProvider reads the SMS code from stdin, a port of pymax's
// auth.providers.ConsoleSmsCodeProvider.
type ConsoleSmsCodeProvider struct{}

func (ConsoleSmsCodeProvider) GetCode(ctx context.Context, phone string, lastErr error) (string, error) {
	if lastErr != nil {
		fmt.Printf("Code rejected: %v\n", lastErr)
	}
	fmt.Printf("Enter SMS code for %s: ", phone)
	return readLine()
}

// PasswordProvider supplies the account's 2FA password, a port of pymax's
// auth.providers.PasswordProvider.
type PasswordProvider interface {
	GetPassword(ctx context.Context, hint string) (string, error)
}

// ConsolePasswordProvider reads the 2FA password from stdin without
// echoing it, a port of pymax's auth.providers.ConsolePasswordProvider.
type ConsolePasswordProvider struct{}

func (ConsolePasswordProvider) GetPassword(ctx context.Context, hint string) (string, error) {
	prompt := "Enter 2FA password"
	if hint != "" {
		prompt += fmt.Sprintf(" (hint: %s)", hint)
	}
	prompt += ": "
	fmt.Print(prompt)

	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return readLine()
}

// QrHandler shows the QR login link to the user, a port of pymax's
// auth.providers.QrHandler.
type QrHandler interface {
	ShowQR(ctx context.Context, qrURL string) error
}

// ConsoleQrHandler prints the QR login link to stdout.
//
// NOTE: pymax renders a scannable ASCII QR code via the Python `qrcode`
// library. gomax does not vendor a QR-rendering dependency for this; it
// prints the raw link, which the user can open directly or feed to any QR
// code generator/scanner.
type ConsoleQrHandler struct{}

func (ConsoleQrHandler) ShowQR(ctx context.Context, qrURL string) error {
	fmt.Printf("Open this link (or scan it as a QR code) to log in: %s\n", qrURL)
	return nil
}

// EmailCodeProvider supplies the email verification code used when setting
// up 2FA, a port of pymax's auth.providers.EmailCodeProvider.
type EmailCodeProvider interface {
	GetCode(ctx context.Context, email string) (string, error)
}

// RegistrationProvider supplies the first/last name needed to finish
// registering a brand-new account. It is called once per attempt (not just
// once total): MAX validates the name server-side (e.g. rejecting digits
// or punctuation) and returns a business error without invalidating the
// registration token, so a rejected attempt can be retried with corrected
// input against the same token — no fresh SMS code required. lastErr is
// nil on the first call and holds the previous attempt's rejection
// otherwise.
type RegistrationProvider interface {
	GetRegistration(ctx context.Context, lastErr error) (api.RegistrationConfig, error)
}

// ConsoleRegistrationProvider reads first/last name from stdin, a port of
// pymax's interactive registration prompt.
type ConsoleRegistrationProvider struct{}

func (ConsoleRegistrationProvider) GetRegistration(ctx context.Context, lastErr error) (api.RegistrationConfig, error) {
	if lastErr != nil {
		fmt.Printf("Registration rejected: %v\n", lastErr)
	}
	fmt.Print("Enter first name: ")
	firstName, err := readLine()
	if err != nil {
		return api.RegistrationConfig{}, err
	}
	fmt.Print("Enter last name (optional): ")
	lastName, err := readLine()
	if err != nil {
		return api.RegistrationConfig{}, err
	}
	return api.RegistrationConfig{FirstName: firstName, LastName: lastName}, nil
}

// ConsoleEmailCodeProvider reads the email code from stdin, a port of
// pymax's auth.providers.ConsoleEmailCodeProvider.
type ConsoleEmailCodeProvider struct{}

func (ConsoleEmailCodeProvider) GetCode(ctx context.Context, email string) (string, error) {
	fmt.Printf("Enter 2FA email code for %s: ", email)
	return readLine()
}

func readLine() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
