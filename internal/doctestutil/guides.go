//go:build test

package doctestutil

// Headings and table column names of the three developer guides split out of
// the README, docs/dev/guides/generation.md, docs/dev/guides/cli.md, and
// docs/dev/guides/development.md, that the guard tests find a section or a
// table by. The guides are Japanese only, so
// each is one Japanese string, and it must equal the guide's text.
const (
	// GenRecommendedConfigHeading is generation.md's section with the
	// recommended configuration block.
	GenRecommendedConfigHeading = "### 推奨構成"
	// GenVariableColumn and GenUnsetColumn head the first and third columns
	// of generation.md's configuration table, the document's first table with
	// a GenVariableColumn column.
	GenVariableColumn = "変数"
	GenUnsetColumn    = "未設定のとき"
	// GenEffortProviderColumn and GenEffortValuesColumn head the first two
	// columns of generation.md's table of accepted effort values. The values
	// column finds the table: no other table has it.
	GenEffortProviderColumn = "プロバイダ"
	GenEffortValuesColumn   = "受理する effort"

	// CLIFlagColumn heads the first column of cli.md's flag table.
	CLIFlagColumn = "フラグ"
	// CLIUsageHeading is cli.md's section on the token usage lines.
	CLIUsageHeading = "## トークンの使用量"
	// CLIEstimatedCostHeading is cli.md's section on the cost lines.
	CLIEstimatedCostHeading = "## 推定の料金"
	// CLIOutFileHeading is cli.md's section with the --out file example.
	CLIOutFileHeading = "### `--out` のファイル"

	// DevAcceptedColumn heads the column of development.md's table of the
	// development build's scenario variables that lists the values each
	// accepts.
	DevAcceptedColumn = "受理する値"
	// DevBuildHeading is development.md's section on the extension's
	// development build (make ext-build-dev). DevVariableColumn and
	// DevDefaultColumn head its table of the scenario variables, whose
	// accepted values are under DevAcceptedColumn.
	DevBuildHeading   = "## 開発用の拡張のビルド"
	DevVariableColumn = "変数"
	DevDefaultColumn  = "既定の値"
	// DevDeepSeekIntegrationHeading is development.md's section on the
	// DeepSeek integration test.
	DevDeepSeekIntegrationHeading = "### DeepSeek の統合テスト"
	// DevOpenAIIntegrationHeading is development.md's section on the
	// OpenAI integration test.
	DevOpenAIIntegrationHeading = "### OpenAI の統合テスト"
	// DevCLIIntegrationHeading is development.md's section on the CLI
	// integration test.
	DevCLIIntegrationHeading = "### CLI の統合テスト"
)
