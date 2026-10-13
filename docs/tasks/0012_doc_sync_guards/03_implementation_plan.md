# 実装計画書：文書の同期 guard を node から Go へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-13 |
| Review date | 2026-10-13 |
| Reviewer | isseis |
| Comments | PR-1 の実装時、本計画が基準とした `f94ae62` より後に `cmd/yt2column/docs_test.go` が変わっており（`TestProjectOverviewDocumentsConfig`・`TestClaudePlanRecordsManualRun` は削除、`TestReviewEvaluationRecords` ほかに置き換え、`tableContains` は `tableFlagContains` に改名）、§1.3・§1.4・ステップ 1-1・1-11・PR-1 のレビュー観点・PR-1 の判定理由・§6.1 の参照が古くなっていた。これらを現在のコードに合わせて編集上訂正した。決定は変えていない。 |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のコードと文書の行番号は、断りがない限り HEAD `f94ae62`（ブランチ `doc-guard-refactor`）で確かめた。ただし `cmd/yt2column/docs_test.go` の行番号と既存の doc guard の名前は、PR-1 の実装時に現在の HEAD で確かめ直した。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める 4 つの機能要件（F-001〜F-004）を、architecture の設計どおりに実装する。`extension/test/docs.test.ts`（node）が行っている文書のテキスト同期の 6 種類の検査を、node を必要としない Go のテスト（`cmd/yt2column/extension_docs_test.go`）へ移し、`make test` に含める。移した検査は node 側から削除し、同じ検査を二重に持たない。文書だけの変更で `make ext-test` を要求しないよう `CLAUDE.md` の規則を見直し、`.gitignore` だけの変更では引き続き `make ext-test` を要求する形にする。最後に、移した各検査が対象を壊すと落ちることを確かめ、その手順と結果を本計画の §5.1 に記録する。

