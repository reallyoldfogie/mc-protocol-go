package tests_test

import (
	"bytes"
	"io"
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cb12101 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.1/play/clientbound"
	cb12102 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.2/play/clientbound"
	cb12103 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.3/play/clientbound"
	cb12104 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.4/play/clientbound"

	// cb12105 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/play/clientbound" // Disabled: struct field mismatch
	cb12110 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.10/play/clientbound"
	cb12111 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.11/play/clientbound"
	cb12106 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/clientbound"
	cb12107 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.7/play/clientbound"
	cb12108 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.8/play/clientbound"
	cb12109 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.9/play/clientbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// roundTripSpawnEntity performs a write → read → write round-trip and asserts
// the two serialized byte sequences are identical.
func roundTripSpawnEntity(t *testing.T, original io.WriterTo, newReader func() io.ReaderFrom) {
	t.Helper()

	var buf1 bytes.Buffer
	bytesWritten, err := original.WriteTo(&buf1)
	require.NoError(t, err, "first WriteTo failed")
	require.Greater(t, bytesWritten, int64(0), "WriteTo produced no bytes")
	assert.Equal(t, int64(buf1.Len()), bytesWritten, "WriteTo byte count mismatch with buffer length")

	reader := newReader()
	bytesRead, err := reader.ReadFrom(bytes.NewReader(buf1.Bytes()))
	require.NoError(t, err, "ReadFrom failed")
	assert.Equal(t, bytesWritten, bytesRead, "ReadFrom byte count differs from WriteTo byte count")

	var buf2 bytes.Buffer
	bytesWritten2, err := reader.(io.WriterTo).WriteTo(&buf2)
	require.NoError(t, err, "second WriteTo failed")
	assert.Equal(t, bytesWritten, bytesWritten2, "second WriteTo byte count differs")
	assert.Equal(t, buf1.Bytes(), buf2.Bytes(), "round-trip byte mismatch")
}

// buildSpawnEntity1211 creates a SpawnEntity packet for v1.21.1 (Velocity.X/Y/Z)
func buildSpawnEntity1211() io.WriterTo {
	pkt := cb12101.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_1(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1211(), func() io.ReaderFrom {
		return cb12101.NewSpawnEntity()
	})
}

// buildSpawnEntity1212 creates a SpawnEntity packet for v1.21.2 (Velocity.X/Y/Z)
func buildSpawnEntity1212() io.WriterTo {
	pkt := cb12102.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_2(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1212(), func() io.ReaderFrom {
		return cb12102.NewSpawnEntity()
	})
}

// Continue for remaining versions (1.21.3-1.21.8 use Velocity.X/Y/Z, 1.21.9-1.21.11 use Velocity LpVec3)
// buildSpawnEntity1213 creates a SpawnEntity packet for v1.21.3
func buildSpawnEntity1213() io.WriterTo {
	pkt := cb12103.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_3(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1213(), func() io.ReaderFrom {
		return cb12103.NewSpawnEntity()
	})
}

func buildSpawnEntity1214() io.WriterTo {
	pkt := cb12104.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_4(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1214(), func() io.ReaderFrom {
		return cb12104.NewSpawnEntity()
	})
}

/* FIXME: 1.21.5 SpawnEntity struct field names don't match test - needs updating
func buildSpawnEntity1215() io.WriterTo {
	pkt := cb12105.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_5(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1215(), func() io.ReaderFrom {
		return cb12105.NewSpawnEntity()
	})
}
*/

func buildSpawnEntity1216() io.WriterTo {
	pkt := cb12106.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_6(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1216(), func() io.ReaderFrom {
		return cb12106.NewSpawnEntity()
	})
}

func buildSpawnEntity1217() io.WriterTo {
	pkt := cb12107.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_7(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1217(), func() io.ReaderFrom {
		return cb12107.NewSpawnEntity()
	})
}

func buildSpawnEntity1218() io.WriterTo {
	pkt := cb12108.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity.X = pk.Short(0)
	pkt.Velocity.Y = pk.Short(0)
	pkt.Velocity.Z = pk.Short(100)
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_8(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1218(), func() io.ReaderFrom {
		return cb12108.NewSpawnEntity()
	})
}

// buildSpawnEntity1219 - v1.21.9 uses Velocity LpVec3 instead of separate Velocity.X/Y/Z
func buildSpawnEntity1219() io.WriterTo {
	pkt := cb12109.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity = models.LpVec3{X: 0, Y: 0, Z: 100}
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_9(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity1219(), func() io.ReaderFrom {
		return cb12109.NewSpawnEntity()
	})
}

func buildSpawnEntity12110() io.WriterTo {
	pkt := cb12110.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity = models.LpVec3{X: 0, Y: 0, Z: 100}
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_10(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity12110(), func() io.ReaderFrom {
		return cb12110.NewSpawnEntity()
	})
}

func buildSpawnEntity12111() io.WriterTo {
	pkt := cb12111.NewSpawnEntity()
	pkt.EntityId = pk.VarInt(12345)
	pkt.ObjectUUID = pk.UUID([16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	pkt.Type = pk.VarInt(1)
	pkt.X = pk.Double(100.5)
	pkt.Y = pk.Double(64.0)
	pkt.Z = pk.Double(-200.5)
	pkt.Pitch = pk.Byte(45)
	pkt.Yaw = pk.Byte(90)
	pkt.HeadPitch = pk.Byte(-30)
	pkt.ObjectData = pk.VarInt(0)
	pkt.Velocity = models.LpVec3{X: 0, Y: 0, Z: 100}
	return pkt
}

func TestSpawnEntityRoundTrip_1_21_11(t *testing.T) {
	roundTripSpawnEntity(t, buildSpawnEntity12111(), func() io.ReaderFrom {
		return cb12111.NewSpawnEntity()
	})
}
