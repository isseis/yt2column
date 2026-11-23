//go:build test

package ciskip

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	workflowRelPath   = "../../.github/workflows/ci.yml"
	cmdMainRelPath    = "../../cmd/ciskip/main.go"
	jobIndent         = 2
	jobFieldIndent    = 4
	stepItemIndent    = 6
	stepFieldIndent   = 8
	nestedFieldIndent = 6
	envEntryIndent    = 10
	checkJobTimeout   = "10"
	fakeMktempExit    = "3"
)

type ciStep struct {
	name    string
	id      string
	env     map[string]string
	runBody string
	hasRun  bool
}

type ciJob struct {
	id             string
	name           string
	timeoutMinutes string
	needs          []string
	ifCond         string
	hasIf          bool
	hasNeeds       bool
	hasPermissions bool
	permissions    map[string]string
	permScalar     string
	outputs        map[string]string
	steps          []ciStep
}

type ciWorkflow struct {
	rawLines      []string
	hasTopPerms   bool
	topPerms      map[string]string
	topPermScalar string
	jobs          map[string]*ciJob
	jobOrder      []string
}

func readWorkflow(t *testing.T) *ciWorkflow {
	t.Helper()
	data, err := os.ReadFile(workflowRelPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowRelPath, err)
	}
	return parseWorkflow(t, string(data))
}

func parseWorkflow(t *testing.T, content string) *ciWorkflow {
	t.Helper()
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	wf := &ciWorkflow{
		rawLines: lines,
		topPerms: make(map[string]string),
		jobs:     make(map[string]*ciJob),
	}

	inJobs := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		if indent == 0 {
			inJobs = false
			if line == "jobs:" {
				inJobs = true
				continue
			}
			if strings.HasPrefix(line, "permissions:") {
				wf.hasTopPerms = true
				rest := strings.TrimSpace(strings.TrimPrefix(line, "permissions:"))
				if rest != "" {
					wf.topPermScalar = rest
				} else {
					i = parseKeyValueMap(lines, i, jobIndent, wf.topPerms)
				}
			}
			continue
		}
		header := strings.TrimSpace(stripYAMLComment(line))
		if inJobs && indent == jobIndent && strings.HasSuffix(header, ":") {
			jobID := strings.TrimSuffix(header, ":")
			end := i + 1
			for end < len(lines) {
				next := lines[end]
				nt := strings.TrimSpace(next)
				if nt != "" && !strings.HasPrefix(nt, "#") && leadingSpaces(next) <= jobIndent {
					break
				}
				end++
			}
			job := parseJob(jobID, lines[i:end])
			wf.jobs[jobID] = job
			wf.jobOrder = append(wf.jobOrder, jobID)
			i = end - 1
		}
	}
	return wf
}

func parseJob(id string, lines []string) *ciJob {
	job := &ciJob{
		id:          id,
		permissions: make(map[string]string),
		outputs:     make(map[string]string),
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if leadingSpaces(line) != jobFieldIndent {
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "name":
			job.name = val
		case "timeout-minutes":
			job.timeoutMinutes = val
		case "if":
			job.hasIf = true
			job.ifCond = val
		case "needs":
			job.hasNeeds = true
			if val != "" {
				job.needs = parseInlineList(val)
			} else {
				i = parseBlockList(lines, i, nestedFieldIndent, &job.needs)
			}
		case "permissions":
			job.hasPermissions = true
			if val != "" {
				job.permScalar = val
			} else {
				i = parseKeyValueMap(lines, i, nestedFieldIndent, job.permissions)
			}
		case "outputs":
			i = parseKeyValueMap(lines, i, nestedFieldIndent, job.outputs)
		case "steps":
			steps, nextIdx := parseSteps(lines, i)
			job.steps = steps
			i = nextIdx
		}
	}
	return job
}

func parseSteps(lines []string, start int) ([]ciStep, int) {
	var steps []ciStep
	i := start + 1
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		indent := leadingSpaces(line)
		if indent < stepItemIndent {
			break
		}
		if indent == stepItemIndent && strings.HasPrefix(trimmed, "- ") {
			end := i + 1
			for end < len(lines) {
				next := lines[end]
				nt := strings.TrimSpace(next)
				if nt != "" && !strings.HasPrefix(nt, "#") && leadingSpaces(next) <= stepItemIndent {
					break
				}
				end++
			}
			steps = append(steps, parseOneStep(lines[i:end]))
			i = end
			continue
		}
		i++
	}
	return steps, i - 1
}

