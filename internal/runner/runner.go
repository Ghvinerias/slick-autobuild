package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"slick-autobuild/internal/logging"
	"slick-autobuild/internal/planner"
)

// Options configures task execution.
type Options struct {
	Logger        *logging.Logger
	WorkspaceRoot string
}

// validateDockerImage ensures the Docker image name is safe
func validateDockerImage(image string) error {
	// Docker image names follow a specific format: [registry/]name[:tag]
	// Allow alphanumeric, dots, dashes, underscores, colons, and forward slashes
	validImageRegex := regexp.MustCompile(`^[a-zA-Z0-9._/-]+:[a-zA-Z0-9._-]+$|^[a-zA-Z0-9._/-]+$`)
	if !validImageRegex.MatchString(image) {
		return fmt.Errorf("invalid Docker image name: %s", image)
	}
	return nil
}

// validatePath ensures the path doesn't contain path traversal attempts
func validatePath(path string) error {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: path traversal detected in %s", path)
	}
	return nil
}

// RunTask executes the given task using Docker to ensure toolchain isolation.
// MVP: minimal commands, no caching yet.
func RunTask(ctx context.Context, task planner.Task, opts Options, pkgManager string, buildScripts []string) error {
	if opts.Logger == nil {
		opts.Logger = logging.New(false)
	}
	
	// Validate workspace path
	if err := validatePath(opts.WorkspaceRoot); err != nil {
		return fmt.Errorf("invalid workspace root: %w", err)
	}
	
	workDir := filepath.Join(opts.WorkspaceRoot, task.Path)
	if _, err := os.Stat(workDir); err != nil {
		return fmt.Errorf("task path missing: %s: %w", task.Path, err)
	}

	image, command := dockerSpec(task, pkgManager, buildScripts)
	
	// Validate the Docker image name for security
	if err := validateDockerImage(image); err != nil {
		return fmt.Errorf("security check failed: %w", err)
	}
	
	opts.Logger.Debug("docker run spec", map[string]interface{}{"image": image, "cmd": command})

	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", opts.WorkspaceRoot),
		"-w", filepath.ToSlash(filepath.Join("/workspace", task.Path)),
		image,
		"bash", "-lc", command,
	}
	// #nosec G204 - Docker arguments are validated and constructed from controlled data
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	return nil
}

func dockerSpec(task planner.Task, pkgManager string, buildScripts []string) (image string, command string) {
	switch task.Kind {
	case "dotnet":
		image = "mcr.microsoft.com/dotnet/sdk:" + task.Version
		// Basic restore + build
		command = "dotnet restore && dotnet build -c Release"
	case "node":
		image = "node:" + task.Version
		if pkgManager == "" {
			pkgManager = "npm"
		}
		if len(buildScripts) == 0 {
			buildScripts = []string{"build"}
		}
		// Single build script only (first) for MVP
		buildCmd := buildScripts[0]
		switch pkgManager {
		case "pnpm":
			command = fmt.Sprintf("corepack enable && pnpm install --frozen-lockfile || pnpm install && pnpm run %s", buildCmd)
		case "yarn":
			command = fmt.Sprintf("corepack enable && yarn install --frozen-lockfile || yarn install && yarn run %s", buildCmd)
		default:
			command = fmt.Sprintf("npm install && npm run %s", buildCmd)
		}
	default:
		image = "alpine:latest"
		command = "echo unsupported task kind"
	}
	return
}
