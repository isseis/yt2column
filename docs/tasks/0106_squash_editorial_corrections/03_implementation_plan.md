# 実装計画書：編集上の訂正の行を、squash コミットの本文に引き継ぐ

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-19 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容と `file:line` の行番号は、コミット `21aef492` で確かめた。設計書が確かめたコミット `60de9ced` から、本計画が触れるファイルのうち変わったのは `docs/dev/adr/0047-runpr-merge-question.md` だけである（`git diff --stat 60de9ced 21aef492` を本計画が触れるパスに限って実行した結果）。この変更は、設計書の PR が ADR-0047 の「決定」に項目を足したもので、設計書は ADR-0047 の行番号を引いていない。そのため、設計書が引く行番号は今も合う。

用語は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う（ずれの記述・訂正の行・引き継ぐ範囲・範囲の順・受け付けない文字・squash の本文・承認の質問・落とした記録の行・受け付けた行・拒否した行・落とす指定・訂正のブロック・予約した接頭辞・訂正の確かめ・マージの後の照合）。本計画は、次の用語を加える。

-   **トレーラーの読み取り:** 設計書 3.4 の確かめ 7 で、`git interpret-trailers --parse --no-divider` を、利用者の git の設定を読ませずに走らせること（§1.3 の決定 2）。
-   **配線:** `internal/mergepr` の部品を `Tool` のメソッドと `cmd/mergepr` のサブコマンドから呼び、`/mergepr`・`/runpr` の手順から使えるようにすること。PR-2 で行う。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおり、`mergepr` に訂正の行の読み取り・分類・訂正のブロック・訂正の確かめ・マージの後の照合を足す。これを `prepare`・`paragraph`・`check-body`・`merge`・`cleanup` に配線し、`/mergepr` と `/runpr` の手順、手引き、規則の文書、ADR をそろえて、要件書の F-001〜F-005 を満たす。

### 1.2. 実装の原則

-   **設計書 8 章の優先順位のとおり、道具の部品を先に、配線と文書を後に入れる。** フェーズ 1 は 8 章の 1 のうち配線を除く部品、フェーズ 2 は 8 章の 1 の残り（配線とプロトコル番号）と 8 章の 2・3 である。PR はフェーズごとに 1 つ、計 2 つにする（§1.3 の決定 1）。
-   **PR-1 は、道具の振る舞いもプロトコル番号も変えない。** 部品は `Tool` からまだ呼ばれず、テストからだけ呼ばれる。
-   **プロトコル番号を変える PR では、道具・コマンドの定義・手引き・文書のガードを同じ PR で変える（設計書 8 章の 2、ADR-0014）。** PR-2 の `Protocol` は、実装の時点の `origin/main` の値 + 1 にする（`CLAUDE.md`「Protocol numbers」）。
-   **値の中身で振る舞いを選ぶコードは、`CLAUDE.md` の例外と同じ PR で入れる。** 例外の文は PR-1 で最終の形で書く（§1.3 の決定 4）。
-   **テストは、確かめる仕組みを壊して失敗することを確かめ、そのことをコミットのメッセージに書く（`CLAUDE.md`「Testing Strategy」）。** 各ステップの「壊して確かめる」は、この義務を実装者のタスクとして書いたものである。

### 1.3. 計画の際に決めたこと（2026-11-19）

1.  **PR を 2 つに分ける（人が選んだ）。** PR-1 は `internal/mergepr` の部品とそのテスト、`CLAUDE.md` の例外。PR-2 は、配線、`Protocol` の引き上げ、`cmd/mergepr`、`mergepr.md`・`runpr.md`・手引き・`docs_test.go`、規則の文書、`package_reference.md`、ADR-0039・ADR-0036・ADR-0047 のずれの記述の削除である。
    -   **理由:** 部品（バイト列の読み取り、文字の分類、確かめ）と、マージの手順の変更を、別々に詳しくレビューできる。PR-1 は配線しないので、`main` の道具の振る舞いは PR-2 まで変わらない。
    -   **採らなかった案:** 1 つの PR にまとめる案は、新しい Go のファイルとテスト、`runpr.md` の大きな改訂が 1 つのレビューに集まるので採らない。3 つに分けて規則の文書と ADR を最後に置く案も採らない。理由は 2 つある。1 つは、ADR-0047 のずれの記述を、`runpr.md` を直す PR で消す必要があること。もう 1 つは、配線の PR と規則の文書の PR の間、ADR-0039 のずれの記述が半分だけ正しい状態になることである。
    -   ADR：なし。PR の切り方はこのタスクの中で閉じる。
2.  **トレーラーの読み取りでは、利用者の git の設定を読ませない（人が選んだ）。** 設計書 3.4 が計画に委ねた 2 点を、次のとおり決める。
    -   設定を読ませない方法: `git` を、環境変数 `GIT_CONFIG_NOSYSTEM=1` と `GIT_CONFIG_GLOBAL=/dev/null` を付けて走らせる。`HOME` と `XDG_CONFIG_HOME` は、新しく作った空の一時ディレクトリに向ける。走らせる場所はリポジトリの外のその一時ディレクトリにし、`GIT_CEILING_DIRECTORIES` にその親のディレクトリを指定して、リポジトリを探させない（作業ディレクトリそのものを指定しても探すのを止めない。git 2.50.1 で確かめた）。`HOME` と `XDG_CONFIG_HOME` を向けるのは、`GIT_CONFIG_GLOBAL` を読まない古い git でも、利用者の設定を読ませないためである。
    -   本文の渡し方: 件名・空行・本文をつなげたメッセージを、標準入力で渡す（`cmdrunner` の `RunInput`）。一時ファイルは作らない。
    -   **理由:** git 2.50.1 で、同じメッセージに対する `--parse` の出力が、`~/.gitconfig` の `trailer.separators` と `core.commentChar` で変わった（2026-11-19 にコミット `21aef492` の作業ツリーで確かめた）。設定を足すと、既定の設定では何も出力しないメッセージから 2 行が出力された。上の環境を付けると、同じ `~/.gitconfig` があっても何も出力しなかった。GitHub や後の読み手は利用者の設定を使わないので、既定の設定で確かめる。
    -   **採らなかった案:** `-c` で既知のキーだけを固定する案は、`include` や、将来足されるキーが残るので採らない。利用者の設定のまま走らせる案は、確かめの結果がマシンの設定で変わるので採らない。
    -   ADR：なし。確かめの実装の細部で、このタスクの中で閉じる。
3.  **範囲のコミットは、1 コミットに 1 回 `git cat-file commit <oid>` で読む（人が選んだ）。**
    -   **理由:** 出力がそのままコミットオブジェクトで、読む形が少ない。PR のコミットは数十件ほどで、マージ 1 回の時間は CI の待ち時間に比べて無視できる（`CLAUDE.md`「Performance」）。
    -   **採らなかった案:** `--batch` で読む案は、起動は 1 回で済むが、ヘッダーの行と長さの解析が要り、壊れ方が増えるので採らない。
    -   ADR：なし。
4.  **`CLAUDE.md` の「Declare, don't infer」の例外（設計書 1.2 の決定 12）は、PR-1 で最終の形で書く。** 例外の範囲に入るコード（訂正の行の読み取り、訂正の確かめ）は PR-1 で入るので、例外より先に `main` に入らないようにする。PR-1 の時点では、まだサブコマンドから呼ばれない。それでも、PR-2 で文を直さずに済むように、配線した後の形で書く。
    -   ADR：なし。`CLAUDE.md` の規則は ADR にしない（`CLAUDE.md`「Design decisions (ADR)」）。
5.  **範囲の列挙は、順序を引数で選べる形にして共有する。プロトコル番号の確かめは、今の順序のままにする。** 設計書 2.2 の「範囲の順（`--topo-order`）で列挙できる形にして」に従う。訂正の行の読み取りは `--topo-order` で、プロトコル番号の確かめは今と同じ順序で列挙する。プロトコル番号の確かめの報告に出るコミットの並びが、本タスクで変わらないようにするためである。設計書 3.7 の「`protocolcheck_test.go` の範囲の列挙のテストは、`--topo-order` を足した列挙に合わせる」は、共有にした列挙の形に既存のテストを合わせることと読む。プロトコル番号の確かめの順序は変えないので、既存のテストの期待値は変わらない。
    -   ADR：なし。
6.  **文書の置き換えの値の名前は `<drop>` と `<rejected-drop>` にする（設計書 3.7 が計画に委ねた点）。** `<drop>` は `1:disputed` に、`<rejected-drop>` は 40 桁の 16 進数のコミット ID と行番号（`<ID>:3` の形）に置き換える。文書は、落とす指定を `--drop <drop>` と `--drop-rejected <rejected-drop>` の形で書く。`[...]` や `...` で省略や繰り返しを表す書き方は、`text` のコードブロックか、コードスパンの外の地の文に置く。1 つの呼び出しに `<drop>` を 2 回書かない（同じ値に置き換わり、重複として拒まれる）。これらの規則は、`TestDocumentedInvocationsParse` が `sh` のブロックとコードスパンの呼び出しをそのまま `parse` に通すためにある。
    -   ADR：なし。
