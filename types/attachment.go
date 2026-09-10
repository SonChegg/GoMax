// Package types holds the domain models exchanged with the Max protocol:
// messages, chats, users, attachments and dispatched events. It is a port
// of pymax's types.domain and types.events packages.
package types

import "encoding/json"

// AttachmentType identifies the kind of a message attachment, a port of
// pymax's types.domain.attachments.enums.AttachmentType.
type AttachmentType string

const (
	AttachmentPhoto          AttachmentType = "PHOTO"
	AttachmentVideo          AttachmentType = "VIDEO"
	AttachmentFile           AttachmentType = "FILE"
	AttachmentSticker        AttachmentType = "STICKER"
	AttachmentAudio          AttachmentType = "AUDIO"
	AttachmentControl        AttachmentType = "CONTROL"
	AttachmentContact        AttachmentType = "CONTACT"
	AttachmentCall           AttachmentType = "CALL"
	AttachmentShare          AttachmentType = "SHARE"
	AttachmentInlineKeyboard AttachmentType = "INLINE_KEYBOARD"
	AttachmentPoll           AttachmentType = "POLL"
	AttachmentUnknown        AttachmentType = "UNKNOWN"
)

// TranscriptionStatus is the status of an audio attachment's transcription,
// a port of pymax's types.domain.attachments.enums.TranscriptionStatus.
type TranscriptionStatus string

const (
	TranscriptionFailed        TranscriptionStatus = "FAILED"
	TranscriptionMediaNotReady TranscriptionStatus = "MEDIA_NOT_READY"
	TranscriptionNotSupported  TranscriptionStatus = "NOT_SUPPORTED"
	TranscriptionProcessing    TranscriptionStatus = "PROCESSING"
	TranscriptionSuccess       TranscriptionStatus = "SUCCESS"
	TranscriptionStatusUnknown TranscriptionStatus = "UNKNOWN"
)

// PollFlags is a bitmask of poll settings, a port of pymax's
// types.domain.attachments.enums.PollFlags.
type PollFlags int

const (
	PollFlagAnonymous   PollFlags = 1
	PollFlagMultiselect PollFlags = 2
	PollFlagRevote      PollFlags = 4
	PollFlagClosed      PollFlags = 8
	PollFlagQuiz        PollFlags = 16
	PollFlagCanForward  PollFlags = 32
)

// CallType is the medium of a call attachment.
type CallType string

const (
	CallTypeAudio CallType = "AUDIO"
	CallTypeVideo CallType = "VIDEO"
)

// HangupType is why a call attachment ended.
type HangupType string

const (
	HangupMissed   HangupType = "MISSED"
	HangupRejected HangupType = "REJECTED"
	HangupCanceled HangupType = "CANCELED"
	HangupHungup   HangupType = "HUNGUP"
)

// Attachment is implemented by every concrete attachment type. It is a port
// of pymax's types.domain.message.Attachment union.
type Attachment interface {
	// Type returns the attachment's discriminator ("_type" in the wire format).
	Type() AttachmentType
}

// PhotoAttachment is an inbound photo attachment, a port of pymax's
// types.domain.attachments.photo.PhotoAttachment.
type PhotoAttachment struct {
	BaseURL    string `json:"baseUrl"`
	Height     int    `json:"height"`
	Width      int    `json:"width"`
	PhotoID    int64  `json:"photoId"`
	PhotoToken string `json:"photoToken"`
}

func (PhotoAttachment) Type() AttachmentType { return AttachmentPhoto }

// VideoAttachment is an inbound video attachment, a port of pymax's
// types.domain.attachments.video.VideoAttachment.
type VideoAttachment struct {
	Height    int    `json:"height"`
	Width     int    `json:"width"`
	VideoID   int64  `json:"videoId"`
	Duration  *int   `json:"duration,omitempty"`
	Thumbnail string `json:"thumbnail"`
	Token     string `json:"token"`
	VideoType int    `json:"videoType"`
}

func (VideoAttachment) Type() AttachmentType { return AttachmentVideo }

// VideoRequest is returned by MessageService.GetVideoByID, a port of pymax's
// types.domain.attachments.video.VideoRequest. The direct download URL is
// picked as the highest-quality "MP4_<n>" key, falling back to legacy
// dynamicUrl, matching pymax's select_video_url validator.
type VideoRequest struct {
	External any    `json:"EXTERNAL,omitempty"`
	Cache    bool   `json:"cache"`
	URL      string `json:"url"`
}

