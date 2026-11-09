# 実装計画書：ADR の編集上の訂正を、コマンドの定義と requirements_process.md の訂正の記録に対応させる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 3 章の冒頭と 6 章の用語と、設計書の冒頭の用語（実装の PR・訂正の一覧の確かめ・タスク文書の項目・ADR の項目・ADR の候補の一覧・候補のスクリプト・差分の確認）を、同じ意味で使う。本計画の「確認のプログラム」は、設計書 7.2 の差分の確認のスクリプトに当たる（本計画 §1.3）。比べる 2 つのコミットのうち、変更前のもの（マージベース）を「基点」、変更後のもの（PR のブランチの最新のコミット）を「先頭」と呼ぶ。

既存のファイルの行番号は、コミット `9441515482654f847d43e93962d4437ad255c91e`（`main`）で確かめた。設計書が行番号を確かめたコミット `9475c02b` からこのコミットまでに、本タスクが変えるファイルと読むテストは変わっていない（`git diff --stat 9475c02b 9441515 -- .claude/commands docs/dev/conventions internal/fixprguard cmd/yt2column docs/dev/adr/0039-adr-writing-criteria.md` の出力が空）。そのため、設計書が示す行番号はそのまま使える。その間に変わった ADR は 0012・0035・0040 と索引で、ADR-0012 はタスク 0075 の要件の PR（#753）で `廃止` になった（本計画 §1.3）。

申し送りの H-01〜H-05 は、設計書の付録 A がすべて扱っている。本タスクに `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

訂正の行と訂正の段落の定義を、ADR にも当てはまるように広げ（F-001）、規約の「After the PR is created」の制限を、訂正の行を運ぶ手順のある経路でだけ認める形に改める（F-005）。`/fixpr`・ローカルの CLI レビュー・`/runplan` が ADR の訂正の行を書くようにする（F-002〜F-004）。`/fixpr` は、エージェントが返した ADR の項目を、`origin/main` の版の候補のスクリプトで確かめる。同じ PR で ADR-0039 のずれの記述を、編集上の訂正として消す（F-007）。変えないことを求める AC は、`_checks/` の 1 回限りの確認のプログラムで確かめる。

### 1.2. 実装の原則

-   **文面は、設計書 3 章の英語の案を使う。** 言い回しは、案の意味を変えない範囲で整えてよい。意味を変えるときは、実装を止めて設計書を改める（設計書 3 章の冒頭）。
-   **変えない部分は、1 バイトも違わないようにする。** 設計書 3.1 の「変えないもの」、`requirements_process.md`「Editing an approved document」、設計書 2.2 の「変えないファイル」がこれに当たる。
-   **候補のスクリプトは、設計書 3.2.2 の表の性質をすべて持つ。** 各性質は、本計画 §2 のステップ 2-3 のテストの場合で確かめる。
-   **`editorial` の項目の `path` を、コマンドの引数にもコマンドの文字列にもしない**（設計書 1.2・5.1）。
-   **ADR-0039 の変更は、編集上の訂正として、PR を作る前のコミットに訂正の行とともに入れる**（設計書 3.9）。

### 1.3. 計画の際に決めたこと（2026-11-09）

人間のレビュアーと次の 2 点を決めた。

-   **候補のスクリプトを bash で書くことは、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の対象外とし、ADR-0014 を変えない。**
    -   **背景:** 設計書 1.2 は、bash スクリプトにする形を [ADR-0012](../../dev/adr/0012-lightweight-check-in-bash.md) と同じと述べる。しかし、ADR-0012 は設計書が確かめたコミットの後に `廃止` になった。また、設計書は ADR-0014（決定的な判定は、テストのある Go の開発ツールに置く）に触れていない。
    -   **理由:** ADR-0014 の「決定」の 1 つ目の項目は、対象を「判定と副作用を持つ開発の道具」に限っている。候補のスクリプトは git を読んで一覧を出力するだけで、副作用を持たない。ADR-0014 の狙いである次の 2 点も満たす。
        -   判定をエージェントに任せない（比較はスクリプトが行う。設計書 1.2）。
        -   判定の誤りをテストで壊して確かめられる（ステップ 2-3）。
    -   **採らなかった案:** ADR-0014 に例外を足す案は、設計書が行った ADR についての判断を変えることになり、設計書を `draft` に戻す必要がある。Go のツールにする案は、設計のやり直しと、インストールの手順の追加が要る。どちらも、得るものに対して重いので採らない。
    -   ADR：なし。ADR-0014 の決定の範囲を当てはめた判断で、その決定を変えない。
-   **テストは `lightweight_docs_test.go` に依存させない。bash を探す処理だけを共有のヘルパーのファイルに移す。**
    -   **背景:** 設計書 7.1 は、テストの土台に `lightweight_docs_test.go` の仕組みを再利用するとし、共通にする範囲を計画に委ねた。ところが、タスク 0075（要件は承認済み）はこのファイルを削除する（0075 要件書の AC-01）。
    -   **決めたこと:** bash を探す型と関数（`lwBash`・`lightweightBashes`、`cmd/yt2column/lightweight_docs_test.go:536-562`）を、新しい `cmd/yt2column/test_helpers_bash.go`（`//go:build test`）に移し、名前を本計画 §1.5 の表のとおり変える。コードブロックの行とコードスパンを取り出す `codeTexts`（`lightweight_docs_test.go:329-347`）も、`codeSpanRE` の横の `cmd/yt2column/docs_test.go` に移す。`codeTexts` が使うのは `doctestutil` と `codeSpanRE`（`docs_test.go:418`）だけである。git の土台（リポジトリ・`HOME`・環境の許可リスト・決まった作者での git の実行）は、新しいテストのファイルが自前で持つ。`lwFixture` の同じ処理（`:711-748`）は `lwFixture` のフィールドに結びついたメソッドであり、切り出すには 0075 が消すテストを作り直す必要があるからである。この重複は、0075 が `lightweight_docs_test.go` を消すと解消する。`newLWFixture` は軽量プロセスの条件の表を読むので再利用しない。
    -   **理由:** 0075 が `lightweight_docs_test.go` を消しても、本タスクのテストは壊れない。bash を探す処理は 1 か所に保たれる（`CLAUDE.md`「DRY」）。
    -   **採らなかった案:** 本タスクのテストに bash を探す処理を写す案は、0075 が元を消すまで同じ処理が 2 つになるので採らない。`lwFixture` をそのまま使う案は、0075 がそれを先に切り出す必要が生じるので採らない。
    -   ADR：なし。テストのファイルの分け方の決定で、本タスクの中で閉じる。

