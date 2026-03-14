package tests_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cb12110 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.10/play/clientbound"
)

// TestPlayerInfoMCPR_Packet37_Parse tests parsing packet #37 from WaterDiagonalBot replay.
// Packet #37 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 962ms, Length: 23 bytes
// Hex: 1d 01 10 57 61 74 65 72 44 69 61 67 6f 6e 61 6c 42 6f 74 00 00 01 00
func TestPlayerInfoMCPR_Packet37_Parse(t *testing.T) {
	base64Data := "HQEQV2F0ZXJEaWFnb25hbEJvdAAAAQA="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (first byte: 0x1D = 0b00011101)
	assert.True(t, pkt.Action.AddPlayer())
	assert.False(t, pkt.Action.InitializeChat())
	assert.True(t, pkt.Action.UpdateGameMode())
	assert.True(t, pkt.Action.UpdateListed())
	assert.True(t, pkt.Action.UpdateLatency())
	assert.False(t, pkt.Action.UpdateDisplayName())
	assert.False(t, pkt.Action.UpdateHat())
	assert.False(t, pkt.Action.UpdateListOrder())

	// Verify data array length (0x01 = 1 entry)
	data := pkt.Data.Get()
	require.Equal(t, 1, len(data))

	// Verify UUID
	entry := data[0]
	expectedUUID := uuid.MustParse("3323a5ae-4fcd-3ca1-9f7d-292e374db857")
	assert.Equal(t, expectedUUID, uuid.UUID(entry.Uuid))
}

// TestPlayerInfoMCPR_Packet40_Parse tests parsing packet #40 from WaterDiagonalBot replay.
// Packet #40 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 962ms, Length: 3 bytes
// Hex: ff 00
func TestPlayerInfoMCPR_Packet40_Parse(t *testing.T) {
	base64Data := "/wA="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	assert.True(t, pkt.Action.AddPlayer())
	assert.True(t, pkt.Action.InitializeChat())
	assert.True(t, pkt.Action.UpdateGameMode())
	assert.True(t, pkt.Action.UpdateListed())
	assert.True(t, pkt.Action.UpdateLatency())
	assert.True(t, pkt.Action.UpdateDisplayName())
	assert.True(t, pkt.Action.UpdateHat())
	assert.True(t, pkt.Action.UpdateListOrder())

	// Verify data array is empty (0x00 = 0 entries)
	data := pkt.Data.Get()
	require.Equal(t, 0, len(data))
}

// TestPlayerInfoMCPR_Packet41_Parse tests parsing packet #41 from WaterDiagonalBot replay.
// Packet #41 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 965ms, Length: 44 bytes
// Hex: ff 01 33 23 a5 ae 4f cd 3c a1 9f 7d 29 2e 37 4d b8 57 10 57 61 74 65 72 44 69 61 67 6f 6e 61 6c 42 6f 74 00 00 00 01 00 00 00 00
func TestPlayerInfoMCPR_Packet41_Parse(t *testing.T) {
	base64Data := "/wEzI6WuT808oZ99KS43TbhXEFdhdGVyRGlhZ29uYWxCb3QAAAABAAAAAA=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	assert.True(t, pkt.Action.AddPlayer())

	// Verify data array length
	data := pkt.Data.Get()
	require.Equal(t, 1, len(data))

	// Verify UUID
	entry := data[0]
	expectedUUID := uuid.MustParse("3323a5ae-4fcd-3ca1-9f7d-292e374db857")
	assert.Equal(t, expectedUUID, uuid.UUID(entry.Uuid))
}

// TestPlayerInfoMCPR_Packet50_Parse tests parsing packet #50 from WaterDiagonalBot replay.
// Packet #50 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 974ms, Length: 20 bytes
// Hex: 80 01 33 23 a5 ae 4f cd 3c a1 9f 7d 29 2e 37 4d b8 57 01
func TestPlayerInfoMCPR_Packet50_Parse(t *testing.T) {
	base64Data := "gAEzI6WuT808oZ99KS43TbhXAQ=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0x80 = 0b10000000 = only update_list_order set)
	assert.False(t, pkt.Action.AddPlayer())
	assert.False(t, pkt.Action.InitializeChat())
	assert.False(t, pkt.Action.UpdateGameMode())
	assert.False(t, pkt.Action.UpdateListed())
	assert.False(t, pkt.Action.UpdateLatency())
	assert.False(t, pkt.Action.UpdateDisplayName())
	assert.False(t, pkt.Action.UpdateHat())
	assert.True(t, pkt.Action.UpdateListOrder())

	// Verify data array length
	data := pkt.Data.Get()
	require.Equal(t, 1, len(data))

	// Verify UUID
	entry := data[0]
	expectedUUID := uuid.MustParse("3323a5ae-4fcd-3ca1-9f7d-292e374db857")
	assert.Equal(t, expectedUUID, uuid.UUID(entry.Uuid))
}

// TestPlayerInfoMCPR_Packet51_Parse tests parsing packet #51 from WaterDiagonalBot replay.
// Packet #51 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 974ms, Length: 44 bytes
// Hex: ff 01 33 23 a5 ae 4f cd 3c a1 9f 7d 29 2e 37 4d b8 57 10 57 61 74 65 72 44 69 61 67 6f 6e 61 6c 42 6f 74 00 00 00 01 00 00 00 00
func TestPlayerInfoMCPR_Packet51_Parse(t *testing.T) {
	base64Data := "/wEzI6WuT808oZ99KS43TbhXEFdhdGVyRGlhZ29uYWxCb3QAAAABAAAAAA=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	assert.True(t, pkt.Action.AddPlayer())

	// Verify data array length
	data := pkt.Data.Get()
	require.Equal(t, 1, len(data))

	// Verify UUID
	entry := data[0]
	expectedUUID := uuid.MustParse("3323a5ae-4fcd-3ca1-9f7d-292e374db857")
	assert.Equal(t, expectedUUID, uuid.UUID(entry.Uuid))
}

// TestPlayerInfoMCPR_Packet85_Parse tests parsing packet #85 from WaterDiagonalBot replay.
// Packet #85 from WaterDiagonalBot_20260312_203321.mcpr
// Timestamp: 1107ms, Length: 44 bytes
// Hex: ff 01 33 23 a5 ae 4f cd 3c a1 9f 7d 29 2e 37 4d b8 57 10 57 61 74 65 72 44 69 61 67 6f 6e 61 6c 42 6f 74 00 00 00 01 00 00 00 00
func TestPlayerInfoMCPR_Packet85_Parse(t *testing.T) {
	base64Data := "/wEzI6WuT808oZ99KS43TbhXEFdhdGVyRGlhZ29uYWxCb3QAAAABAAAAAA=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	assert.True(t, pkt.Action.AddPlayer())

	// Verify data array length
	data := pkt.Data.Get()
	require.Equal(t, 1, len(data))

	// Verify UUID
	entry := data[0]
	expectedUUID := uuid.MustParse("3323a5ae-4fcd-3ca1-9f7d-292e374db857")
	assert.Equal(t, expectedUUID, uuid.UUID(entry.Uuid))
}
