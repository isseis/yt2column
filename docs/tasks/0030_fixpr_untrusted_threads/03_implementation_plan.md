# 実装計画書：fixpr でレビュースレッドの本文を信頼できないデータとして扱う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-007 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。判定と書き込みを担う開発用ツール `fixprguard`（`cmd/fixprguard` + `internal/fixprguard` + `internal/cmdrunner`）を作り、`/fixpr` と `/runpr` の文書がそれを呼ぶように切り替える。設計書 8 章の順序の制約をそのまま 6 つのフェーズにする。

-   **フェーズ 1（`internal/cmdrunner`）:** 外部コマンドの実行の境界を `internal/mergepr` から移して共有する。挙動は変えない。
-   **フェーズ 2（読み取り）:** 信頼の判定、スレッドの 2 段階の取得、区切りのブロック、`head`・`list`・`begin` の中身。
-   **フェーズ 3（変更範囲と git の状態）:** `inspect`・`finish`・`push` の中身。
-   **フェーズ 4（書き込み）:** `reply`・`edit-pr` の中身。
-   **フェーズ 5（コマンドと文書）:** `cmd/fixprguard`、`Makefile`、ツールの利用ガイド。
-   **フェーズ 6（コマンド文書の切り替え）:** `fixpr.md`・`runpr.md`・`_context.md`・用語集・文書のガード。**フェーズ 5 の PR をマージして `make install-fixprguard` を実行した後に**始める（設計書 8 章の 6）。

### 1.2. 実装原則

-   判定の分岐は Go のコードに置き、設計書の記述を計画で言い換えない。各ステップは設計書の節を参照する。
-   `CLAUDE.md`「Testing Strategy」に従い、各テストは対象を壊して失敗することを確かめてからコミットし、コミットメッセージに書く。壊す対象は設計書 7.2 の一覧から、そのフェーズで作ったものを選ぶ。
-   外部コマンドは、Go のテストでは `internal/cmdrunner/testutil` の fake で置き換える。git は、一時ディレクトリに作った実際のリポジトリで試す。`yt-dlp`・LLM API・Webhook・実際の GitHub は呼ばない。
-   依存パッケージを加えない（`CLAUDE.md`「Dependencies」）。標準ライブラリと `internal/strictjson` だけを使う。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの定義（`.claude/commands/`）も英語で書く。
-   `gosec` の指摘（`exec.CommandContext` の引数、ファイルの読み取り）を抑止するときは、`internal/mergepr/runner.go:19` と同じく、該当する最小の範囲に理由付きの `//nolint:gosec` を付ける。同じ種類の箇所すべてに同じ扱いを適用する。マジックナンバー（`mnd`）になる上限の値は、名前付きの定数にする。

### 1.3. 既存コード調査結果

調査は、コミット `bf1d576` で行った（設計書が確かめた `911b535` の後、`.claude/commands/` と `internal/` に変更はない）。

#### 1.3.1. 名前の変更の台帳

| 古い名前 | 新しい名前 | 場所と扱い |
|---|---|---|
| `mergepr.Runner`（インターフェース。`internal/mergepr/mergepr.go:46`） | `cmdrunner.Runner` | `mergepr.Runner` は `cmdrunner.Runner` の型の別名として残す。`New(runner Runner)`（`mergepr.go:57`）の呼び出し側は変わらない |
| `mergepr.NewOSRunner`（`internal/mergepr/runner.go:12`）と `osRunner`（`runner.go:14`） | `cmdrunner.NewOS(Options{})` | `NewOSRunner` は残し、`cmdrunner.NewOS` に委ねる。`osRunner` は取り除く。`cmd/mergepr/main.go:29` は変えない |

台帳にない名前の変更はない。以降の節は、この表から導かれる。

#### 1.3.2. すでにあるもの・足りないもの・変えるもの

| 領域 | すでにあるもの | 足りないもの・変えるもの |
|---|---|---|
| 外部コマンドの実行 | `internal/mergepr/runner.go` の `osRunner`。標準出力を上限なしで読み、失敗時に引数と標準エラーを含む文面を返す（`runner.go:18-26`）。fake は `internal/mergepr/test_helpers.go:48` の `fakeRunner`（`Run` だけを実装） | 出力の上限、`WaitDelay`、標準入力、型付きの失敗 `ExitError`、作業ディレクトリ。`Options.Dir` は設計書 3.2 の `Options` にないが、一時リポジトリの git と `make` を試すテストに要るので加える（ゼロ値は現在のディレクトリで、現行の挙動）。設計書 3.2 は「フィールドの並び」を実装で決めるとしており、設計書と食い違わない |
| strict な JSON | `internal/strictjson`（公開関数は `ParseObject`・`Required`・`RequiredString`・`OptionalString`。`strictjson.go:72,135,145,163`） | 追加なし。`internal/fixprguard` が使う。重複キーと `null` の拒否は `ParseObject` の挙動を、`internal/fixprguard` のテストで確かめる |
| 開発用ツールの先例 | `cmd/mergepr/main.go`（サブコマンドと `flag`）、`Makefile` の `install-mergepr`（`Makefile:68`）、`.PHONY` の行（`Makefile:53`） | `cmd/fixprguard`、`install-fixprguard`、`.PHONY` への追加 |
| パッケージの一覧のガード | `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:667`）は、非テストのソースを持つ `cmd/`・`internal/` のパッケージと、`testutil` のパッケージの行が `package_reference.md` にないと失敗する | `internal/cmdrunner`・`internal/cmdrunner/testutil`（フェーズ 1）、`internal/fixprguard`（フェーズ 2）、`cmd/fixprguard`（フェーズ 5）の行を、パッケージを作るフェーズで加える。加えないとそのフェーズの `make test` が通らない |
| `testutil` の数のガード | `TestFakesCarryBuildTag` は `testutil` のファイルの数を `21` と固定している（`pipeline_test.go:619`） | `internal/cmdrunner/testutil/mocks.go` を加えるフェーズ 1 で `22` にする |
| 文書のガード | `doc-guard` ジョブが `./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline` を毎回実行する（`.github/workflows/ci.yml:170-183`） | 新しい文書のガード `cmd/yt2column/fixpr_docs_test.go` は `cmd/yt2column` に置くので、CI の設定は変えない。`.claude/commands/` を読む既存のテストはない（`rg -n "claude/commands|fixpr|runpr" -g '*_test.go' .` が何も返さないことを `bf1d576` で確かめた） |
| 削除済みの語のガード | `TestSharedTokenRemoved`（`internal/pipeline/relay_guard_test.go:395`）が `docs/dev/` の下の `.md` と `CLAUDE.md` を読み、削除した語を探す | 新しい文書（`fixprguard_guide`）に、その語を書かない。実行は `make test` で確かめる |
| コマンドの文書 | `fixpr.md`（489 行。Phase 1〜6 は 141・198・258・333・374・429 行、Final report は 454 行）、`runpr.md`（333 行。ステップ 1・3・4 は 48・140・232 行、Authorization は 32 行） | 設計書 3.10 の表のとおり。`git add -A`（`fixpr.md:347`）、`REPLYBODY_EOF`（`fixpr.md:400,402,410,413`）、`comments(first:10)`（`fixpr.md:164,177`、`runpr.md:197`）、`coderabbitai[bot]` の直書き（`runpr.md:169,171`）を取り除く |
| ツールの文書 | `mergepr_guide.ja.md`（日本語の原本）と `mergepr_guide.md`（英訳）。両方が `Runner` インターフェースに触れる（`.ja.md:235`、`.md:236`） | `fixprguard_guide.ja.md` と `fixprguard_guide.md` を新設する。`mergepr_guide` の 2 つは、`Runner` の所在（`cmdrunner`）の記述を直す |

