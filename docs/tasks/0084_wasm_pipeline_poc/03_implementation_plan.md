# 実装計画書：Go の WASM 版のパイプラインを拡張のページで動かす PoC と、構成の判断に要る値の測定

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md) の F-001〜F-005 を、[02_architecture.md](02_architecture.md) の設計に従って実装し、測定する手順を定める。作業は設計書 §8 の 4 つの段階に対応する 4 つのフェーズに分かれる。

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

### 1.3. 計画の際に決めたこと（2026-11-11）

-   **`YtDlpSource.Fetch` のローカル変数 `transcript` を `cached` に改名し、要件書 AC-05 と設計書 §3.1.3 にこの書き換えを足した。** `internal/transcript/ytdlp.go:69`〜`74` では、ローカル変数 `transcript` のスコープの中で `Transcript{}` を返している。移した後にこれを `transcript.Transcript{}` と修飾すると、ローカル変数がパッケージ名を隠すため、コンパイルできない。これは、作業用の写しでファイルを実際に移して確かめた（§1.4）。import に別名を付ける案は、そのファイルのすべての修飾の綴りが変わり、AC-04・AC-05 の `diff` の検証も複雑になるので採らなかった。人の答えは「変数を改名」である。決定の変更なので、要件書と設計書は `draft` に戻し、本書の PR で承認し直す。ADR: なし（このタスクの中で閉じる）。
-   **フェーズ 2・3 は `/runplan` の外の対話のセッションで進め、`/runplan` はフェーズ 4 から進める。** `/runplan` は、次の PR の作成ポイントまでを 1 つのブランチで実装し、グリーンゲートとレビューを経て PR を開く作りである。PoC のコードは別のブランチ（マージしない）に置き、測定は人がブラウザを操作して実 API の呼び出しを 1 回ずつ承認するので、この流れに収まらない（設計書 §8）。手順は次のとおりとし、人の答えは「対話セッションで」である。
    1.  PR-1 のマージの後、`/runpr` が次のブランチ（PR-2 のブランチ）を作る。
    2.  人が対話のセッションで指示し、PoC のブランチを `origin/main` から作って、フェーズ 2 を実装する。続けてフェーズ 3 を測る。
    3.  同じセッションで、PR-2 のブランチにフェーズ 2・3 のチェックを付けたコミットを積む。測った値は、まだ文書にしない形でスクラッチの領域に置く。
    4.  `/runplan` がフェーズ 4 を進め、調査記録を書いて PR-2 を開く。

    ADR: なし（このタスクの進め方の決定で、タスクの外に及ばない）。
-   **PoC のブランチの名前は `poc/0084-wasm-pipeline` とする。** 作業ブランチの命名の規約（`<GitHub のユーザ名>/<slug>-<NN>`）に合わない名前にすることで、`/runplan`・`/runpr` が自分の作業ブランチと取り違えないようにする。ADR: なし。

### 1.4. 既存コード調査結果

調べたのはコミット 4135c266 の時点のコードで、Go は 1.27.1 である。

**分割の試行。** 作業用の写し（`git archive` で展開したもの）で、設計書 §3.1.4 のファイルを `internal/ytdlpsource` に移し、識別子の修飾を書き換えて、ビルドの結果を見た。

-   移すファイルが残る側から使う識別子は、公開の型・番兵・関数（`Segment`・`Transcript`・`TranscriptSource`・`ValidateVideoURL`・`ErrNoSubtitles`・`ErrParseInfo`・`ErrParseSubtitles`・`ErrYtDlpExec`・`ErrInvalidVideoURL`・`ParseError`）と、非公開の `videoIDPattern` だけだった。`videoIDPattern` を使うのは、設計書 §3.1.3 の 2 か所である。
-   修飾の書き換えでコンパイルできないのは、`internal/transcript/ytdlp.go:69`〜`74` の 1 か所だけだった（§1.3）。テストのコード（`-tags test,integration`）には、同じ衝突はなかった。
-   移した後の 4 つのパッケージ（`internal/transcript`・`internal/writer`・`internal/llm/claude`・`internal/pricing`）は、次の 2 つの検査を通った。`internal/writer` の開くフラグは、この試行では `os.O_RDONLY` に置き換えて確かめた。
    -   `GOOS=js GOARCH=wasm go build`。
    -   `internal/writer` と `internal/transcript` への `GOOS=js GOARCH=wasm go vet`。
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

