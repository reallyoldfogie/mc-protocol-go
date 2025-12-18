#!/usr/bin/env node

/**
 * Auto Packet Generator for Minecraft Protocol Testing
 *
 * This tool automatically generates test cases for ALL packets in a version by:
 * 1. Introspecting minecraft-data protocol definitions
 * 2. Generating sensible default values for each field type
 * 3. Serializing with node-minecraft-protocol
 * 4. Outputting ground truth test cases
 *
 * Usage:
 *   node auto-generator.js [--version 1.21.5] [--output auto-tests.jsonl] [--states play,login] [--direction clientbound]
 */

const mc = require('minecraft-protocol')
const mcData = require('minecraft-protocol/node_modules/minecraft-data')
const fs = require('fs')
const path = require('path')

/**
 * Load versions from config file
 */
function loadVersionsFromConfig(configPath) {
  try {
    const configContent = fs.readFileSync(configPath, 'utf8')
    const versions = []

    // Simple parser for YAML versions array
    let inVersionsSection = false
    for (const line of configContent.split('\n')) {
      const trimmed = line.trim()

      if (trimmed === 'versions:') {
        inVersionsSection = true
        continue
      }

      if (inVersionsSection) {
        // Stop if we hit another top-level key
        if (trimmed && !trimmed.startsWith('-') && !trimmed.startsWith('#') && trimmed.includes(':')) {
          break
        }

        // Parse version line
        const match = trimmed.match(/^-\s+"([^"]+)"/)
        if (match) {
          versions.push(match[1])
        }
      }
    }

    return versions
  } catch (err) {
    console.error(`Failed to load config from ${configPath}: ${err.message}`)
    return null
  }
}

/**
 * Build version-protocol mapping dynamically from minecraft-data
 * Includes ALL versions that minecraft-data knows about, not just requested versions
 * This allows us to map any requested version to its protocol and find fallback versions
 */
function buildVersionProtocolMap() {
  const versionProtocolMap = {}

  // Get all versions known to minecraft-data
  const allVersions = mcData.versions.pc

  for (const versionInfo of allVersions) {
    const minecraftVersion = versionInfo.minecraftVersion
    const protocolVersion = versionInfo.version

    if (minecraftVersion && protocolVersion) {
      versionProtocolMap[minecraftVersion] = protocolVersion
    }
  }

  return versionProtocolMap
}

/**
 * Find the best data version to use for a given version
 * Always uses the LATEST version with the same protocol number
 * This ensures we have the most complete/tested protocol data
 */
function findDataVersion(targetVersion, versionProtocolMap) {
  // Get the protocol number for the target version
  const targetProtocol = versionProtocolMap[targetVersion]
  if (!targetProtocol) {
    // Target version not in map, try it directly as fallback
    try {
      const data = mcData(targetVersion)
      if (data) {
        console.warn(`Version ${targetVersion} not in protocol map, using directly`)
        return { version: targetVersion, data }
      }
    } catch (e) {
      // Ignore
    }
    console.error(`Unknown version ${targetVersion} - not in version-protocol map and no data available`)
    return null
  }

  // Find all versions with the same protocol, use the LATEST one
  const sameProtocolVersions = Object.entries(versionProtocolMap)
    .filter(([ver, proto]) => proto === targetProtocol)
    .map(([ver]) => ver)
    .sort() // Sort to get latest (semver-ish)
    .reverse() // Latest first

  // Try each candidate version, starting with the latest
  for (const candidateVersion of sameProtocolVersions) {
    try {
      const data = mcData(candidateVersion)
      if (data) {
        if (candidateVersion !== targetVersion) {
          console.log(`  Using data from ${candidateVersion} for version ${targetVersion} (both use protocol ${targetProtocol})`)
        }
        return { version: candidateVersion, data }
      }
    } catch (e) {
      // Try next candidate
      continue
    }
  }

  console.error(`No data available for any version using protocol ${targetProtocol}`)
  return null
}

