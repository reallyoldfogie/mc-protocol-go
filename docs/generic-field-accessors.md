# Generic Field Accessor Interfaces

## Overview
When a field appears with different types across protocol versions, the field accessor system generates **generic interfaces with type constraints** instead of falling back to `any`. This preserves type safety while supporting multiple types.

## How It Works

### Field Discovery
During code generation, the system:
1. Scans all protocol versions for each field name
2. Tracks all unique types seen for that field
3. If multiple types exist, generates a generic interface with a type constraint

### Type Constraints
For fields with multiple types, a type constraint interface is generated:

```go
// IdType is a type constraint for the Id field.
// This field has different types across versions: any | pk.Int | pk.Long | pk.String | pk.VarInt
type IdType interface {
    any | pk.Int | pk.Long | pk.String | pk.VarInt
}
```

### Generic Interfaces
The getter/setter interfaces then use this constraint:

```go
// IdGetter provides read access to the Id field.
// This is a generic interface because the field has different types across versions.
type IdGetter[T IdType] interface {
    GetId() T
}

// IdSetter provides write access to the Id field.
type IdSetter[T IdType] interface {
    SetId(T)
}
```

## Usage Examples

### Single Type (Most Common)
For fields that always have the same type:

```go
// Non-generic interface
type TransactionIdGetter interface {
    GetTransactionId() pk.VarInt
}

// Usage
if getter, ok := pkt.(models.TransactionIdGetter); ok {
    id := getter.GetTransactionId() // Returns pk.VarInt
}
```

### Multiple Types (Generic)
For fields with different types across versions:

```go
// Generic interface with type parameter
type CountGetter[T CountType] interface {
    GetCount() T
}

// CountType constraint: pk.Short | pk.VarInt

// Usage - specify the concrete type
if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
    count := getter.GetCount() // Returns pk.VarInt
}

// Or check for pk.Short
if getter, ok := pkt.(models.CountGetter[pk.Short]); ok {
    count := getter.GetCount() // Returns pk.Short
}
```

### Version-Agnostic Code
When you don't know which type a packet uses:

```go
func processCount(pkt models.PacketMarshaller) int32 {
    // Try pk.VarInt first (more common)
    if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
        return int32(getter.GetCount())
    }
    // Fall back to pk.Short
    if getter, ok := pkt.(models.CountGetter[pk.Short]); ok {
        return int32(getter.GetCount())
    }
    return -1 // Field not present
}
```

## Examples of Generic Fields

### Count Field
- **Types**: `pk.Short | pk.VarInt`
- **Versions**: Varies by packet
- **Usage**: Message acknowledgements, item stacks, etc.

```go
type CountType interface {
    pk.Short | pk.VarInt
}

type CountGetter[T CountType] interface {
    GetCount() T
}
```

### Id Field
- **Types**: `pk.Int | pk.Long | pk.String | pk.VarInt | any`
- **Versions**: Varies by packet type and version
- **Usage**: Entity IDs, packet IDs, transaction IDs, etc.

```go
type IdType interface {
    any | pk.Int | pk.Long | pk.String | pk.VarInt
}

type IdGetter[T IdType] interface {
    GetId() T
}
```

### EntityId Field
- **Types**: `pk.Int | pk.VarInt | any`
- **Versions**: Varies across versions
- **Usage**: Entity references

```go
type EntityIdType interface {
    any | pk.Int | pk.VarInt
}

type EntityIdGetter[T EntityIdType] interface {
    GetEntityId() T
}
```

## Benefits

### Type Safety
✅ **Compile-time type checking** - No runtime type assertions needed for field values
✅ **IDE support** - Auto-completion and type hints work correctly
✅ **Refactoring safety** - Type changes are caught at compile time

### Clarity
✅ **Explicit types** - You know exactly what types are possible
✅ **Documentation** - Type constraints document version differences
✅ **Discoverability** - Easy to see what types a field can have

### Performance
✅ **Zero overhead** - Generic instantiations are resolved at compile time
✅ **No allocations** - Type parameters don't require heap allocations
✅ **Direct access** - Same performance as non-generic interfaces

## Comparison with Old Approach

### Before (Using `any`)
```go
type IdGetter interface {
    GetId() any  // Lost type information!
}

// Usage
if getter, ok := pkt.(models.IdGetter); ok {
    id := getter.GetId()
    // Must type-assert: id.(pk.Int), id.(pk.VarInt), etc.
    if varIntId, ok := id.(pk.VarInt); ok {
        // Use varIntId
    }
}
```

### After (Using Generics)
```go
type IdGetter[T IdType] interface {
    GetId() T  // Type-safe!
}

// Usage
if getter, ok := pkt.(models.IdGetter[pk.VarInt]); ok {
    id := getter.GetId() // Already pk.VarInt, no assertion needed!
    // Use id directly
}
```

## When Interfaces Are Generated

### Single Type → Non-Generic
If all versions use the same type:
```go
type TransactionIdGetter interface {
    GetTransactionId() pk.VarInt
}
```

### Multiple Types → Generic
If different versions use different types:
```go
type CountGetter[T CountType] interface {
    GetCount() T
}
```

## Backward Compatibility

The generic interface approach is:
- ✅ **Fully backward compatible** - Packet getter/setter methods are unchanged
- ✅ **Opt-in** - Use generics when needed, ignore them otherwise
- ✅ **Migration friendly** - Can gradually adopt generic interfaces

Direct method calls still work identically:
```go
pkt := serverbound.NewMessageAcknowledgement()
pkt.SetCount(42)  // Still works exactly the same
count := pkt.GetCount()  // Returns pk.VarInt
```

## Additional Resources

- See `docs/field-access-patterns.md` for general usage patterns
- See `example_usage.go` for working examples
- See test files for comprehensive usage examples
