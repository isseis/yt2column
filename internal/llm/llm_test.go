//go:build test

package llm

import (
	"errors"
	"testing"
)

func TestGenerateRequestValidate(t *testing.T) {
	valid := GenerateRequest{SystemPrompt: "system", UserPrompt: "user", MaxOutputTokens: 256}

	t.Run("accepts a valid request", func(t *testing.T) {
		if err := valid.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("accepts whitespace-only prompts", func(t *testing.T) {
		req := GenerateRequest{SystemPrompt: " \n", UserPrompt: "\t", MaxOutputTokens: 0}
		if err := req.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("accepts zero and positive MaxOutputTokens", func(t *testing.T) {
		for _, n := range []int{0, 1, 4096} {
			req := valid
			req.MaxOutputTokens = n
			if err := req.Validate(); err != nil {
				t.Errorf("Validate(MaxOutputTokens=%d) error = %v, want nil", n, err)
			}
		}
	})

	rejected := map[string]GenerateRequest{
		"empty SystemPrompt":         {SystemPrompt: "", UserPrompt: "user"},
		"empty UserPrompt":           {SystemPrompt: "system", UserPrompt: ""},
		"invalid SystemPrompt UTF-8": {SystemPrompt: "sys\xff", UserPrompt: "user"},
		"invalid UserPrompt UTF-8":   {SystemPrompt: "system", UserPrompt: "user\xff"},
		"negative MaxOutputTokens":   {SystemPrompt: "system", UserPrompt: "user", MaxOutputTokens: -1},
	}
	for name, req := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("Validate() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}
