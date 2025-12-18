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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Used by all converters; change with UseJavaEncoding() or UseBedrockEncoding()
var byteOrder = binary.ByteOrder(binary.BigEndian) // NOTE: CHANGED FROM DEFAULT IN nbt2json

// If longAsString is true, nbt long (int64) will be a string of the number instead of a valueLeast/valueMost uint32 pair
var longAsString = true // NOTE: CHANGED FROM DEFAULT IN nbt2json

// NbtParseError is when the nbt data does not match an expected pattern. Pass it message string and downstream error
type NbtParseError struct {
	str     string
	err     error
	tagType byte   // Optional: actual tag type received (for debugging)
	version string // Optional: protocol version (for debugging)
}

func (e NbtParseError) Error() string {
	var str string
	if e.err != nil {
		str = fmt.Sprintf(": %+v", e.err)
	}
	extra := ""
	if e.tagType != 0 {
		extra = fmt.Sprintf(" (received tag type: %d, version: %s)", e.tagType, e.version)
	}
	return fmt.Sprintf("Error parsing NBT: %s%s%s", e.str, str, extra)
}

// NbtJson is the top-level JSON document; it is exported for reflect, and client code shouldn't use it
type NbtJson struct {
	Name           string             `json:"name"`
	Version        string             `json:"version"`
	Nbt2JsonUrl    string             `json:"nbt2JsonUrl"`
	ConversionTime string             `json:"conversionTime,omitempty"`
	Comment        string             `json:"comment,omitempty"`
	Nbt            []*json.RawMessage `json:"nbt"`
}

// NbtTag represents one NBT tag for each struct; it is exported for reflect, and client code shouldn't use it
type NbtTag struct {
	TagType byte        `json:"tagType"`
	Name    string      `json:"name"`
	Value   interface{} `json:"value,omitempty"`
}

// NbtTagList represents an NBT tag list; it is exported for reflect, and client code shouldn't use it
type NbtTagList struct {
	TagListType byte          `json:"tagListType"`
	List        []interface{} `json:"list"`
}

// NbtLong stores a 64-bit int into two 32-bit values for json portability. ValueMost are the high 32 bits and ValueLeast are the low 32 bits.
//
//	using uint32s to avoid Go trying to outsmart us on "negative" int32s
type NbtLong struct {
	ValueLeast uint32 `json:"valueLeast"`
	ValueMost  uint32 `json:"valueMost"`
}

// Turns an int64 (nbt long) into a valueLeast/valueMost json pair
func longToIntPair(i int64) NbtLong {
	var nbtLong NbtLong
	nbtLong.ValueLeast = uint32(i & 0xffffffff)
	nbtLong.ValueMost = uint32(i >> 32)
	return nbtLong
}

