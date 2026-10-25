# 設計への申し送り：既定の LLM を Claude Opus 5.5 にする

## 本書の位置づけ

[01_requirements.md](01_requirements.md) の作成とレビューの過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計・実装の段階で決める、または注意すべき事項をまとめる。

-   `02_architecture.md` は、本書の各項目について、採用した手段または扱わない理由を記録する。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   本書は承認対象の文書ではない。設計の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | F-001・F-004・F-005 | プロバイダの既定に依存するテストと統合テスト | AC-01・AC-16・AC-24 |
| H-02 | F-002・F-003・F-006 | 既定のモデル名の置き場所 | AC-03・AC-08〜AC-10・AC-25・AC-29 |
| H-03 | F-001〜F-004 | 既定と拒否を当てはめる処理の組み立て | AC-01〜AC-20 |
| H-04 | F-004 | 拒否の理由の文言 | AC-19 |
| H-05 | F-005 | #171 の effort の既定との組み合わせ | AC-21・AC-22 |
| H-06 | F-006 | 文書を読むテストと、#488 の文書との衝突 | AC-25〜AC-29 |

## H-01: プロバイダの既定に依存するテストと統合テスト

-   `YT2COLUMN_LLM_PROVIDER` を設定せずに `deepseek` を前提とするテストの環境（`internal/config/config_test.go`・`cmd/yt2column/run_test.go` の表、localhost サーバのテストの環境を組み立てる `cmd/yt2column-server/test_helpers.go` の `newServerEnv` など）は、既定が `claude` になり、F-004 の拒否が加わると、振る舞いが変わる。どのテストが既定に依存しているかを洗い出し、DeepSeek を前提とするものは `YT2COLUMN_LLM_PROVIDER=deepseek` を明示する。
-   CLI の統合テストの環境は、`cmd/yt2column/integration_test.go:144-149` の閉じた map で組み立てていて、`YT2COLUMN_MODEL` と `DEEPSEEK_API_KEY` はあるが、プロバイダの変数はない。変更の後は F-004 の拒否で失敗する（Anthropic には送らない）。AC-24 を満たすよう、`YT2COLUMN_LLM_PROVIDER=deepseek` を加える。`integration_slack_test.go` は同じ環境を使うかを確かめ、使わなければ同じく直す。

## H-02: 既定のモデル名の置き場所

-   既定のモデル名 `claude-opus-5-5` を、コードの 1 か所に定数として持つ（要件書 §4.5）。候補は、既定を当てはめる `internal/config` か、Claude の設定値の型を `config` と `claude` が共有する `internal/llm/claudeparam` である。
-   `internal/pricing` の単価の表にも同じ文字列がある。単価の表は、応答が報告したモデル名を引く参照表であり（`project_overview.md`「前提・制約」）、既定とは役割が違う。2 つを 1 つの定数にまとめるかは、役割の違いを踏まえて決める。
-   文書（README・`project_overview.md`）に書いた既定のモデル名がコードの定数と一致することを、文書の値を読むテストで確かめるかを決める（`CLAUDE.md`「Document tests check values ...」は値の比較を許す）。

## H-03: 既定と拒否を当てはめる処理の組み立て

-   いまの `parseProvider` は、変数が存在しないときに `ProviderDeepSeek` を返す（`internal/config/config.go:437-450`）。`YT2COLUMN_REVIEW_PROVIDER` の未設定は呼び出し側で扱う。`loadProviderOptional`（同 `:399-411`）も `ProviderDeepSeek` を直接書いている。既定を 2 か所に書かないようにする。
-   F-004 の拒否には、プロバイダが「設定した値」か「既定で決まった値」かを知る必要がある。いまの `LLMConfig` はプロバイダの値だけを持つ。区別を型で持つか、読み込みの途中の値として持つかを決める（`CLAUDE.md`「Declare, don't infer」）。
-   モデルの既定は、プロバイダが確定した後に、`Provider` の値で `switch` して決める。`default` は既定なし（拒否）とし、新しいプロバイダが加わったときに既定を黙って当てはめないようにする。
-   3 つの経路で同じ規則が当てはまるようにする。生成を実行する実行（`loadModel`）、検証・推敲だけを実行する実行（`loadModelOptional` と `loadReview` の受け継ぎ）、`YT2COLUMN_REVIEW_PROVIDER` だけを設定した場合（`loadReview`）である。AC-06・AC-13 の「既定を当てはめない」場合と、AC-19 の「拒否を 1 件だけ報告する」に注意する。
-   `LLMConfig` のアクセサの文書コメント（`Model` は空になりうる、など）と、`loadReview` の「A set provider requires a set model」などのコメントを、既定の導入に合わせて見直す。

## H-04: 拒否の理由の文言

-   AC-19 の拒否の理由は、空の `YT2COLUMN_LLM_PROVIDER` の理由（`reasonUnset`）とは別の、固定の定数（`internal/config/config.go:59-73` の `reason...`）として足す。文言は設計で決める。例: `must be set when YT2COLUMN_MODEL or YT2COLUMN_REVIEW_MODEL is set`。
-   番兵は `ErrMissing` とする（要件書 AC-16〜AC-18）。テストは `errors.Is` と `*VarError` の `Name` で確かめる。AC-02 の空の値の拒否と区別するときは、`Reason` を理由の定数と比べる（文言の文字列を直接書かない。`CLAUDE.md`「Error Testing」）。

## H-05: #171 の effort の既定との組み合わせ

-   AC-22 は、#171 の effort の既定が入っていることを前提とする。#171 の実装が先に main に入る（要件書 §5）ので、本タスクの設計は #171 の設計（effort の既定の置き場所、既定を当てはめる関数）に合わせ、既定を当てはめる処理の形をそろえる。
-   #171 の設計がまだ決まっていない時点で本タスクの設計を進める場合は、effort の既定に関わる部分を #171 に委ね、その旨を記録する。
-   AC-21 で報告される拒否には、#171 の前なら `YT2COLUMN_CLAUDE_EFFORT` の拒否も加わる。#171 の後には加わらない。

## H-06: 文書を読むテストと、#488 の文書との衝突

-   `project_overview.md` の環境変数の名前などを読む Go のテストがある（`CLAUDE.md`「Development Notes」）。表を書き換えるときに、これらのテストを壊さないことを確かめる。README を変えるので、`make ext-test`（README の表と節を読む拡張のテスト）も実行する。
-   #488（タスク 0040、PR #489）も README と `project_overview.md` を書き直し、#488 の実装は本タスクの実装の後にマージする（要件書 §5）。本タスクの実装の時点で #488 の文書がすでに main にある場合は、その文書に合わせて書く。
