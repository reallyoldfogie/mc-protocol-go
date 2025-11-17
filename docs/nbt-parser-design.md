# NBT Parser Design Document

## Overview

This document outlines the design and implementation plan for a custom NBT (Named Binary Tag) parser for the mc-protocol-go project. The parser will replace the previous `pk.NBTField` dependency with a custom solution based on code from the [nbt2json](https://github.com/midnightfreddie/nbt2json) library.

## Background

NBT (Named Binary Tag) is a structured binary format used by Minecraft for storing data. The format consists of:
- **Tag Type**: A byte identifying the type of data (0-12)
- **Tag Name**: A UTF-8 string (except for type 0 and list items)
- **Payload**: The actual data, format depends on tag type

### NBT Tag Types
- **0**: TAG_End - Marks end of compound tags
- **1**: TAG_Byte - int8
- **2**: TAG_Short - int16
- **3**: TAG_Int - int32
- **4**: TAG_Long - int64
- **5**: TAG_Float - float32
- **6**: TAG_Double - float64
- **7**: TAG_Byte_Array - Array of int8
- **8**: TAG_String - UTF-8 string with int16 length prefix
- **9**: TAG_List - List of unnamed tags of same type
- **10**: TAG_Compound - Named tags until TAG_End
- **11**: TAG_Int_Array - Array of int32
- **12**: TAG_Long_Array - Array of int64

## Design Goals

1. **Parse NBT data from byte streams** without converting to JSON
2. **Provide a Go-friendly API** for reading NBT structures
3. **Support both reading and writing** NBT data
4. **Maintain compatibility** with Minecraft protocol requirements
5. **Preserve the MIT license** and copyright notice from nbt2json

## Architecture

### Package Structure

The NBT parser will be organized in the `models` package alongside existing data structures:

```
models/
  ├── nbt.go          # Existing file with nbt2json-based code
  ├── nbt_reader.go   # New: NBT reading implementation
  ├── nbt_writer.go   # New: NBT writing implementation (future)
  └── nbt_types.go    # New: NBT type definitions and interfaces
```

### Core Types

#### NBTValue Interface
```go
type NBTValue interface {
    Type() NBTType
    WriteTo(w io.Writer) error
    // Additional methods as needed
}
```

#### NBTType
```go
type NBTType byte

const (
    TypeEnd       NBTType = 0
    TypeByte      NBTType = 1
    TypeShort     NBTType = 2
    TypeInt       NBTType = 3
    TypeLong      NBTType = 4
    TypeFloat     NBTType = 5
    TypeDouble    NBTType = 6
    TypeByteArray NBTType = 7
    TypeString    NBTType = 8
    TypeList      NBTType = 9
    TypeCompound  NBTType = 10
    TypeIntArray  NBTType = 11
    TypeLongArray NBTType = 12
)
```

#### NBT Tag Structure
```go
type NBTTag struct {
    Name  string
    Value NBTValue
}
```

#### Concrete Types
Each NBT type will have a corresponding Go struct:
- `NBTByte` (int8)
- `NBTShort` (int16)
- `NBTInt` (int32)
- `NBTLong` (int64)
- `NBTFloat` (float32)
- `NBTDouble` (float64)
- `NBTByteArray` ([]int8)
- `NBTString` (string)
- `NBTList` ([]NBTValue with homogeneous type)
- `NBTCompound` (map[string]NBTValue or []NBTTag)
- `NBTIntArray` ([]int32)
- `NBTLongArray` ([]int64)

### NBT Reader

#### NBTReader Type
```go
type NBTReader struct {
    r         io.Reader
    byteOrder binary.ByteOrder
}

func NewNBTReader(r io.Reader) *NBTReader
func (nr *NBTReader) ReadTag() (*NBTTag, error)
func (nr *NBTReader) ReadValue(tagType NBTType) (NBTValue, error)
```

#### Key Functions
- `ReadTag()` - Reads a complete NBT tag (type + name + value)
- `ReadValue(tagType)` - Reads just the value based on type
- `readString()` - Reads UTF-8 string with int16 length prefix
- `readCompound()` - Reads compound tags until TAG_End
- `readList()` - Reads list with type prefix and length

### Integration with Existing Code

The new NBT parser should:
1. **Integrate with Buffer type** in `models/buffer.go` for io.Reader/io.Writer compatibility
2. **Support binary.ByteOrder configuration** (BigEndian for Java Edition)
3. **Provide error types** similar to existing `NbtParseError`
4. **Follow project conventions** as defined in user rules

## Implementation Plan

### Phase 1: Type Definitions (nbt_types.go) ✅ COMPLETED
1. ✅ Define `NBTType` constants
2. ✅ Create `NBTValue` interface
3. ✅ Implement concrete types for each NBT tag type
4. ✅ Implement `Type()` method for each concrete type
5. ✅ Add comprehensive documentation

### Phase 2: NBT Reader (nbt_reader.go) ✅ COMPLETED
1. ✅ Create `NBTReader` struct with io.Reader
2. ✅ Implement `NewNBTReader` constructor
3. ✅ Implement `ReadTag()` for reading named tags
4. ✅ Implement `ReadValue()` with switch for all tag types
5. ✅ Implement helper methods:
   - `readString()` for UTF-8 strings
   - `readByteArray()` for byte arrays
   - `readIntArray()` for int arrays
   - `readLongArray()` for long arrays
   - `readList()` for homogeneous lists
   - `readCompound()` for compound tags
6. ✅ Handle TAG_End properly in compounds
7. ✅ Add error handling and parsing errors
8. ✅ Implement `ReadFrom()` and `WriteTo()` for all NBT types

### Phase 3: Testing ✅ COMPLETED
1. ✅ Create test file `nbt_test.go`
2. ✅ Test each primitive type (byte, short, int, long, float, double)
3. ✅ Test string reading with various lengths
4. ✅ Test array types (byte, int, long)
5. ✅ Test list with different element types
6. ✅ Test compound tags with nested structures
7. ✅ Test error cases (invalid types, truncated data)
8. ✅ Test complex real-world structures (player inventory)
9. ✅ All tests passing

### Phase 4: Integration ✅ COMPLETED
1. ✅ Create `NBTField` wrapper type for packet compatibility
2. ✅ Update generator to use `models.NBTField` instead of `pk.NBTField`
3. ✅ Update `Option` type to handle `models.NBTField`
4. ✅ Update template code generation for NBTField initialization
5. ✅ Code compiles successfully

### Phase 5: NBT Writer ✅ COMPLETED
1. ✅ Implement `WriteTo()` methods for all NBT types
2. ✅ Test round-trip reading and writing
3. ✅ NBTField wrapper supports both read and write

### Phase 6: Refactoring Existing Code (Optional)
1. Review `nbt.go` for code that can be removed or simplified
2. Keep `Nbt2Json` function if still needed for debugging/tools
3. Update copyright notices as appropriate
4. Ensure all code follows project style guidelines

## Implementation Status

**Status**: ✅ **COMPLETED**

All core functionality has been implemented and tested:

- **New Files Created**:
  - `models/nbt_types.go` - NBT type definitions and structures
  - `models/nbt_reader.go` - NBT reading/writing implementation
  - `models/nbt_field.go` - Packet-compatible NBT field wrapper
  - `models/nbt_test.go` - Comprehensive test suite
  - `docs/nbt-parser-design.md` - Design documentation

- **Files Modified**:
  - `models/option.go` - Added support for `models.NBTField`
  - `internal/generator/packets.go` - Updated to generate `models.NBTField` instead of `pk.NBTField`

- **Test Results**: All tests passing (17 test suites, 0 failures)

- **Next Steps**:
  - Regenerate protocol files to replace `pk.NBTField` with `models.NBTField`
  - Remove old `nbt.go` code if no longer needed
  - Update any documentation referencing the old NBT implementation

## Code Style Guidelines

Based on project rules:
- Use `any` instead of `interface{}`
- Avoid package-level variables
- Accept interfaces, return structs
- Constants should have type aliases
- Use lowercase package names
- Avoid single-letter variables (except loop counters)
- Use testify require/assert for tests
- Document all exported types and functions

## Example Usage

```go
// Reading NBT data
reader := NewNBTReader(bytes.NewReader(nbtData))
tag, err := reader.ReadTag()
if err != nil {
    return err
}

// Type assertion to access specific fields
if compound, ok := tag.Value.(*NBTCompound); ok {
    if nameValue, exists := compound.Get("name"); exists {
        if nameStr, ok := nameValue.(*NBTString); ok {
            fmt.Println("Name:", nameStr.Value)
        }
    }
}
```

## Dependencies

- Standard library only:
  - `bytes` - For buffer operations
  - `encoding/binary` - For binary encoding/decoding
  - `io` - For reader/writer interfaces
  - `fmt` - For error messages
  - `math` - For NaN handling in floats

## Testing Strategy

1. **Unit tests** for each NBT type
2. **Integration tests** with real Minecraft NBT data
3. **Benchmark tests** to compare with previous implementation
4. **Fuzzing tests** for robustness (optional)

## Migration Path

- NOT needed, this code hasn't been released yet.  All usage at this point is in local projects. We can just replace usage of NBTField with the new code, although we do need to make sure we implement pk.FieldEncode and pk.FieldDecoder inorder to maintain compatibility with the rest of the module.

1. Implement new NBT parser alongside existing code
2. Test thoroughly with real protocol data
3. Update packet parsing code incrementally
4. Deprecate `pk.NBTField` usage
5. Remove old dependencies once fully migrated

## Open Questions

1. Should we support both BigEndian (Java) and LittleEndian (Bedrock)? NO, just BigEndian (Java)
2. Do we need to preserve the JSON conversion functionality? NO
3. Should NBTCompound use a map or preserve insertion order with a slice?
4. Do we need streaming support for very large NBT structures? No

## License

The NBT parser implementation will maintain the MIT license from the original nbt2json library, with proper attribution to Jim Nelson. The copyright notice will be preserved in all relevant files.

## References

- Original nbt2json library: https://github.com/midnightfreddie/nbt2json
- Minecraft NBT specification: https://wiki.vg/NBT
- Existing implementation: `models/nbt.go`
- NBTField definition: `/home/reallyoldfogie/src/github.com/reallyoldfogie/vendor/github.com/Tnze/go-mc/net/packet/types.go`

