package models_test

import (
	"testing"

	pk "github.com/Tnze/go-mc/net/packet"

	v1_21_6_serverbound "github.com/reallyoldfogie/mc-protocol-go/data/1.21.6/play/serverbound"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

// BenchmarkDirectFieldAccess measures direct getter/setter performance
func BenchmarkDirectFieldAccess(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	b.ResetTimer()
	for range b.N {
		pkt.SetCount(100)
		_ = pkt.GetCount()
	}
}

// BenchmarkInterfaceFieldAccess measures interface-based getter/setter performance
func BenchmarkInterfaceFieldAccess(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()
	var marshaller models.PacketMarshaller = pkt

	getter, _ := marshaller.(models.CountGetter[pk.VarInt])
	setter, _ := marshaller.(models.CountSetter[pk.VarInt])

	b.ResetTimer()
	for range b.N {
		setter.SetCount(100)
		_ = getter.GetCount()
	}
}

// BenchmarkDeprecatedMapFieldAccess measures old map-based API performance
func BenchmarkDeprecatedMapFieldAccess(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	b.ResetTimer()
	for range b.N {
		pkt.SetFields(map[string]pk.FieldEncoder{
			"Count": pk.VarInt(100),
		})
		fields := pkt.GetFields()
		_ = fields["Count"].(pk.VarInt)
	}
}

// BenchmarkTypeAssertion measures type assertion overhead
func BenchmarkTypeAssertion(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()
	var marshaller models.PacketMarshaller = pkt

	b.ResetTimer()
	for range b.N {
		if getter, ok := marshaller.(models.CountGetter[pk.VarInt]); ok {
			_ = getter.GetCount()
		}
	}
}

// BenchmarkVersionAgnosticCheck measures typical version-agnostic pattern
func BenchmarkVersionAgnosticCheck(b *testing.B) {
	packets := []models.PacketMarshaller{
		v1_21_6_serverbound.NewMessageAcknowledgement(),
		v1_21_6_serverbound.NewQueryEntityNbt(),
		v1_21_6_serverbound.NewLockDifficulty(),
	}

	b.ResetTimer()
	for range b.N {
		for _, pkt := range packets {
			if getter, ok := pkt.(models.CountGetter[pk.VarInt]); ok {
				_ = getter.GetCount()
			}
		}
	}
}

// BenchmarkMultipleFieldAccess measures accessing multiple fields
func BenchmarkMultipleFieldAccess(b *testing.B) {
	pkt := v1_21_6_serverbound.NewQueryEntityNbt()

	b.ResetTimer()
	for range b.N {
		if getter, ok := interface{}(pkt).(models.TransactionIdGetter[pk.VarInt]); ok {
			_ = getter.GetTransactionId()
		}
		if getter, ok := interface{}(pkt).(models.EntityIdGetter[pk.VarInt]); ok {
			_ = getter.GetEntityId()
		}
	}
}

// BenchmarkInterfaceAllocation measures if interface conversions allocate
func BenchmarkInterfaceAllocation(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if getter, ok := interface{}(pkt).(models.CountGetter[pk.VarInt]); ok {
			_ = getter.GetCount()
		}
	}
}

// BenchmarkGetFieldsMapCreation measures map allocation in old API
func BenchmarkGetFieldsMapCreation(b *testing.B) {
	pkt := v1_21_6_serverbound.NewMessageAcknowledgement()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = pkt.GetFields()
	}
}
