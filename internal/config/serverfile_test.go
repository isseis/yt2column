//go:build test

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/privatefile"
	"github.com/isseis/yt2column/internal/redact"
)

// testServerFilePath is the path the parser tests pass; it appears in every
// message they check.
const testServerFilePath = "/home/u/.config/yt2column/config.env"

// spacedTestServerFilePath is a path whose HOME holds a space, for the tests
// that need a second path.
const spacedTestServerFilePath = "/Users/J Doe/.config/yt2column/config.env"

// quotedHomePath is a path whose HOME holds a single quote, for the test that
// pins that the chmod command is left out when the redaction would break it.
const quotedHomePath = "/home/o'brien/.config/yt2column/config.env"

// fileMark is part of every value, unknown key, and rejected line the format
// tests write, so a message that carried the file's content would be
// detected.
const fileMark = "Q9FILEMARK"

// TestParseServerFileAccepts fixes the accepted shapes: blank and comment
// lines set nothing, a value is taken exactly as written ("#" without a space
// before it and "=" included), an empty value is present and empty, the last
// line needs no LF, and a file of exactly the size limit parses.
func TestParseServerFileAccepts(t *testing.T) {
	// A comment line that brings the file to exactly serverFileMaxSize bytes
	// together with the setting line after it. The parser has no size check
	// (Read enforces the limit); the case pins that a line of nearly that
	// length is read whole, which a line scanner's token limit would refuse.
	setting := "ANTHROPIC_API_KEY=k\n"
	padded := "#" + strings.Repeat("x", serverFileMaxSize-len(setting)-2) + "\n" + setting
	if len(padded) != serverFileMaxSize {
		t.Fatalf("padded file is %d bytes, want %d", len(padded), serverFileMaxSize)
	}

	cases := []struct {
		name string
		data string
		want map[string]string
	}{
		{name: "empty file", data: "", want: map[string]string{}},
		{
			name: "blank and comment lines only",
			data: "\n# ANTHROPIC_API_KEY=x\n#\n\n#export X=\"y\" # z\n",
			want: map[string]string{},
		},
		{
			name: "every key",
			data: "YT2COLUMN_LLM=deepseek/deepseek-flash\nYT2COLUMN_REVIEW_LLM=claude/claude-opus-5-5+low\n" +
				"DEEPSEEK_API_KEY=d\nANTHROPIC_API_KEY=a\nANTHROPIC_WORKSPACE_ID=w\nSLACK_WEBHOOK_URL=https://example.com/h\n",
			want: map[string]string{
				"YT2COLUMN_LLM": "deepseek/deepseek-flash", "YT2COLUMN_REVIEW_LLM": "claude/claude-opus-5-5+low",
				"DEEPSEEK_API_KEY": "d", "ANTHROPIC_API_KEY": "a", "ANTHROPIC_WORKSPACE_ID": "w",
				"SLACK_WEBHOOK_URL": "https://example.com/h",
			},
		},
		{
			name: "hash without a space and equals signs are part of the value",
			data: "SLACK_WEBHOOK_URL=https://example.com/hook#x=y\n",
			want: map[string]string{"SLACK_WEBHOOK_URL": "https://example.com/hook#x=y"},
		},
		{
			name: "a space inside the value is kept",
			data: "ANTHROPIC_WORKSPACE_ID=a b\n",
			want: map[string]string{"ANTHROPIC_WORKSPACE_ID": "a b"},
		},
		{
			name: "empty value",
			data: "DEEPSEEK_API_KEY=\n",
			want: map[string]string{"DEEPSEEK_API_KEY": ""},
		},
		{
			name: "last line without LF",
			data: "# c\nANTHROPIC_API_KEY=k",
			want: map[string]string{"ANTHROPIC_API_KEY": "k"},
		},
		{
			name: "a line of nearly the size limit",
			data: padded,
			want: map[string]string{"ANTHROPIC_API_KEY": "k"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, errs := parseServerFile(testServerFilePath, []byte(tc.data))
			if len(errs) != 0 {
				t.Fatalf("rejected: %v", errs)
			}
			for _, key := range serverFileKeys {
				value, ok := file.lookup(key)
				wantValue, wantOK := tc.want[key]
				if ok != wantOK || value != wantValue {
					t.Errorf("lookup(%s) = %q, %v; want %q, %v", key, value, ok, wantValue, wantOK)
				}
			}
		})
	}
}

