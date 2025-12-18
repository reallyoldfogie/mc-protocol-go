package models

// PORTIONS OF THIS FILE COME FROM OR ARE BASED ON https://github.com/midnightfreddie/nbt2json

// Copyright (c) 2020 Jim Nelson

// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:

// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// NBTReader reads NBT data from an io.Reader
type NBTReader struct {
	reader    io.Reader
	byteOrder binary.ByteOrder
}

// NewNBTReader creates a new NBT reader with BigEndian byte order (Java Edition)
func NewNBTReader(reader io.Reader) *NBTReader {
	return &NBTReader{
		reader:    reader,
		byteOrder: binary.BigEndian,
	}
}

// ReadTag reads a complete NBT tag (type + name + value)
func (nr *NBTReader) ReadTag() (*NBTTag, int64, error) {
	var tagType NBTType
	if err := binary.Read(nr.reader, nr.byteOrder, &tagType); err != nil {
		return nil, 0, NbtParseError{str: "Reading tag type", err: err}
	}
	bytesRead := int64(1)

	if tagType == TypeEnd {
		return nil, bytesRead, NbtParseError{str: "Unexpected TAG_End", err: nil}
	}

	// Read tag name
	var nameLen int16
	if err := binary.Read(nr.reader, nr.byteOrder, &nameLen); err != nil {
		return nil, bytesRead, NbtParseError{str: "Reading tag name length", err: err}
	}
	bytesRead += 2

	nameBytes := make([]byte, nameLen)
	if err := binary.Read(nr.reader, nr.byteOrder, &nameBytes); err != nil {
		return nil, bytesRead, NbtParseError{str: fmt.Sprintf("Reading tag name (length=%d)", nameLen), err: err}
	}
	bytesRead += int64(nameLen)

	value, n, err := nr.ReadValue(tagType)
	if err != nil {
		return nil, bytesRead, err
	}
	bytesRead += n

	return &NBTTag{
		Name:  string(nameBytes),
		Value: value,
	}, bytesRead, nil
}

// ReadValue reads an NBT value of the specified type
func (nr *NBTReader) ReadValue(tagType NBTType) (NBTValue, int64, error) {
	switch tagType {
	case TypeByte:
		v, err := nr.readByte()
		return v, 1, err
	case TypeShort:
		v, err := nr.readShort()
		return v, 2, err
	case TypeInt:
		v, err := nr.readInt()
		return v, 4, err
	case TypeLong:
		v, err := nr.readLong()
		return v, 8, err
	case TypeFloat:
		v, err := nr.readFloat()
		return v, 4, err
	case TypeDouble:
		v, err := nr.readDouble()
		return v, 8, err
	case TypeByteArray:
		ba, err := nr.readByteArray()
		if err != nil {
			return nil, 0, err
		}
		return ba, int64(4 + len(ba.Value)), nil
	case TypeString:
		s, err := nr.readString()
		if err != nil {
			return nil, 0, err
		}
		return s, int64(2 + len(s.Value)), nil
	case TypeList:
		l, n, err := nr.readList()
		return l, n, err
	case TypeCompound:
		c, n, err := nr.readCompound()
		return c, n, err
	case TypeIntArray:
		ia, err := nr.readIntArray()
		if err != nil {
			return nil, 0, err
		}
		return ia, int64(4 + len(ia.Value)*4), nil
	case TypeLongArray:
		la, err := nr.readLongArray()
		if err != nil {
			return nil, 0, err
		}
		return la, int64(4 + len(la.Value)*8), nil
	default:
		return nil, 0, NbtParseError{str: fmt.Sprintf("Unknown tag type: %d", tagType), err: nil}
	}
}

func (nr *NBTReader) readByte() (*NBTByte, error) {
	var value int8
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading byte", err: err}
	}
	return &NBTByte{Value: value}, nil
}

func (nr *NBTReader) readShort() (*NBTShort, error) {
	var value int16
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading short", err: err}
	}
	return &NBTShort{Value: value}, nil
}

func (nr *NBTReader) readInt() (*NBTInt, error) {
	var value int32
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading int", err: err}
	}
	return &NBTInt{Value: value}, nil
}

func (nr *NBTReader) readLong() (*NBTLong, error) {
	var value int64
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading long", err: err}
	}
	return &NBTLong{Value: value}, nil
}

func (nr *NBTReader) readFloat() (*NBTFloat, error) {
	var value float32
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading float", err: err}
	}
	return &NBTFloat{Value: value}, nil
}

func (nr *NBTReader) readDouble() (*NBTDouble, error) {
	var value float64
	if err := binary.Read(nr.reader, nr.byteOrder, &value); err != nil {
		return nil, NbtParseError{str: "Reading double", err: err}
	}
	return &NBTDouble{Value: value}, nil
}

