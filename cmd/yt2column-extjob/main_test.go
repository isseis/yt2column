//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// markerKey stands for an API key. It is printable ASCII, so the module
// accepts it, and a run must never write it.
const markerKey = "sk-ant-marker-5f1c0e7a" //nolint:gosec // a test marker, not a credential

// testWorkspaceID is a workspace ID claudeparam.ParseWorkspaceID accepts.
const testWorkspaceID = "wrkspc_01ABC"

// standardSpelling is the default scenario, which produces an article.
var standardSpelling = devfake.ScenarioSpelling{
	Article: "standard", Failure: "none", FailAt: "none", DelaySeconds: "0", SettingsCheck: "ok",
}

// fakeArgs returns the -llm=fake arguments of s.
func fakeArgs(s devfake.ScenarioSpelling) []string {
	return []string{
		"-llm=fake", "-article=" + s.Article, "-failure=" + s.Failure, "-fail-at=" + s.FailAt,
		"-delay=" + s.DelaySeconds, "-settings-check=" + s.SettingsCheck,
	}
}

// resultStatus reads the status member, and a failure or rejection reason, of
// the one JSON line on stdout.
func resultStatus(t *testing.T, stdout string) (status, reason string) {
	t.Helper()
	var got struct {
		Status  string `json:"status"`
		Failure string `json:"failure"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON value: %v; stdout:\n%s", err, stdout)
	}
	return got.Status, got.Failure + got.Reason
}

// TestRunCallsNoBackendBeforeAccepting checks that a run whose arguments,
// scenario, input file, or API key variable is not accepted constructs no
// LLM implementation, writes no result, and does not write the API key, even
// in a message that would hold it (an input path that contains it).
func TestRunCallsNoBackendBeforeAccepting(t *testing.T) {
	input := writeInput(t, testInput)
	withKey := map[string]string{"ANTHROPIC_API_KEY": markerKey}
	misspelled := standardSpelling
	misspelled.Article = "Standard"
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"no -llm", []string{"-input=" + input}, withKey},
		{"another -llm", []string{"-llm=deepseek", "-input=" + input}, withKey},
		{"fake with a misspelled scenario", append(fakeArgs(misspelled), "-input="+input), nil},
		{"fake without a scenario argument", append(fakeArgs(standardSpelling)[:5], "-input="+input), nil},
		{"claude with a scenario argument", []string{"-llm=claude", "-article=standard", "-input=" + input}, withKey},
		{"claude without ANTHROPIC_API_KEY", []string{"-llm=claude", "-input=" + input}, nil},
		{"no -input", fakeArgs(standardSpelling), nil},
		{"an extra word", append(fakeArgs(standardSpelling), "-input="+input, "extra"), nil},
		// Without the repeat rule the last -llm would win and run claude.
		{"a repeated -llm", []string{"-llm=fake", "-llm=claude", "-input=" + input}, withKey},
		{"a repeated scenario argument", append(fakeArgs(standardSpelling), "-article=long", "-input="+input), nil},
		{"a missing input file named after the key", []string{"-llm=claude", "-input=" + input + "." + markerKey}, withKey},
		{"a stray word holding the key", []string{"-llm=claude", input + "." + markerKey}, withKey},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingBackends{claudeScenario: mustScenario(t, standardSpelling)}
			got := runWith(t, tc.args, tc.env, r)
			if got.code != exitUsage {
				t.Errorf("run() = %d, want %d; stderr:\n%s", got.code, exitUsage, got.stderr)
			}
			if r.claudeCalls != 0 || len(r.fakeCalls) != 0 {
				t.Errorf("constructors called: claude %d, fake %d; want none", r.claudeCalls, len(r.fakeCalls))
			}
			if got.stdout != "" {
				t.Errorf("stdout = %q, want nothing", got.stdout)
			}
			if strings.Contains(got.stderr, markerKey) {
				t.Errorf("the API key reached stderr:\n%s", got.stderr)
			}
		})
	}
}

// TestRunFake checks that -llm=fake runs the scenario its arguments name and
// writes the module's result, with the exit code its status gives, and the
// progress events to stderr.
func TestRunFake(t *testing.T) {
	failing := devfake.ScenarioSpelling{Article: "long", Failure: "empty", FailAt: "shorten", DelaySeconds: "0", SettingsCheck: "unverified"}
	tests := []struct {
		name     string
		spelling devfake.ScenarioSpelling
		code     int
		status   string
	}{
		{"an article", standardSpelling, exitDone, "done"},
		{"a failure", failing, exitFailed, "failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingBackends{}
			got := runWith(t, append(fakeArgs(tc.spelling), "-input="+writeInput(t, testInput)), nil, r)
			if got.code != tc.code {
				t.Errorf("run() = %d, want %d; stderr:\n%s", got.code, tc.code, got.stderr)
			}
			if len(r.fakeCalls) != 1 || r.fakeCalls[0].Spelling() != tc.spelling {
				t.Fatalf("fake constructor calls = %+v, want one with %+v", r.fakeCalls, tc.spelling)
			}
			if r.claudeCalls != 0 {
				t.Errorf("claude constructor called %d times, want 0", r.claudeCalls)
			}
			if status, _ := resultStatus(t, got.stdout); status != tc.status {
				t.Errorf("result status = %q, want %q", status, tc.status)
			}
			if !strings.Contains(got.stderr, programName+": progress {") {
				t.Errorf("stderr has no progress event:\n%s", got.stderr)
			}
		})
	}
}

// TestRunInputRejected checks that an input the module does not accept,
// including one over its size limit, is reported with the module's rejection
// and generates nothing. The over-limit file is a valid request of exactly the
// limit followed by one space, so a read that stopped at the limit would pass
// the module a request it accepts.
func TestRunInputRejected(t *testing.T) {
	atLimit := strings.Repeat(" ", extjob.MaxPrepareRequestBytes-len(testInput)) + testInput
	tests := map[string]string{
		"not a prepare request": `{"videoUrl":1}`,
		"over the size limit":   atLimit + " ",
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			r := &recordingBackends{}
			got := runWith(t, append(fakeArgs(standardSpelling), "-input="+writeInput(t, data)), nil, r)
			if got.code != exitFailed {
				t.Errorf("run() = %d, want %d; stderr:\n%s", got.code, exitFailed, got.stderr)
			}
			if status, reason := resultStatus(t, got.stdout); status != "rejected" || reason != "invalid_body" {
				t.Errorf("result = %q %q, want rejected invalid_body", status, reason)
			}
			if strings.Contains(got.stderr, programName+": progress ") {
				t.Errorf("stderr reports progress for a rejected input:\n%s", got.stderr)
			}
		})
	}
}

// TestRunClaudeSettings checks the settings a -llm=claude run gives the
// module: the API key from the environment, and the workspace ID, which
// reaches the backend as no workspace when its variable is unset.
func TestRunClaudeSettings(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		wantWorkspace string
		wantSet       bool
	}{
		{"workspace unset", map[string]string{"ANTHROPIC_API_KEY": markerKey}, "", false},
		{"workspace set", map[string]string{"ANTHROPIC_API_KEY": markerKey, "ANTHROPIC_WORKSPACE_ID": testWorkspaceID}, testWorkspaceID, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingBackends{claudeScenario: mustScenario(t, standardSpelling)}
			got := runWith(t, []string{"-llm=claude", "-input=" + writeInput(t, testInput)}, tc.env, r)
			if got.code != exitDone {
				t.Fatalf("run() = %d, want %d; stdout:\n%s\nstderr:\n%s", got.code, exitDone, got.stdout, got.stderr)
			}
			if r.claudeCalls != 1 || len(r.fakeCalls) != 0 || len(r.claude.configs) != 1 {
				t.Fatalf("constructors: claude %d, fake %d; want only claude once", r.claudeCalls, len(r.fakeCalls))
			}
			cfg := r.claude.configs[0]
			if key, err := cfg.AnthropicAPIKey().Reveal(); err != nil || key != markerKey {
				t.Errorf("API key given to the backend is not the environment's (err %v)", err)
			}
			if workspace, set := cfg.AnthropicWorkspaceID().Value(); workspace != tc.wantWorkspace || set != tc.wantSet {
				t.Errorf("workspace ID = %q, %v; want %q, %v", workspace, set, tc.wantWorkspace, tc.wantSet)
			}
		})
	}
}

// TestRunClaudeKeepsKeyOutOfOutput checks that the API key from the
// environment is written to neither stdout nor stderr, on the path that
// produces an article and on each path that rejects or fails.
func TestRunClaudeKeepsKeyOutOfOutput(t *testing.T) {
	// A key with a space is not printable ASCII without spaces, so the
	// module rejects the settings; it is still a marker to search for.
	const invalidKey = markerKey + " x"
	rejectedKey := devfake.ScenarioSpelling{Article: "standard", Failure: "api_key_rejected", FailAt: "draft", DelaySeconds: "0", SettingsCheck: "ok"}
	tests := []struct {
		name     string
		key      string
		scenario devfake.ScenarioSpelling
		input    string
		code     int
		result   string
	}{
		{"an article", markerKey, standardSpelling, testInput, exitDone, "done"},
		{"the API rejects the key", markerKey, rejectedKey, testInput, exitFailed, "failed api_key_rejected"},
		{"the settings are invalid", invalidKey, standardSpelling, testInput, exitFailed, "failed settings_invalid"},
		{"the input is rejected", markerKey, standardSpelling, `{}`, exitFailed, "rejected invalid_body"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingBackends{claudeScenario: mustScenario(t, tc.scenario)}
			env := map[string]string{"ANTHROPIC_API_KEY": tc.key}
			got := runWith(t, []string{"-llm=claude", "-input=" + writeInput(t, tc.input)}, env, r)
			if got.code != tc.code {
				t.Errorf("run() = %d, want %d; stderr:\n%s", got.code, tc.code, got.stderr)
			}
			if status, reason := resultStatus(t, got.stdout); strings.TrimSpace(status+" "+reason) != tc.result {
				t.Errorf("result = %q %q, want %q", status, reason, tc.result)
			}
			if strings.Contains(got.stdout, markerKey) || strings.Contains(got.stderr, markerKey) {
				t.Errorf("the API key reached the output:\nstdout:\n%s\nstderr:\n%s", got.stdout, got.stderr)
			}
		})
	}
}

// TestRunClaudeEmptyKey checks that an API key variable that is set but
// empty is passed on, not treated as unset: the module rejects the settings,
// and the run ends with a failed result rather than a usage error.
func TestRunClaudeEmptyKey(t *testing.T) {
	r := &recordingBackends{claudeScenario: mustScenario(t, standardSpelling)}
	got := runWith(t, []string{"-llm=claude", "-input=" + writeInput(t, testInput)}, map[string]string{"ANTHROPIC_API_KEY": ""}, r)
	if got.code != exitFailed {
		t.Errorf("run() = %d, want %d; stderr:\n%s", got.code, exitFailed, got.stderr)
	}
	if status, reason := resultStatus(t, got.stdout); status != "failed" || reason != "settings_invalid" {
		t.Errorf("result = %q %q, want failed settings_invalid", status, reason)
	}
}

// TestRunClaudeWarnsOfHTTP2Debug checks that a -llm=claude run warns, before
// it calls the API, when GODEBUG turns on the HTTP/2 log, which may write the
// key's header to stderr, and does not warn otherwise.
func TestRunClaudeWarnsOfHTTP2Debug(t *testing.T) {
	for _, tc := range []struct {
		name    string
		godebug string
		warn    bool
	}{
		{"http2debug on", "http2debug=2", true},
		{"GODEBUG without http2debug", "panicnil=1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"ANTHROPIC_API_KEY": markerKey, "GODEBUG": tc.godebug}
			r := &recordingBackends{claudeScenario: mustScenario(t, standardSpelling)}
			got := runWith(t, []string{"-llm=claude", "-input=" + writeInput(t, testInput)}, env, r)
			if got.code != exitDone {
				t.Fatalf("run() = %d, want %d; stderr:\n%s", got.code, exitDone, got.stderr)
			}
			// The warning's wording is what the developer reads; its mention
			// of http2debug is the observable contract.
			if warned := strings.Contains(got.stderr, "warning: GODEBUG enables http2debug"); warned != tc.warn {
				t.Errorf("warned = %v, want %v; stderr:\n%s", warned, tc.warn, got.stderr)
			}
		})
	}
}

// failingWriter fails every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// TestRunResultNotWritten checks that a generation whose result cannot be
// written to stdout does not end with the success code.
func TestRunResultNotWritten(t *testing.T) {
	var stderr bytes.Buffer
	r := &recordingBackends{}
	args := append(fakeArgs(standardSpelling), "-input="+writeInput(t, testInput))
	if code := run(context.Background(), args, lookupOf(nil), failingWriter{}, &stderr, r.backends()); code != exitFailed {
		t.Errorf("run() = %d, want %d; stderr:\n%s", code, exitFailed, stderr.String())
	}
}
