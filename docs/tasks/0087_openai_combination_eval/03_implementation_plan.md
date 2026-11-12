# 実装計画書：Claude と OpenAI の組み合わせの評価と、本番の構成の見直し

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的と実装方針

[要件定義書](01_requirements.md)（以下「要件」）の AC-01〜AC-30 を、[設計書](02_architecture.md)（以下「設計」）に従って実装する。評価の手順と記録の形は設計 §3.1〜§3.8 で決まっている。本書は、作るファイル、実行する順序、各 AC を確かめる検証を定める。

- フェーズは設計 §8 の 6 つの優先順位にそのまま対応させる。実 API を呼ぶのは、フェーズ 2 で一括の承認を受けた後のフェーズ 3・4 だけである。
- 評価の手順は一度きりなので、`make test` に入る恒久的なテストは作らない（設計 §7.1）。`_eval/` のテストとスクリプトは、パスを指定して実行し、出力を評価記録または本書 §7 に記録する。
- 評価記録 `04_evaluation.md` には、主セッションが集計のツールの値を日本語の表に書き写す（設計 §2.1・§3.7）。書き写した値が集計の値と一致することは、集計のツールの照合（`check-record`）で確かめる。
- フェーズ 6（本番の構成の変更）は、フェーズ 5 の判断で本番の構成を変えると決めた場合だけ行う。

### 1.2. 計画の際に決めたこと（2026-11-11）

1. **`_eval/` は、シェル（zsh）と Go で書く。** コマンドを順に呼ぶだけのスクリプト（1 件の実行の `run.sh`、ハッシュの一覧、やり直しの移動、確認の実行）は zsh で書く。判定と計算（起動の判定、集計、照合、採点の素材、見積もりと上限、単価）は、`_eval/evaltool/` の 1 つの `main` パッケージに Go で書き、サブコマンドで分ける。
    - **理由:** 1 件の実行は 0018 と同じく zsh の `time` で測り、基準の構成の所要時間と測り方をそろえる。起動の判定は 2 つ以上の条件を持ち、入力を与えて確かめるテストが要るので Go で書く（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の方針にも沿う）。上限・補助の推定コスト・単価は `internal/pricing` の `Estimate` を再利用し、単価を写さない。zsh の `time` の出力は既定の `TIMEFMT` のまま使い、0018 のログと同じ形にする。
    - レビュアーが推奨案を選んだ。
    - **ADR の判断:** 作らない。このタスクの中で閉じる道具の選択であるためである。
2. **PR は 3 本に分ける。** PR-1 は道具（ステップ 1-1〜1-5）、PR-2 はビルドの固定と実行の前の確認から結論まで（ステップ 1-6・1-7 とフェーズ 2〜5）、PR-3 は本番の構成の変更（フェーズ 6。変える場合だけ）である。
    - **理由:** 費用を払う前に、起動と上限の道具を PR-1 でレビューできる。評価の記録は 1 本の PR にまとめる。1 段目の記録は 2 段目の前に PR でレビューされない。ただし、1 段目の判定は 2 段目の前に人がセッションの中で確認する（設計 §3.6）ので、この点は許容する。
    - レビュアーが推奨案を選んだ。
    - **ADR の判断:** 作らない。このタスクの中で閉じる進め方であるためである。

次の項目は、承認済みの文書の範囲で本計画が決めたもので、人には尋ねていない。いずれも ADR は作らない。このタスクの中で閉じる手順であるためである。

3. **ビルドの固定、ハッシュの一覧とバックアップ、実行の前の確認（設計 §3.4）は、PR-1 のマージの後に行う（ステップ 1-6・1-7）。** PR-1 のレビューで `_eval/` が直ると、その前に記録した確認が古くなるためである。設計 §8 の 1（準備）に属する作業なので、フェーズ 1 のステップとして置き、PR-2 に含める。
4. **照合（`check-record`）は、表の見出しではなく値を比べる。** 評価記録の表には、表ごとに名前を付けた印（HTML のコメント）を置く。照合は、印の中の表の行を記事などの鍵で集計の値と対応させ、値のセル（数値・識別子・判定）だけを比べる。見出しと注は日本語で書き、比べない。
5. **構成の変数の表は `run.sh` だけに置き、Go のツールは `run.sh` に表を出させて読む**（設計 §3.1 の「この表を持つのは実行のスクリプトだけ」）。Go のツールは、起動の判定（同じ OpenAI のモデルの間隔）と上限の計算に使うモデルを、この出力から得る。料金表にないモデル（モデル ID の打ち間違いを含む）があれば、見積もりと起動は止まる。
6. **見積もりと上限の表（設計 §3.10）は、入力のファイルから `estimate` が作って `_eval/` に置き、`launch` はその表を読む。** 表は生成したものをコミットし、入力のファイルとともにレビューする。`launch` と `estimate` は同じ上限の関数を使う。
7. **起動の判定は、設計 §3.5 のとおり、`cost-total: run` の行がない実行に上限の額を数える。評価記録の使った額の合計（AC-20）からは、人が支払い前と確かめた実行だけを除く。**
    - **理由:** CLI は、LLM を 1 度も呼ばずに終わった実行（設定の誤り、入力の拒否）でも、要約を書かずに止まった実行（panic、`kill -9`）でもトークンの行を書かない（[cli.md](../../dev/guides/cli.md)「トークンの使用量」）。panic の終了コードは設定の誤りと同じ `2` になりうるので、ログからは両者を区別できない。そのため、起動の判定は閉じる側に倒し、支払い前の実行も上限で数える。
    - `aggregate` は、終了コードがあり `origin` が `run` の `tokens:` の行がない実行を「支払い前の候補」として別の表に出す。主セッションが標準エラーを読んで番兵を記録し、人が支払い前と確かめた実行を、一覧のファイル（`0087/prepayment.tsv`）に書く。`aggregate` はこの一覧の実行だけを、評価記録の合計とやり直しの回数から除き、除いた実行とその上限の額を別に出す。`launch` はこの一覧を読まない。
8. **承認の記録 `0087/approval` は追記だけにする。** 最初の承認の時刻と上限を書き、改めての承認は時刻と新しい上限を追記する。`launch` は最後の上限を使い、承認より前に始まった実行の一覧（AC-19）は最初の承認の時刻と比べる。
9. **採点のサブエージェントは、`_eval/scorer-agent.md` の定義で起動する。** 定義は、道具を `tools: Read` に限り、モデルを `claude-opus-5-5` にする。`tools` が許可リストであること、`model` が完全なモデル ID を受け付けることは、Claude Code の文書（https://code.claude.com/docs/en/sub-agents.md 、2026-11-11 に確認）で確かめた。定義は人の許可を得てユーザーのエージェントのディレクトリ（`~/.claude/agents/`）に写し、2 段目の採点の後に消す。このディレクトリは現時点で存在しない（2026-11-11 に確認）。新しいディレクトリに最初のファイルを置いた場合は再起動が要る（同じ文書）ので、写した後にセッションを開き直す。
10. **`launch` は、1 段目の判定のファイルがそろい人の確認の印があるまで、`B-*` を起動しない**（要件 AC-08 の順序）。

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、設計 §1.4 に扱いが記録されている。

### 1.3. 既存コード調査結果

調査の基準はコミット `986449c6`（`origin/main`）である。下の行番号はこのコミットを指す。

