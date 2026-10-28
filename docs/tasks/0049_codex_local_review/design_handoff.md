# 設計への引き継ぎ事項（0049_codex_local_review）

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項である。`02_architecture.md` は、項目ごとに、とった方法か、当てはまらない理由を記録する。

## H-01: レビューの範囲の指定

`codex review` の範囲の指定は `--base <BRANCH>`、`--commit <SHA>`、`--uncommitted` である（`codex review --help`。0.163.0）。CodeRabbit CLI の `--base-commit <BASE>` と同じ「BASE から HEAD までのコミット済みの変更」を指定する方法を決める。候補は、`--base` にコミットの SHA を渡せるか、BASE を指す一時的な参照を作るか（AC-08 のとおり、実行の後に消す）、レビューの指示（PROMPT）で範囲を伝えるか、である。AC-01 の 2 の観測に基づいて選ぶ。`/fixpr` の Phase 4.5 の BASE は `<commitSha>^` であり、`/runplan` の手順 7.7 の BASE はフェーズ群の始まりのコミットなので、ブランチの分岐点ではない範囲も扱える必要がある。

## H-02: 書き込ませない設定と非対話の実行

作業ツリーに書き込ませないこと（AC-08）と、承認を求めて止まらないこと（AC-01 の 4）を、どの設定で実現するかを決める（`-c sandbox_mode=...`、`-c approval_policy=...` など）。利用者の `~/.codex/config.toml` の値に依存せず、コマンドの引数で明示する方が、利用者の設定の変更で振る舞いが変わらない。

## H-03: Claude Code の権限の許可

AC-17 のために `.claude/settings.json` の `permissions.allow` に加えるパターンを決める（今は `Bash(coderabbit review *)`。`.claude/settings.json:4`）。パターンが、共通手順の使う形だけを許し、書き込みやサンドボックスの解除を許す引数（`--dangerously-bypass-approvals-and-sandbox` など）を付けた実行まで許さないようにする。

## H-04: CodeRabbit CLI の失敗の判定

AC-03 の「失敗」を何で判定するかを決める。終了コードで判定できるか、利用の上限に当たっても終了コードが 0 になる場合があるかを確かめる。出力の文面を調べて失敗を推測する方法は、`CLAUDE.md` の「Declare, don't infer」に照らして避けるか、避けられない理由を記録する。

## H-05: Codex CLI の出力の取り込み

出力の形（`codex review` の標準出力、構造化された出力があるか）、スクラッチパッドへの保存、指摘と「指摘なし」と読み取れない出力の区別（AC-09 の 4・6）、時間の上限（AC-09 の 3）、大きさの上限（AC-09 の 5）を決める。出力の区別はエージェントが出力を読んで行う判断であり、Go のコードではないが、`CLAUDE.md` の「Declare, don't infer」と「Reject, don't normalize」に照らして、どう区別するかと、判断できない出力を失敗とすることを手順に明記する。CodeRabbit CLI の `-c CLAUDE.md` に当たる、プロジェクトの規則をレビューに渡す方法（レビューの指示、`AGENTS.md` など）も決める。

## H-06: コミットのメッセージの記録の形式

AC-12 の記録の形式を決める。今の行は `CodeRabbit local review:` と `CodeRabbit local review: not run` であり、`_lib/task-pr-handoff.md` の Part B 手順 3 と `/runplan` の手順 5a が読む。前のセッションが残した今の形の行を読み続けられること（4.4）と、どちらの CLI の指摘か、異系統のレビューを受けていない範囲かを区別できることを両立させる。

## H-07: 共通手順のファイル名

共通手順は `.claude/commands/_lib/coderabbit-local-review.md` にある。Codex CLI の代替を加えた後もこの名前のままにするか、名前を変えて参照をすべて直すかを決める。名前を変える場合は、参照しているコマンドの文書と、文書を読むテスト（`cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go` など）への影響を確かめる。

## H-08: 環境変数の引き継ぎ（任意）

秘密のファイルと環境変数を Codex が読む経路は、残るリスクとして記録するだけにした（`01_requirements.md` の 1 節の決定）。そのうえで、Codex のシェルに環境変数を引き継がない設定（`shell_environment_policy` など）が手間なく使えるなら、採用してよい。採用は要件ではない。
