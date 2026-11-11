# 実装計画書：Claude と OpenAI の組み合わせの評価と、本番の構成の見直し

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的と実装方針

[要件定義書](01_requirements.md)（以下「要件」）の AC-01〜AC-30 を、[設計書](02_architecture.md)（以下「設計」）に従って実装する。評価の手順と記録の形は設計 §3.1〜§3.8 で決まっているので、本書は、作るファイル、実行する順序、各 AC を確かめる検証を定める。

- フェーズは設計 §8 の 6 つの優先順位にそのまま対応させる。フェーズ 1 は API を呼ばない。実 API を呼ぶのは、フェーズ 2 で一括の承認を受けた後のフェーズ 3・4 だけである。
- 評価の手順は一度きりなので、`make test` に入る恒久的なテストは作らない（設計 §7.1）。`_eval/` の検証は、`_eval/` のテストとスクリプトをパスを指定して実行し、出力を評価記録または本書の検証の表に記録する。
- 評価記録 `04_evaluation.md` に書く数値と表は、主セッションが手で書き写さない。集計のツールが Markdown の表として出し、主セッションがそれを貼る。貼った表が集計の出力と一致することは、集計のツールの照合で確かめる（ステップ 1-1）。
- フェーズ 6（本番の構成の変更）は、フェーズ 5 の判断で本番の構成を変えると決めた場合だけ行う。変えない場合は、フェーズ 5 でフェーズ 6 と PR-3 のチェックボックスをすべて `[-]` にし、その理由として評価記録の結論への参照を書く。

### 1.2. 計画の際に決めたこと（2026-11-11）

1. **`_eval/` は、1 件の実行とハッシュの一覧をシェル（zsh）で、起動・集計・採点の素材・見積もりを Go で書く。** Go のほうは、`_eval/evaltool/` の 1 つの `main` パッケージにまとめ、サブコマンドで分ける。
    - **理由:** 1 件の実行は 0018 と同じく zsh の `time` で測る。基準の構成の所要時間と測り方がそろい、集計のツールが読む `time` の出力の形が 1 つで済む。起動の判定（承認の記録・構成の間隔・費用の予約）は 2 つ以上の条件を持ち、入力を与えて確かめるテストが要る。そのため Go で書き、`_eval/evaltool/` のテストで確かめる（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の方針にも沿う）。費用の上限と補助の推定コストは `internal/pricing` の `Estimate` を再利用し、単価を写さない。
    - レビュアーが推奨案を選んだ。
    - **ADR の判断:** 作らない。このタスクの中で閉じる道具の選択であるためである。
2. **PR は 3 本に分ける。** PR-1 は準備（フェーズ 1）、PR-2 は一括の承認・1 段目・2 段目・比較と判断（フェーズ 2〜5）、PR-3 は本番の構成の変更（フェーズ 6。変える場合だけ）である。
    - **理由:** 費用を払う前に、起動と上限のスクリプトを PR-1 でレビューできる。評価の記録は 1 本の PR にまとめ、記録だけの PR を増やさない。
    - レビュアーが推奨案を選んだ。
    - **ADR の判断:** 作らない。このタスクの中で閉じる進め方であるためである。

次は、承認済みの文書の範囲で計画が決めた。人に尋ねていない。

3. **評価記録の数値の表は、集計のツールの出力を貼り、照合のサブコマンドで確かめる**（§1.1）。AC-01・AC-06・AC-09・AC-14〜AC-18・AC-20・AC-21・AC-23 は、値を持つ記録を求めている。自然言語だけを定める AC ではないので、`manual` の読解だけでは検証にならない（`requirements_process.md` §4）。ADR は作らない。
4. **F-006 の判定（条件 (i)〜(iii)、手順 2 の選び方、手順 3 の除外）と B のモデルの選択（要件 F-002）を、集計のツールで計算する。** どちらも決定的な規則であり、主セッションの読み取りの誤りを避けるためである。設計 §3.7 の出す値に加えるもので、設計の決定は変えない。ADR は作らない。
5. **採点のサブエージェントは、読み取りの道具だけを持つエージェントの定義（`_eval/scorer-agent.md`）で起動する。** 設計 §3.6 の「ファイルを読む道具だけを与える」を、Claude Code のエージェントの定義の `tools` で表す。定義はリポジトリに置いてレビューし、採点の前にユーザーのエージェントのディレクトリ（`~/.claude/agents/`）へ写し、採点の後に消す。ADR は作らない。

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、設計 §1.4 に扱いが記録されている。

### 1.3. 既存コード調査結果

調査の基準はコミット `986449c6`（`origin/main`）である。下の行番号はこのコミットを指す。

