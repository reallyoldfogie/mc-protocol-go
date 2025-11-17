# Add Packet Marshalling to Generated Structs

## Overview
Add the ability for generated packet structs to Marshal/Scan themselves to/from `pk.Packet` format, compatible with the Tnze/go-mc packet library. This will enable seamless integration between our generated protocol structures and the underlying packet transmission layer.

## Key Clarifications

### 1. ReadFrom/WriteTo vs Marshal/Scan
- **ReadFrom/WriteTo**: Field-level serialization, remains unchanged
- **Marshal/Scan**: NEW packet-level methods that wrap ReadFrom/WriteTo
- They work together, not as replacements
- Both will continue to be generated

### 2. Backward Compatibility
- **Not a concern**: Module is unpublished, no compatibility constraints
- Freedom to make breaking changes as needed
- Focus on getting the design right

### 3. Template Refactoring
- **Priority**: Templates are 500+ lines and becoming unmaintainable
- **Action**: Break into smaller, logical components during implementation
- **Goal**: Maintainable code generation without sacrificing functionality

### 4. Testing with mc-agent
- **Project**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-agent`
- **Status**: Already implements version detection and uses PacketMgr
- **Use Case**: Perfect real-world test for the new packet marshalling
- **Benefit**: Will eliminate manual field extraction errors

### 5. Packet Type Identification
- **CRITICAL**: Only types with `packet_` prefix are actual packets
- **Marshal/Scan generation**: Only for types starting with `packet_` from protodef-go
- **ReadFrom/WriteTo generation**: For ALL types (continues as-is)
- **PacketID field**: Only added to `packet_*` types
- **Constructor functions**: Only for `packet_*` types

**IsPacket Flag Flow**:
1. **Input from protodef-go**: Type name = `"packet_clientbound_sound"`
2. **Early detection** (in `processType`): Detect prefix, set `t.IsPacket = true`, store `t.PacketID`
3. **Name conversion**: `"packet_clientbound_sound"` → `"ClientboundSound"` (prefix removed)
4. **Template execution**: Uses `.IsPacket` flag to conditionally generate packet methods
5. **Output**: Struct with Marshal/Scan methods

**Examples**:
- Input: `packet_clientbound_sound`
  - IsPacket: `true`
  - Output struct: Gets Marshal/Scan/PacketID/Constructor + ReadFrom/WriteTo
- Input: `Vec2f`
  - IsPacket: `false`
  - Output struct: Only gets ReadFrom/WriteTo
- Input: `Array`
  - IsPacket: `false`
  - Output: Only gets ReadFrom/WriteTo

## Background

### Current State
- Generated packet structs in `data/<version>/` directories define packet structures
- Packet IDs are defined as constants in `packetid.go` files
- No direct way to convert between struct instances and `pk.Packet` format
- The `PacketMgr` interface provides ID lookups but doesn't handle packet instances

### Target Integration
The Tnze/go-mc packet library defines:
```go
type Packet struct {
    ID   int32
    Data []byte
}

