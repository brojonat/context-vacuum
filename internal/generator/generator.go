package generator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brojonat/context-vacuum/internal/parser"
	"github.com/brojonat/context-vacuum/internal/storage"
	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
)

// Generator combines enabled sources into a context file
type Generator struct {
	store  *storage.Store
	parser *parser.Parser
	logger *slog.Logger
}

// NewGenerator creates a new Generator with explicit dependencies
func NewGenerator(store *storage.Store, parser *parser.Parser, logger *slog.Logger) *Generator {
	return &Generator{
		store:  store,
		parser: parser,
		logger: logger,
	}
}

// GenerateOptions holds options for context generation
type GenerateOptions struct {
	OutputPath string
	Format     string // "claude", "cursor", or custom
	PresetName string
}

// GenerateToString creates context content and returns it as a string
func (g *Generator) GenerateToString(ctx context.Context, opts GenerateOptions) (string, error) {
	// Query DB for enabled sources
	sources, err := g.store.Queries().ListEnabledSources(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list enabled sources: %w", err)
	}

	if len(sources) == 0 {
		return "", fmt.Errorf("no enabled sources found")
	}

	g.logger.DebugContext(ctx, "generating context to string",
		"source_count", len(sources),
	)

	// Check for cache misses and parse fresh content if needed
	updatedSources, err := g.checkAndRefreshCache(ctx, sources)
	if err != nil {
		return "", fmt.Errorf("failed to refresh cache: %w", err)
	}

	// Generate content based on format
	var content string
	switch opts.Format {
	case "claude", "":
		content = g.generateClaudeFormat(updatedSources)
	case "cursor":
		content = g.generateCursorFormat(updatedSources)
	default:
		content = g.generateDefaultFormat(updatedSources)
	}

	return content, nil
}

// Generate creates a context file from all enabled sources
func (g *Generator) Generate(ctx context.Context, opts GenerateOptions) error {
	// Query DB for enabled sources
	sources, err := g.store.Queries().ListEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list enabled sources: %w", err)
	}

	if len(sources) == 0 {
		return fmt.Errorf("no enabled sources found")
	}

	g.logger.DebugContext(ctx, "generating context",
		"source_count", len(sources),
		"output_path", opts.OutputPath,
	)

	// Check for cache misses and parse fresh content if needed
	updatedSources, err := g.checkAndRefreshCache(ctx, sources)
	if err != nil {
		return fmt.Errorf("failed to refresh cache: %w", err)
	}

	// Generate content based on format
	var content string
	switch opts.Format {
	case "claude", "":
		content = g.generateClaudeFormat(updatedSources)
	case "cursor":
		content = g.generateCursorFormat(updatedSources)
	default:
		content = g.generateDefaultFormat(updatedSources)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(opts.OutputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(opts.OutputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Record in history
	presetName := sql.NullString{
		String: opts.PresetName,
		Valid:  opts.PresetName != "",
	}
	_, err = g.store.Queries().CreateHistory(ctx, dbgen.CreateHistoryParams{
		PresetName:  presetName,
		OutputPath:  opts.OutputPath,
		SourceCount: int64(len(updatedSources)),
	})
	if err != nil {
		g.logger.WarnContext(ctx, "failed to record history", "error", err)
	}

	g.logger.InfoContext(ctx, "context generated",
		"source_count", len(updatedSources),
		"output_path", opts.OutputPath,
	)

	return nil
}

// checkAndRefreshCache checks each source for cache misses and refreshes content if needed
func (g *Generator) checkAndRefreshCache(ctx context.Context, sources []dbgen.Source) ([]dbgen.Source, error) {
	updatedSources := make([]dbgen.Source, 0, len(sources))

	for _, source := range sources {
		needsRefresh, freshContent, err := g.detectCacheMiss(ctx, source)
		if err != nil {
			g.logger.WarnContext(ctx, "failed to check cache miss, using cached content",
				"source", source.Name,
				"error", err,
			)
			// Use cached content if check fails
			updatedSources = append(updatedSources, source)
			continue
		}

		if needsRefresh {
			// Parse extracted fresh content and store in cache
			hash := storage.ComputeHash(freshContent)
			if err := g.store.Queries().UpdateSourceContent(ctx, dbgen.UpdateSourceContentParams{
				Content: freshContent,
				Hash:    hash,
				ID:      source.ID,
			}); err != nil {
				g.logger.WarnContext(ctx, "failed to update cache, using cached content",
					"source", source.Name,
					"error", err,
				)
				updatedSources = append(updatedSources, source)
				continue
			}

			g.logger.DebugContext(ctx, "refreshed cache",
				"source", source.Name,
				"type", source.SourceType,
			)

			// Update source with fresh content
			source.Content = freshContent
			source.Hash = hash
		}

		updatedSources = append(updatedSources, source)
	}

	return updatedSources, nil
}

// detectCacheMiss checks if a source needs to be refreshed
// Returns: (needsRefresh, freshContent, error)
func (g *Generator) detectCacheMiss(ctx context.Context, source dbgen.Source) (bool, string, error) {
	switch source.SourceType {
	case "file":
		// For files, parse and compare hash
		content, err := g.parser.ParseFile(source.Path)
		if err != nil {
			return false, "", fmt.Errorf("failed to parse file: %w", err)
		}

		currentHash := storage.ComputeHash(content)
		if currentHash != source.Hash {
			return true, content, nil
		}

		return false, "", nil

	case "url", "bookmark":
		// For URLs, always re-fetch to check for changes
		content, err := g.parser.ParseURL(source.Path)
		if err != nil {
			return false, "", fmt.Errorf("failed to parse URL: %w", err)
		}

		currentHash := storage.ComputeHash(content)
		if currentHash != source.Hash {
			return true, content, nil
		}

		return false, "", nil

	default:
		return false, "", fmt.Errorf("unknown source type: %s", source.SourceType)
	}
}

// generateClaudeFormat generates content in Claude.md format
func (g *Generator) generateClaudeFormat(sources []dbgen.Source) string {
	var sb strings.Builder

	sb.WriteString("# Development Context\n\n")
	sb.WriteString("The following content consists of curated context for LLM assistants.\n\n")
	sb.WriteString("---\n\n")

	for i, source := range sources {
		sb.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, source.Name))
		sb.WriteString(fmt.Sprintf("**Source:** %s (%s)\n\n", source.Path, source.SourceType))
		sb.WriteString("```\n")
		sb.WriteString(source.Content)
		sb.WriteString("\n```\n\n")
		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// generateCursorFormat generates content in Cursor format
func (g *Generator) generateCursorFormat(sources []dbgen.Source) string {
	var sb strings.Builder

	sb.WriteString("# Cursor Context\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	for _, source := range sources {
		sb.WriteString(fmt.Sprintf("## %s\n\n", source.Name))
		sb.WriteString(source.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// generateDefaultFormat generates content in default format
func (g *Generator) generateDefaultFormat(sources []dbgen.Source) string {
	var sb strings.Builder

	for i, source := range sources {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("=== %s ===\n", source.Name))
		sb.WriteString(source.Content)
	}

	return sb.String()
}
