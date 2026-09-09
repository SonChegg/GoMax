package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// ChatMemberOperation is add-vs-remove for CHAT_MEMBERS_UPDATE requests, a
// port of pymax's api.chats.enums.ChatMemberOperation.
type ChatMemberOperation string

const (
	ChatMemberAdd    ChatMemberOperation = "add"
	ChatMemberRemove ChatMemberOperation = "remove"
)

// ChannelPermissions is a bitmask of admin permissions, a port of pymax's
// api.chats.enums.ChannelPermissions.
type ChannelPermissions int

const (
	PermAddRemoveMember ChannelPermissions = 2
	PermAddAdmin        ChannelPermissions = 4
	PermChangeChatInfo  ChannelPermissions = 8
	PermPinMessage      ChannelPermissions = 16
	PermPostMessage     ChannelPermissions = 256
	PermEditMessage     ChannelPermissions = 512
	PermDeleteMessage   ChannelPermissions = 1024
)

// ChatService implements chat/group/channel management, a port of pymax's
// api.chats.service.ChatService.
type ChatService struct {
	env     *Env
	uploads *UploadService
}

// NewChatService builds a chat service bound to env, delegating photo
// uploads to uploads.
func NewChatService(env *Env, uploads *UploadService) *ChatService {
	return &ChatService{env: env, uploads: uploads}
}

func processJoinLink(link string) string {
	const prefix = "join/"
	if idx := strings.Index(link, prefix); idx != -1 {
		return link[idx:]
	}
	return ""
}

func (s *ChatService) joinChat(ctx context.Context, link string) (*types.Chat, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatJoin, map[string]any{"link": link})
	if err != nil {
		return nil, err
	}
	chat, err := requireItemModel[types.Chat](frame, "chat")
	if err != nil {
		return nil, err
	}
	s.env.CacheChat(&chat)
	return &chat, nil
}

// CreateGroup creates a new group chat, a port of pymax's
// ChatService.create_group.
func (s *ChatService) CreateGroup(ctx context.Context, name string, participantIDs []int64, notify bool) (*types.Chat, *types.Message, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeMsgSend, map[string]any{
		"message": map[string]any{
			"cid": time.Now().UnixMilli(),
			"attaches": []map[string]any{{
				"_type":    "CONTROL",
				"event":    "new",
				"chatType": "CHAT",
				"title":    name,
				"userIds":  participantIDs,
			}},
		},
		"notify": notify,
	})
	if err != nil {
		return nil, nil, err
	}

	chat, err := decodeItemModel[types.Chat](frame, "chat")
	if err != nil || chat == nil {
		return nil, nil, err
	}
	s.env.CacheChat(chat)

	message, err := requireModel[types.Message](frame)
	if err != nil {
		return nil, nil, err
	}
	return chat, &message, nil
}

// InviteUsersToGroup invites users into a group, a port of pymax's
// ChatService.invite_users_to_group.
func (s *ChatService) InviteUsersToGroup(ctx context.Context, chatID int64, userIDs []int64, showHistory bool) (*types.Chat, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatMembersUpdate, map[string]any{
		"chatId":      chatID,
		"userIds":     userIDs,
		"showHistory": showHistory,
		"operation":   string(ChatMemberAdd),
	})
	if err != nil {
		return nil, err
	}
	chat, err := decodeItemModel[types.Chat](frame, "chat")
	if err != nil || chat == nil {
		return chat, err
	}
	s.env.CacheChat(chat)
	return chat, nil
}

// InviteUsersToChannel invites users into a channel; identical to
// InviteUsersToGroup on the wire, a port of pymax's
// ChatService.invite_users_to_channel.
func (s *ChatService) InviteUsersToChannel(ctx context.Context, chatID int64, userIDs []int64, showHistory bool) (*types.Chat, error) {
	return s.InviteUsersToGroup(ctx, chatID, userIDs, showHistory)
}

// RemoveUsersFromGroup removes users from a group, a port of pymax's
// ChatService.remove_users_from_group.
func (s *ChatService) RemoveUsersFromGroup(ctx context.Context, chatID int64, userIDs []int64, cleanMsgPeriod int) error {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatMembersUpdate, map[string]any{
		"chatId":         chatID,
		"userIds":        userIDs,
		"operation":      string(ChatMemberRemove),
		"cleanMsgPeriod": cleanMsgPeriod,
	})
	if err != nil {
		return err
	}
	if chat, _ := decodeItemModel[types.Chat](frame, "chat"); chat != nil {
		s.env.CacheChat(chat)
	}
	return nil
}

