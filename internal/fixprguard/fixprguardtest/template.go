//go:build test

package fixprguardtest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	errInvalidTemplateArg = errors.New("invalid fixture template argument")
	errSpecialFile        = errors.New("fixture template contains a non-regular, non-directory entry")
	errUnexpectedConfig   = errors.New("fixture template .git/config does not match expected path entries")
)

// Template is a fixture root (work, remote.git, home) built once and copied
// per test.
type Template struct {
	root string
}

// BuildTemplate builds the template under root with build. TestMain calls it
// once, after IsolateWorkingDir.
func BuildTemplate(root string, build func(root string) error) (*Template, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: template root is empty", errInvalidTemplateArg)
	}
	if build == nil {
		return nil, fmt.Errorf("%w: template build function is nil", errInvalidTemplateArg)
	}
	if err := build(root); err != nil {
		return nil, fmt.Errorf("build fixture template: %w", err)
	}
	return &Template{root: root}, nil
}

// Copy copies the template to dst and writes the two path values of the work
// tree's .git/config (remote.origin.url, core.excludesFile) for dst.
func (t *Template) Copy(dst string) error {
	if t == nil || t.root == "" {
		return fmt.Errorf("%w: fixture template is not initialized", errInvalidTemplateArg)
	}
	if dst == "" {
		return fmt.Errorf("%w: destination directory is empty", errInvalidTemplateArg)
	}
	if strings.ContainsAny(dst, "\n\r\x00") {
		return fmt.Errorf("%w: destination path contains a control character", errInvalidTemplateArg)
	}
	if err := copyTree(t.root, dst); err != nil {
		return err
	}
	return rewriteWorkGitConfig(t.root, dst)
}

func copyTree(srcRoot, dstRoot string) error {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return fmt.Errorf("read template dir %s: %w", srcRoot, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(srcRoot, entry.Name())
		dstPath := filepath.Join(dstRoot, entry.Name())
		info, err := os.Lstat(srcPath)
		if err != nil {
			return fmt.Errorf("stat template entry %s: %w", srcPath, err)
		}
		mode := info.Mode()
		switch {
		case mode.IsDir():
			if err := os.Mkdir(dstPath, dirMode); err != nil { //nolint:gosec // copies a directory inside the test's temporary fixture
				return fmt.Errorf("mkdir %s: %w", dstPath, err)
			}
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
			if err := os.Chmod(dstPath, mode.Perm()); err != nil { //nolint:gosec // preserves the template directory's permissions
				return fmt.Errorf("chmod %s: %w", dstPath, err)
			}
		case mode.IsRegular():
			if err := copyFile(srcPath, dstPath, mode.Perm()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %s (%s)", errSpecialFile, entry.Name(), mode.Type())
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src) //nolint:gosec // reads a file from the test fixture template
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm) //nolint:gosec // writes a file into the test's temporary fixture
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", dst, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", dst, closeErr)
	}
	if err := os.Chmod(dst, perm); err != nil { //nolint:gosec // preserves the template file's permissions
		return fmt.Errorf("chmod %s: %w", dst, err)
	}
	return nil
}

