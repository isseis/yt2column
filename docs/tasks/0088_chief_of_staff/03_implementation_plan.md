# 実装計画書：chief が workspace の状況を示し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-12 |
| Review date | 2026-11-12 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `88618e43` で確かめた。

用語（chief・chief の workspace・タスクの workspace・エージェント・入力を受け付ける状態・自動送信・答えの中継・chief の待ち受け・実装計画書の PR・プランの PR・chief のセッション・道具・`/chief` の定義・送り先の表・状態のディレクトリ・周・観測の記録・画面の読み取り・照合値・送信の記録・Claude Code の transcript）は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **入力の型のパッケージ:** pane へ送る入力を表す型だけを持つパッケージ `internal/chief/paneinput`（§1.3 の決定 5）。
-   **採取用の workspace:** 画面のサンプルを採り、chief を確かめるために人が用意する、使い捨ての Herdr の workspace（§1.3 の決定 1）。タスクの作業には使わない。
-   **実地の確かめ:** 本物の Herdr とエージェントで行う手での確かめ（設計書 7.3）。結果を §7 の実行の記録に書く。
-   **呼び出しの記録係:** テストで、道具が走らせる外部のコマンドを、引数・作業ディレクトリ・環境とともに受け取り、決めた応答を返す fake（§1.3 の決定 19）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、Go の道具 `chief`（`cmd/chief`・`internal/chief`）、`/chief` の定義、`_context.md` の送り先の表、PR マーカーの `**自動送信**` の欄を作り、要件書の F-001〜F-011 を満たす。

### 1.2. 実装の原則

-   **読む部品から順に作る。** 画面の読み取り → ほかの読み取り → 状態の区分 → 送信 → 中継 → 待ち受けと自動送信 → 定義と文書の順に PR を分ける（設計書 8 章）。道具は PR-7 で `make install-chief` と `/chief` の定義が入るまで使われないので、途中の PR のマージで入力が送られることはない。
-   **送る入力は入力の型のパッケージの型でだけ作る。** Herdr へ書く部品は、その型だけを引数に取り、文字列を取らない（設計書 3.1.3、§1.3 の決定 5）。
-   **画面の文字列を調べるのは `screen` で始まるファイルだけにする。** ほかのファイルは、そのファイルが返す型だけで振る舞いを選ぶ（設計書 5.2、§1.3 の決定 6）。
-   **外部のコマンドは fake で置き換え、テストを本物の環境から切り離す。** ユニットテストは herdr・git・gh を本物で呼ばず、本物の状態のディレクトリにも触れない（§1.3 の決定 19）。本物の Herdr を使うのは実地の確かめ（PR-1・PR-7）だけである。
-   **時刻と待ちは差し込む。** 道具は時刻と待ちを差し込める形で持ち、記録の時刻は差し込んだ時計から書く。テストは実時間で待たない。
-   **ADR の決定の中のずれの記述は、ずれを解いた PR で直す。** ADR-0042 は PR-2、ADR-0014 は PR-7 で消す。ADR-0046 は、PR-2・PR-6 で残るずれを述べる形に書き換え、PR-7 で消す。直したことは commit のメッセージと PR の説明の訂正の段落に書く（`docs/dev/conventions/adr.md` の「Editorial corrections」）。
-   **各テストは壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-12）

1.  **画面のサンプルは、PR-1 の `/runplan` のセッションが人と一緒に採る。** 人が採取用の workspace を用意し、エージェントが `AskUserQuestion` で状態を作るよう頼み、`herdr agent read` で読んで、採取した人を特定できる情報を置き換え、エージェントの版を記録する。`/runpr` のマージの確認と各コマンドの報告のように、採取用の workspace で作れない画面は、実際のタスクの workspace にその画面が出ているときに読み取りだけで採る。理由: どちらの方法でも状態を作るのに人の手が要る。エージェントが採れば、置き換えと版の記録を毎回同じ手順で行える。ADR: なし（採取の手順はこのタスクの中で閉じるため）。
2.  **PR 番号の検証は、`internal/chief/paneinput` が `fixprguard.ParsePRNumber`（`internal/fixprguard/types.go:132-140`）をそのまま使う。** 理由: 規則を 1 か所に保ったまま、信頼の境界の道具である `fixprguard` を変えずに済む（変えると再インストールが要る）。ゼロ値は `fixprguard.PRNumber{}` との比較で見分ける。ADR: なし。
3.  **PR を 7 つに分け、設計書 8 章の 1〜7 に 1 つずつ対応させる。** 送信・中継・自動送信は、それぞれ信頼の境界なので、別の PR に分けてレビューの焦点を保つ。8 章（「目安の順序」）から次を動かす。どれも、その PR が自分のテストとガードを通すための前提である。
    -   画面の読み取りと文字の無害化（8 章の 2）を PR-1 に入れる。採ったサンプルを確かめるテストが要るため。
    -   CLAUDE.md の「Declare, don't infer」の例外と「Go Idioms」の日本語の literal の例外（8 章の 7）を、それが要るコードを入れる PR-1（画面）と PR-2（計画書）に入れる（決定 17）。
    -   `**自動送信**` の欄の読み取り（8 章の 6）を、同じ場所の規則で読む PR マーカーの読み取りとあわせて PR-2 に入れる。文書に欄を足すのは PR-6 のままにする。
    -   通知の記録（8 章の 6）を、並べ方（待っている時間）の前提として PR-3 に入れる。
    -   Herdr へ書く部品（8 章の 2）を、それを使う送信の PR-4 に入れる。
    -   `package_reference.md` の行（8 章の 7）を、パッケージを作る PR に入れる（`TestPackageReferenceListsPackages` が求める）。
    -   ADR-0042 のずれの記述（8 章の 7）を、ずれを解く送り先の表の PR-2 で消す（設計書 2.2 の表の最後の行のとおり）。
    -   CI の `doc-guard` に道具のパッケージを足すこと（決定 18）を、`./internal/chief` は PR-2 に、`./cmd/chief` はそのパッケージを作る PR-3 に入れる（ステップ 2-8・3-6）。

    ADR: なし。
4.  **待ち受けと自動送信の時間の値は次のとおりにする。** `wait` は 3 秒ごとに `herdr agent list` を読み、既定の時間切れを 10 分にする。観測の記録を自動送信の条件に数えるのは、その観測を記録した周から今の周までが 15 分より短いときだけにする。持ち越しの観測は、最初に `MERGED` を見てから 10 分まで数える（設計書 3.7）。理由: `/runpr` はマージの後も数分 pane を動かすことがあり、窓に余裕が要る。ADR: なし（定数の値）。
5.  **入力の型をパッケージ `internal/chief/paneinput` に置く。** 設計書 3.1.3 の型（`CommandKind`・`Command`・`RouteKey`・`AgentKind`・`ClaudeModel`・`OpenCodeModel`・`Route`）、`TaskID`・`IssueNumber`、リセットとモデルの切り替えの入力（PR-2）、中継のキー入力（PR-5）を置く。
    -   構造体の型は、unexported の欄とコンストラクタを持つ。`internal/chief` は composite literal でそれを作れない。`Route` の欄も unexported にする（設計書 3.1.3 の素描は exported の欄だが、作る経路を 1 つにするため変える）。
    -   列挙の型（`CommandKind` など）は、ほかのパッケージから型の変換で範囲の外の値を作れる。コンストラクタと、値で振る舞いを選ぶ `switch` は、範囲の外の値を `default` で拒む。
    -   `paneinput` は `internal/fixprguard` を import するので、依存のない葉のパッケージではない。

    理由: 送る入力は型で限られるので、その型を作る経路をコンパイラとコンストラクタに守らせる（`mkplan.md` step 5 の具体型の作り方の確かめ）。ADR: なし（パッケージの境界はこのタスクの中で閉じるため。設計書 3.1.2 が計画に委ねた）。
6.  **`internal/chief` は 1 つのパッケージとし、部品をファイルで分ける。画面と transcript の文字列を調べるのは `screen` で始まるファイルだけにする。**
    -   `screen` で始まるファイルが型に読むもの: 質問の形、マージの確認、`/runpr` の報告の NEXT、PR を作ったコマンドの報告、OpenCode のモデルの行と `/new` の後の画面、入力欄の残りの文字、Claude Code の transcript のモデルを切り替えた記録の行（`screen_transcript.go`）。
    -   ほかのファイルの規則: `detail.go`・`send.go`・`autosend.go` などは、その型だけを使う。文字の無害化は `sanitize.go` に置く。
    -   CLAUDE.md の「Declare, don't infer」の例外は、範囲を `screen` で始まるファイルと、chief のセッションが行う報告の要約に限って書く（設計書 5.2）。
    -   例外に入れないもの: `task.go` と `routing.go` が計画書・状態の表・`_context.md` を読むのは、リポジトリが自分で定めた文書の文法を読むことで、外部の文字列から振る舞いを推し量ることではない（`opus-review-decision.sh` と同じ扱い）。

    ADR: なし。
7.  **外部のコマンドの時間の上限を、名前のある定数で持つ。**
    -   herdr は 10 秒、`git fetch` は 120 秒、そのほかの git と gh は 30 秒とする。
    -   コマンドを送った後に実行中になるのを、`herdr agent get` を読み直して 15 秒まで待つ。`herdr agent prompt` の `--wait` は使わない（Herdr 0.9.1 の help では、`--wait` は送った後の最初に合う状態を待つだけである）。`/clear`・`/model`・`/new` は実行中にならないので、設計書 3.1.3 の「前の入力が終わった」の確かめで待つ。
    -   Claude Code の transcript の切り替えの記録は、500 ミリ秒の間隔で 10 回まで読み直す（設計書 3.1.7）。
    -   PR の詳細の控えは、控えに書いた時刻で 14 日より古いものを `start` が消す（設計書 3.5）。

    理由: どれも、実地の確かめで観測した遅れに余裕を持たせた値で、変えるときは定数 1 か所を直す。ADR: なし。