#### 1.3.3. 外部の前提の確かめ

| 前提 | 確かめた根拠 |
|---|---|
| `gh api` は標準入力から本文を読める | `gh api --help`（`gh` 2.96.0）に `--input file` と「`-` で標準入力」とある |
| `gh pr view --json isCrossRepository,headRepository,headRepositoryOwner`、権限の照会、GraphQL の `comments` の項目 | 設計書 3.3 の観測の表（2026-10-21、`gh` 2.96.0）。計画では再観測しない。観測できなかった値は設計書が安全側に倒している |
| `go install ./cmd/fixprguard` が `$(go env GOPATH)/bin` に入れる | `Makefile:66-69` の `install-mergepr` と同じ形。外部モジュールを使わないので、ネットワークは要らない |
| 利用者の git の設定がテストを乱さない | 手元の git は 2.50.1 で、グローバルの設定に署名の要求がありうる。一時リポジトリのテストは、リポジトリ内の設定で署名とフックを無効にし、グローバルの設定とシステムの設定を読まないようにする（実装時の義務。3 章のフェーズ 3 のステップ 3-7） |

#### 1.3.4. 実装時に決めること

-   `Worktree` の git の呼び出しが対象のディレクトリをどう渡すか（`Options.Dir` か、`git -C`）。設計書 3.7 の「起動する git の制限」と両立する方を、ステップ 3-2 で選び、理由をコミットメッセージに書く。公開の型（設計書 3.2）は、引数が増えても要件書の AC を変えない。
-   `fixpr_docs_test.go` が参照するツール側の値（commit の件名、`make` のターゲット、`protocol` の番号）の公開の形。ステップ 3-5 と 5-1 で決める。

### 1.4. 計画の際に決めたこと（2026-10-21）

利用者に確かめて決めたこと:

-   **ツール側の PR は、設計書 8 章の順に分ける（PR-1〜PR-5）。コマンド文書の切り替えは PR-6 とする。** `Worktree` の `inspect` と `finish`・`push` は、基準値の比較と判定した集合を共有するので、同じ PR に入れる。理由: 分けると PR-3a 単体では `finish` が使われず、基準値の比較を使う側が後の PR になる。レビューの大きさより、不変条件を 1 つの PR で読めることを優先した。
-   **`fixprguard_guide` は日本語版を先に作り、英語版を翻訳する。** 設計書 2.2 は英語のみとしていたが、`mergepr_guide.ja.md` と `mergepr_guide.md` の構成に揃える。理由: `CLAUDE.md` の翻訳手順は日本語版を先にコミットするとしており、`mergepr_guide` に先例がある。設計書 2.2 の表の「新設」の 1 行が、2 行（`.ja.md` と `.md`）に増える。AC は変わらない。

設計書が決めたこと（再び開かない）: ガードツールを Go で作ること、判定したパスだけを stage すること、2 段階の取得、`internal/cmdrunner` に移すこと、文書の切り替えをツールの導入の後にすること。

### 1.5. 申し送りの項目への対応

`implementation_handoff.md` は、このタスクにない。`design_handoff.md` の H-01〜H-10 は、設計書の付録 A がすべて記録している（`911b535` で確かめた）。計画で新しく扱うものはない。

## 2. 実装ステップ (Implementation Steps)

各フェーズの終わりに、グリーンゲート（`make test && make lint`）を通す。ステップに付けた設計書の節は、実装の仕様の出所である。テストの表の行や期待する文面は、計画に書かず、コードを書くときに決める。

### フェーズ 1: `internal/cmdrunner`（設計書 8 章の 1）

#### ステップ 1-1: `internal/cmdrunner` を作る

