# 要件定義書：codexreview の状態の比較を、この worktree の状態と HEAD のブランチに絞る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、開発用ツール `codexreview` が Codex CLI の実行の前後で比べる git の状態を、レビューする worktree の状態と HEAD のブランチに絞るための要件を定義する（[#778](https://github.com/isseis/yt2column/issues/778)）。ほかの worktree の操作で、ローカルの CLI レビューが止まらないようにする。

**背景:**

ローカルの CLI レビューは、CodeRabbit CLI が失敗したとき、`codexreview` を経て Codex CLI でレビューする（`.claude/commands/_lib/local-cli-review.md:101-116`）。`codexreview` は、Codex CLI の実行の前後で git の状態を比べ、違えば `state_changed`（終了コード `3`）を返す。共通手順は、それを `stop` として扱い、呼び出し元のコマンドごと止める（同 128 行目・157-165 行目）。

比べるものは、タスク 0049 の要件定義書の AC-08 が定めた（`docs/tasks/0049_codex_local_review/01_requirements.md:117`）。今の `codexreview` は、次の 4 つを比べる（`internal/codexreview/state.go:41-77`）。

-   この worktree の `git status`（すべての未追跡ファイルとサブモジュールの変更を含む）
-   HEAD（HEAD がいるブランチの名前と、HEAD が指す commit。`state.go:79-95`）
-   ローカルのすべてのブランチとタグと、それぞれが指すオブジェクト（`refs/heads` と `refs/tags`）
-   stash の一覧

違いがあれば、結果の `changed` に違った項目の名前（`status`・`head`・`refs`・`stash`）を入れる。`changed_refs` には、違った参照と、それを checkout している worktree を入れる（`state.go:117-164`、`internal/codexreview/types.go:40-41`）。

ブランチ、タグ、stash は、同じリポジトリのすべての worktree が共有する。そのため、別の worktree の操作でも `state_changed` になる。たとえば、並行する `/mergepr` の後片付けがローカルの `main` を fast-forward してマージ済みのブランチを消したときや、別のセッションが自分のブランチに commit したときである。0049 の設計書は、この停止を「共有の参照による停止」として承知のうえで受け入れていた（`docs/tasks/0049_codex_local_review/02_architecture.md:616`）。しかし実際に、PR #777 を作る前に行ったローカルの CLI レビューが、別の worktree の `refs/heads/main` と、別のタスクのマージ済みのブランチの変化で止まった。この worktree の `git status` と HEAD は変わっていなかった（issue #778）。

`/mergepr` を並行して実行することは多く、停止の費用は大きい。止まると、利用者の判断を待つことになる。やり直せば、Codex の割り当てをもう 1 回使うことになる。一方、CodeRabbit CLI の成否は出力だけで決まり（`local-cli-review.md:83-99`）、git の状態は比べないので、同じ状況でも止まらない。

