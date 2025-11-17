package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

type Bitflags pk.Byte

func (bf Bitflags) WriteTo(w io.Writer) (int64, error) {
	pkByte := (pk.Byte)(bf)
	return pkByte.WriteTo(w)
}

func (bf *Bitflags) ReadFrom(r io.Reader) (int64, error) {
	pkByte := (*pk.Byte)(bf)
	return pkByte.ReadFrom(r)
}

// HasBit checks if a specific bit position is set
// bitPosition is 0-indexed from the right (LSB)
func (bf Bitflags) HasBit(bitPosition int) bool {
	return (byte(bf) & (1 << bitPosition)) != 0
}

// SetFlag sets or clears a specific bit position.
// bitPosition is 0-indexed from the right (LSB).
func (bf *Bitflags) SetFlag(bitPosition int, value bool) {
	if value {
		*bf = Bitflags(byte(*bf) | (1 << bitPosition))
	} else {
		*bf = Bitflags(byte(*bf) &^ (1 << bitPosition))
	}
}