次の点は、計画で決めたことである。理由とともに記す。PR のレビューで異論があれば改める。どれも本タスクの中で閉じるので、ADR は作らない（`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない）。

-   **フェーズは設計書 8 章の優先順位に沿って 4 つに分け、PR の作成ポイントは最後に 1 つだけ置く。** 設計書 1.2 は実装の PR を 1 本と決めている。フェーズは、定義・候補のスクリプト・コマンド・ADR-0039 と差分の確認の 4 つである（本計画 §3.1）。
-   **候補のスクリプトの名前は、設計書のとおり `.claude/commands/_lib/adr-correction-candidates.sh` とする。**
-   **候補のスクリプトの比較の出力は、1 行目を `origin-main <コミットの完全な ID>` とし、2 行目からは引数ごとに 1 行の `match <一覧の値>` か `no-match` とする**（設計書 3.2.2 の表の「比較」の行が、出力の形を計画に委ねた）。引数がないときは、一覧を 1 行に 1 つ出力する。
    -   **1 行目:** 判定に使った `refs/remotes/origin/main` のコミットである。落とした項目の決まった理由（設計書 3.2.2）に添える値を、別のコマンドを足さずに得るためである。
    -   **`/fixpr` のセッションの読み方:** 引数の順に行を読み、`match ` で始まる行の残りを一覧の値として項目に添える。`no-match` と、それ以外の形の行の項目は落とす。次のときは、ADR の項目をすべて落とす。
        -   終了コードが 0 でない（`match` の行があっても）。
        -   1 行目の形が違う。
        -   行の数が、引数の数に 1 を足した数と違う。
    -   **この形にする理由:** 空の行で一致なしを表すと、出力の行の数の誤りと見分けにくい。
-   **比較では、ファイルの内容をコマンド置換で読まない。** コマンド置換は末尾の改行を落とすので、末尾に改行を足した `path` が一致してしまう。一覧の値と内容は、`cmp` のようにバイトで比べる道具で比べる。`cmp` は macOS（`/usr/bin/cmp`）と CI の Linux（diffutils）にある。`printf %s <値> | cmp -s - <末尾に改行のあるファイル>` が 1 で終わることは、計画を書くときに macOS で確かめた。
-   **変更の判定と `main` の判定に使う git の操作は、計画を書くときに確かめた形を前提にする。** 確かめたのは、macOS の git 2.50.1（Apple Git-155）と `/bin/bash` 3.2.57 である。一時的なリポジトリで、ADR の 4 つのファイルを、内容の変更・シンボリックリンクへの置き換え・`git mv` による名前の変更・削除の 4 通りに変えた。そのうえで、次の点を見た。
    -   `git diff --no-renames -z --name-only --diff-filter=M HEAD` が出力したのは、内容を変えたファイルだけだった。`--name-status` では、シンボリックリンクへの置き換えは `T`、名前の変更は `D` と `A` になった。
    -   モードだけを変えたファイル（`chmod +x`）も、`--diff-filter=M` に出力された。設計書 3.2.2 の「内容が変わった」より範囲が広いが、出るのは `main` にもある、変更された ADR のファイルだけなので、確かめの結果に誤りは生じない。この差は受け入れる。
    -   名前の変更は、検出の有無にかかわらず `M` にならない（レビュアーが別のリポジトリで確かめた）。そのため、名前の変更の検出を切ることは、テストで観測できない。`--no-renames` は守りとして残し、その理由をスクリプトのコメントに書く。設計書 7.1 の名前の変更の場合は、変更の判定の場合として残す。
    -   `main` の判定は、`git ls-tree -z --name-only refs/remotes/origin/main -- docs/dev/adr/` を 1 回実行し、変更の一覧と突き合わせて行う。この形は、木にないパスでも 0 で終わり、木を読めないとき（参照がブロブを指すとき）は 128 で終わった。そのため、0 でない終了コードは失敗だけを表す。`git cat-file -e` はパスがないときも失敗のときも 128 で終わるので、この判定には使わない。
    -   `/bin/bash` 3.2 で `set -o pipefail` が効いた。
-   **候補のスクリプトは、呼び出し元の環境を持ち込まない。** `lightweight-check.sh` と同じく、最初に `PATH`・`HOME`・`TMPDIR` だけを残した `env -i` で自分を起動し直す（`.claude/commands/_lib/lightweight-check.sh` の冒頭の契約）。これで、起動し直した後のシェルと git には、`BASH_ENV`・書き出されたシェルの関数・`SHELLOPTS`・`GIT_` で始まる変数が届かない。git の全体と system の設定も読ませない。ただし、bash は起動するときに、スクリプトより先に `BASH_ENV` のファイルを読む。そのファイルが定めた関数は起動し直すと消えるが、そのファイルが標準出力に書いたものは出力に混ざる。`BASH_ENV` は利用者自身の環境で、信頼の境界の外から来る値ではない。また、出力が崩れると、`/fixpr` のセッションは ADR の項目をすべて落とす（上の出力の読み方）。そのため、呼び出しの行で環境を空にする手順は足さず、この残るリスクを受け入れる。
-   **テストは、作業ツリーにある候補のスクリプトのファイルを、土台のリポジトリの中で直接実行する。** 確かめるのはスクリプトの振る舞いで、`origin/main` からの取り出しは呼び出しのテスト（ステップ 3-2）が確かめる。
-   **確認のプログラムは Go で書き、`//go:build test` を付けて、ビルドした実行ファイルを走らせる。** 置き場所は `docs/tasks/0074_adr_editorial_correction/_checks/main.go` である。ADR-0039 から消す 4 文（ステップ 4-1）などの日本語の文字列は、同じディレクトリの `data.json` に置く。引数の形（`[<base> <head>]`、省くとマージベースと `HEAD`）、終了コード（違反は 1、読み取りの失敗は 2）、データのファイルを作業ツリーから読むことは、タスク 0070 の `_checks/` にならう（`docs/tasks/0070_adr_writing_criteria/_checks/main.go`、同計画 §1.3）。理由は次の 3 つである。
    -   `go run` では、違反と読み取りの失敗の終了コードを区別できない。
    -   `-tags test` を付けると、`internal/doctestutil` の節の読み取りを再利用できる。
    -   `_checks/` には、日本語の文字列を許す guard-test literal exception（`CLAUDE.md`「Go Idioms」）が及ばないので、Go のソースの文字列は英語に保つ。
