# Packet Generator

This tool uses [node-minecraft-protocol](https://github.com/PrismarineJS/node-minecraft-protocol) as a reference implementation to generate test packets with known field values.

## Purpose

The existing packet validation performs round-trip testing (parse → serialize → compare bytes), but doesn't verify that parsed field values are correct. This tool solves that by:

1. **Defining packets with known field values** - We specify exact values for each field
2. **Using node-minecraft-protocol to serialize** - Reference implementation creates correct bytes
3. **Providing ground truth data** - Our Go parser can validate it extracts the correct values

## Installation

```bash
cd testing/packet-generator
npm install
```

## Usage

### Generate Test Packets

```bash
# Generate test packets for 1.21.5 (default)
node generator.js

# Generate for specific version
node generator.js --version 1.21.4

# Specify output file
node generator.js --output my-tests.jsonl
```

### Output Format

The tool outputs JSONL (JSON Lines) format where each line is a test case:

```json
{
  "description": "Single player ADD_PLAYER action",
  "version": "1.21.5",
  "packetName": "player_info",
  "packetId": "player_info",
  "groundTruth": {
    "action": 0,
    "data": [
      {
        "UUID": "d3527a0b-bc03-45d5-a878-2aafdd8c8a43",
        "name": "TestPlayer1",
        "properties": [],
        "gamemode": 1,
        "ping": 50,
        "displayName": "{\"text\":\"Test Player 1\"}"
      }
    ]
  },
  "serialized": "0a01d3527a0b...",
  "timestamp": "2025-11-17T20:30:00.000Z"
}
```

- `groundTruth`: The known field values we want to validate
- `serialized`: Hex-encoded packet bytes from node-minecraft-protocol

## Automatic Test Generation

For comprehensive testing, use the auto-generator to generate tests for ALL packets:

```bash
# Generate tests for all versions from config (default)
node auto-generator.js

# Generate for specific versions
node auto-generator.js --versions 1.21.5
node auto-generator.js --versions 1.21.4,1.21.5,1.21.6

# Use custom config file
node auto-generator.js --config path/to/config.yaml

# Explicitly specify all states (same as default)
node auto-generator.js --versions 1.21.5 --states all

# Explicitly specify both directions (same as default)
node auto-generator.js --versions 1.21.5 --direction both

# Generate for specific state only
node auto-generator.js --versions 1.21.5 --states play

# Generate clientbound only
node auto-generator.js --versions 1.21.5 --direction clientbound

# Multiple specific states
node auto-generator.js --versions 1.21.5 --states login,configuration,play

# Limit number of packets (useful for testing)
node auto-generator.js --versions 1.21.5 --max 50

# Skip problematic packets
node auto-generator.js --versions 1.21.5 --skip chunk_data,map_chunk

# Custom output file (all versions in one file)
node auto-generator.js --versions 1.21.5 --output all-packets.jsonl

# Output one file per version
node auto-generator.js --output-dir generated-tests
# Creates: generated-tests/1.21.1.jsonl, generated-tests/1.21.2.jsonl, etc.

# Multiple versions with separate files
node auto-generator.js --versions 1.21.4,1.21.5,1.21.6 --output-dir tests-by-version

# Generate randomized, non-zero field values (single variant)
node auto-generator.js --versions 1.21.5 --mode random

# Generate both zero-default AND randomized variants per packet
node auto-generator.js --versions 1.21.5 --mode both

# Deterministic random values via seed
node auto-generator.js --versions 1.21.5 --mode random --seed 12345
```

The auto-generator:
- Introspects minecraft-data protocol definitions
- By default emits BOTH zero-default and randomized variants per packet (`--mode both`)
- Automatically generates sensible default values for all field types (zeros mode)
- Can generate randomized non-zero values for broader parser validation (random/both modes)
- Serializes packets using node-minecraft-protocol
- Outputs ground truth test cases

**Note**: Some complex packets may fail to generate (arrays with required items, complex switch types, etc.). The generator will report success/failure counts.

## Manual Test Case Creation

For specific test cases or complex scenarios, edit `generator.js` and add test cases using `generateTestPacket()`:

```javascript
testPackets.push(generateTestPacket('packet_name', {
  field1: value1,
  field2: value2,
  // ... all fields
}, 'Description of what this tests'))
```

## Validating Against Go Parser

The generated test packets can be used with the ground truth validation tool:

```bash
# Validate for specific version
go run ./cmd/groundtruth-validation -test-file test-packets.jsonl -version 1.21.5

# Validate for all versions in config
go run ./cmd/groundtruth-validation -test-file test-packets.jsonl

# Show successful validations too
go run ./cmd/groundtruth-validation -test-file test-packets.jsonl -version 1.21.5 -show-success
```

This will:
1. Read the serialized bytes
2. Parse with our Go implementation
3. Compare parsed values against `groundTruth`
4. Report any mismatches

## Key Advantages

1. **Known Ground Truth** - We know exactly what values should be parsed
2. **Reference Implementation** - node-minecraft-protocol is widely used and tested
3. **Targeted Testing** - Can create specific test cases for problematic packets
4. **Cross-validation** - Validates our implementation against another

## Limitations

- Only tests packets that node-minecraft-protocol supports
- Node.js and Go may have different floating point precision
- Some packets require server state (like chunk data)
 - Arrays with an explicit count field are synchronized with their count: in random/both modes a small non-zero count (1–2) is set and that many elements are generated; in zeros mode the count is 0 and the array is empty. Arrays with a length prefix (`countType`) are also populated (1–2) in random/both modes.
