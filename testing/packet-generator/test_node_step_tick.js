const mc = require('minecraft-protocol')

const version = '1.21.5'
const serializer = mc.createSerializer({ version, isServer: false })
const deserializer = mc.createDeserializer({ version, isServer: false })

try {
  // Serialize step_tick with tickSteps=1
  const buffer = serializer.createPacketBuffer({
    name: 'step_tick',
    params: { tickSteps: 1 }
  })

  console.log('Serialized buffer:', buffer.toString('hex'))
  console.log('Buffer bytes:', Array.from(buffer))

  // Try to deserialize it back
  const parsed = deserializer.parsePacketBuffer(buffer)
  console.log('Parsed back:', JSON.stringify(parsed.data, null, 2))
} catch (err) {
  console.error('Error:', err.message)
  console.error(err.stack)
}
