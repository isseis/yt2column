//go:build test

package deepseek

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
)

// assertBodyAccepted runs Generate against a server that answers with body
// and requires success.
func assertBodyAccepted(t *testing.T, body []byte) llm.GenerateResponse {
	t.Helper()
	server := newResponseServer(t, http.StatusOK, body)
	response, err := generateAgainst(t, server.URL, validRequest(), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	return response
}

// assertBodyRejected runs Generate against a server that answers with body,
// checks the wanted sentinel and the shared failure assertions, and returns
// the error for further diagnostics checks.
func assertBodyRejected(t *testing.T, body []byte, want error) error {
	t.Helper()
	server := newResponseServer(t, http.StatusOK, body)
	response, err := generateAgainst(t, server.URL, validRequest(), nil)
	if !errors.Is(err, want) {
		t.Errorf("Generate() error = %v, want %v", err, want)
	}
	assertRejected(t, response, err)
	return err
}

// assertUsageUnknown requires body to be accepted with the fixture's text,
// model, and model version, and an unknown usage whose reason is want. The
// accepted response must be unchanged apart from the usage.
func assertUsageUnknown(t *testing.T, fixture, body []byte, want string) {
	t.Helper()
	response := assertBodyAccepted(t, body)
	document := documentOf(t, fixture)
	content, _ := messageOf(t, document)[keyContent].(string)
	model, _ := document[keyModel].(string)
	fingerprint, _ := document[keySystemFingerprint].(string)
	if response.Text != content || response.Model != model || response.ModelVersion != fingerprint {
		t.Errorf("response = %+v, want Text %q, Model %q, ModelVersion %q", response, content, model, fingerprint)
	}
	if _, known := response.Usage.Counts(); known {
		t.Fatalf("Usage = %+v, want an unknown usage", response.Usage)
	}
	if got := response.Usage.UnknownReason(); got != want {
		t.Errorf("UnknownReason() = %q, want %q", got, want)
	}
}

func TestGenerateResponseFixtures(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	length := readFixture(t, testdataLengthFixture)

	t.Run("stop fixture", func(t *testing.T) {
		document := documentOf(t, stop)
		response := assertBodyAccepted(t, stop)
		content, _ := messageOf(t, document)[keyContent].(string)
		model, _ := document[keyModel].(string)
		fingerprint, _ := document[keySystemFingerprint].(string)
		if response.Text != content {
			t.Errorf("Text = %q, want %q", response.Text, content)
		}
		if response.Model != model {
			t.Errorf("Model = %q, want %q", response.Model, model)
		}
		if response.ModelVersion != fingerprint {
			t.Errorf("ModelVersion = %q, want %q", response.ModelVersion, fingerprint)
		}
	})

	t.Run("length fixture is truncated", func(t *testing.T) {
		assertBodyRejectedResponse(t, length, llm.ErrTruncated, lengthFixtureUsage)
	})

	t.Run("content whitespace is preserved", func(t *testing.T) {
		document := documentOf(t, stop)
		content, _ := messageOf(t, document)[keyContent].(string)
		want := " \n" + content + "\t "
		messageOf(t, document)[keyContent] = want
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.Text != want {
			t.Errorf("Text = %q, want %q", response.Text, want)
		}
	})

	t.Run("unknown members are ignored at every level", func(t *testing.T) {
		document := documentOf(t, stop)
		document["unknown_top"] = map[string]any{"nested": []any{1, 2, 3}}
		document["id"] = float64(42)
		choice := choiceOf(t, document)
		choice["unknown_choice"] = "x"
		message := messageOf(t, document)
		message["unknown_message"] = []any{nil, true}
		message["role"] = float64(7)
		response := assertBodyAccepted(t, encodeDocument(t, document))
		content, _ := messageOf(t, document)[keyContent].(string)
		if response.Text != content {
			t.Errorf("Text = %q, want %q", response.Text, content)
		}
	})
}

func TestGenerateModelFromResponse(t *testing.T) {
	document := documentOf(t, readFixture(t, testdataStopFixture))
	const responseModel = "deepseek-v4-1-flash"
	document[keyModel] = responseModel
	response := assertBodyAccepted(t, encodeDocument(t, document))
	if response.Model != responseModel {
		t.Errorf("Model = %q, want %q", response.Model, responseModel)
	}
}

func TestGenerateFinishReason(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)

	t.Run("length with non-empty content is truncated", func(t *testing.T) {
		document := documentOf(t, readFixture(t, testdataLengthFixture))
		messageOf(t, document)[keyContent] = "partial answer"
		assertBodyRejectedResponse(t, encodeDocument(t, document), llm.ErrTruncated, lengthFixtureUsage)
	})

	for _, reason := range []string{"content_filter", "insufficient_system_resource", "tool_calls", "surprise_value"} {
		t.Run("unexpected "+reason, func(t *testing.T) {
			body := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":"`+reason+`"`)
			err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, stopFixtureUsage)
			if errors.Is(err, llm.ErrTruncated) {
				t.Errorf("error %v must not match ErrTruncated", err)
			}
			if !strings.Contains(err.Error(), reason) {
				t.Errorf("error %q does not name the finish_reason %q", err, reason)
			}
		})
	}

	t.Run("unexpected empty string", func(t *testing.T) {
		// An empty finish_reason is a string of the accepted kind, so its
		// value is classified by the finish-reason check, not rejected as a
		// shape error.
		body := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":""`)
		err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, stopFixtureUsage)
		if errors.Is(err, ErrInvalidResponse) {
			t.Errorf("error %v must not match ErrInvalidResponse", err)
		}
	})

	t.Run("a long unexpected finish_reason is capped", func(t *testing.T) {
		const long = "reason-longer-than-the-sixty-four-byte-cap-0123456789abcdefghijklmnopqrstuvwxyz"
		if len(long) <= maxReasonBytes {
			t.Fatalf("the test reason must be longer than %d bytes", maxReasonBytes)
		}
		body := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":"`+long+`"`)
		err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, stopFixtureUsage)
		if !strings.Contains(err.Error(), long[:maxReasonBytes]) {
			t.Errorf("error %q does not include the first %d bytes of the reason", err, maxReasonBytes)
		}
		if strings.Contains(err.Error(), long[maxReasonBytes:]) {
			t.Errorf("error %q includes the reason beyond %d bytes", err, maxReasonBytes)
		}
	})
}

