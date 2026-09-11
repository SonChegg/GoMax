package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SonChegg/PyMax/protocol"
)

var photoExtensions = map[string]string{
	".jpg": "image/jpg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
}

// mediaSource is the shared byte-source implementation behind Photo, File,
// Video, VideoNote and Voice, a port of pymax's files.base.BaseFile.
type mediaSource struct {
	raw  []byte
	path string
	url  string
	name string
}

func newMediaSource(raw []byte, path, url, name string) (mediaSource, error) {
	m := mediaSource{raw: raw, path: path, url: url, name: name}
	sources := 0
	if raw != nil {
		sources++
	}
	if path != "" {
		sources++
	}
	if url != "" {
		sources++
	}
	if sources == 0 {
		return m, fmt.Errorf("gomax: one of raw, url or path must be provided")
	}
	if sources > 1 {
		return m, fmt.Errorf("gomax: only one of raw, url or path must be provided")
	}
	if raw != nil && name == "" {
		return m, fmt.Errorf("gomax: name must be provided for raw data")
	}
	if m.name == "" {
		if path != "" {
			m.name = filepath.Base(path)
		} else if url != "" {
			m.name = filepath.Base(url)
		}
	}
	return m, nil
}

// Name returns the file name pymax sends in Content-Disposition.
func (m mediaSource) Name() string { return m.name }

