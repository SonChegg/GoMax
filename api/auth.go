package api

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/session"
	"github.com/SonChegg/PyMax/types"
)

// AuthType identifies the kind of auth-token request/response, a port of
// pymax's api.auth.enums.AuthType.
type AuthType string

const (
	AuthTypeStartAuth AuthType = "START_AUTH"
	AuthTypeCheckCode AuthType = "CHECK_CODE"
	AuthTypeRegister  AuthType = "REGISTER"
	AuthTypeResend    AuthType = "RESEND"
	AuthTypeLogin     AuthType = "LOGIN"
)

// TwoFactorAction identifies a 2FA management action, a port of pymax's
// api.auth.enums.TwoFactorAction.
type TwoFactorAction int

const (
	TwoFactorSetPassword     TwoFactorAction = 0
	TwoFactorUpdatePassword  TwoFactorAction = 1
	TwoFactorRestorePassword TwoFactorAction = 2
	TwoFactorHint            TwoFactorAction = 3
	TwoFactorEmail           TwoFactorAction = 4
	TwoFactorRemove2FA       TwoFactorAction = 5
)

// ProfileOption identifies a numeric profile flag related to 2FA, a port of
// pymax's api.auth.enums.ProfileOptions.
type ProfileOption int

const (
	ProfileOptionEsiaVerified                ProfileOption = 1
	ProfileOptionSecondFactorPasswordEnabled ProfileOption = 2
	ProfileOptionSecondFactorHasEmail        ProfileOption = 3
	ProfileOptionSecondFactorHasHint         ProfileOption = 4
)

// EmailCodeProvider supplies the email verification code used when setting
// up 2FA email verification via AuthService.Set2FA, a port of pymax's
// auth.providers.EmailCodeProvider.
//
// NOTE: this interface is intentionally re-declared here (rather than
// imported from package auth) because package auth already imports package
// api for its Deps.Auth field; importing it back here would create an
// import cycle. Any type satisfying this method set (including
// auth.ConsoleEmailCodeProvider) can be passed as an EmailCodeProvider.
type EmailCodeProvider interface {
	GetCode(ctx context.Context, email string) (string, error)
}

// ConsoleEmailCodeProvider reads the 2FA email code from stdin, a port of
// pymax's auth.providers.ConsoleEmailCodeProvider. It is the default used by
// Set2FA when an email is supplied but no EmailCodeProvider is given.
type ConsoleEmailCodeProvider struct{}

