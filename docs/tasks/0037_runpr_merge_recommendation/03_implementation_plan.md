# 実装計画書：/runpr のマージの質問で、次のコマンドを続けて実行する選択肢を推奨にしない

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-003 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。変えるのは、`.claude/commands/` の下のコマンドの定義（`runpr.md`・`_lib/task-pr-handoff.md`・`mergepr.md`・`_context.md`）と、`CLAUDE.md`・`docs/dev/developer_guide/requirements_process.md` の記述だけである。Go のコード・テスト・拡張のコードは変えない（設計書 1.1・3.8）。このほかに、本書の検証の成果物として、検査のスクリプト `docs/tasks/0037_runpr_merge_recommendation/check_wording.sh` を新しく作る（1.4）。

設計書 8 章の 5 つの項目を、順にフェーズ 1〜5 とする。すべてのフェーズを 1 つの PR にまとめる（設計書 8 章）。

用語は、要件書 6 章と設計書の冒頭の定義に従う（マージの質問、PRE_MERGE、マージだけの選択肢、続けて実行する選択肢、最終報告、NEXT、実装モデル要件、次のブランチ、次のブランチを作り終えた、`/clear` の案内、再開の方法、次の PR マーカー）。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   変える文書の文は英語で書く（要件書 AC-14、`CLAUDE.md`「Language」「Documents」）。具体的な英語の文は、実装で決める（設計書 3 章の冒頭）。
-   「Merge」の選択肢の推奨は、`runpr.md` の step 4 の選択肢の一覧にだけ書く。ほかの文書は推奨に触れない（設計書 1.1）。
-   検査のスクリプトは、変更前のコミット（`a4aa70a`）のファイルに対して実行したときに失敗することを確かめてから、コミットする。確かめ方と結果は、そのコミットのメッセージに書く（`CLAUDE.md`「Testing Strategy」）。

### 1.3. 既存コード調査結果

調査はコミット `a4aa70a` で行った。設計書が確かめたコミット `e529b68` から、`.claude/`・`CLAUDE.md`・`docs/dev/`・`cmd/`・`internal/`・`extension/` に変更はない（`git diff --stat e529b68 a4aa70a -- .claude CLAUDE.md docs/dev cmd internal extension` の出力が空）。そのため、設計書の `file:line` は `a4aa70a` でもそのまま使える。以下の `file:line` も `a4aa70a` のものである。

#### 1.3.1. 変える箇所の一覧

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `.claude/commands/runpr.md:357-368` | 「Merge」の選択肢の一覧。先頭が続けて実行する選択肢で「(recommended; only when NEXT is set)」（`:359-360`）、2 番目がマージだけの選択肢（`:361-362`） | 設計書 3.1 の表の順序と推奨にする | 1-1 |
| `runpr.md:345-347` | NEXT を示す箇所。「the next PR marker's `実装モデル要件`, so the user can decide whether this session's model fits」 | 設計書 3.2 のとおり、次の PR マーカーの定義、値がないときの扱い、値を示す理由を改める | 1-1 |
| `runpr.md:472-474` | step 5 の最後の項目。続けて実行する選択肢なら NEXT を実行し、そうでなければ「report the final status」 | 設計書 3.3 の表の場合分けと、`/clear` の案内の 4 項目にする | 2-1 |
| `runpr.md:445-458` | step 5 の片付け・`git fetch`・次のブランチの作成・detach。止まったときの報告を定めていない | 設計書 3.4 の、止まったときの報告の 4 項目と再開の方法を加える | 2-2 |
| `runpr.md:50-69` | Authorization の段落。`git ls-remote` は `fixprguard` が行う操作としてだけ挙がっている（`:58`）。`git fetch` は `origin main` だけを許す | 次のブランチが `origin` にあるかを確かめる読み取りの操作（設計書 4.3 の `git ls-remote`）を、step 5 の操作として加える。`fixpr_docs_test.go:658-660` が丸ごと固定する `fixprguard` の操作の節と、`:66` の文は変えない | 2-2 |
| `runpr.md:2` | `description` の「..., merge it, and run the next command」 | 設計書 3.5 の表の 1 行目 | 3-1 |
| `runpr.md:30-31` | 冒頭の「... the bookkeeping, the merge, and the next command, and carry it out.」 | 設計書 3.5 の表の 2 行目 | 3-1 |
| `runpr.md:443` | 見出し「## 5. Switch branch, then run NEXT」 | 設計書 3.5 の表の 3 行目 | 3-1 |
| `CLAUDE.md:86` | `/runpr` の行の「..., merge it, and run the next command; ...」 | 設計書 3.6 の表の 1 行目 | 4-1 |
| `CLAUDE.md:98-102` | 「... merge with `/mergepr`, and run the next command, which opens the next PR and stops again.」（`:101`） | 設計書 3.6 の表の 2 行目 | 4-1 |
| `.claude/commands/_lib/task-pr-handoff.md:19-23` | 「... and the next command to run (`runpr.md` steps 2 and 4).」（`:22-23` で行をまたぐ） | 設計書 3.6 の表の 3 行目 | 4-2 |
| `docs/dev/developer_guide/requirements_process.md:55` | 「... approve the document, merge, and run the next command. On that answer it commits the approval, merges, and starts the next command ...」 | 設計書 3.6 の表の 4 行目 | 4-3 |
| `.claude/commands/mergepr.md:122-125` | 「... its step 4 asks this question together with the pre-merge bookkeeping and the next command, ...」（`:124`） | 設計書 3.6 の表の 5 行目 | 4-4 |
| `.claude/commands/_context.md:63` | 「... and shown in the merge question when `/runpr` offers the next `/runplan`」 | 設計書 3.6 の表の 6 行目 | 4-5 |

