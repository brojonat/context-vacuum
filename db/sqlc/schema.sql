-- SQLite schema for context-vacuum

-- sources table: stores all cached files and URLs
CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    source_type TEXT NOT NULL CHECK(source_type IN ('file', 'url', 'bookmark')),
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    hash TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0, 1)),
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- Create index on enabled for fast filtering
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);

-- Create index on name for lookups
CREATE INDEX IF NOT EXISTS idx_sources_name ON sources(name);

-- Create index on hash for duplicate detection
CREATE INDEX IF NOT EXISTS idx_sources_hash ON sources(hash);

-- presets table: stores named collections of enabled sources
CREATE TABLE IF NOT EXISTS presets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- preset_sources: junction table for many-to-many relationship
CREATE TABLE IF NOT EXISTS preset_sources (
    preset_id INTEGER NOT NULL,
    source_id INTEGER NOT NULL,
    PRIMARY KEY (preset_id, source_id),
    FOREIGN KEY (preset_id) REFERENCES presets(id) ON DELETE CASCADE,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);

-- history table: track generated contexts
CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    preset_name TEXT,
    output_path TEXT NOT NULL,
    source_count INTEGER NOT NULL,
    generated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- Create index on generated_at for sorting
CREATE INDEX IF NOT EXISTS idx_history_generated_at ON history(generated_at DESC);
