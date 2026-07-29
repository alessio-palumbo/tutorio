package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("TUTORIO_CONFIG_PATH", "/tmp/custom-tutorio.yaml")
	if got := Path(); got != "/tmp/custom-tutorio.yaml" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadAppliesDefaultsToPartialNestedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ollama:\n  model: qwen3.5:9b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ollama.MaxOutputTokens != 8192 {
		t.Fatalf("got max output tokens %d", cfg.Ollama.MaxOutputTokens)
	}
	if cfg.Ollama.ContextWindow != 32768 {
		t.Fatalf("got context window %d", cfg.Ollama.ContextWindow)
	}
	if cfg.Processing.SegmentCharacters != 12000 {
		t.Fatalf("got segment characters %d", cfg.Processing.SegmentCharacters)
	}
	if cfg.Tools.PDFToTextPath != "pdftotext" {
		t.Fatalf("got pdftotext path %q", cfg.Tools.PDFToTextPath)
	}
}

func TestLoadUsesRecommendedDefaultModel(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ollama.Model != "gemma4:e4b" {
		t.Fatalf("got default model %q", cfg.Ollama.Model)
	}
}

func TestEnsureCreatesEditableConfigurationOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	created, err := Ensure(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected configuration to be created")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"model: gemma4:e4b", "segment_characters: 12000", "yt_dlp_path:"} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("generated configuration does not contain %q:\n%s", expected, data)
		}
	}
	if err = os.WriteFile(path, []byte("custom: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err = Ensure(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("existing configuration was unexpectedly replaced")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "custom: true\n" {
		t.Fatalf("existing configuration changed: %q", data)
	}
}

func TestUpdateToolsPreservesOtherConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := "database:\n  path: /tmp/library.db\nollama:\n  model: custom-model\nprocessing:\n  segment_characters: 9000\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	tools := Tools{YTDLPPath: "/tools/yt-dlp", PDFToTextPath: "/tools/pdftotext", PDFToCairoPath: "/tools/pdftocairo"}
	if err := UpdateTools(path, tools); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tools != tools {
		t.Fatalf("tools = %#v", cfg.Tools)
	}
	if cfg.Database.Path != "/tmp/library.db" || cfg.Ollama.Model != "custom-model" || cfg.Processing.SegmentCharacters != 9000 {
		t.Fatalf("unrelated configuration changed: %#v", cfg)
	}
}

func TestUpdateToolsRejectsEmptyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ollama:\n  model: gemma4:e4b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := UpdateTools(path, Tools{YTDLPPath: "yt-dlp", PDFToTextPath: "", PDFToCairoPath: "pdftocairo"})
	if err == nil || !strings.Contains(err.Error(), "pdftotext_path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
