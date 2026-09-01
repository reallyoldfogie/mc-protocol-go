package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"unicode"

	// "github.com/davecgh/go-spew/spew"
	"github.com/reallyoldfogie/mc-protocol-go/models"

	"github.com/reallyoldfogie/protodef-go/datatypes"
	"github.com/reallyoldfogie/protodef-go/namespace"
	"github.com/reallyoldfogie/protodef-go/protocol"
)

// Tracks which generated container types require parent context and which parent
// fields (and optional bitflag members) they reference. Keys are container type
// names (already identifier-ized), values are context keys like "action" or
// "action/add_player".
var parentContextRequirements = map[string][]string{}

// Tracks explicit count arrays: maps "ContainerName.FieldName" to count field name
var explicitCountArrayFields = map[string]string{}

// Tracks processed containers for expanding type aliases (case-insensitive key -> container)
var containerRegistry = map[string]*datatypes.Container{}

func generatePacketIDs(baseDir, version string, packetsData inversePacketParse) inversePacketParse {
	fmt.Println("generating", version, "packetid.go")

	err := os.MkdirAll(baseDir, 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	packetIdFile, err := os.Create(filepath.Join(baseDir, "packetid.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		panic(err)
	}
	defer packetIdFile.Close()

	pkgName := versionToPkg(version)
	fmt.Println("[generateSounds] setting package name ", version, "=>", pkgName)

	tmpl := strings.ReplaceAll(packetTemplate, "{{PACKAGE_NAME}}", pkgName)
	if err := template.Must(template.New("").Funcs(template.FuncMap{
		"contains":   strings.Contains,
		"trimPrefix": strings.TrimPrefix,
	}).Parse(tmpl)).Execute(packetIdFile, packetsData); err != nil {
		panic(err)
	}
	return packetsData
}

type inversePacketParse struct {
	Configuration             inversePacketMap
	Login                     inversePacketMap
	Play                      inversePacketMap
	Handshake                 inversePacketMap
	Status                    inversePacketMap
	MaxPlayClientboundID      models.ProtocolID
	MaxPlayServerboundID      models.ProtocolID
	MaxLoginClientboundID     models.ProtocolID
	MaxLoginServerboundID     models.ProtocolID
	MaxConfigClientboundID    models.ProtocolID
	MaxConfigServerboundID    models.ProtocolID
	MaxStatusClientboundID    models.ProtocolID
	MaxHandshakeClientboundID models.ProtocolID
	MaxHandshakeServerboundID models.ProtocolID
}
type packetData struct {
	Name          string // Name derived from packet_* proto type (matches generated struct names)
	ID            int64
	Hex           string // Hex representation of ID
	StructName    string // Actual generated struct name from protodef-go (for constructors)
	ProtoTypeName string // Original protocol type name (e.g., "packet_common_cookie_request")
	AltName       string // Alternative name from packets.json (go-mc compatible naming)
}
type inversePacketMap struct {
	Clientbound map[models.ProtocolID]packetData
	Serverbound map[models.ProtocolID]packetData
}

// TypeWithPacketInfo wraps a datatypes.Type with packet metadata
type TypeWithPacketInfo struct {
	*datatypes.Type
	IsPacket  bool
	PacketID  int32
	Namespace string // e.g., "login", "configuration", "play"
	Direction string // e.g., "clientbound", "serverbound"
}

func getPacketInfoFromProtocol(protocolDefinitions *protocol.Protocol) inversePacketParse {
	rval := inversePacketParse{
		Configuration: inversePacketMap{
			Clientbound: map[models.ProtocolID]packetData{},
			Serverbound: map[models.ProtocolID]packetData{},
		},
		Login: inversePacketMap{
			Clientbound: map[models.ProtocolID]packetData{},
			Serverbound: map[models.ProtocolID]packetData{},
		},
		Play: inversePacketMap{
			Clientbound: map[models.ProtocolID]packetData{},
			Serverbound: map[models.ProtocolID]packetData{},
		},
		Handshake: inversePacketMap{
			Clientbound: map[models.ProtocolID]packetData{},
			Serverbound: map[models.ProtocolID]packetData{},
		},
		Status: inversePacketMap{
			Clientbound: map[models.ProtocolID]packetData{},
			Serverbound: map[models.ProtocolID]packetData{},
		},
	}

	// caser := cases.Title(language.AmericanEnglish)

	buildFor := func(nsName string, inverseMap *inversePacketMap) {
		ns, ok := protocolDefinitions.Namespaces[nsName]
		if !ok || ns == nil {
			fmt.Printf("DEBUG [getPacketInfoFromProtocol]: Namespace '%s' not found or nil\n", nsName)
			fmt.Printf("DEBUG [getPacketInfoFromProtocol]: Available namespaces: %v\n", func() []string {
				keys := make([]string, 0, len(protocolDefinitions.Namespaces))
				for k := range protocolDefinitions.Namespaces {
					keys = append(keys, k)
				}
				return keys
			}())
			return
		}
		// Helper to process one direction
		processDir := func(direction string, out map[models.ProtocolID]packetData) {
			// Pull bound namespace
			var boundNS *namespace.Namespace
			if direction == "clientbound" {
				boundNS = ns.Namespaces["toClient"]
			} else {
				boundNS = ns.Namespaces["toServer"]
			}
			if boundNS == nil {
				return
			}
			// Find packet container
			var packetType *datatypes.Type
			for _, t := range boundNS.Types {
				if t.Name == "packet" {
					packetType = t
					break
				}
			}
			if packetType == nil || packetType.Extras == nil {
				return
			}
			container, ok := packetType.Extras.(*datatypes.Container)
			if !ok || len(container.Fields) < 2 {
				return
			}
			var mapper *datatypes.Mapper
			var switchType *datatypes.Switch
			for _, field := range container.Fields {
				if field.Name == "name" && field.Type != nil && field.Type.Extras != nil {
					if m, ok := field.Type.Extras.(*datatypes.Mapper); ok {
						mapper = m
					}
				}
				if field.Name == "params" && field.Type != nil && field.Type.Extras != nil {
					if s, ok := field.Type.Extras.(*datatypes.Switch); ok {
						switchType = s
					}
				}
			}
			if mapper == nil || switchType == nil {
				return
			}
			for idStr, symAny := range mapper.Mappings {
				sym, ok := symAny.(string)
				if !ok {
					continue
				}
				var id int64
				if _, err := fmt.Sscanf(idStr, "0x%x", &id); err != nil {
					continue
				}
				var protoTypeName string
				if typeRef, ok := switchType.Fields[sym]; ok && typeRef != nil {
					protoTypeName = typeRef.TypeName
					if protoTypeName == "" {
						protoTypeName = typeRef.Name
					}
				}
				// Name must be derived from packet_* type so it matches struct names exactly
				nameFromProto := toIdentifier(protoTypeName)
				// Handle void/empty types specially - use "Void" as identifier
				if nameFromProto == "struct{}" || nameFromProto == "" {
					nameFromProto = "models.Void"
				}
				pd := packetData{
					Name:          nameFromProto,
					ID:            id,
					Hex:           fmt.Sprintf("%0#2X", id),
					ProtoTypeName: protoTypeName,
				}
				out[models.ProtocolID{ID: id}] = pd
			}
		}
		processDir("clientbound", inverseMap.Clientbound)
		processDir("serverbound", inverseMap.Serverbound)
	}

	buildFor("configuration", &rval.Configuration)
	buildFor("login", &rval.Login)
	buildFor("play", &rval.Play)
	buildFor("handshaking", &rval.Handshake)
	fmt.Printf("DEBUG [getPacketInfoFromProtocol]: After buildFor handshaking: Clientbound=%d, Serverbound=%d\n",
		len(rval.Handshake.Clientbound), len(rval.Handshake.Serverbound))
	buildFor("status", &rval.Status)
	fmt.Printf("DEBUG [getPacketInfoFromProtocol]: After buildFor status: Clientbound=%d, Serverbound=%d\n",
		len(rval.Status.Clientbound), len(rval.Status.Serverbound))

	// Compute guards for all namespaces
	// Play
	rval.MaxPlayClientboundID = models.ProtocolID{ID: -1}
	for id := range rval.Play.Clientbound {
		if id.ID > rval.MaxPlayClientboundID.ID {
			rval.MaxPlayClientboundID = id
		}
	}
	rval.MaxPlayServerboundID = models.ProtocolID{ID: -1}
	for id := range rval.Play.Serverbound {
		if id.ID > rval.MaxPlayServerboundID.ID {
			rval.MaxPlayServerboundID = id
		}
	}
	// Guard is max+1
	rval.MaxPlayClientboundID.ID++
	rval.MaxPlayServerboundID.ID++

	// Login
	rval.MaxLoginClientboundID = models.ProtocolID{ID: -1}
	for id := range rval.Login.Clientbound {
		if id.ID > rval.MaxLoginClientboundID.ID {
			rval.MaxLoginClientboundID = id
		}
	}
	rval.MaxLoginServerboundID = models.ProtocolID{ID: -1}
	for id := range rval.Login.Serverbound {
		if id.ID > rval.MaxLoginServerboundID.ID {
			rval.MaxLoginServerboundID = id
		}
	}
	rval.MaxLoginClientboundID.ID++
	rval.MaxLoginServerboundID.ID++

	// Configuration
	rval.MaxConfigClientboundID = models.ProtocolID{ID: -1}
	for id := range rval.Configuration.Clientbound {
		if id.ID > rval.MaxConfigClientboundID.ID {
			rval.MaxConfigClientboundID = id
		}
	}
	rval.MaxConfigServerboundID = models.ProtocolID{ID: -1}
	for id := range rval.Configuration.Serverbound {
		if id.ID > rval.MaxConfigServerboundID.ID {
			rval.MaxConfigServerboundID = id
		}
	}
	rval.MaxConfigClientboundID.ID++
	rval.MaxConfigServerboundID.ID++

	// Status
	rval.MaxStatusClientboundID = models.ProtocolID{ID: -1}
	for id := range rval.Status.Clientbound {
		if id.ID > rval.MaxStatusClientboundID.ID {
			rval.MaxStatusClientboundID = id
		}
	}
	// Status serverbound doesn't typically have guards needed, but compute for consistency
	rval.MaxStatusClientboundID.ID++

	// Handshake
	rval.MaxHandshakeClientboundID = models.ProtocolID{ID: -1}
	for id := range rval.Handshake.Clientbound {
		if id.ID > rval.MaxHandshakeClientboundID.ID {
			rval.MaxHandshakeClientboundID = id
		}
	}
	rval.MaxHandshakeServerboundID = models.ProtocolID{ID: -1}
	for id := range rval.Handshake.Serverbound {
		if id.ID > rval.MaxHandshakeServerboundID.ID {
			rval.MaxHandshakeServerboundID = id
		}
	}
	rval.MaxHandshakeClientboundID.ID++
	rval.MaxHandshakeServerboundID.ID++

	return rval
}

// updatePacketDataWithStructNames updates the inversePacketParse with actual generated struct names
// This is called after struct generation so we know the mapping from packet IDs to struct names
func updatePacketDataWithStructNames(packetData *inversePacketParse, metadata map[string]struct {
	IsPacket  bool
	PacketID  int32
	Namespace string
	Direction string
}) {
	fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updating packet data with struct names from metadata\n")
	fmt.Printf("DEBUG [updatePacketDataWithStructNames]: metadata has %d entries\n", len(metadata))

	// Build namespace-aware map: namespace+direction+packetID -> structName
	// This prevents collisions when different namespaces have the same packet IDs
	type namespaceKey struct {
		namespace string
		direction string
		packetID  int32
	}
	idToStruct := make(map[namespaceKey]string)

	for structName, meta := range metadata {
		if meta.Namespace == "login" && meta.Direction == "clientbound" && meta.PacketID < 5 {
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Checking '%s': IsPacket=%v, Namespace='%s', Direction='%s', PacketID=%d\n",
				structName, meta.IsPacket, meta.Namespace, meta.Direction, meta.PacketID)
		}
		if meta.IsPacket && meta.Namespace != "" && meta.Direction != "" && meta.PacketID >= 0 {
			key := namespaceKey{
				namespace: meta.Namespace,
				direction: meta.Direction,
				packetID:  meta.PacketID,
			}
			idToStruct[key] = structName
			if meta.Namespace == "login" && meta.Direction == "clientbound" && meta.PacketID < 5 {
				fmt.Printf("DEBUG [updatePacketDataWithStructNames]: ADDED to idToStruct: LOGIN %s/%s ID %d -> struct '%s'\n",
					meta.Namespace, meta.Direction, meta.PacketID, structName)
			}
		}
	}

	// Debug: print all login/clientbound keys in idToStruct
	for k := range idToStruct {
		if k.namespace == "login" && k.direction == "clientbound" {
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: idToStruct has key login/clientbound ID %d\n", k.packetID)
		}
	}

	// Update all packet data with struct names using namespace-aware lookups
	for id, data := range packetData.Login.Clientbound {
		if id.ID < 2 {
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Looking up Login.Clientbound ID %d, have %d total entries in idToStruct\n", id.ID, len(idToStruct))
		}
		key := namespaceKey{namespace: "login", direction: "clientbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Login.Clientbound[id] = data
			if id.ID < 2 {
				fmt.Printf("DEBUG [updatePacketDataWithStructNames]: FOUND and Updated Login.Clientbound ID %d: Name='%s', StructName='%s'\n",
					id.ID, data.Name, data.StructName)
			}
		} else if id.ID < 2 {
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: NOT FOUND for Login.Clientbound ID %d\n", id.ID)
		}
	}
	for id, data := range packetData.Login.Serverbound {
		key := namespaceKey{namespace: "login", direction: "serverbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Login.Serverbound[id] = data
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updated Login.Serverbound ID %d: Name='%s', StructName='%s'\n",
				id.ID, data.Name, data.StructName)
		}
	}
	for id, data := range packetData.Configuration.Clientbound {
		key := namespaceKey{namespace: "configuration", direction: "clientbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Configuration.Clientbound[id] = data
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updated Configuration.Clientbound ID %d: Name='%s', StructName='%s'\n",
				id.ID, data.Name, data.StructName)
		}
	}
	for id, data := range packetData.Configuration.Serverbound {
		key := namespaceKey{namespace: "configuration", direction: "serverbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Configuration.Serverbound[id] = data
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updated Configuration.Serverbound ID %d: Name='%s', StructName='%s'\n",
				id.ID, data.Name, data.StructName)
		}
	}
	for id, data := range packetData.Play.Clientbound {
		key := namespaceKey{namespace: "play", direction: "clientbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Play.Clientbound[id] = data
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updated Play.Clientbound ID %d: Name='%s', StructName='%s'\n",
				id.ID, data.Name, data.StructName)
		}
	}
	for id, data := range packetData.Play.Serverbound {
		key := namespaceKey{namespace: "play", direction: "serverbound", packetID: int32(id.ID)}
		if structName, ok := idToStruct[key]; ok {
			data.StructName = structName
			packetData.Play.Serverbound[id] = data
			fmt.Printf("DEBUG [updatePacketDataWithStructNames]: Updated Play.Serverbound ID %d: Name='%s', StructName='%s'\n",
				id.ID, data.Name, data.StructName)
		}
	}
}

// extractPacketIDMapFromNamespace extracts packet ID to type name mappings from a namespace's packet container
// The packet container has a mapper (ID -> symbolic name) and a switch (symbolic name -> type name)
func extractPacketIDMapFromNamespace(ns *namespace.Namespace, direction string) map[int32]string {
	result := make(map[int32]string)

	// Get the appropriate bound namespace
	var boundNS *namespace.Namespace
	if direction == "clientbound" {
		boundNS = ns.Namespaces["toClient"]
	} else {
		boundNS = ns.Namespaces["toServer"]
	}

	if boundNS == nil {
		return result
	}

	// Find the "packet" type which contains the mapper and switch
	var packetType *datatypes.Type
	for _, t := range boundNS.Types {
		if t.Name == "packet" {
			packetType = t
			break
		}
	}

	if packetType == nil || packetType.Extras == nil {
		return result
	}

	// The packet type is a container with two fields: "name" (mapper) and "params" (switch)
	container, ok := packetType.Extras.(*datatypes.Container)
	if !ok || len(container.Fields) < 2 {
		return result
	}

	// Extract the mapper (ID -> symbolic name)
	var mapper *datatypes.Mapper
	var switchType *datatypes.Switch

	for _, field := range container.Fields {
		if field.Name == "name" && field.Type != nil && field.Type.Extras != nil {
			if m, ok := field.Type.Extras.(*datatypes.Mapper); ok {
				mapper = m
			}
		}
		if field.Name == "params" && field.Type != nil && field.Type.Extras != nil {
			if s, ok := field.Type.Extras.(*datatypes.Switch); ok {
				switchType = s
			}
		}
	}

	if mapper == nil || switchType == nil {
		return result
	}

	// Build the mapping: ID -> symbolic name -> type name
	for idStr, symbolicNameAny := range mapper.Mappings {
		// Type assert the symbolic name
		symbolicName, ok := symbolicNameAny.(string)
		if !ok {
			continue
		}

		// Parse the hex ID
		var id int64
		if _, err := fmt.Sscanf(idStr, "0x%x", &id); err != nil {
			continue
		}

		// Look up the type name in the switch
		if typeRef, ok := switchType.Fields[symbolicName]; ok && typeRef != nil {
			// Use TypeName (the original protodef-go name) not Name (the converted struct name)
			originalTypeName := typeRef.TypeName
			if originalTypeName == "" {
				originalTypeName = typeRef.Name
			}
			result[int32(id)] = originalTypeName
			fmt.Printf("DEBUG [extractPacketIDMapFromNamespace]: ID 0x%02X -> '%s' -> '%s' (TypeName='%s', Name='%s')\n",
				id, symbolicName, originalTypeName, typeRef.TypeName, typeRef.Name)
		}
	}

	return result
}

