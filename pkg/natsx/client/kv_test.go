package client

import (
	"testing"
	"time"
)

func TestBucketConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *BucketConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: &BucketConfig{
				Name:         "test-bucket",
				History:      5,
				Replicas:     1,
				OnMemory:     false,
				Compression:  false,
				MaxBytes:     1024 * 1024,
				MaxValueSize: 1024,
				TTL:          time.Hour,
			},
			expectErr: false,
		},
		{
			name: "empty name",
			config: &BucketConfig{
				Name: "",
			},
			expectErr: true,
		},
		{
			name: "invalid name starting with dot",
			config: &BucketConfig{
				Name: ".invalid",
			},
			expectErr: true,
		},
		{
			name: "invalid name starting with hyphen",
			config: &BucketConfig{
				Name: "-invalid",
			},
			expectErr: true,
		},
		{
			name: "name too long",
			config: &BucketConfig{
				Name: "this-bucket-name-is-way-too-long-and-exceeds-the-maximum-length-allowed",
			},
			expectErr: true,
		},
		{
			name: "valid name with dots and underscores",
			config: &BucketConfig{
				Name: "valid.bucket_name",
			},
			expectErr: false,
		},
		{
			name: "valid name with numbers",
			config: &BucketConfig{
				Name: "bucket123",
			},
			expectErr: false,
		},
		{
			name: "invalid name with spaces",
			config: &BucketConfig{
				Name: "invalid name",
			},
			expectErr: true,
		},
		{
			name: "invalid name with special characters",
			config: &BucketConfig{
				Name: "invalid@bucket",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("BucketConfig.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestBucketConfig_ValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		config      *BucketConfig
		expectError string
	}{
		{
			name: "empty name",
			config: &BucketConfig{
				Name: "",
			},
			expectError: "bucket name cannot be empty",
		},
		{
			name: "name starting with dot",
			config: &BucketConfig{
				Name: ".invalid",
			},
			expectError: "bucket name cannot start with dot or hyphen",
		},
		{
			name: "name starting with hyphen",
			config: &BucketConfig{
				Name: "-invalid",
			},
			expectError: "bucket name cannot start with dot or hyphen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
				return
			}
			if err.Error() != "invalid bucket name: "+tt.expectError {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectError, err.Error())
			}
		})
	}
}

func TestNewKVManager_InvalidConfig(t *testing.T) {
	// Test with invalid bucket config - this should fail during validation
	// before even trying to connect to JetStream
	invalidConfig := BucketConfig{
		Name: "", // Empty name should fail validation
	}

	manager, err := NewKVManager(nil, invalidConfig)
	if err == nil {
		t.Error("NewKVManager should fail with invalid config")
	}
	if manager != nil {
		t.Error("NewKVManager should return nil manager on validation failure")
	}

	// Check that the error is about validation
	expectedErrorPrefix := "invalid bucket config:"
	if err != nil && err.Error()[:len(expectedErrorPrefix)] != expectedErrorPrefix {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestDefaultInitTimeout(t *testing.T) {
	// Test that the default timeout constant has a reasonable value
	if defaultInitTimeout != 5*time.Second {
		t.Errorf("Expected default init timeout to be 5 seconds, got %v", defaultInitTimeout)
	}
}

// Test helper functions for validation that are used by KVManager

func TestValidateKeyForKV(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		expectErr bool
	}{
		{
			name:      "valid key",
			key:       "test.key",
			expectErr: false,
		},
		{
			name:      "valid key with underscores",
			key:       "test_key",
			expectErr: false,
		},
		{
			name:      "valid key with hyphens",
			key:       "test-key",
			expectErr: false,
		},
		{
			name:      "empty key",
			key:       "",
			expectErr: true,
		},
		{
			name:      "key with spaces",
			key:       "test key",
			expectErr: true,
		},
		{
			name:      "key starting with dot",
			key:       ".test",
			expectErr: true,
		},
		{
			name:      "key ending with dot",
			key:       "test.",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateKey() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestValidateValueForKV(t *testing.T) {
	tests := []struct {
		name      string
		value     []byte
		expectErr bool
	}{
		{
			name:      "valid value",
			value:     []byte("test value"),
			expectErr: false,
		},
		{
			name:      "empty value",
			value:     []byte{},
			expectErr: false,
		},
		{
			name:      "nil value",
			value:     nil,
			expectErr: true,
		},
		{
			name:      "large value within limit",
			value:     make([]byte, MaxValueSize),
			expectErr: false,
		},
		{
			name:      "value too large",
			value:     make([]byte, MaxValueSize+1),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValue(tt.value)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateValue() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestValidateBucketNameForKV(t *testing.T) {
	tests := []struct {
		name      string
		bucket    string
		expectErr bool
	}{
		{
			name:      "valid bucket name",
			bucket:    "test-bucket",
			expectErr: false,
		},
		{
			name:      "valid bucket with dots",
			bucket:    "test.bucket",
			expectErr: false,
		},
		{
			name:      "valid bucket with underscores",
			bucket:    "test_bucket",
			expectErr: false,
		},
		{
			name:      "empty bucket name",
			bucket:    "",
			expectErr: true,
		},
		{
			name:      "bucket name starting with dot",
			bucket:    ".test",
			expectErr: true,
		},
		{
			name:      "bucket name starting with hyphen",
			bucket:    "-test",
			expectErr: true,
		},
		{
			name:      "bucket name too long",
			bucket:    "this-is-a-very-long-bucket-name-that-exceeds-the-maximum-allowed",
			expectErr: true,
		},
		{
			name:      "bucket name with spaces",
			bucket:    "test bucket",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketName(tt.bucket)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateBucketName() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
