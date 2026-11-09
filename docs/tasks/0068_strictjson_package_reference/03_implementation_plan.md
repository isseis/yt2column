# 実装計画書：internal/strictjson の使い手の一覧をコードに合わせ、一覧の置き場所を 1 か所にする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-09 |
| Review date | 2026-11-09 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語（部品・使い手・使い手の一覧・正本）と、設計書の冒頭の用語（一覧の文・照合のテスト・表のテスト・走査・作った入力のテスト）を同じ意味で使う。そのほかに、次の用語を使う。

-   **一回の確認のプログラム:** 設計書 7.3 の「スクリプト」に当たる、`docs/tasks/0068_strictjson_package_reference/_checks/` の Go のプログラム（本計画 §1.3）。
-   **確かめること:** 一回の確認のプログラムが行う個々の検査（ステップ 6-1 の 6 つ）。
-   **基点と head:** 基点はマージの基点（`git merge-base origin/main HEAD`）、head は PR のブランチの先頭のコミットである。

既存のファイルの行番号は、断りがない限りコミット `a452a888`（`origin/main`）で確かめた。設計書が確かめた `68d84224` から `a452a888` までに、`internal/pipeline` と `internal/doctestutil` と `.github/workflows/ci.yml` は変わっていない。`package_reference.md`・`CLAUDE.md`・`project_overview.md`・ADR-0020 は変わったが、設計書が引く行（`package_reference.md:29`、`CLAUDE.md:312-315`、`project_overview.md:112`、`0020-strict-json.md:23`）の位置と中身は変わっていない。確かめには `git diff --stat 68d84224 a452a888 -- internal/pipeline internal/doctestutil docs/dev/conventions/package_reference.md CLAUDE.md docs/dev/project_overview.md docs/dev/adr/0020-strict-json.md .github/workflows/ci.yml` と各行の `sed -n` を使った。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`package_reference.md` の使い手の一覧に 4 つのパッケージを足してコードに合わせ（F-001）、`CLAUDE.md` と `project_overview.md` の古い一覧を要約に置き換える（F-002）。一覧とコードのずれで `make test` を失敗させる照合のテストを `internal/pipeline` に足し（F-003）、ADR-0020 をコードに合わせる（F-004）。

### 1.2. 実装の原則

-   **設計書 8 章の順に、フェーズごとにコミットする。** どのコミットでも `make test` が通るようにする（設計書 8 章）。
-   **失敗の経路は、作った入力のテストと、実物を壊す確認の両方で確かめる（設計書 1.2 の H-05）。** 壊した変更はコミットせず、結果をコミットメッセージに書く。
-   **変更の前後の関係と、head の状態のうち恒久的なテストにしないものは、一回の確認のプログラムで確かめる。** 文の意味は PR のレビューで読む（設計書 7.3・7.4）。

### 1.3. 計画の際に決めたこと（2026-11-09）

人間のレビュアーが決めたもの:

