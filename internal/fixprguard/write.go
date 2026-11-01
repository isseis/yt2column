package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/strictjson"
)

// maxTextFileBytes caps a reply body, PR title or PR body file.
const maxTextFileBytes = 64 << 10

// readTextFile reads a file the session wrote for reply or edit-pr. Contract:
// only a regular file (not a symbolic link, FIFO or device) of at most
// maxTextFileBytes holding valid UTF-8 that is not blank and has no NUL is
// accepted; anything else is
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
	case strings.TrimSpace(string(data)) == "":
		return "", fmt.Errorf("%w: text file empty", ErrInvalidInput)
	case !utf8.Valid(data):
		return "", fmt.Errorf("%w: text file encoding", ErrInvalidInput)
	case strings.ContainsRune(string(data), 0):
		return "", fmt.Errorf("%w: text file has a NUL byte", ErrInvalidInput)
	}
	return string(data), nil
}

// readTitleFile reads a PR title file: a text file holding exactly one line
// with no line terminator and no control character (C0, DEL, C1, or a Unicode
// line or paragraph separator).
func readTitleFile(name string) (string, error) {
	title, err := readTextFile(name)
	if err != nil {
		return "", err
	}
	if strings.ContainsFunc(title, isControl) {
		return "", fmt.Errorf("%w: title is not one plain line", ErrInvalidInput)
	}
	return title, nil
}

// isControl reports a control character (C0, DEL, or C1) or a Unicode line
// or paragraph separator: a rune that would break or hide a line.
func isControl(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }

// checkPostText rejects text that would make Codex act when posted from the
// user's account. Contract: text containing CodexMention anywhere, with ASCII
// letters compared case-insensitively and no word boundary required (so
// "@codexbot" and "a@codex.example" are rejected too), is ErrInvalidInput;
// nothing else is. Non-ASCII look-alikes (a full-width at sign, a zero-width
// character inside the word) are not GitHub mentions and are not normalized.
// It checks content only, so it runs after readTextFile and before any call
// to GitHub.
func checkPostText(text string) error {
	folded := strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, text)
	if strings.Contains(folded, CodexMention) {
		return errCodexMention
	}
	return nil
}

// editorialCorrectionPrefix starts every line of a finish commit body: the
// line that records an editorial correction to an approved task document.
const editorialCorrectionPrefix = "Editorial correction: "

// checkCommitBody holds a finish commit body to the editorial correction
// lines. Contract: the body is one or more lines, each starting with
// editorialCorrectionPrefix followed by a non-blank description, with no
// empty line, no '@', and no control character (C0, DEL, C1, or a Unicode
// line or paragraph separator) other than the LF ending a line; it may end
// with one LF and no more. Anything else is ErrInvalidInput, so the body
// cannot carry a line of another kind, such as a review record line or a
// trailer. It checks content only, so it runs after readTextFile.
func checkCommitBody(body string) error {
	for line := range strings.SplitSeq(strings.TrimSuffix(body, "\n"), "\n") {
		description, ok := strings.CutPrefix(line, editorialCorrectionPrefix)
		switch {
		case !ok:
			return fmt.Errorf("%w: commit body line is not an editorial correction", ErrInvalidInput)
		case strings.TrimSpace(description) == "":
			return fmt.Errorf("%w: commit body line has no description", ErrInvalidInput)
		case strings.ContainsFunc(line, func(r rune) bool { return unicode.IsControl(r) || r == ' ' || r == ' ' }):
			return fmt.Errorf("%w: commit body has a control character", ErrInvalidInput)
		case strings.Contains(line, "@"):
			return fmt.Errorf("%w: commit body has '@'", ErrInvalidInput)
		}
	}
	return nil
}

// resolveMutation resolves one review thread; the thread ID is a variable, never
// part of the query text.
const resolveMutation = `mutation($threadId: ID!) {
  resolveReviewThread(input: {threadId: $threadId}) {
    thread { id isResolved }
  }
}`

// ReplyRequest is the input of reply. Fingerprint is the one begin printed for
// the thread; Accept repeats begin's --accept pairs, so a named untrusted thread
// is answered only while it still has the fingerprint the user saw.
type ReplyRequest struct {
	PR          PRNumber
	ThreadID    string
	CommentID   int64
	Fingerprint Fingerprint
	BodyFile    string
	Accept      []Accept
}

