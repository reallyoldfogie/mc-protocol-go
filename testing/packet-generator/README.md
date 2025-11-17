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

## Adding New Test Cases

Edit `generator.js` and add test cases using `generateTestPacket()`:

```javascript
testPackets.push(generateTestPacket('packet_name', {
  field1: value1,
  field2: value2,
  // ... all fields
}, 'Description of what this tests'))
```

## Validating Against Go Parser

The generated test packets can be used with the packet validation tool:

```bash
go run ./cmd/packet-validation --ground-truth test-packets.jsonl
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