func TestGenerateValidationOrder(t *testing.T) {
	// The length fixture has an empty content: the finish reason must win.
	response, err := generateAgainst(t, newResponseServer(t, http.StatusOK, readFixture(t, testdataLengthFixture)).URL, validRequest(), nil)
	if !errors.Is(err, llm.ErrTruncated) {
		t.Errorf("Generate() error = %v, want ErrTruncated", err)
	}
	if errors.Is(err, llm.ErrEmptyResponse) {
		t.Errorf("Generate() error = %v, must not match ErrEmptyResponse", err)
	}
	assertRejectedResponse(t, response, err, testModel, lengthFixtureUsage)
}

func TestGenerateEmptyContent(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	whitespace := map[string]string{
		"empty":            "",
		"spaces":           "   ",
		"newlines":         "\n\r\n",
		"tabs":             "\t\t",
		"full-width space": "\u3000",
	}
	for name, content := range whitespace {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, stop)
			messageOf(t, document)[keyContent] = content
			assertBodyRejectedResponse(t, encodeDocument(t, document), llm.ErrEmptyResponse, stopFixtureUsage)
		})
	}

	t.Run("reasoning_content alone is not an answer", func(t *testing.T) {
		document := documentOf(t, stop)
		messageOf(t, document)[keyContent] = ""
		assertBodyRejectedResponse(t, encodeDocument(t, document), llm.ErrEmptyResponse, stopFixtureUsage)
	})

	t.Run("reasoning_content is not part of Text", func(t *testing.T) {
		const marker = "REASONING-MARKER-8481"
		document := documentOf(t, stop)
		messageOf(t, document)["reasoning_content"] = marker + " and more"
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if strings.Contains(response.Text, marker) {
			t.Errorf("Text %q contains the reasoning_content marker", response.Text)
		}
	})
}

