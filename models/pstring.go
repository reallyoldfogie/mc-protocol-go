package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

// PString represents a length-prefixed string
type PString struct {
	Value string
}

func (ps PString) WriteTo(w io.Writer) (int64, error) {
	s := pk.String(ps.Value)
	return s.WriteTo(w)
}

func (ps *PString) ReadFrom(r io.Reader) (int64, error) {
	var s pk.String
	n, err := s.ReadFrom(r)
	ps.Value = string(s)
	return n, err
}
