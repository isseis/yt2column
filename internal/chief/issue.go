package chief

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// IssueStatus is what GitHub says an issue number is. The zero value is
// IssueUnreadable: a value that was not read, or whose answer was not one of
// the accepted shapes, is never taken for any of the others.
type IssueStatus int

const (
	// IssueUnreadable is no accepted answer.
	IssueUnreadable IssueStatus = iota
	// IssueOpen is an open issue with no draft label: a task may start on it.
	IssueOpen
	// IssueDraft is an open issue with the label `draft`: it is not ready
	// for /mkreq.
	IssueDraft
	// IssueClosed is a closed issue, whatever its labels.
	IssueClosed
	// IssuePullRequest is a pull request, not an issue.
	IssuePullRequest
	// IssueNotFound is a number that names neither an issue nor a pull
	// request.
	IssueNotFound
)

// Issue is what ReadIssue read for one number.
type Issue struct {
	Status IssueStatus
	// Number is the number that was asked for.
	Number paneinput.IssueNumber
	// Title is the issue's title with the characters that could rewrite the
	// terminal made visible (Sanitize) and cut at issueTitleMax characters. It
	// is set for an issue (open, draft, closed) only, and decides nothing: no
	// path, name, or input is made from it.
	Title string
	// TitleCut reports that the title was cut.
	TitleCut bool
}

// The limits of the issue read. The response is a small, fixed shape; a
// larger one is not it. The title limit keeps it on one line of the display.
const (
	issueResponseMax = 64 << 10
	issueTitleMax    = 200
)

// draftLabel is the name of the label that marks an issue as a draft, compared
// byte for byte.
const draftLabel = "draft"

// issueQuery is the one question chief asks GitHub about a number. It is a
// query, never a mutation, and asks for an issue's labels so a draft can be
// told from a final issue.
const issueQuery = `query($owner: String!, $repo: String!, $n: Int!) {
  repository(owner: $owner, name: $repo) {
    issueOrPullRequest(number: $n) {
      __typename
      ... on Issue { number state title labels(first: 100) { totalCount nodes { name } } }
      ... on PullRequest { number }
    }
  }
}`

// issueArgs returns the arguments of the gh call that asks issueQuery about
// number. gh fills {owner} and {repo} from the remote of the directory it runs
// in, and -F passes n as a number.
func issueArgs(number paneinput.IssueNumber) []string {
	return []string{"api", "graphql", "-F", "owner={owner}", "-F", "repo={repo}", "-F", "n=" + number.String(), "-f", "query=" + issueQuery}
}

// errNoIssueNumber reports an issue read asked for with the zero number.
var errNoIssueNumber = errors.New("no issue number")

// ReadIssue asks GitHub what number is, with gh run in dir (chief's own
// worktree, so the repository is its remote's). Contract: the answer is one of
// three accepted shapes (an issue, a pull request, or the single NOT_FOUND
// error of a number that exists nowhere) or it is an error that wraps
// errGHOutput; every other answer, a failure of gh included, is an error and
// never a status. The caller refuses to make anything on an error.
func (g GitReader) ReadIssue(ctx context.Context, dir string, number paneinput.IssueNumber) (Issue, error) {
	if number.String() == "" {
		return Issue{}, errNoIssueNumber
	}
	out, err := g.cmds.gh(ctx, dir, issueArgs(number)...)
	exit := 0
	if err != nil {
		exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
		if !ok || exitErr.Code != 1 {
			return Issue{}, err
		}
		out, exit = exitErr.Stdout, 1
	}
	return parseIssue(out, exit, number)
}

