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

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-005 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。変えるのは `CLAUDE.md`・`.claude/commands/` の下のコマンドの定義・`docs/translation_glossary.md` だけで、Go のコードとテストは変えない（設計書 1.1・7.1）。設計書 8 章の順序を、そのまま 5 つのフェーズにする。すべてのフェーズを 1 つの PR にまとめる（設計書 8 章）。

-   **フェーズ 1（検査のスクリプトと `_context.md`）:** 静的な検査のスクリプトを作り、タイトルの規約の節とプランの PR の判定を `_context.md` に設ける。
-   **フェーズ 2（`CLAUDE.md` と用語集）:** 言語の規則と用語を改める。
-   **フェーズ 3（PR のタイトルを書くコマンド）:** `_lib/task-pr-handoff.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mkplan2.md` を改める。
-   **フェーズ 4（`mergepr.md`）:** squash のコミットの件名の決め方を改める。
-   **フェーズ 5（全体の確かめ）:** 検査のスクリプトの全体と、ビルドの確認を通す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計書の規則を計画で言い換えない。コマンドの定義に書く文は英語で書く（`CLAUDE.md:47-48`）。
-   形は、設計書 3.1 のタイトルの規約の節にだけ書く。ほかのファイルは節の名前（`PR title and squash subject`）で参照する。例外は、`mkplan2.md` が挿入する PR マーカーのひな形だけである（設計書 1.1・3.9）。
-   検査は、コミットするスクリプト `docs/tasks/0032_japanese_pr_title/check_rules.sh` に置く（1.4）。各フェーズは、自分の変える箇所の検査をスクリプトに先に加え、ファイルを変える前に、その検査が失敗することを確かめる。変えた後に通ることを確かめ、両方をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。
-   各コマンドが作るコミットのメッセージは、英語のまま変えない（要件書 AC-10、設計書 3.4・3.8）。
-   変えた後に `make test` と `make lint` が変わらず通ることを、各フェーズの完了条件で確かめる（設計書 7.1）。

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
| `docs/translation_glossary.md:59` | 「材料」の備考 | 設計書 3.11 | 2-2 |

変えない箇所は次のとおりである。

-   `.claude/commands/mkreq.md:107` の `<concise English summary>` と `mkarch.md:76`・`mkplan.md:100` の `docs(NNNN): draft ...` は、コミットの件名なので変えない（要件書 AC-10、設計書 3.4）。
-   `.claude/commands/runpr.md:321` の `gh issue create --title "PR #<number> のレビュー指摘の追跡"` は、issue のタイトルの固定の文字列で、PR のタイトルでもエージェントが書く値でもないので変えない（要件書 2.3）。
-   `docs/dev/developer_guide/mergepr_guide.md` と `mergepr_guide.ja.md` は変えない（設計書 3.7 の末尾）。`state.json` の `title` は `internal/mergepr/mergepr.go:78` の `Title string \`json:"title"\`` で、ガイドの説明（`mergepr_guide.md:120`）と一致する。
-   Go のコードとテスト。`.claude/` を読むテストは `internal/fixprguard/scope_test.go:46` だけで、これはパスの分類の表の入力であり、コマンドの定義の文を読まない（`git grep -ln '\.claude' -- '*_test.go'`、`dd4ce5d`）。

#### 1.3.2. 外部の前提の確かめ

