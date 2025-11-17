# Refactoring Plan: go generate to Standalone Program

## Overview
This document outlines the refactoring of the mc-protocol-go generator from a `go generate` workflow to a standalone CLI application.

## Motivation
- **Better separation of concerns**: Generator tool code separate from library code
- **Easier distribution**: Can be installed and versioned independently
- **Configuration management**: External configuration files instead of hardcoded values
- **Improved maintainability**: Clear project structure following Go best practices
- **Explicit execution**: Users explicitly run the generator instead of implicit go generate

## Current State

### Architecture
- Generator runs via `//go:generate go run $GOFILE templates.go gen_packet.go getMCVersionData.go parsePackets.go`
- Multiple generator `.go` files in root directory mixed with library code
- Version configuration hardcoded in `gen_data.go` (lines 34-36)
- Generated packages output to `data/<version>/`
- Cache structure:
  - `.cache/<version>.json` - Index files mapping keys to file paths
  - `.cache/metadata/` - Downloaded and extracted Minecraft data

### Files to Refactor
- `gen_data.go` - Main orchestration and generation logic
- `gen_packet.go` - Packet generation logic  
- `templates.go` - Template definitions
- `getMCVersionData.go` - Minecraft version data downloading
- `parsePackets.go` - Protocol parsing from PrismarineJS data

## Target Architecture

### Project Structure
```
mc-protocol-go/
├── cmd/
│   └── generator/          # Standalone generator application
│       └── main.go
├── internal/
│   └── generator/          # Generator implementation code
│       ├── config.go       # Configuration types and loading
│       ├── generator.go    # Main orchestration (from gen_data.go)
│       ├── packets.go      # Packet generation (from gen_packet.go)
│       ├── blocks.go       # Block generation logic
│       ├── sounds.go       # Sound generation logic
│       ├── protocol.go     # Protocol struct generation
│       ├── templates.go    # Template definitions
│       ├── downloader.go   # MC version data fetching (from getMCVersionData.go)
│       └── parser.go       # Protocol parsing (from parsePackets.go)
├── configs/                # Configuration files
│   └── config.yaml         # Default generator configuration
├── data/                   # Generated version-specific packages (unchanged)
│   ├── <version>/         # e.g., 1.21.5/
│   │   ├── packetid.go
│   │   ├── blockid.go
│   │   └── soundid.go
│   └── versions/          # Version managers
│       ├── versionProtocol.go
│       ├── blockMgr.go
│       ├── soundMgr.go
│       └── packetMgr.go
├── models/                 # Existing models package (unchanged)
├── .cache/                 # Cache directory
│   ├── <version>.json     # Index files
│   └── metadata/          # Downloaded MC data
├── docs/                   # Documentation
└── go.mod
```

### Configuration System

#### Configuration File (`configs/config.yaml`)
```yaml
versions:
  - "1.21.5"
  # - "1.21.1"
  # - "1.21.2"

output:
  data_dir: "data"              # Generated Go packages

cache:
  cache_dir: ".cache"           # Index/lookup files
  metadata_dir: ".cache/metadata"  # Downloaded/extracted MC data
  ttl_days: 7                   # Cache expiration

memory:
  gc_percent: 50
  memory_limit: "512MiB"
  max_procs: 2
```

#### Configuration Types
```go
type Config struct {
    Versions []string       `yaml:"versions"`
    Output   OutputConfig   `yaml:"output"`
    Cache    CacheConfig    `yaml:"cache"`
    Memory   MemoryConfig   `yaml:"memory"`
}

type OutputConfig struct {
    DataDir string `yaml:"data_dir"`
}

type CacheConfig struct {
    CacheDir    string `yaml:"cache_dir"`     // Index files
    MetadataDir string `yaml:"metadata_dir"`  // Downloaded content
    TTLDays     int    `yaml:"ttl_days"`
}

type MemoryConfig struct {
    GCPercent   int    `yaml:"gc_percent"`
    MemoryLimit string `yaml:"memory_limit"`
    MaxProcs    int    `yaml:"max_procs"`
}
```

### CLI Interface

#### Usage Examples
```bash
# Generate using default config
generator

# Generate for specific versions
generator --versions 1.21.5,1.21.4

# Use custom config file
generator --config configs/config.yaml

# Override config values
generator --config configs/config.yaml --versions 1.21.5

# Specify output directory
generator --output-dir ./data

# Low memory mode (override config)
generator --low-memory --memory-limit 512M
```

