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
	"io"

	"github.com/pkg/errors"

	go_version "github.com/aquasecurity/go-version/pkg/version"
)

// NBTField represents an NBT field in a packet, similar to pk.NBTField.
// It wraps an NBTCompound for compatibility with the rest of the packet system.
type NBTField struct {
	Version string
	// Value is the NBT compound value. Can be nil to represent an empty NBT.
	Value *NBTCompound
}

// currentNBTVersion holds the process-wide default version used for NBT name handling when
// an NBTField instance does not have Version explicitly set. This lets callers (or generated
// code) set the active protocol version just before (de)serialization.
//
// WARNING: This is a global, process-wide setting and is NOT thread-safe across versions.
// If multiple protocol versions are being parsed/serialized concurrently, callers must
// avoid interleaving different version contexts or must set NBTField.Version explicitly
// on each instance to override this global.
var currentNBTVersion string

// SetCurrentNBTVersion sets the default protocol version used by NBTField ReadFrom/WriteTo
// when the instance's Version field is empty.
func SetCurrentNBTVersion(v string) { currentNBTVersion = v }

// WriteTo writes the NBT field to a writer.
// If Value is nil, it writes a TAG_End byte (0x00).
// Otherwise, it writes the full NBT tag with name and compound value.
func (nf NBTField) WriteTo(writer io.Writer) (int64, error) {
	if nf.Value == nil {
		// Write TAG_End for empty NBT
		nn, err := writer.Write([]byte{byte(TypeEnd)})
		return int64(nn), err
	}

	bytesWritten := int64(0)

	// Write tag type (Compound = 10)
	if err := binary.Write(writer, binary.BigEndian, TypeCompound); err != nil {
		return bytesWritten, err
	}
	bytesWritten++
	// Name presence changed in 1.20.5: versions before 1.20.5 include name; 1.20.5+ omit it.
	effVersion := nf.Version
	if effVersion == "" {
		effVersion = currentNBTVersion
	}
	if effVersion != "" {
		var version, _ = go_version.Parse("1.20.5")
		var nfVersion, _ = go_version.Parse(effVersion)
		if nfVersion.Compare(version) < 0 {

			// Write empty name (NBT fields in packets typically have empty names)
			if err := binary.Write(writer, binary.BigEndian, int16(0)); err != nil {
				return bytesWritten, err
			}
			bytesWritten += 2
		}
	}
	// Write the compound value
	nn, err := nf.Value.WriteTo(writer)
	if err != nil {
		return bytesWritten, err
	}
	bytesWritten += nn

	return bytesWritten, nil
}

// ReadFrom reads the NBT field from a reader.
// It reads a complete NBT tag including type and name.
func (nf *NBTField) ReadFrom(reader io.Reader) (int64, error) {
	bytesRead := int64(0)

	// Read tag type
	var tagType NBTType
	if err := binary.Read(reader, binary.BigEndian, &tagType); err != nil {
		return bytesRead, err
	}
	bytesRead++

	// If TAG_End, the NBT is empty
	if tagType == TypeEnd {
		nf.Value = nil
		return bytesRead, nil
	}

	// In version 1.20.5 the name element was removed
	effVersion := nf.Version
	if effVersion == "" {
		effVersion = currentNBTVersion
	}
	if effVersion != "" {
		var version, _ = go_version.Parse("1.20.5")
		var nfVersion, _ = go_version.Parse(effVersion)
		if nfVersion.Compare(version) < 0 {
			// Read tag name length
			var nameLen int16
			if err := binary.Read(reader, binary.BigEndian, &nameLen); err != nil {
				return bytesRead, err
			}
			bytesRead += 2

			// Read and discard name (we don't use it)
			if nameLen > 0 {
				nameBytes := make([]byte, nameLen)
				if err := binary.Read(reader, binary.BigEndian, &nameBytes); err != nil {
					return bytesRead, err
				}
				bytesRead += int64(nameLen)
			}
		}
	}

	// Only support compound tags for NBTField
	if tagType != TypeCompound {
		return bytesRead, NbtParseError{str: "NBTField expected compound tag", err: errors.New("invalid tag type"), tagType: byte(tagType), version: effVersion}
	}

	// Read the compound value (with byte count)
	nbtReader := NewNBTReader(reader)
	compound, n, err := nbtReader.readCompound()
	if err != nil {
		return bytesRead, err
	}
	bytesRead += n

	nf.Value = compound
	return bytesRead, nil
}

// NewNBTField creates a new NBT field with an empty compound
func NewNBTField() *NBTField {
	return &NBTField{
		Value: &NBTCompound{Tags: []NBTTag{}},
	}
}

// NewNBTFieldWithCompound creates a new NBT field with the given compound
func NewNBTFieldWithCompound(compound *NBTCompound) *NBTField {
	return &NBTField{
		Value: compound,
	}
}
