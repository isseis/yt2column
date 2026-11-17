package numreserve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/tasknaming"
)

// ghNotAuthenticated is gh's exit status when it has no credentials; it then
// prints no status line.
const ghNotAuthenticated = 4

// Reservation is a number the tool decided and the record tag it created.
type Reservation struct {
	Number tasknaming.Number
	// Tag is the record tag's full ref name.
	Tag string
	// Commit is the origin/main commit the tag points at.
	Commit string
}

// Reserve decides a number in space and creates its record tag.
//
// Contract: it writes at most one ref on GitHub, the record tag of the number
// it returns; a creation that GitHub refused with 422 for a ref that reading
// back shows to exist writes nothing. It returns a number only after GitHub
// answered 201 for its tag, and every other outcome is an error, a
// *StopError for every expected one. The whole run is bounded by the
// per-run time limit.
func (t *Tool) Reserve(ctx context.Context, space Space) (Reservation, error) {
	if t.refused != nil {
		return Reservation{}, t.refused
	}
	tagPrefix, err := space.tagPrefix()
	if err != nil {
		return Reservation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.bounds.runTimeout)
	defer cancel()
	c, err := t.collect(ctx, space, tagPrefix)
	if err != nil {
		return Reservation{}, err
	}
	return t.create(ctx, tagPrefix, c)
}

// create tries candidates from the largest number in use plus one.
//
// Contract: the jump check comes before the four-digit check, so a stray
// branch numbered 9999 stops as jump and names the branch; the attempt check
// comes last, so the maxPosts-th creation is still tried.
func (t *Tool) create(ctx context.Context, tagPrefix string, c collected) (Reservation, error) {
	candidate := c.largest() + 1
	for posts := 0; ; posts++ {
		step := fmt.Sprintf("choosing a number after %d creations", posts)
		if candidate > c.base+t.bounds.jumpWidth {
			return Reservation{}, jumpStop(step, candidate, c, t.bounds.jumpWidth)
		}
		if candidate > maxNumber {
			return Reservation{}, stop(StopExhausted, step, "candidate %d is above %04d", candidate, maxNumber)
		}
		if posts == t.bounds.maxPosts {
			return Reservation{}, stop(StopAttempts, step, "%d creations were all taken by other runs", posts)
		}
		n, err := tasknaming.ParseNumber(fmt.Sprintf("%04d", candidate))
		if err != nil {
			return Reservation{}, err
		}
		tag := tagPrefix + n.String()
		created, err := t.createTag(ctx, tag, c.mainSHA)
		if err != nil {
			return Reservation{}, err
		}
		if created {
			return Reservation{Number: n, Tag: tag, Commit: c.mainSHA}, nil
		}
		candidate++
	}
}

// jumpStop names every input above the base, which is what moved the
// candidate past the jump width.
func jumpStop(step string, candidate int, c collected, width int) *StopError {
	var above []string
	for _, u := range c.others {
		if u.number > c.base {
			above = append(above, fmt.Sprintf("%s (%04d)", u.where, u.number))
		}
	}
	detail := fmt.Sprintf("candidate %04d is more than %d above the base %04d", candidate, width, c.base)
	if len(above) == 0 {
		return stop(StopJump, step, "%s; no branch or work-tree input is above the base", detail)
	}
	return stop(StopJump, step, "%s; above the base: %s", detail, listed(above))
}

// createTag creates tag at sha with POST git/refs and reports whether it was
// created (true) or GitHub confirmed a ref of that name exists (false).
// Every other outcome is a stop (architecture 3.3.2).
func (t *Tool) createTag(ctx context.Context, tag, sha string) (bool, error) {
	step := "creating " + tag
	out, code, err := t.ghAPI(ctx, step, "-X", "POST", "repos/"+Repository+"/git/refs", "-f", "ref="+tag, "-f", "sha="+sha)
	if err != nil {
		return false, err
	}
	status, ok := statusCode(out)
	if !ok {
		if code == ghNotAuthenticated {
			return false, stop(StopPermission, step, "gh exited with status %d and no HTTP status line (not authenticated)", code)
		}
		return false, stop(StopNetworkOrGitHub, step, "gh exited with status %d and no accepted HTTP status line", code)
	}
	switch status {
	case http.StatusCreated:
		return true, nil
	case http.StatusUnprocessableEntity:
		return false, t.confirmTaken(ctx, tag)
	case http.StatusTooManyRequests:
		return false, stop(StopNetworkOrGitHub, step, "HTTP %d (rate limit)", status)
	case http.StatusForbidden:
		limited, readable := rateLimited(out)
		if !readable {
			return false, stop(StopNetworkOrGitHub, step, "HTTP %d with headers of an unaccepted form", status)
		}
		if limited {
			return false, stop(StopNetworkOrGitHub, step, "HTTP %d (rate limit)", status)
		}
		return false, stop(StopPermission, step, "HTTP %d", status)
	case http.StatusUnauthorized, http.StatusNotFound:
		return false, stop(StopPermission, step, "HTTP %d", status)
	default:
		return false, stop(StopNetworkOrGitHub, step, "HTTP %d", status)
	}
}