-   **一回の確認は Go のプログラムにする。** 置き場所は `docs/tasks/0068_strictjson_package_reference/_checks/main.go` とする。`go build -tags test -o <scratchpad の中のパス> ./docs/tasks/0068_strictjson_package_reference/_checks/` でビルドし、できた実行ファイルを `[<base> <head>]` の引数で直接走らせる。設計書 7.3 が実装計画に委ねた選択である。理由: ADR の節や表の行を `internal/doctestutil` で読め、AC-12 の確かめ方では照合のテストの文字列リテラルを `go/parser` で取り出せる。違反（終了コード 1）と、確かめられない入力（終了コード 2）も分けられる。シェルでは、一覧の文を除いた行の比較やリテラルの取り出しを正規表現で書くことになり、誤りやすい。先例はタスク 0064・0067 の `_checks` である。`go run` を使わないのは、0067 の計画 §1.3 が記録したとおり、`go run` ではプログラムの終了コードが 1 か 2 かを区別できないためである。ADR の判断: ADR は作らない。このタスクの一回の確認の書き方で、タスクの中で閉じる。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **設計書の仮の名前をそのまま使い、テストは 1 つのファイルに置く。** 新しいファイルは `internal/pipeline/strictjson_users_test.go`（`//go:build test`、`package pipeline`）とする。走査 `goSources`、import の読み取り `importersOf`、表の行の発見 `strictjsonRowCell`（次の項目）、一覧の文の読み取り `parseUserList`、集合の比較 `diffUsers` と、照合のテスト `TestPackageReferenceStrictjsonUsers`、作った入力のテスト（`TestGoSources`・`TestImportersOf`・`TestStrictjsonRowCell`・`TestParseUserList`・`TestDiffUsers`）をここに置く。同じパッケージのほかの `_test.go` と同じく `//go:build test` を付ける（`internal/pipeline/pipeline_test.go:1`、`relay_guard_test.go:1`）。どれもパッケージの中だけで使うので、`testutil/` にも `test_helpers.go` にも置かない（`docs/dev/conventions/test_organization.md`）。
-   **表の行を見つける処理と、失敗のメッセージを組み立てる処理を、照合のテストから関数として切り出す。** 設計書 3.2 の手順 1〜2（表・列・行を見つけ、行がちょうど 1 つであることを確かめる）を、セルを返すかエラーを返す関数にし、作った文書の断片で `TestStrictjsonRowCell` が確かめる。設計書 7.2 は、行の欠落と重複（AC-09）を実物を壊す確認だけで確かめるとしたが、関数にすれば恒久的なテストでも確かめられる。同じく、`diffUsers` の結果から報告の文を組み立てる処理を関数にし、文が名前（と、一覧にない使い手では import するファイル）を含むことを `TestDiffUsers` が確かめる（AC-06〜AC-08 の「失敗の出力はそのパッケージの名前を示す」）。どちらも設計書 2.2 の「名前とファイルの分け方は実装計画で決める」の範囲で、設計の決定は変えない。
-   **一回の確認のプログラムに、設計書 7.3 の 3 つの範囲の確かめ方に加えて、head の状態を見る確かめることを 3 つ置く。** AC-11（`doc-guard` が照合のテストを実行すること）、AC-12（照合のテストが一覧の写しを持たないこと）、AC-15（ADR-0020 を変えるコミットのメッセージの correction line）である。設計書の付録 B は AC-12・AC-15 を manual とし、AC-11 を「静的確認」としたが、どれも文書の文の意味ではなく、コード・設定・コミットの値なので、実行される確かめが要る（mkplan の規則）。manual の読み取りは残し、それを補う。設計の決定は変えない。
-   **一回の確認のプログラムが使う日本語の文字列は、プログラムの横のデータのファイルに置く。** ADR の見出し（`## 決定`・`## 結果`・`## 経緯`）などである。Go のソースの文字列は英語に保つ。guard-test literal exception（`CLAUDE.md`「Go Idioms」）は `_test.go` の guard test と `internal/doctestutil` だけに及び、`_checks/` には及ばないためである。先例はタスク 0067 の `_checks/data.json` である。
-   **PR は 1 本にする。** 設計書 1.2 の決定である。

### 1.4. 既存コード調査結果

| 領域 | すでにあるもの | 足りないもの・変えるもの |
|---|---|---|
| 部品の使い手 | `a452a888` で 11 パッケージ（`internal/artifact`・`internal/codexreview`・`internal/coveragecompare`・`internal/fixprguard`・`internal/llm/claude`・`internal/llm/deepseek`・`internal/nmhost`・`internal/publisher`・`internal/server`・`internal/transcript`・`internal/writer`）。`git grep -l '"github.com/isseis/yt2column/internal/strictjson"' -- '*.go'` から `_test.go` と `docs/` の下のファイルを除き、ディレクトリにまとめた。設計書 3.1 の 11 個と同じで、`68d84224` の後に入った `internal/privatefile` などは使い手でない | 一覧に 4 つを足す（フェーズ 3） |
| 一覧の文 | `docs/dev/conventions/package_reference.md:29` のセルの末尾の `Shared by …, and `internal/fixprguard`` の 7 つ | 設計書 3.1 の形にする（フェーズ 3） |
| 表のテスト | `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:625`）。走査は同 `:639-659`、`prompts` の追加は `:660`、空の確かめは `:661-663` | 走査を `goSources` に置き換える（フェーズ 1、設計書 3.4） |
| 表の読み取り | `packageReferenceRows`（`pipeline_test.go:690`）は最初のコードスパンだけを返す。`internal/doctestutil` の `FindTable`（`tables.go:63`）・`ColumnIndex`（`:79`）・`TableRow`（`:90`、最初の一致だけを返す）・`StripBackticks`（`:100`）・`ReadDoc`（`read.go:13`） | `packageReferenceRows` は変えない。照合のテストは `FindTable`・`ColumnIndex`・`StripBackticks`・`ReadDoc` を使い、行は自分で数える（設計書 3.2） |
| 起点の定数 | `repoRoot`（`internal/pipeline/relay_guard_test.go:21`、値 `"../.."`） | 照合のテストの起点に使う |
| 変えない走査 | `productionGoFiles`（`relay_guard_test.go:26`）、`TestFakesCarryBuildTag` の走査（`pipeline_test.go:555-575`）、`TestWriterImports`・`TestConfigImports` の import の読み取り（`pipeline_test.go:801`・`:859`） | 変えない（設計書 3.4） |
| `doc-guard` | `.github/workflows/ci.yml:187` が `go test -tags test … ./internal/pipeline` を走らせる | 変えない。照合のテストを `internal/pipeline` に置けば AC-11 を満たす |
| CLAUDE.md の項目 | `CLAUDE.md:312-315` の `internal/strictjson` の項目（5 つの使い手を挙げる） | 設計書 3.5 の文にする（フェーズ 4） |
| `project_overview.md` の行 | `docs/dev/project_overview.md:112`（コードブロックの中） | 設計書 3.5 の行にする（フェーズ 4） |
| CLAUDE.md のリンクの確かめ | `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`） | 変えない。足すリンクの行き先を確かめる |
| ADR-0020 | 「決定」の `0020-strict-json.md:23` の項目、「結果」の末尾は `:39`、「経緯」の末尾は `:47` | 設計書 3.8 のとおりに変える（フェーズ 5） |
| ADR の構造の確かめ | `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）。題名・状態・索引・見出しを確かめ、経緯の項目の中身は確かめない | 変えない。フェーズ 5 で通ることを確かめる |
| タスク 0065 の PR #727 | `a452a888` の祖先の `cf76c1f6` が #727 の squash commit で、`origin/main` に入っている（`git log --oneline origin/main`） | 設計書 6.2 の順序の制約は満たされている。マージの前の確かめは要らない |
| 一回の確認の先例 | `docs/tasks/0067_stage_terminology/_checks/main.go`（`//go:build test`、`doctestutil` を使う、終了コード 0・1・2、データのファイル）、`docs/tasks/0064_stderr_drain_test_deadline/_checks/`（`go/parser` で関数を読む） | 同じ形で置く（フェーズ 6） |

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-05 は、設計書の付録 A に記録されている。

