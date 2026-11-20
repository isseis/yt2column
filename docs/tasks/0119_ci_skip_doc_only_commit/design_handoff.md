# Design Handoff: 0119_ci_skip_doc_only_commit

要件定義書（`01_requirements.md`）の起草過程で挙がった、設計（`02_architecture.md`）で決めるべき事項を申し送る。

## H-01: `pull_request` の合成マージコミット（`refs/pull/<N>/merge`）における PR head・親コミット・`main` 不変性の検証手段

**懸念:** GitHub Actions の `on: pull_request` では、`actions/checkout@v5` は既定で合成マージコミット `refs/pull/<N>/merge`（親 1 = `base.sha`、親 2 = `head.sha`）をチェックアウトする。そのため、チェックアウト直後の `HEAD` に対してそのまま「親が 1 つか」「`HEAD~1..HEAD` の差分」を調べると常にマージコミット扱いになる。また、親コミット（`head.sha~1`）に対する過去の CI がどの `main` コミットと合成して走ったかを正確に検証する手段を決める必要がある。

**重要性:** 誤って合成マージコミットを調べると常にスキップが効かず、逆に `main` の更新を見落とすと古い `main` 前提で通った親 CI を根拠に未検証の合成結果を誤スキップしてしまう。

**候補アプローチ:**
-   PR head コミットは `github.event.pull_request.head.sha`（`HEAD^2` と一致するかも検証可能）で明示的に参照し、その親コミット数（`git rev-list --parents -n 1 <head_sha>`）と差分（`<head_sha>~1..<head_sha>`）を調べる。
-   `main` の不変性の検証は、例えば (a) 親コミットの CI ワークフロー実行が持つ情報（作成・開始時刻と `origin/main` の最新コミットの時刻／または API から取れる `base` 情報、あるいは親 CI が検証した `base` SHA の確認方法）を調査したうえで、偽陰性（安全側）は許しても偽陽性（誤スキップ）を起こさない方式を設計する。時刻比較に頼る場合は、GitHub 側のコミット時刻（`committer date` 等）の偽装・巻き戻りやレース条件への耐性を確認し、可能であれば `base` SHA そのもの（例: 親の合成コミットや workflow artifact / API フィールド）で比較できるかを設計時に確かめる。

**関連 ID:** F-001, F-003, AC-01, AC-11

## H-02: 親コミットの CI 結果を取得する GitHub API エンドポイントと最小読み取り権限

**懸念:** 親コミットに対する本リポジトリの `CI` ワークフロー（`pull_request`）の最新実行と、各ジョブ（`check-changes`・`test`・`lint`・`extension`）の結論を取得するには、Actions API（`actions: read`）を使う案と Checks API（`checks: read`）を使う案がある。

**重要性:** Checks API だけでは別ワークフローの同名ジョブや古い再実行（`run_attempt`）との区別・ワークフロー単位の対応付けが曖昧になりやすい一方、Actions API（`/actions/workflows/<id>/runs` + `/actions/runs/<run_id>/jobs`）はワークフロー・イベント（`pull_request`）・最新の `run_attempt`・各ジョブの結論を明確に紐付けられる。また、同じ `head_sha` に複数 PR や複数 run がある場合の絞り込み条件とページネーション・サイズ上限を決める必要がある。

**候補アプローチ:**
-   GitHub Actions REST API（`actions: read` のみを `check-changes` ジョブに付与）で、`ci.yml` ワークフローの `head_sha=<parent_sha>` かつ `event=pull_request` の run 一覧を取得し、対象 PR かつ最新の `run_attempt` を選んでジョブ一覧の結論を検証する案を検討する。
-   ワークフローの識別（ファイルパス `.github/workflows/ci.yml` か ID か）、ジョブの識別（ジョブ ID か表示名か）、ページネーション打ち切り上限・応答サイズ上限・タイムアウトを設計で確定する。

**関連 ID:** F-002, F-003, F-004, §3.2, AC-05〜AC-08, AC-12, AC-15

## H-03: 判定ロジックの実装手段と配置先（ADR-0014 との整合）

**懸念:** [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) は「分岐が多く安全側に倒すことが要る決定的な判定は、テストのある Go の開発ツールに置く」方針を定めている（`docs/dev/conventions/dev_tools.md`）。本タスクのスキップ判定は JSON 厳密検査・git 差分分類・複数条件の fail-safe 判定を伴うため、`.github/workflows/ci.yml` にインラインの bash で書くか、Go のコード（例: per-run ツールや `internal/` のパッケージ）で書いてユニットテストと `internal/strictjson` を活用するかを決める必要がある。

**重要性:** `ci.yml` の `check-changes` はすでに `actions/setup-go` を実行しているため（`.github/workflows/ci.yml:26-30`）、Go で実装すれば追加のセットアップ費用なしに `internal/strictjson` による重複キー・型・UTF-8 の厳密検査と豊富なユニットテストを低コストで実現できる。一方、`cmd/` や `internal/` に置く場合は `dev_tools.md`・`package_reference.md`・`depguard`・`make deadcode` との整合を考慮する必要がある。

**候補アプローチ:**
-   Go のパッケージ＋コマンド（例: `cmd/ciskip` / `internal/ciskip` または `tools/` 配下の Go プログラム）として実装し、`check-changes` から呼び出す案と、シェルスクリプト＋テストの案を比較して決める。
-   `has-code-changes` の差分分類（`*.md`・`docs/`・`.claude/`・`LICENSE` 除外）も、`--no-renames`・`-z`・`core.quotePath=false` でリネームや制御文字パスを正しく扱える形にする（F-001 AC-04）。

**関連 ID:** F-001〜F-004, §3.2, §4.5, AC-01〜AC-13

## H-04: 既存テスト（`extension/test/repository.test.ts`）との整合

**懸念:** `extension/test/repository.test.ts:143-251` は `.github/workflows/ci.yml` の `check-changes` ジョブの特定の行（`has-extension-changes` 出力宣言、`git -c core.quotePath=false diff -z --no-renames --name-only origin/main...HEAD`、`if bash extension/scripts/wasm-dep-dirs.sh > "$dep_dirs"; then`、`| bash extension/scripts/has-extension-changes.sh "$dep_dirs")`、`has_extension_changes=true` フォールバック）を文字列・正規表現で固定している。

**重要性:** `check-changes` のステップ構成やスクリプト呼び出しを不用意に書き換えると既存の拡張テストが壊れる。既存の安全性のピンを弱めずにスキップ判定を組み込む必要がある。

**候補アプローチ:**
-   既存の `Classify changed files` ステップ（`origin/main...HEAD` の通常分類と `wasm-dep-dirs.sh` / `has-extension-changes.sh` の呼び出し・フォールバック）を保ったまま、通常分類が `true` になった場合に追加のスキップ判定を行い、条件を満たしたときだけ出力を `false` に上書きする（または後続ステップで最終出力を決める）構成を検討する。テストを変更する必要がある場合は、守っている不変条件を弱めないことを設計で示す。

**関連 ID:** F-004, AC-12, AC-16, AC-17