本計画のフェーズ 1〜4 は、architecture §8 の実装優先順位のフェーズ 1〜4 と同じ名前・同じ順序である。用語（文書の同期 guard、guard、挙動のテスト、`ext-` ターゲット、拡張 ID、壊し確認）は architecture の冒頭の「用語」に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。特に、移した guard は既存の doc guard と同じ `cmd/yt2column` パッケージに置いてヘルパを共有し（原則 2）、拡張 ID は manifest の `key` から導出して文書に固定値として置かず（原則 4）、ignore 規則の guard は node 側に残す（原則 7）。
- 新設・変更するファイルは architecture §3.7 の責務表を基本とする。同表が挙げないテスト（追加のヘルパ単体テスト）は、そのステップに対象ファイルとして明記する。
- Go のコメント・識別子・文字列リテラルは英語で書く。ただし、日本語の文書の見出し・表のセルと照合するための文字列リテラルは `CLAUDE.md`「Go Idioms」の例外に当たるので日本語で書く。`AC-NN`・`F-NNN`・`H-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- **検査 6 が照合する行ラベルを、Go ソースに `AC-<数字>` のリテラルとして書かない。** `runplan.md` ステップ 6.5 の Check 1（`\bAC-[0-9]+[a-z]?\b`）が、変更した Go ファイルのソースにある `AC-<数字>` を拒否するためである。期待するラベルは、番号を受け取ってラベルを返す小さなヘルパ（例: `acLabel(2)` が単独の行、`acLabel(10, 11)` が `・` でつないだ組み合わせの行を返す）で組み立て、期待する 11 行の表を番号の並びとして読める形に保つ。このラベルは計画文書への参照ではなく、文書の表と照合するデータである。ヘルパにその旨と、Check 1 に掛からないよう組み立てている理由をコメントで書き、検査の回避を隠さない。文字列 `AC-<数字>` を直接書くと、この検査に掛かる。
- 各テストは、対象の分岐を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。フェーズ 4 で、移した 6 種の検査の壊し確認を行い、結果を §5.1 に記録する。
- 各フェーズの完了条件は、`make fmt` の後に `make test` と `make lint`（グリーンゲート、`_context.md`）が通ることである。フェーズ 2 は `make ext-check` も通す。フェーズ 2 の後は node のテストが文書を読まないので、文書（`CLAUDE.md`）だけを変えるフェーズ 3 では、フェーズ 3 で書く規則のとおり `make ext-test` を要求しない。

### 1.3. 既存コード調査結果

- **`cmd/yt2column/docs_test.go`（フェーズ 1 で変える）。** `//go:build test` の `package main`。再利用するヘルパは、`readDoc`（`:289-296`）、`markdownTables`（`:301-324`）、`splitTableRow`（`:328-336`）、`isTableSeparator`（`:339-350`）、`findTable`（`:353-365`）、`tableRow`（`:369-376`）、`tableRowsWithFirstCell`（`:381-389`）、`tableFlagContains`（`:395-408`）、`stripBackticks`（`:411-413`）、`stepBlock`（`:467-489`）、パス定数（`:20-27`）である。新しいファイルにこれらを複製しない。ステップ 1-1・1-2 で加える `headingLevel`（`:419-433`）・`sectionByHeading`（`:440-463`）もこのファイルに置く。
- **`docSection`（`docs_test.go:360-381`）。** `## ` の見出しだけを扱い、見出し行を含めずに次の `## ` までを返す。呼び出しは `TestReviewEvaluationRecords`（`:138`）と、その補助の `columnEvaluationVideoIDs`（`:193`）の 2 か所だけである。`### 8.1.`・`#### 3.12.1.` を切れないので、architecture §3.5 のとおり汎用の `sectionByHeading` に置き換える（§1.4）。
- **`repositoryRoot`（`cmd/yt2column/makefile_test.go:21`）。** `"../.."`。拡張の文書 guard もリポジトリのルートからの相対パスに使う。
- **`extension/test/docs.test.ts`（フェーズ 2 で削除）。** 検査は 6 つで、`sectionByHeading`・`sliceBetween`・`bodyAfterHeadingLine`・`tableRows` を自前で持つ。拡張 ID は `manifest.json` の `key` から `extensionIdFromManifestKey` で導出する（`:89-93`）。検査 2 は `Makefile` を行頭 `ext-<名前>:`、`CLAUDE.md` を行頭 `` - `make ext-<名前>` `` で集め、重複を除いて比較する（`:105-123`）。検査 6 は §5.1 の表の 0 起点の 2〜4 列だけを空でないことの対象にしており、確認の内容（1 列目）は見ていない（`:214`）。architecture §3.3 のとおり、移す際に確認の内容も対象に加える（現行の表は 1 列目がすべて埋まっているので、既存の文書は落ちない）。
- **`extension/test/helpers/`（変更なし）。** `extensionId.ts`（`:9-20`）は `manifest.test.ts:7,85,89` と `docs.test.ts:4,93` が使う。`repoFile.ts` は `repository.test.ts:14` と `docs.test.ts:5` が使う。`paths.ts` は `repoFile.ts` と `checkDist.test.ts:16`・`typecheck.test.ts:8`・`lintRules.test.ts:6`・`ciChanges.test.ts:6`・`checkLockfile.test.ts:9` が使う。`docs.test.ts` の削除で未使用になるヘルパはない見込みであり、フェーズ 2 で `make ext-check` により確かめる。
- **`extension/test/repository.test.ts`（変更なし）。** `:57-75` の「build outputs are ignored」は `git check-ignore --quiet` で `extension/node_modules/...`・`extension/dist/...`・`key.pem`・`extension/key.pem` を検査する（ignore 規則の guard。architecture §3.6 で node 側に残す）。`:229-258` の「go targets do not need node」は `make -n build test lint deadcode` の出力に `node`・`npm`・`npx` が現れないことを検査する。これらは変更しない。
- **`CLAUDE.md`（フェーズ 3 で変える）。** 変える規則は `:211-215` の「Development Notes」の箇条書きである。`:209` の「編集の後に `make test` と `make lint` を実行する」は維持する。
- **`Makefile`（変更なし）。** `test:` のレシピ（`:68-69`）は `go test -tags test -race -v ./...` で、`node`・`npm` を呼ばない。`ext-` ターゲットは `ext-install`（`:255`）・`ext-typecheck`（`:260`）・`ext-lint`（`:264`）・`ext-fmt-check`（`:268`）・`ext-fmt`（`:273`）・`ext-test`（`:277`）・`ext-build`（`:282`）・`ext-check`（`:287`）の 8 つである。`.PHONY`（`:49`）の行は行頭が `.PHONY` なので検査 2 の抽出に一致しない。
- **`extension/static/manifest.json`（読み取りのみ）。** `key` は `:5`、`permissions` は `:14` に `["activeTab","contextMenus","scripting","storage"]` を持つ。
- **guard が読む文書（変更しない）。** `README.md:267`（拡張 ID）、`docs/dev/security.md:95`（§8.1 の見出し）・`:102`（拡張 ID）、`docs/dev/project_overview.md:7`（`## 決定済みの方針`。`:9` の `TypeScript`）・`:71`（`## 想定ディレクトリ構成`。`:91` の `extension/`）、`docs/tasks/0007_browser_extension_skeleton/02_architecture.md:810`（`#### 3.12.1.`）・`:846`（`#### 3.12.2.`）・`:873`（`#### 3.12.3.`）、同 `03_implementation_plan.md:443`（`### 5.1. 手動の確認と記録`）。`#109 への含意は…`（`02_architecture.md:839,865`）は `#` の直後に空白がないので見出しではない（architecture §3.2）。
- **`internal/pipeline/pipeline_test.go` の guard（変更なし）。** `TestFakesCarryBuildTag`（`:534`）は `testutil/` のファイル・`test_helpers*.go`・テスト専用パッケージのソースの第一行を固定し、`_test.go` は対象にしない（architecture §3.8）。本計画は `testutil/` や `test_helpers*.go` を追加しないので、固定するファイル数は変わらない。
- **`extension/package.json:12`。** テストのスクリプトは `node --test "test/**/*.test.ts"`。`docs.test.ts` を消しても、ほかの `.test.ts` は残る。
- **外部前提の確認。** Go は `go.mod` の版で標準ライブラリのみを使う（`encoding/base64`・`crypto/x509`・`crypto/sha256`・`encoding/json`・`regexp`・`strings`・`strconv`・`maps`・`slices`・`fmt`）。外部モジュールは追加しない（requirements 制約・`CLAUDE.md`「Dependencies」）。`x509.ParsePKIXPublicKey` は公開鍵を解析し、`x509.MarshalPKIXPublicKey` は DER の SubjectPublicKeyInfo を再エンコードする（`go doc crypto/x509`）。`extensionIdFromManifestKey`（`extension/test/helpers/extensionId.ts`）と同じ値になることは、検査 1・5 が文書に記録された値と照合することで確かめる（architecture §3.4・§7.1）。
- **意図して変えないもの。** 拡張の挙動のテスト（`manifest.test.ts` ほか）、`Makefile`、`.github/workflows/ci.yml` のジョブの分類・トリガ、ignore 規則の guard（node 側）、`0011` の拡張 ID の記載（`docs/tasks/0011_localhost_server/01_requirements.md:131,137,220`。guard の対象外。architecture §11）。
- **計画上の判断（architecture の決定は変えない）。**
  - AC-10（`CLAUDE.md` の規則）と AC-13（壊し確認の記録）は、文書の内容そのものについての基準である。この 2 つは、architecture §3.1・§3.7 が設計する 6 種の guard とは別であり、新たな guard を足さず、requirements §3.1 の「静的確認」のとおり、文書を読んで確かめる（`requirements_process.md` §4 は、文書の存在についての基準に `static` の検証を認める）。architecture §3.9 の AC の対応もこの扱いのままである。
  - 新しいヘルパ（`sectionByHeading`・`sectionByHeadingPrefix`）は architecture §3.2 が契約を定めるので、その境界の挙動は小さな単体テスト（`TestSectionByHeading` など）で固定する（architecture §3.7 の `docs_test.go` の変更の一部）。`extensionIDFromManifestKey` には、固定値の正しさのテストを足さない（architecture §7.1）。ただし、解析できない `key` でエラーを返す分岐は、このヘルパの単体テストで 1 件だけ確かめる（固定値によらない）。

