package coveragecompare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/version"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// pinnedCommits are BASE and HEAD resolved once to commit IDs, with HEAD's
// tree. Every later git operation of the run uses these IDs, never the BASE
// argument or the name HEAD, so a ref that moves during the run (a fetch in
// another terminal) cannot change what is measured.
type pinnedCommits struct {
	base, head, headTree string
}

// objectID is a full commit or tree ID: SHA-1 or SHA-256 in lowercase hex.
var objectID = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// resolveCommits resolves BASE and HEAD in the user's repository. A BASE that
// is empty or does not name a commit stops as usage. --end-of-options is
// there so that no BASE is read as an option; the ^{commit} suffix already
// keeps a BASE that starts with "-" from matching one.
func resolveCommits(ctx context.Context, git gitRunner, base string) (pinnedCommits, error) {
	if base == "" {
		return pinnedCommits{}, usageStop("BASE is not given")
	}
	baseID, err := resolveObject(ctx, git, base+"^{commit}")
	if err != nil {
		if _, isExit := errors.AsType[*cmdrunner.ExitError](err); isExit && ctx.Err() == nil {
			return pinnedCommits{}, usageStop("BASE does not resolve to a commit")
		}
		return pinnedCommits{}, commandStop(ctx, PointBase, "resolving BASE failed", err)
	}
	headID, err := resolveObject(ctx, git, "HEAD^{commit}")
	if err != nil {
		return pinnedCommits{}, commandStop(ctx, PointHead, "resolving HEAD failed", err)
	}
	tree, err := resolveObject(ctx, git, headID+"^{tree}")
	if err != nil {
		return pinnedCommits{}, commandStop(ctx, PointHead, "resolving the tree of HEAD failed", err)
	}
	return pinnedCommits{base: baseID, head: headID, headTree: tree}, nil
}

// resolveObject returns the full ID rev names.
func resolveObject(ctx context.Context, git gitRunner, rev string) (string, error) {
	out, err := git.run(ctx, "rev-parse", "--verify", "--end-of-options", rev)
	if err != nil {
		return "", err
	}
	id := strings.TrimSuffix(string(out), "\n")
	if !objectID.MatchString(id) {
		return "", fmt.Errorf("%w: %q", errNotObjectID, id)
	}
	return id, nil
}

// errNotObjectID reports git rev-parse output that is not one full object ID.
var errNotObjectID = errors.New("git rev-parse printed no object ID")

// usageStop is the stop for a BASE argument the tool cannot use.
func usageStop(detail string) *StopError {
	return &StopError{Kind: StopUsage, Reasons: []StopReason{{Detail: detail}}}
}

