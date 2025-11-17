package models

import (
	"io"
)

// Buffer represents raw bytes without length prefix
type Buffer struct {
	Data []byte
}

func (b Buffer) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write(b.Data)
	return int64(nn), err
}

func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	b.Data = data
	return int64(len(b.Data)), err
}
