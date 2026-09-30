# 実装計画書：yt-dlp による字幕取得

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-09-30 |
| Review date | 2026-09-30 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md`（以下「requirements」）で定義した字幕取得を、`02_architecture.md`（以下「architecture」）の設計どおりに `internal/transcript` へ実装する。`YtDlpSource` が `TranscriptSource` を実装し、動画 URL を検証して動画 ID を取り出し、`yt-dlp` を起動して字幕 json3 と info.json を取得・検証し、動画 ID ごとのキャッシュ（2 スロットとポインタ）で再利用・置き換え・削除・掃除を行う。実 `yt-dlp` とネットワークを使う統合テストは `make test-integration` に分離する。

本計画は architecture §8 の実装優先順位（純粋な処理 → 外部コマンドの境界 → キャッシュと `Fetch` → 統合テストと lint 経路 → ドキュメント）をそのままフェーズとする。

### 1.2. 実装原則

- architecture §1.1 の設計原則 6 項目に従う。特に、信頼できない入力は境界で検証して補正せず（AC-02・AC-03・AC-12・AC-15）、キャッシュの変更は成功時のポインタの置き換え 1 回だけとし（AC-34・AC-58）、`TranscriptSource`・`Transcript`・`Segment` と `internal/pipeline` は変更しない（AC-25）。
- 本番コードで新設するファイルは architecture §2.1・§3.8 に列挙された 7 ファイル（`errors.go`・`video_id.go`・`json3.go`・`info.go`・`exec.go`・`cache.go`・`ytdlp.go`）に限る。7 ファイルはすべて新設であり、8 つ目の本番ファイルは追加しない。json3 と info.json が共有する厳密デコード処理（architecture §3.4）は `json3.go` に置き、両パーサが同じパッケージ内で呼ぶ。
- ユニットテストは、`NewYtDlpSource` で構築した `YtDlpSource` の非公開の executor フィールドを fake に置き換える（同一パッケージのテストから差し替える。architecture §3.2・§3.3）。
- テストは architecture §7 に従う。ユニットテストは実 `yt-dlp` もネットワークも呼ばず、パッケージ内の fake `commandExecutor` と `testdata/` の実出力、`t.TempDir` のキャッシュを使う（AC-27）。
- パッケージ内で共有するテストヘルパーは `test_organization.md` の分類 B に従い、`internal/transcript/test_helpers.go` に `//go:build test` を付けて置く。`test_helpers.go` を使うすべての `_test.go` の先頭にも `//go:build test` を付ける（`internal/pipeline/pipeline_test.go:1` と同じ理由）。`integration_test.go` だけは `//go:build integration` とし、統合テストの環境変数の確認は共有ヘルパーに置かず `integration_test.go` 内で直接行う。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN` / `F-NNN` は Go ソースに書かず、本計画にだけ記録する（`runplan.md` の pre-commit チェック）。
- 上限値・猶予・固定名は名前付き定数にする（`mnd`・`goconst` 対策。`.golangci.yml:88-101` の `ignored-numbers` はパーミッションのみ）。
- 各フェーズの完了条件は `make fmt` → `make test` → `make lint` が通ること。各テストは、対応する分岐・対策を実際に壊して失敗することを確認してコミットメッセージに記録する（CLAUDE.md「Testing Strategy」）。

### 1.3. 既存コード調査結果

リポジトリの現状を HEAD `7180e80`（ブランチ `task/0002-ytdlp-transcript-source-02`）で確認した。追跡対象のファイルに未コミットの変更はない（本計画書は新規の未追跡ファイル）。

- **`internal/transcript` は既存の型と fake だけを持つ。** `Segment`・`Transcript`・`TranscriptSource`（`internal/transcript/transcript.go:8-28`）と fake（`internal/transcript/testutil/mocks.go`・`mocks_test.go`）があり、本タスクはこれらを変更しない（architecture §1.3）。本タスクで削除・改名する既存シンボルはなく、更新が必要になる既存テストもない。
- **新しい fake を `testutil/` に追加しない。** 本タスクの fake は未公開 interface `commandExecutor` 用で同一パッケージに置く（architecture §3.2・§3.3）。`internal/pipeline/pipeline_test.go:509-529` の `TestFakesCarryBuildTag` は `testutil/` のファイル数を 8 件に固定している（`:514`）ため、`testutil/` を増やさなければこの guard は変更不要である。
- **`internal/pipeline` の既存 guard は影響を受けない。** `TestCommonTypesFieldSets`（`internal/pipeline/pipeline_test.go:375`）・`TestInterfaceContracts`（`:423`）・`TestInterfaceDocComments`（`:448`）・`TestSecretRevealExclusive`（`:486`）は `transcript.go`・`secret` の契約を固定しており、本タスクはこれらを変更しない。`pipeline_test.go` は変更しない。
- **フィクスチャは一部コミット済みで、まだテストから使われていない。** 実出力の `testdata/2tcCWM-sRBw.ja.json3`・`testdata/2tcCWM-sRBw.info.json`・`testdata/README.md` はコミット `4bf3ffd` で追加済み（CC BY の出典と IP 置換の記録を含む）。`internal/`・`cmd/` から `testdata/` を参照するテストは現存しない（`rg -n "testdata/" internal cmd` の結果は 0 件、2026-09-30 に HEAD `7180e80` で実行）。既存の実出力は変更せず、AC-52・AC-53 が要求する不正バイト列・サロゲートの合成サンプルだけをフェーズ 1 で追加する。
- **Makefile は lint のタグが `test` のみで、`test-integration` を持たない。** `Makefile:10` の `GOLINT` は `run --build-tags test`、`:63-64` の `test` は `go test -tags test -race -v ./...`、`:67-68` の `test-ci` は `-tags test -race -coverprofile`。architecture §3.8 のとおり `test-integration` の追加と lint タグの `test,integration` 化が必要である。
- **pre-commit の lint タグだけが未変更。** `.pre-commit-config.yaml:23` は `run --build-tags test`。`testdata/` の除外は `check-added-large-files` が初期設定から（`:37`）、`trailing-whitespace`・`end-of-file-fixer` がコミット `4bf3ffd` で追加済み（`:32`・`:34`）のため、変更はタグの 1 箇所で足りる。
- **CI の lint タグも未変更。** `.github/workflows/ci.yml:88` は `--build-tags test --timeout=5m`。`make test-ci` は `:61` で実行される。
- **ドキュメントの更新対象。** `docs/dev/developer_guide/package_reference.md:14` の `internal/transcript` の行は現在「型と interface を定義する」であり、実装を追加する説明に更新する。`docs/dev/developer_guide/requirements_process.md:97` の境界チェックは「標準ライブラリが黙って修復・無視する入力は拒否する」と読め、消費しないメンバーを受理する本要件（requirements §3.2、AC-64・AC-67）と整合させる（`design_handoff.md` の H-13）。該当の言い回しは同ファイルにのみ現れる（`rg` で確認）。`docs/dev/project_overview.md:26-30` は最終的な起動引数と事前調査の実施時期を反映済みで、architecture §3.8 に変更対象として挙がっていない。
- **`cmd/yt2column/main.go` は変更しない。** 現在は空の `main()` のみで（`cmd/yt2column/main.go:1-4`）、設定読み込みと CLI 配線は #6 が行う。
- **外部前提の確認。** `go.mod:3` は `go 1.26.5`、手元のツールチェーンは go1.27.1（`go version`、2026-09-30）。`exec.Cmd.WaitDelay`（Go 1.20 以降）と `errors.AsType`（Go 1.26、CLAUDE.md:206-215）が使える。依存追加は不要で、標準ライブラリのみ（`.golangci.yml:49-59` の depguard）。ベースラインの `go test -tags test ./...` は本計画作成時に成功した（2026-09-30、HEAD `7180e80`。全パッケージが `ok` または `[no test files]`）。
- **architecture との整合。** 本計画は architecture §8 のフェーズ構成・順序をそのまま採用する。調査で architecture の修正が必要な不整合は見つかっていない。

### 1.4. implementation_handoff.md の項目への対応

| ID | 対応 |
|---|---|
| I-01 | フェーズ 1 で、json3・info.json の共通の厳密デコードが、トップレベルの値の後に実際に EOF へ到達したことを確認して後続データを拒否する（`TestParseSubtitlesRejectsSimple`・`TestParseInfoRejectsMalformed`）。`json.Decoder.More` を EOF 判定に使わない。 |
| I-02 | 該当しない。requirements F-003 の改訂でローリング重複の除去を取りやめ、`dDurationMs` を読まないため終端の計算が無い（architecture §3.7）。 |
| I-03 | フェーズ 1 で、消費するメンバーの重複を `json.Decoder.Token` によるトークン走査で検出する（対象は architecture §3.4 の列挙どおり。`TestParseSubtitlesDuplicateMembers`・`TestParseInfoDuplicateMembers`）。消費しないメンバーの重複は受理する。 |
| I-04 | フェーズ 3 で、ポインタの読み取りを有界にする（種別と大きさを lstat で確認し、1 バイトを超える内容を全体として読まない）。巨大なポインタはキャッシュミスになり、dangling なエントリとして削除される（`TestFetchOversizedPointer`）。 |