// checkAncestor stops as not-ancestor unless BASE is a proper ancestor of
// HEAD.
func checkAncestor(ctx context.Context, git gitRunner, ids pinnedCommits) error {
	if ids.base == ids.head {
		return notAncestorStop("BASE and HEAD are the same commit")
	}
	_, err := git.run(ctx, "merge-base", "--is-ancestor", ids.base, ids.head)
	if err == nil {
		return nil
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 && ctx.Err() == nil {
		return notAncestorStop("BASE is not an ancestor of HEAD")
	}
	return commandStop(ctx, PointNone, "checking whether BASE is an ancestor of HEAD failed", err)
}

// notAncestorStop is the stop for a BASE that is not a proper ancestor.
func notAncestorStop(detail string) *StopError {
	return &StopError{Kind: StopNotAncestor, Reasons: []StopReason{{Detail: detail}}}
}

// statusArgs list every uncommitted change: tracked, staged, and untracked
// files, and submodules. A file git ignores is not listed.
var statusArgs = []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"}

// checkClean stops as dirty when the user's worktree or index has an
// uncommitted change, so what is measured is what is committed.
func checkClean(ctx context.Context, git gitRunner) error {
	out, err := git.run(ctx, statusArgs...)
	if err != nil {
		return commandStop(ctx, PointNone, "reading git status failed", err)
	}
	if len(out) == 0 {
		return nil
	}
	reason := StopReason{Detail: "the worktree or the index has uncommitted changes", Excerpt: string(out)}
	return &StopError{Kind: StopDirty, Reasons: []StopReason{reason}}
}

// toolchainStop is the stop for a pinned version that the installed tool does
// not match, at the point and file it names.
func toolchainStop(point Point, file, detail string) *StopError {
	reason := StopReason{Point: point, Detail: detail}
	if file != "" {
		reason.Func = &FuncID{File: file}
	}
	return &StopError{Kind: StopToolchain, Reasons: []StopReason{reason}}
}

// requireFile reads a file the toolchain check needs; an absent file stops as
// toolchain.
func requireFile(src source, file string) ([]byte, error) {
	data, exists, err := src.readFile(file)
	if err != nil {
		return nil, readFailure(src, file, err)
	}
	if !exists {
		return nil, toolchainStop(src.point(), file, "the file is absent from the point's commit")
	}
	return data, nil
}

// checkGoVersion checks that the installed go, whose version is goVersion
// (what go env GOVERSION prints), is at least the go line of each point's
// go.mod. The toolchain line is not read: with GOTOOLCHAIN=local it requests
// no version.
func checkGoVersion(goVersion string, sources ...source) error {
	if !version.IsValid(goVersion) {
		return toolchainStop(PointNone, "", fmt.Sprintf("go env GOVERSION printed %q, which is not a Go version", goVersion))
	}
	for _, src := range sources {
		data, err := requireFile(src, "go.mod")
		if err != nil {
			return err
		}
		line, ok := goModGoLine(data)
		want := "go" + line
		if !ok || !version.IsValid(want) {
			return toolchainStop(src.point(), "go.mod", "the go line of go.mod cannot be read")
		}
		if version.Compare(goVersion, want) < 0 {
			return toolchainStop(src.point(), "go.mod",
				fmt.Sprintf("go.mod requires go %s, and the installed go is %s", line, goVersion))
		}
	}
	return nil
}

// goModGoLine returns the version of the one go directive of a go.mod. A
// go.mod without one, with two, or with a malformed one is reported as not
// read.
func goModGoLine(data []byte) (string, bool) {
	var found []string
	for raw := range bytes.SplitSeq(data, []byte("\n")) {
		line := string(raw)
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "go" {
			continue
		}
		if len(fields) != goDirectiveFields {
			return "", false
		}
		found = append(found, fields[1])
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// goDirectiveFields is the number of fields of a go directive: "go" and the
// version.
const goDirectiveFields = 2

// The extension's pinned versions live in these files of each commit.
const (
	nodeVersionFile = "extension/.node-version"
	packageJSONFile = "extension/package.json"
	lockfileFile    = "extension/package-lock.json"
)

// checkExtVersions checks the installed node and npm against each point's
// pins, the user's extension/node_modules, and that BASE and HEAD pin the same
// lockfile, so both points are measured with the same dependencies.
//
// The Makefile's ext_check_versions compares the same values, from the
// worktree, for the other ext-* targets: nodeVersion (node --version) against
// extension/.node-version with its leading "v", and npmVersion (npm --version)
// against the packageManager of extension/package.json after "npm@". Change
// both together.
func checkExtVersions(nodeVersion, npmVersion, repoRoot string, base, head source) error {
	var locks [][]byte
	for _, src := range []source{base, head} {
		data, err := requireFile(src, nodeVersionFile)
		if err != nil {
			return err
		}
		want := strings.TrimRight(string(data), "\n")
		if want == "" || nodeVersion != "v"+want {
			return toolchainStop(src.point(), nodeVersionFile,
				fmt.Sprintf("Node.js %q is required, and node --version printed %q", want, nodeVersion))
		}
		data, err = requireFile(src, packageJSONFile)
		if err != nil {
			return err
		}
		wantNPM, ok := packageManagerNPM(data)
		if !ok {
			return toolchainStop(src.point(), packageJSONFile, "the packageManager of package.json is not npm@<version>")
		}
		if npmVersion != wantNPM {
			return toolchainStop(src.point(), packageJSONFile,
				fmt.Sprintf("npm %q is required, and npm --version printed %q", wantNPM, npmVersion))
		}
		lock, err := requireFile(src, lockfileFile)
		if err != nil {
			return err
		}
		locks = append(locks, lock)
	}
	info, err := os.Stat(filepath.Join(repoRoot, "extension", "node_modules"))
	if err != nil || !info.IsDir() {
		return toolchainStop(PointNone, "extension/node_modules", "the directory is missing; run 'make ext-install' first")
	}
	if !bytes.Equal(locks[0], locks[1]) {
		return toolchainStop(PointNone, lockfileFile, "BASE and HEAD pin different dependencies; compare them in separate commits")
	}
	return nil
}

// packageManagerNPM returns the npm version the packageManager member of a
// package.json pins: the part after "npm@", which must not be empty.
func packageManagerNPM(data []byte) (string, bool) {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return "", false
	}
	members, err := object.Collect("packageManager")
	if err != nil {
		return "", false
	}
	value, err := strictjson.RequiredString(members, "packageManager")
	if err != nil {
		return "", false
	}
	pinned, ok := strings.CutPrefix(value, "npm@")
	return pinned, ok && pinned != ""
}
