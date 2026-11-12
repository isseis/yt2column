# 実装計画書：Go の WASM 版のパイプラインを拡張のページで動かす PoC と、構成の判断に要る値の測定

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md) の F-001〜F-005 を、[02_architecture.md](02_architecture.md) の設計に従って実装し、測定する手順を定める。作業は、設計書 §8 の 4 つの段階に対応して、4 つのフェーズに分ける。

| フェーズ | 内容 | 置き場所 | 進め方 |
|---|---|---|---|
| 1 | F-001：コアの分割 | `main`（PR-1） | `/runplan`・`/runpr` |
| 2 | F-002・F-003：PoC のコード | PoC のブランチ（マージしない） | 対話のセッション（§1.3） |
| 3 | F-004：測定 | 人のブラウザと PoC のブランチ | 対話のセッション（§1.3） |
| 4 | F-005：調査記録 | `main`（PR-2） | `/runplan`・`/runpr` |

本書の行の番号は、断りがなければコミット 4135c266 のものである。

### 1.2. 実装の方針

-   **フェーズ 1 の差分は、要件書 AC-04・AC-05 が許す書き換えに限る。** 設計書 §1.1 の「`main` の変更は移動に限る」に従う。許す書き換えの一覧は、設計書 §3.1.3 の表と §3.1.4 の表である。
-   **フェーズ 2 の PoC のコードは、既存のコードを変えずに使う**（設計書 §1.1）。PoC のブランチで既存のパッケージを書き換えたくなったら、書き換えずに観測として記録する。
-   **実 API の呼び出しは、1 回ずつ人の承認を得てから行う**（要件書 §5、設計書 §6.3）。費用の目安（約 4 USD）を超えそうなら、超える前に止めて人に尋ねる。
-   **PoC のコードにはテストを書かない**（要件書 §5「受け入れの確かめ方の例外」、設計書 §7.2）。
-   **PoC の Go と TypeScript のコメント・識別子・文字列リテラルは英語で書く。** 画面の表示文も英語とし、メッセージのカタログは使わない（`CLAUDE.md`「Go Idioms」。PoC は多言語に対応しない）。

### 1.3. 計画の際に決めたこと（2026-11-11）

-   **`YtDlpSource.Fetch` のローカル変数 `transcript` を `cached` に改名し、要件書 AC-05 と設計書 §3.1.3 にこの書き換えを足した。** `internal/transcript/ytdlp.go:69`〜`74` では、ローカル変数 `transcript` のスコープの中で `Transcript{}` を返している。移した後にこれを `transcript.Transcript{}` と修飾すると、ローカル変数がパッケージ名を隠すため、コンパイルできない。これは、作業用の写しでファイルを実際に移して確かめた（§1.4）。import に別名を付ける案は、そのファイルのすべての修飾の綴りが変わり、AC-04・AC-05 の `diff` の検証も複雑になるので採らなかった。人の答えは「変数を改名」である。決定の変更なので、要件書と設計書は `draft` に戻し、本書の PR で承認し直す。ADR: なし（このタスクの中で閉じる）。
-   **フェーズ 2・3 は `/runplan` の外の対話のセッションで進め、`/runplan` はフェーズ 4 から進める。** `/runplan` は、次の PR の作成ポイントまでを 1 つのブランチで実装し、グリーンゲートとレビューを経て PR を開く作りである。PoC のコードは別のブランチ（マージしない）に置き、測定は人がブラウザを操作して実 API の呼び出しを 1 回ずつ承認するので、この流れに収まらない（設計書 §8）。手順は次のとおりとし、人の答えは「対話セッションで」である。
    1.  PR-1 のマージの後、`/runpr` が次のブランチ（PR-2 のブランチ）を作る。`/runpr` のマージの問いでは、次のコマンドを実行しない答えを選ぶ（`/runplan` はフェーズ 2 の未完了のチェックで止まるため）。
    2.  人が対話のセッションで指示し、PoC のブランチを `origin/main` から作って、フェーズ 2 を実装する。続けてフェーズ 3 を測る。
    3.  同じセッションで、PR-2 のブランチにフェーズ 2・3 のチェックを付けたコミットを積む。測った値は、リポジトリの外の測定の記録（`$HOME/.local/share/yt2column/0084-measurements/`）に置く。セッションごとのスクラッチの領域は、フェーズ 4 の別のセッションから読めないので使わない。
    4.  `/runplan` がフェーズ 4 を進め、調査記録を書いて PR-2 を開く。

    ADR: なし（このタスクの進め方の決定で、タスクの外に及ばない）。
-   **PoC のブランチの名前は `poc/0084-wasm-pipeline` とする。** 作業ブランチの命名の規約（`<GitHub のユーザ名>/<slug>-<NN>`）に合わない名前にすることで、`/runplan`・`/runpr` が自分の作業ブランチと取り違えないようにする。ADR: なし。

**実装の際に決めたこと（2026-11-11）:**

-   **番兵の一覧を固定するテストを、`internal/transcript` のテストに残す（ステップ 1-1）。** 番兵の一覧を固定するテスト `TestExportedSentinelErrorsUnchanged`（`internal/transcript/json3_test.go`）は、テストを動かすパッケージのディレクトリの、`Err` で始まる公開の名前を走査して、6 つの番兵と照らす。このテストを `internal/ytdlpsource` へ移すと、走査する先の番兵のない側のディレクトリになり、`make test` が失敗する（実際に移して確かめた）。番兵は移した後も `internal/transcript`（`errors.go`）にあるので、このテストは `internal/transcript` のテストに残す。人の答えは「`internal/transcript` へ戻す」である。要件書 AC-04 と設計書 §3.1.3 に、このテストの移動と補助の複製を足した。ADR: なし（このタスクの中で閉じる）。

**実装の際に決めたこと（2026-11-11）:**

-   **入口が送るモデルを固定の 2 つに限り、止まった呼び出しとみなすまでの時間を呼び出しごとに決める（ステップ 2-2・2-4）。** 測定の前の Opus のレビューで見つかった 2 つの問題への対処である。1 つは、モデルの欄に貼り付けた API キーが、失敗の文を通して画面に出ることである。もう 1 つは、20 分の打ち切りが正常な `generate` より短いことである。人の答えは、「モデルと effort を固定の選択肢に」と「時間を呼び出しごとにし、結果も残す」である。内容と理由は、設計書 §1.2.2 に書いた。ADR: なし（PoC の中に閉じる）。

### 1.4. 既存コード調査結果

調べたのはコミット 4135c266 の時点のコードで、Go は 1.27.1 である。

**分割の試行。** 作業用の写し（`git archive` で展開したもの）で、設計書 §3.1.4 のファイルを `internal/ytdlpsource` に移し、識別子の修飾を書き換えて、ビルドの結果を見た。