### 1.4. 名前の変更の一覧

本計画の他の箇所は、この表に従う。

| 変更前 | 変更後 | ステップ |
|---|---|---|
| `docSection(doc, heading string) string`（`cmd/yt2column/docs_test.go:360`） | `sectionByHeading(doc, heading string) string`（同じファイル。呼び出しは `## ` を含む見出し行全体を渡す） | 1-1 |

### 1.5. 申し送り（design_handoff）への対応

`implementation_handoff.md` はない。`design_handoff.md` の H-NN はすべて architecture §10 が方針を記録しており、本計画は architecture のとおりに実装する。

| 項目 | 本計画での扱い |
|---|---|
| H-01（置き場所） | フェーズ 1 で `cmd/yt2column/extension_docs_test.go` を新設する（architecture §3.7） |
| H-02（境界とヘルパの再利用） | ステップ 1-1 で `docSection` を `sectionByHeading` に置き換え、表のヘルパを再利用する |
| H-03（文書だけの変更を CI で保護するか） | 変えない。CI の分類・トリガはそのまま（architecture §7.4・§11。#181） |
| H-04（タスク `0007` の文書の訂正） | 編集しない。`docs.test.ts` への言及は履歴として残す（architecture §10） |
| H-05（拡張 ID と manifest の読み取り） | ステップ 1-4 で実装する（architecture §3.4） |
| H-06（`extension/test` のヘルパの再利用と削除） | フェーズ 2 で `docs.test.ts` を削除し、他のテストが使うヘルパは残す |
| H-07（ignore 規則の guard） | node 側に残す。`.gitignore` だけの変更では `make ext-test` を要求するよう `CLAUDE.md` に明記する（フェーズ 3） |

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ヘルパと Go の guard（F-001）

**対象ファイル:** `cmd/yt2column/docs_test.go`（変更）、`cmd/yt2column/extension_docs_test.go`（新設）

- [x] **ステップ 1-1**: `docs_test.go` の `docSection` を汎用の `sectionByHeading` に置き換える（契約は architecture §3.2）。呼び出し 2 か所（`TestReviewEvaluationRecords`・`columnEvaluationVideoIDs`）を `sectionByHeading(doc, "## <見出し>")` に直す。既存の検査（`TestReviewEvaluationRecords`）の期待値は変えない。
- [x] **ステップ 1-2**: `docs_test.go` に、`sectionByHeading` の境界の挙動を固定する小さな単体テストを加える。見出し行が結果に含まれないこと、同じか高いレベルの次の見出し行の手前で止まること、見出しがないとき空文字列を返すこと、`#` の直後に空白がない行を見出しと見なさないことを確かめる。

### PR-1 作成ポイント: general heading-section helper

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `refactor(0012): generalize the doc section helper for heading levels`

