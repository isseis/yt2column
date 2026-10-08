# 実装計画書：ブラウザ拡張の開発環境と入力の収集

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-07 |
| Review date | 2026-10-07 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義したブラウザ拡張の開発環境と入力の収集を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   `extension/` に TypeScript の開発環境（版の固定、型検査、lint、フォーマット、ユニットテスト、ビルド、成果物の検査）を用意し、Makefile のターゲットと CI のジョブを加える。Go の手順の結果は変えない。
-   拡張 ID を固定した Manifest V3 の manifest を作る。
-   収集と確認（`collect`）、文言、表示用の要約、表示、2 つの起動の経路を作る。
-   Chrome と Brave で手動の確認を行い、結果を本計画に記録する。文書を更新する。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。

本計画は、設計書 8 章の実装優先順位 1〜5 を、そのままフェーズ 1〜5 とする。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に、判定をブラウザの API から切り離すこと（`core/`）、ブラウザの API を依存の interface の向こうに置くこと、収集と確認を `collect` の 1 か所に集めること、拒否の理由を型で表すこと、収集した値を補正しないこと、読み取った文字列を `textContent` だけで表示することを守る。
-   新設するファイルは設計書 2.1・3.13 に挙げたものと、本計画が加える次のファイルに限る。
    -   `extension/scripts/has-extension-changes.sh`（CI の変更の判定。ステップ 1-6）
    -   設計書 3.11 にないテストのファイル: `typecheck.test.ts`・`lintRules.test.ts`・`checkDist.test.ts`・`repository.test.ts`・`ciChanges.test.ts`（フェーズ 1）、`docs.test.ts`（フェーズ 5）
-   Go のソースとテスト（`*.go`）を変更しない。そのため、文書と設定を確かめるガードのテストも、Go ではなく `extension/test/` の `node --test` のテストとして書く（要件書 5.・AC-05）。
-   拡張のソースのコメント・識別子・文字列リテラル・利用者に表示する文言は英語で書く（要件書 F-001）。例外は、日本語の文書の見出しや表の列名と照合するためにテストが持つ文字列リテラル（`docs.test.ts` の「想定ディレクトリ構成」など）だけとする。既存の `cmd/yt2column/docs_test.go` の「設定（環境変数）」と同じ扱いである。`AC-NN`・`F-NNN`・`I-NN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4 と同じ扱い）。
-   複数のテストのファイルが使う補助（拡張 ID の計算、fake の `SelectionReader`・`SummaryStore`・`chrome` の API）は、`extension/test/helpers/` の下の名前を付けたモジュールに置き、テストのファイルの間で複製しない（`test_organization.md` の考え方を拡張に当てはめたもの）。`node --test` のテストのファイルのパターンに一致しない名前にする。
-   テストの名前（`describe` の名前）は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストと lint の規則は、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（[CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」、設計書 7.1）。壊す対象は、各フェーズの最後のステップに挙げる。
-   各フェーズの完了条件は、`make ext-check` → `make test` → `make lint` が通ることである。フェーズ 1 では、加えて `make deadcode`・`make build` を実行する（AC-05）。
-   **ネットワークへのアクセス:** `npm install`・`make ext-install`（`npm ci`）はレジストリへアクセスする。実装者は、初回の実行の前に利用者の承認を得る（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」）。依存パッケージの追加・更新は `npm install --ignore-scripts <pkg>` で行い、更新した lockfile をコミットする（設計書 3.8）。
-   **Node.js の版:** Makefile の拡張のターゲットは、`node --version` が `extension/.node-version` と一致しないと失敗する（設計書 3.8）。調査の時点の作業環境は、Node.js が `v26.4.0`、npm が `11.17.0` で、設計書が定める Node.js 24 の LTS と異なる（2026-10-07 に `node --version`・`npm --version` で確認）。この作業環境には、版の管理のツール（`fnm`・`mise`・`nvm`・`volta`）もない。実装者は、利用者の承認を得てから（ツールと Node.js の取得はネットワークにアクセスする）、`.node-version` の版を用意して作業する（§6.1）。

### 1.3. 既存コード調査結果

HEAD `35b7829`（ブランチ `issei/browser-extension-04`）で確認した。設計書は既存のファイルの行番号をコミット `426eb2e` で記している。以下の `file:line` は HEAD `35b7829` のものである。

**拡張のコード**

-   リポジトリに、TypeScript・JavaScript のソース、`package.json`、Node.js の設定はない。`extension/` は存在しない。拡張のコードと設定はすべて新設であり、再利用できる既存の実装はない。

**`Makefile`**

-   `.PHONY` は `Makefile:49` の 1 行である。拡張のターゲットはここに加える。
-   `fmt-all` は `Makefile:150-152` で、`find . -name '*.go' -not -path './vendor/*'` を使う（`Makefile:152`）。ここに `-not -path './extension/*'` を加える（設計書 3.10）。
-   `test`（`Makefile:68`）・`test-ci`（`:72`）・`lint`（`:134`）は `./...` を使う。`deadcode`（`:154`）と `build`（`:53`）は `./cmd/yt2column` を起点にする。
-   既存の Go のテストが `Makefile` を読む。
    -   `TestLintTagsIncludeIntegration`（`internal/transcript/ytdlp_test.go:1746`）は、`Makefile`・`.pre-commit-config.yaml`・`ci.yml` のそれぞれについて、`golangci-lint@` を含む行に `--build-tags` があるか、`vet -tags integration ./...` の文字列があるかを確かめる。
    -   `internal/llm/deepseek/makefile_test.go` と `cmd/yt2column/makefile_test.go` は、`GOTEST` をスタブに替えて `make -s <target>` を実行する。このとき環境変数は `PATH`・`HOME`・`TMPDIR` などに限る（`internal/llm/deepseek/testutil/make.go:81-111`）。
-   本計画では、拡張のターゲットに、`golangci-lint@` を含む行も `GOTEST` を使うレシピも書かない。Node.js の呼び出しもレシピの中だけに限る（ステップ 1-3）。そうすれば、上の 2 種類のテストの結果は変わらないはずである。確認はステップ 1-7 の `make test` で行う。

**`.github/workflows/ci.yml`**

-   `check-changes` のジョブ（`ci.yml:12-43`）は、`git diff --name-only origin/main...HEAD` の結果から `has-code-changes` を決める（一覧の取得は `ci.yml:26`、判定は `ci.yml:35`）。判定はシェルのインラインの `grep` で、テストはない。本計画は `has-code-changes` の判定を変更しない（設計書 3.10）。
-   `has-extension-changes` の判定は 4 つのパターンの OR になる（設計書 3.10）。実装計画の作成の規則（`.claude/commands/mkplan.md` の Conditional checks）の「2 つ以上の条件を持つ CI の判定は、スクリプトに取り出してテストする」に従い、`extension/scripts/has-extension-changes.sh`（標準入力のファイルの一覧から `true`・`false` を出力する）に取り出し、`ciChanges.test.ts` でテストする。置き場所を `extension/` の下にするのは、このスクリプトの変更自体が `^extension/` のパターンで拡張のジョブを起動するためである。`check-changes` のジョブは Node.js を用意しないので、スクリプトは bash で書く。
-   ワークフローの最上位の `permissions: contents: read` は `ci.yml:7-8` にある。拡張のジョブはこれを引き継ぐ。
-   `TestLintTagsIncludeIntegration` は `ci.yml` も読む（`args:` を含む行と `vet -tags integration ./...`）。拡張のジョブに `args:` を含む行を書かない。

**`.gitignore`・`.pre-commit-config.yaml`・`go.mod`**

-   `.gitignore` は 9 行（空行を含む）で、拡張に関する行はない。`build/`（`.gitignore:1`）は拡張の `dist/` と重ならない。
-   `.pre-commit-config.yaml` は `pre-commit-hooks` を `rev: v5.0.0` で固定している。`detect-private-key` は未使用である。
-   `go.mod` は `go 1.26.5`（`go.mod:3`）で、`ignore` の指示はない。`ignore` の指示の効果（`extension/node_modules` の下の Go のパッケージが `go list ./...` に現れなくなること）は、設計書 3.10 が作業ディレクトリでの実験で確かめている。

**秘密鍵の検出（AC-09）**

-   設計書 7.4 は、`git grep -n "PRIVATE KEY"` が何も見つけないことを確かめるとしている。しかし HEAD `35b7829` でこのコマンドを実行すると、要件書（`01_requirements.md:120`）と設計書（`02_architecture.md:296`・`:1092`）にある説明の文が 3 件見つかる。一方、PEM の見出しの形 `git grep -n -E -e "-----BEGIN [A-Z0-9 ]*PRIVATE KEY"` では何も見つからなかった（終了コード 1。2026-10-07、HEAD `35b7829`）。そこで本計画は、設計書 7.4 の `git grep` による確認を、PEM の見出しの形（`-----BEGIN [A-Z0-9 ]*PRIVATE KEY`。行のどこに現れても一致する）で追跡中のファイルを検査するテストに置き換える。このテストはステップ 1-5 の `repository.test.ts` に置く。この置き換えは、要件書 AC-09 の「PEM の `PRIVATE KEY` ブロック」と同じ対象を確かめる編集上の修正である。そのため、設計書 7.4 の記述を本計画と同じコミットで改め、設計書の `Comments` に記録した（`requirements_process.md`「Editing an approved document」。設計書の状態は `approved` のまま。レビューで編集上の修正として受理された）。

**拡張 ID**

-   設計書 3.12.1 の `key` を base64 から戻し、Node.js の `crypto.createPublicKey`（`format: "der"`・`type: "spki"`）で解析すると、`rsa`・2048 ビットの公開鍵になった。その DER の SHA-256 から設計書 3.1 の計算で求めた拡張 ID は `clfmbbcdpnjcefbdihdoahomaifbabkk` で、設計書 3.12.1 の値と一致した（2026-10-07、Node.js `v26.4.0`、スクラッチの一時ファイルで実行）。

**文書**

-   `README.md` の開発の節は `## Development`（`README.md:117`）である。`docs/dev/project_overview.md` の「言語: Go」は `:9`、「想定ディレクトリ構成」は `:69` にある。`docs/dev/security.md` には `## 8. ブラウザ拡張と localhost サーバ（#43）`（`:69`）がある。拡張の節は、この §8 の中に加える（ステップ 5-5）。`CLAUDE.md` は `### Build Commands`（`:41`）・`## Development Notes`（`:183`）・`### Dependencies`（`:197`）を持つ。
-   既存の Go の文書のテスト（`cmd/yt2column/docs_test.go`）は、README の使い方・フラグ・終了コード・設定の表と、`project_overview.md` の「設定（環境変数）」の表を確かめる。本計画の文書の変更は、これらの表を変えない。

