# 設計への申し送り：既定の LLM を Claude Opus 5.5 にする

## 本書の位置づけ

[01_requirements.md](01_requirements.md) の作成とレビューの過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計・実装の段階で決める、または注意すべき事項をまとめる。

-   `02_architecture.md` は、本書の各項目について、採用した手段または扱わない理由を記録する。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   本書は承認対象の文書ではない。設計の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | F-001 | プロバイダの既定に依存するテストと統合テスト | AC-01・AC-15 |
| H-02 | F-002・F-003 | 既定のモデル名の置き場所 | AC-04・AC-08・AC-10・AC-19 |
| H-03 | F-001〜F-003 | 既定を当てはめる処理の組み立て | AC-01〜AC-12 |
| H-04 | F-004 | #171 の effort の既定との組み合わせ | AC-13・AC-14 |
| H-05 | F-005 | 文書を読むテストと、#488 の文書との衝突 | AC-16〜AC-19 |

## H-01: プロバイダの既定に依存するテストと統合テスト

-   `YT2COLUMN_LLM_PROVIDER` を指定せずに `deepseek` を前提とするテストの環境（`cmd/yt2column/run_test.go`・`internal/config/config_test.go` の表、`cmd/yt2column/test_helpers_integration.go` が組み立てる CLI の統合テストの環境など）は、既定が `claude` になると振る舞いが変わる。どのテストが既定に依存しているかを洗い出し、DeepSeek を前提とするものは `YT2COLUMN_LLM_PROVIDER=deepseek` を明示する。
-   CLI の統合テスト（`make test-integration-cli`・`make test-integration-cli-slack`）は DeepSeek の API を呼ぶ前提で、モデルの既定は `deepseek-flash` である（README「CLI integration test」）。本番の既定が変わっても、統合テストの送り先が意図せず Anthropic に変わらないようにする。

## H-02: 既定のモデル名の置き場所

-   既定のモデル名 `claude-opus-5-5` を、コードの 1 か所に定数として持つ（要件書 §4.5）。候補は `internal/config`（既定を当てはめる場所）か `internal/llm/claudeparam`（Claude の設定値の型を `config` と `claude` が共有する場所）である。
-   `internal/pricing` の単価の表にも同じ文字列がある。単価の表は応答が報告したモデル名を引く参照表であり（`project_overview.md`「前提・制約」）、既定とは役割が違う。2 つを 1 つの定数にまとめるかは、役割の違いを踏まえて決める。
-   文書（README・`project_overview.md`）に書いた既定のモデル名がコードの定数と一致することを、文書の値を読むテストで確かめるかを決める（`CLAUDE.md`「Document tests check values ...」は値の比較を許す）。

## H-03: 既定を当てはめる処理の組み立て

-   いまの `parseProvider` は、変数が存在しないときに `ProviderDeepSeek` を返す（`internal/config/config.go:437-451`）。`YT2COLUMN_REVIEW_PROVIDER` は存在しない場合を呼び出し側で扱う。`loadProviderOptional`（同 `:397-410`）も `ProviderDeepSeek` を直接書いている。既定を 2 か所に書かないようにする。
-   モデルの既定は、プロバイダが確定した後に、`Provider` の値で `switch` して決める。`default` は既定なし（拒否）とし、新しいプロバイダが加わったときに既定を黙って当てはめないようにする（`CLAUDE.md`「Declare, don't infer」）。
-   生成を実行する実行（`loadModel`）、生成を実行しない実行（`loadModelOptional` と `loadReview` の受け継ぎ）、検証・推敲のプロバイダだけを指定した場合（`loadReview`）の 3 つの経路で、同じ規則が当てはまるようにする。`YT2COLUMN_LLM_PROVIDER` が不正なときにモデルの既定を当てはめないこと（AC-07）、`YT2COLUMN_REVIEW_PROVIDER` が不正なときに検証・推敲のモデルの既定を当てはめないこと（AC-12）に注意する。
-   `LLMConfig` のアクセサの文書コメント（`Model` は空になりうる、など）を、既定の導入に合わせて見直す。

## H-04: #171 の effort の既定との組み合わせ

-   AC-14 は、#171 の effort の既定が入っていることを前提とする。#171 の実装が先に main に入る（要件書 §5）ので、本タスクの設計は #171 の設計（effort の既定の置き場所、既定を当てはめる関数）に合わせ、既定を当てはめる処理の形をそろえる。
-   #171 の設計がまだ決まっていない時点で本タスクの設計を進める場合は、effort の既定に関わる部分を #171 に委ね、その旨を記録する。

## H-05: 文書を読むテストと、#488 の文書との衝突

-   `project_overview.md` の環境変数の名前などを読む Go のテストがある（`CLAUDE.md`「Development Notes」）。表の「未設定のとき」の列を書き換えるときに、これらのテストを壊さないことを確かめる。README を変えるので、`make ext-test`（README の表と節を読む拡張のテスト）も実行する。
-   #488（タスク 0040、PR #489）も README と `project_overview.md` を書き直す。後にマージするほうが、先にマージされた内容に合わせる（要件書 §5）。実装計画で、#488 の進み具合を確かめてから文書の変更を書く手順を置く。
