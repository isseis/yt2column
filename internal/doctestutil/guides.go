//go:build test

package doctestutil

// Headings and table column names of the two developer guides split out of the
// README, docs/dev/guides/cli.md and docs/dev/guides/development.md, that the
// guard tests find a section or a table by. The guides are Japanese only, so
// each is one Japanese string, and it must equal the guide's text.
const (
	// CLIFlagColumn heads the first column of cli.md's flag table.
	CLIFlagColumn = "フラグ"
	// CLIUsageHeading is cli.md's section on the token usage lines.
	CLIUsageHeading = "## トークンの使用量"
	// CLIEstimatedCostHeading is cli.md's section on the cost lines.
	CLIEstimatedCostHeading = "## 推定の料金"
	// CLIOutFileHeading is cli.md's section with the --out file example.
	CLIOutFileHeading = "### `--out` のファイル"

	// DevAPIHeading is development.md's section on the localhost server's
	// HTTP API: the curl examples, the rejection-reason table, and the job
	// API and its code tables.
	DevAPIHeading = "## localhost サーバの API"
	// DevDeepSeekIntegrationHeading is development.md's section on the
	// DeepSeek integration test.
	DevDeepSeekIntegrationHeading = "### DeepSeek の統合テスト"
	// DevCLIIntegrationHeading is development.md's section on the CLI
	// integration test.
	DevCLIIntegrationHeading = "### CLI の統合テスト"
	// DevCLIWebhookIntegrationHeading is development.md's section on the CLI
	// webhook integration test.
	DevCLIWebhookIntegrationHeading = "### CLI と Webhook の統合テスト"

	// DevReasonColumn, DevStatusColumn, and DevDisplayColumn head the three
	// columns of development.md's rejection-reason table. The reason column
	// keeps the name of the response member it lists.
	DevReasonColumn  = "reason"
	DevStatusColumn  = "ステータスコード"
	DevDisplayColumn = "拡張の表示"
)

// DevAPICodeTableHeader returns the first column header of the development.md
// table that lists the job API codes of the set named name, as
// internal/server's test accessor names the set, and whether there is one. A
// set the guide does not translate here has no header, so a new set fails the
// guard until its locator is added.
func DevAPICodeTableHeader(name string) (string, bool) {
	header, ok := map[string]string{
		"job state":      "ジョブの状態",
		"step":           "工程",
		"step state":     "工程の状態",
		"failure":        "失敗",
		"publish state":  "投稿の状態",
		"publish reason": "投稿の理由",
	}[name]
	return header, ok
}
