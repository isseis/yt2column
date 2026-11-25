package fixprguard

import (
	"context"
	"errors"
	"fmt"

	"github.com/isseis/yt2column/internal/ghci"
)

// CIChecksRequest names the pull request whose GitHub Actions CI checks are
// read. ExpectedHead, when non-empty, must be 40 lowercase hex characters.
type CIChecksRequest struct {
	PR           PRNumber
	ExpectedHead string // optional 40 lowercase hex
}

// CIChecksResult is the output of ci-checks. Failing is always a non-nil slice.
type CIChecksResult struct {
	Protocol int                   `json:"protocol"`
	State    ghci.State            `json:"state"`
	Reason   ghci.UnreadableReason `json:"reason"`
	Head     string                `json:"head"`
	Suites   int                   `json:"suites"`
	Checks   int                   `json:"checks"`
	Failing  []ghci.FailingCheck   `json:"failing"`
}

// CIChecks resolves the local repository and reads GitHub Actions CI checks
// for req.PR once under githubTimeout.
//
// Contract: invalid caller arguments return ErrInvalidInput before any gh call;
// every external failure, schema rejection, pagination cap breach, or
// ExpectedHead mismatch returns a CIChecksResult with StateUnreadable,
// Protocol set, and a non-nil Failing slice.
func (g *GitHub) CIChecks(ctx context.Context, req CIChecksRequest) (CIChecksResult, error) {
	if !req.PR.usable() {
		return CIChecksResult{}, fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	if req.ExpectedHead != "" && !validHex(req.ExpectedHead, sha1HexLength) {
		return CIChecksResult{}, fmt.Errorf("%w: expected head", ErrInvalidInput)
	}
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()

	owner, repo, err := ghci.ResolveRepo(ctx, g.run)
	if err != nil {
		return unreadableCIChecks(ciRepoErrorReason(ctx, err)), nil
	}
	res, err := ghci.ReadPR(ctx, g.run, ghci.Request{
		Owner:        owner,
		Repo:         repo,
		PRNumber:     req.PR.n,
		ExpectedHead: req.ExpectedHead,
	})
	if err != nil {
		return CIChecksResult{}, fmt.Errorf("%w: CI checks request", ErrInvalidInput)
	}
	if res.State == ghci.StateUnreadable && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.Reason = ghci.UnreadableCall
	}
	failing := res.Failing
	if failing == nil {
		failing = []ghci.FailingCheck{}
	}
	return CIChecksResult{
		Protocol: Protocol,
		State:    res.State,
		Reason:   res.Reason,
		Head:     res.Head,
		Suites:   res.Suites,
		Checks:   res.Checks,
		Failing:  failing,
	}, nil
}

func unreadableCIChecks(reason ghci.UnreadableReason) CIChecksResult {
	return CIChecksResult{
		Protocol: Protocol,
		State:    ghci.StateUnreadable,
		Reason:   reason,
		Failing:  []ghci.FailingCheck{},
	}
}

func ciRepoErrorReason(ctx context.Context, err error) ghci.UnreadableReason {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ghci.UnreadableCall
	}
	if repoErr, ok := errors.AsType[*ghci.RepoError](err); ok {
		switch repoErr.Reason {
		case ghci.UnreadableCall, ghci.UnreadableRejected, ghci.UnreadableLimit:
			return repoErr.Reason
		default:
			return ghci.UnreadableRejected
		}
	}
	return ghci.UnreadableCall
}
