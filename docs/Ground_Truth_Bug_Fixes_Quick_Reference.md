# Ground Truth Bug Fixes - Quick Reference

Quick reference for fixing the 4 bugs discovered through ground truth validation.

## Bug Summary

| # | Bug | Packet | Severity | Effort | Files |
|---|-----|--------|----------|--------|-------|
| 1 | Entity Metadata Terminator | entity_metadata | High | 2h | 2-3 |
| 2 | Ticking State NaN | set_ticking_state | Medium | 1h | 1-2 |
| 3 | Step Tick Value | step_tick | Medium | 30m | 1 |
| 4 | Validator State Support | add_resource_pack | Low | 30m | 1 |

## Recommended Order

1. **Bug 4** (30min) - Improves testing capability
2. **Bug 3** (30min) - Simple fix
3. **Bug 2** (1h) - Medium complexity
4. **Bug 1** (2h) - Most complex

**Total Estimated Time**: 4 hours

## Quick Fixes

### Bug 4: Validator State Support
**File**: `internal/packetlogtest/groundtruth.go`
**Line**: ~116 (in `validateTestCase`)

**Current**:
```go
packetInstance, err := packetMgr.GetClientboundPacketByID(models.ClientboundPacketID(packet.ID))
```

**Fix**:
```go
// Try PLAY, CONFIG, then LOGIN states
packetInstance, err := packetMgr.GetClientboundPacketByID(models.ClientboundPacketID(packet.ID))
if err != nil {
    packetInstance, err = packetMgr.GetClientboundConfigPacketByID(models.ClientboundPacketID(packet.ID))
    if err != nil {
        packetInstance, err = packetMgr.GetClientboundLoginPacketByID(models.ClientboundPacketID(packet.ID))
    }
}
```

### Bug 3: Step Tick Value
**File**: `data/1.21.5/play/clientbound/types.go`
**Search**: `type ClientboundStepTick struct`

**Check**:
1. Verify single field: `TickSteps pk.VarInt`
2. Check ReadFrom reads the VarInt
3. Check Scan passes data correctly

**Likely fix**: Ensure field is being read from packet data

### Bug 2: Ticking State NaN
**File**: `data/1.21.5/play/clientbound/types.go`
**Search**: `type ClientboundSetTickingState struct`

**Check**:
1. Field order: `TickRate pk.Float` then `IsFrozen pk.Boolean`
2. Verify protocol spec matches
3. Check ReadFrom reads float before boolean

**Likely fix**: Swap field order or fix ReadFrom sequence

### Bug 1: Entity Metadata Terminator
**File**: `data/1.21.5/play/clientbound/types.go` or `basetypes/types.go`
**Search**: `EntityMetadata`

**Check**:
1. How metadata array is parsed
2. If terminator (0xFF or 0x7F) is handled
3. Protocol definition for entity metadata

**Options**:
- Add terminator to protocol JSON
- Custom ReadFrom for metadata array
- Update generator template

## Validation Commands

```bash
# After each fix
go run ./cmd/groundtruth-validation -test-file testing/packet-generator/test-packets.jsonl

# Full regression test
go run ./cmd/packet-validation -paths testing/packetlogs -summary

# Build check
go build ./...
```

## Expected Results

### Before Fixes
- Ground truth passing: 4/16 (25%)
- Entity metadata: FAIL (terminator error)
- Set ticking state: FAIL (NaN)
- Step tick: FAIL (value 0 vs 1)
- Add resource pack: FAIL (state mismatch)

### After Fixes
- Ground truth passing: 8+/16 (50%+)
- Entity metadata: PASS
- Set ticking state: PASS
- Step tick: PASS
- Add resource pack: PASS

## Git Workflow

```bash
# Create branch for fixes
git checkout -b fix/ground-truth-bugs

# Make fixes incrementally, commit after each
git add .
git commit -m "Fix ground truth validator multi-state support"
git commit -m "Fix StepTick value parsing"
git commit -m "Fix SetTickingState float parsing"
git commit -m "Fix entity metadata terminator handling"

# Final validation
go run ./cmd/groundtruth-validation -test-file testing/packet-generator/test-packets.jsonl
go run ./cmd/packet-validation -paths testing/packetlogs -summary

# Commit updated results
git add testing/packet-generator/FINDINGS.md
git commit -m "Update ground truth validation results after fixes"
```

## Testing Checklist

- [ ] Bug 4 fix: add_resource_pack validates
- [ ] Bug 3 fix: step_tick shows tickSteps=1
- [ ] Bug 2 fix: set_ticking_state shows tickRate=20.0
- [ ] Bug 1 fix: entity_metadata parses without error
- [ ] No regressions in existing tests
- [ ] Build succeeds: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Ground truth improvement: 4→8+ passing
- [ ] Documentation updated

## Success Metrics

**Primary**: All 4 bugs fixed and validated
**Secondary**: Ground truth pass rate increases to 50%+
**Tertiary**: No new failures in packet-validation.json
