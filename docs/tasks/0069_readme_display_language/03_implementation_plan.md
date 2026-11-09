# 実装計画書：README の表示の言語の段落を、ビューアにも触れる形に直す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-09 |
| Review date | 2026-11-09 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語（ビューア・ポップアップ・表示の言語・ずれの記述）と、設計書の冒頭の用語（表示の言語の段落・実装の PR・内容のコミット・差分の確認・編集上の訂正・訂正の行・訂正の段落）を同じ意味で使う。本計画の「確認のプログラム」は、設計書 7.2 の「差分の確認のスクリプト」に当たる（本計画 §1.3）。

既存のファイルの行番号は、コミット `ccc55981fc6e8907e2b94e5bf7af046f2d622bc5`（`main`）で確かめた。設計書が行番号を確かめたコミット `960e3737` からこのコミットまでに、`README.md`・`README.en.md`・`docs/dev/adr/`・`extension/src/` を変えたコミットはない（`git log --oneline 960e3737..ccc55981 -- README.md README.en.md docs/dev/adr extension/src` の出力が空）。そのため、設計書の行番号はそのまま使える。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`README.md` の表示の言語の段落を、ポップアップとビューアの両方に触れる形に直し、`/mktrans` で `README.en.md` に写す（F-001・F-002）。同じ PR で、ADR-0029 のずれの記述を消す（F-003）。前後の関係を述べる AC は、`_checks/` の 1 回限りの確認のプログラムで確かめる。

### 1.2. 実装の原則

-   **変える箇所は設計書 1.1 の 3 か所と、このタスクのディレクトリの下だけにする。** 拡張のコード、メッセージカタログ、Go のコードとテスト、ADR の索引は変えない（設計書 2.2）。
-   **`README.md` を先に直し、`/mktrans` は sub-step として実行する**（設計書 3.2）。単独の `/mktrans` は用語集を変えて別にコミットするので実行しない。
-   **段落の文面は実装で決める。** 本計画は、段落が述べる内容を設計書 3.1 の表に委ね、文面を書かない。
-   **新しい恒久的なテストは足さない**（設計書 1.2・7.1）。

### 1.3. 計画の際に決めたこと（2026-11-09）

人間のレビュアーに諮るべき未決の判断はなかった（設計書が PR の数、コミットの分け方、差分の確認の性質まで決めている）。次の点は計画で決めたことであり、理由とともに記す。PR のレビューで異論があれば改める。どれも本タスクの中で閉じるので、ADR は作らない（`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない）。

-   **フェーズは 1 つにし、PR の作成ポイントは置かない。** 設計書 8 章は PR を 1 本と決めており、手順も一続きである。フェーズが 1 つの計画には PR の作成ポイントを置かず、`/runplan` がフェーズの完了の後に PR を作る（`.claude/commands/runplan.md:231`）。
-   **確認のプログラムは Go で書き、ビルドした実行ファイルを走らせる。** 置き場所は `docs/tasks/0069_readme_display_language/_checks/main.go` とし、先頭に `//go:build test` を置く。`go build -tags test -o <scratchpad の中のパス> ./docs/tasks/0069_readme_display_language/_checks/` でビルドし、実行ファイルを `[<base> <head>]` の引数で走らせる。引数を省くと、`origin/main` と `HEAD` のマージベースと `HEAD` を比べる。理由は次のとおりである。
    -   段落の切り出しと比較を Go で書けば、シェルと `awk` の方言（macOS の BSD 版と GNU 版）の違いに左右されない。
    -   `go run` は、プログラムの終了コードが 0 でないとき、その値によらず 1 で終わる。そのため、違反（1）と読み取りの失敗（2）を区別できない。先例のタスク 0067 の計画が確かめている（`docs/tasks/0067_stage_terminology/03_implementation_plan.md:34`）。
    -   `-tags test` を付けると、`internal/doctestutil`（`//go:build test`）の見出しと節の読み取りを再利用できる。引数の形と、git でファイルを読む関数の形は、タスク 0067 の `_checks/main.go` にならう。
