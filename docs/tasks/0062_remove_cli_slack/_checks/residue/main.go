//go:build test

// Command residue checks, once per pull request of task 0062, that what the
// task removes is gone at the pull request's head and that the browser
// extension and the server's product code did not change:
//
//	go run -tags test ./docs/tasks/0062_remove_cli_slack/_checks/residue/ <merge-base> <head>
//
// Pass the merge-base with origin/main (git merge-base origin/main HEAD) and
// the head of the pull request. Every file is read from those commits with
// git (git ls-tree, git show, git diff), never from the working tree, and the
// Go code is checked by its syntax tree (packages, declarations, comments,
// and string literals), not by a text search, so a local variable or another
// package's identifier with the same name does not count. A product file is a
// Go file whose name does not end in _test.go and whose first //go:build line
// does not start with the test tag. The Go files under docs/tasks/ are never
// searched: the checks of a task hold the words they look for.
//
// The checks:
//
//   - slack-flag: builds the CLI of head in a temporary worktree and runs it
//     with each form of the removed flag (--slack and -slack, alone and with
//     =true and =false, each with and without --out, and --slack after the
//     video URL, with and without --out), each run in a new temporary
//     directory with an environment built from nothing: PATH=/usr/bin:/bin, a
//     fake DeepSeek model and key, a cache directory that does not exist yet,
//     a fake yt-dlp that creates a marker file and exits 1, a
//     SLACK_WEBHOOK_URL that passes the CLI's validation and points at a
//     loopback listener that counts the connections it accepts, and proxy
//     variables that point at a closed loopback address. A form holds when
//     the CLI exits 2, the marker does not exist, the cache directory still
//     does not exist, the --out path does not exist, the listener accepted no
//     connection, and standard error holds the undefined-flag line and the
//     usage hint (the flag before the URL) or the argument-count line and the
//     usage hint (the flag after it). A control run with --out alone must
//     create the marker, which shows the environment lets a run reach yt-dlp.
//   - cli-code: cmd/yt2column's product files refer to none of
//     publisher.NewSlackWebhookPublisher, publisher.SlackWebhookPublisher,
//     publisher.SlackPostError, publisher.SlackHTTPStatusError,
//     publisher.ErrSlackMention, and publisher.SlackPostTimeout; cliOptions
//     has no field slack and deps no field newSlackPublisher; nothing declares
//     errSlackFalse, errOutAndSlack, destinationWebhook, or
//     newSlackPublisher; no call defines a flag named "slack"; and
//     internal/job declares neither RemoteOutput nor outputKind.
//   - slack-literal: no Go file outside docs/tasks/ has a string literal that
//     starts with -slack or --slack, holds --slack as a word, or holds
//     YT2COLUMN_CLI_SLACK_INTEGRATION, and the Makefile does not hold --slack.
//   - cli-words: no comment and no string literal of a product file of
//     cmd/yt2column or internal/job holds "webhook", ignoring case.
//   - config: internal/config's Config has neither the method SlackWebhookURL
//     nor RequireSlackWebhookURL, and ServerConfig has the method WebhookURL.
//   - make-target: the Makefile holds neither test-integration-cli-slack nor
//     CLI_SLACK_INTEGRATION_TIMEOUT, and cmd/yt2column/integration_slack_test.go
//     does not exist.
//   - unchanged: nothing under extension/ changed between the commits, and no
//     product file of internal/server/ (judged on either side) changed.
//
// Each check prints OK or NG with its findings. Exit status: 0 when every
// check holds, 1 when one does not, 2 when a check cannot be run (wrong
// arguments, a git command, a build, or a parse that fails). The temporary
// worktree and directories are removed however the run ends.
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the merge. The //go:build test first line makes the
// repository-wide environment access check treat the file as test code.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

// errNoGoFile reports a directory with no Go file.
var errNoGoFile = errors.New("no Go file")

// testConstraint matches a first //go:build line that starts with the test
// tag. The tag must end there, so a tag such as "testing" is not matched.
var testConstraint = regexp.MustCompile(`^//go:build test([^A-Za-z0-9_.]|$)`)

// slackWord matches --slack as a word: not inside a longer flag name.
var slackWord = regexp.MustCompile(`(^|[^A-Za-z0-9_-])--slack([^A-Za-z0-9_-]|$)`)

// modulePath is the module's import path.
const modulePath = "github.com/isseis/yt2column"

