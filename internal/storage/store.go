package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
	_ "github.com/mattn/go-sqlite3"
)

// Store manages all database operations for context-vacuum
type Store struct {
	db      *sql.DB
	queries *dbgen.Queries
	logger  *slog.Logger
}

// NewStore creates a new Store with explicit dependencies
func NewStore(dbPath string, logger *slog.Logger) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open SQLite connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	queries := dbgen.New(db)

	logger.DebugContext(context.Background(), "database initialized",
		"path", dbPath,
	)

	return &Store{
		db:      db,
		queries: queries,
		logger:  logger,
	}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for transactions
func (s *Store) DB() *sql.DB {
	return s.db
}

// Queries returns the generated queries interface
func (s *Store) Queries() *dbgen.Queries {
	return s.queries
}

// ComputeHash computes SHA256 hash of content
func ComputeHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// initSchema initializes the database schema
func initSchema(db *sql.DB) error {
	schema := `
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
`

	_, err := db.Exec(schema)
	return err
}
