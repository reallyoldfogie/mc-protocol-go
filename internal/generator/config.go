package generator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the generator configuration
type Config struct {
	Versions []string     `yaml:"versions"`
	Output   OutputConfig `yaml:"output"`
	Cache    CacheConfig  `yaml:"cache"`
	Memory   MemoryConfig `yaml:"memory"`
}

// OutputConfig defines output directory settings
type OutputConfig struct {
	DataDir string `yaml:"data_dir"`
}

// CacheConfig defines cache directory settings
type CacheConfig struct {
	CacheDir    string `yaml:"cache_dir"`    // Index/lookup files
	MetadataDir string `yaml:"metadata_dir"` // Downloaded/extracted MC data
	TTLDays     int    `yaml:"ttl_days"`
}

// MemoryConfig defines memory optimization settings
type MemoryConfig struct {
	GCPercent   int    `yaml:"gc_percent"`
	MemoryLimit string `yaml:"memory_limit"`
	MaxProcs    int    `yaml:"max_procs"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Versions: []string{"1.21.5"},
		Output: OutputConfig{
			DataDir: "data",
		},
		Cache: CacheConfig{
			CacheDir:    ".cache",
			MetadataDir: ".cache/metadata",
			TTLDays:     7,
		},
		Memory: MemoryConfig{
			GCPercent:   50,
			MemoryLimit: "512MiB",
			MaxProcs:    2,
		},
	}
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Versions) == 0 {
		return fmt.Errorf("at least one version must be specified")
	}

	if c.Output.DataDir == "" {
		return fmt.Errorf("output data_dir must be specified")
	}

	if c.Cache.CacheDir == "" {
		return fmt.Errorf("cache cache_dir must be specified")
	}

	if c.Cache.MetadataDir == "" {
		return fmt.Errorf("cache metadata_dir must be specified")
	}

	if c.Cache.TTLDays < 0 {
		return fmt.Errorf("cache ttl_days must be non-negative")
	}

	if c.Memory.GCPercent < 0 {
		return fmt.Errorf("memory gc_percent must be non-negative")
	}

	if c.Memory.MaxProcs < 1 {
		return fmt.Errorf("memory max_procs must be at least 1")
	}

	return nil
}
