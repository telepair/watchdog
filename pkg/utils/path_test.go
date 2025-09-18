package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "tilde path",
			input:    "~",
			expected: homeDir,
			wantErr:  false,
		},
		{
			name:     "tilde with subpath",
			input:    "~/Documents",
			expected: filepath.Join(homeDir, "Documents"),
			wantErr:  false,
		},
		{
			name:     "relative path",
			input:    "./test",
			expected: filepath.Join(currentDir, "test"),
			wantErr:  false,
		},
		{
			name:     "absolute path",
			input:    "/tmp/test",
			expected: "/tmp/test",
			wantErr:  false,
		},
		{
			name:     "environment variable",
			input:    "$HOME/test",
			expected: filepath.Join(homeDir, "test"),
			wantErr:  false,
		},
		{
			name:     "environment variable with braces",
			input:    "${HOME}/test",
			expected: filepath.Join(homeDir, "test"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ExpandPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpandPathWithCustomEnv(t *testing.T) {
	// Set custom environment variable
	originalValue := os.Getenv("TEST_PATH")
	defer os.Setenv("TEST_PATH", originalValue)

	os.Setenv("TEST_PATH", "/custom/path")

	result, err := ExpandPath("$TEST_PATH/subdir")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	expected := "/custom/path/subdir"
	if result != expected {
		t.Errorf("ExpandPath() = %v, want %v", result, expected)
	}
}

func BenchmarkExpandPath(b *testing.B) {
	paths := []string{
		"~/Documents",
		"./relative/path",
		"/absolute/path",
		"$HOME/test",
		"${HOME}/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			_, _ = ExpandPath(path)
		}
	}
}

func TestExpandPathEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "path with multiple tildes",
			input:   "~/~/test",
			wantErr: false,
		},
		{
			name:    "path with tilde in middle",
			input:   "/tmp/~/test",
			wantErr: false,
		},
		{
			name:    "undefined environment variable",
			input:   "$UNDEFINED_VAR/test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == "" {
				t.Errorf("ExpandPath() returned empty result for input %q", tt.input)
			}
			// Verify result is absolute path (unless empty input)
			if tt.input != "" && result != "" && !filepath.IsAbs(result) {
				t.Errorf("ExpandPath() returned non-absolute path: %v", result)
			}
		})
	}
}
