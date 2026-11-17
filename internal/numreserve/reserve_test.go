//go:build test

package numreserve

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// remainingHeader is the rate-limit header of the samples, with its value.
var remainingHeader = regexp.MustCompile(`X-Ratelimit-Remaining: [0-9]+`)

// TestReserveCreateOutcomes checks each answer to the creation in both
// spaces: only 201 decides a number, only 422 confirmed by reading back a ref
// of exactly that name moves to the next number, and everything else stops
// with its declared kind. Every failing gh run carries a body and standard
// error the stop must not repeat (stopKind checks).
func TestReserveCreateOutcomes(t *testing.T) {
	tests := []struct {
		name  string
		steps func(t *testing.T, s scenario, first, second string) []cmdrunnertestutil.Step
		want  func(first, second string) outcome
	}{
		{
			"201 decides the first candidate", posts(func(t *testing.T) (string, error) { return created(t), nil }),
			wantNumber(true),
		},
		{
			"HTTP/1.1 201", posts(func(t *testing.T) (string, error) { return withStatus(created(t), "HTTP/1.1 201 Created"), nil }),
			wantNumber(true),
		},
		{
			"HTTP/2 201", posts(func(t *testing.T) (string, error) { return withStatus(created(t), "HTTP/2 201 Created"), nil }),
			wantNumber(true),
		},
		{"422 confirmed moves to the next number", func(t *testing.T, s scenario, first, second string) []cmdrunnertestutil.Step {
			return append(s.collectSteps(t),
				postStep(s.tag(first), "", ghFailed(1, taken(t))),
				getStep(s.tag(first), readBack(t, s.tag(first)), nil),
				postStep(s.tag(second), created(t), nil))
		}, wantNumber(false)},
		{"422 then a ref of another name", afterTaken(func(t *testing.T, tag string) (string, error) {
			return readBack(t, tag+"0"), nil
		}), wantStop(StopMalformed)},
		{"422 then a body that is not an object", afterTaken(func(t *testing.T, tag string) (string, error) {
			head, _, _ := strings.Cut(readBack(t, tag), "\r\n\r\n")
			return head + "\r\n\r\n[" + `{"ref":"` + tag + `"}]`, nil
		}), wantStop(StopMalformed)},
		{
			"422 then 404", afterTaken(func(t *testing.T, _ string) (string, error) { return "", ghFailed(1, notFound(t)) }),
			wantStop(StopNetworkOrGitHub),
		},
		{
			"422 then no status line", afterTaken(func(*testing.T, string) (string, error) { return "", ghFailed(1, leak) }),
			wantStop(StopNetworkOrGitHub),
		},
		{
			"422 then a time-out", afterTaken(func(*testing.T, string) (string, error) { return "", context.DeadlineExceeded }),
			wantStop(StopNetworkOrGitHub),
		},
		{"401", failedPost(401, ""), wantStop(StopPermission)},
		{"403", failedPost(403, ""), wantStop(StopPermission)},
		{"404", failedPost(404, ""), wantStop(StopPermission)},
		{"403 with no remaining rate limit", failedPost(403, "0"), wantStop(StopNetworkOrGitHub)},
		{"403 with Retry-After", posts(func(t *testing.T) (string, error) {
			return "", ghFailed(1, withHeader(withStatus(taken(t), "HTTP/2.0 403 Forbidden"), "retry-after: 60"))
		}), wantStop(StopNetworkOrGitHub)},
		{"403 with unreadable headers", posts(func(*testing.T) (string, error) {
			return "", ghFailed(1, "HTTP/2.0 403 Forbidden\nno header block")
		}), wantStop(StopNetworkOrGitHub)},
		{"429", failedPost(429, ""), wantStop(StopNetworkOrGitHub)},
		{"500", failedPost(500, ""), wantStop(StopNetworkOrGitHub)},
		{"200 instead of 201", failedPost(200, ""), wantStop(StopNetworkOrGitHub)},
		{
			"HTTP/3 status line", posts(func(t *testing.T) (string, error) { return withStatus(created(t), "HTTP/3 201 Created"), nil }),
			wantStop(StopNetworkOrGitHub),
		},
		{"no status line", posts(func(*testing.T) (string, error) { return "", ghFailed(1, leak) }), wantStop(StopNetworkOrGitHub)},
		{
			"status line wins over gh's exit status", posts(func(t *testing.T) (string, error) { return "", ghFailed(4, created(t)) }),
			wantNumber(true),
		},
		{"422 then 200 with an unreadable header block", afterTaken(func(_ *testing.T, tag string) (string, error) {
			return "HTTP/2.0 200 OK\n" + `{"ref":"` + tag + `"}`, nil
		}), wantStop(StopMalformed)},
		{"gh not authenticated", posts(func(*testing.T) (string, error) { return "", ghFailed(4, "") }), wantStop(StopPermission)},
		{"gh times out", posts(func(*testing.T) (string, error) { return "", context.DeadlineExceeded }), wantStop(StopNetworkOrGitHub)},
		{"gh cannot be run", posts(func(*testing.T) (string, error) { return "", os.ErrNotExist }), wantStop(StopNetworkOrGitHub)},
		{"gh output exceeds the cap", posts(func(*testing.T) (string, error) {
			return "", fmt.Errorf("gh: %w", cmdrunner.ErrOutputTooLarge)
		}), wantStop(StopNetworkOrGitHub)},
	}
	spaces := []struct {
		space         Space
		first, second string
	}{
		{SpaceTask, "0101", "0102"},
		{SpaceADR, "0052", "0053"},
	}
	for _, sp := range spaces {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", sp.first, tt.name), func(t *testing.T) {
				s := scenario{space: sp.space}
				got, err := run(t, sp.space, defaultBounds(), tt.steps(t, s, sp.first, sp.second)...)
				want := tt.want(sp.first, sp.second)
				if want.number != "" {
					if err != nil || got.Number.String() != want.number || got.Tag != s.tag(want.number) {
						t.Errorf("Reserve = %+v, %v; want %s", got, err, want.number)
					}
					return
				}
				if kind := stopKind(t, err); kind != want.kind {
					t.Errorf("kind = %v (%v), want %v", kind, err, want.kind)
				}
			})
		}
	}
}

