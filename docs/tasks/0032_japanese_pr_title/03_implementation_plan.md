# 実装計画書：PR のタイトルを日本語にし、squash のコミットは英語のままにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-005 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。変えるのは `CLAUDE.md`・`.claude/commands/` の下のコマンドの定義・`docs/translation_glossary.md`・`mergepr` のガイドだけで、Go のコードとテストは変えない（設計書 1.1・7.1）。このほかに、検査のスクリプト `docs/tasks/0032_japanese_pr_title/check_rules.sh` を新しく作る（1.4）。これは本書の検証の成果物で、規則を述べるファイルではないので、設計書 3.11 の責務表（規則を述べるファイルの一覧）には含めない。設計書 8 章の順序を、そのまま 5 つのフェーズにする。すべてのフェーズを 1 つの PR にまとめる（設計書 8 章）。

-   **フェーズ 1（検査のスクリプトと `_context.md`）:** 静的な検査のスクリプトを作り、タイトルの規約の節とプランの PR の判定を `_context.md` に設ける。
-   **フェーズ 2（`CLAUDE.md` と用語集）:** 言語の規則と用語を改める。
-   **フェーズ 3（PR のタイトルを書くコマンド）:** `_lib/task-pr-handoff.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mkplan2.md` を改める。
-   **フェーズ 4（`mergepr.md` とガイド）:** squash のコミットの件名の決め方を改める。
-   **フェーズ 5（全体の確かめ）:** 古い記述の検査、設計書との突き合わせ、ビルドの確認を行う。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計書の規則を計画で言い換えない。コマンドの定義に書く文は英語で書く（`CLAUDE.md:47-48`）。
-   形は、設計書 3.1 のタイトルの規約の節にだけ書く。ほかのファイルは節の名前（`PR title and squash subject`）で参照する。例外は、`mkplan2.md` が挿入する PR マーカーのひな形だけである（設計書 1.1・3.9）。件名を確かめる正規表現も、この節に置く（1.4）。
-   検査は、コミットするスクリプト `docs/tasks/0032_japanese_pr_title/check_rules.sh` に置く（1.4）。実行はリポジトリの最上位で `sh docs/tasks/0032_japanese_pr_title/check_rules.sh [<検査の名前>...]` とする。本書では、以下この実行を「`check_rules.sh` を実行する」と書く。
-   **各ステップの完了条件:** そのステップが加えた検査が、(1) ファイルを変える前に失敗し、(2) 変えた後に通り、(3) 変えた後のファイルから、検査が確かめる性質を述べた文を 1 つずつ一時的に消すか書き換え、そのたびに、その性質の名前を示して検査が失敗することを確かめる。3 つの結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。以下の各ステップの「完了条件」は、この 3 つに加えるものだけを書く。
-   各コマンドが作るコミットのメッセージは、英語のまま変えない（要件書 AC-10、設計書 3.4・3.8）。
-   各フェーズの最後に `make test && make lint` を実行し、変更の前と同じく通ることを確かめる（設計書 7.1）。

### 1.3. 既存コード調査結果

調査は、コミット `dd4ce5d` で行った。設計書が確かめたコミット `8c3a680` から、`.claude/`・`CLAUDE.md`・`docs/translation_glossary.md`・`docs/dev/` に変更はない（`git diff --quiet 8c3a680 dd4ce5d -- .claude CLAUDE.md docs/translation_glossary.md docs/dev` の終了コード 0 で確かめた）。そのため、設計書の `file:line` は `dd4ce5d` でもそのまま使える。

#### 1.3.1. 変える箇所の一覧

古い規則の記述を、`docs/tasks/` を除く追跡中のファイルで `git grep` により数え上げた（`dd4ce5d`）。数え上げに使った語は、`English title`・`concise English`・`title already fits`・`built from it`・`conventional-commit title`・`PR title`（大文字小文字を区別しない）、`feat(<task-id>)`、`--title "`、`plan-driven`・``came from `/runplan` ``、`PR のタイトル`・`推奨タイトル`・`件名` である。

| 箇所 | 今の記述 | 変え方 | ステップ |
|---|---|---|---|
| `CLAUDE.md:50-51` | PR のタイトルを英語の conventional-commit のタイトルにする例外 | 設計書 3.3 | 2-1 |
| `.claude/commands/_context.md:62` | `Conventional commit scope format: feat(<task-id>): <concise English title>` | 設計書 3.9 | 1-4 |
| `.claude/commands/_lib/task-pr-handoff.md:35-37` | PR_TEXT がコミットのタイトル | 設計書 3.4 | 3-1 |
| `.claude/commands/_lib/task-pr-handoff.md:98` | `--title "<title>"` | 設計書 3.1.1 | 3-1 |
| `.claude/commands/_lib/task-pr-handoff.md:100-101` | `in English (the squash commit subject is built from it)` | 設計書 3.4 | 3-1 |
| `.claude/commands/runplan.md:91` | PR_TEXT が `推奨タイトル` そのもの | 設計書 3.5 | 3-2 |
| `.claude/commands/runplan.md:198` | `the title is feat(<task-id>): <concise English title>` | 設計書 3.5 | 3-2 |
| `.claude/commands/runpr.md:82` | `--title "<title>"` | 設計書 3.1.1 | 3-3 |
| `.claude/commands/runpr.md:85-87` | `English, since the squash commit subject is built from it` | 設計書 3.6 | 3-3 |
| `.claude/commands/runpr.md:105-108` | 「Plan PR」の定義 | 設計書 3.2（`_context.md` への参照に替える） | 3-3 |
| `.claude/commands/runpr.md:270-273` | `mergepr.md` の step 3 の項目の列挙 | 設計書 3.6・3.7（接頭辞を決めた規則の記録を加える） | 3-3 |
| `.claude/commands/fixpr.md:462` | `gh pr edit NUMBER --title "<real title>"` | 設計書 3.1.1 | 3-4 |
| `.claude/commands/fixpr.md:466` | `Keep the title an English conventional-commit title` | 設計書 3.6 | 3-4 |
| `.claude/commands/mkplan2.md:62`・`:80`・`:129` | 英語の `feat(<task-id>)` の `推奨タイトル` | 設計書 3.9 | 3-5 |
| `.claude/commands/mergepr.md:49-52` | `When the PR title already fits, use it.` と `plan-driven PR` | 設計書 3.7 | 4-1 |
| `.claude/commands/mergepr.md:60`・`:107` | `plan-driven PR`・``when the PR came from `/runplan` `` | 設計書 3.2（「プランの PR」の語にそろえる） | 4-3 |
| `docs/dev/developer_guide/mergepr_guide.md:93`・`mergepr_guide.ja.md:93` | 承認の質問で示す項目の列挙（PR の URL、base ブランチ、CI の結果、件名と本文） | 設計書 3.7 の末尾（接頭辞を決めた規則の記録を加える） | 4-3 |
| `docs/translation_glossary.md:59` | 「材料」の備考 | 設計書 3.11 | 2-2 |