## 2. 実装のステップ (Implementation Steps)

各フェーズの終わりに、`make fmt`・`make test`・`make lint` を通してからコミットする。

### フェーズ 1: 走査の共有（設計書 8 章の 1、3.4）

**変更するファイル:** `internal/pipeline/strictjson_users_test.go`（新規）、`internal/pipeline/pipeline_test.go`

-   [ ] **ステップ 1-1**: `goSources` と `goSource` の型を、設計書 3.4 の規則（`cmd/` と `internal/` を歩く、`testdata` と `_`・`.` で始まるディレクトリとファイルを飛ばす、`_test.go` も返す、ビルドタグを見ない）と 3.6 の型のとおりに書く。
-   [ ] **ステップ 1-2**: `TestGoSources` を書く。`t.TempDir` の下に作ったソースで、次を確かめる（設計書 7.1）。
    -   飛ばすもの（`testdata`、`_` で始まるディレクトリ、`.` で始まるディレクトリ、`_` で始まるファイル、`.` で始まるファイル）を返さない。
    -   `cmd/` と `internal/` の外の `.go` ファイル（起点の直下のファイルと、`_` でも `.` でも始まらない別のディレクトリ `other/` の下のファイル）を返さない。実際のリポジトリでは、`cmd/` と `internal/` の外にある唯一の使い手 `docs/tasks/0018_article_quality/_material_check` は `_` の規則でも飛ばされるので、この確かめはこの入力でしか観測できない（CLAUDE.md「A layered path needs inputs only one layer can handle」）。
    -   `_test.go`、`//go:build test`・`//go:build ignore` のファイル、GOOS の接尾辞を持つファイル（`_windows.go`）を返す。
    -   起点のパス自身が `_` または `.` で始まる要素を含んでも歩く（起点を `t.TempDir` の下の `_root/` にする）。
    -   返すパッケージが起点からの `/` 区切りの相対パスである。
    -   空の `cmd/` と `internal/` だけを持つ起点では何も返さない。
-   [ ] **ステップ 1-3**: `TestPackageReferenceListsPackages` の走査（`pipeline_test.go:639-659`）を、`goSources` が返すファイルから設計書 3.4 の 3 種類のパッケージを数える形に置き換える。`prompts` の追加と空の確かめは残す。
-   [ ] **ステップ 1-4**: 置き換えの前後で、`TestPackageReferenceListsPackages` が数えるパッケージの集合が同じであることを確かめる（一時的に集合を出力して比べ、出力はコミットしない）。`.go` ファイルのない `testutil` のディレクトリを一時的に作ると、置き換えた後は数えないことも確かめる。結果をコミットメッセージに書く。
-   [ ] **ステップ 1-5**: `TestGoSources` の各確かめについて、`goSources` の該当する規則を一時的に外して失敗することを確かめ、コミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。壊し方は、少なくとも次の 4 種類を含む。
    -   飛ばす規則のそれぞれを外す。
    -   `cmd/`・`internal/` ではなく起点の全体を歩く。
    -   名前の規則を起点のパスにも当てる。
    -   `_test.go` を返さない。

**完了の条件:** `TestGoSources` と `TestPackageReferenceListsPackages` が通り、`make test`・`make lint` が通る。

### フェーズ 2: 読み取りと比較の部品（設計書 8 章の 2、3.2〜3.4、3.6）

**変更するファイル:** `internal/pipeline/strictjson_users_test.go`

