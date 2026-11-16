//go:build test

package extjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

const (
	testKey         = "sk-ant-TESTKEY-marker-7f3a"
	testWorkspaceID = "wrkspc_01ABC"
)

// settingsRequest builds a checkSettings request from raw JSON values.
func settingsRequest(key, workspace string) string {
	return `{"anthropicApiKey":` + key + `,"anthropicWorkspaceId":` + workspace + `}`
}

func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestCheckSettingsRejectsShape checks each rule of the stored settings'
// shape: it answers rejected with the offending fields, and never calls the
// Backend (so a malformed value is never sent to the API).
func TestCheckSettingsRejectsShape(t *testing.T) {
	both := `"anthropicApiKey","anthropicWorkspaceId"`
	cases := []struct{ name, request, fields string }{
		{"key empty", settingsRequest(`""`, `""`), `"anthropicApiKey"`},
		{"key with a space", settingsRequest(quote("sk ant"), `""`), `"anthropicApiKey"`},
		{"key with a newline", settingsRequest(quote("sk\nant"), `""`), `"anthropicApiKey"`},
		{"key with a non-ASCII character", settingsRequest(quote("sk-\u00e9"), `""`), `"anthropicApiKey"`},
		{"key padded", settingsRequest(quote(" "+testKey), `""`), `"anthropicApiKey"`},
		{"key not a string", settingsRequest(`1`, `""`), `"anthropicApiKey"`},
		{"key null", settingsRequest(`null`, `""`), `"anthropicApiKey"`},
		{"workspace padded", settingsRequest(quote(testKey), quote(" "+testWorkspaceID)), `"anthropicWorkspaceId"`},
		{"workspace with a space", settingsRequest(quote(testKey), quote("wrk spc")), `"anthropicWorkspaceId"`},
		{"workspace not a string", settingsRequest(quote(testKey), `5`), `"anthropicWorkspaceId"`},
		{"both", settingsRequest(quote("a b"), quote("c d")), both},
		{"missing workspace", `{"anthropicApiKey":` + quote(testKey) + `}`, `"anthropicWorkspaceId"`},
		{"unknown member", settingsRequest(quote(testKey), `""`)[:len(settingsRequest(quote(testKey), `""`))-1] + `,"x":1}`, both},
		{"not an object", `[]`, both},
		{"not json", `x`, both},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{}
			got := NewModule(b).CheckSettingsJSON(context.Background(), tc.request)
			if want := `{"status":"rejected","fields":[` + tc.fields + `]}`; got != want {
				t.Errorf("result = %s, want %s", got, want)
			}
			if b.checkCalls != 0 {
				t.Errorf("CheckKey called %d times for a rejected shape, want 0", b.checkCalls)
			}
		})
	}
}