-   **確認のプログラムは、ADR-0039 を変えるコミットの訂正の行も確かめる**（AC-20 の記録の形）。比べる範囲にあるマージコミット以外のコミットのうち、ADR-0039 を変える最初のコミットについて、次の 3 つを確かめる。0070 の計画 §1.3 と同じ確かめ方である。
    -   そのコミットがある。
    -   ADR-0039 のほかのファイルを変えない。
    -   メッセージに、第 1 カラムから `Editorial correction: docs/dev/adr/0039-adr-writing-criteria.md: ` で始まり、`; no decision changed` で終わり、`@` と `<` を含まない行が 1 行以上ある。

### 1.4. 既存コード調査結果

行番号は `9441515` での値である。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| `docs/dev/conventions/requirements_process.md`「Recording in the PR description」（`:377-409`） | 訂正の行の箇条 `:379-391`。`<document path>` を定める文「`<document path>` is the repository-relative path of the task document.」は `:388` の行頭から始まり、同じ行の「The」から次の文が始まる。訂正の段落の箇条 `:392-409`。「Each item is a correction line put into Japanese, one item for each correction line.」は `:403-404` にまたがり、`:404` の途中の「When」から次の文が始まる | `:388` の文を設計書 3.1 の案に置き換え、`:403-404` の文の後に設計書 3.1 の 1 文を足す。どちらも、同じ行の前後の文と改行の位置を保つ（ステップ 1-1） |
| 同「Editing an approved document」（`:23-69`） | タスク文書の訂正の規則 | 変えない（AC-18） |
| 規約 `docs/dev/conventions/adr.md`「Editorial corrections」 | 「Recording」の段落 `:298-306`（読み替えの句は `:302`）。「After the PR is created」の段落 `:308-314`。「Discrepancy statements」の 3 つ目の箇条の最後の文 `:348-350`（`:348` の途中の「A statement」から始まる） | 設計書 3.3.1〜3.3.3 のとおり改める（ステップ 1-2） |
| `.claude/commands/fixpr.md` Phase 3 | `editorial` を求める文 `:484-490`。「Migration check」 `:503-529`。「Correction list」 `:531-548`。項目が通る条件の箇条 `:536-546` | 設計書 3.2.1・3.2.2 のとおり改め、候補のスクリプトの取り出しと実行のコードブロックを足す（ステップ 3-1） |
| 同 Phase 4・Phase 6・Final report | Phase 4 の訂正の行 `:620-668`。Phase 6 の追記 `:837-860`。Final report の 9 項目め `:951-962` | Phase 4 は ADR の項目の行のパスに一覧の値を使うと足す。Final report の 9 項目めに、追記した訂正の列挙を足す。Phase 6 は変えない（ステップ 3-1） |
| `.claude/commands/_lib/local-cli-review.md` | step 5 の訂正の行の文 `:205-209`。step 7 の表の `Editorial correction:` の行 `:234`。メンションの確かめ `:243-254` | 設計書 3.4 のとおり、`:205-209` と `:234` を ADR の訂正に広げる。メンションの確かめは変えない（ステップ 3-2） |
| `.claude/commands/runplan.md:63` | 自己点検の「Editorial correction」の箇条（タスク文書だけ） | 設計書 3.5 の箇条を後に足す（ステップ 3-3） |
| `internal/fixprguard/write.go:97-98` | `editorialCorrectionPrefix` のコメントが「an editorial correction to an approved task document」と述べる | コメントだけを直す（ステップ 3-4） |
| `docs/dev/adr/0039-adr-writing-criteria.md:20` | 「決定」の 4 つ目の項目の最後の 4 文（「規約は今も」から「…issues/732 で扱う。」まで）。`git grep -nE 'issues/732([^0-9]|$)' origin/main -- docs/dev/adr/` の当たりは、この行だけ（`9441515` で実行） | 4 文を消す（ステップ 4-1） |
| `cmd/yt2column/lightweight_docs_test.go` | `lwBash`（`:536-539`）・`lightweightBashes`（`:541-562`）。使う箇所は同じファイルの中だけ（`:929`・`:1007`・`:1231`・`:1238`・`:1260`・`:1405`・`:1410`・`:1716`）。git の土台 `lwFixture`（`:570-`）は、軽量プロセスの条件の表を読む | `lwBash`・`lightweightBashes` を `test_helpers_bash.go` に移して名前を変える（ステップ 2-1）。ほかは変えない |
| `TestLightweightCheckCallSite`（`lightweight_docs_test.go:366-396`） | `doctestutil.CommandDocs` の各文書の、コードブロックの行とコードスパンを `codeTexts`（`:329-347`、呼び出しはこのテストだけ）で取り出す。スクリプトの名前か `<check-script>` を含むものが、取り出しの行（`origin/main` の版をファイルに書き出す行）・実行の行（書き出したファイルを `bash` で実行する行）・名前だけのどれかであることを確かめる | 新しい呼び出しのテストの手本にする。`codeTexts` を `docs_test.go` に移して共有する（本計画 §1.3） |
| `internal/doctestutil` | `CommandDocs`（`protocol.go:85`）、`SectionByHeading`（`markdown.go:90`）、`FencedBlocks`（`markdown.go:165`）、`ScanDocLines`（`markdown.go:130`）、`ReadDoc`（`read.go:13`） | 呼び出しのテストで再利用する |
| `TestEnvAccessConfined`（`internal/config/envaccess_test.go`） | `_test.go` と、`test`・`integration` のタグがないと成り立たないファイルは、テストのコードとして扱う（`envaccess_test.go:289-305`） | `test_helpers_bash.go` には `//go:build test` を付ける。`_checks/main.go` は環境変数を読まない |
| `.claude/settings.json` | `git show` と `bash` の許可の項目はない | 変えない |
| CLAUDE.md「Development Notes」（`:680`） | `make test` が bash と git を求める理由として `TestLightweightCheck` を挙げる | 変えない（AC-17）。新しいテストも同じ道具を求める（本計画 §5.1） |
| 既存のテスト | 設計書 3.8 の表のとおり | 変えない（H-03） |

