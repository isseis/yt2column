//go:build test

package main

import (
	"testing"

	"github.com/isseis/yt2column/internal/llm/devfake"
)

// setEmbedded sets the five linker variables for one test and restores them.
func setEmbedded(t *testing.T, s devfake.ScenarioSpelling) {
	t.Helper()
	saved := devfake.ScenarioSpelling{
		Article: scenarioArticle, Failure: scenarioFailure, FailAt: scenarioFailAt,
		DelaySeconds: scenarioDelaySeconds, SettingsCheck: scenarioSettingsCheck,
	}
	t.Cleanup(func() {
		scenarioArticle, scenarioFailure, scenarioFailAt = saved.Article, saved.Failure, saved.FailAt
		scenarioDelaySeconds, scenarioSettingsCheck = saved.DelaySeconds, saved.SettingsCheck
	})
	scenarioArticle, scenarioFailure, scenarioFailAt = s.Article, s.Failure, s.FailAt
	scenarioDelaySeconds, scenarioSettingsCheck = s.DelaySeconds, s.SettingsCheck
}
