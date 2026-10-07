//go:build test

package loopbacktest

import "testing"

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
	rejected := []string{
		"https://api.deepseek.com/chat/completions",
		"http://192.168.1.10:8080/chat",
		"http://localhost:8080/chat",
		"http://[2001:db8::1]:8080/chat",
		"https:///chat",
		"127.0.0.1:8080/chat",
	}
	for _, endpoint := range rejected {
		if err := ValidateURL(endpoint); err == nil {
			t.Errorf("ValidateURL(%q) error = nil, want a rejection", endpoint)
		}
	}
}