## 2. 実装ステップ (Implementation Steps)

各フェーズの完了条件は「そのフェーズの `make fmt` → `make test` → `make lint` が通ること」。ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名は §5 の受け入れ基準の検証表にまとめ、各フェーズのタスクでは重複して列挙しない。`internal/transcript` の責務が変わる各フェーズ（1〜3）で、`package_reference.md` の同パッケージの行をそのフェーズの変更に合わせて更新する（architecture §8 と `package_reference.md:3-4`）。

### フェーズ 1: 純粋な処理（`errors.go`・`video_id.go`・`json3.go`・`info.go`）

**対象ファイル**
- 新設: `internal/transcript/errors.go`・`internal/transcript/video_id.go`・`internal/transcript/json3.go`・`internal/transcript/info.go`
- 新設: `internal/transcript/video_id_test.go`・`internal/transcript/json3_test.go`・`internal/transcript/info_test.go`（すべて `//go:build test`）
- 新設: `internal/transcript/test_helpers.go`（`//go:build test`。このフェーズでは `testdata/` のパス定数のみ。フェーズ 3 で fake executor とキャッシュ状態ヘルパーを追加する）
- 新設: `testdata/invalid_utf8.json3`・`testdata/unpaired_surrogate.json3`・`testdata/invalid_utf8.info.json`・`testdata/unpaired_surrogate.info.json`（AC-52・AC-53 が要求する合成サンプル）
- 変更: `testdata/README.md`・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 1-1**: `errors.go` に architecture §3.1 の 5 つの番兵と `ParseError`（`Path`・`Err`、`Error()`・`Unwrap()`）を定義する。`Unwrap` は `ErrParseSubtitles` または `ErrParseInfo` を返す契約を英語のドキュメントコメントに書く。
- [x] **ステップ 1-2**: `video_id.go` に URL 検証（F-001・architecture §3.6）を実装する。スキーム・ホスト・パス形式・動画 ID の文字種・余分なパス要素を検証し、動画 ID と正規化 URL を返す。`v` 以外のクエリパラメータとフラグメントは無視し、URL 文字列は他へ渡さない。拒否は理由を付けて `ErrInvalidVideoURL` をラップする。
- [x] **ステップ 1-3**: `video_id_test.go` を作成し、§5 の AC 表に挙げた URL 検証のテストを実装する（AC-01〜AC-05）。
- [x] **ステップ 1-4**: AC-52・AC-53 が要求する `testdata/` の合成サンプルを 4 ファイル追加する。json3 と info.json のそれぞれについて、不正な UTF-8 バイト列を含むものと、対になっていないサロゲートのエスケープ（例: `"utf8":"\ud800"`・`"title":"\ud800"`）を含むものを用意する。あわせて `testdata/README.md` に、これらが実出力ではなくテスト用の合成サンプルであることと、既存の実出力の条件（CC BY・IP 置換）が適用されないことを記載し、4 ファイルの名前と内容（不正バイト列 / サロゲートエスケープ）を突き合わせて確認する。
- [x] **ステップ 1-5**: `test_helpers.go` を作成し、`testdata/` の実出力と合成サンプルのパスを定数として定義する。以降のテストファイルは相対パスのリテラルを書かず、この定数を使う。
- [x] **ステップ 1-6**: json3 と info.json の共通の厳密デコード（architecture §3.4、design_handoff H-05・H-12、implementation_handoff I-01・I-03）を `json3.go` に実装し、`info.go` から呼ぶ。生バイト列の UTF-8 検証、対になっていないサロゲートエスケープの検出、トップレベルがちょうど 1 つの値で後続データが無いこと（EOF 到達）の確認、`null` と型不一致の区別、`int64` 範囲の検証、消費するメンバーのトークン段階での重複検出を含める。消費しないメンバーの重複は無視する。新たな本番ファイルは追加しない。
- [x] **ステップ 1-7**: `json3.go` に json3 パーサ（F-003・architecture §3.4・§3.7）を実装する。`events[].segs[].utf8` の連結、`segs` なし・空 `utf8`・空白のみのイベントの除外、本文を持つイベントだけの `tStartMs` 検証（H-16）、重複除去を行わないこと、サイズ 8 MiB・イベント数 65,536 の上限、失敗時の `ErrParseSubtitles` を返す `*ParseError` を実装する。
- [x] **ステップ 1-8**: `json3_test.go` を作成し、§5 の AC 表に挙げた json3 パーサのテストを実装する（AC-10〜AC-13・AC-32・AC-45・AC-46・AC-48・AC-49・AC-52・AC-54・AC-55・AC-57・AC-67）。AC-52 はステップ 1-4 のサンプルを入力にし、パースの前にサンプル自身の性質（不正な UTF-8 バイト列を含む / `\ud800` のエスケープを含む）を検証して、サンプルが別の理由で不正な JSON と判定され、本来確かめたい検証が働いたか分からないままテストが成功するのを防ぐ。`TestParseSubtitlesUTF8` には受け入れケースも加える。有効な高/低サロゲートペア（非 BMP 文字、例: `\ud83d\ude00`）を含む正当な入力、およびエスケープされたバックスラッシュに続けて `ud800` と書いた通常テキスト（`\\ud800`）を含む正当な入力が受理されることを検証し、生バイト列の判定が正当な入力を過剰に拒否しないことを確かめる。
- [x] **ステップ 1-9**: `info.go` に info.json パーサ（F-004）を実装する。`id` と要求された動画 ID の照合、`title`・`channel` の必須と空の拒否、`description` の任意、消費フィールドの型検証、サイズ 8 MiB の上限、`ErrParseInfo` を返す `*ParseError` を実装する。
- [x] **ステップ 1-10**: `info_test.go` を作成し、§5 の AC 表に挙げた info.json パーサのテストを実装する（AC-14〜AC-16・AC-47・AC-50・AC-53・AC-64・AC-65）。AC-53 はステップ 1-4 のサンプルを入力にし、ステップ 1-8 と同じくサンプル自身の性質を先に検証する。`TestParseInfoUTF8` にもステップ 1-8 と同じ受け入れケース（有効なサロゲートペアと `\\ud800` の通常テキストの受理）を加える。
- [x] **ステップ 1-11**: `package_reference.md:14` の `internal/transcript` の行を、このフェーズで加わる URL 検証・json3/info.json パーサ・番兵エラーを含む説明に更新する。`testutil` の行は変更しない。
- [x] **ステップ 1-12**: 主要な分岐を実装時に壊して各テストが失敗することを確認し、コミットメッセージに記録する。対象の例: EOF 確認を外すと `TestParseSubtitlesRejectsSimple`・`TestParseInfoRejectsMalformed` が失敗する、`null` をゼロ値として受理すると `TestParseSubtitlesElementKinds`・`TestParseInfoTypeMismatch` が失敗する、上限の境界（ちょうど/超過）を取り違えると `TestParseSubtitlesLimits`・`TestParseInfoLimits` が失敗する、破棄イベントの `tStartMs` も検証すると `TestParseSubtitlesTimestamps` が失敗する、動画 ID を補正すると `TestValidateVideoURL` が失敗する、生バイト列の UTF-8 検証を外すと `TestParseSubtitlesUTF8` が失敗する、サロゲートのエスケープ検出を外すと `TestParseSubtitlesUTF8`・`TestParseInfoUTF8` が失敗する。
- [x] **ステップ 1-13**: `make fmt` → `make test` → `make lint` を通す。`test_helpers.go` はこのフェーズの `make test`（`-tags test`）でコンパイルされることを確認する。

### PR-1 作成ポイント: pure processing (errors, URL validation, parsers)

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13

**推奨タイトル**: `feat(0002): add sentinel errors, URL validation, and transcript parsers`

**レビュー観点**: 消費するフィールドだけを厳密に検証し、未知メンバーの重複は受理する境界が実装とテストで一致していること（AC-64・AC-67） / 不正 UTF-8・対になっていないサロゲート・後続データ・`null` とゼロ値の区別が標準ライブラリの補正に頼らず拒否されること（AC-12・AC-52〜AC-55・AC-57） / URL 検証が補正せず、動画 ID 以外の要素をキャッシュのパスや `yt-dlp` の引数に使わないこと（AC-02〜AC-05） / `testdata/` の合成サンプルが意図した理由で不正になっており、テストがサンプル自身の性質を先に検証していること（AC-13・AC-52・AC-53）

**実装モデル要件**: standard

**判定理由**: 純粋な処理とそのテストに限られ、競合する実装方針の併記・リカバリや状態機械などの高リスクな制御・パネルモードのトリガーに該当せず、Conditional checks も build-tag のコンパイル確認（`test_helpers.go`）の 1 件だけのため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 外部コマンドの境界（`exec.go`）

**対象ファイル**
- 新設: `internal/transcript/exec.go`・`internal/transcript/exec_test.go`（`//go:build test`）

