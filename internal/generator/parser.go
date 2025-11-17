package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/protodef-go/protodef-go/protocol"
	protodef "github.com/protodef-go/protodef-go/protodef"
)

const (
	baseURI      = "https://github.com/PrismarineJS/minecraft-data/raw/master/data/"
	dataPathsURI = baseURI + "dataPaths.json"
)

type versions map[string]map[string]versionPath

type versionPath struct {
	Protocol             string `json:"protocol"`
	BlockCollisionShapes string `json:"blockCollisionShapes"`
}

type jsonData struct {
	Protocols            string
	BlockCollisionShapes string
}

// type protocol struct {
// 	Status        clientServer `json:"status,omitempty"`
// 	Login         clientServer `json:"login,omitempty"`
// 	Configuration clientServer `json:"configuration,omitempty"`
// 	Play          clientServer `json:"play,omitempty"`
// 	Types         clientServer `json:"types,omitempty"`
// 	Handshaking   clientServer `json:"handshaking,omitempty"`
// }

// type clientServer struct {
// 	ToClient ToClientServer `json:"toClient,omitempty"`
// 	ToServer ToClientServer `json:"toServer,omitempty"`
// }

// type ToClientServer struct {
// 	Types types `json:"types,omitempty"`
// }

// type container struct {
// 	props []property
// }

// type property struct {
// 	Name string
// 	Type any
// }

// func (p *property) UnmarshalJSON(in []byte) error {
// 	var val any
// 	err := json.Unmarshal(in, &val)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// type types map[string]any

// func (t *types) UnmarshalJSON(in []byte) error {
// 	fmt.Printf("\ntypes::UnmarshalJSON in => %#v\n", string(in))
// 	var val any
// 	err := json.Unmarshal(in, &val)
// 	if err != nil {
// 		return err
// 	}

// 	if newVal, ok := val.(map[string]any); ok {
// 		*t = newVal

// 		updatedNewVal := map[string]any{}
// 		for k, v := range newVal {
// 			fmt.Println(k)
// 			if arr, ok := v.([]any); ok {
// 				if len(arr) > 0 {
// 					if arr[0] == "container" {
// 						props, err := parseContainer(arr)
// 						if err != nil {
// 							return err
// 						}
// 						updatedNewVal[k] = props
// 					}
// 				}
// 			}
// 		}
// 		if len(updatedNewVal) > 0 {
// 			*t = updatedNewVal
// 		}
// 	}
// 	fmt.Printf("types::UnmvarshalJSON parsed => %#v\n", t)

// 	return nil
// }

// func parseContainer(propsArr any) ([]property, error) {
// 	tmp, err := json.Marshal(propsArr)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var props []property
// 	err = json.Unmarshal(tmp, &props)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return props, nil
// }

func loadJSON(metadataDir, version2Use, version string) (jsonData, error) {
	var body []byte

	if version2Use == "" {
		return jsonData{}, fmt.Errorf("[loadJSON] invalid empty version provided")
	}

	parts := strings.Split(dataPathsURI, "/")
	dataPathsJSONPath := filepath.Join(metadataDir, parts[len(parts)-1])
	fmt.Println("dataPathsJSONPath:", dataPathsJSONPath)
	if fileInfo, err := os.Stat(dataPathsJSONPath); err == nil {
		if fileInfo.ModTime().After(time.Now().Add(time.Duration(-7*24) * time.Hour)) {
			body, err = os.ReadFile(dataPathsJSONPath)
			if err != nil {
				fmt.Println("error reading", dataPathsJSONPath, ": ", err.Error())
				body = nil
			}
		} else {
			fmt.Println(dataPathsJSONPath, "cache not expired")
		}
	} else {
		fmt.Println(dataPathsJSONPath, "err: ", err.Error())
	}

	if body == nil {
		fmt.Println("fetching", dataPathsURI)
		dataPathRes, err := http.Get(dataPathsURI)
		if err != nil {
			return jsonData{}, fmt.Errorf("could not reach version manifest: %w", err)
		}
		defer dataPathRes.Body.Close()

		body, err = io.ReadAll(dataPathRes.Body)
		if err != nil {
			return jsonData{}, fmt.Errorf("failed to read datapath response body: %s", err.Error())
		}
		{
			outFile, err := os.Create(dataPathsJSONPath)
			if err != nil {
				panic(err)
			}
			outFile.Write(body)
			outFile.Close()
		}
	}

	var versionInfo versions
	if err := json.NewDecoder(strings.NewReader(string(body))).Decode(&versionInfo); err != nil {
		return jsonData{}, fmt.Errorf("could not decode datapath JSON: %s", err.Error())
	}

	var data jsonData

	if versionDataPaths, ok := versionInfo["pc"][version2Use]; ok {
		var buf []byte
		protocolJSONPath := filepath.Join(metadataDir, version, "downloads", "protocol.json")
		if fileInfo, err := os.Stat(protocolJSONPath); err == nil {
			if fileInfo.ModTime().After(time.Now().Add(time.Duration(-7*24) * time.Hour)) {
				buf, err = os.ReadFile(protocolJSONPath)
				if err != nil {
					buf = nil
				}
			}
		}

		if buf == nil {
			protocolRes, err := http.Get(baseURI + versionDataPaths.Protocol + "/protocol.json")
			if err != nil {
				return jsonData{}, fmt.Errorf("failed to get protocol data: %s", err.Error())
			}
			defer protocolRes.Body.Close()
			if protocolRes.StatusCode != 200 {
				return jsonData{}, fmt.Errorf("failed to get protocol data: Invalid StatusCode [%d] ", protocolRes.StatusCode)
			}

			buf, err = io.ReadAll(protocolRes.Body)
			if err != nil {
				return jsonData{}, fmt.Errorf("failed to read protocol response body: %s", err.Error())
			}
			{
				os.MkdirAll(filepath.Dir(protocolJSONPath), 0750)
				outFile, err := os.Create(protocolJSONPath)
				if err != nil {
					panic(err)
				}
				outFile.Write(buf)
				outFile.Close()
			}
		}
		data.Protocols = string(buf)

		return data, nil

	} else {
		return jsonData{}, fmt.Errorf("invalid version: %s", version2Use)
	}
}

func parseProtocol(version, in string) (*protocol.Protocol, error) {
	os.MkdirAll("tmp", 0700)

	tmpFile, err := os.CreateTemp("tmp", "protocol_"+version)
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp file")
	}

	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	tmpFile.Write([]byte(in))

	data, err := protodef.ReadProtocolFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read protocol file: %s", err.Error())
	}
	// fmt.Println()
	// spew.Dump(data)
	// fmt.Println()

	return data, nil
}

func GetProtocolData(metadataDir, version2Use, version string) (*protocol.Protocol, error) {
	data, err := loadJSON(metadataDir, version2Use, version)
	if err != nil {
		return nil, err
	}

	protodef, err := parseProtocol(version, data.Protocols)
	if err != nil {
		return nil, err
	}

	return protodef, nil
}