| 対象 | 既存のものと、本タスクでの扱い |
|---|---|
| CLI の料金と使用量の行 | `yt2column: tokens: <stage> <origin> known input=… cache_read=… cache_write=… output=… <model-field>` と `cost:`・`cost-total:` の行の形は [cli.md](../../dev/guides/cli.md) の「トークンの使用量」「推定の料金」（`docs/dev/guides/cli.md:76-77`・`:162-163`）が定める。集計のツールはこの形を読む。エラーの番兵は文面から推測しない（設計 §3.7） |
| 単価と推定コスト | `internal/pricing/pricing.go:104` の `Estimate(model, usage, callTime)` は公開されている。単価の表 `rates` は公開されていない（`internal/pricing/rates.go:19`）。OpenAI のモデルは料金の条件 `llm.PricingContextStandard30m` を要する（`rates.go:134`・`:143`）ので、使用量は `llm.KnownUsageWithPricingContext`（`internal/llm/usage.go:86`）で作る。上限と補助の推定コストは `Estimate` で求め、単価を `_eval/` に写さない。AC-15 の単価の記録は `rates.go` の行を読むスクリプトで出す |
| 再開と必要な設定 | `cmd/yt2column/run.go:424-436` の `resumeNeed`。`--after generation` は生成の設定を、`--after shorten` は検証・推敲の設定だけを、`--after review` は何も要さない。設計 §3.1・§3.2・§3.4 の前提と一致する |
| 機械的な判定 | `docs/tasks/0008_column_prompt/check_article.py`（Python）。記事のフロントマターの形に依存するので、固定した CLI が書いた記事を読めることを実行の前の確認で確かめる（ステップ 1-4） |
| 0018 の道具と記録 | `~/yt2column-eval/0018/tools/` の zsh と Python のスクリプト（写さない。設計の決めたこと 2）。0018 のログは `~/yt2column-eval/0018/log/{B3g,B3,C03}/`。集計の正しさを確かめる既知の値は [0018 評価記録](../0018_article_quality/02_evaluation.md) §4.8 の表（B3 の 10 件の合計 3.615544 USD、平均 0.361554、最大 0.610696、所要時間の平均 1:53.4・最大 3:06.2。C03 の合計 3.572464 USD、所要時間の平均 1:52.1・最大 3:05.2） |
| 0077 の実測 | 見積もりと上限の入力のトークン数は、[0077 設計書](../0077_openai_llm_client/02_architecture.md) §1.4 の表の `roNZ-rLxhUE` の行である |
| リポジトリ内の Go の前例 | `docs/tasks/0018_article_quality/_material_check/main.go`。`go list ./...` は `_` で始まるディレクトリを含めない（`986449c6` で `go list ./... \| grep -c docs` が `0`）。パスを指定すれば `go list`・`go build`・`go test` の対象になる（`go list ./docs/tasks/0018_article_quality/_material_check` が成功） |
| 既定の LLM（F-007） | `internal/config/config.go:69-81` の `defaultProvider`・`defaultClaudeModel`・`defaultLLMValue`。継承は `loadLLM`（`config.go:508-537`）の `cfg.review = generation`（`:533`）。キーの検査は `loadProviderVars`（`:645`）。テスト用の関数は `internal/config/test_helpers.go` の `DefaultLLMValueForTest`・`DefaultClaudeModelForTest` で、使うのは `cmd/yt2column/docs_test.go:312` と `cmd/yt2column/run_test.go:1583` である。`defaultClaudeModel` を直接使うテストは `internal/config/config_test.go` の 1368・1407・1419・1438・1447・1543-1544・1564-1565・2195・2307 行である |
| 既定に依存するテスト（F-007） | 設計 §3.9.3 のテストはすべて `986449c6` にある（`config_test.go` の `TestLoadDefaults`:102・`TestLoadClaudeEffortDefault`:304・`TestLoadGenerationDefaults`:1358・`TestLoadReviewDefaults`:1399・`TestLoadDefaultProviderVariables`:1488・`TestLoadServerLLMDefaults`:1529・`TestLoadProductionDefaults`:1551・`TestLoadInheritsEffort`:1773、`serverfile_test.go:325` の `TestServerFileTemplate`、`run_test.go:1564` の `TestRunDefaultLLMConfig`、`docs_test.go:310` の `TestLLMDefaultsDocsContract`・`:699` の `TestProductionProfileDocsContract`）。設計 §3.9.3 にないもので、既定のキー（`ANTHROPIC_API_KEY`）だけで起動することを前提にしうるのは、`cmd/yt2column-server/run_test.go` の `TestStartupFailures`:47・`TestServerIgnoresEnvSettings`:488・`TestConfigInitThenStart`:1390 と、`cmd/yt2column/run_test.go:1541` の `TestRunClaudeConfigurationErrorsOneLineEach` である。どれを直すかは、選んだ構成で実行して決める（ステップ 6-1） |
| 既定を書いた文書（F-007） | `claude-opus-5-5` と `YT2COLUMN_REVIEW_LLM` を `docs/tasks/` の外で検索した結果（`986449c6`）は、設計 §3.9.2 の表の文書に加えて、`docs/translation_glossary.md`（`YT2COLUMN_REVIEW_LLM` の語の定義だけで、既定は書いていない）、`docs/dev/guides/claude_output_limit.md:46`（Claude のモデルの一覧で、既定ではない）、`testdata/README.md:123`（保存した応答のモデル）である。この 3 つは既定を書いていないので変えない。コードの検索結果のうち、アダプタと料金のテスト（`internal/llm/claude/`・`internal/pricing/`・`internal/llm/provider/provider_test.go`・`cmd/yt2column/cost_test.go`）は、テストの中のモデル名で、既定ではない |

改名するシンボルは、フェーズ 6 で `defaultClaudeModel` と `DefaultClaudeModelForTest` を変える場合だけである（設計 §3.9.2）。その台帳はステップ 6-1 に置く。

## 2. 実装手順 (Implementation Steps)

`_eval/` は `docs/tasks/0087_openai_combination_eval/_eval/` を、`0087/` は `~/yt2column-eval/0087/` を指す。`_eval/` のコメント・識別子・文字列は英語で書く（`CLAUDE.md`「Language」）。

