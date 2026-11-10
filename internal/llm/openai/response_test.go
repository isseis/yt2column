//go:build test

package openai

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
)

// mutation changes a completed test document before it is encoded.
type mutation func(t *testing.T, document map[string]any)

// bodyOf returns the encoded completed document after mutate.
func bodyOf(t *testing.T, mutate mutation) []byte {
	t.Helper()
	document := completedDocument()
	mutate(t, document)
	return encodeDocument(t, document)
}

func TestResponseShape(t *testing.T) {
	t.Run("the base response is accepted", func(t *testing.T) {
		response := assertAccepted(t, encodeDocument(t, completedDocument()))
		want := llm.GenerateResponse{Text: testAnswer, Model: testResponseModel, Usage: standardUsage(t, testCounts)}
		if response != want {
			t.Errorf("response = %+v, want %+v", response, want)
		}
	})

	base := encodeDocument(t, completedDocument())
	invalidBodies := map[string][]byte{
		"empty":            {},
		"whitespace only":  []byte(" \n\t"),
		"not JSON":         []byte("not json"),
		"top-level array":  []byte("[]"),
		"top-level null":   []byte("null"),
		"empty object":     []byte("{}"),
		"trailing object":  append(append([]byte{}, base...), []byte(" {}")...),
		"trailing text":    append(append([]byte{}, base...), []byte(" x")...),
		"invalid UTF-8":    replaceOnce(t, base, testAnswer, "bad \xff text"),
		"lone surrogate":   replaceOnce(t, base, testAnswer, `bad \ud800 text`),
		"invalid in usage": replaceOnce(t, base, `"total_tokens":160`, `"total_tokens":16x`),
		"unpaired surrogate in an ignored member": replaceOnce(t, base, `"object":"response"`, `"object":"\udc00"`),
	}
	if !utf8.Valid(invalidBodies["lone surrogate"]) || !json.Valid(invalidBodies["lone surrogate"]) {
		t.Fatal("the lone-surrogate body must be valid UTF-8 and JSON, so only the surrogate check rejects it")
	}
	for name, body := range invalidBodies {
		t.Run("rejects "+name, func(t *testing.T) {
			assertBodyRejected(t, body, ErrInvalidResponse)
		})
	}

	rejected := map[string]mutation{
		// top level
		"missing model":  func(_ *testing.T, d map[string]any) { delete(d, keyModel) },
		"empty model":    func(_ *testing.T, d map[string]any) { d[keyModel] = "" },
		"null model":     func(_ *testing.T, d map[string]any) { d[keyModel] = nil },
		"numeric model":  func(_ *testing.T, d map[string]any) { d[keyModel] = 1 },
		"missing status": func(_ *testing.T, d map[string]any) { delete(d, keyStatus) },
		"null status":    func(_ *testing.T, d map[string]any) { d[keyStatus] = nil },
		"numeric status": func(_ *testing.T, d map[string]any) { d[keyStatus] = 1 },
		"missing output": func(_ *testing.T, d map[string]any) { delete(d, keyOutput) },
		"null output":    func(_ *testing.T, d map[string]any) { d[keyOutput] = nil },
		"object output":  func(_ *testing.T, d map[string]any) { d[keyOutput] = map[string]any{} },
		"string error":   func(_ *testing.T, d map[string]any) { d[keyError] = fakeSecretMarker },
		"array error":    func(_ *testing.T, d map[string]any) { d[keyError] = []any{} },
		"incomplete_details object on a completed response": func(_ *testing.T, d map[string]any) {
			d[keyIncompleteDetails] = map[string]any{keyReason: reasonMaxOutputTokens}
		},
		"incomplete without incomplete_details": func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			delete(d, keyIncompleteDetails)
		},
		"incomplete with null incomplete_details": func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
		},
		"incomplete with string incomplete_details": func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			d[keyIncompleteDetails] = reasonMaxOutputTokens
		},
		"incomplete without a reason": func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			d[keyIncompleteDetails] = map[string]any{}
		},
		"incomplete with a numeric reason": func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			d[keyIncompleteDetails] = map[string]any{keyReason: 1}
		},
		// output elements
		"null output element":   func(_ *testing.T, d map[string]any) { d[keyOutput] = []any{nil} },
		"string output element": func(_ *testing.T, d map[string]any) { d[keyOutput] = []any{"x"} },
		"output element without a type": func(t *testing.T, d map[string]any) {
			delete(messageOf(t, d), keyType)
		},
		"numeric output type": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyType] = 1 },
		"unknown output type": func(t *testing.T, d map[string]any) {
			d[keyOutput] = append(outputOf(t, d), map[string]any{keyType: fakeSecretMarker})
		},
		"differently cased output type": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyType] = "Message" },
		"a valid message followed by an invalid element": func(t *testing.T, d map[string]any) {
			d[keyOutput] = append(outputOf(t, d), nil)
		},
		// messages
		"message without a role":   func(t *testing.T, d map[string]any) { delete(messageOf(t, d), keyRole) },
		"message with a user role": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyRole] = "user" },
		"message with a null role": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyRole] = nil },
		"message without a status": func(t *testing.T, d map[string]any) { delete(messageOf(t, d), keyStatus) },
		"message with a null status": func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyStatus] = nil
		},
		"message without content":     func(t *testing.T, d map[string]any) { delete(messageOf(t, d), keyContent) },
		"message with null content":   func(t *testing.T, d map[string]any) { messageOf(t, d)[keyContent] = nil },
		"message with string content": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyContent] = "x" },
		"message with an unknown phase": func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyPhase] = fakeSecretMarker
		},
		"message with a numeric phase": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyPhase] = 1 },
		"message with a differently cased phase": func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyPhase] = "Final_Answer"
		},
		// content elements
		"null content element": func(t *testing.T, d map[string]any) { messageOf(t, d)[keyContent] = []any{nil} },
		"content element without a type": func(t *testing.T, d map[string]any) {
			delete(textPartOf(t, d), keyType)
		},
		"unknown content type": func(t *testing.T, d map[string]any) {
			textPartOf(t, d)[keyType] = fakeSecretMarker
		},
		"output_text without text":   func(t *testing.T, d map[string]any) { delete(textPartOf(t, d), keyText) },
		"output_text with null text": func(t *testing.T, d map[string]any) { textPartOf(t, d)[keyText] = nil },
		"output_text with numeric text": func(t *testing.T, d map[string]any) {
			textPartOf(t, d)[keyText] = 1
		},
		"refusal without its text": func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyContent] = []any{map[string]any{keyType: contentRefusal}}
		},
		"refusal with a numeric text": func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyContent] = []any{map[string]any{keyType: contentRefusal, keyRefusal: 1}}
		},
		"an invalid commentary message": func(t *testing.T, d map[string]any) {
			d[keyOutput] = append(outputOf(t, d), map[string]any{keyType: outputMessage, keyPhase: phaseCommentary})
		},
	}
	for name, mutate := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			assertBodyRejected(t, bodyOf(t, mutate), ErrInvalidResponse)
		})
	}

	duplicates := []struct {
		name, old, replacement string
	}{
		{"model", `"model":"` + testResponseModel + `"`, `"model":"` + testResponseModel + `","model":"` + testResponseModel + `"`},
		{"status", `"status":"completed","usage"`, `"status":"completed","status":"completed","usage"`},
		{"output", `"output":[`, `"output":[],"output":[`},
		{"error", `"error":null`, `"error":null,"error":null`},
		{"incomplete_details", `"incomplete_details":null`, `"incomplete_details":null,"incomplete_details":null`},
		{"message role", `"role":"assistant"`, `"role":"assistant","role":"assistant"`},
		{"message phase", `"phase":"final_answer"`, `"phase":"final_answer","phase":"final_answer"`},
		{"output_text text", `"text":"` + testAnswer + `"`, `"text":"x","text":"` + testAnswer + `"`},
		{"content type", `"text":"` + testAnswer + `","type":"output_text"`, `"text":"` + testAnswer + `","type":"output_text","type":"output_text"`},
		{"model through an escaped key", `"model":"` + testResponseModel + `"`, `"model":"` + testResponseModel + `","model":"other"`},
	}
	for _, tc := range duplicates {
		t.Run("rejects a duplicate "+tc.name, func(t *testing.T) {
			assertBodyRejected(t, replaceOnce(t, base, tc.old, tc.replacement), ErrInvalidResponse)
		})
	}

	t.Run("a differently cased consumed key is not the member", func(t *testing.T) {
		assertBodyRejected(t, replaceOnce(t, base, `"model":`, `"Model":`), ErrInvalidResponse)
	})

	t.Run("an escaped consumed key is the member", func(t *testing.T) {
		body := replaceOnce(t, base, `"model":`, `"model":`)
		if response := assertAccepted(t, body); response.Model != testResponseModel {
			t.Errorf("Model = %q, want %q", response.Model, testResponseModel)
		}
	})

	t.Run("extensible members are ignored, duplicates and other kinds included", func(t *testing.T) {
		body := replaceOnce(t, base, `"id":"resp_test"`, `"id":"resp_test","id":1,"extra":{"model":null}`)
		body = replaceOnce(t, body, `"id":"rs_test"`, `"id":"rs_test","id":null,"status":5,"content":"x","summary":7`)
		body = replaceOnce(t, body, `"annotations":[]`, `"annotations":[],"annotations":"x","logprobs":{}`)
		body = replaceOnce(t, body, `"id":"msg_test"`, `"id":"msg_test","id":[],"extra":true`)
		if response := assertAccepted(t, body); response.Text != testAnswer {
			t.Errorf("Text = %q, want %q", response.Text, testAnswer)
		}
	})

	t.Run("incomplete_details may carry extra members", func(t *testing.T) {
		document := truncatedDocument()
		document[keyIncompleteDetails] = map[string]any{keyReason: reasonMaxOutputTokens, "extra": []any{1}}
		assertBodyRejectedWithResponse(t, encodeDocument(t, document), llm.ErrTruncated, standardUsage(t, testCounts))
	})
}