func parseOneStep(lines []string) ciStep {
	step := ciStep{env: make(map[string]string)}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		field := ""
		switch {
		case i == 0 && indent == stepItemIndent && strings.HasPrefix(trimmed, "- "):
			field = strings.TrimPrefix(trimmed, "- ")
		case indent == stepFieldIndent:
			field = trimmed
		default:
			continue
		}
		key, val, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "name":
			step.name = val
		case "id":
			step.id = val
		case "env":
			i = parseKeyValueMap(lines, i, envEntryIndent, step.env)
		case "run":
			step.hasRun = true
			if val == "|" || val == "|-" || val == "|+" || val == ">" || val == ">-" || val == ">+" {
				body, nextIdx := parseBlockScalar(lines, i, stepFieldIndent)
				step.runBody = body
				i = nextIdx
			} else {
				step.runBody = val
			}
		}
	}
	return step
}

func parseBlockScalar(lines []string, headerIdx, parentIndent int) (string, int) {
	var raw []string
	minIndent := -1
	i := headerIdx + 1
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			raw = append(raw, "")
			i++
			continue
		}
		indent := leadingSpaces(line)
		if indent <= parentIndent {
			break
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
		raw = append(raw, line)
		i++
	}
	for len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	if minIndent < 0 {
		return "", i - 1
	}
	dedented := make([]string, len(raw))
	for idx, l := range raw {
		if l == "" {
			dedented[idx] = ""
		} else {
			dedented[idx] = l[minIndent:]
		}
	}
	return strings.Join(dedented, "\n") + "\n", i - 1
}

func parseKeyValueMap(lines []string, headerIdx, entryIndent int, dst map[string]string) int {
	i := headerIdx + 1
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if leadingSpaces(line) != entryIndent {
			break
		}
		k, v, ok := strings.Cut(trimmed, ":")
		if ok {
			dst[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
		i++
	}
	return i - 1
}

func parseBlockList(lines []string, headerIdx, itemIndent int, dst *[]string) int {
	i := headerIdx + 1
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if leadingSpaces(line) != itemIndent || !strings.HasPrefix(trimmed, "- ") {
			break
		}
		*dst = append(*dst, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		i++
	}
	return i - 1
}

func parseInlineList(val string) []string {
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		inner := strings.TrimSpace(val[1 : len(val)-1])
		if inner == "" {
			return nil
		}
		parts := strings.Split(inner, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, strings.TrimSpace(p))
		}
		return out
	}
	return []string{val}
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func findStepByID(t *testing.T, job *ciJob, id string) ciStep {
	t.Helper()
	var found []ciStep
	for _, s := range job.steps {
		if s.id == id {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("job %q has %d steps with id %q, want 1", job.id, len(found), id)
	}
	return found[0]
}

// extractCLIFlagsFromMain parses cmd/ciskip/main.go and returns all flag names
// registered on the FlagSet in parseArgs.
func extractCLIFlagsFromMain(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, cmdMainRelPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", cmdMainRelPath, err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "parseArgs" {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok || recv.Name != "flags" {
				return true
			}
			var argIdx int
			switch sel.Sel.Name {
			case "StringVar", "BoolVar", "IntVar", "Int64Var", "UintVar", "Uint64Var", "Float64Var", "DurationVar", "TextVar", "Var":
				argIdx = 1
			case "String", "Bool", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration", "Func", "BoolFunc":
				argIdx = 0
			case "SetOutput", "Parse", "NArg":
				return true
			default:
				t.Fatalf("unexpected flags.%s call in %s parseArgs; update extractCLIFlagsFromMain", sel.Sel.Name, cmdMainRelPath)
				return false
			}
			if len(call.Args) <= argIdx {
				t.Fatalf("flags.%s call has %d args, need index %d", sel.Sel.Name, len(call.Args), argIdx)
			}
			lit, ok := call.Args[argIdx].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Fatalf("flags.%s flag name argument must be a string literal", sel.Sel.Name)
			}
			unquoted, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("unquote flag name %s: %v", lit.Value, err)
			}
			names = append(names, unquoted)
			return true
		})
		return false
	})
	if len(names) == 0 {
		t.Fatalf("no flags found in %s parseArgs", cmdMainRelPath)
	}
	slices.Sort(names)
	return names
}

