//go:build test

package main

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
)

// Paths from cmd/yt2column to the documents these tests check.
const (
	readmePath            = "../../README.md"
	cliDocPath            = "../../docs/dev/guides/cli.md"
	developmentDocPath    = "../../docs/dev/guides/development.md"
	projectOverviewPath   = "../../docs/dev/project_overview.md"
	securityPath          = "../../docs/dev/security.md"
	productionProfilePath = "../../docs/dev/production_profile.md"
	promptsReadmePath     = "../../prompts/README.md"
	reviewEvaluationPath  = "../../docs/tasks/0010_post_generation_review/02_evaluation.md"
	pricingRatesPath      = "../../internal/pricing/rates.go"
)

// The headings test (a) and test (b) read.
const (
	productionProfileSection = "## 本番の構成"
	quickStartHeading        = "## Quick start"
	recommendedConfigHeading = "### Recommended configuration"
)

// markdownLinkRE matches an inline Markdown link and captures its text and
// target. These documents use no reference links, and a link text or target
// holds no nested bracket or closing parenthesis.
var markdownLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// markdownLinkTargets returns the target of every inline Markdown link outside
// a fenced code block in text, in order. A link-like string inside an example
// is not a link.
func markdownLinkTargets(text string) []string {
	var targets []string
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(text))) {
		if line.Block {
			continue
		}
		for _, match := range markdownLinkRE.FindAllStringSubmatch(line.Text, -1) {
			targets = append(targets, match[2])
		}
	}
	return targets
}

// linkOpenerCount returns how many Markdown link openers doc has outside
// fenced code blocks. A link form the extraction drops would leave the count
// above the number of targets it found.
func linkOpenerCount(doc string) int {
	openers := 0
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		if !line.Block {
			openers += strings.Count(line.Text, "](")
		}
	}
	return openers
}

// checkLinkCount checks that the number of Markdown link openers outside code
// blocks in doc equals the number of targets the extraction found, so a link
// form it drops cannot go unchecked.
func checkLinkCount(t *testing.T, name, doc string, targets int) {
	t.Helper()
	if openers := linkOpenerCount(doc); openers != targets {
		t.Errorf("%s has %d link openers outside code blocks but the extraction found %d targets; a link form it drops would go unchecked", name, openers, targets)
	}
}

// headingAnchors returns the set of anchors the heading lines of doc outside
// code blocks produce. It fails the test when two headings share an anchor:
// the suffix GitHub adds to a repeated heading is not modelled.
func headingAnchors(t *testing.T, docName, doc string) map[string]struct{} {
	t.Helper()
	anchors := make(map[string]struct{})
	for _, heading := range doctestutil.HeadingsOutsideBlocks(doc) {
		anchor := doctestutil.AnchorFromHeading(heading)
		if _, seen := anchors[anchor]; seen {
			t.Errorf("%s has two headings with the anchor %q, which this helper does not model", docName, anchor)
		}
		anchors[anchor] = struct{}{}
	}
	return anchors
}

// numberedItemRE matches the first line of a top-level numbered list item.
var numberedItemRE = regexp.MustCompile(`^\d+\.\s`)

// numberedItems returns the top-level numbered list items of section. An item
// starts at a line outside a code block that begins with a number and a period
// at column 0, and continues to the next item or the next blank line.
func numberedItems(section string) []string {
	var items, current []string
	flush := func() {
		if len(current) > 0 {
			items = append(items, strings.Join(current, "\n"))
			current = nil
		}
	}
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(section))) {
		switch {
		case line.Block:
			if current != nil {
				current = append(current, line.Text)
			}
		case numberedItemRE.MatchString(line.Text):
			flush()
			current = []string{line.Text}
		case strings.TrimSpace(line.Text) == "":
			flush()
		case current != nil:
			current = append(current, line.Text)
		}
	}
	flush()
	return items
}

// TestREADMEDocumentsFlags checks that the CLI guide (cli.md) documents every
// flag run defines. The flag names come from newFlagSet, not from a list kept
// here, so a new flag fails this test until cli.md names it.
func TestREADMEDocumentsFlags(t *testing.T) {
	doc := doctestutil.ReadDoc(t, cliDocPath)
	table := doctestutil.FindTable(doc, doctestutil.CLIFlagColumn)
	if table == nil {
		t.Fatal("cli.md has no flags table")
	}
	fs, _ := newFlagSet()
	fs.VisitAll(func(f *flag.Flag) {
		if !tableFlagContains(table, "--"+f.Name) {
			t.Errorf("flags table does not document --%s", f.Name)
		}
	})
}

