package fixprguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	// maxThreadsPerPage and maxCommentsPerThread are GraphQL's first:100.
	maxThreadsPerPage    = 100
	maxCommentsPerThread = 100
	// maxListThreads caps the threads list reads over every page; a longer
	// list is refused rather than returned in part. maxListPages also bounds
	// the pages, so pages that keep reporting more without threads end too.
	maxListThreads = 2000
	maxListPages   = maxListThreads/maxThreadsPerPage + 1
	// maxResponseBytes caps one GitHub response.
	maxResponseBytes = 16 << 20

	commentURLPrefix = "https://github.com/"
)

// commentFields are the per-comment fields of the stage-one query. They carry
// no body: the trust judgment is made from authors and times only, and an
// untrusted author's text never enters this process.
const commentFields = `databaseId url path line author { __typename login } createdAt updatedAt lastEditedAt`

// stageOneQuery reads one page of review threads without comment bodies.
const stageOneQuery = `query($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          comments(first: 100) {
            pageInfo { hasNextPage }
            nodes { ` + commentFields + ` }
          }
        }
      }
    }
  }
}`

// stageTwoQuery reads the named threads with comment bodies. It is sent only
// for threads the stage-one judgment selected for processing.
const stageTwoQuery = `query($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on PullRequestReviewThread {
      id
      isResolved
      comments(first: 100) {
        pageInfo { hasNextPage }
        nodes { ` + commentFields + ` body }
      }
    }
  }
}`

// comment is one review comment as GitHub returned it. kindRaw and loginRaw
// keep the author fields verbatim for the fingerprint; the trust judgment reads
// them only through authorsOf. body is set by the stage-two fetch only.
type comment struct {
	databaseID    int64
	url           string
	path          string
	line          *int64
	authorPresent bool
	kindRaw       string
	loginRaw      string
	createdAt     string
	updatedAt     string
	lastEditedAt  *string
	body          string
}

// thread is one review thread. moreComments is the comments' hasNextPage: the
// thread has comments beyond the ones fetched.
type thread struct {
	id           string
	resolved     bool
	comments     []comment
	moreComments bool
}

// threadPage is one page of the stage-one query.
type threadPage struct {
	threads     []thread
	hasNextPage bool
	endCursor   string
}

// fetchThreadPage runs the stage-one query for one page. after is empty for the
// first page.
func (g *GitHub) fetchThreadPage(ctx context.Context, repo repository, pr PRNumber, after string) (threadPage, error) {
	variables := map[string]any{"owner": repo.owner, "name": repo.name, "number": pr.n}
	if after != "" {
		variables["after"] = after
	}
	out, err := g.graphql(ctx, stageOneQuery, variables)
	if err != nil {
		return threadPage{}, err
	}
	return parseThreadPage(out)
}

// fetchAllThreads reads every page, refusing a PR with more than
// maxListThreads threads.
func (g *GitHub) fetchAllThreads(ctx context.Context, repo repository, pr PRNumber) ([]thread, error) {
	var threads []thread
	after := ""
	for pages := 1; ; pages++ {
		page, err := g.fetchThreadPage(ctx, repo, pr, after)
		if err != nil {
			return nil, err
		}
		threads = append(threads, page.threads...)
		if len(threads) > maxListThreads {
			return nil, fmt.Errorf("%w: thread cap", ErrLimitExceeded)
		}
		if !page.hasNextPage {
			return threads, nil
		}
		if pages >= maxListPages {
			return nil, fmt.Errorf("%w: page cap", ErrLimitExceeded)
		}
		after = page.endCursor
	}
}