// UnmarshalJSON implements the MP4_<quality> URL selection pymax performs
// in VideoRequest.select_video_url.
func (v *VideoRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if ext, ok := raw["EXTERNAL"]; ok {
		v.External = ext
	}
	if cache, ok := raw["cache"].(bool); ok {
		v.Cache = cache
	}

	if url, ok := raw["url"].(string); ok {
		v.URL = url
		return nil
	}

	bestQuality := -1
	bestURL := ""
	for key, val := range raw {
		urlStr, ok := val.(string)
		if !ok || len(key) < 5 || key[:4] != "MP4_" && key[:4] != "mp4_" {
			continue
		}
		quality := parseIntSafe(key[4:])
		if quality > bestQuality {
			bestQuality = quality
			bestURL = urlStr
		}
	}
	if bestURL != "" {
		v.URL = bestURL
		return nil
	}

	if legacy, ok := raw["dynamicUrl"].(string); ok {
		v.URL = legacy
	} else if legacy, ok := raw["dynamic_url"].(string); ok {
		v.URL = legacy
	}
	return nil
}

func parseIntSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// AudioAttachment is an inbound audio attachment, a port of pymax's
// types.domain.attachments.audio.AudioAttachment.
type AudioAttachment struct {
	Duration            *int                 `json:"duration,omitempty"`
	AudioID             *int64               `json:"audioId,omitempty"`
	Wave                *string              `json:"wave,omitempty"`
	TranscriptionStatus *TranscriptionStatus `json:"transcriptionStatus,omitempty"`
	URL                 *string              `json:"url,omitempty"`
	Token               *string              `json:"token,omitempty"`
}

func (AudioAttachment) Type() AttachmentType { return AttachmentAudio }

// FileAttachment is an inbound file attachment, a port of pymax's
// types.domain.attachments.file.FileAttachment.
type FileAttachment struct {
	FileID int64  `json:"fileId"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Token  string `json:"token"`
}

func (FileAttachment) Type() AttachmentType { return AttachmentFile }

// FileRequest is returned by MessageService.GetFileByID, a port of pymax's
// types.domain.attachments.file.FileRequest.
type FileRequest struct {
	Unsafe bool   `json:"unsafe"`
	URL    string `json:"url"`
}

// StickerAttachment is an inbound sticker attachment, a port of pymax's
// types.domain.attachments.sticker.StickerAttachment.
type StickerAttachment struct {
	AuthorType  *string  `json:"authorType,omitempty"`
	LottieURL   *string  `json:"lottieUrl,omitempty"`
	URL         string   `json:"url"`
	StickerID   int64    `json:"stickerId"`
	Tags        []string `json:"tags,omitempty"`
	Width       int      `json:"width"`
	SetID       *int64   `json:"setId,omitempty"`
	Time        int64    `json:"time"`
	StickerType string   `json:"stickerType"`
	Audio       bool     `json:"audio"`
	Height      int      `json:"height"`
}

func (StickerAttachment) Type() AttachmentType { return AttachmentSticker }

// ContactAttachment is an inbound shared-contact attachment, a port of
// pymax's types.domain.attachments.contact.ContactAttachment.
type ContactAttachment struct {
	ContactID *int64  `json:"contactId,omitempty"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	Name      *string `json:"name,omitempty"`
	PhotoURL  *string `json:"photoUrl,omitempty"`
}

func (ContactAttachment) Type() AttachmentType { return AttachmentContact }

// ShareAttachment is an inbound link-preview attachment, a port of pymax's
// types.domain.attachments.share.ShareAttachment.
type ShareAttachment struct {
	URL         *string        `json:"url,omitempty"`
	Title       *string        `json:"title,omitempty"`
	Description *string        `json:"description,omitempty"`
	Image       map[string]any `json:"image,omitempty"`
}

func (ShareAttachment) Type() AttachmentType { return AttachmentShare }

// PollAnswer is one answer option of a poll, a port of pymax's
// types.domain.attachments.poll.PollAnswer.
type PollAnswer struct {
	Text     string `json:"text"`
	AnswerID *int64 `json:"answerId,omitempty"`
}

// PollVote is a single user's vote for one answer, a port of pymax's
// types.domain.attachments.poll.PollVote.
type PollVote struct {
	Timestamp int64 `json:"timestamp"`
	UserID    int64 `json:"userId"`
}

// PollResult is the vote tally for one answer, a port of pymax's
// types.domain.attachments.poll.PollResult.
type PollResult struct {
	AnswerID  int64      `json:"answerId"`
	VoteCount int64      `json:"voteCount"`
	Votes     []PollVote `json:"votes"`
	Rate      int64      `json:"rate"`
	Options   int64      `json:"options"`
}

// PollState is the current tally of a poll, a port of pymax's
// types.domain.attachments.poll.PollState.
type PollState struct {
	Total           int64        `json:"total"`
	Result          []PollResult `json:"result,omitempty"`
	VoterPreviewIDs []int64      `json:"voterPreviewIds"`
}