func generateProtocolStructs(version string, protocolDefinitions *protocol.Protocol, files map[string]string) (inversePacketParse, map[string]struct {
	IsPacket  bool
	PacketID  int32
	Namespace string
	Direction string
}, error) {
	// Reset global state for clean generation
	unnamedTypeCounters = map[string]int{}
	currentStructContext = ""
	packetMetadata = make(map[string]struct {
		IsPacket  bool
		PacketID  int32
		Namespace string
		Direction string
	})
	parentContextRequirements = make(map[string][]string)
	explicitCountArrayFields = make(map[string]string)
	containerRegistry = make(map[string]*datatypes.Container)
	// typeRegistry = make(map[string]*datatypes.Type)

	// Note: packet IDs will be extracted directly from protocol.json
	fmt.Printf("DEBUG [generateProtocolStructs]: Protocol contains %d namespaces\n", len(protocolDefinitions.Namespaces))

	errs := []error{}

	// DEBUG: List all types received
	fmt.Printf("DEBUG [generateProtocolStructs]: Received %d types:\n", len(protocolDefinitions.Types))
	for i, t := range protocolDefinitions.Types {
		if t.Name == "ContainerID" || t.Name == "optvarint" {
			fmt.Printf("  [%d] Name='%s' TypeName='%s'\n", i, t.Name, t.TypeName)
		}
	}

	// First pass: Build baseTypes map with all type names
	// This must be done before processType is called to handle forward references
	baseTypes := map[string]string{
		// Manually-defined basetypes that aren't in the protocol definition
		// Note: Do NOT add generic types like Array or Option here - they need type parameters
		"bitflags": "models.Bitflags",
		"idset":    "IDSet",
		"void":     "models.Void",
	}
	fmt.Printf("DEBUG [generateProtocolStructs]: Total types in protocolDefinitions.Types: %d\n", len(protocolDefinitions.Types))
	for _, t := range protocolDefinitions.Types {
		rawName := t.Name
		// DEBUG: Log specific types
		if t.Name == "ContainerID" || t.Name == "optvarint" {
			fmt.Printf("DEBUG [generateProtocolStructs - LOOP]: Found type name='%s', TypeName='%s', Extras=%v\n",
				t.Name, t.TypeName, t.Extras != nil)
		}
		// DEBUG: Log types with empty names
		if t.Name == "" && t.Extras != nil {
			extrasName := t.Extras.GetName()
			fmt.Printf("DEBUG [generateProtocolStructs]: TOP-LEVEL type has empty Name but Extras.GetName()='%s' TypeName='%s'\n", extrasName, t.TypeName)
		}
		// Skip meta-type keywords that don't have actual type names
		// A type like HashedSlot with TypeName="container" is an actual type, not a keyword
		switch t.Name {
		case "array", "switch", "container", "option", "bitfield":
			// These are meta-type keywords when used as type names, skip them
			continue
		case "":
			// Empty name means it's an inline type definition, skip it
			fmt.Printf("DEBUG: t.Name is empty, skipping type: %#v\n", t)
			continue
		default:
			// Add all named types to baseTypes, regardless of their TypeName
			// (e.g., HashedSlot has TypeName="container" but is a real type)
			// Use lowercase key for case-insensitive lookup
			baseTypes[strings.ToLower(rawName)] = t.Name
			if rawName == "ContainerID" || rawName == "optvarint" || rawName == "vec3i" || rawName == "HashedSlot" || rawName == "Slot" {
				fmt.Printf("DEBUG: Adding to baseTypes: '%s' -> '%s' (TypeName='%s')\n", rawName, t.Name, t.TypeName)
			}
		}
	}

	// DEBUG: Log specific types
	if val, ok := baseTypes["SlotComponentType"]; ok {
		fmt.Printf("DEBUG [generateProtocolStructs]: FIRST PASS - baseTypes['SlotComponentType'] = '%s'\n", val)
	}
	if val, ok := baseTypes["ContainerID"]; ok {
		fmt.Printf("DEBUG [generateProtocolStructs]: FIRST PASS - baseTypes['ContainerID'] = '%s'\n", val)
	} else {
		fmt.Printf("DEBUG [generateProtocolStructs]: FIRST PASS - ContainerID NOT in baseTypes\n")
	}

	// Second pass: Process types now that baseTypes map is complete
	types := make([]*datatypes.Type, 0)
	for _, t := range protocolDefinitions.Types {
		switch t.Name {
		case "array", "switch", "container", "option", "bitfield":
			continue
		default:
			newTypes := processType(t, baseTypes, false, true, nil) // true = isGeneratingBaseTypes
			types = append(types, newTypes...)
		}
	}

	// Third pass: Add all generated basetype names to the baseTypes map
	// This ensures that types like CommandNode that are generated from containers
	// are available for lookup when processing clientbound/serverbound types
	for _, t := range types {
		if t.Name != "" && !strings.Contains(t.Name, ".") {
			baseTypes[strings.ToLower(t.Name)] = t.Name
			fmt.Printf("DEBUG: Added generated basetype to map: '%s'\n", t.Name)
		}
	}

	err := generateMultipleTypesFiles(filepath.Join("data", version, "basetypes"), version, "basetypes", types, true)
	if err != nil {
		errs = append(errs, err)
	}

	// Create packet data for use in namespace processing using protocol.json only
	updatedPacketsData := getPacketInfoFromProtocol(protocolDefinitions)

	for name, namespace := range protocolDefinitions.Namespaces {
		err := processNamespace(version, name, namespace, baseTypes, updatedPacketsData)
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Return packet data and metadata - update will be done later with all versions' metadata
	if len(errs) > 0 {
		return inversePacketParse{}, nil, errors.Join(errs...)
	}
	return updatedPacketsData, packetMetadata, nil
}

// typeGroup represents a group of related types (e.g., a packet and its helpers)
type typeGroup struct {
	packetType   *datatypes.Type   // Main packet type (nil for shared types)
	relatedTypes []*datatypes.Type // Helper types used by this packet
}

// analyzeBasetypeDependencies categorizes basetypes by naming patterns and functionality
func analyzeBasetypeDependencies(types []*datatypes.Type) map[string]*typeGroup {
	// Build a map of type name -> type for quick lookup
	typeMap := make(map[string]*datatypes.Type)
	for _, t := range types {
		typeMap[t.Name] = t
	}

	// Category assignments: category name -> list of types
	categories := make(map[string][]*datatypes.Type)

	// Categorize each type based on naming patterns
	for _, t := range types {
		var category string

		// Check type name prefixes for categorization
		switch {
		case strings.HasPrefix(t.Name, "Common"):
			category = "common_types"
		case strings.HasPrefix(t.Name, "EntityMetadata"):
			category = "entity_metadata"
		case strings.HasPrefix(t.Name, "CommandNode"):
			category = "command_node"
		case strings.HasPrefix(t.Name, "Item") && !strings.HasPrefix(t.Name, "ItemStack"):
			// ItemStack is handled separately as it may be in models package
			category = "item_types"
		case strings.HasPrefix(t.Name, "ArmorTrim"):
			category = "armor_types"
		case t.Name == "Tags" || t.Name == "TagsTagsElement":
			category = "tags"
		case t.TypeName == "mapper":
			category = "mappers"
		case isSimpleAlias(t):
			category = "aliases"
		default:
			// Everything else goes to misc
			category = "misc_types"
		}

		categories[category] = append(categories[category], t)
	}

	// Track which types have been assigned to avoid duplicates
	assignedTypes := make(map[string]bool)

	// For each category, add only types that were directly categorized
	// (no dependency collection to avoid duplicates)
	groups := make(map[string]*typeGroup)
	for categoryName, categoryTypes := range categories {
		// Only include types that haven't been assigned yet
		typeList := make([]*datatypes.Type, 0, len(categoryTypes))
		for _, t := range categoryTypes {
			if !assignedTypes[t.Name] {
				typeList = append(typeList, t)
				assignedTypes[t.Name] = true
			}
		}

		// Skip empty groups
		if len(typeList) == 0 {
			continue
		}

		groups[categoryName] = &typeGroup{
			packetType:   nil,
			relatedTypes: typeList,
		}
	}

	return groups
}

// isSimpleAlias checks if a type is a simple type alias
func isSimpleAlias(t *datatypes.Type) bool {
	// Simple aliases are types that are just wrappers around another type
	// without complex logic (e.g., type Foo models.Option[Bar])
	if t.Extras != nil {
		return false // Has complex structure
	}
	// Check if it's a simple type reference
	if strings.Contains(t.TypeName, "models.") || strings.Contains(t.TypeName, "pk.") {
		return true
	}
	return false
}

// shouldIncludeInCategory determines if a dependent type should be included in the same file
func shouldIncludeInCategory(t *datatypes.Type, category string) bool {
	// Don't cross-pollinate major categories
	switch category {
	case "common_types":
		return strings.HasPrefix(t.Name, "Common")
	case "entity_metadata":
		return strings.HasPrefix(t.Name, "EntityMetadata")
	case "command_node":
		return strings.HasPrefix(t.Name, "CommandNode")
	case "item_types":
		return strings.HasPrefix(t.Name, "Item") && !strings.HasPrefix(t.Name, "ItemStack")
	case "armor_types":
		return strings.HasPrefix(t.Name, "ArmorTrim")
	case "tags":
		return t.Name == "Tags" || strings.Contains(t.Name, "Tags")
	case "mappers":
		return t.TypeName == "mapper"
	case "aliases":
		return isSimpleAlias(t)
	default:
		// For misc, be more permissive
		return true
	}
}

// analyzeDependencies analyzes type dependencies and groups them by packet
func analyzeDependencies(types []*datatypes.Type) map[string]*typeGroup {
	// Build a map of type name -> type for quick lookup
	typeMap := make(map[string]*datatypes.Type)
	for _, t := range types {
		typeMap[t.Name] = t
	}

	// Track which types are referenced by which packets
	typeUsage := make(map[string]map[string]bool)   // typeName -> set of packet names that use it
	packetTypes := make(map[string]*datatypes.Type) // packet name -> packet type

	// Identify packet types and build usage map
	for _, t := range types {
		if meta, exists := packetMetadata[t.Name]; exists && meta.IsPacket {
			packetTypes[t.Name] = t
			typeUsage[t.Name] = make(map[string]bool)
			typeUsage[t.Name][t.Name] = true // Packet uses itself

			// Find all types referenced by this packet
			collectTypeReferences(t, typeMap, typeUsage[t.Name])
		}
	}

	// Build groups: one per packet, plus one for shared types
	groups := make(map[string]*typeGroup)

	// Create a group for each packet
	for packetName, packetType := range packetTypes {
		group := &typeGroup{
			packetType:   packetType,
			relatedTypes: []*datatypes.Type{},
		}

		// Add referenced types to this group
		for typeName := range typeUsage[packetName] {
			if typeName != packetName { // Don't add packet itself as a related type
				if refType, ok := typeMap[typeName]; ok {
					group.relatedTypes = append(group.relatedTypes, refType)
				}
			}
		}

		groups[packetName] = group
	}

	// Identify shared types (used by multiple packets or not used by any packet)
	// First, count how many packets use each type
	typeToPackets := make(map[string][]string) // typeName -> list of packets that use it
	for packetName, usages := range typeUsage {
		for typeName := range usages {
			if typeName != packetName { // Don't count packet using itself
				typeToPackets[typeName] = append(typeToPackets[typeName], packetName)
			}
		}
	}

	// Determine which types are shared
	sharedTypeNames := make(map[string]bool)
	for typeName, packets := range typeToPackets {
		// Types used by 2+ packets are shared
		if len(packets) > 1 {
			sharedTypeNames[typeName] = true
		}
	}

	// Also mark mapper types and the main Packet struct as shared
	for _, t := range types {
		if t.TypeName == "mapper" || t.Name == "Packet" || t.Name == "PacketName" {
			sharedTypeNames[t.Name] = true
		}
	}

	// Remove shared types from packet groups
	for packetName, group := range groups {
		if packetName == "shared" {
			continue
		}

		// Filter out shared types from related types
		filteredRelated := []*datatypes.Type{}
		for _, t := range group.relatedTypes {
			if !sharedTypeNames[t.Name] {
				filteredRelated = append(filteredRelated, t)
			}
		}
		group.relatedTypes = filteredRelated
	}

	// Collect shared types
	sharedTypes := []*datatypes.Type{}
	for _, t := range types {
		// Skip packet types themselves
		if _, isPacket := packetTypes[t.Name]; isPacket {
			continue
		}

		if sharedTypeNames[t.Name] {
			sharedTypes = append(sharedTypes, t)
		}
	}

	// Also include types not used by any packet (orphaned types)
	for _, t := range types {
		// Skip packet types themselves
		if _, isPacket := packetTypes[t.Name]; isPacket {
			continue
		}

		// Skip if already marked as shared
		if sharedTypeNames[t.Name] {
			continue
		}

		// Check if type is used by any packet
		isUsed := false
		for _, usages := range typeUsage {
			if usages[t.Name] {
				isUsed = true
				break
			}
		}

		// If not used by any packet, add to shared types
		if !isUsed {
			sharedTypes = append(sharedTypes, t)
		}
	}

	// Create shared group
	if len(sharedTypes) > 0 {
		groups["shared"] = &typeGroup{
			packetType:   nil,
			relatedTypes: sharedTypes,
		}
	}

	return groups
}

// collectTypeReferences recursively collects all type names referenced by a type
func collectTypeReferences(t *datatypes.Type, typeMap map[string]*datatypes.Type, refs map[string]bool) {
	if t == nil || t.Extras == nil {
		return
	}

	switch extras := t.Extras.(type) {
	case *datatypes.Container:
		for _, field := range extras.Fields {
			if field.Type != nil {
				collectTypeReferencesFromTypeName(field.Type.TypeName, typeMap, refs)
				collectTypeReferences(field.Type, typeMap, refs)
			}
		}
	case *datatypes.Array:
		if extras.Type != nil {
			collectTypeReferencesFromTypeName(extras.Type.TypeName, typeMap, refs)
			collectTypeReferences(extras.Type, typeMap, refs)
		}
	case *datatypes.Switch:
		for _, field := range extras.Fields {
			if field != nil {
				collectTypeReferencesFromTypeName(field.TypeName, typeMap, refs)
			}
		}
	case *datatypes.Option:
		if extras.Type != nil {
			collectTypeReferencesFromTypeName(extras.Type.TypeName, typeMap, refs)
			collectTypeReferences(extras.Type, typeMap, refs)
		}
	}
}

// collectTypeReferencesFromTypeName extracts type names from a typename string
func collectTypeReferencesFromTypeName(typeName string, typeMap map[string]*datatypes.Type, refs map[string]bool) {
	// Handle generic types like "Array[Foo]" or "pk.Option[Bar]"
	if idx := strings.Index(typeName, "["); idx > 0 {
		// Extract type parameter
		if endIdx := strings.LastIndex(typeName, "]"); endIdx > idx {
			inner := typeName[idx+1 : endIdx]
			// Remove package prefixes
			inner = strings.TrimPrefix(inner, "basetypes.")
			inner = strings.TrimPrefix(inner, "models.")
			inner = strings.TrimPrefix(inner, "pk.")
			if _, ok := typeMap[inner]; ok {
				refs[inner] = true
			}
		}
		return
	}

	// Handle simple type references
	typeName = strings.TrimPrefix(typeName, "basetypes.")
	typeName = strings.TrimPrefix(typeName, "models.")
	typeName = strings.TrimPrefix(typeName, "pk.")
	if _, ok := typeMap[typeName]; ok {
		refs[typeName] = true
	}
}

// generateMultipleTypesFiles generates split types files for a namespace
func generateMultipleTypesFiles(basePath, version, packageName string, inTypes []*datatypes.Type, areBaseTypes bool) error {
	// Analyze dependencies and group types based on whether this is basetypes or packets
	var groups map[string]*typeGroup
	if areBaseTypes {
		groups = analyzeBasetypeDependencies(inTypes)
	} else {
		groups = analyzeDependencies(inTypes)
	}

	// Generate individual files for each group
	for groupName, group := range groups {
		// Skip empty groups
		if len(group.relatedTypes) == 0 && group.packetType == nil {
			continue
		}

		// Combine packet type with its related types
		types := []*datatypes.Type{}
		if group.packetType != nil {
			types = append(types, group.packetType)
		}
		types = append(types, group.relatedTypes...)

		// Generate file with appropriate naming
		var fileName string
		if areBaseTypes {
			// For basetypes, use the category name directly
			fileName = fmt.Sprintf("%s.go", groupName)
		} else {
			// For packets, use packet_ prefix or special names for shared types
			if groupName == "shared" {
				// Handle shared packet types separately below
				continue
			}
			fileName = fmt.Sprintf("packet_%s.go", strings.ToLower(groupName))
		}

		if err := generateSingleTypesFile(basePath, version, packageName, fileName, types, areBaseTypes); err != nil {
			return fmt.Errorf("failed to generate %s: %w", fileName, err)
		}
	}

	// Generate shared types file (only for packets, not basetypes)
	if !areBaseTypes {
		if sharedGroup, ok := groups["shared"]; ok {
			// Separate mapper types and packet struct from other shared types
			mapperTypes := []*datatypes.Type{}
			packetTypes := []*datatypes.Type{}
			otherShared := []*datatypes.Type{}

			for _, t := range sharedGroup.relatedTypes {
				if t.TypeName == "mapper" || strings.Contains(t.Name, "Mapper") || strings.Contains(t.Name, "Mappings") {
					mapperTypes = append(mapperTypes, t)
				} else if t.Name == "Packet" || t.Name == "PacketName" {
					packetTypes = append(packetTypes, t)
				} else {
					otherShared = append(otherShared, t)
				}
			}

			// Generate shared_types.go for common types
			if len(otherShared) > 0 {
				if err := generateSingleTypesFile(basePath, version, packageName, "shared_types.go", otherShared, false); err != nil {
					return fmt.Errorf("failed to generate shared_types.go: %w", err)
				}
			}

			// Generate packet_mapper.go for mapper types
			if len(mapperTypes) > 0 {
				if err := generateSingleTypesFile(basePath, version, packageName, "packet_mapper.go", mapperTypes, false); err != nil {
					return fmt.Errorf("failed to generate packet_mapper.go: %w", err)
				}
			}

			// Generate packet.go for main Packet struct
			if len(packetTypes) > 0 {
				if err := generateSingleTypesFile(basePath, version, packageName, "packet.go", packetTypes, false); err != nil {
					return fmt.Errorf("failed to generate packet.go: %w", err)
				}
			}
		}
	}

	return nil
}

// generateSingleTypesFile generates a single types file with the given types
func generateSingleTypesFile(basePath, version, packageName, fileName string, inTypes []*datatypes.Type, areBaseTypes bool) error {
	packageName = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, packageName)

	basePath = strings.ToLower(basePath)
	err := os.MkdirAll(basePath, 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create type package folder (basePath:%s) (package: %s) [%v]\n", basePath, packageName, err)
		return err
	}

	typesFile, err := os.Create(filepath.Join(basePath, fileName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", fileName, err)
		return err
	}
	defer typesFile.Close()

	// Build header
	var header string
	if areBaseTypes {
		header = strings.ReplaceAll(baseTypesHeader, "{{PACKAGE_NAME}}", packageName)
	} else {
		header = strings.ReplaceAll(typesHeader, "{{PACKAGE_NAME}}", packageName)
	}

	// Deduplicate types
	seenTypes := map[*datatypes.Type]int{}
	seenTypeNames := map[string]*datatypes.Type{}
	tmpTypes := []*datatypes.Type{}

	for _, t := range inTypes {
		if strings.Contains(t.Name, ".") {
			continue
		}

		if _, ok := seenTypes[t]; ok {
			continue
		}

		if existingType, ok := seenTypeNames[t.Name]; ok {
			if typesAreEquivalent(existingType, t) {
				continue
			}
			suffix := 1
			newName := fmt.Sprintf("%s_%d", t.Name, suffix)
			for _, exists := seenTypeNames[newName]; exists; _, exists = seenTypeNames[newName] {
				suffix++
				newName = fmt.Sprintf("%s_%d", t.Name, suffix)
			}
			fmt.Fprintf(os.Stderr, "Warning: duplicate type name '%s' detected. Renaming to '%s'\n", t.Name, newName)
			t.Name = newName
			if t.Extras != nil {
				t.Extras.SetName(newName)
			}
		}

		seenTypes[t] = 1
		seenTypeNames[t.Name] = t
		tmpTypes = append(tmpTypes, t)
	}

	inTypes = tmpTypes

	tmpConcatStructFile, _ := os.Create(filepath.Join("./tmp/full_struct.tmpl"))
	fmt.Fprintf(tmpConcatStructFile, "%s", structsTmpl+bitflagWrapperTmpl+bitflagWrapperTypeTmpl)
	tmpConcatStructFile.Close()

	// Generate type definitions
	var buf strings.Builder
	if err := template.Must(template.New("").Funcs(template.FuncMap{
		"toContainer":                  toContainer,
		"toArray":                      toArray,
		"toBitfield":                   toBitfield,
		"toSwitch":                     toSwitch,
		"toOption":                     toOption,
		"toMapper":                     toMapper,
		"toRegistryEntryHolder":        toRegistryEntryHolder,
		"toRegistryEntryHolderSet":     toRegistryEntryHolderSet,
		"toEntityMetadataLoop":         toEntityMetadataLoop,
		"toNative":                     toNative,
		"toIdentifier":                 toIdentifier,
		"toUpper":                      strings.ToUpper,
		"add":                          add,
		"hasFieldMethods":              hasFieldMethods,
		"containerHasParentReferences": containerHasParentReferences,
		"isSwitch":                     isTemplateSwitch,
		"getSwitchInfo":                getSwitchInfo,
		"getCompareToFieldName":        getCompareToFieldName,
		"getCompareToExpression":       getCompareToExpression,
		"isCompareToFieldMapper":       isCompareToFieldMapper,
		"isBitflagMemberAccess":        isBitflagMemberAccess,
		"getBitflagMemberName":         getBitflagMemberName,
		"getBitflagCheckCode":          getBitflagCheckCode,
		"isArrayWithContextElements":   isArrayWithContextElements,
		"getParentRefsForArrayContext": getParentRefsForArrayContext,
		"isExplicitCountArray":         isExplicitCountArray,
		"explicitCountArrayFieldName":  explicitCountArrayFieldName,
		"exprForCtxKey":                exprForCtxKey,
		"exprForCtxKeyWithPrefix":      exprForCtxKeyWithPrefix,
		"isParentCompareTo":            isParentCompareTo,
		"ctxKeyForSwitch":              ctxKeyForSwitch,
		"typeRequiresParentContext":    typeRequiresParentContext,
		"getParentRefsForType":         getParentRefsForType,
		"sanitizeIdentifier":           sanitizeIdentifier,
		"isBitflagsField":              isBitflagsField,
		"bitflags":                     getBitflagsForField,
		"wrapperName":                  bitflagsWrapperName,
		"resolveFieldType":             resolveFieldTypeForBitflags,
		"switchHasValidCases":          switchHasValidCases,
		"countNonSwitchFields":         countNonSwitchFields,
		"countSwitchFields":            countSwitchFields,
		"not":                          notFunc,
		"isNestedSwitch":               isNestedSwitch,
		"getNestedSwitchInfo":          getNestedSwitchInfo,
		"dict":                         dict,
		"isPacketType":                 isPacketType,
		"getPacketID":                  getPacketID,
		"isNBTFieldType":               isNBTFieldType,
		"formatRawDef":                 formatRawDefinition,
	}).Parse(structsTmpl+bitflagWrapperTmpl+bitflagWrapperTypeTmpl)).ExecuteTemplate(&buf, "structsTmpl", inTypes); err != nil {
		panic(err)
	}

	// Post-process and add imports
	typeDefsOutput := buf.String()
	if !areBaseTypes {
		typeDefsOutput = fixUnprefixedBaseTypes(typeDefsOutput)

		var imports []string
		// Only add imports that are actually used
		if strings.Contains(typeDefsOutput, "fmt.") {
			imports = append(imports, `"fmt"`)
		}
		if strings.Contains(typeDefsOutput, "io.") {
			imports = append(imports, `"io"`)
		}
		if strings.Contains(typeDefsOutput, "log.") {
			imports = append(imports, `"log"`)
		}
		if strings.Contains(typeDefsOutput, "bytes.") {
			imports = append(imports, `"bytes"`)
		}
		if strings.Contains(typeDefsOutput, "basetypes.") {
			imports = append(imports, `"github.com/reallyoldfogie/mc-protocol-go/data/`+version+`/basetypes"`)
		}
		if strings.Contains(typeDefsOutput, "models.") {
			imports = append(imports, `"github.com/reallyoldfogie/mc-protocol-go/models"`)
		}
		if strings.Contains(typeDefsOutput, "pk.") {
			imports = append(imports, `pk "github.com/Tnze/go-mc/net/packet"`)
		}
		if strings.Contains(typeDefsOutput, "errors.") {
			imports = append(imports, `"github.com/pkg/errors"`)
		}

		// Build custom header with only necessary imports
		header = "// Generated by gen_protocol. DO NOT EDIT\npackage " + packageName + "\n"
		if len(imports) > 0 {
			header += "\nimport (\n\t" + strings.Join(imports, "\n\t") + "\n)\n"
		}
		header += "\n"
	} else {
		// For basetypes, use the same dynamic import detection
		var imports []string
		if strings.Contains(typeDefsOutput, "fmt.") {
			imports = append(imports, `"fmt"`)
		}
		if strings.Contains(typeDefsOutput, "io.") {
			imports = append(imports, `"io"`)
		}
		if strings.Contains(typeDefsOutput, "log.") {
			imports = append(imports, `"log"`)
		}
		if strings.Contains(typeDefsOutput, "bytes.") {
			imports = append(imports, `"bytes"`)
		}
		if strings.Contains(typeDefsOutput, "models.") {
			imports = append(imports, `"github.com/reallyoldfogie/mc-protocol-go/models"`)
		}
		if strings.Contains(typeDefsOutput, "pk.") {
			imports = append(imports, `pk "github.com/Tnze/go-mc/net/packet"`)
		}
		if strings.Contains(typeDefsOutput, "errors.") {
			imports = append(imports, `"github.com/pkg/errors"`)
		}

		// Build custom header with only necessary imports
		header = "// Generated by gen_protocol (baseTypesHeader). DO NOT EDIT\npackage " + packageName + "\n"
		if len(imports) > 0 {
			header += "\nimport (\n\t" + strings.Join(imports, "\n\t") + "\n)\n"
		}
		header += "\n"
	}

	// Combine header and type definitions
	fullContent := header + typeDefsOutput

	// Format the generated code
	formatted, err := format.Source([]byte(fullContent))
	if err != nil {
		// If formatting fails, write unformatted code and log the error
		fmt.Fprintf(os.Stderr, "Warning: failed to format %s: %v\n", filepath.Join(basePath, fileName), err)
		_, writeErr := typesFile.WriteString(fullContent)
		return writeErr
	}

	// Write formatted code
	_, err = typesFile.Write(formatted)
	return err
}

func generateTypesFile(basePath, version, packageName string, inTypes []*datatypes.Type, areBaseTypes bool) error {
	packageName = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, packageName)

	basePath = strings.ToLower(basePath)
	err := os.MkdirAll(basePath, 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create type package folder (basePath:%s) (package: %s) [%v]\n", basePath, packageName, err)
		return err
	}

	typesFile, err := os.Create(filepath.Join(basePath, "types.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	defer typesFile.Close()

	// Build header
	var header string
	if areBaseTypes {
		header = strings.ReplaceAll(baseTypesHeader, "{{PACKAGE_NAME}}", packageName)
	} else {
		header = strings.ReplaceAll(typesHeader, "{{PACKAGE_NAME}}", packageName)
	}

	// Deduplicate types by both pointer and name
	seenTypes := map[*datatypes.Type]int{}
	seenTypeNames := map[string]*datatypes.Type{}
	tmpTypes := []*datatypes.Type{}

	for _, t := range inTypes {
		// Skip types with invalid names (containing dots) that can't be Go type names
		// These are types like "models.Void" that should never be generated
		if strings.Contains(t.Name, ".") {
			fmt.Printf("DEBUG: Skipping type with invalid name containing dot: '%s'\n", t.Name)
			continue
		}

		// Skip if we've already seen this exact type pointer
		if _, ok := seenTypes[t]; ok {
			continue
		}

		// Check if we've seen a type with this name before
		if existingType, ok := seenTypeNames[t.Name]; ok {
			// If types are structurally identical, skip the duplicate
			if typesAreEquivalent(existingType, t) {
				continue
			}
			// If types differ, rename the new one with a numeric suffix
			suffix := 1
			newName := fmt.Sprintf("%s_%d", t.Name, suffix)
			for _, exists := seenTypeNames[newName]; exists; _, exists = seenTypeNames[newName] {
				suffix++
				newName = fmt.Sprintf("%s_%d", t.Name, suffix)
			}
			fmt.Fprintf(os.Stderr, "Warning: duplicate type name '%s' detected with different structure. Renaming to '%s'\n", t.Name, newName)
			t.Name = newName
			if t.Extras != nil {
				t.Extras.SetName(newName)
			}
		}

		seenTypes[t] = 1
		seenTypeNames[t.Name] = t
		tmpTypes = append(tmpTypes, t)
	}

	inTypes = tmpTypes

	// typesFile.WriteString("// types:\n")
	// for _, v := range inTypes {
	// 	typesFile.WriteString("/*\n")
	// 	typesFile.WriteString(spew.Sdump(v))
	// 	typesFile.WriteString("*/\n")
	// }
	// typesFile.WriteString("\n")
	os.MkdirAll("./tmp", 0600)
	tmpFile, err := os.CreateTemp("./tmp", "structsTmpl*.tmpl")
	if err == nil {
		os.WriteFile(tmpFile.Name(), []byte(structsTmpl+bitflagWrapperTmpl+bitflagWrapperTypeTmpl), 0600)
	}
	// Write to a buffer first so we can post-process
	var buf strings.Builder
	if err := template.Must(template.New("").Funcs(template.FuncMap{
		"toContainer":                  toContainer,
		"toArray":                      toArray,
		"toBitfield":                   toBitfield,
		"toSwitch":                     toSwitch,
		"toOption":                     toOption,
		"toMapper":                     toMapper,
		"toRegistryEntryHolder":        toRegistryEntryHolder,
		"toRegistryEntryHolderSet":     toRegistryEntryHolderSet,
		"toEntityMetadataLoop":         toEntityMetadataLoop,
		"toNative":                     toNative,
		"toIdentifier":                 toIdentifier,
		"toUpper":                      strings.ToUpper,
		"add":                          add,
		"hasFieldMethods":              hasFieldMethods,
		"containerHasParentReferences": containerHasParentReferences,
		"isSwitch":                     isTemplateSwitch,
		"getSwitchInfo":                getSwitchInfo,
		"getCompareToFieldName":        getCompareToFieldName,
		"getCompareToExpression":       getCompareToExpression,
		"isCompareToFieldMapper":       isCompareToFieldMapper,
		"isBitflagMemberAccess":        isBitflagMemberAccess,
		"getBitflagMemberName":         getBitflagMemberName,
		"getBitflagCheckCode":          getBitflagCheckCode,
		"isArrayWithContextElements":   isArrayWithContextElements,
		"getParentRefsForArrayContext": getParentRefsForArrayContext,
		"isExplicitCountArray":         isExplicitCountArray,
		"explicitCountArrayFieldName":  explicitCountArrayFieldName,
		"exprForCtxKey":                exprForCtxKey,
		"exprForCtxKeyWithPrefix":      exprForCtxKeyWithPrefix,
		"isParentCompareTo":            isParentCompareTo,
		"ctxKeyForSwitch":              ctxKeyForSwitch,
		"typeRequiresParentContext":    typeRequiresParentContext,
		"getParentRefsForType":         getParentRefsForType,
		"sanitizeIdentifier":           sanitizeIdentifier,
		"isBitflagsField":              isBitflagsField,
		"bitflags":                     getBitflagsForField,
		"wrapperName":                  bitflagsWrapperName,
		"resolveFieldType":             resolveFieldTypeForBitflags,
		"switchHasValidCases":          switchHasValidCases,
		"countNonSwitchFields":         countNonSwitchFields,
		"countSwitchFields":            countSwitchFields,
		"not":                          notFunc,
		"isNestedSwitch":               isNestedSwitch,
		"getNestedSwitchInfo":          getNestedSwitchInfo,
		"dict":                         dict,
		"isPacketType":                 isPacketType,
		"getPacketID":                  getPacketID,
		"isNBTFieldType":               isNBTFieldType,
		"formatRawDef":                 formatRawDefinition,
	}).Parse(structsTmpl+bitflagWrapperTmpl+bitflagWrapperTypeTmpl)).ExecuteTemplate(&buf, "structsTmpl", inTypes); err != nil {
		panic(err)
	}

	// Post-process: Fix unprefixed basetype references when not generating basetypes
	typeDefsOutput := buf.String()
	if !areBaseTypes {
		// Fix unprefixed Array, Bitfield, and Bitflags references
		typeDefsOutput = fixUnprefixedBaseTypes(typeDefsOutput)

		// Add imports based on usage in the type definitions
		var imports []string
		if strings.Contains(typeDefsOutput, "bytes.") {
			imports = append(imports, `"bytes"`)
		}
		if strings.Contains(typeDefsOutput, "basetypes.") {
			imports = append(imports, `"github.com/reallyoldfogie/mc-protocol-go/data/`+version+`/basetypes"`)
		}
		if strings.Contains(typeDefsOutput, "models.") {
			imports = append(imports, `"github.com/reallyoldfogie/mc-protocol-go/models"`)
		}

		// Build import block
		importBlock := ""
		if len(imports) > 0 {
			importBlock = strings.Join(imports, "\n\t\t") + "\n\t\t"
		}
		header = strings.ReplaceAll(header, "// IMPORTS_PLACEHOLDER", importBlock)

		// Remove pk import if not used
		if !strings.Contains(typeDefsOutput, "pk.") {
			header = strings.ReplaceAll(header, `pk "github.com/Tnze/go-mc/net/packet"`, `// pk "github.com/Tnze/go-mc/net/packet" // unused`)
		}
	} else {
		// For basetypes package, add imports based on usage
		// Note: baseTypesHeader already includes "fmt", "io", "log", "models", and "pk"
		var imports []string
		if strings.Contains(typeDefsOutput, "bytes.") {
			imports = append(imports, `"bytes"`)
		}
		// Don't add models - it's already in the base header

		// Build import block
		importBlock := ""
		if len(imports) > 0 {
			importBlock = strings.Join(imports, "\n\t\t") + "\n\t\t"
		}
		header = strings.ReplaceAll(header, "// IMPORTS_PLACEHOLDER", importBlock)

		// Remove pk import if not used
		if !strings.Contains(typeDefsOutput, "pk.") {
			header = strings.ReplaceAll(header, `pk "github.com/Tnze/go-mc/net/packet"`, `// pk "github.com/Tnze/go-mc/net/packet" // unused`)
		}
	}

	// Combine header and type definitions
	fullContent := header + typeDefsOutput

	// Format the generated code
	formatted, err := format.Source([]byte(fullContent))
	if err != nil {
		// If formatting fails, write unformatted code and log the error
		fmt.Fprintf(os.Stderr, "Warning: failed to format %s: %v\n", filepath.Join(basePath, "types.go"), err)
		_, writeErr := typesFile.WriteString(fullContent)
		return writeErr
	}

	// Write formatted code
	_, err = typesFile.Write(formatted)
	return err
}

const (
	structsTmpl = `
	// START structsTmpl 

{{define "structsTmpl"}}
{{- range .}}
{{if eq .TypeName "container"}}
  {{/* Emit bitflag wrappers for any bitflag fields in this container */}}
  {{- $c := toContainer .}}
  {{- range $c.Fields}}
    {{- if isBitflagsField .}}
      {{- template "bitflagWrapperTmpl" dict "container" $c "field" .}}
    {{- end}}
  {{- end}}
  {{- template "structTmpl" dict "container" $c "type" .}}

{{else if eq .TypeName "array"}}
  {{template "arrayTmpl" .}}

{{else if eq .TypeName "bitfield"}}
  // {{.Comment}}
  {{template "bitfieldTmpl" .}}

{{else if eq .TypeName "bitflags"}}
  {{template "bitflagWrapperTypeTmpl" .}}

{{else if eq .TypeName "switch"}}
  {{template "switchTmpl" .}}

{{else if eq .TypeName "mapper"}}
  {{template "mapperTmpl" .}}

{{else if eq .TypeName "registryEntryHolder"}}
  {{template "registryEntryHolderTmpl" .}}

{{else if eq .TypeName "registryEntryHolderSet"}}
  {{template "registryEntryHolderSetTmpl" .}}

{{else if eq .TypeName "entityMetadataLoop"}}
  {{template "entityMetadataLoopTmpl" .}}

{{else if eq .TypeName print "pk." .Name}}
type {{.Name}} {{.TypeName}}

{{else}}
type {{.Name}} = {{.TypeName}}
{{end}}
{{- end}}
{{end}}

{{define "arrayTmpl"}} type {{.Name}} {{.TypeName}} {{end}}
 

{{define "structTmpl"}}
{{- $container := .container}}
{{- $type := .type}}
{{- $isPacket := isPacketType $type}}
{{- $packetID := getPacketID $type}}
{{- if $type.RawDefinition}}
// Protodef: {{formatRawDef $type.RawDefinition}}
{{- end}}
type {{$container.Name}} struct {
{{- if $isPacket}}
	packetID int32
{{- end}}
{{- range $container.Fields}}
	{{- if .Type.RawDefinition}}
	// {{formatRawDef .Type.RawDefinition}}
	{{- end}}
	{{.Name}} {{resolveFieldType $container .}}
{{- end}}
}


{{- if $isPacket}}
// New{{$container.Name}} creates a new {{$container.Name}} packet with the correct packet ID.
func New{{$container.Name}}() *{{$container.Name}} {
	return &{{$container.Name}}{packetID: {{$packetID}}}
}

// PacketID returns the protocol ID for this packet type.
func (p *{{$container.Name}}) PacketID() int32 {
	return p.packetID
}

// SetPacketID sets the protocol ID for this packet type.
// This is used when the same packet structure is reused across multiple stages with different IDs.
func (p *{{$container.Name}}) SetPacketID(id int32) {
	p.packetID = id
}

// Marshal serializes the packet into wire format.
func (p *{{$container.Name}}) Marshal() pk.Packet {
	return pk.Marshal(
		p.packetID
		{{- range $container.Fields}},
		{{- if eq .Type.TypeName "pk.Field"}}
		p.{{.Name}}
		{{- else}}
		&p.{{.Name}}
		{{- end}}
		{{- end}})
}

// Scan deserializes a wire-format packet into this struct.
func (p *{{$container.Name}}) Scan(packet pk.Packet) error {
	if packet.ID != p.packetID {
		return fmt.Errorf("packet ID mismatch: expected %d, got %d", p.packetID, packet.ID)
	}
	{{- range $container.Fields}}
	{{- if eq .Type.TypeName "pk.NBTField"}}
	// Initialize NBTField.V for {{.Name}} to prevent nbt decode errors
	if p.{{.Name}}.V == nil {
		var tmp pk.String
		p.{{.Name}}.V = &tmp
	}
	{{- end}}
	{{- if eq .Type.TypeName "models.NBTField"}}
	// Initialize models.NBTField.Value for {{.Name}} to prevent nbt decode errors
	if p.{{.Name}}.Value == nil {
		p.{{.Name}}.Value = &models.NBTCompound{Tags: []models.NBTTag{}}
	}
	{{- end}}
	{{- end}}
	{{- $hasArraysWithContext := false}}
	{{- range $container.Fields}}
	{{- if isArrayWithContextElements .}}
	{{- $hasArraysWithContext = true}}
	{{- end}}
	{{- end}}
	{{- $hasSwitchFields := false}}
	{{- range $container.Fields}}
	{{- if isSwitch .}}
	{{- $hasSwitchFields = true}}
	{{- end}}
	{{- end}}
	{{- if or $hasArraysWithContext $hasSwitchFields}}
	// This packet has {{if $hasArraysWithContext}}arrays that need parent context{{end}}{{if and $hasArraysWithContext $hasSwitchFields}} and {{end}}{{if $hasSwitchFields}}switch fields{{end}}, so we read fields individually
	r := bytes.NewReader(packet.Data)
	var totalBytes int64
	var bytesRead int64
	var err error
	{{- range $container.Fields}}
	{{- if isArrayWithContextElements .}}
	// Prepare parent context for array '{{.Name}}'
	ctx := models.NewParentContext()
	{{- range getParentRefsForArrayContext .}}
	ctx.SetField("{{.}}", {{exprForCtxKeyWithPrefix $container . "p"}})
	{{- end}}
	p.{{.Name}}.SetParentContext(ctx)
	{{- end}}
	{{- if isSwitch .}}
	{{- template "switchScanTmpl" dict "field" . "container" $container "prefix" "p."}}
	{{- else}}
	bytesRead, err = p.{{.Name}}.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return fmt.Errorf("scanning packet field[{{.Name}}] error: %w", err)
	}
	{{- end}}
	{{- end}}
	_ = totalBytes // Unused in Scan()
	return nil
	{{- else}}
	return packet.Scan(
		{{- range $container.Fields}}
		{{- if eq .Type.TypeName "pk.Field"}}
		p.{{.Name}},
		{{- else}}
		&p.{{.Name}},
		{{- end}}
		{{- end}})
	{{- end}}
}

// GetFields returns a map of all packet fields for version-agnostic access.
// Use this when you need to access fields dynamically or when working with version-specific types
// that don't have stable cross-version interfaces.
//
// For version-specific code with type safety, use the typed getter methods (e.g., GetCount()).
// For semi-agnostic code with fields that have stable types, use the typed interfaces (e.g., CountGetter).
func (p *{{$container.Name}}) GetFields() map[string]pk.FieldEncoder {
	fields := map[string]pk.FieldEncoder{}
	{{- range $container.Fields}}
	fields["{{.Name}}"] = {{if eq .Type.TypeName "pk.Field"}}p.{{.Name}}{{else}}&p.{{.Name}}{{end}}
	{{- end}}
	return fields
}
// SetFields updates packet fields from a map for version-agnostic access.
// Use this when you need to set fields dynamically or when working with version-specific types
// that don't have stable cross-version interfaces.
//
// For version-specific code with type safety, use the typed setter methods (e.g., SetCount()).
// For semi-agnostic code with fields that have stable types, use the typed interfaces (e.g., CountSetter).
func (p *{{$container.Name}}) SetFields(fields map[string]pk.FieldEncoder) {
	{{- range $container.Fields}}
	if val, ok := fields["{{.Name}}"]; ok {
		{{if eq .Type.TypeName "pk.Field"}}p.{{.Name}} = val.(pk.Field){{else}}p.{{.Name}} = *val.(*{{resolveFieldType $container .}}){{end}}
	}
	{{- end}}
}
// Typed field accessor methods for version-specific type-safe access
{{- range $container.Fields}}
// Get{{.Name}} returns the {{.Name}} field value.
// Note: This method returns the actual field type, which may be version-specific.
// For version-agnostic access, use GetFields() or check for typed interfaces.
func (p *{{$container.Name}}) Get{{.Name}}() {{resolveFieldType $container .}} {
	return p.{{.Name}}
}

// Set{{.Name}} sets the {{.Name}} field value.
// Note: This method accepts the actual field type, which may be version-specific.
// For version-agnostic access, use SetFields() or check for typed interfaces.
func (p *{{$container.Name}}) Set{{.Name}}(val {{resolveFieldType $container .}}) {
	p.{{.Name}} = val
}
{{end}}
{{- end}}
		

{{- if not (containerHasParentReferences $container)}}
func (t *{{$container.Name}}) ReadFrom(r io.Reader) (totalBytes int64, err error) {
	{{- if $container.Fields}}
	{{- if hasFieldMethods $container}}
	var bytesRead int64

	{{- range $field := $container.Fields}}
	{{- if isSwitch $field}}
	{{- template "switchReadTmpl" dict "field" $field "container" $container "prefix" "t."}}
	{{- else if ne $field.Type.TypeName "[]byte"}}
	{{- if isExplicitCountArray $field}}
	// Initialize ExplicitCountArray with count field name
	t.{{$field.Name}}.CountFieldName = "{{explicitCountArrayFieldName $container $field.Name}}"
	// Prepare parent context for explicit count array '{{$field.Name}}'
	{{$field.Name}}_ctx := models.NewParentContext()
	{{$field.Name}}_ctx.SetField("{{explicitCountArrayFieldName $container $field.Name}}", t.{{explicitCountArrayFieldName $container $field.Name}})
	t.{{$field.Name}}.SetParentContext({{$field.Name}}_ctx)
	{{- else if isArrayWithContextElements $field}}
	// Prepare parent context for array '{{$field.Name}}'
	{{$field.Name}}_ctx := models.NewParentContext()
	{{- range getParentRefsForArrayContext $field}}
	{{$field.Name}}_ctx.SetField("{{.}}", {{exprForCtxKey $container .}})
	{{- end}}
	t.{{$field.Name}}.SetParentContext({{$field.Name}}_ctx)
	{{- end}}
	bytesRead, err = t.{{$field.Name}}.ReadFrom(r)

	totalBytes += bytesRead
	if err != nil {
		return totalBytes, errors.Wrap(err, "failed to read field {{$field.Name}}")
	}
	{{- end}}
	{{- end}}

	return totalBytes, nil
	{{- else}}
	_ = r // TODO: Implement Field methods for all field types
	return 0, nil
	{{- end}}
	{{- else}}
	return 0, nil
	{{- end}}
}


func (t {{$container.Name}}) WriteTo(w io.Writer) (totalBytes int64, err error) {
	{{- if $container.Fields}}
	{{- if hasFieldMethods $container}}
	var bytesWritten int64

	defer func() {
		log.Printf("[{{$container.Name}}.WriteTo] totalBytes: %d err: %#v", totalBytes, err)
	}()

	{{- range $container.Fields}}
	{{- if isSwitch .}}
	{{- $sw := getSwitchInfo .}}
	{{- $fieldName := .Name}}
	// Switch field {{$fieldName}} based on {{if $sw.CompareTo}}{{$sw.CompareTo}}{{else}}static value{{end}}
	if t.{{$fieldName}} != nil {
		// Write switch field value if it implements WriteTo
		if writer, ok := t.{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
			bytesWritten, err = writer.WriteTo(w)
			totalBytes += bytesWritten
			if err != nil {
				return totalBytes, err
			}
		} else {
			// Not a void case and doesn't implement WriteTo
			return totalBytes, fmt.Errorf("switch field {{$fieldName}} value does not implement WriteTo: %T", t.{{$fieldName}})
		}
	}
	{{- else if ne .Type.TypeName "[]byte"}}
	bytesWritten, err = t.{{.Name}}.WriteTo(w)
	totalBytes += bytesWritten
	if err != nil {
		return totalBytes, err
	}
	{{- end}}
	{{- end}}
	return totalBytes, nil
	{{- else}}
	_ = w // TODO: Implement Field methods for all field types
	return 0, nil
	{{- end}}
	{{- else}}
	return 0, nil
	{{- end}}
}

{{- else}}
// {{$container.Name}} requires parent context for proper (de)serialization of switch fields.
func (t *{{$container.Name}}) ReadFrom(r io.Reader) (int64, error) {
	return 0, fmt.Errorf("{{$container.Name}} requires parent context, use ReadFromWithParentContext")
}

func (t {{$container.Name}}) WriteTo(w io.Writer) (int64, error) {
	return 0, fmt.Errorf("{{$container.Name}} requires parent context, use WriteToWithParentContext")
}

func (t *{{$container.Name}}) ReadFromWithParentContext(r io.Reader, ctx models.ParentContext) (totalBytes int64, err error) {
	{{- if $container.Fields}}
	{{- if hasFieldMethods $container}}
	var bytesRead int64
	{{- range $container.Fields}}
	{{- if isSwitch .}}
	{{- template "switchReadWithParentCtxTmpl" dict "field" . "container" $container "prefix" "t."}}
	{{- else if ne .Type.TypeName "[]byte"}}
	bytesRead, err = t.{{.Name}}.ReadFrom(r)
	totalBytes += bytesRead
	if err != nil {
		return totalBytes, errors.Wrap(err, "failed to read field {{.Name}} with parent context")
	}
	{{- end}}
	{{- end}}
	return totalBytes, nil
	{{- else}}
	_ = r
	return 0, nil
	{{- end}}
	{{- else}}
	return 0, nil
	{{- end}}
}

func (t {{$container.Name}}) WriteToWithParentContext(w io.Writer, ctx models.ParentContext) (totalBytes int64, err error) {
	// Write switch fields using their underlying encoders when set; other fields normally.
	{{- if $container.Fields}}
	{{- if hasFieldMethods $container}}
	var bytesWritten int64
	{{- range $container.Fields}}
		{{- if isSwitch .}}
			{{- template "switchWriteWithParentCtxTmpl" dict "field" . "container" $container "prefix" "t."}}
		{{- else if ne .Type.TypeName "[]byte"}}
		bytesWritten, err = t.{{.Name}}.WriteTo(w)
		totalBytes += bytesWritten
		if err != nil {
			return totalBytes, err
		}
		{{- end}}
	{{- end}}
return totalBytes, nil
	{{- else}}
	_ = w
	return 0, nil
	{{- end}}
	{{- else}}
	return 0, nil
	{{- end}}
}
{{- end}}
{{end}}

{{define "switchReadTmpl"}}
{{- $sw := getSwitchInfo .field}}
{{- $fieldName := .field.Name}}
{{- $container := .container}}
{{- $prefix := .prefix}}
	// Switch field {{$fieldName}} based on {{if $sw.CompareTo}}{{$sw.CompareTo}}{{else}}static value{{end}}
	{{- $length := len $sw.Fields}}
	{{- if ne $length 0}}
	{{- if $sw.CompareTo}}
	{{- if isBitflagMemberAccess $sw}}
	// Check bitflag member
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{getBitflagCheckCode $sw $prefix}})
	{{- else}}
	// Convert compareTo value to string for matching
	compareValue{{$fieldName}} := {{getCompareToExpression $sw $container $prefix}}
	{{- end}}
	{{- else}}
	// Use static compareToValue for matching
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{printf "%#v" $sw.CompareToValue}})
	{{- end}}

	switch compareValue{{$fieldName}} {
	{{- range $key, $type := $sw.Fields}}
	{{- if $type}}
	{{- if isNestedSwitch $type}}
	{{- $nestedSw := getNestedSwitchInfo $type}}
	case "{{$key}}":
		// Nested switch based on {{if $nestedSw.CompareTo}}{{$nestedSw.CompareTo}}{{else}}static value{{end}}
		{{- if $nestedSw.CompareTo}}
		{{- if isBitflagMemberAccess $nestedSw}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{getBitflagCheckCode $nestedSw $prefix}})
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := {{getCompareToExpression $nestedSw $container $prefix}}
		{{- end}}
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{printf "%#v" $nestedSw.CompareToValue}})
		{{- end}}
		switch compareValueNested{{$fieldName}}{{$key}} {
		{{- range $nestedKey, $nestedType := $nestedSw.Fields}}
		{{- if $nestedType}}
		{{- if ne $nestedType.TypeName "[]byte"}}
		case "{{$nestedKey}}":
			var val {{$nestedType.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to read nested switch field {{$fieldName}} case {{$key}} -> {{$nestedKey}}")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- end}}
		{{- end}}
		{{- end}}
		{{- if $nestedSw.Default}}
		{{- if ne $nestedSw.Default.TypeName "[]byte"}}
		default:
			var val {{$nestedSw.Default.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to read nested switch field {{$fieldName}} default case for parent case {{$key}}")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- else}}
		default:
			// Void case - no data to read
			{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
		{{- else}}
		default:
			return totalBytes, fmt.Errorf("nested switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValueNested{{$fieldName}}{{$key}})
		{{- end}}
		}
	{{- else if and (ne $type.TypeName "[]byte") (ne $type.TypeName "models.Void") (ne $type.TypeName "struct{}")}}
	case "{{$key}}":
		var val {{$type.TypeName}}
		{{- if typeRequiresParentContext $type}}
		// Type requires parent context - build context from current container fields
		ctx_{{$fieldName}}_{{sanitizeIdentifier $key}} := models.NewParentContext()
		{{- range getParentRefsForType $type}}
		ctx_{{$fieldName}}_{{sanitizeIdentifier $key}}.SetField("{{.}}", {{exprForCtxKey $container .}})
		{{- end}}
		bytesRead, err = val.ReadFromWithParentContext(r, ctx_{{$fieldName}}_{{sanitizeIdentifier $key}})
		{{- else}}
		bytesRead, err = val.ReadFrom(r)
		{{- end}}
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} case {{$key}}")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else if or (eq $type.TypeName "[]byte") (eq $type.TypeName "models.Void") (eq $type.TypeName "struct{}")}}
	case "{{$key}}":
		// Void case - no data to read
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read void switch field {{$fieldName}} case {{$key}}")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
	{{- end}}
	{{- end}}
	{{- if $sw.Default}}
	{{- if and (ne $sw.Default.TypeName "[]byte") (ne $sw.Default.TypeName "models.Void") (ne $sw.Default.TypeName "struct{}")}}
	default:
		var val {{$sw.Default.TypeName}}
		bytesRead, err = val.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} default case")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else}}
	default:
		// Void case - no data to read
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read void switch field {{$fieldName}} default case")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
    {{- else}}
    default:
        {{- if isCompareToFieldMapper $sw $container }}
        // Mapper-backed discriminator with no explicit data for this value: treat as void
        var __void models.Void
        {{$prefix}}{{$fieldName}} = &__void
        {{- else if eq .field.Type.TypeName "pk.Field" }}
        // No explicit default; treat as void (no data)
        // Per minecraft.wiki protocol docs: "If properties for parser are not specified, then this parser has no properties"
        // Using Buffer.ReadFrom() here would call io.ReadAll() and consume ALL remaining data, breaking array parsing
        var __void models.Void
        bytesRead, err = __void.ReadFrom(r)
        totalBytes += bytesRead
        if err != nil {
            return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} default void case")
        }
        {{$prefix}}{{$fieldName}} = &__void
        {{- else }}
        return totalBytes, fmt.Errorf("switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValue{{$fieldName}})
        {{- end }}
    {{- end}}
	}
	{{- else}}
	_ = t.{{$fieldName}} // No switch cases to handle
	{{- end}}
{{end}}

{{define "switchScanTmpl"}}
{{- $sw := getSwitchInfo .field}}
{{- $fieldName := .field.Name}}
{{- $container := .container}}
{{- $prefix := .prefix}}
	// Switch field {{$fieldName}} based on {{if $sw.CompareTo}}{{$sw.CompareTo}}{{else}}static value{{end}}
	{{- $length := len $sw.Fields}}
	{{- if ne $length 0}}
	{{- if $sw.CompareTo}}
	{{- if isBitflagMemberAccess $sw}}
	// Check bitflag member
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{getBitflagCheckCode $sw $prefix}})
	{{- else}}
	// Convert compareTo value to string for matching
	compareValue{{$fieldName}} := {{getCompareToExpression $sw $container $prefix}}
	{{- end}}
	{{- else}}
	// Use static compareToValue for matching
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{printf "%#v" $sw.CompareToValue}})
	{{- end}}

	switch compareValue{{$fieldName}} {
	{{- range $key, $type := $sw.Fields}}
	{{- if $type}}
	{{- if isNestedSwitch $type}}
	{{- $nestedSw := getNestedSwitchInfo $type}}
	case "{{$key}}":
		// Nested switch based on {{if $nestedSw.CompareTo}}{{$nestedSw.CompareTo}}{{else}}static value{{end}}
		{{- if $nestedSw.CompareTo}}
		{{- if isBitflagMemberAccess $nestedSw}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{getBitflagCheckCode $nestedSw $prefix}})
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := {{getCompareToExpression $nestedSw $container $prefix}}
		{{- end}}
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{printf "%#v" $nestedSw.CompareToValue}})
		{{- end}}
		switch compareValueNested{{$fieldName}}{{$key}} {
		{{- range $nestedKey, $nestedType := $nestedSw.Fields}}
		{{- if $nestedType}}
		{{- if ne $nestedType.TypeName "[]byte"}}
		case "{{$nestedKey}}":
			var val {{$nestedType.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return errors.Wrap(err,"scanning packet field[{{$fieldName}}] case {{$key}} -> {{$nestedKey}}")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- end}}
		{{- end}}
		{{- end}}
		{{- if $nestedSw.Default}}
		{{- if ne $nestedSw.Default.TypeName "[]byte"}}
		default:
			var val {{$nestedSw.Default.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return errors.Wrap(err,"scanning packet field[{{$fieldName}}] default case for parent case {{$key}}")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- else}}
		default:
			// Void case - no data to read
			{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
		{{- else}}
		default:
			return errors.New("nested switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValueNested{{$fieldName}}{{$key}})
		{{- end}}
		}
	{{- else if and (ne $type.TypeName "[]byte") (ne $type.TypeName "models.Void") (ne $type.TypeName "struct{}")}}
	case "{{$key}}":
		var val {{$type.TypeName}}
		bytesRead, err = val.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return errors.Wrap(err,"scanning packet field[{{$fieldName}}] case {{$key}}")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else if or (eq $type.TypeName "[]byte") (eq $type.TypeName "models.Void") (eq $type.TypeName "struct{}")}}
	case "{{$key}}":
		// Void case - no data to read
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return errors.Wrap(err, "failed to read void switch field {{$fieldName}} case {{$key}}")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
	{{- end}}
	{{- end}}
	{{- if $sw.Default}}
	{{- if and (ne $sw.Default.TypeName "[]byte") (ne $sw.Default.TypeName "models.Void") (ne $sw.Default.TypeName "struct{}")}}
	default:
		var val {{$sw.Default.TypeName}}
		bytesRead, err = val.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return errors.Wrap(err,"scanning packet field[{{$fieldName}}] default case")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else}}
	default:
		// Void case - no data to read
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return errors.Wrap(err, "failed to read void switch field {{$fieldName}} default case")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
    {{- else}}
    default:
        {{- if isCompareToFieldMapper $sw $container }}
        // Mapper-backed discriminator with no explicit data for this value: treat as void
        var __void models.Void
        {{$prefix}}{{$fieldName}} = &__void
        {{- else if eq .field.Type.TypeName "pk.Field" }}
        // No explicit default; treat as void (no data)
        // Per minecraft.wiki protocol docs: "If properties for parser are not specified, then this parser has no properties"
        // Using Buffer.ReadFrom() here would call io.ReadAll() and consume ALL remaining data, breaking array parsing
        var __void models.Void
        bytesRead, err = __void.ReadFrom(r)
        totalBytes += bytesRead
        if err != nil {
            return errors.Wrap(err,"failed to read switch field {{$fieldName}} default void case")
        }
        {{$prefix}}{{$fieldName}} = &__void
        {{- else }}
        return totalBytes, errors.New("switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValue{{$fieldName}})
        {{- end }}
    {{- end}}
	}
	{{- else}}
	_ = p.{{$fieldName}} // No switch cases to handle
	{{- end}}
{{end}}

{{define "switchReadWithParentCtxTmpl"}}
{{- $sw := getSwitchInfo .field}}
{{- $fieldName := .field.Name}}
{{- $container := .container}}
{{- $prefix := .prefix}}
	// Switch field {{$fieldName}} using parent context based on {{if $sw.CompareTo}}{{$sw.CompareTo}}{{else}}static value{{end}}
	{{- $length := len $sw.Fields}}
	{{- if ne $length 0}}
	{{- if $sw.CompareTo}}
	{{- if isParentCompareTo $sw}}
	compareValue{{$fieldName}} := fmt.Sprintf("%v", ctx.GetField("{{ctxKeyForSwitch $sw}}"))
	{{- else if isBitflagMemberAccess $sw}}
	// Local bitflag member access (not parent) - use field
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{getBitflagCheckCode $sw $prefix}})
	{{- else}}
	compareValue{{$fieldName}} := {{getCompareToExpression $sw $container $prefix}}
	{{- end}}
	{{- else}}
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{printf "%#v" $sw.CompareToValue}})
	{{- end}}

	switch compareValue{{$fieldName}} {
	{{- range $key, $type := $sw.Fields}}
	{{- if $type}}
	{{- if isNestedSwitch $type}}
	{{- $nestedSw := getNestedSwitchInfo $type}}
	case "{{$key}}":
		{{- if $nestedSw.CompareTo}}
		{{- if isParentCompareTo $nestedSw}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", ctx.GetField("{{ctxKeyForSwitch $nestedSw}}"))
		{{- else if isBitflagMemberAccess $nestedSw}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{getBitflagCheckCode $nestedSw $prefix}})
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := {{getCompareToExpression $nestedSw $container $prefix}}
		{{- end}}
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{printf "%#v" $nestedSw.CompareToValue}})
		{{- end}}
		switch compareValueNested{{$fieldName}}{{$key}} {
		{{- range $nestedKey, $nestedType := $nestedSw.Fields}}
		{{- if $nestedType}}
		{{- if ne $nestedType.TypeName "[]byte"}}
		case "{{$nestedKey}}":
			var val {{$nestedType.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to read nested switch field {{$fieldName}} case {{$key}} -> {{$nestedKey}} with parent context")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- end}}
		{{- end}}
		{{- end}}
		{{- if $nestedSw.Default}}
		{{- if ne $nestedSw.Default.TypeName "[]byte"}}
		default:
			var val {{$nestedSw.Default.TypeName}}
			bytesRead, err = val.ReadFrom(r)
			totalBytes += bytesRead
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to read nested switch field {{$fieldName}} default case for parent case {{$key}} with parent context")
			}
			{{$prefix}}{{$fieldName}} = &val
		{{- else}}
		default:
			{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
		{{- else}}
		default:
			return totalBytes, errors.New("nested switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValueNested{{$fieldName}}{{$key}})
		{{- end}}
		}
	{{- else if and (ne $type.TypeName "[]byte") (ne $type.TypeName "models.Void") (ne $type.TypeName "struct{}")}}
	case "{{$key}}":
		var val {{$type.TypeName}}
		{{- if typeRequiresParentContext $type}}
		bytesRead, err = val.ReadFromWithParentContext(r, ctx)
		{{- else}}
		bytesRead, err = val.ReadFrom(r)
		{{- end}}
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} case {{$key}} with parent context")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else if or (eq $type.TypeName "[]byte") (eq $type.TypeName "models.Void") (eq $type.TypeName "struct{}")}}
	case "{{$key}}":
		// Void case - no data to read
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read void switch field {{$fieldName}} case {{$key}}")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
	{{- end}}
	{{- end}}
	{{- if $sw.Default}}
	{{- if and (ne $sw.Default.TypeName "[]byte") (ne $sw.Default.TypeName "models.Void") (ne $sw.Default.TypeName "struct{}")}}
	default:
		var val {{$sw.Default.TypeName}}
		bytesRead, err = val.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} default case with parent context")
		}
		{{$prefix}}{{$fieldName}} = &val
	{{- else}}
	default:
		{{- if eq (resolveFieldType $container $.field) "pk.Field"}}
		var __void models.Void
		bytesRead, err = __void.ReadFrom(r)
		totalBytes += bytesRead
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read void switch field {{$fieldName}} default case")
		}
		{{$prefix}}{{$fieldName}} = &__void
		{{- else}}
		{{$prefix}}{{$fieldName}} = struct{}{}
		{{- end}}
	{{- end}}
    {{- else}}
    default:
        {{- if isCompareToFieldMapper $sw $container }}
        // Mapper-backed discriminator with no explicit data for this value: treat as void
        var __void models.Void
        {{$prefix}}{{$fieldName}} = &__void
        {{- else if eq .field.Type.TypeName "pk.Field" }}
        // No explicit default; treat as void (no data)
        // Per minecraft.wiki protocol docs: "If properties for parser are not specified, then this parser has no properties"
        // Using Buffer.ReadFrom() here would call io.ReadAll() and consume ALL remaining data, breaking array parsing
        var __void models.Void
        bytesRead, err = __void.ReadFrom(r)
        totalBytes += bytesRead
        if err != nil {
            return totalBytes, errors.Wrap(err, "failed to read switch field {{$fieldName}} default void case")
        }
        {{$prefix}}{{$fieldName}} = &__void
        {{- else }}
        return totalBytes, fmt.Errorf("switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValue{{$fieldName}})
        {{- end }}
    {{- end}}
	}
	{{- else}}
	_ = t.{{$fieldName}}
	{{- end}}
{{end}}
{{define "switchWriteWithParentCtxTmpl"}}
{{- $sw := getSwitchInfo .field}}
{{- $fieldName := .field.Name}}
{{- $container := .container}}
{{- $prefix := .prefix}}
	// Switch field {{$fieldName}} using parent context based on {{if $sw.CompareTo}}{{$sw.CompareTo}}{{else}}static value{{end}}
	{{- $length := len $sw.Fields}}
	{{- if ne $length 0}}
	{{- if $sw.CompareTo}}
	{{- if isParentCompareTo $sw}}
	compareValue{{$fieldName}} := fmt.Sprintf("%v", ctx.GetField("{{ctxKeyForSwitch $sw}}"))
	{{- else if isBitflagMemberAccess $sw}}
	// Local bitflag member access (not parent) - use field
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{getBitflagCheckCode $sw $prefix}})
	{{- else}}
	compareValue{{$fieldName}} := {{getCompareToExpression $sw $container $prefix}}
	{{- end}}
	{{- else}}
	compareValue{{$fieldName}} := fmt.Sprintf("%v", {{printf "%#v" $sw.CompareToValue}})
	{{- end}}

switch compareValue{{$fieldName}} {
	{{- range $key, $type := $sw.Fields}}
	{{- if $type}}
	{{- if isNestedSwitch $type}}
	{{- $nestedSw := getNestedSwitchInfo $type}}
case "{{$key}}":
		// Nested switch based on {{if $nestedSw.CompareTo}}{{$nestedSw.CompareTo}}{{else}}static value{{end}}
		{{- if $nestedSw.CompareTo}}
		{{- if isBitflagMemberAccess $nestedSw}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{getBitflagCheckCode $nestedSw $prefix}})
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := {{getCompareToExpression $nestedSw $container $prefix}}
		{{- end}}
		{{- else}}
		compareValueNested{{$fieldName}}{{$key}} := fmt.Sprintf("%v", {{printf "%#v" $nestedSw.CompareToValue}})
		{{- end}}
		switch compareValueNested{{$fieldName}}{{$key}} {
		{{- range $nestedKey, $nestedType := $nestedSw.Fields}}
		{{- if $nestedType}}
		{{- if ne $nestedType.TypeName "[]byte"}}
		case "{{$nestedKey}}":
			if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
				bytesWritten, err = writer.WriteTo(w)
				totalBytes += bytesWritten
				if err != nil {
					return totalBytes, errors.Wrap(err, "failed to write nested switch field {{$fieldName}} case {{$key}} -> {{$nestedKey}}")
				}
			}
		{{- end}}
		{{- end}}
		{{- end}}
		{{- if $nestedSw.Default}}
		{{- if ne $nestedSw.Default.TypeName "[]byte"}}
		default:
			if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
				bytesWritten, err = writer.WriteTo(w)
				totalBytes += bytesWritten
				if err != nil {
					return totalBytes, errors.Wrap(err, "failed to write nested switch field {{$fieldName}} default case for parent case {{$key}}")
				}
			}
		{{- else}}
		default:
			// Void case - no data to write
		{{- end}}
		{{- else}}
		default:
			return totalBytes, fmt.Errorf("nested switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValueNested{{$fieldName}}{{$key}})
		{{- end}}
		}
	{{- else if and (ne $type.TypeName "[]byte") (ne $type.TypeName "models.Void") (ne $type.TypeName "struct{}")}}
case "{{$key}}":
		if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
			bytesWritten, err = writer.WriteTo(w)
			totalBytes += bytesWritten
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to write switch field {{$fieldName}} case {{$key}}")
			}
		}
	{{- else if or (eq $type.TypeName "[]byte") (eq $type.TypeName "models.Void") (eq $type.TypeName "struct{}")}}
case "{{$key}}":
		// Void case - no data to write
		if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
			bytesWritten, err = writer.WriteTo(w)
			totalBytes += bytesWritten
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to write void switch field {{$fieldName}} case {{$key}}")
			}
		}
	{{- end}}
	{{- end}}
	{{- end}}
	{{- if $sw.Default}}
	{{- if and (ne $sw.Default.TypeName "[]byte") (ne $sw.Default.TypeName "models.Void") (ne $sw.Default.TypeName "struct{}")}}
default:
		if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
			bytesWritten, err = writer.WriteTo(w)
			totalBytes += bytesWritten
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to write switch field {{$fieldName}} default case")
			}
		}
	{{- else}}
default:
		// Void case - no data to write
		if writer, ok := {{$prefix}}{{$fieldName}}.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
			bytesWritten, err = writer.WriteTo(w)
			totalBytes += bytesWritten
			if err != nil {
				return totalBytes, errors.Wrap(err, "failed to write void switch field {{$fieldName}} default case")
			}
		}
	{{- end}}
    {{- else}}
    default:
        {{- if isCompareToFieldMapper $sw $container }}
        // Mapper-backed discriminator with no explicit data for this value: treat as void
        {{- else if eq .field.Type.TypeName "pk.Field" }}
        // No explicit default; treat as void (no data)
        {{- else }}
        return totalBytes, fmt.Errorf("switch field {{$fieldName}}: unknown case value %s (no default defined in protocol)", compareValue{{$fieldName}})
        {{- end }}
    {{- end}}
}
	{{- else}}
	_ = {{$prefix}}{{$fieldName}}
	{{- end}}
{{end}}
{{define "bitfieldTmpl"}}{{$bitfield := toBitfield .}}type {{.Name}} struct { {{range $bitfield.Fields}}
	{{toIdentifier .Name}} int64{{end}}
}

func (b *{{.Name}}) ReadFrom(r io.Reader) (int64, error) {
	// Calculate total bits and bytes needed
	totalBits := 0{{range $bitfield.Fields}}
	totalBits += {{.Size}}{{end}}
	
	if totalBits % 8 != 0 {
		return 0, fmt.Errorf("bitfield {{.Name}} total size %d is not a multiple of 8", totalBits)
	}
	
	numBytes := totalBits / 8
	data := make([]byte, numBytes)
	
	nn, err := io.ReadFull(r, data)
	if err != nil {
		return int64(nn), errors.Wrap(err, "failed to read bitfield {{.Name}}")
	}
	
	// Convert bytes to uint64 (big-endian)
	var packed uint64
	for i := 0; i < numBytes; i++ {
		packed |= uint64(data[i]) << (8 * (numBytes - 1 - i))
	}
	
	// Extract bit fields
	currentOffset := 0{{range $bitfield.Fields}}
	// Extract {{.Name}} ({{.Size}} bits, signed={{.Signed}})
	{{.Name}}_mask := uint64((1 << {{.Size}}) - 1)
	{{.Name}}_value := (packed >> (totalBits - currentOffset - {{.Size}})) & {{.Name}}_mask
	{{if .Signed}}// Sign extend if negative
	if {{.Name}}_value & (1 << ({{.Size}} - 1)) != 0 {
		// Sign extend by converting to signed and back
		signBit := uint64(1) << {{.Size}}
		{{.Name}}_value = {{.Name}}_value - signBit
	}
	b.{{toIdentifier .Name}} = int64({{.Name}}_value){{else}}b.{{toIdentifier .Name}} = int64({{.Name}}_value){{end}}
	currentOffset += {{.Size}}{{end}}
	
	return int64(nn), nil
}

func (b {{.Name}}) WriteTo(w io.Writer) (int64, error) {
	// Calculate total bits and bytes needed
	totalBits := 0{{range $bitfield.Fields}}
	totalBits += {{.Size}}{{end}}
	
	if totalBits % 8 != 0 {
		return 0, fmt.Errorf("bitfield {{.Name}} total size %d is not a multiple of 8", totalBits)
	}
	
	numBytes := totalBits / 8
	
	// Pack bit fields into uint64
	var packed uint64
	currentOffset := 0{{range $bitfield.Fields}}
	// Pack {{.Name}} ({{.Size}} bits)
	{{.Name}}_value := uint64(b.{{toIdentifier .Name}}) & ((1 << {{.Size}}) - 1)
	packed |= {{.Name}}_value << (totalBits - currentOffset - {{.Size}})
	currentOffset += {{.Size}}{{end}}
	
	// Convert uint64 to bytes (big-endian)
	data := make([]byte, numBytes)
	for i := 0; i < numBytes; i++ {
		data[i] = byte(packed >> (8 * (numBytes - 1 - i)))
	}
	
	nn, err := w.Write(data)
	return int64(nn), err
}
{{end}}

{{define "switchTmpl"}}{{$switch := toSwitch .}}type {{.Name}} struct {
	Value any
}

func (s *{{.Name}}) ReadFrom(r io.Reader) (int64, error) {
	// TODO: Switch types require context from parent container for compareTo field
	// This is a placeholder implementation
	return 0, errors.New("switch type {{.Name}} requires parent context for field '{{$switch.CompareTo}}'")
}

func (s {{.Name}}) WriteTo(w io.Writer) (int64, error) {
	// TODO: Switch types require context from parent container for compareTo field  
	// This is a placeholder implementation
	return 0, errors.New("switch type {{.Name}} requires parent context for field '{{$switch.CompareTo}}'")
}
{{end}}

{{define "mapperTmpl"}}{{$mapper := toMapper .}}type {{.Name}} struct {
	Value string
}

var {{.Name}}Mappings = map[int64]string{ {{range $key, $value := $mapper.Mappings}}
	{{$key}}: "{{$value}}",{{end}}
}

func (m *{{.Name}}) ReadFrom(r io.Reader) (int64, error) {
	var key {{if $mapper.Type}}{{toNative $mapper.Type.TypeName $mapper.Type nil false}}{{else}}pk.VarInt{{end}}
	n, err := key.ReadFrom(r)
	if err != nil {
		return n, errors.Wrap(err, "failed to read {{.Name}} key")
	}
	
	value, ok := {{.Name}}Mappings[int64(key)]
	if !ok {
		// Use numeric key as fallback for unknown/undocumented values
		m.Value = fmt.Sprintf("unknown_%d", key)
		return n, nil
	}
	m.Value = value
	return n, nil
}

func (m {{.Name}}) WriteTo(w io.Writer) (int64, error) {
	for k, v := range {{.Name}}Mappings {
		if v == m.Value {
			key := {{if $mapper.Type}}{{toNative $mapper.Type.TypeName $mapper.Type nil false}}{{else}}pk.VarInt{{end}}(k)
			return key.WriteTo(w)
		}
	}
	return 0, errors.Errorf("unknown {{.Name}} value: '%s'", m.Value)
}
{{end}}

{{define "registryEntryHolderTmpl"}}{{$reh := toRegistryEntryHolder .}}type {{.Name}} struct {
	IsRegistryID bool
	RegistryID   pk.VarInt
	{{if $reh.Otherwise.Type}}Data         {{toNative $reh.Otherwise.Type.TypeName $reh.Otherwise.Type nil false}}{{else}}Data         struct{}{{end}}
}

func (r *{{.Name}}) ReadFrom(reader io.Reader) (int64, error) {
	var totalBytes int64
	
	// Read the varint - it's either a registry ID or 0 (indicating data follows)
	var id pk.VarInt
	n, err := id.ReadFrom(reader)
	totalBytes += n
	if err != nil {
		return totalBytes, errors.Wrap(err, "failed to read registry entry holder ID")
	}
	
	if id != 0 {
		// Non-zero means this is a registry ID (subtract 1 to get actual ID)
		r.IsRegistryID = true
		r.RegistryID = id - 1
	} else {
		// Zero means data structure follows
		r.IsRegistryID = false
		{{if $reh.Otherwise.Type}}n, err = r.Data.ReadFrom(reader)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read registry entry holder data")
		}{{end}}
	}
	
	return totalBytes, nil
}

func (r {{.Name}}) WriteTo(w io.Writer) (int64, error) {
	var totalBytes int64
	
	if r.IsRegistryID {
		// Write registry ID + 1
		id := r.RegistryID + 1
		n, err := id.WriteTo(w)
		return totalBytes + n, errors.Wrap(err, "failed to write registry entry holder ID")
	} else {
		// Write 0 followed by data
		var zero pk.VarInt = 0
		n, err := zero.WriteTo(w)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to write registry entry holder zero ID")
		}
		{{if $reh.Otherwise.Type}}n, err = r.Data.WriteTo(w)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to write registry entry holder data")
		}{{end}}
	}
	
	return totalBytes, nil
}
{{end}}

{{define "registryEntryHolderSetTmpl"}}{{$rehs := toRegistryEntryHolderSet .}}type {{.Name}} struct {
	// RegistryEntryHolderSet can hold either a single named tag or a list of numeric IDs
	IsTagList bool
	Tag       {{if $rehs.Base.Type}}{{toNative $rehs.Base.Type.TypeName $rehs.Base.Type nil false}}{{else}}pk.String{{end}}
	IDs       models.Array[pk.VarInt,{{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}}]
}

func (r *{{.Name}}) ReadFrom(reader io.Reader) (int64, error) {
	var totalBytes int64

	// Read the varint count - if 0, a single tag follows; otherwise IDs follow
	var count pk.VarInt
	n, err := count.ReadFrom(reader)
	totalBytes += n
	if err != nil {
		return totalBytes, errors.Wrap(err, "failed to read registry entry holder set count")
	}

	if count == 0 {
		// Tag representation - read a single tag
		r.IsTagList = true
		n, err = r.Tag.ReadFrom(reader)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to read registry entry holder set tag")
		}
	} else {
		// IDs list representation - count already read, now read count-1 more IDs
		r.IsTagList = false
		ary := make([]{{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}}, count)
		ary[0] = {{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}}(count - 1)
		for i := 1; i < int(count); i++ {
			var id {{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}}
			n, err := id.ReadFrom(reader)
			totalBytes += n
			if err != nil {
				return totalBytes, errors.Wrapf(err, "failed to read registry entry holder set ID at index %d", i)
			}
			ary[i] = id
		}
		r.IDs.Ary.Ary = any(ary)
	}

	return totalBytes, nil
}

func (r {{.Name}}) WriteTo(w io.Writer) (int64, error) {
	var totalBytes int64
	
	if r.IsTagList {
		// Write 0 followed by single tag
		var zero pk.VarInt = 0
		n, err := zero.WriteTo(w)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to write registry entry holder set zero count for tag")
		}
		n, err = r.Tag.WriteTo(w)
		totalBytes += n
		return totalBytes, err
	} else {
		// Write IDs (with first ID + 1 as the "count")
		idsAry, ok := r.IDs.Ary.Ary.([]{{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}})
		if !ok || len(idsAry) == 0 {
			return totalBytes, nil
		}
		// Write first ID + 1
		firstPlusOne := {{if $rehs.Otherwise.Type}}{{toNative $rehs.Otherwise.Type.TypeName $rehs.Otherwise.Type nil false}}{{else}}pk.VarInt{{end}}(idsAry[0]) + 1
		n, err := firstPlusOne.WriteTo(w)
		totalBytes += n
		if err != nil {
			return totalBytes, errors.Wrap(err, "failed to write registry entry holder set first ID + 1")
		}
		// Write remaining IDs
		for i := 1; i < len(idsAry); i++ {
			n, err := idsAry[i].WriteTo(w)
			totalBytes += n
			if err != nil {
				return totalBytes, errors.Wrapf(err, "failed to write registry entry holder set ID at index %d", i)
			}
		}
	}
	
	return totalBytes, nil
}
{{end}}

{{define "entityMetadataLoopTmpl"}}{{$eml := toEntityMetadataLoop .}}type {{.Name}} struct {
	EndVal  pk.UnsignedByte
	Entries []{{if $eml.Type}}{{toNative $eml.Type.TypeName $eml.Type nil false}}{{else}}pk.UnsignedByte{{end}}
}

func (t *{{.Name}}) ReadFrom(r io.Reader) (totalRead int64, err error) {
	var bytesRead int64
	loopEntryCount := 0
	
	// Read entries in a loop until we encounter the terminator (endVal)
	for {
		// Read the next byte to check if it's the terminator
		var marker pk.UnsignedByte
		bytesRead, err = marker.ReadFrom(r)
		totalRead += bytesRead
		if err != nil {
			return totalRead, errors.Wrap(err, "failed to read entity metadata loop marker")
		}

		// Check if this is the terminator
		if marker == {{$eml.EndVal}} {
			t.EndVal = marker
			break
		}

		// Not a terminator - this byte is the Key field of an entry
		// Prepend the marker byte back to the reader so ReadFrom can read it
		var entry {{if $eml.Type}}{{toNative $eml.Type.TypeName $eml.Type nil false}}{{else}}pk.UnsignedByte{{end}}
		{{if $eml.Type}}
		markerBuf := []byte{byte(marker)}
		combinedReader := io.MultiReader(bytes.NewReader(markerBuf), r)
		bytesRead, err = entry.ReadFrom(combinedReader)
		{{else}}
		bytesRead, err = entry.ReadFrom(r)
		{{end}}
		totalRead += bytesRead
		if err != nil {
			return totalRead, errors.Wrap(err, "failed to read entity metadata loop entry" )
		}
		t.Entries = append(t.Entries, entry)
		loopEntryCount++
	}
	
	return totalRead, nil
}

func (t {{.Name}}) WriteTo(w io.Writer) (totalWritten int64, err error) {
	var bytesWritten int64
	
	// Write all entries
	for _, entry := range t.Entries {
		bytesWritten, err = entry.WriteTo(w)
		if err != nil {
			return totalWritten + bytesWritten, errors.Wrap(err, "failed to write entity metadata loop entry")
		}
		totalWritten += bytesWritten
	}
	
	// Write terminator
	t.EndVal = {{$eml.EndVal}}
	bytesWritten, err = t.EndVal.WriteTo(w)
	totalWritten += bytesWritten
	
	return totalWritten, errors.Wrap(err, "failed to write entity metadata loop terminator")
}
{{end}}

`
)

func add(a, b int) int { return a + b }

// isNBTFieldType checks if a field's type is models.NBTField or contains NBTField in an Option
func isNBTFieldType(field *datatypes.ContainerField) bool {
	if field.Type == nil {
		return false
	}
	// Check for direct models.NBTField or pk.NBTField (for backwards compatibility)
	if field.Type.TypeName == "models.NBTField" || field.Type.TypeName == "pk.NBTField" {
		return true
	}
	// Check for models.Option[models.NBTField] or models.Option[pk.NBTField]
	if strings.Contains(field.Type.TypeName, "models.Option[models.NBTField]") || strings.Contains(field.Type.TypeName, "models.Option[pk.NBTField]") {
		return true
	}
	// AnonymousNBT does not need initialization - it reads the tag type from the wire
	return false
}

// hasFieldMethods checks if a container has any fields that need ReadFrom/WriteTo calls
func hasFieldMethods(c *datatypes.Container) bool {
	skippedTypes := map[string]bool{
		"struct{}":       true,
		"[]byte":         true,
		"basetypes.Tags": true,
		"Tags":           true,
	}
	for _, field := range c.Fields {
		if field.Type != nil && !skippedTypes[field.Type.TypeName] {
			return true
		}
		// Also return true if there are switch fields
		if isTemplateSwitch(field) {
			return true
		}
	}
	return false
}

// isTemplateSwitch checks if a container field is a switch type
func isTemplateSwitch(field *datatypes.ContainerField) bool {
	return field.Type != nil && field.Type.Extras != nil && field.Type.TypeName == "pk.Field"
}

// getSwitchInfo extracts switch metadata from a field
func getSwitchInfo(field *datatypes.ContainerField) *datatypes.Switch {
	if field.Type != nil && field.Type.Extras != nil {
		if sw, ok := field.Type.Extras.(*datatypes.Switch); ok {
			return sw
		}
	}
	return nil
}

// isBitflagMemberAccess checks if the switch compareTo references a bitflag member
// e.g., "../action/add_player" or "flags/has_redirect_node"
func isBitflagMemberAccess(sw *datatypes.Switch) bool {
	if sw == nil || sw.CompareTo == "" {
		return false
	}
	path := strings.TrimPrefix(sw.CompareTo, "../")
	return strings.Contains(path, "/")
}

// getBitflagMemberName extracts the bitflag member name from a compareTo path
// e.g., "../action/add_player" -> "add_player"
func getBitflagMemberName(sw *datatypes.Switch) string {
	if sw == nil || sw.CompareTo == "" {
		return ""
	}
	path := strings.TrimPrefix(sw.CompareTo, "../")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// getCompareToFieldName converts the compareTo reference to a proper Go field name
// Returns the field name without any bitflag member suffix
func getCompareToFieldName(sw *datatypes.Switch) string {
	if sw == nil || sw.CompareTo == "" {
		return ""
	}

	path := sw.CompareTo

	// Handle parent references (../)
	if after, ok := strings.CutPrefix(path, "../"); ok {
		// Remove the "../" prefix
		path = after
	}

	// Handle sub-field or bitflag member access (e.g., "action/add_player")
	if strings.Contains(path, "/") {
		// Extract just the field name (first part before /)
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			return toIdentifier(parts[0])
		}
	}

	// Convert to identifier (capitalizes and handles special cases)
	return toIdentifier(path)
}

// isMapperExtras checks if a type has Mapper extras (indicating it's a mapper type)
func isMapperExtras(t *datatypes.Type) (bool, bool) {
	if t == nil || t.Extras == nil {
		return false, false
	}
	_, ok := t.Extras.(*datatypes.Mapper)
	return ok, ok
}

// isCompareToFieldMapper checks if the field referenced by a switch's compareTo is a mapper type
// It looks up the field in the container and checks its type
func isCompareToFieldMapper(sw *datatypes.Switch, container *datatypes.Container) bool {
	if sw == nil || sw.CompareTo == "" || container == nil {
		return false
	}

	// Get the field name (without parent references or sub-paths)
	fieldName := getCompareToFieldName(sw)
	if fieldName == "" {
		return false
	}

	// Find the field in the container
	for _, field := range container.Fields {
		if field == nil || field.Type == nil {
			continue
		}
		// Convert field name to identifier format for comparison
		if toIdentifier(field.Name) == fieldName {
			// Check if this field's type is a mapper based on its Extras
			if mapper, ok := isMapperExtras(field.Type); ok {
				return mapper
			}

			// Fallback: Check based on naming pattern
			// Mapper types typically have "Type" suffix and come from mappers in the protocol
			// (EntityMetadataEntryType, ParticleType, etc.)
			typeName := field.Type.TypeName
			if strings.HasSuffix(typeName, "Type") && typeName != "RestrictedKeyType" {
				// Additional check: mapper types are usually short names ending in Type
				// Exclude things like "HasKnownType" (boolean) or "FilterType" (VarInt)
				if !strings.Contains(typeName, "Has") && !strings.Contains(typeName, "Filter") {
					return true
				}
			}
		}
	}

	return false
}

// getCompareToExpression generates the correct expression for comparing a field value
// For mapper types, it accesses the .Value field; for others, it uses fmt.Sprintf
func getCompareToExpression(sw *datatypes.Switch, container *datatypes.Container, prefix string) string {
	fieldName := getCompareToFieldName(sw)
	if fieldName == "" {
		return ""
	}

	fieldRef := prefix + fieldName

	// Check if the field is actually a mapper type by looking it up in the container
	if isCompareToFieldMapper(sw, container) {
		return fieldRef + ".Value"
	}

	// For non-mapper types, use fmt.Sprintf
	return fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", fieldRef)
}

// switchHasValidCases checks if a switch has any valid case statements
func switchHasValidCases(sw *datatypes.Switch) bool {
	if sw == nil {
		return false
	}
	// Check if there are any valid fields (non-void, non-empty cases)
	for _, fieldType := range sw.Fields {
		if fieldType != nil {
			// Pass nil for baseTypes since template helpers work with already-processed types
			nativeType := toNative(fieldType.TypeName, fieldType, nil, false)
			if nativeType != "models.Void" && nativeType != "[]byte" {
				return true
			}
		}
	}
	// Check if there's a valid default case (not just void)
	if sw.Default != nil {
		// Pass nil for baseTypes since template helpers work with already-processed types
		defaultType := toNative(sw.Default.TypeName, sw.Default, nil, false)
		return defaultType != "struct{}" && defaultType != "[]byte"
	}
	return false
}

// countNonSwitchFields counts fields that aren't switches
func countNonSwitchFields(fields []*datatypes.ContainerField) int {
	count := 0
	for _, field := range fields {
		if !isTemplateSwitch(field) {
			count++
		}
	}
	return count
}

// countSwitchFields counts switch fields
func countSwitchFields(fields []*datatypes.ContainerField) int {
	count := 0
	for _, field := range fields {
		if isTemplateSwitch(field) {
			count++
		}
	}
	return count
}

// notFunc is a helper for template negation
func notFunc(b bool) bool {
	return !b
}

// isNestedSwitch checks if a type has Extras that is a *datatypes.Switch
func isNestedSwitch(t *datatypes.Type) bool {
	if t == nil || t.Extras == nil {
		return false
	}
	_, ok := t.Extras.(*datatypes.Switch)
	return ok
}

// getNestedSwitchInfo extracts nested switch metadata from a type
func getNestedSwitchInfo(t *datatypes.Type) *datatypes.Switch {
	if t == nil || t.Extras == nil {
		return nil
	}
	if sw, ok := t.Extras.(*datatypes.Switch); ok {
		return sw
	}
	return nil
}

// dict creates a map from key-value pairs for use in templates
func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires an even number of arguments")
	}
	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		result[key] = values[i+1]
	}
	return result, nil
}