#### 1.3.2. 変えない箇所

次の記述は、`/runpr` のマージの後の振る舞いを述べるが、NEXT を必ず実行するとは読めないので変えない（設計書 2.1・3.5・3.6）。`a4aa70a` で読んで確かめた。

-   `runpr.md:66`（Authorization の「the next command (NEXT) when the user chooses to run it in step 4」）。同じ段落に加える操作（1.3.1）は、この文とは別に書く。
-   `runpr.md:147`（step 2 の見出し「Decide the PR's bookkeeping and next command」）。NEXT を決めるだけで、実行するとは述べない。
-   `.claude/commands/mkplan2.md:107`、`.claude/commands/runplan.md:111-115`、`.claude/commands/mkreq.md:15-17`、`.claude/commands/mergepr.md:140-144`。
-   `requirements_process.md:41-53` の「Review Flow」の図（設計書 3.6）。

#### 1.3.3. 既存のテストへの影響

-   `cmd/yt2column/fixpr_docs_test.go:36-38` は、`runpr.md` の step 1・step 3・step 4 の見出しを定数として読む。step 4 の見出し（`runpr.md:328`）は変えない。step 5 の見出しは、どのテストも読まない。
-   step 4 の節の中では、`fixpr_docs_test.go:318-320` が「If a new thread appeared, an untrusted one included, do not merge」（`runpr.md` step 4 の 3）を、`:651-660` が Authorization の段落を読む。どちらも本タスクが変える箇所ではない。
-   `git grep -n -i -e 'run the next command' -e 'carry it out' -e 'starts the next command' -e 'run NEXT' -e 'recommended' -e 'next command to run' -e 'shown in the merge question' -- '*_test.go' 'extension/*.ts'` は、`a4aa70a` で一致がない（終了コード 1）。本タスクで直す既存のテストはない（設計書 1.4）。
-   Authorization の段落は、`fixpr_docs_test.go:651-660` の `TestAuthorizationSections` が、`fixprguard` の操作の節を丸ごと固定している。ステップ 2-2 で操作を加えるときは、この節を書き換えない。
-   タスク 0032 のコミット済みのスクリプト `docs/tasks/0032_japanese_pr_title/check_rules.sh` は、本タスクが書き換える範囲を読む。`check_runpr_title` は `runpr.md` の step 4 の 2（`^2\. Ask with one` から `^3\. ` まで。ステップ 1-1 の範囲を含む）を、`check_mergepr_subject` は `mergepr.md` の step 3（ステップ 4-4 の範囲の隣）を、`check_context_marker_conventions` は `_context.md` の「PR marker conventions」（ステップ 4-5）を読み、`no_stale_rules` はすべてのファイルを読む。CI では実行されない。`a4aa70a` で実行すると、すべての検査が `ok` で 0 で終わる。タスク 0036 と同じく、ステップ 5-2 で実行する。

#### 1.3.4. 古い記述の検索の語と、今の一致

設計書 7.2 が「実装プランで確定する」とした検索の語を、次のとおり確定する。どれも 3.5・3.6 が置き換える今の文言か、続けて実行する選択肢に推奨を付ける記述である。`a4aa70a` で、`docs/tasks/` を除く追跡中のファイルを検索した結果を示す。行は `git grep -n -F -e <語> a4aa70a -- ':!docs/tasks/'` で得た。行をまたぐ語も見落とさないよう、各ファイルの改行とタブを空白にし、空白の並びを 1 つにしてから照合し直した。そのときのコマンドは次のとおりである。このコマンドはファイル名だけを出す。出力は、7 つの語のすべてで表のファイルと一致した（`git ls-tree` は除外の pathspec `:!` を受け付けないので、`grep -v` で外した）。

