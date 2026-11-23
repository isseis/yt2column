//go:build test

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// highReviewDecisionPath is the work tree's copy of the high-review decision
// script. The tests run it directly to verify its behavior; that command
// documents run only origin/main's copy is TestHighReviewDecisionCallSite's
// concern.
const (
	highReviewDecisionPath = "../../.claude/commands/_lib/high-review-decision.sh"
	contextDocRepoPath     = ".claude/commands/_context.md"
)

// hrdFixture is a fixture repository with origin/main holding _context.md and
// the target plan(s), and a feature branch forked from origin/main.
type hrdFixture struct {
	*gitFixture
	bash scriptBash
	mb   string
}

func newHRDFixture(t *testing.T, bash scriptBash, contextContent, markersPlan, singlePlan string) *hrdFixture {
	t.Helper()
	f := &hrdFixture{gitFixture: newGitFixture(t), bash: bash}
	f.write(rrSource, "m0\n")
	if contextContent != "" {
		f.write(contextDocRepoPath, contextContent)
	}
	if markersPlan != "" {
		f.write(rrMarkersPlan, markersPlan)
	}
	if singlePlan != "" {
		f.write(rrSinglePlan, singlePlan)
	}
	f.commit("init origin/main")
	f.mb = f.rev("HEAD")
	f.git("update-ref", "refs/remotes/origin/main", f.mb)
	f.git("switch", "-q", "-c", "feature")
	return f
}

func (f *hrdFixture) head() string {
	f.t.Helper()
	return f.rev("HEAD")
}

func (f *hrdFixture) work(subject string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.repo, rrSource))
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(rrSource, string(data)+subject+"\n")
	f.commit(subject)
	return f.head()
}

func (f *hrdFixture) record(subject string, body ...string) string {
	f.t.Helper()
	message := subject
	if len(body) > 0 {
		message += "\n\n" + strings.Join(body, "\n")
	}
	f.git("commit", "-q", "--allow-empty", "-m", message)
	return f.head()
}

func (f *hrdFixture) commitAndRev(message string) string {
	f.t.Helper()
	f.commit(message)
	return f.head()
}

func (f *hrdFixture) run(plan, target, base string, opts scriptRunOptions) scriptRun {
	f.t.Helper()
	return f.runScript(f.bash, highReviewDecisionPath, []string{plan, target, base}, opts)
}

// markersPlanWith replaces both model-requirement values in the markers.md
// sample (both are ": opus") with val, or removes those lines when val is "".
func markersPlanWith(t *testing.T, val string) string {
	t.Helper()
	src := rrPlanSample(t, "markers.md")
	if val != "" {
		if !strings.Contains(src, ": opus") {
			t.Fatal("markers.md sample has no ': opus' value")
		}
		return strings.ReplaceAll(src, ": opus", ": "+val)
	}
	var out []string
	for line := range strings.Lines(src) {
		if strings.Contains(line, ": opus") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "")
}

// singlePlanWith replaces the model-requirement value in the single.md sample
// (": sonnet") with val, or removes that line when val is "".
func singlePlanWith(t *testing.T, val string) string {
	t.Helper()
	src := rrPlanSample(t, "single.md")
	if val != "" {
		if !strings.Contains(src, ": sonnet") {
			t.Fatal("single.md sample has no ': sonnet' value")
		}
		return strings.Replace(src, ": sonnet", ": "+val, 1)
	}
	var out []string
	for line := range strings.Lines(src) {
		if strings.Contains(line, ": sonnet") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "")
}

// markersPlanWithFencedValue wraps the first requirement value line in a
// fenced code block.
func markersPlanWithFencedValue(t *testing.T) string {
	t.Helper()
	src := rrPlanSample(t, "markers.md")
	var out []string
	wrapped := false
	for line := range strings.Lines(src) {
		if !wrapped && strings.Contains(line, ": opus") {
			out = append(out, "```text\n", line, "```\n")
			wrapped = true
			continue
		}
		out = append(out, line)
	}
	if !wrapped {
		t.Fatal("markers.md sample has no ': opus' line")
	}
	return strings.Join(out, "")
}