-   **`gh` のタイトルのフラグ:** `gh pr create --help`（`gh` 2.96.0）には `-t, --title string` だけがあり、タイトルをファイルから読むフラグはない。`--body-file` は本文だけに使える。設計書 3.1.1 のコマンド置換の方法をそのまま使う。
-   **ASCII の確かめの方法:** 設計書 3.7 が実装プランに委ねた「どう確かめるか」は、件名のファイルに対する 1 回の `LC_ALL=C grep -Eqx` にする（1.4）。件名の形の正規表現は、タイトルの規約の節の 11 個の `<type>` と `<scope>` の文字の範囲から組み、要約の部分は印字可能な ASCII 文字（空白で始まらず、空白で終わらない）とし、最後を ` (#<number>)` とする。この方法で、次の 11 個の件名を試した（`dd4ce5d` の時点の手元の環境。`/usr/bin/grep`（BSD grep 2.6.0-FreeBSD）と ugrep 7.8.4 で同じ結果）。受け付けたのは `docs(0032): draft the plan (#413)` と `fix: tidy (#1)` の 2 つで、全角の括弧・全角のコロン・キリル文字の `о`・日本語の要約・大文字の `Feat`・`!` 付き・コロンの後の 2 つの空白・要約の中の全角空白・`(#` の前の 2 つの空白の 9 つを拒んだ。正規表現の文字どおりの形は、ステップ 4-2 で書くときに決める。
-   **検査のスクリプトの実行環境:** 開発の環境の `/bin/sh` は bash 3.2.57 で、`grep` はシェルの関数で ugrep に置き換えられていることがある（`type grep` で確かめた）。スクリプトは POSIX の `sh` で書き、ファイルの検索には `git grep` を使い、ASCII の確かめの試しにだけ `LC_ALL=C grep` を使う。

#### 1.3.3. ほかのタスクとの重なり

-   **タスク 0030 の PR-6（未着手）:** `fixpr.md` と `runpr.md` を `fixprguard` を呼ぶ形に切り替える（`docs/tasks/0030_fixpr_untrusted_threads/03_implementation_plan.md` のフェーズ 6）。その設計書は、`fixpr.md` の Phase 6 のエージェントにタイトルと本文の案を JSON で返させ、`fixprguard edit-pr` がタイトルのファイルを読んで `gh pr edit` をシェルなしで実行する形にする（同 `02_architecture.md:667`）。また、`cmd/yt2column/fixpr_docs_test.go` に `fixpr.md` と `runpr.md` の文のガードを加える。本タスクのステップ 3-3・3-4 は、その PR が先にマージされているかで手順が分かれる（ステップ 3-4 に書く）。
-   **タスク 0031:** 2026-10-21 の時点で、要件書（PR #407）だけがマージされ、PR #411（文書だけ）が開いている。本タスクが変える段落とは重ならない（設計書 3.10 の H-05）。
-   どちらのタスクについても、先に `main` に入った変更の上に重ね、相手の記述を書き換えない（設計書 3.10 の H-05）。実装を始めるとき、`git log origin/main` で両タスクの進み具合を確かめる（ステップ 1-1）。

#### 1.3.4. 実装時に決めること

-   コマンドの定義に書く英語の文面。設計書 3 章の規則の内容を変えない範囲で決める。
-   検査のスクリプトの、各検査が探す語と、探す範囲（ファイル全体か、特定の step の段落か）。各検査は、1.4 と 5 章の表に挙げた性質を確かめる。
-   ASCII の確かめの正規表現の文字どおりの形（1.3.2）。

### 1.4. 計画の際に決めたこと（2026-10-21）

-   **静的な検査は、タスクのディレクトリに置くスクリプトで行う:** `docs/tasks/0032_japanese_pr_title/check_rules.sh` をコミットし、実装の PR の中で実行して結果を記録する。設計書 7.1（テストを追加しない）と 7.2（`git grep` で確かめる）に合い、設計書を改める必要がないためである。Go のガードのテストにして CI で古い記述の再発を防ぐ案は採らない。設計書 7.1 と食い違い、設計書の承認をやり直すことになるためである。このスクリプトは CI では実行されないので、本タスクの後に古い記述が戻っても検出されない（7 章のリスク）。
-   **ASCII の確かめは、件名のファイルに対する `LC_ALL=C grep -Eqx` とする:** 1.3.2 のとおり、手元の 2 つの `grep` で同じ結果になることを確かめた。新しい道具やバイナリの変更を要しない（設計書 9 章の YAGNI）。検査のスクリプトは、`mergepr.md` に書いたこのコマンドを取り出して見本の件名に実行し、受け付けと拒否を確かめる（ステップ 4-2）。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-05 は、すべて設計書 3.10 に記録されている。H-05（タスク 0031 との重なり）は 1.3.3 で実装の手順に落とした。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 検査のスクリプトと `_context.md`（設計書 8 章の 1）

#### ステップ 1-1: 検査のスクリプトの枠組み

