package slackwebhook

import (
	"net/url"
	"slices"
	"testing"
)

func TestValidURL(t *testing.T) {
	accepted := []string{
		"https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx",
		"https://hooks.slack.com/services/T000/B000/XXXX",
		"https://hooks.slack.com.example/services/x",
		"https://HOOKS.SLACK.COM/services/x",
		"https://hooks.slack.com/",
		"HTTPS://example.com/x",
	}
	for i, value := range accepted {
		t.Run("accepted "+string(rune('a'+i)), func(t *testing.T) {
			if !ValidURL(value) {
				t.Errorf("ValidURL(%q) = false, want true", value)
			}
		})
	}

	rejected := []string{
		"http://mattermost.example.com/hooks/x",
		"mattermost.example.com/hooks/x",
		"https:///hooks/x",
		"https://mattermost.example.com/hooks/x\n",
		"https://hooks.slack.com/services/%zz",
		"https://hooks.slack.com/%",
		"https://hooks.slack.com/services/x\x7f",
		"https://:443/x",
		"",
	}
	for i, value := range rejected {
		t.Run("rejected "+string(rune('a'+i)), func(t *testing.T) {
			if ValidURL(value) {
				t.Errorf("ValidURL(%q) = true, want false", value)
			}
		})
	}
}

func TestSensitiveParts(t *testing.T) {
	t.Run("non-ascii path and query", func(t *testing.T) {
		const value = "https://mattermost.example.com/hooks/p\u00e4th/segment-\u00e5bc?q=\u00fc&x=1"
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatalf("url.Parse(%q) error = %v", value, err)
		}
		parts := SensitiveParts(value)
		for _, want := range []string{
			value,
			parsed.String(),
			parsed.Path,
			parsed.EscapedPath(),
			parsed.RawQuery,
			parsed.Query().Encode(),
			"segment-\u00e5bc",
		} {
			if !slices.Contains(parts, want) {
				t.Errorf("SensitiveParts(%q) = %q, want it to contain %q", value, parts, want)
			}
		}
		if parsed.EscapedPath() == parsed.Path {
			t.Fatalf("EscapedPath() = %q, want a form that differs from Path", parsed.Path)
		}
		if parsed.Query().Encode() == parsed.RawQuery {
			t.Fatalf("Query().Encode() = %q, want a form that differs from RawQuery", parsed.RawQuery)
		}
	})

	t.Run("userinfo as given and as written", func(t *testing.T) {
		const value = "https://us%65rname:secret@mattermost.example.com/hooks/segment-abcdefgh?raw=a%20b"
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatalf("url.Parse(%q) error = %v", value, err)
		}
		if parsed.User.String() == "us%65rname:secret" {
			t.Fatal("url.User.String did not re-encode the userinfo; the test no longer exercises two forms")
		}
		parts := SensitiveParts(value)
		for _, want := range []string{"us%65rname:secret", parsed.User.String()} {
			if !slices.Contains(parts, want) {
				t.Errorf("SensitiveParts(%q) = %q, want it to contain %q", value, parts, want)
			}
		}
	})

	t.Run("last 8 characters by rune and by byte", func(t *testing.T) {
		const value = "https://mattermost.example.com/hooks/\u65e5\u672c\u8a9e\u65e5\u672c\u8a9e\u65e5\u672c\u8a9e"
		runes := []rune(value)
		runeTail := string(runes[len(runes)-exposedTail:])
		byteTail := value[len(value)-exposedTail:]
		if runeTail == byteTail {
			t.Fatalf("rune tail %q equals byte tail; the test no longer exercises both", runeTail)
		}
		parts := SensitiveParts(value)
		for _, want := range []string{runeTail, byteTail} {
			if !slices.Contains(parts, want) {
				t.Errorf("SensitiveParts(%q) = %q, want it to contain %q", value, parts, want)
			}
		}
	})

	t.Run("omits parts shorter than the exposed tail", func(t *testing.T) {
		for _, value := range []string{"https://host/a", "https://host/"} {
			parts := SensitiveParts(value)
			for _, short := range []string{"a", "/"} {
				if slices.Contains(parts, short) {
					t.Errorf("SensitiveParts(%q) = %q, must not contain %q", value, parts, short)
				}
			}
			for _, part := range parts {
				if len(part) < exposedTail {
					t.Errorf("SensitiveParts(%q) = %q, has a part shorter than %d bytes", value, parts, exposedTail)
				}
			}
		}
	})

	t.Run("empty value", func(t *testing.T) {
		if parts := SensitiveParts(""); parts != nil {
			t.Errorf("SensitiveParts(\"\") = %q, want nil", parts)
		}
	})
}
