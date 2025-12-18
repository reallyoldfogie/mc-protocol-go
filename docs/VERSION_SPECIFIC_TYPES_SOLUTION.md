# Version-Specific Types: Problem & Solution

## The Problem

When implementing type-safe field accessors with generic interfaces, we encountered a fundamental issue:

**Version-specific types (like `PlayerInfoActionBitflags`) cannot satisfy generic interface constraints.**

### Example of the Issue

Consider the `Action` field in packets:
- In `PlayerInfo` packet (v1.21.5): `Action PlayerInfoActionBitflags` (embeds `pk.UnsignedByte`)
- In `AdvancementTab` packet: `Action pk.VarInt`

The field discovery system generates:
```go
type ActionType interface {
    Bitflags | pk.Byte | pk.UnsignedByte | pk.VarInt
}

type ActionGetter[T ActionType] interface {
    GetAction() T
}
```

But `PlayerInfoActionBitflags` is **not** in this constraint—it's a wrapper struct that embeds `pk.UnsignedByte`, making it incompatible with the constraint.

### Why Version-Specific Types Exist

Version-specific types like `PlayerInfoActionBitflags` are generated wrapper structs that provide named accessor methods over bitflag fields:

```go
type PlayerInfoActionBitflags struct {
    pk.UnsignedByte
}

func (bf PlayerInfoActionBitflags) AddPlayer() bool { /* ... */ }
func (bf PlayerInfoActionBitflags) InitializeChat() bool { /* ... */ }
// etc.
```

These types are **essential** because they:
1. Provide semantic meaning to bitflag values
2. Give IDE autocomplete for available flags
3. Prevent errors from manually calculating bit positions
4. Are version-specific (different versions have different flags)

## The Solution: Three Access Patterns

Since version-specific types cannot be used in version-agnostic interfaces, we provide **three complementary access patterns**, each suited for different use cases.

### Pattern 1: Version-Agnostic Access (Dynamic/Runtime)

**When to use**: Working with version-specific types or dynamic packet handling

**How it works**: Use `GetFields()`/`SetFields()` with runtime type checking

```go
fields := pkt.GetFields()
if action, ok := fields["Action"]; ok {
    switch v := action.(type) {
    case v1211.PlayerInfoActionBitflags:
        fmt.Printf("v1.21.1: AddPlayer=%v\n", v.AddPlayer())
    case v1216.PlayerInfoActionBitflags:
        fmt.Printf("v1.21.6: AddPlayer=%v\n", v.AddPlayer())
    case pk.VarInt:
        fmt.Printf("VarInt: %d\n", v)
    }
}
```

**Pros**:
- Works with any packet from any version
- Can access all fields including version-specific types
- Good for proxies, debugging tools, generic handlers

**Cons**:
- No compile-time type safety
- Requires runtime type assertions
- More verbose

### Pattern 2: Semi-Agnostic Access (Stable Primitives)

**When to use**: Fields with stable primitive types across versions

**How it works**: Use typed interfaces with generics

```go
// Single type across versions
if getter, ok := pkt.(models.CountGetter); ok {
    count := getter.GetCount() // pk.VarInt
}

// Multiple types, try each
if getter, ok := pkt.(models.EntityIdGetter[pk.Int]); ok {
    id := getter.GetEntityId() // pk.Int
} else if getter, ok := pkt.(models.EntityIdGetter[pk.VarInt]); ok {
    id := getter.GetEntityId() // pk.VarInt
}
```

**Pros**:
- Type-safe for primitive types
- Works across versions
- No need to import version-specific packages

**Cons**:
- **Only works for fields with primitive types**
- Cannot access version-specific types (bitflags wrappers, custom structs)
- Need to try multiple type parameters for multi-type fields

### Pattern 3: Version-Specific Access (Full Type Safety)

**When to use**: You know the protocol version

**How it works**: Import version-specific package and use typed methods

```go
import v1215 "github.com/.../data/1.21.5/play/clientbound"

pkt := v1215.NewPlayerInfo()
action := pkt.GetAction() // PlayerInfoActionBitflags with full type info

if action.AddPlayer() {
    fmt.Println("Add player flag is set")
}

newAction := v1215.PlayerInfoActionBitflags{}
newAction.SetAddPlayer(true)
pkt.SetAction(newAction)
```

