package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Root is the top-level configuration structure for the build tool.
type Root struct {
	Runtime RuntimeConfig   `yaml:"runtime"`
	Matrix  []MatrixEntry   `yaml:"matrix"`
	Defaults DefaultSection `yaml:"defaults"`
}

type RuntimeConfig struct {
	Dotnet VersionSet `yaml:"dotnet"`
	Node   VersionSet `yaml:"node"`
}

type VersionSet struct {
	Versions []string `yaml:"versions"`
}

type MatrixEntry struct {
	Path          string   `yaml:"path"`
	Type          string   `yaml:"type"`
	Frameworks    []string `yaml:"frameworks"` // dotnet specific (SDK versions override)
	NodeVersions  []string `yaml:"nodeVersions"`
	PackageManager string  `yaml:"packageManager"`
	BuildScripts  []string `yaml:"buildScripts"`
	Docker        *DockerConfig `yaml:"docker,omitempty"`
}

type DockerConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Repository string   `yaml:"repository"`
	Tags       []string `yaml:"tags"`
	Push       bool     `yaml:"push"`
	Registries []string `yaml:"registries"`
	Dockerfile string   `yaml:"dockerfile"`
}

type DefaultSection struct {
	Concurrency int    `yaml:"concurrency"`
	ArtifactDir string `yaml:"artifactDir"`
}

// validatePath ensures the path is safe and doesn't contain path traversal attempts
func validatePath(path string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
		return fmt.Errorf("invalid path: path traversal detected in %s", path)
	}
	
	return nil
}

// Load reads a YAML config file.
func Load(path string) (*Root, error) {
	// Validate the config file path
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}
	
	// #nosec G304 - Path is validated above to prevent traversal attacks
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Root
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return &r, nil
}
