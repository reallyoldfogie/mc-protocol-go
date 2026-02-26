package models

import (
	"io"

	"github.com/pkg/errors"
)

// RestBuffer represents raw bytes that consume the remaining packet payload.
type RestBuffer struct {
	Data []byte
}

func (b RestBuffer) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write(b.Data)
	return int64(nn), errors.Wrap(err, "RestBuffer.WriteTo failed")
}

func (b *RestBuffer) ReadFrom(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	b.Data = data
	// io.ReadAll never returns EOF on success; it returns nil.
	// If we got an error, it's a real error, so propagate it as-is.
	// DEBUG: Log how much data was read
	if len(data) > 0 {
		println("[RestBuffer.ReadFrom] Read", len(data), "bytes")
	}
	return int64(len(b.Data)), err
}