```sh
git ls-tree -r --name-only a4aa70a | grep -v '^docs/tasks/' | while IFS= read -r f; do
	git show "a4aa70a:$f" | LC_ALL=C tr '\n\t' '  ' | LC_ALL=C tr -s ' ' | LC_ALL=C grep -q -F -- "<語>" && printf '%s\n' "$f"
done
```

| 語 | 今の一致 |
|---|---|
| `and run the next command` | `runpr.md:2`、`CLAUDE.md:86`、`CLAUDE.md:101`、`requirements_process.md:55` |
| `and carry it out` | `runpr.md:31` |
| `starts the next command` | `requirements_process.md:55` |
| `the next command to run`（行をまたぐ） | `_lib/task-pr-handoff.md`（`:22-23`） |
| `pre-merge bookkeeping and the next command` | `mergepr.md:124` |
| `then run NEXT` | `runpr.md:443` |
| `run NEXT (recommended` | `runpr.md:359` |

-   `CLAUDE.md:101` は、設計書 7.2 が挙げていない一致である。設計書 3.6 の表の 2 行目（`CLAUDE.md:98-102`）が書き換える範囲にあるので、ステップ 4-1 で直る。
-   `pre-merge bookkeeping and the next command` は、変えない `runpr.md:66` に一致しない（設計書の Comments）。ほかの語も、1.3.2 の変えない箇所には一致しない。
-   `_context.md:63` の古い記述（AC-13）は検索の語に入れない。値を示す場所を加える書き方では、今の文言がそのまま残りうるので、古い文言がないことは AC-13 の確認にならないためである。AC-13 は記述を読んで確かめる（5 章）。

#### 1.3.5. 実装時に決めること

-   各文書の英語の文（設計書 3 章の冒頭、H-03）。
-   step 4 の選択肢の一覧で、推奨を項目のどこに書くか（今は括弧書き。設計書 3.1）。
-   step 4 で示した `実装モデル要件` の値を step 5 で使うことを、step 4 と step 5 のどちらに書くか（ステップ 1-1）。
-   次のブランチが `origin` にあるかを確かめる操作の形（`git ls-remote` の引数など。ステップ 2-2）。
-   `check_wording.sh` の検査の分け方と出力の形（ステップ 5-1 の性質を満たす範囲で決める）。
-   新しい英語の文には、1.3.4 の語を含めない。たとえば「... and run the next command when the user chooses」は、正しい意味でも `check_wording.sh` に失敗する。

### 1.4. 計画の際に決めたこと（2026-10-23）

-   **古い記述がないことの確かめは、タスクのディレクトリに置くスクリプトで行う:** `docs/tasks/0037_runpr_merge_recommendation/check_wording.sh` をコミットし、実装の PR の中で実行して結果を記録する。確かめるのは、1.3.4 の語が残っていないことだけである。先例はタスク 0032 の `check_rules.sh` である。レビュアーが決めた。理由は次のとおりである。設計書 7.2 の `git grep` を、実行される成果物にできる。CI では実行されないので、設計書 1.2（H-01）の「文書のガードのテストは加えない」と食い違わない。step 4 の「Merge」の一覧の構造（推奨が先頭の項目だけに付くこと）もスクリプトで確かめる案は採らなかった。自然言語の文を固定する検査は、言い回しを直すたびに失敗し、中身が誤っていても通るためである（タスク 0036 の 1.4 の先例）。Go のガードのテストにして CI で確かめ続ける案も採らなかった。H-01 の判断を広げて解釈することになるためである。
-   **AC-01・AC-03〜AC-10・AC-13・AC-14 は、記述を読んで確かめる（`manual`）:** 上の決定と設計書 1.2（H-01）の帰結である。これらの AC には `test` も `static` もない。mkplan の規則の例外として 5 章に記す。後の変更で推奨が戻っても検出されないリスクは、設計書 4.4 のとおり受け入れる。
-   **次のブランチの状態を確かめる操作を、Authorization の段落に加える:** 設計書 4.3 は、止まったときに `git ls-remote` などの読み取りで次のブランチの状態を確かめるとしている。`/runpr` の Authorization の段落は、`git ls-remote` を `fixprguard` が行う操作としてだけ挙げており（`runpr.md:58`）、ネットワークの操作には承認が要る（`CLAUDE.md`「Tool Execution Safety」）。そのため、ステップ 2-2 でこの操作を step 5 の操作として加える。設計書 3.5 が変えないとした `runpr.md:66` の文は変えず、設計の決定も変えない。
-   **設計書 8 章の項目をそのままフェーズにし、1 つの PR にまとめる:** 設計書 8 章の順序と、すべての変更を 1 つの PR にまとめる決定に従う。タスク 0032 と同じ形である。

