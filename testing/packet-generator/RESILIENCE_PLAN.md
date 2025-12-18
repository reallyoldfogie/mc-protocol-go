# Auto-Generator Resilience Plan

## Problem Analysis

Running the auto-generator against all 8 versions in the config revealed several critical issues:

### 1. **Version Support Crash** (Critical)
```
Error: Unsupported protocol version '768' (attempted to use '767' data)
```
- **Issue**: The tool crashes when processing version 1.21.2
- **Root Cause**: minecraft-protocol/minecraft-data version mismatch with protocol numbers
- **Impact**: Tool cannot complete multi-version generation
- **Priority**: P0 - Blocks all multi-version workflows

### 2. **Array Handling** (High Frequency)
```
SizeOf error: Cannot read properties of null (reading 'length')
```
- **Issue**: Empty arrays return `null` instead of `[]`
- **Affected Packets**: statistics, success, registry_data, feature_flags, tags, tab_complete, declare_commands, window_items, chat_suggestions, chunk_biomes
- **Impact**: ~30-40% of packet generation failures
- **Priority**: P1 - Major impact on success rate

### 3. **Buffer Handling** (High Frequency)
```
The first argument must be of type string or an instance of Buffer... Received null
```
- **Issue**: Empty buffers return `null` instead of `Buffer.alloc(0)`
- **Affected Packets**: encryption_begin (both directions)
- **Impact**: ~10-15% of packet generation failures
- **Priority**: P1 - Affects critical security packets

### 4. **Void/Empty Packets** (Medium Frequency)
```
Read error: Unexpected buffer end while reading VarInt
```
- **Issue**: Packets with no fields generate empty data, but serializer expects at least a VarInt
- **Affected Packets**: ping_start, reset_chat, login_acknowledged, chunk_batch_start, pong, abilities, etc.
- **Impact**: ~15-20% of packet generation failures
- **Priority**: P2 - Common pattern across many simple packets

### 5. **Complex Type Recognition** (Low Impact)
```
Unknown type: buffer,[object Object]
Unknown type: array,[object Object]
Unknown type: switch,[object Object]
Unknown type: option,vec3f64
Unknown type: option,restBuffer
```
- **Issue**: Complex nested types not recognized by default value generator
- **Impact**: Informational warnings, but leads to null values
- **Priority**: P2 - Contributes to failures but not direct cause

### 6. **Undefined Field Access** (Medium Frequency)
```
Read error for undefined : undefined
```
- **Issue**: Various packets with missing or incorrectly generated fields
- **Affected Packets**: acknowledge_player_digging, block_break_animation, tile_entity_data, set_cooldown, custom_payload, disconnect, damage_event, etc.
- **Impact**: ~10-15% of packet generation failures
- **Priority**: P2 - Requires case-by-case investigation

### 7. **Optional Complex Types** (Low Frequency)
```
Cannot read properties of undefined (reading 'length')
```
- **Issue**: Optional fields with complex types generate `undefined` which breaks serialization
- **Affected Packets**: set_slot, set_creative_slot
- **Impact**: <5% of packet generation failures
- **Priority**: P3 - Edge cases

## Success Metrics (Version 1.21.1)

From the partial run:
- **Handshaking**: 2/2 successful (100%)
- **Status**: 3/4 successful (75%)
- **Login Clientbound**: 2/5 successful (40%)
- **Login Serverbound**: 1/4 successful (25%)
- **Configuration Clientbound**: 3/9 successful (33%)
- **Configuration Serverbound**: 4/5 successful (80%)
- **Play Clientbound**: Partial run, many failures
- **Play Serverbound**: Not measured

**Overall Success Rate**: Approximately 30-40% across all packet types

## Proposed Solutions

### Phase 1: Critical Fixes (P0)

#### 1.1 Version Error Handling
**Goal**: Prevent crashes, continue processing other versions

```javascript
// In the version processing loop
const data = mcData(version)
if (!data) {
  console.error(`Failed to load data for version ${version}, skipping...`)
  continue
}

// Wrap serializer creation in try-catch
try {
  const serializer = mc.createSerializer({ state: mcState, isServer, version })
  const deserializer = mc.createDeserializer({ state: mcState, isServer, version })
} catch (err) {
  console.error(`Failed to create serializer for ${version}: ${err.message}`)
  console.error('Skipping this version')
  continue
}
```

**Benefits**:
- Tool completes full run even with unsupported versions
- Generates files for all supported versions
- Clear error messages for debugging

#### 1.2 Graceful Packet Failure Handling
**Goal**: One packet failure doesn't crash the tool

```javascript
// Already implemented, but ensure all paths are covered
try {
  const testCase = generateTestPacket(...)
  if (testCase) {
    // Success
  } else {
    // Failure logged, continue
  }
} catch (err) {
  console.error(`Unexpected error generating ${packetName}:`, err.message)
  failCount++
  continue
}
```

### Phase 2: High-Impact Fixes (P1)

#### 2.1 Fix Array Default Values
**Goal**: Return empty arrays instead of null

```javascript
// In generateDefaultValue()
case 'array':
  // Return empty array - even if spec says element type is required,
  // an empty array is valid JSONL and won't crash serializer
  return []
```

**Impact**: Should fix ~30-40% of failures immediately

#### 2.2 Fix Buffer Default Values
**Goal**: Return empty buffers instead of null

```javascript
// In generateDefaultValue()
case 'buffer':
case 'restBuffer':
case 'ByteArray':
  return Buffer.alloc(0)

// Also handle buffer type in spec
case 'buffer':
  if (spec.countType) {
    return Buffer.alloc(0)
  }
  return Buffer.alloc(0)
```

