// Package codexreview runs Codex CLI's local review once, with fixed
// arguments, and judges what it returns: whether the run completed, whether
// its final message has one of the accepted forms, where each finding points,
// and whether the run changed the repository's git state.
//
// It is a developer tool. Codex's output is untrusted: it is checked against a
// fixed grammar, never repaired, and no text of it reaches an error message,
// a Detail token, or anything other than the Title and Body of a Finding.
package codexreview

// Protocol is the version of the tool's input and output contract. The command
// documents pass it back with --protocol, so an installed binary that predates
// the documents stops instead of judging by older rules.
const Protocol = 1

// Kind is the result of one codexreview run. The zero value is not a valid kind.
type Kind string

// The kinds a run ends in.
const (
	KindFindings       Kind = "findings"         // the review completed and reported one or more findings
	KindNoFindings     Kind = "no_findings"      // the review completed and reported none
	KindStateChanged   Kind = "state_changed"    // the compared git state differs after the run; the caller stops
	KindUsage          Kind = "usage"            // the arguments are malformed or the protocol does not match
	KindPrecondition   Kind = "precondition"     // the repository is not in a state to run the review
	KindCodexNotFound  Kind = "codex_not_found"  // codex is not on PATH
	KindTimeout        Kind = "timeout"          // codex did not finish within the limit and was stopped
	KindCodexFailed    Kind = "codex_failed"     // codex exited unsuccessfully
	KindIncomplete     Kind = "incomplete"       // codex exited successfully without declaring a completed turn
	KindOutputTooLarge Kind = "output_too_large" // an output exceeded its size limit
	KindOutputEmpty    Kind = "output_empty"     // the final message is empty
	KindUnreadable     Kind = "unreadable"       // the final message matches neither accepted form
)

// Result is the JSON object codexreview writes to standard output.
type Result struct {
	Protocol            int       `json:"protocol"`
	Kind                Kind      `json:"kind"`
	Detail              string    `json:"detail"`               // a fixed token naming the cause within Kind, never codex's text
	Changed             []string  `json:"changed"`              // for KindStateChanged: which compared items differ
	ChangedRefs         []string  `json:"changed_refs"`         // for KindStateChanged: refs that differ, with the worktree each is checked out in
	CodexVersion        string    `json:"codex_version"`        // the output of codex --version, when it was read
	RunDir              string    `json:"run_dir"`              // where the final message is kept
	InstructionsChanged bool      `json:"instructions_changed"` // BASE..HEAD touches a file named AGENTS.md, AGENTS.override.md or CLAUDE.md
	Findings            []Finding `json:"findings"`             // for KindFindings only
}

// Finding is one finding parsed from the final message. Title and Body are
// codex's text: untrusted data, never to be copied into a record.
type Finding struct {
	Path    string `json:"path"` // relative to the worktree root, with forward slashes
	Start   int    `json:"start"`
	End     int    `json:"end"`
	InRange bool   `json:"in_range"` // the file changed in BASE..HEAD, exists, and holds the lines
	Title   string `json:"title"`
	Body    string `json:"body"`
}
