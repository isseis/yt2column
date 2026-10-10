//go:build test

package strictjson

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseObject(t *testing.T) {
	t.Run("accepts one object", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"a":1,"b":"x"}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		if !object.Has("a") || !object.Has("b") {
			t.Errorf("Has(a)=%v Has(b)=%v, want both true", object.Has("a"), object.Has("b"))
		}
	})

	t.Run("accepts an empty object", func(t *testing.T) {
		object, err := ParseObject([]byte(`{}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		if object.Has("a") {
			t.Error("empty object reports a member")
		}
	})

	rejected := map[string]struct {
		data   []byte
		target error
	}{
		"invalid utf8":        {[]byte("{\"a\":\"\xff\"}"), errInvalidEncoding},
		"unpaired high":       {[]byte(`{"a":"\ud800"}`), errUnpairedSurrogate},
		"unpaired low":        {[]byte(`{"a":"\udc00"}`), errUnpairedSurrogate},
		"unpaired in ignored": {[]byte(`{"a":"ok","b":"\ud800"}`), errUnpairedSurrogate},
		"trailing object":     {[]byte(`{"a":1} {}`), errTrailingJSON},
		"trailing text":       {[]byte(`{"a":1} x`), nil},
		"trailing char":       {[]byte(`{"a":1}x`), nil},
		"empty":               {[]byte(``), nil},
		"truncated":           {[]byte(`{`), nil},
		"top-level null":      {[]byte(`null`), errNotJSONObject},
		"top-level array":     {[]byte(`[]`), errNotJSONObject},
		"top-level string":    {[]byte(`"x"`), errNotJSONObject},
		"top-level number":    {[]byte(`42`), errNotJSONObject},
	}
	for name, tc := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			_, err := ParseObject(tc.data)
			if err == nil {
				t.Fatalf("ParseObject(%q) error = nil, want an error", tc.data)
			}
			if tc.target != nil && !errors.Is(err, tc.target) {
				t.Errorf("ParseObject(%q) error = %v, want %v", tc.data, err, tc.target)
			}
		})
	}

	t.Run("accepts a valid surrogate pair", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"a":"\ud83d\ude00"}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("a")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		text, err := consumed["a"].AsString()
		if err != nil {
			t.Fatalf("AsString error = %v", err)
		}
		if text != "\U0001F600" {
			t.Errorf("AsString = %q, want %q", text, "\U0001F600")
		}
	})

	t.Run("accepts an escaped backslash before u", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"a":"\\ud800"}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("a")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		text, err := consumed["a"].AsString()
		if err != nil {
			t.Fatalf("AsString error = %v", err)
		}
		if text != `\ud800` {
			t.Errorf("AsString = %q, want %q", text, `\ud800`)
		}
	})
}

func TestObjectCollect(t *testing.T) {
	object, err := ParseObject([]byte(`{"a":1,"b":"x","c":true,"a":2}`))
	if err != nil {
		t.Fatalf("ParseObject error = %v", err)
	}

	t.Run("missing key is absent", func(t *testing.T) {
		consumed, err := object.Collect("missing")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if len(consumed) != 0 {
			t.Errorf("Collect = %v, want an empty map", consumed)
		}
	})

	t.Run("rejects a duplicate consumed key", func(t *testing.T) {
		_, err := object.Collect("a")
		if !errors.Is(err, errDuplicateMember) {
			t.Errorf("Collect error = %v, want errDuplicateMember", err)
		}
	})

	t.Run("no keys consumes nothing", func(t *testing.T) {
		consumed, err := object.Collect()
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if len(consumed) != 0 {
			t.Errorf("Collect() = %v, want an empty map", consumed)
		}
	})

	t.Run("accepts duplicate ignored keys", func(t *testing.T) {
		consumed, err := object.Collect("b", "c")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if len(consumed) != 2 {
			t.Fatalf("Collect returned %d members, want 2", len(consumed))
		}
		text, err := consumed["b"].AsString()
		if err != nil || text != "x" {
			t.Errorf("b = %q, %v; want x, nil", text, err)
		}
	})

	t.Run("accepts a duplicate ignored when another key is consumed", func(t *testing.T) {
		other, err := ParseObject([]byte(`{"ignore":1,"ignore":2,"keep":"v"}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := other.Collect("keep")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		text, err := consumed["keep"].AsString()
		if err != nil || text != "v" {
			t.Errorf("keep = %q, %v; want v, nil", text, err)
		}
	})
}

func TestObjectCollectOnly(t *testing.T) {
	t.Run("accepts listed keys with one missing", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"a":1}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.CollectOnly("a", "b")
		if err != nil {
			t.Fatalf("CollectOnly error = %v", err)
		}
		if len(consumed) != 1 {
			t.Fatalf("CollectOnly returned %d members, want 1", len(consumed))
		}
		if value, err := consumed["a"].AsInt64(); err != nil || value != 1 {
			t.Errorf("a = %d, %v; want 1, nil", value, err)
		}
	})

	rejected := map[string]struct {
		document string
		keys     []string
		target   error
	}{
		"unlisted key":           {`{"a":1,"marker-leak":2}`, []string{"a"}, errUnlistedMember},
		"duplicate unlisted key": {`{"marker-leak":1,"marker-leak":2}`, []string{"a"}, errUnlistedMember},
		"duplicate listed key":   {`{"marker-leak":1,"marker-leak":2}`, []string{"marker-leak"}, errDuplicateMember},
	}
	for name, tc := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			object, err := ParseObject([]byte(tc.document))
			if err != nil {
				t.Fatalf("ParseObject error = %v", err)
			}
			_, err = object.CollectOnly(tc.keys...)
			if !errors.Is(err, tc.target) {
				t.Errorf("CollectOnly error = %v, want %v", err, tc.target)
			}
			if err != nil && strings.Contains(err.Error(), "marker-leak") {
				t.Errorf("CollectOnly error = %q, must not contain the key", err.Error())
			}
		})
	}
}

