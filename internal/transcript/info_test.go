//go:build test

package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"unicode/utf8"
)

func TestParseInfoRealData(t *testing.T) {
	data := readTestdataFile(t, testdataRealInfo)

	info, err := parseInfo(testdataRealInfo, testdataRealVideoID, data)
	if err != nil {
		t.Fatalf("parseInfo(%s) error = %v", testdataRealInfo, err)
	}

	var oracle struct {
		Title       string `json:"title"`
		Channel     string `json:"channel"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatalf("oracle decode: %v", err)
	}
	if info.Title != oracle.Title {
		t.Errorf("Title = %q, want %q", info.Title, oracle.Title)
	}
	if info.ChannelName != oracle.Channel {
		t.Errorf("ChannelName = %q, want %q", info.ChannelName, oracle.Channel)
	}
	if info.Description != oracle.Description {
		t.Errorf("Description = %q, want %q", info.Description, oracle.Description)
	}
	if info.Title == "" || info.ChannelName == "" {
		t.Error("Title and ChannelName must be non-empty")
	}
}

func TestParseInfoRejectsMalformed(t *testing.T) {
	const valid = `{"id":"2tcCWM-sRBw","title":"t","channel":"c"}`
	cases := map[string]string{
		"truncated":        `{`,
		"not json":         `nope`,
		"top-level array":  `[]`,
		"top-level null":   `null`,
		"top-level number": `42`,
		"trailing object":  valid + ` {}`,
		"trailing text":    valid + ` x`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertInfoRejected(t, input)
		})
	}
}

func TestParseInfoMissingOrEmptyFields(t *testing.T) {
	rejected := map[string]string{
		"missing title":   `{"id":"2tcCWM-sRBw","channel":"c"}`,
		"empty title":     `{"id":"2tcCWM-sRBw","title":"","channel":"c"}`,
		"missing channel": `{"id":"2tcCWM-sRBw","title":"t"}`,
		"empty channel":   `{"id":"2tcCWM-sRBw","title":"t","channel":""}`,
	}
	for name, input := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			assertInfoRejected(t, input)
		})
	}

	accepted := map[string]string{
		"missing description": `{"id":"2tcCWM-sRBw","title":"t","channel":"c"}`,
		"empty description":   `{"id":"2tcCWM-sRBw","title":"t","channel":"c","description":""}`,
	}
	for name, input := range accepted {
		t.Run("accept/"+name, func(t *testing.T) {
			info, err := parseInfo("inline.info.json", testdataRealVideoID, []byte(input))
			if err != nil {
				t.Fatalf("parseInfo(%q) error = %v", input, err)
			}
			if info.Title != "t" || info.ChannelName != "c" || info.Description != "" {
				t.Errorf("parseInfo(%q) = %+v, want title t, channel c, empty description", input, info)
			}
		})
	}
}

func TestParseInfoIDMismatch(t *testing.T) {
	// otherVideoID exercises the wantID parameter with a value other than the
	// fixture's ID, matching the phase-3 caller that passes the requested ID.
	const otherVideoID = "aaaaaaaaaaa"
	cases := map[string]struct {
		wantID string
		input  string
	}{
		"different expected id": {otherVideoID, `{"id":"2tcCWM-sRBw","title":"t","channel":"c"}`},
		"mismatch":              {testdataRealVideoID, `{"id":"other","title":"t","channel":"c"}`},
		"missing":               {testdataRealVideoID, `{"title":"t","channel":"c"}`},
		"null":                  {testdataRealVideoID, `{"id":null,"title":"t","channel":"c"}`},
		"nonstring":             {testdataRealVideoID, `{"id":5,"title":"t","channel":"c"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertInfoRejectedWithID(t, tc.wantID, tc.input)
		})
	}
}

