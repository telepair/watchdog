package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands relative paths, tilde (~), and environment variables to absolute paths
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Handle tilde expansion
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, path[2:])
	} else if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = homeDir
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// EnsurePath ensures that the directory exists for the given path.
// If path is a directory, it creates the directory.
// If path is a file, it creates the parent directory.
func EnsurePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand the path to handle ~ and environment variables
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return path, err
	}

	// Check if path exists and determine if it's a directory or file
	info, err := os.Stat(expandedPath)
	if err != nil {
		// Path doesn't exist, need to determine if it's meant to be a directory or file
		var dirPath string

		// If path ends with a separator or has no extension, treat as directory
		if strings.HasSuffix(expandedPath, string(filepath.Separator)) || filepath.Ext(expandedPath) == "" {
			dirPath = expandedPath
		} else {
			// Path has extension, treat as file and use parent directory
			dirPath = filepath.Dir(expandedPath)
		}

		return expandedPath, os.MkdirAll(dirPath, 0750)
	}

	// Path exists
	if info.IsDir() {
		// Already a directory, nothing to do
		return expandedPath, nil
	}

	// Path exists but is not a directory, return error
	return expandedPath, fmt.Errorf("path %s exists but is not a directory", expandedPath)
}
