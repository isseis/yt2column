//go:build test

package cmdrunnertestutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// recorder turns a Fatalf into a panic the test can recover, so the fake's own
// failure reports can be observed.
type recorder struct{ failed string }

func (r *recorder) Helper() {}
func (r *recorder) Fatalf(format string, args ...any) {
	r.failed = fmt.Sprintf(format, args...)
	panic(r)
}

// failure runs f and returns the message of the Fatalf it triggered, or "" when
// none did.
func failure(f func(*recorder)) (msg string) {
	r := &recorder{}
	defer func() {
		if p := recover(); p != nil {
			if p != any(r) {
				panic(p)
			}
			msg = r.failed
		}
	}()
	f(r)
	return ""
}

func TestFakeReplaysInOrder(t *testing.T) {
	f := New(t, Step{Name: "git", Args: []string{"status"}, Out: "clean"}, Step{Name: "gh", Args: []string{"pr"}, Out: "x"})
	out, err := f.Run(t.Context(), "git", "status")
	if err != nil || string(out) != "clean" {
		t.Fatalf("Run = %q, %v", out, err)
	}
	if _, err := f.RunInput(t.Context(), []byte("in"), "gh", "pr"); err != nil {
		t.Fatal(err)
	}
	f.Done()
	if f.Stdin[0] != nil || string(f.Stdin[1]) != "in" {
		t.Errorf("Stdin = %q, want [nil in]", f.Stdin)
	}
}

func TestFakeFailsOnMisuse(t *testing.T) {
	tests := []struct {
		name string
		do   func(r *recorder)
		want string
	}{
		{"wrong order", func(r *recorder) {
			New(r, Step{Name: "a"}, Step{Name: "b"}).Run(t.Context(), "b")
		}, "want a"},
		{"wrong args", func(r *recorder) {
			New(r, Step{Name: "a", Args: []string{"x"}}).Run(t.Context(), "a", "y")
		}, "want a"},
		{"extra call", func(r *recorder) {
			f := New(r, Step{Name: "a"})
			f.Run(t.Context(), "a")
			f.Run(t.Context(), "a")
		}, "unexpected call"},
		{"missing call", func(r *recorder) {
			New(r, Step{Name: "a"}).Done()
		}, "ran 0 of 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The wording distinguishes which check fired, so it is asserted.
			if msg := failure(tt.do); !strings.Contains(msg, tt.want) {
				t.Errorf("failure = %q, want it to contain %q", msg, tt.want)
			}
		})
	}
}

func TestFakeEffectAndError(t *testing.T) {
	ran := false
	boom := errors.New("boom")
	f := New(t,
		Step{Name: "a", Effect: func() error { ran = true; return nil }, Out: "ok"},
		Step{Name: "b", Err: boom},
		Step{Name: "c", Effect: func() error { return boom }},
	)
	if out, err := f.Run(t.Context(), "a"); err != nil || string(out) != "ok" || !ran {
		t.Errorf("effect step = %q, %v, ran=%v", out, err, ran)
	}
	if out, err := f.Run(t.Context(), "b"); !errors.Is(err, boom) || out != nil {
		t.Errorf("error step = %q, %v; want nil output and boom", out, err)
	}
	if _, err := f.Run(t.Context(), "c"); !errors.Is(err, boom) {
		t.Errorf("effect error = %v, want boom", err)
	}
}