8.  **`--protocol` の番号は 1 から始める。** `internal/doctestutil/protocol.go:17-22` の道具の一覧に `chief` を足す。今の `fixprguard` のガードは `codexreview` のものでないすべての `--protocol` を数える（`cmd/fixprguard/docs_test.go:380-388`）ので、道具が `fixprguard` の呼び出しと、どの道具の呼び出しにも属さない `--protocol` だけを数えるように直す。ADR: なし。
9.  **`.claude/settings.json` で拒む herdr の呼び出しを、次の 3 つの組とする（2026-11-12、人が選んだ）。**
    -   pane へ入力を送るか、pane でコマンドを走らせるか、pane の入力の経路を変える操作: `herdr agent prompt`・`herdr agent send-keys`・`herdr agent start`・`herdr agent attach`・`herdr pane send-text`・`herdr pane send-keys`・`herdr pane run`・`herdr pane input`。`agent attach` はエージェントの端末に直接つなぐ操作、`pane input` は pane の入力の振り分けを設定する操作で、どちらも pane に入力を届ける経路になりうる。
    -   エージェントの状態とセッションを Herdr に報告する操作: `herdr pane report-agent`・`herdr pane report-agent-session`・`herdr pane release-agent`。chief は、入力を受け付ける状態、ロックの持ち主、`/clear` の確かめをこれらの値で行うので、偽られると確かめが外れる。
    -   サブコマンドの前に、別のセッションか別の機械を指す引数を書く形: `herdr --session`・`herdr --machine`・`herdr --remote`（`--remote-keybindings` を含む）で始まるもの。上の規則の前方一致を外せるため。`herdr --help`・`--version`・`--skill`・`--default-config` は読み取りなので拒まない。

    読み取りの呼び出し（`list`・`get`・`read`・`api`、`--help` など）は拒まない。ほかのセッションが読むために使うことがあり、送ることも偽ることもできないためである。どれも Herdr 0.9.1 の `herdr --help`・`herdr agent --help`・`herdr pane --help` で確かめた。パスで書いた呼び出しなど、規則の外の書き方は残る危険とする（設計書 5.2 の残る危険のまとめ）。設計書 3.9・5.2 は入力を送る呼び出しだけを挙げており、この決定はそれを人の選択で広げる。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（リポジトリのすべてのセッションに効く守りの前提であるため）。
10. **gh は `GH_REPO` と `GH_HOST` を除いた環境で走らせる。git は `cmdrunner.GitHardening()` の引数と `cmdrunner.GitEnv` の環境で走らせる。**
    -   gh が環境変数の指す別のリポジトリの PR を読み、自動送信の引き金にしてしまわないよう、PR をタスクの worktree の remote から引かせるためである。`fixprguard` がこの 2 つを拒むのと同じ考え方である（`internal/config/envaccess_test.go:70-72` の注記）。
    -   `GitHardening()`（`internal/cmdrunner/git.go:19`）は、ほかのエージェントが書く worktree の設定から、fsmonitor のフックなどが走らないようにする。

    終了コードの契約（設計書 3.1.1）は変えない。ADR: なし。
11. **workspace の名前は `herdr workspace list` の `label` から取る。** Herdr 0.9.1 の `herdr agent list` の出力は `label` を持たず、`herdr workspace list` が持つ（2026-11-12 に両方の出力で確かめた）。ADR: なし。
12. **gh の配列の出力のために `strictjson.ParseArray` を足す。** `gh pr list --json` と `gh pr checks --json` は配列を返す見込みで、今の `strictjson` はオブジェクトだけを読む（`internal/strictjson/strictjson.go:70-87`）。配列であることは PR-1 の採取で gh の実物の出力で確かめ、違えば PR-2 の前に計画を直す。ADR: なし（ADR-0020 の部品の中の追加である）。
13. **状態のディレクトリでは、workspace ごとの記録と排他を、worktree の正規のパスの SHA-256 の 16 進を名前にした下のディレクトリに置く。**
    -   正規のパスは、`git rev-parse --show-toplevel` の出力に `filepath.EvalSymlinks` をかけたものとする。Herdr の `cwd` と、セッションが渡す `--workspace` は、この形に直してから今のタスクの workspace と照らす。同じ worktree をシンボリックリンク（macOS の `/private` を含む）で指しても、送信の停止の記録を見落とさないためである。
    -   `internal/cachelock` はディレクトリごとに決まった名前のファイルをロックする（`internal/cachelock/cachelock.go:21,73`）ので、workspace ごとにディレクトリを分ける。
    -   ロックを引き継ぐときの排他は、状態のディレクトリの直下で `cachelock.Acquire` を取る。
    -   ディレクトリは先に `privatefile.MkdirBelow` で作り、`cachelock.Acquire` の `MkdirAll` が作る経路を使わない。

    ADR: なし。
14. **セッションの振る舞いだけで実現する部分は、`/chief` の定義を読む `manual` と実地の確かめで確かめる。**
    -   対象は、AC-05 の表の下への表示、AC-13 の「何もしない」、AC-14、AC-16、AC-22 の問い、AC-24 のバックグラウンド、AC-58 の通知の送信、AC-60 の `wait` の失敗の扱いである。これらは `/chief` の定義の手順の文だけで実現する。
    -   エージェントが文に従うかはテストで確かめられず、文の語を探すテストは置かない（ADR-0016、0081 の計画書 §1.2 と同じ扱い）。
    -   道具が出す材料（並べた一覧、質問の文、送り先、排他など）は `test` で確かめる。
    -   AC-14 と AC-16 は、道具の側に確かめる材料がなく、`manual` だけで確かめる。これは、`test`・`static`・`diff` のどれかを求める `docs/dev/conventions/requirements_process.md` §4 の規則の例外とする。理由は上の 2 つめの箇条のとおりで、この 2 つの AC は `/chief` の定義の文と、セッションがそれに従うことだけで実現するためである。範囲はこの 2 つの AC に限り、ほかの AC は道具の側の `test` を持つ。

    ADR: なし。
15. **`mkreq.md` は変えない。** 確かめた結果、`mkreq.md` は「PR 構成」の節の行を書かず、`_lib/simple-process.md:162-165` と `mkplan.md:175,205-209` がそれを書く（設計書 2.2 が確かめを計画に委ねた）。ADR: なし。
16. **PR-7 は、試作をリポジトリの外へ移してから `/chief` の定義を書く。** 試作の `.claude/commands/chief.md` と `.claude/chief/` はこの worktree にあり、git の共通ディレクトリの `info/exclude` で除いている（2026-11-12 に確かめた）。除いたままだと、同じパスに書く新しい定義を `git add` が拾わない。ADR: なし。
17. **CLAUDE.md の「Go Idioms」に、`internal/chief` の日本語の literal の例外を足す。テストの日本語のサンプルは `testdata/` のファイルに置く。**
    -   計画書の見出し・欄の名前・値の行の先頭（`作成ポイント`、`**実装モデル要件**: `、`**自動送信**: `、`PR 構成`、`PR を作成した` など）と、状態の表の項目は日本語の文書の文字列と等しい必要がある。エージェントが日本語で出す画面の文字列もある。
    -   範囲は、`internal/chief` の `screen` で始まるファイルと、計画書と状態の表を読むファイル（`task.go`）の、文書か画面の文字列と等しくなければならない literal に限る。コメント・識別子・道具が出す文は英語のままとする。
    -   例外は、それが要る literal を入れる PR（画面は PR-1、計画書は PR-2）で足す。テストが読む計画書と画面のサンプルは `internal/chief/testdata/` のファイルに置き、Go の literal にしない。

    理由: 今の例外が認めるのは、ガードのテスト、`doctestutil`、`_checks/`、`opus-review-decision.sh` の literal だけである（CLAUDE.md の「Go Idioms」）。ADR: なし（CLAUDE.md の規則は ADR にしない）。
18. **CI の `doc-guard` のジョブに `./cmd/chief` と `./internal/chief` を足す。** `check-changes` は `.claude/` と `*.md` だけの変更を code でないとし、`doc-guard` だけが走る（`.github/workflows/ci.yml:191`）。足さないと、`_context.md` の送り先の表、`mkplan2.md` の PR マーカーの形、`.claude/settings.json`、`/chief` の定義だけを変える PR で、それを守るテストが CI で走らない。CLAUDE.md の「Development Notes」の `doc-guard` の一覧と、PATH の bash が 4 以上であるという `make test` の前提（ステップ 2-5）も、同じ PR で書く。ADR: なし。
19. **テストを本物の環境から切り離す。**
    -   `cmd/chief` の `main` は、引数、環境変数を読む関数、外部のコマンドの runner を作る関数、時計を渡して処理を呼ぶだけにする。テストは、状態のディレクトリを `t.TempDir()` の下にした環境の表と、呼び出しの記録係を渡す。
    -   `internal/chief` が runner を作る経路を 1 つにし、テストはそれを呼び出しの記録係に置き換える。記録係は、すべてのテストで、呼ばれた gh と git が読み取りの操作の一覧にあることを確かめ、作業ディレクトリと環境を記録する。
    -   テストの補助は、念のため `PATH` を、`herdr` と `gh` が見つからない空のディレクトリにする。
    -   一時的な git のリポジトリと bash のスクリプトは、`internal/codexreview/testutil` の `NewRepo` と同じく、許した変数だけの環境、一時的な `HOME`、`GIT_CONFIG_NOSYSTEM=1`、署名を切った設定、決めた作者で走らせる。

    理由: `make test` と pre-commit のテストは Herdr の pane の中で走り、環境に `HERDR_PANE_ID` があり、本物の `herdr` と、動いている chief のロックもありうる。ADR: なし。
20. **`git fetch origin main` は、workspace ごとではなく周ごとに 1 回、chief の worktree で行う。** `refs/remotes/origin/main` はすべての worktree で共有され、マージの直後には `/runpr` と `/mergepr` も同じ参照を fetch する。同時に更新すると、どちらかが参照のロックで失敗しうる。fetch を周に 1 回にして重なりを小さくする。fetch を省くことはしない（設計書 3.7 の条件「この周の `git fetch origin main` が成功し」を変えないため）。道具の側の fetch が失敗したときは、設計書 3.7 のとおり自動送信の条件を満たさないものとし、次の周で確かめ直す。ADR: なし。
21. **実地の確かめでの自動送信は、人が通常の `/runpr` でマージする実際のタスクの PR で確かめる。** 確かめのためだけの PR をマージしない。自動送信で送られるのは、そのタスクの本当の次のコマンドで、その後の作業はふだんどおり進める。`make install-chief` はエージェントが行わず、人がこのブランチから行い、マージの後に `main` から入れ直す（ADR-0014 の「道具は `main` からインストールして使う」）。ADR: なし。
22. **画面のサンプルを pre-commit の空白のフックと git の改行の変換から外す。** `.pre-commit-config.yaml` の trailing-whitespace と end-of-file-fixer の除外（51・53 行）と `.gitattributes` の `-text` に `internal/chief/testdata/screens/` を足す。端末の画面の行末の空白と、入力欄の形が書き換えられないためである。ADR: なし。

本計画で主要な決定（CLAUDE.md の「Design decisions (ADR)」の基準）は決定 9 だけで、ADR-0046 を本 PR で更新する。ほかの決定はこのタスクの中で閉じる。

### 1.4. 既存コード調査結果