func TestParseInfoTypeMismatch(t *testing.T) {
	cases := map[string]string{
		"description null": `{"id":"2tcCWM-sRBw","title":"t","channel":"c","description":null}`,
		"title number":     `{"id":"2tcCWM-sRBw","title":1,"channel":"c"}`,
		"title null":       `{"id":"2tcCWM-sRBw","title":null,"channel":"c"}`,
		"channel array":    `{"id":"2tcCWM-sRBw","title":"t","channel":[]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertInfoRejected(t, input)
		})
	}
}

func TestParseInfoDuplicateMembers(t *testing.T) {
	rejected := map[string]string{
		"duplicate id":          `{"id":"2tcCWM-sRBw","id":"2tcCWM-sRBw","title":"t","channel":"c"}`,
		"duplicate title":       `{"id":"2tcCWM-sRBw","title":"t","title":"t","channel":"c"}`,
		"duplicate channel":     `{"id":"2tcCWM-sRBw","title":"t","channel":"c","channel":"c"}`,
		"duplicate description": `{"id":"2tcCWM-sRBw","title":"t","channel":"c","description":"","description":""}`,
	}
	for name, input := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			assertInfoRejected(t, input)
		})
	}

	t.Run("accept/unknown member", func(t *testing.T) {
		input := `{"id":"2tcCWM-sRBw","title":"t","channel":"c","foo":1,"foo":2}`
		info, err := parseInfo("inline.info.json", testdataRealVideoID, []byte(input))
		if err != nil {
			t.Fatalf("parseInfo error = %v", err)
		}
		if info.Title != "t" || info.ChannelName != "c" {
			t.Errorf("parseInfo = %+v, want title t, channel c", info)
		}
	})
}

func TestParseInfoLimits(t *testing.T) {
	base := []byte(`{"id":"2tcCWM-sRBw","title":"t","channel":"c"}`)

	t.Run("size at limit", func(t *testing.T) {
		info, err := parseInfo("inline.info.json", testdataRealVideoID, padToSize(base, maxInfoBytes))
		if err != nil {
			t.Fatalf("parseInfo at size limit error = %v", err)
		}
		if info.Title != "t" || info.ChannelName != "c" {
			t.Errorf("parseInfo = %+v, want title t, channel c", info)
		}
	})

	t.Run("size over limit", func(t *testing.T) {
		assertInfoRejected(t, string(padToSize(base, maxInfoBytes+1)))
	})
}

func TestParseInfoUTF8(t *testing.T) {
	t.Run("invalid UTF-8 sample", func(t *testing.T) {
		data := readTestdataFile(t, testdataInvalidUTF8Info)
		if utf8.Valid(data) {
			t.Fatal("sample is valid UTF-8, so it does not exercise the raw-byte check")
		}
		assertInfoRejected(t, string(data))
	})

	t.Run("unpaired surrogate sample", func(t *testing.T) {
		data := readTestdataFile(t, testdataSurrogateInfo)
		if !utf8.Valid(data) {
			t.Fatal("sample is not valid UTF-8, so it would be rejected for the wrong reason")
		}
		if !bytes.Contains(data, []byte(`\ud800`)) {
			t.Fatal("sample does not contain the expected surrogate escape")
		}
		assertInfoRejected(t, string(data))
	})

	t.Run("low surrogate", func(t *testing.T) {
		assertInfoRejected(t, `{"id":"2tcCWM-sRBw","title":"\udc00","channel":"c"}`)
	})

	t.Run("valid surrogate pair accepted", func(t *testing.T) {
		input := `{"id":"2tcCWM-sRBw","title":"\ud83d\ude00","channel":"c"}`
		info, err := parseInfo("inline.info.json", testdataRealVideoID, []byte(input))
		if err != nil {
			t.Fatalf("parseInfo error = %v", err)
		}
		if info.Title != "\U0001F600" {
			t.Errorf("Title = %q, want %q", info.Title, "\U0001F600")
		}
	})

	t.Run("escaped backslash accepted", func(t *testing.T) {
		input := `{"id":"2tcCWM-sRBw","title":"\\ud800","channel":"c"}`
		info, err := parseInfo("inline.info.json", testdataRealVideoID, []byte(input))
		if err != nil {
			t.Fatalf("parseInfo error = %v", err)
		}
		if info.Title != `\ud800` {
			t.Errorf("Title = %q, want %q", info.Title, `\ud800`)
		}
	})
}

func assertInfoRejected(t *testing.T, input string) {
	t.Helper()
	assertInfoRejectedWithID(t, testdataRealVideoID, input)
}

func assertInfoRejectedWithID(t *testing.T, wantID, input string) {
	t.Helper()
	info, err := parseInfo("inline.info.json", wantID, []byte(input))
	if !errors.Is(err, ErrParseInfo) {
		t.Fatalf("parseInfo(%q) error = %v, want ErrParseInfo", input, err)
	}
	if info != (videoInfo{}) {
		t.Errorf("parseInfo(%q) returned %+v on error", input, info)
	}
	parseErr, ok := errors.AsType[*ParseError](err)
	if !ok {
		t.Fatalf("parseInfo(%q) error is not *ParseError", input)
	}
	if parseErr.Path != "inline.info.json" {
		t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, "inline.info.json")
	}
}
