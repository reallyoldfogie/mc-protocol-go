package generator

import (
	"testing"
)

func TestFormatRawDefinition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple single line",
			input:    `"varint"`,
			expected: `"varint"`,
		},
		{
			name:     "single line with spaces",
			input:    `  "i32"  `,
			expected: `"i32"`,
		},
		{
			name: "multi-line container",
			input: `["container", [
	{"name": "x", "type": "i32"},
	{"name": "y", "type": "i32"}
]]`,
			expected: `["container", [
// 	{"name": "x", "type": "i32"},
// 	{"name": "y", "type": "i32"}
// ]]`,
		},
		{
			name: "multi-line array definition",
			input: `["array", {
	"countType": "varint",
	"type": "u16"
}]`,
			expected: `["array", {
// 	"countType": "varint",
// 	"type": "u16"
// }]`,
		},
		{
			name: "multi-line switch",
			input: `["switch", {
	"compareTo": "type",
	"fields": {
		"1": "i32",
		"2": "buffer"
	}
}]`,
			expected: `["switch", {
// 	"compareTo": "type",
// 	"fields": {
// 		"1": "i32",
// 		"2": "buffer"
// 	}
// }]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRawDefinition(tt.input)
			if result != tt.expected {
				t.Errorf("formatRawDefinition() =\n%q\n\nwant:\n%q", result, tt.expected)
			}
		})
	}
}
