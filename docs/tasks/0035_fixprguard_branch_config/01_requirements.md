# 要件定義書：fixprguard の config の比較から、ほかのブランチの `branch.<name>.*` を外す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ガードツール `fixprguard` が git の config を基準値と比べるときに、ほかのブランチの `branch.<name>.*` の設定を比べる対象から外すための要件を定義する（[#431](https://github.com/isseis/yt2column/issues/431)）。外すのは、今のブランチと `main` のどちらでもないブランチの設定である。それ以外の設定は今までどおり厳密に比べ、違えば止まる。

**背景:**

`fixprguard` は、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で作られた、`/fixpr` と `/runpr` のガードツールである。`begin` が基準値を取り、`inspect`・`finish`・`push` が git の状態を基準値と比べる（`docs/dev/developer_guide/fixprguard_guide.md` 5 章）。

config については、今は次の 2 つを比べている。

-   git のディレクトリの場所（`.git` ファイルと `commondir` が指す先）と、リポジトリの config・worktree の config のファイルのバイト列のダイジェスト（`internal/fixprguard/gitstate.go:279-298` の `configDigest`）。
-   `git config --list --show-origin -z` の出力全体のダイジェスト（同 `:484-494` の `configListDigest`）。git が読むすべての設定の値を、読み込み元のファイルとともに含む。

比べるのは `CheckState`（同 `:514-561`）で、`inspect`・`finish`・`push` が git や `gh` を起動する前に呼ぶ。順序は、ファイルのバイト列の比較、`git config --list` の比較、hooks の場所を求める `git rev-parse` の順である。設定が書き換えられたまま git を起動しないための順序である（0030 `02_architecture.md` 3.7）。

リポジトリの config は、すべての worktree が共有している。そのため、並行して動くほかのセッションがブランチを操作するだけで、このセッションの `inspect`・`finish`・`push` が `git-state-changed`（`parts` は `config`）で止まる。issue #431 は、config に書き込む操作として次を挙げている。

| 操作 | config への書き込み |
|---|---|
| `git worktree add -b X origin/main` | `branch.X.remote`・`branch.X.merge` を加える |
| `git push -u`、`gh pr checkout` | `branch.X.*` を加える |
| `git branch -d X`（`/mergepr` の片付けを含む） | `[branch "X"]` の節を消す |

PR #429 の `/fixpr` は、これが原因で Phase 3.5 で止まった。止まると、修正のフェーズのエージェントの変更を捨ててやり直すことになる。worktree を並べて作業すると、この手間が繰り返し生じる。