// check is one named check. It returns the findings that break it, none when
// it holds, or an error when it cannot be run.
type check struct {
	name string
	run  func(base, head string) ([]string, error)
}

var checks = []check{
	{"slack-flag", checkSlackFlag},
	{"cli-code", checkCLICode},
	{"slack-literal", checkSlackLiteral},
	{"cli-words", checkCLIWords},
	{"config", checkConfig},
	{"make-target", checkMakeTarget},
	{"unchanged", checkUnchanged},
}

// cleanups are run before the process exits, however it exits.
var cleanups []func()

func main() {
	os.Exit(runChecks())
}

func runChecks() int {
	defer func() {
		for _, c := range slices.Backward(cleanups) {
			c()
		}
	}()
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: residue <merge-base> <head>")
		return 2
	}
	base, head := os.Args[1], os.Args[2]
	failed := false
	for _, c := range checks {
		findings, err := c.run(base, head)
		if err != nil {
			fmt.Fprintf(os.Stderr, "residue: %s: %v\n", c.name, err)
			return 2
		}
		if len(findings) == 0 {
			fmt.Printf("OK   %s\n", c.name)
			continue
		}
		failed = true
		fmt.Printf("NG   %s: %d finding(s)\n", c.name, len(findings))
		for _, f := range findings {
			fmt.Printf("     %s\n", f)
		}
	}
	if failed {
		return 1
	}
	return 0
}

// tempDir makes a temporary directory that is removed before the process
// exits.
func tempDir() (string, error) {
	dir, err := os.MkdirTemp("", "residue-")
	if err != nil {
		return "", err
	}
	cleanups = append(cleanups, func() { _ = os.RemoveAll(dir) })
	return dir, nil
}

// buildCLI builds cmd/yt2column of commit in a temporary worktree and returns
// the binary's path. The worktree is removed before buildCLI returns; the
// binary's directory before the process exits.
func buildCLI(commit string) (bin string, err error) {
	dir, err := tempDir()
	if err != nil {
		return "", err
	}
	tree := filepath.Join(dir, "tree")
	if out, err := exec.Command("git", "worktree", "add", "--detach", tree, commit).CombinedOutput(); err != nil { //nolint:gosec // a commit this tool was given
		return "", fmt.Errorf("git worktree add: %w: %s", err, out)
	}
	defer func() {
		if out, rmErr := exec.Command("git", "worktree", "remove", "--force", tree).CombinedOutput(); rmErr != nil && err == nil { //nolint:gosec // as above
			err = fmt.Errorf("git worktree remove: %w: %s", rmErr, out)
		}
	}()
	bin = filepath.Join(dir, "yt2column")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/yt2column") //nolint:gosec // a fixed package
	cmd.Dir = tree
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %w: %s", err, out)
	}
	return bin, nil
}

// videoURL is a video URL the CLI accepts.
const videoURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

// usageHintLine is the line the CLI ends every usage error with.
const usageHintLine = "yt2column: run 'yt2column -h' for usage"

// flagRun is one run of the CLI by checkSlackFlag. args builds the arguments
// from the --out path; wantLine is the line standard error must hold besides
// the usage hint.
type flagRun struct {
	name     string
	args     func(out string) []string
	wantLine string
}

// flagRuns lists the forms of the removed flag.
func flagRuns() []flagRun {
	const undefined = "yt2column: flag provided but not defined: -slack"
	const argCount = "yt2column: expected exactly one video URL after the flags, got 2 arguments"
	var runs []flagRun
	for _, form := range []string{"--slack", "--slack=true", "--slack=false", "-slack", "-slack=true", "-slack=false"} {
		runs = append(runs,
			flagRun{form, func(string) []string { return []string{form, videoURL} }, undefined},
			flagRun{"--out " + form, func(out string) []string { return []string{"--out", out, form, videoURL} }, undefined},
		)
	}
	return append(runs,
		flagRun{"URL --slack", func(string) []string { return []string{videoURL, "--slack"} }, argCount},
		flagRun{"--out URL --slack", func(out string) []string { return []string{"--out", out, videoURL, "--slack"} }, argCount},
	)
}

// runResult is what one CLI run left behind.
type runResult struct {
	code        int
	stderr      []string
	ytDlpRan    bool
	cacheExists bool
	outExists   bool
	accepted    int64
}

