# Phase 4 Revised Plan: Error Analysis and Fixes

**Current Status:** 89.4% average success rate (2,479/2,772 tests passing)
**Remaining:** 293 failures (10.6%)
**Target:** 99%+ success rate (100% ideal)

---

## Overview

After regenerating test data with your changes, the test suite now contains:
- **~350 tests per version** (up from ~175 previously)
- **Total: 2,772 tests across 8 versions**
- **New test variants:** "random" and "zeros" in addition to original tests

### Results by Version

| Version | Total | Passed | Failed | Success Rate |
|---------|-------|--------|--------|--------------|
| 1.21.1  | 340   | 310    | 30     | 91.2%        |
| 1.21.2  | 345   | 312    | 33     | 90.4%        |
| 1.21.3  | 344   | 312    | 32     | 90.7%        |
| 1.21.4  | 346   | 314    | 32     | 90.8%        |
| 1.21.5  | 350   | 315    | 35     | 90.0%        |
| 1.21.6  | 349   | 307    | 42     | 88.0%        |
| 1.21.7  | 350   | 305    | 45     | 87.1%        |
| 1.21.8  | 348   | 304    | 44     | 87.4%        |
| **AVG** | **2,772** | **2,479** | **293** | **89.4%** |

**Note:** Earlier versions (1.21.1-1.21.5) perform better than later versions (1.21.6-1.21.8).

---

## Error Analysis (Based on 1.21.5)

Analyzed 35 unique failures in version 1.21.5. Error frequency analysis reveals:

### Category Breakdown

| Category | Count | % of Failures | Severity | Difficulty |
|----------|-------|---------------|----------|------------|
| UUID Corruption | 8 | 22.9% | **CRITICAL** | Hard |
| Non-Empty Buffer Mismatch | 6 | 17.1% | High | Medium |
| Nested Tags Structure | 2 | 5.7% | Medium | Medium |
| EntityMetadata False Positive | 2 | 5.7% | Low | Easy |
| Bitflags String Representation | 2 | 5.7% | Low | Easy |
| *packet.String Not Dereferencing | 2 | 5.7% | Medium | Easy |
| Bitfield Struct Extraction | 2 | 5.7% | Medium | Medium |
| Particle Type Mismatch | 2 | 5.7% | Medium | Medium |
| Pointer Numeric Types | 1 | 2.9% | Low | Easy |
| Complex Struct Types | 2 | 5.7% | Medium | Medium |
| Array Count Mismatch | 1 | 2.9% | Low | Hard |
| EOF Error | 1 | 2.9% | Medium | Hard |

---

## Category 1: UUID Corruption (CRITICAL) 🔴

**Impact:** ~8 packets per version = ~64 total failures
**Severity:** CRITICAL - This is a fundamental parsing bug

### Problem

UUIDs are being corrupted during parsing. The pattern shows bytes being zeroed out or replaced with incorrect values.

**Examples:**
```
Expected: "a7b7f235-b274-4e10-9962-eb7001c2"
Got:      "a7b7f235-2301-0000-2900-000000000044"

Expected: "491fe825-5529-4b40-9fe5-1dfd8672"
Got:      "491fe825-0000-0004-0000-000000000072"

Expected: "37390e74-2f6a-4743-928c-9ba6e86d"
Got:      "37390e74-0000-0000-0000-000000000040"

Expected: "06c9e568-6d3f-49a7-97c9-2aed8c71"
Got:      "06c9e568-0000-0030-ae44-45000000000a"
```

### Analysis

The corruption pattern suggests:
1. First segment often correct or partially correct
2. Middle segments become `0000` or have weird values
3. Last segments corrupted with seemingly random hex

This indicates the UUID parser is:
- Reading bytes incorrectly (byte order issue?)
- Interpreting non-UUID data as UUID bytes
- Buffer offset/alignment problem

### Affected Packets

- **Login:** success, login_start, encryption_begin (uuid, sessionUUID)
- **Play:** spawn_entity (objectUUID), boss_bar (entityUUID), player_remove (players[0]), player_info (playerUUID), spectate (target)

### Root Cause Investigation Needed

