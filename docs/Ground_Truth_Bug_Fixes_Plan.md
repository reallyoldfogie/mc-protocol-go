# Ground Truth Bug Fixes - Implementation Plan

This document provides a detailed plan for fixing the 4 bugs discovered through ground truth validation against node-minecraft-protocol.

## Overview

**Bugs Discovered**: 4
**Priority**: High (these are real parsing errors, not type differences)
**Estimated Effort**: 1-2 days

## Bug 1: Entity Metadata - Unknown Terminator (127/0x7F)

### Description
**Error**: `unknown EntityMetadataEntryType key: 127`
**Packet**: entity_metadata (ID=92)
**Severity**: High - Prevents parsing of entity metadata packets

### Root Cause Analysis
Entity metadata uses a special terminator value (0xFF/255 or 0x7F/127) to mark the end of the metadata list. Our parser doesn't recognize this terminator.

**Expected Behavior**:
- Metadata is a list of entries
- Each entry: `index (byte) | type (VarInt) | value (varies by type)`
- List ends with terminator: `0xFF` (255) or `0x7F` (127)

**Actual Behavior**:
- Parser tries to process terminator as a metadata type
- Fails with "unknown EntityMetadataEntryType key: 127"

### Investigation Steps
1. Read `/data/1.21.5/play/clientbound/types.go` - find `ClientboundEntityMetadata` struct
2. Search for `EntityMetadataEntryType` definition
3. Check how metadata array is parsed
4. Compare with Minecraft wiki specification for entity metadata format

### Fix Plan
1. **Locate the metadata parsing code**
   - File: `data/1.21.5/play/clientbound/types.go`
   - Struct: `ClientboundEntityMetadata`
   - Field: `Metadata` array

2. **Check generator template**
   - File: `internal/generator/packets.go`
   - Look for entity metadata specific handling
   - Check if terminator is defined in protocol JSON

3. **Implement fix** (one of these approaches):

   **Option A**: Add terminator to protocol definition
   - Update protocol JSON to include 0xFF as valid metadata type
   - Mark it as special "end of list" type
   - Regenerate code

   **Option B**: Custom parsing in ReadFrom
   - Override ReadFrom to handle terminator specially
   - Read entries until terminator (0xFF) is found
   - Don't try to parse terminator as entry

   **Option C**: Fix in generator template
   - Update template to generate terminator-aware array parsing
   - Check for terminator value before reading type/value

4. **Verification**
   - Run ground truth validation on entity_metadata packet
   - Check packet-validation.json for ClientboundEntityMetadata errors
   - Should reduce from current failures to 0

### Files to Modify
- `data/1.21.5/basetypes/types.go` or `data/1.21.5/play/clientbound/types.go`
- Possibly `internal/generator/packets.go` (if generator fix)
- Possibly protocol JSON files in `versions/1.21.5/`

---

## Bug 2: Set Ticking State - NaN Parsing

### Description
**Error**: `field 'tickRate': expected 20, got NaN`
**Packet**: set_ticking_state (ID=120)
**Severity**: Medium - Packet parses but with incorrect values

### Root Cause Analysis
The `tickRate` field is a float that's being parsed as NaN (Not a Number), indicating a parsing error.

**Expected**: `tickRate: 20.0` (float32)
**Actual**: `tickRate: NaN`

### Investigation Steps
1. Find `ClientboundSetTickingState` in `data/1.21.5/play/clientbound/types.go`
2. Check field order and types
3. Compare with node-minecraft-protocol's packet definition
4. Test ReadFrom method with known bytes

### Fix Plan
1. **Locate packet definition**
   - File: `data/1.21.5/play/clientbound/types.go`
   - Struct: `ClientboundSetTickingState`

2. **Check field order**
   - Verify field order matches protocol spec
   - Check data types (should be float32 for tickRate)

3. **Debug with test packet**
   - Use hex dump from ground truth test: `set_ticking_state`
   - Manually parse bytes to see what's wrong
   - Compare with node-minecraft-protocol parsing

4. **Likely issues**:
   - Field order reversed (isFrozen before tickRate)
   - Wrong type (reading as int instead of float)
   - Missing field causing offset

5. **Fix**:
   - Correct field order in struct definition
   - Ensure correct ReadFrom sequence
   - Regenerate if protocol definition is wrong

6. **Verification**
   - Ground truth test should show `tickRate: 20.0`
   - Test with various tick rates (0.5, 1.0, 20.0, etc.)

### Files to Modify
- `data/1.21.5/play/clientbound/types.go`
- Possibly protocol JSON if field order is wrong

---

## Bug 3: Step Tick - Value Mismatch

### Description
**Error**: `field 'tickSteps': expected 1, got 0`
**Packet**: step_tick (ID=121)
**Severity**: Medium - Off-by-one or field reading error

### Root Cause Analysis
Simple packet with single field shows value mismatch.

**Expected**: `tickSteps: 1` (VarInt)
**Actual**: `tickSteps: 0`

### Investigation Steps
1. Find `ClientboundStepTick` in `data/1.21.5/play/clientbound/types.go`
2. Check if it's reading correct field
3. Verify VarInt decoding
4. Check for off-by-one errors

### Fix Plan
1. **Locate packet definition**
   - File: `data/1.21.5/play/clientbound/types.go`
   - Struct: `ClientboundStepTick`

2. **Verify packet structure**
   - Should have single field: `TickSteps` (VarInt)
   - Check ReadFrom implementation

3. **Debug**
   - Hex dump shows: packet should contain VarInt(1)
   - Check if Scan is reading from correct offset
   - Verify packet.Data doesn't include packet ID

4. **Likely issues**:
   - Reading from wrong offset
   - Default value (0) not being overwritten
   - Field not being read at all