// runCLI runs bin with args (built from the --out path) in a new temporary
// directory and an environment built from nothing.
func runCLI(bin string, args func(out string) []string) (runResult, error) {
	dir, err := tempDir()
	if err != nil {
		return runResult{}, err
	}
	marker := filepath.Join(dir, "yt-dlp-ran")
	fake := filepath.Join(dir, "yt-dlp")
	// The fake yt-dlp has the shape of transcripttestutil.NewTripwire: it
	// creates the marker and fails without writing a subtitle.
	q := "'" + strings.ReplaceAll(marker, "'", `'\''`) + "'"
	script := "#!/bin/sh\nprintf '%s' '' > " + q + ".tmp && mv " + q + ".tmp " + q + "\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil { //nolint:gosec // the fake yt-dlp must be executable
		return runResult{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return runResult{}, err
	}
	defer func() { _ = listener.Close() }()
	var accepted atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return runResult{}, err
	}
	proxy := "http://" + closed.Addr().String()
	if err := closed.Close(); err != nil {
		return runResult{}, err
	}
	token := make([]byte, 8)
	if _, err := rand.Read(token); err != nil {
		return runResult{}, err
	}
	cache := filepath.Join(dir, "cache")
	out := filepath.Join(dir, "article.md")
	cmd := exec.Command(bin, args(out)...) //nolint:gosec // the CLI this tool built
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"YT2COLUMN_LLM=deepseek/fake-model",
		"DEEPSEEK_API_KEY=fake-deepseek-key",
		"YT2COLUMN_CACHE_DIR=" + cache,
		"YT2COLUMN_YTDLP_PATH=" + fake,
		"SLACK_WEBHOOK_URL=https://" + listener.Addr().String() + "/hooks/" + hex.EncodeToString(token),
		"HTTPS_PROXY=" + proxy,
		"HTTP_PROXY=" + proxy,
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return runResult{}, fmt.Errorf("run the CLI: %w", err)
		}
		code = exit.ExitCode()
	}
	return runResult{
		code:        code,
		stderr:      strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n"),
		ytDlpRan:    exists(marker),
		cacheExists: exists(cache),
		outExists:   exists(out),
		accepted:    accepted.Load(),
	}, nil
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// checkSlackFlag runs the CLI of head with each form of the removed flag, and
// the control run.
func checkSlackFlag(_, head string) ([]string, error) {
	bin, err := buildCLI(head)
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, r := range flagRuns() {
		res, err := runCLI(bin, r.args)
		if err != nil {
			return nil, err
		}
		if res.code != 2 {
			findings = append(findings, fmt.Sprintf("%s: exit status %d, want 2", r.name, res.code))
		}
		if res.ytDlpRan {
			findings = append(findings, r.name+": the fake yt-dlp ran (marker created)")
		}
		if res.cacheExists {
			findings = append(findings, r.name+": the cache directory was created")
		}
		if res.outExists {
			findings = append(findings, r.name+": the --out path was created")
		}
		if res.accepted != 0 {
			findings = append(findings, fmt.Sprintf("%s: the webhook listener accepted %d connection(s)", r.name, res.accepted))
		}
		if !slices.Equal(res.stderr, []string{r.wantLine, usageHintLine}) {
			findings = append(findings, fmt.Sprintf("%s: standard error is %q, want %q and the usage hint", r.name, res.stderr, r.wantLine))
		}
	}
	control, err := runCLI(bin, func(out string) []string { return []string{"--out", out, videoURL} })
	if err != nil {
		return nil, err
	}
	if !control.ytDlpRan {
		findings = append(findings, fmt.Sprintf("control (--out alone): the fake yt-dlp did not run (exit status %d, standard error %q)", control.code, control.stderr))
	}
	return findings, nil
}

// removedRefs are the names of internal/publisher the CLI no longer refers to.
var removedRefs = []string{
	"NewSlackWebhookPublisher", "SlackWebhookPublisher", "SlackPostError",
	"SlackHTTPStatusError", "ErrSlackMention", "SlackPostTimeout",
}

// flagDefiners are the flag.FlagSet methods (and flag package functions) that
// define a flag; the name is their first argument.
var flagDefiners = []string{
	"Bool", "BoolVar", "BoolFunc", "String", "StringVar", "Int", "IntVar",
	"Int64", "Int64Var", "Uint", "UintVar", "Uint64", "Uint64Var", "Float64",
	"Float64Var", "Duration", "DurationVar", "Var", "Func", "TextVar",
}

