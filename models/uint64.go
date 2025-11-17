package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

type UInt64 uint64

func (ui UInt64) WriteTo(w io.Writer) (int64, error) {
	l := pk.Long(ui)
	return l.WriteTo(w)
}

func (ui *UInt64) ReadFrom(r io.Reader) (int64, error) {
	var l pk.Long
	nn, err := l.ReadFrom(r)
	if err == nil {
		*ui = UInt64(l)
	}
	return nn, err
}