対応 issue: [#431](https://github.com/isseis/yt2column/issues/431)（関連: [#395](https://github.com/isseis/yt2column/issues/395)（タスク 0030）、[#424](https://github.com/isseis/yt2column/issues/424)（タスク 0034、PR #429））

**本書の記述範囲:** 本書は、何を比べ、何を比べないか、止まるときと止まらないときに利用者から何が見えるかを定める。次の事項は設計（`02_architecture.md`）で決める。

-   設定の値の一覧（3.1）の求め方と、今のブランチの名前の求め方。
-   起動の順序の保証を保つ方法（[design_handoff.md](design_handoff.md) の H-01）。
-   基準値のトークンの形。
-   テストの手段。

AC-10 は、ファイルのバイト列をそのまま比べることを許さない。そのため、ファイルのバイト列の比較は、今の形のままでは残せない。issue #431 は Go で config のファイルを解析することを避けたいとしているので、設計の第一の案は、バイト列の比較をやめることである。設計で起動の順序の保証（AC-14）を確かめられない場合は、設計を進めずにレビュアーに戻す。

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #431 で決めたこと:**

-   **ほかのブランチの `branch.<name>.*` を比べない:** issue の理由は、ほかのブランチの upstream の設定はこのブランチの操作に効かないので、外しても保証は弱まらない、というものである。ただし、ツールの実行の後に残る影響については、要件定義の際に `main` を例外とした（下の決定）。
-   **今のブランチの `branch.*` と、それ以外の設定は、今までどおり厳密に比べる:** 今のブランチの設定は、`push` の宛先に効くためである（`internal/fixprguard/gitstate.go:617-650` の `pushTarget`・`resolvePushTarget` は、`branch.<b>.pushRemote`・`branch.<b>.remote` を読む）。
-   **git の場所の比較を残す:** `.git` ファイルや `commondir` を書き換えて、リポジトリの複製を指させる変更を検出するためである（`gitstate.go:279-282`）。
-   **内容の比較は `git config --list -z` の出力で行う方向で検討する:** Go で config のファイルを解析すると壊れやすいからである。今のバイト列の比較は、危険な設定（`core.fsmonitor` など）を git の起動前に見つけるために行っている。`git config` はコマンドを起動しないので、`config --list` を先に行っても同じ保証を保てるかを、設計で確かめる。
-   **文書を改める:** ガイド（`fixprguard_guide.md`・`fixprguard_guide.ja.md`）と、0030 の文書の保証・残余のリスクの記述を改める。
-   **`Protocol` の扱い:** 出力または引数の契約が変わる場合は、ガイド 9 章に従って `Protocol` を上げる。
-   **進め方:** タスク 0034（#424、git が無視するファイルの検出）より先に、実装まで終えてマージする。0034 の PR #429 は、それまで止めておく。

**要件定義の際に決めたこと（2026-10-22）:**

-   **`main` のブランチの設定は比べ続ける:** `branch.main.*` は、今のブランチでなくても、今までどおり比べる（AC-06）。理由: ほかのブランチの設定は、ツールが実行の間に起動する git には効かない。しかし、修正のフェーズのエージェントや build checks のコードが書き換えた設定は、実行の後にも残る。例えば `branch.main.pushRemote` や `branch.main.remote` に URL を書けば、後で `main` の上で行う `git push`・`git pull` の宛先が変わる。このとき `remote.*` は変わらないので、`remote.*` の比較では見つからない。また、`/mergepr` は、同じ worktree で `main` に切り替えて `git merge --ff-only` を実行する（`internal/mergepr/mergepr.go:357-362`）。`git merge` は `branch.main.mergeOptions` を読む。一方、並行するセッションのブランチの操作（issue #431 の表）は、`main` の設定には書き込まない。そのため、`main` を比べ続けても issue の目的は果たせる。`main` 以外のほかのブランチの書き換えは、残余のリスクとして受け入れる（4.2）。
-   **設定の値の一覧を変えない編集では止めない:** config のファイルの、コメント・空行・字下げだけの編集のように、設定の値の一覧（3.1）を変えない編集は、止める対象にしない（AC-10）。理由: こうした編集では、git の振る舞いは変わらず、コマンドも起動されない。また、ファイルのバイト列で比べ続けると、ほかのブランチの節の追加と削除もバイト列を変えるので、本タスクの目的を果たせない。
-   **読み込み元を問わず除外する:** 外す対象のブランチの設定は、どのファイル（システム・グローバル・include 先・リポジトリ・worktree の config）にあっても比べない（AC-03）。理由: 通常は、ブランチの操作はリポジトリの config に書き込む。ただ、ほかのファイルにあっても、効き方は同じである。読み込み元で区別しないほうが規則も単純である。
-   **空のサブセクションも外す:** `[branch ""]` の設定（`<name>` が空のブランチの設定）は、外す対象のブランチの設定として扱い、拒否もしない（3.1・3.3）。理由: git の構文として正しく、空の名前のブランチは作れないので、git がこれを使うことはない。拒否すると、正しい config なのに `git-failed` で止まることになる。
-   **`begin` の後に config が読めなくなったら config の変化として止める:** `begin` では読めた設定の値の一覧が、比較の時点で読めない場合（構文の誤り、FIFO やデバイスを指す include など）は、`git-state-changed`（`parts` は `config`）で止まる（AC-12）。理由: 今は、ファイルのバイト列の比較がこれを `config` の違いとして止めている。`git-failed` で止まると、ガイド 8 章の案内（`git status` が動くか確かめる）では、利用者が原因に辿り着けない。
-   **前の版の基準値を受け付けない:** 本タスクの前の版の `begin` が出力した基準値を渡されたら、`inspect`・`finish`・`push` は終了コード `2` で拒否する（AC-15）。理由: 比べる内容の意味が変わる。そのため、前の版の基準値を黙って受け付けると、どの規則で比べたのかが分からなくなる。タスク 0034 の要件案（PR #429 の AC-03）とも揃う。
-   **`Protocol` を必ず上げる:** issue #431 は、契約が変わる場合に上げるとしていた。本タスクでは、条件を付けずに上げる（AC-19）。理由: AC-15 で前の版の基準値を拒否するので、`--baseline` が受け付ける形と、`begin` の出力が必ず変わる。これは、ガイド 9 章の「出力または引数の契約の変更」に当たる。上げないと、改めたコマンドの文書から古いバイナリを呼んでも `--protocol` の検査を通り、古いバイナリが前の規則で比べ続ける。
-   **0030 の承認済みの文書は、本文を改めて `approved` を保つ:** 0030 の `01_requirements.md`・`02_architecture.md` の該当する記述を改める。そのうえで、`Comments` に、タスク 0035（#431）の要件で決めた変更であることを記録する（AC-17）。理由: 変更の決定は、本書の承認で得る。そのため、0030 の文書を改めて承認し直す必要はない。どの記述が、いつ、なぜ変わったかは `Comments` から辿れる。これは文書の改訂の規則の例外である（5 章）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   並行して動くほかのセッションがほかのブランチを操作しても、このセッションの `/fixpr`・`/runpr` が `git-state-changed` で止まらないようにする。
-   今のブランチと `main` の設定と、ブランチの設定でないすべての設定の変化は、今までどおり検出して止める。
-   書き換えられた設定のまま git・`gh`・make を起動しない、という 0030 の保証を保つ。

### 2.2. スコープ (In Scope)

-   `inspect`・`finish`・`push` の config の比較から、外す対象のブランチの設定を外すこと（F-001）。
-   それ以外の設定と、git のディレクトリの場所の比較を保つこと（F-002）。
-   前の版の基準値の拒否と、出力の契約（F-003）。
-   ガイド（英語・日本語）と、0030 の `01_requirements.md`・`02_architecture.md` の記述の改訂（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **hooks・info・HEAD・push 先の比較の変更。** issue #431 が挙げたのは config の比較だけである。
-   **ブランチの設定以外で、並行するセッションが config に書き込む変更。** 例えば `remote.<name>.*` の追加である。これらは今までどおり止まる（AC-08）。理由: issue #431 は `branch.<name>.*` に限っている。また、`remote.*` は、`remote.pushDefault` などを介して今のブランチの push 先に効きうる。
-   **基準値を入力に持たない `head`・`list`・`fence`・`reply`・`edit-pr`。** 今も比較しない（0030 `02_architecture.md` 5.3「基準値を持たないサブコマンドの起動前の比較」）。本タスクも変えない。
-   **タスク 0034（#424）の要件。** 0034 の要件案は、`gitstate.go:98-113` を引いて基準値の内容を説明している。本タスクで基準値が変わったら、0034 の側で、PR #429 を再開するときに合わせる。
-   **`/fixpr`・`/runpr` のコマンドの文書（`.claude/commands/`）の手順。** 止まり方の種類と `parts` の値は変わらないので、手順は変えない。`Protocol` を上げるので、コマンドの文書が渡す `--protocol` の値を改めることだけは、F-004 に含む（AC-19）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **ブランチの設定:** 名前が `branch.<name>.<key>` の形の設定。`<name>` は git の config のサブセクションで、最初の `.` と最後の `.` の間の部分である。`<name>` は `.` を含みうる（例: `branch.feature.a.remote` の `<name>` は `feature.a`）。`<name>` は空でもよい（`[branch ""]`）。`branch.autoSetupMerge` のように `<name>` を持たない 2 段の名前は、ブランチの設定ではない。
-   **今のブランチ:** `begin` が基準値を取ったときに HEAD が指していたブランチ。`begin` は、HEAD がブランチを指していないときは終了コード `2` で止まるので（`gitstate.go:719-721`、ガイド 2 章）、今のブランチは常に定まる。比較の時点で HEAD が別のブランチを指している、または detached になっている場合は、AC-13 のとおり止まる。
-   **外す対象のブランチの設定:** ブランチの設定のうち、`<name>` が今のブランチの名前とも `main` とも一致しないもの。一致は、git が出力する `<name>` のバイト列で判定する。大文字・小文字の違いも、違う名前として扱う。そのブランチが存在しない場合の設定も、外す対象に含む。
-   **設定の値の一覧:** git が読むすべての設定の項目を、git が読む順序で並べたもの。各項目は、読み込み元、設定の名前、値（値を持たない項目もある）から成る。今の `configListDigest` がダイジェストを取る対象である（`gitstate.go:484-494`）。

### 3.2. 機能一覧

#### F-001: 外す対象のブランチの設定の変化で止まらない

`inspect`・`finish`・`push` は、config を基準値と比べるときに、外す対象のブランチの設定を比べない。

**Acceptance Criteria**:

-   **AC-01**: `begin` の後に、外す対象のブランチの設定が加わる、値が変わる、または消える。そのほかに git の状態の変化がなければ、`inspect`・`finish`・`push` は `git-state-changed` で止まらない。設定を変える手段は問わない。issue #431 の表の操作（別の worktree での `git worktree add -b X origin/main`・`git push -u`・`gh pr checkout`・`git branch -d X`）と、`git config` による直接の書き込みを含む。ただし、操作が外す対象のブランチの設定のほかにも書き込む場合（例: fork の PR の `gh pr checkout` が `remote.*` を加える場合）は、AC-08 のとおり止まる。
-   **AC-02**: 外す対象のブランチの `[branch "X"]` の節の全体が加わる、または消える場合も、AC-01 と同じく止まらない。`[branch ""]` の節も同じである。
-   **AC-03**: AC-01・AC-02 は、その設定がどのファイルにある場合にも成り立つ。リポジトリの config、worktree の config、グローバルの config、システムの config、`include.path`・`includeIf` で読み込まれるファイルを含む。
-   **AC-04**: 今のブランチの名前が別のブランチの名前の一部と重なっても、外す対象のブランチの設定であれば止まらない。例えば、今のブランチが `feature` のときの `branch.feature.a.remote`（`<name>` は `feature.a`）や、`[branch "Feature"]` の節に書いた `remote`（大文字・小文字の違い）の変化である。

#### F-002: それ以外の config の変化では、今までどおり止まる

外す対象のブランチの設定を除いた設定の値の一覧と、git のディレクトリの場所が基準値と違えば、`inspect`・`finish`・`push` は今までどおり `git-state-changed` で止まる。`parts` は `config` を含む。

**Acceptance Criteria**:

-   **AC-05**: 今のブランチの設定（`branch.<今のブランチ>.<key>`）が加わる、値が変わる、または消えると止まる。今のブランチの名前が `.` を含む場合も同じである。今のブランチが `feature` のときに、古い書き方の `[branch.Feature]` の節に書いた設定（git は `<name>` を小文字にして `feature` と読む）も同じである。
-   **AC-06**: 今のブランチが `main` でなくても、`main` のブランチの設定（`branch.main.<key>`）が加わる、値が変わる、または消えると止まる。
-   **AC-07**: `<name>` を持たない 2 段の `branch.*`（例: `branch.autoSetupMerge`・`branch.autoSetupRebase`・`branch.sort`）が加わる、値が変わる、または消えると止まる。
-   **AC-08**: ブランチの設定でない設定が、どのファイルにあっても、加わる、値が変わる、消える、または読み込み元が変わると止まる。例: `core.hooksPath`、`remote.origin.url`、`remote.<name>.*` の追加、`remote.pushDefault`、`include.path`・`includeIf` の追加と、include 先のファイルの変化のうち、設定の値の一覧に現れるもの。
-   **AC-09**: 外す対象のブランチの設定を除いた、設定の値の一覧の項目の順序が変わると止まる。理由: git は、同じ名前の設定のうち後に読んだものを使い、複数の値を持つ設定はその順序で使う。
-   **AC-10**: config のファイルの、設定の値の一覧を変えない変更では止まらない。例: コメント・空行・字下げだけの変更。
-   **AC-11**: git のディレクトリの場所（`.git` ファイルと `commondir` が指す先）が変わると止まる。
-   **AC-12**: `begin` では読めた設定の値の一覧が、比較の時点で読めない場合は、`git-state-changed` で止まり、`parts` は `config` である。例: config のファイルの構文の誤り、`include.path` が FIFO やデバイスを指す場合。この場合も、AC-14 の起動前の比較の保証を保つ。
-   **AC-13**: `begin` の後に、HEAD が別のブランチに移っている、または detached になっていると止まる。`parts` は `head` と `config` の少なくとも一方を含む。
-   **AC-14**: config の違いで止まるときも、0030 の起動前の比較の保証を保つ。`inspect` は `gh` を、`finish` は make のターゲット・stage・commit を、`push` は push を実行しない。変わった設定が指すコマンド（例: `core.hooksPath` の先の hook）も起動しない。

#### F-003: 前の版の基準値の拒否と、出力の契約

前の版の基準値を拒否し、止まり方の種類と出力の内容は今までどおりに保つ。

**Acceptance Criteria**:

-   **AC-15**: 本タスクの前の版の `begin` が出力した基準値（`v1.` に続く 7 つの項目の形。`gitstate.go:28-32`）を `--baseline` に渡すと、`inspect`・`finish`・`push` はそれを受け付けない。止まり方は、既存の基準値の形の誤りと同じで、終了コード `2` である（`gitstate.go:121-138` の `ParseBaseline` が `ErrInvalidInput` で拒否する場合と同じ）。
-   **AC-16**: 止まり方の種類と `parts` の値の集合（`head`・`remote`・`hooks`・`config`・`info`）は変わらない。`inspect`・`finish`・`push` の止まったときの出力、標準エラーの診断、基準値のトークンには、ブランチの名前、設定の名前と値、読み込み元のパスを平文で含めない（0030 の標準エラーの規則（ガイド 4 章「Standard error」）と、今の基準値がダイジェストだけを持つこと（`gitstate.go:98-113`）に揃える）。

#### F-004: 文書の改訂

0030 の文書とガイドの、config の比較の対象範囲を述べる記述を、本書の規則に合わせる。

**Acceptance Criteria**:

-   **AC-17**: 0030 の `01_requirements.md` と `02_architecture.md` のうち、config の比較の対象範囲を述べる記述のすべてに、次の 3 点が書かれている。(a) 外す対象のブランチの設定は比べないこと（今のブランチと `main` の設定は比べること）。(b) 設定の値の一覧を変えない編集は検出しないこと。(c) 外す対象のブランチの設定の書き換えは、残余のリスクであること（4.2）。対象の記述には、少なくとも次を含む。`01_requirements.md` の 1 章の決定（修正のフェーズの異常）、AC-26、4.2（層 (3)（commit される変更と git の状態の確認）の説明と残余のリスク）。`02_architecture.md` の 2 章の push の説明、3.6 の `finish` の不変条件、3.7（基準値の表の config の行と、起動前の比較の順序）、5.2 の守りの表、5.3、テストの一覧。両文書の `Status` は `approved` のままで、`Comments` に、`(decision)` を付けて、タスク 0035（#431）の要件で決めた変更であることと、その日付が記録されている。
-   **AC-18**: `fixprguard_guide.md` と `fixprguard_guide.ja.md` では、config の比較の対象範囲を述べる記述のすべてが、AC-17 の (a)〜(c) を述べている。対象には、1 章の層 (3)、5 章の `inspect`・`push`、7.1 の「git is not started with rewritten hooks or config」の行、7.2、8 章の `git-state-changed` の行を含む。7.1 の行は、外す対象のブランチの設定を除いた config を基準値と比べる、という範囲に書き改められている。
-   **AC-19**: `internal/fixprguard` の `Protocol` は、本タスクの前の版の値より大きい。この `Protocol`、ガイド 4 章の現在の番号、`.claude/commands/` のコマンドの文書が渡す `--protocol` の値が一致している。

### 3.3. 信頼できない入力

設定の値の一覧は git の出力である。その元になる config のファイルは、修正のフェーズのエージェントや build checks のコードが書き換えうる。ブランチの設定かどうかを設定の名前から判定するので、この出力は信頼できない入力として扱う。

-   **受理する形:** git が出力する、設定の値の一覧の項目の並び。各項目は、読み込み元、設定の名前、値から成る。値を持たない項目（`[core]` の下に `=` なしで書いた `bare` のような暗黙の真の値）も受理する。値を持たない項目と、空の値を持つ項目は、比べるときに区別する。具体的な区切り（`-z` の出力の形）は設計で、使う git の版の出力で確かめて決める（design_handoff.md の H-04）。
-   **解釈する範囲:** 設定の名前のうち、セクション（`branch` かどうか）とサブセクション（`<name>`）だけを解釈する。サブセクションは、UTF-8 として解釈せずにバイト列のまま比べる。読み込み元と値は解釈せず、比べるだけである。解釈せずに受け入れる（拡張可能と宣言する）項目はない。同じ名前の項目が繰り返し現れるのは正当な形（複数の値を持つ設定、後に読んだものの優先）であり、拒否しない。比べる対象にも、順序のとおりに含める（AC-09）。
-   **拒否:** 受理する形に合わない項目が一つでもあれば、比較を続けずに止まる。止まり方は AC-12 と同じく `git-state-changed`（`parts` は `config`）である。例えば、最後の項目の終わりの区切りが欠けた出力、名前が空の項目である。形を補って解釈を続けることはしない（CLAUDE.md「Reject, don't normalize」）。
-   **上限:** 既存の git の出力の上限（`cmd/fixprguard/main.go:43-45` の `gitMaxOutput`、64 MiB）を超えれば、今までどおり `git-failed` で止まる。項目の数の上限は設けない。出力の上限で足りるからである。
-   **`begin` の時点:** `begin` が設定の値の一覧を読めない場合や、受理する形に合わない場合は、比べる基準値がないので、今の git の失敗と同じく `git-failed` で止まり、基準値を出力しない。
-   **部分的な結果:** 拒否したときに、一部だけを比べた結果で続けることはしない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。実行時間は、`make test` と GitHub の呼び出しが支配する（CLAUDE.md「Performance」）。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の適用条件には当たらない。本タスクは、`yt-dlp` の起動、API キー・Webhook の URL、キャッシュ、LLM API・Webhook への送信、信頼できないテキストのプロンプトや記事への埋め込みのいずれにも触れない。よって `N/A` とする。
-   ガードツールとしての要件は、次のとおりである。
    -   **実行の間の保証:** 外す対象のブランチの設定は、ツールが実行の間に起動する git と `gh` の振る舞いを変えない、という前提に立つ。この前提は、設計で確かめる（[design_handoff.md](design_handoff.md) H-02）。
    -   **起動の順序の保証:** 書き換えられた設定のまま、git・`gh`・make を起動しない（AC-14）。
    -   **残余のリスク（新たに受け入れるもの）:**
        -   修正のフェーズのエージェントや build checks のコードが、外す対象のブランチの設定を書き換えても検出しない。書き換えは実行の後にも残り、そのブランチの上で後に行う操作（`git push`・`git pull`・`git merge` など）の宛先や振る舞いを変えうる。
        -   その worktree でそのブランチに切り替えて `/fixpr` を実行すれば、そのブランチが今のブランチになるので、`begin` の後の書き換えは検出される。ガードツールを使わない操作では検出されない。
        -   `main` の設定は比べ続けるので、このリスクに含まない。設定の値の一覧を変えない編集も検出しないが、git の振る舞いを変えないので、リスクにならない。
        -   0030 の残余のリスク（0030 `01_requirements.md` 4.2・`02_architecture.md` 5.3）は、ほかに変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   並行して動くほかのセッションのブランチの操作で、`/fixpr` の Phase 3.5 以降が止まらない（F-001）。
-   設定の値の一覧を読めない場合や、形が想定と違う場合は、続けずに止まる（AC-12・3.3）。

### 4.4. 互換性 (Compatibility)

-   前の版の基準値は受け付けない（AC-15）。1 回の `/fixpr` の途中でツールを入れ替えると、その回は終了コード `2` で止まる。`begin` からやり直せばよい。
-   止まり方の種類・`parts`・終了コードの意味は変えない（AC-16）。
-   使う git の最低の版は、設計で確かめる（design_handoff.md の H-08）。

### 4.5. 保守性 (Maintainability)

-   外す対象のブランチの設定かどうかの判定は 1 か所に置き、`begin` と `inspect`・`finish`・`push` で同じ規則を使う。
-   ガイド 9 章の規則（7.1 の保証を変えるときは、実装を壊してテストが失敗することを確かめ、commit のメッセージに書く）に従う。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。外部のライブラリを加えない（CLAUDE.md「Dependencies」）。
-   **先行するタスクの方針の例外:** 0030 `01_requirements.md` の AC-26 (c) は、「git の設定ファイル（リポジトリの config と、worktree の config）」が違えば止まると定めている。0030 `02_architecture.md` 3.7 は、config のファイルの内容のダイジェストを基準値に持つと定めている。本タスクは、外す対象のブランチの設定と、設定の値の一覧を変えない編集を、この比較から外す。理由は 1 章の「issue #431 で決めたこと」と「要件定義の際に決めたこと」のとおりである。0030 の文書は AC-17 のとおり改める。
-   **文書の改訂の規則の例外:** `docs/dev/developer_guide/requirements_process.md` の「Editing an approved document」は、承認済みの文書の決定を変えるときは、`Status` を `draft` に戻して承認し直すと定めている（例外は `/runplan` の実装の途中の決定だけである）。本タスクは、0030 の 2 つの文書の決定を変えるが、`Status` を `approved` のままにする（AC-17）。理由: 変更の決定は本書の承認で得られ、0030 の文書の改訂はその反映である。`Comments` の記録には `(decision)` を付け、本書を参照する。0030 の文書の改訂は、本書が承認された後に行う。
-   タスク 0034（#424）より先に、実装まで終えてマージする。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。「基準値」（baseline）・「ガードツール」（guard tool）・「片付け」（cleanup）は同書の用法に従う。「ダイジェスト」（digest）は 0030 の文書の用法に従う。
-   **ブランチの設定・今のブランチ・外す対象のブランチの設定・設定の値の一覧:** 3.1 で定義する。