func TestGenerateInvalidBody(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	bodies := map[string][]byte{
		"empty":                  {},
		"whitespace only":        []byte(" \n\t"),
		"not JSON":               []byte("not json"),
		"top-level array":        []byte("[]"),
		"top-level null":         []byte("null"),
		"top-level string":       []byte(`"text"`),
		"top-level number":       []byte("42"),
		"trailing object":        append(append([]byte{}, stop...), []byte(" {}")...),
		"trailing text":          append(append([]byte{}, stop...), []byte(" x")...),
		"top-level empty object": []byte("{}"),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			assertBodyRejected(t, body, ErrInvalidResponse)
		})
	}
}

func TestGenerateConsumedMembers(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	mutations := map[string]func(t *testing.T, document map[string]any){
		"missing model":         func(_ *testing.T, d map[string]any) { delete(d, keyModel) },
		"null model":            func(_ *testing.T, d map[string]any) { d[keyModel] = nil },
		"empty model":           func(_ *testing.T, d map[string]any) { d[keyModel] = "" },
		"numeric model":         func(_ *testing.T, d map[string]any) { d[keyModel] = float64(3) },
		"missing choices":       func(_ *testing.T, d map[string]any) { delete(d, keyChoices) },
		"null choices":          func(_ *testing.T, d map[string]any) { d[keyChoices] = nil },
		"object choices":        func(_ *testing.T, d map[string]any) { d[keyChoices] = map[string]any{} },
		"string choices":        func(_ *testing.T, d map[string]any) { d[keyChoices] = "x" },
		"missing finish_reason": func(t *testing.T, d map[string]any) { delete(choiceOf(t, d), keyFinishReason) },
		"null finish_reason":    func(t *testing.T, d map[string]any) { choiceOf(t, d)[keyFinishReason] = nil },
		"numeric finish_reason": func(t *testing.T, d map[string]any) { choiceOf(t, d)[keyFinishReason] = float64(1) },
		"missing message":       func(t *testing.T, d map[string]any) { delete(choiceOf(t, d), keyMessage) },
		"null message":          func(t *testing.T, d map[string]any) { choiceOf(t, d)[keyMessage] = nil },
		"numeric message":       func(t *testing.T, d map[string]any) { choiceOf(t, d)[keyMessage] = float64(1) },
		"missing content":       func(t *testing.T, d map[string]any) { delete(messageOf(t, d), keyContent) },
		"null content":          func(t *testing.T, d map[string]any) { messageOf(t, d)[keyContent] = nil },
		"numeric content":       func(t *testing.T, d map[string]any) { messageOf(t, d)[keyContent] = float64(1) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, stop)
			mutate(t, document)
			err := assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
			if errors.Is(err, llm.ErrEmptyResponse) {
				t.Errorf("error %v must not match ErrEmptyResponse", err)
			}
		})
	}
}

func TestGenerateChoicesShape(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	mutations := map[string]func(t *testing.T, document map[string]any){
		"empty array":          func(_ *testing.T, d map[string]any) { d[keyChoices] = []any{} },
		"two elements":         func(t *testing.T, d map[string]any) { d[keyChoices] = []any{choiceOf(t, d), choiceOf(t, d)} },
		"null element":         func(_ *testing.T, d map[string]any) { d[keyChoices] = []any{nil} },
		"string element":       func(_ *testing.T, d map[string]any) { d[keyChoices] = []any{"x"} },
		"number element":       func(_ *testing.T, d map[string]any) { d[keyChoices] = []any{float64(1)} },
		"empty object element": func(_ *testing.T, d map[string]any) { d[keyChoices] = []any{map[string]any{}} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, stop)
			mutate(t, document)
			err := assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
			if errors.Is(err, llm.ErrEmptyResponse) {
				t.Errorf("error %v must not match ErrEmptyResponse", err)
			}
		})
	}
}