// fetchBodies runs the stage-two query for ids and returns the threads in the
// same order, each with its comment bodies.
func (g *GitHub) fetchBodies(ctx context.Context, ids []string) ([]thread, error) {
	out, err := g.graphql(ctx, stageTwoQuery, map[string]any{"ids": ids})
	if err != nil {
		return nil, err
	}
	data, err := graphqlData(out)
	if err != nil {
		return nil, err
	}
	nodes, err := memberArray(data, "nodes")
	if err != nil {
		return nil, err
	}
	if len(nodes) != len(ids) {
		return nil, rejected("node count")
	}
	threads := make([]thread, 0, len(nodes))
	for i, node := range nodes {
		t, err := parseThread(node, true)
		if err != nil {
			return nil, err
		}
		if t.id != ids[i] {
			return nil, rejected("node order")
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// graphql sends a query with its variables as the JSON request body on
// standard input, so no value passes through flag type conversion.
func (g *GitHub) graphql(ctx context.Context, query string, variables map[string]any) ([]byte, error) {
	request, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, fmt.Errorf("encode query: %w", err)
	}
	out, err := g.runGH(ctx, request, "api", "graphql", "--input", "-")
	if err != nil {
		return nil, ghFailure("graphql", err)
	}
	return out, nil
}

// graphqlData checks a whole GraphQL response and returns its data object. A
// response that carries errors is a failure as a whole, even with data.
func graphqlData(out []byte) (strictjson.Object, error) {
	if len(out) > maxResponseBytes {
		return strictjson.Object{}, rejected("response size")
	}
	root, err := strictjson.ParseObject(out)
	if err != nil {
		return strictjson.Object{}, rejected("response JSON")
	}
	if root.Has("errors") {
		return strictjson.Object{}, rejected("response errors")
	}
	return memberObject(root, "data")
}

func parseThreadPage(out []byte) (threadPage, error) {
	data, err := graphqlData(out)
	if err != nil {
		return threadPage{}, err
	}
	repo, err := memberObject(data, "repository")
	if err != nil {
		return threadPage{}, err
	}
	pull, err := memberObject(repo, "pullRequest")
	if err != nil {
		return threadPage{}, err
	}
	reviewThreads, err := memberObject(pull, "reviewThreads")
	if err != nil {
		return threadPage{}, err
	}
	pageInfo, err := memberObject(reviewThreads, "pageInfo")
	if err != nil {
		return threadPage{}, err
	}
	hasNextPage, err := memberBool(pageInfo, "hasNextPage")
	if err != nil {
		return threadPage{}, err
	}
	page := threadPage{hasNextPage: hasNextPage}
	if hasNextPage {
		members, err := pageInfo.Collect("endCursor")
		if err != nil {
			return threadPage{}, rejected("endCursor")
		}
		if page.endCursor, err = strictjson.RequiredString(members, "endCursor"); err != nil {
			return threadPage{}, rejected("endCursor")
		}
	}
	nodes, err := memberArray(reviewThreads, "nodes")
	if err != nil {
		return threadPage{}, err
	}
	if len(nodes) > maxThreadsPerPage {
		return threadPage{}, rejected("thread page size")
	}
	for _, node := range nodes {
		t, err := parseThread(node, false)
		if err != nil {
			return threadPage{}, err
		}
		page.threads = append(page.threads, t)
	}
	return page, nil
}

// parseThread reads one thread node. withBody is true for the stage-two
// response, the only one that carries comment bodies.
func parseThread(node strictjson.Value, withBody bool) (thread, error) {
	obj, err := node.AsObject()
	if err != nil {
		return thread{}, rejected("thread")
	}
	members, err := obj.Collect("id", "isResolved")
	if err != nil {
		return thread{}, rejected("thread")
	}
	id, err := strictjson.RequiredString(members, "id")
	if err != nil || !validThreadID(id) {
		return thread{}, rejected("thread id")
	}
	resolved, err := memberBool(obj, "isResolved")
	if err != nil {
		return thread{}, err
	}
	comments, err := memberObject(obj, "comments")
	if err != nil {
		return thread{}, err
	}
	pageInfo, err := memberObject(comments, "pageInfo")
	if err != nil {
		return thread{}, err
	}
	more, err := memberBool(pageInfo, "hasNextPage")
	if err != nil {
		return thread{}, err
	}
	nodes, err := memberArray(comments, "nodes")
	if err != nil {
		return thread{}, err
	}
	if len(nodes) > maxCommentsPerThread {
		return thread{}, rejected("comment page size")
	}
	t := thread{id: id, resolved: resolved, moreComments: more}
	for _, node := range nodes {
		c, err := parseComment(node, withBody)
		if err != nil {
			return thread{}, err
		}
		t.comments = append(t.comments, c)
	}
	return t, nil
}

// parseComment reads one comment node. A null author is valid input: it is a
// deleted account and becomes reason (c), not a rejected response. A missing or
// non-string __typename or login is likewise left for the judgment to report.
func parseComment(node strictjson.Value, withBody bool) (comment, error) {
	obj, err := node.AsObject()
	if err != nil {
		return comment{}, rejected("comment")
	}
	keys := []string{"databaseId", "url", "path", "line", "author", "createdAt", "updatedAt", "lastEditedAt"}
	if withBody {
		keys = append(keys, "body")
	}
	members, err := obj.Collect(keys...)
	if err != nil {
		return comment{}, rejected("comment")
	}
	var c comment
	id, err := strictjson.Required(members, "databaseId")
	if err != nil {
		return comment{}, rejected("comment databaseId")
	}
	if c.databaseID, err = id.AsNonNegativeInt64(); err != nil || c.databaseID == 0 {
		return comment{}, rejected("comment databaseId")
	}
	if c.url, err = strictjson.RequiredString(members, "url"); err != nil || !validCommentURL(c.url) {
		return comment{}, rejected("comment url")
	}
	if c.path, err = strictjson.RequiredString(members, "path"); err != nil {
		return comment{}, rejected("comment path")
	}
	if c.line, err = nullableInt(members, "line"); err != nil {
		return comment{}, rejected("comment line")
	}
	if c.createdAt, err = strictjson.RequiredString(members, "createdAt"); err != nil {
		return comment{}, rejected("comment createdAt")
	}
	if c.updatedAt, err = strictjson.RequiredString(members, "updatedAt"); err != nil {
		return comment{}, rejected("comment updatedAt")
	}
	if c.lastEditedAt, err = nullableString(members, "lastEditedAt"); err != nil {
		return comment{}, rejected("comment lastEditedAt")
	}
	if err := readAuthor(members, &c); err != nil {
		return comment{}, err
	}
	if withBody {
		body, err := strictjson.Required(members, "body")
		if err != nil {
			return comment{}, rejected("comment body")
		}
		if c.body, err = body.AsString(); err != nil {
			return comment{}, rejected("comment body")
		}
	}
	return c, nil
}

// readAuthor reads the author object. Only its shape is checked here: a
// non-string __typename or login is kept as empty and judged later.
func readAuthor(members map[string]strictjson.Value, c *comment) error {
	author, err := strictjson.Required(members, "author")
	if err != nil {
		return rejected("comment author")
	}
	if author.IsNull() {
		return nil
	}
	obj, err := author.AsObject()
	if err != nil {
		return rejected("comment author")
	}
	fields, err := obj.Collect("__typename", "login")
	if err != nil {
		return rejected("comment author")
	}
	c.authorPresent = true
	if v, ok := fields["__typename"]; ok {
		c.kindRaw, _ = v.AsString()
	}
	if v, ok := fields["login"]; ok {
		c.loginRaw, _ = v.AsString()
	}
	return nil
}

// validCommentURL accepts a github.com URL of printable ASCII without spaces;
// the URL is printed in reports, so it must not carry anything else.
func validCommentURL(s string) bool {
	if !strings.HasPrefix(s, commentURLPrefix) {
		return false
	}
	for i := range len(s) {
		if s[i] <= ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// fingerprint hashes the thread ID and, for each comment in fetch order, its
// database ID, author kind and login, and its three times, followed by the
// comment count and hasNextPage. Each field is quoted, so no two different
// threads share an encoding. Adding, editing or deleting a comment changes it.
func fingerprint(t thread) Fingerprint {
	h := sha256.New()
	writeField(h, t.id)
	for _, c := range t.comments {
		writeField(h, strconv.FormatInt(c.databaseID, 10))
		writeField(h, strconv.FormatBool(c.authorPresent))
		writeField(h, c.kindRaw)
		writeField(h, c.loginRaw)
		writeField(h, c.createdAt)
		writeField(h, c.updatedAt)
		if c.lastEditedAt == nil {
			writeField(h, "null")
		} else {
			writeField(h, strconv.Quote(*c.lastEditedAt))
		}
	}
	writeField(h, strconv.Itoa(len(t.comments)))
	writeField(h, strconv.FormatBool(t.moreComments))
	return Fingerprint(hex.EncodeToString(h.Sum(nil)))
}

func writeField(h hash.Hash, s string) {
	_, _ = h.Write([]byte(strconv.Quote(s)))
	_, _ = h.Write([]byte{'\n'})
}

// rejected reports a response that failed the strict check; rule names the
// check, never a value.
func rejected(rule string) error {
	return fmt.Errorf("%w: %s", ErrResponseRejected, rule)
}

func memberObject(obj strictjson.Object, key string) (strictjson.Object, error) {
	members, err := obj.Collect(key)
	if err != nil {
		return strictjson.Object{}, rejected(key)
	}
	value, err := strictjson.Required(members, key)
	if err != nil {
		return strictjson.Object{}, rejected(key)
	}
	child, err := value.AsObject()
	if err != nil {
		return strictjson.Object{}, rejected(key)
	}
	return child, nil
}

func memberArray(obj strictjson.Object, key string) ([]strictjson.Value, error) {
	members, err := obj.Collect(key)
	if err != nil {
		return nil, rejected(key)
	}
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, rejected(key)
	}
	values, err := value.AsArray()
	if err != nil {
		return nil, rejected(key)
	}
	return values, nil
}

func memberBool(obj strictjson.Object, key string) (bool, error) {
	members, err := obj.Collect(key)
	if err != nil {
		return false, rejected(key)
	}
	value, err := strictjson.Required(members, key)
	if err != nil {
		return false, rejected(key)
	}
	b, err := value.AsBool()
	if err != nil {
		return false, rejected(key)
	}
	return b, nil
}

// nullableInt reads a required member that is either null or a non-negative
// integer.
func nullableInt(members map[string]strictjson.Value, key string) (*int64, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, err
	}
	if value.IsNull() {
		return nil, nil
	}
	n, err := value.AsNonNegativeInt64()
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// nullableString reads a required member that is either null or a non-empty
// string.
func nullableString(members map[string]strictjson.Value, key string) (*string, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, err
	}
	if value.IsNull() {
		return nil, nil
	}
	s, err := value.AsString()
	if err != nil || s == "" {
		return nil, rejected(key)
	}
	return &s, nil
}
