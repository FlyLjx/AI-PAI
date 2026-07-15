package users

import "time"

type User struct {
	ID              string
	Email           string
	InviteCode      string
	InvitedBy       string
	InvitedIP       string
	PasswordHash    string
	Credits         float64
	Role            string
	Status          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PublicUser struct {
	ID              string  `json:"id"`
	Email           string  `json:"email"`
	Credits         float64 `json:"credits"`
	InviteCode      string  `json:"inviteCode"`
	Role            string  `json:"role"`
	Status          string  `json:"status"`
	EmailVerifiedAt *string `json:"emailVerifiedAt"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	Subscription    any     `json:"subscription"`
}

func ToPublicUser(user *User) PublicUser {
	var verifiedAt *string
	if user.EmailVerifiedAt != nil {
		value := user.EmailVerifiedAt.Format(time.RFC3339)
		verifiedAt = &value
	}
	return PublicUser{
		ID:              user.ID,
		Email:           user.Email,
		Credits:         user.Credits,
		InviteCode:      user.InviteCode,
		Role:            user.Role,
		Status:          user.Status,
		EmailVerifiedAt: verifiedAt,
		CreatedAt:       user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       user.UpdatedAt.Format(time.RFC3339),
		Subscription:    nil,
	}
}