-   移すファイルが、`internal/transcript` に残る側から使う識別子は、公開の型・番兵・関数（`Segment`・`Transcript`・`TranscriptSource`・`ValidateVideoURL`・`ErrNoSubtitles`・`ErrParseInfo`・`ErrParseSubtitles`・`ErrYtDlpExec`・`ErrInvalidVideoURL`・`ParseError`）と、非公開の `videoIDPattern` だけだった。`videoIDPattern` を使うのは、設計書 §3.1.3 の 2 か所である。
-   修飾の書き換えでコンパイルできないのは、`internal/transcript/ytdlp.go:69`〜`74` の 1 か所だけだった（§1.3）。テストのコード（`-tags test,integration`）には、同じ衝突はなかった。
-   移した後の 4 つのパッケージ（`internal/transcript`・`internal/writer`・`internal/llm/claude`・`internal/pricing`）は、次の 2 つの検査を通った。
    -   `GOOS=js GOARCH=wasm go build`。
    -   `internal/writer` と `internal/transcript` への `GOOS=js GOARCH=wasm go vet`。

    なお、この試行では `internal/writer` の開くフラグを `os.O_RDONLY` に置き換えて確かめた。
-   上の 4 つのパッケージの `GOOS=js GOARCH=wasm go list -deps` には、`internal/ytdlpsource` も `os/exec` も含まれなかった。

**テストの補助の分け方（`internal/transcript/test_helpers.go`）。**

-   残る側のテスト（`copied_test.go`・`copied_format_test.go`・`video_id_test.go`・`makefile_test.go`）が使う補助は、`readTestdataFile`（`:232`）と定数 `testdataCopiedTranscripts` だけである。
-   `readTestdataFile` は、移す側のテスト（`info_test.go`・`json3_test.go`・`ytdlp_test.go`）も使う。これは設計書 §3.1.3 のとおり両方に置く。
-   ほかの補助（`newTestSource`・`placeSlot`・`testVideoID`・`sampleAllowlistEnv` など）と testdata のパスの定数は、移す側のテストだけが使う。

**移す宣言を使う、パッケージの外の箇所。** どれも、§2 のステップ 1-1 で import と修飾を書き換える。

| 箇所 | 使うもの |
|---|---|
| `internal/job/job.go:111` | `NewYtDlpSource`・`Options` |
| `internal/job/job_test.go:67` | `SeedCacheForTest` |
| `cmd/yt2column/run_test.go:74` | 同上 |
| `cmd/yt2column/test_helpers_integration.go:111` | 同上 |
| `internal/cachelock/cachelock_test.go:332` | `NewYtDlpSource`・`Options` |
| `docs/tasks/0018_article_quality/_material_check/main.go:158` | `NewYtDlpSource`・`Options` |

**移すファイルのパスを書いている箇所。**

| 箇所 | 扱い |
|---|---|
| `internal/config/envaccess_test.go:69`・`:392`〜`393`・`:412`・`:599`〜`600`・`:611` | パスの文字列を書き換える（設計書 §3.1.3） |
| `Makefile:216`（`test-integration`） | `./internal/ytdlpsource` に書き換える。`Makefile:114`（`test-parse-norace`）は残す |
| `docs/dev/cache_consistency.md:3`・`:158`・`:170` | パッケージとファイルの場所を書き換える |
| `docs/dev/project_overview.md:112` | ディレクトリの一覧を書き換え、`internal/ytdlpsource/` の行を足す |
| `docs/dev/adr/0033-transcript-cache-slots.md:24` | 決定の中のパッケージの名前を直す（編集上の訂正） |
| `CLAUDE.md:293`〜`296` | `YtDlpSource` の置き場所を書き換える |
| `.claude/commands/runplan.md:71`・`.claude/commands/mkplan.md:64` | 例に挙げた json3 のパーサと `TestParseJSON3` の場所を書き換える |
| `extension/test/ciChanges.test.ts:43`・`:63` | 変えない。変更の分類の入力に使う例のパスで、実在のファイルを指さない |
| `docs/tasks/0064_stderr_drain_test_deadline/_checks/drain_test_unchanged.go` | 変えない。過去のタスクの一度だけの検査（設計書 §3.1.4） |
| `docs/dev/production_profile.md:19`・`docs/dev/adr/0007-manual-transcript-copy.md:21` | 変えない。型の名前だけを書き、パッケージを書いていない |
| `docs/dev/adr/0033-transcript-cache-slots.md:34`・`:36`、`docs/dev/adr/0020-strict-json.md:17`・`:29`、`docs/dev/adr/0038-deadcode-and-race-free-timing.md:44` | 変えない。採らなかった案と、決めた当時の理由を、当時の形で書いている |
| `cmd/yt2column/makefile_test.go:44` のコメント（「transcript integration test」） | 変えない。字幕の統合テストを指す言葉で、パッケージの名前ではない。変えると AC-04 の範囲を超える |
| `CLAUDE.md:193`（`make lint` の説明）・`docs/dev/guides/development.md:7`〜`17`（`make` のターゲットの一覧） | `make wasm-check` を足す（ステップ 1-4） |

**文書を確かめる既存のテスト。**

-   `internal/pipeline/pipeline_test.go:629` の `TestPackageReferenceListsPackages` は、`package_reference.md` に `internal/ytdlpsource` の行がないと失敗する。
-   `internal/pipeline/strictjson_users_test.go:609` の `TestPackageReferenceStrictjsonUsers` は、`internal/strictjson` の行の「Shared by」が import の実際と合わないと失敗する。
-   どちらも、ステップ 1-1 の `package_reference.md` の書き換えを `make test` で確かめる。新しい文書のテストは足さない。
-   `Makefile` と `ci.yml` を読む既存のテスト（`TestLintTagsIncludeIntegration`（`ytdlp_test.go` とともに移る）、`TestCIRunsParseNorace`（`internal/transcript/makefile_test.go`）、拡張の `extension/test/repository.test.ts`）は、変えずに通す。

**ADR-0019 の食い違いの記述。** `docs/dev/adr/0019-pipeline-component-interfaces.md:22`・`:23` の 2 つの決定は、どちらも「この決定はまだコードに反映していない。この差分は #771 の実装で解消する。」という食い違いの記述を持つ。PR-1 がこの食い違いを解消するので、同じ PR で 2 つとも消す（[adr.md](../../dev/conventions/adr.md)「Discrepancy statements」）。

**`make lint` と CI。**

-   `Makefile:324`〜`326` の `lint` は、golangci-lint と `go vet -tags integration ./...` を走らせる。
-   `.github/workflows/ci.yml:79`〜`116` の `lint` のジョブは、golangci-lint の action を使い、`make lint` を呼ばない。このジョブには `actions/setup-go` の手順（`:88`〜`92`）があるので、`make wasm-check` の手順を足すだけで Go のツールチェーンを使える。
-   `lint` のジョブは、`check-changes` が Go のコードの変更を見つけたときだけ走る（`:83`）。

**型の構築の経路。** 本タスクは、具体的な型で振る舞いを分ける設計を持たない。この項目は当てはまらない。

## 2. 実装の手順 (Implementation Steps)

### フェーズ 1: F-001 のコアの分割（`main`）

