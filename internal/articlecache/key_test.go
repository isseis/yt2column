//go:build test

package articlecache

import (
	"errors"
	"strings"
	"testing"
)

var testSecret = [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

func baseKeyInput() KeyInput {
	return KeyInput{
		VideoURL:    "https://www.youtube.com/watch?v=abcdefghijk",
		Transcript:  "0:00\nhello",
		Title:       "title",
		ChannelName: "channel",
		Description: "description",
	}
}

func TestKeySameInputSameKey(t *testing.T) {
	keyer := NewKeyer(testSecret)
	if a, b := keyer.Key(baseKeyInput()), keyer.Key(baseKeyInput()); a != b {
		t.Errorf("Key differs for the same input: %s and %s", a, b)
	}
}

func TestKeyChangesWithEachValue(t *testing.T) {
	keyer := NewKeyer(testSecret)
	base := keyer.Key(baseKeyInput())
	cases := map[string]func(*KeyInput){
		"video url":    func(in *KeyInput) { in.VideoURL = "https://www.youtube.com/watch?v=abcdefghijl" },
		"transcript":   func(in *KeyInput) { in.Transcript += "!" },
		"title":        func(in *KeyInput) { in.Title = "" },
		"channel name": func(in *KeyInput) { in.ChannelName = "channel2" },
		"description":  func(in *KeyInput) { in.Description = "Description" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := baseKeyInput()
			change(&in)
			if got := keyer.Key(in); got == base {
				t.Errorf("Key unchanged after changing %s", name)
			}
		})
	}
}

// Each pair keeps the concatenation of every value the same and moves one
// boundary, so only the length prefix tells the inputs apart.
func TestKeyBoundaryShift(t *testing.T) {
	keyer := NewKeyer(testSecret)
	cases := map[string][2]KeyInput{
		"title and channel name": {
			{VideoURL: "u", Transcript: "t", Title: "ab", ChannelName: "c"},
			{VideoURL: "u", Transcript: "t", Title: "a", ChannelName: "bc"},
		},
		"transcript and title": {
			{VideoURL: "u", Transcript: "tx", Title: ""},
			{VideoURL: "u", Transcript: "t", Title: "x"},
		},
		"channel name and description": {
			{VideoURL: "u", Transcript: "t", ChannelName: "", Description: "d"},
			{VideoURL: "u", Transcript: "t", ChannelName: "d", Description: ""},
		},
	}
	for name, pair := range cases {
		t.Run(name, func(t *testing.T) {
			if keyer.Key(pair[0]) == keyer.Key(pair[1]) {
				t.Error("moving a boundary between two values gave the same key")
			}
		})
	}
}

func TestKeyUsesSecret(t *testing.T) {
	other := testSecret
	other[0] ^= 0xff
	if NewKeyer(testSecret).Key(baseKeyInput()) == NewKeyer(other).Key(baseKeyInput()) {
		t.Error("two secrets gave the same key for one input")
	}
}

func TestParseKeyRoundTrip(t *testing.T) {
	key := NewKeyer(testSecret).Key(baseKeyInput())
	parsed, err := ParseKey(key.String())
	if err != nil {
		t.Fatalf("ParseKey(String()) error = %v", err)
	}
	if parsed != key {
		t.Errorf("ParseKey(String()) = %s, want %s", parsed, key)
	}
}

func TestParseKeyRejects(t *testing.T) {
	valid := NewKeyer(testSecret).Key(baseKeyInput()).String()
	cases := map[string]string{
		"63 characters":   valid[:63],
		"65 characters":   valid + "0",
		"uppercase":       strings.ToUpper(valid[:1]) + "A" + valid[2:],
		"leading space":   " " + valid[1:],
		"trailing space":  valid[:63] + " ",
		"surrounded":      " " + valid + " ",
		"non-hexadecimal": "g" + valid[1:],
		"non-ascii":       "\u00e9" + valid[2:],
		"empty":           "",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseKey(input)
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("ParseKey(%q) error = %v, want ErrInvalidKey", input, err)
			}
			// The error text is the contract here: it must never carry the
			// untrusted string, so the test reads the text itself.
			if input != "" && strings.Contains(err.Error(), input) {
				t.Errorf("ParseKey error %q holds the input", err)
			}
		})
	}
}
