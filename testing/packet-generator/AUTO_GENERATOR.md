# Auto Packet Generator

Automatically generates ground truth test cases for ALL packets in a Minecraft version by introspecting the protocol definitions.

## Features

- **Automatic Discovery**: Introspects minecraft-data to find all available packets
- **Smart Value Generation**: Generates sensible default values for all field types
- **Multi-State Support**: Can generate packets for any protocol state (handshaking, status, login, configuration, play)
- **Bi-directional**: Supports both clientbound and serverbound packets
- **Version Aware**: Works with any Minecraft version supported by node-minecraft-protocol

## Usage

### Basic Usage

```bash
# Generate tests for ALL versions from config, ALL states, BOTH directions (default)
node auto-generator.js

# Output will be: auto-tests.jsonl
```

**Defaults when flags are omitted:**
- `--versions`: Loads from config file (default: `../../configs/config.yaml`)
- `--states`: Generates for **all states** (handshaking, status, login, configuration, play)
- `--direction`: Generates for **both directions** (clientbound and serverbound)
- `--output`: Defaults to `auto-tests.jsonl`

### Advanced Options

```bash
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
node auto-generator.js --versions 1.21.5 --skip map,chunk_data,world_particles

# Custom output file (all versions in one file)
node auto-generator.js --versions 1.21.5 --output my-tests.jsonl

# Output one file per version
node auto-generator.js --output-dir generated-tests
# Creates: generated-tests/1.21.1.jsonl, generated-tests/1.21.2.jsonl, etc.

# Multiple versions with separate files
node auto-generator.js --versions 1.21.4,1.21.5,1.21.6 --output-dir tests-by-version

# Full example: multiple versions, specific state, separate files per version
node auto-generator.js --versions 1.21.4,1.21.5,1.21.6 \
  --states play \
  --direction clientbound \
  --output-dir play-packets-by-version
```

## Output Modes

The auto-generator supports two output modes:

### Single File Mode (default)
When using `--output`, all versions are written to a single JSONL file with each packet tagged with its version:
```bash
node auto-generator.js --versions 1.21.4,1.21.5 --output all-tests.jsonl
# Creates: all-tests.jsonl (contains packets from both versions)
```

### Multi-File Mode
When using `--output-dir`, each version gets its own file:
```bash
node auto-generator.js --versions 1.21.4,1.21.5 --output-dir tests
# Creates: tests/1.21.4.jsonl, tests/1.21.5.jsonl
```

The directory will be created if it doesn't exist. File names are based on version numbers (e.g., `1.21.5.jsonl`).

**Note:** `--output-dir` overrides `--output` if both are specified.

## Success Rate

Based on testing with Minecraft 1.21.5 PLAY state:

- **Total clientbound packets**: 123
- **Successfully generated**: 60 (48.8%)
- **Failed to generate**: 63 (51.2%)

Failures are typically due to:
1. **Arrays requiring elements**: Packets with arrays that must have at least one element
2. **Complex switch types**: Switch types without default cases or with complex dependencies
3. **Special data structures**: Packets requiring NBT data, chunk data, or server-side state
4. **Interdependent fields**: Fields that depend on values of other fields

## Validating Generated Tests

Use the ground truth validation tool to validate the generated test cases:

```bash
# Validate for specific version
go run ./cmd/groundtruth-validation -test-file auto-tests.jsonl -version 1.21.5

# Validate for all versions in config
go run ./cmd/groundtruth-validation -test-file auto-tests.jsonl

# Show successful validations too
go run ./cmd/groundtruth-validation -test-file auto-tests.jsonl -version 1.21.5 -show-success
```

## Output Format

Each line in the output JSONL file contains:

```json
{
  "description": "Auto-generated clientbound play spawn_entity",
  "version": "1.21.5",
  "state": "play",
  "direction": "clientbound",
  "packetName": "spawn_entity",
  "packetId": "spawn_entity",
  "groundTruth": {
    "entityId": 0,
    "objectUUID": "00000000-0000-0000-0000-000000000000",
    "type": 0,
    "x": 0,
    "y": 0,
    "z": 0,
    "pitch": 0,
    "yaw": 0,
    "headYaw": 0,
    "data": 0,
    "velocityX": 0,
    "velocityY": 0,
    "velocityZ": 0
  },
  "serialized": "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  "timestamp": "2025-11-19T07:30:00.000Z"
}
```

