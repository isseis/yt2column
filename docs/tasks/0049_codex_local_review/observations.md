# 試験運用の観測の記録（0049 codex_local_review）

[01_requirements.md](01_requirements.md) の F-001（AC-01・AC-02）の観測の記録。設計（`02_architecture.md`）は、この記録を入力にする。

## 観測の条件

-   **日付:** 2026-10-28（各実行の時刻は UTC）。
-   **`codex` CLI の版:** `codex-cli 0.163.0`（`codex --version`）。
-   **モデルと推論の強さ:** `gpt-6.1-sol`、`medium`。どちらも利用者の `~/.codex/config.toml` の値で、コマンドの引数では指定していない。R4（`CODEX_HOME` を空にした実行）では、設定ファイルがないため推論の強さが `none` と表示された。
-   **承認:** `codex` の各実行は、実行の前に 1 回ずつ利用者の承認を得た（要件定義書 1 節の決定）。
-   **対象のリポジトリ:** 本物のリポジトリではなく、スクラッチパッドに複製した使い捨てのリポジトリ（以下 fixture。`git clone --no-hardlinks` で作った）で実行した。`main`（`12f38cf`）の上に次の 3 つのコミットを作り、作業ツリーにも変更を残した。どの場所のバグがレビューで指摘されるかで、範囲を確かめる。

| 場所 | 内容 | 範囲に入るべきか |
|---|---|---|
| コミット `ad32c34`（BASE） | `internal/zztrial/outside.go` を加える。範囲外のバグ（配列の範囲外の添字）を含む | 入らない（BASE 自身） |
| コミット `f6b0aa6` | `internal/zztrial/inside.go` を加える。範囲内のバグ（nil の map への書き込み）を含む | 入る |
| コミット `c220f50`（HEAD） | `internal/zztrial/inside2.go` を加える。バグのない関数 | 入る |
| コミットしていない変更 | `outside.go` に関数を足す。バグ（定数のゼロ除算）を含む | 入らない |
| git に追加していないファイル | `internal/zztrial/untracked.go`。バグ（負の添字）を含む | 入らない |
| git-ignore されているファイル | `.env`。ダミーの値だけを入れた | 入らない（読まれたかを確かめる） |

BASE（`ad32c34`）は fixture のブランチの分岐点（`12f38cf`）ではない。この BASE は、`/fixpr` の Phase 4.5 の `<commitSha>^` や、`/runplan` のフェーズ群の始まりに当たる。

## 実行の一覧

`<BASE>` は `ad32c34fcbc78858c2ba2e047d2b3bd966705488`、`<CLEAN>` は `c220f508db57c13329016401eb6ae3fbc4c3099d` である。`<scratch>` はスクラッチパッドのディレクトリを表す。どの実行にも `-c 'sandbox_mode="read-only"' -c 'approval_policy="never"'` を付けた（以下、表では `<RO>` と書く）。

| 実行 | 開始（UTC） | 作業ディレクトリ | コマンド | 終了コード | 時間 |
|---|---|---|---|---|---|
| R1 | 12:12:27 | fixture（コミットしていない変更と、git に追加していないファイルがある） | `codex exec review --base <BASE> <RO> --json -o <scratch>/last.md` | 0 | 36 秒 |
| R2 | 12:20:55 | fixture の HEAD から `git worktree add --detach` で作った一時 worktree（変更なし） | `codex review --base <BASE> <RO>` | 0 | 33 秒 |
| R3 | 12:22:13 | R2 と同じ一時 worktree | `codex exec review --commit <CLEAN> <RO> --json -o <scratch>/last.md` | 0 | 15 秒 |
| R4 | 12:26:29 | R2 と同じ一時 worktree | `env CODEX_HOME=<scratch>/empty codex review --commit <CLEAN> <RO>`（認証情報なし） | 1 | 17 秒 |
| R5 | 12:26:56 | R2 と同じ一時 worktree | `env HTTPS_PROXY=http://127.0.0.1:9 …（http・all も同じ） codex exec review --commit <CLEAN> <RO> --json`（届かないプロキシ） | 終わらなかったため、300 秒で `SIGALRM` を送って打ち切った（終了コード 142） | 300 秒以上 |

## AC-01 の各項目

