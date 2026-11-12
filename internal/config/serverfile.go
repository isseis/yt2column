package config

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/privatefile"
)

// serverFileKeys are the six keys the configuration file accepts: the names
// of the variables the CLI reads, written once at the top of config.go. Any
// other key, SLACK_WEBHOOK_URL included, is rejected as an unknown key.
var serverFileKeys = []string{
	llmEnv, reviewLLMEnv, deepSeekAPIKeyEnv,
	anthropicAPIKeyEnv, openAIAPIKeyEnv, anthropicWorkspaceIDEnv,
}

// serverFileMaxSize is the largest configuration file accepted, in bytes.
const serverFileMaxSize = 64 << 10

// serverFileMaxReported is how many rejected lines are reported one by one.
const serverFileMaxReported = 20

// The sentinels of the configuration file's rejections, one per class. A
// caller tells the classes apart with errors.Is; the privatefile errors they
// are mapped from never leave this package.
var (
	// ErrConfigFileMissing reports that nothing is at the configuration
	// file's path.
	ErrConfigFileMissing = errors.New("the configuration file does not exist")
	// ErrConfigFileUnsafe reports a file that must not be read: a symbolic
	// link, not a regular file, another user's, or not exactly mode 0600.
	ErrConfigFileUnsafe = errors.New("the configuration file must not be read")
	// ErrConfigFileUnreadable reports a file that could not be opened or read
	// for any other reason.
	ErrConfigFileUnreadable = errors.New("the configuration file could not be read")
	// ErrConfigFileFormat reports content outside the accepted format, the
	// size limit included.
	ErrConfigFileFormat = errors.New("the configuration file is not in the accepted format")
	// ErrConfigFileExists reports that config init found something at the
	// path and left it unchanged.
	ErrConfigFileExists = errors.New("a configuration file already exists")
	// ErrConfigFileCreate reports that config init could not create the file
	// or a directory above it.
	ErrConfigFileCreate = errors.New("the configuration file could not be created")
)

// Fixed reasons of the format rejections. A line gets exactly one, the first
// that applies in the order below, so a reason never depends on more than
// the line itself and the keys seen before it.
const (
	reasonLineForbiddenByte = "contains a control character, a carriage return, DEL, or U+FEFF"
	reasonLineLeadingSpace  = "starts with a space"
	reasonLineNoEquals      = `is not empty, not a comment starting with "#", and has no "="`
	reasonLineValueSpace    = "has a value that starts or ends with a space"
	reasonLineValueQuote    = `has a value that starts with a quotation mark; quotes are not removed`
	reasonLineValueComment  = `has a space followed by "#" in the value; a comment must be on a line of its own`
	reasonLineDuplicate     = "is set on an earlier line too"
	reasonFileNotUTF8       = "the file is not valid UTF-8"
)

// Fixed reasons built from code values when the package is initialized.
var (
	// reasonLineUnknownKey rejects a key that is not exactly one of
	// serverFileKeys. It lists the accepted keys, never the one read.
	reasonLineUnknownKey = `has a key before the first "=" that is not one of ` + strings.Join(serverFileKeys, ", ")
	// reasonFileTooLarge rejects a file over serverFileMaxSize.
	reasonFileTooLarge = "the file is larger than " + strconv.Itoa(serverFileMaxSize) + " bytes"
)

// Fixed reasons of the rejections of the file itself.
const (
	reasonFileMissing    = "does not exist; create it with yt2column-server config init"
	reasonFileUnreadable = "could not be read"
	reasonFileExists     = "already exists; nothing was changed"
	reasonBaseMissing    = "does not exist or is not a directory; nothing was created"
	reasonFileCreate     = "could not be created"
	// reasonNothingChanged follows the reason of a refused file, so the user
	// knows the server repaired nothing.
	reasonNothingChanged = "; nothing was changed"
)

// fileErrorForm is the shape of a FileError's message. Its zero value is the
// plain shape, so a FileError built without a form still names its path and
// reason.
type fileErrorForm int

