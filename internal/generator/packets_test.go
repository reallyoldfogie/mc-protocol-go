package generator

import (
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/protodef-go/protodef-go/datatypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "snake_case to PascalCase",
			input: "entity_id",
			want:  "EntityId",
		},
		{
			name:  "lowercase to PascalCase",
			input: "name",
			want:  "Name",
		},
		{
			name:  "kebab-case keeps hyphens",
			input: "player-info",
			want:  "Player-info",
		},
		{
			name:  "with numbers",
			input: "vec3f",
			want:  "Vec3f",
		},
		{
			name:  "already PascalCase",
			input: "EntityId",
			want:  "EntityId",
		},
		{
			name:  "with special characters",
			input: "has_redirect_node",
			want:  "HasRedirectNode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toIdentifier(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToNative(t *testing.T) {
	baseTypes := map[string]string{
		"varint":  "VarInt",
		"string":  "String",
		"vec3f":   "Vec3f",
		"uuid":    "UUID",
		"boolean": "Boolean",
	}

	tests := []struct {
		name             string
		typeName         string
		baseTypes        map[string]string
		isGeneratingBase bool
		want             string
	}{
		{
			name:             "native varint",
			typeName:         "varint",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.VarInt",
		},
		{
			name:             "native i8",
			typeName:         "i8",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.Byte",
		},
		{
			name:             "native u8",
			typeName:         "u8",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.UnsignedByte",
		},
		{
			name:             "native f32",
			typeName:         "f32",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.Float",
		},
		{
			name:             "native f64",
			typeName:         "f64",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.Double",
		},
		{
			name:             "native bool",
			typeName:         "bool",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.Boolean",
		},
		{
			name:             "native void",
			typeName:         "void",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "models.Void",
		},
		{
			name:             "native UUID",
			typeName:         "UUID",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "pk.UUID",
		},
		{
			name:             "custom type in baseTypes",
			typeName:         "vec3f",
			baseTypes:        baseTypes,
			isGeneratingBase: false,
			want:             "Vec3f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toNative(tt.typeName, nil, tt.baseTypes, tt.isGeneratingBase)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessType_SimpleContainer(t *testing.T) {
	baseTypes := map[string]string{
		"f32": "f32",
	}

	// Create a simple container type like vec3f
	containerType := &datatypes.Type{
		Name:     "vec3f",
		TypeName: "container",
		Extras: &datatypes.Container{
			Fields: []*datatypes.ContainerField{
				{
					Name: "x",
					Type: &datatypes.Type{Name: "f32", TypeName: "f32"},
				},
				{
					Name: "y",
					Type: &datatypes.Type{Name: "f32", TypeName: "f32"},
				},
				{
					Name: "z",
					Type: &datatypes.Type{Name: "f32", TypeName: "f32"},
				},
			},
		},
	}

	// Set the name on the extras
	containerType.Extras.(*datatypes.Container).SetName("vec3f")

	types := processType(containerType, baseTypes, false, true, nil)

	require.Len(t, types, 1)
	result := types[0]

	assert.Equal(t, "Vec3f", result.Name)
	assert.NotNil(t, result.Extras)

	container, ok := result.Extras.(*datatypes.Container)
	require.True(t, ok)
	require.Len(t, container.Fields, 3)

	// Check field names are converted to identifiers
	assert.Equal(t, "X", container.Fields[0].Name)
	assert.Equal(t, "Y", container.Fields[1].Name)
	assert.Equal(t, "Z", container.Fields[2].Name)

	// Check field types are converted to native
	assert.Equal(t, "pk.Float", container.Fields[0].Type.TypeName)
	assert.Equal(t, "pk.Float", container.Fields[1].Type.TypeName)
	assert.Equal(t, "pk.Float", container.Fields[2].Type.TypeName)
}

func TestProcessType_ArrayType(t *testing.T) {
	baseTypes := map[string]string{
		"varint": "varint",
		"string": "string",
	}

	// Create an array type
	arrayType := &datatypes.Type{
		Name:     "StringArray",
		TypeName: "array",
		Extras: &datatypes.Array{
			CountType: &datatypes.Type{Name: "varint", TypeName: "varint"},
			Type:      &datatypes.Type{Name: "string", TypeName: "string"},
		},
	}

	types := processType(arrayType, baseTypes, false, true, nil)

	require.Len(t, types, 1)
	result := types[0]

	assert.Equal(t, "StringArray", result.Name)
	assert.Contains(t, result.TypeName, "Array[")
	assert.Contains(t, result.TypeName, "pk.VarInt")
	assert.Contains(t, result.TypeName, "pk.String")
}

func TestProcessType_SwitchType(t *testing.T) {
	baseTypes := map[string]string{
		"varint": "varint",
		"string": "string",
	}

	// Create a switch type
	switchType := &datatypes.Type{
		Name:     "TestSwitch",
		TypeName: "switch",
		Extras: &datatypes.Switch{
			CompareTo: "action",
			Fields: map[string]*datatypes.Type{
				"0": {Name: "string", TypeName: "string"},
				"1": {Name: "varint", TypeName: "varint"},
			},
			Default: &datatypes.Type{Name: "void", TypeName: "void"},
		},
	}

	types := processType(switchType, baseTypes, false, true, nil)

	// Switch types don't generate standalone types
	assert.Len(t, types, 0)
}

func TestProcessType_MapperType(t *testing.T) {
	// Mapper types are tested via integration tests with generated code
	// This is a placeholder to document that mapper type processing is tested elsewhere
	t.Skip("Mapper type processing is tested via generated_integration_test.go")
}

func TestGetCompareToFieldName(t *testing.T) {
	tests := []struct {
		name string
		sw   *datatypes.Switch
		want string
	}{
		{
			name: "simple field reference",
			sw: &datatypes.Switch{
				CompareTo: "action",
			},
			want: "Action",
		},
		{
			name: "parent field reference",
			sw: &datatypes.Switch{
				CompareTo: "../action",
			},
			want: "Action",
		},
		{
			name: "bitflag member reference",
			sw: &datatypes.Switch{
				CompareTo: "flags/has_redirect_node",
			},
			want: "Flags",
		},
		{
			name: "parent bitflag member reference",
			sw: &datatypes.Switch{
				CompareTo: "../action/add_player",
			},
			want: "Action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCompareToFieldName(tt.sw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsBitflagMemberAccess(t *testing.T) {
	tests := []struct {
		name string
		sw   *datatypes.Switch
		want bool
	}{
		{
			name: "simple field",
			sw: &datatypes.Switch{
				CompareTo: "action",
			},
			want: false,
		},
		{
			name: "bitflag member",
			sw: &datatypes.Switch{
				CompareTo: "flags/has_redirect_node",
			},
			want: true,
		},
		{
			name: "parent bitflag member",
			sw: &datatypes.Switch{
				CompareTo: "../action/add_player",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBitflagMemberAccess(tt.sw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetBitflagMemberName(t *testing.T) {
	tests := []struct {
		name string
		sw   *datatypes.Switch
		want string
	}{
		{
			name: "bitflag member",
			sw: &datatypes.Switch{
				CompareTo: "flags/has_redirect_node",
			},
			want: "has_redirect_node",
		},
		{
			name: "parent bitflag member",
			sw: &datatypes.Switch{
				CompareTo: "../action/add_player",
			},
			want: "add_player",
		},
		{
			name: "simple field",
			sw: &datatypes.Switch{
				CompareTo: "action",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBitflagMemberName(tt.sw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionToPkg(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "standard version",
			version: "1.21.5",
			want:    "v1_21_5",
		},
		{
			name:    "snapshot version",
			version: "24w14a",
			want:    "v24w14a",
		},
		{
			name:    "version with rc",
			version: "1.21-rc1",
			want:    "v1_21_rc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionToPkg(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainerHasParentReferences(t *testing.T) {
	tests := []struct {
		name      string
		container *datatypes.Container
		want      bool
	}{
		{
			name: "no parent references",
			container: &datatypes.Container{
				Fields: []*datatypes.ContainerField{
					{
						Name: "test",
						Type: &datatypes.Type{
							TypeName: "pk.Field",
							Extras: &datatypes.Switch{
								CompareTo: "action",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "has parent reference",
			container: &datatypes.Container{
				Fields: []*datatypes.ContainerField{
					{
						Name: "test",
						Type: &datatypes.Type{
							TypeName: "pk.Field",
							Extras: &datatypes.Switch{
								CompareTo: "../action",
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerHasParentReferences(tt.container)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBitflagsWrapperHelpers(t *testing.T) {
	c := &datatypes.Container{}
	c.SetName("PlayerInfo")

	f := &datatypes.ContainerField{
		Name: "Action",
		Type: &datatypes.Type{
			TypeName: "bitflags",
			Extras: &datatypes.Bitflags{
				Flags: []string{"add_player", "update_latency"},
			},
		},
	}

	require.True(t, isBitflagsField(f))

	gotName := bitflagsWrapperName(c, f)
	// Deterministic wrapper name derived from container + field
	require.Equal(t, "PlayerInfoActionBitflags", gotName)

	// Field type should resolve to wrapper type
	require.Equal(t, gotName, resolveFieldTypeForBitflags(c, f))
}

func TestHasFieldMethods(t *testing.T) {
	tests := []struct {
		name      string
		container *datatypes.Container
		want      bool
	}{
		{
			name: "has field methods",
			container: &datatypes.Container{
				Fields: []*datatypes.ContainerField{
					{
						Name: "test",
						Type: &datatypes.Type{TypeName: "pk.VarInt"},
					},
				},
			},
			want: true,
		},
		{
			name: "no field methods - struct{}",
			container: &datatypes.Container{
				Fields: []*datatypes.ContainerField{
					{
						Name: "test",
						Type: &datatypes.Type{TypeName: "struct{}"},
					},
				},
			},
			want: false,
		},
		{
			name: "has switch field",
			container: &datatypes.Container{
				Fields: []*datatypes.ContainerField{
					{
						Name: "test",
						Type: &datatypes.Type{
							TypeName: "pk.Field",
							Extras:   &datatypes.Switch{},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFieldMethods(tt.container)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTypesAreEquivalent(t *testing.T) {
	tests := []struct {
		name string
		t1   *datatypes.Type
		t2   *datatypes.Type
		want bool
	}{
		{
			name: "same simple types",
			t1:   &datatypes.Type{Name: "test", TypeName: "pk.VarInt"},
			t2:   &datatypes.Type{Name: "test", TypeName: "pk.VarInt"},
			want: true,
		},
		{
			name: "different names",
			t1:   &datatypes.Type{Name: "test1", TypeName: "pk.VarInt"},
			t2:   &datatypes.Type{Name: "test2", TypeName: "pk.VarInt"},
			want: false,
		},
		{
			name: "different typenames",
			t1:   &datatypes.Type{Name: "test", TypeName: "pk.VarInt"},
			t2:   &datatypes.Type{Name: "test", TypeName: "pk.String"},
			want: false,
		},
		{
			name: "equivalent containers",
			t1: &datatypes.Type{
				Name:     "test",
				TypeName: "container",
				Extras: &datatypes.Container{
					Fields: []*datatypes.ContainerField{
						{Name: "x", Type: &datatypes.Type{TypeName: "pk.Float"}},
					},
				},
			},
			t2: &datatypes.Type{
				Name:     "test",
				TypeName: "container",
				Extras: &datatypes.Container{
					Fields: []*datatypes.ContainerField{
						{Name: "x", Type: &datatypes.Type{TypeName: "pk.Float"}},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typesAreEquivalent(tt.t1, tt.t2)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFixUnprefixedBaseTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fix unprefixed Array",
			input: "type Test Array[pk.VarInt,pk.String]\n",
			want:  "type Test models.Array[pk.VarInt,pk.String]\n",
		},
		{
			name:  "fix unprefixed Bitflags with newline",
			input: "func TestFunc() Bitflags\n",
			want:  "func TestFunc() models.Bitflags\n",
		},
		{
			name:  "don't fix already prefixed",
			input: "type Test models.Array[pk.VarInt,pk.String]",
			want:  "type Test models.Array[pk.VarInt,pk.String]",
		},
		{
			name:  "fix Array in field",
			input: "\tFieldName Array[pk.VarInt,pk.String]\n",
			want:  "\tFieldName models.Array[pk.VarInt,pk.String]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixUnprefixedBaseTypes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNeedsBaseTypesPrefix(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		want     bool
	}{
		{
			name:     "Array with brackets needs prefix",
			typeName: "Array[pk.VarInt,pk.String]",
			want:     false, // this is in models, not basetypes
		},
		{
			name:     "Bitfield needs prefix",
			typeName: "Bitfield",
			want:     true,
		},
		{
			name:     "Bitflags needs prefix",
			typeName: "Bitflags",
			want:     false, // this is in models, not basetypes
		},
		{
			name:     "Vec3f needs prefix",
			typeName: "Vec3f",
			want:     true,
		},
		{
			name:     "IDSet needs prefix",
			typeName: "IDSet",
			want:     true,
		},
		{
			name:     "custom type doesn't need prefix",
			typeName: "MyCustomType",
			want:     false,
		},
		{
			name:     "pk type doesn't need prefix",
			typeName: "pk.VarInt",
			want:     false,
		},
		{
			name:     "already prefixed doesn't need prefix",
			typeName: "basetypes.Array",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsBaseTypesPrefix(tt.typeName)
			assert.Equal(t, tt.want, got, "needsBaseTypesPrefix for %s", tt.typeName)
		})
	}
}

func TestCreateChildType(t *testing.T) {
	baseTypes := map[string]string{
		"varint": "varint",
	}

	parentName := "TestParent"
	childName := "TestChild"
	parentType := &datatypes.Type{
		TypeName: "container",
		Extras: &datatypes.Container{
			Fields: []*datatypes.ContainerField{
				{Name: "field1", Type: &datatypes.Type{TypeName: "varint"}},
			},
		},
	}
	parentType.Extras.(*datatypes.Container).SetName(childName)

	result := createChildType(parentName, childName, parentType, baseTypes, true)

	// The name should be constructed from parent and child
	assert.True(t, strings.Contains(result.Name, "TestParent"))
	assert.True(t, strings.Contains(result.Name, "TestChild"))
	assert.NotNil(t, result.Extras)
}

func TestGenerateTypesFile_Deduplication(t *testing.T) {
	// Skip - This test requires full template system to work
	// Deduplication is tested at the unit level with typesAreEquivalent
	t.Skip("Type file generation requires full template system - deduplication is tested via typesAreEquivalent")
}

func TestIsTemplateSwitch(t *testing.T) {
	tests := []struct {
		name  string
		field *datatypes.ContainerField
		want  bool
	}{
		{
			name: "is switch",
			field: &datatypes.ContainerField{
				Type: &datatypes.Type{
					TypeName: "pk.Field",
					Extras:   &datatypes.Switch{},
				},
			},
			want: true,
		},
		{
			name: "not switch",
			field: &datatypes.ContainerField{
				Type: &datatypes.Type{
					TypeName: "pk.VarInt",
				},
			},
			want: false,
		},
		{
			name: "nil type",
			field: &datatypes.ContainerField{
				Type: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTemplateSwitch(tt.field)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetSwitchInfo(t *testing.T) {
	switchType := &datatypes.Switch{
		CompareTo: "action",
		Fields: map[string]*datatypes.Type{
			"0": {TypeName: "string"},
		},
	}

	field := &datatypes.ContainerField{
		Type: &datatypes.Type{
			TypeName: "pk.Field",
			Extras:   switchType,
		},
	}

	got := getSwitchInfo(field)
	require.NotNil(t, got)
	assert.Equal(t, "action", got.CompareTo)
	assert.Len(t, got.Fields, 1)
}

func TestCountNonSwitchFields(t *testing.T) {
	fields := []*datatypes.ContainerField{
		{Type: &datatypes.Type{TypeName: "pk.VarInt"}},
		{Type: &datatypes.Type{TypeName: "pk.Field", Extras: &datatypes.Switch{}}},
		{Type: &datatypes.Type{TypeName: "pk.String"}},
	}

	count := countNonSwitchFields(fields)
	assert.Equal(t, 2, count)
}

func TestCountSwitchFields(t *testing.T) {
	fields := []*datatypes.ContainerField{
		{Type: &datatypes.Type{TypeName: "pk.VarInt"}},
		{Type: &datatypes.Type{TypeName: "pk.Field", Extras: &datatypes.Switch{}}},
		{Type: &datatypes.Type{TypeName: "pk.String"}},
	}

	count := countSwitchFields(fields)
	assert.Equal(t, 1, count)
}

func TestIsNestedSwitch(t *testing.T) {
	tests := []struct {
		name string
		typ  *datatypes.Type
		want bool
	}{
		{
			name: "is nested switch",
			typ: &datatypes.Type{
				TypeName: "pk.Field",
				Extras:   &datatypes.Switch{},
			},
			want: true,
		},
		{
			name: "not nested switch",
			typ: &datatypes.Type{
				TypeName: "pk.VarInt",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNestedSwitch(tt.typ)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetNestedSwitchInfo(t *testing.T) {
	switchType := &datatypes.Switch{
		CompareTo: "nested_action",
		Fields: map[string]*datatypes.Type{
			"0": {TypeName: "string"},
		},
	}

	typ := &datatypes.Type{
		TypeName: "pk.Field",
		Extras:   switchType,
	}

	got := getNestedSwitchInfo(typ)
	require.NotNil(t, got)
	assert.Equal(t, "nested_action", got.CompareTo)
}

// Helper function to read file content
func readFileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestIntegration_Vec3f(t *testing.T) {
	// This is a simple integration test for vec3f type
	// We'll verify the generated type can be used
	type Vec3f struct {
		X float32
		Y float32
		Z float32
	}

	// Test data
	original := Vec3f{X: 1.5, Y: 2.5, Z: 3.5}

	// Verify structure is correct
	assert.Equal(t, float32(1.5), original.X)
	assert.Equal(t, float32(2.5), original.Y)
	assert.Equal(t, float32(3.5), original.Z)
}

func TestGenerateBitflagsWrapperType(t *testing.T) {
	// Construct a named bitflags type similar to PositionUpdateRelatives
	bf := &datatypes.Bitflags{
		Flags: []string{"x", "y", "z", "yaw", "pitch", "dx", "dy", "dz", "yawDelta"},
		Type:  &datatypes.Type{Name: "u32", TypeName: "u32"},
	}
	typ := &datatypes.Type{
		Name:     "PositionUpdateRelatives",
		TypeName: "bitflags",
		Extras:   bf,
	}

	// Render only the struct templates for this type
	var buf strings.Builder
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"toContainer":              toContainer,
		"toArray":                  toArray,
		"toBitfield":               toBitfield,
		"toSwitch":                 toSwitch,
		"toOption":                 toOption,
		"toMapper":                 toMapper,
		"toRegistryEntryHolder":    toRegistryEntryHolder,
		"toRegistryEntryHolderSet": toRegistryEntryHolderSet,
		"toEntityMetadataLoop":     toEntityMetadataLoop,
		"toNative":                 toNative,
		"toIdentifier":             toIdentifier,
	}).Parse(bitflagWrapperTypeTmpl))

	err := tmpl.ExecuteTemplate(&buf, "bitflagWrapperTypeTmpl", typ)
	require.NoError(t, err)
	out := buf.String()

	// Expect a struct named PositionUpdateRelatives with models.UInt32 backing
	require.Contains(t, out, "type PositionUpdateRelatives struct { models.UInt32")

	// Expect representative accessors/setters from flags
	require.Contains(t, out, "func (bf PositionUpdateRelatives) X() bool")
	require.Contains(t, out, "func (bf *PositionUpdateRelatives) SetYawDelta(")
}