// Reply posts the body file as a reply to the comment and then resolves the
// thread. Contract: it writes only when, judged afresh now, the thread is
// trusted or named with its current fingerprint (else ErrThreadNotTrusted), its
// current fingerprint is req.Fingerprint (else ErrThreadChanged), and the
// comment is the thread's first one, the only one GitHub's replies endpoint
// takes (else ErrCommentNotInThread). The body travels as a JSON request on
// standard input, never through a shell, a flag value or a temporary file, so
// GitHub receives the file's text exactly. The thread is resolved only after
// the reply succeeded and a fresh fetch shows no comment beside the judged ones
// and the reply, so a comment that arrived meanwhile is not hidden in a
// resolved thread. Once the reply may have been posted, every failure is
// ErrReplyNotResolved, so the caller does not post it again. A body that
// mentions Codex (checkPostText) is rejected before any call to GitHub.
func (g *GitHub) Reply(ctx context.Context, req ReplyRequest) error {
	if err := g.requireReviewer(); err != nil {
		return err
	}
	accepted, err := checkAccepts(req.Accept)
	if err != nil {
		return err
	}
	if !req.PR.usable() || !validThreadID(req.ThreadID) || req.CommentID <= 0 || !validFingerprint(req.Fingerprint) {
		return fmt.Errorf("%w: reply arguments", ErrInvalidInput)
	}
	body, err := readTextFile(req.BodyFile)
	if err != nil {
		return err
	}
	if err := checkPostText(body); err != nil {
		return err
	}
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return err
	}
	threads, err := g.unresolvedThreads(ctx, repo, req.PR)
	if err != nil {
		return err
	}
	// A thread not found is the zero thread, which has no comments.
	t := threads[req.ThreadID]
	if len(t.comments) == 0 {
		return ErrThreadChanged
	}
	current := fingerprint(t)
	assessment := assessThread(ctx, authorsOf(t), !t.moreComments, g.reviewers, newPermissionLookups(g, repo).lookup)
	if !assessment.Trusted {
		acc, named := accepted[t.id]
		if !named {
			return ErrThreadNotTrusted
		}
		if acc.Fingerprint != current {
			return ErrThreadChanged
		}
	}
	if req.Fingerprint != current {
		return ErrThreadChanged
	}
	if t.comments[0].databaseID != req.CommentID {
		return ErrCommentNotInThread
	}
	replyID, err := g.postReply(ctx, repo, req.PR, req.CommentID, body)
	if err != nil {
		return err
	}
	if err := g.checkOnlyReplyAdded(ctx, repo, req.PR, t, replyID); err != nil {
		return fmt.Errorf("%w: %w", ErrReplyNotResolved, err)
	}
	if err := g.resolveThread(ctx, t.id); err != nil {
		return fmt.Errorf("%w: %w", ErrReplyNotResolved, err)
	}
	return nil
}

// postReply posts body and returns the new comment's database ID. A failure
// GitHub answered is ErrResponseRejected (nothing was posted); a timeout, a
// cancellation, or an unreadable success response is ErrReplyNotResolved, since
// the reply may exist.
func (g *GitHub) postReply(ctx context.Context, repo repository, pr PRNumber, commentID int64, body string) (int64, error) {
	request, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return 0, fmt.Errorf("encode reply: %w", err)
	}
	endpoint := "repos/" + repo.owner + "/" + repo.name + "/pulls/" + pr.String() + "/comments/" + strconv.FormatInt(commentID, 10) + "/replies"
	out, err := g.runGH(ctx, request, "api", "--method", "POST", endpoint, "--input", "-")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return 0, fmt.Errorf("%w: %w", ErrReplyNotResolved, ghFailure("reply", err))
		}
		return 0, ghFailure("reply", err)
	}
	id, err := postedCommentID(out)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrReplyNotResolved, err)
	}
	return id, nil
}

// postedCommentID reads the id of the comment the replies endpoint created.
func postedCommentID(out []byte) (int64, error) {
	if len(out) > maxResponseBytes {
		return 0, rejected("response size")
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return 0, rejected("reply response")
	}
	members, err := obj.Collect("id")
	if err != nil {
		return 0, rejected("reply id")
	}
	value, err := strictjson.Required(members, "id")
	if err != nil {
		return 0, rejected("reply id")
	}
	id, err := value.AsNonNegativeInt64()
	if err != nil || id == 0 {
		return 0, rejected("reply id")
	}
	return id, nil
}

// checkOnlyReplyAdded fetches the thread again and confirms it is still
// unresolved and holds exactly the judged comments followed by the reply.
// Another comment, edit or deletion is ErrThreadChanged.
func (g *GitHub) checkOnlyReplyAdded(ctx context.Context, repo repository, pr PRNumber, judged thread, replyID int64) error {
	threads, err := g.unresolvedThreads(ctx, repo, pr)
	if err != nil {
		return err
	}
	// A thread not found is the zero thread, which has no comments.
	now := threads[judged.id]
	if len(now.comments) != len(judged.comments)+1 || now.comments[len(now.comments)-1].databaseID != replyID {
		return ErrThreadChanged
	}
	before := now
	before.comments = now.comments[:len(judged.comments)]
	if fingerprint(before) != fingerprint(judged) {
		return ErrThreadChanged
	}
	return nil
}

