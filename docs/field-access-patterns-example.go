package docs

import (
	"fmt"

	pk "github.com/Tnze/go-mc/net/packet"
	v1211 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.1/play/clientbound"
	v1216 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/clientbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// This file demonstrates the three patterns for accessing packet fields.

// Pattern 1: VERSION-AGNOSTIC ACCESS (Dynamic/Runtime types)
// Use GetFields()/SetFields() for fields with version-specific types.
// This pattern works with any packet from any version.
func ExampleVersionAgnosticAccess(pkt models.PacketMarshaller) {
	fields := pkt.GetFields()

	// Access Action field (which may be a version-specific bitflags type)
	if action, ok := fields["Action"]; ok {
		fmt.Printf("Action field: %v (type: %T)\n", action, action)

		// You can type-assert to what you expect, but you lose type safety
		// and need to handle different types for different versions
		switch v := action.(type) {
		case v1211.PlayerInfoActionBitflags:
			fmt.Printf("v1.21.1 PlayerInfo: AddPlayer=%v\n", v.AddPlayer())
		case v1216.PlayerInfoActionBitflags:
			fmt.Printf("v1.21.6 PlayerInfo: AddPlayer=%v\n", v.AddPlayer())
		case pk.VarInt:
			fmt.Printf("VarInt action: %d\n", v)
		default:
			fmt.Printf("Unknown action type: %T\n", v)
		}
	}

	// You can also set fields this way
	newFields := map[string]pk.FieldEncoder{
		"Count": pk.VarInt(42),
	}
	pkt.SetFields(newFields)
}

// Pattern 2: SEMI-AGNOSTIC ACCESS (Stable primitive types)
// Use typed interfaces for fields that have stable types across versions.
// This gives you some type safety while remaining version-agnostic.
func ExampleSemiAgnosticAccess(pkt models.PacketMarshaller) {
	// Check if packet has Count field (multiple types across versions)
	// Try as pk.VarInt first (most common)
	if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
		count := getter.GetCount() // Returns pk.VarInt
		fmt.Printf("Count (pk.VarInt): %d\n", count)

		// Set it back
		if setter, ok := pkt.(models.CountSetter[pk.VarInt]); ok {
			setter.SetCount(count + 1)
		}
	} else if getter, ok := pkt.(models.CountGetter[pk.Short]); ok {
		// Some versions use pk.Short
		count := getter.GetCount() // Returns pk.Short
		fmt.Printf("Count (pk.Short): %d\n", count)
	}

	// For fields with multiple types across versions, use generics
	// Try as pk.Int first
	if getter, ok := pkt.(models.EntityIdGetter[pk.Int]); ok {
		entityId := getter.GetEntityId() // Returns pk.Int
		fmt.Printf("EntityId (as pk.Int): %d\n", entityId)
	} else if getter, ok := pkt.(models.EntityIdGetter[pk.VarInt]); ok {
		// Or try as pk.VarInt
		entityId := getter.GetEntityId() // Returns pk.VarInt
		fmt.Printf("EntityId (as pk.VarInt): %d\n", entityId)
	}

	// Note: This pattern doesn't work for version-specific types like
	// PlayerInfoActionBitflags. Use Pattern 1 or 3 for those.
}

// Pattern 3: VERSION-SPECIFIC ACCESS (Full type safety)
// Import specific version and use typed getter/setter methods.
// This gives you full type safety but requires knowing the version.
func ExampleVersionSpecificAccess() {
	// Working with v1.21.1
	pkt1211 := v1211.NewPlayerInfo()

	// Get the Action field with full type information
	action := pkt1211.GetAction() // Returns PlayerInfoActionBitflags

	// Access bitflag methods with full IDE support
	if action.AddPlayer() {
		fmt.Println("Add player flag is set")
	}

	// Set fields with type safety
	newAction := v1211.PlayerInfoActionBitflags{}
	newAction.SetAddPlayer(true)
	newAction.SetInitializeChat(true)
	pkt1211.SetAction(newAction)

	// Working with v1.21.6 - same code structure but different types
	pkt1216 := v1216.NewPlayerInfo()
	action1216 := pkt1216.GetAction() // Returns v1.21.6 PlayerInfoActionBitflags

	if action1216.AddPlayer() {
		fmt.Println("Add player flag is set (v1.21.6)")
	}
}

// Practical example: Version-agnostic client handling PlayerInfo
func HandlePlayerInfoPacket(pkt models.PacketMarshaller, version string) {
	// Use Pattern 1 for version-specific fields like Action
	fields := pkt.GetFields()

	if actionField, ok := fields["Action"]; ok {
		// Handle based on version
		switch version {
		case "1.21.1":
			if action, ok := actionField.(v1211.PlayerInfoActionBitflags); ok {
				fmt.Printf("v1.21.1 - AddPlayer: %v, UpdateGameMode: %v\n",
					action.AddPlayer(), action.UpdateGameMode())
			}
		case "1.21.6":
			if action, ok := actionField.(v1216.PlayerInfoActionBitflags); ok {
				fmt.Printf("v1.21.6 - AddPlayer: %v, UpdateGameMode: %v\n",
					action.AddPlayer(), action.UpdateGameMode())
			}
		}
	}

	// Use Pattern 2 for stable fields
	// (PlayerInfo.Data might have stable structure)
	if dataGetter, ok := pkt.(interface{ GetData() any }); ok {
		data := dataGetter.GetData()
		fmt.Printf("Player data: %v\n", data)
	}
}

// When to use each pattern:
//
// Pattern 1 (GetFields/SetFields):
//   - Working with packets dynamically (e.g., proxy/debugging tools)
//   - Fields with version-specific types (bitflags wrappers, custom types)
//   - Generic packet handlers that work across all versions
//
// Pattern 2 (Typed interfaces):
//   - Fields with primitive types (pk.VarInt, pk.String, etc.)
//   - Semi-agnostic code that works across versions with minor type variations
//   - When you want some type safety without committing to a version
//
// Pattern 3 (Version-specific getters):
//   - You know the protocol version
//   - Need full type safety and IDE support
//   - Working with version-specific features
//   - Maximum compile-time guarantees
