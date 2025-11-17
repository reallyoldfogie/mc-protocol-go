# NBT Parser Usage Guide

## Overview

The custom NBT parser provides a type-safe way to read and write Minecraft's NBT (Named Binary Tag) format without external dependencies. It's fully integrated with the packet system through the `models.NBTField` type.

## Basic Types

### Primitive Types

```go
// Create NBT primitives
byteVal := models.NBTByte{Value: 42}
shortVal := models.NBTShort{Value: 1000}
intVal := models.NBTInt{Value: 123456}
longVal := models.NBTLong{Value: 9876543210}
floatVal := models.NBTFloat{Value: 3.14}
doubleVal := models.NBTDouble{Value: 2.718281828}
stringVal := models.NBTString{Value: "Hello World"}
```

### Array Types

```go
// Create NBT arrays
byteArray := models.NBTByteArray{Value: []int8{1, 2, 3, 4, 5}}
intArray := models.NBTIntArray{Value: []int32{100, 200, 300}}
longArray := models.NBTLongArray{Value: []int64{1000000, 2000000}}
```

### List Types

```go
// Create homogeneous list of integers
intList := models.NBTList{
    ListType: models.TypeInt,
    Values: []models.NBTValue{
        &models.NBTInt{Value: 10},
        &models.NBTInt{Value: 20},
        &models.NBTInt{Value: 30},
    },
}

// Create list of strings
stringList := models.NBTList{
    ListType: models.TypeString,
    Values: []models.NBTValue{
        &models.NBTString{Value: "first"},
        &models.NBTString{Value: "second"},
    },
}
```

### Compound Types

```go
// Create a compound tag (like a map/struct)
compound := models.NBTCompound{
    Tags: []models.NBTTag{
        {Name: "playerName", Value: &models.NBTString{Value: "Steve"}},
        {Name: "health", Value: &models.NBTFloat{Value: 20.0}},
        {Name: "level", Value: &models.NBTInt{Value: 42}},
    },
}

// Access values from compound
if nameVal, ok := compound.Get("playerName"); ok {
    if nameStr, ok := nameVal.(*models.NBTString); ok {
        fmt.Println("Player name:", nameStr.Value)
    }
}

// Set or update values
compound.Set("experience", &models.NBTInt{Value: 1500})
```

## Reading and Writing NBT

### Using io.Reader and io.Writer

```go
import (
    "bytes"
    "github.com/reallyoldfogie/mc-protocol-go/models"
)

// Writing NBT to bytes
buf := &bytes.Buffer{}
compound := models.NBTCompound{
    Tags: []models.NBTTag{
        {Name: "name", Value: &models.NBTString{Value: "TestPlayer"}},
        {Name: "score", Value: &models.NBTInt{Value: 100}},
    },
}

bytesWritten, err := compound.WriteTo(buf)
if err != nil {
    log.Fatal(err)
}

// Reading NBT from bytes
var readCompound models.NBTCompound
bytesRead, err := readCompound.ReadFrom(buf)
if err != nil {
    log.Fatal(err)
}
```

### Using NBTReader

```go
// Create a reader for more complex operations
reader := models.NewNBTReader(bytes.NewReader(nbtData))

// Read a complete named tag
tag, err := reader.ReadTag()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Tag name: %s, Type: %d\n", tag.Name, tag.Value.Type())
```

## Using in Packets

The `models.NBTField` type is designed to work with the packet system:

```go
// In packet definitions (generated code will use this)
type MyPacket struct {
    packetID int32
    Data     models.NBTField
}

// Reading a packet with NBT
var packet MyPacket
_, err := packet.Data.ReadFrom(reader)

// Accessing the NBT data
if packet.Data.Value != nil {
    if nameVal, ok := packet.Data.Value.Get("name"); ok {
        // Process the name value
    }
}

// Creating NBT data for a packet
packet.Data = models.NBTField{
    Value: &models.NBTCompound{
        Tags: []models.NBTTag{
            {Name: "id", Value: &models.NBTString{Value: "minecraft:diamond"}},
            {Name: "count", Value: &models.NBTByte{Value: 64}},
        },
    },
}

_, err = packet.Data.WriteTo(writer)
```

## Working with Optional NBT

For optional NBT fields (using `models.Option[models.NBTField]`):