**タスク**
- [x] **ステップ 2-1**: `exec.go` に `commandExecutor` interface（architecture §3.2）と `os/exec` を使う実装を追加する。実装は、引数配列での起動（シェルを経由しない）、`exec.CommandContext` によるタイムアウト・キャンセル、`WaitDelay` 相当の猶予 5 秒（architecture §3.7、定数化）、受け取った `io.Writer` の標準エラー出力への接続、allowlist 環境の組み立て（architecture §3.7 の集合、設定されている変数のみ、allowlist の変数が 1 つも設定されていなくても非 nil の空スライスを渡すこと、秘密情報を含めないこと）を含む。`Run` は生の実行結果を返し、タイムアウト・キャンセルの判別は行わない（`Fetch` の責務。architecture §3.2）。
- [x] **ステップ 2-2**: 同じく `exec.go` に、標準エラー出力を 4 KiB まで保持しつつ超過分も読み捨てる `cappedWriter` を追加する。標準エラー出力の伏字化（architecture §4.2）はこのフェーズでは追加しない。呼び出し元の `Fetch` と検証する `TestFetchStderrCapAndRedaction` がフェーズ 3 で揃うため、このフェーズで追加すると呼び出しもテストもない非公開関数になり、`unused` によって `make lint` のゲートが通らない（ステップ 3-3 で追加する）。
- [x] **ステップ 2-3**: `exec_test.go` を作成し、§5 の AC 表に挙げたテストを実装する（AC-07〜AC-09・AC-44・AC-51・AC-56・AC-62・AC-66）。`t.TempDir` に置いたヘルパー実行ファイル（標準エラー出力を 4 KiB を大きく超えて書いてから非ゼロ終了するもの、および標準エラー出力を開いたまま残る子孫を起動するもの）を使い、ドレインと `WaitDelay` による有界な待機を検証する。`WaitDelay` の検証は標準エラー出力を `io.Writer` に接続した状態で行う（接続しないと os/exec がパイプを作らず、猶予の経路に入らない）。`TestCommandExecutorEnvAllowlist` は、シェルスクリプトのヘルパーが自身の変数（`PWD`・`SHLVL`・`_`）を子の環境に加えてしまうため、テストバイナリを `-test.run` で再実行するヘルパーに環境をそのまま報告させる（architecture §7.1 が認める「テストバイナリの再実行」）。待機の有界性は固定の猶予 + 十分な余裕で判定し、厳密な時間比較にしない。実 executor をヘルパー実行ファイルを通して直接検証する `TestCommandExecutorEnvAllowlist`（子プロセスが受け取る環境が allowlist の設定済み変数とその値だけで、allowlist の変数が 1 つも設定されていなければ非 nil の空環境）、`TestCommandExecutorNoShell`（シェルメタ文字を含む引数がシェルに解釈されずそのまま届く）、`TestCommandExecutorStartFailure`（存在しないパスと実行不能ファイルで executor がエラーを返して握り潰さない。`Fetch` が `ErrYtDlpExec` へ対応付ける検証は `Fetch` が加わるフェーズ 3 で同テストに追加する）も実装する。
- [x] **ステップ 2-4**: `package_reference.md` の `internal/transcript` の行を、このフェーズで加わる外部コマンド実行の境界を含む説明に更新する。
- [x] **ステップ 2-5**: 主要な分岐を壊して失敗を確認し、コミットメッセージに記録する。対象の例: 4 KiB の境界（ちょうど/超過）を取り違えると `TestCappedWriter` が失敗する、`cappedWriter` のドレインを止めると `TestCommandExecutorDrainsStderr` が失敗する、`WaitDelay` を外すと `TestCommandExecutorWaitDelay` が失敗する、allowlist を組み立てず親環境をそのまま `exec.Cmd.Env` に渡すと `TestCommandExecutorEnvAllowlist` が失敗する、引数を `sh -c` 経由で渡すように変えると `TestCommandExecutorNoShell` が失敗する、実 `exec.Cmd` の起動エラーを握り潰すと `TestCommandExecutorStartFailure` が失敗する。
- [x] **ステップ 2-6**: `make fmt` → `make test` → `make lint` を通す。`gosec` の G204（可変のコマンド名での実行）が指摘された場合は、`exec.CommandContext` の呼び出しに限定した最小の `//nolint:gosec` を理由コメント付きで付ける（ファイル全体に広げない）。指摘の有無と対応をコミットメッセージに記録する。

### PR-2 作成ポイント: external command boundary (exec.go)

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `feat(0002): add the yt-dlp execution boundary with capped stderr and env allowlist`

**レビュー観点**: 引数を配列で渡しシェルを経由しないこと、タイムアウト・キャンセル後に `WaitDelay` で有界に戻ること（AC-07・AC-08） / 標準エラー出力の 4 KiB 保持とドレイン（読み取りを止めない）が `cappedWriter` と実プロセスの両方で成立すること（AC-51・AC-56） / allowlist 環境が設定済みの変数だけを渡し、1 つも設定されていなくても非 nil の空環境になること（AC-44・AC-66） / 実プロセスを使うテストが固定の猶予と余裕で判定し、厳密な時間比較で不安定になっていないこと（§4.1・§6）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-1・2-3 の実プロセスを使う `WaitDelay`・ドレインの検証が、子孫プロセスのライフサイクルを扱う孤立した複雑なステップ（リスク隔離の対象）に該当するため。競合する実装方針の併記とパネルモードのトリガーには該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: キャッシュと `Fetch`（`cache.go`・`ytdlp.go`）

**対象ファイル**
- 新設: `internal/transcript/cache.go`・`internal/transcript/ytdlp.go`
- 変更: `internal/transcript/exec.go`（標準エラー出力の伏字化を追加）
- 変更: `internal/transcript/exec_test.go`（`TestCommandExecutorStartFailure` に `Fetch` が実 executor の起動失敗を `ErrYtDlpExec` へ対応付ける検証を追加）
- 新設: `internal/transcript/cache_test.go`・`internal/transcript/ytdlp_test.go`（`//go:build test`）
- 変更: `internal/transcript/test_helpers.go`（fake `commandExecutor`、キャッシュ状態 S1〜S6 を作るヘルパーを追加）

