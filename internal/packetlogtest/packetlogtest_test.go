package packetlogtest

import (
	"context"
	"testing"
	"time"

	"github.com/reallyoldfogie/mc-protocol-go/data/versions"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// TestRunRoundTripChecks_SingleClientbound_smoke is a smoke test that ensures
// that RunRoundTripChecks can process at least one real clientbound packet
// for a known version using the generated PacketMgr.
func TestRunRoundTripChecks_SingleClientbound_smoke(t *testing.T) {
	// Use a simple, well-known clientbound packet that is easy to construct
	// and round-trip. We synthesize the PacketLog rather than depending on
	// external log files.

	mgr := versions.GetPacketMgrForVersion("1.21.5")
	if mgr == nil {
		t.Skip("no packet manager for version 1.21.5")
	}

	// Construct a real clientbound Difficulty packet using the generator APIs.
	// This ensures we get a realistic wire-format payload.
	id := mgr.GetClientboundPacketID("ClientboundDifficulty")
	packet, err := mgr.GetClientboundPacketByID(id)
	if err != nil {
		t.Fatalf("GetClientboundPacketByID(ClientboundDifficulty) failed: %v", err)
	}

	wire := packet.Marshal()

	pl := &models.PacketLog{
		ID:              int32(wire.ID),
		Name:            "ClientboundDifficulty",
		Timestamp:       time.Now(),
		Data:            append([]byte(nil), wire.Data...),
		Version:         mgr.Name(),
		ProtocolVersion: uint(mgr.VersionProtocol()),
	}

	src := NewSliceSource([]*models.PacketLog{pl})

	ctx := context.Background()
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{StopOnFirstError: true})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned error: %v", err)
	}

	if summary.Total != 1 {
		t.Fatalf("expected Total=1, got %d", summary.Total)
	}
	if summary.Failed != 0 {
		t.Fatalf("expected Failed=0, got %d (errors=%v)", summary.Failed, summary.Errors)
	}
	if summary.Succeeded != 1 {
		t.Fatalf("expected Succeeded=1, got %d", summary.Succeeded)
	}
}

// TestRunRoundTripChecks_UnknownVersion ensures that unknown versions are
// handled gracefully and reported as failures.
func TestRunRoundTripChecks_UnknownVersion(t *testing.T) {
	pl := &models.PacketLog{
		ID:              10,
		Name:            "ClientboundDifficulty",
		Timestamp:       time.Now(),
		Data:            []byte{0x00, 0x00},
		Version:         "999.999.999", // Unknown version
		ProtocolVersion: 0,
	}

	src := NewSliceSource([]*models.PacketLog{pl})

	ctx := context.Background()
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{StopOnFirstError: false})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned unexpected error: %v", err)
	}

	if summary.Total != 1 {
		t.Fatalf("expected Total=1, got %d", summary.Total)
	}
	if summary.Failed != 1 {
		t.Fatalf("expected Failed=1 for unknown version, got %d", summary.Failed)
	}
	if summary.Succeeded != 0 {
		t.Fatalf("expected Succeeded=0, got %d", summary.Succeeded)
	}
	if len(summary.Errors) == 0 {
		t.Fatal("expected at least one error for unknown version")
	}
	if summary.Errors[0].Stage != "packetmgr" {
		t.Fatalf("expected error stage 'packetmgr', got %q", summary.Errors[0].Stage)
	}
}

// TestRunRoundTripChecks_UnknownDirection ensures that packets with unknown
// direction prefixes are handled gracefully.
func TestRunRoundTripChecks_UnknownDirection(t *testing.T) {
	pl := &models.PacketLog{
		ID:              10,
		Name:            "UnknownDirectionPacket", // No Clientbound/Serverbound prefix
		Timestamp:       time.Now(),
		Data:            []byte{0x00, 0x00},
		Version:         "1.21.5",
		ProtocolVersion: 770,
	}

	src := NewSliceSource([]*models.PacketLog{pl})

	ctx := context.Background()
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{StopOnFirstError: false})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned unexpected error: %v", err)
	}

	if summary.Total != 1 {
		t.Fatalf("expected Total=1, got %d", summary.Total)
	}
	if summary.Failed != 1 {
		t.Fatalf("expected Failed=1 for unknown direction, got %d", summary.Failed)
	}
	if len(summary.Errors) == 0 {
		t.Fatal("expected at least one error for unknown direction")
	}
	if summary.Errors[0].Stage != "direction" {
		t.Fatalf("expected error stage 'direction', got %q", summary.Errors[0].Stage)
	}
}

