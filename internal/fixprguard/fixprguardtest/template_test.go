//go:build test

package fixprguardtest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeMinimalTemplate(t *testing.T, configBody string) *Template {
	t.Helper()
	root := t.TempDir()
	tpl, err := BuildTemplate(root, func(r string) error {
		if err := os.MkdirAll(filepath.Join(r, "work", ".git"), 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r, "remote.git"), 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r, "home"), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(r, "home", "excludes"), nil, 0o600); err != nil {
			return err
		}
		if configBody != "" {
			body := strings.ReplaceAll(configBody, "{{ROOT}}", r)
			if err := os.WriteFile(filepath.Join(r, "work", ".git", "config"), []byte(body), 0o600); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	return tpl
}

const validTemplateConfig = `[core]
	excludesfile = {{ROOT}}/home/excludes
	ignorecase = false
[remote "origin"]
	url = {{ROOT}}/remote.git
`

func TestBuildTemplateValidation(t *testing.T) {
	t.Parallel()
	if _, err := BuildTemplate("", func(string) error { return nil }); !errors.Is(err, errInvalidTemplateArg) {
		t.Errorf("empty root: err = %v, want errInvalidTemplateArg", err)
	}
	if _, err := BuildTemplate(t.TempDir(), nil); !errors.Is(err, errInvalidTemplateArg) {
		t.Errorf("nil build: err = %v, want errInvalidTemplateArg", err)
	}
	sentinel := errors.New("boom")
	if _, err := BuildTemplate(t.TempDir(), func(string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Errorf("build error: err = %v, want sentinel", err)
	}
	var nilTpl *Template
	if err := nilTpl.Copy(t.TempDir()); !errors.Is(err, errInvalidTemplateArg) {
		t.Errorf("nil template Copy: err = %v, want errInvalidTemplateArg", err)
	}
	tpl := makeMinimalTemplate(t, validTemplateConfig)
	if err := tpl.Copy(""); !errors.Is(err, errInvalidTemplateArg) {
		t.Errorf("empty dst: err = %v, want errInvalidTemplateArg", err)
	}
	if err := tpl.Copy(filepath.Join(t.TempDir(), "bad\nname")); !errors.Is(err, errInvalidTemplateArg) {
		t.Errorf("control char in dst: err = %v, want errInvalidTemplateArg", err)
	}
}

func TestTemplateCopyRejectsConfigMismatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		config string
	}{
		{
			name:   "missing_config",
			config: "",
		},
		{
			name: "missing_excludesfile",
			config: `[core]
	ignorecase = false
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
		{
			name: "missing_remote_url",
			config: `[core]
	excludesfile = {{ROOT}}/home/excludes
`,
		},
		{
			name: "wrong_excludesfile_value",
			config: `[core]
	excludesfile = /elsewhere/home/excludes
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
		{
			name: "wrong_remote_url_value",
			config: `[core]
	excludesfile = {{ROOT}}/home/excludes
[remote "origin"]
	url = /elsewhere/remote.git
`,
		},
		{
			name: "duplicate_excludesfile",
			config: `[core]
	excludesfile = {{ROOT}}/home/excludes
	excludesfile = {{ROOT}}/home/excludes
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
		{
			name: "unexpected_root_reference",
			config: `[core]
	excludesfile = {{ROOT}}/home/excludes
	hooksPath = {{ROOT}}/hooks
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
		{
			name: "unterminated_quote",
			config: `[core]
	excludesfile = "{{ROOT}}/home/excludes
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
		{
			name: "unsupported_escape",
			config: `[core]
	excludesfile = "{{ROOT}}/home\t/excludes"
[remote "origin"]
	url = {{ROOT}}/remote.git
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tpl := makeMinimalTemplate(t, tc.config)
			dst := t.TempDir()
			if err := tpl.Copy(dst); !errors.Is(err, errUnexpectedConfig) {
				t.Errorf("Copy = %v, want errUnexpectedConfig", err)
			}
		})
	}
}

func TestTemplateCopyRejectsSpecialFile(t *testing.T) {
	t.Parallel()
	tpl := makeMinimalTemplate(t, validTemplateConfig)
	link := filepath.Join(tpl.root, "work", "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := tpl.Copy(t.TempDir()); !errors.Is(err, errSpecialFile) {
		t.Errorf("Copy with symlink = %v, want errSpecialFile", err)
	}
}

func TestTemplateCopyQuotesSpecialDestinationChars(t *testing.T) {
	t.Parallel()
	quotedConfig := `[core]
	excludesfile = "{{ROOT}}/home/excludes"
[remote "origin"]
	url = "{{ROOT}}/remote.git"
`
	tpl := makeMinimalTemplate(t, quotedConfig)
	dst := filepath.Join(t.TempDir(), "sub test #01")
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if err := tpl.Copy(dst); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dst, "work", ".git", "config"))
	if err != nil {
		t.Fatalf("read copied config: %v", err)
	}
	got := string(raw)
	wantExcludes := `"` + filepath.Join(dst, "home", "excludes") + `"`
	wantRemote := `"` + filepath.Join(dst, "remote.git") + `"`
	if !strings.Contains(got, "excludesfile = "+wantExcludes) {
		t.Errorf("copied config missing quoted excludesfile %s:\n%s", wantExcludes, got)
	}
	if !strings.Contains(got, "url = "+wantRemote) {
		t.Errorf("copied config missing quoted url %s:\n%s", wantRemote, got)
	}
}

func TestGitConfigQuoteRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw        string
		wantQuoted string
	}{
		{raw: `/tmp/plain/path`, wantQuoted: `"/tmp/plain/path"`},
		{raw: `/tmp/with space/#01/dir`, wantQuoted: `"/tmp/with space/#01/dir"`},
		{raw: `/tmp/with\backslash/and"quote"/dir`, wantQuoted: `"/tmp/with\\backslash/and\"quote\"/dir"`},
	} {
		quoted := quoteGitConfigValue(tc.raw)
		if quoted != tc.wantQuoted {
			t.Errorf("quoteGitConfigValue(%q) = %q, want %q", tc.raw, quoted, tc.wantQuoted)
		}
		got, err := unquoteGitConfigValue(quoted)
		if err != nil {
			t.Errorf("unquoteGitConfigValue(%q) err = %v", quoted, err)
			continue
		}
		if got != tc.raw {
			t.Errorf("round-trip(%q) = %q", tc.raw, got)
		}
	}
	for _, bad := range []string{
		`"unescaped"quote"`,
		`"dangling\"`,
		`unquoted # comment`,
	} {
		if _, err := unquoteGitConfigValue(bad); !errors.Is(err, errUnexpectedConfig) {
			t.Errorf("unquoteGitConfigValue(%q) = %v, want errUnexpectedConfig", bad, err)
		}
	}
}
