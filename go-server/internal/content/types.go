package content

import (
	"errors"
	"time"
)

var (
	ErrAnnouncementNotFound    = errors.New("announcement not found")
	ErrAnnouncementNotEligible = errors.New("announcement is not eligible for this user")
	ErrAnnouncementInactive    = errors.New("announcement is inactive")
	ErrAnnouncementNoReward    = errors.New("announcement has no reward")
	ErrAnnouncementBalanceCap  = errors.New("announcement reward exceeds balance limit")
)

type Announcement struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	DisplayMode       string   `json:"displayMode"`
	TargetType        string   `json:"targetType"`
	Status            string   `json:"status"`
	SortOrder         int      `json:"sortOrder"`
	RewardCredits     float64  `json:"rewardCredits"`
	RewardClaimed     bool     `json:"rewardClaimed"`
	UserIDs           []string `json:"userIds"`
	TargetCount       *int     `json:"targetCount,omitempty"`
	ReadCount         *int     `json:"readCount,omitempty"`
	UnreadCount       *int     `json:"unreadCount,omitempty"`
	ReadRate          *float64 `json:"readRate,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
	createdAtInternal time.Time
	updatedAtInternal time.Time
}

type RewardClaimResult struct {
	RewardCredits float64 `json:"rewardCredits"`
	Granted       bool    `json:"granted"`
	BalanceAfter  float64 `json:"balanceAfter"`
}