### 1.5. 申し送りの項目への対応

-   `implementation_handoff.md` はない。
-   `design_handoff.md` の H-01〜H-03 は、すべて設計書 3.7 に記録されている。H-01 の帰結は 1.4 に、H-02 はステップ 1-1・2-1 に、H-03 は 1.3.5 に反映した。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: `runpr.md` の step 4（設計書 8 章の 1）

#### ステップ 1-1: マージの質問の「Merge」の選択肢とメッセージ

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] `runpr.md:357-368` の「Merge」の選択肢の一覧を、設計書 3.1 の表の順序・示す条件・推奨にする。推奨の付け方が NEXT の有無と PR の種類によらないことを明記する（AC-01）。続けて実行する選択肢には、示す条件（NEXT があるとき）だけを書き、推奨を書かない（AC-02）。「承認なしでマージ」と「マージしない」は、示す条件を変えず、推奨を付けない（AC-03）。「Offer those of these that apply, and always the last」は残す（設計書 3.1）。
-   [ ] `runpr.md:345-347` の NEXT を示す箇所を、設計書 3.2 の 3 点のとおりに改める（AC-04）: (1) 次の PR マーカーとはどのマーカーかを定める。(2) 値がないときは、値がないことと、分かる範囲でその理由を示す。(3) 値を示す理由に、`/clear` の後の新しいセッションをどのモデルで始めるかの判断を加える。
-   [ ] 次の 2 点を、step 4 か step 5 のどちらかに書く（設計書 1.2・3.2 の最後の段落）。step 4 で示した `実装モデル要件` の値（または値がないこと）を、step 5 の `/clear` の案内でも使うこと。step 4 を 2 回以上尋ねたときは、マージを決めた最後の質問の値を使うこと。
-   [ ] PR レビューの質問（`runpr.md:352-356`）と deferred のスレッドの質問（`runpr.md:348-351`）は変えない（要件書 2.3）。

**完了条件**: 変更後の step 4 を読み、設計書 3.1 の表の 4 行と 3.2 の 3 点が書かれていること、「Merge」の選択肢の中で推奨を書いた箇所がマージだけの選択肢の 1 か所だけであることを確かめた（PR レビューの質問の推奨は、今のまま残る）。

### フェーズ 2: `runpr.md` の step 5（設計書 8 章の 2）

#### ステップ 2-1: 最終報告の場合分けと `/clear` の案内

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] `runpr.md:472-474` を、設計書 3.3 の表の 5 つの場合に分ける（AC-05・AC-07〜AC-09）。続けて実行する選択肢を選んだときの振る舞い（NEXT を実行し、NEXT が PR を開いて止まったところで終わる）は今のままにする（AC-09）。
-   [ ] `/clear` の案内に載せる項目を、設計書 3.3 の 4 項目として英語で書く（AC-05・AC-06）。最終報告は日本語で示すことを書き、日本語の文例は載せない（設計書 1.2 の H-03）。
-   [ ] 次のブランチの作成（`runpr.md:451-454`）と、プランの PR の「次のブランチへ切り替えた」の記録（`runpr.md:174-176`）は変えない（AC-10）。

**完了条件**: 変更後の step 5 を読み、設計書 3.3 の表の 5 つの場合と `/clear` の案内の 4 項目が書かれていること、`runpr.md:451-454`・`:174-176` が PR の差分に現れないことを確かめた。

#### ステップ 2-2: 止まったときの報告と再開の方法

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] step 5 が次のブランチを作り終える前に止まったときの最終報告を、設計書 3.4 の 4 項目（止まった理由、次のブランチの状態、計画書の記録との食い違い、再開の方法）として書く（AC-07）。境界は「`_lib/task-pr-handoff.md` の Part A step 4 を最後まで終えたか」とする（設計書 3.4 の冒頭）。
-   [ ] 再開の方法として、次のブランチの 3 つの状態（チェックアウトされている、`origin` にだけある、ない）ごとの手順と、`mergepr` の片付けが残ったときの `mergepr cleanup --state <state-file>` を書く（設計書 3.4 の項目 4）。`/runpr` の再実行は再開の方法として示さないことを書く（設計書 3.4 の最後の段落）。
-   [ ] この扱いを、続けて実行する選択肢を選んだときにも適用すると書く（設計書 3.3 の最後の項目）。
-   [ ] NEXT がないときの片付け・`git fetch`・detach の失敗の報告は今のままにする（設計書 2.2・4.2）。
-   [ ] 次のブランチが `origin` にあるかを確かめる読み取りの操作（設計書 4.3 の `git ls-remote`）を、Authorization の段落（`runpr.md:50-69`）に step 5 の操作として加える。ローカルの `origin/<branch>` の参照だけで判断しない。`gh issue develop` が `origin` にブランチを作った後に失敗すると、その参照がないことがあるためである。`TestAuthorizationSections` が固定する節（1.3.3）と `:66` の文は変えない。
-   [ ] 再開の方法に書くコマンドを、`git help switch`（ローカルにないブランチ名を渡したとき、追跡するブランチを作るのは `origin/<branch>` の参照があるときだけ）と `_lib/task-pr-handoff.md` Part A step 3・4（`:69-104`）に照らして確かめる。`origin` にだけあるブランチは、`git fetch origin <branch>` をしてから切り替える手順にする。確かめた出どころをコミットメッセージに書く。