const (
	// formPlain is "configuration file <Path> <Reason>".
	formPlain fileErrorForm = iota
	// formLine is "configuration file <Path>: line <Line>: [<Key> ]<Reason>".
	formLine
	// formWholeFile is "configuration file <Path>: <Reason>".
	formWholeFile
	// formUnsafe is formPlain followed by reasonNothingChanged.
	formUnsafe
	// formMode is the mode refusal with the mode found and the chmod
	// command that fixes it.
	formMode
	// formDetail is formPlain followed by ": <detail>".
	formDetail
	// formBase is "<Path> <Reason>", Path being the missing base directory.
	formBase
	// formCreate is "could not create the configuration file <Path>:
	// <detail>".
	formCreate
)

// FileError reports one rejection about the configuration file. Path is the
// path the rejection is about: the configuration file, or the base directory
// when config init finds it missing. Line is the line number, 0 when the
// rejection is not about one line. Key is set only when the reason names one
// of serverFileKeys (a duplicate). Reason is a fixed text chosen from code
// constants. It never holds a value, a line, or a key read from the file.
type FileError struct {
	Path   string
	Line   int
	Key    string
	Reason string
	Err    error // one of the ErrConfigFile* sentinels
	// detail is the OS error text or the mode found, for the message only.
	detail string
	form   fileErrorForm
	// leftover is the temporary file Create could not remove after the
	// failure, or ""; the message asks the user to remove it.
	leftover string
}

// Error implements the error interface in the shape the error's form names,
// followed by the request to remove a leftover temporary file.
func (e *FileError) Error() string {
	if e.leftover != "" {
		return e.message() + "; remove the temporary file " + e.leftover + " by hand"
	}
	return e.message()
}

// message is the error's text without the leftover request.
func (e *FileError) message() string {
	switch e.form {
	case formLine:
		reason := e.Reason
		if e.Key != "" {
			reason = e.Key + " " + reason
		}
		return fmt.Sprintf("configuration file %s: line %d: %s", e.Path, e.Line, reason)
	case formWholeFile:
		return "configuration file " + e.Path + ": " + e.Reason
	case formUnsafe:
		return "configuration file " + e.Path + " " + e.Reason + reasonNothingChanged
	case formMode:
		// The command names the path as one shell word when it can: a path
		// with a space (a HOME such as "/Users/J Doe") then stays one word.
		// The redaction of the output line escapes a backslash and a double
		// quote, which the shell needs to escape a single quote, so a path
		// with one of those cannot be shown as a command that runs as
		// displayed; it is named at the start of the line instead.
		if quoted, ok := shellQuotePath(e.Path); ok {
			return fmt.Sprintf("configuration file %s has mode %s, not exactly %04o; run chmod %o %s",
				e.Path, e.detail, privatefile.UnixMode(privatefile.Mode), privatefile.UnixMode(privatefile.Mode), quoted)
		}
		return fmt.Sprintf("configuration file %s has mode %s, not exactly %04o; run chmod %o on the file",
			e.Path, e.detail, privatefile.UnixMode(privatefile.Mode), privatefile.UnixMode(privatefile.Mode))
	case formDetail:
		return "configuration file " + e.Path + " " + e.Reason + ": " + e.detail
	case formBase:
		return e.Path + " " + e.Reason
	case formCreate:
		return "could not create the configuration file " + e.Path + ": " + e.detail
	default:
		return "configuration file " + e.Path + " " + e.Reason
	}
}

// shellQuotePath quotes path as one word of a POSIX shell, in single quotes,
// which the redaction of the output line leaves alone: a path with a space
// (a HOME such as "/Users/J Doe") can then be copied as it is. It reports
// false when the path holds a single quote (which would end the word), a
// double quote or a backslash (which the redaction escapes), or a
// non-printable character: the shell would need an escape that the redaction
// alters, so no command naming such a path runs as displayed.
func shellQuotePath(path string) (string, bool) {
	if strings.ContainsAny(path, "'\"\\") {
		return "", false
	}
	for _, r := range path {
		if !strconv.IsPrint(r) {
			return "", false
		}
	}
	return "'" + path + "'", true
}