-   **確認のプログラムが使う日本語の文字列は、プログラムの横のデータのファイルに置く。** 段落の位置を決める前後の段落の目印と、ADR-0029 の `## 決定` の見出しは日本語である。`_checks/` には guard-test literal exception（`CLAUDE.md`「Go Idioms」）が及ばないので、Go のソースの文字列は英語に保ち、日本語の文字列は `docs/tasks/0069_readme_display_language/_checks/data.json` に置く。先例はタスク 0067 の `_checks/data.json` である。データのファイルは、比べるコミットの中ではなく作業ツリーから読む。ステップ 1-6 で、データのファイルを含まない一時的なコミットを先頭にして実行するためである。ADR-0029 の項目の 1 文目の末尾は、文字ではなく、項目の初めの太字の範囲を閉じる `**` で決める。
-   **AC-08・AC-10・AC-11 の一部も、同じプログラムで確かめる。** 設計書は AC-08 と AC-10 を manual、AC-11 を「確かめて記録する」としている。プログラムは比べる 2 つのコミットとその間のコミットを読むので、次の 3 つを追加の手間なく機械で確かめられる。
    -   AC-08：ADR-0029 の変更後の項目が、変更前の項目を 1 文目の末尾で切ったものに等しいこと。設計書 7.2 の AC-09 の行の確認そのものなので、AC-09 の行と合わせる。
    -   AC-10：ADR-0029 を変えるコミットのメッセージに、訂正の行があること。
    -   AC-11：#727 のマージのコミットが、PR の先頭の祖先であること。

    PR の説明の訂正の段落は、PR の作成の後にしか読めないので、manual のままにする。どれも設計書の確かめ方を置き換えるのではなく、加えるものである。AC-10 と AC-11 は、変更の前後の関係を述べる AC ではないが、PR の内容とそのコミットについての確認であり、恒久的なテストにはしない。そのため、AC-04 などと同じく、PR のブランチで 1 回だけ走らせ、比べた 2 つのコミット ID とともに記録する `diff` の種類とする。

### 1.4. 既存コード調査結果

コードの変更はない。変える文書と、変更に関わる既存の仕組みは次のとおりである（行番号は `ccc55981`）。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| `README.md:574-575` | 表示の言語の段落（3 文）。前の段落は `README.md:570-572`（「拡張で設定するものはない。」で始まる）、後の段落は `README.md:577`（「送るには、」で始まる） | 設計書 3.1 の内容に改める |
| `README.en.md:786-789` と `README.en.md:1` | 対応する段落と同期の目印。後の段落は `README.en.md:791` の「To send,」で始まる | `/mktrans` の sub-step が更新する |
| `docs/dev/adr/0029-extension-ui-languages.md:24` | 「決定」の最後の項目（4 文。2〜4 文目がずれの記述）。#719 を挙げる記述は、タスクの文書の外ではここだけである | 2〜4 文目を消す |
| `cmd/yt2column/readme_sync_test.go:79` `TestReadmeSyncMarker` | 同期の目印の検査。`make readme-sync-check` と、2 つの README の変更で動く pre-commit hook（`.pre-commit-config.yaml` の `readme-sync`）も実行する | 変えない。AC-06 の検証に使う |
| 2 つの README と ADR を読むほかの検査 | 設計書 3.4 の表のとおり。`TestReadmeVersionsShareStructure`（`cmd/yt2column/readme_structure_test.go:18`）、`TestReadmeLinks`（`cmd/yt2column/docs_test.go:672`）、`metadataDocs.test.ts`、`TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:437`）、`TestADRRecords`（`cmd/yt2column/adr_test.go:434`） | 変えない。AC-12 の `make test`・`make ext-test` で通ることを確かめる |
| ADR-0029 の文面を読むテスト | ない（`cmd`・`internal` の `*_test.go` と `extension/test` の `*.ts` を `719` と `0029-extension` で探して、該当なし） | なし |
| `TestEnvAccessConfined`（`internal/config/envaccess_test.go:381`） | リポジトリ全体の Go のファイルを走査し、許した場所の外で環境変数を読むことを拒む。`_checks/` の下のファイルも走査の対象になる | 確認のプログラムは環境変数を読まない。`exec.Command` の `Env` も設定しない |
| #727 のマージのコミット | `cf76c1f622df0883230e9b1de0c426f6aef9cb88` は `ccc55981` の祖先（`git merge-base --is-ancestor` が成功） | AC-11 の確認を確認のプログラムに入れる |
| `_checks/` の先例 | `docs/tasks/0067_stage_terminology/_checks/main.go`（`//go:build test`、`[<base> <head>]` の引数、終了コード 0・1・2）と `data.json` | 同じ形にならう。`go list ./...` は `_` で始まるディレクトリを無視するので、`make lint`・`make deadcode` の対象にならない |
| CI の変更の判定 | `.github/workflows/ci.yml:36` は、`.md` で終わるファイルと `docs/` の下のファイルをコードの変更から除く | 実装の PR の変更はどれもこれに当たるので、`test` と `lint` のジョブは動かない。動くのは `doc-guard` と、README の変更で動く拡張のジョブである（設計書 3.4） |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: README の段落の直しと ADR-0029 の訂正（全 AC）