### 1.5. 名前の変更

| 種類 | 変更前 | 変更後 |
|---|---|---|
| 型 | `lwBash` | `scriptBash` |
| 関数 | `lightweightBashes` | `scriptBashes` |
| `lwBash`・`lightweightBashes` のファイル | `cmd/yt2column/lightweight_docs_test.go` の中 | `cmd/yt2column/test_helpers_bash.go`（新規、`//go:build test`） |
| `codeTexts` のファイル（名前は変えない） | `cmd/yt2column/lightweight_docs_test.go` の中 | `cmd/yt2column/docs_test.go` |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 定義（設計書 8 章の優先順位 1）

**変更するファイル:** `docs/dev/conventions/requirements_process.md`、`docs/dev/conventions/adr.md`

-   [ ] **ステップ 1-1**: `requirements_process.md`「Recording in the PR description」を設計書 3.1 のとおり改める。`:388` の 1 文を、`<document path>`・索引の行・ADR の訂正の条件を述べる文に置き換え、訂正の段落の箇条の `:403-404` の文の後に 1 文を足す。同じ行にある前後の文は変えない。設計書 3.1 の「変えないもの」と「Editing an approved document」には触れない。
-   [ ] **ステップ 1-2**: 規約を設計書 3.3 のとおり改める。「Recording」の読み替えの句を消し（3.3.1）、「After the PR is created」の段落を書き換え（3.3.2）、「Discrepancy statements」の `:348-350` の文を置き換える（3.3.3）。見出しの語 `**After the PR is created.**` は残す。規約は ADR ではないので、訂正の行は書かない。

**完了の条件:** `make test` と `make lint` が通る（`TestADRRecords` と `TestInstructionDocPaths` を含む）。

### フェーズ 2: 候補のスクリプトとそのテスト（設計書 8 章の優先順位 2）

**変更するファイル:** `cmd/yt2column/test_helpers_bash.go`（新規）、`cmd/yt2column/lightweight_docs_test.go`、`cmd/yt2column/docs_test.go`、`.claude/commands/_lib/adr-correction-candidates.sh`（新規）、`cmd/yt2column/adr_candidates_docs_test.go`（新規）

-   [ ] **ステップ 2-1（リファクタリングのコミット）**: 本計画 §1.5 の表のとおり、`lwBash` と `lightweightBashes` を `test_helpers_bash.go` に移して名前を変え、`lightweight_docs_test.go` の使う箇所をすべて新しい名前にする。`codeTexts` を名前を変えずに `docs_test.go` に移す。関数のコメントは、`/runissue` に限らず、テストが bash のスクリプトを実行する 2 つの bash を返すと述べる。振る舞いは変えない。`TestLightweightCheck` と `TestLightweightCheckCallSite` がそのまま通ることを確かめる。`make fmt`・`make test`・`make lint` を通し、このコミットには他の変更を入れない。
-   [ ] **ステップ 2-2**: 候補のスクリプトを書く。設計書 3.2.2 の表のすべての性質と、本計画 §1.3 の出力の形・比較の方法を持たせる。冒頭のコメントに、使い方、契約（表の性質）、出力と終了コード、`/fixpr` が `origin/main` の版を実行すること、bash で書く理由（本計画 §1.3 の ADR-0014 の判断）を書く。コメント・診断のメッセージ・出力は英語にする。bash 3.2 で動く書き方にする。
-   [ ] **ステップ 2-3**: `adr_candidates_docs_test.go` に `TestADRCorrectionCandidates` を足す。各場合を、`scriptBashes` が返すそれぞれの bash で実行する。
    -   **土台:** ファイルの中に小さな git の土台を持つ。具体的には、`t.TempDir` のリポジトリ、場合ごとの空の `HOME` と `TMPDIR`、許可リストの環境（`lwFixture.env` と同じく `PATH`・`HOME`・`TMPDIR`・`GIT_CONFIG_NOSYSTEM=1`・`GIT_TERMINAL_PROMPT=0`・`LC_ALL=C` だけ）、決まった作者での git の実行である。`main` に ADR などのファイルをコミットし、`refs/remotes/origin/main` をそこに向けてから、ブランチを作る。
    -   **場合:** 設計書 7.1 の表の 15 の場合に加えて、設計書 3.2.2 の性質のうち、その表では確かめられないものについての場合を足す。
        -   参照: `refs/remotes/origin/main` がなく、変更も何もないときも、0 でない終了コードで終わる。
        -   `main` の判定: 参照はあるが木を読めない（参照がブロブを指す）ときは、0 でない終了コードで終わる。
        -   比較: 空の内容・NUL を含む内容・ないファイルは一致なしになる。一致と一致なしを混ぜた複数の引数では、引数の順に 1 行ずつ出力する。一覧が空のときも、引数ごとに `no-match` を出力する。AC-05 が挙げる値（`..` を含むもの、大文字を含むもの、末尾の `/`、索引）は、それぞれ一致なしになる。
        -   入力: 標準入力と、`GIT_DIR` などの `GIT_` で始まる環境変数を与えても、出力が変わらない。
        -   環境: `LC_ALL` を C 以外の UTF-8 のロケールにしても、`git` を置き換える関数を定める `BASH_ENV`・書き出されたシェルの関数・`SHELLOPTS` を与えても、出力が変わらない。git の設定は、リポジトリと全体の両方の範囲で試す。設定には、素の git の出力を実際に変えるものを選び、比べる前に、その設定が素の git の出力を変えることを確かめる（`checkIgnoresGitConfig`、`lightweight_docs_test.go:1716-1748` と同じ形）。
        -   失敗: 途中の段の git が失敗する場合は、パイプラインの最後でない段で失敗させる。

        各場合は、確かめる性質だけがそのファイルを一覧から除くように作る（設計書 7.1）。出力の行と終了コードの両方を確かめ、実行のたびに場合の `TMPDIR` が空であることも確かめる。
    -   **テストのソース:** 形の外の名前（規約の ADR のファイル名の形に合わない名前）は、ほかの土台のファイルと語幹が重ならない名前にする（macOS の APFS は大文字と小文字を区別しないため）。ASCII でない値は、Go の文字列で `\u` のエスケープで書く。
    -   **道具:** git・bash・`cmp` が見つからないときは、飛ばさずに、どの道具がないかを述べて失敗する。
    -   **壊して確かめる:** コミットの前に、設計書 3.2.2 の表の性質を 1 つずつ壊し、対応する場合が失敗することを確かめる。壊したものと、失敗した場合を、コミットのメッセージに書く（`CLAUDE.md`「Testing Strategy」）。壊してもどの場合も失敗しない性質には、場合を足す。名前の変更の検出のように観測できない性質は、その理由をコミットのメッセージとテストのコメントに書く。