// TestCheckSettingsOutcomes checks the four results and what the Backend is
// asked: the shape is accepted first and the call gets the checked values and
// a 30-second deadline. An empty workspace ID is none, accepted.
func TestCheckSettingsOutcomes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ok", nil, `{"status":"ok"}`},
		{"key rejected", fmt.Errorf("claude: %w", llm.ErrAuthRejected), `{"status":"key_rejected"}`},
		{"other status", errors.New("http 500"), `{"status":"unverified"}`},
		{"deadline", context.DeadlineExceeded, `{"status":"unverified"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{checkErr: tc.err}
			if got := NewModule(b).CheckSettingsJSON(context.Background(), settingsRequest(quote(testKey), `""`)); got != tc.want {
				t.Errorf("result = %s, want %s", got, tc.want)
			}
			if b.checkCalls != 1 {
				t.Fatalf("CheckKey called %d times, want 1", b.checkCalls)
			}
			deadline, ok := b.checkCtx.Deadline()
			if !ok || time.Until(deadline) > keyCheckTimeout || time.Until(deadline) < keyCheckTimeout-5*time.Second {
				t.Errorf("deadline = %v (set %v), want about %v from now", deadline, ok, keyCheckTimeout)
			}
		})
	}
}

// TestCheckSettingsPassesCheckedValues checks the configuration the Backend
// receives holds the key and the workspace ID as given, and none when the ID
// is empty.
func TestCheckSettingsPassesCheckedValues(t *testing.T) {
	b := &fakeBackend{}
	m := NewModule(b)
	m.CheckSettingsJSON(context.Background(), settingsRequest(quote(testKey), quote(testWorkspaceID)))
	if got, err := b.lastConfig.AnthropicAPIKey().Reveal(); err != nil || got != testKey {
		t.Errorf("key = %q, %v; want the one sent", got, err)
	}
	if _, set := b.lastConfig.AnthropicWorkspaceID().Value(); !set {
		t.Error("the workspace ID was dropped")
	}
	m.CheckSettingsJSON(context.Background(), settingsRequest(quote(testKey), `""`))
	if _, set := b.lastConfig.AnthropicWorkspaceID().Value(); set {
		t.Error("an empty workspace ID was sent as a workspace")
	}
}

// preparedModule returns a module of backend b that prepared the standard
// input, and its digest.
func preparedModule(t *testing.T, b Backend) (*Module, string) {
	t.Helper()
	m := NewModule(b)
	var got prepareResult
	if err := json.Unmarshal([]byte(m.PrepareJSON(`{`+validMembers+`}`)), &got); err != nil || got.Status != "ok" {
		t.Fatalf("prepare: %+v, %v", got, err)
	}
	return m, got.InputDigest
}

func generateBody(digest, settings string) string {
	return `{"inputDigest":` + quote(digest) + `,"settings":` + settings + `}`
}

func decodeResult(t *testing.T, data string) resultJSON {
	t.Helper()
	var got resultJSON
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return got
}

var pendingSteps = stepsJSON{Draft: "pending", Shorten: "pending", Review: "pending"}

// TestGenerateRejectsShape checks the generate request is a closed set:
// each malformed request is an internal failure with request_shape, with every
// step pending, and nothing reaches the Backend.
func TestGenerateRejectsShape(t *testing.T) {
	settings := `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":""}`
	cases := map[string]string{
		"not json":                      `x`,
		"not object":                    `[]`,
		"missing digest":                `{"settings":` + settings + `}`,
		"missing settings":              `{"inputDigest":"x"}`,
		"digest not a string":           `{"inputDigest":1,"settings":` + settings + `}`,
		"settings not an object":        `{"inputDigest":"x","settings":"s"}`,
		"unknown member":                `{"inputDigest":"x","settings":` + settings + `,"extra":1}`,
		"duplicate member":              `{"inputDigest":"x","inputDigest":"y","settings":` + settings + `}`,
		"unknown settings member":       `{"inputDigest":"x","settings":{"anthropicApiKey":"k","anthropicWorkspaceId":"","model":"m"}}`,
		"duplicate settings key":        `{"inputDigest":"x","settings":{"anthropicApiKey":"k","anthropicApiKey":"j"}}`,
		"missing api key":               `{"inputDigest":"x","settings":{"anthropicWorkspaceId":""}}`,
		"legacy flat credentials":       `{"inputDigest":"x","anthropicApiKey":"k","anthropicWorkspaceId":""}`,
		"settings with trailing":        `{"inputDigest":"x","settings":` + settings + `}{}`,
		"settings null":                 `{"inputDigest":"x","settings":null}`,
		"unpaired surrogate":            `{"inputDigest":"\ud800","settings":` + settings + `}`,
		"settings with a missing value": `{"inputDigest":"x","settings":{"anthropicApiKey":,"anthropicWorkspaceId":""}}`,
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			b := &fakeBackend{}
			m, _ := preparedModule(t, b)
			got := decodeResult(t, m.GenerateJSON(context.Background(), request, nil))
			if got.Status != "failed" || got.Failure != "internal" || got.InternalDetail != DetailRequestShape || got.Steps != pendingSteps {
				t.Errorf("result = %+v, want internal request_shape with pending steps", got)
			}
			if b.clientCalls != 0 {
				t.Error("the Backend built clients for a malformed request")
			}
		})
	}
}