// TestSlackDocsContract pins the machine-checkable contract values of webhook
// publishing: the test environment variables and the make targets, which the
// development guide (development.md) and security.md name, and the per-message
// limit, which README.md and project_overview.md state. The values come from
// the publisher's exported test constants, not from the documents.
func TestSlackDocsContract(t *testing.T) {
	// The documents state the limit with a thousands separator, so the
	// literal below is pinned to the publisher's own exported test constant:
	// a changed limit fails this test, not only the integration size check.
	if publisher.SlackMaxMessageRunesForTest != 16383 {
		t.Fatalf("the publisher per-message limit is %d, not 16,383; update the documents and this test", publisher.SlackMaxMessageRunesForTest)
	}
	values := []string{
		publishertestutil.SlackOptInEnv,
		publishertestutil.CLISlackOptInEnv,
		publishertestutil.WebhookURLEnv,
		publishertestutil.SlackIntegrationOptions.MakeTarget,
		publishertestutil.CLISlackIntegrationOptions.MakeTarget,
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"development.md", doctestutil.ReadDoc(t, developmentDocPath)},
		{"security.md", doctestutil.ReadDoc(t, securityPath)},
	} {
		requireMentions(t, doc.name, doc.text, values)
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"README.md", doctestutil.ReadDoc(t, readmePath)},
		{"project_overview.md", doctestutil.ReadDoc(t, projectOverviewPath)},
	} {
		if !strings.Contains(doc.text, "16,383") {
			t.Errorf("%s does not state the per-message limit 16,383", doc.name)
		}
	}
}

// requireMentions fails the test for each value that text does not contain.
func requireMentions(t *testing.T, docName, text string, values []string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(text, value) {
			t.Errorf("%s does not mention %q", docName, value)
		}
	}
}

// TestClaudeDocsContract pins the machine-checkable contract values of the
// Claude provider: the integration test's environment variables. They come
// from the claude test package's exported constants, not from the documents.
func TestClaudeDocsContract(t *testing.T) {
	values := []string{
		claudetestutil.APIKeyEnv,
		claudetestutil.WorkspaceIDEnv,
		claudetestutil.OptInEnv,
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"development.md", doctestutil.ReadDoc(t, developmentDocPath)},
		{"security.md", doctestutil.ReadDoc(t, securityPath)},
	} {
		requireMentions(t, doc.name, doc.text, values)
	}
}

// TestDeepSeekDocsContract pins the machine-checkable contract values of the
// DeepSeek integration tests: the key they read, the opt-in that arms each
// one, and the model variable. Every integration-test section of the
// development guide (development.md) names its own values, so a section left
// behind by a rename fails. The values come from the deepseek and publisher
// test packages' exported constants, not from the document.
func TestDeepSeekDocsContract(t *testing.T) {
	development := doctestutil.ReadDoc(t, developmentDocPath)
	for _, section := range []struct {
		name    string
		heading string
		values  []string
	}{
		{
			name:    "deepseek",
			heading: doctestutil.DevDeepSeekIntegrationHeading,
			values:  []string{deepseektestutil.APIKeyEnv, deepseektestutil.DeepSeekOptInEnv, deepseektestutil.ModelEnv},
		},
		{
			name:    "cli",
			heading: doctestutil.DevCLIIntegrationHeading,
			values:  []string{deepseektestutil.APIKeyEnv, deepseektestutil.CLIOptInEnv, deepseektestutil.ModelEnv},
		},
		{
			name:    "cli_slack",
			heading: doctestutil.DevCLIWebhookIntegrationHeading,
			values:  []string{deepseektestutil.APIKeyEnv, publishertestutil.CLISlackOptInEnv, deepseektestutil.ModelEnv},
		},
	} {
		t.Run(section.name, func(t *testing.T) {
			text := doctestutil.SectionOutsideBlocks(development, section.heading)
			if text == "" {
				t.Fatalf("development.md has no %q section", section.heading)
			}
			requireMentions(t, "development.md "+section.heading, text, section.values)
		})
	}
}

// TestLLMDefaultsDocsContract checks that the configuration tables of
// README.md and project_overview.md state the code's LLM default: each table
// has exactly one row for each LLM variable, and the first code span of the
// "when unset" cell of the YT2COLUMN_LLM row is the value the config package
// applies. The YT2COLUMN_REVIEW_LLM row inherits the generation selection,
// which no code constant spells, so only its presence is checked. A stale
// value written as a code span in front of the new one fails; prose in front
// of it, and the rest of the cell, are not checked here but read in review.
func TestLLMDefaultsDocsContract(t *testing.T) {
	want := []struct{ name, value string }{
		{config.LLMEnvForTest(), config.DefaultLLMValueForTest()},
		{config.ReviewLLMEnvForTest(), ""}, // presence only
	}
	for _, doc := range []struct {
		name, path, nameHeader, unsetHeader string
	}{
		{"README.md", readmePath, "Variable", "When unset"},
		{"project_overview.md", projectOverviewPath, "変数", "未設定のとき"},
	} {
		t.Run(doc.name, func(t *testing.T) {
			table := doctestutil.FindTable(doctestutil.ReadDoc(t, doc.path), doc.nameHeader)
			if table == nil {
				t.Fatalf("%s has no table with a %q column", doc.name, doc.nameHeader)
			}
			column := slices.Index(table[0], doc.unsetHeader)
			if column < 0 {
				t.Fatalf("%s: the %q table has no %q column", doc.name, doc.nameHeader, doc.unsetHeader)
			}
			for _, v := range want {
				// A second row for the same variable would go unchecked, so
				// exactly one is required.
				rows := tableRowsWithFirstCell(table, v.name)
				if len(rows) != 1 || len(rows[0]) <= column {
					t.Errorf("%s: %d rows for %s, want exactly one with a %q cell", doc.name, len(rows), v.name, doc.unsetHeader)
					continue
				}
				if v.value == "" {
					continue
				}
				cell := rows[0][column]
				value, ok := firstCodeSpan(cell)
				if !ok || value != v.value {
					t.Errorf("%s: %s %q cell = %q, want its first code span to be %q", doc.name, v.name, doc.unsetHeader, cell, v.value)
				}
			}
		})
	}
}

