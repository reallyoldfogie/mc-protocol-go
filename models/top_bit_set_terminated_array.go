package models

import (
	"bytes"
	"errors"
	"io"
	"log"

	pk "github.com/Tnze/go-mc/net/packet"
)

// ErrTopBitArrayTooLong is returned if the decode exceeds a safety limit.
var ErrTopBitArrayTooLong = errors.New("topBitSetTerminatedArray too long")

// DefaultMaxTopBitElems is used when TopBitSetTerminatedArray.MaxElems <= 0.
const DefaultMaxTopBitElems = 4096

// TopBitSetTerminatedArray[T] implements the ProtoDef-style "topBitSetTerminatedArray":
// - Reads a sequence of 7-bit slot indices where MSB=1 means "more elements follow"
// - For each decoded slot index, reads one element of type T from the stream
// - On write, emits slot indices (all but last with MSB=1) followed by all T values in order
//
// NOTE: This is inverted from a common "MSB marks last element" convention.
// It intentionally matches the Java reference implementation used by Minecraft:
// EntityEquipmentUpdateS2CPacket reads with:
//
//	do { ... i = buf.readByte(); ... } while ((i & -128) != 0);
//
// and writes with MSB set on non-final entries.
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

// ReadFrom reads slot indices and values in interleaved fashion, matching Java's
// EntityEquipmentUpdateS2CPacket:
//
//	do { i = buf.readByte(); slot = i & 0x7F; item = decode(buf); } while ((i & 0x80) != 0)
//
// The first byte of each element carries the MSB continue-flag.  We consume it,
// record the clean slot index, then splice a reader that feeds the cleaned byte
// (MSB stripped) followed by the rest of the stream.  The entry's ReadFrom
// therefore sees a normal slot value with no protocol flag leaked through.
func (a *TopBitSetTerminatedArray[T]) ReadFrom(r io.Reader) (int64, error) {
	max := a.MaxElems
	if max <= 0 {
		max = DefaultMaxTopBitElems
	}

	// Read all remaining data upfront so each entry gets a *bytes.Reader,
	// which natively implements io.ByteReader.  This avoids subtle
	// position-drift bugs that occur when io.MultiReader (which does NOT
	// implement ByteReader) forces downstream VarInt readers to wrap it
	// in a bufio.Reader whose read-ahead buffer is lost between entries.
	allData, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	var totalBytes int64
	pos := 0
	fullLen := len(allData)

	for {
		if len(a.SlotIndices) >= max {
			return totalBytes, ErrTopBitArrayTooLong
		}

		if pos >= len(allData) {
			return totalBytes, io.ErrUnexpectedEOF
		}

		log.Printf("[TopBitSetTerminatedArray] before cleaning flagByte: numIndices=%d pos=%d/%d remainingData=% X (%d bytes)",
			len(a.SlotIndices), pos, fullLen, allData[pos:], len(allData[pos:]))

		// Read the flag byte and strip the MSB continue-flag.
		flagByte := allData[pos]
		cleanedFlagByte := flagByte & 0x7F
		a.SlotIndices = append(a.SlotIndices, cleanedFlagByte)

		log.Printf("[TopBitSetTerminatedArray] flagByte=0x%02X (%08b) cleanedFlagByte=0x%02X (%08b) numIndices=%d pos=%d/%d",
			flagByte, flagByte, cleanedFlagByte, cleanedFlagByte, len(a.SlotIndices), pos, fullLen)

		// Replace the flag byte in-place with the clean slot value so the
		// entry's ReadFrom sees a normal slot byte without the MSB flag.
		allData[pos] = cleanedFlagByte
		entryReader := bytes.NewReader(allData[pos:])

		log.Printf("[TopBitSetTerminatedArray] remaining allData after updating cleanedFlagByte: % X (%d bytes)",
			allData[pos:], len(allData[pos:]))

		var tmp T
		val := &tmp
		a.Values = append(a.Values, val)

		var bytesRead int64
		if dec, ok := any(val).(ParentContextAwareDecoder); ok && a.parentCtx != nil {
			bytesRead, err = dec.ReadFromWithParentContext(entryReader, a.parentCtx)
		} else if rf, ok := any(val).(io.ReaderFrom); ok {
			bytesRead, err = rf.ReadFrom(entryReader)
		} else {
			return totalBytes, errors.New("value does not implement io.ReaderFrom")
		}

		pos += int(bytesRead)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, err
		}

		if flagByte&0x80 == 0 {
			break
		}
	}
	return totalBytes, nil
}

// WriteTo writes entries in interleaved fashion, matching Java's EntityEquipmentUpdateS2CPacket.
// Each entry's first byte carries the MSB continue-flag.  We buffer each entry's
// serialized output, then set MSB=1 on non-final entries and MSB=0 on the final
// entry before writing the buffer to w.
// Matches Java: buf.writeByte(bl ? k | -128 : k); where bl = (j != i-1)
func (a TopBitSetTerminatedArray[T]) WriteTo(w io.Writer) (int64, error) {
	var totalBytes int64

	if len(a.Values) == 0 {
		// Empty array: write a single zero byte as immediate terminator (MSB clear).
		bytesWritten, err := w.Write([]byte{0x00})
		return int64(bytesWritten), err
	}

	for i := range a.Values {
		if a.Values[i] == nil {
			return totalBytes, errors.New("TopBitSetTerminatedArray value is nil")
		}

		// Write the entry into a buffer so we can patch the first byte.
		var buf bytes.Buffer
		if enc, ok := any(a.Values[i]).(ParentContextAwareEncoder); ok && a.parentCtx != nil {
			if _, err := enc.WriteToWithParentContext(&buf, a.parentCtx); err != nil {
				return totalBytes, err
			}
		} else if enc, ok := any(a.Values[i]).(io.WriterTo); ok {
			if _, err := enc.WriteTo(&buf); err != nil {
				return totalBytes, err
			}
		} else {
			return totalBytes, errors.New("value does not implement io.WriterTo")
		}

		data := buf.Bytes()
		if len(data) == 0 {
			return totalBytes, errors.New("TopBitSetTerminatedArray entry wrote zero bytes")
		}

		// Patch the first byte (the slot/key byte) with the continue flag.
		data[0] &= 0x7F // clear MSB
		if i != len(a.Values)-1 {
			data[0] |= 0x80 // non-final: set continue flag
		}

		bytesWritten, err := w.Write(data)
		totalBytes += int64(bytesWritten)
		if err != nil {
			return totalBytes, err
		}
	}
	return totalBytes, nil
}

// GetFields returns an empty map; kept for consistency with other field wrappers.
func (a TopBitSetTerminatedArray[T]) GetFields() map[string]pk.FieldEncoder {
	return make(map[string]pk.FieldEncoder)
}

// SetFields is a no-op.
func (a *TopBitSetTerminatedArray[T]) SetFields(fields map[string]pk.FieldEncoder) {}
