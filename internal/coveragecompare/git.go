package coveragecompare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// gitEnv returns the environment every git command of the tool runs with.
//
// Contract: no inherited GIT_* variable passes (cmdrunner.GitEnv), so the
// user's environment cannot point git at another repository, and the global
// and system configuration are not read, so a filter, a smudge, or a diff.*
// setting there cannot change what git reads or prints. The two variables are
// added after GitEnv, which drops every GIT_* variable it is given.
func gitEnv(environ []string) []string {
	return append(cmdrunner.GitEnv(environ), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
}

// gitRunner runs git in one directory.
type gitRunner struct {
	runner cmdrunner.Runner
}

// newGit returns the git of dir. Every command it runs gets gitEnv and is
// preceded by cmdrunner.GitHardening, so configuration cannot launch a
// command during a read.
func newGit(dir string, environ []string) gitRunner {
	return gitRunner{runner: cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: gitEnv(environ)})}
}

// run runs one git subcommand and returns its standard output.
func (g gitRunner) run(ctx context.Context, args ...string) ([]byte, error) {
	return g.runner.Run(ctx, "git", append(cmdrunner.GitHardening(), args...)...)
}

// commitSource reads the files of one point from its pinned commit, not from
// a worktree.
//
// Contract: a file is present only when the commit's tree holds it as a
// regular file; its content is the blob as stored, without a textconv. The
// tree is listed once, when the source is made, and each file is read at most
// once. ctx is the run's context, which bounds the reads made later through
// the source interface.
type commitSource struct {
	ctx    context.Context
	at     Point
	commit string
	git    gitRunner
	files  map[string]struct{}
	cache  map[string][]byte
}

// newCommitSource lists the regular files of commit.
func newCommitSource(ctx context.Context, git gitRunner, point Point, commit string) (*commitSource, error) {
	out, err := git.run(ctx, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, commandStop(ctx, point, "listing the files of the point's commit failed", err)
	}
	src := &commitSource{ctx: ctx, at: point, commit: commit, git: git, files: map[string]struct{}{}, cache: map[string][]byte{}}
	for entry := range bytes.SplitSeq(out, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		meta, path, ok := strings.Cut(string(entry), "\t")
		if !ok {
			return nil, internalStop(point, "git ls-tree printed an entry without a path", errLsTreeEntry)
		}
		// An entry is "<mode> <type> <object>\t<path>".
		fields := strings.Fields(meta)
		if len(fields) != lsTreeFields {
			return nil, internalStop(point, "git ls-tree printed an entry of another shape", errLsTreeEntry)
		}
		// 100644 and 100755 are the regular files; a symbolic link (120000)
		// or a submodule (160000) holds no source to read.
		if fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") {
			src.files[path] = struct{}{}
		}
	}
	return src, nil
}

// lsTreeFields is the number of fields before the path of a git ls-tree entry.
const lsTreeFields = 3

// errLsTreeEntry reports a git ls-tree entry the tool cannot read.
var errLsTreeEntry = errors.New("unexpected git ls-tree entry")

func (s *commitSource) point() Point { return s.at }

func (s *commitSource) readFile(path string) ([]byte, bool, error) {
	if _, ok := s.files[path]; !ok {
		return nil, false, nil
	}
	if data, ok := s.cache[path]; ok {
		return data, true, nil
	}
	data, err := s.git.run(s.ctx, "show", "--no-textconv", s.commit+":"+path)
	if err != nil {
		return nil, false, fmt.Errorf("git show %s:%s: %w", s.commit, path, err)
	}
	s.cache[path] = data
	return data, true, nil
}

// internalStop is the tool's own failure, with err as the reason's excerpt.
func internalStop(point Point, detail string, err error) *StopError {
	reason := StopReason{Point: point, Detail: detail, Excerpt: err.Error()}
	return &StopError{Kind: StopInternal, Reasons: []StopReason{reason}, Err: err}
}

// commandStop is the internal stop for a command that failed where its
// failure is the tool's own. When ctx is done, the failure is the
// cancellation, whatever the command reported, so a signal is never taken for
// the command's own failure.
func commandStop(ctx context.Context, point Point, detail string, err error) *StopError {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return interruptedStop(ctxErr)
	}
	return internalStop(point, detail, err)
}

// interruptedStop is the internal stop of a run whose context was canceled,
// by a signal in the tool.
func interruptedStop(err error) *StopError {
	return internalStop(PointNone, "the run was interrupted", err)
}
