# Multi-Runtime Build Tool MVP

## Goal
A Go CLI that reproducibly builds:
- Multiple .NET (Core/SDK) versions
- Multiple JS/TS project types (npm, pnpm, yarn, turbo, next, vite, angular, react, vue, svelte)
- Produces versioned, cacheable build artifacts

## Primary Use Cases (Initial)
1. Given a config file, build listed projects against specified runtime/framework versions.
2. Matrix builds: (project X framework version Y).
3. Consistent artifact outputs (zipped or directory) with manifest.
4. Runs locally first; later CI-friendly.

## Out of Scope (MVP)
- Dependency vulnerability scanning
- Test execution
- Container image building
- Distributed build farm

## High-Level Architecture
```
main.go (main entry point)
internal/
  config (parse YAML/JSON)
  planner (expand matrix)
  runner/
    dotnet
    node
  exec (wrapper w/ env + timeout + streaming logs)
  cache (hash inputs -> artifact reuse)
  artifact (store + manifest)
  logging (structured)
  detect (infer project type)
  docker (image building and registry operations)
pkg/
  semver utils
  hashing
```

## Runtime Strategy
Option A (MVP): Use Docker images per toolchain (simplest isolation).
Option B (later): Native installed SDK managers (asdf, nvm, manual).
Decision: Start with Docker to avoid host pollution.

## Config (MVP Sketch)
```yaml
runtime:
  dotnet:
    versions: ["6.0.415", "8.0.100"]
  node:
    versions: ["18.20.2", "20.11.1"]
matrix:
  - path: services/api
    type: dotnet
    frameworks: ["6.0.415"]
  - path: frontend
    type: node
    packageManager: pnpm
    nodeVersions: ["20.11.1"]
  - path: libs/ui
    type: node
    buildScripts: ["build"]
defaults:
  concurrency: 4
  artifactDir: dist
```
Flags override file values.

## Dotnet Build Steps (MVP)
- dotnet restore
- dotnet build -c Release
- (optional later) dotnet publish

## Node Build Steps (MVP)
- Install (respect lock file + chosen package manager)
- Run build script (fallback: npm run build)

## Detection Heuristics
- *.csproj → dotnet
- package.json → node
- angular.json → angular
- next.config.* → next
- vite.config.* → vite

## Caching
Key = hash(toolchain version + lock files + build config).
Store artifacts under `.buildcache/<key>/`.
If hit → copy to output + note `reused=true`.

## Artifacts
Output directory structure: `./out/<project>/<tool-version>/`.
Include `manifest.json`:
```json
{
  "project": "...",
  "toolchain": "...",
  "version": "...",
  "hash": "...",
  "buildTimeMs": 1234,
  "reused": false
}
```

## Concurrency
Worker pool (size = min(config, CPU cores)).
Cancellation on first fatal error (flag to continue later).

## Logging
- Structured (JSON) when `--json`
- Human readable otherwise
- Levels: info / warn / error / debug

## CLI Commands (MVP)
- `build`          Run builds (default)
- `plan`           Show expanded matrix without executing
- `clean`          Remove cache + out
- `inspect <key>`  Show manifest
- `version`        Show tool version

## Flags (MVP)
- `--config build.yaml`
- `--concurrency N`
- `--json`
- `--no-cache`
- `--only path1,path2`
- `--dry-run`

## Error Strategy
Fail fast by default; later add `--keep-going`.
Exit codes:
0 success
1 build failure
2 config error
3 internal error

## Security (MVP)
- No network sandboxing
- Warn if running arbitrary pre/post scripts (defer support)

## Stretch (Post-MVP)
- Tests execution
- Container image packaging
- SBOM generation
- Remote cache
- Plugin system
- Git metadata embedding
- TUI dashboard

## Simplest Happy Path Flow
1. Parse config → validate.
2. Detect project types if not declared.
3. Expand matrix.
4. For each build task:
   a. Compute cache key.
   b. If hit → reuse.
   c. Else run inside docker image (e.g. `mcr.microsoft.com/dotnet/sdk:6.0`, `node:20`).
   d. Collect artifacts + manifest.
5. Summarize results.

## Initial Docker Images Needed
- `mcr.microsoft.com/dotnet/sdk:<version>`
- `node:<version>` (bundled corepack for pnpm/yarn)

## Risks
- Large matrix explosions
- Docker performance on some hosts
- Multi-package monorepos (pnpm workspaces) require root-level install

## MVP Success Criteria
- Builds at least one dotnet project with 2 SDK versions
- Builds at least one node project with 2 node versions
- Cache works (2nd run reuses)
- `plan` command shows expected matrix
- Artifacts + manifest generated

## Proposed Next Step
Scaffold module layout + minimal CLI (`plan` + `build --dry-run`).

## Open Questions
- Preferred project directory name? (e.g. `multibuild`, `forge`, `buildtool`).
- Need support for monorepo root install for Node in MVP?
- Artifact compression (zip) in MVP or defer?
