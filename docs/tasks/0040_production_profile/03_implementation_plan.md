# 実装計画書：本番の構成と品質を共通の文書にまとめ、文書の主経路をブラウザ拡張に改める

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-26 |
| Review date | 2026-10-26 |
| Reviewer | isseis |
| Comments | 2026-10-26: ステップ 3-3 の壊し方の記述を正した。`# x` の行を持つコードブロックを Quick start の途中に入れたときは、節がそこで終わらず、テストは通る（切り出しが壊れたときは補助の単体テストが失敗する）。決定は変えない（編集上の修正）。 |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.2」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `59e1650` で確かめた。`file:line` はこのコミットの行番号を指す。設計書が引く行番号はコミット `f87f2a1` のもので、`CLAUDE.md` の 310 行より後ろは、その後の変更で 3 行ずれている（たとえば、設計書の `CLAUDE.md:324-328` は、本書では `CLAUDE.md:327-331`）。README・`project_overview.md`・`internal/config`・`cmd/yt2column` の行番号は、2 つのコミットで同じである。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次の 3 つを行う。

-   共通の文書 `docs/dev/production_profile.md` を作る（F-001）。
-   README・`project_overview.md`・`CLAUDE.md`・`security.md`・`prompts/README.md`・0008 評価記録・用語集を、主経路を先に書く形に改め、共通の文書を参照させる（F-002〜F-005）。
-   文書の値と構造を確かめるテスト (a)〜(c)（設計書 6.2）と、変更の前後の関係を確かめる差分の確認のスクリプト（設計書 7.2）を加える。

コードの振る舞いは変えない（要件書 §2.3）。

### 1.2. 実装の方針

-   **文面は設計書の各節に従って書き、本書は繰り返さない。** 各文書の書き直しの範囲と内容は設計書 3.1〜3.9 が定めている。本書のステップは、どの節に従って何を書くかと、どう確かめるかだけを示す。
-   **設計書 3.6 の制約を、README と `CLAUDE.md` のすべての編集に当てる。** README に新しい表を置かない。新しいコードブロックに `#` で始まる行を書かない。新しい見出しは `## Quick start` と `### Recommended configuration` の 2 つだけにする。既存の本文から、テストが読む値（環境変数の名前・`make` のターゲット・拡張の ID・`16,383` など）を消さない。
-   **評価の数値は写すだけにする。** 件数・金額・頻度は、0008 評価記録・0018 評価記録の値を丸めも言い換えもせずに写し、出典の節を示す（要件書 §4.5）。
-   **テストの補助は既存のものを使う。** `readDoc`・`sectionByHeading`・`findTable`・`tableRowsWithFirstCell`・`lookupFrom` と、`internal/config` の `...ForTest` 関数を使う（1.4）。
-   **すべてを 1 つの PR にする**（設計書 8 章）。フェーズは作業とコミットの単位で、PR の境界ではない。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと差分の確認のスクリプトは、設計書 7.1・7.2 が挙げる壊し方で失敗することを実装のときに確かめ、その結果をコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。
-   **Go のソースは英語で書く。** コメント・識別子・文字列リテラルに日本語を書かない。例外は、日本語の文書の見出しや表のセルと一致しなければならないテストのリテラル（`## 本番の構成`・`生成`・`字数の調整`・`検証・推敲`・`## 決定済みの方針`、差分の確認のスクリプトの `## 設定（環境変数）`）だけである。既存のテストにも同じ例がある（`cmd/yt2column/docs_test.go:131` の `"変数"`・`"未設定のとき"`）。`runplan.md` 6.5 の非 ASCII の検査はこれらの行（スクリプトの行を含む）を挙げるので、レビューではこれらを意図したものとして確かめる。差分の確認のスクリプトでは、判定の名前に AC の番号を使わない（`runplan.md` 6.5 の検査が `AC-NN` を拒む）。スクリプトは、0008 評価記録の §6 と `security.md` の §1 以降を、ASCII の見出しの接頭辞（`## 6.`・`## 1.`）と表の行の接頭辞（`| Status |` など）で探す。

### 1.3. 計画の際に決めたこと（2026-10-26）

設計書が決めずに残した点は、どれも人間のレビュアーに諮るほどの違いを生まないので、理由とともにここに記す。