func TestObjectHas(t *testing.T) {
	object, err := ParseObject([]byte(`{"a":null,"b":{}}`))
	if err != nil {
		t.Fatalf("ParseObject error = %v", err)
	}
	if !object.Has("a") {
		t.Error("Has(a) = false, want true for a null member")
	}
	if !object.Has("b") {
		t.Error("Has(b) = false, want true")
	}
	if object.Has("c") {
		t.Error("Has(c) = true, want false")
	}

	var zero Object
	if zero.Has("a") {
		t.Error("zero Object.Has(a) = true, want false")
	}
}

func TestRequiredAndRequiredString(t *testing.T) {
	object, err := ParseObject([]byte(`{"title":"t","empty":"","null":null,"number":1}`))
	if err != nil {
		t.Fatalf("ParseObject error = %v", err)
	}
	consumed, err := object.Collect("title", "empty", "null", "number")
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}

	t.Run("Required accepts a present member", func(t *testing.T) {
		value, err := Required(consumed, "title")
		if err != nil {
			t.Fatalf("Required error = %v", err)
		}
		text, err := value.AsString()
		if err != nil || text != "t" {
			t.Errorf("title = %q, %v; want t, nil", text, err)
		}
	})

	t.Run("Required rejects a missing member", func(t *testing.T) {
		_, err := Required(consumed, "missing")
		if !errors.Is(err, errMissingField) {
			t.Errorf("Required error = %v, want errMissingField", err)
		}
	})

	t.Run("RequiredString accepts a non-empty string", func(t *testing.T) {
		text, err := RequiredString(consumed, "title")
		if err != nil || text != "t" {
			t.Errorf("RequiredString = %q, %v; want t, nil", text, err)
		}
	})

	rejected := map[string]struct {
		key    string
		target error
	}{
		"missing":    {"missing", errMissingField},
		"empty":      {"empty", errEmptyField},
		"null":       {"null", errValueNotString},
		"non-string": {"number", errValueNotString},
	}
	for name, tc := range rejected {
		t.Run("RequiredString rejects "+name, func(t *testing.T) {
			if _, err := RequiredString(consumed, tc.key); !errors.Is(err, tc.target) {
				t.Errorf("RequiredString(%q) error = %v, want %v", tc.key, err, tc.target)
			}
		})
	}
}

func TestOptionalString(t *testing.T) {
	object, err := ParseObject([]byte(`{"present":"v","empty":"","null":null,"number":1}`))
	if err != nil {
		t.Fatalf("ParseObject error = %v", err)
	}
	consumed, err := object.Collect("present", "empty", "null", "number")
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}

	t.Run("missing member is absent", func(t *testing.T) {
		value, present, err := OptionalString(consumed, "missing")
		if err != nil || present || value != "" {
			t.Errorf("OptionalString = %q, %v, %v; want empty, false, nil", value, present, err)
		}
	})

	t.Run("present member is returned", func(t *testing.T) {
		value, present, err := OptionalString(consumed, "present")
		if err != nil || !present || value != "v" {
			t.Errorf("OptionalString = %q, %v, %v; want v, true, nil", value, present, err)
		}
	})

	rejected := map[string]struct {
		key    string
		target error
	}{
		"empty":      {"empty", errEmptyField},
		"null":       {"null", errValueNotString},
		"non-string": {"number", errValueNotString},
	}
	for name, tc := range rejected {
		t.Run("OptionalString rejects "+name, func(t *testing.T) {
			if _, _, err := OptionalString(consumed, tc.key); !errors.Is(err, tc.target) {
				t.Errorf("OptionalString(%q) error = %v, want %v", tc.key, err, tc.target)
			}
		})
	}
}