// Nbt2Json converts uncompressed NBT byte array to JSON byte array
func Nbt2Json(b []byte, comment string) ([]byte, error) {
	var nbtJson NbtJson

	nbtJson.ConversionTime = time.Now().Format(time.RFC3339)
	nbtJson.Comment = comment
	buf := bytes.NewReader(b)
	// var nbtJson.nbt []*json.RawMessage
	for buf.Len() > 0 {
		element, err := getTag(buf)
		if err != nil {
			return nil, err
		}
		myTemp := json.RawMessage(element)
		nbtJson.Nbt = append(nbtJson.Nbt, &myTemp)
	}
	jsonOut, err := json.MarshalIndent(nbtJson, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonOut, nil
}

// getTag broken out form Nbt2Json to allow recursion with reader but public input is []byte
func getTag(r *bytes.Reader) ([]byte, error) {
	var data NbtTag
	err := binary.Read(r, byteOrder, &data.TagType)
	if err != nil {
		return nil, NbtParseError{str: "Reading TagType", err: err}
	}
	// do not try to fetch name for TagType 0 which is compound end tag
	if data.TagType != 0 {
		var err error
		var nameLen int16
		err = binary.Read(r, byteOrder, &nameLen)
		if err != nil {
			return nil, NbtParseError{str: "Reading Name length", err: err}
		}
		name := make([]byte, nameLen)
		err = binary.Read(r, byteOrder, &name)
		if err != nil {
			return nil, NbtParseError{str: fmt.Sprintf("Reading Name - is UseJavaEncoding or UseBedrockEncoding set correctly? Name length decoded is %d", nameLen), err: err}
		}
		data.Name = string(name[:])
	}
	data.Value, err = getPayload(r, data.TagType)
	if err != nil {
		return nil, err
	}
	outJson, err := json.MarshalIndent(data, "", "  ")
	return outJson, err
}

// Gets the tag payload. Had to break this out from the main function to allow tag list recursion
func getPayload(r *bytes.Reader, tagType byte) (interface{}, error) {
	var output interface{}
	var err error
	switch tagType {
	case 0:
		// end tag for compound; do nothing further
	case 1:
		var i int8
		err = binary.Read(r, byteOrder, &i)
		if err != nil {
			return nil, NbtParseError{str: "Reading int8", err: err}
		}
		output = i
	case 2:
		var i int16
		err = binary.Read(r, byteOrder, &i)
		if err != nil {
			return nil, NbtParseError{str: "Reading int16", err: err}
		}
		output = i
	case 3:
		var i int32
		err = binary.Read(r, byteOrder, &i)
		if err != nil {
			return nil, NbtParseError{str: "Reading int32", err: err}
		}
		output = i
	case 4:
		var i int64
		err = binary.Read(r, byteOrder, &i)
		if err != nil {
			return nil, NbtParseError{str: "Reading int64", err: err}
		}
		if longAsString {
			output = fmt.Sprintf("%d", i)
		} else {
			output = longToIntPair(i)
		}
	case 5:
		var f float32
		err = binary.Read(r, byteOrder, &f)
		if err != nil {
			return nil, NbtParseError{str: "Reading float32", err: err}
		}
		output = f
	case 6:
		var f float64
		err = binary.Read(r, byteOrder, &f)
		if err != nil {
			return nil, NbtParseError{str: "Reading float64", err: err}
		}
		if math.IsNaN(f) {
			output = "NaN"
		} else {
			output = f
		}
	case 7:
		var byteArray []int8
		var oneByte int8
		var numRecords int32
		err := binary.Read(r, byteOrder, &numRecords)
		if err != nil {
			return nil, NbtParseError{str: "Reading byte array tag length", err: err}
		}
		for i := int32(1); i <= numRecords; i++ {
			err = binary.Read(r, byteOrder, &oneByte)
			if err != nil {
				return nil, NbtParseError{str: "Reading byte in byte array tag", err: err}
			}
			byteArray = append(byteArray, oneByte)
		}
		output = byteArray
	case 8:
		var strLen int16
		err := binary.Read(r, byteOrder, &strLen)
		if err != nil {
			return nil, NbtParseError{str: "Reading string tag length", err: err}
		}
		utf8String := make([]byte, strLen)
		err = binary.Read(r, byteOrder, &utf8String)
		if err != nil {
			return nil, NbtParseError{str: "Reading string tag data", err: err}
		}
		output = string(utf8String[:])
	case 9:
		var tagList NbtTagList
		err = binary.Read(r, byteOrder, &tagList.TagListType)
		if err != nil {
			return nil, NbtParseError{str: "Reading TagType", err: err}
		}
		var numRecords int32
		err := binary.Read(r, byteOrder, &numRecords)
		if err != nil {
			return nil, NbtParseError{str: "Reading list tag length", err: err}
		}
		for i := int32(1); i <= numRecords; i++ {
			payload, err := getPayload(r, tagList.TagListType)
			if err != nil {
				return nil, NbtParseError{str: "Reading list tag item", err: err}
			}
			tagList.List = append(tagList.List, payload)
		}
		output = tagList
	case 10:
		var compound []json.RawMessage
		var tagType byte
		for err = binary.Read(r, byteOrder, &tagType); tagType != 0; err = binary.Read(r, byteOrder, &tagType) {
			if err != nil {
				return nil, NbtParseError{str: "compound: reading next tag type", err: err}
			}
			_, err = r.Seek(-1, 1)
			if err != nil {
				return nil, NbtParseError{str: "seeking back one", err: err}
			}
			tag, err := getTag(r)
			if err != nil {
				return nil, NbtParseError{str: "compound: reading a child tag", err: err}
			}
			compound = append(compound, json.RawMessage(string(tag)))
		}
		if compound == nil {
			// Explicitly give empty array else value will be null instead of []
			output = []int{}
		} else {
			output = compound
		}
	case 11:
		var intArray []int32
		var numRecords, oneInt int32
		err := binary.Read(r, byteOrder, &numRecords)
		if err != nil {
			return nil, NbtParseError{str: "Reading int array tag length", err: err}
		}
		for i := int32(1); i <= numRecords; i++ {
			err := binary.Read(r, byteOrder, &oneInt)
			if err != nil {
				return nil, NbtParseError{str: "Reading int in int array tag", err: err}
			}
			intArray = append(intArray, oneInt)
		}
		output = intArray
	case 12:
		var longArray []NbtLong
		var longStringArray []string
		var numRecords, oneInt int64
		err := binary.Read(r, byteOrder, &numRecords)
		if err != nil {
			return nil, NbtParseError{str: "Reading long array tag length", err: err}
		}
		for i := int64(1); i <= numRecords; i++ {
			err := binary.Read(r, byteOrder, &oneInt)
			if err != nil {
				return nil, NbtParseError{str: "Reading long in long array tag", err: err}
			}
			longArray = append(longArray, longToIntPair(oneInt))
			longStringArray = append(longStringArray, fmt.Sprintf("%d", oneInt))
		}
		if longAsString {
			output = longStringArray
		} else {
			output = longArray
		}

	default:
		return nil, NbtParseError{str: fmt.Sprintf("TagType %d not recognized", tagType), err: nil}
	}
	return output, nil
}