func Marshal[ID ~int32 | int](id ID, fields ...FieldEncoder) (pk Packet)
func (p Packet) Scan(fields ...FieldDecoder) error
```

## Goals

1. **Add Marshal Method**: Generate `Marshal() (pk.Packet, error)` methods that serialize struct fields into a `pk.Packet`
2. **Add Scan Method**: Generate `Scan(pk.Packet) error` methods that deserialize `pk.Packet` into struct fields
3. **Store Packet ID**: Add a private `packetID` field to each packet struct to maintain its ID
4. **Define PacketMarshaller Interface**: Create a common interface that all packet structs implement
5. **Enhance PacketMgr**: Add methods to retrieve packet struct instances by ID that return the PacketMarshaller interface

## Implementation Plan

### Phase 0: Define PacketMarshaller Interface

#### 0.1 Create Interface Definition
- **Location**: New file `models/packet_marshaller.go`
- **Interface Definition**:
  ```go
  package models
  
  import pk "github.com/Tnze/go-mc/net/packet"
  
  // PacketMarshaller is the common interface implemented by all generated packet structs.
  // It provides methods to convert between Go structs and wire-format packets.
  type PacketMarshaller interface {
      // Marshal converts the packet struct into a pk.Packet ready for transmission.
      Marshal() (pk.Packet, error)
      
      // Scan populates the packet struct from a received pk.Packet.
      Scan(packet pk.Packet) error
      
      // PacketID returns the packet's protocol ID.
      PacketID() int32
  }
  ```

#### 0.2 Benefits of Interface Approach
- **Type Safety**: Return type is well-defined, no need for type assertions
- **IDE Support**: Auto-completion and type checking work immediately
- **Documentation**: Interface documents the contract clearly
- **Testability**: Easy to create mock implementations
- **Future-Proof**: Can add methods without breaking existing code

### Phase 1: Modify Packet Struct Generation

#### 1.1 Add Private packetID Field
- **Location**: `internal/generator/packets.go` - packet generation templates
- **Changes**:
  - Add `packetID int32` as first field in generated structs
  - Initialize in constructor/factory functions
  - Keep field unexported to prevent external modification

#### 1.2 Generate PacketID Accessor Method
- **Method Signature**: `func (p *PacketStruct) PacketID() int32`
- **Implementation**: Simple getter for the private field
  ```go
  func (p *{{.PacketName}}) PacketID() int32 {
      return p.packetID
  }
  ```

#### 1.3 Generate Marshal Method
- **Method Signature**: `func (p *PacketStruct) Marshal() (pk.Packet, error)`
- **Implementation**:
  - Use `pk.Marshal()` function from go-mc library
  - Pass `p.packetID` as ID parameter
  - Pass struct fields as `FieldEncoder` arguments
  - Handle nested structs and complex types appropriately
  - Return marshalled packet or error

**Template Example**:
```go
func (p *{{.PacketName}}) Marshal() (pk.Packet, error) {
    return pk.Marshal(p.packetID, {{range .Fields}}
        p.{{.Name}},{{end}}
    ), nil
}
```

#### 1.4 Generate Scan Method
- **Method Signature**: `func (p *PacketStruct) Scan(packet pk.Packet) error`
- **Implementation**:
  - Verify packet ID matches expected ID
  - Use `packet.Scan()` to deserialize fields
  - Pass pointers to struct fields as `FieldDecoder` arguments
  - Return error on ID mismatch or decode failure

**Template Example**:
```go
func (p *{{.PacketName}}) Scan(packet pk.Packet) error {
    if packet.ID != p.packetID {
        return fmt.Errorf("packet ID mismatch: expected %d, got %d", p.packetID, packet.ID)
    }
    return packet.Scan({{range .Fields}}
        &p.{{.Name}},{{end}}
    )
}
```

### Phase 2: Update Packet ID Generation

#### 2.1 Modify PacketID Template
- **Location**: `internal/generator/packets.go` - `packetTemplate` constant
- **Changes**:
  - Generate constructor functions for each packet type
  - Initialize `packetID` field with appropriate constant value
  - Example: `func NewLoginClientboundHello() *LoginClientboundHello { return &LoginClientboundHello{packetID: 0x01} }`

**Note**: Return pointer to enable interface satisfaction without additional allocations

#### 2.2 Update Packet Struct Definitions
- Ensure all generated packet structs include:
  - Private `packetID int32` field
  - Public constructor function returning `*PacketStruct`
  - PacketID() accessor method
  - Marshal/Scan methods
  - All methods satisfy `models.PacketMarshaller` interface

### Phase 3: Enhance PacketMgr Interface

#### 3.1 Add Packet Factory Methods
- **Location**: `data/versions/packetMgr.go` generation
- **New Methods** (return `models.PacketMarshaller` instead of `any`):
  ```go
  type PacketMgr interface {
      // ... existing methods ...
      
      // Clientbound packet factories
      GetClientboundLoginPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      GetClientboundConfigPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      GetClientboundPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      
      // Serverbound packet factories
      GetServerboundLoginPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
      GetServerboundConfigPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
      GetServerboundPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
  }
  ```

#### 3.2 Implementation Strategy
- Generate switch statements mapping IDs to packet constructors
- Return initialized packet struct (pointer) with `packetID` set
- Return as `models.PacketMarshaller` - no casting needed by caller
- Return error for unknown IDs

**Generated Code Example**:
```go
func (p V1_21_1Packet) GetClientboundLoginPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error) {
    switch id {
    case v1_21_1.LoginClientboundHello:
        return v1_21_1.NewLoginClientboundHello(), nil
    case v1_21_1.LoginClientboundGameProfile:
        return v1_21_1.NewLoginClientboundGameProfile(), nil
    // ... more cases
    default:
        return nil, fmt.Errorf("unknown clientbound login packet ID: %d", id)
    }
}
```

### Phase 4: Template Updates and Refactoring

**IMPORTANT**: The templates in `packets.go` have become large and complex. This phase should include refactoring them into smaller, more maintainable components.

#### 4.0 Refactor Template Structure
- **Problem**: Current `structsTmpl` constant is 500+ lines and hard to maintain
- **Solution**: Move templates to external files in a `templates` sub-directory
- **Approach**:
  1. Create `internal/generator/templates/` directory
  2. Move each template definition to its own `.tmpl` file
  3. Update generator code to load templates from files using `embed` or file I/O
  4. Organize templates by type (struct, array, bitfield, switch, etc.)
  5. Each template file contains one logical template definition

**New Template File Structure**:
```
internal/generator/templates/
├── base.tmpl              # Main structsTmpl definition
├── struct.tmpl            # Container/struct template
├── array.tmpl             # Array template
├── bitfield.tmpl          # Bitfield template
├── switch.tmpl            # Switch template
├── mapper.tmpl            # Mapper template
├── registry_holder.tmpl   # Registry entry holder template
├── metadata_loop.tmpl     # Entity metadata loop template
├── headers.tmpl           # baseTypeDefs and typesHeader
└── packet_template.tmpl   # packetTemplate for packet IDs
```

**Benefits of External Templates**:
1. **Maintainability**: Each template is in its own file, easier to find and edit
2. **Syntax Highlighting**: Editor support for `.tmpl` files
3. **Separation of Concerns**: Template logic separated from Go code
4. **Version Control**: Easier to track changes to individual templates
5. **Testing**: Can test templates independently
6. **Readability**: No need for string escaping or concatenation

**Implementation Notes**:
- Use `//go:embed` to embed template files into the binary
- Templates can still reference each other using `{{template "name" .}}`
- Keep template functions registered in Go code (toContainer, toArray, etc.)
- Update `generateTypesFile` to load templates from embedded filesystem

