package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/extjob/claudebackend"
	"github.com/isseis/yt2column/internal/extjob/devbackend"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/redact"
)

// Exit codes: exitDone when the generation produced an article, exitFailed
// when the module rejected the input or the generation failed (its JSON is
// on standard output either way), and exitUsage when nothing was run: the
// arguments, the scenario, the input file, or the API key's variable.
const (
	exitDone   = 0
	exitFailed = 1
	exitUsage  = 2
)

// programName prefixes every line written to standard error.
const programName = "yt2column-extjob"

// The accepted values of -llm. There is no default.
const (
	llmClaude = "claude"
	llmFake   = "fake"
)

// fakeAPIKey is the API key the fake runs give the module, which checks the
// settings' shape before it calls any Backend. The development backend never
// sends it anywhere.
const fakeAPIKey = "devfake-key-never-sent" //nolint:gosec // a placeholder the fake backend ignores, not a credential

// backends are the constructors of the two LLM implementations. A test
// replaces them to observe whether, and with what, they are called.
type backends struct {
	claude func() extjob.Backend
	fake   func(devfake.Scenario) extjob.Backend
}

// productionBackends returns the real constructors.
func productionBackends() backends {
	return backends{claude: claudebackend.New, fake: devbackend.New}
}

// errUsage reports arguments run does not accept.
var errUsage = errors.New("usage")

// errRepeated reports an argument given more than once.
var errRepeated = errors.New("given more than once")

// mebibyte converts the module's size limit for the usage text.
const mebibyte = 1 << 20

// usage is the text written for an argument error. It names every argument
// and that -llm has no default; the size limit comes from the module.
var usage = fmt.Sprintf(`usage: yt2column-extjob -llm=claude -input=<file>
       yt2column-extjob -llm=fake -article=<a> -failure=<f> -fail-at=<s> -delay=<seconds> -settings-check=<c> -input=<file>
-llm is required. claude calls the Claude API with the Anthropic API key and
workspace ID variables the CLI reads (the workspace ID only when it is set);
fake calls nothing and takes all five scenario arguments. Each argument is
given at most once. The input file is a prepare request (JSON) of at most
%d MiB. Do not commit an input file: it may hold a copied transcript.`, extjob.MaxPrepareRequestBytes/mebibyte)

// onceString is a string flag that rejects a second value, so a repeated
// argument (such as -llm=fake -llm=claude) is an error rather than the last
// value silently winning.
type onceString struct {
	value *string
	set   bool
}

func (o *onceString) String() string {
	if o.value == nil {
		return ""
	}
	return *o.value
}

func (o *onceString) Set(v string) error {
	if o.set {
		return errRepeated
	}
	o.set = true
	*o.value = v
	return nil
}

// options are the parsed arguments.
type options struct {
	llm      string
	scenario devfake.ScenarioSpelling
	input    string
}

// parseArgs reads the arguments. It accepts -llm=claude without any scenario
// argument and -llm=fake with any of them (the scenario parser rejects a
// missing one), and rejects everything else, including extra words and an
// argument given twice.
func parseArgs(args []string) (options, error) {
	var opts options
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for name, value := range map[string]*string{
		"llm":            &opts.llm,
		"input":          &opts.input,
		"article":        &opts.scenario.Article,
		"failure":        &opts.scenario.Failure,
		"fail-at":        &opts.scenario.FailAt,
		"delay":          &opts.scenario.DelaySeconds,
		"settings-check": &opts.scenario.SettingsCheck,
	} {
		fs.Var(&onceString{value: value}, name, "")
	}
	if err := fs.Parse(args); err != nil {
		return options{}, fmt.Errorf("%w: %w", errUsage, err)
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("%w: unexpected word %q", errUsage, fs.Arg(0))
	}
	if opts.input == "" {
		return options{}, fmt.Errorf("%w: -input is required", errUsage)
	}
	switch opts.llm {
	case llmFake:
	case llmClaude:
		scenarioSet := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name != "llm" && f.Name != "input" {
				scenarioSet = true
			}
		})
		if scenarioSet {
			return options{}, fmt.Errorf("%w: the scenario arguments are only for -llm=fake", errUsage)
		}
	default:
		return options{}, fmt.Errorf("%w: -llm must be %s or %s", errUsage, llmClaude, llmFake)
	}
	return opts, nil
}

// storedSettings is the settings member of a generate request, in the shape
// the running viewer reads from chrome.storage.local.
type storedSettings struct {
	APIKey      string `json:"anthropicApiKey"`
	WorkspaceID string `json:"anthropicWorkspaceId"`
}

