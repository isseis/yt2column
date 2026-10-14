//go:build unix

// Package authtoken holds the shared token that authenticates the extension,
// and reads or creates the file that stores it. The token is the only thing
// that distinguishes the extension's requests from any other local process, so
// its file is created atomically and checked strictly, and reading it never
// repairs or regenerates it.
package authtoken

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/isseis/yt2column/internal/secret"
)

const (
	// tokenFileName is the token file's name. It matches the name
	// internal/config derives under the configuration directory.
	tokenFileName = "server-token"
	// tokenBytes is the number of random bytes drawn from random.
	tokenBytes = 32
	// tokenLength is the token's length in characters: 32 bytes as lowercase
	// hexadecimal.
	tokenLength = 64
	// readLimit is the number of bytes read from the file. One byte past the
	// token length proves the file ends after it, so a 65-byte file is
	// rejected instead of being truncated.
	readLimit = tokenLength + 1
	// dirMode is the permission of the token directory this package creates.
	// An existing directory keeps its own mode.
	dirMode = 0o700
	// othersMode is the set of permission bits a readable or writable token
	// file must not carry. A file that any user other than its owner can read
	// or write is rejected.
	othersMode = 0o077
)

// tokenPattern is the token file's accepted content: exactly 64 lowercase
// hexadecimal characters, with no leading or trailing byte.
var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// The fixed reasons a token file is rejected. A reason is chosen from these
// constants and built from nothing else, so the file's content cannot reach an
// error message.
const (
	reasonSymlink     = "is a symbolic link"
	reasonNotRegular  = "is not a regular file"
	reasonPermissions = "is readable or writable by others"
	reasonOwner       = "is not owned by the current user"
	reasonContent     = "does not hold exactly 64 lowercase hexadecimal characters"
)

// ErrInvalidTokenFile reports that the shared token file is present but does
// not meet the accepted form. A failure for any other reason does not wrap it,
// so a caller can tell a rejected file from a missing one or an I/O error. The
// path and the reason are carried by *FileError.
var ErrInvalidTokenFile = errors.New("the shared token file is not acceptable")

// FileError names the token file and a fixed reason it was rejected. It never
// holds the file's content, so it is safe to log even when the file was a
// secret in the wrong form.
type FileError struct {
	Path   string
	Reason string // one of the reason constants above
}

// Error implements the error interface as "<path>: <reason>".
func (e *FileError) Error() string {
	return e.Path + ": " + e.Reason
}

// Unwrap returns ErrInvalidTokenFile, so errors.Is(err, ErrInvalidTokenFile) is
// true for every rejected file.
func (e *FileError) Unwrap() error {
	return ErrInvalidTokenFile
}

// Token is the shared token that authenticates the extension. Its value is
// held as a secret.Secret, so printing a Token never shows the value. The zero
// value matches nothing.
type Token struct {
	value secret.Secret
}

// LoadOrCreate reads the token file at path, or creates it when it does not
// exist. created reports whether this call generated the token. random is the
// source of the token bytes; production passes crypto/rand.Reader. A file that
// exists but does not meet the accepted form is reported as a *FileError
// wrapping ErrInvalidTokenFile and is neither repaired nor regenerated.
func LoadOrCreate(path string, random io.Reader) (token Token, created bool, err error) {
	token, err = read(path)
	if err == nil {
		return token, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Token{}, false, err
	}
	token, err = create(path, random)
	if err != nil {
		return Token{}, false, err
	}
	return token, true, nil
}

// Matches reports whether presented equals the token, in constant time with
// respect to the content of presented. It returns false for the zero Token and
// for an empty presented value, so an empty file and an empty header are never
// a match. The token's length is a public part of the protocol, so comparing
// lengths first leaks nothing.
func (t Token) Matches(presented string) bool {
	if presented == "" {
		return false
	}
	value, err := t.value.Reveal()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(presented)) == 1
}

// IsZero reports whether t is the zero Token.
func (t Token) IsZero() bool {
	_, err := t.value.Reveal()
	return err != nil
}

// Value returns the token as a secret.Secret, for the one-time display after
// creation and for the log redaction list. The zero Token returns the zero
// secret.
func (t Token) Value() secret.Secret {
	return t.value
}

// read returns the token stored in the file at path, or a *FileError when the
// file is present but not acceptable, or the open or stat error otherwise (a
// missing file is an fs.ErrNotExist error, which LoadOrCreate turns into a
// creation).
func read(path string) (Token, error) {
	// O_NOFOLLOW refuses a symbolic link at the name instead of reading its
	// target; O_NONBLOCK makes opening a named pipe return at once instead of
	// waiting for a writer. The type is still checked below.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // the path is the caller-derived token file
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return Token{}, &FileError{Path: path, Reason: reasonSymlink}
		}
		return Token{}, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return Token{}, fmt.Errorf("stat the shared token file %s: %w", path, err)
	}
	switch {
	case !info.Mode().IsRegular():
		return Token{}, &FileError{Path: path, Reason: reasonNotRegular}
	case info.Mode().Perm()&othersMode != 0:
		return Token{}, &FileError{Path: path, Reason: reasonPermissions}
	}
	if !ownedByCurrentUser(info) {
		return Token{}, &FileError{Path: path, Reason: reasonOwner}
	}

	data, err := io.ReadAll(io.LimitReader(file, readLimit))
	if err != nil {
		return Token{}, fmt.Errorf("read the shared token file %s: %w", path, err)
	}
	if !tokenPattern.Match(data) {
		return Token{}, &FileError{Path: path, Reason: reasonContent}
	}
	value, err := secret.New(string(data))
	if err != nil {
		return Token{}, fmt.Errorf("hold the shared token: %w", err)
	}
	return Token{value: value}, nil
}

// ownedByCurrentUser reports whether the file info names a file whose owner is
// the effective user.
func ownedByCurrentUser(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return uint32(os.Geteuid()) == st.Uid //nolint:gosec // a user ID is non-negative and fits in uint32
}

// create generates a token from random and publishes it at path. It writes a
// temporary file in the same directory, syncs it, and gives it the final name
// with link(2); the name therefore only ever refers to a complete token even if
// another process reads it while the write is in flight. When link(2) reports
// that the name already exists, another process created the token first, so
// this call reads (and never overwrites) that file.
func create(path string, random io.Reader) (Token, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return Token{}, fmt.Errorf("create the token directory %s: %w", dir, err)
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return Token{}, fmt.Errorf("generate the shared token: %w", err)
	}
	value := hex.EncodeToString(raw)

	file, err := os.CreateTemp(dir, tokenFileName+".tmp-*")
	if err != nil {
		return Token{}, fmt.Errorf("create a temporary token file in %s: %w", dir, err)
	}
	tmpPath := file.Name()
	if _, err := file.WriteString(value); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return Token{}, fmt.Errorf("write the temporary token file %s: %w", tmpPath, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return Token{}, fmt.Errorf("sync the temporary token file %s: %w", tmpPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return Token{}, fmt.Errorf("close the temporary token file %s: %w", tmpPath, err)
	}

	if err := os.Link(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		if errors.Is(err, fs.ErrExist) {
			return read(path)
		}
		return Token{}, fmt.Errorf("name the shared token file %s: %w", path, err)
	}
	// The token is complete under its final name. A leftover temporary file
	// does not affect the next read, so its removal is best effort.
	_ = os.Remove(tmpPath)

	secretValue, err := secret.New(value)
	if err != nil {
		return Token{}, fmt.Errorf("hold the shared token: %w", err)
	}
	return Token{value: secretValue}, nil
}
