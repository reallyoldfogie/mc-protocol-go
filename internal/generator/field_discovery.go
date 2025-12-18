package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/protodef-go/protodef-go/datatypes"
	"github.com/protodef-go/protodef-go/protocol"
)

// FieldInfo contains metadata about a field found across protocol versions
type FieldInfo struct {
	Name     string          // Field name (e.g., "Count", "X")
	Types    map[string]bool // All Go types seen for this field
	Versions map[string]bool // Which versions have this field
}

// FieldAccessorData is used for generating the field_accessors.go template
type FieldAccessorData struct {
	Fields []FieldAccessorInfo
}

// FieldAccessorInfo contains info for generating a single field accessor interface pair
type FieldAccessorInfo struct {
	Name        string   // Field name
	Type        string   // Go type (constraint name for generic interface)
	Types       []string // All types (for generating constraints)
	Constraint  string   // Type constraint name
	VersionList string   // Comma-separated list of versions
}

// DiscoverFields scans all protocol definitions across versions to find unique fields.
// It returns a map of field name -> FieldInfo containing type and version information.
func DiscoverFields(versionProtocols map[string]*protocol.Protocol) (map[string]*FieldInfo, error) {
	fields := make(map[string]*FieldInfo)

	for version, proto := range versionProtocols {
		if proto == nil {
			continue
		}

		fmt.Printf("[FieldDiscovery] Scanning version %s...\n", version)

		// Scan all namespaces (login, play, configuration, etc.)
		for _, namespace := range proto.Namespaces {
			if namespace == nil {
				continue
			}

			// Check both toClient and toServer
			for _, direction := range []string{"toClient", "toServer"} {
				boundNS := namespace.Namespaces[direction]
				if boundNS == nil {
					continue
				}

				// Scan all types in this namespace
				for _, t := range boundNS.Types {
					if t == nil {
						continue
					}

					// Only process container types (structs/packets)
					if t.TypeName != "container" || t.Extras == nil {
						continue
					}

					container, ok := t.Extras.(*datatypes.Container)
					if !ok || container == nil {
						continue
					}

					// Extract fields from this container
					for _, field := range container.Fields {
						if field == nil || field.Type == nil {
							continue
						}

						fieldName := toIdentifier(field.Name)
						if fieldName == "" {
							continue
						}

						// Resolve the field's Go type
						fieldType := resolveFieldTypeString(field, container)
						if fieldType == "" {
							continue
						}
						// Normalize type for interface generation
						fieldType = normalizeTypeForInterface(fieldType)

						// Add or update field info
						if existing, ok := fields[fieldName]; ok {
							// Field exists, add version and type
							existing.Versions[version] = true
							existing.Types[fieldType] = true
							if len(existing.Types) > 1 {
								fmt.Printf("[FieldDiscovery] Field '%s' has multiple types (will use generic): %v\n",
									fieldName, getTypeNames(existing.Types))
							}
						} else {
							// New field
							fields[fieldName] = &FieldInfo{
								Name:     fieldName,
								Types:    map[string]bool{fieldType: true},
								Versions: map[string]bool{version: true},
							}
						}
					}
				}
			}

			// Also scan base types for container types
			for _, t := range proto.Types {
				if t == nil {
					continue
				}

				if t.TypeName != "container" || t.Extras == nil {
					continue
				}

				container, ok := t.Extras.(*datatypes.Container)
				if !ok || container == nil {
					continue
				}

				for _, field := range container.Fields {
					if field == nil || field.Type == nil {
						continue
					}

					fieldName := toIdentifier(field.Name)
					if fieldName == "" {
						continue
					}

					fieldType := resolveFieldTypeString(field, container)
					if fieldType == "" {
						continue
					}
					// Normalize type for interface generation
					fieldType = normalizeTypeForInterface(fieldType)

					if existing, ok := fields[fieldName]; ok {
						existing.Versions[version] = true
						existing.Types[fieldType] = true
						if len(existing.Types) > 1 {
							fmt.Printf("[FieldDiscovery] Field '%s' in basetypes has multiple types (will use generic): %v\n",
								fieldName, getTypeNames(existing.Types))
						}
					} else {
						fields[fieldName] = &FieldInfo{
							Name:     fieldName,
							Types:    map[string]bool{fieldType: true},
							Versions: map[string]bool{version: true},
						}
					}
				}
			}
		}
	}

	fmt.Printf("[FieldDiscovery] Found %d unique fields across all versions\n", len(fields))
	return fields, nil
}

