# 実装計画書：fake の LLM を使う開発用のサーバ

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | 初版の承認（PR #844）。 |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指し、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 B が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（fake の構成要素・開発用のサーバ・シナリオ・ジョブ・工程・`perJobWriter`）は、要件書 §6 と設計書 §1.3 と同じ意味で使う。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

実際の LLM を呼ばずに拡張とサーバを手で確かめられるように、fake の構成要素 `internal/llm/devfake` と、それを使う開発用のサーバ `cmd/yt2column-devserver` を作る（要件書 §2.1）。何をどう作るかは設計書 §3 が定める。本計画は、作業の順序・PR の分け方・確かめ方を定める。

### 1.2. 実装の原則

-   **着手の条件を最初に確かめる。** タスク 0083 の最後の実装の PR と、#771 の F-001（`make wasm-check` を含む）の実装が `main` に入っていなければ、何も書かずに止まる（設計書 §1.1・§8、ステップ 1-1）。
-   **設計書 §8 の順に進める。** `devfake` → 開発用のサーバ → 文書。
-   **パッケージを作る PR に、そのパッケージの `package_reference.md` の行を入れる。** `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:629`、756d2f37）は、行のないパッケージがあると `make test` を失敗させる。
-   **加えたテストは、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く**（`CLAUDE.md`「Testing Strategy」）。壊す対象を取り違えやすいものは、各ステップで挙げる。
-   **Go のソースのコメント・識別子・文字列のリテラルは英語で書く**（`CLAUDE.md`「Go Idioms」）。日本語の記事の文面は、`go:embed` で埋め込むファイルに置く（設計書「設計の際に決めたこと」）。
-   **各フェーズの後に、`make fmt` → `make test` → `make lint` → `make deadcode` を通す。** `docs/dev/guides/development.md` を変えるフェーズは `make ext-test` も通す（`CLAUDE.md`「Development Notes」）。
-   **タイミングに頼るテストは、レースのない形にする**（設計書 §7）。遅れと重なりは、待つ処理（以下、待ち方）の差し替えで作り、実時間を待たない。テストの側で時間の上限を置くときは、`make test` が `-race` で走ることを考えて余裕を持たせる（[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の「結果」）。

### 1.3. 計画の際に決めたこと（2026-11-11）

1.  **PR を 3 本に分ける。** PR-1 は `devfake`（型・クライアント・文面・単体テスト）、本番のバイナリとの分離のテストの共通の補助と `devfake` の部分、`make wasm-check` への追加、`test_organization.md` の例外（AC-28）、`package_reference.md` の 2 行。PR-2 は開発用のサーバ、`make build-dev`、`internal/config` の説明と `envaccess_test.go`、分離のテストへの開発用のサーバの追加、`package_reference.md` の 1 行。PR-3 は残りの文書（`security.md`・`development.md`・`project_overview.md`・`CLAUDE.md`・ADR-0045 の文の削除）と、拡張からの手での確認（AC-05）。理由: 設計書 §8 の 3 項目に合わせると、各 PR が 1 つの関心にまとまり、単独でグリーンゲートを通る。パッケージ表の行だけは、ガードのテストのため、パッケージを作る PR に入れる。`test_organization.md` の例外（AC-28）も、例外の対象になる `devfake` と同じ PR に入れる。例外のないまま `devfake` が入ると、規則に反するコードとしてレビューされるためである。
    -   採らなかった案: 2 本にし、文書をすべて PR-2 に入れる案。PR-2 が大きくなる。
    -   人の答え: 3 本の案を選んだ。
    -   ADR: なし（タスクの中で閉じる、PR の分け方である）。
2.  **遅れの待ち方は、`Scenario` の非公開のフィールドに持たせ、テストからはビルドタグ `test` の `internal/llm/devfake/test_helpers.go` の関数で差し替える。開発用のサーバの `run` は、テストで差し替える関数の組（`deps`）に「シナリオを包む関数」を持ち、本番では何も変えない関数を渡す。** 理由: 本番のバイナリの API に、テストのためだけの入口を加えない。ビルドタグ `test` の補助の公開の関数を別のパッケージのテストから使う前例がある（`internal/config/test_helpers.go:38` の `SocketPathTailForTest`、756d2f37）。名前は `…ForTest` の形にする。設計書 §3.2 の「待ち方を呼ぶ位置は、遅れが 0 の呼び出しでも必ず通る場所にする」と、§7.1・§7.2 の「形は実装計画で決める」の具体化である。
    -   採らなかった案: `NewScenario` か公開の関数で待ち方を受け取る案。本番の API にテストの入口が加わる。
    -   人の答え: `test_helpers.go` の案を選んだ。
    -   ADR: なし（パッケージの中のテストの入口の形である）。
3.  **開発用のサーバのテストがジョブの API を呼ぶ補助は、`cmd/yt2column-devserver/test_helpers.go` に必要な分だけ書く。** 本番のサーバのテストの `requestBody`・`unixClient`・`doJobs`・`waitJobDone`（`cmd/yt2column-server/test_helpers.go:434-505`、756d2f37）は `package main` の中にあり、別のパッケージから使えない。理由: 共通のパッケージに切り出すと、0083 が書き換えた本番のサーバのテストまで変えることになる。開発用のサーバは #773 でサーバと一緒に消える。設計書が `run.go` のつなぎのコードの写しを受け入れたのも、この理由による。ソケットへの送り方は `internal/udstest` の `Transport` を使い、写さない。
    -   採らなかった案: test-only の共通のパッケージに切り出し、本番のサーバのテストもそれを使うように書き換える案。
    -   人の答え: 最小の写しの案を選んだ。
    -   ADR: なし（テストの補助の置き場所である）。
4.  **引数の値の綴りは、`devfake` の 3 つの列挙型の `String()` が持つ。`Scenario` は `slog.LogValuer` を実装し、起動の行（AC-10）はシナリオを 1 つの属性で出す。開発用のサーバの引数の解析は、各列挙型の値の `String()` と照合して綴りを受理する。** 理由: 綴りの表が 1 か所になり、起動の行と引数の解析が食い違わない。設計書 §3.1 が実装計画に任せた「表示用のメソッドの名前と文字列」の具体化である。表示用のメソッドは `LogValue` だけで、`Scenario` の値を個別に返すメソッドは作らない（設計書 §3.1）。列挙値を並べる手段（解析が照合する、受理する値の一覧）は `devfake` が公開する。
    -   受理する値の一覧は、設計書の綴りの表の値だけを持つ。`ArticleStandard` は含み（`standard`）、`FailureNone` と `FailAtNone` は含まない。この 2 つの `String()` は `none` を返し、起動の行だけが使う。引数の `-fail none`・`-fail-step none` は知らない値として拒否する（AC-09）。
    -   起動の行の属性は、`socket`（ソケットのパス）、`llm`（値は `fake`）、`model`（`ModelName`）、`scenario` の組（`article`・`fail`・`failStep`・`delay`）とする。`fail` は失敗なしで `none`、`failStep` は失敗なしでは出さない。`delay` は秒数の整数である。
    -   採らなかった案: 綴りの表を開発用のサーバに置き、起動の行には解析した綴りを出し、`Scenario` に表示用のメソッドを持たせない案。
    -   人の答え: `devfake` の `String()` の案を選んだ。
    -   ADR: なし（パッケージの中の型の形である）。
5.  **本番のバイナリとの分離のテストの共通の処理は、ビルドタグ `test` の新しいパッケージ `internal/godepstest` に置く。** 設計書 §7.3 は「共通の処理は、ビルドタグ `test` の補助にまとめる」とし、置き場所を実装計画に任せている。この補助は、本番の 3 つの `cmd/` と `devfake` の 4 つのパッケージのテストが使うので、パッケージの中の `test_helpers.go`（`docs/dev/conventions/test_organization.md` の Classification B）には置けない。公開の API だけを使う補助なので、`internal/udstest` と同じく、test-only のパッケージにする。`package_reference.md` に行を加える（PR-1）。
    -   ADR: なし（テストの補助の置き場所である）。
6.  **AC-23 は、手での実行の記録に加え、`cmd/yt2column/makefile_test.go` のテストで確かめる。** `TestMakeBuildBuildsEveryBinary`（`makefile_test.go:221`、756d2f37）は、`make -n build` と `make -n all` の展開を読む。これを広げ、`build` と `all` がビルドするパッケージの集まりが本番の 3 つとちょうど等しいことを確かめる（今は 3 つを含むことだけを確かめている）。`make -n build-dev` がビルドするパッケージの集まりが `./cmd/yt2column-devserver` の 1 つだけで、出力先が `build/dev/yt2column-devserver` であることを確かめるテストを加える。設計書 §7.4 の「`make` を実行して確かめ、記録する」は、そのまま行う。
    -   ADR: なし（確かめ方の追加である）。
7.  **AC-14 を読んで確かめるときの対応表（0063 の §3.2 の規則ごとの、要素を網羅した記事の該当の箇所）は、本計画の §7「実行の記録」の PR-1 の欄に書く。** 設計書「設計の際に決めたこと」が置き場所を実装計画に任せている。記録の場所を本計画の 1 か所にし、PR の説明が消えても残るようにする。
    -   ADR: なし（記録の場所である）。
8.  **`development.md` の新しい節が、コードの定める値（引数の名前、受理する値、タイトルの印、`make build-dev`）を挙げていることを、ガードのテストで確かめる。** `CLAUDE.md`「Testing Strategy」は、文書に書いた値の変更には対応するテストを求める。`development.md` の節が定数の値を挙げることを確かめる前例がある（`cmd/yt2column/docs_test.go::TestDeepSeekDocsContract`）。テストは `cmd/yt2column-devserver` に置き、値を `devfake` の受理する値の一覧と定数、引数の定義から求める。文の意味は確かめない。このパッケージは CI の `doc-guard` のジョブ（`CLAUDE.md`「Development Notes」）が走らせないので、文書だけを変える PR では、このテストは手元の `make test` と、コードを含む PR の CI の `test` のジョブだけで走る。開発用のサーバとこのテストは #773 で消えるので、`doc-guard` のジョブの対象を広げず、この差を受け入れる。
    -   ADR: なし（確かめ方の追加である）。

### 1.4. 既存コード調査結果

コードの行番号は 756d2f37 の時点のものである。0083 と #771 の F-001 の実装の後は行番号が変わるので、着手のときに読み直す（ステップ 1-1）。

| 領域 | 既にあるもの | 足りないもの・変えるもの |
|---|---|---|
| 本番のサーバの起動の手順 | `cmd/yt2column-server/run.go` の `stderrWriter`（`:88-97`）・`lockedWriter`（`:100-110`）・`listenHint`（`:249-258`）・`releaseLock`（`:262-266`）・`runServer` の順序（`:163-243`）。`main.go` の `SIGPIPE` の無視と `signalrun.Run` | 開発用のサーバに同じ役目のコードを書く（設計書「設計の際に決めたこと」の 1 つ目）。本番の `run.go` は変えない。0083 の後の停止の案内の文を読み直して合わせる |
| `writer` の設定 | `writerOptions`（`run.go:76-81`）と、本番と CLI の設定を比べる基準の `writertestutil.DefaultOptions`（`internal/writer/testutil/helpers.go:22`） | 開発用のサーバの設定を、`TestWriterOptionsMatchCLIDefaults`（`cmd/yt2column-server/run_test.go:794`）と同じく `DefaultOptions` と比べるテストを加える |
| `writer` の契約 | `ProgressWriter`（`internal/writer/writer.go:94-97`）、`NewProgressWriter`（`:214`）、`TokenUsageError`（`errors.go:145`）。字数の調整は本文が `maxBodyChars`（`length.go:9`、4,000）を超えるときだけ動く（`writer.go:435`） | `perJobWriter` を書く。ジョブの中で `NewProgressWriter` が失敗したら、使用量が空の `*writer.TokenUsageError` で返す（設計書 §3.4） |
| `LLMClient` | `LLMClient`（`internal/llm/llm.go:76`）、番兵 `ErrTruncated`（`:16`）・`ErrEmptyResponse`（`:18`）。単体テスト用の fake `internal/llm/testutil`（ビルドタグ `test`） | `devfake` を新しく書く。単体テスト用の fake は変えない |
| ソケットのパス | `config.LoadHost`（`internal/config/config.go:436`）と、その「`HOME` だけを読む」契約（`:430-434`） | `HostConfig`・`LoadHost` の説明に開発用のサーバを書き加える（振る舞いは変えない） |
| 環境を読む場所 | `allowedEnvRefs`（`internal/config/envaccess_test.go:65-81`）と、`TestEnvAccessConfined` の観測の検査（`:392-399`） | 両方に `cmd/yt2column-devserver/main.go` の `value:os.LookupEnv` を加える。既存の期待が変わるのはこの 2 か所だけである（設計書 §3.5） |
| `make` のターゲット | `build`（`Makefile:64-68`）、`.PHONY`（`:57`）、`clean` は `build/` を丸ごと消す（`:89`）。`.gitignore` は `build/` を無視する。`TestMakeBuildBuildsEveryBinary`（`cmd/yt2column/makefile_test.go:221`） | `build-dev` を加え、`.PHONY` に加える。テストを広げる（§1.3 の決定 6）。`make wasm-check` は #771 の F-001 が加えるので、着手のときにその形を読む |
| ジョブの API のテストの補助 | `cmd/yt2column-server/test_helpers.go` の `requestBody`・`unixClient`・`doJobs`・`waitJobDone`（`:434-505`、`package main`）。`internal/udstest` の `Dir`・`Transport` | 開発用のサーバの `test_helpers.go` に最小の補助を書く（§1.3 の決定 3） |
| 外部のコマンドを起動するテスト | `cmd/yt2column/deadcode_test.go`、`cmd/yt2column/makefile_test.go`（`make` を起動する）。`cmd/yt2column-server/relay_e2e_test.go:192-207` は `go build` を起動し、`os.Environ()` に `GOOS`・`GOARCH`・`GOFLAGS=` を加える。`internal/maketestutil/make.go:131` は、`test` のタグのファイルで外部のコマンドを起動し、行を限った `//nolint:gosec` を付ける（`make lint` は `test` のタグでも lint し、gosec の `_test.go` の除外はこのファイルに及ばない）。`go list` を起動するテストはまだない | `internal/godepstest` を新しく書く（§1.3 の決定 5）。環境は `os.Environ()` に `COVERAGE_COMPARE_GOENV` の 5 つを上書きして与える。この 5 つは `Makefile:146` と `makefile_test.go:263` に続く 3 つ目の写しになるが、テストの補助の中に閉じるので受け入れる |
| test-only のパッケージの登録 | `TestFakesCarryBuildTag` は、`testOnlyPackageDirs`（`internal/pipeline/pipeline_test.go:540-546`）に挙げたディレクトリのファイルだけ、ビルドタグ `test` を確かめる。`internal/udstest` などが挙がっている | `internal/godepstest` を加える |
| 文書のガード | `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:629`）、`TestADRRecords`、`cmd/yt2column-server/docs_test.go`（`development.md`・`security.md`・`project_overview.md` を読む）、拡張の `serverReasons.test.ts`（`development.md` の拒否の理由の表を読む） | パッケージの行はパッケージと同じ PR に入れる。`development.md` を変える PR は `make ext-test` を走らせる |

`devfake` はどの型にも構成の分岐を持たせない（`Scenario` と `Job` はフィールドを非公開にし、`NewScenario` と `Scenario.NewJob` でしか作れない。設計書 §3.1）。`devfake` は新しいリーフパッケージなので、パッケージの中で `Scenario` を作るのは `NewScenario` とテストの補助だけになる。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: fake の構成要素（設計書 §8 の 1）

**変更するファイル:** `internal/llm/devfake/`（新規。Go のファイル、文面のファイル 4 つ、テスト、`test_helpers.go`）、`internal/godepstest/`（新規）、`cmd/yt2column/`・`cmd/yt2column-server/`・`cmd/yt2column-nmhost/` の分離のテスト（各 1 つ、新規）、`internal/pipeline/pipeline_test.go`（`testOnlyPackageDirs`）、`Makefile`（`wasm-check`）、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/test_organization.md`

-   [x] **ステップ 1-1**: 着手の条件を確かめる（設計書 §1.1）。`origin/main` で、次の 2 つが成り立つことを確かめる。どちらかが成り立たなければ、何も書かずに止まって人に報告する。
    -   0083 の最後の実装の PR が入っている（`docs/tasks/0083_remove_slack_webhook/03_implementation_plan.md` の最後の PR の「PR がマージされた」に印がある。`internal/server` に `WebhookPublisher` がない）。
    -   #771 の F-001 の `make wasm-check` がある（`Makefile` にターゲットがあり、`make lint` から呼ばれる）。#772 が TS への一本化を決めて `make wasm-check` を置かなかった場合は、設計書 §1.1 のとおり #772 の見直しの結果に従うので、止まって人に報告する。

    あわせて、§1.4 の行番号を、着手のときのコードで読み直す。設計書の前提と食い違う箇所があれば、決定の変更として人に確かめる（`requirements_process.md`「Editing an approved document」）。
-   [x] **ステップ 1-2**: `devfake` の型と `NewScenario` を書く（設計書 §3.1・§4.1）。
    -   列挙型 `Article`・`Failure`・`FailAt`、定数 `MaxDelay`・`ModelName`・`TitleMarker`、`Scenario`・`NewScenario`、番兵 `ErrInvalidScenario`・`ErrSimulatedAPIFailure`・`ErrUnexpectedCall`。
    -   3 つの列挙型に `String()`（引数の綴り。設計書「設計の際に決めたこと」の綴り）と、受理する値の一覧を持たせる（§1.3 の決定 4）。`Scenario` に `LogValue` を持たせる。範囲の外の列挙値の `String()` は、受理する綴りのどれとも一致しない値を返す。
    -   テスト `internal/llm/devfake/scenario_test.go::TestNewScenario`: 受理と拒否の表（列挙値の外、失敗の種類か失敗させる呼び出しの片方だけの指定、長い記事でない記事での `FailAtShorten`、負の遅れ、`MaxDelay` を超える遅れ、`MaxDelay` ちょうどと 0 の受理）。拒否は `errors.Is(err, ErrInvalidScenario)` で確かめる。`Scenario` は待ち方の関数を持つので（ステップ 1-3）、テストは `Scenario` を `==` や `reflect.DeepEqual` で比べず、`LogValue` で比べる。
    -   テスト `scenario_test.go::TestEnumStrings`: 3 つの列挙型の受理する値の一覧と `String()` が、設計書の綴りの表とちょうど等しく、重複がないこと。一覧が `FailureNone` と `FailAtNone` を含まないこと。範囲の外の値の `String()` が受理する綴りのどれとも一致しないこと。`TestScenarioLogValue`: `LogValue` の属性の名前と値（§1.3 の決定 4）。
    -   `NewScenario` の検査を 1 つずつ外し、そのたびに該当する行が失敗することを確かめる。
-   [x] **ステップ 1-3**: `Job` と 2 つのクライアントを書く（設計書 §3.2・§4.2、§1.3 の決定 2）。
    -   `Scenario.NewJob`・`Job.Generation`・`Job.Review`。生成のクライアントは呼ばれた回数を排他制御のもとで数える。応答と失敗は設計書 §3.2 の表のとおりにする。`switch` の `default` は `ErrUnexpectedCall` を含めたエラーを返す。
    -   遅れ: 呼び出しのたびに、応答の前に待ち方を呼ぶ。待ち方は `Scenario` の非公開のフィールドで、空なら本物の待ち方（遅れが 0 ならタイマーを使わない。遅れがあれば、タイマーと `ctx.Done()` のうち先に来るほうで戻る）を使う。待った後に `ctx.Err()` を確かめる。
    -   `internal/llm/devfake/test_helpers.go`（`//go:build test`）に、`NewScenario` が受理した `Scenario` を受け取り、待ち方だけを差し替えた写しを返す `…ForTest` の関数を置く。生の値から `Scenario` を作る補助は置かない。
    -   2 つのクライアントの型が `llm.LLMClient` を満たすことを、コンパイル時の代入（`var _ llm.LLMClient = …`）で確かめる（AC-01）。
    -   テスト `client_test.go::TestJobResponses`: シナリオごとに、次のことを確かめる。
        -   応答: `Text` の 1 行目が `# ` と `TitleMarker` で始まる。`Model` が `ModelName`、`ModelVersion` が空、`Usage` がゼロ値である。
        -   失敗のエラー: `errors.Is` で `llm.ErrEmptyResponse`・`llm.ErrTruncated`・`ErrSimulatedAPIFailure`・`context.DeadlineExceeded` を確かめる。
        -   失敗の応答の形: 空の応答と打ち切りでは `Model` を持ち `Text` が空、API のエラーとタイムアウトではゼロ値である。
        -   API のエラーは、`llm` の番兵も `context` のエラーも含まない。API のエラーのシナリオでは、どの呼び出しでも `ErrSimulatedAPIFailure` である（設計書 §4.1）。
        -   生成のクライアントの 3 回目と、検証・推敲のクライアントの 2 回目は `ErrUnexpectedCall` である。
        -   パッケージの中のテストで、範囲の外の失敗の種類を持つ `Scenario` の値を作り、`switch` の `default` が `ErrUnexpectedCall` を返す。
    -   テスト `client_test.go::TestJobsIndependent`（AC-04）: 次の 3 つの場合に、どのジョブも 1 回目に生成の応答を返すこと。(a) 2 つのジョブを交互に呼ぶ。(b) 待ち方の差し替えで、2 つのジョブを 1 回目の呼び出しの中で止めてから進める。(c) 前のジョブを呼んだ後に新しいジョブを作る。
    -   テスト `client_test.go::TestIgnoresRequest`（AC-04）: `SystemPrompt`・`UserPrompt`・`MaxOutputTokens` を変えても応答と失敗が同じこと。
    -   テスト `client_test.go::TestDelayAndCancel`（AC-19・AC-20）:
        -   差し替えた待ち方が、遅れが 0 のシナリオでも 0 でないシナリオでも、呼び出しごとに 1 回、指定の長さで呼ばれること。
        -   本物の待ち方で、遅れを `MaxDelay` にし、短い期限（例えば 100 ミリ秒）の `ctx` で呼ぶと、`context.DeadlineExceeded` が返ること。呼び出しは別の goroutine で行い、テストの側の上限（`-race` でも十分な、例えば 30 秒）を超えたら失敗にする。
        -   遅れが 0 で取り消し済みの `ctx` でも `ctx.Err()` が返ること。
    -   次のそれぞれの変更を加え、括弧に挙げたテストが失敗することを確かめる。生成のクライアントの数え方をジョブの間で共有する（`TestJobsIndependent`）。待ち方の呼び出しを遅れが 0 のときに飛ばす（`TestDelayAndCancel` の回数）。タイマーと `ctx.Done()` の待ちを `time.Sleep` に変える（`TestDelayAndCancel` の期限の行）。待った後の `ctx.Err()` の確かめを外す（`TestDelayAndCancel` の取り消し済みの行）。
-   [x] **ステップ 1-4**: 4 つの決まった記事の文面を書いて埋め込み、`writer` を通るテストを書く（設計書 §3.3、AC-12〜AC-15）。
    -   ファイルは `internal/llm/devfake/articles/` に置く（標準・長い記事の生成・長い記事の字数の調整の後・要素を網羅、の 4 つ）。ファイル名は実装の際に決める。文面は作文で、実在の動画・人物・団体の内容を含まない（`docs/dev/security.md` §7）。
    -   要素を網羅した記事は、0063 の §3.2 の規則のうち、`writer` の検査（`checkBodyMarkdown`、`internal/writer/markdown.go:108`）を通る入力で現れうるものを、それぞれ 1 つ以上含む。規則ごとの該当の箇所の対応表を §7「実行の記録」の PR-1 の欄に書く（§1.3 の決定 7）。
    -   テスト `articles_test.go::TestArticlesPassWriter`（外部のテストのパッケージ `devfake_test`）: 3 つの記事の種類ごとに、本物の `writer.NewProgressWriter` を `writertestutil.DefaultOptions` の設定で作り、ジョブのクライアントで記事を完成させる。本文の字数（4,000 以下）は数え直さず、記事が完成することで確かめる。`writer` は字数の調整の後も最後の検査でも、4,000 字を超える本文を拒否するためである（`internal/writer/shorten.go:37`、`writer.go:481`、756d2f37）。あわせて次の 5 点を確かめる。
        -   タイトルが `TitleMarker` で始まること。
        -   標準の記事の見出しが 2 つ以上あること。
        -   長い記事で字数の調整が動くこと（`Article.Shorten` が nil でない）。
        -   完成した長い記事の見出しが、標準の記事より多いこと。
        -   標準の記事と要素を網羅した記事では、字数の調整が動かないこと（`Article.Shorten` が nil）。
    -   長い記事の生成の文面の本文を 4,000 字以下に縮めると、このテストが失敗することを確かめる。
-   [x] **ステップ 1-5**: 分離のテストの共通の補助と、`devfake` の部分を書く（設計書 §7.3、§1.3 の決定 5、AC-02・AC-21・AC-22）。
    -   `internal/godepstest`（`//go:build test`）: 1 つのパッケージの推移的な依存を、ビルドタグを付けずに `GOOS=darwin` と `GOOS=linux` のそれぞれで `go list -deps` で求める。
        -   子の `go` には、`os.Environ()` に `COVERAGE_COMPARE_GOENV`（`Makefile:146`）の 5 つを上書きした環境を与え、作業ディレクトリをモジュールのルート（`go list -m -f {{.Dir}}` で求める）にする。
        -   `go` が見つからない、終了コードが 0 でない、出力が空か読めない場合は、`t.Skip` にせず失敗にする。`-e` は付けない。
        -   必ずあるはずのパッケージが結果にあることを確かめる手段と、禁止するパスがないことを確かめる手段を持つ。
        -   中心の処理はエラーを返し、モジュールのルートを引数で受け取る。`testing.TB` で失敗にする薄い包みが、ルートを求めて中心の処理を呼ぶ。自身のテストが、ルートを求める段と依存を求める段を分けて、失敗の経路を観測できるようにするためである。
        -   `exec.Command` の行に、行を限った `//nolint:gosec` と理由の注釈を付ける（`internal/maketestutil/make.go:131` と同じ形）。
    -   `internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に `internal/godepstest` を加える。`godepstest` のファイルの 1 行目からビルドタグを外すと `TestFakesCarryBuildTag` が失敗することを確かめる。
    -   `internal/godepstest` の自身のテスト（エラーを返す中心の処理を呼ぶ）: 存在しないパッケージ、`go` のない `PATH`、`PATH` の先頭に置いた偽の `go`（何も出さない場合と、読めない出力を出す場合。`internal/maketestutil` の偽のコマンドと同じやり方）、依存にない「必ずあるはずのパッケージ」、依存にある禁止するパス（例えば `fmt`）の、それぞれでエラーになること。
    -   `internal/llm/devfake/deps_test.go::TestDependencies`（AC-02・AC-01）: `devfake` の依存に `internal/llm` があり、`internal/server`・`internal/articlecache`・`internal/serversock`・`internal/privatefile`・`internal/nmhost`・`cmd/` の下のパッケージ・`net`・`os/exec` がないこと。
    -   本番の 3 つの `cmd/` のそれぞれのテスト（`cmd/yt2column/devdeps_test.go`・`cmd/yt2column-server/devdeps_test.go`・`cmd/yt2column-nmhost/devdeps_test.go` の `TestNoDevDependency`、AC-21・AC-22）: 自分のパッケージの依存に、必ずあるはずのパッケージ（CLI は `internal/writer`、サーバは `internal/server`、ホストは `internal/nmhost`）があり、`devfake` がないこと。`devfake` のパスは `devfake` の型から求める（設計書 §7.3）。
    -   次のそれぞれの変更を加え、括弧に挙げたテストが失敗することを確かめる（設計書 §7.3）。本番の 3 つの `cmd/` のそれぞれに `devfake` の import を足す（その `cmd/` の `TestNoDevDependency`）。`internal/server` に `devfake` の import を足す（`cmd/yt2column-server/devdeps_test.go::TestNoDevDependency`）。`devfake` に `internal/server` の import を足す（`TestDependencies`）。
-   [x] **ステップ 1-6**: `make wasm-check` のビルドの対象に `./internal/llm/devfake` を加える（AC-03、設計書 §3.5）。ステップ 1-1 で読んだ #771 の F-001 のターゲットの形に合わせる。`devfake` の外部のテストのパッケージは `internal/writer`（`GOOS=js` でビルドできない `internal/transcript` に依存する）を import するので、ターゲットがテストのファイルをコンパイルする形（`go vet` や `go test -c` を `devfake` にかける形）なら、加えずに止まって人に確かめる。`devfake` に `GOOS=js` でビルドできないファイルを一時的に足すと `make wasm-check` が失敗することを確かめる。
-   [x] **ステップ 1-7**: 文書の行を加える。
    -   `docs/dev/conventions/package_reference.md` に `internal/llm/devfake` と `internal/godepstest` の行を加える（`TestPackageReferenceListsPackages`）。
    -   `docs/dev/conventions/test_organization.md` の「Pipeline fakes」に、`internal/llm/devfake` が例外であることを、範囲（開発用のバイナリに組み込む fake の構成要素だけ）と理由（ビルドタグ `test` のファイルにはバイナリから使えない）とともに書く（AC-28）。英語で書く。`devfake` の Go のファイルを一時的に `testutil/` の下に動かすと `TestFakesCarryBuildTag` が失敗することを確かめ、コミットメッセージに書く。

**完了の条件:** 上のテストを加え、すべて通る。`make wasm-check` が `devfake` をビルドする。§7 に AC-14 の対応表がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: devfake component and dependency guard

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0086): サーバに依存しない fake の LLM の構成要素と、本番のバイナリから締め出す検査を加える`

**レビュー観点**: 失敗の種類ごとのエラーが状態の取得の理由のコードに対応するか / 待ち方の差し替えがレースのない重なりを作り、遅れが 0 でも取り消しを確かめるか / 分離のテスト（ステップ 1-5、ほかのステップと独立に読める）が失敗すべきときに飛ばずに失敗するか / 要素を網羅した記事が 0063 の規則を網羅するか（§7 の対応表）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3 がジョブをまたぐ並行の呼び出しの排他制御と取り消しを扱う高リスクの手順で、ステップ 1-5 と 1-3 がビルドタグ `test` だけでコンパイルする補助（`internal/godepstest`、`devfake/test_helpers.go`）を加え、mkplan.md の Conditional checks のビルドタグの項と gosec の抑止の項に当たる。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/865
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 開発用のサーバ（設計書 §8 の 2）

**変更するファイル:** `cmd/yt2column-devserver/`（新規。`main.go`・`run.go` などの Go のファイル、テスト、`test_helpers.go`）、`cmd/yt2column/devdeps_test.go`・`cmd/yt2column-server/devdeps_test.go`・`cmd/yt2column-nmhost/devdeps_test.go`、`internal/godepstest/`、`Makefile`、`cmd/yt2column/makefile_test.go`、`internal/config/config.go`・`envaccess_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 2-1**: 引数の解析を書く（設計書 §3.4・§4.3、§1.3 の決定 4、AC-08・AC-09）。
    -   `flag.FlagSet` と引数ごとの `flag.Value` の型で、`-article`・`-fail`・`-fail-step`・`-delay` を受理する。綴りは `devfake` の列挙型の `String()` と照合する。2 回目の指定、値の欠け、解析の後に残る語を拒否する。`-delay` は 10 進の数字だけを受理し、先頭が `0` で 2 桁以上の値と、空の値を拒否する。秒数を `time.Duration` に変える前に、`MaxDelay` の秒数を超える値（整数に収まらない値を含む）を拒否する。秒数を掛ける変換が桁あふれして範囲内の値になり、`NewScenario` の範囲の検査をすり抜けるのを防ぐためである。引数の組み合わせの検査と、`time.Duration` にした後の範囲の検査は `devfake.NewScenario` に任せる。`-h`・`-help` は `flag.ErrHelp` で受け、案内を出して終了コード `0` にする。
    -   拒否の理由の行は `redact.Line` を通し、続けて `-h` と同じ案内を出す。`flag` が自分で出す案内は出さない。
-   [ ] **ステップ 2-2**: 起動の手順と `perJobWriter` を書く（設計書 §3.4・§6.1、設計書「設計の際に決めたこと」の 1 つ目、§1.3 の決定 2・4）。
    -   `main.go`: `SIGPIPE` を無視し、`signalrun.Run` に `run` を渡す。`os.Args[1:]`・`os.LookupEnv`・`os.Stderr`・`productionDeps()` を渡すのはここだけにする。`productionDeps()`（本番の `cmd/yt2column-server/run.go:54` と同じ形）は、シナリオを変えずに返す関数と、本物の `writer.NewProgressWriter` を返す。
    -   `run`: 引数 → `NewScenario` → `deps` のシナリオを包む関数 → `config.LoadHost` → 起動のときの `writer` の確かめ → `server.New`（記事の作成は `perJobWriter`、ログの出力先は直列化した標準エラー出力、秘密は空）→ `serversock.Acquire`・`Listen` → 起動の行 → `Serve` → ロックの解放、の順。終了コードは設計書 §3.4 のとおりにする。停止の案内は `context.AfterFunc` で書き、文は 0083 の後の本番の案内に合わせる。
    -   起動の行の属性は §1.3 の決定 4 のとおりにする。
    -   `perJobWriter`: `Write`・`WriteWithProgress` のたびに `NewJob` で新しいクライアントの組を作り、`writer.NewProgressWriter` で本番と同じ設定の `writer` を作って呼ぶ。作るのに失敗したら、使用量が空の `*writer.TokenUsageError` で返す。
-   [ ] **ステップ 2-3**: 開発用のサーバのテストを書く（設計書 §7.2、§1.3 の決定 3）。テストは `run` に一時ディレクトリの `HOME`（`udstest.Dir`）と環境を引く関数を渡して起動し、ソケットにジョブの API のリクエストを送る。
    -   `run_test.go::TestRunArgs`（AC-08・AC-09）: 受理する指定と拒否する指定の表。拒否では終了コード `2`、標準エラー出力に受理する値の案内があること、`HOME` の下に何も作らないこと。`-h`・`-help` では終了コード `0`、案内があること、何も作らないこと。要件書 AC-09 の各項（知らない引数・知らない値・余分な語・繰り返し・値の欠け・片方だけの失敗の指定・遅れの各形・長い記事でない記事での字数の調整の失敗）について、表に 1 行以上を置く。加えて、`-fail none`・`-fail-step none`（§1.3 の決定 4）、`-delay 600` の受理と `-delay 601` の拒否、`time.Duration` への掛け算で桁があふれる値（例えば `18446744074`）と整数に収まらない値の拒否の行を置く。
    -   `run_test.go::TestStartupFailures`（設計書 §3.4・§7.2「写したコード」）: `HOME` がない・相対のときの終了コード `2`、起動のときの `writer` の作成の失敗（`deps` で差し替える）の `2`、ソケットのディレクトリが受理されない形のときの `1` と案内、拒否の理由の行が非表示の文字をエスケープすること。前例は `cmd/yt2column-server/run_test.go:47` の `TestStartupFailures`。
    -   `run_test.go::TestScenarioCombinations`（AC-24・AC-12〜AC-19）: 組み合わせは、`devfake` の受理する値の一覧と失敗なしを掛け合わせ、`NewScenario` が受理するものだけを残して作り、その数が 31 であることを先に確かめる。サブテストごとに `udstest.Dir` の別の `HOME` で起動し、遅れを 1 秒にし、待ち方を差し替えて実時間を待たずに、生成の開始と状態の取得を行う。差し替えた待ち方の記録は、サーバの goroutine とテストの間で排他制御する。完成するシナリオでは、タイトルの印と `cost.state` が `unknown` であること、失敗するシナリオでは、失敗の工程と理由のコード（要件書 AC-17）が期待どおりであること、前の工程が完了していること、記事がないこと。待ち方が呼ばれた回数がシナリオの呼び出しの回数（要件書 AC-19）と一致すること。
    -   `run_test.go::TestJobsIndependent`（AC-04）: 別の動画の別の入力を `plain` の送り方で 2 つ送り、待ち方の差し替えで両方を呼び出しの中で止めてから進める。2 つが終わった後に別の入力で 3 つ目を送る。3 つが同じ結果になること。失敗のシナリオ（例えば字数の調整での失敗）でも行う。
    -   `run_test.go::TestJobWriterFailure`（設計書 §3.4）: ジョブの中で `writer` の作成が失敗すると（`deps` で 2 回目の作成を失敗させる）、`perJobWriter` が使用量の空の `*writer.TokenUsageError` を返し、そのジョブが記事を持たずに失敗すること。
    -   `run_test.go::TestIgnoresConfigAndEnv`（AC-06・AC-07）: 設定ファイルがない場合・モードが `0600` でない場合・書式が誤っている場合に、起動の行（ケースごとに違う `socket` の属性を除く）と記事が同じになること。環境を引く関数が引いた名前が `HOME` だけであること。
    -   `run_test.go::TestListeningLine`（AC-10・AC-05）: 起動の行の属性。`socket` が、`HOME` と `config.SocketPathTailForTest(runtime.GOOS)` から求めたパスと等しいこと。
    -   `run_test.go::TestLockHeld`（AC-11）: 同じソケットのパスのロックを `serversock.Acquire` で持ち、取ったその場で `t.Cleanup` に `Release` を登録する。その状態で起動すると、終了コード `1`、別のサーバが動いていることの案内、起動の行がないこと。
    -   `run_test.go::TestStopNotice`（AC-20 の後半を含む）: 待ち方の差し替えで `ctx.Done()` まで止まるジョブを 1 つ動かした状態で停止すると、終了コード `0` で終わり、ロックを解放し、ジョブの取り消しがログに残ること。案内は `Serve` の後に書かれることがあるので（設計書「設計の際に決めたこと」の 1 つ目）、上限のある待ち（本番のテストの `waitStderr` と同じ形）で読む。
    -   `run_test.go::TestProductionDeps`: `productionDeps()` のシナリオの関数が、受け取ったシナリオを `LogValue` で同じ値のまま返すこと、遅れが 0 のシナリオで記事が完成すること。
    -   `run_test.go::TestWriterOptionsMatchCLIDefaults`: `perJobWriter` が作る `writer` の設定が `writertestutil.DefaultOptions` と等しいこと（`writer` を作る関数を `deps` で差し替えて観測する）。
    -   `test_helpers.go`（`//go:build test`）: ジョブの API の最小の補助と、`deps` の差し替え。本番のサーバのテストの補助の写しや、本番の `run.go` の写しで `dupl` が報告されたら、構成を変えずに、行を限った `//nolint:dupl` と §1.3 の決定 3（または設計書「設計の際に決めたこと」の 1 つ目）を示す注釈で抑える。
    -   次のそれぞれの変更を加え、括弧に挙げたテストが失敗することを確かめる。`perJobWriter` がクライアントを共有する（`TestJobsIndependent`）。起動の行を `Acquire` の前に出す（`TestLockHeld`）。環境から `HOME` 以外を引く（`TestIgnoresConfigAndEnv`）。`productionDeps()` のシナリオの関数が待ち方を差し替える（`TestProductionDeps`）。遅れの桁あふれの確かめを外す（`TestRunArgs`）。
-   [ ] **ステップ 2-4**: 分離のテストに開発用のサーバを加える（設計書 §7.3）。`internal/godepstest` に、`cmd/yt2column-devserver` のパスを `go list` で求め、見つかることを先に確かめる手段を加える。本番の 3 つの `cmd/` の `TestNoDevDependency` に、`cmd/yt2column-devserver` がないことの確かめを加える。`cmd/yt2column-devserver` のパスの求め方を壊す（存在しないパスを渡す）と、3 つの `TestNoDevDependency` が飛ばずに失敗することを確かめる。
-   [ ] **ステップ 2-5**: `make build-dev` を加える（設計書 §3.5、§1.3 の決定 6、AC-23）。
    -   `build-dev` は `./cmd/yt2column-devserver` を `build/dev/yt2column-devserver` にビルドする。`build` と同じビルドの引数にする。`.PHONY` に加える。`make build` は変えない。
    -   `cmd/yt2column/makefile_test.go`: `TestMakeBuildBuildsEveryBinary` の確かめを、`build` と `all` がビルドするパッケージの集まりが本番の 3 つとちょうど等しいことに改める。`TestMakeBuildDev` を加え、`make -n build-dev` がビルドするパッケージの集まりが `./cmd/yt2column-devserver` の 1 つだけで、出力先が `build/dev/yt2column-devserver` であることを確かめる（§1.3 の決定 6）。`build` のレシピに `./cmd/yt2column-devserver` を足すと前者が失敗することを確かめる。
    -   `make build` と `make build-dev` を実行し、`build/` の直下にできたファイルと `build/dev/` の中身を §7「実行の記録」の PR-2 の欄に書く。
-   [ ] **ステップ 2-6**: `internal/config` の説明と環境の検査を変える（設計書 §3.5、H-04）。
    -   `internal/config/config.go` の `HostConfig` と `LoadHost` の説明に、開発用のサーバも使うことと、「`HOME` だけを読む」契約が 2 つの入口を縛ることを書き加える。振る舞いは変えない。
    -   `internal/config/envaccess_test.go` の `allowedEnvRefs` に `cmd/yt2column-devserver/main.go` の `value:os.LookupEnv` を 1 回加え、`TestEnvAccessConfined` の観測の検査に同じ行を加える。`main.go` から `os.LookupEnv` を外すと観測の検査が失敗することを確かめる。
-   [ ] **ステップ 2-7**: `docs/dev/conventions/package_reference.md` に `cmd/yt2column-devserver` の行を加える（`TestPackageReferenceListsPackages`）。

**完了の条件:** 上のテストを加え、すべて通る。§7 に AC-23 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: development server binary

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0086): fake の LLM で動く開発用のサーバ yt2column-devserver と make build-dev を加える`

**レビュー観点**: 本番の run.go から写した部分（終了コード・ロックの解放・案内）が本番と同じ振る舞いか / 引数の拒否が何も作らずに終了コード 2 で終わり、値を直さないか / 同時と続けたジョブが同じ結果になることを、レースなしに確かめているか / 環境から HOME だけを読むか

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2・2-3 がソケット・ロック・シグナルの起動の手順と並行のジョブを扱い、mkplan.md の Conditional checks のうち、繰り返し起動する CLI の起動の検査と、外部の状態（ソケット・ロック）の後始末の 2 つに当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 文書と手での確認（設計書 §8 の 3）

**変更するファイル:** `docs/dev/security.md`、`docs/dev/guides/development.md`、`cmd/yt2column-devserver/docs_test.go`（新規）、`internal/doctestutil/`（見出しの定数）、`docs/dev/project_overview.md`、`CLAUDE.md`、`docs/dev/adr/0045-dev-fake-llm.md`、本計画（§7 の記録）

-   [ ] **ステップ 3-1**: `docs/dev/security.md` §8.2 の「Native Messaging のホスト」の項の前に、「開発用のサーバ（`cmd/yt2column-devserver`・`internal/llm/devfake`）」の項を加え、設計書 §5.3 の 5 つの規則を書く。§8.2 の「残余のリスク（許容する）」の一覧に 1 項を加える（AC-25）。
-   [ ] **ステップ 3-2**: `docs/dev/guides/development.md` の「make のターゲット」の一覧に `make build-dev` を加え、新しい節「fake の LLM でサーバを動かす」を加える（AC-26）。節には、作り方、起動のしかた（引数と受理する値、タイトルの印、起動の行の例）、本番のサーバと同時には動かせないこと、拡張から確かめる手順、やり直しの成功は確かめられないこと、生成の枠（2 つ）が遅れの間も埋まることを書く。引数と値の綴りは `devfake` の `String()` と同じにする。`make ext-test` を走らせる。
    -   `cmd/yt2column-devserver/docs_test.go::TestDevelopmentDocsDevServer`（§1.3 の決定 8）: 新しい節が、引数の名前、`devfake` の受理する値の一覧の各値、`devfake.TitleMarker`、`make build-dev` を挙げていること。節の見出しの文字列は、`CLAUDE.md`「Go Idioms」の guard-test の literal の例外にもとづき、`internal/doctestutil` に置く。値の 1 つを節から消すとテストが失敗することを確かめる。
-   [ ] **ステップ 3-3**: `docs/dev/project_overview.md` の想定ディレクトリ構成に 2 つのパッケージを加える。`CLAUDE.md` の「Build Commands」に `make build-dev` を加え、「Architecture Overview」の `LLMClient` の項に、`internal/llm/devfake` が本番のアダプタではない開発用の実装であることを書き加える（AC-27）。
-   [ ] **ステップ 3-4**: `docs/dev/adr/0045-dev-fake-llm.md` の「この決定は、まだコードと security.md に反映していない。…」の文を消す（AC-29）。編集上の訂正として、コミットメッセージに訂正の行を書く。
-   [ ] **ステップ 3-5**: 拡張からの手での確認を行い、§7「実行の記録」の PR-3 の欄に書く（AC-05）。`development.md` の手順で開発用のサーバを起動し、ホストと拡張を変えずに、拡張から生成の開始と状態の取得ができること、標準・長い・要素を網羅した記事がビューアで表示されること、失敗のシナリオで失敗の表示になること、タイトルの印と、料金が不明であることの表示を確かめる。手順どおりに動かない箇所があれば、`development.md` を直す。

**完了の条件:** 文書と `TestDevelopmentDocsDevServer` が加わり、`make test`・`make lint`・`make ext-test` が通る。§7 に AC-05 の記録がある。

### PR-3 作成ポイント: documentation and manual check

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `docs(0086): 開発用のサーバの規則と使い方を文書に書き、拡張からの手での確認を記録する`

**レビュー観点**: security.md §8.2 の 5 つの規則と残余のリスク / development.md の引数と値の綴りがコードと一致するか / ADR-0045 の文の削除が編集上の訂正の範囲か / AC-05 の記録

**実装モデル要件**: standard

**判定理由**: 文書の追加と、値の有無を確かめる 1 つのガードのテストと、手での確認で、mkplan.md のパネルの契機にも Conditional checks にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（フェーズ 1） | `internal/llm/devfake`、`internal/godepstest`、分離のテスト、`make wasm-check` への追加 | fake の構成要素があり、本番のバイナリが依存しないことを `make test` が確かめる。まだどこからも使われない |
| M2（フェーズ 2） | `cmd/yt2column-devserver`、`make build-dev` | 開発用のサーバを作って起動できる |
| M3（フェーズ 3） | 文書、手での確認の記録 | 文書と ADR-0045 がコードと一致する |

順序は設計書 §8 のとおりである。着手の条件はステップ 1-1 が確かめる。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | `devfake`、分離のテストの補助と `devfake` の部分、`make wasm-check`、AC-28 の例外 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 開発用のサーバ、`make build-dev`、`internal/config` の説明と環境の検査 | frontier-recommended |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 文書、ADR-0045 の文の削除、拡張からの手での確認 | standard |

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** 設計書 §7.1 のとおり。`devfake` の受理と拒否、応答と失敗、ジョブの独立、プロンプトを読まないこと、遅れと取り消しを、待ち方の差し替えで実時間を待たずに確かめる（ステップ 1-2・1-3）。文面は本物の `writer` を通して確かめる（ステップ 1-4）。
-   **結合テスト:** 開発用のサーバを `run` で起動し、ソケットにジョブの API のリクエストを送る（ステップ 2-3）。受理するすべての組み合わせ、同時と続けたジョブ、設定ファイルと環境、起動の行、ロックを確かめる。実際の LLM・ネットワーク・Webhook は呼ばない。
-   **分離のテスト:** `go list -deps` を起動する Go のテストを、調べる対象のパッケージのテストに置く（ステップ 1-5・2-4）。失敗すべきときに飛ばさず失敗することを、import を足して確かめる。
-   **静的な検査:** `make wasm-check`（ステップ 1-6）、`make -n` の展開を読むテスト（ステップ 2-5）、環境を読む場所の検査（ステップ 2-6）、パッケージ表と ADR の形のガード。
-   **手での確かめ:** AC-14 の対応表（PR-1）、AC-23 の `make` の実行（PR-2）、AC-05 の拡張からの確認（PR-3）、AC-25〜AC-29 の文書の文。
-   **後方互換:** 本番のバイナリ・ホスト・拡張の振る舞いは変えない（要件書 §4.4）。既存のテストで期待が変わるのは、`envaccess_test.go` の一覧と観測の検査、`internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs`、`TestMakeBuildBuildsEveryBinary` の確かめ（3 つを含む → 3 つにちょうど等しい）だけである。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 0083 と #771 の F-001 が `main` に入る前に着手する | ステップ 1-1 で確かめ、入っていなければ止まる |
| #772 が TS への一本化を決め、`make wasm-check` がない | ステップ 1-1 で止まり、#772 の見直しの結果に従う（設計書 §1.1） |
| 0083 の後の本番の `run.go` の形が、設計書の前提と違う | ステップ 1-1 で読み直し、食い違えば決定の変更として人に確かめる |
| 分離のテストが、`go` がない・出力が空などで黙って通る | 失敗に倒し、必ずあるはずのパッケージを先に確かめる。import を足して失敗することを確かめる（ステップ 1-5） |
| 分離のテストが Go のテストのキャッシュで通る | 調べる対象のパッケージのテストに置く（設計書 §7.3）。ただし、手元の OS とビルドタグ `test` ではコンパイルされないファイル（例えば `_linux.go`、`!test` のファイル）だけで `devfake` を import した場合は、手元の `make test`（`-count=1` なし）がキャッシュで通りうる。これは CI の `make test-ci`（`-count=1`、`Makefile:102`）が検出することに任せる |
| 並行のテストが時間に頼って不安定になる | 待ち方の差し替えで重なりを作る（§1.3 の決定 2） |
| 要素を網羅した記事が 0063 の規則の一部を欠く | §7 の対応表を PR-1 のレビューで読む |
| 文面を変えると、字数の条件（4,000 字の前後）を外れる | `TestArticlesPassWriter` が本物の `writer` で確かめる |
| 利用者が本番の代わりに開発用のサーバを動かす | 残余のリスクとして受け入れる。タイトルの印・起動の行・料金の不明と、`security.md` の規則で気づけるようにする（設計書 §5.1） |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: `make test`・`make ext-test` が走らせるテスト。
-   `static`: `make` のターゲット（`make wasm-check`）か、`make test` が走らせるガードのテスト。
-   `manual`: PR のレビューで読むか、手で操作して確かめる。記録は下の「実行の記録」に書く。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `internal/llm/devfake/deps_test.go::TestDependencies`（`net`・`os/exec` がない）、`LLMClient` を満たすことはコンパイル時の代入（`make test` のビルド）、ファイルに触れないことはコードを読んで確かめる | test、manual | ステップ 1-3・1-5 | PR-1 |
| AC-02 | `internal/llm/devfake/deps_test.go::TestDependencies` | test | ステップ 1-5 | PR-1 |
| AC-03 | `make wasm-check`（`make lint` と CI の `lint` のジョブが呼ぶ） | static | ステップ 1-6 | PR-1 |
| AC-04 | `internal/llm/devfake/client_test.go::TestJobsIndependent`・`::TestIgnoresRequest`、`cmd/yt2column-devserver/run_test.go::TestJobsIndependent` | test | ステップ 1-3・2-3 | PR-1、PR-2 |
| AC-05 | `cmd/yt2column-devserver/run_test.go::TestListeningLine`（`HOME` から導出したソケットのパス）、`::TestScenarioCombinations`（そのソケットで生成の開始と状態の取得）、拡張からの手での確認（ステップ 3-5） | test、manual | ステップ 2-2・2-3・3-5 | PR-2、PR-3 |
| AC-06 | `cmd/yt2column-devserver/run_test.go::TestIgnoresConfigAndEnv` | test | ステップ 2-3 | PR-2 |
| AC-07 | `cmd/yt2column-devserver/run_test.go::TestIgnoresConfigAndEnv`、`internal/config/envaccess_test.go::TestEnvAccessConfined` | test | ステップ 2-3・2-6 | PR-2 |
| AC-08 | `cmd/yt2column-devserver/run_test.go::TestRunArgs`、`internal/llm/devfake/scenario_test.go::TestEnumStrings`、`cmd/yt2column-devserver/docs_test.go::TestDevelopmentDocsDevServer`（`development.md` の綴り） | test | ステップ 1-2・2-1・3-2 | PR-1、PR-2、PR-3 |
| AC-09 | `cmd/yt2column-devserver/run_test.go::TestRunArgs`・`::TestStartupFailures`、`internal/llm/devfake/scenario_test.go::TestNewScenario` | test | ステップ 1-2・2-1・2-3 | PR-1、PR-2 |
| AC-10 | `cmd/yt2column-devserver/run_test.go::TestListeningLine`、`internal/llm/devfake/scenario_test.go::TestScenarioLogValue` | test | ステップ 1-2・2-2 | PR-1、PR-2 |
| AC-11 | `cmd/yt2column-devserver/run_test.go::TestLockHeld`。逆向き（本番のサーバ）は、本番が同じ `serversock.Acquire` を使うことと `internal/serversock` のロックのテスト | test | ステップ 2-2・2-3 | PR-2 |
| AC-12 | `internal/llm/devfake/articles_test.go::TestArticlesPassWriter`、`cmd/yt2column-devserver/run_test.go::TestScenarioCombinations` | test | ステップ 1-4・2-3 | PR-1、PR-2 |
| AC-13 | `internal/llm/devfake/articles_test.go::TestArticlesPassWriter`、`cmd/yt2column-devserver/run_test.go::TestScenarioCombinations` | test | ステップ 1-4・2-3 | PR-1、PR-2 |
| AC-14 | `internal/llm/devfake/articles_test.go::TestArticlesPassWriter`（字数と完成）、0063 の §3.2 の規則との対応表を読む（下の「実行の記録」の PR-1） | test、manual | ステップ 1-4 | PR-1 |
| AC-15 | `internal/llm/devfake/articles_test.go::TestArticlesPassWriter`、`cmd/yt2column-devserver/run_test.go::TestScenarioCombinations`、`cmd/yt2column-devserver/docs_test.go::TestDevelopmentDocsDevServer`（`development.md` の印） | test | ステップ 1-4・2-3・3-2 | PR-1、PR-2、PR-3 |
| AC-16 | `cmd/yt2column-devserver/run_test.go::TestScenarioCombinations`（`cost.state` が `unknown`）、`internal/llm/devfake/client_test.go::TestJobResponses`（`Model` が `ModelName`）、ビューアの表示（ステップ 3-5） | test、manual | ステップ 1-3・2-3・3-5 | PR-1、PR-2、PR-3 |
| AC-17 | `cmd/yt2column-devserver/run_test.go::TestScenarioCombinations`、`internal/llm/devfake/client_test.go::TestJobResponses` | test | ステップ 1-3・2-3 | PR-1、PR-2 |
| AC-18 | `cmd/yt2column-devserver/run_test.go::TestScenarioCombinations` | test | ステップ 2-3 | PR-2 |
| AC-19 | `internal/llm/devfake/client_test.go::TestDelayAndCancel`、`cmd/yt2column-devserver/run_test.go::TestScenarioCombinations`（待ちの回数） | test | ステップ 1-3・2-3 | PR-1、PR-2 |
| AC-20 | `internal/llm/devfake/client_test.go::TestDelayAndCancel`、`cmd/yt2column-devserver/run_test.go::TestStopNotice`（停止で取り消したジョブ）。取り消しの分類は本番の既存のテスト `internal/server/server_test.go::TestServeShutdownCancelsGeneration`・`internal/server/generation_test.go::TestGenerationFailureReasons` が確かめる | test | ステップ 1-3・2-3 | PR-1、PR-2 |
| AC-21 | `cmd/yt2column/devdeps_test.go::TestNoDevDependency`、`cmd/yt2column-server/devdeps_test.go::TestNoDevDependency`、`cmd/yt2column-nmhost/devdeps_test.go::TestNoDevDependency` | test | ステップ 1-5・2-4 | PR-1、PR-2 |
| AC-22 | AC-21 と同じテスト。`devfake` の import を足して失敗することを確かめ、コミットメッセージに書く | test | ステップ 1-5・2-4 | PR-1、PR-2 |
| AC-23 | `cmd/yt2column/makefile_test.go::TestMakeBuildBuildsEveryBinary`・`::TestMakeBuildDev`、`make build`・`make build-dev` の実行の記録（下の「実行の記録」の PR-2） | test、manual | ステップ 2-5 | PR-2 |
| AC-24 | `cmd/yt2column-devserver/run_test.go::TestScenarioCombinations` | test | ステップ 2-3 | PR-2 |
| AC-25 | `docs/dev/security.md` §8.2 を読む | manual | ステップ 3-1 | PR-3 |
| AC-26 | `cmd/yt2column-devserver/docs_test.go::TestDevelopmentDocsDevServer`（引数・値・印・`make build-dev`）、`docs/dev/guides/development.md` のほかの記述（同時に動かせないこと、拡張から確かめる手順、やり直し）を読む | test、manual | ステップ 3-2 | PR-3 |
| AC-27 | `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（パッケージの表）、`project_overview.md` と `CLAUDE.md` を読む | test、manual | ステップ 1-7・2-7・3-3 | PR-1、PR-2、PR-3 |
| AC-28 | `docs/dev/conventions/test_organization.md` を読む。`devfake` が `testutil/` の下にないことは、`internal/pipeline/pipeline_test.go::TestFakesCarryBuildTag`（`testutil/` の下のファイルの 1 行目を確かめる。`pipeline_test.go:564`、756d2f37）で確かめ、ファイルを動かして失敗することをステップ 1-7 で確かめる | manual、test | ステップ 1-7 | PR-1 |
| AC-29 | `TestADRRecords`（ADR と索引の形）、ADR-0045 の決定を要件書の決定と突き合わせて読む | test、manual | ステップ 3-4 | PR-3 |

