# chief のテストデータ

`internal/chief` のテストが読む、実物から採ったサンプルである。タスク 0088 の PR-1（実装計画書のステップ 1-2〜1-4）で採った。リポジトリに置く実データの規則は [security.md](../../../docs/dev/security.md) §7 と [ADR-0032](../../../docs/dev/adr/0032-testdata-license.md) にある。

## 採った日と版

-   採った日: 2026-11-12
-   Herdr 0.9.1、Claude Code 2.1.322、OpenCode v2.2.6、gh 2.96.0
-   OpenCode のモデル: `DeepSeek V4.1 Flash 1017`（提供元 `OpenCode Go`）。Claude Code のモデル: Opus 5.5（`/model` で Sonnet 5.5 に切り替えたものを含む）

エージェントの版が上がって画面の形が変わったときは、同じ手順で採り直す。

## 採り方

-   **採取用の workspace。** 人が Herdr に使い捨ての workspace を用意し、Claude Code と OpenCode の pane を 1 つずつ起動した。エージェント（PR-1 の `/runplan` のセッション）が、その pane にだけ `herdr agent prompt` で頼んで状態を作り、`herdr agent send-keys` でキー入力を確かめた。今はリポジトリの `.claude/settings.json` がこの 2 つを拒む（[chief_guide.md](../../../docs/dev/guides/chief_guide.md) §6）。拒否は許可の設定より強いので、採り直すときは、人が採取用の pane で入力し、エージェントは画面を読む `herdr agent read`（拒まれない）だけを使う。
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
| `claude-permission-warning.ansi` | 権限の確認。折り返したコマンドの行に `│` が付き、`Do you want to proceed?` の前に警告の行と空行がある。タスク 0088 の PR-7 の実地の確かめで、2026-11-15 に Claude Code 2.1.325 の画面から採った |
| `claude-permission-wrapped.ansi` | 権限の確認。選択肢 2 のラベルが pane の幅を超え、ラベルの開始の桁に字下げした行に折り返している。タスク 0088 の PR-7 の実地の確かめで、2026-11-15 に Claude Code 2.1.322 の画面から採った |
| `claude-input-left.ansi` | 入力欄に送っていない文字が残っている |
| `claude-working.ansi` | 実行中 |
| `claude-after-clear.ansi` | `/clear` の後 |
| `claude-titled-rule.ansi` | `/clear` の後の入力待ち。入力欄の上の罫線に、セッションの名前（`Plan 0091 phase group`）が入っている。タスク 0088 の PR-7 の後、2026-11-16 に Claude Code 2.1.325 の画面から、読み取りだけで採った |
| `claude-after-model.ansi` | `/model sonnet` の後 |
| `claude-merge-question.ansi` | マージの確認（上の「マージの確認」のとおり再現したもの） |
| `claude-command-report.ansi` | PR に関わるコマンドの最後の報告 |
| `claude-command-report-recap.ansi` | PR に関わるコマンドの最後の報告。実行の終わりの行と入力欄の間に、しばらく操作がなかった後の `※ recap:` の要約がある。タスク 0088 の PR-7 の実地の確かめで、2026-11-16 に Claude Code 2.1.325 の画面から、読み取りだけで採った |
| `opencode-question-single.ansi` | 1 つを選ぶ問い。選択の位置は 1（色だけで示される） |
| `opencode-question-single-cursor.ansi` | 同じ問いで、選択の位置を 3 に動かしたもの |
| `opencode-question-multi.ansi` | 複数を選ぶ問い |
| `opencode-question-tabs.ansi` | 2 つの問い（タブ）の 1 つめ |
| `opencode-question-tabs-second.ansi` | 1 つめに答えた後の 2 つめ |
| `opencode-question-tabs-review.ansi` | 両方に答えた後の確かめ |
| `opencode-input-left.ansi` | 入力欄に送っていない文字が残っている |
| `opencode-done.ansi` | 応答を終えた後の入力待ち（入力欄は空で、案内文がない） |
| `opencode-after-new.ansi` | `/new` の後。灰色の案内文がある |
| `opencode-model-no-effort.ansi` | 推論の強さを持たないモデル（DeepSeek V4 Pro）の入力待ち。状態の行の区切り ` · ` が 1 つだけ。タスク 0088 の PR-7 の実地の確かめで、2026-11-15 に OpenCode v2.2.6 の画面から採った |
| `opencode-merge-question.ansi` | マージの確認（上の「マージの確認」のとおり再現したもの） |
| `opencode-runpr-report.txt` | `/runpr` の最後の報告（テキストだけ） |