-   **フェンスを区別する走査の補助は `cmd/yt2column/docs_test.go` に置く（設計書 3.11）:** `sectionByHeading`・`headingLevel` など、文書を読む補助がすでにこのファイルにあるためである。`firstCodeBlockAfter` は `output_layout_docs_test.go` に残し、行の走査だけを新しい補助に任せる。新しいファイルは作らない。
-   **テスト (a) は、表のプロバイダのセルを `LoadServer` に通して比べる:** `config.Provider` は文字列に戻す関数を持たない（`internal/config/config.go:18-28` の `type Provider int` は `String` などのメソッドを持たない）。`DefaultProviderValueForTest`（`internal/config/test_helpers.go:53`）は既定の文字列を返すが、設計書 6.2 は `LoadServer` が読み込んだ構成と比べることを求める。新しい `...ForTest` 関数を足すと、設計書 3.10 の 4 つの関数から外れる。そこで、表の 1 行のプロバイダ・モデル・effort のセルの値を、生成の 3 つの変数として閉じた map（テストが与える変数だけを持ち、プロセスの環境変数を読まない map）に入れ、`LoadServer` で読む。得られた生成の構成を、ブロックの変数を加えずに読み込んだ構成の同じ段階の値と比べる。プロバイダ以外のセルも同じ方法で比べ、比べ方を 1 つにする。
-   **差分の確認のスクリプトは Go で書く:** フェンスの行で状態を切り替える処理は、シェルより Go のほうが確かに書ける。Go はプロジェクトの開発環境にあり、新しい依存を加えない。`docs/tasks/0040_production_profile/_checks/main.go`（`package main`）に置き、`go run ./docs/tasks/0040_production_profile/_checks/` で実行する。名前が `_` で始まるディレクトリは `go list ./...` の対象外なので、`make lint`・`make deadcode` の対象にならない。コミット `59e1650` で、このディレクトリに置いた `package main` が `go run` で実行でき、`go list ./... | grep -c _checks` が `0` になることを確かめた（確かめた後、ファイルは消した）。ただし、`internal/config/envaccess_test.go` の `TestEnvAccessConfined` は、リポジトリ全体を `filepath.WalkDir` で走査し、`_` で始まるディレクトリも本番のコードとして読む（`internal/config/envaccess_test.go:91-125`）。そのため、スクリプトは環境変数を読む関数（`os.Getenv`・`os.Environ`・`(*exec.Cmd).Environ` など）と秘密情報の変数名を参照してはならない。同じ置き方の前例に `docs/tasks/0018_article_quality/_material_check/main.go` がある。
-   **`firstCodeBlockAfter` は、見出しを見つけた後の行だけをフェンスを区別する走査に渡す（設計書 3.11）:** 文書の最初からフェンスを数えると、見出しと同じ行が前のコードブロックの中にある場合や、閉じていないフェンスがある場合に、結果が変わる。見出しは今と同じく行を順に比べて探し、見出しの後の行だけを新しい補助で走査すれば、振る舞いは変わらない。
-   **差分の確認に、`project_overview.md` の「設定（環境変数）」の節が変わらないことの判定を加え、AC-20 の確かめ方に足す:** 設計書 7.2 の 3 つの判定に加える 4 つ目である。AC-20 は表の変数名が残ることを求めるが、既存のテストが読むのは表のうち 3 つの変数の行と、`YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` の行だけである。設計書 3.3 はこの節を変えないと定めているので、判定は設計と矛盾しない。
-   **フェーズは設計書 8 章の順序に合わせて 7 つに分け、PR は 1 つにする:** 設計書 8 章の 8 つの手順のうち、最後の 2 つ（差分の確認と、`make test`・`make ext-test`・`make lint`）を、PR の前の最後のフェーズにまとめる。

### 1.4. 既存コード調査結果

**再利用するテストの補助（`cmd/yt2column`）**

| 補助 | 場所 | 使い道 |
|---|---|---|
| `readDoc` | `cmd/yt2column/docs_test.go:231` | 文書を読む |
| `sectionByHeading`・`headingLevel` | `cmd/yt2column/docs_test.go:382`・`:361` | テスト (a)・(c) の節の切り出し。コードブロックの中の `#` 行でも節が終わるので、テスト (b) では使わない（設計書 6.2） |
| `findTable`・`tableRowsWithFirstCell`・`stripBackticks` | `cmd/yt2column/docs_test.go:295`・`:323`・`:353` | テスト (a) の「本番の構成」の表 |
| `firstCodeBlockAfter` | `cmd/yt2column/output_layout_docs_test.go:59-80` | フェンスを区別する走査を共有の補助にした後も、振る舞いが変わらないこと。`TestFirstCodeBlockAfter`（`:129`）と `TestOutputLayoutDocsContract`（`:84`）が確かめる |
| `lookupFrom` | `cmd/yt2column/test_helpers_integration.go:103-108`（`//go:build test \|\| integration`） | テスト (a) の閉じた map |
| パスの定数 | `cmd/yt2column/docs_test.go:22-29`、`claudePath`（`cmd/yt2column/extension_docs_test.go:26`） | 共通の文書と `prompts/README.md` のパスを加える。`CLAUDE.md` は `claudePath` を使う |

**`internal/config`**

-   `internal/config/test_helpers.go`（`//go:build test`）に、`SlackWebhookEnvForTest`（`:16`）・`DefaultProviderValueForTest`（`:53`）などがある。変数名の定数は `internal/config/config.go:31-44` にある。足りないのは、`providerEnv`・`modelEnv`・`claudeEffortEnv`・`anthropicAPIKeyEnv` を返す関数で、設計書 3.10 の 4 つを加える。
-   `LoadServer`（`internal/config/config.go:282`）は、LLM の設定・`SLACK_WEBHOOK_URL`・`HOME` を読み、拒否をまとめて返す。`HOME` は絶対パスであればよく、存在は確かめない（`internal/config/cachedir.go:74-80`、`internal/config/socketpath.go:49-63`）。ソケットのパスの長さの上限があるので、テスト (a) の `HOME` には短い固定の絶対パスを使う（設計書 6.2）。
-   結果の値は、`LLMConfig` の `Provider()`・`Model()`・`ReviewProvider()`・`ReviewModel()`・`ClaudeEffort()`（`internal/config/config.go:140-185`）で得る。`Provider` は比べられる整数の型で、文字列に戻す関数はない（1.3）。
-   字数の調整は生成のクライアントで行う（`CLAUDE.md:149-152`）。そのため、テスト (a) は「本番の構成」の表の「字数の調整」の行を、生成の構成と比べる。

**新しく加えるもの**

