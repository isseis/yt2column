//go:build js && wasm

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Outcomes of a longOutput call. The adapter does not return the API's
// stop_reason, so the finish reason is recorded in these four classes.
const (
	outcomeCompleted        = "completed"
	outcomeTruncated        = "truncated"
	outcomeUnexpectedFinish = "unexpected_finish"
	outcomeFailed           = "failed"
)

// longOutputSystemPrompt is the fixed system prompt of every variant.
const longOutputSystemPrompt = "You are a careful writer. Follow the length and structure instructions exactly. " +
	"Never summarize, never abbreviate, and never stop before the last section."

// longOutputPrompts are the fixed user prompts, chosen by variant. Both ask
// for more text than 36,000 output tokens, so a completed or truncated answer
// measures a response near the 40,000 max_tokens default.
var longOutputPrompts = map[string]string{
	"essay": "Write one continuous essay in English on the history of written communication, " +
		"from clay tablets to electronic mail. Organize it into exactly 120 numbered sections, " +
		"each about 250 words long, for a total of about 30,000 words. Write every section in full prose.",
	"essay_longer": "Write one continuous essay in English on the history of written communication, " +
		"from clay tablets to electronic mail. Organize it into exactly 160 numbered sections, " +
		"each about 300 words long, for a total of about 48,000 words. Write every section in full prose.",
}

// longOutputResult is the LongOutputResult of the page interface. The
// generated text itself is never returned, only its length.
type longOutputResult struct {
	Outcome      string         `json:"outcome"`
	OutputTokens *uint64        `json:"outputTokens"`
	InputTokens  *uint64        `json:"inputTokens"`
	OutputChars  *int           `json:"outputChars"`
	ElapsedMs    int64          `json:"elapsedMs"`
	Failure      *failureReport `json:"failure"`
}

// runLongOutput sends one fixed long-output prompt with the same adapter
// conditions as generate (default max_tokens, 15-minute timeout).
func runLongOutput(transport *browserTransport, request string) longOutputResult {
	if len(request) > maxRequestBytes {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidInput, "the request exceeds the size limit")}
	}
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidInput, "the request is not one valid JSON object")}
	}
	members, err := object.Collect("apiKey", "workspaceId", "browserAccessHeader", "setting", "variant")
	if err != nil {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidInput, "a member appears more than once")}
	}
	rawCall, failure := readAPICall(members)
	if failure != nil {
		return longOutputResult{Outcome: outcomeFailed, Failure: failure}
	}
	setting, failure := readModelSetting(members, "setting")
	if failure != nil {
		return longOutputResult{Outcome: outcomeFailed, Failure: failure}
	}
	variant, err := strictjson.RequiredString(members, "variant")
	if err != nil {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidInput, "variant is missing, empty, or not a string")}
	}
	prompt, ok := longOutputPrompts[variant]
	if !ok {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidInput, "variant is not one of the fixed prompts")}
	}
	call, ok := parseAPICall(rawCall)
	if !ok {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidConfig, invalidConfigMessage)}
	}
	client, ok := newClient(call, setting)
	if !ok {
		return longOutputResult{Outcome: outcomeFailed, Failure: newFailure(kindInvalidConfig, invalidConfigMessage)}
	}

	transport.setCall(call.browserAccessHeader)
	defer transport.setCall(false)
	start := time.Now()
	response, err := client.Generate(context.Background(), llm.GenerateRequest{
		SystemPrompt: longOutputSystemPrompt,
		UserPrompt:   prompt,
	})
	result := longOutputResult{ElapsedMs: time.Since(start).Milliseconds()}
	// The adapter returns the usage of a truncated, unexpectedly finished, or
	// empty response; with any other error the usage is unknown (nil).
	result.InputTokens, result.OutputTokens = tokenCounts(response.Usage)
	switch {
	case err == nil:
		result.Outcome = outcomeCompleted
		chars := utf8.RuneCountInString(response.Text)
		result.OutputChars = &chars
	case errors.Is(err, llm.ErrTruncated):
		result.Outcome = outcomeTruncated
	case errors.Is(err, llm.ErrUnexpectedFinishReason):
		result.Outcome = outcomeUnexpectedFinish
	default:
		result.Outcome = outcomeFailed
		result.Failure = classify(err)
	}
	return result
}

// probeDummyKey is the fixed value the probe sends as x-api-key. It is not a
// real API key; the probe server records whether it arrived.
const probeDummyKey = "poc-0084-dummy-key"

// Probe timings. A stall case runs with a short adapter timeout; a cancel
// case runs with a long one and cancels the context shortly after the send.
const (
	probeDefaultTimeout = 30 * time.Second
	probeStallTimeout   = 3 * time.Second
	probeCancelTimeout  = 60 * time.Second
	probeCancelAfter    = 2 * time.Second
)

// probeCase is one row of the probe table: the server path, the adapter
// timeout, whether the context is canceled after the send, the fetch redirect
// mode given through the js.fetch:redirect header ("" leaves fetch's default,
// follow), and whether the path is on the foreign origin instead of the base.
type probeCase struct {
	path         string
	timeout      time.Duration
	cancel       bool
	redirectMode string
	foreign      bool
}