次のものは採れなかった。OpenCode の権限の確認は、採取に使った OpenCode の設定がどの操作も許していたためである。OpenCode で PR を作ったコマンドの報告は、その画面がどの pane にも残っていなかったためである。Claude Code の `/runpr` の報告と PR を作ったコマンドの報告は、画面ではなく transcript から読むので、画面のサンプルを採らず、transcript のサンプル（下の「報告の transcript」）で採った。Claude Code のモデルの行と `/new` の後の画面は、chief が使わない（Claude Code は transcript と `agent_session` で確かめる）。自由に書く答えだけを求める問いには、どちらのエージェントにも専用の画面がない。エージェントが文で尋ねて止まると、入力欄の画面（`*-done.ansi` などと同じ形）になり、質問とは読まない。選択式の問いの「Type something.」「Type your own answer」は、自分で書く答えに移る選択肢として、中継できる選択肢から除く。

### `herdr/`・`transcript/`・`gh/`

| ファイル | 中身 |
|---|---|
| `herdr/agent-get-*.json` | `herdr agent get` の出力（`blocked`・`idle`・`done` の各状態） |
| `herdr/agent-list.json`・`herdr/workspace-list.json` | `herdr agent list`・`herdr workspace list` の出力 |
| `transcript/claude-clear-model.jsonl` | Claude Code の transcript |
| `transcript/claude-queued-input.jsonl` | Claude Code がエージェントの作業中に届いた入力を記録する行（人の文、ほかのエージェントからの報告、タスクの通知）。下の「人の入力の transcript」のとおり採った |
| `transcript/claude-human-input.jsonl` | Claude Code の transcript の、人の入力とそれ以外の行。下の「人の入力の transcript」のとおり採った |
| `transcript/claude-runpr-report.jsonl` | Claude Code で走らせた `/runpr` の実行の終わり。PR の記録（`pr-link` の行）と、`NEXT: なし（…）` の行を含む最後の報告がある。下の「報告の transcript」のとおり採った |
| `transcript/claude-command-report.jsonl` | PR を作ったコマンド（`/mkplan`）の実行の終わり。PR を作った後の PR の記録、画面より長い報告、人の入力（`continue`）、その後の短い最後の報告、recap（`subtype` が `away_summary` の `system` の行）がある |
| `transcript/claude-recap-notification.jsonl` | 終わった応答の後の recap と、task-notification に応えて終わった応答。キューに積まれた task-notification の行を含む |
| `transcript/claude-synthetic.jsonl` | 終わった応答、人の入力、PR の記録、合成の応答（`message.model` が `<synthetic>`）、人の入力の順の行 |
| `transcript/claude-compact.jsonl` | 終わった応答、recap、PR の記録、compact の境目と要約（`isCompactSummary` が true の `user` の行）、その後の `attachment` の行 |
| `transcript/claude-auto-continuation.jsonl` | 思考と本文の 2 行に分かれた終わった応答の後の、origin の種類が `auto-continuation` の行と、続く応答 |
| `gh/pr-list-*.json` | `gh pr list --head <ブランチ> --json number,state,headRefName,title,url` |
| `gh/pr-view-*.json` | `gh pr view <番号> --json state,number,title,url,headRefName,headRefOid,additions,deletions,changedFiles,commits,files,body,reviews` |
| `gh/pr-checks-*.json` | `gh pr checks <番号> --json name,state,bucket,workflow` |
| `gh/exits.txt`・`gh/pr-view-missing.stderr` | 各コマンドの終了コードと、ない PR を読んだときの標準エラー |

CI のジョブのない PR は、採った時点のこのリポジトリになかったので採っていない。

### 人の入力の transcript

`transcript/claude-human-input.jsonl` は、タスク 0088 の PR-4 で、2026-11-15 に Claude Code 2.1.325 の transcript から採った。PR-4 の `/runplan` のセッションの transcript で、行の種類（人が打ったカスタムコマンド、人が打った文、`/clear` の記録、ほかのエージェントからの報告、タスクの通知、ツールの結果、エージェントの応答）ごとに最初の 1 行を、順序を保って取った。各行は、chief が読む項目（`type`・`origin` の `kind`・`message`）と、行の種類を表す項目（`isMeta`・`promptSource`・`turnOrigin`）だけに絞った。人が打った 2 行の `message` は採ったままである。それ以外の行の `message` は、長い本文や作業ディレクトリのパスを含むので、同じ形の短い文に置き換えた。ID は上の表の規則で置き換えた。