// posts scripts the collection and one creation answered by answer.
func posts(answer func(t *testing.T) (string, error)) func(*testing.T, scenario, string, string) []cmdrunnertestutil.Step {
	return func(t *testing.T, s scenario, first, _ string) []cmdrunnertestutil.Step {
		out, err := answer(t)
		return append(s.collectSteps(t), postStep(s.tag(first), out, err))
	}
}

// failedPost answers the creation with status; remaining, when not empty,
// replaces the sample's X-Ratelimit-Remaining value.
func failedPost(status int, remaining string) func(*testing.T, scenario, string, string) []cmdrunnertestutil.Step {
	return posts(func(t *testing.T) (string, error) {
		response := withStatus(taken(t), fmt.Sprintf("HTTP/2.0 %d Status", status))
		if remaining != "" {
			response = remainingHeader.ReplaceAllString(response, "X-Ratelimit-Remaining: "+remaining)
		}
		return "", ghFailed(1, response)
	})
}

// afterTaken answers the creation with 422 and the reading back with answer.
func afterTaken(answer func(t *testing.T, tag string) (string, error)) func(*testing.T, scenario, string, string) []cmdrunnertestutil.Step {
	return func(t *testing.T, s scenario, first, _ string) []cmdrunnertestutil.Step {
		out, err := answer(t, s.tag(first))
		return append(s.collectSteps(t),
			postStep(s.tag(first), "", ghFailed(1, taken(t))),
			getStep(s.tag(first), out, err))
	}
}

