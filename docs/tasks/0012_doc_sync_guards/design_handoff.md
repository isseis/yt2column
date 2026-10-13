# 設計への申し送り：文書の同期 guard を node から Go へ移す

本書は、[01_requirements.md](01_requirements.md) のレビュー（と起草）で挙がった事項のうち、設計の段階で決めるものを集める。`02_architecture.md` は、各項目について、採った方針か、当てはまらない理由を記録する。

## H-01: 移した guard の置き場所

-   **事項:** 移した 6 種類の検査（F-001）を、どの Go のパッケージのどのファイルに置くかを決める。候補: 既存の `cmd/yt2column/docs_test.go` に加える、同じ `cmd/yt2column` パッケージに別の `_test.go` を足す、新しいパッケージ（例: `internal/...`）を作る。
-   **理由:** 検査はリポジトリのファイルを読むだけで、製品のコード（`cmd/yt2column` の `main` パッケージ）の関数に依存しない。既存の `docs_test.go` は README・`project_overview.md`・`security.md` とタスク `0005`・`0006`・`0009` の文書を同じ `package main` の `_test.go`（`//go:build test`）で検査している。`make test` は `./...` を対象にするので、どのパッケージに置いても実行される。
-   **候補の方針:** 文書の guard は 1 か所に集約する。`cmd/yt2column` は「文書の guard の置き場」として既に機能しているので、同じパッケージに置く。`docs_test.go` のヘルパ（`readDoc`・`markdownTables`・`findTable`・`docSection`）を再利用する（H-02）。拡張に固有の検査（拡張 ID・`ext-` ターゲット・permission・`extension/`）は、`main` に新しい関数を足さず、テストのファイルに閉じる。
-   **関連:** F-001、AC-01、保守性

## H-02: 既存の `cmd/yt2column/docs_test.go` との役割の境界と、ヘルパの再利用

-   **事項:** 既存の `docs_test.go` と移した検査の役割の境界をどこに引くか、Markdown のヘルパ（見出しの節の切り出し、表の分解）をどう共有するかを決める。
-   **理由:** 既存の `docs_test.go` は、README の設定表・終了コード表、`project_overview.md` の設定表、`security.md` の Slack・Claude の記載、タスクの手動実行の記録を検査している。移す検査は対象のファイルが重なる（`README.md`・`security.md`・`project_overview.md`・実装計画）が、照合する内容は重ならない。`docs.test.ts` の `sectionByHeading`・`tableRows`・`sliceBetween`・`bodyAfterHeadingLine` に相当する処理は、`docs_test.go` の `docSection`・`markdownTables`・`findTable` にほぼ対応する。
-   **候補の方針:** 役割の境界は「既存の検査は CLI と各プロバイダの契約、移す検査はブラウザ拡張の契約」とする。既存のヘルパで足りるものは再利用し、足りないもの（`#### 3.12.1.` のような 3 階層の見出しで節を切る、`## ` で切る `docSection` の一般化）だけを足す。`docs_test.go` の既存の検査を移設や改名で壊さない。`docs.test.ts` の `sectionByHeading` は「同レベル以下の次の見出しまで」で切るが、`docSection` は `## ` 固定なので、`### 8.1.` や `#### 3.12.1.` を切るには一般化が要る。既存の `docSection` の挙動を変えて既存の検査を壊さないよう、新しいヘルパを足すか、既存を後方互換に一般化する。
-   **関連:** F-001、AC-01〜AC-07、保守性

## H-03: 文書のみの変更を CI で保護するか

-   **事項:** 移した検査は `make test` に含まれるが、`ci.yml` の `check-changes` は `*.md`・`docs/`・`.claude/`・`LICENSE` だけの変更をコードの変更とみなさないため、Go のテストのジョブ（`test`）は文書のみの変更で走らない。この現状を本タスクで変えるか、ローカルの `make test`（`CLAUDE.md:204`）に委ねるかを決める。
-   **理由:** `CLAUDE.md:206-210` の規則が存在したのは、文書のみの変更が拡張の CI のジョブを走らせず、拡張の文書 guard が壊れ得るためである。移設だけでは、文書のみの変更で CI が文書 guard を走らせるようにはならない（移設前から、拡張の guard も Go の `docs_test.go` も文書のみの変更では CI で走っていない）。したがって移設は CI の穴を広げも狭めもしないが、ローカルの規則を外す以上、その穴を明示的に決める必要がある。
-   **候補の方針:** 本タスクでは CI のトリガを変えず、文書のみの変更はローカルの `make test`（node 不要）に委ねる。`CLAUDE.md` には「文書のみの変更でも `make test` を実行する」ことを残す。「文書のみの変更でも `test` のジョブを走らせる」案（`check-changes` の分類に、文書 guard が読むファイルを加える、または常に走る小さなジョブを足す）は、実行時間の増加と `check-changes` の設計の変更を伴うので、必要なら別の issue とする。
-   **関連:** F-003、AC-10、AC-11、スコープ外（CI の変更）

## H-04: 承認済みのタスク `0007` の文書の訂正の扱い

