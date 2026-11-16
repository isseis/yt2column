// Package main is the development article generation module of the browser
// extension: the extension job (internal/extjob) with the fake LLM of one
// scenario (internal/extjob/devbackend), registered for JavaScript. The
// scenario is embedded at build time, one -X main.<variable>=<value> per
// variable below in a single -ldflags string, by the extension's development
// build (make ext-build-dev, which the extension build task adds).
//
// A js build registers the module's functions only when the embedded values
// parse; a native build parses the same values and reports the result, so the
// build script can reject a scenario before building the WebAssembly. It must
// never depend on the Claude adapter or the release backend, which
// internal/extjob's dependency test checks.
package main

import (
	"fmt"
	"io"

	"github.com/isseis/yt2column/internal/llm/devfake"
)

// The scenario's five spellings, set by the linker. Each is empty unless the
// build sets it, and devfake.ParseScenario rejects an empty value, so a
// variable the build did not set (or set under a misspelled name, which the
// linker ignores) stops the module instead of selecting a default.
var (
	scenarioArticle       string
	scenarioFailure       string
	scenarioFailAt        string
	scenarioDelaySeconds  string
	scenarioSettingsCheck string
)

// Exit codes of the native build.
const (
	exitOK      = 0
	exitInvalid = 1
)

// embeddedScenario parses the five embedded spellings with
// devfake.ParseScenario, unchanged.
func embeddedScenario() (devfake.Scenario, error) {
	return devfake.ParseScenario(devfake.ScenarioSpelling{
		Article:       scenarioArticle,
		Failure:       scenarioFailure,
		FailAt:        scenarioFailAt,
		DelaySeconds:  scenarioDelaySeconds,
		SettingsCheck: scenarioSettingsCheck,
	})
}

// checkEmbedded parses the embedded scenario and returns the exit code of the
// native build: exitOK when it parses, and exitInvalid with the reason on
// stderr when it does not. It writes nothing on success.
func checkEmbedded(stderr io.Writer) int {
	if _, err := embeddedScenario(); err != nil {
		reportRejected(stderr, err)
		return exitInvalid
	}
	return exitOK
}

// reportRejected writes why the embedded scenario was not accepted. The
// scenario holds no secret.
func reportRejected(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "yt2column-wasm-dev: the embedded scenario is not accepted: %v\n", err)
}
