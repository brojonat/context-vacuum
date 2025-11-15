package generator_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brojonat/context-vacuum/internal/generator"
	"github.com/brojonat/context-vacuum/internal/parser"
	"github.com/brojonat/context-vacuum/internal/storage"
	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
)

func setupTestGenerator(t *testing.T) (*generator.Generator, *storage.Store, func()) {
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

	p := parser.NewParser(10 * 1024 * 1024) // 10MB
	gen := generator.NewGenerator(store, p, logger)

	cleanup := func() {
		store.Close()
	}

	return gen, store, cleanup
}

func TestGenerator_GenerateClaudeFormat(t *testing.T) {
	gen, store, cleanup := setupTestGenerator(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test content for generation"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add source to database
	hash := storage.ComputeHash(content)
	_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "test-source",
		SourceType: "file",
		Path:       testFile,
		Content:    content,
		Hash:       hash,
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Generate context
	outputPath := filepath.Join(tmpDir, "output.md")
	err = gen.Generate(ctx, generator.GenerateOptions{
		OutputPath: outputPath,
		Format:     "claude",
	})
	if err != nil {
		t.Fatalf("failed to generate context: %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output file was not created")
	}

	// Verify content
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Development Context") {
		t.Error("output does not contain expected header")
	}

	if !strings.Contains(outputStr, content) {
		t.Error("output does not contain source content")
	}

	if !strings.Contains(outputStr, "test-source") {
		t.Error("output does not contain source name")
	}
}

func TestGenerator_CacheRefresh(t *testing.T) {
	gen, store, cleanup := setupTestGenerator(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test file with initial content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "initial content"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add source with initial content
	initialHash := storage.ComputeHash(initialContent)
	_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "refresh-test",
		SourceType: "file",
		Path:       testFile,
		Content:    initialContent,
		Hash:       initialHash,
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Generate with initial content
	outputPath1 := filepath.Join(tmpDir, "output1.md")
	if err := gen.Generate(ctx, generator.GenerateOptions{
		OutputPath: outputPath1,
		Format:     "default",
	}); err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	output1, _ := os.ReadFile(outputPath1)
	if !strings.Contains(string(output1), initialContent) {
		t.Error("first output should contain initial content")
	}

	// Modify the file
	updatedContent := "updated content"
	if err := os.WriteFile(testFile, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("failed to update test file: %v", err)
	}

	// Generate again - should detect cache miss and refresh
	outputPath2 := filepath.Join(tmpDir, "output2.md")
	if err := gen.Generate(ctx, generator.GenerateOptions{
		OutputPath: outputPath2,
		Format:     "default",
	}); err != nil {
		t.Fatalf("failed to generate after update: %v", err)
	}

	output2, _ := os.ReadFile(outputPath2)
	if !strings.Contains(string(output2), updatedContent) {
		t.Error("second output should contain updated content")
	}

	if strings.Contains(string(output2), initialContent) {
		t.Error("second output should not contain initial content")
	}

	// Verify cache was updated in database
	source, err := store.Queries().GetSourceByName(ctx, "refresh-test")
	if err != nil {
		t.Fatalf("failed to get source: %v", err)
	}

	if source.Content != updatedContent {
		t.Errorf("cached content should be updated, got %q, want %q", source.Content, updatedContent)
	}

	updatedHash := storage.ComputeHash(updatedContent)
	if source.Hash != updatedHash {
		t.Error("cached hash should be updated")
	}
}

func TestGenerator_NoEnabledSources(t *testing.T) {
	gen, store, cleanup := setupTestGenerator(t)
	defer cleanup()

	ctx := context.Background()

	// Create a disabled source
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash := storage.ComputeHash("test")
	_, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       "disabled-source",
		SourceType: "file",
		Path:       testFile,
		Content:    "test",
		Hash:       hash,
		Enabled:    0, // Disabled
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Try to generate - should fail with no enabled sources
	outputPath := filepath.Join(tmpDir, "output.md")
	err = gen.Generate(ctx, generator.GenerateOptions{
		OutputPath: outputPath,
		Format:     "claude",
	})

	if err == nil {
		t.Error("expected error when no enabled sources, got nil")
	}

	if !strings.Contains(err.Error(), "no enabled sources") {
		t.Errorf("expected 'no enabled sources' error, got: %v", err)
	}
}
