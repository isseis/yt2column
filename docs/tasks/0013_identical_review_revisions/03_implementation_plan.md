# 実装計画書：変更のない検証・推敲の項目

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-14 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のコードとテストの行番号は、断りがない限り HEAD `edb914d` で確かめた。architecture は行番号を `56062fd` で確かめているが、`56062fd` から `edb914d` までの間に本計画が触れるファイル（`internal/writer/revision.go`・`review.go`・`revision_test.go`・`review_test.go`・`steps_test.go`・`internal/publisher/reviewlog.go`・`reviewlog_test.go`）は変わっていない（`git log 56062fd..edb914d -- <これらのパス>` が空）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める 2 つの機能要件（F-001・F-002）を、architecture の設計どおりに実装する。検証・推敲の応答の項目で `before` と `after` が同じ文字列である「無変更の項目」を、既存の受理の規則のまま受理し（F-001）、`--review-log` の一覧ではその項目に固定の 1 行 `No change.` を加えて変更がなかったことを示す（F-002）。受理の規則・当てはめ（`applyRevisions`）・要約・失敗の種類・変更のある項目の書式は変えない。

本計画のフェーズ 1〜3 は、architecture §8 の実装優先順位の 3 項目と同じ名前・同じ順序である（判定・書式・統合の確認）。用語（無変更の項目、一覧、一覧のファイル、要約、文面）は architecture の冒頭の「用語」に従う。

### 1.2. 実装原則

- architecture §1.1 の 6 つの設計原則に従う。特に、無変更の判定は `writer.Revision` の意味として型に置き（原則 1）、一覧のファイルを書き出す `internal/publisher` は自前の比較を書かず `Revision.NoChange()` を呼ぶ。
- 変更するファイルは architecture §2.2 の責務表のとおりとする。テストは同表のとおり既存の `_test.go` に加える。新しいファイル・`testutil/` ヘルパ・`test_helpers*.go` は作らない。
- 受理の規則（`Revision.check`・`parseReviewResponse`・`ReviewResult.Check`）と、当てはめ、要約、エラーの種類、変更のある項目の書式は変えない（architecture §3.3）。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。`No change.` は固定の英語の文字列リテラルである。
- 変更した分岐を対象とする各テストは、対象の分岐を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。手順は §4.2 に従う。変更しないコード（`applyRevisions`）を対象とする既存の回帰テストは、この壊し確認の対象外である。
- 各フェーズの完了条件は、グリーンゲート（`_context.md` の "Green gate"、`make test && make lint`）と `make fmt` が通ることである。

### 1.3. 既存コード調査結果