// TestRunRoundTripChecks_UnknownPacketName ensures that unknown packet names
// are handled gracefully.
func TestRunRoundTripChecks_UnknownPacketName(t *testing.T) {
	pl := &models.PacketLog{
		ID:              999,
		Name:            "ClientboundNonexistentPacket", // Unknown packet
		Timestamp:       time.Now(),
		Data:            []byte{0x00, 0x00},
		Version:         "1.21.5",
		ProtocolVersion: 770,
	}

	src := NewSliceSource([]*models.PacketLog{pl})

	ctx := context.Background()
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{StopOnFirstError: false})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned unexpected error: %v", err)
	}

	if summary.Total != 1 {
		t.Fatalf("expected Total=1, got %d", summary.Total)
	}
	if summary.Failed != 1 {
		t.Fatalf("expected Failed=1 for unknown packet name, got %d", summary.Failed)
	}
	if len(summary.Errors) == 0 {
		t.Fatal("expected at least one error for unknown packet name")
	}
	if summary.Errors[0].Stage != "roundtrip" {
		t.Fatalf("expected error stage 'roundtrip', got %q", summary.Errors[0].Stage)
	}
}

// TestRunRoundTripChecks_FilterByVersion ensures that version filtering works.
func TestRunRoundTripChecks_FilterByVersion(t *testing.T) {
	mgr := versions.GetPacketMgrForVersion("1.21.5")
	if mgr == nil {
		t.Skip("no packet manager for version 1.21.5")
	}

	id := mgr.GetClientboundPacketID("ClientboundDifficulty")
	packet, err := mgr.GetClientboundPacketByID(id)
	if err != nil {
		t.Fatalf("GetClientboundPacketByID failed: %v", err)
	}

	wire := packet.Marshal()

	pl1 := &models.PacketLog{
		ID:              int32(wire.ID),
		Name:            "ClientboundDifficulty",
		Timestamp:       time.Now(),
		Data:            append([]byte(nil), wire.Data...),
		Version:         "1.21.5",
		ProtocolVersion: uint(mgr.VersionProtocol()),
	}
	pl2 := &models.PacketLog{
		ID:              int32(wire.ID),
		Name:            "ClientboundDifficulty",
		Timestamp:       time.Now(),
		Data:            append([]byte(nil), wire.Data...),
		Version:         "1.21.1", // Different version
		ProtocolVersion: 767,
	}

	src := NewSliceSource([]*models.PacketLog{pl1, pl2})

	ctx := context.Background()
	// Filter to only 1.21.5
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{
		Versions:         []string{"1.21.5"},
		StopOnFirstError: false,
	})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned unexpected error: %v", err)
	}

	if summary.Total != 2 {
		t.Fatalf("expected Total=2, got %d", summary.Total)
	}
	if summary.Skipped != 1 {
		t.Fatalf("expected Skipped=1, got %d (should skip 1.21.1)", summary.Skipped)
	}
	if summary.Succeeded != 1 {
		t.Fatalf("expected Succeeded=1, got %d", summary.Succeeded)
	}
}

// TestRunRoundTripChecks_MaxPackets ensures MaxPackets limit is respected.
func TestRunRoundTripChecks_MaxPackets(t *testing.T) {
	mgr := versions.GetPacketMgrForVersion("1.21.5")
	if mgr == nil {
		t.Skip("no packet manager for version 1.21.5")
	}

	id := mgr.GetClientboundPacketID("ClientboundDifficulty")
	packet, err := mgr.GetClientboundPacketByID(id)
	if err != nil {
		t.Fatalf("GetClientboundPacketByID failed: %v", err)
	}

	wire := packet.Marshal()

	// Create 10 identical packets
	logs := make([]*models.PacketLog, 10)
	for i := range logs {
		logs[i] = &models.PacketLog{
			ID:              int32(wire.ID),
			Name:            "ClientboundDifficulty",
			Timestamp:       time.Now(),
			Data:            append([]byte(nil), wire.Data...),
			Version:         mgr.Name(),
			ProtocolVersion: uint(mgr.VersionProtocol()),
		}
	}

	src := NewSliceSource(logs)

	ctx := context.Background()
	summary, err := RunRoundTripChecks(ctx, src, RoundTripOptions{
		MaxPackets:       3, // Limit to 3
		StopOnFirstError: false,
	})
	if err != nil {
		t.Fatalf("RunRoundTripChecks returned unexpected error: %v", err)
	}

	if summary.Total != 3 {
		t.Fatalf("expected Total=3 (limited by MaxPackets), got %d", summary.Total)
	}
	if summary.Succeeded != 3 {
		t.Fatalf("expected Succeeded=3, got %d", summary.Succeeded)
	}
}