**完了条件**: 変更後の step 5 を読み、3.4 の報告の 4 項目と 3 つの状態ごとの再開の方法が書かれていること、再開の方法のコマンドを上の出どころに照らして確かめたこと、Authorization の段落に加えた操作が step 5 の確かめの操作と一致することを確かめた。

### フェーズ 3: `runpr.md` の frontmatter・冒頭・step 5 の見出し（設計書 8 章の 3）

#### ステップ 3-1: `/runpr` の説明

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] `runpr.md:2` の `description`、`:30-31` の冒頭の説明、`:443` の step 5 の見出しを、設計書 3.5 の表の「改めた後に述べること」のとおりに改める（AC-11）。
-   [ ] `runpr.md:66` の Authorization の文は変えない（設計書 3.5、1.3.2）。

**完了条件**: 3 か所が、NEXT を示し、利用者が選んだときに実行すると述べ、NEXT を必ず実行するとは述べていないことを読んで確かめた。

### フェーズ 4: ほかの文書（設計書 8 章の 4）

設計書 3.6 の表の各行を、1 つのステップにする。どの記述も英語で書き、推奨には触れない（設計書 3.6 の表の後の項目）。

#### ステップ 4-1: `CLAUDE.md`

**変更するファイル**: `CLAUDE.md`

-   [ ] 「Claude Commands」の `/runpr` の行（`CLAUDE.md:86`）を、設計書 3.6 の表の 1 行目のとおりに改める（AC-12）。
-   [ ] タスクの PR の流れの段落（`CLAUDE.md:98-102`）を、設計書 3.6 の表の 2 行目のとおりに改める（AC-12）。

#### ステップ 4-2: `_lib/task-pr-handoff.md`

**変更するファイル**: `.claude/commands/_lib/task-pr-handoff.md`

-   [ ] 冒頭の説明（`:19-23`）を、設計書 3.6 の表の 3 行目のとおりに改める（AC-12）。Part A は変えない（設計書 2.1）。

#### ステップ 4-3: `requirements_process.md`

**変更するファイル**: `docs/dev/developer_guide/requirements_process.md`

-   [ ] 「Review Flow」の段落（`:55`）を、設計書 3.6 の表の 4 行目のとおりに改める（AC-12）。図（`:41-53`）は変えない。

#### ステップ 4-4: `mergepr.md`

**変更するファイル**: `.claude/commands/mergepr.md`

-   [ ] step 3 の、`runpr.md` の中で実行するときの説明（`:122-125`）を、設計書 3.6 の表の 5 行目のとおりに改める（AC-12）。

#### ステップ 4-5: `_context.md`

**変更するファイル**: `.claude/commands/_context.md`

-   [ ] 「PR marker conventions」の `実装モデル要件` の説明（`:63`）を、設計書 3.6 の表の 6 行目のとおりに改める（AC-13）。案内を示す条件は書かず、`runpr.md` の step 5 を参照する。

**完了条件（フェーズ 4 の全体。ステップ 4-1〜4-5 に共通）**: 5 つのファイルの変更箇所を読み、設計書 3.6 の表の各行が述べることと一致し、英語で書かれ、推奨に触れていないことを確かめた。

### フェーズ 5: 静的な確認と最終確認（設計書 8 章の 5）

#### ステップ 5-1: 検査のスクリプト

**変更するファイル**: `docs/tasks/0037_runpr_merge_recommendation/check_wording.sh`（新規）

