package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	// MaxHostnameLength is the maximum allowed hostname length in characters.
	MaxHostnameLength = 253
	// MaxKeyLength is the maximum allowed key length for KV operations.
	MaxKeyLength = 256
	// MaxSubjectLength is the maximum allowed NATS subject length.
	MaxSubjectLength = 255
	// MaxBucketNameLength is the maximum allowed KV bucket name length.
	MaxBucketNameLength = 63
	// MaxValueSize is the maximum allowed value size for KV storage (1MB).
	MaxValueSize = 1024 * 1024
	// DefaultConnectivityCheckTimeout is the default timeout for connectivity checks.
	DefaultConnectivityCheckTimeout = 5 * time.Second
)

var (
	// Valid key pattern: alphanumeric, dots, hyphens, underscores.
	validKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	// Valid subject pattern for NATS subjects.
	validSubjectPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+(\.[a-zA-Z0-9._-]+)*$`)
)

// ValidateNATSURL validates a NATS URL.
func ValidateNATSURL(natsURL string) error {
	if natsURL == "" {
		return errors.New("URL cannot be empty")
	}

	parsed, err := url.Parse(natsURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check allowed schemes
	switch parsed.Scheme {
	case "nats", "tls", "ws", "wss":
		// Valid schemes
	default:
		return fmt.Errorf("unsupported scheme: %s, only 'nats', 'tls', 'ws' and 'wss' are supported", parsed.Scheme)
	}

	// Validate hostname
	if parsed.Hostname() == "" {
		return errors.New("hostname is required")
	}

	// Check for localhost variations (secure for internal use)
	hostname := parsed.Hostname()
	if hostname != "localhost" && hostname != "127.0.0.1" && hostname != "::1" {
		// For non-localhost, do additional validation
		if len(hostname) > MaxHostnameLength {
			return fmt.Errorf("hostname too long (max %d characters)", MaxHostnameLength)
		}
	}

	// Validate port - no additional validation needed since url.Parse handles it
	_ = parsed.Port()

	return nil
}

// ValidateKey validates a key name for KV operations.
func ValidateKey(key string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	if len(key) > MaxKeyLength {
		return fmt.Errorf("key too long (max %d characters)", MaxKeyLength)
	}

	if !validKeyPattern.MatchString(key) {
		return errors.New("key contains invalid characters, only alphanumeric, dots, hyphens and " +
			"underscores are allowed")
	}

	// Check for reserved patterns
	if strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
		return errors.New("key cannot start or end with a dot")
	}

	if strings.Contains(key, "..") {
		return errors.New("key cannot contain consecutive dots")
	}

	return nil
}

// ValidateSubject validates a NATS subject.
func ValidateSubject(subject string) error {
	if subject == "" {
		return errors.New("subject cannot be empty")
	}

	if len(subject) > MaxSubjectLength {
		return fmt.Errorf("subject too long (max %d characters)", MaxSubjectLength)
	}

	// Handle wildcard subjects
	if strings.Contains(subject, "*") || strings.Contains(subject, ">") {
		return validateWildcardSubject(subject)
	}

	// Handle regular subjects
	if !validSubjectPattern.MatchString(subject) {
		return errors.New("subject contains invalid characters")
	}

	return nil
}

// validateWildcardSubject validates subjects containing wildcards (* or >).
func validateWildcardSubject(subject string) error {
	if strings.Contains(subject, " ") {
		return errors.New("subject cannot contain spaces")
	}

	parts := strings.Split(subject, ".")
	for i, part := range parts {
		if err := validateSubjectPart(part, i, len(parts)); err != nil {
			return err
		}
	}

	return nil
}

// validateSubjectPart validates a single part of a wildcard subject.
func validateSubjectPart(part string, index, totalParts int) error {
	if part == "" {
		return errors.New("subject contains empty token")
	}

	switch part {
	case ">":
		if index != totalParts-1 {
			return errors.New("> wildcard must be the last token")
		}
	case "*":
		// Star wildcard is always valid
	default:
		if !validSubjectPattern.MatchString(part) {
			return fmt.Errorf("subject contains invalid token: %s", part)
		}
	}

	return nil
}

// ValidateBucketName validates a KV bucket name.
func ValidateBucketName(name string) error {
	if name == "" {
		return errors.New("bucket name cannot be empty")
	}

	if len(name) > MaxBucketNameLength {
		return fmt.Errorf("bucket name too long (max %d characters)", MaxBucketNameLength)
	}

	if !validKeyPattern.MatchString(name) {
		return errors.New("bucket name contains invalid characters, only alphanumeric, dots, hyphens and " +
			"underscores are allowed")
	}

	// Bucket names should not start with dots or hyphens
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return errors.New("bucket name cannot start with dot or hyphen")
	}

	return nil
}

// ValidateValue validates a value for KV storage.
func ValidateValue(value []byte) error {
	if value == nil {
		return errors.New("value cannot be nil")
	}

	// NATS has a default message size limit, but we'll be more conservative
	if len(value) > MaxValueSize {
		return fmt.Errorf("value too large (max %d bytes)", MaxValueSize)
	}

	return nil
}

// CheckConnectivity tests connectivity to a NATS server
// Returns nil if connection is successful, error otherwise.
func CheckConnectivity(natsURL string) error {
	return CheckConnectivityWithTimeout(natsURL, DefaultConnectivityCheckTimeout)
}

// CheckConnectivityWithTimeout tests connectivity to a NATS server with a custom timeout.
func CheckConnectivityWithTimeout(natsURL string, timeout time.Duration) error {
	if natsURL == "" {
		return errors.New("NATS URL cannot be empty")
	}

	// Validate URL format first
	if err := ValidateNATSURL(natsURL); err != nil {
		return fmt.Errorf("invalid NATS URL: %w", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Configure connection options with timeout
	opts := []nats.Option{
		nats.Timeout(timeout),
		nats.MaxReconnects(0), // Don't retry for connectivity check
		nats.Name("gopkg.natsx.connectivity-check"),
		// Disable handlers to avoid log noise during connectivity checks
		nats.DisconnectErrHandler(func(*nats.Conn, error) {}),
		nats.ReconnectHandler(func(*nats.Conn) {}),
		nats.ClosedHandler(func(*nats.Conn) {}),
		nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}),
	}

	// Try to connect
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS server: %w", err)
	}
	defer nc.Close()

	// Check if connection is actually established
	if !nc.IsConnected() {
		return errors.New("connection established but not connected")
	}

	// Optional: perform a basic ping to ensure the connection is working
	done := make(chan error, 1)
	go func() {
		defer close(done)
		// Try to flush pending data to test if connection is truly working
		if flushErr := nc.Flush(); flushErr != nil {
			done <- fmt.Errorf("connection flush failed: %w", flushErr)
			return
		}
		done <- nil
	}()

	// Wait for either context timeout or flush completion
	select {
	case <-ctx.Done():
		return fmt.Errorf("connectivity check timed out after %v", timeout)
	case flushResult := <-done:
		if flushResult != nil {
			return flushResult
		}
	}

	return nil
}
