//go:build test

package writertestutil

import (
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// DefaultOptions returns the writer.Options of a run with every
// post-generation step at its default: the shorten and review steps enabled,
// every template built in, and no artifacts saved. review is the review step's
// client.
//
// It is the one definition of the default that the CLI (with no step flags)
// and the server must both build. Each binary's test compares its own options
// with this value, so a change to the CLI's default fails the CLI test until
// this value changes, and that change fails the server test until the server
// follows.
func DefaultOptions(review llm.LLMClient) writer.Options {
	return writer.Options{
		Shorten: writer.ShortenOptions{Enabled: true},
		Review:  writer.ReviewOptions{Enabled: true, Client: review},
	}
}
