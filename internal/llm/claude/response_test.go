//go:build test

package claude

import (
	"bytes"
	"encoding/json"
	"errors"
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
		assertBodyRejected(t, maxTokens, llm.ErrTruncated)
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

	t.Run("unknown members are ignored at every level", func(t *testing.T) {
		document := documentOf(t, endTurn)
		document["unknown_top"] = map[string]any{"nested": []any{1, 2, 3}}
		document["id"] = float64(42)
		blockOfType(t, document, blockThinking)["unknown_block"] = "x"
		textBlockOf(t, document)[keyText] = "answer"
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.Text != "answer" {
			t.Errorf("Text = %q, want %q", response.Text, "answer")
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
		err := assertBodyRejected(t, body, llm.ErrTruncated)
		if errors.Is(err, llm.ErrEmptyResponse) {
			t.Errorf("error %v must not match ErrEmptyResponse", err)
		}
	})

	for _, reason := range []string{"refusal", "stop_sequence", "tool_use", "pause_turn", "model_context_window_exceeded", "surprise_value"} {
		t.Run("unexpected "+reason, func(t *testing.T) {
			body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"`+reason+`"`)
			err := assertBodyRejected(t, body, llm.ErrUnexpectedFinishReason)
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
		err := assertBodyRejected(t, body, llm.ErrUnexpectedFinishReason)
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
		err := assertBodyRejected(t, body, llm.ErrUnexpectedFinishReason)
		if !strings.Contains(err.Error(), long[:maxReasonBytes]) {
			t.Errorf("error %q does not include the first %d bytes of the reason", err, maxReasonBytes)
		}
		if strings.Contains(err.Error(), long[maxReasonBytes:]) {
			t.Errorf("error %q includes the reason beyond %d bytes", err, maxReasonBytes)
		}
	})
}

func TestGenerateValidationOrder(t *testing.T) {
	// The max_tokens fixture has a text block; removing it must still report a
	// truncation, not an empty response.
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
	assertRejected(t, response, err)
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
			assertBodyRejected(t, encodeDocument(t, document), llm.ErrEmptyResponse)
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
			assertBodyRejected(t, encodeDocument(t, document), llm.ErrEmptyResponse)
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

	t.Run("non-consumed duplicates are accepted", func(t *testing.T) {
		body := replaceOnce(t, endTurn, `"id":"msg_`, `"id":"dup","id":"msg_`)
		body = replaceOnce(t, body, `"usage":{`, `"usage":{},"usage":{`)
		body = replaceOnce(t, body, `"signature":"AAAA`, `"signature":"dup","signature":"AAAA`)
		document := documentOf(t, body)
		wantText, _ := textBlockOf(t, document)[keyText].(string)
		response := assertBodyAccepted(t, body)
		if response.Text != wantText {
			t.Errorf("Text = %q, want %q", response.Text, wantText)
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
	t.Run("the fixture's unconsumed members are ignored", func(t *testing.T) {
		// The fixture already carries id, container, stop_sequence,
		// stop_details, usage, diagnostics, and a thinking signature.
		document := documentOf(t, readFixture(t, testdataEndTurnFixture))
		for _, key := range []string{"id", "usage", "stop_details"} {
			if _, ok := document[key]; !ok {
				t.Fatalf("the fixture has no %q member to exercise", key)
			}
		}
		textBlockOf(t, document)[keyText] = "answer"
		response := assertBodyAccepted(t, encodeDocument(t, document))
		if response.Text != "answer" {
			t.Errorf("Text = %q, want %q", response.Text, "answer")
		}
	})

	t.Run("unknown members of every kind are ignored", func(t *testing.T) {
		body := []byte(`{"model":"claude-opus-5-5","stop_reason":"end_turn",` +
			`"id":42,"unknown_top":[1,2,3],` +
			`"content":[{"type":"text","text":"answer","citations":[{"x":1}],"unknown_block":true}],` +
			`"usage":{"output_tokens":1}}`)
		response := assertBodyAccepted(t, body)
		if response.Text != "answer" {
			t.Errorf("Text = %q, want %q", response.Text, "answer")
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
}

func TestGenerateStopReasonMessageNamesRequestValues(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"max_tokens"`)
	document := documentOf(t, body)
	document[keyContent] = []any{blockOfType(t, document, blockThinking)}
	err := assertBodyRejected(t, encodeDocument(t, document), llm.ErrTruncated)
	if !strings.Contains(err.Error(), strconv.Itoa(defaultMaxOutputTokens)) {
		t.Errorf("error %q does not name the sent max_tokens", err)
	}
	if !strings.Contains(err.Error(), "lower the effort") {
		t.Errorf("error %q does not advise lowering the effort", err)
	}
}
