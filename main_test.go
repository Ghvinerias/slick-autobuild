package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"slick-autobuild/internal/cache"
	"slick-autobuild/internal/config"
	"slick-autobuild/internal/detect"
	"slick-autobuild/internal/planner"
)

func TestConfigLoad(t *testing.T) {
	// Test loading the existing build.yaml
	cfg, err := config.Load("build.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if len(cfg.Matrix) == 0 {
		t.Error("Expected matrix entries, got none")
	}
	
	if len(cfg.Runtime.Dotnet.Versions) == 0 {
		t.Error("Expected dotnet versions, got none")
	}
	
	if len(cfg.Runtime.Node.Versions) == 0 {
		t.Error("Expected node versions, got none")
	}
}

func TestPlannerExpand(t *testing.T) {
	cfg := &config.Root{
		Runtime: config.RuntimeConfig{
			Dotnet: config.VersionSet{Versions: []string{"6.0.415"}},
			Node:   config.VersionSet{Versions: []string{"18.20.2"}},
		},
		Matrix: []config.MatrixEntry{
			{Path: "test/api", Type: "dotnet"},
			{Path: "test/web", Type: "node"},
		},
	}
	
	plan := planner.Expand(cfg, map[string]struct{}{})
	
	if len(plan.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(plan.Tasks))
	}
	
	// Check first task
	if plan.Tasks[0].Kind != "dotnet" || plan.Tasks[0].Version != "6.0.415" {
		t.Errorf("First task incorrect: %+v", plan.Tasks[0])
	}
	
	// Check second task  
	if plan.Tasks[1].Kind != "node" || plan.Tasks[1].Version != "18.20.2" {
		t.Errorf("Second task incorrect: %+v", plan.Tasks[1])
	}
}

func TestCacheKey(t *testing.T) {
	task := planner.Task{
		Path:    "test/project",
		Kind:    "node",
		Version: "18.20.2",
	}
	
	// Create temporary workspace
	tmpDir := t.TempDir()
	
	key1, err := cache.Key(task, tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}
	
	key2, err := cache.Key(task, tmpDir)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}
	
	// Keys should be consistent
	if key1 != key2 {
		t.Errorf("Cache keys should be consistent: %s != %s", key1, key2)
	}
	
	if len(key1) != 12 {
		t.Errorf("Cache key should be 12 characters, got %d", len(key1))
	}
}

func TestDetectProjectType(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()
	
	// Test .NET project detection
	dotnetDir := filepath.Join(tmpDir, "dotnet-project")
	if err := os.MkdirAll(dotnetDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create a dummy .csproj file
	csprojPath := filepath.Join(dotnetDir, "test.csproj")
	if err := os.WriteFile(csprojPath, []byte("<Project></Project>"), 0644); err != nil {
		t.Fatal(err)
	}
	
	dotnetProject := detect.InferProjectType(dotnetDir)
	if dotnetProject == nil || dotnetProject.Kind != "dotnet" {
		t.Errorf("Expected dotnet project detection, got %+v", dotnetProject)
	}
	
	// Test Node.js project detection
	nodeDir := filepath.Join(tmpDir, "node-project")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create a dummy package.json file
	packageJsonPath := filepath.Join(nodeDir, "package.json")
	if err := os.WriteFile(packageJsonPath, []byte(`{"name": "test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	
	nodeProject := detect.InferProjectType(nodeDir)
	if nodeProject == nil || nodeProject.Kind != "node" {
		t.Errorf("Expected node project detection, got %+v", nodeProject)
	}
	
	if nodeProject.PackageManager != "npm" {
		t.Errorf("Expected npm package manager, got %s", nodeProject.PackageManager)
	}
}

func TestParseOnly(t *testing.T) {
	// Test empty selection
	result := parseOnlyHelper("")
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %+v", result)
	}
	
	// Test single selection
	result = parseOnlyHelper("project1")
	if len(result) != 1 || result["project1"] != struct{}{} {
		t.Errorf("Expected single project selection, got %+v", result)
	}
	
	// Test multiple selections
	result = parseOnlyHelper("project1,project2, project3")
	if len(result) != 3 {
		t.Errorf("Expected 3 project selections, got %d", len(result))
	}
	
	for _, project := range []string{"project1", "project2", "project3"} {
		if _, ok := result[project]; !ok {
			t.Errorf("Expected project %s to be selected", project)
		}
	}
}

// Helper function to test parseOnly without flag dependency
func parseOnlyHelper(only string) map[string]struct{} {
	m := map[string]struct{}{}
	if only == "" {
		return m
	}
	for _, p := range strings.Split(only, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = struct{}{}
		}
	}
	return m
}