// containerHasParentReferences checks if any switch fields reference parent fields (../)
func containerHasParentReferences(c *datatypes.Container) bool {
	if c == nil {
		return false
	}
	for _, field := range c.Fields {
		if sw := getSwitchInfo(field); sw != nil {
			if strings.HasPrefix(sw.CompareTo, "../") {
				return true
			}
		}
	}
	return false
}

// collectParentContextRefs extracts a de-duplicated list of parent references
// used by switches within a container. References are returned as context keys
// like "action" or "action/add_player" (without the leading "../").
func collectParentContextRefs(c *datatypes.Container) []string {
	if c == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, f := range c.Fields {
		if sw := getSwitchInfo(f); sw != nil {
			if strings.HasPrefix(sw.CompareTo, "../") {
				key := strings.TrimPrefix(sw.CompareTo, "../")
				if !seen[key] {
					out = append(out, key)
					seen[key] = true
				}
			}
		}
	}
	return out
}

// getBitflagCheckCode generates code to check if a bitflag/bitfield has a specific bit/field set
// For bitflags (models.Bitflags): Returns "t.Action.HasBit(0)"
// For bitfields (struct): Returns "t.Flags.HasRedirectNode"
func getBitflagCheckCode(sw *datatypes.Switch, fieldAccessPrefix string) string {
	if !isBitflagMemberAccess(sw) {
		return ""
	}

	fieldName := getCompareToFieldName(sw)
	memberName := getBitflagMemberName(sw)

	// Map member names that are used in bitfields (struct fields)
	// These should be accessed as t.FieldName.MemberName
	bitfieldMembers := map[string]bool{
		"has_redirect_node":      true,
		"has_command":            true,
		"has_custom_suggestions": true,
		"command_node_type":      true,
	}

	// Check if this is a bitfield member (struct field access)
	if bitfieldMembers[memberName] {
		// Convert snake_case to CamelCase for struct field access
		structFieldName := toIdentifier(memberName)
		return fmt.Sprintf("%s%s.%s", fieldAccessPrefix, fieldName, structFieldName)
	}

	// Otherwise it's a bitflags member (bit position check)
	// Map flag names to bit positions for Bitflags type
	flagBitPositions := map[string]int{
		"add_player":          0,
		"initialize_chat":     1,
		"update_game_mode":    2,
		"update_listed":       3,
		"update_latency":      4,
		"update_display_name": 5,
		"update_hat":          6,
		"update_list_order":   7,
	}

	bitPos, ok := flagBitPositions[memberName]
	if !ok {
		// If we don't know the bit position, return a placeholder
		// This will cause a compile error that helps us identify missing mappings
		return fmt.Sprintf("/* TODO: Unknown bitflag member '%s' */ false", memberName)
	}

	return fmt.Sprintf("%s%s.HasBit(%d)", fieldAccessPrefix, fieldName, bitPos)
}