### 1.4. implementation_handoff.md の項目への対応

| 項目 | 対応 |
|---|---|
| I-01（要約の削除の失敗時の契約とテスト） | `SummaryStore.remove` の契約を「削除に失敗したら reject する」とする（`chrome.storage.session.remove` の失敗をそのまま伝える）。`runMenuLaunch` は、`windows.create` の失敗の後の `remove` と `signalDisplayFailure` を、それぞれ独立に捕捉する。`remove` の失敗は `log.error` に記録し、成否にかかわらずバッジを表示する。実装はステップ 4-2・4-3、テストはステップ 4-6 の `launch.test.ts`（`windows.create` と `remove` を同時に失敗させる行） |
| I-02（AC-09 の秘密鍵の検査の実装（検出の正規表現とテストの配置）） | 検出の正規表現（PEM の見出し `-----BEGIN [A-Z0-9 ]*PRIVATE KEY`）とテストの配置（`repository.test.ts` の行と、常時実行の CI のジョブ `secret-scan`）を、本計画 §1.3 とステップ 1-5・1-6 に記す。設計書 7.4 は検証する性質だけを述べる（本計画と同じコミットで改めた） |

## 2. 実装ステップ (Implementation Steps)

本節の各ステップは、末尾に **対象:**（そのステップが触るファイルの集合で、フェーズの「対象ファイル」の部分集合）と **完了:**（そのステップの検証。確認だけのステップは「なし（確認のみ）」）を記す。フェーズの「完了条件」は、そのフェーズのすべてのステップの後に確かめる。

### フェーズ 1: 開発環境

**対象ファイル**
-   新設: `extension/` の設定のファイル（`.node-version`・`.npmrc`・`package.json`・`package-lock.json`・`tsconfig.json`・`tsconfig.core.json`・`tsconfig.build.json`・`eslint.config.js`・`.prettierrc.json`・`.prettierignore`）、`extension/scripts/`（`check-lockfile.ts`・`copy-static.ts`・`check-dist.ts`・`has-extension-changes.sh`）、`extension/src/core/acceptedUrl.ts`、`extension/test/`（`acceptedUrl`・`checkLockfile`・`checkDist`・`typecheck`・`lintRules`・`repository`・`ciChanges` の `*.test.ts`）
-   変更: `Makefile`、`go.mod`、`.gitignore`、`.pre-commit-config.yaml`、`.github/workflows/ci.yml`

**タスク**
-   [ ] **ステップ 1-1**: `extension/` の設定のファイルを、設計書 2.1・3.8 のとおりに作る。依存パッケージは設計書 3.8 の表のものに限り、すべて `devDependencies` にする。lockfile は承認を得てから `npm install --ignore-scripts` で作る（§1.2）。`eslint.config.js` には、次の 3 種類を禁止する規則を入れる: `eval` 系（`no-eval`・`no-implied-eval`・`no-new-func`）、HTML を解釈する API（設計書 3.7 の一覧）、動的な `import()`。`CollectedInput` への型の表明の禁止はフェーズ 3 で加える。
    -   **対象:** `extension/.node-version`・`.npmrc`・`package.json`・`package-lock.json`・`tsconfig.json`・`tsconfig.core.json`・`tsconfig.build.json`・`eslint.config.js`・`.prettierrc.json`・`.prettierignore`。**完了:** `make ext-install` が通り、TypeScript・ESLint・Prettier の各設定が読み込める。
-   [ ] **ステップ 1-2**: `extension/scripts/` の 3 つのスクリプトを作る（設計書 3.8・3.9・5.2）。`check-lockfile` と `check-dist` は、判定を関数として export し、テストから一時ディレクトリを渡して呼べる形にする。`check-dist` は、このフェーズではファイルの集合とモジュールの指定を確かめる。manifest と HTML の参照の検査は、参照先のファイルがそろうステップ 4-5 で加える。`static/` はフェーズ 2 まで存在しないので、`copy-static` と `check-dist` は `static/` がない場合を空として扱う。
    -   **対象:** `extension/scripts/check-lockfile.ts`・`copy-static.ts`・`check-dist.ts`。**完了:** 3 つのスクリプトがそれぞれ実行でき、`static/` がない状態を空として扱う。
-   [ ] **ステップ 1-3**: `Makefile` に設計書 3.8 の `ext-` のターゲットを加え、`.PHONY`（`Makefile:49`）に足す。版の確認（Node.js と npm）はすべての `ext-` のターゲット（`ext-install` を含む）で、`node_modules` の有無の確認は `ext-install` 以外の `ext-` のターゲットで行う。どちらもレシピの中だけで行い、解析時の `$(shell ...)`、`:=` による Node.js の呼び出し、全体の `export` を使わない。Go のターゲットが Node.js を必要としないためである（要件書 F-001）。`fmt-all` の `find` に `-not -path './extension/*'` を加える。`go.mod` に `ignore ./extension` を、`.gitignore` に設計書 3.8 の 3 行を、`.pre-commit-config.yaml` の `pre-commit-hooks` に `detect-private-key` を加える。
    -   **対象:** `Makefile`・`go.mod`・`.gitignore`・`.pre-commit-config.yaml`。**完了:** `make ext-install` が通り、`go list ./...` に `extension/` で始まるパッケージが現れない。
-   [ ] **ステップ 1-4**: `src/core/acceptedUrl.ts` に `isAcceptedWatchUrl`（設計書 3.4）を作り、`test/acceptedUrl.test.ts` で要件書 3.2 の受理しない URL の例のすべてと、受理する URL（`t`・`list`・フラグメント付き、`:443` 付き）を確かめる。設計書 8 章の「`core/` の 1 ファイルとテスト 1 つ」はこのファイルとする。
    -   **対象:** `extension/src/core/acceptedUrl.ts`・`extension/test/acceptedUrl.test.ts`。**完了:** `extension/test/acceptedUrl.test.ts::isAcceptedWatchUrl` が通る。