// outcome is a run's expected result: a number, or a stop of kind.
type outcome struct {
	number string
	kind   StopKind
}

func wantNumber(first bool) func(string, string) outcome {
	return func(f, s string) outcome {
		if first {
			return outcome{number: f}
		}
		return outcome{number: s}
	}
}

func wantStop(kind StopKind) func(string, string) outcome {
	return func(string, string) outcome { return outcome{kind: kind} }
}

// takenUntil scripts n creations answered 422 and confirmed, from number from.
func takenUntil(t *testing.T, s scenario, from, n int) []cmdrunnertestutil.Step {
	var steps []cmdrunnertestutil.Step
	for i := range n {
		tag := s.tag(fmt.Sprintf("%04d", from+i))
		steps = append(steps, postStep(tag, "", ghFailed(1, taken(t))), getStep(tag, readBack(t, tag), nil))
	}
	return steps
}

// TestReserveLimits checks each bound at its edge and one past it, in both
// spaces: the number of creations, the four-digit limit, and the jump width,
// whose stop names the input that caused it.
func TestReserveLimits(t *testing.T) {
	type row struct {
		name   string
		s      scenario
		taken  int    // creations answered 422 before the last step
		from   int    // the first candidate, when taken is not 0
		want   string // the number decided, or "" for a stop
		kind   StopKind
		detail string // text the stop's detail must contain
	}
	tests := []row{
		{name: "task tenth creation decides", s: scenario{space: SpaceTask}, taken: 9, from: 101, want: "0110"},
		{name: "task no eleventh creation", s: scenario{space: SpaceTask}, taken: 10, from: 101, kind: StopAttempts},
		{name: "adr tenth creation decides", s: scenario{space: SpaceADR}, taken: 9, from: 52, want: "0061"},
		{name: "adr no eleventh creation", s: scenario{space: SpaceADR}, taken: 10, from: 52, kind: StopAttempts},
		{name: "task candidate 9999", s: scenario{space: SpaceTask, mainTasks: []string{"9998_a/"}}, want: "9999"},
		{name: "task candidate 10000", s: scenario{space: SpaceTask, mainTasks: []string{"9999_a/"}}, kind: StopExhausted},
		{name: "adr candidate 9999", s: scenario{space: SpaceADR, mainADRs: []string{"9998-a.md"}}, want: "9999"},
		{name: "adr candidate 10000", s: scenario{space: SpaceADR, mainADRs: []string{"9999-a.md"}}, kind: StopExhausted},
		{name: "task base plus 20", s: scenario{space: SpaceTask, branches: []remoteBranch{{name: "owner/0119-a-01", sha: otherSHA}}}, want: "0120"},
		{
			name: "task base plus 21", s: scenario{space: SpaceTask, branches: []remoteBranch{{name: "owner/0120-a-01", sha: otherSHA}}},
			kind: StopJump, detail: `origin branch "owner/0120-a-01" (0120)`,
		},
		{
			name: "task local branch past the jump", s: scenario{space: SpaceTask, local: []string{"owner/0999-stray-01"}},
			kind: StopJump, detail: `local branch "owner/0999-stray-01" (0999)`,
		},
		{
			name: "task jump comes before 10000", s: scenario{space: SpaceTask, branches: []remoteBranch{{name: "owner/9999-a-01", sha: otherSHA}}},
			kind: StopJump, detail: `origin branch "owner/9999-a-01" (9999)`,
		},
		{name: "adr base plus 20", s: scenario{space: SpaceADR, branches: []remoteBranch{{name: "b", sha: otherSHA, adrs: []string{"0070-a.md"}}}}, want: "0071"},
		{
			name: "adr base plus 21", s: scenario{space: SpaceADR, branches: []remoteBranch{{name: "b", sha: otherSHA, adrs: []string{"0071-a.md"}}}},
			kind: StopJump, detail: `"docs/dev/adr/0071-a.md" on origin branch "b" (0071)`,
		},
		{
			name: "adr base is origin/main", s: scenario{space: SpaceADR, mainADRs: []string{"0060-a.md"}, workADRs: []string{"0081-a.md"}},
			kind: StopJump, detail: `"docs/dev/adr/0081-a.md" in the work tree (0081)`,
		},
		{name: "adr base includes record tags", s: scenario{space: SpaceADR, tags: []string{"0062"}, workADRs: []string{"0081-a.md"}}, want: "0082"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := append(tt.s.collectSteps(t), takenUntil(t, tt.s, tt.from, tt.taken)...)
			if tt.want != "" {
				steps = append(steps, postStep(tt.s.tag(tt.want), created(t), nil))
			}
			got, err := run(t, tt.s.space, defaultBounds(), steps...)
			if tt.want != "" {
				if err != nil || got.Number.String() != tt.want {
					t.Fatalf("Reserve = %+v, %v; want %s", got, err, tt.want)
				}
				return
			}
			if kind := stopKind(t, err); kind != tt.kind {
				t.Fatalf("kind = %v (%v), want %v", kind, err, tt.kind)
			}
			if !strings.Contains(err.Error(), tt.detail) {
				t.Errorf("stop %q does not contain %q", err, tt.detail)
			}
		})
	}
}

