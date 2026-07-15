package content

import "time"

type Announcement struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	DisplayMode       string   `json:"displayMode"`
	TargetType        string   `json:"targetType"`
	Status            string   `json:"status"`
	SortOrder         int      `json:"sortOrder"`
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
