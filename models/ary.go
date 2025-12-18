package models

// This file contains a copy of the Ary type from github.com/Tnze/go-mc/net/packet
// with a critical bug fix. Both projects use the MIT license.
//
// Original source: https://github.com/Tnze/go-mc/blob/master/net/packet/util.go
//
// Bug fix: Line 75 in the original code called array.Slice() without assigning
// the result back to the array variable, causing the array to retain its original
// size instead of being resized to the length read from the packet. This caused
// the loop to iterate more times than intended, leading to EOF errors.
//
// Fixed by changing:
//     array.Slice(0, int(Len))
// to:
//     array.Set(array.Slice(0, int(Len)))

import (
	"errors"
	"io"
	"reflect"

	pk "github.com/Tnze/go-mc/net/packet"
)

// Ary is used to send or receive the packet field like "Array of X"
// which has a count must be known from the context.
//
// Typically, you must decode an integer representing the length. Then
// receive the corresponding amount of data according to the length.
// In this case, the field Len should be a pointer of integer type so
// the value can be updating when Packet.Scan() method is decoding the
// previous field.
// In some special cases, you might want to read an "Array of X" with a fix length.
// So it's allowed to directly set an integer type Len, but not a pointer.
//
// Note that Ary DO read or write the Len. You aren't need to do so by your self.
type Ary[LEN pk.VarInt | pk.VarLong | pk.Byte | pk.UnsignedByte | pk.Short | pk.UnsignedShort | pk.Int | pk.Long] struct {
	Ary any // Slice or Pointer of Slice of FieldEncoder, FieldDecoder or both (Field)
}

func (a Ary[LEN]) WriteTo(w io.Writer) (n int64, err error) {
	array := reflect.ValueOf(a.Ary)
	for array.Kind() == reflect.Ptr {
		array = array.Elem()
	}
	Len := LEN(array.Len())
	if nn, err := any(&Len).(pk.FieldEncoder).WriteTo(w); err != nil {
		return n, err
	} else {
		n += nn
	}
	for i := 0; i < array.Len(); i++ {
		elem := array.Index(i)
		nn, err := elem.Interface().(pk.FieldEncoder).WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (a Ary[LEN]) ReadFrom(r io.Reader) (n int64, err error) {
	var Len LEN
	if nn, err := any(&Len).(pk.FieldDecoder).ReadFrom(r); err != nil {
		return nn, err
	} else {
		n += nn
	}
	if Len < 0 {
		return n, errors.New("array length less than zero")
	}

	array := reflect.ValueOf(a.Ary)
	for array.Kind() == reflect.Ptr {
		array = array.Elem()
	}
	if !array.CanAddr() {
		panic(errors.New("the contents of the Ary are not addressable"))
	}
	if array.Cap() < int(Len) {
		array.Set(reflect.MakeSlice(array.Type(), int(Len), int(Len)))
	} else {
		// BUG FIX: The original go-mc code was missing the .Set() call here
		array.Set(array.Slice(0, int(Len)))
	}
	for i := 0; i < int(Len); i++ {
		elem := array.Index(i)
		nn, err := elem.Addr().Interface().(pk.FieldDecoder).ReadFrom(r)
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, err
}