#### 4.1 Update structsTmpl Template
- **Location**: `internal/generator/packets.go` - `structsTmpl` constant
- **Add to structTmpl/containerTmpl block**:
- **Key**: Use `.IsPacket` flag (set during type processing) to conditionally generate packet methods

  ```
  {{define "structTmpl"}}type {{.Name}} struct {
      {{if .IsPacket}}packetID int32  // Private field - only for actual packets
      {{end}}{{range .Fields}}
      {{.Name}} {{.Type.TypeName}}{{end}}
  }
  
  {{if .IsPacket}}
  // Packet-specific methods - only generated for types that had packet_ prefix
  
  // New{{.Name}} creates a new {{.Name}} packet with the correct packet ID.
  func New{{.Name}}() *{{.Name}} {
      return &{{.Name}}{packetID: {{.PacketID}}}
  }
  
  // PacketID returns the protocol ID for this packet type.
  func (p *{{.Name}}) PacketID() int32 {
      return p.packetID
  }
  
  // Marshal serializes the packet into wire format.
  func (p *{{.Name}}) Marshal() (pk.Packet, error) {
      return pk.Marshal(p.packetID, {{range .Fields}}p.{{.Name}},{{end}}), nil
  }
  
  // Scan deserializes a wire-format packet into this struct.
  func (p *{{.Name}}) Scan(packet pk.Packet) error {
      if packet.ID != p.packetID {
          return fmt.Errorf("packet ID mismatch: expected %d, got %d", p.packetID, packet.ID)
      }
      return packet.Scan({{range .Fields}}&p.{{.Name}},{{end}})
  }
  {{end}}
  
  // ReadFrom/WriteTo are ALWAYS generated (for all types, packet or not)
  {{if not (containerHasParentReferences .)}}func (t *{{.Name}}) ReadFrom(r io.Reader) (totalBytes int64, err error) {
      // ... existing ReadFrom implementation ...
  }
  
  func (t *{{.Name}}) WriteTo(w io.Writer) (int64, error) {
      // ... existing WriteTo implementation ...
  }{{end}}
  {{end}}
  ```