### 1. 割り当て

-   **結論:** PR レビューと同じ割り当てを消費すると判断した。ただし、PR レビューが同じ割り当てを消費することは、直接には観測していない（下記）。
-   **ローカルの記録:** 各実行のセッションの記録（`~/.codex/sessions/2026/10/28/rollout-*.jsonl` の `token_count` の `rate_limits`）では、割り当ては `limit_id: "codex"` で、5 時間の区間（`window_minutes: 300`）と 1 週間の区間（`10080`）ごとの使用率がある。5 時間の区間の使用率は、R1 の間に 2% から 3% に、R2 の間に 3% から 4% に上がり、R3 の間は 4% のままだった。1 週間の区間の使用率は 4% のままだった。R4・R5 にはこの記録がない。
-   **送り先:** ChatGPT のプランでログインした実行は `chatgpt.com/backend-api/codex/responses` に送る（R5 の標準エラーに出た接続先）。認証情報がないとき（R4）は `api.openai.com/v1/responses` に送った。
-   **判断の根拠:** 0031 の観測で、Codex の割り当ては Codex・Work・Workspace Agents・ChatGPT for Excel で共通であり、コードレビューに固有の上限の表示はないと分かっている（`docs/tasks/0031_codex_pr_review/observations.md` の「5. 割り当て」）。ローカルの実行も、ChatGPT のプランに付く Codex の割り当て（`limit_id: "codex"`）を消費する。したがって、PR レビューと同じ割り当てを消費すると判断した。
-   **観測していないこと:** PR レビューの前後でローカルの使用率を比べることはしていない。使用率を読むには `codex` を実行する必要があり、そのたびに割り当てを使うためである。また、利用者のほかの ChatGPT の利用も同じ使用率に入るので、比べても PR レビューの分だけを取り出せない。
-   **1 回の消費:** 5 時間の区間の使用率で 1% ほど（R1・R2）。指摘のない小さなコミット（R3）では、表示の上では増えなかった。

### 2. コミットの範囲

-   **結論:** `--base <BASE の SHA>` だけでは、コミットしていない変更が範囲に入ってしまい、成り立たない。HEAD を切り離した、変更のない一時 worktree で実行すれば、BASE から HEAD までのコミット済みの変更だけが範囲になった。
-   **`--base` にコミットの SHA を渡せるか:** 渡せた。ヘルプには `--base <BRANCH>` とあるが、SHA を受け付けた。Codex に渡る指示は「ブランチ `<BASE>` に対する変更をレビューせよ。マージベースは `<BASE>` であり、`git diff <BASE>` で変更を見よ」という形だった（R1 のセッションの記録のうち、レビューを行う側のスレッドの最初の利用者のメッセージ。スレッドが 2 つあることは 5 の「構造化された結果」で述べる）。BASE が HEAD の祖先なので、マージベースは BASE 自身になる。BASE がブランチの分岐点でなくても、BASE より前のコミット（`outside.go` の範囲外のバグ）は指摘されなかった（R1・R2）。
-   **コミットしていない変更（R1）:** 範囲に入った。Codex は指示どおり `git diff <BASE>`（HEAD を指定しない形）を実行した。この形は作業ツリーと BASE の差分を出すので、コミットしていない `outside.go` の変更も含まれる。指摘 2 件のうち 1 件（優先度 P1。優先度は 5 で述べる）が、このコミットしていない変更のバグだった。
-   **git に追加していないファイル（R1）:** 範囲に入らなかった。`git diff` は追跡していないファイルを含まず、`untracked.go` のバグは指摘されなかった。ただし Codex は `git status --short` と `ls -la internal/zztrial` を実行しており、ファイルの存在は見ている。
-   **一時 worktree（R2）:** fixture の HEAD から `git worktree add --detach` で作った worktree には、コミットしていない変更も、git に追加していないファイルも、git-ignore されているファイルもない。ここで同じ `--base <BASE>` を実行すると、指摘は範囲内の `inside.go` の 1 件だけで、`git diff --stat <BASE>` も `inside.go` と `inside2.go` の 2 ファイルだけを示した。一時 worktree は（`--detach` のため）ブランチなどの参照を作らないが、`.git/worktrees/` に管理用の情報を作る。
-   **`--commit <SHA>`（R3）:** 1 つのコミットだけを範囲にできた。Codex は `git show <SHA>` で変更を見た。

