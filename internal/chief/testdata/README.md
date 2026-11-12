# chief のテストデータ

`internal/chief` のテストが読む、実物から採ったサンプルである。タスク 0088 の PR-1（実装計画書のステップ 1-2〜1-4）で採った。リポジトリに置く実データの規則は [security.md](../../../docs/dev/security.md) §7 と [ADR-0032](../../../docs/dev/adr/0032-testdata-license.md) にある。

## 採った日と版

-   採った日: 2026-11-12
-   Herdr 0.9.1、Claude Code 2.1.322、OpenCode v2.2.6、gh 2.96.0
-   OpenCode のモデル: `DeepSeek V4.1 Flash 1017`（提供元 `OpenCode Go`）。Claude Code のモデル: Opus 5.5（`/model` で Sonnet 5.5 に切り替えたものを含む）

エージェントの版が上がって画面の形が変わったときは、同じ手順で採り直す。

## 採り方

-   **採取用の workspace。** 人が Herdr に使い捨ての workspace を用意し、Claude Code と OpenCode の pane を 1 つずつ起動した。エージェント（PR-1 の `/runplan` のセッション）が、その pane にだけ `herdr agent prompt` で頼んで状態を作り、`herdr agent send-keys` でキー入力を確かめた。
-   **画面。** `herdr agent read <pane> --format ansi` で読んだ。エージェントが `blocked` の間は `--source recent` を Herdr が拒むので、`--source visible --lines 200` で読んだ（pane に見えている行だけになる）。それ以外は `--source recent --lines 200` で読んだ。
-   **マージの確認。** 採った日には、どのタスクの workspace にも `/runpr` のマージの質問が出ていなかった。採取用の pane に、`.claude/commands/runpr.md` の step 4 の item 3 と同じ問いの文・選択肢の名前と説明・要約の行の形で問いを出させて採った。問いの部品の形は本物の `/runpr` と同じだが、要約の値（PR #123 など）は作ったものである。
-   **報告。** `claude-command-report.ansi` と `opencode-runpr-report.txt` は、実際のタスクの workspace に残っていた報告を、読み取りだけで採った。`opencode-runpr-report.txt` は、ANSI の形で採る前に pane が空にされたので、テキストの形だけである。
-   **Herdr の出力。** `herdr agent get`・`herdr agent list`・`herdr workspace list` の出力である。
-   **Claude Code の transcript。** `/clear` の後の新しいセッションで `/model sonnet` と `/model opus` を送った後の transcript のファイル（`~/.claude/projects/<作業ディレクトリから作った名前>/<セッション ID>.jsonl`）の写しである。
-   **gh の出力。** このリポジトリの PR について、`gh pr list`・`gh pr view`・`gh pr checks` を `--json` で走らせた出力である。`exits.txt` に各コマンドの終了コードがあり、`*.stderr` は空でなかった標準エラーである。

## 置き換えたもの

採った人を特定できる値を、コミットする前に次のとおり置き換えた。置き換えの後に、ホームのパスと利用者名が残っていないことを確かめた。

| 元の値 | 置き換えた値 |
|---|---|
| 利用者の GitHub のメールアドレス（`<番号>+<ログイン名>@users.noreply.github.com`） | `1+owner@users.noreply.github.com` |
| 利用者の GitHub のノード ID | `U_OWNER` |
| 利用者の氏名 | `Owner Name` |
| ホームのパス `/Users/<利用者名>`（transcript のディレクトリ名の `-Users-<利用者名>-` を含む） | `/Users/user`（`-Users-user-`） |
| GitHub のログイン名 | `owner` |
| そのほかの利用者名 | `user` |
| セッション ID などの UUID | `00000000-0000-4000-8000-<12 桁の通し番号>`。同じ値は同じ値に置き換えた |

置き換えた文字列は元より短いことがあるので、画面の行の幅は元と同じとは限らない。

## ファイル

### `screens/`

画面のサンプルで、`.ansi` は端末の色と属性の制御を含む。pre-commit の空白のフックと git の改行の変換から外している（`.pre-commit-config.yaml`・`.gitattributes`）。