**タスク**
- [x] **ステップ 3-1**: `test_helpers.go` に fake `commandExecutor` とキャッシュ状態ヘルパーを追加する。fake は引数・環境・渡された `io.Writer`（標準エラー出力）を記録し、ケースごとに設定できる関数で出力の書き込み・終了状態の模擬・`ctx` の状態に応じた失敗を行う。タイムアウト・キャンセルを模擬するときは、実装と同じく context のエラーではない失敗を返す（architecture §3.2）。キャッシュ状態ヘルパーは architecture §6.3 の S1〜S6 を固定名で直接作れるようにする。パーミッションを変えるヘルパーは、変えた時点で `t.Cleanup` に復元を登録する。`test_helpers.go` を使う `_test.go` に `//go:build test` を付け、`integration_test.go` は `//go:build integration` とする（§1.2）。
- [x] **ステップ 3-2**: `cache.go` に固定名の組み立て、ポインタの有界な読み取り（I-04）、ヒット判定とスロットの読み込み、書き込み先スロットの準備、ポインタの一時ファイルの排他作成とリネームによるコミット、dangling なエントリの判定と削除の機構を、architecture §3.5 の手順どおりに実装する。公開メソッド `RemoveCache`（ポインタを先に削除）と `PruneCache`（列挙・名前の判定・種別確認・`errors.Join` での集約・`ctx` の打ち切り）は `ytdlp.go` に置き、これらの機構を呼ぶ（ステップ 3-3）。ディレクトリは `0o700`、ファイルは `0o600`（AC-19）。他の動画・無関係なエントリには触れず、シンボリックリンクを辿らない。
- [x] **ステップ 3-3**: `ytdlp.go` に `Options`・`YtDlpSource`・`NewYtDlpSource`・`Fetch`・`RemoveCache`・`PruneCache` を実装する。`NewYtDlpSource` は実 executor を設定し、`CacheDir` が空、または `Timeout` が 0 以下なら拒否する（AC-28）。`Fetch` は architecture §6.1 の順序で進め、開始前に `ctx` の終了を確認し（AC-26）、キャッシュヒットでは `yt-dlp` を起動せず、ミス・強制再取得ではフェーズ 2 の executor と allowlist 環境で起動して検証・コミットする。動画 ID を得た後は成否によらず終了時に当該動画の dangling なエントリを削除する（ベストエフォート。AC-69）。エラーは architecture §4.1 の形で番兵をラップし、パース失敗は `*ParseError` でパスを保持し、標準エラー出力は伏字化してから含める。伏字化の処理（proxy 変数の値と URL の userinfo の置き換え、上限の境界で切れた断片も残さないこと。architecture §4.2）はこのステップで `exec.go` に追加する。環境の組み立ては architecture §3.8 で `exec.go` の責務とされているため、その秘密情報を守るこの処理も同じファイルに置く。`var _ TranscriptSource = (*YtDlpSource)(nil)` のコンパイル時アサーションを置く。
- [x] **ステップ 3-4**: `cache_test.go` を作成し、§5 の AC 表に挙げたキャッシュの削除・掃除のテストを実装する（AC-19・AC-20・AC-70〜AC-75）。パーミッションで失敗を起こすテスト（`TestRemoveCacheFailure`・`TestPruneCachePartialFailure`・`TestFetchDanglingDeleteFailure`）は非 root の実行環境（CI とローカル開発）を前提とし、root を検出したら「unsupported test environment」と明示して失敗させる（黙ってパス・スキップしない）。ディレクトリのモードを変えた時点で `t.Cleanup` に復元を登録する（§4.1）。
- [x] **ステップ 3-5**: `ytdlp_test.go` を作成し、§5 の AC 表に挙げた `Fetch` レベルのテストを実装する（AC-04・AC-06〜AC-09・AC-14・AC-17〜AC-18・AC-20〜AC-31・AC-34〜AC-36・AC-44・AC-51・AC-56・AC-58〜AC-63・AC-66・AC-68・AC-69）。テストは、構築後に非公開の executor フィールドを `test_helpers.go` の fake に置き換え、`Options.YtDlpPath` には存在しないパスを指定する（差し替え忘れが実 `yt-dlp` の起動にならないようにする）。`TestFetchCacheHit` は `testdata/` の実出力を配置したキャッシュから `Transcript` の `VideoID`・正規化 `VideoURL`・`Title`・`ChannelName`・`Description` に入ること、および別形式の URL（`youtu.be/<id>`）で要求したヒットでも `VideoID` と `VideoURL` が同じ正規化値になることを検証する（AC-14・AC-25、design_handoff H-15）。`Title`・`ChannelName`・`Description` の検証は、テストがフィクスチャからパーサとは独立に（例: `encoding/json` でテスト自身がデコードして）導出した互いに異なる期待値と比較する形にし、フィールドの取り違え（`Title` と `ChannelName` の交換）や 1 つの値の全フィールドへの流用が失敗するようにする。期待値はフィクスチャから導出し、日本語の値を Go ソースのリテラルとして埋め込まない。`TestFetchCacheMiss` でも `VideoID` と正規化 `VideoURL` を検証する。`TestFetchSentinelDistinction` は、URL 検証・`ErrYtDlpExec`・`ErrParseSubtitles`・`ErrParseInfo`・`ErrNoSubtitles`・`context.DeadlineExceeded`・`context.Canceled` の各カテゴリの失敗を実 `Fetch` の経路で 1 回ずつ発生させ、各エラーが自分の番兵または context エラーと一致し、他のどの番兵・context エラーとも一致しないことを検証する（AC-24。正の一致だけでなく相互の非一致も固定する）。`TestFetchOversizedPointer` のポインタは、有効な 1 バイト（`a` または `b`）で始まり 1 バイトを大きく超える内容にし、先頭 1 バイトだけを見る実装を検出できるようにする。
- [x] **ステップ 3-6**: `package_reference.md` の `internal/transcript` の行を、`YtDlpSource`（`Fetch`・`RemoveCache`・`PruneCache`）とキャッシュを含む最終的な説明に更新する。
- [x] **ステップ 3-7**: 主要な分岐を壊して失敗を確認し、コミットメッセージに記録する。対象の例: `Fetch` 終了時の dangling の削除を外すと `TestFetchInterruptedStates` が失敗する、`RemoveCache` でポインタを最後に削除すると `TestRemoveCacheFailure` が失敗する、掃除でディレクトリを列挙せず固定名だけを見ると `TestPruneCache` が失敗する、コミット前にキャッシュを書き換えると `TestFetchForceRefreshFailureKeepsCache` が失敗する、検証順序を info.json 優先に変えると `TestFetchValidationOrder` が失敗する、allowlist を `os.Environ()` に変えると `TestFetchEnvAllowlist` が失敗する、ポインタの大きさの確認を外して先頭 1 バイトだけを見る実装にすると `TestFetchOversizedPointer` が失敗する、キャッシュ読み込みの `ParseError` に書き込み先スロットのパスを入れると `TestFetchCacheParseError` が失敗する、dangling の削除失敗で `Fetch` の戻り値を変えると `TestFetchDanglingDeleteFailure` が失敗する、字幕なしのエラーから動画 ID を外すと `TestFetchNoSubtitles` が失敗する、すべての失敗に無関係な番兵（例: `ErrInvalidVideoURL`）を `errors.Join` で混ぜると `TestFetchSentinelDistinction` が失敗する。
- [x] **ステップ 3-8**: `make fmt` → `make test` → `make lint` を通す。`test_helpers.go` がこのフェーズの `make test`（`-tags test`）でコンパイルされることを確認する（`Makefile:63-64`）。

### PR-3 作成ポイント: cache and Fetch orchestration

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0002): add the transcript cache and YtDlpSource.Fetch`

**レビュー観点**: キャッシュを変更する点がポインタのリネーム 1 回だけで、コミット前の失敗・中断で既存キャッシュが変わらないこと（AC-34・AC-58・§6.3） / dangling なエントリの判定と削除が `Fetch` 時・`RemoveCache`・`PruneCache` で同じ規則を使い、削除失敗が `Fetch` の戻り値を変えないこと（AC-69・AC-70・AC-72・AC-75） / 字幕 → info.json の決定的な検証順序、番兵の相互判別、`*ParseError` のパスが成立すること（AC-24・AC-36・AC-68） / 標準エラー出力の伏字化と、固定名だけに触れてシンボリックリンクを辿らない規則が守られていること（AC-59・§4.2）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-2・3-3 のキャッシュ更新の状態機械（S1〜S6）と中断回復が、リカバリフローと状態機械を含む孤立した高リスクなステップに該当し、gosec G304 の抑制で Conditional checks にも該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 統合テストと lint 経路（`integration_test.go`・Makefile・pre-commit・CI）

**対象ファイル**
- 新設: `internal/transcript/integration_test.go`（`//go:build integration`）
- 変更: `internal/transcript/ytdlp_test.go`（`TestIntegrationTestBuildTag`・`TestLintTagsIncludeIntegration` を追加）
- 変更: `Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml`・本計画書（手動実行の記録）

**タスク**
- [x] **ステップ 4-1**: `integration_test.go` を作成する（F-008・architecture §7.2）。テスト自身が `YT2COLUMN_TEST_VIDEO_URL` の未設定・空を検出し、変数名を明示して失敗し、スキップしない（AC-42）。`YT2COLUMN_TEST_VIDEO_ID` が非空なら動画 ID を照合し、空なら照合しない。キャッシュは `t.TempDir` を使う。1 つのテスト関数内のサブテストとして、取得（AC-39）、目印ファイルを書き込むラッパー実行ファイルを `YtDlpPath` に指定した 2 回目のキャッシュ再利用（AC-40）、キャッシュを目印入りの有効な内容に置き換えたうえでの強制再取得（AC-41）を検証する。AC-40 は 2 回目の `Transcript` が 1 回目と等しく（メタ情報・セグメントの並び・各開始時刻）、目印ファイルが存在しないことを検証する。AC-41 は結果に目印が現れないことを検証する。ラッパー用と強制再取得用に `YtDlpSource` を構築し直す。`yt-dlp` 不在やネットワーク失敗はスキップせず失敗させる。あわせて `ytdlp_test.go` に `TestIntegrationTestBuildTag`（AC-37）を追加し、`integration_test.go` の先頭行が `//go:build integration` であること、およびファイルが存在しない・先頭行が違う場合は明確に失敗することを検証する。`integration_test.go` は `test_helpers.go`（`//go:build test`）のシンボルを使わず、`-tags integration` 単独でコンパイルできるようにする（`make test-integration` は `test` タグを付けないため）。
- [x] **ステップ 4-2**: `Makefile` に `test-integration` ターゲットを追加する。既定値を Make 変数（`YT2COLUMN_TEST_VIDEO_URL`・`YT2COLUMN_TEST_VIDEO_ID`、architecture §3.7 の URL・ID）として `?=` で定義し、実 `yt-dlp` とネットワークを使うことを表示し、`-tags integration`・`-count=1`・明示的な `-timeout`・`-v` を付けて `./internal/transcript` を実行する（AC-38）。テストは環境変数を `os.Getenv` で読むため、レシピは両変数をテストプロセスの環境へ明示的に渡す（Make 変数の定義だけでは子プロセスへ渡らない）。`.PHONY` に追加する。あわせて `GOLINT` のタグを `test,integration` に変更する（AC-43）。
- [x] **ステップ 4-3**: `.pre-commit-config.yaml:23` の golangci-lint フックのタグを `test,integration` に変更する（AC-43）。`testdata/` の除外（`:32`・`:34`・`:37`）は変更済みであることを確認し、変更しない。
- [x] **ステップ 4-4**: `.github/workflows/ci.yml:88` の lint 引数のタグを `test,integration` に変更し、`ytdlp_test.go` に `TestLintTagsIncludeIntegration` を追加する。この guard は `Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の 3 箇所すべてが `test,integration` を含むことを検証し、3 箇所のタグが揃っていることを機械的に固定する（AC-43）。あわせて、golangci-lint は `test` タグのヘルパーと一緒にしか `integration_test.go` をコンパイルせず、`make test-integration` が実際に使う `-tags integration` 単独のビルドを検査しないため、3 箇所すべてに `go vet -tags integration ./...` を追加し（`make lint` のレシピ・pre-commit の `go-vet-integration` フック・CI の lint ジョブのステップ）、同じ guard がその存在も検証する（フェーズ 4 のレビューでの追加。`2362d57`）。guard は `--build-tags` を golangci-lint を実行する行に限って照合し、コメント中の記述では満たされないようにする。
- [ ] **ステップ 4-5**: 手動実行を完了条件として実施する（AC-33）。`make test-integration` を実行し（実 `yt-dlp` とネットワークを使うため、実施前にユーザーの承認を得る）、§5.1 に使用した動画 URL・動画 ID・結果を記録する。既定の動画が利用できない場合は、要件 F-008 に従って URL と ID を差し替え、その旨も記録する。
- [x] **ステップ 4-6**: 環境変数を設定せずにコミット済みの `integration_test.go` を `go test -tags integration ./internal/transcript` で直接実行し、スキップせず変数名を含むエラーで失敗することを確認して出力を記録する（AC-42）。
- [ ] **ステップ 4-7**: 主要な分岐を壊して失敗を確認し、コミットメッセージに記録する。対象の例: `integration_test.go` に lint 違反（未使用の変数など）を一時的に入れると `make lint` が失敗する（AC-43）、`TestIntegrationTestBuildTag` の検証対象の先頭行を `//go:build test` に変えると同テストが失敗する（AC-37）、3 箇所のうち 1 つの lint タグを `test` に戻すと `TestLintTagsIncludeIntegration` が失敗する（AC-43）、`make test-integration` から `-count=1` を外して 2 回実行すると 2 回目が `(cached)` を表示し、付けた場合は 2 回とも実行されることを `-v` 出力で確認して記録する（AC-38）。
- [ ] **ステップ 4-8**: `make fmt` → `make test` → `make lint` を通し、`make test-integration` も通す。

