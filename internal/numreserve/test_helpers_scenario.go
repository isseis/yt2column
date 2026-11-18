//go:build test

package numreserve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// The SHAs the scenarios use. Each is 40 lowercase hexadecimal digits.
const (
	mainSHA  = "1111111111111111111111111111111111111111"
	otherSHA = "2222222222222222222222222222222222222222"
	thirdSHA = "3333333333333333333333333333333333333333"
	blobSHA  = "4444444444444444444444444444444444444444"
	// probeRef is the ref of the sample GET response (testdata/gh/get-200.txt).
	probeRef = "refs/tags/numreserve-probe/20261117T124351Z-53774"
	// leak marks text a stop detail must never carry: a response body, a
	// command's standard error, or a URL.
	leak = "LEAK-https://example.invalid/token"
)

// readSample returns a file under testdata/.
func readSample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The gh api -i outputs, built from the samples.

func created(t *testing.T) string { return readSample(t, "gh/post-201.txt") }

func taken(t *testing.T) string { return readSample(t, "gh/post-422-same-sha.txt") + leak }

func notFound(t *testing.T) string { return readSample(t, "gh/get-404.txt") + leak }

// readBack is the sample GET 200 response with its ref replaced by ref.
func readBack(t *testing.T, ref string) string {
	t.Helper()
	sample := readSample(t, "gh/get-200.txt")
	if !strings.Contains(sample, `"ref":"`+probeRef+`"`) {
		t.Fatal("testdata/gh/get-200.txt does not hold the probe ref")
	}
	return strings.Replace(sample, probeRef, ref, 1)
}

// withStatus replaces the status line of a response.
func withStatus(response, status string) string {
	_, rest, _ := strings.Cut(response, "\n")
	return status + "\n" + rest
}

// withHeader adds a header line right after the status line.
func withHeader(response, header string) string {
	status, rest, _ := strings.Cut(response, "\n")
	return status + "\n" + header + "\r\n" + rest
}

// ghFailed is gh exiting with code and stdout, and with standard error that
// must never reach a detail.
func ghFailed(code int, stdout string) error {
	return &cmdrunner.ExitError{Code: code, Stdout: []byte(stdout), Stderr: []byte(leak)}
}

// gitFailed is git exiting with code and standard error that must never
// reach a detail.
func gitFailed(code int) error {
	return &cmdrunner.ExitError{Code: code, Stderr: []byte(leak)}
}

func gitStep(out string, err error, args ...string) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "git", Args: append(cmdrunner.GitHardening(), args...), Out: out, Err: err}
}

// scriptedGH spells out the gh api -i arguments the tool must run, apart
// from ghArgs, so a change to ghArgs fails the scripts.
func scriptedGH(args ...string) []string {
	return append([]string{"api", "--hostname", "github.com", "-i"}, args...)
}

func postStep(tag, out string, err error) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{
		Name: "gh",
		Args: scriptedGH("-X", "POST", "repos/isseis/yt2column/git/refs", "-f", "ref="+tag, "-f", "sha="+mainSHA),
		Out:  out, Err: err,
	}
}

func getStep(tag, out string, err error) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{
		Name: "gh",
		Args: scriptedGH("repos/isseis/yt2column/git/ref/" + strings.TrimPrefix(tag, "refs/")),
		Out:  out, Err: err,
	}
}

// remoteBranch is one origin branch of a scenario, with the ADR file names
// its tree holds.
type remoteBranch struct {
	name string
	sha  string
	adrs []string
}

// scenario describes the inputs one run reads. Its zero value plus a space
// is the baseline: origin/main holds task 0100 and ADR 0051, and nothing
// else uses a number.
type scenario struct {
	space     Space
	mainTasks []string
	mainADRs  []string
	branches  []remoteBranch
	tags      []string
	// remoteExtra holds raw git ls-remote lines added after the others.
	remoteExtra []string
	local       []string
	headADRs    []string
	workADRs    []string
}

func (s scenario) tagPrefix() string {
	prefix, err := s.space.tagPrefix()
	if err != nil {
		panic(err)
	}
	return prefix
}

func (s scenario) tag(number string) string { return s.tagPrefix() + number }