変えない箇所は次のとおりである。

-   `.claude/commands/mkreq.md:107` の `<concise English summary>`、`mkarch.md:76`・`mkplan.md:100` の `docs(NNNN): draft ...`、`japrose.md:133` の `docs(<task-id>): improve Japanese prose`、`_lib/task-pr-handoff.md:69` の件名の判定は、コミットの件名なので変えない（要件書 AC-10、設計書 3.4・3.8）。
-   `.claude/commands/runpr.md:321` の `gh issue create --title "PR #<number> のレビュー指摘の追跡"` は、issue のタイトルの固定の文字列で、PR のタイトルでもエージェントが書く値でもないので変えない（要件書 2.3）。
-   `state.json` の `title` は `internal/mergepr/mergepr.go:78` の `Title string \`json:"title"\`` で、`mergepr_guide.md:120` の説明と一致する。ガイドのこの説明は変えない。
-   Go のコードとテスト。`.claude/` を読むテストは `internal/fixprguard/scope_test.go:46` だけで、これはパスの分類の表の入力であり、コマンドの定義の文を読まない（`git grep -ln '\.claude' -- '*_test.go'`、`dd4ce5d`）。用語集は、文書のガード（`cmd/yt2column-server/docs_test.go:279-304`、`cmd/yt2column/copied_transcript_docs_test.go:17,272`、`internal/pipeline/relay_guard_test.go:436`）が表として読む。ステップ 2-2 は表の形を保ち、これらのガードを通す。

#### 1.3.2. 外部の前提の確かめ

-   **`gh` のタイトルのフラグ:** `gh pr create --help`（`gh` 2.96.0）には `-t, --title string` だけがあり、タイトルをファイルから読むフラグはない。`--body-file` は本文だけに使える。設計書 3.1.1 のコマンド置換の方法をそのまま使う。
-   **ASCII の確かめの方法:** 設計書 3.7 が本書に委ねた「どう確かめるか」は、件名のファイルに対する `LC_ALL=C grep -Eqx` にする（1.4）。次の正規表現とコマンドで試した（`dd4ce5d` の時点の手元の環境）。

    ```
    R='(feat|fix|docs|refactor|test|perf|build|ci|chore|style|revert)(\([a-z0-9-]+\))?: [!-~]([ -~]*[!-~])? \(#[0-9]+\)'
    LC_ALL=C /usr/bin/grep -Eqx "$R" <subject-file>
    ```

    `/usr/bin/grep`（BSD grep 2.6.0-FreeBSD）と ugrep 7.8.4 で、結果は同じだった。受け付けたのは `docs(0032): draft the plan (#413)` と `fix: tidy (#1)` の 2 つである。拒んだのは、全角の括弧・全角のコロン・キリル文字の `о`・日本語の要約・大文字の `Feat`・`!` 付き・コロンの後の 2 つの空白・要約の中の全角空白・`(#` の前の 2 つの空白の 9 つである。ステップ 1-2 は、この正規表現を出発点にする。`grep -x` は、ファイルのどれか 1 行が合えば成功する。そのため、正規表現の確かめの前に、件名のファイルがちょうど 1 行（改行で終わる 1 行、または改行のない 1 行）であることを確かめる。2 行目を持つファイルや、行の中に CR を含むファイルを、合わないものとする。
-   **検査のスクリプトの実行環境:** 開発環境の `/bin/sh` は bash 3.2.57 である。また、`grep` はシェルの関数で ugrep に置き換えられていることがある（`type grep` で確かめた）。スクリプトは POSIX の `sh` で書き、ファイルの検索には `git grep` を使い、件名の確かめにだけ `LC_ALL=C grep` を使う。

#### 1.3.3. ほかのタスクとの重なり

-   **タスク 0030 の PR-6（未着手）:** `fixpr.md` と `runpr.md` を `fixprguard` を呼ぶ形に切り替える（`docs/tasks/0030_fixpr_untrusted_threads/03_implementation_plan.md` のフェーズ 6）。
    -   タスク 0030 が変えるファイルのうち、`_context.md`（同 6-1）、`fixpr.md`（6-2）、`runpr.md`（6-3）、`docs/translation_glossary.md`（6-4）、`CLAUDE.md` と `_lib/task-pr-handoff.md`（6-5）の 6 つが、本タスクが変えるファイルと重なる。
    -   その設計書は、`fixpr.md` の Phase 6 のエージェントにタイトルと本文の案を JSON で返させる。`fixprguard edit-pr` がタイトルのファイルを読み、`gh pr edit` をシェルなしで実行する（同 `02_architecture.md:667`）。
    -   `cmd/yt2column/fixpr_docs_test.go` に、`fixpr.md` と `runpr.md` の文のガードを加える。同 6-2 は、`runpr.md` の `--title "`（PR の作成）を正当な記述として残すとしている。
