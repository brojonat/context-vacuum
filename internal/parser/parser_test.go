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

func TestParser_ParseURL_HTML(t *testing.T) {
	// This test requires network access and hits a real URL
	// Skip in CI or when -short flag is provided
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := parser.NewParser(10 * 1024 * 1024)

	// Test parsing a real HTML page
	content, err := p.ParseURL("https://brojonat.com/posts/uv/")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	// Verify the content has semantic structure
	if !strings.Contains(content, "# Snippets") {
		t.Error("expected markdown heading to be preserved")
	}

	if !strings.Contains(content, "```") {
		t.Error("expected code blocks to be preserved")
	}

	if !strings.Contains(content, "uv") {
		t.Error("expected content to contain 'uv'")
	}

	// Verify navigation elements are excluded
	if strings.Contains(content, "menu\nApps\nConsultation") {
		t.Error("expected navigation elements to be excluded")
	}

	// Content should be readable paragraphs, not fragmented
	lines := strings.Split(content, "\n")
	fragmentedLines := 0
	for _, line := range lines {
		// Count very short lines (< 10 chars) as fragmented
		if len(strings.TrimSpace(line)) > 0 && len(strings.TrimSpace(line)) < 10 {
			fragmentedLines++
		}
	}
	// Should have relatively few fragmented lines (< 20% of total)
	if float64(fragmentedLines)/float64(len(lines)) > 0.2 {
		t.Errorf("content appears too fragmented: %d/%d lines are very short", fragmentedLines, len(lines))
	}
}

// Note: extractTextFromHTML is tested indirectly through ParseURL
// when fetching HTML pages