func TestGenerateDuplicateMembers(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	rejected := []struct {
		name string
		body []byte
	}{
		{"model with equal values", replaceOnce(t, stop,
			`"model":"deepseek-flash"`, `"model":"deepseek-flash","model":"deepseek-flash"`)},
		{"model with different values", replaceOnce(t, stop,
			`"model":"deepseek-flash"`, `"model":"deepseek-flash","model":"other"`)},
		{"finish_reason with equal values", replaceOnce(t, stop,
			`"finish_reason":"stop"`, `"finish_reason":"stop","finish_reason":"stop"`)},
		{"finish_reason with different values", replaceOnce(t, stop,
			`"finish_reason":"stop"`, `"finish_reason":"stop","finish_reason":"length"`)},
		{"content", replaceOnce(t, stop,
			`"role":"assistant","content":`, `"role":"assistant","content":"dup","content":`)},
		{"system_fingerprint", replaceOnce(t, stop,
			`"system_fingerprint":"`, `"system_fingerprint":"dup","system_fingerprint":"`)},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			assertBodyRejected(t, tc.body, ErrInvalidResponse)
		})
	}

	t.Run("non-consumed duplicates are accepted", func(t *testing.T) {
		body := replaceOnce(t, stop, `{"id":"`, `{"id":"dup","id":"`)
		body = replaceOnce(t, body, `"index":0`, `"index":0,"index":0`)
		body = replaceOnce(t, body, `"role":"assistant"`, `"role":"assistant","role":"duplicate"`)
		document := documentOf(t, body)
		content, _ := messageOf(t, document)[keyContent].(string)
		response := assertBodyAccepted(t, body)
		if response.Text != content {
			t.Errorf("Text = %q, want %q", response.Text, content)
		}
	})
}

func TestGenerateEncoding(t *testing.T) {
	bodyWith := func(messageMembers string) []byte {
		format := `{"model":"deepseek-flash","choices":[{"finish_reason":"stop","message":{` +
			`"role":"assistant",` + messageMembers + `}}],"system_fingerprint":"fp"}`
		return []byte(format)
	}
	invalidUTF8Content := bodyWith(`"content":"` + "\xff" + `"`)
	invalidUTF8Reasoning := bodyWith(`"content":"ok","reasoning_content":"` + "\xff" + `"`)
	surrogateContent := bodyWith(`"content":"\ud800"`)
	surrogateReasoning := bodyWith(`"content":"ok","reasoning_content":"\ud800"`)

	if utf8.Valid(invalidUTF8Content) || utf8.Valid(invalidUTF8Reasoning) {
		t.Fatal("the invalid-UTF-8 inputs must not be valid UTF-8")
	}
	if !utf8.Valid(surrogateContent) || !json.Valid(surrogateContent) {
		t.Fatal("the surrogate input must be valid UTF-8 and JSON, so only the surrogate check can reject it")
	}
	if !utf8.Valid(surrogateReasoning) || !json.Valid(surrogateReasoning) {
		t.Fatal("the reasoning surrogate input must be valid UTF-8 and JSON")
	}

	rejected := map[string][]byte{
		"invalid UTF-8 in content":                invalidUTF8Content,
		"invalid UTF-8 in reasoning_content":      invalidUTF8Reasoning,
		"unpaired surrogate in content":           surrogateContent,
		"unpaired surrogate in reasoning_content": surrogateReasoning,
	}
	for name, body := range rejected {
		t.Run(name, func(t *testing.T) {
			assertBodyRejected(t, body, ErrInvalidResponse)
		})
	}

	t.Run("a valid surrogate pair is accepted", func(t *testing.T) {
		response := assertBodyAccepted(t, bodyWith(`"content":"\ud83d\ude00"`))
		if response.Text != "\U0001F600" {
			t.Errorf("Text = %q, want the decoded surrogate pair", response.Text)
		}
	})
}

func TestGenerateSizeLimit(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)

	t.Run("exactly the limit", func(t *testing.T) {
		body := append(append([]byte{}, stop...), bytes.Repeat([]byte(" "), maxResponseBytes-len(stop))...)
		if len(body) != maxResponseBytes {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes)
		}
		assertBodyAccepted(t, body)
	})

	t.Run("limit with leading blank lines", func(t *testing.T) {
		const blankLines = "\n\r\n\n"
		body := append([]byte(blankLines), stop...)
		body = append(body, bytes.Repeat([]byte(" "), maxResponseBytes-len(body))...)
		if len(body) != maxResponseBytes {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes)
		}
		assertBodyAccepted(t, body)
	})

	t.Run("one byte over the limit", func(t *testing.T) {
		body := append(append([]byte{}, stop...), bytes.Repeat([]byte(" "), maxResponseBytes-len(stop)+1)...)
		if len(body) != maxResponseBytes+1 {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes+1)
		}
		assertBodyRejected(t, body, ErrInvalidResponse)
	})
}

