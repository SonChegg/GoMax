package types

// Name is one name entry of a user (Max supports multiple named variants,
// e.g. legal vs. display name), a port of pymax's types.domain.name.Name.
type Name struct {
	Name      *string `json:"name,omitempty"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	NameType  *string `json:"type,omitempty"`
}

// Presence is a user's online/last-seen state, a port of pymax's
// types.domain.presence.Presence.
type Presence struct {
	Seen   *int64 `json:"seen,omitempty"`
	Status *int   `json:"status,omitempty"`
}

// ContactInfo is a phone-book contact used with UserService.ImportContacts,
// a port of pymax's types.domain.user.ContactInfo.
type ContactInfo struct {
	Phone     string  `json:"phone"`
	FirstName string  `json:"firstName"`
	LastName  *string `json:"lastName,omitempty"`
}

// User is a Max contact/user, a port of pymax's types.domain.user.User.
//
// NOTE: see the NOTE on Message regarding pymax's auto-bound convenience
// methods (add_contact/remove_contact/...); gomax exposes the equivalent
// operations as explicit api.UserService methods instead.
type User struct {
	ID               int64          `json:"id"`
	AccountStatus    *int           `json:"accountStatus,omitempty"`
	RegistrationTime *int64         `json:"registrationTime,omitempty"`
	Country          *string        `json:"country,omitempty"`
	BaseRawURL       *string        `json:"baseRawUrl,omitempty"`
	BaseURL          *string        `json:"baseUrl,omitempty"`
	Names            []Name         `json:"names,omitempty"`
	Options          []string       `json:"options,omitempty"`
	PhotoID          *int64         `json:"photoId,omitempty"`
	UpdateTime       *int64         `json:"updateTime,omitempty"`
	Phone            *int64         `json:"phone,omitempty"`
	Status           *string        `json:"status,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Gender           any            `json:"gender,omitempty"`
	Link             any            `json:"link,omitempty"`
	WebApp           any            `json:"webApp,omitempty"`
	MenuButton       map[string]any `json:"menuButton,omitempty"`
}

// Member is a user found in a chat's member/join-request list, a port of
// pymax's types.domain.member.Member.
type Member struct {
	Contact  User     `json:"contact"`
	Presence Presence `json:"presence"`
}

// Profile is the current account's profile, a port of pymax's
// types.domain.profile.Profile.
type Profile struct {
	Contact        User  `json:"contact"`
	ProfileOptions []int `json:"profileOptions,omitempty"`
}