- **`Revision`（`internal/writer/revision.go:42-48`）。** フィールド集合は `Before`・`Action`・`After`・`Reason`・`Evidence` の 5 つで、本計画はこれを変えない。`NoChange()` を加えるのに伴い、`Revision.check()`（同 `:53`）と `ReviewResult.Check()`（同 `:26`）の doc コメントに、`ActionReplace` で `Before == After` の項目を受理する（拒否しない）ことを 1 文加える（architecture §3.1）。検査の条件そのものは変えない。
- **`Before` と `After` の比較は、既存のコードに無い。** `revision.go` 全体と `internal/` の非テストコードを `Before == After`・`Before != After` で検索して確認した（`internal/writer/review.go:116` は `ActionReplace` の代入のみ）。したがって、判定は `NoChange()` の 1 か所にだけ置く。
- **`NoChange` と `No change.` は既存のコードに無い。** リポジトリ全体を検索した結果、`NoChange` という識別子と `No change.` という文字列リテラルは存在しない（`Makefile:205` の `No changed Go files to format` は無関係な別の文字列）。名前の衝突は無い。
- **既存の受理・当てはめは、無変更の項目を既に受理している（コードは変えない）。** `parseReviewResponse`（`internal/writer/review.go:38`）・`parseRevision`（同 `:75`）・`applyRevisions`（同 `:149-192`）は `Before == After` を調べない。`Revision.check`（`revision.go:53-85`）も、空値・制御文字・未知の action と reason だけを拒否する。`applyRevisions` は `Before` の範囲を `After` で置き換えるだけなので、両者が同じなら文面は変わらない（architecture §1.1 原則 2・4）。
- **`renderReviewLog`（`internal/publisher/reviewlog.go:74-93`）。** 項目ごとに `Before:` の引用（`:84`）に続けて、削除なら `Deleted.`（`:85-86`）、それ以外は `After:` の引用（`:87-89`）を書く。本計画は、`ActionReplace` かつ `revision.NoChange()` のとき `After:` の引用の直後に `No change.` の 1 行を加える（architecture §3.2。`Deleted.` と同じ位置、すなわち `Evidence:` の直前）。先頭の `- Revisions:` の件数（`:76-77`）と `quoteBlock`（`:96-102`）は変えない。`internal/publisher` は既に `internal/writer` を import している（`:10`）。
- **再利用するテストとヘルパ。**
  - `internal/writer/revision_test.go`: `TestReviewResultCheck`（`:63`）と `validRevision`（`:20`）。無変更の項目の受理のケースを `TestReviewResultCheck` の `accepted` 表に加える。
  - `internal/writer/review_test.go`: `TestParseReviewResponse`（`:62`）・`TestApplyRevisions`（`:203`）と、応答を組み立てる `reviewResponse`・`replaceItem`・`deleteItem`（`:12-31`）。無変更の項目は、`before` と `after` に同じ値を渡す `replaceItem` で作る。
  - `internal/writer/steps_test.go`: `TestWriteReviewApplies`（`:329`）と、`validTranscript`（`internal/writer/test_helpers.go:51`）・`withStepsEnabled`（同 `:99`）・`newStepClient`（同 `:108`）・`stepResult`（同 `:114`）・`mustNew`（同 `:119`）。`Write` の経路はここで確かめる。
  - `internal/publisher/reviewlog_test.go`: `TestReviewLogWriterFormat`（`:28`）と `reviewArticle`（`:19`）。`validArticle`（`file_test.go:17`）と `dirEntries`（同 `:28`）も再利用する。
- **ガードテストは更新しない（architecture §7）。** `internal/pipeline/pipeline_test.go` の `TestCommonTypesFieldSets`（`:378`）は `Article` のフィールドを固定し、`Revision` のメソッドは対象にしない。本計画はフィールド集合を変えず、`exportedFieldTypes` はメソッドを見ないので通る。`TestInterfaceDocComments`（同 `:455`）は 4 つのインタフェースの doc コメントを固定し、`ReviewResult.Check` は対象外である。`TestFakesCarryBuildTag`（同 `:534`）は `testutil/` と `test_helpers*.go` の第一行を固定するが、本計画はそれらを追加しないので固定するファイル数は変わらない。
- **更新しない文書（architecture §1.1 原則 3・§2.2）。** `docs/dev/security.md:75`（§6）は一覧のファイルに入る値を既に扱っており、追加する `No change.` は固定のリテラルなので変更を要しない。`docs/dev/developer_guide/package_reference.md` は一覧のファイルの書き出しの流れを述べるが書式は記述しない。0010 の `02_architecture.md` §3.8・`02_evaluation.md` は編集しない（現行の書式は 0010 §3.8 と architecture §3.2 の和とする）。
- **外部前提。** 標準ライブラリの `strings` だけを使い、外部モジュール・外部コマンド・ネットワーク・CI のスキーマ・コンテナを新たに使わない（requirements §5・`CLAUDE.md`「Dependencies」）。検証は Go のテストだけであり、`make test` の実行以外に確かめる外部前提は無い。

### 1.4. 申し送りへの対応

`implementation_handoff.md`・`design_handoff.md` はどちらも無い。したがって本計画が対応すべき `I-NN`・`H-NN` の項目は無い。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 判定

**対象ファイル:** `internal/writer/revision.go`（変更）、`internal/writer/revision_test.go`（変更）

