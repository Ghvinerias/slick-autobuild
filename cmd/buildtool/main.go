package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"slick-autobuild/internal/artifact"
	"slick-autobuild/internal/config"
	"slick-autobuild/internal/logging"
	"slick-autobuild/internal/planner"
	"slick-autobuild/internal/runner"
)

var (
	flagConfig      = flag.String("config", "build.yaml", "Path to config file")
	flagConcurrency = flag.Int("concurrency", 0, "Max concurrent builds (default: CPU cores)")
	flagJSON        = flag.Bool("json", false, "JSON logging output")
	flagNoCache     = flag.Bool("no-cache", false, "Disable build cache")
	flagOnly        = flag.String("only", "", "Comma separated project paths to include")
	flagDryRun      = flag.Bool("dry-run", false, "Plan only; do not execute builds")
	flagVersion     = flag.Bool("version", false, "Print version and exit")
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
	sem := make(chan struct{}, conc)
	errCh := make(chan error, len(plan.Tasks))
	for _, t := range plan.Tasks {
		sem <- struct{}{}
		go func(task planner.Task) {
			defer func() { <-sem }()
			start := time.Now()
			logger.Info("build start", map[string]interface{}{"path": task.Path, "kind": task.Kind, "version": task.Version})

			// Find matrix entry for extra fields (package manager, build scripts)
			var pkgMgr string
			var scripts []string
			for _, me := range cfg.Matrix {
				if me.Path == task.Path && me.Type == task.Kind {
					pkgMgr = me.PackageManager
					scripts = me.BuildScripts
					break
				}
			}

			runErr := runner.RunTask(ctx, task, runner.Options{Logger: logger, WorkspaceRoot: workspaceRoot}, pkgMgr, scripts)
			if runErr != nil {
				logger.Error("build failed", map[string]interface{}{"path": task.Path, "error": runErr})
				errCh <- runErr
				return
			}

			elapsed := time.Since(start)
			outDir := filepath.Join("out", task.Path, task.Version)
			_ = artifact.WriteManifest(outDir, artifact.Manifest{
				Project: task.Path,
				Kind: task.Kind,
				Toolchain: task.Kind,
				Version: task.Version,
				Hash: "", // caching not yet implemented
				BuildTimeMs: elapsed.Milliseconds(),
				Reused: false,
			})
			logger.Info("build complete", map[string]interface{}{"path": task.Path, "elapsed_ms": elapsed.Milliseconds()})
		}(t)
	}
	for i := 0; i < cap(sem); i++ { sem <- struct{}{} }
	close(errCh)
	for e := range errCh { if e != nil { return errors.New("one or more builds failed") } }
	logger.Info("all tasks completed", nil)
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
	os.Exit(1)
}