### フェーズ 1：準備（API を呼ばない）

#### ステップ 1-1：集計・起動・採点の素材・見積もりの Go のツール

**追加ファイル:** `_eval/evaltool/` の Go のソースとテスト。

- [ ] `estimate` サブコマンドを作る。`_eval/bounds.json`（ステップ 1-2）の入力のトークン数から、設計 §3.5 の見積もりの表（構成ごとの 10 件の見積もりと、字数の調整の上乗せ）と、1 件の実行の上限（設計 §3.5「起動する実行の上限」の規則）を `Estimate` で求めて出す。
- [ ] `aggregate` サブコマンドを作る。設計 §3.7 の値（トークン数と推定コスト、所要時間、失敗した実行、手順から外れた実行、比較の表、使った額）を、`04_evaluation.md` に貼る Markdown の表として出す。加えて次を出す。
    - 承認の記録の時刻より前に始まった実行の一覧（AC-19）。
    - 記事ごとのやり直しの回数が 2 回を超えた記事の一覧（AC-17）。
    - B のモデルの選択（要件 F-002。C-astra と C-sol の合格件数。同数なら両方）。
    - F-006 の条件 (i)〜(iii) の判定、手順 2 の選び方、手順 3 で除いた候補を除いた選び直し。基準の値は `_eval/baseline.json`（ステップ 1-2）から読む。
    - 製品の制約 1〜3 が直ったかどうか（要件の決めたこと 13）。
- [ ] 判定のファイル（`0087/score/<段>/judgments.*`。主セッションが人の確認の後に書く）を読み、合格件数と判定の分布を数える。次のどれかに当たるファイルは拒否する。記事・観点・判定（解釈・文言）の欠け、観点「根拠」の合格で人の確認の印がないもの、製品の制約 1〜3 の判定で人の確認の印がないもの。
- [ ] `check-record` サブコマンドを作る。`04_evaluation.md` の中の、集計の表を囲む印（HTML のコメント）の間の表を、`aggregate` の出力と比べ、違う表の名前を出して失敗する。
- [ ] `launch` サブコマンドを作る。設計 §3.5 の 3 つの確かめ（承認の記録と構成、同じ OpenAI のモデルの直前の構成からの 30 分、使った額と起動する実行の上限の和）がすべて通った場合だけ、`_eval/run.sh` を 1 つの構成の 10 件（またはやり直しの 1 件）について並列に起動し、全件の終了を待つ。CLI のパスは引数で受け取り、`run.sh` に渡す。使った額は `aggregate` と同じ関数で求める（設計 §3.7「使った額の計算は 1 か所」）。
- [ ] `material` サブコマンドを作る。設計 §3.6 の手順 2 のとおり、記事と字数の調整の出力からタイトルと本文だけを取り出し、検証・推敲の一覧からモデル名の行を除き、段と動画ごとに構成を伏せた名前で乱順に並べる。名前と構成の対応は別のファイルに書く。
- [ ] `_eval/evaltool/` のテストを書く。外部のディレクトリを読まず、`t.TempDir` に作った合成のログ・`.when`・保存した出力・判定のファイルを使う。少なくとも次を確かめる。
    - `aggregate`: `known`・`unknown`・`reused` の行、`cost-total: run` の行がない実行の上限での計上、`reused` を使った額に数えないこと、識別子の不一致と構成 A の `cost: shorten run` の検出、支払い前の失敗（`origin` が `run` の `tokens:` の行がない）と支払い後の失敗の区別、所要時間の足し方（構成 A、roNZ-rLxhUE-1 の 54 秒を含む）、成功した実行だけの平均と最大。
    - F-006: 合格件数の差がちょうど 2 件と 1 件、推定コストの 3 倍と所要時間の 2 倍のちょうどの値と超える値、動画ごとの必須の観点の不合格の比べ方、得られなかった記事と不明の推定コストのある構成、手順 2 の同数の扱い、手順 3 の除外。
    - B のモデルの選択の同数。
    - 判定のファイルの拒否（欠けと人の確認の印のない行）。
    - `launch`: 承認の記録がない、構成が記録にない、上限を超える、30 分たっていない、のそれぞれで、スタブの CLI（引数を記録して終わるスクリプト）が一度も起動されないこと。すべて通る場合に 10 件が起動されること。
    - `material`: 出力にフロントマター、モデル名、構成 ID が含まれないこと。
    - `check-record`: 表が 1 つ違う場合に、その表の名前を出して失敗すること。
- [ ] 各テストを書いた後、確かめる処理を一時的に壊して失敗することを確かめ、コミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。

**完了条件:** `go test ./docs/tasks/0087_openai_combination_eval/_eval/...` と `go vet` が同じパスで成功する。

#### ステップ 1-2：実行のスクリプトと値のファイル

**追加ファイル:** `_eval/run.sh`・`_eval/hashlist.sh`・`_eval/retire.sh`・`_eval/rates.sh`・`_eval/bounds.json`・`_eval/baseline.json`・`_eval/scoring-prompt.md`・`_eval/scorer-agent.md`。

