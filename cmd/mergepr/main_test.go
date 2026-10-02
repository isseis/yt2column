package main

import (
	"runtime/debug"
	"testing"
)

func TestRevisionFromSettings(t *testing.T) {
	cases := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"clean checkout", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "false"}}, "abc"},
		{"modified checkout", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"}}, ""},
		{"no modified flag", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}, "abc"},
		{"no revision", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := revisionFromSettings(tc.settings); got != tc.want {
				t.Errorf("revisionFromSettings = %q, want %q", got, tc.want)
			}
		})
	}
}