-   [ ] POSIX の `sh` で書く。1.3.4 の 7 つの語のそれぞれについて、`docs/tasks/` を除くファイルに残っていないことを確かめる。いずれかの語が残っていれば、その語と一致したファイルを表示し、0 以外で終わる。どの語も残っていなければ 0 で終わる。
-   [ ] 照合は、1.3.4 のコマンドと同じ形で行う。対象のファイルを一覧にし（作業ツリーでは追跡中のファイルと、無視されていない未追跡のファイル。`CHECK_REV` ではそのツリーのファイル）、`docs/tasks/` を外し（要件書 2.3。スクリプト自身と本タスクの文書も外れる）、各ファイルの改行・タブ・空白の並びを 1 つの空白にしてから `grep -F` で照合する。`git grep` は 1 行ずつ照合するので、行をまたぐ語（`the next command to run`）を見つけられない。行の番号は出さず、ファイル名を出す。
-   [ ] 環境変数 `CHECK_REV` で git のリビジョンを渡すと、作業ツリーではなく、そのリビジョンのツリーを検索する。存在しないリビジョンを渡したときと、git のリポジトリの外で実行したときは、検査の失敗とは別の終了コードで終わる（タスク 0032 の `check_rules.sh` と同じ形）。
-   [ ] スクリプトは、文書から読んだ文字列をシェルのコマンドとして実行しない。
-   [ ] 壊して確かめる（結果はこのステップのコミットメッセージに書く）:
    -   `CHECK_REV=a4aa70a`（変更前のコミット）で実行し、0 以外で終わること、表示が 1.3.4 の表の語とファイルの組とちょうど一致し、`docs/tasks/` の下のファイルを含まないことを確かめる。
    -   作業ツリーで、1.3.4 の語の 1 つを `docs/tasks/` の外の未追跡のファイルに書いて実行し、0 以外で終わってそのファイルを表示することを確かめてから、ファイルを消す。追跡中のファイル（たとえば `CLAUDE.md`）に語を書き戻したときも同じく確かめ、元に戻す。
    -   存在しないリビジョンを `CHECK_REV` に渡したときと、git のリポジトリの外で実行したときに、検査の失敗とは別の終了コードで終わることを確かめる。

**完了条件**: 作業ツリーで実行して 0 で終わり、上の壊して確かめる 3 つが期待どおりに終わる。

#### ステップ 5-2: 最終確認

**変更するファイル**: `03_implementation_plan.md`（チェックボックスの更新。以下の結果は、このコミットのメッセージに書く）