// tableRowsWithFirstCell returns every body row whose first cell equals first,
// ignoring backticks. It is doctestutil.TableRow for a table that can hold more
// than one row per key, such as the review-log item table.
func tableRowsWithFirstCell(table [][]string, first string) [][]string {
	var rows [][]string
	for _, row := range table[1:] {
		if len(row) > 0 && doctestutil.StripBackticks(row[0]) == first {
			rows = append(rows, row)
		}
	}
	return rows
}

// tableFlagContains reports whether the flags table documents want, a flag name
// such as --out. It splits the first cell (the flag column) into tokens and
// requires an exact match, so a documented --output does not satisfy --out and a
// flag named in another row's description does not count as documented.
func tableFlagContains(table [][]string, want string) bool {
	for _, row := range table[1:] {
		if len(row) == 0 {
			continue
		}
		tokens := strings.FieldsFunc(row[0], func(r rune) bool {
			return r == '`' || r == ',' || r == ' ' || r == '\t'
		})
		if slices.Contains(tokens, want) {
			return true
		}
	}
	return false
}

// codeSpanRE matches one Markdown code span and captures its content. These
// documents do not nest code spans.
var codeSpanRE = regexp.MustCompile("`([^`]*)`")

// firstCodeSpan returns the content of the first code span in cell.
func firstCodeSpan(cell string) (string, bool) {
	match := codeSpanRE.FindStringSubmatch(cell)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// exportLineRE matches one line of a configuration block: "export", one space,
// an uppercase variable name, "=", and a value of letters, digits, ".", "_",
// "-", and "/" (the separator of an LLM variable's provider and model ID).
var exportLineRE = regexp.MustCompile(`^export ([A-Z][A-Z0-9_]*)=([A-Za-z0-9._/-]+)$`)

// checkExportBlock checks that section holds exactly one sh code block and that
// every line of it has the export-line shape, names one of want, and does not
// repeat a variable. It returns the block's values. The failure messages name
// the document, section, line, and variable, never a line's value: a secret
// written into the block by mistake must not reach the log.
func checkExportBlock(t *testing.T, docName, sectionName, section string, want []string) map[string]string {
	t.Helper()
	var shBlocks []doctestutil.FencedBlock
	for _, block := range doctestutil.FencedBlocks(section) {
		if block.Info == "sh" {
			shBlocks = append(shBlocks, block)
		}
	}
	if len(shBlocks) != 1 {
		t.Fatalf("%s %s: %d sh code blocks, want exactly one", docName, sectionName, len(shBlocks))
	}
	if !shBlocks[0].Closed {
		t.Fatalf("%s %s: the sh code block has no closing fence; an unclosed fence runs to the end of the document", docName, sectionName)
	}
	values := make(map[string]string, len(want))
	for i, line := range shBlocks[0].Lines {
		match := exportLineRE.FindStringSubmatch(line)
		if match == nil {
			t.Errorf("%s %s: block line %d is not an export line of the form export <NAME>=<value>", docName, sectionName, i+1)
			continue
		}
		name, value := match[1], match[2]
		if _, repeated := values[name]; repeated {
			t.Errorf("%s %s: block line %d repeats the variable %s", docName, sectionName, i+1, name)
			continue
		}
		values[name] = value
	}
	for _, name := range want {
		if _, ok := values[name]; !ok {
			t.Errorf("%s %s: the block does not set the variable %s", docName, sectionName, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if !slices.Contains(want, name) {
			t.Errorf("%s %s: the block sets the unexpected variable %s", docName, sectionName, name)
		}
	}
	accepted := make(map[string]string, len(want))
	for _, name := range want {
		if value, ok := values[name]; ok {
			accepted[name] = value
		}
	}
	return accepted
}

// docsProfileEnv is the closed environment test (a) loads the server
// configuration from: a dummy API key, a dummy Webhook URL under the reserved
// .invalid domain, and a short fixed HOME, because the socket path derived from
// it must fit the operating system's limit. No key of the process environment
// reaches LoadServer through it.
func docsProfileEnv() map[string]string {
	return map[string]string{
		config.AnthropicAPIKeyEnvForTest(): "docs-test-api-key",
		config.SlackWebhookEnvForTest():    "https://example.invalid/webhook",
		"HOME":                             "/tmp/yt2column-docs-test",
	}
}

// checkExportBlockMatchesDefaults loads the server configuration without the
// block's variables and with them, and fails unless both load and their
// generation and review provider, model, and effort are equal. It returns the
// configuration loaded without the block, which is the code's default profile.
func checkExportBlockMatchesDefaults(t *testing.T, docName, sectionName string, block map[string]string) config.LLMConfig {
	t.Helper()
	without, err := config.LoadServer(lookupFrom(docsProfileEnv()))
	if err != nil {
		t.Fatalf("%s %s: LoadServer on the default variables: %v", docName, sectionName, err)
	}
	env := docsProfileEnv()
	maps.Copy(env, block)
	loaded, err := config.LoadServer(lookupFrom(env))
	if err != nil {
		t.Fatalf("%s %s: LoadServer rejected the block's variables: %v", docName, sectionName, err)
	}
	// The model names are document values, so a mismatch says only whether
	// they are equal: a secret pasted into the block by mistake must not reach
	// the log.
	got, want := loaded.LLM(), without.LLM()
	modelEqual := got.Model() == want.Model()
	reviewModelEqual := got.ReviewModel() == want.ReviewModel()
	if got.Provider() != want.Provider() || !modelEqual ||
		got.ReviewProvider() != want.ReviewProvider() || !reviewModelEqual ||
		got.ClaudeEffort() != want.ClaudeEffort() {
		t.Errorf("%s %s: the block's profile differs from the code's default profile "+
			"(provider %v/%v, model equal %t, review provider %v/%v, review model equal %t, effort %v/%v)",
			docName, sectionName, got.Provider(), want.Provider(), modelEqual,
			got.ReviewProvider(), want.ReviewProvider(), reviewModelEqual,
			got.ClaudeEffort(), want.ClaudeEffort())
	}
	return without.LLM()
}

// profileStages are the rows the shared document's profile table must hold,
// in the order the article is written.
var profileStages = []string{"生成", "字数の調整", reviewProfileStage}

// reviewProfileStage is the profile table's row for the review step, whose
// values go into YT2COLUMN_REVIEW_LLM.
const reviewProfileStage = "検証・推敲"

// productionProfileVars returns the variables that describe the production
// profile: the generation provider and model, which the review step inherits,
// and the Claude effort, which all three stages share.
func productionProfileVars() []string {
	return []string{
		config.LLMEnvForTest(),
		config.ClaudeEffortEnvForTest(),
	}
}

// tableColumn returns the index of the column whose header equals name,
// ignoring case and backticks, or -1. It matches the way doctestutil.FindTable
// and tableRowsWithFirstCell read a header cell.
func tableColumn(header []string, name string) int {
	for i, cell := range header {
		if strings.EqualFold(doctestutil.StripBackticks(cell), name) {
			return i
		}
	}
	return -1
}

// checkProfileTable checks the shared document's profile table: each stage has
// exactly one row, and the first code spans of its provider and model cells,
// joined by "/" into one LLM variable value, and of its effort cell, loaded
// through LoadServer, give the default profile's values for that stage. The
// review row's value goes into YT2COLUMN_REVIEW_LLM and the other rows' into
// YT2COLUMN_LLM. The shortening stage calls the generation client, so its row
// is compared with the generation values.
func checkProfileTable(t *testing.T, section string, defaults config.LLMConfig) {
	t.Helper()
	table := doctestutil.FindTable(section, "段階")
	if table == nil {
		t.Fatal("production_profile.md: the profile section has no table with a 段階 column")
	}
	names := []string{"プロバイダ", "モデル", "effort"}
	columns := make([]int, len(names))
	for i, name := range names {
		if columns[i] = tableColumn(table[0], name); columns[i] < 0 {
			t.Fatalf("production_profile.md: the profile table has no %s column", name)
		}
	}
	for _, row := range table[1:] {
		if len(row) > 0 && !slices.Contains(profileStages, doctestutil.StripBackticks(row[0])) {
			t.Errorf("production_profile.md: the profile table has a row for %q, which is not a stage of the profile", row[0])
		}
	}
	for _, stage := range profileStages {
		rows := tableRowsWithFirstCell(table, stage)
		if len(rows) != 1 {
			t.Errorf("production_profile.md: the profile table has %d rows for %q, want exactly one", len(rows), stage)
			continue
		}
		values := make([]string, len(names))
		complete := true
		for i, name := range names {
			if len(rows[0]) <= columns[i] {
				t.Errorf("production_profile.md: the %q row has %d cells, want more than %d", stage, len(rows[0]), columns[i])
				complete = false
				continue
			}
			value, ok := firstCodeSpan(rows[0][columns[i]])
			if !ok {
				t.Errorf("production_profile.md: the %q row's %s cell has no code span", stage, name)
				complete = false
				continue
			}
			values[i] = value
		}
		if !complete {
			continue
		}
		review := stage == reviewProfileStage
		llmEnv := config.LLMEnvForTest()
		if review {
			llmEnv = config.ReviewLLMEnvForTest()
		}
		env := docsProfileEnv()
		env[llmEnv] = values[0] + "/" + values[1]
		env[config.ClaudeEffortEnvForTest()] = values[2]
		cfg, err := config.LoadServer(lookupFrom(env))
		if err != nil {
			t.Errorf("production_profile.md: LoadServer rejects the %q row's values: %v", stage, err)
			continue
		}
		got := cfg.LLM()
		gotProvider, gotModel := got.Provider(), got.Model()
		wantProvider, wantModel := defaults.Provider(), defaults.Model()
		if review {
			gotProvider, gotModel = got.ReviewProvider(), got.ReviewModel()
			wantProvider, wantModel = defaults.ReviewProvider(), defaults.ReviewModel()
		}
		// The model name comes from the document, so the diagnostic says only
		// whether it matches: a secret pasted into the cell by mistake must not
		// reach the log.
		modelEqual := gotModel == wantModel
		if gotProvider != wantProvider || !modelEqual || got.ClaudeEffort() != defaults.ClaudeEffort() {
			t.Errorf("production_profile.md: the %q row gives provider %v, model equal %t, effort %v; the default profile gives provider %v, effort %v",
				stage, gotProvider, modelEqual, got.ClaudeEffort(), wantProvider, defaults.ClaudeEffort())
		}
	}
}

// TestProductionProfileDocsContract checks the shared production-profile
// document's profile section and the README's recommended configuration
// section: each configuration block holds exactly the variables of the
// production profile and loads to the code's default profile, the README's
// subsection lies inside Configuration, and the shared document's table names
// the same profile for each stage.
func TestProductionProfileDocsContract(t *testing.T) {
	want := productionProfileVars()

	doc := doctestutil.ReadDoc(t, productionProfilePath)
	section := doctestutil.SectionOutsideBlocks(doc, productionProfileSection)
	if section == "" {
		t.Fatalf("production_profile.md has no %q section", productionProfileSection)
	}
	values := checkExportBlock(t, "production_profile.md", productionProfileSection, section, want)
	defaults := checkExportBlockMatchesDefaults(t, "production_profile.md", productionProfileSection, values)
	checkProfileTable(t, section, defaults)

	readme := doctestutil.ReadDoc(t, readmePath)
	recommended := doctestutil.SectionOutsideBlocks(readme, recommendedConfigHeading)
	if recommended == "" {
		t.Fatalf("README has no %q section", recommendedConfigHeading)
	}
	configSection := doctestutil.SectionOutsideBlocks(readme, "## Configuration")
	inConfiguration := false
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(configSection))) {
		if !line.Block && strings.TrimSpace(line.Text) == recommendedConfigHeading {
			inConfiguration = true
		}
	}
	if !inConfiguration {
		t.Errorf("README: %q is not inside the Configuration section", recommendedConfigHeading)
	}
	readmeValues := checkExportBlock(t, "README.md", recommendedConfigHeading, recommended, want)
	checkExportBlockMatchesDefaults(t, "README.md", recommendedConfigHeading, readmeValues)
}

// migrationHeading is the README section that shows how to move from the
// retired LLM variables to the new ones.
const migrationHeading = "### Migrating from the retired LLM variables"

// TestMigrationDocsContract checks the README's migration example: the
// section holds one text block of the retired generation variables and one sh
// block setting YT2COLUMN_LLM alone, and the sh block loads, through
// LoadServer, to the generation provider and model the retired block named.
// The retired names come from the config
// package, whose table orders the generation provider variable first and the
// generation model variable second.
func TestMigrationDocsContract(t *testing.T) {
	section := doctestutil.SectionOutsideBlocks(doctestutil.ReadDoc(t, readmePath), migrationHeading)
	if section == "" {
		t.Fatalf("README has no %q section", migrationHeading)
	}
	after := checkExportBlock(t, "README.md", migrationHeading, section, []string{config.LLMEnvForTest()})

	retired := config.RetiredLLMVarsForTest()
	providerVar, modelVar := retired[0], retired[1]
	var before map[string]string
	for _, block := range doctestutil.FencedBlocks(section) {
		if block.Info != "text" {
			continue
		}
		if before != nil {
			t.Fatalf("README %s: more than one text block, want exactly one", migrationHeading)
		}
		before = map[string]string{}
		for i, line := range block.Lines {
			name, value, ok := strings.Cut(strings.TrimRight(line, "\n"), "=")
			if !ok || !slices.Contains(retired, name) {
				t.Errorf("README %s: text block line %d is not <retired variable>=<value>", migrationHeading, i+1)
				continue
			}
			before[name] = value
		}
	}
	if before == nil {
		t.Fatalf("README %s: no text block of the retired variables", migrationHeading)
	}
	if len(before) != 2 || before[providerVar] == "" || before[modelVar] == "" {
		t.Fatalf("README %s: the text block sets %d variables, want exactly %s and %s",
			migrationHeading, len(before), providerVar, modelVar)
	}

	load := func(value string) config.LLMConfig {
		t.Helper()
		env := docsProfileEnv()
		env[config.DeepSeekAPIKeyEnvForTest()] = "docs-test-api-key"
		env[config.LLMEnvForTest()] = value
		cfg, err := config.LoadServer(lookupFrom(env))
		if err != nil {
			t.Fatalf("README %s: LoadServer rejects a value of the example: %v", migrationHeading, err)
		}
		return cfg.LLM()
	}
	got := load(after[config.LLMEnvForTest()])
	want := load(before[providerVar] + "/" + before[modelVar])
	// The model names come from the document, so the diagnostic says only
	// whether they are equal: a secret pasted into the block by mistake must
	// not reach the log.
	if got.Provider() != want.Provider() || got.Model() != want.Model() {
		t.Errorf("README %s: the new setting gives provider %v (want %v), model equal %t; want the retired setting's",
			migrationHeading, got.Provider(), want.Provider(), got.Model() == want.Model())
	}
}

// TestQuickStartLinks checks the README's Quick start section: it comes before
// Requirements, every numbered item links to a README heading, and every link
// in the section is either such an anchor or an existing relative path.
func TestQuickStartLinks(t *testing.T) {
	readme := doctestutil.ReadDoc(t, readmePath)
	section := doctestutil.SectionOutsideBlocks(readme, quickStartHeading)
	if section == "" {
		t.Fatalf("README has no %q section", quickStartHeading)
	}
	headings := doctestutil.HeadingsOutsideBlocks(readme)
	at, requirements := slices.Index(headings, quickStartHeading), slices.Index(headings, "## Requirements")
	if at < 0 || requirements < 0 || at > requirements {
		t.Errorf("README: %q is at heading %d, Requirements at %d; Quick start must come first", quickStartHeading, at, requirements)
	}
	anchors := headingAnchors(t, "README.md", readme)
	items := numberedItems(section)
	if len(items) == 0 {
		t.Fatalf("README %s has no numbered list", quickStartHeading)
	}
	for _, item := range items {
		linked := false
		for _, target := range markdownLinkTargets(item) {
			if anchor, ok := strings.CutPrefix(target, "#"); ok {
				if _, known := anchors[anchor]; known {
					linked = true
				}
			}
		}
		if !linked {
			first, _, _ := strings.Cut(item, "\n")
			t.Errorf("README Quick start item %q has no link to a README heading", first)
		}
	}
	// TestReadmeLinks checks that every link in the README, this section's
	// included, resolves; only the step guidance's own rule is checked here.
	for _, target := range markdownLinkTargets(section) {
		if strings.Contains(target, "://") {
			t.Errorf("README Quick start links to the external URL %q; link to a README section instead", target)
		}
	}
}

// TestProductionProfileLinks checks that every document that describes the
// production profile links to the shared document by a relative path that
// resolves to the file the test reads. The expected target comes from the path
// constant, not from a literal copied out of a document.
func TestProductionProfileLinks(t *testing.T) {
	want, err := filepath.Abs(productionProfilePath)
	if err != nil {
		t.Fatalf("resolve %s: %v", productionProfilePath, err)
	}
	for _, tc := range []struct{ name, path, heading string }{
		{"README.md", readmePath, recommendedConfigHeading},
		{"project_overview.md", projectOverviewPath, "## 決定済みの方針"},
		{"CLAUDE.md", claudePath, "## Quick Links"},
		{"prompts/README.md", promptsReadmePath, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := doctestutil.ReadDoc(t, tc.path)
			if tc.heading != "" {
				if text = doctestutil.SectionOutsideBlocks(text, tc.heading); text == "" {
					t.Fatalf("%s has no %q section", tc.name, tc.heading)
				}
			}
			for _, target := range markdownLinkTargets(text) {
				if strings.Contains(target, "://") {
					continue
				}
				path, _, _ := strings.Cut(target, "#")
				got, err := filepath.Abs(filepath.Join(filepath.Dir(tc.path), path))
				if err == nil && got == want {
					return
				}
			}
			where := "the whole document"
			if tc.heading != "" {
				where = strconv.Quote(tc.heading)
			}
			t.Errorf("%s has no link that resolves to %s in %s", tc.name, productionProfilePath, where)
		})
	}
}