// Parse command line arguments
const args = process.argv.slice(2)
let versions = null // Will be loaded from config if not specified
let configPath = '../../configs/config.yaml' // Default config path
let outputFile = 'auto-tests.jsonl'
let outputDir = null // If set, output one file per version
let states = ['handshaking', 'status', 'login', 'configuration', 'play'] // Default to all states
let directions = ['clientbound', 'serverbound'] // Default to both directions
let maxPackets = null // No limit by default
let skipPackets = [] // Packets to skip
let mode = 'both' // zeros | random | both (default: both)
let seed = null // optional deterministic seed for random mode

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--versions' && i + 1 < args.length) {
    versions = args[++i].split(',')
  } else if (args[i] === '--config' && i + 1 < args.length) {
    configPath = args[++i]
  } else if (args[i] === '--output' && i + 1 < args.length) {
    outputFile = args[++i]
  } else if (args[i] === '--output-dir' && i + 1 < args.length) {
    outputDir = args[++i]
  } else if (args[i] === '--states' && i + 1 < args.length) {
    const statesArg = args[++i]
    if (statesArg === 'all') {
      states = ['handshaking', 'status', 'login', 'configuration', 'play']
    } else {
      states = statesArg.split(',')
    }
  } else if (args[i] === '--direction' && i + 1 < args.length) {
    const directionArg = args[++i]
    if (directionArg === 'both') {
      directions = ['clientbound', 'serverbound']
    } else {
      directions = directionArg.split(',')
    }
  } else if (args[i] === '--max' && i + 1 < args.length) {
    maxPackets = parseInt(args[++i])
  } else if (args[i] === '--skip' && i + 1 < args.length) {
    skipPackets = args[++i].split(',')
  } else if (args[i] === '--randomize') {
    mode = 'random'
  } else if (args[i] === '--mode' && i + 1 < args.length) {
    const m = args[++i]
    if (m === 'zeros' || m === 'random' || m === 'both') mode = m
  } else if (args[i] === '--seed' && i + 1 < args.length) {
    const s = Number(args[++i])
    if (!Number.isNaN(s)) seed = s >>> 0
  } else if (args[i] === '--help' || args[i] === '-h') {
    console.log('Usage: node auto-generator.js [options]')
    console.log('')
    console.log('Options:')
    console.log('  --versions <versions>    Comma-separated versions (default: from config)')
    console.log('  --config <path>          Config file path (default: ../../configs/config.yaml)')
    console.log('  --output <file>          Output file for all versions (default: auto-tests.jsonl)')
    console.log('  --output-dir <dir>       Output directory (one file per version, e.g., dir/1.21.5.jsonl)')
    console.log('                           If specified, overrides --output')
    console.log('  --states <states>        Comma-separated states (default: all)')
    console.log('                           Options: all, handshaking,status,login,configuration,play')
    console.log('  --direction <dirs>       Comma-separated directions (default: both)')
    console.log('                           Options: both, clientbound,serverbound')
    console.log('  --max <n>                Maximum packets to generate')
    console.log('  --skip <packets>         Comma-separated packet names to skip')
    console.log('  --randomize              Shortcut for --mode random')
    console.log('  --mode <m>               zeros | random | both (default: both)')
    console.log('  --seed <n>               Deterministic seed for random mode')
    console.log('  --help, -h               Show this help')
    process.exit(0)
  }
}

// Load versions from config if not specified
if (!versions) {
  versions = loadVersionsFromConfig(configPath)
  if (!versions || versions.length === 0) {
    console.error('No versions specified and failed to load from config')
    process.exit(1)
  }
  console.log(`Loaded versions from config: ${versions.join(', ')}`)
}

// Build version-protocol mapping dynamically for fallback support
const versionProtocolMap = buildVersionProtocolMap()
console.log(`Built version-protocol map for ${Object.keys(versionProtocolMap).length} minecraft versions`)