| 対象 | 今あるもの | このタスクで行うこと |
|---|---|---|
| `internal/cmdrunner` | `Runner`・`InputRunner`・`Options`（`Dir`・`Env`・`MaxOutput` など、`runner.go:25-60`）、`GitHardening`（`git.go:19`）、`GitEnv`（`git.go:36`）、`testutil.Fake`（`testutil/mocks.go:34-90`。決まった順の呼び出しを確かめ、`Options` は見ない） | runner と git の補助はそのまま使う。作業ディレクトリと環境を確かめるため、テストは呼び出しの記録係を使う（決定 19） |
| `internal/strictjson` | `ParseObject` と値の読み取り（`strictjson.go:72-330`）。最上位の配列は読めない | `ParseArray` を足す（決定 12） |
| `internal/privatefile` | `Read`・`Create`（`link(2)` で置き、すでにあれば作らない）・`MkdirBelow`（`privatefile.go:226,314,402`）、持ち主と権限の確かめ `check`（181 行、`privatefile_test.go` が表で確かめている）。消す操作はない | `Remove` を足す（設計書 3.5）。持ち主の確かめは `check` を使う |
| `internal/cachelock` | `Acquire(dir)` が `dir` の `.yt2column.lock` を待たずに flock する（`cachelock.go:21,73-100`）。`Lock.Close`（119 行） | workspace ごとと、ロックの引き継ぎの排他に使う（決定 13） |
| `internal/fixprguard` | `PRNumber`・`ParsePRNumber`（`types.go:126-140`）。0 と先頭の 0 を拒む（`types.go:158-170`） | `paneinput` から使う（決定 2）。変えない |
| `internal/doctestutil` | `ProtocolFlags`・`ToolFixprguard`・`ToolCodexreview`（`protocol.go:17-80`） | `ToolChief` を足す（決定 8） |
| `cmd/fixprguard/docs_test.go` | `fixprguardProtocolMismatches`（380-388 行）が、`codexreview` のものでないすべての `--protocol` を数える | `fixprguard` か道具のないものだけを数えるよう直す（決定 8） |
| `cmd/codexreview/docs_test.go` | `TestSettingsAllowsTool`（80 行）が、`launchFree`（50 行、`coderabbit`・`codexreview`）にない語の `allow` を拒む。`TestDocumentedProtocolMatches`（158 行）、`TestDocumentedInvocationsParse`（255 行）、`TestNoDirectCodexInvocation`（554 行）、`TestInstallTargetInMakefile`（655 行） | `chief` が codex を走らせないことを確かめて `launchFree` に足す。`cmd/chief/docs_test.go` で同じ形のガードを作る |
| `internal/codexreview/testutil` | `NewRepo`：切り離した環境と署名を切った一時的な git のリポジトリ | 一時的なリポジトリの作り方の手本にする（決定 19） |
| `cmd/yt2column/test_helpers_bash.go` | `scriptBashes`（コマンドの文書の bash のスクリプトを走らせる） | PR マーカーの読み方の比較のテストで、bash を探す考え方を参考にする。`opus-review-decision.sh` は bash 4 以上を要する（`.claude/commands/_lib/opus-review-decision.sh:49`）ので、PATH の bash だけを使い、4 より古ければ失敗にする |
| `internal/config/envaccess_test.go` | 環境を読んでよい場所の一覧 `allowedEnvRefs`（65-80 行） | `cmd/chief/main.go` の行を足す（PR-3） |
| `internal/pipeline/pipeline_test.go`・`strictjson_users_test.go` | `TestPackageReferenceListsPackages`（629 行）が、非テストのソースを持つパッケージすべての行を求める。`TestPackageReferenceStrictjsonUsers`（657 行）が `strictjson` の利用者の一覧を求める | パッケージを作る PR で `package_reference.md` に行を足す（`internal/chief` は PR-1、`internal/chief/paneinput` は PR-2、`cmd/chief` は PR-3）。PR-2 で `strictjson` の利用者に `internal/chief` を足す |
| `.github/workflows/ci.yml` | `doc-guard` が決まったパッケージだけを走らせる（191 行） | `./cmd/chief`・`./internal/chief` を足す（決定 18） |
| `.pre-commit-config.yaml`・`.gitattributes` | 空白のフックの除外は `testdata/` と `internal/publisher/testdata/golden/` だけ（51・53 行） | 画面のサンプルを足す（決定 22） |
| `.claude/commands/_context.md` | 「PR marker conventions」の第 1 部の読み手の一覧（64-71 行）、第 3 部の `opus-review-decision.sh` の注記（96-98 行）、第 5 部の表（108-117 行）、欄の一覧（62 行） | 送り先の表の節を足し、第 1 部・第 3 部に chief を足し、第 5 部のモデルの列を置き換える（PR-2）。欄の一覧と読み方に `**自動送信**` を足す（PR-6） |
| `runplan.md`・`runpr.md`・`mkplan2.md` | モデルの案内と値の意味（§3.3 の検索の結果） | 送り先の表を名指す形にする（PR-2） |
| `mkplan2.md`・`mkplan.md`・`_lib/simple-process.md` | PR マーカーの形（`mkplan2.md:59-77`）、値の決め方（`mkplan2.md:41-50`）、PR 境界のチェックリスト（`mkplan2.md:124-136`）、単一フェーズの「PR 構成」の 2 行（`mkplan.md:175,205-209`、`_lib/simple-process.md:162-165`） | `**自動送信**` を足す（PR-6） |
| `.claude/commands/_lib/opus-review-decision.sh` | `origin/main` の `_context.md` の「PR marker conventions」の節の 2 つのセルの行を旧い値の表として読む（134-150 行）。値を HEAD と `origin/main` の両方の計画書から読み、範囲の commit で判断を変える（179-207・225-250・319-330 行） | 変えない。送り先の表は節の外に置き、`**自動送信**` の説明に 2 つのセルの表を置かない |
| `.claude/settings.json` | `permissions.allow` に 2 行だけ（`coderabbit review`・`codexreview`） | `chief` を許し、決定 9 の呼び出しを `permissions.deny` で拒む（PR-7） |
| ADR | ADR-0014 の 19 行、ADR-0042 の 21 行、ADR-0046 の「この決定は、まだ」で始まる段落に、0088 の実装で解くずれの記述がある。ADR-0046 は settings.json の拒否を決定に持たない | §1.2 のとおり、解いた PR で直す。決定 9 を本 PR で ADR-0046 に足す |
| 試作 | この worktree の `.claude/commands/chief.md` と `.claude/chief/`（`hold.tsv` に `0084_wasm_pipeline_poc` の `PR-1` の行）。リポジトリには入っていない | PR-7 で移行する（決定 16） |
| 画面の読み取り、Herdr の読み書き、状態の区分、送信、中継、待ち受け、記録 | ない | 新しく作る。今ある実装で代わりになるものはない |

### 1.5. 実装の際に決めたこと（2026-11-12）

-   **要件書の AC-30・AC-46 を外したので、それを実装するステップと確かめを除く。** タスク 0089（#866）で、`/runpr` のマージの質問から「NEXT をこのセッションで実行する」答えがなくなった（[ADR-0047](../../dev/adr/0047-runpr-merge-question.md)）。ステップ 5-2 の除く選択肢からその答えを、§7 から AC-30・AC-46 の行を除く。理由: 対象のない振る舞いを実装しないため。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) の、この 2 つの AC を外すと書いたずれの文を、本 PR で消す（§1.2 の「ずれの記述は、ずれを解いた PR で直す」のとおり）。
-   **マージの確認のサンプルは、0089 の後の `/runpr` の step 4 の item 3（マージの質問）で採る。item 1 の deferred のスレッドの問いは、ふつうの 1 つを選ぶ問いとして採り、読む（ステップ 1-2・1-6、設計書 1.3）。** 理由: 0089 で step 4 にマージの質問の前の問いが加わり、「マージの確認」の形をどちらの問いで固定するかを決める必要があったため。ADR: なし（画面の扱いはこのタスクの中で閉じるため）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 画面のサンプルと画面の読み取り（設計書 8 章の 1、2 のうち画面の読み取りと文字の無害化）

**変更するファイル:** `internal/chief/testdata/screens/`（新規）、`internal/chief/testdata/README.md`（新規）、`internal/chief/doc.go`・`sanitize.go`・`screen*.go` とそのテスト（新規）、`.pre-commit-config.yaml`、`.gitattributes`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、本計画

-   [ ] **ステップ 1-1**: 採取の準備をする。
    -   `.pre-commit-config.yaml` と `.gitattributes` に、画面のサンプルのディレクトリを足す（決定 22）。
    -   人に採取用の workspace（Claude Code と OpenCode の pane を 1 つずつ）を用意してもらい、Claude Code・OpenCode・Herdr の版を記録する（設計書 3.4.2）。
    -   `herdr agent read` の `--source`（`visible`・`recent` など）と行数のうち、OpenCode のマージの確認の選択肢が見えるものを確かめて決める（設計書 1.2 の観測）。
-   [ ] **ステップ 1-2**: 画面と出力を採る（決定 1）。
    -   設計書 3.4.1 の表のすべての行と、8 章の 1 が挙げる画面を、エージェントごとに採る。マージの確認は 0089 の後の `/runpr` の step 4 の item 3（マージの質問）の画面で採り、item 1 の deferred のスレッドの問いが出ていれば、ふつうの 1 つを選ぶ問いのサンプルとして採る（§1.5）。
    -   `herdr agent list`・`get`・`workspace list` の出力を採る。
    -   Claude Code の transcript の置き場所の規則、モデルを切り替えた記録の行、`/clear` の前後の `agent_session` を確かめる。
    -   gh の `pr list`・`pr view`・`pr checks` の `--json` の出力と終了コードを、通った PR、待っている PR、失敗した PR、CI のない PR で採る。このリポジトリの PR を読む読み取り専用の操作で、人の承認を得てから行う。
-   [ ] **ステップ 1-3**: 採ったものから、採取した人を特定できる情報（利用者の名前、ホームのパス、ホスト名、セッション ID）を置き換える（security.md §7、ADR-0032）。置き換えの規則、版、採った日を `testdata/README.md` に書く。サンプルが置き換えの規則に従っていること（ホームの絶対パスがない、など）と、設計書 3.4.1 の表の各行のサンプルがエージェントごとにあることを確かめるテストを書く。
-   [ ] **ステップ 1-4**: 選択肢を選ぶキー入力をエージェントごとに確かめる（設計書 3.4.2）。採取用の workspace の pane に実際に送り、今の選択の位置に依らずに同じ選択肢が選ばれることを確かめる。確かめられないエージェントは、中継の選択肢を示さないものとして記録する。
-   [ ] **ステップ 1-5**: `sanitize.go` に、文字の無害化と読む量の上限を書く（設計書 3.4.3）。テストは、置き換える文字の種類ごとと上限の境界で、置き換えと切ったことの印を確かめる。
-   [ ] **ステップ 1-6**: `screen*.go` に画面の読み取りを書く（設計書 3.4.1・3.4.2、決定 6）。
    -   返す型の列挙のゼロ値を「合わない」にし、サンプルで確かめた形だけを受け付ける。
    -   質問は、中継するかどうかに依らず、問いごとの文と選択肢と、中継できない理由の種類を返す（AC-05・AC-25・AC-26）。入力欄の残りの文字は文字列で返す（AC-36・AC-54）。
    -   `/runpr` の報告の NEXT は、コマンドの種類とタスクの番号の型で返す（タスクの番号の型は、PR-2 で入力の型のパッケージの `TaskID` に置き換える）。
    -   テストは、サンプルごとの読み取りと、合わない画面（一部だけ合う、2 つの形に合う、上限で切った）が「合わない」になることを確かめる。
