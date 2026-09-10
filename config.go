package gomax

import (
	"crypto/rand"
	"encoding/hex"
	mathrand "math/rand"
	"time"

	"github.com/SonChegg/PyMax/api"
	"github.com/SonChegg/PyMax/auth"
	"github.com/SonChegg/PyMax/session"
)

// DeviceType is the kind of device gomax presents itself as, a port of
// pymax's api.session.enums.DeviceType.
type DeviceType string

const (
	DeviceAndroid DeviceType = "ANDROID"
	DeviceIOS     DeviceType = "IOS"
	DeviceDesktop DeviceType = "DESKTOP"
	DeviceWeb     DeviceType = "WEB"
)

// RegistrationConfig holds the profile data used to finish registering a
// brand-new account after SMS verification, re-exported from api for
// caller convenience.
type RegistrationConfig = api.RegistrationConfig

// androidDevices is a representative sample of real Android device
// identities pymax's ExtraConfig.generate_user_agent picks from at random.
//
// NOTE: trimmed from pymax's full ~30-entry table for brevity; behavior
// (a plausible, randomly chosen Android device fingerprint) is unchanged.
var androidDevices = []struct {
	name, osVersion, screen, arch string
}{
	{"Samsung SM-A525F", "Android 13", "405dpi 405dpi 1080x2400", "arm64-v8a"},
	{"Samsung SM-G991B", "Android 14", "421dpi 421dpi 1080x2400", "arm64-v8a"},
	{"Xiaomi 2109119DG", "Android 13", "395dpi 395dpi 1080x2400", "arm64-v8a"},
	{"Pixel 7", "Android 14", "416dpi 416dpi 1080x2400", "arm64-v8a"},
	{"Pixel 8", "Android 14", "428dpi 428dpi 1080x2400", "arm64-v8a"},
	{"OnePlus NE2213", "Android 14", "525dpi 525dpi 1440x3216", "arm64-v8a"},
	{"realme RMX3085", "Android 13", "409dpi 409dpi 1080x2400", "arm64-v8a"},
	{"HUAWEI ELS-NX9", "Android 12", "441dpi 441dpi 1080x2340", "arm64-v8a"},
	{"HONOR RMO-NX1", "Android 13", "391dpi 391dpi 1080x2388", "arm64-v8a"},
}

var localeTimezones = []struct{ locale, timezone string }{
	{"ru", "Europe/Moscow"},
	{"ru", "Europe/Kaliningrad"},
	{"ru", "Europe/Samara"},
	{"ru", "Asia/Yekaterinburg"},
	{"ru", "Asia/Novosibirsk"},
	{"ru", "Asia/Vladivostok"},
}

const (
	webAppVersion = "26.8.4"
	webScreen     = "1080x1920 1.0x"
	webHeaderUA   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
)

// GenerateDeviceID returns a random 16-hex-character device identifier, a
// port of pymax's config.generate_device_id.
func GenerateDeviceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateUserAgent picks a random plausible Android device identity for
// appVersion/buildNumber, a port of pymax's ExtraConfig.generate_user_agent.
func generateUserAgent(deviceType DeviceType, appVersion string, buildNumber int) session.UserAgent {
	device := androidDevices[mathrand.Intn(len(androidDevices))]
	locale := localeTimezones[mathrand.Intn(len(localeTimezones))]

	return session.UserAgent{
		DeviceType:     string(deviceType),
		AppVersion:     appVersion,
		OsVersion:      device.osVersion,
		Timezone:       locale.timezone,
		Screen:         device.screen,
		PushDeviceType: "GCM",
		Arch:           device.arch,
		Locale:         locale.locale,
		BuildNumber:    buildNumber,
		DeviceName:     device.name,
		DeviceLocale:   locale.locale,
	}
}