**Impact**: Should fix ~10-15% of failures

#### 2.3 Detect Complex Array Types
**Goal**: Handle nested array definitions properly

```javascript
function generateDefaultValue(typeName, protocol, depth = 0) {
  // ...existing code...

  // Check if this is an array type name
  if (typeName.startsWith('array<') || (Array.isArray(typeName) && typeName[0] === 'array')) {
    return []
  }

  // Check if this is a buffer type name
  if (typeName === 'buffer' || typeName === 'restBuffer') {
    return Buffer.alloc(0)
  }
}
```

### Phase 3: Medium-Impact Fixes (P2)

#### 3.1 Better Type Detection in generateValueFromDefinition
**Goal**: Improve handling of complex type definitions

```javascript
function generateValueFromDefinition(typeDef, protocol, depth = 0) {
  if (!Array.isArray(typeDef) || typeDef.length < 2) {
    // Try to handle it as a simple type name
    if (typeof typeDef === 'string') {
      return generateDefaultValue(typeDef, protocol, depth)
    }
    return null
  }

  const [kind, spec] = typeDef

  switch (kind) {
    case 'buffer':
      return Buffer.alloc(0)

    case 'array':
      return []

    case 'option':
      // For optional types, return undefined for simplicity
      // BUT: some packets require non-undefined optional values
      // Consider returning a simple default instead
      if (typeof spec === 'string') {
        // For simple optional types, return undefined
        return undefined
      } else {
        // For complex optional types, generate the inner type
        // This might work better for serialization
        return generateValueFromDefinition(spec, protocol, depth + 1)
      }

    // ... rest of cases
  }
}
```

#### 3.2 Add Fallback for Unknown Types
**Goal**: Don't let unknown types return null silently

```javascript
default:
  // Log unknown types but try to continue
  if (process.env.DEBUG) {
    console.warn(`Unknown type during generation: ${typeName}`)
  }

  // Try to look it up in protocol
  const typesDef = protocol.types ? protocol.types[typeName] : null
  if (typesDef) {
    return generateValueFromDefinition(typesDef, protocol, depth + 1)
  }

  // Last resort: return null and let the caller handle it
  return null
```

#### 3.3 Handle Void Packets
**Goal**: Skip or generate minimal data for packets with no fields

```javascript
function generatePacketParams(packetName, packetDef, protocol) {
  if (!Array.isArray(packetDef) || packetDef.length < 2) {
    return null
  }

  const [kind, fields] = packetDef

  if (kind !== 'container') {
    console.warn(`Packet ${packetName} is not a container: ${kind}`)
    return null
  }

  // Handle empty containers (void packets)
  if (!fields || fields.length === 0) {
    console.warn(`Packet ${packetName} has no fields (void packet), skipping`)
    return null
  }

  const params = {}
  for (const field of fields) {
    params[field.name] = generateDefaultValue(field.type, protocol)
  }

  return params
}
```

### Phase 4: Low-Priority Improvements (P3)

#### 4.1 Add Debug Mode
```javascript
// Add --debug flag
let debugMode = false
if (args[i] === '--debug') {
  debugMode = true
  process.env.DEBUG = '1'
}

// Use throughout code
if (debugMode) {
  console.log('Full error:', err.stack)
  console.log('Packet def:', JSON.stringify(packetDef, null, 2))
}
```

#### 4.2 Per-Version Error Summary
```javascript
// Track errors per version
const errorsByVersion = {}

// At end of version processing
console.log(`\nVersion ${version} Summary:`)
console.log(`  Success: ${versionSuccessCount}`)
console.log(`  Failed: ${versionFailCount}`)
if (versionFailCount > 0) {
  console.log(`  Top failure reasons:`)
  // Group and display top errors
}
```

#### 4.3 Skip List File Support
```javascript
// Allow loading skip list from file
if (args[i] === '--skip-file' && i + 1 < args.length) {
  const skipFile = args[++i]
  const skipContent = fs.readFileSync(skipFile, 'utf8')
  skipPackets = skipContent.split('\n').map(line => line.trim()).filter(l => l && !l.startsWith('#'))
}
```

## Implementation Priority

1. **Immediate** (Can do in current session):
   - Fix version error handling (try-catch around serializer creation)
   - Fix array default values (return [] instead of null)
   - Fix buffer default values (return Buffer.alloc(0) instead of null)

2. **Short-term** (Next session):
   - Improve type detection
   - Handle void packets
   - Add debug mode

3. **Medium-term** (Future enhancement):
   - Per-version error summaries
   - Skip list file support
   - Better complex type handling

## Expected Outcomes

After Phase 1 & 2 fixes:
- **Success rate**: 50-60% (up from 30-40%)
- **Multi-version**: Tool completes all versions without crashing
- **Usability**: Clear error messages, continues on failures

After Phase 3:
- **Success rate**: 65-75%
- **Coverage**: Most common packet types work

After Phase 4:
- **Developer experience**: Excellent debugging capabilities
- **Maintainability**: Easy to add exceptions and special cases

## Testing Strategy

1. **Unit testing**: Test each type generator with edge cases
2. **Integration testing**: Run against all versions
3. **Metrics**: Track success rate per version, per state, per direction
4. **Regression**: Compare before/after success rates

## Notes

- Some packets may be inherently impossible to auto-generate (e.g., chunks, NBT data)
- Manual test cases in `generator.js` still needed for complex scenarios
- Focus on high-value packets (handshaking, status, login, basic play)