**レビュー観点**: `sectionByHeading` が `## `・`### `・`#### ` のいずれでも「同じか高いレベルの次の見出し行の手前まで」で切り、見出し行を結果に含めないこと（ステップ 1-1・1-2、architecture §3.2） / 呼び出しを直した既存の検査（`TestReviewEvaluationRecords`）の期待値が変わっていないこと（ステップ 1-1、§1.4 の名前の変更の一覧） / `#` の直後に空白がない行（例 `#109 への含意は…`）を見出しと見なさないこと（ステップ 1-2、architecture §3.2）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 が、既存の doc guard（`TestReviewEvaluationRecords`）の切り出す範囲を変えないまま `docSection` を汎用の `sectionByHeading` に一般化する、振る舞いを保つ一般化の判断を要する隔離したステップであり、この PR に隔離してレビューするため（architecture §3.5、§6.1 のリスク）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 1-3**: `extension_docs_test.go` を新設する（`package main`、先頭行は `//go:build test`）。`readDoc`・`findTable`・`sectionByHeading`・`repositoryRoot` を再利用する（表の分解 `markdownTables` は `findTable` の内部で使う）。`manifest.json`（`key`・`permissions`）を `encoding/json` で読む。manifest と各文書はテスト関数の冒頭で 1 回だけ読み、サブテストで共有する（requirements §4.1）。
- [x] **ステップ 1-4**: `sectionByHeadingPrefix(doc, prefix string) string` を実装する（契約は architecture §3.2。前後の空白を除いて `prefix` で始まる最初の見出し行を探し、その行を含めず、次の見出し行の手前までを返す。見出しがなければ空文字列）。`extensionIDFromManifestKey(key string) (string, error)` を実装する（`encoding/base64`・`crypto/x509`・`crypto/sha256`。解析できない値はエラーを返す。契約は architecture §3.4）。解析できない `key` でエラーを返すことを確かめる小さな単体テストを加える（固定値によらない）。
- [x] **ステップ 1-5**: 検査 1（拡張 ID の記載。`README.md`・`docs/dev/security.md`）のサブテストを実装する。導出値が両文書の本文に含まれることを確かめる（architecture §3.3）。
- [x] **ステップ 1-6**: 検査 2（`ext-` ターゲットの一覧。`Makefile`・`CLAUDE.md`）のサブテストを実装する（抽出規則は architecture §3.3 の契約）。両者から集めた名前の集合が一致することを確かめ、`Makefile` から 1 件も集まらなければ失敗させる。
- [x] **ステップ 1-7**: 検査 3（manifest の permission。`docs/dev/security.md` §8.1）のサブテストを実装する。§8.1 の `要求する権限` の列挙と manifest の `permissions` を集合として両方向に比較し、どちらか片方だけにある権限（manifest にあって文書にない権限、文書にあって manifest にない権限）を落とす（architecture §3.3）。
- [x] **ステップ 1-8**: 検査 4（`docs/dev/project_overview.md` の記述）のサブテストを実装する（architecture §3.3）。
- [x] **ステップ 1-9**: 検査 5（`0007` の `02_architecture.md` の調査の記録）のサブテストを実装する（architecture §3.3）。
- [x] **ステップ 1-10**: 検査 6（`0007` の `03_implementation_plan.md` §5.1）のサブテストを実装する（照合の内容は architecture §3.3）。表の本文の行の数が期待する行の数と一致することも確かめ、余分な行・欠けた行のどちらでも落ちるようにする（現行 `docs.test.ts` の `deepEqual` と同じ）。期待する行ラベルは §1.2 のとおりヘルパで番号から組み立て、Go ソースに `AC-<数字>` のリテラルを書かない。
- [x] **ステップ 1-11**: フェーズ 1 の完了条件として、`make fmt` → `make test` → `make lint` を通す。既存の doc guard（`TestReviewEvaluationRecords`）と README の guard（`TestREADMEDocumentsFlags`）が引き続き通ることを確かめる。

**完了条件:** 6 種の検査が `make test` で通り、node も npm も起動しない。既存の doc guard の期待値が変わっていない。

### PR-2 作成ポイント: Go port of the extension doc-sync guards

**対象ステップ**: 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11

**推奨タイトル**: `feat(0012): port the extension doc-sync guards to Go tests`

**レビュー観点**: 拡張 ID を `manifest.json` の `key` から導出し（base64 → DER → `x509` で公開鍵に再エンコード → SHA-256 の先頭 16 バイト → 各桁 `a`〜`p`）、文書側に固定値を置かないこと（ステップ 1-4・1-5、AC-02、architecture §3.4）と、`sectionByHeadingPrefix` の境界（見出し行を結果に含めず、レベルを問わず次の見出し行の手前で止まり、見出しがなければ空文字列）が小さな単体テストで固定されていること（ステップ 1-4、§1.3、architecture §3.2） / 6 種の検査が `make test` で走り、node も npm も起動しないこと、かつ各 guard を実装した PR-2 のコミットで対象を壊すと落ちることを確かめてコミットメッセージに記録していること（ステップ 1-3〜1-10、AC-01・AC-11、`CLAUDE.md`「Testing Strategy」。体系的な 6 種の記録は PR-5） / `ext-` ターゲットの抽出規則（`Makefile` は行頭 `ext-<名前>:`、`CLAUDE.md` は行頭 `` - `make ext-<名前>` ``）と、`Makefile` から 1 件も集まらないときに失敗させること（ステップ 1-6、AC-03、architecture §3.3） / 検査 6 の期待する行ラベルを `AC-<数字>` のリテラルで書かず、番号からヘルパで組み立てること（ステップ 1-10、§1.2。`runplan.md` ステップ 6.5 の Check 1 に掛からないため）

**実装モデル要件**: standard

