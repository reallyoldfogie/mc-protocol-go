# RawDefinition Comments in Generated Code

## Overview

The code generator now includes the original protodef JSON definition as comments in generated Go structs. This makes it much easier to understand and debug protocol structures by seeing both the original definition and the generated Go code side-by-side.

## Features

- **Struct-level comments**: Shows the complete protodef definition for each container type
- **Field-level comments**: Shows the protodef type definition for each field
- **Multi-line preservation**: Complex JSON structures are preserved across multiple comment lines
- **No truncation**: Full definitions are included, regardless of length

## Examples

### Simple Packet Structure

Given this protodef definition:
```json
{
  "packet_login_start": [
    "container",
    [
      {"name": "username", "type": "pstring"},
      {"name": "player_uuid", "type": "UUID"}
    ]
  ]
}
```

The generated Go code will include:
```go
// Protodef: ["container", [
// 	{"name": "username", "type": "pstring"},
// 	{"name": "player_uuid", "type": "UUID"}
// ]]
type PacketLoginStart struct {
	packetID int32
	// "pstring"
	Username pk.String
	// "UUID"
	PlayerUUID pk.UUID
}
```

### Complex Nested Structure

For more complex definitions with switches and nested containers:
```json
{
  "entity_metadata_entry": [
    "container",
    [
      {"name": "index", "type": "u8"},
      {"name": "type", "type": "varint"},
      {
        "name": "value",
        "type": [
          "switch",
          {
            "compareTo": "type",
            "fields": {
              "0": "i8",
              "1": "varint",
              "2": "f32"
            }
          }
        ]
      }
    ]
  ]
}
```

The generated code will show:
```go
// Protodef: ["container", [
// 	{"name": "index", "type": "u8"},
// 	{"name": "type", "type": "varint"},
// 	{
// 		"name": "value",
// 		"type": [
// 			"switch",
// 			{
// 				"compareTo": "type",
// 				"fields": {
// 					"0": "i8",
// 					"1": "varint",
// 					"2": "f32"
// 				}
// 			}
// 		]
// 	}
// ]]
type EntityMetadataEntry struct {
	// "u8"
	Index pk.UnsignedByte
	// "varint"
	Type pk.VarInt
	// ["switch", {
	// 	"compareTo": "type",
	// 	"fields": {
	// 		"0": "i8",
	// 		"1": "varint",
	// 		"2": "f32"
	// 	}
	// }]
	Value EntityMetadataEntryValueSwitch
}
```

## Use Cases

### 1. Debugging Protocol Issues

When troubleshooting packet serialization/deserialization problems, you can quickly see:
- What the original protodef type was
- How it maps to Go types
- What the complete structure looks like

### 2. Understanding Type Mappings

See at a glance how protodef types map to Go types:
- `"varint"` → `pk.VarInt`
- `"pstring"` → `pk.String`
- `["array", {...}]` → `models.Array[T]`

### 3. Protocol Documentation

The generated code serves as self-documenting, showing both the implementation and the specification in one place.

### 4. Version Comparison

When comparing protocol versions, you can easily see:
- Which fields changed types
- Which structures were added or removed
- How complex types evolved

## Implementation Details

The RawDefinition is captured at parsing time by the protodef-go library and preserved through the entire generation pipeline:

1. **Parsing**: When protodef JSON is parsed, the `RawDefinition` field in `datatypes.Type` stores the original JSON snippet
2. **Template Processing**: The `formatRawDef` template function formats the raw JSON for Go comments
3. **Code Generation**: Comments are emitted before each struct and field definition

### Formatting Rules

- **Single-line definitions**: Emitted as-is after trimming whitespace
- **Multi-line definitions**: Each line after the first is prefixed with `// `
- **Indentation**: Original indentation is preserved to maintain JSON structure
- **No truncation**: Full definitions are always included

## Benefits

1. **Better debugging**: Quickly identify discrepancies between protocol spec and implementation
2. **Easier onboarding**: New developers can understand protocol structures faster
3. **Self-documentation**: Code includes its own specification
4. **Version tracking**: Changes in protocol definitions are visible in diffs

## Related

- [protodef-go RawDefinition Feature](https://github.com/reallyoldfogie/protodef-go/docs/rawdefinition.md)
- [Minecraft Protocol Documentation](https://wiki.vg/Protocol)
