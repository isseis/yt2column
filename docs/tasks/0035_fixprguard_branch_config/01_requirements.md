# 要件定義書：fixprguard の config の比較から、ほかのブランチの `branch.<name>.*` を外す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`fixprguard` が git の config を基準値と比べるときに、今のブランチ以外のブランチの `branch.<name>.*` の設定を比べる対象から外すための要件を定義する（[#431](https://github.com/isseis/yt2column/issues/431)）。それ以外の設定は今までどおり厳密に比べ、違えば止まる。

**背景:**

`fixprguard` は、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で作られた `/fixpr` と `/runpr` の守りである。`begin` が基準値を取り、`inspect`・`finish`・`push` が git の状態を基準値と比べる（`docs/dev/developer_guide/fixprguard_guide.md` 5 章）。

config については、今は次の 2 つを比べている。

-   git のディレクトリの場所（`.git` ファイルと `commondir` が指す先）と、リポジトリの config・worktree の config のファイルのバイト列の digest（`internal/fixprguard/gitstate.go:279-298` の `configDigest`）。
-   `git config --list --show-origin -z` の出力全体の digest（同 `:484-494` の `configListDigest`）。git が読むすべての設定の値を、読み込み元のファイルとともに含む。

比べるのは `CheckState`（同 `:514-561`）で、`inspect`・`finish`・`push` が git や `gh` を起動する前に呼ぶ。順序は、ファイルのバイト列の比較、`git config --list` の比較、hooks の場所を求める `git rev-parse` の順である。設定が書き換えられたまま git を起動しないための順序である（0030 `02_architecture.md` 3.7）。

リポジトリの config は、すべての worktree が共有している。そのため、並行して動くほかのセッションがブランチを操作するだけで、このセッションの `inspect`・`finish`・`push` が `git-state-changed`（`parts` は `config`）で止まる。issue #431 は、config に書き込む操作として次を挙げている。

| 操作 | config への書き込み |
|---|---|
| `git worktree add -b X origin/main` | `branch.X.remote`・`branch.X.merge` を加える |
| `git push -u`、`gh pr checkout` | `branch.X.*` を加える |
| `git branch -d X`（`/mergepr` の後片付けを含む） | `[branch "X"]` の節を消す |

PR #429 の `/fixpr` は、これが原因で Phase 3.5 で止まった。止まると、修正のフェーズのエージェントの変更を捨ててやり直すことになる。worktree を並べて作業すると、この手間が繰り返し生じる。