// markersPlanWithTwoValues duplicates the first requirement value line.
func markersPlanWithTwoValues(t *testing.T) string {
	t.Helper()
	src := rrPlanSample(t, "markers.md")
	var out []string
	duped := false
	for line := range strings.Lines(src) {
		out = append(out, line)
		if !duped && strings.Contains(line, ": opus") {
			out = append(out, line)
			duped = true
		}
	}
	if !duped {
		t.Fatal("markers.md sample has no ': opus' line")
	}
	return strings.Join(out, "")
}

// replaceMappingSection replaces the ### Harness model mapping section in the
// real _context.md up to the next heading with replacement.
func replaceMappingSection(t *testing.T, contextDoc, replacement string) string {
	t.Helper()
	const heading = "### Harness model mapping\n"
	before, rest, ok := strings.Cut(contextDoc, heading)
	if !ok {
		t.Fatal("_context.md has no ### Harness model mapping section")
	}
	_, after, ok := strings.Cut(rest, "\n### ")
	if !ok {
		t.Fatal("_context.md has no heading after ### Harness model mapping")
	}
	return before + replacement + "### " + after
}

// replaceContextRow replaces one exact row in contextDoc and fails t if the
// row was not found, so a fixture edit cannot silently become a no-op.
func replaceContextRow(t *testing.T, contextDoc, oldRow, newRow string) string {
	t.Helper()
	updated := strings.Replace(contextDoc, oldRow, newRow, 1)
	if updated == contextDoc {
		t.Fatalf("_context.md does not contain row %q", oldRow)
	}
	return updated
}

func assertRun(t *testing.T, got scriptRun, wantStdout string) {
	t.Helper()
	if got.code != 0 || got.stdout != wantStdout || got.stderr != "" {
		t.Fatalf("got %v, want exit 0, stdout %q, empty stderr", got, wantStdout)
	}
}

// assertFailureRun checks a fail-closed decision failure. Every failure of
// high-review-decision.sh exits 0 and prints "RUN\n" on stdout, so the stderr
// diagnostic substring is the only observable contract identifying which
// validation rule rejected the input.
func assertFailureRun(t *testing.T, got scriptRun, wantStderrSubstr string) {
	t.Helper()
	if got.code != 0 || got.stdout != "RUN\n" {
		t.Fatalf("got %v, want exit 0 and stdout %q", got, "RUN\n")
	}
	if !strings.Contains(got.stderr, wantStderrSubstr) {
		t.Fatalf("stderr %q does not contain %q", got.stderr, wantStderrSubstr)
	}
}

