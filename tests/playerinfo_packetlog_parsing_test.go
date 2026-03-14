package tests_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	cb12110 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.10/play/clientbound"
)

// TestPlayerInfoPacketLog_Packet1_Parse tests parsing the first PlayerInfo packet from WaterDiagonalBot packet log.
// From WaterDiagonalBot_20260312_203321.log:
// data_length: 2, data: "/wA="
// This is a minimal PlayerInfo packet with action mask and empty player list.
func TestPlayerInfoPacketLog_Packet1_Parse(t *testing.T) {
	// Decode base64 from packet log
	base64Data := "/wA="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	require.True(t, pkt.Action.AddPlayer())
	require.True(t, pkt.Action.InitializeChat())
	require.True(t, pkt.Action.UpdateGameMode())
	require.True(t, pkt.Action.UpdateListed())
	require.True(t, pkt.Action.UpdateLatency())
	require.True(t, pkt.Action.UpdateDisplayName())
	require.True(t, pkt.Action.UpdateHat())
	require.True(t, pkt.Action.UpdateListOrder())

	// Verify data array is empty (0x00 = 0 entries)
	data := pkt.Data.Get()
	require.Equal(t, 0, len(data))
}

// TestPlayerInfoPacketLog_Packet2_Parse tests parsing the second PlayerInfo packet from WaterDiagonalBot packet log.
// From WaterDiagonalBot_20260312_203321.log:
// data_length: 43, data: " /wEzI6WuT808oZ99KS43TbhXEFdhdGVyRGlhZ29uYWxCb3QAAAABAAAAAA=="
// This packet contains an add_player action with player profile data.
func TestPlayerInfoPacketLog_Packet2_Parse(t *testing.T) {
	// Decode base64 from packet log
	base64Data := "/wEzI6WuT808oZ99KS43TbhXEFdhdGVyRGlhZ29uYWxCb3QAAAABAAAAAA=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0xFF = all actions enabled)
	require.True(t, pkt.Action.AddPlayer())

	// Verify data array has at least one entry
	data := pkt.Data.Get()
	require.Greater(t, len(data), 0, "should have at least one data entry")
}

// TestPlayerInfoPacketLog_Packet3_Parse tests parsing the third PlayerInfo packet from WaterDiagonalBot packet log.
// From WaterDiagonalBot_20260312_203321.log:
// data_length: 19, data: "gAEzI6WuT808oZ99KS43TbhXAQ=="
// This packet contains update actions for player data.
func TestPlayerInfoPacketLog_Packet3_Parse(t *testing.T) {
	// Decode base64 from packet log
	base64Data := "gAEzI6WuT808oZ99KS43TbhXAQ=="
	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	require.NoError(t, err, "failed to decode base64 data")

	pkt := cb12110.NewPlayerInfo()
	bytesRead, err := pkt.ReadFrom(bytes.NewReader(rawBytes))
	require.NoError(t, err, "failed to read PlayerInfo packet")
	require.Equal(t, int64(len(rawBytes)), bytesRead, "byte count mismatch")

	// Verify action bitflags (0x80 = only update_list_order set)
	require.False(t, pkt.Action.AddPlayer())
	require.False(t, pkt.Action.InitializeChat())
	require.False(t, pkt.Action.UpdateGameMode())
	require.False(t, pkt.Action.UpdateListed())
	require.False(t, pkt.Action.UpdateLatency())
	require.False(t, pkt.Action.UpdateDisplayName())
	require.False(t, pkt.Action.UpdateHat())
	require.True(t, pkt.Action.UpdateListOrder())

	// Verify data array has at least one entry
	data := pkt.Data.Get()
	require.Greater(t, len(data), 0, "should have at least one data entry")
}
