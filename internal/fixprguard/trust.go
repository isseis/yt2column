package fixprguard

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	// maxPermissionLookups caps the permission lookups of one call, so many
	// accounts commenting many times cannot exhaust the run time or the API
	// quota. Lookups past the cap become reason (d).
	maxPermissionLookups = 100

	httpProtoPrefix  = "HTTP/"
	statusCodeDigits = 3
)

// trustedRoles are the role_name values that grant write access. Any other
// value, including a custom role and an unknown one, is not trusted.
var trustedRoles = []string{"write", "maintain", "admin"}

// author is everything the trust judgment sees of a comment: its author and
// its URL. It has no body field, so the judgment cannot depend on what a
// comment says.
type author struct {
	present    bool
	kind       AuthorKind
	login      Login
	commentURL string
}

// authorsOf drops everything but the authors from a thread's comments.
func authorsOf(t thread) []author {
	authors := make([]author, 0, len(t.comments))
	for _, c := range t.comments {
		a := author{present: c.authorPresent, kind: authorKindOf(c.kindRaw), commentURL: c.url}
		if login, err := ParseLogin(c.loginRaw); err == nil {
			a.login = login
		}
		authors = append(authors, a)
	}
	return authors
}

// permission is the outcome of one permission lookup: trusted, not trusted
// (cause none), or unavailable (cause set).
type permission struct {
	trusted bool
	cause   Cause
}

// permissionFunc looks up the repository permission of a login.
type permissionFunc func(ctx context.Context, login Login) permission

// assessThread judges a thread from its authors alone. The thread is trusted
// only when it is complete and every author is the reviewer bot or a user with
// write permission; otherwise every applicable reason is collected, in comment
// order, without duplicates.
func assessThread(ctx context.Context, authors []author, complete bool, reviewer Login, lookup permissionFunc) Assessment {
	var findings []Finding
	add := func(f Finding) {
		if !slices.Contains(findings, f) {
			findings = append(findings, f)
		}
	}
	if !complete || len(authors) == 0 {
		add(Finding{Reason: ReasonIncompleteComments})
	}
	for _, a := range authors {
		if !a.present {
			add(Finding{Reason: ReasonNoAuthorName, CommentURL: a.commentURL})
			continue
		}
		if !a.login.usable() {
			add(Finding{Reason: ReasonNoAuthorName, CommentURL: a.commentURL})
		}
		switch a.kind {
		case AuthorBot:
			if a.login.usable() && a.login != reviewer {
				add(Finding{Reason: ReasonOtherBot, Account: a.login})
			}
		case AuthorUser:
			if !a.login.usable() {
				continue
			}
			switch p := lookup(ctx, a.login); {
			case p.cause != CauseNone:
				add(Finding{Reason: ReasonPermissionUnavailable, Cause: p.cause})
			case !p.trusted:
				add(Finding{Reason: ReasonNoWritePermission, Account: a.login})
			}
		default:
			add(Finding{Reason: ReasonUnknownAuthorKind})
		}
	}
	return Assessment{Trusted: len(findings) == 0, Findings: findings}
}

// permissionLookups answers permission lookups for one call: one REST lookup
// per distinct login, at most maxPermissionLookups in all, in the order asked.
type permissionLookups struct {
	g       *GitHub
	repo    repository
	results map[Login]permission
	used    int
}

func newPermissionLookups(g *GitHub, repo repository) *permissionLookups {
	return &permissionLookups{g: g, repo: repo, results: map[Login]permission{}}
}

func (p *permissionLookups) lookup(ctx context.Context, login Login) permission {
	if result, ok := p.results[login]; ok {
		return result
	}
	if p.used >= maxPermissionLookups {
		return permission{cause: CauseBudgetExceeded}
	}
	p.used++
	result := p.g.queryPermission(ctx, p.repo, login)
	p.results[login] = result
	return result
}

func (p *permissionLookups) budget() LookupBudget {
	return LookupBudget{Used: p.used, Remaining: maxPermissionLookups - p.used}
}

// queryPermission runs the REST permission lookup with --include, so the HTTP
// status of a failure can be mapped to a Cause without reading gh's error text.
// Only role_name is read; other members are ignored because GitHub adds fields.
func (g *GitHub) queryPermission(ctx context.Context, repo repository, login Login) permission {
	out, err := g.runGH(ctx, nil, "api", "--include",
		"repos/"+repo.owner+"/"+repo.name+"/collaborators/"+login.String()+"/permission")
	if err != nil {
		return permission{cause: failureCause(err)}
	}
	if len(out) > maxResponseBytes {
		return permission{cause: CauseRejectedResponse}
	}
	status, body, ok := splitIncluded(out)
	if !ok || status != http.StatusOK {
		return permission{cause: CauseRejectedResponse}
	}
	obj, err := strictjson.ParseObject(body)
	if err != nil {
		return permission{cause: CauseRejectedResponse}
	}
	members, err := obj.Collect("role_name")
	if err != nil {
		return permission{cause: CauseRejectedResponse}
	}
	value, err := strictjson.Required(members, "role_name")
	if err != nil {
		return permission{cause: CauseRejectedResponse}
	}
	role, err := value.AsString()
	if err != nil {
		return permission{cause: CauseRejectedResponse}
	}
	return permission{trusted: slices.Contains(trustedRoles, role)}
}

// failureCause maps a failed lookup to a Cause from the HTTP status gh printed
// with --include, never from the error text.
func failureCause(err error) Cause {
	if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
		return CauseRejectedResponse
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return CauseFailed
	}
	status, _, ok := splitIncluded(exitErr.Stdout)
	if !ok {
		return CauseFailed
	}
	switch status {
	case http.StatusNotFound:
		return CauseNotFound
	case http.StatusForbidden:
		return CauseForbidden
	case http.StatusTooManyRequests:
		return CauseRateLimited
	default:
		return CauseFailed
	}
}

// splitIncluded splits the output of gh api --include into the status code of
// its first line ("HTTP/2.0 404 Not Found") and the body after the first empty
// line. ok is false for any other shape.
func splitIncluded(out []byte) (status int, body []byte, ok bool) {
	first, rest, found := strings.Cut(string(out), "\n")
	if !found {
		return 0, nil, false
	}
	fields := strings.Fields(strings.TrimSuffix(first, "\r"))
	if len(fields) < 2 || !strings.HasPrefix(fields[0], httpProtoPrefix) || len(fields[1]) != statusCodeDigits {
		return 0, nil, false
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, nil, false
	}
	for {
		line, after, found := strings.Cut(rest, "\n")
		if !found {
			return 0, nil, false
		}
		rest = after
		if strings.TrimSuffix(line, "\r") == "" {
			return code, []byte(rest), true
		}
	}
}