#### 4.2 Update Header Templates
- Add necessary imports:
  - `"fmt"` for error messages
  - `pk "github.com/Tnze/go-mc/net/packet"` for packet types
  - `"github.com/reallyoldfogie/mc-protocol-go/models"` (for interface checking/docs)
- Ensure imports are added to both `baseTypeDefs` and `typesHeader` templates

### Phase 5: Generator Code Updates

#### 5.1 Detect and Flag Packet Types Early
- **Location**: `internal/generator/packets.go` - `generateProtocolStructs()` and `processNamespace()`
- **Changes**:
  - Detect types with `packet_` prefix BEFORE name conversion
  - Set `IsPacket = true` flag on these types
  - Map packet names to their IDs from the parsed protocol data
  - Store packet ID in the type metadata
  - Preserve these flags through all type processing stages

**Example Implementation**:
```go
func processType(t *datatypes.Type, baseTypes map[string]string, isAnon bool, isGeneratingBaseTypes bool, packetIDMap map[string]int32) []*datatypes.Type {
    // BEFORE name conversion - check for packet_ prefix
    originalName := strings.ToLower(t.Name)
    if strings.HasPrefix(originalName, "packet_") {
        t.IsPacket = true
        // Look up packet ID from the parsed protocol data
        // packetIDMap is built from getPacketInfo() inversePacketParse data
        if packetID, ok := packetIDMap[originalName]; ok {
            t.PacketID = packetID
            fmt.Printf("DEBUG: Detected packet type '%s' with ID 0x%X\n", originalName, t.PacketID)
        } else {
            fmt.Printf("WARNING: Packet type '%s' has no ID mapping\n", originalName)
        }
    }
    
    // NOW do name conversion
    t.Name = toIdentifier(t.Name) // Removes packet_ prefix and converts to PascalCase
    
    // Continue with rest of processing...
    // The IsPacket flag and PacketID are now preserved
}
```

**Packet ID Source**:
- Packet IDs come from `getPacketInfo()` function (already exists in `packets.go`)
- File location: `.cache/metadata/<version>/data_generator/reports/packets.json`
  - Example: `.cache/metadata/1.21.1/data_generator/reports/packets.json`
- Structure:
  ```json
  {
    "play": {
      "clientbound": {
        "minecraft:sound": { "protocol_id": 99 },
        ...
      },
      "serverbound": {
        "minecraft:use_item": { "protocol_id": 38 },
        ...
      }
    },
    "login": { ... },
    "configuration": { ... }
  }
  ```
- The `getPacketInfo()` function already parses this and creates `inversePacketParse` structure
- Need to build a map from packet type name → packet ID
  - Map key format: `"packet_clientbound_sound"` (matches protodef-go type name)
  - Map value: protocol_id from JSON (e.g., `99`)
- Pass this map to `processType()` for ID lookup during type processing

#### 5.2 Extend Type Metadata to Track Packet Types

**Problem**: By the time templates execute, `packet_` prefix is removed during Go name conversion

**Solution**: Add `IsPacket` flag to type metadata

```go
// In datatypes or generator package
type TypeMetadata struct {
    IsPacket bool  // NEW: Set to true if original name started with "packet_"
    PacketID int32 // Only meaningful if IsPacket == true
}

// Extend existing Type structure (from protodef-go or local)
// Add field to track if this is a packet type
type Type struct {
    Name     string
    TypeName string
    Extras   any
    // ... existing fields ...
    IsPacket bool  // NEW: True if original name had "packet_" prefix
    PacketID int32 // NEW: The packet ID (only valid if IsPacket==true)
}
```

**Implementation Notes**:
- Set `IsPacket = true` when processing types that start with `packet_` (before name conversion)
- Preserve this flag through all processing stages
- Template uses this flag to conditionally generate Marshal/Scan methods

#### 5.3 Update Template Execution
- Pass `PacketInfo` instead of raw types to template
- Ensure packet ID is accessible in templates via `.PacketID`