func TestValueAccessors(t *testing.T) {
	t.Run("string kinds", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"s":"x","n":1,"b":true,"o":{},"a":[]}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("s", "n", "b", "o", "a")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if text, err := consumed["s"].AsString(); err != nil || text != "x" {
			t.Errorf("AsString(s) = %q, %v; want x, nil", text, err)
		}
		for _, key := range []string{"n", "b", "o", "a"} {
			if _, err := consumed[key].AsString(); !errors.Is(err, errValueNotString) {
				t.Errorf("AsString(%q) error = %v, want errValueNotString", key, err)
			}
		}
	})

	t.Run("number kinds", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"positive":7,"negative":-3,"fraction":1.5,"overflow":9223372036854775808,"string":"7"}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("positive", "negative", "fraction", "overflow", "string")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if value, err := consumed["positive"].AsInt64(); err != nil || value != 7 {
			t.Errorf("AsInt64(positive) = %d, %v; want 7, nil", value, err)
		}
		if value, err := consumed["negative"].AsInt64(); err != nil || value != -3 {
			t.Errorf("AsInt64(negative) = %d, %v; want -3, nil", value, err)
		}
		if _, err := consumed["fraction"].AsInt64(); err == nil {
			t.Error("AsInt64(fraction) error = nil, want an error")
		}
		if _, err := consumed["overflow"].AsInt64(); err == nil {
			t.Error("AsInt64(overflow) error = nil, want an error")
		}
		if _, err := consumed["string"].AsInt64(); !errors.Is(err, errValueNotNumber) {
			t.Errorf("AsInt64(string) error = %v, want errValueNotNumber", err)
		}
	})

	t.Run("integer kind mismatches", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"null":null,"bool":true,"object":{},"array":[]}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("null", "bool", "object", "array")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		for _, key := range []string{"null", "bool", "object", "array"} {
			if _, err := consumed[key].AsInt64(); !errors.Is(err, errValueNotNumber) {
				t.Errorf("AsInt64(%q) error = %v, want errValueNotNumber", key, err)
			}
		}
	})

	t.Run("object and array kinds", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"o":{"x":1},"a":[1,"x",null,{}],"n":null}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("o", "a", "n")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}

		nested, err := consumed["o"].AsObject()
		if err != nil {
			t.Fatalf("AsObject(o) error = %v", err)
		}
		nestedConsumed, err := nested.Collect("x")
		if err != nil {
			t.Fatalf("Collect(x) error = %v", err)
		}
		if value, err := nestedConsumed["x"].AsInt64(); err != nil || value != 1 {
			t.Errorf("AsInt64(x) = %d, %v; want 1, nil", value, err)
		}

		values, err := consumed["a"].AsArray()
		if err != nil {
			t.Fatalf("AsArray(a) error = %v", err)
		}
		if len(values) != 4 {
			t.Fatalf("AsArray(a) returned %d values, want 4", len(values))
		}
		if value, err := values[0].AsInt64(); err != nil || value != 1 {
			t.Errorf("values[0] = %d, %v; want 1, nil", value, err)
		}
		if text, err := values[1].AsString(); err != nil || text != "x" {
			t.Errorf("values[1] = %q, %v; want x, nil", text, err)
		}
		if _, err := values[2].AsString(); !errors.Is(err, errValueNotString) {
			t.Errorf("values[2].AsString error = %v, want errValueNotString", err)
		}
		if _, err := values[3].AsObject(); err != nil {
			t.Errorf("values[3].AsObject error = %v", err)
		}

		if _, err := consumed["n"].AsObject(); !errors.Is(err, errNotJSONObject) {
			t.Errorf("AsObject(null) error = %v, want errNotJSONObject", err)
		}
		if _, err := consumed["n"].AsArray(); !errors.Is(err, errNotArray) {
			t.Errorf("AsArray(null) error = %v, want errNotArray", err)
		}
		if _, err := consumed["o"].AsArray(); !errors.Is(err, errNotArray) {
			t.Errorf("AsArray(object) error = %v, want errNotArray", err)
		}
		if _, err := consumed["a"].AsObject(); !errors.Is(err, errNotJSONObject) {
			t.Errorf("AsObject(array) error = %v, want errNotJSONObject", err)
		}
	})

	t.Run("zero value rejects every accessor", func(t *testing.T) {
		var zero Value
		if _, err := zero.AsString(); !errors.Is(err, errZeroValue) {
			t.Errorf("AsString error = %v, want errZeroValue", err)
		}
		if _, err := zero.AsInt64(); !errors.Is(err, errZeroValue) {
			t.Errorf("AsInt64 error = %v, want errZeroValue", err)
		}
		if _, err := zero.AsObject(); !errors.Is(err, errZeroValue) {
			t.Errorf("AsObject error = %v, want errZeroValue", err)
		}
		if _, err := zero.AsArray(); !errors.Is(err, errZeroValue) {
			t.Errorf("AsArray error = %v, want errZeroValue", err)
		}
	})

	t.Run("array element order is preserved", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"a":["first","second"]}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("a")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		values, err := consumed["a"].AsArray()
		if err != nil {
			t.Fatalf("AsArray error = %v", err)
		}
		var got []string
		for _, value := range values {
			text, err := value.AsString()
			if err != nil {
				t.Fatalf("AsString error = %v", err)
			}
			got = append(got, text)
		}
		if want := []string{"first", "second"}; !reflect.DeepEqual(got, want) {
			t.Errorf("array = %v, want %v", got, want)
		}
	})
}

