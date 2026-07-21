# tutorio roadmap

The roadmap deliberately grows capabilities by adding adapters and use cases around a stable guide domain. Later phases should be informed by real guide quality and performance data rather than built speculatively.

## Phase 0 — foundation (this repository)

- Clean package boundaries, dependency injection, structured logging, cancellation-aware interfaces.
- YouTube and transcript-file source adapters.
- TXT, SRT, and VTT parsing; cleaning; bounded segmentation.
- Ollama chat adapter and structured guide schema.
- Structural verification and SQLite persistence.
- Minimal Wails compile/library shell.

Exit criterion: all packages and a production Wails bundle build locally.

## Phase 1 — usable MVP

- Add native file selection and full guide reader/editor screens.
- Introduce persisted jobs with progress events, cancellation, retry, and recovery after restart.
- Evaluate an optional bounded synthesis pass to improve cohesion across the existing per-segment generation and deterministic merge flow.
- Add semantic verification against transcript evidence, JSON repair, and user-visible quality diagnostics.
- Add editable prompt profiles and model discovery/health checks.
- Detect yt-dlp/Ollama prerequisites and provide actionable setup errors.
- Expand the opt-in real-pipeline integration test with deterministic adapter fixtures for CI.

Exit criterion: a user can reliably compile, inspect, edit, and revisit a long tutorial without using a terminal.

## Phase 2 — richer ingestion and output

- Add local video ingestion and a Whisper transcription adapter.
- Add screenshot extraction as a separate media-enrichment stage.
- Add a vision-provider interface and Gemma-family implementation.
- Add PDF export behind an exporter interface.
- Support playlists as batch/job composition rather than special-casing YouTube.

Exit criterion: source ingestion and exports can be extended without changing the core pipeline or guide domain.

## Phase 3 — learning tools

- Derive flashcards and quizzes from saved, verified guide concepts.
- Add spaced/progressive learning sessions and local progress storage.
- Track derivation provenance so generated questions link back to guide steps and timestamps.

Exit criterion: learning artifacts are reproducible, editable, and traceable to source evidence.

## Phase 4 — source ecosystem

- Add PDF, blog/documentation, and permitted course-platform adapters.
- Add source capability metadata (text, timestamps, images, chapters) so the pipeline selects applicable stages.
- Add plugin-style registration and versioned schema migrations when third-party extensions become necessary.
