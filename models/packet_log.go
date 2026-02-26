package models

import "time"

type PacketLog struct {
	Timestamp       time.Time `json:"timestamp"`
	ID              int32     `json:"packet_id"`
	Name            string    `json:"name"`
	Data            []byte    `json:"data"`
	Version         string    `json:"version"`
	ProtocolVersion uint      `json:"protocol_version"`
	Direction       string    `json:"direction"` // "clientbound" or "serverbound"
	State           string    `json:"state"`     // "handshaking", "status", "login", "play"
	Source          string    `json:"source,"`
}