設計書 8 章の優先順位 1〜4 を、この順で行う。優先順位 5（PR の作成）は、フェーズの完了の後に `/runplan` が行う。

**変更するファイル:** `README.md`、`README.en.md`、`docs/dev/adr/0029-extension-ui-languages.md`、`docs/tasks/0069_readme_display_language/_checks/main.go`（新規）、`docs/tasks/0069_readme_display_language/_checks/data.json`（新規）、`docs/tasks/0069_readme_display_language/03_implementation_plan.md`（進み具合と §7 の記録）

-   [x] **ステップ 1-1（内容のコミット 1）**: `README.md:574-575` の段落を、設計書 3.1 の表の 3 文の内容に改める。1 文目の主語は画面の文言だけを指す語にし、記事の本文が選んだ言語で表示されるとは読めない形にする（AC-01）。段落は 1 つのまま、場所も変えない。段落の外は変えない（AC-04）。
-   [x] **ステップ 1-2（内容のコミット 1）**: `make readme-sync-check` を実行し、失敗したら `.claude/commands/mktrans.md`「When a caller runs it」のとおり `/mktrans README.md` を sub-step として実行する。変更後の `README.en.md` の段落の 3 文目が「英語の版」を述べていること（設計書 3.2）と、段落の外の行が変わっていないことを読んで確かめる。どちらか一方でも満たさなければ、`README.en.md` を `/mktrans` の前の状態に戻してやり直す（設計書 4.2）。`/mktrans` が止まったときは、`README.md` だけをコミットせずに作業を止め、利用者に報告する。用語集にない語として報告された語を書き留める（PR の説明に載せる）。
-   [x] **ステップ 1-3（内容のコミット 1）**: `make readme-sync-check` と `make ext-test` が通ることを確かめ、`README.md` と `README.en.md` を 1 つのコミットに入れる。
-   [x] **ステップ 1-4（内容のコミット 2）**: `docs/dev/adr/0029-extension-ui-languages.md:24` の項目から、ずれの記述の 3 文を消す。変更の前後は次のとおりで、項目のほかの部分と ADR のほかの行は変えない（AC-08・AC-09）。
    -   変更前：`-   **表示の言語の決まり方を利用者に説明するのは [README.md](../../../README.md) の「ブラウザ拡張」節の段落である。** この段落は、ポップアップの言語だけを述べている。現在のコードでは、ビューアも同じ規則で選んだ言語で表示する。段落がビューアに触れていないことは、記録した issue [#719](https://github.com/isseis/yt2column/issues/719) で扱う。`
    -   変更後：`-   **表示の言語の決まり方を利用者に説明するのは [README.md](../../../README.md) の「ブラウザ拡張」節の段落である。**`

    このファイルだけをコミットする。コミットのメッセージには、設計書 3.3 の形の訂正の行を 1 行だけ書く（AC-10）。行は `Editorial correction: docs/dev/adr/0029-extension-ui-languages.md: ` で始まり、何を何に直したかを自分の言葉で述べ、`; no decision changed` で終わる。`@` と `<` を含めない。経緯の項目は足さない。PR は squash でマージされ、`main` に入るコミットのメッセージは `/mergepr` が書くので、AC-10 が求める記録は、マージの前の PR のコミットと PR の説明にあるものである。
