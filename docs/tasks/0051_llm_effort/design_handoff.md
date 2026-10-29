# 設計への申し送り：0051 `YT2COLUMN_LLM` に effort を含め、`YT2COLUMN_CLAUDE_EFFORT` を廃止する

要件定義（[01_requirements.md](01_requirements.md)）の検討で挙がった、設計で決める事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: 拒否を分類する番兵エラーとエラーの理由の文言

effort の部分の拒否（AC-05・AC-06・AC-10）と `YT2COLUMN_CLAUDE_EFFORT` の拒否（AC-20）を、既存の `ErrMissing`・`ErrInvalid`（`internal/config/config.go:125-130`）のどちらに分類するかを決める。effort の部分が空の値（`claude/claude-opus-5-5+`）は、変数そのものは空でないので、空の変数の `ErrMissing` と揃えるかどうかを決める必要がある。エラーの理由は固定の定数から選ぶ（`config.go:77-101`）。AC-06 は、effort の部分が空か不正な値と、`+` が 2 つ以上の値とで、示す内容を分けることを求める。モデル ID が空の値（`claude/+high`）は、今の `reasonLLMForm`（`config.go:82`）を使うか、別の理由にするかも決める。AC-06 のメッセージが示す effort の値の一覧は、`claudeparam` の値の定義（`internal/llm/claudeparam/claudeparam.go:26-34`）から作り、`config` に値を書き写さない（01_requirements.md 4.5）。

## H-02: effort を設定のどこに持たせるか

今の `LLMConfig` は effort を 1 つだけ持ち（`config.go:170`）、`provider.newClient` が生成と検証・推敲のどちらにも同じ値を渡す（`internal/llm/provider/provider.go:59-86`）。effort を段階ごとの選択（`llmSelection`。`config.go:157-160`）に持たせるか、別の形にするかを決める。`ClaudeEffort` のアクセサ（`config.go:215-220`）を段階ごとのアクセサに置き換える形も決める。後の OpenAI のプロバイダ（2.3）が effort の値の一覧を足すだけで載せられるようにすることが目的だが（4.5）、まだないプロバイダのための抽象を先回りして作らない（CLAUDE.md「YAGNI」）。プロバイダの API の要求の形は、そのアダプタのパッケージにとどめる（CLAUDE.md「Provider details stay in their adapter package」）。effort の型は、今と同じく `claudeparam` を `config` と `claude` が共有する形を基本とする。

## H-03: 値の分解の順序と、プロバイダの変数の検査

今の `parseLLM`（`config.go:509-530`）は、最初の `/` で区切り、残りをそのままモデル ID とする。effort の部分を解釈する位置と順序を決める。AC-25 は、プロバイダの部分が正しく effort の部分だけが不正な値でも、そのプロバイダの変数を検査することを求める。今の「モデル ID だけが空の値でもプロバイダを返す」契約（`config.go:506-508`）を、effort の部分の拒否にも広げる形が考えられる。DeepSeek に `+` があるかどうかの判定（AC-10）は、プロバイダが決まった後に、プロバイダごとの `switch` で行う形が CLAUDE.md「Declare, don't infer」に沿う。

## H-04: 既定の effort の表し方

既定の値 `defaultLLMValue`（`config.go:67`）に `+high` を含めるか、`+` のない値の既定の effort をプロバイダごとに持つか（今の `defaultClaudeEffort`。`config.go:592-596`）を決める。AC-03 は、`+` のない値（未設定の場合の既定値を含む）の effort をプロバイダの既定にすることを求めるので、後者の仕組みはどちらにしても要る。`docs/dev/guides/claude_output_limit.md:10` は既定の effort の所在を `defaultClaudeEffort` の名前で示しているので、名前を変える場合は合わせて直す（AC-32）。

## H-05: 旧変数の一覧への追加

`YT2COLUMN_CLAUDE_EFFORT` を、旧変数の一覧（`retiredLLMVars`。`config.go:111-118`）に加えるかどうかを決める。今の一覧の要素は `reasonRetiredLLM` か `reasonRetiredReviewLLM` を持ち、それぞれ代わりの変数を 1 つ示す（`config.go:92-100`）。AC-20 は 2 つの変数を示すので、新しい理由が要る。`claudeEffortEnv`（`config.go:36`）の定数と、`config.ClaudeEffortEnvForTest`（`internal/config/test_helpers.go`）の扱いも決める。

## H-06: テストとテスト補助の `YT2COLUMN_CLAUDE_EFFORT`

`YT2COLUMN_CLAUDE_EFFORT` で環境を組み立てるテストがある（`cmd/yt2column/run_test.go:635`・`:1985-2007`、`cmd/yt2column-server/run_test.go:334`、`internal/llm/provider/provider_test.go:103`・`:266`・`:349-359`、`cmd/yt2column/docs_test.go:440-570`）。effort を値に含める形への置き換えを決める。`cmd/yt2column/integration_env_test.go:76` は、両方の段階が DeepSeek の実行が `YT2COLUMN_CLAUDE_EFFORT` を読まないことを確かめている。本タスクの後は、この変数が設定されているかどうかを、どの実行でも調べる（01_requirements.md 5 章）。このテストの確かめる内容を、それに合わせて変える必要がある。

## H-07: 文書を検査するガードテスト

AC-33 のガードテストの追従の方法を決める。本番の構成のガード（`cmd/yt2column/docs_test.go:440-570` のあたり）は、今は `YT2COLUMN_LLM` と `YT2COLUMN_CLAUDE_EFFORT` を別々の変数として文書の設定のブロックから読み、コードの既定と突き合わせている。設定のブロックが `YT2COLUMN_LLM=claude/claude-opus-5-5+high` の 1 行になる（AC-29）ので、値の effort の部分を、設定を読み込む本物の処理で解釈してコードの既定と比べる形が考えられる。CLAUDE.md「Testing Strategy」の文書テストの規則（値と構造を検査し、文章を検査しない。古い名前がないことを主張しない）に従う。AC-27・AC-28・AC-30〜AC-32 の文章の内容は、PR のレビューで読んで確かめる（`manual`）。
