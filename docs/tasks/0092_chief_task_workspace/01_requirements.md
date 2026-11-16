# 要件定義書：chief が issue からタスク用の worktree と workspace を作り、タスクが終わったら確かめて消す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-16 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、chief（[0088 要件定義書](../0088_chief_of_staff/01_requirements.md)）の機能を 2 つ定義する。1 つは、人が示した issue から、タスク用の git の worktree と Herdr の workspace を作り、エージェントを起動して `/mkreq` を送ること。もう 1 つは、タスクが終わった workspace を、人に確かめたうえで消すことである。

**背景:**

今は、タスクを始めるたびに人が手で worktree と workspace を作り、シェル・OpenCode・Claude Code の pane を開いている（[chief 利用ガイド](../../dev/guides/chief_guide.md) の「1. 前提条件」の「workspace の作り」）。chief の今の振る舞いは次のとおりである。

-   chief は、タスクのない workspace のうち、エージェントが入力を受け付ける状態のものへ、人が示した issue の `/mkreq` を送る（0088 要件定義書の AC-38、`internal/chief/request.go:345`）。その workspace は人が用意する（0088 要件定義書の「要件定義の際に決めたこと（2026-11-11）」の `/mkreq` の項）。
-   chief はタスクを worktree のブランチから読む。HEAD が detached の worktree は、ブランチからタスクの有無が分からないので「要確認」にする（`internal/chief/state.go:259-262`）。この行は、表のほかのすべての行の後に並ぶ（0088 要件定義書の AC-03）。
-   `/runpr` は、タスクの最後のプランの PR をマージした後、HEAD を `origin/main` へ detached にし（`git switch --detach origin/main`）、要件定義書の `対応 issue` を閉じる（`.claude/commands/runpr.md:1255-1281`）。そのため、完了したタスクの worktree からは、ブランチでタスクを読めない。
-   リポジトリの `.claude/settings.json` は、Claude Code のセッションに `herdr agent start`・`herdr workspace close`・`herdr worktree remove` などの呼び出しを禁じている（`.claude/settings.json:8-32`、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)）。道具 `chief` は子のプロセスとして `herdr` を呼ぶので、この拒否の影響を受けない（chief 利用ガイドの「6. `.claude/settings.json` の拒否」）。
-   chief の記録は、送るコマンドとタスクを決めるためには使わない（ADR-0046 の決定）。