// typesAreEquivalent checks if two types are structurally equivalent
func typesAreEquivalent(t1, t2 *datatypes.Type) bool {
	if t1.Name != t2.Name || t1.TypeName != t2.TypeName {
		return false
	}

	// If both have no extras, they're equivalent
	if t1.Extras == nil && t2.Extras == nil {
		return true
	}

	// If only one has extras, they're different
	if (t1.Extras == nil) != (t2.Extras == nil) {
		return false
	}

	// Compare container types
	c1, isC1 := t1.Extras.(*datatypes.Container)
	c2, isC2 := t2.Extras.(*datatypes.Container)
	if isC1 && isC2 {
		if len(c1.Fields) != len(c2.Fields) {
			return false
		}
		for i := range c1.Fields {
			if c1.Fields[i].Name != c2.Fields[i].Name {
				return false
			}
			if c1.Fields[i].Type == nil && c2.Fields[i].Type == nil {
				continue
			}
			if c1.Fields[i].Type == nil || c2.Fields[i].Type == nil {
				return false
			}
			if c1.Fields[i].Type.TypeName != c2.Fields[i].Type.TypeName {
				return false
			}
		}
		return true
	}

	// For other types (arrays, switches, etc.), consider them equivalent if basic properties match
	// This is a conservative approach
	return true
}
func getMask() { //TO DO: FIX TO GET MASK
	/*
		package main

		import (
			"fmt"
			"math/bits"
		)

		func main() {
			currentOffset := uint64(0)
			offsets := []uint64{4, 4, 4}
			start := uint64(3513) // Binary: 110110111001 [13,11,9]

			for _, offset := range offsets {
				mask := uint64(1<<uint64(start) - 1)

				for i := range uint64(bits.Len64(start)) {
					if i >= offset+currentOffset || i < currentOffset {
					} else {
						//fmt.Print(1)
						mask = nullPosFromLeft(i, mask)
					}
				}
				fmt.Println()
				fmt.Printf("%064b & %064b\n", start, ^mask)
				fmt.Println((start & ^mask) >> currentOffset)
				currentOffset += offset
			}
		}
		func nullPosFromLeft(pos uint64, start uint64) uint64 {

			num := start
			// Create a mask with 1 at the desired position
			mask := 1 << pos

			// Invert the mask
			invertedMask := uint64(^mask)

			// Perform bitwise AND to set the bit to 0
			num &= uint64(invertedMask)
			//num &= mask

			//fmt.Printf("Number after setting bit at position %2d to 0: %4d (Binary: %012b)\n", pos, num, num)

			return num
		}

	*/
}

func toContainer(t *datatypes.Type) *datatypes.Container {
	return t.Extras.(*datatypes.Container)
}
func toArray(t *datatypes.Type) *datatypes.Array {
	return t.Extras.(*datatypes.Array)
}