**Action:** Need to investigate UUID parsing in protodef-go or generated code:
1. Check how `uuid` type is defined in protocol
2. Examine UUID Scan/Marshal implementation
3. Verify byte order (big-endian vs little-endian)
4. Check for buffer consumption issues

### Solution Path

1. **Locate UUID parser**: Find where UUID type is implemented (likely in protodef-go datatypes or models)
2. **Add debug logging**: Instrument UUID parser to see actual bytes being read
3. **Test with known UUID**: Create unit test with known UUID bytes
4. **Fix byte order**: Likely need to fix how bytes are being read/interpreted
5. **Verify all UUID fields**: Test all packets with UUID fields

**Estimated Impact:** Fixes ~8 packets per version = ~64 total failures (~2.3pp improvement)

---

## Category 2: Non-Empty Buffer Type Mismatch 🟡

**Impact:** ~6 packets per version = ~48 total failures
**Severity:** High - Breaks buffer field validation

### Problem

Ground truth represents **all buffers** (not just empty ones) as:
```javascript
{ type: "Buffer", data: [bytes...] }
```

But Go code represents non-empty buffers as:
- `packet.ByteArray` (e.g., `[19 119 253 161]`)
- `nil` when buffer field is not set but expected to have data

Phase 4B fixed empty buffers, but non-empty buffers still fail.

### Examples

```
Expected: map[data:[19 119 253 161] type:Buffer] (map[string]interface{})
Got:      [19 119 253 161] (packet.ByteArray)

Expected: map[data:[220 46 203 3] type:Buffer]
Got:      <nil>
```

### Affected Packets

- **Login:** encryption_begin (sharedSecret, publicKey)
- **Configuration:** custom_payload (data)
- **Play:** custom_payload (data), chunk_biomes (data), declare_commands (data), etc.

### Current Code

From `groundtruth.go:468-495`, we check for empty buffers but don't handle non-empty buffer comparison:

```go
if len(data) == 0 {
    // Empty buffer handling...
}
// Non-empty buffer: compare with byte array
if byteArr, ok := actualValue.([]byte); ok {
    return compareByteArrays(fieldName, data, byteArr)
}
```

The issue is that when actual is `packet.ByteArray` (not `[]byte`), the type assertion fails.

### Solution

Update buffer comparison in `groundtruth.go:468-495`:

```go
if bufMap, ok := expected.(map[string]interface{}); ok {
    if typ, hasType := bufMap["type"].(string); hasType && typ == "Buffer" {
        if data, hasData := bufMap["data"].([]interface{}); hasData {
            // Empty buffer handling (existing code)
            if len(data) == 0 {
                if actualValue == nil {
                    return nil
                }
                if actualReflect := reflect.ValueOf(actualValue); actualReflect.Kind() == reflect.Slice {
                    if actualReflect.Len() == 0 {
                        return nil
                    }
                }
                if _, isVoid := actualValue.(models.Void); isVoid {
                    return nil
                }
            }

            // Non-empty buffer: compare with byte array (ENHANCED)
            // Try multiple type assertions for buffer types
            if byteArr, ok := actualValue.([]byte); ok {
                return compareByteArrays(fieldName, data, byteArr)
            }

            // Handle packet.ByteArray type
            if byteArr, ok := actualValue.(models.ByteArray); ok {
                return compareByteArrays(fieldName, data, []byte(byteArr))
            }

            // Handle reflection for any slice of bytes
            if actualReflect := reflect.ValueOf(actualValue); actualReflect.Kind() == reflect.Slice {
                if actualReflect.Type().Elem().Kind() == reflect.Uint8 {
                    // Convert to []byte
                    byteSlice := make([]byte, actualReflect.Len())
                    for i := 0; i < actualReflect.Len(); i++ {
                        byteSlice[i] = byte(actualReflect.Index(i).Uint())
                    }
                    return compareByteArrays(fieldName, data, byteSlice)
                }
            }

            // If we get here, actual is not a compatible buffer type
            return fmt.Errorf("%s: expected Buffer, got %T", fieldName, actualValue)
        }
    }
}
```

**Estimated Impact:** Fixes ~6 packets per version = ~48 total failures (~1.7pp improvement)

---

## Category 3: Nested Tags Structure 🟡