**変更するファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`

-   [ ] `Runner`・`InputRunner`・`Options`（`MaxOutput`・`WaitDelay`・`Dir`）・`ExitError`・`ErrOutputTooLarge`・`NewOS` を作る（設計書 3.2、1.3.2 の `Dir`）。
-   [ ] `MaxOutput` を超えた出力は、切り詰めた出力を返さず `ErrOutputTooLarge` で失敗する。上限を超えた後も、子プロセスがブロックしないように読み捨てて待つ。
-   [ ] `WaitDelay` を、キャンセルの後の待ちの上限に使う（子孫のプロセスがパイプを持ち続けても待ちが終わる）。
-   [ ] `Options` がゼロ値のとき、エラーの文面は `internal/mergepr/runner.go:25` と同じ形（コマンド・引数・終了の状態・標準エラー）で、かつ `errors.AsType[*ExitError]` で取り出せる。
-   [ ] `exec.CommandContext` に、`runner.go:19` と同じ理由の `//nolint:gosec` を付ける。
-   [ ] テストを `runner_test.go` に書く（出力の上限、上限を超えた後の読み捨て、`WaitDelay`、標準入力、`ExitError`、`Dir`）。

**完了条件**: `go test -tags test ./internal/cmdrunner` が通る。

#### ステップ 1-2: `internal/mergepr` を移す

**変更するファイル**: `internal/mergepr/mergepr.go`、`internal/mergepr/runner.go`、`docs/dev/developer_guide/mergepr_guide.ja.md`、`docs/dev/developer_guide/mergepr_guide.md`

-   [ ] `Runner` を `cmdrunner.Runner` の別名にし、`NewOSRunner` が `cmdrunner.NewOS(cmdrunner.Options{})` を返すようにする（1.3.1 の台帳）。`osRunner` を取り除く。
-   [ ] `internal/mergepr` の既存のテスト（`runner_test.go` の `TestOSRunnerReturnsOutput`・`TestOSRunnerReportsFailure` を含む）を変えずに通す。これらが、上限を指定しない既定で挙動が変わらないことの確認になる。cmdrunner 側に同じ内容のテストを重ねない。
-   [ ] `mergepr_guide.ja.md:235` と `mergepr_guide.md:236` の `Runner` の説明に、所在が `internal/cmdrunner` であることを加える。

**完了条件**: `go test -tags test ./internal/mergepr ./cmd/mergepr` が、テストを変えずに通る。

#### ステップ 1-3: fake と文書のガード

**変更するファイル**: `internal/cmdrunner/testutil/mocks.go`（`package cmdrunnertestutil`、`//go:build test`）、`internal/pipeline/pipeline_test.go`、`docs/dev/developer_guide/package_reference.md`

-   [ ] 期待するコマンドの列を順に再生し、順序の違いと余分な呼び出しで失敗する fake を作る。`InputRunner` も実装し、標準入力の内容を記録する。`internal/mergepr/test_helpers.go:48` の `fakeRunner` と同じ考え方で、`internal/mergepr` の `fakeRunner` は置き換えない（`Run` だけを実装する別の型で、移すと `mergepr` のテストの書き換えが増えるだけである）。
-   [ ] `TestFakesCarryBuildTag` の `21` を `22` にする（`pipeline_test.go:619`）。
-   [ ] `package_reference.md` に `internal/cmdrunner` と `internal/cmdrunner/testutil` の行を加える。

**完了条件**: `make test && make lint`。壊して確かめる: ステップ 1-1 の上限の確認を外すと、上限のテストが失敗する。

### PR-1 作成ポイント: shared command runner

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `refactor(0030): move the command runner to internal/cmdrunner`

**レビュー観点**: `internal/mergepr` の挙動が変わらないこと / 出力の上限と待ちの上限の扱い / `Options.Dir` の追加

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 が、出力の上限を超えた後の読み捨てと `WaitDelay` という、パイプの並行処理を伴う単独の複雑な処理を含む。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 読み取り（設計書 8 章の 2）

#### ステップ 2-1: 型とセンチネルエラー

**変更するファイル**: `internal/fixprguard/errors.go`、`internal/fixprguard/types.go`（名前は実装で決めてよい）、対応するテスト

-   [ ] 設計書 3.2・4 章の型（`Login`・`PRNumber`・`AuthorKind`・`Reason`・`Cause`・`Finding`・`Assessment`・`Fingerprint`・`Accept`）とセンチネルエラーを作る。ゼロ値は最も仮定の少ない解釈にし、`switch` の `default` は想定外（信頼しない）に倒す。
-   [ ] `ParseLogin`（文法は設計書 3.3）と `ParsePRNumber`（10 進数だけ。設計書 3.1）を、補正せずに拒否する形で作る。
-   [ ] エラーの文面に、GitHub から来た値と外部コマンドの引数・標準エラーを含めない（設計書 4 章）。

#### ステップ 2-2: スレッドの取得と指紋

**変更するファイル**: `internal/fixprguard/threads.go`、`internal/fixprguard/threads_test.go`

-   [ ] 1 段目（本文なし）と 2 段目（処理するスレッドの本文）の問い合わせと応答の検査を作る（設計書 3.4）。検査は `internal/strictjson` を使い、`errors` を含む応答、16 MiB を超える応答、`hasNextPage` が真のコメント（上限 100 件）を拒否する。
-   [ ] 指紋を作る。コメントの追加・編集・削除のそれぞれで変わることを、テストで確かめる。
-   [ ] `begin` 用に 1 ページ（100 スレッド）と `capHit`、`list` 用に全ページ（上限 2000 スレッド、超えたら `thread-cap`）を作る。
-   [ ] 外部コマンドの呼び出しに、時間の上限（GitHub は 2 分）を設ける。

#### ステップ 2-3: 信頼の判定

**変更するファイル**: `internal/fixprguard/trust.go`、`internal/fixprguard/trust_test.go`

