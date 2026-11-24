# 実装計画書：古い記録行の読み方規則を、対象ブランチがなくなった後に外す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-24 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的

本書は、[01_requirements.md](01_requirements.md)（F-001〜F-004、AC-01〜AC-12）および承認済みの [02_architecture.md](02_architecture.md) に基づき、タスク 0118 が移行期の後方互換として残した古い記録行 3 形式（`Codex local review:`・`(codex)`・`(coderabbit: <kind>; codex: <kind>)`）の読み方規則を、`origin` に該当ブランチがないことを確認したうえで安全に撤去するための実装計画を定める。

### 1.2. 実装方針

本タスクは Go の製品コード（`internal/`・`cmd/yt2column/*.go` の非テストファイル）を変更せず、タスク専用の移行ゲートスクリプト、共有コマンド定義 3 ファイル、判定スクリプト `review-records.sh`、および回帰テスト `review_records_test.go` を更新する。設計の詳細は [02_architecture.md](02_architecture.md) §1〜§8 に従い、次の方針で進める。

- **移行ゲート優先:** 実装開始前に `_checks/check_open_branches.sh` を整備・検証したうえで実 `origin` に対して実行し、終了コード `0`（`matches: 0`）を確認できない限りフェーズ 2 以降へ進まない（F-001）。
- **テスト非削除移行:** `cmd/yt2column/review_records_test.go` の既存 `(codex)` 受理ケースを削除せず拒否ケースへ書き換えることで、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) の削除専用コミットと `make coverage-compare` を不要に保ち、スクリプト変更と一貫して検証する（設計書 §1.3 決定 1）。
- **単一の正本と単一の正規表現:** 記録行の正本を `.claude/commands/_lib/local-cli-review.md`、見た範囲の正規表現を `.claude/commands/_lib/review-records.sh` の `SEEN_RE` の 1 定義に保ち、全モード（`coverage`・`unreviewed`・`cross-check`・互換別名 `fable`）へ一括適用する（設計書 §1.1・§3.3）。
- **新規テスト補助ファイル不設:** 既存の `rrFixture` および `rrFixture.seen` をそのまま再利用するため、`testutil/` や `test_helpers*.go` は新設しない（`docs/dev/conventions/test_organization.md`）。

### 1.3. 既存コード調査結果

作成時点の `origin/main`（コミット `3fff7f51b6bf51721f1bbbe09e87ec2e4ea9d67d`）を調査し、設計書 §1・§3.4 の基準から進んだ変更と各変更対象の位置を確定した。

- **タスク 0120 のマージ状況:** コミット `3fff7f51`（PR #1115）でタスク 0120 が `origin/main` にマージ済みである。したがって、設計書 §3.4「タスク 0120 先行マージ時の差し替え対応表」の全 4 行をそのまま適用する（`.claude/commands/_lib/fable-review.md` は `.claude/commands/_lib/cross-check-review.md` へ改名済み、`review-records.sh` は `cross-check` モードと互換別名 `fable` を持ち、`review_records_test.go` は `crossCheckCases`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` を持つ）。
- **コマンド定義 3 ファイルの対象位置（コミット `3fff7f51`）:**
  - `.claude/commands/_lib/local-cli-review.md`: `## Reading the record lines` は `:293-326`。撤去対象は `:307-308` の `and \`Codex local review:\``、`:314` の `(<cli>)`、および `:323-326` の旧形式読み取り・退役条件段落の 3 か所のみ。範囲なし `CodeRabbit local review: not run`（`:304-306`）等の維持対象はそのまま残す。
  - `.claude/commands/_lib/task-pr-handoff.md`: `Part B step 3` の指摘転記項目は `:320-331`。撤去対象は `:324-325` の `and \`Codex local review:\`` と `:329-331` の Codex CLI および Codex の言及のみ。`Cross-check finding:`・`Fable finding:` の読み取り、本文転記禁止、`**Mention check.**`（`:276`）、`review-records.sh` の呼び出しブロック（`:228-232,340-344`）は不変。
  - `.claude/commands/_lib/cross-check-review.md`: `## The prompt` の `Settled issues` は `:540-550`。撤去対象は `:543-544` の `` `Codex local review:` (legacy lines from older branches), `` のみ。`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` は維持する。
  - 3 ファイル全体の変更見込みは十数行の小さな差分にとどまるため、AC-06 は設計書 §7.2 のとおり記録した diff の読み取りとする。
- **`review-records.sh` の対象位置（コミット `3fff7f51`）:**
  - `SEEN_RE` の定義は `:269` の 1 か所、参照は `judge_seen`（`:832`、`coverage`・`unreviewed` 共通）、`cross_check` ルール 2（`:927`）、`cross_check` ルール 4（`:982`）の 3 か所のみで、第 3 キャプチャグループは参照されていない。
  - `NOT_RUN_RE` は `:271` であり変更しない。ヘッダの `unreviewed` モード説明コメントは `:79` にある。
