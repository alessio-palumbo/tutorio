// Package html renders portable, self-contained HTML guides.
package html

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/alessio/tutorio/internal/guide"
)

type Exporter struct {
	template *template.Template
}

type exportStep struct {
	guide.Step
	DisplayNumber int
}

type pageData struct {
	Guide guide.Guide
	Steps []exportStep
}

func New() Exporter {
	functions := template.FuncMap{
		"timestampURL": timestampURL,
		"pageURL":      pageURL,
		"formatTime":   formatTime,
	}
	return Exporter{template: template.Must(template.New("guide").Funcs(functions).Parse(document))}
}

func (Exporter) Extension() string { return ".html" }

func (e Exporter) Render(ctx context.Context, value guide.Guide) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	steps := make([]exportStep, 0, len(value.Steps))
	lastSegment, displayNumber := -1, 0
	for _, step := range value.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if step.SourceSegment != lastSegment {
			lastSegment, displayNumber = step.SourceSegment, 0
		}
		displayNumber++
		steps = append(steps, exportStep{Step: step, DisplayNumber: displayNumber})
	}
	var output bytes.Buffer
	if err := e.template.Execute(&output, pageData{Guide: value, Steps: steps}); err != nil {
		return nil, fmt.Errorf("render HTML guide: %w", err)
	}
	return output.Bytes(), nil
}

