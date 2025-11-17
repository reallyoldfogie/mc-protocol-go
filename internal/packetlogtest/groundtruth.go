package packetlogtest

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"

	"github.com/reallyoldfogie/mc-protocol-go/models"
	pk "github.com/Tnze/go-mc/net/packet"
)

// GroundTruthTestCase represents a test packet with known field values
type GroundTruthTestCase struct {
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	PacketName  string                 `json:"packetName"`
	PacketID    json.RawMessage        `json:"packetId"` // Can be int or string from node-minecraft-protocol
	GroundTruth map[string]interface{} `json:"groundTruth"`
	Serialized  string                 `json:"serialized"` // hex-encoded
	Timestamp   string                 `json:"timestamp"`
}

// GroundTruthResult represents the result of validating against ground truth
type GroundTruthResult struct {
	Description string
	PacketName  string
	PacketID    int32
	Success     bool
	Errors      []string
}

// ValidateGroundTruth validates our Go parser against ground truth test cases
func ValidateGroundTruth(testFile string, packetMgr models.PacketMgr) ([]GroundTruthResult, error) {
	f, err := os.Open(testFile)
	if err != nil {
		return nil, fmt.Errorf("open test file: %w", err)
	}
	defer f.Close()

	var results []GroundTruthResult
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var testCase GroundTruthTestCase
		if err := json.Unmarshal([]byte(line), &testCase); err != nil {
			return nil, fmt.Errorf("line %d: parse test case: %w", lineNum, err)
		}

		result := validateTestCase(testCase, packetMgr)
		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}

	return results, nil
}

func validateTestCase(tc GroundTruthTestCase, packetMgr models.PacketMgr) GroundTruthResult {
	result := GroundTruthResult{
		Description: tc.Description,
		PacketName:  tc.PacketName,
		Success:     true,
	}

	// Decode hex bytes
	data, err := hex.DecodeString(tc.Serialized)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("decode hex: %v", err))
		return result
	}

	// Parse the packet bytes
	// The serialized data includes the packet ID as the first VarInt
	reader := bytes.NewReader(data)

	var packetID pk.VarInt
	if _, err := packetID.ReadFrom(reader); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("read packet ID: %v", err))
		return result
	}

	// Get remaining data
	remaining, err := io.ReadAll(reader)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("read remaining data: %v", err))
		return result
	}

	packet := pk.Packet{
		ID:   int32(packetID),
		Data: remaining,
	}

	// Store the packet ID in the result
	result.PacketID = packet.ID

	// Create packet instance using manager (assuming play state for now)
	packetInstance, err := packetMgr.GetClientboundPacketByID(models.ClientboundPacketID(packet.ID))
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("create packet instance: %v", err))
		return result
	}

	// Scan the packet
	if err := packetInstance.Scan(packet); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("scan packet: %v", err))
		return result
	}

	// Compare fields
	errors := compareFields(tc.GroundTruth, packetInstance, tc.PacketName)
	if len(errors) > 0 {
		result.Success = false
		result.Errors = append(result.Errors, errors...)
	}

	return result
}

func compareFields(expected map[string]interface{}, packet interface{}, packetName string) []string {
	var errors []string

	// Use reflection to access packet fields
	v := reflect.ValueOf(packet)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return []string{fmt.Sprintf("packet is not a struct: %v", v.Kind())}
	}

	t := v.Type()

	// Check each expected field
	for fieldName, expectedValue := range expected {
		// Find the field in the struct (case-insensitive)
		var fieldValue reflect.Value
		var found bool

		for i := 0; i < t.NumField(); i++ {
			structField := t.Field(i)
			if structField.Name == fieldName ||
			   toLowerFirst(structField.Name) == fieldName {
				fieldValue = v.Field(i)
				found = true
				break
			}
		}

		if !found {
			errors = append(errors, fmt.Sprintf("field '%s' not found in packet struct", fieldName))
			continue
		}

		// Compare values
		if err := compareValue(fieldName, expectedValue, fieldValue); err != nil {
			errors = append(errors, err.Error())
		}
	}

	return errors
}

func compareValue(fieldName string, expected interface{}, actual reflect.Value) error {
	// Get the actual value
	var actualValue interface{}

	// Handle different types
	switch actual.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		actualValue = actual.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		actualValue = actual.Uint()
	case reflect.Float32, reflect.Float64:
		actualValue = actual.Float()
	case reflect.String:
		actualValue = actual.String()
	case reflect.Bool:
		actualValue = actual.Bool()
	default:
		// For complex types, try to get the underlying value
		if actual.CanInterface() {
			actualValue = actual.Interface()
		} else {
			return fmt.Errorf("field '%s': cannot access value", fieldName)
		}
	}

	// Compare based on expected type
	switch exp := expected.(type) {
	case float64:
		// JSON numbers are always float64
		switch act := actualValue.(type) {
		case float64:
			if !floatEqual(exp, act) {
				return fmt.Errorf("field '%s': expected %v, got %v", fieldName, exp, act)
			}
		case float32:
			if !floatEqual(exp, float64(act)) {
				return fmt.Errorf("field '%s': expected %v, got %v", fieldName, exp, act)
			}
		case int64:
			if exp != float64(act) {
				return fmt.Errorf("field '%s': expected %v, got %v", fieldName, exp, act)
			}
		case uint64:
			if exp != float64(act) {
				return fmt.Errorf("field '%s': expected %v, got %v", fieldName, exp, act)
			}
		default:
			return fmt.Errorf("field '%s': expected number %v, got %T(%v)", fieldName, exp, act, act)
		}
	case string:
		if str, ok := actualValue.(string); ok {
			if exp != str {
				return fmt.Errorf("field '%s': expected %q, got %q", fieldName, exp, str)
			}
		} else {
			return fmt.Errorf("field '%s': expected string %q, got %T(%v)", fieldName, exp, actualValue, actualValue)
		}
	case bool:
		if b, ok := actualValue.(bool); ok {
			if exp != b {
				return fmt.Errorf("field '%s': expected %v, got %v", fieldName, exp, b)
			}
		} else {
			return fmt.Errorf("field '%s': expected bool %v, got %T(%v)", fieldName, exp, actualValue, actualValue)
		}
	default:
		// Generic comparison
		if !reflect.DeepEqual(expected, actualValue) {
			return fmt.Errorf("field '%s': expected %v (%T), got %v (%T)",
				fieldName, expected, expected, actualValue, actualValue)
		}
	}

	return nil
}

func floatEqual(a, b float64) bool {
	// Use a small epsilon for floating point comparison
	const epsilon = 1e-9
	return math.Abs(a-b) < epsilon
}

func toLowerFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = rune(runes[0] + ('a' - 'A'))
	return string(runes)
}