設計書 §3.1。1 つの PR（PR-1）にまとめる。

#### ステップ 1-1: yt-dlp の経路を `internal/ytdlpsource` へ移す

**変えるファイル:** 設計書 §3.1.4 の表の Go のファイル、`Makefile` の `test-integration` の目標、`docs/dev/conventions/package_reference.md`。

-   [x] `git mv` で、`exec.go`・`ytdlp.go`・`cache.go`・`info.go`・`json3.go` と、そのテスト（`cache_test.go`・`exec_test.go`・`info_test.go`・`json3_test.go`・`ytdlp_test.go`・`integration_test.go`・`test_helpers_cache_seed.go`）を `internal/ytdlpsource/` へ移す。
-   [x] 移したファイルのパッケージ名の行・import の行・修飾を書き換える。
-   [x] `YtDlpSource.Fetch` のローカル変数 `transcript` を `cached` に改名する（設計書 §3.1.3）。
-   [x] `pruneCandidateID` を `transcript.NormalizedVideoURL` の 2 つ目の戻り値で判定する形に書き換える（設計書 §3.1.3）。書き換えた条件の否定を外すと `TestPruneCache`（移す前は `internal/transcript/cache_test.go:56`）が失敗することを確かめ、コミットメッセージに書く。`testVideoID` の失敗する分岐はテストで確かめないが、これはテストの補助なので受け入れる。
-   [x] `internal/transcript/test_helpers.go` を分ける。§1.4 の分け方のとおり、移す側だけが使う補助を `internal/ytdlpsource/test_helpers.go` へ移し、`readTestdataFile` は両方に置き、`testVideoID` は設計書 §3.1.3 の形にする。ビルドの制約の行（`//go:build test`）は両方に置く。
-   [x] `json3_test.go` の `TestExportedSentinelErrorsUnchanged` を、`internal/transcript/sentinel_test.go`（新規）へ、本体を変えずに移す。補助の `exportedErrNames` と `parsePackageFile` もそこに複製する。`parsePackageFile` は `internal/ytdlpsource/json3_test.go` の `TestTimestampReadingIsUnified` も使うので残し、`exportedErrNames` はそのテストだけが使うので移す側から外す（設計書 §3.1.3）。
-   [x] `internal/ytdlpsource/doc.go` に、パッケージの doc コメントを英語で書く。
-   [x] §1.4 の「移す宣言を使う、パッケージの外の箇所」の 6 か所の import と修飾を書き換える。
-   [x] `internal/config/envaccess_test.go` のパスの文字列 7 つ（`:69`・`:392`・`:393`・`:412` のキー・`:599`・`:600`・`:611`）だけを `internal/ytdlpsource/ytdlp.go` に書き換える。`:412` のキーに続く偽のソースの本文（`package transcript` で始まる）は変えない。走査は import と呼び出しだけを見るので、この文字列の `package` の行に依らない（`envaccess_test.go:99`〜`135`）。許可する参照の種類と数は変えない。
-   [x] `docs/dev/conventions/package_reference.md` を移動に合わせる（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers` がこのステップの `make test` で確かめるため）。`internal/transcript` の行から yt-dlp の経路を外し、`internal/ytdlpsource` の行を足す。書き換える行と足す行は、[ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md) に従い、責務の要約・主な公開の名前・仕様へのリンク（キャッシュは `cache_consistency.md`）だけにする。`internal/strictjson` の行の「Shared by」で、`internal/transcript` を `internal/ytdlpsource` に替える。
-   [x] `Makefile:216` の `test-integration` のパスを `./internal/ytdlpsource` に書き換える。
-   [x] `go vet ./docs/tasks/0018_article_quality/_material_check` が通ることを確かめる（`./...` に含まれないため。`go build` は作業ツリーに実行ファイルを残すので使わない）。
-   [x] `go test -tags integration -list . ./internal/ytdlpsource` で、統合テストが新しい場所で見つかることを確かめる（ネットワークは使わない）。

**完了の条件:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。`go vet -tags test,integration ./internal/ytdlpsource` が通る。

#### ステップ 1-2: `internal/writer` の上書き用のテンプレートを開く処理を分ける

**変えるファイル:** `internal/writer/template.go`、`internal/writer/template_open.go`（新規）、`internal/writer/template_open_js.go`（新規）。

-   [x] `overrideOpenFlags` と `readOverrideFile` を `template.go` から `template_open.go`（`//go:build !js`）へ、中身を変えずに移す（設計書 §3.1.2）。
-   [x] `template_open_js.go`（`//go:build js`）に、ファイルシステムに触れずに失敗する `readOverrideFile` を書く。エラーは `ErrInvalidTemplate` と `errors.ErrUnsupported` の両方を元のエラーとして含めて返し、文には `templateSource.String` を含める（設計書 §4.1）。
-   [x] `js` のビルドのこの振る舞いは、ネイティブの `make test` ではコンパイルされないので、Go のテストを足さない。ステップ 1-3 の `make wasm-check` の `go vet` で検査する（設計書 §7.1）。

**完了の条件:** `make test`・`make lint`・`GOOS=js GOARCH=wasm go vet ./internal/writer` が通る。`internal/writer` の既存のテンプレートのテスト（`template_test.go`）は変えずに通る。

#### ステップ 1-3: `make wasm-check` と CI の手順を足す

**変えるファイル:** `Makefile`、`.github/workflows/ci.yml`。

-   [x] `Makefile` に `wasm-check` の目標を足し、`.PHONY` に加える。目標は、AC-01 の 4 つのパッケージを `GOOS=js GOARCH=wasm go build` し、`internal/writer` と `internal/transcript` に `GOOS=js GOARCH=wasm go vet` を掛ける（設計書 §1.2）。
-   [x] `lint` の目標から `wasm-check` を呼ぶ。
-   [x] `ci.yml` の `lint` のジョブに、`make wasm-check` を走らせる手順を 1 つ足す。
-   [x] 目標の中で `$(shell ...)` などを使って、`go` を Makefile の読み込み時に評価しない。`extension/test/repository.test.ts:229`〜`257` が、`go` のない PATH で `make -n build test lint deadcode` を走らせ、標準エラーが空であることを確かめるからである。
-   [x] 検査が失敗しうることを確かめる。作業用の変更で次のことを 1 つずつ確かめ、変更を捨てる。確かめたことをコミットメッセージに書く（設計書 §7.1）。
    -   `internal/writer/template_open_js.go` を消すと、`make wasm-check` が失敗する。
    -   AC-01 の 4 つのパッケージのそれぞれに、型の誤りを持つ `//go:build js` のファイルを 1 つ足すと、`make wasm-check` が失敗する。

**完了の条件:** `make wasm-check`・`make lint`・`make ext-test` が通り、`make lint` の出力に `wasm-check` の実行が現れる。

#### ステップ 1-4: 文書を移動に合わせる

**変えるファイル:** 設計書 §3.1.4 の表の文書と、`docs/dev/adr/0019-pipeline-component-interfaces.md`。