-   [ ] **ステップ 2-1**: `importersOf` を設計書 3.4 のとおりに書く（`_test.go` でないファイルを `parser.ImportsOnly` で読み、与えた import のパスを持つファイルをパッケージごとにまとめる。別名・`_`・`.` の import も数える。読めない、または解析できないファイルはエラー）。
-   [ ] **ステップ 2-2**: `TestImportersOf` を書く。`t.TempDir` の下のソースで、`_test.go` だけが import するパッケージが入らないこと、`//go:build test` のファイルだけが import するパッケージが入ること、別名・`_`・`.` の import が数えられること、構文を解析できないファイルでエラーになることを確かめる（設計書 7.1）。
-   [ ] **ステップ 2-3**: 表の行を見つける関数 `strictjsonRowCell` を書く（本計画 §1.3）。設計書 3.2 の手順 1〜2 を行い、`internal/strictjson` の行の `Responsibility` のセルを返す。表がない、列がない、行がない、行が 2 つ以上ある、の 4 つの場合は、それぞれ別の番兵（`errNoPackageTable`・`errNoResponsibilityColumn`・`errNoStrictjsonRow`・`errDuplicateStrictjsonRow`）を元のエラーとして含めて返す。エラーの文には探した見出し・列名・パッケージ名と見つかった行の数を含める（設計書 4 章）。`TestStrictjsonRowCell` は、作った文書の断片（行が 1 つ、表がない、列がない、行がない、行が 2 つ）で、成功の行はセルを、失敗の行は `errors.Is` で番兵を確かめる（CLAUDE.md「Error Testing」）。
-   [ ] **ステップ 2-4**: `parseUserList` と番兵 `errNoUserList`・`errMalformedUserList` を設計書 3.2・3.6 のとおりに書く。`TestParseUserList` は設計書 7.1 の成功の行と失敗の行を持ち、失敗の行は `errors.Is` で番兵を確かめ、重複した名前を重複を残したまま返すことを確かめる。
-   [ ] **ステップ 2-5**: `diffUsers` と、その結果から報告の文を組み立てる関数を書く（設計書 3.3、本計画 §1.3）。`TestDiffUsers` は、一覧にない使い手、使い手でない名前（存在しない名前と `internal/strictjson` 自身）、重複した名前、3 つとも空の行を持ち、各行で 3 つの結果と、報告の文が該当する名前（一覧にない使い手では import するファイルも）を含むことを確かめる。
-   [ ] **ステップ 2-6**: ステップ 2-2〜2-5 の各テストについて、対象の関数の該当する分岐を一時的に壊して失敗することを確かめ、コミットメッセージに書く。壊し方は、少なくとも次を含む。
    -   `importersOf` が `_test.go` を除かない。別名・`_`・`.` の import を数えない。解析できないファイルを飛ばす。
    -   `strictjsonRowCell` が行を数えずに最初の行を返す。番兵を取り違える。
    -   `parseUserList` が区切りの検査を緩める。重複を除いて返す。
    -   `diffUsers` が重複を報告しない。使い手でない名前を報告しない。報告の文から import するファイルを落とす。

Go のソースのコメント・識別子・文字列は英語で書き、AC や F の番号を書かない（`requirements_process.md` §4）。作った入力の中のパッケージ名は AC-12 の対象外である（要件書 AC-12）。したがって、ステップ 6-1 の AC-12 の確かめることは、作った入力のテストの中の値と、照合のテストが一覧の写しとして持つ値を区別して見る。

**完了の条件:** 作った入力のテストが通り、`make test`・`make lint`・`make deadcode` が通る。

### フェーズ 3: 照合のテストと一覧の修正（設計書 8 章の 3、3.1・3.3・3.4・4 章）

