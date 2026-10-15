//go:build test

package claude

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
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

// assertBodyAccepted runs Generate against a server that answers with body
// and requires success.
func assertBodyAccepted(t *testing.T, body []byte) llm.GenerateResponse {
	t.Helper()
	server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
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
	server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
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
	wantText, _ := textBlockOf(t, document)[keyText].(string)
	wantModel, _ := document[keyModel].(string)
	if response.Text != wantText || response.Model != wantModel || response.ModelVersion != "" {
		t.Errorf("response = %+v, want Text %q, Model %q, ModelVersion %q", response, wantText, wantModel, "")
	}
	if _, known := response.Usage.Counts(); known {
		t.Fatalf("Usage = %+v, want an unknown usage", response.Usage)
	}
	if got := response.Usage.UnknownReason(); got != want {
		t.Errorf("UnknownReason() = %q, want %q", got, want)
	}
}

func TestGenerateResponseFixtures(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	maxTokens := readFixture(t, testdataMaxTokensFixture)

	t.Run("end_turn fixture", func(t *testing.T) {
		document := documentOf(t, endTurn)
		response := assertBodyAccepted(t, endTurn)
		wantText, _ := textBlockOf(t, document)[keyText].(string)
		wantModel, _ := document[keyModel].(string)
		if response.Text != wantText {
			t.Errorf("Text = %q, want %q", response.Text, wantText)
		}
		if response.Model != wantModel {
			t.Errorf("Model = %q, want %q", response.Model, wantModel)
		}
		if response.ModelVersion != "" {
			t.Errorf("ModelVersion = %q, want an empty string", response.ModelVersion)
		}
	})

	t.Run("max_tokens fixture is truncated", func(t *testing.T) {
		assertBodyRejectedResponse(t, maxTokens, llm.ErrTruncated, maxTokensFixtureModel, maxTokensFixtureUsage)
	})

	t.Run("text whitespace is preserved", func(t *testing.T) {
		document := documentOf(t, endTurn)
		text, _ := textBlockOf(t, document)[keyText].(string)
		want := " \n" + text + "\t "
		textBlockOf(t, document)[keyText] = want
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.Text != want {
			t.Errorf("Text = %q, want %q", response.Text, want)
		}
	})
}

func TestGenerateModelFromResponse(t *testing.T) {
	document := documentOf(t, readFixture(t, testdataEndTurnFixture))
	const responseModel = "claude-sonnet-5-5"
	document[keyModel] = responseModel
	response := assertBodyAccepted(t, encodeDocument(t, document))
	if response.Model != responseModel {
		t.Errorf("Model = %q, want %q", response.Model, responseModel)
	}
}