-   [x] `CLAUDE.md`：「Architecture Overview」の `TranscriptSource` の段落の `YtDlpSource` の置き場所を書き換える。「Code Quality」の一覧に `make wasm-check` を足し、`make lint` の説明に `wasm-check` を呼ぶことを足す。
-   [x] `docs/dev/guides/development.md` の `make` のターゲットの一覧（`:7`〜`17`）に `make wasm-check` を足す。この文書は拡張のテストが読むので、`make ext-test` を走らせる。
-   [x] `.claude/commands/runplan.md:71`・`.claude/commands/mkplan.md:64` の例の場所を `internal/ytdlpsource` に書き換える。
-   [x] `docs/dev/project_overview.md:112` のディレクトリの一覧を書き換える。
-   [x] `docs/dev/cache_consistency.md` のパッケージとファイルの場所（`:3`・`:158`・`:170`）を書き換える。
-   [x] `docs/dev/adr/0033-transcript-cache-slots.md:24` のパッケージの名前を直す。編集上の訂正として、コミットメッセージに訂正の行を書く。
-   [x] `docs/dev/adr/0019-pipeline-component-interfaces.md` の 2 つの食い違いの記述（§1.4）を消す。2 つ目の決定の「検査の目標は 0084 設計書が定める」を、`make wasm-check`（`CLAUDE.md`「Code Quality」）を指す形に直す。どちらも編集上の訂正として、コミットメッセージに訂正の行を書く。
-   [x] 書き換えた文書のパスが HEAD に実在することを、書いたときに `git ls-files` で確かめる（`TestInstructionDocPaths` は `docs/dev/` の外のパスを確かめないため）。

**完了の条件:** `make test`（`TestInstructionDocPaths` を含む）が通る。

#### ステップ 1-5: 一度だけの検査のスクリプトを書き、結果を記録する

**変えるファイル:** `docs/tasks/0084_wasm_pipeline_poc/_checks/` の下のスクリプト、本書の §7。

-   [x] AC-01・AC-02 を確かめるシェルのスクリプトを書く（設計書 §7.1）。確かめる性質は次の 3 つである。
    -   AC-01 の 4 つのパッケージが `GOOS=js GOARCH=wasm go build` を通る。`internal/writer` と `internal/transcript` が `GOOS=js GOARCH=wasm go vet` を通る。
    -   4 つのパッケージの `GOOS=js GOARCH=wasm go list -deps` に、`internal/ytdlpsource` が含まれない。
    -   依存のうち、このリポジトリのパッケージが、どれも `os/exec` を import しない。
-   [x] AC-04・AC-05・AC-06 を確かめるシェルのスクリプトを書く。merge-base と PR のコミットを比べ、次の性質を確かめる。
    -   Go のファイル（製品のコードとテストのコード）について、パッケージ名の行・import の行・ビルドの制約の行・移動に伴う修飾を正規化すると、移した宣言と書き換えた呼び出し側の差分がなくなる。正規化の後に残る差分は、設計書 §3.1.3 の表の箇所（ローカル変数の改名と、番兵のテストの移動・補助の複製を含む）と `doc.go` だけである。
    -   2 つに分けた `test_helpers.go` は、正規化した後の行の多重集合（`sort` した行）で比べる。分ける前の行と、分けた後の 2 つのファイルを合わせた行の違いが、`readTestdataFile` の複製と `testVideoID` の書き換えの行だけである。
    -   `internal/writer` の分割は移動として確かめる。`template.go` から消えた行が、`template_open.go` のビルドの制約・パッケージ名・import の行を除いた本文と同じである。新しい内容は `template_open_js.go` だけである。
    -   PR 全体で変わったファイルの一覧（`--name-only --no-renames`）が、設計書 §3.1.4 の表のファイル、`docs/dev/guides/development.md`、ADR-0019・ADR-0033、`docs/tasks/0084_wasm_pipeline_poc/` の下に限られる。
    -   `Makefile` の差分が、`test-integration` のパスの書き換えと、`wasm-check` の目標・`.PHONY` への追加・`lint` からの呼び出しに限られる。`.github/workflows/ci.yml` の差分が、`lint` のジョブに足す 1 つの手順に限られ、その手順が `make wasm-check` を実行する。
    -   `.golangci.yml`・`go.mod`・`go.sum` に差分がない（AC-06）。
-   [x] どちらのスクリプトも、各検査が少なくとも 1 つの壊し方で失敗することを確かめる。
-   [x] PR-1 のブランチで 2 つのスクリプトを走らせ、出力と両方のコミットの ID を §7 の「スクリプトの結果」に書く。
-   [x] AC-05 のうち `docs/dev/conventions/package_reference.md` の差分は、差分の記録を読む形で確かめる（文書の小さな差分のため）。§7 の差分の記録の欄に、記録を書く。

**完了の条件:** 2 つのスクリプトがどちらも成功を報告し、§7 の AC-01・AC-02・AC-04・AC-05・AC-06 の行に結果が記録されている。

### PR-1 作成ポイント: core split for the WASM build

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0084): yt-dlp の経路を internal/ytdlpsource に分け、コアを WASM でビルドできるようにする`

**レビュー観点**: 移動の差分が AC-04・AC-05 の許す書き換えに限られ（`_checks/` の出力）、`pruneCandidateID` の判定が `videoIDPattern` と等価なこと / `js` のビルドの `readOverrideFile` がファイルシステムに触れず、2 つの番兵を元のエラーとして含めて返すこと / `make wasm-check` が AC-01 の 4 つのパッケージをすべて検査し、`make lint` と CI の `lint` のジョブから走ること / 移したファイルのパスを書いた文書と ADR の書き換えに漏れがないこと

**実装モデル要件**: frontier-recommended

**判定理由**: 条件付きの検査（Conditional checks）の 2 つに当たる。ビルドのタグの下だけでコンパイルされる新しいソース（`template_open_js.go`、`internal/ytdlpsource/test_helpers.go`）と、多くのテスト・文書が指すファイルの移動である。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/843
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: PoC のコード（PoC のブランチ）

設計書 §3.2〜§3.5。§1.3 のとおり、`/runplan` の外の対話のセッションで進める。`/runplan` はこのフェーズを実装しない。このフェーズのチェックが付いていなければ、`/runplan` は止まって報告する。

PoC のブランチでは、`make ext-build` が通ることだけを求める。`make ext-check`・`make test` は求めない（設計書 §5.3）。

#### ステップ 2-1: PoC のブランチを作る

-   [x] PR-1 のマージの後の `origin/main` から、`poc/0084-wasm-pipeline` を作る（§1.3）。

#### ステップ 2-2: WASM の入口（`cmd/yt2column-wasm`）

設計書 §3.2・§4.2・§6.1。

-   [x] `//go:build js && wasm` の `main` パッケージとして作り、`globalThis.yt2column` に `buildInfo`・`generate`・`longOutput`・`probe` を登録する。
-   [x] `generate` の入力の検査を、`internal/server/request.go:63`〜`122` の `validateBody` と同じ段・同じ順で書く。PoC で加えたメンバの検査も書く（設計書 §3.2「受け付ける形」）。
-   [x] `writer.NewProgressWriter` を、CLI と同じ条件（埋め込みのテンプレート・字数の調整と検証・推敲を有効・期限 15 分）で作る。15 分の値の出典として、`provider.LLMTimeout` をコメントに書く。
-   [x] 失敗の種類を、設計書 §4.2 の表のとおり番兵で選ぶ。`invalid_config` の文は固定にする。
-   [x] 工程ごとの値を、`ProgressObserver` の時刻と `RunUsage`・`pricing.Estimate` から組み立てる（設計書 §2.4・§3.2）。
-   [x] 4 つの関数で、panic を `recover` で捕まえる。
-   [x] `browserAccessHeader` のときだけヘッダを足す `http.DefaultTransport` の包みを入れる（設計書 §1.2）。
-   [x] `longOutput` と `probe` を設計書 §3.2 のとおりに作る。`probe` は送り先を検査してから送る。

