package models

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNBTByte(t *testing.T) {
	tests := []struct {
		name  string
		value int8
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -42},
		{"max", 127},
		{"min", -128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtByte := NBTByte{Value: tt.value}

			// Test WriteTo
			buf := &bytes.Buffer{}
			nn, err := nbtByte.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(1), nn)

			// Test ReadFrom
			var readByte NBTByte
			nn2, err := readByte.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(1), nn2)
			assert.Equal(t, tt.value, readByte.Value)
			assert.Equal(t, TypeByte, readByte.Type())
		})
	}
}

func TestNBTShort(t *testing.T) {
	tests := []struct {
		name  string
		value int16
	}{
		{"zero", 0},
		{"positive", 12345},
		{"negative", -12345},
		{"max", 32767},
		{"min", -32768},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtShort := NBTShort{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtShort.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(2), nn)

			var readShort NBTShort
			nn2, err := readShort.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(2), nn2)
			assert.Equal(t, tt.value, readShort.Value)
			assert.Equal(t, TypeShort, readShort.Type())
		})
	}
}

func TestNBTInt(t *testing.T) {
	tests := []struct {
		name  string
		value int32
	}{
		{"zero", 0},
		{"positive", 123456789},
		{"negative", -123456789},
		{"max", 2147483647},
		{"min", -2147483648},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtInt := NBTInt{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtInt.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4), nn)

			var readInt NBTInt
			nn2, err := readInt.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4), nn2)
			assert.Equal(t, tt.value, readInt.Value)
			assert.Equal(t, TypeInt, readInt.Type())
		})
	}
}

func TestNBTLong(t *testing.T) {
	tests := []struct {
		name  string
		value int64
	}{
		{"zero", 0},
		{"positive", 9876543210},
		{"negative", -9876543210},
		{"max", 9223372036854775807},
		{"min", -9223372036854775808},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtLong := NBTLong{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtLong.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(8), nn)

			var readLong NBTLong
			nn2, err := readLong.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(8), nn2)
			assert.Equal(t, tt.value, readLong.Value)
			assert.Equal(t, TypeLong, readLong.Type())
		})
	}
}

func TestNBTFloat(t *testing.T) {
	tests := []struct {
		name  string
		value float32
	}{
		{"zero", 0.0},
		{"positive", 3.14159},
		{"negative", -3.14159},
		{"max", math.MaxFloat32},
		{"small", 1.175494351e-38},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtFloat := NBTFloat{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtFloat.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4), nn)

			var readFloat NBTFloat
			nn2, err := readFloat.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4), nn2)
			assert.InDelta(t, tt.value, readFloat.Value, 0.0001)
			assert.Equal(t, TypeFloat, readFloat.Type())
		})
	}
}

func TestNBTDouble(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{"zero", 0.0},
		{"positive", 3.141592653589793},
		{"negative", -3.141592653589793},
		{"max", math.MaxFloat64},
		{"small", 2.2250738585072014e-308},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtDouble := NBTDouble{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtDouble.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(8), nn)

			var readDouble NBTDouble
			nn2, err := readDouble.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(8), nn2)
			assert.InDelta(t, tt.value, readDouble.Value, 0.000001)
			assert.Equal(t, TypeDouble, readDouble.Type())
		})
	}
}

func TestNBTDoubleNaN(t *testing.T) {
	nbtDouble := NBTDouble{Value: math.NaN()}

	buf := &bytes.Buffer{}
	nn, err := nbtDouble.WriteTo(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(8), nn)

	var readDouble NBTDouble
	nn2, err := readDouble.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, int64(8), nn2)
	assert.True(t, math.IsNaN(readDouble.Value))
}

func TestNBTString(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"simple", "hello"},
		{"with spaces", "hello world"},
		{"unicode", "Hello 世界 🌍"},
		{"long", "This is a longer string to test the NBT string encoding and decoding"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtString := NBTString{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtString.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(2+len(tt.value)), nn)

			var readString NBTString
			_, err = readString.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.value, readString.Value)
			assert.Equal(t, TypeString, readString.Type())
		})
	}
}

func TestNBTByteArray(t *testing.T) {
	tests := []struct {
		name  string
		value []int8
	}{
		{"empty", []int8{}},
		{"single", []int8{42}},
		{"multiple", []int8{1, 2, 3, 4, 5}},
		{"negative", []int8{-1, -2, -3}},
		{"mixed", []int8{-128, 0, 127}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtByteArray := NBTByteArray{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtByteArray.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4+len(tt.value)), nn)

			var readByteArray NBTByteArray
			_, err = readByteArray.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.value, readByteArray.Value)
			assert.Equal(t, TypeByteArray, readByteArray.Type())
		})
	}
}

