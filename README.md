# mc-protocol-go

A Go library for working with the Minecraft network protocol. It uses code generation to produce versioned protocol structures from Minecraft's protocol definitions, so packet types, IDs, and (de)serialization code exist for each supported Minecraft version under `data/<version>/`.

**Module:** `github.com/reallyoldfogie/mc-protocol-go`

The generator (`cmd/generator`) is the actively maintained part of this repository. Other CLI tools are included for validation/debugging support and are documented briefly below.

## Requirements

- Go (see `go.mod` for the required version)
- This repo depends on [`protodef-go`](https://github.com/reallyoldfogie/protodef-go) for parsing protocol definitions

## Running the Generator

The generator reads Minecraft protocol definitions and emits Go packages under `data/<version>/` (packet types, packet IDs, block/sound IDs, etc.).

```bash
# Generate code for a specific version
go run ./cmd/generator -config configs/config.yaml -versions 1.21.5 -output-dir data

# Generate code for all versions configured in configs/config.yaml
go run ./cmd/generator -config configs/config.yaml

# Low-memory mode (useful on constrained systems)
./run_gen_lowmem.sh
```

### Options

| Flag | Description |
|---|---|
| `-config` | Path to config file (default: `configs/config.yaml`) |
| `-versions` | Comma-separated list of versions to generate (overrides config) |
| `-output-dir` | Output directory (overrides config) |
| `-cache-dir` | Cache directory (overrides config) |
| `-memory-limit` | Memory limit, e.g. `512M`, `1G` |
| `-gc-percent` | GC aggressiveness (lower = more frequent collection) |
| `-max-procs` | Max OS threads |

### Configuration

The primary config file is `configs/config.yaml`:

```yaml
versions:           # Minecraft versions to generate
  - "1.21.5"
output:
  data_dir: "data"  # Where generated code goes
cache:
  cache_dir: ".cache"              # Index files
  metadata_dir: ".cache/metadata"  # Minecraft data downloads
  ttl_days: 7
memory:
  gc_percent: 50         # GC aggressiveness
  memory_limit: "512MiB" # Soft memory limit
  max_procs: 2           # Max OS threads
```

### After Generating

Generated code is committed to the repository, so after running the generator you should verify and commit the results:

```bash
go build ./...
go test ./...
go vet ./...
go fmt ./...
```

## Using the Library

Generated packages live under `data/<version>/`, split by namespace (`handshaking`, `status`, `login`, `configuration`, `play`) and direction (`clientbound`, `serverbound`). Shared types live in `data/<version>/basetypes`. Import the package for the Minecraft version you need, e.g.:

```go
import "github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/play/clientbound"
```

Core interfaces and shared utilities (NBT parsing, packet marshalling, arrays, options, bitflags, etc.) live in `models/`.

## Other Tools

These tools are not under active maintenance but are kept around for validation and debugging support:

- **`cmd/packet-validation`** — Round-trip validation (Scan/Marshal, ReadFrom/WriteTo) against captured packet logs. See `cmd/packet-validation/README.md`.
  ```bash
  go run ./cmd/packet-validation -paths testing/packetlogs -summary
  ```
- **`cmd/groundtruth-validation`** — Compares generated output against ground-truth data. See `cmd/groundtruth-validation/README.md`.
- **`cmd/validate-packets`** — Additional packet validation utility.
- **`cmd/test_uuid_parse`** — Small utility for testing UUID parsing.

## Project Structure

```
.
├── cmd/
│   ├── generator/              # Code generator CLI (actively maintained)
│   ├── packet-validation/      # Packet log round-trip validation CLI
│   ├── groundtruth-validation/ # Ground-truth comparison CLI
│   ├── validate-packets/       # Additional packet validation CLI
│   └── test_uuid_parse/        # UUID parsing test utility
├── internal/
│   ├── generator/              # Generator implementation
│   └── packetlogtest/          # Packet log test helpers
├── models/                     # Core types (NBT, packet interfaces, etc.)
├── data/<version>/             # Generated protocol code (committed)
├── versions/<version>/         # Source protocol metadata
├── testing/packetlogs/         # Sample packet logs for validation
├── configs/                    # Generator configuration
└── docs/                       # Design docs and implementation notes
```

## License

See [LICENSE](LICENSE).