console.log(`Auto-generating test packets for Minecraft versions: ${versions.join(', ')}`)
console.log(`States: ${states.join(', ')}`)
console.log(`Directions: ${directions.join(', ')}`)
console.log(`Mode: ${mode}${seed != null ? ` (seed=${seed})` : ''}`)
console.log()

/**
 * Convert BigInt values to strings for JSON serialization
 */
function bigIntReplacer(key, value) {
  if (typeof value === 'bigint') {
    return value.toString()
  }
  return value
}

/**
 * Deterministic pseudo-random number generator (LCG)
 */
let _rng_state = 0x9e3779b9 // default non-zero
function initRng(s) {
  _rng_state = (s == null ? Date.now() : s) >>> 0
  if (_rng_state === 0) _rng_state = 0x9e3779b9
}
function randUint32() {
  // LCG parameters from Numerical Recipes
  _rng_state = (Math.imul(1664525, _rng_state) + 1013904223) >>> 0
  return _rng_state
}
function randFloat() {
  return (randUint32() >>> 8) / 16777216 // [0,1)
}
function randInt(min, max) {
  if (max < min) [min, max] = [max, min]
  const r = randFloat()
  return Math.floor(r * (max - min + 1)) + min
}
function randBool() { return randUint32() & 1 ? true : false }
function randNonZeroInt(min, max) {
  // Ensure range contains non-zero; fallback to 1 if not
  if (min === 0 && max === 0) return 1
  let val = randInt(min, max)
  if (val === 0) val = (min < 0) ? -1 : 1
  return val
}
function randString(len = 8) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let s = 'auto_'
  for (let i = 0; i < len; i++) s += chars[randInt(0, chars.length - 1)]
  return s
}
function randUUIDv4() {
  // Simple v4 UUID
  const hex = () => randUint32().toString(16).padStart(8, '0')
  const a = hex(); const b = hex(); const c = hex(); const d = hex()
  return `${a.slice(0, 8)}-${b.slice(0, 4)}-4${b.slice(5, 8)}-${(8 + (parseInt(c[0], 16) % 4)).toString(16)}${c.slice(1, 4)}-${d.slice(0, 12)}`
}
function randBuffer(len = 4) {
  const buf = Buffer.alloc(len)
  for (let i = 0; i < len; i++) buf[i] = randInt(1, 255) // non-zero bytes
  return buf
}

// Initialize RNG once args are parsed
initRng(seed)

/**
 * Generate a default value for a given type
 */
