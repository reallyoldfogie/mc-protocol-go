package models

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnonymousNBT_ReadWrite_Compound(t *testing.T) {
	// Test with compound tag (the original NBTField behavior)
	SetCurrentNBTVersion("1.21.5")

	original := &AnonymousNBT{
		TagType: TypeCompound,
		Value: &NBTCompound{
			Tags: []NBTTag{
				{Name: "test", Value: &NBTString{Value: "hello"}},
				{Name: "number", Value: &NBTInt{Value: 42}},
			},
		},
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	require.Greater(t, n, int64(0))

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeCompound, result.TagType)
	assert.NotNil(t, result.Value)

	// Verify the compound
	compound, ok := result.Value.(*NBTCompound)
	require.True(t, ok)
	assert.Len(t, compound.Tags, 2)
}

func TestAnonymousNBT_ReadWrite_Int(t *testing.T) {
	// Test with int tag (the failing case from the bug)
	SetCurrentNBTVersion("1.21.5")

	original := &AnonymousNBT{
		TagType: TypeInt,
		Value:   &NBTInt{Value: 123456},
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	require.Greater(t, n, int64(0))

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeInt, result.TagType)
	assert.NotNil(t, result.Value)

	// Verify the int value
	intVal, ok := result.Value.(*NBTInt)
	require.True(t, ok)
	assert.Equal(t, int32(123456), intVal.Value)
}

func TestAnonymousNBT_ReadWrite_String(t *testing.T) {
	// Test with string tag
	SetCurrentNBTVersion("1.21.5")

	original := &AnonymousNBT{
		TagType: TypeString,
		Value:   &NBTString{Value: "test string"},
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	require.Greater(t, n, int64(0))

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeString, result.TagType)
	assert.NotNil(t, result.Value)

	// Verify the string value
	strVal, ok := result.Value.(*NBTString)
	require.True(t, ok)
	assert.Equal(t, "test string", strVal.Value)
}

func TestAnonymousNBT_ReadWrite_List(t *testing.T) {
	// Test with list tag
	SetCurrentNBTVersion("1.21.5")

	original := &AnonymousNBT{
		TagType: TypeList,
		Value: &NBTList{
			ListType: TypeInt,
			Values: []NBTValue{
				&NBTInt{Value: 1},
				&NBTInt{Value: 2},
				&NBTInt{Value: 3},
			},
		},
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	require.Greater(t, n, int64(0))

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeList, result.TagType)
	assert.NotNil(t, result.Value)

	// Verify the list
	listVal, ok := result.Value.(*NBTList)
	require.True(t, ok)
	assert.Equal(t, TypeInt, listVal.ListType)
	assert.Len(t, listVal.Values, 3)
}

func TestAnonymousNBT_ReadWrite_Empty(t *testing.T) {
	// Test with empty NBT (TAG_End)
	SetCurrentNBTVersion("1.21.5")

	original := &AnonymousNBT{
		TagType: TypeEnd,
		Value:   nil,
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n) // Just the TAG_End byte

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeEnd, result.TagType)
	assert.Nil(t, result.Value)
}

func TestAnonymousNBT_ReadWrite_AllTypes(t *testing.T) {
	// Test all NBT types to ensure comprehensive support
	SetCurrentNBTVersion("1.21.5")

	testCases := []struct {
		name    string
		tagType NBTType
		value   NBTValue
	}{
		{"Byte", TypeByte, &NBTByte{Value: 127}},
		{"Short", TypeShort, &NBTShort{Value: 32767}},
		{"Int", TypeInt, &NBTInt{Value: 2147483647}},
		{"Long", TypeLong, &NBTLong{Value: 9223372036854775807}},
		{"Float", TypeFloat, &NBTFloat{Value: 3.14159}},
		{"Double", TypeDouble, &NBTDouble{Value: 2.718281828}},
		{"ByteArray", TypeByteArray, &NBTByteArray{Value: []int8{1, 2, 3, 4, 5}}},
		{"String", TypeString, &NBTString{Value: "hello world"}},
		{"IntArray", TypeIntArray, &NBTIntArray{Value: []int32{100, 200, 300}}},
		{"LongArray", TypeLongArray, &NBTLongArray{Value: []int64{1000, 2000, 3000}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := &AnonymousNBT{
				TagType: tc.tagType,
				Value:   tc.value,
			}

			// Write
			buf := &bytes.Buffer{}
			n, err := original.WriteTo(buf)
			require.NoError(t, err)
			require.Greater(t, n, int64(0))

			// Read
			result := &AnonymousNBT{}
			n2, err := result.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, n, n2)
			assert.Equal(t, tc.tagType, result.TagType)
			assert.NotNil(t, result.Value)
			assert.Equal(t, tc.tagType, result.Value.Type())
		})
	}
}

func TestAnonymousNBT_Version_Pre1_20_5(t *testing.T) {
	// Test with pre-1.20.5 version that includes name field
	SetCurrentNBTVersion("1.20.4")

	original := &AnonymousNBT{
		TagType: TypeString,
		Value:   &NBTString{Value: "test"},
	}

	// Write
	buf := &bytes.Buffer{}
	n, err := original.WriteTo(buf)
	require.NoError(t, err)
	// Should be: 1 byte (type) + 2 bytes (name length) + 2 bytes (string length) + 4 bytes ("test")
	require.Greater(t, n, int64(5))

	// Read
	result := &AnonymousNBT{}
	n2, err := result.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
	assert.Equal(t, TypeString, result.TagType)

	strVal, ok := result.Value.(*NBTString)
	require.True(t, ok)
	assert.Equal(t, "test", strVal.Value)
}

func TestNewAnonymousNBT(t *testing.T) {
	// Test constructor
	anbt := NewAnonymousNBT()
	assert.NotNil(t, anbt)
	assert.Equal(t, TypeCompound, anbt.TagType)
	assert.NotNil(t, anbt.Value)

	compound, ok := anbt.Value.(*NBTCompound)
	require.True(t, ok)
	assert.NotNil(t, compound.Tags)
}

func TestNewAnonymousNBTWithValue(t *testing.T) {
	// Test constructor with value
	intVal := &NBTInt{Value: 999}
	anbt := NewAnonymousNBTWithValue(intVal)
	assert.NotNil(t, anbt)
	assert.Equal(t, TypeInt, anbt.TagType)
	assert.Equal(t, intVal, anbt.Value)
}
