package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type responseClient struct {
	body string
}

func (client responseClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(client.body)),
	}, nil
}

func TestOllamaCapturesSeparateEvaluationDurations(t *testing.T) {
	client := responseClient{body: `{
			"model":"test-model",
			"message":{"role":"assistant","content":"{\"title\":\"Guide\"}"},
			"done":true,
			"prompt_eval_count":120,
			"eval_count":30,
			"total_duration":9000000000,
			"prompt_eval_duration":2000000000,
			"eval_duration":3000000000
		}`}

	response, err := NewOllama(client, "http://ollama.test", "test-model").Complete(context.Background(), Request{
		Messages: []Message{{Role: "user", Content: "Generate"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.PromptTokens != 120 || response.OutputTokens != 30 {
		t.Fatalf("unexpected token counts: %#v", response)
	}
	if response.DurationNanos != 9_000_000_000 || response.PromptDurationNanos != 2_000_000_000 || response.OutputDurationNanos != 3_000_000_000 {
		t.Fatalf("unexpected durations: %#v", response)
	}
}