対応 issue: [#839](https://github.com/isseis/yt2column/issues/839)（親: [#836](https://github.com/isseis/yt2column/issues/836)、関連: [#837](https://github.com/isseis/yt2column/issues/837)・[#838](https://github.com/isseis/yt2column/issues/838)（タスク 0088））

**本書の記述範囲:** 本書は、chief が何を作り、何を送り、何を消し、何を消さないかという観測できる振る舞いを定める。次の事項は設計（`02_architecture.md`）で決める。道具のサブコマンドと引数の形、Herdr と git の呼び出し方（`herdr worktree create` を使うかなど）、pane の配置、エージェントが起動したことの確かめ方と待つ時間、作ったことの記録の置き場所と形、問いと表示の文言、テストの手段。要件の段階で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #839 で決めたこと:**

-   **タスク用の worktree を git で作り、Herdr の workspace をその worktree のディレクトリで開く。シェルのほかに 2 つの pane を開いて、OpenCode と Claude Code を起動する。** 今は人が手で行っている形を、そのまま自動にするためである。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する。
-   **人は chief で issue 番号を答えるだけでよい。worktree のパス、workspace の名前は chief が決め、作り、エージェントを起動し、Claude Code の pane に `/mkreq <issue>` を送る。** 人が名前を考えたり、`git worktree` や Herdr を手で操作したりしなくて済むようにするためである（0088 の PR-7 の実地の確かめの後に、人が求めた。2026-11-16）。ADR: ADR-0046 を更新する。
-   **タスクのすべての PR がマージされたら、chief が人に確かめたうえで workspace と worktree を消す。消す前に、未コミットの変更と push していないコミットがないことを確かめる。** 作業を失わずに、終わった workspace を片付けるためである。ADR: ADR-0046 を更新する。
-   **タスクのブランチとタスクのディレクトリの名前は、今までどおり `/mkreq` が issue から決める。** 名前を二重に決めないためである。ADR: なし（今の `/mkreq` の規則を変えないため）。

**要件定義の際に決めたこと（2026-11-16）:**

-   **進め方は通常の進め方にする。対象外の判断: 対象外である（上書きなし）。** chief が、リポジトリの外にディレクトリを作って消し、ほかのエージェントを起動するようになる。これは開発用ツールの信頼の境界を変えるので、簡易な進め方の対象外である（[requirements_process.md](../../dev/conventions/requirements_process.md) の「Simple Process」の「Excluded changes」）。親 issue #836 も、chief の信頼の境界を簡易な進め方の対象外としている。ADR-0046 の決定を変え、道具、コマンドの定義、CLAUDE.md、利用ガイドに及ぶことも理由である。ADR: なし（進め方の選択はこのタスクの中で閉じるため）。
-   **worktree のパスと workspace の名前は、issue 番号から機械的に決める。パスは `~/.herdr/worktrees/yt2column/issue-<N>`、workspace の名前は `issue-<N>` とする（`<N>` は issue 番号）（F-002）。** chief が issue の題名などの中身で振る舞いを選ばずに済み、CLAUDE.md の「Declare, don't infer」に合うためである。パスに、chief が送れない文字（`_` など。chief 利用ガイドの「8. 既知の制限」の「worktree のパスの文字」）も入らない。人に確かめる問いも増えない。`/mkreq` の後は、表の行にタスクの番号が出るので区別できる。issue の内容から名前を付けて人が確かめる案は採らない。ADR: ADR-0046 を更新する。
-   **作る worktree の HEAD は、`origin/main` から detached にする（F-002）。** `/mkreq` は `origin/main` からタスクのブランチを作る（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part A の step 4）。一時的なブランチを作ると、`/mkreq` がタスクのブランチへ移った後に残る。作った直後の worktree は、`/mkreq` がブランチを作るまで「要確認」になる（`internal/chief/state.go:259-262`）。ただし chief は作った直後に `/mkreq` を送る（F-003）ので、妨げにならない。一時的なブランチ `issue-<N>` を作る案は採らない。ADR: ADR-0046 を更新する。
-   **chief が削除を提案するのは、chief が作った workspace のうち、作ったときの issue のタスクが完了したものだけにする（F-005）。** 人が手で作った workspace は対象にしない。完了したタスクの worktree は `origin/main` で detached になり、ブランチからタスクを読めない。そのため chief は、作ったときに記録した issue 番号から、その issue を `対応 issue` とするタスクを `origin/main` で探し、完了したかを確かめる。これは、記録を削除の候補を名指すことにも使うという点で、ADR-0046 の記録の決定を広げる。名指された workspace は、git、Herdr、`origin/main` の文書で確かめ直す。記録を送るコマンドとタスクの決定に使わないことは変えない。chief が作った workspace のうちタスクのないもの（`/mkreq` が止まったもの）を対象に含める案は採らない。ADR: ADR-0046 を更新する。
-   **worktree・workspace の作成と削除、エージェントの起動は、道具 `chief` だけが行う。セッションへの `.claude/settings.json` の拒否は緩めない（F-002・F-006、4.2）。** chief のセッションが道具に渡すのは、人が答えた issue 番号と、人が選んだ workspace だけである。パスと名前は道具が決める。0088 と同じく、セッションが信頼できない文字列に従っても、決まった範囲の外の操作をしないようにするためである（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、ADR-0046）。拒否を緩めてセッションに herdr を直接呼ばせる案は採らない。ADR: ADR-0046 を更新する。
-   **作る前に、GitHub から issue を読んで確かめる。存在しない issue、閉じた issue、PR の番号は拒み、題名を示す（F-001）。chief を起動したことで承認する読み取り専用のネットワークの操作に、issue を読む操作を加える（4.2、5 章）。** 番号を打ち間違えたときに、違う issue の workspace を作らないためである。確かめずに `/mkreq` の確かめに任せる案は採らない。その案では、`/mkreq` が止まっても workspace が残る。ADR: ADR-0046 を更新する。
-   **消す前の確かめは、次の厳しい形にする（F-006）。追跡していないファイルを含めて変更がないこと。HEAD が `origin/main` から届くこと。workspace のどのエージェントも実行中でなく、質問でも止まっていないこと。** issue が挙げる 2 つの条件に加え、エージェントの作業を途中で失わないためである。完了したタスクの worktree は `origin/main` で detached になっているはずなので、普通の場合はこの形でも妨げにならない。未コミットの変更がなく、HEAD が origin のどれかのブランチから届けばよく、エージェントの状態を問わない案は採らない。ADR: ADR-0046 を更新する。
-   **issue を閉じること（issue #839 の「やること」の 4）は、このタスクの範囲外とする。** `/runpr` が、タスクの最後のプランの PR の後に、すでに `対応 issue` を閉じている（`.claude/commands/runpr.md:1257-1281`）ためである。ADR: なし（chief の振る舞いを変えないため）。
-   **作成の途中で失敗したときは、止まって、作ったものと失敗した段を示す。作ったものは消さず、`/mkreq` も送らない（AC-09）。** 消す操作を減らし、片付けは人が確かめてから行うためである。作ったものを消してから止まる案は採らない。ADR: なし（ADR-0046 の「分からないときは止まって示す」の範囲に収まるため）。
-   **`issue-<N>` のパスか worktree か workspace がすでにあるときは、何も作らずに拒む（AC-07）。** その issue のタスクがすでにあるかは、`/mkreq` の step 1 が確かめるので、chief は確かめない。すでにあるものへ `/mkreq` を送る案は採らない。ADR: なし。
-   **タスクのない既存の workspace へ `/mkreq` を送る今の流れ（0088 の AC-38）は残し、新しい workspace を作ることを選択肢に加える（F-004）。** 人が手で作った workspace でも、今までどおり始められるようにするためである。新しく作ることだけにする案は採らない。ADR: なし。
-   **作った後に `/mkreq` を送れなかったときは、送れなかったことと理由を示し、人がその pane で `/mkreq <N>` を打つ。chief は、その workspace に `/mkreq` を改めて提案しない（AC-13）。** 作った直後の worktree は detached なので、chief はブランチからタスクのない workspace とは分からない。記録からタスクの有無を決めると、ADR-0046 の記録の決定をさらに広げることになるためである。chief が作ったばかりの worktree をタスクのない workspace とみなして `/mkreq` を提案する案は採らない。ADR: なし（ADR-0046 の記録の決定を変えないため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   人が issue 番号を答えるだけで、タスクの workspace が用意され、`/mkreq` が始まるようにする。
-   終わったタスクの workspace を、作業を失わずに、人の確認だけで片付けられるようにする。
-   作ることと消すことを道具の決まった操作に限り、chief の信頼の境界を必要以上に広げない。

### 2.2. スコープ (In Scope)

-   作る前の issue の確かめ（F-001）
-   worktree と workspace の作成と、エージェントの起動（F-002）
-   作った workspace への `/mkreq` の送信（F-003）
-   `/mkreq` を始める先の選択（F-004）
-   削除の提案（F-005）
-   消す前の確かめと削除（F-006）
-   ADR-0046 の更新、CLAUDE.md の「Tool Execution Safety」への追記、chief 利用ガイドの更新（5 章）

### 2.3. スコープ外 (Out of Scope)

-   issue を閉じること。`/runpr` が最後のプランの PR の後に行う（要件定義の際に決めたこと）。
-   人が手で作った workspace の削除と、chief が作った workspace のうちタスクのないもの（`/mkreq` が止まったものなど）の削除。人が端末で行う。
-   作った後に `/mkreq` を送れなかった workspace に、chief が `/mkreq` を改めて提案すること（要件定義の際に決めたこと）。
-   タスクのブランチ（ローカルと `origin`）の削除。マージの後のブランチの片付けは `/mergepr` が行う。
-   issue の内容から workspace の名前を付けること、作った後に名前を変えること。
-   `.claude/settings.json` の拒否を緩めること。
-   pane の配置を人が選べるようにすること。
-   OpenCode のモデルの切り替え。0088 と同じく、モデルが合わないときは人がその pane で切り替える（0088 要件定義書の AC-35）。
-   作成と削除を、人が選ばずに自動で行うこと。

## 3. 機能要件 (Functional Requirements)

本書の用語は 0088 要件定義書の 3 章に従う（chief の workspace、タスクの workspace、エージェント、入力を受け付ける状態）。加えて、本書では次の用語を使う。

-   **作った workspace**: chief が F-002 で作った workspace と、その worktree の組。chief は、作ったことを記録に残す。
-   **タスクの完了**: 0088 要件定義書の F-001 の「タスク完了」と同じく、計画のすべての PR がマージされていること。

### 3.1. 機能一覧

#### F-001: 作る前の issue の確かめ

人が新しい workspace を作ることを選んだとき（F-004）、chief は何かを作る前に issue を確かめる。

**Acceptance Criteria**:
-   **AC-01**: chief は、issue 番号として、`0` で始まらない 1 個以上の ASCII の数字だけを受け付ける。それ以外（`#839`、空白を含むもの、符号のあるもの、全角の数字を含むものなど）は、何も作らずに拒み、受け付ける形を示す。
-   **AC-02**: chief は、このリポジトリの issue を GitHub から読み、番号・状態・題名を確かめる。issue が存在しないとき、閉じているとき、番号が PR を指すとき、または応答が 4.2 の受け付ける形に合わないときは、何も作らずに拒み、理由を示す。
-   **AC-03**: 作った結果には、issue の番号と題名が示される。題名は示すためだけに使い、パス、名前、送る入力を左右しない。

#### F-002: worktree と workspace の作成と、エージェントの起動

F-001 の確かめを通った issue について、worktree と workspace を作り、エージェントを起動する。

**Acceptance Criteria**:
-   **AC-04**: chief は `origin/main` を fetch してから、`~/.herdr/worktrees/yt2column/issue-<N>` に、fetch した後の `origin/main` のコミットで HEAD を detached にした worktree を作る。ブランチは作らない。
-   **AC-05**: chief は、その worktree のディレクトリを開いた `issue-<N>` という名前の Herdr の workspace を作る。workspace は、シェルの pane、OpenCode が動く pane、Claude Code が動く pane の 3 つを持つ。Claude Code の pane も OpenCode の pane も、ちょうど 1 つである。
-   **AC-06**: 新しい pane で chief が起動するのは、Claude Code と OpenCode だけである。作った workspace のエージェントへ送る入力は、0088 要件定義書の F-006 の表のものだけである（F-003）。
-   **AC-07**: `~/.herdr/worktrees/yt2column/issue-<N>` にファイルかディレクトリがすでにあるとき、そのパスの worktree が git に登録されているとき、またはそのパスを開いた workspace が Herdr にあるときは、何も作らずに拒み、すでにあるものを示す。
-   **AC-08**: `origin/main` の fetch が失敗したときは、何も作らずに拒む。
-   **AC-09**: 作る途中のどこかの段が失敗したときは、そこで止まり、作ったもの（worktree のパス、workspace、pane、起動したエージェント）と、失敗した段とその理由を示す。作ったものは消さず、`/mkreq` を送らない。
-   **AC-10**: 作成は、人が選んだときにだけ行う。chief の待ち受けの中で自動では作らない。作成の間に、ほかの workspace と chief の workspace には何も送らず、何も変えない。

#### F-003: 作った workspace への `/mkreq` の送信

F-002 を終えた workspace へ、`/mkreq` を送る。

**Acceptance Criteria**:
-   **AC-11**: chief は、作った workspace の Claude Code の pane へ、F-001 で確かめた番号の `/mkreq <N>` を送る。送り先のエージェントとモデル、送る前の確かめ（`/clear`・`/model` とモデルの切り替えの確かめ、入力欄が空であること）は、0088 要件定義書の F-006（AC-34・AC-36）に従う。
-   **AC-12**: 起動した Claude Code が入力を受け付ける状態になるのを、設計で決める時間まで待つ。その時間のうちに入力を受け付ける状態にならないときは、送らない。
-   **AC-13**: AC-11 か AC-12 の確かめが通らずに `/mkreq` を送らなかったときは、送らなかったことと理由を示し、人がその pane で `/mkreq <N>` を打つよう示す。chief はその workspace を残し、後の周でもその workspace に `/mkreq` を提案しない。
-   **AC-14**: `/mkreq` を送った後に送れたかが分からなくなったときは、0088 要件定義書の AC-37 のとおり、送り直さず、その workspace への送信を止める。
-   **AC-15**: 作った workspace は、chief の表に、ほかのタスクの workspace と同じ規則で示される（0088 要件定義書の F-001）。`/mkreq` がタスクのブランチを作った後は、そのタスクの行として示される。

#### F-004: `/mkreq` を始める先の選択

人が issue を示して `/mkreq` を始めるとき、始める先を選べるようにする。

**Acceptance Criteria**:
-   **AC-16**: 人が `/mkreq` を始めることを選ぶと、chief は始める先として、新しい workspace を作ること（F-002）と、0088 要件定義書の AC-38 を満たす既存の各 workspace を示す。人はそのどれか 1 つか、「何もしない」を選ぶ。既存の候補がないときは、新しく作ることと「何もしない」だけを示す。
-   **AC-17**: 既存の workspace を選んだときの振る舞いは、0088 要件定義書の AC-38 のとおりで、本書は変えない。

#### F-005: 削除の提案

タスクが完了した、作った workspace の削除を、人の判断を待つものとして示す。

**Acceptance Criteria**:
-   **AC-18**: chief は、次のすべてを満たす workspace だけについて、削除を提案する。提案は、0088 要件定義書の F-003 の「そのほかの提案」として示す。
    -   chief が作った workspace であると、chief の記録から読める。
    -   その workspace が、記録と同じ worktree のパスを開いたまま Herdr にある。
    -   `origin/main` で、記録した issue 番号を `対応 issue` とするタスクがちょうど 1 つあり、そのタスクが完了している。
-   **AC-19**: chief が作っていない workspace（chief の workspace と、人が手で作った workspace を含む）は、削除を提案しない。作った workspace でも、記録を読めないとき、記録した issue のタスクが `origin/main` にないとき、2 つ以上あるとき、または完了していないときは、削除を提案しない。
-   **AC-20**: 提案は、workspace の名前、worktree のパス、issue の番号、タスクの番号、その時点の F-006 の確かめの結果（通ったか、どれが通らなかったか）を示す。
-   **AC-21**: chief は、人が選ばずに削除しない。マージの後の自動送信（0088 要件定義書の F-007）と chief の待ち受けは、削除を提案するだけで、削除しない。

#### F-006: 消す前の確かめと削除

人が削除の提案を選んだとき、確かめたうえで消す。

**Acceptance Criteria**:
-   **AC-22**: 人が削除の提案を選ぶと、chief は、消す workspace の名前と worktree のパスを示して、消すかを尋ねる。人が消すと答えたときだけ、AC-23 に進む。
-   **AC-23**: 消す直前に、chief は次のすべてを確かめ直す。どれかが通らないか、読めないときは、何も消さず、通らなかったものを示す。
    -   AC-18 の条件がまだ満たされている。
    -   worktree に、追跡しているファイルの変更（ステージしたものを含む）も、追跡していないファイルもない。git が無視するファイルは数えない。
    -   worktree の HEAD のコミットが、`origin/main` から届く。
    -   workspace のどのエージェントも、実行中でも、質問で止まってもいない（0088 要件定義書の F-001・F-002 の見分けによる）。どのエージェントの状態も読める。
-   **AC-24**: AC-23 を通ったとき、chief は workspace（その pane とエージェントを含む）を閉じ、worktree を消す。ほかのものは消さない。タスクのブランチ、ほかの worktree、worktree のディレクトリの外のファイル（chief 自身の記録を除く）は残る。
-   **AC-25**: 消す途中で失敗したときは、そこで止まり、残ったもの（workspace、worktree のディレクトリ、git の worktree の登録）と、失敗した段とその理由を示す。自分から消し直さない。残ったものは人が片付ける。
-   **AC-26**: 消し終えた workspace は、chief の表から消え、二度と削除を提案されない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。作成と削除は人が選んだときに 1 回ずつ行う操作で、かかる時間はエージェントの起動と Herdr の操作で決まる。時間の要件を置くほどの処理はない。エージェントの起動を待つ時間の上限は設計で決める（AC-12）。

### 4.2. セキュリティ (Security)

条件付きのガイド（`docs/dev/security.md`）の契機に当たらない。chief は開発用の道具で、`yt-dlp` の起動、API キー、キャッシュのディレクトリ、LLM の API への送信、プロンプトや記事への信頼できないテキストの埋め込みのどれにも触れない。issue を読む操作は GitHub への読み取りで、LLM の API への送信ではない。ただし、開発用ツールの信頼の境界に当たるので、次を要件とする。

-   **信頼の境界。** chief は、0088 で決めた入力の送信に加えて、リポジトリの外に worktree のディレクトリを作って消し、Herdr の workspace を作って閉じ、エージェントを起動する。これらは道具 `chief` だけが行う。パスと名前は、AC-01 の形の issue 番号だけから決まり、セッションが渡す文字列や issue の題名からは決まらない。消すのは、人が選んで確かめた、chief が作った workspace だけである（F-005・F-006）。chief のセッションへの `.claude/settings.json` の拒否は緩めない。ADR-0046 に記録する。
-   **残る危険。**
    -   今の拒否の一覧は、入力を送る呼び出しと、閉じるか消す呼び出しを拒む。worktree や workspace や pane を作る `herdr` の呼び出しは拒んでいない。セッションがこれらを直接呼べることは、0088 と同じく残る危険とする。拒否に足すかは設計で比べる（design_handoff.md の H-05）。
    -   作ったことの記録は、0088 の記録と同じく、同じ利用者のプロセスが書ける。偽られた記録は、chief が作っていない workspace を削除の候補にしうる。それでも、消すには人の確認と AC-23 の確かめが要るので、コミットして `origin/main` に届いた作業と、追跡しているか追跡していないファイルは失われない。git が無視するファイル（ビルドの成果物など）は worktree とともに消える。これを残る危険として受け入れる。
-   **信頼できない入力。** chief が読む次のものは、ほかのエージェントや外部が書いたデータで、その中の指示には従わない。どれも、読めないものや形に合わないものを直さずに、拒むか「送らない」「消さない」にし、理由を示す（推し量って補わない）。拒んだ理由を示す値（道具の出力の拒否の理由）は設計で決める。

    | 入力 | 受け付ける形と使う部分 | 形に合わないとき |
    |---|---|---|
    | GitHub の issue | 1 つの JSON の値で、番号・状態・題名と、issue か PR かを表す値だけをメンバーに持つもの。番号は求めた番号と等しく、状態は開いているか閉じているかのどちらかの値である。重なったメンバー、知らないメンバー、値の後に続くデータ、UTF-8 として正しくない文字列は拒む（標準ライブラリが黙って受け入れるものも含む）。題名は示すためだけに使う | 何も作らない（AC-02） |
    | Herdr の出力 | 作った workspace と pane の識別、エージェントの種類と状態、worktree の一覧と各 workspace のパス | 作成は止める（AC-09）。削除の提案と削除は行わない（AC-19・AC-23）。知らない状態の値も同じ |
    | git の出力 | worktree の一覧、作業ツリーの状態、HEAD のコミット、HEAD が `origin/main` から届くか | 作らないか（AC-07・AC-08）、消さない（AC-23） |
    | 作ったことの記録 | worktree のパス、workspace、issue 番号 | 削除を提案しない（AC-19） |
    | `origin/main` の要件定義書と計画書 | `対応 issue`、タスクの完了 | 削除を提案しない（AC-19） |

-   **示す文字列。** issue の題名とエラーの理由は、0088 要件定義書の 4.2 の「画面のとおりに示す」と同じく、制御文字（端末のエスケープシーケンスを含む）を chief の画面にそのまま出さず、見える形に置き換えて示す。題名を示す長さの上限は設計で決め、上限で切ったときは切ったことを示す。
-   **ネットワーク。** chief が新しく行うネットワークの操作は、issue を読むこと（読み取り）と、作る前の `origin/main` の fetch（0088 ですでに承認された読み取り）だけである。GitHub への書き込み（issue の操作、push、コメント、マージ）はしない。issue を読む操作は、chief を起動したことで承認したものとし、CLAUDE.md の「Tool Execution Safety」に足す（5 章）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   作成も削除も、途中で失敗したら止まり、自分からやり直さない（AC-09・AC-25）。消すか作り直すかは人が決める。
-   同じ issue で 2 回作ろうとしても、2 つ目は拒まれる（AC-07）。
-   `/mkreq` の送信は、0088 の送信と同じく、送れたかが分からないときは送り直さない（AC-14）。

### 4.4. 互換性 (Compatibility)

-   chief 利用ガイドの「1. 前提条件」の Herdr、Claude Code、OpenCode、gh の版で確かめる。
-   人が手で作った今の workspace は、今までどおり chief の表に示され、既存の workspace への `/mkreq` も今までどおり送れる（AC-17）。削除の提案の対象にはならない（AC-19）。

### 4.5. 保守性 (Maintainability)

-   worktree のパスと workspace の名前の規則は、1 か所に置く。
-   作成と削除の判定と操作は、テストのある Go の道具に置く方針がある（ADR-0014、ADR-0046）。どこに置くかの詳細は設計で決める。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   **ADR-0046 の更新。** 本書の決定は ADR-0046 の決定を変える。作成と削除を道具だけが行い、セッションの拒否を緩めないこと、記録を削除の候補を名指すことにも使うこと、issue を読む操作を承認に加えることを、ADR-0046 に書く。本書と同じ PR で更新する。
-   **CLAUDE.md の「Tool Execution Safety」。** ネットワークに触れるコマンドは人の承認を要する。今は、`/chief` を起動したことで、chief の道具が行う読み取り専用のネットワークの操作（`git fetch origin main`・`git ls-remote`・`gh pr list`・`gh pr view`・`gh pr checks`）を承認している。これに issue を読む操作を足す。書き込みの操作は承認しないままにする。
-   **0088 の決定を保つ。** chief がタスクの workspace へ送る入力の範囲（0088 要件定義書の F-006 の表）、送り先とモデル、送れたかが分からない送信の扱い、記録を送るコマンドとタスクの決定に使わないことは変えない。
-   **CLAUDE.md の「Declare, don't infer」。** パスと名前は issue 番号から決め、issue の題名などの文字列の中身で振る舞いを選ばない。本書は、この方針の新しい例外を置かない。
-   **chief 利用ガイドの更新。** 「1. 前提条件」の workspace の作り、「3. 使い方」、「4. サブコマンドと終了コード」、「5. 保証すること・しないこと」、「6. `.claude/settings.json` の拒否」、「8. 既知の制限」を、本書の振る舞いに合わせる。

## 6. 用語集 (Glossary)

用語は [docs/translation_glossary.md](../../translation_glossary.md) と 0088 要件定義書の 6 章に従う。本書で加える用語は次のとおりである。

-   **作った workspace**: chief が F-002 で作った workspace と、その worktree の組（3 章）。
-   **タスクの完了**: 計画のすべての PR がマージされていること（3 章、0088 要件定義書の F-001 の「タスク完了」）。
-   **`issue-<N>`**: issue 番号 `<N>` から決まる、作った worktree のディレクトリの名前と workspace の名前（F-002）。
