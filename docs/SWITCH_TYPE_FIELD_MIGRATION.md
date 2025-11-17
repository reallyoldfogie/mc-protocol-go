# Switch Type Migration: `any` → `pk.Field`

## Overview

Currently, switch datatypes in the protocol are being generated with type `any`, which prevents proper serialization/deserialization through the standard `pk.Field` interface. This document outlines the plan to change switch fields from `any` to `pk.Field` while maintaining backward compatibility and correct code generation.

## Executive Summary

### The Core Issue

Switch fields are currently typed as `any`, which causes templates to **skip** them during serialization. This means:
- ❌ Switches cannot be serialized through normal `pk.Marshal`/`pk.Scan` 
- ❌ Switch fields are excluded from ReadFrom/WriteTo standard field iteration
- ❌ Custom inline switch handling must do ALL the work

### The Solution

Change switch field type from `any` to `pk.Field` AND remove the skip checks in templates. This enables:
- ✅ Switches can use standard pk.Field interface methods
- ✅ Type safety through interface constraints
- ✅ Consistent behavior with other field types

### Critical Changes Required

1. **Change type declaration**: `field.Type.TypeName = "any"` → `"pk.Field"` (3 locations)
2. **Remove skip checks**: Delete `(ne .Type.TypeName "any")` conditionals from templates (5 locations)
3. **Update test expectations**: Change assertions from `"any"` to `"pk.Field"` (9 locations)
4. **Decision needed**: Whether to include switches in Marshal/Scan or keep them excluded

### Key Insight

The templates already use `{{if isSwitch .}}` to handle switches separately in ReadFrom/WriteTo, so there's no risk of double-processing. The only question is whether Marshal/Scan should also handle switches or continue to skip them.

## Background

### Current Implementation

As documented in `SWITCH_IMPLEMENTATION.md`, switches are conditional types that select different data types based on a comparison value. Currently:

```go
type CommonServerLinksArrayType struct {
    HasKnownType pk.Boolean
    KnownType    any        // ❌ Currently using any
    UnknownType  any        // ❌ Currently using any
    Link         pk.String
}
```

### Desired Implementation

```go
type CommonServerLinksArrayType struct {
    HasKnownType pk.Boolean
    KnownType    pk.Field   // ✅ Should use pk.Field
    UnknownType  pk.Field   // ✅ Should use pk.Field
    Link         pk.String
}
```

### Why This Change?

1. **Type Safety**: `pk.Field` is an interface that ensures types implement both `io.ReaderFrom` and `io.WriterTo`
2. **Consistency**: All other protocol fields use concrete types that implement `pk.Field`
3. **Serialization**: Allows switch fields to be properly serialized/deserialized like any other field
4. **Interoperability**: Makes switch fields compatible with the rest of the pk ecosystem

## Problem Analysis

### The Fundamental Issue

Currently, switch fields use type `any`, which causes them to be **skipped** in all serialization paths:

```go
// Current behavior with "any"
type Example struct {
    SomeFlag pk.Boolean
    Data     any  // ← Skipped by Marshal, Scan, ReadFrom, WriteTo templates!
}

func (p *Example) Marshal() (pk.Packet, error) {
    // Data field is NOT included because TypeName == "any"
    return pk.Marshal(p.packetID, &p.SomeFlag), nil
}
```

This means:
- ❌ Switch fields are not serialized through normal channels
- ❌ Custom inline switch handling code is generated separately
- ❌ Switch fields cannot be used with standard pk.Field interfaces

### The Solution

Changing to `pk.Field` makes switch fields **participate** in standard serialization:

```go
// New behavior with "pk.Field"
type Example struct {
    SomeFlag pk.Boolean
    Data     pk.Field  // ← Processed by all serialization methods!
}

func (p *Example) Marshal() (pk.Packet, error) {
    // Data field IS included because it implements pk.Field
    return pk.Marshal(p.packetID, &p.SomeFlag, &p.Data), nil
}
```

This enables:
- ✅ Switch fields work like any other field
- ✅ Standard pk.Field interface methods handle serialization
- ✅ Consistent behavior across all field types

### Location of Type Assignment

The switch field type is set to `any` in `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`:

**Line 2056:**
```go
case "switch":
    // Switch types are handled inline - use any (interface{}) for the field type
    // Keep Extras with switch metadata for inline generation in container methods
    field.Type.TypeName = "any"  // ← This is where the type is set
```

### Current Template Handling