- [ ] `run.sh`（zsh）を作る。設計 §3.1 の構成の変数の表（ここだけに置く）、§3.2 の再開の工程、§3.3 の写しの作り方とハッシュの照合、出力先がすでにあれば起動しないこと、`.when` の内容を実装する。API キーの値を読まず、書かず、表示しない。
- [ ] `hashlist.sh`（zsh）を作る。設計 §3.3 のハッシュの一覧（相対パス・モード・SHA-256、`LC_ALL=C` の順）を作り、前の一覧と `diff` する。
- [ ] `retire.sh`（zsh）を作る。やり直しの前に、前の実行の出力を `0087/failed/<構成 ID>/<記事>-f<N>/` に移す（設計 §4「やり直しの手順」）。
- [ ] `rates.sh` を作る。評価に使うコミットの `internal/pricing/rates.go` から 3 つのモデルの行の単価を出し、`claude-opus-5-5` の行の単価を B3 の CLI のコミット `2fc2e2a` の同じ行と比べる（設計 §3.4 の項目 3、AC-15）。
- [ ] `bounds.json` に、見積もりと上限の入力（0077 の実測のトークン数、5 本のうち最も長い動画の固定の部分、アダプタの出力の上限、Claude の検証・推敲の B3 の 9 件の平均）と出典を置く。値の出典は設計 §3.5 である。
- [ ] `baseline.json` に、基準の構成の値（合格件数、記事 1 本の推定コストの平均、所要時間の平均、動画ごとの必須の観点の不合格の数、製品の制約 1〜3 の有無）と出典を置く。値は 0008 評価記録 §6.2・§6.3、0018 評価記録 §4.8 から写す。動画ごとの不合格の数は、設計 §3.4 の項目 4 の見込み（leE02uqgaKg が 1 件、ほかが 0 件）を、0008 評価記録 §6.2 の表から数え直して確かめる。
- [ ] `scoring-prompt.md`（英語）に、設計 §3.6 の手順 3 の 5 つの項目を書く。物差し 2 の定義は 0008 要件定義書 F-006 と 0018 評価記録 §5.5 を参照させ、写さない。
- [ ] `scorer-agent.md` に、Claude Code のエージェントの定義（`tools` は読み取りの道具だけ、モデルは `opus`、本文は `scoring-prompt.md` を読むことの指示）を書く。`tools` の書き方は Claude Code のサブエージェントの文書で確かめ、確かめた日を定義のコメントに書く。

**完了条件:** ステップ 1-4 の確かめで、各スクリプトが期待どおりに動く。

#### ステップ 1-3：評価記録の骨組みと見積もり

**追加ファイル:** `docs/tasks/0087_openai_combination_eval/04_evaluation.md`。

- [ ] 設計 §3.8 の節 1〜10 を作る。節 2（手順）には、スクリプトの名前と、設計 §3 の手順を具体化したコマンドを書く。記録の節は見出しと、集計の表を囲む印だけを置く。
- [ ] `estimate` の出力（見積もりの表と 1 件の上限）を節 5 に貼る。設計 §3.5 の概算の値と大きく違う場合は、その理由を書く。

#### ステップ 1-4：API を呼ばない確かめと、ビルドの固定

設計 §8 の 1 のうち、実行する確かめである。結果は `04_evaluation.md` の節 2〜4・6 に記録する。順序は設計 §6 の図（ビルドの固定 → ハッシュの一覧とバックアップ → 実行の前の確認）に従う。

- [ ] `_eval/selftest.sh` を作って実行する。`go test`（ステップ 1-1）と、`hashlist.sh` の確かめ（一時ディレクトリの 1 ファイルの内容またはモードを変えると `diff` が空でなくなる。設計 §7.1）を行う。
- [ ] `_eval/reproduce0018.sh` を作って実行する。`aggregate` を 0018 の B3・C03 のログと保存した出力に当て、0018 評価記録 §4.8 の値（§1.3 の表）が出ることを確かめる（設計 §7.1）。
- [ ] `_eval/repo_impact.sh` を作って実行する。マージベースと HEAD で、`go list ./...`、`make test` が実行するテストの名前の一覧、`make lint` の指摘が同じであることを確かめる（設計 §7.3、要件 §4.5）。
- [ ] CLI のビルドを固定する（設計 §3.1）。未コミットの変更のない作業ツリーで `make build` を 1 回行い、`0087/bin/<コミット>/` に写し、SHA-256 と main から到達できるコミットを記録する。
- [ ] `hashlist.sh` で最初のハッシュの一覧を作り、バックアップを作って展開して一覧と照合し、アーカイブのハッシュを記録する（設計 §3.3）。
- [ ] 設計 §3.4 の項目 1〜4 を行う。項目 2 は、構成ごと・記事ごとに `run.sh` を API キーを取り除いて（`env -u`）固定した CLI で起動し、出力を `0087/preflight/` に置く。すべての起動で API キーを取り除く。
- [ ] `check_article.py` を、項目 1 の `--after review` が書いた記事に当て、今の CLI の記事を読めることを確かめる。
- [ ] `scorer-agent.md` を `~/.claude/agents/` に写し、項目 1 の記事を 1 件だけ伏せた素材にして試しに採点させ、出力の表の形を確かめる。採点の結果は評価に使わない。写した定義は採点の後に消すことを節 2 に書く。
- [ ] `_eval/secretscan.sh` を作って実行する。環境にある API キーの値と `sk-` の接頭辞を、`_eval/` と `04_evaluation.md` の内容と比べ、値を表示せずに有無だけを出す（設計 §7.4）。

**完了条件:** 上の確かめがすべて成功し、出力が記録されている。CLI は API を一度も呼んでいない（項目 2 の起動はすべて終了コード `2`）。

### PR-1 作成ポイント: evaluation tooling and preflight

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `docs(0087): 評価のスクリプトと評価記録の骨組みを加え、API を呼ばない確かめを記録する`

