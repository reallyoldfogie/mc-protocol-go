package models

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"

	go_version "github.com/aquasecurity/go-version/pkg/version"
)

// AnonymousNBT represents a polymorphic NBT value that can be any NBT type.
// Unlike NBTField which only accepts compound tags, AnonymousNBT can handle
// any NBT tag type (byte, short, int, long, float, double, byte array, string,
// list, compound, int array, long array).
//
// This is used for fields like "item_name" in Minecraft 1.21.5 where text
// components changed from JSON strings to direct NBT structures.
type AnonymousNBT struct {
	Version string
	// TagType is the type of the NBT tag
	TagType NBTType
	// Value is the actual NBT value
	Value NBTValue
}

// WriteTo writes the anonymous NBT to a writer.
// If Value is nil, it writes a TAG_End byte (0x00).
// Otherwise, it writes the tag type and value according to version rules.
func (anbt AnonymousNBT) WriteTo(writer io.Writer) (int64, error) {
	if anbt.Value == nil {
		// Write TAG_End for empty NBT
		nn, err := writer.Write([]byte{byte(TypeEnd)})
		return int64(nn), err
	}

	bytesWritten := int64(0)

	// Write tag type
	if err := binary.Write(writer, binary.BigEndian, anbt.TagType); err != nil {
		return bytesWritten, err
	}
	bytesWritten++

	// Name presence changed in 1.20.5: versions before 1.20.5 include name; 1.20.5+ omit it.
	effVersion := anbt.Version
	if effVersion == "" {
		effVersion = currentNBTVersion()
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

	// Write the value
	nn, err := anbt.Value.WriteTo(writer)
	if err != nil {
		return bytesWritten, err
	}
	bytesWritten += nn

	return bytesWritten, nil
}

// ReadFrom reads an anonymous NBT from a reader.
// It reads a tag type byte, optionally a name (depending on version), and then the value.
func (anbt *AnonymousNBT) ReadFrom(reader io.Reader) (int64, error) {
	bytesRead := int64(0)

	// Read tag type
	var tagType NBTType
	if err := binary.Read(reader, binary.BigEndian, &tagType); err != nil {
		return bytesRead, err
	}
	bytesRead++
	// log.Printf("[AnonymousNBT.ReadFrom] Read tag type: %d (bytesRead so far: %d)", tagType, bytesRead)

	// If TAG_End, the NBT is empty
	if tagType == TypeEnd {
		anbt.TagType = TypeEnd
		anbt.Value = nil
		// log.Printf("[AnonymousNBT.ReadFrom] TAG_End encountered, returning %d bytes", bytesRead)
		return bytesRead, nil
	}

	// In version 1.20.5 the name element was removed
	effVersion := anbt.Version
	if effVersion == "" {
		effVersion = currentNBTVersion()
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

	// Read the value based on tag type
	nbtReader := NewNBTReader(reader)
	value, n, err := nbtReader.ReadValue(tagType)
	if err != nil {
		return bytesRead, errors.Wrapf(err, "failed to read anonymous NBT value of type %d", tagType)
	}
	bytesRead += n
	// log.Printf("[AnonymousNBT.ReadFrom] Read value of type %d, value bytes: %d, total bytes: %d", tagType, n, bytesRead)

	anbt.TagType = tagType
	anbt.Value = value
	return bytesRead, nil
}

// NewAnonymousNBT creates a new anonymous NBT with an empty compound
func NewAnonymousNBT() *AnonymousNBT {
	return &AnonymousNBT{
		TagType: TypeCompound,
		Value:   &NBTCompound{Tags: []NBTTag{}},
	}
}

// NewAnonymousNBTWithValue creates a new anonymous NBT with the given value
func NewAnonymousNBTWithValue(value NBTValue) *AnonymousNBT {
	return &AnonymousNBT{
		TagType: value.Type(),
		Value:   value,
	}
}
