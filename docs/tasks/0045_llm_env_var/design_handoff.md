# 設計への申し送り：0045 LLM のプロバイダとモデルを 1 つの環境変数で指定する

要件定義（[01_requirements.md](01_requirements.md)）の検討で挙がった、設計で決める事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: 拒否を分類する番兵エラーとエラーの理由の文言

旧変数の拒否（F-003）と、`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の構文の拒否（AC-05）を、既存の `ErrMissing`・`ErrInvalid`（`internal/config/config.go:99-104`）のどちらに分類するか、新しい番兵を加えるかを決める。空の値（AC-04）は、現行のほかの変数と同じく `ErrMissing` が自然である。エラーの理由（`VarError` の `Reason`）は固定の定数から選ぶ（`config.go:72-92`）。構文の誤りの種類（`/` がない、モデル ID が空、プロバイダが不正）ごとにエラーの理由を分けるかどうかも、ここで決める。旧変数のエラーの理由は、代わりに使う新しい変数の名前を含む（AC-14）。

## H-02: 旧変数の検査を置く場所

旧変数の検査は、`LoadFor` が受け取る `Need` によらず行い（`Need{}` を含む。AC-12）、`LoadServer` でも行い（AC-13）、`LoadHost`・`LoadHostHome` では行わない（AC-17）。この条件を満たす置き場所と、ほかの拒否とまとめて報告する方法（AC-15・AC-20）を決める。CLI が `--slack` の `SLACK_WEBHOOK_URL` の未設定を設定の読み込みの後に報告する現行の順序（`cmd/yt2column/run.go:336-340`）は、AC-20 が変えないとしている。まとめる対象を広げる場合は、要件の変更として扱う。

## H-03: 前後の空白を拒否する層

AC-06 は、前後に空白があるモデル ID を、LLM の API に何も送らずに止めることを求める。現行では、アダプタの `New`（`internal/llm/claude/claude.go:60-62`・`internal/llm/deepseek/deepseek.go:60-62`）が拒否し、CLI もサーバも構築の失敗として終了コード `2` で止まる。設定の段階でも拒否するか、アダプタに任せるかを決める。設定の段階で拒否する場合は、メッセージが `YT2COLUMN_LLM` などの変数を名指しするようになる。ただし、`--no-review` のように、その段階のクライアントを作らない実行では止めないという AC-06 の振る舞いを保つ必要がある。現行の CLI がクライアントを作るのは、その段階を実行するときだけである（`cmd/yt2column/run.go:577-589`）。なお、要件定義の際に「設定で拒否するモデル ID は空の値だけ」と決めている（01_requirements.md §1）。設定の段階で拒否する案を採るなら、その決定の変更として扱う。

## H-04: 不要になる仕組みの整理

モデルだけを設定した状態がなくなるので、`settingOrigin`・`stageOrigins`・`modelWithoutProvider`・`loadModelOptional`・`defaultModel` と、それらが要る分岐（`config.go:388-610`）を整理する。継承（AC-07）と、生成の設定を検査せずに検証・推敲の設定を検査する実行の振る舞い（AC-10）を保ったまま、どこまで簡単にできるかを決める。`LLMConfig` のアクセサ（`Provider`・`Model`・`ReviewProvider`・`ReviewModel`）の形を変えるかどうかも決める。

## H-05: テストとテスト補助の旧変数

`cmd/yt2column`・`cmd/yt2column-server`・`cmd/yt2column-nmhost`・`internal/llm/provider` のテストとテスト補助は、旧変数で環境を組み立てている（`cmd/yt2column/test_helpers_integration.go:162-168`・`cmd/yt2column-server/test_helpers.go:125-128` など）。新しい変数への置き換えと、旧変数を拒否することを確かめるテストの組み立てを決める。

## H-06: 文書を検査するガードテスト

AC-27 のガードテスト（README・`project_overview.md`・`production_profile.md` の環境変数の名前と既定値を検査するもの）の追従の方法を決める。CLAUDE.md「Testing Strategy」の文書テストの規則（値と構造を検査し、文章を検査しない。古い名前がないことを主張しない）に従う。本番の構成のガード（`cmd/yt2column/docs_test.go:803-868` の `checkProfileTable`・`productionProfileVars`）は、プロバイダとモデルを別々の変数として文書の表と突き合わせている。名前を変えるだけでは済まず、組の値を検査する形に作り直す必要がある。AC-26 の「旧変数を現行の設定として説明しない」は文章の内容なので、PR のレビューで読んで確かめる（`manual`）。

## H-07: モデル ID を要求本文の外に出さないこと

4.2 は、モデル ID を要求本文（JSON）の値としてだけ送り、URL には入れないことを求める。現行の 2 つのアダプタは、これを満たしている。将来、URL のパスにモデルを入れる API のプロバイダを追加するときは、モデル ID の中の `/` や `..` がパスを変えないようにする必要がある。本タスクの範囲ではないが、設計でこの制約をどこに書き残すかを決める。
