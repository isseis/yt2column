//go:build test

package chief

import (
	"context"
	"testing"
)

// call returns a recorded call of name with args in dir.
func call(name, dir string, args ...string) recordedCall {
	return recordedCall{name: name, dir: dir, args: args}
}

// TestRecorderAllowsOnlyTheExactWrite checks, for each write the tool makes,
// that the call the test allowed passes and that a call differing in one
// place does not: another ID or path, an extra option, or another working
// directory. The recorder fails a test on a write that does not pass.
func TestRecorderAllowsOnlyTheExactWrite(t *testing.T) {
	const chiefTree = "/work/chief"
	cases := []struct {
		name  string
		allow func(r *recorder)
		ok    recordedCall
		bad   []recordedCall
	}{
		{
			"workspace create",
			func(r *recorder) { r.allowWorkspaceCreate("/h/issue-7", "issue-7", nil) },
			call(herdrProgram, "", "workspace", "create", "--cwd", "/h/issue-7", "--label", "issue-7", "--no-focus"),
			[]recordedCall{
				call(herdrProgram, "", "workspace", "create", "--cwd", "/h/issue-8", "--label", "issue-7", "--no-focus"),
				call(herdrProgram, "", "workspace", "create", "--cwd", "/h/issue-7", "--label", "issue-8", "--no-focus"),
				call(herdrProgram, "", "workspace", "create", "--cwd", "/h/issue-7", "--label", "issue-7"),
			},
		},
		{
			"tab rename",
			func(r *recorder) { r.allowTabRename("w1:t1", "cmd", nil) },
			call(herdrProgram, "", "tab", "rename", "w1:t1", "cmd"),
			[]recordedCall{call(herdrProgram, "", "tab", "rename", "w2:t1", "cmd")},
		},
		{
			"tab create",
			func(r *recorder) { r.allowTabCreate("w1", "/h/issue-7", "claude", nil) },
			call(herdrProgram, "", "tab", "create", "--workspace", "w1", "--cwd", "/h/issue-7", "--label", "claude", "--no-focus"),
			[]recordedCall{call(herdrProgram, "", "tab", "create", "--workspace", "w2", "--cwd", "/h/issue-7", "--label", "claude", "--no-focus")},
		},
		{
			"agent start",
			func(r *recorder) { r.allowAgentStart("issue-7-claude", "claude", "w1:p2", nil) },
			call(herdrProgram, "", herdrAgent, "start", "issue-7-claude", "--kind", "claude", "--pane", "w1:p2", "--timeout", agentStartTimeoutMS),
			[]recordedCall{
				call(herdrProgram, "", herdrAgent, "start", "issue-7-claude", "--kind", "claude", "--pane", "w9:p2", "--timeout", agentStartTimeoutMS),
				call(herdrProgram, "", herdrAgent, "start", "issue-7-claude", "--kind", "opencode", "--pane", "w1:p2", "--timeout", agentStartTimeoutMS),
			},
		},
		{
			"workspace close",
			func(r *recorder) { r.allowWorkspaceClose("w1", nil) },
			call(herdrProgram, "", "workspace", "close", "w1"),
			[]recordedCall{
				call(herdrProgram, "", "workspace", "close", "w2"),
				call(herdrProgram, "", "workspace", "close", "w1", "--group"),
			},
		},
		{
			"worktree add",
			func(r *recorder) { r.allowWorktreeAdd(chiefTree, "/h/issue-7", "abc123") },
			call(gitProgram, chiefTree, "worktree", "add", "--detach", "/h/issue-7", "abc123"),
			[]recordedCall{
				call(gitProgram, chiefTree, "worktree", "add", "--detach", "/h/issue-8", "abc123"),
				call(gitProgram, chiefTree, "worktree", "add", "--detach", "/h/issue-7", "def456"),
				call(gitProgram, "/elsewhere", "worktree", "add", "--detach", "/h/issue-7", "abc123"),
				call(gitProgram, chiefTree, "worktree", "add", "-b", "x", "/h/issue-7", "abc123"),
			},
		},
		{
			"worktree remove",
			func(r *recorder) { r.allowWorktreeRemove(chiefTree, "/h/issue-7") },
			call(gitProgram, chiefTree, "worktree", "remove", "/h/issue-7"),
			[]recordedCall{
				call(gitProgram, chiefTree, "worktree", "remove", "/h/issue-8"),
				call(gitProgram, chiefTree, "worktree", "remove", "--force", "/h/issue-7"),
				call(gitProgram, "/elsewhere", "worktree", "remove", "/h/issue-7"),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRecorder(t)
			if _, ok := r.allowedWrite(c.ok); ok {
				t.Fatal("the call is allowed before the test allowed it")
			}
			c.allow(r)
			if _, ok := r.allowedWrite(c.ok); !ok {
				t.Errorf("the allowed call %s was refused", c.ok.line())
			}
			for _, bad := range c.bad {
				if _, ok := r.allowedWrite(bad); ok {
					t.Errorf("%s was allowed, but only %s was", bad.line(), c.ok.line())
				}
			}
			// No write is also a read: the allowance is the only way through.
			if isReadOnly(c.ok) {
				t.Errorf("%s counts as read-only", c.ok.line())
			}
		})
	}
}

// TestRecorderAnswersAnAllowedWrite runs an allowed write through the
// commands the code under test uses, so the answer reaches it.
func TestRecorderAnswersAnAllowedWrite(t *testing.T) {
	r := newRecorder(t)
	r.allowWorkspaceClose("w1", []byte("closed"))
	out, err := r.commands(nil).herdr(context.Background(), "workspace", "close", "w1")
	if err != nil || string(out) != "closed" {
		t.Fatalf("workspace close = %q, %v", out, err)
	}
}