-   [x] **ステップ 1-5（コミット 3）**: 確認のプログラムを `_checks/main.go` と `_checks/data.json` に書く（本計画 §1.3 の形）。確かめる項目は次のとおりである。
    -   AC-04・AC-07・AC-09（AC-08 も兼ねる）・変えたファイルの範囲：設計書 7.2 の表のとおり。
    -   AC-10：比べる範囲のコミットのうちマージでないもの（`--no-merges`）について、次の 3 つを確かめる。(a) ADR-0029 を変えるコミットが 1 つ以上ある。(b) そのどれもが ADR-0029 以外のファイルを変えない。(c) そのどれもが、メッセージに ADR-0029 の訂正の行（`Editorial correction: docs/dev/adr/0029-extension-ui-languages.md: ` で始まり `; no decision changed` で終わり、`@` と `<` を含まない行）をちょうど 1 つ持つ。マージのコミットを除くのは、`main` を取り込むと、そのマージのコミットが `docs/dev/adr/` の下のほかのタスクの変更を含むためである。
    -   AC-11：`cf76c1f622df0883230e9b1de0c426f6aef9cb88` が PR の先頭の祖先である。

    設計書 7.2 の 3 つの性質（2 つのコミットを引数で受け取って作業ツリーの文書を読まない、段落の位置を前後の変わらない段落で決める、違反でも読み取りの失敗でも 0 でない終了コードで終わる）を持たせる。基点が先頭の祖先でなければ、読み取りの失敗として終わる。違反は終了コード 1、読み取りの失敗は 2 で終わる。出力には、比べた 2 つのコミットの完全な ID と、項目ごとの結果を含める。コメント・識別子・文字列は英語で書く。`make fmt` と `go vet -tags test ./docs/tasks/0069_readme_display_language/_checks/` を通す。
-   [x] **ステップ 1-6（コミット 3）**: 確認のプログラムが違反と読み取りの失敗を検出できることを確かめる。PR のブランチから `git worktree` で使い捨てのブランチを作り、壊し方を 1 件ずつ別の一時的なコミットとして作る。それぞれを先頭として、ビルドした実行ファイルを PR のブランチの作業ツリーから走らせ、壊した項目だけが失敗し、ほかの項目は通ることと、終了コード（違反は 1、読み取りの失敗は 2）を確かめる。
    -   **作り方:** 一時的なコミットは `git commit --no-verify` で作る。`README.md` を変えると、pre-commit hook の `readme-sync` が同期の目印のずれを理由にコミットを拒むためである。`--no-verify` は使い捨ての worktree の中だけで使う。ADR-0029 に関する壊し方は、使い捨てのブランチで、内容のコミット 1 の上に、内容を変えた内容のコミット 2 を作り直して作る。こうすると、AC-10 の項目が巻き添えで失敗しない。変える行は、目印にする段落の外から選ぶ。
    -   **壊す対象:** 次の失敗の経路を、それぞれ 1 件以上の壊し方で確かめる。
        -   AC-04：`README.md` の表示の言語の段落の外の 1 行の変更。段落の中に空行を入れて 2 つの段落にすること。
        -   AC-07：`README.en.md` の段落の外の 1 行の変更。段落を 2 つの段落にすること。
        -   AC-09（AC-08 も兼ねる）：ADR-0029 の、「決定」の最後の項目の外の 1 行の変更。項目の 1 文目の変更。ADR-0029 以外の ADR の変更。`docs/dev/adr/` の下のファイルの追加。
        -   範囲：`README.md`・`README.en.md`・ADR-0029・このタスクのディレクトリの外のファイルの変更。
        -   AC-10：訂正の行がないこと。行の終わりの `; no decision changed` がないこと。行が 2 行あること。ADR-0029 を変えるコミットが `README.md` も変えること。
        -   AC-11：`cf76c1f6` を含まない先頭。`cf76c1f6` の親から使い捨てのブランチを作り、内容のコミット 1 と 2 をこの順に cherry-pick する（`cf76c1f6` の前でも、ADR-0029 の項目は同じ太字の 1 文目で始まる）。基点に `cf76c1f6` の親を、先頭にこの使い捨てのブランチの先頭を渡す。
        -   読み取りの失敗：目印にする段落が `README.md` にないこと、2 回現れること。`README.en.md` の目印にする段落がないこと。ADR-0029 の `## 決定` の見出しがないこと。基点と先頭を入れ替えて渡すこと（基点が先頭の祖先でない）。

        壊し方ごとに、失敗した項目と通った項目を確かめ、狙った項目のほかが失敗するときは壊し方を直す。読み取りの失敗の壊し方では、終了コードが 2 であることを確かめ、ほかの項目の結果は問わない。ADR-0029 の壊し方で作り直す内容のコミット 2 には、正しい訂正の行を残す。

    使い捨てのブランチは push せず、確かめた後に worktree とともに消す。PR のブランチの履歴は変えない。壊し方ごとの結果を、プログラムを足すコミットのメッセージに書いて、`_checks/main.go` と `_checks/data.json` をコミットする。