function generateDefaultValue(typeName, protocol, depth = 0) {
  // Prevent infinite recursion
  if (depth > 10) {
    return null
  }

  // If typeName is actually a type definition (array), handle it directly
  if (Array.isArray(typeName)) {
    return generateValueFromDefinition(typeName, protocol, depth)
  }

  // Handle built-in types
  switch (typeName) {
    // Integers
    case 'i8': case 'i16': case 'i32': return mode === 'random' || mode === 'both' ? randNonZeroInt(1, 5) : 0
    case 'u8': case 'u16': case 'u32': return mode === 'random' || mode === 'both' ? randNonZeroInt(1, 5) : 0
    case 'varint': return mode === 'random' || mode === 'both' ? randNonZeroInt(1, 5) : 0
    case 'i64': case 'u64': case 'varlong': return (mode === 'random' || mode === 'both') ? BigInt(randNonZeroInt(1, 5)) : 0n

    // Floats
    case 'f32': case 'f64': return (mode === 'random' || mode === 'both') ? (randInt(1, 100) / 10) : 0.0

    // Strings
    case 'string': case 'pstring': return (mode === 'random' || mode === 'both') ? randString(6) : 'test'

    // Boolean
    case 'bool': return (mode === 'random' || mode === 'both') ? randBool() : false

    // UUID
    case 'UUID': return (mode === 'random' || mode === 'both') ? randUUIDv4() : '00000000-0000-0000-0000-000000000000'

    // Buffers
    case 'buffer': case 'restBuffer': case 'ByteArray': return (mode === 'random' || mode === 'both') ? randBuffer(4) : Buffer.alloc(0)

    // Positions
    case 'position': return (mode === 'random' || mode === 'both') ? { x: randNonZeroInt(1, 16), y: randNonZeroInt(1, 255), z: randNonZeroInt(1, 16) } : { x: 0, y: 64, z: 0 }
    case 'vec2f': return (mode === 'random' || mode === 'both') ? { x: randInt(1, 10), y: randInt(1, 10) } : { x: 0, y: 0 }
    case 'vec3f': case 'vec3f64': return (mode === 'random' || mode === 'both') ? { x: randInt(1, 10), y: randInt(1, 10), z: randInt(1, 10) } : { x: 0, y: 0, z: 0 }
    case 'vec4f': return (mode === 'random' || mode === 'both') ? { x: randInt(1, 10), y: randInt(1, 10), z: randInt(1, 10), w: randInt(1, 10) } : { x: 0, y: 0, z: 0, w: 0 }
    case 'vec3i': return (mode === 'random' || mode === 'both') ? { x: randNonZeroInt(1, 10), y: randNonZeroInt(1, 10), z: randNonZeroInt(1, 10) } : { x: 0, y: 0, z: 0 }

    // Special types
    case 'Slot': case 'UntrustedSlot': case 'HashedSlot':
      return { present: false }

    case 'void': return undefined
    case 'anonymousNbt': case 'anonOptionalNbt':
      // Return empty object for anonymousNbt
      // node-minecraft-protocol's NBT serializer will handle it
      return {}

    // Container ID
    case 'ContainerID': return (mode === 'random' || mode === 'both') ? 1 : 0

    default:
      // Look up complex type in protocol
      const typesDef = protocol.types[typeName]
      if (!typesDef) {
        console.warn(`Unknown type: ${typeName}`)
        return null
      }

      return generateValueFromDefinition(typesDef, protocol, depth + 1)
  }
}

/**
 * Generate a value from a type definition
 */