### Phase 6: PacketMgr Generation Updates

#### 6.1 Update versionsPacketMgr Template
- **Location**: `internal/generator/generator.go` or relevant generation code
- **Update Interface Definition** (use `models.PacketMarshaller` return type):
  ```go
  type PacketMgr interface {
      // ... existing methods ...
      
      // Factory methods by ID - return PacketMarshaller interface
      GetClientboundLoginPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      GetClientboundConfigPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      GetClientboundPacketByID(id models.ClientboundPacketID) (models.PacketMarshaller, error)
      GetServerboundLoginPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
      GetServerboundConfigPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
      GetServerboundPacketByID(id models.ServerboundPacketID) (models.PacketMarshaller, error)
  }
  ```

#### 6.2 Generate Implementation for Each Version
- For each version (V1_21_1Packet, V1_21_2Packet, etc.):
  - Generate 6 new methods (3 clientbound, 3 serverbound)
  - Each method returns `models.PacketMarshaller`
  - Each method contains switch statement mapping IDs to constructors
  - Handle all packet types for that version
  
### Phase 7: Testing Strategy

#### 7.1 Unit Tests
- **Test Marshal**:
  - Create packet struct with known values
  - Marshal to `pk.Packet`
  - Verify ID matches expected
  - Verify Data field contains correct serialized data

- **Test Scan**:
  - Create `pk.Packet` with known ID and data
  - Scan into struct
  - Verify all fields populated correctly

- **Test Round-Trip**:
  - Create struct → Marshal → Scan → Compare

- **Test Interface Satisfaction**:
  - Verify all packet structs implement `models.PacketMarshaller`
  - Test that interface methods work without casting

#### 7.2 Integration Tests
- Test with actual protocol flow (login, config, play states)
- Verify compatibility with Tnze/go-mc connection handling
- Test edge cases: empty packets, optional fields, complex nested structures

#### 7.4 mc-agent Integration Test
- **Test Project**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-agent/daze/main.go`
- **Purpose**: Real-world proof-of-concept for version-agnostic bot
- **Current Status**: Already has version detection and PacketMgr setup
- **Current Issue**: Manual field extraction with `p.Scan()` - doesn't use generated packet structs

**Current mc-agent implementation** (lines 79-97, 176-198):
```go
// Version detection already working
func init() {
    *mcVersion, protocolVersion, err = rof_utils.CheckServerVersion(*address, 0)
    packetMgr = mc_versions.GetPacketMgrForVersion(*mcVersion)
    blockMgr = mc_versions.GetBlockMgrForVersion(*mcVersion)
    soundMgr = mc_versions.GetSoundMgrForVersion(*mcVersion)
}

// Current packet handling - manual field extraction
var soundListener = bot.PacketHandler{
    ID: packetMgr.GetClientboundPacketID("ClientboundSound"),
    F: func(p pk.Packet) error {
        var SoundID pk.VarInt
        var SoundCategory pk.VarInt
        var X, Y, Z pk.Int
        var Volume, Pitch pk.Float
        var Seed pk.Long
        if err := p.Scan(&SoundID, &SoundCategory, &X, &Y, &Z, &Volume, &Pitch, &Seed); err != nil {
            return err
        }
        return onSound(int32(SoundID), ...)
    },
}
```

**New approach with generated packet structs**:
```go
// Version detection remains the same - already working
func init() {
    *mcVersion, protocolVersion, err = rof_utils.CheckServerVersion(*address, 0)
    packetMgr = mc_versions.GetPacketMgrForVersion(*mcVersion)
    blockMgr = mc_versions.GetBlockMgrForVersion(*mcVersion)
    soundMgr = mc_versions.GetSoundMgrForVersion(*mcVersion)
}