#### ステップ 2-3: プローブ用サーバ（`cmd/yt2column-wasmprobe`）

-   [x] 設計書 §3.3 の表の経路と記録を持つ、ネイティブのサーバを作る。`localhost` のポートは `127.0.0.1` と `::1` の両方で待ち受ける。
-   [x] ログの書き先をリポジトリの外のパスにすることを、パッケージの doc コメントに書く（AC-28）。

#### ステップ 2-4: PoC のページとビルド

設計書 §3.4。

-   [x] `extension/static/poc.html` と `extension/src/poc.ts` を作る。ボタンの規則・ランタイムの終わり・呼び出しごとの打ち切り・モデルと effort の選択肢・ビルドの同一性・API キーの扱い・表示を、設計書 §3.4 のとおりにする。
-   [x] 記事を `parseArticleMarkdown` と `buildArticleBody` で描く。失敗の文と工程ごとの値は `textContent` でだけ表示する。
-   [x] `extension/static/poc/` の成果物を `.gitignore` に入れる。
-   [x] WASM のビルド・`wasm_exec.js` の写し・拡張のビルドを 1 回で行う `Makefile` の目標を足す。作業ツリーに変更があれば、何もビルドせずに失敗する。
-   [-] `extension/scripts/check-dist.ts` が `poc/` の成果物を拒否するなら、拒否しないように変える。（不要だった：`check-dist.ts` は `static/poc/` の成果物を拒否しなかった。）

#### ステップ 2-5: ビルドと push

-   [x] `make ext-build` と上の `Makefile` の目標が通ることを確かめる。
-   [x] AC-14 の観点で PoC のコードを読み直す。API キーが保存先・URL・`console`・エラーの表示に流れないことを確かめる。
-   [x] 測定の前に、PoC のブランチの `origin/main` からの範囲を `/weakreview --no-fix` でレビューし、結果を測定の記録（§1.3）に書く。テスト用の API キーを通す前に、PoC のコードを自分以外の目で読むためである。
-   [x] PoC のブランチを origin に push する。

### フェーズ 3: 測定

設計書 §6.3・§7.3・§7.4。フェーズ 2 と同じく対話のセッションで進め、`/runplan` は実装しない。実 API の呼び出しは、1 回ずつ人の承認を得る。

#### ステップ 3-1: 準備

-   [x] 測定のためのブラウザのプロファイルを作り、クラッシュの報告を切る（設計書 §5.2）。
-   [x] テスト用のワークスペースの利用の上限と、コンソールの使用量の始まりの値を記録する。

#### ステップ 3-2: 設計書 §7.3 の順の測定

各回の値を、測定の記録（§1.3）に書きためる。API キー・ワークスペース ID・記事と字幕の本文は書かない（AC-28）。

測定の途中で PoC のコードや manifest を変えるとき（CSP・`host_permissions`・ヘッダの組み合わせ、Offscreen Document）は、次のとおりにする。

-   変えるたびに PoC のブランチにコミットを足す。amend と force push はしない。
-   ステップ 2-4 の `Makefile` の目標（設計書 §3.4）でビルドし直す。
-   測定の記録の各回に、その回のビルドの SHA を書く。

-   [x] 1. プローブ（Chrome・Brave）。
-   [x] 2. 読み込みと CSP、サイズと読み込みの時間（Chrome・Brave、3 回以上）。
-   [x] 3. CORS の 3 つの組み合わせ（Haiku）。
-   [x] 4. 問い 1・2（要件書の背景の 3 つの問いのうち 1 と 2）の試行（AC-17・AC-18・AC-22、Haiku、Chrome・Brave）。
-   [x] 5. 長い応答（AC-19、Haiku）。
-   [x] 6. 本番の構成の 3 本（AC-20、Chrome）。動画は 0008 評価記録 §3.2 の 5 本から、動画 1（`roNZ-rLxhUE`）を含む 3 本を選び、字幕は文字起こしパネルからコピーする。
-   [-] 7. Offscreen Document（AC-21）。問い 2 が失敗したブラウザがあるときだけ行う。作る前に設計書 §3.5 の例外を人に承認してもらい、承認の後に設計書 §3.5 の表のファイルを作って同じ試行を行う。行わなかったときは、その理由を記録する。（行わなかった：Chrome と Brave のどちらにも問い 2 の失敗がなかった。理由は測定の記録に書いた。）
-   [x] 8. 後片付け。テスト用の API キーを失効させ、新しいキーに替える。（キーは失効させた。新しいキーは、次に要るときに作る。）

#### ステップ 3-3: 測定の後の確かめ

-   [x] PoC のブランチの最後の状態を origin に push する。
-   [x] 設計書 §7.4 の確かめ（拡張のストレージ・Cookie・パスワードの管理）を行い、結果を記録する。
-   [x] 測定の途中で取ったログ（`chrome://net-export` の記録・HAR・画面の写し）を消す（AC-28）。（`net-export` の記録 2 つを消した。HAR と画面の写しは取っていない。ダミーのキーだけを含むプローブ用サーバのログは、ステップ 4-2 の読み合わせの後に、測定の記録と一緒に消す。）
-   [x] PR-2 のブランチに、フェーズ 2・3 のチェックを付けたコミットを積む（§1.3）。件名は `docs(0084): record the PoC and measurement progress` とし、`docs(0084): draft` で始めない（`/runplan` の作業ブランチの判定が、それを文書のコマンドのコミットとみなすため）。

### フェーズ 4: 調査記録（`main`）

設計書 §3.6。`/runplan` で進める。

#### ステップ 4-1: 調査記録を書く

**変えるファイル:** `docs/research/wasm_pipeline_poc/README.md`（新規）。

-   [x] 設計書 §3.6 の見出しで、3 つの問いへの答え（AC-24）・測定の条件（AC-25）・測った値・確かめていない事項（AC-27）を書く。
-   [x] PoC のブランチの名前と、測ったコミットの SHA を書く（AC-25・AC-26）。
-   [x] AC-19 の見積もり（本番の 3 本で観測した Opus の出力の速さから求めた、40,000 トークンの所要時間）を、長い応答の所要時間と並べ、答えと分けて書く（AC-24）。

