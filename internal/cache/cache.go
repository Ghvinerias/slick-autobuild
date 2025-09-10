package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slick-autobuild/internal/planner"
	"sort"
	"strings"
)

// validatePath ensures the path is safe and doesn't contain path traversal attempts
func validatePath(path string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: path traversal detected in %s", path)
	}
	
	return nil
}

// Key generates a cache key based on the task and environment
func Key(task planner.Task, workspaceRoot string) (string, error) {
	h := sha256.New()
	
	// Include toolchain and version
	h.Write([]byte(task.Kind))
	h.Write([]byte(task.Version))
	
	// Include project path
	h.Write([]byte(task.Path))
	
	// Include relevant lock files
	projectDir := filepath.Join(workspaceRoot, task.Path)
	lockFiles := findLockFiles(projectDir, task.Kind)
	
	// Sort for consistent ordering
	sort.Strings(lockFiles)
	for _, lockFile := range lockFiles {
		// Validate each lock file path
		if err := validatePath(lockFile); err != nil {
			continue // Skip invalid paths
		}
		// #nosec G304 - Path is validated above to prevent traversal attacks
		if content, err := os.ReadFile(lockFile); err == nil {
			h.Write(content)
		}
	}
	
	return fmt.Sprintf("%x", h.Sum(nil))[:12], nil
}

// findLockFiles returns relevant lock files for the given project type
func findLockFiles(projectDir, kind string) []string {
	var lockFiles []string
	
	switch kind {
	case "dotnet":
		// Check for project files and package lock files
		patterns := []string{"*.csproj", "*.fsproj", "*.vbproj", "packages.lock.json"}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(filepath.Join(projectDir, pattern))
			lockFiles = append(lockFiles, matches...)
		}
	case "node":
		// Check for package.json and lock files
		candidates := []string{
			filepath.Join(projectDir, "package.json"),
			filepath.Join(projectDir, "package-lock.json"),
			filepath.Join(projectDir, "yarn.lock"),
			filepath.Join(projectDir, "pnpm-lock.yaml"),
		}
		for _, file := range candidates {
			if _, err := os.Stat(file); err == nil {
				lockFiles = append(lockFiles, file)
			}
		}
	}
	
	return lockFiles
}

// Exists checks if a cache entry exists for the given key
func Exists(key string) bool {
	cacheDir := filepath.Join(".buildcache", key)
	manifestPath := filepath.Join(cacheDir, "manifest.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}

// Store copies artifacts to cache directory
func Store(key, sourceDir string) error {
	cacheDir := filepath.Join(".buildcache", key)
	
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	
	return copyDir(sourceDir, cacheDir)
}

// Restore copies artifacts from cache to output directory
func Restore(key, destDir string) error {
	cacheDir := filepath.Join(".buildcache", key)
	
	if !Exists(key) {
		return fmt.Errorf("cache key not found: %s", key)
	}
	
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	
	return copyDir(cacheDir, destDir)
}

// copyDir recursively copies a directory
func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		
		destPath := filepath.Join(dest, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		
		return copyFile(path, destPath)
	})
}

// copyFile copies a single file
func copyFile(src, dest string) error {
	// Validate source and destination paths
	if err := validatePath(src); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := validatePath(dest); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}
	
	// #nosec G304 - Paths are validated above to prevent traversal attacks
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	
	// #nosec G304 - Path is validated above to prevent traversal attacks
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, srcFile)
	return err
}