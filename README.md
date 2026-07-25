# tutorio

Tutorio is a local-first desktop application that reconstructs long-form tutorials as practical learning manuals. It is intentionally **not a summariser**: generated guides preserve procedures, explanations, commands, warnings, mistakes, shortcuts, and source timestamps.

Everything runs on the user's machine. There are no cloud services, accounts, or authentication.

## Download

Prebuilt macOS, Windows, and Linux artifacts are published on the [GitHub Releases page](https://github.com/alessio-palumbo/tutorio/releases).

To try Tutorio:

1. Open the latest release.
2. Download the artifact for your platform.
3. Extract the archive.
4. Run `tutorio`.

### macOS quarantine

macOS builds are self-signed and are not notarized yet. After extracting the release archive, macOS may block the app. Remove the quarantine attribute before opening it:

```sh
xattr -dr com.apple.quarantine tutorio.app
```

Run the command from the folder containing `tutorio.app`, or replace `tutorio.app` with its full path.

## Current foundation

This repository provides a buildable Wails application and the architectural spine for the MVP:

- YouTube subtitle retrieval through `yt-dlp`.
- local `.txt`, `.srt`, and `.vtt` transcript ingestion.
- local text-based PDF ingestion with durable source chunks and page-aware evidence through Poppler.
- transcript cleaning and cue-aware segmentation.
- structured JSON generation through a local Ollama model.
- structural verification before persistence.
- a single-worker background compilation queue with cancellation and automatic restart recovery.
- interrupted-job-first recovery and user-controlled “Run first” preemption for pending compilations; completed sections and source identity are retained.
- persistent per-section results with targeted retry after interruption.
- active-section timing, slow-call indicators, and inspectable local model diagnostics.
- verified transcript excerpts, clickable source timestamps, and exact PDF evidence previews.
- section-level overviews, prerequisite deduplication, and locally bundled KaTeX formula rendering.
- visible source sections, guide editing, single-section regeneration, source-grounded deep dives, and Markdown export.
- collapsible guide sections that double as a compact index, plus compact reference blocks for supporting material.
- SQLite storage and a Wails guide library/reader.

The UI is deliberately small. Model setup and native transcript-file selection remain in the usable-MVP phase described in [the roadmap](docs/ROADMAP.md).

## Architecture

Dependency direction points inward. Domain and orchestration packages do not import Wails, SQLite, Ollama, or process-execution details.

```text
Wails UI ─> jobs.Manager ─> jobs.Pipeline ─> source / transcript / guide interfaces
yt-dlp ───┤                         ▲            ▲
Ollama ───┤                         │            │
SQLite ───┘                    domain models  use-case rules
```

### How compilation works

```mermaid
flowchart TD
    A[YouTube URL or transcript file] --> B[Source adapter]
    B --> C[Timestamped transcript or page-aware source chunks]
    C --> D[Clean and split into bounded segments]
    D --> E[Generate structured JSON and source chunk IDs with Ollama]
    E --> F[Validate citations and resolve exact source evidence]
    F --> G[Merge sections, deduplicate, and build cheat sheet]
    G --> H[Synthesize a concise guide overview]
    H --> I[Verify guide structure]
    I --> J[(Save complete guide in SQLite)]
    J --> K[Display in library and guide reader]
```

1. The UI persists a pending job and returns immediately; a single background worker runs queued jobs without competing Ollama requests.
2. The source registry selects the YouTube or local-file adapter.
3. `yt-dlp` retrieves YouTube subtitles; TXT, SRT, and VTT files are parsed directly; Poppler extracts text PDFs by physical page.
4. Transcript text is cleaned while timestamps are preserved. PDF text is persisted as immutable, content-addressed source chunks before generation.
5. Oversized transcripts and cues are divided into model-sized segments.
6. Ollama reconstructs each segment as structured guide content; live progress is sent to Wails.
7. Model variations are normalized. For PDFs, returned chunk IDs are checked against the exact chunks supplied to that request, deduplicated, capped, and resolved to stored text. Unknown IDs are discarded and unsupported steps remain uncited.
8. Each completed section and its performance metadata are persisted so failed work can resume without repeating successful sections.
9. A small, non-blocking Ollama request synthesizes a guide-level overview from stored section titles and summaries. If it fails, the complete guide is still saved and the reader offers a retry.
10. The result is verified, stored as structured JSON in SQLite, and loaded by the library reader after restart.

Package responsibilities:

| Package | Responsibility |
| --- | --- |
| repository `main.go` | Wails-required composition root and lifecycle |
| `cmd/app` | Desktop frontend assets (kept under the conventional app shell) |
| `internal/source` | Source contract and adapter registry |
| `internal/source/youtube` | `yt-dlp` process adapter |
| `internal/source/local` | Local transcript-file adapter |
| `internal/transcript` | Source-neutral parsing, cleaning, segmentation |
| `internal/llm` | Model-provider contract and Ollama HTTP adapter |
| `internal/guide` | Guide domain, generation and verification contracts |
| `internal/evidence` | Registered sources, immutable source chunks, evidence resolution |
| `internal/jobs` | Background queue, pipeline use case, recovery, and stage orchestration |
| `internal/storage/sqlite` | SQLite connection, schema, guide repository |
| `internal/config` | YAML configuration and local defaults |
| `internal/ui` | Thin Wails-facing application API |
| `internal/exporter` | Output contract and Markdown implementation |

Interfaces are owned near the code that consumes their behavior and kept small. `context.Context` crosses every operation that can block. Dependencies are assembled only in the root `main.go`; no package-level mutable state is used. Wails v2's binding generator requires its Go entrypoint at the project root, so this is the one intentional variation from the usual `cmd/app/main.go` layout.

### Extension strategy

New content sources implement `source.Source` and register at startup. Whisper can become another source/transcription adapter; screenshots can be introduced as an optional enrichment stage; vision models can implement a provider boundary parallel to `llm.Provider`. Exporters and learning-artifact generators should be independent use cases over persisted `guide.Guide` values. Playlist support should compose child jobs rather than enlarge the YouTube adapter.

This keeps future media capabilities out of the text-only MVP while leaving clear seams for them.

## Guide model

The stored guide includes a synthesized overview with readiness metadata, prerequisites, final outcome, ordered steps, citations, transcript evidence, important concepts, commands, keyboard shortcuts, warnings, common mistakes, cheat sheet, appendix, source timestamps, source-grounded deep dives, and generation metadata. SQLite stores searchable identity/summary columns plus the complete versionable guide as validated JSON. Registered sources, immutable source chunks, and evidence identities are normalized for reuse. Jobs and individual transcript/model sections are persisted separately for recovery, targeted regeneration, overview retry, timing, and local diagnostics. The base schema is in [`internal/storage/sqlite/schema.sql`](internal/storage/sqlite/schema.sql), with numbered changes in [`internal/storage/sqlite/migrations`](internal/storage/sqlite/migrations).

For a newly generated PDF guide, selecting a citation opens a lightweight evidence drawer containing the exact extracted chunk, its neighbouring chunks, the source title, physical PDF page, and a locally rendered page preview for figures, tables, and formatting. “Open full PDF” remains a secondary native-viewer fallback. Older saved guides with page-only references still show the source and physical page, but correctly show no excerpt; recompile the PDF to create durable evidence. See [the evidence architecture decision](docs/architecture/evidence.md).

For production evolution, add numbered embedded migrations rather than editing an already-released migration.

## Prerequisites

- Go 1.25 or newer.
- Wails v2 and its platform prerequisites.
- Node.js/npm for the Vite frontend.
- `yt-dlp` on `PATH` for YouTube sources.
- Poppler's `pdftotext` and `pdftocairo` on `PATH` for PDF text and page previews.
- Ollama running locally with the configured model, initially `qwen3:8b`.

For example:

```sh
ollama pull qwen3:8b
ollama serve
```

On macOS, install the local source tools with:

```sh
brew install yt-dlp poppler
```

Scanned or image-only PDFs are detected but require a future OCR adapter; Tutorio does not silently generate a guide from missing text.

Tutorio invokes tools locally and sends model requests only to the configured Ollama URL.

## Configuration

Tutorio resolves configuration in this order:

1. The path in `TUTORIO_CONFIG_PATH`, when set.
2. `config.yaml` in the current working directory (convenient for `wails dev`).
3. The OS user config directory (`tutorio/config.yaml`).

If none exists, local defaults are used. Copy `config.example.yaml` to `config.yaml` for development. The startup log reports the selected path and Ollama model. The default database is placed under the OS user config directory unless `database.path` overrides it.

```yaml
database:
  path: ./data/tutorio.db
ollama:
  base_url: http://127.0.0.1:11434
  model: qwen3:8b
  max_output_tokens: 8192
  context_window: 32768
tools:
  yt_dlp_path: yt-dlp
  pdftotext_path: pdftotext
  pdftocairo_path: pdftocairo
processing:
  segment_characters: 12000
```

`segment_characters` is a conservative character budget, not a tokenizer. A later model-aware segmenter can replace it without changing the pipeline.

### Choosing an Ollama model

There is no universal best model: tutorial reconstruction trades generation time, completeness, concision, schema reliability, and local memory use.

| Model | Expected fit for tutorio |
| --- | --- |
| `gemma4:e4b` | Recommended quality baseline when concise, tightly scoped guide sections matter more than generation speed. In initial testing it was slower but less verbose. |
| `qwen3.5:9b` | Useful detail-oriented alternative. Initial testing extracted more material, but sometimes produced overly verbose or unnecessary content. |
| `qwen3.5:4b` | A useful newer, smaller Qwen competitor for speed/quality experiments against Gemma 4 E4B. It has not yet been validated by this project. |
| `qwen3:8b` | Older text-only Qwen 3 baseline. The tag means “Qwen 3, 8B parameters”; it is not a Qwen 3.8 release and does not supersede Qwen 3.5. |

Keep `max_output_tokens`, `context_window`, segmentation, source URL, and prompt settings identical when comparing models. Compare total duration, step usefulness, timestamp accuracy, shortcut/command extraction, schema reliability, and unnecessary repetition—not output length alone.

## Development

Install dependencies and run tests:

```sh
go mod tidy
go test ./...
```

Run the desktop app with live frontend reload:

```sh
wails dev
```

Build the production desktop bundle:

```sh
wails build
```

Useful equivalents are available through `make test`, `make dev`, and `make build`.

When adding a feature, put domain data/rules in an inner package, define the smallest needed interface, implement infrastructure in an adapter package, and wire it at the composition root. Use fakes at the interface boundary in unit tests. External-process and HTTP adapters should additionally have fixture-driven contract tests.

Run the opt-in local integration test against a real URL and the configured Ollama model with:

```sh
TUTORIO_CONFIG_PATH="$PWD/config.yaml" \
TUTORIO_TEST_URL="https://www.youtube.com/watch?v=VIDEO_ID" \
go test -tags=integration ./internal/integration -run TestCompileYouTube -v -count=1 -timeout=20m
```

The test uses a temporary SQLite database and does not add its output to the desktop library.

## Release builds

The release workflow builds macOS, Windows, and Linux artifacts from tags matching `v*` and publishes them to a GitHub Release.

Current release limitations:

- macOS notarization is not configured.
- Windows signing is not configured.
- Linux packaging contains the Wails build output rather than a distribution-specific package.
- Runtime tools such as Ollama, `yt-dlp`, and Poppler are not bundled.

## Deliberate limitations

- Long transcripts are generated section-by-section and merged deterministically. A later synthesis pass may improve cross-section narrative cohesion without sacrificing provenance.
- Deep dives deliberately use only the saved source transcript and current section steps; optional cited web research is not implemented.
- PDF step citations are grounded to exact extracted chunks. Broader claim-level semantic verification and support classification remain future work.
- The queue deliberately runs one compilation at a time to avoid concurrent local-model memory pressure.
- The backend supports transcript-file import, while native file selection and its frontend control are Phase 1 UI work.
- Audio transcription, screenshots, vision, playlists, PDF export, flashcards, quizzes, and progressive learning are architecture extension points only.

See [docs/ROADMAP.md](docs/ROADMAP.md) for phased delivery criteria.
