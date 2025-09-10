package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"slick-autobuild/internal/config"
	"slick-autobuild/internal/logging"
)

// ImageBuilder handles Docker image creation and pushing
type ImageBuilder struct {
	logger *logging.Logger
}

// validateDockerTag ensures the Docker tag is safe
func validateDockerTag(tag string) error {
	// Docker tags can contain lowercase and uppercase letters, digits, underscores, periods, and dashes
	// They cannot start with a period or dash and cannot contain consecutive periods
	validTagRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !validTagRegex.MatchString(tag) || strings.HasPrefix(tag, ".") || strings.HasPrefix(tag, "-") {
		return fmt.Errorf("invalid Docker tag: %s", tag)
	}
	return nil
}

// validateRepositoryName ensures the repository name is safe  
func validateRepositoryName(repo string) error {
	// Repository names can contain lowercase letters, digits, and separators
	validRepoRegex := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	if !validRepoRegex.MatchString(repo) {
		return fmt.Errorf("invalid repository name: %s", repo)
	}
	return nil
}

// NewImageBuilder creates a new Docker image builder
func NewImageBuilder(logger *logging.Logger) *ImageBuilder {
	return &ImageBuilder{
		logger: logger,
	}
}

// BuildAndPush builds a Docker image for the given project and pushes it to registries
func (ib *ImageBuilder) BuildAndPush(ctx context.Context, projectPath string, dockerConfig *config.DockerConfig, workspaceRoot string) error {
	if dockerConfig == nil || !dockerConfig.Enabled {
		return nil
	}

	// Validate repository name
	if err := validateRepositoryName(dockerConfig.Repository); err != nil {
		return fmt.Errorf("security check failed: %w", err)
	}

	workDir := filepath.Join(workspaceRoot, projectPath)
	dockerfilePath := filepath.Join(workDir, dockerConfig.Dockerfile)
	if dockerConfig.Dockerfile == "" {
		dockerfilePath = filepath.Join(workDir, "Dockerfile")
	}

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		ib.logger.Warn("Dockerfile not found, skipping Docker build", map[string]interface{}{
			"path":       projectPath,
			"dockerfile": dockerfilePath,
		})
		return nil
	}

	ib.logger.Info("starting Docker image build", map[string]interface{}{
		"path":       projectPath,
		"repository": dockerConfig.Repository,
		"tags":       dockerConfig.Tags,
	})

	// Determine tags to use
	tags := dockerConfig.Tags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	// Validate all tags
	for _, tag := range tags {
		if err := validateDockerTag(tag); err != nil {
			return fmt.Errorf("security check failed: %w", err)
		}
	}

	// Build the image with all tags
	for i, tag := range tags {
		fullTag := fmt.Sprintf("%s:%s", dockerConfig.Repository, tag)
		
		var buildArgs []string
		if i == 0 {
			// First build with tag
			buildArgs = []string{"build", "-t", fullTag, "."}
		} else {
			// Additional tags
			buildArgs = []string{"tag", fmt.Sprintf("%s:%s", dockerConfig.Repository, tags[0]), fullTag}
		}

		if i == 0 {
			// Only run build once
			// #nosec G204 - Arguments are validated and constructed from controlled data
			cmd := exec.CommandContext(ctx, "docker", buildArgs...)
			cmd.Dir = workDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("docker build failed for %s: %w", projectPath, err)
			}

			ib.logger.Info("Docker image built successfully", map[string]interface{}{
				"path": projectPath,
				"tag":  fullTag,
			})
		} else {
			// Tag additional versions
			// #nosec G204 - Arguments are validated and constructed from controlled data
			cmd := exec.CommandContext(ctx, "docker", buildArgs...)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("docker tag failed for %s: %w", fullTag, err)
			}
		}
	}

	// Push to registries if enabled
	if dockerConfig.Push {
		if err := ib.pushToRegistries(ctx, dockerConfig, projectPath); err != nil {
			return fmt.Errorf("failed to push Docker images: %w", err)
		}
	}

	return nil
}

