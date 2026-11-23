# Design Handoff: 0119_ci_skip_doc_only_commit

要件定義書（`01_requirements.md`）の起草とレビューで挙がった、設計（`02_architecture.md`）で決めるべき事項を申し送る。

## H-01: `pull_request` の合成マージコミット（`refs/pull/<N>/merge`）における PR head・親コミット・`main` 不変性の検証手段

**懸念:** GitHub Actions の `on: pull_request` では、`actions/checkout@v5` は既定で合成マージコミット `refs/pull/<N>/merge`（親 1 = 合成時点の `base.sha`、親 2 = `head.sha`）をチェックアウトする。そのため、チェックアウト直後の `HEAD` に対してそのまま「親が 1 つか」「`HEAD~1..HEAD` の差分」を調べると常にマージコミット扱いになる。さらに、(a) 合成マージコミット作成時点の `HEAD^1` とチェックアウト時の `origin/main` の間で `main` が進むレース条件（`HEAD^1 != origin/main`）、および (b) 親コミット（`head.sha~1`）に対する過去の CI がどの `main` コミットと合成して走ったかを正確に検証する手段を決める必要がある。

**重要性:** 合成マージコミットを誤って調べると常にスキップが効かず、逆に `HEAD^1` と `origin/main` のずれや親 CI 以降の `main` 更新を見落とすと、古い `main` 前提で通った親 CI を根拠に未検証の合成結果を誤スキップしてしまう。

**候補アプローチ:**
-   PR head コミットは `github.event.pull_request.head.sha`（`HEAD^2` と一致するかも検証）で明示的に参照し、その親コミット数（`git rev-list --parents -n 1 <head_sha>`）と差分（`<head_sha>~1..<head_sha>`）を調べる。
-   `HEAD^1` と `origin/main` が一致していることも確認し、一致しない（合成後に `main` が進んだ）場合は安全側へ倒す。
-   親コミットの CI が検証した `main` との不変性については、GitHub API から取得できる情報（例: 親 CI 実行の作成・開始時刻と `origin/main` のコミット時刻／API の push 時刻の比較、または `base` SHA そのものを比較できる手段の有無）を設計時に調査し、偽陰性（安全側）は許しても偽陽性（誤スキップ）を起こさない方式を設計する。時刻比較を用いる場合は、git の `committer date` 偽装・巻き戻りや秒単位の競合に対する耐性を評価する。

**関連 ID:** F-001, F-003, AC-01, AC-11

## H-02: 親コミットの CI 結果を取得する GitHub API エンドポイント・再実行ジョブの取得・最小読み取り権限

**懸念:** 親コミットに対する本リポジトリ・同一 PR の `CI` ワークフロー（`pull_request`）の最新実行と、各ジョブ（`check-changes`・`test`・`lint`・`extension`）の結論を取得するには、Actions API（`actions: read`）を使う案と Checks API（`checks: read`）を使う案がある。また、親コミットで `doc-guard` だけが落ちて「Re-run failed jobs」で再実行された場合（`run_attempt = 2`）、GitHub Actions REST API の `/actions/runs/{run_id}/attempts/{attempt_number}/jobs` は attempt 2 で走ったジョブ（`doc-guard`）しか返さず、attempt 1 から再利用された `check-changes`・`test`・`lint`・`extension` が欠落する仕様上の落とし穴がある。

**重要性:** 誤ったエンドポイントを選ぶと、別ワークフローの同名ジョブとの取り違えや、「Re-run failed jobs」後の親コミットでの意図しないフォールバック・誤判定を招く。また、同一実行内に同じ名前のジョブが重複して現れる異常応答も拒否する必要がある。

**候補アプローチ:**
-   GitHub Actions REST API（`actions: read` のみを `check-changes` ジョブに付与）で、`ci.yml` ワークフローの `head_sha=<parent_sha>` かつ `event=pull_request` の run 一覧を取得し、完了状態で絞り込む前にまず対象 PR に紐付く全実行の中から最新実行を選ぶ。その最新実行が `status == completed` かつ結論が `success` または `failure`（`cancelled`・`timed_out` 等を除外し、未完了・キャンセル時は古い完了実行へ遡らず通常分類へフォールバックする）であることを確認したうえで、`/actions/runs/{run_id}/jobs?filter=latest`（全 attempt を横断した最新ジョブ一覧）を取得し、必須 4 ジョブがそれぞれちょうど 1 件ずつ存在して条件を満たすか検証する案を検討する。
-   `check-changes` 成功後にワークフローがキャンセルされた場合のワークフロー結論と後続ジョブ結論（`skipped` / `cancelled`）の挙動を、設計時に GitHub Actions の仕様で確認する。
-   ワークフローの識別（ファイルパス `.github/workflows/ci.yml` か ID か）、PR 紐付けの検証（`pull_requests` 配列が空または対象 PR を含まない場合の扱い）、ページネーション打ち切り上限・応答サイズ上限（展開後を含む）・タイムアウト・HTTPS 固定とリダイレクト拒否を設計で確定する。