**完了の条件:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。ステップ 2-1 のコミットは、移動と名前の変更だけを含む。

### フェーズ 3: コマンドとコメント（設計書 8 章の優先順位 3〜5）

**変更するファイル:** `.claude/commands/fixpr.md`、`cmd/yt2column/adr_candidates_docs_test.go`、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/runplan.md`、`internal/fixprguard/write.go`

-   [ ] **ステップ 3-1**: `fixpr.md` を設計書 3.2 のとおり改める。
    -   Phase 3 の `editorial` を求める文（`:484-490`）に、ADR の訂正と、含めない変更（PR 自身の文の直し、ADR の決定の変更）を足す（3.2.1）。
    -   「Correction list」の通る条件を、タスク文書の項目と ADR の項目に分ける（3.2.2）。
    -   「Correction list」の中に、候補のスクリプトを実行する手順を足す。実行は、ADR の項目が 1 つ以上あるとき、「Migration check」の後、Phase 3.5 の前に行う。コードブロックは、`refs/remotes/origin/main` の版をスクラッチパッドのファイル `<candidates-script>` に書き出す行と、それを `bash` で実行して `<path-file>` を渡す行の 2 行である。プレースホルダは、`TestLightweightCheckCallSite` が拒む `<check-script>` と別の名前にする。`path` を `<path-file>` に書く方法（項目ごとに 1 ファイル、そのままの内容で、改行を足さない）と、本計画 §1.3 の出力の読み方（すべてを落とす 3 つのときを含む）を文で述べる。
    -   落とした ADR の項目の決まった理由（3.2.2）を述べる。
    -   Phase 4 の 1 の文に、ADR の項目の訂正の行の `<document path>` には一致した一覧の値を書く、と足す（3.2.3）。
    -   Final report の 9 項目めに、Phase 6 が追記した訂正を、その文書と説明とともに 1 つずつ挙げる、と足す（3.2.4）。
    -   Phase 6 の手順の文は変えない。
-   [ ] **ステップ 3-2**: `adr_candidates_docs_test.go` に `TestADRCorrectionCandidatesCallSite` を足す。対象は `doctestutil.CommandDocs` の各文書である。次の 3 つを確かめる。
    -   スクリプトの名前か、`<candidates-script>`・`<path-file>` のどれかを含むコードブロックの行が、ステップ 3-1 の取り出しの行か実行の行のどちらかと等しい。
    -   それらを含むコードスパンが、名前かプレースホルダだけである。
    -   `fixpr.md` の Phase 3 の節（`doctestutil.SectionByHeading`）に、取り出しの行と実行の行がそれぞれ 1 回以上ある。

    取り出しの行が `refs/remotes/origin/main:` を略さずに使い、作業ツリーのパスを実行しないことは、行の全体が等しいことで確かめる。コードの取り出しには、`docs_test.go` に移した `codeTexts` を使う。コミットの前に、次のそれぞれでテストが失敗することを確かめ、コミットのメッセージに書く。
    -   取り出しの行を `origin/main:` に略したもの。
    -   作業ツリーのパスを実行する行。
    -   実行の行の `<path-file>` を `path` の値に置き換えたもの。
    -   正しい実行の行を残したまま、違う 2 つ目の実行の行を足したもの。
    -   取り出しの行か実行の行をなくしたもの。
-   [ ] **ステップ 3-3**: `local-cli-review.md` の step 5（`:205-209`）と step 7 の表の `Editorial correction:` の行（`:234`）を、設計書 3.4 のとおり ADR の訂正に広げる。PR 自身の文の直しには訂正の行を書かない、と足す。メンションの確かめ（`:243-254`）と失敗の種類の表は変えない。
-   [ ] **ステップ 3-4**: `runplan.md:63` の箇条の後に、設計書 3.5 の ADR の訂正の箇条を足す。
-   [ ] **ステップ 3-5**: `write.go:97-98` のコメントを、設計書 3.7 のとおり直す。変更前は「`// editorialCorrectionPrefix starts every line of a finish commit body: the`」と「`// line that records an editorial correction to an approved task document.`」の 2 行である。変更後は「`// editorialCorrectionPrefix starts every line of a finish commit body: the`」と「`// line that records an editorial correction to a task document or an ADR.`」の 2 行にする。コードの行と `checkCommitBody` のコメントは変えない。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る（`cmd/codexreview/docs_test.go` と、設計書 3.8 の既存のテストを含む）。

### フェーズ 4: ADR-0039 の訂正と差分の確認（設計書 8 章の優先順位 6・7）

**変更するファイル:** `docs/dev/adr/0039-adr-writing-criteria.md`、`docs/tasks/0074_adr_editorial_correction/_checks/main.go`（新規）、`docs/tasks/0074_adr_editorial_correction/_checks/data.json`（新規）、本計画（進み具合と §7 の記録）

-   [ ] **ステップ 4-1（ADR-0039 のコミット）**: `git fetch origin main`（ネットワークの操作なので、利用者の承認を得る）の後、`git grep -nE 'issues/732([^0-9]|$)' origin/main -- docs/dev/adr/` をもう一度実行し、ほかの ADR に #732 を扱うずれの記述がないことを確かめる。ADR-0039 の「決定」の 4 つ目の項目の最後の 4 文（設計書 3.9）を消す。項目のほかの文、ほかの節、「状態」、「結果」は変えない。「経緯」に項目を足さない。このファイルだけをコミットする。
    -   **訂正の行:** このコミットのメッセージに、訂正の行を英語で第 1 カラムから書く（`requirements_process.md`「Recording in the PR description」の形）。行は `Editorial correction: docs/dev/adr/0039-adr-writing-criteria.md: ` で始め、何を消したかを自分の言葉で述べ、`; no decision changed` で終える。`@` と `<` を含めない。
    -   **この手順を明記する理由:** 実装の PR を動かす `/runplan` は、変える前の `runplan.md` を読んでいる。変える前の `runplan.md` は、ADR の訂正の行を書く手順を持たない（設計書 8 章の 6）。