| 対象 | 既存のものと、本タスクでの扱い |
|---|---|
| CLI の料金と使用量の行 | `tokens:`・`cost:`・`cost-total:` の行の形は [cli.md](../../dev/guides/cli.md) の「トークンの使用量」「推定の料金」（`docs/dev/guides/cli.md:76-77`・`:162-163`）が定める。トークンの行を書かない実行は料金の行も書かない（同「推定の料金」）。集計のツールはこの形を読む。エラーの番兵は文面から推測しない（設計 §3.7） |
| 単価と推定コスト | `internal/pricing/pricing.go:104` の `Estimate(model, usage, callTime)` は公開されている。単価の表 `rateTable`（`internal/pricing/rates.go:87`）は公開されていない。OpenAI の 2 つのモデルは、入力が 272,000 トークンを超えると別の単価になり（`schemePromptTier`）、料金の条件 `llm.PricingContextStandard30m` を要する（`rates.go:134`・`:143`）。使用量は `llm.KnownUsageWithPricingContext`（`internal/llm/usage.go:86`）で作る |
| 再開と必要な設定 | `cmd/yt2column/run.go:424-436` の `resumeNeed`。`--after generation` は生成の設定を、`--after shorten` は検証・推敲の設定だけを、`--after review` は何も要さない。設計 §3.1・§3.2・§3.4 の前提と一致する |
| 機械的な判定 | `docs/tasks/0008_column_prompt/check_article.py`（Python）。記事のフロントマターの形に依存するので、固定した CLI が書いた記事を読めることを実行の前の確認で確かめる（ステップ 1-7） |
| 0018 のデータ | ログは `~/yt2column-eval/0018/log/{B3g,B3,C03}/`、失敗した実行は `0018/failed/B3/roNZ-rLxhUE-1-f1/` にある。0018 の `.when` は `start`・`end`・`exit=` の行だけで（C03 は `after` の行も持つ）、0087 の `.when`（設計 §3.1）より項目が少ない。`time` の行は zsh の既定の形（`… 41.366 total` や `… 2:15.45 total`）である。集計の正しさを確かめる既知の値は [0018 評価記録](../0018_article_quality/02_evaluation.md) §4.8 の表（B3 の 10 件の合計 3.615544 USD、平均 0.361554、最大 0.610696、所要時間の平均 1:53.4・最大 3:06.2。C03 の合計 3.572464 USD、所要時間の平均 1:52.1・最大 3:05.2）である |
| 0077 の実測 | 見積もりと上限の入力のうち OpenAI のトークン数は、[0077 設計書](../0077_openai_llm_client/02_architecture.md) §1.4 の表の `roNZ-rLxhUE` の行である。Claude の検証・推敲の入力は 0077 にないので、B3 の `roNZ-rLxhUE-*` の検証・推敲の `input` を、記事の分を差し引かずに使う |
| リポジトリ内の Go の前例 | `docs/tasks/0018_article_quality/_material_check/main.go`。`go list ./...` は `_` で始まるディレクトリを含めない（`986449c6` で `go list ./... \| grep -c docs` が `0`）。`...` のパターンは、`_` のディレクトリが起点でも何も含めない（`go test ./docs/tasks/0018_article_quality/_material_check/...` は「matched no packages」で終了コード `1`）。パッケージのパスをそのまま書けば対象になる（`go list ./docs/tasks/0018_article_quality/_material_check` が成功）。`make lint` も `_eval/` を見ない |
| 既定の LLM（F-007） | `internal/config/config.go:69-81` の `defaultProvider`・`defaultClaudeModel`・`defaultLLMValue`。継承は `loadLLM`（`config.go:508-537`）の `cfg.review = generation`（`:533`）。キーの検査は `loadProviderVars`（`:645`）。テスト用の関数は `internal/config/test_helpers.go` の `DefaultLLMValueForTest`・`DefaultClaudeModelForTest` で、使うのは `cmd/yt2column/docs_test.go:312` と `cmd/yt2column/run_test.go:1583` である。`defaultClaudeModel` を直接使うテストは `internal/config/config_test.go` の 1368・1407・1419・1438・1447・1543-1544・1564-1565・2195・2307 行である |
| 既定に依存するテスト（F-007） | 設計 §3.9.3 のテストはすべて `986449c6` にある（`config_test.go` の `TestLoadDefaults`:102・`TestLoadClaudeEffortDefault`:304・`TestLoadGenerationDefaults`:1358・`TestLoadReviewDefaults`:1399・`TestLoadDefaultProviderVariables`:1488・`TestLoadServerLLMDefaults`:1529・`TestLoadProductionDefaults`:1551・`TestLoadInheritsEffort`:1773、`serverfile_test.go:325` の `TestServerFileTemplate`、`run_test.go:1564` の `TestRunDefaultLLMConfig`、`docs_test.go:310` の `TestLLMDefaultsDocsContract`・`:699` の `TestProductionProfileDocsContract`）。設計 §3.9.3 にないもので、既定のキー（`ANTHROPIC_API_KEY`）だけで起動することを前提にしうるのは、`cmd/yt2column-server/run_test.go` の `TestStartupFailures`:47・`TestServerIgnoresEnvSettings`:488・`TestConfigInitThenStart`:1390 と、`cmd/yt2column/run_test.go:1541` の `TestRunClaudeConfigurationErrorsOneLineEach` である |
| 既定に依存する文書（F-007） | `claude-opus-5-5`・`YT2COLUMN_REVIEW_LLM`・`本番の構成`・`ANTHROPIC_API_KEY` を `docs/tasks/` の外で検索した（`986449c6`）。設計 §3.9.2 の表にないもので変えうるのは、`docs/dev/adr/0025-claude-output-limit.md`（本番の出力の上限を 40,000 と記録する決定。生成が OpenAI になる C・B で変わる）、`docs/dev/adr/0041-openai-api.md:43`（本番の構成の変更は後続の評価で決めるという記述）、`docs/dev/guides/claude_output_limit.md:7`（本番の構成の工程が変わったら測り直す条件）、`docs/dev/guides/development.md:32`（開発用のサーバの設定が `ANTHROPIC_API_KEY` だけを書く）である。`docs/translation_glossary.md`・`testdata/README.md:123`・`prompts/README.md:9`・`docs/dev/adr/0035-language-policy.md` は既定の値を書いていない。コードの検索結果のうち、アダプタと料金のテスト（`internal/llm/claude/`・`internal/pricing/`・`internal/llm/provider/provider_test.go`・`cmd/yt2column/cost_test.go`）のモデル名は既定ではない |

### 1.4. 実装の際に決めたこと（2026-11-12）

1. **ステップ 1-7 の `.when` の LLM の変数の値は、人ではなく機械で設計 §3.1 の表と照合する。** 設計書の §3.1 の表から構成 ID と 2 つの変数の値を取り出し、preflight の `.when` の値の組と `diff` する。
    - **理由:** 人が、機械的に確かめられる照合は機械で行うよう求めた。表の値と記録の値の文字列の一致なので、機械の照合は人の照合より見落としがない。
    - **ADR の判断:** 作らない。このタスクの中で閉じる手順であるためである。

## 2. 実装手順 (Implementation Steps)

`_eval/` は `docs/tasks/0087_openai_combination_eval/_eval/` を、`evaltool` は `_eval/evaltool/` のパッケージを、`0087/` は `~/yt2column-eval/0087/` を指す。`_eval/` のコメント・識別子・文字列・データのファイルの鍵は英語で書く（`CLAUDE.md`「Language」）。判定の観点も英語の鍵で持ち（例: 「根拠」は `evidence`）、評価記録では日本語の名前で示す。

