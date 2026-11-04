# 要件定義書：CLI の Webhook への投稿（`--slack`）と `SlackWebhookPublisher.Publish` の削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-04 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、CLI（`yt2column`）から Slack 互換の Incoming Webhook へ記事を投稿する機能（`--slack`）と、そのためにだけ残っているコード・テスト・文書を削除するための要件を定義する（[#642](https://github.com/isseis/yt2column/issues/642)）。削除した後、CLI の出力先は `--out` のファイルだけになる。

**背景:**

記事を Webhook に投稿する主な経路は、ブラウザ拡張のビューアである。ビューアがジョブの API の投稿（`POST /jobs/{key}/publish`）を送り、サーバは `PostMessages` で記事を投稿する（`internal/server/publish.go:13-20` のインターフェイス `WebhookPublisher` は `PostMessages` だけを持つ）。

CLI は開発用の経路だが、`--slack` を付けると記事を自分で Webhook に投稿できる（`cmd/yt2column/run.go:225`、`docs/dev/guides/cli.md` の「Webhook への投稿」）。そのためにだけ残っているコードがある。

-   `SlackWebhookPublisher.Publish`（`internal/publisher/slack.go:166-179`）。設定の確認、`PrepareSlackMessages` による記事の分割、`PostMessages(…, 0, …)` による送信を順に呼ぶだけである。これを呼ぶのは、CLI の実行が `job.RemoteOutput`（`internal/job/job.go:54-57`）を通して渡すパイプラインの公開の段階（`internal/pipeline/pipeline.go:116`）だけで、サーバは呼ばない（タスク 0059 の AC-07）。
-   CLI の出力先の選択のうち、Webhook の分岐。`destinationWebhook`（`cmd/yt2column/run.go:164-170`）、`--out` と `--slack` の組み合わせの拒否（`run.go:172-179`、`chooseDestination` `run.go:409-438`）、`deps.newSlackPublisher`・`newSlackPublisher`（`run.go:123`・`run.go:151-160`）、Webhook の出力の組み立て（`buildOutput` `run.go:645-662`）、投稿の完了の報告（`reportSuccess` `run.go:852-857`）、投稿の失敗のヒント（`reportWebhookHints` `run.go:1005-1037`）、検証・推敲のログの失敗の報告の Webhook の分岐（`run.go:984-986`）、`GODEBUG=http2debug` の警告の Webhook の分岐（`warnHTTP2Debug` `run.go:638-640`）。
-   CLI の設定の読み出し。CLI の `LoadFor` は `SLACK_WEBHOOK_URL` を読んで検証し（`internal/config/config.go:472-490`、`loadSlackWebhook` `config.go:721-735`）、`--out` の実行でも、不正な値なら設定の誤り（終了コード `2`）にする。CLI は値を `Config.SlackWebhookURL`・`Config.RequireSlackWebhookURL`（`config.go:302-317`）で受け取り、標準エラーへの出力の伏せ字の対象に加える（`cmd/yt2column/run.go:335-341`）。

CLI からの投稿は、拡張のビューアからの投稿と役割が重なる。開発で記事を確かめるには `--out` のファイルで足りる。