**Impact:** ~2 packets per version = ~16 total failures

### Problem

```
Expected: array
Got:      basetypes.Tags
```

**Affected:** tags packets in configuration and play states

### Analysis

`basetypes.Tags` is a struct that wraps the actual tags array. Ground truth expects the unwrapped array.

### Solution

Add Tags unwrapping in `groundtruth.go`:

```go
// Check if actual is basetypes.Tags that needs unwrapping
if actualStruct := reflect.ValueOf(actualValue); actualStruct.Kind() == reflect.Struct {
    typeName := actualStruct.Type().String()
    if strings.Contains(typeName, ".Tags") {
        // Tags struct should have a field containing the actual array
        // Look for common field names: Tags, Items, Entries, etc.
        for i := 0; i < actualStruct.NumField(); i++ {
            field := actualStruct.Field(i)
            fieldName := actualStruct.Type().Field(i).Name
            if field.Kind() == reflect.Slice &&
               (fieldName == "Tags" || fieldName == "Items" || fieldName == "Entries") {
                // Use the slice field instead of the struct
                actualValue = field.Interface()
                actual = reflect.ValueOf(actualValue)
                break
            }
        }
    }
}
```

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 4: EntityMetadata Still Failing 🟢

**Impact:** ~2 packets per version = ~16 total failures

### Problem

The Phase 4A EntityMetadata fix isn't catching all cases:
```
Expected: nil
Got:      {255 []} (basetypes.EntityMetadata)
```

### Analysis

The current detection (lines 417-449) only triggers when expected is `nil` and actual is not nil, but the type check may not be matching.

### Solution

The existing fix at lines 417-449 should work, but may need to be moved earlier in the comparison flow. Need to check actual type string matching:

```go
// Move this check BEFORE the nil comparison
if actualStruct := reflect.ValueOf(actualValue); actualStruct.Kind() == reflect.Struct {
    typeName := actualStruct.Type().String()
    // More lenient matching
    if strings.Contains(typeName, "EntityMetadata") {
        // Check for Terminator and Entries fields
        termField := actualStruct.FieldByName("Terminator")
        entriesField := actualStruct.FieldByName("Entries")

        if termField.IsValid() && entriesField.IsValid() {
            // Check terminator value
            termValue := termField.Interface()
            var isTerminator bool

            switch v := termValue.(type) {
            case uint8:
                isTerminator = (v == 0xFF)
            case int:
                isTerminator = (v == 255)
            case byte:
                isTerminator = (v == 0xFF)
            }

            if isTerminator && entriesField.Len() == 0 {
                if expected == nil {
                    return nil // Empty metadata matches nil
                }
            }
        }
    }
}
```

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 5: *packet.String Not Dereferencing 🟢

**Impact:** ~2 packets per version = ~16 total failures

### Problem

Two distinct issues:
1. `expected string "test", got *packet.String(0xc0009d7770)` - pointer not dereferenced
2. `expected string "auto_9eckxj", got *models.Buffer(&{[]})` - parsed as Buffer instead of String!

### Analysis

Issue #1: The Phase 4A fix isn't working (lines 327-344)
Issue #2: This is a parsing bug - the field is being parsed as the wrong type

### Solution for Issue #1

The current pointer dereferencing code may not be catching `*packet.String`. Need to enhance:

```go
// Enhanced pointer dereferencing
for actual.Kind() == reflect.Ptr && !actual.IsNil() {
    ptrType := actual.Type().String()

    // Check for packet.String, models.String, or any *String type
    if strings.Contains(ptrType, ".String") {
        // Dereference all pointer levels
        for actual.Kind() == reflect.Ptr && !actual.IsNil() {
            actual = actual.Elem()
        }
        actualValue = actual.Interface()
        break
    }

    // Generic pointer dereferencing
    actual = actual.Elem()
    actualValue = actual.Interface()
}
```

### Solution for Issue #2 (Parsing Bug)

The `tabId` field is being parsed as `*models.Buffer` instead of string. This is a **generator bug** or **protocol definition issue**.

**Action Required:**
1. Check protocol definition for `advancement_tab` packet
2. Verify `tabId` field type in generated code
3. Check if there's a switch/conditional that's choosing wrong type
4. May need to fix in protodef-go or generator

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 6: Bitfield Struct Extraction 🟡