-   [x] **ステップ 1-7（コミット 4）**: `make test`・`make ext-test`・`make lint` を実行して通ることを確かめる（AC-12）。`git fetch origin main` の後、確認のプログラムをビルドして引数なしで走らせる。出力、終了コード、比べた 2 つのコミットの完全な ID を本計画 §7 の該当する行に書き、本計画の進み具合とともにコミットする。このコミットはこのタスクのディレクトリの下しか変えないので、記録した結果は変わらない（設計書 6.1）。
-   [x] **ステップ 1-8**: 記録したコミットの後に、`README.md`・`README.en.md`・`docs/dev/adr/`・`_checks/` を変えるコミットや、`main` を取り込むコミットが入ったら、確認のプログラムを走らせ直して §7 の記録を改める（設計書 7.2 の最後の段落）。`/runplan` のフェーズの後のレビュー（deadcode の確認、サブエージェントのレビュー、ローカルの CLI のレビュー）の修正のコミットも、これに当たる。PR を作る（`_lib/task-pr-handoff.md` Part B）前に、記録が PR の先頭のものであることを確かめる。

**完了の条件:** 内容のコミット 1・2、確認のプログラムのコミット 3、記録のコミット 4 がこの順にある。確認のプログラムが終了コード 0 で終わり、その記録が §7 にある。`make test`・`make ext-test`・`make lint`・`make readme-sync-check` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: README の直し | `README.md` と `README.en.md` の内容のコミット 1 | 1-1〜1-3 |
| M2: ADR-0029 の訂正 | 訂正の行つきの内容のコミット 2 | 1-4 |
| M3: 差分の確認 | `_checks/main.go`・`_checks/data.json` と、壊して確かめた記録（コミット 3）、§7 の実行の記録（コミット 4） | 1-5〜1-8 |

各マイルストーンは前のマイルストーンを前提とする。M3 の後、`/runplan` が PR を作る。

### 3.2. PR 構成

PR は 1 本である（本計画 §1.3）。フェーズが 1 つなので PR の作成ポイントはなく、`/runplan` がフェーズの完了の後に PR を作り、§6 に PR の記録を足す。CI で動くジョブは本計画 §1.4 の「CI の変更の判定」の行のとおりである。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

-   新しい恒久的なテストは足さない（設計書 7.1）。既存のテストが通ることを `make test` と `make ext-test` で確かめる（AC-12）。どの検査が影響を受けうるかは設計書 3.4 の表のとおりである。
-   同期の目印は `make readme-sync-check` で確かめる（AC-06）。pre-commit hook も同じテストを実行する。