**レビュー観点**: 起動の判定と費用の上限の計算 / 基準の構成の保存した出力を書き換えない写し方 / 採点の素材から構成が分からないこと / API キーと全文がリポジトリに入らないこと

**実装モデル要件**: frontier-required

**判定理由**: 1-1 の `launch` が実 API の費用を承認の範囲に止める唯一の関門で、外部のサービスへの支払いを守るセキュリティの関門に当たる。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2：一括の承認

#### ステップ 2-1：見積もりを示して承認を受ける

- [ ] `04_evaluation.md` 節 5 の見積もりの表（1 段目と 2 段目の構成・回数・推定コスト）と上限 30 USD を示し、人の承認を受ける（要件 F-005）。
- [ ] 承認の日・構成・上限を `0087/approval` に書き、`04_evaluation.md` 節 5 に記録する。

**完了条件:** 承認の記録がある。この時点まで、API キーを渡した CLI の起動はない。

### フェーズ 3：1 段目

#### ステップ 3-1：4 構成を実行する

- [ ] `launch` で `C-sol` → `C-astra` → `A-sol` → `A-astra` の順に、1 構成ずつ実行する（設計 §3.5）。`launch` が止まった場合は、その理由を人に示す。上限による停止では、人の承認を改めて受けて `0087/approval` を書き直す。
- [ ] 失敗した実行は、設計 §4 の表で扱いを決める。やり直す場合は `retire.sh` の後に `launch` で 1 件を起動する。番兵は主セッションが標準エラーを読んで記録し、人が確認する。
- [ ] 1 段目の後に `hashlist.sh` で一覧を作り、最初の一覧との `diff` が空であることを記録する。

#### ステップ 3-2：採点と人の確認

- [ ] `check_article.py` で長さ・節の数・タイトルの字数を判定する。
- [ ] `material` で 1 段目の伏せた素材を作り、`scorer-agent.md` のエージェントで、動画ごとに 1 つのサブエージェントが採点する（設計 §3.6 の手順 3）。採点の全文は `0087/score/1/` に置く。
- [ ] 採点を構成ごとの表に戻し、設計 §3.6 の手順 4 の項目を人に確認してもらう。人が変えた判定と理由を記録し、判定のファイルを書く。
- [ ] `aggregate` の出力を `04_evaluation.md` 節 7 に貼り、不合格・品質の 1 の引用、製品の制約 1〜3 の引用、番兵、判定者を書く。

**完了条件:** 1 段目の 4 構成の判定が人の確認を終え、記録されている。

### フェーズ 4：2 段目

#### ステップ 4-1：B を作って採点する

- [ ] `aggregate` が出す B のモデルの選択を `04_evaluation.md` 節 8 に記録する。
- [ ] `launch` で `B-<モデル>` を実行する（同数なら `B-sol` → `B-astra`）。失敗の扱いはステップ 3-1 と同じである。
- [ ] 2 段目の後に `hashlist.sh` で一覧を作り、`diff` が空であることを記録する。
- [ ] ステップ 3-2 と同じ手順で採点し、人の確認を受け、節 8 に記録する。
- [ ] 採点がすべて終わった後、`~/.claude/agents/` に写したエージェントの定義を消す。

**完了条件:** B の各構成の判定が人の確認を終え、記録されている。

### フェーズ 5：比較と判断

#### ステップ 5-1：比較の表と F-006 の判断

- [ ] `aggregate` の比較の表、使った額、F-006 の判定を、`04_evaluation.md` 節 9・10 に貼る。プロンプトのキャッシュから読んだ入力のある構成は、補助の推定コストの平均で F-006 の判定が変わるかを書く（設計 §3.5）。
- [ ] F-006 の手順 2 で選んだ構成（候補がなければ「変えない」）を、失敗した実行の回数と費用とともに人に示し、承認を受ける。人が承認しない場合は、その候補を除いて `aggregate` で選び直す（要件 F-006 の手順 3）。
- [ ] 結論と節 1 の要約を書く。
- [ ] `check-record` を実行し、出力を本書 §7 の検証の表に記録する。
- [ ] `secretscan.sh` を実行し、出力を記録する。
- [ ] 本番の構成を変えない場合は、フェーズ 6 と PR-3 のチェックボックスをすべて `[-]` にし、理由として `04_evaluation.md` の結論を参照する。AC-24〜AC-30 の検証の行に「該当しない」と書く。

**完了条件:** 評価記録の結論が人の承認を受け、`check-record` が成功している。

### PR-2 作成ポイント: evaluation runs and decision

**対象ステップ**: 2-1 / 3-1 / 3-2 / 4-1 / 5-1

**推奨タイトル**: `docs(0087): Claude と OpenAI の組み合わせの評価を記録し、本番の構成を判断する`

**レビュー観点**: 評価記録の値が集計の出力と一致すること / 人が確認した判定と変更の理由 / F-006 の判断と人の承認 / 引用が短く全文がないこと

**実装モデル要件**: frontier-required

**判定理由**: 3-1・4-1 が実 API を呼ぶ有料の実行を含み、外部の資源に重く触れる。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6：本番の構成の変更（変える場合だけ）

選んだ構成が C か、A・B かで範囲が分かれる（設計 §3.9 の表）。A・B は R1（設計 §1.2 の決めたこと 1、§3.9.1）を入れる。

#### ステップ 6-1：既定の構成と継承の規則