// checkCLICode checks that the CLI no longer builds a Webhook publisher, holds
// the flag, or chooses a destination, and that the job has no remote output.
func checkCLICode(_, head string) ([]string, error) {
	var findings []string
	files, err := packageFiles(head, "cmd/yt2column")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !f.product {
			continue
		}
		for _, pos := range f.qualifiedRefs(modulePath+"/internal/publisher", removedRefs) {
			findings = append(findings, fmt.Sprintf("%s refers to a removed publisher name", pos))
		}
		for _, tf := range [][2]string{{"cliOptions", "slack"}, {"deps", "newSlackPublisher"}} {
			if pos, ok := f.field(tf[0], tf[1]); ok {
				findings = append(findings, fmt.Sprintf("%s declares the field %s.%s", pos, tf[0], tf[1]))
			}
		}
		for _, name := range []string{"errSlackFalse", "errOutAndSlack", "destinationWebhook", "newSlackPublisher"} {
			if pos, ok := f.declares(name); ok {
				findings = append(findings, fmt.Sprintf("%s declares %s", pos, name))
			}
		}
		for _, pos := range f.flagDefinitions("slack") {
			findings = append(findings, fmt.Sprintf("%s defines the flag slack", pos))
		}
	}
	jobFiles, err := packageFiles(head, "internal/job")
	if err != nil {
		return nil, err
	}
	for _, f := range jobFiles {
		for _, name := range []string{"RemoteOutput", "outputKind"} {
			if pos, ok := f.declares(name); ok {
				findings = append(findings, fmt.Sprintf("%s declares %s", pos, name))
			}
		}
	}
	return findings, nil
}

// checkSlackLiteral checks that no Go string literal passes or names the
// removed flag or opt-in variable, and that the Makefile does not hold the
// flag.
func checkSlackLiteral(_, head string) ([]string, error) {
	all, err := allGoFiles(head)
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, f := range all {
		for _, lit := range f.stringLiterals() {
			switch {
			case strings.HasPrefix(lit.value, "-slack"), strings.HasPrefix(lit.value, "--slack"):
				findings = append(findings, fmt.Sprintf("%s has a string literal starting with %q", lit.pos, lit.value[:min(len(lit.value), 12)]))
			case slackWord.MatchString(lit.value):
				findings = append(findings, fmt.Sprintf("%s has a string literal holding --slack", lit.pos))
			case strings.Contains(lit.value, "YT2COLUMN_CLI_SLACK_INTEGRATION"):
				findings = append(findings, fmt.Sprintf("%s has a string literal holding YT2COLUMN_CLI_SLACK_INTEGRATION", lit.pos))
			}
		}
	}
	makefile, err := gitShow(head, "Makefile")
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(string(makefile), "\n") {
		if strings.Contains(line, "--slack") {
			findings = append(findings, fmt.Sprintf("Makefile:%d holds --slack", i+1))
		}
	}
	return findings, nil
}

// checkCLIWords checks that the product comments and string literals of the
// CLI and the job do not mention a webhook.
func checkCLIWords(_, head string) ([]string, error) {
	var findings []string
	for _, dir := range []string{"cmd/yt2column", "internal/job"} {
		files, err := packageFiles(head, dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.product {
				continue
			}
			for _, pos := range f.textContaining("webhook") {
				findings = append(findings, fmt.Sprintf("%s holds \"webhook\"", pos))
			}
		}
	}
	return findings, nil
}

// checkConfig checks that Config no longer returns the Webhook URL and that
// ServerConfig still does.
func checkConfig(_, head string) ([]string, error) {
	files, err := packageFiles(head, "internal/config")
	if err != nil {
		return nil, err
	}
	var findings []string
	serverHas := false
	for _, f := range files {
		if !f.product {
			continue
		}
		for _, name := range []string{"SlackWebhookURL", "RequireSlackWebhookURL"} {
			if pos, ok := f.method("Config", name); ok {
				findings = append(findings, fmt.Sprintf("%s declares Config.%s", pos, name))
			}
		}
		if _, ok := f.method("ServerConfig", "WebhookURL"); ok {
			serverHas = true
		}
	}
	if !serverHas {
		findings = append(findings, "internal/config does not declare ServerConfig.WebhookURL")
	}
	return findings, nil
}