-   [ ] **ステップ 1-7**: CLAUDE.md の「Declare, don't infer」の例外（範囲は決定 6、理由と境界は要件書 5 章と設計書 5.2）と、「Go Idioms」の日本語の literal の例外のうち画面の部分（決定 17）を足す。`package_reference.md` に `internal/chief` の行を足す。
-   [ ] **ステップ 1-8**: §7 の「PR-1 の実行の記録」に次を書く。設計書と違う事実が出たら、設計書の誤りとして人に示し、計画と設計書を直してから先へ進む。
    -   版、`herdr agent read` の引数、画面の行数（200 行で足りたか。足りなければ値と理由）
    -   キー入力
    -   transcript の規則と行の形
    -   gh の出力の形と終了コード（配列かどうか。決定 12）

**完了の条件:** 設計書 3.4.1 の表の各行のサンプルがあり、テストが通る。§7 に PR-1 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: screen samples and screen reading

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0088): 画面のサンプルを採り、chief の画面の読み取りと文字の無害化を作る`

**レビュー観点**: サンプルに採取した人を特定できる情報が残っていないか / 合わない画面を「合わない」にするか（広く解釈していないか） / キー入力が選択の位置に依らないか / CLAUDE.md の 2 つの例外の範囲

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-4 が、外部の画面の形とキー入力を本物の Herdr で確かめる実現性の確かめ（Phase 0 型の調査）である。Sonnet で進めると、形の見落としや選択の位置に依るキー入力を確かめたものとして記録しやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 入力の型、Herdr・タスク・送り先の表の読み取り（設計書 8 章の 2）

**変更するファイル:** `internal/chief/paneinput/`（新規）、`internal/strictjson/strictjson.go`、`internal/chief/screen*.go`（タスクの番号の型の置き換え）、`internal/chief/runner.go`・`herdr.go`・`task.go`・`routing.go`・`markers_parity_test.go` とそのテスト（新規）、`internal/chief/test_helpers.go`（新規、`//go:build test`）、`.claude/commands/_context.md`・`runplan.md`・`runpr.md`・`mkplan2.md`、`.github/workflows/ci.yml`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/adr/0042-implementation-model-tiers.md`・`0046-chief-of-staff.md`

-   [ ] **ステップ 2-1**: `internal/chief/paneinput` を作る（決定 5、設計書 3.1.3）。`Command` とそのコマンドの組み立て、リセットとモデルの切り替えの入力を置く。`PRNumber` は `fixprguard.PRNumber` を使う（決定 2）。ステップ 1-6 のタスクの番号の型をこの `TaskID` に置き換える。テストは、各コンストラクタが不正な値を拒むこと、ゼロ値と範囲の外の列挙の値が送れる入力にならないことを確かめる。`package_reference.md` に行を足す。
-   [ ] **ステップ 2-2**: `strictjson.ParseArray` を足す（決定 12）。テストは、配列に特有のこと（最上位が配列でないものを拒む、要素の数の上限）だけを確かめ、`ParseObject` と共有する検査は確かめ直さない。`package_reference.md` の `strictjson` の行に `ParseArray` と、利用者の一覧に `internal/chief` を足す。
-   [ ] **ステップ 2-3**: `runner.go` に、外部のコマンドの runner を作る唯一の経路（決定 7 の時間の上限、決定 10 の git と gh の引数と環境）を書く。`test_helpers.go` に呼び出しの記録係と、一時的な git のリポジトリの補助を書く（決定 19）。`herdr.go` に Herdr の読み取りを書く（設計書 3.1.2）。`herdr agent list`・`get`・`read`・`workspace list` の出力を、`AgentStatus`（ゼロ値は読めない）、エージェントの種類、`agent_session`、作業ディレクトリ、`state_change_seq`、workspace の名前（決定 11）の型に読む。テストは、ステップ 1-2 の実物の出力と、未知の状態の値・欠けた項目・形の崩れた JSON を読めないとすることを確かめる。
-   [ ] **ステップ 2-4**: `task.go` にタスクの読み取りを書く（設計書 3.1.4 のタスクの決め方、`_context.md` の「PR marker conventions」の第 2〜4 部）。
    -   ブランチ名からのタスク、`origin/main` の文書の Status と Process（requirements_process.md の「Cell text」）を読む。
    -   `### PR-N 作成ポイント` と「PR 構成」の節の値（コードブロックの中を読まない）、旧い値の表、`PR を作成した`・`PR がマージされた` の記録、次に作るプランの PR を読む。
    -   `**自動送信**` の欄（設計書 3.8。欄がなければ `auto`、2 つ以上か値が違えば読めない）を読む。
    -   AC-40 のために、ブランチの計画書で、次に作るプランの PR の対象ステップがすべて済んでいるかを読む。
    -   CLAUDE.md の「Go Idioms」の例外に計画書の部分を足す（決定 17）。テストの計画書は `testdata/plans/` に置き、各読み取りの場合と読めない場合を確かめる。`opus` と `sonnet+opus-review` を取り違えないことは、このテストが受け持つ（ステップ 2-5 の比較では区別できないため）。
-   [ ] **ステップ 2-5**: PR マーカーの読み方を `opus-review-decision.sh` と比べるテスト `markers_parity_test.go` を書く（設計書 3.8、7.1）。
    -   一時的な git のリポジトリ（決定 19 の環境）で、`refs/remotes/origin/main` と HEAD に同じ計画書を置き、`origin/main` の `_context.md` はリポジトリの今のファイルとする。HEAD は `origin/main` より 1 つ先で、計画書と `README.en.md` 以外のファイルを変え、実装の記録の行と Opus のレビューの記録の行を持たない。BASE はマージの基点とする。この形でないと、スクリプトは値に依らない判断を返す（`opus-review-decision.sh:70,208-217,320-331`）。
    -   スクリプトが区別するのは、`sonnet`（`NONE value-sonnet`）、値がない（`NONE value-absent`）、それ以外（`opus`・`sonnet+opus-review`・読めない値・置き場所が 2 つ以上、どれも `RUN`）の 3 つである。道具の読み取りがこの 3 つのどれに当たるかを、サンプルごとに比べる。
    -   リポジトリの今の `_context.md` を使うので、送り先の表と `**自動送信**` の説明を旧い値の表として読まないことも確かめられる。
    -   bash は PATH のものを使い、版が 4 より古ければ失敗にする。
-   [ ] **ステップ 2-6**: `_context.md` と案内を送り先の表に合わせる（設計書 3.3）。直す箇所は §3.3 の検索の結果のすべてである。
    -   `_context.md` に「Command routing」の節を「PR marker conventions」の外に足し、設計書 3.3 の表を置く。
    -   第 5 部の表の「Model to start the session with」の列を、送り先の表の行を名指す形にする。第 1 部の読み手の一覧と、第 3 部の `opus-review-decision.sh` の注記に、chief の読み取りを足す。
    -   `runplan.md:20-21,60` と `runpr.md:749,763,1059` のモデルの案内を、送り先の表を読む形にする。
    -   `mkplan2.md:41-44` の値の意味から、どのモデルで実装するかを外し、送り先の表を名指す（値の意味は、実装の重さと Opus のレビューの有無として残す）。42 行の `opus` の説明（「implement the PR on Opus」）は §3.3 の検索の語に合わないが、同じくモデルを述べるので直す。
-   [ ] **ステップ 2-7**: `routing.go` に送り先の表の読み取りを書く（設計書 3.3）。`origin/main` の `_context.md` を `git show` で読み、節・列・行の数と形・モデルの集合を検証して、`RouteKey` から `paneinput.Route` を引く。テストは次を確かめる。
    -   リポジトリの今の `_context.md` が読めること。
    -   形の崩れた表と、集合の外のモデルを拒むこと。
    -   `sonnet+opus-review` の行を OpenCode・`DeepSeek V4.1 Flash` に変えた写しで、その `RouteKey` の送り先だけが変わること（AC-39）。
-   [ ] **ステップ 2-8**: `.github/workflows/ci.yml` の `doc-guard` に `./internal/chief` を足し、CLAUDE.md の「Development Notes」の一覧と、PATH の bash が 4 以上であるという前提を書く（決定 18）。`./cmd/chief` は、パッケージを作るステップ 3-6 で足す。
-   [ ] **ステップ 2-9**: ADR を直す。どちらも commit のメッセージと PR の説明に訂正を書く。
    -   ADR-0042 の 21 行のずれの記述のうち、`_context.md`・`runplan.md`・`runpr.md` に反映していないという文を消す。値の名前の見直し（#760）の文は残す。
    -   ADR-0046 の「この決定は、まだ」で始まるずれの記述を、このマージの後に残るずれ（道具、コマンドの定義、CLAUDE.md の一部、`**自動送信**` の欄、settings.json）を述べる形に書き換える。

