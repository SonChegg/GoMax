package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SonChegg/PyMax/internal/markdown"
	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// ItemType selects regular vs. delayed ("send later") history items, a
// port of pymax's api.messages.enums.ItemType.
type ItemType string

const (
	ItemTypeRegular ItemType = "REGULAR"
	ItemTypeDelayed ItemType = "DELAYED"
)

// ReadAction identifies what CHAT_MARK is marking, a port of pymax's
// api.messages.enums.ReadAction.
type ReadAction string

const (
	ReadActionMessage  ReadAction = "READ_MESSAGE"
	ReadActionReaction ReadAction = "READ_REACTION"
)

// SendAttachment is any value MessageService.SendMessage/EditMessage
// accepts as an outgoing attachment: *Photo, *File, *Video, *VideoNote,
// *Voice or types.Poll.
type SendAttachment any

// MessageService implements message send/edit/history/reactions/etc, a
// port of pymax's api.messages.service.MessageService.
type MessageService struct {
	env     *Env
	uploads *UploadService

	mu   sync.Mutex
	prev int64
}

// NewMessageService builds a message service bound to env, delegating
// attachment uploads to uploads.
func NewMessageService(env *Env, uploads *UploadService) *MessageService {
	return &MessageService{env: env, uploads: uploads, prev: time.Now().UnixMilli()}
}

func (s *MessageService) nextCid() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	if now <= s.prev {
		now = s.prev + 1
	}
	s.prev = now
	return now
}

func (s *MessageService) uploadAttachments(ctx context.Context, attachments []SendAttachment) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		switch v := attachment.(type) {
		case *Voice:
			p, err := s.uploads.UploadVoice(ctx, v)
			if err != nil {
				return nil, fmt.Errorf("gomax: voice upload failed: %w", err)
			}
			out = append(out, toPayload(p))
		case *Photo:
			p, err := s.uploads.UploadPhoto(ctx, v, false)
			if err != nil {
				return nil, fmt.Errorf("gomax: photo upload failed: %w", err)
			}
			out = append(out, toPayload(p))
		case *Video:
			p, err := s.uploads.UploadVideo(ctx, v, nil)
			if err != nil {
				return nil, fmt.Errorf("gomax: video upload failed: %w", err)
			}
			out = append(out, toPayload(p))
		case *VideoNote:
			p, err := s.uploads.UploadVideo(ctx, nil, v)
			if err != nil {
				return nil, fmt.Errorf("gomax: video upload failed: %w", err)
			}
			out = append(out, toPayload(p))
		case *File:
			p, err := s.uploads.UploadFile(ctx, v)
			if err != nil {
				return nil, fmt.Errorf("gomax: file upload failed: %w", err)
			}
			out = append(out, toPayload(p))
		case types.Poll:
			out = append(out, map[string]any{
				"_type":    "POLL",
				"title":    v.Title,
				"answers":  v.Answers,
				"settings": int(v.Settings),
			})
		default:
			return nil, fmt.Errorf("gomax: unsupported attachment type %T", attachment)
		}
	}
	return out, nil
}

func convertSendAt(sendAt time.Time) int64 { return sendAt.UnixMilli() }

// SendMessage sends a new message, a port of pymax's
// MessageService.send_message.
func (s *MessageService) SendMessage(ctx context.Context, chatID int64, text string, replyTo int64, attachments []SendAttachment, notify bool, sendAt *time.Time) (*types.Message, error) {
	if text == "" && len(attachments) == 0 {
		return nil, fmt.Errorf("gomax: either text or attachments must be provided")
	}

	var cleanText *string
	var elements []types.Element
	if text != "" {
		clean, els := markdown.Format(text)
		cleanText = &clean
		elements = els
	}

	attaches, err := s.uploadAttachments(ctx, attachments)
	if err != nil {
		return nil, err
	}

	message := map[string]any{
		"cid":      s.nextCid(),
		"elements": elements,
		"attaches": attaches,
	}
	if cleanText != nil {
		message["text"] = *cleanText
	}
	if replyTo != 0 {
		message["link"] = map[string]any{"type": "REPLY", "messageId": replyTo}
	}
	if sendAt != nil {
		message["delayedAttributes"] = map[string]any{
			"timeToFire":   convertSendAt(*sendAt),
			"notifySender": notify,
		}
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgSend, map[string]any{
		"chatId":  chatID,
		"message": message,
		"notify":  notify,
	})
	if err != nil {
		return nil, err
	}
	sent, err := requireModel[types.Message](frame)
	if err != nil {
		return nil, err
	}
	return &sent, nil
}

