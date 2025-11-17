# parse-packetlog

A CLI tool for validating Minecraft protocol packet logs by performing round-trip Scan/Marshal and ReadFrom/WriteTo checks on logged packet data.

## Usage

```bash
parse-packetlog [options] --paths <file-or-directory>[,<file-or-directory>...]
```

## Description

This tool reads packet log files (JSON lines format) created by `mc-agent` and validates that the generated protocol code can correctly parse and round-trip each packet. It helps identify bugs in the code generator by comparing the original packet data with the result of parsing and re-marshaling.

The tool performs the following checks for each packet:
1. **Scan/Marshal round-trip**: Parses the raw packet data using `Scan()` and marshals it back using `Marshal()`, then compares the results
2. **ReadFrom validation**: If the packet type implements `io.ReaderFrom`, validates it can read the packet data
3. **WriteTo validation**: If the packet type implements `io.WriterTo`, validates it can write the packet data correctly

## Options

### Input
- `--paths string` - Comma-separated list of log files or directories to process (required)
  - Directories are searched recursively for `.log` files

### Filtering
- `-versions string` - Comma-separated list of versions to process (e.g., `1.21.5,1.21.4`)
- `-include string` - Comma-separated list of packet name patterns to include
- `-exclude string` - Comma-separated list of packet name patterns to exclude
- `-max-packets int` - Maximum packets to process per file (0=unlimited, default: 0)
- `-stop-on-first` - Stop processing file on first error (default: false)

### Output
- `-max-errors int` - Maximum errors to collect and display (default: 100)
- `-verbose` - Enable verbose progress output to stderr
- `-json string` - Write JSON results to specified file (text output still goes to stdout)
- `-summary` - Show only summary statistics, no individual errors

## Examples

### Basic usage - single file
```bash
parse-packetlog --paths testing/packetlogs/1.21.5/sample_clientbound.log
```

### Process entire directory
```bash
parse-packetlog --paths /home/user/mc-agent/logs
```

### Process multiple paths
```bash
parse-packetlog --paths file1.log,/path/to/logs/,file2.log
```

### Filter by packet type
```bash
# Only validate ClientboundEntity* packets
parse-packetlog --paths logs/ --include ClientboundEntity

# Exclude known problematic packets
parse-packetlog --paths logs/ --exclude ClientboundDeclareRecipes,ClientboundDeclareCommands
```

### Summary only output
```bash
parse-packetlog --paths logs/ --summary
```

### JSON output for automation
```bash
# JSON written to file, text output to stdout
parse-packetlog --paths logs/ --json results.json

# JSON with summary text output
parse-packetlog --paths logs/ --json results.json --summary

# JSON only (suppress text with quiet redirection)
parse-packetlog --paths logs/ --json results.json > /dev/null
```

### Limit processing
```bash
# Process only first 1000 packets per file
parse-packetlog --paths logs/ --max-packets 1000

# Show only first 10 errors
parse-packetlog --paths logs/ --max-errors 10
```

## Output Format

### Text Output (default)

```
File: /path/to/file.log
Total: 236, Succeeded: 204, Failed: 32, Skipped: 0
By Version:
  1.21.5: 204
By Direction:
  clientbound: 204
By Packet Name (top 10):
  ClientboundEntityHeadRotation: 61
  ClientboundEntityMoveLook: 53
  ...
Errors (first 10):
  1. [roundtrip] ClientboundDeclareRecipes (id=126 version=1.21.5): packetlogtest: Scan failed...
  2. [roundtrip] ClientboundDeclareCommands (id=16 version=1.21.5): packetlogtest: Scan failed...
  ...
```

For multiple files, an overall summary is also displayed.

### JSON Output (--json results.json)

JSON is written to the specified file while text output goes to stdout. This allows you to:
- Get both human-readable output in the terminal and machine-readable JSON for processing
- Avoid mixing debug output from generated code with JSON data

JSON format:
```json
{
  "files": [
    {
      "path": "/path/to/file.log",
      "summary": {
        "Total": 236,
        "Skipped": 0,
        "Succeeded": 204,
        "Failed": 32,
        "ByVersion": {"1.21.5": 204},
        "ByDirection": {"clientbound": 204},
        "ByName": {...},
        "Errors": [...]
      }
    }
  ],
  "total": {
    "Total": 236,
    "Succeeded": 204,
    ...
  }
}
```

Example usage:
```bash
# Get both text and JSON output
parse-packetlog --paths logs/ --json results.json

# Extract specific data with jq
parse-packetlog --paths logs/ --json results.json > /dev/null
jq '.total.Failed' results.json
```

## Exit Codes

- `0` - All packets processed successfully
- `1` - One or more packets failed validation

## Common Error Types

- **EOF errors**: Generator is not consuming all packet data during Scan
- **Marshal data mismatch**: Round-trip produces different data than original
- **Panic errors**: Interface conversion failures or missing methods in generated code
- **NBT decode errors**: Incorrect NBT field handling in generator
- **Unknown metadata types**: Missing EntityMetadata type mappings

## Integration with CI

```bash
#!/bin/bash
# Example CI script with JSON artifact
if ! parse-packetlog --paths logs/ --max-errors 50 --json ci-results.json; then
    echo "Packet validation failed!"
    # JSON results still written to ci-results.json for analysis
    exit 1
fi

# Process JSON results
jq '.total | {total: .Total, failed: .Failed, success_rate: (.Succeeded / .Total * 100)}' ci-results.json
```

## See Also

- `internal/packetlogtest` - Underlying library used by this tool
- `docs/PACKET_LOG_TESTING_PLAN.md` - Overall testing framework design
- `models/packet_log.go` - PacketLog struct definition
