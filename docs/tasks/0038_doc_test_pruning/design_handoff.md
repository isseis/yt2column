# 設計への申し送り：0038 文書のテストの削除

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、項目ごとに取った方法か、当てはまらない理由を記す。

-   **H-01: テストごとの判定の一覧。** 要件定義の際の調査で、境界にあるものとして次の種類が見つかった。設計で、F-001 の K1〜K4・D1〜D4 のどれに当たるかをテストごと・確かめごとに決め、一覧にする。
    -   コードの定数の値と自然言語の語句が同じ行にあることを確かめるもの（`cmd/yt2column-server/docs_test.go` の project_overview.md の行、security.md 8.2 の「枠（`N`）」）。値の部分だけを残す方法を決める。
    -   コードから導いた値を、文章の中の言い回しを手がかりに取り出すもの（`TestDocumentedProtocolMatches` の「The current number is `N`」）。取り出しの手がかりが文言でも、確かめは値の照合なので K1 として残す、という扱いでよいかを確かめる。
    -   本物のパーサで解析しつつ、文書が含むべき呼び出しの一覧を直書きしているもの（`TestDocumentedInvocationsParse` の `cmd*` の一覧）。解析の部分（K3）と、直書きの一覧の部分（D2）を分ける。
    -   `TestSlackDocsContract` のように、コードの定数を直書きのリテラルと等しいと確かめてから、そのリテラルで文書を探すもの（`16,383`）。K1 として扱えるかを決める。
    -   `TestCopiedTranscriptSurveyRecords` の、`testdata/` のファイルの一覧と文書の照合（K1）と、出所の記録の項目名（`取得日` など。`docs/dev/security.md` 7 章の規則に対応する）の存在。項目名を K2 として扱うか D2 として扱うかを決める。
    -   `TestContextAndCommandsAgree` のうち、`_context.md` の行を `fixprguard` のパーサで解析する部分（K1）、`wantReviewers` との照合（K4）、文言の部分（D1・D2）、`fixpr.md` などに名前を書かないことの確かめ（D3 か、単一の情報源の確かめか）。
    -   `TestPostingCommandsInventory`（コマンドを解析する。K3 の候補）と、同じファイルの `TestPostingPathsCheckMention`（文の固定と順序）の分け方。
-   **H-02: 共有の補助の扱い。** `cmd/yt2column` パッケージの補助（`readDoc`、`sectionByHeading`、`findTable`、`flat`、`requireSection`、`requirePhrases` など）は、複数のファイルのテストが使っている。`cmd/yt2column-server/docs_test.go` には同じ名前の補助の写しがある（`:356` のコメント）。削除の後に残る使い手を確かめ、使われなくなる補助と、その単体テスト（`TestSectionByHeading`、`TestSectionByHeadingPrefix`、`TestClaudeBullet`、`TestWithoutSection`、`TestCutSection`、`TestGuideTable` など）の扱いを決める。
-   **H-03: 削除の順序とテスト間の結合。** `TestGuideDescribesReviewerList`（`cmd/fixprguard/docs_test.go:311`）の 9 章の確かめは、`cmd/yt2column/fixpr_docs_test.go` のソースに `var wantReviewers ` と `func TestContextReviewerRowsMalformed(` があることを grep で確かめている。片方だけを先に変えると、もう片方が落ちる。削除する順序と、PR の分け方を決める。
-   **H-04: `go tool cover -func` の確かめの手段。** 文書のテストの多くは製品のコードを通らないが、`TestArtifactDocsContract`（`run -h`）、`TestOutputLayoutDocsContract`（`FilePublisher` の出力）、`TestDocumentedInvocationsParse`（`parse()`）のように通るものがある。比べる範囲（パッケージ、`-tags test`、`-coverpkg`）と、記録の形を決める。
-   **H-05: 拡張のジョブの起動条件。** `metadataDocs.test.ts` の判定の結果、node のテストが読む文書が変わる場合の、`has-extension-changes.sh`、`ciChanges.test.ts`、`CLAUDE.md` の変更を決める。`README.md` は `serverReasons.test.ts` が読み続けるので、起動条件に残る見込みである。
-   **H-06: タスクの文書の改訂の範囲。** AC の表の行に加えて、本文（たとえば 0035 の `03_implementation_plan.md` 1.4、0012 の計画の検査 1〜6）が削除したテストを確かめの手段として述べている箇所の扱いを決める。過去のタスクの本文は記録であり、AC の確かめの表だけを改めて本文は改めない、という選択肢もある。AC-13 を満たす範囲を決める。0010 の計画には古いパス（`cmd/docs_test.go::TestReviewEvaluationRecords`）の行がある。
-   **H-07: `TestSharedTokenRemoved` の分け方。** 同じテストが、Go のコードと拡張のソース（残す）と、文書（D3 で削除）の両方を調べている。README の「Upgrading from the shared-token version」節を除外する仕組み（`withoutSection` など）も、文書の確かめを削除すると不要になる。
