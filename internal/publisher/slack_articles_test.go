//go:build test

package publisher_test

import (
	"testing"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
)

// TestIntegrationArticles pins the shape of the fixed articles the webhook
// integration test posts, so `make test` catches a drift without posting:
// the single-message article becomes one message of exactly the per-message
// limit, the split article becomes two or more, and neither is rejected by
// the preparation. Markers from two instants must give the same lengths.
func TestIntegrationArticles(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 999_000_000, time.FixedZone("JST", 9*60*60)),
	} {
		t.Run(at.Format(time.RFC3339Nano), func(t *testing.T) {
			single, split := publishertestutil.IntegrationArticles(at)
			if single.Title == split.Title {
				t.Errorf("both articles have the title %q; the channel cannot tell them apart", single.Title)
			}

			messages, err := publisher.SlackMessagesForTest(single)
			if err != nil {
				t.Fatalf("single: preparation error = %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("single: %d messages, want 1", len(messages))
			}
			if got := utf8.RuneCountInString(messages[0]); got != publisher.SlackMaxMessageRunesForTest {
				t.Errorf("single: message has %d code points, want exactly %d", got, publisher.SlackMaxMessageRunesForTest)
			}
			if utf8.RuneCountInString(messages[0]) == len(messages[0]) {
				t.Error("single: message is ASCII only; it must exercise counting in code points")
			}

			messages, err = publisher.SlackMessagesForTest(split)
			if err != nil {
				t.Fatalf("split: preparation error = %v", err)
			}
			if len(messages) < 2 {
				t.Errorf("split: %d messages, want 2 or more", len(messages))
			}
		})
	}
}