func toTopBitSetTerminatedArray(t *datatypes.Type) *datatypes.TopBitSetTerminatedArray {
	return t.Extras.(*datatypes.TopBitSetTerminatedArray)
}

func toBitfield(t *datatypes.Type) *datatypes.Bitfield {
	return t.Extras.(*datatypes.Bitfield)
}

func toOption(t *datatypes.Type) *datatypes.Option {
	return t.Extras.(*datatypes.Option)
}

func toSwitch(t *datatypes.Type) *datatypes.Switch {
	return t.Extras.(*datatypes.Switch)
}

func toMapper(t *datatypes.Type) *datatypes.Mapper {
	return t.Extras.(*datatypes.Mapper)
}

func toBuffer(t *datatypes.Type) *datatypes.Buffer {
	return t.Extras.(*datatypes.Buffer)
}

func isContainer(t *datatypes.Type) (*datatypes.Container, bool) {
	if strings.ToLower(t.TypeName) == "container" {
		if container, ok := t.Extras.(*datatypes.Container); ok {
			return container, true
		}
	}
	return nil, false
}

func isArray(t *datatypes.Type) (*datatypes.Array, bool) {
	if strings.ToLower(t.TypeName) == "array" {
		if array, ok := t.Extras.(*datatypes.Array); ok {
			return array, true
		}
	}
	return nil, false
}

func isBitfield(t *datatypes.Type) (*datatypes.Bitfield, bool) {
	if strings.ToLower(t.TypeName) == "bitfield" {
		if bitfield, ok := t.Extras.(*datatypes.Bitfield); ok {
			return bitfield, true
		}
	}
	return nil, false
}

func isBitflags(t *datatypes.Type) (*datatypes.Bitflags, bool) {
	if strings.ToLower(t.TypeName) == "bitflags" {
		if bitflags, ok := t.Extras.(*datatypes.Bitflags); ok {
			return bitflags, true
		}
	}
	return nil, false
}

func isSwitch(t *datatypes.Type) (*datatypes.Switch, bool) {
	if strings.ToLower(t.TypeName) == "switch" {
		if switchType, ok := t.Extras.(*datatypes.Switch); ok {
			return switchType, true
		}
	}
	return nil, false
}

// Generic Phase 1 helpers for bitflag wrappers
func isBitflagsField(f *datatypes.ContainerField) bool {
	if f == nil || f.Type == nil || f.Type.Extras == nil {
		return false
	}
	_, ok := f.Type.Extras.(*datatypes.Bitflags)
	return ok
}

func getBitflagsForField(f *datatypes.ContainerField) *datatypes.Bitflags {
	if f == nil || f.Type == nil || f.Type.Extras == nil {
		return nil
	}
	if bf, ok := f.Type.Extras.(*datatypes.Bitflags); ok {
		return bf
	}
	return nil
}

// arrayElementTypeName extracts the element type name from a formatted
// models.Array[...] type string.
func arrayElementTypeName(typeName string) string {
	// Expect format: models.Array[<Len>,<Elem>]
	start := strings.Index(typeName, ",")
	end := strings.LastIndex(typeName, "]")
	if start < 0 || end < 0 || start+1 >= end {
		return ""
	}
	return strings.TrimSpace(typeName[start+1 : end])
}

// isArrayWithContextElements returns true if the field is an array whose
// element type has recorded parent context requirements.
func isArrayWithContextElements(field *datatypes.ContainerField) bool {
	if field == nil || field.Type == nil {
		return false
	}
	if !strings.HasPrefix(field.Type.TypeName, "models.Array[") {
		return false
	}
	elem := arrayElementTypeName(field.Type.TypeName)
	_, ok := parentContextRequirements[elem]
	return ok
}

// isExplicitCountArray returns true if the field is an ExplicitCountArray
func isExplicitCountArray(field *datatypes.ContainerField) bool {
	if field == nil || field.Type == nil {
		return false
	}
	return strings.HasPrefix(field.Type.TypeName, "models.ExplicitCountArray[")
}

// explicitCountArrayFieldName extracts the count field name for an ExplicitCountArray field.
// Returns empty string if not found.
func explicitCountArrayFieldName(container *datatypes.Container, fieldName string) string {
	if container == nil {
		return ""
	}
	// Look up in explicitCountArrayFields map
	mappingKey := fmt.Sprintf("%s.%s", container.GetName(), fieldName)
	return explicitCountArrayFields[mappingKey]
}

// getParentRefsForArrayContext returns the recorded context keys required by the
// array's element type.
func getParentRefsForArrayContext(field *datatypes.ContainerField) []string {
	if field == nil || field.Type == nil {
		return nil
	}
	elem := arrayElementTypeName(field.Type.TypeName)
	return parentContextRequirements[elem]
}

// exprForCtxKey builds a Go expression string to read a parent's field/member
// referenced by a context key (e.g., "action/add_player") from the current
// container receiver variable "t".
func exprForCtxKey(container *datatypes.Container, key string) string {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return ""
	}
	parentField := toIdentifier(parts[0])
	if len(parts) > 1 {
		member := toIdentifier(parts[1])
		// Check if the parent field is a bitflags type
		if container != nil {
			for _, field := range container.Fields {
				if toIdentifier(field.Name) == parentField && isBitflagsField(field) {
					// Bitflags use methods that return bool, so call the method
					return fmt.Sprintf("t.%s.%s()", parentField, member)
				}
			}
		}
		// For bitfield members: use direct field access since bitfields have int64 fields
		// (e.g., t.Flags.HasCustomSuggestions)
		return fmt.Sprintf("t.%s.%s", parentField, member)
	}
	return fmt.Sprintf("t.%s", parentField)
}

// exprForCtxKeyWithPrefix builds a Go expression string to read a parent's field/member
// referenced by a context key, using the specified receiver prefix instead of "t".
func exprForCtxKeyWithPrefix(container *datatypes.Container, key string, prefix string) string {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return ""
	}
	parentField := toIdentifier(parts[0])
	if len(parts) > 1 {
		member := toIdentifier(parts[1])
		// Check if the parent field is a bitflags type
		if container != nil {
			for _, field := range container.Fields {
				if toIdentifier(field.Name) == parentField && isBitflagsField(field) {
					// Bitflags use methods that return bool, so call the method
					return fmt.Sprintf("%s.%s.%s()", prefix, parentField, member)
				}
			}
		}
		// For bitfield members: use direct field access since bitfields have int64 fields
		// (e.g., t.Flags.HasCustomSuggestions)
		return fmt.Sprintf("%s.%s.%s", prefix, parentField, member)
	}
	return fmt.Sprintf("%s.%s", prefix, parentField)
}

// isParentCompareTo reports if a switch compares to a parent reference (../..).
func isParentCompareTo(sw *datatypes.Switch) bool {
	return sw != nil && strings.HasPrefix(sw.CompareTo, "../")
}

// typeRequiresParentContext returns true if the type requires parent context for (de)serialization
func typeRequiresParentContext(t *datatypes.Type) bool {
	if t == nil {
		return false
	}
	// Check if the type's name is in the parentContextRequirements map
	_, requiresContext := parentContextRequirements[t.TypeName]
	return requiresContext
}

// getParentRefsForType returns the context keys required by a type
func getParentRefsForType(t *datatypes.Type) []string {
	if t == nil {
		return nil
	}
	return parentContextRequirements[t.TypeName]
}

// sanitizeIdentifier removes characters that are not valid in Go identifiers
func sanitizeIdentifier(s string) string {
	// Replace common invalid characters with underscores
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// formatRawDefinition formats a RawDefinition string for use in Go comments.
// It preserves multi-line JSON structure by prefixing each line with "// ".
func formatRawDefinition(raw string) string {
	if raw == "" {
		return ""
	}

	// Split into lines
	lines := strings.Split(raw, "\n")

	// If it's a single-line definition, return it as-is
	if len(lines) == 1 {
		return strings.TrimSpace(raw)
	}

	// For multi-line definitions, format each line with proper comment prefix
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n// ")
		}
		result.WriteString(strings.TrimRight(line, " \t"))
	}

	return result.String()
}

// ctxKeyForSwitch returns the context key to lookup for a parent-referenced compareTo.
func ctxKeyForSwitch(sw *datatypes.Switch) string {
	if sw == nil {
		return ""
	}
	return strings.TrimPrefix(sw.CompareTo, "../")
}

func bitflagsWrapperName(container *datatypes.Container, field *datatypes.ContainerField) string {
	if container == nil || field == nil {
		return ""
	}
	return toIdentifier(container.GetName() + field.Name + "Bitflags")
}

func resolveFieldTypeForBitflags(container *datatypes.Container, field *datatypes.ContainerField) string {
	if isBitflagsField(field) {
		return bitflagsWrapperName(container, field)
	}
	if field != nil && field.Type != nil {
		return field.Type.TypeName
	}
	return ""
}

const bitflagWrapperTmpl = `
// START bitflagWrapperTmpl
{{define "bitflagWrapperTmpl"}}
{{$c := .container}}{{$f := .field}}{{$name := wrapperName $c $f}}
{{ $bf := bitflags $f }}
{{ $under := "" }}
{{ if $bf.Type }}{{ $under = toNative $bf.Type.TypeName $bf.Type nil false }}{{ end }}
// {{$name}} provides named accessors over a bitflag field.
type {{$name}} struct {
    {{- if $bf.Type }} {{ $under }} {{ else }} models.Bitflags {{ end }}
}

{{- range $i, $flag := $bf.Flags }}
func (bf {{$name}}) {{toIdentifier $flag}}() bool {
    {{- if eq $under "pk.UnsignedByte" }}
    v := uint64(uint8(bf.UnsignedByte))
    {{- else if eq $under "pk.Byte" }}
    v := uint64(uint8(bf.Byte))
    {{- else if eq $under "pk.UnsignedShort" }}
    v := uint64(uint16(bf.UnsignedShort))
    {{- else if eq $under "pk.Short" }}
    v := uint64(uint16(bf.Short))
    {{- else if eq $under "pk.Int" }}
    v := uint64(uint32(bf.Int))
    {{- else if eq $under "pk.Long" }}
    v := uint64(uint64(bf.Long))
    {{- else if eq $under "models.UInt32" }}
    v := uint64(uint32(bf.UInt32))
    {{- else if eq $under "models.UInt64" }}
    v := uint64(bf.UInt64)
    {{- else }}
    return bf.Bitflags.HasBit({{$i}})
    {{- end }}
    return (v & (1 << {{$i}})) != 0
}

func (bf *{{$name}}) Set{{toIdentifier $flag}}(value bool) {
    {{- if eq $under "pk.UnsignedByte" }}
    v := uint8(bf.UnsignedByte)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UnsignedByte = pk.UnsignedByte(v)
    {{- else if eq $under "pk.Byte" }}
    v := uint8(bf.Byte)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Byte = pk.Byte(v)
    {{- else if eq $under "pk.UnsignedShort" }}
    v := uint16(bf.UnsignedShort)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UnsignedShort = pk.UnsignedShort(v)
    {{- else if eq $under "pk.Short" }}
    v := uint16(bf.Short)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Short = pk.Short(v)
    {{- else if eq $under "pk.Int" }}
    v := uint32(bf.Int)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Int = pk.Int(int32(v))
    {{- else if eq $under "pk.Long" }}
    v := uint64(bf.Long)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Long = pk.Long(int64(v))
    {{- else if eq $under "models.UInt32" }}
    v := uint32(bf.UInt32)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UInt32 = models.UInt32(v)
    {{- else if eq $under "models.UInt64" }}
    v := uint64(bf.UInt64)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UInt64 = models.UInt64(v)
    {{- else }}
    bf.Bitflags.SetFlag({{$i}}, value)
    {{- end }}
}
{{- end }}
{{end}}

`

// Standalone bitflags type wrapper generation
const bitflagWrapperTypeTmpl = `
// START bitflagWrapperTypeTmpl

{{define "bitflagWrapperTypeTmpl"}}
{{$name := .Name}}
{{ $bit := .Extras }}
{{ $under := "" }}
{{ if $bit.Type }}{{ $under = toNative $bit.Type.TypeName $bit.Type nil false }}{{ end }}
// {{$name}} provides named accessors over a bitflag field.
type {{$name}} struct {
    {{- if $bit.Type }} {{ $under }} {{ else }} models.Bitflags {{ end }}
}
{{- range $i, $flag := $bit.Flags }}
func (bf {{$name}}) {{toIdentifier $flag}}() bool {
    {{- if eq $under "pk.UnsignedByte" }}
    v := uint64(uint8(bf.UnsignedByte))
    {{- else if eq $under "pk.Byte" }}
    v := uint64(uint8(bf.Byte))
    {{- else if eq $under "pk.UnsignedShort" }}
    v := uint64(uint16(bf.UnsignedShort))
    {{- else if eq $under "pk.Short" }}
    v := uint64(uint16(bf.Short))
    {{- else if eq $under "pk.Int" }}
    v := uint64(uint32(bf.Int))
    {{- else if eq $under "pk.Long" }}
    v := uint64(uint64(bf.Long))
    {{- else if eq $under "models.UInt32" }}
    v := uint64(uint32(bf.UInt32))
    {{- else if eq $under "models.UInt64" }}
    v := uint64(bf.UInt64)
    {{- else }}
    return bf.Bitflags.HasBit({{$i}})
    {{- end }}
    return (v & (1 << {{$i}})) != 0
}

func (bf *{{$name}}) Set{{toIdentifier $flag}}(value bool) {
    {{- if eq $under "pk.UnsignedByte" }}
    v := uint8(bf.UnsignedByte)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UnsignedByte = pk.UnsignedByte(v)
    {{- else if eq $under "pk.Byte" }}
    v := uint8(bf.Byte)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Byte = pk.Byte(v)
    {{- else if eq $under "pk.UnsignedShort" }}
    v := uint16(bf.UnsignedShort)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UnsignedShort = pk.UnsignedShort(v)
    {{- else if eq $under "pk.Short" }}
    v := uint16(bf.Short)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Short = pk.Short(v)
    {{- else if eq $under "pk.Int" }}
    v := uint32(bf.Int)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Int = pk.Int(int32(v))
    {{- else if eq $under "pk.Long" }}
    v := uint64(bf.Long)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.Long = pk.Long(int64(v))
    {{- else if eq $under "models.UInt32" }}
    v := uint32(bf.UInt32)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UInt32 = models.UInt32(v)
    {{- else if eq $under "models.UInt64" }}
    v := uint64(bf.UInt64)
    if value { v |= (1 << {{$i}}) } else { v &^= (1 << {{$i}}) }
    bf.UInt64 = models.UInt64(v)
    {{- else }}
    bf.Bitflags.SetFlag({{$i}}, value)
    {{- end }}
}
{{- end }}
{{end}}

`

func isOption(t *datatypes.Type) (*datatypes.Option, bool) {
	if strings.ToLower(t.TypeName) == "option" {
		if option, ok := t.Extras.(*datatypes.Option); ok {
			return option, true
		}
	}
	return nil, false
}

func isMapper(t *datatypes.Type) bool {
	return strings.ToLower(t.TypeName) == "mapper"
}

func isBuffer(t *datatypes.Type) bool {
	return strings.ToLower(t.TypeName) == "buffer"
}

func isRegistryEntryHolder(t *datatypes.Type) (*datatypes.RegistryEntryHolder, bool) {
	if strings.ToLower(t.TypeName) == "registryentryholder" {
		if reh, ok := t.Extras.(*datatypes.RegistryEntryHolder); ok {
			return reh, true
		}
	}
	return nil, false
}

func toRegistryEntryHolder(t *datatypes.Type) *datatypes.RegistryEntryHolder {
	return t.Extras.(*datatypes.RegistryEntryHolder)
}

func isRegistryEntryHolderSet(t *datatypes.Type) (*datatypes.RegistryEntryHolderSet, bool) {
	if strings.ToLower(t.TypeName) == "registryentryholderset" {
		if rehs, ok := t.Extras.(*datatypes.RegistryEntryHolderSet); ok {
			return rehs, true
		}
	}
	return nil, false
}

func toRegistryEntryHolderSet(t *datatypes.Type) *datatypes.RegistryEntryHolderSet {
	return t.Extras.(*datatypes.RegistryEntryHolderSet)
}

func isEntityMetadataLoop(t *datatypes.Type) (*datatypes.EntityMetadataLoop, bool) {
	if strings.ToLower(t.TypeName) == "entitymetadataloop" {
		if eml, ok := t.Extras.(*datatypes.EntityMetadataLoop); ok {
			return eml, true
		}
	}
	return nil, false
}

func toEntityMetadataLoop(t *datatypes.Type) *datatypes.EntityMetadataLoop {
	return t.Extras.(*datatypes.EntityMetadataLoop)
}

// isPacketType checks if a type is a packet type (had packet_ prefix)
func isPacketType(t *datatypes.Type) bool {
	if t == nil {
		return false
	}
	meta, ok := packetMetadata[t.Name]
	return ok && meta.IsPacket
}

// getPacketID returns the packet ID for a packet type
func getPacketID(t *datatypes.Type) int32 {
	if t == nil {
		return 0
	}
	if meta, ok := packetMetadata[t.Name]; ok {
		return meta.PacketID
	}
	return 0
}

