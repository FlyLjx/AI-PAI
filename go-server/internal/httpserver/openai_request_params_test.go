package httpserver

import (
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestCompatRequestParamsSummarizesInlineImages(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/images/edits", nil)
	params := compatRequestParams(req, compatImageInput{
		Model:          "image-model",
		Prompt:         "replace the background",
		N:              1,
		ResponseFormat: "b64_json",
		ReferenceItems: []any{"https://example.com/reference.png"},
		Image:          "data:image/png;base64,abcdefghijklmnopqrstuvwxyz",
	})

	image, ok := params["image"].(map[string]any)
	if !ok || image["type"] != "inline_image" || image["mediaType"] != "image/png" {
		t.Fatalf("inline image was not summarized: %#v", params["image"])
	}
	if params["prompt"] != "replace the background" || params["response_format"] != "b64_json" {
		t.Fatalf("request parameters are incomplete: %#v", params)
	}
	references, ok := params["referenceImages"].([]any)
	if !ok || len(references) != 1 || references[0] != "https://example.com/reference.png" {
		t.Fatalf("reference image aliases were not captured: %#v", params["referenceImages"])
	}
}

func TestCompatRequestParamsRecordsMultipartMetadata(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/images/edits", nil)
	req.MultipartForm = &multipart.Form{File: map[string][]*multipart.FileHeader{
		"image": {{Filename: `C:\uploads\source.png`, Size: 2048}},
	}}
	params := compatRequestParams(req, compatImageInput{Model: "image-model", Prompt: "edit", N: 1})

	image, ok := params["image"].(map[string]any)
	if !ok || image["fileName"] != "source.png" || image["sizeBytes"] != int64(2048) {
		t.Fatalf("multipart metadata is incomplete: %#v", params["image"])
	}
}
