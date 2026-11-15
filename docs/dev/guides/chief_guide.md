# chief 利用ガイド

chief は、Herdr の workspace で並行して進むタスクを見渡し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る開発用の道具である。Go の道具 `chief`（`cmd/chief`・`internal/chief`）と、それを呼ぶコマンド `/chief`（`.claude/commands/chief.md`）からなる。判定と送信はすべて道具が行い、`/chief` のセッションは道具の出力を人に示し、人の選んだものを道具の引数で渡すだけである（[ADR-0046](../adr/0046-chief-of-staff.md)、[ADR-0014](../adr/0014-deterministic-checks-in-go.md)）。要件と設計は [0088 要件定義書](../../tasks/0088_chief_of_staff/01_requirements.md) と [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) にある。

## 1. 前提条件

| 項目 | 条件 |
|---|---|
| Herdr | 0.9.1 で確かめた。`/chief` は Herdr の pane の中の Claude Code で動かす。Herdr の外では何も読まずに止まる |
| エージェント | Claude Code 2.1.322 と OpenCode v2.2.6 の画面で確かめた。版が上がって画面の形が変わると、その workspace は「要確認」になり、何も送らない |
| gh | 2.96.0 で確かめた。`gh auth login` を済ませておく |
| bash | `make test` には PATH の最初の `bash` が 4 以上であることが要る（CLAUDE.md の「Development Notes」） |
| workspace の作り | タスクごとに git の worktree を作り、それを Herdr の workspace で開く。送り先の pane は、workspace の中に送り先の種類のエージェント（Claude Code か OpenCode）の pane がちょうど 1 つあるときだけ選ばれる |
| chief の workspace | `/chief` を動かす workspace は、タスクの workspace と別にする。chief は自分の workspace へは送らない |

## 2. インストール

```bash
make install-chief
```

`go install ./cmd/chief` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（ADR-0014 の「道具は `main` からインストールして使う」）。インストール済みのバイナリが作られたコミットは、`go version -m "$(command -v chief)"` の `vcs.revision` で確かめられる。

`/chief` の定義と道具は `--protocol` の番号で版を照合する。現在の番号は `1` である。番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで入れ直す。

`.claude/settings.json` は `Bash(chief *)` を許すので、待ち受けの間に許可を尋ねない。道具は `chief` の語で始まり、ほかのコマンドと連ねない形でだけ呼ぶ（`/chief` の定義の「Calling the tool」）。

## 3. 使い方

chief の workspace の Claude Code の pane で、次のどちらかを実行する。

| 呼び方 | 振る舞い |
|---|---|
| `/chief` | 状況の表を示し、人の判断を待つものを尋ね、その後は待ち受けを続ける。待ち受けの間は、マージの後の次のコマンドを自動で送る |
| `/chief` に引数 `once` | 状況の表を 1 回示し、尋ね、答えに応じて動いて止まる。自動送信はしない |

chief は同時に 1 つだけ動く。別のセッションで動いている chief があれば、2 つ目は表を示さずに止まる。前のセッションが終わっていれば（`/clear` を含む）、新しいセッションがロックを引き継ぐ。

人の判断を待つものは、エージェントを止めている質問、`/runpr` のマージの確認、レビュー待ちの PR、そのほかの提案の順に、同じ種類の中では待っている時間の長い順に示される。1 回の問いで示せるのは 3 つと「何もしない」で、示せなかったものは問いの文に workspace の名前で挙がる。その名前を自由記述の答えに書けば選べる。

## 4. サブコマンドと終了コード

どのサブコマンドも `--protocol N` を必須とし、標準出力に 1 つの JSON を書く。サブコマンドの細部は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.1.1 にある。

| サブコマンド | 何をするか |
|---|---|
| `start` | ロックを取り、状態のディレクトリを確かめ、古い記録を片付け、終わり方の分からない送信を送信の停止に変える |
| `snapshot` | 状況を読み、状況の表、人の判断を待つもの、新しい通知を返す。送らない |
| `tick` | `snapshot` に加えて、PR が開いているのを見た記録を作り、自動送信の条件を満たすものを送る |
| `wait` | 前の周の基準から、タスクの workspace のエージェントの状態が変わるまで待つ（既定は 10 分で時間切れ） |
| `send` | 人の選んだコマンドを送る。`/runpr` の後は、PR を作ったコマンドの pane を空にする |
| `relay` | 人の選んだ答えを、質問で止まっているエージェントへ中継する |
| `detail` | workspace の開いている PR の詳細を集め、控える |
| `release` | 人の選択で、workspace の送信の停止を解く |