**完了の条件:** 設計書 3.1.2 の「Herdr の読み書き」の読み取りの部分、「タスクの読み取り」「送り先の表の読み取り」、3.3 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: input types, readers, and the routing table

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0088): chief の入力の型と読み取りの部品を作り、送り先の表を _context.md に置く`

**レビュー観点**: 入力の型がコンストラクタでしか作れず、範囲の外の値を拒むか / PR マーカーの読み方が opus-review-decision.sh と食い違わないか / 送り先の表が読めないときに送らない側に倒れるか / 案内と mkplan2.md の値の意味が送り先の表と一致するか

**実装モデル要件**: opus

**判定理由**: ステップ 2-1 は、送れる入力を型で限る信頼の境界の核（security-gate）で、2-7 は送るモデルを決める。Sonnet では、型の変換で範囲の外の値を作れる抜けや、表の形の検証の甘さが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 状態の区分、`snapshot`、`start` とロック、`cmd/chief`（設計書 8 章の 3）

**変更するファイル:** `internal/privatefile/privatefile.go`、`internal/chief/records.go`・`workspace.go`・`state.go`・`pending.go`・`network_test.go` など（新規）、`internal/chief/test_helpers.go`、`cmd/chief/main.go`・`main_test.go`（新規）、`internal/config/envaccess_test.go`、`.github/workflows/ci.yml`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 3-1**: `privatefile.Remove` を足す（設計書 3.5 の片付け）。決まった形の通常のファイルだけを消し、シンボリックリンクと持ち主か権限の違うファイルを拒む。持ち主の確かめは今の `check` を使い、テストは `Remove` に特有の拒否（シンボリックリンク、通常のファイルでない、権限）だけを確かめる。`package_reference.md` の行に `Remove` を足す。
-   [ ] **ステップ 3-2**: `records.go` に、状態のディレクトリ（設計書 3.5 の場所、決定 13）、記録の作成（`privatefile.Create`）と読み取り（`privatefile.Read`）、chief のロックと引き継ぎ、`start` していないサブコマンドの拒否（`ErrNotStarted`）、通知の記録を書く。`start` の片付け（設計書 3.5）は、ここでは終わったセッションの通知の記録を消すところまでを書く（消すのはステップ 3-1 の `privatefile.Remove`）。周・観測・送信などの記録と、その片付けは、それを使うフェーズで足す。テストは次を確かめる。
    -   同じ名前の記録を 2 回作れないこと、読めない記録を読めないとすること。
    -   `start` が、終わったセッションの通知の記録を消し、今のセッションの通知の記録を残すこと。
    -   生きている持ち主のロックで `start` が拒むこと、持ち主でない呼び出しを拒むこと。
    -   引き継ぎの確かめ直し。テストが状態のディレクトリの排他を持ったまま、引き継ぐ側が古いと見たロックを生きている持ち主のものに置き換え、その後に排他を放すと、引き継ぐ側が引き継ぎを拒むこと。ディレクトリは並行の処理の前に作っておく（`MkdirBelow` は umask を変えるため）。
    -   同じ worktree をシンボリックリンクで指しても、同じ workspace の記録になること（決定 13）。
-   [ ] **ステップ 3-3**: `workspace.go` に、chief の workspace とタスクの workspace の見分け（設計書 3.1.4、git の共通ディレクトリで比べる）、worktree の git の状態の読み取り、ブランチの開いている PR の読み取り、周ごとの fetch（決定 20）を書く。workspace は決まった順で並べて読む。テストは、見分けの各場合と、git と gh の失敗が「要確認」の理由になることを確かめる。
-   [ ] **ステップ 3-4**: `state.go` に、状態の区分、行の中身と並べ方、次のコマンド、送り先の表が読めないときの扱い、AC-40 の引き継ぎの提案を書く（設計書 3.1.4）。「質問で停止」の行には、質問の文と選択肢と pane を入れる（AC-05）。テストは次を確かめる。
    -   状態ごと、要確認の理由ごと。
    -   次のコマンドの規則の行ごと（簡易な進め方の 3 文書のマージの後を含む）。
    -   実装モデル要件が読めないとき、またはないときに `/runplan` を示さないこと。
    -   AC-40 の条件を 1 つずつ外した場合。
-   [ ] **ステップ 3-5**: `pending.go` に、人の判断を待つものの並べ方と新しい通知を書く（設計書 3.1.5）。テストは、種類の順、待っている時間の順、同じ項目をセッションの中で 2 回通知しないこと、記録を作れないときの時刻の扱いを確かめる。
-   [ ] **ステップ 3-6**: `cmd/chief/main.go` に、サブコマンドの解析、`--protocol`（決定 8）、終了コード（設計書 3.1.1）、出力の JSON（3.1.8）、`start` と `snapshot` を書く。
    -   環境（`HERDR_PANE_ID`・`XDG_STATE_HOME`・`HOME`・`CLAUDE_CONFIG_DIR` と、子プロセスに渡す環境）は `main` でだけ読み、決定 19 の形で処理に渡す。`internal/config/envaccess_test.go` の一覧に足す。
    -   テストは、Herdr の外で起動したとき何も読まずに 3 で終わること、拒む理由ごとの終了コード、`--protocol` の食い違い、使い方の誤りを確かめる。
    -   `package_reference.md` に `cmd/chief` の行を、`ci.yml` の `doc-guard` と CLAUDE.md の一覧に `./cmd/chief` を足す（ステップ 2-8）。
-   [ ] **ステップ 3-7**: `network_test.go` に、呼び出しの記録係が、道具が走らせた gh と git がすべて読み取りの操作の一覧にあり、gh の環境に `GH_REPO`・`GH_HOST` がなく、git がタスクの worktree か chief の worktree で走ったことを確かめるテストを書く（AC-61、決定 10）。`snapshot` が `gh pr checks` を呼ばず、PR の詳細を集めないことも確かめる（AC-04）。

**完了の条件:** `chief start` と `chief snapshot` が fake の外部コマンドで動き、設計書 3.1.4・3.1.5・3.5 のロックと通知の記録がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: state classification, snapshot, and the lock

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `feat(0088): chief の状態の区分と snapshot、start とロックを作る`

**レビュー観点**: 読めないものが「要確認」になり、推し量って別の状態にしないか / ロックの引き継ぎで 2 つの chief が同時に持たないか / 次のコマンドの規則が runpr.md の定義と一致するか / テストが本物の Herdr と状態のディレクトリに触れないか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-2 のロックの引き継ぎは並行して走る 2 つのプロセスの排他で、3-4 は 6 つの状態と次のコマンドの規則を持つ状態機械である。Sonnet では、引き継ぎで確かめ直す順序や、どれにも当たらない場合の扱いを誤りやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 送信、リセット、送信の停止と `release`、PR の詳細と `detail`（設計書 8 章の 4）

**変更するファイル:** `internal/chief/herdr.go`、`internal/chief/send.go`・`transcript.go`・`reset.go`・`detail.go`・`records.go`・`state.go` とそのテスト、`internal/chief/screen*.go`、`internal/chief/network_test.go`、`cmd/chief/main.go`・`main_test.go`

-   [ ] **ステップ 4-1**: `herdr.go` に、`herdr agent prompt`・`send-keys` で書く部品を足す（`--wait` は使わない。決定 7）。引数は入力の型のパッケージの型だけを取り、範囲の外の列挙の値と、chief の workspace の pane を拒む（AC-31）。テストで、どちらも何も送らないことを確かめる。
-   [ ] **ステップ 4-2**: `send.go` に、送り先の pane の決定と、入力ごとの確かめ（設計書 3.1.3 の表）を書く。OpenCode のモデルの行と `/new` の後の画面は `screen*.go` の読み取りの型で、Claude Code のモデルの切り替えは `screen_transcript.go` の型で確かめる（決定 6）。`transcript.go` には transcript の場所の決定と読み直し（決定 7）だけを置く。送らなかったときは、入力欄の残りの文字を結果に入れる（AC-36）。テストは次を確かめる。
    -   送り先の種類の pane が 0 か 2 つ以上のとき送らないこと。
    -   最初の入力の前の確かめのそれぞれ（画面が合わないときを含む。AC-12）で何も送らないこと。
    -   入力の間で確かめが外れたとき残りを送らないこと。
    -   モデルの確かめのそれぞれ。
-   [ ] **ステップ 4-3**: `records.go` に、送信の記録（始めと終わり）、送信の停止、workspace ごとの排他、終わり方の分からない送信の扱い、`release` を足す（設計書 3.5）。`start` の片付けに送信の記録を足す。テストは次を確かめる。
    -   送った後の失敗で送り直さず、停止の記録を作ること。
    -   終わりの記録のない始めの記録が停止になること。始めの記録は記録の部品で直接作るか、途中で止まったものとして扱う道具が持っていた workspace の排他を閉じてから確かめる（同じプロセスの中では flock が残るため）。
    -   道具を作り直しても停止が残ること、`release` が停止を解くこと。
    -   テストが workspace の排他を持っている間は、送信が `NotSentBusy` を返すこと。
-   [ ] **ステップ 4-4**: `detail.go` に PR の詳細を書く（設計書 3.1.6）。PR を作ったコマンドの報告は `screen*.go` の読み取りの型で受け取る。`gh pr checks` が待っているジョブで 8、失敗したジョブか CI のないとき 1 で終わることを、ステップ 1-2 の記録のとおりに扱い、標準出力が厳格に読めれば結果とする。テストは次を確かめる。
    -   CI の結果をまとめずに返すこと（終了コード 8 と 1 の場合を含む）。
    -   レビューの本文を出力に含めないこと。
    -   報告と PR の説明の出どころ、項目ごとの「取得できなかった」、「レビュー省略可」の判定、`/runpr` の送り先の表示。
    -   PR と head ごとの控え（`start` の片付けに 14 日より古い控えを足す）と、pane を空にした後の `detail` が控えた報告を返すこと（AC-55）。
-   [ ] **ステップ 4-5**: `send` の引数の検証とコマンドの組み立て（設計書 3.1.1）、`/mkreq` の条件（3.1.4）、`runplan-takeover`、`/runpr` の後のリセット（3.1.3、`reset.go`）を書き、`cmd/chief` に `send`・`detail`・`release` を足す。テストは次を確かめる。
    -   引数の組み合わせの誤り、開いていない PR の番号、今のタスクの workspace に当たらない `--workspace`。
    -   タスクのある workspace への `/mkreq`。
    -   リセットの候補の条件のそれぞれ（同じ pane、報告がない、`state_change_seq` が変わった、入力欄に文字がある）と、リセットの前に控えを作ること。
-   [ ] **ステップ 4-6**: 行の「実行中」の表示を、送信の記録から示す形にする（設計書 3.1.4 の行の中身）。`network_test.go` に `detail` と `send` の経路を足す。
-   [ ] **ステップ 4-7**: `ActionResult` の結果（送らなかった・送った後に失敗した・送った）を、送信とリセットの各経路でテストする（AC-62・AC-63）。

**完了の条件:** 設計書 3.1.3・3.1.6・3.1.7 と 3.5 の送信の記録・送信の停止・workspace ごとの排他がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: sending, reset, stop, and detail

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0088): chief の送信とリセット、送信の停止、PR の詳細を作る`

**レビュー観点**: 送る入力が型の外から作れないか / 送った後の失敗と終わり方の分からない送信で送り直さないか / リセットの候補を画面で広げていないか / レビューの本文と制御文字が出力に出ないか

**実装モデル要件**: opus

**判定理由**: ステップ 4-1〜4-5 は、ほかのエージェントのセッションへ入力を送る信頼の境界（security-gate）である。Sonnet では、入力の間の確かめの抜けや、送った後の失敗を「送らなかった」と扱う誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 答えの中継と `relay`（設計書 8 章の 5）

**変更するファイル:** `internal/chief/relay.go` とそのテスト、`internal/chief/state.go`、`internal/chief/paneinput/`、`cmd/chief/main.go`・`main_test.go`

-   [ ] **ステップ 5-1**: 照合値を作る部品を書く（設計書 3.6）。`snapshot` の出力の中継できる質問に照合値を足し、中継しない質問には照合値を返さない（質問の文と選択肢は PR-3 から返している）。テストは、照合値が `state_change_seq`・セッション・問いの文・選択肢・選択の位置のどれが変わっても変わること、中継しない質問の種類ごとに照合値がないことを確かめる。
-   [ ] **ステップ 5-2**: 入力の型のパッケージに中継のキー入力の型を足し、選択肢の番号とステップ 1-4 で確かめたキー入力からだけ作る。`relay.go` に中継を書く（設計書 3.6）。排他、読み直し、照合値の比較、Claude Code の `blocked` の確かめ、すべての問いの答え、選択肢の範囲、除く選択肢（自分で書く選択肢）、送信の記録、送った後の失敗での停止を持つ。
-   [ ] **ステップ 5-3**: `cmd/chief` に `relay` を足す。テストは次を確かめる。
    -   照合値が違うとき、一部の問いだけに答えたとき、範囲の外の番号、除いた選択肢を選んだときに、何も送らないこと。
    -   キー入力を送り始めた後の失敗で、停止の記録を作ること。
    -   送った結果を返すこと。

