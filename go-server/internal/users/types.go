package users

import (
	"errors"
	"time"
)

var (
	ErrEmailNotVerified = errors.New("请先完成邮箱验证后创建 API Key")
	ErrInvalidCredits   = errors.New("余额必须在 0 到 99999999.9999 之间")
)

type User struct {
	ID               string
	Email            string
	InviteCode       string
	InvitedBy        string
	InvitedIP        string
	PasswordHash     string
	Credits          float64
	Role             string
	Status           string
	SyncSize         bool
	LastLoginIP      string
	LastAPIIP        string
	ActiveLast30Days bool
	EmailVerifiedAt  *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreditLog struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balanceAfter"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"createdAt"`
}

type PublicUser struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Credits          float64 `json:"credits"`
	InviteCode       string  `json:"inviteCode"`
	Role             string  `json:"role"`
	Status           string  `json:"status"`
	SyncSize         bool    `json:"syncSize"`
	LastLoginIP      string  `json:"lastLoginIp,omitempty"`
	LastAPIIP        string  `json:"lastApiIp,omitempty"`
	ActiveLast30Days bool    `json:"activeLast30Days,omitempty"`
	EmailVerifiedAt  *string `json:"emailVerifiedAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	Subscription     any     `json:"subscription"`
}

type AdminUserPageInput struct {
	Keyword  string
	Status   string
	Billing  string
	Activity string
	Page     int
	PageSize int
}

type AdminUserPageStats struct {
	Total            int `json:"total"`
	Active           int `json:"active"`
	Verified         int `json:"verified"`
	Subscribed       int `json:"subscribed"`
	ActiveLast30Days int `json:"activeLast30Days"`
}

func RequireEmailVerifiedForAPIKey(user *User) error {
	if user == nil || user.EmailVerifiedAt == nil {
		return ErrEmailNotVerified
	}
	return nil
}

func ToPublicUser(user *User) PublicUser {
	var verifiedAt *string
	if user.EmailVerifiedAt != nil {
		value := user.EmailVerifiedAt.Format(time.RFC3339)
		verifiedAt = &value
	}
	return PublicUser{
		ID:               user.ID,
		Email:            user.Email,
		Credits:          user.Credits,
		InviteCode:       user.InviteCode,
		Role:             user.Role,
		Status:           user.Status,
		SyncSize:         user.SyncSize,
		LastLoginIP:      user.LastLoginIP,
		LastAPIIP:        user.LastAPIIP,
		ActiveLast30Days: user.ActiveLast30Days,
		EmailVerifiedAt:  verifiedAt,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
		Subscription:     nil,
	}
}