-   **事項:** `extension/test/docs.test.ts` を削除すると、タスク `0007` の承認済み文書の `docs.test.ts` への言及が古くなる。これを訂正するか、履歴として残すかを決める。
-   **理由:** 該当する記述は、`03_implementation_plan.md` の 3 節（英語の例外の説明）、ステップ 5-6（`test/docs.test.ts` を作る）、ステップ 5-8（壊し確認に `make ext-check` を使う）、§5.1 の説明（`docs.test.ts::manual checks are recorded`（ステップ 5-6））、`02_architecture.md` の関連箇所である。`03_implementation_plan.md` の §5.1 の表そのものは、移した検査 6 が読む（AC-07）ので、内容を変えない。
-   **候補の方針:** 履歴の記録（ステップ 5-6・5-8 と §5.1 の実行の記録）は、当時そうだった事実なので書き換えない。ただし、現在も有効な指針として読める記述（3 節の例外の説明、§5.1 の「`docs.test.ts::manual checks are recorded` が確かめる」という現在形の文）は、移設後の場所を指すよう訂正する。これは `requirements_process.md` §0 の「Editorial correction」に当たる（決定は変わらない）ので、`0007` の両文書の `Document Status` の `Comments` に、決定を変えない訂正である旨を記す。訂正の要否と範囲は設計で決める。
-   **関連:** F-002、制約（タスク `0007` の扱い）

## H-05: 拡張 ID の導出と manifest の読み取りの Go 実装

-   **事項:** `extension/static/manifest.json` の `key` から拡張 ID を導出する Go の実装と、manifest の `permissions` を読む方法を決める。
-   **理由:** 現行の `extensionIdFromManifestKey`（`extension/test/helpers/extensionId.ts`）は、`key` を base64 から DER の SubjectPublicKeyInfo に戻し、`crypto.createPublicKey` で解析し、DER の SHA-256 の先頭 16 バイトを 16 進で表し、各桁 `0`〜`f` を `a`〜`p` に置き換える。Go では `encoding/base64`・`crypto/x509`（`ParsePKIXPublicKey`）・`crypto/sha256` で同じ計算ができる。`encoding/json` で manifest を読む。
-   **候補の方針:** manifest は `permissions` を `[]string` として読む構造体に `json.Unmarshal` する。`key` は base64 の DER を `x509.ParsePKIXPublicKey` に渡す。`key` の解析に失敗したら、テストは落ちる（現行 TS の `assert.throws` と同じ）。拡張 ID の導出はテストのヘルパ関数とし、検査 1・5 で共有する。文書側の固定値（`clfmbbcdpnjcefbdihdoahomaifbabkk`）は照合に使うだけで、文書の検査は導出値で行う（AC-02）。
-   **関連:** F-001、AC-02、互換性

## H-06: `extension/test` のヘルパの再利用と削除

-   **事項:** `extension/test/docs.test.ts` を削除した後、`extension/test/helpers/extensionId.ts`・`repoFile.ts`・`paths.ts` を残すか削除するかを決める。
-   **理由:** `extensionIdFromManifestKey` は `manifest.test.ts` も使う（`extensionId.ts` は残す）。`readRepoFile`（`repoFile.ts`）は `repository.test.ts` も使う（残す）。`paths.ts` は `repoFile.ts` が使う。`docs.test.ts` だけが使っていたものはない可能性が高いが、削除後に `make ext-lint`・`make ext-typecheck` で未使用がないことを確かめる必要がある（AC-09）。
-   **候補の方針:** 他のテストが使うヘルパは残す。`docs.test.ts` だけが使っていたヘルパ・import・文字列リテラル（日本語の見出しと照合するためのリテラルを含む）は削除する。削除の後、`make ext-check` が通ることを確かめる。Go 側に移した検査の日本語の見出し・表のセルのリテラルは、`CLAUDE.md`「Go Idioms」の例外（日本語の文書の見出しや表のセルと照合するためのリテラル）に当たる。
-   **関連:** F-002、AC-08、AC-09

## H-07: `.gitignore` の ignore 規則の guard の扱い

-   **事項:** `.gitignore` を守る guard（`extension/test/repository.test.ts` の `git check-ignore` で `extension/node_modules`・`extension/dist`・`*.pem` を検査する部分）を node 側に残すか、Go へ移すかを決める。
-   **理由:** `CLAUDE.md:206-210` の規則は、文書だけでなく `.gitignore` の変更も「拡張の guard テストを壊し得る」対象として `make ext-test` を求めている。parenthesis の「the ignore rules」がこの検査を指す。移す 6 種類の検査（`docs.test.ts`）には含まれない。`.gitignore` のみの変更で node を要求しないようにするには、この検査も Go（`make test`）へ移す必要がある。Go のテストから `git` を起動することになる点は、既存の Go のテスト（`docs_test.go` はファイルを読むだけ）にない要素であり、トレードオフがある。
-   **候補の方針:**
    -   **案 A（node に残す）:** ignore 規則の検査は node の `repository.test.ts` に残し、`.gitignore` のみの変更では引き続き `make ext-test` を要求する。`CLAUDE.md` にその旨を明記する。変更は最小で、挙動のテストは node に残すという方針（F-002）と整合する。
    -   **案 B（Go へ移す）:** ignore 規則の検査も Go へ移し、`make test` で行う。Go のテストが `git check-ignore`（または Go での glob の評価）を使う。`.gitignore` のみの変更でも node が不要になり、issue の意図（docs-only で node を要求しない）に完全に一致するが、Go のテストに外部コマンド（`git`）への依存を持ち込む。
    -   判断の材料: `.gitignore` のみの変更の頻度、`git` をテストから起動してよいかの既存の方針、issue #180 の意図（表題は「docs-only の変更で node を要求しない」）。
-   **関連:** F-003、AC-10、AC-12、スコープ（`.gitignore`）