// TestParseServerFileRejects fixes the rejection of each shape outside the
// format: the sentinel, the line number counted from 1 (0 for a rejection of
// the whole file), the fixed reason, and the key of a duplicate. The
// message's wording is part of what the user sees, so it is checked for the
// path, the line number, and the key, and for holding none of the file's
// content.
func TestParseServerFileRejects(t *testing.T) {
	// Three lines before the rejected one, so the line number counts them;
	// one is an accepted setting, so a rejected file that still returned
	// accepted values would be detected.
	const prefix = "# header\n\nSLACK_WEBHOOK_URL=https://example.com/h\n"
	cases := []struct {
		name       string
		data       string
		wantLine   int
		wantReason string
		wantKey    string
	}{
		{name: "no equals sign", data: prefix + fileMark + "\n", wantLine: 4, wantReason: reasonLineNoEquals},
		{name: "a key alone", data: prefix + "ANTHROPIC_API_KEY\n", wantLine: 4, wantReason: reasonLineNoEquals},
		{name: "lowercase key", data: prefix + "anthropic_api_key=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "export prefix", data: prefix + "export ANTHROPIC_API_KEY=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "retired variable", data: prefix + "YT2COLUMN_MODEL=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "GODEBUG", data: prefix + "GODEBUG=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "HOME", data: prefix + "HOME=/" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "unknown key", data: prefix + fileMark + "=x\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "ideographic space in the key", data: prefix + "ANTHROPIC_API_KEY\u3000=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "no-break space in the key", data: prefix + "ANTHROPIC_API_KEY\u00a0=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "space after the key", data: prefix + "ANTHROPIC_API_KEY =" + fileMark + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "leading space", data: prefix + " ANTHROPIC_API_KEY=" + fileMark + "\n", wantLine: 4, wantReason: reasonLineLeadingSpace},
		{name: "spaces only", data: prefix + "   \n", wantLine: 4, wantReason: reasonLineLeadingSpace},
		{name: "space before the value", data: prefix + "ANTHROPIC_API_KEY= " + fileMark + "\n", wantLine: 4, wantReason: reasonLineValueSpace},
		{name: "space after the value", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + " \n", wantLine: 4, wantReason: reasonLineValueSpace},
		{name: "double-quoted value", data: prefix + `ANTHROPIC_API_KEY="` + fileMark + `"` + "\n", wantLine: 4, wantReason: reasonLineValueQuote},
		{name: "single-quoted value", data: prefix + "ANTHROPIC_API_KEY='" + fileMark + "'\n", wantLine: 4, wantReason: reasonLineValueQuote},
		{name: "comment after the value", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + " # personal\n", wantLine: 4, wantReason: reasonLineValueComment},
		// Lines that match more than one reason get the first in the fixed
		// order.
		{name: "unknown key before a quote", data: prefix + `ANTHROPIC_API_KEY ="` + fileMark + `"` + "\n", wantLine: 4, wantReason: reasonLineUnknownKey},
		{name: "space before a quote", data: prefix + `ANTHROPIC_API_KEY= "` + fileMark + `"` + "\n", wantLine: 4, wantReason: reasonLineValueSpace},
		{name: "quote before a comment", data: prefix + `ANTHROPIC_API_KEY="` + fileMark + ` #b"` + "\n", wantLine: 4, wantReason: reasonLineValueQuote},
		{
			name: "a second key rejected for a quote is not a duplicate", data: prefix + "ANTHROPIC_API_KEY=a\nANTHROPIC_API_KEY=\"" + fileMark + "\"\n",
			wantLine: 5, wantReason: reasonLineValueQuote,
		},
		{
			name: "duplicate key", data: prefix + "ANTHROPIC_API_KEY=a\nANTHROPIC_API_KEY=" + fileMark + "\n",
			wantLine: 5, wantReason: reasonLineDuplicate, wantKey: "ANTHROPIC_API_KEY",
		},
		{name: "CRLF line end", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\r\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "tab", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\tx\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "NUL", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\x00\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "DEL", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\x7f\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "byte order mark at the start", data: "\ufeffANTHROPIC_API_KEY=" + fileMark + "\n", wantLine: 1, wantReason: reasonLineForbiddenByte},
		{name: "byte order mark in a value", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\ufeff\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "control character in a comment", data: prefix + "# " + fileMark + "\x01\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "carriage return in a comment", data: prefix + "# " + fileMark + "\r\n", wantLine: 4, wantReason: reasonLineForbiddenByte},
		{name: "invalid UTF-8", data: prefix + "ANTHROPIC_API_KEY=" + fileMark + "\xff\n", wantLine: 0, wantReason: reasonFileNotUTF8},
		{name: "invalid UTF-8 in a comment", data: prefix + "# " + fileMark + "\xc3\n", wantLine: 0, wantReason: reasonFileNotUTF8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, errs := parseServerFile(testServerFilePath, []byte(tc.data))
			for _, key := range serverFileKeys {
				if _, ok := file.lookup(key); ok {
					t.Errorf("a rejected file still sets %s", key)
				}
			}
			if len(errs) != 1 {
				t.Fatalf("got %d rejections %v, want 1", len(errs), errs)
			}
			err := errs[0]
			if !errors.Is(err, ErrConfigFileFormat) {
				t.Errorf("%v does not wrap ErrConfigFileFormat", err)
			}
			fileErr, ok := errors.AsType[*FileError](err)
			if !ok {
				t.Fatalf("got %T, want *FileError", err)
			}
			if fileErr.Path != testServerFilePath || fileErr.Line != tc.wantLine ||
				fileErr.Reason != tc.wantReason || fileErr.Key != tc.wantKey {
				t.Errorf("got path %q line %d reason %q key %q; want %q %d %q %q",
					fileErr.Path, fileErr.Line, fileErr.Reason, fileErr.Key,
					testServerFilePath, tc.wantLine, tc.wantReason, tc.wantKey)
			}
			msg := err.Error()
			if !strings.Contains(msg, testServerFilePath) {
				t.Errorf("message %q does not name the path", msg)
			}
			if tc.wantLine > 0 && !strings.Contains(msg, fmt.Sprintf("line %d:", tc.wantLine)) {
				t.Errorf("message %q does not give line %d", msg, tc.wantLine)
			}
			if tc.wantLine == 0 && strings.Contains(msg, "line ") {
				t.Errorf("message %q gives a line number for a rejection of the whole file", msg)
			}
			if tc.wantKey != "" && !strings.Contains(msg, tc.wantKey) {
				t.Errorf("message %q does not name the key %s", msg, tc.wantKey)
			}
			if strings.Contains(msg, fileMark) {
				t.Errorf("message %q holds the file's content", msg)
			}
		})
	}
}

// TestParseServerFileOverflow fixes the report of many rejected lines: the
// first serverFileMaxReported in line order, one by one, and the count of the
// rest; exactly serverFileMaxReported rejections leave no count.
func TestParseServerFileOverflow(t *testing.T) {
	// badLines returns n rejected lines, each after an accepted comment line,
	// so the rejected line numbers are 2, 4, 6, ...
	badLines := func(n int) []byte {
		var b bytes.Buffer
		for range n {
			b.WriteString("# ok\n" + fileMark + "\n")
		}
		return b.Bytes()
	}
	for _, tc := range []struct {
		name        string
		rejected    int
		wantOmitted int
		wantCount   string
	}{
		{name: "at the reporting limit", rejected: serverFileMaxReported, wantOmitted: 0},
		{name: "over the reporting limit", rejected: serverFileMaxReported + 5, wantOmitted: 5, wantCount: ": 5 more lines were rejected"},
		{name: "one over the reporting limit", rejected: serverFileMaxReported + 1, wantOmitted: 1, wantCount: ": 1 more line was rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := parseServerFile(spacedTestServerFilePath, badLines(tc.rejected))
			var lines []int
			var overflow *FormatOverflowError
			for i, err := range errs {
				if o, ok := errors.AsType[*FormatOverflowError](err); ok {
					if i != len(errs)-1 {
						t.Errorf("the count is rejection %d of %d, want the last", i+1, len(errs))
					}
					overflow = o
					continue
				}
				fileErr, ok := errors.AsType[*FileError](err)
				if !ok {
					t.Fatalf("rejection %d is %T", i, err)
				}
				lines = append(lines, fileErr.Line)
			}
			if len(lines) != serverFileMaxReported {
				t.Fatalf("got %d line rejections, want %d", len(lines), serverFileMaxReported)
			}
			for i, line := range lines {
				if want := 2 * (i + 1); line != want {
					t.Errorf("line rejection %d is line %d, want %d", i, line, want)
				}
			}
			switch {
			case tc.wantOmitted == 0 && overflow != nil:
				t.Errorf("got a count of %d at the reporting limit", overflow.Omitted)
			case tc.wantOmitted > 0 && overflow == nil:
				t.Errorf("no count of the %d unreported lines", tc.wantOmitted)
			case overflow != nil:
				if overflow.Omitted != tc.wantOmitted || overflow.Path != spacedTestServerFilePath {
					t.Errorf("got %+v, want %d omitted at %s", overflow, tc.wantOmitted, spacedTestServerFilePath)
				}
				if !errors.Is(overflow, ErrConfigFileFormat) {
					t.Errorf("the count does not wrap ErrConfigFileFormat")
				}
				// The line is what the user reads, so its wording is checked.
				if msg := overflow.Error(); !strings.Contains(msg, spacedTestServerFilePath) || !strings.HasSuffix(msg, tc.wantCount) {
					t.Errorf("message %q does not give the path and the count", msg)
				}
			}
		})
	}
}

