package gomax

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SonChegg/PyMax/api"
	"github.com/SonChegg/PyMax/auth"
	"github.com/SonChegg/PyMax/connection"
	"github.com/SonChegg/PyMax/dispatch"
	"github.com/SonChegg/PyMax/protocol"
	protocoltcp "github.com/SonChegg/PyMax/protocol/tcp"
	protocolws "github.com/SonChegg/PyMax/protocol/websocket"
	"github.com/SonChegg/PyMax/session"
	"github.com/SonChegg/PyMax/transport"
	"github.com/SonChegg/PyMax/types"
)

// Facade groups every api service, a port of pymax's api.facade.ApiFacade.
type Facade struct {
	Auth     *api.AuthService
	Messages *api.MessageService
	Chats    *api.ChatService
	Users    *api.UserService
	Uploads  *api.UploadService
	Sessions *api.SessionsService
}

// runtime is the shared connection/session/dispatch machinery behind Client
// and WebClient, a port of pymax's app.App. C is the concrete client type
// (*Client or *WebClient) handlers are invoked with.
type runtime[C any] struct {
	cfg   Config
	isWeb bool
	phone string

	env        *api.Env
	api        *Facade
	dispatcher *dispatch.Dispatcher[C]
	authFlow   auth.Flow
	store      session.Store
	catalog    *auth.VersionCatalog

	mu         sync.Mutex
	conn       *connection.Manager
	started    bool
	pingCancel context.CancelFunc
	pingDone   chan struct{}
}

func newRuntime[C any](cfg Config, isWeb bool, phone string, authFlow auth.Flow, router *dispatch.Router[C]) (*runtime[C], error) {
	cfg.withDefaults()

	env := api.NewEnv()
	env.DeviceID = cfg.DeviceID
	env.MtInstanceID = cfg.MtInstanceID
	env.Proxy = cfg.Proxy
	env.UploadTimeout = cfg.UploadTimeout
	env.RegistrationConfig = cfg.RegistrationConfig
	env.PasswordMaxAttempts = cfg.PasswordMaxAttempts
	env.Sync = cfg.Sync
	env.AppVersion = cfg.AppVersion
	env.SetInteractive(true) // pymax's ClientConfig.interactive defaults to True; toggle at
	// runtime via UserService.SetPresence, matching pymax's SelfService.set_presence.

	uploads := api.NewUploadService(env)
	facade := &Facade{
		Auth:     api.NewAuthService(env),
		Messages: api.NewMessageService(env, uploads),
		Chats:    api.NewChatService(env, uploads),
		Users:    api.NewUserService(env),
		Uploads:  uploads,
		Sessions: api.NewSessionsService(env),
	}

	catalog, err := auth.NewVersionCatalog()
	if err != nil {
		return nil, err
	}

	return &runtime[C]{
		cfg:        cfg,
		isWeb:      isWeb,
		phone:      phone,
		env:        env,
		api:        facade,
		dispatcher: dispatch.NewDispatcher(router),
		authFlow:   authFlow,
		catalog:    catalog,
	}, nil
}

// buildConnection constructs the TCP or WebSocket transport/reader/codec
// stack, a port of pymax's Client._build_connection / WebClient._build_connection.
func (r *runtime[C]) buildConnection() (*connection.Manager, error) {
	if r.isWeb {
		t := transport.NewWebSocketTransport(r.cfg.URL, r.cfg.Proxy)
		reader := connection.NewWSReader(t)
		return connection.NewManager(reader, t, protocolws.NewProtocol()), nil
	}

	t, err := transport.NewTCPTransport(r.cfg.Host, r.cfg.Port, r.cfg.Proxy, r.cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	reader := connection.NewTCPReader(t)
	return connection.NewManager(reader, t, protocoltcp.NewProtocol()), nil
}

// buildStore builds the session store, a port of pymax's App.__init__'s
// store selection.
func (r *runtime[C]) buildStore() (session.Store, error) {
	if r.cfg.Store != nil {
		return r.cfg.Store, nil
	}
	if !r.cfg.PersistSession {
		return session.NewInMemoryStore(), nil
	}
	return session.NewSQLiteStore(r.cfg.WorkDir, r.cfg.SessionName)
}

// invoke sends opcode/payload and returns the decoded response, erroring
// with *APIError on cmd=ERROR responses, a port of pymax's App.invoke.
func (r *runtime[C]) invoke(ctx context.Context, opcode protocol.Opcode, payload map[string]any) (protocol.InboundFrame, error) {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn == nil {
		return protocol.InboundFrame{}, fmt.Errorf("gomax: client runtime is not initialized")
	}

	seq := conn.NextSeq()
	frame := protocol.OutboundFrame{
		Ver:     r.codecVersion(),
		Opcode:  opcode,
		Cmd:     protocol.CommandRequest,
		Seq:     seq,
		Payload: payload,
	}

	reqCtx := ctx
	var cancel context.CancelFunc
	if r.cfg.RequestTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, r.cfg.RequestTimeout)
		defer cancel()
	}

	resp, err := conn.Request(reqCtx, frame)
	if err != nil {
		return protocol.InboundFrame{}, err
	}
	if resp.Cmd == protocol.CommandError {
		return resp, r.buildAPIError(resp)
	}
	return resp, nil
}