// parseIssue reads the answer of issueQuery for number, which gh wrote with
// exit status exit.
func parseIssue(out []byte, exit int, number paneinput.IssueNumber) (Issue, error) {
	if len(out) > issueResponseMax {
		return Issue{}, fmt.Errorf("%w: the answer is larger than %d bytes", errGHOutput, issueResponseMax)
	}
	top, err := parseObjectMembers(out, "data", "errors")
	if err != nil {
		return Issue{}, err
	}
	data, err := requiredObjectMember(top, "data")
	if err != nil {
		return Issue{}, err
	}
	repository, err := objectMember(data, "repository")
	if err != nil {
		return Issue{}, err
	}
	switch exit {
	case 0:
		if _, has := top["errors"]; has {
			return Issue{}, fmt.Errorf("%w: the answer holds errors", errGHOutput)
		}
		return parseIssueNode(repository, number)
	case 1:
		return parseIssueNotFound(top, repository)
	default:
		return Issue{}, fmt.Errorf("%w: gh exited with %d", errGHOutput, exit)
	}
}

// parseObjectMembers reads data as one object and returns the members named by
// keys, rejecting a member whose name differs from a key only in letter case.
func parseObjectMembers(data []byte, keys ...string) (map[string]strictjson.Value, error) {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	members, err := object.CollectExactCase(keys...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	return members, nil
}

// requiredObjectMember returns the named member of members as an object.
func requiredObjectMember(members map[string]strictjson.Value, key string) (strictjson.Object, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return strictjson.Object{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	object, err := value.AsObject()
	if err != nil {
		return strictjson.Object{}, fmt.Errorf("%w: %s: %w", errGHOutput, key, err)
	}
	return object, nil
}

// objectMember reads the one member key of object, with letter-case variants
// rejected, as an object.
func objectMember(object strictjson.Object, key string) (strictjson.Object, error) {
	members, err := object.CollectExactCase(key)
	if err != nil {
		return strictjson.Object{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	return requiredObjectMember(members, key)
}

// parseIssueNode reads the issueOrPullRequest member of repository for an
// answer that gh wrote with exit status 0.
func parseIssueNode(repository strictjson.Object, number paneinput.IssueNumber) (Issue, error) {
	repoMembers, err := repository.CollectExactCase("issueOrPullRequest")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	node, err := requiredObjectMember(repoMembers, "issueOrPullRequest")
	if err != nil {
		return Issue{}, err
	}
	fields, err := node.CollectExactCase("__typename", "number", "state", "title", "labels")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	typename, err := strictjson.RequiredString(fields, "__typename")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	if err := requireNumber(fields, number); err != nil {
		return Issue{}, err
	}
	switch typename {
	case "PullRequest":
		return Issue{Status: IssuePullRequest, Number: number}, nil
	case "Issue":
		return parseIssueFields(fields, number)
	default:
		return Issue{}, fmt.Errorf("%w: an answer of another kind", errGHOutput)
	}
}

// requireNumber checks that the answer is about the number that was asked
// for.
func requireNumber(fields map[string]strictjson.Value, number paneinput.IssueNumber) error {
	value, err := strictjson.Required(fields, "number")
	if err != nil {
		return fmt.Errorf("%w: %w", errGHOutput, err)
	}
	got, err := value.AsNonNegativeInt64()
	if err != nil || fmt.Sprint(got) != number.String() {
		return fmt.Errorf("%w: the answer is about another number", errGHOutput)
	}
	return nil
}

// parseIssueFields reads the members of an Issue node.
func parseIssueFields(fields map[string]strictjson.Value, number paneinput.IssueNumber) (Issue, error) {
	state, err := strictjson.RequiredString(fields, "state")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	title, err := strictjson.RequiredStringAllowEmpty(fields, "title")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	names, err := parseLabelNames(fields)
	if err != nil {
		return Issue{}, err
	}
	issue := Issue{Number: number}
	issue.Title, issue.TitleCut = cutTitle(Sanitize(title))
	switch state {
	case "CLOSED":
		// The state decides first: a closed issue with the draft label is
		// reported as closed.
		issue.Status = IssueClosed
	case ghOpen:
		issue.Status = IssueOpen
		if slices.Contains(names, draftLabel) {
			issue.Status = IssueDraft
		}
	default:
		return Issue{}, fmt.Errorf("%w: an issue state that is neither open nor closed", errGHOutput)
	}
	return issue, nil
}

// parseLabelNames reads the labels member: totalCount and the names of nodes.
// Contract: only an answer that holds every label is accepted, so the count
// must equal the number of nodes; a name that is not a string is rejected. The
// names are used to look for the draft label and are never shown.
func parseLabelNames(fields map[string]strictjson.Value) ([]string, error) {
	labelsValue, err := strictjson.Required(fields, "labels")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	labels, err := labelsValue.AsObject()
	if err != nil {
		return nil, fmt.Errorf("%w: labels: %w", errGHOutput, err)
	}
	members, err := labels.CollectExactCase("totalCount", "nodes")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	countValue, err := strictjson.Required(members, "totalCount")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	count, err := countValue.AsNonNegativeInt64()
	if err != nil {
		return nil, fmt.Errorf("%w: labels totalCount: %w", errGHOutput, err)
	}
	nodesValue, err := strictjson.Required(members, "nodes")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	nodes, err := nodesValue.AsArray()
	if err != nil {
		return nil, fmt.Errorf("%w: labels nodes: %w", errGHOutput, err)
	}
	if int64(len(nodes)) != count {
		return nil, fmt.Errorf("%w: the answer holds %d of %d labels", errGHOutput, len(nodes), count)
	}
	names := make([]string, 0, len(nodes))
	for i, node := range nodes {
		object, err := node.AsObject()
		if err != nil {
			return nil, fmt.Errorf("%w: label %d: %w", errGHOutput, i, err)
		}
		fields, err := object.CollectExactCase("name")
		if err != nil {
			return nil, fmt.Errorf("%w: label %d: %w", errGHOutput, i, err)
		}
		name, err := strictjson.RequiredString(fields, "name")
		if err != nil {
			return nil, fmt.Errorf("%w: label %d: %w", errGHOutput, i, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// parseIssueNotFound reads the answer of a number that exists nowhere, which
// gh writes with exit status 1: issueOrPullRequest is null and the one error
// is of type NOT_FOUND at the path of that member. Its message is not read.
func parseIssueNotFound(top map[string]strictjson.Value, repository strictjson.Object) (Issue, error) {
	repoMembers, err := repository.CollectExactCase("issueOrPullRequest")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	node, err := strictjson.Required(repoMembers, "issueOrPullRequest")
	if err != nil || !node.IsNull() {
		return Issue{}, fmt.Errorf("%w: a failed answer with a value", errGHOutput)
	}
	errorsValue, err := strictjson.Required(top, "errors")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	list, err := errorsValue.AsArray()
	if err != nil || len(list) != 1 {
		return Issue{}, fmt.Errorf("%w: not exactly one error", errGHOutput)
	}
	object, err := list[0].AsObject()
	if err != nil {
		return Issue{}, fmt.Errorf("%w: error: %w", errGHOutput, err)
	}
	fields, err := object.CollectExactCase("type", "path")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: error: %w", errGHOutput, err)
	}
	kind, err := strictjson.RequiredString(fields, "type")
	if err != nil || kind != "NOT_FOUND" {
		return Issue{}, fmt.Errorf("%w: an error that is not NOT_FOUND", errGHOutput)
	}
	pathValue, err := strictjson.Required(fields, "path")
	if err != nil {
		return Issue{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	path, err := pathValue.AsArray()
	if err != nil || len(path) != 2 {
		return Issue{}, fmt.Errorf("%w: error path", errGHOutput)
	}
	for i, want := range []string{"repository", "issueOrPullRequest"} {
		got, err := path[i].AsString()
		if err != nil || got != want {
			return Issue{}, fmt.Errorf("%w: error path", errGHOutput)
		}
	}
	return Issue{Status: IssueNotFound}, nil
}

// cutTitle cuts text at issueTitleMax characters and reports whether it cut.
func cutTitle(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= issueTitleMax {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:issueTitleMax]), true
}