**関連 ID:** F-002, F-003, F-004, §3.2, §4.2, AC-05〜AC-08, AC-12, AC-15

## H-03: 判定ロジックの実装手段・配置先およびリポジトリ規約との整合

**懸念:** [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) は「分岐が多く安全側に倒すことが要る決定的な判定は、テストのある Go の開発ツールに置く」方針を定めている（`docs/dev/conventions/dev_tools.md`）。本タスクのスキップ判定は JSON 厳密検査・git 差分分類・複数条件の fail-safe 判定を伴うため、Go のコード（例: `cmd/` + `internal/`）で実装してユニットテストと `internal/strictjson` を活用するか、シェルスクリプトで実装するかを決める必要がある。なお、Go のツールとして実装する場合は、次の既存規約・ガードテストとの整合を設計で決定する必要がある。
-   [ADR-0024](../../dev/adr/0024-env-access-in-config.md) および `internal/config/envaccess_test.go`: テスト以外の Go コードでの `os.Getenv` / `os.LookupEnv` / `os.Environ` は許可リストされた入口に限られる（フラグ／標準入力で渡すか、ADR-0024 と許可リストを更新するかを決める）。
-   `CLAUDE.md`「Declare, don't infer」: `git` 出力やファイルパスを解析する Go ツール（`coveragecompare`・`numreserve` 等）には明示的な例外記述を置いている。
-   `docs/dev/conventions/dev_tools.md` および `docs/dev/conventions/package_reference.md`: ツールの区分（installed / per-run / CI 専用）、パッケージ一覧、`depguard`、`make deadcode` との整合。

**重要性:** `ci.yml` の `check-changes` はすでに `actions/setup-go` を実行しているため（`.github/workflows/ci.yml:26-30`）、Go で実装すれば追加のセットアップ費用なしに `internal/strictjson` による重複キー・型・UTF-8 の厳密検査と包括的なユニットテストを実現できる一方、上の規約との整合を設計時に漏らすと `make test` / `doc-guard` で衝突する。

**候補アプローチ:**
-   Go のパッケージ＋コマンド（例: `cmd/ciskip` / `internal/ciskip` 等）として実装し、`check-changes` から `go run` で呼ぶ案（または `tools/`・シェルスクリプト案）を比較して決める。
-   `has-code-changes` の差分分類（`*.md`・`docs/`・`.claude/`・`LICENSE` 除外）も、`git -c core.quotePath=false diff -z --no-renames --name-only` 相当の NUL 区切り・リネーム両側展開でリネームや非 ASCII・制御文字パスを正しく扱える形にする（F-001 AC-04）。

**関連 ID:** F-001〜F-004, §3.2, §4.5, AC-01〜AC-13

## H-04: 既存テスト（`extension/test/repository.test.ts`）との整合

**懸念:** `extension/test/repository.test.ts:143-251` は `.github/workflows/ci.yml` の `check-changes` ジョブの特定の行（`has-extension-changes` 出力宣言、`git -c core.quotePath=false diff -z --no-renames --name-only origin/main...HEAD`、`if bash extension/scripts/wasm-dep-dirs.sh > "$dep_dirs"; then`、`| bash extension/scripts/has-extension-changes.sh "$dep_dirs")`、`has_extension_changes=true` フォールバック）を文字列・正規表現で固定している。

**重要性:** `check-changes` のステップ構成やスクリプト呼び出しを不用意に書き換えると既存の拡張テストが壊れる。既存の安全性のピンを弱めずにスキップ判定を組み込む必要がある。

**候補アプローチ:**
-   既存の `Classify changed files` ステップ（`origin/main...HEAD` の通常分類と `wasm-dep-dirs.sh` / `has-extension-changes.sh` の呼び出し・フォールバック）を保ったまま、通常分類が `true` になり、かつ `wasm-dep-dirs.sh` が成功した場合に追加のスキップ判定を行い、条件を満たしたときだけ出力を `false` に上書きする（または後続ステップで最終出力を決める）構成を検討する。テストの更新が必要な場合は、守っている不変条件を弱めないことを設計で示す。

**関連 ID:** F-004, AC-12, AC-16, AC-17
