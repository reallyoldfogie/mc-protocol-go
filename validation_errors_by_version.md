# Validation Errors by Version - Detailed Breakdown

This document provides a detailed breakdown of validation errors for each version.

## Summary Statistics

| Version | Total | Pass | Fail | Pass % | Top Error Type | Count |
|---------|-------|------|------|--------|----------------|-------|
| 1.21.1 | 168 | 100 | 68 | (59.5%) | Position type | ~15 |
| 1.21.2 | 173 | 101 | 72 | (58.4%) | Position type | ~15 |
| 1.21.3 | 173 | 101 | 72 | (58.4%) | Position type | ~15 |
| 1.21.4 | 173 | 100 | 73 | (57.8%) | Position type | ~15 |
| 1.21.5 | 176 | 99 | 77 | (56.2%) | Position type | ~15 |
| 1.21.6 | 176 | 94 | 82 | (53.4%) | Position type | ~15 |
| 1.21.7 | 176 | 94 | 82 | (53.4%) | Position type | ~15 |
| 1.21.8 | 176 | 94 | 82 | (53.4%) | Position type | ~15 |

---

## Error Categories (Common Across All Versions)

### 1. Position Type Mismatch (~15-17 per version)
**Pattern:** `expected map[x:0 y:64 z:0], got {0 0 64} (basetypes.Position)`

Affected packets:
- block_break_animation
- block_action  
- block_change
- world_event
- spawn_position
- open_sign_entity
- query_block_nbt
- generate_structure
- pick_item_from_block
- block_dig
- update_command_block
- update_jigsaw_block
- update_structure_block
- set_test_block
- update_sign
- test_instance_block_action
- block_place

### 2. Array Content Mismatch (~40-45 per version)
**Pattern:** `expected [], got {{0xc000...} <nil>} (models.Array[...])`

Affects virtually all packets with array fields.

### 3. ByteArray/Buffer Mismatch (~10-12 per version)
**Pattern:** `expected map[data:[] type:Buffer], got []`

Affected packets:
- encryption_begin (publicKey, verifyToken, sharedSecret)
- chat_session_update (publicKey, signature)
- custom_payload (data field)
- map_chunk (chunkData)
- login_plugin_request (data)

### 4. Long as String (~3-4 per version)
**Pattern:** `expected string "0", got int64(0)`

Affected fields:
- keepAliveId
- id (ping/pong)
- age, time (update_time)
- expireTime

### 5. Field Not Found (~7-10 per version)
**Pattern:** `field 'field_name' not found in packet struct`

Common missing fields:
- tick_steps, tick_rate, is_frozen (ticking state packets)
- size_x, size_y, size_z (structure blocks)
- offset_x, offset_y, offset_z (structure blocks)
- selection_priority, placement_priority (jigsaw blocks)
- entity_name (reset_score)
- feet_eyes (face_player)
- window_id, slot_id (inventory packets)
- track_output (command blocks)

---

## Version-Specific Issues

### 1.21.1-1.21.4
- No major version-specific issues
- Slightly fewer total packets (168-173 vs 176)
- Consistent error patterns

### 1.21.5+
- **New issue:** Particle type switch errors
  - `world_particles` packet fails with unknown case value "angry_villager"
  - Particle system was updated in this version

### 1.21.6-1.21.8
- Success rate drops from 56-59% to 53%
- Additional complex packets introduced
- More nested struct comparison failures

---

## Not Yet Implemented (All Versions)

The following packets consistently fail across all versions with "not implemented yet":

**Handshaking State (2 packets):**
- set_protocol
- legacy_server_list_ping

**Status State (4 packets):**
- server_info (clientbound)
- ping (clientbound)
- ping_start (serverbound)
- ping (serverbound)

These require adding handshaking/status state support to the validation framework.

---

## Top 10 Failing Packets (by frequency across versions)

1. **map_chunk** - 9 field mismatches (arrays, buffer, position in heightmaps)
2. **update_light** - 6 field mismatches (all arrays)
3. **update_structure_block** - 8 field mismatches (position + missing fields)
4. **test_instance_block_action** - 2 complex struct mismatches
5. **player_info** - 2 mismatches (array + bitflags)
6. **advancements** - 3 array mismatches
7. **declare_commands** - 1 array mismatch (but complex CommandNode type)
8. **declare_recipes** - 2 array mismatches
9. **encryption_begin** - 2 buffer mismatches
10. **custom_payload** - 1 buffer/void mismatch (multiple states)

---

## Detailed Per-Version Logs

Full validation logs available at:
- `/tmp/validation-1.21.1.log`
- `/tmp/validation-1.21.2.log`
- `/tmp/validation-1.21.3.log`
- `/tmp/validation-1.21.4.log`
- `/tmp/validation-1.21.5.log`
- `/tmp/validation-1.21.6.log`
- `/tmp/validation-1.21.7.log`
- `/tmp/validation-1.21.8.log`

---

*Generated: 2025-11-19*