-   [ ] 設計書 3.3 の判定の表を実装する。権限の照会は REST で、`login` ごとに 1 回、全体で 100 回までとし、照会の順序を決定的にする。失敗は `Cause` に写す。
-   [ ] 権限の応答は `internal/strictjson` で検査し、`role_name` だけを読む（ほかのキーは無視してよいと宣言する。設計書 3.3）。
-   [ ] 理由は該当するものをすべて集める（AC-11）。判定は著者と権限の情報だけを入力にし、本文を受け取らない形にする（AC-08）。

#### ステップ 2-4: 区切りのブロック

**変更するファイル**: `internal/fixprguard/fence.go`、`internal/fixprguard/fence_test.go`

-   [ ] `Fence(data, rand)` を作る（設計書 3.5）。nonce がデータに含まれていたら選び直し、使い果たしたら失敗する。
-   [ ] 設計書 7.2 の最後の段落に従い、nonce の確認だけが防げる入力（nonce と同じ文字列を含む本文）と、JSON のエスケープだけが防げる入力（改行と引用符で行頭の区切りを作ろうとする本文）を、別々のテストにする。

#### ステップ 2-5: `GitHub` 型

**変更するファイル**: `internal/fixprguard/github.go`、`internal/fixprguard/github_test.go`

-   [ ] `NewGitHub`（`--reviewer` が使えない名前なら失敗）、`Head`（設計書 3.9）、`List`、`Begin`（`accept` の指紋の照合を含む）、`ThreadPaths` を作る。`Begin` と `List` は最初に fork の判定をする。
-   [ ] `begin` の出力の形（`threadId`・`databaseId`・`authorType`・`body`・`path`・`line`・`url` に `fingerprint` を加える。設計書 3.4）と、`list` が本文を含まないことを、型の上で表す。
-   [ ] 想定外の著者のスレッドの本文を取得する問い合わせが、fake に一度も届かないことをテストで確かめる。

#### ステップ 2-6: テストの補助と文書のガード

**変更するファイル**: `internal/fixprguard/test_helpers.go`（`//go:build test`）、`docs/dev/developer_guide/package_reference.md`

-   [ ] GitHub の応答の組み立て（スレッド・コメント・権限）をまとめた補助を、`internal/cmdrunner/testutil` の fake の上に作る。
-   [ ] `package_reference.md` に `internal/fixprguard` の行を加え、`internal/strictjson` の行の共有先に `internal/fixprguard` を加える。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: 権限の判定を常に信頼にする、`role_name` の許可の一覧に `read` を加える、`Fence` の nonce の確認を外す、2 段目の指紋の確認を外す。

### PR-2 作成ポイント: read-side trust judgment and thread fetch

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `feat(0030): add the fixprguard trust judgment and thread fetch`

**レビュー観点**: 信頼の判定の表と理由の収集（AC-01〜AC-08）/ 想定外の著者の本文を取得しないこと（AC-09）/ 区切りの nonce の確認（AC-18）/ 応答の厳格な検査

**実装モデル要件**: frontier-required

**判定理由**: 本文を取得しないという安全の保証を担う判定のコードを含み、セキュリティの門にあたる（`mkplan.md` の panel mode の trigger）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 変更範囲と git の状態（設計書 8 章の 3）

#### ステップ 3-1: 範囲の規則

**変更するファイル**: `internal/fixprguard/scope.go`、`internal/fixprguard/scope_test.go`

-   [ ] 設計書 3.6 の規則 1〜6、パスの検査、コマンドの実行に関わるファイルの定義（H-10）を、1 か所（`scope.go`）に実装する。名指しされた想定外の著者のスレッドでは、`path` そのものの例外を適用しない。
-   [ ] 実行に関わるファイルの照合は、ASCII の大文字小文字を同一視する。ASCII でない名前は、`path` とバイト単位で一致するときだけ範囲に入る。

#### ステップ 3-2: git の基準値と、起動する git の制限

**変更するファイル**: `internal/fixprguard/gitstate.go`、`internal/fixprguard/gitstate_test.go`

-   [ ] `Baseline` を作る。形式は設計書 3.7（バージョンの接頭辞付き）。形が合わない値は `ErrInvalidInput` で拒否する。
-   [ ] hooks・config・info のダイジェストと、skip-worktree・assume-unchanged のフラグの確認を作る。`core.hooksPath` と worktree の共有の git のディレクトリに対応する。
-   [ ] git の起動の制限（設定が起動する外部コマンドの無効化、`GIT_*` の除去、ロックと端末の無効化）と、git・`gh` を起動する前のダイジェストの比較を作る（設計書 3.7）。
-   [ ] 1.3.4 の作業ディレクトリの渡し方を決める。

#### ステップ 3-3: 変更の数え上げ

**変更するファイル**: `internal/fixprguard/changes.go`、`internal/fixprguard/changes_test.go`

-   [ ] 名前の変更の検出を無効にした数え上げ、変更の種類と行数（バイナリは行数の代わりにバイナリであること）、`lstat` による種類の判定、`JudgedSetID` を作る（設計書 3.6）。
-   [ ] 上限（1000 ファイル、8 MiB）と、超えたときの `change-cap` の停止を作る。上限の値は定数にする。

#### ステップ 3-4: `Worktree` の `Begin` と `Inspect`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_test.go`

-   [ ] `Begin`（作業ツリーの確認 AC-20、基準値の採取）と `Inspect`（基準値との比較、数え上げ、範囲との照合）を作る。`Inspect` は GitHub から `path` を取り直す側（`GitHub.ThreadPaths`）に委ね、呼び出し元の `path` を受け取らない。
-   [ ] 範囲外の一覧に、ファイルごとの変更の種類と行数を含める（AC-23）。

#### ステップ 3-5: `Finish`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_test.go`

