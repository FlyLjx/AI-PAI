package generation

import (
	"context"
	"fmt"
	"strings"

	"aipi-go/internal/models"
	"aipi-go/internal/providers"
)

const maxUpstreamImageBatch = 4

type ImageRequest struct {
	TaskID                string
	Capability            string
	Operation             string
	Provider              providers.Provider
	Model                 models.Model
	Prompt                string
	SizeTier              string
	Size                  string
	Quantity              int
	OutputFormat          string
	TransparentBackground bool
	ReferenceImageURLs    []string
	MaskImageURL          string
}

func (s *Service) callImageGeneration(ctx context.Context, input ImageRequest) (any, error) {
	expectedQuantity := input.Quantity
	if expectedQuantity < 1 {
		expectedQuantity = 1
	}

	images := []ExtractedImage{}
	requestCount := 0
	maxRequests := expectedQuantity + maxUpstreamImageBatch
	if maxRequests < 3 {
		maxRequests = 3
	}

	for len(images) < expectedQuantity {
		if requestCount >= maxRequests {
			return nil, fmt.Errorf("上游实际返回 %d 张，少于请求的 %d 张", len(images), expectedQuantity)
		}
		requestCount++

		remaining := expectedQuantity - len(images)
		batchQuantity := maxUpstreamImageBatch
		if remaining < batchQuantity {
			batchQuantity = remaining
		}
		batchInput := input
		if expectedQuantity > maxUpstreamImageBatch || requestCount > 1 {
			batchInput.TaskID = fmt.Sprintf("%s#batch-%d", input.TaskID, requestCount)
		}
		batchInput.Quantity = batchQuantity
		payload, err := s.callImageJSON(ctx, batchInput, requestCount)
		if err != nil {
			return nil, err
		}
		batchImages := ExtractImages(payload)
		beforeCount := len(images)
		images = uniqueImages(append(images, batchImages...))
		if len(batchImages) < batchQuantity || len(images)-beforeCount < batchQuantity {
			s.logPartialImageBatch(input.TaskID, requestCount, batchQuantity, len(batchImages), len(images), expectedQuantity)
		}
	}
	if len(images) > expectedQuantity {
		images = images[:expectedQuantity]
	}
	return map[string]any{"data": images}, nil
}

func (s *Service) logPartialImageBatch(taskID string, batchIndex int, requested int, returned int, total int, expected int) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Warn("generation upstream image partial batch",
		"taskId", strings.TrimSpace(taskID),
		"batch", batchIndex,
		"requestedQuantity", requested,
		"returnedQuantity", returned,
		"aggregatedQuantity", total,
		"expectedQuantity", expected,
	)
}