// New packet handling - use generated structs
var soundListener = bot.PacketHandler{
    ID: packetMgr.GetClientboundPacketID("ClientboundSound"),
    F: func(p pk.Packet) error {
        // Get the appropriate packet struct for this version
        packet, err := packetMgr.GetClientboundPacketByID(p.ID)
        if err != nil {
            return err
        }
        
        // Scan into struct - no manual field list!
        if err := packet.Scan(p); err != nil {
            return err
        }
        
        // Access fields via the interface
        soundID := packet.PacketID()
        
        // For field access, type assert to concrete type
        // (This is necessary because field names/types may vary by version)
        if soundPacket, ok := packet.(interface {
            GetSoundID() pk.VarInt
            GetSoundCategory() pk.VarInt
            GetX() pk.Int
            GetY() pk.Int
            GetZ() pk.Int
            GetVolume() pk.Float
            GetPitch() pk.Float
            GetSeed() pk.Long
        }); ok {
            return onSound(
                int32(soundPacket.GetSoundID()),
                int32(soundPacket.GetSoundCategory()),
                float64(soundPacket.GetX())/8,
                float64(soundPacket.GetY())/8,
                float64(soundPacket.GetZ())/8,
                float32(soundPacket.GetVolume()),
                float32(soundPacket.GetPitch()),
                int32(soundPacket.GetSeed()),
            )
        }
        
        return fmt.Errorf("unexpected packet type: %T", packet)
    },
}
```

**Key Benefits for mc-agent**:
1. ✓ Version detection already implemented
2. ✓ PacketMgr already in use
3. **NEW**: Type-safe packet structs instead of manual field lists
4. **NEW**: No more forgetting fields or getting order wrong
5. **NEW**: Compile-time checking of field types
6. **NEW**: Easier to maintain when packet structures change

#### 7.3 PacketMgr Tests
- Test factory methods for all packet types
- Verify error handling for unknown IDs
- Test that returned interface works directly (no casting needed)
- Verify all returned packets satisfy interface

### Phase 8: Documentation

#### 8.1 Code Documentation
- Add godoc comments to:
  - `models.PacketMarshaller` interface
  - Marshal/Scan methods
  - PacketID() accessor
  - Constructor functions
  - PacketMgr factory methods

#### 8.2 Usage Examples
Create examples showing:
```go
// Creating and marshalling a packet
packet := v1_21_1.NewLoginClientboundHello()
packet.ServerID = "example"
pk, err := packet.Marshal()
if err != nil {
    // handle error
}

// Scanning a received packet - no casting needed!
mgr := versions.GetPacketMgrForVersion("1.21.1")
packet, err := mgr.GetClientboundLoginPacketByID(pk.ID)
if err != nil {
    // handle error
}
// packet is already a PacketMarshaller, use it directly
err = packet.Scan(pk)
if err != nil {
    // handle error
}

