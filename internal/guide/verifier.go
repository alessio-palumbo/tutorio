package guide

import (
	"context"
	"fmt"
	"strings"
)

type Verifier interface {
	Verify(ctx context.Context, value Guide) error
}
type StructuralVerifier struct{}

func NewStructuralVerifier() StructuralVerifier { return StructuralVerifier{} }
func (StructuralVerifier) Verify(ctx context.Context, g Guide) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var missing []string
	if strings.TrimSpace(g.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(g.Overview) == "" {
		missing = append(missing, "overview")
	}
	if strings.TrimSpace(g.FinalOutcome) == "" {
		missing = append(missing, "final outcome")
	}
	if len(g.Steps) == 0 {
		missing = append(missing, "steps")
	}
	if len(missing) > 0 {
		return fmt.Errorf("guide verification failed: missing %s", strings.Join(missing, ", "))
	}
	for i, step := range g.Steps {
		if step.Number == 0 {
			g.Steps[i].Number = i + 1
		}
		if strings.TrimSpace(step.Title) == "" || strings.TrimSpace(step.Explanation) == "" {
			return fmt.Errorf("guide verification failed: step %d is incomplete", i+1)
		}
	}
	return nil
}