-   [ ] **ステップ 4-2**: 確認のプログラムを `_checks/main.go` と `_checks/data.json` に書く（本計画 §1.3 の形）。確かめる項目は、設計書 7.2 の表の 5 つの行と、ADR-0039 を変える最初のコミットの訂正の行（本計画 §1.3）である。あわせて、`_lib/task-pr-handoff.md` と `runpr.md` に差分がないことも確かめる（設計書 2.2・3.6）。
    -   **AC-02 の範囲:** 基点の文書からは、置き換え前の 1 文（`:388` の行頭から「The line contains」の直前まで）を除く。先頭の文書からは、その位置にある新しい文と、訂正の段落の箇条に足した 1 文を除く。新しい文の範囲は、前の変えない文「…with `Editorial correction: `.」の終わりから「The line contains」の始まりまで、足す 1 文は「one item for each correction line.」の終わりから「When a PR is created」の始まりまでとする。境の空白と改行をどう扱うかを、プログラムのコメントに書く。置き換え前の文が先頭にないことも確かめる。
    -   **AC-16 の範囲:** `write.go` は、基点と先頭を `go/scanner` でトークンに分け、コメントのトークンを除いた並びが等しいことを確かめる。`internal/fixprguard/` と `cmd/fixprguard/` のほかのファイルは、1 バイトも違わないことを確かめる。
    -   **今と同じであることの行:** 次の部分が 1 バイトも違わないことも確かめる。`fixpr.md` の Phase 6 の節（AC-09・AC-15）、`fixpr.md` の「Correction list」のタスク文書の項目の条件の箇条（`:537-541`、AC-06）、`local-cli-review.md` のメンションの確かめ（`:243-254`）と失敗の種類の表（AC-10）。設計書の案どおりに書くと、これらの部分を変えることになるときは、その行を外し、理由を §7 に書く。
    -   **入力:** 2 つのコミットを引数で受け取り、比べる文書を作業ツリーから読まない。
    -   **部分の決め方:** 部分の位置は、行番号ではなく、見出しと変えない文の内容で決める。
    -   **終了コード:** 違反は 1、読み取りの失敗は 2 で終わる。基点が先頭の祖先でないときは、読み取りの失敗にする。
    -   **出力:** 比べた 2 つのコミットの完全な ID と、項目ごとの結果を含める。
    -   **ソースの文字列:** コメント・識別子・文字列は英語で書く。日本語は `data.json` に置く。
    -   `make fmt` と `go vet -tags test ./docs/tasks/0074_adr_editorial_correction/_checks/` を通す。
-   [ ] **ステップ 4-3**: 確認のプログラムが違反と読み取りの失敗を検出できることを確かめる。手順は 0070 の計画のステップ 1-4 にならう。PR のブランチから `git worktree` で使い捨てのブランチを作り、壊し方を 1 件ずつ一時的なコミットにして、それぞれを先頭として走らせる。各項目について、その項目だけが失敗することと、終了コードを確かめる。壊し方の例は次のとおりである。
    -   `requirements_process.md` の訂正の行の箇条の変えない文（「The line contains no `@` or `<`」を含む）と、形のコードブロックに、それぞれ 1 バイト足す。
    -   「Editing an approved document」に 1 バイト足す。
    -   `write.go` のコードの行をコメントにする。末尾にコメントのある行のコードのトークンを変える。`cmd/fixprguard/` のファイルを変える。
    -   AC-17 の 4 つのファイルと、ステップ 4-2 の「今と同じであることの行」の部分を、1 つずつ変える。
    -   ADR-0039 の消す 4 文の 1 つを残す。ほかの項目を変える。
    -   ADR-0039 の部分の見出しと、AC-02 の範囲の目印を、それぞれ消すか 2 回現れるようにする（読み取りの失敗）。

    コミットのメッセージを壊すもの（訂正の行をなくす、`@` か `<` を含める、`; no decision changed` で終えない、ステップ 4-1 のコミットでほかのファイルも変える）は、使い捨てのブランチでステップ 4-1 のコミットを作り直し、その上の先頭で走らせる。一時的なコミットを上に足しても、ステップ 4-1 のコミットは変わらないためである。

    使い捨てのブランチは push せず、確かめた後に worktree とともに消す。壊し方ごとの結果をコミットのメッセージに書き、`_checks/` をコミットする。
-   [ ] **ステップ 4-4（記録のコミット）**: `make test` と `make lint` を実行して通ることを確かめる（AC-22）。`git fetch origin main` を実行する（ネットワークの操作なので、利用者の承認を得る）。作業ツリーに未コミットの変更がない状態で、確認のプログラムを引数なしで走らせる。出力・終了コード・比べた 2 つのコミットの完全な ID を本計画 §7 に書き、本計画の進み具合とともにコミットする。
-   [ ] **ステップ 4-5**: 記録した先頭の後に、本計画のファイルのほかを変えるコミット（`main` を取り込むコミットと、`/runplan` のフェーズの後のレビューの修正のコミットを含む）が入ったら、確認のプログラムを走らせ直して §7 の記録を改める。
    -   PR を作る前に ADR-0039 を直すときは、`git commit --fixup` と `GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash` でステップ 4-1 のコミットに畳み込む（PR を作る前は push していない）。畳み込みは、ステップ 4-1 より後のすべてのコミットを作り直すので、記録した先頭はブランチから外れる。そのため、畳み込んだ後は、確認のプログラムを走らせ直し、§7 の記録を新しいコミットの ID で書き直す。
    -   PR を作る前に、記録した先頭がブランチの祖先であることと、その後のコミットがどれも本計画のファイルしか変えていないことを確かめる。

**完了の条件:** 確認のプログラムが終了コード 0 で終わり、その記録が §7 にある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: ADR editorial corrections carried by the commands

