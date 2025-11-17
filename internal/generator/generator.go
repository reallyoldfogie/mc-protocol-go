package generator

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	go_version "github.com/aquasecurity/go-version/pkg/version"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/reallyoldfogie/mc-protocol-go/models"
)

type packVersion struct {
	Resource int32 `json:"resource,omitempty"`
	Data     int32 `json:"data,omitempty"`
}

type versionParseObj struct {
	ID              string      `json:"id,omitempty"`
	Name            string      `json:"name,omitempty"`
	WorldVersion    uint64      `json:"world_version,omitempty"`
	SeriesID        string      `json:"series_id,omitempty"`
	ProtocolVersion uint64      `json:"protocol_version,omitempty"`
	PackVersion     packVersion `json:"pack_version,omitempty"`
	BuildTime       time.Time   `json:"build_time,omitempty"`
	JavaComponent   string      `json:"java_component,omitempty"`
	JavaVersion     int32       `json:"java_version,omitempty"`
	Stable          bool        `json:"stable,omitempty"`
	UseEditor       bool        `json:"use_editor,omitempty"`
}

type versionTemplateData struct {
	Name             string
	TargetName       string
	BlockStructName  string
	SoundStructName  string
	PacketStructName string
	VarName          string
	ProtocolVersion  uint64
	PacketData       inversePacketParse
}