**完了の条件:** 設計書 3.6 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-5 作成ポイント: answer relay

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `feat(0088): chief の答えの中継を作る`

**レビュー観点**: 照合値が質問の変化をすべて拾うか / 人が選んでいない答えや選択肢の文を送らないか / 中継しない質問と除く選択肢が漏れていないか

**実装モデル要件**: opus

**判定理由**: ステップ 5-2 は、止まっているエージェントに人の答えを送る信頼の境界（security-gate）である。Sonnet では、照合値に入れる値の漏れや、選択の位置に依るキー入力を送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 待ち受けと自動送信、PR マーカーの `**自動送信**` の欄（設計書 8 章の 6）

**変更するファイル:** `internal/chief/wait.go`・`tick.go`・`autosend.go`・`records.go` とそのテスト、`internal/chief/network_test.go`、`cmd/chief/main.go`・`main_test.go`・`docs_test.go`（新規）、`.claude/commands/_context.md`・`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md`、`docs/dev/adr/0046-chief-of-staff.md`

-   [ ] **ステップ 6-1**: `wait.go` に `wait` を書く（設計書 3.1.1、決定 4）。基準は不透明な値とし、タスクの workspace のエージェントの状態か `state_change_seq` が変わると戻り、chief の workspace の変化では戻らない。テストは次を確かめる。
    -   変化のある場合とない場合、`blocked` のまま変わらないエージェントで戻らないこと。
    -   時間切れ、herdr の失敗で失敗の終了コードになること。
-   [ ] **ステップ 6-2**: `records.go` に、周の記録、観測の記録（持ち越しを含む）、自動送信の記録を足す（設計書 3.5）。`start` の片付けに、終わったセッションの周・観測の記録を足す。
-   [ ] **ステップ 6-3**: `autosend.go` に自動送信を書く（設計書 3.7）。
    -   AC-41 の条件の表、タスクと workspace の確かめ直し、報告の照合（NEXT は `screen*.go` の読み取りの型で受け取る）、ブランチのない状態の規則を持つ。
    -   順序（排他、読み直し、最初の確かめ、自動送信の記録、送信）、満たさないときの提案と持ち越し、自動送信の後のリセットを持つ。
    -   テストは、AC-41 の条件を 1 つずつ外した入力の表で送らないことと、設計書 7.1 の 3 項目めが挙げる場合を確かめる（直前の周より前の観測、タスクの違い、ブランチのない状態で報告の PR の番号が違う、持ち越しの時間切れ、マージの直後の周で実行中だったときに次の周で送れる）。
-   [ ] **ステップ 6-4**: `tick.go` に `tick` を書き、`cmd/chief` に `tick` と `wait` を足す。`network_test.go` に `tick` の経路を足す。
-   [ ] **ステップ 6-5**: `**自動送信**` の欄を文書に足す（設計書 3.8）。直す箇所は次のとおりで、「PR marker conventions」の節に 2 つのセルの表を置かない（`opus-review-decision.sh:134-150` が旧い値の表として読むため）。
    -   `_context.md` の「PR marker conventions」：欄の一覧と、読み方の規則（第 2〜3 部と同じ場所と行の形、欠けたら `auto`、2 つ以上か値が違えば読めない）。
    -   `mkplan2.md`：step 4（`hold` にする場合）、step 6 の形、step 7 の表の例、step 9 のチェックリスト。
    -   `mkplan.md`：step 8.5 の単一フェーズの節の行（行の順と空行の規則を、3 つの行に合わせて直す）と、step 8 のチェックリストの単一フェーズの項目（`mkplan.md:175,205-209`）。
    -   `_lib/simple-process.md`：「Writing the three documents」（162-165 行）と「Reviewing the three documents」。
-   [ ] **ステップ 6-6**: `cmd/chief/docs_test.go` に、次の形を道具の `**自動送信**` の読み取り（ステップ 2-4）で読めることと、`_context.md` の欄の一覧に `**自動送信**` があることを確かめるテストを書く（設計書 7.2 の 2 項目め）。
    -   `mkplan2.md` の step 6 の PR マーカーの形のコードブロック。
    -   `mkplan.md` の step 8.5 と `_lib/simple-process.md` が書かせる「PR 構成」の節の行の形。
-   [ ] **ステップ 6-7**: ADR-0046 の「この決定は、まだ」で始まるずれの記述を、このマージの後に残るずれ（コマンドの定義、CLAUDE.md の一部、道具のインストール、settings.json）を述べる形に書き換え、commit のメッセージと PR の説明に訂正を書く。

**完了の条件:** 設計書 3.1.1 の `wait`・`tick`、3.5 の周・観測・自動送信の記録、3.7・3.8 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-6 作成ポイント: wait, tick, and automatic sending

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7

**推奨タイトル**: `feat(0088): chief の待ち受けとマージの後の自動送信を作り、PR マーカーに自動送信の欄を足す`

**レビュー観点**: 自動送信の引き金が画面に依らないか / 1 つのマージで 2 回送らないか（起動し直し、持ち越し、記録の後の確かめの失敗） / 見ていないマージを提案にするか / 欄の読み方が実装モデル要件と同じ場所の規則か

**実装モデル要件**: opus

**判定理由**: ステップ 6-3 は人に尋ねずに入力を送る判断（security-gate）で、排他と記録の順序を持つ並行の処理である。Sonnet では、排他を取った後の確かめ直しや、自動送信の記録を作る順序を誤り、2 回送るか見ていないマージで送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: `/chief` の定義、設定、文書、移行、実地の確かめ（設計書 8 章の 7）

**変更するファイル:** `.claude/commands/chief.md`（新規）、`.claude/settings.json`、`internal/doctestutil/protocol.go`・`protocol_test.go`、`cmd/fixprguard/docs_test.go`、`cmd/codexreview/docs_test.go`、`cmd/chief/docs_test.go`、`Makefile`、`CLAUDE.md`、`docs/dev/guides/chief_guide.md`（新規）、`docs/dev/adr/0014-deterministic-checks-in-go.md`・`0046-chief-of-staff.md`、本計画

-   [ ] **ステップ 7-1**: 試作をリポジトリの外へ移す（決定 16）。
    -   人の承認を得て、この worktree の試作の `.claude/commands/chief.md` と `.claude/chief/` をリポジトリの外へ移し、git の共通ディレクトリの `info/exclude` の試作の 3 行を消す。
    -   `.claude/chief/hold.tsv` の行（`0084_wasm_pipeline_poc` の `PR-1` の後）について、0084 の計画書にまだ止める必要があるかを人に確かめる。要るなら、0084 の計画書に `**自動送信**: hold` を足す PR を、ステップ 7-6 の実地の確かめの前に別に作る（設計書 8 章の 7）。
-   [ ] **ステップ 7-2**: `.claude/commands/chief.md` を書く（設計書 3.2）。`chief` の呼び出しは `--protocol 1` を書く。次のガードを足す。
    -   `internal/doctestutil/protocol.go` に `ToolChief` を足し、`protocol_test.go` に chief の行を足す。
    -   `cmd/fixprguard/docs_test.go` の数える条件を直し（決定 8）、chief の場合を足す。
    -   `cmd/chief/docs_test.go` に、`--protocol` の番号が道具と合うこと、定義の中の呼び出しが道具の引数の解析を通ること、定義が `herdr` を直接呼ばないことのガードを書く（設計書 7.2 の 1 項目め、`cmd/codexreview/docs_test.go:158,255,554` と同じ形）。
-   [ ] **ステップ 7-3**: `.claude/settings.json` に `Bash(chief *)` を `allow` に、決定 9 の呼び出しを `deny` に足す。
    -   `chief` が codex を走らせないことを確かめ、`cmd/codexreview/docs_test.go` の `launchFree` にその旨の注記とともに `chief` を足し、`TestMayBypassTool` に行を足す。
    -   `cmd/chief/docs_test.go` に、許可の規則と、拒否の規則の決まった形のそれぞれがあることを確かめるガードを書く（設計書 7.2 の 3 項目め）。
-   [ ] **ステップ 7-4**: 文書と導入の手順を足す。
    -   `Makefile` に `install-chief` を足し、`cmd/chief/docs_test.go` に `cmd/codexreview/docs_test.go:655` と同じ形のガードを書く。
    -   `docs/dev/guides/chief_guide.md` を書く（設計書 2.2 の表の行）。手引きが持つ値（サブコマンド、終了コード、`--protocol`、状態のディレクトリ）を道具の定義と比べるガードを `cmd/chief/docs_test.go` に書く。
    -   `CLAUDE.md` の「Quick Links」「Build Commands」「Claude Commands」「Tool Execution Safety」（設計書 3.9）を直し、`/chief` の定義が手引きを参照するときは「Japanese guides an agent reads」の一覧を直す。
-   [ ] **ステップ 7-5**: ADR-0014 の 19 行のずれの記述を消し、道具の一覧とインストールの項目に `chief` を足す。ADR-0046 の「この決定は、まだ」で始まるずれの記述を消す。どちらも commit のメッセージと PR の説明に訂正を書く。
-   [ ] **ステップ 7-6**: 実地の確かめを行う（設計書 7.3、決定 21）。結果を §7 の「PR-7 の実行の記録」に書く。
    -   人がこのブランチで `make install-chief` を行い、`/chief` を新しいセッションで動かす。
    -   採取用の workspace に向けて、中継、モデルの確かめ（Claude Code と OpenCode）、リセット、ロック（2 つ目の chief が拒まれる）、送信の停止と `release` を、それぞれ 1 回以上行う。
    -   自動送信は、人が通常の `/runpr` でマージする実際のタスクの PR で確かめる。マージの直後に `/runpr` と chief の fetch が重なっても、どちらも失敗しないか、失敗したら次の周で確かめ直すことを確かめる（決定 20）。
    -   §1.3 の決定 14 が挙げるセッションの振る舞いを確かめる。表の下の質問の要点と pane、示せる数を超えたときの挙げ方（実際のタスクの workspace で人の判断を待つものが 4 つ以上あるとき）、まだ答えのない問い、詳細の後の問い、`/runpr` の送信をバックグラウンドで呼ぶこと、通知、`wait` の失敗である。
    -   終わったら、人がマージの後に `main` から入れ直すことを手引きのとおりに示す。

**完了の条件:** 設計書 3.2・3.9 の項目、2.2 の表の `Makefile`・手引き・`CLAUDE.md` の行がそろっている。§7 に PR-7 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-7 作成ポイント: chief command, settings, and documents

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6

**推奨タイトル**: `feat(0088): /chief の定義と設定と手引きを足し、chief を実地で確かめる`

**レビュー観点**: `/chief` の定義が判定を持たず、`herdr` を直接呼ばないか / settings.json の拒否が決定 9 の呼び出しをすべて拒み、既存の許可のガードと矛盾しないか / CLAUDE.md の承認の範囲が設計書のとおりか / 実地の確かめの記録

**実装モデル要件**: opus

