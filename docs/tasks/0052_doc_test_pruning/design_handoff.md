# 設計への申し送り：0052 文書のテストの整理

要件定義（`01_requirements.md`）の作成の際に挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、各項目についてとった方法か、当てはまらない理由を記す。

-   **H-01: 調査の起点。** 2026-10-31 の `66222946` での下調べで、次のものが候補に挙がった（当たると決めたものではない）。D5: `TestMigrationDocsContract`、`TestServerDocsContract` の `ReadmeUpgradeHeading` による除外、`checkContextReviewers` の `oldReviewerRow`、`TestRunEvaluationProcedureFlags`（タスク 0010 の評価の手順を実行する）。D6: `TestReadmeSyncStepPlacement`、`TestReadmeVersionLinks`、`TestProductionProfileLinks`、`TestQuickStartLinks`、`TestGuideNamesGitStateParts` の `fixpr.md` の部分、`metadataDocs.test.ts` の socket directory の参照。D7: `TestSlackDocsContract`・`TestClaudeDocsContract` の複数の文書での同じ値の確認、`TestCostDocsContract` の `project_overview.md` の部分、`TestReadmeVersionsShareStructure` と `internal/doctestutil/structure.go`、節の切り出しの補助の重複（`doctestutil.SectionByHeading`・`SectionOutsideBlocks`、`cmd/fixprguard` の `cutSection`、`requireSection`、`itemSection`、`firstCodeBlockAfter`、拡張の `markdownSection`）、`TestDeclaredPermissions`、`TestAgentsCodeReviewRules`、`TestInstallTargetDocumented`。D1〜D4 の残り: `TestCodeRabbitConfigStopsReview` の英文、`TestCostDocsContract` の `rates.go` のコメント、`TestGuideNamesGitStateParts` の `partsList` のロケータ。K4・K5 との重なりを確かめるもの: `TestCodexMentionOnlyInRequestRow`、`TestContextAndCommandsAgree` の不在の確認、`TestRunsCodex` の表。0051 のマージの後に調べ直す。
-   **H-02: 補助の整理の手段。** 節の切り出しの補助が複数のパッケージにある。統合する（`internal/doctestutil` に寄せる）か、取り除いたテストに合わせて削除するだけにするかを決める。統合は D7 (a)・(b) の範囲に限る（要件 2.3）。
-   **H-03: カバレッジの比べ方。** `go tool cover -func` の出力から、製品のコード（AC-11 の定義）の関数を絞り込む方法と、記録の形を決める。テスト用の補助（`internal/doctestutil`）の関数は比較の対象から外れる。
-   **H-04: K4・K5 の確認の対応表。** AC-02 の「本タスクの後にそれを確かめるテスト」を一覧にする方法を決める。統合でテストの名前が変わる場合も辿れるようにする。
-   **H-05: `//go:build test` のアクセサの削除。** `config.RetiredLLMVarsForTest` など、テストのためだけに製品のパッケージに置いたアクセサを、削除するテストに合わせて取り除くとき、`make deadcode` と `depguard` の扱いを確かめる。
-   **H-06: PR の分け方。** パッケージごと、種類（D5〜D7）ごとなど。レビュアーが AC-04 の一覧を PR ごとに読める大きさにする。
-   **H-07: 実装中の計画の確かめ方。** 実装を始める時点で、まだ実装していない作業を含む承認済みの計画と、計画のまだない承認済みの設計書を見つける方法（F-006）。0051 は、実装の開始の条件により、その時点で完了したタスクになる（要件 AC-17）。
-   **H-08: `test_organization.md` の確認。** issue #591 は `docs/dev/conventions/test_organization.md` を記述の更新の対象に挙げる。0038 の時点では文書のテストを扱っていなかった（0038 要件 1 章）。取り除く補助の名前が書かれていないかを確かめ、なければ変えない。
