package client

import (
	"errors"
	"fmt"
)

var (
	// ErrClientClosed is returned when operations are attempted on a closed client.
	ErrClientClosed = errors.New("client is closed")

	// ErrNotConnected is returned when client is not connected.
	ErrNotConnected = errors.New("client not connected")

	// ErrConnectionBusy is returned when connection is busy or channel is full.
	ErrConnectionBusy = errors.New("connection busy")

	// ErrInvalidSubject is returned when an invalid subject is provided.
	ErrInvalidSubject = errors.New("invalid subject")

	// ErrInvalidKey is returned when an invalid key is provided for KV operations.
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue is returned when an invalid value is provided for KV operations.
	ErrInvalidValue = errors.New("invalid value")
)

// WrapValidationError wraps a validation error with context.
func WrapValidationError(field string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid %s: %w", field, err)
}
