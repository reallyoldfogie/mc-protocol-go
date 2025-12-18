package models

import (
	"reflect"

	pk "github.com/Tnze/go-mc/net/packet"
)

// PacketMarshaller is the common interface implemented by all generated packet structs.
// It provides methods to convert between Go structs and wire-format packets.
type PacketMarshaller interface {
	// Marshal converts the packet struct into a pk.Packet ready for transmission.
	Marshal() pk.Packet

	// Scan populates the packet struct from a received pk.Packet.
	Scan(packet pk.Packet) error

	// PacketID returns the packet's protocol ID.
	PacketID() int32

	GetFields() map[string]pk.FieldEncoder
	SetFields(fields map[string]pk.FieldEncoder)
}

// GetPacketFieldValue returns a Go-native value for the named field on the
// provided PacketMarshaller. For named basic pk types (e.g., pk.String,
// pk.Boolean, pk.VarInt) it converts them to their underlying built-in types
// (string, bool, int32, etc). For non-basic or complex types, the original
// value is returned as-is.
func GetPacketFieldValue(p PacketMarshaller, name string) (any, bool) {
	if p == nil {
		return nil, false
	}
	fields := p.GetFields()
	fe, ok := fields[name]
	if !ok {
		return nil, false
	}
	return toNativeValue(fe), true
}

// GetPacketFieldEncoder returns the raw pk.FieldEncoder for the named field, if present.
func GetPacketFieldEncoder(p PacketMarshaller, name string) (pk.FieldEncoder, bool) {
	if p == nil {
		return nil, false
	}
	fe, ok := p.GetFields()[name]
	return fe, ok
}

// PacketFieldExists reports whether a field with the given name exists on the packet.
func PacketFieldExists(p PacketMarshaller, name string) bool {
	if p == nil {
		return false
	}
	_, ok := p.GetFields()[name]
	return ok
}

// GetPacketFieldAs retrieves a field by name and attempts to return it as type T.
// It first converts pk basic named types to their underlying built-ins, and then
// attempts a direct assertion or reflect-based conversion to T.
func GetPacketFieldAs[T any](p PacketMarshaller, name string) (T, bool) {
	var zero T
	v, ok := GetPacketFieldValue(p, name)
	if !ok {
		return zero, false
	}
	// Direct type assertion
	if tv, ok := v.(T); ok {
		return tv, true
	}
	// Limited reflective conversion: allow conversion from underlying built-in
	// type to pk named types in the go-mc packet package. This supports cases
	// like string -> pk.String or int32 -> pk.VarInt without permitting
	// arbitrary numeric conversions (e.g., int32 -> float64).
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return zero, false
	}
	rt := reflect.TypeOf((*T)(nil)).Elem()
	if rt.PkgPath() == "github.com/Tnze/go-mc/net/packet" && rv.Type().ConvertibleTo(rt) {
		cv := rv.Convert(rt)
		return cv.Interface().(T), true
	}
	return zero, false
}

// toNativeValue converts common go-mc pk named basic types into their underlying
// Go built-in types. Non-basic or unsupported kinds are returned unchanged.
func toNativeValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	// Unwrap pointer to value if present
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	// Convert named basic types to their underlying built-ins
	switch rv.Kind() {
	case reflect.String:
		return rv.Convert(reflect.TypeOf("")).Interface()
	case reflect.Bool:
		return rv.Convert(reflect.TypeOf(true)).Interface()
	case reflect.Int:
		return rv.Convert(reflect.TypeOf(int(0))).Interface()
	case reflect.Int8:
		return rv.Convert(reflect.TypeOf(int8(0))).Interface()
	case reflect.Int16:
		return rv.Convert(reflect.TypeOf(int16(0))).Interface()
	case reflect.Int32:
		return rv.Convert(reflect.TypeOf(int32(0))).Interface()
	case reflect.Int64:
		return rv.Convert(reflect.TypeOf(int64(0))).Interface()
	case reflect.Uint:
		return rv.Convert(reflect.TypeOf(uint(0))).Interface()
	case reflect.Uint8:
		return rv.Convert(reflect.TypeOf(uint8(0))).Interface()
	case reflect.Uint16:
		return rv.Convert(reflect.TypeOf(uint16(0))).Interface()
	case reflect.Uint32:
		return rv.Convert(reflect.TypeOf(uint32(0))).Interface()
	case reflect.Uint64:
		return rv.Convert(reflect.TypeOf(uint64(0))).Interface()
	case reflect.Float32:
		return rv.Convert(reflect.TypeOf(float32(0))).Interface()
	case reflect.Float64:
		return rv.Convert(reflect.TypeOf(float64(0))).Interface()
	default:
		// Non-basic or composite types are returned as-is
		return v
	}
}