-   **タスク 0031:** 2026-10-21 の時点で、要件書（PR #407）だけがマージされ、PR #411（文書だけ）が開いている。本タスクが変える段落とは重ならない（設計書 3.10 の H-05）。
-   どちらのタスクについても、先に `main` に入った変更の上に重ね、相手の記述を書き換えない（設計書 3.10 の H-05）。タスク 0030 と 0031 の進み具合は、ステップ 1-1、フェーズ 3 の前、ステップ 5-3 の 3 回確かめる。
-   確かめた結果、タスク 0030 の PR-6 が `main` に入ったかの答えが前回と変わっていれば、ブランチを `origin/main` の上に重ね直し、PR-6 の有無で内容が変わる次のステップのうち、すでに終えたものをやり直す。
    -   ステップ 3-3・3-4: もう一方の手順を採る。
    -   ステップ 5-1 の検査 `title_via_file`: 期待の数を改める（PR-6 の後は 2、前は 3）。
    -   そのあと、ステップ 5-1〜5-3 のうちすでに終えたもの（検査の全体の実行、設計書との突き合わせ、ビルドの確認）を、重ね直した後のファイルでもう一度行う。

#### 1.3.4. 実装時に決めること

-   コマンドの定義に書く英語の文面。設計書 3 章の規則の内容を変えない範囲で決める。
-   検査のスクリプトの各検査が探す語と、探す範囲（ファイル全体か、特定の step の段落か）。各検査は、各ステップと 5 章の表に挙げた性質を確かめる。

### 1.4. 計画の際に決めたこと（2026-10-21）

-   **静的な検査は、タスクのディレクトリに置くスクリプトで行う:** `docs/tasks/0032_japanese_pr_title/check_rules.sh` をコミットし、実装の PR の中で実行して結果を記録する。設計書 7.1（テストを追加しない）と 7.2（`git grep` で確かめる）に合い、設計書を改める必要がないためである。Go のガードのテストにして CI で古い記述の再発を防ぐ案は採らない。設計書 7.1 と食い違い、設計書の承認をやり直すことになるためである。このスクリプトは CI では実行されないので、本タスクの後に古い記述が戻っても検出されない（7 章のリスク）。
-   **件名の確かめは、件名のファイルが 1 行であることの確かめと `LC_ALL=C grep -Eqx` とし、正規表現はタイトルの規約の節に置く:** 1.3.2 のとおり、手元の 2 つの `grep` で同じ結果になることを確かめた。新しいツールの導入やバイナリの変更を要しない（設計書 9 章の YAGNI）。正規表現は `<type>` の一覧と `<scope>` の文字の範囲を反映するので、`mergepr.md` ではなく、表と同じ節に置く。`mergepr.md` は、節の正規表現で確かめると参照する。`<type>` を増やすときは、同じ節の表と正規表現を改める（設計書 9 章の「節の表だけを改める」の範囲）。検査のスクリプトは、表と正規表現の `<type>` の集合が一致することを確かめる（ステップ 1-2）。
-   **タスク 0030 の PR-6 が先にマージされ、そのガードのテストが本タスクの書き換える文を固定していた場合は、止まって利用者に尋ねる:** 本タスクは Go のテストを変えない計画なので、そのガードの期待値を変えるかは、計画を変える判断になるためである。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-05 は、すべて設計書 3.10 に記録されている。H-05（タスク 0031 との重なり）は、1.3.3 で実装の手順に落とした。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 検査のスクリプトと `_context.md`（設計書 8 章の 1）

#### ステップ 1-1: 検査のスクリプトの枠組み

**変更するファイル**: `docs/tasks/0032_japanese_pr_title/check_rules.sh`（新規）

-   [ ] `git log origin/main` で、タスク 0030 の PR-6 とタスク 0031 の実装が `main` に入っているかを確かめ、結果をコミットメッセージに書く（1.3.3）。`git fetch` は、利用者の承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。
-   [ ] POSIX の `sh` で書く。名前の付いた検査を順に実行し、失敗した検査の名前と、見つかった行（または見つからなかった性質）を表示し、1 つでも失敗すれば 0 以外で終わる。引数で検査の名前を渡すと、その検査だけを実行する。
-   [ ] ファイルの検索は `git grep` で行い、`docs/tasks/` を対象から外す（要件書 AC-17 の範囲）。環境変数 `CHECK_REV` で git のリビジョンを渡すと、作業ツリーではなく、そのリビジョンのツリーを検索する（ステップ 5-1 で使う）。存在しないリビジョンを渡すと、0 以外で終わる。
-   [ ] スクリプトは、文書から取り出した文字列をシェルのコマンドとして実行しない（`eval` しない）。文書から取り出すのは正規表現の文字列だけで、それを固定のコマンドで `grep` に渡す。

**完了条件**: 検査を 1 つも持たないスクリプトが 0 で終わり、存在しない検査の名前と、存在しない `CHECK_REV` を渡すと 0 以外で終わる。

