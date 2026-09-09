package api

import (
	"context"
	"fmt"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// ContactAction is add-vs-remove for CONTACT_UPDATE requests, a port of
// pymax's api.users.enums.ContactAction.
type ContactAction string

const (
	ContactActionAdd    ContactAction = "ADD"
	ContactActionRemove ContactAction = "REMOVE"
)

// UserService implements contact/profile lookups, a port of pymax's
// api.users.service.UserService.
type UserService struct{ env *Env }

// NewUserService builds a user service bound to env.
func NewUserService(env *Env) *UserService { return &UserService{env: env} }

// GetCachedUser returns a previously fetched user without a round-trip, a
// port of pymax's UserService.get_cached_user.
func (s *UserService) GetCachedUser(userID int64) *types.User {
	return s.env.CachedUser(userID)
}

// GetUsers resolves users by ID, using the local cache where possible, a
// port of pymax's UserService.get_users.
func (s *UserService) GetUsers(ctx context.Context, userIDs []int64) ([]types.User, error) {
	result := make(map[int64]types.User, len(userIDs))
	var missing []int64
	for _, id := range userIDs {
		if u := s.env.CachedUser(id); u != nil {
			result[id] = *u
		} else {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		users, err := s.FetchUsers(ctx, missing)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			result[u.ID] = u
		}
	}

	out := make([]types.User, 0, len(userIDs))
	for _, id := range userIDs {
		if u, ok := result[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

// GetUser resolves a single user by ID, a port of pymax's
// UserService.get_user.
func (s *UserService) GetUser(ctx context.Context, userID int64) (*types.User, error) {
	if u := s.env.CachedUser(userID); u != nil {
		return u, nil
	}
	users, err := s.FetchUsers(ctx, []int64{userID})
	if err != nil || len(users) == 0 {
		return nil, err
	}
	return &users[0], nil
}

// FetchUsers fetches users by ID from the server, bypassing the cache, a
// port of pymax's UserService.fetch_users.
func (s *UserService) FetchUsers(ctx context.Context, userIDs []int64) ([]types.User, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeContactInfo, map[string]any{"contactIds": userIDs})
	if err != nil {
		return nil, err
	}
	users, err := decodeList[types.User](frame, "contacts")
	if err != nil {
		return nil, err
	}
	for i := range users {
		s.env.CacheUser(&users[i])
	}
	return users, nil
}

// SearchByPhone looks up a user by phone number, a port of pymax's
// UserService.search_by_phone.
func (s *UserService) SearchByPhone(ctx context.Context, phone string) (*types.User, error) {
	frame, err := s.env.Invoke(ctx, protocol.OpcodeContactInfoByPhone, map[string]any{"phone": phone})
	if err != nil {
		return nil, err
	}
	user, err := requireItemModel[types.User](frame, "contact")
	if err != nil {
		return nil, err
	}
	s.env.CacheUser(&user)
	return &user, nil
}

func (s *UserService) contactAction(ctx context.Context, contactID int64, action ContactAction) (protocol.InboundFrame, error) {
	return s.env.Invoke(ctx, protocol.OpcodeContactUpdate, map[string]any{
		"contactId": contactID,
		"action":    string(action),
	})
}

// AddContact adds a user to the account's contacts, a port of pymax's
// UserService.add_contact.
func (s *UserService) AddContact(ctx context.Context, contactID int64) (*types.User, error) {
	frame, err := s.contactAction(ctx, contactID, ContactActionAdd)
	if err != nil {
		return nil, err
	}
	user, err := requireItemModel[types.User](frame, "contact")
	if err != nil {
		return nil, err
	}
	s.env.CacheUser(&user)
	return &user, nil
}

// RemoveContact removes a user from the account's contacts, a port of
// pymax's UserService.remove_contact.
func (s *UserService) RemoveContact(ctx context.Context, contactID int64) error {
	if _, err := s.contactAction(ctx, contactID, ContactActionRemove); err != nil {
		return err
	}
	s.env.DropCachedUser(contactID)
	return nil
}

// ImportContacts uploads phone-book contacts and returns any matched Max
// users, a port of pymax's UserService.import_contacts.
func (s *UserService) ImportContacts(ctx context.Context, contacts []types.ContactInfo) ([]types.User, error) {
	contactList := make(map[string]map[string]string, len(contacts))
	for _, c := range contacts {
		contactList[c.Phone] = map[string]string{"firstName": c.FirstName}
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeSync, map[string]any{"contactList": contactList})
	if err != nil {
		return nil, err
	}
	users, err := decodeList[types.User](frame, "contacts")
	if err != nil {
		return nil, err
	}
	for i := range users {
		s.env.CacheUser(&users[i])
	}
	return users, nil
}

// GetChatID returns the dialog chat ID between two users, a port of
// pymax's UserService.get_chat_id.
func (s *UserService) GetChatID(firstUserID, secondUserID int64) int64 {
	return firstUserID ^ secondUserID
}

// ChangeProfile updates the account's own name/description/photo, a port
// of pymax's api.self.service.SelfService.change_profile.
func (s *UserService) ChangeProfile(ctx context.Context, firstName, lastName, description string, photo *Photo, uploads *UploadService) (*types.Profile, error) {
	payload := map[string]any{"firstName": firstName, "avatarType": "USER_AVATAR"}
	if lastName != "" {
		payload["lastName"] = lastName
	}
	if description != "" {
		payload["description"] = description
	}
	if photo != nil {
		if uploads == nil {
			return nil, fmt.Errorf("gomax: photo provided without an upload service")
		}
		attach, err := uploads.UploadPhoto(ctx, photo, true)
		if err != nil {
			return nil, err
		}
		payload["photoToken"] = attach.PhotoToken
	}

	frame, err := s.env.Invoke(ctx, protocol.OpcodeProfile, payload)
	if err != nil {
		return nil, err
	}
	profile, err := requireItemModel[types.Profile](frame, "profile")
	if err != nil {
		return nil, err
	}
	s.env.SetMe(&profile)
	s.env.CacheUser(&profile.Contact)
	return &profile, nil
}

// SetPresence toggles whether subsequent requests (notably the ping loop)
// report the account as online/interactive, a port of pymax's
// api.self.service.SelfService.set_presence.
func (s *UserService) SetPresence(online bool) {
	s.env.SetInteractive(online)
}
