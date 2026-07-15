package models

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID                 string
	ProviderID         string
	ProviderName       *string
	ProviderType       *string
	ProviderStatus     *string
	ModelName          string
	DisplayName        string
	Capability         string
	Cost1K             float64
	Cost2K             float64
	Cost4K             float64
	MarkupPercent      float64
	PriceChangePercent float64
	Price1K            float64
	Price2K            float64
	Price4K            float64
	AppendSizeToPrompt bool
	EnabledSizeTiers   []string
	SortOrder          int
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PublicModel struct {
	ID                 string   `json:"id"`
	ProviderID         string   `json:"providerId"`
	ProviderName       *string  `json:"providerName,omitempty"`
	ProviderType       *string  `json:"providerType,omitempty"`
	ProviderStatus     *string  `json:"providerStatus,omitempty"`
	ModelName          string   `json:"modelName"`
	DisplayName        string   `json:"displayName"`
	Capability         string   `json:"capability"`
	Cost1K             float64  `json:"cost1k"`
	Cost2K             float64  `json:"cost2k"`
	Cost4K             float64  `json:"cost4k"`
	MarkupPercent      float64  `json:"markupPercent"`
	PriceChangePercent float64  `json:"priceChangePercent"`
	Price1K            float64  `json:"price1k"`
	Price2K            float64  `json:"price2k"`
	Price4K            float64  `json:"price4k"`
	AppendSizeToPrompt bool     `json:"appendSizeToPrompt"`
	EnabledSizeTiers   []string `json:"enabledSizeTiers"`
	SortOrder          int      `json:"sortOrder"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

func ToPublic(model Model) PublicModel {
	return PublicModel{
		ID:                 model.ID,
		ProviderID:         model.ProviderID,
		ProviderName:       model.ProviderName,
		ProviderType:       model.ProviderType,
		ProviderStatus:     model.ProviderStatus,
		ModelName:          model.ModelName,
		DisplayName:        model.DisplayName,
		Capability:         model.Capability,
		Cost1K:             model.Cost1K,
		Cost2K:             model.Cost2K,
		Cost4K:             model.Cost4K,
		MarkupPercent:      model.MarkupPercent,
		PriceChangePercent: model.PriceChangePercent,
		Price1K:            model.Price1K,
		Price2K:            model.Price2K,
		Price4K:            model.Price4K,
		AppendSizeToPrompt: model.AppendSizeToPrompt,
		EnabledSizeTiers:   model.EnabledSizeTiers,
		SortOrder:          model.SortOrder,
		Status:             model.Status,
		CreatedAt:          model.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          model.UpdatedAt.Format(time.RFC3339),
	}
}

func ParseEnabledSizeTiers(raw []byte) []string {
	defaults := []string{"1k", "2k", "4k"}
	if len(raw) == 0 {
		return defaults
	}
	var tiers []string
	if err := json.Unmarshal(raw, &tiers); err != nil || len(tiers) == 0 {
		return defaults
	}
	allowed := map[string]bool{"1k": true, "2k": true, "4k": true}
	seen := map[string]bool{}
	result := []string{}
	for _, tier := range tiers {
		if allowed[tier] && !seen[tier] {
			result = append(result, tier)
			seen[tier] = true
		}
	}
	if len(result) == 0 {
		return defaults
	}
	return result
}

func ParseEnabledSizeTiersFromStrings(tiers []string) []string {
	if len(tiers) == 0 {
		return []string{"1k", "2k", "4k"}
	}
	allowed := map[string]bool{"1k": true, "2k": true, "4k": true}
	seen := map[string]bool{}
	result := []string{}
	for _, tier := range tiers {
		if allowed[tier] && !seen[tier] {
			result = append(result, tier)
			seen[tier] = true
		}
	}
	if len(result) == 0 {
		return []string{"1k", "2k", "4k"}
	}
	return result
}
