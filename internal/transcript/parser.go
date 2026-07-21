package transcript

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parser converts an imported transcript stream into a Document.
type Parser interface {
	Parse(ctx context.Context, name string, r io.Reader) (Document, error)
}

// FileParser supports plain text, SubRip, and WebVTT transcripts.
type FileParser struct{}

func NewFileParser() FileParser { return FileParser{} }

var timestampLine = regexp.MustCompile(`(?m)^\s*((?:\d{1,2}:)?\d{2}:\d{2}[,.]\d{3})\s+-->\s+((?:\d{1,2}:)?\d{2}:\d{2}[,.]\d{3}).*$`)

func (FileParser) Parse(ctx context.Context, name string, r io.Reader) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Document{}, fmt.Errorf("read transcript: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	doc := Document{SourceID: name, Title: strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".txt" || ext == "" {
		doc.Cues = []Cue{{Text: strings.TrimSpace(string(data))}}
		return doc, nil
	}
	if ext != ".srt" && ext != ".vtt" {
		return Document{}, fmt.Errorf("unsupported transcript extension %q", ext)
	}
	doc.Cues, err = parseTimed(string(data))
	if err != nil {
		return Document{}, err
	}
	return doc, nil
}

func parseTimed(raw string) ([]Cue, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	matches := timestampLine.FindAllStringSubmatchIndex(raw, -1)
	cues := make([]Cue, 0, len(matches))
	for i, match := range matches {
		start, err := parseTimestamp(raw[match[2]:match[3]])
		if err != nil {
			return nil, err
		}
		end, err := parseTimestamp(raw[match[4]:match[5]])
		if err != nil {
			return nil, err
		}
		textStart := match[1]
		textEnd := len(raw)
		if i+1 < len(matches) {
			textEnd = matches[i+1][0]
		}
		lines := []string{}
		scanner := bufio.NewScanner(strings.NewReader(raw[textStart:textEnd]))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !onlyDigits(line) && !strings.HasPrefix(line, "NOTE") {
				lines = append(lines, line)
			}
		}
		if len(lines) > 0 {
			cues = append(cues, Cue{Start: start, End: end, Text: strings.Join(lines, " ")})
		}
	}
	if len(cues) == 0 {
		return nil, fmt.Errorf("transcript contains no timestamped cues")
	}
	return cues, nil
}
func onlyDigits(s string) bool { _, err := strconv.Atoi(s); return err == nil }
func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(strings.ReplaceAll(s, ",", "."), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
	var h int
	var m int
	var sec float64
	var err error
	if len(parts) == 3 {
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		parts = parts[1:]
	}
	m, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	sec, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second)), nil
}