### フェーズ 1：準備（API を呼ばない）

#### ステップ 1-1：集計・照合・採点の素材・見積もりの Go のツール

**追加ファイル:** `_eval/evaltool/` の Go のソースとテスト。

- [x] 入力の読み取りを作る。0087 の `log/`・`failed/` の標準エラーと `.when`、保存した出力の `id`、判定のファイルを読む。0018 の配置（構成 ID の `B3g`・`B3`・`C03`、項目の少ない `.when`、`0018/failed/`）は、別の読み取り口で 0087 と同じ内部の形にする。roNZ-rLxhUE-1 の字数の調整の時間（設計 §3.7 の 54 秒）は、定数にせず 0018 の B3 のやり直しのログから求める。
- [x] 判定のファイル（`0087/score/<段>/judgments.tsv`）の形を決めて読む。少なくとも、記事の名前・構成 ID・観点・解釈による判定・文言どおりの判定・判定者（`script`・`llm`・`human`）・人の確認の印を持つ。長さ・節の数・タイトルの字数は、判定のファイルに書き写さず、`check_article.py` の出力のファイル（`0087/score/<段>/check_article.txt`）から読む。次のどれかに当たるファイルは拒否する。記事・観点・判定の欠け、人の確認の印のない「根拠」の合格、人の確認の印のない製品の制約 1〜3 の判定。
- [x] `aggregate` サブコマンドを作る。次の値を、評価記録の表ごとに鍵（記事・構成・工程など）と値の形で出す。
    - 基準の構成の 10 件の、工程ごとの保存した出力の `id` と生成の日付（AC-01。`0018/base/B3/`・`0018/work/B3/` から読む）。
    - 実行の表: 記事ごとの構成・`--after`・LLM の変数・開始の時刻・工程ごとの保存した出力の `id`、構成の実行の順序と並列に起動した組（AC-06）。
    - すべての `.when` が `YT2COLUMN_YTDLP_PATH=/usr/bin/false` と `--keep-cache` を記録していること（AC-02）。
    - 構成 A の識別子の照合と、`cost: shorten run` の行がある実行（手順から外れた実行。AC-03）。
    - 設計 §3.7 の値: 工程ごとのトークン数と推定コスト（`reused` の出どころを含む）、所要時間、失敗した実行、比較の表、プロンプトのキャッシュから読んだ入力のある工程の数と補助の推定コスト、使った額と `unknown` の工程の数。
    - 判定の表: 記事・観点ごとの両方の判定と判定者、構成ごとの合格件数（両方の判定）、品質の観点の判定の分布（AC-07・AC-09・AC-10）。
    - 引用が要る記事と観点の一覧（必須の観点の不合格と品質の観点の 1。AC-12）と、製品の制約 1〜3 の記事ごとの有無（AC-13）。
    - 承認より前に始まった実行の一覧（AC-19）と、やり直しの回数が 2 回を超えた記事の一覧（AC-17）。
    - B のモデルの選択（要件 F-002）。
    - F-006 の条件 (i)〜(iii) の判定、手順 2 の選び方、手順 3 で人が除いた候補を除いた選び直し、製品の制約 1〜3 が直ったか（AC-21〜AC-23）。基準の値は `_eval/baseline.json`（ステップ 1-2）から読む。
- [x] 使った額は、§1.2 の決めたこと 7 のとおりに数え、支払い前の候補の表を出す。この計算は 1 つの関数に置き、`aggregate` と `launch`（ステップ 1-4）が使う。
- [x] `estimate` サブコマンドを作る。`_eval/bounds-input.json`（ステップ 1-2）と `run.sh` が出す構成の表（テストでは合成の表）から、設計 §3.5 の見積もりの表と、構成・工程ごとの上限の表（`_eval/bounds.tsv`）を `Estimate` で作る。料金表にないモデルがあれば止まる。
- [x] `rates` サブコマンドを作る。評価に使うモデルの単価（通常の入力・キャッシュの読み取り・キャッシュの書き込み・出力、OpenAI の 2 つの単価の段）を、`Estimate` に区分ごとの使用量を与えて求め、100 万トークンあたりに換算して出す（AC-15）。OpenAI の通常の段は 272,000 トークン以下の使用量で、もう 1 つの段はそれを超える使用量で求める。
- [x] `check-record` サブコマンドを作る。`04_evaluation.md` の印の名前の集合が、`aggregate`・`estimate`・`rates` が出す表の名前の集合と一致することと、各表の値のセルが一致することを確かめる。印の欠け・余分・重複、表の値の違いのそれぞれで、その表の名前を出して失敗する。比べた入力の SHA-256 を出す。
- [x] `material` サブコマンドを作る。設計 §3.6 の手順 2 のとおり、伏せた素材と、名前と構成の対応のファイルを作る。対応のファイルは、採点のサブエージェントに渡す素材のディレクトリの外に置く。
- [x] `evaltool` のテストを書く。外部のディレクトリを読まず、`t.TempDir` に作った合成の入力だけを使う。各テストは、すべての条件が通る場合（正の対照）と、条件を 1 つずつ変えた場合を同じ入力から作る。少なくとも次を確かめる。
    - 入力: `known`・`unknown`・`reused` の行、0018 の配置の読み取り。
    - 使った額: `reused` を数えないこと、`cost-total: run` の行がない実行に上限の額を数えること、支払い前の候補の表に出る実行と出ない実行。
    - 所要時間: 構成 A の足し方（roNZ-rLxhUE-1 の字数の調整の時間を含む）、成功した実行だけの平均と最大。
    - 失敗: 支払い前の候補の区別、`prepayment.tsv` の実行を合計とやり直しの回数から除くこと、`launch` の使った額からは除かないこと。
    - 判定: 両方の判定での合格件数と分布、判定のファイルの拒否。
    - 承認より前に始まった実行の一覧（承認の時刻ちょうど、その前、追記された承認の記録）。
    - F-006: 合格件数の差がちょうど 2 件と 1 件、推定コストが基準と等しい場合と低い場合、3 倍と 2 倍のちょうどの値と超える値、動画ごとの必須の観点の不合格の比べ方、得られなかった記事と不明の推定コスト、手順 2 の同数、手順 3 の除外。補助の推定コストで判定が変わる場合の表示。
    - B のモデルの選択の同数。
    - 上限と見積もり: 手で計算した値と比べる。構成 A・B の検証・推敲（モデルが異なる場合の本文のバイト数）、構成 A の字数の調整、272,000 トークンを超える単価の段を含める。
    - `material`: タイトルと本文が出力にあり、対応のファイルがすべての名前を構成に戻せること。フロントマター・字数の調整の出力・一覧の各所に置いたモデル名と構成 ID が、出力に含まれないこと。
    - `check-record`: すべて一致する場合の成功と、値の違い・印の欠け・印の重複の失敗。
- [x] 各テストを書いた後、確かめる処理を一時的に壊して失敗することを確かめ、コミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。

**完了条件:** `go test ./docs/tasks/0087_openai_combination_eval/_eval/evaltool` と、同じパスの `go vet` が成功する。

#### ステップ 1-2：値のファイルと補助のスクリプト

**追加ファイル:** `_eval/bounds-input.json`・`_eval/baseline.json`・`_eval/scoring-prompt.md`・`_eval/scorer-agent.md`・`_eval/hashlist.sh`・`_eval/retire.sh`・`_eval/rates-compare.sh`。

