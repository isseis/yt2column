//go:build test

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/extjob/devbackend"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// testInput is a prepare request the module accepts.
const testInput = `{"videoUrl":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","transcript":"0:00\nHello\n","title":"T"}`

// recordingBackend wraps a Backend and records the configurations its
// NewClients is given.
type recordingBackend struct {
	inner   extjob.Backend
	configs []config.LLMConfig
}

func (b *recordingBackend) NewClients(cfg config.LLMConfig) (extjob.Deps, error) {
	b.configs = append(b.configs, cfg)
	return b.inner.NewClients(cfg)
}

func (b *recordingBackend) CheckKey(ctx context.Context, cfg config.LLMConfig) error {
	return b.inner.CheckKey(ctx, cfg)
}

func (b *recordingBackend) Info() extjob.BackendInfo { return b.inner.Info() }

// recordingBackends counts the calls of the two constructors. The claude
// constructor returns a development backend of claudeScenario wrapped in a
// recordingBackend, so a -llm=claude run calls no API; the fake constructor
// records the scenario it is given.
type recordingBackends struct {
	claudeScenario devfake.Scenario
	claudeCalls    int
	claude         *recordingBackend
	fakeCalls      []devfake.Scenario
}

func (r *recordingBackends) backends() backends {
	return backends{
		claude: func() extjob.Backend {
			r.claudeCalls++
			r.claude = &recordingBackend{inner: devbackend.New(r.claudeScenario)}
			return r.claude
		},
		fake: func(s devfake.Scenario) extjob.Backend {
			r.fakeCalls = append(r.fakeCalls, s)
			return devbackend.New(s)
		},
	}
}

// mustScenario parses a scenario the test needs.
func mustScenario(t *testing.T, s devfake.ScenarioSpelling) devfake.Scenario {
	t.Helper()
	scenario, err := devfake.ParseScenario(s)
	if err != nil {
		t.Fatalf("ParseScenario(%+v) error = %v", s, err)
	}
	return scenario
}

// writeInput writes data to a file in the test's directory and returns its
// path.
func writeInput(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// lookupOf answers the names in env and no other.
func lookupOf(env map[string]string) config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// runResult is what one run returned and wrote.
type runResult struct {
	code           int
	stdout, stderr string
}

// runWith runs the command with args, the environment env, and the recording
// constructors r.
func runWith(t *testing.T, args []string, env map[string]string, r *recordingBackends) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, lookupOf(env), &stdout, &stderr, r.backends())
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}
