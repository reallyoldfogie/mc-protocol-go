const mc = require('minecraft-protocol')

const version = '1.21.5'
const serializer = mc.createSerializer({ version, isServer: false })
const deserializer = mc.createDeserializer({ version, isServer: false })

console.log('Test 1: tickRate=20, isFrozen=false')
try {
  const buffer1 = serializer.createPacketBuffer({
    name: 'set_ticking_state',
    params: { tickRate: 20, isFrozen: false }
  })
  console.log('  Serialized:', buffer1.toString('hex'))
  console.log('  Buffer bytes:', Array.from(buffer1))

  const parsed1 = deserializer.parsePacketBuffer(buffer1)
  console.log('  Parsed:', JSON.stringify(parsed1.data))
} catch (err) {
  console.error('  Error:', err.message)
}

console.log('\nTest 2: tickRate=1.0, isFrozen=true')
try {
  const buffer2 = serializer.createPacketBuffer({
    name: 'set_ticking_state',
    params: { tickRate: 1.0, isFrozen: true }
  })
  console.log('  Serialized:', buffer2.toString('hex'))
  const parsed2 = deserializer.parsePacketBuffer(buffer2)
  console.log('  Parsed:', JSON.stringify(parsed2.data))
} catch (err) {
  console.error('  Error:', err.message)
}

// Decode the hex bytes manually
console.log('\nManual decode of 7FC00000:')
const testBuffer = Buffer.from('7fc00000', 'hex')
console.log('  As float32 (big-endian):', testBuffer.readFloatBE(0))
console.log('  As float32 (little-endian):', testBuffer.readFloatLE(0))

console.log('\nWhat should 20.0 be:')
const twentyBuffer = Buffer.allocUnsafe(4)
twentyBuffer.writeFloatBE(20.0, 0)
console.log('  20.0 as hex (big-endian):', twentyBuffer.toString('hex'))