function generateValueFromDefinition(typeDef, protocol, depth = 0) {
  if (!Array.isArray(typeDef) || typeDef.length < 2) {
    return null
  }

  const [kind, spec] = typeDef

  switch (kind) {
    case 'container':
      // Generate object with all fields; handle arrays with explicit count field
      const obj = {}
      const overrides = {}
      // First pass: decide lengths for arrays that rely on an explicit count field,
      // and choose discriminators for switch fields (so compareTo gets set before use).
      for (const field of spec) {
        const t = field.type
        if (Array.isArray(t) && t[0] === 'array') {
          const arrSpec = t[1] || {}
          if (arrSpec && typeof arrSpec.count === 'string') {
            // Choose a length consistent with current mode
            const len = (mode === 'random' || mode === 'both') ? randInt(1, 2) : 0
            overrides[arrSpec.count] = len
          }
        } else if (Array.isArray(t) && t[0] === 'switch') {
          const sw = t[1] || {}
          if (sw && typeof sw.compareTo === 'string' && sw.compareTo) {
            // If discriminator not chosen yet, pick a case now so compareTo is set early
            if (!Object.prototype.hasOwnProperty.call(overrides, sw.compareTo)) {
              const fieldsMap = sw.fields || {}
              const keys = Object.keys(fieldsMap)
              if (keys.length > 0) {
                // Prefer a simple/safe key if present (e.g., '0'), else first
                let chosen = keys[0]
                if (keys.includes('0')) chosen = '0'
                else if (mode === 'random' || mode === 'both') chosen = keys[randInt(0, keys.length - 1)]
                overrides[sw.compareTo] = chosen
              } else if (sw.default) {
                // Leave discriminator unset; default will be used
              }
            }
          }
        }
      }
      // Second pass: populate fields, honoring overrides and generating arrays accordingly
      for (const field of spec) {
        if (Object.prototype.hasOwnProperty.call(overrides, field.name)) {
          obj[field.name] = overrides[field.name]
          continue
        }
        const t = field.type
        // Handle switch with awareness of compareTo discriminator in this container
        if (Array.isArray(t) && t[0] === 'switch') {
          const sw = t[1] || {}
          const fieldsMap = sw.fields || {}
          const keys = Object.keys(fieldsMap)
          let caseKey = undefined
          // Try to respect an already-generated discriminator
          if (typeof sw.compareTo === 'string' && Object.prototype.hasOwnProperty.call(obj, sw.compareTo)) {
            const discrVal = obj[sw.compareTo]
            if (Object.prototype.hasOwnProperty.call(fieldsMap, discrVal)) {
              caseKey = discrVal
            } else if (Object.prototype.hasOwnProperty.call(fieldsMap, String(discrVal))) {
              caseKey = String(discrVal)
            }
          }
          // If no match yet, choose a case and set override for the discriminator
          if (!caseKey) {
            if (keys.length > 0) {
              let chosen = keys[0]
              if (mode === 'random' || mode === 'both') { 
                chosen = keys[randInt(0, keys.length - 1)] 
              } else if (keys.includes('0')) { 
                chosen = '0' 
              }
              else 
              caseKey = chosen
              if (typeof sw.compareTo === 'string' && sw.compareTo) {
                overrides[sw.compareTo] = caseKey
              }
            } else if (sw.default) {
              // No explicit cases; use default branch
              obj[field.name] = generateDefaultValue(sw.default, protocol, depth)
              continue
            } else {
              obj[field.name] = null
              continue
            }
          }
          obj[field.name] = generateDefaultValue(fieldsMap[caseKey], protocol, depth)
          continue
        }
        if (Array.isArray(t) && t[0] === 'array') {
          const arrSpec = t[1] || {}
          if (arrSpec && typeof arrSpec.count === 'string' && Object.prototype.hasOwnProperty.call(overrides, arrSpec.count)) {
            const len = overrides[arrSpec.count] >>> 0
            const out = []
            for (let i = 0; i < len; i++) {
              out.push(generateDefaultValue(arrSpec.type, protocol, depth))
            }
            obj[field.name] = out
            // Ensure count field present even if not part of this container (best effort)
            if (!Object.prototype.hasOwnProperty.call(obj, arrSpec.count)) {
              obj[arrSpec.count] = len
            }
            continue
          }
        }
        obj[field.name] = generateDefaultValue(field.type, protocol, depth)
      }
      return obj

    case 'array':
      // In random mode, generate a small non-empty array when safe
      if ((mode === 'random' || mode === 'both') && spec && (spec.countType || !spec.count)) {
        const len = randInt(1, 2)
        const out = []
        for (let i = 0; i < len; i++) {
          out.push(generateDefaultValue(spec.type, protocol, depth))
        }
        return out
      }
      // Default: empty array
      return []

    case 'option':
      // Return undefined for optional fields
      return undefined

    case 'switch':
      // For switch types, use the default case if available
      // Otherwise, try to use the first case
      if (spec.default) {
        return generateDefaultValue(spec.default, protocol, depth)
      }
      // Try to find a simple case to use
      if (spec.fields && Object.keys(spec.fields).length > 0) {
        const keys = Object.keys(spec.fields)
        const chosenKey = (mode === 'random' || mode === 'both') ? keys[randInt(0, keys.length - 1)] : keys[0]
        return generateDefaultValue(spec.fields[chosenKey], protocol, depth)
      }
      return null

    case 'mapper':
      // Prefer returning the mapping key (string) when available so that
      // downstream switch(compareTo=...) can match case labels.
      try {
        if (spec && spec.mappings && typeof spec.mappings === 'object') {
          const keys = Object.keys(spec.mappings)
          if (keys.length > 0) {
            const key = (mode === 'random' || mode === 'both') ? keys[randInt(0, keys.length - 1)] : keys[0]
            return key
          }
        }
        // Fall back to generating an underlying numeric if mappings absent.
        const underlying = spec && spec.type
        if (underlying === 'varint' || underlying === 'i32' || underlying === 'u32') {
          const upper = (spec && typeof spec.max === 'number') ? spec.max : 113
          return (mode === 'random' || mode === 'both') ? randInt(1, upper) : 0
        }
      } catch (_) {
        // ignore and fall through
      }
      return generateDefaultValue(spec?.type, protocol, depth)

    case 'bitfield':
    case 'bitflags':
      // Return small non-zero mask in random mode
      return (mode === 'random' || mode === 'both') ? 1 : 0

    case 'buffer':
      // Handle fixed-size buffers (e.g., buffer[3] -> {count: 3})
      if (spec && typeof spec.count === 'number') {
        return (mode === 'random' || mode === 'both') ? randBuffer(spec.count) : Buffer.alloc(spec.count)
      }
      // Handle variable-size buffers with countType
      if (spec && spec.countType && (mode === 'random' || mode === 'both')) {
        return randBuffer(4)
      }
      return Buffer.alloc(0)

    case 'registryEntryHolder':
      // Registry entry holders can be either an ID reference or full data
      // node-minecraft-protocol expects either:
      //   - {chatType: <id>} for ID reference mode
      //   - {data: <full_object>} for full data mode
      if (spec && spec.otherwise) {
        // Full data mode - wrap the generated value in a data field
        const fullData = generateDefaultValue(spec.otherwise.type, protocol, depth)
        return { data: fullData }
      }
      // Fallback: use ID reference mode
      // Use the field name from spec (e.g., "chatType" for ChatTypesHolder)
      const idField = spec && spec.chatType !== undefined ? 'chatType' : 'id'
      const idValue = (mode === 'random' || mode === 'both') ? randNonZeroInt(1, 5) : 0
      return { [idField]: idValue }

    case 'registryEntryHolderSet':
      // Set of registry entries - generate empty set for simplicity
      return []

    default:
      console.warn(`Unknown kind: ${kind}`)
      return null
  }
}

