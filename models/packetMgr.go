package models

type PacketMgr interface {
	// GetClientboundPacketIDGuard() ClientboundPacketID
	Name() string
	VersionProtocol() uint64

	GetClientboundLoginPacketID(name string) ClientboundPacketID
	GetServerboundLoginPacketID(name string) ServerboundPacketID

	GetClientboundConfigPacketID(name string) ClientboundPacketID
	GetServerboundConfigPacketID(name string) ServerboundPacketID

	GetClientboundPacketID(name string) ClientboundPacketID
	GetServerboundPacketID(name string) ServerboundPacketID

	ClientboundLoginToString(id ClientboundPacketID) string
	ClientboundConfigToString(id ClientboundPacketID) string
	ServerboundConfigToString(id ServerboundPacketID) string

	ClientboundToString(id ClientboundPacketID) string
	ServerboundToString(id ServerboundPacketID) string

	// Packet factory methods that return PacketMarshaller interface
	GetClientboundLoginPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundConfigPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetClientboundPacketByID(id ClientboundPacketID) (PacketMarshaller, error)
	GetServerboundLoginPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
	GetServerboundConfigPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
	GetServerboundPacketByID(id ServerboundPacketID) (PacketMarshaller, error)
}
