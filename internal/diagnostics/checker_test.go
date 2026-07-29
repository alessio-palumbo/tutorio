package diagnostics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckerReportsAvailableModelAndTools(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"models":[{"name":"gemma4:e4b"}]}`))}, nil
	})}
	toolName := "tool"
	if runtime.GOOS == "windows" {
		toolName += ".exe"
	}
	tool := filepath.Join(t.TempDir(), toolName)
	if err := os.WriteFile(tool, []byte("tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := New(client, "http://ollama.test", "gemma4:e4b", "/config.yaml", map[string]string{
		"yt-dlp": tool, "pdftotext": tool, "pdftocairo": tool,
	}).Check(context.Background())
	if len(report.Checks) != 5 {
		t.Fatalf("got %d checks", len(report.Checks))
	}
	if len(report.Tools) != 3 || report.Tools[0].Path != tool {
		t.Fatalf("unexpected tool settings: %#v", report.Tools)
	}
	for _, check := range report.Checks {
		if check.Status != "ready" {
			t.Fatalf("%s status = %q: %s", check.ID, check.Status, check.Message)
		}
	}
}

func TestCheckerExplainsUnavailableOllamaAndTools(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("service unavailable")
	})}
	report := New(client, "http://ollama.test", "gemma4:e4b", "/config.yaml", map[string]string{
		"yt-dlp": "/missing/yt-dlp", "pdftotext": "/missing/pdftotext", "pdftocairo": "/missing/pdftocairo",
	}).Check(context.Background())
	for _, check := range report.Checks {
		if check.Status == "ready" {
			t.Fatalf("%s unexpectedly ready", check.ID)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