対応 issue: [#642](https://github.com/isseis/yt2column/issues/642)（関連: [#526](https://github.com/isseis/yt2column/issues/526)、タスク `0059_remove_legacy_articles_api`）

**本書の記述範囲:** 本書は、削除した後の CLI の観測できる振る舞い（受け付けるフラグ、拒否、読む環境変数）、削除するもの、残す送信の経路の確かめ方、文書が書くことを定める。削除の進め方（コミットの分け方、型・インターフェイスの形、テストの書き直し方、`deadcode` が見つけた関数の扱い）は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #642 で決めたこと:**

-   **CLI の出力先は `--out` だけにする:** `--slack` と、Webhook を出力先とする処理を `cmd/yt2column` から取り除く。CLI からの投稿は拡張のビューアからの投稿と役割が重なり、開発で記事を確かめるには `--out` のファイルで足りるためである。
-   **`--out` を必須にする:** 出力先を指定しないことの拒否（`errNoDestination`）は、`--out` が必須であることを示す拒否に改める。出力先の選択肢が 1 つになるためである。
-   **CLI は `SLACK_WEBHOOK_URL` を読まない。サーバは引き続き読む:** 拡張のビューアからの投稿にはサーバの Webhook の設定が要るためである。
-   **`SlackWebhookPublisher.Publish` を削除する:** `SlackWebhookPublisher` は `publisher.Publisher` インターフェイスを満たさなくなる。`FilePublisher.Publish` は CLI のファイルへの保存に使うので残す。
-   **CLI だけが使う設定の読み出しを削除する:** `Config.SlackWebhookURL`・`Config.RequireSlackWebhookURL` と、CLI の `LoadFor` が `SLACK_WEBHOOK_URL` を読む処理を削除する。サーバの `LoadServer`（`config.go:381`）と `loadSlackWebhook` は残す。
-   **削除するテストの振る舞いを、残る関数で確かめ直す:** `Publish` を使う `internal/publisher` のテストが確かめている振る舞いのうち、サーバに残る `PrepareSlackMessages` と `PostMessages` の振る舞いは、それらを直接使うテストで確かめ直す。削除の前後でカバレッジを比べる。
-   **`make test-integration-cli-slack` を削除する:** CLI の投稿の統合テストのためのターゲットであり、CLI の投稿がなくなるためである。
-   **`deadcode` で、ほかに使われなくなった関数がないことを確かめる。**
-   **拡張のビューアからの投稿と、サーバのジョブの API（`POST /jobs/{key}/publish`）は変えない。**

**要件定義の際に決めたこと（2026-11-04）:**

-   **`--slack` は未知のフラグとして拒否する:** 削除した後に `--slack` が渡されたときは、ほかの未知のフラグと同じく、フラグの解析の誤りとして終了コード `2` で拒否する（F-001）。`--slack` のための専用の拒否や、ビューアからの投稿への案内は加えない。理由: 利用者は開発者 1 人で、テストの段階にあり、移行のための仕組みを持たない方針であるため。専用の拒否を加えると、そのコードとテストが削除した機能のために残り続ける。
-   **CLI は `SLACK_WEBHOOK_URL` を無視する:** CLI の実行は `SLACK_WEBHOOK_URL` を読まない。設定されていても、値を検証せず、拒否もせず、標準エラーへの出力の伏せ字の対象にも加えない（F-002）。理由: サーバと CLI が同じ環境（`.envrc` など）を使っても、CLI が影響を受けないようにするため。CLI はもう Webhook に送らず、`yt-dlp` には許可した環境変数だけを渡す（`internal/transcript/ytdlp.go:209`）ので、CLI がこの値を受け取って出力に書く経路はない。設定されていたら拒否する案は、サーバのために設定した環境のままでは CLI を実行できなくなるので採らない。
-   **`make test-integration-slack` を残し、サーバに残る送信の経路で投稿する:** このターゲットの統合テストは、今は `Publish` で投稿する（`internal/publisher/slack_integration_test.go:23-34`）。`Publish` の削除の後は、サーバの投稿のジョブと同じく、記事を送る前の検査と分割（`PrepareSlackMessages`）を通してから送信（`PostMessages`）する形に改める（F-004）。理由: 検証の対象環境である Mattermost の本物の Webhook に記事を送れることを確かめる手段は、`make test-integration-cli-slack` を削除した後はこのターゲットだけになるため。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Webhook への投稿の経路を、拡張のビューアからサーバのジョブの API を通るものだけにする。
-   CLI だけが投稿のために持つコード・テスト・文書を取り除き、保守の対象を減らす。
-   サーバに残る送信の経路（`PrepareSlackMessages` と `PostMessages`）の振る舞いを、削除の前と同じ範囲で確かめ続ける。本物の Webhook への投稿も、統合テストで確かめ続ける。

### 2.2. スコープ (In Scope)

-   CLI の `--slack` フラグと、Webhook を出力先とする処理の削除。出力先の選択、組み合わせの拒否、投稿の完了の報告、投稿の失敗のヒント、`GODEBUG=http2debug` の警告と検証・推敲のログの失敗の報告の Webhook の分岐を含む（F-001）。
-   CLI が `SLACK_WEBHOOK_URL` を読まないようにすること（F-002）。
-   `SlackWebhookPublisher.Publish`、`Config.SlackWebhookURL`・`Config.RequireSlackWebhookURL`、CLI の `LoadFor` の `SLACK_WEBHOOK_URL` の読み込み、およびこの削除で使われなくなるコードの削除（F-003）。
-   CLI の投稿のテストとテストの補助、`Publish` を使う `internal/publisher` のテスト、`SlackMessagesForTest` の説明（`internal/publisher/test_helpers_slack.go:73-76`。「`Publish` が送る」と説明している）の削除・書き直し（F-003・F-004）。
-   `Makefile` の `test-integration-cli-slack` の削除と、それに触れる注釈・ガードのテストの書き直し（F-003）。
-   `make test-integration-slack` の統合テストを、サーバに残る送信の経路で投稿する形に改めること（F-004）。
-   文書の書き直し: `docs/dev/guides/cli.md`、`README.md` と `README.en.md`、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/security.md`、`docs/dev/guides/development.md`、`docs/dev/production_profile.md`、`CLAUDE.md` のうち、CLI の投稿・`--slack`・`test-integration-cli-slack`・`SlackWebhookPublisher.Publish` に触れる記述と、文書のガードのテストが読む値（F-005）。
-   Go のコメントのうち、CLI の `--slack` や `Publish` を今あるものとして説明するもの（例: `cmd/yt2column-server/run.go:61`、`cmd/yt2column-server/run_test.go:374`）の書き直し（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   拡張のビューアからの投稿と、サーバのジョブの API（`POST /jobs/{key}/publish`）の振る舞い。issue #642 が変えないと定めている。
-   サーバの `SLACK_WEBHOOK_URL` の読み込みと検証（`LoadServer`、`loadSlackWebhook`）。サーバは引き続き使う。
-   `publisher.Publisher` インターフェイスと `FilePublisher`、パイプライン（`internal/pipeline`）の公開の段階。CLI のファイルへの保存に使い続ける。
-   記事を Slack 向けの形にする処理（`PrepareSlackMessages`、分割、`silent`、メンションになりうる書式の検査）の振る舞い。サーバの投稿が使い続けるので変えない。
-   削除した `--slack` に対する移行の案内（「要件定義の際に決めたこと」）。
-   過去のタスクの文書（`docs/tasks/` の下のタスク 0062 以外）。その時点の記録であり、書き直さない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: CLI の出力先を `--out` だけにする

CLI は `--slack` を受け付けず、記事を `--out` のファイルにだけ書く。`--out` は必須になる。CLI のテストでないコードは Webhook へ送る手段を組み立てない。

**Acceptance Criteria**:
- **AC-01**: `--slack` を含む実行（`--slack`、`--slack=true`、`--slack=false` のどれでも、`--out` を同時に指定した場合も含む）は、終了コード `2` で終わる。標準エラーには、`--slack` が定義されていないフラグであることを示す行と、使い方の案内の行（`usageHint`）を書く。キャッシュのディレクトリ、`yt-dlp`、LLM の API、`--out` のパスのどれにも触れない。
- **AC-02**: `--out` を指定しない実行（動画の URL とほかのフラグは正しい）は、終了コード `2` で終わる。標準エラーには、`--out` が必須であることを示す拒否の行と、使い方の案内の行を書く。この拒否は、ほかの拒否と区別できる固有のセンチネルエラーに対応する。キャッシュのディレクトリ、`yt-dlp`、LLM の API には触れない。
- **AC-03**: `-h`/`--help` が標準出力に書く使い方は、`--slack` を挙げない。記事を `--out` のファイルに書くこと、`--out` が必須であることを示す。終了コードは `0` のままである。
- **AC-04**: `--out` を指定した実行の振る舞いは変わらない。空の `--out` の拒否、`--review-log` と `--out` が同じファイルを指すことの拒否、キャッシュのディレクトリの中の `--out` の拒否、記事をファイルに書いたことの報告の行、終了コード（`docs/dev/guides/cli.md` の「終了コード」のうち、Webhook に関わらない部分）は、削除の前と同じである。
- **AC-05**: `cmd/yt2column` のテストでないコードは、`publisher.SlackWebhookPublisher` を作らず、`internal/publisher` の Webhook の投稿の失敗を表す型・センチネルエラー（`SlackPostError`、`SlackHTTPStatusError`、`ErrSlackMention`）と投稿の上限時間（`SlackPostTimeout`）を参照しない。`GODEBUG` が `http2debug` を有効にしているときの CLI の警告は、API キーについての 1 行だけになり、Webhook URL について警告する行はない。

#### F-002: CLI は `SLACK_WEBHOOK_URL` を読まない

CLI の実行は `SLACK_WEBHOOK_URL` を読まず、その値にかかわらず振る舞いを変えない。サーバの `SLACK_WEBHOOK_URL` の扱いは変わらない。

**Acceptance Criteria**:
- **AC-06**: CLI の実行は、環境変数を読む関数（`run` に渡す `config.LookupFunc`）に `SLACK_WEBHOOK_URL` を問い合わせない。
- **AC-07**: `SLACK_WEBHOOK_URL` が不正な値（空の文字列、`https` でない URL など、今の CLI が設定の誤りにする値）に設定されていても、CLI の実行はそれを設定の誤りにしない。ほかの引数と設定が正しい `--out` の実行は、`SLACK_WEBHOOK_URL` が未設定の場合と同じ結果（終了コード、`--out` のファイル、標準エラーの行）になる。
- **AC-08**: サーバ（`yt2column-server`）の `SLACK_WEBHOOK_URL` の扱いは変わらない。未設定なら起動して投稿の操作を `webhook_not_configured` で拒否し、不正な値なら起動の前に設定の誤りとして拒否し、正しい値なら投稿の操作でその Webhook に投稿する。

#### F-003: CLI の投稿のためだけのコードとテストを削除する

**Acceptance Criteria**:
- **AC-09**: `publisher.SlackWebhookPublisher` は `Publish` メソッドを持たず、`publisher.Publisher` インターフェイスを満たさない。`publisher.FilePublisher` は `Publish` を持ち、`publisher.Publisher` を満たす。
- **AC-10**: `config.Config` は Webhook URL を返すメソッド（`SlackWebhookURL`・`RequireSlackWebhookURL`）を持たない。`config.ServerConfig.WebhookURL` は残る。
- **AC-11**: `Makefile` には `test-integration-cli-slack` のターゲットがなく、`.PHONY` とターゲットの注釈もそれを挙げない。CLI の投稿の統合テスト（`cmd/yt2column/integration_slack_test.go`）と、そのオプトインの変数 `YT2COLUMN_CLI_SLACK_INTEGRATION` を読むコードはない。
- **AC-12**: リポジトリの Go のコード（テストを含む）と `Makefile` に、CLI のフラグとしての `--slack` を今あるものとして扱う記述（フラグの定義、テストのケース、コメント）がない。
- **AC-13**: 削除の後、`make deadcode` は何も出力せず、終了状態 `0` で終わる。
- **AC-14**: テストを削除するコミットは、CLAUDE.md の「Testing Strategy」の「Deleting a test is a claim that must be checked」の手順に従う。そのコミットを単独にし、親のコミットを `BASE` とした `make coverage-compare` の出力が `coverage-compare: PASS` で終わる。その出力を最後の行まで、そのコミットのメッセージと PR の説明に書く。

#### F-004: サーバに残る送信の経路を確かめ続ける

**Acceptance Criteria**:
- **AC-15**: 削除の前に `Publish` を通して確かめていた振る舞いのうち、`PrepareSlackMessages` と `PostMessages` の振る舞い（送る前の検査による拒否と、そのとき何も送らないこと、分割、`silent`、メッセージの順序と間隔、投稿の失敗が送り終えたメッセージの数を示すこと）は、それらを直接呼ぶテストで確かめる。AC-14 の比較で、`internal/publisher` のテストでない関数の命令のカバレッジは下がらない。
- **AC-16**: `make test-integration-slack` は残る。統合テストは、今と同じ固定の 2 つの記事（1 通のメッセージの上限ちょうどの記事と、複数のメッセージに分かれる記事）を、送る前の検査と分割（`PrepareSlackMessages`）を通してから送信（`PostMessages`）して、本物のテスト用の Webhook に投稿する。オプトイン（`YT2COLUMN_SLACK_INTEGRATION=1`）、Webhook URL を `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` からだけ読むこと、URL がないときや `GODEBUG` が `http2debug` を有効にしているときに何も送らずに失敗すること、出力に URL とその一部を書かないことは、今のまま（`docs/dev/security.md` §2）である。

#### F-005: 文書とコメントを改める

**Acceptance Criteria**:
- **AC-17**: `docs/dev/guides/cli.md` は、`--slack` も、CLI に必要なものとしての `SLACK_WEBHOOK_URL` も挙げない。フラグの表は CLI が定義するフラグと一致し、`--out` を必須として説明する。「Webhook への投稿」の節は CLI の投稿を説明せず、終了コード `0` の説明は Webhook への投稿を挙げない。設定の表の説明は、CLI が `SLACK_WEBHOOK_URL` を読まないことを示す。
- **AC-18**: `README.md` と `README.en.md` は、CLI が `--slack` で投稿することを説明しない。「Webhook への投稿」の節の CLI の再送の説明と、`--slack` で投稿する方法への案内（`README.md:191-192`）を、CLI が投稿しないことに合わせて改める。`README.en.md` は `/mktrans` で `README.md` に合わせ、`make readme-sync-check` が通る。
- **AC-19**: `docs/dev/project_overview.md`（冒頭の説明、`SLACK_WEBHOOK_URL` の行）、`docs/dev/conventions/package_reference.md`（`cmd/yt2column` の行）、`docs/dev/security.md`（§2 の統合テストの表、`GODEBUG` の項）、`docs/dev/guides/development.md`（`make test-integration-cli-slack` の記述）、`docs/dev/production_profile.md`（`SLACK_WEBHOOK_URL` が要る場合の記述）、`CLAUDE.md`（「Architecture Overview」の `Publisher` の項。`SlackWebhookPublisher` は `Publisher` を満たさなくなる）は、CLI の投稿、`--slack`、`make test-integration-cli-slack`、`SlackWebhookPublisher.Publish` を今あるものとして説明しない。`SLACK_WEBHOOK_URL` はサーバだけが読むものとして説明する。`make test-integration-slack` の説明は、F-004 の投稿の形に合う。
- **AC-20**: 文書のガードのテスト（`cmd/yt2column/docs_test.go` の `TestSlackDocsContract` などの、文書の値をコードの値と比べるテスト）は、削除した後のコードの値（フラグ、`make` のターゲット、オプトインの変数）を文書と比べ、`make test` が通る。

#### F-006: 変えないもの

**Acceptance Criteria**:
- **AC-21**: `extension/` の下のファイルと、`internal/server/` のテストでない Go のファイルは、merge-base から変わらない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。機能の削除であり、残る経路の処理は変えない。

### 4.2. セキュリティ (Security)

本タスクは Webhook URL（`SLACK_WEBHOOK_URL`）の扱いと Webhook への送信の経路を変えるので、`docs/dev/security.md` を適用する。

-   **送信の経路:** 削除した後、Webhook に送るのは、サーバの投稿のジョブ（`PostMessages`）と `make test-integration-slack` の統合テストだけになる（F-001・F-004）。CLI はネットワークで送るものとして LLM の API だけを持つ（`security.md` §3）。
-   **秘密情報:** CLI は `SLACK_WEBHOOK_URL` の値を持たない（F-002）。そのため、CLI の標準エラーへの出力の伏せ字の対象から Webhook URL がなくなる。CLI が `yt-dlp` に渡す環境は許可した変数だけで（`security.md` §1、`internal/transcript/ytdlp.go:209`）、`SLACK_WEBHOOK_URL` を含まない。API キーの伏せ字は変えない。
-   **`GODEBUG=http2debug`:** CLI の警告は、API キーが標準エラーに書かれうることだけを示す（AC-05）。サーバの警告は変えない。
-   **統合テスト:** `make test-integration-slack` は、`security.md` §2 のテスト用の Webhook URL の規則（本番の `SLACK_WEBHOOK_URL` を読まないこと、URL がないときと `http2debug` が有効なときに失敗すること、出力に URL とその一部を書かないこと）を保つ（AC-16）。
-   **信頼できない入力の境界:** 本タスクは新しい入力を受け取らない。CLI が受け取らなくなる `SLACK_WEBHOOK_URL` のほかは、受け取る入力とその検査を変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   CLI の `--out` の実行の振る舞い（キャッシュ、上限時間、中断、検証・推敲のログ、再開）は変えない（AC-04）。

### 4.4. 互換性 (Compatibility)

-   CLI は `--slack` を受け付けなくなる。移行のための専用の拒否や案内は設けない（「要件定義の際に決めたこと」）。
-   `SLACK_WEBHOOK_URL` を設定したままでも、CLI の `--out` の実行はそのまま動く（AC-07）。
-   拡張、Native Messaging のホスト、サーバの間のやり取りは変えない（AC-08・AC-21）。

### 4.5. 保守性 (Maintainability)

-   CLI だけが使う投稿のためのコード、テスト、`make` のターゲット、文書の記述を残さない（F-003・F-005）。`make deadcode` が通る（AC-13）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   テストの削除は、CLAUDE.md の「Testing Strategy」の手順（削除を単独のコミットにし、`make coverage-compare` の結果を記録する）に従う（AC-14）。issue #642 は `go tool cover -func` での比較を挙げているが、その後にタスク 0055（#658）で手順が `make coverage-compare` に改められたので、今の手順に従う。
-   `README.md` を変えたら、同じコミットで `/mktrans` により `README.en.md` に反映する（CLAUDE.md「Translation Guidelines」）。
-   本物の Webhook に投稿する `make test-integration-slack` は、実行の前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。
-   文書のテストは、値と構造を確かめ、文の意味を確かめない。削除した記述が文書にないことを確かめる恒久のテストは加えない（CLAUDE.md「Testing Strategy」）。削除の確かめが一度だけ要るときは、タスクの `_checks/` の下で行う。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。

-   **投稿**: 記事を Slack 互換の Incoming Webhook に送ること。サーバの投稿のジョブが `PostMessages` で行う。
-   **送る前の検査と分割**: 記事を Slack 向けのメッセージにする前の検査（メンションになりうる書式の拒否など）と、メッセージの上限に合わせた分割。`PrepareSlackMessages` が行う。
-   **出力先**: CLI が記事を書く先。本タスクの後は `--out` のファイルだけになる。
-   **センチネルエラー**: `errors.Is` で見分ける、パッケージが定義した固有のエラーの値。
