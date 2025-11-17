package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/reallyoldfogie/mc-protocol-go/internal/generator"
)

func main() {
	// Define CLI flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	versions := flag.String("versions", "", "Comma-separated list of MC versions to generate (overrides config)")
	outputDir := flag.String("output-dir", "", "Output directory for generated code (overrides config)")
	cacheDir := flag.String("cache-dir", "", "Cache directory for index files (overrides config)")
	metadataDir := flag.String("metadata-dir", "", "Metadata directory for downloads (overrides config)")
	memoryLimit := flag.String("memory-limit", "", "Memory limit (e.g., 512M, 1G) (overrides config)")
	gcPercent := flag.Int("gc-percent", 0, "GC percent (overrides config, 0 means use config value)")
	maxProcs := flag.Int("max-procs", 0, "Maximum number of OS threads (overrides config, 0 means use config value)")

	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override config with CLI flags
	if *versions != "" {
		cfg.Versions = strings.Split(*versions, ",")
		// Trim spaces from version strings
		for i := range cfg.Versions {
			cfg.Versions[i] = strings.TrimSpace(cfg.Versions[i])
		}
	}

	if *outputDir != "" {
		cfg.Output.DataDir = *outputDir
	}

	if *cacheDir != "" {
		cfg.Cache.CacheDir = *cacheDir
	}

	if *metadataDir != "" {
		cfg.Cache.MetadataDir = *metadataDir
	}

	if *memoryLimit != "" {
		cfg.Memory.MemoryLimit = *memoryLimit
	}

	if *gcPercent > 0 {
		cfg.Memory.GCPercent = *gcPercent
	}

	if *maxProcs > 0 {
		cfg.Memory.MaxProcs = *maxProcs
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation error: %v\n", err)
		os.Exit(1)
	}

	// Run generator
	fmt.Println("=== Minecraft Protocol Generator ===")
	fmt.Printf("Versions: %v\n", cfg.Versions)
	fmt.Printf("Output: %s\n", cfg.Output.DataDir)
	fmt.Printf("Cache: %s\n", cfg.Cache.CacheDir)
	fmt.Printf("Metadata: %s\n", cfg.Cache.MetadataDir)
	fmt.Println("=====================================")
	fmt.Println()

	if err := generator.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nGeneration completed successfully!")
}

func loadConfig(path string) (*generator.Config, error) {
	// Try to load config file
	cfg, err := generator.LoadConfig(path)
	if err != nil {
		// If config file doesn't exist, use default config
		if os.IsNotExist(err) {
			fmt.Printf("Config file not found at %s, using default configuration\n", path)
			return generator.DefaultConfig(), nil
		}
		return nil, err
	}
	return cfg, nil
}
