package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"slick-autobuild/internal/artifact"
	"slick-autobuild/internal/cache"
	"slick-autobuild/internal/config"
	"slick-autobuild/internal/docker"
	"slick-autobuild/internal/logging"
	"slick-autobuild/internal/planner"
	"slick-autobuild/internal/runner"
)

// validatePath ensures the path is safe and doesn't contain path traversal attempts
func validatePath(path string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)
	
	// Check for path traversal attempts that try to escape the working directory
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: path traversal detected in %s", path)
	}
	
	return nil
}

var (
	flagConfig      = flag.String("config", "build.yaml", "Path to config file")
	flagConcurrency = flag.Int("concurrency", 0, "Max concurrent builds (default: CPU cores)")
	flagJSON        = flag.Bool("json", false, "JSON logging output")
	flagNoCache     = flag.Bool("no-cache", false, "Disable build cache")
	flagOnly        = flag.String("only", "", "Comma separated project paths to include")
	flagDryRun      = flag.Bool("dry-run", false, "Plan only; do not execute builds")
	flagVersion     = flag.Bool("version", false, "Print version and exit")
	flagNoDocker    = flag.Bool("no-docker", false, "Disable Docker image building")
	flagPushImages  = flag.Bool("push-images", false, "Force push Docker images (overrides config)")
)

// Error exit codes as defined in MVP
const (
	ExitSuccess       = 0
	ExitBuildFailure  = 1 
	ExitConfigError   = 2
	ExitInternalError = 3
)

const version = "0.0.1-dev"

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(version)
		return
	}

	args := flag.Args()
	cmd := "build"
	if len(args) > 0 {
		cmd = args[0]
	}

	// Basic switch – only plan/build for MVP scaffold
	switch cmd {
	case "plan":
		if err := runPlan(); err != nil {
			fatal(err)
		}
	case "build":
		if err := runBuild(); err != nil {
			fatal(err)
		}
	case "clean":
		if err := runClean(); err != nil {
			fatal(err)
		}
	case "version":
		fmt.Println(version)
	case "inspect":
		if len(args) < 2 {
			fatal(fmt.Errorf("inspect command requires a key argument"))
		}
		if err := runInspect(args[1]); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unknown command: %s", cmd))
	}
}

func runPlan() error {
	cfg, err := config.Load(*flagConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(*flagJSON)
	selected := parseOnly()
	plan := planner.Expand(cfg, selected)
	logger.Info("plan generated", map[string]interface{}{"tasks": len(plan.Tasks)})
	printPlan(plan)
	return nil
}

func runBuild() error {
	if *flagDryRun {
		return runPlan()
	}
	cfg, err := config.Load(*flagConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(*flagJSON)
	selected := parseOnly()
	plan := planner.Expand(cfg, selected)
	conc := *flagConcurrency
	if conc <= 0 {
		conc = runtime.NumCPU()
	}
	logger.Info("starting builds", map[string]interface{}{"tasks": len(plan.Tasks), "concurrency": conc})

	workspaceRoot, _ := os.Getwd()
	ctx := context.Background()

	// Check if Docker is available for projects that need it (only if not disabled)
	if !*flagNoDocker {
		hasDockerProjects := false
		registriesToLogin := make(map[string]bool)
		
		for _, task := range plan.Tasks {
			for _, me := range cfg.Matrix {
				if me.Path == task.Path && me.Type == task.Kind && me.Docker != nil && me.Docker.Enabled {
					hasDockerProjects = true
					// Collect unique registries for login
					registries := me.Docker.Registries
					if len(registries) == 0 {
						registriesToLogin["docker.io"] = true
					} else {
						for _, reg := range registries {
							registriesToLogin[reg] = true
						}
					}
				}
			}
		}
		
		if hasDockerProjects {
			if err := docker.CheckDockerAvailable(ctx); err != nil {
				return fmt.Errorf("Docker is required but not available: %w", err)
			}
			
			// Login to registries if credentials are available
			for registry := range registriesToLogin {
				if err := docker.LoginToRegistry(ctx, registry, logger); err != nil {
					logger.Warn("failed to login to registry", map[string]interface{}{
						"registry": registry,
						"error": err,
					})
				}
			}
		}
	}

	sem := make(chan struct{}, conc)
	errCh := make(chan error, len(plan.Tasks))
	for _, t := range plan.Tasks {
		sem <- struct{}{}
		go func(task planner.Task) {
			defer func() { <-sem }()
			start := time.Now()
			
			// Generate cache key
			cacheKey, err := cache.Key(task, workspaceRoot)
			if err != nil {
				logger.Error("cache key generation failed", map[string]interface{}{"path": task.Path, "error": err})
				errCh <- err
				return
			}
			
			outDir := filepath.Join("out", task.Path, task.Version)
			
			// Check cache if not disabled
			var reused bool
			if !*flagNoCache && cache.Exists(cacheKey) {
				logger.Info("cache hit", map[string]interface{}{"path": task.Path, "key": cacheKey})
				if err := cache.Restore(cacheKey, outDir); err != nil {
					logger.Error("cache restore failed", map[string]interface{}{"path": task.Path, "error": err})
					errCh <- err
					return
				}
				reused = true
			} else {
				logger.Info("build start", map[string]interface{}{"path": task.Path, "kind": task.Kind, "version": task.Version, "key": cacheKey})

				// Find matrix entry for extra fields (package manager, build scripts, docker config)
				var pkgMgr string
				var scripts []string
				var dockerCfg *config.DockerConfig
				for _, me := range cfg.Matrix {
					if me.Path == task.Path && me.Type == task.Kind {
						pkgMgr = me.PackageManager
						scripts = me.BuildScripts
						dockerCfg = me.Docker
						break
					}
				}

				runErr := runner.RunTask(ctx, task, runner.Options{Logger: logger, WorkspaceRoot: workspaceRoot}, pkgMgr, scripts)
				if runErr != nil {
					logger.Error("build failed", map[string]interface{}{"path": task.Path, "error": runErr})
					errCh <- runErr
					return
				}
				
				// Build Docker image if enabled and not disabled by flag
				if !*flagNoDocker && dockerCfg != nil && dockerCfg.Enabled {
					// Override push setting if flag is provided
					if *flagPushImages {
						dockerCfg.Push = true
					}
					
					imageBuilder := docker.NewImageBuilder(logger)
					if err := imageBuilder.BuildAndPush(ctx, task.Path, dockerCfg, workspaceRoot); err != nil {
						logger.Error("Docker image build/push failed", map[string]interface{}{"path": task.Path, "error": err})
						// Don't fail the entire build for Docker failures, just log warning
						logger.Warn("continuing with build despite Docker failure", map[string]interface{}{"path": task.Path})
					}
				}
				
				// Store in cache if not disabled
				if !*flagNoCache {
					if err := cache.Store(cacheKey, outDir); err != nil {
						logger.Error("cache store failed", map[string]interface{}{"path": task.Path, "error": err})
						// Don't fail the build for cache store failures
					}
				}
			}

			elapsed := time.Since(start)
			_ = artifact.WriteManifest(outDir, artifact.Manifest{
				Project: task.Path,
				Kind: task.Kind,
				Toolchain: task.Kind,
				Version: task.Version,
				Hash: cacheKey,
				BuildTimeMs: elapsed.Milliseconds(),
				Reused: reused,
			})
			
			if reused {
				logger.Info("build reused", map[string]interface{}{"path": task.Path, "elapsed_ms": elapsed.Milliseconds()})
			} else {
				logger.Info("build complete", map[string]interface{}{"path": task.Path, "elapsed_ms": elapsed.Milliseconds()})
			}
		}(t)
	}
	for i := 0; i < cap(sem); i++ { sem <- struct{}{} }
	close(errCh)
	for e := range errCh { if e != nil { return errors.New("one or more builds failed") } }
	logger.Info("all tasks completed", nil)
	return nil
}

func runClean() error {
	logger := logging.New(*flagJSON)
	
	// Remove cache directory
	cacheDir := ".buildcache"
	if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}
	
	// Remove output directory
	outDir := "out"
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove output directory: %w", err)
	}
	
	logger.Info("clean completed", map[string]interface{}{
		"cache_dir": cacheDir,
		"out_dir": outDir,
	})
	return nil
}