**変更するファイル**: `docs/tasks/0032_japanese_pr_title/check_rules.sh`（新規）

-   [ ] `git log origin/main` で、タスク 0030 の PR-6 とタスク 0031 の実装が `main` に入っているかを確かめ、結果をコミットメッセージに書く（1.3.3）。
-   [ ] POSIX の `sh` で書き、リポジトリの最上位で `sh docs/tasks/0032_japanese_pr_title/check_rules.sh` と実行する形にする。名前の付いた検査を順に実行し、失敗した検査の名前と、見つかった行（または見つからなかった記述）を表示し、1 つでも失敗すれば 0 以外で終わる。引数で検査の名前を渡すと、その検査だけを実行する（各フェーズで、自分の検査だけを先に失敗させて確かめるため）。
-   [ ] ファイルの検索は `git grep` で行い、`docs/tasks/` を対象から外す（要件書 AC-17 の範囲）。

**完了条件**: 検査を 1 つも持たないスクリプトが 0 で終わり、存在しない検査の名前を渡すと 0 以外で終わる。

#### ステップ 1-2: タイトルの規約の節

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_title_section` を加える。`_context.md` に節「PR title and squash subject」があり、その節が、設計書 3.1 の表の 6 項目（PR のタイトル・受け付ける接頭辞・タスクの文書の PR のタイトル・プランの PR のタイトル・`推奨タイトル`・squash のコミットの件名）を述べ、11 個の `<type>` のすべてと `<scope>` の文字の範囲を挙げ、合わない接頭辞を直さずにないものとして扱うと述べることを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.1 の表と、受け付ける接頭辞の表（合わない例を含む）、3.1.1 のタイトルの渡し方とバッククォートの規則を、「Process convention」の中に「PR marker conventions」と並べて書く。PR のタイトルは `<scope>` のある形だけとする（設計書 3.1 の表の 1 行目）。

#### ステップ 1-3: プランの PR の判定

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_plan_pr` を加える。`_context.md` にプランの PR の判定の定義があり、head のツリーを読むこと、URL 全体の一致、2 つ以上のタスクで一致したときと読めないときに止まること、最後の PR の規則を述べることを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.2 の定義を書く。中身は `runpr.md:105-108` と同じにし、読むツリーと止まる 2 つの場合を加える。`runpr.md` 側の置き換えはステップ 3-3 で行う。

#### ステップ 1-4: PR マーカーの規約の `推奨タイトル`

**変更するファイル**: `.claude/commands/_context.md`、`check_rules.sh`

-   [ ] 検査 `context_marker_conventions` を加える。「PR marker conventions」が、`推奨タイトル` は PR のタイトルであって squash のコミットの件名ではないと述べ、タイトルの規約の節を参照することを確かめる。変える前に失敗することを確かめる。
-   [ ] `_context.md:62` の行を、設計書 3.9 の表の 1 行目のとおりに改める。

**完了条件**: `sh check_rules.sh context_title_section context_plan_pr context_marker_conventions` が 0 で終わる。`make test && make lint` が通る。

### フェーズ 2: `CLAUDE.md` と用語集（設計書 8 章の 2）

#### ステップ 2-1: `CLAUDE.md` の「Language」

**変更するファイル**: `CLAUDE.md`、`check_rules.sh`

-   [ ] 検査 `claude_language` を加える。「Language」の節が、PR のタイトルを GitHub に置く日本語のものに含め、PR のタイトルを英語にする例外を述べず、タイトルの規約の節を参照し、squash のコミットの件名と本文を英語で書き、件名を `/mergepr` が PR の材料から書くと述べることを確かめる。bot のコマンドの例外が残ることも確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.3 のとおりに改める。

#### ステップ 2-2: 用語集

**変更するファイル**: `docs/translation_glossary.md`、`check_rules.sh`

-   [ ] 検査 `glossary_terms` を加える。設計書 3.11 の 4 語（接頭辞・受け付ける接頭辞・要約・プランの PR）が英訳とともにあり、「材料」の備考が diff を含むことを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.11 のとおりに加え、改める。英訳は、コマンドの定義で使う英語の語（`prefix`・`accepted prefix`・`summary`・`plan PR`）と一致させる。