-   [ ] **ステップ 1-5**: 開発環境のテストを作る。
    -   `checkLockfile.test.ts`: レジストリ以外の `resolved` を持つ lockfile を拒否し、レジストリだけの lockfile を受理する。
    -   `checkDist.test.ts`: 一時ディレクトリの `src/`・`static/`・`dist/` で、余分なファイル、欠けたファイル、相対パスでないモジュールの指定、`dist/` にない相対パスの指定、動的な `import()` のそれぞれを拒否し、正しい組を受理する。
    -   `typecheck.test.ts`（AC-02）: 一時ディレクトリに、`tsconfig.json` と `tsconfig.build.json` をそれぞれ `extends` する設定と、型の合わない代入を 1 つ含むファイルを作り、`tsc` が失敗し、ビルドの設定では出力のファイルを作らないことを確かめる。`strict` のときだけ誤りになる入力（`null` を `string` に代入する、暗黙の `any` の引数）の行も置き、`strict` を外す変更はこの行で失敗させる。同じ設定で型の合うファイルが成功すること（対照）も確かめ、設定の誤り（`rootDir` の外など）で失敗しているのではないことを示す。
    -   `lintRules.test.ts`（AC-23 の 2 つ目の防御・AC-27）: ESLint の Node.js の API で `eslint.config.js` を読み、`eval`・`new Function`・文字列を渡す `setTimeout`・設計書 3.7 の HTML を解釈する API の各々・動的な `import()` を含むコードがそれぞれ違反になり、`textContent` への代入が違反にならないことを確かめる。
    -   `repository.test.ts`: (1) `git check-ignore` で `extension/node_modules/` と `extension/dist/` の下のパス、`*.pem` に一致するパス（`key.pem`・`extension/key.pem`）が無視されること（AC-06。`*.pem` は AC-09 の補助）。(2) `git ls-files` の追跡中のファイルのどの行にも PEM の秘密鍵の見出し（§1.3 の形）がないこと（AC-09。このテストのソース自身が検索の対象に一致しない書き方にする）。(3) `Makefile` の `ext-install` のレシピが `npm ci` を `--ignore-scripts` 付きで呼び、`Makefile` と `ci.yml` が `npm install` を呼ばないこと、`package-lock.json` が追跡されていること（AC-03）。(4) `ci.yml` の拡張のジョブが 6 つの `make ext-*` を別々のステップで実行し、ジョブの `if:` が `check-changes` の出力 `has-extension-changes` を参照し、`check-changes` がその出力を宣言していること（AC-07）。(5) Node.js と npm を含まない `PATH`（`make`・`sh` などへのシンボリックリンクだけを置いた一時ディレクトリ）で `make -n build test lint deadcode` が成功し、標準エラーに何も出ないこと。ステップ 1-3 で定めた「Go のターゲットが Node.js を必要としない」ことを確かめる。(6) `ci.yml` に、`has-extension-changes` に依存しない常時実行のジョブ `secret-scan` があり（`if:` を持たない）、追跡中のファイルの秘密鍵を検査すること（AC-09）。
    -   **対象:** `extension/test/checkLockfile.test.ts`・`checkDist.test.ts`・`typecheck.test.ts`・`lintRules.test.ts`・`repository.test.ts`。**完了:** これらのテストが `make ext-test` で通る。
-   [ ] **ステップ 1-6**: CI を変更する（設計書 3.10）。`has-extension-changes.sh` を作り、`check-changes` のジョブで `has-code-changes` と同じ変更の一覧を標準入力に渡して出力 `has-extension-changes` を決める。ジョブ `extension` を加え、設計書 3.10 の 6 つのステップ、`actions/setup-go`（既存のジョブと同じく `go-version-file: go.mod`）、`go list ./...` に `github.com/isseis/yt2column/extension/` で始まるパッケージがないことの確認を置く。`ciChanges.test.ts` は、スクリプトを `bash` で実行して次を確かめる。4 つのパターンごとに、そのパターンにだけ一致する 1 ファイルの一覧（`extension/` の下のファイル、`Makefile`、`.github/workflows/` の下のファイル、`go.mod`）を作り、それぞれ `true` になる。workflow のパターン（設計書 3.10 の `^\.github/workflows/`）が `ci.yml` だけに狭まっていないことを確かめるため、workflow については、`.github/workflows/ci.yml` だけの一覧と `.github/workflows/release.yml` だけの一覧を別々に置く（どちらも `true` になる。両方を 1 つの一覧に入れると、`ci.yml` に狭めたパターンでも `true` になり、狭めたことを検出できない）。`extension/` の例には `extension/scripts/has-extension-changes.sh` 自身も含める。一方、Go のファイルだけ、`docs/` の下のファイルだけ、`README.md` だけ、`extension` を部分文字列として含むだけのパス（例: `notes/myextension.txt`）だけの一覧、各パターンの文字列を先頭以外に含むか後ろに続きを持つパスだけの一覧（`notes/extension/x.txt`・`tools/Makefile`・`Makefile.local`・`tools/go.mod`・`tools/.github/workflows/ci.yml`。パターンの `^`・`$` を外した変更を検出する）と、空の一覧では、それぞれ `false` になる。あわせて、`has-extension-changes` に依存せず常に実行するジョブ `secret-scan` を加える。このジョブは、チェックアウトの後に、追跡中のファイルに PEM の秘密鍵の見出し（§1.3 の形）がないことを `git grep` で確かめる（AC-09）。
    -   **対象:** `.github/workflows/ci.yml`・`extension/scripts/has-extension-changes.sh`・`extension/test/ciChanges.test.ts`。**完了:** `extension/test/ciChanges.test.ts::has-extension-changes` と `extension/test/repository.test.ts::ci runs every extension step` が通る。
-   [ ] **ステップ 1-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象は次のとおりである。
    -   `isAcceptedWatchUrl` の各条件を 1 つずつ外す。
    -   `check-lockfile` のレジストリの判定を外す。
    -   `check-dist` の各検査を 1 つずつ外す。
    -   `tsconfig.build.json` の `noEmitOnError` を外す。
    -   `strict` を定義する設定のファイルから `strict` を外す（`strict` のときだけ誤りになる行が失敗する）。
    -   `eslint.config.js` の各規則を 1 つずつ外す。
    -   `.gitignore` の拡張の行を外す。
    -   `has-extension-changes.sh` のパターンを 1 つずつ外す。各パターンの `^`・`$` を 1 つずつ外す。workflow のパターンは `^\.github/workflows/ci\.yml$` に狭める（`release.yml` の行で失敗する）。
    -   PEM の見出しを含むファイルを一時的にステージする（コミットしない）。
    -   `ext-install` のレシピから `--ignore-scripts` を外す。
    -   `npm ci` を `npm install` に替える。
    -   `ci.yml` の 6 つのステップの 1 つを消す。
    -   ジョブの `if:` の出力の名前を変える。
    -   Go のターゲットのために解析時の `$(shell node --version)` を加える。
    -   CI の `go list ./...` のステップから `extension/` の判定を外す。
    -   Makefile の版の確認と `node_modules` の確認をそれぞれ外す（版の違う `.node-version` と、`node_modules` がない状態で確かめる）。
    -   テストのファイルの 1 つに失敗するアサーションを入れる（`make ext-test` が失敗すれば、テストが 0 件で成功していないことが分かる）。

    CI の `go list ./...` のステップは、`go.mod` から `ignore ./extension` を外したときに失敗することを、手元で同じコマンドを実行して確かめる。依存パッケージが Go のファイルを含まなくなっていて失敗しない場合は、そのことを記録する。続けて、`make ext-install` → `make ext-check`、`make test`・`make lint`・`make deadcode`・`make build` を通す。最後に AC-05 の比較を行い、§5.1 に記録する。比較の内容は、`extension/node_modules` と `extension/dist` がある状態と、`extension/` を一時的に退避した状態とで、4 つの Go の手順の成否と `go list ./...` の出力を比べることである。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。
