# Slick AutoBuild

A Go CLI tool for reproducibly building multiple .NET and Node.js projects with different runtime versions.

## Features

- ✅ **Multi-runtime builds**: Support for .NET (6.0, 8.0+) and Node.js (18+, 20+) projects
- ✅ **Matrix builds**: Build projects against multiple framework/runtime versions  
- ✅ **Docker isolation**: Uses Docker images for consistent, clean builds
- ✅ **Docker image packaging**: Build and push Docker images to registries
- ✅ **Caching system**: Hash-based artifact caching to avoid redundant builds
- ✅ **Concurrent builds**: Configurable concurrency with worker pools
- ✅ **Project detection**: Auto-detect project types from files (*.csproj, package.json)
- ✅ **Registry support**: Push to Docker Hub, GitHub Container Registry, AWS ECR, Azure CR
- ✅ **Structured logging**: JSON or human-readable output
- ✅ **Build artifacts**: Versioned outputs with manifest.json metadata

## Installation

```bash
# Build from source
git clone https://github.com/Ghvinerias/learning-golang
cd learning-golang/slick-autobuild
make build-linux  # or build-win for Windows

# Use the binary
./build/linux/slick-autobuild version
```

## Quick Start

1. Create a `build.yaml` configuration:

```yaml
runtime:
  dotnet:
    versions:
      - 6.0.415
      - 8.0.100
  node:
    versions:
      - 18.20.2
      - 20.11.1

matrix:
  - path: services/api
    type: dotnet
    frameworks:
      - 6.0.415
  - path: frontend
    type: node
    packageManager: pnpm
    nodeVersions:
      - 20.11.1

defaults:
  concurrency: 4
  artifactDir: dist
```

## Docker Image Packaging

Slick-AutoBuild can build and push Docker images to popular registries after successful builds.

### Configuration

Add Docker configuration to your matrix entries:

```yaml
matrix:
  - path: services/api
    type: dotnet
    frameworks:
      - 8.0.100
    docker:
      enabled: true
      repository: "myorg/api"
      tags: ["latest", "v1.0.0", "stable"]
      push: true
      registries: ["docker.io", "ghcr.io"]
      dockerfile: "Dockerfile"  # Optional, defaults to "Dockerfile"
```

### Supported Registries

- **Docker Hub**: `docker.io` (default)
- **GitHub Container Registry**: `ghcr.io`
- **AWS Elastic Container Registry**: `123456789.dkr.ecr.us-west-2.amazonaws.com`
- **Azure Container Registry**: `myregistry.azurecr.io`
- **Google Container Registry**: `gcr.io`

### Authentication

Set environment variables for registry authentication:

```bash
# Docker Hub
export DOCKER_USERNAME=myusername
export DOCKER_PASSWORD=mypassword

# GitHub Container Registry  
export GITHUB_ACTOR=myusername
export GITHUB_TOKEN=ghp_token

# AWS ECR (requires AWS CLI configured)
aws configure

# Generic registries
export MYREGISTRY_AZURECR_IO_USERNAME=myusername
export MYREGISTRY_AZURECR_IO_PASSWORD=mypassword
```

### Docker Build Process

1. After successful project build, check if Docker is enabled
2. Look for Dockerfile in project directory
3. Build Docker image with specified tags
4. Push to configured registries (if push: true)

### CLI Options for Docker

- `--no-docker` - Disable Docker image building
- `--push-images` - Force push images (overrides config push: false)

2. Plan your builds:

```bash
./slick-autobuild plan
```

3. Execute builds:

```bash
./slick-autobuild build
```

## Commands

- `build` - Execute builds (default command)
- `plan` - Show build matrix without executing
- `clean` - Remove cache and output directories
- `inspect <key>` - Show manifest for cache key
- `version` - Display tool version

## CLI Options

- `--config build.yaml` - Configuration file path
- `--concurrency N` - Max concurrent builds (default: CPU cores)
- `--json` - JSON logging output
- `--no-cache` - Disable build cache
- `--only path1,path2` - Build only specific projects
- `--dry-run` - Plan only, don't execute
- `--no-docker` - Disable Docker image building
- `--push-images` - Force push Docker images (overrides config)

## Project Detection

The tool automatically detects project types:

- `*.csproj, *.fsproj, *.vbproj, *.sln` → .NET projects
- `package.json` → Node.js projects
- `angular.json` → Angular projects
- `next.config.*` → Next.js projects  
- `vite.config.*` → Vite projects

## Docker Requirements

Ensure Docker is installed and running. The tool uses these images:

- .NET: `mcr.microsoft.com/dotnet/sdk:<version>`
- Node.js: `node:<version>` (with corepack for pnpm/yarn)

## Build Artifacts

Outputs are stored in `./out/<project>/<tool-version>/` with a `manifest.json`:

```json
{
  "project": "services/api",
  "kind": "dotnet", 
  "toolchain": "dotnet",
  "version": "6.0.415",
  "hash": "a1b2c3d4e5f6",
  "buildTimeMs": 12345,
  "reused": false,
  "createdAt": "2025-01-15T10:30:00Z"
}
```

## Caching

Build cache is stored in `.buildcache/<key>/` where key is generated from:

- Toolchain version
- Project path
- Lock files (package-lock.json, packages.lock.json, yarn.lock, pnpm-lock.yaml)
- Project files (*.csproj, package.json)

## Error Codes

- `0` - Success
- `1` - Build failure  
- `2` - Configuration error
- `3` - Internal error

## Examples

See the `examples/` directory for sample projects and configurations.