### 4.2. 差分の確認

-   AC-04・AC-07・AC-09（AC-08 も兼ねる）、変えたファイルの範囲、AC-10 のコミットのメッセージ、AC-11 を、確認のプログラムで 1 回確かめる（ステップ 1-5・1-7・1-8）。
-   確認のプログラムが違反と読み取りの失敗を検出できることは、ステップ 1-6 で壊し方ごとに確かめる。

### 4.3. 読んで確かめる

-   AC-01〜AC-03・AC-05・AC-08、AC-10 の訂正の行の内容と PR の説明の訂正の段落は、PR のレビューで読む（設計書 7.3）。

### 4.4. 結合テスト・後方互換のテスト

N/A。実行時の振る舞いを変えない（設計書 7.4）。

## 5. リスク管理 (Risk Management)

### 5.1. 技術のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| `/mktrans` が段落の外の行を変える、または 3 文目を日本語版の直訳（「日本語の版」）にする | AC-05・AC-07 を満たさない | ステップ 1-2 で読んで確かめ、`README.en.md` を戻してやり直す。確認のプログラムも段落の外の変更を検出する。やり直しても直らなければ止まって報告する（設計書 4.2） |
| `/mktrans` が止まる | `README.md` だけのコミットになる | `README.md` だけをコミットしない。pre-commit hook と CI の `doc-guard` も、目印のずれた README を拒む（設計書 4.2） |
| 段落の 1 文目の主語が「ビューア」になり、記事の本文も英語になると読める | AC-01 を満たさない | ステップ 1-1 で主語を画面の文言に限る。PR のレビューで読んで確かめる |
| 目印にする前後の段落が、`main` の取り込みで変わるか、2 回以上現れる | 確認のプログラムが段落の位置を決められない | 確認のプログラムは読み取りの失敗として終わる。そのときは `data.json` の目印を選び直し、ステップ 1-6 をやり直す |
| `/fixpr` が PR の作成の後に ADR-0029 を直す | ADR-0029 を変えるコミットが増える | そのコミットにも ADR-0029 の訂正の行を書く。確認のプログラムは、ADR-0029 を変えるマージでないコミットのそれぞれに訂正の行を求めるので、走らせ直して記録を改める（ステップ 1-8） |