-   [ ] **ステップ 1-8**: PR の CI で、拡張のジョブの 6 つのステップと Go の確認が通ることを確かめる。続けて、設計書 7.3 の AC-07 の 6 つの変更を 1 つずつ別のコミットとして push し、それぞれで対応するステップが失敗して CI が失敗することを確かめてから、その変更を戻す。結果（コミット、失敗したステップ）を §5.1 に記録する。
    -   **対象:** なし（確認のみ）。**完了:** PR の CI が通り、AC-07 の 6 つの変更のコミットで対応するステップが失敗することを確かめ、§5.1 に記録する。

**完了条件:** `make ext-check`・`make test`・`make lint`・`make deadcode`・`make build` が通り、§5.1 の AC-02・AC-03・AC-05・AC-07 の行が記録されている。

### PR-1 作成ポイント: extension toolchain, guard tests, and CI

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0007): bootstrap the browser extension toolchain and CI`

**レビュー観点**: `ext-` のターゲットが Node.js を必要とせず、Go の手順（`make test`・`make lint`・`make deadcode`・`make build`）の結果を変えないこと（ステップ 1-3・1-7、AC-05） / `has-extension-changes.sh` が 4 つのパターンと `^`・`$` を正しく判定し、拡張のジョブが `has-extension-changes` で起動すること（ステップ 1-6、AC-07） / 追跡中のファイルに PEM の秘密鍵がなく、その検査が常時実行のジョブ `secret-scan` と `repository.test.ts` で担われること（ステップ 1-5・1-6、AC-09） / lockfile が追跡され、インストールが `npm ci --ignore-scripts` に限られること（ステップ 1-3・1-5、AC-03）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-6 の CI のジョブの追加がパネルモードの引き金「CI・外部資源の面」に当たり、ステップ 1-8 の 6 つのコミットで CI を段階的に失敗させる確認が段階的なロールアウトに、秘密鍵の常時チェック（AC-09）がセキュリティゲートに当たり、さらにステップ 1-1 が `.node-version`（Node.js 24）と作業環境（`v26.4.0`）の差および設計書 3.8 の設定を持つ TypeScript の版の選定という未確定の判断を含むため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: manifest

**対象ファイル**
-   新設: `extension/static/manifest.json`、`extension/test/manifest.test.ts`

**タスク**
-   [ ] **ステップ 2-1**: `static/manifest.json` を設計書 3.1 の表のとおりに作る。`key` は設計書 3.12.1 の値をそのまま使い、鍵を生成し直さない。`background`・`action` が指すファイル（`background.js`・`popup.html`）はフェーズ 4 で作る。
    -   **対象:** `extension/static/manifest.json`。**完了:** `make ext-build` が通り、`manifest.json` が JSON として妥当である。
-   [ ] **ステップ 2-2**: `test/manifest.test.ts` を作る（設計書 3.11）。`permissions` が設計書 3.1 の 4 つとちょうど一致すること、宣言しない項目（設計書 3.1 の表の `host_permissions`・`optional_permissions`・`optional_host_permissions`・`content_scripts`・`content_security_policy`・`web_accessible_resources`・`externally_connectable`・`options_ui`・`commands`）がないこと、`manifest_version`・`minimum_chrome_version`・`background`（`type: "module"`）・`action.default_popup` が設計書 3.1 の値であること、`key` が RSA 2048 ビットの SubjectPublicKeyInfo として解析でき、そこから計算した拡張 ID が `clfmbbcdpnjcefbdihdoahomaifbabkk` であることを確かめる。拡張 ID を計算する関数は `test/helpers/` に置き、ステップ 5-6 でも使う。
    -   **対象:** `extension/test/manifest.test.ts`。**完了:** `extension/test/manifest.test.ts` が通る。
-   [ ] **ステップ 2-3**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `permissions` に `tabs` を足す、宣言しない項目のそれぞれを 1 つずつ足す、`manifest_version`・`minimum_chrome_version`・`background.type`・`action.default_popup` をそれぞれ変える、`key` の 1 文字を変える、`key` を PKCS#8 の秘密鍵の base64 に置き換える（一時的に生成し、コミットせず削除する）。`make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`manifest.test.ts` が通る。

### PR-2 作成ポイント: extension manifest and fixed ID

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `feat(0007): add the extension manifest with a fixed ID`

**レビュー観点**: `key` を設計書 3.12.1 の値から生成し直さず、そこから計算した拡張 ID が `clfmbbcdpnjcefbdihdoahomaifbabkk` になること（ステップ 2-1・2-2、AC-08・AC-09） / `permissions` が設計書 3.1 の 4 つとちょうど一致し、宣言しない項目がなく、CSP を変更しないこと（ステップ 2-2、AC-12・AC-26・AC-27） / `background`（`type: "module"`）・`action.default_popup`・`manifest_version`・`minimum_chrome_version` が設計書 3.1 の値であること（ステップ 2-2）

**実装モデル要件**: standard

**判定理由**: manifest の各値は設計書 3.1 と 3.12.1 で確定しており、`既存コード調査結果` に競合する実装方針の併記がなく、パネルモードの引き金・2 つ以上の Conditional check・隔離すべき高リスクなステップのいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: core

**対象ファイル**
-   新設: `extension/src/core/types.ts`・`collect.ts`・`messages.ts`・`summary.ts`、`extension/test/collect.test.ts`・`messages.test.ts`・`summary.test.ts`
-   変更: `extension/eslint.config.js`、`extension/test/lintRules.test.ts`

`acceptedUrl.ts` はステップ 1-4 で作成済みである。

**タスク**
-   [ ] **ステップ 3-1**: `core/types.ts` に設計書 3.2 の型を作る。
    -   **対象:** `extension/src/core/types.ts`。**完了:** `make ext-typecheck` が通る。
-   [ ] **ステップ 3-2**: `core/collect.ts` に `collect`・`CollectedInput`・`CollectOutcome`・`PageSelection`・`SelectionReader` を作る（設計書 3.2・3.3）。判定の順序、`documentUrl` の照合、`\p{White_Space}` による空白文字の判定、例外を投げないことは設計書 3.3 のとおりとする。`eslint.config.js` に、`core/collect.ts` とテスト以外での `CollectedInput` への型の表明の禁止を加え、`lintRules.test.ts` に、ほかのファイルの名前での `as CollectedInput` が違反になり、`core/collect.ts` の名前では違反にならない行を加える。
    -   **対象:** `extension/src/core/collect.ts`・`extension/eslint.config.js`・`extension/test/lintRules.test.ts`。**完了:** `make ext-typecheck` が通り、`extension/test/lintRules.test.ts` の `CollectedInput` の規則が通る。
-   [ ] **ステップ 3-3**: `core/messages.ts` に `rejectionMessage` を作る（設計書 3.5）。`switch` の `default` で `never` を受ける。
    -   **対象:** `extension/src/core/messages.ts`。**完了:** `make ext-typecheck` が通る。
-   [ ] **ステップ 3-4**: `core/summary.ts` に `summarize`・`parseSummary` を作る（設計書 3.5）。
    -   **対象:** `extension/src/core/summary.ts`。**完了:** `make ext-typecheck` が通る。
-   [ ] **ステップ 3-5**: テストを作る（設計書 3.11）。
    -   `collect.test.ts`: 判定の順序、`launch` が `undefined`、空白文字の各種（AC-16・AC-17 の各値、U+0085 を拒否し U+FEFF だけを受理する）、`read` の失敗、`documentUrl` の不一致、`title` が `undefined`、対象外のページで `read` が呼ばれないこと、前後の空白・空行が `CollectedInput` に残ること。
    -   `messages.test.ts`: 4 つの理由の `text` が互いに異なること、`steps` が `not-watch-page`・`empty-selection` にだけあること。
    -   `summary.test.ts`: `characterCount`（サロゲートペアを 1 と数える）、`lineCount`（`\r\n`・`\r`・`\n`、前後の空行）、各値の切り詰めの境界（上限ちょうどと 1 超え）とサロゲートペアを分けないこと、`*Truncated` の値。`parseSummary` が `summarize` の出力を受理し、項目の欠け・余分な項目・型の違い・未知の `kind`・未知の `reason` を拒否すること。
    -   **対象:** `extension/test/collect.test.ts`・`messages.test.ts`・`summary.test.ts`。**完了:** これらのテストが通る。
