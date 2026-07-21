// Package exporter defines guide output boundaries.
package exporter

import (
	"context"
	"github.com/alessio/tutorio/internal/guide"
)

type Exporter interface {
	Extension() string
	Render(ctx context.Context, value guide.Guide) ([]byte, error)
}