func TestResponseFinish(t *testing.T) {
	usage := standardUsage(t, testCounts)

	accepted := []struct {
		name   string
		output []any
		want   string
	}{
		{"final answer parts and messages are joined in order without separators", []any{
			messageItem(phaseFinalAnswer, textPart("A "), textPart("b")),
			reasoningItem(),
			messageItem("", textPart("\nc")),
		}, "A b\nc"},
		{"commentary and reasoning are left out", []any{
			messageItem(phaseCommentary, textPart(fakeSecretMarker)),
			reasoningItem(),
			messageItem(phaseFinalAnswer, textPart("answer")),
		}, "answer"},
		{"a null phase is the final answer", []any{
			map[string]any{
				keyType: outputMessage, keyRole: roleAssistant, keyStatus: statusCompleted,
				keyPhase: nil, keyContent: []any{textPart(" kept \t")},
			},
		}, " kept \t"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			response := assertAccepted(t, bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyOutput] = tc.output }))
			if response.Text != tc.want {
				t.Errorf("Text = %q, want %q", response.Text, tc.want)
			}
			if response.Model != testResponseModel || response.ModelVersion != "" {
				t.Errorf("Model = %q, ModelVersion = %q, want %q and empty", response.Model, response.ModelVersion, testResponseModel)
			}
		})
	}

	t.Run("a truncation without text names the limit and advises a lower effort", func(t *testing.T) {
		err := assertBodyRejectedWithResponse(t, encodeDocument(t, truncatedDocument()), llm.ErrTruncated, usage)
		for _, want := range []string{"max_output_tokens " + strconv.Itoa(defaultMaxOutputTokens), "effort medium", "lower the effort"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})

	t.Run("a truncation with text does not advise a lower effort", func(t *testing.T) {
		document := truncatedDocument()
		document[keyOutput] = []any{reasoningItem(), messageItem(phaseFinalAnswer, textPart("partial"))}
		messageOf(t, document)[keyStatus] = statusIncomplete
		err := assertBodyRejectedWithResponse(t, encodeDocument(t, document), llm.ErrTruncated, usage)
		if strings.Contains(err.Error(), "lower the effort") {
			t.Errorf("error %q advises a lower effort although text is present", err)
		}
	})

	t.Run("a truncation wins over every unexpected condition", func(t *testing.T) {
		document := truncatedDocument()
		document[keyError] = map[string]any{"message": fakeSecretMarker}
		document[keyOutput] = []any{map[string]any{
			keyType: outputMessage, keyRole: roleAssistant, keyStatus: statusIncomplete,
			keyContent: []any{map[string]any{keyType: contentRefusal, keyRefusal: fakeSecretMarker}},
		}}
		assertBodyRejectedWithResponse(t, encodeDocument(t, document), llm.ErrTruncated, usage)
	})

	unexpected := []struct {
		name   string
		mutate mutation
		cause  string
	}{
		{"incomplete for another reason", func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			d[keyIncompleteDetails] = map[string]any{keyReason: reasonContentFilter}
		}, "reason content_filter"},
		{"incomplete for an unknown reason", func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusIncomplete
			d[keyIncompleteDetails] = map[string]any{keyReason: fakeSecretMarker}
		}, "reason unknown"},
		{"failed", func(_ *testing.T, d map[string]any) { d[keyStatus] = statusFailed }, "status is failed"},
		{"cancelled", func(_ *testing.T, d map[string]any) { d[keyStatus] = statusCancelled }, "status is cancelled"},
		{"in progress", func(_ *testing.T, d map[string]any) { d[keyStatus] = statusInProgress }, "status is in_progress"},
		{"queued", func(_ *testing.T, d map[string]any) { d[keyStatus] = statusQueued }, "status is queued"},
		{"unknown status with control characters", func(_ *testing.T, d map[string]any) {
			d[keyStatus] = fakeSecretMarker + "\x1b[2J"
		}, "status is unknown"},
		{"empty status", func(_ *testing.T, d map[string]any) { d[keyStatus] = "" }, "status is unknown"},
		{"an error object", func(_ *testing.T, d map[string]any) {
			d[keyError] = map[string]any{"message": fakeSecretMarker}
		}, "error object"},
		{"an unfinished final message", func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyStatus] = statusIncomplete
		}, "message is not completed"},
		{"an unfinished commentary message", func(t *testing.T, d map[string]any) {
			commentary := messageItem(phaseCommentary, textPart("progress"))
			commentary[keyStatus] = fakeSecretMarker
			d[keyOutput] = append(outputOf(t, d), commentary)
		}, "message is not completed"},
		{"a refusal beside the answer", func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyContent] = []any{textPart(testAnswer), map[string]any{keyType: contentRefusal, keyRefusal: fakeSecretMarker}}
		}, "refusal"},
		{"a refusal alone", func(t *testing.T, d map[string]any) {
			messageOf(t, d)[keyContent] = []any{map[string]any{keyType: contentRefusal, keyRefusal: fakeSecretMarker}}
		}, "refusal"},
		{"a failed status wins over an empty answer", func(_ *testing.T, d map[string]any) {
			d[keyStatus] = statusFailed
			d[keyOutput] = []any{}
		}, "status is failed"},
	}
	for _, tc := range unexpected {
		t.Run("unexpected: "+tc.name, func(t *testing.T) {
			err := assertBodyRejectedWithResponse(t, bodyOf(t, tc.mutate), llm.ErrUnexpectedFinishReason, usage)
			if !strings.Contains(err.Error(), tc.cause) {
				t.Errorf("error %q does not name the cause %q", err, tc.cause)
			}
		})
	}

	empty := map[string][]any{
		"no output":                 {},
		"reasoning only":            {reasoningItem()},
		"commentary only":           {messageItem(phaseCommentary, textPart("progress"))},
		"a message without content": {messageItem(phaseFinalAnswer)},
		"empty text":                {messageItem(phaseFinalAnswer, textPart(""))},
		"whitespace text":           {messageItem(phaseFinalAnswer, textPart(" \n\t"), textPart("\r\n"))},
	}
	for name, output := range empty {
		t.Run("empty: "+name, func(t *testing.T) {
			assertBodyRejectedWithResponse(t, bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyOutput] = output }),
				llm.ErrEmptyResponse, usage)
		})
	}
}