var probeCases = map[string]probeCase{
	"redirect_same":    {path: "/redirect/same", timeout: probeDefaultTimeout},
	"redirect_foreign": {path: "/redirect/foreign", timeout: probeDefaultTimeout},
	"redirect_manual":  {path: "/redirect/foreign", timeout: probeDefaultTimeout, redirectMode: "manual"},
	// 0090: fetch's "error" redirect mode, and direct sends to each origin
	// for the CSP connect-src variant of the extension.
	"redirect_error": {path: "/redirect/foreign", timeout: probeDefaultTimeout, redirectMode: "error"},
	"direct_same":    {path: "/target", timeout: probeDefaultTimeout},
	"direct_foreign": {path: "/target", timeout: probeDefaultTimeout, foreign: true},
	"large_body":     {path: "/large", timeout: probeDefaultTimeout},
	"stall_header":   {path: "/stall/header", timeout: probeStallTimeout},
	"stall_body":     {path: "/stall/body", timeout: probeStallTimeout},
	"cancel_header":  {path: "/stall/header", timeout: probeCancelTimeout, cancel: true},
	"cancel_body":    {path: "/stall/body", timeout: probeCancelTimeout, cancel: true},
}

// probeResult is the ProbeResult of the page interface.
type probeResult struct {
	ProbeCase string         `json:"probeCase"`
	Failure   *failureReport `json:"failure"`
	BodyBytes *int           `json:"bodyBytes"`
	FinalURL  *string        `json:"finalUrl"`
	ElapsedMs int64          `json:"elapsedMs"`
}

// runProbe sends one llmhttp.Post with the dummy key to the probe server.
// The request carries no API key member, so a real key can never reach the
// probe server, and the browser access header is never added.
func runProbe(transport *browserTransport, request string) probeResult {
	if len(request) > maxRequestBytes {
		return probeResult{Failure: newFailure(kindInvalidInput, "the request exceeds the size limit")}
	}
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return probeResult{Failure: newFailure(kindInvalidInput, "the request is not one valid JSON object")}
	}
	members, err := object.Collect("baseUrl", "foreignUrl", "probeCase")
	if err != nil {
		return probeResult{Failure: newFailure(kindInvalidInput, "a member appears more than once")}
	}
	name, err := strictjson.RequiredString(members, "probeCase")
	if err != nil {
		return probeResult{Failure: newFailure(kindInvalidInput, "probeCase is missing, empty, or not a string")}
	}
	pc, ok := probeCases[name]
	if !ok {
		return probeResult{Failure: newFailure(kindInvalidInput, "probeCase is not a known case")}
	}
	result := probeResult{ProbeCase: name}
	rawBase, err := strictjson.RequiredString(members, "baseUrl")
	if err != nil {
		result.Failure = newFailure(kindInvalidInput, "baseUrl is missing, empty, or not a string")
		return result
	}
	rawForeign, err := strictjson.RequiredString(members, "foreignUrl")
	if err != nil {
		result.Failure = newFailure(kindInvalidInput, "foreignUrl is missing, empty, or not a string")
		return result
	}
	base, ok := probeOrigin(rawBase, "127.0.0.1")
	if !ok {
		result.Failure = newFailure(kindInvalidInput, "baseUrl must be http://127.0.0.1:<port>")
		return result
	}
	// foreignUrl is checked even for the cases that do not send to it, so a
	// request naming another host is refused before anything is sent.
	foreign, ok := probeOrigin(rawForeign, "localhost")
	if !ok {
		result.Failure = newFailure(kindInvalidInput, "foreignUrl must be http://localhost:<port>")
		return result
	}
	target := base.JoinPath(pc.path).String()
	if pc.foreign {
		target = foreign.JoinPath(pc.path).String()
	}

	transport.setCall(false)
	defer transport.setCall(false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if pc.cancel {
		timer := time.AfterFunc(probeCancelAfter, cancel)
		defer timer.Stop()
	}
	start := time.Now()
	data, err := llmhttp.Post(ctx, llmhttp.Call{
		Client:  nil, // the zero client uses http.DefaultTransport, which is browserTransport
		Timeout: pc.timeout,
		NewRequest: func(callCtx context.Context) (*http.Request, error) {
			req, err := http.NewRequestWithContext(callCtx, http.MethodPost, target, bytes.NewReader([]byte(`{"probe":true}`)))
			if err != nil {
				return nil, err
			}
			req.Header.Set("content-type", "application/json")
			req.Header.Set("x-api-key", probeDummyKey)
			if pc.redirectMode != "" {
				// Go's js transport turns this header into fetch's redirect option.
				req.Header.Set("js.fetch:redirect", pc.redirectMode)
			}
			return req, nil
		},
		Errors: llmhttp.Errors{
			Transport:       claude.ErrTransport,
			InvalidResponse: claude.ErrInvalidResponse,
			Status: func(status int) error {
				return &claude.HTTPStatusError{StatusCode: status}
			},
		},
	})
	result.ElapsedMs = time.Since(start).Milliseconds()
	if final := transport.lastResponseURL(); final != nil {
		finalURL := final.String()
		result.FinalURL = &finalURL
	}
	if err != nil {
		result.Failure = classify(err)
		return result
	}
	size := len(data)
	result.BodyBytes = &size
	return result
}

// probeOrigin parses raw and accepts it only as http://<host>:<port> or
// http://<host>:<port>/ with the expected host name, a port, no user
// information, and nothing after the origin. Any other form is rejected, not
// reduced to the origin.
func probeOrigin(raw, host string) (*url.URL, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}
	if parsed.Scheme != "http" || parsed.User != nil || parsed.Hostname() != host || parsed.Port() == "" {
		return nil, false
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" || parsed.ForceQuery ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, false
	}
	return &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}, true
}
