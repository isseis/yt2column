//go:build test

// This file is not built in place: the residue check copies it into
// cmd/yt2column of a temporary worktree of the commit it checks and runs
// TestRunIgnoresSlackWebhookURL there.

package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestRunIgnoresSlackWebhookURL checks that the CLI never asks for
// SLACK_WEBHOOK_URL, and that a --out run gives the same exit code, --out
// file, and standard error whether the variable is unset, empty, or a value
// the server rejects.
func TestRunIgnoresSlackWebhookURL(t *testing.T) {
	t.Run("never looked up", func(t *testing.T) {
		e := newRunEnv(t)
		e.env["SLACK_WEBHOOK_URL"] = "https://hooks.slack.com/services/T0/B0/MARKER-WEBHOOK"
		var asked []string
		lookup := func(name string) (string, bool) {
			asked = append(asked, name)
			return lookupFrom(e.env)(name)
		}
		if code := run(e.ctx, e.cliArgs(), lookup, &e.stdout, &e.stderr, e.d); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		// The run read its configuration through lookup, so the absence below
		// is not a run that read nothing.
		if !slices.Contains(asked, "DEEPSEEK_API_KEY") {
			t.Fatalf("lookup was asked %q, which lacks DEEPSEEK_API_KEY", asked)
		}
		if slices.Contains(asked, "SLACK_WEBHOOK_URL") {
			t.Errorf("lookup was asked for SLACK_WEBHOOK_URL: %q", asked)
		}
	})

	// outcome is what a run leaves behind, with the run's temporary
	// directory replaced, so runs in different directories compare equal.
	type outcome struct {
		code    int
		article string
		stderr  string
	}
	runWith := func(t *testing.T, set func(env map[string]string)) outcome {
		t.Helper()
		e := newRunEnv(t)
		useFixedClock(e, costGenTime)
		set(e.env)
		code := e.run()
		data, err := os.ReadFile(e.outPath)
		if err != nil {
			t.Fatalf("read --out: %v\nstderr:\n%s", err, e.stderr.String())
		}
		requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
		return outcome{
			code:    code,
			article: string(data),
			stderr:  strings.ReplaceAll(e.stderr.String(), e.base, "<base>"),
		}
	}
	unset := runWith(t, func(env map[string]string) { delete(env, "SLACK_WEBHOOK_URL") })
	if unset.code != exitOK {
		t.Fatalf("exit code with SLACK_WEBHOOK_URL unset = %d, want %d\nstderr:\n%s", unset.code, exitOK, unset.stderr)
	}
	for name, value := range map[string]string{
		"empty":     "",
		"not https": "http://hooks.slack.com/services/MARKER-WEBHOOK",
	} {
		t.Run(name, func(t *testing.T) {
			got := runWith(t, func(env map[string]string) { env["SLACK_WEBHOOK_URL"] = value })
			if got != unset {
				t.Errorf("run with SLACK_WEBHOOK_URL %s = %+v, want the unset run's %+v", name, got, unset)
			}
		})
	}
}
