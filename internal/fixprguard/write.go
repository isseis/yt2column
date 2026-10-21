package fixprguard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/strictjson"
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
// comment belongs to it (else ErrCommentNotInThread). The body travels as a
// JSON request on standard input, never through a shell, a flag value or a
// temporary file, so GitHub receives the file's text exactly. The thread is
// resolved only after the reply succeeded; a failed resolve is
// ErrReplyNotResolved.
func (g *GitHub) Reply(ctx context.Context, req ReplyRequest) error {
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
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return err
	}
	threads, err := g.unresolvedThreads(ctx, repo, req.PR)
	if err != nil {
		return err
	}
	t, ok := threads[req.ThreadID]
	if !ok || len(t.comments) == 0 {
		return ErrThreadChanged
	}
	current := fingerprint(t)
	assessment := assessThread(ctx, authorsOf(t), !t.moreComments, g.reviewer, newPermissionLookups(g, repo).lookup)
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
	if !slices.ContainsFunc(t.comments, func(c comment) bool { return c.databaseID == req.CommentID }) {
		return ErrCommentNotInThread
	}
	if err := g.postReply(ctx, repo, req.PR, req.CommentID, body); err != nil {
		return err
	}
	if err := g.resolveThread(ctx, t.id); err != nil {
		return fmt.Errorf("%w: %w", ErrReplyNotResolved, err)
	}
	return nil
}

func (g *GitHub) postReply(ctx context.Context, repo repository, pr PRNumber, commentID int64, body string) error {
	request, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("encode reply: %w", err)
	}
	endpoint := "repos/" + repo.owner + "/" + repo.name + "/pulls/" + pr.String() + "/comments/" + strconv.FormatInt(commentID, 10) + "/replies"
	if _, err := g.runGH(ctx, request, "api", "--method", "POST", endpoint, "--input", "-"); err != nil {
		return ghFailure("reply", err)
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
// reads the text that was checked, not the file again.
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
	if _, err := g.runGH(ctx, []byte(body), "pr", "edit", req.PR.String(), "--title="+title, "--body-file", "-"); err != nil {
		return ghFailure("pr edit", err)
	}
	return nil
}