func TestGenerateStopReason(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)

	t.Run("max_tokens with non-empty text is truncated", func(t *testing.T) {
		body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"max_tokens"`)
		err := assertBodyRejectedResponse(t, body, llm.ErrTruncated, endTurnFixtureModel, endTurnFixtureUsage)
		if errors.Is(err, llm.ErrEmptyResponse) {
			t.Errorf("error %v must not match ErrEmptyResponse", err)
		}
	})

	for _, reason := range []string{"refusal", "stop_sequence", "tool_use", "pause_turn", "model_context_window_exceeded", "surprise_value"} {
		t.Run("unexpected "+reason, func(t *testing.T) {
			body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"`+reason+`"`)
			err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, endTurnFixtureModel, endTurnFixtureUsage)
			if errors.Is(err, llm.ErrTruncated) {
				t.Errorf("error %v must not match ErrTruncated", err)
			}
			if !strings.Contains(err.Error(), reason) {
				t.Errorf("error %q does not name the stop_reason %q", err, reason)
			}
		})
	}

	t.Run("unexpected empty string", func(t *testing.T) {
		body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":""`)
		err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, endTurnFixtureModel, endTurnFixtureUsage)
		if errors.Is(err, ErrInvalidResponse) {
			t.Errorf("error %v must not match ErrInvalidResponse", err)
		}
	})

	t.Run("a long unexpected stop_reason is capped", func(t *testing.T) {
		const long = "reason-longer-than-the-sixty-four-byte-cap-0123456789abcdefghijklmnopqrstuvwxyz"
		if len(long) <= maxReasonBytes {
			t.Fatalf("the test reason must be longer than %d bytes", maxReasonBytes)
		}
		body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"`+long+`"`)
		err := assertBodyRejectedResponse(t, body, llm.ErrUnexpectedFinishReason, endTurnFixtureModel, endTurnFixtureUsage)
		if !strings.Contains(err.Error(), long[:maxReasonBytes]) {
			t.Errorf("error %q does not include the first %d bytes of the reason", err, maxReasonBytes)
		}
		if strings.Contains(err.Error(), long[maxReasonBytes:]) {
			t.Errorf("error %q includes the reason beyond %d bytes", err, maxReasonBytes)
		}
	})
}

func TestGenerateValidationOrder(t *testing.T) {
	// A max_tokens response with no text block must report a truncation, not
	// an empty response, and the message must name the sent max_tokens and
	// advise lowering the effort.
	document := documentOf(t, readFixture(t, testdataEndTurnFixture))
	document[keyContent] = []any{blockOfType(t, document, blockThinking)}
	document[keyStopReason] = stopMaxTokens
	response, err := generateAgainst(t, llmhttptest.NewResponseServer(t, http.StatusOK, encodeDocument(t, document)).URL, validRequest(), nil)
	if !errors.Is(err, llm.ErrTruncated) {
		t.Errorf("Generate() error = %v, want ErrTruncated", err)
	}
	if errors.Is(err, llm.ErrEmptyResponse) {
		t.Errorf("Generate() error = %v, must not match ErrEmptyResponse", err)
	}
	assertRejectedResponse(t, response, err, endTurnFixtureModel, endTurnFixtureUsage)
	if !strings.Contains(err.Error(), strconv.Itoa(defaultMaxOutputTokens)) {
		t.Errorf("error %q does not name the sent max_tokens", err)
	}
	if !strings.Contains(err.Error(), "lower the effort") {
		t.Errorf("error %q does not advise lowering the effort", err)
	}
}

func TestGenerateEmptyText(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)

	noText := map[string]func(document map[string]any){
		"empty content array": func(d map[string]any) { d[keyContent] = []any{} },
		"thinking block only": func(d map[string]any) {
			d[keyContent] = []any{blockOfType(t, d, blockThinking)}
		},
		"redacted_thinking block only": func(d map[string]any) {
			d[keyContent] = []any{map[string]any{keyType: blockRedactedThinking, "data": "x"}}
		},
	}
	for name, mutate := range noText {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, endTurn)
			mutate(document)
			assertBodyRejectedResponse(t, encodeDocument(t, document), llm.ErrEmptyResponse, endTurnFixtureModel, endTurnFixtureUsage)
		})
	}

	whitespace := map[string]string{
		"empty":    "",
		"spaces":   "   ",
		"newlines": "\n\r\n",
		"tabs":     "\t\t",
	}
	for name, text := range whitespace {
		t.Run("text "+name, func(t *testing.T) {
			document := documentOf(t, endTurn)
			textBlockOf(t, document)[keyText] = text
			assertBodyRejectedResponse(t, encodeDocument(t, document), llm.ErrEmptyResponse, endTurnFixtureModel, endTurnFixtureUsage)
		})
	}
}

func TestGenerateThinkingBlocks(t *testing.T) {
	const thinkingMarker = "THINKING-MARKER-8481"
	const redactedMarker = "REDACTED-MARKER-8482"

	t.Run("thinking content is not part of Text", func(t *testing.T) {
		document := documentOf(t, readFixture(t, testdataEndTurnFixture))
		blockOfType(t, document, blockThinking)["thinking"] = thinkingMarker
		textBlockOf(t, document)[keyText] = "the answer"
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.Text != "the answer" {
			t.Errorf("Text = %q, want %q", response.Text, "the answer")
		}
		if strings.Contains(response.Text, thinkingMarker) {
			t.Errorf("Text %q contains the thinking marker", response.Text)
		}
	})

	t.Run("block order does not matter", func(t *testing.T) {
		textBlock := map[string]any{keyType: blockText, keyText: "the answer"}
		thinkingBlock := map[string]any{keyType: blockThinking, "thinking": thinkingMarker}
		redactedBlock := map[string]any{keyType: blockRedactedThinking, "data": redactedMarker}
		for _, order := range [][]any{
			{textBlock, thinkingBlock, redactedBlock},
			{thinkingBlock, textBlock},
			{redactedBlock, thinkingBlock, textBlock},
		} {
			document := documentOf(t, readFixture(t, testdataEndTurnFixture))
			document[keyContent] = order
			response := assertBodyAccepted(t, encodeDocument(t, document))
			if response.Text != "the answer" {
				t.Errorf("Text = %q, want %q", response.Text, "the answer")
			}
		}
	})
}

func TestGenerateInvalidBody(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	bodies := map[string][]byte{
		"empty":                  {},
		"whitespace only":        []byte(" \n\t"),
		"not JSON":               []byte("not json"),
		"top-level array":        []byte("[]"),
		"top-level null":         []byte("null"),
		"top-level string":       []byte(`"text"`),
		"top-level number":       []byte("42"),
		"trailing object":        append(append([]byte{}, endTurn...), []byte(" {}")...),
		"trailing text":          append(append([]byte{}, endTurn...), []byte(" x")...),
		"top-level empty object": []byte("{}"),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			assertBodyRejected(t, body, ErrInvalidResponse)
		})
	}
}

func TestGenerateConsumedMembers(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	mutations := map[string]func(t *testing.T, document map[string]any){
		"missing model":       func(_ *testing.T, d map[string]any) { delete(d, keyModel) },
		"null model":          func(_ *testing.T, d map[string]any) { d[keyModel] = nil },
		"empty model":         func(_ *testing.T, d map[string]any) { d[keyModel] = "" },
		"numeric model":       func(_ *testing.T, d map[string]any) { d[keyModel] = float64(3) },
		"missing stop_reason": func(_ *testing.T, d map[string]any) { delete(d, keyStopReason) },
		"null stop_reason":    func(_ *testing.T, d map[string]any) { d[keyStopReason] = nil },
		"numeric stop_reason": func(_ *testing.T, d map[string]any) { d[keyStopReason] = float64(1) },
		"missing content":     func(_ *testing.T, d map[string]any) { delete(d, keyContent) },
		"null content":        func(_ *testing.T, d map[string]any) { d[keyContent] = nil },
		"object content":      func(_ *testing.T, d map[string]any) { d[keyContent] = map[string]any{} },
		"string content":      func(_ *testing.T, d map[string]any) { d[keyContent] = "x" },
		"missing type":        func(t *testing.T, d map[string]any) { delete(textBlockOf(t, d), keyType) },
		"null type":           func(t *testing.T, d map[string]any) { textBlockOf(t, d)[keyType] = nil },
		"numeric type":        func(t *testing.T, d map[string]any) { textBlockOf(t, d)[keyType] = float64(1) },
		"missing text":        func(t *testing.T, d map[string]any) { delete(textBlockOf(t, d), keyText) },
		"null text":           func(t *testing.T, d map[string]any) { textBlockOf(t, d)[keyText] = nil },
		"numeric text":        func(t *testing.T, d map[string]any) { textBlockOf(t, d)[keyText] = float64(1) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, endTurn)
			mutate(t, document)
			err := assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
			if errors.Is(err, llm.ErrEmptyResponse) {
				t.Errorf("error %v must not match ErrEmptyResponse", err)
			}
		})
	}
}

func TestGenerateContentShape(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	mutations := map[string]func(t *testing.T, document map[string]any){
		"null element":      func(_ *testing.T, d map[string]any) { d[keyContent] = []any{nil} },
		"string element":    func(_ *testing.T, d map[string]any) { d[keyContent] = []any{"x"} },
		"number element":    func(_ *testing.T, d map[string]any) { d[keyContent] = []any{float64(1)} },
		"unknown type":      func(_ *testing.T, d map[string]any) { textBlockOf(t, d)[keyType] = "tool_use" },
		"two text blocks":   func(t *testing.T, d map[string]any) { d[keyContent] = []any{textBlockOf(t, d), textBlockOf(t, d)} },
		"empty type string": func(_ *testing.T, d map[string]any) { textBlockOf(t, d)[keyType] = "" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			document := documentOf(t, endTurn)
			mutate(t, document)
			err := assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
			if errors.Is(err, llm.ErrEmptyResponse) {
				t.Errorf("error %v must not match ErrEmptyResponse", err)
			}
		})
	}
}

func TestGenerateDuplicateMembers(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	rejected := []struct {
		name string
		body []byte
	}{
		{"model with equal values", replaceOnce(t, endTurn,
			`"model":"claude-opus-5-5"`, `"model":"claude-opus-5-5","model":"claude-opus-5-5"`)},
		{"model with different values", replaceOnce(t, endTurn,
			`"model":"claude-opus-5-5"`, `"model":"claude-opus-5-5","model":"other"`)},
		{"stop_reason with equal values", replaceOnce(t, endTurn,
			`"stop_reason":"end_turn"`, `"stop_reason":"end_turn","stop_reason":"end_turn"`)},
		{"content", replaceOnce(t, endTurn, `"content":[`, `"content":[],"content":[`)},
		{"text", replaceOnce(t, endTurn, `{"type":"text","text":"`, `{"type":"text","text":"dup","text":"`)},
		{"type", replaceOnce(t, endTurn, `{"type":"text","text":"`, `{"type":"text","type":"text","text":"`)},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			assertBodyRejected(t, tc.body, ErrInvalidResponse)
		})
	}

	t.Run("unconsumed duplicates are accepted and a duplicate usage is unknown", func(t *testing.T) {
		// id and signature are unconsumed, so their duplicates are ignored.
		// The usage member is read, but a duplicate makes it unknown rather
		// than rejecting a response whose body is otherwise valid.
		body := replaceOnce(t, endTurn, `"id":"msg_`, `"id":"dup","id":"msg_`)
		body = replaceOnce(t, body, `"usage":{`, `"usage":{},"usage":{`)
		body = replaceOnce(t, body, `"signature":"AAAA`, `"signature":"dup","signature":"AAAA`)
		document := documentOf(t, body)
		wantText, _ := textBlockOf(t, document)[keyText].(string)
		response := assertBodyAccepted(t, body)
		if response.Text != wantText {
			t.Errorf("Text = %q, want %q", response.Text, wantText)
		}
		if _, known := response.Usage.Counts(); known {
			t.Errorf("Usage = %+v, want unknown after a duplicate usage member", response.Usage)
		}
	})
}

func TestGenerateEncoding(t *testing.T) {
	bodyWith := func(contentJSON string) []byte {
		return []byte(`{"model":"claude-opus-5-5","stop_reason":"end_turn","content":` + contentJSON + `}`)
	}
	invalidUTF8Text := bodyWith(`[{"type":"text","text":"` + "\xff" + `"}]`)
	invalidUTF8Thinking := bodyWith(`[{"type":"thinking","thinking":"` + "\xff" + `"},{"type":"text","text":"ok"}]`)
	surrogateText := bodyWith(`[{"type":"text","text":"\ud800"}]`)
	surrogateThinking := bodyWith(`[{"type":"thinking","thinking":"\ud800"},{"type":"text","text":"ok"}]`)

	if utf8.Valid(invalidUTF8Text) || utf8.Valid(invalidUTF8Thinking) {
		t.Fatal("the invalid-UTF-8 inputs must not be valid UTF-8")
	}
	if !utf8.Valid(surrogateText) || !json.Valid(surrogateText) {
		t.Fatal("the surrogate input must be valid UTF-8 and JSON, so only the surrogate check can reject it")
	}
	if !utf8.Valid(surrogateThinking) || !json.Valid(surrogateThinking) {
		t.Fatal("the thinking surrogate input must be valid UTF-8 and JSON")
	}

	rejected := map[string][]byte{
		"invalid UTF-8 in text":          invalidUTF8Text,
		"invalid UTF-8 in thinking":      invalidUTF8Thinking,
		"unpaired surrogate in text":     surrogateText,
		"unpaired surrogate in thinking": surrogateThinking,
	}
	for name, body := range rejected {
		t.Run(name, func(t *testing.T) {
			assertBodyRejected(t, body, ErrInvalidResponse)
		})
	}

	t.Run("a valid surrogate pair is accepted", func(t *testing.T) {
		response := assertBodyAccepted(t, bodyWith(`[{"type":"text","text":"\ud83d\ude00"}]`))
		if response.Text != "\U0001F600" {
			t.Errorf("Text = %q, want the decoded surrogate pair", response.Text)
		}
	})
}

func TestGenerateSizeLimit(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)

	t.Run("exactly the limit", func(t *testing.T) {
		body := append(append([]byte{}, endTurn...), bytes.Repeat([]byte(" "), maxResponseBytes-len(endTurn))...)
		if len(body) != maxResponseBytes {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes)
		}
		assertBodyAccepted(t, body)
	})

	t.Run("limit with leading blank lines", func(t *testing.T) {
		const blankLines = "\n\r\n\n"
		body := append([]byte(blankLines), endTurn...)
		body = append(body, bytes.Repeat([]byte(" "), maxResponseBytes-len(body))...)
		if len(body) != maxResponseBytes {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes)
		}
		assertBodyAccepted(t, body)
	})

	t.Run("one byte over the limit", func(t *testing.T) {
		body := append(append([]byte{}, endTurn...), bytes.Repeat([]byte(" "), maxResponseBytes-len(endTurn)+1)...)
		if len(body) != maxResponseBytes+1 {
			t.Fatalf("body is %d bytes, want %d", len(body), maxResponseBytes+1)
		}
		assertBodyRejected(t, body, ErrInvalidResponse)
	})
}

func TestGenerateUnconsumedMembers(t *testing.T) {
	// Unconsumed members (the id, container, stop_sequence, stop_details,
	// diagnostics, and thinking signature; unknown members of any kind; and
	// the unconsumed members of usage) are ignored, so the response is still
	// built. The usage here omits input_tokens, so it becomes unknown rather
	// than rejecting the response.
	body := []byte(`{"model":"claude-opus-5-5","stop_reason":"end_turn",` +
		`"id":42,"unknown_top":[1,2,3],` +
		`"content":[{"type":"text","text":"answer","citations":[{"x":1}],"unknown_block":true}],` +
		`"usage":{"output_tokens":1}}`)
	response := assertBodyAccepted(t, body)
	if response.Text != "answer" {
		t.Errorf("Text = %q, want %q", response.Text, "answer")
	}
	if _, known := response.Usage.Counts(); known {
		t.Errorf("Usage = %+v, want unknown for a usage missing input_tokens", response.Usage)
	}
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
}

func TestGenerateUsageFixtures(t *testing.T) {
	response := assertBodyAccepted(t, readFixture(t, testdataEndTurnFixture))
	if response.Usage != endTurnFixtureUsage {
		t.Errorf("Usage = %+v, want %+v", response.Usage, endTurnFixtureUsage)
	}
}

func TestGenerateUsageCategories(t *testing.T) {
	const input, cacheRead, cacheWrite, output = 11, 22, 33, 44
	body := responseWithUsage(t, readFixture(t, testdataEndTurnFixture), usageCounts(input, cacheRead, cacheWrite, output))
	response := assertBodyAccepted(t, body)
	want := llm.KnownUsage(llm.TokenCounts{Input: input, CacheRead: cacheRead, CacheWrite: cacheWrite, Output: output})
	if response.Usage != want {
		t.Errorf("Usage = %+v, want %+v", response.Usage, want)
	}
}

func TestGenerateUsageNullCache(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)

	for _, member := range []string{keyCacheReadInputTokens, keyCacheCreationInputTokens} {
		t.Run(member+" null is zero", func(t *testing.T) {
			usage := usageCounts(75, 0, 0, 49)
			usage[member] = json.RawMessage("null")
			response := assertBodyAccepted(t, responseWithUsage(t, endTurn, usage))
			if response.Usage != endTurnFixtureUsage {
				t.Errorf("Usage = %+v, want %+v", response.Usage, endTurnFixtureUsage)
			}
		})
	}
}

func TestGenerateUsageUnknown(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	// Each consumed member, its value in the valid usage (so a duplicate can be
	// injected at the byte level), and whether null is 0 for it.
	members := []struct {
		name     string
		value    int
		nullable bool
	}{
		{keyInputTokens, 1, false},
		{keyCacheReadInputTokens, 2, true},
		{keyCacheCreationInputTokens, 3, true},
		{keyOutputTokens, 4, false},
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
		{"array", json.RawMessage("[]")},
		{"object", json.RawMessage("{}")},
	}

	t.Run("top-level usage", func(t *testing.T) {
		cases := []struct {
			name string
			body []byte
			want string
		}{
			{"missing", responseWithUsage(t, endTurn, nil), "usage:missing"},
			{"null", responseWithUsage(t, endTurn, json.RawMessage("null")), "usage:not-an-object"},
			{"string", responseWithUsage(t, endTurn, json.RawMessage(`"x"`)), "usage:not-an-object"},
			{"number", responseWithUsage(t, endTurn, json.RawMessage("1")), "usage:not-an-object"},
			{"boolean", responseWithUsage(t, endTurn, json.RawMessage("true")), "usage:not-an-object"},
			{"array", responseWithUsage(t, endTurn, json.RawMessage("[]")), "usage:not-an-object"},
			{"duplicate", replaceOnce(t, endTurn, `"usage":{`, `"usage":{},"usage":{`), "usage:duplicate"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				assertUsageUnknown(t, endTurn, tc.body, tc.want)
			})
		}
	})

	for _, member := range members {
		for _, invalid := range invalidValues {
			t.Run(member.name+" "+invalid.name, func(t *testing.T) {
				usage := usageCounts(1, 2, 3, 4)
				usage[member.name] = invalid.value
				assertUsageUnknown(t, endTurn, responseWithUsage(t, endTurn, usage), "usage."+member.name+":not-a-non-negative-integer")
			})
		}

		if !member.nullable {
			t.Run(member.name+" null", func(t *testing.T) {
				usage := usageCounts(1, 2, 3, 4)
				usage[member.name] = json.RawMessage("null")
				assertUsageUnknown(t, endTurn, responseWithUsage(t, endTurn, usage), "usage."+member.name+":not-a-non-negative-integer")
			})
		}

		t.Run(member.name+" missing", func(t *testing.T) {
			usage := usageCounts(1, 2, 3, 4)
			delete(usage, member.name)
			assertUsageUnknown(t, endTurn, responseWithUsage(t, endTurn, usage), "usage."+member.name+":missing")
		})

		t.Run(member.name+" duplicate", func(t *testing.T) {
			body := responseWithUsage(t, endTurn, usageCounts(1, 2, 3, 4))
			needle := fmt.Sprintf("%q:%d", member.name, member.value)
			assertUsageUnknown(t, endTurn, replaceOnce(t, body, needle, needle+","+needle), "usage."+member.name+":duplicate")
		})
	}
}

func TestGenerateUsageIgnoresUnconsumed(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)

	t.Run("differing kinds and unknown members", func(t *testing.T) {
		usage := usageCounts(11, 22, 33, 44)
		usage["cache_creation"] = json.RawMessage(`{"ephemeral_5m_input_tokens":"x"}`)
		usage["output_tokens_details"] = json.RawMessage("[]")
		usage["server_tool_use"] = json.RawMessage("null")
		usage["service_tier"] = json.RawMessage("[1,2,3]")
		usage["inference_geo"] = json.RawMessage(`{"x":true}`)
		usage["unknown_member"] = "not a number"
		response := assertBodyAccepted(t, responseWithUsage(t, endTurn, usage))
		want := llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 22, CacheWrite: 33, Output: 44})
		if response.Usage != want {
			t.Errorf("Usage = %+v, want %+v", response.Usage, want)
		}
	})

	t.Run("a duplicate unconsumed member is ignored", func(t *testing.T) {
		body := replaceOnce(t, endTurn, `"service_tier":"standard"`, `"service_tier":"standard","service_tier":"other"`)
		response := assertBodyAccepted(t, body)
		if response.Usage != endTurnFixtureUsage {
			t.Errorf("Usage = %+v, want %+v", response.Usage, endTurnFixtureUsage)
		}
	})
}

func TestGenerateUsageKeepsVerdict(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	maxTokens := readFixture(t, testdataMaxTokensFixture)
	valid := usageCounts(1, 2, 3, 4)
	broken := usageCounts(json.RawMessage("-1"), json.RawMessage(`"x"`), json.RawMessage("null"), json.RawMessage("[]"))

	verdict := func(t *testing.T, body []byte) error {
		t.Helper()
		server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
		_, err := generateAgainst(t, server.URL, validRequest(), nil)
		return err
	}
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	unexpected := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"refusal"`)
	empty := documentOf(t, endTurn)
	empty[keyContent] = []any{blockOfType(t, empty, blockThinking)}
	invalid := documentOf(t, endTurn)
	invalid[keyModel] = float64(1)

	cases := []struct {
		name       string
		validBody  []byte
		brokenBody []byte
		want       error
	}{
		{"accepted", responseWithUsage(t, endTurn, valid), responseWithUsage(t, endTurn, broken), nil},
		{"truncated", responseWithUsage(t, maxTokens, valid), responseWithUsage(t, maxTokens, broken), llm.ErrTruncated},
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
	endTurn := readFixture(t, testdataEndTurnFixture)
	maxTokens := readFixture(t, testdataMaxTokensFixture)
	counts := llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 22, CacheWrite: 33, Output: 44})

	truncated := documentOf(t, maxTokens)
	truncated[keyUsage] = usageCounts(11, 22, 33, 44)
	assertBodyRejectedResponse(t, encodeDocument(t, truncated), llm.ErrTruncated, maxTokensFixtureModel, counts)

	unexpected := documentOf(t, endTurn)
	unexpected[keyStopReason] = "refusal"
	unexpected[keyUsage] = usageCounts(11, 22, 33, 44)
	assertBodyRejectedResponse(t, encodeDocument(t, unexpected), llm.ErrUnexpectedFinishReason, endTurnFixtureModel, counts)

	empty := documentOf(t, endTurn)
	empty[keyContent] = []any{blockOfType(t, empty, blockThinking)}
	empty[keyUsage] = usageCounts(11, 22, 33, 44)
	assertBodyRejectedResponse(t, encodeDocument(t, empty), llm.ErrEmptyResponse, endTurnFixtureModel, counts)

	t.Run("a broken usage in a truncated response is unknown", func(t *testing.T) {
		body := responseWithUsage(t, maxTokens, usageCounts(json.RawMessage("-1"), 0, 0, 0))
		assertBodyRejectedResponse(t, body, llm.ErrTruncated, maxTokensFixtureModel,
			llm.UnknownUsage(llm.RuleNotNonNegativeInteger, keyInputTokens))
	})
}
