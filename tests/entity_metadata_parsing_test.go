package tests_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	cb12110 "github.com/reallyoldfogie/mc-protocol-go/data/1.21.10/play/clientbound"
)

const (
	// v1_21_10_entity_metadata_packet is a packet log for a partially full shulker box in item form (19 of 27 slots have contents).
	// {components: {}, x: -996, y: 135, Items: [{components: {"minecraft:damage": 388, "minecraft:charged_projectiles": [{count: 1, id: "minecraft:arrow"}]}, count: 1, Slot: 0b, id: "minecraft:crossbow"}, {count: 4, Slot: 1b, id: "minecraft:lead"}, {count: 4, Slot: 2b, id: "minecraft:leather"}, {components: {"minecraft:item_name": {translate: "block.minecraft.ominous_banner"}, "minecraft:tooltip_display": {hidden_components: ["minecraft:banner_patterns"]}, "minecraft:rarity": "uncommon", "minecraft:banner_patterns": [{color: "cyan", pattern: "minecraft:rhombus"}, {color: "light_gray", pattern: "minecraft:stripe_bottom"}, {color: "gray", pattern: "minecraft:stripe_center"}, {color: "light_gray", pattern: "minecraft:border"}, {color: "black", pattern: "minecraft:stripe_middle"}, {color: "light_gray", pattern: "minecraft:half_horizontal"}, {color: "light_gray", pattern: "minecraft:circle"}, {color: "black", pattern: "minecraft:border"}]}, count: 1, Slot: 3b, id: "minecraft:white_banner"}, {count: 1, Slot: 4b, id: "minecraft:ominous_bottle"}, {count: 4, Slot: 5b, id: "minecraft:dead_bush"}, {count: 2, Slot: 6b, id: "minecraft:short_dry_grass"}, {count: 1, Slot: 7b, id: "minecraft:tall_dry_grass"}, {count: 50, Slot: 8b, id: "minecraft:snow"}, {count: 40, Slot: 9b, id: "minecraft:snow_block"}, {count: 7, Slot: 10b, id: "minecraft:bone_meal"}, {count: 8, Slot: 11b, id: "minecraft:oak_sapling"}, {count: 9, Slot: 12b, id: "minecraft:oak_log"}, {count: 3, Slot: 13b, id: "minecraft:dark_oak_planks"}, {count: 3, Slot: 14b, id: "minecraft:cherry_sign"}, {count: 1, Slot: 15b, id: "minecraft:light_gray_terracotta"}, {count: 2, Slot: 16b, id: "minecraft:spider_eye"}, {count: 5, Slot: 17b, id: "minecraft:arrow"}, {count: 2, Slot: 18b, id: "minecraft:string"}], z: -2991, id: "minecraft:shulker_box"}
	v1_21_10_entity_metadata_packet = `{"packet_id":97,"packet_name":"ClientboundEntityMetadata","data_length":187,"data":"n7wDCAcBxQQBAEITAaoKAgADhAMoAQH/BgAABOgJAAAE+QcAAAHtCQQABgoIAAl0cmFuc2xhdGUAHmJsb2NrLm1pbmVjcmFmdC5vbWlub3VzX2Jhbm5lcgAPAAE/CQE/CBgJIAghBwIIJQ8SCAQIAg8BzwsAAATPAQAAAtEBAAAB0gEAADLRAgAAKNMCAAAHuggAAAgxAAAJhgEAAAMqAAAD4QcAAAHuAwAAAuIIAAAF/wYAAAK0BwAA/w=="}`
	// v1_21_10_entity_metadata_packet = `{"packet_id":97,"packet_name":"ClientboundEntityMetadata","data_length":187,"data":"n7wDCAcBxQQBAEITAaoKAgADhAMpAQH/BgAABOgJAAAE+QcAAAHtCQQABgoIAAl0cmFuc2xhdGUAHmJsb2NrLm1pbmVjcmFmdC5vbWlub3VzX2Jhbm5lcgAPAAE/CQE/CBgJIAghBwIIJQ8SCAQIAg8BzwsAAATPAQAAAtEBAAAB0gEAADLRAgAAKNMCAAAHuggAAAgxAAAJhgEAAAMqAAAD4QcAAAHuAwAAAuIIAAAF/wYAAAK0BwAA/w=="}`

	// v1_21_10_entity_spawn_packet spawns in the shulker box item entity
	v1_21_10_entity_spawn_packet = `{"packet_id":1,"packet_name":"ClientboundSpawnEntity","data_length":54,"data":"n7wDF33K7SAlSI6KUbDQUw7hUkbAj408/DgrLkBgyB8F8zEqwKdtQaY7o1YR2H79MzEAWAAA"}`

	// full session log for reference:
	// `{"timestamp":"2026-03-27T08:24:09.9693438-06:00","direction":"clientbound","packet_id":0,"packet_name":"ClientboundVoid","data_length":0,"data":"","connection_name":"ROF_bot","source":"connection"}`
	// `{"timestamp":"2026-03-27T08:24:09.969414209-06:00","direction":"clientbound","packet_id":1,"packet_name":"ClientboundSpawnEntity","data_length":54,"data":"n7wDF33K7SAlSI6KUbDQUw7hUkbAj408/DgrLkBgyB8F8zEqwKdtQaY7o1YR2H79MzEAWAAA","connection_name":"ROF_bot","source":"connection"}`
	// `{"timestamp":"2026-03-27T08:24:09.96948432-06:00","direction":"clientbound","packet_id":97,"packet_name":"ClientboundEntityMetadata","data_length":187,"data":"n7wDCAcBxQQBAEITAaoKAgADhAMoAQH/BgAABOgJAAAE+QcAAAHtCQQABgoIAAl0cmFuc2xhdGUAHmJsb2NrLm1pbmVjcmFmdC5vbWlub3VzX2Jhbm5lcgAPAAE/CQE/CBgJIAghBwIIJQ8SCAQIAg8BzwsAAATPAQAAAtEBAAAB0gEAADLRAgAAKNMCAAAHuggAAAgxAAAJhgEAAAMqAAAD4QcAAAHuAwAAAuIIAAAF/wYAAAK0BwAA/w==","connection_name":"ROF_bot","source":"connection"}`
	// `{"timestamp":"2026-03-27T08:24:09.969532725-06:00","direction":"clientbound","packet_id":0,"packet_name":"ClientboundVoid","data_length":0,"data":"","connection_name":"ROF_bot","source":"connection"}`

)

