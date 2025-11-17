# Switch Implementation

This document describes the switch type code generation implementation.

## Overview

Switches are conditional types that select different data types based on a comparison value. They are implemented inline within container structs, with the switch field typed as `pk.Field`.

## Switch Types

### 1. compareTo Switches (Field-based)

These switches compare against another field in the same container:

```json
{
  "compareTo": "someField",
  "fields": {
    "0": "i8",
    "1": "varint",
    "2": "f32"
  },
  "default": "void"
}
```

**Generated Code:**
```go
type Example struct {
    SomeField pk.VarInt
    Data      pk.Field  // Switch field
}

func (t *Example) ReadFrom(r io.Reader) (int64, error) {
    // ... read SomeField ...
    
    // Convert compareTo value to string for matching
    compareValue := fmt.Sprintf("%v", t.SomeField)
    switch compareValue {
    case "0":
        var val pk.Byte
        bytesRead, err = val.ReadFrom(r)
        // ... error handling ...
        t.Data = val
    case "1":
        var val pk.VarInt
        bytesRead, err = val.ReadFrom(r)
        // ... error handling ...
        t.Data = val
    case "2":
        var val pk.Float
        bytesRead, err = val.ReadFrom(r)
        // ... error handling ...
        t.Data = val
    default:
        // Void case - no data to read
        t.Data = struct{}{}
    }
    return totalBytes, nil
}
```

### 2. compareToValue Switches (Static-based)

These switches compare against a static compile-time value:

```json
{
  "compareToValue": 42,
  "fields": {
    "42": "string",
    "99": "varint"
  }
}
```

**Generated Code:**
```go
type Example struct {
    Data pk.Field  // Switch field
}

func (t *Example) ReadFrom(r io.Reader) (int64, error) {
    // Use static compareToValue for matching
    compareValue := fmt.Sprintf("%v", 42)
    switch compareValue {
    case "42":
        var val pk.String
        bytesRead, err = val.ReadFrom(r)
        // ... error handling ...
        t.Data = val
    case "99":
        var val pk.VarInt
        bytesRead, err = val.ReadFrom(r)
        // ... error handling ...
        t.Data = val
    }
    return totalBytes, nil
}
```

## WriteTo Implementation

Both switch types use the same WriteTo approach - they check if the value implements the `WriteTo` interface:

```go
func (t *Example) WriteTo(w io.Writer) (int64, error) {
    if t.Data != nil {
        // Write switch field value if it implements WriteTo
        if writer, ok := t.Data.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
            bytesWritten, err = writer.WriteTo(w)
            // ... error handling ...
        } else if _, ok := t.Data.(struct{}); !ok {
            // Not a void case and doesn't implement WriteTo
            return totalBytes, fmt.Errorf("switch field Data value does not implement WriteTo: %T", t.Data)
        }
    }
    return totalBytes, nil
}
```

## Special Cases

### Void Types

Cases mapped to `void` or types that compile to `struct{}` are handled specially:
- **ReadFrom**: Sets the field to `struct{}{}`
- **WriteTo**: No data is written (silently skipped)

### Unsupported Types

Types like `[]byte` and `struct{}` that don't implement the Field interface are automatically skipped during code generation to avoid compilation errors.

## Features

✅ **compareTo** field references with proper capitalization  
✅ **compareToValue** static value comparisons  
✅ Field name handling (converts protocol names to Go identifiers)  
✅ Sub-field access handling (e.g., `flags/has_redirect_node`)  
✅ Default case support  
✅ Void type handling  
✅ Error messages for unknown cases  
✅ Type-safe ReadFrom/WriteTo generation

## Limitations

⚠️ Some protocol definitions have missing sibling fields that switches reference  
⚠️ Bitfield member access (e.g., `flags/member_name`) uses only the field name, not the actual bit value  
⚠️ Root variable references (starting with `/`) are not yet fully implemented

## Testing

The implementation has been tested with the Minecraft 1.21.5 protocol and successfully generates code for hundreds of switch types including:
- `SlotComponentData` (complex mapper + switch combination)
- `EntityMetadataEntryValue` (mpk.Field cases)
- `ParticleData` (particle type discrimination)
- `PreviousMessagesArrayType` (signature validation)
