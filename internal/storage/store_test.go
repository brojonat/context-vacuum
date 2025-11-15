package storage_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/brojonat/context-vacuum/internal/storage"
	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
)

func setupTestStore(t *testing.T) (*storage.Store, func()) {
	t.Helper()

	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet during tests
	}))

	store, err := storage.NewStore(dbPath, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

func TestStore_CreateAndGetSource(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create a source
	content := "test content"
	hash := storage.ComputeHash(content)

	source, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "test-source",
		SourceType: "file",
		Path:       "/path/to/file",
		Content:    content,
		Hash:       hash,
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	if source.Name != "test-source" {
		t.Errorf("expected name 'test-source', got %s", source.Name)
	}

	if source.Enabled != 1 {
		t.Errorf("expected enabled to be 1, got %d", source.Enabled)
	}

	// Get the source
	retrieved, err := store.Queries().GetSourceByName(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to get source: %v", err)
	}

	if retrieved.ID != source.ID {
		t.Errorf("expected ID %d, got %d", source.ID, retrieved.ID)
	}

	if retrieved.Content != content {
		t.Errorf("expected content %s, got %s", content, retrieved.Content)
	}
}

func TestStore_UpdateSourceEnabled(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create a source (enabled by default)
	content := "test content"
	hash := storage.ComputeHash(content)

	_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "test-source",
		SourceType: "file",
		Path:       "/path/to/file",
		Content:    content,
		Hash:       hash,
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Disable it
	err = store.Queries().UpdateSourceEnabled(ctx, dbgen.UpdateSourceEnabledParams{
		Enabled: 0,
		Name:    "test-source",
	})
	if err != nil {
		t.Fatalf("failed to update source: %v", err)
	}

	// Verify it's disabled
	source, err := store.Queries().GetSourceByName(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to get source: %v", err)
	}

	if source.Enabled != 0 {
		t.Errorf("expected enabled to be 0, got %d", source.Enabled)
	}
}

func TestStore_ListEnabledSources(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sources with different enabled states
	sources := []struct {
		name    string
		enabled int64
	}{
		{"enabled-1", 1},
		{"disabled-1", 0},
		{"enabled-2", 1},
		{"disabled-2", 0},
	}

	for _, s := range sources {
		hash := storage.ComputeHash(s.name)
		_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
			Name:       s.name,
			SourceType: "file",
			Path:       "/path/" + s.name,
			Content:    s.name,
			Hash:       hash,
			Enabled:    s.enabled,
		})
		if err != nil {
			t.Fatalf("failed to create source %s: %v", s.name, err)
		}
	}

	// List only enabled sources
	enabled, err := store.Queries().ListEnabledSources(ctx)
	if err != nil {
		t.Fatalf("failed to list enabled sources: %v", err)
	}

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled sources, got %d", len(enabled))
	}

	for _, s := range enabled {
		if s.Enabled != 1 {
			t.Errorf("expected source %s to be enabled", s.Name)
		}
	}
}

func TestStore_DeleteSource(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create a source
	hash := storage.ComputeHash("test")
	_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "test-source",
		SourceType: "file",
		Path:       "/path/to/file",
		Content:    "test",
		Hash:       hash,
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Delete it
	err = store.Queries().DeleteSource(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to delete source: %v", err)
	}

	// Verify it's gone
	_, err = store.Queries().GetSourceByName(ctx, "test-source")
	if err == nil {
		t.Error("expected error when getting deleted source, got nil")
	}
}

func TestComputeHash(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty string",
			content:  "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello world",
			content:  "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := storage.ComputeHash(tt.content)
			if hash != tt.expected {
				t.Errorf("expected hash %s, got %s", tt.expected, hash)
			}
		})
	}
}
