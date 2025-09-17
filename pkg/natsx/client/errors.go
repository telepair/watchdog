package client

import (
	"errors"
	"fmt"
)

var (
	// ErrClientClosed is returned when operations are attempted on a closed client.
	ErrClientClosed = errors.New("client is closed")

	// ErrInvalidSubject is returned when an invalid subject is provided.
	ErrInvalidSubject = errors.New("invalid subject")

	// ErrInvalidKey is returned when an invalid key is provided for KV operations.
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue is returned when an invalid value is provided for KV operations.
	ErrInvalidValue = errors.New("invalid value")
)

// ErrorWithMetrics wraps an error and records it in metrics.
func (c *Client) ErrorWithMetrics(err error) error {
	if err != nil && c.metrics != nil {
		c.metrics.RecordError()
	}
	return err
}

// WrapValidationError wraps a validation error with context.
func WrapValidationError(field string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid %s: %w", field, err)
}

// CheckClientState verifies if the client is in a valid state for operations.
func (c *Client) CheckClientState() error {
	c.mu.RLock()
	closed := c.closed || c.conn == nil
	c.mu.RUnlock()

	if closed {
		return c.ErrorWithMetrics(ErrClientClosed)
	}
	return nil
}