**判定理由**: 6 種の検査の照合の内容、`sectionByHeadingPrefix` と拡張 ID の導出の方法が architecture §3.2〜§3.4 で確定しており、`既存コード調査結果` に競合する実装方針の併記がなく、パネルモードの引き金・2 つ以上の Conditional check・隔離すべき高リスクなステップのいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した（[#189](https://github.com/isseis/yt2column/pull/189)）
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: node 側の削除（F-002）

**対象ファイル:** `extension/test/docs.test.ts`（削除）

- [x] **ステップ 2-1**: `extension/test/docs.test.ts` を削除する。移した 6 種の検査と同じ内容を node 側に残さない。
- [x] **ステップ 2-2**: `extension/test/helpers/` の利用状況を確かめ、`docs.test.ts` だけが使っていた import・文字列リテラルが残っていないこと、他のテストが使うヘルパ（`extensionId.ts`・`repoFile.ts`・`paths.ts`）が残っていることを確認する。不要なヘルパ・import があれば削除する。
- [x] **ステップ 2-3**: フェーズ 2 の完了条件として、`make ext-check`（typecheck・lint・fmt-check・test・build。`Makefile:287-292`）を通す。未使用の import・文字列リテラルで `make ext-lint`・`make ext-typecheck` が落ちないことを確かめる。

**完了条件:** `extension/test/docs.test.ts` がなく、移した 6 種の検査が `make ext-test` に含まれない。拡張に残る挙動のテストが `make ext-check` で通る（AC-08・AC-09）。

### PR-3 作成ポイント: remove the duplicated node doc-sync tests

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `refactor(0012): remove the node doc-sync tests moved to Go`

**レビュー観点**: `extension/test/docs.test.ts` を削除し、移した 6 種の検査を node 側に残していないこと（ステップ 2-1、AC-08） / `docs.test.ts` だけが使っていた import・文字列リテラルが残らず、他のテストが使うヘルパ（`extensionId.ts`・`repoFile.ts`・`paths.ts`）が残っていること（ステップ 2-2、AC-09） / `make ext-check` が型検査・lint・fmt-check・test・build のすべてで通ること（ステップ 2-3、AC-09）

**実装モデル要件**: standard

**判定理由**: 移した検査の node 側の複製の削除と未使用のヘルパの始末に限られ、`既存コード調査結果` に競合する実装方針の併記がなく、パネルモードの引き金・2 つ以上の Conditional check・隔離すべき高リスクなステップのいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 規則の見直し（F-003）

**対象ファイル:** `CLAUDE.md`（変更）

- [ ] **ステップ 3-1**: `CLAUDE.md:211-215` の規則を、architecture §3.7 の内容 (a)〜(c) を満たす形に直す。(a) 文書（`README.md`・`CLAUDE.md`・`docs/`）だけの変更では `make ext-test` を要求しない。(b) 文書のテキスト同期の guard は `make test`（`:209` が求める）に含まれ、node を必要としない。(c) `.gitignore` だけの変更では、ignore 規則の検査が node 側に残るので `make ext-test` を要求する。文言は実装で決める。`CLAUDE.md:209` の規則は維持する。
- [ ] **ステップ 3-2**: フェーズ 3 の完了条件として、`make fmt` → `make test` → `make lint` を通す。移した検査 2 が `CLAUDE.md` の `ext-` ターゲットの一覧を読むので、`make test` がこの変更を守る。

**完了条件:** `CLAUDE.md` の規則が文書だけの変更で `make ext-test` を要求せず、`.gitignore` だけの変更の扱いを明記している（AC-10）。

### PR-4 作成ポイント: revise the development note for doc-only changes

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `docs(0012): drop the node requirement for documentation-only changes`

**レビュー観点**: `CLAUDE.md` の「Development Notes」が (a) 文書だけの変更では `make ext-test` を要求しないこと、(b) 文書のテキスト同期の guard は node を必要としない `make test` に含まれること、(c) `.gitignore` だけの変更では ignore 規則の検査が node 側に残るので `make ext-test` を要求すること、の 3 点を明記していること（ステップ 3-1、AC-10・AC-12、architecture §3.7） / `CLAUDE.md:209` の「編集の後に `make test` と `make lint` を実行する」規則を維持していること（ステップ 3-1） / 検査 2 が読む `ext-` ターゲットの一覧を変えていないので `make test` が通ること（ステップ 3-2、AC-03）

**実装モデル要件**: standard

**判定理由**: `CLAUDE.md` の文言の変更だけで、満たす内容 (a)〜(c) は architecture §3.7 で確定しており、`既存コード調査結果` に競合する実装方針の併記がなく、パネルモードの引き金・2 つ以上の Conditional check・隔離すべき高リスクなステップのいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 壊し確認と記録（F-004）

**対象ファイル:** `docs/tasks/0012_doc_sync_guards/03_implementation_plan.md`（本計画の §5.1）

- [ ] **ステップ 4-1**: 移した 6 種の各検査について、architecture §7.2 が挙げる対象を壊し、その検査のサブテストだけを実行して、それが落ちることを確かめる。主な対象は、拡張 ID の 1 文字、manifest の `key` を別の有効な値に変える（検査 1・5 の導出元）、`Makefile` の `ext-` ターゲット 1 つ、`security.md` §8.1 の `要求する権限` から permission 1 つを削除する操作、manifest の `permissions` の permission 1 つ（manifest から消す場合に落ちることも確かめる。検査 3 の集合の比較が両方向であることの確認）、`project_overview.md` の `TypeScript` と `extension/`、`0007` の `3.12.x` の見出しと `3.12.2`・`3.12.3` の本文、`0007` の §5.1 のセルである。壊した変更は、対象ファイルのほかの変更を巻き込まずに戻し（壊し確認の前に、対象ファイルに無関係な未ステージ変更がないことを確かめる）、戻した後に `make test` が通ることを確かめてから次の検査に進む。
- [ ] **ステップ 4-2**: ステップ 4-1 の手順と結果を §5.1 の表に記録する（形式はタスク `0007` の §5.1 に倣う）。
- [ ] **ステップ 4-3**: フェーズ 4 の完了条件として、`make fmt` → `make test` → `make lint` を通す。

**完了条件:** 6 種の各検査の壊し確認の手順と結果が §5.1 に記録されている（AC-13）。

### PR-5 作成ポイント: break-check record for the migrated guards

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0012): record the break-check results for the doc-sync guards`

**レビュー観点**: 移した 6 種の各検査について、対象を壊すと該当のサブテストだけが落ち、戻した後に `make test` が通ることを確かめていること（ステップ 4-1、AC-13） / §5.1 の各欄（検査・壊す対象・実行したテスト・日付・結果）が空でなく、タスク `0007` の §5.1 と同じ形式であること（ステップ 4-2、AC-13） / 壊した変更を戻し忘れていないこと（ステップ 4-1・4-3、§6.1 のリスク）

**実装モデル要件**: standard

**判定理由**: 既に実装した guard の壊し確認と、その結果を本計画の §5.1 に記録する作業に限られ、`既存コード調査結果` に競合する実装方針の併記がなく、パネルモードの引き金・隔離すべき高リスクなステップのいずれにも該当しないため。複数の成果物に固定された対象を一時的に壊す手順（ステップ 4-1）という Conditional check に 1 つだけ該当するが、ステップ 4-1 が 1 件ごとに `git restore` で戻し、ステップ 4-3 がグリーンゲートを通すので、2 つ以上の基準には当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物と完了の合図 |
|---|---|---|
| M1 | フェーズ 1 | `cmd/yt2column/extension_docs_test.go` の 6 種の検査が `make test` で通り、既存の doc guard が変わらず通る |
| M2 | フェーズ 2 | `extension/test/docs.test.ts` がなくなり、`make ext-check` が通る |
| M3 | フェーズ 3 | `CLAUDE.md` の規則が新しくなる |
| M4 | フェーズ 4 | §5.1 の壊し確認の記録がそろい、全体のグリーンゲート（`make test`・`make lint`）が通る |

### 3.2. PR 構成

PR はフェーズ 1 を 2 つに分け、そのほかはフェーズと 1 対 1 に対応させる（PR-1〜PR-5）。PR-1 は既存 `docs_test.go` の `docSection` を汎用の `sectionByHeading` に置き換えるヘルパの変更だけを行い、既存の doc guard を回帰させるリスクを新設の guard のレビューから分離する。PR-2 はその汎用ヘルパを使って `extension_docs_test.go` の 6 種の検査を実装する。フェーズ 2（PR-3）は移した検査の node 側の複製を削除し、フェーズ 3（PR-4）は `CLAUDE.md` の規則を直し、フェーズ 4（PR-5）は移した検査の壊し確認とその記録を行う。各 PR は単独でグリーンゲートを通せる単位とする。

グリーンゲートは `_context.md` の "Green gate"（`make test && make lint`）とする。拡張のファイルを変える PR-3 だけは、これに `make ext-check` を加える（`extension/test/docs.test.ts` の削除後も残る node のテストが型検査・lint・fmt-check・test・build を通ることを確かめるため）。PR-1 の `sectionByHeading` は PR-2 の検査が使うので、PR-1 は PR-2 に先行する（architecture §3.5・§3.8）。ステップ番号の順と文書の順は一致し、各 `### PR-N 作成ポイント` は対象のステップのすべての後にある。