// ForwardMessage forwards messageID from sourceChatID into chatID, a port
// of pymax's MessageService.forward_message.
func (s *MessageService) ForwardMessage(ctx context.Context, chatID, messageID, sourceChatID int64, notify bool) (*types.Message, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgSend, map[string]any{
		"chatId": chatID,
		"message": map[string]any{
			"cid": -s.nextCid(),
			"link": map[string]any{
				"type":      "FORWARD",
				"messageId": fmt.Sprint(messageID),
				"chatId":    sourceChatID,
			},
			"attaches": []map[string]any{},
		},
		"notify": notify,
	})
	if err != nil {
		return nil, err
	}
	message, err := requireModel[types.Message](frame)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// GetMessages fetches specific messages by ID, a port of pymax's
// MessageService.get_messages.
func (s *MessageService) GetMessages(ctx context.Context, chatID int64, messageIDs []int64) ([]types.Message, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgGet, map[string]any{
		"chatId":     chatID,
		"messageIds": messageIDs,
	})
	if err != nil {
		return nil, err
	}
	messages, err := decodeList[types.Message](frame, "messages")
	if err != nil {
		return nil, err
	}
	for i := range messages {
		if messages[i].ChatID == nil {
			messages[i].ChatID = &chatID
		}
	}
	return messages, nil
}

// GetMessage fetches a single message by ID, a port of pymax's
// MessageService.get_message.
func (s *MessageService) GetMessage(ctx context.Context, chatID, messageID int64) (*types.Message, error) {
	messages, err := s.GetMessages(ctx, chatID, []int64{messageID})
	if err != nil || len(messages) == 0 {
		return nil, err
	}
	return &messages[0], nil
}

// EditMessage edits an existing message's text/attachments, a port of
// pymax's MessageService.edit_message.
func (s *MessageService) EditMessage(ctx context.Context, chatID, messageID int64, text string, attachments []SendAttachment) (*types.Message, error) {
	if text == "" && len(attachments) == 0 {
		return nil, fmt.Errorf("gomax: either text or attachments must be provided")
	}

	var cleanText *string
	var elements []types.Element
	if text != "" {
		clean, els := markdown.Format(text)
		cleanText = &clean
		elements = els
	}

	attaches, err := s.uploadAttachments(ctx, attachments)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"chatId":      chatID,
		"messageId":   messageID,
		"elements":    elements,
		"attachments": attaches,
	}
	if cleanText != nil {
		payload["text"] = *cleanText
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgEdit, payload)
	if err != nil {
		return nil, err
	}
	message, err := requireItemModel[types.Message](frame, "message")
	if err != nil {
		return nil, err
	}
	if message.ChatID == nil {
		message.ChatID = &chatID
	}
	return &message, nil
}

// FetchHistory loads a chat's message history, a port of pymax's
// MessageService.fetch_history.
func (s *MessageService) FetchHistory(ctx context.Context, chatID int64, forward, backward int, backwardTime, forwardTime, fromTime int64, itemType ItemType, getChat, getMessages, interactive bool) ([]types.Message, error) {
	if fromTime == 0 {
		fromTime = time.Now().UnixMilli()
	}
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatHistory, map[string]any{
		"chatId":       chatID,
		"forward":      forward,
		"backward":     backward,
		"backwardTime": backwardTime,
		"forwardTime":  forwardTime,
		"from":         fromTime,
		"itemType":     string(itemType),
		"getChat":      getChat,
		"getMessages":  getMessages,
		"interactive":  interactive,
	})
	if err != nil {
		return nil, err
	}
	return decodeList[types.Message](frame, "messages")
}