-   [ ] **ステップ 3-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `collect` の判定の順序を入れ替える、`documentUrl` の照合を外す、空白文字の判定を `\s` や `trim` に替える、`CollectedInput` に `trim` した値を入れる、`rejectionMessage` の 2 つの文言を同じにする、`summarize` の数え方を `length` に替える、`parseSummary` の項目の集合の検査を外す、`core/` のファイルから `document` を参照する（`tsconfig.core.json` の型検査で失敗すること）、`CollectedInput` の lint の規則を外す。`make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`collect`・`messages`・`summary` のテストが通る。

### PR-3 作成ポイント: browser-free core for collection and display

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0007): add the browser-free core for collection and display`

**レビュー観点**: 判定の順序（対象外のページ → 収集の失敗 → 選択範囲が空 → タイトルが空）と、対象外のページで選択範囲を読み取らないこと（ステップ 3-2、AC-15・AC-18・AC-19） / 収集した値を補正せず、前後の空白・空行を残すこと（ステップ 3-2、AC-14） / 空白文字の判定（`\p{White_Space}`。U+0085 を拒否し U+FEFF を受理する）と、サロゲートペアを 1 と数える文字数・行数（ステップ 3-2・3-4、AC-16・AC-17・AC-21） / 拒否の理由を型で表し、`rejectionMessage` の 4 つの文言が互いに異なること（ステップ 3-3、AC-22・AC-31）

**実装モデル要件**: standard

**判定理由**: `既存コード調査結果` に競合する実装方針の併記がなく、Conditional checks・パネルモードの引き金・隔離すべき高リスクなステップのいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 表示と経路の処理

**対象ファイル**
-   新設: `extension/src/ui/render.ts`、`extension/src/launch.ts`、`extension/src/browser/chromeDeps.ts`、`extension/src/background.ts`・`popup.ts`・`result.ts`、`extension/static/popup.html`・`result.html`・`style.css`、`extension/test/render.test.ts`・`launch.test.ts`・`chromeDeps.test.ts`
-   変更: `extension/scripts/check-dist.ts`、`extension/test/checkDist.test.ts`

**タスク**
-   [ ] **ステップ 4-1**: `ui/render.ts` の `renderSummary` と、`static/` の HTML・CSS を作る（設計書 3.7・3.13）。HTML はスクリプトを `<script type="module" src>` で読み、インラインのスクリプトを書かない。
    -   **対象:** `extension/src/ui/render.ts`・`extension/static/popup.html`・`extension/static/result.html`・`extension/static/style.css`。**完了:** `make ext-build` が通る。
-   [ ] **ステップ 4-2**: `launch.ts` に、依存の interface、`launchContextFromTab`、`runMenuLaunch`・`runPopupLaunch`・`runResultWindow` を作る（設計書 3.6）。`runMenuLaunch` の表示の失敗の扱いは設計書 3.6 と §1.4 の I-01 の対応のとおりとする。`Logger` には、最初の引数に固定のラベルだけを渡す。
    -   **対象:** `extension/src/launch.ts`。**完了:** `make ext-typecheck` が通る。
-   [ ] **ステップ 4-3**: `browser/chromeDeps.ts` に、依存の interface と `SelectionReader` の実装を作る（設計書 3.6）。注入する関数は、外の名前を参照しない 1 つの関数とする。`read` は、`executeScript` が失敗した場合、または戻り値の形が想定と違う場合に reject する。`SummaryStore.remove` は、削除の失敗で reject する（§1.4）。この契約を `launch.ts` の `SummaryStore.remove` の doc コメントに書く。
    -   **対象:** `extension/src/browser/chromeDeps.ts`。**完了:** `make ext-typecheck` が通る。
-   [ ] **ステップ 4-4**: エントリポイントを作る（設計書 2.1・3.1・6 章）。`background.ts` は、`onInstalled` で `contextMenus.removeAll()` の完了を待ってから項目を作り、`onClicked` のリスナーをモジュールの最上位で登録する。
    -   **対象:** `extension/src/background.ts`・`popup.ts`・`result.ts`。**完了:** `make ext-build` が通る。
-   [ ] **ステップ 4-5**: `check-dist` に manifest と HTML の参照の検査を加え（設計書 3.9）、`checkDist.test.ts` に、`background.service_worker`・`action.default_popup`・`<script src>`・`<link href>` のそれぞれが `dist/` にないファイルを指す場合を拒否する行を加える。
    -   **対象:** `extension/scripts/check-dist.ts`・`extension/test/checkDist.test.ts`。**完了:** `extension/test/checkDist.test.ts` が通る。
-   [ ] **ステップ 4-6**: テストを作る（設計書 3.11）。
    -   `render.test.ts`: jsdom の要素で、タイトルと選択範囲に `<img src=x onerror=alert(1)>` を含む要約を表示し、`img` 要素がなく、`textContent` に元の文字列があること。成功の場合にタイトル・URL・文字数・行数・プレビューのラベルと値があること。拒否の場合に `text` と、`steps` があるときだけ番号付きのリストがあること。
    -   `launch.test.ts`（収集）: 同じ fake のタブと `SelectionReader` で、2 つの経路が同じ収集の結果（ログに出す値）と同じ要約になること。収集に成功する入力と、拒否になる入力の両方で確かめる（AC-30）。`launchContextFromTab` がタブなし・`id` なしのタブで `undefined` を返し、どちらの経路も「収集の失敗」になること。`Logger` の最初の引数が、信頼できない文字列を含む入力でも固定のラベルであること。`info.selectionText` を使わないこと。前後の空白・空行が経路を通っても残ること。`put` の失敗でウィンドウを開かずバッジを表示すること。`windows.create` の失敗でバッジを表示し同じ鍵を `remove` すること。`windows.create` と `remove` が同時に失敗しても reject せず、バッジを表示し `log.error` に記録すること（I-01）。どの失敗でも reject しないこと。起動の開始でバッジを消すこと。
    -   `launch.test.ts`（表示）: `runPopupLaunch` と `runResultWindow` を jsdom の要素で実行し、`<img src=x onerror=alert(1)>` が要素にならないこと（AC-23）。`runPopupLaunch` が、動画ページでは収集した内容を、対象外のページでは理由と手順を表示すること（AC-28・AC-29 のユニットテストの部分）。`runResultWindow` が要約を読んだ後に消すこと、ハッシュがない・要約がない・形が違うときに固定の文言を表示すること。
    -   `chromeDeps.test.ts`: fake の `executeScript` が形の違う結果（`null`・`undefined`・文字列だけ・項目の欠け・項目の型の違い・空の配列）を返すか例外を投げると `read` が reject すること。注入する関数のソースを、外の名前を持たない環境（`node:vm` など。lint が禁止する `eval`・`new Function` は使わない）で評価し、fake の `window.getSelection` と `location` だけで期待する値を返すこと。
    -   **対象:** `extension/test/render.test.ts`・`launch.test.ts`・`chromeDeps.test.ts`。**完了:** これらのテストが通る。
-   [ ] **ステップ 4-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `renderSummary` の 1 か所を `textContent` から HTML を解釈する API に替える（lint を一時的に無効にして）、`runMenuLaunch` で `info.selectionText` を使う、`put` の失敗の後にウィンドウを開く、`remove` の失敗を `signalDisplayFailure` と同じ `try` に入れる、`runResultWindow` で `take` の代わりに読むだけにする、`parseSummary` を通さずに表示する、注入する関数から外の定数を参照する、`read` の結果の形の検査を外す、`check-dist` の参照の検査を外す。`make ext-check` → `make test` → `make lint` を通し、`dist/` を Chrome に読み込んでエラーがないことを確かめる。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録し、`make ext-check` を通す。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る。`make ext-build` の成果物を Chrome の「パッケージ化されていない拡張機能を読み込む」で読み込み、エラーなく読み込まれることを確かめる（ステップ 4-7 で行う）。