7.  **拒否した行の問い（設計書 1.2 の決定 10）は、`/runpr` のラウンドとして数え、詳細ファイルを `rejected-<round>.md` とする。** 問いの文は、chief がマージの質問と読む形（`internal/chief/screen.go:113` の `mergeQuestionText` と、空白と改行を除いて比べる `:117` の `mergeQuestionSqueezed`。「`PR #<番号> をどうしますか（第 <回> 回。…）`」）のどちらにも合わない文にする。chief がこの問いをマージの質問と取り違えないためである。
    -   ADR：なし。ADR-0047 の決定の細部である（問いを置くことは、設計書の PR で ADR-0047 に足した）。
8.  **`cleanup` に `--body-file` を渡す場所を、設計書 3.5 の 3 つの回復の道筋に加えて、次の 2 か所でも決める。**
    -   `runpr.md` step 5 の再開の報告（`.claude/commands/runpr.md:1435-1437`）: 同じ実行の中なので、マージに使った本文のファイル（`approved-<round>.body` か本文のファイル）を付けて示す。
    -   `runpr.md` step 1 の、PR がすでにマージ済みのときの片付け（`.claude/commands/runpr.md:117-118`）: 新しい実行で、マージに使った本文が分からない。そのため `--body-file` を付けない。この場合、`mergepr` は `corrections carried: not verified` の行を出す（設計書 3.5 の「渡されないとき」）。
    -   ADR：なし。

### 1.4. 既存コード調査結果

コミット `21aef492` で確かめた。

-   **`log.txt` の書き方（変えない）:** `internal/mergepr/mergepr.go:250` は `git log --format=%h %s%n%n%b` で書く。設計書 2.2 のとおり、下書きの材料として今のまま使う。
-   **`git log` は再エンコードし、`cat-file` はしない:** 手元の一時リポジトリで確かめた（2026-11-19、git 2.50.1）。`encoding ISO-8859-1` のヘッダーと、`\xff`・U+FFFE（非文字）・行末の CR を含むメッセージのコミットを、`git hash-object -t commit -w --stdin` で作った（拒まれずに作れた）。`git cat-file commit` は、格納したバイト列をそのまま出した。`git log --format=%B` は、`\xff` などを UTF-8 に変換して出した。テストの補助（ステップ 1-2）は、この方法でコミットを作る。今の `gitRepo.Commit`（`internal/mergepr/test_helpers_git.go:82-115`）は `commit-tree -m` を使うので、任意のバイト列のメッセージを作れない。
-   **範囲の列挙:** `listPRCommits`（`internal/mergepr/protocolcheck.go:508-514`）は `rev-list --reverse --no-commit-header <revListFormat> <head> ^<main>` で列挙し、`--topo-order` を付けない。`history`（`:585-608`）は shallow を拒み、`origin/main` を `resolveCommit`（`:487-498`）で解決し、`isObjectID`（`:474-484`）で head を確かめる。このうち `origin/main` を決め打ちで解決するのは `history` だけで（`mainRef`、`:33`・`:593`）、`listPRCommits` と `resolveCommit` は渡されたコミットを使う。訂正の行の読み取りは `origin/<baseRefName>` を読むので、base を引数に取る形にする（ステップ 1-1）。`history` は、どの失敗も `protocol check: ` で始まる `*protocolUnreadableError` に包むので、共有する部分は素の誤りを返す形にする（ステップ 1-1）。プロトコル番号の確かめの `changedTools`（`:656-674`）は列挙の順に変更を並べるので、その順序は変えない（§1.3 の決定 5）。
-   **base の名前の確かめと解決:** `refs/remotes/origin/` に base の名前をつなげた文字列を、`git check-ref-format` に通す。手元の git 2.50.1 で、`refs/remotes/origin/main^` と `refs/remotes/origin/a..b` は終了コード 1、`refs/remotes/origin/feat/x` は 0 だった（2026-11-19）。ただし、通った名前でも `rev-parse` は別のものとして読みうる。その ref がないとき、`refs/remotes/origin/feat-1-g9c6a73bb` は `git describe` の出力として読まれて短縮のコミット ID に解決し、`refs/heads/refs/remotes/origin/zzz` という名前のブランチがあれば `refs/remotes/origin/zzz` はそのブランチに解決した（`resolveCommit` と同じ `rev-parse --verify --quiet '<名前>^{commit}'` の形。git 2.50.1）。`git show-ref --verify --hash <ref>` はどちらも拒み（終了コード 128）、`refs/remotes/origin/main` は解決した。そのため base は、名前の確かめの後に `git show-ref --verify --hash` で ref そのものの値を読み、その ID を `resolveCommit` に渡してコミットであることを確かめる（ステップ 1-1）。
-   **強化した git の runner:** `newGitRunner`（`internal/mergepr/runner.go:18-20`）は、`os.Environ()` から `GIT_*` を除いた環境（`cmdrunner.GitEnv`、`internal/cmdrunner/git.go:36-45`）で git を走らせる。`internal/config/envaccess_test.go:100` は、`internal/mergepr/runner.go` の `os.Environ` の呼び出しを 1 回と数える。トレーラーの読み取りの環境は、同じ 1 回の呼び出しから作る。呼び出しを増やすときは、同じコミットで `envaccess_test.go` の表とその説明を直す。`cmdrunner.NewOS` の runner は `RunInput`（`internal/cmdrunner/runner.go:106`）で標準入力を渡せる。
-   **`merge` の本文の渡し方:** `internal/mergepr/mergepr.go:382` は `gh pr merge … --body <本文>` で渡す。テストでこの引数を書くのは `internal/mergepr/mergepr_test.go:298` の `mergeStep` の 1 か所で、`TestMerge`・`TestMergeKeepsSummaryWhenMergeFails`・`TestMergeNumberCheck` がそれを使う。`fakeRunner.Run`（`internal/mergepr/test_helpers.go:56-67`）は引数を `slices.Equal` で比べるので、一時ファイルの乱数のパスと、返る前に消えるファイルの中身を確かめるには、手順にフックが要る（ステップ 2-1）。`Merge` と `Cleanup` の引数が変わるので、`TestMergeRejectsLivePR`・`TestMergeRejectsInvalidSubject`・`TestMergeRunsProtocolCheck`・`TestProtocolFetchFailureStops` の merge の部分・`Cleanup` を呼ぶテストのすべても呼び方を直す。一時ファイルは `os.CreateTemp` で作る（所有者だけが読み書きできるモード 0600 で作られる。Go の `os` パッケージの説明による）。`internal/privatefile` は chief のための確かめた読み書きで、ここでは要らない。
-   **示すコマンド:** `cleanupCommand`（`internal/mergepr/mergepr.go:131`）と `ShownCommands`（`:138-140`）は `cleanup --state` だけを作る。`cmd/mergepr/main_test.go:75` の `TestShownCommandsParse` は、示すコマンドを `parse` に通す。`cleanup --body-file` を示すようにするので、`ShownCommands` に本文のパスを渡す形にする（ステップ 2-1）。
-   **`cmd/mergepr`:** `parse`（`cmd/mergepr/main.go:67-104`）は `flag` で 1 回ずつの値を読む。`--drop` と `--drop-rejected` は繰り返して渡すので、`flag.Value` を実装する型で集める。`errUsage`（`:25-27`）、`prepare` の出力（`:118-131`）、`printError`（`:44-51`）、`printReport`（`:152-165`）を変える。`prepare` の出力と `printReport` は標準出力に直接書くので、テストできるように書き先を受け取る形にする（ステップ 2-2）。
-   **文書のガード:** `cmd/mergepr/docs_test.go` の `definitionCalls`（`:30-33`）、`placeholderValues`（`:86-95`）と `placeholder`（`:98`）。`placeholder` は `<…>` の形を拾うので、`<drop>` と `<rejected-drop>` は表に足すだけで拾われる。
-   **`--protocol` を書く文書:** `mergepr` の `--protocol 2` は、`.claude/commands/mergepr.md`（7 か所）、`.claude/commands/runpr.md`（11 か所）、`docs/dev/guides/mergepr_guide.md`（11 行に 13 か所）にある（各ファイルで `grep -o -- '--protocol 2'` を数えた結果。どれも `mergepr` の呼び出しか、手引きの地の文の `mergepr` の番号である）。`TestDocumentedProtocolMatches` が新しい番号と比べる。手引き 3 章の地の文「現在の番号は `2`」（`docs/dev/guides/mergepr_guide.md:48`）は `--protocol` の形でないので、ガードが見ない（§4.4 の照合の項目）。
-   **chief の読み取り:** `internal/chief/screen.go:113` の `mergeQuestionText` は、`/runpr` のマージの質問の文を読む。`:117` の `mergeQuestionSqueezed` は、同じ文を空白と改行を除いて読む。拒否した行の問いの文は、どちらにも合わない形にする（§1.3 の決定 7）。マージの質問の要約の行が 1 行増えることが chief の読み取りに触れるかは、ステップ 2-4 の後に §4.4 で読み合わせる。
-   **ADR のずれの記述:** `docs/dev/adr/0039-adr-writing-criteria.md:21`、`docs/dev/adr/0036-task-document-status-records.md:25`、`docs/dev/adr/0047-runpr-merge-question.md:24` の各項目の最後の 2 文が、#747 へのずれの記述である。ADR-0039 の `:55` と ADR-0036 の `:50`（「結果」）は、#747 の実装の後のことを述べる文で、ずれの記述ではないので変えない。
-   **`fixprguard` の分類との関係:** `internal/fixprguard/write.go:67-74` と `:97-100` は使い回さない（設計書 3.2）。
-   **重複しないテスト:** `TestParseRevList`（`internal/mergepr/protocolcheck_test.go:637`）は、列挙の出力の形の解析を確かめる。ステップ 1-1 は解析を変えないので、これを変えずに使う。`--topo-order` の順序は、ステップ 1-2 の `TestRangeLines` だけで確かめる。