func TestEntitySpawn_v1_21_10_Parse(t *testing.T) {
	var packetLog struct {
		PacketID int32  `json:"packet_id"`
		Data     string `json:"data"`
	}
	err := json.Unmarshal([]byte(v1_21_10_entity_spawn_packet), &packetLog)
	require.NoError(t, err, "failed to unmarshal packet log JSON")

	rawBytes, err := base64.StdEncoding.DecodeString(packetLog.Data)
	require.NoError(t, err, "failed to decode base64 data")

	packet := pk.Packet{
		ID:   packetLog.PacketID,
		Data: rawBytes,
	}

	fmt.Printf("[TestEntitySpawn_v1_21_10_Parse] decoded packet: ID: %d, Data: % X\n", packet.ID, packet.Data)

	spawnEntity := cb12110.NewSpawnEntity()
	err = spawnEntity.Scan(packet)
	require.NoError(t, err, "failed to scan SpawnEntity packet")

	spew.Dump(spawnEntity)

	require.Equal(t, int32(10), spawnEntity.PacketID(), "packet ID should be 1 for ClientboundSpawnEntity")

	// require.Greater(t, len(entityMetadata.GetMetadata().Entries), 0, "should have at least one metadata entry")
}

func TestEntityMetadata_v1_21_10_Parse(t *testing.T) {
	var packetLog struct {
		PacketID int32  `json:"packet_id"`
		Data     string `json:"data"`
	}
	err := json.Unmarshal([]byte(v1_21_10_entity_metadata_packet), &packetLog)
	require.NoError(t, err, "failed to unmarshal packet log JSON")

	rawBytes, err := base64.StdEncoding.DecodeString(packetLog.Data)
	require.NoError(t, err, "failed to decode base64 data")

	packet := pk.Packet{
		ID:   packetLog.PacketID,
		Data: rawBytes,
	}

	fmt.Printf("[TestEntityMetadata_v1_21_10_Parse] decoded packet: ID: %d, Data: % X\n\n", packet.ID, packet.Data)

	entityMetadata := cb12110.NewEntityMetadata()
	require.Equal(t, packet.ID, entityMetadata.PacketID(), "packet ID should match EntityMetadata packet ID")
	// err = entityMetadata.Scan(packet)
	r := bytes.NewReader(packet.Data)
	bytesRead, err := entityMetadata.ReadFrom(r)
	fmt.Printf("[TestEntityMetadata_v1_21_10_Parse] total bytes read from packet data: %d\n", bytesRead)

	spew.Dump(entityMetadata)

	require.NoError(t, err, "failed to scan EntityMetadata packet")

	require.Greater(t, len(entityMetadata.GetMetadata().Entries), 0, "should have at least one metadata entry")
}