/**
 * Generate test parameters for a packet
 */
function generatePacketParams(packetName, packetDef, protocol) {
  if (!Array.isArray(packetDef) || packetDef.length < 2) {
    return null
  }

  const [kind, fields] = packetDef

  if (kind !== 'container') {
    console.warn(`Packet ${packetName} is not a container: ${kind}`)
    return null
  }

  const params = {}
  for (const field of fields) {
    const value = generateDefaultValue(field.type, protocol)
    // Skip undefined values (optional fields that are not present)
    if (value !== undefined) {
      params[field.name] = value
    }
  }

  // Debug logging
  if (packetName === 'chat_message' && process.env.DEBUG) {
    console.log(`Generated params for ${packetName}:`, JSON.stringify(params, (key, value) =>
      typeof value === 'bigint' ? value.toString() + 'n' : value
    , 2))
  }

  return params
}

// Helper to temporarily switch mode and generate params
function generatePacketParamsForMode(packetName, packetDef, protocol, tempMode) {
  const prev = mode
  try {
    mode = tempMode
    return generatePacketParams(packetName, packetDef, protocol)
  } finally {
    mode = prev
  }
}

/**
 * Generate a test packet
 */
function generateTestPacket(version, state, direction, packetName, params, serializer, deserializer) {
  try {
    // Debug logging for chat_message
    if (packetName === 'chat_message' && process.env.DEBUG) {
      console.log(`chat_message params:`, JSON.stringify(params, (key, value) =>
        typeof value === 'bigint' ? value.toString() + 'n' : value
      , 2))
    }

    // Serialize the packet
    const buffer = serializer.createPacketBuffer({ name: packetName, params })

    // Validate by deserializing
    const parsed = deserializer.parsePacketBuffer(buffer)

    // Create test case
    const testCase = {
      description: `Auto-generated ${direction} ${state} ${packetName}`,
      version,
      state,
      direction,
      packetName,
      packetId: parsed.data.name,
      groundTruth: JSON.parse(JSON.stringify(params, bigIntReplacer)),
      serialized: buffer.toString('hex'),
      timestamp: new Date().toISOString()
    }

    return testCase
  } catch (err) {
    console.error(`✗ Failed to generate ${packetName}:`, err.message)
    if (process.env.DEBUG) {
      console.error(err.stack)
    }
    return null
  }
}

