package models

import (
	"fmt"
	"io"
	"reflect"

	pk "github.com/Tnze/go-mc/net/packet"
)

type Array[LENTYPE pk.VarInt | pk.VarLong | pk.Byte | pk.UnsignedByte | pk.Short | pk.UnsignedShort | pk.Int | pk.Long, VALTYPE any] struct {
	Ary           Ary[LENTYPE]
	parentContext ParentContext
}

func (a *Array[LENTYPE, VALTYPE]) ReadFrom(r io.Reader) (int64, error) {
	val := []VALTYPE{}
	a.Ary = Ary[LENTYPE]{Ary: &val}

	if a.parentContext != nil {
		var dummy VALTYPE
		if _, ok := any(dummy).(ParentContextAwareDecoder); ok {
			return a.readFromWithParentContext(r, &val, false)
		}
		if _, ok := any(&dummy).(ParentContextAwareDecoder); ok {
			return a.readFromWithParentContext(r, &val, true)
		}
	}

	return a.Ary.ReadFrom(r)
}

func (a *Array[LENTYPE, VALTYPE]) readFromWithParentContext(r io.Reader, target *[]VALTYPE, usePtr bool) (int64, error) {
	var length LENTYPE
	var totalBytes int64

	type lengthDecoder interface {
		ReadFrom(io.Reader) (int64, error)
	}

	// Turn &length into something with a ReadFrom method
	dec, ok := any(&length).(lengthDecoder)
	if !ok {
		return 0, fmt.Errorf("length type %T does not implement ReadFrom", length)
	}

	n, err := dec.ReadFrom(r)
	if err != nil {
		return n, err
	}
	totalBytes += n

	lengthValue := reflect.ValueOf(length)
	var count int
	switch lengthValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		count = int(lengthValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		count = int(lengthValue.Uint())
	default:
		return totalBytes, nil
	}

	if count < 0 {
		return totalBytes, nil
	}

	elements := make([]VALTYPE, count)
	for i := 0; i < count; i++ {
		var bytesRead int64
		if usePtr {
			if decoder, ok := any(&elements[i]).(ParentContextAwareDecoder); ok {
				bytesRead, err = decoder.ReadFromWithParentContext(r, a.parentContext)
				totalBytes += bytesRead
				if err != nil {
					return totalBytes, err
				}
			}
		} else {
			if decoder, ok := any(elements[i]).(ParentContextAwareDecoder); ok {
				bytesRead, err = decoder.ReadFromWithParentContext(r, a.parentContext)
				totalBytes += bytesRead
				if err != nil {
					return totalBytes, err
				}
			}
		}
	}

	*target = elements
	return totalBytes, nil
}

func (a *Array[LENTYPE, VALTYPE]) SetParentContext(ctx ParentContext) {
	a.parentContext = ctx
}

func (a Array[LENTYPE, VALTYPE]) WriteTo(w io.Writer) (int64, error) {
	if a.parentContext != nil {
		var dummy VALTYPE
		if _, needsContext := any(dummy).(ParentContextAwareEncoder); needsContext {
			return a.writeToWithParentContext(w, false)
		}

		if _, needsContext := any(&dummy).(ParentContextAwareEncoder); needsContext {
			return a.writeToWithParentContext(w, true)
		}
	}

	return a.Ary.WriteTo(w)
}

func (a Array[LENTYPE, VALTYPE]) writeToWithParentContext(w io.Writer, usePtr bool) (totalBytes int64, err error) {
	if a.Ary.Ary == nil {
		return 0, nil
	}

	// 1. Recover the underlying []VALTYPE
	var values []VALTYPE

	switch v := a.Ary.Ary.(type) {
	case []VALTYPE:
		// encoding case: pk.Ary is usually constructed with a plain slice
		values = v
	case *[]VALTYPE:
		// decoding case: pk.Ary is usually constructed with &slice
		values = *v
	default:
		return 0, fmt.Errorf("Array.Ary has unexpected type %T; want []VALTYPE or *[]VALTYPE", a.Ary.Ary)
	}

	// 2. Encode the length using the LENTYPE's WriteTo
	length := LENTYPE(len(values))

	type lengthEncoder interface {
		WriteTo(io.Writer) (int64, error)
	}

	enc, ok := any(length).(lengthEncoder)
	if !ok {
		return 0, fmt.Errorf("LENTYPE %T does not implement WriteTo", length)
	}

	totalBytes, err = enc.WriteTo(w)
	if err != nil {
		return totalBytes, err
	}

	// 3. Write each element, using context-aware path when available
	for i := range values {
		if usePtr {
			if encoder, ok := any(&values[i]).(ParentContextAwareEncoder); ok {
				n, err := encoder.WriteToWithParentContext(w, a.parentContext)
				totalBytes += n
				if err != nil {
					return totalBytes, err
				}
			} else if fe, ok := any(&values[i]).(pk.FieldEncoder); ok {
				// Fallback: normal go-mc encoding if it’s a pk.FieldEncoder
				n, err := fe.WriteTo(w)
				totalBytes += n
				if err != nil {
					return totalBytes, err
				}
			} else {
				// You can decide what you want here (error vs. panic vs. ignore)
				return totalBytes, fmt.Errorf("element %T is neither ContextAwareEncoder nor pk.FieldEncoder", values[i])
			}
		} else {
			if encoder, ok := any(values[i]).(ParentContextAwareEncoder); ok {
				n, err := encoder.WriteToWithParentContext(w, a.parentContext)
				totalBytes += n
				if err != nil {
					return totalBytes, err
				}
			} else if fe, ok := any(values[i]).(pk.FieldEncoder); ok {
				// Fallback: normal go-mc encoding if it’s a pk.FieldEncoder
				n, err := fe.WriteTo(w)
				totalBytes += n
				if err != nil {
					return totalBytes, err
				}
			} else {
				// You can decide what you want here (error vs. panic vs. ignore)
				return totalBytes, fmt.Errorf("element %T is neither ContextAwareEncoder nor pk.FieldEncoder", values[i])
			}
		}
	}

	return totalBytes, nil
}

func (a Array[LENTYPE, VALTYPE]) Length() LENTYPE {
	return LENTYPE(len(a.Ary.Ary.([]VALTYPE)))
}

func (a Array[LENTYPE, VALTYPE]) Get() *[]VALTYPE {
	return a.Ary.Ary.(*[]VALTYPE)
}

func (a *Array[LENTYPE, VALTYPE]) Set(v *[]VALTYPE) {
	a.Ary.Ary = v
}
