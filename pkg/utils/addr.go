package utils

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// ValidateAddr validates the TCP address format without binding to the port.
func ValidateAddr(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("addr is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return fmt.Errorf("resolve tcp addr %q: %w", addr, err)
	}
	return nil
}
