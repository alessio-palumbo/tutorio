# Evidence architecture decision

Tutorio treats evidence preview as the primary citation interaction. Opening the original source remains a secondary fallback for layout, figures, tables, and broader context.

For PDF citations, the drawer may also request an on-demand raster preview of the cited physical page. The preview is rendered locally from the registered source after citation authorization. It supplements exact text evidence; it is not interpreted by a vision model and is not presented as a figure-specific citation.

## Model

- A registered `Source` owns an opaque ID, local locator, title, kind, and content fingerprint. Locators never cross the frontend boundary.
- A `SourceChunk` is an immutable extracted unit. It may represent prose, a heading, code, or a future table fragment; it is not assumed to be a paragraph.
- `Evidence` gives a source chunk a stable, source-neutral evidence identity.
- Guide steps contain citations to evidence IDs. Citation support classification belongs to the citation because the same evidence can support different claims differently.
- Neighbour context is resolved by source and extraction sequence rather than copied into each evidence record.

PDF locations use the physical PDF page index. Printed page labels are stored separately only when actually extracted.

## Identity and immutability

PDF source IDs derive from a content fingerprint. Chunk IDs derive from the source fingerprint, extractor version, physical page, chunk kind, and normalized text. Extraction sequence is deliberately excluded from identity.

Model-returned chunk IDs are treated as untrusted input. The generator accepts only IDs supplied in that request, removes duplicates, applies a per-step maximum, and never invents a citation for unsupported content.

## Compatibility

Existing `SourceReference`, timestamp, and `SourceExcerpt` fields remain readable. Older page-only references open an evidence panel with source and page metadata but no excerpt. Because their persisted source sections predate durable chunks, the PDF must be recompiled to add exact evidence; the original saved guide remains readable.
