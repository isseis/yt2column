//go:build test

package writertestutil

import (
	"strings"

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

// maxChannelNameRunes is the channel name length limit, in code points. The
// over-limit case below is one past it; the writer's own test pins the limit's
// exact boundary.
const maxChannelNameRunes = 200

// ChannelNameCase is one channel name and whether every place that checks a
// channel name must accept it.
type ChannelNameCase struct {
	Name  string
	Value string
	Valid bool
}

// ChannelNameCases is the one table of channel names the three callers of
// writer.CheckChannelName are tested with: server.parseRequest, the check
// before the LLM call, and Article.CheckPublishable. Each caller's test
// asserts it agrees with Valid, so the callers cannot drift apart. Every
// invalid value breaks exactly one rule.
func ChannelNameCases() []ChannelNameCase {
	return []ChannelNameCase{
		{"empty", "", true},
		{"plain", "Some Channel", true},
		{"emoji with zero-width joiner", "\U0001F468\u200d\U0001F469\u200d\U0001F467", true},
		{"whitespace only", " \t", false},
		{"control character", "a\x1bb", false},
		{"invalid UTF-8", "a\xffb", false},
		{"bidirectional override", "a\u202eb", false},
		{"over the length limit", strings.Repeat("a", maxChannelNameRunes+1), false},
	}
}