// Unwrap returns the sentinel that classifies the rejection, and only that:
// an Unwrap() []error would make the server's output split one FileError as
// if it were a joined list.
func (e *FileError) Unwrap() error { return e.Err }

// FormatOverflowError reports how many rejected lines were not reported one
// by one. It wraps ErrConfigFileFormat.
type FormatOverflowError struct {
	Path    string
	Omitted int
}

// Error implements the error interface.
func (e *FormatOverflowError) Error() string {
	if e.Omitted == 1 {
		return "configuration file " + e.Path + ": 1 more line was rejected"
	}
	return fmt.Sprintf("configuration file %s: %d more lines were rejected", e.Path, e.Omitted)
}

// Unwrap returns ErrConfigFileFormat.
func (e *FormatOverflowError) Unwrap() error { return ErrConfigFileFormat }

// readFileError maps a failure of privatefile.Read on the configuration file
// at path to a *FileError. Contract: the class is chosen with errors.Is on
// the privatefile sentinels; anything else, an unknown error included, is
// "could not be read", which still stops the server.
func readFileError(path string, err error) *FileError {
	switch {
	case errors.Is(err, privatefile.ErrNotExist):
		return &FileError{Path: path, Reason: reasonFileMissing, Err: ErrConfigFileMissing, form: formPlain}
	case errors.Is(err, privatefile.ErrUnsafe):
		unsafe, ok := errors.AsType[*privatefile.UnsafeError](err)
		if !ok {
			return &FileError{Path: path, Reason: privatefile.ReasonUnknown.String(), Err: ErrConfigFileUnsafe, form: formUnsafe}
		}
		if unsafe.Reason == privatefile.ReasonMode {
			return &FileError{
				Path: path, Reason: unsafe.Reason.String(), Err: ErrConfigFileUnsafe,
				detail: fmt.Sprintf("%04o", privatefile.UnixMode(unsafe.Mode)), form: formMode,
			}
		}
		return &FileError{Path: path, Reason: unsafe.Reason.String(), Err: ErrConfigFileUnsafe, form: formUnsafe}
	case errors.Is(err, privatefile.ErrTooLarge):
		return &FileError{Path: path, Reason: reasonFileTooLarge, Err: ErrConfigFileFormat, form: formWholeFile}
	default:
		return &FileError{Path: path, Reason: reasonFileUnreadable, Err: ErrConfigFileUnreadable, detail: err.Error(), form: formDetail}
	}
}

// createFileError maps a failure of privatefile.MkdirBelow or
// privatefile.Create for the configuration file at loc to a *FileError.
// Contract: the class is chosen with errors.Is on the privatefile sentinels;
// anything else, an unknown error included, is "could not be created". A
// temporary file Create could not remove is named in every class.
func createFileError(loc serverFileLocation, err error) *FileError {
	var fileErr *FileError
	switch {
	case errors.Is(err, privatefile.ErrExist):
		fileErr = &FileError{Path: loc.path, Reason: reasonFileExists, Err: ErrConfigFileExists, form: formPlain}
	case errors.Is(err, privatefile.ErrBaseMissing):
		fileErr = &FileError{Path: loc.base, Reason: reasonBaseMissing, Err: ErrConfigFileCreate, form: formBase}
	default:
		detail := err
		if left, ok := errors.AsType[*privatefile.LeftoverError](err); ok {
			detail = left.Err
		}
		fileErr = &FileError{Path: loc.path, Reason: reasonFileCreate, Err: ErrConfigFileCreate, detail: detail.Error(), form: formCreate}
	}
	if left, ok := errors.AsType[*privatefile.LeftoverError](err); ok {
		fileErr.leftover = left.Temp
	}
	return fileErr
}

// serverFile is the accepted content of a configuration file: each key that
// appears, with its value as written. Only parseServerFile builds one.
type serverFile struct {
	values map[string]string
}