// generateRequest is the request of GenerateJSON.
type generateRequest struct {
	InputDigest string         `json:"inputDigest"`
	Settings    storedSettings `json:"settings"`
}

// statusOnly reads the status member of a result, and the input digest of a
// prepare result.
type statusOnly struct {
	Status      string `json:"status"`
	InputDigest string `json:"inputDigest"`
}

// readInput reads the input file, at most one byte past the module's limit,
// so a longer file is rejected by the module's own rule without being read
// whole.
func readInput(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the developer names the input file
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, extjob.MaxPrepareRequestBytes+1))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// run runs one generation of the input file and returns the exit code. It
// writes the prepare result (when the module rejects the input) or the
// generate result to stdout, and the progress events and every message to
// stderr.
//
// Contract: no LLM implementation is constructed before the arguments, the
// scenario, the API key's variable, and the input file are accepted; and the
// API key reaches only the module's request, never stdout or stderr. The key's
// variable is read first, whatever -llm says, so every message written to
// stderr (an argument error may echo an argument that holds the key) is
// passed through redact.Line with the key as a secret; the results and
// progress events the module built never carry it.
func run(ctx context.Context, args []string, lookup config.LookupFunc, stdout, stderr io.Writer, b backends) int {
	read := config.ReadExtensionSettings(lookup)
	var secrets []string
	if key, _ := read.APIKey(); key != "" {
		// A message may quote an argument with %q (the flag package and the
		// scenario parser do) before redact.Line escapes the whole line
		// again; the quoted spelling is a secret too, so its escaped form is
		// hidden as well.
		quoted := strconv.Quote(key)
		secrets = append(secrets, key, quoted[1:len(quoted)-1])
	}
	errorf := func(format string, a ...any) {
		_, _ = io.WriteString(stderr, redact.Line(programName+": "+fmt.Sprintf(format, a...), secrets...)+"\n")
	}
	opts, err := parseArgs(args)
	if err != nil {
		errorf("%v", err)
		_, _ = io.WriteString(stderr, usage+"\n")
		return exitUsage
	}
	var scenario devfake.Scenario
	if opts.llm == llmFake {
		if scenario, err = devfake.ParseScenario(opts.scenario); err != nil {
			errorf("%v", err)
			return exitUsage
		}
	}
	settings := storedSettings{APIKey: fakeAPIKey}
	if opts.llm == llmClaude {
		key, set := read.APIKey()
		if !set {
			errorf("the Anthropic API key variable is not set")
			return exitUsage
		}
		if config.HTTP2DebugRequested(lookup) {
			errorf("warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key to standard error")
		}
		// An unset workspace ID is the empty string, as the options page
		// always writes the item; the module reads it as no workspace.
		workspaceID, _ := read.WorkspaceID()
		settings = storedSettings{APIKey: key, WorkspaceID: workspaceID}
	}
	input, err := readInput(opts.input)
	if err != nil {
		errorf("reading the input file: %v", err)
		return exitUsage
	}

	var backend extjob.Backend
	switch opts.llm {
	case llmClaude:
		backend = b.claude()
	case llmFake:
		backend = b.fake(scenario)
	default:
		// parseArgs accepts only the two values.
		errorf("unknown -llm %q", opts.llm)
		return exitUsage
	}
	module := extjob.NewModule(backend)

	prepared := module.PrepareJSON(input)
	var prepare statusOnly
	if err := json.Unmarshal([]byte(prepared), &prepare); err != nil || prepare.Status != "ok" {
		_, _ = io.WriteString(stdout, prepared+"\n")
		errorf("the module did not accept the input file")
		return exitFailed
	}
	request, err := json.Marshal(generateRequest{InputDigest: prepare.InputDigest, Settings: settings})
	if err != nil {
		errorf("building the generate request failed")
		return exitFailed
	}
	result := module.GenerateJSON(ctx, string(request), func(event string) {
		// The module marshals the event with encoding/json, which escapes every
		// control character, so it is written as it is, one line per event.
		_, _ = io.WriteString(stderr, programName+": progress "+event+"\n")
	})
	if _, err := io.WriteString(stdout, result+"\n"); err != nil {
		errorf("writing the result failed: %v", err)
		return exitFailed
	}
	var generated statusOnly
	if err := json.Unmarshal([]byte(result), &generated); err != nil || generated.Status != "done" {
		errorf("the generation did not produce an article")
		return exitFailed
	}
	return exitDone
}
