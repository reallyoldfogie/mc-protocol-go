package models

import (
	"fmt"
	"io"
	"reflect"

	pk "github.com/Tnze/go-mc/net/packet"
)

// ExplicitCountArray represents an array whose length is determined by a field
// in the parent context, rather than being read from the stream. This is used
// when the protocol specifies: ["array", {"count": "fieldName", "type": ...}]
type ExplicitCountArray[VALTYPE any] struct {
	Ary            Ary[pk.VarInt] // Uses VarInt as placeholder, actual count comes from parent context
	CountFieldName string         // Name of the field in parent context that contains the count
	parentContext  ParentContext
}

func (a *ExplicitCountArray[VALTYPE]) ReadFrom(r io.Reader) (int64, error) {
	if a.parentContext == nil {
		return 0, fmt.Errorf("ExplicitCountArray requires parent context but none was provided")
	}

	// Get the count from the parent context field
	countField := a.parentContext.GetField(a.CountFieldName)
	if countField == nil {
		return 0, fmt.Errorf("count field '%s' not found in parent context", a.CountFieldName)
	}

	// Extract the count value using reflection
	countValue := reflect.ValueOf(countField)
	var count int
	switch countValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		count = int(countValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		count = int(countValue.Uint())
	default:
		return 0, fmt.Errorf("count field '%s' has invalid type %T, expected integer", a.CountFieldName, countField)
	}

	if count < 0 {
		return 0, fmt.Errorf("count field '%s' has negative value %d", a.CountFieldName, count)
	}

	// Initialize array with the count
	val := make([]VALTYPE, count)
	a.Ary = Ary[pk.VarInt]{Ary: &val}

	var totalBytes int64

	// Check if element type needs parent context
	var dummy VALTYPE
	needsContext := false
	usePtr := false

	if _, ok := any(dummy).(ParentContextAwareDecoder); ok {
		needsContext = true
		usePtr = false
	} else if _, ok := any(&dummy).(ParentContextAwareDecoder); ok {
		needsContext = true
		usePtr = true
	}

	// Read array elements
	for i := 0; i < count; i++ {
		if needsContext {
			if usePtr {
				if decoder, ok := any(&val[i]).(ParentContextAwareDecoder); ok {
					bytesRead, err := decoder.ReadFromWithParentContext(r, a.parentContext)
					totalBytes += bytesRead
					if err != nil {
						return totalBytes, fmt.Errorf("failed to read element %d: %w", i, err)
					}
				}
			} else {
				if decoder, ok := any(val[i]).(ParentContextAwareDecoder); ok {
					bytesRead, err := decoder.ReadFromWithParentContext(r, a.parentContext)
					totalBytes += bytesRead
					if err != nil {
						return totalBytes, fmt.Errorf("failed to read element %d: %w", i, err)
					}
				}
			}
		} else {
			// Use standard FieldEncoder interface
			if decoder, ok := any(&val[i]).(pk.FieldDecoder); ok {
				bytesRead, err := decoder.ReadFrom(r)
				totalBytes += bytesRead
				if err != nil {
					return totalBytes, fmt.Errorf("failed to read element %d: %w", i, err)
				}
			} else {
				return totalBytes, fmt.Errorf("element type %T does not implement FieldDecoder", val[i])
			}
		}
	}

	return totalBytes, nil
}

func (a *ExplicitCountArray[VALTYPE]) SetParentContext(ctx ParentContext) {
	a.parentContext = ctx
}

func (a ExplicitCountArray[VALTYPE]) WriteTo(w io.Writer) (int64, error) {
	if a.Ary.Ary == nil {
		return 0, nil
	}

	// Don't write the count - it's written by the parent container field
	var totalBytes int64

	// Recover the underlying []VALTYPE
	var values []VALTYPE

	switch v := a.Ary.Ary.(type) {
	case []VALTYPE:
		values = v
	case *[]VALTYPE:
		values = *v
	default:
		return 0, fmt.Errorf("ExplicitCountArray.Ary has unexpected type %T; want []VALTYPE or *[]VALTYPE", a.Ary.Ary)
	}

	// Check if element type needs parent context
	var dummy VALTYPE
	needsContext := false
	usePtr := false

	if _, ok := any(dummy).(ParentContextAwareEncoder); ok {
		needsContext = true
		usePtr = false
	} else if _, ok := any(&dummy).(ParentContextAwareEncoder); ok {
		needsContext = true
		usePtr = true
	}

	// Write each element
	for i := range values {
		if needsContext {
			if usePtr {
				if encoder, ok := any(&values[i]).(ParentContextAwareEncoder); ok {
					n, err := encoder.WriteToWithParentContext(w, a.parentContext)
					totalBytes += n
					if err != nil {
						return totalBytes, fmt.Errorf("failed to write element %d: %w", i, err)
					}
				}
			} else {
				if encoder, ok := any(values[i]).(ParentContextAwareEncoder); ok {
					n, err := encoder.WriteToWithParentContext(w, a.parentContext)
					totalBytes += n
					if err != nil {
						return totalBytes, fmt.Errorf("failed to write element %d: %w", i, err)
					}
				}
			}
		} else {
			// Use standard FieldEncoder interface
			if encoder, ok := any(&values[i]).(pk.FieldEncoder); ok {
				n, err := encoder.WriteTo(w)
				totalBytes += n
				if err != nil {
					return totalBytes, fmt.Errorf("failed to write element %d: %w", i, err)
				}
			} else if encoder, ok := any(values[i]).(pk.FieldEncoder); ok {
				n, err := encoder.WriteTo(w)
				totalBytes += n
				if err != nil {
					return totalBytes, fmt.Errorf("failed to write element %d: %w", i, err)
				}
			} else {
				return totalBytes, fmt.Errorf("element type %T does not implement FieldEncoder", values[i])
			}
		}
	}

	return totalBytes, nil
}

func (a ExplicitCountArray[VALTYPE]) Length() int {
	if a.Ary.Ary == nil {
		return 0
	}
	switch v := a.Ary.Ary.(type) {
	case []VALTYPE:
		return len(v)
	case *[]VALTYPE:
		return len(*v)
	default:
		return 0
	}
}

func (a ExplicitCountArray[VALTYPE]) Get() *[]VALTYPE {
	if a.Ary.Ary == nil {
		return nil
	}
	return a.Ary.Ary.(*[]VALTYPE)
}

func (a *ExplicitCountArray[VALTYPE]) Set(v *[]VALTYPE) {
	a.Ary.Ary = v
}