- [ ] **ステップ 1-1**: `internal/writer/revision.go` に `func (r Revision) NoChange() bool` を加える。定義は architecture §3.1 のとおりとし、`r.Action == ActionReplace` かつ `r.Before != ""` かつ `r.Before == r.After` のときだけ真を返す。doc コメントは英語で書き、「受理された形の置換（空でない `Before` とそれに等しい `After`）のときに真」であることと、一覧のファイルの書き出しが使うことを述べる。
- [ ] **ステップ 1-2**: `Revision.check()`（`revision.go:53`）と `ReviewResult.Check()`（同 `:26`）の doc コメントに、`ActionReplace` で `Before == After` の項目を受理する（拒否しない）ことを述べる 1 文を加える。検査の条件（`switch`・`if` の本体）は変えない。この文言の追加を機械的に検査するガードは無い（architecture §7。`TestInterfaceDocComments` は 4 つのインタフェースの doc コメントだけを対象にする）。文言はステップ 1-5 のグリーンゲートとレビューで確かめる。
- [ ] **ステップ 1-3**: `internal/writer/revision_test.go` に `TestRevisionNoChange` を加える。真偽表は architecture §7 のとおり、（真）`ActionReplace` で `Before == After` かつ `Before` が空でない、（偽）`ActionReplace` で `Before != After`、（偽）`ActionReplace` のゼロ値（`Before` と `After` が空）、（偽）`ActionDelete`、（偽）未知の action を確かめる。`validRevision` を基にした値を使う。
- [ ] **ステップ 1-4**: `TestReviewResultCheck`（`revision_test.go:63`）の `accepted` 表に、`ActionReplace` で `Before == After` の `Revision` を 1 件加える（既存の `accepted`・`rejected` の各行は変えない）。
- [ ] **ステップ 1-5**: フェーズ 1 の完了条件として、`make fmt` → `make test` → `make lint` を通す。既存の `TestReviewResultCheck` の `rejected` の各行が引き続き通ることを確かめる。

**完了条件:** `NoChange()` が真偽表のとおりに動き、無変更の項目が `ReviewResult.Check` で受理される。受理の規則と既存の拒否のケースは変わっていない。

### フェーズ 2: 書式

**対象ファイル:** `internal/publisher/reviewlog.go`（変更）、`internal/publisher/reviewlog_test.go`（変更）

- [ ] **ステップ 2-1**: `renderReviewLog`（`internal/publisher/reviewlog.go:74-93`）の項目ごとの分岐に、`ActionReplace` かつ `revision.NoChange()` のとき `After:` の引用の直後に `No change.` の 1 行を書く場合を加える（architecture §3.2・§6 の図3）。`Deleted.` の分岐、`Before != After` の置換の分岐、先頭の `- Revisions:` の件数、`quoteBlock` は変えない。`No change.` の位置は `Deleted.` と同じ（`Evidence:` の直前）とする。
- [ ] **ステップ 2-2**: `TestReviewLogWriterFormat`（`internal/publisher/reviewlog_test.go:28`）に、無変更の項目のサブテストを 1 件加える。`reviewArticle`（`reviewlog_test.go:19`）で `ActionReplace` かつ `Before == After` の `Revision` を持つ記事を作り、書き出したファイル全体を文字列リテラルと比べる。`Before:`・`After:`・`No change.`・`Evidence:` がこの順で読めることを確かめる（AC-05・AC-06）。既存の `empty list`・`replacement`・`deletion with a multiline evidence` の各サブテストは変えない。
- [ ] **ステップ 2-3**: フェーズ 2 の完了条件として、`make fmt` → `make test` → `make lint` を通す。既存の `TestReviewLogWriterFormat` の置換・削除のサブテストが引き続き通ることを確かめる。

**完了条件:** 無変更の項目の一覧のファイルに `No change.` が `Evidence:` の直前に 1 行入り、変更のある項目の書式と先頭の件数は変わっていない。

### フェーズ 3: 統合の確認

**対象ファイル:** `internal/writer/review_test.go`（変更）、`internal/writer/steps_test.go`（変更）

