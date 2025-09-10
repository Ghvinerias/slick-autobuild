package planner

import (
	"sort"

	"slick-autobuild/internal/config"
)

// Task represents a single build job after matrix expansion.
type Task struct {
	Path    string
	Kind    string // dotnet or node (for MVP)
	Version string // toolchain version (dotnet sdk version or node version)
}

// Plan is the final set of tasks.
type Plan struct {
	Tasks []Task
}

// Expand builds a plan from provided config and optional selection filter.
func Expand(cfg *config.Root, selected map[string]struct{}) Plan {
	var tasks []Task

	for _, m := range cfg.Matrix {
		if len(selected) > 0 {
			if _, ok := selected[m.Path]; !ok {
				continue
			}
		}
		switch m.Type {
		case "dotnet":
			versions := m.Frameworks
			if len(versions) == 0 {
				versions = cfg.Runtime.Dotnet.Versions
			}
			for _, v := range versions {
				if v == "" {
					continue
				}
				tasks = append(tasks, Task{Path: m.Path, Kind: "dotnet", Version: v})
			}
		case "node":
			versions := m.NodeVersions
			if len(versions) == 0 {
				versions = cfg.Runtime.Node.Versions
			}
			for _, v := range versions {
				if v == "" {
					continue
				}
				tasks = append(tasks, Task{Path: m.Path, Kind: "node", Version: v})
			}
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Path == tasks[j].Path {
			if tasks[i].Kind == tasks[j].Kind {
				return tasks[i].Version < tasks[j].Version
			}
			return tasks[i].Kind < tasks[j].Kind
		}
		return tasks[i].Path < tasks[j].Path
	})

	return Plan{Tasks: tasks}
}
