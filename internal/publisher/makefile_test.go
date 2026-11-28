//go:build test

package publisher

import (
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/maketestutil"
)

// TestMakeTestParseNoraceRunsLinearTime checks that test-parse-norace narrows
// go test to include TestNeutralizedDisplayMarkdownLinearTime, so the recipe
// cannot stop running the timing check without a test failing. The recipe's
// whole argument list and its environment are pinned by TestMakeTestParseNorace
// in internal/transcript, which cannot name this package's test; this guard
// checks the name it takes from the function value.
func TestMakeTestParseNoraceRunsLinearTime(t *testing.T) {
	_, invocation := maketestutil.RunTarget(t, filepath.Join("..", ".."), "test-parse-norace", nil, nil, nil)
	i := slices.Index(invocation.Args, "-run")
	if i < 0 || i+1 >= len(invocation.Args) {
		t.Fatalf("GOTEST arguments %q have no -run value", invocation.Args)
	}
	names, err := maketestutil.RunNames(invocation.Args[i+1])
	if err != nil {
		t.Fatalf("test-parse-norace -run value: %v", err)
	}
	if want := linearTimeTestName(); !slices.Contains(names, want) {
		t.Errorf("test-parse-norace -run names %q do not include %q", names, want)
	}
}

// linearTimeTestName returns TestNeutralizedDisplayMarkdownLinearTime's name
// from the function value, so renaming the test changes what the guard expects.
func linearTimeTestName() string {
	name := runtime.FuncForPC(reflect.ValueOf(TestNeutralizedDisplayMarkdownLinearTime).Pointer()).Name()
	return name[strings.LastIndex(name, ".")+1:]
}