func (nr *NBTReader) readByteArray() (*NBTByteArray, error) {
	var length int32
	if err := binary.Read(nr.reader, nr.byteOrder, &length); err != nil {
		return nil, NbtParseError{str: "Reading byte array length", err: err}
	}

	bytes := make([]int8, length)
	if err := binary.Read(nr.reader, nr.byteOrder, &bytes); err != nil {
		return nil, NbtParseError{str: "Reading byte array data", err: err}
	}

	return &NBTByteArray{Value: bytes}, nil
}

func (nr *NBTReader) readString() (*NBTString, error) {
	var length int16
	if err := binary.Read(nr.reader, nr.byteOrder, &length); err != nil {
		return nil, NbtParseError{str: "Reading string length", err: err}
	}

	strBytes := make([]byte, length)
	if err := binary.Read(nr.reader, nr.byteOrder, &strBytes); err != nil {
		return nil, NbtParseError{str: "Reading string data", err: err}
	}

	return &NBTString{Value: string(strBytes)}, nil
}

func (nr *NBTReader) readList() (*NBTList, int64, error) {
	var listType NBTType
	if err := binary.Read(nr.reader, nr.byteOrder, &listType); err != nil {
		return nil, 0, NbtParseError{str: "Reading list type", err: err}
	}
	bytesRead := int64(1)

	var length int32
	if err := binary.Read(nr.reader, nr.byteOrder, &length); err != nil {
		return nil, bytesRead, NbtParseError{str: "Reading list length", err: err}
	}
	bytesRead += 4

	values := make([]NBTValue, 0, length)
	for range length {
		value, n, err := nr.ReadValue(listType)
		if err != nil {
			return nil, bytesRead, NbtParseError{str: "Reading list item", err: err}
		}
		bytesRead += n
		values = append(values, value)
	}

	return &NBTList{
		ListType: listType,
		Values:   values,
	}, bytesRead, nil
}

func (nr *NBTReader) readCompound() (*NBTCompound, int64, error) {
	tags := make([]NBTTag, 0)
	total := int64(0)

	for {
		var tagType NBTType
		if err := binary.Read(nr.reader, nr.byteOrder, &tagType); err != nil {
			return nil, total, NbtParseError{str: "Reading compound tag type", err: err}
		}
		total += 1

		if tagType == TypeEnd {
			break
		}

		// Read tag name
		var nameLen int16
		if err := binary.Read(nr.reader, nr.byteOrder, &nameLen); err != nil {
			return nil, total, NbtParseError{str: "Reading compound tag name length", err: err}
		}
		total += 2

		nameBytes := make([]byte, nameLen)
		if err := binary.Read(nr.reader, nr.byteOrder, &nameBytes); err != nil {
			return nil, total, NbtParseError{str: "Reading compound tag name", err: err}
		}
		total += int64(nameLen)

		value, n, err := nr.ReadValue(tagType)
		if err != nil {
			return nil, total, NbtParseError{str: "Reading compound tag value", err: err}
		}
		total += n

		tags = append(tags, NBTTag{
			Name:  string(nameBytes),
			Value: value,
		})
	}

	return &NBTCompound{Tags: tags}, total, nil
}

func (nr *NBTReader) readIntArray() (*NBTIntArray, error) {
	var length int32
	if err := binary.Read(nr.reader, nr.byteOrder, &length); err != nil {
		return nil, NbtParseError{str: "Reading int array length", err: err}
	}

	ints := make([]int32, length)
	if err := binary.Read(nr.reader, nr.byteOrder, &ints); err != nil {
		return nil, NbtParseError{str: "Reading int array data", err: err}
	}

	return &NBTIntArray{Value: ints}, nil
}

func (nr *NBTReader) readLongArray() (*NBTLongArray, error) {
	var length int32
	if err := binary.Read(nr.reader, nr.byteOrder, &length); err != nil {
		return nil, NbtParseError{str: "Reading long array length", err: err}
	}

	longs := make([]int64, length)
	if err := binary.Read(nr.reader, nr.byteOrder, &longs); err != nil {
		return nil, NbtParseError{str: "Reading long array data", err: err}
	}

	return &NBTLongArray{Value: longs}, nil
}

// ReadFrom implementations for each NBT type

func (b *NBTByte) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readByte()
	if err != nil {
		return 0, err
	}
	*b = *value
	return 1, nil
}

func (s *NBTShort) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readShort()
	if err != nil {
		return 0, err
	}
	*s = *value
	return 2, nil
}

func (i *NBTInt) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readInt()
	if err != nil {
		return 0, err
	}
	*i = *value
	return 4, nil
}

func (l *NBTLong) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readLong()
	if err != nil {
		return 0, err
	}
	*l = *value
	return 8, nil
}

func (f *NBTFloat) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readFloat()
	if err != nil {
		return 0, err
	}
	*f = *value
	return 4, nil
}

func (d *NBTDouble) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readDouble()
	if err != nil {
		return 0, err
	}
	*d = *value
	return 8, nil
}

func (ba *NBTByteArray) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readByteArray()
	if err != nil {
		return 0, err
	}
	*ba = *value
	return int64(4 + len(value.Value)), nil
}

func (s *NBTString) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readString()
	if err != nil {
		return 0, err
	}
	*s = *value
	return int64(2 + len(value.Value)), nil
}