-   [ ] 設計書 3.6 の順序（厳密な確認 → `make fmt` → 整形後の集合の記録 → `make test`・`make lint` → 厳密な比較 → stage → stage の確認 → commit → commit の後の確認）を、1 回の呼び出しの中で強制する。`make` のターゲットは固定値で、引数から受け取らない。
-   [ ] stage は、判定したパスだけを、パスを文字どおりに解釈する指定で行う。`git add -A` を使わない。
-   [ ] commit の件名の組み立てと、`make` のターゲットの一覧を、`cmd/yt2column` の文書のガードが参照できる形で公開する（1.3.4）。

#### ステップ 3-6: `Push`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_test.go`

-   [ ] 設計書 3.6 の `push` の不変条件 (1)〜(4) を、push の直前の再確認と明示した refspec で実装する。

#### ステップ 3-7: 一時リポジトリでのテストと結合テスト

**変更するファイル**: `internal/fixprguard/test_helpers_git.go`（`//go:build test`）、`internal/fixprguard/integration_test.go`

-   [ ] git は一時ディレクトリの実際のリポジトリで、`make` は fake で動かす補助を作る（git のコマンドだけを実際の実行に回し、`make` と `gh` は fake に回す）。一時リポジトリは、リポジトリ内の設定で署名とフックを無効にし、利用者のグローバルの設定とシステムの設定を読まない。
-   [ ] 設計書 7.3 の結合テストを書く。判定の後にファイルを足す、hooks を足す、範囲外のファイルを変える、build checks が範囲内のファイルを書き換える、のそれぞれで、正しい場所で止まること。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: 範囲の規則で実行に関わるファイルの除外を外す、`finish` が build checks の後の確認を省く、`finish` が `git add -A` を使う、基準値の比較から hooks を外す、`push` が再確認を省く。

### PR-3 作成ポイント: change scope and git state guard

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `feat(0030): add the fixprguard scope check, finish and push`

**レビュー観点**: 範囲の規則と実行に関わるファイルの定義（AC-21・AC-23）/ `finish` の順序と判定した集合の一致（AC-21a）/ 基準値と `push` の再確認（AC-26）/ 起動する git の制限

**実装モデル要件**: frontier-required

**判定理由**: 範囲の確認と git の状態の確認という安全の門を、順序の強制と一時リポジトリを使う結合テストとともに実装する（`mkplan.md` の panel mode の trigger）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 書き込み（設計書 8 章の 4）

#### ステップ 4-1: 本文のファイルの検査と引数の文法

**変更するファイル**: `internal/fixprguard/write.go`、`internal/fixprguard/write_test.go`

-   [ ] 本文のファイルの検査を作る（`lstat` で通常のファイル、64 KiB 以下、有効な UTF-8。タイトルは 1 行も確かめる。設計書 3.8）。
-   [ ] スレッド ID・コメント ID の文法の検査を作る（設計書 3.8）。`owner/repo` は呼び出し元から受け取らず、ローカルのリポジトリから求める。

#### ステップ 4-2: `Reply` と `EditPR`

**変更するファイル**: `internal/fixprguard/github.go`、`internal/fixprguard/write.go`、各テスト

-   [ ] `Reply` を作る。現在の状態でスレッドを判定し直し（AC-07）、信頼する著者のスレッドか、名指しで指紋が一致するスレッドで、指紋が現在と一致し、コメント ID がそのスレッドのものであるときだけ書き込む。JSON の本文を組み立てて標準入力で渡し、成功した場合だけ解決する。
-   [ ] `EditPR` を作る。タイトルは `--title=<値>` の形で、本文は `--body-file` で渡す。シェルを介さない。
-   [ ] `reply` の本文に、終端の文字列だけの行、`$(...)`、バッククォート、引用符、`@file` の形、`true`・`null` の文字列を入れたとき、標準入力に渡る JSON の本文が文面と一致することを、テストで確かめる（AC-19）。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: `reply` が信頼の判定を省く。

### PR-4 作成ポイント: guarded GitHub writes

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `feat(0030): add the fixprguard reply and edit-pr writes`

**レビュー観点**: 本文を JSON として標準入力で渡すこと（AC-19）/ 想定外の著者のスレッドへの書き込みの拒否（AC-10）/ 本文のファイルの検査

**実装モデル要件**: frontier-recommended

**判定理由**: 外部への書き込みを担う、隔離された高リスクの処理である。単独の PR で読めるように分ける。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: コマンドと文書（設計書 8 章の 5）

#### ステップ 5-1: `cmd/fixprguard`

**変更するファイル**: `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`

-   [ ] 設計書 3.1 のサブコマンド（`head`・`list`・`begin`・`fence`・`inspect`・`finish`・`push`・`reply`・`edit-pr`）の引数の解釈、JSON の出力、`protocol` の照合、終了コード（0・1・2・3）を `cmd/mergepr/main.go` と同じ構成で作る。
-   [ ] 出力の JSON に `protocol` を含め、`--protocol` が合わなければ終了コード 2 で止まる。`--reviewer` がなければ止まる。
-   [ ] テストは、サブコマンドを複数回呼ぶ。呼ぶたびに `--protocol` と、必要な `--reviewer` を渡す（起動時の検査を毎回通すため）。
-   [ ] 設計書 4 章の `stop` の種類（`dirty-work-tree`・`cross-repository`・`git-state-changed`・`judged-set-changed`・`thread-cap`・`change-cap`）が出力に現れることを、テストで確かめる。

#### ステップ 5-2: `Makefile` と案内

**変更するファイル**: `Makefile`、`CLAUDE.md`、`docs/dev/developer_guide/package_reference.md`

-   [ ] `install-fixprguard` を、`install-mergepr`（`Makefile:68`）と同じ形で加え、`.PHONY` の行（`Makefile:53`）にも加える。
-   [ ] `CLAUDE.md` の「Quick Links」と「Build Commands」に案内を加える。
-   [ ] `package_reference.md` に `cmd/fixprguard` の行を加える。

#### ステップ 5-3: 利用ガイド

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`docs/dev/developer_guide/fixprguard_guide.md`