func runInspect(key string) error {
	logger := logging.New(*flagJSON)
	
	// Try to find manifest in cache first, then in output directory
	manifestPath := filepath.Join(".buildcache", key, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Try alternative locations
		possiblePaths := []string{
			filepath.Join("out", key, "manifest.json"),
		}
		
		found := false
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				manifestPath = path
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("manifest not found for key: %s", key)
		}
	}
	
	// Validate the manifest path
	if err := validatePath(manifestPath); err != nil {
		return fmt.Errorf("invalid manifest path: %w", err)
	}
	
	// #nosec G304 - Path is validated above to prevent traversal attacks
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	
	if *flagJSON {
		fmt.Print(string(data))
	} else {
		var manifest artifact.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("failed to parse manifest: %w", err)
		}
		
		fmt.Printf("Manifest for key: %s\n", key)
		fmt.Printf("  Project: %s\n", manifest.Project)
		fmt.Printf("  Kind: %s\n", manifest.Kind)
		fmt.Printf("  Toolchain: %s\n", manifest.Toolchain)
		fmt.Printf("  Version: %s\n", manifest.Version)
		fmt.Printf("  Hash: %s\n", manifest.Hash)
		fmt.Printf("  Build Time: %d ms\n", manifest.BuildTimeMs)
		fmt.Printf("  Reused: %t\n", manifest.Reused)
		fmt.Printf("  Created At: %s\n", manifest.CreatedAt)
	}
	
	logger.Info("inspect completed", map[string]interface{}{"key": key, "path": manifestPath})
	return nil
}

func parseOnly() map[string]struct{} {
	m := map[string]struct{}{}
	if *flagOnly == "" {
		return m
	}
	for _, p := range strings.Split(*flagOnly, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = struct{}{}
		}
	}
	return m
}

func printPlan(p planner.Plan) {
	fmt.Printf("Plan: %d task(s)\n", len(p.Tasks))
	for _, t := range p.Tasks {
		fmt.Printf(" - %s | kind=%s version=%s\n", t.Path, t.Kind, t.Version)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	
	// Determine appropriate exit code based on error type
	exitCode := ExitInternalError // default
	errStr := err.Error()
	
	if strings.Contains(errStr, "load config") || 
	   strings.Contains(errStr, "parse yaml") ||
	   strings.Contains(errStr, "config error") {
		exitCode = ExitConfigError
	} else if strings.Contains(errStr, "build failed") ||
	         strings.Contains(errStr, "one or more builds failed") {
		exitCode = ExitBuildFailure
	}
	
	os.Exit(exitCode)
}
