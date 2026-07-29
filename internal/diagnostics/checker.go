// Package diagnostics reports optional local runtime capabilities without
// preventing Tutorio from opening its existing library.
package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Check struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

type Report struct {
	ConfigPath string  `json:"config_path"`
	Checks     []Check `json:"checks"`
	Tools      []Tool  `json:"tools"`
}

type Tool struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type Checker interface {
	Check(context.Context) Report
}

type LocalChecker struct {
	mu         sync.RWMutex
	client     *http.Client
	baseURL    string
	model      string
	configPath string
	tools      map[string]string
}

func (c *LocalChecker) SetTools(tools map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = cloneTools(tools)
}

func New(client *http.Client, baseURL, model, configPath string, tools map[string]string) *LocalChecker {
	if client == nil {
		client = http.DefaultClient
	}
	return &LocalChecker{
		client: client, baseURL: strings.TrimRight(baseURL, "/"), model: model,
		configPath: configPath, tools: cloneTools(tools),
	}
}

func (c *LocalChecker) Check(ctx context.Context) Report {
	checks, modelsAvailable := c.checkOllama(ctx)
	if modelsAvailable != nil {
		status := "missing"
		message := fmt.Sprintf("Model %s is not installed.", c.model)
		action := fmt.Sprintf("Run ollama pull %s.", c.model)
		if containsModel(modelsAvailable, c.model) {
			status, message, action = "ready", fmt.Sprintf("Model %s is available.", c.model), ""
		}
		checks = append(checks, Check{ID: "ollama-model", Name: "Ollama model", Status: status, Message: message, Action: action})
	} else {
		checks = append(checks, Check{ID: "ollama-model", Name: "Ollama model", Status: "unknown", Message: fmt.Sprintf("Cannot check model %s until Ollama is reachable.", c.model)})
	}
	toolDefinitions := []struct {
		id, name, configKey, install string
	}{
		{"yt-dlp", "YouTube support", "yt-dlp", "Install yt-dlp or set tools.yt_dlp_path."},
		{"pdftotext", "PDF text extraction", "pdftotext", "Install Poppler or set tools.pdftotext_path."},
		{"pdftocairo", "PDF page previews", "pdftocairo", "Install Poppler or set tools.pdftocairo_path."},
	}
	tools := make([]Tool, 0, len(toolDefinitions))
	c.mu.RLock()
	configuredTools := cloneTools(c.tools)
	c.mu.RUnlock()
	for _, tool := range toolDefinitions {
		path := configuredTools[tool.configKey]
		tools = append(tools, Tool{ID: tool.id, Label: tool.name, Path: path})
		if resolved, ok := executablePath(path); ok {
			checks = append(checks, Check{ID: tool.id, Name: tool.name, Status: "ready", Message: resolved})
		} else {
			checks = append(checks, Check{ID: tool.id, Name: tool.name, Status: "missing", Message: fmt.Sprintf("%s was not found.", path), Action: tool.install})
		}
	}
	return Report{ConfigPath: c.configPath, Checks: checks, Tools: tools}
}

func (c *LocalChecker) checkOllama(ctx context.Context) ([]Check, []string) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return []Check{{ID: "ollama", Name: "Ollama service", Status: "missing", Message: "The configured Ollama URL is invalid.", Action: "Check ollama.base_url in config.yaml."}}, nil
	}
	response, err := c.client.Do(request)
	if err != nil {
		return []Check{{ID: "ollama", Name: "Ollama service", Status: "missing", Message: "Ollama is not reachable.", Action: "Open the Ollama app, or run ollama serve if no service manages it."}}, nil
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return []Check{{ID: "ollama", Name: "Ollama service", Status: "missing", Message: fmt.Sprintf("Ollama returned %s.", response.Status), Action: "Check ollama.base_url and the Ollama service."}}, nil
	}
	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return []Check{{ID: "ollama", Name: "Ollama service", Status: "missing", Message: "Ollama returned an unreadable model list.", Action: "Check the configured Ollama service."}}, nil
	}
	models := make([]string, 0, len(payload.Models)*2)
	for _, model := range payload.Models {
		models = append(models, model.Name, model.Model)
	}
	return []Check{{ID: "ollama", Name: "Ollama service", Status: "ready", Message: "Ollama is running."}}, models
}

func executablePath(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	path, err := exec.LookPath(value)
	return path, err == nil
}

func ValidateExecutable(value string) (string, error) {
	if path, ok := executablePath(value); ok {
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute, nil
		}
		return path, nil
	}
	return "", fmt.Errorf("%s is not an executable file", strings.TrimSpace(value))
}

func cloneTools(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func containsModel(models []string, configured string) bool {
	configured = strings.TrimSpace(configured)
	for _, model := range models {
		if model == configured || strings.TrimSuffix(model, ":latest") == strings.TrimSuffix(configured, ":latest") {
			return true
		}
	}
	return false
}