**文書を確かめる既存のテスト。**

-   `internal/pipeline/pipeline_test.go:629` の `TestPackageReferenceListsPackages` は、`package_reference.md` に `internal/ytdlpsource` の行がないと失敗する。
-   `internal/pipeline/strictjson_users_test.go:609` の `TestPackageReferenceStrictjsonUsers` は、`internal/strictjson` の行の「Shared by」が import の実際と合わないと失敗する。
-   どちらも、ステップ 1-4 の文書の書き換えを `make test` で確かめる。新しい文書のテストは足さない。

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

**変えるファイル:** 設計書 §3.1.4 の表の Go のファイルと `Makefile` の `test-integration` の目標。

-   [ ] `git mv` で、`exec.go`・`ytdlp.go`・`cache.go`・`info.go`・`json3.go` と、そのテスト（`cache_test.go`・`exec_test.go`・`info_test.go`・`json3_test.go`・`ytdlp_test.go`・`integration_test.go`・`test_helpers_cache_seed.go`）を `internal/ytdlpsource/` へ移す。
-   [ ] 移したファイルのパッケージ名の行・import の行・修飾を書き換える。
-   [ ] `YtDlpSource.Fetch` のローカル変数 `transcript` を `cached` に改名する（設計書 §3.1.3）。
-   [ ] `pruneCandidateID` を `transcript.NormalizedVideoURL` の 2 つ目の戻り値で判定する形に書き換える（設計書 §3.1.3）。
-   [ ] `internal/transcript/test_helpers.go` を分ける。§1.4 の分け方のとおり、移す側だけが使う補助を `internal/ytdlpsource/test_helpers.go` へ移し、`readTestdataFile` は両方に置き、`testVideoID` は設計書 §3.1.3 の形にする。ビルドの制約の行（`//go:build test`）は両方に置く。
-   [ ] `internal/ytdlpsource/doc.go` に、パッケージの doc コメントを英語で書く。
-   [ ] §1.4 の「移す宣言を使う、パッケージの外の箇所」の 6 か所の import と修飾を書き換える。
-   [ ] `internal/config/envaccess_test.go` のパスの文字列を `internal/ytdlpsource/ytdlp.go` に書き換える。`:412` の、パスに対応づけた偽のソースの `package` の行も合わせる。許可する参照の種類と数は変えない。
-   [ ] `Makefile:216` の `test-integration` のパスを `./internal/ytdlpsource` に書き換える。
-   [ ] `docs/tasks/0018_article_quality/_material_check/main.go` を `go build` し、通ることを確かめる（`./...` に含まれないため）。

**完了の条件:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。`go vet -tags test,integration ./internal/ytdlpsource` が通る。

#### ステップ 1-2: `internal/writer` の上書き用のテンプレートを開く処理を分ける

**変えるファイル:** `internal/writer/template.go`、`internal/writer/template_open.go`（新規）、`internal/writer/template_open_js.go`（新規）。

-   [ ] `overrideOpenFlags` と `readOverrideFile` を `template.go` から `template_open.go`（`//go:build !js`）へ、中身を変えずに移す（設計書 §3.1.2）。
-   [ ] `template_open_js.go`（`//go:build js`）に、ファイルシステムに触れずに失敗する `readOverrideFile` を書く。エラーは `ErrInvalidTemplate` と `errors.ErrUnsupported` の両方を包み、文には `templateSource.String` を含める（設計書 §4.1）。
-   [ ] `js` のビルドのこの振る舞いは、ネイティブの `make test` ではコンパイルされないので、Go のテストを足さない。ステップ 1-3 の `make wasm-check` の `go vet` で検査する（設計書 §7.1）。

