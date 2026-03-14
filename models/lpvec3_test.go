package models

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLpVec3_ZeroVectorEncoding tests that a zero vector is encoded as exactly 1 byte (0x00)
// Reference: Minecraft wiki "Single byte 0x00 when absolute maximum of all values is beneath 1.32766×10⁻⁴"
func TestLpVec3_ZeroVectorEncoding(t *testing.T) {
	v := &LpVec3{X: 0, Y: 0, Z: 0}
	buf := &bytes.Buffer{}

	n, err := v.WriteTo(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "zero vector should encode to exactly 1 byte")
	assert.Equal(t, []byte{0x00}, buf.Bytes(), "zero vector should encode as 0x00")
}

// TestLpVec3_ZeroVectorDecoding tests that 0x00 byte decodes to zero vector
func TestLpVec3_ZeroVectorDecoding(t *testing.T) {
	buf := bytes.NewReader([]byte{0x00})
	v := &LpVec3{}

	n, err := v.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "zero vector should consume exactly 1 byte")
	assert.Equal(t, 0.0, v.X)
	assert.Equal(t, 0.0, v.Y)
	assert.Equal(t, 0.0, v.Z)
}

// TestLpVec3_ZeroVectorDoesNotConsumeBeyond tests that reading a zero vector doesn't consume bytes beyond the zero vector
func TestLpVec3_ZeroVectorDoesNotConsumeBeyond(t *testing.T) {
	// Create a buffer with zero vector (0x00) followed by a marker byte
	data := []byte{0x00, 0xFF} // 0xFF should still be in the buffer after reading
	buf := bytes.NewReader(data)
	v := &LpVec3{}

	n, err := v.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "should read exactly 1 byte for zero vector")

	// Check that the next byte is still available
	remaining := make([]byte, 1)
	nRead, err := buf.Read(remaining)
	require.NoError(t, err)
	assert.Equal(t, 1, nRead)
	assert.Equal(t, byte(0xFF), remaining[0], "next byte should still be available")
}

// TestLpVec3_SpawnEntityVelocities tests encoding of velocity values from SpawnEntity packets
// This is the critical test case: dropped items via /give have zero velocity
func TestLpVec3_SpawnEntityVelocities(t *testing.T) {
	testCases := []struct {
		name     string
		vel      LpVec3
		maxDelta float64 // acceptable delta due to quantization and bit packing
	}{
		{
			name:     "dropped_item_no_velocity (critical fix)",
			vel:      LpVec3{X: 0, Y: 0, Z: 0},
			maxDelta: 0,
		},
		{
			name:     "dropped_item_slight_momentum",
			vel:      LpVec3{X: 0.05, Y: 0.01, Z: -0.02},
			maxDelta: 0.05, // Increased tolerance due to bit packing quantization
		},
		{
			name:     "arrow_initial_velocity",
			vel:      LpVec3{X: 0.5, Y: 0.0, Z: 0.5},
			maxDelta: 0.1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			n, err := tc.vel.WriteTo(buf)
			require.NoError(t, err)
			t.Logf("encoded to %d bytes", n)

			decoded := &LpVec3{}
			_, err = decoded.ReadFrom(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)

			assert.InDelta(t, tc.vel.X, decoded.X, tc.maxDelta, "X velocity should match")
			assert.InDelta(t, tc.vel.Y, decoded.Y, tc.maxDelta, "Y velocity should match")
			assert.InDelta(t, tc.vel.Z, decoded.Z, tc.maxDelta, "Z velocity should match")
		})
	}
}

// TestLpVec3_ByteCountConsistency ensures that bytes written equals bytes read for zero vectors
func TestLpVec3_ByteCountConsistency(t *testing.T) {
	vectors := []LpVec3{
		{0, 0, 0},                  // Zero vector - should encode to 1 byte
		{0.0002, 0.0001, -0.00015}, // Near threshold - should encode to 6 bytes
		{0.1, 0.2, 0.3},            // Normal values - should encode to 6 bytes
	}

	for i, v := range vectors {
		t.Run("vector_"+string(rune(i)), func(t *testing.T) {
			buf := &bytes.Buffer{}
			nWritten, err := v.WriteTo(buf)
			require.NoError(t, err)

			decoded := &LpVec3{}
			nRead, err := decoded.ReadFrom(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)

			assert.Equal(t, nWritten, nRead, "bytes written should equal bytes read")
		})
	}
}

