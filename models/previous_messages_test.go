package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pk "github.com/Tnze/go-mc/net/packet"

	"github.com/reallyoldfogie/mc-protocol-go/data/1.21.7/basetypes"
	"github.com/reallyoldfogie/mc-protocol-go/models"
)

func TestConvertPreviousMessagesToPackedSignatures_V1_21_7(t *testing.T) {
	// Build a PreviousMessages with two entries: one inline signature, one reference
	// Inline signature (Id==0)
	var fb models.FixedBuffer256
	for i := 0; i < len(fb); i++ {
		fb[i] = byte(i)
	}

	elems := []basetypes.PreviousMessagesPreviousMessagesElement{
		{Id: pk.VarInt(0), Signature: &fb},            // inline
		{Id: pk.VarInt(3), Signature: &models.Void{}}, // reference (index 2 on wire, i.e., ID=2)
	}

	pm := basetypes.PreviousMessages{}
	pm.Ary.Ary = elems

	got, err := models.ConvertPreviousMessagesToPackedSignatures(pm)
	assert.NoError(t, err)
	if assert.Len(t, got, 2) {
		// First: inline
		assert.Equal(t, int32(-1), got[0].ID)
		if assert.NotNil(t, got[0].Signature) {
			// Compare first few bytes
			for i := 0; i < 8; i++ {
				assert.Equal(t, fb[i], got[0].Signature[i])
			}
		}

		// Second: reference
		assert.Equal(t, int32(2), got[1].ID) // 3-1
		assert.Nil(t, got[1].Signature)
	}

	// Also verify pointer-to-slice variant is handled
	pm2 := basetypes.PreviousMessages{}
	pm2.Ary.Ary = &elems
	got2, err := models.ConvertPreviousMessagesToPackedSignatures(pm2)
	assert.NoError(t, err)
	assert.Equal(t, got, got2)
}