**完了条件**: `sh check_rules.sh claude_language glossary_terms` が 0 で終わる。`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline`（`CLAUDE.md` を読む文書のガード）が通る。

### フェーズ 3: PR のタイトルを書くコマンド（設計書 8 章の 3）

#### ステップ 3-1: `_lib/task-pr-handoff.md`

**変更するファイル**: `.claude/commands/_lib/task-pr-handoff.md`、`check_rules.sh`

-   [ ] 検査 `handoff_pr_text` を加える。PR_TEXT の定義がタイトルの規約の節のタスクの文書の PR のタイトルを参照し、Part B の step 3 がタイトルを英語とせず、タイトルを Write ツールでファイルに書いてコマンド置換で渡すことを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.4 と 3.1.1 のとおりに改める。`runplan.md` についての PR_TEXT の記述も、設計書 3.4 のとおりに改める。

#### ステップ 3-2: `runplan.md`

**変更するファイル**: `.claude/commands/runplan.md`、`check_rules.sh`

-   [ ] 検査 `runplan_title` を加える。step 5a が、`推奨タイトル` を下書きとして扱い、接頭辞の 3 つの場合（`<scope>` のある受け付ける接頭辞・`<scope>` のない受け付ける接頭辞・受け付ける接頭辞がない）と日本語の要約と、プランを書き換えないことを述べ、step 8 がタイトルの規約の節のプランの PR のタイトルを参照することを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.5 のとおりに改める。`推奨タイトル` の言語を判定する文を書かない（設計書 1.1、H-03）。

#### ステップ 3-3: `runpr.md`

**変更するファイル**: `.claude/commands/runpr.md`、`check_rules.sh`

-   [ ] 検査 `runpr_title` を加える。step 1 がタイトルを PR のタイトルの形の日本語で書き、`<type>(<scope>)` を `git log origin/main` の形に合わせ、ファイルから渡すこと、step 2 が `_context.md` のプランの PR の判定を参照すること、step 4 の 2 の列挙が接頭辞を決めた規則の記録を含むことを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.6 の `runpr.md` の 3 項目のとおりに改める。step 2 の「Plan PR」の文は `_context.md` の参照に置き換え、PR の種類の判定の残り（Task-document PR・Standalone change）は変えない（設計書 3.8）。
-   [ ] タスク 0030 の PR-6 が先にマージされていれば、その後の `runpr.md` の該当の段落に同じ変更を加える。その PR のガード（`cmd/yt2column/fixpr_docs_test.go`）が通ることを `make test` で確かめる。

#### ステップ 3-4: `fixpr.md` の Phase 6

**変更するファイル**: `.claude/commands/fixpr.md`、`check_rules.sh`

-   [ ] 検査 `fixpr_title` を加える。Phase 6 のプロンプトが `.claude/commands/_context.md` の節「PR title and squash subject」を名指しして読ませ、新しいタイトルをその節の PR のタイトルの形で書かせ、タイトルを書き直す条件（本文が大きく古くなったとき、具体的な文を書けたとき）を保つことを確かめる。変える前に失敗することを確かめる。
-   [ ] タスク 0030 の PR-6 がまだマージされていなければ: 設計書 3.6 の `fixpr.md` の項目のとおりに改め、`gh pr edit` のタイトルを 3.1.1 のとおりファイルから渡す。
-   [ ] タスク 0030 の PR-6 が先にマージされていれば: Phase 6 のエージェントがタイトルの案を書く箇所に、上の検査の内容を加える。タイトルは `fixprguard edit-pr` がファイルから読んでシェルなしで渡すので、3.1.1 の目的はすでに満たされている。コマンド置換を加えない。その PR のガードが通ることを `make test` で確かめる。どちらの場合を採ったかをコミットメッセージに書く。

#### ステップ 3-5: `mkplan2.md`

**変更するファイル**: `.claude/commands/mkplan2.md`、`check_rules.sh`