func (ConsoleEmailCodeProvider) GetCode(ctx context.Context, email string) (string, error) {
	fmt.Printf("Enter 2FA email code for %s: ", email)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// StartAuthResponse answers a code request, a port of pymax's
// types.domain.auth.StartAuthResponse.
type StartAuthResponse struct {
	Token              string `json:"token"`
	CodeLength         int    `json:"codeLength"`
	RequestMaxDuration int    `json:"requestMaxDuration"`
	RequestCountLeft   int    `json:"requestCountLeft"`
	AltActionDuration  int    `json:"altActionDuration"`
}

// Token is a bare login/registration token, a port of pymax's
// types.domain.auth.Token.
type Token struct {
	Token string `json:"token"`
}

// TokenAttrs holds the tokens returned after a code/password check, a port
// of pymax's types.domain.auth.TokenAttrs.
type TokenAttrs struct {
	Login    *Token `json:"LOGIN,omitempty"`
	Register *Token `json:"REGISTER,omitempty"`
}

// PasswordChallenge asks for the account's 2FA password, a port of pymax's
// types.domain.auth.PasswordChallenge.
type PasswordChallenge struct {
	TrackID string  `json:"trackId"`
	Hint    *string `json:"hint,omitempty"`
}

// CheckCodeResponse answers a verification-code check, a port of pymax's
// types.domain.auth.CheckCodeResponse.
type CheckCodeResponse struct {
	TokenAttrs        TokenAttrs         `json:"tokenAttrs"`
	PasswordChallenge *PasswordChallenge `json:"passwordChallenge,omitempty"`
}

// LoginToken returns the login token, if the server issued one.
func (r CheckCodeResponse) LoginToken() string {
	if r.TokenAttrs.Login == nil {
		return ""
	}
	return r.TokenAttrs.Login.Token
}

// RegisterToken returns the registration token, if the server issued one.
func (r CheckCodeResponse) RegisterToken() string {
	if r.TokenAttrs.Register == nil {
		return ""
	}
	return r.TokenAttrs.Register.Token
}

// CheckPasswordResponse answers a 2FA password check, a port of pymax's
// types.domain.auth.CheckPasswordResponse.
type CheckPasswordResponse struct {
	TokenAttrs TokenAttrs `json:"tokenAttrs"`
	Error      *string    `json:"error,omitempty"`
}

// LoginToken returns the login token, if the server issued one.
func (r CheckPasswordResponse) LoginToken() string {
	if r.TokenAttrs.Login == nil {
		return ""
	}
	return r.TokenAttrs.Login.Token
}

// RequestQrResponse answers a QR-login request, a port of pymax's
// types.domain.auth.RequestQrResponse.
type RequestQrResponse struct {
	ExpiresAt       int64  `json:"expiresAt"`
	PollingInterval int64  `json:"pollingInterval"`
	QrLink          string `json:"qrLink"`
	TrackID         string `json:"trackId"`
	TTL             int64  `json:"ttl"`
}

// QrStatus is the current status of a QR login, a port of pymax's
// types.domain.auth.QrStatus.
type QrStatus struct {
	ExpiresAt      int64 `json:"expiresAt"`
	LoginAvailable *bool `json:"loginAvailable,omitempty"`
}

// CheckQrResponse wraps QrStatus, a port of pymax's
// types.domain.auth.CheckQrResponse.
type CheckQrResponse struct {
	Status QrStatus `json:"status"`
}

// ConfirmRegistrationResponse is returned after finishing registration, a
// port of pymax's types.domain.auth.ConfirmRegistrationResponse.
type ConfirmRegistrationResponse struct {
	UserToken int64         `json:"userToken"`
	Profile   types.Profile `json:"profile"`
	TokenType AuthType      `json:"tokenType"`
	Token     string        `json:"token"`
}

// LoginConfig carries the server config hash returned on login, a port of
// pymax's types.domain.login.LoginConfig.
type LoginConfig struct {
	Hash *string `json:"hash,omitempty"`
}

// Login2Flags indicates which parts of login2 are required, a port of
// pymax's types.domain.login.Login2Flags.
type Login2Flags struct {
	ConfigEnabled  bool `json:"configEnabled"`
	ContactEnabled bool `json:"contactEnabled"`
	ProfileEnabled bool `json:"profileEnabled"`
}

// Enabled reports whether a login2 round-trip is required at all.
func (f Login2Flags) Enabled() bool { return f.ConfigEnabled || f.ContactEnabled || f.ProfileEnabled }

// LoginResponse is returned by AuthService.Login, a port of pymax's
// types.domain.login.LoginResponse.
type LoginResponse struct {
	Chats       []*types.Chat               `json:"chats,omitempty"`
	Profile     *types.Profile              `json:"profile,omitempty"`
	Messages    map[string][]*types.Message `json:"messages,omitempty"`
	Contacts    []*types.User               `json:"contacts,omitempty"`
	Token       *string                     `json:"token,omitempty"`
	Time        *int64                      `json:"time,omitempty"`
	Config      *LoginConfig                `json:"config,omitempty"`
	Updates     *int                        `json:"updates,omitempty"`
	Login2Flags *Login2Flags                `json:"login2Flags,omitempty"`
}

// UpdateSyncState folds this response's sync-relevant fields into current, a
// port of pymax's LoginResponse.update_sync_state.
func (r LoginResponse) UpdateSyncState(current session.SyncState) session.SyncState {
	next := current
	if r.Time != nil {
		next.ChatsSync = *r.Time
		next.ContactsSync = *r.Time
		next.DraftsSync = *r.Time
		next.PresenceSync = *r.Time
	}
	if r.Config != nil && r.Config.Hash != nil {
		next.ConfigHash = *r.Config.Hash
	}
	return next
}

// Login2Response is returned by AuthService.MobileLogin2, a port of
// pymax's types.domain.login.Login2Response.
type Login2Response struct {
	Profile  *types.Profile `json:"profile,omitempty"`
	Contacts []*types.User  `json:"contactInfos,omitempty"`
	Config   *LoginConfig   `json:"config,omitempty"`
}

// UpdateSyncState folds this response's sync-relevant fields into current, a
// port of pymax's Login2Response.update_sync_state.
func (r Login2Response) UpdateSyncState(current session.SyncState) session.SyncState {
	next := current
	if r.Config != nil && r.Config.Hash != nil {
		next.ConfigHash = *r.Config.Hash
	}
	return next
}

// AuthService implements the auth/login opcodes, a port of pymax's
// api.auth.service.AuthService.
type AuthService struct{ env *Env }

// NewAuthService builds an auth service bound to env.
func NewAuthService(env *Env) *AuthService { return &AuthService{env: env} }

// RequestCode starts SMS authentication for phone, a port of pymax's
// AuthService.request_code.
func (s *AuthService) RequestCode(ctx context.Context, phone string) (StartAuthResponse, error) {
	mode, err := s.mobileMode(ctx)
	if err != nil {
		return StartAuthResponse{}, err
	}

	payload := map[string]any{"phone": phone, "type": string(AuthTypeStartAuth)}
	if mode != nil {
		payload["mode"] = mode
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeAuthRequest, payload)
	if err != nil {
		return StartAuthResponse{}, err
	}
	return requireModel[StartAuthResponse](frame)
}

// SendCode submits the SMS verification code, a port of pymax's
// AuthService.send_code.
func (s *AuthService) SendCode(ctx context.Context, token, code string) (CheckCodeResponse, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeAuth, map[string]any{
		"token":         token,
		"verifyCode":    code,
		"authTokenType": string(AuthTypeCheckCode),
	})
	if err != nil {
		return CheckCodeResponse{}, err
	}
	return requireModel[CheckCodeResponse](frame)
}

