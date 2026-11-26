# 要件定義書：`/mergemain` は拡張の検査の前に `make ext-install` で依存を入れる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/mergemain` が拡張の検査（`make ext-check`）を走らせるときに、その前に `make ext-install` で拡張の依存を入れるための要件を定義する。

**背景:**

新しく作った git の worktree には、ふつう `extension/node_modules` がない。今の `/mergemain` の振る舞いは次のとおりである。

1.  `/mergemain` は、merge コミットの差分（`git diff --name-only <base-head> HEAD`）に `extension/` の下か、`README.md`・`README.en.md`・`docs/dev/guides/development.md` があれば、`make test` と `make lint` の後に `make ext-preflight` を走らせ、通れば `make ext-check` を走らせる（`.claude/commands/mergemain.md:251-267`）。
2.  `make ext-preflight` は、Node.js と npm の版が決めた版と合うことと、`extension/node_modules` があることを確かめる。どちらかが欠ければ失敗する（`Makefile:407-418`）。失敗すると、`/mergemain` は作業の前の状態に戻し、「検査の準備がない」で止まる（`.claude/commands/mergemain.md:264-265`）。
3.  `/mergemain` は `make ext-install` を走らせない。禁止事項の 9 と手順 7 がそう定めている（`.claude/commands/mergemain.md:78-80`、`.claude/commands/mergemain.md:264`）。承認の節は、依存を入れることを承認しないと書いている（`.claude/commands/mergemain.md:27-28`）。根拠は、[0115 要件定義書](../0115_chief_conflict_resolution/01_requirements.md) の AC-26 と、「依存を入れる `make ext-install` はネットワークを使い、`/mergemain` の承認の外にある」という決定である（同書の「要件定義の際に決めたこと（2026-11-19）」）。
4.  `make ext-install` は、Node.js と npm の版を確かめ、`check-lockfile` でロックファイルの取得先が `registry.npmjs.org` だけであることを確かめてから、`npm ci --ignore-scripts --registry=https://registry.npmjs.org/ --replace-registry-host=never` を走らせる（`Makefile:420-428`）。
5.  `mergemain-restore` は、無視されたファイルと追跡していないファイルを消さない（`internal/mergepr/mergemain_restore.go:32-34`）。

そのため、Go や文書だけを変える PR でも、`main` の側で `README.md` か `extension/` が変わっていれば、衝突の解消・README の同期・merge コミット・`make test`・`make lint` をすべて終えた後の手順 7 で `make ext-preflight` が失敗する。`/mergemain` は全体を戻して止まり、chief の自動送信の目的（人が手を入れずに、衝突の解消から push まで終えること）を果たせない。

