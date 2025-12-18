package models

import (
	"io"

	"github.com/pkg/errors"
)

// Buffer represents raw bytes without length prefix
type Buffer struct {
	Data []byte
}

func (b Buffer) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write(b.Data)
	return int64(nn), errors.Wrap(err, "Buffer.WriteTo failed")
}

func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	b.Data = data
	// io.ReadAll never returns EOF on success; it returns nil.
	// If we got an error, it's a real error, so propagate it as-is.
	// DEBUG: Log how much data was read
	if len(data) > 0 {
		println("[Buffer.ReadFrom] Read", len(data), "bytes")
	}
	return int64(len(b.Data)), err
}