`CLAUDE.md`「Testing Strategy」が求める「テストをコミットする前に、対象を壊して落ちることを確かめ、コミットメッセージに書く」は、各 guard を実装する PR-2 のコミットで満たす。PR-5 は、移した 6 種の検査をあらためて体系的に壊し確認し、その手順と結果を §5.1 にまとめて記録する（§1.2 の「フェーズ 4 で…記録する」はこの体系的な記録を指す）。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 | `docs_test.go` の `docSection` を汎用の `sectionByHeading` に置き換え、呼び出しを直し、境界の単体テストを足す | frontier-recommended |
| PR-2 | 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 | `extension_docs_test.go` の 6 種の検査と、`sectionByHeadingPrefix`・`extensionIDFromManifestKey` のヘルパ | standard |
| PR-3 | 2-1 / 2-2 / 2-3 | `extension/test/docs.test.ts` の削除と、未使用になったヘルパ・import の始末 | standard |
| PR-4 | 3-1 / 3-2 | `CLAUDE.md`「Development Notes」の規則（文書だけの変更で `make ext-test` を要求しない） | standard |
| PR-5 | 4-1 / 4-2 / 4-3 | 6 種の検査の壊し確認と、§5.1 の記録 | standard |

### 3.3. 実装順序の根拠

architecture §8 の優先順位（フェーズ 1: ヘルパと Go の guard、フェーズ 2: node 側の削除、フェーズ 3: 規則の見直し、フェーズ 4: 壊し確認と記録）のとおりにする。フェーズ 1 を先に行うのは、移した検査が `make test` で通ってからでないと、node 側の削除（フェーズ 2）で検査のない期間ができるためである。フェーズ 3 は、移した検査が `make test` にあることを前提に `CLAUDE.md` の規則を直す。フェーズ 4 は、壊し確認の記録を本計画の §5.1 に残すので最後に行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. 静的 guard