// CheckPassword submits the 2FA password for a login challenge, a port of
// pymax's AuthService.check_password.
func (s *AuthService) CheckPassword(ctx context.Context, trackID, password string) (CheckPasswordResponse, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeAuthLoginCheckPassword, map[string]any{
		"trackId":  trackID,
		"password": password,
	})
	if err != nil {
		return CheckPasswordResponse{}, err
	}
	return requireModel[CheckPasswordResponse](frame)
}

// ConfirmRegistration finishes registering a brand-new account after SMS
// verification, a port of pymax's AuthService.confirm_registration.
func (s *AuthService) ConfirmRegistration(ctx context.Context, firstName, lastName, token string) (ConfirmRegistrationResponse, error) {
	payload := map[string]any{
		"firstName": firstName,
		"token":     token,
		"tokenType": string(AuthTypeRegister),
	}
	if lastName != "" {
		payload["lastName"] = lastName
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeAuthConfirm, payload)
	if err != nil {
		return ConfirmRegistrationResponse{}, err
	}
	return requireModel[ConfirmRegistrationResponse](frame)
}

// Login logs in with the session token, dispatching to MobileLogin or
// WebLogin depending on the configured device type, a port of pymax's
// AuthService.login.
func (s *AuthService) Login(ctx context.Context, isWeb bool) (LoginResponse, error) {
	if isWeb {
		return s.WebLogin(ctx)
	}
	return s.MobileLogin(ctx)
}

// MobileLogin performs the mobile-client login handshake, a port of
// pymax's AuthService.mobile_login.
func (s *AuthService) MobileLogin(ctx context.Context) (LoginResponse, error) {
	sess := s.env.Session()
	if sess == nil {
		return LoginResponse{}, fmt.Errorf("api: no session available for login")
	}
	hr := s.env.HandshakeResponse()
	if hr == nil || hr.CallsSeed == nil {
		return LoginResponse{}, fmt.Errorf("api: missing handshake response for login")
	}

	ua := s.env.UserAgent()
	arch := ua.Arch
	if arch == "" {
		arch = "arm64-v8a"
	}

	var ccf []byte
	if s.env.GenerateFingerprint != nil {
		var err error
		ccf, err = s.env.GenerateFingerprint(sess.DeviceID, *hr.CallsSeed, arch)
		if err != nil {
			return LoginResponse{}, err
		}
	}

	sync := s.env.Sync.Resolve(sess.Sync)
	payload := map[string]any{
		"userAgent":    userAgentPayload(ua),
		"token":        sess.Token,
		"chatsSync":    sync.ChatsSync,
		"contactsSync": sync.ContactsSync,
		"draftsSync":   sync.DraftsSync,
		"interactive":  s.env.Interactive(),
		"presenceSync": sync.PresenceSync,
		"configHash":   sync.ConfigHash,
		"exp":          map[string]any{"chatsCountGroups": []byte{0x0a, 0x32}},
	}
	if ccf != nil {
		payload["chatCacheFingerprint"] = ccf
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeLogin, payload)
	if err != nil {
		return LoginResponse{}, err
	}
	resp, err := requireModel[LoginResponse](frame)
	if err != nil {
		return LoginResponse{}, err
	}
	s.updateSession(resp)
	return resp, nil
}