**完了の条件:** `make test`・`make lint` が通る。`internal/writer` の既存のテンプレートのテスト（`template_test.go`）は変えずに通る。

#### ステップ 1-3: `make wasm-check` と CI の手順を足す

**変えるファイル:** `Makefile`、`.github/workflows/ci.yml`。

-   [ ] `Makefile` に `wasm-check` の目標を足し、`.PHONY` に加える。目標は、AC-01 の 4 つのパッケージを `GOOS=js GOARCH=wasm go build` し、`internal/writer` と `internal/transcript` に `GOOS=js GOARCH=wasm go vet` を掛ける（設計書 §1.2）。
-   [ ] `lint` の目標から `wasm-check` を呼ぶ。
-   [ ] `ci.yml` の `lint` のジョブに、`make wasm-check` を走らせる手順を 1 つ足す。
-   [ ] 検査が失敗しうることを確かめる。作業用のコミットで `internal/writer/template_open_js.go` を消して `make wasm-check` が失敗することを見てから、そのコミットを捨てる。確かめたことをコミットメッセージに書く（設計書 §7.1）。

**完了の条件:** `make wasm-check` と `make lint` が通り、`make lint` の出力に `wasm-check` の実行が現れる。

#### ステップ 1-4: 文書を移動に合わせる

**変えるファイル:** 設計書 §3.1.4 の表の文書と、`docs/dev/adr/0019-pipeline-component-interfaces.md`。

-   [ ] `docs/dev/conventions/package_reference.md`：`internal/transcript` の行から yt-dlp の経路を外し、`internal/ytdlpsource` の行を足す。`internal/strictjson` の行の「Shared by」で、`internal/transcript` を `internal/ytdlpsource` に替える。
-   [ ] `CLAUDE.md`：「Architecture Overview」の `TranscriptSource` の段落の `YtDlpSource` の置き場所を書き換える。「Code Quality」の一覧に `make wasm-check` を足す。
-   [ ] `.claude/commands/runplan.md:71`・`.claude/commands/mkplan.md:64` の例の場所を `internal/ytdlpsource` に書き換える。
-   [ ] `docs/dev/project_overview.md:112` のディレクトリの一覧を書き換える。
-   [ ] `docs/dev/cache_consistency.md` のパッケージとファイルの場所（`:3`・`:158`・`:170`）を書き換える。
-   [ ] `docs/dev/adr/0033-transcript-cache-slots.md:24` のパッケージの名前を直す。編集上の訂正として、コミットメッセージに訂正の行を書く。
-   [ ] `docs/dev/adr/0019-pipeline-component-interfaces.md` の 2 つの食い違いの記述（§1.4）を消す。同じく編集上の訂正として、訂正の行を書く。

**完了の条件:** `make test`（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestInstructionDocPaths` を含む）が通る。

#### ステップ 1-5: 一度だけの検査のスクリプトを書き、結果を記録する

**変えるファイル:** `docs/tasks/0084_wasm_pipeline_poc/_checks/` の下のスクリプト、本書の §7。

-   [ ] AC-01・AC-02 を確かめるシェルのスクリプトを書く。確かめる性質は次の 3 つである。
    -   AC-01 の 4 つのパッケージが `GOOS=js GOARCH=wasm go build` を通る。`internal/writer` と `internal/transcript` が `GOOS=js GOARCH=wasm go vet` を通る。
    -   4 つのパッケージの `GOOS=js GOARCH=wasm go list -deps` に、`internal/ytdlpsource` が含まれない。
    -   依存のうち、このリポジトリのパッケージが、どれも `os/exec` を import しない。
-   [ ] AC-04・AC-05・AC-06 を確かめるシェルのスクリプトを書く。merge-base と PR のコミットを比べ、Go のファイル（製品のコードとテストのコード）について次の性質を確かめる。
    -   パッケージ名の行・import の行・ビルドの制約の行・移動に伴う修飾を正規化すると、移した宣言と書き換えた呼び出し側の差分がなくなる。
    -   正規化の後に残る差分は、設計書 §3.1.3 の表の箇所（ローカル変数の改名を含む）と、ステップ 1-2 の分割と、`doc.go` だけである。
    -   変わった Go のファイルの一覧が、設計書 §3.1.4 の表の Go のファイルに限られる。
    -   `Makefile` の差分が、`test-integration` のパスの書き換えと、`wasm-check` の目標・`.PHONY` への追加・`lint` からの呼び出しに限られる。`.github/workflows/ci.yml` の差分が、`lint` のジョブに足す 1 つの手順に限られる。
    -   `.golangci.yml`・`go.mod`・`go.sum` に差分がない（AC-06）。
-   [ ] どちらのスクリプトも、各検査が少なくとも 1 つの壊し方で失敗することを確かめる。
-   [ ] PR-1 のブランチで 2 つのスクリプトを走らせ、出力と両方のコミットの ID を §7 の該当する行に書く。
-   [ ] AC-05 のうち `docs/dev/conventions/package_reference.md` の差分は、差分の記録を読む形で確かめる（文書の小さな差分のため）。§7 の差分の記録の欄に、記録を書く。

**完了の条件:** 2 つのスクリプトがどちらも成功を報告し、§7 の AC-01・AC-02・AC-04・AC-05・AC-06 の行に結果が記録されている。

### PR-1 作成ポイント: core split for the WASM build

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0084): yt-dlp の経路を internal/ytdlpsource に分け、コアを WASM でビルドできるようにする`