### 1.5. 申し送りへの対応

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、どれも設計書の付録 A に扱いが記録されている。本計画は、付録 A の扱いをそのまま実装する（H-01 はフェーズ 1・2、H-02 はステップ 1-4・2-3、H-03 はステップ 2-3・2-4、H-04 はステップ 2-6・2-7、H-05 はステップ 1-2〜1-5・2-5、H-06 はステップ 2-5、H-07 はステップ 1-5・2-1、H-08 はステップ 1-3）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 訂正の行の部品（設計書 8 章の 1 のうち、配線を除く部品）

**変えるファイル:** `internal/mergepr/protocolcheck.go`、`internal/mergepr/protocolcheck_test.go`、`internal/mergepr/corrections.go`（新規）、`internal/mergepr/corrections_test.go`（新規）、`internal/mergepr/errors.go`、`internal/mergepr/runner.go`、`internal/mergepr/test_helpers_git.go`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`

-   [x] **ステップ 1-1**: 範囲の列挙を共有できる形にする（設計書 3.1 の「列挙の部品の共有」、§1.3 の決定 5）。
    -   `history` を 2 つに分ける。1 つは、段ごとに素の誤りを返す共有の関数（shallow の拒否、base と head の解決、列挙）である。もう 1 つは、各段の誤りを今の `protocolUnreadableError` の種類（`unreadableHistory`・`unreadableCommits`）に包む、プロトコル番号の確かめの側である。訂正の行の読み取りは、同じ素の誤りを `errCorrectionUnreadable` に包む（設計書 4.1）。
    -   列挙に順序の引数を足し、base を `origin/main` 決め打ちから「`refs/remotes/origin/<baseRefName>` の ref の値」に一般化する。base の名前は、`refs/remotes/origin/` をつなげた名前が `git check-ref-format` を通るときだけ使い、`git show-ref --verify --hash` で ref そのものの値を読む（§1.4）。
    -   プロトコル番号の確かめは、今と同じ順序と同じ base（`origin/main`）で列挙する。既存の `TestCheckProtocolHistories` と `TestCheckProtocolUnreadable`（誤りの種類）が変わらず通ることを確かめる。
    -   テスト（`protocolcheck_test.go`）: 名前の確かめは、台本にした runner で、通らない名前（`main^`、`a:b` など）では ref の読み取りも取得の refspec も作らずに止まることを確かめる。ref の値の読み取りは、実際の git で、名前の確かめは通るが `rev-parse` なら別のものに解決する名前（`git describe` の形の名前と、同じ名前のローカルのブランチがある名前）で止まることを確かめる。範囲の順そのものは、ステップ 1-2 の `TestRangeLines` で確かめ、ここでは重ねない。
    -   壊して確かめる: 名前の確かめを外して、台本にした runner の行が失敗すること。`show-ref` を `rev-parse` に替えて、`git describe` の形の行とローカルのブランチの行が失敗すること。
-   [x] **ステップ 1-2**: 訂正の行の読み取り `correctionSource` とその本番の実装を `internal/mergepr/corrections.go` に作る（設計書 3.1・3.5、§1.3 の決定 2・3）。
    -   `rangeLines`（範囲の訂正の行）、`mergedLines`、トレーラーの読み取りの 3 つを持たせる。トレーラーの読み取りを `correctionSource` に入れるのは、設計書 3.4 の確かめ 7 が「3.1 の読み取りを通して強化した git で呼ぶ」と定めるためである。
    -   メッセージは `git cat-file commit <oid>` の出力の最初の空行より後ろとし（ヘッダーに `gpgsig` などがあっても同じ）、LF で分け、件名を 1 行目と数え、行末の CR を残す。空行のない出力は読めないものとする。どのコミットで失敗しても、読めたところまでを返さない。
    -   `mergedLines` は、base の名前を確かめた後に作る refspec `+refs/heads/<base>:refs/remotes/origin/<base>` で、強化した git で取得し、squash コミットのメッセージから予約した接頭辞の 2 つで始まる行（件名を含む）を読む。
    -   トレーラーの読み取りの環境は §1.3 の決定 2 のとおりにし、一時ディレクトリは呼ぶたびに作って消す。標準入力を渡せる runner（`cmdrunner.InputRunner`）は、その一時ディレクトリと環境で呼ぶたびに作る（`runner.go` の `newTrailerRunner`）。`newGitRunner` は `Runner` を返すまま変えない。`os.Environ` の呼び出しは `runner.go` の 1 回のままにする（§1.4）。
    -   `test_helpers_git.go` に、任意のバイト列のメッセージと、指定した committer の日時を持つコミットを `git hash-object -t commit -w --stdin` で作る補助を足す。今の `gitIn` を標準入力を渡せる形にして使い、`gosec` の抑止を足すときは今の補助と同じく 1 行に限る。
    -   テスト（`corrections_test.go::TestRangeLines`、実際の git）: 件名の行の訂正の行と行番号、範囲の順、`main` から取り込んだコミットの除外、字下げした行・大文字と小文字の違う行・文の途中の語句の除外、パスの種類と残りの形を問わないこと、`encoding` のヘッダーのあるコミットのバイト列、`refs/replace` の無視、shallow のリポジトリでの停止。範囲の順は、日時が入れ違う 2 つの枝とマージのコミットで作り、既定の順序と `--topo-order` の順序が違うことを先に確かめる。
    -   テスト（`::TestRangeLinesStopsMidway`、`fakeRunner` で台本にした git）: 3 つのうち 2 つめの `cat-file` が失敗したとき、結果を返さずに `errCorrectionUnreadable` で止まること。空行のない出力でも止まること。
    -   テスト（`::TestMergedLines`、`file://` の origin を持つ実際の git）: `gpgsig` に似たヘッダーを持つ squash コミットの、件名と本文の 2 つの接頭辞の行を読むこと。取得の失敗で止まること。
    -   テスト（`::TestTrailerReadingIgnoresConfig`）: 設定があると `--parse` が行を出すメッセージについて、トレーラーの読み取りの出力そのものが空であることを、設定の置き場所ごとに確かめる。置き場所は `HOME` の `.gitconfig`、`XDG_CONFIG_HOME` の `git/config`、リポジトリの `.git/config` である。リポジトリの行のために、トレーラーの読み取りは、一時ディレクトリを作る親のディレクトリをテストから差し替えられる形にし、テストはそれをリポジトリの中に向ける（`GIT_CEILING_DIRECTORIES` がリポジトリを探させないことを確かめる）。
    -   壊して確かめる: `cat-file` を `git log` に替える、`--no-replace-objects` を外す、`--topo-order` を外す、読めたところまでを返す、`GIT_CONFIG_GLOBAL` と `HOME`・`XDG_CONFIG_HOME` の向け先をまとめて外す（`HOME` の行が失敗する）、`GIT_CEILING_DIRECTORIES` を外す（`.git/config` の行が失敗する）。`GIT_CONFIG_GLOBAL` と `HOME` の向け先は互いに重なる守りで、片方だけを外しても失敗しない。`GIT_CONFIG_NOSYSTEM` は、テストから system の設定を置けないので確かめられない。重なる守りの片方ずつと `GIT_CONFIG_NOSYSTEM` は、テストで確かめられない守りとしてコミットのメッセージに書く。
-   [x] **ステップ 1-3**: 受け付けない文字の分類と表示形を作る（設計書 3.2・3.6 の「表示形」）。`RejectKind`・`RejectedLine`、重複のまとめ（初めて現れた位置）、受け付けた行の番号付け、拒否した行のコミットと行番号を作る。
    -   テスト（`corrections_test.go::TestClassifyCorrectionLines`・`::TestShownForm`）: 設計書 7.1 の「分類（表）」の境界の文字を、受け付ける側と拒む側の両方から確かめる。表示形（`"` で囲む、`U+XXXX`、`\xNN`、食い違いの報告のタブ）も確かめる。テストの日本語の文字は `\u` のエスケープで書く（Go のソースは英語。`CLAUDE.md`「Go Idioms」）。
    -   壊して確かめる: タブの除外を外す、非文字の判定を外す、重複のまとめを外す。
