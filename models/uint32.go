package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

type UInt32 uint32

func (ui UInt32) WriteTo(w io.Writer) (int64, error) {
	i := pk.Int(ui)
	return i.WriteTo(w)
}

func (ui *UInt32) ReadFrom(r io.Reader) (int64, error) {
	var i pk.Int
	nn, err := i.ReadFrom(r)
	if err == nil {
		*ui = UInt32(i)
	}
	return nn, err
}
