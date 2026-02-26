package models

import (
	"bufio"
	"errors"
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

// ErrTopBitArrayTooLong is returned if the decode exceeds a safety limit.
var ErrTopBitArrayTooLong = errors.New("topBitSetTerminatedArray too long")

// DefaultMaxTopBitElems is used when TopBitSetTerminatedArray.MaxElems <= 0.
const DefaultMaxTopBitElems = 4096

// TopBitSetTerminatedArray[T] implements the ProtoDef-style "topBitSetTerminatedArray":
// - Reads a sequence of 7-bit slot indices terminated by a byte with MSB=1
// - For each decoded slot index, reads one element of type T from the stream
// - On write, emits slot indices (last with MSB=1) followed by all T values in order
//
// The element type T can be a value or pointer type.
// Elements are stored as pointers (*T) internally for consistency with how ReadFrom allocates them.
// ParentContext-aware encoders/decoders are also supported.
//
// SlotIndices must be set by callers before WriteTo if the write path is used.
// In many protocols, the slot index is not redundantly encoded inside T.
type TopBitSetTerminatedArray[T any] struct {
	SlotIndices []byte
	Values      []*T // Store pointers to allow ReadFrom to allocate elements
	MaxElems    int
	parentCtx   ParentContext
}

func (a *TopBitSetTerminatedArray[T]) SetParentContext(ctx ParentContext) { a.parentCtx = ctx }

// ReadFrom reads indices (terminated by MSB=1) then reads one T per index.
func (a *TopBitSetTerminatedArray[T]) ReadFrom(r io.Reader) (int64, error) {
	max := a.MaxElems
	if max <= 0 {
		max = DefaultMaxTopBitElems
	}
	br, ok := r.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(r)
	}
	var n int64
	// 1) read indices
	for {
		if len(a.SlotIndices) >= max {
			return n, ErrTopBitArrayTooLong
		}
		b, err := br.ReadByte()
		if err != nil {
			return n, err
		}
		n++
		a.SlotIndices = append(a.SlotIndices, b&0x7F)
		if b&0x80 != 0 {
			break
		}
	}
	// 2) read values
	cnt := len(a.SlotIndices)
	a.Values = make([]*T, cnt)
	for i := 0; i < cnt; i++ {
		// Allocate a new T
		var tmp T
		a.Values[i] = &tmp
		// Prefer context-aware decoders on pointer receiver
		if dec, ok := any(a.Values[i]).(ParentContextAwareDecoder); ok && a.parentCtx != nil {
			m, err := dec.ReadFromWithParentContext(r, a.parentCtx)
			n += m
			if err != nil {
				return n, err
			}
			continue
		}
		// Check if pointer implements io.ReaderFrom
		if rf, ok := any(a.Values[i]).(io.ReaderFrom); ok {
			m, err := rf.ReadFrom(r)
			n += m
			if err != nil {
				return n, err
			}
			continue
		}
		return n, errors.New("value does not implement io.ReaderFrom")
	}
	return n, nil
}

// WriteTo writes indices (last with MSB=1) then writes each T in order.
func (a TopBitSetTerminatedArray[T]) WriteTo(w io.Writer) (int64, error) {
	var n int64
	// 1) indices
	if len(a.SlotIndices) == 0 {
		m, err := w.Write([]byte{0x80})
		return int64(m), err
	}
	for i := 0; i < len(a.SlotIndices); i++ {
		b := a.SlotIndices[i] & 0x7F
		if i == len(a.SlotIndices)-1 {
			b |= 0x80
		}
		m, err := w.Write([]byte{b})
		n += int64(m)
		if err != nil {
			return n, err
		}
	}
	// 2) values
	for i := range a.Values {
		if a.Values[i] == nil {
			return n, errors.New("TopBitSetTerminatedArray value is nil")
		}
		if enc, ok := any(a.Values[i]).(ParentContextAwareEncoder); ok && a.parentCtx != nil {
			m, err := enc.WriteToWithParentContext(w, a.parentCtx)
			n += m
			if err != nil {
				return n, err
			}
			continue
		}
		// Check if pointer implements io.WriterTo
		if enc, ok := any(a.Values[i]).(io.WriterTo); ok {
			m, err := enc.WriteTo(w)
			n += m
			if err != nil {
				return n, err
			}
			continue
		}
		return n, errors.New("value does not implement io.WriterTo")
	}
	return n, nil
}

// GetFields returns an empty map; kept for consistency with other field wrappers.
func (a TopBitSetTerminatedArray[T]) GetFields() map[string]pk.FieldEncoder {
	return make(map[string]pk.FieldEncoder)
}

// SetFields is a no-op.
func (a *TopBitSetTerminatedArray[T]) SetFields(fields map[string]pk.FieldEncoder) {}