// TestParseServerFileDuplicate fixes that a key's first appearance counts
// even when that line is rejected for another reason, so the second
// appearance is still a duplicate.
func TestParseServerFileDuplicate(t *testing.T) {
	for _, tc := range []struct {
		name        string
		first       string
		firstReason string
	}{
		{name: "first rejected for a quote", first: `ANTHROPIC_API_KEY="a"`, firstReason: reasonLineValueQuote},
		{name: "first rejected for a forbidden byte", first: "ANTHROPIC_API_KEY=a\tb", firstReason: reasonLineForbiddenByte},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := parseServerFile(testServerFilePath, []byte(tc.first+"\nANTHROPIC_API_KEY=b\n"))
			if len(errs) != 2 {
				t.Fatalf("got %d rejections %v, want 2", len(errs), errs)
			}
			first, _ := errors.AsType[*FileError](errs[0])
			second, _ := errors.AsType[*FileError](errs[1])
			if first == nil || first.Line != 1 || first.Reason != tc.firstReason {
				t.Errorf("first rejection %+v, want line 1 with %q", first, tc.firstReason)
			}
			if second == nil || second.Line != 2 || second.Reason != reasonLineDuplicate || second.Key != "ANTHROPIC_API_KEY" {
				t.Errorf("second rejection %+v, want line 2 as a duplicate of ANTHROPIC_API_KEY", second)
			}
		})
	}
}