### PR-4 作成ポイント: integration test and lint tag paths

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8

**推奨タイトル**: `feat(0002): add the yt-dlp integration test and integration lint tags`

**レビュー観点**: 統合テストが `//go:build integration` で既定の `make test` から分離されていること（AC-37） / 環境変数の未設定をスキップせず、変数名を示して失敗させていること（AC-42） / `Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の 3 箇所の lint タグが `test,integration` に揃い、`TestLintTagsIncludeIntegration` が固定していること（AC-43） / `make test-integration` の `-count=1`・明示的なタイムアウト・`-v` と、手動実行の承認・結果の記録（AC-33・AC-38）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-1〜4-5 が実 `yt-dlp` とネットワークを使う重い統合テスト、CI・pre-commit の lint 経路、手動実行にわたり、mkplan.md ステップ 8 のパネルモードトリガー（重い統合テスト / CI / 外部リソースの面）に該当するため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: ドキュメント（`requirements_process.md`）

**対象ファイル**
- 変更: `docs/dev/developer_guide/requirements_process.md`

**タスク**
- [ ] **ステップ 5-1**: `requirements_process.md:97` の境界チェックの記述を、拒否の規則が「消費するフィールド」に適用されることを明示する形に修正する。要件が拡張可能と宣言する未消費メンバーは拒否理由にしないことを、同段落で分かるようにする（design_handoff H-13、requirements §3.2・AC-64・AC-67）。同ファイルの他の箇所（チェックリスト `:101-111`）に同じく対象を限定していない規則が残っていないか確認し、残っていれば同様に整合させる。
- [ ] **ステップ 5-2**: 正の確認と残骸の確認を行う。修正後の該当箇所が「消費するフィールド」に限定されていることを requirements §3.2 と architecture §3.4 に突き合わせて確認する。`docs/` を対象に旧来の対象を限定していない言い回しが残っていないことを検索し、出力を記録する（修正前は `requirements_process.md:97` の 1 件）。
- [ ] **ステップ 5-3**: `make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: requirements process guide alignment

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `docs(0002): align the boundary-check wording with consumed fields`

**レビュー観点**: 境界チェックの記述が「消費するフィールド」に限定され、未消費メンバーの受理（AC-64・AC-67）と矛盾しないこと / チェックリスト（`:101-111`）を含め、対象を限定していない規則が残っていないこと / 修正後の記述が requirements §3.2 と architecture §3.4 に一致していること

**実装モデル要件**: standard

**判定理由**: ドキュメントの記述の整合のみで、競合する実装方針の併記・高リスクな制御・Conditional checks のいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `errors.go`・`video_id.go`・`json3.go`・`info.go`、各テスト、`test_helpers.go`（フィクスチャのパス定数）、AC-52・AC-53 用の合成サンプル、`package_reference.md` の更新 | `make test` / `make lint` が通る |
| M2 | フェーズ 2 | `exec.go`（`commandExecutor`・`cappedWriter`・allowlist 環境）と `exec_test.go`、`package_reference.md` の更新 | 同上 |
| M3 | フェーズ 3 | `cache.go`・`ytdlp.go`・`exec.go` の伏字化・`test_helpers.go`（fake とキャッシュ状態）と `cache_test.go`・`ytdlp_test.go`、`package_reference.md` の更新 | 同上 |
| M4 | フェーズ 4 | `integration_test.go`、Makefile・pre-commit・CI のタグ変更、手動実行の記録 | 同上・`make test-integration` が通る |
| M5 | フェーズ 5 | `requirements_process.md` の更新 | 同上・記述の整合を確認済み |

### 3.2. PR 構成

PR はフェーズと 1 対 1 に対応させる。各 PR は主たる関心事（純粋な処理 / 外部コマンドの境界 / キャッシュと `Fetch` / 統合テストと lint 経路 / ドキュメント）を持ち、単独でグリーンゲートを通せる単位とする。`internal/transcript` の既存契約（`TranscriptSource`・`Transcript`・`Segment`）と `cmd/` は変更しないため、internal の変更が cmd に先行する順序の問題は生じない。実 `yt-dlp`・ネットワーク・CI に触れる PR-4 を frontier-required とする。

PR-3 は、キャッシュ更新の状態機械（S1〜S6）と中断回復・`Fetch` の制御・標準エラー出力の伏字化を 1 つの PR にまとめる。ステップの粒度では分割できないためである。キャッシュの削除・掃除のテスト（ステップ 3-4）はステップ 3-3 が実装する公開メソッドを呼び、キャッシュ書き込み経路の本番の呼び出し元は `Fetch` だけであり、`Fetch` を後続の PR へ分けるとステップ 3-5 のテストを伴わない実装が先にマージされるか、書き込み経路がテストされないまま残る。`NewYtDlpSource` が設定する executor フィールドを読むのも `Fetch` だけであるため、分けると読み手のないフィールドになる。したがって高リスクな状態機械と中断回復を PR-3 の外へ出さず、レビューでは PR-3 のレビュー観点に集約して確認する。PR-4 の lint タグの 3 箇所の変更は手動実行と独立にレビュー・検証でき、手動実行（ステップ 4-5）はネットワークに依存するため、レビューでは両者を分けて評価する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13 | 番兵エラーと `ParseError`、URL 検証、厳密デコードと json3・info.json パーサ、合成サンプル、`package_reference.md` の更新 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | `exec.go`（`commandExecutor`・`cappedWriter`・allowlist 環境）と `exec_test.go`、`package_reference.md` の更新 | frontier-recommended |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | `cache.go`・`ytdlp.go`・`exec.go` の伏字化、fake とキャッシュ状態ヘルパー、`cache_test.go`・`ytdlp_test.go`、`package_reference.md` の更新 | frontier-recommended |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 | 統合テスト、Makefile・pre-commit・CI の lint タグ、手動実行の記録 | frontier-required |
| PR-5 | 5-1 / 5-2 / 5-3 | `requirements_process.md` の境界チェックの整合 | standard |

### 3.3. 実装順序の根拠

architecture §8 の順序（依存される側（下位）から、純粋な処理 → 外部コマンドの境界 → キャッシュと `Fetch` → 統合テストと lint 経路 → ドキュメント）に従う。`video_id.go`・`json3.go`・`info.go` は外部コマンドに依存しないため先に完成させ、`exec.go` のテストは実プロセスを起動する独立した境界として次に置く。`cache.go` と `ytdlp.go` は両者を統合する。統合テストと lint 経路は `Fetch` が完成した後に置く。ドキュメントは実装の確定後に更新する。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。テスト関数名と AC の対応は §5 に示す。

### 4.1. ユニットテスト

architecture §7.1 に従い、実 `yt-dlp` もネットワークも呼ばない（AC-27）。計画固有の事項は次のとおり。