### 3. 作業ツリーに書き込ませない設定

-   **結論:** `-c 'sandbox_mode="read-only"'` で実行でき、どの実行（R1〜R5）でも、前後で次の値は変わらなかった。
    -   `git status --short` の出力
    -   git-ignore されているファイル（`git status --short --ignored` の `!!` の行と、`.env` の SHA-1）。一時 worktree には `.env` がないので、R2〜R5 では「ファイルがない」ことが変わらなかった
    -   `HEAD`
    -   ブランチとタグの一覧（`git for-each-ref refs/heads refs/tags`。各参照の指すコミットも含む）
    -   stash の一覧
    -   コミットしていないファイルと git に追加していないファイルの内容（SHA-1）
-   **設定が効いていること:** 標準エラーと `turn_context` の記録に `sandbox: read-only`（`sandbox_policy: {"type":"read-only"}`）と出た。R2 では、Codex が実行した `git` が `/tmp` にキャッシュのファイルを作れず、`Operation not permitted` の警告を出した。サンドボックスが書き込みを止めた跡である。
-   **読み取りの範囲:** 同じ記録の権限の項目は、読み取りを `:root`（ファイルシステム全体）に許していた。書き込みを止める設定は、読み取りを止めない。
-   **試していないこと:** Codex に書き込みを試みさせる指示（たとえば修正の適用）は与えていない。書き込みを止めたことの直接の跡は、上の `/tmp` の 1 件だけである。

### 4. 非対話の実行

-   **結論:** 承認を求めて止まることなく、最後まで実行できた（R1〜R3）。`-c 'approval_policy="never"'` を付け、標準入力は `/dev/null` にした。`codex review` と `codex exec review` のどちらも、端末を必要としなかった。
-   R4（認証の失敗）は 17 秒で終わった。R5（ネットワークの失敗）は終わらなかった（後述の 6）。

### 5. 出力の形

#### 素の `codex review`（R2・R4）

-   **標準出力:** 最終的なレビュー結果だけ（R2 は 713 バイト）。失敗したとき（R4）は空だった。
-   **標準エラー:** 版の行、設定の表示（`workdir`・`model`・`provider`・`approval`・`sandbox`・`reasoning effort`・`session id`）、Codex が実行した各コマンドとその出力の全文（`exec` の節）、最終的なレビュー結果（`codex` の節）。R2 では約 130KB だった。Codex が `cat CLAUDE.md` などで読んだファイルの中身がそのまま出るためである。

#### `codex exec review --json`（R1・R3・R5）

-   **標準出力:** 1 行に 1 つの JSON（JSONL）。`thread.started`、`turn.started`、コマンドごとの `item.started` と `item.completed`（`type: command_execution`。`command`・`aggregated_output`・`exit_code` を持つ）、最後の `item.completed`（`type: agent_message`。`text` が最終的なレビュー結果）、`turn.completed` の順に出た。R1 は約 127KB、R3 は約 68KB だった。大きさのほとんどは `aggregated_output`（Codex が読んだファイルの中身）である。`turn.completed` の `usage` は、どの数も 0 だった。
-   **`-o <FILE>`:** 最終的なレビュー結果（`agent_message` の `text` と同じ文面）だけが書かれた。R1 は 1,079 バイト、R3 は 165 バイトだった。
-   **失敗したとき（R5）:** `type: error` の行（`message` に再接続の状況）が続いた。

#### レビューの結果の文面

指摘があるときは、次の形だった（R1・R2）。

```text
<全体の要約の段落>

Full review comments:          ← 指摘が 2 件のとき（R1）。1 件のとき（R2）は "Review comment:"

- [P<優先度>] <題名> — <ファイルの絶対パス>:<開始行>-<終了行>
  <本文の段落>
```

-   **位置:** ファイルは絶対パス（作業ディレクトリの下のパス）で、行は `開始-終了` の範囲である（1 行だけでも `10-10` の形）。一時 worktree で実行すると、パスは一時 worktree の下になる。
-   **優先度:** `[P1]`・`[P2]` が出た。
-   **言語:** 題名と本文、要約は日本語だった。Codex は `AGENTS.md` と `CLAUDE.md` を読んでおり（後述の 7）、`CLAUDE.md` の会話の言語の規則に従ったとみられる。言語を指定する引数は付けていない。
-   **見出しが変わる:** 指摘の数によって見出しの文字列が `Full review comments:` と `Review comment:` で変わった。