- [ ] **ステップ 3-1**: `TestParseReviewResponse`（`internal/writer/review_test.go:62`）に、無変更の項目を含む応答を受理するケースを 1 件加える。`replaceItem` に同じ `before` と `after` を渡して無変更の項目を作り、解析結果がその 1 項目を返すことを確かめる（AC-01）。既存の受理・拒否のケースは変えない。
- [ ] **ステップ 3-2**: `TestApplyRevisions`（`review_test.go:203`）の `accepted` 表に、無変更の項目を当てはめた結果が当てはめの対象と同じ文面になるケースを 1 件加える（AC-03）。既存の `accepted`・`rejected` の各行は変えない。
- [ ] **ステップ 3-3**: `internal/writer/steps_test.go` の `TestWriteReviewApplies`（`:329`）に、`Write` が無変更の項目の応答で成功することを確かめるサブテストを加える。無変更の項目だけの応答と、それに変更のある項目を加えた応答の両方で `Write` が成功し、ゼロ値でない `Article` を返し、`Review.Revisions` に無変更の項目が応答の順序どおりに含まれることを確かめる（AC-02）。既存のサブテストと同じ組み立て（`newStepClient`・`withStepsEnabled`・`validTranscript`）を使い、既存のサブテストは変えない。無変更の項目と変更のある項目の両方について、`before` が生成後の文面に一度だけ現れる値にする（`applyRevisions` は `before` が複数回現れると拒否するため）。
- [ ] **ステップ 3-4**: フェーズ 3 の完了条件として、`make fmt` → `make test` → `make lint` を通す。既存の拒否のケース（`TestReviewResultCheck`・`TestParseReviewResponse`・`TestApplyRevisions` の `rejected`、`TestWriteReviewRejects`）が引き続き通ることを確かめる（AC-04）。

**完了条件:** 無変更の項目が解析・検査・当てはめ・`Write` のどの経路でも受理され、既存の拒否の規則と変更のある項目の扱いが変わっていない。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物と完了の合図 |
|---|---|---|
| M1 | フェーズ 1 | `Revision.NoChange()` が真偽表のとおりに動き、無変更の項目が `ReviewResult.Check` で受理される |
| M2 | フェーズ 2 | 一覧のファイルの無変更の項目に `No change.` が入り、変更のある項目の書式は変わらない |
| M3 | フェーズ 3 | 解析・当てはめ・`Write` の各経路で無変更の項目が受理され、全体のグリーンゲート（`make test`・`make lint`）が通る |

### 3.2. 実装順序の根拠

architecture §8 の優先順位（1: 判定、2: 書式、3: 統合の確認）のとおりにする。フェーズ 2 の `renderReviewLog` は `Revision.NoChange()` を呼ぶので、判定（フェーズ 1）が先である。フェーズ 3 は、判定と書式がそろったうえで、応答から記事に至る経路で無変更の項目が受理されることをまとめて確かめる。フェーズ 3 のテストはフェーズ 1 の `NoChange()` に依存しないが、architecture の順序に従う。

PR の境界は本計画では定めない。`/mkplan2 0013` が承認後に埋め込む。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

ユニットテストは API もネットワークも呼ばない（0010 の F-007・AC-26 と同じ）。無変更の項目は、fake の応答（`internal/writer` のテストの応答の部品 `reviewResponse`・`replaceItem`）と、直接組み立てた `writer.Article`（`internal/publisher` の `reviewArticle`）で作る。確かめる対象は次のとおり。

- `NoChange()` の真偽表（`TestRevisionNoChange`、ステップ 1-3）。
- 無変更の項目の受理: `ReviewResult.Check`（`TestReviewResultCheck`、ステップ 1-4）と `parseReviewResponse`（`TestParseReviewResponse`、ステップ 3-1）。
- 当てはめが文面を変えないこと（`TestApplyRevisions`、ステップ 3-2）。
- `Write` が無変更の項目の応答で成功し、`Review.Revisions` に応答の順序どおりに残ること（`steps_test.go`、ステップ 3-3）。
- 一覧のファイルの無変更の項目の書式（`TestReviewLogWriterFormat`、ステップ 2-2）。
- 既存の拒否・変更のある項目の扱いが変わらないこと（既存の `TestReviewResultCheck`・`TestParseReviewResponse`・`TestApplyRevisions` の `rejected`、`TestWriteReviewRejects`、`TestReviewLogWriterFormat` の置換・削除のサブテスト）。

