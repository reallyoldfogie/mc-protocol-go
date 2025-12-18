package models

import (
	"fmt"
	"io"
)

// FixedBuffer3 represents a fixed-size buffer of 3 bytes
type FixedBuffer3 [3]byte

func (fb *FixedBuffer3) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer3) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer4 represents a fixed-size buffer of 4 bytes
type FixedBuffer4 [4]byte

func (fb *FixedBuffer4) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer4) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer8 represents a fixed-size buffer of 8 bytes
type FixedBuffer8 [8]byte

func (fb *FixedBuffer8) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer8) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer16 represents a fixed-size buffer of 16 bytes
type FixedBuffer16 [16]byte

func (fb *FixedBuffer16) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer16) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer20 represents a fixed-size buffer of 20 bytes
type FixedBuffer20 [20]byte

func (fb *FixedBuffer20) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer20) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer32 represents a fixed-size buffer of 32 bytes
type FixedBuffer32 [32]byte

func (fb *FixedBuffer32) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer32) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer64 represents a fixed-size buffer of 64 bytes
type FixedBuffer64 [64]byte

func (fb *FixedBuffer64) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer64) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer128 represents a fixed-size buffer of 128 bytes
type FixedBuffer128 [128]byte

func (fb *FixedBuffer128) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer128) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// FixedBuffer256 represents a fixed-size buffer of 256 bytes
type FixedBuffer256 [256]byte

func (fb *FixedBuffer256) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.ReadFull(r, fb[:])
	return int64(n), err
}

func (fb FixedBuffer256) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(fb[:])
	return int64(n), err
}

// getFixedBufferTypeName returns the appropriate FixedBufferN type name for a given size
// Returns empty string if no predefined type exists for that size
func getFixedBufferTypeName(size int) string {
	switch size {
	case 3:
		return "models.FixedBuffer3"
	case 4:
		return "models.FixedBuffer4"
	case 8:
		return "models.FixedBuffer8"
	case 16:
		return "models.FixedBuffer16"
	case 20:
		return "models.FixedBuffer20"
	case 32:
		return "models.FixedBuffer32"
	case 64:
		return "models.FixedBuffer64"
	case 128:
		return "models.FixedBuffer128"
	case 256:
		return "models.FixedBuffer256"
	default:
		return ""
	}
}

// GetFixedBufferTypeName is exported for use by the generator
func GetFixedBufferTypeName(size int) (string, error) {
	typeName := getFixedBufferTypeName(size)
	if typeName == "" {
		return "", fmt.Errorf("no predefined FixedBuffer type for size %d", size)
	}
	return typeName, nil
}
