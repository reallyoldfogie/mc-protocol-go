# Feature: Add Alternative Packet ID Lookup from packets.json

## Overview
Add ID lookup with alternative names based on packets.json for better compatibility with go-mc ID names, in addition to the existing protocol.json names. This enables interoperability between different Minecraft protocol libraries.

## Background
Currently, packet names are derived solely from protocol.json. This feature will add alternative names from packets.json (compatible with go-mc naming) while maintaining backward compatibility with existing code.

## Requirements
- Build a map by namespace and direction (clientbound/serverbound) from packets.json
- Generate alternative names (e.g., ClientboundUpdateTags, ClientboundConfigUpdateTags)
- Use protocolIDs to match main name (from protocol.json) with alt name (from packets.json)
- Switch statements in Get*PacketID functions should case on both main and alt names
- Transform packet names from "minecraft:packet_*" format by trimming "minecraft:" prefix

## Files Involved
- `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go`
  - `updatePacketDataWithStructNames` (line 212)
  - New function: `enrichPacketDataWithAltNames`
  - New helper: `packetJsonNameToIdentifier`
- `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/generator.go`
  - `generatePacketIDs()` call at line 176 (add enrichment call)
- `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/.cache/metadata/<version>/data_generator/reports/packets.json`
  - Source data for alternative names
- `/home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/templates.go`
  - `packetTemplate` (line 311)
  - Update all `Get*PacketID()` functions to include alt name cases

## Data Structure

### packets.json Structure
```json
{
    "namespace": {
        "clientbound": {
            "minecraft:packet_name": {
                "protocol_id": 0
            }
        },
        "serverbound": {
            "minecraft:packet_name": {
                "protocol_id": 0
            }
        }
    }
}
```

### Name Transformation Examples
- `configuration:clientbound:"minecraft:update_tags"` → `ClientboundConfigUpdateTags` (in GetClientboundConfigPacketID)
- `login:clientbound:"minecraft:cookie_request"` → `ClientboundLoginCookieRequest` (in GetClientboundLoginPacketID)
- `play:clientbound:"minecraft:block_update"` → `ClientboundBlockUpdate` (in GetClientboundPacketID)

## Implementation Plan

### Phase 1: Data Structure Enhancement
**Duration: 1-2 hours**

1. **Extend packetData struct** (`internal/generator/packets.go`, line 54)
   - Add `AltName string` field to store alternative name from packets.json
   - Maintains backward compatibility (existing code uses `Name` field)

### Phase 2: packets.json Parsing
**Duration: 2-3 hours**

2. **Create JSON parsing function** (`internal/generator/packets.go`)
   - Function: `loadPacketsJson(path string) (map[string]map[string]map[string]int64, error)`
   - Returns: `namespace → direction → packet_name → protocol_id`
   - Handle missing file gracefully (log warning, continue)

3. **Create name transformation helper** (`internal/generator/packets.go`)
   - Function: `packetJsonNameToIdentifier(namespace, direction, packetName string) string`
   - Strip "minecraft:" prefix
   - Remove "packet_" prefix if present
   - Convert snake_case to PascalCase
   - Apply direction prefix (Clientbound/Serverbound)
   - Apply namespace suffix for non-play (Login, Config, Status)
   - Examples:
     - `configuration:clientbound:update_tags` → `ClientboundConfigUpdateTags`
     - `play:serverbound:chat_command` → `ServerboundChatCommand`

### Phase 3: Data Enrichment
**Duration: 2-3 hours**

4. **Create enrichment function** (`internal/generator/packets.go`)
   - Function: `enrichPacketDataWithAltNames(packetData *inversePacketParse, packetsJsonPath string) error`
   - Load packets.json
   - For each namespace (configuration, login, play, handshake, status):
     - For each direction (clientbound, serverbound):
       - Match protocol_id from packets.json to existing packetData entries
       - Generate alternative name using transformation helper
       - Store in `AltName` field
       - Skip if alt name equals primary name (avoid duplicates)

5. **Integrate enrichment into generator flow** (`internal/generator/generator.go`, line 176)
   - Call `enrichPacketDataWithAltNames` after `updatePacketDataWithStructNames`
   - Pass packets.json path from files map
   - Handle errors gracefully (warn but continue)

### Phase 4: Template Updates
**Duration: 2-3 hours**

6. **Update packetTemplate** (`internal/generator/templates.go`, line 311)
   - Modify each `Get*PacketID` function to include alt name cases:
     ```go
     case "LoginClientbound{{$value.Name}}":
     {{if $value.AltName}}case "{{$value.AltName}}":{{end}}
         return LoginClientbound{{$value.Name}}
     ```
   - Functions to update:
     - `GetClientboundLoginPacketID` (line 322)
     - `GetServerboundLoginPacketID` (line 333)
     - `GetClientboundConfigPacketID` (line 354)
     - `GetServerboundConfigPacketID` (line 375)
     - `GetClientboundPacketID` (line 396)
     - `GetServerboundPacketID` (line 418)

### Phase 5: Testing & Validation
**Duration: 2-3 hours**

7. **Code generation testing**
   - Run generator on test version (1.21.2)
   - Verify `packetid.go` includes both name variants in switch cases
   - Verify no duplicate case statements
   - Check for proper PascalCase formatting

8. **Functional testing**
   - Test lookup with protocol.json names (backward compatibility)
   - Test lookup with packets.json alternative names (new feature)
   - Verify both resolve to correct packet IDs
   - Test edge cases (missing packets.json, malformed data)

### Phase 6: Documentation & Cleanup
**Duration: 1 hour**

9. **Code documentation**
   - Add godoc comments to new functions
   - Document the dual-name lookup behavior
   - Add inline comments for complex transformations

10. **Remove debug output** (if any added during development)
    - Clean up temporary fmt.Printf statements
    - Ensure production-ready code quality

## Implementation Timeline

### Total Estimated Time: 10-15 hours

| Phase | Task | Duration | Dependencies |
|-------|------|----------|-------------|
| 1 | Data structure enhancement | 1-2 hours | None |
| 2 | packets.json parsing & transformation | 2-3 hours | Phase 1 |
| 3 | Data enrichment & integration | 2-3 hours | Phase 2 |
| 4 | Template updates | 2-3 hours | Phase 3 |
| 5 | Testing & validation | 2-3 hours | Phase 4 |
| 6 | Documentation & cleanup | 1 hour | Phase 5 |

**Estimated Completion: 2-3 days** (assuming part-time work)

## Key Design Decisions

1. **Backward Compatibility**: Existing protocol.json names remain primary; alt names are additions
2. **No Breaking Changes**: Templates generate both case statements, existing code unaffected
3. **Graceful Degradation**: If packets.json missing/malformed, log warning but continue
4. **Duplicate Prevention**: Skip alt name if it matches primary name
5. **Error Handling**: Non-fatal errors for missing alt names (optimization, not requirement)

## Success Criteria

- [ ] Generator successfully processes packets.json for all supported versions
- [ ] Generated code includes both primary and alternative name cases
- [ ] Existing tests pass without modification (backward compatibility)
- [ ] New lookups work with go-mc compatible names
- [ ] No duplicate case statements in generated code
- [ ] Proper error handling for missing/malformed packets.json
- [ ] Code follows project conventions and passes linting
- [ ] `go vet ./...` succeeds with no errors (in both generator and generated code)

## Future Enhancements (Out of Scope)

- Generate reverse lookup (packet ID → alternative names)
- Support for custom name mappings via configuration
- Auto-detection of naming scheme (protocol.json vs packets.json style)
- Validation tool to check name consistency across versions