// Open returns a reader over the source bytes plus their total size.
func (m mediaSource) Open(ctx context.Context) (io.ReadCloser, int64, error) {
	switch {
	case m.raw != nil:
		return io.NopCloser(bytes.NewReader(m.raw)), int64(len(m.raw)), nil
	case m.path != "":
		f, err := os.Open(m.path)
		if err != nil {
			return nil, 0, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		return f, info.Size(), nil
	case m.url != "":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
		if err != nil {
			return nil, 0, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		if resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, 0, fmt.Errorf("gomax: fetching %s: HTTP %s", m.url, resp.Status)
		}
		return resp.Body, resp.ContentLength, nil
	default:
		return nil, 0, fmt.Errorf("gomax: no source configured")
	}
}

// Photo is a photo to send as a message attachment, a port of pymax's
// files.photo.Photo.
type Photo struct{ mediaSource }

// NewPhotoFromPath builds a Photo read from a local file.
func NewPhotoFromPath(path string) (*Photo, error) {
	m, err := newMediaSource(nil, path, "", "")
	return &Photo{m}, err
}

// NewPhotoFromURL builds a Photo pymax will download before uploading.
func NewPhotoFromURL(sourceURL string) (*Photo, error) {
	m, err := newMediaSource(nil, "", sourceURL, "")
	return &Photo{m}, err
}

// NewPhotoFromBytes builds a Photo from in-memory bytes.
func NewPhotoFromBytes(name string, raw []byte) (*Photo, error) {
	m, err := newMediaSource(raw, "", "", name)
	return &Photo{m}, err
}

// validate checks the photo's extension/MIME type, a port of pymax's
// Photo.validate_photo.
func (p *Photo) validate() (ext, contentType string, err error) {
	source := p.path
	if source == "" {
		source = p.name
	}
	if p.url != "" {
		source = p.url
	}

	e := strings.ToLower(filepath.Ext(source))
	mime, ok := photoExtensions[e]
	if !ok {
		return "", "", fmt.Errorf("gomax: invalid photo extension: %s", e)
	}
	return strings.TrimPrefix(e, "."), mime, nil
}

// File is a generic file to send as a message attachment, a port of
// pymax's files.file.File.
type File struct{ mediaSource }

// NewFileFromPath builds a File read from a local file.
func NewFileFromPath(path string) (*File, error) {
	m, err := newMediaSource(nil, path, "", "")
	return &File{m}, err
}

// NewFileFromURL builds a File pymax will download before uploading.
func NewFileFromURL(sourceURL string) (*File, error) {
	m, err := newMediaSource(nil, "", sourceURL, "")
	return &File{m}, err
}

// NewFileFromBytes builds a File from in-memory bytes.
func NewFileFromBytes(name string, raw []byte) (*File, error) {
	m, err := newMediaSource(raw, "", "", name)
	return &File{m}, err
}

// Video is a video to send as a message attachment, a port of pymax's
// files.video.Video.
type Video struct{ mediaSource }

// NewVideoFromPath builds a Video read from a local file.
func NewVideoFromPath(path string) (*Video, error) {
	m, err := newMediaSource(nil, path, "", "")
	return &Video{m}, err
}

// NewVideoFromURL builds a Video pymax will download before uploading.
func NewVideoFromURL(sourceURL string) (*Video, error) {
	m, err := newMediaSource(nil, "", sourceURL, "")
	return &Video{m}, err
}

// NewVideoFromBytes builds a Video from in-memory bytes.
func NewVideoFromBytes(name string, raw []byte) (*Video, error) {
	m, err := newMediaSource(raw, "", "", name)
	return &Video{m}, err
}

// VideoNote is a round video message, a port of pymax's files.video.VideoNote.
//
// NOTE: pymax can auto-detect duration via the optional TinyTag dependency;
// gomax does not vendor an MP4/OGG duration prober, so DurationMs must be
// set explicitly.
type VideoNote struct {
	mediaSource
	DurationMs int64
}

// NewVideoNoteFromPath builds a VideoNote read from a local file.
func NewVideoNoteFromPath(path string, durationMs int64) (*VideoNote, error) {
	m, err := newMediaSource(nil, path, "", "")
	return &VideoNote{m, durationMs}, err
}

// Voice is a voice message (OGG) to send, a port of pymax's files.voice.Voice.
//
// NOTE: see VideoNote's NOTE regarding automatic duration detection.
type Voice struct {
	mediaSource
	DurationMs int64
}

// NewVoiceFromPath builds a Voice read from a local OGG file.
func NewVoiceFromPath(path string, durationMs int64) (*Voice, error) {
	m, err := newMediaSource(nil, path, "", "")
	return &Voice{m, durationMs}, err
}

// NewVoiceFromBytes builds a Voice from in-memory bytes (e.g. a browser's
// MediaRecorder output) — duration must be supplied explicitly since gomax
// doesn't vendor an audio-container duration prober, same as VideoNote.
func NewVoiceFromBytes(name string, raw []byte, durationMs int64) (*Voice, error) {
	m, err := newMediaSource(raw, "", "", name)
	return &Voice{m, durationMs}, err
}

// AttachPhotoPayload is a photo ready to attach to a message, a port of
// pymax's api.uploads.payloads.AttachPhotoPayload.
type AttachPhotoPayload struct {
	Type       string `json:"_type"`
	PhotoToken string `json:"photoToken"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

// VideoAttachPayload is a video/voice/video-note ready to attach to a
// message, a port of pymax's api.uploads.payloads.VideoAttachPayload
// (VoiceAttachPayload/VideoNoteAttachPayload specialize it).
type VideoAttachPayload struct {
	Type      string `json:"_type"`
	VideoID   int64  `json:"videoId,omitempty"`
	AudioID   int64  `json:"audioId,omitempty"`
	Token     string `json:"token,omitempty"`
	VideoType int    `json:"videoType,omitempty"`
	Thumbhash []byte `json:"thumbhash,omitempty"`
	Duration  int64  `json:"duration,omitempty"`
	Wave      []byte `json:"wave,omitempty"`
}

// AttachFilePayload is a file ready to attach to a message, a port of
// pymax's api.uploads.payloads.AttachFilePayload.
type AttachFilePayload struct {
	Type   string `json:"_type"`
	FileID int64  `json:"fileId"`
}

type photoUploadResponse struct {
	Photos map[string]struct {
		Token string `json:"token"`
	} `json:"photos"`
}

type videoUploadResponse struct {
	Info []struct {
		URL     string `json:"url"`
		VideoID int64  `json:"videoId"`
		Token   string `json:"token"`
	} `json:"info"`
}

type fileUploadResponse struct {
	Info []struct {
		URL    string `json:"url"`
		FileID int64  `json:"fileId"`
		Token  string `json:"token"`
	} `json:"info"`
}

// UploadService implements photo/voice/video/file upload, a port of
// pymax's api.uploads.service.UploadService.
type UploadService struct {
	env *Env

	mu           sync.Mutex
	videoWaiters map[int64]chan struct{}
	fileWaiters  map[int64]chan struct{}
	voiceWaiters map[int64]chan struct{}
}

// NewUploadService builds an upload service bound to env.
func NewUploadService(env *Env) *UploadService {
	return &UploadService{
		env:          env,
		videoWaiters: make(map[int64]chan struct{}),
		fileWaiters:  make(map[int64]chan struct{}),
		voiceWaiters: make(map[int64]chan struct{}),
	}
}

// HandleVideoReady resolves the waiter for a NOTIF_ATTACH video-ready
// signal, a port of pymax's UploadService.on_video_attach. Called by the
// root runtime after dispatching a VideoReady event.
func (s *UploadService) HandleVideoReady(videoID int64) {
	s.mu.Lock()
	ch, ok := s.videoWaiters[videoID]
	if ok {
		delete(s.videoWaiters, videoID)
	}
	s.mu.Unlock()
	if ok {
		close(ch)
	}
}

// HandleFileReady resolves the waiter for a NOTIF_ATTACH file-ready signal,
// a port of pymax's UploadService.on_file_attach.
func (s *UploadService) HandleFileReady(fileID int64) {
	s.mu.Lock()
	ch, ok := s.fileWaiters[fileID]
	if ok {
		delete(s.fileWaiters, fileID)
	}
	s.mu.Unlock()
	if ok {
		close(ch)
	}
}

// HandleVoiceReady resolves the waiter for a NOTIF_ATTACH voice-ready
// signal, a port of pymax's UploadService.on_voice_attach.
func (s *UploadService) HandleVoiceReady(audioID int64) {
	s.mu.Lock()
	ch, ok := s.voiceWaiters[audioID]
	if ok {
		delete(s.voiceWaiters, audioID)
	}
	s.mu.Unlock()
	if ok {
		close(ch)
	}
}

// UploadPhoto uploads photo (optionally as the account's profile photo) and
// returns the attachment payload, a port of pymax's
// UploadService.upload_photo.
func (s *UploadService) UploadPhoto(ctx context.Context, photo *Photo, profile bool) (AttachPhotoPayload, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodePhotoUpload, map[string]any{"count": 1, "profile": profile})
	if err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: request photo upload url: %w", err)
	}

	rawURL, _ := payloadItem(frame, "url").(string)
	if rawURL == "" {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: no upload url received")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: invalid photo upload url: %w", err)
	}
	photoID := parsed.Query().Get("photoIds")
	if photoID == "" {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: photo upload url does not contain photoIds")
	}

	ext, contentType, err := photo.validate()
	if err != nil {
		return AttachPhotoPayload{}, err
	}

	reader, _, err := photo.Open(ctx)
	if err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: read photo: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: read photo: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "image."+url.QueryEscape(ext))
	if err != nil {
		return AttachPhotoPayload{}, err
	}
	if _, err := part.Write(data); err != nil {
		return AttachPhotoPayload{}, err
	}
	_ = contentType
	if err := writer.Close(); err != nil {
		return AttachPhotoPayload{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, &body)
	if err != nil {
		return AttachPhotoPayload{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: photo upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: photo upload failed with status %d", resp.StatusCode)
	}

	var result photoUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: decode photo upload response: %w", err)
	}
	entry, ok := result.Photos[photoID]
	if !ok {
		return AttachPhotoPayload{}, fmt.Errorf("gomax: photo upload response missing token for photo_id=%s", photoID)
	}

	// The upload response only carries a token, no dimensions — and without
	// an explicit width/height on the *message* attach payload below, the
	// server falls back to auto-cropping every photo to a square preview
	// (the sender sees that crop; other clients requesting the photo by
	// its real photoId/token later still get the untouched original,
	// which is why this only ever showed up for the sender, not the
	// recipient). Android supplies the true dimensions it already has
	// from decoding the file before upload; decode them here the same way.
	var width, height int
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, height = cfg.Width, cfg.Height
	}

	return AttachPhotoPayload{Type: "PHOTO", PhotoToken: entry.Token, Width: width, Height: height}, nil
}

// UploadVoice uploads a voice message and returns the attachment payload,
// a port of pymax's UploadService.upload_voice.
func (s *UploadService) UploadVoice(ctx context.Context, voice *Voice) (VideoAttachPayload, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeVideoUpload, map[string]any{"count": 1, "type": 2, "uploaderType": 1})
	if err != nil {
		return VideoAttachPayload{}, fmt.Errorf("gomax: request voice upload url: %w", err)
	}

	var result videoUploadResponse
	if err := remarshal(frame.Payload, &result); err != nil || len(result.Info) == 0 {
		return VideoAttachPayload{}, fmt.Errorf("gomax: invalid voice upload response")
	}
	info := result.Info[0]

	reader, size, err := voice.Open(ctx)
	if err != nil {
		return VideoAttachPayload{}, err
	}
	defer reader.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, info.URL, reader)
	if err != nil {
		return VideoAttachPayload{}, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", url.QueryEscape(voice.Name())))
	req.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", size-1, size))
	req.Header.Set("Content-Type", "application/octet-stream")

	// Like UploadVideo's non-note path: the upload POST completing doesn't
	// mean Max has finished processing the audio server-side. Referencing
	// audioId in SendMessage before the matching NOTIF_ATTACH arrives (routed
	// to HandleVoiceReady by runtime.onEvent) got "video.not.ready" back —
	// this wait was the one thing UploadVoice never actually did despite the
	// voiceWaiters/HandleVoiceReady plumbing already existing for it.
	waitCh := make(chan struct{})
	s.mu.Lock()
	s.voiceWaiters[info.VideoID] = waitCh
	s.mu.Unlock()

	resp, err := s.httpClient().Do(req)
	if err != nil {
		s.dropVoiceWaiter(info.VideoID)
		return VideoAttachPayload{}, fmt.Errorf("gomax: voice upload: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		s.dropVoiceWaiter(info.VideoID)
		return VideoAttachPayload{}, fmt.Errorf("gomax: voice upload failed with status %d", resp.StatusCode)
	}
	// The upload CDN answers audio-content rejections (wrong codec, no
	// audible signal, ...) with HTTP 200 and an {"error_code":...} body
	// instead of a non-200 status, so the status check above alone lets a
	// rejected upload sail through to a wait that would then simply never
	// resolve (Max never sends the ready notification for a file it threw
	// away) until ctx's deadline.
	var uploadErr struct {
		ErrorCode string `json:"error_code"`
		ErrorData string `json:"error_data"`
	}
	if json.Unmarshal(respBody, &uploadErr) == nil && uploadErr.ErrorCode != "" {
		s.dropVoiceWaiter(info.VideoID)
		return VideoAttachPayload{}, fmt.Errorf("gomax: voice upload rejected: %s", uploadErr.ErrorData)
	}

	select {
	case <-waitCh:
	case <-ctx.Done():
		s.dropVoiceWaiter(info.VideoID)
		return VideoAttachPayload{}, ctx.Err()
	}

	return VideoAttachPayload{
		Type:     "AUDIO",
		AudioID:  info.VideoID,
		Duration: voice.DurationMs,
		Wave:     bytes.Repeat([]byte{0}, 80),
	}, nil
}

func (s *UploadService) dropVoiceWaiter(audioID int64) {
	s.mu.Lock()
	delete(s.voiceWaiters, audioID)
	s.mu.Unlock()
}

// UploadVideo uploads a video or video-note and returns the attachment
// payload, a port of pymax's UploadService.upload_video.
func (s *UploadService) UploadVideo(ctx context.Context, video *Video, videoNote *VideoNote) (VideoAttachPayload, error) {
	payload := map[string]any{"count": 1, "type": 0, "uploaderType": 0}
	source := mediaSource{}
	isNote := videoNote != nil
	if isNote {
		payload["type"] = 1
		payload["uploaderType"] = 1
		source = videoNote.mediaSource
	} else {
		source = video.mediaSource
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeVideoUpload, payload)
	if err != nil {
		return VideoAttachPayload{}, fmt.Errorf("gomax: request video upload url: %w", err)
	}

	var result videoUploadResponse
	if err := remarshal(frame.Payload, &result); err != nil || len(result.Info) == 0 {
		return VideoAttachPayload{}, fmt.Errorf("gomax: invalid video upload response")
	}
	info := result.Info[0]

	reader, size, err := source.Open(ctx)
	if err != nil {
		return VideoAttachPayload{}, err
	}
	defer reader.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, info.URL, reader)
	if err != nil {
		return VideoAttachPayload{}, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", url.QueryEscape(source.Name())))
	req.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", size-1, size))

	var waitCh chan struct{}
	if !isNote {
		waitCh = make(chan struct{})
		s.mu.Lock()
		s.videoWaiters[info.VideoID] = waitCh
		s.mu.Unlock()
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		s.dropVideoWaiter(info.VideoID)
		return VideoAttachPayload{}, fmt.Errorf("gomax: video upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.dropVideoWaiter(info.VideoID)
		return VideoAttachPayload{}, fmt.Errorf("gomax: video upload failed with status %d", resp.StatusCode)
	}

	if isNote {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		var thumbhash []byte
		if v, ok := body["thumbhash"].(string); ok && v != "" {
			if pad := len(v) % 4; pad != 0 {
				v += strings.Repeat("=", 4-pad)
			}
			thumbhash, _ = base64.StdEncoding.DecodeString(v)
		}
		return VideoAttachPayload{
			Type:      "VIDEO",
			VideoID:   info.VideoID,
			Token:     info.Token,
			VideoType: 1,
			Thumbhash: thumbhash,
			Duration:  videoNote.DurationMs,
		}, nil
	}

	select {
	case <-waitCh:
	case <-ctx.Done():
		s.dropVideoWaiter(info.VideoID)
		return VideoAttachPayload{}, ctx.Err()
	}

	return VideoAttachPayload{Type: "VIDEO", VideoID: info.VideoID, Token: info.Token}, nil
}

func (s *UploadService) dropVideoWaiter(videoID int64) {
	s.mu.Lock()
	delete(s.videoWaiters, videoID)
	s.mu.Unlock()
}

// UploadFile uploads a generic file and returns the attachment payload, a
// port of pymax's UploadService.upload_file.
func (s *UploadService) UploadFile(ctx context.Context, file *File) (AttachFilePayload, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeFileUpload, map[string]any{"count": 1, "type": 0, "uploaderType": 0})
	if err != nil {
		return AttachFilePayload{}, fmt.Errorf("gomax: request file upload url: %w", err)
	}

	var result fileUploadResponse
	if err := remarshal(frame.Payload, &result); err != nil || len(result.Info) == 0 {
		return AttachFilePayload{}, fmt.Errorf("gomax: invalid file upload response")
	}
	info := result.Info[0]

	reader, size, err := file.Open(ctx)
	if err != nil {
		return AttachFilePayload{}, err
	}
	defer reader.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, info.URL, reader)
	if err != nil {
		return AttachFilePayload{}, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", url.QueryEscape(file.Name())))
	req.Header.Set("Content-Range", fmt.Sprintf("0-%d/%d", size-1, size))

	waitCh := make(chan struct{})
	s.mu.Lock()
	s.fileWaiters[info.FileID] = waitCh
	s.mu.Unlock()

	resp, err := s.httpClient().Do(req)
	if err != nil {
		s.mu.Lock()
		delete(s.fileWaiters, info.FileID)
		s.mu.Unlock()
		return AttachFilePayload{}, fmt.Errorf("gomax: file upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.mu.Lock()
		delete(s.fileWaiters, info.FileID)
		s.mu.Unlock()
		return AttachFilePayload{}, fmt.Errorf("gomax: file upload failed with status %d", resp.StatusCode)
	}

	select {
	case <-waitCh:
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.fileWaiters, info.FileID)
		s.mu.Unlock()
		return AttachFilePayload{}, ctx.Err()
	}

	return AttachFilePayload{Type: "FILE", FileID: info.FileID}, nil
}

func (s *UploadService) httpClient() *http.Client {
	if s.env.Proxy == "" {
		return http.DefaultClient
	}
	proxyURL, err := url.Parse(s.env.Proxy)
	if err != nil {
		return http.DefaultClient
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
}
