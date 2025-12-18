package models

import (
	"fmt"
	"reflect"

	"github.com/Tnze/go-mc/chat/sign"
)

// ConvertPreviousMessagesToPackedSignatures converts a versioned basetypes.PreviousMessages
// value into a slice of sign.PackedSignature.
// Mapping based on generated basetypes:
// - Element.Id == 0  => inline signature present; output PackedSignature{ID: -1, Signature: *[256]byte}
// - Element.Id > 0   => reference; output PackedSignature{ID: Id-1, Signature: nil}
func ConvertPreviousMessagesToPackedSignatures(prev any) ([]sign.PackedSignature, error) {
	rv := reflect.ValueOf(prev)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	// Expect struct with field Ary (models.Array) which has field Ary (pk.Ary)
	aryField := rv.FieldByName("Ary")
	if !aryField.IsValid() {
		return nil, fmt.Errorf("convertPrevMsgsGeneric: missing field Ary on %T", prev)
	}
	innerAry := aryField.FieldByName("Ary")
	if !innerAry.IsValid() {
		return nil, fmt.Errorf("convertPrevMsgsGeneric: missing field Ary.Ary on %T", prev)
	}

	raw := innerAry.Interface()
	if raw == nil {
		return nil, nil
	}

	sv := reflect.ValueOf(raw)
	if sv.Kind() == reflect.Ptr {
		if sv.IsNil() {
			return nil, nil
		}
		sv = sv.Elem()
	}
	if sv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("convertPrevMsgsGeneric: Ary.Ary has kind %s, want slice", sv.Kind())
	}

	out := make([]sign.PackedSignature, sv.Len())
	for i := 0; i < sv.Len(); i++ {
		el := sv.Index(i)
		idField := el.FieldByName("Id")
		sigField := el.FieldByName("Signature")
		if !idField.IsValid() || !sigField.IsValid() {
			return nil, fmt.Errorf("convertPrevMsgsGeneric: element missing Id/Signature fields")
		}

		// Read Id numeric value (pk.VarInt underlying is int32)
		var idVal int64
		switch idField.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			idVal = idField.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			idVal = int64(idField.Uint())
		default:
			return nil, fmt.Errorf("convertPrevMsgsGeneric: unexpected Id kind %s", idField.Kind())
		}

		if idVal == 0 {
			// Inline signature present; Signature should be *models.FixedBuffer256
			ps := sign.PackedSignature{ID: -1}
			if sigField.IsZero() {
				out[i] = ps
				continue
			}
			if fb, ok := sigField.Interface().(*FixedBuffer256); ok && fb != nil {
				sig := sign.Signature(*fb)
				ps.Signature = &sig
				out[i] = ps
				continue
			}
			return nil, fmt.Errorf("convertPrevMsgsGeneric: unexpected signature type %T for inline case", sigField.Interface())
		}

		// Reference case: store index-1 and nil signature
		out[i] = sign.PackedSignature{ID: int32(idVal) - 1, Signature: nil}
	}
	return out, nil
}