### PR-4 作成ポイント: rendering and launch paths

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0007): render results and wire the launch paths`

**レビュー観点**: 収集した文字列を `textContent` だけで表示し、`<img src=x onerror=alert(1)>` が要素として解釈されないこと（ステップ 4-1・4-6、AC-23） / 2 つの起動の経路が同じ `collect` と `renderSummary` を使い、同じ収集の結果と要約になること（ステップ 4-2、AC-30） / 表示の失敗の扱い（I-01）: `windows.create` と `SummaryStore.remove` の失敗を独立に捕まえ、`runMenuLaunch` が reject せずバッジを表示すること（ステップ 4-2・4-3、AC-13） / `check-dist` が manifest と HTML の参照先を検査し、注入する関数が外の名前を参照しないこと（ステップ 4-3・4-5、AC-04）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-2・4-3 が、`windows.create` と `SummaryStore.remove` の同時失敗をそれぞれ独立に捕まえて `runMenuLaunch` を reject させない表示の失敗の流れ（I-01）という、回復の流れ（高リスクなステップ）を含むため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 手動の確認と文書

**対象ファイル**
-   新設: `extension/test/docs.test.ts`
-   変更: `README.md`、`CLAUDE.md`、`docs/dev/project_overview.md`、`docs/dev/security.md`、本計画（§5.1）。加えて、ステップ 5-1 の手動の確認でメニューの項目が残らない場合だけ `extension/src/background.ts`（設計書 3.1 の代替の登録）

**タスク**
-   [ ] **ステップ 5-1**: 設計書 7.2 の手動の確認を、macOS の Chrome と Brave（作業時点の安定版）で行い、ブラウザの版・日付・結果を §5.1 に記録する。AC-10・AC-11 では、拡張の無効化と再有効化、ブラウザの再起動の後に項目が残ることも確かめる。残らない場合は、設計書 3.1 のとおり service worker のモジュールの最上位でも登録し、確認をやり直す。
    -   **対象:** `extension/src/background.ts`（代替の登録が必要な場合だけ。通常はなし）。**完了:** 設計書 7.2 の手動の確認の結果を §5.1 の該当行に記録する。
-   [ ] **ステップ 5-2**: `README.md` の `## Development` の下に、要件書 F-008 と設計書 3.13 の `README.md` の行に挙げた内容（Node.js の版の用意、`make ext-install`、ビルド、Chrome と Brave への読み込み、2 つの起動の手段、送信しないこと、コンソールの開き方、拡張 ID）を書く。
    -   **対象:** `README.md`。**完了:** `extension/test/docs.test.ts::documents` が通る。
-   [ ] **ステップ 5-3**: `CLAUDE.md` に、設計書 3.13 の `CLAUDE.md` の行の内容を書く。`ext-` のターゲットは `### Build Commands` などのコマンドの一覧に、変更後の確認（`make ext-check`）は `## Development Notes` に、依存パッケージの方針は `### Dependencies` に置く。あわせて、文書（README・CLAUDE.md・`docs/`）や `.gitignore` だけを変えるときも `make ext-test` を手元で実行することを書く（§6.1。AC-09 の秘密鍵の検査は常時実行の CI のジョブ `secret-scan` が担うので、ここには含めない）。
    -   **対象:** `CLAUDE.md`。**完了:** `extension/test/docs.test.ts::documents` が通る。
-   [ ] **ステップ 5-4**: `docs/dev/project_overview.md` の「言語: Go」（`:9`）に、ブラウザ拡張を TypeScript で書くことを加え、「想定ディレクトリ構成」（`:69`）に `extension/` を加える。
    -   **対象:** `docs/dev/project_overview.md`。**完了:** `extension/test/docs.test.ts::documents` が通る。
-   [ ] **ステップ 5-5**: `docs/dev/security.md` の §8 に、拡張の小節を見出し付きで加え（`docs.test.ts` が見出しでこの小節を見つける）、設計書 3.13 の `security.md` の行の (1)〜(5) と拡張 ID を書く。
    -   **対象:** `docs/dev/security.md`。**完了:** `extension/test/docs.test.ts::documents` が通る。
-   [ ] **ステップ 5-6**: `test/docs.test.ts` を作る。(1) `README.md` と `security.md` に、`manifest.json` の `key` から計算した拡張 ID が現れること（AC-25）。(2) `CLAUDE.md` に、`Makefile` が定義するすべての `ext-` のターゲットの名前が現れること。(3) `security.md` の拡張の節に、`manifest.json` の `permissions` の各値が現れること。(4) `project_overview.md` の「想定ディレクトリ構成」に `extension/` が現れ、「決定済みの方針」に `TypeScript` が現れること。(5) 設計書に 3.12.1〜3.12.3 の節があり、3.12.1 に拡張 ID が現れること（AC-24）。(6) 本計画の §5.1 の表で、手動の確認を伴う各 AC の行の結果の欄が空でないこと。
    -   **対象:** `extension/test/docs.test.ts`。**完了:** `extension/test/docs.test.ts` が通る。
-   [ ] **ステップ 5-7**: 文書の内容を実物と照合する。README の手順をまっさらな作業ディレクトリ（`git worktree` など）で上から実行してビルドと読み込みまで進むこと、README・CLAUDE.md のターゲットの説明が `Makefile` のレシピと一致すること、security.md の権限の理由が設計書 3.1 の表と一致することを確かめ、§5.1 に記録する。
    -   **対象:** なし（確認のみ）。**完了:** README の手順を空の作業ディレクトリで実行してビルドと読み込みまで進み、結果を §5.1 に記録する。
-   [ ] **ステップ 5-8**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: README の拡張 ID の 1 文字を変える、CLAUDE.md から `ext-` のターゲットを 1 つ消す、security.md から権限の名前を 1 つ消す、§5.1 の結果の欄を 1 つ空にする、`project_overview.md` から `extension/` を消す、`project_overview.md` の「決定済みの方針」から `TypeScript` を消す、設計書の 3.12.2 の見出しを消す、設計書 3.12.1 の拡張 ID の 1 文字を変える（いずれも確認の後に戻す）。`make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、§5.1 のすべての行が記録され、`docs.test.ts` が通る。

### PR-5 作成ポイント: manual verification and documentation

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8

**推奨タイトル**: `docs(0007): record the manual checks and document the extension`

**レビュー観点**: README・CLAUDE.md・`project_overview.md`・`security.md` が要件書 F-008 と設計書 3.13 のとおりで、固定した拡張 ID と権限の理由を含むこと（ステップ 5-2〜5-5、AC-25） / `docs.test.ts` が機械的に確かめられる契約値だけを固定し、§5.1 の手動の確認の記録の有無を確かめること（ステップ 5-6、AC-24・AC-25） / §5.1 の手動の確認（Chrome・Brave）のブラウザの版・日付・結果が記録されていること（ステップ 5-1・5-7） / README の手順が実物（`Makefile` のレシピ、設計書 3.1 の権限の表）と一致すること（ステップ 5-7） / ステップ 5-1 の代替の登録で `background.ts` を変えた場合、設計書 3.1 の登録（`onInstalled` とモジュールの最上位）にとどまり、ほかの振る舞いを変えないこと

**実装モデル要件**: standard

**判定理由**: 文書の作成・文書のガードのテスト・手動の確認に限られ、`既存コード調査結果` に競合する実装方針の併記がなく、Conditional checks・パネルモードの引き金・隔離すべき高リスクなステップのいずれにも該当しないため。ステップ 5-1 の service worker のモジュールの最上位での再登録は、設計書 3.1 が定めた手順で、新しい設計の判断を伴わないので、回復の流れの引き金には当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1: 開発環境 | フェーズ 1 | `make ext-check` が通り、Go の手順の結果が変わらない（§5.1 の AC-05 の記録）。PR の CI で拡張のジョブが通り、AC-07 の 6 つの変更で CI が失敗する（§5.1） |
| M2: manifest | フェーズ 2 | `manifest.test.ts` が通る |
| M3: core | フェーズ 3 | `collect`・`messages`・`summary` のテストが通る |
| M4: 経路の処理 | フェーズ 4 | `make ext-build` の成果物を Chrome に読み込める。`render`・`launch`・`chromeDeps` のテストが通る |
| M5: 確認と文書 | フェーズ 5 | §5.1 の手動の確認の記録がそろい、`docs.test.ts` が通る |

### 3.2. PR 構成

PR はフェーズと 1 対 1 に対応させる（PR-1〜PR-5）。各 PR は主たる関心事（拡張の開発環境と CI / manifest と拡張 ID / `core/` の判定と要約 / 表示と経路の処理 / 手動の確認と文書）を持ち、単独でグリーンゲートを通せる単位とする。本タスクの各 PR のグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make ext-check` を加えた `make ext-check` → `make test` → `make lint`（各フェーズの完了条件と同じ）とする。`make test`・`make lint` は Go のソースだけを対象にし、拡張の成果物を確かめないためである。フェーズ 1 のテスト（`typecheck.test.ts`・`checkDist.test.ts`・`checkLockfile.test.ts`・`repository.test.ts`・`ciChanges.test.ts`・`acceptedUrl.test.ts`）は、ステップ 1-1〜1-3 の設定・スクリプト・`Makefile`・Go の設定と、ステップ 1-6 の CI と `has-extension-changes.sh` を確かめ、`acceptedUrl` は 6 つのステップを通す対象になる。そのため、これらの実装とテストを 1 つの PR にまとめる。分けると、`repository.test.ts`・`ciChanges.test.ts` が相手の PR で足す `ci.yml` を参照し、6 つのステップを実行する `acceptedUrl` のテストも相手の PR の成果に依存して、片方のグリーンゲートが通らなくなる。

