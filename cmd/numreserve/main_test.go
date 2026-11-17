//go:build test

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/numreserve"
	"github.com/isseis/yt2column/internal/tasknaming"
)

// fakeReserver returns a fixed result and records the spaces asked for.
type fakeReserver struct {
	reservation numreserve.Reservation
	err         error
	spaces      []numreserve.Space
}

func (f *fakeReserver) Reserve(_ context.Context, space numreserve.Space) (numreserve.Reservation, error) {
	f.spaces = append(f.spaces, space)
	return f.reservation, f.err
}

func runTool(args []string, r reserver) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, r, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunDecides(t *testing.T) {
	for arg, space := range map[string]numreserve.Space{"task": numreserve.SpaceTask, "adr": numreserve.SpaceADR} {
		t.Run(arg, func(t *testing.T) {
			n, err := tasknaming.ParseNumber("0101")
			if err != nil {
				t.Fatal(err)
			}
			r := &fakeReserver{reservation: numreserve.Reservation{Number: n, Tag: "refs/tags/x-number/0101", Commit: "abc"}}
			code, stdout, stderr := runTool([]string{arg}, r)
			if code != exitDecided || stdout != "0101\n" {
				t.Errorf("run = %d, stdout %q; want 0, \"0101\\n\"", code, stdout)
			}
			if stderr != "numreserve: created refs/tags/x-number/0101 at abc\n" {
				t.Errorf("stderr = %q", stderr)
			}
			if len(r.spaces) != 1 || r.spaces[0] != space {
				t.Errorf("Reserve called with %v, want [%v]", r.spaces, space)
			}
		})
	}
}

// TestRunStops checks every way of deciding no number exits 1 with standard
// output empty and one standard-error line, which carries the reason word
// only for a StopError of a declared kind.
func TestRunStops(t *testing.T) {
	type row struct {
		name   string
		err    error
		stderr string
	}
	var tests []row
	for _, kind := range numreserve.StopKinds() {
		reason, _ := kind.Reason()
		tests = append(tests, row{
			reason, &numreserve.StopError{Kind: kind, Step: "step", Detail: "detail"},
			"numreserve: stop " + reason + ": step: detail\n",
		})
	}
	tests = append(tests,
		row{"undeclared kind", &numreserve.StopError{Kind: numreserve.StopKind(99), Step: "step", Detail: "detail"}, "numreserve: stopped: step: detail\n"},
		row{"not a StopError", numreserve.ErrUnderTest, "numreserve: " + numreserve.ErrUnderTest.Error() + "\n"},
		row{"no number", nil, "numreserve: no number was decided\n"},
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runTool([]string{"task"}, &fakeReserver{err: tt.err})
			if code != exitStopped || stdout != "" || stderr != tt.stderr {
				t.Errorf("run = %d, stdout %q, stderr %q; want 1, \"\", %q", code, stdout, stderr, tt.stderr)
			}
		})
	}
}

// TestRunRejectsArguments checks a command line of any other form exits 2
// before Reserve is called.
func TestRunRejectsArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"task", "adr"}, {"task", "--x"}, {"-h"}, {"--help"}, {"Task"}, {"tasks"}, {""}} {
		r := &fakeReserver{err: errors.New("must not be called")}
		code, stdout, stderr := runTool(args, r)
		if code != exitUsage || stdout != "" || !strings.HasPrefix(stderr, "usage: ") || len(r.spaces) != 0 {
			t.Errorf("run(%q) = %d, stdout %q, stderr %q, %d calls; want 2 with usage and no call", args, code, stdout, stderr, len(r.spaces))
		}
	}
}