func TestNBTIntArray(t *testing.T) {
	tests := []struct {
		name  string
		value []int32
	}{
		{"empty", []int32{}},
		{"single", []int32{12345}},
		{"multiple", []int32{1, 2, 3, 4, 5}},
		{"negative", []int32{-100, -200, -300}},
		{"large", []int32{2147483647, -2147483648}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtIntArray := NBTIntArray{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtIntArray.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4+len(tt.value)*4), nn)

			var readIntArray NBTIntArray
			_, err = readIntArray.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.value, readIntArray.Value)
			assert.Equal(t, TypeIntArray, readIntArray.Type())
		})
	}
}

func TestNBTLongArray(t *testing.T) {
	tests := []struct {
		name  string
		value []int64
	}{
		{"empty", []int64{}},
		{"single", []int64{9876543210}},
		{"multiple", []int64{1, 2, 3, 4, 5}},
		{"negative", []int64{-1000000, -2000000, -3000000}},
		{"large", []int64{9223372036854775807, -9223372036854775808}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbtLongArray := NBTLongArray{Value: tt.value}

			buf := &bytes.Buffer{}
			nn, err := nbtLongArray.WriteTo(buf)
			require.NoError(t, err)
			assert.Equal(t, int64(4+len(tt.value)*8), nn)

			var readLongArray NBTLongArray
			_, err = readLongArray.ReadFrom(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.value, readLongArray.Value)
			assert.Equal(t, TypeLongArray, readLongArray.Type())
		})
	}
}

func TestNBTList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		nbtList := NBTList{
			ListType: TypeInt,
			Values:   []NBTValue{},
		}

		buf := &bytes.Buffer{}
		nn, err := nbtList.WriteTo(buf)
		require.NoError(t, err)
		assert.Equal(t, int64(5), nn) // 1 byte type + 4 bytes length

		var readList NBTList
		_, err = readList.ReadFrom(buf)
		require.NoError(t, err)
		assert.Equal(t, TypeInt, readList.ListType)
		assert.Empty(t, readList.Values)
		assert.Equal(t, TypeList, readList.Type())
	})

	t.Run("list of ints", func(t *testing.T) {
		nbtList := NBTList{
			ListType: TypeInt,
			Values: []NBTValue{
				&NBTInt{Value: 100},
				&NBTInt{Value: 200},
				&NBTInt{Value: 300},
			},
		}

		buf := &bytes.Buffer{}
		_, err := nbtList.WriteTo(buf)
		require.NoError(t, err)

		var readList NBTList
		_, err = readList.ReadFrom(buf)
		require.NoError(t, err)
		assert.Equal(t, TypeInt, readList.ListType)
		assert.Len(t, readList.Values, 3)
		assert.Equal(t, int32(100), readList.Values[0].(*NBTInt).Value)
		assert.Equal(t, int32(200), readList.Values[1].(*NBTInt).Value)
		assert.Equal(t, int32(300), readList.Values[2].(*NBTInt).Value)
	})

	t.Run("list of strings", func(t *testing.T) {
		nbtList := NBTList{
			ListType: TypeString,
			Values: []NBTValue{
				&NBTString{Value: "alpha"},
				&NBTString{Value: "beta"},
				&NBTString{Value: "gamma"},
			},
		}

		buf := &bytes.Buffer{}
		_, err := nbtList.WriteTo(buf)
		require.NoError(t, err)

		var readList NBTList
		_, err = readList.ReadFrom(buf)
		require.NoError(t, err)
		assert.Equal(t, TypeString, readList.ListType)
		assert.Len(t, readList.Values, 3)
		assert.Equal(t, "alpha", readList.Values[0].(*NBTString).Value)
		assert.Equal(t, "beta", readList.Values[1].(*NBTString).Value)
		assert.Equal(t, "gamma", readList.Values[2].(*NBTString).Value)
	})
}