**レビュー観点**: 移動の差分が AC-04・AC-05 の許す書き換えに限られること（`_checks/` の出力） / `js` のビルドの `readOverrideFile` がファイルシステムに触れず、2 つの番兵を包むこと / `make wasm-check` が `make lint` と CI の `lint` のジョブから走ること / 移したファイルのパスを書いた文書と ADR の書き換えに漏れがないこと

**実装モデル要件**: standard

**判定理由**: ファイルの移動と書き換えは機械的で、§1.4 の試行で方式を確かめ済みである。競合する方式も、2 つ以上に当たる条件付きの検査（Conditional checks）もない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: PoC のコード（PoC のブランチ）

設計書 §3.2〜§3.5。§1.3 のとおり、`/runplan` の外の対話のセッションで進める。`/runplan` はこのフェーズを実装しない。チェックが付いていなければ、止まって報告する。

PoC のブランチでは、`make ext-build` が通ることだけを求める。`make ext-check`・`make test` は求めない（設計書 §5.3）。

#### ステップ 2-1: PoC のブランチを作る

-   [ ] PR-1 のマージの後の `origin/main` から、`poc/0084-wasm-pipeline` を作る（§1.3）。

#### ステップ 2-2: WASM の入口（`cmd/yt2column-wasm`）

設計書 §3.2・§4.2・§6.1。

-   [ ] `//go:build js && wasm` の `main` パッケージとして作り、`globalThis.yt2column` に `buildInfo`・`generate`・`longOutput`・`probe` を登録する。
-   [ ] `generate` の入力の検査を、`internal/server/request.go:63`〜`122` の `validateBody` と同じ段・同じ順で書く。PoC で加えたメンバの検査も書く（設計書 §3.2「受け付ける形」）。
-   [ ] `writer.NewProgressWriter` を、CLI と同じ条件（埋め込みのテンプレート・字数の調整と検証・推敲を有効・期限 15 分）で作る。15 分の値の出典として、`provider.LLMTimeout` をコメントに書く。
-   [ ] 失敗の種類を、設計書 §4.2 の表のとおり番兵で選ぶ。`invalid_config` の文は固定にする。
-   [ ] 工程ごとの値を、`ProgressObserver` の時刻と `RunUsage`・`pricing.Estimate` から組み立てる（設計書 §2.4・§3.2）。
-   [ ] 4 つの関数の panic を回復する。
-   [ ] `browserAccessHeader` のときだけヘッダを足す `http.DefaultTransport` の包みを入れる（設計書 §1.2）。
-   [ ] `longOutput` と `probe` を設計書 §3.2 のとおりに作る。`probe` は送り先を検査してから送る。