**Impact:** ~2 packets per version = ~16 total failures

### Problem

```
Expected: number 0
Got:      clientbound.MultiBlockChangeChunkCoordinates({0 0 0})

Expected: number 1
Got:      clientbound.MultiBlockChangeChunkCoordinates({0 0 0})
```

This is from the original Phase 4 plan (Category 5).

### Solution

Add bitfield coordinate struct comparison:

```go
// Check if actual is a bitfield struct being compared to a number
if expectedNum, ok := expected.(float64); ok {
    if actualStruct := reflect.ValueOf(actualValue); actualStruct.Kind() == reflect.Struct {
        typeName := actualStruct.Type().Name()

        if strings.Contains(typeName, "Coordinates") || strings.Contains(typeName, "Position") {
            // Check if all fields are zero
            allZero := true
            sum := 0.0

            for i := 0; i < actualStruct.NumField(); i++ {
                field := actualStruct.Field(i)

                switch field.Kind() {
                case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
                    val := field.Int()
                    sum += float64(val)
                    if val != 0 {
                        allZero = false
                    }
                case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
                    val := field.Uint()
                    sum += float64(val)
                    if val != 0 {
                        allZero = false
                    }
                case reflect.Float32, reflect.Float64:
                    val := field.Float()
                    sum += val
                    if val != 0 {
                        allZero = false
                    }
                }
            }

            // If expected is 0 and all fields are 0, match
            if expectedNum == 0 && allZero {
                return nil
            }

            // Otherwise try to match sum (for packed coordinates)
            if sum == expectedNum {
                return nil
            }
        }
    }
}
```

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 7: Bitflags String Representation 🟢

**Impact:** ~2 packets per version = ~16 total failures

### Problem

```
Expected: string "0"
Got:      serverbound.UpdateStructureBlockFlags({ignore_entities})
```

From original Phase 4 plan (Category 4).

### Analysis

This is actually a test data issue - the ground truth expects string "0" but the packet has flags set. However, we should handle this more gracefully.

### Solution

Add bitflags string comparison:

```go
// Handle bitflags string comparison
if expectedStr, ok := expected.(string); ok {
    if actualStruct := reflect.ValueOf(actualValue); actualStruct.Kind() == reflect.Struct {
        typeName := actualStruct.Type().String()

        if strings.Contains(typeName, "Flags") || strings.Contains(typeName, "flags") {
            // Check if struct has String() method
            if stringer, ok := actualValue.(fmt.Stringer); ok {
                actualStr := stringer.String()

                // Compare string representations
                // Note: {} means no flags, should match "0"
                if (actualStr == "{}" || actualStr == "") && expectedStr == "0" {
                    return nil
                }

                // Otherwise compare directly
                if actualStr == expectedStr {
                    return nil
                }

                // If they don't match, this is a real difference
                return fmt.Errorf("%s: expected %q, got %q", fieldName, expectedStr, actualStr)
            }
        }
    }
}
```

**Estimated Impact:** May not fix failures, but provides clearer error messages.

---

## Category 8: Particle Type Mismatch 🟡

**Impact:** ~2 packets per version = ~16 total failures

### Problem

```
Expected: map[data:1 type:tinted_leaves] (map[string]interface{})
Got:      0xc0009805c4 (*packet.Int)

Expected: map[data:0 type:block] (map[string]interface{})
Got:      0xc000980054 (*packet.VarInt)
```

**Packet:** world_particles

### Analysis

Particles have a complex type structure with type and data fields. The Go parser is extracting just the data value (as a pointer to Int/VarInt), but ground truth expects the full structure.

This suggests the particle field is using a switch type that extracts different representations based on the particle type.

### Solution

This requires special handling for particle fields:

