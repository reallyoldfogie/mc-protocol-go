package models

import pk "github.com/Tnze/go-mc/net/packet"

// PacketMarshaller is the common interface implemented by all generated packet structs.
// It provides methods to convert between Go structs and wire-format packets.
type PacketMarshaller interface {
	// Marshal converts the packet struct into a pk.Packet ready for transmission.
	Marshal() pk.Packet

	// Scan populates the packet struct from a received pk.Packet.
	Scan(packet pk.Packet) error

	// PacketID returns the packet's protocol ID.
	PacketID() int32

	GetFields() map[string]pk.FieldEncoder
	SetFields(fields map[string]pk.FieldEncoder)
}