#### ステップ 2-3: プローブ用サーバ（`cmd/yt2column-wasmprobe`）

-   [ ] 設計書 §3.3 の表の経路と記録を持つ、ネイティブのサーバを作る。`localhost` のポートは `127.0.0.1` と `::1` の両方で待ち受ける。
-   [ ] ログをリポジトリの外に書くことを、起動の説明に書く（AC-28）。

#### ステップ 2-4: PoC のページとビルド

設計書 §3.4。

-   [ ] `extension/static/poc.html` と `extension/src/poc.ts` を作る。ボタンの規則・ランタイムの終わり・20 分の打ち切り・ビルドの同一性・API キーの扱い・表示を、設計書 §3.4 のとおりにする。
-   [ ] 記事を `parseArticleMarkdown` と `buildArticleBody` で描く。失敗の文と工程ごとの値は `textContent` でだけ表示する。
-   [ ] `extension/static/poc/` の成果物を `.gitignore` に入れる。
-   [ ] WASM のビルド・`wasm_exec.js` の写し・拡張のビルドを 1 回で行う `Makefile` の目標を足す。作業ツリーに変更があれば、何もビルドせずに失敗する。
-   [ ] `extension/scripts/check-dist.ts` が `poc/` の成果物を拒否するなら、拒否しないように変える。

#### ステップ 2-5: ビルドと push

-   [ ] `make ext-build` と上の `Makefile` の目標が通ることを確かめる。
-   [ ] AC-14 の観点で PoC のコードを読み直す。API キーが保存先・URL・`console`・エラーの表示に流れないことを確かめる。
-   [ ] PoC のブランチを origin に push する。

### フェーズ 3: 測定

設計書 §6.3・§7.3・§7.4。フェーズ 2 と同じく対話のセッションで進め、`/runplan` は実装しない。実 API の呼び出しは、1 回ずつ人の承認を得る。

#### ステップ 3-1: 準備

-   [ ] 測定のためのブラウザのプロファイルを作り、クラッシュの報告を切る（設計書 §5.2）。
-   [ ] テスト用のワークスペースの利用の上限と、コンソールの使用量の始まりの値を記録する。

#### ステップ 3-2: 設計書 §7.3 の順の測定

各回の値を、スクラッチの領域の測定の記録に書きためる。API キー・ワークスペース ID・記事と字幕の本文は書かない（AC-28）。

-   [ ] 1. プローブ（Chrome・Brave）。
-   [ ] 2. 読み込みと CSP、サイズと読み込みの時間（Chrome・Brave、3 回以上）。
-   [ ] 3. CORS の 3 つの組み合わせ（Haiku）。
-   [ ] 4. 問い 1・2 の試行（AC-17・AC-18・AC-22、Haiku、Chrome・Brave）。
-   [ ] 5. 長い応答（AC-19、Haiku）。
-   [ ] 6. 本番の構成の 3 本（AC-20、Chrome）。
-   [ ] 7. Offscreen Document（AC-21）。問い 2 が失敗したブラウザがあるときだけ行い、作る前に設計書 §3.5 の例外を人に承認してもらう。行わなかったときは、その理由を記録する。
-   [ ] 8. 後片付け。テスト用の API キーを失効させ、新しいキーに替える。

#### ステップ 3-3: 測定の後の確かめ

-   [ ] 設計書 §7.4 の確かめ（拡張のストレージ・Cookie・パスワードの管理）を行い、結果を記録する。
-   [ ] 測定の途中で取ったログ（`chrome://net-export` の記録・HAR・画面の写し）を消す（AC-28）。
-   [ ] PR-2 のブランチに、フェーズ 2・3 のチェックを付けたコミットを積む（§1.3）。

### フェーズ 4: 調査記録（`main`）

設計書 §3.6。`/runplan` で進める。

#### ステップ 4-1: 調査記録を書く

**変えるファイル:** `docs/research/wasm_pipeline_poc/README.md`（新規）。