対応 issue: [#431](https://github.com/isseis/yt2column/issues/431)（関連: [#395](https://github.com/isseis/yt2column/issues/395)（タスク 0030）、[#424](https://github.com/isseis/yt2column/issues/424)（タスク 0034、PR #429））

**本書の記述範囲:** 本書は、何を比べ、何を比べないか、止まるときと止まらないときに利用者から何が見えるかを定める。次の事項は設計（`02_architecture.md`）で決める。

-   設定の値の一覧の求め方と、今のブランチの名前の求め方。
-   ファイルのバイト列の比較を残すか、やめるか。
-   基準値のトークンの形。
-   起動の順序の保証を保つ方法。
-   テストの手段。

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #431 で決めたこと:**

-   **ほかのブランチの `branch.<name>.*` を比べない:** ほかのブランチの upstream の設定は、このブランチの操作に効かない。そのため、比べる対象から外しても保証は弱まらない。
-   **今のブランチの `branch.*` と、それ以外の設定は、今までどおり厳密に比べる:** 今のブランチの設定は、`push` の宛先に効くためである（`internal/fixprguard/gitstate.go:617-650` の `pushTarget`・`resolvePushTarget` は、`branch.<b>.pushRemote`・`branch.<b>.remote` を読む）。
-   **git の場所の比較を残す:** `.git` ファイルや `commondir` を書き換えて、リポジトリの複製を指させる変更を検出するためである（`gitstate.go:279-282`）。
-   **内容の比較は `git config --list -z` の出力で行う方向で検討する:** Go で config のファイルを解析すると壊れやすいからである。ファイルのバイト列の比較をやめるかどうかは、設計で決める。やめる場合は、起動の順序の保証を保てるかを設計で確かめる。今のバイト列の比較は、git を起動する前に、危ない設定（`core.fsmonitor` など）を見つけるために先に行っている。`git config` はコマンドを起動しないので、`config --list` を先に行っても同じ保証を保てるかが、確かめる点である。
-   **文書を改める:** ガイド（`fixprguard_guide.md`・`fixprguard_guide.ja.md`）と、0030 の文書の保証・残余のリスクの記述を改める。
-   **`Protocol` の扱い:** 出力または引数の契約が変わる場合は、ガイド 9 章に従って `Protocol` を上げる。
-   **進め方:** タスク 0034（#424、git が無視するファイルの検出）より先に、実装まで終えてマージする。0034 の PR #429 は、それまで止めておく。

**要件定義の際に決めたこと（2026-10-22）:**

-   **git が読む値を変えない編集では止めない:** config のファイルの、コメント・空行・字下げだけの編集は、止める対象にしない（AC-10）。理由: こうした編集では、git の振る舞いは変わらず、コマンドも起動されない。また、ファイルのバイト列で比べ続けると、ほかのブランチの節の追加と削除もバイト列を変えるので、本タスクの目的を果たせない。
-   **読み込み元を問わず除外する:** ほかのブランチの `branch.<name>.*` は、どのファイル（システム・グローバル・include 先・リポジトリ・worktree の config）にあっても比べない（AC-03）。理由: 通常は、ブランチの操作はリポジトリの config に書き込む。ただ、ほかのファイルにある `branch.<other>.*` も、今のブランチの操作に効かない点は同じである。読み込み元で区別しないほうが規則も単純である。
-   **前の版の基準値を受け付けない:** 本タスクの前の版の `begin` が出力した基準値を渡されたら、`inspect`・`finish`・`push` は終了コード `2` で拒否する（AC-13）。理由: 比べる内容の意味が変わる。そのため、前の版の基準値を黙って受け付けると、どの規則で比べたのかが分からなくなる。タスク 0034 の要件案（PR #429 の AC-03）とも揃う。
-   **0030 の承認済みの文書は、本文を改めて `approved` を保つ:** 0030 の `01_requirements.md`・`02_architecture.md` の該当する記述を改める。そのうえで、`Comments` に、タスク 0035（#431）の要件で決めた変更であることを記録する（AC-15）。理由: 変更の決定は、本書の承認で得る。そのため、0030 の文書を改めて承認し直す必要はない。どの記述が、いつ、なぜ変わったかは `Comments` から辿れる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   並行して動くほかのセッションがほかのブランチを操作しても、このセッションの `/fixpr`・`/runpr` が `git-state-changed` で止まらないようにする。
-   今のブランチの設定と、それ以外のすべての設定の変化は、今までどおり検出して止める。
-   書き換えられた設定のまま git・`gh`・make を起動しない、という 0030 の保証を保つ。

### 2.2. スコープ (In Scope)

-   `inspect`・`finish`・`push` の config の比較から、ほかのブランチの `branch.<name>.*` を外すこと（F-001）。
-   それ以外の設定と、git のディレクトリの場所の比較を保つこと（F-002）。
-   前の版の基準値の拒否（F-003）。
-   ガイド（英語・日本語）と、0030 の `01_requirements.md`・`02_architecture.md` の記述の改訂（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **hooks・info・HEAD・push 先の比較の変更。** issue #431 が挙げたのは config の比較だけである。並行するセッションは、通常、これらを変えない（hooks と info は共有の git のディレクトリにあるが、ブランチの操作では書き込まれない）。
-   **ほかのブランチの設定以外で、並行するセッションが config に書き込む変更。** 例えば、fork の PR を `gh pr checkout` したときに加わる `remote.<name>.*`、`git maintenance register` がグローバルの config に加える `maintenance.repo` である。これらは今までどおり止まる（AC-07）。理由: issue #431 は `branch.<name>.*` に限っている。また、`remote.*` は、`remote.pushDefault` などを介して今のブランチの push 先に効きうる。
-   **基準値を入力に持たない `head`・`list`・`fence`・`reply`・`edit-pr`。** 今も比較しない（0030 `02_architecture.md` 5.3「基準値を持たないサブコマンドの起動前の比較」）。本タスクも変えない。
-   **タスク 0034（#424）の要件。** 0034 の要件案は、基準値の内容を `gitstate.go:98-113` を引いて説明している。本タスクで基準値が変わったら、0034 の側で、PR #429 を再開するときに合わせる。
-   **`/fixpr`・`/runpr` のコマンドの文書（`.claude/commands/`）の手順。** 止まり方の種類と `parts` の値は変わらないので、手順は変えない。`Protocol` を上げる場合に、コマンドの文書が渡す `--protocol` の値を改めることだけは、F-004 に含む。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **ブランチの設定:** 名前が `branch.<name>.<key>` の形の設定。`<name>` は git の config のサブセクションで、最初の `.` と最後の `.` の間の部分である。`<name>` は `.` を含みうる（例: `branch.feature.a.remote` の `<name>` は `feature.a`）。`branch.autoSetupMerge` のように `<name>` を持たない 2 段の名前は、ブランチの設定ではない。
-   **今のブランチ:** 比較の時点で、作業ツリーの HEAD がいるブランチ。
-   **ほかのブランチの設定:** `<name>` が今のブランチの名前とバイト列で一致しない、ブランチの設定。git が `branch.<name>.*` を使うのは、`<name>` がブランチの名前と完全に一致するときだけだからである。大文字・小文字の違いも、違う名前として扱う。そのブランチが存在しない場合も含む。
-   **設定の値の一覧:** git が読むすべての設定の値を、git が読む順序で並べ、それぞれの読み込み元とともに持つもの。今の `configListDigest` が digest を取る対象である（`gitstate.go:484-494`）。

### 3.2. 機能一覧

#### F-001: ほかのブランチの設定の変化で止まらない

`inspect`・`finish`・`push` は、config を基準値と比べるときに、ほかのブランチの設定を比べない。

**Acceptance Criteria**:

-   **AC-01**: `begin` の後に、ほかのブランチの設定が加わる、値が変わる、または消える。そのほかに git の状態の変化がなければ、`inspect`・`finish`・`push` は `git-state-changed` で止まらない。設定を変える手段は問わない。issue #431 の表の操作（別の worktree での `git worktree add -b X origin/main`・`git push -u`・`gh pr checkout`・`git branch -d X`）と、`git config` による直接の書き込みを含む。
-   **AC-02**: ほかのブランチの `[branch "X"]` の節の全体が加わる、または消える場合も、AC-01 と同じく止まらない。
-   **AC-03**: AC-01・AC-02 は、ほかのブランチの設定がどのファイルにある場合にも成り立つ。リポジトリの config、worktree の config、グローバルの config、システムの config、`include.path`・`includeIf` で読み込まれるファイルを含む。
-   **AC-04**: 今のブランチの名前が別のブランチの名前の一部と重なっても、ほかのブランチの設定であれば止まらない。例えば、今のブランチが `feature` のときの `branch.feature.a.remote`（`<name>` は `feature.a`）や、`branch.Feature.remote`（大文字・小文字の違い）の変化である。

#### F-002: それ以外の config の変化では、今までどおり止まる

ほかのブランチの設定を除いた、設定の値の一覧と、git のディレクトリの場所が基準値と違えば、`inspect`・`finish`・`push` は今までどおり `git-state-changed` で止まる。`parts` は `config` を含む。

**Acceptance Criteria**:

-   **AC-05**: 今のブランチの設定（`branch.<今のブランチ>.<key>`）が加わる、値が変わる、または消えると止まる。今のブランチの名前が `.` を含む場合も同じである。
-   **AC-06**: `<name>` を持たない 2 段の `branch.*`（例: `branch.autoSetupMerge`・`branch.autoSetupRebase`・`branch.sort`）が加わる、値が変わる、または消えると止まる。
-   **AC-07**: ブランチの設定でない設定が、どのファイルにあっても、加わる、値が変わる、消える、または読み込み元が変わると止まる。例: `core.hooksPath`、`remote.origin.url`、`remote.<name>.*` の追加、`remote.pushDefault`、`include.path`・`includeIf` の追加と、include 先のファイルの中身の変化。
-   **AC-08**: 設定の値の一覧の中で、ほかのブランチの設定を除いた部分の順序が変わると止まる。理由: git は、同じ名前の設定のうち後に読んだものを使い、複数の値を持つ設定はその順序で使う。
-   **AC-09**: git のディレクトリの場所（`.git` ファイルと `commondir` が指す先）が変わると止まる。
-   **AC-10**: config のファイルの、コメント・空行・字下げだけの変更（設定の値の一覧を変えない変更）では止まらない。
-   **AC-11**: config の違いで止まるときも、0030 の起動前の比較の保証を保つ。`inspect` は `gh` を、`finish` は make のターゲット・stage・commit を、`push` は push を実行しない。変わった設定が指すコマンド（例: `core.hooksPath` の先の hook）も起動しない。
-   **AC-12**: `begin` の後に、HEAD が別のブランチに移っていると止まる。`parts` は `head` と `config` の少なくとも一方を含む。

#### F-003: 前の版の基準値を受け付けない

**Acceptance Criteria**:

-   **AC-13**: 本タスクの前の版の `begin` が出力した基準値を `--baseline` に渡すと、`inspect`・`finish`・`push` はそれを受け付けない。止まり方は、既存の基準値の形の誤りと同じで、終了コード `2` である（`gitstate.go:121-138` の `ParseBaseline` が `ErrInvalidInput` で拒否する場合と同じ）。
-   **AC-14**: 止まり方の種類と `parts` の値の集合（`head`・`remote`・`hooks`・`config`・`info`）は変わらない。標準出力・標準エラー・基準値には、ブランチの名前、設定の名前と値、読み込み元のパスを含めない（0030 の標準エラーの規則。ガイド 4 章「Standard error」）。

#### F-004: 文書の改訂

**Acceptance Criteria**:

-   **AC-15**: 0030 の `01_requirements.md`（AC-26 と 4.2 の残余のリスク）と `02_architecture.md`（3.7 の基準値の表の config の行、5.3）に、次の 2 点が書かれている。(a) ほかのブランチの設定は比べないこと。(b) git が読む値を変えない編集は検出しないこと。両文書の `Status` は `approved` のままで、`Comments` に、タスク 0035（#431）の要件で決めた変更であることと、その日付が記録されている。
-   **AC-16**: `fixprguard_guide.md` と `fixprguard_guide.ja.md` が、AC-15 の (a)・(b) を、7.1 の保証か 7.2 の「保証しないこと」に書いている。8 章の `git-state-changed` の行の説明が、F-001・F-002 の比べる範囲と食い違わない。
-   **AC-17**: `Protocol` を上げる場合は、`internal/fixprguard` の `Protocol`、ガイド 4 章の現在の番号、`.claude/commands/` のコマンドの文書が渡す `--protocol` の値が一致している。

### 3.3. 信頼できない入力

設定の値の一覧は git の出力である。その元になる config のファイルは、修正のフェーズのエージェントや build checks のコードが書き換えうる。ブランチの設定かどうかを設定の名前から判定するので、この出力は信頼できない入力として扱う。

-   **受理する形:** git が出力する、設定の値の一覧の項目の並び。各項目は、読み込み元、設定の名前、値から成る。具体的な形（`-z` の区切り、読み込み元の書式）は設計で決める。
-   **使うもの:** 設定の名前の、セクション（`branch` かどうか）とサブセクション（`<name>`）だけを解釈する。読み込み元と値は解釈せず、比べるだけである。拡張可能と宣言するものはない。同じ名前の項目が繰り返し現れるのは正当な形（複数の値を持つ設定、後に読んだものの優先）であり、拒否しない。比べる対象にも、順序のとおりに含める（AC-08）。
-   **拒否:** 受理する形に合わない項目が一つでもあれば、比較を続けずに止まる。区切りが欠けた項目、名前が空の項目、`branch` のセクションでサブセクションが空の項目がこれに当たる。止まり方は、既存の git の出力が想定の形でない場合と同じで、`git-failed`（`ErrGitFailed`）である。形を補って解釈を続けることはしない（CLAUDE.md「Reject, don't normalize」）。サブセクションは、UTF-8 として解釈せずにバイト列のまま比べる。
-   **上限:** 既存の git の出力の上限（`cmd/fixprguard/main.go:43-45` の `gitMaxOutput`、64 MiB）を超えれば、今までどおり `git-failed` で止まる。
-   **部分的な結果:** 拒否したときに、一部だけを比べた結果で続けることはしない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比較の回数と起動する git の数は、今と同じ程度にとどめる。実行時間は、`make test` と GitHub の呼び出しが支配する（CLAUDE.md「Performance」）。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の適用条件には当たらない。本タスクは、`yt-dlp` の起動、API キー・Webhook の URL、キャッシュ、LLM API・Webhook への送信、信頼できないテキストのプロンプトや記事への埋め込みのいずれにも触れない。よって `N/A` とする。
-   `fixprguard` の守りとしての要件は、次のとおりである。
    -   **保証を弱めない範囲:** 比べない対象に加えるのは、ほかのブランチの設定だけである（F-001）。ツールが起動する git（`status`・`diff`・`add`・`commit`・`ls-remote`・`push`）がブランチの設定を読むのは、今のブランチの分だけである。そのため、ほかのブランチの設定が変わっても、ツールの git の振る舞いは変わらない。この前提が成り立つことは、設計で確かめる（[design_handoff.md](design_handoff.md) H-02）。
    -   **起動の順序の保証:** 書き換えられた設定のまま、git・`gh`・make を起動しない（AC-11）。
    -   **残余のリスク（新たに受け入れるもの）:** 修正のフェーズのエージェントが、ほかのブランチの設定や、config のコメント・空行を書き換えても検出しない。これらは、ツールが起動する git の振る舞いを変えないので、受け入れる。ただし、その後にそのブランチへ切り替えれば、HEAD の違いとして止まる（AC-12）。0030 の残余のリスク（0030 `01_requirements.md` 4.2・`02_architecture.md` 5.3）は、ほかに変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   並行して動くほかのセッションのブランチの操作で、`/fixpr` の Phase 3.5 以降が止まらない（F-001）。
-   設定の値の一覧を読めない場合や、形が想定と違う場合は、続けずに止まる（3.3）。

### 4.4. 互換性 (Compatibility)

-   前の版の基準値は受け付けない（AC-13）。1 回の `/fixpr` の途中でツールを入れ替えると、その回は終了コード `2` で止まる。`begin` からやり直せばよい。
-   止まり方の種類・`parts`・終了コードの意味は変えない（AC-14）。
-   git の版の前提は 0030 と同じである（`gitstate.go:68-71`。git 2.5 以降の機能を使う）。

### 4.5. 保守性 (Maintainability)

-   ブランチの設定かどうかの判定は 1 か所に置き、`begin` と `inspect`・`finish`・`push` で同じ規則を使う。
-   ガイド 9 章の規則（7.1 の保証を変えるときは、実装を壊してテストが失敗することを確かめ、commit のメッセージに書く）に従う。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。外部のライブラリを加えない（CLAUDE.md「Dependencies」）。
-   **先行するタスクの方針の例外:** 0030 `01_requirements.md` の AC-26 (c) は、「git の設定ファイル（リポジトリの config と、worktree の config）」が違えば止まると定めている。0030 `02_architecture.md` 3.7 は、config のファイルの内容の digest を基準値に持つと定めている。本タスクは、ほかのブランチの設定と、値を変えない編集をこの比較から外す。理由は 1 章の「issue #431 で決めたこと」と「要件定義の際に決めたこと」のとおりである。0030 の文書は AC-15 のとおり改める。
-   タスク 0034（#424）より先に、実装まで終えてマージする。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。「基準値」（baseline）はガイドと 0030 の用法に従う。
-   **ブランチの設定・今のブランチ・ほかのブランチの設定・設定の値の一覧:** 3.1 で定義する。