func formatTime(seconds float64) string {
	total := int(seconds + .5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func timestampURL(uri string, seconds float64) template.URL {
	parsed, err := url.Parse(uri)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	query := parsed.Query()
	query.Set("t", fmt.Sprintf("%ds", int(seconds+.5)))
	parsed.RawQuery = query.Encode()
	return template.URL(parsed.String()) // #nosec G203 -- restricted to parsed HTTP(S) URLs.
}

func pageURL(uri string, page int) template.URL {
	if strings.TrimSpace(uri) == "" || page < 1 {
		return ""
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" {
		parsed = &url.URL{Scheme: "file", Path: uri}
	} else if parsed.Scheme != "file" {
		return ""
	}
	parsed.Fragment = fmt.Sprintf("page=%d", page)
	return template.URL(parsed.String()) // #nosec G203 -- restricted to local file URLs.
}

const document = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Guide.Title}} — Tutorio</title>
<style>
:root{color-scheme:light;--ink:#20231f;--muted:#62675f;--green:#26352b;--line:#dcd8cd;--paper:#f7f5ef;--card:#fff;--gold:#e6a642}*{box-sizing:border-box}html{background:var(--paper);color:var(--ink);font:17px/1.65 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}body{margin:0}.page{width:min(900px,calc(100% - 32px));margin:0 auto;padding:64px 0 90px}header{padding-bottom:32px;border-bottom:1px solid var(--line)}.brand,.kicker{color:#59705b;font-size:.72rem;font-weight:700;letter-spacing:.16em;text-transform:uppercase}h1,h2,h3{font-family:Georgia,"Times New Roman",serif;line-height:1.2}h1{margin:.25rem 0 1rem;font-size:clamp(2.5rem,8vw,4.4rem)}h2{margin:2.8rem 0 1rem;font-size:2rem}h3{margin:.1rem 0 .7rem;font-size:1.45rem}p{white-space:pre-line}.overview{max-width:760px;color:#434842;font:1.18rem/1.7 Georgia,"Times New Roman",serif}.outcome{margin:36px 0;padding:22px 26px;border-left:5px solid var(--gold);background:var(--card)}ul{padding-left:1.35rem}li+li{margin-top:.45rem}.steps{display:grid;gap:18px}.step{display:grid;grid-template-columns:44px 1fr;gap:18px;padding:24px;border:1px solid var(--line);border-radius:14px;background:var(--card);break-inside:avoid}.number{display:grid;width:38px;height:38px;place-items:center;border-radius:50%;background:var(--green);color:#fff;font-weight:700}.label{margin:1.1rem 0 .35rem;color:#59705b;font-size:.74rem;font-weight:800;letter-spacing:.08em;text-transform:uppercase}.chips{display:flex;flex-wrap:wrap;gap:7px}.chip{padding:4px 9px;border-radius:999px;background:#e9eee8;color:#405941;font-size:.78rem;text-decoration:none}.evidence{margin:1rem 0;padding:12px 16px;border-left:3px solid #78907b;background:#f3f6f1;color:#4d514c}pre{overflow:auto;padding:16px;border-radius:9px;background:#202923;color:#f6f4ee;white-space:pre-wrap;word-break:break-word}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.reference-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:16px}.reference{padding:18px 20px;border:1px solid var(--line);border-radius:10px;background:var(--card)}.reference h2{margin:0 0 .7rem;font-size:1.4rem}.shortcuts{width:100%;border-collapse:collapse;background:var(--card)}th,td{padding:10px 12px;border:1px solid var(--line);text-align:left;vertical-align:top}.deep-dive{margin:18px 0;padding:22px;border:1px solid #c9d5c5;border-radius:12px;background:#eef2ea}footer{margin-top:56px;padding-top:20px;border-top:1px solid var(--line);color:var(--muted);font-size:.82rem}@media(max-width:600px){html{font-size:16px}.page{width:min(100% - 22px,900px);padding-top:28px}.step{grid-template-columns:1fr;padding:19px}.number{width:34px;height:34px}.reference-grid{grid-template-columns:1fr}th,td{padding:8px;font-size:.88rem}}@media print{html{background:#fff;font-size:11pt}.page{width:100%;padding:0}.step,.reference,.outcome,.deep-dive{box-shadow:none}a{color:inherit}.step{break-inside:avoid}h2,h3{break-after:avoid}footer{margin-top:30px}}
</style>
</head>
<body><main class="page">
<header><div class="brand">Tutorio learning guide</div><h1>{{.Guide.Title}}</h1>{{if .Guide.Overview}}<div class="overview">{{.Guide.Overview}}</div>{{end}}</header>
{{if .Guide.FinalOutcome}}<section class="outcome"><div class="kicker">Final outcome</div><p>{{.Guide.FinalOutcome}}</p></section>{{end}}
{{if .Guide.Prerequisites}}<section><h2>Prerequisites</h2><ul>{{range .Guide.Prerequisites}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
{{if .Steps}}<section><h2>Step-by-step guide</h2><div class="steps">{{range .Steps}}<article class="step"><div class="number">{{.DisplayNumber}}</div><div><h3>{{.Title}}</h3><p>{{.Explanation}}</p>
{{if .SourceExcerpt}}<blockquote class="evidence"><strong>Supporting transcript</strong><br>{{.SourceExcerpt}}</blockquote>{{end}}
{{if or .Citations .References .Timestamps}}<div class="label">Sources</div><div class="chips">{{range .Citations}}<span class="chip">{{if .Label}}{{.Label}}{{else}}Source evidence{{end}}</span>{{end}}{{range .References}}{{$reference := .}}{{if eq .Kind "page"}}{{with pageURL $.Guide.SourceURI .PageStart}}<a class="chip" href="{{.}}">PDF page {{$reference.PageStart}}</a>{{else}}<span class="chip">PDF page {{$reference.PageStart}}</span>{{end}}{{else}}{{with timestampURL $.Guide.SourceURI .StartSeconds}}<a class="chip" href="{{.}}">{{formatTime $reference.StartSeconds}}</a>{{else}}<span class="chip">{{formatTime $reference.StartSeconds}}</span>{{end}}{{end}}{{end}}{{range .Timestamps}}{{$timestamp := .}}{{with timestampURL $.Guide.SourceURI .StartSeconds}}<a class="chip" href="{{.}}">{{formatTime $timestamp.StartSeconds}}{{if $timestamp.Label}} · {{$timestamp.Label}}{{end}}</a>{{else}}<span class="chip">{{formatTime $timestamp.StartSeconds}}{{if $timestamp.Label}} · {{$timestamp.Label}}{{end}}</span>{{end}}{{end}}</div>{{end}}
{{if .Actions}}<div class="label">Actions</div><ul>{{range .Actions}}<li>{{.}}</li>{{end}}</ul>{{end}}{{if .Commands}}<div class="label">Commands</div>{{range .Commands}}<pre><code>{{.}}</code></pre>{{end}}{{end}}{{if .Warnings}}<div class="label">Warnings</div><ul>{{range .Warnings}}<li>{{.}}</li>{{end}}</ul>{{end}}</div></article>{{end}}</div></section>{{end}}
{{if .Guide.DeepDives}}<section><h2>Deep dives</h2>{{range .Guide.DeepDives}}<article class="deep-dive"><h3>{{.Title}}</h3><p>{{.Explanation}}</p>{{if .KeyPoints}}<div class="label">Key points</div><ul>{{range .KeyPoints}}<li>{{.}}</li>{{end}}</ul>{{end}}{{if .Examples}}<div class="label">Examples</div><ul>{{range .Examples}}<li>{{.}}</li>{{end}}</ul>{{end}}{{if .Caveats}}<div class="label">Caveats</div><ul>{{range .Caveats}}<li>{{.}}</li>{{end}}</ul>{{end}}</article>{{end}}</section>{{end}}
<div class="reference-grid">
{{if .Guide.ImportantConcepts}}<section class="reference"><h2>Important concepts</h2><ul>{{range .Guide.ImportantConcepts}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
{{if .Guide.Warnings}}<section class="reference"><h2>Warnings</h2><ul>{{range .Guide.Warnings}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
{{if .Guide.CommonMistakes}}<section class="reference"><h2>Common mistakes</h2><ul>{{range .Guide.CommonMistakes}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
{{if .Guide.CheatSheet}}<section class="reference"><h2>Cheat sheet</h2><ul>{{range .Guide.CheatSheet}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
{{if .Guide.Appendix}}<section class="reference"><h2>Appendix</h2><ul>{{range .Guide.Appendix}}<li>{{.}}</li>{{end}}</ul></section>{{end}}
</div>
{{if .Guide.Commands}}<section><h2>Commands</h2>{{range .Guide.Commands}}<pre><code>{{.Value}}</code></pre>{{if .Description}}<p>{{.Description}}</p>{{end}}{{end}}</section>{{end}}
{{if .Guide.KeyboardShortcuts}}<section><h2>Keyboard shortcuts</h2><table class="shortcuts"><thead><tr><th>Keys</th><th>Action</th><th>Context</th></tr></thead><tbody>{{range .Guide.KeyboardShortcuts}}<tr><td><code>{{.Keys}}</code></td><td>{{.Action}}</td><td>{{.Context}}</td></tr>{{end}}</tbody></table></section>{{end}}
<footer>Generated locally with Tutorio{{if .Guide.Generation.Model}} using {{.Guide.Generation.Model}}{{end}}.</footer>
</main></body></html>`