-   テスト (a)〜(c) と、フェンスを区別する走査の補助、フェンスを区別する節の切り出し、見出しのアンカーの補助、リンクの取り出しの補助（`cmd/yt2column/docs_test.go`）。テスト (a)・(c) が切り出す節（`## 本番の構成`・`### Recommended configuration`・`## 決定済みの方針`・`## Quick Links`）は、`#` で始まる行を持つコードブロックを含まないので、設計書 6.2 のとおり既存の `sectionByHeading` を使う。
-   `internal/config/test_helpers.go` の 4 つの関数。
-   差分の確認のスクリプト（`docs/tasks/0040_production_profile/_checks/main.go`）。リポジトリに `_checks/` はまだない（同じ置き方の前例は 1.3）。

**既存のテストへの影響（変えないもの）**

設計書 3.6 の表のテストは、検査の内容を変えない。本書のステップが 1.2 の制約を守れば、どれも変えずに通る。`project_overview.md` の値を読むテスト（AC-20）は次の 4 つで、書き直す箇所（冒頭・`:10`・`:11`・`:12`・`:20-22`・`:26`・`:47`）はどれも、これらが読む行（`:36`・`:52`・`:60`・`:109-112`・`:124`）と重ならない（設計書 3.3）。

-   `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（サブテスト `project_overview.md`。`:109-114`）
-   `cmd/yt2column/docs_test.go::TestCostDocsContract`（`costPrefix`）
-   `cmd/yt2column/docs_test.go::TestSlackDocsContract`（`16,383`）
-   `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`（表の既定の値）

**型の構成の経路:** 本タスクは、具象の型で振る舞いを分ける設計を持たないので、該当しない。

**改名:** 本タスクは識別子・ファイル・テストを改名しない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 共通の文書

#### ステップ 1-1: `docs/dev/production_profile.md` を作る

**変更するファイル:** `docs/dev/production_profile.md`（新規）

-   [x] 設計書 3.1 の表のとおり、題「本番の構成と品質」、冒頭の段落、5 つの `##` の節（`## パイプライン`・`## 本番の構成`・`## 品質の目標と結果`・`## 製品の制約`・`## 構成の比較と、構成を決めた理由`）を、この順序と文言で書く。
-   [x] 「本番の構成」の節に、設計書 3.1 の 5 列の表（段階のセルは「生成」「字数の調整」「検証・推敲」、値はコードの書式）と、設計書 3.2 の形の構成のブロック（`sh`、`export` の 3 行）を置く。
-   [x] 「製品の制約」の 8 項目を、設計書 3.1 の「写し方」の表のとおりに 0008 評価記録 §6.3 から写す。項目 5 は、設計書 3.1 の箇条書きの 4 点（受理と失敗の通知、通知が長さの失敗を区別しないことと `requestId`、送り直しの費用、通知の投稿の失敗）を主経路の結果として先に書く。
-   [x] 「品質の目標と結果」の節に、評価の条件（yt-dlp の字幕で生成したこと、0018 評価記録 §4.9.1 への参照、`max_tokens` 16,000 と本番の上限 40,000）を書く（設計書 1.2・3.1）。
-   [x] 写した数値（件数・頻度・金額・倍率）を、出典の 0008 評価記録 §6.2〜§6.4 と 1 つずつ照らす。「パイプライン」の表の部品の名前は、コード（`internal/transcript`・`internal/writer`・`internal/publisher`）と照らす。項目 5 のコードについての記述は、設計書 3.1 が引く箇所（`internal/server/job.go:138-155`・`internal/publisher/slack_failure.go:111-112`・`README.md:76-79`・`:941-942`・`:953-955`）と照らす。評価の条件は、出典（`docs/tasks/0041_claude_effort_max_tokens/01_requirements.md:19`、0018 評価記録 §4.9.1・§4.7.2）と照らす。秘密情報の変数は名前だけで書く（AC-15）。未解決の issue の節や一覧を置かない（AC-07）。

**完了の条件:** ファイルがあり、5 つの節の見出しが設計書 3.1 の文言と一致する。この時点ではテストはまだない（フェーズ 2 で加える）。コミットする。

### フェーズ 2: テストの補助とテスト (a)

#### ステップ 2-1: `internal/config/test_helpers.go` に 4 つの関数を加える

**変更するファイル:** `internal/config/test_helpers.go`

-   [x] 設計書 3.10 の `LLMProviderEnvForTest`・`ModelEnvForTest`・`ClaudeEffortEnvForTest`・`AnthropicAPIKeyEnvForTest` を、既存の `...ForTest` 関数と同じ形（それぞれ対応する定数を返す 1 行の関数と英語の doc コメント）で加える。

#### ステップ 2-2: フェンスを区別する走査の補助を加える

**変更するファイル:** `cmd/yt2column/docs_test.go`、`cmd/yt2column/output_layout_docs_test.go`

-   [x] 文書の行を、各行がコードブロックの中かどうかとともに走査する補助を `docs_test.go` に加え、フェンスの行・ブロックの中の `#` 行・閉じていないフェンスを確かめる単体テストを加える。
-   [x] `firstCodeBlockAfter` を、見出しの後の行だけをその補助に渡す形に書き換える（1.3）。`TestFirstCodeBlockAfter` と `TestOutputLayoutDocsContract` を変えずに通す。

#### ステップ 2-3: テスト (a) を共通の文書に当てる

**変更するファイル:** `cmd/yt2column/docs_test.go`

