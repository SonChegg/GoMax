package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SonChegg/PyMax/protocol"
)

// encodeTestPNG builds a minimal w x h PNG so UploadPhoto has real image
// bytes to decode dimensions from.
func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

// TestUploadPhotoReportsRealDimensions guards against a regression where
// the message's attach payload was missing width/height entirely: Max's
// send-confirmation then echoed the photo back as 1x1, and the server's own
// image pipeline defaulted to auto-cropping the sender's own view of the
// photo to a square preview — a real bug reported against mappi, fixed by
// having UploadPhoto decode the source bytes itself and set Width/Height.
func TestUploadPhotoReportsRealDimensions(t *testing.T) {
	const wantW, wantH = 267, 106

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		resp := map[string]any{"photos": map[string]any{"photo-id-1": map[string]any{"token": "tok-abc"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ri := &recordedInvoke{responses: map[protocol.Opcode]map[string]any{
		protocol.OpcodePhotoUpload: {"url": fmt.Sprintf("%s?photoIds=photo-id-1", srv.URL)},
	}}
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewUploadService(env)

	photo, err := NewPhotoFromBytes("test.png", encodeTestPNG(t, wantW, wantH))
	if err != nil {
		t.Fatalf("NewPhotoFromBytes: %v", err)
	}

	payload, err := svc.UploadPhoto(context.Background(), photo, false)
	if err != nil {
		t.Fatalf("UploadPhoto: %v", err)
	}
	if payload.PhotoToken != "tok-abc" {
		t.Errorf("PhotoToken = %q, want tok-abc", payload.PhotoToken)
	}
	if payload.Width != wantW || payload.Height != wantH {
		t.Errorf("dimensions = %dx%d, want %dx%d", payload.Width, payload.Height, wantW, wantH)
	}
}

// TestUploadVoiceFromBytes guards NewVoiceFromBytes (added for mappi's
// browser-recorded voice messages, which only ever exist as in-memory
// bytes — never a path on disk like NewVoiceFromPath assumes) end to end
// through UploadVoice.
func TestUploadVoiceFromBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		resp := map[string]any{"info": []map[string]any{{"url": "unused", "videoId": int64(555), "token": "tok-voice"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ri := &recordedInvoke{responses: map[protocol.Opcode]map[string]any{
		protocol.OpcodeVideoUpload: {"info": []map[string]any{{"url": srv.URL, "videoId": int64(555), "token": "tok-voice"}}},
	}}
	env := NewEnv()
	env.Invoke = ri.invoke
	svc := NewUploadService(env)

	voice, err := NewVoiceFromBytes("voice.ogg", []byte("fake opus bytes"), 4200)
	if err != nil {
		t.Fatalf("NewVoiceFromBytes: %v", err)
	}

	payload, err := svc.UploadVoice(context.Background(), voice)
	if err != nil {
		t.Fatalf("UploadVoice: %v", err)
	}
	if payload.Type != "AUDIO" {
		t.Errorf("Type = %q, want AUDIO", payload.Type)
	}
	if payload.AudioID != 555 {
		t.Errorf("AudioID = %d, want 555", payload.AudioID)
	}
	if payload.Duration != 4200 {
		t.Errorf("Duration = %d, want 4200", payload.Duration)
	}
}