対応 issue: [#1183](https://github.com/isseis/yt2column/issues/1183)（関連: [#1019](https://github.com/isseis/yt2column/issues/1019)（タスク 0115。`/mergemain` と chief の解消の自動送信））

本書は次のことを決める。

-   `/mergemain` が `make ext-install` を走らせる条件と、その失敗の扱い。
-   `/mergemain` の承認の範囲と禁止事項の変更。
-   止まって戻したときの `extension/node_modules` の扱い。
-   報告に足す項目。

次のことは `02_architecture.md` が決める（[design_handoff.md](design_handoff.md)）。

-   `make ext-install` を人の許可の確認なしに走らせる仕組み（`.claude/settings.json` の規則を足すかどうか）。
-   手順 7 に `make ext-preflight` を残すかどうかと、手順の書き方。

### issue #1183 で決めたこと

以下の F- と AC- の番号は、3 章の機能要件を指す。

-   `/mergemain` の承認の範囲に `make ext-install` を含め、拡張の検査の前に依存を入れられるようにする（F-001・F-002）。 新しい worktree では依存がないのがふつうで、入れなければ、拡張に関わる取り込みのたびに人の手が要るためである。
    -   ADR：[ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（下の「要件定義の際に決めたこと」の ADR の項を参照）。
-   Node.js と npm の版が合わないときなど、`make ext-install` で直せないときと、`make ext-install` 自身が失敗したときは、今までどおり作業の前の状態に戻し、「検査の準備がない」で止まる（AC-03）。 直せない準備の不足を、検査の失敗と区別して人に示すためである。
    -   ADR：ADR-0046 の同じ更新に含める。

### 要件定義の際に決めたこと（2026-11-26）

-   進め方は通常の進め方（`full`）とする。除外の判断：この変更は、簡易な進め方から除外される変更に当たる（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人に尋ねずに送られる `/mergemain` に、ネットワークから依存を取得して入れることを許すので、開発の道具の信頼の境界（ADR-0046 の「背景」、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）に触れる。 人の答え：通常の進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   PR のブランチが `extension/package.json`・`extension/package-lock.json`・`extension/scripts/check-lockfile.ts`・`Makefile` などを変えていても、条件を付けずに `make ext-install` を走らせ、レビューされていない取得の規則で依存を入れうることを、残る危険として受け入れる（AC-01、4.2）。 今の `/mergemain` も、`make test` と `make ext-check` で、merge の後のツリー（PR のブランチの `Makefile` と Go・TypeScript のコード）を利用者の権限で走らせており、`make ext-install` で増えるのは、ロックファイルが指すパッケージの取得だけだからである。依存を取得する規則のファイルが merge の後のツリーで `main` と同じときだけ入れる案は採らなかった。`Makefile` を変える PR で止まりやすくなり、得る安全が小さいためである。 人の答え：条件を付けずに許し、危険を受け入れる。
    -   ADR：ADR-0046 を更新する（「結果」に残る危険を足す）。
-   `extension/node_modules` があるときも、`make ext-check` が要る取り込みでは、毎回 `make ext-install` を走らせる（AC-01）。 `main` の側でロックファイルが変わると、ある `node_modules` は古くなり、`make ext-check` が依存の食い違いで失敗しうるためである。`node_modules` がないか、merge コミットの差分に `extension/package.json` か `extension/package-lock.json` があるときだけ入れる案は採らなかった。ブランチの側で古くなった場合を見落とし、依存の有無と新しさを推し量る規則が増えるためである。毎回ネットワークから取得する時間は受け入れる。 人の答え：古い `node_modules` も対象に含め、`make ext-check` が要るときは毎回入れる。
    -   ADR：ADR-0046 の同じ更新に含める。
    -   この決定は、0115 要件定義書の AC-28 の「無視されたファイルと、始める前からあった追跡していないファイルは消さない」の例外である（AC-09）。`npm ci` は、始める前からあった `extension/node_modules` を消して作り直す。
-   止まって戻すとき、`/mergemain` が入れた `extension/node_modules` は消さずに残す（AC-08）。 無視されたファイルなので、戻した後の PR のブランチと作業ツリーの追跡しているファイルには影響せず、次の検査で使い回せるためである。消すには、`mergemain-restore` が行わない削除が要る。この決定は、0115 要件定義書の AC-28 の「`/mergemain` が作ったファイルが残っていない」の例外である。 人の答え：残す。
    -   ADR：ADR-0046 の同じ更新に含める。
-   `make ext-install` を走らせたことと、`make ext-install` が失敗したときに `extension/node_modules` が欠けているか不完全でありうることを、最後の報告に書く（AC-06・AC-03）。 `npm ci` は始める前の `node_modules` を消すので、途中で失敗すると、人が手で入れ直す必要があるためである。
    -   ADR：なし。報告の項目は本タスクの中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `extension/node_modules` のない worktree でも、拡張に関わる取り込みを `/mergemain` が人の手を借りずに解消し、push まで終えられるようにする。
-   `main` の側でロックファイルが変わった取り込みでも、`make ext-check` を merge の後のロックファイルの依存で走らせる。

### 2.2. スコープ (In Scope)

-   `.claude/commands/mergemain.md` の承認の節、禁止事項、手順 7、止まる理由の対応、報告の項目。
-   CLAUDE.md の「Tool Execution Safety」の `/mergemain` の承認の範囲。
-   `make ext-install` を人の許可の確認なしに走らせるための設定（要否と形は設計で決める）。
-   [ADR-0046](../../dev/adr/0046-chief-of-staff.md) の `/mergemain` の承認と残る危険の更新（本書の PR で行う）。

### 2.3. スコープ外 (Out of Scope)

-   `Makefile` の `ext-install`・`ext-preflight`・`ext-check` の中身。 今の `ext-install` の守り（版の確かめ、`check-lockfile`、`--ignore-scripts`、取得先の固定）をそのまま使うためである。
-   `mergepr` の取り込みの道具（`mergemain-*`）と、その判定。 依存を入れることは `make` の責務で、取り込みの判定は変わらないためである。
-   chief の自動送信の条件と、人が選んで送る条件。 `/mergemain` の中の手順だけが変わるためである。
-   `make ext-check` が要るかどうかの条件（差分のパスの規則）。0115 要件定義書の AC-26 のままとする。
-   `/mergemain` のほかの検査（`make test`・`make lint`）と、取り込みの規則。
-   `/mergemain` 以外のコマンドで `make ext-install` を走らせること。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **拡張の検査が要る取り込み:** merge コミットの差分（`git diff --name-only <base-head> HEAD`）に、`extension/` の下のパスか、`README.md`・`README.en.md`・`docs/dev/guides/development.md` がある取り込み（0115 要件定義書の AC-26）。
-   **依存を入れる:** `make ext-install` を走らせること。

### 3.2. 機能一覧

#### F-001: 拡張の検査の前に依存を入れる

`/mergemain` は、拡張の検査が要る取り込みでは、`make ext-check` の前に `make ext-install` を走らせる。

**Acceptance Criteria**:

-   **AC-01**: 拡張の検査が要る取り込みでは、`make test` と `make lint` が通った後、`make ext-check` の前に、`make ext-install` を走らせる。`extension/node_modules` がないときも、あるときも走らせる。PR のブランチが `extension/package.json`・`extension/package-lock.json`・`extension/scripts/check-lockfile.ts`・`Makefile` を変えていても、それを理由に止まらない。
-   **AC-02**: 拡張の検査が要らない取り込みでは、`make ext-install` を走らせない。
-   **AC-03**: `make ext-install` が失敗したとき（Node.js か npm の版が合わない、`check-lockfile` が拒む、`npm ci` が失敗する、ネットワークにつながらないなど）は、`make ext-check` を走らせず、作業の前の状態に戻して、理由の種類「検査の準備がない」で止まる。報告の詳細には、`make ext-install` が失敗したことと、`extension/node_modules` が欠けているか不完全でありうるので、原因を直した後に人が `make ext-install` を走らせる必要があることを書く。
-   **AC-04**: `make ext-install` が通った後に `make ext-check` が失敗したときは、今までどおり、作業の前の状態に戻して、理由の種類「検査の失敗」で止まる。
-   **AC-05**: chief が人に尋ねずに送った `/mergemain`（Claude Code の auto mode）でも、`make ext-install` を走らせるときに、人の許可の確認を待って止まらない。
-   **AC-06**: 「解消した」の報告には、`make ext-install` を走らせたかどうかを書く。

#### F-002: 承認の範囲と禁止事項

`/mergemain` の承認に `make ext-install` を含め、禁止事項から外す。

**Acceptance Criteria**:

-   **AC-07**: CLAUDE.md の「Tool Execution Safety」の `/mergemain` の項と、`.claude/commands/mergemain.md` の承認の節は、`/mergemain` の実行が、その PR について、拡張の検査が要る取り込みでの `make ext-install`（`registry.npmjs.org` からの依存の取得を含む）を承認することを書く。CLAUDE.md の差分は、この項に `make ext-install` の承認を足すことに限る。
-   **AC-08**: `.claude/commands/mergemain.md` の禁止事項は、`make ext-install` を禁じない。`.claude/commands/mergemain.md` の差分は、承認の節、禁止事項の 9、手順 7、止まる理由の対応の「検査の準備がない」の行、報告の項目のうち、`make ext-install` とその失敗に関わる記述に限る。

#### F-003: 止まったときの `extension/node_modules`

`/mergemain` が入れた依存は、止まって戻した後も残る。

**Acceptance Criteria**:

-   **AC-09**: `make ext-install` を走らせた後に止まって戻したとき、`extension/node_modules` は消さない。戻した後の PR のブランチと作業ツリーは、`extension/node_modules` を除き、0115 要件定義書の AC-28 の状態になる。
-   **AC-10**: `make ext-install` が、始める前からあった `extension/node_modules` を消して作り直すことは、0115 要件定義書の AC-28 の「無視されたファイルは消さない」に反しない。この例外は `extension/node_modules` に限る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   拡張の検査が要る取り込みでは、`npm ci` の分だけ `/mergemain` の時間が延びる。毎回ネットワークから取得することを受け入れる（「要件定義の際に決めたこと」）。

### 4.2. セキュリティ (Security)

-   `docs/dev/security.md` の発動条件（`yt-dlp` の起動、API キー、キャッシュ、LLM API への送信、信頼できない文字列のプロンプトか記事への埋め込み）には当たらない（N/A）。`docs/dev/security.md` の「`npm ci --ignore-scripts` でインストールする」規則は変えない。
-   開発の道具の信頼の境界について、次を残る危険として受け入れる。
    -   PR のブランチはレビューの前なので、ブランチがロックファイル・`check-lockfile`・`Makefile` を変えていれば、`make ext-install` はレビューされていない規則で依存を取得する。今の `make test` と `make ext-check` も、merge の後のツリーのコードを利用者の権限で走らせており、増えるのはロックファイルが指すパッケージの取得である。
    -   取得したパッケージのインストール時のスクリプトは、`main` の `Makefile` の `--ignore-scripts` が保たれる限り走らない。ブランチが `Makefile` を変えれば、この守りも外れうる。
-   この危険を ADR-0046 の「結果」に書く（本書の PR で更新する）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `make ext-install` の途中の失敗では、`extension/node_modules` が欠けるか不完全なまま残りうる。人が報告を読んで入れ直す（AC-03）。

### 4.4. 互換性 (Compatibility)

-   chief が送る `/mergemain` は、ブランチの `mergemain.md` が `origin/main` と同じときだけ送られる（ADR-0046）。変更は `main` に入った後に送られる `/mergemain` から効く。

### 4.5. 保守性 (Maintainability)

-   依存の入れ方の正本は `Makefile` の `ext-install` に置いたままとし、`mergemain.md` は `make ext-install` を呼ぶだけにする。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   CLAUDE.md の「Tool Execution Safety」と、`docs/dev/conventions/dev_tools.md` の Rule 5 に従う。エージェントは開発の道具を入れない（`make ext-install` が入れるのは拡張の開発用の npm パッケージで、開発の道具ではない）。
-   0115 要件定義書の AC-26・AC-28 の、本書が例外とする部分のほかは変えない。

## 6. 用語集 (Glossary)

-   **拡張の検査が要る取り込み:** 3.1 の定義による。
-   **依存を入れる:** 3.1 の定義による。
-   用語は `docs/translation_glossary.md` と統一する。