ステップは並べ替えていないので、ステップ番号の順と文書の順は一致し、各 `### PR-N 作成ポイント` は直前のフェーズの完了条件の後にある。フェーズ 4 の表示の失敗の流れ（I-01）は `launch.ts` と `chromeDeps.ts` の中だけにあり、専用の PR には分けない。`launch.ts` を使うエントリポイント（ステップ 4-4）と `check-dist` の参照の検査（ステップ 4-5）が `launch.ts` より後に来るため、この流れをフェーズ 4 の最後のステップには置けないからである。同じ PR の中で、単純なレンダリング（ステップ 4-1）の直後に置く。`core/` の判定（PR-3）は、それを使う表示と経路の処理（PR-4）に先行する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | 拡張の設定・スクリプト・`Makefile` の `ext-` ターゲット・`go.mod`/`.gitignore`/`.pre-commit-config.yaml`・`acceptedUrl` と、ガードのテスト・CI のジョブ | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 | `manifest.json`（固定した `key` と権限）と `manifest.test.ts` | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | `core/` の型・`collect`・`messages`・`summary` とそのテスト | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | `render`・`launch`・`chromeDeps`・エントリポイント・HTML と CSS、経路の表示の失敗の扱い | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 | 手動の確認の記録と文書（README・CLAUDE.md・`project_overview.md`・`security.md`）と `docs.test.ts`、必要なら `background.ts` の代替の登録 | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。フェーズ 1 で、CI と成果物の検査を最初に用意し、以後のフェーズの変更がすべて同じ検査を通るようにする。フェーズ 2 の manifest が参照する `background.js`・`popup.html` はフェーズ 4 で作るので、`check-dist` の参照の検査はフェーズ 4（ステップ 4-5）で加える。手動の確認は、すべての経路がそろうフェーズ 5 で行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 3.11・7.1 のとおり、`node --test` で、fake の依存と jsdom を使う。実際のブラウザを起動しない。テストの文字列は合成したものだけを使い、YouTube のページの実データを加えない（要件書 5.）。
-   判定の網羅は `core/` のテスト（`acceptedUrl`・`collect`・`summary`）で行う。`launch.test.ts` は経路の配線（同じ `collect` を通ること、表示の失敗の扱い、表示の手段）を確かめ、`core/` のテストの行の表を繰り返さない。
-   設定と文書のガード（`typecheck`・`lintRules`・`repository`・`ciChanges`・`docs`）は、子プロセス（`tsc`・`git`・`bash`）やファイルの読み取りを使う。どれもネットワークにアクセスしない。
-   AC-03 は実際のインストール（レジストリへのアクセスを伴い、`npm ci` が lockfile と `package.json` の食い違いで失敗する）に依存するため、ユニットテストにしない。`repository.test.ts` が、Makefile と CI が `npm ci` を `--ignore-scripts` 付きで呼び、`npm install` を呼ばないこと、lockfile が追跡されていることを静的に確かめ、実際の失敗はステップ 1-8 の CI のコミットで確かめる。AC-05 も Go の手順全体の結果の比較でありユニットテストになじまないため、CI の `go list ./...` の確認とステップ 1-7 の比較で確かめる。どちらも `static`・`manual` の検証である。`requirements_process.md` §4 は `static` だけの検証を文書の記述の有無に限っているので、これはその例外であり、本計画のレビューで判断する。

### 4.2. 統合テスト

ブラウザを起動する自動テストは設けない（要件書 5.）。設計書 7.2 の手動の確認をステップ 5-1 で行い、§5.1 に記録する。

### 4.3. 後方互換性

-   Go のソースとテストを変更しない。Go の手順の結果が変わらないことは、ステップ 1-7 の比較と、CI の拡張のジョブの `go list ./...` の確認で確かめる（AC-05）。
-   既存の Go のテストのうち `Makefile`・`ci.yml`・`.pre-commit-config.yaml` を読むもの（§1.3）は、ステップ 1-7 の `make test` で通ることを確かめる。

### 4.4. 網羅率