// Poll is a poll to send as a message attachment, a port of pymax's
// types.domain.attachments.poll.Poll.
type Poll struct {
	Title    string       `json:"title"`
	Answers  []PollAnswer `json:"answers"`
	Settings PollFlags    `json:"settings"`
}

func (Poll) Type() AttachmentType { return AttachmentPoll }

// PollAttachment is a poll received as a message attachment, a port of
// pymax's types.domain.attachments.poll.PollAttachment.
type PollAttachment struct {
	Poll
	PollID  int64     `json:"pollId"`
	Version int       `json:"version"`
	State   PollState `json:"state"`
}

func (PollAttachment) Type() AttachmentType { return AttachmentPoll }

// CallAttachment is an inbound call-log attachment, a port of pymax's
// types.domain.attachments.call.CallAttachment.
type CallAttachment struct {
	Duration       *int        `json:"duration,omitempty"`
	ConversationID any         `json:"conversationId,omitempty"`
	ContactIDs     []int64     `json:"contactIds,omitempty"`
	CallType       *CallType   `json:"callType,omitempty"`
	HangupType     *HangupType `json:"hangupType,omitempty"`
}

func (CallAttachment) Type() AttachmentType { return AttachmentCall }

// ControlAttachment is an inbound service/control attachment (e.g. group
// created/renamed), a port of pymax's types.domain.attachments.control.ControlAttachment.
type ControlAttachment struct {
	Event string  `json:"event"`
	Title *string `json:"title,omitempty"`
}

func (ControlAttachment) Type() AttachmentType { return AttachmentControl }

// InlineKeyboardAttachment is an inbound inline-keyboard attachment, a port
// of pymax's types.domain.attachments.keyboards.inline.InlineKeyboardAttachment.
type InlineKeyboardAttachment struct {
	Keyboard map[string]any `json:"keyboard"`
}

func (InlineKeyboardAttachment) Type() AttachmentType { return AttachmentInlineKeyboard }

// UnknownAttachment holds any attachment whose "_type" pymax/gomax does not
// model explicitly, preserving the raw fields, a port of pymax's
// types.domain.attachments.unknown.UnknownAttachment.
type UnknownAttachment struct {
	RawType string
	Raw     map[string]any
}

func (u UnknownAttachment) Type() AttachmentType { return AttachmentType(u.RawType) }

// UnmarshalJSON dispatches on the "_type" discriminator field to the
// concrete Attachment implementation, a port of the discriminated union
// pymax builds with pydantic's Field(discriminator="type").
func unmarshalAttachment(data []byte) (Attachment, error) {
	var probe struct {
		Type string `json:"_type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}

	switch AttachmentType(probe.Type) {
	case AttachmentPhoto:
		var a PhotoAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentVideo:
		var a VideoAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentFile:
		var a FileAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentSticker:
		var a StickerAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentAudio:
		var a AudioAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentControl:
		var a ControlAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentContact:
		var a ContactAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentCall:
		var a CallAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentShare:
		var a ShareAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentInlineKeyboard:
		var a InlineKeyboardAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	case AttachmentPoll:
		var a PollAttachment
		err := json.Unmarshal(data, &a)
		return a, err
	default:
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		return UnknownAttachment{RawType: probe.Type, Raw: raw}, nil
	}
}

// Attachments is a slice of Attachment that knows how to unmarshal the
// mixed-type JSON array Max sends for Message.attaches.
type Attachments []Attachment

// UnmarshalJSON decodes each element via unmarshalAttachment.
func (a *Attachments) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	out := make(Attachments, 0, len(raws))
	for _, raw := range raws {
		att, err := unmarshalAttachment(raw)
		if err != nil {
			return err
		}
		out = append(out, att)
	}
	*a = out
	return nil
}

// MarshalJSON re-adds the "_type" discriminator field UnmarshalJSON
// consumes. Without this, Go's default marshaling of an interface slice
// only emits each concrete attachment's own struct fields, with nothing
// to tell a caller (e.g. a consumer re-serializing a decoded Message to
// its own JSON API) whether a given element was a PhotoAttachment, a
// StickerAttachment, or anything else — the discriminator only survived
// one direction.
func (a Attachments) MarshalJSON() ([]byte, error) {
	out := make([]map[string]any, len(a))
	for i, att := range a {
		if u, ok := att.(UnknownAttachment); ok {
			// Raw was captured from the original wire object before
			// dispatching on its type, so it already carries "_type".
			out[i] = u.Raw
			continue
		}

		data, err := json.Marshal(att)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		m["_type"] = string(att.Type())
		out[i] = m
	}
	return json.Marshal(out)
}
