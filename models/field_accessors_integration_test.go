package models_test

import (
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/stretchr/testify/assert"

	"github.com/reallyoldfogie/mc-protocol-go/data/1.21.1/play/serverbound"
	v1_21_6_serverbound "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/serverbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// TestVersionAgnosticFieldAccess demonstrates using field accessor interfaces
// to work with packets across different protocol versions
func TestVersionAgnosticFieldAccess(t *testing.T) {
	// Create a version-agnostic function that works with any packet that has a Count field
	processCountPacket := func(pkt models.PacketMarshaller) int32 {
		// Count field returns pk.VarInt
		if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
			return int32(getter.GetCount())
		}
		return -1
	}

	// Test with 1.21.6 MessageAcknowledgement
	v1216Pkt := v1_21_6_serverbound.NewMessageAcknowledgement()
	v1216Pkt.SetCount(42)
	assert.Equal(t, int32(42), processCountPacket(v1216Pkt), "1.21.6 packet should return count")

	// Test with 1.21.1 - if MessageAcknowledgement exists in that version
	// This demonstrates the same code works across versions
	v1211Pkt := serverbound.NewEntityAction()
	// EntityAction doesn't have Count field, should return -1
	assert.Equal(t, int32(-1), processCountPacket(v1211Pkt), "EntityAction should not have Count field")
}

// TestInterfaceImplementation verifies packets implement the correct interfaces
func TestInterfaceImplementation(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Verify it implements CountGetter
	var _ models.CountGetter[pk.VarInt] = pkt
	// Verify it implements CountSetter
	var _ models.CountSetter[pk.VarInt] = pkt

	// Use the interface methods
	pkt.SetCount(100)
	assert.Equal(t, pk.VarInt(100), pkt.GetCount())

	// Verify via interface
	if getter, ok := any(pkt).(models.CountGetter[pk.VarInt]); ok {
		assert.Equal(t, pk.VarInt(100), getter.GetCount())
	} else {
		t.Fatal("MessageAcknowledgement should implement CountGetter")
	}
}

// TestMultipleFields demonstrates checking for multiple fields
func TestMultipleFields(t *testing.T) {
	// Demonstrate checking for multiple interface implementations
	pkt := v1_21_6_serverbound.NewQueryEntityNbt()

	// This packet has both TransactionId and EntityId fields
	var hasTransactionId, hasEntityId bool

	if _, ok := any(pkt).(models.TransactionIdGetter[pk.VarInt]); ok {
		hasTransactionId = true
	}
	if _, ok := any(pkt).(models.EntityIdGetter[pk.VarInt]); ok {
		hasEntityId = true
	}

	assert.True(t, hasTransactionId, "QueryEntityNbt should have TransactionId")
	assert.True(t, hasEntityId, "QueryEntityNbt should have EntityId")

	// Test with a packet that doesn't have these fields
	pkt2 := serverbound.NewEntityAction()
	_, ok := any(pkt2).(models.TransactionIdGetter[pk.VarInt])
	assert.False(t, ok, "EntityAction should not have TransactionId")
}

// TestMissingFieldGraceful verifies that packets without a field handle it gracefully
func TestMissingFieldGraceful(t *testing.T) {
	pkt := v1_21_6_serverbound.NewLockDifficulty()

	// LockDifficulty doesn't have Count field
	_, ok := any(pkt).(models.CountGetter[pk.VarInt])
	assert.False(t, ok, "LockDifficulty should not implement CountGetter")

	// This is the safe pattern - check before using
	if getter, ok := any(pkt).(models.CountGetter[pk.VarInt]); ok {
		_ = getter.GetCount()
		t.Fatal("Should not reach here")
	}
}

// TestSetterInterface demonstrates modifying fields via interface
func TestSetterInterface(t *testing.T) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	// Function that modifies any packet with a Count field
	incrementCount := func(p models.PacketMarshaller) bool {
		getter, hasGetter := p.(models.CountGetter[pk.VarInt])
		setter, hasSetter := p.(models.CountSetter[pk.VarInt])

		if hasGetter && hasSetter {
			currentValue := getter.GetCount()
			setter.SetCount(currentValue + 1)
			return true
		}
		return false
	}

	// Set initial value
	pkt.SetCount(10)

	// Increment it
	modified := incrementCount(pkt)
	assert.True(t, modified, "Should have modified the packet")
	assert.Equal(t, pk.VarInt(11), pkt.GetCount(), "Count should be incremented")

	// Try again
	incrementCount(pkt)
	assert.Equal(t, pk.VarInt(12), pkt.GetCount(), "Count should be incremented again")
}
