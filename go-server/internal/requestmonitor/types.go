package requestmonitor

import (
	"encoding/json"
	"time"
)

type Record struct {
	ID            string
	Method        string
	Path          string
	QueryParams   json.RawMessage
	BodyParams    json.RawMessage
	SourceIP      string
	SourceHost    string
	Origin        string
	Referer       string
	UserAgent     string
	StatusCode    int
	DurationMS    int64
	ResponseBytes int64
	CreatedAt     time.Time
}

type Filters struct {
	Range    string
	Keyword  string
	Method   string
	Status   string
	Page     int
	PageSize int
	Now      time.Time
}

type Summary struct {
	Total             int64   `json:"total"`
	Successful        int64   `json:"successful"`
	ClientErrors      int64   `json:"clientErrors"`
	ServerErrors      int64   `json:"serverErrors"`
	ErrorRate         float64 `json:"errorRate"`
	AverageDurationMS float64 `json:"averageDurationMs"`
	UniqueSources     int64   `json:"uniqueSources"`
}

type TrendPoint struct {
	Time       string `json:"time"`
	Total      int64  `json:"total"`
	Successful int64  `json:"successful"`
	Errors     int64  `json:"errors"`
}

type FrequencyItem struct {
	Name              string  `json:"name"`
	Count             int64   `json:"count"`
	Errors            int64   `json:"errors"`
	AverageDurationMS float64 `json:"averageDurationMs"`
}

type Log struct {
	ID            string `json:"id"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	QueryParams   any    `json:"queryParams"`
	BodyParams    any    `json:"bodyParams"`
	SourceIP      string `json:"sourceIp"`
	SourceHost    string `json:"sourceHost"`
	Origin        string `json:"origin"`
	Referer       string `json:"referer"`
	UserAgent     string `json:"userAgent"`
	StatusCode    int    `json:"statusCode"`
	DurationMS    int64  `json:"durationMs"`
	ResponseBytes int64  `json:"responseBytes"`
	CreatedAt     string `json:"createdAt"`
}

type Snapshot struct {
	Range        string          `json:"range"`
	Summary      Summary         `json:"summary"`
	Trend        []TrendPoint    `json:"trend"`
	TopEndpoints []FrequencyItem `json:"topEndpoints"`
	TopSources   []FrequencyItem `json:"topSources"`
	Items        []Log           `json:"items"`
}