// lookup has the signature of LookupFunc, so the value loaders shared with
// the CLI read the file through it. A key written with an empty value is
// present with "", as an empty environment variable is.
func (f serverFile) lookup(name string) (string, bool) {
	value, ok := f.values[name]
	return value, ok
}

// keyError returns the rejection of a key read from the file: e marked
// VarSourceConfigFile, with the reason of an ErrMissing chosen by whether the
// file has the key ("is not set") or has it with an empty value ("is empty").
// The choice is made from the parsed keys, never from e's text.
func (f serverFile) keyError(e *VarError) *VarError {
	out := *e
	out.Source = VarSourceConfigFile
	if errors.Is(e.Err, ErrMissing) {
		if _, present := f.values[e.Name]; present {
			out.Reason = reasonEmpty
		} else {
			out.Reason = reasonNotSet
		}
	}
	return &out
}

// readServerFile reads the configuration file at path with privatefile.Read
// and parses it. Contract: it returns the content only when the file passed
// every check and every line was accepted; otherwise a *FileError, or the
// format rejections joined flat, so the server reports each on its own line.
func readServerFile(path string) (serverFile, error) {
	data, err := privatefile.Read(path, serverFileMaxSize)
	if err != nil {
		return serverFile{}, readFileError(path, err)
	}
	file, rejections := parseServerFile(path, data)
	if len(rejections) > 0 {
		return serverFile{}, errors.Join(rejections...)
	}
	return file, nil
}

// parseServerFile returns the accepted content, or the rejections: one
// *FileError per rejected line, at most serverFileMaxReported of them in line
// order, and a *FormatOverflowError when more lines were rejected. Contract:
// a file with any rejection yields no content, so no value of a rejected file
// is ever used; nothing is repaired; and no rejection holds a value, a line,
// or a key read from the file. The grammar is the one declared by the
// accepted format: a line's kind is decided by its first byte and its first
// "=", and a key by exact match against serverFileKeys.
func parseServerFile(path string, data []byte) (serverFile, []error) {
	if !utf8.Valid(data) {
		return serverFile{}, []error{&FileError{Path: path, Reason: reasonFileNotUTF8, Err: ErrConfigFileFormat, form: formWholeFile}}
	}
	// A final LF leaves an empty last element, which reads as a blank line
	// and so changes nothing.
	lines := bytes.Split(data, []byte{'\n'})
	values := map[string]string{}
	seen := map[string]struct{}{}
	var rejections []error
	omitted := 0
	for i, line := range lines {
		key, value, reason := parseServerFileLine(line, seen)
		if reason == "" {
			if key != "" {
				values[key] = value
			}
			continue
		}
		if len(rejections) == serverFileMaxReported {
			omitted++
			continue
		}
		rejections = append(rejections, &FileError{Path: path, Line: i + 1, Key: key, Reason: reason, Err: ErrConfigFileFormat, form: formLine})
	}
	if omitted > 0 {
		rejections = append(rejections, &FormatOverflowError{Path: path, Omitted: omitted})
	}
	if len(rejections) > 0 {
		return serverFile{}, rejections
	}
	return serverFile{values: values}, nil
}