// TestGenerateInputMismatch checks a digest that is not the kept input's, and
// a generate with nothing kept, is an internal failure with input_mismatch and
// no client built.
func TestGenerateInputMismatch(t *testing.T) {
	settings := `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":""}`
	b := &fakeBackend{}
	m, digest := preparedModule(t, b)
	other := strings.Repeat("0", 64)
	if other == digest {
		t.Fatal("test digest collides")
	}
	for name, module := range map[string]*Module{"other digest": m, "nothing kept": NewModule(b)} {
		t.Run(name, func(t *testing.T) {
			got := decodeResult(t, module.GenerateJSON(context.Background(), generateBody(other, settings), nil))
			if got.Failure != "internal" || got.InternalDetail != DetailInputMismatch || got.Steps != pendingSteps {
				t.Errorf("result = %+v, want internal input_mismatch", got)
			}
		})
	}
	if b.clientCalls != 0 {
		t.Error("clients were built for a mismatched input")
	}
}

// TestGenerateSettingsInvalid checks each rule of the stored settings at
// generate: settings_invalid, every step pending, and the Backend never asked
// for clients. A value is never repaired: an absent or padded one fails.
func TestGenerateSettingsInvalid(t *testing.T) {
	cases := map[string]string{
		"key not a string":       `{"anthropicApiKey":1,"anthropicWorkspaceId":""}`,
		"key null":               `{"anthropicApiKey":null,"anthropicWorkspaceId":""}`,
		"key empty":              `{"anthropicApiKey":"","anthropicWorkspaceId":""}`,
		"key padded":             `{"anthropicApiKey":" k ","anthropicWorkspaceId":""}`,
		"key with a space":       `{"anthropicApiKey":"a b","anthropicWorkspaceId":""}`,
		"workspace absent":       `{"anthropicApiKey":` + quote(testKey) + `}`,
		"workspace not a string": `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":2}`,
		"workspace null":         `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":null}`,
		"workspace padded":       `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":" w"}`,
	}
	for name, settings := range cases {
		t.Run(name, func(t *testing.T) {
			b := &fakeBackend{}
			m, digest := preparedModule(t, b)
			got := decodeResult(t, m.GenerateJSON(context.Background(), generateBody(digest, settings), nil))
			if got.Status != "failed" || got.Failure != "settings_invalid" || got.Steps != pendingSteps || got.InternalDetail != DetailNone {
				t.Errorf("result = %+v, want settings_invalid with pending steps", got)
			}
			if b.clientCalls != 0 {
				t.Error("the Backend built clients for invalid settings")
			}
		})
	}
}

// TestGenerateAcceptsEmptyWorkspace checks an empty workspace ID is the
// choice of none: the run goes on and the clients are built with no workspace.
func TestGenerateAcceptsEmptyWorkspace(t *testing.T) {
	scenario, err := devfake.NewScenario(devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, 0)
	if err != nil {
		t.Fatalf("NewScenario: %v", err)
	}
	job := scenario.NewJob()
	b := &fakeBackend{deps: Deps{Generation: job.Generation(), Review: job.Review()}}
	m, digest := preparedModule(t, b)
	got := decodeResult(t, m.GenerateJSON(context.Background(), generateBody(digest, `{"anthropicApiKey":`+quote(testKey)+`,"anthropicWorkspaceId":""}`), nil))
	if got.Status != "done" || got.Article == nil {
		t.Fatalf("result = %+v, want done", got)
	}
	if _, set := b.lastConfig.AnthropicWorkspaceID().Value(); set {
		t.Error("an empty workspace ID reached the clients as a workspace")
	}
}