// TestReadmeLinks checks the links of the README and of the two developer
// guides split out of it (cli.md and development.md): every relative link
// points at an existing path, and every link to a heading, in the same
// document or in another one, resolves to an anchor the helper produces from
// the target document's headings. GitHub resolves those, so they are the
// oracle for the anchor rule.
func TestReadmeLinks(t *testing.T) {
	for _, doc := range []struct{ name, path string }{
		{"README.md", readmePath},
		{"cli.md", cliDocPath},
		{"development.md", developmentDocPath},
	} {
		t.Run(doc.name, func(t *testing.T) {
			text := doctestutil.ReadDoc(t, doc.path)
			anchors := headingAnchors(t, doc.name, text)
			targets := markdownLinkTargets(text)
			checkLinkCount(t, doc.name, text, len(targets))
			for _, target := range targets {
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				path, anchor, hasAnchor := strings.Cut(target, "#")
				known := anchors
				if path != "" {
					resolved := filepath.Join(filepath.Dir(doc.path), path)
					if _, err := os.Stat(resolved); err != nil {
						t.Errorf("%s links to %q, which does not exist", doc.name, target)
						continue
					}
					if !hasAnchor {
						continue
					}
					known = headingAnchors(t, path, doctestutil.ReadDoc(t, resolved))
				}
				if _, ok := known[anchor]; !ok {
					t.Errorf("%s links to %q, which the anchor helper does not produce", doc.name, target)
				}
			}
		})
	}
}