func processType(t *datatypes.Type, baseTypes map[string]string, isAnon bool, isGeneratingBaseTypes bool, packetIDMap map[string]int32) []*datatypes.Type {
	// EARLY DETECTION: Check for packet_ prefix BEFORE any name conversion
	// Try multiple keys: Name, TypeName, and Extras.GetName()
	originalName := t.Name
	originalNameLower := strings.ToLower(originalName)
	originalTypeName := t.TypeName
	originalTypeNameLower := strings.ToLower(originalTypeName)
	originalExtrasName := ""
	if t.Extras != nil {
		originalExtrasName = strings.ToLower(t.Extras.GetName())
	}

	isPacket := strings.HasPrefix(originalNameLower, "packet_") ||
		strings.HasPrefix(originalTypeNameLower, "packet_") ||
		strings.HasPrefix(originalExtrasName, "packet_")
	var packetID int32
	var foundID bool
	if isPacket {
		// Try all possible keys to find the packet ID
		for _, key := range []string{originalNameLower, originalTypeNameLower, originalExtrasName} {
			if key != "" {
				if id, ok := packetIDMap[key]; ok {
					packetID = id
					foundID = true
					fmt.Printf("DEBUG [processType]: Detected packet type '%s' (key='%s') with ID 0x%02X\n", originalName, key, packetID)
					break
				}
			}
		}
		if !foundID {
			fmt.Printf("WARNING [processType]: Packet type '%s' (TypeName='%s', ExtrasName='%s') has no ID mapping. Available keys: %v\n",
				originalName, originalTypeName, originalExtrasName, getMapKeys(packetIDMap))
		}
	}

	if t.Extras == nil {
		// Skip generating type aliases for meta-types that shouldn't exist as standalone types
		originalNameCheck := strings.ToLower(t.Name)
		if originalNameCheck == "option" || originalNameCheck == "array" || originalNameCheck == "container" {
			return []*datatypes.Type{}
		}

		// SPECIAL CASE: Expand packet type aliases by copying referenced container's fields
		// This allows packets that alias other containers to be full packet structs with PacketID support
		if isPacket && foundID {
			// Try to look up the referenced type in containerRegistry
			lookupKey := strings.ToLower(t.TypeName)
			if refContainer, ok := containerRegistry[lookupKey]; ok {
				fmt.Printf("DEBUG [processType]: Expanding packet type alias '%s' from container '%s'\n", t.Name, t.TypeName)
				// Create a new container for this packet, copying fields from the referenced container
				newContainer := &datatypes.Container{}
				newContainer.SetName(t.Name)

				// Deep copy the fields from the referenced container
				newFields := make([]*datatypes.ContainerField, len(refContainer.Fields))
				for i, field := range refContainer.Fields {
					// Create a copy of the field
					fieldCopy := &datatypes.ContainerField{
						Name: field.Name,
						Anon: field.Anon,
					}
					// Copy the type
					if field.Type != nil {
						fieldCopy.Type = &datatypes.Type{
							Name:          field.Type.Name,
							TypeName:      field.Type.TypeName,
							Comment:       field.Type.Comment,
							RawDefinition: field.Type.RawDefinition,
						}
						// Deep copy Extras if present (for complex types)
						if field.Type.Extras != nil {
							fieldCopy.Type.Extras = field.Type.Extras
						}
					}
					newFields[i] = fieldCopy
				}
				newContainer.Fields = newFields

				// Convert this type to a container so it gets proper packet struct treatment
				t.TypeName = "container"
				t.Extras = newContainer
				// Don't process as simple type alias - let container processing handle it below
				return processType(t, baseTypes, isAnon, isGeneratingBaseTypes, packetIDMap)
			}
		}

		// Save original values before ANY conversion (for comparison and identifier generation)
		originalNameValue := t.Name
		originalTypeNameValue := t.TypeName

		// Convert TypeName first to get the proper native type
		t.TypeName = toNative(t.TypeName, t, baseTypes, isGeneratingBaseTypes)
		// Check if TypeName needs basetypes prefix
		if !isGeneratingBaseTypes && !strings.Contains(t.TypeName, ".") {
			if _, ok := baseTypes[strings.ToLower(t.TypeName)]; ok {
				fmt.Printf("DEBUG [processType - simple type alias]: Found '%s' in baseTypes, adding prefix\n", t.TypeName)
				t.TypeName = "basetypes." + t.TypeName
			}
		}
		convertedNameValue := toNative(originalNameValue, t, baseTypes, isGeneratingBaseTypes)

		// Skip generating type aliases in these cases:
		// 1. Original name and typename were the same (e.g., both "varint")
		// 2. The converted name would create invalid syntax (e.g., "pk.VarInt" can't be a type name)
		// 3. The converted name is a native type that's already defined (e.g., "Void", "pk.*")
		if originalNameValue == originalTypeNameValue {
			fmt.Printf("DEBUG: Skipping type alias - original name == typename: '%s'\n", originalNameValue)
			return []*datatypes.Type{}
		}
		// Skip if the Name contains basetypes.Void (invalid type name with dot)
		// But allow semantic type aliases like ContainerID even if they alias pk.* types
		if t.TypeName == "models.Void" || strings.Contains(t.Name, "models.Void") || strings.Contains(convertedNameValue, "models.Void") {
			fmt.Printf("DEBUG: Skipping type alias for invalid Void type: name='%s' typename='%s'\n", convertedNameValue, t.TypeName)
			return []*datatypes.Type{}
		}
		// Skip standalone Void type but allow other pk.* aliases
		if t.TypeName == "models.Void" && originalNameValue == originalTypeNameValue {
			fmt.Printf("DEBUG: Skipping standalone Void type\n")
			return []*datatypes.Type{}
		}

		fmt.Printf("DEBUG: Generating type alias: name='%s' (original='%s') typename='%s'\n",
			convertedNameValue, originalNameValue, t.TypeName)

		// Convert name to a valid identifier using the ORIGINAL name (before toNative conversion)
		convertedName := toIdentifier(originalNameValue)
		if strings.HasPrefix(convertedName, "UnnamedType") {
			fmt.Printf("DEBUG [processType - t.Extras == nil]: UNNAMED TYPE GENERATED:\n")
			fmt.Printf("  originalName='%s'\n", originalName)
			fmt.Printf("  originalExtrasName='<nil Extras>'\n")
			fmt.Printf("  t.Name after GetName='%s'\n", t.Name)
			fmt.Printf("  convertedName='%s'\n", convertedName)
			fmt.Printf("  t.TypeName='%s'\n", t.TypeName)
			fmt.Printf("  t.Extras=%v\n", t.Extras != nil)
		}
		t.Name = convertedName

		return []*datatypes.Type{t}
	}

	// Get the actual name from Extras if t.Name is empty
	if t.Name == "" && t.Extras != nil {
		extrasName := t.Extras.GetName()
		fmt.Printf("DEBUG [processType]: Populating empty t.Name from Extras: '%s'\n", extrasName)
		t.Name = extrasName
	}
	convertedName := toIdentifier(t.Name)
	if strings.HasPrefix(convertedName, "UnnamedType") {
		fmt.Printf("DEBUG [processType]: UNNAMED TYPE GENERATED:\n")
		fmt.Printf("  originalName='%s'\n", originalName)
		fmt.Printf("  originalExtrasName='%s'\n", originalExtrasName)
		fmt.Printf("  t.Name after GetName='%s'\n", t.Name)
		fmt.Printf("  convertedName='%s'\n", convertedName)
		fmt.Printf("  t.TypeName='%s'\n", t.TypeName)
		fmt.Printf("  t.Extras=%v\n", t.Extras != nil)
	}
	t.Name = convertedName

	// Store packet metadata using the converted name
	// Note: namespace and direction will be set by processNamespace when processing actual packet types
	if isPacket {
		packetMetadata[convertedName] = struct {
			IsPacket  bool
			PacketID  int32
			Namespace string
			Direction string
		}{IsPacket: true, PacketID: packetID, Namespace: "", Direction: ""}
		fmt.Printf("DEBUG [processType]: Stored packet metadata for '%s' (ID: 0x%02X)\n", convertedName, packetID)
	}

	types := []*datatypes.Type{}
	if container, ok := isContainer(t); ok {
		useName := t.Extras.GetName()
		if useName == "" && t.Name != "" {
			useName = t.Name
		}

		convertedName := toIdentifier(useName)
		if strings.HasPrefix(convertedName, "UnnamedType") {
			fmt.Printf("DEBUG [processType - type == container]: UNNAMED TYPE GENERATED:\n")
			fmt.Printf("  originalName='%s'\n", originalName)
			fmt.Printf("  originalExtrasName='%s'\n", originalExtrasName)
			fmt.Printf("  t.Name after GetName='%s'\n", t.Name)
			fmt.Printf("  convertedName='%s'\n", convertedName)
			fmt.Printf("  t.TypeName='%s'\n", t.TypeName)
			fmt.Printf("  t.Extras=%#v\n", t.Extras)
			fmt.Printf("  \n")
		}
		t.Extras.SetName(convertedName)
		// Scope unnamed type counter to this struct so names are stable across runs.
		prevStructContext := currentStructContext
		currentStructContext = convertedName
		defer func() { currentStructContext = prevStructContext }()

		// DEBUG: Log container fields before filtering
		fmt.Printf("DEBUG [gen_packet.go]: Container '%s' has %d fields before filtering:\n", t.Name, len(container.Fields))
		for i, field := range container.Fields {
			typeInfo := "nil"
			if field.Type != nil {
				typeInfo = fmt.Sprintf("%s (typename=%s)", field.Type.Name, field.Type.TypeName)
			}
			fmt.Printf("  [%d] Field='%s' Type=%s Anon=%v\n", i, field.Name, typeInfo, field.Anon)
		}

		// Filter out fields with nil Type
		filteredFields := []*datatypes.ContainerField{}
		for _, field := range container.Fields {
			if field.Type != nil {
				filteredFields = append(filteredFields, field)
			}
		}

		// DEBUG: Log after filtering
		if len(filteredFields) != len(container.Fields) {
			fmt.Printf("DEBUG [gen_packet.go]: Container '%s' FILTERED OUT %d fields (from %d to %d)\n",
				t.Name, len(container.Fields)-len(filteredFields), len(container.Fields), len(filteredFields))
		}

		container.Fields = filteredFields

		// Record parent context requirements for this container if any switches
		// reference parent fields via "../" in compareTo paths.
		if containerHasParentReferences(container) {
			refs := collectParentContextRefs(container)
			if len(refs) > 0 {
				parentContextRequirements[t.Name] = refs
			}
		}
		for _, field := range container.Fields {
			field.Name = toIdentifier(field.Name)
			if field.Type == nil {
				continue
			}
			// DEBUG: Log basetype lookup
			// Try to look up by TypeName first (the actual type reference), then fall back to Name
			lookupKey := field.Type.TypeName
			if lookupKey == "" {
				lookupKey = field.Type.Name
			}
			// Try lookup with original case first, then lowercase (since protocol types are lowercase)
			// First try direct toNative conversion on the lookupKey, but skip for complex types with Extras
			// that need special processing (option, array, container, etc.)
			lookupKeyLower := strings.ToLower(lookupKey)
			isComplexType := field.Type.Extras != nil && (lookupKeyLower == "option" || lookupKeyLower == "array" || lookupKeyLower == "container" || lookupKeyLower == "switch" || lookupKeyLower == "bitfield" || lookupKeyLower == "topbitsetalternative" || lookupKeyLower == "topbitsetterminatedarray")
			if !isComplexType {
				nativeCheck := toNative(lookupKey, field.Type, baseTypes, isGeneratingBaseTypes)
				if nativeCheck != lookupKey && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
					// It's a primitive type, use the native conversion directly
					fmt.Printf("DEBUG [gen_packet.go]: Field '%s.%s' type '%s' CONVERTED to native '%s'\n",
						t.Name, field.Name, lookupKey, nativeCheck)
					field.Type.Name = nativeCheck
					field.Type.TypeName = nativeCheck
					continue // Skip the rest of processing for this field
				}
			}
			// Not a primitive or is a complex type, try baseTypes lookup
			fieldName, ok := baseTypes[lookupKey]
			if !ok {
				fieldName, ok = baseTypes[strings.ToLower(lookupKey)]
			}
			if ok {
				fmt.Printf("DEBUG [gen_packet.go]: Field '%s.%s' type '%s' (typename='%s') FOUND in baseTypes as '%s'\n",
					t.Name, field.Name, field.Type.Name, field.Type.TypeName, fieldName)
				// Check if the baseTypes value itself is a primitive
				nativeCheckOfBase := toNative(fieldName, nil, baseTypes, isGeneratingBaseTypes)
				if nativeCheckOfBase != fieldName && (strings.HasPrefix(nativeCheckOfBase, "pk.") || strings.Contains(nativeCheckOfBase, "[")) {
					// The basetype resolves to a primitive
					field.Type.Name = nativeCheckOfBase
					field.Type.TypeName = nativeCheckOfBase
				} else if !strings.Contains(fieldName, ".") {
					// Not a primitive - convert fieldName to proper Go identifier
					fieldName = toIdentifier(fieldName)
					if isGeneratingBaseTypes {
						// Within basetypes package, no prefix needed
						field.Type.Name = fieldName
						field.Type.TypeName = fieldName
					} else {
						// Outside basetypes, add package qualifier
						field.Type.Name = "basetypes." + fieldName
						field.Type.TypeName = "basetypes." + fieldName
					}
				} else {
					field.Type.Name = fieldName
					field.Type.TypeName = fieldName
				}
			} else {
				// Not found in baseTypes, convert using toNative as fallback
				fmt.Printf("DEBUG [gen_packet.go]: Field '%s.%s' type '%s' (typename='%s') NOT FOUND in baseTypes, converting\n",
					t.Name, field.Name, field.Type.Name, field.Type.TypeName)
				field.Type.Name = toIdentifier(field.Type.Name)
				// IMPORTANT: If this field has Extras (inline complex type), defer toNative until after
				if field.Type.Extras == nil {
					// Don't convert TypeName if it's already a native or package-qualified type
					if !strings.Contains(field.Type.TypeName, ".") {
						field.Type.TypeName = toNative(field.Type.TypeName, field.Type, baseTypes, isGeneratingBaseTypes)
					}
				}
			}
			// Special handling for topBitSetTerminatedArray which might have lost its Extras
			lowerTypeName := strings.ToLower(field.Type.TypeName)
			if lowerTypeName == "topbitsetterminatedarray" || lowerTypeName == "topbitsetalternative" {
				if field.Type.Extras == nil {
					fmt.Printf("WARNING [gen_packet.go]: topBitSetTerminatedArray field '%s.%s' has no Extras, skipping special processing\n", t.Name, field.Name)
				} else {
					fmt.Printf("DEBUG [gen_packet.go]: Processing topBitSetTerminatedArray field '%s.%s' with Extras\n", t.Name, field.Name)
					parentName := t.Name
					tbsa := toTopBitSetTerminatedArray(field.Type)
					array := (*datatypes.Array)(nil)
					if tbsa != nil && tbsa.Type != nil {
						// Convert TopBitSetTerminatedArray to Array for processing
						array = &datatypes.Array{Type: tbsa.Type}
					}
					if array == nil || array.Type == nil {
						fmt.Printf("WARNING [gen_packet.go]: topBitSetTerminatedArray field '%s.%s' has invalid array Extras\n", t.Name, field.Name)
						// Fall through to default handling
					} else {
						// Build child type name
						childName := field.Name + "Entry"
						parentType := array.Type
						childType := createChildType(parentName, childName, parentType, baseTypes, isGeneratingBaseTypes)
						entryTypeName := childType.Name
						if childType.Name != childType.TypeName {
							types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
						}
						// Check if entry type needs basetypes prefix
						if !isGeneratingBaseTypes && entryTypeName != "" && !strings.Contains(entryTypeName, ".") && !strings.HasPrefix(entryTypeName, "pk.") {
							lookupKey := strings.ToLower(entryTypeName)
							if _, ok := baseTypes[lookupKey]; ok {
								fmt.Printf("DEBUG [gen_packet.go]: TopBitSetTerminatedArray entry type '%s' FOUND in baseTypes, adding prefix\n", entryTypeName)
								entryTypeName = "basetypes." + entryTypeName
							}
						}
						// Set field type to models.TopBitSetTerminatedArray[EntryType]
						fmt.Printf("DEBUG [gen_packet.go]: Setting topBitSetTerminatedArray field '%s.%s' type to models.TopBitSetTerminatedArray[%s]\n", t.Name, field.Name, entryTypeName)
						field.Type.TypeName = "models.TopBitSetTerminatedArray[" + entryTypeName + "]"
						field.Type.Extras = nil
					}
				}
			} else if field.Type.Extras != nil {
				parentName := t.Name
				if t.Name == "EntityEquipment" || field.Name == "Equipments" {
					fmt.Printf("DEBUG [gen_packet.go]: About to switch on field '%s.%s' TypeName='%s', Extras=%v\n", t.Name, field.Name, field.Type.TypeName, field.Type.Extras != nil)
				}
				switch strings.ToLower(field.Type.TypeName) {
				case "buffer":
					// Buffer types can be fixed-size or variable-length
					buffer := toBuffer(field.Type)
					if buffer.Count > 0 {
						// Fixed-size buffer - use predefined FixedBufferN type if available
						if fixedType, err := models.GetFixedBufferTypeName(buffer.Count); err == nil {
							field.Type.TypeName = fixedType
						} else {
							// No predefined type for this size - fall back to pk.ByteArray
							fmt.Printf("WARNING: No FixedBuffer type for size %d, using pk.ByteArray instead\n", buffer.Count)
							field.Type.TypeName = "pk.ByteArray"
						}
					} else {
						// Variable-length buffer (with countType) - use pk.ByteArray
						field.Type.TypeName = "pk.ByteArray"
					}
					field.Type.Extras = nil // Clear extras since we've converted to a simple type
					continue
				case "container":
					childName := field.Name
					parentType := field.Type
					childType := createChildType(parentName, childName, parentType, baseTypes, isGeneratingBaseTypes)
					// Update field type to reference the child type
					// Child types from containers are always local to the current package
					field.Type.TypeName = childType.Name
					field.Type.Extras = nil
					types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
				case "array":
					array := toArray(field.Type)
					if array.Type == nil {
						continue
					}

					childName := array.Type.Name
					// If array element has no name, try to get it from Extras
					if childName == "" && array.Type.Extras != nil {
						childName = array.Type.Extras.GetName()
					}
					// Try to get the element type from TypeName if Name is empty
					if childName == "" {
						childName = array.Type.TypeName
					}
					// Ensure uniqueness per struct field: if no name derived, use field name; otherwise prepend it
					if childName == "" {
						childName = field.Name
					} else {
						childName = field.Name + "_" + childName
					}

					// Check if this is a native type that doesn't need a child type
					nativeElementType := toNative(childName, array.Type, baseTypes, isGeneratingBaseTypes)
					elementTypeName := ""

					// Check if type has Extras (complex inline type)
					if array.Type.Extras != nil {
						// Complex type with Extras - need to create a child type
						parentType := array.Type
						childType := createChildType(parentName, childName, parentType, baseTypes, isGeneratingBaseTypes)
						elementTypeName = childType.Name
						if childType.Name != childType.TypeName {
							types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
						}
					} else {
						// No Extras - simple type reference, use converted name
						elementTypeName = nativeElementType
					}
					// Check if element type needs basetypes prefix
					if !isGeneratingBaseTypes && elementTypeName != "" && !strings.Contains(elementTypeName, ".") && !strings.HasPrefix(elementTypeName, "pk.") {
						// Check if it's in baseTypes map
						lookupKey := strings.ToLower(elementTypeName)
						if _, ok := baseTypes[lookupKey]; ok {
							fmt.Printf("DEBUG [gen_packet.go]: Array element type '%s' (lookup='%s') FOUND in baseTypes, adding prefix\n", elementTypeName, lookupKey)
							elementTypeName = "basetypes." + elementTypeName
						} else {
							fmt.Printf("DEBUG [gen_packet.go]: Array element type '%s' (lookup='%s') NOT in baseTypes (isGenBase=%v hasExtras=%v)\n",
								elementTypeName, lookupKey, isGeneratingBaseTypes, array.Type.Extras != nil)
						}
					}

					// Check if this array has an explicit count field (e.g., "count": "addedComponentCount")
					if array.CountFieldName != "" {
						// Explicit count array - uses ExplicitCountArray type
						fmt.Printf("DEBUG [gen_packet.go]: Array field '%s.%s' has explicit count field '%s'\n",
							t.Name, field.Name, array.CountFieldName)
						// Check if the count field exists in the current container
						countFieldExists := false
						if container != nil {
							for _, f := range container.Fields {
								if strings.EqualFold(f.Name, array.CountFieldName) {
									countFieldExists = true
									break
								}
							}
						}
						// Only mark as requiring parent context if count field is NOT in this container
						// (i.e., it requires the parent to provide it)
						if !countFieldExists {
							fmt.Printf("DEBUG [gen_packet.go]: Count field '%s' NOT found in container '%s', marking as requiring parent context\n",
								array.CountFieldName, t.Name)
							if parentContextRequirements[t.Name] == nil {
								parentContextRequirements[t.Name] = []string{}
							}
							// Add the count field as a dependency
							parentContextRequirements[t.Name] = append(parentContextRequirements[t.Name], array.CountFieldName)
						} else {
							fmt.Printf("DEBUG [gen_packet.go]: Count field '%s' FOUND in container '%s', no parent context needed\n",
								array.CountFieldName, t.Name)
						}
						// Store the array field -> count field mapping
						mappingKey := fmt.Sprintf("%s.%s", t.Name, field.Name)
						explicitCountArrayFields[mappingKey] = toIdentifier(array.CountFieldName)
						// Generate ExplicitCountArray type
						field.Type.TypeName = fmt.Sprintf("models.ExplicitCountArray[%s]", elementTypeName)
						field.Type.Extras = nil
					} else {
						// Implicit count array - uses regular Array type
						countTypeName := "pk.VarInt"
						if array.CountType != nil {
							countTypeName = toNative(array.CountType.Name, array.CountType, baseTypes, isGeneratingBaseTypes)
						}
						// Array is now in models package
						field.Type.TypeName = "models.Array[" + countTypeName + "," + elementTypeName + "]"
						field.Type.Extras = nil
					}
				case "topBitSetTerminatedArray", "topbitsetalternative":
					fmt.Printf("DEBUG [gen_packet.go]: Found topBitSetTerminatedArray field '%s.%s', TypeName='%s', Extras=%v\n", t.Name, field.Name, field.Type.TypeName, field.Type.Extras != nil)
					// topBitSetTerminatedArray: array of entries terminated by a byte with MSB set
					// Structure: ["topBitSetTerminatedArray", { "type": [containerDefinition] }]
					// The nested container defines the structure of each entry
					// We extract this nested type and generate an entry type to hold it

					if field.Type.Extras == nil {
						fmt.Printf("WARNING [gen_packet.go]: topBitSetTerminatedArray field '%s.%s' has no Extras\n", t.Name, field.Name)
						continue
					}

					// Extract the nested container definition
					// The Extras contains the array metadata with Type pointing to the entry container
					array := toArray(field.Type)
					if array == nil || array.Type == nil {
						fmt.Printf("WARNING [gen_packet.go]: topBitSetTerminatedArray field '%s.%s' has invalid array Extras\n", t.Name, field.Name)
						continue
					}

					// Build child type name using the pattern ParentName_FieldName + "Entry"
					childName := field.Name + "Entry"

					// Create entry type from the nested container definition
					parentType := array.Type
					childType := createChildType(parentName, childName, parentType, baseTypes, isGeneratingBaseTypes)
					entryTypeName := childType.Name

					// Process the entry type recursively to handle any nested complexity
					if childType.Name != childType.TypeName {
						types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					}

					// Check if entry type needs basetypes prefix
					if !isGeneratingBaseTypes && entryTypeName != "" && !strings.Contains(entryTypeName, ".") && !strings.HasPrefix(entryTypeName, "pk.") {
						lookupKey := strings.ToLower(entryTypeName)
						if _, ok := baseTypes[lookupKey]; ok {
							fmt.Printf("DEBUG [gen_packet.go]: TopBitSetTerminatedArray entry type '%s' FOUND in baseTypes, adding prefix\n", entryTypeName)
							entryTypeName = "basetypes." + entryTypeName
						}
					}

					// Set field type to models.TopBitSetTerminatedArray[EntryType]
					fmt.Printf("DEBUG [gen_packet.go]: Processing topBitSetTerminatedArray field '%s.%s' with entry type '%s'\n", t.Name, field.Name, entryTypeName)
					newTypeName := "models.TopBitSetTerminatedArray[" + entryTypeName + "]"
					fmt.Printf("DEBUG [gen_packet.go]: Setting field.Type.TypeName to '%s'\n", newTypeName)
					field.Type.TypeName = newTypeName
					field.Type.Extras = nil
					fmt.Printf("DEBUG [gen_packet.go]: After setting, field.Type.TypeName='%s'\n", field.Type.TypeName)
				case "bitfield":
					// Bitfield types within fields - generate specialized struct with custom ReadFrom/WriteTo
					childType := createChildType(parentName, field.Name, field.Type, baseTypes, isGeneratingBaseTypes)
					field.Type.TypeName = childType.Name
					// Keep Extras so we can generate proper ReadFrom/WriteTo with field information
					// field.Type.Extras will be used by the bitfieldTmpl template
					types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					continue
				case "option":
					option := toOption(field.Type)
					if option.Type == nil {
						continue
					}
					// DEBUG: Log option inner type
					if strings.Contains(t.Name, "BlockPredicate") {
						fmt.Printf("DEBUG [gen_packet.go]: Option field '%s.%s' inner TypeName='%s' Name='%s' Extras=%v\n",
							t.Name, field.Name, option.Type.TypeName, option.Type.Name, option.Type.Extras != nil)
					}
					// Check if the option contains complex types that need special handling
					optionInnerTypeName := strings.ToLower(option.Type.TypeName)

					// Special handling for optional-switch: don't generate a child type, just use pk.Field like regular switches
					if optionInnerTypeName == "switch" && option.Type.Extras != nil {
						// Optional switch field - treat like regular switch but with protocol-defined boolean presence
						// The boolean prefix will be handled by the protodef protocol definition
						field.Type.TypeName = "pk.Field"       // Treat as regular switch field
						field.Type.Extras = option.Type.Extras // Preserve switch metadata
						// Keep field.Type.Extras for template to access switch info

						// CRITICAL: Ensure the comparison field exists in the container
						// For optional switches like ["option", ["switch", {"compareTo": "type", ...}]],
						// the comparison field (e.g., "type") must be a sibling field in the container
						if sw, ok := option.Type.Extras.(*datatypes.Switch); ok && sw.CompareTo != "" {
							compareFieldName := getCompareToFieldName(sw)
							if compareFieldName != "" {
								// Check if the comparison field exists in the container
								fieldExists := false
								for _, f := range container.Fields {
									if toIdentifier(f.Name) == compareFieldName {
										fieldExists = true
										break
									}
								}

								if fieldExists {
									// Mark the container as requiring context for this switch
									// so the template knows to use the parent field for comparison
									if _, exists := parentContextRequirements[t.Name]; !exists {
										parentContextRequirements[t.Name] = []string{}
									}
									parentContextRequirements[t.Name] = append(parentContextRequirements[t.Name], compareFieldName)
									fmt.Printf("DEBUG [optional-switch]: Marked container '%s' as requiring context for switch comparison field '%s'\n",
										t.Name, compareFieldName)
								} else {
									fmt.Printf("WARNING [optional-switch]: Container '%s' field '%s' references comparison field '%s' which is NOT FOUND in container\n",
										t.Name, field.Name, compareFieldName)
								}
							}
						}

						// Process switch case types to generate proper names for containers
						// This handles inline containers in switch cases like ["container", [...]]
						if sw, ok := option.Type.Extras.(*datatypes.Switch); ok {
							// Process each switch case
							for caseName, caseType := range sw.Fields {
								if caseType == nil {
									continue
								}
								caseTypeLower := strings.ToLower(caseType.TypeName)

								// Check if this case contains a container or other complex type
								if caseType.Extras != nil &&
									(caseTypeLower == "" || caseTypeLower == "container" ||
										caseTypeLower == "bitfield" || caseTypeLower == "registryentryholder" ||
										caseTypeLower == "registryentryholderset" || caseTypeLower == "option" ||
										caseTypeLower == "mapper" || caseTypeLower == "array") {
									// Generate a child type for this complex case
									childTypeName := toIdentifier(parentName + "_" + field.Name + "_" + caseName)
									childType := *caseType
									childType.Name = childTypeName
									if childType.Extras != nil {
										childType.Extras.SetName(childTypeName)
									}
									// Process and add the child type
									childTypes := processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)
									types = append(types, childTypes...)
									// Update the case to reference the new type
									caseType.Name = childTypeName
									caseType.TypeName = childTypeName
									// Clear Extras since the fields have been extracted
									caseType.Extras = nil
									sw.Fields[caseName] = caseType
								} else {
									// Simple type - resolve via toNative() like regular switch cases
									lookupKey := caseType.TypeName
									if lookupKey == "" {
										lookupKey = caseType.Name
									}
									// Convert to native type
									nativeTypeName := toNative(lookupKey, caseType, baseTypes, isGeneratingBaseTypes)
									// Check if toNative converted it
									if nativeTypeName != lookupKey {
										// toNative converted it - check if it needs basetypes prefix
										if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
											if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
												nativeTypeName = "basetypes." + nativeTypeName
											} else if needsBaseTypesPrefix(nativeTypeName) {
												nativeTypeName = "basetypes." + nativeTypeName
											}
										}
										caseType.Name = nativeTypeName
										caseType.TypeName = nativeTypeName
										sw.Fields[caseName] = caseType
									}
								}
							}

							// Process default case if present
							if sw.Default != nil {
								defaultTypeLower := strings.ToLower(sw.Default.TypeName)
								if sw.Default.Extras != nil && (defaultTypeLower == "" || defaultTypeLower == "container" || defaultTypeLower == "registryentryholder" || defaultTypeLower == "registryentryholderset" || defaultTypeLower == "option" || defaultTypeLower == "mapper" || defaultTypeLower == "array") {
									// Generate a child type for this complex default case
									childTypeName := toIdentifier(parentName + "_" + field.Name + "_default")
									childType := *sw.Default
									childType.Name = childTypeName
									if childType.Extras != nil {
										childType.Extras.SetName(childTypeName)
									}
									// Process and add the child type
									childTypes := processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)
									types = append(types, childTypes...)
									// Update the default to reference the new type
									sw.Default.Name = childTypeName
									sw.Default.TypeName = childTypeName
									sw.Default.Extras = nil
								} else {
									// Simple type default - resolve via toNative()
									lookupKey := sw.Default.TypeName
									if lookupKey == "" {
										lookupKey = sw.Default.Name
										if lookupKey == "" {
											// No type info, skip
											continue
										}
									}
									// Convert to native type
									nativeTypeName := toNative(lookupKey, sw.Default, baseTypes, isGeneratingBaseTypes)
									// Check if toNative converted it
									if nativeTypeName != lookupKey {
										// toNative converted it - check if it needs basetypes prefix
										if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
											if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
												nativeTypeName = "basetypes." + nativeTypeName
											} else if needsBaseTypesPrefix(nativeTypeName) {
												nativeTypeName = "basetypes." + nativeTypeName
											}
										}
										sw.Default.Name = nativeTypeName
										sw.Default.TypeName = nativeTypeName
									}
								}
							}
						}

						continue // Skip further option processing
					}

					// Handle nested complex types that have Extras (registryEntryHolder, array with complex elements, etc.)
					if option.Type.Extras != nil && (optionInnerTypeName == "registryentryholderset" || optionInnerTypeName == "registryentryholder" ||
						optionInnerTypeName == "array" || optionInnerTypeName == "container" ||
						optionInnerTypeName == "option" || optionInnerTypeName == "mapper") {
						// Complex type with Extras - need to generate it as a child type
						fmt.Printf("DEBUG [gen_packet.go]: Option field '%s.%s' contains %s (Extras=%v)\n",
							t.Name, field.Name, optionInnerTypeName, option.Type.Extras != nil)
						// Create child type name from parent + field name
						fmt.Printf("DEBUG: Creating child type from parentName='%s' field.Name='%s' concat='%s'\n",
							parentName, field.Name, parentName+"_"+field.Name)
						childTypeName := toIdentifier(parentName + "_" + field.Name)
						fmt.Printf("DEBUG: childTypeName='%s'\n", childTypeName)
						// Create a copy of the option's inner type with the new name
						innerType := *option.Type
						innerType.Name = childTypeName
						if innerType.Extras != nil {
							innerType.Extras.SetName(childTypeName)
						}
						// Process the inner type recursively - this will handle arrays, containers, etc.
						types = append(types, processType(&innerType, baseTypes, false, isGeneratingBaseTypes, nil)...)
						// Option is now in models package
						field.Type.TypeName = "models.Option[" + childTypeName + "]"
						field.Type.Extras = nil
						continue
					}
					childName := field.Name
					parentType := field.Type
					childType := createChildType(parentName, childName, parentType, baseTypes, isGeneratingBaseTypes)
					nativeOptionName := toNative(option.Type.Name, option.Type, baseTypes, isGeneratingBaseTypes)
					// Replace struct{} and []byte with pk.ByteArray as they don't implement FieldEncoder
					if nativeOptionName == "struct{}" || nativeOptionName == "[]byte" {
						nativeOptionName = "pk.ByteArray"
					}
					// Check if option inner type needs basetypes prefix
					if !isGeneratingBaseTypes && !strings.Contains(nativeOptionName, ".") && !strings.HasPrefix(nativeOptionName, "pk.") {
						if _, ok := baseTypes[strings.ToLower(nativeOptionName)]; ok {
							nativeOptionName = "basetypes." + nativeOptionName
						}
					}
					// Option is now in models package
					field.Type.TypeName = "models.Option[" + nativeOptionName + "]"
					field.Type.Extras = nil // Mark as processed
					if childType.Name != childType.TypeName {
						types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					}
					continue // Skip toNative call below since we've already formatted
				case "switch":
					// DEBUG: Log all switch field processing
					fmt.Printf("DEBUG [processType switch]: parentName='%s', field.Name='%s'\n", parentName, field.Name)
					// Switch types are handled inline - use pk.Field for the field type
					// Keep Extras with switch metadata for inline generation in container methods
					field.Type.TypeName = "pk.Field"
					// Keep field.Type.Extras for template to access switch info
					// Resolve switch case types using baseTypes
					if switchType, ok := field.Type.Extras.(*datatypes.Switch); ok {
						// DEBUG: Log CommandNode ExtraNodeData processing
						if parentName == "CommandNode" && field.Name == "extraNodeData" {
							fmt.Printf("DEBUG [CommandNode.ExtraNodeData]: Processing switch with %d cases\n", len(switchType.Fields))
						}
						// Resolve each case type
						for caseName, caseType := range switchType.Fields {
							if caseType != nil {
								// Check if this is a complex type that needs child type generation
								caseTypeLower := strings.ToLower(caseType.TypeName)
								// DEBUG: Log CommandNode ExtraNodeData cases
								if parentName == "CommandNode" && field.Name == "extraNodeData" {
									fmt.Printf("DEBUG [CommandNode.ExtraNodeData]: Case '%s': TypeName='%s', caseTypeLower='%s', Extras=%v\n",
										caseName, caseType.TypeName, caseTypeLower, caseType.Extras != nil)
								}
								// Handle nested switch specially - switches should be inline with any type
								// TODO: Support nested switches by generating inline handling code
								if caseType.Extras != nil && caseTypeLower == "switch" {
									// Nested switch - keep as pk.Field but preserve Extras for inline generation
									// The template will need to handle this recursively
									caseType.Name = "pk.Field"
									caseType.TypeName = "pk.Field"
									// Process the nested switch's case types
									if nestedSwitch, ok := caseType.Extras.(*datatypes.Switch); ok {
										for nestedCaseName, nestedCaseType := range nestedSwitch.Fields {
											if nestedCaseType != nil {
												lookupKey := nestedCaseType.TypeName
												if lookupKey == "" {
													lookupKey = nestedCaseType.Name
												}
												nativeTypeName := toNative(lookupKey, nestedCaseType, baseTypes, isGeneratingBaseTypes)
												if nativeTypeName != lookupKey {
													if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
														if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
															nativeTypeName = "basetypes." + nativeTypeName
														}
													}
													nestedCaseType.Name = nativeTypeName
													nestedCaseType.TypeName = nativeTypeName
												} else if typeName, ok := baseTypes[strings.ToLower(lookupKey)]; ok {
													nativeCheck := toNative(typeName, nil, baseTypes, isGeneratingBaseTypes)
													if nativeCheck != typeName && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
														nestedCaseType.Name = nativeCheck
														nestedCaseType.TypeName = nativeCheck
													} else {
														typeName = toIdentifier(typeName)
														if !isGeneratingBaseTypes && !strings.Contains(typeName, ".") && typeName != "struct{}" && typeName != "models.Void" {
															typeName = "basetypes." + typeName
														}
														nestedCaseType.Name = typeName
														nestedCaseType.TypeName = typeName
													}
												} else {
													nestedCaseType.TypeName = nativeTypeName
													nestedCaseType.Name = nativeTypeName
												}
												nestedSwitch.Fields[nestedCaseName] = nestedCaseType
											}
										}
										// Process nested switch default if present
										if nestedSwitch.Default != nil {
											lookupKey := nestedSwitch.Default.TypeName
											if lookupKey == "" {
												lookupKey = nestedSwitch.Default.Name
											}
											nativeTypeName := toNative(lookupKey, nestedSwitch.Default, baseTypes, isGeneratingBaseTypes)
											if nativeTypeName != lookupKey {
												if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
													if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
														nativeTypeName = "basetypes." + nativeTypeName
													}
												}
												nestedSwitch.Default.Name = nativeTypeName
												nestedSwitch.Default.TypeName = nativeTypeName
											} else if typeName, ok := baseTypes[strings.ToLower(lookupKey)]; ok {
												nativeCheck := toNative(typeName, nil, baseTypes, isGeneratingBaseTypes)
												if nativeCheck != typeName && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
													nestedSwitch.Default.Name = nativeCheck
													nestedSwitch.Default.TypeName = nativeCheck
												} else {
													typeName = toIdentifier(typeName)
													if !isGeneratingBaseTypes && !strings.Contains(typeName, ".") && typeName != "struct{}" && typeName != "models.Void" {
														typeName = "basetypes." + typeName
													}
													nestedSwitch.Default.Name = typeName
													nestedSwitch.Default.TypeName = typeName
												}
											} else {
												nestedSwitch.Default.TypeName = nativeTypeName
												nestedSwitch.Default.Name = nativeTypeName
											}
										}
									}
									// Keep Extras for recursive switch generation in template
									fmt.Printf("DEBUG: Keeping nested switch as 'any' for field '%s' case '%s' - Extras preserved for inline generation\n", field.Name, caseName)
								} else if caseType.Extras != nil && (caseTypeLower == "" || caseTypeLower == "container" || caseTypeLower == "bitfield" || caseTypeLower == "registryentryholder" || caseTypeLower == "registryentryholderset" || caseTypeLower == "option" || caseTypeLower == "mapper" || caseTypeLower == "array") {
									// Generate a child type for this complex case
									childTypeName := toIdentifier(parentName + "_" + field.Name + "_" + caseName)
									childType := *caseType
									childType.Name = childTypeName
									if childType.Extras != nil {
										childType.Extras.SetName(childTypeName)
									}
									// Process and add the child type
									childTypes := processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)
									types = append(types, childTypes...)
									// Update the case to reference the new type
									caseType.Name = childTypeName
									caseType.TypeName = childTypeName
									// Keep Extras for mapper types so isCompareToFieldMapper() can detect them
									// For other types (containers, bitfields, etc.), we clear Extras since the fields
									// have been extracted into the generated child struct
									if caseTypeLower != "mapper" {
										caseType.Extras = nil
									}
								} else {
									// Simple type - resolve via baseTypes or toNative
									lookupKey := caseType.TypeName
									if lookupKey == "" {
										lookupKey = caseType.Name
									}
									// Handle meta-type keywords like "switch" that shouldn't be used as type names
									if strings.ToLower(lookupKey) == "switch" {
										// Switch types are inline and use pk.Field
										caseType.Name = "pk.Field"
										caseType.TypeName = "pk.Field"
										continue
									}
									if strings.Contains(lookupKey, "cookie") || strings.Contains(lookupKey, "Cookie") {
										fmt.Printf("DEBUG [switch case]: Processing case '%s' with lookupKey='%s'\n", caseName, lookupKey)
									}
									// First try to convert to native type (handles primitives like varint, i8, f32, void)
									nativeTypeName := toNative(lookupKey, caseType, baseTypes, isGeneratingBaseTypes)
									if strings.Contains(lookupKey, "cookie") || strings.Contains(lookupKey, "Cookie") {
										fmt.Printf("DEBUG [switch case]: toNative('%s') returned '%s'\n", lookupKey, nativeTypeName)
									}
									// Check if toNative actually converted it (primitives, void, etc. will be converted)
									if nativeTypeName != lookupKey {
										// toNative converted it - check if it needs basetypes prefix
										// Void is a native type defined in baseTypeDefs, don't add prefix or generate it
										if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
											// Check if this type is in baseTypes (meaning it's defined in basetypes package)
											if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
												nativeTypeName = "basetypes." + nativeTypeName
											} else if needsBaseTypesPrefix(nativeTypeName) {
												nativeTypeName = "basetypes." + nativeTypeName
											}
										}
										caseType.Name = nativeTypeName
										caseType.TypeName = nativeTypeName
									} else if typeName, ok := baseTypes[strings.ToLower(lookupKey)]; ok {
										// Found in baseTypes - check if it needs native conversion too
										nativeCheck := toNative(typeName, nil, baseTypes, isGeneratingBaseTypes)
										if nativeCheck != typeName && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
											// The basetype itself is a primitive
											caseType.Name = nativeCheck
											caseType.TypeName = nativeCheck
										} else {
											// It's a custom basetype, use identifier
											typeName = toIdentifier(typeName)
											// Don't add basetypes prefix to native types like struct{} and Void
											if !isGeneratingBaseTypes && !strings.Contains(typeName, ".") && typeName != "struct{}" && typeName != "models.Void" {
												typeName = "basetypes." + typeName
											}
											caseType.Name = typeName
											caseType.TypeName = typeName
										}
									} else {
										// Not found anywhere, use the native conversion as-is
										caseType.TypeName = nativeTypeName
										caseType.Name = nativeTypeName
									}
								}
								switchType.Fields[caseName] = caseType
							}
						}
						// Resolve default type if present
						if switchType.Default != nil {
							// Check if default is a container or other complex type that needs child type generation
							// If Extras is present, the default contains fields and needs a child type
							fmt.Printf("[DEBUG][switch default] switchType.Default.Extrass: %#v\ns", switchType.Default.Extras)
							defaultTypeLower := strings.ToLower(switchType.Default.TypeName)
							if switchType.Default.Extras != nil && (defaultTypeLower == "" || defaultTypeLower == "container" || defaultTypeLower == "registryentryholder" || defaultTypeLower == "registryentryholderset" || defaultTypeLower == "option" || defaultTypeLower == "mapper" || defaultTypeLower == "array") {
								// Generate a child type for this complex default case
								childTypeName := toIdentifier(parentName + "_" + field.Name + "_default")
								childType := *switchType.Default
								childType.Name = childTypeName
								if childType.Extras != nil {
									childType.Extras.SetName(childTypeName)
								}
								// Process and add the child type
								childTypes := processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)
								types = append(types, childTypes...)
								// Update the default to reference the new type
								switchType.Default.Name = childTypeName
								switchType.Default.TypeName = childTypeName
								switchType.Default.Extras = nil
							} else {
								// Simple type or already processed - use standard resolution
								lookupKey := switchType.Default.TypeName
								if lookupKey == "" {
									lookupKey = switchType.Default.Name
								}
								// First try to convert to native type
								nativeTypeName := toNative(lookupKey, switchType.Default, baseTypes, isGeneratingBaseTypes)
								// Check if toNative actually converted it
								if nativeTypeName != lookupKey {
									// toNative converted it - check if it needs basetypes prefix
									// Void is a native type defined in baseTypeDefs, don't add prefix
									if !isGeneratingBaseTypes && !strings.Contains(nativeTypeName, ".") && !strings.HasPrefix(nativeTypeName, "pk.") && nativeTypeName != "struct{}" && nativeTypeName != "models.Void" {
										// Check if this type is in baseTypes (meaning it's defined in basetypes package)
										if _, ok := baseTypes[strings.ToLower(nativeTypeName)]; ok {
											nativeTypeName = "basetypes." + nativeTypeName
										} else if needsBaseTypesPrefix(nativeTypeName) {
											nativeTypeName = "basetypes." + nativeTypeName
										}
									}
									switchType.Default.Name = nativeTypeName
									switchType.Default.TypeName = nativeTypeName
								} else if typeName, ok := baseTypes[strings.ToLower(lookupKey)]; ok {
									// Found in baseTypes - check if it needs native conversion
									nativeCheck := toNative(typeName, nil, baseTypes, isGeneratingBaseTypes)
									if nativeCheck != typeName && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
										switchType.Default.Name = nativeCheck
										switchType.Default.TypeName = nativeCheck
									} else {
										typeName = toIdentifier(typeName)
										// Don't add basetypes prefix to native types like struct{} and Void
										if !isGeneratingBaseTypes && !strings.Contains(typeName, ".") && typeName != "struct{}" && typeName != "models.Void" {
											typeName = "basetypes." + typeName
										}
										switchType.Default.Name = typeName
										switchType.Default.TypeName = typeName
									}
								} else {
									// Not found anywhere, use the native conversion
									switchType.Default.TypeName = nativeTypeName
									switchType.Default.Name = nativeTypeName
								}
							}
						}
					}
					continue // Don't create separate switch type
				case "mapper":
					// Mapper types within fields - generate specialized struct
					childType := createChildType(parentName, field.Name, field.Type, baseTypes, isGeneratingBaseTypes)
					field.Type.TypeName = childType.Name
					// Keep Extras so we can generate proper ReadFrom/WriteTo
					// field.Type.Extras = nil
					types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					continue
				case "registryentryholder":
					// RegistryEntryHolder types within fields - generate specialized struct
					childType := createChildType(parentName, field.Name, field.Type, baseTypes, isGeneratingBaseTypes)
					field.Type.TypeName = childType.Name
					// Keep Extras so we can generate proper ReadFrom/WriteTo
					types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					continue
				case "registryentryholderset":
					// RegistryEntryHolderSet types within fields - generate specialized struct
					childType := createChildType(parentName, field.Name, field.Type, baseTypes, isGeneratingBaseTypes)
					field.Type.TypeName = childType.Name
					// Keep Extras so we can generate proper ReadFrom/WriteTo
					types = append(types, processType(&childType, baseTypes, field.Anon, isGeneratingBaseTypes, nil)...)
					continue
				}
			}
			if field.Type.Extras == nil {
				// Handle special types that might not have been processed yet
				switch strings.ToLower(field.Type.TypeName) {
				case "option":
					// Option without extras - use generic option
					// Use ByteArray for unknown option types as it implements FieldEncoder
					if !isGeneratingBaseTypes {
						field.Type.TypeName = "basetypes."
					}
					field.Type.TypeName += "models.Option[pk.ByteArray]"

				case "array":
					// Array without extras - fallback to byte slice
					field.Type.TypeName = "[]byte"
				default:
					// Call toNative for other types
					nativeType := toNative(field.Type.TypeName, field.Type, baseTypes, isGeneratingBaseTypes)
					// Prefix basetypes for base-defined types if not generating basetypes
					if !isGeneratingBaseTypes && needsBaseTypesPrefix(nativeType) {
						nativeType = "basetypes." + nativeType
					}
					field.Type.TypeName = nativeType
				}
				// Also update the Name to match
				field.Type.Name = field.Type.TypeName
			}
		}

		// Post-processing: Ensure comparison fields are included for optional switches
		// This handles cases where a container has an optional switch that references a compareTo field
		if container != nil {
			for _, field := range container.Fields {
				if field.Type != nil && field.Type.TypeName == "pk.Field" && field.Type.Extras != nil {
					// This is a switch field (wrapped in pk.Field for optional switches)
					if sw, ok := field.Type.Extras.(*datatypes.Switch); ok && sw.CompareTo != "" && !strings.HasPrefix(sw.CompareTo, "../") {
						// Get the comparison field name
						compareFieldName := getCompareToFieldName(sw)
						if compareFieldName != "" {
							// Check if this field exists in container.Fields
							fieldExists := false
							for _, f := range container.Fields {
								if toIdentifier(f.Name) == compareFieldName {
									fieldExists = true
									break
								}
							}

							// If field doesn't exist, try to infer and add it
							if !fieldExists {
								fmt.Printf("DEBUG [comparison-field]: Container '%s' is missing comparison field '%s' for optional switch '%s'\n",
									t.Name, compareFieldName, field.Name)

								// Infer the type from the switch cases (most common type)
								// This is a heuristic - we look at the first non-void case to guess the type
								inferredType := "pk.VarInt" // Default assumption
								for _, caseType := range sw.Fields {
									if caseType != nil && caseType.TypeName != "" && caseType.TypeName != "models.Void" && caseType.TypeName != "struct{}" {
										// Use toNative to get the actual Go type
										nativeType := toNative(caseType.TypeName, caseType, baseTypes, isGeneratingBaseTypes)
										if nativeType != caseType.TypeName && nativeType != "models.Void" {
											inferredType = nativeType
											break
										}
									}
								}

								// Create a new field for the comparison
								newField := &datatypes.ContainerField{
									Name: compareFieldName,
									Type: &datatypes.Type{
										Name:     compareFieldName,
										TypeName: inferredType,
										Comment:  "// Inferred comparison field for switch '" + field.Name + "'",
									},
									Anon: false,
								}

								// Insert at the beginning of the fields (before the switch field)
								// so it's read first
								newFields := make([]*datatypes.ContainerField, 0, len(container.Fields)+1)
								newFields = append(newFields, newField)
								newFields = append(newFields, container.Fields...)
								container.Fields = newFields

								fmt.Printf("DEBUG [comparison-field]: Added inferred comparison field '%s' with type '%s'\n",
									compareFieldName, inferredType)
							}
						}
					}
				}
			}
		}

		types = append(types, t)
		// Register container for potential type alias expansion
		containerRegistry[strings.ToLower(container.GetName())] = container
	} else if array, ok := isArray(t); ok {
		fmt.Println(array.GetName())
		// types = append(types, &datatypes.Type{Name: "Array_" + array.GetName() + "_TO_DO_processType", TypeName: "[]string"})
		array := toArray(t)
		if array.Type == nil {
			return types
		}

		// Determine the element type name
		elementTypeName := ""
		// First try to resolve the element type from its TypeName
		if array.Type.TypeName != "" {
			// Check if this is a known basetype (case-insensitive lookup)
			if knownType, ok := baseTypes[strings.ToLower(array.Type.TypeName)]; ok {
				// Found in baseTypes - but still need to convert primitives like u8 → pk.UnsignedByte
				elementTypeName = toNative(knownType, nil, baseTypes, isGeneratingBaseTypes)
				if elementTypeName == knownType {
					// Not a primitive, use as identifier and add basetypes prefix if needed
					elementTypeName = toIdentifier(knownType)
					if !isGeneratingBaseTypes && !strings.Contains(elementTypeName, ".") {
						elementTypeName = "basetypes." + elementTypeName
					}
				}
				fmt.Printf("DEBUG [processType - array]: Element type '%s' found in baseTypes as '%s', converted to '%s'\n", array.Type.TypeName, knownType, elementTypeName)
			} else {
				// Convert to native or identifier
				elementTypeName = toNative(array.Type.TypeName, array.Type, baseTypes, isGeneratingBaseTypes)
				if elementTypeName == array.Type.TypeName {
					elementTypeName = toIdentifier(array.Type.TypeName)
				}
			}
		}
		// Fallback to Name if TypeName didn't resolve
		if elementTypeName == "" && array.Type.Name != "" {
			elementTypeName = toIdentifier(array.Type.Name)
		}
		// If still empty, try Extras
		if elementTypeName == "" && array.Type.Extras != nil {
			elementTypeName = toIdentifier(array.Type.Extras.GetName())
		}

		// Create a child type if the element has complex Extras that need processing
		// This includes inline containers, switches, options, etc.
		if array.Type.Extras != nil {
			// For inline complex types, use the array name to create a child type name
			childName := elementTypeName
			// If elementTypeName is struct{} or empty, derive a proper name
			if childName == "struct{}" || childName == "" || childName == "models.Void" {
				childName = t.Name + "Element"
				// Try to get a better name from Extras
				if array.Type.Extras != nil {
					extrasName := array.Type.Extras.GetName()
					if extrasName != "" && extrasName != "container" && extrasName != "array" {
						childName = toIdentifier(extrasName)
					}
				}
			}
			childType := createChildType(t.Name, childName, array.Type, baseTypes, isGeneratingBaseTypes)
			elementTypeName = childType.Name
			// Always process the child type to generate its structure
			types = append(types, processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)...)
		}

		countTypeName := "pk.VarInt"
		if array.CountType != nil {
			countTypeName = toNative(array.CountType.Name, array.CountType, baseTypes, isGeneratingBaseTypes)
		}

		t.TypeName = "models.Array[" + countTypeName + "," + elementTypeName + "]"
		t.Extras = nil
		// Append the array type alias itself
		types = append(types, t)
	} else if bitfield, ok := isBitfield(t); ok {
		fmt.Println("bitfield: ", bitfield.GetName(), "anon:", isAnon)
		// Bitfield types are generated with custom struct and methods
		// Keep the extras so the template can access field information
		types = append(types, t)
	} else if bitflags, ok := isBitflags(t); ok {
		// Preserve bitflags as a first-class type so templates can generate
		// a typed wrapper with named accessors and setters based on flags.
		// Keep TypeName as "bitflags" and retain Extras for template use.
		fmt.Printf("DEBUG [processType]: Preserving bitflags '%s' for wrapper generation\n", t.Name)
		if bitflags.GetName() == "" {
			bitflags.SetName(t.Name)
		}
		types = append(types, t)
	} else if switchType, ok := isSwitch(t); ok {
		fmt.Println("switch: ", switchType, "anon:", isAnon)
		// Switches should only exist as fields within containers
		// Standalone switches are not supported - skip generation
		return types
	} else if option, ok := isOption(t); ok {
		// Handle top-level option types
		if option.Type != nil {
			optionTypeName := toNative(option.Type.Name, option.Type, baseTypes, isGeneratingBaseTypes)
			// Replace struct{} and []byte with pk.ByteArray as they don't implement FieldEncoder
			if optionTypeName == "struct{}" || optionTypeName == "[]byte" {
				optionTypeName = "pk.ByteArray"
			}
			// If the option's inner type has complex Extras (container, array, etc.) that
			// resolved to models.Void, create a child type so it gets properly generated
			// as a struct with ReadFrom/WriteTo methods.
			if option.Type.Extras != nil && optionTypeName == "models.Void" {
				childName := "Data"
				if extrasName := option.Type.Extras.GetName(); extrasName != "" && extrasName != "container" && extrasName != "array" && extrasName != "option" {
					childName = toIdentifier(extrasName)
				}
				childType := createChildType(t.Name, childName, option.Type, baseTypes, isGeneratingBaseTypes)
				optionTypeName = childType.Name
				fmt.Printf("DEBUG [processType - option]: Created child type '%s' for complex option inner type in '%s'\n", childType.Name, t.Name)
				types = append(types, processType(&childType, baseTypes, false, isGeneratingBaseTypes, nil)...)
			}
			// Check if option inner type needs basetypes prefix
			if !isGeneratingBaseTypes && !strings.Contains(optionTypeName, ".") {
				lookupKey := strings.ToLower(optionTypeName)
				if _, ok := baseTypes[lookupKey]; ok {
					fmt.Printf("DEBUG [processType - top-level option]: Found '%s' in baseTypes, adding prefix\n", optionTypeName)
					optionTypeName = "basetypes." + optionTypeName
				} else {
					fmt.Printf("DEBUG [processType - top-level option]: Type '%s' (lookup='%s') NOT in baseTypes\n", optionTypeName, lookupKey)
				}
			}
			optionName := "models.Option"
			t.TypeName = optionName + "[" + optionTypeName + "]"
		} else {
			// Use ByteArray for unknown option types as it implements FieldEncoder
			t.TypeName = "models.Option[pk.ByteArray]"
		}
		t.Extras = nil
		types = append(types, t)
	} else if reh, ok := isRegistryEntryHolder(t); ok {
		// Handle registryEntryHolder types - they're like switches but simpler
		// They hold either a registry ID or an alternate data structure
		fmt.Printf("DEBUG [processType]: Processing registryEntryHolder '%s'\n", t.Name)

		// RegistryEntryHolder types use switch-like behavior, stored as `any`
		// Keep the Extras for template to generate proper ReadFrom/WriteTo
		t.TypeName = "registryEntryHolder" // Mark for template processing

		// Process the Otherwise type if it exists
		if reh.Otherwise.Type != nil {
			// Make sure the otherwise type is properly converted
			lookupKey := reh.Otherwise.Type.TypeName
			if lookupKey == "" {
				lookupKey = reh.Otherwise.Type.Name
			}
			// Lowercase the lookup key for baseTypes map lookup
			lookupKey = strings.ToLower(lookupKey)

			if fieldName, ok := baseTypes[lookupKey]; ok {
				fmt.Printf("DEBUG [processType]: registryEntryHolder '%s' otherwise type '%s' FOUND in baseTypes as '%s'\n",
					t.Name, lookupKey, fieldName)
				// Convert to proper identifier
				if !strings.Contains(fieldName, ".") {
					fieldName = toIdentifier(fieldName)
					if isGeneratingBaseTypes {
						reh.Otherwise.Type.Name = fieldName
						reh.Otherwise.Type.TypeName = fieldName
					} else {
						reh.Otherwise.Type.Name = "basetypes." + fieldName
						reh.Otherwise.Type.TypeName = "basetypes." + fieldName
					}
				} else {
					reh.Otherwise.Type.Name = fieldName
					reh.Otherwise.Type.TypeName = fieldName
				}
			} else {
				fmt.Printf("DEBUG [processType]: registryEntryHolder '%s' otherwise type '%s' NOT FOUND in baseTypes\n",
					t.Name, lookupKey)
				reh.Otherwise.Type.Name = toNative(reh.Otherwise.Type.Name, reh.Otherwise.Type, baseTypes, isGeneratingBaseTypes)
				if !strings.Contains(reh.Otherwise.Type.TypeName, ".") {
					reh.Otherwise.Type.TypeName = toNative(reh.Otherwise.Type.TypeName, reh.Otherwise.Type, baseTypes, isGeneratingBaseTypes)
				}
			}

			// Process nested types if the otherwise type has Extras
			if reh.Otherwise.Type.Extras != nil {
				childTypes := processType(reh.Otherwise.Type, baseTypes, false, isGeneratingBaseTypes, nil)
				types = append(types, childTypes...)
			}
		}

		types = append(types, t)
	} else if rehs, ok := isRegistryEntryHolderSet(t); ok {
		// Handle registryEntryHolderSet types - arrays/sets of registry entries
		fmt.Printf("DEBUG [processType]: Processing registryEntryHolderSet '%s'\n", t.Name)

		// RegistryEntryHolderSet types are similar to registryEntryHolder
		// Keep the Extras for template to generate proper ReadFrom/WriteTo
		t.TypeName = "registryEntryHolderSet" // Mark for template processing

		// Process the Base type if it exists
		if rehs.Base.Type != nil {
			lookupKey := rehs.Base.Type.TypeName
			if lookupKey == "" {
				lookupKey = rehs.Base.Type.Name
			}
			// Lowercase the lookup key for baseTypes map lookup
			lookupKey = strings.ToLower(lookupKey)

			if fieldName, ok := baseTypes[lookupKey]; ok {
				fmt.Printf("DEBUG [processType]: registryEntryHolderSet '%s' base type '%s' FOUND in baseTypes as '%s'\n",
					t.Name, lookupKey, fieldName)
				if !strings.Contains(fieldName, ".") {
					fieldName = toIdentifier(fieldName)
					if isGeneratingBaseTypes {
						rehs.Base.Type.Name = fieldName
						rehs.Base.Type.TypeName = fieldName
					} else {
						rehs.Base.Type.Name = "basetypes." + fieldName
						rehs.Base.Type.TypeName = "basetypes." + fieldName
					}
				} else {
					rehs.Base.Type.Name = fieldName
					rehs.Base.Type.TypeName = fieldName
				}
			} else {
				rehs.Base.Type.Name = toNative(rehs.Base.Type.Name, rehs.Base.Type, baseTypes, isGeneratingBaseTypes)
				if !strings.Contains(rehs.Base.Type.TypeName, ".") {
					rehs.Base.Type.TypeName = toNative(rehs.Base.Type.TypeName, rehs.Base.Type, baseTypes, isGeneratingBaseTypes)
				}
			}
		}

		// Process the Otherwise type if it exists
		if rehs.Otherwise.Type != nil {
			lookupKey := rehs.Otherwise.Type.TypeName
			if lookupKey == "" {
				lookupKey = rehs.Otherwise.Type.Name
			}

			// First try to convert to native type (handles primitives like varint, i8, f32)
			nativeTypeName := toNative(lookupKey, rehs.Otherwise.Type, baseTypes, isGeneratingBaseTypes)
			// Check if toNative actually converted it (primitives will be converted)
			if nativeTypeName != lookupKey && (strings.HasPrefix(nativeTypeName, "pk.") || strings.Contains(nativeTypeName, "[")) {
				// It's a native/primitive type, use the converted name
				rehs.Otherwise.Type.Name = nativeTypeName
				rehs.Otherwise.Type.TypeName = nativeTypeName
			} else if fieldName, ok := baseTypes[strings.ToLower(lookupKey)]; ok {
				// Found in baseTypes - check if it needs native conversion too
				fmt.Printf("DEBUG [processType]: registryEntryHolderSet '%s' otherwise type '%s' FOUND in baseTypes as '%s'\n",
					t.Name, lookupKey, fieldName)
				nativeCheck := toNative(fieldName, nil, baseTypes, isGeneratingBaseTypes)
				if nativeCheck != fieldName && (strings.HasPrefix(nativeCheck, "pk.") || strings.Contains(nativeCheck, "[")) {
					// The basetype itself is a primitive
					rehs.Otherwise.Type.Name = nativeCheck
					rehs.Otherwise.Type.TypeName = nativeCheck
				} else if !strings.Contains(fieldName, ".") {
					// It's a custom basetype, use identifier
					fieldName = toIdentifier(fieldName)
					if isGeneratingBaseTypes {
						rehs.Otherwise.Type.Name = fieldName
						rehs.Otherwise.Type.TypeName = fieldName
					} else {
						rehs.Otherwise.Type.Name = "basetypes." + fieldName
						rehs.Otherwise.Type.TypeName = "basetypes." + fieldName
					}
				} else {
					rehs.Otherwise.Type.Name = fieldName
					rehs.Otherwise.Type.TypeName = fieldName
				}
			} else {
				// Not found anywhere, use the native conversion as-is
				rehs.Otherwise.Type.Name = nativeTypeName
				rehs.Otherwise.Type.TypeName = nativeTypeName
			}

			// Process nested types if the otherwise type has Extras
			if rehs.Otherwise.Type.Extras != nil {
				childTypes := processType(rehs.Otherwise.Type, baseTypes, false, isGeneratingBaseTypes, nil)
				types = append(types, childTypes...)
			}
		}

		types = append(types, t)
	} else if eml, ok := isEntityMetadataLoop(t); ok {
		// Handle entityMetadataLoop types - special loop structure that reads until endVal
		fmt.Printf("DEBUG [processType]: Processing entityMetadataLoop '%s'\n", t.Name)

		// EntityMetadataLoop types use special loop logic
		// Keep the Extras for template to generate proper ReadFrom/WriteTo
		t.TypeName = "entityMetadataLoop" // Mark for template processing

		// Process the element Type if it exists
		if eml.Type != nil {
			lookupKey := eml.Type.TypeName
			if lookupKey == "" {
				lookupKey = eml.Type.Name
			}

			if fieldName, ok := baseTypes[lookupKey]; ok {
				fmt.Printf("DEBUG [processType]: entityMetadataLoop '%s' element type '%s' FOUND in baseTypes as '%s'\n",
					t.Name, lookupKey, fieldName)
				// Convert to proper identifier
				if !strings.Contains(fieldName, ".") {
					fieldName = toIdentifier(fieldName)
					if isGeneratingBaseTypes {
						eml.Type.Name = fieldName
						eml.Type.TypeName = fieldName
					} else {
						eml.Type.Name = "basetypes." + fieldName
						eml.Type.TypeName = "basetypes." + fieldName
					}
				} else {
					eml.Type.Name = fieldName
					eml.Type.TypeName = fieldName
				}
			} else {
				fmt.Printf("DEBUG [processType]: entityMetadataLoop '%s' element type '%s' NOT FOUND in baseTypes\n",
					t.Name, lookupKey)
				eml.Type.Name = toNative(eml.Type.Name, eml.Type, baseTypes, isGeneratingBaseTypes)
				if !strings.Contains(eml.Type.TypeName, ".") {
					eml.Type.TypeName = toNative(eml.Type.TypeName, eml.Type, baseTypes, isGeneratingBaseTypes)
				}
			}

			// Process nested types if the element type has Extras
			if eml.Type.Extras != nil {
				childTypes := processType(eml.Type, baseTypes, false, isGeneratingBaseTypes, nil)
				types = append(types, childTypes...)
			}
		}

		types = append(types, t)
	} else if isMapper(t) {
		// Handle top-level mapper types - keep Extras for code generation
		t.TypeName = "mapper" // Mark as mapper for template processing
		// Keep t.Extras (the *datatypes.Mapper) for ReadFrom/WriteTo generation
		types = append(types, t)
	} else if isBuffer(t) {
		// Handle top-level buffer types
		t.TypeName = "pk.ByteArray"
		t.Extras = nil
		types = append(types, t)
	}
	return types
}