func TestGenerateKeepAliveBlankLines(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)

	t.Run("leading blank lines are accepted", func(t *testing.T) {
		body := append([]byte("\n\r\n\n\r\n"), stop...)
		assertBodyAccepted(t, body)
	})

	t.Run("whitespace-only body names its size", func(t *testing.T) {
		body := []byte(" \n\r\n\t ")
		err := assertBodyRejected(t, body, ErrInvalidResponse)
		if !strings.Contains(err.Error(), "whitespace only") {
			t.Errorf("error %q does not say the body was whitespace only", err)
		}
		if !strings.Contains(err.Error(), strconv.Itoa(len(body))) {
			t.Errorf("error %q does not name the body size %d", err, len(body))
		}
	})

	t.Run("a non-JSON Unicode space is not called whitespace", func(t *testing.T) {
		body := []byte("\u00a0")
		if len(bytes.TrimSpace(body)) != 0 {
			t.Fatal("the input must be Unicode whitespace that bytes.TrimSpace strips")
		}
		err := assertBodyRejected(t, body, ErrInvalidResponse)
		if strings.Contains(err.Error(), "whitespace only") {
			t.Errorf("error %q calls a non-JSON-whitespace body whitespace only", err)
		}
	})
}

func TestGenerateErrorMemberDiagnostics(t *testing.T) {
	const marker = "ERROR-VALUE-MARKER"

	t.Run("missing consumed members at the top level", func(t *testing.T) {
		body := []byte(`{"error":{"message":"` + marker + `","type":"authentication_error"}}`)
		err := assertBodyRejected(t, body, ErrInvalidResponse)
		if !strings.Contains(err.Error(), `top-level "error"`) {
			t.Errorf("error %q does not report the top-level error member", err)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("error %q contains the error member value", err)
		}
	})

	t.Run("a later shape failure also reports the error member", func(t *testing.T) {
		body := []byte(`{"error":{"message":"` + marker + `"},"model":"deepseek-flash",` +
			`"choices":[{"finish_reason":"stop"}],"system_fingerprint":"fp"}`)
		err := assertBodyRejected(t, body, ErrInvalidResponse)
		if !strings.Contains(err.Error(), `top-level "error"`) {
			t.Errorf("error %q does not report the top-level error member", err)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("error %q contains the error member value", err)
		}
	})
}

func TestGenerateSystemFingerprint(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)

	t.Run("value is copied", func(t *testing.T) {
		document := documentOf(t, stop)
		fingerprint, _ := document[keySystemFingerprint].(string)
		if fingerprint == "" {
			t.Fatal("the fixture has no system_fingerprint to compare")
		}
		response := assertBodyAccepted(t, stop)
		if response.ModelVersion != fingerprint {
			t.Errorf("ModelVersion = %q, want %q", response.ModelVersion, fingerprint)
		}
	})

	t.Run("missing member is accepted as empty", func(t *testing.T) {
		document := documentOf(t, stop)
		delete(document, keySystemFingerprint)
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.ModelVersion != "" {
			t.Errorf("ModelVersion = %q, want an empty string", response.ModelVersion)
		}
	})

	rejected := map[string]func(document map[string]any){
		"null":         func(d map[string]any) { d[keySystemFingerprint] = nil },
		"number":       func(d map[string]any) { d[keySystemFingerprint] = float64(1) },
		"empty string": func(d map[string]any) { d[keySystemFingerprint] = "" },
	}
	for name, mutate := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			document := documentOf(t, stop)
			mutate(document)
			assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
		})
	}

	t.Run("rejects a duplicate", func(t *testing.T) {
		body := replaceOnce(t, stop, `"system_fingerprint":"`, `"system_fingerprint":"dup","system_fingerprint":"`)
		assertBodyRejected(t, body, ErrInvalidResponse)
	})
}

func TestGenerateUsageFixtures(t *testing.T) {
	response := assertBodyAccepted(t, readFixture(t, testdataStopFixture))
	if response.Usage != stopFixtureUsage {
		t.Errorf("Usage = %+v, want %+v", response.Usage, stopFixtureUsage)
	}
}

func TestGenerateUsageCategories(t *testing.T) {
	const input, cacheRead, output = 11, 22, 33
	body := responseWithUsage(t, readFixture(t, testdataStopFixture), usageCounts(input, cacheRead, output))
	response := assertBodyAccepted(t, body)
	want := llm.KnownUsage(llm.TokenCounts{Input: input, CacheRead: cacheRead, CacheWrite: 0, Output: output})
	if response.Usage != want {
		t.Errorf("Usage = %+v, want %+v", response.Usage, want)
	}
}