// extractWorkflowCLIFlags extracts all --<name>= flags passed to `go run ./cmd/ciskip`
// in the check step's run body, failing if the invocation is missing or any flag repeats.
func extractWorkflowCLIFlags(t *testing.T, runBody string) []string {
	t.Helper()
	lines := strings.Split(runBody, "\n")
	var commands []string
	for i := 0; i < len(lines); i++ {
		if !strings.Contains(lines[i], "go run ./cmd/ciskip") {
			continue
		}
		var b strings.Builder
		for {
			b.WriteString(lines[i])
			b.WriteByte('\n')
			if !strings.HasSuffix(strings.TrimRight(lines[i], " \t"), `\`) || i+1 >= len(lines) {
				break
			}
			i++
		}
		commands = append(commands, b.String())
	}
	if len(commands) != 1 {
		t.Fatalf("found %d `go run ./cmd/ciskip` invocations in check step, want 1", len(commands))
	}
	flagRe := regexp.MustCompile(`--([a-z0-9-]+)=`)
	submatches := flagRe.FindAllStringSubmatch(commands[0], -1)
	seen := make(map[string]bool, len(submatches))
	var names []string
	for _, m := range submatches {
		name := m[1]
		if seen[name] {
			t.Fatalf("duplicate CLI flag --%s in ci.yml check step", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func TestCIWorkflowCheckChangesPin(t *testing.T) {
	wf := readWorkflow(t)
	checkJob := wf.jobs["check-changes"]
	if checkJob == nil {
		t.Fatal("ci.yml has no check-changes job")
	}

	t.Run("non_code_regex_matches_isNonCodePath", func(t *testing.T) {
		normal := findStepByID(t, checkJob, "normal")
		reExtract := regexp.MustCompile(`grep -vE '([^']+)'`)
		matches := reExtract.FindAllStringSubmatch(normal.runBody, -1)
		if len(matches) != 1 {
			t.Fatalf("found %d `grep -vE '...'` patterns in normal step, want 1", len(matches))
		}
		compiled, err := regexp.Compile(matches[0][1])
		if err != nil {
			t.Fatalf("regexp.Compile(%q): %v", matches[0][1], err)
		}

		samplePaths := []string{
			"",
			".md",
			"README.md",
			"README.en.md",
			"a.md",
			"nested/dir/notes.md",
			"docs/dev/security.md",
			"docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh",
			".claude/commands/runplan.md",
			".claude/settings.json",
			"LICENSE",
			"LICENSE.txt",
			"MY_LICENSE",
			"cmd",
			"foo_md",
			"xclaude/a.go",
			"docs",
			".claude",
			"internal/ciskip/ciskip.go",
			"cmd/ciskip/main.go",
			".github/workflows/ci.yml",
			"Makefile",
			"go.mod",
			"go.sum",
			"my_docs/file.go",
			"foo/docs/file.go",
			"foo/.claude/file.go",
			"notes.md.bak",
			"notes.mdx",
			"extension/src/index.ts",
			"extension/scripts/has-extension-changes.sh",
			".gitignore",
		}
		var nonCodeCount, codeCount int
		for _, p := range samplePaths {
			gotRegex := compiled.MatchString(p)
			gotGo := isNonCodePath(p)
			if gotRegex != gotGo {
				t.Errorf("path %q: ci.yml regex=%v, isNonCodePath=%v", p, gotRegex, gotGo)
			}
			if gotGo {
				nonCodeCount++
			} else {
				codeCount++
			}
		}
		if nonCodeCount == 0 || codeCount == 0 {
			t.Fatalf("sample paths must cover both non-code (%d) and code (%d)", nonCodeCount, codeCount)
		}
	})

	t.Run("required_job_names_match_ci_workflow", func(t *testing.T) {
		var workflowNames []string
		seen := make(map[string]bool)
		for _, id := range wf.jobOrder {
			job := wf.jobs[id]
			if id != "check-changes" && !slices.Contains(job.needs, "check-changes") {
				continue
			}
			if job.name == "" {
				t.Fatalf("job %q has empty name", id)
			}
			if seen[job.name] {
				t.Fatalf("duplicate job display name %q in ci.yml", job.name)
			}
			seen[job.name] = true
			workflowNames = append(workflowNames, job.name)
		}
		slices.Sort(workflowNames)
		want := slices.Clone(requiredJobNames)
		slices.Sort(want)
		if !slices.Equal(workflowNames, want) {
			t.Errorf("ci.yml check-changes + dependents = %v, internal/ciskip requiredJobNames = %v", workflowNames, want)
		}
	})

	t.Run("always_run_and_heavy_job_conditions", func(t *testing.T) {
		for _, alwaysID := range []string{"doc-guard", "secret-scan"} {
			job := wf.jobs[alwaysID]
			if job == nil {
				t.Fatalf("ci.yml is missing always-run job %q", alwaysID)
			}
			if job.hasNeeds {
				t.Errorf("always-run job %q must not declare needs (got %v)", alwaysID, job.needs)
			}
			if job.hasIf {
				t.Errorf("always-run job %q must not declare if (got %q)", alwaysID, job.ifCond)
			}
		}

		heavyWant := map[string]string{
			"test":      "needs.check-changes.outputs.has-code-changes == 'true'",
			"lint":      "needs.check-changes.outputs.has-code-changes == 'true'",
			"extension": "needs.check-changes.outputs.has-extension-changes == 'true'",
		}
		for id, wantIf := range heavyWant {
			job := wf.jobs[id]
			if job == nil {
				t.Fatalf("ci.yml is missing heavy job %q", id)
			}
			if !slices.Equal(job.needs, []string{"check-changes"}) {
				t.Errorf("job %q needs = %v, want [check-changes]", id, job.needs)
			}
			if !job.hasIf || job.ifCond != wantIf {
				t.Errorf("job %q if = %q, want %q", id, job.ifCond, wantIf)
			}
		}

		wantOutputs := map[string]string{
			"has-code-changes":      "${{ steps.check.outputs.has-code-changes }}",
			"has-extension-changes": "${{ steps.check.outputs.has-extension-changes }}",
		}
		if !maps.Equal(checkJob.outputs, wantOutputs) {
			t.Errorf("check-changes outputs = %v, want %v", checkJob.outputs, wantOutputs)
		}
	})

	t.Run("check_changes_wiring_and_cli_flags", func(t *testing.T) {
		if checkJob.timeoutMinutes != checkJobTimeout {
			t.Errorf("check-changes timeout-minutes = %q, want %q", checkJob.timeoutMinutes, checkJobTimeout)
		}

		for _, step := range checkJob.steps {
			if step.hasRun && strings.Contains(step.runBody, "${{") {
				t.Errorf("check-changes step %q embeds ${{ directly in run body", step.name)
			}
		}

		normal := findStepByID(t, checkJob, "normal")
		if _, hasToken := normal.env["GH_TOKEN"]; hasToken {
			t.Error("normal classification step must not receive GH_TOKEN")
		}
		normalDepDirsIdx := strings.Index(normal.runBody, `dep_dirs="$RUNNER_TEMP/wasm-dep-dirs"`)
		normalInitIdx := strings.Index(normal.runBody, "wasm_dep_dirs_ok=false\n")
		normalIfIdx := strings.Index(normal.runBody, `if bash extension/scripts/wasm-dep-dirs.sh > "$dep_dirs"; then`)
		normalTrueIdx := strings.Index(normal.runBody, "wasm_dep_dirs_ok=true\n")
		normalElseIdx := -1
		if normalIfIdx >= 0 {
			if rel := strings.Index(normal.runBody[normalIfIdx:], "\nelse\n"); rel >= 0 {
				normalElseIdx = normalIfIdx + rel
			}
		}
		if normalDepDirsIdx < 0 {
			t.Error(`normal step must define dep_dirs="$RUNNER_TEMP/wasm-dep-dirs"`)
		}
		if normalInitIdx < 0 || normalIfIdx < 0 || normalTrueIdx < 0 || normalElseIdx < 0 ||
			normalInitIdx >= normalIfIdx || normalIfIdx >= normalTrueIdx || normalTrueIdx >= normalElseIdx {
			t.Errorf(
				"normal step must initialize wasm_dep_dirs_ok=false before if and set wasm_dep_dirs_ok=true inside then before else (indices: init=%d if=%d true=%d else=%d)",
				normalInitIdx, normalIfIdx, normalTrueIdx, normalElseIdx,
			)
		}
		if !strings.Contains(normal.runBody, `echo "wasm-dep-dirs-ok=$wasm_dep_dirs_ok" >> "$GITHUB_OUTPUT"`) {
			t.Error("normal step does not write wasm-dep-dirs-ok to GITHUB_OUTPUT")
		}

		check := findStepByID(t, checkJob, "check")
		wantEnv := map[string]string{
			"GH_TOKEN":                "${{ secrets.GITHUB_TOKEN }}",
			"GOTOOLCHAIN":             "local",
			"CISKIP_NORMAL_CODE":      "${{ steps.normal.outputs.has-code-changes }}",
			"CISKIP_NORMAL_EXT":       "${{ steps.normal.outputs.has-extension-changes }}",
			"CISKIP_WASM_DEP_DIRS_OK": "${{ steps.normal.outputs.wasm-dep-dirs-ok }}",
			"CISKIP_REPO":             "${{ github.repository }}",
			"CISKIP_HEAD_REPO":        "${{ github.event.pull_request.head.repo.full_name }}",
			"CISKIP_BASE_REPO":        "${{ github.event.pull_request.base.repo.full_name }}",
			"CISKIP_BASE_REF":         "${{ github.event.pull_request.base.ref }}",
			"CISKIP_PR_NUMBER":        "${{ github.event.pull_request.number }}",
			"CISKIP_HEAD_SHA":         "${{ github.event.pull_request.head.sha }}",
			"CISKIP_RUN_ATTEMPT":      "${{ github.run_attempt }}",
		}
		if !maps.Equal(check.env, wantEnv) {
			t.Errorf("check step env = %v, want %v", check.env, wantEnv)
		}

		wfFlags := extractWorkflowCLIFlags(t, check.runBody)
		cliFlags := extractCLIFlagsFromMain(t)
		if !slices.Equal(wfFlags, cliFlags) {
			t.Errorf("ci.yml check step flags = %v, cmd/ciskip flags = %v", wfFlags, cliFlags)
		}
	})
}