func TestValueAsBool(t *testing.T) {
	t.Run("accepts true and false", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"true":true,"false":false}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("true", "false")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		if value, err := consumed["true"].AsBool(); err != nil || !value {
			t.Errorf("AsBool(true) = %v, %v; want true, nil", value, err)
		}
		if value, err := consumed["false"].AsBool(); err != nil || value {
			t.Errorf("AsBool(false) = %v, %v; want false, nil", value, err)
		}
	})

	t.Run("rejects other kinds", func(t *testing.T) {
		object, err := ParseObject([]byte(`{"null":null,"string":"true","number":1}`))
		if err != nil {
			t.Fatalf("ParseObject error = %v", err)
		}
		consumed, err := object.Collect("null", "string", "number")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		for _, key := range []string{"null", "string", "number"} {
			if _, err := consumed[key].AsBool(); !errors.Is(err, errValueNotBool) {
				t.Errorf("AsBool(%q) error = %v, want errValueNotBool", key, err)
			}
		}
	})

	t.Run("rejects the zero value", func(t *testing.T) {
		var zero Value
		if _, err := zero.AsBool(); !errors.Is(err, errZeroValue) {
			t.Errorf("AsBool error = %v, want errZeroValue", err)
		}
	})
}

func TestAsArrayElementLimit(t *testing.T) {
	build := func(t *testing.T, elements int) Value {
		t.Helper()
		var document strings.Builder
		document.WriteString(`{"a":[`)
		for i := range elements {
			if i > 0 {
				document.WriteByte(',')
			}
			document.WriteByte('0')
		}
		document.WriteString(`]}`)
		object, err := ParseObject([]byte(document.String()))
		if err != nil {
			t.Fatalf("ParseObject(%d elements) error = %v", elements, err)
		}
		consumed, err := object.Collect("a")
		if err != nil {
			t.Fatalf("Collect error = %v", err)
		}
		return consumed["a"]
	}

	t.Run("accepts one below the limit", func(t *testing.T) {
		values, err := build(t, maxArrayElements-1).AsArray()
		if err != nil {
			t.Fatalf("AsArray error = %v", err)
		}
		if len(values) != maxArrayElements-1 {
			t.Errorf("AsArray returned %d values, want %d", len(values), maxArrayElements-1)
		}
	})

	t.Run("accepts the limit", func(t *testing.T) {
		values, err := build(t, maxArrayElements).AsArray()
		if err != nil {
			t.Fatalf("AsArray error = %v", err)
		}
		if len(values) != maxArrayElements {
			t.Errorf("AsArray returned %d values, want %d", len(values), maxArrayElements)
		}
	})

	t.Run("rejects one above the limit", func(t *testing.T) {
		if _, err := build(t, maxArrayElements+1).AsArray(); !errors.Is(err, errTooManyElements) {
			t.Errorf("AsArray error = %v, want errTooManyElements", err)
		}
	})
}