// Generate test packets for all versions
// Store packets per version for potential separate file output
const testPacketsByVersion = {}
const testPackets = [] // For single file output
let successCount = 0
let failCount = 0

for (const targetVersion of versions) {
  testPacketsByVersion[targetVersion] = []
  console.log(`\n${'='.repeat(60)}`)
  console.log(`Processing version: ${targetVersion}`)
  console.log(`${'='.repeat(60)}`)

  // Load minecraft-data for this version (with fallback to latest version with same protocol)
  const dataVersionInfo = findDataVersion(targetVersion, versionProtocolMap)
  if (!dataVersionInfo) {
    console.error(`Failed to load data for version ${targetVersion}, skipping...`)
    continue
  }

  const { version: dataVersion, data } = dataVersionInfo

  for (const stateName of states) {
    // Map state names to mc.states
    const stateKey = stateName.toUpperCase()
    const mcState = mc.states[stateKey]

    if (!mcState) {
      console.warn(`Unknown state: ${stateName}`)
      continue
    }

    const stateProtocol = data.protocol[stateName]
    if (!stateProtocol) {
      console.warn(`No protocol data for state: ${stateName}`)
      continue
    }

    for (const directionName of directions) {
      const directionKey = directionName === 'clientbound' ? 'toClient' : 'toServer'
      const packetDefs = stateProtocol[directionKey]

      if (!packetDefs || !packetDefs.types) {
        console.warn(`No ${directionName} packets for state ${stateName}`)
        continue
      }

      // Create serializer/deserializer for this state
      // Use dataVersion for serialization but targetVersion for packet tagging
      // For round-trip validation, serializer and deserializer use OPPOSITE isServer values:
      // - Clientbound: Server sends (isServer=true), Client receives (isServer=false)
      // - Serverbound: Client sends (isServer=false), Server receives (isServer=true)
      const isServerSerializer = directionName === 'clientbound'
      const isServerDeserializer = directionName === 'serverbound'
      let serializer, deserializer
      try {
        serializer = mc.createSerializer({ state: mcState, isServer: isServerSerializer, version: dataVersion })
        deserializer = mc.createDeserializer({ state: mcState, isServer: isServerDeserializer, version: dataVersion })
      } catch (err) {
        console.error(`Failed to create serializer for ${targetVersion} (data version ${dataVersion}): ${err.message}`)
        console.error('Skipping this state/direction')
        continue
      }

      // Get all packet definitions
      const packetNames = Object.keys(packetDefs.types)
        .filter(k => k.startsWith('packet_'))
        .map(k => k.replace('packet_', ''))
        .filter(name => !skipPackets.includes(name))

      console.log(`\nGenerating ${packetNames.length} ${directionName} ${stateName} packets...`)

      for (const packetName of packetNames) {
        if (maxPackets && testPackets.length >= maxPackets) {
          console.log('Reached maximum packet limit')
          break
        }

        const packetDef = packetDefs.types[`packet_${packetName}`]

        // Build protocol context with state-specific types merged with global types
        // This allows lookup of types like ChatTypesHolder which are in play.toClient.types
        const protocolContext = {
          ...data.protocol,
          types: {
            ...data.protocol.types,  // Global types
            ...(packetDefs.types || {}) // State-specific types (includes ChatTypesHolder, etc.)
          }
        }

        const modesToRun = mode === 'both' ? ['zeros', 'random'] : [mode]
        for (const runMode of modesToRun) {
          const params = generatePacketParamsForMode(packetName, packetDef, protocolContext, runMode)
          if (!params) {
            failCount++
            continue
          }

          // Debug logging for chat packets
          if (process.env.DEBUG && packetName.includes('chat')) {
            // Handle BigInt serialization
            console.log(`\nDEBUG ${packetName} [${runMode}] params:`, JSON.stringify(params, (key, value) =>
              typeof value === 'bigint' ? value.toString() + 'n' : value
            , 2))
          }

          const testCase = generateTestPacket(targetVersion, stateName, directionName, packetName, params, serializer, deserializer)
          if (testCase) {
            // Label description with mode when running both
            if (mode === 'both') testCase.description += ` (${runMode})`
            testPackets.push(testCase)
            testPacketsByVersion[targetVersion].push(testCase)
            successCount++
            console.log(`✓ ${packetName}${mode === 'both' ? ` [${runMode}]` : ''}: ${testCase.serialized.length / 2} bytes`)
          } else {
            failCount++
          }
        }
      }
    }

    if (maxPackets && testPackets.length >= maxPackets) {
      break
    }
  }

  if (maxPackets && testPackets.length >= maxPackets) {
    console.log('Reached maximum packet limit across all versions')
    break
  }
}