// resolveFieldTypeString converts a ContainerField to its Go type string representation
func resolveFieldTypeString(field *datatypes.ContainerField, container *datatypes.Container) string {
	if field == nil || field.Type == nil {
		return ""
	}

	// Use the existing resolveFieldType helper if available, or implement basic resolution
	// For now, we'll implement a simplified version that handles common cases
	t := field.Type

	switch t.TypeName {
	case "varint":
		return "pk.VarInt"
	case "varlong":
		return "pk.VarLong"
	case "i8":
		return "pk.Byte"
	case "u8":
		return "pk.UByte"
	case "i16":
		return "pk.Short"
	case "u16":
		return "pk.UShort"
	case "i32":
		return "pk.Int"
	case "u32":
		return "pk.UInt"
	case "i64":
		return "pk.Long"
	case "u64":
		return "pk.ULong"
	case "f32":
		return "pk.Float"
	case "f64":
		return "pk.Double"
	case "bool":
		return "pk.Boolean"
	case "string":
		return "pk.String"
	case "UUID":
		return "pk.UUID"
	case "buffer":
		return "[]byte"
	case "void":
		return "models.Void"

	case "array":
		// For arrays, we need the element type
		if t.Name != "" {
			return toIdentifier(t.Name)
		}
		return "models.Array[any]" // Generic fallback

	case "option":
		// For options, we need the wrapped type
		if t.Name != "" {
			return toIdentifier(t.Name)
		}
		return "models.Option[any]" // Generic fallback

	case "container":
		// Use the type's Name if available
		if t.Name != "" {
			return toIdentifier(t.Name)
		}
		return "any" // Fallback for anonymous containers

	default:
		// Try to use the Name field
		if t.Name != "" {
			name := toIdentifier(t.Name)
			// If name already has a package prefix, return as-is
			if strings.Contains(name, ".") {
				return name
			}
			// Otherwise return the identifier
			return name
		}
		// Check if TypeName itself is a valid identifier
		if t.TypeName != "" && !strings.Contains(t.TypeName, " ") {
			return toIdentifier(t.TypeName)
		}
		return "any"
	}
}

