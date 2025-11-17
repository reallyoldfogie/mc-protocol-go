package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitflagsSetFlagAndHasBit(t *testing.T) {
	var bf Bitflags = 0

	// Set bit 0
	bf.SetFlag(0, true)
	require.True(t, bf.HasBit(0))
	require.False(t, bf.HasBit(1))

	// Set bit 4
	bf.SetFlag(4, true)
	require.True(t, bf.HasBit(4))
	// Other bits unaffected
	require.False(t, bf.HasBit(2))

	// Clear bit 0
	bf.SetFlag(0, false)
	require.False(t, bf.HasBit(0))
	// Bit 4 remains set
	require.True(t, bf.HasBit(4))
}
