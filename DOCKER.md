# Docker Usage for slick-autobuild

This document explains how to use the Docker image for slick-autobuild.

## Building the Docker Image

```bash
docker build -t slick-autobuild .
```

The Docker image uses a multi-stage build:
- **Build stage**: Uses `golang:1.22` to compile the Go application
- **Runtime stage**: Uses `gcr.io/distroless/static-debian12` for a minimal, secure runtime

## Running the Docker Container

### Basic Usage

```bash
# Show version
docker run --rm slick-autobuild version

# Show help
docker run --rm slick-autobuild --help

# Plan builds (requires build.yaml in current directory)
docker run --rm -v $(pwd):/workspace -w /workspace slick-autobuild plan

# Execute builds
docker run --rm -v $(pwd):/workspace -w /workspace slick-autobuild build
```

### With Docker Socket (for Docker image building)

If your build configuration includes Docker image building, you need to mount the Docker socket:

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -w /workspace \
  slick-autobuild build
```

### Configuration File

Mount your configuration file if it's not in the current directory:

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -v /path/to/build.yaml:/workspace/build.yaml \
  -w /workspace \
  slick-autobuild build
```

## Image Details

- **Size**: ~2MB (using distroless base)
- **Architecture**: linux/amd64
- **Security**: Runs as non-root user, minimal attack surface
- **Dependencies**: Statically compiled binary with vendored dependencies

## CI/CD Integration

The Docker image is automatically built and pushed when changes are detected in:
- Go source files (`*.go`)
- Go module files (`go.mod`, `go.sum`)
- Dockerfile

Image is available at: `slickg/slick-autobuild:latest`

## Environment Variables

The tool supports the same environment variables as documented in the main README:
- `DOCKER_USERNAME` / `DOCKER_PASSWORD` - for Docker Hub authentication
- `GITHUB_ACTOR` / `GITHUB_TOKEN` - for GitHub Container Registry
- Registry-specific credentials for other container registries