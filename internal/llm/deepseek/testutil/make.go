//go:build test

package deepseektestutil

import (
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/maketestutil"
)

// ErrFirstLineMismatch reports that a file's first line is not the expected
// one.
var ErrFirstLineMismatch = maketestutil.ErrFirstLineMismatch

// recordedEnv names the variables the stub GOTEST records: both opt-ins, so
// a target that exports the wrong one is visible, and the model name.
var recordedEnv = []string{DeepSeekOptInEnv, CLIOptInEnv, ModelEnv}

// MakeInvocation is what the stub GOTEST recorded: its arguments, and the
// recorded variables that were set, by name. An unset variable has no entry,
// so unset and empty differ.
type MakeInvocation = maketestutil.MakeInvocation

// RunMakeTarget runs `make -s <target>` in root with GOTEST replaced by a stub,
// and returns make's output and what the stub recorded. model is the ModelEnv
// entry of the child environment; nil leaves the variable undefined. The stub
// records recordedEnv and, in addition, every variable named in extraEnv, so a
// caller can check opt-ins this package does not define.
func RunMakeTarget(t *testing.T, root, target string, model *string, extraEnv ...string) (string, MakeInvocation) {
	t.Helper()
	return maketestutil.RunTarget(t, root, target, recordedEnv, extraEnv, modelEnv(model))
}

// modelEnv is the child environment entry for the model variable. A nil model
// leaves the variable undefined, so the target's default applies; an empty
// string is passed through, so it does not.
func modelEnv(model *string) map[string]string {
	if model == nil {
		return nil
	}
	return map[string]string{ModelEnv: *model}
}

// ChargedTarget describes a make target that runs a charged integration test.
type ChargedTarget struct {
	// Root is the directory holding the Makefile.
	Root string
	// Target is the make target.
	Target string
	// OptInEnv is the opt-in variable the target must export.
	OptInEnv string
	// Package is the package path the target passes to go test.
	Package string
	// MinTimeout is what the go test -timeout value must exceed, so the test
	// binary does not time out before the calls it makes can.
	MinTimeout time.Duration
}

// deepSeekChargeNotice is a substring every DeepSeek and CLI target prints
// before it calls the real API.
const deepSeekChargeNotice = "calls the real DeepSeek API, which incurs charges"

// CheckChargedTarget runs c.Target under the stub GOTEST with the model
// variable undefined, empty, and set, and checks the DeepSeek charge notice,
// the go test arguments, the -timeout bound, the opt-in, and the model name.
// The package path must be the final, standalone argument, so go test treats
// it as the package and not as the value of a flag such as -run.
func CheckChargedTarget(t *testing.T, c ChargedTarget) {
	t.Helper()
	maketestutil.CheckChargedTarget(t, maketestutil.ChargedTarget{
		Root:         c.Root,
		Target:       c.Target,
		Package:      c.Package,
		MinTimeout:   c.MinTimeout,
		OptInEnv:     c.OptInEnv,
		OptInValue:   OptInValue,
		ChargeNotice: deepSeekChargeNotice,
		Vars:         []maketestutil.TargetVar{{Env: ModelEnv, Default: "deepseek-flash", Custom: "deepseek-custom"}},
		Record:       recordedEnv,
	})
}

// FirstLineIs reports an error unless the file at path exists and its first
// line is exactly want. A different first line wraps ErrFirstLineMismatch.
func FirstLineIs(path, want string) error {
	return maketestutil.FirstLineIs(path, want)
}