-   [ ] 設計書 §3.6 の見出しで、3 つの問いへの答え（AC-24）・測定の条件（AC-25）・測った値・確かめていない事項（AC-27）を書く。
-   [ ] PoC のブランチの名前と、測ったコミットの SHA を書く（AC-25・AC-26）。
-   [ ] 記録を書き終えたら、測定の記録のスクラッチを消す。

#### ステップ 4-2: 調査記録を確かめる

-   [ ] PoC のブランチが origin にあり、記録した SHA がそのブランチから辿れ、`origin/main` からは辿れないことを確かめる。実行したコマンドと出力を、§7 の AC-26 の行に書く。
-   [ ] 調査記録に、API キー・ワークスペース ID・記事と字幕の本文がないことを読んで確かめる（AC-28）。
-   [ ] 調査記録の値が、測定の記録と合っていることを読み合わせる。

### PR-2 作成ポイント: PoC measurement record

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2

**推奨タイトル**: `docs(0084): WASM 版のパイプラインの PoC の調査記録を加える`

**レビュー観点**: 3 つの問いへの答えが、AC-24 の判定の規則と測った値に合っていること / 記録した SHA の PoC のコードが、入口の検査（`validateBody` との対応）と API キーの扱い（AC-14）を満たすこと / 調査記録に API キー・ワークスペース ID・本文が含まれないこと（AC-28） / 確かめていない事項（AC-27）が漏れなく挙がっていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2〜2-4 と 3-2 は、前例のない PoC と実現性の測定（Phase 0 型の PoC）に当たる。

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
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `internal/ytdlpsource` への移動、`internal/writer` の分割、`make wasm-check` と CI、文書と ADR の追随、`_checks/` のスクリプト | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 | 調査記録（`docs/research/wasm_pipeline_poc/README.md`）と本書のチェック。PoC のコードは PoC のブランチにだけ置き、この PR のレビューで記録した SHA のコードを読む | frontier-required |