// WebLogin performs the web-client login handshake, a port of pymax's
// AuthService.web_login.
func (s *AuthService) WebLogin(ctx context.Context) (LoginResponse, error) {
	sess := s.env.Session()
	if sess == nil {
		return LoginResponse{}, fmt.Errorf("api: no session available for login")
	}

	sync := s.env.Sync.Resolve(sess.Sync)
	payload := map[string]any{
		"token":        sess.Token,
		"chatsCount":   40,
		"interactive":  s.env.Interactive(),
		"chatsSync":    sync.ChatsSync,
		"contactsSync": sync.ContactsSync,
		"presenceSync": sync.PresenceSync,
		"draftsSync":   sync.DraftsSync,
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeLogin, payload)
	if err != nil {
		return LoginResponse{}, err
	}
	resp, err := requireModel[LoginResponse](frame)
	if err != nil {
		return LoginResponse{}, err
	}
	s.updateSession(resp)
	return resp, nil
}

// MobileLogin2 performs the optional second login phase (profile/contacts),
// a port of pymax's AuthService.mobile_login2.
func (s *AuthService) MobileLogin2(ctx context.Context, flags Login2Flags) (Login2Response, error) {
	sess := s.env.Session()
	if sess == nil {
		return Login2Response{}, fmt.Errorf("api: no session available for login2")
	}
	sync := s.env.Sync.Resolve(sess.Sync)

	contactsSync := int64(-1)
	if flags.ContactEnabled {
		contactsSync = sync.ContactsSync
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeLogin2, map[string]any{
		"needProfile":  flags.ProfileEnabled,
		"contactsSync": contactsSync,
		"configHash":   sync.ConfigHash,
	})
	if err != nil {
		return Login2Response{}, err
	}
	resp, err := requireModel[Login2Response](frame)
	if err != nil {
		return Login2Response{}, err
	}

	sess2 := s.env.Session()
	if sess2 != nil {
		updated := *sess2
		updated.MtInstanceID = s.env.MtInstanceID
		updated.Sync = resp.UpdateSyncState(sess2.Sync)
		s.env.SetSession(&updated)
		if s.env.Store != nil {
			_ = s.env.Store.SaveSession(ctx, updated)
		}
	}
	return resp, nil
}

func (s *AuthService) updateSession(resp LoginResponse) {
	sess := s.env.Session()
	if sess == nil {
		return
	}
	updated := *sess
	updated.MtInstanceID = s.env.MtInstanceID
	updated.Sync = resp.UpdateSyncState(sess.Sync)
	s.env.SetSession(&updated)
	if s.env.Store != nil {
		_ = s.env.Store.SaveSession(context.Background(), updated)
	}
}

// RequestQR starts QR-code login and returns the link to show the user, a
// port of pymax's AuthService.request_qr.
func (s *AuthService) RequestQR(ctx context.Context) (RequestQrResponse, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeGetQR, map[string]any{})
	if err != nil {
		return RequestQrResponse{}, err
	}
	return requireModel[RequestQrResponse](frame)
}

// CheckQR polls the status of a QR login, a port of pymax's
// AuthService.check_qr.
func (s *AuthService) CheckQR(ctx context.Context, trackID string) (CheckQrResponse, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeGetQRStatus, map[string]any{"trackId": trackID})
	if err != nil {
		return CheckQrResponse{}, err
	}
	return requireModel[CheckQrResponse](frame)
}