func TestGenerateUsageUnknown(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	// Each consumed member and the value it is set to in the valid usage, so a
	// duplicate can be injected at the byte level with replaceOnce.
	members := []struct {
		name  string
		value int
	}{
		{keyPromptCacheMissTokens, 1},
		{keyPromptCacheHitTokens, 2},
		{keyCompletionTokens, 3},
	}
	invalidValues := []struct {
		name  string
		value json.RawMessage
	}{
		{"negative", json.RawMessage("-1")},
		{"negative zero", json.RawMessage("-0")},
		{"fraction", json.RawMessage("1.5")},
		{"exponent", json.RawMessage("1e3")},
		{"out of int64", json.RawMessage("9223372036854775808")},
		{"string", json.RawMessage(`"x"`)},
		{"boolean", json.RawMessage("true")},
		{"null", json.RawMessage("null")},
		{"array", json.RawMessage("[]")},
		{"object", json.RawMessage("{}")},
	}

	t.Run("top-level usage", func(t *testing.T) {
		cases := []struct {
			name string
			body []byte
			want string
		}{
			{"missing", responseWithUsage(t, stop, nil), "usage:missing"},
			{"null", responseWithUsage(t, stop, json.RawMessage("null")), "usage:not-an-object"},
			{"string", responseWithUsage(t, stop, json.RawMessage(`"x"`)), "usage:not-an-object"},
			{"number", responseWithUsage(t, stop, json.RawMessage("1")), "usage:not-an-object"},
			{"boolean", responseWithUsage(t, stop, json.RawMessage("true")), "usage:not-an-object"},
			{"array", responseWithUsage(t, stop, json.RawMessage("[]")), "usage:not-an-object"},
			{"duplicate", replaceOnce(t, stop, `"usage":{`, `"usage":{},"usage":{`), "usage:duplicate"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				assertUsageUnknown(t, stop, tc.body, tc.want)
			})
		}
	})

	for _, member := range members {
		for _, invalid := range invalidValues {
			t.Run(member.name+" "+invalid.name, func(t *testing.T) {
				usage := usageCounts(1, 2, 3)
				usage[member.name] = invalid.value
				assertUsageUnknown(t, stop, responseWithUsage(t, stop, usage), "usage."+member.name+":not-a-non-negative-integer")
			})
		}

		t.Run(member.name+" missing", func(t *testing.T) {
			usage := usageCounts(1, 2, 3)
			delete(usage, member.name)
			assertUsageUnknown(t, stop, responseWithUsage(t, stop, usage), "usage."+member.name+":missing")
		})

		t.Run(member.name+" duplicate", func(t *testing.T) {
			body := responseWithUsage(t, stop, usageCounts(1, 2, 3))
			needle := fmt.Sprintf("%q:%d", member.name, member.value)
			assertUsageUnknown(t, stop, replaceOnce(t, body, needle, needle+","+needle), "usage."+member.name+":duplicate")
		})
	}
}

func TestGenerateUsageIgnoresUnconsumed(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)

	t.Run("differing kinds and unknown members", func(t *testing.T) {
		usage := usageCounts(11, 22, 33)
		usage["prompt_tokens"] = "not a number"
		usage["total_tokens"] = json.RawMessage("[]")
		usage["prompt_tokens_details"] = json.RawMessage(`{"cached_tokens":true}`)
		usage["completion_tokens_details"] = json.RawMessage(`[1,2,3]`)
		usage["unknown_member"] = json.RawMessage("null")
		response := assertBodyAccepted(t, responseWithUsage(t, stop, usage))
		want := llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 22, CacheWrite: 0, Output: 33})
		if response.Usage != want {
			t.Errorf("Usage = %+v, want %+v", response.Usage, want)
		}
	})

	t.Run("a duplicate unconsumed member is ignored", func(t *testing.T) {
		body := replaceOnce(t, stop, `"prompt_tokens":46`, `"prompt_tokens":46,"prompt_tokens":99`)
		response := assertBodyAccepted(t, body)
		if response.Usage != stopFixtureUsage {
			t.Errorf("Usage = %+v, want %+v", response.Usage, stopFixtureUsage)
		}
	})
}

