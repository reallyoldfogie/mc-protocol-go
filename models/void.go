package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

// Void represents an empty field that implements pk.Field.
// It is used for void/empty protocol fields that need to participate
// in serialization but have no data to read or write.
type Void struct {
	packetID int32
}

// ReadFrom implements io.ReaderFrom with a no-op (reads nothing).
func (v *Void) ReadFrom(r io.Reader) (int64, error) {
	return 0, nil
}

// WriteTo implements io.WriterTo with a no-op (writes nothing).
func (v Void) WriteTo(w io.Writer) (int64, error) {
	return 0, nil
}

// Marshal converts the packet struct into a pk.Packet ready for transmission.
func (v Void) Marshal() pk.Packet {
	return pk.Marshal(v.packetID)
}

// Scan populates the packet struct from a received pk.Packet.
func (v Void) Scan(packet pk.Packet) error {
	return nil
}

// PacketID returns the packet's protocol ID.
func (v Void) PacketID() int32 {
	return v.packetID
}

func (v Void) GetFields() map[string]pk.FieldEncoder {
	return map[string]pk.FieldEncoder{}
}

func (v Void) SetFields(fields map[string]pk.FieldEncoder) {}

func NewVoid() PacketMarshaller {
	return &Void{packetID: 0}
}