// TestNumberedItems checks the numbered-list extraction: an item ends at a
// blank line or the next item, a fenced block that belongs to an item stays in
// it, and an indented number is not a top-level item.
func TestNumberedItems(t *testing.T) {
	const section = "intro\n1. first\n   continued\n2. second\n\n   3. indented\n\n```text\n3. not an item\n```\n3. third\n"
	want := []string{"1. first\n   continued", "2. second", "3. third"}
	if got := numberedItems(section); !slices.Equal(got, want) {
		t.Errorf("numberedItems = %q, want %q", got, want)
	}
	const withBlock = "1. run this\n   ```sh\n   3. not an item\n\n   make test\n   ```\n2. next\n"
	wantWithBlock := []string{"1. run this\n   ```sh\n   3. not an item\n\n   make test\n   ```", "2. next"}
	if got := numberedItems(withBlock); !slices.Equal(got, wantWithBlock) {
		t.Errorf("numberedItems with a block inside an item = %q, want %q", got, wantWithBlock)
	}
}

// containsToken reports whether word appears in doc bounded by characters that
// are not letters, digits, underscores, or hyphens, so "known" is not satisfied
// by "unknown" or "recorded-unknown", and "run" is not satisfied by "runs".
func containsToken(doc, word string) bool {
	pattern := `(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_-]|$)`
	return regexp.MustCompile(pattern).MatchString(doc)
}