- fake は `test_helpers.go` の fake `commandExecutor` とし、実装と同じくタイムアウト・キャンセルを context エラーではない失敗として模擬する（architecture §3.2）。注入は `YtDlpSource` の非公開フィールドの置き換えで行い、`Options.YtDlpPath` には存在しないパスを指定する（差し替え忘れが実 `yt-dlp` の起動やネットワーク到達にならないようにする。AC-27）。
- キャッシュヒットのテストは `testdata/` の実出力を有効なスロットに配置して実行し、`Transcript` のメタ情報とセグメントを検証する（AC-14・AC-25）。実データの日本語（タイトル・チャンネル名・本文）は期待値リテラルとして埋め込まない。
- UTF-8・サロゲートの拒否は、フェーズ 1 で追加する `testdata/` の合成サンプルを入力にする（AC-52・AC-53）。
- コミット段階のファイル操作には失敗を注入しない。差し替え可能な境界は `commandExecutor` だけであるため、architecture §6.3 の各状態を直接作って後始末と世代の混在を検証する（AC-69）。
- 実プロセスの境界は `exec_test.go` で `t.TempDir` のヘルパー実行ファイルを使う（AC-08・AC-51・AC-56）。同じ経路で、allowlist 環境（`TestCommandExecutorEnvAllowlist`。設定された変数だけが子プロセスへ渡り、未設定時も非 nil の空環境）、シェルを経由しない引数のそのままの到達（`TestCommandExecutorNoShell`）、起動失敗が握り潰されず `Fetch` が `ErrYtDlpExec` へ対応付けること（`TestCommandExecutorStartFailure`。`Fetch` を使う検証はフェーズ 3 で完成する）を検証する（AC-07・AC-09・AC-44・AC-62・AC-66）。
- パーミッションで失敗を起こすテスト（`TestRemoveCacheFailure`・`TestPruneCachePartialFailure`・`TestFetchDanglingDeleteFailure`）は、root 以外での実行を前提とする（CI とローカル開発は非 root）。ディレクトリのモードを変えた時点で `t.Cleanup` に復元を登録し、`t.TempDir` の後始末が別の理由で失敗しないようにする。root を検出した場合は前提が成立しないため、「unsupported test environment」と明示するメッセージで失敗させる（黙って成功・スキップしない）。
- 各テストは、対象の仕組みを実際に壊して失敗することを確認してからコミットする（CLAUDE.md「Testing Strategy」）。壊す対象と対応するテスト名は各フェーズのタスクに示す。
- **後方互換性:** 既存の型・interface・パイプラインを変更しないため、該当なし（§1.3）。既存テストの更新も不要である。

### 4.2. 統合テスト

architecture §7.2 と F-008 に従う。計画固有の事項として、取得・キャッシュの再利用・強制再取得は 1 つのテスト関数のサブテストとして順に実行し、最初の取得結果とキャッシュを共有する（YouTube へのリクエスト回数を抑える。architecture §1.4 のレート制限の観測を踏まえる）。サブテストは順序に依存する。環境変数の必須確認（AC-42）は取得より前に、変数名を示して失敗させる。

### 4.3. セキュリティテスト

architecture §7.3 に従う。計画固有の事項として、伏字化の検証（architecture §4.2）は fake executor を使った `Fetch` レベルのテストで行い、4 KiB の境界で切れる値も入力に含める。

### 4.4. テストヘルパー

`internal/transcript/test_helpers.go`（`//go:build test`）に、fake `commandExecutor`、キャッシュ状態 S1〜S6 を作るヘルパー、パーミッション変更と復元のヘルパーを置く。統合テストの環境変数の確認は `integration_test.go` 内で直接行い、共有ヘルパーには置かない（ユニットテストでは検証しない）。`testutil/` は追加しない（新しい公開 interface の fake は不要。`TestFakesCarryBuildTag` の固定件数を変えない）。テストの実装詳細（アサーションの書き方、入力の構成）は実装時に決定する。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