- [x] `bounds-input.json` に、見積もりと上限の入力と出典を置く。モデル・工程ごとの固定の部分の入力（§1.3 の 0077 の実測と B3 の値）、出力の上限（OpenAI 6,080、Claude 40,000）、Claude の検証・推敲の B3 の 9 件の平均（0018 評価記録 §4.7.2）である。各値を出典と照合し、照合した結果をコミットメッセージに書く。
- [x] `baseline.json` に、基準の構成の値と出典を置く。合格件数、記事 1 本の推定コストの平均、所要時間の平均、動画ごとの必須の観点の不合格の数、製品の制約 1〜3 の有無である。動画ごとの不合格の数は、0008 評価記録 §6.2 の表から物差し 2 の解釈による判定で数え、設計 §3.4 の項目 4 の見込みと比べる。
- [x] `scoring-prompt.md`（英語）に、設計 §3.6 の手順 3 の 5 つの項目と、出力の表の鍵（英語）を書く。物差し 2 の定義は 0008 要件定義書 F-006 と 0018 評価記録 §5.5 を読ませ、写さない。
- [x] `scorer-agent.md` に、§1.2 の決めたこと 9 のエージェントの定義を書く。定義と `scoring-prompt.md` は、採点のためにサブエージェントに渡した素材のパスだけを読むことを指示する。
- [x] `hashlist.sh`（zsh）を作る。設計 §3.3 のハッシュの一覧を作り、前の一覧と `diff` する。
- [x] `retire.sh`（zsh）を作る。やり直しの前に、前の実行の `work/`・`out/`・`review/`・`log/`・`.when` と `runcache/` を `0087/failed/<構成 ID>/<記事>-f<N>/` に移す（設計 §4「やり直しの手順」）。
- [x] `rates-compare.sh`（zsh）を作る。`git show` で、評価に使うコミットと B3 の CLI のコミット `2fc2e2a` の `internal/pricing/rates.go` から `claude-opus-5-5` の項目を取り出し、比べる（設計 §3.4 の項目 3）。

**完了条件:** 各値のファイルの値を出典と照合した結果がコミットメッセージにある。スクリプトの動作確認はステップ 1-5 で行う。

#### ステップ 1-3：評価記録の骨組み

**追加ファイル:** `docs/tasks/0087_openai_combination_eval/04_evaluation.md`。

- [x] 設計 §3.8 の節 1〜10 を作る。節 2（手順）には、スクリプトとサブコマンドの名前と、設計 §3 の手順を具体化したコマンドを書く。記録の節には、見出しと、`check-record` が読む印を置く。見積もりの表は、`run.sh` ができた後のステップ 1-4 で書き写す。

**完了条件:** 節 1〜10 と、設計 §3.8 の表が各節に挙げる記録の印がある。

#### ステップ 1-4：1 件の実行と起動の判定

**追加ファイル:** `_eval/run.sh`、`evaltool` の `launch` のソースとテスト。

- [x] `run.sh`（zsh）を作る。設計 §3.1 の構成の変数の表（ここだけに置く）、§3.2 の再開の工程、§3.3 の写しの作り方とハッシュの照合、出力先と `runcache/` がすでにあれば起動しないこと、`.when` の内容（設計 §3.1 の項目に、`YT2COLUMN_YTDLP_PATH` の値と `--keep-cache` を加える）を実装する。API キーの値を読まず、書かず、表示しない。
- [x] `run.sh` に 2 つの使い方を加える。構成 ID を与えると構成の変数の表を出す使い方（§1.2 の決めたこと 5）と、出力の根のディレクトリを `0087/` の代わりに `0087/preflight/` にする使い方（ステップ 1-7）である。preflight の使い方は、どの構成 ID でも `--after review` を受け付ける（設計 §3.4 の項目 1）。
- [x] `launch` サブコマンドを作る。次の確認がすべて通った場合だけ、`run.sh` を 1 つの構成の 10 件（またはやり直しの 1 件）について並列に起動し、全件の終了を待つ。
    - 設計 §3.5 の確認: 承認の記録に起動する構成があること、同じ OpenAI のモデルを使う直前の構成の終了から 30 分たったこと、使った額と起動する実行の上限の和が最後の上限以下であること。
    - §1.2 の決めたこと 10 の確認。

    承認の記録が読めない場合、使った額を求められない場合、構成のモデルが料金表にない場合も止まる。`run.sh` のパス・`0087/` の根・CLI のパスとその SHA-256・現在の時刻は引数で受け取る。1 段目の判定のファイルは `0087/` の根の下の `score/1/` から読む。承認の記録は、1 行に 1 つの承認（時刻・承認した構成の一覧・上限）を持ち、構成の確認と上限には最後の行を使う。
- [x] `launch` のテストを書く。`run.sh` の代わりに引数を記録するだけのスタブを使い、正の対照（すべて通ると 10 件を起動する）と、条件を 1 つずつ変えた場合（スタブを一度も起動しない）を同じ入力から作る。少なくとも次を含める。承認の記録がない・壊れている・構成を含まない、上限とちょうど等しい場合（起動する）と超える場合、30 分ちょうどとその前、同じモデルの別の種類の構成（`C-sol` の後の `A-sol` は待ち、`C-sol` の後の `C-astra` は待たない）、やり直しの 1 件の上限、使った額を求められないログ、1 段目の判定がそろわない `B-*`。
- [x] 各テストについて、確かめる条件を一時的に外すとテストが失敗することを確かめ、コミットメッセージに書く。
- [x] `estimate` で `bounds.tsv` を作り、見積もりの表を `04_evaluation.md` 節 5 に書き写す。設計 §3.5 の概算の値と大きく違う場合は、その理由を書く。

**完了条件:** `go test ./docs/tasks/0087_openai_combination_eval/_eval/evaltool` が成功する。

#### ステップ 1-5：道具の確かめ

**追加ファイル:** `_eval/selftest.sh`・`_eval/reproduce0018.sh`・`_eval/repo_impact.sh`・`_eval/secretscan.sh`（いずれも zsh）。

- [x] `selftest.sh` を作って実行する。次を行う。
    - `evaltool` の `go test`・`go vet`、同じパスへの `golangci-lint`（`Makefile` の版）と `gofumpt`。
    - `hashlist.sh` の確かめ: 一時ディレクトリで、変えずに 2 回作った一覧の `diff` が空であることと、1 ファイルの内容またはモードを変えると空でなくなること（設計 §7.1）。
    - `secretscan.sh` の確かめ: 偽の値を環境とファイルに置くと見つけること。
- [x] `secretscan.sh` を作る。環境にある API キーの値と `sk-` の接頭辞を、`_eval/` と `04_evaluation.md` の内容と比べ、値を表示せずに有無だけを出す（設計 §7.4）。キーの変数が環境に 1 つもなければ失敗する。
- [x] `reproduce0018.sh` を作って実行する。`aggregate` を 0018 の B3・C03 のデータに当て、0018 評価記録 §4.8 の値（§1.3 の表）と、`baseline.json` の推定コストの平均と所要時間の平均が出ることを確かめる（設計 §7.1）。
- [x] `repo_impact.sh` を作って実行する。マージベースと HEAD で、`go list ./...`、`make test` が実行するテストの名前の一覧、`make lint` の指摘が同じであることを確かめる（設計 §7.3、要件 §4.5）。
- [x] `rates` と `rates-compare.sh` を実行し、`rates` の `claude-opus-5-5` の単価が `986449c6` の料金表の値（`internal/pricing/rates.go:114`。入力 4、キャッシュの読み取り 0.2、キャッシュの書き込み 5、出力 20 USD/MTok）と一致することを確かめる。
- [x] `secretscan.sh` を実行する。PR の作成の後に `_eval/` か `04_evaluation.md` を変えた場合は、マージの前に実行し直す。
- [x] 出力を `04_evaluation.md` 節 2 に記録する。