func TestNBTCompound(t *testing.T) {
	t.Run("empty compound", func(t *testing.T) {
		nbtCompound := NBTCompound{
			Tags: []NBTTag{},
		}

		buf := &bytes.Buffer{}
		nn, err := nbtCompound.WriteTo(buf)
		require.NoError(t, err)
		assert.Equal(t, int64(1), nn) // Just TAG_End

		var readCompound NBTCompound
		_, err = readCompound.ReadFrom(buf)
		require.NoError(t, err)
		assert.Empty(t, readCompound.Tags)
		assert.Equal(t, TypeCompound, readCompound.Type())
	})

	t.Run("simple compound", func(t *testing.T) {
		nbtCompound := NBTCompound{
			Tags: []NBTTag{
				{Name: "age", Value: &NBTInt{Value: 25}},
				{Name: "name", Value: &NBTString{Value: "Steve"}},
				{Name: "height", Value: &NBTFloat{Value: 1.8}},
			},
		}

		buf := &bytes.Buffer{}
		_, err := nbtCompound.WriteTo(buf)
		require.NoError(t, err)

		var readCompound NBTCompound
		_, err = readCompound.ReadFrom(buf)
		require.NoError(t, err)
		assert.Len(t, readCompound.Tags, 3)

		// Check Get method
		ageVal, ok := readCompound.Get("age")
		require.True(t, ok)
		assert.Equal(t, int32(25), ageVal.(*NBTInt).Value)

		nameVal, ok := readCompound.Get("name")
		require.True(t, ok)
		assert.Equal(t, "Steve", nameVal.(*NBTString).Value)

		heightVal, ok := readCompound.Get("height")
		require.True(t, ok)
		assert.InDelta(t, 1.8, heightVal.(*NBTFloat).Value, 0.01)

		// Test non-existent key
		_, ok = readCompound.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("nested compound", func(t *testing.T) {
		innerCompound := NBTCompound{
			Tags: []NBTTag{
				{Name: "x", Value: &NBTInt{Value: 100}},
				{Name: "y", Value: &NBTInt{Value: 200}},
			},
		}

		outerCompound := NBTCompound{
			Tags: []NBTTag{
				{Name: "position", Value: &innerCompound},
				{Name: "name", Value: &NBTString{Value: "player"}},
			},
		}

		buf := &bytes.Buffer{}
		_, err := outerCompound.WriteTo(buf)
		require.NoError(t, err)

		var readCompound NBTCompound
		_, err = readCompound.ReadFrom(buf)
		require.NoError(t, err)

		posVal, ok := readCompound.Get("position")
		require.True(t, ok)

		posCompound := posVal.(*NBTCompound)
		xVal, ok := posCompound.Get("x")
		require.True(t, ok)
		assert.Equal(t, int32(100), xVal.(*NBTInt).Value)
	})
}

func TestNBTCompoundSet(t *testing.T) {
	compound := NBTCompound{Tags: []NBTTag{}}

	// Add new tag
	compound.Set("name", &NBTString{Value: "test"})
	val, ok := compound.Get("name")
	require.True(t, ok)
	assert.Equal(t, "test", val.(*NBTString).Value)

	// Update existing tag
	compound.Set("name", &NBTString{Value: "updated"})
	val, ok = compound.Get("name")
	require.True(t, ok)
	assert.Equal(t, "updated", val.(*NBTString).Value)
	assert.Len(t, compound.Tags, 1)

	// Add another tag
	compound.Set("age", &NBTInt{Value: 42})
	assert.Len(t, compound.Tags, 2)
}

func TestNBTReaderReadTag(t *testing.T) {
	// Create a buffer with a complete NBT tag
	buf := &bytes.Buffer{}

	// Write tag type (INT)
	buf.WriteByte(byte(TypeInt))
	// Write name length
	buf.WriteByte(0)
	buf.WriteByte(4) // length 4
	// Write name
	buf.WriteString("test")
	// Write value
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(42)

	reader := NewNBTReader(buf)
	tag, err := reader.ReadTag()
	require.NoError(t, err)
	assert.Equal(t, "test", tag.Name)
	assert.Equal(t, TypeInt, tag.Value.Type())
	assert.Equal(t, int32(42), tag.Value.(*NBTInt).Value)
}

func TestNBTReaderComplexStructure(t *testing.T) {
	// Build a complex NBT structure similar to what Minecraft uses
	playerData := NBTCompound{
		Tags: []NBTTag{
			{Name: "playerName", Value: &NBTString{Value: "Steve"}},
			{Name: "health", Value: &NBTFloat{Value: 20.0}},
			{Name: "foodLevel", Value: &NBTInt{Value: 20}},
			{Name: "inventory", Value: &NBTList{
				ListType: TypeCompound,
				Values: []NBTValue{
					&NBTCompound{
						Tags: []NBTTag{
							{Name: "id", Value: &NBTString{Value: "minecraft:diamond_sword"}},
							{Name: "Count", Value: &NBTByte{Value: 1}},
						},
					},
					&NBTCompound{
						Tags: []NBTTag{
							{Name: "id", Value: &NBTString{Value: "minecraft:bread"}},
							{Name: "Count", Value: &NBTByte{Value: 64}},
						},
					},
				},
			}},
		},
	}

	buf := &bytes.Buffer{}
	_, err := playerData.WriteTo(buf)
	require.NoError(t, err)

	var readData NBTCompound
	_, err = readData.ReadFrom(buf)
	require.NoError(t, err)

	// Verify player name
	nameVal, ok := readData.Get("playerName")
	require.True(t, ok)
	assert.Equal(t, "Steve", nameVal.(*NBTString).Value)

	// Verify health
	healthVal, ok := readData.Get("health")
	require.True(t, ok)
	assert.InDelta(t, 20.0, healthVal.(*NBTFloat).Value, 0.01)

	// Verify inventory
	invVal, ok := readData.Get("inventory")
	require.True(t, ok)
	invList := invVal.(*NBTList)
	assert.Len(t, invList.Values, 2)

	// Check first item
	firstItem := invList.Values[0].(*NBTCompound)
	idVal, ok := firstItem.Get("id")
	require.True(t, ok)
	assert.Equal(t, "minecraft:diamond_sword", idVal.(*NBTString).Value)
}

func TestNBTErrorHandling(t *testing.T) {
	t.Run("truncated byte", func(t *testing.T) {
		buf := &bytes.Buffer{}
		var nbtByte NBTByte
		_, err := nbtByte.ReadFrom(buf)
		assert.Error(t, err)
	})

	t.Run("truncated string length", func(t *testing.T) {
		buf := &bytes.Buffer{}
		buf.WriteByte(0) // Only 1 byte of length
		var nbtString NBTString
		_, err := nbtString.ReadFrom(buf)
		assert.Error(t, err)
	})

	t.Run("truncated string data", func(t *testing.T) {
		buf := &bytes.Buffer{}
		buf.WriteByte(0)
		buf.WriteByte(10) // Length 10 but no data
		var nbtString NBTString
		_, err := nbtString.ReadFrom(buf)
		assert.Error(t, err)
	})
}

func TestNBTField(t *testing.T) {
	t.Run("empty NBT field", func(t *testing.T) {
		nbtField := NBTField{Value: nil}

		buf := &bytes.Buffer{}
		nn, err := nbtField.WriteTo(buf)
		require.NoError(t, err)
		assert.Equal(t, int64(1), nn) // Just TAG_End
		assert.Equal(t, byte(TypeEnd), buf.Bytes()[0])

		var readField NBTField
		_, err = readField.ReadFrom(buf)
		require.NoError(t, err)
		assert.Nil(t, readField.Value)
	})

	t.Run("NBT field with data", func(t *testing.T) {
		compound := &NBTCompound{
			Tags: []NBTTag{
				{Name: "name", Value: &NBTString{Value: "TestPlayer"}},
				{Name: "level", Value: &NBTInt{Value: 42}},
			},
		}

		nbtField := NBTField{Value: compound}

		buf := &bytes.Buffer{}
		_, err := nbtField.WriteTo(buf)
		require.NoError(t, err)

		var readField NBTField
		_, err = readField.ReadFrom(buf)
		require.NoError(t, err)
		require.NotNil(t, readField.Value)

		nameVal, ok := readField.Value.Get("name")
		require.True(t, ok)
		assert.Equal(t, "TestPlayer", nameVal.(*NBTString).Value)

		levelVal, ok := readField.Value.Get("level")
		require.True(t, ok)
		assert.Equal(t, int32(42), levelVal.(*NBTInt).Value)
	})

	t.Run("NBT field constructors", func(t *testing.T) {
		field1 := NewNBTField()
		require.NotNil(t, field1.Value)
		assert.Empty(t, field1.Value.Tags)

		compound := &NBTCompound{
			Tags: []NBTTag{
				{Name: "test", Value: &NBTInt{Value: 123}},
			},
		}
		field2 := NewNBTFieldWithCompound(compound)
		require.NotNil(t, field2.Value)
		assert.Len(t, field2.Value.Tags, 1)
	})
}