指摘がないとき（R3）は、要約の段落 1 つだけで、見出しも指摘の行もなかった。要約は、不具合が見つからなかったことを日本語の文で述べていた。文面だけから「指摘なし」を判定するには、見出しと `- [P` の行がないことに頼ることになる。

#### 構造化された結果（セッションの記録）

構造化された結果は、標準出力にも `-o` のファイルにも出ない。セッションの記録（`~/.codex/sessions/<年>/<月>/<日>/rollout-*.jsonl`）のうち、呼び出した側のスレッドの記録（下記）に、`type: event_msg`・`payload.type: item_completed`・`payload.item.type: ExitedReviewMode` の行があり、その `payload.item.review_output` に入っていた（R1・R2・R3）。

```json
{
  "findings": [
    {
      "title": "[P2] <題名>",
      "body": "<本文>",
      "confidence_score": 1.0,
      "priority": 2,
      "code_location": {
        "absolute_file_path": "<ファイルの絶対パス>",
        "line_range": { "start": 5, "end": 7 }
      }
    }
  ],
  "overall_correctness": "patch is incorrect",
  "overall_explanation": "<要約>",
  "overall_confidence_score": 0.99
}
```

-   指摘がないとき（R3）は `findings: []`、`overall_correctness: "patch is correct"` だった。
-   1 回の実行で、セッションの記録が 2 ファイルできた。呼び出した側のスレッドの記録と、レビューを行う側のスレッドの記録である。`ExitedReviewMode` は前者に、`rate_limits`（上の 1）と Codex への指示（上の 2）は後者にあった。ファイル名にはそれぞれのスレッドの ID が入る。R1 では、呼び出した側の記録のファイル名の ID が、`--json` の `thread.started` の `thread_id` と一致した。
-   `--ephemeral` を付けるとセッションを残さないので、この記録も残らないとみられる（試していない）。
-   この記録の形は公開された仕様として確かめていない。版が変われば変わりうる。

### 6. 失敗したとき

-   **認証がない（R4）:** 終了コード 1、標準出力は空、17 秒で終わった。標準エラーには、WebSocket の接続が `401 Unauthorized` で失敗し、5 回の再接続の後に HTTPS に切り替えて、さらに 5 回再接続して失敗したことが出力された。最後の行は `ERROR: unexpected status 401 Unauthorized: …` で、その前に `codex` の節として「レビューが中断された」という旨の 1 行があった。
-   **ネットワークに接続できない（R5）:** 300 秒たっても終わらなかった。JSONL には、WebSocket の再接続（2/5〜5/5）と HTTPS への切り替えの後、`Reconnecting... waiting for network (…)` という `error` の行が 8 回出た。それ以後もネットワークを待ち続けた。300 秒で `SIGALRM` を送って打ち切った。終了コード 142 はこの打ち切りによるもので、`codex` が返したものではない。打ち切りの後に `codex` のプロセスは残っていなかった。時間の上限（AC-09 の 3）を手順の側で設けないと、ネットワークの障害で止まったままになる。
-   **利用の上限:** 観測していない。上限まで使うと、利用者のほかの ChatGPT の利用にも響くためである。

### 7. Codex が読んだもの

Codex が実行したコマンドは、R1・R3 では JSONL の `command_execution` の `command` から、R2 では標準エラーの `exec` の節から取り出した。どの実行も、コマンドの数は 2〜3 で、1 つのコマンドの中で `;` でつないで複数の読み取りを行った。