// Access the populated data with type assertion to specific packet
if hello, ok := packet.(*v1_21_1.LoginClientboundHello); ok {
    fmt.Println("Server ID:", hello.ServerID)
}
```

## Technical Considerations

### ReadFrom/WriteTo vs Marshal/Scan Relationship

**They serve different purposes and work together:**

1. **ReadFrom/WriteTo (Field-level serialization)**:
   - Used for individual field types (pk.VarInt, pk.String, custom structs, etc.)
   - Implements `io.ReaderFrom` and `io.WriterTo` interfaces
   - Works with raw byte streams
   - Currently generated for all struct types
   - Example: `Vec2f.ReadFrom()` reads X and Y fields from a reader

2. **Marshal/Scan (Packet-level serialization)**:
   - Used for complete packets (with ID + data)
   - Works with `pk.Packet` which contains ID and Data fields
   - `Marshal()` uses field WriteTo methods internally via `pk.Marshal(id, fields...)`
   - `Scan()` uses field ReadFrom methods internally via `packet.Scan(&fields...)`
   - Example from mc-agent:
     ```go
     // Marshal packet for sending
     c.Conn.WritePacket(pk.Marshal(packetid.ServerboundUseItem, pk.VarInt(hand)))
     
     // Scan received packet
     if err := p.Scan(&SoundName, &SoundCategory, &X, &Y, &Z, &Volume, &Pitch); err != nil {
         return err
     }
     ```

**How they work together:**
- Marshal/Scan are convenience wrappers at the packet level
- They call ReadFrom/WriteTo on individual fields under the hood
- ReadFrom/WriteTo remain essential for field-level operations
- Both will continue to be generated

**Benefits of adding Marshal/Scan:**
- Encapsulates packet ID with the struct (no need to pass it separately)
- Type-safe packet handling (struct knows its own ID)
- Cleaner API: `packet.Marshal()` vs `pk.Marshal(someID, packet.Field1, packet.Field2, ...)`
- Validation: Scan can verify packet ID matches expected type

### Interface vs Any
**Why `models.PacketMarshaller` is better than `any`:**
- Callers know exactly what methods are available
- No need for type assertions to call Marshal/Scan
- Compiler enforces that all packet structs implement the interface
- Better IDE support and documentation
- Clearer contract between PacketMgr and consumers

### Field Encoder/Decoder Compatibility
- All generated field types must implement `FieldEncoder` and `FieldDecoder` interfaces
- Complex types (Arrays, Options, etc.) already implement these via basetypes
- Nested container types will need their own Marshal/Scan methods (only if they are packets)

### Packet ID Type Safety
- Use `int32` for consistency with go-mc library (cast as needed when passing to go-mc - internal types should be `models.ClientboundPacketID` and `models.ServerboundPacketID`)
- Ensure no conflicts between different packet states (Login, Config, Play)
- PacketID constants already typed as `models.ClientboundPacketID` and `models.ServerboundPacketID`

### Pointer Receivers
- Marshal/Scan methods use pointer receivers for consistency
- Constructors return pointers (`*PacketStruct`) for efficiency
- Interface is satisfied by pointer types

### Generator Template Complexity
- Templates will become more complex with Marshal/Scan generation
- Consider extracting Marshal/Scan generation to separate template definitions
- Ensure proper indentation and formatting in generated code

### Error Handling
- Marshal should rarely error (mostly for encoding failures)
- Scan must validate packet ID before decoding
- Return meaningful errors for debugging

### Performance Considerations
- Marshal/Scan add minimal overhead (single function call)
- PacketID stored in struct (4 bytes) - acceptable tradeoff
- Factory methods use switch statements (O(1) average case)
- Pointer returns avoid unnecessary allocations

## Implementation Order

1. **Phase 0**: Create `models.PacketMarshaller` interface
2. **Phase 1 & 2**: Modify struct generation to add packetID field and implement interface
3. **Phase 4**: Update templates with proper Marshal/Scan/PacketID implementations
4. **Phase 5**: Wire up packet IDs in generator code
5. **Phase 3 & 6**: Enhance PacketMgr with factory methods returning interface
6. **Phase 7**: Write comprehensive tests
7. **Phase 8**: Document usage patterns

## Files to Modify

### New Files
1. `models/packet_marshaller.go` - Interface definition

### Primary Files
1. `internal/generator/packets.go` - Core packet generation logic
2. `internal/generator/generator.go` - PacketMgr generation
3. `data/versions/packetMgr.go` - Generated file (template changes)
4. Template constants in `packets.go`:
   - `structsTmpl`
   - `packetTemplate`
   - `baseTypeDefs` / `typesHeader`

### Test Files (To Create)
1. `internal/generator/packets_marshal_test.go`
2. `data/1.21.1/packets_test.go` (example version)
3. `models/packet_marshaller_test.go` (interface tests)

## Risks & Mitigation

### Risk: Breaking Existing Code
- **Status**: Not a concern - module is unpublished
- **Note**: Since the module hasn't been published yet, we have full freedom to make breaking changes
- **Validation**: Test with mc-agent project to ensure it works with the new API

### Risk: Template Complexity
- **Mitigation**: Break down templates into smaller, reusable components
- **Validation**: Generate code for all versions and verify compilation

### Risk: Performance Impact
- **Mitigation**: Profile generated code, ensure no allocations in hot paths
- **Validation**: Benchmark Marshal/Scan operations

### Risk: Incomplete Field Type Coverage
- **Mitigation**: Audit all field types for FieldEncoder/Decoder implementation
- **Validation**: Comprehensive integration tests

### Risk: Interface Evolution
- **Mitigation**: Design interface carefully upfront; add methods rarely
- **Validation**: Consider future needs (validation, pretty-printing)

## Future Enhancements

1. **Packet Validation**: Add `Validate() error` to interface
2. **Pretty Printing**: Add `String() string` for debugging
3. **JSON Marshalling**: Support JSON encoding for logging/debugging
4. **Packet Pooling**: Reuse packet structs to reduce allocations
5. **Generic Helpers**: Type-safe wrappers for common packet operations

## Implementation Status

### Phase 0: Define PacketMarshaller Interface ✓
- **Status**: COMPLETE
- Created `models/packet_marshaller.go` with PacketMarshaller interface
- Interface includes Marshal(), Scan(), and PacketID() methods

### Phase 1 & 2: Modify Packet Struct Generation ✓
- **Status**: COMPLETE
- Added `TypeWithPacketInfo` wrapper struct to track packet metadata (IsPacket, PacketID)
- Added private `packetID int32` field to packet structs
- Generated constructor functions (New{PacketName}())
- Generated PacketID() accessor methods

### Phase 3: Detect and Flag Packet Types ✓
- **Status**: COMPLETE
- Built `buildPacketIDMap()` function that parses packets.json
- Maps protodef-go packet type names to protocol IDs
- Fixed int64 to int32 type conversions
- Modified `processType()` to detect `packet_` prefix BEFORE name conversion
- Stores packet metadata in global `packetMetadata` map

### Phase 4: Update Templates ✓
- **Status**: COMPLETE
- Updated `structTmpl` to conditionally generate packet methods
- Added Marshal() and Scan() methods for packet types
- Methods filter out `struct{}` and `any` fields (can't be marshalled)
- Added necessary imports (fmt, pk)
- Note: Template refactoring (Phase 4.0) deferred - current priority is functionality

### Phase 5: Generator Code Updates ✓
- **Status**: COMPLETE
- Updated all `processType()` calls to include `packetIDMap` parameter
- Packet ID lookup happens during type processing
- Metadata preserved through all processing stages
- Made package names lowercase using `strings.ToLower(boundName)`

### Phase 6: PacketMgr Generation Updates ✓
- **Status**: COMPLETE
- Updated PacketMgr interface with factory methods returning `models.PacketMarshaller`
- Generated factory method implementations for each version
- Factory methods use switch statements mapping IDs to constructors

### Phase 7: Testing Strategy
- **Status**: IN PROGRESS - Build validation phase
- **Current Issue**: Factory method constructor names don't match generated struct names
  - Factory methods call: `NewLoginDisconnect()`, `NewHello()`, etc. (based on packets.json names)
  - Actual constructors: `NewDisconnect()`, `NewEncryptionBegin()`, etc. (based on protodef-go struct names)
  - **Root Cause**: Factory generator uses packets.json names, but should use protodef-go struct names
  - **Next Step**: Fix factory method generation to map packet IDs to correct struct constructor names

### Phase 8: Documentation
- **Status**: NOT STARTED
- Deferred until implementation is complete and tested

### Phase 9: Code Generation for Version 1.21.5 ⚠️
- **Status**: GENERATED BUT NOT COMPILING
- Generated code for version 1.21.5
- Package naming fixed (now lowercase)
- Factory method package paths corrected
- **Blocking Issue**: Constructor name mismatch prevents compilation

## Success Criteria

1. ✓ `models.PacketMarshaller` interface defined and documented
2. ✓ All generated packet structs implement the interface
3. ⚠️ Round-trip test passes (struct → packet → struct) - BLOCKED by compilation
4. ✓ PacketMgr returns `models.PacketMarshaller` (not `any`)
5. ✓ Usage requires no type assertions to call Marshal/Scan
6. ⚠️ All existing tests continue to pass - BLOCKED by compilation
7. ⚠️ Integration test with go-mc library succeeds - BLOCKED by compilation
8. ❌ Generated code compiles without errors or warnings - CURRENT BLOCKER
9. ⚠️ Documentation covers all new functionality - DEFERRED

## References

- Tnze/go-mc packet library: `/home/reallyoldfogie/src/github.com/reallyoldfogie/vendor/github.com/Tnze/go-mc/net/packet/packet.go`
- Current packet generation: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`
- PacketMgr interface: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/versions/packetMgr.go`
- Models package: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/models/`
- **Packet ID mappings**: `.cache/metadata/<version>/data_generator/reports/packets.json`
  - Contains protocol_id for each packet type
  - Used to map packet names to IDs during generation
- **Test/proof-of-concept**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-agent/daze/main.go`
  - Real-world usage example with version detection
  - Target for integration testing