// getTypeNames returns a sorted slice of type names from a map
func getTypeNames(types map[string]bool) []string {
	names := make([]string, 0, len(types))
	for t := range types {
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}

// getConstraintName generates a type constraint name for a field with multiple types
func getConstraintName(fieldName string) string {
	return fieldName + "Type"
}

// normalizeTypeForInterface normalizes a type string for use in field accessor interfaces.
// For Phase 1, we take a conservative approach:
// - Keep pk.* types (they're always available)
// - Keep models.Void, models.NBTField, models.Bitflags (core models types)
// - Keep []byte
// - Everything else becomes `any` (version-specific types, basetypes, etc.)
//
// In Phase 2, when we generate getter/setter methods on actual packet types,
// those will use the correct concrete types.
func normalizeTypeForInterface(typeName string) string {
	if typeName == "" || typeName == "any" {
		return "any"
	}

	// Keep pk.* types - they're from the go-mc package
	if strings.HasPrefix(typeName, "pk.") {
		return typeName
	}

	// Keep []byte
	if typeName == "[]byte" {
		return typeName
	}

	// Keep only specific models types that are always available
	// But strip the models. prefix since we're generating code IN the models package
	if typeName == "models.Void" {
		return "Void"
	}
	if typeName == "models.NBTField" {
		return "NBTField"
	}
	if typeName == "models.Bitflags" {
		return "Bitflags"
	}

	// Everything else (version-specific types, basetypes, generic types with
	// version-specific parameters, etc.) becomes `any`
	return "any"
}

// GenerateFieldAccessors creates the models/field_accessors.go file with all field interfaces
func GenerateFieldAccessors(fields map[string]*FieldInfo, outputDir string) error {
	// Sort fields by name for consistent output
	fieldNames := make([]string, 0, len(fields))
	for name := range fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	// Build the data structure for the template
	accessors := make([]FieldAccessorInfo, 0, len(fieldNames))
	for _, name := range fieldNames {
		info := fields[name]

		// Build version list
		versions := make([]string, 0, len(info.Versions))
		for v := range info.Versions {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		versionList := strings.Join(versions, ", ")

		// Get sorted types
		types := getTypeNames(info.Types)

		// Always use generic interfaces
		constraint := getConstraintName(name)

		accessors = append(accessors, FieldAccessorInfo{
			Name:        name,
			Type:        constraint,
			Types:       types,
			Constraint:  constraint,
			VersionList: versionList,
		})
	}

	data := FieldAccessorData{
		Fields: accessors,
	}

	// Create the models directory if it doesn't exist
	modelsDir := filepath.Join(outputDir, "models")
	if err := os.MkdirAll(modelsDir, 0750); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Create the output file
	outPath := filepath.Join(modelsDir, "field_accessors.go")
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create field_accessors.go: %w", err)
	}
	defer outFile.Close()

	// Execute template
	tmpl := template.Must(template.New("field_accessors").Parse(fieldAccessorsTemplate))
	if err := tmpl.Execute(outFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	fmt.Printf("[FieldDiscovery] Generated %s with %d field interfaces\n", outPath, len(accessors))
	return nil
}

const fieldAccessorsTemplate = `// Code generated by mc-protocol-go generator. DO NOT EDIT.
package models

import pk "github.com/Tnze/go-mc/net/packet"

// Field accessor interfaces for packet field access.
// Use generic interfaces for all fields to ensure consistency across different version sets.
//
// GENERIC ACCESS (Recommended):
//    Use typed generic interfaces for fields across versions.
//    Example:
//      if getter, ok := pkt.(CountGetter[pk.VarInt]); ok {
//          count := getter.GetCount()
//      }
//      if getter, ok := pkt.(IdGetter[pk.Int]); ok {
//          id := getter.GetId()
//      }
//
// VERSION-SPECIFIC ACCESS (Full type safety):
//    Import specific version and use typed getter/setter methods.
//    Example:
//      import v1215 "github.com/.../data/1.21.5/play/clientbound"
//      pkt := v1215.PlayerInfo{}
//      action := pkt.GetAction() // PlayerInfoActionBitflags
//
// Note: Version-specific types cannot be used with the generic interfaces.
// Use version-specific access for those fields.
{{range .Fields}}
// {{.Constraint}} is a type constraint for the {{.Name}} field.
// This field has types: {{range $i, $t := .Types}}{{if $i}} | {{end}}{{$t}}{{end}}
type {{.Constraint}} interface {
	{{range $i, $t := .Types}}{{if $i}} | {{end}}{{$t}}{{end}}
}

// {{.Name}}Getter provides read access to the {{.Name}} field.
// Implemented by packet types in versions: {{.VersionList}}
type {{.Name}}Getter[T {{.Constraint}}] interface {
	Get{{.Name}}() T
}

// {{.Name}}Setter provides write access to the {{.Name}} field.
// Implemented by packet types in versions: {{.VersionList}}
type {{.Name}}Setter[T {{.Constraint}}] interface {
	Set{{.Name}}(T)
}
{{end}}
`
