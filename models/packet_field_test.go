package models

import (
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/stretchr/testify/assert"
)

// test implementation of PacketMarshaller for field access tests
type testPkt struct {
	packetID int32
	Name     pk.String
	Count    pk.VarInt
	Flag     pk.Boolean
}

func (p *testPkt) Marshal() pk.Packet     { return pk.Marshal(p.packetID, &p.Name, &p.Count, &p.Flag) }
func (p *testPkt) Scan(_ pk.Packet) error { return nil }
func (p *testPkt) PacketID() int32        { return p.packetID }
func (p *testPkt) GetFields() map[string]pk.FieldEncoder {
	return map[string]pk.FieldEncoder{
		"Name":  p.Name,
		"Count": p.Count,
		"Flag":  p.Flag,
	}
}
func (p *testPkt) SetFields(fields map[string]pk.FieldEncoder) {
	if v, ok := fields["Name"]; ok {
		p.Name = v.(pk.String)
	}
	if v, ok := fields["Count"]; ok {
		p.Count = v.(pk.VarInt)
	}
	if v, ok := fields["Flag"]; ok {
		p.Flag = v.(pk.Boolean)
	}
}

func TestGetPacketFieldValue_BasicPkTypes(t *testing.T) {
	p := &testPkt{packetID: 123, Name: pk.String("hello"), Count: pk.VarInt(42), Flag: pk.Boolean(true)}

	v, ok := GetPacketFieldValue(p, "Name")
	assert.True(t, ok)
	s, ok := v.(string)
	assert.True(t, ok)
	assert.Equal(t, "hello", s)

	v, ok = GetPacketFieldValue(p, "Count")
	assert.True(t, ok)
	// pk.VarInt underlying is int32
	i32, ok := v.(int32)
	assert.True(t, ok)
	assert.Equal(t, int32(42), i32)

	v, ok = GetPacketFieldValue(p, "Flag")
	assert.True(t, ok)
	b, ok := v.(bool)
	assert.True(t, ok)
	assert.True(t, b)

	_, ok = GetPacketFieldValue(p, "missing")
	assert.False(t, ok)
}

func TestGetPacketFieldAs_Typed(t *testing.T) {
	p := &testPkt{packetID: 123, Name: pk.String("world"), Count: pk.VarInt(7), Flag: pk.Boolean(false)}

	s, ok := GetPacketFieldAs[string](p, "Name")
	assert.True(t, ok)
	assert.Equal(t, "world", s)

	// request the pk type; conversion from string -> pk.String should work
	ps, ok := GetPacketFieldAs[pk.String](p, "Name")
	assert.True(t, ok)
	assert.Equal(t, pk.String("world"), ps)

	i, ok := GetPacketFieldAs[int32](p, "Count")
	assert.True(t, ok)
	assert.Equal(t, int32(7), i)

	b, ok := GetPacketFieldAs[bool](p, "Flag")
	assert.True(t, ok)
	assert.False(t, b)

	_, ok = GetPacketFieldAs[float64](p, "Count")
	assert.False(t, ok)
}

func TestPacketFieldExistsAndEncoder(t *testing.T) {
	p := &testPkt{packetID: 1, Name: pk.String("x"), Count: pk.VarInt(1), Flag: pk.Boolean(false)}

	assert.True(t, PacketFieldExists(p, "Name"))
	assert.True(t, PacketFieldExists(p, "Count"))
	assert.True(t, PacketFieldExists(p, "Flag"))
	assert.False(t, PacketFieldExists(p, "DoesNotExist"))

	enc, ok := GetPacketFieldEncoder(p, "Name")
	assert.True(t, ok)
	// should be a pk.String encoder
	if s, ok := enc.(pk.String); ok {
		assert.Equal(t, pk.String("x"), s)
	} else {
		t.Fatalf("expected pk.String encoder, got %T", enc)
	}
}