### 実行の記録

手での確かめの結果と、確かめたコミットの ID を、ここに書く。記録したコミットより後に、その対象を変えるコミットが入ったら（`/fixpr` の修正を含む）、確かめ直して記録を改める。

PR-1:

AC-14 の対応表。行番号は `internal/llm/devfake/articles/elements.md` の行で、1 行目がタイトルの行である。`writer` はこの文面の 1 行目の `# ` を除いた残りをタイトルにし、2 行目以降を本文にする。表示用の Markdown は、タイトルの行・本文・出典の行の順に並ぶので、規則の当てはまり方は文面のとおりである。完成することは `TestArticlesPassWriter` が確かめる。

| 規則（0063 の §3.2） | 該当の箇所 |
|---|---|
| B-1 空行 | 2 行目ほか、ブロックの間の空の行 |
| B-2 見出し（`#`） | 1 行目（記事のタイトルの見出し） |
| B-2 見出し（`#` とタブ） | 34 行目 |
| B-2 見出し（`##` と U+0020） | 5・32・38・49・60・75 行目 |
| B-2 見出し（`##` とタブ） | 30 行目 |
| B-2 見出しの文字列へのインラインの規則 | 32 行目（強調とコードスパン） |
| B-2 見出しにならない行（`#` が 3 つ以上） | 18 行目 |
| B-2 見出しにならない行（`#` の直後に空白がない） | 20 行目 |
| B-3 コードブロック（`` ` ``、情報文字列あり） | 62〜65 行目。中の 63 行目の記法と 64 行目の行頭の空白は、そのまま表示される |
| B-3 コードブロック（`~`） | 67〜69 行目 |
| B-3 閉じる行（行頭の空白と、開いた並びより長い並び） | 71〜73 行目 |
| B-4 箇条書き（`-`、空行をはさんだ同じリスト） | 40・41・44 行目 |
| B-4 箇条書き（記号が変わると新しいリスト） | 46 行目（`*`）、47 行目（`+`） |
| B-5 番号付きリスト（`.`） | 51〜53 行目 |
| B-5 番号付きリスト（`)`、区切りが変わると新しいリスト、7 から始まる番号） | 55・56 行目 |
| B-5 項目にならない行 | 28 行目（10 桁の数字）、58 行目（`1\.` で始まる） |
| B-6 項目の継続 | 42 行目（行頭に空白のある、41 行目の項目の続き） |
| B-7 段落（複数の行の段落） | 7・8 行目 |
| 空白を詰めない表示 | 36 行目（連続した U+0020 と U+3000） |
| I-1 エスケープ（区切り文字の前の `\`） | 10 行目（`\*`）、16 行目（`\#`）、26 行目（`\**` の 1 つ目）、58 行目（`\.`） |
| I-1 エスケープ（区切り文字でない文字の前の `\` は文字のまま） | 26 行目（`\あ`、`C:\path`、空白の前の `\`） |
| I-2 コードスパン | 12 行目（1 個の並びと、中に `` ` `` を含む 2 個の並び）、79 行目 |
| I-2 閉じる並びのない `` ` `` | 22 行目 |
| I-3 強い強調 | 10 行目 |
| I-4 強調 | 10・32 行目、26 行目（`\**記号*` の 2 つ目以降） |
| I-3・I-4 の閉じない記号と 3 個以上の並び | 14 行目 |
| I-5 自動リンク（リンクになる） | 77 行目 |
| I-5 自動リンク（F-003 を満たさず、文字のまま） | 79 行目（`<https://example.com>`。直列化で末尾に `/` が補われる）、81 行目（大文字のスキーム、ホストが空、ユーザー名とパスワード、パスの `\`、全角の文字を含むドメイン名） |
| I-5 に当たらない `<` | 81 行目（`<http://`）、83 行目（`<3`、`a < b`） |
| `_` は強調の記号ではない | 83 行目 |
| 入れ子（強い強調の中では I-1 だけ） | 24 行目（中のコードスパンと自動リンクは記法にならない） |

