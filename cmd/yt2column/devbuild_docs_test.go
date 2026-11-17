//go:build test

package main

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// makefileFakeVarRE matches a scenario variable of the extension's development
// build in the Makefile, with its default value.
var makefileFakeVarRE = regexp.MustCompile(`(?m)^(FAKE_[A-Z_]+) = (\S*)$`)

// spellings returns the spellings of values, as devfake lists them.
func spellings[T interface{ String() string }](values []T) []string {
	return strings.Split(devfake.Spellings(values), "|")
}

// TestDevelopmentDocsDevBuild pins development.md's table of the development
// build's scenario variables to the code: each variable the Makefile defines
// has exactly one row and no row names another; the accepted values of each
// row are exactly the spellings devfake.ParseScenario accepts for that value
// ("none" included for the failure and the failing call, the two endpoints
// for the delay); and each row's default is the Makefile's. The Makefile's
// defaults must also make a scenario ParseScenario accepts, so a default the
// build would reject fails here rather than in every make ext-build-dev.
func TestDevelopmentDocsDevBuild(t *testing.T) {
	// Which scenario value each Makefile variable sets, and its accepted
	// spellings. The variables' names are the Makefile's own; the test
	// fails when the Makefile defines another or loses one.
	accepted := map[string][]string{
		"FAKE_ARTICLE":        spellings(devfake.Articles()),
		"FAKE_FAILURE":        append(spellings(devfake.Failures()), devfake.FailureNone.String()),
		"FAKE_FAIL_AT":        append(spellings(devfake.FailAts()), devfake.FailAtNone.String()),
		"FAKE_DELAY":          {"0", strconv.FormatUint(devfake.MaxDelaySeconds, 10)},
		"FAKE_SETTINGS_CHECK": spellings(devfake.SettingsChecks()),
	}

	defaults := map[string]string{}
	for _, m := range makefileFakeVarRE.FindAllStringSubmatch(doctestutil.ReadDoc(t, makefilePath), -1) {
		if _, dup := defaults[m[1]]; dup {
			t.Errorf("the Makefile defines %s more than once", m[1])
		}
		defaults[m[1]] = m[2]
	}
	if got, want := slices.Sorted(maps.Keys(defaults)), slices.Sorted(maps.Keys(accepted)); !slices.Equal(got, want) {
		t.Fatalf("the Makefile's scenario variables are %v, want %v", got, want)
	}
	if _, err := devfake.ParseScenario(devfake.ScenarioSpelling{
		Article:       defaults["FAKE_ARTICLE"],
		Failure:       defaults["FAKE_FAILURE"],
		FailAt:        defaults["FAKE_FAIL_AT"],
		DelaySeconds:  defaults["FAKE_DELAY"],
		SettingsCheck: defaults["FAKE_SETTINGS_CHECK"],
	}); err != nil {
		t.Errorf("the Makefile's default scenario is rejected: %v", err)
	}

	development := doctestutil.ReadDoc(t, developmentDocPath)
	section := doctestutil.SectionOutsideBlocks(development, doctestutil.DevBuildHeading)
	if section == "" {
		t.Fatalf("development.md has no %q section", doctestutil.DevBuildHeading)
	}
	table := doctestutil.FindTable(section, doctestutil.DevVariableColumn)
	if table == nil {
		t.Fatalf("development.md %q has no variables table", doctestutil.DevBuildHeading)
	}
	acceptedCol := doctestutil.ColumnIndex(table[0], doctestutil.DevAcceptedColumn)
	defaultCol := doctestutil.ColumnIndex(table[0], doctestutil.DevDefaultColumn)
	if acceptedCol < 0 || defaultCol < 0 {
		t.Fatalf("development.md %q variables table lacks the %q or %q column", doctestutil.DevBuildHeading, doctestutil.DevAcceptedColumn, doctestutil.DevDefaultColumn)
	}

	rows := map[string]int{}
	for _, row := range table[1:] {
		if len(row) <= max(acceptedCol, defaultCol) {
			t.Errorf("development.md %q has a short row %q", doctestutil.DevBuildHeading, row)
			continue
		}
		name := doctestutil.StripBackticks(row[0])
		want, ok := accepted[name]
		if !ok {
			t.Errorf("development.md %q has a row for %q, which the Makefile does not define", doctestutil.DevBuildHeading, name)
			continue
		}
		rows[name]++
		var got []string
		for _, m := range codeSpanRE.FindAllStringSubmatch(row[acceptedCol], -1) {
			got = append(got, m[1])
		}
		if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("development.md %s accepts %v, want exactly %v", name, got, want)
		}
		if got := doctestutil.StripBackticks(row[defaultCol]); got != defaults[name] {
			t.Errorf("development.md %s defaults to %q, the Makefile to %q", name, got, defaults[name])
		}
	}
	for name := range accepted {
		if rows[name] != 1 {
			t.Errorf("development.md %q has %d rows for %s, want 1", doctestutil.DevBuildHeading, rows[name], name)
		}
	}
}