-   [ ] 日本語版を先に書き、コミットする。内容は、ツールの使い方、保証すること・しないこと（設計書 5.2・5.3）、止まったときの回復の手順、範囲の規則の一覧（設計書 3.6）、各サブコマンドを単独で実行する手順（設計書 5.4）。
-   [ ] 英語版を `/mktrans` で翻訳する。
-   [ ] ガイドに書いたコマンドのうち、ネットワークを使わないもの（`fence` と、一時リポジトリでの `inspect`・`finish`）を実際に実行し、出力と終了コードがガイドの記述と一致することを確かめる。結果は PR の説明に書く。

#### ステップ 5-4: 手動の確認

-   [ ] 利用者の承認を得て、実際の PR に対し `fixprguard head`・`list`・`begin`（読み取りだけ）を実行し、判定と理由が GitHub 上の実際のコメントと合うことを確かめる（`CLAUDE.md`「Tool Execution Safety」。ネットワークを使うので、承認なしに実行しない）。結果を PR の説明に書く。

**完了条件**: `make test && make lint`、`make build` が成功する。

### PR-5 作成ポイント: fixprguard command, Makefile and guide

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `feat(0030): add the fixprguard command and its guide`

**レビュー観点**: サブコマンドと終了コードの契約 / `protocol` の照合 / ガイドの記述が実際の挙動と一致すること / `Makefile`・`CLAUDE.md`・`package_reference.md` の整合

**実装モデル要件**: standard

**判定理由**: 既存のツール（`cmd/mergepr`）と同じ構成の配線と文書で、未確定の方式も隔離された高リスクの処理もない。trigger は該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: コマンド文書の切り替え（設計書 8 章の 6）

**前提（このフェーズに入る前に）**: PR-5 がマージされ、利用者が `main` で `make install-fixprguard` を実行している。`command -v fixprguard` が通り、`fixprguard` の `protocol` の番号がソースの値と一致することを、最初に確かめる。満たされていなければ、このフェーズを始めず、利用者に実行を求める。

#### ステップ 6-1: `_context.md`

**変更するファイル**: `.claude/commands/_context.md`

-   [ ] 自動レビュアーの名前（`coderabbitai`。GraphQL の `author.login` の形。REST の形は `coderabbitai[bot]`）とガードツール（`fixprguard`、`make install-fixprguard`）の 2 行を「Domain-specific」に加える（設計書 3.10、AC-35）。

#### ステップ 6-2: `fixpr.md`

**変更するファイル**: `.claude/commands/fixpr.md`

-   [ ] 設計書 3.10 の `fixpr.md` の表の各行を反映する（冒頭の説明、Authorization の新設、Convergence policy、Caller override、Shared rules の R6、Phase 1〜6、Final report）。
-   [ ] Phase 1 の `begin` に置き換えた後、`comments(first:10)` と `git add -A`、`REPLYBODY_EOF` の heredoc を取り除く（1.3.2 の行）。Phase 4 の commit の件名（`fix: address PR #NUMBER review comments`）は、ツールが組み立てる件名と一致させる。
-   [ ] コマンドの定義は英語で書く。

#### ステップ 6-3: `runpr.md`

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] 設計書 3.10 の `runpr.md` の表の各行を反映する（冒頭、Authorization、ステップ 1 の `head`、ステップ 3 の `list` と質問、ステップ 4）。
-   [ ] `coderabbitai[bot]` の直書き（`runpr.md:169,171`）を、`_context.md` の名前から作る形に替える。
-   [ ] `/fixpr` を自動で再実行する条件（`runpr.md:212-217`）の文面は変えない（AC-33）。

#### ステップ 6-4: 用語集

**変更するファイル**: `docs/translation_glossary.md`

-   [ ] AC-37 の語（フェーズ・信頼する著者・想定外の著者のスレッド・範囲外の変更）と、設計書 3.10 の語（ガードツール・区切りのブロック・指紋・基準値・名指し）を、英訳とともに加える。「フェーズ」の備考に、「段階」（`stage`）との区別を書く。

#### ステップ 6-5: 文書のガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

-   [ ] 設計書 7.4 の (1)〜(7) を検査するテストを書く。ツール側の値（commit の件名、`make` のターゲット、`protocol` の番号）は、ステップ 3-5 で公開した値から読み、文書に値を写さない。
-   [ ] AC 別の文書の確認（5 章の表の `fixpr_docs_test.go::TestFixprDocContract` と `::TestRunprDocContract` の部分テスト）を書く。

#### ステップ 6-6: 手動の確認

-   [ ] 切り替えの後、実際の `/fixpr` と `/runpr` の実行を観察する（想定外の著者のスレッドがある PR か、ない PR で、フェーズ 1 の `begin` から最終報告までが文書のとおりに進むこと）。想定外の著者のスレッドの実例は、リポジトリの外の人が書かないと作れないので、実例がなければ、`fixprguard begin` の出力を使い、文書の流れを読み合わせる。結果を PR の説明に書く。

**完了条件**: `make test && make lint`。壊して確かめる: ステップ 6-5 のテストの検査対象を 1 つ元に戻し（例: `runpr.md` に `coderabbitai[bot]` を戻す）、テストが失敗すること。

### PR-6 作成ポイント: command document switch

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6

**推奨タイトル**: `feat(0030): switch fixpr and runpr to the fixprguard tool`

**レビュー観点**: 文書の流れが設計書 6.1・6.2 と一致すること / 取り除くべき直書きが残っていないこと / 名指しの経路が直前の報告に限られること（AC-12）/ Authorization の網羅

**実装モデル要件**: frontier-recommended