func TestHighReviewDecision(t *testing.T) {
	t.Parallel()
	bash := requireBash4(t)
	realContext := doctestutil.ReadDoc(t, contextPath)

	const (
		claudeHighRow = "| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |"
		agHighRow     = "| `Antigravity` | `high` | `swarm` | `gemini-4-neon-high` | `gemini-4-neon-high` |"
	)

	t.Run("values", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			val    string
			single bool
			want   string
		}{
			{"high", "high", false, "RUN\n"},
			{"standard+high-review", "standard+high-review", false, "RUN\n"},
			{"low", "low", false, "NONE value-low\n"},
			{"absent", "", false, "NONE value-absent\n"},
			{"unreadable", "unknown-tier", false, "RUN\n"},
			{"old opus", "opus", false, "RUN\n"},
			{"old sonnet+opus-review", "sonnet+opus-review", false, "RUN\n"},
			{"old sonnet", "sonnet", false, "NONE value-low\n"},
			{"old frontier-required", "frontier-required", false, "RUN\n"},
			{"old frontier-recommended", "frontier-recommended", false, "RUN\n"},
			{"old standard", "standard", false, "NONE value-low\n"},
			{"single low", "low", true, "NONE value-low\n"},
			{"single absent", "", true, "NONE value-absent\n"},
			{"single high", "high", true, "RUN\n"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				planPath, target := rrMarkersPlan, "1"
				markers, single := markersPlanWith(t, tc.val), ""
				if tc.single {
					planPath, target = rrSinglePlan, "single"
					markers, single = "", singlePlanWith(t, tc.val)
				}
				f := newHRDFixture(t, bash, realContext, markers, single)
				f.work("change")
				assertRun(t, f.run(planPath, target, f.mb, scriptRunOptions{}), tc.want)
			})
		}
	})

	t.Run("all-high agent coverage", func(t *testing.T) {
		t.Parallel()
		t.Run("covered with excludable commits", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record 1", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			c2 := f.work("step 2")
			f.record("record 2", "Implementation model: "+c1[:12]+".."+c2[:12]+" claude-opus-4-6, claude-opus-4-7")
			f.write(rrMarkersPlan, markersPlanWith(t, "high")+"\n")
			f.commit("plan only")
			f.write("README.en.md", "updated\n")
			f.commit("readme only")
			f.record("empty only")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "NONE all-high\n")
		})

		t.Run("standard+high-review covered by high agent", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "standard+high-review"), "")
			c1 := f.work("step 1")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "NONE all-high\n")
		})

		t.Run("commit covered by both high and non-high records is tainted", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("sonnet record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-sonnet-4-6")
			f.record("opus record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("uncovered commit runs", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			f.work("step 2 uncovered")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("merge commit runs", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			f.git("switch", "-q", "-c", "side")
			f.write("side.txt", "side\n")
			f.commit("side work")
			f.git("switch", "-q", "feature")
			f.work("feature work")
			f.git("merge", "-q", "--no-ff", "--no-edit", "side")
			m := f.head()
			f.record("record", "Implementation model: "+f.mb[:12]+".."+m[:12]+" claude-opus-4-7")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})
	})

	t.Run("done records", func(t *testing.T) {
		t.Parallel()
		t.Run("new High review done with trailing excludable commits", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			doneLine := "High review: done " + f.mb[:12] + ".." + c1[:12] + " (critical 0, major 0, minor 2)"
			rec := f.record("record high review", doneLine)
			f.write("README.en.md", "doc\n")
			readme := f.commitAndRev("update readme")
			f.write(rrMarkersPlan, markersPlanWith(t, "high")+"\n")
			planCommit := f.commitAndRev("update plan")
			want := "SKIP " + doneLine + "\n" +
				rec[:12] + " record high review\n" +
				readme[:12] + " update readme\n" +
				planCommit[:12] + " update plan\n"
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), want)
		})

		t.Run("legacy Opus review done skips", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			doneLine := "Opus review: done " + f.mb[:12] + ".." + c1[:12] + " (critical 0, major 1, minor 0)"
			rec := f.record("record opus review", doneLine)
			want := "SKIP " + doneLine + "\n" + rec[:12] + " record opus review\n"
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), want)
		})

		t.Run("non-excludable commit after END runs", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record high review", "High review: done "+f.mb[:12]+".."+c1[:12]+" (critical 0, major 0, minor 0)")
			f.work("step 2")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("indented and malformed done lines ignored", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("bad done lines",
				"  High review: done "+f.mb[:12]+".."+c1[:12]+" (critical 0, major 0, minor 0)",
				"High review: done "+f.mb[:12]+".."+c1[:12]+" (critical 0, major 0)",
			)
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("done record starting after BASE ignored", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			c2 := f.work("step 2")
			f.record("narrow done", "High review: done "+c1[:12]+".."+c2[:12]+" (critical 0, major 0, minor 0)")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})
	})

	t.Run("model record and pattern matching", func(t *testing.T) {
		t.Parallel()
		agAgentContext := replaceContextRow(t, realContext, agHighRow,
			"| `Antigravity` | `high` | `agent` | `gemini-4-neon-high` | `gemini-4-neon-high` |")
		dotContext := replaceContextRow(t, realContext, claudeHighRow,
			"| `Claude Code` | `high` | `agent` | `opus` | `v1.0-*` |")
		exactDotContext := replaceContextRow(t, realContext, claudeHighRow,
			"| `Claude Code` | `high` | `agent` | `opus` | `v1.0` |")
		maxLiteral := strings.Repeat("a", 64)
		maxLiteralContext := replaceContextRow(t, realContext, claudeHighRow,
			"| `Claude Code` | `high` | `agent` | `opus` | `"+maxLiteral+"` |")
		maxPrefix := strings.Repeat("b", 63)
		maxPrefixContext := replaceContextRow(t, realContext, claudeHighRow,
			"| `Claude Code` | `high` | `agent` | `opus` | `"+maxPrefix+"*` |")
		fencedHeadingContext := realContext + "\n```markdown\n### Harness model mapping\n```\n"

		cases := []struct {
			name   string
			ctx    string
			models string
			want   string
		}{
			{"mixed high and non-high in one record runs", realContext, "claude-opus-4-7, claude-sonnet-4-6", "RUN\n"},
			{"swarm launch gemini-4-neon-high runs", realContext, "gemini-4-neon-high", "RUN\n"},
			{"mixed agent high and swarm high runs", realContext, "claude-opus-4-7, gemini-4-neon-high", "RUN\n"},
			{"literal pattern with Launch agent satisfies all-high", agAgentContext, "gemini-4-neon-high", "NONE all-high\n"},
			{"prefix dot literal matches", dotContext, "v1.0-a", "NONE all-high\n"},
			{"prefix dot is not regex wildcard", dotContext, "v1X0-a", "RUN\n"},
			{"exact dot literal matches", exactDotContext, "v1.0", "NONE all-high\n"},
			{"exact dot is not regex wildcard", exactDotContext, "v1X0", "RUN\n"},
			{"64-char literal pattern accepted", maxLiteralContext, maxLiteral, "NONE all-high\n"},
			{"63-char prefix plus star accepted", maxPrefixContext, maxPrefix + "x", "NONE all-high\n"},
			{"fenced Harness model mapping heading ignored", fencedHeadingContext, "claude-opus-4-7", "NONE all-high\n"},
			{"48-char suffix accepted", realContext, "claude-opus-" + strings.Repeat("a", 48), "NONE all-high\n"},
			{"49-char suffix rejected", realContext, "claude-opus-" + strings.Repeat("a", 49), "RUN\n"},
			{"empty suffix rejected", realContext, "claude-opus-", "RUN\n"},
			{"uppercase suffix rejected", realContext, "claude-opus-V4", "RUN\n"},
			{"underscore suffix rejected", realContext, "claude-opus-4_7", "RUN\n"},
			{"space in model id rejected", realContext, "claude-opus-4 7", "RUN\n"},
			{"unregistered model rejected", realContext, "gemini-4-argon-high", "RUN\n"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := newHRDFixture(t, bash, tc.ctx, markersPlanWith(t, "high"), "")
				c1 := f.work("step 1")
				f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" "+tc.models)
				assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), tc.want)
			})
		}
	})

	t.Run("fail-closed context validation before early return", func(t *testing.T) {
		t.Parallel()
		const validTable = "### Harness model mapping\n\n" +
			"| Harness | Role | Launch | Model | Verification pattern |\n" +
			"|---|---|---|---|---|\n" +
			"| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n" +
			"| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n" +
			"| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |\n\n"

		cases := []struct {
			name       string
			ctx        string
			wantStderr string
			alsoAbsent bool
		}{
			{
				name:       "missing _context.md",
				ctx:        "",
				wantStderr: "cannot read origin/main:.claude/commands/_context.md",
				alsoAbsent: true,
			},
			{
				name:       "missing mapping section",
				ctx:        replaceMappingSection(t, realContext, ""),
				wantStderr: "missing Harness model mapping section",
			},
			{
				name:       "duplicate mapping section",
				ctx:        replaceMappingSection(t, realContext, validTable+validTable),
				wantStderr: "duplicate Harness model mapping section",
			},
			{
				name:       "empty table with zero data rows",
				ctx:        replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n\n"),
				wantStderr: "missing or empty Harness model mapping table",
			},
			{
				name:       "unclosed fence in _context.md",
				ctx:        realContext + "\n```text\nunclosed\n",
				wantStderr: "unclosed code fence in _context.md",
			},
			{
				name:       "wrong header",
				ctx:        replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Pattern |\n|---|---|---|---|---|\n| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n\n"),
				wantStderr: "invalid Harness model mapping header",
			},
			{
				name:       "wrong separator",
				ctx:        replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|\n| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n\n"),
				wantStderr: "invalid Harness model mapping separator",
			},
			{
				name:       "wrong column count",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |", "| `OpenCode` | `low` | `none` | `DeepSeek` |", 1)),
				wantStderr: "must have 5 columns",
			},
			{
				name:       "indented table row",
				ctx:        replaceMappingSection(t, realContext, "### Harness model mapping\n\n | Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n\n"),
				wantStderr: "indented table row",
			},
			{
				name:       "unquoted cell",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`opus`", "opus", 1)),
				wantStderr: "single code span",
			},
			{
				name:       "multiple tables",
				ctx:        replaceMappingSection(t, realContext, validTable+"prose\n\n| Harness | Role | Launch | Model | Verification pattern |\n\n"),
				wantStderr: "multiple tables",
			},
			{
				name:       "unknown Harness",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`OpenCode`", "`OtherHarness`", 1)),
				wantStderr: "unknown Harness",
			},
			{
				name:       "unknown Role",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`low`", "`ultra`", 1)),
				wantStderr: "unknown Role",
			},
			{
				name:       "unknown Launch",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`none`", "`cli`", 1)),
				wantStderr: "unknown Launch",
			},
			{
				name:       "whitespace-only Model",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`DeepSeek`", "`   `", 1)),
				wantStderr: "invalid Model",
			},
			{
				name:       "duplicate Harness Role",
				ctx:        replaceMappingSection(t, realContext, strings.TrimSuffix(validTable, "\n")+"| `Claude Code` | `high` | `agent` | `opus2` | `claude-opus-2-*` |\n\n"),
				wantStderr: "duplicate (Harness, Role)",
			},
			{
				name:       "none launch with non-dash pattern",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "| `-` |", "| `deepseek` |", 1)),
				wantStderr: "Launch none row must have Verification pattern -",
			},
			{
				name:       "agent launch with dash pattern",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`-`", 1)),
				wantStderr: "cannot have Verification pattern -",
			},
			{
				name:       "whitespace in pattern",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`claude opus`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "regex dot-star metacharacter",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`.*`", 1)),
				wantStderr: "invalid prefix Verification pattern",
				alsoAbsent: true,
			},
			{
				name:       "regex anchors metacharacter",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`^claude-opus$`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "regex alternation pipe forbidden",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`claude-(a|b)`", 1)),
				wantStderr: "forbidden character",
			},
			{
				name:       "regex grouping metacharacter",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`claude-(ab)`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "regex char class in cross-check row",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-fable-*`", "`claude-[a-z]+`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "mid-wildcard",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`claude*opus`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "65-char literal pattern",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`"+strings.Repeat("a", 65)+"`", 1)),
				wantStderr: "invalid literal Verification pattern",
			},
			{
				name:       "64-char prefix plus star",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "`claude-opus-*`", "`"+strings.Repeat("a", 64)+"*`", 1)),
				wantStderr: "invalid prefix Verification pattern",
			},
			{
				name:       "no high agent row",
				ctx:        replaceMappingSection(t, realContext, strings.Replace(validTable, "| `Claude Code` | `high` | `agent` |", "| `Claude Code` | `high` | `swarm` |", 1)),
				wantStderr: "no high-role agent verification pattern",
			},
		}

		for _, tc := range cases {
			t.Run("low-plan/"+tc.name, func(t *testing.T) {
				t.Parallel()
				f := newHRDFixture(t, bash, tc.ctx, markersPlanWith(t, "low"), "")
				assertFailureRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), tc.wantStderr)
			})
			if tc.alsoAbsent {
				t.Run("absent-plan/"+tc.name, func(t *testing.T) {
					t.Parallel()
					f := newHRDFixture(t, bash, tc.ctx, markersPlanWith(t, ""), "")
					assertFailureRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), tc.wantStderr)
				})
			}
		}
	})

	t.Run("environment isolation and failures", func(t *testing.T) {
		t.Parallel()
		t.Run("ignores GIT_DIR and GIT_WORK_TREE", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			got := f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{
				extraEnv: []string{"GIT_DIR=/nonexistent", "GIT_WORK_TREE=/nonexistent"},
			})
			assertRun(t, got, "NONE all-high\n")
		})

		t.Run("ignores preset clean env marker and exported bash functions", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			got := f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{
				extraEnv: []string{
					"HIGH_REVIEW_DECISION_CLEAN_ENV=1",
					"BASH_FUNC_command%%=() { return 0; }",
					"BASH_FUNC_git%%=() { echo poisoned; exit 99; }",
					"BASH_FUNC_awk%%=() { echo poisoned; exit 99; }",
					"BASH_FUNC_printf%%=() { echo poisoned; exit 99; }",
				},
			})
			assertRun(t, got, "NONE all-high\n")
		})

		t.Run("ignores BASH_ENV overriding command and git", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			c1 := f.work("step 1")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+c1[:12]+" claude-opus-4-7")
			bashEnvFile := filepath.Join(t.TempDir(), "poison.sh")
			content := "command() { return 0; }\ngit() { echo poisoned; exit 99; }\nexport GIT_DIR=/nonexistent\n"
			if err := os.WriteFile(bashEnvFile, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			got := f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{
				extraEnv: []string{"BASH_ENV=" + bashEnvFile},
			})
			assertRun(t, got, "NONE all-high\n")
		})

		t.Run("ignores refs/replace", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			f.work("step 1")
			tmpPlan := filepath.Join(t.TempDir(), "low.md")
			if err := os.WriteFile(tmpPlan, []byte(markersPlanWith(t, "low")), 0o600); err != nil {
				t.Fatal(err)
			}
			lowBlobID := strings.TrimSpace(f.git("hash-object", "-w", tmpPlan))
			planBlob := strings.TrimSpace(f.git("rev-parse", "HEAD:"+rrMarkersPlan))
			f.git("update-ref", "refs/replace/"+planBlob, lowBlobID)
			if got := f.git("cat-file", "blob", "HEAD:"+rrMarkersPlan); !strings.Contains(got, ": low") {
				t.Fatalf("plain git did not apply refs/replace, so the test checks nothing: %q", got)
			}
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("HEAD downgrade from origin/main high to low still runs", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "high"), "")
			f.work("step 1")
			f.write(rrMarkersPlan, markersPlanWith(t, "low"))
			f.commit("downgrade plan to low")
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("uncommitted work tree downgrade ignored", func(t *testing.T) {
			t.Parallel()
			// origin/main holds low while HEAD holds high, so reading the
			// uncommitted low file from the work tree instead of HEAD would
			// return NONE value-low and fail this test.
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "low"), "")
			f.write(rrMarkersPlan, markersPlanWith(t, "high"))
			f.commit("upgrade plan to high on branch")
			f.work("step 1")
			f.write(rrMarkersPlan, markersPlanWith(t, "low"))
			assertRun(t, f.run(rrMarkersPlan, "1", f.mb, scriptRunOptions{}), "RUN\n")
		})

		t.Run("bad arguments and git state", func(t *testing.T) {
			t.Parallel()
			f := newHRDFixture(t, bash, realContext, markersPlanWith(t, "low"), "")
			c1 := f.work("step 1")

			assertFailureRun(t, f.runScript(bash, highReviewDecisionPath, []string{rrMarkersPlan, "1"}, scriptRunOptions{}), "expected 3 arguments")
			assertFailureRun(t, f.run("README.md", "1", f.mb, scriptRunOptions{}), "PLAN is not")
			assertFailureRun(t, f.run(rrMarkersPlan, "0", f.mb, scriptRunOptions{}), "TARGET is neither")
			assertFailureRun(t, f.run(rrMarkersPlan, "1", c1, scriptRunOptions{}), "not the merge-base")

			f2 := newHRDFixture(t, bash, realContext, markersPlanWith(t, "low"), "")
			f2.git("update-ref", "-d", "refs/remotes/origin/main")
			assertFailureRun(t, f2.run(rrMarkersPlan, "1", f2.mb, scriptRunOptions{}), "origin/main names no commit")
		})

		t.Run("plan structure failures", func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name       string
				plan       string
				target     string
				wantStderr string
			}{
				{"missing target marker", markersPlanWith(t, "low"), "99", "no PR-99 marker"},
				{"two value lines", markersPlanWithTwoValues(t), "1", "more than one place"},
				{"fenced value line", markersPlanWithFencedValue(t), "1", "more than one place"},
				{"unclosed fence in plan", markersPlanWith(t, "low") + "\n```text\nunclosed\n", "1", "more than one place"},
				{"single target on markers plan", markersPlanWith(t, "low"), "single", "more than one place"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := newHRDFixture(t, bash, realContext, tc.plan, "")
					assertFailureRun(t, f.run(rrMarkersPlan, tc.target, f.mb, scriptRunOptions{}), tc.wantStderr)
				})
			}
		})
	})
}

