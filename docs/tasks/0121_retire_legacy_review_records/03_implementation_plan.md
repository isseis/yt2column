# 実装計画書：古い記録行の読み方規則を、対象ブランチがなくなった後に外す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-24 |
| Review date | 2026-11-24 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的

本書は、[01_requirements.md](01_requirements.md)（F-001〜F-004、AC-01〜AC-12）および承認済みの [02_architecture.md](02_architecture.md) に基づき、タスク 0118 が移行期の後方互換として残した古い記録行 3 形式（`Codex local review:`・`(codex)`・`(coderabbit: <kind>; codex: <kind>)`）の読み方規則を、`origin` に該当ブランチがないことを確認したうえで安全に撤去するための実装計画を定める。

### 1.2. 実装方針

本タスクは Go の製品コード（`internal/`・`cmd/yt2column/*.go` の非テストファイル）を変更せず、タスク専用の移行ゲートスクリプト、共有コマンド定義 3 ファイル、判定スクリプト `review-records.sh`、および回帰テスト `review_records_test.go` を更新する。設計の詳細は [02_architecture.md](02_architecture.md) §1〜§8 に従い、次の方針で進める。

- **移行ゲート優先:** 実装開始前に `_checks/check_open_branches.sh` を整備・検証したうえで実 `origin` に対して実行し、終了コード `0`（`matches: 0`）を確認できない限りコミット確定やフェーズ 2 以降へ進まない（F-001）。
- **テスト非削除移行:** `cmd/yt2column/review_records_test.go` の既存 `(codex)` 受理ケースを削除せず拒否ケースへ書き換えることで、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) の削除専用コミットと `make coverage-compare` を不要に保ち、スクリプト変更と一貫して検証する（設計書 §1.3 決定 1）。
- **単一の正本と単一の正規表現:** 記録行の正本を `.claude/commands/_lib/local-cli-review.md`、見た範囲の正規表現を `.claude/commands/_lib/review-records.sh` の `SEEN_RE` の 1 定義に保ち、全モード（`coverage`・`unreviewed`・`cross-check`・互換別名 `fable`）へ一括適用する（設計書 §1.1・§3.3）。
- **テスト補助ファイルの新設なし:** 既存の `rrFixture` および `rrFixture.seen` をそのまま再利用するため、`testutil/` や `test_helpers*.go` は新設しない（`docs/dev/conventions/test_organization.md`）。

### 1.3. 既存コード調査結果

作成時点の `origin/main`（コミット `3fff7f51b6bf51721f1bbbe09e87ec2e4ea9d67d`）を調査し、設計書 §1・§3.4 の基準から進んだ変更と各変更対象の位置を確定した。

- **タスク 0120 のマージ状況:** コミット `3fff7f51`（PR #1115）でタスク 0120 が `origin/main` にマージ済みである。したがって、設計書 §3.4「タスク 0120 先行マージ時の差し替え対応表」の全 4 行をそのまま適用する（`.claude/commands/_lib/fable-review.md` は `.claude/commands/_lib/cross-check-review.md` へ改名済み、`review-records.sh` は `cross-check` モードと互換別名 `fable` を持ち、`review_records_test.go` は `crossCheckCases`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` を持つ）。
- **コマンド定義 3 ファイルの対象位置（コミット `3fff7f51`）:**
  - `.claude/commands/_lib/local-cli-review.md`: `## Reading the record lines` は `:293-326`。撤去対象は `:307` の `and \`Codex local review:\``（`:308` は変更しない）、`:314` の `(<cli>)`、および `:323-326` の旧形式読み取り・退役条件段落の 3 か所のみ。範囲なし `CodeRabbit local review: not run`（`:304-306`）等の維持対象はそのまま残す。
  - `.claude/commands/_lib/task-pr-handoff.md`: `Part B step 3` の指摘転記項目は `:320-331`。撤去対象は `:324-325` の `and \`Codex local review:\`` と `:329-331` の Codex CLI および Codex の言及のみ。`Cross-check finding:`・`Fable finding:` の読み取り、本文転記禁止、`**Mention check.**`（`:276`）、`review-records.sh` の呼び出しブロック（`:228-232,340-344`）は不変。
  - `.claude/commands/_lib/cross-check-review.md`: `## The prompt` の `Settled issues` は `:540-550`。撤去対象は `:543-544` の `` `Codex local review:` (legacy lines from older branches), `` のみ。`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` は維持する。
  - 3 ファイル全体の変更見込みは十数行の小さな差分にとどまるため、AC-06 は設計書 §7.2 のとおり記録した diff の読み取りとする。
- **`review-records.sh` の対象位置（コミット `3fff7f51`）:**
  - `SEEN_RE` の定義は `:269` の 1 か所、参照は `judge_seen`（`:832`、`coverage`・`unreviewed` 共通）、`cross_check` ルール 2（同じ実行の修正、`:927`）、`cross_check` ルール 4（マージ後確認の修正、`:982`；いずれも設計書 §3.3）の 3 か所のみで、第 3 キャプチャグループは参照されていない。
  - `NOT_RUN_RE` は `:271` であり変更しない。ヘッダの `unreviewed` モード説明コメントは `:79` にある。
- **`cmd/yt2column/review_records_test.go` の対象位置と再利用（コミット `3fff7f51`）:**
  - 既存の `seenRangeForm`（`:83-86`）、`rrFixture`（`:91-98`）、`rrFixture.seen`（`:184-186`）、`readCrossCheckForms`（`:1011-1028`）をそのまま再利用する。
  - 書き換えが必要な既存箇所は 4 か所: `coverageCases()` の `"seen by codex"`（`:355-359`）、`"HEAD behind LAST_REVIEWED"` のコメント（`:381-382`）、`not run` 系 2 ケースの入力（`:425,431`）、および `unreviewedCases()` の `"fix seen by the standalone path's review"`（`:899-906`、`:904` が `"codex"`）。
  - 新規追加先は `coverageCases()`（`TestReviewRecordsCoverage` `:845` と `TestReviewRecordsUnreviewed` `:946` の双方で実行）、`crossCheckCases()`（`TestReviewRecordsCrossCheck` `:1865`）、および `TestReviewRecordsFableAlias`（`:2047`）。