**判定理由**: `/fixpr` と `/runpr` の実際の動作を切り替える、隔離された高リスクの変更である。PR を戻すときは、文書・ガード・用語集を 1 つの単位として戻す（6.1）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | `internal/cmdrunner`、`internal/mergepr` の移行 | PR-1 のマージ。`mergepr` のテストが変更なしで通る |
| M2 | 信頼の判定とスレッドの取得 | PR-2 のマージ |
| M3 | 範囲・git の状態の確認、`finish`・`push` | PR-3 のマージ |
| M4 | `reply`・`edit-pr` | PR-4 のマージ |
| M5 | `fixprguard` のインストールと利用ガイド | PR-5 のマージ、利用者の `make install-fixprguard` |
| M6 | コマンド文書の切り替え | PR-6 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `internal/cmdrunner` の新設、`internal/mergepr` の移行、fake | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | 型・取得・信頼の判定・区切り・`GitHub` 型 | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | 範囲・基準値・数え上げ・`Inspect`・`Finish`・`Push`・結合テスト | frontier-required |
| PR-4 | 4-1 / 4-2 | `Reply`・`EditPR`・本文のファイルの検査 | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 | `cmd/fixprguard`・`Makefile`・利用ガイド（日英）・手動の確認 | standard |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 | コマンド文書・`_context.md`・用語集・文書のガード | frontier-recommended |

### 3.3. 実装順序の根拠

設計書 8 章の順序の制約に従う。フェーズ 1 は挙動を変えないので、先に入れて後の PR の土台にする。フェーズ 2〜4 は、`internal/fixprguard` の中で、読み取り → 変更範囲と git → 書き込みの順に依存する（`Finish` が基準値と範囲を、`Reply` が信頼の判定を使う）。フェーズ 5 は、`cmd/` を `internal/` の後に置く。フェーズ 6 は、ツールがマージされて手元にインストールされるまで始められない（ツールがないと `/fixpr` と `/runpr` が止まる）。この前提は、フェーズ 6 の先頭に書いた。この PR 自体（フェーズ 1〜5）の `/fixpr` と `/runpr` は、従来の手順で動く。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の分類に従い、各フェーズでテストを書く。置き場所は、`internal/cmdrunner/runner_test.go`、`internal/fixprguard/*_test.go`、`cmd/fixprguard/main_test.go`、`cmd/yt2column/fixpr_docs_test.go`。fake は `internal/cmdrunner/testutil/mocks.go`（`package cmdrunnertestutil`、分類 A）、`internal/fixprguard` の補助は `test_helpers.go` / `test_helpers_git.go`（`//go:build test`、分類 B。`docs/dev/developer_guide/test_organization.md`）。表のテストの行や期待値は、コードを書くときに決める。

層が重なる経路では、片方の層だけが防げる入力を作る。区切りは nonce の確認と JSON のエスケープ（ステップ 2-4）、判定した集合と stage は `finish` の厳密な確認と stage の確認（ステップ 3-5）が該当する。

### 4.2. 結合テスト

ステップ 3-7 の、一時リポジトリでの `Begin` から `Push` の直前までの通しのテスト（設計書 7.3）。実際の GitHub と `make` は使わない。

### 4.3. 後方互換性

フェーズ 1 は `internal/mergepr` の既存のテストを変えずに通す。信頼する著者のスレッドの処理（要件書 4.4）は、フェーズ 6 の文書の切り替えまで変わらない。

### 4.4. 壊して失敗を確かめる