func (s scenario) lsRemote() string {
	var b strings.Builder
	b.WriteString(mainSHA + "\trefs/heads/main\n")
	for _, br := range s.branches {
		b.WriteString(br.sha + "\trefs/heads/" + br.name + "\n")
	}
	for _, tag := range s.tags {
		b.WriteString(mainSHA + "\t" + s.tagPrefix() + tag + "\n")
	}
	for _, line := range s.remoteExtra {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// treeOut is git ls-tree --full-tree -z output for entries under dir; a name
// ending in "/" is a tree.
func treeOut(dir string, names ...string) string {
	var b strings.Builder
	for _, name := range names {
		if n, isTree := strings.CutSuffix(name, "/"); isTree {
			b.WriteString("040000 tree " + blobSHA + "\t" + dir + n + "\x00")
			continue
		}
		b.WriteString("100644 blob " + blobSHA + "\t" + dir + name + "\x00")
	}
	return b.String()
}

func (s scenario) mainTaskList() string {
	if s.mainTasks != nil {
		return treeOut(tasksDir, s.mainTasks...)
	}
	return treeOut(tasksDir, "0000_template/", "0100_number_reservation/")
}

func (s scenario) mainADRList() string {
	if s.mainADRs != nil {
		return treeOut(adrDir, s.mainADRs...)
	}
	return treeOut(adrDir, "0051-number-reservation.md", "README.md")
}

func lsRemoteStep(prefix, out string, err error) cmdrunnertestutil.Step {
	return gitStep(out, err, "ls-remote", "origin", "refs/heads/*", prefix+"*")
}

func fetchStep(err error, shas ...string) cmdrunnertestutil.Step {
	return gitStep("", err, append([]string{"fetch", "--no-tags", "--no-write-fetch-head", "origin"}, shas...)...)
}

func treeStep(rev, dir, out string, err error) cmdrunnertestutil.Step {
	return gitStep(out, err, "ls-tree", "--full-tree", "-z", rev, "--", dir)
}

func localStep(out string, err error) cmdrunnertestutil.Step {
	return gitStep(out, err, "for-each-ref", "--format=%(refname)", "refs/heads/")
}

func rootStep(root string, err error) cmdrunnertestutil.Step {
	return gitStep(root+"\n", err, "rev-parse", "--show-toplevel")
}

// collectSteps are the commands a run of the scenario makes before its first
// creation, every one succeeding.
func (s scenario) collectSteps(t *testing.T) []cmdrunnertestutil.Step {
	t.Helper()
	switch s.space {
	case SpaceTask:
		var local strings.Builder
		for _, name := range s.local {
			local.WriteString("refs/heads/" + name + "\n")
		}
		return []cmdrunnertestutil.Step{
			rootStep(t.TempDir(), nil),
			lsRemoteStep(s.tagPrefix(), s.lsRemote(), nil),
			fetchStep(nil, mainSHA),
			treeStep(mainSHA, tasksDir, s.mainTaskList(), nil),
			localStep(local.String(), nil),
		}
	case SpaceADR:
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "dev", "adr")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range append([]string{"README.md", "0051-number-reservation.md"}, s.workADRs...) {
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		shas := []string{mainSHA}
		for _, br := range s.branches {
			if !slices.Contains(shas, br.sha) {
				shas = append(shas, br.sha)
			}
		}
		steps := []cmdrunnertestutil.Step{
			rootStep(root, nil),
			lsRemoteStep(s.tagPrefix(), s.lsRemote(), nil),
			fetchStep(nil, shas...),
			treeStep(mainSHA, adrDir, s.mainADRList(), nil),
		}
		read := map[string]struct{}{}
		for _, br := range s.branches {
			if _, done := read[br.sha]; done {
				continue
			}
			read[br.sha] = struct{}{}
			steps = append(steps, treeStep(br.sha, adrDir, treeOut(adrDir, br.adrs...), nil))
		}
		return append(steps, treeStep("HEAD", adrDir, treeOut(adrDir, append([]string{"0051-number-reservation.md", "README.md"}, s.headADRs...)...), nil))
	default:
		t.Fatalf("scenario has no valid space: %d", s.space)
		return nil
	}
}

// run runs Reserve on the scripted steps with bounds b.
func run(t *testing.T, space Space, b bounds, steps ...cmdrunnertestutil.Step) (Reservation, error) {
	t.Helper()
	tool := newTestTool(cmdrunnertestutil.New(t, steps...), b)
	return tool.Reserve(context.Background(), space)
}

// stopKind returns the kind of a *StopError, failing the test for any other
// error, and checks the detail is one safe line.
func stopKind(t *testing.T, err error) StopKind {
	t.Helper()
	stopErr, ok := errors.AsType[*StopError](err)
	if !ok {
		t.Fatalf("error %v is not a *StopError", err)
	}
	text := stopErr.Error()
	if strings.Contains(text, leak) {
		t.Errorf("stop %q carries a body, standard error, or URL", text)
	}
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			t.Errorf("stop %q carries a control character %#x", text, r)
		}
	}
	return stopErr.Kind
}
