package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/brojonat/context-vacuum/internal/config"
	"github.com/brojonat/context-vacuum/internal/generator"
	"github.com/brojonat/context-vacuum/internal/parser"
	"github.com/brojonat/context-vacuum/internal/storage"
	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
	"github.com/brojonat/context-vacuum/internal/tui"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "context-vacuum",
		Usage: "Blazingly fast CLI tool to dynamically generate LLM context files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Value:   filepath.Join(os.Getenv("HOME"), ".context-vacuum", "config.yaml"),
				Usage:   "Path to config file",
				EnvVars: []string{"CONTEXT_VACUUM_CONFIG"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "warn",
				Usage:   "Log level (debug, info, warn, error)",
				EnvVars: []string{"LOG_LEVEL"},
			},
		},
		Before: setupApp,
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Add file/URL to cache DB",
				ArgsUsage: "<source>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Friendly name for the source",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "enabled",
						Value: true,
						Usage: "Enable source for context generation",
					},
				},
				Action: addSource,
			},
			{
				Name:      "remove",
				Usage:     "Remove source from cache DB",
				ArgsUsage: "<name>",
				Action:    removeSource,
			},
			{
				Name:      "toggle-on",
				Usage:     "Enable source for context generation",
				ArgsUsage: "<name>",
				Action:    toggleOn,
			},
			{
				Name:      "toggle-off",
				Usage:     "Disable source from context generation",
				ArgsUsage: "<name>",
				Action:    toggleOff,
			},
			{
				Name:   "list",
				Usage:  "List all cached sources with status",
				Action: listSources,
			},
			{
				Name:  "generate",
				Usage: "Create context from enabled sources",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "output",
						Value: "",
						Usage: "Output file path (default: stdout)",
					},
					&cli.StringFlag{
						Name:  "format",
						Value: "claude",
						Usage: "Output format (claude, cursor, default)",
					},
				},
				Action: generateContext,
			},
			{
				Name:      "import-bookmarks",
				Usage:     "Import bookmarks into cache DB",
				ArgsUsage: "<file>",
				Action:    importBookmarks,
			},
			{
				Name:   "tui",
				Usage:  "Launch interactive terminal UI",
				Action: launchTUI,
			},
		},
		Action: launchTUI, // Default action if no command is provided
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// setupApp initializes logging before commands run
func setupApp(c *cli.Context) error {
	// Parse log level
	logLevel := parseLogLevel(c.String("log-level"))

	// Setup logger
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, opts))
	slog.SetDefault(logger)

	return nil
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// getStoreAndConfig initializes store and config from context
func getStoreAndConfig(c *cli.Context) (*storage.Store, *config.Config, error) {
	// Load config
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store
	logger := slog.Default()
	store, err := storage.NewStore(cfg.CacheDBPath(), logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return store, cfg, nil
}

func addSource(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("requires exactly one argument: <source>")
	}

	source := c.Args().First()
	name := c.String("name")
	enabled := c.Bool("enabled")

	store, cfg, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	logger := slog.Default()

	// Determine source type and parse content
	p := parser.NewParser(cfg.MaxFileSize)
	var content string
	var sourceType string

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		sourceType = "url"
		content, err = p.ParseURL(source)
		if err != nil {
			return fmt.Errorf("failed to parse URL: %w", err)
		}
	} else {
		sourceType = "file"
		absPath, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("failed to resolve path: %w", err)
		}
		source = absPath
		content, err = p.ParseFile(source)
		if err != nil {
			return fmt.Errorf("failed to parse file: %w", err)
		}
	}

	// Compute hash
	hash := storage.ComputeHash(content)

	// Check if already exists
	existing, err := store.Queries().GetSourceByName(ctx, name)
	if err == nil {
		// Source exists, update it
		logger.DebugContext(ctx, "updating existing source", "name", name)
		if err := store.Queries().UpdateSourceContent(ctx, dbgen.UpdateSourceContentParams{
			Content: content,
			Hash:    hash,
			ID:      existing.ID,
		}); err != nil {
			return fmt.Errorf("failed to update source: %w", err)
		}
		fmt.Printf("Updated source: %s\n", name)
		return nil
	}

	// Create new source
	enabledInt := int64(0)
	if enabled {
		enabledInt = 1
	}

	created, err := store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       name,
		SourceType: sourceType,
		Path:       source,
		Content:    content,
		Hash:       hash,
		Enabled:    enabledInt,
	})
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}

	logger.InfoContext(ctx, "source added",
		"name", name,
		"type", sourceType,
		"enabled", enabled,
	)

	fmt.Printf("Added source: %s (ID: %d)\n", created.Name, created.ID)
	return nil
}