**変更するファイル:** `internal/pipeline/strictjson_users_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 3-1**: `TestPackageReferenceStrictjsonUsers` を書く。`repoRoot` を起点に、設計書 6.1 の流れ（行を見つける → `parseUserList` → `goSources` と `importersOf` → `diffUsers`）で比べ、設計書 4 章の表のとおりに `t.Errorf`（3 つの結果）と `t.Fatalf`（前提の崩れ）で報告する。テストが持つパッケージの名前は `internal/strictjson` と import のパスだけにする（AC-12）。
-   [ ] **ステップ 3-2**: 一覧を直す前に `go test -tags test -run '^TestPackageReferenceStrictjsonUsers$' ./internal/pipeline` が失敗し、4 つの名前と import するファイルを示すことを確かめる（設計書 7.2 の AC-06 の確認）。
-   [ ] **ステップ 3-3**: `package_reference.md:29` の一覧の文を、設計書 3.1 の文にする。セルのほかの部分と表のほかの行は変えない（AC-02）。
-   [ ] **ステップ 3-4**: 設計書 7.2 の壊す確認を、一覧を直した後の状態で 1 つずつ行い、失敗のメッセージが原因を示すことを確かめてから戻す。壊し方は次の 8 つである。
    -   一覧から 1 つを消す。
    -   存在しないパッケージを足す。
    -   1 つを 2 回書く。
    -   一覧の文を消す。
    -   `internal/strictjson` の行を消す。
    -   同じ行を 2 つにする。
    -   照合のテストの import のパスを存在しないパスに変える（使い手が 0 になる）。
    -   照合のテストが `goSources` に渡す起点だけを空のディレクトリに変える（ソースが 0 になる）。

    壊し方と出力の要点をコミットメッセージに書く。
-   [ ] **ステップ 3-5**: 照合のテストと一覧の修正を 1 つのコミットにする（設計書 1.2 の「PR は 1 本にする」）。

**完了の条件:** `TestPackageReferenceStrictjsonUsers` が通り、`make test`・`make lint` が通る。

### フェーズ 4: CLAUDE.md と project_overview.md（設計書 8 章の 4、3.5）

**変更するファイル:** `CLAUDE.md`、`docs/dev/project_overview.md`

-   [ ] **ステップ 4-1**: `CLAUDE.md:312-315` の項目を、設計書 3.5 の英語の文にする。改行の位置は前後の項目の行の長さに合わせてよい。ほかの記述は変えない（AC-05）。
-   [ ] **ステップ 4-2**: `project_overview.md:112` の行を、設計書 3.5 の行にする。ほかの行は変えない（AC-05）。
-   [ ] **ステップ 4-3**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`）を通してからコミットする。`TestInstructionDocPaths` が、足したリンクの行き先を確かめる。

**完了の条件:** 2 つの文書が設計書 3.5 のとおりで、文書のガードが通る。

### フェーズ 5: ADR-0020（設計書 8 章の 5、3.8）

**変更するファイル:** `docs/dev/adr/0020-strict-json.md`

-   [ ] **ステップ 5-1**: 「決定」の `0020-strict-json.md:23` の項目を、設計書 3.8 のとおりに最初の文だけにする（AC-13）。
-   [ ] **ステップ 5-2**: 「結果」の末尾に、設計書 3.8 の 1 項目を足す（AC-14）。
-   [ ] **ステップ 5-3**: 「経緯」の末尾に、設計書 3.8 の形の項目を 1 つ足す。日付は項目を書いた日にする（AC-14）。
-   [ ] **ステップ 5-4**: `TestADRRecords` を含む文書のガードを通してからコミットする。コミットメッセージには、ステップ 5-1 の削除を記録する correction line（`Editorial correction: docs/dev/adr/0020-strict-json.md: <what was changed to what>; no decision changed` の形。`requirements_process.md`「Recording in the PR description」）を書く（AC-15）。

**完了の条件:** ADR-0020 が設計書 3.8 のとおりで、`TestADRRecords` が通る。

### フェーズ 6: 一回の確認（設計書 8 章の 6、7.3）

**変更するファイル:** `docs/tasks/0068_strictjson_package_reference/_checks/main.go`（新規）、同じディレクトリのデータのファイル（新規。名前は実装で決める）、本計画 §5 の実行の記録

-   [ ] **ステップ 6-1**: 一回の確認のプログラムとデータのファイルを書く。確かめることは次の 6 つである。
    -   **一覧の範囲（AC-02）:** `package_reference.md` の差分が、`internal/strictjson` の行の一覧の文の中だけにあること。基点と head の行から一覧の文を除いた残りが同じで、ほかの行が同じであることを確かめる。
    -   **CLAUDE.md と `project_overview.md` の範囲（AC-05）:** 差分が、`CLAUDE.md` の `internal/strictjson` の項目と、`project_overview.md` の `internal/strictjson/` の行の中だけにあること。
    -   **ADR-0020 の範囲（AC-13・AC-14）:** ADR-0020 の差分が、「決定」の「この部品を使うパッケージの一覧は」で始まる項目、「結果」の末尾に足した 1 項目、「経緯」の末尾に足した 1 項目だけであること。head の「決定」の項目が基点の項目の最初の文（太字の文）だけであること。足した経緯の項目が、本タスクの文書へのリンクを持つこと。
    -   **`doc-guard` が照合のテストを実行すること（AC-11）:** head の `ci.yml` の `doc-guard` ジョブの `go test` の行を読み、次の 2 点を確かめる。(1) 行の対象のパッケージに、head で `TestPackageReferenceStrictjsonUsers` を定義するファイルのパッケージが入っていること。(2) 行が `-tags test` を渡し、そのテストを定義するファイルの build constraint がそのタグで満たされること。
    -   **照合のテストが一覧の写しを持たないこと（AC-12）:** head の `internal/pipeline/strictjson_users_test.go` が、一覧の写し、すなわち使い手の名前を並べた値を持たないことを確かめる。照合のテストが使う import のパスや、一覧の文を読むための対象の部品の名前は写しではない。正しい実装ではこの確認が通り、ステップ 6-4 の一回の確認が終了コード 0 で終わる。どの文字列を写しと見なすかの細目は、正しい実装を違反にしない範囲で実装で決める。
    -   **correction line（AC-15）:** 基点から head までのマージでないコミット（`--no-merges`）のうち ADR-0020 を変えるものの少なくとも 1 つが、`Editorial correction: docs/dev/adr/0020-strict-json.md: ` で始まり `; no decision changed` で終わり、`@` も `<` も含まない行をメッセージに持つこと（`requirements_process.md`「Recording in the PR description」）。

    次を守る。
    -   基点と head のコミットを引数に取り、省いたときは `git merge-base origin/main HEAD` と `HEAD` にする。ファイルは `git show <commit>:<path>` で読み、作業ツリーは読まない。差分のファイルとコミットの一覧は、利用者の git の設定に左右されない形（`-z`、`--no-renames`、`--no-ext-diff` など）で読む。
    -   表と節は `internal/doctestutil` の関数で読む。その関数は `test` タグでだけビルドされるため、ファイルの先頭に `//go:build test` を置く（先例は 0067 の `_checks/main.go:1`）。日本語の文字列はデータのファイルに置く（本計画 §1.3）。
    -   終了コードは、すべて通れば 0、違反があれば 1、確かめられない入力（読めないコミット、見つからない見出し・行・関数）では 2 とする。出力は、基点と head のコミット ID と、確かめることごとの結果である。
    -   ソースのコメント・識別子・文字列・出力は英語で書き、確かめることを AC や F の番号ではなく内容で名付ける。冒頭のコメントに、目的と走らせ方と、`make test` と CI が走らせないことを書く。