// TestLpVec3_ZeroThreshold tests that vectors below the threshold are treated as zero
func TestLpVec3_ZeroThreshold(t *testing.T) {
	// The zero threshold is 1.32766e-4
	belowThreshold := LpVec3{X: 1e-5, Y: 2e-5, Z: -3e-5} // max = 3e-5 < 1.32766e-4
	aboveThreshold := LpVec3{X: 0.5, Y: 0.25, Z: -0.75}  // max = 0.75 > 1.32766e-4

	// Below threshold should encode as zero (1 byte)
	buf1 := &bytes.Buffer{}
	n1, err := belowThreshold.WriteTo(buf1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n1, "values below threshold should encode as zero vector (1 byte)")

	// Decode and verify it's zero
	decoded1 := &LpVec3{}
	_, err = decoded1.ReadFrom(bytes.NewReader(buf1.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, 0.0, decoded1.X)
	assert.Equal(t, 0.0, decoded1.Y)
	assert.Equal(t, 0.0, decoded1.Z)

	// Above threshold should encode as non-zero (6+ bytes, may have continuation flag)
	buf2 := &bytes.Buffer{}
	n2, err := aboveThreshold.WriteTo(buf2)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n2, int64(6), "values above threshold should encode as non-zero vector (6+ bytes)")

	// Decode and verify it's not zero
	decoded2 := &LpVec3{}
	_, err = decoded2.ReadFrom(bytes.NewReader(buf2.Bytes()))
	require.NoError(t, err)
	// Use InDelta to account for quantization
	assert.InDelta(t, aboveThreshold.X, decoded2.X, 0.1, "X should round-trip with quantization tolerance")
	assert.InDelta(t, aboveThreshold.Y, decoded2.Y, 0.1, "Y should round-trip with quantization tolerance")
	assert.InDelta(t, aboveThreshold.Z, decoded2.Z, 0.1, "Z should round-trip with quantization tolerance")
}

// TestLpVec3_SimpleNonZeroRoundTrip tests basic non-zero round-trip with loose tolerances to diagnose issues
func TestLpVec3_SimpleNonZeroRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		v         LpVec3
		tolerance float64
	}{
		{
			name:      "positive_single_value",
			v:         LpVec3{X: 1.0, Y: 0, Z: 0},
			tolerance: 0.1,
		},
		{
			name:      "negative_single_value",
			v:         LpVec3{X: -1.0, Y: 0, Z: 0},
			tolerance: 0.1,
		},
		{
			name:      "mixed_simple",
			v:         LpVec3{X: 1.0, Y: -1.0, Z: 0},
			tolerance: 0.1,
		},
		{
			name:      "all_positive",
			v:         LpVec3{X: 0.5, Y: 1.0, Z: 1.5},
			tolerance: 0.2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			n, err := tc.v.WriteTo(buf)
			require.NoError(t, err)
			t.Logf("Written %d bytes: %02x", n, buf.Bytes())

			decoded := &LpVec3{}
			nRead, err := decoded.ReadFrom(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)
			t.Logf("Read %d bytes", nRead)
			t.Logf("Original: X=%.6f, Y=%.6f, Z=%.6f", tc.v.X, tc.v.Y, tc.v.Z)
			t.Logf("Decoded:  X=%.6f, Y=%.6f, Z=%.6f", decoded.X, decoded.Y, decoded.Z)

			assert.InDelta(t, tc.v.X, decoded.X, tc.tolerance, "X should match")
			assert.InDelta(t, tc.v.Y, decoded.Y, tc.tolerance, "Y should match")
			assert.InDelta(t, tc.v.Z, decoded.Z, tc.tolerance, "Z should match")
		})
	}
}

// BenchmarkLpVec3_ReadWrite benchmarks the read/write performance
func BenchmarkLpVec3_ReadWrite(b *testing.B) {
	v := LpVec3{X: 1.5, Y: 2.5, Z: 3.5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		v.WriteTo(buf)
		decoded := &LpVec3{}
		decoded.ReadFrom(bytes.NewReader(buf.Bytes()))
	}
}