対応 issue: [#778](https://github.com/isseis/yt2column/issues/778)（関連: タスク `0049_codex_local_review`、[ADR-0013](../../dev/adr/0013-review-setup.md)、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。対象は、何を比べ、どの変化で止まり、結果に何を示し、共通手順がそれをどう扱うかである。状態の読み方（どの git のコマンドを使うか）、使われなくなるコードの扱い、テストの作り方は、設計（`02_architecture.md`）で決める。要件の段階で挙がった設計の事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #778 で決めたこと:**

-   **比べるものを、レビューが守りたいものに絞る。** 残すのは、この worktree の `git status`、HEAD、HEAD がいるブランチの ref である。外すのは、ほかのブランチ、タグ、stash である。0049 の要件定義書 1 節が述べる目的は、レビューの範囲と、手順が後で作る commit の内容を Codex に変えさせないことである（`docs/tasks/0049_codex_local_review/01_requirements.md:45`）。残す 3 つは、この目的に直接関わる。これで、比べるものは、共通手順が各試行の前に行う確かめ（この worktree が clean であること。`local-cli-review.md:69-75`）とほぼ揃う。CodeRabbit CLI の側は、この確かめのほかに git の状態を比べない。
-   **受け入れるリスク:** Codex が読み取り専用の sandbox（`internal/codexreview/run.go:146` の `sandbox_mode="read-only"`）を破って、ほかのブランチ・タグ・stash を書き換えても、`codexreview` は気づかなくなる。sandbox が効いている限り起こらず、起きてもこのレビューの範囲には影響しない。
-   **採らない案:**
    -   **`codexreview` と `/mergepr` の排他。** `codexreview` の実行中は、`mergepr cleanup` がローカルの ref の更新を待つ。守りは弱めないが、2 つのツールにまたがる約束が増える。また、別のセッションの commit による停止は減らない。
    -   **`/mergepr` の形の変化だけを許す。** `refs/remotes/origin/*` への fast-forward と、ブランチの削除を許す。ブランチの削除を見逃すと、push していない commit の消失に気づけない。
    -   **ほかの worktree の ref の変化なら、自動でやり直す。** Codex の割り当てを余分に使う。

**要件定義の際に決めたこと（2026-11-10）:**

-   **プロセス: フルプロセスにする。** 規約は、開発用ツールの信頼境界を変える変更を簡易プロセスから除外している（`docs/dev/conventions/requirements_process.md`、「Simple Process」の「Excluded changes」）。本タスクは、Codex が読み取り専用の sandbox を破ったときの検出の範囲を狭める。これは `codexreview` の守りを変えることで、信頼境界に関わるので、除外に当たると判断した。変更が及ぶのは `codexreview` と共通手順の 2 か所で、新しい仕組みも作らない。しかし除外に当たるので、簡易プロセスは推奨しなかった。利用者はフルプロセスを選んだ。除外の判断は覆していない。
-   **結果の形は変えず、プロトコルの番号は 1 のままにする（F-002）。** 結果のメンバー（`changed`・`changed_refs` を含む）は今のまま残す。比べなくなったもの（ほかのブランチ、タグ、stash）が結果に現れなくなるだけである。`changed` に入りうる値は `status`・`head`・`refs` で、`refs` と `changed_refs` には HEAD のブランチの ref だけが入りうる。`stash` は現れない。番号を据え置くのは、上げると、次の 2 つの期間に Codex CLI の代替が働かなくなるためである。どちらの期間も `usage`（`protocol_mismatch`）が返り、共通手順はそれを止めずに記録して進む（`local-cli-review.md:131`、`cmd/codexreview/main.go:80-83`）。据え置けば、どの組み合わせでも代替は働き続ける。古いツールは、比べるものが今より多いので、止まる側に倒れる。
    -   利用者が新しいツールを入れた後、`main` をまだ取り込んでいないほかの worktree のブランチが、古い文書のまま `--protocol 1` で呼ぶ間。
    -   入れ直す前のツールを、新しい文書が呼ぶ間。
    -   利用者は、一度は `changed_refs` を外して番号を 2 に上げる案を選んだが、上の費用を示し直したうえでこの案に改めた。
    -   ADR: [ADR-0013](../../dev/adr/0013-review-setup.md) の更新に含める。
-   **タスク 0049 の文書は変えない。** 0049 の要件定義書の AC-08 と、設計書の 5.2「残るリスク」の「共有の参照による停止」は、0049 の時点の決定の記録として残す。今の比較の対象は、本書と ADR-0013 が持つ。後のタスクが前のタスクの決定を変えるときは、前のタスクの文書を書き換えず、今の決定を ADR に持たせる。これがこのリポジトリのやり方である（`CLAUDE.md`、「Design decisions (ADR)」の「Who holds what」）。0049 の文書を直すと、決定の変更として `draft` に戻し、再承認が要る（`requirements_process.md`、「Editing an approved document」）。ADR: ADR-0013 の経緯に、0049 の決定を本タスクが変えたことを書く。
-   **ADR: [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する。** 比べるものの範囲は、`codexreview` と共通手順の 2 か所に効き、受け入れるリスク（sandbox を破られたときに検出できる範囲）も決める。これは `CLAUDE.md` の「Design decisions (ADR)」の基準で主要な決定であり、セキュリティの前提と開発用ツールの方針に当たる。今は、この比較を定める ADR がない。ADR-0013 は、`codexreview` が Codex CLI をどう実行するか（設定・環境変数）を結果の節に持つ ADR なので、そこに加える。決定・採らなかった案・結果・経緯の各節に書く。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   ほかの worktree の操作（ローカルの `main` の fast-forward、マージ済みのブランチの削除、別のブランチへの commit、stash、タグの作成）で、ローカルの CLI レビューが止まらないようにする。
-   この worktree の作業ツリーと HEAD、HEAD のブランチの変化は、今と同じく見つけて止める。

### 2.2. スコープ (In Scope)

-   `codexreview` が Codex CLI の実行の前後で比べるものの範囲（F-001）。
-   `codexreview` の結果に現れる値（`changed`・`changed_refs`）。結果の形とプロトコルの番号は変えない（F-002）。
-   共通手順 `.claude/commands/_lib/local-cli-review.md` の手順 3 の、`state_changed` についての記述（F-003）。
-   ADR-0013 の更新（1 節「要件定義の際に決めたこと」）。

### 2.3. スコープ外 (Out of Scope)

-   **タスク 0049 の文書の書き換え。** 理由は 1 節「要件定義の際に決めたこと」のとおりである。
-   **Codex CLI の実行の前の確かめ（clean、`.codex/` の有無など）と、`state_changed` 以外の結果の種類。** issue が求めていないので変えない。
-   **`state_changed` のときに止まるという扱い。** 違いがあれば止めて、元に戻さずに利用者に報告するという 0049 の扱い（0049 の AC-08）は変えない。変えるのは、何を比べるかだけである。
-   **CodeRabbit CLI の側の確かめ。** issue が求めていない。
-   **`/mergepr` と `codexreview` の排他、ほかの worktree の変化に限った自動のやり直し。** issue が採らない案とした（1 節「issue #778 で決めたこと」）。
-   **git-ignore されているファイルの比較。** 今も比べておらず、読み取り専用の設定で守る（0049 の AC-08）。本タスクでも変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 比べるものの範囲

`codexreview` は、Codex CLI の実行の前後で、次のものだけを比べる。

-   `codexreview` を実行した worktree の `git status`（今と同じく、すべての未追跡ファイルとサブモジュールの変更を含む）
-   HEAD がいるブランチの名前（HEAD が detached なら、detached であること）
-   HEAD が指す commit
-   HEAD がいるブランチの ref が指す commit（HEAD がブランチにいるときだけ）

ほかのブランチ、タグ、stash は比べない。

**Acceptance Criteria**:

- **AC-01**: Codex CLI の実行の間に、HEAD がいるブランチとは別のローカルのブランチが作られるか、指す commit を変えるか、消されたとき、`codexreview` は `state_changed` を返さない。ほかの比べるものに違いがなければ、その実行の結果（指摘、指摘なし、失敗の種類のどれか）を返す。そのブランチをほかの worktree が checkout しているかどうかは問わない。
- **AC-02**: Codex CLI の実行の間に、タグが作られるか、指すオブジェクトを変えるか、消されたとき、`codexreview` は `state_changed` を返さない。ほかの比べるものに違いがなければ、その実行の結果を返す。
- **AC-03**: Codex CLI の実行の間に、stash の一覧が変わった（項目が加わった、消えた、入れ替わった）とき、`codexreview` は `state_changed` を返さない。ほかの比べるものに違いがなければ、その実行の結果を返す。
- **AC-04**: Codex CLI の実行の間に、この worktree の `git status` が変わったとき（追跡しているファイルの変更、未追跡のファイルの追加、サブモジュールの変更を含む）、`codexreview` は今と同じく `state_changed` を返す。
- **AC-05**: Codex CLI の実行の間に、HEAD がいるブランチの名前が変わったとき（別のブランチへの checkout、detached への切り替え、detached からブランチへの切り替え）、または HEAD が指す commit が変わったとき、`codexreview` は今と同じく `state_changed` を返す。
- **AC-06**: HEAD がブランチにいるとき、Codex CLI の実行の間にそのブランチの ref が別の commit を指すようになれば、`codexreview` は `state_changed` を返す。変化を起こしたのがこの worktree か、ほかの worktree かは問わない。
- **AC-07**: AC-04 から AC-06 の違いがあるとき、`state_changed` は今と同じく、ほかのどの結果の種類よりも優先される。
- **AC-08**: 比べるものを読めないときの扱いは今と変えない。
    1. Codex CLI の実行の前に読めないとき、`codexreview` は Codex CLI を実行せず、種類 `precondition`、`detail` `git_failed` を返す（`internal/codexreview/run.go:307-309`）。共通手順は、それを `record` として扱う（`local-cli-review.md:130`）。
    2. Codex CLI の実行の後に読めないとき、`codexreview` は結果を返さずに失敗する（`run.go:416-421`）。共通手順は、それを「使える結果がない」として `stop` で扱う（`local-cli-review.md:145-149`）。実行の間に HEAD のブランチの ref が消され、HEAD が commit を指さなくなったときも、これに当たる。
    3. 読んだ出力が、比べるものの決まった形に合わないときも、読めないときとして扱う。補って比べることはしない。

#### F-002: 結果に現れる値

結果の形とプロトコルの番号は変えない。比べるものの範囲（F-001）に合わせて、結果に現れる値だけが変わる。

**Acceptance Criteria**:

- **AC-09**: `state_changed` の結果の `changed` は、違った項目を `status`・`head`・`refs` のうち当てはまるものだけ、この順に並べる。AC-04 の違いは `status`、AC-05 の違いは `head` で示す。AC-06 の違い（前後で HEAD が同じブランチにいて、その ref が動いた）は `head` と `refs` の両方で示す。そのため、この worktree での commit も `head` と `refs` になる。HEAD のブランチの名前が変わったとき（detached への切り替えと、detached からの切り替えを含む）は、`head` だけで示し、`refs` は現れない。`stash` は、どの結果にも現れない。
- **AC-10**: `state_changed` の結果の `changed_refs` には、AC-06 の違いがあるとき（AC-09 で `refs` を示すとき）だけ、HEAD のブランチの ref が 1 つ入る。今と同じく、それを checkout している worktree が併記される。AC-01 と AC-02 のブランチとタグは、`changed_refs` に現れない。AC-06 の違いがないときの `changed_refs` は、今と同じく空の配列である。
- **AC-11**: `codexreview` のプロトコルの番号は `1` のままである。結果のメンバー（`protocol`・`kind`・`detail`・`changed`・`changed_refs`・`codex_version`・`run_dir`・`instructions_changed`・`findings`）の名前は変えない。共通手順は、今と同じく `--protocol 1` で呼ぶ。

#### F-003: 共通手順の記述

共通手順の記述を、比べるものの範囲（F-001）に合わせる。

**Acceptance Criteria**:

- **AC-12**: 共通手順の手順 3 の今の文（`local-cli-review.md:163-165`）を改める。この文は、ほかの worktree のセッションの commit や stash による `state_changed` も止めると述べている。改めた文は、次の 2 つを述べる。
    1. ほかの worktree の操作でも、この worktree の HEAD のブランチを動かせば `state_changed` になり、その場合も止める。
    2. ほかのブランチ、タグ、stash の変化では `state_changed` にならない。

    `state_changed` の扱いが `stop` であること、`stop` のときに報告するもの（`kind`・`detail`・`changed`・`changed_refs`・`run_dir` など）は変えない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比べるものが減るだけで、Codex CLI の実行（数十秒から数分）に比べて無視できる。

### 4.2. セキュリティ (Security)

-   `docs/dev/security.md` の適用は N/A とする。条件付きのガイドを適用する条件（`_context.md` の「Conditional-guide trigger」）は、製品が外部のコマンドを起動するか、秘密を扱うか、ネットワークに送るかなどを挙げる。本タスクは、そのどれも加えず、変えない。`codexreview` が Codex CLI を経て差分を OpenAI に送ることも変えない。security.md の規則と前提も変えない。開発用ツールの信頼境界に関わる点（1 節）は、security.md ではなく、下の「受け入れるリスク」と ADR-0013 で扱う。
-   **受け入れるリスク:** Codex が読み取り専用の sandbox を破って、HEAD のブランチ以外のブランチ、タグ、stash を書き換えても、`codexreview` は気づかない（AC-01〜AC-03）。HEAD が detached のときは、どのブランチの変化にも気づかない。sandbox が効いている限り起こらない。起きても、この worktree の `git status`、HEAD、HEAD のブランチの ref は引き続き比べる（AC-04〜AC-06）ので、これらの変化は今と同じく見つかる。ただし、push していないほかのブランチの commit が失われても、`codexreview` からは分からない。共有の `.git/config`・hooks など、今も比べていないものの扱いは変えない。
-   **信頼できない入力:** 比べるものは git の出力から読む。Codex がそれに影響しうるが、出力は前後で比べるだけで、指示として扱わない。決まった形に合わない出力は、補わずに読めないときとして扱う（AC-08 の 3）。
-   読み取り専用の sandbox、`codex` に渡す環境変数と設定の制限は変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   ほかの worktree の操作では止まらなくなる（AC-01〜AC-03）。この worktree の変化では、今と同じく止まる（AC-04〜AC-06）。
-   比べるものを読めないときの扱いは変えない（AC-08）。

### 4.4. 互換性 (Compatibility)

-   結果の形とプロトコルの番号は変えない（AC-11）。入れ直す前のツールは、今のまま、ほかのブランチ・タグ・stash の変化でも止まる。利用者は、マージの後に `make install-codexreview` で入れ直す（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の決定「道具は `main` からインストールして使う」）。
-   `state_changed` の種類の名前、終了コード `3`、共通手順での扱い（`stop`）は変えない。

### 4.5. 保守性 (Maintainability)

-   N/A。比べるものの範囲を変えるだけで、保守性に関わる新しい要件はない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `codexreview` は、エージェントがインストールもビルドもしない。利用者が `main` から入れる（`CLAUDE.md`、「Tool Execution Safety」）。
-   ADR-0014 の決定（決定的な判定を、テストのある Go の開発用ツールに置く）に従い、比べるものの判定は `codexreview` に置く。

## 6. 用語集 (Glossary)

-   **HEAD のブランチ:** HEAD がいるブランチ。HEAD が detached のときはない。
-   **比べるもの:** `codexreview` が Codex CLI の実行の前と後で読み、違いがあれば `state_changed` にするもの。本書の F-001 が定める。
-   **ほかの worktree:** 同じリポジトリの、`codexreview` を実行した worktree 以外の worktree。ブランチ、タグ、stash は、すべての worktree が共有する。
-   ほかの用語は `docs/translation_glossary.md` と統一する。

## 変更履歴

- 2026-11-10 承認（isseis、#788）