| 終了コード | 意味 |
|---|---|
| `0` | 処理を終えた。送らなかった判断も含み、出力が理由を持つ |
| `1` | 道具の失敗（読めない入力、記録を作れない、外部のコマンドの失敗、`wait` の時間切れ） |
| `2` | 使い方の誤り |
| `3` | 何もせずに拒んだ（Herdr の外、ほかの chief がロックを持つ、`start` していない、`--protocol` が合わない、状態のディレクトリの土台がない） |

## 5. 保証すること・しないこと

### 5.1 保証すること

-   **送る入力は決まった範囲だけである。** 道具が pane へ送るのは、要件定義書 F-006 の表のコマンド（`/clear`・`/model`・`/new` と、タスクのコマンド）、中継の選択肢の番号、リセットの `/clear`・`/new` だけで、どれも型から組み立てる。文字列の入力は受け取らない。
-   **人が選ぶまで送らない。** 自動送信のほかは、人が `/chief` の問いで選んだものだけを送る。中継は、送る直前に、人に示した質問と選択肢がまだ画面にあることを照合値で確かめる。
-   **自動送信の引き金は画面ではない。** GitHub の PR の状態（`MERGED`）と git の状態で確かめ、画面からの判断は送ることを止めるか条件を狭める向きにだけ働く。マージした PR 1 件につき 1 回までで、chief を起動し直しても 2 回送らない。
-   **分からないものは送らない。** 画面、Herdr の出力、計画書の欄、送り先の表、記録のどれかが決まった形に合わなければ、「要確認」か「送らない」にし、理由を示す。
-   **送れたかが分からない送信は送り直さない。** その workspace への送信を、人が `release` で解くまで止める。
-   **待ち受けの中のネットワークの操作は読み取りだけである。** `git fetch origin main`・`git ls-remote`・`gh pr list`・`gh pr view`・`gh pr checks` だけを呼び、push、マージ、コメント、issue の操作はしない。

### 5.2 保証しないこと

-   **同じ利用者のほかのプロセスからの守り。** 同じ利用者のプロセス（ほかの worktree のエージェントを含む）は、`chief` も `herdr` も直接呼べ、状態のディレクトリも `origin/main` の参照も書ける。chief の守りは、chief のセッションが信頼できない文字列に従っても決まった範囲の外の入力を送らないことにあり、悪意のあるプロセスを防ぐことにはない（設計書 5.2 の残る危険）。
-   **settings.json の拒否の規則の外の書き方。** 拒否は前方一致の規則なので、パスで書いた呼び出しなどは拒めない（6 章）。
-   **人が選ぶ送信の次のコマンドの鮮度。** 人が選んで送るコマンドは、ローカルの `origin/main` の文書から決まる。マージの直後は、次の fetch まで 1 段前のコマンドを示しうる。表で確かめてから選ぶ。

## 6. `.claude/settings.json` の拒否

chief のセッションが道具を通さずに入力を送らないよう、リポジトリの `.claude/settings.json` は次の herdr の呼び出しを拒む（ADR-0046）。この拒否は、リポジトリで動くすべての Claude Code のセッションに効く。道具は子のプロセスとして `herdr` を呼ぶので、影響を受けない。

| 種類 | 拒む呼び出し |
|---|---|
| pane へ入力を送るか、pane でコマンドを走らせるか、pane の入力の経路を変える | `herdr agent prompt`・`herdr agent send-keys`・`herdr agent start`・`herdr agent attach`・`herdr pane send-text`・`herdr pane send-keys`・`herdr pane run`・`herdr pane input` |
| エージェントの状態とセッションを Herdr に報告する | `herdr pane report-agent`・`herdr pane report-agent-session`・`herdr pane release-agent` |
| サブコマンドの前に、別のセッションか別の機械を指す引数を書く | `herdr --session`・`herdr --machine`・`herdr --remote` で始まるもの |

読み取りの呼び出し（`list`・`get`・`read` など）は拒まない。人がこれらの操作を Claude Code に頼むと拒まれるので、その操作は人が端末で直接行う。画面のサンプルを採り直すときのキー入力も、人が行う。

## 7. 状態のディレクトリ

記録とロックは、リポジトリの外の `$XDG_STATE_HOME/yt2column/chief/`（`XDG_STATE_HOME` が絶対パスでなければ `$HOME/.local/state/yt2column/chief/`）に置く。`start` の出力の `state_dir` が、解決したパスである。ファイルは利用者だけが読み書きでき、同じ名前の記録をすでにあれば作らない。