- **既存文書ガードテストへの影響（コミット `3fff7f51`）:** 以下の既存テストはいずれも今回の編集によって前提が崩れないため、更新不要である。
  - `cmd/yt2column/review_records_test.go`: `TestReviewRecordsCallSite`（`:2107-2139`）・`TestRecordLinePrefixesDistinct`（`:2196-2251`）・`TestRecordCommitsCheckMention`（`:2275-2301`）
  - `cmd/yt2column/fixpr_docs_test.go`: `TestContextAndCommandsAgree`（`:1122-1150`）・`TestLocalReviewGuard`（`:1297-1319`）
  - `cmd/yt2column/review_mention_docs_test.go`: `postingSections`（`:60-79`）・`TestMentionsOnlyInAllowedRows`（`:93-137`）・`TestPostingPathsCheckMention`（`:146-169`）・`TestPostingCommandsInventory`（`:238-308`）
  - `cmd/yt2column/cross_check_state_docs_test.go`（`:48-152`）、`internal/ciskip/git_test.go`（`:296`）
- **関連文書（AC-11）の事前確認（コミット `3fff7f51`）:**
  - `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を対象に `rg -n -i "codex" docs/dev/guides/ docs/dev/conventions/ CLAUDE.md | grep -v -i "@codex"` を実行し、出力が空（0 件）であることを確認した（`@codex` メンション拒否の記述は `01_requirements.md` §2.3 のスコープ外）。実装開始時および完了時に再度確認する。
- **外部ツール・ランタイムの前提確認:**
  - 開発環境で `/bin/bash`（GNU bash `3.2.57`）、`PATH` 上の `bash`（GNU bash `5.3.15`）、`git`（`2.50.1`）、`go`（`go1.27.1`）の存在を確認した。
  - `git init -h`（`--bare`）・`git fetch -h`（`--no-tags`・`--quiet`）・`go help build`（単一 `.go` ファイルの `-o` ビルド）・`go help environment`（`GO111MODULE`・`GOTOOLCHAIN`・`GOENV`・`GOFLAGS`・`GOPROXY`・`GOWORK`・`GOCACHE`）・`Makefile:144`（既存の Go ビルド隔離変数 `GOTOOLCHAIN=local GOENV=off GOFLAGS= GOPROXY=off GOWORK=off`）・`.github/workflows/ci.yml:277-291`（`doc-guard` コマンド）・`Makefile:352-376`（`deadcode` ターゲット）を一次情報で確認した。
  - `internal/mergepr/corrections.go:249-289` を確認した。同ファイルは `isNoncharacter`（`:249-251`）・`isInvisible`（`:256-261`、`unicode.Cc`・`Cf`・`Zl`・`Zp`）・`rejectKinds`（`:266-289`、`utf8.DecodeRune` で `r == utf8.RuneError && size == 1` を不正 UTF-8 として検出）を持つが、`isInvisible` は `:257-259` で `\t` を**除外**している。一方、設計書 §3.1 のエスケープ対象は `\t` を**含む** `unicode.Cc` 全体であるため、`_checks/escape_untrusted.go` では `\t` の除外を設けない。
- **申し送り事項:** [design_handoff.md](design_handoff.md) の H-01〜H-03 はすべて [02_architecture.md](02_architecture.md) §1.4 に記録済みである。`implementation_handoff.md` は存在しない。

### 1.4. 計画の際に決めたこと（2026-11-24）

以下の 2 点をレビュアーと合意して決めた。

- **`_checks/check_open_branches.sh` における信頼しない文字列のエスケープ実装方式:**
  - **決定と理由:** Go 標準ライブラリのみを用いる補助ファイル `docs/tasks/0121_retire_legacy_review_records/_checks/escape_untrusted.go` を併設し、`check_open_branches.sh` 実行時に一時ディレクトリへ 1 回 `go build` して使う（案 A）。開発・CI 環境で確実に利用できる `go` 以外のランタイム依存（`python3`・`perl`）を増やさず、`unicode` / `unicode/utf8` の分類（`internal/mergepr/corrections.go:249-289` の `isInvisible` から `\t` の除外を外した形）で `Cc`（タブ含む）・`Cf`・`Zl`・`Zp`・非文字・不正 UTF-8 を正確にエスケープできるためである。`_checks/` は `_` 始まりのため `go list ./...`・`make test`・`make lint`・`make deadcode` の対象外となる。
  - **ADR の判断:** ADR なし（タスク専用の `_checks/` 一時スクリプトの内部実装方式であり、常設の開発ツールや [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の決定を変えないため）。
- **`_checks/check_open_branches.sh` のローカル擬似 `origin` 壊し確認（設計書 §7.3）の配置方法:**
  - **決定と理由:** ローカル検証スクリプト `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh` をコミットし、フェーズ 1 の壊し確認と AC-02 の検証に使う（案 A）。設計書 §7.3 の 7 シナリオすべてをネットワークなしの 1 コマンドで再現実行でき、レビュー修正後の再検証と AC-02 の実行可能な検証対象（`test`）を確実に残せるためである。`_checks/` 配下のため `make test` や CI には含まれない。
  - **ADR の判断:** ADR なし（タスク専用の `_checks/` 配下に検証スクリプトを置くかどうかの選択であり、タスク内で閉じるため）。

## 2. 実装ステップ (Implementation Steps)

フェーズ構成と順序は [02_architecture.md](02_architecture.md) §8 の実装優先順位に従う。なお、ステップ 2-2（`review-records.sh`）とステップ 3-1（`review_records_test.go`）は `make test` 上で密結合しているため、同一 PR にまとめてグリーンゲートを通す。

### フェーズ 1: 移行ゲートの整備と実装開始前の前提確認（F-001）

#### ステップ 1-1: 移行ゲートスクリプトとエスケープ補助の作成

- **変更対象ファイル:**
  - `docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh`（新規）
  - `docs/tasks/0121_retire_legacy_review_records/_checks/escape_untrusted.go`（新規）
- **作業内容:**
  - [x] 設計書 §3.1・§4.2・§5.2・§6.1 に従い、`_checks/check_open_branches.sh` を作成する。引数なし検証、一時 bare リポジトリ作成と `EXIT` トラップでの後始末、非 shallow・非 partial clone 時のみの `alternates` 登録、および相対 `origin` URL のメイン作業ツリー基準での絶対パス解決を実装する。あわせて、`ls-remote --heads` とのブランチ集合一致と `refs/heads/main`・各ブランチの `commit` 種別確認、隔離環境での 40 桁コミット ID 指定による `main` 未到達コミット全体の 3 形式照合、CRLF 行末の `\r` 除去、バッファリング出力と終了コード `0`/`1`/`2` の契約を満たす。コメント・識別子・診断メッセージは英語で書く（`CLAUDE.md`「Language」）。この時点ではコミットを確定しない（ステップ 1-3 の通過後に確定する）。
  - [x] `_checks/escape_untrusted.go` を Go 標準ライブラリのみで作成し、設計書 §3.1「信頼しない文字列の出力時エスケープ」の規則（`\t` を含む `unicode.Cc`・`Cf`・`Zl`・`Zp`・非文字・不正 UTF-8 バイト列の `\xHH` 化、および `\` の `\\` 化；通常の印字可能文字と有効な `U+FFFD` はそのまま通す）を英語コメント・識別子で実装する。`check_open_branches.sh` からは、スクリプト自身のディレクトリ基準で `escape_untrusted.go` のパスを解決し、`GO111MODULE=off GOTOOLCHAIN=local GOENV=off GOFLAGS= GOPROXY=off GOWORK=off`（`Makefile:144` 準拠）で一時ディレクトリへ 1 回 `go build` したバイナリを呼び出す。ビルド失敗時は未ビルドのバイナリを呼ばず、標準出力を空・終了コード `2`・安全な固定診断（標準エラー）でフェイルクローズする。
- **完了条件:**
  - `/bin/bash -n`（bash 3.2）および `bash -n` で `check_open_branches.sh` に構文エラーがなく、上記隔離環境変数付きでの `escape_untrusted.go` の `go vet`・単一ファイル `go build`・`gofumpt` がすべて通ること。

#### ステップ 1-2: ローカル擬似 `origin` による壊し確認スクリプトの作成と実行

- **変更対象ファイル:**
  - `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh`（新規）
- **作業内容:**
  - [x] ネットワークなしのローカル一時リポジトリで `check_open_branches.sh` を自動検証する `_checks/test_check_open_branches.sh` を英語コメント・識別子で作成する。テスト自身が一時作業領域を `mktemp -d`＋`EXIT` トラップで確実に後始末し、検証対象の起動には実行中と同一のシェル `"$BASH"`（`GOCACHE` は維持）を用いる。以下を網羅する。
    - 設計書 §7.3 の全 7 シナリオ: 3 形式・CRLF・孤立ブランチの終了コード `1` 検出、`main` のみ・インデント・`(coderabbit)` の非検出、残留する古い `refs/remotes/origin/*` と未取得の新規ブランチの扱い、外部の `git worktree`（linked worktree）からの実行、存在しない URL・`main` 不在・引数指定の終了コード `2`（標準出力は空）、および呼び出し元リポジトリが変更されないこと。エスケープ検証では、一致行の `\t`・ESC・DEL・C1 制御文字・`Cf`（双方向・ゼロ幅・タグ）・`Zl`/`Zp`・非文字・不正 UTF-8・`\` と、ブランチ名の C1 制御文字・双方向・ゼロ幅（APFS 制約で不正 UTF-8 ブランチ名は除く）、さらに標準エラーの診断エスケープおよび通常の UTF-8・有効な `U+FFFD` が変換されずに通ることを確認する。
    - 終了コード `0` の出力契約（UTC 時刻・40 桁 `origin/main` ID・昇順 12 桁ブランチ一覧・非 `main` 0 本時の `(none)`・`matches: 0`）、終了コード `0`/`1`/`2` いずれでもケース専用 `TMPDIR` が空に戻ること、および設計書 §3.1・§4.2 の安全境界（非 `commit` 参照・`go build` 失敗の終了コード `2`［標準出力は空］、`GIT_DIR`/`GIT_WORK_TREE` 上書き耐性、shallow clone 呼び出し元、ハイフン始まりブランチ名、マージコミット、`replace` 参照無視）。
  - [x] `/bin/bash`（bash 3.2）と `PATH` 上の `bash` の双方で `test_check_open_branches.sh` を実行し、全シナリオが成功することを確認する。
  - [x] 壊し確認の義務: `check_open_branches.sh` および `escape_untrusted.go` に意図的な壊し（3 形式のいずれかの照合を外す、`\t` や制御文字のエスケープを外す、`ls-remote` 集合一致や異常系フェイルクローズを外す等）を入れた際に `test_check_open_branches.sh` が失敗することを確認し、壊しを元に戻す（確認内容はステップ 1-3 通過後のコミットメッセージに記録し、この時点ではコミットしない）。
- **完了条件:**
  - `/bin/bash` および `bash` の双方で `test_check_open_branches.sh` が終了コード `0` で完了すること。

#### ステップ 1-3: 実 `origin` に対する実装開始前の前提確認（AC-01・AC-02・AC-11）

- **変更対象ファイル:**
  - ステップ 1-1・1-2 の `_checks/` 一式（通過時のみコミット）
  - `docs/tasks/0121_retire_legacy_review_records/03_implementation_plan.md`（§7 に実装開始前の確認結果を記録）
- **作業内容:**
  - [x] 実装着手時点の `origin/main` を確認し、`3fff7f51` 以降にコマンド定義・`review-records.sh`・`review_records_test.go`・関連文書（`docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md`）へ新たな変更や旧 3 形式の記述が入っていないか確かめる（AC-11）。
  - [x] 利用者の明示承認を得て、実 `origin` に対して `bash docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh` を単一コマンドとして実行する（設計書 §3.1・§8）。
  - [x] 終了コードが `0` 以外（`1`: 古い記録行を 1 件以上検出、`2`: 取得・読み取り異常）の場合は、該当ブランチ・一致行または診断を報告して作業を停止し、フェーズ 2 以降へ進まない（未コミットの `_checks/` 作業は破棄または保留とし、実装コミット・PR を作成しない。AC-02）。
  - [x] 終了コードが `0`（`matches: 0`）の場合は、出力に対して禁止メンション検査（`<mention-check>`）を実行し、終了コード `1`（問題なし）を確認する。そのうえで、確認日時（UTC）・基準 `origin/main` の 40 桁コミット ID・検査ブランチ一覧・`matches: 0` をコードブロック形式で本計画書 §7 の「実装開始前の確認記録（AC-01）」へ記録し、`_checks/` 一式と本計画書をステップ 1-2 の壊し確認記録とともにコミットする（AC-01、設計書 §5.2・§8）。
- **完了条件:**
  - 実 `origin` に対する `check_open_branches.sh` が終了コード `0`（`matches: 0`）で完了し、`<mention-check>` を通過した記録が本計画書 §7 に追記され、`_checks/` 一式とともにコミットされていること。

### フェーズ 2: コマンド定義の撤去と `review-records.sh` の厳格化（F-002・F-003）

#### ステップ 2-1: コマンド定義 3 ファイルからの古い記録行の読み方規則の撤去

- **変更対象ファイル:**
  - `.claude/commands/_lib/local-cli-review.md`
  - `.claude/commands/_lib/task-pr-handoff.md`
  - `.claude/commands/_lib/cross-check-review.md`
- **作業内容:**
  - [x] `.claude/commands/_lib/local-cli-review.md` の `## Reading the record lines`（`3fff7f51` 時点 `:293-326`）を設計書 §3.2 に従って更新する（AC-03・AC-06）。
    - `:307` を `- Every other \`CodeRabbit local review:\` and \`Codex local review:\` line is a` から `- Every other \`CodeRabbit local review:\` line is a` へ変更する（`:308` は変更しない）。
    - `:314` を `- A \`Local CLI reviewed: <START12>..<END12> (<cli>)\` line names a seen range.` から `- A \`Local CLI reviewed: <START12>..<END12> (coderabbit)\` line names a seen range.` へ変更する。
    - `:323-326` の段落（`Read earlier \`(codex)\` seen ranges and \`(coderabbit: <kind>; codex: <kind>)\` ...` から `contains an old record.` まで）を削除する。範囲なし `CodeRabbit local review: not run`（`:304-306`）等は維持する。
  - [x] `.claude/commands/_lib/task-pr-handoff.md` の `Part B step 3`（`3fff7f51` 時点 `:320-331`）を設計書 §3.2・§3.4 に従って更新する（AC-04・AC-06）。
    - `:324-325` を `` (`git log origin/main..<D>`; the `CodeRabbit local review:` and `Codex local review:` lines, read as `_lib/local-cli-review.md`, `` から `` (`git log origin/main..<D>`; the `CodeRabbit local review:` lines, read as `_lib/local-cli-review.md`, `` へ変更する。
    - `:329-331` を `Describe a Codex CLI or a cross-check finding only by its location and your own summary, as the record line does, never with Codex's or the cross-check worker's text;` から `Describe a cross-check finding only by its location and your own summary, as the record line does, never with the cross-check worker's text;` へ変更する。`Cross-check finding:`・`Fable finding:` の読み取り、本文転記禁止、`**Mention check.**`、`review-records.sh` 呼び出しブロックは維持する。
  - [x] `.claude/commands/_lib/cross-check-review.md` の `## The prompt` — `Settled issues`（`3fff7f51` 時点 `:543-545`）を設計書 §3.2・§3.4 に従って更新する（AC-05・AC-06）。
    - `:543-545` を `` - the rejected findings of `CodeRabbit local review:`, `Codex local review:` (legacy lines from older branches), `Cross-check finding:`, and legacy `Fable finding:` lines in `git log origin/main..HEAD`; `` から `` - the rejected findings of `CodeRabbit local review:`, `Cross-check finding:`, and legacy `Fable finding:` lines in `git log origin/main..HEAD`; `` へ変更する。
- **完了条件:**
  - 3 ファイルの変更差分が上記の撤去と文言調整のみに収まっていること。

#### ステップ 2-2: `review-records.sh` の `SEEN_RE` 厳格化とヘッダコメントの整合

- **変更対象ファイル:**
  - `.claude/commands/_lib/review-records.sh`
- **作業内容:**
  - [x] `:269` の `SEEN_RE` を `SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((coderabbit|codex)\)$'` から `SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(coderabbit\)$'` へ変更する（設計書 §3.3、AC-07〜AC-09）。
  - [x] `:79` のヘッダコメントを `# Mode unreviewed: the commits of a PR neither local CLI reviewed, for the PR` から `# Mode unreviewed: the commits of a PR the local CLI review did not review, for the PR` へ変更する（設計書 §3.3）。
  - [x] `NOT_RUN_RE`（`:271`）、`checked_range`、クリーン環境再実行、git plumbing 読み取り、`fail` 契約に変更がないことを確認する。
- **完了条件:**
  - `/bin/bash -n .claude/commands/_lib/review-records.sh` および `bash -n .claude/commands/_lib/review-records.sh` が成功し、`SEEN_RE` のみが `(coderabbit)` に厳格化されていること（`make test` は密結合するステップ 3-1 と合わせて通す）。

### フェーズ 3: テスト更新・関連文書確認・PR 作成（F-004、`/runplan` 完了点）

#### ステップ 3-1: `cmd/yt2column/review_records_test.go` の既存ケース書き換えと拒否・混在ケースの追加

- **変更対象ファイル:**
  - `cmd/yt2column/review_records_test.go`
- **作業内容:**
  - [x] 既存の 4 か所を設計書 §1.3 決定 1・§3.4・§7.1 に従って書き換える（テストの削除は行わない。Go コード・コメント・文字列リテラルはすべて英語で書く。AC-10）。
    - `coverageCases()` の `"seen by codex"`（`:355-359`）のケース名を実態に合わせ（例: `"codex reviewer"`）、期待値を `f.partial(f.rev("HEAD~1"))`・終了コード `0` へ変更する（`TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed` の双方で実行）。
    - `unreviewedCases()` の `"fix seen by the standalone path's review"`（`:904`）の `f.seen(r, fix, "codex")` を `f.seen(r, fix, "coderabbit")` へ更新する。
    - `coverageCases()` の `not run` 系 2 ケース（`:425,431`）の旧入力 `"(coderabbit: other; codex: timeout)"` を現行の CodeRabbit 単体形式 `"(coderabbit: other)"` へ統一する。
    - `"HEAD behind LAST_REVIEWED"` のコメント（`:381-382`）を `// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,` / `// yet what Codex reviewed is not what is merged.` から `// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,` / `// yet what the PR review saw is not what is merged.` へ変更する。
  - [x] `rrFixture.seen`（`:184-186`）のシグネチャを維持したまま、設計書 §7.1 の新規ケースを追加する（AC-07〜AC-09）。
    - `coverageCases()`（`TestReviewRecordsUnreviewed` でも自動実行）に、大文字小文字違い・括弧内の空白混入や行末の余分な空白の拒否ケース、同一コミット・同一ブランチ上の `(codex)` と有効な `(coderabbit)` の混在（`ALL`）、および部分重複の混在ケース（有効な `(coderabbit)` の範囲に一部コミットが入り、別のコミットが `(codex)` の範囲にしか入らないときに、後者のコミットが `PARTIAL 1` で列挙されること）を追加する。
    - `crossCheckCases()`（`TestReviewRecordsCrossCheck`）に、ルール 2 の `(codex)` 単独拒否（`MISSING`）、ルール 2 の `(codex)` と有効な `(coderabbit)` の混在（`VALID`）、ルール 2 の `(codex)` と `Local CLI review: not run` の混在（`VALID`）の**双方**、ルール 4 の `(codex)` 単独拒否（`MISSING`）、およびルール 4 の `(codex)` と有効な `(coderabbit)` の混在（`VALID`）を追加する。
    - `TestReviewRecordsFableAlias`（`:2047`）に、`fable` 互換別名でルール 2 またはルール 4 の `(codex)` 単独が `MISSING` として拒否される代表ケースを追加する（設計書 §3.4）。
  - [x] 壊し確認の義務: `review-records.sh` の `SEEN_RE` を一時的に `(coderabbit|codex)` へ戻した状態で `go test -tags test ./cmd/yt2column -run 'TestReviewRecords(Coverage|Unreviewed|CrossCheck|FableAlias)'` を実行し、書き換え・追加した `(codex)` 拒否ケース（`PARTIAL`・`MISSING` 期待）がいずれも失敗することを確認してコミットメッセージに記録する（確認後に元に戻す）。
- **完了条件:**
  - `go test -tags test -v ./cmd/yt2column -run TestReviewRecords` が全 bash 環境（bash 3.2・bash 4+）で成功すること。

#### ステップ 3-2: 関連文書の再確認・差分記録・品質関門の実行

- **変更対象ファイル:**
  - `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md`（新たな該当箇所が見つかった場合のみ変更）
  - `docs/tasks/0121_retire_legacy_review_records/03_implementation_plan.md`（AC-06 の diff 記録）
- **作業内容:**
  - [x] `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を再度確認し、撤去対象の 3 形式を現行の読み方規則として記述した箇所が追加されていれば更新する（AC-11）。
  - [x] `docs/dev/conventions/requirements_process.md` §4 の固定コマンドを用いて、本計画書 §7 の `Diff record (AC-06)` に `.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/cross-check-review.md` の差分を記録する（本計画書は比較対象に含めない）。
  - [x] 本計画書 §7 にステップ 1-3 の実装開始前確認記録（`<mention-check>` 通過後のコードブロック）がコミット済みであることを確認し、`PR-1 作成ポイント` で開く PR の説明へそのまま転記できる状態にする。
  - [x] `make fmt`・`make test`・`make lint`・`make deadcode` および CI の `doc-guard` と同一のコマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）を実行し、すべて成功することを確認する（AC-12）。テスト削除が発生していないため `make coverage-compare` は非該当とする（万一削除が生じた場合のみ [ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) に従い削除専用コミットを分けて実行・記録する）。
- **完了条件:**
  - 品質関門の全コマンド、`Diff record (AC-06)` の記録、およびステップ 1-3 の実装開始前確認記録のコミットが完了しており、`PR-1 作成ポイント` へ進める状態であること。

### PR-1 作成ポイント: retire legacy review-record rules and add migration gate

**対象ステップ**: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 3-1 / 3-2

**推奨タイトル**: `refactor(0121): 古い記録行の読み方規則を撤去し移行ゲートと回帰テストを整備`

**レビュー観点**: `_checks/check_open_branches.sh`・`escape_untrusted.go` の隔離取得・フェイルクローズ・出力エスケープと `test_check_open_branches.sh` の網羅性、および PR の説明に転記した実装開始前の `matches: 0` 確認記録 / コマンド定義 3 ファイルの差分が `Diff record (AC-06)` 上で撤去・整合のみに限られること / `review-records.sh` の `SEEN_RE` 厳格化と全モード（`coverage`・`unreviewed`・`cross-check`・`fable`）の回帰テスト / マージ承認直前のマージ前ゲート（`_checks/check_open_branches.sh`）再実行、最終 push 以降の確認日時と最新 `origin/main` に一致する基準コミット ID を含む `matches: 0` 結果の PR の説明への記録、および非ゼロ終了時のマージ停止（設計書 §1.3 決定 5・§8）

**実装モデル要件**: high

**判定理由**: ステップ 1-1〜1-3 とステップ 2-2 が移行ゲートおよびレビューの関門（信頼の境界）の変更（`mkplan.md` step 8 のパネルモード条件・`mkplan2.md` step 4 の `high` 判定条件に該当）であり、上位モデルより軽いモデルでは bash 3.2 隔離・一時 bare 取得・`\t` を含む信頼しない文字エスケープの抜け漏れや `SEEN_RE` の境界誤りでフェイルクローズや関門の厳格化を破る恐れがあるため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

フェーズ 1〜3 は 1 本の実装 PR としてまとめ、ステップ 2-2 とステップ 3-1 の密結合変更を同一グリーンゲートで検証する。

1. **マイルストーン 1（フェーズ 1 完了 — 移行ゲート通過）:** `_checks/check_open_branches.sh`・`_checks/escape_untrusted.go`・`_checks/test_check_open_branches.sh` が完成して全シナリオと壊し確認に通り、実 `origin` で `matches: 0` が確認・記録されたうえで `_checks/` がコミットされている（成果物: `_checks/` 一式、AC-01 実装開始前の記録）。
2. **マイルストーン 2（フェーズ 2 完了 — 規則撤去と厳格化）:** コマンド定義 3 ファイルと `review-records.sh` の更新が完了し、差分が撤去・整合のみに収まっている（成果物: `.claude/commands/_lib/` 配下 4 ファイルの更新）。
3. **マイルストーン 3（フェーズ 3 完了 — 実装 PR 作成）:** `review_records_test.go` の書き換え・新規ケース追加、関連文書確認、`Diff record (AC-06)` の記録、および `make test`・`make lint`・`make deadcode`・`doc-guard` 通過が完了し、実装 PR が開かれている（成果物: 実装 PR）。
4. **マイルストーン 4（マージ前ゲート完了 — `/runplan` 外の手動ゲート）:** PR レビュー・CI 通過後、`/runpr` のマージ承認直前に実 `origin` に対して `_checks/check_open_branches.sh` を再実行し、`matches: 0` の結果（最終 push 以降の確認日時・最新 `origin/main` と一致する基準コミット ID）を `<mention-check>` 通過後に PR の説明へ記録したうえでマージする（設計書 §8「マージ前ゲート」）。

### 3.2 PR 構成

承認済みの [02_architecture.md](02_architecture.md) §8 に従い、フェーズ 1〜3 を 1 本の実装 PR（PR-1）にまとめる。ステップ 2-2 とステップ 3-1 は `make test` 上で密結合しており分離できない。また、フェーズ 1 の `_checks/` 3 ファイルを別 PR に切り出す案も考えられるが、`_checks/` は本タスクの 1 回限りの移行ゲートであり、PR-1 レビュー中に `check_open_branches.sh` の不具合が見つかっても修正後にマージ前ゲートを再実行してから `main` へマージするため、設計書 §8 の 1 PR 構成を維持する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 3-1 / 3-2 | `_checks/` 移行ゲートと検証スクリプトの作成・実 `origin` 前提確認、コマンド定義 3 ファイルの旧規則撤去、`review-records.sh` の `SEEN_RE` 厳格化、`review_records_test.go` の書き換え・回帰ケース追加、および関連文書・差分記録の確認 | high | auto |

## 4. テスト戦略 (Test Strategy)

- **ユニットテスト（`cmd/yt2column/review_records_test.go`）:**
  - `TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` により、リポジトリ上の全 bash 環境（`scriptBashes`: macOS の bash 3.2 および bash 4+）で `review-records.sh` の実動作を検証する（設計書 §7.1）。
  - 既存ケースを書き換え、`(codex)`・大文字小文字違い・余分な空白の拒否、有効な `(coderabbit)` 行（部分重複を含む）やルール 2 の `Local CLI review: not run` 行との混在、およびルール 4（マージ後確認の修正）の拒否・混在を網羅する。テスト削除を行わないため `make coverage-compare` は非該当。
  - 既存の `rrFixture` / `rrFixture.seen` を再利用するため、新規の `testutil/` や `test_helpers*.go` は作成しない（`docs/dev/conventions/test_organization.md`）。
- **移行ゲートのローカル結合検証（`_checks/test_check_open_branches.sh`）:**
  - ネットワークなしのローカル一時 bare リポジトリと linked worktree に対し、`/bin/bash`（bash 3.2）と `PATH` の `bash`（スクリプト内から `"$BASH"` で起動）で検証する。対象は設計書 §7.3 の全 7 シナリオに加え、出力契約（`(none)` 境界を含む）・`TMPDIR` の後始末・非 `commit` 参照・`go build` 失敗・`GIT_DIR` 上書き・shallow clone・ハイフン始まりブランチ名・マージコミット・`replace` 参照無視とする。タスク専用スクリプトのため `make test` と CI には含めない。
- **文書・差分・品質関門:**
  - AC-06 は `requirements_process.md` §4 の記録した diff の読み取り（`Diff record (AC-06)`）で検証し、`/runplan` の PR 作成前と `/runpr` のマージ前に機械照合する。
  - AC-03〜AC-05・AC-11 は文書の自然言語仕様・内容であるため、`CLAUDE.md`（Document tests check values, structure, and executable examples, never prose）に従い文書テストや `rg` チェックは追加せず PR レビューの通読で確認する。
  - 既存の文書ガードテスト群および `make test`・`make lint`・`make deadcode`・CI `doc-guard` の全通過を必須とする（AC-12）。
- **後方互換性テスト:**
  - 本タスクはタスク 0118 の移行期後方互換（AC-28）を意図的に退役させる変更であり（`01_requirements.md` §5）、`(codex)` 行が安全側（未見: `PARTIAL` / `MISSING`、終了コード `0`）へ倒れることと、同一コミット・ブランチ上の現行 `(coderabbit)` 行・`NOT_RUN_RE` による判定が壊れないことを回帰テストで確認する。

## 5. リスク管理 (Risk Management)

- **技術リスク:**
  - **開いたブランチへの古い記録行の残存:** 実装開始前またはマージ前に `origin` の対象ブランチに古い記録行が残っていると、マージ後の `review-records.sh` でそのブランチのコミットが未見に変わる。対策として、ステップ 1-3 とマージ前ゲートの 2 時点で `_checks/check_open_branches.sh` を実行し、終了コード `0` 以外は即時停止する（AC-01・AC-02）。
  - **マージ前ゲートから squash マージまでの時間差（TOCTOU）と形式 1 の残余リスク:** 確認直後からマージ完了までの間に古い記録行を持つブランチが push された場合、形式 2・3（`(codex)` および `(coderabbit: <kind>; codex: <kind>)`、設計書 §3.1）は安全側へ倒れる一方、形式 1（`Codex local review:`）は指摘が PR の説明へ載らなくなる。この残余リスクは設計書 §1.3 決定 4・§5.2 で受容済みであり、マージ承認直前の再実行で時間差を最小化する。
  - **`check_open_branches.sh` の環境差（bash 3.2・大文字小文字非区別 FS・shallow / partial clone・信頼しない文字）:** `/bin/bash`（3.2）でのテスト実行、`ls-remote --heads` とのブランチ集合一致検証、非 shallow・非 partial clone 時のみの `alternates` 利用、および `_checks/escape_untrusted.go` による出力エスケープと `<mention-check>`・コードブロック囲みで緩和する（設計書 §3.1・§5.2）。
  - **並行タスク（例: タスク 0122）によるコマンド定義・テストの変更:** ステップ 1-3 の着手時に最新の `origin/main` を確認し、行位置やテスト構成が変わっていれば現行に合わせて適用する。
- **スケジュール・進行リスク:**
  - **ステップ 1-3 またはマージ前ゲートでの停止:** 他ブランチに古い記録行が残っていた場合、該当 PR のマージまたはクローズを待つ必要がある。フェーズ 1 の段階で判定を先に行い、フェーズ 2・3 の手戻りを防ぐ。
  - **マージ前ゲートのやり直し:** マージ承認前に追加 push または `origin/main` の更新があった場合は、設計書 §8 のとおりマージ前ゲートを再実行し、PR の説明の記録を更新してからマージする（コミット・push は増やさない）。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] マージ前ゲート完了（PR-1 のレビュー・CI 通過後、マージ承認直前に実 `origin` へ `_checks/check_open_branches.sh` を再実行し、最新の `matches: 0` 結果を `<mention-check>` 通過後に PR-1 の説明へ記録）
- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 3-1 / 3-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC ID | 検証種別 | 検証対象・方法 | 対応ステップ |
|---|---|---|---|
| AC-01 | `test`・`static`・`manual` | `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh` で終了コード `0` の出力契約（UTC 時刻・40 桁 `origin/main` ID・昇順ブランチ一覧・`(none)` 境界・`matches: 0`）を検証（`test`）。`docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh` を実装開始前とマージ承認直前の 2 時点で実 `origin` に対して実行し、終了コード `0`・`matches: 0` を確認（`static`）。2 時点それぞれの記録が `<mention-check>` 通過後にコードブロックで計画書または PR の説明（マージ前は PR の説明）へ残っていることを PR レビューで確認（`manual`） | ステップ 1-1・1-2・1-3・3-2（およびマージ前ゲート） |
| AC-02 | `test`・`manual` | `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh`（擬似 `origin` 上の 3 形式・CRLF・孤立ブランチで終了コード `1` と該当ブランチ・一致行出力、存在しない URL・`main` 不在・非 `commit` 参照・`go build` 失敗・引数指定等で終了コード `2`・空の標準出力、ケース専用 `TMPDIR` の後始末、呼び出し元無変更、標準出力・標準エラーの出力エスケープを検証）（`test`）。実 `origin` 実行時に非ゼロ終了であれば実装開始・マージを停止して報告する手順の遵守（`manual`） | ステップ 1-1・1-2・1-3（およびマージ前ゲート） |
| AC-03 | `manual` | PR レビューで `.claude/commands/_lib/local-cli-review.md` の `## Reading the record lines` 節を通読し、指摘の行の接頭辞が `CodeRabbit local review:` のみ・見た範囲の行の形が `Local CLI reviewed: <START12>..<END12> (coderabbit)` のみであり、旧 3 形式の読み取り記述と退役条件の段落を持たないことを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-04 | `manual` | PR レビューで `.claude/commands/_lib/task-pr-handoff.md` の Part B step 3 を通読し、`Codex local review:` および Codex CLI 指摘の記述が外れ、`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` の読み取りと本文転記禁止の規則が維持されていることを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-05 | `manual` | PR レビューで `.claude/commands/_lib/cross-check-review.md` の `## The prompt` — `Settled issues` を通読し、`Codex local review:` の記述が外れ、`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` の却下済み指摘の読み取りが維持されていることを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-06 | `diff` | 記録した diff の読み取り（下の `Diff record (AC-06)`）。比較対象パスは `.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/cross-check-review.md`（本計画書は含めない）。`/runplan` が `requirements_process.md` §4 の固定コマンドで記録し、PR 作成前とマージ前に機械照合する | ステップ 2-1・3-2 |
| AC-07 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（有効な `(coderabbit)` 行が `START`/`END` 検証を通過するとき `ALL`・終了コード `0` となること、単独経路の修正ケースを含む） | ステップ 2-2・3-1 |
| AC-08 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（`(codex)`・大文字小文字違い・余分な空白・別レビュアー名が無視され `PARTIAL <n>`・終了コード `0` となること、および同一コミット・同一ブランチ上［部分重複を含む］の有効な `(coderabbit)` 行に影響しないこと） | ステップ 2-2・3-1 |
| AC-09 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCrossCheck`・`cmd/yt2column/review_records_test.go::TestReviewRecordsFableAlias`（ルール 2・ルール 4 で `(codex)` 行単独は受理されず `MISSING <START>`・終了コード `0` となること、および同一コミット上の有効な `(coderabbit)` 行とルール 2 の `Local CLI review: not run` 行の双方の判定に影響せず `VALID` となること） | ステップ 2-2・3-1 |
| AC-10 | `test`・`manual` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（書き換えた `(codex)` 拒否・`unreviewedCases()` の `(coderabbit)` 受理・`not run` ケースが通ること）（`test`）。PR レビューで旧形式の `not run` 入力文字列および Codex 前提コメントが残っていないことを確認（`manual`） | ステップ 3-1 |
| AC-11 | `manual` | 実装開始時および完了時に `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を確認し、撤去対象の 3 形式を現行の読み方規則として記述した箇所が存在しないこと（該当があれば更新済みであること）を PR レビューで確認（自然言語内容のみの AC。`3fff7f51` 時点の事前確認では該当 0 件） | ステップ 1-3・3-2 |
| AC-12 | `test`・`static`・`manual` | `make test`・`make lint`・`make deadcode` および `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`（`test`・`static`）。CI の `doc-guard` ジョブ通過を PR 上で確認（`manual`）。テスト削除は発生しないため `make coverage-compare` は非該当（万一発生時のみ削除専用コミットで実行・記録） | ステップ 3-2 |

Diff record (AC-06):

- Compared paths: `.claude/commands/_lib/local-cli-review.md`, `.claude/commands/_lib/task-pr-handoff.md`, `.claude/commands/_lib/cross-check-review.md`
- Merge-base: `036fd7c83a885a7cd7ffbc164b10c6efeac0cd5c`
- Compared commit: `b0a5f35917df0efb9887af3878eeabf294e76984`

Path diff:

````text
diff --git a/.claude/commands/_lib/cross-check-review.md b/.claude/commands/_lib/cross-check-review.md
index d42ec3adcb0db1601831f9ec022ca8aaf72db594..ed574bf19a69ca8e8193a170049e3894320dca81 100644
--- a/.claude/commands/_lib/cross-check-review.md
+++ b/.claude/commands/_lib/cross-check-review.md
@@ -540,9 +540,8 @@ part is a summary:
 - **Settled issues.** Each as a location, a handling, and a summary in your
   own words, never a copied line. The worker may raise one again only with a
   concrete scenario:
-  - the rejected findings of `CodeRabbit local review:`, `Codex local
-    review:` (legacy lines from older branches), `Cross-check finding:`, and
-    legacy `Fable finding:` lines in `git log origin/main..HEAD`;
+  - the rejected findings of `CodeRabbit local review:`, `Cross-check finding:`,
+    and legacy `Fable finding:` lines in `git log origin/main..HEAD`;
   - the `High review: kept` and legacy `Opus review: kept` lines of the same
     log;
   - SELF_REJECTED;
diff --git a/.claude/commands/_lib/local-cli-review.md b/.claude/commands/_lib/local-cli-review.md
index 4355966c2fb786b2e23d82f735f7e8bf80d709e4..723dd519fc4d88c1ddec838f60f198e3f12de6ac 100644
--- a/.claude/commands/_lib/local-cli-review.md
+++ b/.claude/commands/_lib/local-cli-review.md
@@ -304,14 +304,14 @@ of these prefixes.
 - A line that is exactly `CodeRabbit local review: not run`, which an earlier
   session wrote when the CodeRabbit CLI failed and has no range, also means an unreviewed
   range; take its range as the whole branch. It is not a finding line.
-- Every other `CodeRabbit local review:` and `Codex local review:` line is a
+- Every other `CodeRabbit local review:` line is a
   finding line; its handling (rejected, `deferred`, `unclear`) is in the line.
 - A `Cross-check review:`, `Cross-check scope:`, `Cross-check finding:`
   (or a legacy `Fable review:`, `Fable scope:`, or `Fable finding:`),
   `Merge check:`, `Merge check scope:`, or `Merge check finding:` line is not a
   local CLI review line; `_lib/cross-check-review.md`, "Reading the record lines",
   reads it.
-- A `Local CLI reviewed: <START12>..<END12> (<cli>)` line names a seen range.
+- A `Local CLI reviewed: <START12>..<END12> (coderabbit)` line names a seen range.
   It is neither a finding line nor an unreviewed range, and it does not
   cancel a `not run` line. `runpr.md` step 3 reads it through
   `.claude/commands/_lib/review-records.sh`, which decides whether the local
@@ -319,8 +319,3 @@ of these prefixes.
   the branch before a PR opens); that script is
   the executable form of this line's reading, and when they disagree, this
   file is right and the script is fixed.
-
-Read earlier `(codex)` seen ranges and `(coderabbit: <kind>; codex: <kind>)`
-failures with the same range semantics. These forms and the
-`Codex local review:` finding prefix may be retired once no open branch
-contains an old record.
diff --git a/.claude/commands/_lib/task-pr-handoff.md b/.claude/commands/_lib/task-pr-handoff.md
index c895be9b52541bc0e6502f4e93cc07a922e35735..55017d77a49baf9aba777dcbf68166f193fc9298 100644
--- a/.claude/commands/_lib/task-pr-handoff.md
+++ b/.claude/commands/_lib/task-pr-handoff.md
@@ -321,13 +321,13 @@ Run this after the calling command's last commit, with a clean working tree.
      `レビュー観点`), including every `unclear` or `deferred` finding of the
      local CLI reviews, including earlier records: this run's SUMMARY, and the finding
      lines earlier sessions recorded in commit messages on the branch
-     (`git log origin/main..<D>`; the `CodeRabbit local review:` and
-     `Codex local review:` lines, read as `_lib/local-cli-review.md`,
+     (`git log origin/main..<D>`; the `CodeRabbit local review:` lines, read as
+     `_lib/local-cli-review.md`,
      "Reading the record lines", says; a `not run` line is not a finding),
      and every `unclear` or `deferred` cross-check finding (its
      `Cross-check finding:` line, or a legacy `Fable finding:` line, read as `_lib/cross-check-review.md`, "Reading the record lines", says).
-     Describe a Codex CLI or a cross-check finding only by its location and
-     your own summary, as the record line does, never with Codex's or the
+     Describe a cross-check finding only by its location and
+     your own summary, as the record line does, never with the
      cross-check worker's text;
    - names the commits of this PR that the local CLI did not review, if any,
      by running `origin/main`'s review records script in `unreviewed` mode
````

実装開始前の確認記録（AC-01）:

- ステップ 1-3 で実 `origin` に対して `_checks/check_open_branches.sh` を実行し、終了コード `0`（`matches: 0`）かつ `<mention-check>` 通過（終了コード `1`）を確認した。

```text
checked_at: 2026-11-24T10:57:46Z
origin/main: 036fd7c83a885a7cd7ffbc164b10c6efeac0cd5c
branches:
  claude/festive-bardeen-cw14ci 4da5da2194a0
  isseis/0092-chief-task-workspace-07 b56048d497e4
  isseis/0115-chief-conflict-resolution-05 a3fde9ac36c2
  isseis/0116-user-readme-05 3bb5b316ff23
  isseis/0120-harness-independent-commands-08 bd4fe1a90375
  isseis/0121-retire-legacy-review-records-04 036fd7c83a88
  isseis/0122-sourcery-trusted-reviewer-04 ae7f242967f9
  isseis/0123-rereview-substituted-reviews-01 525a0834b411
  poc/0084-wasm-pipeline 83ef12bd019b
  poc/0090-extension-only-checks da994ad38130
matches: 0
```

## 8. 完了基準 (Success Criteria)

- **機能網羅:** F-001〜F-004 および AC-01〜AC-12 のすべての実装タスクと検証項目が完了していること。
- **品質:** `make test`・`make lint`・`make deadcode`・CI の `doc-guard` ジョブおよび `_checks/test_check_open_branches.sh`（bash 3.2・bash 4+）がすべて成功し、`review-records.sh` と `check_open_branches.sh` に対する壊し確認の結果がコミットメッセージに記録されていること。
- **セキュリティ:** `(codex)` および不正表記の `Local CLI reviewed:` 行が全モードで無視されること、`check_open_branches.sh` がフェイルクローズと出力エスケープを満たすこと、および PR の説明へ記録する F-001 結果が `<mention-check>` を通過してコードブロックに収まっていること。
- **文書:** コマンド定義 3 ファイルの変更が `Diff record (AC-06)` で撤去・整合のみに限られること、AC-11 の対象文書に現行読み方規則としての旧 3 形式が残っていないこと、および AC-01 の 2 時点（実装開始前・マージ前）の記録がそろっていること。

## 9. 次のステップ (Next Steps)

1. 本実装計画書（`draft`）の PR レビューを受け、`/runpr` のマージの質問で承認（`approved`）およびマージする。
2. 承認後、`/runplan 0121` でフェーズ 1〜3 を実施し、実装 PR を作成する（フェーズ 1 の実 `origin` 確認で非ゼロ終了となった場合は実装へ進まず停止する）。
3. 実装 PR のレビューと CI が通過したら、`/runpr` のマージの質問を承認する**直前**にマージ前ゲート（`_checks/check_open_branches.sh`）を実 `origin` に対して再実行する。非ゼロ終了となった場合はマージせず停止して該当ブランチ・一致行または診断を報告し、終了コード `0`（`matches: 0`）であれば結果を `<mention-check>` 通過後に PR の説明へ記録したうえでマージする（設計書 §8「マージ前ゲート」）。

## 変更履歴

- 2026-11-24 承認（isseis、#1120）