// confirmTaken reads tag back after a 422 and returns nil only when GitHub
// answers 200 with a ref of exactly that name.
func (t *Tool) confirmTaken(ctx context.Context, tag string) error {
	step := "reading back " + tag + " after HTTP 422"
	out, code, err := t.ghAPI(ctx, step, "repos/"+Repository+"/git/ref/"+strings.TrimPrefix(tag, "refs/"))
	if err != nil {
		return err
	}
	status, ok := statusCode(out)
	if !ok {
		return stop(StopNetworkOrGitHub, step, "gh exited with status %d and no accepted HTTP status line", code)
	}
	if status != http.StatusOK {
		return stop(StopNetworkOrGitHub, step, "HTTP %d, so the ref is not confirmed to exist", status)
	}
	if !refMatches(out, tag) {
		return stop(StopMalformed, step, "HTTP 200 without a ref equal to the name asked for")
	}
	return nil
}

// ghAPI runs gh api -i and returns its standard output and exit status. A
// command that exits unsuccessfully still has its output read, because gh
// prints the status line of a failed request. A time-out, an exceeded output
// cap, or a gh that cannot start is a stop.
func (t *Tool) ghAPI(ctx context.Context, step string, args ...string) ([]byte, int, error) {
	out, err := t.runGH(ctx, args...)
	if err == nil {
		return out, 0, nil
	}
	switch {
	case isTimeout(err):
		return nil, 0, stop(StopNetworkOrGitHub, step, "gh did not finish in time")
	case errors.Is(err, cmdrunner.ErrOutputTooLarge):
		return nil, 0, stop(StopNetworkOrGitHub, step, "gh output exceeds the cap")
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return nil, 0, stop(StopNetworkOrGitHub, step, "gh could not be run")
	}
	return exitErr.Stdout, exitErr.Code, nil
}

// statusLine is the accepted form of gh api -i's first line, which ends in
// LF: the protocol, the three-digit code, and the reason phrase.
var statusLine = regexp.MustCompile(`^HTTP/(?:1\.1|2|2\.0) ([0-9]{3})(?: [^\r\n]*)?$`)

// statusCode reads the status code from the first line of gh api -i output.
func statusCode(out []byte) (int, bool) {
	first, _, found := bytes.Cut(out, []byte{'\n'})
	if !found {
		return 0, false
	}
	m := statusLine.FindSubmatch(first)
	if m == nil {
		return 0, false
	}
	code, err := strconv.Atoi(string(m[1]))
	return code, err == nil
}

// splitResponse returns the header lines and the body of gh api -i output.
// The header lines and the blank line that ends them end in CRLF; a header
// block without that blank line, or a header line without a name, is not
// accepted.
func splitResponse(out []byte) ([]string, []byte, bool) {
	_, rest, found := bytes.Cut(out, []byte{'\n'})
	if !found {
		return nil, nil, false
	}
	var headers []string
	for {
		line, after, found := bytes.Cut(rest, []byte("\r\n"))
		if !found {
			return nil, nil, false
		}
		if len(line) == 0 {
			return headers, after, true
		}
		name, _, hasColon := strings.Cut(string(line), ":")
		if !hasColon || name == "" || strings.ContainsAny(name, " \t\n") {
			return nil, nil, false
		}
		headers = append(headers, string(line))
		rest = after
	}
}

// rateLimited reports whether the headers mark a rate limit:
// X-RateLimit-Remaining of 0 or a Retry-After header, names compared without
// letter case. The second result is false when the headers cannot be read.
func rateLimited(out []byte) (limited, readable bool) {
	headers, _, ok := splitResponse(out)
	if !ok {
		return false, false
	}
	for _, h := range headers {
		name, value, _ := strings.Cut(h, ":")
		value = strings.TrimSpace(value)
		switch {
		case strings.EqualFold(name, "Retry-After"):
			return true, true
		case strings.EqualFold(name, "X-RateLimit-Remaining") && value == "0":
			return true, true
		}
	}
	return false, true
}

// refMatches reports whether the body of a GET git/ref response is one JSON
// object whose ref member is exactly tag.
func refMatches(out []byte, tag string) bool {
	_, body, ok := splitResponse(out)
	if !ok {
		return false
	}
	object, err := strictjson.ParseObject(body)
	if err != nil {
		return false
	}
	members, err := object.CollectExactCase("ref")
	if err != nil {
		return false
	}
	ref, err := strictjson.RequiredString(members, "ref")
	return err == nil && ref == tag
}
