package tasks

import "time"

type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusPending    Status = "pending"
	StatusSuccess    Status = "success"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

type Task struct {
	ID                    string
	UserID                string
	ModelID               string
	ProviderID            string
	Capability            string
	Prompt                string
	ReferenceImageURL     *string
	SizeTier              string
	Size                  *string
	OutputFormat          string
	TransparentBackground bool
	Quantity              int
	UserIP                string
	CostCredits           float64
	ModelCostCredits      float64
	RemainingCredits      float64
	DurationSeconds       float64
	Status                Status
	ErrorMessage          *string
	ResultJSON            any
	FavoriteEnabled       bool
	PublicStatus          string
	DisplayEnabled        bool
	DisplayNote           *string
	PublicRequestedAt     *time.Time
	PublicReviewedAt      *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	UserEmail             *string
	ModelName             *string
	ModelDisplayName      *string
	ProviderName          *string
	ProviderBaseURL       *string
}

type PublicTask struct {
	ID                    string   `json:"id"`
	UserID                string   `json:"userId"`
	ModelID               string   `json:"modelId"`
	ProviderID            string   `json:"providerId"`
	Capability            string   `json:"capability"`
	Prompt                string   `json:"prompt"`
	ReferenceImageURL     *string  `json:"referenceImageUrl"`
	SizeTier              string   `json:"sizeTier"`
	Size                  *string  `json:"size"`
	OutputFormat          string   `json:"outputFormat"`
	TransparentBackground bool     `json:"transparentBackground"`
	Quantity              int      `json:"quantity"`
	UserIP                string   `json:"userIp"`
	CostCredits           float64  `json:"-"`
	ModelCostCredits      float64  `json:"-"`
	RemainingCredits      float64  `json:"-"`
	DurationSeconds       float64  `json:"durationSeconds"`
	Status                Status   `json:"status"`
	ErrorMessage          *string  `json:"errorMessage"`
	ResultJSON            any      `json:"resultJson,omitempty"`
	ResultURL             *string  `json:"resultUrl"`
	ResultURLs            []string `json:"resultUrls"`
	DirectResultURL       *string  `json:"directResultUrl"`
	DirectResultURLs      []string `json:"directResultUrls"`
	ThumbnailURL          *string  `json:"thumbnailUrl"`
	ThumbnailURLs         []string `json:"thumbnailUrls"`
	FavoriteEnabled       bool     `json:"favoriteEnabled"`
	PublicStatus          string   `json:"publicStatus"`
	DisplayEnabled        bool     `json:"displayEnabled"`
	DisplayNote           *string  `json:"displayNote"`
	PublicRequestedAt     *string  `json:"publicRequestedAt"`
	PublicReviewedAt      *string  `json:"publicReviewedAt"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
	UserEmail             *string  `json:"userEmail,omitempty"`
	ModelName             *string  `json:"modelName,omitempty"`
	ModelDisplayName      *string  `json:"modelDisplayName,omitempty"`
	ProviderName          *string  `json:"providerName,omitempty"`
}

type AdminTaskListItem struct {
	ID                       string  `json:"id"`
	UserID                   string  `json:"userId"`
	UserEmail                *string `json:"userEmail,omitempty"`
	ModelID                  string  `json:"modelId"`
	ModelName                *string `json:"modelName,omitempty"`
	ModelDisplayName         *string `json:"modelDisplayName,omitempty"`
	SizeTier                 string  `json:"sizeTier"`
	Size                     *string `json:"size"`
	Quantity                 int     `json:"quantity"`
	UserIP                   string  `json:"userIp"`
	CostCredits              float64 `json:"-"`
	DurationSeconds          float64 `json:"durationSeconds"`
	Status                   Status  `json:"status"`
	ErrorMessage             *string `json:"errorMessage"`
	CreatedAt                string  `json:"createdAt"`
	UserSubscriptionPlanName *string `json:"userSubscriptionPlanName,omitempty"`
}

func ToPublic(task *Task) PublicTask {
	directResultURLs := []string{}
	resultURLs := []string{}
	thumbnailURLs := []string{}
	if task.Status == StatusSuccess {
		directResultURLs = ResultURLs(task.ResultJSON)
		for index, value := range directResultURLs {
			directResultURLs[index] = RewriteImageURL(task.ProviderBaseURL, value)
		}
		for index := range directResultURLs {
			resultURLs = append(resultURLs, "/api/tasks/"+task.ID+"/images/"+itoa(index))
			thumbnailURLs = append(thumbnailURLs, "/api/tasks/"+task.ID+"/thumbnails/"+itoa(index))
		}
	}
	var resultURL *string
	var directResultURL *string
	var thumbnailURL *string
	if len(resultURLs) > 0 {
		resultURL = &resultURLs[0]
		thumbnailURL = &thumbnailURLs[0]
	}
	if len(directResultURLs) > 0 {
		directResultURL = &directResultURLs[0]
	}
	displayQuantity := task.Quantity
	if task.Status == StatusSuccess && len(directResultURLs) > 0 {
		displayQuantity = len(directResultURLs)
	}
	publicRequestedAt := formatOptionalTime(task.PublicRequestedAt)
	publicReviewedAt := formatOptionalTime(task.PublicReviewedAt)
	return PublicTask{
		ID:                    task.ID,
		UserID:                task.UserID,
		ModelID:               task.ModelID,
		ProviderID:            task.ProviderID,
		Capability:            task.Capability,
		Prompt:                task.Prompt,
		ReferenceImageURL:     task.ReferenceImageURL,
		SizeTier:              task.SizeTier,
		Size:                  task.Size,
		OutputFormat:          task.OutputFormat,
		TransparentBackground: task.TransparentBackground,
		Quantity:              displayQuantity,
		UserIP:                task.UserIP,
		CostCredits:           task.CostCredits,
		ModelCostCredits:      task.ModelCostCredits,
		RemainingCredits:      task.RemainingCredits,
		DurationSeconds:       task.DurationSeconds,
		Status:                task.Status,
		ErrorMessage:          task.ErrorMessage,
		ResultJSON:            nil,
		ResultURL:             resultURL,
		ResultURLs:            resultURLs,
		DirectResultURL:       directResultURL,
		DirectResultURLs:      directResultURLs,
		ThumbnailURL:          thumbnailURL,
		ThumbnailURLs:         thumbnailURLs,
		FavoriteEnabled:       task.FavoriteEnabled,
		PublicStatus:          task.PublicStatus,
		DisplayEnabled:        task.DisplayEnabled,
		DisplayNote:           task.DisplayNote,
		PublicRequestedAt:     publicRequestedAt,
		PublicReviewedAt:      publicReviewedAt,
		CreatedAt:             task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             task.UpdatedAt.Format(time.RFC3339),
		UserEmail:             task.UserEmail,
		ModelName:             task.ModelName,
		ModelDisplayName:      task.ModelDisplayName,
		ProviderName:          task.ProviderName,
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format(time.RFC3339)
	return &text
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
