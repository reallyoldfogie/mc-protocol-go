package models

import (
	"io"
	"math"

	pk "github.com/Tnze/go-mc/net/packet"
)

// LpVec3 is a compressed 3D vector format used in Minecraft 1.21.5+ for velocities.
// It encodes three doubles in typically just 1-7 bytes using bit packing and scale factors.
// Reference: https://minecraft.wiki/w/Java_Edition_protocol/Data_types#LpVec3
type LpVec3 struct {
	X, Y, Z float64
}

// ReadFrom reads and decodes an LpVec3 from the packet.
// Matches Java VelocityEncoding.readVelocity() exactly.
// Reference: Minecraft source, net.minecraft.network.encoding.VelocityEncoding
func (v *LpVec3) ReadFrom(r io.Reader) (n int64, err error) {
	// Read first byte to check for zero case
	// Protocol: single 0x00 byte encodes zero vector
	var b1 pk.Byte
	var nn int64
	if nn, err = b1.ReadFrom(r); err != nil {
		return 0, err
	}
	n += nn

	// Zero case: if first byte is 0x00, entire vector is zero
	// IMPORTANT: Must return here without reading more bytes to maintain packet alignment
	if b1 == 0 {
		v.X, v.Y, v.Z = 0, 0, 0
		return n, nil
	}

	// Non-zero case: read remaining 5 bytes (b1 already read)
	// In Java: readUnsignedByte() for b1 and b2, readUnsignedInt() for 4-byte int
	var b2, b3, b4, b5, b6 pk.Byte
	if nn, err = b2.ReadFrom(r); err != nil {
		return n, err
	}
	n += nn
	if nn, err = b3.ReadFrom(r); err != nil {
		return n, err
	}
	n += nn
	if nn, err = b4.ReadFrom(r); err != nil {
		return n, err
	}
	n += nn
	if nn, err = b5.ReadFrom(r); err != nil {
		return n, err
	}
	n += nn
	if nn, err = b6.ReadFrom(r); err != nil {
		return n, err
	}
	n += nn

	// Reconstruct the 48-bit long as in Java: m = (l << 16) | (j << 8) | i
	// where i = b1, j = b2, and l = bytes 3-6 as unsigned int (big-endian or little-endian depends on architecture)
	ub1, ub2 := uint8(b1), uint8(b2)
	ub3, ub4, ub5, ub6 := uint8(b3), uint8(b4), uint8(b5), uint8(b6)

	// Construct the 48-bit value (little-endian byte order for mc packet format)
	m := uint64(ub1) | (uint64(ub2) << 8) | (uint64(ub3) << 16) | (uint64(ub4) << 24) | (uint64(ub5) << 32) | (uint64(ub6) << 40)

	// Extract scale factor from bits 0-1 (and potentially 2-X if continuation flag)
	scaleFactor := int32(m & 0x03)
	if (m & 0x04) != 0 { // Fast marker bit set
		// Read additional scale bits as VarInt
		var extraScale pk.VarInt
		if nn2, err := extraScale.ReadFrom(r); err != nil {
			return n, err
		} else {
			n += nn2
		}
		scaleFactor |= int32(extraScale) << 2
	}

	// Extract packed values (15 bits each) using the denormalization bit positions
	// fromLong extracts bits, then multiply by scale factor
	packed1 := int64(m>>3) & 0x7FFF  // Extract bits 3-17
	packed2 := int64(m>>18) & 0x7FFF // Extract bits 18-32
	packed3 := int64(m>>33) & 0x7FFF // Extract bits 33-47

	// Convert packed 15-bit values to signed (-32766 to +32766)
	// using denormalization formula from Java: fromLong(value) = min(value & 32767, 32766) * 2 / 32766 - 1
	fromLong := func(l int64) float64 {
		if l&0x8000 != 0 {
			l = l & 0x7FFF
		}
		if l > 32766 {
			l = 32766
		}
		return float64(l)*2.0/32766.0 - 1.0
	}

	// Denormalize: multiply the denormalized value by the scale factor
	v.X = fromLong(packed1) * float64(scaleFactor)
	v.Y = fromLong(packed2) * float64(scaleFactor)
	v.Z = fromLong(packed3) * float64(scaleFactor)

	return n, nil
}

// WriteTo encodes the LpVec3 and writes it to the writer.
// Matches Java VelocityEncoding.writeVelocity() exactly.
// Reference: Minecraft source, net.minecraft.network.encoding.VelocityEncoding
func (v *LpVec3) WriteTo(w io.Writer) (n int64, err error) {
	// Compute max absolute value
	g := math.Abs(v.X)
	if math.Abs(v.Y) > g {
		g = math.Abs(v.Y)
	}
	if math.Abs(v.Z) > g {
		g = math.Abs(v.Z)
	}

	// Handle zero case: single 0x00 byte when max < 3.051944088384301E-5 (which is 1.32766e-4 / sqrt(2))
	// Reference: Java threshold is 3.051944088384301E-5
	if g < 3.051944088384301e-5 {
		b := pk.Byte(0)
		nn, err := b.WriteTo(w)
		if err != nil {
			return 0, err
		}
		return nn, nil
	}

	// Compute scale factor: ceil(max) - matches Java MathHelper.ceilLong(g)
	l := int64(math.Ceil(g))

	// Determine if we need continuation flag: set if scale factor doesn't fit in 2 bits
	// Java: boolean bl = (l & 3L) != l;
	bl := (l & 3) != l
	var m int64
	if bl {
		m = (l & 3) | 4 // Set fast marker bit (bit 2)
	} else {
		m = l // Scale factor fits in 2 bits
	}

	// Normalize values using Java's toLong formula
	// toLong(value) = round((value * 0.5 + 0.5) * 32766.0)
	toLong := func(value float64) int64 {
		return int64(math.Round((value*0.5 + 0.5) * 32766.0))
	}

	// Encode packed values with scale factor baked in
	// n = toLong(d / l) << 3  (bits 3-17)
	n_shifted := toLong(v.X/float64(l)) << 3
	o_shifted := toLong(v.Y/float64(l)) << 18
	p_shifted := toLong(v.Z/float64(l)) << 33

	// Combine all bits
	q := m | n_shifted | o_shifted | p_shifted

	// Write bytes in little-endian order (as MC packet format requires)
	bytes := []pk.Byte{
		pk.Byte((q >> 0) & 0xFF),  // Byte 0: bits 0-7
		pk.Byte((q >> 8) & 0xFF),  // Byte 1: bits 8-15
		pk.Byte((q >> 16) & 0xFF), // Byte 2: bits 16-23
		pk.Byte((q >> 24) & 0xFF), // Byte 3: bits 24-31
		pk.Byte((q >> 32) & 0xFF), // Byte 4: bits 32-39
		pk.Byte((q >> 40) & 0xFF), // Byte 5: bits 40-47
	}

	for _, b := range bytes {
		nn, err := b.WriteTo(w)
		if err != nil {
			return n, err
		}
		n += nn
	}

	// If continuation flag is set, write extra scale bits as VarInt
	if bl {
		extraScale := pk.VarInt((l >> 2) & 0xFFFFFFFF)
		nn, err := extraScale.WriteTo(w)
		if err != nil {
			return n, err
		}
		n += nn
	}

	return n, nil
}

// varIntSize returns the number of bytes a VarInt will occupy
func varIntSize(value int32) int {
	for i := 1; i <= 5; i++ {
		if (value & ^0x7F) == 0 {
			return i
		}
		value >>= 7
	}
	return 5
}