#### ステップ 4-2: 調査記録を確かめる

-   [x] PoC のブランチが origin にあり、測定の記録と調査記録に書いたすべての SHA がそのブランチから辿れ、`origin/main` からは辿れないことを確かめる。実行したコマンドと出力を、§7 の AC-26 の行に書く。
-   [x] 調査記録に、API キー・ワークスペース ID・記事と字幕の本文がないことを読んで確かめる（AC-28）。
-   [x] 調査記録の値が、測定の記録（§1.3）と合っていることを読み合わせる。
-   [ ] 読み合わせを終えたら、測定の記録（§1.3）を消す。

### PR-2 作成ポイント: PoC measurement record

`/runplan` のタスクの終わりの確認では、AC-07〜AC-23 を、要件書 §5「受け入れの確かめ方の例外」による `manual` の確かめとして受け入れる。

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2

**推奨タイトル**: `docs(0084): WASM 版のパイプラインの PoC の調査記録を加える`

**レビュー観点**: 3 つの問いへの答えが、AC-24 の判定の規則と測った値に合っていること / 記録した SHA の PoC のコードが、入口の検査（`validateBody` との対応）と API キーの扱い（AC-14）を満たすこと / 調査記録に API キー・ワークスペース ID・本文が含まれないこと（AC-28） / 確かめていない事項（AC-27）が漏れなく挙がっていること / 記録した SHA が PoC のブランチから辿れ、`origin/main` から辿れないこと（§7 の AC-26）と、費用の合計が目安の内であること（AC-23）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2〜2-4 と 3-2 は前例のない PoC と実現性の測定（Phase 0 型の PoC）に当たる。対話のセッションでも、この水準のモデルを使う。`/runplan` が書く調査記録も、PoC のコードを読んで問いに答える。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果 | 完了の条件 |
|---|---|---|
| M1 | コアの分割が `main` に入る | PR-1 のマージ |
| M2 | PoC のブランチが origin にある | ステップ 2-5 の完了 |
| M3 | 測定が終わり、テスト用の API キーを替えた | ステップ 3-3 の完了 |
| M4 | 調査記録が `main` に入る | PR-2 のマージ |

順は設計書 §8 のとおりである。フェーズ 2 は PR-1 のマージの後の `main` から始める。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `internal/ytdlpsource` への移動、`internal/writer` の分割、`make wasm-check` と CI、文書と ADR の追随、`_checks/` のスクリプト | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 | 調査記録（`docs/research/wasm_pipeline_poc/README.md`）と本書のチェック。PoC のコードは PoC のブランチにだけ置き、この PR のレビューで記録した SHA のコードを読む | frontier-required |