**完了条件:** 上のスクリプトがすべて成功し、出力が記録されている。

### PR-1 作成ポイント: evaluation tooling

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `chore(0087): 評価の実行・起動・集計の道具と評価記録の骨組みを加える`

**レビュー観点**: 起動の判定と使った額・上限の計算 / 基準の構成の保存した出力を書き換えない写し方 / 採点の素材から構成が分からないこと / API キーと全文がリポジトリに入らないこと

**実装モデル要件**: frontier-required

**判定理由**: 1-4 の `launch` が、実 API の費用を承認の範囲に止める関門（外部のサービスへの支払いを守るセキュリティの関門）を作る。

- [x] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/851
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 1-6：ビルドの固定、ハッシュの一覧とバックアップ

PR-1 のマージの後、新しいブランチで行う（§1.2 の決めたこと 3）。

- [x] CLI のビルドを固定する（設計 §3.1）。未コミットの変更のない作業ツリーで `make build` を 1 回行い、`0087/bin/<コミット>/` に写し、SHA-256 を記録する。ビルドするコミットは main の先頭とし、main から到達できることを記録する。
- [x] `hashlist.sh` で最初のハッシュの一覧を作り、バックアップを作って別の場所へ展開して一覧と照合し、アーカイブのハッシュを記録する（設計 §3.3）。
- [x] 人の許可を得て `scorer-agent.md` を `~/.claude/agents/` に写し、セッションを開き直す（§1.2 の決めたこと 9）。

**完了条件:** バイナリの SHA-256、最初の一覧、アーカイブのハッシュが `04_evaluation.md` 節 2・4 に記録されている。

#### ステップ 1-7：実行の前の確認

- [x] 設計 §3.4 の項目 1〜4 を行う。項目 1 と項目 2 の CLI の起動は、すべて `run.sh` の preflight の使い方で行い、API キーを取り除く（`env -u`）。設計 §3.1 の共通の条件（`yt-dlp` を呼ばない設定、キャッシュの写し、`ANTHROPIC_WORKSPACE_ID` を未設定にする）は `run.sh` がすべての起動に当てる。やり直す場合は、`0087/preflight/` を空にしてから始める。
- [x] 項目 2 の結果を記録する。主セッションが各起動の標準エラーを読み、足りない変数として示されたのが設計 §3.1 の表のキーだけであり、値の誤りがないことを確かめる。`.when` の LLM の変数の値を、設計 §3.1 の表から取り出した値と機械的に照合する（人の求めにより、人の照合から変えた）。
- [x] CLI に関わるツリーが、ステップ 1-6 で固定したコミットと同じコミットの `evaltool` で `estimate` を実行し直し、`bounds.tsv` と節 5 の見積もりの表が変わる場合は書き換える。一括の承認は、書き換えた後の見積もりで受ける。
- [x] 項目 3 は `rates` と `rates-compare.sh` を評価に使うコミットで実行する。`claude-opus-5-5` の単価が違う場合は、基準の構成の推定コストを今の単価で求め直し、`baseline.json` の値を変えて、`04_evaluation.md` 節 6 に記録する（AC-15）。
- [x] `check_article.py` を、項目 1 の `--after review` が書いた記事に当て、今の CLI の記事を読めることを確かめる。
- [x] 項目 1 の記事を 1 件だけ `material` で伏せ、`scorer-agent.md` のエージェントで試しに採点させる。出力の表の形、記録されたモデル ID（`claude-opus-5-5`）、`Read` 以外の道具（コマンドの実行と書き込み）を使えないことを確かめる。採点の結果は評価に使わない。
- [x] ステップ 1-5 の後に `_eval/` を変えた場合は、`selftest.sh`・`reproduce0018.sh`・`repo_impact.sh` を実行し直して記録する。

**完了条件:** 上の確かめがすべて成功し、`04_evaluation.md` の節 2〜4・6 に記録されている。CLI は API を一度も呼んでいない（項目 2 の起動はすべて終了コード `2`、項目 1 は API を呼ばない再開）。

### フェーズ 2：一括の承認

#### ステップ 2-1：見積もりを示して承認を受ける

**変更ファイル:** `04_evaluation.md`（リポジトリの外では `0087/approval`）。

- [ ] `04_evaluation.md` 節 5 の見積もりの表（1 段目と 2 段目の構成・回数・推定コスト）と上限 30 USD を示し、人の承認を受ける（要件 F-005）。
- [ ] 承認の時刻・構成・上限を `0087/approval` に書き（§1.2 の決めたこと 8）、`04_evaluation.md` 節 5 に記録する。

**完了条件:** 承認の記録がある。この時点まで、API キーを渡した CLI の起動はない。

### フェーズ 3：1 段目

#### ステップ 3-1：4 構成を実行する

**変更ファイル:** `04_evaluation.md`。

- [ ] `launch` で `C-sol` → `C-astra` → `A-sol` → `A-astra` の順に、1 構成ずつ実行する（設計 §3.5）。`launch` が止まった場合は、その理由を人に示す。上限による停止では、人の承認を改めて受け、`0087/approval` に追記する。
- [ ] 失敗した実行は、設計 §4 の表で扱いを決める。やり直す場合は `retire.sh` の後に `launch` で 1 件を起動する。番兵は主セッションが標準エラーを読んで記録し、人が確認する。
- [ ] 入力が拒否された実行は、設計 §3.3「拒否された入力」の手順で、元の入力のハッシュを一覧と比べ、変わっていればバックアップから元のモードのまま戻して確かめ、`04_evaluation.md` 節 4 に記録する。
- [ ] 1 段目の後に `hashlist.sh` で一覧を作り、最初の一覧との `diff` が空であることを節 4 に記録する。

**完了条件:** 4 構成の 10 件それぞれについて、記事を得たか、得られなかったことが記録されている。

#### ステップ 3-2：採点と人の確認

**変更ファイル:** `04_evaluation.md`。

- [ ] `check_article.py` で長さ・節の数・タイトルの字数を判定し、出力を `0087/score/1/check_article.txt` に置く。
- [ ] `material` で 1 段目の伏せた素材を作り、`scorer-agent.md` のエージェントで、動画ごとに 1 つのサブエージェントが採点する（設計 §3.6 の手順 3）。採点の全文は `0087/score/1/` に置く。
- [ ] 採点を構成ごとの表に戻し、設計 §3.6 の手順 4 の項目を人に確認してもらう。人が変えた判定と理由を記録し、判定のファイルを書く。
- [ ] `aggregate` の値を `04_evaluation.md` 節 7 に書き写し、不合格・品質の 1 の引用、製品の制約 1〜3 の引用、番兵、採点に使ったモデル ID を書く。

**完了条件:** 1 段目の 4 構成の判定が人の確認を終え、記録されている。

### フェーズ 4：2 段目

#### ステップ 4-1：B を作って採点する

**変更ファイル:** `04_evaluation.md`。