- **`cmd/yt2column/review_records_test.go` の対象位置と再利用（コミット `3fff7f51`）:**
  - 既存の `seenRangeForm`（`:83-86`）、`rrFixture`（`:91-98`）、`rrFixture.seen`（`:184-186`）、`readCrossCheckForms`（`:1011-1028`）をそのまま再利用する。
  - 書き換えが必要な既存箇所は 4 か所: `coverageCases()` の `"seen by codex"`（`:355-359`）、`"HEAD behind LAST_REVIEWED"` のコメント（`:381-382`）、`not run` 系 2 ケースの入力（`:425,431`）、および `unreviewedCases()` の `"fix seen by the standalone path's review"`（`:899-906`、`:904` が `"codex"`）。
  - 新規追加先は `coverageCases()`（`TestReviewRecordsCoverage` `:845` と `TestReviewRecordsUnreviewed` `:946` の双方で実行）、`crossCheckCases()`（`TestReviewRecordsCrossCheck` `:1865`）、および `TestReviewRecordsFableAlias`（`:2047`）。
- **既存文書ガードテストへの影響（コミット `3fff7f51`）:**
  - `cmd/yt2column/review_records_test.go` の `TestReviewRecordsCallSite`（`:2107-2139`）・`TestRecordLinePrefixesDistinct`（`:2196-2251`）・`TestRecordCommitsCheckMention`（`:2275-2301`）、`cmd/yt2column/fixpr_docs_test.go` の `TestContextAndCommandsAgree`（`:1122-1150`）・`TestLocalReviewGuard`（`:1297-1319`）、`cmd/yt2column/review_mention_docs_test.go`（`:60-137`）、`cmd/yt2column/cross_check_state_docs_test.go`（`:48-152`）、および `internal/ciskip/git_test.go`（`:296`）はいずれも今回の編集箇所の前提を変えないため、更新不要である。
- **関連文書（AC-11）の事前確認（コミット `3fff7f51`）:**
  - `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を対象に `rg -n -i "codex" docs/dev/guides/ docs/dev/conventions/ CLAUDE.md | grep -v -i "@codex"` を実行し、出力が空（0 件）であることを確認した（`@codex` メンション拒否の記述は `01_requirements.md` §2.3 のスコープ外）。実装開始時に再度確認する。
- **外部ツール・ランタイムの前提確認:**
  - 開発環境で `/bin/bash`（GNU bash `3.2.57`）、`PATH` 上の `bash`（GNU bash `5.3.15`）、`git`（`2.50.1`）、`go`（`go1.27.1`）の存在を確認した。
  - `git init -h`（`--bare`）・`git fetch -h`（`--no-tags`・`--quiet`）・`go help run`・`.github/workflows/ci.yml:277-291`（`doc-guard` の実行コマンド `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）・`Makefile:352-376`（`deadcode` ターゲット）を一次情報で確認した。
  - `internal/mergepr/corrections.go:250-261` を確認し、信頼しない文字の範囲（`unicode.Cc`・`unicode.Cf`・`unicode.Zl`・`unicode.Zp`・非文字 `0xFDD0..0xFDEF` / `r&0xFFFE == 0xFFFE`・不正 UTF-8 バイト列）が Go 標準ライブラリで正確に判定できることを確認した。
- **申し送り事項:** [design_handoff.md](design_handoff.md) の H-01〜H-03 はすべて [02_architecture.md](02_architecture.md) §1.4 に記録済みである。`implementation_handoff.md` は存在しない。

### 1.4. 計画の際に決めたこと（2026-11-24）

以下の 2 点をレビュアーと合意して決めた。