統合テストは該当なしである。このタスクはコンテナ・DB・ネットワーク・キャッシュなどの外部状態を触らず、確かめるのは 2 つのパッケージ内の純粋な処理だけである。後方互換は、変更のある項目の受理・拒否と書式が変わらないことを既存の拒否・置換・削除のテスト（AC-04・AC-07）で確かめる。

### 4.2. 壊し確認

`CLAUDE.md`「Testing Strategy」に従い、実装時に次の壊し確認を行い、その結果をコミットメッセージに記録する。手順は、対象の分岐を外す（または常に偽を返すようにする）→ 名指ししたテストだけを実行する → 落ちることを確かめる → 戻して `make test` が通ることを確かめる、の順とする。

- `Revision.NoChange()` の判定を常に偽にすると、`TestRevisionNoChange` の真の行と、`TestReviewLogWriterFormat` の無変更のサブテストが落ちること（後者が、`renderReviewLog` が自前の比較ではなく `NoChange()` を呼んでいることを確かめる）。
- `NoChange()` を `Before` が空のゼロ値にも真を返すようにすると、`TestRevisionNoChange` のゼロ値の行が落ちること。
- `renderReviewLog` の `No change.` の分岐を外すと、`TestReviewLogWriterFormat` の無変更のサブテストが落ちること。
- 無変更の項目の受理を拒否する検査を（一時的に）加えると、`TestReviewResultCheck`・`TestParseReviewResponse`・`TestWriteReviewApplies` の無変更のサブテストが落ちること。
- ステップ 3-2 の `TestApplyRevisions` の無変更の行は、変更しない `applyRevisions` を対象とする回帰テストである。`applyRevisions` は本計画で変えないので、この行には壊し確認を課さず、既存の当てはめが文面を変えないことの回帰として扱う。

### 4.3. テストヘルパー

新しいテストのファイルは既存の `_test.go`（`internal/writer/revision_test.go`・`review_test.go`・`steps_test.go`、`internal/publisher/reviewlog_test.go`）である。`testutil/` のヘルパや `test_helpers*.go` は追加しない（`test_organization.md` の Classification A・B のどちらも使わない）。したがって `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:534`）の固定するファイル数は変わらない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | test | `internal/writer/revision_test.go::TestRevisionNoChange`（真偽表）・`internal/writer/revision_test.go::TestReviewResultCheck`（無変更の項目の受理の行）・`internal/writer/review_test.go::TestParseReviewResponse`（無変更の項目を含む応答の受理のサブテスト）。`make test` で実行する |
| AC-02 | test | `internal/writer/steps_test.go::TestWriteReviewApplies`（無変更の項目だけの応答、および無変更と変更を合わせた応答で `Write` が成功し、`Review.Revisions` に応答の順序どおりに含まれること）。`make test` で実行する |
| AC-03 | test | `internal/writer/review_test.go::TestApplyRevisions`（無変更の項目を当てはめた結果が当てはめの対象と同じ文面になる行）。`make test` で実行する |
| AC-04 | test | `internal/writer/revision_test.go::TestReviewResultCheck`・`internal/writer/review_test.go::TestParseReviewResponse`・`internal/writer/review_test.go::TestApplyRevisions` の既存の `rejected` の行と、`internal/writer/steps_test.go::TestWriteReviewRejects` の既存の拒否の行、`internal/publisher/reviewlog_test.go::TestReviewLogWriterFormat` の置換・削除のサブテスト。`make test` で実行する |
| AC-05 | test | `internal/publisher/reviewlog_test.go::TestReviewLogWriterFormat` の無変更のサブテスト（`Before:`・`After:`・`No change.`・`Evidence:` がこの順で読めること、`No change.` が `Evidence:` の直前にあること）。`make test` で実行する |
| AC-06 | test | 同上のサブテストで、`Before:` の引用・`After:` の引用・`Evidence:` の引用と見出しの理由の値（`reason`）が無変更の項目でも読めることをファイル全体の文字列で固定する。`make test` で実行する |
| AC-07 | test | `internal/publisher/reviewlog_test.go::TestReviewLogWriterFormat` の既存の `replacement`・`deletion with a multiline evidence`・`empty list` の各サブテスト（削除の `Deleted.`、置換の `After:`、先頭の `- Revisions:` の件数が変わらないこと）。`make test` で実行する |