-   [x] **ステップ 1-4**: 落とす指定と訂正のブロックを作る（設計書 3.3）。`DropCase`・`Drop`・`RejectedDrop`・`Drops`・`ParseDrop`・`ParseRejectedDrop`、落とした記録の行、`paragraph.txt` の中身のバイト列（空なら 0 バイト、空でなければ最後に空行）を作る。
    -   テスト（`::TestCorrectionBlock`・`::TestParseDrop`・`::TestParseRejectedDrop`）: 設計書 7.1 の「ブロック（表）」と、形の誤り（`:` のない値、数でない番号、`+1`・`01`・桁あふれ、0 以下の番号、知らない場合、40 桁の小文字の 16 進数でないコミット ID）を確かめる。行の数と長さに上限がないことも確かめる。
    -   壊して確かめる: 落とす前の重複のまとめを外す、最後の空行を外す、0 の件数を省く。
-   [x] **ステップ 1-5**: 訂正の確かめ（設計書 3.4 の 1〜7）とマージの後の照合（3.5 の比べ方）を、`correctionSource`・落とす指定・件名と本文のバイト列を受け取る関数として作る。`errors.go` に、`errCorrectionUnreadable`・`errCorrectionMismatch`・`errCorrectionsNotCarried`・`CorrectionMismatchError`・`CorrectionsNotCarriedError` を足す（設計書 4.1）。
    -   `paragraph` が使う確かめ 1〜3 だけの入口と、`check-body` と `merge` が使う確かめ 1〜7 の入口が、同じコードを通るようにする。
    -   テスト（`::TestCheckCorrections`）: 設計書 7.1 の「確かめ（表）」の各行が止まることと、止まらない場合を確かめる。止まるときは、誤りの種類（`errors.Is` と `errors.AsType`）と、欠けた行・余分な行などのフィールドを確かめる。確かめ 7 の行は、本番のトレーラーの読み取り（実際の git）を使う。
    -   テスト（`::TestTrailerReading`、実際の git）: 設計書 7.1 の「トレーラー」の振る舞い。
    -   テスト（`::TestCompareMergedLines`）: 一致・欠けた行・余分な行・読めないとき（`Detail`）。
    -   壊して確かめる: 確かめ 2（宣言のない拒否した行、範囲外の番号、拒否した行を指さない `--drop-rejected`）と確かめ 4〜7 を 1 つずつ外し、その確かめを対象にするテストの行が失敗すること。
-   [x] **ステップ 1-6**: `CLAUDE.md` の「Declare, don't infer」に、`mergepr` の訂正の行の読み取りと訂正の確かめの例外を足す（設計書 1.2 の決定 12、§1.3 の決定 4）。範囲は、コミットのメッセージの 1 列目の接頭辞と文字の分類、件名と本文の予約した接頭辞の行と段落の形、本文のトレーラーの読み取りである。理由と境界は、ほかの道具の例外と同じ組み立てで書く。
-   [x] **ステップ 1-7**: `docs/dev/conventions/package_reference.md` の `internal/mergepr` の行の公開する名前に、このフェーズで公開する名前（`ParseDrop`・`Drops` など）を足す。行の役割の説明は、配線するステップ 2-6 で直す。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る。`internal/mergepr/mergepr.go` と `cmd/mergepr` を変えていない（`Tool` の振る舞いと `Protocol` は変わらない）。テストからだけ呼ばれる部品が `make deadcode`（`-test` 付き）に挙がらない。