移した 6 種の検査そのものが、文書・`Makefile`・manifest の契約を固定する静的 guard である（architecture §7.1）。`make test` の一部として実行され、`make lint` でも `-tags test,integration` でコンパイルされる。`yt-dlp`・LLM API・Webhook を呼ばず、node も起動しない。`extensionIDFromManifestKey` の計算の正しさに専用の固定値テストは足さない。文書に記録された値（`README.md`・`security.md`・`0007` の §3.12.1）と照合することで確かめる（architecture §3.4・§7.1）。ヘルパ（`sectionByHeading`・`sectionByHeadingPrefix`・`extensionIDFromManifestKey`）は、それぞれの境界とエラーの分岐を小さな単体テストで固定する。

### 4.2. 壊し確認

`CLAUDE.md`「Testing Strategy」に従い、フェーズ 4 で移した 6 種の各検査の対象を壊し、そのサブテストだけが落ちることを確かめて戻す（対象と手順はステップ 4-1）。各壊し確認は、対象の分岐を通るサブテストを名指しで実行する。手順と結果は §5.1 に記録する。

### 4.3. 実行環境

Go の guard は macOS と Linux（CI）のどちらでも `make test` で動く。node も npm も必要としない（`Makefile:68-69`）。拡張の経路（`make ext-check`）は従来どおり `.node-version`（`24.21.0`）の node と `make ext-install`（`npm ci`。ネットワーク）を必要とする。手元にその環境がない場合の扱いは §6 に記す。

### 4.4. テストヘルパー

新しいテストのファイルは `_test.go` であり、`testutil/` や `test_helpers*.go` を追加しない（`test_organization.md` の Classification A は使わない）。共有のヘルパ（`readDoc`・`markdownTables`・`findTable`・`sectionByHeading`）は既存の `cmd/yt2column/docs_test.go` に置き、`extension_docs_test.go` から使う（architecture §3.2・§3.7）。`docs_test.go` が既に同じパッケージのヘルパを `_test.go` の中に持つ前例に従う。したがって `TestFakesCarryBuildTag` の固定するファイル数は変わらない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | test | `cmd/yt2column/extension_docs_test.go` の 6 種のサブテスト。`make test`（`Makefile:68-69`）で実行され、node・npm を起動しない |
| AC-02 | test | 検査 1・5 のサブテスト（導出値で照合）。ステップ 4-1 の壊し確認（文書の拡張 ID の 1 文字、manifest の `key`） |
| AC-03 | test | 検査 2 のサブテスト。ステップ 4-1 の壊し確認（`Makefile` の `ext-` ターゲット 1 つ、`CLAUDE.md` の一覧） |
| AC-04 | test | 検査 3 のサブテスト。ステップ 4-1 の壊し確認（`manifest.json` の permission の追加と削除、`security.md` §8.1 の `要求する権限` の記載の削除） |
| AC-05 | test | 検査 4 のサブテスト。ステップ 4-1 の壊し確認（`project_overview.md` の `TypeScript`・`extension/`） |
| AC-06 | test | 検査 5 のサブテスト。ステップ 4-1 の壊し確認（`3.12.x` の見出しの削除、`3.12.2`・`3.12.3` の本文を空にする） |
| AC-07 | test | 検査 6 のサブテスト。ステップ 4-1 の壊し確認（§5.1 の行の削除、セルを空にする） |
| AC-08 | static / test | `extension/test/docs.test.ts` が存在せず、`extension/test/` に移した 6 種の検査（`extensionIdFromManifestKey` を文書と照合する、`CLAUDE.md`・`security.md`・`project_overview.md`・`0007` の文書を読む）が残っていないこと（static。差分と `git grep` で確かめる）。`make ext-check`（`Makefile:287-292`）が削除した状態で通ること（test。これは AC-09 の確認であり、検査が残っていても通るので AC-08 の不在の確認にはならない） |
| AC-09 | test | `make ext-check`（`ext-typecheck`・`ext-lint`・`ext-fmt-check`・`ext-test`・`ext-build`） |
| AC-10 | static | `CLAUDE.md`「Development Notes」の節が architecture §3.7 の (a)〜(c)（文書だけの変更は `make ext-test` 不要、移した guard は `make test`、`.gitignore` だけの変更は `make ext-test`）を明記していること（requirements §3.1 は静的確認とする） |
| AC-11 | test / static | `extension_docs_test.go` の 6 種が `make test` で実行されること（test）と、`extension/test/repository.test.ts::go targets do not need node`（`make -n test` の出力に node・npm が現れない。static） |
| AC-12 | test / static | `extension/test/repository.test.ts::build outputs are ignored`（`git check-ignore`。`make ext-test` で実行）と、`CLAUDE.md` が `.gitignore` だけの変更で `make ext-test` を要求する旨を明記していること（static） |
| AC-13 | test / static | ステップ 4-1 の 6 種の壊し確認（`extension_docs_test.go` の該当サブテストを壊した状態で実行）と、§5.1 の記録（静的確認） |