// TestServerFileTemplate fixes the template's shape: it parses with no
// rejection and sets no key, so it holds only blank and comment lines, and it
// has a commented-out "# <KEY>=" line for every accepted key.
func TestServerFileTemplate(t *testing.T) {
	file, errs := parseServerFile(testServerFilePath, []byte(serverFileTemplate))
	if len(errs) != 0 {
		t.Fatalf("the template is rejected: %v", errs)
	}
	for _, key := range serverFileKeys {
		if _, ok := file.lookup(key); ok {
			t.Errorf("the template sets %s", key)
		}
	}
	lines := strings.Split(serverFileTemplate, "\n")
	for _, key := range serverFileKeys {
		if !slices.Contains(lines, "# "+key+"=") {
			t.Errorf("no %q line", "# "+key+"=")
		}
	}
}

// TestFileErrorMapping fixes how each privatefile failure becomes a
// configuration file rejection: the config sentinel, the path, and no
// privatefile error reachable through it. The messages are what the user
// reads to fix the file, so each is checked for the path and the part that
// says what to do; an unknown failure falls to the class that still stops.
func TestFileErrorMapping(t *testing.T) {
	const path = testServerFilePath
	loc := serverFileLocation{path: path, base: "/home/u", dirs: []string{".config", "yt2column"}}
	pathErr := func(err error) error { return &fs.PathError{Op: "op", Path: path, Err: err} }
	osErr := &fs.PathError{Op: "open", Path: path, Err: syscall.EACCES}
	unknown := errors.New("Q9 unknown failure")
	leftoverTemp := "/home/u/.config/yt2column/.config.env.tmp-123"

	cases := []struct {
		name     string
		got      *FileError
		want     error
		wantPath string
		wantText []string
	}{
		{
			name: "read: missing", got: readFileError(path, pathErr(privatefile.ErrNotExist)),
			want: ErrConfigFileMissing, wantPath: path, wantText: []string{"does not exist", "yt2column-server config init"},
		},
		{
			name: "read: symbolic link", got: readFileError(path, &privatefile.UnsafeError{Path: path, Reason: privatefile.ReasonSymlink}),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{privatefile.ReasonSymlink.String(), "nothing was changed"},
		},
		{
			name: "read: not regular", got: readFileError(path, &privatefile.UnsafeError{Path: path, Reason: privatefile.ReasonNotRegular}),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{privatefile.ReasonNotRegular.String(), "nothing was changed"},
		},
		{
			name: "read: foreign owner", got: readFileError(path, &privatefile.UnsafeError{Path: path, Reason: privatefile.ReasonForeignOwner}),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{privatefile.ReasonForeignOwner.String(), "nothing was changed"},
		},
		{
			name: "read: mode",
			got:  readFileError(spacedTestServerFilePath, &privatefile.UnsafeError{Path: spacedTestServerFilePath, Reason: privatefile.ReasonMode, Mode: 0o644}),
			want: ErrConfigFileUnsafe, wantPath: spacedTestServerFilePath,
			wantText: []string{"mode 0644", "0600", "chmod 600 '" + spacedTestServerFilePath + "'"},
		},
		{
			name: "read: mode with the setuid bit",
			got:  readFileError(path, &privatefile.UnsafeError{Path: path, Reason: privatefile.ReasonMode, Mode: 0o600 | fs.ModeSetuid}),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{"mode 4600", "chmod 600 '" + path + "'"},
		},
		{
			name: "read: mode with a single quote in the path",
			got:  readFileError(quotedHomePath, &privatefile.UnsafeError{Path: quotedHomePath, Reason: privatefile.ReasonMode, Mode: 0o644}),
			want: ErrConfigFileUnsafe, wantPath: quotedHomePath,
			wantText: []string{"mode 0644", "chmod 600 on the file"},
		},
		{
			name: "read: unsafe without its reason", got: readFileError(path, fmt.Errorf("wrapped: %w", privatefile.ErrUnsafe)),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{privatefile.ReasonUnknown.String(), "nothing was changed"},
		},
		{
			name: "read: unknown unsafe reason", got: readFileError(path, &privatefile.UnsafeError{Path: path}),
			want: ErrConfigFileUnsafe, wantPath: path, wantText: []string{privatefile.ReasonUnknown.String()},
		},
		{
			name: "read: too large", got: readFileError(path, pathErr(privatefile.ErrTooLarge)),
			want: ErrConfigFileFormat, wantPath: path, wantText: []string{"65536 bytes"},
		},
		{
			name: "read: an OS error", got: readFileError(path, osErr),
			want: ErrConfigFileUnreadable, wantPath: path, wantText: []string{"could not be read", syscall.EACCES.Error()},
		},
		{
			name: "read: an unknown error", got: readFileError(path, unknown),
			want: ErrConfigFileUnreadable, wantPath: path, wantText: []string{"could not be read", unknown.Error()},
		},
		{
			name: "create: exists", got: createFileError(loc, pathErr(privatefile.ErrExist)),
			want: ErrConfigFileExists, wantPath: path, wantText: []string{"already exists", "nothing was changed"},
		},
		{
			name: "create: base missing", got: createFileError(loc, &fs.PathError{Op: "mkdir", Path: loc.base, Err: privatefile.ErrBaseMissing}),
			want: ErrConfigFileCreate, wantPath: loc.base, wantText: []string{"does not exist or is not a directory", "nothing was created"},
		},
		{
			name: "create: an OS error", got: createFileError(loc, osErr),
			want: ErrConfigFileCreate, wantPath: path, wantText: []string{"could not create the configuration file", syscall.EACCES.Error()},
		},
		{
			name: "create: an unknown error", got: createFileError(loc, unknown),
			want: ErrConfigFileCreate, wantPath: path, wantText: []string{"could not create the configuration file", unknown.Error()},
		},
		{
			name: "create: exists with a leftover temporary file",
			got:  createFileError(loc, &privatefile.LeftoverError{Err: pathErr(privatefile.ErrExist), Temp: leftoverTemp, RemoveErr: syscall.EACCES}),
			want: ErrConfigFileExists, wantPath: path, wantText: []string{"already exists", "remove the temporary file " + leftoverTemp},
		},
		{
			name: "create: a failure with a leftover temporary file",
			got:  createFileError(loc, &privatefile.LeftoverError{Err: osErr, Temp: leftoverTemp, RemoveErr: syscall.EACCES}),
			want: ErrConfigFileCreate, wantPath: path, wantText: []string{"could not create the configuration file", "remove the temporary file " + leftoverTemp},
		},
	}
	sentinels := []error{
		ErrConfigFileMissing, ErrConfigFileUnsafe, ErrConfigFileUnreadable,
		ErrConfigFileFormat, ErrConfigFileExists, ErrConfigFileCreate,
	}
	privateSentinels := []error{
		privatefile.ErrNotExist, privatefile.ErrUnsafe, privatefile.ErrTooLarge,
		privatefile.ErrExist, privatefile.ErrBaseMissing,
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range sentinels {
				if got := errors.Is(tc.got, s); got != (s == tc.want) {
					t.Errorf("errors.Is(%v, %v) = %v", tc.got, s, got)
				}
			}
			for _, s := range privateSentinels {
				if errors.Is(tc.got, s) {
					t.Errorf("%v exposes privatefile's %v", tc.got, s)
				}
			}
			if _, ok := errors.AsType[*privatefile.UnsafeError](tc.got); ok {
				t.Errorf("%v exposes *privatefile.UnsafeError", tc.got)
			}
			if tc.got.Path != tc.wantPath {
				t.Errorf("got path %q, want %q", tc.got.Path, tc.wantPath)
			}
			msg := tc.got.Error()
			for _, want := range append([]string{tc.wantPath}, tc.wantText...) {
				if !strings.Contains(msg, want) {
					t.Errorf("message %q does not contain %q", msg, want)
				}
			}
		})
	}
}

