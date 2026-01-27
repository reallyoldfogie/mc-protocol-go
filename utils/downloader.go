package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/reallyoldfogie/mc-protocol-go/models"
)

const (
	versionManifestV2 = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
)

func GetVersionMetadata(mcVersion string, outDirName string) (*models.VersionMetaData, *os.File, error) {
	metadataPath := outDirName + string(os.PathSeparator) + "metadata.json"

	// Check if metadata file already exists
	if _, err := os.Stat(metadataPath); err == nil {
		// File exists, read and parse it
		metadataFile, err := os.Open(metadataPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open existing metadata.json: %s", err.Error())
		}
		var versionMetadata models.VersionMetaData
		if err := json.NewDecoder(metadataFile).Decode(&versionMetadata); err != nil {
			metadataFile.Close()
			return nil, nil, fmt.Errorf("failed to decode existing metadata.json: %w", err)
		}
		// Reopen for consistency with creation path
		metadataFile.Close()
		metadataFile, err = os.Open(metadataPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to reopen metadata.json: %s", err.Error())
		}
		fmt.Printf("Metadata file %s already exists, skipping download\n", metadataPath)
		return &versionMetadata, metadataFile, nil
	}

	var manifest struct {
		Latest struct {
			Release string `json:"release"`
		} `json:"latest"`
		Versions []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"versions"`
	}

	manifestRes, err := http.Get(versionManifestV2)
	if err != nil {
		return nil, nil, fmt.Errorf("could not reach version manifest: %w", err)
	}
	defer manifestRes.Body.Close()

	if err := json.NewDecoder(manifestRes.Body).Decode(&manifest); err != nil {
		return nil, nil, fmt.Errorf("could not decode manifest JSON: %w", err)
	}

	if strings.ToLower(mcVersion) == "latest" {
		mcVersion = manifest.Latest.Release
	}

	var versionURL string
	for _, v := range manifest.Versions {
		if mcVersion == v.ID {
			versionURL = v.URL
			break
		}

	}
	if versionURL == "" {
		return nil, nil, errors.New("could not determine versionURL")
	}

	var versionMetadata models.VersionMetaData
	versionRes, err := http.Get(versionURL)
	if err != nil {
		return nil, nil, fmt.Errorf("could not reach versionURL: %w", err)
	}
	defer versionRes.Body.Close()

	bodyBytes, err := io.ReadAll(versionRes.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %s", err.Error())
	}

	metadataFile, err := os.Create(metadataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metadata.json file: %s", err.Error())
	}

	versionRes.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if err := json.NewDecoder(versionRes.Body).Decode(&versionMetadata); err != nil {
		return nil, nil, fmt.Errorf("could not decode version JSON: %w", err)
	}

	return &versionMetadata, metadataFile, nil
}

func GetVersionFiles(mcVersion, outDirName string) (files map[string]string, err error) {
	files = map[string]string{}
	outDirName = strings.Join([]string{outDirName, mcVersion, "downloads"}, string(os.PathSeparator))
	err = os.MkdirAll(outDirName, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory structure (%s): %s", outDirName, err.Error())
	}

	versionMetadata, metadataFile, err := GetVersionMetadata(mcVersion, outDirName)
	if err != nil {
		return nil, fmt.Errorf("failed to get version metadata: %s", err.Error())
	}

	files["metadata.json"] = metadataFile.Name()

	clientFile, err := downloadFile(versionMetadata.Downloads.Client.URL, outDirName+string(os.PathSeparator)+"client.jar")
	if err != nil {
		return nil, fmt.Errorf("failed to download client.jar")
	}
	defer clientFile.Close()
	files["client.jar"] = clientFile.Name()

	serverFile, err := downloadFile(versionMetadata.Downloads.Server.URL, outDirName+string(os.PathSeparator)+"server.jar")
	if err != nil {
		return nil, fmt.Errorf("failed to download server.jar")
	}
	defer serverFile.Close()

	files["server.jar"] = serverFile.Name()

	assetIndex, err := downloadFile(versionMetadata.AssetIndex.URL, outDirName+string(os.PathSeparator)+"asset_index.json")
	if err != nil {
		files = nil
		return nil, fmt.Errorf("failed to download server.jar")
	}
	defer assetIndex.Close()

	files["asset_index.json"] = assetIndex.Name()
	return files, nil
}

func downloadFile(uri string, targetName string) (outFile *os.File, err error) {
	// Check if file already exists
	if _, err := os.Stat(targetName); err == nil {
		// File exists, open and return it
		outFile, err = os.Open(targetName)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing file %s: %s", targetName, err.Error())
		}
		fmt.Printf("File %s already exists, skipping download\n", targetName)
		return outFile, nil
	}

	serverResp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get url (%s): %s", uri, err.Error())
	}

	defer serverResp.Body.Close()

	outFile, err = os.Create(targetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %s", targetName, err.Error())
	}

	_, err = io.Copy(outFile, serverResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to copy response to file: %s", err.Error())
	}
	return outFile, nil
}

func GenerateReports(outDirName, serverFileName string) (baseDir string, err error) {
	javaCmd := exec.Command("which", "java")
	var javaOut strings.Builder
	javaCmd.Stdout = &javaOut
	err = javaCmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute `which java` command (%s) to generate reports:%s", javaCmd.String(), err.Error())
	}
	javaExe := strings.Trim(javaOut.String(), "\n")
	fmt.Printf("which java Cmd output: %q\n", javaExe)

	err = os.MkdirAll(outDirName, 0750)
	if err != nil {
		return "", fmt.Errorf("failed to create directory structure (%s): %s", outDirName, err.Error())
	}

	baseDir = filepath.Join(outDirName, "data_generator")
	cmd := exec.Command(javaExe, "-DbundlerMainClass=net.minecraft.data.Main", "-jar", serverFileName, "--reports", "--server", "--output", baseDir)

	var out strings.Builder
	var errOut strings.Builder

	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()

	fmt.Printf("cmd output: %q\n", out.String())
	fmt.Printf("stdErr: %q\n", errOut.String())

	if err != nil {
		return "", fmt.Errorf("failed to execute java command (%s) to generate reports:%s", cmd.String(), err.Error())
	}
	return baseDir, nil
}