// fixUnprefixedBaseTypes adds models. prefix to unprefixed Array, Bitflags references
// and basetypes. prefix to other basetype references
func fixUnprefixedBaseTypes(content string) string {
	// Note: Array, Option, Bitflags, RestBuffer, PString, UInt32, UInt64, and Void are now in models package
	// Other types like Mapper remain in basetypes package

	// Fix Array[ references - now in models package
	content = strings.ReplaceAll(content, " Array[", " models.Array[")
	content = strings.ReplaceAll(content, "\tArray[", "\tmodels.Array[")
	// Handle edge case where Array is at start of line (though unlikely)
	content = strings.ReplaceAll(content, "\nArray[", "\nmodels.Array[")
	// Handle Array inside generic type parameters (e.g., models.Option[Array[...], *Array[...]])
	content = strings.ReplaceAll(content, "[Array[", "[models.Array[")
	content = strings.ReplaceAll(content, ",Array[", ",models.Array[")
	// Handle pointer to Array in generics
	content = strings.ReplaceAll(content, "*Array[", "*models.Array[")

	// Fix Option[ references - now in models package
	content = strings.ReplaceAll(content, " Option[", " models.Option[")
	content = strings.ReplaceAll(content, "\tOption[", "\tmodels.Option[")
	content = strings.ReplaceAll(content, "\nOption[", "\nmodels.Option[")
	content = strings.ReplaceAll(content, "[Option[", "[models.Option[")
	content = strings.ReplaceAll(content, ",Option[", ",models.Option[")
	content = strings.ReplaceAll(content, "*Option[", "*models.Option[")

	// Fix Bitflags references (not Bitfield - those are custom generated types) - now in models package
	content = strings.ReplaceAll(content, " Bitflags\n", " models.Bitflags\n")
	content = strings.ReplaceAll(content, "\tBitflags\n", "\tmodels.Bitflags\n")
	content = strings.ReplaceAll(content, " Bitflags ", " models.Bitflags ")
	content = strings.ReplaceAll(content, "\tBitflags ", "\tmodels.Bitflags ")

	// Fix RestBuffer, PString, UInt32, UInt64 references - now in models package
	for _, typeName := range []string{"RestBuffer", "PString", "UInt32", "UInt64"} {
		content = strings.ReplaceAll(content, " "+typeName+"\n", " models."+typeName+"\n")
		content = strings.ReplaceAll(content, "\t"+typeName+"\n", "\tmodels."+typeName+"\n")
		content = strings.ReplaceAll(content, " "+typeName+" ", " models."+typeName+" ")
		content = strings.ReplaceAll(content, "\t"+typeName+" ", "\tmodels."+typeName+" ")
	}

	// Fix Mapper references - remains in basetypes package
	content = strings.ReplaceAll(content, " Mapper\n", " basetypes.Mapper\n")
	content = strings.ReplaceAll(content, "\tMapper\n", "\tbasetypes.Mapper\n")
	content = strings.ReplaceAll(content, " Mapper ", " basetypes.Mapper ")
	content = strings.ReplaceAll(content, "\tMapper ", "\tbasetypes.Mapper ")

	// Fix custom vector and composite types that remain in basetypes package
	// These include LpVec3, Velocity, and similar composite types from protocol
	// Only match when they are used as types (followed by newline or used in generics), not field names
	for _, typeName := range []string{"LpVec3"} {
		// Match as field type at end of line: "FieldName TypeName\n"
		content = strings.ReplaceAll(content, " "+typeName+"\n", " basetypes."+typeName+"\n")
		content = strings.ReplaceAll(content, "\t"+typeName+"\n", "\tbasetypes."+typeName+"\n")
		// Match in generic type parameters: Array[LpVec3], Option[LpVec3], etc.
		content = strings.ReplaceAll(content, "["+typeName+"]", "[basetypes."+typeName+"]")
		content = strings.ReplaceAll(content, "["+typeName+",", "[basetypes."+typeName+",")
		content = strings.ReplaceAll(content, ","+typeName+"]", ",basetypes."+typeName+"]")
		content = strings.ReplaceAll(content, ","+typeName+",", ",basetypes."+typeName+",")
		// Match pointer types: *LpVec3
		content = strings.ReplaceAll(content, "*"+typeName, "*basetypes."+typeName)
	}

	// Clean up any double-prefixes that might have been created
	content = strings.ReplaceAll(content, "basetypes.basetypes.", "basetypes.")
	content = strings.ReplaceAll(content, "models.models.", "models.")
	// Fix cases where models types were incorrectly prefixed with basetypes
	content = strings.ReplaceAll(content, "basetypes.models.", "models.")

	return content
}

