// Package llm defines a model-provider boundary independent of guide generation.
package llm

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Request struct {
	Messages    []Message `json:"messages"`
	Format      string    `json:"format,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	ContextSize int       `json:"context_size,omitempty"`
}
type Response struct {
	Content string `json:"content"`
}
type Provider interface {
	Complete(ctx context.Context, request Request) (Response, error)
}
