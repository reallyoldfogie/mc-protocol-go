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
	"io"
)

// NBTType represents the type of an NBT tag
type NBTType byte

const (
	TypeEnd       NBTType = 0
	TypeByte      NBTType = 1
	TypeShort     NBTType = 2
	TypeInt       NBTType = 3
	TypeLong      NBTType = 4
	TypeFloat     NBTType = 5
	TypeDouble    NBTType = 6
	TypeByteArray NBTType = 7
	TypeString    NBTType = 8
	TypeList      NBTType = 9
	TypeCompound  NBTType = 10
	TypeIntArray  NBTType = 11
	TypeLongArray NBTType = 12
)

// NBTValue represents any NBT value that can be read or written
type NBTValue interface {
	Type() NBTType
	io.WriterTo
	io.ReaderFrom
}

// NBTTag represents a named NBT tag with a type and value
type NBTTag struct {
	Name  string
	Value NBTValue
}

// NBTByte represents an NBT byte (int8)
type NBTByte struct {
	Value int8
}

func (b NBTByte) Type() NBTType {
	return TypeByte
}

// NBTShort represents an NBT short (int16)
type NBTShort struct {
	Value int16
}

func (s NBTShort) Type() NBTType {
	return TypeShort
}

// NBTInt represents an NBT int (int32)
type NBTInt struct {
	Value int32
}

func (i NBTInt) Type() NBTType {
	return TypeInt
}

// NBTLong represents an NBT long (int64)
type NBTLong struct {
	Value int64
}

func (l NBTLong) Type() NBTType {
	return TypeLong
}

// NBTFloat represents an NBT float (float32)
type NBTFloat struct {
	Value float32
}

func (f NBTFloat) Type() NBTType {
	return TypeFloat
}

// NBTDouble represents an NBT double (float64)
type NBTDouble struct {
	Value float64
}

func (d NBTDouble) Type() NBTType {
	return TypeDouble
}

// NBTByteArray represents an NBT byte array ([]int8)
type NBTByteArray struct {
	Value []int8
}

func (ba NBTByteArray) Type() NBTType {
	return TypeByteArray
}

// NBTString represents an NBT string
type NBTString struct {
	Value string
}

func (s NBTString) Type() NBTType {
	return TypeString
}

// NBTList represents an NBT list of homogeneous values
type NBTList struct {
	ListType NBTType
	Values   []NBTValue
}

func (l NBTList) Type() NBTType {
	return TypeList
}

// NBTCompound represents an NBT compound tag (map of named tags)
type NBTCompound struct {
	Tags []NBTTag
}

func (c NBTCompound) Type() NBTType {
	return TypeCompound
}

// Get retrieves a tag value by name from the compound
func (c NBTCompound) Get(name string) (NBTValue, bool) {
	for _, tag := range c.Tags {
		if tag.Name == name {
			return tag.Value, true
		}
	}
	return nil, false
}

// Set adds or updates a tag in the compound
func (c *NBTCompound) Set(name string, value NBTValue) {
	for idx := range c.Tags {
		if c.Tags[idx].Name == name {
			c.Tags[idx].Value = value
			return
		}
	}
	c.Tags = append(c.Tags, NBTTag{Name: name, Value: value})
}

// NBTIntArray represents an NBT int array ([]int32)
type NBTIntArray struct {
	Value []int32
}

func (ia NBTIntArray) Type() NBTType {
	return TypeIntArray
}

// NBTLongArray represents an NBT long array ([]int64)
type NBTLongArray struct {
	Value []int64
}

func (la NBTLongArray) Type() NBTType {
	return TypeLongArray
}
