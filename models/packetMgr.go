package models

type ClientboundPacketMgr interface {
	GetClientboundLoginPacketID(name string) ClientboundPacketID
	GetClientboundConfigPacketID(name string) ClientboundPacketID
	GetClientboundPacketID(name string) ClientboundPacketID
	ClientboundLoginToString(id ClientboundPacketID) string
	ClientboundConfigToString(id ClientboundPacketID) string
	ClientboundToString(id ClientboundPacketID) string
	GetClientboundLoginPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundConfigPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundHandshakingPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundStatusPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
}
type ServerboundPacketMgr interface {
	GetServerboundLoginPacketID(name string) ServerboundPacketID

	GetServerboundConfigPacketID(name string) ServerboundPacketID

	GetServerboundPacketID(name string) ServerboundPacketID

	ServerboundConfigToString(id ServerboundPacketID) string

	ServerboundToString(id ServerboundPacketID) string

	// Packet factory methods that return PacketMarshaller interface
	GetServerboundLoginPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
	GetServerboundConfigPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
	GetServerboundPacketByID(id ServerboundPacketID) (PacketMarshaller, error)

	GetServerboundHandshakingPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
	GetServerboundStatusPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
}

type PacketMgr interface {
	// GetClientboundPacketIDGuard() ClientboundPacketID
	ClientboundPacketMgr
	ServerboundPacketMgr

	Name() string
	VersionProtocol() uint64

	// GetEntityTypeID returns the protocol ID for an entity type name
	// e.g., GetEntityTypeID("minecraft:player") returns 148 for 1.21.5
	// Returns -1 if the entity type is not found
	GetEntityTypeID(name string) int32
}