| ファイル | 画面 |
|---|---|
| `claude-idle.ansi` | Claude Code の入力待ち。入力欄に薄い字（`ESC[2m`）の案内文がある |
| `claude-question-single.ansi` | 1 つを選ぶ問い。選択の位置は 1 |
| `claude-question-single-cursor.ansi` | 同じ問いで、選択の位置を矢印で 3 に動かしたもの |
| `claude-question-multi.ansi` | 複数を選ぶ問い（`[ ]` の印と `Submit` のタブ） |
| `claude-question-tabs.ansi` | 2 つの問い（タブ）の 1 つめ。2 つめの問いは画面にない |
| `claude-question-tabs-second.ansi` | 1 つめに答えた後の 2 つめ |
| `claude-question-tabs-review.ansi` | 両方に答えた後の確かめ（`Submit answers`） |
| `claude-permission.ansi` | 権限の確認（`Bash command`、`Do you want to proceed?`） |
| `claude-input-left.ansi` | 入力欄に送っていない文字が残っている |
| `claude-working.ansi` | 実行中 |
| `claude-after-clear.ansi` | `/clear` の後 |
| `claude-after-model.ansi` | `/model sonnet` の後 |
| `claude-merge-question.ansi` | マージの確認（上の「マージの確認」のとおり再現したもの） |
| `claude-command-report.ansi` | PR に関わるコマンドの最後の報告 |
| `opencode-question-single.ansi` | 1 つを選ぶ問い。選択の位置は 1（色だけで示される） |
| `opencode-question-single-cursor.ansi` | 同じ問いで、選択の位置を 3 に動かしたもの |
| `opencode-question-multi.ansi` | 複数を選ぶ問い |
| `opencode-question-tabs.ansi` | 2 つの問い（タブ）の 1 つめ |
| `opencode-question-tabs-second.ansi` | 1 つめに答えた後の 2 つめ |
| `opencode-question-tabs-review.ansi` | 両方に答えた後の確かめ |
| `opencode-input-left.ansi` | 入力欄に送っていない文字が残っている |
| `opencode-done.ansi` | 応答を終えた後の入力待ち（入力欄は空で、案内文がない） |
| `opencode-after-new.ansi` | `/new` の後。灰色の案内文がある |
| `opencode-merge-question.ansi` | マージの確認（上の「マージの確認」のとおり再現したもの） |
| `opencode-runpr-report.txt` | `/runpr` の最後の報告（テキストだけ） |

次のものは採れなかった。OpenCode の権限の確認は、採取に使った OpenCode の設定がどの操作も許していたためである。OpenCode で PR を作ったコマンドの報告は、その画面がどの pane にも残っていなかったためである。Claude Code の `/runpr` の報告は、`/runpr` を OpenCode で走らせるので要らない。Claude Code のモデルの行と `/new` の後の画面は、chief が使わない（Claude Code は transcript と `agent_session` で確かめる）。

### `herdr/`・`transcript/`・`gh/`

| ファイル | 中身 |
|---|---|
| `herdr/agent-get-*.json` | `herdr agent get` の出力（`blocked`・`idle`・`done` の各状態） |
| `herdr/agent-list.json`・`herdr/workspace-list.json` | `herdr agent list`・`herdr workspace list` の出力 |
| `transcript/claude-clear-model.jsonl` | Claude Code の transcript |
| `gh/pr-list-*.json` | `gh pr list --head <ブランチ> --json number,state,headRefName,title,url` |
| `gh/pr-view-*.json` | `gh pr view <番号> --json state,number,title,url,headRefName,headRefOid,additions,deletions,changedFiles,commits,files,body,reviews` |
| `gh/pr-checks-*.json` | `gh pr checks <番号> --json name,state,bucket,workflow` |
| `gh/exits.txt`・`gh/pr-view-missing.stderr` | 各コマンドの終了コードと、ない PR を読んだときの標準エラー |

CI のジョブのない PR は、採った時点のこのリポジトリになかったので採っていない。
