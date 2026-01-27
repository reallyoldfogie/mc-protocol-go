package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersionJars(t *testing.T) {
	tests := []struct {
		name string

		version string
	}{
		{
			name:    "1.21.1",
			version: "1.21.1",
		},
		{
			name:    "1.21.3",
			version: "1.21.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := GetVersionFiles(tt.version, "testing")
			require.NoError(t, err, "unexpected error")
			require.NotNil(t, files, "files should not be nil")

			for key, path := range files {
				fileInfo, err := os.Stat(path)
				assert.NoError(t, err)
				assert.Equal(t, key, fileInfo.Name())
			}
			// os.RemoveAll("testing")
		})
	}
}

func TestGenerateReports(t *testing.T) {
	tests := []struct {
		name       string
		outDirName string
		version    string

		wantErr bool
	}{
		{
			name:       "1.21.1 reports",
			outDirName: "testing" + string(os.PathSeparator) + "reports",
			version:    "1.21.1",
		},
		{
			name:       "1.21.3 reports",
			outDirName: "testing" + string(os.PathSeparator) + "reports",
			version:    "1.21.3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := GetVersionFiles(tt.version, "jars")
			require.NoError(t, err, "unexpected error")
			require.NotNil(t, files, "files shouldn't be nil")

			serverFile, ok := files["server.jar"]
			require.True(t, ok)
			require.NotNil(t, serverFile)

			fmt.Println("serverFile Name: ", serverFile)
			fmt.Println("PATH=", os.Getenv("PATH"))
			outputDir := filepath.Join("generated", tt.version)
			_, err = GenerateReports(outputDir, serverFile)
			defer os.RemoveAll(outputDir)

			if !tt.wantErr {
				assert.NoError(t, err, "GenerateReports() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				assert.NoError(t, err, "GenerateReports() error = %v, wantErr %v", err, tt.wantErr)
			}

			// cleanup
			for _, name := range []string{"libraries", "logs", "versions", "jars"} {
				os.RemoveAll(name)
			}
		})
	}
}