#### CLI Flags
- `--config` / `-c`: Path to configuration file
- `--versions` / `-v`: Comma-separated list of MC versions to generate
- `--output-dir` / `-o`: Output directory for generated code
- `--cache-dir`: Cache directory for index files
- `--metadata-dir`: Metadata directory for downloads
- `--low-memory`: Enable low memory mode (sets aggressive GC)
- `--memory-limit`: Memory limit (e.g., 512M, 1G)

## Implementation Steps

### Phase 1: Structure Setup
1. Create directory structure
   - `mkdir -p cmd/generator internal/generator configs`
2. Create configuration package
   - `internal/generator/config.go` with types and loading functions
3. Create default config file
   - `configs/config.yaml` with sensible defaults

### Phase 2: Code Migration
4. Move and refactor generator core
   - `gen_data.go` → `internal/generator/generator.go`
   - Update to use Config struct instead of constants
   - Remove `//go:generate` directive
   - Change package from `main` to `generator`
5. Move supporting generator files
   - `gen_packet.go` → `internal/generator/packets.go`
   - `templates.go` → `internal/generator/templates.go`
   - `getMCVersionData.go` → `internal/generator/downloader.go`
   - `parsePackets.go` → `internal/generator/parser.go`
   - Split large files into logical units (blocks, sounds, protocol)
   - Update package declarations to `generator`
   - Export main functions (capitalize names)

### Phase 3: CLI Application
6. Create CLI application
   - `cmd/generator/main.go` with:
     - Flag parsing
     - Config file loading
     - Config merging (CLI flags override config file)
     - Call generator functions
     - Error handling and progress reporting

### Phase 4: Testing & Documentation
7. Update build/run scripts
   - Modify `run_gen_lowmem.sh` to call new CLI
8. Test generation workflow
   - Run with default config
   - Verify output in `data/` directory
   - Test CLI flag overrides
9. Update documentation
   - README.md with new usage instructions
   - Installation instructions
   - Migration guide for existing users

## Benefits

### For Development
- ✅ Clear separation between generator tool and generated library
- ✅ Generator can be versioned and distributed independently
- ✅ Easier to test generator logic in isolation
- ✅ Cleaner git history (generator changes separate from library changes)

### For Users
- ✅ Configuration via files instead of editing code
- ✅ CLI flags for quick overrides
- ✅ Can install as standalone tool: `go install ./cmd/generator`
- ✅ More discoverable and self-documenting

### For Maintenance
- ✅ Organized structure following Go best practices
- ✅ Adheres to established project rules for configuration
- ✅ Easier onboarding for new contributors
- ✅ Better IDE support with proper package boundaries

## Non-Breaking Changes

### What Stays the Same
- ✅ Generated packages remain at `github.com/reallyoldfogie/mc-protocol-go/data/<version>`
- ✅ No import path changes for consumers
- ✅ Models package unchanged
- ✅ Cache structure and behavior unchanged
- ✅ Generated code format unchanged

### What Changes
- ⚠️ Users must run generator explicitly instead of `go generate`
- ⚠️ Version configuration moves from code to config file
- ⚠️ Generator source files no longer in root directory

## Migration Path

### For Maintainers
1. Run new generator to verify output matches existing
2. Compare generated files to ensure equivalence
3. Remove old generator files from root after validation

### For Users
1. Update build scripts from `go generate` to `go run ./cmd/generator`
2. Create config file if customization needed
3. Optional: Install generator tool for convenience

### Temporary Compatibility
Can maintain both approaches temporarily:
- Keep old files temporarily with deprecation notice
- New `//go:generate` wrapper that calls new CLI
- Gradual migration over several releases

## Timeline

### Immediate (This Session)
- Create directory structure
- Implement configuration system
- Create CLI application skeleton
- Move and refactor first file (generator.go)

### Short Term
- Move remaining generator files
- Complete CLI implementation
- Update scripts and documentation
- Initial testing

### Medium Term
- Remove old generator files
- Finalize documentation
- Announce change to users

## Success Criteria

- [ ] Generator runs as standalone program
- [ ] Generated output identical to current implementation
- [ ] Configuration loaded from file
- [ ] CLI flags work and override config
- [ ] Memory optimization settings functional
- [ ] Cache behavior unchanged
- [ ] Documentation updated
- [ ] No breaking changes for library consumers
- [ ] All tests pass

## References

- Go Project Layout: https://github.com/golang-standards/project-layout
- Configuration best practices: External config files with CLI overrides
- Project rules applied:
  - Rule 2O5MCusKn0ig1WCSircvg0: Configuration files in `configs/` folder
  - Rule 6bEYpvbbLzAphQanjzYHTE: Configuration code in separate package
  - Rule iOzOmafaK4bINggR2vktAp: CLI arguments override config file
  - Rule thaCD4e9u4J35e5VSYKZR6: Configuration file named `config`