// rewriteWorkGitConfig replaces the two template-root path entries in
// <dst>/work/.git/config (core.excludesFile and remote.origin.url) with
// quoted paths under dst. It fails unless each entry appears exactly once with
// its expected template-root value and no other line references srcRoot.
func rewriteWorkGitConfig(srcRoot, dstRoot string) error {
	cfgPath := filepath.Join(dstRoot, "work", ".git", "config")
	info, err := os.Lstat(cfgPath)
	if err != nil {
		return fmt.Errorf("%w: stat config: %w", errUnexpectedConfig, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: config is not a regular file", errUnexpectedConfig)
	}
	raw, err := os.ReadFile(cfgPath) //nolint:gosec // reads .git/config inside the test's temporary fixture
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	wantExcludes := filepath.Join(srcRoot, "home", "excludes")
	wantRemote := filepath.Join(srcRoot, "remote.git")
	newExcludes := filepath.Join(dstRoot, "home", "excludes")
	newRemote := filepath.Join(dstRoot, "remote.git")

	lines := strings.Split(string(raw), "\n")
	section := ""
	excludesCount := 0
	urlCount := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if lhs, rhs, ok := strings.Cut(line, "="); ok {
			key := strings.TrimSpace(lhs)
			switch {
			case strings.EqualFold(section, "[core]") && strings.EqualFold(key, "excludesfile"):
				val, err := unquoteGitConfigValue(rhs)
				if err != nil || val != wantExcludes {
					return fmt.Errorf("%w: core.excludesfile line %q", errUnexpectedConfig, line)
				}
				lines[i] = strings.TrimRight(lhs, " \t") + " = " + quoteGitConfigValue(newExcludes)
				excludesCount++
				continue
			case isOriginRemoteSection(section) && strings.EqualFold(key, "url"):
				val, err := unquoteGitConfigValue(rhs)
				if err != nil || val != wantRemote {
					return fmt.Errorf("%w: remote.origin.url line %q", errUnexpectedConfig, line)
				}
				lines[i] = strings.TrimRight(lhs, " \t") + " = " + quoteGitConfigValue(newRemote)
				urlCount++
				continue
			}
		}
		if strings.Contains(line, srcRoot) {
			return fmt.Errorf("%w: unexpected template root reference in line %q", errUnexpectedConfig, line)
		}
	}
	if excludesCount != 1 || urlCount != 1 {
		return fmt.Errorf("%w: excludesfile=%d url=%d, want 1 each", errUnexpectedConfig, excludesCount, urlCount)
	}
	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(cfgPath, []byte(updated), info.Mode().Perm()); err != nil { //nolint:gosec // writes .git/config inside the test's temporary fixture
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func isOriginRemoteSection(section string) bool {
	if len(section) < 2 || section[0] != '[' || section[len(section)-1] != ']' {
		return false
	}
	inner := strings.TrimSpace(section[1 : len(section)-1])
	name, rest, ok := strings.Cut(inner, " ")
	if !ok || !strings.EqualFold(name, "remote") {
		return false
	}
	return strings.TrimSpace(rest) == `"origin"`
}

func quoteGitConfigValue(v string) string {
	var b strings.Builder
	b.Grow(len(v) + len(`""`))
	b.WriteByte('"')
	for i := range len(v) {
		if v[i] == '\\' || v[i] == '"' {
			b.WriteByte('\\')
		}
		b.WriteByte(v[i])
	}
	b.WriteByte('"')
	return b.String()
}

func unquoteGitConfigValue(rhs string) (string, error) {
	s := strings.TrimLeft(rhs, " \t")
	if strings.HasPrefix(s, `"`) {
		s = strings.TrimRight(s, " \t")
		if len(s) < len(`""`) || s[len(s)-1] != '"' {
			return "", fmt.Errorf("%w: unterminated quoted git config value", errUnexpectedConfig)
		}
		inner := s[1 : len(s)-1]
		var b strings.Builder
		b.Grow(len(inner))
		for i := 0; i < len(inner); i++ {
			switch inner[i] {
			case '\\':
				if i+1 >= len(inner) {
					return "", fmt.Errorf("%w: dangling escape in git config value", errUnexpectedConfig)
				}
				i++
				if inner[i] != '\\' && inner[i] != '"' {
					return "", fmt.Errorf("%w: unsupported escape \\%c in git config value", errUnexpectedConfig, inner[i])
				}
				b.WriteByte(inner[i])
			case '"':
				return "", fmt.Errorf("%w: unescaped quote in git config value", errUnexpectedConfig)
			default:
				b.WriteByte(inner[i])
			}
		}
		return b.String(), nil
	}
	s = strings.TrimRight(s, " \t")
	if strings.ContainsAny(s, `"#\`) || strings.Contains(s, ";") {
		return "", fmt.Errorf("%w: unexpected syntax in unquoted git config value", errUnexpectedConfig)
	}
	return s, nil
}