-   [x] 共通の文書のパスの定数を、既存のパスの定数（`docs_test.go:22-29`）に加える。
-   [x] `TestProductionProfileDocsContract` を加える。設計書 6.2 の (a) の契約のとおり、共通の文書の `## 本番の構成` の節について次を確かめる。この時点では README の節は対象にしない（ステップ 3-3 で加える）。
    -   節がある。節の中の `sh` のコードブロックがちょうど 1 つで、各行が設計書 3.2 の行の形に合い、同じ変数の行が 2 つない。
    -   ブロックの変数名の集合が、ステップ 2-1 の関数で得る 3 つの変数名（プロバイダ・モデル・effort）と等しい。
    -   閉じた map（ダミーの `ANTHROPIC_API_KEY`、`.invalid` のドメインの `https` のダミーの Webhook URL、短い固定の絶対パスの `HOME`。変数名は `AnthropicAPIKeyEnvForTest`・`SlackWebhookEnvForTest` から得る）を `LoadServer` に通した構成と、その map にブロックの変数を加えて通した構成を比べ、生成と検証・推敲のプロバイダ・モデル、effort が等しい。後者は拒否されない。
    -   「本番の構成」の表に、「生成」「字数の調整」「検証・推敲」の行がそれぞれちょうど 1 つある。各行のプロバイダ・モデル・effort のセルから最初のコードの書式の値を取り出し、1.3 の方法で `LoadServer` に通す。得られた構成が、ブロックを加えずに読み込んだ構成の同じ段階の値（「字数の調整」は生成の値）と等しい。コードの書式の値がないセルと、`LoadServer` が拒否した行は失敗にする。
-   [x] 失敗の文言には、文書・節・行番号・変数名を書き、ブロックの行の値を書かない（設計書 4 章）。
-   [x] 壊して失敗を確かめる: 設計書 7.1 の (a) の壊し方（変数名の書き誤り、モデル名の変更、3 つの変数の 1 つの削除、4 行目の追加、値の引用符、表の effort のセルの変更）に加え、同じ変数の行の重複、`sh` のブロックを 2 つにする・なくす、節の見出しの変更、表の段階の行の重複、表のセルを、受理されない値（例: `hgh`）に変える、を 1 つずつ当て、それぞれでテストが失敗することを確かめる。結果をコミットのメッセージに書く。

**完了の条件:** `make fmt`、`make test`、`make lint` が通る。コミットする。

### フェーズ 3: README

#### ステップ 3-1: 冒頭・Requirements・Usage を書き直す

**変更するファイル:** `README.md`

-   [x] 冒頭（`README.md:3-15`）を設計書 3.2 の 1 のとおりに書き直し、図を設計書 3.3 の形（英語のラベル）にする。
-   [x] Requirements（`README.md:19-27`）を、設計書 3.2 の 3 のとおり、見出しを加えずに太字の導入の行で 2 つのまとまりに分ける。
-   [x] Usage の見出し（`README.md:29`）の直後に、設計書 3.2 の 4 の段落を加える。

#### ステップ 3-2: Quick start と推奨構成の小節を加える

**変更するファイル:** `README.md`

-   [x] 状態の注記（`README.md:17`）と `## Requirements` の間に `## Quick start` を加え、設計書 3.2 の 2 の表の 6 つの手順を番号付きの箇条書きで書く。コードブロックと表は使わない。
-   [x] 「Configuration」の変数の表と、それに続く段落の後、`### Using DeepSeek`（`README.md:785`）の前に `### Recommended configuration` を加え、設計書 3.2 の 5 のとおり、構成のブロックとブロックの後の文を書く。
-   [x] 既定の構成の段落（`README.md:745-748`）を推奨構成の小節の中へ移し、「指定しなくても同じ構成になる」の文と 1 つにまとめる（設計書 3.2 の 5）。
-   [x] Quick start と Requirements に書いたコマンドとパスを、正本と照らす。`make` のターゲットは `Makefile`、バイナリの名前と `build/` は `make build` と `cmd/`、`extension/dist` は `make ext-build` の出力先、Node.js のバージョンのファイルは `extension/.node-version`、各手順の前提の句はリンク先の README の節（設計書 3.2 の 2 の表が引く行）と照らす。

#### ステップ 3-3: テスト (a) を README に当て、テスト (b) を加える

**変更するファイル:** `cmd/yt2column/docs_test.go`

-   [x] `TestProductionProfileDocsContract` の対象に、README の `### Recommended configuration` の節を加える。ブロックの形・変数名の集合・`LoadServer` の照合を、共通の文書と同じ手順で当てる（表の照合は共通の文書だけ）。あわせて、その見出しが、コードブロックの外の `## Configuration` と、その次の、`#` または `##` の見出しの間にあることを確かめる（AC-13 の構造の部分）。
-   [x] 見出しから GitHub のアンカーを作る補助と、Markdown のリンクの取り出しの補助を加える。アンカーの規則は設計書 6.2 に従い、本書には写さない。補助が作るアンカーが、GitHub がその見出しに対して実際に解決するアンカーと一致することを、README のコードブロックの外の見出し（インラインマークアップを含む見出しも含む）についてテストで確かめる。同じ見出しに付く接尾辞は扱わないので、コードブロックの外の見出しに重複がないことも確かめる。
-   [x] `TestQuickStartLinks` を加える。設計書 6.2 の (b) の契約のとおり、README の `## Quick start` の節（終わりは、コードブロックの外の次の、`#` または `##` の見出し）について、節があること、その見出しがコードブロックの外の `## Requirements` より前にあること（AC-09 の構造の部分）、番号付きの各項目が README の見出しへのリンクを 1 つ以上持つこと、各アンカーがコードブロックの外の見出しから作るアンカーのどれかと一致すること、節の中のほかのリンクが README の中のアンカーか存在する相対パスであることを確かめる。
-   [x] 節の切り出しとアンカーの補助に、単体テストを加える。節の切り出しは、コードブロックの中の `#` 行で節が終わらないことを確かめる。アンカーは、README の Quick start の外にある既存のページ内リンク（`](#...)`。GitHub で解決しているものとして扱う）がすべて、補助で作るアンカーのどれかと一致することを確かめる。
-   [x] 壊して失敗を確かめる: (a) は README のブロックについて設計書 7.1 の壊し方のうち表以外を当てる。(b) は設計書 7.1 の (b) の壊し方（存在しないアンカー、コードブロックの中の `#` 行のアンカー、1 つの手順のリンクを外す）に加え、存在しない相対パスへのリンク、`## Quick start` の見出しの変更と `## Requirements` の後ろへの移動、`### Recommended configuration` の `## Configuration` の外への移動を当てる。`# x` の行を持つコードブロックを Quick start の途中に入れたときは、節がそこで終わらず、テストは通る（切り出しが壊れたときは、補助の単体テストが失敗する）。結果をコミットのメッセージに書く。