- **`_checks/check_open_branches.sh` における信頼しない文字列のエスケープ実装方式:**
  - **決定と理由:** Go 標準ライブラリのみを用いる補助ファイル `docs/tasks/0121_retire_legacy_review_records/_checks/escape_untrusted.go` を併設し、`check_open_branches.sh` 実行時に一時ディレクトリへ 1 回 `go build` して使う（案 A）。開発・CI 環境で確実に利用できる `go` 以外のランタイム依存（`python3`・`perl`）を増やさず、`internal/mergepr/corrections.go:250-261` と同じ `unicode` / `unicode/utf8` の分類で `Cc`（タブ含む）・`Cf`・`Zl`・`Zp`・非文字・不正 UTF-8 を正確にエスケープできるためである。`_checks/` は `_` 始まりのため `go list ./...`・`make test`・`make lint`・`make deadcode` の対象外となる。
  - **ADR の判断:** ADR なし（タスク専用の `_checks/` 一時スクリプトの内部実装方式であり、常設の開発ツールや [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の決定を変えないため）。
- **`_checks/check_open_branches.sh` のローカル擬似 `origin` 壊し確認（設計書 §7.3）の配置方法:**
  - **決定と理由:** ローカル検証スクリプト `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh` をコミットし、フェーズ 1 の壊し確認と AC-02 の検証に使う（案 A）。設計書 §7.3 の 7 シナリオすべてをネットワークなしの 1 コマンドで再現実行でき、レビュー修正後の再検証と AC-02 の実行可能な検証対象（`test`）を確実に残せるためである。`_checks/` 配下のため `make test` や CI には含まれない。
  - **ADR の判断:** ADR なし（タスク専用の `_checks/` 配下に検証スクリプトを置くかどうかの選択であり、タスク内で閉じるため）。

## 2. 実装ステップ (Implementation Steps)

フェーズ構成と順序は [02_architecture.md](02_architecture.md) §8 の実装優先順位に従う。

### フェーズ 1: 移行ゲートの整備と実装開始前の前提確認（F-001）

#### ステップ 1-1: 移行ゲートスクリプトとエスケープ補助の作成

- **変更対象ファイル:**
  - `docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh`（新規）
  - `docs/tasks/0121_retire_legacy_review_records/_checks/escape_untrusted.go`（新規）
- **作業内容:**
  - [ ] 設計書 §3.1・§4.2・§5.2・§6.1 に従い、`_checks/check_open_branches.sh` を作成する（引数なし検証、一時 bare リポジトリ作成と `EXIT` トラップでの後始末、非 shallow・非 partial clone 時のみの `alternates` 登録、相対 `origin` URL のメイン作業ツリー基準での絶対パス解決、`ls-remote --heads` とのブランチ集合一致と `refs/heads/main` の `commit` 種別確認、隔離環境での 40 桁コミット ID 指定による `main` 未到達コミット全体の 3 形式照合、CRLF 行末の `\r` 除去、バッファリング出力と終了コード `0`/`1`/`2` の契約）。コメント・識別子・診断メッセージは英語で書く（`CLAUDE.md`「Language」）。
  - [ ] `_checks/escape_untrusted.go` を Go 標準ライブラリのみで作成し、設計書 §3.1「信頼しない文字列の出力時エスケープ」の規則（`Cc`［タブ含む］・`Cf`・`Zl`・`Zp`・非文字・不正 UTF-8 バイト列の `\xHH` 化、および `\` の `\\` 化）を実装する。`check_open_branches.sh` からは実行ごとに一時ディレクトリへ 1 回 `go build` したバイナリ（作業ディレクトリに依存しないよう `GO111MODULE=off` 等で単一ファイルビルド）を呼び出し、出力のブランチ名・一致行および標準エラーの診断に適用する。
- **完了条件:**
  - `bash -n` および `/bin/bash -n`（bash 3.2）で構文エラーがなく、ステップ 1-2 の全検証シナリオを通過できる状態であること。

#### ステップ 1-2: ローカル擬似 `origin` による壊し確認スクリプトの作成と実行

- **変更対象ファイル:**
  - `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh`（新規）
- **作業内容:**
  - [ ] 設計書 §7.3 の全 7 シナリオ（3 形式と CRLF・孤立ブランチの終了コード `1` 検出、`main` のみ・インデント・`(coderabbit)` の非検出、stale な `refs/remotes/origin/*` と未取得新規ブランチの扱い、linked worktree からの実行、存在しない URL・`main` 不在・引数指定での終了コード `2` と空 stdout、呼び出し元リポジトリ無変更、制御文字・双方向制御文字・ゼロ幅文字・タグ文字・非文字・不正 UTF-8・`\` を含む一致行およびブランチ名のエスケープ出力）をネットワークなしのローカル一時リポジトリで自動検証する `_checks/test_check_open_branches.sh` を作成する。
  - [ ] `/bin/bash`（bash 3.2）と `PATH` 上の `bash` の双方で `test_check_open_branches.sh` を実行し、全シナリオが成功することを確認する。
  - [ ] `check_open_branches.sh` および `escape_untrusted.go` に意図的な壊し（3 形式のいずれかの照合を外す、異常系フェイルクローズを外す、制御文字エスケープを外す等）を入れた際に `test_check_open_branches.sh` が失敗することを確認し、確認内容をコミットメッセージに記録する（確認後に壊しを元に戻す）。
- **完了条件:**
  - `/bin/bash docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh` および `bash docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh` が終了コード `0` で完了すること。

#### ステップ 1-3: 実 `origin` に対する実装開始前の前提確認（AC-01・AC-02）

- **変更対象ファイル:**
  - なし（確認結果の記録先として本計画書 §7 または実装 PR の説明に結果を残す）
- **作業内容:**
  - [ ] 実装着手時点の `origin/main` を確認し、`3fff7f51` 以降にコマンド定義・`review-records.sh`・`review_records_test.go` への新たな変更が入っていないか確かめる。
  - [ ] 利用者の明示承認を得て、実 `origin` に対して `bash docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh` を単一コマンドとして実行する（設計書 §3.1・§8）。
  - [ ] 終了コードが `0` 以外（`1`: 古い記録行を 1 件以上検出、`2`: 取得・読み取り異常）の場合は、該当ブランチ・一致行または診断を報告して作業を停止し、フェーズ 2 以降へ進まない（未コミットの作業は破棄または保留とし、実装コミット・PR を作成しない。AC-02）。
  - [ ] 終了コードが `0`（`matches: 0`）の場合は、出力に対して `<mention-check>` を実行して終了コード `1`（問題なし）を確認したうえで、確認日時（UTC）・基準 `origin/main` の 40 桁コミット ID・検査ブランチ一覧・`matches: 0` をコードブロック形式で本計画書 §7（または実装 PR の説明）へ記録する（AC-01、設計書 §5.2）。
- **完了条件:**
  - 実 `origin` に対する `check_open_branches.sh` が終了コード `0`（`matches: 0`）で完了し、`<mention-check>` を通過した記録が保持されていること。

### フェーズ 2: コマンド定義の撤去と `review-records.sh` の厳格化（F-002・F-003）

#### ステップ 2-1: コマンド定義 3 ファイルからの古い記録行の読み方規則の撤去

- **変更対象ファイル:**
  - `.claude/commands/_lib/local-cli-review.md`
  - `.claude/commands/_lib/task-pr-handoff.md`
  - `.claude/commands/_lib/cross-check-review.md`
- **作業内容:**
  - [ ] `.claude/commands/_lib/local-cli-review.md` の `## Reading the record lines`（`3fff7f51` 時点 `:293-326`）を設計書 §3.2 に従って更新する（AC-03・AC-06）。
    - `:307-308` を `- Every other \`CodeRabbit local review:\` and \`Codex local review:\` line is a` から `- Every other \`CodeRabbit local review:\` line is a` へ変更する。
    - `:314` を `- A \`Local CLI reviewed: <START12>..<END12> (<cli>)\` line names a seen range.` から `- A \`Local CLI reviewed: <START12>..<END12> (coderabbit)\` line names a seen range.` へ変更する。
    - `:323-326` の段落（`Read earlier \`(codex)\` seen ranges and \`(coderabbit: <kind>; codex: <kind>)\` ...` から `contains an old record.` まで）を削除する。範囲なし `CodeRabbit local review: not run`（`:304-306`）等は維持する。
  - [ ] `.claude/commands/_lib/task-pr-handoff.md` の `Part B step 3`（`3fff7f51` 時点 `:320-331`）を設計書 §3.2・§3.4 に従って更新する（AC-04・AC-06）。
    - `:324-325` を `` (`git log origin/main..<D>`; the `CodeRabbit local review:` and `Codex local review:` lines, read as `_lib/local-cli-review.md`, `` から `` (`git log origin/main..<D>`; the `CodeRabbit local review:` lines, read as `_lib/local-cli-review.md`, `` へ変更する。
    - `:329-331` を `Describe a Codex CLI or a cross-check finding only by its location and your own summary, as the record line does, never with Codex's or the cross-check worker's text;` から `Describe a cross-check finding only by its location and your own summary, as the record line does, never with the cross-check worker's text;` へ変更する。`Cross-check finding:`・`Fable finding:` の読み取り、本文転記禁止、`**Mention check.**`、`review-records.sh` 呼び出しブロックは維持する。
  - [ ] `.claude/commands/_lib/cross-check-review.md` の `## The prompt` — `Settled issues`（`3fff7f51` 時点 `:543-545`）を設計書 §3.2・§3.4 に従って更新する（AC-05・AC-06）。
    - `:543-545` を `` - the rejected findings of `CodeRabbit local review:`, `Codex local review:` (legacy lines from older branches), `Cross-check finding:`, and legacy `Fable finding:` lines in `git log origin/main..HEAD`; `` から `` - the rejected findings of `CodeRabbit local review:`, `Cross-check finding:`, and legacy `Fable finding:` lines in `git log origin/main..HEAD`; `` へ変更する。
- **完了条件:**
  - 3 ファイルの変更差分が上記の撤去と文言調整のみに収まっていること。

#### ステップ 2-2: `review-records.sh` の `SEEN_RE` 厳格化とヘッダコメントの整合

- **変更対象ファイル:**
  - `.claude/commands/_lib/review-records.sh`
- **作業内容:**
  - [ ] `:269` の `SEEN_RE` を `SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((coderabbit|codex)\)$'` から `SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(coderabbit\)$'` へ変更する（設計書 §3.3、AC-07〜AC-09）。
  - [ ] `:79` のヘッダコメントを `# Mode unreviewed: the commits of a PR neither local CLI reviewed, for the PR` から `# Mode unreviewed: the commits of a PR the local CLI review did not review, for the PR` へ変更する（設計書 §3.3）。
  - [ ] `NOT_RUN_RE`（`:271`）、`checked_range`、クリーン環境再実行、git plumbing 読み取り、`fail` 契約に変更がないことを確認する。
- **完了条件:**
  - `/bin/bash -n .claude/commands/_lib/review-records.sh` および `bash -n .claude/commands/_lib/review-records.sh` が成功し、`SEEN_RE` のみが `(coderabbit)` に厳格化されていること。

### フェーズ 3: テスト更新・関連文書確認・PR 作成（F-004、`/runplan` 完了点）

#### ステップ 3-1: `cmd/yt2column/review_records_test.go` の既存ケース書き換えと拒否・混在ケースの追加

- **変更対象ファイル:**
  - `cmd/yt2column/review_records_test.go`
- **作業内容:**
  - [ ] 既存の 4 か所を設計書 §1.3 決定 1・§3.4・§7.1 に従って書き換える（テストの削除は行わない。Go コード・コメント・文字列リテラルはすべて英語で書く。AC-10）。
    - `coverageCases()` の `"seen by codex"`（`:355-359`）のケース名を実態に合わせ（例: `"codex reviewer"`）、期待値を `f.partial(f.rev("HEAD~1"))`・終了コード `0` へ変更する（`TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed` の双方で実行）。
    - `unreviewedCases()` の `"fix seen by the standalone path's review"`（`:904`）の `f.seen(r, fix, "codex")` を `f.seen(r, fix, "coderabbit")` へ更新する。
    - `coverageCases()` の `not run` 系 2 ケース（`:425,431`）の旧入力 `"(coderabbit: other; codex: timeout)"` を現行の CodeRabbit 単体形式 `"(coderabbit: other)"` へ統一する。
    - `"HEAD behind LAST_REVIEWED"` のコメント（`:381-382`）を `// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,` / `// yet what Codex reviewed is not what is merged.` から `// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,` / `// yet what the PR review saw is not what is merged.` へ変更する。
  - [ ] `rrFixture.seen`（`:184-186`）のシグネチャを維持したまま、設計書 §7.1 の新規ケースを追加する（AC-07〜AC-09）。
    - `coverageCases()`（`TestReviewRecordsUnreviewed` でも自動実行）に、大文字小文字違い・前後空白や末尾空白の拒否ケース、および同一コミット・同一ブランチ上の `(codex)` と有効な `(coderabbit)` の混在ケースを追加する。
    - `crossCheckCases()`（`TestReviewRecordsCrossCheck`）に、ルール 2 の `(codex)` 単独拒否（`MISSING`）・ルール 2 の `(codex)` と有効 `(coderabbit)` または `Local CLI review: not run` の混在（`VALID`）・ルール 4 の `(codex)` 単独拒否（`MISSING`）・ルール 4 の `(codex)` と有効 `(coderabbit)` の混在（`VALID`）を追加する。
    - `TestReviewRecordsFableAlias`（`:2047`）に、`fable` 互換別名でルール 2 またはルール 4 の `(codex)` 単独が `MISSING` として拒否される代表ケースを追加する（設計書 §3.4）。
  - [ ] 壊し確認の義務: `review-records.sh` の `SEEN_RE` を一時的に `(coderabbit|codex)` へ戻した状態で `go test -tags test ./cmd/yt2column -run 'TestReviewRecords(Coverage|Unreviewed|CrossCheck|FableAlias)'` を実行し、書き換え・追加した `(codex)` 拒否ケース（`PARTIAL`・`MISSING` 期待）がいずれも失敗することを確認してコミットメッセージに記録する（確認後に元に戻す）。
- **完了条件:**
  - `go test -tags test -v ./cmd/yt2column -run TestReviewRecords` が全 bash 環境（bash 3.2・bash 4+）で成功すること。

#### ステップ 3-2: 関連文書の再確認・差分記録・品質関門の実行と PR 作成

- **変更対象ファイル:**
  - `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md`（新たな該当箇所が見つかった場合のみ変更）
  - `docs/tasks/0121_retire_legacy_review_records/03_implementation_plan.md`（AC-06 の diff 記録および AC-01 実装開始前確認の記録先として使う場合）
- **作業内容:**
  - [ ] `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を再度検索・確認し、撤去対象の 3 形式を現行の読み方規則として記述した箇所が追加されていれば更新する（AC-11）。
  - [ ] `docs/dev/conventions/requirements_process.md` §4 の固定コマンドを用いて、本計画書 §7 の `Diff record (AC-06)` に `.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/cross-check-review.md` の差分を記録する（本計画書は比較対象に含めない）。
  - [ ] `make fmt`・`make test`・`make lint`・`make deadcode` および CI の `doc-guard` と同一のコマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）を実行し、すべて成功することを確認する（AC-12）。テスト削除が発生していないため `make coverage-compare` は非該当とする（万一削除が生じた場合のみ [ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) に従い削除専用コミットを分けて実行・記録する）。
  - [ ] 実装 PR を作成する。PR の説明にステップ 1-3 の実装開始前確認記録（`<mention-check>` 通過後のコードブロック）を記載し、`レビュー観点` に「`/runpr` のマージの質問に答える直前にマージ前ゲート（`_checks/check_open_branches.sh`）を再実行し、その結果が PR の説明に記録されていることを確認する（設計書 §1.3 決定 5・§8）」と明記する。
- **完了条件:**
  - 品質関門の全コマンドと `Diff record (AC-06)` の再照合が成功し、実装 PR が作成されていること。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

1. **マイルストーン 1（フェーズ 1 完了 — 移行ゲート通過）:** `_checks/check_open_branches.sh`・`_checks/escape_untrusted.go`・`_checks/test_check_open_branches.sh` が完成して全 7 シナリオと壊し確認に通り、実 `origin` で `matches: 0` が確認・記録されている（成果物: `_checks/` 一式、AC-01 実装開始前の記録）。
2. **マイルストーン 2（フェーズ 2 完了 — 規則撤去と厳格化）:** コマンド定義 3 ファイルと `review-records.sh` の更新が完了し、差分が撤去・整合のみに収まっている（成果物: `.claude/commands/_lib/` 配下 4 ファイルの更新）。
3. **マイルストーン 3（フェーズ 3 完了 — 実装 PR 作成）:** `review_records_test.go` の書き換え・新規ケース追加、関連文書確認、`Diff record (AC-06)` の記録、および `make test`・`make lint`・`make deadcode`・`doc-guard` 通過が完了し、実装 PR が開かれている（成果物: 実装 PR）。
4. **マイルストーン 4（マージ前ゲート完了 — `/runplan` 外の手動ゲート）:** PR レビュー・CI 通過後、`/runpr` のマージ承認直前に実 `origin` に対して `_checks/check_open_branches.sh` を再実行し、`matches: 0` の結果（最終 push 以降の確認日時・最新 `origin/main` と一致する基準コミット ID）を `<mention-check>` 通過後に PR の説明へ記録したうえでマージする（設計書 §8「マージ前ゲート」）。

## 4. テスト戦略 (Test Strategy)

- **ユニットテスト（`cmd/yt2column/review_records_test.go`）:**
  - `TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` により、リポジトリ上の全 bash 環境（`scriptBashes`: macOS の bash 3.2 および bash 4+）で `review-records.sh` の実動作を検証する（設計書 §7.1）。
  - 既存ケースを書き換え、`(codex)`・大文字小文字違い・余分な空白の拒否、有効な `(coderabbit)` 行やルール 2 の `Local CLI review: not run` 行との混在、およびルール 4（マージ後確認の修正）の拒否・混在を網羅する。テスト削除を行わないため `make coverage-compare` は非該当。
  - 既存の `rrFixture` / `rrFixture.seen` を再利用するため、新規の `testutil/` や `test_helpers*.go` は作成しない（`docs/dev/conventions/test_organization.md`）。
- **移行ゲートのローカル結合検証（`_checks/test_check_open_branches.sh`）:**
  - 設計書 §7.3 の 7 シナリオをネットワークなしのローカル一時 bare リポジトリ・linked worktree に対して `/bin/bash`（bash 3.2）と `PATH` の `bash` で実行し、3 形式検出・非検出・stale 参照隔離・フェイルクローズ（終了コード `2`・空 stdout）・呼び出し元無変更・出力エスケープを検証する。タスク専用スクリプトのため `make test` と CI には含めない。
- **文書・差分・品質関門:**
  - AC-06 は `requirements_process.md` §4 の記録した diff の読み取り（`Diff record (AC-06)`）で検証し、`/runplan` の PR 作成前と `/runpr` のマージ前に機械照合する。
  - AC-03〜AC-05・AC-11 は文書の自然言語仕様・内容であるため、`CLAUDE.md`（Document tests check values, structure, and executable examples, never prose）に従い文書テストや `rg` チェックは追加せず PR レビューの通読で確認する。
  - 既存の文書ガードテスト群および `make test`・`make lint`・`make deadcode`・CI `doc-guard` の全通過を必須とする（AC-12）。
- **後方互換性テスト:**
  - 本タスクはタスク 0118 の移行期後方互換（AC-28）を意図的に退役させる変更であり（`01_requirements.md` §5）、`(codex)` 行が安全側（未見: `PARTIAL` / `MISSING`、終了コード `0`）へ倒れることと、同一コミット・ブランチ上の現行 `(coderabbit)` 行・`NOT_RUN_RE` による判定が壊れないことを回帰テストで確認する。

## 5. リスク管理 (Risk Management)

- **技術リスク:**
  - **開いたブランチへの古い記録行の残存:** 実装開始前またはマージ前に `origin` の対象ブランチに古い記録行が残っていると、マージ後の `review-records.sh` でそのブランチのコミットが未見に変わる。対策として、ステップ 1-3 とマージ前ゲートの 2 時点で `_checks/check_open_branches.sh` を実行し、終了コード `0` 以外は即時停止する（AC-01・AC-02）。
  - **マージ前ゲートから squash マージまでの時間差（TOCTOU）と形式 1 の残余リスク:** 確認直後からマージ完了までの間に古い記録行を持つブランチが push された場合、形式 2・3 は安全側へ倒れる一方、形式 1（`Codex local review:`）は指摘が PR 説明へ載らなくなる。この残余リスクは設計書 §1.3 決定 4・§5.2 で受容済みであり、マージ承認直前の再実行で時間差を最小化する。
  - **`check_open_branches.sh` の環境差（bash 3.2・大文字小文字非区別 FS・shallow / partial clone・信頼しない文字）:** `/bin/bash`（3.2）でのテスト実行、`ls-remote --heads` とのブランチ集合一致検証、非 shallow・非 partial clone 時のみの `alternates` 利用、および `_checks/escape_untrusted.go` による出力エスケープと `<mention-check>`・コードブロック囲みで緩和する（設計書 §3.1・§5.2）。
  - **並行タスク（例: タスク 0122）によるコマンド定義・テストの変更:** ステップ 1-3 の着手時に最新の `origin/main` を確認し、行位置やテスト構成が変わっていれば現行に合わせて適用する。
- **スケジュール・進行リスク:**
  - **ステップ 1-3 またはマージ前ゲートでの停止:** 他ブランチに古い記録行が残っていた場合、該当 PR のマージまたはクローズを待つ必要がある。フェーズ 1 の段階で判定を先に行い、フェーズ 2・3 の手戻りを防ぐ。
  - **マージ前ゲートのやり直し:** マージ承認前に追加 push または `origin/main` の更新があった場合は、設計書 §8 のとおりマージ前ゲートを再実行し、PR の説明の記録を更新してからマージする（コミット・push は増やさない）。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1: `_checks/check_open_branches.sh`・`_checks/escape_untrusted.go` 作成、ステップ 1-2: `_checks/test_check_open_branches.sh` 作成・実行・壊し確認、ステップ 1-3: 実 `origin` 確認と記録）
- [ ] フェーズ 2 完了（ステップ 2-1: コマンド定義 3 ファイルの更新、ステップ 2-2: `review-records.sh` の `SEEN_RE` 厳格化・コメント整合）
- [ ] フェーズ 3 完了（ステップ 3-1: `review_records_test.go` 更新・新規ケース追加・壊し確認、ステップ 3-2: 関連文書再確認・`Diff record (AC-06)` 記録・品質関門通過・実装 PR 作成）
- [ ] マージ前ゲート完了（PR レビュー・CI 通過後、マージ承認直前に実 `origin` へ `_checks/check_open_branches.sh` を再実行し、`<mention-check>` 通過後に結果を PR の説明へ記録）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC ID | 検証種別 | 検証対象・方法 | 対応ステップ |
|---|---|---|---|
| AC-01 | `static`・`manual` | `docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh` を実装開始前とマージ承認直前の 2 時点で実 `origin` に対して実行し、終了コード `0`・`matches: 0` を確認（`static`）。2 時点それぞれの確認日時（UTC）・基準 `origin/main` の 40 桁コミット ID・検査ブランチ一覧・`matches: 0` が `<mention-check>` 通過後にコードブロックで計画書または PR の説明（マージ前は PR の説明）へ記録されていることを PR レビューで確認（`manual`） | ステップ 1-1・1-3・3-2（およびマージ前ゲート） |
| AC-02 | `test`・`manual` | `docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh`（擬似 `origin` 上の 3 形式・CRLF・孤立ブランチで終了コード `1` と該当ブランチ・一致行出力、存在しない URL・`main` 不在・引数指定等で終了コード `2`・空 stdout、呼び出し元無変更、出力エスケープを検証）（`test`）。実 `origin` 実行時に非ゼロ終了であれば実装開始・マージを停止して報告する手順の遵守（`manual`） | ステップ 1-1・1-2・1-3（およびマージ前ゲート） |
| AC-03 | `manual` | PR レビューで `.claude/commands/_lib/local-cli-review.md` の `## Reading the record lines` 節を通読し、指摘の行の接頭辞が `CodeRabbit local review:` のみ・見た範囲の行の形が `Local CLI reviewed: <START12>..<END12> (coderabbit)` のみであり、旧 3 形式の読み取り記述と退役条件の段落を持たないことを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-04 | `manual` | PR レビューで `.claude/commands/_lib/task-pr-handoff.md` の Part B step 3 を通読し、`Codex local review:` および Codex CLI 指摘の記述が外れ、`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` の読み取りと本文転記禁止の規則が維持されていることを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-05 | `manual` | PR レビューで `.claude/commands/_lib/cross-check-review.md` の `## The prompt` — `Settled issues` を通読し、`Codex local review:` の記述が外れ、`CodeRabbit local review:`・`Cross-check finding:`・`Fable finding:` の却下済み指摘の読み取りが維持されていることを確認（自然言語内容のみの AC。変更範囲は下の `Diff record (AC-06)` でも確認） | ステップ 2-1 |
| AC-06 | `diff` | 記録した diff の読み取り（下の `Diff record (AC-06)`）。比較対象パスは `.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/cross-check-review.md`（本計画書は含めない）。`/runplan` が `requirements_process.md` §4 の固定コマンドで記録し、PR 作成前とマージ前に機械照合する | ステップ 2-1・3-2 |
| AC-07 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（有効な `(coderabbit)` 行が `START`/`END` 検証を通過するとき `ALL`・終了コード `0` となること、単独経路の修正ケースを含む） | ステップ 2-2・3-1 |
| AC-08 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（`(codex)`・大文字小文字違い・余分な空白・別レビュアー名が無視され `PARTIAL <n>`・終了コード `0` となること、および同一コミット・同一ブランチ上の有効な `(coderabbit)` 行に影響しないこと） | ステップ 2-2・3-1 |
| AC-09 | `test` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCrossCheck`・`cmd/yt2column/review_records_test.go::TestReviewRecordsFableAlias`（ルール 2・ルール 4 で `(codex)` 行単独は受理されず `MISSING <START>`・終了コード `0` となること、および同一コミット上の有効な `(coderabbit)` 行やルール 2 の `Local CLI review: not run` 行による判定に影響せず `VALID` となること） | ステップ 2-2・3-1 |
| AC-10 | `test`・`manual` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`・`cmd/yt2column/review_records_test.go::TestReviewRecordsUnreviewed`（書き換えた `(codex)` 拒否・`unreviewedCases()` の `(coderabbit)` 受理・`not run` ケースが通ること）（`test`）。PR レビューで旧形式の `not run` 入力文字列および Codex 前提コメントが残っていないことを確認（`manual`） | ステップ 3-1 |
| AC-11 | `manual` | 実装開始時および完了時に `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` を確認し、撤去対象の 3 形式を現行の読み方規則として記述した箇所が存在しないこと（該当があれば更新済みであること）を PR レビューで確認（自然言語内容のみの AC。`3fff7f51` 時点の事前確認では該当 0 件） | ステップ 3-2 |
| AC-12 | `test`・`static`・`manual` | `make test`・`make lint`・`make deadcode` および `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`（`test`・`static`）。CI の `doc-guard` ジョブ通過を PR 上で確認（`manual`）。テスト削除は発生しないため `make coverage-compare` は非該当（万一発生時のみ削除専用コミットで実行・記録） | ステップ 3-2 |

Diff record (AC-06):

- Compared paths: `.claude/commands/_lib/local-cli-review.md`, `.claude/commands/_lib/task-pr-handoff.md`, `.claude/commands/_lib/cross-check-review.md`

## 8. 完了基準 (Success Criteria)

- **機能網羅:** F-001〜F-004 および AC-01〜AC-12 のすべての実装タスクと検証項目が完了していること。
- **品質:** `make test`・`make lint`・`make deadcode`・CI の `doc-guard` ジョブおよび `_checks/test_check_open_branches.sh`（bash 3.2・bash 4+）がすべて成功し、`review-records.sh` と `check_open_branches.sh` に対する壊し確認の結果がコミットメッセージに記録されていること。
- **セキュリティ:** `(codex)` および不正表記の `Local CLI reviewed:` 行が全モードで無視されること、`check_open_branches.sh` がフェイルクローズと出力エスケープを満たすこと、および PR 説明へ記録する F-001 結果が `<mention-check>` を通過してコードブロックに収まっていること。
- **文書:** コマンド定義 3 ファイルの変更が `Diff record (AC-06)` で撤去・整合のみに限られること、AC-11 の対象文書に現行読み方規則としての旧 3 形式が残っていないこと、および AC-01 の 2 時点（実装開始前・マージ前）の記録がそろっていること。

## 9. 次のステップ (Next Steps)

1. 本実装計画書（`draft`）の PR レビューを受け、`/runpr` のマージの質問で承認（`approved`）およびマージする。
2. 承認後、`/runplan 0121` でフェーズ 1〜3 を実施し、実装 PR を作成する（フェーズ 1 の実 `origin` 確認で非ゼロ終了となった場合は実装へ進まず停止する）。
3. 実装 PR のレビューと CI が通過したら、`/runpr` のマージの質問を承認する**直前**にマージ前ゲート（`_checks/check_open_branches.sh`）を実 `origin` に対して再実行し、`matches: 0` の結果を `<mention-check>` 通過後に PR の説明へ記録したうえでマージする（設計書 §8「マージ前ゲート」）。
