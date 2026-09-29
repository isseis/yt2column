package secret

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// testPlaintext is a value that must never appear in formatted output.
const testPlaintext = "hunter2-do-not-leak"

// publicHolder exposes a Secret as an exported field.
type publicHolder struct {
	S Secret
}

// privateHolder holds a Secret in an unexported field.
type privateHolder struct {
	s Secret
}

func mustNewSecret(t *testing.T) Secret {
	t.Helper()
	s, err := New(testPlaintext)
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", testPlaintext, err)
	}
	return s
}

func TestSecretFmtRedaction(t *testing.T) {
	s := mustNewSecret(t)

	delegated := []struct {
		verb string
		want string
	}{
		{"%s", redacted},
		{"%v", redacted},
		{"%+v", redacted},
		{"%#v", redacted},
		{"%q", `"` + redacted + `"`},
		{"%d", redacted},
		{"%x", redacted},
		{"%c", redacted},
		{"%U", redacted},
		{"%b", redacted},
		{"%e", redacted},
		{"%f", redacted},
		{"%z", redacted},
	}
	for _, tc := range delegated {
		t.Run(tc.verb, func(t *testing.T) {
			got := fmt.Sprintf(tc.verb, s)
			if got != tc.want {
				t.Errorf("fmt.Sprintf(%q, Secret) = %q, want %q", tc.verb, got, tc.want)
			}
			if strings.Contains(got, testPlaintext) {
				t.Errorf("fmt.Sprintf(%q, Secret) leaked the value", tc.verb)
			}
		})
	}

	t.Run("public_field", func(t *testing.T) {
		holder := publicHolder{S: s}
		const wantPlusV = "{S:" + redacted + "}"
		if got := fmt.Sprintf("%+v", holder); got != wantPlusV {
			t.Errorf("fmt.Sprintf(%%+v, publicHolder) = %q, want %q", got, wantPlusV)
		}
		for _, verb := range []string{"%v", "%+v", "%#v"} {
			got := fmt.Sprintf(verb, holder)
			if strings.Contains(got, testPlaintext) {
				t.Errorf("fmt.Sprintf(%q, publicHolder) leaked the value", verb)
			}
			if !strings.Contains(got, redacted) {
				t.Errorf("fmt.Sprintf(%q, publicHolder) = %q, want it to contain %q", verb, got, redacted)
			}
		}
	})
}

func TestSecretStringGoString(t *testing.T) {
	s := mustNewSecret(t)

	if got := s.String(); got != redacted {
		t.Errorf("Secret.String() = %q, want %q", got, redacted)
	}
	if got := s.GoString(); got != redacted {
		t.Errorf("Secret.GoString() = %q, want %q", got, redacted)
	}
}

func TestSecretFmtNoDelegateVerbs(t *testing.T) {
	s := mustNewSecret(t)

	for _, verb := range []string{"%p", "%T", "%w"} {
		t.Run(verb, func(t *testing.T) {
			got := fmt.Sprintf(verb, s)
			if strings.Contains(got, testPlaintext) {
				t.Errorf("fmt.Sprintf(%q, Secret) = %q leaked the value", verb, got)
			}
		})
	}
}

func TestSecretUnexportedFieldNoLeak(t *testing.T) {
	s := mustNewSecret(t)
	holder := privateHolder{s: s}

	for _, verb := range []string{"%v", "%+v", "%#v"} {
		t.Run(verb, func(t *testing.T) {
			got := fmt.Sprintf(verb, holder)
			if strings.Contains(got, testPlaintext) {
				t.Errorf("fmt.Sprintf(%q, privateHolder) = %q leaked the value", verb, got)
			}
		})
	}
}

func TestSecretSlogRedaction(t *testing.T) {
	s := mustNewSecret(t)

	t.Run("direct", func(t *testing.T) {
		var buf bytes.Buffer
		slog.New(slog.NewTextHandler(&buf, nil)).Info("test", "secret", s)
		out := buf.String()
		if strings.Contains(out, testPlaintext) {
			t.Errorf("slog output leaked the value: %q", out)
		}
		if !strings.Contains(out, "secret="+redacted) {
			t.Errorf("slog output = %q, want it to contain %q", out, "secret="+redacted)
		}
	})

	t.Run("public_field", func(t *testing.T) {
		var buf bytes.Buffer
		slog.New(slog.NewTextHandler(&buf, nil)).Info("test", "holder", publicHolder{S: s})
		out := buf.String()
		if strings.Contains(out, testPlaintext) {
			t.Errorf("slog output leaked the value: %q", out)
		}
		if !strings.Contains(out, redacted) {
			t.Errorf("slog output = %q, want it to contain %q", out, redacted)
		}
	})

	t.Run("private_field", func(t *testing.T) {
		var buf bytes.Buffer
		slog.New(slog.NewTextHandler(&buf, nil)).Info("test", "holder", privateHolder{s: s})
		out := buf.String()
		if strings.Contains(out, testPlaintext) {
			t.Errorf("slog output leaked the value: %q", out)
		}
	})
}

func TestSecretJSONRedaction(t *testing.T) {
	s := mustNewSecret(t)

	got, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal(Secret) returned error: %v", err)
	}
	if want := `"` + redacted + `"`; string(got) != want {
		t.Errorf("json.Marshal(Secret) = %s, want %s", got, want)
	}

	got, err = json.Marshal(publicHolder{S: s})
	if err != nil {
		t.Fatalf("json.Marshal(publicHolder) returned error: %v", err)
	}
	if want := `{"S":"` + redacted + `"}`; string(got) != want {
		t.Errorf("json.Marshal(publicHolder) = %s, want %s", got, want)
	}

	//nolint:staticcheck // a holder with only unexported fields is exactly what this test exercises.
	got, err = json.Marshal(privateHolder{s: s})
	if err != nil {
		t.Fatalf("json.Marshal(privateHolder) returned error: %v", err)
	}
	if want := `{}`; string(got) != want {
		t.Errorf("json.Marshal(privateHolder) = %s, want %s", got, want)
	}
}

func TestSecretReveal(t *testing.T) {
	s := mustNewSecret(t)

	got, err := s.Reveal()
	if err != nil {
		t.Fatalf("Reveal() returned error: %v", err)
	}
	if got != testPlaintext {
		t.Errorf("Reveal() = %q, want %q", got, testPlaintext)
	}
}

func TestSecretNewEmpty(t *testing.T) {
	got, err := New("")
	if err == nil {
		t.Fatal("New(\"\") returned nil error")
	}
	if !errors.Is(err, errEmptyValue) {
		t.Errorf("New(\"\") error = %v, want %v", err, errEmptyValue)
	}
	if _, revealErr := got.Reveal(); revealErr == nil {
		t.Error("Secret returned by New(\"\") is usable, want an error")
	}
}

func TestSecretZeroValueReveal(t *testing.T) {
	var s Secret

	got, err := s.Reveal()
	if err == nil {
		t.Fatal("zero value Reveal() returned nil error")
	}
	if !errors.Is(err, errZeroValue) {
		t.Errorf("zero value Reveal() error = %v, want %v", err, errZeroValue)
	}
	if got != "" {
		t.Errorf("zero value Reveal() = %q, want empty string", got)
	}
}