// assertUsage runs Generate on body, which must keep its verdict want (nil
// for success), and requires the response usage to be wantUsage.
func assertUsage(t *testing.T, body []byte, want error, wantUsage llm.Usage) {
	t.Helper()
	if want == nil {
		if response := assertAccepted(t, body); response.Usage != wantUsage || response.Text != testAnswer {
			t.Errorf("response = %+v, want Text %q and Usage %+v", response, testAnswer, wantUsage)
		}
		return
	}
	assertBodyRejectedWithResponse(t, body, want, wantUsage)
}

func TestResponseUsage(t *testing.T) {
	t.Run("the categories map without double counting reasoning or adding total_tokens", func(t *testing.T) {
		document := completedDocument()
		usageOf(t, document)["total_tokens"] = 999999
		usageOf(t, document)["output_tokens_details"] = map[string]any{"reasoning_tokens": 39}
		assertUsage(t, encodeDocument(t, document), nil, standardUsage(t, testCounts))
	})

	t.Run("the cache counts may use the whole input", func(t *testing.T) {
		document := completedDocument()
		detailsOf(t, document)[keyCachedTokens] = 70
		assertUsage(t, encodeDocument(t, document), nil,
			standardUsage(t, llm.TokenCounts{Input: 0, CacheRead: 70, CacheWrite: 50, Output: 40}))
	})

	t.Run("the largest int64 counts are accepted", func(t *testing.T) {
		const maxInt64 = "9223372036854775807"
		body := replaceOnce(t, encodeDocument(t, completedDocument()), `"output_tokens":40`, `"output_tokens":`+maxInt64)
		want := testCounts
		want.Output = 1<<63 - 1
		assertUsage(t, body, nil, standardUsage(t, want))
	})

	// unknownRow is a usage mutation and the unknown usage it must produce.
	type unknownRow struct {
		name   string
		mutate mutation
		want   llm.Usage
	}
	unknown := []unknownRow{
		{
			"missing usage", func(_ *testing.T, d map[string]any) { delete(d, keyUsage) },
			llm.UnknownUsage(llm.RuleMissing, ""),
		},
		{
			"null usage", func(_ *testing.T, d map[string]any) { d[keyUsage] = nil },
			llm.UnknownUsage(llm.RuleNotObject, ""),
		},
		{
			"array usage", func(_ *testing.T, d map[string]any) { d[keyUsage] = []any{} },
			llm.UnknownUsage(llm.RuleNotObject, ""),
		},
		{"missing input_tokens_details", func(t *testing.T, d map[string]any) {
			delete(usageOf(t, d), keyInputTokensDetails)
		}, llm.UnknownUsage(llm.RuleMissing, "input_tokens_details")},
		{"null input_tokens_details", func(t *testing.T, d map[string]any) {
			usageOf(t, d)[keyInputTokensDetails] = nil
		}, llm.UnknownUsage(llm.RuleNotObject, "input_tokens_details")},
		{"cache reads and writes beyond the input", func(t *testing.T, d map[string]any) {
			detailsOf(t, d)[keyCachedTokens] = 71
		}, llm.UnknownUsage(llm.RuleInconsistent, "input_tokens_details")},
	}
	counts := []struct{ member, path string }{
		{keyInputTokens, "input_tokens"},
		{keyCachedTokens, "input_tokens_details.cached_tokens"},
		{keyCacheWriteTokens, "input_tokens_details.cache_write_tokens"},
		{keyOutputTokens, "output_tokens"},
	}
	for _, count := range counts {
		holder := func(t *testing.T, d map[string]any) map[string]any {
			if count.path == count.member {
				return usageOf(t, d)
			}
			return detailsOf(t, d)
		}
		unknown = append(unknown, unknownRow{count.path + " missing", func(t *testing.T, d map[string]any) {
			delete(holder(t, d), count.member)
		}, llm.UnknownUsage(llm.RuleMissing, count.path)})
		for _, value := range []any{
			nil, "1", -1, 1.5, json.RawMessage("-0"), json.RawMessage("1e2"),
			json.RawMessage("1.0"), json.RawMessage("9223372036854775808"), true,
		} {
			unknown = append(unknown, unknownRow{count.path + " " + string(mustJSON(t, value)), func(t *testing.T, d map[string]any) {
				holder(t, d)[count.member] = value
			}, llm.UnknownUsage(llm.RuleNotNonNegativeInteger, count.path)})
		}
	}

	// A broken usage never changes the verdict of the body: each row runs on
	// a success and on the three failures that carry the usage.
	verdicts := []struct {
		name string
		base func() map[string]any
		want error
	}{
		{"success", completedDocument, nil},
		{"truncation", truncatedDocument, llm.ErrTruncated},
		{"unexpected finish", func() map[string]any {
			document := completedDocument()
			document[keyStatus] = statusFailed
			return document
		}, llm.ErrUnexpectedFinishReason},
		{"empty answer", func() map[string]any {
			document := completedDocument()
			document[keyOutput] = []any{reasoningItem()}
			return document
		}, llm.ErrEmptyResponse},
	}
	for _, verdict := range verdicts {
		for _, tc := range unknown {
			t.Run(verdict.name+": "+tc.name, func(t *testing.T) {
				document := verdict.base()
				tc.mutate(t, document)
				assertUsage(t, encodeDocument(t, document), verdict.want, tc.want)
			})
		}
	}

	duplicates := []struct {
		name, old, replacement, want string
	}{
		{"usage", `"usage":{`, `"usage":{},"usage":{`, "usage:duplicate"},
		{"input_tokens", `"input_tokens":120`, `"input_tokens":120,"input_tokens":120`, "usage.input_tokens:duplicate"},
		{"input_tokens_details", `"input_tokens_details":{`, `"input_tokens_details":{},"input_tokens_details":{`, "usage.input_tokens_details:duplicate"},
		{"cached_tokens", `"cached_tokens":30`, `"cached_tokens":30,"cached_tokens":30`, "usage.input_tokens_details.cached_tokens:duplicate"},
		{"cache_write_tokens", `"cache_write_tokens":50`, `"cache_write_tokens":50,"cache_write_tokens":50`, "usage.input_tokens_details.cache_write_tokens:duplicate"},
		{"output_tokens", `"output_tokens":40`, `"output_tokens":40,"output_tokens":40`, "usage.output_tokens:duplicate"},
	}
	for _, tc := range duplicates {
		t.Run("duplicate "+tc.name, func(t *testing.T) {
			body := replaceOnce(t, encodeDocument(t, completedDocument()), tc.old, tc.replacement)
			response := assertAccepted(t, body)
			if got := response.Usage.UnknownReason(); got != tc.want {
				t.Errorf("UnknownReason() = %q, want %q", got, tc.want)
			}
			if response.Usage.PricingContext() != llm.PricingContextUnspecified {
				t.Errorf("PricingContext() = %v, want unspecified for an unknown usage", response.Usage.PricingContext())
			}
		})
	}

	t.Run("unconsumed usage members are ignored, duplicates and other kinds included", func(t *testing.T) {
		body := replaceOnce(t, encodeDocument(t, completedDocument()), `"total_tokens":160`,
			`"total_tokens":"x","total_tokens":null,"output_tokens_details":-1`)
		body = replaceOnce(t, body, `"cached_tokens":30`, `"cached_tokens":30,"audio_tokens":"x","audio_tokens":null`)
		assertUsage(t, body, nil, standardUsage(t, testCounts))
	})

	t.Run("a broken usage in an invalid body is not reported", func(t *testing.T) {
		document := completedDocument()
		delete(document, keyUsage)
		delete(document, keyModel)
		assertBodyRejected(t, encodeDocument(t, document), ErrInvalidResponse)
	})
}

