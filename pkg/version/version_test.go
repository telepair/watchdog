package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	// Store original values
	originalVersion := Version
	originalGitCommit := GitCommit
	originalBuildDate := BuildDate

	// Test with default values
	info := Get()

	if info.Version != Version {
		t.Errorf("expected version %s, got %s", Version, info.Version)
	}

	if info.GitCommit != GitCommit {
		t.Errorf("expected git commit %s, got %s", GitCommit, info.GitCommit)
	}

	if info.BuildDate != BuildDate {
		t.Errorf("expected build date %s, got %s", BuildDate, info.BuildDate)
	}

	if info.GoVersion != runtime.Version() {
		t.Errorf("expected Go version %s, got %s", runtime.Version(), info.GoVersion)
	}

	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if info.Platform != expectedPlatform {
		t.Errorf("expected platform %s, got %s", expectedPlatform, info.Platform)
	}

	// Test with custom values
	Version = "1.0.0"
	GitCommit = "abc123"
	BuildDate = "2024-01-01"

	info = Get()

	if info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", info.Version)
	}

	if info.GitCommit != "abc123" {
		t.Errorf("expected git commit abc123, got %s", info.GitCommit)
	}

	if info.BuildDate != "2024-01-01" {
		t.Errorf("expected build date 2024-01-01, got %s", info.BuildDate)
	}

	// Restore original values
	Version = originalVersion
	GitCommit = originalGitCommit
	BuildDate = originalBuildDate
}

func TestInfo_String(t *testing.T) {
	info := Info{
		Version:   "1.0.0",
		GitCommit: "abc123",
		BuildDate: "2024-01-01",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
	}

	result := info.String()
	expected := "Watchdog 1.0.0 (commit: abc123, built: 2024-01-01, go: go1.21.0, platform: linux/amd64)"

	if result != expected {
		t.Errorf("expected string %s, got %s", expected, result)
	}

	// Test with empty values
	info = Info{}
	result = info.String()

	if !strings.Contains(result, "Watchdog") {
		t.Error("string representation should contain 'Watchdog'")
	}

	if !strings.Contains(result, "commit:") {
		t.Error("string representation should contain 'commit:'")
	}

	if !strings.Contains(result, "built:") {
		t.Error("string representation should contain 'built:'")
	}

	if !strings.Contains(result, "go:") {
		t.Error("string representation should contain 'go:'")
	}

	if !strings.Contains(result, "platform:") {
		t.Error("string representation should contain 'platform:'")
	}
}

func TestVersionVariables(t *testing.T) {
	// Test that global variables are properly initialized
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}

	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}

	if GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	// Test that GoVersion matches runtime.Version()
	if GoVersion != runtime.Version() {
		t.Errorf("expected GoVersion %s, got %s", runtime.Version(), GoVersion)
	}
}

func BenchmarkGet(b *testing.B) {
	for b.Loop() {
		_ = Get()
	}
}

func BenchmarkInfo_String(b *testing.B) {
	info := Info{
		Version:   "1.0.0",
		GitCommit: "abc123",
		BuildDate: "2024-01-01",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
	}

	b.ResetTimer()
	for b.Loop() {
		_ = info.String()
	}
}