**対象ステップ**: 1-1 / 1-2 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0074): ADR の編集上の訂正をコマンドの手順で記録して PR の説明へ運ぶ`

**レビュー観点**: 候補のスクリプトが設計書 3.2.2 の性質をすべて持ち、テストの各場合がその性質だけを確かめているか / fixpr.md が path をコマンドに渡さず、origin/main の版のスクリプトだけを実行するか / 規約・requirements_process.md・各コマンドの文面が設計書 3 章の案の意味どおりか / ADR-0039 の訂正の行と PR の説明の訂正の段落がそろっているか

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2・3-1 は、信頼しないデータ（エージェントが返した path）を確かめる信頼の境界の確認で、mkplan.md step 8 の panel mode の契機（security-gate）に当たる

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: 定義 | `requirements_process.md` と規約の変更 | 1-1・1-2 |
| M2: 候補のスクリプト | テストの共有の処理の移動、スクリプト、そのテスト | 2-1〜2-3 |
| M3: コマンド | `fixpr.md` と呼び出しのテスト、`local-cli-review.md`、`runplan.md`、`write.go` のコメント | 3-1〜3-5 |
| M4: ADR-0039 と差分の確認 | 訂正の行つきの ADR-0039 のコミット、`_checks/`、§7 の記録 | 4-1〜4-5 |

各マイルストーンは前のマイルストーンを前提とする。M3 の `fixpr.md` は、M2 のスクリプトの名前と出力の形を参照する。M4 の差分の確認は、M1〜M3 の変更が終わった後の状態を比べる。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | 定義・候補のスクリプトとテスト・コマンド・ADR-0039 の訂正・差分の確認 | frontier-required |

PR は 1 本である（設計書 1.2）。規約が PR を作った後の訂正を認める変更と、コマンドがそれを運ぶ変更が、同時に `main` に入るためである。PR の説明には、ADR-0039 の訂正の段落を載せる。この段落は、`_lib/task-pr-handoff.md` Part B step 3 の手順が、ステップ 4-1 のコミットの訂正の行から書く。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

-   `TestADRCorrectionCandidates`（ステップ 2-3）: 候補のスクリプトの、設計書 3.2.2 の表の各性質を、2 つの bash で確かめる。
-   `TestADRCorrectionCandidatesCallSite`（ステップ 3-2）: コマンドの定義が、候補のスクリプトを `refs/remotes/origin/main` の版としてだけ取り出して実行することを確かめる。
-   既存のテストは変えない（設計書 3.8）。ステップ 2-1 の移動と名前の変更は、`TestLightweightCheck` と `TestLightweightCheckCallSite` がそのまま通ることで確かめる。

### 4.2. 差分の確認

-   設計書 7.2 の表の各行と、ADR-0039 を変えるコミットの訂正の行を、確認のプログラムで 1 回確かめる（ステップ 4-2・4-4・4-5）。
-   確認のプログラムが違反と読み取りの失敗を検出できることは、ステップ 4-3 で壊し方ごとに確かめる。

### 4.3. 読んで確かめる

-   設計書 7.3 のとおり、AC-01・AC-03〜AC-15・AC-19 の文の意味を、PR のレビューで読んで確かめる。AC-05〜AC-07 のうち、セッションが `path` をファイルに書いて渡す手順と、出力に従って落とす手順も、`fixpr.md` を読んで確かめる。

### 4.4. 結合テスト・後方互換のテスト

-   **結合テスト:** PR を作った後に、PR の説明に ADR-0039 の訂正の段落があることを確かめる（設計書 7.4）。PR を作った後の `/fixpr` の経路は、候補のスクリプトが `origin/main` に入るまで ADR の項目をすべて落とすので、実装の PR の中では通しで確かめられない。
-   **後方互換:** `fixprguard` の振る舞いを変えないので、再インストールは要らない（要件書 4.4）。

## 5. リスク管理 (Risk Management)

### 5.1. 技術のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| bash の書き方の誤りで、性質の 1 つが崩れる（`pipefail` の抜け、コマンド置換による末尾の改行の除去、利用者の git の設定の影響） | 一覧の確かめを、通るべきでない項目が通ってしまう | ステップ 2-3 の各場合を 2 つの bash で実行し、性質を 1 つずつ壊して失敗を確かめる。比較はバイトで比べる道具で行う（本計画 §1.3） |
| タスク 0075 が先にマージされ、`lightweight_docs_test.go` が消える | ステップ 2-1 の移動の元がなくなる | ステップ 2-1 の移動は、`scriptBash`・`scriptBashes` を `test_helpers_bash.go` に、`codeTexts` を `docs_test.go` に置くことだけを目標にする。元のファイルがなければ、0075 のマージの前の版から写す。新しいテストは元のファイルに依存しない（本計画 §1.3） |
| タスク 0075 が CLAUDE.md「Development Notes」の `TestLightweightCheck` の項目を消すと、`make test` が bash・git・`cmp` を求める理由が書かれなくなる | 新しいテストの道具の前提が文書から消える | 本タスクは CLAUDE.md を変えない（AC-17）。0075 の設計で、この項目を `TestADRCorrectionCandidates` に書き換え、`cmp` も足すよう、PR の説明に記す |
| 文面を整えるうちに、設計書 3 章の案の意味が変わる | 要件と設計から外れる | 意味を変える言い換えをしない。変える必要が出たら、実装を止めて設計書を改める |
| PR のレビュアーが、ADR-0039 の変更を編集上の訂正と分類したことに異を唱える | ADR-0039 に「経緯」の項目が要り、訂正の行と差分の確認が事実と合わなくなる | 実装を止めて利用者に諮り、決めたことを本計画の概要の章の「実装の際に決めたこと（YYYY-MM-DD）」に記録する |
| 実装の PR 自身の `/fixpr` のラウンドで、ADR の項目がすべて落ちる | ADR の訂正が PR の説明に自動では載らない | 設計書 5.2 のとおりで、Final report に載った訂正をレビュアーが手で PR の説明に足す |

### 5.2. 日程のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| `fixpr.md`・`local-cli-review.md`・`runplan.md`・規約を変える別のタスク（0072・0073・0075 など）が先にマージされる | 行番号と文脈がずれ、`main` の取り込みで衝突する | 規約の「Taking in `main`」のとおりに取り込み、設計書の案を新しい文脈に当てはめる。ステップ 4-5 で差分の確認の記録を改める |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確認のプログラムは `docs/tasks/0074_adr_editorial_correction/_checks/`（`main.go` と `data.json`。以下、`_checks`）である。実行の記録（日付、基点と記録した先頭の完全なコミット ID、出力、終了コード）は、ステップ 4-4・4-5 で本節末の「実行の記録」に書く。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | manual | `requirements_process.md`「Recording in the PR description」の訂正の行の箇条を読む（設計書 3.1） | 1-1 |
| AC-02 | diff | `_checks`。訂正の行の箇条は、置き換えた文を前後で除いた残りが 1 バイトも違わない。訂正の段落の箇条は、足した 1 文を除いた残りと、形のコードブロックが 1 バイトも違わない（ステップ 4-2 の「AC-02 の範囲」） | 4-2〜4-5 |
| AC-03 | manual | 規約の「Recording」を読む（設計書 3.3.1） | 1-2 |
| AC-04 | manual | `fixpr.md` の Phase 3 の `editorial` の文を読む（設計書 3.2.1） | 3-1 |
| AC-05 | test | `cmd/yt2column/adr_candidates_docs_test.go::TestADRCorrectionCandidates`（形の判定と比較の場合） | 2-3 |
| AC-05 | test | `cmd/yt2column/adr_candidates_docs_test.go::TestADRCorrectionCandidatesCallSite`（`path` を引数にしない呼び出しの形） | 3-2 |
| AC-05 | manual | `path` をファイルの内容として渡す手順を `fixpr.md` で読む | 3-1 |
| AC-06 | test | `TestADRCorrectionCandidates`（変更の判定と `main` の判定の場合） | 2-3 |
| AC-06 | diff | `_checks`（`fixpr.md` のタスク文書の項目の条件の箇条が前後で 1 バイトも違わない） | 4-2〜4-5 |
| AC-06 | manual | `description` の条件を `fixpr.md` で読む | 3-1 |
| AC-07 | test | `TestADRCorrectionCandidates`（PR 自身が作った ADR・索引・形の外の名前が一覧に出ない場合） | 2-3 |
| AC-07 | manual | 落とした項目を Final report に載せ、ほかの修正を止めないことを `fixpr.md` で読む | 3-1 |
| AC-08 | manual | `fixpr.md` の Phase 3 の、`editorial` に含めない変更の文を読む | 3-1 |
| AC-09 | diff | `_checks`（`fixpr.md` の Phase 6 の節が前後で 1 バイトも違わない） | 4-2〜4-5 |
| AC-09 | manual | `fixpr.md` の Phase 4 の、一覧の値で訂正の行を書く文と、メンションの確かめ・`finish`・失敗の扱いが変わらないことを読む | 3-1 |
| AC-10 | diff | `_checks`（`local-cli-review.md` のメンションの確かめと失敗の種類の表が前後で 1 バイトも違わない） | 4-2〜4-5 |
| AC-10 | manual | `local-cli-review.md` の step 5 と step 7 の表を読む | 3-3 |
| AC-11 | manual | 同上（PR の前後を区別しないこと） | 3-3 |
| AC-12 | manual | `runplan.md` の足した箇条を読む | 3-4 |
| AC-13 | manual | 規約の「After the PR is created」と「Discrepancy statements」を読む | 1-2 |
| AC-14 | manual | `requirements_process.md` の訂正の段落の箇条を読む。PR の説明の訂正の段落を見る（設計書 7.4） | 1-1・PR の作成の後 |
| AC-14 | diff | `_checks`（`_lib/task-pr-handoff.md` と `runpr.md` に差分がない） | 4-2〜4-5 |
| AC-15 | diff | `_checks`（`fixpr.md` の Phase 6 の節が前後で 1 バイトも違わない） | 4-2〜4-5 |
| AC-15 | manual | Final report の 9 項目めを読む | 3-1 |
| AC-16 | diff | `_checks`（`write.go` のコメントを除いたトークンの並びが前後で等しく、`internal/fixprguard/` のほかのファイルと `cmd/fixprguard/` に差分がない） | 4-2〜4-5 |
| AC-17 | diff | `_checks`（`runissue.md`・`mergepr.md`・`CLAUDE.md`・ADR-0036 に差分がない） | 4-2〜4-5 |
| AC-18 | diff | `_checks`（「Editing an approved document」の節が前後で 1 バイトも違わない） | 4-2〜4-5 |
| AC-19 | diff | `_checks` の AC-20 の行（ADR-0039 の差分が「決定」の 4 文の削除だけなので、「結果」は変わらない） | 4-2〜4-5 |
| AC-19 | manual | ADR-0039 の「結果」を読む | 4-1 |
| AC-20 | diff | `_checks`（ADR-0039 の差分が「決定」の 4 つ目の項目の中の 4 文の削除だけで、4 文のどれも残らない。ADR-0039 を変える最初のコミットに訂正の行がある） | 4-2〜4-5 |
| AC-20 | manual | PR の説明の訂正の段落に ADR-0039 の項目があることを見る（設計書 7.4） | PR の作成の後 |
| AC-21 | test | `TestADRCorrectionCandidatesCallSite`。既存の `TestFixprguardCallSites`・`TestFixprPhase6CountFilter` が通る（`make test`、CI の `doc-guard`） | 3-2 |
| AC-22 | static | `make test`・`make lint` | 4-4 |

### 実行の記録

（ステップ 4-4・4-5 で書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 本計画 §7 のすべての行の検証が通る。
-   **品質:** `make test`・`make lint`・`make deadcode` が通る。`TestADRCorrectionCandidates` の各場合が、2 つの bash で通る。
-   **セキュリティ:** 候補のスクリプトのテストが、形の外の名前・PR 自身が作った ADR・ローカルのブランチによる参照のすり替え・見た目の似た文字と末尾の改行を含む内容を、一覧に入れないか一致させないことを確かめる。呼び出しのテストが、作業ツリーの版を実行しないことを確かめる（設計書 7.4）。
-   **文書:** 規約・`requirements_process.md`・コマンドの定義・ADR-0039 の文が、PR のレビューで読んで設計書 3 章と食い違わない。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの後、`/runpr` の案内に従って、タスクの issue（#732）を閉じる。
-   ADR の訂正の行を squash コミットの本文に引き継ぐかは、#747 で扱う（設計書 9 章）。
-   タスク 0075 の設計で、CLAUDE.md「Development Notes」の bash と git の項目を、`TestADRCorrectionCandidates` を挙げ、`cmp` も求める形に改める（本計画 §5.1）。