### 5.2. 日程のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| README を変える別のタスク（タスク 0066 の残りなど）が先にマージされ、`main` を取り込む | 同期の目印が衝突し、`/mktrans` の再実行と確認のプログラムの記録の取り直しの手間が増える | `/mktrans` をやり直し（`.claude/commands/mktrans.md`「Taking in `main`」）、AC-05 の段落を読み直し、ステップ 1-8 で記録を改める（設計書 7.2）。変更は 1 段落なので、やり直しは同じセッションの中で済む |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] フェーズ 1 完了（ステップ 1-1〜1-8）
- [x] グリーンゲート（`_context.md` の "Green gate" 参照）と `make ext-test` が通ることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/737
- [x] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確認のプログラムは `docs/tasks/0069_readme_display_language/_checks/`（`main.go` と `data.json`。以下、`_checks`）である。実行の記録（日付、基点と PR の先頭の完全なコミット ID、出力、終了コード）は、ステップ 1-7・1-8 で各行に書く。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | manual | 変更後の `README.md` の段落を PR のレビューで読む。記事の本文まで UI の言語で表示されると読めないこと（設計書 3.1・7.3） | 1-1 |
| AC-02 | manual | 同上 | 1-1 |
| AC-03 | manual | 同上 | 1-1 |
| AC-04 | diff | `_checks`（`README.md` の表示の言語の段落の外が前後で同じで、変更後の段落が 1 つの段落のまま。変えたファイルが `README.md`・`README.en.md`・ADR-0029・このタスクのディレクトリの下だけであることも確かめる）。実行の記録（2026-11-09、基点 `ed64cf39549c153e5d8143e32b6b92bf28328505`、先頭 `419269d159b76fcaede1bc4cefb728f085af03ce`、終了コード 0）: `OK   readme: README.md: text outside the display-language paragraph is unchanged, the paragraph is one block, and every changed file is in scope` | 1-5〜1-8 |
| AC-05 | manual | 変更後の `README.en.md` の段落を PR のレビューで読む。3 文目が「英語の版」を述べること（設計書 3.2・7.3） | 1-2 |
| AC-06 | test | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`（`make readme-sync-check`、`make test`、CI の `doc-guard`） | 1-2・1-3・1-7 |
| AC-07 | diff | `_checks`（`README.en.md` の 1 行目と表示の言語の段落の外が前後で同じで、変更後の段落が 1 つの段落のまま）。実行の記録（2026-11-09、基点 `ed64cf39549c153e5d8143e32b6b92bf28328505`、先頭 `419269d159b76fcaede1bc4cefb728f085af03ce`、終了コード 0）: `OK   readme-en: README.en.md: only the sync marker and the display-language paragraph changed, and the paragraph is one block` | 1-5〜1-8 |
| AC-08 | manual | 変更後の ADR-0029 の「決定」の最後の項目を PR のレビューで読む（設計書 7.3）。項目の形は AC-09 の行の `_checks` でも確かめる | 1-4 |
| AC-09 | diff | `_checks`（`docs/dev/adr/` の下で変わったのは ADR-0029 だけで、追加・削除・名前の変更がなく、ADR-0029 の変更後の項目は変更前の項目を 1 文目の末尾で切ったものに等しく、ほかの部分は前後で同じ。AC-08 も兼ねる）。実行の記録（2026-11-09、基点 `ed64cf39549c153e5d8143e32b6b92bf28328505`、先頭 `419269d159b76fcaede1bc4cefb728f085af03ce`、終了コード 0）: `OK   adr: docs/dev/adr/0029-extension-ui-languages.md: the decision section's last item is its first sentence and nothing else in the ADR changed` | 1-5〜1-8 |
| AC-10 | diff | `_checks`（ADR-0029 を変えるマージでないコミットが 1 つ以上あり、どれも ADR-0029 だけを変え、メッセージに ADR-0029 の訂正の行をちょうど 1 行持つ）。実行の記録（2026-11-09、基点 `ed64cf39549c153e5d8143e32b6b92bf28328505`、先頭 `419269d159b76fcaede1bc4cefb728f085af03ce`、終了コード 0）: `OK   corrections: each non-merge commit that changes ADR-0029 carries exactly one editorial correction line` | 1-4〜1-8 |
| AC-10 | manual | 訂正の行の内容と、PR の説明の訂正の段落を PR のレビューで読む（設計書 7.3） | 1-4、PR の作成 |
| AC-11 | diff | `_checks`（`cf76c1f622df0883230e9b1de0c426f6aef9cb88` が PR の先頭の祖先）。実行の記録（2026-11-09、基点 `ed64cf39549c153e5d8143e32b6b92bf28328505`、先頭 `419269d159b76fcaede1bc4cefb728f085af03ce`、終了コード 0）: `OK   merge-order: cf76c1f622df0883230e9b1de0c426f6aef9cb88 is an ancestor of the head commit` | 1-5〜1-8 |
| AC-12 | test | `make test`、`make ext-test` | 1-3・1-7 |

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** `README.md` と `README.en.md` の段落がポップアップとビューアの両方に触れ、ADR-0029 のずれの記述がない。§7 のすべての AC の検証が済んでいる。
-   **品質:** `make test`・`make ext-test`・`make lint`・`make readme-sync-check` が通る。`_checks` が終了コード 0 で終わり、壊し方ごとに失敗することを確かめた記録がコミットのメッセージにある。
-   **セキュリティ:** N/A（設計書 5 章）。
-   **文書:** 変更は設計書 1.1 の 3 か所とこのタスクのディレクトリの下だけにある。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0069` でフェーズ 1 を行い、PR を作る。
-   実装の PR のマージの後、issue #719 を閉じる（`/runpr` が閉じる。`Closes #N` では自動で閉じない）。

## 変更履歴

- 2026-11-09 承認（isseis、#734）