- [ ] `aggregate` が出す B のモデルの選択を `04_evaluation.md` 節 8 に記録する。
- [ ] `launch` で `B-<モデル>` を実行する（同数なら `B-sol` → `B-astra`）。失敗と拒否の扱いはステップ 3-1 と同じである。
- [ ] 2 段目の後に `hashlist.sh` で一覧を作り、`diff` が空であることを記録する。
- [ ] ステップ 3-2 と同じ手順で採点し（出力は `0087/score/2/`）、人の確認を受け、節 8 に記録する。
- [ ] 採点がすべて終わった後、`~/.claude/agents/` に写したエージェントの定義を消す。

**完了条件:** B の各構成の判定が人の確認を終え、記録されている。

### フェーズ 5：比較と判断

#### ステップ 5-1：比較の表と F-006 の判断

**変更ファイル:** `04_evaluation.md`、本書。

- [ ] `aggregate` の比較の表、使った額、F-006 の判定を、`04_evaluation.md` 節 9・10 に書き写す。プロンプトのキャッシュから読んだ入力のある構成は、補助の推定コストの平均で F-006 の判定が変わるかを書く（設計 §3.5）。
- [ ] F-006 の手順 2 で選んだ構成（候補がなければ「変えない」）を、失敗した実行の回数と費用とともに人に示し、承認を受ける。人が承認しない場合は、その候補を除いて `aggregate` で選び直す（要件 F-006 の手順 3）。
- [ ] 結論と節 1 の要約を書く。
- [ ] PR-2 で `_eval/` を変えた場合は、`selftest.sh`・`reproduce0018.sh`・`repo_impact.sh` を実行し直して記録する。
- [ ] `check-record` と `secretscan.sh` を実行し、出力・入力の SHA-256・実行したコミットを本書 §7 の「記録」に書く。PR の作成の後に `04_evaluation.md` か `_eval/` を変えた場合は、マージの前に 2 つを実行し直して記録を書き換える。
- [ ] 本番の構成を変えない場合は、次をすべて `[-]` にし、理由として `04_evaluation.md` の結論を参照する。ステップ 6-1・6-2 のチェックボックス、PR-3 の作成ポイントの 4 つのチェックボックス、§6 の PR-3 の行、§8 の「本番の構成を変える場合」の行。§7 の AC-24〜AC-30 の行に「該当しない」と書く。

**完了条件:** 評価記録の結論が人の承認を受け、`check-record` が成功している。

### PR-2 作成ポイント: evaluation runs and decision

**対象ステップ**: 1-6 / 1-7 / 2-1 / 3-1 / 3-2 / 4-1 / 5-1

**推奨タイトル**: `docs(0087): Claude と OpenAI の組み合わせの評価を記録し、本番の構成を判断する`

**レビュー観点**: 評価記録の値が集計の値と一致すること / 人が確認した判定と変更の理由 / F-006 の判断と人の承認 / 引用が短く全文がないこと

**実装モデル要件**: frontier-required

**判定理由**: 3-1・4-1 が実 API を呼ぶ有料の実行を含み、外部の資源に重く触れる。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6：本番の構成の変更（変える場合だけ）

選んだ構成が C か、A・B かで範囲が分かれる（設計 §3.9 の表）。A・B は R1（設計 §1.2 の決めたこと 1、§3.9.1）を入れる。ステップ 6-1 だけでは文書のガードテストが失敗するので、グリーンゲートはステップ 6-2 の後に確かめる。

#### ステップ 6-1：既定の構成と継承の規則

**変更ファイル:** `internal/config/config.go`・`config_test.go`・`serverfile.go`・`serverfile_test.go`・`test_helpers.go`、`cmd/yt2column/run_test.go`、`cmd/yt2column-server/run_test.go`。

- [ ] 設計 §3.9.2 の表のとおり既定の定数を変える。C・B では、次の改名の台帳に従う。新しい名前は実装で決め、本書の台帳を同じコミットで埋める。

    | 変更前の名前 | 変更後の名前 | 場所 |
    |---|---|---|
    | `defaultClaudeModel` | （プロバイダによらない名前） | `internal/config/config.go:76` と §1.3 の `config_test.go` の行 |
    | `DefaultClaudeModelForTest` | （同上に合わせる） | `internal/config/test_helpers.go`、`cmd/yt2column/run_test.go:1583` |

- [ ] A・B では、検証・推敲の既定の定数を加え、`loadLLM` に R1 の分岐を加える。`parseLLM` を通す。`serverfile.go` の雛形の `YT2COLUMN_REVIEW_LLM` の「When unset」の説明を R1 と代償に合わせる。テスト用に検証・推敲の既定を返す関数を `test_helpers.go` に加える。
- [ ] 設計 §3.9.3 のテストと、§1.3 に挙げた 4 つのテスト（`TestStartupFailures`・`TestServerIgnoresEnvSettings`・`TestConfigInitThenStart`・`TestRunClaudeConfigurationErrorsOneLineEach`）を新しい既定で実行し、失敗するものを直す。
- [ ] A・B では、R1 の 3 つの分岐と、選んだ構成のプロバイダのキーだけを必須にすることを、`LoadFor` と `LoadServer` の両方で確かめるテストを加える。
- [ ] 新しい既定で、足りないキーが終了コード `2` と足りないキーの名前で拒否され、API を呼ばないこと、使わないプロバイダのキーを要さないことを、CLI（`cmd/yt2column/run_test.go`）とサーバ（`TestStartupFailures`）のテストで確かめる。既存のテストを広げるか加えるかは実装で決め、§7 の AC-25 の行に名前を書く。
- [ ] 直したテストと加えたテストが、既定の定数（A・B では R1 の分岐も）を元に戻すと失敗することを確かめ、コミットメッセージに書く。

#### ステップ 6-2：文書・ガードテスト・ADR

**変更ファイル:** §1.3 と設計 §3.9.2 の文書、`cmd/yt2column/docs_test.go`。

- [ ] `claude-opus-5-5`・`YT2COLUMN_REVIEW_LLM`・`本番の構成`・`ANTHROPIC_API_KEY`・`40,000` を `docs/tasks/` の外で検索し直し、§1.3 の結果と設計 §3.9.2 の表を突き合わせて、変える箇所の一覧を作る。一覧はコミットメッセージに書く。
- [ ] `docs/dev/production_profile.md`・`docs/dev/project_overview.md`・`docs/dev/security.md`・`docs/dev/conventions/package_reference.md` を、設計 §3.9.2 の表と §3.9.4 のとおりに変える。A・B では `docs/dev/guides/cli.md` も変える。OpenAI を既定に含む場合は `docs/dev/guides/development.md` の開発用のサーバの設定と ADR-0041 の記述を、C・B では `docs/dev/guides/claude_output_limit.md` を変える。品質・料金・所要時間の値は、`check-record` で確かめた評価記録の表から写す。
- [ ] `README.md` を変え、同じコミットで `/mktrans README.md` で `README.en.md` を同期する。
- [ ] A・B では、`TestLLMDefaultsDocsContract` を `YT2COLUMN_REVIEW_LLM` の既定の値も照合するように広げ、`TestProductionProfileDocsContract` の `productionProfileVars` に `YT2COLUMN_REVIEW_LLM` を加える。`docsProfileEnv` に OpenAI のキーが要るかを、テストを実行して確かめる（設計 §3.9.3）。
- [ ] ADR-0004 を更新し、索引の題名を合わせる（AC-28）。A・B では ADR-0003 を、C・B では ADR-0025 を更新する。どれも [adr.md](../../dev/conventions/adr.md) に従う。
- [ ] `make test`・`make lint`・`make ext-test`・`make readme-sync-check` を実行する。