// DeleteMessage deletes one or more messages, a port of pymax's
// MessageService.delete_message.
func (s *MessageService) DeleteMessage(ctx context.Context, chatID int64, messageIDs []int64, forMe bool) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeMsgDelete, map[string]any{
		"chatId":     chatID,
		"messageIds": messageIDs,
		"forMe":      forMe,
	})
	return err
}

// PinMessage pins a message in a chat, a port of pymax's
// MessageService.pin_message.
func (s *MessageService) PinMessage(ctx context.Context, chatID, messageID int64, notifyPin bool) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeChatUpdate, map[string]any{
		"chatId":       chatID,
		"notifyPin":    notifyPin,
		"pinMessageId": messageID,
	})
	return err
}

// GetVideoByID resolves a temporary playback URL for a video attachment, a
// port of pymax's MessageService.get_video_by_id.
func (s *MessageService) GetVideoByID(ctx context.Context, chatID, messageID, videoID int64) (*types.VideoRequest, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeVideoPlay, map[string]any{
		"chatId": chatID, "messageId": messageID, "videoId": videoID,
	})
	if err != nil {
		return nil, err
	}
	return decodeModel[types.VideoRequest](frame)
}

// GetFileByID resolves a temporary download URL for a file attachment, a
// port of pymax's MessageService.get_file_by_id.
func (s *MessageService) GetFileByID(ctx context.Context, chatID, messageID, fileID int64) (*types.FileRequest, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeFileDownload, map[string]any{
		"chatId": chatID, "messageId": messageID, "fileId": fileID,
	})
	if err != nil {
		return nil, err
	}
	return decodeModel[types.FileRequest](frame)
}

// AddReaction adds or replaces the account's reaction on a message, a port
// of pymax's MessageService.add_reaction.
func (s *MessageService) AddReaction(ctx context.Context, chatID, messageID int64, reaction string) (*types.ReactionInfo, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgReaction, map[string]any{
		"chatId":    chatID,
		"messageId": messageID,
		"reaction":  map[string]any{"reactionType": "EMOJI", "id": reaction},
	})
	if err != nil {
		return nil, err
	}
	return decodeItemModel[types.ReactionInfo](frame, "reactionInfo")
}

// GetReactions fetches the reactions on one or more messages, a port of
// pymax's MessageService.get_reactions.
func (s *MessageService) GetReactions(ctx context.Context, chatID int64, messageIDs []int64) (map[string]types.ReactionInfo, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgGetReactions, map[string]any{
		"chatId": chatID, "messageIds": messageIDs,
	})
	if err != nil {
		return nil, err
	}
	item := payloadItem(frame, "messagesReactions")
	if item == nil {
		return nil, nil
	}
	var out map[string]types.ReactionInfo
	if err := remarshal(item, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveReaction removes the account's reaction from a message, a port of
// pymax's MessageService.remove_reaction.
func (s *MessageService) RemoveReaction(ctx context.Context, chatID, messageID int64) (*types.ReactionInfo, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgCancelReaction, map[string]any{
		"chatId": chatID, "messageId": messageID,
	})
	if err != nil {
		return nil, err
	}
	return decodeItemModel[types.ReactionInfo](frame, "reactionInfo")
}

// ReadMessage marks a message read, a port of pymax's
// MessageService.read_message.
func (s *MessageService) ReadMessage(ctx context.Context, chatID, messageID int64) (types.ReadState, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatMark, map[string]any{
		"type":      string(ReadActionMessage),
		"chatId":    chatID,
		"messageId": messageID,
		"mark":      time.Now().UnixMilli(),
	})
	if err != nil {
		return types.ReadState{}, err
	}
	return requireModel[types.ReadState](frame)
}

// VotePoll casts a vote on a poll attachment, a port of pymax's
// MessageService.vote_poll.
func (s *MessageService) VotePoll(ctx context.Context, chatID, messageID, pollID int64, answerIDs []int64) (types.PollState, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeSendVote, map[string]any{
		"chatId": chatID, "messageId": messageID, "pollId": pollID, "answersIds": answerIDs,
	})
	if err != nil {
		return types.PollState{}, err
	}
	return requireItemModel[types.PollState](frame, "state")
}