func (r *runtime[C]) codecVersion() int {
	if r.isWeb {
		return 11
	}
	return 10
}

func (r *runtime[C]) buildAPIError(resp protocol.InboundFrame) *APIError {
	var apiErr apiErrorPayload
	_ = decodePayload(resp.Payload, &apiErr)
	return &APIError{
		Opcode:           resp.Opcode,
		Code:             apiErr.Error,
		Title:            apiErr.Title,
		Message:          apiErr.Message,
		LocalizedMessage: apiErr.LocalizedMessage,
		Payload:          resp.Payload,
	}
}

// open connects, handshakes and authenticates, a port of pymax's App.start.
func (r *runtime[C]) open(ctx context.Context) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if r.store == nil {
		store, err := r.buildStore()
		if err != nil {
			return err
		}
		r.store = store
		r.env.Store = store
	}

	conn, err := r.buildConnection()
	if err != nil {
		return err
	}
	conn.OnEvent = r.onEvent
	conn.OnClose = r.onClose

	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()

	sess, err := r.store.LoadSession(ctx)
	if err != nil {
		return err
	}

	deviceID := r.cfg.DeviceID
	if deviceID == "" {
		deviceID = GenerateDeviceID()
		r.cfg.DeviceID = deviceID
	}
	mtInstanceID := r.cfg.MtInstanceID

	userAgent := r.buildUserAgent()
	if sess != nil {
		if sess.MtInstanceID != "" {
			mtInstanceID = sess.MtInstanceID
		} else {
			sess.MtInstanceID = mtInstanceID
		}
		userAgent = session.ResolveUserAgent(userAgent, sess.UserAgent)
		deviceID = sess.DeviceID
	}
	r.env.MtInstanceID = mtInstanceID
	r.env.SetUserAgent(userAgent)

	if err := conn.Open(ctx); err != nil {
		return err
	}

	handshake, err := r.api.Sessions.Handshake(ctx, r.isWeb, userAgent, deviceID)
	if err != nil {
		_ = conn.Close(ctx)
		return fmt.Errorf("gomax: handshake failed: %w", err)
	}
	r.env.SetHandshakeResponse(&handshake)

	if !r.isWeb {
		fp, err := r.catalog.Resolve(r.cfg.AppVersion)
		if err != nil {
			_ = conn.Close(ctx)
			return err
		}
		gen := auth.NewFingerprintGenerator(fp)
		r.env.GenerateFingerprint = gen.GenerateFingerprint
	}

	r.startPingLoop()

	if sess == nil {
		if r.cfg.Token != "" {
			sess = &session.Info{
				Token:        r.cfg.Token,
				DeviceID:     deviceID,
				Phone:        r.phone,
				MtInstanceID: mtInstanceID,
				UserAgent:    &userAgent,
				Sync:         session.NewSyncState(),
			}
		} else {
			deps := auth.Deps{
				Auth:                r.api.Auth,
				Phone:               r.phone,
				RegistrationConfig:  r.cfg.RegistrationConfig,
				PasswordMaxAttempts: r.cfg.PasswordMaxAttempts,
			}
			result, err := r.authFlow.Authenticate(ctx, deps)
			if err != nil {
				r.stopPingLoop()
				_ = conn.Close(ctx)
				return err
			}
			if result.Token == "" {
				r.stopPingLoop()
				_ = conn.Close(ctx)
				return fmt.Errorf("gomax: authentication failed: no token received")
			}
			sess = &session.Info{
				Token:        result.Token,
				DeviceID:     deviceID,
				Phone:        r.phone,
				MtInstanceID: mtInstanceID,
				UserAgent:    &userAgent,
				Sync:         session.NewSyncState(),
			}
		}
		if err := r.store.SaveSession(ctx, *sess); err != nil {
			r.stopPingLoop()
			_ = conn.Close(ctx)
			return err
		}
	}
	r.env.SetSession(sess)

	loginResp, err := r.api.Auth.Login(ctx, r.isWeb)
	if err != nil {
		r.stopPingLoop()
		_ = conn.Close(ctx)
		return err
	}

	var login2Resp *api.Login2Response
	if loginResp.Login2Flags != nil && loginResp.Login2Flags.Enabled() {
		resp, err := r.api.Auth.MobileLogin2(ctx, *loginResp.Login2Flags)
		if err != nil {
			r.stopPingLoop()
			_ = conn.Close(ctx)
			return err
		}
		login2Resp = &resp
	}

	var me *types.Profile
	switch {
	case login2Resp != nil && loginResp.Login2Flags != nil && loginResp.Login2Flags.ProfileEnabled && login2Resp.Profile != nil:
		me = login2Resp.Profile
	case loginResp.Profile != nil:
		me = loginResp.Profile
	case login2Resp != nil && login2Resp.Profile != nil:
		me = login2Resp.Profile
	default:
		r.stopPingLoop()
		_ = conn.Close(ctx)
		return fmt.Errorf("gomax: login response does not contain a profile")
	}

	if loginResp.Token != nil && *loginResp.Token != sess.Token {
		_ = r.store.UpdateToken(ctx, sess.Token, *loginResp.Token)
		sess.Token = *loginResp.Token
		r.env.SetSession(sess)
	}

	r.env.SetMe(me)
	r.env.CacheUser(&me.Contact)
	r.env.SetChats(loginResp.Chats)

	if login2Resp != nil && loginResp.Login2Flags != nil && loginResp.Login2Flags.ContactEnabled {
		r.env.SetContacts(login2Resp.Contacts)
	} else {
		r.env.SetContacts(loginResp.Contacts)
	}

	messages := map[int64][]*types.Message{}
	for chatIDStr, msgs := range loginResp.Messages {
		var chatID int64
		fmt.Sscanf(chatIDStr, "%d", &chatID)
		messages[chatID] = msgs
	}
	r.env.SetMessages(messages)

	r.mu.Lock()
	r.started = true
	r.mu.Unlock()

	return nil
}