**判定理由**: ステップ 7-6 は本物の Herdr のほかのエージェントへ入力を送る確かめ（外部の資源に触れる面）で、7-3 は信頼の境界のガードである。Sonnet では、確かめの範囲を落とすか、採取用の workspace の外の pane に送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1） | 画面のサンプル、画面の読み取り、無害化 | 画面の形がテストで固定される |
| M2（PR-2） | 入力の型、読み取りの部品、送り先の表 | `runplan.md`・`runpr.md`・`mkplan2.md` が送り先の表を示す |
| M3（PR-3） | `chief start`・`chief snapshot` | 道具が状況の表の材料を返す（まだ使われない） |
| M4（PR-4） | `send`・`detail`・`release`、リセット | 道具が人の選んだコマンドを送れる |
| M5（PR-5） | `relay` | 道具が答えを中継できる |
| M6（PR-6） | `wait`・`tick`、自動送信、`**自動送信**` の欄 | 新しい計画書が欄を持つ |
| M7（PR-7） | `/chief`、settings.json、手引き、CLAUDE.md | chief が使える |

順序は設計書 8 章のとおりで、8 章から動かしたものは §1.3 の決定 3 にすべて挙げた。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | 画面のサンプル、画面の読み取り、文字の無害化、CLAUDE.md の例外 | opus |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | 入力の型、Herdr・タスク・送り先の表の読み取り、`_context.md` の送り先の表、CI の `doc-guard` | opus |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | 状態の区分、`snapshot`、`start` とロック、`cmd/chief` | sonnet+opus-review |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | 送信、リセット、送信の停止、`detail` | opus |
| PR-5 | 5-1 / 5-2 / 5-3 | 答えの中継 | opus |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 | 待ち受け、自動送信、`**自動送信**` の欄 | opus |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 | `/chief` の定義、設定、手引き、CLAUDE.md、移行、実地の確かめ | opus |

本計画の PR マーカーは `**自動送信**` の欄を持たない。欄は PR-6 で定まり、chief は PR-7 のマージまで使われないためである。欄がないマーカーは `auto` として扱われる（AC-50）。

### 3.3. 送り先の表へ置き換える箇所の検索

ステップ 2-6 が直す箇所を、コミット `b61a8a1f`（`88618e43` に本計画だけを足したもの）で次のとおり探した。`_lib/review-subagent-pattern.md:109` はレビューのモデルの話で、送り先と関係がないので直さない。検索の語は Sonnet と「model to start」の形だけを拾うので、`opus` の値の説明でモデルを述べる `mkplan2.md:42` は出力にないが、ステップ 2-6 で直す（`rg -n 'on Opus' .claude/commands` で確かめた。ほかに出る行は、下の出力にある `runplan.md:21` と、Opus のレビューの話で送り先と関係がない行である）。

```text
$ rg -n 'on Sonnet|to Sonnet|implement.*Sonnet|Model to start|model each reading recommends|model that reading recommends|recommends none|model to start' .claude/commands docs/dev/project_overview.md CLAUDE.md
.claude/commands/mkplan2.md:41:   - **Model requirement**: Decide which model implements each PR and whether an Opus review is added, so Opus is spent
.claude/commands/mkplan2.md:43:     - `sonnet+opus-review` — implement the PR on Sonnet, and run one Opus review before the PR opens (`runplan.md` dec
.claude/commands/mkplan2.md:44:     - `sonnet` — implement the PR on Sonnet with no further review. Its trigger: everything else.
.claude/commands/runplan.md:20:> through its table), and which model each reading recommends: start the
.claude/commands/runplan.md:21:> session on Opus for `opus`, and on Sonnet for `sonnet+opus-review` and
.claude/commands/runplan.md:60:   - the model that reading recommends (the same section's table), or that it recommends none;
.claude/commands/_context.md:109:     model to start a session with from this table, by naming it rather than
.claude/commands/_context.md:113:     | Reading | Model to start the session with | Opus review |
.claude/commands/_lib/review-subagent-pattern.md:109:  rather than downgrading further (e.g. to haiku); a task pinned to Sonnet because it
.claude/commands/runpr.md:749:   NEXT will open, and the model to start the new session with. That PR is
.claude/commands/runpr.md:763:   before NEXT, which model to start the new session with. Step 5 reuses the
.claude/commands/runpr.md:1059:  4. when NEXT is `/runplan`, the `実装モデル要件` and the model to start the
```