// needsBaseTypesPrefix checks if a type name needs to be prefixed with "basetypes."
func needsBaseTypesPrefix(typeName string) bool {
	// Check if it's already prefixed
	if strings.HasPrefix(typeName, "basetypes.") || strings.HasPrefix(typeName, "pk.") || strings.HasPrefix(typeName, "models.") {
		return false
	}

	// Types now in models package - should NOT get basetypes prefix
	modelsTypes := []string{"Array", "Option", "Bitflags", "RestBuffer", "PString", "UInt32", "UInt64", "Void"}
	for _, mt := range modelsTypes {
		if typeName == mt || strings.HasPrefix(typeName, mt+"[") {
			return false
		}
	}

	// Explicit list of known basetypes that remain in basetypes package
	basetypeNames := []string{"Bitfield", "Mapper", "IDSet", "CommandNode", "LpVec3", "Velocity"}
	if slices.Contains(basetypeNames, typeName) {
		return true
	}

	// Check if it starts with known basetype prefixes
	if strings.HasPrefix(typeName, "Common") {
		return true
	}

	// Check for Vec types (Vec2f, Vec3i, LpVec3, etc.)
	if strings.HasPrefix(typeName, "Vec") && len(typeName) > 3 {
		return true
	}
	return false
}

func createChildType(parentName, childName string, parentType *datatypes.Type, baseTypes map[string]string, isGeneratingBaseTypes bool) datatypes.Type {
	// if child name resolves to a native type (i.e. i32 -> pk.Int), return that instead of creating a compound name
	nativeName := toNative(childName, nil, baseTypes, isGeneratingBaseTypes)
	// Check if it's a native type by seeing if toNative changed it (native types get pk. prefix or are Go builtins)
	if nativeName != childName && (strings.HasPrefix(nativeName, "pk.") || nativeName == "uint64" || nativeName == "struct{}") {
		return datatypes.Type{Name: nativeName, TypeName: nativeName}
	}

	// Also check if parentType itself is a native/simple type
	if parentType.Name == parentType.TypeName {
		return datatypes.Type{Name: nativeName, TypeName: nativeName}
	}

	// If childName already contains a package qualifier (e.g., "basetypes.SomeType"),
	// don't try to create a child type - just use it as-is
	if strings.Contains(childName, ".") {
		// This is already a qualified type reference, use it directly
		parentType.TypeName = childName
		parentType.Extras = nil
		return datatypes.Type{Name: childName, TypeName: childName}
	}

	childType := *parentType
	newName := toIdentifier(parentName + "_" + childName)
	if strings.HasPrefix(newName, "UnnamedType") {
		fmt.Printf("DEBUG [createChildType]: UNNAMED TYPE CREATED\n")
		fmt.Printf("  parentName='%s'\n", parentName)
		fmt.Printf("  childName='%s'\n", childName)
		fmt.Printf("  combined='%s'\n", parentName+"_"+childName)
		fmt.Printf("  newName='%s'\n", newName)
	}
	childType.Name = newName
	if childType.Extras != nil {
		childType.Extras.SetName(childType.Name)
	}

	parentType.TypeName = childType.Name
	// Keep Extras for mapper types so isCompareToFieldMapper() can detect them
	// For other types, we clear Extras since the fields have been extracted into the child type
	if _, isMapper := parentType.Extras.(*datatypes.Mapper); !isMapper {
		parentType.Extras = nil
	}
	return childType
}

func processNamespace(version, nsName string, namespace *namespace.Namespace, baseTypes map[string]string, packetsData inversePacketParse) error {
	// Reset per-namespace tracking maps if any (none currently)

	// file, err := os.Create(filepath.Join(version, nsName+".go"))
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	// 	return err
	// }
	// defer file.Close()
	orgNSName := nsName
	// nsName = caser.String(nsName)

	// nsMap := map[string]map[string]map[string]string{}
	for boundName, boundNamespace := range namespace.Namespaces {
		types := []*datatypes.Type{}
		switch boundName {
		case "toClient":
			boundName = "Clientbound"
		case "toServer":
			boundName = "ServerBound"
		default:
			return fmt.Errorf("unknown boundName %s", boundName)
		}

		// Build a map of types local to this namespace to avoid treating them as basetypes
		// Only include types that are actually DEFINED here (have Extras with container/mapper/etc.)
		// Don't include simple type references (like "Slot" which is defined in basetypes)
		localTypes := make(map[string]bool)
		for _, theType := range boundNamespace.Types {
			if theType.Name != "" && theType.Extras != nil {
				// This type has a definition (container, mapper, etc.), not just a reference
				typeNameLower := strings.ToLower(theType.TypeName)
				if typeNameLower == "container" || typeNameLower == "mapper" || typeNameLower == "switch" ||
					typeNameLower == "array" || typeNameLower == "bitfield" || typeNameLower == "option" ||
					typeNameLower == "registryentryholder" || typeNameLower == "registryentryholderset" {
					localTypes[strings.ToLower(theType.Name)] = true
				}
			}
		}

		// Create a modified baseTypes map that excludes local types
		effectiveBaseTypes := make(map[string]string)
		for k, v := range baseTypes {
			if !localTypes[k] {
				effectiveBaseTypes[k] = v
			}
		}

		// Extract packet ID mappings from the protocol data for this namespace/direction
		direction := strings.ToLower(boundName)
		packetIDToProtoTypeName := extractPacketIDMapFromNamespace(namespace, direction)
		fmt.Printf("DEBUG [processNamespace]: Extracted %d packet mappings for %s/%s\n", len(packetIDToProtoTypeName), orgNSName, direction)

		// Build a reverse map: protodef-go type name (packet_xxx) -> packet ID
		// This will be used to assign IDs when we detect packet types
		protoTypeNameToID := make(map[string]int32)
		for id, protoTypeName := range packetIDToProtoTypeName {
			protoTypeNameToID[protoTypeName] = id
		}

		updatedNames := map[string]string{}
		for _, theType := range boundNamespace.Types {
			// Use the protocol-derived packet ID map for this namespace/direction
			newTypes := processType(theType, effectiveBaseTypes, false, false, protoTypeNameToID) // false = not generating basetypes
			for _, t := range newTypes {
				if t.Extras != nil {
					t.Extras.SetName(t.Name)
				}
				// Update packet metadata with namespace and direction
				// (packet ID was already set by processType using the protocol-derived map)
				if meta, exists := packetMetadata[t.Name]; exists && meta.IsPacket {
					meta.Namespace = strings.ToLower(orgNSName)
					meta.Direction = direction
					packetMetadata[t.Name] = meta
					fmt.Printf("DEBUG [processNamespace]: Updated packet '%s' with namespace=%s, direction=%s, ID=0x%02X\n",
						t.Name, meta.Namespace, meta.Direction, meta.PacketID)
				}
			}
			types = append(types, newTypes...)
		}

		for _, t := range types {
			if t.Extras != nil {
				t.Extras.UpdateContainedNames(updatedNames) // this isn't working!!!! i.e. element struct was renamed to LoginClientBound_, but element wasn't :<
			}
		}
		// Use multi-file generation for namespaces (splits into packet_*.go files)
		generateMultipleTypesFiles(filepath.Join("data", version, orgNSName, boundName), version, strings.ToLower(boundName), types, false)
	}

	return nil
}

func camelOrSnakeToSpace(s string) string {
	var result strings.Builder
	nextToUpper := false

	containsPkgPrefix := strings.Contains(s, ".")
	for i, r := range s {
		if i == 0 && !containsPkgPrefix {
			result.WriteRune(unicode.ToUpper(r))
			continue
		}

		if unicode.IsUpper(r) {
			result.WriteRune(' ')
			result.WriteRune(unicode.ToUpper(r))
			nextToUpper = false
		} else if r == '_' {
			result.WriteRune(' ')
			nextToUpper = true
		} else {
			if nextToUpper {
				r = unicode.ToUpper(r)
				nextToUpper = false
			}
			result.WriteRune(r)
		}
	}

	return result.String()
}

var (
	// per-struct counters for anonymous/unknown type names; keyed by the struct being built
	unnamedTypeCounters = map[string]int{}
	// tracks which struct is currently being processed so toIdentifier can scope counters
	currentStructContext = ""
	// Global type registry to track generated type names and prevent duplicates
	// typeRegistry = make(map[string]*datatypes.Type)

	// Global packet metadata map: type name (after conversion) -> packet metadata
	packetMetadata = make(map[string]struct {
		IsPacket  bool
		PacketID  int32
		Namespace string
		Direction string
	})
)

// getMapKeys returns a slice of keys from a string->int32 map for debugging
func getMapKeys(m map[string]int32) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// structNameToProtoType converts a Go struct name back to protocol type name
// e.g., "EncryptionBegin" -> "packet_encryption_begin", "CommonCookieRequest" -> "packet_common_cookie_request"
func structNameToProtoType(structName string) string {
	// Insert underscores before uppercase letters and convert to lowercase
	var result strings.Builder
	for i, r := range structName {
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	protoName := result.String()
	// Add packet_ prefix if not already present
	if !strings.HasPrefix(protoName, "packet_") {
		protoName = "packet_" + protoName
	}
	return protoName
}

func toIdentifier(in string) string {
	// Handle empty or invalid input
	if in == "" {
		scope := currentStructContext
		if scope == "" {
			scope = "_global"
		}
		unnamedTypeCounters[scope]++
		fmt.Println("")
		return fmt.Sprintf("UnnamedType%04d", unnamedTypeCounters[scope])
	}

	// Preserve Go built-in types and keywords as-is
	if in == "struct{}" || in == "any" || in == "pk.Field" || in == "Void" || in == "int" || in == "string" || in == "bool" {
		return in
	}

	// Preserve already-formatted types like "pk.Option[...]" or "Array[...]" or "basetypes.Something"
	// These should not be processed further
	if strings.Contains(in, "[") || strings.HasPrefix(in, "pk.") || strings.HasPrefix(in, "basetypes.") || strings.HasPrefix(in, "models.") || strings.Contains(in, ".") {
		return in
	}

	// If the input already contains a package qualifier (e.g., "pk.Boolean"),
	// we need to extract just the type name for use as a struct name
	if strings.Contains(in, ".") {
		// Check if it looks like a package-qualified identifier
		parts := strings.Split(in, ".")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			// For pk.* types, return as-is since they're from external package
			if parts[0] == "pk" {
				return in
			}
			// For basetypes.*, strip the prefix and just use the type name
			// This handles cases where Type.Name was incorrectly set to "basetypes.Void"
			if parts[0] == "basetypes" {
				return parts[1]
			}
			// Other package qualifiers - return as-is
			return in
		}
	}

	out := strings.TrimPrefix(in, "packet_")
	// Replace dots, slashes, and colons with underscores to create valid Go identifiers
	// Do this BEFORE camelOrSnakeToSpace so they get converted to camelCase properly
	out = strings.ReplaceAll(out, ".", "_")
	out = strings.ReplaceAll(out, "/", "_")
	out = strings.ReplaceAll(out, ":", "_")
	out = camelOrSnakeToSpace(out)
	//out = caser.String(out)
	out = strings.ReplaceAll(out, " ", "")

	// Ensure the identifier starts with a letter or underscore
	if out != "" && !unicode.IsLetter(rune(out[0])) && out[0] != '_' {
		out = "Type" + out
	}

	// If still empty after processing, provide a unique default
	if out == "" {
		scope := currentStructContext
		if scope == "" {
			scope = "_global"
		}
		unnamedTypeCounters[scope]++
		out = fmt.Sprintf("UnnamedType%04d", unnamedTypeCounters[scope])
	}

	// fmt.Println("[toIdentifier]", in, "=>", out)
	return out
}

func toNative(name string, in *datatypes.Type, baseTypes map[string]string, isGeneratingBaseTypes bool) string {
	checkName := name
	if in != nil && in.TypeName != "" {
		checkName = in.TypeName
	}

	// If we're generating basetypes and the checkName already has the basetypes prefix, strip it
	if isGeneratingBaseTypes && strings.HasPrefix(checkName, "basetypes.") {
		checkName = strings.TrimPrefix(checkName, "basetypes.")
	}

	pkgPrefix := "basetypes."
	if isGeneratingBaseTypes {
		pkgPrefix = ""
	}

	switch checkName {
	case "u8", "U8":
		return "pk.UnsignedByte"
	case "i8":
		return "pk.Byte"
	case "i16":
		return "pk.Short"
	case "u16":
		return "pk.UnsignedShort"
	case "i32":
		return "pk.Int"
	case "u32":
		return "models.UInt32"
	case "i64":
		return "pk.Long"
	case "u64":
		return "models.UInt64"
	case "f32":
		return "pk.Float"
	case "f64":
		return "pk.Double"
	case "varint":
		return "pk.VarInt"
	case "optvarint":
		return "models.OptVarInt"
	case "varint64", "varlong":
		return "pk.VarLong"
	case "bool":
		return "pk.Boolean"
	case "uuid", "UUID":
		return "pk.UUID"
	case "string":
		return "pk.String"
	case "pstring":
		// pstring is a protocol string type
		return "pk.String"
	case "bytearray", "ByteArray":
		return "pk.ByteArray"
	case "void", "Void":
		// Void type represents no data
		return "models.Void"
	case "container":
		// Generic container type without specific structure
		return "models.Void"
	case "restBuffer":
		return "models.RestBuffer"
	case "AnonymousNbt", "anonymousNbt":
		return "models.AnonymousNBT"
	case "anonOptionalNbt", "AnonOptionalNbt":
		return "models.AnonymousNBT" // Supports all NBT tag types polymorphically (No Option wrapper, tag type indicates presence)
	case "registryEntryHolder", "RegistryEntryHolder":
		// registryEntryHolder should have been processed into a named type
		// If we reach here, it's an error in the processing pipeline
		if in != nil {
			// Prefer explicit names if present
			if in.Name != "" {
				return toIdentifier(in.Name)
			}
			if in.Extras != nil {
				if n := in.Extras.GetName(); n != "" {
					return toIdentifier(n)
				}
			}
		}
		return "RegistryEntryHolder" // fallback, but shouldn't happen
	case "registryEntryHolderSet", "RegistryEntryHolderSet":
		// Prefer a concrete generated name if available; otherwise leave generic
		if in != nil {
			if in.Name != "" && !strings.EqualFold(in.Name, "registryEntryHolderSet") {
				return toIdentifier(in.Name)
			}
			if in.Extras != nil {
				if n := in.Extras.GetName(); n != "" {
					return toIdentifier(n)
				}
			}
		}
		return "RegistryEntryHolderSet"
	case "entityMetadataLoop", "EntityMetadataLoop":
		// entityMetadataLoop is a special loop structure that reads entries until endVal
		// It should be processed as a special type with custom ReadFrom/WriteTo methods
		if in != nil && in.Extras != nil {
			// Return marker to indicate special handling needed
			return "EntityMetadataLoop"
		}
		return "EntityMetadata" // fallback
	case "topBitSetTerminatedArray", "TopBitSetTerminatedArray":
		return "models.TopBitSetTerminatedArray"
	case "lpvec3", "LpVec3", "lpVec3":
		return "models.LpVec3"
	case "option":
		// Handle option types - convert to models.Option with appropriate type parameters
		if in != nil && in.Extras != nil {
			if option, ok := in.Extras.(*datatypes.Option); ok && option.Type != nil {
				optionTypeName := toNative(option.Type.Name, option.Type, baseTypes, isGeneratingBaseTypes)
				// Replace struct{} and []byte with pk.ByteArray as they don't implement FieldEncoder
				if optionTypeName == "struct{}" || optionTypeName == "[]byte" {
					optionTypeName = "pk.ByteArray"
				}
				// Check if option inner type needs basetypes prefix
				if !isGeneratingBaseTypes && !strings.Contains(optionTypeName, ".") && !strings.HasPrefix(optionTypeName, "pk.") && optionTypeName != "pk.ByteArray" {
					if baseTypes != nil {
						if _, ok := baseTypes[strings.ToLower(optionTypeName)]; ok {
							optionTypeName = "basetypes." + optionTypeName
						}
					} else if needsBaseTypesPrefix(optionTypeName) {
						// Fallback heuristic when baseTypes map not available
						optionTypeName = "basetypes." + optionTypeName
					}
				}
				return "models.Option[" + optionTypeName + "]"
			}
		}
		// Fallback for options without type information - use ByteArray as it implements FieldEncoder
		return "models.Option[pk.ByteArray]"
	case "mapper":
		// Mappers are dynamic types - use Mapper base type
		return pkgPrefix + "Mapper"
	case "array":
		// Handle array types that weren't processed in processType
		if in != nil && in.Extras != nil {
			if array, ok := in.Extras.(*datatypes.Array); ok {
				countType := "pk.VarInt"
				if array.CountType != nil {
					countType = toNative(array.CountType.Name, array.CountType, baseTypes, isGeneratingBaseTypes)
				}
				if array.Type != nil {
					elementType := toNative(array.Type.Name, array.Type, baseTypes, isGeneratingBaseTypes)
					// Check if element type needs basetypes prefix
					if !isGeneratingBaseTypes && !strings.Contains(elementType, ".") && !strings.HasPrefix(elementType, "pk.") {
						if baseTypes != nil {
							if _, ok := baseTypes[strings.ToLower(elementType)]; ok {
								elementType = "basetypes." + elementType
							}
						} else if needsBaseTypesPrefix(elementType) {
							// Fallback heuristic when baseTypes map not available
							elementType = "basetypes." + elementType
						}
					}
					return "models.Array[" + countType + "," + elementType + "]"
				}
			}
		}
		// Fallback for arrays without proper type information
		return "[]byte"
	case "buffer":
		// Buffer types can be fixed-size or variable-length
		if in != nil && in.Extras != nil {
			if buffer, ok := in.Extras.(*datatypes.Buffer); ok {
				if buffer.Count > 0 {
					// Fixed-size buffer - use predefined FixedBufferN type if available
					if fixedType, err := models.GetFixedBufferTypeName(buffer.Count); err == nil {
						return fixedType
					}
					// No predefined type for this size - fall back to pk.ByteArray
					fmt.Printf("WARNING: No FixedBuffer type for size %d in toNative, using pk.ByteArray instead\n", buffer.Count)
				}
			}
		}
		// Variable-length buffer (with countType) - use pk.ByteArray
		return "pk.ByteArray"
	case "bitflags":
		// Bitflags can have different underlying types (u8, u16, u32, u64)
		// Check if we have the type information in the extras
		if in != nil && in.Extras != nil {
			if bitflags, ok := in.Extras.(*datatypes.Bitflags); ok && bitflags.Type != nil {
				// Map the bitflags underlying type to the appropriate Go type
				underlyingTypeName := bitflags.Type.Name
				if underlyingTypeName == "" {
					underlyingTypeName = bitflags.Type.TypeName
				}
				// Convert the underlying type to its native Go equivalent
				switch strings.ToLower(underlyingTypeName) {
				case "u8":
					return "pk.UnsignedByte"
				case "u16":
					return "pk.UnsignedShort"
				case "u32":
					return "models.UInt32"
				case "u64":
					return "models.UInt64"
				case "i8":
					return "pk.Byte"
				case "i16":
					return "pk.Short"
				case "i32":
					return "pk.Int"
				case "i64":
					return "pk.Long"
				default:
					// Unknown type, fall back to default Bitflags
					fmt.Printf("WARNING: Unknown bitflags underlying type '%s', falling back to models.Bitflags\n", underlyingTypeName)
					return "models.Bitflags"
				}
			}
		}
		// Default fallback for bitflags without type information
		return "models.Bitflags"
	case "bitfield":
		// Bitfield types are custom-generated structs based on their field definitions
		// The actual type name should already be set by processType
		// If we reach here, it means the type wasn't processed correctly
		if in != nil && in.Name != "" {
			return in.Name
		}
		// Fallback - shouldn't normally reach this
		return "struct{}"
	default:
		return toIdentifier(checkName)
	}
}

// loadPacketsJson loads and parses packets.json file
// Returns map[namespace][direction][packet_name]protocol_id
func loadPacketsJson(path string) (map[string]map[string]map[string]int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open packets.json: %w", err)
	}
	defer file.Close()

	var data map[string]map[string]map[string]struct {
		ProtocolID int64 `json:"protocol_id"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode packets.json: %w", err)
	}

	// Convert to simpler structure
	result := make(map[string]map[string]map[string]int64)
	for namespace, directions := range data {
		result[namespace] = make(map[string]map[string]int64)
		for direction, packets := range directions {
			result[namespace][direction] = make(map[string]int64)
			for packetName, packetInfo := range packets {
				result[namespace][direction][packetName] = packetInfo.ProtocolID
			}
		}
	}

	return result, nil
}

// packetJsonNameToIdentifier transforms packets.json name to Go identifier
// Examples:
//   - configuration:clientbound:update_tags -> ClientboundConfigUpdateTags
//   - login:clientbound:cookie_request -> ClientboundLoginCookieRequest
//   - play:serverbound:chat_command -> ServerboundChatCommand
func packetJsonNameToIdentifier(namespace, direction, packetName string) string {
	// Strip "minecraft:" prefix if present
	packetName = strings.TrimPrefix(packetName, "minecraft:")

	// Convert snake_case to PascalCase
	parts := strings.Split(packetName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[0:1]) + part[1:]
		}
	}
	baseName := strings.Join(parts, "")

	// Add direction prefix
	var prefix string
	if direction == "clientbound" {
		prefix = "Clientbound"
	} else {
		prefix = "Serverbound"
	}

	// Add namespace suffix for non-play namespaces
	var suffix string
	switch namespace {
	case "configuration":
		suffix = "Config"
	case "login":
		suffix = "Login"
	case "status":
		suffix = "Status"
	case "handshake":
		suffix = "Handshake"
	case "play":
		// Play namespace has no suffix
		suffix = ""
	}

	// Construct final name
	if suffix != "" {
		return prefix + suffix + baseName
	}
	return prefix + baseName
}

// enrichPacketDataWithAltNames enriches packet data with alternative names from packets.json
func enrichPacketDataWithAltNames(packetData *inversePacketParse, packetsJsonPath string) error {
	// Load packets.json
	packetsData, err := loadPacketsJson(packetsJsonPath)
	if err != nil {
		// Log warning but don't fail - packets.json is optional
		fmt.Printf("Warning: Could not load packets.json: %v\n", err)
		return nil
	}

	fmt.Printf("Enriching packet data with alternative names from packets.json\n")

	// Helper to build primary name for a packet
	buildPrimaryName := func(namespace, direction string, name string) string {
		if direction == "clientbound" {
			switch namespace {
			case "configuration":
				return "ClientboundConfig" + name
			case "login":
				return "LoginClientbound" + name
			case "status":
				return "ClientboundStatus" + name
			case "play":
				return "Clientbound" + name
			default:
				return "Clientbound" + name
			}
		} else {
			switch namespace {
			case "configuration":
				return "ServerboundConfig" + name
			case "login":
				return "LoginServerbound" + name
			case "status":
				return "ServerboundStatus" + name
			case "play":
				return "Serverbound" + name
			default:
				return "Serverbound" + name
			}
		}
	}

	// Helper to process one namespace
	processNamespace := func(namespace string, inverseMap *inversePacketMap) {
		if packetsData[namespace] == nil {
			return
		}

		// Build set of all primary names for clientbound to detect duplicates
		clientboundPrimaryNames := make(map[string]bool)
		for _, data := range inverseMap.Clientbound {
			primaryName := buildPrimaryName(namespace, "clientbound", data.Name)
			clientboundPrimaryNames[primaryName] = true
		}

		// Build set of all primary names for serverbound to detect duplicates
		serverboundPrimaryNames := make(map[string]bool)
		for _, data := range inverseMap.Serverbound {
			primaryName := buildPrimaryName(namespace, "serverbound", data.Name)
			serverboundPrimaryNames[primaryName] = true
		}

		// Process clientbound
		if packetsData[namespace]["clientbound"] != nil {
			for packetName, protocolID := range packetsData[namespace]["clientbound"] {
				key := models.ProtocolID{ID: protocolID}
				if data, exists := inverseMap.Clientbound[key]; exists {
					altName := packetJsonNameToIdentifier(namespace, "clientbound", packetName)
					// Only set alt name if it doesn't conflict with any primary name
					if !clientboundPrimaryNames[altName] {
						data.AltName = altName
						inverseMap.Clientbound[key] = data
					}
				}
			}
		}

		// Process serverbound
		if packetsData[namespace]["serverbound"] != nil {
			for packetName, protocolID := range packetsData[namespace]["serverbound"] {
				key := models.ProtocolID{ID: protocolID}
				if data, exists := inverseMap.Serverbound[key]; exists {
					altName := packetJsonNameToIdentifier(namespace, "serverbound", packetName)
					// Only set alt name if it doesn't conflict with any primary name
					if !serverboundPrimaryNames[altName] {
						data.AltName = altName
						inverseMap.Serverbound[key] = data
					}
				}
			}
		}
	}

	// Process all namespaces
	processNamespace("configuration", &packetData.Configuration)
	processNamespace("login", &packetData.Login)
	processNamespace("play", &packetData.Play)
	processNamespace("handshake", &packetData.Handshake)
	processNamespace("status", &packetData.Status)

	return nil
}
