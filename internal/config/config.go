// Package config loads local YAML configuration with safe defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database   Database   `yaml:"database"`
	Ollama     Ollama     `yaml:"ollama"`
	Tools      Tools      `yaml:"tools"`
	Processing Processing `yaml:"processing"`
}
type Database struct {
	Path string `yaml:"path"`
}
type Ollama struct {
	BaseURL         string `yaml:"base_url"`
	Model           string `yaml:"model"`
	MaxOutputTokens int    `yaml:"max_output_tokens"`
	ContextWindow   int    `yaml:"context_window"`
}
type Tools struct {
	YTDLPPath      string `yaml:"yt_dlp_path"`
	PDFToTextPath  string `yaml:"pdftotext_path"`
	PDFToCairoPath string `yaml:"pdftocairo_path"`
}
type Processing struct {
	SegmentCharacters int `yaml:"segment_characters"`
}

// Path resolves configuration in order of explicit environment override,
// project-local development config, then the OS user config directory.
func Path() string {
	if path := os.Getenv("TUTORIO_CONFIG_PATH"); path != "" {
		return path
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return DefaultPath()
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "tutorio.yaml"
	}
	return filepath.Join(dir, "tutorio", "config.yaml")
}
func defaults() Config {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return Config{Database: Database{Path: filepath.Join(dir, "tutorio", "tutorio.db")}, Ollama: Ollama{BaseURL: "http://127.0.0.1:11434", Model: "gemma4:e4b", MaxOutputTokens: 8192, ContextWindow: 32768}, Tools: Tools{YTDLPPath: "yt-dlp", PDFToTextPath: "pdftotext", PDFToCairoPath: "pdftocairo"}, Processing: Processing{SegmentCharacters: 12000}}
}

// Ensure creates an editable first-run configuration without replacing an
// existing file. Executables are recorded as absolute paths when they can be
// found safely in the application environment or a common installation path.
func Ensure(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect %s: %w", path, err)
	}
	cfg := defaults()
	cfg.Tools.YTDLPPath = discoverExecutable("yt-dlp")
	cfg.Tools.PDFToTextPath = discoverExecutable("pdftotext")
	cfg.Tools.PDFToCairoPath = discoverExecutable("pdftocairo")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return false, fmt.Errorf("encode default configuration: %w", err)
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create configuration directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create %s: %w", path, err)
	}
	content := append([]byte("# Tutorio local configuration. Existing files are never overwritten.\n"), data...)
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	if err = file.Close(); err != nil {
		return false, fmt.Errorf("close %s: %w", path, err)
	}
	return true, nil
}

func discoverExecutable(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		if absolute, absErr := filepath.Abs(path); absErr == nil {
			return absolute
		}
		return path
	}
	var directories []string
	switch runtime.GOOS {
	case "darwin":
		directories = []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin"}
	case "linux":
		directories = []string{"/usr/local/bin", "/usr/bin", "/snap/bin"}
	}
	for _, directory := range directories {
		path := filepath.Join(directory, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return name
}

func Load(path string) (Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Database.Path == "" {
		return Config{}, fmt.Errorf("database.path is required")
	}
	if cfg.Ollama.Model == "" {
		return Config{}, fmt.Errorf("ollama.model is required")
	}
	if cfg.Ollama.MaxOutputTokens <= 0 {
		cfg.Ollama.MaxOutputTokens = 8192
	}
	if cfg.Ollama.ContextWindow <= 0 {
		cfg.Ollama.ContextWindow = 32768
	}
	if cfg.Processing.SegmentCharacters <= 0 {
		cfg.Processing.SegmentCharacters = 12000
	}
	if cfg.Tools.PDFToTextPath == "" {
		cfg.Tools.PDFToTextPath = "pdftotext"
	}
	if cfg.Tools.PDFToCairoPath == "" {
		cfg.Tools.PDFToCairoPath = "pdftocairo"
	}
	return cfg, nil
}