-   [ ] **ステップ 6-2**: ユーザーの許可を得てから `git fetch origin main` を行い、`origin/main` が `HEAD` の祖先でなければ `git merge origin/main` で取り込む。取り込みで使い手が増減していれば、一覧を直して照合のテストを通し、コミットする。取り込みで ADR-0020 の「結果」か「経緯」に項目が足されていれば、本タスクの項目がそれぞれの末尾に残るようにし、経緯の項目の日付が取り込んだ項目より前なら、`docs/dev/conventions/adr.md`「History entries」に従って日付を改める。最後に `git merge-base --is-ancestor origin/main HEAD` が成功することを確かめる。
-   [ ] **ステップ 6-3**: 確かめることのそれぞれが違反を報告できることを確かめる。使い捨てのブランチに一時的なコミットを作り、基点と head を引数で渡して走らせ、確かめ終えたらブランチを消す。一時的なコミットは PR に入れない。確かめることごとに 1 つ以上の壊し方を用意し、壊し方と、報告された確かめることと、終了コードを、本計画 §5 の実行の記録に書く。少なくとも次を含む。
    -   `package_reference.md` の `internal/strictjson` の行の一覧の文の外、および別の行を変える。
    -   `CLAUDE.md` と `project_overview.md` の、対象の項目と行の外を変える。
    -   ADR-0020 の「決定」のほかの項目を変える、「決定」の項目に 2 つ目の文を残す、「結果」に 2 項目足す、「経緯」の項目からリンクを消す、「採らなかった案」を変える。
    -   `ci.yml` の `doc-guard` の行から `./internal/pipeline` を消す。照合のテストのファイルの build constraint を、`doc-guard` が渡すタグで満たされないものに変える。
    -   照合のテスト（作った入力のテストの外）に、一覧の写しとなる使い手の名前を足す（違反）。対象の部品の名前や、テストが使う import のパスを足す（違反にならない）。
    -   ADR-0020 を変えるコミットのメッセージから correction line を消す。
    -   読めないコミットを渡す（終了コード 2）。
-   [ ] **ステップ 6-4**: `go vet -tags test ./docs/tasks/0068_strictjson_package_reference/_checks/` を通す（`make lint` は `_` で始まるディレクトリを見ない）。すべての変更をコミットした後に、PR のブランチで一回の確認のプログラムを走らせ、終了コード 0 で終わることを確かめる。出力の全文と、基点と head のコミット ID を、本計画 §5 の実行の記録に書いてコミットする。記録のコミットは本計画だけを変え、本計画はどの確かめることの対象にも入らないので、記録のコミットの後に走らせ直さなくてよい。記録には、確かめた head と PR の head の違いが本計画だけを変えたコミットであることを書く。後のコミット（`/fixpr` の回など）が、一回の確認が対象とするファイル（`package_reference.md`・`CLAUDE.md`・`project_overview.md`・ADR-0020・`ci.yml`・`strictjson_users_test.go`）を変えたときは、走らせ直して記録を改める。
-   [ ] **ステップ 6-5**: PR の説明に書く事項を控える。ADR-0020 の「決定」の削除を記録する correction paragraph（AC-15。`requirements_process.md`「Recording in the PR description」）、一回の確認のプログラムの出力、ステップ 3-4 と 6-3 の壊す確認の要点、本計画 §1.3 で設計書の付録 B の確かめ方を補ったこと（AC-09・AC-11・AC-12・AC-15）である。