func removeSource(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("requires exactly one argument: <name>")
	}

	name := c.Args().First()

	store, _, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	logger := slog.Default()

	if err := store.Queries().DeleteSource(ctx, name); err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	logger.InfoContext(ctx, "source removed", "name", name)
	fmt.Printf("Removed source: %s\n", name)
	return nil
}

func toggleOn(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("requires exactly one argument: <name>")
	}

	name := c.Args().First()

	store, _, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.Queries().UpdateSourceEnabled(ctx, dbgen.UpdateSourceEnabledParams{
		Enabled: 1,
		Name:    name,
	}); err != nil {
		return fmt.Errorf("failed to enable source: %w", err)
	}

	fmt.Printf("Enabled source: %s\n", name)
	return nil
}

func toggleOff(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("requires exactly one argument: <name>")
	}

	name := c.Args().First()

	store, _, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.Queries().UpdateSourceEnabled(ctx, dbgen.UpdateSourceEnabledParams{
		Enabled: 0,
		Name:    name,
	}); err != nil {
		return fmt.Errorf("failed to disable source: %w", err)
	}

	fmt.Printf("Disabled source: %s\n", name)
	return nil
}

func listSources(c *cli.Context) error {
	store, _, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	sources, err := store.Queries().ListSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sources: %w", err)
	}

	if len(sources) == 0 {
		fmt.Println("No sources found")
		return nil
	}

	fmt.Printf("%-5s %-30s %-10s %-10s %s\n", "ID", "Name", "Type", "Enabled", "Path")
	fmt.Println(strings.Repeat("-", 80))

	for _, source := range sources {
		enabled := "no"
		if source.Enabled == 1 {
			enabled = "yes"
		}
		fmt.Printf("%-5d %-30s %-10s %-10s %s\n",
			source.ID,
			truncate(source.Name, 30),
			source.SourceType,
			enabled,
			truncate(source.Path, 40),
		)
	}

	return nil
}

func generateContext(c *cli.Context) error {
	outputPath := c.String("output")
	format := c.String("format")

	store, cfg, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	logger := slog.Default()

	// Create parser
	p := parser.NewParser(cfg.MaxFileSize)

	// Create generator
	gen := generator.NewGenerator(store, p, logger)

	// If no output specified, print to stdout
	if outputPath == "" || outputPath == "-" {
		content, err := gen.GenerateToString(ctx, generator.GenerateOptions{
			Format: format,
		})
		if err != nil {
			return fmt.Errorf("failed to generate context: %w", err)
		}
		fmt.Print(content)
		return nil
	}

	// Make output path absolute relative to current directory
	if !filepath.IsAbs(outputPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		outputPath = filepath.Join(cwd, outputPath)
	}

	// Generate context to file
	if err := gen.Generate(ctx, generator.GenerateOptions{
		OutputPath: outputPath,
		Format:     format,
	}); err != nil {
		return fmt.Errorf("failed to generate context: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Context generated: %s\n", outputPath)
	return nil
}

func importBookmarks(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("requires exactly one argument: <file>")
	}

	bookmarkFile := c.Args().First()

	store, cfg, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	logger := slog.Default()

	// Parse bookmarks
	p := parser.NewParser(cfg.MaxFileSize)
	bookmarks, err := p.ParseBookmarkHTML(bookmarkFile)
	if err != nil {
		return fmt.Errorf("failed to parse bookmarks: %w", err)
	}

	logger.InfoContext(ctx, "importing bookmarks", "count", len(bookmarks))

	// Add each bookmark as a source (disabled by default)
	imported := 0
	for _, bookmark := range bookmarks {
		// Try to fetch content
		content, err := p.ParseURL(bookmark.URL)
		if err != nil {
			logger.WarnContext(ctx, "failed to fetch bookmark",
				"title", bookmark.Title,
				"url", bookmark.URL,
				"error", err,
			)
			continue
		}

		hash := storage.ComputeHash(content)

		// Check if already exists by hash
		if _, err := store.Queries().GetSourceByHash(ctx, hash); err == nil {
			logger.DebugContext(ctx, "bookmark already exists", "title", bookmark.Title)
			continue
		}

		// Create source
		_, err = store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
			Name:       bookmark.Title,
			SourceType: "bookmark",
			Path:       bookmark.URL,
			Content:    content,
			Hash:       hash,
			Enabled:    0, // Disabled by default
		})
		if err != nil {
			logger.WarnContext(ctx, "failed to create source",
				"title", bookmark.Title,
				"error", err,
			)
			continue
		}

		imported++
	}

	fmt.Printf("Imported %d/%d bookmarks\n", imported, len(bookmarks))
	return nil
}

func launchTUI(c *cli.Context) error {
	store, _, err := getStoreAndConfig(c)
	if err != nil {
		return err
	}
	defer store.Close()

	return tui.Run(store)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
