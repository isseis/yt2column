//go:build test

package deepseek

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
		assertBodyRejected(t, length, llm.ErrTruncated)
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
		server := newResponseServer(t, http.StatusOK, encodeDocument(t, document))
		response, err := generateAgainst(t, server.URL, validRequest(), nil)
		if !errors.Is(err, llm.ErrTruncated) {
			t.Errorf("Generate() error = %v, want ErrTruncated", err)
		}
		assertRejected(t, response, err)
	})

	for _, reason := range []string{"content_filter", "insufficient_system_resource", "tool_calls", "surprise_value"} {
		t.Run("unexpected "+reason, func(t *testing.T) {
			body := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":"`+reason+`"`)
			err := assertBodyRejected(t, body, llm.ErrUnexpectedFinishReason)
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
		err := assertBodyRejected(t, body, llm.ErrUnexpectedFinishReason)
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
	// The length fixture has an empty content: the finish reason must win.
	response, err := generateAgainst(t, newResponseServer(t, http.StatusOK, readFixture(t, testdataLengthFixture)).URL, validRequest(), nil)
	if !errors.Is(err, llm.ErrTruncated) {
		t.Errorf("Generate() error = %v, want ErrTruncated", err)
	}
	if errors.Is(err, llm.ErrEmptyResponse) {
		t.Errorf("Generate() error = %v, must not match ErrEmptyResponse", err)
	}
	assertRejected(t, response, err)
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
			assertBodyRejected(t, encodeDocument(t, document), llm.ErrEmptyResponse)
		})
	}

	t.Run("reasoning_content alone is not an answer", func(t *testing.T) {
		document := documentOf(t, stop)
		messageOf(t, document)[keyContent] = ""
		assertBodyRejected(t, encodeDocument(t, document), llm.ErrEmptyResponse)
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
