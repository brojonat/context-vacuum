package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brojonat/context-vacuum/internal/parser"
)

func TestParser_ParseFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test content\n"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	p := parser.NewParser(10 * 1024 * 1024) // 10MB

	parsed, err := p.ParseFile(testFile)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if parsed != content {
		t.Errorf("expected content %q, got %q", content, parsed)
	}
}

func TestParser_ParseFile_SizeLimit(t *testing.T) {
	// Create a temporary file that's too large
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	content := strings.Repeat("x", 1024)

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	p := parser.NewParser(512) // 512 bytes max

	_, err := p.ParseFile(testFile)
	if err == nil {
		t.Error("expected error for file exceeding size limit, got nil")
	}
}

func TestParser_ParseFile_NotFound(t *testing.T) {
	p := parser.NewParser(10 * 1024 * 1024)

	_, err := p.ParseFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// Note: extractTextFromHTML is tested indirectly through ParseURL
// when fetching HTML pages
