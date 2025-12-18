package models

import "time"

type PacketLog struct {
	ID              int32     `json:"id"`
	Name            string    `json:"name"`
	Timestamp       time.Time `json:"timestamp"`
	Data            []byte    `json:"data"`
	Version         string    `json:"version"`
	ProtocolVersion uint      `json:"protocol_version"`
	Direction       string    `json:"direction"` // "clientbound" or "serverbound"
	State           string    `json:"state"`     // "handshaking", "status", "login", "play"
}
