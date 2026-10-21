# 要件定義書：git が無視する実行関連のファイルへの書き込みを fixprguard が検出する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/fixpr` の修正のフェーズで git が無視するファイルのうち実行関連のファイルが作られ、書き換えられ、または削除されたときに、ガードツール `fixprguard` がそれを検出して止まるための要件を定義する（[#424](https://github.com/isseis/yt2column/issues/424)）。

**背景:**

`fixprguard` は、`/fixpr` と `/runpr` が信頼できないテキスト（PR のレビューのスレッド）を扱う間の守りとして、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で作られた。`begin` は作業ツリーが clean であることを確かめてから基準値を取る。`inspect`・`finish`・`push` は、git の状態を基準値と比べ、変更の集合を判定した集合と比べる。

これらの検査は、どれも git の目を通してファイルを見る。

-   clean の検査は `git status --porcelain=v1 -z --untracked-files=all` の出力が空であることを確かめる（`internal/fixprguard/worktree.go:63-75`）。
-   commit の前の検査で、追跡されていないファイルを数えるのは `git ls-files --others --exclude-standard` である（`internal/fixprguard/worktree.go:351`）。
-   基準値が持つのは、HEAD・ブランチ・push 先のブランチの OID・hooks・config のファイル・git が読む config の値の一覧・info のファイル（`info/exclude`・`info/attributes`）の 7 つである（`internal/fixprguard/gitstate.go:98-113`）。作業ツリーのファイルは入っていない。

このため、`.gitignore` が無視するファイルは、どの検査にも映らない。修正のフェーズのエージェント（信頼できないテキストを読んだ主体）が無視されるファイルに書き込んでも、ツールは止まらない。書き込まれたファイルは commit も push もされないので、利用者に報告されることもない。

このリポジトリでは `.claude/settings.local.json` が無視される（`.gitignore:3`）。ここに `hooks` や `permissions.allow` を書かれると、次回以降の Claude Code のセッションで任意のコマンドが実行されるか、危険な操作が確認なしで許可される。`.envrc`・`.env` も無視されるので同じ経路になる。ただし `.envrc` については、内容が変わると direnv が再度の `allow` を求めるので、被害は少し緩和される。

ほかにも同じ種類の経路がある。Go のツールは、`.gitignore` を参照せずにパッケージを探す。そのため、`go test ./...` は無視されるディレクトリ（例: `build/`、`extension/node_modules/`）の中の `.go` ファイルもコンパイルして実行する。無視される場所に置かれた `_test.go` は、それ以後に利用者が実行する `make test` で毎回実行される。

0030 は、無視されるファイルを残余のリスクとして記録した（0030 `01_requirements.md` 4.2、0030 `02_architecture.md` 5.3）。ただし、そこで想定したのは build checks がその回の `make test` で `_test.go` を実行することだけであり、次のセッションや後の実行まで残る書き込みは想定していなかった。