#### ステップ 1-2: タイトルの規約の節

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_title_section` を加える。この検査は、`_context.md` に節「PR title and squash subject」があり、その節が次のことを述べることを確かめる。
    -   設計書 3.1 の表の 6 項目（PR のタイトル・受け付ける接頭辞・タスクの文書の PR のタイトル・プランの PR のタイトル・`推奨タイトル`・squash のコミットの件名）を述べる。
    -   受け付ける `<type>` の表にちょうど 11 個の値があり、それが要件書 AC-13 の 11 個と一致する。`<scope>` の文字の範囲を述べる。
    -   合わない接頭辞は直さず、接頭辞がないものとして扱うと述べる。
    -   PR のタイトルを `<scope>` のある形に限る（設計書 3.1 の表の 1 行目）。
    -   タイトルをファイルに書いてコマンド置換で渡すこと、また PR のタイトルと `推奨タイトル` でバッククォートを使わないことを述べる（設計書 3.1.1）。
-   [ ] 検査 `context_subject_regex` を加える。節の中に、決まった見出しの付いたコードブロックで、件名を確かめる正規表現がちょうど 1 つあり、その `<type>` の選択肢の集合が、表の 11 個と同じであることを確かめる。正規表現が見つからないか、2 つ以上あれば失敗する。
-   [ ] 検査 `subject_regex_samples` を加える。上の正規表現を取り出し、件名のファイルが 1 行であることを確かめたうえで `LC_ALL=C grep -Eqx` を見本の件名のファイルに対して実行し、結果が期待のとおりであることを確かめる。見本は次のとおりとする。
    -   受け付ける見本: 11 個の `<type>` のそれぞれ 1 つ。`<scope>` のあるものと、ないもの。
    -   拒む見本: 1.3.2 で拒んだ 9 つ。設計書 3.1 の「それ以外」の例のそれぞれ（空の括弧・`<scope>` の大文字・`:` の後のノーブレークスペース・`‐` と `−`）。加えて、未知の `<type>`・`(#<number>)` のないもの・`(#<number>)` の後の空白・タブ・合う行の後に 2 行目を持つもの・行の中に CR を含むもの。
-   [ ] 設計書 3.1 の 2 つの表（合わない例を含む）、3.1.1 の渡し方とバッククォートの規則、件名を確かめる正規表現（1.3.2 を出発点にする）を、「Process convention」の節の中に、「PR marker conventions」と並べて置く。

**完了条件**: 1.2 の 3 つ。加えて、`context_subject_regex` が、表から `<type>` を 1 つ消したときと、正規表現から 1 つ消したときの両方で失敗することを確かめる。`subject_regex_samples` が、正規表現から `<type>` を 1 つ消したとき、`<scope>` の文字の範囲を `[a-zA-Z0-9_-]` に広げたとき、1 行の確かめを外したときに失敗することを確かめる。

#### ステップ 1-3: プランの PR の判定

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_plan_pr` を加える。`_context.md` にプランの PR の判定の定義があり、次のことを述べると確かめる: PR の head のツリーを読むこと、URL 全体が一致する行だけを数えること、一致する行が複数のタスクのプランにあるとき、またはツリーを読めないときに止まること、最後の PR かどうかの規則。
-   [ ] 設計書 3.2 の定義を書く。中身は `runpr.md:105-108` と同じにし、読むツリーと止まる 2 つの場合を加える。`runpr.md` 側の置き換えはステップ 3-3 で行う。

**完了条件**: 1.2 の 3 つ。

#### ステップ 1-4: PR マーカーの規約の `推奨タイトル`

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_marker_conventions` を加える。「PR marker conventions」が、`推奨タイトル` は PR のタイトルであって squash のコミットの件名ではないと述べ、タイトルの規約の節を参照することを確かめる。
-   [ ] `_context.md:62` の行を、設計書 3.9 の表の 1 行目のとおりに改める。

**完了条件**: 1.2 の 3 つ。フェーズの最後として `make test && make lint` が通る。

### フェーズ 2: `CLAUDE.md` と用語集（設計書 8 章の 2）

#### ステップ 2-1: `CLAUDE.md` の「Language」

**変更するファイル**: `CLAUDE.md`、`check_rules.sh`

-   [ ] 検査 `claude_language` を加える。「Language」の節について、次のことを確かめる。
    -   PR のタイトルを、GitHub に置く日本語のものに含める。PR のタイトルを英語にする例外を述べない。
    -   タイトルの規約の節を参照する。
    -   squash のコミットの件名と本文を英語で書き、件名は `/mergepr` が PR の材料から書くと述べる。
    -   bot のコマンドの例外が残っている。
-   [ ] 設計書 3.3 のとおりに改める。

**完了条件**: 1.2 の 3 つ。

#### ステップ 2-2: 用語集

**変更するファイル**: `docs/translation_glossary.md`、`check_rules.sh`

-   [ ] 検査 `glossary_terms` を加える。設計書 3.11 の 4 語（接頭辞・受け付ける接頭辞・要約・プランの PR）が英訳とともにあり、「材料」の備考が diff を含むことを確かめる。
-   [ ] 設計書 3.11 のとおりに加え、改める。英訳は、コマンドの定義で使う英語の語（`prefix`・`accepted prefix`・`summary`・`plan PR`）と一致させる。表の形は変えない（1.3.1）。

**完了条件**: 1.2 の 3 つ。フェーズの最後として `make test && make lint` が通る（用語集を読む文書のガードを含む）。

### フェーズ 3: PR のタイトルを書くコマンド（設計書 8 章の 3）

このフェーズを始める前に、タスク 0030 の PR-6 が `main` に入ったかをもう一度確かめる。入っていれば、ブランチを `origin/main` の上に重ね直し、`make test` とここまでの検査を実行し、通ることを確かめてから始める。

#### ステップ 3-1: `_lib/task-pr-handoff.md`

**変更するファイル**: `.claude/commands/_lib/task-pr-handoff.md`、`check_rules.sh`

-   [ ] 検査 `handoff_pr_text` を加える。次のことを確かめる。
    -   PR_TEXT の定義が、タイトルの規約の節のタスクの文書の PR のタイトルを参照する。
    -   Part B の step 3 が、タイトルを英語としない。
    -   Part B の step 3 が、タイトルを Write ツールでファイルに書き、コマンド置換で渡す。
-   [ ] 設計書 3.4 と 3.1.1 のとおりに改める。`runplan.md` についての PR_TEXT の記述も、設計書 3.4 のとおりに改める。

**完了条件**: 1.2 の 3 つ。

#### ステップ 3-2: `runplan.md`

**変更するファイル**: `.claude/commands/runplan.md`、`check_rules.sh`

-   [ ] 検査 `runplan_title` を加える。step 5a と step 8 について、次のことを確かめる。
    -   step 5a が、`推奨タイトル` を下書きとして扱う。
    -   step 5a が、接頭辞の 3 つの場合（`<scope>` のある受け付ける接頭辞・`<scope>` のない受け付ける接頭辞・受け付ける接頭辞がない）を述べる。
    -   step 5a が、要約を日本語で書き、プランを書き換えないと述べる。
    -   step 8 が、タイトルの規約の節のプランの PR のタイトルを参照する。
-   [ ] 設計書 3.5 のとおりに改める。`推奨タイトル` の言語を判定する文を書かない（設計書 1.1、H-03）。

**完了条件**: 1.2 の 3 つ。

#### ステップ 3-3: `runpr.md`

**変更するファイル**: `.claude/commands/runpr.md`、`check_rules.sh`

-   [ ] 検査 `runpr_title` を加える。次のことを確かめる。
    -   step 1 が、タイトルを PR のタイトルの形の日本語で書き、`<type>(<scope>)` を `git log origin/main` の形に合わせ、ファイルから渡す。
    -   step 2 が、`_context.md` のプランの PR の判定を参照する。
    -   step 4 の 2 の列挙が、接頭辞を決めた規則の記録を含む。
-   [ ] 設計書 3.6 の `runpr.md` の 3 項目のとおりに改める。step 2 の「Plan PR」の文は `_context.md` の参照に置き換え、PR の種類の判定の残り（Task-document PR・Standalone change）は変えない（設計書 3.8）。
-   [ ] タスク 0030 の PR-6 が先にマージされていれば、その後の `runpr.md` の該当の段落に同じ変更を加え、`make test` でそのガード（`cmd/yt2column/fixpr_docs_test.go`）が通ることを確かめる。ガードが本タスクの書き換える文を固定していて通らなければ、止まって利用者に尋ねる（1.4）。

**完了条件**: 1.2 の 3 つ。

#### ステップ 3-4: `fixpr.md` の Phase 6

**変更するファイル**: `.claude/commands/fixpr.md`、`check_rules.sh`

-   [ ] 検査 `fixpr_title` を加える。Phase 6 のプロンプトについて、次のことを確かめる。
    -   `.claude/commands/_context.md` の節「PR title and squash subject」を名指しして読ませる。
    -   新しいタイトルを、その節の PR のタイトルの形で書かせる。
    -   タイトルを書き直す条件（本文が大きく古くなったとき、具体的な文を書けたとき）を保つ。
-   [ ] タスク 0030 の PR-6 がまだマージされていなければ、設計書 3.6 の `fixpr.md` の項目のとおりに改め、`gh pr edit` のタイトルを 3.1.1 のとおりファイルから渡す。
-   [ ] タスク 0030 の PR-6 が先にマージされていれば、Phase 6 のエージェントがタイトルの案を書く箇所に、上の検査の内容を加える。タイトルは `fixprguard edit-pr` がファイルから読んでシェルなしで渡すので、3.1.1 の目的（シェルに展開させない）はすでに満たされている。コマンド置換は加えない。このとき、設計書 3.1.1 と 3.6 の `fixpr.md` についての記述（`gh pr edit` に `--title "$(cat <file>)"` で渡す）を、`fixprguard edit-pr` がタイトルのファイルを読む形に改め、`Comments` に editorial の修正として記録する（決定は変わらない）。ステップ 3-3 と同じく、そのガードが通らなければ止まって利用者に尋ねる。どちらの場合を採ったかをコミットメッセージに書く。

**完了条件**: 1.2 の 3 つ。

#### ステップ 3-5: `mkplan2.md`

**変更するファイル**: `.claude/commands/mkplan2.md`、`check_rules.sh`

-   [ ] 検査 `mkplan2_recommended_title` を加える。次のことを確かめる。
    -   挿入する書式が `` `<type>(<task-id>): <Japanese summary>` `` である。
    -   欄の説明が、タイトルの規約の節を参照する。
    -   レビューのチェックリストが、形に合うこととバッククォートを含まないことを確かめると述べる。
-   [ ] 設計書 3.9 の表の 2〜4 行目のとおりに改める。

**完了条件**: 1.2 の 3 つ。

#### ステップ 3-6: コミットのメッセージ

**変更するファイル**: `check_rules.sh`

-   [ ] 検査 `commit_subjects_unchanged` を加える。`.claude/commands/` の下のすべてのファイルを対象に、次のことを確かめる（要件書 AC-10）。
    -   各コマンドが作るコミットの件名の記述が残っている。対象は、`mkreq.md`・`mkarch.md`・`mkplan.md` の `docs(NNNN): draft ...`、`fixpr.md` の `fix: address PR #NUMBER review comments`、`runpr.md` の `docs(<task-id>): record the review of PR #<number>` である。
    -   コミットのメッセージを日本語で書く指示がない。

**完了条件**: この検査は変える前から通るので、1.2 の (1) に代えて、壊して確かめる。`runpr.md` の記録のコミットの件名を一時的に消したときと、`runplan.md` にコミットのメッセージを日本語で書く文を一時的に加えたときの両方で失敗することを確かめ、戻す。フェーズの最後として `make test && make lint` が通る。

### フェーズ 4: `mergepr.md` とガイド（設計書 8 章の 4）

#### ステップ 4-1: 件名の規則

**変更するファイル**: `.claude/commands/mergepr.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_subject` を加える。step 2 について、次のことを確かめる。
    -   件名の形を、タイトルの規約の節で参照する。
    -   要約を英語で PR の材料から書き、PR のタイトルを要約の情報源にしない。
    -   接頭辞を設計書 3.7 の 3 つの規則で決め、規則の文がその順（プランの PR → タイトルの接頭辞 → PR の材料）に並ぶ。順は行の位置で確かめる。
    -   プランの PR では、タイトルからは `<type>` だけを使う。
    -   `<type>` を表の値で書く。
    -   PR のタイトルを `state.json` の `title` から読む。
    -   PR のタイトル・本文・コミットのメッセージ・diff をデータとして扱う規則を、接頭辞を読むときにも適用する。
    -   本文を英語で書く。
    -   プランの PR を `_context.md` の判定で見分け、`/runpr` の中では `/runpr` の step 2 の判定をそのまま使う。
    -   判定できないときに止まる。
-   [ ] 設計書 3.7 のとおりに、`mergepr.md:49-52` の「Subject」の項目を置き換える（設計書 3.2・4.3 の文の置き場所を含む）。

**完了条件**: 1.2 の 3 つ。

#### ステップ 4-2: 承認の前の確かめ

**変更するファイル**: `.claude/commands/mergepr.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_subject` に、次のことを加える。
    -   step 2 が、件名のファイルを書いた後、承認の質問の前に、件名のファイルが 1 行であることと、節の正規表現に合うことを確かめる（確かめ方そのものはタイトルの規約の節にあり、ステップ 1-2 の `subject_regex_samples` が試す）。
    -   合わなければ書き直し、合わないまま承認の質問へ進まない（設計書 4.3）。
    -   step 3 で利用者の求めで件名を直したときも、確かめ直す。
-   [ ] 上の 3 つを step 2・step 3 に書く。

**完了条件**: 1.2 の 3 つ。

#### ステップ 4-3: 接頭辞を決めた規則の表示とプランの PR の語

**変更するファイル**: `.claude/commands/mergepr.md`、`docs/dev/developer_guide/mergepr_guide.md`、`docs/dev/developer_guide/mergepr_guide.ja.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_subject` に、次のことを加える。
    -   step 3 で見せる項目が、接頭辞を決めた規則の記録を含む。
    -   `mergepr.md` に、`plan-driven` と ``came from `/runplan` `` が残らない。
-   [ ] 検査 `mergepr_guide_items` を加える。両方のガイドの 93 行目の段落が、接頭辞を決めた規則の記録を含むことを確かめる。
-   [ ] 設計書 3.7 の「接頭辞を決めた規則を示す」のとおりに、step 2 に記録を書かせ、step 3 の項目に加える。`mergepr.md:51`・`:60`・`:107` の言い回しを「プランの PR」（`plan PR`）にそろえる（設計書 3.2）。
-   [ ] 両方のガイドの 93 行目の列挙に、接頭辞を決めた規則の記録を加える（設計書 3.7 の末尾）。日本語版を先に直し、英語版をそれに合わせる（`CLAUDE.md`「Translation Guidelines」）。

**完了条件**: 1.2 の 3 つ。フェーズの最後として `make test && make lint` が通る。

### フェーズ 5: 全体の確かめ（設計書 8 章の 5）

#### ステップ 5-1: 古い記述が残っていないこと

**変更するファイル**: `check_rules.sh`

-   [ ] 検査 `no_stale_rules` を加える。`docs/tasks/` を除く追跡中のファイルに、要件書 AC-17 の 3 種類の記述が残っていないことを確かめる。探す語は、1.3.1 の数え上げの語のうち古い規則を表すもの（英語のタイトル、件名をタイトルから作る、`feat` に限る `推奨タイトル`）とする。英語の `推奨タイトル` を入力として受け付ける説明（要件書 AC-05）は、探す語に当たらない文面で書く。
-   [ ] 検査 `title_via_file` を加える。`.claude/commands/` の、`gh pr create` または `gh pr edit` と `--title` を同じ行に含むすべての行について、`--title` が `"$(cat <file>)"` の形であることを確かめる（設計書 3.1.1）。`--title` を含まない説明の行（`_lib/task-pr-handoff.md:46`、`runpr.md:35` の Authorization の列挙など）は対象外とする。対象の行の数が期待の数より少なければ失敗する。対象の行が見つからないまま検査が通ることを防ぐためである。期待の数は、`_lib/task-pr-handoff.md` と `runpr.md` の 2 つで、タスク 0030 の PR-6 がマージされる前は `fixpr.md` を加えた 3 つである。`runpr.md:321` の issue のタイトルは対象外である（1.3.1）。
-   [ ] `CHECK_REV` にステップ 1-1 のコミットを渡し、そのツリーで両方の検査が失敗することを確かめる。

**完了条件**: 上の 3 つ。

#### ステップ 5-2: 設計書との突き合わせ

**変更するファイル**: 指摘を受けたときだけ、ステップ 1-2〜4-3 で変えたファイル（`_context.md`・`CLAUDE.md`・`docs/translation_glossary.md`・`_lib/task-pr-handoff.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mkplan2.md`・`mergepr.md`・`mergepr` のガイド）

-   [ ] レビューのサブエージェントに、変えた各ファイルの新しい文を、設計書 3.1〜3.9 の規則と 1 文ずつ突き合わせさせる（`.claude/commands/_lib/review-subagent-pattern.md`）。検査のスクリプトは語の有無しか確かめないので、規則の意味（順序・例外・止まる条件）が設計書と一致することを、ここで確かめる。指摘を直し、直したことをコミットメッセージに書く。

**完了条件**: レビューに Critical と Major の指摘が残らない。

#### ステップ 5-3: 全体の実行と PR の準備

**変更するファイル**: なし（確かめと PR の本文だけ。ブランチを重ね直したときは、1.3.3 に挙げたステップのファイル）

-   [ ] タスク 0030 の PR-6 が `main` に入ったかを、もう一度確かめる。フェーズ 3 の前と答えが変わっていれば、1.3.3 のとおりブランチを重ね直し、そこに挙げたステップをやり直す。
-   [ ] `check_rules.sh` を引数なしで実行し、0 で終わることを確かめる。出力をコミットメッセージに書く。
-   [ ] `make test && make lint` を実行する。
-   [ ] PR の本文に次の 2 つを書く。
    -   設計書 8 章の「移行の注意」（マージの後、ほかの worktree を `origin/main` に進めるか、セッションをやり直してから `/mergepr`・`/runpr` を実行する）。
    -   本タスクが先にマージされたときは、タスク 0030 の PR-6 が本タスクの規則（タイトルの規約の節の参照。要件書 AC-09）を保つ必要があること。

**完了条件**: 上の 4 つ。

### PR-1 作成ポイント: Japanese PR titles and English squash subjects

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 / 5-3

**推奨タイトル**: `feat(0032): write PR titles in Japanese and keep squash subjects in English`

**レビュー観点**: タイトルの規約の節が設計書 3.1・3.2 と一致し、ほかのファイルが形を書き写さずに参照していること / `mergepr.md` の step 2 の接頭辞の決め方と承認の前の確かめが設計書 3.7 のとおりであること / タイトルを 3 か所でファイルとコマンド置換で渡していること（設計書 3.1.1）と、古い記述が残っていないこと / `check_rules.sh` の各検査が偽の成功を出さないこと（正規表現の取り出し方を含む）と、タスク 0030 の PR-6 との重なりの扱い

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1〜4-3 は、マージのたびにエージェントが読んで従う件名の決め方を置き換える、高リスクの変更である。単一の PR にまとめる制約（設計書 8 章）のもとで、振る舞いを変えるステップの中では最後に置いた。panel mode の契機には当たらない。移行は単一の PR で一度に切り替わり、段階的な展開を要しない。また、本タスクは `docs/dev/security.md` の適用の条件に当たらない（要件書 4.2、設計書 5 章）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | 検査のスクリプトの枠組み、タイトルの規約の節、プランの PR の判定 | フェーズ 1 の完了条件 |
| M2 | `CLAUDE.md` の「Language」と用語集 | フェーズ 2 の完了条件 |
| M3 | PR のタイトルを書くコマンドの定義 | フェーズ 3 の完了条件 |
| M4 | `mergepr.md` の件名の決め方とガイド | フェーズ 4 の完了条件 |
| M5 | 全体の検査と PR | PR-1 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 / 5-3 | タイトルの規約の節、`CLAUDE.md`、用語集、PR のタイトルを書くコマンド、`mergepr.md` とガイド、検査のスクリプト | frontier-recommended |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。ほかのすべての変更がタイトルの規約の節とプランの PR の判定を参照するので、フェーズ 1 を最初に置く。検査のスクリプトの枠組みもフェーズ 1 に置き、各ステップが自分の検査を先に加えられるようにする。PR を 1 つにするのは、コマンドの定義がマージされた時点でそのまま次のセッションの振る舞いになり、途中の状態を `main` に置くと規則が互いに矛盾するためである（設計書 8 章）。差分は文書と 1 つのスクリプトだけで、各ステップのコミットが検査の失敗と成功を記録するので、1 つの PR でもレビューできる。

`推奨タイトル` は、本書を書いている時点の規則（`mkplan2.md:62` の `feat(<task-id>)` の英語）に従う。本タスクの後の `/runplan` は、英語の値からも日本語の PR のタイトルを書く（要件書 AC-05）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

追加しない（設計書 7.1）。Go のコードとテストを変えないので、`make test` と `make lint` が変更の前後で同じく通ることを、各フェーズの最後に確かめる。

### 4.2. 静的な検査

`docs/tasks/0032_japanese_pr_title/check_rules.sh` の名前の付いた検査で行う（1.4）。各検査は 1.2 の完了条件のとおり、変える前の失敗、変えた後の成功、性質ごとの失敗を確かめる。変える前から通る検査（`commit_subjects_unchanged`）と、正規表現を広げすぎた場合と 1 行の確かめを外した場合（`subject_regex_samples`）は、壊して確かめる手順をステップに書いた。`subject_regex_samples` は、節に書いた正規表現を実際に見本の件名に実行する。語の有無で確かめられない規則の意味は、ステップ 5-2 の突き合わせで確かめる。

### 4.3. 結合テスト（手動）

設計書 7.3 の確かめを、PR-1 のマージの後に行う。

-   **正常の経路:** 次の 3 つを観察する。次のタスクの文書のコマンドが開く PR のタイトル。英語の `推奨タイトル` から `/runplan` が書く PR のタイトル。タイトルが日本語の PR と英語の PR のそれぞれについて、`/mergepr` が書く件名。
-   **形の崩れた入力の試し:** 全角の文字・大文字・`!` 付きの接頭辞、`<scope>` がタスク ID と違うプランの PR、`$HOME` とバッククォートを含むタイトルの受け渡しを試す。
    -   テスト用の PR の作成とタイトルの変更は、利用者の承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。
    -   試した後は step 3 で断って `mergepr discard` で片付け、テスト用の PR を閉じ、そのブランチを消す。
-   結果は issue #408 のコメントに書く。PR-1 は本タスクの最後のプランの PR なので、そのマージで #408 は閉じる（`runpr.md` の step 4 の 1）。閉じた issue にもコメントは書ける。

### 4.4. 後方互換性

本タスクの前に開いた英語のタイトルの PR も、新しい `mergepr.md` で英語の件名を書けること（要件書 4.4、AC-12）を、4.3 の正常の経路に含める。既存のプランの英語の `推奨タイトル` は書き換えない（設計書 3.9）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`static` は `check_rules.sh` の検査（ファイルの記述の有無と位置）、`test` は文書に書いた正規表現を見本の入力に実行する検査、`manual` はマージの後の観察（4.3）を表す。検査の名前はすべて `docs/tasks/0032_japanese_pr_title/check_rules.sh` の中にある。本タスクの受け入れ基準は、コマンドの定義と規則の文書が何を述べるかを定めるので（要件書 1 節）、`static` で確かめる。エージェントが定義に従って動くことは、`manual` で補う。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | static | `check_rules.sh::claude_language` |
| AC-02 | static | `check_rules.sh::claude_language`、`::context_title_section` |
| AC-03 | static | `check_rules.sh::claude_language` |
| AC-04 | static + manual | `check_rules.sh::handoff_pr_text`、`::context_title_section`。4.3 の正常の経路 |
| AC-05〜AC-07 | static + manual | `check_rules.sh::runplan_title`、`::context_title_section`。4.3 の正常の経路 |
| AC-08 | static | `check_rules.sh::runpr_title` |
| AC-09 | static | `check_rules.sh::fixpr_title` |
| AC-10 | static | `check_rules.sh::commit_subjects_unchanged` |
| AC-11 | static + test | `check_rules.sh::mergepr_subject`、`::subject_regex_samples`（件名の形と ASCII の文字） |
| AC-12 | static + manual | `check_rules.sh::mergepr_subject`（3 つの規則とその順）、`::context_plan_pr`、`::runpr_title`（step 4 の 2 の列挙）、`::mergepr_guide_items`。4.3 の正常の経路と形の崩れた入力の試し |
| AC-13 | static + test + manual | `check_rules.sh::context_title_section`、`::context_subject_regex`（受け付ける形の定義）。`::subject_regex_samples` が確かめるのは、件名が受け付ける形の接頭辞を持つこと（AC-13 の最後の項目）である。エージェントが、タイトルの接頭辞を受け付けるか、ないものとして扱うかを正しく判断することは、4.3 の形の崩れた入力の試しで確かめる |
| AC-14 | static | `check_rules.sh::mergepr_subject` |
| AC-15 | static | `check_rules.sh::context_marker_conventions` |
| AC-16 | static | `check_rules.sh::context_marker_conventions`、`::context_title_section`、`::mkplan2_recommended_title` |
| AC-17 | static | `check_rules.sh::no_stale_rules`、`::title_via_file` |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 / 5-3）

マージの後に、次のことを行う。この項目は `main` の上で記録できないので、結果を issue #408 のコメントに書いたことをもって済みとする（4.3）。

- 4.3 の手動の確かめ（新しいセッションか、`origin/main` に進めた worktree で行う。設計書 8 章の移行の注意）

## 7. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| タスク 0030 の PR-6 と、6 つのファイル（1.3.3）を同時に変える | 進み具合を 3 回確かめ（ステップ 1-1・フェーズ 3 の前・ステップ 5-3）、答えが変わっていれば 1.3.3 のとおり重ね直して、そこに挙げたステップをやり直す。そのガードのテストが本タスクの文を固定していれば、止まって利用者に尋ねる（1.4） |
| 検査のスクリプトは CI で実行されないので、本タスクの後に古い記述が戻っても検出されない | 1.4 で受け入れた。形は 1 か所（タイトルの規約の節）にあり、ほかは参照するだけなので、戻る余地は小さい |
| タスク 0030 の後、`fixprguard` は `docs/tasks/**/*.sh` を実行に関わるファイルに分類しないので、レビューのスレッドを通じて `check_rules.sh` を変えさせられるおそれがある | `check_rules.sh` は本タスクの PR の中でだけ使い、CI でも実行しない。分類を広げるかはタスク 0030 の範囲なので、本タスクでは扱わない |
| 古い定義を読み込んだセッションや古い worktree で `/mergepr` を実行し、日本語の件名を提案する | 設計書 8 章の移行の注意を PR の本文に書く（ステップ 5-3）。件名は承認の質問で利用者が確かめる |
| エージェントが接頭辞を誤って読む | 設計書 5.4 のとおり、承認の前の形と ASCII の確かめ（ステップ 4-2）と、接頭辞を決めた規則の表示（ステップ 4-3）で、利用者が気づける |
| 検査の語が、正しい新しい記述にも当たる | 各検査は変えた後に通ることを確かめる。当たった場合は、検査の語を狭めるか、新しい記述の文面を変える。どちらを採ったかをコミットメッセージに書く |

## 8. 成功基準 (Success Criteria)

-   [ ] `check_rules.sh` を引数なしで実行し、0 で終わる（すべての AC の `static`・`test`）。
-   [ ] `make test && make lint` が通る。
-   [ ] ステップ 5-2 の突き合わせに、Critical と Major の指摘が残らない。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの後、4.3 の手動の確かめを行い、結果を issue #408 のコメントに書く。
-   本タスクの後に古い記述の再発が問題になれば、`check_rules.sh` の検査を Go の文書のガードに移すことを検討する（1.4）。