PR の分け方の理由：PR-1 は `main` に入るコードの変更で、それだけでグリーンゲートを通る。PR-2 は文書だけの変更で、PoC のコードと測定の結果をまとめてレビューする（要件書 §5）。フェーズ 2・3 は PR を開かないので、記録を入れる PR-2 に含める。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト。** フェーズ 1 は新しい振る舞いを足さないので、新しいテストを書かない。既存のテストを移し、変えずに通す（AC-04）。テストの関数を消さないので、カバレッジの比較（`make coverage-compare`）の対象にならない。
-   **`js` のビルドの振る舞い。** `template_open_js.go` は、ネイティブのテストではコンパイルされない。`make wasm-check` の `go vet` で型の正しさを検査し、振る舞い（ファイルシステムに触れずに失敗する）は PR-1 のレビューでコードを読んで確かめる。PoC のブランチでは、入口から上書きのパスを渡す経路がない（設計書 §3.2 の入口は空のパスを渡す）。
-   **統合テスト。** `make test-integration` の対象のパッケージが変わる。yt-dlp と network を使うので、PR-1 では、`go vet -tags integration ./...`（`make lint` の一部）でコンパイルだけを確かめる。
-   **後方の互換性。** CLI とサーバの振る舞いが変わらないことは、AC-04・AC-05 の `diff` の検証と、変えずに通る既存のテストで確かめる。
-   **PoC。** テストを書かない（§1.2）。AC-07〜AC-23 は、PR-2 のレビューで、記録した値・観測と、記録した SHA のコードを読んで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 移動の書き換えに、§1.4 の試行で見つからなかった衝突が残る | PR-1 のビルドが通らない | ステップ 1-1 の完了の条件で、`-tags test,integration` の vet まで確かめる。AC-05 の許す範囲で直せないものは、`/runplan` の決定の変更として人に尋ねる |
| `make wasm-check` を足すと、`make lint` の時間が延びる | 開発の待ち時間 | WASM のビルドは 4 つのパッケージだけで、数秒の見込みである。目立って延びたら PR-1 のレビューで扱う |
| 測定の費用が目安を超える | 予算の超過 | 実 API の呼び出しごとに承認を得て、各回の概算を合計する。約 4 USD を超えそうなら止めて尋ねる（設計書 §6.3） |
| 問い 2 が失敗し、Offscreen Document の試行が要る | 作業と費用が増え、API キーの扱いが AC-14 と両立しない | ステップ 3-2 の 7 で、作る前に人の承認を得る（設計書 §3.5） |
| テスト用の API キーが記録や画面に残る | キーの漏れ | 専用のプロファイルで測り、ログをコミットせずに消し、測定の後にキーを失効させる（ステップ 3-2 の 8・3-3） |
| 測定の間に `main` が先へ進む | 調査記録が古いコードの値になる | 記録には測った SHA を書く。`make wasm-check` が、WASM でビルドできる状態を保つ |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 | 担う PR |
|---|---|---|---|
| AC-01 | static | `make wasm-check`（ステップ 1-3）、`_checks/` の AC-01・AC-02 のスクリプト（ステップ 1-5） | PR-1 |
| AC-02 | static | `_checks/` の AC-01・AC-02 のスクリプト（ステップ 1-5） | PR-1 |
| AC-03 | static | `make test`・`make lint`・`make deadcode` | PR-1 |
| AC-04 | diff | `_checks/` の AC-04〜AC-06 のスクリプト（ステップ 1-5） | PR-1 |
| AC-05 | diff | Go のファイル・`Makefile`・`.github/workflows/ci.yml` は `_checks/` の AC-04〜AC-06 のスクリプト。`docs/dev/conventions/package_reference.md` は差分の記録の読み（下の記録） | PR-1 |
| AC-06 | diff | `_checks/` の AC-04〜AC-06 のスクリプト | PR-1 |
| AC-29 | static | `make wasm-check`、`make lint`（`wasm-check` を呼ぶ）。失敗しうることは、ステップ 1-3 の壊し方で確かめてコミットメッセージに書く | PR-1 |
| AC-29 | manual | PR-1 の CI の `lint` のジョブの記録で、`make wasm-check` の手順が走ったことを見る | PR-1 |
| AC-07〜AC-23 | manual | PR-2 のレビューで、調査記録の値・観測と、記録した SHA の PoC のコードを読む（要件書 §5「受け入れの確かめ方の例外」） | PR-2 |
| AC-24・AC-25・AC-27・AC-28 | manual | PR-2 のレビューで調査記録を読む（要件書 §5） | PR-2 |
| AC-26 | manual | ステップ 4-2 で実行したコマンドと出力（この行に書く）を、PR-2 のレビューで読む | PR-2 |

スクリプトの結果（ステップ 1-5 で書く）:

-   AC-01・AC-02 のスクリプト：未記入
-   AC-04〜AC-06 のスクリプト：未記入

Diff record (AC-05):

- Compared paths: `docs/dev/conventions/package_reference.md`

AC-26 の確かめ（ステップ 4-2 で書く）：未記入

## 8. 成功基準 (Success Criteria)

-   **機能の完成度。** AC-01〜AC-06・AC-29 を PR-1 で、AC-07〜AC-28 を PR-2 で満たす。
-   **品質。** PR-1 で `make test`・`make lint`・`make deadcode`・`make wasm-check` が通り、既存のテストを 1 つも消さず、アサーションも変えない。
-   **セキュリティ。** フェーズ 1 で、`js` 以外のビルドの開くフラグと yt-dlp の起動の守りが変わらない（AC-05 の `diff` の検証）。測定の後にテスト用の API キーを失効させ、調査記録にキーと本文がない（AC-28）。
-   **文書。** 移したファイルのパスを書いた文書（§1.4 の表）と ADR-0019・ADR-0033 が、分割の後の構成に合う。調査記録が `main` に入る。

## 9. 次のステップ (Next Steps)

-   本書の承認の後、`/runplan` でフェーズ 1 を進め、PR-1 を開く。
-   PR-1 のマージの後、対話のセッションでフェーズ 2・3 を進める（§1.3）。その後、`/runplan` でフェーズ 4 を進め、PR-2 を開く。
-   PR-2 のマージの後、#771 を閉じ、調査記録を根拠に #772 で構成を決める。