-   [ ] 検査 `mkplan2_recommended_title` を加える。挿入する書式が `` `<type>(<task-id>): <Japanese summary>` `` で、欄の説明がタイトルの規約の節を参照し、レビューのチェックリストが形とバッククォートを含まないことを確かめると述べることを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.9 の表の 2〜4 行目のとおりに改める。

#### ステップ 3-6: コミットのメッセージ

**変更するファイル**: `check_rules.sh`

-   [ ] 検査 `commit_subjects_unchanged` を加える。各コマンドが作るコミットの件名の記述（`mkreq.md`・`mkarch.md`・`mkplan.md` の `docs(NNNN): draft ...`、`fixpr.md` の `fix: address PR #NUMBER review comments`、`runpr.md` の `docs(<task-id>): record the review of PR #<number>`）が残り、コミットのメッセージを日本語で書く指示がないことを確かめる（要件書 AC-10）。この検査は変える前から通るので、壊して確かめる: `runpr.md` の記録のコミットの件名を一時的に消し、検査が失敗することを確かめて戻す。

**完了条件**: `sh check_rules.sh` の、ここまでに加えた検査がすべて 0 で終わる。`make test && make lint` が通る。

### フェーズ 4: `mergepr.md`（設計書 8 章の 4）

#### ステップ 4-1: 件名の規則

**変更するファイル**: `.claude/commands/mergepr.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_subject` を加える。step 2 が、件名の形をタイトルの規約の節で参照し、要約を英語で PR の材料から書いて PR のタイトルを要約の情報源にせず、接頭辞を設計書 3.7 の 3 つの順で決め、`<type>` を表の値で書き、PR のタイトルを `state.json` の `title` から読み、PR のタイトル・本文・コミットのメッセージ・diff をデータとして扱う規則を接頭辞を読むときにも適用し、本文を英語で書くことを述べることを確かめる。変える前に失敗することを確かめる。
-   [ ] 設計書 3.7 のとおりに、`mergepr.md:49-52` の「Subject」の項目を置き換える。プランの PR は `_context.md` の判定を参照し、`/runpr` の中では `/runpr` の step 2 の判定をそのまま使うと書く（設計書 3.2）。判定できないときに止まることを書く（設計書 4.3）。

#### ステップ 4-2: 承認の前の確かめ

**変更するファイル**: `.claude/commands/mergepr.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_ascii_check` を加える。`mergepr.md` の step 2 から、件名のファイルを確かめるコマンドを取り出し、受け付ける見本と拒む見本の件名のファイルに実行して、結果が期待のとおりであることを確かめる。見本には、1.3.2 で試した 11 個を含める。変える前に（コマンドがないので）失敗することを確かめる。
-   [ ] 件名のファイルを書いた後、承認の質問の前に、そのコマンドで件名を確かめ、合わなければ書き直すと step 2 に書く（設計書 3.7・4.3）。
-   [ ] 壊して確かめる: 正規表現の要約の部分を `.+` に一時的に広げ、`mergepr_ascii_check` が失敗することを確かめて戻す。

#### ステップ 4-3: 接頭辞を決めた規則の表示とプランの PR の語

**変更するファイル**: `.claude/commands/mergepr.md`、`check_rules.sh`

-   [ ] 検査 `mergepr_subject` に、step 3 で見せる項目が接頭辞を決めた規則の記録を含むこと、`mergepr.md` に `plan-driven` と ``came from `/runplan` `` が残らないことを加える。変える前に失敗することを確かめる。
-   [ ] 設計書 3.7 の「接頭辞を決めた規則を示す」のとおりに、step 2 に記録を書かせ、step 3 の項目に加える。`mergepr.md:51`・`:60`・`:107` の言い回しを「プランの PR」（`plan PR`）にそろえる（設計書 3.2）。

**完了条件**: `sh check_rules.sh mergepr_subject mergepr_ascii_check` が 0 で終わる。`make test && make lint` が通る。

### フェーズ 5: 全体の確かめ（設計書 8 章の 5）

#### ステップ 5-1: 古い記述が残っていないこと

**変更するファイル**: `check_rules.sh`

