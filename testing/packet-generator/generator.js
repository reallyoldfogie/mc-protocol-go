#!/usr/bin/env node

/**
 * Packet Generator for Minecraft Protocol Testing
 *
 * This tool uses node-minecraft-protocol as a reference implementation to:
 * 1. Serialize packets with known field values
 * 2. Output test cases with ground truth data for validation
 *
 * Usage:
 *   node generator.js [--version 1.21.5] [--output test-packets.jsonl]
 */

const mc = require('minecraft-protocol')
const fs = require('fs')
const path = require('path')

// Parse command line arguments
const args = process.argv.slice(2)
let version = '1.21.5'
let outputFile = 'test-packets.jsonl'

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--version' && i + 1 < args.length) {
    version = args[++i]
  } else if (args[i] === '--output' && i + 1 < args.length) {
    outputFile = args[++i]
  } else if (args[i] === '--help' || args[i] === '-h') {
    console.log('Usage: node generator.js [--version 1.21.5] [--output test-packets.jsonl]')
    console.log('')
    console.log('Options:')
    console.log('  --version <version>  Minecraft version (default: 1.21.5)')
    console.log('  --output <file>      Output file (default: test-packets.jsonl)')
    console.log('  --help, -h           Show this help')
    process.exit(0)
  }
}

console.log(`Generating test packets for Minecraft ${version}`)

// Create a server-side client (which can serialize packets)
const serializer = mc.createSerializer({
  state: mc.states.PLAY,
  isServer: true,
  version
})

// Create deserializer for validation
const deserializer = mc.createDeserializer({
  state: mc.states.PLAY,
  isServer: true,
  version
})

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
 * Generate a test packet with known values
 * @param {string} packetName - The packet name (e.g., 'player_info')
 * @param {object} params - The packet parameters with known values
 * @param {string} description - Description of what this test case validates
 * @returns {object} Test case object with ground truth and serialized data
 */
function generateTestPacket(packetName, params, description) {
  try {
    // Serialize the packet
    const buffer = serializer.createPacketBuffer({ name: packetName, params })

    // Validate by deserializing
    const parsed = deserializer.parsePacketBuffer(buffer)

    // Create test case
    const testCase = {
      description,
      version,
      packetName,
      packetId: parsed.data.name, // Actual packet ID used
      groundTruth: JSON.parse(JSON.stringify(params, bigIntReplacer)),
      serialized: buffer.toString('hex'),
      timestamp: new Date().toISOString()
    }

    console.log(`✓ Generated ${packetName}: ${buffer.length} bytes`)
    return testCase
  } catch (err) {
    console.error(`✗ Failed to generate ${packetName}:`, err.message)
    if (process.env.DEBUG) {
      console.error(err.stack)
    }
    return null
  }
}

// Test packet definitions
const testPackets = []

// Simple packets first (avoid player_info for now as it's complex and changed in recent versions)

// Position packet
testPackets.push(generateTestPacket('position', {
  x: 123.456,
  y: 64.0,
  z: -789.012,
  yaw: 180.0,
  pitch: -45.0,
  flags: 0x00,
  teleportId: 42
}, 'Player position update'))

// Keep alive
testPackets.push(generateTestPacket('keep_alive', {
  keepAliveId: 12345678n
}, 'Keep alive packet'))

// Chat message
testPackets.push(generateTestPacket('system_chat', {
  content: JSON.stringify({ text: 'Hello, World!' }),
  overlay: false
}, 'System chat message'))

// Set experience
testPackets.push(generateTestPacket('experience', {
  experienceBar: 0.5,
  level: 10,
  totalExperience: 500
}, 'Experience update'))

// Update health
testPackets.push(generateTestPacket('update_health', {
  health: 15.5,
  food: 18,
  foodSaturation: 5.0
}, 'Health and hunger update'))