### PR-1 作成ポイント: correction-line reader, classification, block, and checks in internal/mergepr

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0106): 訂正の行を読み、分類し、確かめる部品を mergepr に足す`

**レビュー観点**: コミットオブジェクトのバイト列を読み、再エンコードも置き換えのオブジェクトも通らないか / 受け付けない文字の分類の境界（タブ・CR・非文字・不正なバイト）が設計書 3.2 のとおりか / 確かめ 1〜7 とトレーラーの読み取りが設計書 3.4 のとおりで、利用者の設定に左右されないか / CLAUDE.md の例外の範囲と境界が、実装が中身を読む場所と合うか（例外の文は配線した後の形で書き、`paragraph`・`check-body` は PR-2 で入る。§1.3 の決定 4）

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-5 がマージの前の確かめ（パネルの対象の security-gate）を新しく作り、Sonnet ではバイト列の扱い（行末の CR、非文字、不正な UTF-8 の表示）や git の設定の遮断を取りこぼしやすい

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1029
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 配線、コマンドの定義と手引き、規則の文書と ADR（設計書 8 章の 1 の残り、2、3）

**変えるファイル:** `internal/mergepr/mergepr.go`、`internal/mergepr/test_helpers.go`、`internal/mergepr/mergepr_test.go`、`cmd/mergepr/main.go`、`cmd/mergepr/main_test.go`、`cmd/mergepr/docs_test.go`、`.claude/commands/mergepr.md`、`.claude/commands/runpr.md`、`.claude/commands/_context.md`、`docs/dev/guides/mergepr_guide.md`、`docs/dev/conventions/requirements_process.md`、`docs/dev/conventions/adr.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/adr/0039-adr-writing-criteria.md`、`docs/dev/adr/0036-task-document-status-records.md`、`docs/dev/adr/0047-runpr-merge-question.md`

-   [x] **ステップ 2-1**: `Tool` に配線する（設計書 3.5・3.6・3.7・6.2）。
    -   `Tool` に `correctionSource` を持たせ、`New` は本番の実装を入れる。`test_helpers.go` に偽物を足す。偽物は、`fakeChecker` と同じく呼ばれた時点の `fakeRunner` の位置を記録する。
    -   `test_helpers.go` の `fakeRunner` に、呼ばれた時点で引数とファイルを調べるフックを持つ手順を足す。`gh pr merge` の手順は、`--body-file` の値のパスを除いて引数を比べ、フックの中でそのファイルの中身・モード・場所を記録し、呼び出し側の本文のファイルを書き換える。
    -   `Prepare` は、作業ディレクトリを作る前に範囲の訂正の行を読む（読めなければ何も作らずに止まる）。`corrections.txt` を書き、`Prepared` に `Accepted` と `Rejected` を返す。拒否した行があっても止まらない。
    -   `Paragraph`・`CheckBody` を足す。どちらも `state.json` の head と base から範囲を求め直し、ネットワークに触れない。`Paragraph` が書く前の作業ディレクトリの確かめは、`removeWorkDir`（`internal/mergepr/mergepr.go:421-440`）の確かめを共有の関数に切り出して使う。
    -   `Merge` は設計書 6.2 の順序で動く。本文は最初に 1 回だけ読み、確かめたバイト列を `os.CreateTemp` の一時ファイル（作業ディレクトリの外）に書いて `--body-file` で渡し、どの道筋でも返る前に消す。片付けの後にマージの後の照合を行う。作業ディレクトリは、照合の結果によらず、今と同じく片付けが終われば消す（設計書 3.5 の「作業ディレクトリ」）。照合の誤りは、マージが済んでいて準備を捨てずマージをやり直さないことを示す（設計書 3.5 の「結果の示し方」）。`Cleanup` は任意の本文のパスを受け取る。
    -   `Report` に照合の結果を足す。`cleanupCommand` と `ShownCommands` は本文のパスを付けた `cleanup` を示す。`Protocol` を `origin/main` の値 + 1 にする。パッケージの説明（`internal/mergepr/mergepr.go:1-11`）の保証の文に、訂正の確かめとマージの後の照合を足す。
    -   `mergepr_test.go` の既存のテスト: `Merge` と `Cleanup` の引数の変更に、呼び出すテスト（§1.4）をすべて合わせる。`TestPrepare` に `corrections.txt` を足す。
    -   新しいテスト（`mergepr_test.go`）: `::TestMergePassesCheckedBody`（渡したファイルが確かめたバイト列で、モード 0600、作業ディレクトリの外にあり、`Merge` が返った後に消えている。`gh pr merge` が失敗する道筋でも消えている）、`::TestMergeStopsOnCorrectionCheck`（`gh pr merge` を呼ばず、確かめがプロトコル番号の確かめの後、番号の確かめの前に走る）、`::TestMergePostMergeCheck`（照合の相手が、フックが書き換えた後の本文のファイルではなく確かめたバイト列である。片付けの誤りが照合の誤りより先に並ぶ。照合が食い違っても、片付けが終わっていれば作業ディレクトリが消えている）、`::TestCleanupPostMergeCheck`（`--body-file` のときだけ照合する）、`::TestPrepareWritesCorrections`、`::TestParagraph`・`::TestCheckBody`（台本の手順が 0 の `fakeRunner` で、git と gh を呼ばない）、`::TestParagraphKeepsForgedWorkDirectory`（偽の親のディレクトリ、目印のないディレクトリ、無関係のディレクトリには書かない）。
    -   壊して確かめる: `Merge` の訂正の確かめを外す、本文を確かめの後に読み直す、照合で本文のファイルを読み直す、一時ファイルを消さない、`Paragraph` の作業ディレクトリの確かめを外す。
-   [x] **ステップ 2-2**: `cmd/mergepr` を変える（設計書 3.6・4.2）。
    -   `paragraph`・`check-body` のサブコマンドを足す。`--drop`・`--drop-rejected`（繰り返し）を `flag.Value` で集め、`cleanup --body-file` を解析する。形の誤りと重複（同じ番号で場合が違うものを含む）は、何も読まずに `errUsage` で止める。
    -   `prepare` の出力を書き先を受け取る関数に切り出し、`printReport` も書き先を受け取る形にする。設計書 3.6 の出力の行（`corrections:`・`correction rejected:`・`paragraph:`・`corrections carried:` と、`dir:` の行のファイルの並びへの `corrections.txt`）を足す。`errUsage` の文とパッケージの説明（`cmd/mergepr/main.go:1-8`）に、設計書 3.6 のサブコマンドの形を足す。
    -   `printError` は、`CorrectionMismatchError` と `CorrectionsNotCarriedError` を理由ごとに 1 行ずつ出す（設計書 4.2）。
    -   テスト（`main_test.go`）: `::TestParseRequiresProtocol` の表に、新しいサブコマンド・落とす指定・`cleanup --body-file` の行と、落とす指定の形の誤り・重複、`prepare`・`cleanup`・`discard` への `--drop`、`discard` への `--body-file` の行を足す。`::TestShownCommandsParse` は本文のパスを付けた `cleanup` を通す。`::TestPrintPrepared`（`correction rejected:` の行）、`::TestPrintReport`（`corrections carried:` の 2 つの形）、`::TestPrintErrorListsCorrectionMismatch` を足す。
    -   壊して確かめる: 重複の検出を外す、`printError` の理由ごとの行を外す、`corrections carried: not verified` の行を外す。
-   [x] **ステップ 2-3**: `.claude/commands/mergepr.md` の step 1〜4 を、設計書 3.8 の「`.claude/commands/mergepr.md`」のとおりに変える（AC-07・AC-08・AC-09・AC-10・AC-11・AC-13・AC-15・AC-17）。拒否した行の問いと、本文の組み立て（`paragraph` と `cat`）、`check-body` と `merge` の呼び出し、受け付ける修正と適用しない修正、止まったときの報告である。訂正の行の形は `requirements_process.md`「Recording in the PR description」を名指して参照し、書き写さない。呼び出しは §1.3 の決定 6 の形で書く。
-   [x] **ステップ 2-4**: `.claude/commands/runpr.md` を、設計書 3.8 の「`.claude/commands/runpr.md`」と §1.3 の決定 7・8 のとおりに変える。
    -   step 4 の item 2: 拒否した行の問い（`rejected-<round>.md`）と、本文の形ごとの `check-body`。問いごとの規則（問いの前の言及の確かめ、ラウンドと詳細ファイル、詳細ファイルを書けないときに準備を捨てて止まること、選択肢でない答えには何もせず新しいラウンドとして聞き直すこと）を、deferred の問いとマージの質問と同じく、この問いにも当てはめる（`.claude/commands/runpr.md:786-823` と、item 1 の答えの扱い）。
    -   item 3: 詳細ファイルの `C<番号>` と落とす指定、要約の `訂正の行:` の行と、要約の行の上限の文（「at most 13 lines」を 14 行に）。
    -   item 3 の自由記述: `C<番号>:<場合>` の答え。数字だけの答えは今のまま除外したスレッドの番号として読むこと（`.claude/commands/runpr.md:1140-1152`）。ほかの自由記述で本文を書き直したときは、新しいラウンドの前に本文の形ごとに `check-body` を走らせること。
    -   落とす指定を保つ期間: マージの質問を最初に聞いてから step 2 に戻るまで。PRE_MERGE の後の準備のやり直しの間も保ち、step 2 に戻ったら捨てる。
    -   item 4: 準備のやり直しの後、同じ落とす指定で `merge` に進む。
    -   item 5: 訂正の確かめで止まったときと、マージの後の照合が食い違ったときの報告。
    -   step 1・step 5 の `cleanup`（§1.3 の決定 8）。
    -   ほかのガードが読む `runpr.md` の見出しと `fixprguard` の呼び出し（`cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/fixprguard/docs_test.go`）は変えない。変えるときは、同じコミットでそれらのガードを直す。
-   [x] **ステップ 2-5**: 手引き `docs/dev/guides/mergepr_guide.md` と、文書のガード `cmd/mergepr/docs_test.go` を変える（設計書 3.7・3.9、1.2 の決定 9）。
    -   手引き: 3 章のプロトコル番号（地の文の番号を含む）、4 章の流れと図、5.2 の `paragraph`・`check-body`・落とす指定・`cleanup --body-file`・受け付けた行の番号の振り方、6.1 の保証、7 章の `correction check:` と `post-merge check:` で止まったときの対処を変える。
    -   `docs_test.go`: `definitionCalls` を設計書 3.7 のとおりにし、`placeholderValues` に `<drop>`・`<rejected-drop>` と、文書の呼び出しが新しく使う置き換えの名前をすべて足す（§1.3 の決定 6）。
-   [x] **ステップ 2-6**: 規則の文書を、設計書 3.9 の表のとおりに変える（AC-18）。対象は、`.claude/commands/_context.md` の「PR merge method」、`docs/dev/conventions/requirements_process.md` の記録の置き場所の段落と「Recording in the PR description」、`docs/dev/conventions/adr.md` の「Editorial corrections」の「Recording」である。あわせて、`docs/dev/conventions/package_reference.md` の `internal/mergepr` と `cmd/mergepr` の行に、訂正の行の引き継ぎと、このフェーズで公開する名前（`Paragraph`・`CheckBody` など）を足す。
-   [x] **ステップ 2-7**: ADR のずれの記述を消す（AC-16、設計書 3.9）。ADR-0039 の「決定」の項目（`:21`）の最後の 2 文を消し、仕様の置き場所のリンクを要件書から `mergepr.md` と手引きに変える。ADR-0036（`:25`）と ADR-0047（`:24`）の項目からも、最後のずれの記述を消す。どれも編集上の訂正なので、このコミットのメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`「Editorial corrections」）。§7 の AC-16 の diff 記録を、このコミットについて取る。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る。§4.4 の照合の項目を済ませた。