// ConfirmQR exchanges a confirmed QR login for a session token, a port of
// pymax's AuthService.confirm_qr.
func (s *AuthService) ConfirmQR(ctx context.Context, trackID string) (CheckCodeResponse, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeLoginByQR, map[string]any{"trackId": trackID})
	if err != nil {
		return CheckCodeResponse{}, err
	}
	return requireModel[CheckCodeResponse](frame)
}

// AuthorizeQRLogin approves a QR login as the already-authenticated
// account scanning it, a port of pymax's AuthService.authorize_qr_login.
func (s *AuthService) AuthorizeQRLogin(ctx context.Context, qrLink string) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeAuthQRApprove, map[string]any{"qrLink": qrLink})
	return err
}

// createAuthTrack starts a fresh auth "track" used by the 2FA management
// endpoints (Set2FA, Remove2FA, ChangePassword), a port of pymax's
// AuthService._get_track_id.
func (s *AuthService) createAuthTrack(ctx context.Context) (string, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeAuthCreateTrack, map[string]any{"type": 0})
	if err != nil {
		return "", err
	}
	trackID, _ := payloadItem(frame, "trackId").(string)
	if trackID == "" {
		return "", fmt.Errorf("api: failed to create auth track")
	}
	return trackID, nil
}

// setPassword submits the 2FA password to be validated for trackID, a port
// of pymax's AuthService._set_password.
func (s *AuthService) setPassword(ctx context.Context, trackID, password string) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeAuthValidatePassword, map[string]any{
		"trackId":  trackID,
		"password": password,
	})
	return err
}

// setEmail requests an email verification code and confirms it against
// trackID, a port of pymax's AuthService._set_email.
func (s *AuthService) setEmail(ctx context.Context, trackID, email string, provider EmailCodeProvider) error {
	if _, err := s.env.Invoke(ctx, protocol.OpcodeAuthVerifyEmail, map[string]any{
		"trackId": trackID,
		"email":   email,
	}); err != nil {
		return err
	}

	code, err := provider.GetCode(ctx, email)
	if err != nil {
		return err
	}

	_, err = s.env.Invoke(ctx, protocol.OpcodeAuthCheckEmail, map[string]any{
		"trackId":    trackID,
		"verifyCode": code,
	})
	return err
}

// setHint sets the 2FA password hint for trackID, a port of pymax's
// AuthService._set_hint.
func (s *AuthService) setHint(ctx context.Context, trackID, hint string) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeAuthValidateHint, map[string]any{
		"trackId": trackID,
		"hint":    hint,
	})
	return err
}

// checkTwoFactorPassword verifies the account's existing 2FA password
// against trackID before a management operation (remove/change password), a
// port of pymax's AuthService._check_2fa_password. Note this uses
// AUTH_CHECK_PASSWORD, distinct from CheckPassword's
// AUTH_LOGIN_CHECK_PASSWORD used during the login flow.
func (s *AuthService) checkTwoFactorPassword(ctx context.Context, trackID, password string) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeAuthCheckPassword, map[string]any{
		"trackId":  trackID,
		"password": password,
	})
	return err
}

// Set2FA sets the account's 2FA password, optionally attaching a recovery
// email and/or a password hint, a port of pymax's AuthService.set_2fa.
// email and hint are optional (nil means "not provided", matching pymax's
// MISSING sentinel). If email is provided and emailCodeProvider is nil,
// ConsoleEmailCodeProvider is used, matching pymax's default.
func (s *AuthService) Set2FA(ctx context.Context, password string, email, hint *string, emailCodeProvider EmailCodeProvider) (bool, error) {
	trackID, err := s.createAuthTrack(ctx)
	if err != nil {
		return false, err
	}

	if err := s.setPassword(ctx, trackID, password); err != nil {
		return false, err
	}

	hasEmail := email != nil
	hasHint := hint != nil

	if hasEmail {
		provider := emailCodeProvider
		if provider == nil {
			provider = ConsoleEmailCodeProvider{}
		}
		if err := s.setEmail(ctx, trackID, *email, provider); err != nil {
			return false, err
		}
	}

	if hasHint {
		if err := s.setHint(ctx, trackID, *hint); err != nil {
			return false, err
		}
	}

	expectedCapabilities := []TwoFactorAction{TwoFactorSetPassword}
	if hasHint {
		expectedCapabilities = append(expectedCapabilities, TwoFactorHint)
	}
	if hasEmail {
		expectedCapabilities = append(expectedCapabilities, TwoFactorEmail)
	}

	payload := map[string]any{
		"trackId":              trackID,
		"password":             password,
		"expectedCapabilities": expectedCapabilities,
	}
	if hasHint {
		payload["hint"] = *hint
	}

	if _, err := s.env.Invoke(ctx, protocol.OpcodeAuthSet2FA, payload); err != nil {
		return false, err
	}
	return true, nil
}