| 実行 | 読んだもの |
|---|---|
| R1 | `pwd`、`git diff --stat <BASE>`、`git diff <BASE>`、`git status --short`、`git log -3`、`ls -la internal/zztrial`、`outside.go`・`inside.go`、`AGENTS.md`、`CLAUDE.md`、`docs/dev/security.md`、`docs/dev/developer_guide/test_organization.md`、`docs/dev/developer_guide/requirements_process.md`、`internal` の下の `AGENTS.md` などの検索 |
| R2 | `pwd`、`git diff --stat <BASE>`、`git diff <BASE>`、`git log -3`、`find .. -name AGENTS.md -o -name AGENTS.override.md`、`CLAUDE.md`（全文と一部の再読）、`AGENTS.md`、R1 と同じ 3 つの文書、`rg 'zztrial|func Counts' docs internal/zztrial`、`inside.go` |
| R3 | `git status --short`、`git show --stat <CLEAN>`、`git show <CLEAN>`、`AGENTS.md`、`CLAUDE.md`、`ls internal/zztrial` と `internal/zztrial/*`（`inside.go`・`inside2.go`・`outside.go`）、`test_organization.md`、`requirements_process.md`、`internal` の下の `AGENTS.md` などの検索 |

-   **差分の外のファイル:** どの実行も、差分の外のファイルを読んだ。`AGENTS.md` と `CLAUDE.md` は毎回、`AGENTS.md` が挙げる文書（`security.md`・`test_organization.md`・`requirements_process.md`）もほぼ毎回読んだ。R3 は差分にない `outside.go` と `inside.go` も読んだ。
-   **作業ディレクトリの外:** R2 は `find ..` で作業ディレクトリの親（スクラッチパッド）を探し、別の worktree（fixture）の `AGENTS.md` を見つけた。読み取りがファイルシステム全体に許されていること（上の 3）と合わせると、作業ディレクトリの外も読みうる。
-   **秘密のファイル:** `.env` を読んだ跡はなかった。`.env` のダミーの値は、どの実行の出力にも、セッションの記録にも現れなかった。ただし `.env` があったのは R1 の fixture だけで、R2〜R5 の一時 worktree には `.env` がない。
-   **指示ファイル:** 指示しなくても `AGENTS.md` と `CLAUDE.md` を読み、`AGENTS.md` の内容はレビューを行う側のスレッドの指示にも入っていた（セッションの記録の `# AGENTS.md instructions for <作業ディレクトリ>`）。

### 8. 1 回の実行の時間

| 実行 | 内容 | 時間 |
|---|---|---|
| R1 | 3 ファイルの差分（コミットしていない変更を含む）、指摘 2 件 | 36 秒 |
| R2 | 2 ファイルの差分、指摘 1 件 | 33 秒 |
| R3 | 1 ファイルのコミット、指摘なし | 15 秒 |
| R4 | 認証の失敗 | 17 秒 |
| R5 | ネットワークの失敗 | 終わらず（300 秒で打ち切り） |

差分は数十行で、実際の変更より小さい。大きな差分での時間は観測していない。

## AC-02 の判定

| 項目 | 結果 |
|---|---|
| 2. コミットの範囲 | **そのままでは成り立たない。** `--base <BASE>` は作業ツリーとの差分を見るので、コミットしていない変更が範囲に入る。HEAD を切り離した変更のない一時 worktree で実行すれば成り立った（R2） |
| 3. 書き込ませない設定 | 成り立った（`sandbox_mode="read-only"`）。ただし読み取りはファイルシステム全体に許される |
| 4. 非対話の実行 | 成り立った（`approval_policy="never"`）。ただしネットワークの障害では終わらないので、時間の上限が要る |
| 5. 出力の読み取り | 成り立った。文面は指摘の数で見出しが変わる。構造化された結果はセッションの記録にだけある |

項目 2 は、そのままでは成り立たない。AC-02 に従い、設計に進む前にこの結果を利用者に報告し、続けるかどうかは利用者が決める。続ける場合に設計で選べる方法は、少なくとも次の 3 つである（`design_handoff.md` の H-01）。

-   一時 worktree を作ってそこで実行する（R2 で確かめた）。AC-08 の「手順が実行の前に自分で作り、実行の後に消すもの」に当たる。
-   作業ツリーに変更がないことを実行の前に確かめ、変更があれば Codex CLI のレビューの失敗とする（観測していない）。
-   範囲をレビューの指示（PROMPT）で伝える（観測していない。`--base` と PROMPT を同時に使えるかも確かめていない）。

AC-01 の 1 で割り当てを共有すると判断したことは、AC-02 により、進むのを妨げない（要件定義書 1 節の決定）。