現れない規則と形:

-   B-3 の閉じる行がないコードブロックは、`writer` が閉じないコードブロックを拒否する（`internal/writer/markdown.go` の `proseLines`）ので現れない。情報文字列に `` ` `` を含む `` ` `` の並びの行と、行頭に空白のあるコードブロックらしい行も、`writer` がコードブロックに似た行として拒否する（`looksLikeFence`）ので現れない。
-   I-5 の形のうち、`<` の直後が英字で自動リンクの文法を満たさないもの（空白・制御文字を含むものなど）は、`writer` が生の HTML として拒否する（`checkLineHTML`）ので現れない。
-   B-2 の「記号を除いた残りが空」の行（`## ` で終わる行）は、`writer` の検査は通るが、文面に入れていない。行末の空白を pre-commit の `trailing-whitespace` が取り除き、ファイルに残せないためである。取り除いた後の `##` は、20 行目と同じく「`#` の直後に空白がない行」になる。

確かめたコミットは、この表を加えたコミットである。

PR-2:

（ステップ 2-5 で、AC-23 の `make build`・`make build-dev` の実行の結果を書く。）

PR-3:

（ステップ 3-5 で、AC-05 の拡張からの確認の結果を書く。）

## 8. 成功の基準 (Success Criteria)

-   要件書の AC-01〜AC-29 が、§7 の確かめで満たされている。
-   各 PR で `make test`・`make lint`・`make deadcode` が通り、PR-3 で `make ext-test` が通る。
-   §7 の「実行の記録」に、AC-14・AC-23・AC-05 の記録がある。
-   セキュリティ: 本番の 3 つのバイナリの依存に `devfake` も開発用のサーバもない（AC-21）。開発用のサーバは環境から `HOME` だけを読み、設定ファイルも API キーも読まない（AC-06・AC-07）。`devfake` はネットワークにも外部コマンドにも触れない（AC-01）。
-   文書: 設計書 §3.5 の表の文書と ADR-0045 が、今のコードと一致する。

## 9. 次のステップ (Next Steps)

-   拡張だけの構成への移行（[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)）で、`devfake` を開発用のビルドに配線する。開発用のサーバは #773 でサーバと一緒に消す（設計書 §9）。
-   #772 が TS への一本化を決めたら、`make wasm-check` の `devfake` の行と ADR-0045 の該当の項を #772 が見直す。

## 変更履歴

- 2026-11-11 承認（isseis、#844）
