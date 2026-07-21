// Package markdown renders portable Markdown guides.
package markdown

import (
	"bytes"
	"context"
	"fmt"
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
		timestamps(&b, g.SourceURI, step.Timestamps)
		section(&b, "Actions", step.Actions)
		if len(step.Commands) > 0 {
			write("**Commands**\n\n")
			for _, command := range step.Commands {
				write("```text\n%s\n```\n\n", command)
			}
		}
		section(&b, "Warnings", step.Warnings)
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
	if len(g.SourceTimestamps) > 0 {
		write("## Source timestamps\n\n")
		timestamps(&b, g.SourceURI, g.SourceTimestamps)
	}
	return b.Bytes(), nil
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