// TestReserveDeadlines checks every command runs under both the per-command
// and the per-run time limit: with either one shorter, every call's context
// ends no later than that limit allows.
func TestReserveDeadlines(t *testing.T) {
	tests := []struct {
		name            string
		command, perRun time.Duration
	}{
		{"per-command limit is shorter", time.Hour, 2 * time.Hour},
		{"per-run limit is shorter", 2 * time.Hour, time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scenario{space: SpaceADR}
			b := defaultBounds()
			b.commandTimeout, b.runTimeout = tt.command, tt.perRun
			fake := cmdrunnertestutil.New(t, append(s.collectSteps(t), postStep(s.tag("0052"), created(t), nil))...)
			recorder := &deadlineRunner{inner: fake}
			start := time.Now()
			if _, err := newTestTool(recorder, b).Reserve(context.Background(), SpaceADR); err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			end := time.Now()
			if recorder.missing != 0 || len(recorder.deadlines) != recorder.calls {
				t.Fatalf("%d of %d calls ran without a deadline", recorder.missing, recorder.calls)
			}
			limit := min(tt.command, tt.perRun)
			for i, d := range recorder.deadlines {
				if d.After(end.Add(limit)) || d.Before(start.Add(limit-time.Minute)) {
					t.Errorf("call %d deadline is %v after the start, want about %v", i+1, d.Sub(start), limit)
				}
				if d.After(start.Add(tt.perRun).Add(time.Second)) {
					t.Errorf("call %d deadline is past the per-run limit", i+1)
				}
			}
		})
	}
}

// TestReserveTimeLimits checks a command that does not finish stops the run
// as network-or-github, under either limit.
func TestReserveTimeLimits(t *testing.T) {
	tests := []struct {
		name            string
		command, perRun time.Duration
	}{
		{"per-command limit", 20 * time.Millisecond, time.Hour},
		{"per-run limit", time.Hour, 20 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scenario{space: SpaceTask}
			b := defaultBounds()
			b.commandTimeout, b.runTimeout = tt.command, tt.perRun
			// The second call, the fetch, never finishes.
			fake := cmdrunnertestutil.New(t, s.collectSteps(t)[:taskFetch]...)
			_, err := newTestTool(&deadlineRunner{inner: fake, holdCall: taskFetch + 1}, b).Reserve(context.Background(), SpaceTask)
			if kind := stopKind(t, err); kind != StopNetworkOrGitHub {
				t.Errorf("kind = %v (%v), want network-or-github", kind, err)
			}
		})
	}
}
