package config

import (
	"os"
	"path/filepath"
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
}