// checkMakeTarget checks that the CLI's Webhook integration target and test
// are gone.
func checkMakeTarget(_, head string) ([]string, error) {
	makefile, err := gitShow(head, "Makefile")
	if err != nil {
		return nil, err
	}
	var findings []string
	for i, line := range strings.Split(string(makefile), "\n") {
		for _, word := range []string{"test-integration-cli-slack", "CLI_SLACK_INTEGRATION_TIMEOUT"} {
			if strings.Contains(line, word) {
				findings = append(findings, fmt.Sprintf("Makefile:%d holds %s", i+1, word))
			}
		}
	}
	const test = "cmd/yt2column/integration_slack_test.go"
	out, err := exec.Command("git", "ls-tree", "--name-only", head, test).Output() //nolint:gosec // a commit this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s %s: %w", head, test, err)
	}
	if strings.TrimSpace(string(out)) != "" {
		findings = append(findings, test+" exists")
	}
	return findings, nil
}

// checkUnchanged checks that extension/ and internal/server's product files
// did not change between the commits.
func checkUnchanged(base, head string) ([]string, error) {
	var findings []string
	changed, err := changedFiles(base, head, "extension/")
	if err != nil {
		return nil, err
	}
	for _, name := range changed {
		findings = append(findings, name+" changed")
	}
	changed, err = changedFiles(base, head, "internal/server/")
	if err != nil {
		return nil, err
	}
	for _, name := range changed {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		product := false
		for _, commit := range []string{base, head} {
			data, err := gitShow(commit, name)
			if errors.Is(err, errMissing) {
				continue
			}
			if err != nil {
				return nil, err
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, name, data, parser.ParseComments|parser.PackageClauseOnly)
			if err != nil {
				return nil, fmt.Errorf("parse %s at %s: %w", name, commit, err)
			}
			if !hasTestConstraint(file) {
				product = true
			}
		}
		if product {
			findings = append(findings, name+" (a product file) changed")
		}
	}
	return findings, nil
}

// changedFiles returns the files under dir that differ between the commits.
func changedFiles(base, head, dir string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", "--no-renames", base, head, "--", dir).Output() //nolint:gosec // commits this tool was given
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", dir, err)
	}
	var names []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// errMissing reports a path that does not exist at a commit.
var errMissing = errors.New("missing at the commit")

// gitShow returns the content of rel at commit, or errMissing when rel does
// not exist there.
func gitShow(commit, rel string) ([]byte, error) {
	out, err := exec.Command("git", "ls-tree", "--name-only", commit, rel).Output() //nolint:gosec // a commit and path this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s %s: %w", commit, rel, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil, fmt.Errorf("%w: %s at %s", errMissing, rel, commit)
	}
	data, err := exec.Command("git", "show", commit+":"+rel).Output() //nolint:gosec // as above
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", commit, rel, err)
	}
	return data, nil
}

// allGoFiles parses every Go file at commit outside docs/tasks/ and outside
// testdata directories.
func allGoFiles(commit string) ([]goFile, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", commit).Output() //nolint:gosec // a commit this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree -r %s: %w", commit, err)
	}
	dirs := map[string]struct{}{}
	for name := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, "docs/tasks/") || strings.Contains("/"+name, "/testdata/") {
			continue
		}
		dirs[path.Dir(name)] = struct{}{}
	}
	var all []goFile
	for _, dir := range slices.Sorted(func(yield func(string) bool) {
		for d := range dirs {
			if !yield(d) {
				return
			}
		}
	}) {
		files, err := packageFiles(commit, dir)
		if err != nil {
			return nil, err
		}
		all = append(all, files...)
	}
	return all, nil
}

// goFile is one parsed Go file at a commit.
type goFile struct {
	path    string
	product bool // neither a _test.go file nor a file with a test build constraint
	fset    *token.FileSet
	file    *ast.File
}

// packageFiles parses every Go file directly in dir at commit.
func packageFiles(commit, dir string) ([]goFile, error) {
	prefix := dir + "/"
	if dir == "." {
		prefix = ""
	}
	out, err := exec.Command("git", "ls-tree", "--name-only", commit, prefix).Output() //nolint:gosec // a commit this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s %s: %w", commit, dir, err)
	}
	var files []goFile
	for name := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if !strings.HasSuffix(name, ".go") || path.Dir(name) != dir {
			continue
		}
		data, err := exec.Command("git", "show", commit+":"+name).Output() //nolint:gosec // as above
		if err != nil {
			return nil, fmt.Errorf("git show %s:%s: %w", commit, name, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, data, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s at %s: %w", name, commit, err)
		}
		files = append(files, goFile{
			path:    name,
			product: !strings.HasSuffix(name, "_test.go") && !hasTestConstraint(file),
			fset:    fset,
			file:    file,
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: %s at %s", errNoGoFile, dir, commit)
	}
	return files, nil
}

// hasTestConstraint reports whether the file's first //go:build line starts
// with the test tag.
func hasTestConstraint(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:build") {
				return testConstraint.MatchString(c.Text)
			}
		}
	}
	return false
}