func (r *runtime[C]) buildUserAgent() session.UserAgent {
	if r.cfg.UserAgent != nil {
		return *r.cfg.UserAgent
	}
	if r.isWeb {
		return generateWebUserAgent()
	}
	fp, err := r.catalog.Resolve(r.cfg.AppVersion)
	build := 0
	if err == nil {
		build = fp.BuildNumber
	}
	return generateUserAgent(DeviceAndroid, r.cfg.AppVersion, build)
}

func (r *runtime[C]) startPingLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.mu.Lock()
	r.pingCancel = cancel
	r.pingDone = done
	r.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if _, err := r.invoke(ctx, protocol.OpcodePing, map[string]any{"interactive": r.env.Interactive()}); err != nil {
				if ctx.Err() != nil {
					return
				}
				r.mu.Lock()
				conn := r.conn
				r.mu.Unlock()
				if conn != nil {
					conn.Fail(context.Background(), fmt.Errorf("gomax: ping failed: %w", err))
				}
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *runtime[C]) stopPingLoop() {
	r.mu.Lock()
	cancel := r.pingCancel
	done := r.pingDone
	r.pingCancel = nil
	r.pingDone = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *runtime[C]) onClose(err error) {
	r.mu.Lock()
	r.started = false
	r.mu.Unlock()
}

func (r *runtime[C]) onEvent(ctx context.Context, frame protocol.InboundFrame) {
	r.dispatcher.Dispatch(ctx, frame)

	if frame.Cmd != protocol.CommandRequest || frame.Opcode != protocol.OpcodeNotifAttach {
		return
	}
	if v, ok := frame.Payload["fileId"]; ok {
		r.api.Uploads.HandleFileReady(toInt64(v))
	} else if v, ok := frame.Payload["videoId"]; ok {
		r.api.Uploads.HandleVideoReady(toInt64(v))
	} else if v, ok := frame.Payload["audioId"]; ok {
		r.api.Uploads.HandleVoiceReady(toInt64(v))
	}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}

func (r *runtime[C]) close(ctx context.Context) error {
	r.stopPingLoop()
	r.dispatcher.StopStartupTasks()

	r.mu.Lock()
	conn := r.conn
	r.started = false
	r.mu.Unlock()

	var err error
	if conn != nil {
		err = conn.Close(ctx)
	}
	if r.store != nil {
		_ = r.store.Close(ctx)
	}
	return err
}

func (r *runtime[C]) isOpen() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conn != nil && r.conn.IsOpen()
}

func (r *runtime[C]) waitClosed(ctx context.Context) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.WaitClosed(ctx)
}
