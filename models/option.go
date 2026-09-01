package models

import (
	"fmt"
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

// Option is a generic type that stores a pointer to the element type T
// T is the element type; Option stores *T so we can allocate it on ReadFrom.
type Option[T any] struct {
	Has pk.Boolean
	Val *T // pointer to T (so Option[ItemEffectDetail] -> Val is *ItemEffectDetail)
}

func (o Option[T]) WriteTo(w io.Writer) (n int64, err error) {
	n1, err := o.Has.WriteTo(w)
	if err != nil || !o.Has {
		return n1, err
	}
	// Ensure that Val implements io.WriterTo at runtime:
	if o.Val == nil {
		return n1, fmt.Errorf("Option has true Has but Val is nil")
	}
	if wt, ok := any(o.Val).(io.WriterTo); ok {
		n2, err := wt.WriteTo(w)
		return n1 + n2, err
	}
	return n1, fmt.Errorf("value does not implement io.WriterTo")
}

func (o *Option[T]) ReadFrom(r io.Reader) (n int64, err error) {
	n1, err := o.Has.ReadFrom(r)
	if err != nil || !o.Has {
		return n1, err
	}
	// allocate a new T if needed
	if o.Val == nil {
		var tmp T
		o.Val = &tmp
		// Special handling for pk.NBTField: initialize V field
		if nbtField, ok := any(o.Val).(*pk.NBTField); ok {
			var tmp pk.String
			// nbtField.V = &map[string]any{}
			nbtField.V = &tmp
		}
		// Special handling for models.NBTField: initialize Value field
		if nbtField, ok := any(o.Val).(*NBTField); ok {
			// Initialize empty compound and inherit current default NBT version
			nbtField.Value = &NBTCompound{Tags: []NBTTag{}}
			if nbtField.Version == "" {
				nbtField.Version = currentNBTVersion()
			}
		}
	}
	if rf, ok := any(o.Val).(io.ReaderFrom); ok {
		n2, err := rf.ReadFrom(r)
		return n1 + n2, err
	}
	return n1, fmt.Errorf("value does not implement io.ReaderFrom")
}

// Pointer returns the pointer to the element or nil.
func (o *Option[T]) Pointer() *T {
	if o.Has {
		return o.Val
	}
	return nil
}