// topLevel returns the position of the top-level declaration of name (a
// constant, variable, type, or function without a receiver).
func (f goFile) topLevel(name string) (token.Position, bool) {
	for _, decl := range f.file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == name {
				return f.fset.Position(d.Name.Pos()), true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						return f.fset.Position(s.Name.Pos()), true
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name == name {
							return f.fset.Position(n.Pos()), true
						}
					}
				}
			}
		}
	}
	return token.Position{}, false
}

// declares returns the position of the declaration of name: a top-level
// declaration, or a method of any receiver.
func (f goFile) declares(name string) (token.Position, bool) {
	if pos, ok := f.topLevel(name); ok {
		return pos, true
	}
	for _, decl := range f.file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv != nil && d.Name.Name == name {
			return f.fset.Position(d.Name.Pos()), true
		}
	}
	return token.Position{}, false
}

// method returns the position of the method name of the receiver type
// typeName (value or pointer receiver).
func (f goFile) method(typeName, name string) (token.Position, bool) {
	for _, decl := range f.file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok || d.Recv == nil || len(d.Recv.List) == 0 || d.Name.Name != name {
			continue
		}
		expr := d.Recv.List[0].Type
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		}
		if id, ok := expr.(*ast.Ident); ok && id.Name == typeName {
			return f.fset.Position(d.Name.Pos()), true
		}
	}
	return token.Position{}, false
}

// field returns the position of the field named field in the struct type
// named typeName declared at the top level.
func (f goFile) field(typeName, field string) (token.Position, bool) {
	for _, decl := range f.file.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range d.Specs {
			s, ok := spec.(*ast.TypeSpec)
			if !ok || s.Name.Name != typeName {
				continue
			}
			st, ok := s.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fl := range st.Fields.List {
				for _, n := range fl.Names {
					if n.Name == field {
						return f.fset.Position(n.Pos()), true
					}
				}
			}
		}
	}
	return token.Position{}, false
}

// qualifiedRefs returns the positions of the selectors pkg.Name, for each
// name in names, where pkg is the file's name for the import path importPath.
func (f goFile) qualifiedRefs(importPath string, names []string) []token.Position {
	local := ""
	for _, imp := range f.file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == importPath {
			local = path.Base(p)
			if imp.Name != nil {
				local = imp.Name.Name
			}
		}
	}
	if local == "" {
		return nil
	}
	var found []token.Position
	ast.Inspect(f.file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == local && slices.Contains(names, sel.Sel.Name) {
			found = append(found, f.fset.Position(sel.Pos()))
		}
		return true
	})
	return found
}

// flagDefinitions returns the positions of the calls x.Method(..., "name",
// ...) whose method defines a flag (flagDefiners) and whose name argument is
// the string literal name. The name is the first argument of Bool, String,
// ..., and the second of BoolVar, StringVar, ..., so any argument counts.
func (f goFile) flagDefinitions(name string) []token.Position {
	var found []token.Position
	ast.Inspect(f.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !slices.Contains(flagDefiners, sel.Sel.Name) {
			return true
		}
		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && v == name {
				found = append(found, f.fset.Position(call.Pos()))
			}
		}
		return true
	})
	return found
}

// stringLiteral is one string literal: its position and value.
type stringLiteral struct {
	pos   token.Position
	value string
}

// stringLiterals returns every string literal in the file, unquoted.
func (f goFile) stringLiterals() []stringLiteral {
	var found []stringLiteral
	ast.Inspect(f.file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			value = lit.Value
		}
		found = append(found, stringLiteral{pos: f.fset.Position(lit.Pos()), value: value})
		return true
	})
	return found
}

// textContaining returns the positions of the comments and string literals
// that contain word, ignoring case.
func (f goFile) textContaining(word string) []token.Position {
	word = strings.ToLower(word)
	var found []token.Position
	for _, group := range f.file.Comments {
		for _, c := range group.List {
			if strings.Contains(strings.ToLower(c.Text), word) {
				found = append(found, f.fset.Position(c.Pos()))
			}
		}
	}
	for _, lit := range f.stringLiterals() {
		if strings.Contains(strings.ToLower(lit.value), word) {
			found = append(found, lit.pos)
		}
	}
	return found
}
