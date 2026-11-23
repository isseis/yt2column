# 設計への申し送り：古い記録行の読み方規則を、対象ブランチがなくなった後に外す

`01_requirements.md` の起草時に挙がった設計・実装上の論点を記録する。`02_architecture.md` でそれぞれの扱い（採用した方針、または該当しない理由）を記録すること。

## H-01: `cmd/yt2column/review_records_test.go` の `(codex)` テストケースの扱いと `coverage-compare`

-   **懸念:** `cmd/yt2column/review_records_test.go:252-256` に既存の `{name: "seen by codex", ..., want: rrAll}` ケースがあり、同ファイル `:342-346` には別レビュアー `(fable)` を拒否する `"another reviewer"` ケースがある。`(codex)` 拒否（AC-08・AC-09）を検証するにあたり、`"seen by codex"` を削除して `make coverage-compare`（ADR-0037）を通すか、期待値を `PARTIAL` に書き換えて `(codex)` 拒否テストとして残すか（加えて `fableCases()` に `(codex)` 拒否ケースを追加するか）を設計で決める必要がある。
-   **重要な理由:** Go テストの削除を伴う場合は、削除のみのコミットを分けて `make coverage-compare BASE=<親>` を通す必要があり（`CLAUDE.md`・ADR-0037）、コミット構成と実装計画のステップに影響するためである。
-   **候補案:**
    -   案 A: `"seen by codex"` の期待値を `PARTIAL`（`f.partial(f.rev("HEAD~1"))`）へ変更し（ケース名も実態に合わせる）、`fableCases()` にも `(codex)` が無視されるケースを追加する（テスト削除が発生しないため、`coverage-compare` の専用コミットが不要になる）。
    -   案 B: `"seen by codex"` を削除のみのコミットで削除し（`make coverage-compare` を実行して PASS を記録）、別コミットで `review-records.sh` の `SEEN_RE` 更新と `(codex)` 拒否ケース（`coverage`・`fable`）の追加を行う。
    -   いずれの場合も、`rrFixture.seen(start, end, cli string)`（`review_records_test.go:143-145`）は `"coderabbit"` 以外の負例（`"codex"`・`"fable"`）を組み立てるために `cli` 引数を残すか、負例専用ヘルパに整理するかを決める。
-   **関連 F-/AC-:** F-003・F-004、AC-08・AC-09・AC-10・AC-12

## H-02: PR #1066（タスク 0114）・PR #1051（タスク 0115）との先行依存および実装開始ゲート

-   **懸念:** 2026-11-23 時点で、`origin/isseis/0114-fable-record-after-merge-04`（PR #1066）と `origin/isseis/0115-chief-conflict-resolution-02`（PR #1051）に古い記録行が残っている。さらに PR #1066 は `.claude/commands/_lib/review-records.sh` と `cmd/yt2column/review_records_test.go` 自体に新しいモード（`unreviewed` 等）と記録行を追加する変更を含んでいる。
-   **重要な理由:** PR #1066 がマージされると `review-records.sh` と `review_records_test.go` の構造・行番号・モード一覧が変わるため、設計・計画の時点で F-001 の検査スクリプトまたはコマンド手順と、実装計画のゲート（`自動送信: hold` などの扱い）および PR #1066 マージ後の差分確認を織り込んでおく必要がある。
-   **候補案:**
    -   F-001 の確認用の検証コマンド（または `docs/tasks/0121_retire_legacy_review_records/_checks/` の確認スクリプト）を設計し、実装開始前と PR マージ前の両方で実行できる形にする。
    -   `03_implementation_plan.md` の実装 PR マーカーで `**自動送信**: hold` を検討し、解除条件を「`origin` の対象ブランチの古い記録行が 0 件であること（PR #1066・PR #1051 の完了を含む）」と明記する。
-   **関連 F-/AC-:** F-001・F-003、AC-01・AC-02・AC-07〜AC-09

## H-03: 既存文書ガードテストへの影響確認

-   **懸念:** `cmd/yt2column/review_records_test.go`・`cmd/yt2column/fixpr_docs_test.go`・`cmd/yt2column/review_mention_docs_test.go` は、`.claude/commands/_lib/local-cli-review.md`・`_lib/task-pr-handoff.md`・`_lib/fable-review.md`・`_lib/review-records.sh` の見出し・表・プレフィックスを読み取って検査している。
-   **重要な理由:** F-002・F-003 の変更が既存の文書ガードテストのパース前提（たとえば `TestRecordLinePrefixesDistinct` や `TestReviewRecordsCallSite`、`TestPostingPathsCheckMention`）に抵触しないかを設計時に確認し、必要なら同じコミットでテストを更新する必要がある。
-   **候補案:** 設計時に `cmd/yt2column/*_test.go` が変更対象の 3 コマンド文書と `review-records.sh` から読んでいる箇所を洗い出し、更新の要否を `02_architecture.md` §3・§7 に明記する。
-   **関連 F-/AC-:** F-002・F-003・F-004、AC-03〜AC-06・AC-12