// generateWebUserAgent builds the user agent gomax's WebClient sends, a
// port of pymax's ExtraConfig.generate_web_user_agent.
func generateWebUserAgent() session.UserAgent {
	locale := localeTimezones[mathrand.Intn(len(localeTimezones))]
	return session.UserAgent{
		DeviceType:      string(DeviceWeb),
		AppVersion:      webAppVersion,
		OsVersion:       "Linux",
		Timezone:        locale.timezone,
		Screen:          webScreen,
		Locale:          locale.locale,
		DeviceName:      "Chrome",
		DeviceLocale:    locale.locale,
		HeaderUserAgent: webHeaderUA,
	}
}

// Config holds every tunable of Client/WebClient, a port of pymax's
// ExtraConfig merged with the parts of ClientConfig/DeviceConfig callers
// can influence. Zero values pick pymax's defaults; see NewClient/NewWebClient.
type Config struct {
	// Connection.
	Host       string // TCP host. Default: api2.oneme.ru
	Port       int    // TCP port. Default: 443
	DisableTLS bool   // Disable TLS on the raw TCP transport. Default: false (TLS enabled)
	URL        string // WebSocket URL for WebClient. Default: wss://api.oneme.ru/websocket
	Proxy      string // "socks5://" or "http(s)://" proxy URL.
	// DisableReconnect stops Start from reconnecting after network errors.
	// Default: false (reconnect enabled). A plain bool can't tell "unset"
	// from "explicitly false", so pymax's reconnect=True default is
	// expressed here as an opt-out rather than an opt-in field.
	DisableReconnect bool
	ReconnectDelay   time.Duration // Default: 1s
	RequestTimeout   time.Duration // Default: 30s
	UploadTimeout    time.Duration // Default: 15m

	// Session/auth.
	Token       string // Pre-issued token; skips interactive auth if set.
	WorkDir     string // Directory for the session database. Default: "."
	SessionName string // Session database file name. Default: session.db
	// NoPersistSession keeps the session in memory instead of SQLite.
	// Default: false (persisted to SQLite); see the DisableReconnect note
	// on why this is an opt-out flag rather than a plain "Persist" bool.
	NoPersistSession    bool
	Store               session.Store
	DeviceID            string
	MtInstanceID        string
	UserAgent           *session.UserAgent
	RegistrationConfig  *RegistrationConfig
	PasswordMaxAttempts *int
	// DisableRelogin turns off automatic re-authentication on a revoked
	// login token. Default: false (relogin enabled); see the
	// DisableReconnect note on why this is an opt-out flag.
	DisableRelogin bool
	Sync           session.SyncOverrides

	// App version (Client/mobile only).
	AppVersion string
	Catalog    *auth.VersionCatalog

	// Auth flow overrides. AuthFlow takes precedence over the
	// provider-specific fields below, matching pymax's auth_flow vs.
	// sms_code_provider/password_provider/qr_provider constructor args.
	AuthFlow         auth.Flow
	SmsCodeProvider  auth.SmsCodeProvider
	PasswordProvider auth.PasswordProvider
	QrProvider       auth.QrHandler
	// RegistrationProvider, if set, is used instead of RegistrationConfig
	// when SMS login discovers the phone has no MAX account yet: it's
	// called interactively and can retry with corrected input against the
	// same registration token if MAX rejects the name, without requiring a
	// new SMS code.
	RegistrationProvider auth.RegistrationProvider
}

func (c *Config) withDefaults() {
	if c.Host == "" {
		c.Host = "api2.oneme.ru"
	}
	if c.Port == 0 {
		c.Port = 443
	}
	if c.URL == "" {
		c.URL = "wss://api.oneme.ru/websocket"
	}
	if c.ReconnectDelay == 0 {
		c.ReconnectDelay = time.Second
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}
	if c.UploadTimeout == 0 {
		c.UploadTimeout = 15 * time.Minute
	}
	if c.WorkDir == "" {
		c.WorkDir = "."
	}
	if c.SessionName == "" {
		c.SessionName = "session.db"
	}
	if c.MtInstanceID == "" {
		c.MtInstanceID = GenerateDeviceID()
	}
	if c.AppVersion == "" {
		c.AppVersion = auth.RecommendedAppVersion
	}
}
