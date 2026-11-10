//go:build test

package codexreview

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// headCommit is a commit ID of the one form parseHead accepts.
const headCommit = "0123456789abcdef0123456789abcdef01234567"

// TestParseHead checks the forms parseHead accepts and that every other form
// of either output is rejected without being repaired or quoted.
func TestParseHead(t *testing.T) {
	accepted := []struct {
		name          string
		commitOut     string
		branchOut     string
		detached      bool
		wantBranchRef string
	}{
		{name: "branch", commitOut: headCommit + "\n", branchOut: "refs/heads/main\n", wantBranchRef: "refs/heads/main"},
		{name: "branch_with_slash", commitOut: headCommit + "\n", branchOut: "refs/heads/feature/x\n", wantBranchRef: "refs/heads/feature/x"},
		{name: "detached", commitOut: headCommit + "\n", detached: true},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			branchRef, commit, err := parseHead([]byte(tt.commitOut), []byte(tt.branchOut), tt.detached)
			if err != nil || branchRef != tt.wantBranchRef || commit != headCommit {
				t.Errorf("parseHead = %q, %q, %v, want %q, %q, nil", branchRef, commit, err, tt.wantBranchRef, headCommit)
			}
		})
	}

	const valid = "refs/heads/main\n"
	rejected := []struct {
		name      string
		commitOut string
		branchOut string
		detached  bool
		// marker is a part of the rejected output that must not reach the
		// error.
		marker string
	}{
		{name: "commit_without_newline", commitOut: headCommit, branchOut: valid, marker: headCommit},
		{name: "commit_two_lines", commitOut: headCommit + "\n" + headCommit + "\n", branchOut: valid, marker: headCommit},
		{name: "commit_uppercase", commitOut: strings.ToUpper(headCommit) + "\n", branchOut: valid, marker: strings.ToUpper(headCommit)},
		{name: "commit_39_digits", commitOut: headCommit[:39] + "\n", branchOut: valid, marker: headCommit[:39]},
		{name: "commit_64_digits", commitOut: headCommit + headCommit[:24] + "\n", branchOut: valid, marker: headCommit[:24]},
		{name: "commit_not_hex", commitOut: "g" + headCommit[1:] + "\n", branchOut: valid, marker: "g" + headCommit[1:]},
		{name: "commit_empty", commitOut: "", branchOut: valid},
		{name: "branch_without_newline", commitOut: headCommit + "\n", branchOut: "refs/heads/marker", marker: "marker"},
		{name: "branch_two_lines", commitOut: headCommit + "\n", branchOut: "refs/heads/main\nrefs/heads/marker\n", marker: "marker"},
		{name: "branch_outside_heads", commitOut: headCommit + "\n", branchOut: "refs/tags/marker\n", marker: "marker"},
		{name: "branch_heads_only", commitOut: headCommit + "\n", branchOut: "refs/heads/\n", marker: "refs/heads/"},
		{name: "branch_with_nul", commitOut: headCommit + "\n", branchOut: "refs/heads/ma\x00rker\n", marker: "rker"},
		{name: "branch_empty_not_detached", commitOut: headCommit + "\n", branchOut: ""},
		{name: "detached_with_branch", commitOut: headCommit + "\n", branchOut: "refs/heads/marker\n", detached: true, marker: "marker"},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			branchRef, commit, err := parseHead([]byte(tt.commitOut), []byte(tt.branchOut), tt.detached)
			if !errors.Is(err, errHeadUnreadable) || branchRef != "" || commit != "" {
				t.Fatalf("parseHead = %q, %q, %v, want %v", branchRef, commit, err, errHeadUnreadable)
			}
			// The output is untrusted: the error names the rule only.
			if tt.marker != "" && strings.Contains(err.Error(), tt.marker) {
				t.Errorf("error %q quotes the rejected output", err)
			}
		})
	}
}

// TestReadHead checks that readHead hands parseHead what symbolic-ref
// printed when it reports a detached HEAD, so output there is rejected
// rather than dropped.
func TestReadHead(t *testing.T) {
	gitArgs := func(args ...string) []string { return append(cmdrunner.GitHardening(), args...) }
	tests := []struct {
		name      string
		branchOut string
		wantErr   error
	}{
		{name: "detached", branchOut: ""},
		{name: "detached_with_output", branchOut: "refs/heads/main\n", wantErr: errHeadUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := git{runner: cmdrunnertestutil.New(t,
				cmdrunnertestutil.Step{Name: "git", Args: gitArgs("rev-parse", "--verify", "HEAD"), Out: headCommit + "\n"},
				cmdrunnertestutil.Step{Name: "git", Args: gitArgs("symbolic-ref", "-q", "HEAD"), Err: &cmdrunner.ExitError{Code: 1, Stdout: []byte(tt.branchOut)}},
			)}
			branchRef, commit, err := readHead(context.Background(), g)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("readHead = %q, %q, %v, want %v", branchRef, commit, err, tt.wantErr)
				}
				return
			}
			if err != nil || branchRef != "" || commit != headCommit {
				t.Errorf("readHead = %q, %q, %v, want \"\", %q, nil", branchRef, commit, err, headCommit)
			}
		})
	}
}

// TestDescribeRefs checks that each ref is named with the worktree it is
// checked out in, and alone when no worktree has it or the list cannot be
// read.
func TestDescribeRefs(t *testing.T) {
	listArgs := append(cmdrunner.GitHardening(), "worktree", "list", "--porcelain", "-z")
	// The path does not exist, so resolving its symbolic links leaves it as
	// it is.
	const list = "worktree /nonexistent/wt\x00HEAD " + headCommit + "\x00branch refs/heads/main\x00\x00"
	refs := []string{"refs/heads/main", "refs/heads/other"}
	tests := []struct {
		name string
		step cmdrunnertestutil.Step
		want []string
	}{
		{
			name: "listed",
			step: cmdrunnertestutil.Step{Name: "git", Args: listArgs, Out: list},
			want: []string{"refs/heads/main (worktree /nonexistent/wt)", "refs/heads/other"},
		},
		{
			name: "list_fails",
			step: cmdrunnertestutil.Step{Name: "git", Args: listArgs, Err: errors.New("worktree list failed")},
			want: refs,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := git{runner: cmdrunnertestutil.New(t, tt.step)}
			if got := describeRefs(context.Background(), g, refs); !slices.Equal(got, tt.want) {
				t.Errorf("describeRefs = %q, want %q", got, tt.want)
			}
		})
	}
}
