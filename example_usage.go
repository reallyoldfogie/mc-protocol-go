// Build tag to exclude from normal builds
//go:build ignore

package main

import (
	"fmt"

	pk "github.com/Tnze/go-mc/net/packet"
	v1_21_1 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.1/play/serverbound"
	v1_21_6 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/serverbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// Example: Version-agnostic packet processor using generic interfaces
// This function works with packets from ANY protocol version
func processAnyPacketWithCount(pkt models.PacketMarshaller) {
	fmt.Printf("Processing packet ID: %d\n", pkt.PacketID())

	// Check if this packet has a Count field
	if getter, ok := pkt.(models.CountGetter); ok {
		count := getter.GetCount()
		fmt.Printf("  Found Count field: %d\n", count)

		// Modify it if needed
		if setter, ok := pkt.(models.CountSetter); ok {
			setter.SetCount(count + 1)
			fmt.Printf("  Incremented Count to: %d\n", getter.GetCount())
		}
		return
	}

	fmt.Println("  No Count field in this packet")
}

// Example: Direct field access when you know the type
func directAccess() {
	fmt.Println("\n=== Direct Field Access ===")

	// Create a 1.21.6 MessageAcknowledgement packet
	pkt := v1_21_6.NewMessageAcknowledgement()

	// Set and get field directly - type-safe!
	pkt.SetCount(42)
	count := pkt.GetCount()

	fmt.Printf("Direct access - Count: %d\n", count)
}

// Example: Version-agnostic handling
func versionAgnosticHandling() {
	fmt.Println("\n=== Version-Agnostic Handling ===")

	// Create packets from different versions
	pkt1_21_6 := v1_21_6.NewMessageAcknowledgement()
	pkt1_21_6.SetCount(100)

	pkt1_21_1 := v1_21_1.NewEntityAction()
	// EntityAction doesn't have a Count field

	// Process both with the same function
	fmt.Println("Processing 1.21.6 packet:")
	processAnyPacketWithCount(pkt1_21_6)

	fmt.Println("\nProcessing 1.21.1 packet:")
	processAnyPacketWithCount(pkt1_21_1)
}

// Example: Multiple field checks with generic interfaces
func multipleFieldChecks() {
	fmt.Println("\n=== Multiple Field Checks ===")

	processEntityPacket := func(pkt models.PacketMarshaller) {
		// EntityId and Action are both generic
		// Check for EntityId with pk.VarInt type
		idGetter, hasId := pkt.(models.EntityIdGetter[pk.VarInt])
		// Check for Action with pk.VarInt type (most common for EntityAction)
		actionGetter, hasAction := pkt.(models.ActionGetter[pk.VarInt])

		if hasId && hasAction {
			fmt.Printf("Entity ID (VarInt): %v, Action (VarInt): %v\n",
				idGetter.GetEntityId(),
				actionGetter.GetAction())
		} else {
			fmt.Printf("Packet fields: EntityId=%v, Action=%v\n", hasId, hasAction)
		}
	}

	pkt := v1_21_1.NewEntityAction()
	processEntityPacket(pkt)
}

// Example: Generic interface with multiple type handling
func genericInterfaceExample() {
	fmt.Println("\n=== Generic Interface Type Safety ===")

	// Count can be pk.VarInt or pk.Short depending on packet/version
	pkt := v1_21_6.NewMessageAcknowledgement()
	pkt.SetCount(42)

	// Type-safe access - we know this packet uses pk.VarInt
	// Convert to interface first, then type assert
	var pktInterface models.PacketMarshaller = pkt
	if getter, ok := pktInterface.(models.CountGetter); ok {
		count := getter.GetCount() // Returns pk.VarInt directly!
		fmt.Printf("Count (type-safe): %d\n", count)

		// Can use it directly without type assertion
		var varIntValue pk.VarInt = count // No type assertion needed!
		fmt.Printf("Direct assignment: %d\n", varIntValue)
	}

	fmt.Println("\n✅ Full compile-time type safety with generics!")
}

func main() {
	fmt.Println("Type-Safe Field Accessor Examples")
	fmt.Println("==================================")

	directAccess()
	versionAgnosticHandling()
	multipleFieldChecks()
	genericInterfaceExample()

	fmt.Println("\n✅ All examples completed successfully!")
	fmt.Println("\nFor more details, see:")
	fmt.Println("  - docs/field-access-patterns.md")
	fmt.Println("  - docs/generic-field-accessors.md")
}
