package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TestGenerationData holds the data needed to generate test files for a specific version
type TestGenerationData struct {
	Version           string
	PackageName       string
	HasStepTick       bool
	HasEntityTeleport bool
}

// generateTests generates test files for a version
func generateTests(baseDir, version string) error {
	pkgName := versionToPkg(version)

	// Determine which types are available in this version
	testData := TestGenerationData{
		Version:           version,
		PackageName:       pkgName,
		HasStepTick:       hasType(baseDir, "play/clientbound", "StepTick"),
		HasEntityTeleport: hasType(baseDir, "play/clientbound", "EntityTeleport"),
	}

	// Generate bitfield test
	if err := generateBitfieldTest(baseDir, testData); err != nil {
		return fmt.Errorf("failed to generate bitfield test: %w", err)
	}

	// Generate integration test
	if err := generateIntegrationTest(baseDir, testData); err != nil {
		return fmt.Errorf("failed to generate integration test: %w", err)
	}

	return nil
}

func generateBitfieldTest(baseDir string, data TestGenerationData) error {
	testFile, err := os.Create(filepath.Join(baseDir, "bitfield_test.go"))
	if err != nil {
		return err
	}
	defer testFile.Close()

	tmpl := template.Must(template.New("bitfield").Parse(bitfieldTestTemplate))
	return tmpl.Execute(testFile, data)
}

func generateIntegrationTest(baseDir string, data TestGenerationData) error {
	testFile, err := os.Create(filepath.Join(baseDir, "integration_test.go"))
	if err != nil {
		return err
	}
	defer testFile.Close()

	tmpl := template.Must(template.New("integration").Parse(integrationTestTemplate))
	return tmpl.Execute(testFile, data)
}

// hasType checks if a type exists in the generated code
func hasType(baseDir, subpackage, typeName string) bool {
	// Check if the file exists and contains the type
	file := filepath.Join(baseDir, subpackage, "types.go")
	content, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "type "+typeName+" struct")
}
