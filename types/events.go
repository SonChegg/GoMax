package types

import "encoding/json"

// TypingEvent notifies that a user is typing in a chat, a port of pymax's
// types.events.typing.TypingEvent.
type TypingEvent struct {
	ChatID int64 `json:"chatId"`
	UserID int64 `json:"userId"`
}

// ReactionUpdateEvent notifies that a message's reactions changed, a port
// of pymax's types.events.reaction.ReactionUpdateEvent.
type ReactionUpdateEvent struct {
	MessageID  string            `json:"messageId"`
	ChatID     int64             `json:"chatId"`
	Counters   []ReactionCounter `json:"counters,omitempty"`
	TotalCount int               `json:"totalCount"`
}

// PresenceEvent notifies that a user's presence changed, a port of pymax's
// types.events.presence.PresenceEvent.
type PresenceEvent struct {
	Presence Presence `json:"presence"`
	UserID   int64    `json:"userId"`
}

// MessageReadEvent notifies that a chat's read marker changed, a port of
// pymax's types.events.mark.MessageReadEvent.
type MessageReadEvent struct {
	SetAsUnread bool  `json:"setAsUnread"`
	ChatID      int64 `json:"chatId"`
	UserID      int64 `json:"userId"`
	Mark        int64 `json:"mark"`
}

// VideoUploadSignal notifies that an uploaded video finished processing, a
// port of pymax's types.events.video.VideoUploadSignal.
type VideoUploadSignal struct {
	VideoID int64 `json:"videoId"`
}

// FileUploadSignal notifies that an uploaded file finished processing, a
// port of pymax's types.events.file.FileUploadSignal.
type FileUploadSignal struct {
	FileID int64 `json:"fileId"`
}

// AudioUploadSignal notifies that an uploaded voice message finished
// processing, a port of pymax's types.events.voice.AudioUploadSignal.
type AudioUploadSignal struct {
	AudioID int64 `json:"audioId"`
}

// MessageDeleteEvent notifies that one or more messages were deleted, a
// port of pymax's types.events.message.MessageDeleteEvent.
//
// Max sends two different payload shapes for this event depending on the
// opcode (142 wraps a "chat" object, 128 wraps a "message" object);
// UnmarshalJSON normalizes both, a port of pymax's normalize_payload
// before-validator.
type MessageDeleteEvent struct {
	MessageIDs []int64  `json:"messageIds"`
	ChatID     int64    `json:"chatId"`
	Chat       *Chat    `json:"chat,omitempty"`
	Message    *Message `json:"message,omitempty"`
	TTL        bool     `json:"ttl"`
}

func (e *MessageDeleteEvent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	ttl := false
	if v, ok := raw["ttl"]; ok {
		_ = json.Unmarshal(v, &ttl)
	}
	e.TTL = ttl

	if chatRaw, ok := raw["chat"]; ok { // opcode == NOTIF_CHAT-style delete (142)
		var chat Chat
		if err := json.Unmarshal(chatRaw, &chat); err != nil {
			return err
		}
		e.Chat = &chat
		e.ChatID = chat.ID

		if idsRaw, ok := raw["messageIds"]; ok {
			return json.Unmarshal(idsRaw, &e.MessageIDs)
		}
		return nil
	}

	if messageRaw, ok := raw["message"]; ok { // opcode == NOTIF_MESSAGE-style delete (128)
		var message Message
		if err := json.Unmarshal(messageRaw, &message); err != nil {
			return err
		}
		e.Message = &message
		e.MessageIDs = []int64{message.ID}

		if chatIDRaw, ok := raw["chatId"]; ok {
			return json.Unmarshal(chatIDRaw, &e.ChatID)
		}
		return nil
	}

	return nil
}
