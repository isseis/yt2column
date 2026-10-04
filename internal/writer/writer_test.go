//go:build test

package writer

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
)

func TestNewRejectsNilClient(t *testing.T) {
	cases := []struct {
		name   string
		client llm.LLMClient
	}{
		{"nil interface", nil},
		{"typed nil", (*llmtestutil.FakeLLMClient)(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := New(tc.client, Options{})
			if !errors.Is(err, errNilLLMClient) {
				t.Errorf("New error = %v, want %v", err, errNilLLMClient)
			}
			if w != nil {
				t.Errorf("New returned a writer %v, want nil", w)
			}
		})
	}
}
