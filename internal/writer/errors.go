package writer

import "errors"

// Sentinel errors returned by New and Write. They are wrapped with context;
// callers identify a failure with errors.Is.
var (
	ErrInvalidTemplate   = errors.New("invalid prompt template")
	ErrInvalidTranscript = errors.New("invalid transcript")
	ErrMalformedOutput   = errors.New("malformed LLM output")
)

var (
	// errNilLLMClient rejects a nil or typed-nil LLM client in New. It is not
	// exported: a missing dependency is a programming error, not a condition
	// callers branch on.
	errNilLLMClient = errors.New("LLM client is nil")
	// errNotRegularFile is joined with ErrInvalidTemplate when an override
	// path names a directory, FIFO, device, or socket.
	errNotRegularFile = errors.New("not a regular file")
	// errTemplateDefinition and errDisallowedSyntax are joined with
	// ErrInvalidTemplate when a template uses {{define}} or {{block}}, or
	// syntax outside the allowlist, so tests can tell which check rejected
	// a template.
	errTemplateDefinition = errors.New("define and block are not allowed")
	errDisallowedSyntax   = errors.New("disallowed syntax")
)