func (l *NBTList) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, n, err := nr.readList()
	if err != nil {
		return 0, err
	}
	*l = *value
	return n, nil
}

func (c *NBTCompound) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, n, err := nr.readCompound()
	if err != nil {
		return 0, err
	}
	*c = *value
	return n, nil
}

func (ia *NBTIntArray) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readIntArray()
	if err != nil {
		return 0, err
	}
	*ia = *value
	return int64(4 + len(value.Value)*4), nil
}

func (la *NBTLongArray) ReadFrom(reader io.Reader) (int64, error) {
	nr := NewNBTReader(reader)
	value, err := nr.readLongArray()
	if err != nil {
		return 0, err
	}
	*la = *value
	return int64(4 + len(value.Value)*8), nil
}

// WriteTo implementations for each NBT type

func (b NBTByte) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, b.Value); err != nil {
		return 0, err
	}
	return 1, nil
}

func (s NBTShort) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, s.Value); err != nil {
		return 0, err
	}
	return 2, nil
}

func (i NBTInt) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, i.Value); err != nil {
		return 0, err
	}
	return 4, nil
}

func (l NBTLong) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, l.Value); err != nil {
		return 0, err
	}
	return 8, nil
}

func (f NBTFloat) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, f.Value); err != nil {
		return 0, err
	}
	return 4, nil
}

func (d NBTDouble) WriteTo(writer io.Writer) (int64, error) {
	if math.IsNaN(d.Value) {
		// Write the bit pattern for NaN
		if err := binary.Write(writer, binary.BigEndian, math.Float64bits(d.Value)); err != nil {
			return 0, err
		}
	} else {
		if err := binary.Write(writer, binary.BigEndian, d.Value); err != nil {
			return 0, err
		}
	}
	return 8, nil
}

func (ba NBTByteArray) WriteTo(writer io.Writer) (int64, error) {
	length := int32(len(ba.Value))
	if err := binary.Write(writer, binary.BigEndian, length); err != nil {
		return 0, err
	}
	if err := binary.Write(writer, binary.BigEndian, ba.Value); err != nil {
		return int64(4), err
	}
	return int64(4 + len(ba.Value)), nil
}

func (s NBTString) WriteTo(writer io.Writer) (int64, error) {
	strBytes := []byte(s.Value)
	length := int16(len(strBytes))
	if err := binary.Write(writer, binary.BigEndian, length); err != nil {
		return 0, err
	}
	nn, err := writer.Write(strBytes)
	return int64(2 + nn), err
}

func (l NBTList) WriteTo(writer io.Writer) (int64, error) {
	if err := binary.Write(writer, binary.BigEndian, l.ListType); err != nil {
		return 0, err
	}
	length := int32(len(l.Values))
	if err := binary.Write(writer, binary.BigEndian, length); err != nil {
		return 1, err
	}

	bytesWritten := int64(1 + 4)
	for _, value := range l.Values {
		nn, err := value.WriteTo(writer)
		if err != nil {
			return bytesWritten, err
		}
		bytesWritten += nn
	}

	return bytesWritten, nil
}

func (c NBTCompound) WriteTo(writer io.Writer) (int64, error) {
	bytesWritten := int64(0)

	for _, tag := range c.Tags {
		// Write tag type
		if err := binary.Write(writer, binary.BigEndian, tag.Value.Type()); err != nil {
			return bytesWritten, err
		}
		bytesWritten++

		// Write tag name
		nameBytes := []byte(tag.Name)
		nameLen := int16(len(nameBytes))
		if err := binary.Write(writer, binary.BigEndian, nameLen); err != nil {
			return bytesWritten, err
		}
		bytesWritten += 2

		nn, err := writer.Write(nameBytes)
		if err != nil {
			return bytesWritten, err
		}
		bytesWritten += int64(nn)

		// Write tag value
		nn2, err := tag.Value.WriteTo(writer)
		if err != nil {
			return bytesWritten, err
		}
		bytesWritten += nn2
	}

	// Write TAG_End
	if err := binary.Write(writer, binary.BigEndian, TypeEnd); err != nil {
		return bytesWritten, err
	}
	bytesWritten++

	return bytesWritten, nil
}

func (ia NBTIntArray) WriteTo(writer io.Writer) (int64, error) {
	length := int32(len(ia.Value))
	if err := binary.Write(writer, binary.BigEndian, length); err != nil {
		return 0, err
	}
	if err := binary.Write(writer, binary.BigEndian, ia.Value); err != nil {
		return 4, err
	}
	return int64(4 + len(ia.Value)*4), nil
}

func (la NBTLongArray) WriteTo(writer io.Writer) (int64, error) {
	length := int32(len(la.Value))
	if err := binary.Write(writer, binary.BigEndian, length); err != nil {
		return 0, err
	}
	if err := binary.Write(writer, binary.BigEndian, la.Value); err != nil {
		return 4, err
	}
	return int64(4 + len(la.Value)*8), nil
}
