package fixprguard

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// maxTextFileBytes caps a reply body, PR title or PR body file.
const maxTextFileBytes = 64 << 10

// readTextFile reads a file the session wrote for reply or edit-pr. Contract:
// only a regular file (not a symbolic link, FIFO or device) of at most
// maxTextFileBytes holding non-empty valid UTF-8 is accepted; anything else is
// rejected, not repaired, so no other local file can be posted to GitHub. The
// type is checked on the opened file (opened without following a link and
// without blocking), not by a stat before opening, so a file swapped in between
// a check and the open cannot pass.
func readTextFile(name string) (string, error) {
	f, err := openRegular(name, false)
	if err != nil {
		return "", fmt.Errorf("%w: text file is not a readable regular file", ErrInvalidInput)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxTextFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: text file not readable", ErrInvalidInput)
	}
	switch {
	case len(data) > maxTextFileBytes:
		return "", fmt.Errorf("%w: text file size", ErrInvalidInput)
	case len(data) == 0:
		return "", fmt.Errorf("%w: text file empty", ErrInvalidInput)
	case !utf8.Valid(data):
		return "", fmt.Errorf("%w: text file encoding", ErrInvalidInput)
	}
	return string(data), nil
}

// readTitleFile reads a PR title file: a text file holding exactly one line
// with no line terminator.
func readTitleFile(name string) (string, error) {
	title, err := readTextFile(name)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(title, "\r\n") {
		return "", fmt.Errorf("%w: title is not one line", ErrInvalidInput)
	}
	return title, nil
}