// checkExampleLines checks the example lines in section that use prefix: the
// first three words after the prefix must be in allowed. A line whose first word
// is a placeholder (one starting with "<") shows the line's shape, not an
// example, and is skipped. what names the section in the failure messages, and
// the call fails t when the section has no example line.
func checkExampleLines(t *testing.T, section, prefix string, allowed map[string]bool, what string) {
	t.Helper()
	examples := 0
	for line := range strings.SplitSeq(section, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), prefix)
		if !ok {
			continue
		}
		words := strings.Fields(rest)
		if len(words) > 0 && strings.HasPrefix(words[0], "<") {
			continue
		}
		examples++
		if len(words) < 3 {
			t.Errorf("%s example line %q has fewer than three words after the prefix", what, line)
			continue
		}
		for i := range 3 {
			if !allowed[words[i]] {
				t.Errorf("%s example line %q has the unknown word %q at position %d", what, line, words[i], i+1)
			}
		}
	}
	if examples == 0 {
		t.Errorf("%s has no example line", what)
	}
}

// TestTokenUsageDocsContract checks that the CLI guide's (cli.md) Token usage
// section contains the line's words and every reason llm.Usage can produce.
//
// The behavior these words describe is pinned by running tests, not by this
// test: the line format by cmd/yt2column's TestRunTokenLinesGoldenFormat, and
// the usage validation by the adapters' TestGenerateUsageUnknown and
// internal/artifact's TestParseRecordRejectsUsage.
func TestTokenUsageDocsContract(t *testing.T) {
	cli := doctestutil.ReadDoc(t, cliDocPath)
	section := doctestutil.SectionByHeading(cli, doctestutil.CLIUsageHeading)
	if section == "" {
		t.Fatal("cli.md has no Token usage section")
	}
	// Every format word run.go defines, plus the three stage names. The
	// value-carrying fields are matched as substrings; the fixed words are
	// matched as whole tokens, so "known" is not satisfied by "unknown" and
	// "run" is not satisfied by "runs".
	for _, word := range []string{tokensInput, tokensCacheRead, tokensCacheWrite, tokensOutput, tokensReason, tokensModel} {
		if !strings.Contains(section, word) {
			t.Errorf("cli.md Token usage section does not contain %q", word)
		}
	}
	for _, word := range []string{
		tokensPrefix, tokensRun, tokensReused, tokensKnown, tokensUnknown, tokensNoModel,
		writer.StageGeneration.String(), writer.StageShorten.String(), writer.StageReview.String(),
	} {
		if !containsToken(section, word) {
			t.Errorf("cli.md Token usage section does not contain the token %q", word)
		}
	}
	// Every member-less reason llm.Usage can produce, obtained from the code so
	// the test does not spell them. cli.md also documents the member-named
	// forms (`usage.<member>:<rule>`) in prose.
	reasons := []llm.Usage{
		llm.UnknownUsage(llm.RuleNotRead, ""),
		llm.UnknownUsage(llm.RuleRecordedUnknown, ""),
		llm.UnknownUsage(llm.RuleMissing, ""),
		llm.UnknownUsage(llm.RuleDuplicate, ""),
		llm.UnknownUsage(llm.RuleNotObject, ""),
		llm.UnknownUsage(llm.RuleNotNonNegativeInteger, ""),
		llm.UnknownUsage(llm.UnknownRule(99), ""),
	}
	for _, usage := range reasons {
		if reason := usage.UnknownReason(); !strings.Contains(section, reason) {
			t.Errorf("cli.md Token usage section does not contain the reason %q", reason)
		}
	}
	// Each example line's first three words after the prefix are known words.
	allowed := map[string]bool{
		tokensRun: true, tokensReused: true, tokensKnown: true, tokensUnknown: true,
	}
	for _, stage := range []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
		allowed[stage.String()] = true
	}
	checkExampleLines(t, section, tokensPrefix, allowed, "cli.md Token usage section")
}