**完了の条件:** `make fmt`、`make test`、`make ext-test`、`make lint` が通る（README を変えるので `make ext-test` も走らせる）。コミットする。

### フェーズ 4: ほかの文書の主経路と、共通の文書への参照

#### ステップ 4-1: `docs/dev/project_overview.md` を書き直す

**変更するファイル:** `docs/dev/project_overview.md`

-   [x] 設計書 3.3 の「書き直す箇所」の 1〜5（冒頭、「実行形式」、「字幕取得」、「LLM」、パイプラインの図と説明）を、そのとおりに書き直す。図は日本語のラベルで、設計書 3.3 の形にする。「LLM」の項には、#490・#171 の issue へのリンクと、共通の文書へのリンク（`project_overview.md` からの相対パス `production_profile.md`）を書き足す。
-   [x] 設計書 3.3 の「変えない箇所」（`:105-130`）と、既存のテストが読む行（`:36`・`:52`・`:60`・`:124`）を変えない。
-   [x] 書き直した「実行形式」「字幕取得」を、設計書 3.3 の決定の対照表の 12 行と 1 行ずつ照らし、すべての決定が残っていることを確かめる。

#### ステップ 4-2: `CLAUDE.md` を書き直す

**変更するファイル:** `CLAUDE.md`

-   [x] 設計書 3.4 の 1〜5 のとおりに書き直す。Quick Links の「Project」に共通の文書へのリンクと英語の説明を加える。Architecture Overview の冒頭の一文（`CLAUDE.md:135`）と図（`:137-139`）を改める。`TranscriptSource` の説明（`:141-144`）の順序を入れ替える。Development Notes の文書のテストの一覧（`CLAUDE.md:327-331`）に、新しいテストが読む対象を加える。
-   [x] "- `make ext-…`" の形の行と、PR のレビューの依頼の言及を加えない（設計書 3.6 の表）。

#### ステップ 4-3: `docs/dev/security.md` と `prompts/README.md` を改める

**変更するファイル:** `docs/dev/security.md`、`prompts/README.md`

-   [x] `docs/dev/security.md:5` を、設計書 3.5 の 3 つを述べる文に改める。§1 以降は変えない。
-   [x] `prompts/README.md:9` の評価と製品の制約の参照を、共通の文書へのリンクに改める。v5 へのリンクは残す（設計書 3.7）。

#### ステップ 4-4: テスト (c) を加える

**変更するファイル:** `cmd/yt2column/docs_test.go`

-   [x] `prompts/README.md` のパスの定数を加える。
-   [x] `TestProductionProfileLinks` を加える。設計書 6.2 の (c) の契約のとおり、README の `### Recommended configuration`、`project_overview.md` の `## 決定済みの方針`、`CLAUDE.md` の `## Quick Links` の各節と、`prompts/README.md` の全体に、相対パスを解決すると共通の文書のパスの定数と同じファイルになるリンクがあることを確かめる。リンクの取り出しは、ステップ 3-3 の補助を使う。期待するリンク先は、パスの定数から導き、文書から写したリテラルを置かない。
-   [x] 壊して失敗を確かめる: 4 つの文書のそれぞれで、リンクのパスを書き誤るとテストが失敗することを確かめる。結果をコミットのメッセージに書く。

**完了の条件:** `make fmt`、`make test`、`make lint` が通る。コミットする。

### フェーズ 5: 0008 評価記録への参照

#### ステップ 5-1: 0008 評価記録に参照を足す

**変更するファイル:** `docs/tasks/0008_column_prompt/02_evaluation.md`

-   [x] 設計書 3.8 の 1〜3 のとおりに書き足す。§6 の見出しの直後に引用のブロックを 1 つ加える。§6.5 の最後の項目の文の後に、リンクと日付の新しい文を加える。`Comments` に、参照を足した日付と、編集上の修正で決定を変えていないこと（`(editorial)`）を書く。
-   [x] §6 の既存の行と、Document Status の `Status`・`Review date`・`Reviewer` の行を変えない。

**完了の条件:** コミットする（確かめはフェーズ 7 の差分の確認と、PR のレビューで行う）。

### フェーズ 6: 用語集

#### ステップ 6-1: 用語集に用語を加える

**変更するファイル:** `docs/translation_glossary.md`

-   [x] 設計書 3.9 の 6 行を加え、「段階」の行（`docs/translation_glossary.md:39`）の備考に、`ArticleWriter` の中の生成・字数の調整・検証・推敲を指す用法を加える。
-   [x] README・`CLAUDE.md` に書いた英語の言い方（primary path・development path・production profile・recommended configuration）が、加えた行の英訳と一致することを読んで確かめる。

