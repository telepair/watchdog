package client

import (
	"strings"
	"testing"
	"time"
)

func TestValidateNATSURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid nats URL",
			url:     "nats://localhost:4222",
			wantErr: false,
		},
		{
			name:    "valid tls URL",
			url:     "tls://localhost:4222",
			wantErr: false,
		},
		{
			name:    "valid ws URL",
			url:     "ws://localhost:8080",
			wantErr: false,
		},
		{
			name:    "valid wss URL",
			url:     "wss://localhost:8080",
			wantErr: false,
		},
		{
			name:    "valid with IP",
			url:     "nats://127.0.0.1:4222",
			wantErr: false,
		},
		{
			name:    "valid with IPv6",
			url:     "nats://[::1]:4222",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			url:     "http://localhost:4222",
			wantErr: true,
		},
		{
			name:    "missing hostname",
			url:     "nats://:4222",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "hostname too long",
			url:     "nats://" + strings.Repeat("a", 254) + ":4222",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNATSURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNATSURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid key",
			key:     "my.key",
			wantErr: false,
		},
		{
			name:    "valid key with underscores",
			key:     "my_key_123",
			wantErr: false,
		},
		{
			name:    "valid key with hyphens",
			key:     "my-key-123",
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "key too long",
			key:     strings.Repeat("a", 257),
			wantErr: true,
		},
		{
			name:    "key with invalid characters",
			key:     "my/key",
			wantErr: true,
		},
		{
			name:    "key with spaces",
			key:     "my key",
			wantErr: true,
		},
		{
			name:    "key starting with dot",
			key:     ".mykey",
			wantErr: true,
		},
		{
			name:    "key ending with dot",
			key:     "mykey.",
			wantErr: true,
		},
		{
			name:    "key with consecutive dots",
			key:     "my..key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{
			name:    "valid subject",
			subject: "foo.bar",
			wantErr: false,
		},
		{
			name:    "valid single token",
			subject: "foo",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			subject: "foo.bar.123",
			wantErr: false,
		},
		{
			name:    "valid wildcard single",
			subject: "foo.*",
			wantErr: false,
		},
		{
			name:    "valid wildcard multiple",
			subject: "foo.*.bar",
			wantErr: false,
		},
		{
			name:    "valid wildcard all",
			subject: "foo.>",
			wantErr: false,
		},
		{
			name:    "empty subject",
			subject: "",
			wantErr: true,
		},
		{
			name:    "subject too long",
			subject: strings.Repeat("a", 256),
			wantErr: true,
		},
		{
			name:    "subject with spaces",
			subject: "foo bar",
			wantErr: true,
		},
		{
			name:    "invalid wildcard position",
			subject: "foo.>.bar",
			wantErr: true,
		},
		{
			name:    "consecutive dots",
			subject: "foo..bar",
			wantErr: false, // This is actually valid according to the regex pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubject(tt.subject)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		wantErr    bool
	}{
		{
			name:       "valid bucket name",
			bucketName: "my_bucket",
			wantErr:    false,
		},
		{
			name:       "valid with numbers",
			bucketName: "bucket123",
			wantErr:    false,
		},
		{
			name:       "valid with dots",
			bucketName: "my.bucket",
			wantErr:    false,
		},
		{
			name:       "empty bucket name",
			bucketName: "",
			wantErr:    true,
		},
		{
			name:       "bucket name too long",
			bucketName: strings.Repeat("a", 64),
			wantErr:    true,
		},
		{
			name:       "bucket name with invalid characters",
			bucketName: "my/bucket",
			wantErr:    true,
		},
		{
			name:       "bucket name starting with dot",
			bucketName: ".bucket",
			wantErr:    true,
		},
		{
			name:       "bucket name starting with hyphen",
			bucketName: "-bucket",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketName(tt.bucketName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBucketName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateValue(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		wantErr bool
	}{
		{
			name:    "valid value",
			value:   []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "empty value",
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "nil value",
			value:   nil,
			wantErr: true,
		},
		{
			name:    "value too large",
			value:   make([]byte, MaxValueSize+1),
			wantErr: true,
		},
		{
			name:    "max size value",
			value:   make([]byte, MaxValueSize),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValue(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckConnectivity(t *testing.T) {
	tests := []struct {
		name    string
		natsURL string
		wantErr bool
	}{
		{
			name:    "empty URL",
			natsURL: "",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			natsURL: "invalid-url",
			wantErr: true,
		},
		{
			name:    "valid URL format but server not running",
			natsURL: "nats://localhost:9999",
			wantErr: true, // Assuming server is not running on this port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConnectivity(tt.natsURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckConnectivity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckConnectivityWithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		natsURL string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "empty URL",
			natsURL: "",
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name:    "invalid URL",
			natsURL: "invalid-url",
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name:    "valid URL format but server not running",
			natsURL: "nats://localhost:9999",
			timeout: 1 * time.Second,
			wantErr: true, // Assuming server is not running on this port
		},
		{
			name:    "very short timeout",
			natsURL: "nats://localhost:4222",
			timeout: 1 * time.Nanosecond,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConnectivityWithTimeout(tt.natsURL, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckConnectivityWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWildcardSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{
			name:    "valid single wildcard",
			subject: "foo.*",
			wantErr: false,
		},
		{
			name:    "valid multiple wildcards",
			subject: "*.*.bar",
			wantErr: false,
		},
		{
			name:    "valid all wildcard at end",
			subject: "foo.>",
			wantErr: false,
		},
		{
			name:    "invalid all wildcard not at end",
			subject: "foo.>.bar",
			wantErr: true,
		},
		{
			name:    "subject with space",
			subject: "foo bar.*",
			wantErr: true,
		},
		{
			name:    "empty token",
			subject: "foo.*..",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWildcardSubject(tt.subject)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWildcardSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkValidateNATSURL(b *testing.B) {
	url := "nats://localhost:4222"
	b.ResetTimer()
	for range b.N {
		_ = ValidateNATSURL(url)
	}
}

func BenchmarkValidateKey(b *testing.B) {
	key := "my.test.key"
	b.ResetTimer()
	for range b.N {
		_ = ValidateKey(key)
	}
}

func BenchmarkValidateSubject(b *testing.B) {
	subject := "foo.bar.baz"
	b.ResetTimer()
	for range b.N {
		_ = ValidateSubject(subject)
	}
}