func TestGenerateUsageKeepsVerdict(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	length := readFixture(t, testdataLengthFixture)
	valid := usageCounts(1, 2, 3)
	broken := usageCounts(json.RawMessage("-1"), json.RawMessage("null"), json.RawMessage(`"x"`))

	verdict := func(t *testing.T, body []byte) error {
		t.Helper()
		server := newResponseServer(t, http.StatusOK, body)
		_, err := generateAgainst(t, server.URL, validRequest(), nil)
		return err
	}
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	unexpected := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":"content_filter"`)
	empty := documentOf(t, stop)
	messageOf(t, empty)[keyContent] = ""
	invalid := documentOf(t, stop)
	invalid[keyModel] = float64(1)

	cases := []struct {
		name       string
		validBody  []byte
		brokenBody []byte
		want       error
	}{
		{"accepted", responseWithUsage(t, stop, valid), responseWithUsage(t, stop, broken), nil},
		{"truncated", responseWithUsage(t, length, valid), responseWithUsage(t, length, broken), llm.ErrTruncated},
		{"unexpected finish reason", responseWithUsage(t, unexpected, valid), responseWithUsage(t, unexpected, broken), llm.ErrUnexpectedFinishReason},
		{"empty response", responseWithUsage(t, encodeDocument(t, empty), valid), responseWithUsage(t, encodeDocument(t, empty), broken), llm.ErrEmptyResponse},
		{"invalid response", responseWithUsage(t, encodeDocument(t, invalid), valid), responseWithUsage(t, encodeDocument(t, invalid), broken), ErrInvalidResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validErr := verdict(t, tc.validBody)
			brokenErr := verdict(t, tc.brokenBody)
			if !errors.Is(validErr, tc.want) || !errors.Is(brokenErr, tc.want) {
				t.Errorf("errors = %v, %v, want %v", validErr, brokenErr, tc.want)
			}
			if errText(validErr) != errText(brokenErr) {
				t.Errorf("errors differ: %q vs %q", errText(validErr), errText(brokenErr))
			}
		})
	}
}

func TestGenerateRejectedResponseUsage(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	length := readFixture(t, testdataLengthFixture)
	counts := llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 22, CacheWrite: 0, Output: 33})

	truncated := documentOf(t, length)
	truncated[keyUsage] = usageCounts(11, 22, 33)
	assertBodyRejectedResponse(t, encodeDocument(t, truncated), llm.ErrTruncated, counts)

	unexpected := documentOf(t, stop)
	choiceOf(t, unexpected)[keyFinishReason] = "content_filter"
	unexpected[keyUsage] = usageCounts(11, 22, 33)
	assertBodyRejectedResponse(t, encodeDocument(t, unexpected), llm.ErrUnexpectedFinishReason, counts)

	empty := documentOf(t, stop)
	messageOf(t, empty)[keyContent] = ""
	empty[keyUsage] = usageCounts(11, 22, 33)
	assertBodyRejectedResponse(t, encodeDocument(t, empty), llm.ErrEmptyResponse, counts)

	t.Run("the rejected response's model, not the client's, is carried", func(t *testing.T) {
		// The fixtures' model equals the test client's model, so a body with a
		// distinct model pins that Model comes from the response.
		const responseModel = "deepseek-v4-1-flash"
		document := documentOf(t, length)
		document[keyModel] = responseModel
		document[keyUsage] = usageCounts(11, 22, 33)
		server := newResponseServer(t, http.StatusOK, encodeDocument(t, document))
		response, err := generateAgainst(t, server.URL, validRequest(), nil)
		if !errors.Is(err, llm.ErrTruncated) {
			t.Errorf("Generate() error = %v, want ErrTruncated", err)
		}
		assertRejectedResponse(t, response, err, responseModel, counts)
	})

	t.Run("a broken usage in a truncated response is unknown", func(t *testing.T) {
		body := responseWithUsage(t, length, usageCounts(json.RawMessage("-1"), 0, 0))
		assertBodyRejectedResponse(t, body, llm.ErrTruncated,
			llm.UnknownUsage(llm.RuleNotNonNegativeInteger, keyPromptCacheMissTokens))
	})
}