AC ごとの検証は次のとおり。`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲット・コミット済みスクリプトを指す。`manual` は補助的な確認であり、`test` または `static` を置き換えない。テストの配置は architecture §3.8 の一覧と一部異なる（キャッシュ系の AC を `ytdlp_test.go` に置く等）。配置の正は本表とし、architecture §3.8 のテストファイル別の列挙は AC の対応関係の要約として読む。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | 各 URL 形式から動画 ID を取り出す | test | `internal/transcript/video_id_test.go::TestValidateVideoURL` |
| AC-02 | 不正な動画 ID を補正しない | test | `internal/transcript/video_id_test.go::TestValidateVideoURL` |
| AC-03 | 非対応形式を `ErrInvalidVideoURL` で拒否 | test | `internal/transcript/video_id_test.go::TestValidateVideoURL` |
| AC-04 | `v` 以外のパラメータの `../` を受理し、パスは動画 ID のみから組み立てる | test | `internal/transcript/video_id_test.go::TestValidateVideoURL`・`internal/transcript/ytdlp_test.go::TestFetchCachePathUsesOnlyVideoID` |
| AC-05 | 正規化 URL の組み立て | test | `internal/transcript/video_id_test.go::TestValidateVideoURL` |
| AC-06 | 固定引数での起動と `-P` / `-o` / `--` | test | `internal/transcript/ytdlp_test.go::TestFetchYtDlpArgs` |
| AC-07 | シェルを経由せず、差し替え可能で、引数と環境をテストから観測できる | test | `internal/transcript/ytdlp_test.go::TestFetchYtDlpArgs`・`internal/transcript/exec_test.go::TestCommandExecutorNoShell`（シェルメタ文字を含む引数が実 executor を通ってそのまま届く） |
| AC-08 | タイムアウト・キャンセルの判別と有界な待機 | test | `internal/transcript/exec_test.go::TestCommandExecutorWaitDelay`・`internal/transcript/ytdlp_test.go::TestFetchTimeoutAndCancel` |
| AC-09 | 非ゼロ終了・実行ファイル不在で `ErrYtDlpExec`、標準エラー出力は 4 KiB まで | test | `internal/transcript/ytdlp_test.go::TestFetchYtDlpFailure`・`internal/transcript/exec_test.go::TestCommandExecutorStartFailure`（存在しないパス・実行不能ファイルで実 executor がエラーを返し、`Fetch` が `ErrYtDlpExec` へ対応付ける） |
| AC-10 | `segs[].utf8` の連結と `tStartMs` | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData` |
| AC-11 | `segs` なし・空 `utf8` の除外 | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData`（`segs` なし・空白のみ）・`TestParseSubtitlesEmptySegs`（空 `utf8` の最小入力） |
| AC-12 | 不正 JSON・`null`・非配列・後続データの拒否 | test | `internal/transcript/json3_test.go::TestParseSubtitlesRejectsSimple`・`TestParseSubtitlesDuplicateMembers` |
| AC-13 | `testdata/` の実出力でテスト | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData` |
| AC-14 | タイトル・チャンネル名・概要欄の取り出し | test | `internal/transcript/info_test.go::TestParseInfoRealData`・`internal/transcript/ytdlp_test.go::TestFetchCacheHit`（`testdata/` の info.json を配置したキャッシュから `Transcript` の 3 フィールドに入ること、および 3 フィールドがテスト自身がフィクスチャからパーサとは独立に導出した互いに異なる期待値と一致すること（フィールドの取り違え・1 つの値の全フィールドへの流用を検出）を検証） |
| AC-15 | 不正 JSON・後続データを `ErrParseInfo` で拒否 | test | `internal/transcript/info_test.go::TestParseInfoRejectsMalformed` |
| AC-16 | 概要欄は任意、タイトル・チャンネルの欠落・空は拒否 | test | `internal/transcript/info_test.go::TestParseInfoMissingOrEmptyFields` |
| AC-17 | キャッシュヒットでは `yt-dlp` を起動しない | test | `internal/transcript/ytdlp_test.go::TestFetchCacheHit` |
| AC-18 | キャッシュミスでは取得して保存する | test | `internal/transcript/ytdlp_test.go::TestFetchCacheMiss` |
| AC-19 | ディレクトリ `0o700`・ファイル `0o600` | test | `internal/transcript/cache_test.go::TestCachePermissions` |
| AC-20 | 動画 ID からのファイル名組み立てと掃除の名前判定 | test | `internal/transcript/cache_test.go::TestPruneCache`・`internal/transcript/ytdlp_test.go::TestFetchCachePathUsesOnlyVideoID` |
| AC-21 | 字幕ファイルなしで `ErrNoSubtitles` | test | `internal/transcript/ytdlp_test.go::TestFetchNoSubtitles` |
| AC-22 | エラーメッセージから字幕なしと対象が分かる | test | `internal/transcript/ytdlp_test.go::TestFetchNoSubtitles`（メッセージに動画 ID と字幕なしの理由が含まれることを検証） |
| AC-23 | セグメント 0 件は `ErrNoSubtitles` | test | `internal/transcript/ytdlp_test.go::TestFetchNoSubtitles` |
| AC-24 | 番兵の相互判別 | test | `internal/transcript/video_id_test.go::TestValidateVideoURL`・`internal/transcript/ytdlp_test.go::TestFetchSentinelDistinction`（URL 検証・`ErrYtDlpExec`・`ErrParseSubtitles`・`ErrParseInfo`・`ErrNoSubtitles`・`context.DeadlineExceeded`・`context.Canceled` の各カテゴリの失敗を実 `Fetch` の経路で 1 回ずつ起こし、各エラーが自分の番兵・context エラーと一致して他のいずれとも一致しないことを検証）・`TestFetchYtDlpFailure`・`TestFetchTimeoutAndCancel`・`TestFetchParseErrorPath`・`TestFetchCacheParseError`・`TestFetchNoSubtitles` |
| AC-25 | `TranscriptSource` の実装と `Transcript` の内容 | test / static | コンパイル時アサーション（`make test` が評価）・`internal/transcript/ytdlp_test.go::TestFetchCacheMiss`・`TestFetchCacheHit`（`VideoID` と正規化 `VideoURL`。`youtu.be/<id>` からのヒットでも同じ正規化値であることを検証。design_handoff H-15） |
| AC-26 | 開始前・実行中のキャンセル | test | `internal/transcript/ytdlp_test.go::TestFetchTimeoutAndCancel` |
| AC-27 | ユニットテストが実 `yt-dlp` もネットワークも呼ばない | static | `make test`（§4.1 のとおり、全ユニットテストが `test_helpers.go` の fake executor に差し替え、`Options.YtDlpPath` に存在しないパスを指定する構成であること） |
| AC-28 | タイムアウト 0 以下の構築を拒否 | test | `internal/transcript/ytdlp_test.go::TestNewYtDlpSource` |
| AC-29 | 一部だけのキャッシュはミス | test | `internal/transcript/ytdlp_test.go::TestFetchPartialCache`・`TestFetchOversizedPointer` |
| AC-30 | 残存ファイルを今回の出力と誤認しない | test | `internal/transcript/ytdlp_test.go::TestFetchStaleFileNotMistaken` |
| AC-31 | 採用する字幕ファイルの規則 | test | `internal/transcript/ytdlp_test.go::TestFetchSubtitleSelection` |
| AC-32 | 実データでイベントとセグメントが 1 対 1 | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData` |
| AC-33 | 手動実行を完了条件に含め、URL と結果を計画に記録 | test / manual | `make test-integration` の実行と §5.1 への記録 |
| AC-34 | 強制再取得の置き換えと旧ファイルの残存なし | test | `internal/transcript/ytdlp_test.go::TestFetchForceRefresh` |
| AC-35 | キャッシュ削除後の再取得 | test | `internal/transcript/ytdlp_test.go::TestFetchCacheInvalidation` |
| AC-36 | `*ParseError` のパスと番兵 | test | `internal/transcript/ytdlp_test.go::TestFetchParseErrorPath`・`TestFetchCacheParseError`（有効なキャッシュの字幕・info.json を壊し、`errors.AsType[*ParseError]` の `Path` が読み込んだキャッシュファイルのパスと一致すること、対応する番兵、キャッシュが変更されないことを検証） |
| AC-37 | 統合テストが既定のテストに含まれない | static / test | `internal/transcript/ytdlp_test.go::TestIntegrationTestBuildTag`・`make test`・`make test-ci` |
| AC-38 | `make test-integration` の実行内容 | test | `Makefile` の `test-integration` ターゲットとその実行（§5.1） |
| AC-39 | 実際の動画からの取得 | test | `internal/transcript/integration_test.go::TestIntegration`（fetch サブテスト） |
| AC-40 | キャッシュの再利用 | test | `internal/transcript/integration_test.go::TestIntegration`（cache_reuse サブテスト） |
| AC-41 | 強制再取得 | test | `internal/transcript/integration_test.go::TestIntegration`（force_refresh サブテスト） |
| AC-42 | URL 未設定でスキップせず失敗 | test | コミット済みの `internal/transcript/integration_test.go::TestIntegration` を環境変数を設定せずに直接実行し（フェーズ 4 ステップ 4-6）、スキップせず変数名を示すエラーで失敗することを確認して結果を記録 |
| AC-43 | 統合テストが lint の解析対象 | static | `internal/transcript/ytdlp_test.go::TestLintTagsIncludeIntegration`（`Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の 3 箇所）・`make lint`（`--build-tags test,integration`） |
| AC-44 | 環境 allowlist | test | `internal/transcript/ytdlp_test.go::TestFetchEnvAllowlist`・`internal/transcript/exec_test.go::TestCommandExecutorEnvAllowlist`（実 executor を通して子プロセスが受け取る環境を検証） |
| AC-45 | `tStartMs` の欠落・不正の拒否 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps` |
| AC-46 | 時間的に離れた同一本文の保持 | test | `internal/transcript/json3_test.go::TestParseSubtitlesKeepsRepeatedText` |
| AC-47 | info.json の `id` の照合 | test | `internal/transcript/info_test.go::TestParseInfoIDMismatch` |
| AC-48 | 字幕サイズの上限 | test | `internal/transcript/json3_test.go::TestParseSubtitlesLimits` |
| AC-49 | イベント数の上限 | test | `internal/transcript/json3_test.go::TestParseSubtitlesLimits` |
| AC-50 | info.json のサイズの上限 | test | `internal/transcript/info_test.go::TestParseInfoLimits` |
| AC-51 | 標準エラー出力の保持は先頭 4 KiB まで | test | `internal/transcript/exec_test.go::TestCappedWriter`・`TestCommandExecutorDrainsStderr`・`internal/transcript/ytdlp_test.go::TestFetchStderrCapAndRedaction` |
| AC-52 | 不正 UTF-8・サロゲートの拒否（json3） | test | `internal/transcript/json3_test.go::TestParseSubtitlesUTF8`（`testdata/` の合成サンプル 2 件の拒否に加え、有効な高/低サロゲートペア（非 BMP 文字）を含む入力と `\\ud800` が通常テキストとして現れる入力の受理を検証し、生バイト列の判定が正当な入力を過剰に拒否しないことを確認） |
| AC-53 | 不正 UTF-8・サロゲートの拒否（info.json） | test | `internal/transcript/info_test.go::TestParseInfoUTF8`（`testdata/` の合成サンプル 2 件の拒否に加え、有効な高/低サロゲートペア（非 BMP 文字）を含む入力と `\\ud800` が通常テキストとして現れる入力の受理を検証し、生バイト列の判定が正当な入力を過剰に拒否しないことを確認） |
| AC-54 | `tStartMs` の `int64` 範囲外の拒否 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps` |
| AC-55 | `events` の非オブジェクト要素の拒否 | test | `internal/transcript/json3_test.go::TestParseSubtitlesElementKinds` |
| AC-56 | 大量の標準エラー出力の後の非ゼロ終了でタイムアウトを待たない | test | `internal/transcript/exec_test.go::TestCommandExecutorDrainsStderr`・`internal/transcript/ytdlp_test.go::TestFetchStderrCapAndRedaction` |
| AC-57 | `segs`・`utf8` の形の拒否 | test | `internal/transcript/json3_test.go::TestParseSubtitlesElementKinds` |
| AC-58 | 強制再取得の失敗で既存キャッシュを保護 | test | `internal/transcript/ytdlp_test.go::TestFetchForceRefreshFailureKeepsCache`（非ゼロ終了・タイムアウト・キャンセル・字幕なしの正常終了・不正 json3・不正 info.json・`id` 不一致・info.json なしの 8 ケース） |
| AC-59 | 別動画・無関係なファイルの不変と一時ファイルの非残存 | test | `internal/transcript/ytdlp_test.go::TestFetchUnrelatedEntriesUntouched`（成功・失敗の後に別動画のキャッシュと無関係なファイルが不変であること）・`TestFetchForceRefreshFailureKeepsCache`（AC-58 の各失敗の後に当該動画の `<id>.notes` と書き込み先スロットが残らないこと）・`TestFetchEntryTypeMismatch`（固定名に置いた種別の異なるエントリに触れずファイルシステムエラーを返すこと） |
| AC-60 | 字幕あり info.json なしで `ErrParseInfo` | test | `internal/transcript/ytdlp_test.go::TestFetchInfoMissing` |
| AC-61 | 両方なしで `ErrNoSubtitles`（字幕優先） | test | `internal/transcript/ytdlp_test.go::TestFetchNoSubtitles` |
| AC-62 | 起動・待機失敗の番兵 | test | `internal/transcript/ytdlp_test.go::TestFetchYtDlpFailure`・`internal/transcript/exec_test.go::TestCommandExecutorStartFailure`（実 executor の起動失敗が握り潰されず `ErrYtDlpExec` へ対応付けられる） |
| AC-63 | 読み取り不能な字幕ファイルの `*ParseError` | test | `internal/transcript/ytdlp_test.go::TestFetchUnreadableSubtitleFile`・`TestFetchSymlinkSlotOutput`（fake の出力とキャッシュの両方で、通常ファイルでない字幕・info.json が対応する番兵とパスを持つエラーになり、リンク先が不変であることを検証） |
| AC-64 | info.json の未消費メンバーの受理 | test | `internal/transcript/info_test.go::TestParseInfoRealData`・`TestParseInfoDuplicateMembers` |
| AC-65 | info.json の消費フィールドの型不正の拒否 | test | `internal/transcript/info_test.go::TestParseInfoTypeMismatch` |
| AC-66 | allowlist の集合の正確さ | test | `internal/transcript/ytdlp_test.go::TestFetchEnvAllowlist`・`internal/transcript/exec_test.go::TestCommandExecutorEnvAllowlist`（設定された変数とその値だけが子プロセスへ渡り、1 つも設定されていなければ非 nil の空環境） |
| AC-67 | json3 の未消費メンバーの受理と破棄イベントの `tStartMs` を検証しないこと | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData`・`TestParseSubtitlesDuplicateMembers`・`TestParseSubtitlesTimestamps` |
| AC-68 | 正常終了後の字幕優先の検証順序 | test | `internal/transcript/ytdlp_test.go::TestFetchValidationOrder`（実行後の複合失敗と、キャッシュ読み込みの複合失敗（0 件の字幕 + 不正な info.json、不正な字幕 + 不正な info.json）で字幕側の番兵が返ることを検証。design_handoff H-14） |
| AC-69 | 中断状態の後始末と混在の検出 | test | `internal/transcript/ytdlp_test.go::TestFetchInterruptedStates`（architecture §6.3 の S1〜S6）・`TestFetchOversizedPointer`・`TestFetchDanglingDeleteFailure`（dangling の削除失敗が `Fetch` の戻り値を変えないこと） |
| AC-70 | 掃除で dangling なエントリだけを削除 | test | `internal/transcript/cache_test.go::TestPruneCache`・`internal/transcript/ytdlp_test.go::TestFetchOversizedPointer` |
| AC-71 | 掃除が規則外・不正 ID・種別違いに触れない | test | `internal/transcript/cache_test.go::TestPruneCache` |
| AC-72 | 掃除の部分失敗の集約 | test | `internal/transcript/cache_test.go::TestPruneCachePartialFailure` |
| AC-73 | キャッシュの削除の成功 | test | `internal/transcript/cache_test.go::TestRemoveCache` |
| AC-74 | キャッシュなし・不正 URL での削除 | test | `internal/transcript/cache_test.go::TestRemoveCache` |
| AC-75 | キャッシュの削除の途中失敗と後始末 | test | `internal/transcript/cache_test.go::TestRemoveCacheFailure` |

**横断検索項目**（`make test` / `make lint` では検出できないもの）: フェーズ 5 の `requirements_process.md` の修正について、旧来の対象を限定していない言い回しが `docs/` の他の箇所に残っていないことを検索で確認する。`package_reference.md` の `internal/transcript` の行が実装後の責務と一致することを突き合わせて確認する。本タスクは既存シンボルの削除・改名を含まないため、それ以外の横断検索は不要である。

### 5.1. 手動実行の記録 (AC-33)

`make test-integration` の実行結果をここに記録する（実施はフェーズ 4 ステップ 4-5）。

- 実施日: （未実施）
- 使用した動画 URL: （未実施。既定 `https://www.youtube.com/watch?v=EQCUZyB4DqE` を予定）
- 期待する動画 ID: （未実施。既定 `EQCUZyB4DqE` を予定）
- 結果: （未実施。取得・キャッシュ再利用・強制再取得の各サブテストの成否を記録する）