-   [ ] `sh docs/tasks/0037_runpr_merge_recommendation/check_wording.sh` を実行して 0 で終わることを確かめる。
-   [ ] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh` を実行して、すべての検査が `ok` で 0 で終わることを確かめる（1.3.3）。
-   [ ] `make test`・`make lint` と、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行して通ることを確かめる（AC-15）。Go のコードと `README.md`・`docs/dev/security.md` は変えないので、`make ext-test` は要らない（`CLAUDE.md`「Development Notes」）。
-   [ ] 5.1 の横断の検索を行う。
-   [ ] PR の本文に、設計書 8 章の「移行の注意」（古い定義のままの worktree とセッションの扱い）を書く。

**完了条件**: 上がすべて終わり、`make test && make lint` が通る。

### PR-1 作成ポイント: merge-only recommendation and /clear guidance in /runpr

**対象ステップ**: 1-1 / 2-1 / 2-2 / 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 5-2

**推奨タイトル**: `feat(0037): /runpr のマージの質問でマージだけを推奨にし、最終報告で /clear してから次のコマンドを実行するよう案内する`

**レビュー観点**: step 4 の「Merge」の選択肢の順序と推奨が設計書 3.1 のとおりで、推奨を書いたのがマージだけの選択肢の 1 か所だけであること / step 5 の `/clear` の案内と、止まったときの報告・再開の方法（コマンドと Authorization に加えた操作を含む）が設計書 3.3・3.4 のとおりであること / ほかの文書が NEXT を必ず実行すると読めず、推奨に触れていないこと / Authorization の段落が 1 段落のままで `fixprguard` の節を変えていないこと、タスク 0032 の `check_rules.sh` が通ること / `check_wording.sh` が偽の成功を出さないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2 は、マージの後に止まったときの報告と再開の方法を定める回復の手順（recovery flows）で、high-risk/complex step の契機に当たる。設計書 8 章の単一 PR・固定順序の制約により、PR の中で分離も末尾への配置もできないので、レビュー観点の 2 つめで重点的に確かめる。Authorization に加えるのは読み取りの `git ls-remote` だけで（設計書 4.3・5.1）、移行はマージの振る舞いを変えない（設計書 8 章）ので、panel mode の契機には当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | step 4 の「Merge」の選択肢とメッセージ | ステップ 1-1 の完了条件 |
| M2 | step 5 の最終報告、`/clear` の案内、止まったときの報告 | ステップ 2-1・2-2 の完了条件 |
| M3 | `runpr.md` の説明 | ステップ 3-1 の完了条件 |
| M4 | ほかの文書 | フェーズ 4 の完了条件 |
| M5 | 検査のスクリプトと最終確認 | ステップ 5-2 の完了条件と PR-1 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 5-2 | `runpr.md` の step 4・step 5・説明、ほかの 5 つの文書、`check_wording.sh` | frontier-recommended |

すべてのフェーズを 1 つの PR にまとめる（設計書 8 章、3.3）。

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   step 4 は推奨を定める唯一の場所なので、最初に置く（設計書 8 章の 1）。step 5 の `/clear` の案内は、step 4 で示した `実装モデル要件` の値を使うので、step 4 の後に置く。
-   `runpr.md` の説明（フェーズ 3）とほかの文書（フェーズ 4）は、step 4・step 5 の振る舞いを述べるので、その後に置く。
-   検査のスクリプト（フェーズ 5）は、すべての書き換えの後に作業ツリーで通ることを確かめる。変更前のコミットで失敗することは `CHECK_REV` で確かめるので、作る順序には依らない。
-   すべてを 1 つの PR にまとめる。コマンドの定義は、マージされた時点で次のセッションの振る舞いになるので、`runpr.md` とそれを説明する文書が食い違う状態を `main` に置かないためである（設計書 8 章）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

追加しない。Go のコード・拡張のコードを変えず、文書のガードのテストも加えない（設計書 7.1・1.2）。既存のテストへの影響がないことは 1.3.3 で確かめた。ステップ 5-2 で `make test`・`make lint` と文書のガードを実行する（AC-15）。

### 4.2. 文書の確認

古い記述がないことは、ステップ 5-1 の `check_wording.sh` で確かめる（5 章の `static`。1.4）。新しい記述があることと、その中身が設計書のとおりであることは、各フェーズの完了条件で記述を読んで確かめ、PR のレビューで読み合わせる（5 章の `manual`。設計書 7.2 が「静的（記述の読み取り）」と呼ぶもの）。タスク 0032 の `check_rules.sh` も、ステップ 5-2 で実行する（1.3.3）。

### 4.3. 統合テスト（手動）

本タスクの PR のマージの後、次に `/runpr` を実行するときに、設計書 7.3 の 3 つの項目を確かめる。step 5 が次のブランチを作り終える前に止まる場合（AC-07）は、意図して起こすと実際のリポジトリと issue に副作用が出るので、手動では確かめない（設計書 7.3）。

### 4.4. 後方互換性

マージの質問の選択肢の種類と、それぞれを選んだときの振る舞い（記録、マージ、ブランチの作成、NEXT の実行）は変えない（要件書 4.4）。変える前の定義を読んだ worktree やセッションの扱いは、設計書 8 章の「移行の注意」のとおりで、ステップ 5-2 で PR の本文に書く。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行して振る舞いを確かめるもの（`make` ターゲット・CI のジョブ）、`static` はコミット済みのスクリプト `check_wording.sh` の実行、`manual` は記述の読み取りと PR のレビューでの確認を表す。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | manual | `runpr.md` step 4 の「Merge」の一覧（ステップ 1-1 の完了条件、PR のレビュー）。マージ後の統合テスト（4.3） |
| AC-02 | static + manual | `sh docs/tasks/0037_runpr_merge_recommendation/check_wording.sh`（`run NEXT (recommended` が残っていない）。示す条件と、推奨を書いていないことは、ステップ 1-1 の完了条件と PR のレビューで確かめる。マージ後の統合テスト（4.3） |
| AC-03 | manual | ステップ 1-1 の完了条件、PR のレビュー |
| AC-04 | manual | ステップ 1-1 の完了条件、PR のレビュー |
| AC-05 | manual | ステップ 2-1 の完了条件、PR のレビュー。マージ後の統合テスト（4.3） |
| AC-06 | manual | ステップ 2-1 の完了条件、PR のレビュー。マージ後の統合テスト（4.3） |
| AC-07 | manual | ステップ 2-2 の完了条件（再開の方法のコマンドの確かめと、Authorization の段落を含む）、PR のレビュー |
| AC-08 | manual | ステップ 2-1 の完了条件、PR のレビュー。マージ後の統合テスト（4.3） |
| AC-09 | manual | ステップ 2-1 の完了条件、PR のレビュー |
| AC-10 | manual | ステップ 2-1 の完了条件（`runpr.md:451-454`・`:174-176` が PR の差分に現れない） |
| AC-11 | static + manual | `check_wording.sh`（`runpr.md:2`・`:31`・`:443` の古い語が残っていない）。新しい記述は、ステップ 3-1 の完了条件と PR のレビューで確かめる |
| AC-12 | static + manual | `check_wording.sh`（`CLAUDE.md:86`・`:101`、`_lib/task-pr-handoff.md:22-23`、`requirements_process.md:55`、`mergepr.md:124` の古い語が残っていない）。新しい記述は、フェーズ 4 の完了条件と PR のレビューで確かめる |
| AC-13 | manual | ステップ 4-5、フェーズ 4 の完了条件、PR のレビュー（1.3.4 の最後の項目） |
| AC-14 | manual | フェーズ 3・4 の完了条件、PR のレビュー |
| AC-15 | test | `make test`・`make lint`、文書のガード（ステップ 5-2）、CI の `doc-guard` のジョブ |

-   AC-01・AC-03〜AC-10・AC-13・AC-14 は `test` も `static` も持たない。設計書 1.2（H-01）と 1.4 の決定による例外である。後の変更で推奨や案内が戻っても検出されないリスクは、設計書 4.4 のとおり受け入れる。
-   `check_wording.sh` が確かめるのは、古い語が残っていないことだけである。新しい記述があることは確かめない（設計書 7.2）。

### 5.1. 横断の検索

`make test`・`make lint`・`check_wording.sh` が検出しないものだけを挙げる。どれも `manual` で、行った検索とその結果を、ステップ 5-2 のコミットメッセージに書く。

-   [ ] フェーズ 4 で変えた 5 つのファイルと、`runpr.md` の step 4 以外の箇所が、「Merge」の選択肢の推奨（`recommended` など）に触れていない（設計書 1.1）。
-   [ ] `docs/tasks/` を除くファイルに、`/runpr` がマージの後に NEXT を必ず実行する、または既定で実行すると読める記述が、1.3.4 の語のほかに残っていない。`git grep -n -i -e 'next command' -e 'NEXT' -- ':!docs/tasks/'` の一致を、1.3.1 で書き換えた箇所と 1.3.2 の変えない箇所に照らし合わせ、どちらにも当たらない一致がないことを確かめる。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 5-2）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 推奨や案内が後の変更で戻っても、テストでは検出されない | 設計書 4.4 のとおり受け入れる。推奨は step 4 の 1 か所に集め（設計書 1.1）、PR のレビューで見つける |
| 古い記述が、1.3.4 の語と違う言い回しで残る | 5.1 の 2 つめの横断の検索で確かめる |
| `check_wording.sh` が、検索の対象を誤って常に 0 で終わる | ステップ 5-1 の壊して確かめる 3 つ（変更前のコミット、作業ツリーに語を書いたとき、誤った呼び出し）で確かめる |
| 再開の方法のコマンドが、ブランチの状態によっては失敗する（`origin` にだけあるブランチに `git switch` するときなど） | ステップ 2-2 で、`git help switch` と Part A step 3・4 に照らして確かめ、必要な `git fetch` を手順に入れる |
| 次のブランチが `origin` にあるかの確認が、承認されていないネットワークの操作になる | ステップ 2-2 で、その操作を Authorization の段落に加える |
| タスク 0032 の `check_rules.sh` が読む範囲を書き換え、その検査が失敗する | ステップ 5-2 で実行する（1.3.3） |
| 本タスクの PR の `/runpr` を古い定義のチェックアウトから実行し、古い定義と新しい文書が混じる | 設計書 8 章の「移行の注意」のとおり、マージの振る舞いは変わらないので支障はない。PR の本文に書く（ステップ 5-2） |
| step 5 の止まったときの報告が、実際と違う次のブランチの状態を述べる | 設計書 3.4 の 3 点（名前、`origin` とローカルの有無、今のブランチ）を確かめて述べると書く（ステップ 2-2） |

### 7.2. スケジュールのリスク

-   CodeRabbit の PR レビューは 1 時間あたりの回数が限られるので、PR を開いてからレビューが届くまで待つことがある（`runpr.md` step 3）。

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] 各フェーズの完了条件の確認が終わっている。
-   [ ] `make test && make lint` と、PR の CI の `doc-guard` のジョブが通る。
-   [ ] タスク 0032 の `check_rules.sh` が 0 で終わる。
-   [ ] `check_wording.sh` の壊して確かめた結果と、5.1 の横断の検索の結果が、コミットメッセージに書かれている。
-   [ ] 文書の完全さ: 1.3.1 の各行が書き換わり、フェーズ 1〜4 の完了条件の確認が終わっている。
-   [ ] セキュリティ: N/A（要件書 4.2、設計書 5.1）。加えるのは読み取りの `git ls-remote` だけで、外部に書き込む操作は増えない（設計書 4.3）。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0037` で実装する。
-   実装の PR のマージの後、ほかの worktree を `origin/main` に進め、セッションをやり直してから `/runpr` を実行する（設計書 8 章の「移行の注意」）。
-   その後の最初の `/runpr` で、4.3 の統合テストを行う。
-   実装の PR は本タスクの最後のプランの PR なので、そのマージで `/runpr` が issue #401 を閉じる（`runpr.md` step 5）。閉じたことを確かめ、閉じていなければ手で閉じる。