すべて `test`（`make test` が実行する Go のテスト）であり、`static`・`manual` だけの AC は無い。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `NoChange()` を `Before` が空のゼロ値にも真を返すようにすると、受理されない値で真になり、定義域が広がる | 呼び出し元が `Check` を通していない値で誤って `No change.` を書く | architecture §3.1 のとおり `r.Before != ""` を条件に含める。ステップ 1-3 の真偽表のゼロ値の行で固定する（§4.2 の壊し確認で落ちることを確認する） |
| `No change.` を `After:` の引用の代わりに書くと、AC-06 の「`After:` の引用が読める」に反する | 無変更の項目の書式が要件を満たさない | architecture §3.2 のとおり `After:` の引用を残し、`No change.` を `Evidence:` の直前に加える。ステップ 2-2 のファイル全体の文字列で固定する |
| `renderReviewLog` の分岐の追加で、変更のある項目の書式が変わる | 既存の一覧のファイルの読み手の解釈が変わり、AC-07 に反する | 無変更の項目のときだけ `No change.` を書く分岐にし、`Deleted.`・`After:` の分岐と先頭の件数は変えない。ステップ 2-3 で既存のサブテストが通ることを確かめる |
| 無変更の項目の当てはめが、`before` が文面に一度だけ現れるという既存の条件に抵触する | `Write` の無変更のケースが `applyRevisions` の拒否で落ちる | 変更のある項目と無変更の項目の `before` を、生成後の文面に一度だけ現れる値にする（ステップ 3-2・3-3） |
| テストが主張どおりに落ちないままコミットされる | 検証が空振りする | ステップ 1-3・2-2・3-1〜3-3 のテストについて §4.2 の壊し確認を行い、結果をコミットメッセージに記録する |

### 6.2. スケジュールのリスク

本タスクは 3 フェーズ・2 パッケージに閉じ、外部依存が無いため、スケジュール上のリスクは低い。フェーズごとにグリーンゲートを通すので、途中で止めても各フェーズは独立して完結する。

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 のステップ 1-1〜1-5 が完了し、M1 の完了条件を満たす
- [ ] フェーズ 2 のステップ 2-1〜2-3 が完了し、M2 の完了条件を満たす
- [ ] フェーズ 3 のステップ 3-1〜3-4 が完了し、M3 の完了条件を満たす
- [ ] §4.2 の壊し確認をすべて行い、結果をコミットメッセージに記録した
- [ ] §5 のすべての AC の検証が通り、§5 の表の `test` の検証が `make test` で実行される
- [ ] グリーンゲート（`make test && make lint`）が通る

## 8. 成功基準 (Success Criteria)

- **機能:** 無変更の項目が `ReviewResult.Check`・`parseReviewResponse`・`Write` の各経路で受理され（AC-01〜AC-03）、一覧のファイルで `No change.` が `Evidence:` の直前に読める（AC-05・AC-06）。変更のある項目の書式と受理・拒否の規則は変わらない（AC-04・AC-07）。
- **品質:** `make test`・`make lint` が通る。追加した各テストが、§4.2 の壊し確認で対象の分岐を外すと落ちることが確認され、結果がコミットメッセージに記録されている。
- **セキュリティ:** 新しい外部モジュール・外部コマンド・ネットワーク送信・キャッシュの読み書きを加えない。`No change.` は応答の値を含まない固定の英語の 1 行であり、無変更の項目の値は既存と同じく引用ブロックの中に置く（architecture §5）。無変更の判定は `CheckPublishable` の後にだけ使う。
- **文書:** 変更するソースのコメント・識別子・文字列リテラルは英語である。`AC-NN`・`F-NNN` を Go ソースに書かない。0010 の文書と `package_reference.md` は編集しない（architecture §1.1 原則 3・§2.2）。

## 9. 次のステップ (Next Steps)

- 本書が `approved` になったら、`/mkplan2 0013` で PR の境界を本書に埋め込み、その後 `/runplan 0013` でフェーズ 1 から実装する。
- 要約の観測性（無変更の項目を要約の件数から分ける変更）は、要約の書式を定める別のタスクで扱う（architecture §9）。