5. **Fix**:
   - Ensure ReadFrom is called and reads the VarInt
   - Check Scan method properly passes data
   - Verify no initialization issues

6. **Verification**
   - Ground truth test: `tickSteps: 1`
   - Test with different values (1, 5, 10, 100)

### Files to Modify
- `data/1.21.5/play/clientbound/types.go`

---

## Bug 4: Add Resource Pack - State Mismatch

### Description
**Error**: `packet ID mismatch: expected 0, got 74`
**Packet**: add_resource_pack (ID=74)
**Severity**: Low - Validator limitation, not parsing bug

### Root Cause Analysis
Ground truth validator assumes all packets are in PLAY state, but `add_resource_pack` can exist in both CONFIG and PLAY states with different IDs.

**Issue**: Validator calls `GetClientboundPacketByID()` which is PLAY-only
**Need**: Multi-state support in validator

### Investigation Steps
1. Check packet manager for add_resource_pack
2. Verify it exists in both CONFIG and PLAY
3. Update validator to detect/handle multiple states

### Fix Plan
1. **Update ground truth validator**
   - File: `internal/packetlogtest/groundtruth.go`
   - Function: `validateTestCase()`

2. **Add state detection**:
   ```go
   // Try each state until we find the packet
   var packetInstance models.PacketMarshaller
   var err error

   // Try PLAY first
   packetInstance, err = packetMgr.GetClientboundPacketByID(models.ClientboundPacketID(packet.ID))
   if err != nil {
       // Try CONFIG
       packetInstance, err = packetMgr.GetClientboundConfigPacketByID(models.ClientboundPacketID(packet.ID))
       if err != nil {
           // Try LOGIN
           packetInstance, err = packetMgr.GetClientboundLoginPacketByID(models.ClientboundPacketID(packet.ID))
       }
   }
   ```

3. **Alternative: Use test case metadata**
   - Add `state` field to GroundTruthTestCase JSON
   - Have generator include state information
   - Use that to call correct factory method

4. **Update packet generator**
   - Include state info in test packet JSON
   - Document which state each packet belongs to

5. **Verification**
   - add_resource_pack should validate successfully
   - Test with packets from all states

### Files to Modify
- `internal/packetlogtest/groundtruth.go`
- `testing/packet-generator/generator.js` (optional, for state metadata)

---

## Implementation Order

### Phase 1: Investigation (1-2 hours)
1. ✅ Read and understand each packet's current implementation
2. ✅ Examine protocol JSON definitions
3. ✅ Review generator templates for special cases
4. ✅ Collect hex dumps and expected values from test packets

### Phase 2: Fixes (3-4 hours)
**Priority Order**:

1. **Bug 4 (Validator fix)** - Easiest, improves testing capability
   - Estimated: 30 minutes
   - Files: 1
   - Risk: Low

2. **Bug 3 (Step Tick)** - Simple value mismatch
   - Estimated: 30 minutes
   - Files: 1
   - Risk: Low

3. **Bug 2 (Ticking State)** - Field parsing issue
   - Estimated: 1 hour
   - Files: 1-2
   - Risk: Medium

4. **Bug 1 (Entity Metadata)** - Most complex, affects generator
   - Estimated: 2 hours
   - Files: 2-3
   - Risk: Medium

### Phase 3: Validation (1 hour)
1. ✅ Run ground truth validation after each fix
2. ✅ Verify existing round-trip tests still pass
3. ✅ Check packet-validation.json for improvements
4. ✅ Document results

### Phase 4: Regeneration (if needed)
If protocol JSON changes or generator templates change:
1. Regenerate all versions: `go run ./cmd/generator -config configs/config.yaml`
2. Build and test: `go build ./... && go test ./...`
3. Re-run full validation suite

---

## Success Criteria

### For Each Bug
- ✅ Ground truth test passes for the packet
- ✅ No new failures introduced
- ✅ Round-trip tests still pass
- ✅ Code compiles without errors

### Overall
- ✅ All 4 bugs fixed
- ✅ Ground truth validation: 8+ packets passing (from current 4)
- ✅ No regressions in packet-validation.json
- ✅ Documentation updated

---

## Testing Strategy

### Unit Tests
Create focused tests for each fix:
```go
func TestEntityMetadataTerminator(t *testing.T) {
    // Test packet with metadata terminator
}

func TestSetTickingStateFloat(t *testing.T) {
    // Test tickRate parsing
}

func TestStepTickValue(t *testing.T) {
    // Test tickSteps value
}
```

### Integration Tests
- Run full ground truth validation suite
- Compare before/after results
- Verify packet counts improve

### Regression Tests
- Run existing packet validation: `go run ./cmd/packet-validation -paths testing/packetlogs -summary`
- Ensure no new failures appear
- Check that failed packet count decreases

---

## Risk Assessment

### Low Risk
- Bug 4 (Validator): Only affects testing, not parsing
- Bug 3 (Step Tick): Single field, isolated change

### Medium Risk
- Bug 2 (Ticking State): Could affect field ordering
- Bug 1 (Entity Metadata): May require generator changes

### Mitigation
1. Make changes incrementally
2. Test after each change
3. Keep git commits separate for easy rollback
4. Verify against multiple test cases

---

## Documentation Updates

After fixes are complete:
1. Update `FINDINGS.md` with results
2. Add notes to `CLAUDE.md` about entity metadata handling
3. Document any generator template changes
4. Update packet validation README with new success rates

---

## Notes

- Entity metadata terminator is the most critical fix
- Some "failures" in ground truth are expected type differences
- Focus on actual parsing bugs, not type system differences
- The ground truth validation tool is valuable for future development