The templates currently **skip** `any` fields because they cannot be serialized:
1. `ReadFrom` methods skip fields with `TypeName == "any"`
2. `WriteTo` methods skip fields with `TypeName == "any"`
3. `Marshal`/`Scan` methods skip fields with `TypeName == "any"`

However, switches have special inline handling:
1. Switch statements in `ReadFrom` methods instantiate concrete types based on compareTo values
2. Switch WriteTo code type-asserts to `io.WriterTo` interface
3. Void cases (struct{}) are handled specially

**Example from templates (line 750, 781):**
```go
// ReadFrom currently skips any fields
{{if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "[]byte") ... (ne .Type.TypeName "any")}}

// WriteTo currently skips any fields  
{{if and (ne .Type.TypeName "struct{}") ... (ne .Type.TypeName "any")}}
```

**⚠️ IMPORTANT**: Once we change to `pk.Field`, these fields should **NOT** be skipped because `pk.Field` is a proper interface that supports serialization!

## Implementation Plan

### Phase 1: Code Generation Changes

**File**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`

#### Step 1.1: Change Field Type Assignment
- **Location**: Line 2056
- **Change**: Replace `field.Type.TypeName = "any"` with `field.Type.TypeName = "pk.Field"`
- **Impact**: All newly generated switch fields will use `pk.Field` instead of `any`

```go
case "switch":
    // Switch types are handled inline - use pk.Field for the field type
    // Keep Extras with switch metadata for inline generation in container methods
    field.Type.TypeName = "pk.Field"  // ✅ Changed from "any"
    // Keep field.Type.Extras for template to access switch info
