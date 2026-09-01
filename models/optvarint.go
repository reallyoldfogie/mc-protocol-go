package models

import (
	pk "github.com/Tnze/go-mc/net/packet"
)

// OptVarInt is a type alias for pk.VarInt used in protocol definitions where optvarint is specified.
// Unlike Option[VarInt] which reads a boolean prefix followed by a VarInt,
// OptVarInt is just a plain VarInt with no boolean prefix.
// This is used in entity metadata entry types like optional_block_state and optional_unsigned_int
// which are semantically optional but have no wire-level presence indicator.
// if the value is zero, the option is false, otherwise it has a meaningful value
type OptVarInt = pk.VarInt