**変更ファイル:** `internal/config/config.go`・`config_test.go`・`serverfile.go`・`serverfile_test.go`・`test_helpers.go`、`cmd/yt2column/run_test.go`、`cmd/yt2column-server/run_test.go`（ステップの中で要否を決める）。

- [ ] 設計 §3.9.2 の表のとおり既定の定数を変える。C・B では、次の改名の台帳に従う。台帳の新しい名前は実装で決め、本書の台帳を同じコミットで埋める。

    | 旧い名前 | 新しい名前 | 場所 |
    |---|---|---|
    | `defaultClaudeModel` | （プロバイダによらない名前） | `internal/config/config.go:76` と §1.3 の `config_test.go` の行 |
    | `DefaultClaudeModelForTest` | （同上に合わせる） | `internal/config/test_helpers.go`、`cmd/yt2column/run_test.go:1583` |

- [ ] A・B では、検証・推敲の既定の定数を加え、`loadLLM` に R1 の分岐を加える。`parseLLM` を通す。`serverfile.go` の雛形の `YT2COLUMN_REVIEW_LLM` の「When unset」の説明を R1 と代償に合わせる。テスト用に検証・推敲の既定を返す関数を `test_helpers.go` に加える。
- [ ] 設計 §3.9.3 のテストと §1.3 の追加の 4 つのテストを新しい既定で実行し、失敗するものを直す。A・B では、R1 の 3 つの分岐と、選んだ構成のプロバイダのキーだけを必須にすることを、`LoadFor` と `LoadServer` の両方で確かめるテストを加える。
- [ ] 直したテストと加えたテストが、既定の定数（と A・B では R1 の分岐）を元に戻すと失敗することを確かめ、コミットメッセージに書く。

#### ステップ 6-2：文書・ガードテスト・ADR

**変更ファイル:** 設計 §3.9.2 の文書の表の文書、`cmd/yt2column/docs_test.go`、`internal/config/test_helpers.go`。

- [ ] `claude-opus-5-5` と `YT2COLUMN_REVIEW_LLM` を `docs/tasks/` の外で検索し直し、§1.3 の結果と設計 §3.9.2 の表に照らして、変える箇所の一覧を作る。一覧はコミットメッセージに書く。
- [ ] `docs/dev/production_profile.md`・`docs/dev/project_overview.md`・`docs/dev/security.md`・`docs/dev/conventions/package_reference.md` を、設計 §3.9.2 の表と §3.9.4 のとおりに変える。A・B では `docs/dev/guides/cli.md` も変える。値は評価記録から写し、出典として参照する。
- [ ] `README.md` を変え、同じコミットで `/mktrans README.md` で `README.en.md` を同期する。
- [ ] A・B では、`TestLLMDefaultsDocsContract` を `YT2COLUMN_REVIEW_LLM` の既定の値も照合するように広げ、`TestProductionProfileDocsContract` の `productionProfileVars` に `YT2COLUMN_REVIEW_LLM` を加える。`docsProfileEnv` に OpenAI のキーが要るかを、テストを実行して確かめる（設計 §3.9.3）。
- [ ] ADR-0004 を更新し、索引の題名を合わせる（AC-28）。A・B では ADR-0003 を更新する（設計 §1.2 の決めたこと 1 の「ADR の判断」）。どちらも [adr.md](../../dev/conventions/adr.md) に従う。
- [ ] `make test`・`make lint`・`make ext-test`・`make readme-sync-check` を実行する。

**完了条件:** 上の 4 つの make ターゲットが成功する。

### PR-3 作成ポイント: production profile change

**対象ステップ**: 6-1 / 6-2

**推奨タイトル**: `feat(0087): 本番の構成を評価で選んだ構成に変える`

**レビュー観点**: 未設定のときの検証・推敲の選択（R1）と必須のキー / 既定を書いた文書とガードテストの漏れ / ADR-0004・ADR-0003 の決定と採らなかった案

**実装モデル要件**: frontier-recommended

**判定理由**: 6-1 がすべての利用者の既定の送り先と必須のキーを変える、独立した高リスクのステップである。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序

設計 §8 の順に、準備 → 一括の承認 → 1 段目 → 2 段目 → 比較と判断 → 本番の構成の変更と進める。

| マイルストーン | 成果物 |
|---|---|
| M1（PR-1 のマージ） | `_eval/` の道具、`04_evaluation.md` の骨組みと見積もり、API を呼ばない確かめの記録 |
| M2（フェーズ 3 の終わり） | 1 段目の 4 構成の記録と人の確認 |
| M3（PR-2 のマージ） | 評価記録の完成と、本番の構成の判断 |
| M4（PR-3 のマージ。変える場合だけ） | 新しい既定と、それを書いた文書・ADR |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | 評価のスクリプトと評価記録の骨組み、API を呼ばない確かめ | frontier-required |
| PR-2 | 2-1 / 3-1 / 3-2 / 4-1 / 5-1 | 評価の実行・採点・記録と、本番の構成の判断 | frontier-required |
| PR-3 | 6-1 / 6-2 | 本番の構成の変更（変える場合だけ） | frontier-recommended |

PR-2 は複数のセッションにわたる。`/runplan` は 1 フェーズずつ進め、PR-2 の作成ポイントまで同じブランチに積む。

## 4. テスト戦略 (Test Strategy)