### 5.2. 環境変数を設定しない直接実行の記録 (AC-42)

フェーズ 4 ステップ 4-6。コミット `2aa4f74` の `integration_test.go` を、`YT2COLUMN_TEST_VIDEO_URL`・`YT2COLUMN_TEST_VIDEO_ID` を環境から外して直接実行した（2026-10-01）。スキップせず、変数名を含むメッセージで失敗した。

```
$ env -u YT2COLUMN_TEST_VIDEO_URL -u YT2COLUMN_TEST_VIDEO_ID go test -tags integration -count=1 -v ./internal/transcript
=== RUN   TestIntegration
    integration_test.go:43: YT2COLUMN_TEST_VIDEO_URL is not set: set it to the URL of a video with Japanese subtitles, or run `make test-integration`
--- FAIL: TestIntegration (0.00s)
FAIL
FAIL	github.com/isseis/yt2column/internal/transcript	0.375s
FAIL
```

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `gosec` の G204（可変のコマンド名での実行）が `exec.go` で指摘される | `make lint` が通らない | `exec.CommandContext` の呼び出しに限定した最小の `//nolint:gosec` と理由コメントを付ける（PR-2 ステップ 2-6）。テストファイルは gosec の除外対象（`.golangci.yml:103-113`）。 |
| `gosec` の G304（変数パスでのファイル読み取り）が `cache.go`・`ytdlp.go` で指摘される | `make lint` が通らない | 指摘された場合は読み取りの呼び出しに限定した最小の `//nolint:gosec` と理由コメントを付ける。同じ形の読み取りすべてに同じ対応を適用する。指摘の有無を PR-3 のゲートで確認する。 |
| `test_helpers.go` は `_test.go` ではないため、`.golangci.yml:103-113` のテスト向け除外が効かない | `make lint` が通らない | errcheck・goconst・err113・dupl を含む lint に適合させる（未チェックのエラーを残さない、固定文字列を定数化する、静的センチネルを使う、重複を避ける）。`make lint` は `--build-tags test,integration` で解析する。 |
| `mnd` が上限値・猶予の数値リテラルを指摘する | `make lint` が通らない | 8 MiB・65,536・4 KiB・5 秒・待機の余裕を名前付き定数にする。 |
| `goconst` が `a`・`b`・`current`・`current.tmp` の繰り返しを指摘する | `make lint` が通らない | 固定名とサフィックスをパッケージ定数にする。 |
| `dupl` が json3 と info.json のパーサの類似構造を指摘する | `make lint` が通らない | 共通の厳密デコードを 1 箇所に集約する。それでも指摘される場合は、理由付きの最小の `//nolint:dupl` を検討する。 |
| 実プロセスを使う `WaitDelay` のテストが環境負荷で不安定になる | テストの間欠失敗 | 固定の猶予に十分な余裕を加えた上限で判定し、厳密な時間比較をしない。 |
| パーミッションによる削除失敗の注入が root 実行時に成立しない | AC-72・AC-75・AC-69 の検証が通らない | §4.1 のとおり非 root での実行を前提とする（CI とローカル開発は非 root）。モードを変えた時点で `t.Cleanup` に復元を登録する。root を検出した場合は「unsupported test environment」と明示して失敗させ、黙って成功・スキップしない。 |
| YouTube のレート制限（HTTP 429）や動画の非公開化で統合テストが失敗する | M4 の完了が遅れる | 統合テストは `t.TempDir` を使い再実行可能にする。手動実行の記録に失敗時の URL 差し替えを残す。既定の動画が使えない場合は F-008 に従って URL と ID を差し替える。 |
| `make test-integration` がネットワークと実 `yt-dlp` を使う | 実行にユーザーの承認が必要 | PR-4 ステップ 4-5 で承認を得てから実行する（CLAUDE.md「Tool Execution Safety」）。 |
| `testdata/` のフィクスチャが大きい（約 1.8 MB） | リポジトリの増加 | 対応不要。コミット `4bf3ffd` で追加済みで、git の圧縮により増分は小さい（architecture 付録A）。 |
| `make deadcode` が未配線の公開メソッドを指摘する | 参考情報のみ | グリーンゲートは `make test && make lint` であり、`deadcode` は含まれない。CLI 配線は #6 が行う。指摘が出た場合はコミットメッセージに記録する。 |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
- [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る
- [ ] PR-4: `make test-integration` が通り、§5.1 に使用した動画 URL・動画 ID・結果が記録されている
- [ ] 各テストについて、対応する分岐・対策を壊し、テストが失敗することを確認済み（コミットメッセージに記録）
- [ ] `internal/transcript/test_helpers.go` と、これを使う全 `_test.go` に `//go:build test` がある。統合テストの環境変数の確認は `integration_test.go` 内にある
- [ ] `integration_test.go` の先頭行が `//go:build integration` である
- [ ] `Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の lint タグがすべて `test,integration` である（`TestLintTagsIncludeIntegration`）
- [ ] `cmd/yt2column/main.go` と `internal/pipeline` が変更されていない
- [ ] 既存の実出力フィクスチャ（`testdata/2tcCWM-sRBw.*`）が変更されておらず、合成サンプル 4 件だけが追加されている

## 8. 成功基準 (Success Criteria)

- 全 AC（AC-01〜AC-75）に、§5 のとおり `test` または `static` の検証がある。`manual` は補助である。
- `make test` と `make lint` が通る（グリーンゲート）。`make test-integration` が通り、その結果が §5.1 に記録されている。
- ユニットテストは実 `yt-dlp` もネットワークも呼ばない。統合テストは既定のテストに含まれない。
- `docs/dev/developer_guide/package_reference.md` と `requirements_process.md` が実装と整合している。
- `internal/transcript/transcript.go`・`internal/pipeline`・`cmd/yt2column/main.go` が変更されていない。

## 9. 次のステップ (Next Steps)

- 本計画は `approved`。PR 境界は §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済み。
- `/runplan 0002` で PR の順に実装する（各 PR は独立してグリーンゲートを通す）。
- 実装完了後、#6 が `Options` を CLI フラグと環境変数に配線し、投稿の成功後に `RemoveCache` を呼び、各実行で `PruneCache` を呼ぶ。