func TestCIWorkflowPermissions(t *testing.T) {
	wf := readWorkflow(t)

	if !wf.hasTopPerms || wf.topPermScalar != "" {
		t.Fatalf("top-level permissions missing or scalar (%q)", wf.topPermScalar)
	}
	wantTop := map[string]string{"contents": "read"}
	if !maps.Equal(wf.topPerms, wantTop) {
		t.Errorf("top-level permissions = %v, want %v", wf.topPerms, wantTop)
	}

	checkJob := wf.jobs["check-changes"]
	if checkJob == nil {
		t.Fatal("ci.yml has no check-changes job")
	}
	if !checkJob.hasPermissions || checkJob.permScalar != "" {
		t.Fatalf("check-changes permissions missing or scalar (%q)", checkJob.permScalar)
	}
	wantCheck := map[string]string{
		"contents": "read",
		"actions":  "read",
		"checks":   "read",
	}
	if !maps.Equal(checkJob.permissions, wantCheck) {
		t.Errorf("check-changes permissions = %v, want %v", checkJob.permissions, wantCheck)
	}

	for _, id := range wf.jobOrder {
		if id == "check-changes" {
			continue
		}
		if wf.jobs[id].hasPermissions {
			t.Errorf("job %q must not declare permissions", id)
		}
	}

	forbidden := regexp.MustCompile(`(?i):\s*write\b|\bwrite-all\b|\bread-all\b`)
	for idx, line := range wf.rawLines {
		code := stripYAMLComment(line)
		if forbidden.MatchString(code) {
			t.Errorf("ci.yml:%d contains forbidden permission token: %q", idx+1, line)
		}
	}
}