- **`_eval/` のテスト:** `_eval/evaltool/` の Go のテストは、合成の入力だけを使い、`t.TempDir` の外を読まない。`make test` には入らない。`_eval/selftest.sh` がパスを指定して実行する。
- **既知の値との照合:** 集計の足し方は、`reproduce0018.sh` で 0018 の記録の値を再現して確かめる。
- **API を呼ばない起動:** `run.sh` と CLI の組み合わせは、API キーを取り除いた実際の起動で確かめる（設計 §3.4 の項目 2）。同じ CLI を何度も起動するので、すべての起動で同じ条件（API キーを取り除く、出力を `preflight/` に置く）をそろえる。
- **リポジトリへの影響:** `repo_impact.sh` で、`go list ./...`・`make test` のテストの名前・`make lint` の指摘が変わらないことを確かめる。
- **F-007:** 既存のテストを新しい既定で直し、A・B では R1 のテストを加える。直したテストが既定を戻すと失敗することを確かめる。
- **後方互換:** F-007 で変える既定について、版の間の互換の仕組みは設けない（設計 §3.9.4）。利用者に見える変化は README と `production_profile.md` に書く。
- 各 PR の完了時に `make test`・`make lint` を実行する。Go を編集した場合は先に `make fmt` を実行する。PR-3 では `make ext-test` も実行する。既存のテストは削除しない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 承認の範囲を超えて費用がかかる | `launch` が起動の前に予約の上限を確かめ、超えるなら止まる（ステップ 1-1）。上限の前提（1 トークンが 1 バイト以上）が外れる危険は評価記録に書く（設計 §3.5） |
| 基準の構成の保存した出力を書き換える | 写しだけを CLI に渡し、ハッシュの一覧を 3 回比べ、バックアップを持つ（ステップ 1-2・1-4・3-1・4-1） |
| 10 件の並列でレート制限（HTTP 429）が起きる | 工程の失敗としてやり直し、内容による失敗と分けて記録する（設計 §3.5）。所要時間に注記する |
| 採点が構成やプロバイダの名前に引きずられる、素材の指示に操られる | 伏せた素材と、読み取りの道具だけのエージェントで採点し、人が不合格と「根拠」の合格を確認する（ステップ 3-2） |
| 評価記録への書き写しの誤り | 集計の出力を貼り、`check-record` で照合する（ステップ 5-1） |
| 評価が複数日にわたり、OpenAI のモデルの振る舞いが変わる | 実行の日時を記録し、比べるときに併せて示す（要件 §5） |
| F-007 の既定の変更で、`ANTHROPIC_API_KEY` だけの利用者が止まる | 閉じる側に倒れる振る舞いとして README に書く（設計 §3.9.4）。PR-3 を独立させ、戻すときは PR-3 のコミットを戻す |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 3-1 / 3-2 / 4-1 / 5-1）
- [ ] PR-3 マージ済み、または本番の構成を変えないため該当なし（対象ステップ: 6-1 / 6-2）
- [ ] §7 の表のすべての AC の検証結果を記録した

## 7. 受け入れ条件の検証 (Acceptance Criteria Verification)

型は、`test` がテストの実行、`static` が委ねたスクリプトまたはサブコマンドの実行、`manual` が PR のレビューでの読解である。`_eval/` のテストとスクリプトは `make test` に入らず、記録したステップで実行する。

- `test: evaltool` は `go test ./docs/tasks/0087_openai_combination_eval/_eval/evaltool/...`（`_eval/selftest.sh` が実行する）を指す。
- `static: check-record` は、ステップ 5-1 で `check-record` を `04_evaluation.md` に当てた実行を指す。出力と、実行したコミットを下の「記録」に書く。
- 評価の記録の AC（AC-01〜AC-23）の「判定者や人の確認が記録されている」部分は、人の行為の記録なので `manual` で読む。