（出力の行は 150 文字で切った。）

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 道具の判定と副作用は、`internal/chief` と `cmd/chief` のユニットテストで確かめる。外部のコマンドは呼び出しの記録係で、画面は `testdata/screens/` のサンプルで、計画書は `testdata/plans/` のサンプルで、時刻と待ちは差し込んだ fake で置き換える（§1.2、決定 19）。設計書 7.1 の各項目を、§2 の各ステップのテストが受け持つ。
-   **切り離し:** テストは、本物の `herdr`・`gh`、本物の状態のディレクトリ、利用者の git の設定と署名に触れない（決定 19）。
-   **並行の処理:** ロックの引き継ぎと workspace の排他は、テストが排他を持ったまま状態を変える形で、決まった順に起こす（ステップ 3-2・4-3）。ゴルーチンの競争の結果には頼らない。
-   **テストの補助:** `internal/chief` の中だけで使う補助（呼び出しの記録係、Herdr の出力と画面の fake の組み立て、一時的な git のリポジトリ）は `internal/chief/test_helpers.go`（必要なら `test_helpers_<category>.go`）に `//go:build test` で置く（test_organization.md の Classification B）。`exec.Command` を呼ぶ補助には、その呼び出しだけに理由つきの `//nolint:gosec` を付ける。ほかのパッケージから使う補助は作らない。
-   **境界の値:** 読む量の上限、`wait` と自動送信の時間の窓（決定 4）、transcript の読み直しの回数（決定 7）、片付けの日数は、境界の前後をテストする。
-   **文書のガード:** `cmd/chief/docs_test.go` に、`--protocol` の番号と呼び出しの解析と `herdr` を直接呼ばないこと（ステップ 7-2）、settings.json の許可と拒否（7-3）、`install-chief` と手引きの値（7-4）、PR マーカーの形の `**自動送信**`（6-6）を置く。CI の `doc-guard` がこれらと `internal/chief` の文書を読むテストを走らせる（決定 18）。「Tool Execution Safety」の承認と CLAUDE.md の例外の文は、PR のレビューで読む（ADR-0016、設計書 7.2）。
-   **既存のガード:** `TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`internal/config/envaccess_test.go`・`TestADRRecords`・`TestInstructionDocPaths`・`fixprguard` と `codexreview` の `--protocol` と settings.json のガードが、文書と構成の変更を確かめる（§1.4）。直すものは、各ステップに挙げた。
-   **実地の確かめ:** PR-1 で画面とキー入力を、PR-7 で chief 全体を、本物の Herdr で確かめる（設計書 7.3、決定 21）。
-   **後方互換:** 旧い値の実装モデル要件と、`**自動送信**` の欄のない PR マーカーを、ステップ 2-4・2-5 のテストで読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 採ったサンプルに、採取した人を特定できる情報が残る | ステップ 1-3 の置き換えの規則とテスト。PR-1 のレビュー観点に入れる |
| サンプルの行末の空白が pre-commit で書き換えられる | 決定 22 |
| エージェントの版が上がって画面の形が変わる | 合わない画面は「要確認」か「送らない」になる（設計書 3.4.2）。採り直しは新しい PR で行う。settings.json の拒否のため、エージェントが pane へキー入力を送る採り直しは人が行う |
| キー入力が選択の位置に依り、違う選択肢を選ぶ | ステップ 1-4 で実際に送って確かめ、確かめられないエージェントには中継を示さない |
| PR マーカーの読み方が `opus-review-decision.sh` と食い違う | ステップ 2-5 のテストで、同じサンプルを両方に読ませる。区別できない値はステップ 2-4 のテストが受け持つ |
| 送り先の表を節の中に置き、`opus-review-decision.sh` が旧い値の表として読む | 表を節の外に置き（ステップ 2-6）、`**自動送信**` の説明に 2 つのセルの表を置かない（6-5）。ステップ 2-5 のテストが今の `_context.md` で確かめる |
| Claude Code の transcript の場所か行の形が変わる | 最初の入力の前の確かめで止まり、何も送らない（設計書 3.1.7） |
| chief の fetch がマージの直後の `/runpr` や `/mergepr` の fetch と重なり、どちらかが失敗する | 決定 20。PR-7 の実地の確かめで重なりを確かめる |
| テストが開発者の本物の Herdr か chief のロックに触れる | 決定 19 |
| 途中の PR の道具を誰かが手で走らせ、入力が送られる | PR-7 まで `install-chief` と `/chief` がない。途中で手で走らせても、送るのは型で決まった入力だけである |
| 試作の定義が、新しい定義と同じパスで除かれたまま残る | ステップ 7-1 でリポジトリの外へ移してから書く（決定 16） |
| `hold.tsv` の止める行を移し忘れ、0084 の PR が自動送信される | ステップ 7-1 で人に確かめ、要るなら実地の確かめの前に 0084 の計画書を直す |
| 実地の確かめで、意図しない pane に送る | 中継・リセット・停止は採取用の workspace だけに向けて行う。自動送信は人がマージを選んだ実際の PR に限る（決定 21） |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
-   [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: 下の表のテストで、振る舞いを確かめる。`make test` が走らせる。テストの名前は目安で、実装で名前を変えたときは、この表を同じ commit で直す。
-   `static`: ガードのテストで、文書と設定の値と構造を確かめる。`make test` と CI の `doc-guard` が走らせる。
-   `manual`: PR のレビューで `/chief` の定義の該当箇所を読み、PR-7 の実地の確かめ（ステップ 7-6）で振る舞いを確かめる（§1.3 の決定 14）。

表のパスは、`internal/chief/` を `ic/`、`cmd/chief/` を `cc/` と略す。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `ic/state_test.go::TestSnapshotExcludesChiefWorkspace`、`ic/workspace_test.go::TestTaskWorkspaces` | test | 3-3・3-4 | PR-3 |
| AC-02 | `ic/state_test.go::TestRowContents`（実行中の行は送信の記録から） | test | 3-4・4-6 | PR-3、PR-4 |
| AC-03 | `ic/state_test.go::TestRowOrder` | test | 3-4 | PR-3 |
| AC-04 | `ic/network_test.go::TestSnapshotSkipsDetail`、`/chief` の定義 | test、manual | 3-7・7-2 | PR-3、PR-7 |
| AC-05 | `ic/state_test.go::TestQuestionInRow`、`/chief` の定義 | test、manual | 1-6・3-4・7-2 | PR-1、PR-3、PR-7 |
| AC-06 | `ic/state_test.go::TestNeedsCheckReasons`、`ic/workspace_test.go::TestWorkspaceReadFailures` | test | 3-3・3-4 | PR-3 |
| AC-07 | `cc/main_test.go::TestStartOutsideHerdr` | test | 3-6 | PR-3 |
| AC-08 | `ic/records_test.go::TestStartRefusesLiveOwner`、`ic/records_test.go::TestLockTakeoverRecheck` | test | 3-2 | PR-3 |
| AC-09 | `ic/screen_test.go::TestParseScreenSamples`、`ic/state_test.go::TestClassifyWorkspaceStates` | test | 1-6・3-4 | PR-1、PR-3 |
| AC-10 | 同上（OpenCode の質問のサンプル） | test | 1-6・3-4 | PR-1、PR-3 |
| AC-11 | 同上（マージの確認のサンプル）、`ic/state_test.go::TestMergeQuestionOffersNoRunpr` | test | 1-6・3-4 | PR-1、PR-3 |
| AC-12 | `ic/screen_test.go::TestParseScreenRejectsNonMatching`、`ic/state_test.go::TestNeedsCheckReasons`、`ic/send_test.go::TestSendChecksBeforeFirstInput`（合わない画面） | test | 1-6・3-4・4-2 | PR-1、PR-3、PR-4 |
| AC-13 | `ic/pending_test.go::TestPendingOrder`、`/chief` の定義（「何もしない」） | test、manual | 3-5・7-2 | PR-3、PR-7 |
| AC-14 | `/chief` の定義と実地の確かめ（§1.3 の決定 14 の例外）。示す順の材料は AC-13 の test | manual | 7-2・7-6 | PR-7 |
| AC-15 | `ic/network_test.go::TestSnapshotSkipsDetail`（`snapshot` が送らない）、`/chief` の定義 | test、manual | 3-7・7-2 | PR-3、PR-7 |
| AC-16 | `/chief` の定義と実地の確かめ（§1.3 の決定 14 の例外） | manual | 7-2・7-6 | PR-7 |
| AC-17 | `ic/detail_test.go::TestDetailItems`、`/chief` の定義（要約） | test、manual | 4-4・7-2 | PR-4、PR-7 |
| AC-18 | `ic/detail_test.go::TestDetailKeepsCheckBuckets` | test | 4-4 | PR-4 |
| AC-19 | `ic/detail_test.go::TestDetailReportSource` | test | 4-4 | PR-4 |
| AC-20 | `ic/detail_test.go::TestDetailOmitsReviewBody` | test | 4-4 | PR-4 |
| AC-21 | `ic/detail_test.go::TestDetailPartialFailure` | test | 4-4 | PR-4 |
| AC-22 | `ic/detail_test.go::TestDetailItems`（送り先の表示）、`/chief` の定義（問い） | test、manual | 4-4・7-2 | PR-4、PR-7 |
| AC-23 | `ic/detail_test.go::TestReviewSkippable` | test | 4-4 | PR-4 |
| AC-24 | `ic/records_test.go::TestWorkspaceBusy`、`/chief` の定義（バックグラウンド） | test、manual | 4-3・7-2 | PR-4、PR-7 |
| AC-25 | `ic/sanitize_test.go::TestSanitize`、`ic/screen_test.go::TestParseScreenSamples`（すべての問い） | test | 1-5・1-6 | PR-1 |
| AC-26 | `ic/screen_test.go::TestParseScreenRejectsNonMatching`、`ic/relay_test.go::TestNoCheckValueForUnrelayable` | test | 1-6・5-1 | PR-1、PR-5 |
| AC-27 | `ic/relay_test.go::TestRelayRequiresAllAnswers` | test | 5-2 | PR-5 |
| AC-28 | `ic/relay_test.go::TestRelayCheckValueMismatch` | test | 5-2 | PR-5 |
| AC-29 | `ic/paneinput/paneinput_test.go::TestRelayKeysFromNumbersOnly`、`ic/relay_test.go::TestRelaySendsChosenOption` | test | 5-2 | PR-5 |
| AC-31 | `ic/paneinput/paneinput_test.go::TestZeroAndOutOfRangeAreUnusable`、`ic/herdr_test.go::TestWriterRefuses`、`cc/docs_test.go::TestSettingsDeniesPaneInput` | test、static | 2-1・4-1・7-3 | PR-2、PR-4、PR-7 |
| AC-32 | `ic/task_test.go::TestReadModelRequirement`、`ic/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript` | test | 2-4・2-5 | PR-2 |
| AC-33 | `ic/state_test.go::TestUnreadableRequirementOffersNoRunplan` | test | 3-4 | PR-3 |
| AC-34 | `ic/send_test.go::TestClaudeModelSwitchCheck` | test | 4-2 | PR-4 |
| AC-35 | `ic/send_test.go::TestOpenCodeModelMatch` | test | 4-2 | PR-4 |
| AC-36 | `ic/send_test.go::TestSendChecksBeforeFirstInput`（残りの文字を返す） | test | 4-2 | PR-4 |
| AC-37 | `ic/send_test.go::TestSendFailureAfterInputStops`、`ic/records_test.go::TestUnfinishedSendBecomesStop`、`ic/records_test.go::TestStopSurvivesRestart`、`ic/records_test.go::TestReleaseClearsStop`、`ic/records_test.go::TestWorkspaceKeyIsCanonical` | test | 3-2・4-2・4-3 | PR-3、PR-4 |
| AC-38 | `ic/send_test.go::TestMkreqTarget` | test | 4-5 | PR-4 |
| AC-39 | `ic/routing_test.go::TestRoutingRowChangesOneRoute` | test | 2-7 | PR-2 |
| AC-40 | `ic/task_test.go::TestBranchPlanStepsDone`、`ic/state_test.go::TestTakeoverProposal`、`ic/send_test.go::TestSendArgumentValidation`（`runplan-takeover`） | test | 2-4・3-4・4-5 | PR-2、PR-3、PR-4 |
| AC-41 | `ic/autosend_test.go::TestAutoSendConditions` | test | 6-3 | PR-6 |
| AC-42 | `ic/autosend_test.go::TestAutoSendReportMismatch` | test | 6-3 | PR-6 |
| AC-43 | `ic/state_test.go::TestNextCommand`（簡易な進め方の 3 文書の後） | test | 3-4 | PR-3 |
| AC-44 | `ic/autosend_test.go::TestAutoSendOnce`（起動し直し、記録を読めないとき） | test | 6-2・6-3 | PR-6 |
| AC-45 | `ic/autosend_test.go::TestAutoSendConditions`（満たさなかった条件の提案） | test | 6-3 | PR-6 |
| AC-47 | `ic/autosend_test.go::TestAutoSendNeverMkreqOrRunpr`、`ic/state_test.go::TestClassifyWorkspaceStates`（タスク完了） | test | 3-4・6-3 | PR-3、PR-6 |
| AC-48 | `cc/docs_test.go::TestMarkerFormsCarryAutoSend`（`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md` の形、`_context.md` の欄の一覧） | static | 6-5・6-6 | PR-6 |
| AC-49 | `ic/autosend_test.go::TestAutoSendHold` | test | 6-3 | PR-6 |
| AC-50 | `ic/task_test.go::TestReadAutoSendField`（欄がない場合）、`ic/autosend_test.go::TestAutoSendConditions` | test | 2-4・6-3 | PR-2、PR-6 |
| AC-51 | `ic/task_test.go::TestReadAutoSendField`（2 つ以上、値が違う場合）、`ic/autosend_test.go::TestAutoSendConditions` | test | 2-4・6-3 | PR-2、PR-6 |
| AC-52 | `ic/reset_test.go::TestResetAfterRunpr` | test | 4-5 | PR-4 |
| AC-53 | `ic/autosend_test.go::TestResetAfterAutoSend` | test | 6-3 | PR-6 |
| AC-54 | `ic/reset_test.go::TestResetChecks`（残りの文字を返す） | test | 4-5 | PR-4 |
| AC-55 | `ic/reset_test.go::TestDetailSavedBeforeReset`、`ic/detail_test.go::TestDetailAfterReset` | test | 4-4・4-5 | PR-4 |
| AC-56 | `ic/wait_test.go::TestWaitReturnsOnChange`、`ic/autosend_test.go::TestAutoSendConditions`、実地の確かめ | test、manual | 6-1・6-3・7-6 | PR-6、PR-7 |
| AC-57 | `ic/wait_test.go::TestWaitKeepsWaitingOnBlocked`、`ic/wait_test.go::TestWaitIgnoresChiefWorkspace` | test | 6-1 | PR-6 |
| AC-58 | `ic/pending_test.go::TestNoticesOncePerSession`、`/chief` の定義（通知の送信） | test、manual | 3-5・7-2 | PR-3、PR-7 |
| AC-59 | `ic/autosend_test.go::TestAutoSendUnobservedMerge` | test | 6-3 | PR-6 |
| AC-60 | `ic/state_test.go::TestNeedsCheckReasons`（Herdr の失敗）、`ic/wait_test.go::TestWaitFailure`、`/chief` の定義 | test、manual | 3-4・6-1・7-2 | PR-3、PR-6、PR-7 |
| AC-61 | `ic/network_test.go::TestNetworkCommandsAreReadOnly`（呼び出しの記録係がすべてのテストで確かめる一覧を含む） | test | 3-7・4-6・6-4（記録係は 2-3） | PR-3、PR-4、PR-6 |
| AC-62 | `ic/send_test.go::TestActionResults`、`ic/relay_test.go::TestRelaySendsChosenOption`、`ic/autosend_test.go::TestAutoSendConditions` | test | 4-7・5-3・6-3 | PR-4、PR-5、PR-6 |
| AC-63 | `ic/send_test.go::TestActionResults` | test | 4-7 | PR-4 |

### PR-1 の実行の記録

（ステップ 1-8 で書く）

### PR-7 の実行の記録

（ステップ 7-6 で書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の表のすべての行の確かめが通り、PR-7 の実地の確かめで設計書 7.3 の項目をそれぞれ 1 回以上確かめた。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは壊して失敗を確かめ、commit のメッセージに書いた。
-   **セキュリティ:** 設計書 7.4 の項目（無害化、上限、照合値、型のゼロ値と範囲の外の値で送らない、chief の workspace に送らない）をテストで確かめ、settings.json の拒否をガードで確かめた。
-   **文書:** `/chief` の定義、手引き、CLAUDE.md、`_context.md`、`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md`、`package_reference.md` が実装と一致し、ADR-0014・ADR-0042・ADR-0046 の決定の中のずれの記述が消えた。

## 9. 次のステップ (Next Steps)

-   PR-7 のマージの後、人が `make install-chief` を `main` から行い、chief の workspace で `/chief` を動かす。
-   試作から移る手順は手引き（`chief_guide.md`）に従う。
-   `sonnet+opus-review` の `/runplan` を OpenCode・DeepSeek V4.1 Flash に差し替えて比べること、#839（worktree と workspace の作成）、#840（`/runpr` の Opus への切り替え）、#760（値の名前の見直し）は、別のタスクで扱う。

## 変更履歴

- 2026-11-12 承認（isseis、#855）
- 2026-11-12 セッション中の再承認：AC-30・AC-46 を実装するステップと確かめを除き、マージの確認のサンプルを 0089 の後のマージの質問で採る（§1.5・1-2・5-2・§7）（isseis、#未記入）