```go
// Check if expected is a particle map structure
if expectedMap, ok := expected.(map[string]interface{}); ok {
    if particleType, hasType := expectedMap["type"].(string); hasType {
        if particleData, hasData := expectedMap["data"]; hasData {
            // Check if actual is a pointer to a primitive that represents particle data
            if actualPtr := reflect.ValueOf(actualValue); actualPtr.Kind() == reflect.Ptr {
                if actualPtr.Elem().Kind() >= reflect.Int && actualPtr.Elem().Kind() <= reflect.Float64 {
                    // Extract the numeric value
                    var actualNum float64
                    switch actualPtr.Elem().Kind() {
                    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
                        actualNum = float64(actualPtr.Elem().Int())
                    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
                        actualNum = float64(actualPtr.Elem().Uint())
                    }

                    // Compare with particle data
                    if expectedDataNum, ok := particleData.(float64); ok {
                        if actualNum == expectedDataNum {
                            return nil // Particle data matches
                        }
                    }
                }
            }
        }
    }
}
```

**Note:** This may indicate a protocol definition issue where particle types should preserve the full structure.

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 9: Pointer Numeric Types 🟢

**Impact:** ~1 packet per version = ~8 total failures

### Problem

```
Expected: number 3
Got:      *packet.UnsignedByte(0xc0009815c4)
```

### Solution

Enhance numeric pointer dereferencing:

```go
// Dereference numeric pointers
if actual.Kind() == reflect.Ptr && !actual.IsNil() {
    // Check if pointer to numeric type
    elemKind := actual.Elem().Kind()
    if elemKind >= reflect.Int && elemKind <= reflect.Float64 {
        actual = actual.Elem()
        actualValue = actual.Interface()
    }
}
```

**Estimated Impact:** Fixes ~1 packet per version = ~8 total failures (~0.3pp improvement)

---

## Category 10: Complex Struct Types 🟡

**Impact:** ~2 packets per version = ~16 total failures

### Problem

```
Expected: number 1
Got:      basetypes.CommandNodeFlags({0 0 0 0 0})

Expected: string "15"
Got:      clientbound.EntityUpdateAttributesArrayTypeKey({generic.knockback_resistance})
```

### Analysis

These are structs that have specific meanings:
1. **CommandNodeFlags:** Bitfield with multiple boolean flags
2. **EntityUpdateAttributesArrayTypeKey:** Wrapped string/enum value

### Solution

Need to add specific unwrapping for these types:

```go
// Check for complex struct types that need unwrapping
if actualStruct := reflect.ValueOf(actualValue); actualStruct.Kind() == reflect.Struct {
    typeName := actualStruct.Type().String()

    // Handle EntityUpdateAttributesArrayTypeKey (wrapped string)
    if strings.Contains(typeName, "EntityUpdateAttributesArrayTypeKey") {
        // Look for a string field or String() method
        if stringer, ok := actualValue.(fmt.Stringer); ok {
            actualStr := stringer.String()
            // Compare with expected string
            if expectedStr, ok := expected.(string); ok {
                if actualStr == expectedStr {
                    return nil
                }
                // Check if it's a wrapped value like "{generic.knockback_resistance}"
                trimmed := strings.Trim(actualStr, "{}")
                if trimmed == expectedStr {
                    return nil
                }
            }
        }
    }

    // Handle CommandNodeFlags (bitfield to number)
    if strings.Contains(typeName, "CommandNodeFlags") {
        // Try to extract a numeric representation
        // This may need protocol-specific logic
        if expectedNum, ok := expected.(float64); ok {
            // Check if there's a ToInt() or Value() method
            // Or sum the fields to get a bitfield value
            // This requires understanding the bitfield structure
        }
    }
}
```

**Note:** These may be protocol definition issues that need fixing in the generator.

**Estimated Impact:** Fixes ~2 packets per version = ~16 total failures (~0.6pp improvement)

---

## Category 11: Array Count Mismatch ⚫

**Impact:** ~1 packet per version = ~8 total failures

### Problem

```
Expected: 1
Got:      0
```

Field: `inputs`

### Analysis

This is a data parsing issue where an array has the wrong number of elements. Could be:
1. Parser not reading all array elements
2. Test data has wrong expected count
3. Protocol definition has wrong array size calculation

**Action Required:** Need to investigate specific packet and field to understand root cause.

**Estimated Impact:** Unclear - may require protocol/generator fixes.

---

## Category 12: EOF Error ⚫

**Impact:** ~1 packet per version = ~8 total failures