```

#### Step 1.2: Update Nested Switch Handling
- **Location**: Lines 2070-2071
- **Change**: Update nested switch type assignments to use `pk.Field`

```go
if caseType.Extras != nil && caseTypeLower == "switch" {
    // Nested switch - keep as pk.Field but preserve Extras
    caseType.Name = "pk.Field"       // ✅ Changed from "any"
    caseType.TypeName = "pk.Field"   // ✅ Changed from "any"
```

#### Step 1.3: Update Switch Type Check
- **Location**: Lines 2167-2170
- **Change**: Update the switch type keyword handling

```go
if strings.ToLower(lookupKey) == "switch" {
    // Switch types are inline and use pk.Field
    caseType.Name = "pk.Field"       // ✅ Changed from "any"
    caseType.TypeName = "pk.Field"   // ✅ Changed from "any"
    continue
}
```

### Phase 2: Template Updates

**File**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`

#### Step 2.1: Remove Skip Logic for Switch Fields

**⚠️ CRITICAL CHANGE**: The templates currently skip `any` fields because they can't be serialized. After changing to `pk.Field`, we need to **REMOVE** these skip checks so switch fields are properly processed.

**Line 719 (Marshal method) - OPTION 1: Remove skip check to include switches:**
```go
// Before (skips any fields):
return pk.Marshal(p.packetID{{range $container.Fields}}{{if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "any")}}, &p.{{.Name}}{{end}}{{end}}), nil

// After (processes pk.Field fields):
return pk.Marshal(p.packetID{{range $container.Fields}}{{if ne .Type.TypeName "struct{}"}}, &p.{{.Name}}{{end}}{{end}}), nil
```

**⚠️ IMPORTANT**: This will attempt to serialize switch fields through pk.Marshal, which requires pk.Field interface. This should work IF the switch field has been populated by ReadFrom.

**Alternative OPTION 2: Keep switches excluded from Marshal/Scan (safer):**
```go
// Keep switches out of Marshal/Scan since they're handled by ReadFrom/WriteTo
return pk.Marshal(p.packetID{{range $container.Fields}}{{if and (ne .Type.TypeName "struct{}") (not (isSwitch .))}}, &p.{{.Name}}{{end}}{{end}}), nil
```

**Line 727 (Scan method) - OPTION 1: Remove skip check:**
```go
// Before (skips any fields):
return packet.Scan({{range $container.Fields}}{{if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "any")}}&p.{{.Name}}, {{end}}{{end}})

// After (processes pk.Field fields):
return packet.Scan({{range $container.Fields}}{{if ne .Type.TypeName "struct{}"}}&p.{{.Name}}, {{end}}{{end}})
```

**Alternative OPTION 2: Keep switches excluded (safer):**
```go
return packet.Scan({{range $container.Fields}}{{if and (ne .Type.TypeName "struct{}") (not (isSwitch .))}}&p.{{.Name}}, {{end}}{{end}})
```

**💡 RECOMMENDATION**: Use OPTION 2 (keep switches excluded from Marshal/Scan) initially. Since ReadFrom/WriteTo already have inline switch handling via `{{if isSwitch .}}`, Marshal/Scan don't need to handle switches. Test both options to see which works better.

**Line 750 (ReadFrom method) - REMOVE the skip check:**
```go
// Before (skips any fields):
{{if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "[]byte") (ne .Type.TypeName "basetypes.Tags") (ne .Type.TypeName "Tags") (ne .Type.TypeName "any")}}

// After (processes pk.Field fields):
{{if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "[]byte") (ne .Type.TypeName "basetypes.Tags") (ne .Type.TypeName "Tags")}}
```

**Line 781 (WriteTo method) - REMOVE the skip check:**
```go
// Before (skips any fields):
{{else if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "[]byte") (ne .Type.TypeName "basetypes.Tags") (ne .Type.TypeName "Tags") (ne .Type.TypeName "any")}}

// After (processes pk.Field fields):
{{else if and (ne .Type.TypeName "struct{}") (ne .Type.TypeName "[]byte") (ne .Type.TypeName "basetypes.Tags") (ne .Type.TypeName "Tags")}}
```

**Lines 821, 842 (Nested switch templates) - REMOVE the skip check:**
```go
// Before (skips any fields):
{{if and (ne $nestedType.TypeName "struct{}") (ne $nestedType.TypeName "[]byte") (ne $nestedType.TypeName "any")}}
{{if and (ne $type.TypeName "struct{}") (ne $type.TypeName "[]byte") (ne $type.TypeName "any")}}

// After (processes pk.Field fields):
{{if and (ne $nestedType.TypeName "struct{}") (ne $nestedType.TypeName "[]byte")}}
{{if and (ne $type.TypeName "struct{}") (ne $type.TypeName "[]byte")}}
```

**Why this matters**: 
- Currently: Templates skip `any` fields → switch fields are NOT included in Marshal/Scan/ReadFrom/WriteTo
- After change: Templates process `pk.Field` fields → switch fields ARE included in all serialization methods
- This enables proper serialization/deserialization of switch fields through the standard pk.Field interface

#### Step 2.2: Update Switch Template Function
**Location**: Line 1233

```go
// Before:
func isTemplateSwitch(field *datatypes.ContainerField) bool {
    return field.Type != nil && field.Type.Extras != nil && field.Type.TypeName == "any"
}

// After:
func isTemplateSwitch(field *datatypes.ContainerField) bool {
    return field.Type != nil && field.Type.Extras != nil && field.Type.TypeName == "pk.Field"
}
```

### Phase 3: Test File Updates

**File**: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets_test.go`

Update test expectations to use `pk.Field` instead of `any`:
- Lines 422, 439, 496, 704, 746, 760, 771, 788, 819

Example:
```go
// Before:
if field.Type.TypeName != "any" {
    t.Errorf("expected switch field type to be 'any', got %s", field.Type.TypeName)
}

// After:
if field.Type.TypeName != "pk.Field" {
    t.Errorf("expected switch field type to be 'pk.Field', got %s", field.Type.TypeName)
}
```

### Phase 4: Regenerate Protocol Code

After making all code changes:

1. **Backup existing generated code** (optional but recommended)
   ```bash
   cp -r data/1.21.5 data/1.21.5.backup
   ```

2. **Run the generator** for all versions
   ```bash
   # Run generator with your configuration
   go run cmd/generator/main.go --config configs/generator.yaml
   ```

3. **Verify generated code** - Check that:
   - Switch fields now use `pk.Field` instead of `any`
   - ReadFrom/WriteTo methods are generated correctly
   - No compilation errors occur

### Phase 5: Verification & Testing

#### Step 5.1: Compile Generated Code
```bash
cd data/1.21.5
go build ./...
```

#### Step 5.2: Run Existing Tests
```bash
go test ./data/1.21.5/...
```

#### Step 5.3: Run Generator Tests
```bash
go test ./internal/generator/...
```

#### Step 5.4: Integration Testing
Test specific switch-heavy structures:
- `SlotComponentData` (complex mapper + switch combination)
- `EntityMetadataEntryValue` (pk.Field cases)
- `ParticleData` (particle type discrimination)
- `CommonServerLinksArrayType` (boolean-based switch)

## Risk Analysis

### Critical Behavioral Changes ⚠️

1. **Switch fields will now be serialized through standard paths**
   - **Current behavior**: Switch fields (type `any`) are skipped in Marshal/Scan/ReadFrom/WriteTo
   - **New behavior**: Switch fields (type `pk.Field`) will be included in all serialization methods
   - **Impact**: This fundamentally changes how switch fields are processed
   - **Verification Required**:
     - Test that switch ReadFrom/WriteTo still work correctly
     - Verify inline switch logic (with Extras metadata) still generates properly
     - Ensure no double-serialization occurs (inline switch code + standard serialization)

2. **Template skip logic must be removed**
   - **Critical**: If we change to `pk.Field` but keep the skip checks, switches won't work
   - **Must remove**: All `(ne .Type.TypeName "any")` conditionals from templates
   - **Verification**: Grep for `"any"` in template strings to ensure none are missed

### Medium Risk Changes ⚠️

1. **Inline switch handling coordination**
   - **Issue**: Switches currently have TWO code paths:
     - Inline switch statements (via `{{template "switchReadTmpl"}}` and WriteTo special handling)
     - Standard field serialization (currently skipped for `any`)
   - **Risk**: After change, both paths will be active
   - **Mitigation**: Review generated code to ensure inline switch logic and standard serialization don't conflict
   - **Expected behavior**:
     - **ReadFrom**: Inline switch logic reads data and assigns to field (e.g., `t.Data = val`)
     - **ReadFrom standard path**: SKIPPED via `{{if isSwitch .}}...{{else if ...}}` check (line 750)
     - **WriteTo**: Inline switch logic writes the field value using type assertion
     - **WriteTo standard path**: SKIPPED via `{{if isSwitch .}}...{{else if ...}}` check (line 766)
   - **✅ GOOD NEWS**: The template already uses `{{if isSwitch .}}` to handle switches separately!
   - **No conflict**: Inline switch logic and standard iteration are mutually exclusive in the template

2. **Type assertion compatibility**: Code that type-asserts switch fields
   - **Current**: `t.Data.(interface{ WriteTo(io.Writer) (int64, error) })`
   - **New**: This will still work since `pk.Field` requires WriteTo
   - **Verification**: Review all switch WriteTo template code (lines 766-780)

### Low Risk Changes ✅

1. **Type declaration change**: `any` → `pk.Field`
   - **Why low risk**: Both are interfaces
   - **Note**: All switch case types already implement `io.ReaderFrom` and `io.WriterTo`

2. **Import statements**: Generated code may need `pk` import if not already present
   - **Why low risk**: The baseTypes template already imports pk
   - **Verification**: Check generated imports

3. **Void cases**: `struct{}` handling with `pk.Field`
   - **Why low risk**: Templates already handle void cases specially
   - **Note**: `struct{}` is still excluded by conditional checks

## Rollback Plan

If issues arise after implementation:

1. **Revert code changes**: Use git to revert commits
   ```bash
   git revert <commit-hash>
   ```

2. **Restore backup**: If generated code was backed up
   ```bash
   rm -rf data/1.21.5
   mv data/1.21.5.backup data/1.21.5
   ```

3. **Regenerate with reverted code**
   ```bash
   go run cmd/generator/main.go --config configs/generator.yaml
   ```

## Success Criteria

- [ ] All switch fields in generated code use `pk.Field` instead of `any`
- [ ] Generated code compiles without errors
- [ ] All existing tests pass
- [ ] Switch ReadFrom/WriteTo methods work correctly
- [ ] No performance regression in serialization/deserialization
- [ ] Documentation updated to reflect the change

## Timeline Estimate

- **Phase 1**: 1-2 hours (code changes)
- **Phase 2**: 1-2 hours (template updates)
- **Phase 3**: 30 minutes (test updates)
- **Phase 4**: 30 minutes (regeneration)
- **Phase 5**: 2-3 hours (testing and verification)

**Total**: ~6-8 hours

## Notes

- The `pk.Field` interface from go-mc is simply a marker interface for types that implement both `io.ReaderFrom` and `io.WriterTo`
- All switch case types already implement these interfaces, so the migration should be seamless
- The main benefit is improved type safety and better integration with the pk ecosystem
- This change aligns with the existing codebase pattern of using concrete types that implement `pk.Field`

## References

- Original implementation: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/docs/SWITCH_IMPLEMENTATION.md`
- Main generator: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/generator.go`
- Switch generation logic: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`
- Generated examples: `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/basetypes/types.go`
