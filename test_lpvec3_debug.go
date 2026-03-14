package models

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/reallyoldfogie/mc-protocol-go/models"
)

func TestDebugWriteTo(t *testing.T) {
	v := models.LpVec3{X: -0.5, Y: -0.25, Z: -0.75}
	buf := &bytes.Buffer{}
	n, err := v.WriteTo(buf)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	fmt.Printf("Wrote %d bytes: %02x\n", n, buf.Bytes())
}
