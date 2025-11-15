package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	CacheDir       string `yaml:"cache_dir"`
	OutputDir      string `yaml:"output_dir"`
	MaxFileSize    int64  `yaml:"max_file_size"`
	ExcludePattern string `yaml:"exclude_pattern"`
	LogLevel       string `yaml:"log_level"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".context-vacuum")

	return &Config{
		CacheDir:       cacheDir,
		OutputDir:      filepath.Join(cacheDir, "output"),
		MaxFileSize:    10 * 1024 * 1024, // 10MB
		ExcludePattern: "*.test.ts,*.spec.ts,node_modules/*",
		LogLevel:       "warn",
	}
}

// Load loads configuration from file, or creates default if not exists
func Load(configPath string) (*Config, error) {
	// If config doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := cfg.Save(configPath); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save saves configuration to file
func (c *Config) Save(configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// CacheDBPath returns the path to the cache database
func (c *Config) CacheDBPath() string {
	return filepath.Join(c.CacheDir, "cache.db")
}

// PresetsDir returns the presets directory path
func (c *Config) PresetsDir() string {
	return filepath.Join(c.CacheDir, "presets")
}