// stripYAMLComment removes an unquoted '#' comment suffix from a YAML line.
func stripYAMLComment(line string) string {
	inSingle := false
	inDouble := false
	for i := range len(line) {
		c := line[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if c == '#' && !inSingle && !inDouble {
			return line[:i]
		}
	}
	return line
}

type checkStepRunResult struct {
	stdout  string
	stderr  string
	outputs map[string]string
	goCalls int
	goArgs  []string
	tempDir string
}

func runCheckStepScript(
	t *testing.T,
	script string,
	envOverrides map[string]string,
	fakeGoScript string,
	fakeMktempScript string,
) checkStepRunResult {
	t.Helper()

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	runnerTemp := filepath.Join(root, "runner-temp")
	ghOutput := filepath.Join(root, "github-output")
	goLog := filepath.Join(root, "go-calls.log")
	goArgsFile := filepath.Join(root, "go-args.log")

	for _, dir := range []string{binDir, runnerTemp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(ghOutput, nil, 0o600); err != nil {
		t.Fatalf("init github output: %v", err)
	}

	writeExec := func(name, body string) {
		p := filepath.Join(binDir, name)
		if err := os.WriteFile(p, []byte(body), 0o700); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	writeExec("go", fakeGoScript)
	if fakeMktempScript != "" {
		writeExec("mktemp", fakeMktempScript)
	}

	envMap := map[string]string{
		"PATH":                    binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"RUNNER_TEMP":             runnerTemp,
		"GITHUB_OUTPUT":           ghOutput,
		"FAKE_GO_LOG":             goLog,
		"FAKE_GO_ARGS":            goArgsFile,
		"CISKIP_NORMAL_CODE":      "true",
		"CISKIP_NORMAL_EXT":       "false",
		"CISKIP_WASM_DEP_DIRS_OK": "true",
		"CISKIP_REPO":             testRepo,
		"CISKIP_HEAD_REPO":        testRepo,
		"CISKIP_BASE_REPO":        testRepo,
		"CISKIP_BASE_REF":         "main",
		"CISKIP_PR_NUMBER":        testPRNumber,
		"CISKIP_HEAD_SHA":         testSHA2,
		"CISKIP_RUN_ATTEMPT":      "1",
	}
	for k, v := range envOverrides {
		if v == "\x00UNSET" {
			delete(envMap, k)
		} else {
			envMap[k] = v
		}
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}

	cmd := exec.CommandContext(t.Context(), "bash", "-eo", "pipefail", "-c", script)
	cmd.Dir = root
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("check step script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	outBytes, err := os.ReadFile(ghOutput)
	if err != nil {
		t.Fatalf("read GITHUB_OUTPUT: %v", err)
	}
	outputs := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSuffix(string(outBytes), "\n"), "\n") {
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("malformed GITHUB_OUTPUT line %q", line)
		}
		if prev, dup := outputs[k]; dup {
			t.Fatalf("duplicate GITHUB_OUTPUT key %q (previous=%q, next=%q)", k, prev, v)
		}
		outputs[k] = v
	}

	goCalls := 0
	if data, err := os.ReadFile(goLog); err == nil {
		for l := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			if l != "" {
				goCalls++
			}
		}
	}
	var goArgs []string
	if data, err := os.ReadFile(goArgsFile); err == nil {
		for l := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
			if l != "" {
				goArgs = append(goArgs, l)
			}
		}
	}

	return checkStepRunResult{
		stdout:  stdout.String(),
		stderr:  stderr.String(),
		outputs: outputs,
		goCalls: goCalls,
		goArgs:  goArgs,
		tempDir: runnerTemp,
	}
}