**完了条件:** 上の 4 つの make ターゲットが成功する。

### PR-3 作成ポイント: production profile change

**対象ステップ**: 6-1 / 6-2

**推奨タイトル**: `feat(0087): 本番の構成を評価で選んだ構成に変える`

**レビュー観点**: 未設定のときの検証・推敲の選択（R1）と必須のキー / 既定の送り先の変化と既定を書いた文書・ガードテストの漏れ / ADR-0004・ADR-0003・ADR-0025 の決定と採らなかった案

**実装モデル要件**: frontier-required

**判定理由**: 6-1 が、すべての利用者の既定の送り先と必須のキーを同時に変える移行（振る舞いを強める変更と弱める変更が同時に起きる）に当たる。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序

設計 §8 の順に、準備 → 一括の承認 → 1 段目 → 2 段目 → 比較と判断 → 本番の構成の変更と進める。準備のうち、ビルドの固定と実行の前の確認（ステップ 1-6・1-7）は PR-1 のマージの後に行う（§1.2 の決めたこと 3）。

| マイルストーン | 成果物 |
|---|---|
| M1（PR-1 のマージ） | `_eval/` の道具とその確かめ、`04_evaluation.md` の骨組みと見積もり |
| M2（ステップ 1-7 の終わり） | ビルドの固定、ハッシュの一覧とバックアップ、実行の前の確認の記録 |
| M3（フェーズ 3 の終わり） | 1 段目の 4 構成の記録と人の確認 |
| M4（PR-2 のマージ） | 評価記録の完成と、本番の構成の判断 |
| M5（PR-3 のマージ。変える場合だけ） | 新しい既定と、それを書いた文書・ADR |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 評価の道具と評価記録の骨組み | frontier-required |
| PR-2 | 1-6 / 1-7 / 2-1 / 3-1 / 3-2 / 4-1 / 5-1 | 実行の前の確認、評価の実行・採点・記録、本番の構成の判断 | frontier-required |
| PR-3 | 6-1 / 6-2 | 本番の構成の変更（変える場合だけ） | frontier-required |

PR-2 は複数のセッションにわたる。`/runplan` は 1 フェーズずつ進め、PR-2 の作成ポイントまで同じブランチに積む。

## 4. テスト戦略 (Test Strategy)

- **`_eval/` のテスト:** `evaltool` の Go のテストは、合成の入力だけを使い、`t.TempDir` の外を読まない。`make test` にも `make lint` にも入らないので、`selftest.sh` がパッケージのパスを指定して `go test`・`go vet`・`golangci-lint`・`gofumpt` を実行する。
- **既知の値との照合:** 集計の足し方は、`reproduce0018.sh` で 0018 の記録の値を再現して確かめる。単価は `986449c6` の料金表の値と比べる。
- **API を呼ばない起動:** CLI の起動は、すべて `run.sh` の preflight の使い方で、API キーを取り除いて行う（ステップ 1-7）。
- **リポジトリへの影響:** `repo_impact.sh` で、`go list ./...`・`make test` のテストの名前・`make lint` の指摘が変わらないことを確かめる。
- **書き写しの照合:** 評価記録の値は `check-record` で集計の値と照合し、記録の後に変えた場合は照合し直す。
- **F-007:** 既存のテストを新しい既定で直し、A・B では R1 のテストを加える。直したテストが既定を戻すと失敗することを確かめる。
- **後方互換:** F-007 で変える既定について、版の間の互換の仕組みは設けない（設計 §3.9.4）。利用者に見える変化は README と `production_profile.md` に書く。
- 各 PR の完了時に `make test`・`make lint` を実行する。Go を編集した場合は先に `make fmt` を実行する。PR-3 では `make ext-test` も実行する。既存のテストは削除しない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 承認の範囲を超えて費用がかかる | `launch` が起動の前に、使った額と起動する実行の上限の和が最後の上限を超えないかを確かめ、超える場合と使った額を求められない場合は止まる（ステップ 1-4）。上限の前提（1 トークンが 1 バイト以上）が外れる危険は評価記録に書く（設計 §3.5） |
| 基準の構成の保存した出力を書き換える | 写しだけを CLI に渡し、ハッシュの一覧を 3 回比べ、バックアップを持つ（ステップ 1-4・1-6・3-1・4-1） |
| 構成のモデル ID の打ち間違いで、有料の実行が別のモデルを呼ぶ、または失敗する | 料金表にないモデルで見積もりと起動が止まる（§1.2 の決めたこと 5）。`.when` の値を人が設計 §3.1 の表と照合する（ステップ 1-7） |
| 10 件の並列でレート制限（HTTP 429）が起きる | 工程の失敗としてやり直し、内容による失敗と分けて記録する（設計 §3.5）。所要時間に注記する |
| 採点が構成やプロバイダの名前に引きずられる、素材の指示に操られる | 伏せた素材と、`Read` だけを持つエージェントで採点し、人が不合格と「根拠」の合格を確認する（ステップ 3-2） |
| 評価記録への書き写しの誤り | `check-record` で照合する（ステップ 5-1） |
| 評価が複数日にわたり、OpenAI のモデルの振る舞いが変わる | 実行の日時を記録し、比べるときに併せて示す（要件 §5） |
| F-007 の既定の変更で、`ANTHROPIC_API_KEY` だけの利用者が止まる | 閉じる側に倒れる振る舞いとして README に書く（設計 §3.9.4）。PR-3 を独立させ、戻すときは PR-3 の squash のコミットを戻す |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 1-6 / 1-7 / 2-1 / 3-1 / 3-2 / 4-1 / 5-1）
- [ ] PR-3 マージ済み（対象ステップ: 6-1 / 6-2）
- [ ] §7 の表のすべての AC の検証結果を記録した

## 7. 受け入れ条件の検証 (Acceptance Criteria Verification)

型は、`test` がテストの実行、`static` がコミットしたスクリプトかサブコマンドの実行、`manual` が PR のレビューでの読解である。`_eval/` のテストとスクリプトは `make test` に入らず、記録したステップで実行する。

- `test: evaltool` は `go test ./docs/tasks/0087_openai_combination_eval/_eval/evaltool`（`selftest.sh` が実行する）を指す。
- `static: check-record` は、ステップ 5-1 で `check-record` を `04_evaluation.md` に当てた実行を指す。印の名前の集合と値のセルを照合する。
- 評価の記録の AC のうち、人の行為（確認・承認）と自然言語の記述（引用・理由・場所の説明）は `manual` で読む。