各フェーズの完了条件に、設計書 7.2 の一覧から選んだ対象を書いた。実装者は、対象を実際に壊してテストが失敗することを確かめ、コミットメッセージに書く。その壊し方が、名指しのテストの通る経路を実際に壊していることも確かめる（例えば `Fence` の nonce の確認を外すときは、nonce と同じ文字列を含む本文のテストが失敗すること）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行できて誤った挙動で失敗するテスト、`static` は文書のガードのテストなど実行される検査、`manual` は観察を表す。テストの名前は、対応するステップが作る。文書（コマンドの手順）が担う AC は、LLM の実行そのものをテストできないので、ツール側の出力の契約（`test`）と文書の記述の検査（`static`）を重ね、実際の実行の観察（`manual`）を補う（設計書 5.2）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01〜AC-04 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors`（著者の種類・権限・混在・名前の違う Bot・名前が同じ User） |
| AC-05 | test | `internal/fixprguard/threads_test.go::TestCommentCap`（11 件以上、100 件超） |
| AC-06 | test | `internal/fixprguard/trust_test.go::TestAssessUnverifiableAuthor`（著者なし・名前の不備・種類の不明・照会の失敗の原因・予算超過） |
| AC-07 | test | `internal/fixprguard/github_test.go::TestBeginAssessesEveryCall`、`::TestReplyReassesses` |
| AC-08 | test | `internal/fixprguard/github_test.go::TestAssessmentIgnoresBody` |
| AC-09 | test | `internal/fixprguard/github_test.go::TestBeginNeverFetchesUntrustedBody`、`::TestBeginStageTwoFingerprintMismatch` |
| AC-10 | test | `internal/fixprguard/write_test.go::TestReplyRefusesUntrustedThread` |
| AC-11 | test | `internal/fixprguard/github_test.go::TestBeginReportsAllReasons`。`static`: `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/final_report` |
| AC-12 | test | `internal/fixprguard/github_test.go::TestBeginAcceptsNamedThread`。`static`: `fixpr_docs_test.go::TestFixprDocContract/accept_only_reported`。名指しを直前の報告に限ることは文書に依存する（設計書 5.2） |
| AC-13 | test | `internal/fixprguard/github_test.go::TestBeginAcceptChangedFingerprint`（追加・編集・削除） |
| AC-14 | test | `cmd/fixprguard/main_test.go::TestBeginNothingToProcess`。`static`: `fixpr_docs_test.go::TestFixprDocContract/nothing_to_process` |
| AC-15 | test | `internal/fixprguard/integration_test.go::TestFinishCommitSubject`。`static`: `fixpr_docs_test.go::TestFixprDocContract/round_counting` |
| AC-16・AC-17 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/fenced_prompts` |
| AC-18 | test | `internal/fixprguard/fence_test.go::TestFenceNonceAbsent`、`::TestFenceRetriesOnCollision`、`::TestFenceGivesUp` |
| AC-19 | test | `internal/fixprguard/write_test.go::TestReplyBodyVerbatim`、`::TestEditPRVerbatim` |
| AC-20 | test | `internal/fixprguard/worktree_test.go::TestBeginRejectsDirtyWorkTree`（未追跡を含む。GitHub を呼ばない） |
| AC-21 | test | `internal/fixprguard/changes_test.go::TestEnumerateChanges`、`internal/fixprguard/scope_test.go::TestScopeRules`、`internal/fixprguard/integration_test.go::TestFinishAbsorbsFormatting` |
| AC-21a | test | `internal/fixprguard/integration_test.go::TestFinishStopsOnJudgedSetMismatch`、`::TestFinishStopsWhenBuildChecksRewrite`、`::TestFinishStagesJudgedPathsOnly` |
| AC-22・AC-23 | test | `internal/fixprguard/worktree_test.go::TestInspectInScope`、`::TestInspectListsOutOfScope`（種類・行数・バイナリ、終了コード 3 は `cmd/fixprguard/main_test.go::TestInspectExitCode`） |
| AC-24・AC-25 | test | `internal/fixprguard/integration_test.go::TestFinishAfterOutOfScope`、`::TestInspectLeavesWorkTree`。`static`: `fixpr_docs_test.go::TestFixprDocContract/out_of_scope_stop` |
| AC-26 | test | `internal/fixprguard/gitstate_test.go::TestBaselineDetectsChange`、`::TestBaselineIgnoresIndexAndFetchHead`、`::TestGitNotLaunchedAfterConfigChange` |
| AC-27 | test | `internal/fixprguard/github_test.go::TestHeadRequiresSameRepository`。`static`: `fixpr_docs_test.go::TestRunprDocContract/head_before_checkout` |
| AC-28〜AC-31 | static + manual | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（質問・停止・マージしないことの記述）。ツール側の `stop` の種類は `cmd/fixprguard/main_test.go::TestStopKinds`。実際の実行の観察はステップ 6-6 |
| AC-32 | test | `internal/fixprguard/github_test.go::TestListOmitsBodies`。`static`: `fixpr_docs_test.go::TestRunprDocContract/no_body_fetch` |
| AC-33 | static | `fixpr_docs_test.go::TestRunprDocContract/auto_fixpr_condition` |
| AC-34 | static | `fixpr_docs_test.go::TestFixprDocContract/people_answered_scope` |
| AC-35 | static | `fixpr_docs_test.go::TestContextAndCommandsAgree`（`_context.md` が名前を定め、`fixpr.md`・`runpr.md` が `--reviewer` に渡す） |
| AC-36 | static | `fixpr_docs_test.go::TestAuthorizationSections` |
| AC-37 | static | `fixpr_docs_test.go::TestGlossaryTerms` |

ネットワークを使わない文書とツールの確認の手動の補助は、ステップ 5-3 から 5-4 と 6-6 に置いた。`manual` は `test` や `static` の代わりにしない。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 対策 |
|---|---|
| ツールを導入する PR の間（PR-1〜PR-5）は、この PR 自体の `/fixpr` が従来の手順で動き、防御がない | 設計書 8 章が受け入れている。PR-1〜PR-5 は、コマンド文書を変えないので、既存の挙動に影響しない |
| PR-6 を戻すと、文書とガードと用語集の整合が崩れる | PR-6 は 1 つの単位として戻す（`git revert` で PR のマージコミットごと）。ガードのテストが文書を縛っているので、一部だけ戻すと CI が失敗する |
| 利用者の git のグローバルの設定（署名など）が、一時リポジトリのテストを乱す | ステップ 3-7 で、リポジトリ内の設定で無効にし、グローバルの設定を読まない |
| 同じ `protocol` の番号のまま規則が変わり、古いバイナリが残る | 設計書 5.3 が受け入れている。ツールを変える PR のマージの後は、再インストールが要ることを `fixprguard_guide` に書く |
| フェーズ 6 の前提（インストール）が満たされないまま始まる | フェーズ 6 の先頭の前提の確認で止める |

### 6.2. スケジュールのリスク

-   PR-2 と PR-3 は大きい。レビューの量を減らすため、各ステップの完了のたびにコミットを分ける。PR を分けると、基準値の比較を使う側が後になる（1.4）。

## 7. 実装チェックリスト (Implementation Checklist)

PR ごとにまとめる。各ステップの詳細は 2 章のチェックボックスで追う。

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4）
- [ ] 利用者が `make install-fixprguard` を実行した
- [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6）
- [ ] 要件書の AC-01〜AC-37 のすべてが、5 章の検証でカバーされている

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-37 を、5 章の検証が満たす。信頼する著者のスレッドの処理は、今までどおりである（要件書 4.4）。
-   **品質:** `make test && make lint` が各 PR で通る。新しいテストは、壊して失敗することを確かめ、コミットメッセージに書く。
-   **セキュリティ:** 想定外の著者のスレッドの本文が、ツールのプロセス、エージェント、`/fixpr` と `/runpr` のセッションのどれにも入らない（AC-09・AC-32）。返信と PR の説明の文面が、シェルのコマンドにならない（AC-19）。
-   **文書:** `fixprguard_guide`（日英）、`package_reference.md`、`CLAUDE.md`、用語集が実装と一致する。

## 9. 次のステップ (Next Steps)

1.  この計画書を人のレビューにかけ、`approved` にする（`/runpr` が承認を記録する）。
2.  承認の後、`/runplan 0030` で PR-1 から実装する。PR-5 のマージの後は、利用者が `make install-fixprguard` を実行してから PR-6 に進む。
