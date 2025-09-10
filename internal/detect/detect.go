package detect

import (
	"os"
	"path/filepath"
)

// ProjectType represents the detected project type
type ProjectType struct {
	Kind           string   // "dotnet", "node"
	Frameworks     []string // For dotnet projects
	PackageManager string   // For node projects  
	BuildScripts   []string // For node projects
}

// InferProjectType attempts to detect the project type based on files in the directory
func InferProjectType(projectPath string) *ProjectType {
	// Check for .NET projects
	if hasDotNetFiles(projectPath) {
		return &ProjectType{
			Kind: "dotnet",
		}
	}
	
	// Check for Node.js projects  
	if hasNodeFiles(projectPath) {
		pt := &ProjectType{
			Kind: "node",
		}
		
		// Detect package manager
		if hasFile(projectPath, "pnpm-lock.yaml") {
			pt.PackageManager = "pnpm"
		} else if hasFile(projectPath, "yarn.lock") {
			pt.PackageManager = "yarn"
		} else {
			pt.PackageManager = "npm"
		}
		
		// Set default build script
		pt.BuildScripts = []string{"build"}
		
		return pt
	}
	
	return nil
}

// hasDotNetFiles checks for .NET project indicators
func hasDotNetFiles(projectPath string) bool {
	patterns := []string{"*.csproj", "*.fsproj", "*.vbproj", "*.sln"}
	
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(projectPath, pattern))
		if len(matches) > 0 {
			return true
		}
	}
	
	return false
}

// hasNodeFiles checks for Node.js project indicators  
func hasNodeFiles(projectPath string) bool {
	return hasFile(projectPath, "package.json")
}

// hasFile checks if a file exists in the given directory
func hasFile(projectPath, filename string) bool {
	fullPath := filepath.Join(projectPath, filename)
	_, err := os.Stat(fullPath)
	return err == nil
}

// HasAngularFiles checks for Angular-specific files
func HasAngularFiles(projectPath string) bool {
	return hasFile(projectPath, "angular.json")
}

// HasNextFiles checks for Next.js-specific files
func HasNextFiles(projectPath string) bool {
	candidates := []string{"next.config.js", "next.config.ts", "next.config.mjs"}
	for _, candidate := range candidates {
		if hasFile(projectPath, candidate) {
			return true
		}
	}
	return false
}

// HasViteFiles checks for Vite-specific files
func HasViteFiles(projectPath string) bool {
	candidates := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}
	for _, candidate := range candidates {
		if hasFile(projectPath, candidate) {
			return true
		}
	}
	return false
}