## Default Value Generation

The auto-generator uses these defaults for common types:

| Type | Default Value |
|------|---------------|
| `i8`, `i16`, `i32`, `u8`, `u16`, `u32`, `varint` | `0` |
| `i64`, `u64`, `varlong` | `0n` (BigInt) |
| `f32`, `f64` | `0.0` |
| `string`, `pstring` | `"test"` |
| `bool` | `false` |
| `UUID` | `"00000000-0000-0000-0000-000000000000"` |
| `buffer`, `restBuffer`, `ByteArray` | Empty buffer |
| `position` | `{x: 0, y: 64, z: 0}` |
| `array` | `[]` (empty array) |
| `option` | `undefined` |
| `Slot` | `{present: false}` |

For complex types (containers, switches, etc.), the generator recursively generates values based on field definitions.

### Randomized Mode

You can also generate packets with randomized non-zero values to further validate parsing logic:

```
node auto-generator.js --versions 1.21.5 --mode random
```

Or emit both zero-default and randomized variants per packet (this is the default behavior):

```
node auto-generator.js --versions 1.21.5 --mode both
```

Randomization notes:

- Numeric types use small non-zero values (e.g., 1..5)
- Floats use small non-zero values (e.g., 0.1, 0.2, ...)
- Strings are non-empty alphanumeric (e.g., `auto_xxxxxx`)
- UUIDs are randomly generated v4
- Buffers are small non-empty (default 4 bytes)
- Positions and vectors use small non-zero components
- Bitflags get a small non-zero mask
- Arrays with an explicit count field are synchronized with their count: in random/both modes a small non-zero count (1–2) is set and that many elements are generated; in zeros mode the count is 0 and the array is empty. Arrays with a length prefix (`countType`) are also populated (1–2) in random/both modes.
- Optional fields remain absent by default

Use `--seed <n>` for deterministic random values (LCG-based PRNG). Default mode is `both`.

## Known Limitations

1. **Empty Arrays**: Arrays are generated as empty, which may fail for packets requiring at least one element
2. **Switch Types**: Complex switch types may not generate correctly
3. **Server State**: Packets requiring server-side state (like chunk data) cannot be fully generated
4. **Circular References**: Deep recursion is limited to prevent infinite loops
5. **Custom Types**: Some custom types may not be recognized and will generate `null`
6. **Type Comparison Issues**: Some complex Go types (UUID, Position, etc.) may show as failed in validation due to type comparison limitations, even though parsing works correctly

## Improving Success Rate

To improve the success rate for your use case:

1. **Identify failed packets**: Check the console output for failed packet names
2. **Add skip list**: Use `--skip` to exclude consistently failing packets
3. **Manual test cases**: For critical failed packets, create manual test cases in `generator.js`
4. **Enhance generator**: Extend `generateDefaultValue()` function to handle specific types better

## Comparison with Manual Generator

| Feature | Auto-generator | Manual generator |
|---------|----------------|------------------|
| Coverage | ~50% of packets | Only manually added |
| Effort | Fully automated | Manual per packet |
| Accuracy | Default values | Specific test values |
| Use case | Comprehensive testing | Targeted testing |

**Recommendation**: Use both approaches:
- Auto-generator for broad coverage and regression testing
- Manual generator for specific edge cases and complex scenarios

## Troubleshooting

### "Unknown type" warnings

These are informational - the generator is encountering a type it doesn't have a built-in default for. It will try to look it up in the protocol definitions.

### "Failed to generate" errors

Common causes:
- Arrays that need at least one element for serialization
- Switch types without a suitable default case
- Fields requiring complex initialization

Use `--skip` to exclude these packets or add manual test cases for them.

### Validation panics

Some generated packets may cause panics in the Go parser due to nil pointer issues. This indicates bugs in the Go implementation that should be fixed. Use `--skip` to exclude problematic packets while debugging.