// TestModeMessageSurvivesRedaction pins the displayed mode rejection: the
// server writes every line through redact.Line, which escapes a backslash and
// a double quote, so a path it cannot quote must not be named in the chmod
// command. The command would otherwise be a shell syntax error when copied.
func TestModeMessageSurvivesRedaction(t *testing.T) {
	modeError := func(path string) string {
		return readFileError(path, &privatefile.UnsafeError{Path: path, Reason: privatefile.ReasonMode, Mode: 0o644}).Error()
	}
	unquotable := []string{
		quotedHomePath,
		`/home/a\b/.config/yt2column/config.env`,
		"/home/a\"b/.config/yt2column/config.env",
	}
	for _, path := range unquotable {
		displayed := redact.Line(modeError(path))
		if strings.Contains(displayed, "chmod 600 '") {
			t.Errorf("displayed message %q carries a chmod command for a path it cannot quote", displayed)
		}
		if !strings.Contains(displayed, "chmod 600 on the file") {
			t.Errorf("displayed message %q does not name the fix", displayed)
		}
	}
	safe := spacedTestServerFilePath
	displayed := redact.Line(modeError(safe))
	if !strings.Contains(displayed, "chmod 600 '"+safe+"'") {
		t.Errorf("displayed message %q does not carry the copyable command", displayed)
	}
}