// pushToRegistries pushes the built image to all configured registries
func (ib *ImageBuilder) pushToRegistries(ctx context.Context, dockerConfig *config.DockerConfig, projectPath string) error {
	registries := dockerConfig.Registries
	if len(registries) == 0 {
		registries = []string{"docker.io"} // Default to Docker Hub
	}

	tags := dockerConfig.Tags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	for _, registry := range registries {
		ib.logger.Info("pushing to registry", map[string]interface{}{
			"path":     projectPath,
			"registry": registry,
		})

		for _, tag := range tags {
			var fullTag string
			if registry == "docker.io" {
				// Docker Hub doesn't need registry prefix
				fullTag = fmt.Sprintf("%s:%s", dockerConfig.Repository, tag)
			} else {
				// Other registries need the registry prefix
				fullTag = fmt.Sprintf("%s/%s:%s", registry, dockerConfig.Repository, tag)
			}

			// Tag for the specific registry if not Docker Hub
			if registry != "docker.io" {
				sourceTag := fmt.Sprintf("%s:%s", dockerConfig.Repository, tag)
				// #nosec G204 - Arguments are validated and constructed from controlled data
				tagCmd := exec.CommandContext(ctx, "docker", "tag", sourceTag, fullTag)
				if err := tagCmd.Run(); err != nil {
					return fmt.Errorf("failed to tag image for registry %s: %w", registry, err)
				}
			}

			// Push the image
			// #nosec G204 - Arguments are validated and constructed from controlled data
			pushCmd := exec.CommandContext(ctx, "docker", "push", fullTag)
			pushCmd.Stdout = os.Stdout
			pushCmd.Stderr = os.Stderr

			if err := pushCmd.Run(); err != nil {
				return fmt.Errorf("failed to push %s to %s: %w", fullTag, registry, err)
			}

			ib.logger.Info("successfully pushed to registry", map[string]interface{}{
				"path":     projectPath,
				"registry": registry,
				"tag":      fullTag,
			})
		}
	}

	return nil
}

// CheckDockerAvailable verifies that Docker is available and running
func CheckDockerAvailable(ctx context.Context) error {
	// #nosec G204 - Fixed command with no user input
	cmd := exec.CommandContext(ctx, "docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not available or not running: %w", err)
	}
	return nil
}

// LoginToRegistry performs docker login to a registry if credentials are available
func LoginToRegistry(ctx context.Context, registry string, logger *logging.Logger) error {
	// Check for registry-specific environment variables
	var username, password string
	
	switch {
	case registry == "docker.io" || registry == "":
		username = os.Getenv("DOCKER_USERNAME")
		password = os.Getenv("DOCKER_PASSWORD")
	case strings.Contains(registry, "ghcr.io"):
		username = os.Getenv("GITHUB_ACTOR")
		password = os.Getenv("GITHUB_TOKEN")
	case strings.Contains(registry, "amazonaws.com"):
		// AWS ECR uses different authentication method
		return loginToECR(ctx, registry, logger)
	default:
		// Generic registry credentials
		username = os.Getenv(fmt.Sprintf("%s_USERNAME", strings.ToUpper(strings.ReplaceAll(registry, ".", "_"))))
		password = os.Getenv(fmt.Sprintf("%s_PASSWORD", strings.ToUpper(strings.ReplaceAll(registry, ".", "_"))))
	}

	if username == "" || password == "" {
		logger.Warn("no credentials found for registry, skipping login", map[string]interface{}{
			"registry": registry,
		})
		return nil
	}

	// #nosec G204 - Arguments are constructed from environment variables and validated registry names
	cmd := exec.CommandContext(ctx, "docker", "login", "-u", username, "--password-stdin", registry)
	cmd.Stdin = strings.NewReader(password)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to login to registry %s: %w", registry, err)
	}

	logger.Info("successfully logged into registry", map[string]interface{}{
		"registry": registry,
		"username": username,
	})
	
	return nil
}

// loginToECR handles AWS ECR authentication
func loginToECR(ctx context.Context, registry string, logger *logging.Logger) error {
	// Extract region from ECR URL
	parts := strings.Split(registry, ".")
	if len(parts) < 4 {
		return fmt.Errorf("invalid ECR registry format: %s", registry)
	}
	region := parts[3]

	// Use AWS CLI to get login token
	// #nosec G204 - Arguments are constructed from validated registry name
	cmd := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", region)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get ECR login token: %w", err)
	}

	// Login to ECR
	// #nosec G204 - Registry name is validated from input
	loginCmd := exec.CommandContext(ctx, "docker", "login", "--username", "AWS", "--password-stdin", registry)
	loginCmd.Stdin = strings.NewReader(string(output))
	
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login to ECR: %w", err)
	}

	logger.Info("successfully logged into ECR", map[string]interface{}{
		"registry": registry,
		"region":   region,
	})

	return nil
}