| AC | ステップ | 型と実行する検証 |
|---|---|---|
| AC-01 | 1-4 / 5-1 | static: `check-record`（基準の構成の識別子と日付の表）。manual: 判定の出典と実行の条件の違いの記述 |
| AC-02 | 1-4 / 3-1 / 4-1 | static: `_eval/hashlist.sh` のキャッシュの一覧の `diff`（3 回、`04_evaluation.md` 節 4 に記録）。manual: キャッシュの場所と取得日の記述 |
| AC-03 | 1-1 / 3-1 / 5-1 | test: evaltool（識別子の不一致の検出）。static: `check-record`（構成 A の識別子の表と、手順から外れた実行の一覧が空であること） |
| AC-04 | 1-4 / 3-1 / 4-1 | test: `_eval/selftest.sh`（`hashlist.sh` の変更の検出）。static: `_eval/hashlist.sh` の基準の構成の一覧の `diff`（3 回、節 4 に記録） |
| AC-05 | 1-1 / 3-1 / 4-1 | test: evaltool（支払い前の失敗の区別）。manual: 拒否された入力の記録（拒否がなければ「なし」の記録） |
| AC-06 | 1-2 / 1-4 / 5-1 | static: 設計 §3.4 の項目 2 の起動の `.when`（節 2 に記録）、`check-record`（実行の表） |
| AC-07 | 1-1 / 3-2 / 5-1 | test: evaltool（判定のファイルの欠けの拒否）。static: `check-record`（1 段目の 4 構成の判定の表） |
| AC-08 | 1-1 / 4-1 / 5-1 | test: evaltool（B のモデルの選択と同数）。static: `check-record`（選択の表と B の判定の表）。manual: 1 段目の人の確認の日と B の実行の開始の日時の記録 |
| AC-09 | 1-1 / 3-2 / 4-1 / 5-1 | test: evaltool（合格件数の数え方）。static: `check-record`（両方の判定の表と合格件数） |
| AC-10 | 3-2 / 4-1 | static: `check-record`（判定者の列を持つ判定の表）。manual: 採点に使ったモデル名、人が変えた判定と理由 |
| AC-11 | 1-1 / 3-2 / 4-1 | test: evaltool（人の確認の印のない「根拠」の合格の拒否）。manual: 人の確認の記録 |
| AC-12 | 1-1 / 3-2 / 4-1 | static: `check-record`（引用が要る記事と観点の一覧の表）。manual: 引用と、誤りを入れた工程・直さなかった工程 |
| AC-13 | 1-1 / 3-2 / 4-1 | test: evaltool（人の確認の印のない制約の判定の拒否）。static: `check-record`（制約 1〜3 の表）。manual: 引用 |
| AC-14 | 1-1 / 5-1 | test: evaltool（`known`・`unknown`・`reused` の行）。static: `check-record`（トークン数と推定コストの表） |
| AC-15 | 1-2 / 1-4 | static: `_eval/rates.sh`（出力を節 6 に記録） |
| AC-16 | 1-1 / 1-4 / 5-1 | test: evaltool（所要時間の足し方）。static: `_eval/reproduce0018.sh`（節 2 に記録）、`check-record`（所要時間の表） |
| AC-17 | 1-1 / 3-1 / 4-1 / 5-1 | test: evaltool（失敗の集計とやり直しの回数）。static: `check-record`（失敗した実行の表と、やり直しが 2 回を超えた記事の一覧が空であること）。manual: 番兵と設定の誤りの分け方 |
| AC-18 | 1-1 / 5-1 | test: evaltool（成功した実行だけの平均と最大）。static: `check-record`（比較の表） |
| AC-19 | 1-1 / 2-1 / 5-1 | test: evaltool（`launch` が承認の記録なしでスタブを起動しないこと）。static: `check-record`（承認より前に始まった実行の一覧が空であること）。manual: 示した見積もりと承認の日の記録 |
| AC-20 | 1-1 / 5-1 | test: evaltool（使った額の数え方と、上限を超えるときの `launch` の停止）。static: `check-record`（使った額の表） |
| AC-21 | 1-1 / 1-2 / 5-1 | test: evaltool（F-006 の条件の境界）。static: `check-record`（F-006 の判定の表）。manual: `baseline.json` の動画ごとの不合格の数と 0008 評価記録 §6.2 の照合 |
| AC-22 | 1-1 / 5-1 | test: evaltool（手順 2 の選び方と手順 3 の除外）。manual: 人の承認、示した失敗の回数と費用、除いた候補と理由 |
| AC-23 | 1-1 / 5-1 | test: evaltool（制約が直ったかの判定）。static: `check-record`（制約が直ったかの表）。manual: 基準の構成の値と 0008 評価記録 §6.3 の照合 |
| AC-24 | 6-1 | test: `internal/config/config_test.go::TestLoadProductionDefaults`・`TestLoadDefaults`・`TestLoadServerLLMDefaults`、`cmd/yt2column/run_test.go::TestRunDefaultLLMConfig`。A・B では R1 の新しいテスト（ステップ 6-1 で名前を決め、ここに書く） |
| AC-25 | 6-1 | test: `internal/config/config_test.go::TestLoadDefaultProviderVariables`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（ステップ 6-1 で、足りないキーの名前と終了コードを確かめる行があることを確かめる） |
| AC-26 | 6-2 | static: `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`・`TestProductionProfileDocsContract`、`internal/config/serverfile_test.go::TestServerFileTemplate`。manual: `production_profile.md` の品質・制約・比較の値と評価記録の照合 |
| AC-27 | 6-2 | manual: `project_overview.md` のモデル名の例外の項目（自然言語の内容だけを定める） |
| AC-28 | 6-2 | static: `cmd/yt2column/adr_test.go::TestADRRecords`。manual: ADR-0004 の決定・採らなかった案・経緯 |
| AC-29 | 6-2 | manual: README の両版と `security.md` の送り先の記述（自然言語の内容だけを定める） |
| AC-30 | 6-2 | static: `make test`・`make lint`・`make ext-test`・`make readme-sync-check` |

AC-24〜AC-30 は、本番の構成を変える場合だけ行う。変えない場合は、ステップ 5-1 で各行に「該当しない」と結論への参照を書く。

**記録:**（`/runplan` が `check-record` の出力と、実行したコミットを書く）

## 8. 完了条件 (Success Criteria)

- [ ] AC-01〜AC-23 の検証が完了し、評価記録の結論が人の承認を受けている。
- [ ] 実 API を呼んだ実行の推定コストの合計が、承認の上限以下である。
- [ ] 基準の構成の保存した出力と評価用のキャッシュが、評価の前後で変わっていない。
- [ ] `_eval/` と `04_evaluation.md` に API キーと全文がない（`secretscan.sh` とレビュー）。
- [ ] 本番の構成を変える場合は、AC-24〜AC-30 の検証が完了している。

## 9. 次のステップ (Next Steps)

本書が承認された後、`/runplan 0087` で PR-1 から進める。PR-1 のマージの後、フェーズ 2 で一括の承認を受ける。OpenAI 向けのテンプレートの調整の余地があると判断した場合は、別の issue にする（要件の決めたこと 4）。
