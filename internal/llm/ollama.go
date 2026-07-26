package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}
type Ollama struct {
	client         HTTPClient
	baseURL, model string
}

func NewOllama(client HTTPClient, baseURL, model string) *Ollama {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &Ollama{client: client, baseURL: strings.TrimRight(baseURL, "/"), model: model}
}
func (o *Ollama) Complete(ctx context.Context, request Request) (Response, error) {
	body, err := json.Marshal(struct {
		Model    string             `json:"model"`
		Messages []Message          `json:"messages"`
		Stream   bool               `json:"stream"`
		Think    bool               `json:"think"`
		Format   string             `json:"format,omitempty"`
		Options  map[string]float64 `json:"options,omitempty"`
	}{o.model, request.Messages, false, false, request.Format, map[string]float64{"temperature": request.Temperature, "num_predict": float64(request.MaxTokens), "num_ctx": float64(request.ContextSize)}})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("call Ollama: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode/100 != 2 {
		return Response{}, fmt.Errorf("Ollama returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var result struct {
		Model              string  `json:"model"`
		Message            Message `json:"message"`
		Done               bool    `json:"done"`
		DoneReason         string  `json:"done_reason"`
		EvalCount          int     `json:"eval_count"`
		PromptEvalCount    int     `json:"prompt_eval_count"`
		TotalDuration      int64   `json:"total_duration"`
		PromptEvalDuration int64   `json:"prompt_eval_duration"`
		EvalDuration       int64   `json:"eval_duration"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return Response{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	content := strings.TrimSpace(result.Message.Content)
	if content == "" {
		return Response{}, fmt.Errorf("Ollama returned empty content (done=%t, reason=%q, generated_tokens=%d); check that the selected model supports chat and structured output", result.Done, result.DoneReason, result.EvalCount)
	}
	if result.DoneReason == "length" {
		return Response{}, fmt.Errorf("Ollama exhausted its context after %d prompt tokens and %d generated tokens; increase ollama.context_window or compile a shorter source", result.PromptEvalCount, result.EvalCount)
	}
	return Response{
		Content:             content,
		Model:               result.Model,
		PromptTokens:        result.PromptEvalCount,
		OutputTokens:        result.EvalCount,
		DurationNanos:       result.TotalDuration,
		PromptDurationNanos: result.PromptEvalDuration,
		OutputDurationNanos: result.EvalDuration,
	}, nil
}
