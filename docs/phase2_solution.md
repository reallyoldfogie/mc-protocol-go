# Phase 2 Solution Documentation

## Problem Statement

After implementing the Phase 1 fix to create Type objects for custom type references in `protodef-go/datatypes/type.go`, we encountered a new issue: references to custom types were not being resolved to their actual definitions, causing "undefined type" errors.

## Root Cause Analysis

There were TWO interconnected issues:

### Issue 1: Timing - baseTypes Map Population Order

The `baseTypes` map was being populated in the same loop that called `processType()`. This meant:
1. Types were processed in the order they appeared in protocol.json
2. When `SlotComponent` was processed, `SlotComponentType` hadn't been added to `baseTypes` yet
3. The field reference couldn't be resolved

### Issue 2: Lookup Key - Wrong Field Used

In `gen_packet.go`, the code was looking up `field.Type.Name` in the `baseTypes` map, but:
- `field.Type.Name` = "type" (the field name)
- `field.Type.TypeName` = "SlotComponentType" (the actual type reference)

The lookup should use `TypeName`, not `Name`.

## Solution

### Part 1: Fix GetType in protodef-go (from Phase 1)

**File:** `/home/reallyoldfogie/src/github.com/reallyoldfogie/protodef-go/datatypes/type.go`

```go
func GetType(name string, d gjson.Result) *Type {
    var t *Type
    if d.Type == gjson.String {
        t = GetNativeType(d.String())
        if t != nil {
            return t
        }
        // NEW: Handle custom type references
        return &Type{
            Name:     d.String(),
            TypeName: d.String(),
        }
    }
    // ... rest of function
}
```

This ensures that string type references like `"SlotComponentType"` create a Type object instead of returning nil.

### Part 2: Two-Pass baseTypes Map Build

**File:** `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/gen_packet.go`

Changed from single-pass:
```go
baseTypes := map[string]string{}
types := make([]*datatypes.Type, 0)
for _, t := range protocolDefinitions.Types {
    // Process and add to baseTypes in same loop
    newTypes := processType(t, map[string]string{}, false, true)
    types = append(types, newTypes...)
    baseTypes[rawName] = t.Name
}
```

To two-pass:
```go
// FIRST PASS: Build complete baseTypes map
baseTypes := map[string]string{}
for _, t := range protocolDefinitions.Types {
    switch t.Name {
    case "array", "switch", "container", "option", "bitfield":
        continue
    default:
        baseTypes[rawName] = t.Name
    }
}

// SECOND PASS: Process types with complete baseTypes map
types := make([]*datatypes.Type, 0)
for _, t := range protocolDefinitions.Types {
    switch t.Name {
    case "array", "switch", "container", "option", "bitfield":
        continue
    default:
        newTypes := processType(t, baseTypes, false, true)
        types = append(types, newTypes...)
    }
}
```

### Part 3: Fix Lookup Key

**File:** `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/gen_packet.go`

Changed from:
```go
if fieldName, ok := baseTypes[field.Type.Name]; ok {
```

To:
```go
// Use TypeName (the actual type reference) instead of Name (the field name)
lookupKey := field.Type.TypeName
if lookupKey == "" {
    lookupKey = field.Type.Name
}
if fieldName, ok := baseTypes[lookupKey]; ok {
```

## Results

After implementing these fixes:

```
DEBUG [gen_packet.go]: Field 'SlotComponent.Type' type 'type' (typename='SlotComponentType') FOUND in baseTypes as 'SlotComponentType'
DEBUG [gen_packet.go]: Field 'UntrustedSlotComponent.Type' type 'type' (typename='SlotComponentType') FOUND in baseTypes as 'SlotComponentType'
```

The custom type references are now:
1. Created as Type objects (Phase 1 fix)
2. Looked up correctly using TypeName (Part 3)
3. Found in baseTypes map (Part 2)
4. Resolved to `basetypes.SlotComponentType`

## Next Steps

1. Remove debug logging once generation is verified to work correctly
2. Run full test suite to ensure no regressions
3. Verify generated code compiles and runs correctly
4. Move to Phase 3 (template fixes for unreachable code warnings)