PR の分け方の理由：PR-1 は `main` に入るコードの変更で、それだけでグリーンゲートを通る。PR-2 は文書だけの変更で、PoC のコードと測定の結果をまとめてレビューする（要件書 §5）。フェーズ 2・3 は PR を開かないので、記録を入れる PR-2 に含める。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト。** フェーズ 1 は新しい振る舞いを足さないので、新しいテストを書かない。既存のテストを移し、変えずに通す（AC-04）。テストの関数を消さないので、カバレッジの比較（`make coverage-compare`）の対象にならない。
-   **`js` のビルドの振る舞い。** `template_open_js.go` は、ネイティブのテストではコンパイルされない。`make wasm-check` の `go vet` で型の正しさを検査し、振る舞い（ファイルシステムに触れずに失敗する）は PR-1 のレビューでコードを読んで確かめる。PoC のブランチでは、入口から上書きのパスを渡す経路がない（設計書 §3.2 の入口は空のパスを渡す）。
-   **統合テスト。** `make test-integration` の対象のパッケージが変わる。統合テストは yt-dlp とネットワークを使う。PR-1 では、`go vet -tags integration ./...`（`make lint` の一部）でコンパイルを、`go test -tags integration -list` で新しい場所にテストがあることを確かめる（ステップ 1-1）。
-   **後方互換性。** CLI とサーバの振る舞いが変わらないことは、AC-04・AC-05 の `diff` の検証と、変えずに通る既存のテストで確かめる。
-   **PoC。** テストを書かない（§1.2）。AC-07〜AC-23 は、PR-2 のレビューで、記録した値・観測と、記録した SHA のコードを読んで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 移動の書き換えに、§1.4 の試行で見つからなかった衝突が残る | PR-1 のビルドが通らない | ステップ 1-1 の完了の条件で、`-tags test,integration` の vet まで確かめる。AC-05 の許す範囲で直せないものは、`/runplan` の決定の変更として人に尋ねる |
| `make wasm-check` を足すと、`make lint` の時間が延びる | 開発の待ち時間 | WASM のビルドは 4 つのパッケージだけで、数秒の見込みである。目立って延びたら PR-1 のレビューで扱う |
| 測定の費用が目安を超える | 予算の超過 | 実 API の呼び出しごとに承認を得て、各回の概算を合計する。約 4 USD を超えそうなら止めて尋ねる（設計書 §6.3） |
| 問い 2 が失敗し、Offscreen Document の試行が要る | 作業と費用が増え、API キーの扱いが AC-14 と両立しない | ステップ 3-2 の 7 で、作る前に人の承認を得る（設計書 §3.5） |
| テスト用の API キーが記録や画面に残る | キーの漏れ | 専用のプロファイルで測り、ログをコミットせずに消し、測定の後にキーを失効させる（ステップ 3-2 の 8・3-3） |
| タスク 0085（[#817](https://github.com/isseis/yt2column/issues/817)、承認済み）が `package_reference.md` を種類ごとの表に分ける | PR-1 と書き換えが衝突し、AC-05 の差分の記録が古くなる | 後にマージする側が、`internal/ytdlpsource` の行を 0085 の構成要素の表に置く。PR-1 が `main` を取り込んだら、AC-05 の差分の記録を取り直す |
| 測定の間に `main` が先へ進む | 調査記録が古いコードの値になる | 記録には測った SHA を書く。`make wasm-check` が、WASM でビルドできる状態を保つ |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 | 担う PR |
|---|---|---|---|
| AC-01 | static | `make wasm-check`（ステップ 1-3）、`_checks/` の AC-01・AC-02 のスクリプト（ステップ 1-5） | PR-1 |
| AC-02 | static | `_checks/` の AC-01・AC-02 のスクリプト（ステップ 1-5） | PR-1 |
| AC-03 | static | `make test`・`make lint`・`make deadcode`（`make deadcode` は CI で走らないので、出力とコミットの ID を PR-1 の説明に書く） | PR-1 |
| AC-04 | diff | `_checks/` の AC-04〜AC-06 のスクリプト（ステップ 1-5） | PR-1 |
| AC-05 | diff | Go のファイル・`Makefile`・`.github/workflows/ci.yml`・変わったファイルの一覧は `_checks/` の AC-04〜AC-06 のスクリプト。`docs/dev/conventions/package_reference.md` は差分の記録の読み（下の記録。ファイルの一覧を含む） | PR-1 |
| AC-05 | manual | `js` のビルドの `readOverrideFile` がファイルシステムに触れず、2 つの番兵を元のエラーとして含めて返すことは、PR-1 のレビューでコードを読んで確かめる。`js` のテストを走らせるには Node.js の実行器が要り、Go の手順から Node を呼ばない（[ADR-0026](../../dev/adr/0026-extension-platform.md)）ため、テストにしない | PR-1 |
| AC-06 | diff | `_checks/` の AC-04〜AC-06 のスクリプト | PR-1 |
| AC-29 | static | `make wasm-check`、`make lint`（`wasm-check` を呼ぶ）。失敗しうることは、ステップ 1-3 の壊し方で確かめてコミットメッセージに書く | PR-1 |
| AC-29 | static | `ci.yml` の `lint` のジョブに `make wasm-check` の手順があることは、`_checks/` の AC-04〜AC-06 のスクリプト（コミットするスクリプト）で確かめる | PR-1 |
| AC-29 | manual | 補助として、PR-1 の CI の `lint` のジョブの記録で、`make wasm-check` の手順が走ったことを見る | PR-1 |
| AC-07〜AC-23 | manual | PR-2 のレビューで、調査記録の値・観測と、記録した SHA の PoC のコードを読む（要件書 §5「受け入れの確かめ方の例外」） | PR-2 |
| AC-24・AC-25・AC-27・AC-28 | manual | PR-2 のレビューで調査記録を読む（要件書 §5） | PR-2 |
| AC-26 | manual | ステップ 4-2 で実行したコマンドと出力（この行に書く）を、PR-2 のレビューで読む | PR-2 |

スクリプトの結果（ステップ 1-5 で書く）:

-   AC-01・AC-02 のスクリプト（`_checks/check_wasm.sh`、merge-base `e6e051aedf2a2f4066bdd4f39c4a7504f9e945f0`、PR のコミット `8d7157f0adacd7ab71c32bddb4c6a4cdd539c844`）：成功（終了状態 0）。出力：

    ```text
    OK AC-01: build of 4 packages and vet of writer and transcript for js/wasm
    OK AC-02: no ytdlpsource dependency, no repository package imports os/exec
    ```
-   AC-04〜AC-06 のスクリプト（`_checks/check_diff.sh`、merge-base `e6e051aedf2a2f4066bdd4f39c4a7504f9e945f0`、PR のコミット `8d7157f0adacd7ab71c32bddb4c6a4cdd539c844`）：成功（終了状態 0）。出力：

    ```text
    OK files: every changed file is one the design names
    OK moved files: 12 files differ only in the designed places
    OK test_helpers.go: split differs only by the duplicated readTestdataFile and testVideoID
    OK callers: only import lines, qualifiers, and the envaccess path string changed
    OK writer: template_open.go holds exactly the lines that left template.go
    OK Makefile and ci.yml: only the designed lines changed
    OK AC-06: .golangci.yml, go.mod, go.sum unchanged
    ```

Diff record (AC-05):

- Compared paths: `docs/dev/conventions/package_reference.md`
- Merge-base: `e6e051aedf2a2f4066bdd4f39c4a7504f9e945f0`
- Compared commit: `b996eb69cd9a839e4de41f2a88ba3d714aa3c646`

Path diff:

````text
diff --git a/docs/dev/conventions/package_reference.md b/docs/dev/conventions/package_reference.md
index 2c1287bdf692aded1fd47d419de9b0d50110ec5d..b3c7f6ff54d50846e97e73ee7b05440310ce0603 100644
--- a/docs/dev/conventions/package_reference.md
+++ b/docs/dev/conventions/package_reference.md
@@ -70,8 +70,9 @@ add a kind here or to widen one of these definitions.
 | `internal/pipeline` | Runs the three stages in order: `Pipeline`, `Stage`, `StageError`, `ErrNilStage`. |
 | `internal/pricing` | Estimates an LLM call's cost from its token counts and the providers' published rates: `Estimate`, `Totals`, `StageCost`, `Amount`, `RateBasis`, `UnknownReason`. Updating the rates: [pricing update guide](../guides/pricing_update.md). |
 | `internal/publisher` | Defines the publishing stage: `Publisher`, `FilePublisher`, `SlackWebhookPublisher`, `PrepareSlackMessages`, `FileContent`, `ReviewLogWriter`; `ErrOutputExists`, `*KeptFileError`, `SlackHTTPStatusError`, `*SlackPostError`. File format: [CLI guide](../guides/cli.md); post text: [README](../../../README.md#slack-への投稿). |
-| `internal/transcript` | Defines the transcript stage (`Transcript`, `TranscriptSource`) with `YtDlpSource` and `CopiedSource`; `ValidateVideoURL` and `NormalizedVideoURL` are the video URL rules. Cache guarantees: [cache consistency](../cache_consistency.md); the `yt-dlp` boundary: [security.md](../security.md). |
+| `internal/transcript` | Defines the transcript stage (`Transcript`, `TranscriptSource`) with `CopiedSource`; `ValidateVideoURL` and `NormalizedVideoURL` are the video URL rules. The `yt-dlp` path and its cache live in `internal/ytdlpsource`. |
 | `internal/writer` | Defines the article-writing stage: `Article`, `ArticleWriter`, `ProgressWriter`, `New`, `NewProgressWriter`, `RunUsage`, `CheckChannelName`, `ProseRanges`. The template contract: [prompts/README.md](../../../prompts/README.md). |
+| `internal/ytdlpsource` | Implements `transcript.TranscriptSource` with `YtDlpSource`: runs `yt-dlp` without a shell and parses its json3 and info.json output. It is the development path of the CLI and is not built into the WebAssembly core. The cache's guarantees: [cache consistency](../cache_consistency.md); the `yt-dlp` boundary: [security.md](../security.md). |
 | `prompts` | Embeds the default prompt templates and returns them: `System`/`User`, `ShortenSystem`/`ShortenUser`, `ReviewSystem`/`ReviewUser`. [README](../../../prompts/README.md) documents the contract. |

 ## Server and Host
@@ -112,7 +113,7 @@ add a kind here or to widen one of these definitions.
 | `internal/redact` | Keeps secret values out of output: `Line`, `Writer`, `SecretStrings`. |
 | `internal/secret` | Holds a secret (API key, Webhook URL) and guarantees it never appears in `fmt`, `log/slog`, or JSON output. |
 | `internal/signalrun` | `Run` calls a program's main function under a context that the first subscribed signal cancels; a second signal terminates the process. |
-| `internal/strictjson` | Extracts values from JSON documents that passed strict checks: `Object.CollectOnly`, `Object.Has`, `Value.AsBool`, `Value.IsNull`, `Value.AsNonNegativeInt64`, `Value.Raw`. The rejected inputs: [0003 design §3.5](../../tasks/0003_deepseek_llm_client/02_architecture.md). Shared by `internal/transcript`, `internal/llm/deepseek`, `internal/llm/claude`, `internal/llm/openai`, `internal/publisher`, `internal/writer`, `internal/artifact`, `internal/fixprguard`, `internal/server`, `internal/nmhost`, `internal/codexreview`, and `internal/coveragecompare` |
+| `internal/strictjson` | Extracts values from JSON documents that passed strict checks: `Object.CollectOnly`, `Object.Has`, `Value.AsBool`, `Value.IsNull`, `Value.AsNonNegativeInt64`, `Value.Raw`. The rejected inputs: [0003 design §3.5](../../tasks/0003_deepseek_llm_client/02_architecture.md). Shared by `internal/ytdlpsource`, `internal/llm/deepseek`, `internal/llm/claude`, `internal/llm/openai`, `internal/publisher`, `internal/writer`, `internal/artifact`, `internal/fixprguard`, `internal/server`, `internal/nmhost`, `internal/codexreview`, and `internal/coveragecompare` |

 ## Development Tools

````

File list (only for an AC that limits which files change):

````text
.claude/commands/mkplan.md
.claude/commands/runplan.md
.github/workflows/ci.yml
CLAUDE.md
Makefile
cmd/yt2column/run_test.go
cmd/yt2column/test_helpers_integration.go
docs/dev/adr/0019-pipeline-component-interfaces.md
docs/dev/adr/0033-transcript-cache-slots.md
docs/dev/cache_consistency.md
docs/dev/conventions/package_reference.md
docs/dev/guides/development.md
docs/dev/project_overview.md
docs/tasks/0018_article_quality/_material_check/main.go
docs/tasks/0084_wasm_pipeline_poc/01_requirements.md
docs/tasks/0084_wasm_pipeline_poc/02_architecture.md
docs/tasks/0084_wasm_pipeline_poc/03_implementation_plan.md
docs/tasks/0084_wasm_pipeline_poc/_checks/check_diff.sh
docs/tasks/0084_wasm_pipeline_poc/_checks/check_wasm.sh
internal/cachelock/cachelock_test.go
internal/config/envaccess_test.go
internal/job/job.go
internal/job/job_test.go
internal/transcript/cache.go
internal/transcript/cache_test.go
internal/transcript/exec.go
internal/transcript/exec_test.go
internal/transcript/info.go
internal/transcript/info_test.go
internal/transcript/integration_test.go
internal/transcript/json3.go
internal/transcript/json3_test.go
internal/transcript/sentinel_test.go
internal/transcript/test_helpers.go
internal/transcript/test_helpers_cache_seed.go
internal/transcript/ytdlp.go
internal/transcript/ytdlp_test.go
internal/writer/template.go
internal/writer/template_open.go
internal/writer/template_open_js.go
internal/ytdlpsource/cache.go
internal/ytdlpsource/cache_test.go
internal/ytdlpsource/doc.go
internal/ytdlpsource/exec.go
internal/ytdlpsource/exec_test.go
internal/ytdlpsource/info.go
internal/ytdlpsource/info_test.go
internal/ytdlpsource/integration_test.go
internal/ytdlpsource/json3.go
internal/ytdlpsource/json3_test.go
internal/ytdlpsource/test_helpers.go
internal/ytdlpsource/test_helpers_cache_seed.go
internal/ytdlpsource/ytdlp.go
internal/ytdlpsource/ytdlp_test.go
````
AC-26 の確かめ（ステップ 4-2、2026-11-12）：PoC のブランチが origin にあり、測定の記録と調査記録に書いた PoC のコミットのすべてが、そのブランチから辿れ、`origin/main`（`03fecb1e2316746bf2606473ee6eb3471baf2c34`）から辿れない。`git merge-base --is-ancestor` の終了状態は、辿れるとき 0、辿れないとき 1 である。測定の記録に書いた分岐の元 247c1d17 は、`origin/main` と PoC のブランチの merge-base そのものなので、対象から除いた。

```bash
git fetch -q origin main '+refs/heads/poc/0084-wasm-pipeline:refs/remotes/origin/poc/0084-wasm-pipeline'
git ls-remote --exit-code origin refs/heads/poc/0084-wasm-pipeline
for c in 2e369055 395f3feb 51b7aafe f016d7c7 5bcc3c4b a7b09d81 83ef12bd; do
  git merge-base --is-ancestor $c origin/poc/0084-wasm-pipeline; a=$?
  git merge-base --is-ancestor $c origin/main; m=$?
  echo "$(git rev-parse --short=8 $c) poc=$a main=$m"
done
git merge-base origin/main origin/poc/0084-wasm-pipeline
```

```text
83ef12bd019bf1456a0fb77aa7d7f35a2c848c8a	refs/heads/poc/0084-wasm-pipeline
2e369055 poc=0 main=1
395f3feb poc=0 main=1
51b7aafe poc=0 main=1
f016d7c7 poc=0 main=1
5bcc3c4b poc=0 main=1
a7b09d81 poc=0 main=1
83ef12bd poc=0 main=1
247c1d172576ca95b0f7895426068202cd6e407d
```

## 8. 成功基準 (Success Criteria)

-   **機能の完成度。** AC-01〜AC-06・AC-29 を PR-1 で、AC-07〜AC-28 を PR-2 で満たす。
-   **品質。** PR-1 で `make test`・`make lint`・`make deadcode`・`make wasm-check` が通り、既存のテストを 1 つも消さず、アサーションも変えない。
-   **セキュリティ。** フェーズ 1 で、`js` 以外のビルドの開くフラグと yt-dlp の起動の守りが変わらない（AC-05 の `diff` の検証）。測定の後にテスト用の API キーを失効させ、調査記録にキーと本文がない（AC-28）。
-   **文書。** 移したファイルのパスを書いた文書（§1.4 の表）と ADR-0019・ADR-0033 が、分割の後の構成に合う。調査記録が `main` に入る。

## 9. 次のステップ (Next Steps)

-   本書の承認の後、`/runplan` でフェーズ 1 を進め、PR-1 を開く。
-   PR-1 のマージの後、対話のセッションでフェーズ 2・3 を進める（§1.3）。その後、`/runplan` でフェーズ 4 を進め、PR-2 を開く。
-   PR-2 のマージの後、#771 を閉じ、調査記録を根拠に #772 で構成を決める。

## 変更履歴

-   2026-11-11 承認（isseis、#830）
-   2026-11-11 セッション中の再承認：番兵の一覧を固定するテスト（`TestExportedSentinelErrorsUnchanged`）を `internal/transcript` のテストに残し、その補助の複製を許す（isseis、#843）
-   2026-11-11 セッション中の再承認：入口が送るモデルを固定の 2 つに限り、止まった呼び出しとみなすまでの時間を呼び出しごとに決める（ステップ 2-2・2-4。設計書 §1.2.2）（isseis、#未記入）