-   [ ] 検査 `no_stale_rules` を加える。`docs/tasks/` を除く追跡中のファイルに、要件書 AC-17 の 3 種類の記述が残っていないことを確かめる。探す語は、1.3.1 の数え上げの語のうち古い規則を表すもの（英語のタイトル、件名をタイトルから作る、`feat` に限る `推奨タイトル`）とする。英語の `推奨タイトル` を入力として受け付ける説明（要件書 AC-05）は、探す語に当たらない文面で書く。
-   [ ] 検査 `title_via_file` を加える。`gh pr create` と `gh pr edit` の行に、タイトルを二重引用符の中に直接置く形（`--title "<`）が残っていないことを確かめる。`runpr.md:321` の issue のタイトルは対象の外である（1.3.1）。
-   [ ] 両方の検査が、ステップ 1-1 のコミット（変える前のファイル）では失敗することを、`git stash` を使わずに確かめる（たとえば、そのコミットを別の worktree に checkout して実行する）。

#### ステップ 5-2: 全体の実行と PR の準備

-   [ ] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh` を引数なしで実行し、0 で終わることを確かめる。出力をコミットメッセージに書く。
-   [ ] `make test && make lint` を実行する。
-   [ ] PR の本文に、設計書 8 章の「移行の注意」（マージの後、ほかの worktree を `origin/main` に進めるか、セッションをやり直してから `/mergepr`・`/runpr` を実行する）を書く。

**完了条件**: 上の 3 つ。

### PR-1 作成ポイント: Japanese PR titles and English squash subjects

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2

**推奨タイトル**: `feat(0032): write PR titles in Japanese and keep squash subjects in English`

**レビュー観点**: タイトルの規約の節が設計書 3.1・3.2 と一致し、ほかのファイルが形を書き写さずに参照していること / `mergepr.md` の step 2 の接頭辞の決め方と承認の前の確かめが設計書 3.7 のとおりであること / 古い記述（英語のタイトル、タイトルから件名を作る）が残っていないこと / タスク 0030 の PR-6 との重なりの扱い（ステップ 3-3・3-4）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1〜4-3 は、マージのたびにエージェントが読んで従う件名の決め方を置き換える、隔離された高リスクの変更である（誤るとどの PR の件名にも効く）。ステップ 3-3・3-4 は、タスク 0030 の進み具合で手順が 2 つに分かれる。Go のコードは変えず、検査はスクリプトが担うので、`required` にはしない。

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
| M4 | `mergepr.md` の件名の決め方 | フェーズ 4 の完了条件 |
| M5 | 全体の検査と PR | PR-1 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 | タイトルの規約の節、`CLAUDE.md`、用語集、PR のタイトルを書くコマンド、`mergepr.md`、検査のスクリプト | frontier-recommended |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。ほかのすべての変更がタイトルの規約の節とプランの PR の判定を参照するので、フェーズ 1 を最初に置く。検査のスクリプトの枠組みもフェーズ 1 に置き、各フェーズが自分の検査を先に加えられるようにする。PR を 1 つにするのは、コマンドの定義がマージされた時点でそのまま次のセッションの振る舞いになり、途中の状態を `main` に置くと規則が互いに矛盾するためである（設計書 8 章）。差分は文書だけで、1 つの PR でもレビューできる量である。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

追加しない（設計書 7.1）。Go のコードとテストを変えないので、`make test` と `make lint` が変更の前後で同じく通ることを、各フェーズの完了条件で確かめる。

### 4.2. 静的な検査

`docs/tasks/0032_japanese_pr_title/check_rules.sh` の名前の付いた検査で行う（1.4）。各検査は、それを加えるステップで、変える前に失敗し、変えた後に通ることを確かめる。変える前から通る検査（`commit_subjects_unchanged`）と、正規表現の広げすぎ（`mergepr_ascii_check`）は、壊して確かめる手順をステップに書いた。`mergepr_ascii_check` は、文書に書いたコマンドを実際に見本の件名に実行するので、誤った正規表現で失敗する。

### 4.3. 結合テスト（手動）

設計書 7.3 の、マージの後の確かめを行う。正常の経路（次のタスクの文書のコマンドが開く PR のタイトル、英語の `推奨タイトル` からの `/runplan` の PR のタイトル、日本語と英語のタイトルの PR の件名）と、形の崩れた入力の試し（全角の文字・大文字・`!` 付きの接頭辞、`<scope>` がタスク ID と違うプランの PR、`$HOME` とバッククォートを含むタイトルの受け渡し）である。形の崩れた入力の試しはテスト用の PR で行い、step 3 で断って `mergepr discard` で片付ける。結果は、本タスクの PR-1 の後の最初の該当する PR で観察し、この計画書の 8 章のチェックボックスに記録する。

### 4.4. 後方互換性

本タスクの前に開いた英語のタイトルの PR も、新しい `mergepr.md` で英語の件名を書けること（要件書 4.4、AC-12）を、4.3 の手動の確かめに含める。既存のプランの英語の `推奨タイトル` は書き換えない（設計書 3.9）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`static` は `check_rules.sh` の検査（ファイルの記述の有無）、`test` は文書に書いたコマンドを見本の入力に実行する検査、`manual` はマージの後の観察（4.3）を表す。検査の名前はすべて `docs/tasks/0032_japanese_pr_title/check_rules.sh` の中にある。本タスクの受け入れ基準は、コマンドの定義と規則の文書が何を述べるかを定めるので（要件書 1 節）、`static` で確かめる。エージェントが定義に従って動くことは、`manual` で補う。

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
| AC-11 | static + test | `check_rules.sh::mergepr_subject`、`::mergepr_ascii_check` |
| AC-12 | static + manual | `check_rules.sh::mergepr_subject`、`::context_plan_pr`、`::runpr_title`（step 4 の 2 の列挙）。4.3 の正常の経路と形の崩れた入力の試し |
| AC-13 | static + test + manual | `check_rules.sh::context_title_section`、`::mergepr_ascii_check`。4.3 の形の崩れた入力の試し |
| AC-14 | static | `check_rules.sh::mergepr_subject` |
| AC-15 | static | `check_rules.sh::context_marker_conventions` |
| AC-16 | static | `check_rules.sh::context_marker_conventions`、`::context_title_section`、`::mkplan2_recommended_title` |
| AC-17 | static | `check_rules.sh::no_stale_rules`、`::title_via_file` |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2）

## 7. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| タスク 0030 の PR-6 と `fixpr.md`・`runpr.md` の同じ段落を変える | ステップ 1-1 で進み具合を確かめ、ステップ 3-3・3-4 の 2 つの手順のどちらかを採る。本タスクが先にマージされたときは、タスク 0030 の PR-6 が本タスクの規則（タイトルの規約の節の参照、AC-09）を保つ必要があることを、本タスクの PR の本文に書く |
| 検査のスクリプトは CI で実行されないので、本タスクの後に古い記述が戻っても検出されない | 1.4 で受け入れた。規則は 1 か所（タイトルの規約の節）にあり、ほかは参照するだけなので、戻る余地は小さい |
| 古い定義を読み込んだセッションや古い worktree で `/mergepr` を実行し、日本語の件名を提案する | 設計書 8 章の移行の注意を PR の本文に書く（ステップ 5-2）。件名は承認の質問で利用者が確かめる |
| エージェントが接頭辞を誤って読む | 設計書 5.4 のとおり、承認の前の形と ASCII の確かめ（ステップ 4-2）と、接頭辞を決めた規則の表示（ステップ 4-3）で、利用者が気づける |
| 検査の語が、正しい新しい記述にも当たる | 各検査は変えた後に通ることを確かめる。当たった場合は、検査の語を狭めるか、新しい記述の文面を変える。どちらを採ったかをコミットメッセージに書く |

## 8. 成功基準 (Success Criteria)

-   [ ] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh` が 0 で終わる（すべての AC の `static`・`test`）。
-   [ ] `make test && make lint` が通る。
-   [ ] 4.3 の手動の確かめの結果を記録した（正常の経路、形の崩れた入力の試し）。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの後、4.3 の手動の確かめを、マージの後に初めて実行するタスクの文書のコマンドと `/runplan`・`/mergepr` で行い、8 章に記録する。
-   本タスクの後に古い記述の再発が問題になれば、`check_rules.sh` の検査を Go の文書のガードに移すことを検討する（1.4）。
