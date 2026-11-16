//go:build test

package devbackend

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

func scenarioOf(t *testing.T, s devfake.ScenarioSpelling) devfake.Scenario {
	t.Helper()
	scenario, err := devfake.ParseScenario(s)
	if err != nil {
		t.Fatalf("ParseScenario(%+v): %v", s, err)
	}
	return scenario
}

var standard = devfake.ScenarioSpelling{Article: "standard", Failure: "none", FailAt: "none", DelaySeconds: "0", SettingsCheck: "ok"}

// TestCheckKeyResults checks each settings check result of the scenario is
// what CheckKey returns: accepted, an error matching llm.ErrAuthRejected, and
// an error that matches neither it nor a nil.
func TestCheckKeyResults(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		wantNil  bool
		rejected bool
	}{{"ok", true, false}, {"key_rejected", false, true}, {"unverified", false, false}} {
		t.Run(tc.spelling, func(t *testing.T) {
			s := standard
			s.SettingsCheck = tc.spelling
			err := New(scenarioOf(t, s)).CheckKey(context.Background(), config.LLMConfig{})
			if (err == nil) != tc.wantNil || errors.Is(err, llm.ErrAuthRejected) != tc.rejected {
				t.Errorf("CheckKey = %v, want nil %v, rejected %v", err, tc.wantNil, tc.rejected)
			}
		})
	}
}

// TestNewClientsFollowsScenario checks the clients are the scenario's, not
// built from the configuration (the zero one is accepted), and that two runs
// get independent clients: a failing scenario fails the draft call of each.
func TestNewClientsFollowsScenario(t *testing.T) {
	s := standard
	s.Failure, s.FailAt = "api_key_rejected", "draft"
	b := New(scenarioOf(t, s))
	for run := range 2 {
		deps, err := b.NewClients(config.LLMConfig{})
		if err != nil {
			t.Fatalf("NewClients: %v", err)
		}
		if _, err := deps.Generation.Generate(context.Background(), llm.GenerateRequest{}); !errors.Is(err, llm.ErrAuthRejected) {
			t.Errorf("run %d: the first draft call error = %v, want a rejected key", run, err)
		}
	}
}

// TestInfoReportsScenario checks the declaration carries the scenario's five
// values, for each article and a non-zero delay.
func TestInfoReportsScenario(t *testing.T) {
	for _, article := range devfake.Articles() {
		s := devfake.ScenarioSpelling{Article: article.String(), Failure: "timeout", FailAt: "review", DelaySeconds: "7", SettingsCheck: "unverified"}
		want := extjob.FakeInfo(extjob.FakeScenario{Article: article.String(), Failure: "timeout", FailAt: "review", DelaySeconds: 7, SettingsCheck: "unverified"})
		if got := New(scenarioOf(t, s)).Info(); got != want {
			t.Errorf("%s: Info = %+v, want %+v", article, got, want)
		}
	}
}

// TestDependencies checks the development backend links neither the Claude
// adapter nor an external command. The network itself cannot be forbidden
// transitively: extjob checks the API key's text with llmhttp, which imports
// net/http. TestDirectImports forbids it in this package's own imports.
func TestDependencies(t *testing.T) {
	module := "github.com/isseis/yt2column"
	godepstest.Check(t, "./internal/extjob/devbackend",
		[]string{module + "/internal/extjob", module + "/internal/llm/devfake"},
		[]string{module + "/internal/llm/claude", "os/exec"})
}

// TestDirectImports checks the package's own (non-test) files import no
// network package and not the Claude adapter, so the fake path stays
// separate from the real one in the source, not only in the link.
func TestDirectImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", e.Name(), err)
		}
		checked++
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("import %s: %v", imp.Path.Value, err)
			}
			if path == "net" || strings.HasPrefix(path, "net/") || strings.HasSuffix(path, "/internal/llm/claude") {
				t.Errorf("%s imports %s", e.Name(), path)
			}
		}
	}
	if checked == 0 {
		t.Error("no source file was checked")
	}
}
