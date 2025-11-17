# parse-packetlog CLI Tool

## Overview

The packet log testing framework has been refactored from a Go test-based approach to a standalone CLI tool (`cmd/parse-packetlog`). This provides a more flexible and user-friendly interface for validating generated Minecraft protocol packet types against real packet captures.

## What Changed

### Before (Test-based approach)
- Validation was performed via `go test` in `testing/packetlog_roundtrip_test.go`
- Required Go toolchain and test knowledge to run
- Limited filtering and output options
- Harder to integrate into non-Go workflows

### After (CLI tool approach)
- Standalone executable `parse-packetlog` in `cmd/parse-packetlog/main.go`
- Can be run directly without Go toolchain (after building)
- Rich command-line flags for filtering and output control
- Easy integration with CI/CD, scripts, and automation
- Multiple output formats (text, summary, JSON)

## Architecture

```
┌─────────────────────────────────────┐
│   cmd/parse-packetlog/main.go      │
│   (CLI interface, 296 lines)        │
│   • Flag parsing                    │
│   • File discovery                  │
│   • Result aggregation              │
│   • Output formatting               │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  internal/packetlogtest/            │
│  packetlogtest.go (520 lines)       │
│  • PacketLogSource interface        │
│  • FileSource, SliceSource          │
│  • RunRoundTripChecks               │
│  • Packet resolution logic          │
│  • Scan/Marshal validation          │
│  • ReadFrom/WriteTo validation      │
│  • Panic recovery                   │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  data/versions + generated packets  │
│  • GetPacketMgrForVersion           │
│  • Get*PacketByID factories         │
│  • PacketMarshaller interface       │
└─────────────────────────────────────┘
```

## Files Created/Modified

### Created
- `cmd/parse-packetlog/main.go` - Main CLI implementation (296 lines)
- `cmd/parse-packetlog/README.md` - Comprehensive usage documentation
- `docs/PARSE_PACKETLOG_CLI.md` - This document

### Modified
- `internal/packetlogtest/packetlogtest.go` - Added panic recovery in `processPacket()`
- `docs/PACKET_LOG_TESTING_PLAN.md` - Updated to reflect CLI tool implementation

### Removed
- `testing/packetlog_roundtrip_test.go` - Replaced by CLI tool

## Features

### Input Handling
- **Single file**: `parse-packetlog file.log`
- **Multiple files**: `parse-packetlog file1.log file2.log`
- **Directory (recursive)**: `parse-packetlog /path/to/logs/`
- **Mixed**: `parse-packetlog file.log /path/to/logs/`

### Filtering Options
- `--versions 1.21.5,1.21.4` - Filter by Minecraft version
- `--include ClientboundEntity` - Include only matching packet names
- `--exclude ClientboundDeclare` - Exclude matching packet names
- `--max-packets 1000` - Limit packets processed per file
- `--stop-on-first` - Stop processing file on first error

### Output Options
- **Default**: Detailed text output with errors
- `--summary`: Statistics only, no error details
- `--json`: Machine-readable JSON output
- `--max-errors 50`: Limit number of errors displayed
- `--verbose`: Enable progress messages to stderr

### Exit Codes
- `0` - All packets validated successfully
- `1` - One or more packets failed validation

## Usage Examples

### Basic validation
```bash
parse-packetlog testing/packetlogs/1.21.5/sample_clientbound.log
```

Output:
```
File: /path/to/sample_clientbound.log
Total: 10, Succeeded: 6, Failed: 4, Skipped: 0
By Version:
  1.21.5: 6
By Direction:
  clientbound: 6
By Packet Name (top 10):
  ClientboundLogin: 1
  ClientboundDifficulty: 1
  ...
Errors (first 4):
  1. [roundtrip] ClientboundDeclareRecipes (id=126 version=1.21.5): ...
  2. [roundtrip] ClientboundDeclareCommands (id=16 version=1.21.5): ...
  ...
```

### Validate entire directory
```bash
parse-packetlog /home/user/mc-agent/logs
```

