//go:build test

package coveragecompare

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
)

// TestChildEnv runs the tool with an environment that holds variables which
// could make a suite run only part of its tests (NODE_OPTIONS, GOFLAGS,
// GOENV), move its coverage (NODE_V8_COVERAGE, GOCOVERDIR), point git
// elsewhere (GIT_DIR), a secret, and a variable no rule names. Every go, node,
// and npm the tool starts gets exactly the passed variables it was given and
// the variables the tool sets, with the passed values unchanged.
func TestChildEnv(t *testing.T) {
	t.Parallel()
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			repo, base := f.repo(t)
			env := newFakeEnv(t)
			env.set(f.behavior())
			passed := map[string]string{
				"PATH": fakeBinDir, "HOME": env.home,
				"USER": "tester", "TZ": "UTC", "LANG": "C.UTF-8", "LC_ALL": "C", "LC_CTYPE": "C.UTF-8",
				"GOCACHE": "/cache/go", "GOMODCACHE": "/cache/mod",
			}
			for _, name := range slices.Sorted(maps.Keys(passed)) {
				if name != "PATH" && name != "HOME" {
					env.environ = append(env.environ, name+"="+passed[name])
				}
			}
			env.environ = append(env.environ,
				"NODE_OPTIONS=--test-only", "NODE_V8_COVERAGE=/coverage", "GOFLAGS=-run=TestNone",
				"GOENV=/user/go.env", "GOCOVERDIR=/coverage/go", "GIT_DIR=/elsewhere",
				"DEEPSEEK_API_KEY=secret", "UNRELATED=1")

			r := runTool(context.Background(), env, repo, f.target, base)

			if r.Outcome != OutcomePass {
				t.Fatalf("outcome = %v (stop %v, reasons %+v), want a pass", r.Outcome, r.Stop, r.Reasons)
			}
			starts := env.starts()
			if len(starts) == 0 {
				t.Fatal("no fake start was recorded")
			}
			for _, start := range starts {
				got := map[string]string{}
				for _, entry := range start.Env {
					name, value, _ := strings.Cut(entry, "=")
					if _, repeated := got[name]; repeated {
						t.Errorf("%s %q: %s is given twice", start.Name, start.Args, name)
					}
					got[name] = value
				}
				tmp := got["TMPDIR"]
				if !strings.HasPrefix(tmp, env.tmp+"/"+workPrefix) || !strings.HasSuffix(tmp, "/tmp") {
					t.Errorf("%s %q: TMPDIR = %q, want the work directory's tmp under %s", start.Name, start.Args, tmp, env.tmp)
				}
				want := maps.Clone(passed)
				maps.Copy(want, map[string]string{
					"TMPDIR": tmp, "GOTMPDIR": tmp, "GOTOOLCHAIN": "local", "GOENV": "off",
					"GOFLAGS": "", "GOPROXY": "off", "GOWORK": "off",
				})
				if !maps.Equal(got, want) {
					t.Errorf("%s %q: environment names %v, want %v (values %v, want %v)",
						start.Name, start.Args, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)), got, want)
				}
			}
		})
	}
}