// fakeGoHelper builds a fake `go` executable that logs calls and arguments,
// extracts --result-file, runs extraBody, and exits with exitCode.
func fakeGoHelper(extraBody string, exitCode int) string {
	return "#!/usr/bin/env bash\n" +
		"set -eu\n" +
		"echo called >> \"$FAKE_GO_LOG\"\n" +
		"printf '%s\\n' \"$@\" > \"$FAKE_GO_ARGS\"\n" +
		"result_file=\"\"\n" +
		"for arg in \"$@\"; do\n" +
		"  case \"$arg\" in\n" +
		"    --result-file=*) result_file=\"${arg#--result-file=}\" ;;\n" +
		"  esac\n" +
		"done\n" +
		extraBody + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
}

func assertOutputs(t *testing.T, got map[string]string, wantCode, wantExt string) {
	t.Helper()
	want := map[string]string{
		"has-code-changes":      wantCode,
		"has-extension-changes": wantExt,
	}
	if !maps.Equal(got, want) {
		t.Errorf("GITHUB_OUTPUT = %v, want %v", got, want)
	}
}

func TestCIWorkflowCheckStepExecution(t *testing.T) {
	wf := readWorkflow(t)
	checkJob := wf.jobs["check-changes"]
	if checkJob == nil {
		t.Fatal("ci.yml has no check-changes job")
	}
	checkStep := findStepByID(t, checkJob, "check")
	script := checkStep.runBody

	t.Run("both_false_skips_go_invocation", func(t *testing.T) {
		res := runCheckStepScript(
			t,
			script,
			map[string]string{
				"CISKIP_NORMAL_CODE": "false",
				"CISKIP_NORMAL_EXT":  "false",
			},
			fakeGoHelper("echo 'go should not be called' >&2", 1),
			"",
		)
		if res.goCalls != 0 {
			t.Errorf("go was called %d times, want 0", res.goCalls)
		}
		assertOutputs(t, res.outputs, "false", "false")
	})

	t.Run("empty_and_invalid_normal_outputs_normalize_to_true", func(t *testing.T) {
		cases := []struct {
			name     string
			codeEnv  string
			extEnv   string
			wantCode string
			wantExt  string
		}{
			{name: "empty_code_false_ext", codeEnv: "", extEnv: "false", wantCode: "true", wantExt: "false"},
			{name: "false_code_invalid_ext", codeEnv: "false", extEnv: "maybe", wantCode: "false", wantExt: "true"},
			{name: "both_unset", codeEnv: "\x00UNSET", extEnv: "\x00UNSET", wantCode: "true", wantExt: "true"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res := runCheckStepScript(
					t,
					script,
					map[string]string{
						"CISKIP_NORMAL_CODE": tc.codeEnv,
						"CISKIP_NORMAL_EXT":  tc.extEnv,
					},
					fakeGoHelper(`printf 'skip=false\n' > "$result_file"`, 0),
					"",
				)
				if res.goCalls != 1 {
					t.Errorf("go calls = %d, want 1", res.goCalls)
				}
				assertOutputs(t, res.outputs, tc.wantCode, tc.wantExt)
			})
		}
	})

	t.Run("skip_true_outputs_false_and_passes_stdout", func(t *testing.T) {
		const (
			distinctRepo     = "owner/repo"
			distinctHeadRepo = "fork-owner/fork-repo"
			distinctBaseRepo = "base-owner/base-repo"
		)
		noticeLine := "::notice title=ci-base-sha::" + testSHA1
		diagLine := "ciskip: decision=skip reason=ok parent=444444444444 run=5001 base=111111111111"
		fakeGo := fakeGoHelper(
			"printf '%s\\n' '"+noticeLine+"'\n"+
				"printf '%s\\n' '"+diagLine+"'\n"+
				`printf 'skip=true\n' > "$result_file"`,
			0,
		)
		res := runCheckStepScript(
			t,
			script,
			map[string]string{
				"CISKIP_NORMAL_CODE": "true",
				"CISKIP_NORMAL_EXT":  "true",
				"CISKIP_REPO":        distinctRepo,
				"CISKIP_HEAD_REPO":   distinctHeadRepo,
				"CISKIP_BASE_REPO":   distinctBaseRepo,
			},
			fakeGo,
			"",
		)
		if res.goCalls != 1 {
			t.Fatalf("go calls = %d, want 1", res.goCalls)
		}
		assertOutputs(t, res.outputs, "false", "false")
		if !strings.Contains(res.stdout, noticeLine+"\n") {
			t.Errorf("stdout missing notice line %q:\n%s", noticeLine, res.stdout)
		}
		if !strings.Contains(res.stdout, diagLine+"\n") {
			t.Errorf("stdout missing diagnostic line %q:\n%s", diagLine, res.stdout)
		}

		// Verify every distinct env var lands on its own CLI flag.
		wantPrefix := []string{
			"run",
			"./cmd/ciskip",
			"--repo=" + distinctRepo,
			"--head-repo=" + distinctHeadRepo,
			"--base-repo=" + distinctBaseRepo,
			"--base-ref=main",
			"--pr-number=" + testPRNumber,
			"--head-sha=" + testSHA2,
			"--run-attempt=1",
			"--dep-dirs=" + filepath.Join(res.tempDir, "wasm-dep-dirs"),
			"--wasm-dep-dirs-ok=true",
		}
		if len(res.goArgs) != len(wantPrefix)+1 {
			t.Fatalf("go args = %v, want %d entries", res.goArgs, len(wantPrefix)+1)
		}
		if !slices.Equal(res.goArgs[:len(wantPrefix)], wantPrefix) {
			t.Errorf("go args prefix = %v, want %v", res.goArgs[:len(wantPrefix)], wantPrefix)
		}
		if !strings.HasPrefix(res.goArgs[len(wantPrefix)], "--result-file="+filepath.Join(res.tempDir, "ciskip-result.")) {
			t.Errorf("unexpected --result-file arg: %q", res.goArgs[len(wantPrefix)])
		}

		// Verify the temporary result file was cleaned up on EXIT.
		entries, err := os.ReadDir(res.tempDir)
		if err != nil {
			t.Fatalf("read tempDir: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("temporary result file leaked in RUNNER_TEMP: %v", entries)
		}
	})

	t.Run("skip_false_preserves_normal_classification", func(t *testing.T) {
		cases := []struct {
			name     string
			code     string
			ext      string
			wasmOK   string
			wantWasm string
		}{
			{name: "code_only", code: "true", ext: "false", wasmOK: "true", wantWasm: "--wasm-dep-dirs-ok=true"},
			{name: "wasm_dep_dirs_failed_fallback", code: "false", ext: "true", wasmOK: "false", wantWasm: "--wasm-dep-dirs-ok=false"},
			{name: "code_and_ext_true_wasm_dep_dirs_failed", code: "true", ext: "true", wasmOK: "false", wantWasm: "--wasm-dep-dirs-ok=false"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				diagLine := "ciskip: decision=run reason=code_changed"
				fakeGo := fakeGoHelper(
					"printf '%s\\n' '"+diagLine+"'\n"+
						`printf 'skip=false\n' > "$result_file"`,
					0,
				)
				res := runCheckStepScript(
					t,
					script,
					map[string]string{
						"CISKIP_NORMAL_CODE":      tc.code,
						"CISKIP_NORMAL_EXT":       tc.ext,
						"CISKIP_WASM_DEP_DIRS_OK": tc.wasmOK,
					},
					fakeGo,
					"",
				)
				if res.goCalls != 1 {
					t.Fatalf("go calls = %d, want 1", res.goCalls)
				}
				assertOutputs(t, res.outputs, tc.code, tc.ext)
				if !slices.Contains(res.goArgs, tc.wantWasm) {
					t.Errorf("go args %v missing %q", res.goArgs, tc.wantWasm)
				}
				if !strings.Contains(res.stdout, diagLine+"\n") {
					t.Errorf("stdout missing diagnostic line:\n%s", res.stdout)
				}
				if strings.Contains(res.stdout, "fallback to normal classification") {
					t.Errorf("unexpected fallback log on clean skip=false:\n%s", res.stdout)
				}
			})
		}
	})

	t.Run("unexpected_result_file_falls_back", func(t *testing.T) {
		cases := []struct {
			name       string
			goBody     string
			secretText string
			wantStatus string
		}{
			{name: "empty_file", goBody: `: > "$result_file"`, wantStatus: "status=1"},
			{name: "missing_newline", goBody: `printf 'skip=true' > "$result_file"`, wantStatus: "status=1"},
			{name: "extra_line", goBody: `printf 'skip=true\nSECRET_VALUE\n' > "$result_file"`, secretText: "SECRET_VALUE", wantStatus: "status=1"},
			{name: "deleted_file", goBody: `rm -f "$result_file"`, wantStatus: "status=2"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res := runCheckStepScript(
					t,
					script,
					map[string]string{
						"CISKIP_NORMAL_CODE": "true",
						"CISKIP_NORMAL_EXT":  "false",
					},
					fakeGoHelper(tc.goBody, 0),
					"",
				)
				assertOutputs(t, res.outputs, "true", "false")
				if !strings.Contains(res.stdout, "fallback to normal classification") || !strings.Contains(res.stdout, tc.wantStatus) {
					t.Errorf("stdout missing fallback log with %s:\n%s", tc.wantStatus, res.stdout)
				}
				if tc.secretText != "" && (strings.Contains(res.stdout, tc.secretText) || strings.Contains(res.stderr, tc.secretText)) {
					t.Errorf("output leaked unexpected result-file content %q:\nstdout:\n%s\nstderr:\n%s", tc.secretText, res.stdout, res.stderr)
				}
			})
		}
	})

	t.Run("go_nonzero_exit_falls_back", func(t *testing.T) {
		for _, code := range []int{1, 2} {
			t.Run("exit_"+strconv.Itoa(code), func(t *testing.T) {
				// Write skip=true even before exiting non-zero to prove the exit code gates reading the file.
				res := runCheckStepScript(
					t,
					script,
					map[string]string{
						"CISKIP_NORMAL_CODE": "true",
						"CISKIP_NORMAL_EXT":  "true",
					},
					fakeGoHelper(`printf 'skip=true\n' > "$result_file"`, code),
					"",
				)
				assertOutputs(t, res.outputs, "true", "true")
				wantStatus := "status=" + strconv.Itoa(code)
				if !strings.Contains(res.stdout, "fallback to normal classification") || !strings.Contains(res.stdout, wantStatus) {
					t.Errorf("stdout missing fallback log with %s:\n%s", wantStatus, res.stdout)
				}
			})
		}
	})

	t.Run("mktemp_failure_falls_back", func(t *testing.T) {
		cases := []struct {
			name       string
			fakeMktemp string
			wantStatus string
		}{
			{name: "nonzero_exit", fakeMktemp: "#!/usr/bin/env bash\nexit " + fakeMktempExit + "\n", wantStatus: "status=" + fakeMktempExit},
			{name: "empty_stdout", fakeMktemp: "#!/usr/bin/env bash\nexit 0\n", wantStatus: "status=0"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res := runCheckStepScript(
					t,
					script,
					map[string]string{
						"CISKIP_NORMAL_CODE": "true",
						"CISKIP_NORMAL_EXT":  "false",
					},
					fakeGoHelper("echo 'go should not be called' >&2", 1),
					tc.fakeMktemp,
				)
				if res.goCalls != 0 {
					t.Errorf("go was called %d times after mktemp failure, want 0", res.goCalls)
				}
				assertOutputs(t, res.outputs, "true", "false")
				if !strings.Contains(res.stdout, "fallback to normal classification") || !strings.Contains(res.stdout, tc.wantStatus) {
					t.Errorf("stdout missing fallback log with %s:\n%s", tc.wantStatus, res.stdout)
				}
			})
		}
	})
}