// ChangeGroupSettings updates group permission toggles, a port of pymax's
// ChatService.change_group_settings. Nil pointers leave a setting
// unchanged.
func (s *ChatService) ChangeGroupSettings(ctx context.Context, chatID int64, allCanPinMessage, onlyOwnerCanChangeIconTitle, onlyAdminCanAddMember, onlyAdminCanCall, membersCanSeePrivateLink *bool) error {
	options := map[string]any{}
	if allCanPinMessage != nil {
		options["ALL_CAN_PIN_MESSAGE"] = *allCanPinMessage
	}
	if onlyOwnerCanChangeIconTitle != nil {
		options["ONLY_OWNER_CAN_CHANGE_ICON_TITLE"] = *onlyOwnerCanChangeIconTitle
	}
	if onlyAdminCanAddMember != nil {
		options["ONLY_ADMIN_CAN_ADD_MEMBER"] = *onlyAdminCanAddMember
	}
	if onlyAdminCanCall != nil {
		options["ONLY_ADMIN_CAN_CALL"] = *onlyAdminCanCall
	}
	if membersCanSeePrivateLink != nil {
		options["MEMBERS_CAN_SEE_PRIVATE_LINK"] = *membersCanSeePrivateLink
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatUpdate, map[string]any{"chatId": chatID, "options": options})
	if err != nil {
		return err
	}
	if chat, _ := decodeItemModel[types.Chat](frame, "chat"); chat != nil {
		s.env.CacheChat(chat)
	}
	return nil
}

// ChangeGroupProfile updates a group's name/description/photo, a port of
// pymax's ChatService.change_group_profile.
func (s *ChatService) ChangeGroupProfile(ctx context.Context, chatID int64, name, description string, photo *Photo) error {
	payload := map[string]any{"chatId": chatID}
	if name != "" {
		payload["theme"] = name
	}
	if description != "" {
		payload["description"] = description
	}
	if photo != nil {
		attach, err := s.uploads.UploadPhoto(ctx, photo, false)
		if err != nil {
			return err
		}
		payload["photoToken"] = attach.PhotoToken
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatUpdate, payload)
	if err != nil {
		return err
	}
	if chat, _ := decodeItemModel[types.Chat](frame, "chat"); chat != nil {
		s.env.CacheChat(chat)
	}
	return nil
}

// JoinGroup joins a group by its invite link, a port of pymax's
// ChatService.join_group.
func (s *ChatService) JoinGroup(ctx context.Context, link string) (*types.Chat, error) {
	proceed := processJoinLink(link)
	if proceed == "" {
		return nil, fmt.Errorf("gomax: invalid group link")
	}
	return s.joinChat(ctx, proceed)
}

// JoinChannel joins a channel by its invite link, a port of pymax's
// ChatService.join_channel.
func (s *ChatService) JoinChannel(ctx context.Context, link string) (*types.Chat, error) {
	proceed := processJoinLink(link)
	if proceed == "" {
		proceed = link
	}
	return s.joinChat(ctx, proceed)
}

// ResolveGroupByLink looks up a chat by its invite link without joining, a
// port of pymax's ChatService.resolve_group_by_link.
func (s *ChatService) ResolveGroupByLink(ctx context.Context, link string) (*types.Chat, error) {
	proceed := processJoinLink(link)
	if proceed == "" {
		return nil, fmt.Errorf("gomax: invalid group link")
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeLinkInfo, map[string]any{"link": proceed})
	if err != nil {
		return nil, err
	}
	return decodeItemModel[types.Chat](frame, "chat")
}

// ReworkInviteLink reissues a group's private invite link, a port of
// pymax's ChatService.rework_invite_link.
func (s *ChatService) ReworkInviteLink(ctx context.Context, chatID int64) (*types.Chat, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatUpdate, map[string]any{
		"revokePrivateLink": true,
		"chatId":            chatID,
	})
	if err != nil {
		return nil, err
	}
	chat, err := requireItemModel[types.Chat](frame, "chat")
	if err != nil {
		return nil, err
	}
	s.env.CacheChat(&chat)
	return &chat, nil
}

