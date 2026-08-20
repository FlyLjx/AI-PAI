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
	ID                     string
	UserID                 string
	ModelID                string
	ProviderID             string
	Capability             string
	Prompt                 string
	ReferenceImageURL      *string
	SizeTier               string
	Size                   *string
	OutputFormat           string
	SyncSize               bool
	TransparentBackground  bool
	Quantity               int
	SubscriptionQuotaUnits int
	UserIP                 string
	CostCredits            float64
	ModelCostCredits       float64
	RemainingCredits       float64
	DurationSeconds        float64
	Status                 Status
	ErrorMessage           *string
	ResultJSON             any
	StoredResultURLs       []string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	UserEmail              *string
	ModelName              *string
	ModelDisplayName       *string
	ProviderName           *string
	ProviderBaseURL        *string
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
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
	UserEmail             *string  `json:"userEmail,omitempty"`
	ModelName             *string  `json:"modelName,omitempty"`
	ModelDisplayName      *string  `json:"modelDisplayName,omitempty"`
	ProviderName          *string  `json:"providerName,omitempty"`
}

// TaskHistoryItem is the lightweight projection used by user task history.
// Result payloads and image locations belong to task detail responses only.
type TaskHistoryItem struct {
	ID                    string  `json:"id"`
	ModelID               string  `json:"modelId"`
	ProviderID            string  `json:"providerId"`
	Capability            string  `json:"capability"`
	Prompt                string  `json:"prompt"`
	SizeTier              string  `json:"sizeTier"`
	Size                  *string `json:"size"`
	OutputFormat          string  `json:"outputFormat"`
	TransparentBackground bool    `json:"transparentBackground"`
	Quantity              int     `json:"quantity"`
	UserIP                string  `json:"userIp"`
	DurationSeconds       float64 `json:"durationSeconds"`
	Status                Status  `json:"status"`
	ErrorMessage          *string `json:"errorMessage"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	ModelName             *string `json:"modelName,omitempty"`
	ModelDisplayName      *string `json:"modelDisplayName,omitempty"`
	ProviderName          *string `json:"providerName,omitempty"`
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
	if task.Status == StatusSuccess {
		directResultURLs = append(directResultURLs, task.StoredResultURLs...)
		if len(directResultURLs) == 0 {
			directResultURLs = ResultURLs(task.ResultJSON)
		}
		for index, value := range directResultURLs {
			directResultURLs[index] = RewriteImageURL(task.ProviderBaseURL, value)
		}
	}
	var resultURL *string
	var directResultURL *string
	if len(directResultURLs) > 0 {
		resultURL = &directResultURLs[0]
		directResultURL = &directResultURLs[0]
	}
	displayQuantity := task.Quantity
	if task.Status == StatusSuccess && len(directResultURLs) > 0 {
		displayQuantity = len(directResultURLs)
	}
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
		ResultURLs:            directResultURLs,
		DirectResultURL:       directResultURL,
		DirectResultURLs:      directResultURLs,
		ThumbnailURL:          resultURL,
		ThumbnailURLs:         directResultURLs,
		CreatedAt:             task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             task.UpdatedAt.Format(time.RFC3339),
		UserEmail:             task.UserEmail,
		ModelName:             task.ModelName,
		ModelDisplayName:      task.ModelDisplayName,
		ProviderName:          task.ProviderName,
	}
}

func ToHistory(task *Task) TaskHistoryItem {
	return TaskHistoryItem{
		ID:                    task.ID,
		ModelID:               task.ModelID,
		ProviderID:            task.ProviderID,
		Capability:            task.Capability,
		Prompt:                task.Prompt,
		SizeTier:              task.SizeTier,
		Size:                  task.Size,
		OutputFormat:          task.OutputFormat,
		TransparentBackground: task.TransparentBackground,
		Quantity:              task.Quantity,
		UserIP:                task.UserIP,
		DurationSeconds:       task.DurationSeconds,
		Status:                task.Status,
		ErrorMessage:          task.ErrorMessage,
		CreatedAt:             task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             task.UpdatedAt.Format(time.RFC3339),
		ModelName:             task.ModelName,
		ModelDisplayName:      task.ModelDisplayName,
		ProviderName:          task.ProviderName,
	}
}