// TestCostDocsContract checks that the CLI guide's (cli.md) Estimated cost
// section uses every word run.go's cost format defines, that its example lines use only
// those words and the stage names in the fixed positions, that
// project_overview.md contains the cost prefix, and that the rate table's
// leading comment points at the update procedure.
//
// The behavior these words describe is pinned by running tests, not by this
// test: the line format by cmd/yt2column's TestRunCostLinesGoldenFormat.
func TestCostDocsContract(t *testing.T) {
	cli := doctestutil.ReadDoc(t, cliDocPath)
	section := doctestutil.SectionByHeading(cli, doctestutil.CLIEstimatedCostHeading)
	if section == "" {
		t.Fatal("cli.md has no Estimated cost section")
	}
	// The value-carrying fields of the cost lines, taken from run.go so the
	// test does not spell them.
	for _, word := range []string{costPrefix, costTotalPrefix, costUSD, costReason, costAt} {
		if !strings.Contains(section, word) {
			t.Errorf("cli.md Estimated cost section does not contain %q", word)
		}
	}
	// The fixed words of the lines, matched as whole tokens so "known" is not
	// satisfied by "unknown". The two total-kind words are checked in their
	// format position below, because both also appear in the section's prose.
	for _, word := range []string{
		costKnown, costUnknown, costNoTime, costInvalid,
		writer.StageGeneration.String(), writer.StageShorten.String(), writer.StageReview.String(),
	} {
		if !containsToken(section, word) {
			t.Errorf("cli.md Estimated cost section does not contain the token %q", word)
		}
	}
	// Each total kind must appear on a cost-total line, not only in prose.
	for _, kind := range []string{costTotalRun, costTotalArticle} {
		if !strings.Contains(section, costTotalPrefix+kind) {
			t.Errorf("cli.md Estimated cost section has no %q total line", costTotalPrefix+kind)
		}
	}
	// Every rate basis and unknown reason the word mapping returns, obtained
	// from the code so the test does not spell them. The zero value and a value
	// outside the enumeration map to costInvalid, which the token loop above
	// already checks, so this only checks the mapping-specific words.
	for _, basis := range []pricing.RateBasis{
		pricing.BasisUnset, pricing.BasisFlat, pricing.BasisOffPeak, pricing.BasisPeak,
		pricing.BasisShortPrompt, pricing.BasisLongPrompt, pricing.RateBasis(99),
	} {
		if word := costBasisWord(basis); word != costInvalid && !containsToken(section, word) {
			t.Errorf("cli.md Estimated cost section does not contain the rate-basis word %q", word)
		}
	}
	for _, reason := range []pricing.UnknownReason{
		pricing.ReasonUnset, pricing.ReasonUsageUnknown, pricing.ReasonNoModel,
		pricing.ReasonUnpricedModel, pricing.ReasonInvalid, pricing.UnknownReason(99),
	} {
		if word := costReasonWord(reason); word != costInvalid && !containsToken(section, word) {
			t.Errorf("cli.md Estimated cost section does not contain the reason word %q", word)
		}
	}
	// Each example line's first three words after the prefix are the stage, the
	// origin, and the cost state.
	allowed := map[string]bool{
		tokensRun: true, tokensReused: true, costKnown: true, costUnknown: true,
	}
	for _, stage := range []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
		allowed[stage.String()] = true
	}
	checkExampleLines(t, section, costPrefix, allowed, "cli.md Estimated cost section")

	overview := doctestutil.ReadDoc(t, projectOverviewPath)
	if !strings.Contains(overview, costPrefix) {
		t.Errorf("project_overview.md does not contain %q", costPrefix)
	}

	rates := doctestutil.ReadDoc(t, pricingRatesPath)
	head, _, _ := strings.Cut(rates, "\npackage ")
	if !strings.Contains(head, "pricing_update.md") {
		t.Error("internal/pricing/rates.go's leading comment does not point at pricing_update.md")
	}
}
