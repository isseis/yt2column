# 設計への申し送り：0045 LLM のプロバイダとモデルを 1 つの環境変数で指定する

要件定義（[01_requirements.md](01_requirements.md)）の検討で挙がった、設計で決める事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: 拒否を分類する番兵エラーと理由の文言

旧変数の拒否（F-003）と、`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の構文の拒否（AC-05）を、既存の `ErrMissing`・`ErrInvalid`（`internal/config/config.go:99-104`）のどちらに分類するか、新しい番兵を加えるかを決める。空の値（AC-04）は、現行のほかの変数と同じく `ErrMissing` が自然である。拒否の理由は固定の定数から選ぶ（`config.go:72-92`）。構文の誤りの種類（`/` がない、モデル ID が空、プロバイダが不正）ごとに理由を分けるかどうかも、ここで決める。旧変数の理由は、代わりに使う新しい変数の名前を含む（AC-14）。

## H-02: 旧変数の検査を置く場所

旧変数の検査は、`LoadFor` が受け取る `Need` によらず行い（`Need{}` を含む。AC-12）、`LoadServer` でも行い（AC-13）、`LoadHost`・`LoadHostHome` では行わない（AC-17）。この条件を満たす置き場所と、ほかの拒否とまとめて報告する方法（AC-15・AC-20）を決める。

## H-03: 前後の空白を拒否する層

AC-06 は、前後に空白があるモデル ID を、LLM の API に何も送らずに止めることを求める。現行では、アダプタの `New`（`internal/llm/claude/claude.go:60-62`・`internal/llm/deepseek/deepseek.go:60-62`）が拒否し、CLI もサーバも構築の失敗として終了コード `2` で止まる。設定の段階でも拒否するか、アダプタに任せるかを決める。設定の段階で拒否する場合は、メッセージが `YT2COLUMN_LLM` などの変数を名指しするようになる。

## H-04: 不要になる仕組みの整理

モデルだけを設定した状態がなくなるので、`settingOrigin`・`stageOrigins`・`modelWithoutProvider`・`loadModelOptional`・`defaultModel` と、それらが要る分岐（`config.go:388-610`）を整理する。継承（AC-07）と、検証・推敲だけを実行する再開の実行の振る舞い（AC-10）を保ったまま、どこまで簡単にできるかを決める。`LLMConfig` のアクセサ（`Provider`・`Model`・`ReviewProvider`・`ReviewModel`）の形を変えるかどうかも決める。

## H-05: テストとテスト補助の旧変数

`cmd/yt2column`・`cmd/yt2column-server`・`cmd/yt2column-nmhost`・`internal/llm/provider` のテストとテスト補助は、旧変数で環境を組み立てている（`cmd/yt2column/test_helpers_integration.go:162-168`・`cmd/yt2column-server/test_helpers.go:125-128` など）。新しい変数への置き換えと、旧変数を拒否することを確かめるテストの組み立てを決める。

## H-06: 文書を検査するガードテスト

AC-27 のガードテスト（README・`project_overview.md`・`production_profile.md` の環境変数の名前と既定値を検査するもの）の追従の方法を決める。CLAUDE.md「Testing Strategy」の文書テストの規則（値と構造を検査し、文章を検査しない。古い名前がないことを主張しない）に従う。AC-26 の「旧変数を現行の設定として説明しない」は文章の内容なので、PR のレビューで読んで確かめる（`manual`）。