// parseServerFileLine reads one line, without its LF. It returns the key and
// value of an accepted setting line, empty strings for an accepted blank or
// comment line, or the fixed reason of the rejection. A line whose part
// before the first "=" is exactly one of serverFileKeys is recorded in seen
// even when it is rejected, so a later line with that key is a duplicate.
// Of the rejections, only a duplicate returns its key, so the rejection can
// name it; key is then one of serverFileKeys, never text read from the file.
func parseServerFileLine(line []byte, seen map[string]struct{}) (key, value, reason string) {
	text := string(line)
	key, value, hasEquals := strings.Cut(text, "=")
	isKey := hasEquals && slices.Contains(serverFileKeys, key)
	_, duplicate := seen[key]
	if isKey {
		seen[key] = struct{}{}
	}
	switch {
	case strings.ContainsFunc(text, isForbiddenRune):
		return "", "", reasonLineForbiddenByte
	case text == "", text[0] == '#':
		return "", "", ""
	case text[0] == ' ':
		return "", "", reasonLineLeadingSpace
	case !hasEquals:
		return "", "", reasonLineNoEquals
	case !isKey:
		return "", "", reasonLineUnknownKey
	case value != "" && (value[0] == ' ' || value[len(value)-1] == ' '):
		return "", "", reasonLineValueSpace
	case value != "" && (value[0] == '"' || value[0] == '\''):
		return "", "", reasonLineValueQuote
	case strings.Contains(value, " #"):
		return "", "", reasonLineValueComment
	case duplicate:
		return key, "", reasonLineDuplicate
	default:
		return key, value, ""
	}
}

// isForbiddenRune reports a rune no line may hold: a control character
// (U+0000 to U+001F, the carriage return and the tab included), DEL, or the
// byte order mark U+FEFF anywhere.
func isForbiddenRune(r rune) bool {
	return r < 0x20 || r == 0x7f || r == 0xfeff
}

// serverFileTemplate is the content config init writes: blank lines and
// comment lines only, one description and one commented-out "# <KEY>=" line
// for each of serverFileKeys. The keys and the default come from the
// constants, so the template cannot name a key the file does not accept.
const serverFileTemplate = `# yt2column-server configuration file.
#
# Each setting is one line of the form KEY=VALUE. The value is used exactly as
# written: it is not quoted, has no space around "=", and has no comment after
# it. A line starting with "#" is a comment. The server reads this file only
# at startup, and only while its mode is exactly 0600.
#
# To set a key, remove the "# " before it and write the value after "=".

# The LLM that writes and shortens the article, as <provider>/<model ID>,
# optionally followed by +<effort>; the provider is ` + providerDeepSeek + `, ` + providerClaude + `, or ` + providerOpenAI + `.
# When unset: ` + defaultLLMValue + `.
# ` + llmEnv + `=

# The LLM that reviews and polishes the article, in the same form.
# When unset: the same as ` + llmEnv + ` (its value or its default), effort
# included.
# ` + reviewLLMEnv + `=

# The DeepSeek API key, required when a step uses ` + providerDeepSeek + `.
# ` + deepSeekAPIKeyEnv + `=

# The Anthropic API key, required when a step uses ` + providerClaude + `.
# ` + anthropicAPIKeyEnv + `=

# The OpenAI API key, required when a step uses ` + providerOpenAI + `.
# ` + openAIAPIKeyEnv + `=

# The Anthropic workspace ID, for an API key not bound to a workspace.
# When unset, no workspace ID is sent.
# ` + anthropicWorkspaceIDEnv + `=
`

// InitResult is what InitServerFile created. LeftoverTemp is the name of the
// temporary file it could not remove after a success, or "".
type InitResult struct {
	Path         string
	LeftoverTemp string
}

// InitServerFile creates the configuration file's template. It derives the
// path from HOME alone (a *VarError naming HOME when it cannot), creates the
// missing directories below the base, and creates the file. Contract: it
// reads no variable but HOME, so the template never holds a value from the
// environment; what is already at the path is never changed
// (ErrConfigFileExists); and a *VarError is returned only for HOME, every
// other failure being a *FileError.
func InitServerFile(lookup LookupFunc) (InitResult, error) {
	loc, homeErr := serverFileLocationFor(runtime.GOOS, lookup)
	if homeErr != nil {
		return InitResult{}, homeErr
	}
	if err := privatefile.MkdirBelow(loc.base, loc.dirs); err != nil {
		return InitResult{}, createFileError(loc, err)
	}
	leftover, err := privatefile.Create(loc.path, []byte(serverFileTemplate))
	if err != nil {
		return InitResult{}, createFileError(loc, err)
	}
	return InitResult{Path: loc.path, LeftoverTemp: leftover}, nil
}
