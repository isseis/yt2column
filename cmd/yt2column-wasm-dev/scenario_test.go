//go:build test

package main

import (
	"bytes"
	"testing"

	"github.com/isseis/yt2column/internal/llm/devfake"
)

// TestEmbeddedScenarioAccepted checks that each variable reaches its own
// value of the scenario: every value differs from the default, so a variable
// wired to another field, or not read, changes the parsed spelling.
func TestEmbeddedScenarioAccepted(t *testing.T) {
	want := devfake.ScenarioSpelling{
		Article: "long", Failure: "empty", FailAt: "shorten", DelaySeconds: "3", SettingsCheck: "key_rejected",
	}
	setEmbedded(t, want)
	scenario, err := embeddedScenario()
	if err != nil {
		t.Fatalf("embeddedScenario() error = %v", err)
	}
	if got := scenario.Spelling(); got != want {
		t.Errorf("embeddedScenario().Spelling() = %+v, want %+v", got, want)
	}
	var stderr bytes.Buffer
	if code := checkEmbedded(&stderr); code != exitOK || stderr.Len() != 0 {
		t.Errorf("checkEmbedded() = %d with stderr %q, want %d and nothing written", code, stderr.String(), exitOK)
	}
}

// TestEmbeddedScenarioRejected checks that a variable the build did not set
// stops the module instead of selecting a default. The other rejections are
// devfake.ParseScenario's, which its own tests cover.
func TestEmbeddedScenarioRejected(t *testing.T) {
	valid := devfake.ScenarioSpelling{Article: "standard", Failure: "none", FailAt: "none", DelaySeconds: "0", SettingsCheck: "ok"}
	unset := valid
	unset.SettingsCheck = ""
	setEmbedded(t, unset)
	var stderr bytes.Buffer
	if code := checkEmbedded(&stderr); code != exitInvalid || stderr.Len() == 0 {
		t.Errorf("checkEmbedded() = %d with stderr %q, want %d and a reason", code, stderr.String(), exitInvalid)
	}
}
