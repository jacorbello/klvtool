package envcheck

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LookPathFunc resolves a binary on PATH. It is injected for testability.
type LookPathFunc func(file string) (string, error)

// VersionRunner executes a version command and returns its combined output.
type VersionRunner func(ctx context.Context, name string, args ...string) (string, error)

// ToolHealth describes one checked executable.
type ToolHealth struct {
	Name    string
	Path    string
	Version string
	Healthy bool
	Error   string
}

// ModuleHealth describes one required backend module or plugin.
type ModuleHealth struct {
	Name    string
	Healthy bool
	Error   string
}

// BackendHealth describes the health of a media backend and its required tools.
type BackendHealth struct {
	Name           string
	Tools          []ToolHealth
	Modules        []ModuleHealth
	Healthy        bool
	MissingTools   []string
	MissingModules []string
}

// Report is the structured result of an environment check.
type Report struct {
	Platform        string
	GuidanceSummary string
	Guidance        []string
	Backends        []BackendHealth
}

// BackendsByName indexes backend health reports by backend name.
func (r Report) BackendsByName() map[string]*BackendHealth {
	index := make(map[string]*BackendHealth, len(r.Backends))
	for i := range r.Backends {
		backend := &r.Backends[i]
		index[backend.Name] = backend
	}
	return index
}

type backendSpec struct {
	name    string
	tools   []string
	modules []string
}

var backendSpecs = []backendSpec{
	{name: "ffmpeg", tools: []string{"ffmpeg", "ffprobe"}},
	{name: "gstreamer", tools: []string{"gst-launch-1.0", "gst-inspect-1.0", "gst-discoverer-1.0"}, modules: []string{"tsdemux"}},
}

// Detect checks the configured backend tools and returns a structured report.
func Detect(ctx context.Context, goos string, env map[string]string, lookPath LookPathFunc, run VersionRunner) Report {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if run == nil {
		run = defaultVersionRunner
	}

	guidance := InstallGuidance(goos, env)
	report := Report{
		Platform:        guidance.Platform,
		GuidanceSummary: guidance.Summary,
		Guidance:        guidance.Steps,
		Backends:        make([]BackendHealth, 0, len(backendSpecs)),
	}

	for _, spec := range backendSpecs {
		report.Backends = append(report.Backends, detectBackend(ctx, spec, lookPath, run))
	}

	return report
}

func detectBackend(ctx context.Context, spec backendSpec, lookPath LookPathFunc, run VersionRunner) BackendHealth {
	backend := BackendHealth{Name: spec.name}
	allHealthy := true
	var inspectPath string

	for _, toolName := range spec.tools {
		tool := ToolHealth{Name: toolName}

		path, err := lookPath(toolName)
		if err != nil {
			tool.Error = err.Error()
			backend.MissingTools = append(backend.MissingTools, toolName)
			backend.Tools = append(backend.Tools, tool)
			allHealthy = false
			continue
		}

		tool.Path = path
		if toolName == "gst-inspect-1.0" {
			inspectPath = path
		}
		version, err := run(ctx, path, versionArgs(toolName)...)
		if err != nil {
			tool.Error = err.Error()
			backend.Tools = append(backend.Tools, tool)
			allHealthy = false
			continue
		}

		tool.Version = strings.TrimSpace(version)
		tool.Healthy = true
		backend.Tools = append(backend.Tools, tool)
	}

	for _, moduleName := range spec.modules {
		module := ModuleHealth{Name: moduleName}
		switch inspectPath {
		case "":
			module.Error = "gst-inspect-1.0 unavailable"
			backend.MissingModules = append(backend.MissingModules, moduleName)
			allHealthy = false
		default:
			if _, err := run(ctx, inspectPath, moduleName); err != nil {
				module.Error = err.Error()
				backend.MissingModules = append(backend.MissingModules, moduleName)
				allHealthy = false
			} else {
				module.Healthy = true
			}
		}
		backend.Modules = append(backend.Modules, module)
	}

	backend.Healthy = allHealthy && len(backend.Tools) == len(spec.tools) && len(backend.Modules) == len(spec.modules)
	return backend
}

func defaultVersionRunner(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func versionArgs(toolName string) []string {
	switch toolName {
	case "ffmpeg", "ffprobe":
		return []string{"-version"}
	default:
		return []string{"--version"}
	}
}