func TestResponsePricingContext(t *testing.T) {
	base := encodeDocument(t, completedDocument())
	unverified := []struct {
		name string
		body []byte
	}{
		{"missing", bodyOf(t, func(_ *testing.T, d map[string]any) { delete(d, keyServiceTier) })},
		{"null", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = nil })},
		{"numeric", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = 1 })},
		{"object", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = map[string]any{} })},
		{"another tier", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = "flex" })},
		{"differently cased", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = "Default" })},
		{"unknown with a fake secret", bodyOf(t, func(_ *testing.T, d map[string]any) { d[keyServiceTier] = fakeSecretMarker })},
		{"duplicate with equal values", replaceOnce(t, base, `"service_tier":"default"`, `"service_tier":"default","service_tier":"default"`)},
	}
	for _, tc := range unverified {
		t.Run(tc.name+" is unverified and keeps the text and counts", func(t *testing.T) {
			want, err := llm.KnownUsageWithPricingContext(testCounts, llm.PricingContextUnverified)
			if err != nil {
				t.Fatalf("KnownUsageWithPricingContext error = %v", err)
			}
			assertUsage(t, tc.body, nil, want)
		})
	}

	t.Run("an unverified tier keeps the counts of a truncation", func(t *testing.T) {
		document := truncatedDocument()
		document[keyServiceTier] = "priority"
		want, err := llm.KnownUsageWithPricingContext(testCounts, llm.PricingContextUnverified)
		if err != nil {
			t.Fatalf("KnownUsageWithPricingContext error = %v", err)
		}
		assertBodyRejectedWithResponse(t, encodeDocument(t, document), llm.ErrTruncated, want)
	})

	t.Run("the default tier is the confirmed Standard condition", func(t *testing.T) {
		response := assertAccepted(t, base)
		if got := response.Usage.PricingContext(); got != llm.PricingContextStandard30m {
			t.Errorf("PricingContext() = %v, want PricingContextStandard30m", got)
		}
	})

	t.Run("an unknown usage carries no condition", func(t *testing.T) {
		response := assertAccepted(t, bodyOf(t, func(_ *testing.T, d map[string]any) { delete(d, keyUsage) }))
		if got := response.Usage.PricingContext(); got != llm.PricingContextUnspecified {
			t.Errorf("PricingContext() = %v, want PricingContextUnspecified", got)
		}
	})
}

// mustJSON encodes value for a subtest name.
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode %v: %v", value, err)
	}
	return data
}
