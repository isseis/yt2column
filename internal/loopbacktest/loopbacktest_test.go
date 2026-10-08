//go:build test

package loopbacktest

import (
	"errors"
	"testing"
)

func TestValidateURL(t *testing.T) {
	accepted := []string{
		"http://127.0.0.1:8080/chat",
		"http://[::1]:8080/chat",
	}
	for _, endpoint := range accepted {
		if err := ValidateURL(endpoint); err != nil {
			t.Errorf("ValidateURL(%q) error = %v, want nil", endpoint, err)
		}
	}

	rejected := []struct {
		endpoint string
		want     error
	}{
		{"https://api.deepseek.com/chat/completions", errTestEndpointNotLoopback},
		{"http://192.168.1.10:8080/chat", errTestEndpointNotLoopback},
		{"http://localhost:8080/chat", errTestEndpointNotLoopback},
		{"http://[2001:db8::1]:8080/chat", errTestEndpointNotLoopback},
		{"https:///chat", errTestEndpointNoHost},
		{"127.0.0.1:8080/chat", nil},
	}
	for _, tc := range rejected {
		err := ValidateURL(tc.endpoint)
		if err == nil {
			t.Errorf("ValidateURL(%q) error = nil, want a rejection", tc.endpoint)
			continue
		}
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("ValidateURL(%q) error = %v, want %v", tc.endpoint, err, tc.want)
		}
	}
}