| 記録 | 置き場所 | 何のためか |
|---|---|---|
| ロック | `lock.json` | chief を 1 つに限る |
| 通知の記録 | `notice-*.json` | 同じものを 2 回通知しない。待っている時間による並べ方 |
| 周の記録・PR が開いているのを見た記録 | `round-*.json`・`observe-*.json` | 自動送信の条件（chief の待ち受けの中でマージを見たこと） |
| 自動送信の記録 | `autosend-<PR 番号>.json` | マージした PR 1 件につき 1 回に限る |
| workspace ごとの記録 | worktree の正規のパスの SHA-256 の 16 進を名前にした下のディレクトリ | 送信の記録（`send-*.json`）、PR の詳細の控え（`detail-*.json`）、送信の停止（`stop.json`） |

-   **送信の停止を解く。** `/chief` の問いで `停止を解く: <workspace の名前>` と答えるか、ほかの方法がないときだけ、その workspace のディレクトリの `stop.json` を人が消す。停止の理由と、そのときの画面は、状況の表に示される。消す前に、止まった送信がその pane で何をしたかを確かめる。
-   **片付け。** `start` が、終わったセッションの周・観測・通知の記録、始めと終わりの両方を書いた古い送信の記録、14 日より古い PR の詳細の控えを消す。終わりの記録のない送信の記録は消さず、送信の停止に変える。自動送信の記録は消さない（消すと、同じマージで 2 回送りうる）。
-   **すべてを消す。** chief が動いていないときに、ディレクトリごと消してよい。消した後は、chief を起動する前のマージを自動では送らず、提案として示す。

## 8. 既知の制限

-   **Claude Code の `/model` は既定のモデルも書き換える。** chief が Claude Code の pane で `/model` を送ると、そのセッションのモデルに加えて、利用者の Claude Code の既定のモデル（新しいセッションに使うもの）も書き換わる。既定のモデルは、chief が最後に送ったモデルになる。既定を書き換えない切り替えの方法がないので、受け入れている（設計書 1.3）。
-   **OpenCode のモデルは切り替えない。** OpenCode の pane のモデルが送り先の表と違うときは、chief は送らずにそのことを示す。人がその pane でモデルを切り替え、chief の問いで送り直すかを選ぶ。
-   **worktree のパスの文字。** Claude Code の pane へ送るときは、モデルが切り替わったことを Claude Code の transcript で確かめる。worktree のパスが英数字・`/`・`.`・`-` 以外の文字（たとえば `_`）を含むと transcript の場所を求められないので、その pane には何も送らない（`unsupported_path`）。その文字だけのパスの worktree を使うか、その pane でコマンドを打つ。
-   **コミットの数の上限。** gh は 1 つの PR のコミットを 100 件までしか返さない。PR の詳細は、100 件のときに「100 件以上」と示す。全体の数は GitHub の PR のページ（`gh pr view <番号> --web`）で確かめる。
-   **問いが開いている間は待ち受けが止まる。** `/chief` の問いに答えるまで、次の周は走らない。そのため、問いを開いたままにすると、その間のマージの次のコマンドは送られない。`/chief` は、新しく増えた人の判断を待つものがあるときだけ問いを開く。離れるときは「何もしない」を選ぶ。
-   **中継しない質問。** 権限の確認、自由に書く答え、複数を選ぶ問い、1 つの画面に複数の問い（タブ）がある質問、読む量の上限で切った質問は、中身を示すだけで中継しない。人がその pane で直接答える。

## 9. 試作から移る

0088 より前の試作（`.claude/commands/chief.md` と `.claude/chief/` のシェルスクリプト）を worktree に置いていたときは、次の順に移る。

1.  試作の `.claude/commands/chief.md` と `.claude/chief/` を、リポジトリの外へ移すか消す。試作を git の共通ディレクトリの `info/exclude` で除いていたときは、その行も消す。除いたままだと、同じパスの新しい定義が見えなくなる。
2.  試作の `hold.tsv` に、自動送信を止める行が残っていれば、その計画書の PR マーカーに `**自動送信**: hold` を書く PR を作る（`_context.md` の「PR marker conventions」）。chief は `hold.tsv` を読まない。
3.  `main` を更新し、`make install-chief` を実行する。
4.  chief の workspace の Claude Code の pane で、新しいセッションの `/chief` を実行する。
