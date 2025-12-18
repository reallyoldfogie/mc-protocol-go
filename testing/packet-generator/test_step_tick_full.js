const mc = require('minecraft-protocol')

const version = '1.21.5'
const serializer = mc.createSerializer({ version, isServer: false })
const deserializer = mc.createDeserializer({ version, isServer: false })

console.log('Test 1: tickSteps=1')
try {
  const buffer1 = serializer.createPacketBuffer({
    name: 'step_tick',
    params: { tickSteps: 1 }
  })
  console.log('  Serialized:', buffer1.toString('hex'))
  const parsed1 = deserializer.parsePacketBuffer(buffer1)
  console.log('  Parsed:', JSON.stringify(parsed1.data))
} catch (err) {
  console.error('  Error:', err.message)
}

console.log('\nTest 2: tickSteps=0')
try {
  const buffer2 = serializer.createPacketBuffer({
    name: 'step_tick',
    params: { tickSteps: 0 }
  })
  console.log('  Serialized:', buffer2.toString('hex'))
  const parsed2 = deserializer.parsePacketBuffer(buffer2)
  console.log('  Parsed:', JSON.stringify(parsed2.data))
} catch (err) {
  console.error('  Error:', err.message)
}

console.log('\nTest 3: tickSteps=5')
try {
  const buffer3 = serializer.createPacketBuffer({
    name: 'step_tick',
    params: { tickSteps: 5 }
  })
  console.log('  Serialized:', buffer3.toString('hex'))
  const parsed3 = deserializer.parsePacketBuffer(buffer3)
  console.log('  Parsed:', JSON.stringify(parsed3.data))
} catch (err) {
  console.error('  Error:', err.message)
}
