# Field Access Patterns

This document describes the type-safe field accessor pattern for working with packet structures across different Minecraft protocol versions.

## Overview

The protocol generator creates getter/setter methods for each field in every packet struct. These methods implement version-agnostic interfaces defined in `models/field_accessors.go`, allowing you to write code that works across multiple protocol versions without knowing the specific packet structure.

## Quick Start

### Direct Field Access (When You Know the Type)

```go
import v1_21_6 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/serverbound"

// Create packet
pkt := v1_21_6.NewMessageAcknowledgement()

// Direct field access
pkt.SetCount(42)
count := pkt.GetCount()
```

### Version-Agnostic Access (Recommended)

```go
import (
    pk "github.com/Tnze/go-mc/net/packet"
    "github.com/reallyoldfogie/mc-protocol-go/models"
)

// Function that works with ANY packet that has a Count field
func processPacket(pkt models.PacketMarshaller) {
    // Check if packet has Count field (Count can be pk.VarInt or pk.Short)
    if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
        count := getter.GetCount()
        fmt.Printf("Count: %d\n", count)
    }
    
    // Modify if writable
    if setter, ok := pkt.(models.CountSetter[pk.VarInt]); ok {
        setter.SetCount(100)
    }
}
```

## Common Patterns

### Pattern 1: Read Field If Present

```go
func getCount(pkt models.PacketMarshaller) (pk.VarInt, bool) {
    // Specify the type parameter for generic interfaces
    if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
        return getter.GetCount(), true
    }
    return 0, false
}
```

### Pattern 2: Modify Field If Present

```go
func setCount(pkt models.PacketMarshaller, value pk.VarInt) bool {
    // Specify the type parameter for generic interfaces
    if setter, ok := pkt.(models.CountSetter[pk.VarInt]); ok {
        setter.SetCount(value)
        return true
    }
    return false
}
```

### Pattern 3: Work With Multiple Fields

```go
// For fields with multiple types, check each field individually
func extractPosition(pkt models.PacketMarshaller) (x, y, z float64, ok bool) {
    // X, Y, Z are generic - try common type (pk.Double)
    xGetter, hasX := pkt.(models.XGetter[pk.Double])
    yGetter, hasY := pkt.(models.YGetter[pk.Double])
    zGetter, hasZ := pkt.(models.ZGetter[pk.Double])
    
    if hasX && hasY && hasZ {
        return float64(xGetter.GetX()), float64(yGetter.GetY()), float64(zGetter.GetZ()), true
    }
    return 0, 0, 0, false
}
```

### Pattern 4: Generic Field Processor

```go
// Process any packet, checking for known fields
func processAnyPacket(pkt models.PacketMarshaller) {
    // Try different field types (EntityId is generic)
    if getter, ok := pkt.(models.EntityIdGetter[pk.VarInt]); ok {
        fmt.Printf("Entity ID: %v\n", getter.GetEntityId())
    }
    
    // Action is not generic (single type)
    if getter, ok := pkt.(models.ActionGetter); ok {
        fmt.Printf("Action: %v\n", getter.GetAction())
    }
    
    // ... check other fields as needed
}
```

## Migration from Old API

### Old API (Deprecated)

```go
// ❌ Old way - runtime type assertions, no compile-time safety
fields := pkt.GetFields()
if countField, ok := fields["Count"]; ok {
    count := countField.(pk.VarInt)  // Type assertion required
    // use count
}

// Setting values
pkt.SetFields(map[string]pk.FieldEncoder{
    "Count": pk.VarInt(42),
})
```

### New API (Recommended)

```go
// ✅ New way - type-safe, compile-time checked
if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
    count := getter.GetCount()  // Already pk.VarInt, fully type-safe
    // use count
}

// Setting values
if setter, ok := pkt.(models.CountSetter[pk.VarInt]); ok {
    setter.SetCount(42)
}
```

## Available Interfaces

All field accessor interfaces are defined in `models/field_accessors.go`. Each field has a getter and setter interface pair:

### Non-Generic Interfaces (Single Type)
For fields that always have the same type across versions:
- `TransactionIdGetter` / `TransactionIdSetter` - For TransactionId fields (pk.VarInt)
- `HostGetter` / `HostSetter` - For Host fields (pk.String)
- `HealthGetter` / `HealthSetter` - For Health fields (pk.Float)
- ... and many more

### Generic Interfaces (Multiple Types)
For fields that have different types across versions, use type parameters:
- `CountGetter[T CountType]` / `CountSetter[T CountType]` - Types: `pk.Short | pk.VarInt`
- `EntityIdGetter[T EntityIdType]` / `EntityIdSetter[T EntityIdType]` - Types: `any | pk.Int | pk.VarInt`
- `IdGetter[T IdType]` / `IdSetter[T IdType]` - Types: `any | pk.Int | pk.Long | pk.String | pk.VarInt`
- `XGetter[T XType]`, `YGetter[T YType]`, `ZGetter[T ZType]` - Types: `any | pk.Double | pk.Int`
- ... and many more

**Note**: Over 400+ field accessor interfaces total as of 1.21.6. See `docs/generic-field-accessors.md` for details on generic interfaces.

## Type Handling

### Concrete Types (pk.* types)

Fields with concrete types from the `go-mc/net/packet` package are preserved:
- `pk.VarInt`, `pk.String`, `pk.Boolean`, `pk.Double`, etc.

### Generic Interfaces (Multiple Types)

**New in latest version**: When a field appears with different types across protocol versions, generic interfaces with type constraints are used instead of `any`:

