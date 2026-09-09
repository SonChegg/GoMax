package types

// ChatType is the kind of a chat, a port of pymax's
// types.domain.enums.ChatType.
type ChatType string

const (
	ChatTypeDialog  ChatType = "DIALOG"
	ChatTypeChat    ChatType = "CHAT"
	ChatTypeChannel ChatType = "CHANNEL"
)

// Chat is a Max dialog, group or channel, a port of pymax's
// types.domain.chat.Chat.
//
// NOTE: see the NOTE on Message regarding pymax's auto-bound convenience
// methods (answer/history/leave/...); gomax exposes the equivalent
// operations as explicit api.ChatService/api.MessageService methods instead.
type Chat struct {
	ID                       int64            `json:"id"`
	Type                     ChatType         `json:"type"`
	Status                   string           `json:"status"`
	Owner                    int64            `json:"owner"`
	Participants             map[string]int64 `json:"participants,omitempty"`
	Title                    *string          `json:"title,omitempty"`
	BaseRawIconURL           *string          `json:"baseRawIconUrl,omitempty"`
	BaseIconURL              *string          `json:"baseIconUrl,omitempty"`
	LastMessage              *Message         `json:"lastMessage,omitempty"`
	LastEventTime            int64            `json:"lastEventTime"`
	LastDelayedUpdateTime    int64            `json:"lastDelayedUpdateTime"`
	LastFireDelayedErrorTime int64            `json:"lastFireDelayedErrorTime"`
	Created                  int64            `json:"created"`
	NewMessages              int              `json:"newMessages"`
	Link                     *string          `json:"link,omitempty"`
	Access                   *AccessType      `json:"access,omitempty"`
	Restrictions             *int             `json:"restrictions,omitempty"`
	PinnedMessage            *Message         `json:"pinnedMessage,omitempty"`
	ParticipantsCount        int              `json:"participantsCount"`
	Description              *string          `json:"description,omitempty"`
	Options                  any              `json:"options,omitempty"`
	JoinTime                 int64            `json:"joinTime"`
	InvitedBy                *int64           `json:"invitedBy,omitempty"`
	Modified                 int64            `json:"modified"`
	MessagesCount            int              `json:"messagesCount"`
	HasBots                  *bool            `json:"hasBots,omitempty"`
	PrevMessageID            *int64           `json:"prevMessageId,omitempty"`
	AdminParticipants        map[string]any   `json:"adminParticipants,omitempty"`
	Admins                   []int64          `json:"admins,omitempty"`
	Cid                      *int64           `json:"cid,omitempty"`
}

// IsDialog reports whether the chat is a one-on-one dialog.
func (c Chat) IsDialog() bool { return c.Type == ChatTypeDialog }

// IsGroup reports whether the chat is a group.
func (c Chat) IsGroup() bool { return c.Type == ChatTypeChat }

// IsChannel reports whether the chat is a channel.
func (c Chat) IsChannel() bool { return c.Type == ChatTypeChannel }