**完了の条件:** 用語集の変更だけを 1 つのコミットにする（`.claude/commands/mktrans.md` の手順。設計書 3.9）。

### フェーズ 7: 差分の確認と、最後の確かめ

#### ステップ 7-1: 差分の確認のスクリプトを加えて実行する

**変更するファイル:** `docs/tasks/0040_production_profile/_checks/main.go`（新規）、本書

-   [x] 設計書 7.2 の 3 つの判定（README の見出し、0008 評価記録の §6 と Document Status の 3 行、`security.md` の §1 以降）に、`project_overview.md` の `## 設定（環境変数）` の節（設計書 3.3 の「変えない箇所」）が変わらないことの判定を加え、4 つの判定を行うスクリプトを Go で書く（1.3）。
    -   この節は、見出しの行 `## 設定（環境変数）` から次の `## ` の行までとする。見出しの文字列は、1.2 の例外（日本語の文書の見出しと一致しなければならないリテラル）として書く。
    -   スクリプトは、merge-base（`git merge-base origin/main HEAD`）と `HEAD` の 2 つの版のファイルを `git show` で読み、判定ごとに合否を出す。合わなかった行は、行番号と内容も出す。コードブロックの判定はフェンスの行で切り替える。いずれかの判定が不合格なら 0 以外で終わる。
    -   判定の名前に AC の番号を使わない。ソースは、この見出しのリテラルのほかは ASCII だけで書く（1.2）。環境変数を読む関数と秘密情報の変数名を参照しない（1.3）。
-   [x] 壊して失敗を確かめる: スクリプトは head のコミットを読むので、push しない使い捨てのブランチに壊したコミットを作って実行し、確かめた後にブランチを消す。既存の見出しを 1 つ変える、§6 の既存の 1 行を変える、§1 の 1 行を変える、「設定（環境変数）」の節の 1 行を変える、の 4 つで、それぞれの判定が不合格になることを確かめる。結果をスクリプトのコミットのメッセージに書く。
-   [x] `go vet ./docs/tasks/0040_production_profile/_checks/` と `make fmt` を通し、`make test`（`TestEnvAccessConfined` がスクリプトを読む）を通す。
-   [ ] `git fetch origin main`（`_lib/task-pr-handoff.md` の Authorization が `/runplan` に認める）を実行する。すべての文書の変更をコミットしたら、PR の head でスクリプトを実行し、出力と、merge-base と head の 2 つのコミットの ID を、7 章の AC-12・AC-20・AC-22・AC-26 の行に記録する。記録を本書にコミットする（記録のコミットは文書の変更を含まないので、判定の結果は変わらない）。この後、判定する 4 つのファイルのどれかが変わったとき（`/fixpr` などの修正）や、`main` をブランチに取り込んだときは、実行し直して記録を置き換える。

#### ステップ 7-2: 最後の確かめ

-   [ ] `make test`、`make ext-test`、`make lint`、`make deadcode` を走らせ、すべて通ることを確かめる。
-   [ ] 新しいテストを含む `doc-guard` の CI ジョブの対象（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`、`.github/workflows/ci.yml:187`）が通ることを確かめる。

**完了の条件:** 上のコマンドがすべて通り、7 章の `diff` の行に記録がある。

### PR-1 作成ポイント: production profile document and primary-path rewrite

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 5-1 / 6-1 / 7-1 / 7-2

**推奨タイトル**: `docs(0040): 本番の構成と品質の共通の文書を加え、文書の主経路をブラウザ拡張に改める`

**レビュー観点**: 共通の文書の数値と 8 項目が 0008 評価記録 §6.2〜§6.4 と一致するか / `project_overview.md` の書き直しが設計書 3.3 の決定の対照表の決定をすべて保つか / テスト (a)〜(c) が設計書 6.2 の契約を満たし、壊した結果がコミットのメッセージにあるか / 差分の確認の出力と 2 つのコミットの ID が 7 章に記録されているか

**実装モデル要件**: standard

**判定理由**: 前例のない設計判断も、競合する実装案もない。条件付きの検査に当たるのはステップ 2-1（`test` タグだけでコンパイルするファイル）とステップ 2-3（同じ検証を通る `LoadServer` の繰り返しの呼び出し）で、それぞれ 1 つずつであり、2 つ以上に当たるステップはない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズの順序は設計書 8 章の順序に従う。

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1: 共通の文書 | 1 | `docs/dev/production_profile.md` |
| M2: 構成の照合 | 2 | 4 つの `...ForTest` 関数、走査の補助、テスト (a)（共通の文書） |
| M3: README | 3 | README の書き直し、テスト (a)（README）、テスト (b) |
| M4: ほかの文書 | 4 | `project_overview.md`・`CLAUDE.md`・`security.md`・`prompts/README.md`、テスト (c) |
| M5: 記録と用語 | 5・6 | 0008 評価記録の参照、用語集 |
| M6: 確かめ | 7 | 差分の確認のスクリプトと記録、すべてのチェックの通過 |

フェーズ 2 は、フェーズ 1 の共通の文書を前提にする。フェーズ 4 のテスト (c) は、フェーズ 3 のリンクの取り出しの補助を前提にする。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 5-1 / 6-1 / 7-1 / 7-2 | 共通の文書、主経路への書き直し、参照の張り替え、用語集、テスト (a)〜(c)、差分の確認 | standard |

1 つの PR にする理由は設計書 8 章のとおりである。文書だけを先に入れるとテストが確かめない期間ができ、テストだけを先に入れると共通の文書がなくて失敗する。

## 4. テスト戦略 (Test Strategy)

### 4.1. 加えるテスト

| テスト | 場所 | 確かめること |
|---|---|---|
| (a) `TestProductionProfileDocsContract` | `cmd/yt2column/docs_test.go` | 共通の文書と README の構成のブロック、共通の文書の「本番の構成」の表を、`LoadServer` の結果と照合する（設計書 6.2） |
| (b) `TestQuickStartLinks` | 同上 | README の Quick start の各手順のリンクが、README の見出しに解決する（設計書 6.2） |
| (c) `TestProductionProfileLinks` | 同上 | 4 つの文書が共通の文書へリンクする（設計書 6.2） |
| 補助の単体テスト | 同上 | フェンスを区別する節の切り出しと、見出しのアンカー（ステップ 3-3） |

-   **環境からの独立:** テスト (a) は `lookupFrom` の閉じた map だけを `LoadServer` に渡し、プロセスの環境変数を読まない。開発者の環境に残った `YT2COLUMN_*` の変数が結果を変えない。ネットワークに接続しない。
-   **2 回の `LoadServer` の条件:** ブロックや表の値を加えて読む呼び出しも、加えずに読む呼び出しも、同じダミーの API キー・Webhook URL・`HOME` を持つ map から作る。どの呼び出しも、照合する値に届く前に `SLACK_WEBHOOK_URL` や `HOME` の不足で拒否されないようにするためである。ブロックの変数は変数名の集合を確かめた後に加えるので、ダミーの値を上書きしない（設計書 6.2）。
-   **ビルドタグ:** 新しいテストは `docs_test.go`（`//go:build test`）に置き、`lookupFrom`（`test || integration`）と `internal/config` の `...ForTest`（`test`）を使う。`make test` と `doc-guard` の CI ジョブは `-tags test` で走るので、どちらもこれらをコンパイルする。
-   **壊して確かめる:** 各テストの壊し方は、ステップ 2-3・3-3・4-4 に挙げた。実装者が壊して失敗を確かめ、コミットのメッセージに書く。