### Summary only
```bash
parse-packetlog --summary logs/
```

### JSON output for automation
```bash
parse-packetlog --json logs/ | jq '.total.Failed'
```

### Filter by packet type
```bash
# Only entity-related packets
parse-packetlog --include ClientboundEntity logs/

# Exclude known problematic packets
parse-packetlog --exclude ClientboundDeclareRecipes,ClientboundDeclareCommands logs/
```

### Limit processing for quick checks
```bash
parse-packetlog --max-packets 100 --max-errors 10 logs/
```

## Integration Examples

### CI/CD (GitHub Actions)
```yaml
- name: Validate packet logs
  run: |
    go build -o parse-packetlog ./cmd/parse-packetlog
    ./parse-packetlog --max-errors 50 testing/packetlogs/
```

### Shell script
```bash
#!/bin/bash
set -e

# Validate all logs in directory
if ! ./bin/parse-packetlog "$LOG_DIR"; then
    echo "Packet validation failed!"
    exit 1
fi

echo "All packets validated successfully"
```

### Makefile
```makefile
.PHONY: validate-packets
validate-packets:
	@echo "Validating packet logs..."
	@./bin/parse-packetlog testing/packetlogs/
	@echo "Validation complete!"
```

## Error Types Detected

The CLI tool successfully identifies various generator bugs:

1. **EOF errors**: Generator not consuming all packet data during Scan
   - Example: `ClientboundDeclareRecipes`, `ClientboundDeclareCommands`

2. **Marshal data mismatches**: Round-trip produces different data
   - Example: `ClientboundPosition` (61 bytes → 58 bytes)

3. **Panic errors**: Interface conversion failures, missing methods
   - Example: `ClientboundPlayerInfo` interface conversion panic

4. **NBT decode errors**: Incorrect NBT field handling
   - Example: `ClientboundServerData` non-pointer passed to Decode

5. **Unknown metadata types**: Missing EntityMetadata type mappings
   - Example: `ClientboundEntityMetadata` unknown key 65

6. **Unexpected EOF**: Incomplete packet data
   - Example: `ClientboundAdvancements`, `ClientboundPlayerChat`

## Performance

### Unit Tests
- 6 tests in `internal/packetlogtest/packetlogtest_test.go`
- Run time: < 30ms
- Coverage: error paths, filtering, limiting

### CLI Tool
- Fixture validation (10 packets): ~20ms
- Real log validation (236 packets): ~50ms
- Memory efficient: streaming JSON-lines decoding

## Future Enhancements

1. **CI Integration**: Add GitHub Actions job to run on fixtures
2. **Watch mode**: Auto-rerun when log files change
3. **Diff mode**: Compare results between runs
4. **HTML report**: Generate browsable error report
5. **Parallel processing**: Process multiple files concurrently

## Migration Guide

### For developers using the old test approach

**Before:**
```bash
MC_PACKETLOG_DIR=/path/to/logs go test ./testing -run PacketLogs
```

**After:**
```bash
parse-packetlog /path/to/logs
```

**Equivalent test fixture validation:**
```bash
# Old: go test ./testing -run TestPacketLogs_RoundTrip_Fixtures
# New:
parse-packetlog testing/packetlogs/
```

## Troubleshooting

### Debug output pollutes JSON
The generated code emits debug logs to stdout. Redirect stderr to filter them:
```bash
parse-packetlog --json logs/ 2>/dev/null | jq .
```

### Large log files
Use filtering to reduce processing time:
```bash
parse-packetlog --max-packets 1000 logs/
```

### CI timeout
Limit errors and packets:
```bash
parse-packetlog --max-errors 20 --max-packets 5000 logs/
```

## See Also

- `cmd/parse-packetlog/README.md` - Full usage documentation
- `docs/PACKET_LOG_TESTING_PLAN.md` - Overall framework design
- `internal/packetlogtest/` - Core validation library
- `testing/packetlogs/README.md` - Test fixture documentation
