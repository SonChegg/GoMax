package types

import "encoding/json"

// MessageStatus is the delivery/edit status of a message, a port of pymax's
// types.domain.enums.MessageStatus.
type MessageStatus string

const (
	MessageStatusEdited  MessageStatus = "EDITED"
	MessageStatusRemoved MessageStatus = "REMOVED"
)

// LinkType identifies whether a message Link is a reply or a forward, a
// port of pymax's types.domain.enums.LinkType.
type LinkType string

const (
	LinkTypeReply   LinkType = "REPLY"
	LinkTypeForward LinkType = "FORWARD"
)

// AccessType is a chat's access level, a port of pymax's
// types.domain.enums.AccessType.
type AccessType string

const (
	AccessPublic  AccessType = "PUBLIC"
	AccessPrivate AccessType = "PRIVATE"
	AccessSecret  AccessType = "SECRET"
)

// ElementAttributes carries extra data for a formatted text Element (e.g.
// the URL of a LINK element), a port of pymax's
// types.domain.element.ElementAttributes.
type ElementAttributes struct {
	URL *string `json:"url,omitempty"`
}

// Element is one formatted-text span of a message, a port of pymax's
// types.domain.element.Element.
type Element struct {
	Type       string             `json:"type"`
	From       *int               `json:"from,omitempty"`
	Length     *int               `json:"length,omitempty"`
	Attributes *ElementAttributes `json:"attributes,omitempty"`
}

// ReactionCounter is the count of one reaction kind on a message, a port of
// pymax's types.domain.message.ReactionCounter.
type ReactionCounter struct {
	Count    int    `json:"count"`
	Reaction string `json:"reaction"`
}

// ReactionInfo summarizes the reactions on a message, a port of pymax's
// types.domain.message.ReactionInfo.
type ReactionInfo struct {
	TotalCount   int               `json:"totalCount"`
	Counters     []ReactionCounter `json:"counters,omitempty"`
	YourReaction *string           `json:"yourReaction,omitempty"`
}

// ReadState is returned after marking a message read, a port of pymax's
// types.domain.message.ReadState.
type ReadState struct {
	Unread int `json:"unread"`
	Mark   int `json:"mark"`
}

// DelayedAttributes describes a scheduled ("send later") message, a port of
// pymax's types.domain.message.DelayedAttributes.
type DelayedAttributes struct {
	TimeToFire      int64 `json:"timeToFire"`
	NotifySender    bool  `json:"notifySender"`
	NotifyOpponents bool  `json:"notifyOpponents"`
}

// Link is the source-message link of a reply or forward, a port of pymax's
// types.domain.message.ReplyLink / ForwardLink union.
type Link struct {
	Type LinkType `json:"type"`

	Message *Message `json:"message"`
	ChatID  int64    `json:"chatId"`

	// Forward-only fields.
	ChatName       *string     `json:"chatName,omitempty"`
	ChatLink       *string     `json:"chatLink,omitempty"`
	ChatAccessType *AccessType `json:"chatAccessType,omitempty"`
	ChatIconURL    *string     `json:"chatIconUrl,omitempty"`
}

// Message is a Max chat message, a port of pymax's types.domain.message.Message.
//
// NOTE: pymax attaches convenience methods to Message (reply/answer/edit/...)
// by binding a live service reference onto the model after it is decoded.
// gomax deliberately does not replicate that auto-binding (it would require
// mutable, implicitly-shared state on a plain data type); the equivalent
// operations are plain methods on api.MessageService that take the message
// or chat/message IDs explicitly, e.g. client.Messages.Reply(ctx, msg, ...).
type Message struct {
	ID            int64              `json:"id"`
	ChatID        *int64             `json:"chatId,omitempty"`
	Sender        *int64             `json:"sender,omitempty"`
	Text          string             `json:"text"`
	Time          int64              `json:"time"`
	MsgType       string             `json:"type"`
	Cid           *int64             `json:"cid,omitempty"`
	Attaches      Attachments        `json:"attaches,omitempty"`
	Stats         map[string]any     `json:"stats,omitempty"`
	Status        *MessageStatus     `json:"status,omitempty"`
	ReactionInfo  *ReactionInfo      `json:"reactionInfo,omitempty"`
	Options       any                `json:"options,omitempty"`
	PrevMessageID any                `json:"prevMessageId,omitempty"`
	TTL           *bool              `json:"ttl,omitempty"`
	Unread        *int               `json:"unread,omitempty"`
	Mark          *int               `json:"mark,omitempty"`
	Elements      []Element          `json:"elements,omitempty"`
	DelayedAttrs  *DelayedAttributes `json:"delayedAttributes,omitempty"`
	Link          *Link              `json:"link,omitempty"`
}

// messageWireEvent is the shape of a "new message" notification frame,
// where the message is nested under "message" alongside event-only fields.
// A port of pymax's Message._unwrap_message_event before-validator.
type messageWireEvent struct {
	Message       json.RawMessage `json:"message"`
	ChatID        *int64          `json:"chatId"`
	PrevMessageID any             `json:"prevMessageId"`
	TTL           *bool           `json:"ttl"`
	Unread        *int            `json:"unread"`
	Mark          *int            `json:"mark"`
}

// UnmarshalJSON accepts either a bare message payload or a notification
// envelope ({"message": {...}, "chatId": ..., ...}), unwrapping the latter
// exactly like pymax's Message._unwrap_message_event.
func (m *Message) UnmarshalJSON(data []byte) error {
	var envelope messageWireEvent
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Message) > 0 {
		type alias Message
		var inner alias
		if err := json.Unmarshal(envelope.Message, &inner); err != nil {
			return err
		}
		*m = Message(inner)
		if envelope.ChatID != nil {
			m.ChatID = envelope.ChatID
		}
		if envelope.PrevMessageID != nil {
			m.PrevMessageID = envelope.PrevMessageID
		}
		if envelope.TTL != nil {
			m.TTL = envelope.TTL
		}
		if envelope.Unread != nil {
			m.Unread = envelope.Unread
		}
		if envelope.Mark != nil {
			m.Mark = envelope.Mark
		}
		return nil
	}

	type alias Message
	var direct alias
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}
	*m = Message(direct)
	return nil
}
