package gomax

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SonChegg/PyMax/protocol"
)

// APIError is returned when the server answers a request with cmd=ERROR, a
// port of pymax's exceptions.ApiError.
type APIError struct {
	Opcode           protocol.Opcode
	Code             string // server-provided error code, e.g. "FAIL_LOGIN_TOKEN"
	Title            string
	Message          string
	LocalizedMessage string
	Payload          map[string]any
}

func (e *APIError) Error() string {
	parts := make([]string, 0, 4)
	for _, p := range []string{e.LocalizedMessage, e.Message} {
		if p != "" && !contains(parts, p) {
			parts = append(parts, p)
		}
	}
	if e.Title != "" {
		parts = append(parts, fmt.Sprintf("(%s)", e.Title))
	}
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.Code))
	}
	if len(parts) == 0 {
		return "api request failed"
	}
	return strings.Join(parts, " ")
}

func contains(items []string, v string) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}

// UploadError is returned when uploading a photo/video/voice/file fails, a
// port of pymax's exceptions.UploadError.
type UploadError struct{ msg string }

func (e *UploadError) Error() string { return e.msg }

// NewUploadError builds an UploadError with the given message.
func NewUploadError(msg string) *UploadError { return &UploadError{msg: msg} }

// apiErrorPayload is the wire shape of a cmd=ERROR response, a port of
// pymax's types.domain.error.MaxApiError.
type apiErrorPayload struct {
	Error            string `json:"error"`
	Title            string `json:"title"`
	Message          string `json:"message"`
	LocalizedMessage string `json:"localizedMessage"`
}

// decodePayload round-trips payload (a map[string]any) through JSON into
// dst.
func decodePayload(payload map[string]any, dst any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
