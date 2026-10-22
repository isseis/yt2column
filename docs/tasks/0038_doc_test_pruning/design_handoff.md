# 設計への申し送り：0038 文書のテストの削除

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、項目ごとに取った方法か、当てはまらない理由を記す。

-   **H-01: テストごとの判定の一覧。** 要件定義の際の調査で、境界にあるものとして次の種類が見つかった。設計で、F-001 の K1〜K5・D1〜D4 のどれに当たるかをテストごと・確認ごとに決め、一覧にする。
    -   コードの定数の値と自然言語の語句が同じ行にあることを確かめるもの（`cmd/yt2column-server/docs_test.go` の project_overview.md の行、security.md 8.2 の「枠（`N`）」）。値の部分だけを残す方法を決める。
    -   本物のパーサで解析しつつ、文書が含むべき呼び出しの一覧を直書きしているもの（`TestDocumentedInvocationsParse` の `cmd*` の一覧）。解析の部分は K3、呼び出しの一覧は K5 に当たる。
    -   `TestCopiedTranscriptSurveyRecords` の、`testdata/` のファイルの一覧と文書の照合（K1）と、出所の記録の項目名（`取得日` など。`docs/dev/security.md` 7 章の規則に対応する）の存在。項目名を K2 として扱うか D2 として扱うかを決める。
    -   `TestContextAndCommandsAgree` の各部分。`_context.md` の行を `fixprguard` のパーサで解析する部分、`wantReviewers` との照合（K4）、文言の部分（D1・D2）、`fixpr.md` などが名前を書かないことの確認（D3 か K4 の一部か）。
    -   `TestRemovedCommandLiterals` の各語。ガードツールを迂回するコマンド（K5）と、それ以外の古い語（D3）を分ける。
    -   `TestGuideCoversStopKindsAndScope` の、許可リストに `*.go` がないことの直書きの確認と、`CLAUDE.md` からガイドへのリンクの確認。
-   **H-02: 共有の補助の扱い。** `cmd/yt2column` パッケージの補助（`readDoc`、`sectionByHeading`、`findTable`、`flat`、`requireSection`、`requirePhrases` など）は、複数のファイルのテストが使っている。`cmd/yt2column-server/docs_test.go` には同じ名前の補助の写しがある（`:356` のコメント）。削除の後に残る使い手を確かめ、使われなくなる補助と、その単体テスト（`TestSectionByHeading`、`TestSectionByHeadingPrefix`、`TestClaudeBullet`、`TestWithoutSection`、`TestCutSection`、`TestGuideTable` など）の扱いを決める。
-   **H-03: 削除の順序とテストの間の依存。** `TestGuideDescribesReviewerList`（`cmd/fixprguard/docs_test.go:311`）の 9 章の確認は、`cmd/yt2column/fixpr_docs_test.go` のソースに `var wantReviewers ` と `func TestContextReviewerRowsMalformed(` があることを確かめている。この確認はテストのソースを値の出どころとするので D2 に当たるが、名前を変える場合は順序に依存がある。削除の順序と PR の分け方を決める。
-   **H-04: カバレッジの比較の手段。** 文書のテストの多くは製品のコードを通らないが、`TestArtifactDocsContract`（`run -h`）、`TestOutputLayoutDocsContract`（`FilePublisher` の出力）、`TestDocumentedInvocationsParse`（`parse()`）のように通るものがある。比べる範囲（パッケージ、`-tags test`、`-coverpkg`）と、記録の形を決める。
-   **H-05: 拡張のジョブの起動条件。** `metadataDocs.test.ts` の判定の結果、node のテストが読む文書が変わる場合の、`has-extension-changes.sh`、`ciChanges.test.ts`、`CLAUDE.md` の変更を決める。`README.md` は `serverReasons.test.ts` が読み続けるので、起動条件に残る見込みである。
-   **H-06: タスクの文書の改訂の箇所。** AC の検証方法を述べる箇所（F-005）を、タスクごとに特定する。タスクによって、計画の受け入れ基準の検証の表の形が違う（たとえば 0012 の計画は、検査 1〜6 の組で AC を確かめる）。0010 の計画には古いパス（`cmd/docs_test.go::TestReviewEvaluationRecords`）の行がある。
-   **H-07: `TestSharedTokenRemoved` の分け方。** 同じテストが、Go のコードと拡張のソース（残す）と、文書（D3 で削除）の両方を調べている。README の「Upgrading from the shared-token version」節を除外する仕組み（`withoutSection` など）も、文書の確認を削除すると不要になる。
-   **H-08: 未実装の計画の確認の時点。** F-006 の「本タスクの実装の時点」を、どの PR のどの段階で確かめるかを決める（タスク 0034 の計画の進み具合）。
