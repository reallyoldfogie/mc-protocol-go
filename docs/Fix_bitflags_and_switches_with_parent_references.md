# Fix Bitflags and switches with parent references

Goals:
1. Fix bitflags to be accessible by name, with fields of this type properly generated (and referenced as needed for serialization - models.Bitflag will probably need a SetFlag() function that cooresponds to the HasBit() function).  I have included a sample of what I would like the implementation to look like, along with usage code in PlayerInfo et. al. (see #2)
2. I need to fully implement the ReadFrom/WriteTo implementation for PlayerInfo et.al., taken from packet_player_info in /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/.cache/metadata/1.21.5/downloads/protocol.json. protocol.json is parsed by protodef-go (/home/reallyoldfogie/src/github.com/reallyoldfogie/protodef-go).  Currently the Switch type elements that have references to a field in their parent are being generated as pk.Field, but they need to be generated as show in the POC below.  
   a. We need to update the generator code to accomplish this task. See /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/packets.go, /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/generator.go and /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/internal/generator/templates.go.

The problem: PlayerInfoArrayType is a container struct with switch fields that reference parent fields (using ../). The generator detects these parent references via the containerHasParentReferences function and skips generating ReadFrom/WriteTo methods for it. However, PlayerInfoArrayType is used as an array element type in models.Array[pk.VarInt, PlayerInfoArrayType], which requires it to implement packet.FieldDecoder interface (which includes the ReadFrom method).

The root cause: When a container has fields that are switches with compareTo values like ../action (referencing a parent field), the generator assumes the container can't have standalone ReadFrom/WriteTo methods because those parent fields won't be available. However, when this type is used as an array element, it still needs these methods, and the array reading code needs to handle the parent context somehow.

*** Unknown: I don't know how to pass the value of PlayerInfo to PlayerInfoArrayType (and its properties) so they can be properly serialized at runtime. If changes to models.Array are needed, they need to be backwards compatible for other uses that don't have parental references.


```protodef /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/.cache/metadata/1.21.5/downloads/protocol.json
"packet_player_info": [
          "container",
          [
            {
              "name": "action",
              "type": [
                "bitflags",
                {
                  "type": "u8",
                  "flags": [
                    "add_player",
                    "initialize_chat",
                    "update_game_mode",
                    "update_listed",
                    "update_latency",
                    "update_display_name",
                    "update_hat",
                    "update_list_order"
                  ]
                }
              ]
            },
            {
              "name": "data",
              "type": [
                "array",
                {
                  "countType": "varint",
                  "type": [
                    "container",
                    [
                      {
                        "name": "uuid",
                        "type": "UUID"
                      },
                      {
                        "name": "player",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/add_player",
                            "fields": {
                              "true": "game_profile"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "chatSession",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/initialize_chat",
                            "fields": {
                              "true": "chat_session"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "gamemode",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_game_mode",
                            "fields": {
                              "true": "varint"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "listed",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_listed",
                            "fields": {
                              "true": "varint"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "latency",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_latency",
                            "fields": {
                              "true": "varint"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "displayName",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_display_name",
                            "fields": {
                              "true": [
                                "option",
                                "anonymousNbt"
                              ]
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "listPriority",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_list_order",
                            "fields": {
                              "true": "varint"
                            },
                            "default": "void"
                          }
                        ]
                      },
                      {
                        "name": "showHat",
                        "type": [
                          "switch",
                          {
                            "compareTo": "../action/update_hat",
                            "fields": {
                              "true": "bool"
                            },
                            "default": "void"
                          }
                        ]
                      }
                    ]
                  ]
                }
              ]
            }
          ]
        ],
```


```go /home/reallyoldfogie/src/github.com/reallyoldfogie/vendor/github.com/Tnze/go-mc/net/packet/types.go pk.Byte
	// Byte is signed 8-bit integer, two's complement
	Byte int8
```

```go /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/models/bitflags.go models.Bitflags
package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

type Bitflags pk.Byte

func (bf Bitflags) WriteTo(w io.Writer) (int64, error) {
	pkByte := (pk.Byte)(bf)
	return pkByte.WriteTo(w)
}

func (bf *Bitflags) ReadFrom(r io.Reader) (int64, error) {
	pkByte := (*pk.Byte)(bf)
	return pkByte.ReadFrom(r)
}

// HasBit checks if a specific bit position is set
// bitPosition is 0-indexed from the right (LSB)
func (bf Bitflags) HasBit(bitPosition int) bool {
	return (byte(bf) & (1 << bitPosition)) != 0
}

```

## OLD (current) 

```go /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/play/clientbound/types.go (snippet)
// {PlayerInfoArrayType container  0xc000369b30}
type PlayerInfoArrayType struct {
	Uuid         pk.UUID
	Player       pk.Field
	ChatSession  pk.Field
	Gamemode     pk.Field
	Listed       pk.Field
	Latency      pk.Field
	DisplayName  pk.Field
	ListPriority pk.Field
	ShowHat      pk.Field
}

// {PlayerInfo container  0xc000369a70}
type PlayerInfo struct {
	packetID int32

	Action models.Bitflags
	Data   models.Array[pk.VarInt, PlayerInfoArrayType]
}

// NewPlayerInfo creates a new PlayerInfo packet with the correct packet ID.
func NewPlayerInfo() *PlayerInfo {
	return &PlayerInfo{packetID: 63}
}

// PacketID returns the protocol ID for this packet type.
func (p *PlayerInfo) PacketID() int32 {
	return p.packetID
}

// Marshal serializes the packet into wire format.
func (p *PlayerInfo) Marshal() pk.Packet {
	return pk.Marshal(p.packetID, &p.Action, &p.Data)
}

// Scan deserializes a wire-format packet into this struct.
func (p *PlayerInfo) Scan(packet pk.Packet) error {
	if packet.ID != p.packetID {
		return fmt.Errorf("packet ID mismatch: expected %d, got %d", p.packetID, packet.ID)
	}
	return packet.Scan(&p.Action, &p.Data)
}

func (p *PlayerInfo) GetFields() map[string]pk.FieldEncoder {
	fields := map[string]pk.FieldEncoder{}
	fields["Action"] = p.Action
	fields["Data"] = p.Data
	return fields
}

func (p *PlayerInfo) SetFields(fields map[string]pk.FieldEncoder) {
	fmt.Printf("<no value>\n")

	if val, ok := fields["Action"]; ok {
		p.Action = val.(models.Bitflags)
	}
	if val, ok := fields["Data"]; ok {
		p.Data = val.(models.Array[pk.VarInt, PlayerInfoArrayType])
	}
}

func (t *PlayerInfo) ReadFrom(r io.Reader) (totalBytes int64, err error) {

	var bytesRead int64

	bytesRead, err = t.Action.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = t.Data.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	return totalBytes, nil
}

func (t PlayerInfo) WriteTo(w io.Writer) (totalBytes int64, err error) {
	var bytesWritten int64

	defer func() {
		log.Printf("[ChatMessage.WriteTo] totalBytes: %d err: %#v", totalBytes, err)
	}()
	bytesWritten, err = t.Action.WriteTo(w)
	totalBytes += bytesWritten
	if err != nil {
		return totalBytes, err
	}

	bytesWritten, err = t.Data.WriteTo(w)
	totalBytes += bytesWritten
	if err != nil {
		return totalBytes, err
	}

	return totalBytes, nil
}
```

## NEW UPDATED PARTIAL POC

```go /home/reallyoldfogie/src/github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/play/clientbound/types.go (snippet) NEW UPDATED PARTIAL POC
type PlayerInfoActionBitFlags struct {
	models.Bitflags
}

func (p PlayerInfoActionBitFlags) AddPlayer() bool         { return p.Bitflags.HasBit(0) }
func (p PlayerInfoActionBitFlags) InitializeChat() bool    { return p.Bitflags.HasBit(1) }
func (p PlayerInfoActionBitFlags) UpdateGameMode() bool    { return p.Bitflags.HasBit(2) }
func (p PlayerInfoActionBitFlags) UpdateListed() bool      { return p.Bitflags.HasBit(3) }
func (p PlayerInfoActionBitFlags) UpdateLatency() bool     { return p.Bitflags.HasBit(4) }
func (p PlayerInfoActionBitFlags) UpdateDisplayName() bool { return p.Bitflags.HasBit(5) }
func (p PlayerInfoActionBitFlags) UpdateHat() bool         { return p.Bitflags.HasBit(6) }
func (p PlayerInfoActionBitFlags) UpdateListOrder() bool   { return p.Bitflags.HasBit(7) }

type PlayerInfoArrayTypePlayer struct {
	ParentAction PlayerInfoActionBitFlags // SOMEHOW WE NEED TO SET THIS SO PROPER SERIALIZATION CAN HAPPEN
	True         basetypes.GameProfile
	Default      models.Void
}

func (t *PlayerInfoArrayTypePlayer) ReadFrom(r io.Reader) (totalBytes int64, err error) {
	compareValueAction := t.ParentAction.AddPlayer()

	switch compareValueAction {
	case true:
		return t.True.ReadFrom(r)
	default:
		return t.Default.ReadFrom(r)
	}
}

// {PlayerInfoArrayType container  0xc000120c30}
type PlayerInfoArrayType struct {
	Uuid         pk.UUID
	Player       PlayerInfoArrayTypePlayer
	ChatSession  pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	Gamemode     pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	Listed       pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	Latency      pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	DisplayName  pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	ListPriority pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
	ShowHat      pk.Field // NEED SIMILAR TO PlayerInfoArrayTypePlayer
}

func (p *PlayerInfoArrayType) ReadFrom(r io.Reader) (totalBytes int64, err error) {
	var bytesRead int64
	bytesRead, err = p.Uuid.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.Player.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.ChatSession.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.Gamemode.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.Listed.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.Latency.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.DisplayName.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.ListPriority.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	bytesRead, err = p.ShowHat.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

}

// {PlayerInfo container  0xc000120ba0}
type PlayerInfo struct {
	packetID int32

	Action models.Bitflags
	Data   models.Array[pk.VarInt, PlayerInfoArrayType]
}

// NewPlayerInfo creates a new PlayerInfo packet with the correct packet ID.
func NewPlayerInfo() *PlayerInfo {
	return &PlayerInfo{packetID: 63}
}

// PacketID returns the protocol ID for this packet type.
func (p *PlayerInfo) PacketID() int32 {
	return p.packetID
}

// Marshal serializes the packet into wire format.
func (p *PlayerInfo) Marshal() pk.Packet {
	return pk.Marshal(p.packetID, &p.Action, &p.Data)
}

// Scan deserializes a wire-format packet into this struct.
func (p *PlayerInfo) Scan(packet pk.Packet) error {
	if packet.ID != p.packetID {
		return fmt.Errorf("packet ID mismatch: expected %d, got %d", p.packetID, packet.ID)
	}
	return packet.Scan(&p.Action, &p.Data)
}

func (p *PlayerInfo) GetFields() map[string]pk.FieldEncoder {
	fields := map[string]pk.FieldEncoder{}
	fields["Action"] = p.Action
	fields["Data"] = p.Data
	return fields
}

func (p *PlayerInfo) SetFields(fields map[string]pk.FieldEncoder) {
	fmt.Printf("<no value>\n")

	if val, ok := fields["Action"]; ok {
		p.Action = val.(models.Bitflags)
	}
	if val, ok := fields["Data"]; ok {
		p.Data = val.(models.Array[pk.VarInt, PlayerInfoArrayType])
	}
}

func (t *PlayerInfo) ReadFrom(r io.Reader) (totalBytes int64, err error) {

	var bytesRead int64

	bytesRead, err = t.Action.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	// data := models.Array[pk.VarInt, PlayerInfoArrayType]{} // how to pass t.Action to PlayerInfoArrayType for use in ReadFrom?
	bytesRead, err = t.Data.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, err
	}

	return totalBytes, nil
}

func (t PlayerInfo) WriteTo(w io.Writer) (totalBytes int64, err error) {
	var bytesWritten int64

	defer func() {
		log.Printf("[ChatMessage.WriteTo] totalBytes: %d err: %#v", totalBytes, err)
	}()
	bytesWritten, err = t.Action.WriteTo(w)
	totalBytes += bytesWritten
	if err != nil {
		return totalBytes, err
	}

	bytesWritten, err = t.Data.WriteTo(w)
	totalBytes += bytesWritten
	if err != nil {
		return totalBytes, err
	}

	return totalBytes, nil
}
```