`transcript/claude-queued-input.jsonl` は、同じ日に、このプロジェクトのほかの worktree の Claude Code 2.1.304・2.1.305 の transcript から採った。エージェントの作業中に届いた入力は、`type` が `attachment` で、`attachment` の `type` が `queued_command` の行になり、`attachment` の中の `origin` の `kind` が届けたものを示す。`kind` が `human`・`peer`・`task-notification` の行を 1 行ずつ取り、`attachment` の `type`・`commandMode`・`origin` の `kind`・`prompt` に絞り、`prompt` は同じ形の短い文に置き換えた。2.1.322〜2.1.325 の transcript にも同じ形の行があることを確かめた。

### 報告の transcript

`transcript/` の `claude-runpr-report.jsonl`・`claude-command-report.jsonl`・`claude-recap-notification.jsonl`・`claude-synthetic.jsonl`・`claude-compact.jsonl`・`claude-auto-continuation.jsonl` は、タスク 0095 の PR-1（実装計画書のステップ 1-1〜1-5）で、2026-11-17 に、このプロジェクトの worktree の Claude Code の transcript から、読み取りだけで採った。

-   **出どころと版。** `claude-runpr-report.jsonl` は、runpr.md に NEXT の行を定めた PR の `/runpr` を、人が Claude Code 2.1.325 の pane で走らせたセッションから採った。`claude-command-report.jsonl` は、0095 の実装計画書の PR を作った `/mkplan` のセッション（2.1.325）から、`claude-compact.jsonl` は 0088 の PR を作った `/runplan` のセッション（2.1.325）から、`claude-auto-continuation.jsonl` は 0091 の `/runplan` のセッション（2.1.325）から、`claude-recap-notification.jsonl` と `claude-synthetic.jsonl` は 0093 の `/runplan` の同じセッション（2.1.326）から採った。
-   **採った範囲。** 各ファイルは、1 つのセッションの transcript の、連続した行の範囲である。`claude-runpr-report.jsonl` は、人が入力した `/runpr` の行から始まる範囲と、最後の報告で終わる範囲の 2 つを、`claude-command-report.jsonl` は、人が入力した `/mkplan` の行と、PR を作る直前から recap までの範囲をつないだ。つないだ間の行（ツールの呼び出しとその結果）は削った。そのほかは 1 つの範囲である。
-   **削った行。** 報告の読み取り部品が読まない種類の行（`api-request`・`api-request-blob`・`file-history-snapshot`・`queue-operation`・`mode`・`ai-title`・`last-prompt`・`session-owner`・`atis-latch`・`tools-used`・`cost-state` など）を削った。
-   **削った項目。** 残した行は、`type` と、次の項目だけに絞った。`cwd`・`gitBranch`・`sessionId`・`uuid`・`timestamp`・`version` などは残していない。
    -   `assistant` の行: `isSidechain` と、`message` の `id`・`model`・`stop_reason`・`content`。`content` の要素は `type` だけにし、`type` が `text` の要素だけ `text` を残した。
    -   `user` の行: `isSidechain`・`isMeta`・`isCompactSummary`・`origin`・`promptSource`・`turnOrigin` と、`message` の `role`・`content`。`content` の `tool_result` の要素は `type` と、`content` を `ok` に置き換えたものだけにした。
    -   `attachment` の行: `isSidechain` と、`attachment` の `type`。`queued_command` のときは、`commandMode`・`origin`・`prompt` も残した。
    -   `system` の行: `subtype`。recap（`away_summary`）は `content` も残した。
    -   `pr-link` の行: `prNumber`・`prUrl`・`prRepository`。
-   **置き換えた本文。** 終わった応答の本文、recap の本文、人が打ったコマンドと `continue` は、採ったまま（上の「置き換えたもの」の規則の置き換えの後）である。それ以外の応答の本文は `Working on it.` に、task-notification・compact の要約・利用の上限の知らせなどの本文は同じ形の短い文に置き換えた。`claude-synthetic.jsonl` の人の入力のうち、貼り付けた文の 1 行は `Short text.` に置き換えた。`claude-command-report.jsonl` の報告の、PR レビューを頼むコメントの本文は、`the review request` に置き換えた。
-   **置き換えた値。** 上の「置き換えたもの」の表のとおりである。PR の記録の `prUrl`・`prRepository` と本文の URL・ブランチ名の GitHub のログイン名は `owner` に、合成の応答の `message.id`（UUID）は同じ表の規則で置き換えた。作業ディレクトリの外の一時ディレクトリのパスはなかった。