// Entity metadata - simple metadata update
testPackets.push(generateTestPacket('entity_metadata', {
  entityId: 123,
  metadata: [
    // Index 0: Byte flags
    { key: 0, type: 0, value: 0 },
    // Terminator
    { key: 255, type: 0, value: 0 }
  ]
}, 'Entity metadata with basic flags'))

// Difficulty
testPackets.push(generateTestPacket('difficulty', {
  difficulty: 2, // Normal
  locked: false
}, 'Set difficulty to normal'))

// Held item slot
testPackets.push(generateTestPacket('held_item_slot', {
  slot: 0
}, 'Set held item slot'))

// Update time
testPackets.push(generateTestPacket('update_time', {
  age: 1000n,
  time: 6000n
}, 'Update world time'))

// Spawn position
testPackets.push(generateTestPacket('spawn_position', {
  location: { x: 0, y: 64, z: 0 },
  angle: 0.0
}, 'Set spawn position'))

// Update view position
testPackets.push(generateTestPacket('update_view_position', {
  chunkX: 0,
  chunkZ: 0
}, 'Update view position'))

// Abilities
testPackets.push(generateTestPacket('abilities', {
  flags: 2, // Flying
  flyingSpeed: 0.05,
  walkingSpeed: 0.1
}, 'Player abilities'))

// Entity update attributes - simple attribute
testPackets.push(generateTestPacket('entity_update_attributes', {
  entityId: 123,
  properties: [
    {
      key: 'minecraft:generic.max_health',
      value: 20.0,
      modifiers: []
    }
  ]
}, 'Entity attributes - max health'))

// Initialize world border
testPackets.push(generateTestPacket('initialize_world_border', {
  x: 0.0,
  z: 0.0,
  oldDiameter: 60000000.0,
  newDiameter: 60000000.0,
  speed: 0n,
  portalTeleportBoundary: 29999984,
  warningBlocks: 5,
  warningTime: 15
}, 'Initialize world border'))

// Set ticking state
testPackets.push(generateTestPacket('set_ticking_state', {
  tickRate: 20.0,
  isFrozen: false
}, 'Set ticking state'))

// Step tick
testPackets.push(generateTestPacket('step_tick', {
  tickSteps: 1
}, 'Step tick'))

// Game event (thunder)
testPackets.push(generateTestPacket('game_state_change', {
  reason: 7, // Begin raining
  gameMode: 0.0
}, 'Game event - begin rain'))

// Set title text
testPackets.push(generateTestPacket('title', {
  action: 0, // Set title
  text: JSON.stringify({ text: 'Test Title' })
}, 'Set title'))

// Set compression threshold
testPackets.push(generateTestPacket('set_compression', {
  threshold: 256
}, 'Set compression threshold'))

// Tab complete
testPackets.push(generateTestPacket('tab_complete', {
  transactionId: 1,
  start: 0,
  length: 0,
  matches: []
}, 'Tab complete - no matches'))

// NBT query response
testPackets.push(generateTestPacket('nbt_query_response', {
  transactionId: 1,
  nbt: null
}, 'NBT query response - null'))

// Resource pack send (1.21.5 uses add_resource_pack)
testPackets.push(generateTestPacket('add_resource_pack', {
  uuid: 'd3527a0b-bc03-45d5-a878-2aafdd8c8a43',
  url: 'https://example.com/pack.zip',
  hash: '',
  forced: false,
  promptMessage: undefined
}, 'Add resource pack'))

// Write output file
const outputPath = path.resolve(outputFile)
const outputStream = fs.createWriteStream(outputPath)

testPackets.filter(p => p !== null).forEach(packet => {
  outputStream.write(JSON.stringify(packet) + '\n')
})

outputStream.end()

console.log(`\nGenerated ${testPackets.filter(p => p !== null).length} test packets`)
console.log(`Output written to: ${outputPath}`)
console.log('\nUsage in Go validator:')
console.log('  go run ./cmd/packet-validation -ground-truth ' + outputFile)
