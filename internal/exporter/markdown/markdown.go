// Package markdown renders portable Markdown guides.
package markdown

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/alessio/tutorio/internal/guide"
)

type Exporter struct{}

func New() Exporter                { return Exporter{} }
func (Exporter) Extension() string { return ".md" }
func (Exporter) Render(ctx context.Context, g guide.Guide) ([]byte, error) {
	var b bytes.Buffer
	write := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	write("# %s\n\n%s\n\n", g.Title, g.Overview)
	section(&b, "Prerequisites", g.Prerequisites)
	write("## Final outcome\n\n%s\n\n", g.FinalOutcome)
	write("## Step-by-step guide\n\n")
	for _, step := range g.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		write("### %d. %s\n\n%s\n\n", step.Number, step.Title, step.Explanation)
		if step.SourceExcerpt != "" {
			write("> Supporting transcript: %s\n\n", step.SourceExcerpt)
		}
		references(&b, g.SourceURI, step.References, step.Timestamps)
		section(&b, "Actions", step.Actions)
		if len(step.Commands) > 0 {
			write("**Commands**\n\n")
			for _, command := range step.Commands {
				write("```text\n%s\n```\n\n", command)
			}
		}
		section(&b, "Warnings", step.Warnings)
	}
	if len(g.DeepDives) > 0 {
		write("## Deep dives\n\n")
		for _, item := range g.DeepDives {
			write("### %s\n\n%s\n\n", item.Title, item.Explanation)
			bulletList(&b, "Key points", item.KeyPoints)
			bulletList(&b, "Examples", item.Examples)
			bulletList(&b, "Caveats", item.Caveats)
			bulletList(&b, "Source evidence", item.Evidence)
		}
	}
	section(&b, "Important concepts", g.ImportantConcepts)
	if len(g.Commands) > 0 {
		write("## Commands\n\n")
		for _, command := range g.Commands {
			write("```text\n%s\n```\n%s\n\n", command.Value, command.Description)
		}
	}
	if len(g.KeyboardShortcuts) > 0 {
		write("## Keyboard shortcuts\n\n| Keys | Action | Context |\n| --- | --- | --- |\n")
		for _, item := range g.KeyboardShortcuts {
			write("| %s | %s | %s |\n", escape(item.Keys), escape(item.Action), escape(item.Context))
		}
		write("\n")
	}
	section(&b, "Warnings", g.Warnings)
	section(&b, "Common mistakes", g.CommonMistakes)
	section(&b, "Cheat sheet", g.CheatSheet)
	section(&b, "Appendix", g.Appendix)
	if len(g.SourceReferences) > 0 || len(g.SourceTimestamps) > 0 {
		write("## Source references\n\n")
		references(&b, g.SourceURI, g.SourceReferences, g.SourceTimestamps)
	}
	return b.Bytes(), nil
}
func references(b *bytes.Buffer, uri string, values []guide.SourceReference, fallback []guide.Timestamp) {
	if len(values) == 0 {
		timestamps(b, uri, fallback)
		return
	}
	b.WriteString("**Source:** ")
	for index, value := range values {
		if index > 0 {
			b.WriteString(" · ")
		}
		if value.Kind == "page" {
			label := fmt.Sprintf("page %d", value.PageStart)
			if value.PageEnd > value.PageStart {
				label = fmt.Sprintf("pages %d-%d", value.PageStart, value.PageEnd)
			}
			if value.Label != "" {
				label += " " + value.Label
			}
			fileURL := (&url.URL{Scheme: "file", Path: uri, Fragment: fmt.Sprintf("page=%d", value.PageStart)}).String()
			fmt.Fprintf(b, "[%s](%s)", label, fileURL)
			continue
		}
		label := formatTime(value.StartSeconds)
		if value.Label != "" {
			label += " " + value.Label
		}
		fmt.Fprintf(b, "[%s](%s)", label, timestampURL(uri, value.StartSeconds))
	}
	b.WriteString("\n\n")
}
func bulletList(b *bytes.Buffer, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "**%s**\n\n", title)
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			fmt.Fprintf(b, "- %s\n", value)
		}
	}
	b.WriteString("\n")
}
func section(b *bytes.Buffer, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			fmt.Fprintf(b, "- %s\n", value)
		}
	}
	b.WriteString("\n")
}
func timestamps(b *bytes.Buffer, uri string, values []guide.Timestamp) {
	if len(values) == 0 {
		return
	}
	b.WriteString("**Source:** ")
	for index, value := range values {
		if index > 0 {
			b.WriteString(" · ")
		}
		label := formatTime(value.StartSeconds)
		if value.Label != "" {
			label += " " + value.Label
		}
		if uri != "" {
			fmt.Fprintf(b, "[%s](%s)", label, timestampURL(uri, value.StartSeconds))
		} else {
			b.WriteString(label)
		}
	}
	b.WriteString("\n\n")
}
func formatTime(seconds float64) string {
	total := int(seconds + .5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
func timestampURL(uri string, seconds float64) string {
	separator := "?"
	if strings.Contains(uri, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%st=%ds", uri, separator, int(seconds+.5))
}
func escape(value string) string { return strings.ReplaceAll(value, "|", "\\|") }