```go
// Optional NBT field
type PacketWithOptionalNBT struct {
    Description models.Option[models.NBTField]
}

// Setting an optional NBT value
nbtField := models.NBTField{
    Value: &models.NBTCompound{
        Tags: []models.NBTTag{
            {Name: "text", Value: &models.NBTString{Value: "Hello"}},
        },
    },
}

packet := PacketWithOptionalNBT{
    Description: models.Option[models.NBTField]{
        Has: true,
        Val: &nbtField,
    },
}

// Reading optional NBT
if packet.Description.Has && packet.Description.Val != nil {
    // Access the NBT data
    if textVal, ok := packet.Description.Val.Value.Get("text"); ok {
        fmt.Println("Description:", textVal.(*models.NBTString).Value)
    }
}
```

## Complex Example: Player Inventory

```go
// Create a player inventory structure
playerData := models.NBTCompound{
    Tags: []models.NBTTag{
        {Name: "playerName", Value: &models.NBTString{Value: "Steve"}},
        {Name: "health", Value: &models.NBTFloat{Value: 20.0}},
        {Name: "foodLevel", Value: &models.NBTInt{Value: 20}},
        {
            Name: "inventory",
            Value: &models.NBTList{
                ListType: models.TypeCompound,
                Values: []models.NBTValue{
                    &models.NBTCompound{
                        Tags: []models.NBTTag{
                            {Name: "id", Value: &models.NBTString{Value: "minecraft:diamond_sword"}},
                            {Name: "Count", Value: &models.NBTByte{Value: 1}},
                            {Name: "Damage", Value: &models.NBTShort{Value: 0}},
                        },
                    },
                    &models.NBTCompound{
                        Tags: []models.NBTTag{
                            {Name: "id", Value: &models.NBTString{Value: "minecraft:bread"}},
                            {Name: "Count", Value: &models.NBTByte{Value: 64}},
                        },
                    },
                },
            },
        },
    },
}

// Serialize and deserialize
buf := &bytes.Buffer{}
_, err := playerData.WriteTo(buf)
if err != nil {
    log.Fatal(err)
}

var loadedData models.NBTCompound
_, err = loadedData.ReadFrom(buf)
if err != nil {
    log.Fatal(err)
}

// Access nested data
if invVal, ok := loadedData.Get("inventory"); ok {
    invList := invVal.(*models.NBTList)
    fmt.Printf("Player has %d items\n", len(invList.Values))
    
    for i, itemVal := range invList.Values {
        item := itemVal.(*models.NBTCompound)
        if idVal, ok := item.Get("id"); ok {
            fmt.Printf("Item %d: %s\n", i, idVal.(*models.NBTString).Value)
        }
    }
}
```

## Type Reference

### NBT Type Constants

```go
const (
    TypeEnd       NBTType = 0  // End marker for compounds
    TypeByte      NBTType = 1  // int8
    TypeShort     NBTType = 2  // int16
    TypeInt       NBTType = 3  // int32
    TypeLong      NBTType = 4  // int64
    TypeFloat     NBTType = 5  // float32
    TypeDouble    NBTType = 6  // float64
    TypeByteArray NBTType = 7  // []int8
    TypeString    NBTType = 8  // string
    TypeList      NBTType = 9  // homogeneous list
    TypeCompound  NBTType = 10 // named tags
    TypeIntArray  NBTType = 11 // []int32
    TypeLongArray NBTType = 12 // []int64
)
```

### Interface

All NBT values implement the `NBTValue` interface:

```go
type NBTValue interface {
    Type() NBTType
    io.WriterTo
    io.ReaderFrom
}
```

## Error Handling

The NBT parser uses the `NbtParseError` type for errors:

```go
_, err := compound.ReadFrom(reader)
if err != nil {
    if nbtErr, ok := err.(models.NbtParseError); ok {
        fmt.Printf("NBT parse error: %s\n", nbtErr.Error())
    }
}
```

## Migration from pk.NBTField

If you have existing code using `pk.NBTField`:

1. **Replace the type**:
   ```go
   // Old
   field pk.NBTField
   
   // New
   field models.NBTField
   ```

2. **Access the value**:
   ```go
   // Old (used V any)
   if field.V != nil {
       // type assert and use
   }
   
   // New (uses Value *NBTCompound)
   if field.Value != nil {
       if val, ok := field.Value.Get("key"); ok {
           // use val
       }
   }
   ```

3. **Regenerate protocol files** with the updated generator to get `models.NBTField` types.

## Performance Notes

- The parser uses `binary.BigEndian` (Java Edition byte order)
- All operations return bytes read/written for precise tracking
- Memory allocation is optimized for typical use cases
- Round-trip encoding/decoding is fully tested

## License

This implementation is based on [nbt2json](https://github.com/midnightfreddie/nbt2json) by Jim Nelson and is licensed under the MIT License. See file headers for full copyright notice.