### 4.2. 差分の確認

AC-12・AC-22・AC-26 は、`docs/tasks/0040_production_profile/_checks/main.go` で確かめる（ステップ 7-1、設計書 7.2）。AC-20 も、既存のテストに加えて、「設定（環境変数）」の節が変わらないことをこのスクリプトで確かめる。PR のブランチで 1 回実行し、結果を 7 章に記録する。`make test` と CI では実行せず、マージの後は保守しない（`requirements_process.md` §4）。

### 4.3. 既存のテスト

既存のテストの検査の内容は変えない。`make test`・`make ext-test`・`make lint` が通ることを、フェーズ 2〜4 とフェーズ 7 で確かめる（要件書 §4.3）。`firstCodeBlockAfter` の書き換えは、`TestFirstCodeBlockAfter` と `TestOutputLayoutDocsContract` が変えずに通ることで確かめる。

### 4.4. 加えないテスト

設計書 7.1 の「加えないテスト」のとおり、評価の数値（AC-04・AC-05）、単価の表のモデル名、README の既存の見出しの保存（`diff` で確かめる）、0008 評価記録に足す参照は、テストにしない。統合テストとセキュリティのテストは加えない（設計書 7.4）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| README の新しいコードブロックや表が、既存のテストの節や表の取り出しを変える | 既存のテストが、気づかれずに別の対象を確かめる | 1.2 の制約（新しい表を置かない、新しいコードブロックに `#` の行を書かない）を守る。ステップ 3-3 で `make test` と `make ext-test` を走らせる。差分の確認で既存の見出しが変わらないことを確かめる |
| 共通の文書に写した数値が、出典と食い違う | 共通の文書が、品質について誤ったことを伝える | ステップ 1-1 で出典と 1 つずつ照らす。PR のレビューの観点に入れる（PR-1 の `レビュー観点`） |
| `project_overview.md` の並べ替えで、方針の決定が落ちる | 決定済みの方針が気づかれずに変わる | ステップ 4-1 で設計書 3.3 の対照表と 1 行ずつ照らす。PR のレビューで差分と対照表を照らす |
| テスト (b) のアンカーの規則が GitHub の規則と違う | 正しいリンクが失敗する、または誤ったリンクが通る | ステップ 3-3 で、README の既存のアンカーへのリンクが補助のアンカーと一致することを確かめ、補助に単体テストを加える |
| コード上の既定が後のタスクで変わる | テスト (a) が失敗する | 設計どおりの振る舞いである（設計書 9 章）。構成を変えるタスクが、共通の文書と推奨構成の小節を更新する |

