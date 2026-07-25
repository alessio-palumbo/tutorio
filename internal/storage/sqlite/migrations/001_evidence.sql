CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    locator TEXT NOT NULL,
    title TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sources_fingerprint ON sources(fingerprint);

CREATE TABLE IF NOT EXISTS source_chunks (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    text TEXT NOT NULL,
    physical_page INTEGER NOT NULL DEFAULT 0,
    printed_label TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL,
    UNIQUE(source_id, sequence)
);
CREATE INDEX IF NOT EXISTS idx_source_chunks_location ON source_chunks(source_id, physical_page, sequence);

CREATE TABLE IF NOT EXISTS evidence (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    chunk_id TEXT NOT NULL REFERENCES source_chunks(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    UNIQUE(chunk_id)
);
CREATE INDEX IF NOT EXISTS idx_evidence_source ON evidence(source_id);
