package pdf

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/alessio/tutorio/internal/evidence"
)

type PageRenderer struct {
	mu     sync.RWMutex
	binary string
	runner CommandRunner
}

func NewPageRenderer(binary string, runner CommandRunner) *PageRenderer {
	if strings.TrimSpace(binary) == "" {
		binary = "pdftocairo"
	}
	return &PageRenderer{binary: binary, runner: runner}
}

func (r *PageRenderer) SetBinary(binary string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.binary = strings.TrimSpace(binary)
}

func (r *PageRenderer) binaryPath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.binary
}

func (r *PageRenderer) Render(ctx context.Context, source evidence.Source, physicalPage int) (evidence.Visual, error) {
	if source.Kind != "pdf" {
		return evidence.Visual{}, fmt.Errorf("visual rendering is not supported for source kind %q", source.Kind)
	}
	if physicalPage < 1 {
		return evidence.Visual{}, fmt.Errorf("physical PDF page must be positive")
	}
	page := strconv.Itoa(physicalPage)
	binary := r.binaryPath()
	output, err := r.runner.Run(ctx, binary, "-png", "-singlefile", "-f", page, "-l", page, "-scale-to", "2000", source.Locator, "-")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return evidence.Visual{}, fmt.Errorf("%s was not found; install Poppler or configure tools.pdftocairo_path: %w", binary, err)
		}
		return evidence.Visual{}, fmt.Errorf("render PDF page %d with %s: %w", physicalPage, binary, err)
	}
	if len(output) == 0 {
		return evidence.Visual{}, fmt.Errorf("render PDF page %d: renderer returned no image", physicalPage)
	}
	return evidence.Visual{
		Kind:         evidence.EvidenceImage,
		SourceID:     source.ID,
		PhysicalPage: physicalPage,
		MediaType:    "image/png",
		DataURL:      "data:image/png;base64," + base64.StdEncoding.EncodeToString(output),
	}, nil
}