日程のリスク: 1 つの PR が大きくなるが、ほとんどが文書の変更で、テストの変更は `cmd/yt2column/docs_test.go` と `internal/config/test_helpers.go` に限られる。フェーズごとにコミットを分けるので、レビュアーはコミットの単位で読める。

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 5-1 / 6-1 / 7-1 / 7-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` はテスト、`diff` は差分の確認のスクリプト、`manual` は PR のレビューでの読み取りを指す。テストの場所は `cmd/yt2column/docs_test.go` である。

| AC | 確かめ方 | 実装 | 内容 |
|---|---|---|---|
| AC-01 | test: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract` ＋ manual | 1-1 | 3 つの段階のプロバイダ・モデル・effort はテスト。段階の一覧・役割・テンプレートはレビュー |
| AC-02 | test: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract` ＋ manual | 1-1 | 構成のブロックの変数名・値と既定との一致はテスト。検証・推敲の継承、effort の共通、既定であること、指定が要る変数の記述はレビュー |
| AC-03 | manual | 1-1 | 品質の目標・F-006 へのリンク・件数・§6.2 への参照 |
| AC-04 | manual | 1-1 | 8 項目と頻度・金額を 0008 評価記録 §6.3 と照らす。項目 5・6・8 の写し方は設計書 3.1 の表と照らす |
| AC-05 | manual | 1-1 | 比較の表と理由を 0008 評価記録 §6.4 と照らす |
| AC-06 | manual | 1-1・4-1 | 冒頭の更新の規則と出典。`project_overview.md`「LLM」の項の参照の張り替え |
| AC-07 | manual | 1-1 | 未解決の issue の節・一覧がないこと |
| AC-08 | manual | 3-1 | README の冒頭と図 |
| AC-09 | test: `cmd/yt2column/docs_test.go::TestQuickStartLinks` ＋ manual | 3-2 | 節が Requirements より前にあること、各手順のリンクと解決はテスト。手順の順序と内容はレビュー |
| AC-10 | manual | 3-1 | Requirements の書き分け |
| AC-11 | manual | 3-1 | Usage の段落 |
| AC-12 | diff: `docs/tasks/0040_production_profile/_checks/main.go`（README の見出しの判定） | 3-1・3-2 | 実行の記録: （ステップ 7-1 で記録する。merge-base と head のコミットの ID、出力） |
| AC-13 | test: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract` ＋ manual | 3-2 | 小節が「Configuration」の中にあること、3 つの変数と値はテスト。`ANTHROPIC_API_KEY` と検証・推敲の記述はレビュー |
| AC-14 | test: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`・`cmd/yt2column/docs_test.go::TestProductionProfileLinks` ＋ manual | 3-2 | 既定との一致は (a)、共通の文書へのリンクは (c)。文はレビュー |
| AC-15 | test: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract` ＋ manual | 1-1・3-2 | ブロックの変数名の集合が 3 つの変数と等しいこと（秘密情報の変数がないこと）はテスト。文章の中の値の例がないことはレビュー |
| AC-16 | manual | 4-1 | `project_overview.md` の冒頭 |
| AC-17 | manual | 4-1 | 設計書 3.3 の決定の対照表と差分を照らす |
| AC-18 | manual | 4-1 | `project_overview.md` の図と説明 |
| AC-19 | test: `cmd/yt2column/docs_test.go::TestProductionProfileLinks` ＋ manual | 4-1 | 共通の文書へのリンクはテスト。記述と #490・#171 のリンクはレビュー |
| AC-20 | test: `cmd/yt2column-server/docs_test.go::TestServerDocsContract`・`cmd/yt2column/docs_test.go::TestCostDocsContract`・`cmd/yt2column/docs_test.go::TestSlackDocsContract`・`cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`（既存） ＋ diff: `docs/tasks/0040_production_profile/_checks/main.go`（「設定（環境変数）」の節の判定） | 4-1 | 既存のテストが変えずに通ること（1.4）。既存のテストが読まない表の変数名（`DEEPSEEK_API_KEY` など）も残ることは、節が変わらないことの差分の確認。実行の記録: （ステップ 7-1 で記録する） |
| AC-21 | test: `cmd/yt2column/docs_test.go::TestProductionProfileLinks` ＋ manual | 4-3 | 共通の文書へのリンクはテスト。v5 へのリンクが残ることはレビュー |
| AC-22 | diff: `docs/tasks/0040_production_profile/_checks/main.go`（0008 評価記録の判定） ＋ manual | 5-1 | §6 の差分が追加だけで、Document Status の 3 つの行が変わらないことは差分の確認。追記した参照と `Comments` はレビュー。実行の記録: （ステップ 7-1 で記録する） |
| AC-23 | test: `cmd/yt2column/docs_test.go::TestProductionProfileLinks` ＋ manual | 4-2 | リンクはテスト。説明の文はレビュー |
| AC-24 | manual | 4-2 | `CLAUDE.md` の冒頭の一文と図 |
| AC-25 | manual | 1-1 | 共通の文書の評価の条件の記述 |
| AC-26 | diff: `docs/tasks/0040_production_profile/_checks/main.go`（`security.md` の判定） ＋ manual | 4-3 | §1 以降が変わらないことは差分の確認。`:5` の新しい文はレビュー。実行の記録: （ステップ 7-1 で記録する） |

`manual` だけの AC（AC-03〜AC-08・AC-10・AC-11・AC-16〜AC-18・AC-24・AC-25）は、文書の文面だけを定める AC である（要件書 §5、`requirements_process.md` §4）。値や構造の部分を持つ AC には、テストか差分の確認を当てた。

**申し送りの対応:** `implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。

## 8. 成功基準 (Success Criteria)

-   **機能の完全性:** 7 章のすべての AC の確かめ方が実行され、`test` と `diff` が合格し、`manual` の行が PR のレビューで確かめられている。
-   **品質:** `make test`・`make ext-test`・`make lint`・`make deadcode` が通る。新しいテストは、ステップ 2-3・3-3・4-4 の壊し方で失敗することが確かめられ、コミットのメッセージに書かれている。
-   **セキュリティ:** 文書とテストに秘密情報の値がない。テスト (a) の失敗の文言がブロックの行の値を出さない。テストはネットワークに接続しない（設計書 5 章）。
-   **文書:** 共通の文書・README・`project_overview.md`・`CLAUDE.md`・`security.md`・`prompts/README.md`・0008 評価記録・用語集の変更が、設計書 3.1〜3.9 と一致する。

## 9. 次のステップ

-   本書の承認の後、`/runplan 0040` でフェーズ 1〜7 を実装し、PR-1 を開く。
