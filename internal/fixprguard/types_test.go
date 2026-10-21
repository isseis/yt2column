//go:build test

package fixprguard

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const testFP = Fingerprint("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

func TestParseLogin(t *testing.T) {
	for _, valid := range []string{"coderabbitai", "a", "isseis", "a-b", "A1", "0x", strings.Repeat("a", 39)} {
		t.Run("valid/"+valid, func(t *testing.T) {
			login, err := ParseLogin(valid)
			if err != nil {
				t.Fatalf("ParseLogin(%q) = %v", valid, err)
			}
			if login.String() != valid {
				t.Errorf("String() = %q, want %q", login.String(), valid)
			}
		})
	}
	for _, invalid := range []string{
		"", "-a", "a_b", "a/b", "..", "a b", "a.b", strings.Repeat("a", 40),
		"coderabbitai[bot]", "\u00e9", "a\n", "../../x",
	} {
		t.Run("invalid/"+invalid, func(t *testing.T) {
			if _, err := ParseLogin(invalid); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("ParseLogin(%q) = %v, want ErrInvalidInput", invalid, err)
			}
		})
	}
}

func TestParsePRNumber(t *testing.T) {
	for _, valid := range []string{"1", "42", "999999999"} {
		t.Run("valid/"+valid, func(t *testing.T) {
			pr, err := ParsePRNumber(valid)
			if err != nil {
				t.Fatalf("ParsePRNumber(%q) = %v", valid, err)
			}
			if pr.String() != valid {
				t.Errorf("String() = %q, want %q", pr.String(), valid)
			}
		})
	}
	for _, invalid := range []string{
		"", "0", "042", "+1", "-1", "1.0", " 1", "1 ", "1234567890", "main",
		"https://github.com/isseis/yt2column/pull/42", "#42", "\u0664\u0662",
	} {
		t.Run("invalid/"+invalid, func(t *testing.T) {
			if _, err := ParsePRNumber(invalid); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("ParsePRNumber(%q) = %v, want ErrInvalidInput", invalid, err)
			}
		})
	}
}

func TestIDGrammar(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"PRRT_kwDOP1a2b3c4d5e6f7", true},
		{"PRRT_a-b_c", true},
		{"PRRT_" + strings.Repeat("a", 95), true},
		{"PRRT_" + strings.Repeat("a", 96), false},
		{"PRRT_", false},
		{"prrt_abc", false},
		{"PRR_abc", false},
		{"PRRT_../x", false},
		{"PRRT_a/b", false},
		{"PRRT_a.b", false},
		{"https://github.com/isseis/yt2column/pull/42#discussion_r1", false},
		{"PRRT_a b", false},
		{"PRRT_a=b", false},
	} {
		if got := validThreadID(tc.id); got != tc.want {
			t.Errorf("validThreadID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
	for _, valid := range []string{"1", "2553061234", "999999999999999999"} {
		if _, err := ParseCommentID(valid); err != nil {
			t.Errorf("ParseCommentID(%q) = %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "0", "01", "-1", "1e3", "../1", "1/2", "9999999999999999999", "https://github.com/x/y/pull/1#discussion_r1"} {
		if _, err := ParseCommentID(invalid); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ParseCommentID(%q) = %v, want ErrInvalidInput", invalid, err)
		}
	}
}

func TestParseAccept(t *testing.T) {
	got, err := ParseAccept("PRRT_abc=" + string(testFP))
	if err != nil {
		t.Fatalf("ParseAccept = %v", err)
	}
	if want := (Accept{ThreadID: "PRRT_abc", Fingerprint: testFP}); got != want {
		t.Errorf("ParseAccept = %+v, want %+v", got, want)
	}
	for _, invalid := range []string{
		"",
		"PRRT_abc",
		"PRRT_abc=",
		"=" + string(testFP),
		"PRRT_a/b=" + string(testFP),
		"PRRT_abc=" + strings.ToUpper(string(testFP)),
		"PRRT_abc=" + string(testFP[:63]),
		"PRRT_abc=" + string(testFP) + "0",
		"PRRT_abc=" + string(testFP) + "=x",
		"PRRT_abc =" + string(testFP),
	} {
		if _, err := ParseAccept(invalid); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ParseAccept(%q) = %v, want ErrInvalidInput", invalid, err)
		}
	}
	t.Run("named_twice", func(t *testing.T) {
		a := Accept{ThreadID: "PRRT_abc", Fingerprint: testFP}
		if _, err := checkAccepts([]Accept{a, a}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("checkAccepts(duplicate) = %v, want ErrInvalidInput", err)
		}
	})
	t.Run("unparsed_pair", func(t *testing.T) {
		if _, err := checkAccepts([]Accept{{ThreadID: "../x", Fingerprint: testFP}}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("checkAccepts(bad ID) = %v, want ErrInvalidInput", err)
		}
	})
}

func TestResolveRepository(t *testing.T) {
	step := func(out string) cmdrunnertestutil.Step {
		s := repoStep()
		s.Out = out
		return s
	}
	t.Run("valid", func(t *testing.T) {
		g, _ := newTestGitHub(t, step(`{"nameWithOwner":"isseis/yt2column.go","visibility":"PUBLIC"}`))
		repo, err := g.resolveRepository(t.Context())
		if err != nil {
			t.Fatalf("resolveRepository = %v", err)
		}
		if repo != (repository{owner: "isseis", name: "yt2column.go"}) {
			t.Errorf("repository = %+v", repo)
		}
	})
	for name, out := range map[string]string{
		"no_slash":         `{"nameWithOwner":"isseis"}`,
		"extra_slash":      `{"nameWithOwner":"isseis/yt2column/x"}`,
		"dot_dot_name":     `{"nameWithOwner":"isseis/.."}`,
		"dot_dot_owner":    `{"nameWithOwner":"../yt2column"}`,
		"empty":            `{"nameWithOwner":""}`,
		"null":             `{"nameWithOwner":null}`,
		"missing":          `{}`,
		"duplicate_member": `{"nameWithOwner":"isseis/yt2column","nameWithOwner":"evil/repo"}`,
		"not_json":         `isseis/yt2column`,
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, step(out))
			if _, err := g.resolveRepository(t.Context()); !errors.Is(err, ErrResponseRejected) {
				t.Errorf("resolveRepository(%s) = %v, want ErrResponseRejected", out, err)
			}
		})
	}
}

func TestFindingJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		finding Finding
		want    string
	}{
		{"account", Finding{Reason: ReasonNoWritePermission, Account: mustLogin(t, "alice")}, `{"reason":"no-write-permission","account":"alice"}`},
		{"cause", Finding{Reason: ReasonPermissionUnavailable, Cause: CauseRateLimited}, `{"reason":"permission-unavailable","cause":"rate-limited"}`},
		{"comment_url", Finding{Reason: ReasonNoAuthorName, CommentURL: testCommentURL(1)}, `{"reason":"no-author-name","commentUrl":"` + testCommentURL(1) + `"}`},
		{"bare", Finding{Reason: ReasonIncompleteComments}, `{"reason":"incomplete-comments"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.finding)
			if err != nil {
				t.Fatalf("Marshal = %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal = %s, want %s", got, tc.want)
			}
		})
	}
	// A finding whose reason was never set must not reach the output.
	for _, f := range []Finding{{}, {Reason: ReasonNoWritePermission + 100}, {Reason: ReasonPermissionUnavailable, Cause: CauseFailed + 1}} {
		if _, err := json.Marshal(f); !errors.Is(err, errUnknownEnum) {
			t.Errorf("Marshal(%+v) = %v, want errUnknownEnum", f, err)
		}
	}
}