// GetChats resolves chats by ID, using the local cache where possible, a
// port of pymax's ChatService.get_chats.
func (s *ChatService) GetChats(ctx context.Context, chatIDs []int64) ([]types.Chat, error) {
	result := make(map[int64]types.Chat, len(chatIDs))
	var missing []int64
	for _, id := range chatIDs {
		if chat := s.env.CachedChat(id); chat != nil {
			result[id] = *chat
		} else {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		frame, err := s.env.Invoke(ctx, protocol.OpcodeChatInfo, map[string]any{"chatIds": missing})
		if err != nil {
			return nil, err
		}
		chats, err := decodeList[types.Chat](frame, "chats")
		if err != nil {
			return nil, err
		}
		for i := range chats {
			s.env.CacheChat(&chats[i])
			result[chats[i].ID] = chats[i]
		}
	}

	out := make([]types.Chat, 0, len(chatIDs))
	for _, id := range chatIDs {
		if chat, ok := result[id]; ok {
			out = append(out, chat)
		}
	}
	return out, nil
}

// GetChat resolves a single chat by ID, a port of pymax's
// ChatService.get_chat.
func (s *ChatService) GetChat(ctx context.Context, chatID int64) (*types.Chat, error) {
	chats, err := s.GetChats(ctx, []int64{chatID})
	if err != nil {
		return nil, err
	}
	if len(chats) == 0 {
		return nil, fmt.Errorf("gomax: chat not found in response")
	}
	return &chats[0], nil
}

// GetChatMembers pages through a chat's member list, a port of pymax's
// ChatService.get_chat_members.
func (s *ChatService) GetChatMembers(ctx context.Context, chatID int64, marker int64, count int) ([]types.Member, int64, error) {
	if count == 0 {
		count = 50
	}
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatMembers, map[string]any{
		"type": "MEMBER", "chatId": chatID, "marker": marker, "count": count,
	})
	if err != nil {
		return nil, 0, err
	}
	members, err := decodeList[types.Member](frame, "members")
	if err != nil {
		return nil, 0, err
	}
	nextMarker, _ := payloadItem(frame, "marker").(float64)
	return members, int64(nextMarker), nil
}

// LeaveGroup leaves a group (also used for channels), a port of pymax's
// ChatService.leave_group / leave_channel.
func (s *ChatService) LeaveGroup(ctx context.Context, chatID int64) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeChatLeave, map[string]any{"chatId": chatID})
	if err != nil {
		return err
	}
	s.env.DropCachedChat(chatID)
	return nil
}

// FetchChats pages through the account's chat list, a port of pymax's
// ChatService.fetch_chats.
func (s *ChatService) FetchChats(ctx context.Context, marker int64) ([]types.Chat, error) {
	if marker == 0 {
		marker = time.Now().UnixMilli()
	}
	frame, err := s.env.Invoke(ctx, protocol.OpcodeChatsList, map[string]any{"marker": marker})
	if err != nil {
		return nil, err
	}
	chats, err := decodeList[types.Chat](frame, "chats")
	if err != nil {
		return nil, err
	}
	for i := range chats {
		s.env.CacheChat(&chats[i])
	}
	return chats, nil
}

// DeleteChat deletes a chat, a port of pymax's ChatService.delete_chat.
func (s *ChatService) DeleteChat(ctx context.Context, chatID int64, lastEventTime int64, forAll bool) error {
	if lastEventTime == 0 {
		lastEventTime = time.Now().UnixMilli()
	}
	_, err := s.env.Invoke(ctx, protocol.OpcodeChatDelete, map[string]any{
		"chatId": chatID, "lastEventTime": lastEventTime, "forAll": forAll,
	})
	if err != nil {
		return err
	}
	s.env.DropCachedChat(chatID)
	return nil
}

// AddAdmin grants channel admin permissions to a user, a port of pymax's
// ChatService.add_admin.
func (s *ChatService) AddAdmin(ctx context.Context, chatID, userID int64, permissions ChannelPermissions) error {
	_, err := s.env.Invoke(ctx, protocol.OpcodeChatMembersUpdate, map[string]any{
		"chatId":      chatID,
		"userIds":     []int64{userID},
		"type":        "ADMIN",
		"operation":   "add",
		"permissions": int(permissions),
	})
	return err
}