// TestGenerateKeepsKeyOutOfOutput runs a whole generation with a marker key
// and checks neither the result nor any progress event, in a success and in a
// failure, carries it, and that a Backend failure is a plain internal result.
func TestGenerateKeepsKeyOutOfOutput(t *testing.T) {
	settings := `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":` + quote(testWorkspaceID) + `}`
	for name, failure := range map[string]devfake.Failure{"done": devfake.FailureNone, "failed": devfake.FailureAPIError} {
		t.Run(name, func(t *testing.T) {
			failAt := devfake.FailAtNone
			if failure != devfake.FailureNone {
				failAt = devfake.FailAtDraft
			}
			scenario, err := devfake.NewScenario(devfake.ArticleStandard, failure, failAt, 0)
			if err != nil {
				t.Fatalf("NewScenario: %v", err)
			}
			job := scenario.NewJob()
			b := &fakeBackend{deps: Deps{Generation: job.Generation(), Review: job.Review()}}
			m, digest := preparedModule(t, b)
			var events []string
			out := m.GenerateJSON(context.Background(), generateBody(digest, settings), func(e string) { events = append(events, e) })
			if len(events) == 0 {
				t.Error("no progress event was sent")
			}
			for _, text := range append(events, out) {
				if strings.Contains(text, testKey) || strings.Contains(text, testWorkspaceID) {
					t.Errorf("output carries the credentials: %s", text)
				}
			}
			for _, e := range events {
				var ev progressEvent
				if err := json.Unmarshal([]byte(e), &ev); err != nil || ev.Steps == (stepsJSON{}) {
					t.Errorf("progress event %q does not decode as steps: %v", e, err)
				}
			}
		})
	}
	t.Run("backend failure", func(t *testing.T) {
		b := &fakeBackend{clientsErr: errors.New(testKey)}
		m, digest := preparedModule(t, b)
		out := m.GenerateJSON(context.Background(), generateBody(digest, settings), nil)
		if got := decodeResult(t, out); got.Failure != "internal" || strings.Contains(out, testKey) {
			t.Errorf("result = %s, want a plain internal failure without the error text", out)
		}
	})
}

// TestBuildInfo checks buildInfo reports the Backend's declaration, and that
// the zero BackendInfo names no implementation.
func TestBuildInfo(t *testing.T) {
	scenario := FakeScenario{Article: "long", Failure: "none", FailAt: "none", DelaySeconds: 3, SettingsCheck: "ok"}
	tests := []struct {
		name string
		info BackendInfo
		want string // the llm member as JSON, or "" when absent
	}{
		{"claude", ClaudeInfo(), `{"kind":"claude"}`},
		{"fake", FakeInfo(scenario), `{"kind":"fake","scenario":{"article":"long","failure":"none","failAt":"none","delaySeconds":3,"settingsCheck":"ok"}}`},
		{"zero value", BackendInfo{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]json.RawMessage
			if err := json.Unmarshal([]byte(NewModule(&fakeBackend{info: tc.info}).BuildInfoJSON()), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			for _, member := range []string{"goVersion", "vcsRevision", "vcsModified", "mainPath"} {
				if _, ok := got[member]; !ok {
					t.Errorf("buildInfo has no %s", member)
				}
			}
			llmMember, present := got["llm"]
			if tc.want == "" && present {
				t.Errorf("llm = %s, want it absent", llmMember)
			}
			if tc.want != "" && string(llmMember) != tc.want {
				t.Errorf("llm = %s, want %s", llmMember, tc.want)
			}
		})
	}
}

// panickingBackend panics in the two calls Module makes to a Backend.
type panickingBackend struct{ fakeBackend }

func (*panickingBackend) NewClients(config.LLMConfig) (Deps, error) { panic("boom-clients") }
func (*panickingBackend) CheckKey(context.Context, config.LLMConfig) error {
	panic("boom-check")
}

// TestModuleRecoversBackendPanics checks a Backend panic does not escape the
// entry points: the settings check reports unverified (the call could not be
// made), and generate reports an internal failure with the panic detail.
func TestModuleRecoversBackendPanics(t *testing.T) {
	b := &panickingBackend{}
	m, digest := preparedModule(t, b)
	if got := m.CheckSettingsJSON(context.Background(), settingsRequest(quote(testKey), `""`)); got != `{"status":"unverified"}` {
		t.Errorf("settings check = %s, want unverified", got)
	}
	settings := `{"anthropicApiKey":` + quote(testKey) + `,"anthropicWorkspaceId":""}`
	got := decodeResult(t, m.GenerateJSON(context.Background(), generateBody(digest, settings), nil))
	if got.Failure != "internal" || got.InternalDetail != DetailPanic {
		t.Errorf("generate = %+v, want internal with the panic detail", got)
	}
}