// resolveThread resolves the thread and confirms from the response that it is
// the thread named and that it is now resolved.
func (g *GitHub) resolveThread(ctx context.Context, threadID string) error {
	out, err := g.graphql(ctx, resolveMutation, map[string]any{"threadId": threadID})
	if err != nil {
		return err
	}
	data, err := graphqlData(out)
	if err != nil {
		return err
	}
	resolved, err := memberObject(data, "resolveReviewThread")
	if err != nil {
		return err
	}
	th, err := memberObject(resolved, "thread")
	if err != nil {
		return err
	}
	members, err := th.Collect("id")
	if err != nil {
		return rejected("thread id")
	}
	if id, err := strictjson.RequiredString(members, "id"); err != nil || id != threadID {
		return rejected("thread id")
	}
	isResolved, err := memberBool(th, "isResolved")
	if err != nil {
		return err
	}
	if !isResolved {
		return rejected("thread not resolved")
	}
	return nil
}

// EditPRRequest is the input of edit-pr.
type EditPRRequest struct {
	PR        PRNumber
	TitleFile string
	BodyFile  string
}

// EditPR sets the PR's title and body from the two files, without a shell. The
// title is passed as --title=<value>, so a title starting with '-' is not read
// as a flag, and the body is passed on standard input (--body-file -), so gh
// reads the text that was checked, not the file again. A title or body that
// mentions Codex (checkPostText) is rejected before gh runs.
func (g *GitHub) EditPR(ctx context.Context, req EditPRRequest) error {
	if !req.PR.usable() {
		return fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	title, err := readTitleFile(req.TitleFile)
	if err != nil {
		return err
	}
	body, err := readTextFile(req.BodyFile)
	if err != nil {
		return err
	}
	if err := checkPostText(title); err != nil {
		return err
	}
	if err := checkPostText(body); err != nil {
		return err
	}
	if _, err := g.runGH(ctx, []byte(body), "pr", "edit", req.PR.String(), "--title="+title, "--body-file", "-"); err != nil {
		return ghFailure("pr edit", err)
	}
	return nil
}

// AppendPRRequest names the PR whose description gets the file's content
// appended after its current body.
type AppendPRRequest struct {
	PR       PRNumber
	BodyFile string
}

// AppendPR appends the file's text to the PR description. Contract: the file
// passes readTextFile and checkPostText and holds no '@' or '<' (no mention,
// notification, or HTML comment that hides text), all checked before gh
// runs. The current body is read with gh pr view and kept byte for byte; the
// text follows it after an empty line (directly when the body is empty). The
// result must fit maxTextFileBytes and pass checkPostText, a current body
// that already mentions Codex included; otherwise it is ErrInvalidInput and
// nothing is written. The result is written as the body alone, on standard
// input, with no title. The current body never reaches an error or the
// output. A reviewer's edit between the read and the write is overwritten:
// gh pr edit has no conditional update.
func (g *GitHub) AppendPR(ctx context.Context, req AppendPRRequest) error {
	if !req.PR.usable() {
		return fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	text, err := readTextFile(req.BodyFile)
	if err != nil {
		return err
	}
	if err := checkPostText(text); err != nil {
		return err
	}
	if strings.ContainsAny(text, "@<") {
		return fmt.Errorf("%w: appended text has '@' or '<'", ErrInvalidInput)
	}
	current, err := g.prBody(ctx, req.PR)
	if err != nil {
		return err
	}
	body := appendParagraph(current, text)
	if len(body) > maxTextFileBytes {
		return fmt.Errorf("%w: appended PR body size", ErrInvalidInput)
	}
	if err := checkPostText(body); err != nil {
		return err
	}
	if _, err := g.runGH(ctx, []byte(body), "pr", "edit", req.PR.String(), "--body-file", "-"); err != nil {
		return ghFailure("pr edit", err)
	}
	return nil
}

// appendParagraph puts text after current with an empty line between them,
// leaving current's bytes as they are: one LF is added when current ends a
// line (LF or CRLF), two when it does not, and none when it is empty.
func appendParagraph(current, text string) string {
	switch {
	case current == "":
		return text
	case strings.HasSuffix(current, "\n"):
		return current + "\n" + text
	default:
		return current + "\n\n" + text
	}
}

// prBody reads the PR's current description. The response must be one
// strict JSON object, within maxResponseBytes, whose body member is a string
// (an empty one included); anything else, and a failed gh, is
// ErrResponseRejected naming the rule only.
func (g *GitHub) prBody(ctx context.Context, pr PRNumber) (string, error) {
	out, err := g.runGH(ctx, nil, "pr", "view", pr.String(), "--json", "body")
	if err != nil {
		return "", ghFailure("pr view", err)
	}
	if len(out) > maxResponseBytes {
		return "", rejected("response size")
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return "", rejected("pr view")
	}
	members, err := obj.Collect("body")
	if err != nil {
		return "", rejected("pr view")
	}
	value, err := strictjson.Required(members, "body")
	if err != nil {
		return "", rejected("PR body")
	}
	body, err := value.AsString()
	if err != nil {
		return "", rejected("PR body")
	}
	return body, nil
}