`core/`・`launch.ts`・`ui/render.ts`・`browser/chromeDeps.ts` の `SelectionReader` の実装は、ユニットテストで実行する。`chromeDeps.ts` のそのほかの実装とエントリポイントは、設計書 3.11 のとおりユニットテストせず、手動の確認で確かめる。フェーズ 4 の完了時に `node --test --experimental-test-coverage` の出力で、ユニットテストの対象のファイルに実行されない分岐が残っていないかを確かめ、残る分岐をコミットメッセージに挙げる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベル: `test` は失敗しうる実行のテストまたは `make` のターゲット、`static` は設定・文書・記録を確かめるテストや CI のステップ、`manual` は手動の確認である。テストの場所は `extension/test/<file>::<describe の名前>` で示す。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `make ext-check`（CI のジョブ `extension` の 6 つのステップ） | ステップ 1-1〜1-3・1-6 |
| AC-02 | test・manual | `extension/test/typecheck.test.ts::type errors fail typecheck and build`、ステップ 1-8 の型検査の変更での CI の失敗 | ステップ 1-1・1-5 |
| AC-03 | static・manual（理由は §4.1） | `extension/test/repository.test.ts::install uses npm ci`、ステップ 1-8 の版の範囲の変更での CI の失敗 | ステップ 1-3・1-5 |
| AC-04 | test | `extension/test/checkDist.test.ts::checkDist`、`make ext-build`（`check-dist` を毎回実行する） | ステップ 1-2・1-5・4-5 |
| AC-05 | static・manual（理由は §4.1） | CI のジョブ `extension` の `go list ./...` のステップ、ステップ 1-7 の比較の記録（§5.1） | ステップ 1-3・1-6・1-7 |
| AC-06 | test | `extension/test/repository.test.ts::build outputs are ignored` | ステップ 1-3・1-5 |
| AC-07 | test・static・manual | `extension/test/ciChanges.test.ts::has-extension-changes`、`extension/test/repository.test.ts::ci runs every extension step`、ステップ 1-8 の CI の記録（§5.1） | ステップ 1-6・1-8 |
| AC-08 | test・manual | `extension/test/manifest.test.ts::key`（拡張 ID の計算）、ステップ 5-1（§5.1） | ステップ 2-1・2-2 |
| AC-09 | test | `extension/test/manifest.test.ts::key`（公開鍵としての解析）、`extension/test/repository.test.ts::no private key is tracked`、CI の常時実行のジョブ `secret-scan`。pre-commit の `detect-private-key` は補助 | ステップ 1-3・1-5・1-6・2-2 |
| AC-10 | static・manual | `extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1） | ステップ 2-1・4-4 |
| AC-11 | static・manual | `extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1） | ステップ 2-1・4-4 |
| AC-12 | test | `extension/test/manifest.test.ts::undeclared keys` | ステップ 2-1・2-2 |
| AC-13 | static・manual | `extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1）。値を補正しない経路は AC-14 のテストが確かめる | ステップ 3-2・4-2・4-3 |
| AC-14 | test | `extension/test/collect.test.ts::collect`（前後の空白・空行）、`extension/test/launch.test.ts::launch paths`（経路を通った値） | ステップ 3-2・4-2 |
| AC-15 | test | `extension/test/acceptedUrl.test.ts::isAcceptedWatchUrl` | ステップ 1-4 |
| AC-16 | test | `extension/test/collect.test.ts::collect`（空白文字の各種） | ステップ 3-2 |
| AC-17 | test | `extension/test/collect.test.ts::collect`（タイトルの空白文字） | ステップ 3-2 |
| AC-18 | test | `extension/test/collect.test.ts::collect`（`read` の失敗）、`extension/test/chromeDeps.test.ts::selection reader` | ステップ 3-2・4-3 |
| AC-19 | test | `extension/test/collect.test.ts::collect`（対象外のページで `read` を呼ばない） | ステップ 3-2 |
| AC-20 | test・static・manual | `extension/test/collect.test.ts::collect`（`documentUrl` の不一致）、`extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1） | ステップ 3-2・4-2 |
| AC-21 | test・manual | `extension/test/summary.test.ts::summarize`、`extension/test/render.test.ts::renderSummary`、ステップ 5-1（§5.1） | ステップ 3-4・4-1 |
| AC-22 | test・manual | `extension/test/messages.test.ts::rejectionMessage`、ステップ 5-1（§5.1） | ステップ 3-3 |
| AC-23 | test | `extension/test/render.test.ts::renderSummary`、`extension/test/launch.test.ts::display`、`extension/test/lintRules.test.ts::html parsing APIs` | ステップ 1-1・4-1・4-2 |
| AC-24 | static | `extension/test/docs.test.ts::investigation is recorded` | 設計書 3.12（作成済み） |
| AC-25 | static・manual | `extension/test/docs.test.ts::documents`、ステップ 5-7 の照合（§5.1） | ステップ 5-2〜5-5 |
| AC-26 | test | `extension/test/manifest.test.ts::permissions` | ステップ 2-1・2-2 |
| AC-27 | test | `extension/test/manifest.test.ts::undeclared keys`（CSP）、`extension/test/lintRules.test.ts::code from strings`、`make ext-lint` | ステップ 1-1・2-2 |
| AC-28 | test・static・manual | `extension/test/launch.test.ts::display`（動画ページのポップアップ）、`extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1） | ステップ 4-2 |
| AC-29 | test・static・manual | `extension/test/launch.test.ts::display`（対象外のページのポップアップ）、`extension/test/docs.test.ts::manual checks are recorded`、ステップ 5-1（§5.1） | ステップ 4-2 |
| AC-30 | test | `extension/test/launch.test.ts::launch paths` | ステップ 4-2 |
| AC-31 | test・manual | `extension/test/messages.test.ts::rejectionMessage`、`extension/test/render.test.ts::renderSummary`、ステップ 5-1（§5.1） | ステップ 3-3・4-1 |

### 5.1. 手動の確認と記録

実装のときに埋める。`docs.test.ts::manual checks are recorded`（ステップ 5-6）は、手動の確認を伴う AC の行の「結果」が空でないことを確かめる。

| AC | 確認の内容（設計書 7.2・7.3） | ブラウザ・環境 | 日付 | 結果 |
|---|---|---|---|---|
| AC-02 | 型の合わない代入のコミットで CI の型検査のステップが失敗する | CI | | |
| AC-03 | 版の範囲の変更のコミットで CI のインストールのステップが失敗する | CI | | |
| AC-05 | 拡張がある状態とない状態で、`make test`・`make lint`・`make deadcode`・`make build` の成否と `go list ./...` の出力が同じ | macOS | | |
| AC-07 | 本タスクの PR の CI が通る。6 つの変更のコミットで、それぞれ対応するステップが失敗する | CI | | |
| AC-08 | 2 つのディレクトリから読み込んだ拡張 ID が記録した値と一致する | Chrome・Brave | | |
| AC-10・AC-11 | 動画ページで項目が現れ、`https://example.com/` で現れない。無効化と再有効化、再起動の後も現れる | Chrome・Brave | | |
| AC-13 | コンソールの選択範囲の文字列が `window.getSelection().toString()` と一致し、時刻の行と本文の行が改行で区切られている | Chrome・Brave | | |
| AC-20 | ページ内の移動の後、項目が現れ、両方の経路で移った先の URL とタイトルが収集される | Chrome・Brave | | |
| AC-21・AC-22・AC-31 | 両方の経路で、成功・「対象外のページ」・「選択範囲が空」の表示 | Chrome・Brave | | |
| AC-28・AC-29 | ポップアップの表示と、閉じた後の選択範囲。`https://example.com/` での表示 | Chrome・Brave | | |
| AC-25 | README の手順の実行、文書と `Makefile`・設計書 3.1 の照合（ステップ 5-7） | macOS | | |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 作業環境の Node.js（`v26.4.0`）が `.node-version`（Node.js 24）と異なる | 拡張のターゲットが版の確認で失敗する | 版の管理のツールで `.node-version` の版を用意する（§1.2）。版の確認を緩めない |
| 実装の時点の TypeScript の最新の版が、設計書 3.8 の設定（`rewriteRelativeImportExtensions`・`erasableSyntaxOnly` など）を持たないか、意味を変えている | 型検査やビルドが設計どおりに動かない | ステップ 1-1 で、選んだ版の公式の文書でこれらの設定を確かめてから固定する。合わない場合は、設定を持つ版を選ぶ |
| `node --test` が既定で `.ts` のテストのファイルを見つけない版がある | テストが 0 件で成功する | npm のスクリプトでテストのファイルのパターンを明示し、ステップ 1-7 で、テストのファイルの 1 つを失敗させて `make ext-test` が失敗することを確かめる |
| golangci-lint が `go.mod` の `ignore` の指示を扱えない | `make lint` が失敗するか、`extension/` を解析する | ステップ 1-7 の `make lint` と CI の `lint` のジョブで確かめる。失敗した場合は作業を止め、設計書 3.10 の見直しを利用者に相談する |
| `extension/test/` のガードは拡張のジョブでだけ実行される。そのため、`has-extension-changes` の条件（設計書 3.10）に該当しない PR では、`docs.test.ts`（AC-24・AC-25・§5.1）と `repository.test.ts` の AC-06 の検査が CI で実行されない。該当しない PR の例は、`README.md`・`CLAUDE.md`・`docs/`・`.gitignore` だけを変える PR である | 文書と拡張 ID の食い違い、`.gitignore` の拡張の行の削除 | 残るリスクとして受け入れる。文書・`.gitignore` を変えるときは `make ext-test` を手元で実行することを CLAUDE.md に書く（ステップ 5-3）。AC-09 の秘密鍵の検査は常時実行のジョブ `secret-scan`（ステップ 1-6）が担うので、このリスクに含めない。CI の条件は設計書のとおりとし、変えない |
| Brave の安定版が手元にない、または Chrome と振る舞いが違う | 手動の確認が終わらない | 違いを §5.1 に記録し、利用者に報告する。設計書 3.12 の調査で Brave 1.96 の動作は確かめている |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| ステップ 1-8 の AC-07 の確認は、6 つのコミットのそれぞれで CI の完了を待つ | フェーズ 2 以降の作業は別のブランチで並行して進め、CI の結果を待つ間に止めない |
| 手動の確認（ステップ 5-1）は、利用者のブラウザでの操作を必要とする | フェーズ 5 の文書の作業を先に進め、確認の結果を待つ間に止めない |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8）
-   [ ] §5 のすべての AC の検証が通り、§5.1 の記録がそろっている

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の `test`・`static` の検証がすべて通る。§5.1 のすべての行が記録されている。
-   **品質:** `make ext-check`・`make test`・`make lint` が通る。各テストと lint の規則が、壊して失敗することを確かめ、コミットメッセージに記録している。§4.4 の網羅率を確かめている。
-   **セキュリティ:** AC-09・AC-12・AC-23・AC-26・AC-27 のテストが通る。`check-lockfile` と `check-dist` が `make ext-install`・`make ext-build` の中で実行される。
-   **文書:** ステップ 5-7 の照合を終え、`docs.test.ts` が通る。

## 9. 次のステップ (Next Steps)

-   PR の境界は §2・§3.2・§7 に埋め込み済みである（2026-10-08）。`/runplan 0007` で PR-1 から実装する。
-   実装の完了の後、設計書 3.12 の調査の結果（拡張 ID、`Origin` の観測、共有トークンの保存先）を #109 の API の定義の入力として渡す。