**完了の条件:** 一回の確認のプログラムが PR のブランチで終了コード 0 で終わり、その記録が本計画 §5 にある。`make test && make lint` が通る。

### PR-1 作成ポイント: strictjson user list check and doc consolidation

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 / 5-3 / 5-4 / 6-1 / 6-2 / 6-3 / 6-4 / 6-5

**推奨タイトル**: `test(0068): strictjson の使い手の一覧をコードに合わせ、一覧とコードを照合するテストを加える`

**レビュー観点**: 照合のテストと作った入力のテストが、AC-06〜AC-09 の失敗の経路を通ること（ステップ 1-5・2-6・3-4 の壊す確認の記録） / `TestPackageReferenceListsPackages` の走査の載せ替えで、数える集合が変わっていないこと / `CLAUDE.md`・`project_overview.md`・ADR-0020 の文と、ADR-0020 の「決定」の削除を記録する correction paragraph / 一回の確認のプログラムの出力と、ステップ 6-3 の壊し方の記録

**実装モデル要件**: standard

**判定理由**: 競合する実装の案は本計画 §1.3 で決め、mkplan の Conditional checks（外部の状態に触れる結合テスト、環境変数による skip、CI の scripted logic など）に当たるステップはない。最も込み入ったステップ 6-1 の一回の確認のプログラムは、確かめることが 6 つある。しかし、製品にも `make test` にも入らない決定的なプログラムであり、ステップ 6-3 で確かめることごとに壊して違反が報告されることを見る。そのため isolated high-risk step には当たらないと判断した。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: 部品 | `goSources` と表のテストの載せ替え、`importersOf`・`strictjsonRowCell`・`parseUserList`・`diffUsers` と作った入力のテスト | 1〜2 |
| M2: 一覧の修正 | 照合のテスト、`package_reference.md` の一覧 | 3 |
| M3: ほかの文書 | `CLAUDE.md`・`project_overview.md`・ADR-0020 | 4〜5 |
| M4: 一回の確認 | 一回の確認のプログラムとその実行の記録 | 6 |

M2 は M1 の部品を使う。M4 は M1〜M3 のすべてのコミットの後に走らせる。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1〜6-5（すべて） | 走査の共有、照合のテストと作った入力のテスト、`package_reference.md` の一覧、`CLAUDE.md`・`project_overview.md`・ADR-0020、一回の確認のプログラム | standard |

PR を 1 本にするのは設計書 1.2 の決定である。一覧の修正と照合のテストは同じコミットに入れる（ステップ 3-5）。

## 4. テスト戦略 (Test Strategy)

### 4.1 単体テスト（作った入力のテスト）

-   `TestGoSources`・`TestImportersOf`・`TestStrictjsonRowCell`・`TestParseUserList`・`TestDiffUsers` が、走査・import の読み取り・行の発見・一覧の文の読み取り・比較と報告の文の成功と失敗の経路を確かめる（フェーズ 1〜2、設計書 7.1）。外部のプロセスもネットワークも使わない。
-   各テストは、対象を壊して失敗することを確かめてからコミットする（ステップ 1-5・2-6）。

### 4.2 統合テスト（照合のテスト）

-   `TestPackageReferenceStrictjsonUsers` が、実際の `package_reference.md` と `cmd/`・`internal/` のソースを比べる（フェーズ 3、設計書 7.2）。`make test` と CI の `doc-guard` ジョブで実行される。
-   実物を壊す確認はステップ 3-4 で行い、コミットメッセージに記録する。
-   `README.md`・`README.en.md`・`docs/dev/guides/development.md` と `extension/` を変えないので、`make ext-test` は要らない。

### 4.3 一回の確認

-   一回の確認のプログラムを PR のブランチで走らせ、記録する（フェーズ 6）。`make test` と CI は走らせない。`go list ./...` は `_` で始まるディレクトリを無視するので、`make lint`・`make deadcode` の対象にも入らない。

### 4.4 後方互換のテスト

-   N/A。製品のコードは変わらない。挙動の変わる既存のテストは `TestPackageReferenceListsPackages` だけで、今のリポジトリでは結果が変わらないことをステップ 1-4 で確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

表の「一回の確認」は、一回の確認のプログラム（本計画 §1.3 のとおりにビルドした実行ファイル）を指し、その中の確かめることの名前をかっこで添える。一回の確認を挙げた行の基点と head のコミット ID と出力は、次の実行の記録にある。実行の記録（日付、基点と head のコミット ID、出力の全文、終了コード）と、ステップ 6-3 の壊し方の記録は、ステップ 6-4 で次の段落に書く。

**実行の記録:** （ステップ 6-4 で書く）