| AC | ステップ | 型と実行する検証 |
|---|---|---|
| AC-01 | 1-1 / 5-1 | test: evaltool（0018 の配置の読み取り）。static: `check-record`（基準の構成の識別子と日付の表）。manual: 判定の出典と実行の条件の違い（出力の上限の既定、CLI のコミット）の記述 |
| AC-02 | 1-1 / 1-6 / 3-1 / 4-1 / 5-1 | static: `hashlist.sh` のキャッシュの一覧の `diff`（3 回、節 4 に記録）、`check-record`（すべての `.when` が `yt-dlp` を呼ばない設定と `--keep-cache` を記録していること）。manual: キャッシュの場所と取得日の記述 |
| AC-03 | 1-1 / 3-1 / 5-1 | test: evaltool（識別子の不一致と `cost: shorten run` の検出）。static: `check-record`（構成 A の識別子の表と、手順から外れた実行の一覧） |
| AC-04 | 1-5 / 1-6 / 3-1 / 4-1 | test: `selftest.sh`（`hashlist.sh` の変更の検出と、変えない場合の空の `diff`）。static: `hashlist.sh` の基準の構成の一覧の `diff`（3 回、節 4 に記録） |
| AC-05 | 1-1 / 3-1 / 4-1 | test: evaltool（支払い前の失敗の区別）。manual: 拒否された入力と戻したことの記録（拒否がなければ「なし」の記録） |
| AC-06 | 1-1 / 1-4 / 5-1 | test: evaltool（`.when` の読み取り）。static: `check-record`（実行の表、順序と並列の組） |
| AC-07 | 1-1 / 3-2 / 5-1 | test: evaltool（判定のファイルの欠けの拒否）。static: `check-record`（1 段目の 4 構成の判定の表） |
| AC-08 | 1-1 / 1-4 / 4-1 / 5-1 | test: evaltool（B のモデルの選択と同数、`launch` が 1 段目の判定がそろうまで `B-*` を起動しないこと）。static: `check-record`（選択の表と B の判定の表） |
| AC-09 | 1-1 / 3-2 / 4-1 / 5-1 | test: evaltool（両方の判定での合格件数）。static: `check-record`（判定の表と合格件数） |
| AC-10 | 1-1 / 3-2 / 4-1 | static: `check-record`（判定者を持つ判定の表）。manual: 採点に使ったモデル ID、人が変えた判定と理由 |
| AC-11 | 1-1 / 3-2 / 4-1 | test: evaltool（人の確認の印のない「根拠」の合格の拒否）。manual: 人の確認の記録 |
| AC-12 | 1-1 / 3-2 / 4-1 | static: `check-record`（引用が要る記事と観点の一覧）。manual: 引用と、誤りを入れた工程・直さなかった工程 |
| AC-13 | 1-1 / 3-2 / 4-1 | test: evaltool（人の確認の印のない制約の判定の拒否）。static: `check-record`（制約 1〜3 の表）。manual: 引用 |
| AC-14 | 1-1 / 5-1 | test: evaltool（`known`・`unknown`・`reused` の行と補助の推定コスト）。static: `check-record`（トークン数と推定コストの表） |
| AC-15 | 1-1 / 1-5 / 1-7 | test: evaltool（`rates` を含む上限の計算）。static: `rates` の出力と `986449c6` の料金表の値の比較（ステップ 1-5）、`rates-compare.sh`（節 6 に記録）、`check-record`（単価の表） |
| AC-16 | 1-1 / 1-5 / 5-1 | test: evaltool（所要時間の足し方）。static: `reproduce0018.sh`（節 2 に記録）、`check-record`（所要時間の表） |
| AC-17 | 1-1 / 3-1 / 4-1 / 5-1 | test: evaltool（失敗の集計とやり直しの回数）。static: `check-record`（失敗した実行の表と、やり直しが 2 回を超えた記事の一覧）。manual: 番兵と、設定の誤りの分け方 |
| AC-18 | 1-1 / 5-1 | test: evaltool（成功した実行だけの平均と最大、判定の分布）。static: `check-record`（比較の表） |
| AC-19 | 1-1 / 1-4 / 2-1 / 5-1 | test: evaltool（`launch` が承認の記録なしで起動しないこと、承認より前に始まった実行の一覧）。static: `check-record`（見積もりの表と、承認より前に始まった実行の一覧）。manual: 承認の時刻の記録 |
| AC-20 | 1-1 / 1-4 / 5-1 | test: evaltool（使った額の数え方、上限を超えるときの `launch` の停止）。static: `check-record`（使った額の表） |
| AC-21 | 1-1 / 1-2 / 5-1 | test: evaltool（F-006 の条件の境界）。static: `reproduce0018.sh`（`baseline.json` の平均の値）、`check-record`（F-006 の判定の表）。manual: `baseline.json` の動画ごとの不合格の数と 0008 評価記録 §6.2 の照合 |
| AC-22 | 1-1 / 5-1 | test: evaltool（手順 2 の選び方と手順 3 の除外）。manual: 人の承認、示した失敗の回数と費用、除いた候補と理由 |
| AC-23 | 1-1 / 5-1 | test: evaltool（制約が直ったかの判定）。static: `check-record`（制約が直ったかの表）。manual: 基準の構成の値と 0008 評価記録 §6.3 の照合 |
| AC-24 | 6-1 | test: `internal/config/config_test.go::TestLoadDefaults`・`TestLoadGenerationDefaults`・`TestLoadReviewDefaults`・`TestLoadServerLLMDefaults`・`TestLoadProductionDefaults`・`TestLoadInheritsEffort`、`cmd/yt2column/run_test.go::TestRunDefaultLLMConfig`。A・B では R1 の新しいテスト（ステップ 6-1 で名前を決め、ここに書く） |
| AC-25 | 6-1 | test: `internal/config/config_test.go::TestLoadDefaultProviderVariables`、`cmd/yt2column-server/run_test.go::TestStartupFailures`、CLI のテスト（ステップ 6-1 で名前を決め、ここに書く） |
| AC-26 | 6-2 | static: `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`・`TestProductionProfileDocsContract`、`internal/config/serverfile_test.go::TestServerFileTemplate`。manual: `production_profile.md` の品質・料金・所要時間の値と、`check-record` で確かめた評価記録の表の照合 |
| AC-27 | 6-2 | manual: `project_overview.md` のモデル名の例外の項目（自然言語の内容だけを定める） |
| AC-28 | 6-2 | static: `cmd/yt2column/adr_test.go::TestADRRecords`。manual: ADR-0004 の決定・採らなかった案・経緯 |
| AC-29 | 6-2 | manual: README の両版と `security.md` の送り先の記述（自然言語の内容だけを定める） |
| AC-30 | 6-2 | static: `make test`・`make lint`・`make ext-test`・`make readme-sync-check` |

AC-24〜AC-30 は、本番の構成を変える場合だけ行う。変えない場合は、ステップ 5-1 で各行に「該当しない」と結論への参照を書く。

**記録:**（`/runplan` が、`check-record` と `secretscan.sh` の出力、比べた入力の SHA-256、実行したコミットを書く）

## 8. 完了条件 (Success Criteria)

- [ ] AC-01〜AC-23 の検証が完了し、評価記録の結論が人の承認を受けている。
- [ ] 実 API を呼んだ実行の推定コストの合計が、承認の上限以下である。
- [ ] 基準の構成の保存した出力と評価用のキャッシュが、評価の前後で変わっていない。
- [ ] `_eval/` と `04_evaluation.md` に API キーと全文がない（`secretscan.sh` とレビュー）。
- [ ] 本番の構成を変える場合は、AC-24〜AC-30 の検証が完了している。

## 9. 次のステップ (Next Steps)

本書が承認された後、`/runplan 0087` で PR-1 から進める。PR-1 のマージの後、ステップ 1-6・1-7 で実行の前の確認を終えてから、フェーズ 2 で一括の承認を受ける。OpenAI 向けのテンプレートの調整の余地があると判断した場合は、別の issue にする（要件の決めたこと 4）。

## 変更履歴

- 2026-11-11 承認（isseis、#841）
- 2026-11-12 セッション中の再承認：ステップ 1-7 の `.when` の LLM の変数の値を、人ではなく機械で設計 §3.1 の表と照合する（1.4・ステップ 1-7）（isseis、#未記入）
