package types

import "testing"

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
