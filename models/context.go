package models

import (
	"io"

	pk "github.com/Tnze/go-mc/net/packet"
)

// ParentContext provides access to parent field values during deserialization
type ParentContext interface {
	// GetField retrieves a parent field value by name
	GetField(fieldName string) any
}

// ParentContextAwareDecoder is an element type that requires parent context
type ParentContextAwareDecoder interface {
	pk.FieldDecoder
	ReadFromWithParentContext(r io.Reader, ctx ParentContext) (int64, error)
}

// ParentContextAwareEncoder is an element type that requires parent context
type ParentContextAwareEncoder interface {
	pk.FieldEncoder
	WriteToWithParentContext(w io.Writer, ctx ParentContext) (int64, error)
}

// SimpleParentContext provides a basic field access implementation
//
// Prefer element-specific ParentContext implementations that preserve type
// information over using this generic helper where possible.
type SimpleParentContext struct {
	fields map[string]any
}

func NewParentContext() *SimpleParentContext {
	return &SimpleParentContext{fields: make(map[string]any)}
}

func (c *SimpleParentContext) SetField(name string, value any) {
	c.fields[name] = value
}

func (c *SimpleParentContext) GetField(name string) any {
	return c.fields[name]
}
