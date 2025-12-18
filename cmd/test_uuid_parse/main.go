package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/reallyoldfogie/mc-protocol-go/data/1.21.5/login/clientbound"
)

func main() {
	// Hex dump from the actual packet:
	// 36 ca e0 e6 2a a1 3e 2f ba 45 5e 43 d8 f7 6e ce - UUID (16 bytes)
	// 07 - String length (7)
	// 52 4f 46 5f 62 6f 74 - "ROF_bot"
	// 00 - Properties array length (0)

	payloadHex := "36cae0e62aa13e2fba455e43d8f76ece0752" +
		"4f465f626f7400"

	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Payload hex: %s\n", hex.EncodeToString(payload))
	fmt.Printf("Payload length: %d bytes\n", len(payload))
	fmt.Printf("First 16 bytes (UUID): %s\n", hex.EncodeToString(payload[:16]))

	// Create a packet
	packet := pk.Packet{
		ID:   2, // LoginSuccess
		Data: payload,
	}

	// Try to parse it
	success := clientbound.NewSuccess()
	err = success.Scan(packet)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("SUCCESS!\n")
		fmt.Printf("UUID: %v\n", success.GetUuid())
		fmt.Printf("Username: %v\n", success.GetUsername())
		fmt.Printf("Properties: %v\n", success.GetProperties())
	}

	// Also try reading directly
	fmt.Printf("\n--- Direct ReadFrom test ---\n")
	reader := bytes.NewReader(payload)
	var uuid pk.UUID
	n, err := uuid.ReadFrom(reader)
	if err != nil {
		fmt.Printf("ERROR reading UUID: %v (read %d bytes)\n", err, n)
	} else {
		fmt.Printf("UUID read successfully: %v (%d bytes)\n", uuid, n)
	}
}
