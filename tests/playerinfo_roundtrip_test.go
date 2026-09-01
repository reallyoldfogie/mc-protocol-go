package tests_test

import (
	"bytes"
	"io"
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reallyoldfogie/mc-protocol-go/models"

	cb1211 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.1/play/clientbound"
	cb12110 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.10/play/clientbound"
	cb12111 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.11/play/clientbound"
	cb1212 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.2/play/clientbound"
	cb1213 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.3/play/clientbound"
	cb1214 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.4/play/clientbound"
	cb1215 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/play/clientbound"
	cb1216 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/clientbound"
	cb1217 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.7/play/clientbound"
	cb1218 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.8/play/clientbound"
	cb1219 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.9/play/clientbound"
)

var testUUID = pk.UUID(uuid.MustParse("12345678-1234-1234-1234-123456789abc"))

// roundTripPlayerInfo performs a write → read → write round-trip and asserts
// the two serialized byte sequences are identical.
func roundTripPlayerInfo(t *testing.T, original io.WriterTo, newReader func() io.ReaderFrom) {
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

// newParentContext6 builds a parent context for the 6-flag versions (1.21.1).
func newParentContext6(
	addPlayer, initChat, updateGameMode, updateListed, updateLatency, updateDisplayName bool,
) *models.SimpleParentContext {
	ctx := models.NewParentContext()
	ctx.SetField("action/add_player", addPlayer)
	ctx.SetField("action/initialize_chat", initChat)
	ctx.SetField("action/update_game_mode", updateGameMode)
	ctx.SetField("action/update_listed", updateListed)
	ctx.SetField("action/update_latency", updateLatency)
	ctx.SetField("action/update_display_name", updateDisplayName)
	return ctx
}

// newParentContext7 builds a parent context for the 7-flag versions (1.21.2–1.21.6).
func newParentContext7(
	addPlayer, initChat, updateGameMode, updateListed, updateLatency, updateDisplayName, updateListOrder bool,
) *models.SimpleParentContext {
	ctx := newParentContext6(addPlayer, initChat, updateGameMode, updateListed, updateLatency, updateDisplayName)
	ctx.SetField("action/update_list_order", updateListOrder)
	return ctx
}

// newParentContext8 builds a parent context for the 8-flag versions (1.21.7–1.21.11).
func newParentContext8(
	addPlayer, initChat, updateGameMode, updateListed, updateLatency, updateDisplayName, updateHat, updateListOrder bool,
) *models.SimpleParentContext {
	ctx := newParentContext7(addPlayer, initChat, updateGameMode, updateListed, updateLatency, updateDisplayName, updateListOrder)
	ctx.SetField("action/update_hat", updateHat)
	return ctx
}

// --- 1.21.1 (6 flags) ---

func buildPlayerInfo1211() *cb1211.PlayerInfo {
	pkt := cb1211.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1211.PlayerInfoDataArrayType{
		Uuid:        testUUID,
		Player:      &models.Void{},
		ChatSession: &models.Void{},
		Gamemode:    &gamemode,
		Listed:      &listed,
		Latency:     &latency,
		DisplayName: &models.Void{},
	}
	pkt.Data.Set([]cb1211.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext6(false, false, true, true, true, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_1(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1211(), func() io.ReaderFrom {
		return cb1211.NewPlayerInfo()
	})
}

// --- 1.21.2–1.21.6 (7 flags) ---

func buildPlayerInfo1212() *cb1212.PlayerInfo {
	pkt := cb1212.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1212.PlayerInfoDataArrayType{
		Uuid:        testUUID,
		Player:      &models.Void{},
		ChatSession: &models.Void{},
		Gamemode:    &gamemode,
		Listed:      &listed,
		Latency:     &latency,
		DisplayName: &models.Void{},
		// ListPriority: &models.Void{},
	}
	pkt.Data.Set([]cb1212.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext7(false, false, true, true, true, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_2(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1212(), func() io.ReaderFrom {
		return cb1212.NewPlayerInfo()
	})
}

func buildPlayerInfo1213() *cb1213.PlayerInfo {
	pkt := cb1213.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1213.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
	}
	pkt.Data.Set([]cb1213.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext7(false, false, true, true, true, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_3(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1213(), func() io.ReaderFrom {
		return cb1213.NewPlayerInfo()
	})
}

func buildPlayerInfo1214() *cb1214.PlayerInfo {
	pkt := cb1214.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1214.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1214.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_4(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1214(), func() io.ReaderFrom {
		return cb1214.NewPlayerInfo()
	})
}

func buildPlayerInfo1215() *cb1215.PlayerInfo {
	pkt := cb1215.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1215.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1215.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_5(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1215(), func() io.ReaderFrom {
		return cb1215.NewPlayerInfo()
	})
}

func buildPlayerInfo1216() *cb1216.PlayerInfo {
	pkt := cb1216.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1216.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1216.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_6(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1216(), func() io.ReaderFrom {
		return cb1216.NewPlayerInfo()
	})
}

// --- 1.21.4–1.21.6 use 8 flags too (same as 1.21.7–1.21.11) ---

func buildPlayerInfo1217() *cb1217.PlayerInfo {
	pkt := cb1217.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1217.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1217.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_7(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1217(), func() io.ReaderFrom {
		return cb1217.NewPlayerInfo()
	})
}

func buildPlayerInfo1218() *cb1218.PlayerInfo {
	pkt := cb1218.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1218.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1218.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_8(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1218(), func() io.ReaderFrom {
		return cb1218.NewPlayerInfo()
	})
}

func buildPlayerInfo1219() *cb1219.PlayerInfo {
	pkt := cb1219.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb1219.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb1219.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_9(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo1219(), func() io.ReaderFrom {
		return cb1219.NewPlayerInfo()
	})
}

func buildPlayerInfo12110() *cb12110.PlayerInfo {
	pkt := cb12110.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb12110.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb12110.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_10(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo12110(), func() io.ReaderFrom {
		return cb12110.NewPlayerInfo()
	})
}

func buildPlayerInfo12111() *cb12111.PlayerInfo {
	pkt := cb12111.NewPlayerInfo()

	pkt.Action.SetUpdateGameMode(true)
	pkt.Action.SetUpdateListed(true)
	pkt.Action.SetUpdateLatency(true)

	gamemode := pk.VarInt(1)
	listed := pk.VarInt(1)
	latency := pk.VarInt(42)

	entry := cb12111.PlayerInfoDataArrayType{
		Uuid:         testUUID,
		Player:       &models.Void{},
		ChatSession:  &models.Void{},
		Gamemode:     &gamemode,
		Listed:       &listed,
		Latency:      &latency,
		DisplayName:  &models.Void{},
		ListPriority: &models.Void{},
		ShowHat:      &models.Void{},
	}
	pkt.Data.Set([]cb12111.PlayerInfoDataArrayType{entry})
	pkt.Data.SetParentContext(newParentContext8(false, false, true, true, true, false, false, false))

	return pkt
}

func TestPlayerInfoRoundTrip_1_21_11(t *testing.T) {
	roundTripPlayerInfo(t, buildPlayerInfo12111(), func() io.ReaderFrom {
		return cb12111.NewPlayerInfo()
	})
}