const (
	highReviewDecisionShow = "git --no-replace-objects show <origin-main-commit>:.claude/commands/_lib/high-review-decision.sh > <decision-script>"
	highReviewDecisionRun  = "bash <decision-script> <PLAN> <TARGET> <BASE>"
)

var highReviewDecisionNames = []string{
	".claude/commands/_lib/high-review-decision.sh",
	"_lib/high-review-decision.sh",
	"<decision-script>",
}

var highReviewDecisionCallers = []string{
	".claude/commands/_lib/opus-review.md",
}

// TestHighReviewDecisionCallSite checks that every command document runs
// high-review-decision.sh only from origin/main via --no-replace-objects, and
// only from the allowed caller documents.
func TestHighReviewDecisionCallSite(t *testing.T) {
	t.Parallel()
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		blockLines, spans := codeTexts(doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, rel)))
		for i, code := range slices.Concat(blockLines, spans) {
			if !strings.Contains(code, "high-review-decision") &&
				!strings.Contains(code, "<decision-script>") {
				continue
			}
			inBlock := i < len(blockLines)
			switch {
			case inBlock && slices.Contains(highReviewDecisionCallers, rel) &&
				(code == highReviewDecisionShow || code == highReviewDecisionRun):
			case !inBlock && slices.Contains(highReviewDecisionNames, code):
			default:
				t.Errorf("%s has %q, which is neither an approved name nor an approved origin/main call of high-review-decision.sh", rel, code)
			}
		}
	}

	section := headingSection("### Running the decision")(t, doctestutil.ReadDoc(t, opusReviewPath))
	blockLines, _ := codeTexts(section)
	if i := slices.Index(blockLines, highReviewDecisionShow); i < 1 || i+1 >= len(blockLines) ||
		blockLines[i-1] != reviewRecordsRef || blockLines[i+1] != highReviewDecisionRun {
		t.Errorf("opus-review.md's Running the decision has no code block lines %q, %q, %q in this order",
			reviewRecordsRef, highReviewDecisionShow, highReviewDecisionRun)
	}
}