// Run executes the generator with the provided configuration
func Run(cfg *Config) error {

	// Memory optimization: reduce GC overhead and set lower memory limit
	debug.SetGCPercent(cfg.Memory.GCPercent)

	// Parse memory limit
	if cfg.Memory.MemoryLimit != "" {
		limit, err := parseMemoryLimit(cfg.Memory.MemoryLimit)
		if err != nil {
			return fmt.Errorf("invalid memory limit: %w", err)
		}
		debug.SetMemoryLimit(limit)
	}

	runtime.GOMAXPROCS(cfg.Memory.MaxProcs)

	fmt.Println("Processing versions:", cfg.Versions)

	versionProtocolMap := map[string]uint64{}
	protocolMaxVersionMap := map[uint64]string{}
	packetInfo := map[string]inversePacketParse{}
	packetMetadataByVersion := make(map[string]map[string]struct {
		IsPacket  bool
		PacketID  int32
		Namespace string
		Direction string
	})

	sortedVersions := []go_version.Version{}
	errs := []error{}

	versions2Process := []string{}

	for _, version := range cfg.Versions {
		versionDir := filepath.Join(cfg.Output.DataDir, version)
		os.RemoveAll(versionDir)
		files, err := getData(cfg, version)
		if err != nil {
			err := fmt.Errorf("getData failed for version %s: %#v", version, err)
			errs = append(errs, err)
			continue
		}
		os.RemoveAll(filepath.Join(cfg.Output.DataDir, "versions", version))

		os.MkdirAll(versionDir, 0750)
		// Packet IDs are generated after protocol structs are parsed (second phase)
		generateBlocks(versionDir, version, files)
		generateSounds(versionDir, version, files)
		// Generate go.mod for this version package
		// if err := generateVersionGoMod(versionDir, version); err != nil {
		// 	errs = append(errs, fmt.Errorf("failed to generate go.mod for version %s: %w", version, err))
		// }

		// Force GC after processing each version
		runtime.GC()
		debug.FreeOSMemory()

		versionFile, err := os.Open(files["version.json"])
		if err != nil {
			err := fmt.Errorf("failed to open version.json file for version %s: %#v", version, err)
			errs = append(errs, err)
			continue
		}
		defer versionFile.Close()

		var versionObj versionParseObj
		json.NewDecoder(versionFile).Decode(&versionObj)

		versionProtocolMap[version] = versionObj.ProtocolVersion

		ver, err := go_version.Parse(version)
		if err != nil {
			fmt.Printf("failed to parse version: %s\n", err.Error())
		}
		sortedVersions = append(sortedVersions, ver)
		versions2Process = append(versions2Process, version)
		// Release file references after each version
		files = nil
	}

	// After this, the versions are properly sorted
	sort.Sort(go_version.Collection(sortedVersions))

	for _, v := range sortedVersions {
		fmt.Println(v, versionProtocolMap[v.String()])
		protocolMaxVersionMap[versionProtocolMap[v.String()]] = v.String()
	}

	for k, v := range protocolMaxVersionMap {
		fmt.Println(k, v)
	}

	for _, version := range versions2Process {
		version2Use := protocolMaxVersionMap[versionProtocolMap[version]]
		fmt.Println(version, "=>", version2Use)
		// Need to get files again for packet ID mapping
		files, err := getData(cfg, version2Use)
		if err != nil {
			errs = append(errs, fmt.Errorf("getData failed for version %s: %#v", version2Use, err))
			continue
		}
		protocolDefinitions, err := GetProtocolData(cfg.Cache.MetadataDir, version2Use, version)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get protocol data for version (%s): %s", version, err.Error()))
			continue
		}
		updatedPacketData, versionMetadata, err := generateProtocolStructs(version, protocolDefinitions, files)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate protocol structs for version (%s): %s", version, err.Error()))
		} else {
			// Store metadata for this version
			packetMetadataByVersion[version] = versionMetadata
			// Update packet data with struct names using this version's metadata
			updatePacketDataWithStructNames(&updatedPacketData, versionMetadata)
			// Enrich with alternative names from packets.json
			if packetsJsonPath, ok := files["packets.json"]; ok {
				if err := enrichPacketDataWithAltNames(&updatedPacketData, packetsJsonPath); err != nil {
					fmt.Printf("Warning: failed to enrich packet data with alt names: %v\n", err)
				}
			}
			// Store updated packet info
			packetInfo[version] = updatedPacketData
			// Generate per-version packet IDs now that we have packet data from protocol.json
			versionDir := filepath.Join(cfg.Output.DataDir, version)
			generatePacketIDs(versionDir, version, updatedPacketData)
		}

		// Generate test files
		versionDir := filepath.Join(cfg.Output.DataDir, version2Use)
		if err := generateTests(versionDir, version2Use); err != nil {
			fmt.Printf("Warning: failed to generate tests for %s: %v\n", version2Use, err)
		}

		// Release protocol data and force GC
		protocolDefinitions = nil
		runtime.GC()
		debug.FreeOSMemory()
	}
	for _, dir := range []string{"logs", "libraries"} {
		os.RemoveAll(dir)
	}

	os.MkdirAll(filepath.Join(cfg.Output.DataDir, "versions"), 0750)

	versionProtocolFile, err := os.Create(filepath.Join(cfg.Output.DataDir, "versions", "versionProtocol.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer versionProtocolFile.Close()

	blockIdFile, err := os.Create(filepath.Join(cfg.Output.DataDir, "versions", "blockMgr.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer blockIdFile.Close()

	soundIdFile, err := os.Create(filepath.Join(cfg.Output.DataDir, "versions", "soundMgr.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer soundIdFile.Close()

	versionPacketFile, err := os.Create(filepath.Join(cfg.Output.DataDir, "versions", "packetMgr.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer versionPacketFile.Close()

	versions := []versionTemplateData{}
	caser := cases.Title(language.AmericanEnglish)

	for _, version := range cfg.Versions {
		versionData := versionTemplateData{
			Name:             version,
			BlockStructName:  caser.String(versionToPkg(version)) + "Block",
			SoundStructName:  caser.String(versionToPkg(version)) + "Sound",
			PacketStructName: caser.String(versionToPkg(version)) + "Packet",
			TargetName:       versionToPkg(version),
			VarName:          versionToPkg(version),
			ProtocolVersion:  versionProtocolMap[version],
			PacketData:       packetInfo[version],
		}
		versions = append(versions, versionData)

		generatePackageMgr(versionData, cfg)
	}

	fmt.Printf("generating protocolMgr.go\n")
	if err := template.Must(template.New("").Parse(versionsProtocolTmpl)).Execute(versionProtocolFile, versionProtocolMap); err != nil {
		panic(err)
	}

	fmt.Printf("generating blockMgr.go\n")
	if err := template.Must(template.New("").Parse(versionsBlockMgrTmpl)).Execute(blockIdFile, versions); err != nil {
		panic(err)
	}

	fmt.Printf("generating soundMgr.go\n")
	if err := template.Must(template.New("").Parse(versionsSoundMgrTmpl)).Execute(soundIdFile, versions); err != nil {
		panic(err)
	}

	fmt.Printf("generating packetMgr.go\n")
	// DEBUG: Check packet data before template rendering
	for _, v := range versions {
		for id, data := range v.PacketData.Login.Clientbound {
			if id.ID < 2 {
				fmt.Printf("DEBUG [template]: %s Login.Clientbound ID %d: Name='%s', StructName='%s'\n",
					v.Name, id.ID, data.Name, data.StructName)
			}
		}
	}
	if err := template.Must(template.New("").Parse(versionsPacketMgrTmpl)).Execute(versionPacketFile, versions); err != nil {
		panic(err)
	}

	fmt.Println()
	for _, err := range errs {
		fmt.Printf("[ERROR] %s\n", err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d error(s) occurred during generation", len(errs))
	}

	return nil
}

func generatePackageMgr(templateData versionTemplateData, cfg *Config) {
	packetMgrFile, err := os.Create(filepath.Join(cfg.Output.DataDir, templateData.Name, "packetMgr.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// Status               inversePacketMap

	for protocolID, data := range templateData.PacketData.Login.Clientbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Login.Clientbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Login.Serverbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Login.Serverbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Handshake.Clientbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Handshake.Clientbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Handshake.Serverbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Handshake.Serverbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Play.Clientbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Play.Clientbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Play.Serverbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Play.Serverbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Configuration.Clientbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Configuration.Clientbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Configuration.Serverbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Configuration.Serverbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Status.Clientbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Status.Clientbound, protocolID)
		}
	}

	for protocolID, data := range templateData.PacketData.Play.Serverbound {
		if data.StructName == "Void" || data.Name == "Void" {
			delete(templateData.PacketData.Play.Serverbound, protocolID)
		}
	}

	pkgName := versionToPkg(templateData.Name)
	tmpl := strings.ReplaceAll(packetMgrTmpl, "{{PACKAGE_NAME}}", pkgName)

	if err := template.Must(template.New("").Funcs(template.FuncMap{
		"contains":   strings.Contains,
		"trimPrefix": strings.TrimPrefix,
	}).Parse(tmpl)).Execute(packetMgrFile, []versionTemplateData{templateData}); err != nil {
		panic(err)
	}

	defer packetMgrFile.Close()
}

// parseMemoryLimit parses a memory limit string like "512M" or "1G"
func parseMemoryLimit(limit string) (int64, error) {
	limit = strings.ToUpper(strings.TrimSpace(limit))

	var multiplier int64 = 1
	if strings.HasSuffix(limit, "MIB") || strings.HasSuffix(limit, "M") {
		multiplier = 1 << 20
		limit = strings.TrimSuffix(strings.TrimSuffix(limit, "MIB"), "M")
	} else if strings.HasSuffix(limit, "GIB") || strings.HasSuffix(limit, "G") {
		multiplier = 1 << 30
		limit = strings.TrimSuffix(strings.TrimSuffix(limit, "GIB"), "G")
	}

	value, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, err
	}

	return value * multiplier, nil
}

func getData(cfg *Config, version string) (map[string]string, error) {
	var err error
	files := loadCachedFiles(cfg, version)
	if files != nil {
		// Add packets.json to files map if it exists (for cached files)
		if reportsBase, ok := files["reportsBase"]; ok {
			packetsJsonPath := filepath.Join(reportsBase, "reports", "packets.json")
			if _, err := os.Stat(packetsJsonPath); err == nil {
				files["packets.json"] = packetsJsonPath
			}
		}
		return files, nil
	}

	os.RemoveAll(filepath.Join(cfg.Cache.MetadataDir, version))

	files, err = GetVersionFiles(version, cfg.Cache.MetadataDir)
	if err != nil {
		return nil, err
	}

	// extract client jar to get additional assets
	extractedClientBase := filepath.Join(cfg.Cache.MetadataDir, version, "extracted_client")
	err = os.MkdirAll(extractedClientBase, 0750)
	if err != nil {
		return nil, err
	}

	err = unzipFile(files["client.jar"], extractedClientBase)
	if err != nil {
		return nil, err
	}

	files["extractedClientBase"] = extractedClientBase

	extractedAssets := filepath.Join(extractedClientBase, "assets")
	err = filepath.WalkDir(extractedAssets, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			key := strings.TrimPrefix(path, extractedAssets+string(os.PathSeparator))
			if _, ok := files[key]; !ok {
				files[key] = path
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	extractedServerBase := filepath.Join(cfg.Cache.MetadataDir, version, "extracted_server")
	err = os.MkdirAll(extractedClientBase, 0750)
	if err != nil {
		return nil, err
	}

	err = unzipFile(files["server.jar"], extractedServerBase)
	if err != nil {
		return nil, err
	}

	files["extractedServerBase"] = extractedServerBase

	files["version.json"] = filepath.Join(extractedServerBase, "version.json")

	reportsBaseDir := ""
	if serverJar, ok := files["server.jar"]; ok {
		reportsBaseDir, err = GenerateReports(filepath.Join(cfg.Cache.MetadataDir, version), serverJar)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("failed to get server.jar")
	}
	files["reportsBase"] = reportsBaseDir
	// Add packets.json to files map for alternative name lookup
	packetsJsonPath := filepath.Join(reportsBaseDir, "reports", "packets.json")
	if _, err := os.Stat(packetsJsonPath); err == nil {
		files["packets.json"] = packetsJsonPath
	}

	excludeFiles := map[string]bool{
		// "minecraft/sounds/":   true,
		// "realms/":             true,
		// "minecraft/font/":     true,
		// "minecraft/textures/": true,
	}

	var assetIndexFileContents models.AssetIndexFileContents
	assetIndexFileContents.Load(files["asset_index.json"])
	assetFiles := assetIndexFileContents.DownloadAssets(filepath.Join(cfg.Cache.MetadataDir, version, "assets"), excludeFiles)

	for key, val := range assetFiles {
		files[key] = val
	}

	cacheFiles(cfg, version, files)
	return files, nil
}

func versionToPkg(version string) string {
	return "v" + strings.ReplaceAll(version, ".", "_")
}

const (
	blockRegistryName = "minecraft:block"
)

func generateBlocks(baseDir, version string, files map[string]string) {
	fmt.Println("generating", version, "blockid.go")
	blocks, err := getBlockInfo(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = os.MkdirAll(baseDir, 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	blockIdFile, err := os.Create(filepath.Join(baseDir, "blockid.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer blockIdFile.Close()

	pkgName := versionToPkg(version)
	fmt.Println("[generateBlocks] setting package name ", version, "=>", pkgName)
	tmpl := strings.ReplaceAll(blockTmpl, "{{PACKAGE_NAME}}", pkgName)
	if err := template.Must(template.New("").Parse(tmpl)).Execute(blockIdFile, blocks); err != nil {
		panic(err)
	}
}

func getBlockInfo(files map[string]string) ([]*models.ParseBlock, error) {
	out := make([]*models.ParseBlock, 0)
	var blockFileContent models.BlockFileContents
	enUSLang := map[string]string{}

	// enUSLangFile, err := os.Open(filepath.Join(files["extractedClientBase"], "assets", "minecraft", "lang", "en_us.json"))
	enUSLangFile, err := os.Open(files["minecraft/lang/en_us.json"])
	if err != nil {
		return nil, fmt.Errorf("failed to open en_us.json: %s", err.Error())
	}

	err = json.NewDecoder(enUSLangFile).Decode(&enUSLang)
	if err != nil {
		return nil, fmt.Errorf("failed to decode en_us.json file: %s", err.Error())
	}
	blocksFileName := filepath.Join(files["reportsBase"], "reports", "blocks.json")
	blocksFile, err := os.Open(blocksFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open minecraft/sounds.json: %s", err.Error())
	}

	json.NewDecoder(blocksFile).Decode(&blockFileContent)

	registryFilePath := filepath.Join(files["reportsBase"], "reports", "registries.json")
	registryFile, err := os.Open(registryFilePath)
	if err != nil {
		return nil, err
	}
	registries := models.Registries{}
	registries.ReadFrom(registryFile)

	caser := cases.Title(language.AmericanEnglish)

	if blockRegistry, ok := registries[blockRegistryName]; ok {
		for key, pID := range blockRegistry.Entries {
			translatedNameKey := strings.ReplaceAll(key, "minecraft:", "block.minecraft.")
			translatedKey, ok := enUSLang[translatedNameKey]
			if !ok {
				translatedKey = strings.ReplaceAll(key, "minecraft:", "")
			}

			blockName := strings.NewReplacer(".", " ", "'", " ", "_", " ").Replace(translatedKey)
			blockName = caser.String(blockName)
			// blockName = strings.Title(blockName)
			blockName = strings.ReplaceAll(blockName, " ", "")

			minStateID := int64(math.MaxInt64)
			maxStateID := int64(math.MinInt64)
			stateIDs := []uint64{}
			if block, ok := blockFileContent[key]; ok {
				for _, blockState := range block.States {
					stateIDs = append(stateIDs, uint64(blockState.ID))
					if blockState.ID < minStateID {
						minStateID = blockState.ID
					}
					if blockState.ID > int64(maxStateID) {
						maxStateID = blockState.ID
					}
				}
			}
			out = append(out, &models.ParseBlock{
				Name:        blockName,
				ID:          pID.ID,
				IDTag:       key,
				DisplayName: translatedKey,
				// 	Hardness         :,
				// 	Diggable         :,
				// 	DropIDs          :,
				NeedsTools: "",
				MinStateID: uint64(minStateID),
				MaxStateID: uint64(maxStateID),
				StateIDs:   stateIDs,
				// 	Transparent      :,
				// 	FilterLightLevel :,
				// 	EmitLightLevel   :,

			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out, nil

}

const (
	soundRegistryName = "minecraft:sound_event"
	soundTmpl         = `// Code generated by gen_data.go. DO NOT EDIT.

package {{PACKAGE_NAME}}


import (
	"github.com/reallyoldfogie/mc-protocol-go/models"
)
type Sounds struct {
	// SoundNames - map of ids to names for sounds.
	SoundNamesByID map[models.SoundID]string

	SoundSubtitlesByID map[models.SoundID]string

	SoundSubtitleKeysByID map[models.SoundID]string
}

func NewSounds() Sounds {
	return Sounds{
		SoundNamesByID: map[models.SoundID]string{ {{range .}}
			{{.ID}}: "{{.Name}}",{{end}}
		},

		SoundSubtitlesByID: map[models.SoundID]string{ {{range .}} 
			{{.ID}}: "{{.Subtitle}}",{{end}}
		},

		SoundSubtitleKeysByID: map[models.SoundID]string{ {{range .}} 
			{{.ID}}: "{{.SubtitleKey}}",{{end}}
		},
	}
}

`
)

func generateSounds(baseDir, version string, files map[string]string) {
	fmt.Println("generating", version, "soundid.go")
	sounds, err := getSoundInfo(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = os.MkdirAll(baseDir, 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(baseDir + "/soundid.go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	pkgName := versionToPkg(version)
	fmt.Println("[generateSounds] setting package name ", version, "=>", pkgName)

	if err := template.Must(template.New("").Parse(strings.ReplaceAll(soundTmpl, "{{PACKAGE_NAME}}", pkgName))).Execute(f, sounds); err != nil {
		panic(err)
	}
}

func generateVersionGoMod(baseDir, version string) error {
	goModPath := filepath.Join(baseDir, "go.mod")
	goModContent := fmt.Sprintf(`module github.com/reallyoldfogie/mc-protocol-go/data/%s

go 1.24.5

require (
	github.com/Tnze/go-mc v1.20.3-0.20240907175330-9a1f5431370e
	github.com/reallyoldfogie/mc-protocol-go/models v0.0.0
)

replace github.com/reallyoldfogie/mc-protocol-go/models => ../../models
`, version)

	return os.WriteFile(goModPath, []byte(goModContent), 0644)
}

func getSoundInfo(files map[string]string) ([]*models.Sound, error) {
	out := []*models.Sound{}
	var soundsFileContent models.SoundsFileContent
	enUSLang := map[string]string{}

	// enUSLangFile, err := os.Open(filepath.Join(files["extractedClientBase"], "assets", "minecraft", "lang", "en_us.json"))
	enUSLangFile, err := os.Open(files["minecraft/lang/en_us.json"])
	if err != nil {
		return nil, fmt.Errorf("failed to open en_us.json: %s", err.Error())
	}

	err = json.NewDecoder(enUSLangFile).Decode(&enUSLang)
	if err != nil {
		return nil, fmt.Errorf("failed to decode en_us.json file: %s", err.Error())
	}

	soundsFile, err := os.Open(files["minecraft/sounds.json"])
	if err != nil {
		return nil, fmt.Errorf("failed to open minecraft/sounds.json: %s", err.Error())
	}

	json.NewDecoder(soundsFile).Decode(&soundsFileContent)

	registryFilePath := filepath.Join(files["reportsBase"], "reports", "registries.json")
	registryFile, err := os.Open(registryFilePath)
	if err != nil {
		return nil, err
	}
	registries := models.Registries{}
	registries.ReadFrom(registryFile)

	if soundRegistry, ok := registries[soundRegistryName]; ok {
		for key, pID := range soundRegistry.Entries {
			key = strings.TrimPrefix(key, "minecraft:")
			var subTitle, subTitleKey string
			if soundData, ok := soundsFileContent[key]; ok {
				subTitleKey = soundData.Subtitle
			}
			if displayName, ok := enUSLang[subTitleKey]; ok {
				subTitle = displayName
			}

			out = append(out, &models.Sound{ID: pID.ID, Name: key, Subtitle: subTitle, SubtitleKey: subTitleKey})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out, nil
}

func unzipFile(fileName, outputDir string) error {
	fmt.Println("unzipping", fileName, "=>", outputDir)
	// Open a zip archive for reading.
	r, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %s", err)
	}
	defer r.Close()

	// Memory optimization: process files one at a time
	// Iterate through the files in the archive,
	for k, f := range r.File {
		name := f.Name
		// fmt.Printf("\tExtracting %s ...\n", f.Name)
		if strings.Contains(name, "/") {
			pathElements := strings.Split(name, "/")
			name = filepath.Join(pathElements...)
			basePath, _ := filepath.Split(name)
			if basePath != "" {
				// basePath := strings.Join(append([]string{outputDir}, pathElements[:len(pathElements)-1]...), string(os.PathSeparator))
				basePath = filepath.Join(outputDir, basePath)
				os.MkdirAll(basePath, 0750)
			}
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("impossible to open file n°%d in archine: %s", k, err)
		}
		defer rc.Close()
		// define the new file path
		newFilePath := filepath.Join(outputDir, name)

		// CASE 1 : we have a directory
		if f.FileInfo().IsDir() {
			// if we have a directory we have to create it
			err = os.MkdirAll(newFilePath, 0777)
			if err != nil {
				return fmt.Errorf("impossible to MkdirAll: %s", err)
			}
			// we can go to next iteration
			continue
		}

		// CASE 2 : we have a file
		// create new uncompressed file
		uncompressedFile, err := os.Create(newFilePath)
		if err != nil {
			return fmt.Errorf("failed to create uncompressed: %s", err)
		}
		_, err = io.Copy(uncompressedFile, rc)
		uncompressedFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file n°%d: %s", k, err)
		}

		// Periodically trigger GC during unzip to avoid memory buildup
		if k%100 == 0 {
			runtime.GC()
		}
	}
	return nil
}

func loadCachedFiles(cfg *Config, version string) map[string]string {
	fileName := getCacheFileName(cfg, version)
	files := map[string]string{}

	inFile, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("failed to open %s: %s\n", fileName, err.Error())
		return nil
	}
	defer inFile.Close()

	b, err := io.ReadAll(inFile)
	if err != nil {
		fmt.Printf("failed to read %s: %s\n", fileName, err.Error())
		return nil
	}

	err = json.Unmarshal(b, &files)
	if err != nil {
		fmt.Printf("failed to unmarshal files: %s\n", err.Error())
		return nil
	}

	err = validateCachedFiles(cfg, files)
	if err != nil {
		fmt.Printf("cache validation failed: %s\n", err.Error())
		return nil
	}

	return files
}

func cacheFiles(cfg *Config, version string, files map[string]string) {
	fileName := getCacheFileName(cfg, version)

	b, err := json.MarshalIndent(files, "", "    ")
	if err != nil {
		fmt.Printf("[ERROR] failed to marshal files map: %s\n", err.Error())
		return
	}

	outFile, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("[ERROR] failed to create file %s: %s", fileName, err.Error())
		return
	}

	defer outFile.Close()

	outFile.Write(b)
}

func getCacheFileName(cfg *Config, version string) string {
	os.MkdirAll(cfg.Cache.CacheDir, 0750)

	return filepath.Join(cfg.Cache.CacheDir, version+".json")
}

func validateCachedFiles(cfg *Config, files map[string]string) error {
	ttlDuration := time.Duration(-cfg.Cache.TTLDays*24) * time.Hour

	for _, fileName := range files {
		if fileInfo, err := os.Stat(fileName); err != nil {
			return fmt.Errorf("failed to validate cache: %s", err.Error())
		} else {
			if fileInfo.ModTime().Before(time.Now().Add(ttlDuration)) {
				return fmt.Errorf("%s file is too old, ignoring cache", fileName)
			}
		}
	}
	fmt.Println("cache validated.")
	return nil
}