// Remove2FA disables the account's 2FA password, a port of pymax's
// AuthService.remove_2fa.
func (s *AuthService) Remove2FA(ctx context.Context, password string) (bool, error) {
	trackID, err := s.createAuthTrack(ctx)
	if err != nil {
		return false, err
	}

	if err := s.checkTwoFactorPassword(ctx, trackID, password); err != nil {
		return false, err
	}

	_, err = s.env.Invoke(ctx, protocol.OpcodeAuthSet2FA, map[string]any{
		"trackId":              trackID,
		"remove2fa":            true,
		"expectedCapabilities": []TwoFactorAction{TwoFactorRemove2FA},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Check2FA reports whether the account currently has a 2FA password
// enabled, a port of pymax's AuthService.check_2fa. It inspects the cached
// profile (populated by Login/MobileLogin2) rather than calling the server.
func (s *AuthService) Check2FA() bool {
	me := s.env.Me()
	if me == nil {
		return false
	}
	for _, opt := range me.ProfileOptions {
		if ProfileOption(opt) == ProfileOptionSecondFactorPasswordEnabled {
			return true
		}
	}
	return false
}

// ChangePassword changes the account's 2FA password from passwordOld to
// passwordNew, a port of pymax's AuthService.change_password.
func (s *AuthService) ChangePassword(ctx context.Context, passwordOld, passwordNew string) (bool, error) {
	trackID, err := s.createAuthTrack(ctx)
	if err != nil {
		return false, err
	}

	if err := s.checkTwoFactorPassword(ctx, trackID, passwordOld); err != nil {
		return false, err
	}

	if err := s.setPassword(ctx, trackID, passwordNew); err != nil {
		return false, err
	}

	_, err = s.env.Invoke(ctx, protocol.OpcodeAuthSet2FA, map[string]any{
		"trackId":              trackID,
		"password":             passwordNew,
		"expectedCapabilities": []TwoFactorAction{TwoFactorUpdatePassword},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *AuthService) mobileMode(ctx context.Context) ([]byte, error) {
	if s.env.GenerateFingerprint == nil {
		return nil, nil
	}
	hr := s.env.HandshakeResponse()
	if hr == nil || hr.CallsSeed == nil {
		return nil, fmt.Errorf("api: missing handshake response for mode fingerprint")
	}

	sess := s.env.Session()
	deviceID := s.env.DeviceID
	if sess != nil {
		deviceID = sess.DeviceID
	}

	ua := s.env.UserAgent()
	arch := ua.Arch
	if arch == "" {
		arch = "arm64-v8a"
	}
	return s.env.GenerateFingerprint(deviceID, *hr.CallsSeed, arch)
}

func userAgentPayload(ua session.UserAgent) map[string]any {
	m := map[string]any{
		"deviceType":   ua.DeviceType,
		"appVersion":   ua.AppVersion,
		"osVersion":    ua.OsVersion,
		"timezone":     ua.Timezone,
		"screen":       ua.Screen,
		"locale":       ua.Locale,
		"deviceName":   ua.DeviceName,
		"deviceLocale": ua.DeviceLocale,
	}
	if ua.PushDeviceType != "" {
		m["pushDeviceType"] = ua.PushDeviceType
	}
	if ua.Arch != "" {
		m["arch"] = ua.Arch
	}
	if ua.BuildNumber != 0 {
		m["buildNumber"] = ua.BuildNumber
	}
	if ua.HeaderUserAgent != "" {
		m["headerUserAgent"] = ua.HeaderUserAgent
	}
	return m
}