| AC | 確かめ方 | 種類 |
|---|---|---|
| AC-01 | `internal/pipeline/strictjson_users_test.go::TestPackageReferenceStrictjsonUsers` | test |
| AC-02 | 一回の確認（一覧の範囲） | diff |
| AC-03 | 3.5 の文が使い手の名前を挙げず、一覧の置き場所を述べ、拒否するものの記述を残すことを PR のレビューで読む。リンクの行き先は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、変更が項目の中だけであることは一回の確認（CLAUDE.md と `project_overview.md` の範囲） | manual、static、diff |
| AC-04 | 3.5 の行が使い手の名前を挙げず、一覧の置き場所を述べ、部品の説明を残すことを PR のレビューで読む | manual |
| AC-05 | 一回の確認（CLAUDE.md と `project_overview.md` の範囲）。意味が変わらないことは PR のレビューで読む | diff、manual |
| AC-06 | `internal/pipeline/strictjson_users_test.go::TestDiffUsers`（一覧にない使い手と、報告の文の名前・ファイル）。ステップ 3-2・3-4 の壊す確認 | test |
| AC-07 | `internal/pipeline/strictjson_users_test.go::TestDiffUsers`（存在しない名前、`internal/strictjson` 自身）、`internal/pipeline/strictjson_users_test.go::TestImportersOf`（`_test.go` だけが import するパッケージ）。ステップ 3-4 の壊す確認 | test |
| AC-08 | `internal/pipeline/strictjson_users_test.go::TestParseUserList`（重複を残して返す）、`internal/pipeline/strictjson_users_test.go::TestDiffUsers`（重複した名前）。ステップ 3-4 の壊す確認 | test |
| AC-09 | `internal/pipeline/strictjson_users_test.go::TestStrictjsonRowCell`（表・列・行がない、行が 2 つ）、`internal/pipeline/strictjson_users_test.go::TestParseUserList`（一覧の文がない、形に合わない）、`internal/pipeline/strictjson_users_test.go::TestGoSources`（ソースがない）、`internal/pipeline/strictjson_users_test.go::TestImportersOf`（解析できないファイル）。使い手が 0 の場合とソースが 0 の場合は、ステップ 3-4 の壊す確認 | test |
| AC-10 | `internal/pipeline/strictjson_users_test.go::TestPackageReferenceStrictjsonUsers` が `make test` で通る | test |
| AC-11 | 一回の確認（`doc-guard` が照合のテストを実行すること） | static |
| AC-12 | 一回の確認（照合のテストが一覧の写しを持たないこと）。PR のレビューでもコードを読む | static、manual |
| AC-13 | 一回の確認（ADR-0020 の範囲） | diff |
| AC-14 | 一回の確認（ADR-0020 の範囲）。「結果」の 1 文が照合のテストの帰結を述べることは PR のレビューで読む。ADR の構造は `cmd/yt2column/adr_test.go::TestADRRecords` | diff、manual、test |
| AC-15 | 一回の確認（correction line）。PR の説明の correction paragraph は PR のレビューで読む | static、manual |

AC-04 は、文書の文の中身だけを定める AC である（リンクも値も持たない行で、コードブロックの中にある）。そのため、PR のレビューで読むことを確かめ方とする。変更がその行の中だけであることは、AC-05 の一回の確認が確かめる。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 実装の間に `main` で使い手が増減する | 照合のテストが PR のブランチで失敗する | ステップ 6-2 で取り込み、一覧を直す。照合のテストがずれを示すので、見落とさない |
| 走査の載せ替えで、表のテストの数える範囲が変わる | 表のテストが黙って弱くなる | ステップ 1-4 で、前後の集合が同じであることを確かめる |
| 照合のテストが、壊した入力でも通る | ずれが `make test` で見つからない | 作った入力のテスト（フェーズ 2）と、実物を壊す確認（ステップ 3-4） |
| 一回の確認のプログラムが、違反を見逃す | AC-02・AC-05・AC-11〜AC-15 の確かめが空になる | ステップ 6-3 で、確かめることごとに壊して報告されることを確かめる |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 / 5-3 / 5-4 / 6-1 / 6-2 / 6-3 / 6-4 / 6-5）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 本計画 §5 のすべての AC の確かめ方が通る。
-   **品質:** `make test`・`make lint`・`make deadcode` が通る。CI の `doc-guard` ジョブと `Test` ジョブが通る。
-   **セキュリティ:** N/A（要件書 4.2、設計書 5 章）。
-   **文書:** `package_reference.md`・`CLAUDE.md`・`project_overview.md`・ADR-0020 が設計書 3.1・3.5・3.8 のとおりで、本計画 §5 に一回の確認の実行の記録がある。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、`/runpr` がタスクの issue #713 を閉じるかを尋ねる（最後の計画の PR のため）。

## 変更履歴

- 2026-11-09 承認（isseis、#731）