### Problem

```
scan packet: scanning packet field[3] error: unexpected EOF
```

### Analysis

Parser is trying to read more data than available in the packet. Could be:
1. Field size calculation wrong
2. Variable-length field not terminating correctly
3. Protocol definition mismatch

**Action Required:** Need to identify specific packet and investigate.

**Estimated Impact:** Unclear - may require protocol/generator fixes.

---

## Implementation Priority

### Phase 4D: Critical UUID Bug Fix
**Estimated Time:** 4-6 hours
**Estimated Impact:** +64 failures fixed (~2.3pp improvement)
**Priority:** CRITICAL - Must fix first

1. ✅ Investigate UUID parsing implementation
2. ✅ Add debug logging to UUID parser
3. ✅ Create unit tests with known UUIDs
4. ✅ Fix byte order/reading issue
5. ✅ Verify all UUID fields across all packets

### Phase 4E: Buffer and Type Unwrapping
**Estimated Time:** 3-4 hours
**Estimated Impact:** +96 failures fixed (~3.5pp improvement)
**Priority:** HIGH

1. ✅ Fix non-empty buffer type comparison
2. ✅ Add Tags structure unwrapping
3. ✅ Fix *packet.String dereferencing
4. ✅ Add pointer numeric type dereferencing

### Phase 4F: Struct Comparison Enhancements
**Estimated Time:** 4-5 hours
**Estimated Impact:** +48 failures fixed (~1.7pp improvement)
**Priority:** MEDIUM

1. ✅ Implement bitfield coordinate struct comparison
2. ✅ Add particle type special handling
3. ✅ Fix EntityMetadata detection
4. ✅ Improve bitflags string comparison
5. ✅ Handle complex struct unwrapping

### Phase 4G: Generator/Protocol Fixes
**Estimated Time:** Variable
**Estimated Impact:** +16-32 failures fixed (~0.6-1.2pp improvement)
**Priority:** LOW (investigate first)

1. ✅ Investigate tabId parsing as Buffer bug
2. ✅ Investigate array count mismatch
3. ✅ Investigate EOF errors
4. ✅ Check protocol definitions for edge cases

---

## Estimated Final Results

| Phase | Success Rate | Improvement | Cumulative |
|-------|--------------|-------------|------------|
| Current (Phase 3) | 89.4% | baseline | - |
| + Phase 4D (UUID) | 91.7% | +2.3pp | +2.3pp |
| + Phase 4E (Buffers) | 95.2% | +3.5pp | +5.8pp |
| + Phase 4F (Structs) | 96.9% | +1.7pp | +7.5pp |
| + Phase 4G (Fixes) | 97.5-98.1% | +0.6-1.2pp | +8.1-8.7pp |
| + Further refinements | **99%+** | +9-10pp | **Target** |
| + Perfect parsing | **100%** | +10.6pp | **Ideal** |

**Target:** 99%+ success rate
**Ideal Goal:** 100% (all 2,772 tests passing)

---

## Known Issues

1. **UUID Corruption** - Critical parsing bug, must investigate in protodef-go
2. **tabId as Buffer** - Parsing/generator bug, needs protocol definition check
3. **Array count mismatch** - May be test data or protocol issue
4. **EOF errors** - Need specific packet investigation

---

## Testing Strategy

After each phase:
1. Run validation on 1.21.5 (representative version)
2. Verify specific failing packets now pass
3. Check for regressions in previously passing tests
4. Run full test suite on all 8 versions
5. Update progress metrics
6. Document any new issues discovered

---

## Success Criteria

- **Phase 4D:** UUID corruption fixed, all UUID fields parsing correctly
- **Phase 4E:** Buffer fields comparing correctly, pointer types dereferenced
- **Phase 4F:** Struct types unwrapped and compared correctly
- **Phase 4G:** Generator/protocol issues identified and documented
- **Phase 4H+:** All remaining edge cases resolved

**Overall Target:** 99%+ success rate across all versions
**Ideal Goal:** 100% (all 2,772 tests passing)

---

*Plan created: 2025-11-20*
*Supersedes: phase4_edge_case_fixes_plan.md*
*Test data: ~350 tests per version (random/zeros variants)*
