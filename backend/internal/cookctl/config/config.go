// Package config loads cookctl configuration from defaults, files, and env.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultAPIURL  = "http://localhost:8080"
	defaultTimeout = 30 * time.Second
)

// OutputFormat describes the CLI output mode.
type OutputFormat string

const (
	// OutputTable renders tabular output for humans.
	OutputTable OutputFormat = "table"
	// OutputJSON renders raw JSON for automation.
	OutputJSON OutputFormat = "json"
)

// Config holds runtime configuration for cookctl.
type Config struct {
	APIURL  string
	Output  OutputFormat
	Timeout time.Duration
	Debug   bool
}

type fileConfig struct {
	APIURL  string `toml:"api_url"`
	Output  string `toml:"output"`
	Timeout string `toml:"timeout"`
	Debug   *bool  `toml:"debug"`
}

// Default returns the baseline CLI configuration.
func Default() Config {
	return Config{
		APIURL:  defaultAPIURL,
		Output:  OutputTable,
		Timeout: defaultTimeout,
		Debug:   false,
	}
}

// ParseOutput validates and converts a string to an OutputFormat.
func ParseOutput(value string) (OutputFormat, error) {
	switch OutputFormat(strings.ToLower(strings.TrimSpace(value))) {
	case OutputTable:
		return OutputTable, nil
	case OutputJSON:
		return OutputJSON, nil
	default:
		return "", fmt.Errorf("invalid output format: %q", value)
	}
}

// Set implements flag.Value for OutputFormat.
func (o *OutputFormat) Set(value string) error {
	parsed, err := ParseOutput(value)
	if err != nil {
		return err
	}
	*o = parsed
	return nil
}

// String returns the string representation of the output format.
func (o OutputFormat) String() string {
	return string(o)
}

// DefaultDir returns the OS-specific configuration directory for cookctl.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(base, "cookctl"), nil
}

// DefaultConfigPath returns the OS-specific path to config.toml.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// LoadFile reads configuration from the provided path (or default path when empty).
// It does not apply environment overrides.
func LoadFile(path string) (Config, error) {
	configPath := path
	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return Config{}, err
		}
	}

	fileCfg, err := readFileConfig(configPath)
	if err != nil {
		return Config{}, err
	}
	return applyFileConfig(Default(), fileCfg)
}

// Load reads config from the provided path (or the default path when empty),
// applies environment overrides, and returns the merged config.
func Load(path string) (Config, error) {
	cfg := Default()

	configPath := path
	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return Config{}, err
		}
	}

	fileCfg, err := readFileConfig(configPath)
	if err != nil {
		return Config{}, err
	}
	cfg, err = applyFileConfig(cfg, fileCfg)
	if err != nil {
		return Config{}, err
	}

	if value := os.Getenv("COOKING_API_URL"); value != "" {
		cfg.APIURL = value
	}
	if value := os.Getenv("COOKING_OUTPUT"); value != "" {
		output, err := ParseOutput(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Output = output
	}
	if value := os.Getenv("COOKING_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid COOKING_TIMEOUT: %w", err)
		}
		cfg.Timeout = timeout
	}

	return cfg, nil
}

// Save writes the provided configuration to the given path (or default path when empty).
func Save(path string, cfg Config) error {
	configPath := path
	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	if cfg.APIURL == "" {
		return errors.New("api url is required")
	}
	if cfg.Output != OutputTable && cfg.Output != OutputJSON {
		return errors.New("invalid output format")
	}
	if cfg.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	debug := cfg.Debug
	fileCfg := fileConfig{
		APIURL:  cfg.APIURL,
		Output:  string(cfg.Output),
		Timeout: cfg.Timeout.String(),
		Debug:   &debug,
	}
	raw, err := toml.Marshal(fileCfg)
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

// readFileConfig loads config.toml if present and returns zero values otherwise.
func readFileConfig(path string) (fileConfig, error) {
	//nolint:gosec // Path is user-configured by design for loading config.
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg fileConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("parse config file: %w", err)
	}
	return cfg, nil
}

// applyFileConfig overlays file-based values onto the provided config.
func applyFileConfig(cfg Config, fileCfg fileConfig) (Config, error) {
	if fileCfg.APIURL != "" {
		cfg.APIURL = fileCfg.APIURL
	}
	if fileCfg.Output != "" {
		output, err := ParseOutput(fileCfg.Output)
		if err != nil {
			return Config{}, err
		}
		cfg.Output = output
	}
	if fileCfg.Timeout != "" {
		timeout, err := time.ParseDuration(fileCfg.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("invalid timeout: %w", err)
		}
		cfg.Timeout = timeout
	}
	if fileCfg.Debug != nil {
		cfg.Debug = *fileCfg.Debug
	}

	return cfg, nil
}