// Write output files
if (outputDir) {
  // Multi-file mode: one file per version
  const resolvedDir = path.resolve(outputDir)

  // Create directory if it doesn't exist
  if (!fs.existsSync(resolvedDir)) {
    fs.mkdirSync(resolvedDir, { recursive: true })
  }

  const outputFiles = []
  for (const version of versions) {
    const versionPackets = testPacketsByVersion[version]
    if (versionPackets && versionPackets.length > 0) {
      const versionFile = path.join(resolvedDir, `${version}.jsonl`)
      const stream = fs.createWriteStream(versionFile)

      versionPackets.forEach(packet => {
        stream.write(JSON.stringify(packet) + '\n')
      })

      stream.end()
      outputFiles.push(versionFile)
    }
  }

  console.log(`\n${'='.repeat(60)}`)
  console.log(`Versions processed:    ${versions.length}`)
  console.log(`Successfully generated: ${successCount} packets`)
  console.log(`Failed to generate:    ${failCount} packets`)
  console.log(`Total:                 ${successCount + failCount} packets`)
  console.log(`Output directory:      ${resolvedDir}`)
  console.log(`Files created:         ${outputFiles.length}`)
  outputFiles.forEach(file => {
    console.log(`  - ${path.basename(file)}`)
  })
  console.log(`${'='.repeat(60)}`)

  if (successCount > 0) {
    console.log('\nUsage in Go validator:')
    console.log(`  # Validate all versions:`)
    console.log(`  go run ./cmd/groundtruth-validation -test-file ${path.join(resolvedDir, '*.jsonl')}`)
    console.log(`\n  # Or validate specific version:`)
    console.log(`  go run ./cmd/groundtruth-validation -test-file ${outputFiles[0]} -version ${versions[0]}`)
  }
} else {
  // Single file mode: all versions in one file
  const outputPath = path.resolve(outputFile)
  const outputStream = fs.createWriteStream(outputPath)

  testPackets.forEach(packet => {
    outputStream.write(JSON.stringify(packet) + '\n')
  })

  outputStream.end()

  console.log(`\n${'='.repeat(60)}`)
  console.log(`Versions processed:    ${versions.length}`)
  console.log(`Successfully generated: ${successCount} packets`)
  console.log(`Failed to generate:    ${failCount} packets`)
  console.log(`Total:                 ${successCount + failCount} packets`)
  console.log(`Output written to:     ${outputPath}`)
  console.log(`${'='.repeat(60)}`)

  if (successCount > 0) {
    console.log('\nUsage in Go validator:')
    if (versions.length === 1) {
      console.log(`  go run ./cmd/groundtruth-validation -test-file ${outputFile} -version ${versions[0]}`)
    } else {
      console.log(`  # Validate all versions:`)
      console.log(`  go run ./cmd/groundtruth-validation -test-file ${outputFile}`)
      console.log(`\n  # Or validate specific version:`)
      console.log(`  go run ./cmd/groundtruth-validation -test-file ${outputFile} -version ${versions[0]}`)
    }
  }
}