対応 issue: [#424](https://github.com/isseis/yt2column/issues/424)（関連: [#395](https://github.com/isseis/yt2column/issues/395)、タスク `0030_fixpr_untrusted_threads`）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。対象は、どのファイルの、どんな変化を、どのサブコマンドが検出し、止まったときに何を出力するかである。次のことは設計（`02_architecture.md`）で決める。

-   無視されるファイルを列挙する手段と、基準値にその状態を持たせる手段（トークンの形を含む）
-   比較の順序
-   シンボリックリンクと、通常のファイルでないものの扱い
-   `protocol` の番号の扱い
-   テストの手段

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #424 で決めたこと:**

-   **止まり方:** 基準値に含めた無視されるファイルが基準値と違えば、既存の `git-state-changed` で止まる。新しい止まった種類は作らない。止まるのは `inspect`・`finish`・`push` である。理由: 修正のフェーズが git の状態を変えたときと同じく、ツールの外での変化を見つけたことを表すからである。
-   **最低限の対象:** `.claude/settings.local.json` は必ず対象に含める（存在しないことも状態として記録する）。理由: このリポジトリで無視され、次のセッションでのコマンドの実行に直結するからである。
-   **文書を新しい保証に合わせる:** 0030 `02_architecture.md` 5.3「無視されるファイル」と `fixprguard_guide.md` 7 章を改める。理由: 古い保証の範囲のまま残すと、利用者がツールの守りを誤解するからである。
-   **完了条件:** `make test`・`make lint`・`doc-guard` が通る。テストが目的の理由で失敗できることは、`CLAUDE.md`「Testing Strategy」の手順で確かめる。

**要件定義の際に決めたこと（2026-10-22）:**

-   **対象を実行関連のファイルの定義全体にする:** 基準値と比べるのは、git が無視するファイルのうち、`fixprguard_guide.md` 6 章「実行関連のファイル」の定義（`internal/fixprguard/scope.go:36-54`）に当たるもの全部である。issue が案として挙げた「`.claude/` の下とルートのドットファイル」は、この定義に含まれる。理由: `.claude/` の下とルートのドットファイルだけでは、無視される場所に置かれた `.go` ファイル（以後の `make test` で毎回実行される）が覆われないからである。実行関連のファイルの定義は範囲の規則のために既にあり、それを再利用すれば、定義を 1 か所に保てる（`CLAUDE.md`「DRY」）。
-   **利用者自身の正当な書き込みでも止まる:** 修正のフェーズの間に、利用者が Claude Code の許可の確認に「今後は確認しない」と答えると、Claude Code が `.claude/settings.local.json` を書き換える。この場合も `git-state-changed` で止まり、ガイドの 8 章でやり直しの手順を案内する。理由: ツールには、この書き込みと攻撃による書き込みを区別できない。区別できない変化は安全側に倒す（0030 `01_requirements.md` 4.2 の方針）。
-   **止まったときの出力は部分の名前だけ:** `git-state-changed` の `parts` に新しい値 `ignored` を加える。どのファイルが違うかは出力しない。理由: 既存の `parts` も部分の名前だけを示す（`internal/fixprguard/gitstate.go:170-182`）。ファイル名は攻撃者が選べるので、出力に入れるとセッションに指示文を届ける経路になる（0030 `02_architecture.md` 5.3「ファイル名による出力の汚染」）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   修正のフェーズで、git が無視する実行関連のファイルが作られ、書き換えられ、または削除されたら、commit・push の前に `fixprguard` が止まる。
-   次のセッションや以後の `make test` まで残る書き込み（Claude Code のセッションの設定、環境変数のファイル、無視される場所の `.go` ファイル）が、利用者に知られないまま残ることを防ぐ。
-   止まったときに、利用者が原因を調べて取り除く手順を、ガイドで示す。

### 2.2. スコープ (In Scope)

-   `begin` が取る基準値に、git が無視する実行関連のファイルの状態を含めること（F-001）。
-   `inspect`・`finish`・`push` が、その状態を基準値と比べ、違えば止まること（F-002）。
-   止まったときの出力（F-003）。
-   `fixprguard_guide.md`（日本語版・英語版）と、0030 の要件書・設計書の残余のリスクの記述を、新しい保証に合わせて改めること（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **リポジトリの外のファイル**（`~/.claude/settings.json`、シェルの起動ファイルなど）への書き込み。理由: ツールは作業ツリーとその git の状態を守る。リポジトリの外のファイルを修正のフェーズのエージェントが書き換えられるかどうかは、Claude Code の権限の確認（利用者の設定）で決まる。git の設定のファイルは、既存の config の値の一覧の比較（`internal/fixprguard/gitstate.go:484-494`）が覆う。
-   **実行関連のファイルの定義に当たらない、無視されるファイル**（例: `coverage.out`、`build/` の下のバイナリ、`.vscode/` の下のファイル）。理由: build checks・CI・Claude Code が設定として読む、またはコンパイルして実行するファイルではないからである。`make test` などが作るファイルでの誤検出も避けられる。
-   **変えて元に戻すこと、確認と実行の間の書き換え。** 0030 と同じく、ある時点だけの確認である（0030 `02_architecture.md` 5.3「確認と実行の間のずれ」）。
-   **`head`・`list`・`reply`・`edit-pr`。** 基準値を入力に持たないので、比較しない（0030 `02_architecture.md` 5.3「基準値を持たないサブコマンドの起動前の比較」）。`push` の後の書き込みは検出しない。
-   **`begin` の時点で既にある、無視されるファイルの内容の検査。** 利用者が自分で置いた `.claude/settings.local.json` や `.envrc` を、`begin` は拒否しない。記録するだけである。
-   **実行関連のファイルの定義そのものの変更。** 定義は範囲の規則と共有し、本タスクでは変えない。
-   **index の stat の偽造、hooks のディレクトリの外のコード。** 0030 の残余のリスクのまま変えない。

## 3. 機能要件 (Functional Requirements)

本書で「対象のファイル」とは、次の 2 つをともに満たすファイルをいう。

-   作業ツリーの中にあり、git が無視するファイル（`.gitignore`、`info/exclude`、グローバルの除外のファイルのどれかが無視するもの）である。
-   そのパスが、`fixprguard_guide.md` 6 章「実行関連のファイル」の定義（`internal/fixprguard/scope.go:36-54`。`*.go` の規則を含む）に当たる。

例: `.claude/settings.local.json`、ルートの `.envrc`・`.env`、`build/x_test.go`、`extension/node_modules/<pkg>/Makefile`。

対象のファイルの「状態」とは、どの対象のファイルがあるか、およびその一つひとつの種類（通常のファイル・シンボリックリンクなど）・内容・実行の許可・シンボリックリンクの先である。

### 3.1. 機能一覧

#### F-001: 基準値に対象のファイルの状態を含める

`begin` は、基準値を取るときに、対象のファイルの状態を記録する。`.claude/settings.local.json` は、存在しなくても「存在しない」という状態を記録する。

**Acceptance Criteria**:
- **AC-01**: `begin` の時点で対象のファイル（例: 利用者が置いた `.claude/settings.local.json`・`.envrc`）があっても、`begin` はそれを理由に止まらず、終了コード `0` で基準値を出力する。
- **AC-02**: `begin` の後に対象のファイルの状態が変わらなければ、`inspect`・`finish`・`push` の結果は本タスクの前と同じである（対象のファイルを理由に止まらない）。
- **AC-03**: 本タスクの前の版の `begin` が出力した基準値を渡すと、`inspect`・`finish`・`push` はそれを受け付けない。対象のファイルを比べないまま続けることはない。止まり方は既存の基準値の形の誤りと同じで、終了コード `2` である。

#### F-002: `inspect`・`finish`・`push` が対象のファイルの変化を検出する

`inspect`・`finish`・`push` は、git の状態を基準値と比べるときに、対象のファイルの状態も比べる。違えば `git-state-changed` で止まる。

**Acceptance Criteria**:
- **AC-04**: `begin` のときに存在しなかった `.claude/settings.local.json` が `begin` の後に作られると、`inspect` は終了コード `1`、`stop` が `git-state-changed` で止まる。
- **AC-05**: `begin` のときに存在した `.claude/settings.local.json` の内容が `begin` の後に書き換えられると、`inspect` は AC-04 と同じく止まる。利用者が Claude Code の許可の確認に答えたことによる書き換えも、同じく止まる。
- **AC-06**: `begin` のときに存在した対象のファイルが `begin` の後に削除されると、`inspect` は AC-04 と同じく止まる。
- **AC-07**: `.claude/settings.local.json` 以外の対象のファイルについても、AC-04〜AC-06 と同じく止まる。少なくとも次の場合を含む。
    -   ルートの `.envrc`・`.env` が作られる、または書き換えられる。
    -   無視されるディレクトリ（例: `build/`）の中に `.go` ファイルが作られる。
    -   無視されるディレクトリの中の `Makefile` または `*.mk` が作られる、または書き換えられる。
- **AC-08**: 対象のファイルの内容は同じでも、種類・実行の許可・シンボリックリンクの先が変わると、`inspect` は AC-04 と同じく止まる。たとえば、通常のファイルがシンボリックリンクに置き換えられた場合、シンボリックリンクの先が変わった場合、実行の許可だけが変わった場合である。
- **AC-09**: `begin` の後、`finish` を呼ぶ前に対象のファイルが変わっていると、`finish` は `git-state-changed` で止まる。build checks（`make fmt`・`make test`・`make lint`）を 1 つも実行せず、commit を作らない。
- **AC-10**: build checks の実行中（例: `make test` が実行したテストのコード）に対象のファイルが変わると、`finish` は `git-state-changed` で止まり、commit を作らない。
- **AC-11**: `finish` の後、`push` を呼ぶ前に対象のファイルが変わっていると、`push` は `git-state-changed` で止まり、push しない。
- **AC-12**: 無視されるが、対象のファイルではないファイル（例: ルートの `coverage.out`、`build/yt2column`、`.vscode/settings.json`）が作られ、または書き換えられても、`inspect`・`finish`・`push` はそれを理由に止まらない。
- **AC-13**: 新しく置かれた `.gitignore`（入れ子のディレクトリのものを含む）や、グローバルの除外のファイルの書き換えによって無視されるようになった実行関連のファイルも、`begin` の後に作られたものなら、AC-04 と同じく止まる。

#### F-003: 止まったときの出力

**Acceptance Criteria**:
- **AC-14**: 対象のファイルの状態だけが基準値と違うとき、AC-04〜AC-11・AC-13 の止まったときの出力の `parts` は `ignored` だけである。
- **AC-15**: 止まったときの標準出力と標準エラーには、対象のファイルのパス・ファイル名・内容が含まれない。
- **AC-16**: 対象のファイルの状態を記録または比較できないときは、`begin`・`inspect`・`finish`・`push` は既存の `git-failed` で止まる。一部のファイルだけを記録した基準値を出力することはなく、一部だけを比べて続けることもない。読めない場合には、列挙の結果が上限を超える場合と、ファイルを読み取れない場合を含む。対象のファイルが FIFO であっても、ツールは読み取りで待ち続けない。

#### F-004: 文書を新しい保証に合わせる

**Acceptance Criteria**:
- **AC-17**: `fixprguard_guide.md` と `fixprguard_guide.ja.md` の 7.1「保証すること」に、次のことが書かれている。git が無視する実行関連のファイルの作成・書き換え・削除を、`inspect`・`finish`・`push` が基準値と比べて検出する。
- **AC-18**: 同じガイドの 7.2「保証しないこと」の無視されるファイルの項目は、F-001〜F-003 で検出するものを除いた範囲を示している。少なくとも、実行関連のファイルの定義に当たらない無視されるファイル、変えて元に戻したもの、`push` の後の書き込み、リポジトリの外のファイルを示す。
- **AC-19**: 同じガイドの 8 章の `git-state-changed` の行は、`parts` の値に `ignored` を含み、その意味と対処を示している。対処には、次の 2 つが含まれる。
    -   意図しない変更を調べて取り除くこと。特に `.claude/settings.local.json` の `hooks`・`permissions` を確かめる。
    -   修正のフェーズの間に利用者が許可の確認に答えたことが原因なら、内容を確かめてから `begin` からやり直すこと。
- **AC-20**: 0030 `02_architecture.md` 5.3「無視されるファイル」と 0030 `01_requirements.md` 4.2 の残余のリスクの記述に、次のことが書き加えられている。
    -   本タスク（0034）で、無視される実行関連のファイルは検出するようになったこと。
    -   残る範囲は AC-18 と同じであること。

    その文書の `Comments` に、改めた日付と理由が記録されている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   新しい比較が加える時間は、1 回の `/fixpr` の実行の時間（LLM の修正のフェーズと、数十秒以上かかる build checks が占める）に対して、利用者が気づかない程度に収める。`CLAUDE.md`「Performance」に従い、数値の上限は定めず、設計で実際の作業ツリー（`extension/node_modules/` があるもの）で測る（[design_handoff.md](design_handoff.md) H-01）。

### 4.2. セキュリティ (Security)

-   条件付きのセキュリティガイド（`docs/dev/security.md`）のトリガーには該当しない。本タスクは、`yt-dlp` の起動、API キーや Webhook の URL の扱い、キャッシュのディレクトリの読み書き、ネットワークでの送信、信頼できないテキストのプロンプトへの埋め込みの、どれにも新しく関わらない。ただし、本タスクそのものは 0030 の脅威モデル（0030 `02_architecture.md` 5.1）の守りを強める変更であり、その脅威モデルに従う。
-   **信頼できない入力の境界:** 対象のファイルの名前・内容・種類は、修正のフェーズのエージェント（信頼できない主体）が書いたものでありうる。
    -   受理する形: ツールは内容を解析せず、状態として記録して比べるだけである。受理する形は定めない。
    -   出力: 名前と内容は出力に含めない（AC-15）。
    -   上限: 列挙の結果に上限があり、上限を超えた場合や読み取れない場合は `git-failed` で止まる。補正したり、一部だけを使ったりはしない（AC-16）。上限の値は設計で決める。
-   **残余のリスク:** 0030 の残余のリスクのうち「修正のフェーズのエージェントが、git が無視するファイルを変えること」（0030 `01_requirements.md` 4.2）は、本タスクの後は次の範囲に縮む。
    -   実行関連のファイルの定義に当たらない無視されるファイルを変えること。
    -   対象のファイルを変えて、確認の前に元に戻すこと。
    -   `push` の後に変えること。

    これらは引き続き、Claude Code の権限の確認（利用者の設定）に依存する。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   対象のファイルの状態を記録・比較できないときは、安全側に倒して止まる（AC-16）。
-   利用者の正当な書き込み（AC-05 の許可の確認への回答）でも止まる。この場合のやり直しの手順は、ガイドの 8 章が示す（AC-19）。

### 4.4. 互換性 (Compatibility)

-   基準値の形が変わる場合、古い版の基準値は受け付けない（AC-03）。ツールを変える PR のマージの後には再インストールが要る（`fixprguard_guide.md` 3 章・9 章）。
-   止まった種類の一覧は変えない。`git-state-changed` の `parts` の値に `ignored` が加わる。出力の契約が変わるので、`protocol` の番号と、コマンドの文書が渡す `--protocol` の扱いは、設計で決める（H-03）。
-   macOS と Linux で、同じファイルに対して同じ判定をする。実行関連のファイルの定義は、ASCII の大文字と小文字を区別せずに照合する（`internal/fixprguard/scope.go:36-38`）。

### 4.5. 保守性 (Maintainability)

-   実行関連のファイルの定義は、範囲の規則と本タスクの比較とで 1 か所に保つ。
-   完了の時点で、`make test`・`make lint`・`doc-guard`（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る（issue #424 の完了条件）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md` の「Reject, don't normalize」「Declare, don't infer」「Enforce invariants with the type, not with convention」に従う。基準値は厳密に解析し、補正しない（`internal/fixprguard/gitstate.go:121-138`）。
-   標準ライブラリ以外のモジュールを加えない（`CLAUDE.md`「Dependencies」）。
-   0030 の要件書と設計書は承認済みの文書である。改めるときは `docs/dev/developer_guide/requirements_process.md`「Editing an approved document」に従う（H-06）。

## 6. 用語集 (Glossary)

-   **基準値**（baseline）: `fixprguard begin` が出力する git の状態の識別子の組（`docs/translation_glossary.md`）。本タスクで、対象のファイルの状態を加える。
-   **実行関連のファイル**: build checks・CI・Claude Code が設定として読む、またはコンパイルして実行するファイル。定義は `fixprguard_guide.md` 6 章と `internal/fixprguard/scope.go:36-54` にある。
-   **対象のファイル**: git が無視するファイルのうち、実行関連のファイルの定義に当たるもの（3 章の冒頭）。
-   **修正のフェーズ**: `/fixpr` で、エージェントがスレッドの指摘に従ってリポジトリを編集する段階（0030 の用語）。
