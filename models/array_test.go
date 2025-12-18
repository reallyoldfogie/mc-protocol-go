package models

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockContextElement struct {
	value pk.VarInt
	seen  bool
}

func (m *mockContextElement) ReadFrom(r io.Reader) (int64, error) {
	return m.value.ReadFrom(r)
}
func (m *mockContextElement) ReadFromWithParentContext(r io.Reader, ctx ParentContext) (int64, error) {
	ctxValue := ctx.GetField("test")
	if ctxValue != nil {
		m.seen = true
	}

	return m.value.ReadFrom(r)
}

func (m mockContextElement) WriteTo(w io.Writer) (int64, error) {
	return m.value.WriteTo(w)
}

func (m mockContextElement) WriteToWithParentContext(w io.Writer, ctx ParentContext) (int64, error) {
	return m.value.WriteTo(w)
}

func TestArrayReadWriteWithoutParentContext(t *testing.T) {
	buf := &bytes.Buffer{}

	orig := Array[pk.VarInt, pk.VarInt]{}
	origValues := []pk.VarInt{1, 2, 3}
	orig.Ary = Ary[pk.VarInt]{Ary: &origValues}

	_, err := orig.WriteTo(buf)
	require.NoError(t, err)

	var decoded Array[pk.VarInt, pk.VarInt]
	_, err = decoded.ReadFrom(buf)
	require.NoError(t, err)

	require.NotNil(t, decoded.Ary.Ary)

	var values []pk.VarInt

	switch v := decoded.Ary.Ary.(type) {
	case []pk.VarInt:
		// encoding case: pk.Ary is usually constructed with a plain slice
		values = v
	case *[]pk.VarInt:
		// decoding case: pk.Ary is usually constructed with &slice
		values = *v
	default:
		require.NoError(t, fmt.Errorf("Array.Ary has unexpected type %T; want []pk.VarInt or *[]pk.VarInt", decoded.Ary.Ary))
	}

	require.Len(t, values, len(origValues))
	require.Equal(t, origValues, values)
}

func TestArrayReadWithParentContextAwareElements(t *testing.T) {
	buf := &bytes.Buffer{}

	values := []mockContextElement{{value: 5}, {value: 6}}
	ary := Array[pk.VarInt, mockContextElement]{
		Ary: Ary[pk.VarInt]{Ary: &values},
	}
	ary.SetParentContext(NewParentContext())
	ary.parentContext.(*SimpleParentContext).SetField("test", true)

	_, err := ary.WriteTo(buf)
	require.NoError(t, err)

	var decoded Array[pk.VarInt, mockContextElement]
	decoded.SetParentContext(NewParentContext())
	decoded.parentContext.(*SimpleParentContext).SetField("test", true)

	_, err = decoded.ReadFrom(buf)
	require.NoError(t, err)

	require.NotNil(t, decoded.Ary.Ary)

	var decodedValues []mockContextElement

	switch v := decoded.Ary.Ary.(type) {
	case []mockContextElement:
		// encoding case: pk.Ary is usually constructed with a plain slice
		decodedValues = v
	case *[]mockContextElement:
		// decoding case: pk.Ary is usually constructed with &slice
		decodedValues = *v
	default:
		require.NoError(t, fmt.Errorf("Array.Ary has unexpected type %T; want []pk.VarInt or *[]pk.VarInt", decoded.Ary.Ary))
	}

	require.Len(t, decodedValues, len(values))
	for _, elem := range decodedValues {
		assert.True(t, elem.seen, "%v should have been seen", elem.value)
	}
}
