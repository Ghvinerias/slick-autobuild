package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest describes the output of a build task.
type Manifest struct {
	Project     string `json:"project"`
	Kind        string `json:"kind"`
	Toolchain   string `json:"toolchain"`
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	BuildTimeMs int64  `json:"buildTimeMs"`
	Reused      bool   `json:"reused"`
	CreatedAt   string `json:"createdAt"`
}

// WriteManifest writes a manifest.json into the given output directory.
func WriteManifest(outDir string, m Manifest) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