### PR-2 作成ポイント: wire the correction carry-over into mergepr, the merge procedures, and the rules

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0106): 訂正の行を squash の本文に引き継ぎ、マージの前後で確かめる`

**レビュー観点**: `merge` が確かめたバイト列だけを `--body-file` で渡し、確かめの順序が設計書 6.2 のとおりか / `runpr.md` の拒否した行の問い・`C<番号>:<場合>` の答え・止まったときの報告が、PRE_MERGE の commit と push の前後で設計書 3.8 のとおりか / `mergepr.md` が受け付ける修正と適用しない修正を設計書 3.8 のとおりに書き、訂正の行の形を書き写していないか / ADR のずれの記述の削除が、訂正の行とともに記録されているか

**実装モデル要件**: opus

**判定理由**: ステップ 2-1・2-4 がマージの手順（パネルの対象の security-gate）と `/runpr` のマージの質問を同時に変え、プロトコル番号も上げるので、Sonnet では `runpr.md` の止まる場所ごとの準備の捨て方や、照合の相手を確かめたバイト列に限ることを取り違えやすい

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1036
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 確かめ |
|---|---|---|
| M1（PR-1） | 範囲の列挙の共有、訂正の行の読み取り・分類・ブロック・確かめ・照合の部品、`CLAUDE.md` の例外 | `make test`・`make lint`。道具の振る舞いは変わらない |
| M2（PR-2） | 配線、`Protocol` の引き上げ、コマンドの定義・手引き・ガード、規則の文書、ADR のずれの記述の削除 | `make test`・`make lint`、§7 の AC-16 の diff 記録、PR-2 のマージでの実地の確かめ（§9） |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | `internal/mergepr` の部品とテスト、`CLAUDE.md` の例外 | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 配線、`Protocol`、`cmd/mergepr`、`mergepr.md`・`runpr.md`・手引き・ガード、規則の文書、ADR | opus | auto |

PR-2 は `Protocol` を上げるので、PR-2 の `/runpr` の前に、PR-2 のブランチで `mergepr` を入れ直す（`docs/dev/guides/mergepr_guide.md` 3 章、設計書 7.3）。これは `/runplan` の後の作業なので、PR-2 の自動送信には影響しない。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   **実際の git:** 読み取り（ステップ 1-1・1-2）とトレーラーの振る舞い（ステップ 1-5）は、`internal/mergepr/test_helpers_git.go` の一時リポジトリで確かめる。リポジトリは `t.TempDir` の下に置き、テストは `HOME` と `XDG_CONFIG_HOME` を空のディレクトリに向けるので、繰り返して走らせても互いに影響しない。トレーラーの読み取りが利用者の設定を読まないことは、そのテストの中で `HOME` の `.gitconfig` に設定を書いて確かめる。
-   **表のテスト:** 分類・ブロック・確かめは、設計書 7.1 の各表の行を、止まる側と通る側の両方から確かめる。同じ行を 2 つの表に置かない。
-   **`Tool` のテスト:** `fakeRunner` の git と gh のコマンドの並びは、`gh pr merge` の引数を除いて変えない。`gh pr merge` の手順は、フックで渡したファイルを調べる（ステップ 2-1）。訂正の行の読み取りは偽物の `correctionSource` で差し替え、偽物は呼ばれた位置を記録する（設計書 3.7）。
-   **git の版:** トレーラーと再エンコードの振る舞いは git 2.50.1 で確かめた。CI の git でも同じテストが通ることを、PR-1 の CI で確かめる。
-   **`cmd/mergepr`:** 引数の組と、出力の行を確かめる。

### 4.2. 文書のガード

`cmd/mergepr/docs_test.go` の 3 つのテストは、表を足すだけで仕組みは変えない。文の意味（拒否した行の問い、受け付ける修正と適用しない修正、規則の文書の文）は、PR のレビューで読む（ADR-0016）。

### 4.3. 互換性

-   引き継ぐ範囲に訂正の行がない PR の squash の本文は、今と同じになる（AC-06）。`paragraph.txt` が 0 バイトのときに本文が変わらないことを、ステップ 1-4 の表のテストで確かめる。
-   プロトコル番号を上げるので、古い `mergepr` は新しい定義を拒み、新しい `mergepr` は古い定義を拒む。入れ替えの手順は手引き 3 章にある。

### 4.4. 照合の項目（`make test` と `make lint` が見ないもの）

-   [x] 手引き 3 章の地の文の番号（`docs/dev/guides/mergepr_guide.md:48` の「現在の番号は `2`」）を、新しい番号に直した。
-   [x] `runpr.md` と `mergepr.md` の、準備を捨てる・捨てないの指示が、`correction check:`（食い違いと読めないこと）と `post-merge check:` のそれぞれで、設計書 3.5 と 4.2 と合っている。
-   [x] 拒否した行の問いの文が、chief の `mergeQuestionText`（`internal/chief/screen.go:113`）にも `mergeQuestionSqueezed`（`:117`）にも合わない。
-   [x] マージの質問の要約に `訂正の行:` の行を足しても、chief の画面の読み取り（`internal/chief/screen*.go`）が問いと選択肢を今までどおり読める。
-   [x] ステップ 2-4 の小項目（自由記述の 3 つの扱い、落とす指定を保つ期間を含む）が、設計書 3.8 の `runpr.md` の項目をどれも漏らさず含む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| トレーラーの読み取りの環境の作り方によっては、git の版しだいで利用者の設定を読んでしまう | `GIT_CONFIG_GLOBAL` に加えて `HOME`・`XDG_CONFIG_HOME` を一時ディレクトリに向ける（§1.3 の決定 2）。テストで `HOME` の設定が効かないことを確かめる |
| PR-2 の `Protocol` の番号が、ほかの PR と重なる | `CLAUDE.md`「Protocol numbers」のとおり、実装の時点の `origin/main` の値 + 1 にし、`mergepr` の確かめに任せる |
| `runpr.md` の改訂が大きく、止まる場所ごとの扱いを取りこぼす | 設計書 3.8 の項目ごとにステップ 2-4 の小項目を分け、§4.4 の照合の項目で読み合わせる |
| GitHub がマージの本文のバイト列を変える | マージの後の照合で検出し、報告する（設計書 3.5）。PR-2 のマージで実地に確かめる（§9） |
| 0102 が同じ時期に `CLAUDE.md` の例外を足し、競合する | 「Declare, don't infer」の例外の並びに独立した段落として足す。競合したら `main` を取り込んで両方を残す |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前は、`path::TestName` の形で書く。名前は実装で変えてよいが、変えたときはこの表を同じコミットで直す。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | test | `internal/mergepr/corrections_test.go::TestRangeLines`（コミットオブジェクトのバイト列）、`::TestMergedLines`、`::TestCorrectionBlock`、`::TestCheckCorrections`、`::TestCompareMergedLines`、`internal/mergepr/mergepr_test.go::TestMergePassesCheckedBody`、`::TestMergePostMergeCheck`、`cmd/mergepr/main_test.go::TestPrintReport` | 1-2・1-4・1-5・2-1・2-2 |
| AC-01 | manual | PR-2 のマージで、`corrections carried:` の行と `git cat-file commit <squash コミット> \| grep '^Editorial correction: '` の出力を確かめ、PR に記録する（設計書 7.3、§9） | 2-1 |
| AC-02 | test | `internal/mergepr/corrections_test.go::TestRangeLines`（パスの種類と形を問わない）、`::TestClassifyCorrectionLines`（受け付ける側） | 1-2・1-3 |
| AC-03 | test | `internal/mergepr/corrections_test.go::TestRangeLines`（範囲の順）、`::TestClassifyCorrectionLines`（初めて現れた位置にまとめる） | 1-2・1-3 |
| AC-04 | test | `internal/mergepr/corrections_test.go::TestRangeLines`（字下げ・大文字と小文字・文の途中） | 1-2 |
| AC-05 | test | `internal/mergepr/corrections_test.go::TestCorrectionBlock`（上限なし）、`internal/mergepr/mergepr_test.go::TestMergePassesCheckedBody`（`--body-file`） | 1-4・2-1 |
| AC-05 | manual | `mergepr.md` step 2 が、訂正の行を、本文を短く保つ規則の例外として書いていることを、PR-2 のレビューで読む | 2-3 |
| AC-06 | test | `internal/mergepr/corrections_test.go::TestCorrectionBlock`（空のブロックは 0 バイト）、`::TestCheckCorrections`（確かめ 6・7。実際の git）、`::TestTrailerReading`、`::TestTrailerReadingIgnoresConfig` | 1-2・1-4・1-5 |
| AC-06 | manual | `mergepr.md` と `runpr.md` が、訂正のブロックを `Closes` の行とトレーラーの前に置く本文の組み立てを書くことを、PR-2 のレビューで読む。PR-2 のマージで、squash コミットに `git interpret-trailers --parse` を実行し、予約した接頭辞の行が出ないことを確かめる（§9） | 2-3・2-4 |
| AC-07 | test | `internal/mergepr/corrections_test.go::TestCheckCorrections`（拒否した行が本文にあれば余分な行）、`internal/mergepr/mergepr_test.go::TestCheckBody` | 1-5・2-1 |
| AC-07 | static | `cmd/mergepr/docs_test.go::TestDocumentedInvocationsParse`（`mergepr.md` と `runpr.md` が `check-body` を呼ぶ） | 2-5 |
| AC-07 | manual | `runpr.md` が本文の形ごとに `check-body` を走らせることを、PR-2 のレビューで読む | 2-4 |
| AC-08 | test | `internal/mergepr/corrections_test.go::TestCheckCorrections`（欠けた行・余分な行・順序の違いで止まる） | 1-5 |
| AC-08 | manual | `mergepr.md` step 3 の、受け付ける修正と適用しない修正と、人に伝える理由を、PR-2 のレビューで読む | 2-3 |
| AC-09 | test | `internal/mergepr/corrections_test.go::TestCheckCorrections`、`internal/mergepr/mergepr_test.go::TestCheckBody`、`::TestMergeStopsOnCorrectionCheck`（`gh pr merge` を呼ばない）、`cmd/mergepr/main_test.go::TestPrintErrorListsCorrectionMismatch` | 1-5・2-1・2-2 |
| AC-09 | manual | `mergepr.md` と `runpr.md` の、止まったときの報告と、PRE_MERGE の前後での扱いを、PR-2 のレビューで読む | 2-3・2-4 |
| AC-10 | test | `internal/mergepr/corrections_test.go::TestClassifyCorrectionLines`、`::TestShownForm`、`internal/mergepr/mergepr_test.go::TestPrepareWritesCorrections`、`cmd/mergepr/main_test.go::TestPrintPrepared` | 1-3・2-1・2-2 |
| AC-11 | test | `internal/mergepr/corrections_test.go::TestCheckCorrections`（宣言のない拒否した行で止まる）、`internal/mergepr/mergepr_test.go::TestParagraph`、`::TestMergeStopsOnCorrectionCheck` | 1-5・2-1 |
| AC-11 | manual | `mergepr.md` と `runpr.md` の拒否した行の問いと 2 つの直し方を、PR-2 のレビューで読む | 2-3・2-4 |
| AC-12 | test | `internal/mergepr/corrections_test.go::TestClassifyCorrectionLines`（タブを受け付ける）、`::TestRangeLines`（訂正の行でない行は読まない） | 1-2・1-3 |
| AC-13 | test | `internal/mergepr/corrections_test.go::TestCorrectionBlock`（同じバイト列の行が一緒に落ちる）、`::TestParseDrop`、`::TestParseRejectedDrop`、`cmd/mergepr/main_test.go::TestParseRequiresProtocol` | 1-4・2-2 |
| AC-13 | manual | `runpr.md` の `C<番号>:<場合>` の答えの形を、PR-2 のレビューで読む | 2-4 |
| AC-14 | test | `internal/mergepr/corrections_test.go::TestCorrectionBlock`（落とした記録の行の件数と、落とさないときにないこと） | 1-4 |
| AC-15 | test | `internal/mergepr/corrections_test.go::TestCheckCorrections`（宣言のない行の欠けで止まる） | 1-5 |
| AC-15 | manual | `mergepr.md` step 3 が、3 つの場合に当たらない理由で落とす修正を適用しないことを、PR-2 のレビューで読む | 2-3 |
| AC-16 | diff | 記録した diff の読み取り。ずれの記述を消す前と後の関係を見る。比べるパスは `docs/dev/adr/0039-adr-writing-criteria.md`、`docs/dev/adr/0036-task-document-status-records.md`、`docs/dev/adr/0047-runpr-merge-question.md`（下の記録） | 2-7 |
| AC-16 | manual | ADR-0039 のリンクの向け先が `mergepr.md` と手引きであることを、PR-2 のレビューで読む | 2-7 |
| AC-17 | static | `cmd/mergepr/docs_test.go::TestDocumentedInvocationsParse`（`mergepr.md` が `paragraph`・`check-body`・`merge` を呼び、どの呼び出しも引数の確かめを通る） | 2-5 |
| AC-17 | manual | `mergepr.md` が F-001〜F-004 を述べ、訂正の行の引き継ぎを、本文を短く保つ規則の例外として書いていることを、PR-2 のレビューで読む | 2-3 |
| AC-18 | manual | `_context.md`・`requirements_process.md`・`adr.md` の文を、PR-2 のレビューで読む（文の中身だけを定める AC） | 2-6 |
| AC-19 | static | `make test` と `make lint`。そのうち `cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`・`::TestDocumentedInvocationsParse`・`::TestProtocolToolsAgree` | 1-1〜2-7 |

Diff record (AC-16):

- Compared paths: `docs/dev/adr/0039-adr-writing-criteria.md` `docs/dev/adr/0036-task-document-status-records.md` `docs/dev/adr/0047-runpr-merge-question.md`
- Merge-base: `1b8da1cf87623f80ae0c2a54f5fdcf9d25f50559`
- Compared commit: `aa9f0c94d344149b9417b2741a1b3aa7ca6868cf`

Path diff:

````text
diff --git a/docs/dev/adr/0036-task-document-status-records.md b/docs/dev/adr/0036-task-document-status-records.md
index 108be06b6dba1e354d7a1759620c323f673720ff..4c1530d286f7f65c419abf0674eca21e690e405c 100644
--- a/docs/dev/adr/0036-task-document-status-records.md
+++ b/docs/dev/adr/0036-task-document-status-records.md
@@ -22,7 +22,7 @@
 -   **状態の遷移は、文書の末尾の変更履歴に 1 件 1 行で追記する。** 変更履歴には編集上の訂正を書かない。各行は、原則としてその行を加えた PR の番号を持ち、PR を作る前に書いた行の番号は `/runpr` がマージの直前に埋める（[runpr.md](../../../.claude/commands/runpr.md)）。行の形と PR 番号の例外は、[requirements_process.md](../conventions/requirements_process.md)「Change history」に従う。状態の遷移を `Comments` に積むと、今の状態の理由が過去の行に埋もれるためである。
 -   **変更履歴のない文書は、最初の状態の遷移を書くときに移す。** 旧 `Comments` を変更履歴の先頭の項目にし、承認済みの文書なら状態表から承認の行を写す。写せないときは、遷移を書かずに止まる。一括では移さない。対象と手順は、[requirements_process.md](../conventions/requirements_process.md)「Documents without a change history」に従う。一括で移すと、それ自体が決定の変更になって、新たに決める内容のない再承認が必要になるためである（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md)「要件定義の際に決めたこと（2026-11-02）」）。
 -   **編集上の訂正と決定の変更は、PR の説明にも書く。** 編集上の訂正はコミットのメッセージに訂正の行として書き、PR の説明の訂正の段落はその行から作る（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B、[fixpr.md](../../../.claude/commands/fixpr.md)）。形は、[requirements_process.md](../conventions/requirements_process.md)「Recording in the PR description」に従う。`/fixpr` が PR の説明に追記する道具と、その文面を信頼しないデータとして扱うことは、[ADR-0015](0015-untrusted-review-threads.md) が定める。スレッドへの返信やコミットメッセージだけに残すと、訂正の説明が PR の説明にあることを満たさないためである。
--   **`/mergepr` で squash マージするときは、タスク文書の訂正の行も、ADR の訂正の行と同じく squash コミットの本文に引き継ぐ。** 引き継ぎ方は [ADR-0039](0039-adr-writing-criteria.md) の「決定」に従う。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるためである。今の `/mergepr` の手順と規約は、まだ訂正の行を squash コミットの本文に引き継がない。このずれは、[#747](https://github.com/isseis/yt2column/issues/747) で扱う。
+-   **`/mergepr` で squash マージするときは、タスク文書の訂正の行も、ADR の訂正の行と同じく squash コミットの本文に引き継ぐ。** 引き継ぎ方は [ADR-0039](0039-adr-writing-criteria.md) の「決定」に従う。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるためである。
 -   **書式は規則の 1 か所で定め、コマンドの定義は規則の節を名指しして参照する。** `Comments` の値、変更履歴の行の形、訂正の行と訂正の段落の形は、[requirements_process.md](../conventions/requirements_process.md) だけに書く。[runpr.md](../../../.claude/commands/runpr.md)・[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md)・[runplan.md](../../../.claude/commands/runplan.md)・[fixpr.md](../../../.claude/commands/fixpr.md) などのコマンドの定義には、いつどの文書に書くかだけを書く。理由は記録されていない。

 ## 採らなかった案
diff --git a/docs/dev/adr/0039-adr-writing-criteria.md b/docs/dev/adr/0039-adr-writing-criteria.md
index cd978c78bd1ddd03f76f88f58b90e6e7debe7774..fb0000d865c2bebaddd124e62769cc39c6bd8c7b 100644
--- a/docs/dev/adr/0039-adr-writing-criteria.md
+++ b/docs/dev/adr/0039-adr-writing-criteria.md
@@ -18,7 +18,7 @@ ADR は、後の開発者が、変えようとしている主題の決定とそ
 -   **「背景」には、決定が答えている問題と、決定を縛る前提だけを書く。** 決定の根拠そのものである数字は残す。前の決定は「経緯」に、退けた形は「採らなかった案」にあり、「背景」に重ねると今の決定が読み取りにくくなるためである。規則は [adr.md](../conventions/adr.md) の「Writing an ADR」にある。
 -   **「決定」には、後の変更が守るべき規則を 1 項目ずつ、短い理由と、正本の文書かコードの場所とともに書く。** 正本の文書の一覧と腐りやすい実装の細部は書かない。正本とのずれの記述と、その issue へのリンクは残す。ADR を、決定が変わらない限り古くならない文書にするためである。規則は [adr.md](../conventions/adr.md) の「Writing an ADR」にある。
 -   **ADR を作るか変えるのは、その決定を下すか変える PR である。ただし、決定を下しも変えもしない PR も、ADR に編集上の訂正を加えてよい。** 訂正は、決定を変えない決まった種類の直しに限る。訂正はコミットメッセージと PR の説明に記録し、「経緯」には項目を足さない。タスク文書の編集上の訂正（[requirements_process.md](../conventions/requirements_process.md) の「Editing an approved document」）と記録の置き場所をそろえ、例外を作らずに ADR を直せるようにするためである。直した結果が今の決定と食い違う変更と、訂正か迷う変更は、決定の変更として扱う。記録を失わない側に倒すためである。PR を作った後のレビューを受けた修正でも、ADR に訂正を加えてよい。訂正の行と訂正の段落の形は、タスク文書と ADR の両方について 1 か所で定める。コマンドの手順も、タスク文書の訂正と同じく ADR の訂正を記録して PR の説明へ運ぶ。このため、PR を作った後の訂正でも、レビュアーが分類を確かめられる。ただし、PR を作った後に `main` を取り込むときは、ADR に訂正を加えない。その経路には訂正を PR の説明へ運ぶ手順がないためである。訂正の行と訂正の段落の形は [requirements_process.md](../conventions/requirements_process.md) の「Recording in the PR description」にある。レビュアーが分類に異を唱えた変更は決定の変更として扱い、タスク文書のない PR では、決定を PR の説明に記録し、「経緯」の項目からその PR へリンクする。規則は [adr.md](../conventions/adr.md) の「Approval and the PR」「Editorial corrections」「History entries」にある。
--   **`/mergepr` で squash マージするときは、PR のコミットにある訂正の行を、タスク文書と ADR のどちらのものも、要約せず 1 行ずつそのまま squash コミットの本文に引き継ぐ。** 行数にも 1 行の長さにも上限を設けない。人は承認のときに訂正の行を消せず、要件定義書が定める場合に限って落とせる。落としたときは、落としたことを本文に記録する。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるので、訂正の記録を `main` の履歴からたどれるようにするためである。行を受け付ける条件と落とし方の仕様は、[0106 要件定義書](../../tasks/0106_squash_editorial_corrections/01_requirements.md) の「3. 機能要件」にある。今の `/mergepr` の手順と規約は、まだ訂正の行を squash コミットの本文に引き継がない。このずれは、[#747](https://github.com/isseis/yt2column/issues/747) で扱う。
+-   **`/mergepr` で squash マージするときは、PR のコミットにある訂正の行を、タスク文書と ADR のどちらのものも、要約せず 1 行ずつそのまま squash コミットの本文に引き継ぐ。** 行数にも 1 行の長さにも上限を設けない。人は承認のときに訂正の行を消せず、決まった 3 つの場合に限って落とせる。落としたときは、落としたことを本文に記録する。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるので、訂正の記録を `main` の履歴からたどれるようにするためである。行を受け付ける条件と落とし方の仕様は、[mergepr.md](../../../.claude/commands/mergepr.md) と [mergepr の手引き](../guides/mergepr_guide.md) の「6.5 訂正の行の引き継ぎ」にある。
 -   **ずれの記述は、ずれを直す PR が同じ PR で消す。** ずれの記述が事実でない期間を作らないためである。ずれを直す人が記述を見つけられるように、issue から ADR を、ADR から issue をたどれるようにする。ずれの記述を足す PR は、マージを頼む前と `main` を取り込むときに、最新の `main` に照らして、そのずれの記述がまだ事実かどうかを確かめ直す。規則は [adr.md](../conventions/adr.md) の「Writing an ADR」「Editorial corrections」にある。
 -   **`main` の上で `TestADRRecords` が失敗したときの、番号を付け替える直しは、その直しだけの PR で行う。** 番号の付け替えは ADR の識別子を変え、ほかの文書の参照にも及ぶ。関係のない PR のレビュアーは、これを承認する立場にないためである。規則は [adr.md](../conventions/adr.md) の「Taking in `main`」にある。
 -   **ADR の書式（太字の使い方と、項目の多い節の組み立て方）は、日本語の文書の書式の規約に従う。既存の ADR を書式の規約に合わせる書き直しは求めないが、編集上の訂正の 1 種目（基準に合わせる書き直し）として、どの PR も行ってよい。** 書式の規則を、ADR に限らず日本語の文書で 1 つにするためである。書き直しを訂正として認めるのは、訂正がコミットと PR の説明に記録され、レビュアーが確かめられるためである。規則は [書式の規約](../conventions/document_formatting.md) と [adr.md](../conventions/adr.md) の「Writing an ADR」にある。
diff --git a/docs/dev/adr/0047-runpr-merge-question.md b/docs/dev/adr/0047-runpr-merge-question.md
index 4f52b3e3ff631a6e9a3d18f2c27405ba4ff0ad3b..0d53d8fff76715647db828a7bcbfa8d0e4dbd008 100644
--- a/docs/dev/adr/0047-runpr-merge-question.md
+++ b/docs/dev/adr/0047-runpr-merge-question.md
@@ -21,7 +21,7 @@
 -   **マージの質問に、NEXT をその `/runpr` のセッションで実行する選択肢を置かない。`/runpr` は NEXT を示すだけで、実行しない。** 次のコマンドは chief が送るためである（ADR-0046）。step 5 が NEXT の前に `/clear` を勧めていることとも食い違っていた（F-003）。
 -   **承認の判断に要る詳細は、マージの質問のたびに作業ツリーの外の詳細ファイルに書き直し、会話には 1 項目 1 行の要約と、そのファイルのパスだけを出す。question の文は 1 行にし、長い本文を question の欄にも選択肢にも入れない。** 本文が多い PR でも、要約と選択肢を画面に収めるためである。問いのたびに書き直すのは、古い中身で承認しないためである（F-002）。
 -   **`/runpr` の中では、squash commit の件名を要約に、本文と接頭辞を決めた規則の記録を詳細ファイルに示して承認を尋ねる。単独で実行する `/mergepr` は、件名と本文の全文を会話に示す。** 本文は長くなりやすく、要約を画面に収めるためである。単独の `/mergepr` は、承認に要る項目だけを示すので画面に収まる（F-004）。
--   **PR のコミットの訂正の行に受け付けない文字を含む行があるときは、マージの質問の前に、すべてのその行について 1 つの単一選択の問いで、すべて落とすか、ブランチで直すために止まるかを人が選ぶ。行ごとの報告は詳細ファイルに書く。マージの質問では、訂正の行を落とす答えを `C<番号>:<場合>` の形の自由記述で受け付け、訂正の行があるときだけ要約に訂正の行の行を出す。** 受け付けない文字を含む行を `main` に写さないことと、落とす場合を限ることは [ADR-0039](0039-adr-writing-criteria.md) の決定で、どの場合を落とせるかは [0106 要件定義書](../../tasks/0106_squash_editorial_corrections/01_requirements.md) の F-004 が持つ。ブランチで直すときは行によらず `/mergepr` をやり直すので、行ごとに問うても選べる組み合わせは増えない。単一選択にするのは、deferred のスレッドの問いと同じく、chief が中継するのが 1 つを選ぶ問いだけだからである（ADR-0046）。`C` を付けるのは、除外スレッドの判定を覆す数字だけの答えと見分けるためである（[0106 設計書](../../tasks/0106_squash_editorial_corrections/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-19）」）。今の [runpr.md](../../../.claude/commands/runpr.md) と [mergepr.md](../../../.claude/commands/mergepr.md) は、まだこの問いと答えの形を持たない。このずれは、[#747](https://github.com/isseis/yt2column/issues/747) で扱う。
+-   **PR のコミットの訂正の行に受け付けない文字を含む行があるときは、マージの質問の前に、すべてのその行について 1 つの単一選択の問いで、すべて落とすか、ブランチで直すために止まるかを人が選ぶ。行ごとの報告は詳細ファイルに書く。マージの質問では、訂正の行を落とす答えを `C<番号>:<場合>` の形の自由記述で受け付け、訂正の行があるときだけ要約に訂正の行の行を出す。** 受け付けない文字を含む行を `main` に写さないことと、落とす場合を限ることは [ADR-0039](0039-adr-writing-criteria.md) の決定で、どの場合を落とせるかは [0106 要件定義書](../../tasks/0106_squash_editorial_corrections/01_requirements.md) の F-004 が持つ。ブランチで直すときは行によらず `/mergepr` をやり直すので、行ごとに問うても選べる組み合わせは増えない。単一選択にするのは、deferred のスレッドの問いと同じく、chief が中継するのが 1 つを選ぶ問いだけだからである（ADR-0046）。`C` を付けるのは、除外スレッドの判定を覆す数字だけの答えと見分けるためである（[0106 設計書](../../tasks/0106_squash_editorial_corrections/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-19）」）。
 -   **3 文書の PR の「通常の進め方へ戻す」と除外スレッドは、マージの質問で示す。** [ADR-0040](0040-simple-task-documents.md) の決定を保つためである。deferred の問いからも戻せるようにしたことで、ADR-0040 の戻す入口が 1 つ増えた。除外スレッドの判定を覆すスレッドは、番号を自由記述で指定する（F-003）。

 ## 採らなかった案
````

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 のすべての行の検証が通り、PR-2 のマージの squash コミットが、その PR の訂正の行を持つ（§9）。
-   **品質:** 新しいテストは、どれも確かめる仕組みを壊して失敗することを確かめ、そのことがコミットのメッセージにある。
-   **セキュリティ:** 受け付けない文字を含む行が、宣言なしに本文に入らない。本文と件名の予約した接頭辞の行は、訂正のブロックとバイト列として一致するときだけマージされる。
-   **文書:** 手引き・コマンドの定義・規則の文書・ADR が、§7 の manual の行のとおりに読める。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの後に `main` から `mergepr` を入れ直す（振る舞いは変わらないが、手引き 3 章の手順に合わせる）。
-   PR-2 の `/runpr` の前に、PR-2 のブランチで `mergepr` を入れ直す。マージの直後に `main` から入れ直す（手引き 3 章）。
-   PR-2 のマージで、`mergepr` の報告の `corrections carried:` の行と、`git cat-file commit <squash コミット> | grep '^Editorial correction: '` の出力が、PR-2 のコミットの訂正の行（ステップ 2-7 の行を含む）と一致することを確かめる。squash コミットに `git interpret-trailers --parse` を実行し、予約した接頭辞の行が出ないことも確かめる。結果を PR-2 にコメントとして記録する（設計書 7.3、AC-01・AC-06 の manual の行）。
-   PR-2 のマージの後、`/runpr` が issue #747 を閉じる（タスクの最後の計画の PR）。

## 変更履歴

- 2026-11-19 承認（isseis、#1018）
