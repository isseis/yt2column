# 設計への申し送り（0062_remove_cli_slack）

要件の作成で挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、各項目をどう扱ったか、または扱わない理由を記録する。

## H-01: CLI の削除で使われなくなる関数・型の候補

CLI の Webhook の分岐を削除すると、テストでないコードから使われなくなるものがある。`make deadcode`（AC-13）は `test` と `integration` のタグのテストからの到達も数えるので、テストだけが使う関数は報告しない。テストとともに削除するか、残す送信の経路のテストで使い続けるかを、設計で 1 つずつ決める。

-   `job.RemoteOutput` と `job.Output` の `outputKindRemote`（`internal/job/job.go:54-57`）。呼ぶのは CLI の `buildOutput`（`cmd/yt2column/run.go:658`）と `internal/job/job_test.go` の `TestRunRemoteOutput` だけである。
-   `publisher.SlackMessageCount`（`internal/publisher/slack_message.go`）。テストでないコードで呼ぶのは CLI の `reportSuccess`（`run.go:854`）だけである。
-   `cmd/yt2column/test_helpers.go` の `loopbackSlackPublisher`・`refusingSlackPublisher`・`childModeSlackStall` と、`internal/publisher/testutil/integration.go` の `CLISlackIntegrationOptions`・`CLISlackOptInEnv`。
-   `cmd/yt2column/test_helpers_integration.go` の `cliSlackDeepSeekOptions`・`gateCLISlackIntegration`。

## H-02: `Publish` を使うテストの書き直し

`internal/publisher` の `slack_test.go`・`slack_articles_test.go` は、`Publish` を通して送る前の検査・分割・送信を確かめている。`PrepareSlackMessages` と `PostMessages` を直接呼ぶテストで確かめ直す範囲と、既存のテストとの重複（CLAUDE.md「Check for duplication before adding a table test」）を設計で決める。テストの削除は単独のコミットにし、`make coverage-compare` で確かめる（AC-14）。書き直したテストを先のコミットで加え、元のテストを後のコミットで削除すると、削除のコミットの比較が書き直したテストを含んだ状態で行える。

## H-03: `--slack` の拒否の確かめ方

AC-01 の「`--slack` が定義されていないフラグであることを示す行」は、`flag` パッケージの解析の誤りをそのまま書く今の扱い（`run.go:296-304`）から出る。テストはエラーの文言に頼らずに確かめる（CLAUDE.md「Error Testing」）。例えば、`--slack` が `newFlagSet` で定義されていないことと、未知のフラグで終了コード `2` になり副作用がないことを分けて確かめる。

## H-04: `checkReviewLog` と `checkCacheDirs` の引数

`checkReviewLog`（`run.go:446`）と `checkCacheDirs`（`run.go:545`）は出力先 `destination` を受け取り、`destinationFile` のときだけ `--out` と比べる。出力先が 1 つになった後に `destination` の型を残すか、取り除いて `--out` を常に比べるかを設計で決める。どちらでも AC-04 の振る舞いは変わらない。

## H-05: `make test-integration-slack` の書き直し

統合テスト（`internal/publisher/slack_integration_test.go`）を `PrepareSlackMessages` → `PostMessages` の形にする（AC-16）。記事ごとの上限時間の算出（`SlackPublishMaxDurationForTest`、`internal/publisher/test_helpers_slack.go`）と `Makefile` の `SLACK_INTEGRATION_TIMEOUT` の根拠の注釈が、`Publish` の前提で書かれていないかを確かめる。このテストは本物の Webhook に投稿するので、実行には利用者の承認が要る。

## H-06: コミットの分け方と網羅率の比べ方

AC-14 の網羅率の比べ方を設計で決める。タスク 0059 の前例（0059 `02_architecture.md` 1.3 節、`03_implementation_plan.md` 1.3 節）は、テストを移すコミットを製品のコードを変えずに削除より先に置き、削除のコミットはタスクの `_checks/` のスクリプトで関数を (パッケージ, レシーバ, 名前) で対応付けて比べた。本タスクでも、製品のコードを変えないコミットに `make coverage-compare` が使えるよう、コメントの書き直し（AC-20）や残る関数の部分的な書き換えを、テストだけを削除するコミットに混ぜない。`make coverage-compare` が使えるのは、例えば `Publish` と `job.RemoteOutput` のように関数をまるごと削除し、残る関数を書き換えないコミットである。