```go
// Type constraint
type CountType interface {
    pk.Short | pk.VarInt
}

// Generic interfaces
type CountGetter[T CountType] interface {
    GetCount() T
}

type CountSetter[T CountType] interface {
    SetCount(T)
}
```

**Usage**:
```go
// Specify the concrete type you expect
if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
    count := getter.GetCount()  // Returns pk.VarInt directly!
}

// Or try alternative types
if getter, ok := pkt.(models.CountGetter[pk.Short]); ok {
    count := getter.GetCount()  // Returns pk.Short directly!
}
```

**Benefits**:
- ✅ Full compile-time type safety
- ✅ No runtime type assertions needed
- ✅ IDE auto-completion works perfectly
- ✅ Zero performance overhead
- ✅ Self-documenting (constraints show possible types)

See `docs/generic-field-accessors.md` for comprehensive documentation.

### Version-Specific Types

Fields with version-specific types (like custom structs or basetypes) use `any`:
- This allows the interfaces to compile without dependencies on specific versions
- When you access via the concrete packet type, you get the actual type
- When you access via the interface, you get `any` and can use type assertions if needed

### Example

```go
// Via concrete type (version 1.21.6)
pkt := v1_21_6.NewSomePacket()
data := pkt.GetData()  // Returns actual type, e.g., SomeDataStruct

// Via interface (version-agnostic)
var pkInterface models.PacketMarshaller = pkt
if getter, ok := pkInterface.(models.DataGetter); ok {
    data := getter.GetData()  // Returns 'any'
    // Type assert if you need specific type
    if specificData, ok := data.(SomeDataStruct); ok {
        // use specificData
    }
}
```

## Advanced Usage

### Building Version-Agnostic Protocol Handlers

```go
type PacketHandler struct {
    handlers map[int32]func(models.PacketMarshaller)
}

func (h *PacketHandler) Handle(pkt models.PacketMarshaller) {
    // Try to extract common fields
    if idGetter, ok := pkt.(models.EntityIdGetter); ok {
        entityId := idGetter.GetEntityId()
        h.handleEntityPacket(pkt, entityId)
    }
    
    // Fallback to packet ID-based routing
    if handler, ok := h.handlers[pkt.PacketID()]; ok {
        handler(pkt)
    }
}
```

### Working with Dynamic Field Names

If you need to work with field names determined at runtime, use the reflection-based helpers:

```go
import "github.com/reallyoldfogie/mc-protocol-go/models"

// Dynamic field access (when field name comes from config/user input)
fieldName := getFieldNameFromConfig()
if val, ok := models.GetPacketFieldAs[pk.VarInt](pkt, fieldName); ok {
    fmt.Printf("%s: %d\n", fieldName, val)
}
```

## Performance Considerations

### Interface Method Calls

- Interface method calls have ~1ns overhead (negligible)
- Methods directly access struct fields (no reflection)
- No allocations for primitive types

### Comparison with Old API

| Operation | Old API (GetFields) | New API (Interfaces) | Speedup |
|-----------|---------------------|----------------------|---------|
| Field access | Map lookup + type assertion | Direct method call | ~5-10x |
| Memory | Allocates map | No allocation | ♾️ |
| Type safety | Runtime only | Compile-time | N/A |

## Best Practices

1. **Use interfaces for version-agnostic code**
   ```go
   func process(pkt models.PacketMarshaller) {
       if getter, ok := pkt.(models.CountGetter); ok {
           // ...
       }
   }
   ```

2. **Use direct access when you know the type**
   ```go
   pkt := v1_21_6.NewMessageAcknowledgement()
   pkt.SetCount(42)  // Direct field access
   ```

3. **Always check before accessing**
   ```go
   // ✅ Good - check first
   if getter, ok := pkt.(models.CountGetter); ok {
       count := getter.GetCount()
   }
   
   // ❌ Bad - assumes field exists
   getter := pkt.(models.CountGetter)  // Panics if not implemented!
   ```

4. **Combine multiple field checks for related data**
   ```go
   type EntityPacket interface {
       models.EntityIdGetter
       models.XGetter
       models.YGetter
       models.ZGetter
   }
   ```

## Troubleshooting

### "undefined: models.SomeFieldGetter"

The field may not exist in any version. Check `models/field_accessors.go` for available interfaces.

### Field returns `any` instead of specific type

This is expected for version-specific types. Use type assertion if you need the concrete type:

```go
if getter, ok := pkt.(models.DataGetter); ok {
    data := getter.GetData()  // Returns 'any'
    if concreteData, ok := data.(SpecificType); ok {
        // use concreteData
    }
}
```

### Deprecated warnings

If you see deprecation warnings for `GetFields()` or `SetFields()`, migrate to the new getter/setter methods:

```go
// Old: pkt.GetFields()["Count"]
// New: pkt.GetCount()

// Old: pkt.SetFields(map[string]pk.FieldEncoder{"Count": pk.VarInt(42)})
// New: pkt.SetCount(42)
```

## Examples

See `models/field_accessors_integration_test.go` for comprehensive examples of all patterns.

## See Also

- [Generic Field Accessors](generic-field-accessors.md) - Comprehensive guide to generic interfaces
- [Implementation Plan](../Implementation_Plan_Bitflags_and_Parent_References.md) - Original design document
- [models/field_accessors.go](../models/field_accessors.go) - Generated interface definitions
- [models/packet_marshaller.go](../models/packet_marshaller.go) - PacketMarshaller interface and reflection helpers
- Test files (`models/*_test.go`) - Working examples and patterns