// skipIfRoot skips a test that depends on permissions, which root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not hold for root")
	}
}

// testServerFileLocation returns the configuration file's location under
// home on the running system.
func testServerFileLocation(t *testing.T, home string) serverFileLocation {
	t.Helper()
	loc, err := serverFileLocationFor(runtime.GOOS, envLookup(map[string]string{"HOME": home}))
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// requireMode fails the test unless path is exactly mode.
func requireMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky); got != mode {
		t.Errorf("%s has mode %v, want %v", path, got, mode)
	}
}

// requireAbsent fails the test when anything is at path.
func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s exists (Lstat: %v), want nothing there", path, err)
	}
}

// requireEntries fails the test unless dir holds exactly the names want, so
// no temporary file is left whatever its name.
func requireEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s holds %q, want %q", dir, got, want)
	}
}

// TestInitServerFile fixes config init's work below HOME: the template is
// created at the path with mode 0600 and the missing directories with mode
// 0700, reading HOME alone; what is at the path is left unchanged; a missing
// base creates nothing; a failure leaves nothing at the path but keeps the
// directories already created; and a bad HOME is the *VarError, creating
// nothing.
func TestInitServerFile(t *testing.T) {
	// requireFileError checks err is a *FileError wrapping sentinel about
	// path.
	requireFileError := func(t *testing.T, err, sentinel error, path string) {
		t.Helper()
		fileErr, ok := errors.AsType[*FileError](err)
		if !ok || !errors.Is(err, sentinel) || fileErr.Path != path {
			t.Fatalf("got %v (%T), want a *FileError wrapping %v about %s", err, err, sentinel, path)
		}
	}

	t.Run("creates the template", func(t *testing.T) {
		home := t.TempDir()
		loc := testServerFileLocation(t, home)
		if err := os.MkdirAll(loc.base, 0o700); err != nil {
			t.Fatal(err)
		}
		// Every key holds a value in the environment, so the template would
		// show it if InitServerFile read it.
		env := map[string]string{"HOME": home}
		for _, key := range serverFileKeys {
			env[key] = fileMark
		}
		var read []string
		result, err := InitServerFile(recordingLookup(env, &read))
		if err != nil {
			t.Fatal(err)
		}
		requireReadOnlyHome(t, "InitServerFile", read)
		if result.Path != loc.path || result.LeftoverTemp != "" {
			t.Errorf("got %+v, want path %s and no leftover", result, loc.path)
		}
		content, err := os.ReadFile(loc.path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != serverFileTemplate {
			t.Errorf("content %q, want the template", content)
		}
		requireMode(t, loc.path, privatefile.Mode)
		dir := loc.base
		for _, name := range loc.dirs {
			dir = filepath.Join(dir, name)
			requireMode(t, dir, privatefile.DirMode)
		}
		requireEntries(t, filepath.Dir(loc.path), serverFileName)
	})

	t.Run("something already at the path", func(t *testing.T) {
		for _, kind := range []string{"regular file", "dangling symbolic link", "directory"} {
			t.Run(kind, func(t *testing.T) {
				home := t.TempDir()
				loc := testServerFileLocation(t, home)
				if err := os.MkdirAll(filepath.Dir(loc.path), 0o700); err != nil {
					t.Fatal(err)
				}
				var err error
				switch kind {
				case "regular file":
					err = os.WriteFile(loc.path, []byte(fileMark), 0o600)
				case "dangling symbolic link":
					err = os.Symlink(filepath.Join(home, "absent"), loc.path)
				case "directory":
					err = os.Mkdir(loc.path, 0o700)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.Lstat(loc.path)
				if err != nil {
					t.Fatal(err)
				}
				_, err = InitServerFile(envLookup(map[string]string{"HOME": home}))
				requireFileError(t, err, ErrConfigFileExists, loc.path)
				after, err := os.Lstat(loc.path)
				if err != nil {
					t.Fatal(err)
				}
				if before.Mode() != after.Mode() || !os.SameFile(before, after) {
					t.Errorf("what is at the path changed: %v -> %v", before.Mode(), after.Mode())
				}
				if kind == "regular file" {
					if content, _ := os.ReadFile(loc.path); string(content) != fileMark {
						t.Errorf("the existing file now holds %q", content)
					}
				}
			})
		}
	})

	t.Run("base missing", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "absent")
		loc := testServerFileLocation(t, home)
		_, err := InitServerFile(envLookup(map[string]string{"HOME": home}))
		requireFileError(t, err, ErrConfigFileCreate, loc.base)
		requireAbsent(t, home)
	})

	t.Run("parent not writable", func(t *testing.T) {
		skipIfRoot(t)
		home := t.TempDir()
		loc := testServerFileLocation(t, home)
		if err := os.MkdirAll(loc.base, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(loc.base, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(loc.base, 0o700) })
		_, err := InitServerFile(envLookup(map[string]string{"HOME": home}))
		requireFileError(t, err, ErrConfigFileCreate, loc.path)
		requireAbsent(t, filepath.Join(loc.base, loc.dirs[0]))
	})

	t.Run("failure after the directories were created", func(t *testing.T) {
		home := t.TempDir()
		loc := testServerFileLocation(t, home)
		if err := os.MkdirAll(loc.base, 0o700); err != nil {
			t.Fatal(err)
		}
		privatefile.FailCreateBeforeLinkForTest(t)
		_, err := InitServerFile(envLookup(map[string]string{"HOME": home}))
		requireFileError(t, err, ErrConfigFileCreate, loc.path)
		requireAbsent(t, loc.path)
		dir := loc.base
		for _, name := range loc.dirs {
			dir = filepath.Join(dir, name)
			requireMode(t, dir, privatefile.DirMode)
		}
		requireEntries(t, filepath.Dir(loc.path))
	})

	t.Run("bad HOME", func(t *testing.T) {
		for name, env := range map[string]map[string]string{
			"unset":    {},
			"empty":    {"HOME": ""},
			"relative": {"HOME": "rel"},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := InitServerFile(envLookup(env))
				varErr, ok := errors.AsType[*VarError](err)
				if !ok || varErr.Name != "HOME" || !errors.Is(err, ErrMissing) {
					t.Fatalf("got %v (%T), want the *VarError naming HOME", err, err)
				}
			})
		}
	})
}
