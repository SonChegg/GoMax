package types

import (
	"encoding/json"
	"testing"
)

func TestMessageAttachmentsPolymorphicUnmarshal(t *testing.T) {
	data := []byte(`{
		"id": 1,
		"time": 1000,
		"type": "TEXT",
		"text": "hi",
		"attaches": [
			{"_type": "PHOTO", "baseUrl": "u", "height": 1, "width": 2, "photoId": 3, "photoToken": "tok"},
			{"_type": "SOME_FUTURE_TYPE", "foo": "bar"}
		]
	}`)

	var msg Message
	if err := msg.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(msg.Attaches) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(msg.Attaches))
	}

	photo, ok := msg.Attaches[0].(PhotoAttachment)
	if !ok {
		t.Fatalf("expected PhotoAttachment, got %T", msg.Attaches[0])
	}
	if photo.PhotoID != 3 || photo.PhotoToken != "tok" {
		t.Fatalf("unexpected photo attachment: %+v", photo)
	}

	unknown, ok := msg.Attaches[1].(UnknownAttachment)
	if !ok {
		t.Fatalf("expected UnknownAttachment, got %T", msg.Attaches[1])
	}
	if unknown.Type() != "SOME_FUTURE_TYPE" || unknown.Raw["foo"] != "bar" {
		t.Fatalf("unexpected unknown attachment: %+v", unknown)
	}
}

// TestAttachmentsMarshalJSONRoundTripsType guards against a regression: a
// consumer decoding a Message (e.g. a web front-end re-serializing it to
// its own JSON API) needs "_type" to tell a StickerAttachment from a
// PhotoAttachment from anything else. Attachments previously only knew
// how to consume "_type" on decode, not re-emit it on encode, so
// marshaling a decoded Message silently dropped every attachment's type.
func TestAttachmentsMarshalJSONRoundTripsType(t *testing.T) {
	data := []byte(`{
		"id": 1,
		"time": 1000,
		"type": "TEXT",
		"text": "hi",
		"attaches": [
			{"_type": "STICKER", "url": "https://example/sticker.webp", "stickerId": 7, "width": 512, "height": 512, "time": 1, "stickerType": "STATIC", "audio": false},
			{"_type": "PHOTO", "baseUrl": "u", "height": 1, "width": 2, "photoId": 3, "photoToken": "tok"},
			{"_type": "SOME_FUTURE_TYPE", "foo": "bar"}
		]
	}`)

	var msg Message
	if err := msg.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	remarshaled, err := json.Marshal(msg.Attaches)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(remarshaled, &decoded); err != nil {
		t.Fatalf("unmarshal remarshaled: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("expected 3 attachments, got %d", len(decoded))
	}

	wantTypes := []string{"STICKER", "PHOTO", "SOME_FUTURE_TYPE"}
	for i, want := range wantTypes {
		if got := decoded[i]["_type"]; got != want {
			t.Fatalf("attachment %d: got _type=%v, want %q", i, got, want)
		}
	}
	if decoded[0]["stickerId"] != float64(7) {
		t.Fatalf("sticker fields lost in round-trip: %+v", decoded[0])
	}
}

func TestMessageUnwrapsNotificationEnvelope(t *testing.T) {
	data := []byte(`{
		"message": {"id": 5, "time": 1000, "type": "TEXT", "text": "hey"},
		"chatId": 99,
		"prevMessageId": 4,
		"unread": 1,
		"mark": 2
	}`)

	var msg Message
	if err := msg.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg.ID != 5 || msg.ChatID == nil || *msg.ChatID != 99 {
		t.Fatalf("unexpected message after envelope unwrap: %+v", msg)
	}
	if msg.Unread == nil || *msg.Unread != 1 {
		t.Fatalf("expected unread=1, got %+v", msg.Unread)
	}
}
