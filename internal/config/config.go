// Package config loads local YAML configuration with safe defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	YTDLPPath     string `yaml:"yt_dlp_path"`
	PDFToTextPath string `yaml:"pdftotext_path"`
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
	return Config{Database: Database{Path: filepath.Join(dir, "tutorio", "tutorio.db")}, Ollama: Ollama{BaseURL: "http://127.0.0.1:11434", Model: "qwen3:8b", MaxOutputTokens: 8192, ContextWindow: 32768}, Tools: Tools{YTDLPPath: "yt-dlp", PDFToTextPath: "pdftotext"}, Processing: Processing{SegmentCharacters: 12000}}
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
	return cfg, nil
}
