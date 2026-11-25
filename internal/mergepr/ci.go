package mergepr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/ghci"
)

// CI wait bounds. Contract: only GitHub Actions checks are waited for; the
// PollInterval/Timeout/PerCallTimeout/NoCheckGrace values are the ones the
// design fixes for mergepr prepare, and ghciMaxOutput is the cap on one gh
// response (twice the 16 MiB ghci itself accepts).
const (
	ghciMaxOutput      = 32 << 20
	ghciWaitDelay      = 5 * time.Second
	ciPollInterval     = 30 * time.Second
	ciTimeout          = 60 * time.Minute
	ciPerCallTimeout   = 2 * time.Minute
	ciNoCheckGrace     = 5 * time.Minute
	maxReportedFailing = 10
)

// newCIRunner returns the Runner the CI check reading runs gh with: output
// capped at ghciMaxOutput, a bounded wait for the process's pipes after it is
// killed, and the same GIT_*-free environment as the tool's git.
func newCIRunner() Runner {
	return cmdrunner.NewOS(cmdrunner.Options{
		MaxOutput: ghciMaxOutput,
		WaitDelay: ghciWaitDelay,
		Env:       gitEnviron(),
	})
}

// defaultCIWait returns the polling bounds of mergepr prepare.
func defaultCIWait() ghci.WaitOptions {
	return ghci.WaitOptions{
		PollInterval:   ciPollInterval,
		Timeout:        ciTimeout,
		PerCallTimeout: ciPerCallTimeout,
		NoCheckGrace:   ciNoCheckGrace,
	}
}

// waitChecks waits for the GitHub Actions checks of the PR's prepared head.
//
// Contract: the head is pinned with ExpectedHead, so a push after prepare read
// the PR ends the wait with head-changed rather than judging another commit;
// every outcome other than ghci.StatePassed returns an error wrapping
// errChecksFailed. The error names only failing check names and URLs or a
// fixed reason code, never any other text gh returned.
func (t *Tool) waitChecks(ctx context.Context, state State) error {
	owner, repo, err := ghci.ResolveRepo(ctx, t.ci)
	if err != nil {
		return fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	res, err := ghci.WaitPR(ctx, t.ci, ghci.Request{
		Owner:        owner,
		Repo:         repo,
		PRNumber:     state.Number,
		ExpectedHead: state.HeadRefOID,
	}, t.ciWait)
	if err != nil {
		if errors.Is(err, ghci.ErrInvalidRequest) {
			return fmt.Errorf("%w: %w", errChecksFailed, err)
		}
		return fmt.Errorf("%w: wait for GitHub Actions checks ended: %w", errChecksFailed, err)
	}
	if res.State == ghci.StatePassed {
		return nil
	}
	return fmt.Errorf("%w: %s", errChecksFailed, describeCIResult(res))
}

// describeCIResult renders a non-passing result on one line: the failing check
// names (quoted) with their URLs, or the state and its reason code.
func describeCIResult(res ghci.Result) string {
	state, err := res.State.MarshalText()
	if err != nil {
		state = []byte("unknown")
	}
	if len(res.Failing) == 0 {
		reason, err := res.Reason.MarshalText()
		if err != nil || len(reason) == 0 {
			return string(state)
		}
		return string(state) + " (" + string(reason) + ")"
	}
	var parts []string
	for i, f := range res.Failing {
		if i == maxReportedFailing {
			parts = append(parts, fmt.Sprintf("and %d more", len(res.Failing)-maxReportedFailing))
			break
		}
		part := fmt.Sprintf("%q %s", f.Name, f.Conclusion)
		if f.URL != "" {
			part += " " + f.URL
		}
		parts = append(parts, part)
	}
	return string(state) + ": " + strings.Join(parts, "; ")
}
