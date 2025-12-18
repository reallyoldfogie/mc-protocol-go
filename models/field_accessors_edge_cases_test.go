package models_test

import (
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1_21_6_clientbound "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/clientbound"
	v1_21_6_serverbound "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/serverbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// TestNilPacketHandling verifies graceful handling of nil packets
func TestNilPacketHandling(t *testing.T) {
	var pkt models.PacketMarshaller

	// Should not panic, just return false
	_, ok := pkt.(models.CountGetter[pk.VarInt])
	assert.False(t, ok, "nil packet should not implement any interface")
}

// TestZeroValues verifies correct handling of zero-value fields
func TestZeroValues(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Zero value should work
	assert.Equal(t, pk.VarInt(0), pkt.GetCount(), "Should return zero value")

	// Set to zero explicitly
	pkt.SetCount(0)
	assert.Equal(t, pk.VarInt(0), pkt.GetCount(), "Should handle zero value")
}

// TestNegativeValues verifies handling of negative values
func TestNegativeValues(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// VarInt can be negative
	pkt.SetCount(-42)
	assert.Equal(t, pk.VarInt(-42), pkt.GetCount(), "Should handle negative values")
}

// TestMaxValues verifies handling of maximum values
func TestMaxValues(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Test maximum VarInt value
	maxVarInt := pk.VarInt(2147483647) // Max int32
	pkt.SetCount(maxVarInt)
	assert.Equal(t, maxVarInt, pkt.GetCount(), "Should handle max values")
}

// TestChainedOperations verifies multiple operations work correctly
func TestChainedOperations(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Chain multiple sets
	pkt.SetCount(10)
	pkt.SetCount(20)
	pkt.SetCount(30)
	assert.Equal(t, pk.VarInt(30), pkt.GetCount(), "Should handle chained operations")

	// Get multiple times shouldn't change value
	v1 := pkt.GetCount()
	v2 := pkt.GetCount()
	v3 := pkt.GetCount()
	assert.Equal(t, v1, v2)
	assert.Equal(t, v2, v3)
}

// TestInterfaceConsistency verifies getter/setter consistency
func TestInterfaceConsistency(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Cast to interfaces
	getter, okGet := interface{}(pkt).(models.CountGetter[pk.VarInt])
	setter, okSet := interface{}(pkt).(models.CountSetter[pk.VarInt])

	require.True(t, okGet, "Should implement CountGetter")
	require.True(t, okSet, "Should implement CountSetter")

	// Set via interface
	setter.SetCount(123)

	// Get via interface should return same value
	assert.Equal(t, pk.VarInt(123), getter.GetCount())

	// Get via direct method should also return same value
	assert.Equal(t, pk.VarInt(123), pkt.GetCount())
}

// TestMultipleInterfacesOnSamePacket verifies a packet can implement many interfaces
func TestMultipleInterfacesOnSamePacket(t *testing.T) {
	// QueryEntityNbt has multiple fields (TransactionId and EntityId)
	pkt := v1_21_6_serverbound.NewQueryEntityNbt()

	// Should implement multiple interfaces
	var implementedCount int

	if _, ok := interface{}(pkt).(models.TransactionIdGetter[pk.VarInt]); ok {
		implementedCount++
	}
	if _, ok := interface{}(pkt).(models.TransactionIdSetter[pk.VarInt]); ok {
		implementedCount++
	}
	if _, ok := interface{}(pkt).(models.EntityIdGetter[pk.VarInt]); ok {
		implementedCount++
	}
	if _, ok := interface{}(pkt).(models.EntityIdSetter[pk.VarInt]); ok {
		implementedCount++
	}

	// QueryEntityNbt has 2 fields, so should have at least 2 interfaces (may have all 4 if both getter and setter)
	assert.GreaterOrEqual(t, implementedCount, 2, "Should implement at least 2 interfaces (one per field)")
}

// TestDifferentFieldTypes verifies handling of different pk types
func TestDifferentFieldTypes(t *testing.T) {
	// Test with different field types across different packets

	// Boolean field
	pkt1 := v1_21_6_serverbound.NewLockDifficulty()
	if setter, ok := interface{}(pkt1).(models.LockedSetter[pk.Boolean]); ok {
		setter.SetLocked(true)
		if getter, ok := interface{}(pkt1).(models.LockedGetter[pk.Boolean]); ok {
			// Note: May return 'any' type, so we just verify it doesn't panic
			_ = getter.GetLocked()
		}
	}
}

// TestPacketIDPreserved verifies PacketID is preserved with field access
func TestPacketIDPreserved(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()
	originalID := pkt.PacketID()

	// Modify fields
	pkt.SetCount(999)

	// PacketID should remain unchanged
	assert.Equal(t, originalID, pkt.PacketID(), "PacketID should not change")
}

// TestInterfaceTypeAssertion verifies type assertions work correctly
func TestInterfaceTypeAssertion(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Type assertion to interface should work
	var marshaller models.PacketMarshaller = pkt
	getter, ok := marshaller.(models.CountGetter[pk.VarInt])
	require.True(t, ok, "Type assertion should succeed")
	assert.NotNil(t, getter, "Getter should not be nil")
}

// TestConcurrentAccess verifies thread-safety of getter (not setter - setters aren't thread-safe)
func TestConcurrentAccess(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()
	pkt.SetCount(42)

	// Multiple goroutines reading should work fine
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			value := pkt.GetCount()
			assert.Equal(t, pk.VarInt(42), value)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestDifferentPacketTypes verifies pattern works across packet types
func TestDifferentPacketTypes(t *testing.T) {
	packets := []models.PacketMarshaller{
		v1_21_6_serverbound.NewMessageAcknowledgement(),
		v1_21_6_serverbound.NewQueryEntityNbt(),
		v1_21_6_serverbound.NewLockDifficulty(),
	}

	for _, pkt := range packets {
		// Each packet should have a valid PacketID
		assert.NotEqual(t, int32(0), pkt.PacketID(), "PacketID should be non-zero")

		// Try to access Count field (may or may not exist)
		if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
			_ = getter.GetCount() // Should not panic
		}
	}
}

// TestClientboundPackets verifies clientbound packets also work
func TestClientboundPackets(t *testing.T) {
	// Test with clientbound packet (if it has suitable fields)
	pkt := v1_21_6_clientbound.NewPing()

	// Should work with clientbound packets too
	assert.NotNil(t, pkt, "Clientbound packet should be created")
	assert.NotEqual(t, int32(0), pkt.PacketID(), "Should have valid packet ID")

	// Verify the packet has getter/setter methods
	// Ping has Id field of type pk.Int
	pkt.SetId(42)
	assert.Equal(t, pk.Int(42), pkt.GetId(), "Direct field access should work")

	// Note: Due to type differences (pk.Int vs pk.VarInt), Ping.Id may not match
	// the IdGetter interface if the interface was generated for pk.VarInt.
	// This is expected behavior when field types differ across versions.
}

// TestInterfaceNegativeCase verifies packets don't incorrectly implement interfaces
func TestInterfaceNegativeCase(t *testing.T) {
	// LockDifficulty should NOT implement CountGetter
	pkt := v1_21_6_serverbound.NewLockDifficulty()

	_, ok := interface{}(pkt).(models.CountGetter[pk.VarInt])
	assert.False(t, ok, "LockDifficulty should not implement CountGetter")
}

// TestFieldAccessConsistency verifies direct access matches interface access
func TestFieldAccessConsistency(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Set via direct method
	pkt.SetCount(555)

	// Get via direct method
	direct := pkt.GetCount()

	// Get via interface
	var interfaceValue pk.VarInt
	if getter, ok := interface{}(pkt).(models.CountGetter[pk.VarInt]); ok {
		interfaceValue = getter.GetCount()
	}

	// Should be identical
	assert.Equal(t, direct, interfaceValue, "Direct and interface access should match")
}

// TestDeprecatedAPIStillWorks verifies backward compatibility
func TestDeprecatedAPIStillWorks(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Old API (deprecated but should still work)
	fields := pkt.GetFields()
	assert.NotNil(t, fields, "GetFields should still work")

	// Should be able to get field via old API
	if countField, ok := fields["Count"]; ok {
		count := countField.(pk.VarInt)
		_ = count // Should work without panic
	}

	// Old SetFields API should still work
	pkt.SetFields(map[string]pk.FieldEncoder{
		"Count": pk.VarInt(777),
	})

	// Verify it was set
	assert.Equal(t, pk.VarInt(777), pkt.GetCount())
}