### 5.1. 壊し確認の記録

実装のときに埋める。記録の欄（壊す対象・実行したテスト・日付・結果）は空にしない。

| 検査 | 壊す対象 | 実行したテスト | 日付 | 結果 |
|---|---|---|---|---|
| 検査 1（拡張 ID の記載） | | | | |
| 検査 2（`ext-` ターゲットの一覧） | | | | |
| 検査 3（manifest の permission） | | | | |
| 検査 4（project overview の記述） | | | | |
| 検査 5（`0007` の調査の記録） | | | | |
| 検査 6（`0007` の手動確認の記録） | | | | |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `docSection` を `sectionByHeading` に置き換えると、既存の検査（`TestReviewEvaluationRecords`）の切り出す範囲が変わりうる | 既存の doc guard が落ちる | `sectionByHeading` は「同じか高いレベルの次の見出しまで」で切る。ステップ 1-2 の単体テストで境界を固定し、ステップ 1-1 の `make test` で既存の検査が通ることを確かめる |
| `sectionByHeading`・`sectionByHeadingPrefix` は、フェンスで囲んだコードブロックの中の `# ` で始まる行（シェルのコメントなど）も見出しとみなす。`docSection` は `## ` だけを見たので、置き換え後は既存の検査の節もこの行で切れる | 文書のコードブロックにシェルのコメントを足すと、節が途中で切れて guard が落ちる（現行の対象の節にはそのような行はない） | 現行 `docs.test.ts` と同じ定義（architecture §3.2）なので挙動は変えない。guard が落ちたときの原因として、失敗のメッセージに対象の見出しを含める |
| Go の `extensionIDFromManifestKey` の導出値が node の値と一致しない | 検査 1・5 が落ちる、または誤った値を受け入れる | 文書に記録済みの値（`README.md`・`security.md`・`0007` §3.12.1）と照合する。一致しない場合は `x509` の使い方（`ParsePKIXPublicKey`・`MarshalPKIXPublicKey`）を見直す |
| 検査 6 の期待する行ラベルを Go ソースに `AC-<数字>` のリテラルで書く | `runplan.md` ステップ 6.5 の Check 1 が失敗する | §1.2 のとおり `AC-` と番号から組み立てる |
| `extension/test/docs.test.ts` だけが使っていたヘルパ・import が残る | `make ext-lint`・`make ext-typecheck` が落ちる | ステップ 2-2 で利用状況を確認し、ステップ 2-3 の `make ext-check` で確かめる |
| 手元に `.node-version` の node（`24.21.0`）や `extension/node_modules` がない | `make ext-check`・`make ext-test` を実行できない | 固定された版の node と `make ext-install` を用意して実行する。用意できない場合は、`docs.test.ts` の削除が node 側の検査の成功に依存しないこと（削除だけであり、残るテストの期待値を変えないこと）を確かめ、実行しなかったことを PR に書く |
| 文書だけの変更では CI の Go のテストのジョブも拡張のジョブも走らない（`ci.yml:36`） | 移した guard が CI で守られない | 本タスクでは変えない（H-03。`CLAUDE.md:209` のローカルの `make test` に委ねる）。残るリスクは architecture §7.4 のとおり。CI で守る案は別のタスク（#181） |
| 壊し確認のときに文書を壊したまま戻し忘れる | 文書の契約が壊れたまま PR に入る | ステップ 4-1 のとおり、各壊し確認の後に `git restore -- <対象ファイル>` で戻し、`make test` が通ることを確かめてから次に進む |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
- [ ] PR-2 マージ済み（対象ステップ: 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11）
- [ ] PR-3 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
- [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2）
- [ ] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）
- [ ] §5 のすべての AC の検証が通り、§5.1 の記録がそろっている

## 8. 成功基準 (Success Criteria)

- **機能:** 移した 6 種の検査が Go のテストとして `make test` で通り（AC-01）、node 側から同じ検査が消え（AC-08〜AC-09）、`CLAUDE.md` の規則が文書だけの変更で `make ext-test` を要求しない（AC-10〜AC-12）。
- **品質:** `make test`・`make lint`・`make ext-check` が通り、既存の doc guard と拡張の挙動のテストが変わらず通る。各検査が対象を壊すと落ちることが確認され、§5.1 に記録されている（AC-13）。
- **セキュリティ:** 新しい外部モジュールを追加しない。guard はコミットされたファイルだけを読み、秘密情報を扱わない。拡張 ID は manifest の `key`（公開情報）から導出する。
- **文書:** `CLAUDE.md` の「Development Notes」の規則が architecture §3.7 の内容 (a)〜(c) を満たす。他の文書（`README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・タスク `0007` の文書）は現行の記載を維持する。

## 9. 次のステップ (Next Steps)

- PR の境界は §3.2 に埋め込み済みである（2026-10-13）。`/runplan 0012` で PR-1 から実装する。
- 文書だけの変更を CI で守る案（H-03）と、`.gitignore` の ignore 規則の検査を Go へ移す案（H-07 の案 B）は、別のタスク（#181 など）として扱う（architecture §11）。