**Pros**:
- Full compile-time type safety
- IDE autocomplete and refactoring support
- Direct access to version-specific types and methods
- Most efficient (no interface overhead)

**Cons**:
- Requires knowing the version
- Code is version-specific
- Need separate code paths for different versions

## Summary Table

| Pattern | Type Safety | Version Agnostic | Version-Specific Types | Use Case |
|---------|-------------|------------------|------------------------|----------|
| **Pattern 1** (GetFields) | Runtime only | ✅ Yes | ✅ Yes | Proxies, debugging, dynamic handlers |
| **Pattern 2** (Interfaces) | Compile-time | ✅ Yes (primitives only) | ❌ No | Semi-agnostic code with primitives |
| **Pattern 3** (Typed methods) | Full compile-time | ❌ No | ✅ Yes | Version-specific client code |

## Implementation Details

### GetFields/SetFields

The `GetFields()` and `SetFields()` methods are **not deprecated**. They are the primary version-agnostic API for working with version-specific types.

```go
// GetFields returns a map of all packet fields for version-agnostic access.
// Use this when you need to access fields dynamically or when working with version-specific types
// that don't have stable cross-version interfaces.
func (p *PlayerInfo) GetFields() map[string]pk.FieldEncoder
```

### Typed Getter/Setter Methods

Every packet generates typed getter/setter methods for all fields:

```go
// GetAction returns the Action field value.
// Note: This method returns the actual field type, which may be version-specific.
// For version-agnostic access, use GetFields() or check for typed interfaces.
func (p *PlayerInfo) GetAction() PlayerInfoActionBitflags
```

These return the **actual concrete type**, which may be version-specific.

### Generic Interfaces

The `models/field_accessors.go` file contains generic interfaces for fields with stable types:

```go
// For fields with a single stable type
type CountGetter interface {
    GetCount() pk.VarInt
}

// For fields with multiple stable types
type EntityIdType interface {
    pk.Int | pk.VarInt
}

type EntityIdGetter[T EntityIdType] interface {
    GetEntityId() T
}
```

These work **only** for fields with primitive types from the `pk` package.

## Why This Is the Right Solution

1. **Acknowledges Reality**: Version-specific types cannot be made version-agnostic. This is a fundamental truth, not a bug.

2. **Provides Choice**: Each pattern has clear trade-offs. Developers can choose based on their needs.

3. **Maintains Functionality**: All fields are accessible—either dynamically (Pattern 1) or with type safety (Pattern 3).

4. **Optimizes Common Cases**: Pattern 2 provides convenience for the common case of primitive-typed fields.

5. **No False Promises**: We don't pretend version-specific types can be used in version-agnostic interfaces.

## Recommended Approach for Client Code

For a version-agnostic Minecraft client:

1. **Detect version at connection time**
2. **Use Pattern 1** (GetFields) for version-specific fields
3. **Use Pattern 2** (interfaces) for common primitive fields (entityId, x, y, z, etc.)
4. **Maintain version-specific handlers** for packets with significant structural differences

Example:
```go
type PacketHandler struct {
    version string
}

func (h *PacketHandler) HandlePacket(pkt models.Packet) {
    // Pattern 2 for stable primitive fields
    if posGetter, ok := pkt.(models.XGetter[pk.Double]); ok {
        x := posGetter.GetX()
        // Use x...
    }
    
    // Pattern 1 for version-specific fields
    fields := pkt.GetFields()
    if action, ok := fields["Action"]; ok {
        h.handleAction(action)
    }
}

func (h *PacketHandler) handleAction(action pk.FieldEncoder) {
    switch h.version {
    case "1.21.1":
        if a, ok := action.(v1211.PlayerInfoActionBitflags); ok {
            // Handle v1.21.1 action...
        }
    case "1.21.6":
        if a, ok := action.(v1216.PlayerInfoActionBitflags); ok {
            // Handle v1.21.6 action...
        }
    }
}
```

This approach balances type safety with version agnosticism.
