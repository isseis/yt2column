# 要件定義書：文書の言語を読み手で決める方針への整理と、README の日英二言語化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、リポジトリの文書の言語を「誰が書き、誰が承認・利用する文書か」で決まる原則にそろえるための要件を定義する（[#537](https://github.com/isseis/yt2column/issues/537)）。あわせて、`docs/dev/developer_guide/` を規約と手引きの 2 つのディレクトリに分ける。また、README から開発者向けの内容を分け、README を日本語の正本と英語版の二言語で持つ。

**背景:**

文書の言語は、CLAUDE.md の "Documents" 節で、場所ごとの例外を並べて決めている（「Default language is Japanese (exceptions: README.md, CLAUDE.md, docs/dev/developer_guide/\*.md)」、`CLAUDE.md:25`）。原則から導けないので、新しい文書を追加するたびに言語の判断が要る。

`docs/dev/developer_guide/` には、エージェントが従う規約（`requirements_process.md`・`test_organization.md` など）と、人間がツールを使うための手引き（`mergepr_guide.md`・`fixprguard_guide.md` など）が混在している。手引きのうち 2 つは、英語版と日本語版（`.ja.md`）を二重に管理している（`docs/dev/developer_guide/mergepr_guide.md`・`mergepr_guide.ja.md`・`fixprguard_guide.md`・`fixprguard_guide.ja.md`）。

開発者・利用者とも日本語話者を想定しているが、README（`README.md`、1,278 行）は英語だけである。また README には、拡張の利用者には不要な開発者向けの情報が含まれている。CLI の使い方（`README.md:98-143`・`:200-265`・`:441-467`・`:839-866`）、開発用のサーバ（`README.md:1027-1068`）、各種の統合テスト（`README.md:1197-1278`）などである。

対応 issue: [#537](https://github.com/isseis/yt2column/issues/537)

**本書の記述範囲:** 本書は、各文書の置き場所と言語、README と開発者向けの文書の境界、二言語の README の対応を保つための検査と手順を、観測できる結果として定める。ガードテストの作り（パラメータ化の方法、見出しの照合の方法）、構造の一致を調べる手段、リンクを一括で置き換える手段、PR の分け方は、設計（`02_architecture.md`）と実装計画（`03_implementation_plan.md`）で決める。要件の作成中に挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #537 で決めたこと:**

-   **言語の原則:** 人間が判断・承認する文書と人間向けの手引きは日本語にする。エージェントの実行を制御する指示とリポジトリ内のコードは英語にする。エンドユーザー向けの文書（README）は、日本語を正本とし、英語版を同期して維持する。この原則を CLAUDE.md の "Documents" 節と "Language" 節に書く。場所ごとの例外では、新しい文書の言語を決められないためである。
-   **`AGENTS.md`:** エージェント向けなので英語のままにする。
-   **developer_guide の分割:** `requirements_process`・`test_organization`・`mermaid_reference`・`package_reference`・`task_identification` は `docs/dev/conventions/`（英語）に置く。`mergepr_guide`・`fixprguard_guide`・`pricing_update`・`claude_output_limit` は `docs/dev/guides/`（日本語）に置く。手引きは日本語版に一本化して、ファイル名から `.ja` を外し、英語版は削除する。`pricing_update` と `claude_output_limit` は、英語版しかないので日本語に翻訳する。
-   **旧パスへのリンク:** 過去のタスク文書（`docs/tasks/`）を含めて、すべて一括で置き換える。旧パスにはリダイレクト用のスタブを置かない。
-   **fixprguard の入れ直し:** 移動の PR の本文に、`make install-fixprguard` による入れ直しが必要であることを書く。`cmd/fixprguard/main.go:132` の `guidePath` が、利用法の診断で手引きのパスを示すためである。
-   **エンドユーザーの定義:** エンドユーザーは拡張の利用者とする。拡張はストアで公開しておらず、利用者が自分でビルドするので、そのビルド手順は README に残す。
-   **開発者向けの文書:** 開発者向けの内容は別の文書に移し、その文書は日本語版だけとする。置き場所は `docs/dev/guides/` とする。
-   **README の二言語化:** `README.md` を日本語の正本にし、英語版の `README.en.md` を新設する。`README.md` の先頭に英語版へのリンクを置く。英語版は全文を同期して維持する。日本語版を先に変え、`/mktrans` の差分翻訳で英語版に反映する。二言語で持つのは README だけとし、その例外を CLAUDE.md に書く。
-   **ガードテスト:** README を読むガードテストは、日本語版と英語版の両方を検査する。見出しや列名のリテラルは言語ごとに持つ。開発者向けの文書に移った値を読むガードテストは、移った先を読む。日本語版と英語版で、見出しの構造（見出しのレベルの並びと数）とコードブロックの数が一致することを検査するテストを追加する。見出しの文言は比較しない。CLAUDE.md の Go Idioms にある「日本語の見出しや表のセルに一致させるガード用のリテラル」の例外を Go のテストにも適用すると明記する。
-   **英語の UI からの参照:** 拡張の英語のメッセージ（`extension/src/core/i18n/en.ts:77`）は `README.en.md` を指す。CLI のヘルプ（`cmd/yt2column/run.go:259`）は、CLI の説明が移った先の開発者向けの文書を指す。
-   **同期の手順:** README.md を変えうるコマンド（`/runplan`・`/fixpr`・`/runpr`）に、README.md を変えたときは `/mktrans` で `README.en.md` に反映する手順を加える。README.md を変えていない PR では何もしない。構造の一致は、CI でテストが検査する。コマンドの手順は、その前に気づくための保険とする。

**要件定義の際に決めたこと（2026-10-28）:**

-   **README と開発者向けの文書の境界:** 拡張の利用者が観測するものは README に、CLI を使ったときだけ観測するものは開発者向けの文書に置く。CLI と主経路の両方を扱う節（後処理の 2 段階・トークンの使用量・推定の料金・出力の書式・キャッシュ・Webhook への投稿）は、この規則で節の中を分ける（F-003）。節を丸ごと移すと、利用者に必要な説明（Slack への投稿の書式、脚注の料金の意味など）まで README から消えるためである。
-   **設定の表:** Configuration 節は README に残す。CLI だけが読む変数（`YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH`）と、CLI だけに関わる注記は、開発者向けの文書に移す。利用者が設定する変数だけが README に並ぶようにするためである。
-   **移行の手順:** 「Upgrading from the shared-token version」と「Migrating from the retired LLM variables」は、どちらも README に残す。以前の版を使っていた利用者が、更新するときに読む手順だからである。
-   **サーバの API:** curl でサーバを確かめる手順と Job API の値の表は、開発者向けの文書に移す。サーバの API は拡張の利用者の操作に影響しないためである。あわせて、「サーバの API は README に書く」とする CLAUDE.md の記述（`CLAUDE.md:212-213`）を、開発者向けの文書に書くと改める。ただし、拒否の理由（`reason`）と拡張での表示の対応は README にも残す。ポップアップは拒否の理由を表示する（`README.md:1158`）ので、利用者がその意味を README で調べられるようにするためである。ステータスコードを含む完全な表は、開発者向けの文書に置く。
-   **開発者向けの文書の構成:** 2 つの文書に分ける。CLI の使い方は `docs/dev/guides/cli.md` に、開発の手順（`make` のターゲット、開発用のサーバ、拡張の開発用のターゲット、統合テスト、サーバの API）は `docs/dev/guides/development.md` に置く。CLI のヘルプが CLI の説明だけを指せるようにするためである。
-   **`/mktrans` の改訂:** 今回のスコープに含める。今の `/mktrans` は、翻訳の方向をファイル名の `.ja.md` で決める（`.claude/commands/mktrans.md:27-35`）。そのため、`README.md` を渡すと英語から日本語への翻訳と判断し、`README.ja.md` を出力する。これでは issue の同期の手順が動かない。改訂では、`README.md`（日本語）から `README.en.md`（英語）への組を扱えるようにする。二言語で持つ文書は README だけになるので、`.ja.md` の組を前提にした方向の規則と、developer_guide の翻訳の組への言及（`.claude/commands/mktrans.md:20-22`）は削除する。どの組にも当たらないファイルを渡したときは、今と同じく止まって利用者に尋ねる（`.claude/commands/mktrans.md:35`）。
-   **用語集に加える範囲:** README の見出しの訳と、README の中で繰り返し使う用語のうち、用語集（`docs/translation_glossary.md`）にないものを加える。差分翻訳のたびに、同じ語の訳が揺れないようにするためである。
-   **`prompts/README.md` と `testdata/README.md`:** どちらも全体を日本語の文書とし、英語のまま残す部分はない。どちらも、見出しと URL などを除いて、すでに全文が日本語である（`prompts/README.md:1-121`・`testdata/README.md:1-124`。本書の作成時に確かめた）。人間がテンプレートを書くとき、または出典を確かめるときに読む文書であり、エージェントの実行を制御する指示ではない。
-   **英語版から日本語版へのリンク:** `README.en.md` の先頭にも、`README.md` へのリンクを置く。issue が定めるのは日本語版から英語版へのリンクだけだが、どちらの版からも、もう一方の版へ移れるようにするためである。
-   **`--slack` のヘルプの参照先:** issue が挙げる `cmd/yt2column/run.go:259` に加えて、`--slack` のフラグのヘルプ（`cmd/yt2column/run.go:225` の "see README"）も `docs/dev/guides/cli.md` を指す。CLI のヘルプが参照する先を 1 つにそろえるためである。`cli.md` から README の設定の節へリンクする。`cli.md` は日本語だけなので、CLI の英語のヘルプは日本語の文書を指すことになる。CLI は補助の経路であり、読み手は日本語話者の開発者だからである。
-   **サーバの診断の参照先:** サーバの英語の診断（`cmd/yt2column-server/run.go:145` の "see README.md"）は `README.en.md` を指す。拡張の英語のメッセージ（issue の決定）と同じく、主経路の英語の UI が英語の利用者の読める文書を指すようにするためである。
-   **進行中のタスクとの順序:** 本タスクの README の作業（F-003・F-004、および README を読むガードテストの変更）は、`0044_article_job_api` の PR-7（[#550](https://github.com/isseis/yt2column/pull/550)。`docs/tasks/0044_article_job_api/03_implementation_plan.md:463`）と、`0046_popup_redesign` の PR-4（`docs/tasks/0046_popup_redesign/03_implementation_plan.md:377`）がマージされた後に始める。どちらも README の同じ節を変えるためである。F-001・F-002・F-006 は先に進めてよい。
-   **先のタスクの決定の置き換え:** 本タスクは、次の承認済みの決定を置き換える。`0044_article_job_api` の AC-29（Job API の値を README の表に書く。`docs/tasks/0044_article_job_api/01_requirements.md:235`）は、表を `development.md` に置くことに改める。0044 の AC-42（README の拒否の理由の表がサーバと一致する。同 `:299`）は、完全な表を `development.md` に、`reason` と拡張での表示の対応を README の両版に置くことに改める。`0046_popup_redesign` の AC-31（README が引用するポップアップの英語の文言が英語の文言の表と一致する。`docs/tasks/0046_popup_redesign/01_requirements.md:196`）は、`README.en.md` には英語の文言を、`README.md` には日本語の文言を引用することに改める（AC-31a）。サーバの API を開発者向けの文書に移し、README を日本語の正本にするという本タスクの決定に合わせるためである。
-   **エージェントが読む手引き:** `docs/dev/guides/` の手引きは日本語に一本化する（issue の決定）。一方、コマンドの定義は、エージェントに手引きを読ませている（`.claude/commands/fixpr.md:34-35` の `fixprguard_guide`、`.claude/commands/mergepr.md:20-21` の `mergepr_guide`）。これは、AC-01 の原則 (b)「エージェントの実行を制御する指示は英語」の例外として CLAUDE.md に書く。範囲は、コマンドの定義が `docs/dev/guides/` の手引きを参照することに限る。理由は、手引きは人間がツールを使うための文書であり、同じ内容を英語でも持つと二重管理に戻るためである。エージェントは日本語の文書も読める。
-   **ポップアップの文言の引用:** `README.md` は日本語のメッセージカタログ（`extension/src/core/i18n/ja.ts`）の文言を、`README.en.md` は英語のカタログ（`extension/src/core/i18n/en.ts`）の文言を引用する。今の README は英語の文言を引用している（`README.md:1124-1127`）。日本語の README の読み手は日本語のポップアップを見るためである。
-   **英語版の作り方:** `README.md` を日本語にして `/japrose` で整えた後、改訂した `/mktrans` で `README.en.md` を全文翻訳して作る。最初から、英語版が正本の翻訳になるようにするためである。今の英語の README を英語版としてそのまま残すことはしない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   新しい文書を追加するとき、読み手と承認者から言語が一つに決まるようにする。
-   規約と手引きの置き場所を分け、手引きの二重管理をなくす。
-   README を拡張の利用者のための文書にし、日本語で読めるようにする。英語版は日本語版と同期して保つ。
-   二言語の README の構造の一致を、CI で検査する。

### 2.2. スコープ (In Scope)

-   CLAUDE.md の "Documents" 節・"Language" 節・"Translation Guidelines" 節・Go Idioms・Development Notes・Quick Links、`.claude/commands/_context.md`、`AGENTS.md` の、言語の方針と文書のパスに関わる記述の改訂。
-   `docs/dev/developer_guide/` の `docs/dev/conventions/` と `docs/dev/guides/` への分割。手引きの日本語版への一本化と、`pricing_update.md`・`claude_output_limit.md` の日本語への翻訳。
-   旧パスを参照しているすべての箇所の置き換え。コード（`cmd/fixprguard/main.go:132`）、ガードテスト、コードのコメント、`Makefile`、`.claude/commands/` 配下、過去のタスク文書を含む。
-   README から開発者向けの内容を分け、`docs/dev/guides/cli.md` と `docs/dev/guides/development.md` を新設すること。
-   `README.md` の日本語化、`README.en.md` の新設、両版の相互リンク。
-   README を読むガードテストの二言語化と、移った値を読むガードテストの付け替え。構造の一致を調べるテストの追加。CI の判定（拡張のジョブを走らせるかどうか）の更新。
-   README の節の名前で README を参照している、過去のタスク文書以外の文書の更新（`docs/dev/project_overview.md:156`、`docs/dev/production_profile.md:5`、`docs/translation_glossary.md:43` など）。
-   拡張の英語のメッセージと CLI のヘルプの参照先の変更。
-   `/runplan`・`/fixpr`・`/runpr` への同期の手順の追加と、`/mktrans` の改訂。
-   用語集への README の訳語の追加。

### 2.3. スコープ外 (Out of Scope)

-   ソースコード、コメント、識別子、コミットメッセージの言語（今のとおり英語）。issue のスコープ外である。
-   `.claude/commands/` 配下のコマンド定義の言語（今のとおり英語）。issue のスコープ外である。
-   GitHub 上の PR や issue の言語（今のとおり日本語）。issue のスコープ外である。
-   拡張のメッセージカタログの内容。ただし、参照先の変更（F-005）を除く。issue のスコープ外である。日本語のカタログ（`extension/src/core/i18n/ja.ts:80`）は「README」を指しているが、日本語の正本である `README.md` を指すことになるので変えない。
-   過去のタスク文書のうち、README の節を英語の節の名前で述べている記述（例：「README の Configuration 節」）。過去のタスク文書は当時の経緯の記録であり、置き換えるのはファイルのパスへのリンクだけとする（F-002）。
-   `.github/copilot-instructions.md:4` の `*-ja.md` への言及。issue が挙げていない。この記述は、今も本タスクの後も、当たるファイルを持たない（今の日本語版は `.ja.md` であり `-ja.md` ではない）。エージェント向けの英語の指示として言語は原則に合っているので、変えない。
-   ガードテストを CI のどのジョブで走らせるかの分担の変更。今の分担（`CLAUDE.md`、Development Notes）を新しいファイルにも当てはめ、そのために既存の判定（`extension/scripts/has-extension-changes.sh`）に新しいファイルを加えることだけを行う（AC-30・AC-33・AC-41）。
-   本書で分類していない `.md` ファイル（`.github/` 配下、`internal/publisher/testdata/golden/*.md` など）の言語。AC-06 は、本タスクが置き場所や言語を変える文書と、原則の分類を CLAUDE.md に書く文書だけを対象にする。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 言語の原則

文書の言語を、場所ごとの例外ではなく、読み手と承認者で決まる原則として CLAUDE.md に書く。本タスクの後の文書の言語は、この原則（と AC-04・AC-04a の例外）に合っている。

**Acceptance Criteria**:
- **AC-01**: CLAUDE.md の "Documents" 節が、言語を次の 3 つの原則で定めている。(a) 人間が判断・承認する文書と人間向けの手引きは日本語、(b) エージェントの実行を制御する指示とリポジトリ内のコードは英語、(c) エンドユーザー（拡張の利用者）向けの文書である README は日本語を正本として英語版を同期して維持する。場所ごとの例外の列挙（`CLAUDE.md:25`）は残っていない。
- **AC-02**: CLAUDE.md の "Documents" 節が、ディレクトリごとの言語として、`docs/dev/conventions/` は英語、`docs/dev/guides/` は日本語と定めている。
- **AC-03**: CLAUDE.md の "Language" 節の記述が、AC-01 の原則と矛盾しない。
- **AC-04**: CLAUDE.md に、二言語で持つ文書は README（`README.md` と `README.en.md`）だけであること、`README.md` を先に変えて `/mktrans` で `README.en.md` に反映することが、言語の原則の例外として書かれている。`README.en.md` をリポジトリの直下に置くことも、「Documents should be placed under docs」（CLAUDE.md、"Documents" 節）の例外として、README と同じ置き場所にする理由とともに書かれている。
- **AC-04a**: CLAUDE.md に、コマンドの定義が `docs/dev/guides/` の日本語の手引きを参照してエージェントに読ませることが、原則 (b) の例外として、範囲と理由（§1「エージェントが読む手引き」）とともに書かれている。
- **AC-05**: CLAUDE.md の Go Idioms にある「日本語の文書の見出しや表のセルに一致させるガードテストのリテラル」の例外が、拡張のテストだけでなく Go のテストにも適用されると明記されている。
- **AC-06**: 次の表の各文書が、表の言語で書かれている。

    | 文書 | 原則の分類 | 言語 |
    |---|---|---|
    | `CLAUDE.md`、`AGENTS.md`、`.claude/commands/` 配下 | エージェントの実行を制御する指示 | 英語 |
    | `docs/dev/conventions/` 配下 | エージェントが従う規約 | 英語 |
    | `docs/dev/guides/` 配下 | 人間向けの手引き | 日本語 |
    | `docs/dev/` の直下の文書、`docs/tasks/` 配下、`docs/translation_glossary.md` | 人間が判断・承認する文書 | 日本語 |
    | `prompts/README.md`、`testdata/README.md` | 人間向けの手引き | 日本語（全体） |
    | `README.md` | エンドユーザー向けの文書の正本 | 日本語 |
    | `README.en.md` | エンドユーザー向けの文書の英語版 | 英語 |

- **AC-07**: CLAUDE.md は、サーバの API を `docs/dev/guides/development.md` に、サーバの起動を README に書くとしている。API を README に書くとしていた記述（`CLAUDE.md:212-213`）は残っていない。

#### F-002: developer_guide の分割

`docs/dev/developer_guide/` を、英語の規約の `docs/dev/conventions/` と、日本語の手引きの `docs/dev/guides/` に分ける。旧パスへの参照をすべて新しいパスに置き換える。

**Acceptance Criteria**:
- **AC-08**: `docs/dev/conventions/` に `requirements_process.md`・`test_organization.md`・`mermaid_reference.md`・`package_reference.md`・`task_identification.md` がある。それぞれの内容は、移動前の同名のファイルから、リンクのパスの変更を除いて変わっていない。
- **AC-09**: `docs/dev/guides/` に `mergepr_guide.md`・`fixprguard_guide.md`・`pricing_update.md`・`claude_output_limit.md` があり、どれも日本語で書かれている。`mergepr_guide.md` と `fixprguard_guide.md` の内容は、移動前の `.ja.md` の版から変わっていない。例外は、リンクのパスの変更、AC-11 で加える記述、AC-26 で README の節の参照先を改める箇所だけである。`docs/dev/guides/` に `.ja.md` で終わるファイルはない。
- **AC-10**: `pricing_update.md` と `claude_output_limit.md` は、移動前の英語の文書の全文の翻訳である。節の構成と、手順・値・コマンドの例は、移動前と同じである。例外は、リンクのパスの変更と、AC-26 で README の節の参照先を改める箇所（`pricing_update.md:55-67`、`claude_output_limit.md:61`・`:104`）だけである。
- **AC-11**: 英語版の手引き（`mergepr_guide.md` と `fixprguard_guide.md`）を削除する前に、日本語版の手引きが英語版の手引きの内容をすべて含んでいる。英語版の手引きにだけあった記述は、日本語版の手引きに加えてから削除する。
- **AC-12**: `docs/dev/developer_guide/` というディレクトリはない。旧パスに、リダイレクト用のスタブのファイルはない。
- **AC-13**: リポジトリの中で、移動した文書を旧パスで指している参照はない。コード、ガードテスト、コードのコメント、`Makefile`、`.claude/commands/` 配下、CLAUDE.md、AGENTS.md、README、`docs/` 配下（過去のタスク文書を含む）が対象である。例外は、本タスク（`docs/tasks/0048_doc_language_policy/`）の文書が移動を説明するために旧パスを述べる箇所だけである。英語版の手引き（`mergepr_guide.md`・`fixprguard_guide.md`）を指していた参照も、日本語版の手引きを指していた参照も、`docs/dev/guides/` の一本化した手引きを指す。英語版と日本語版の手引きを並べて挙げていた行は、同じパスが 2 度並ぶ形になってもよい（AC-14 により、パス以外は変えない）。
- **AC-14**: 過去のタスク文書で変わるのは、旧パスへの参照だけである。Document Status（`Status`・`Review date`・`Reviewer`・`Comments`）を含め、ほかの記述は変わらない。
- **AC-15**: 置き換えた後、移動した文書を指していた相対リンクは、どれも存在するファイルを指している。
- **AC-16**: `fixprguard` の利用法の診断（`cmd/fixprguard/main.go:268`）が示す手引きのパスは `docs/dev/guides/fixprguard_guide.md` であり、そのファイルは存在する。
- **AC-17**: 移動前に `docs/dev/developer_guide/` の文書を読んでいたガードテスト（`cmd/fixprguard/docs_test.go:28-29` の手引き、`internal/pipeline/pipeline_test.go:707` の `package_reference.md`）は、移動後の文書を検査し、`make test` で通る。`fixprguard` の手引きのガードテストが検査するのは、日本語に一本化した 1 つのファイルである。
- **AC-18**: 移動を含む PR の本文に、マージの後で `make install-fixprguard` による入れ直しが必要であることが書かれている。

#### F-003: README と開発者向けの文書の分離

README を拡張の利用者のための文書にする。CLI を使ったときだけ観測するものは `docs/dev/guides/cli.md` に、開発の手順は `docs/dev/guides/development.md` に移す。どちらも日本語の文書であり、英語版は持たない。

境界の規則は、「拡張の利用者が観測するものは README、CLI を使ったときだけ観測するものは `cli.md`、開発者が開発や確認のために使うもの（curl での確かめ、サーバの API、統合テストなど）は `development.md`」である。規則は、節の単位ではなく文の単位で当てはめる。README に残す節の中にある、CLI だけに関わる文や句は `cli.md` に移す。その例は、`--after` で再開した実行の料金の扱い、「CLI の実行は `2` で終わる」という記述、CLI がクライアントを作る時点の記述である。同様に、curl に関わる文や句は `development.md` に移す。次の AC の表は、今の README の各節へのこの規則の当てはめである。文の単位の振り分けの一覧は、設計で作る（[design_handoff.md](design_handoff.md) H-12）。

**Acceptance Criteria**:
- **AC-19**: README に、次の内容がある。

    | 今の README の節 | README に残す内容 |
    |---|---|
    | 冒頭（概要と図） | 全体 |
    | Quick start | 全体 |
    | Requirements | 主経路の要件と、選んだプロバイダの API キーの説明 |
    | Post-generation steps | 字数の調整と検証・推敲の 2 段階が何をするか、それぞれが LLM を呼ぶので費用が増えること、検証・推敲のプロバイダとモデルを `YT2COLUMN_REVIEW_LLM` で選べること |
    | Output format | 「The Slack post」の小節の全体と、それに関わる、情報の並べ方の説明（`README.md` の Output format の冒頭） |
    | Estimated cost | 投稿の脚注に出る推定の料金の意味と形、推定の料金が請求額と異なりうる理由（再開した実行に関わるものを除く）、サーバの投稿では料金が不明な理由を示せないこと |
    | Webhook publishing | Webhook の URL の規則、投稿の分割、`silent`、メンションになりうる書式の検査と拒否、分割した投稿の途中で失敗した場合 |
    | Localhost server | 節の冒頭の説明と、「Running the server」「Stopping the server」「Installing the native messaging host」「Troubleshooting」「Upgrading from the shared-token version」の各小節 |
    | Checking the server with curl | 拒否の理由（`reason`）と拡張での表示の対応 |
    | Configuration | 主経路で使う変数の行と注記。「Recommended configuration」「Migrating from the retired LLM variables」の各小節を含む。`SLACK_WEBHOOK_URL` の行の「未設定のとき」の欄は、サーバでの扱いを書く |
    | Browser extension | 拡張の説明、Node.js の版の要件、`make ext-install` と `make ext-build` によるビルド、読み込みと更新の手順、拡張の ID、ホストの登録、ポップアップの表示言語、送り方、概要欄が折りたたまれているときの扱い、取得できない値の扱い、結果の読み方、`requestId` の使い方、選択範囲の扱い、ポップアップのコンソールの開き方。今は Development 節の小節である（`README.md:1070`）が、README の最上位の節にする |

- **AC-20**: `docs/dev/guides/cli.md` に、次の内容がある。

    | 今の README の節 | `cli.md` に移す内容 |
    |---|---|
    | Requirements | 「For the CLI only」の要件 |
    | Usage、Exit codes | 全体 |
    | Post-generation steps | `--no-shorten`・`--no-review`・`--review-log`・`--artifacts`・`--after` の説明、記事のモデルの項目の並び、`--no-review` でも検証・推敲の設定を検査すること |
    | Token usage | 全体 |
    | Output format | 「The `--out` file」の小節の全体と、それに関わる、情報の並べ方の説明（`README.md` の Output format の冒頭） |
    | Estimated cost | 標準エラーに出る `cost:`・`cost-total:` の行の説明と例、`--out` のファイルの `estimated_cost_usd`、再開した実行の料金の扱い |
    | Cache and concurrency | 全体 |
    | Webhook publishing | `--slack` の説明、URL の拒否で `--out` の実行も止まること、`--out` のファイルが変わらないこと、生成された記事を `--out` で確かめる方法 |
    | Setting up yt-dlp | 全体 |
    | Configuration | `YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` の行、サーバがこの 2 つを読まないという注記、CLI だけに関わる注記（`--slack` と `--out` での `SLACK_WEBHOOK_URL` の扱い、CLI がクライアントを作る時点、`--after review` などの再開した実行での設定の読み方） |
    | README に残す節の中の、CLI だけに関わる文と句 | 全体 |

- **AC-21**: `docs/dev/guides/development.md` に、次の内容がある。

    | 今の README の節 | `development.md` に移す内容 |
    |---|---|
    | Development | `make` のターゲットの一覧と、開発の進め方への案内 |
    | Running a separate server for development | 全体 |
    | Browser extension | `make ext-check` など、開発のためのターゲット |
    | Checking the server with curl | 全体（ステータスコードを含む拒否の理由の完全な表を含む） |
    | Job API codes | 全体 |
    | Integration test、DeepSeek integration test、Claude integration test、CLI integration test、Webhook integration test、CLI webhook integration test | 全体 |
    | README に残す節の中の、curl に関わる文と句（例：Troubleshooting の、curl が出すメッセージ） | 全体 |

- **AC-22**: 今の README の文・表の行・コードブロックの行は、どれも README・`cli.md`・`development.md` の少なくとも 1 つにある。表やコードブロックを 2 つの文書に分けてもよい。2 つ以上の文書に同じ内容があってよいのは、拒否の理由と拡張での表示の対応、ほかの文書の節を参照するための短い案内、README と `cli.md` の両方の出力に関わる説明（例：Output format の冒頭の、情報の並べ方の説明）、今の README の中ですでに重複している記述（例：旧変数があれば実行が止まること、チャンネル名が `invalid_body` で拒否されること）だけである。
- **AC-23**: README に、開発者向けの文書として `cli.md` と `development.md` へのリンクがある。
- **AC-24**: `cli.md` と `development.md` は日本語で書かれている。英語版のファイルはない。
- **AC-25**: README・`README.en.md`・`cli.md`・`development.md` の相対リンクは、どれも存在するファイルを指している。見出しへのリンク（`#...`）は、どれも、リンク先の文書にある見出しを指している。README の節が移ったために、README の中のリンク（例：Requirements から「Setting up yt-dlp」へのリンク）の先が README にない場合は、移った先の文書の見出しを指す。
- **AC-26**: 過去のタスク文書以外の文書で README の節を名前で参照している箇所は、移った後の文書と見出しを指している。`docs/dev/project_overview.md:156`（Migrating）、`docs/dev/production_profile.md:5`（Recommended configuration）、`docs/translation_glossary.md:43`（推奨構成）、`pricing_update.md`（Estimated cost・Output format・Configuration）、`claude_output_limit.md`（Token usage・Configuration）、CLAUDE.md（Development Notes）、および、過去のタスク文書以外で README の節を名前で参照しているほかのすべての文書が対象である。

#### F-004: README の日英二言語化

`README.md` を日本語の正本にし、英語版の `README.en.md` を新設する。両版の対応を、テストと手順で保つ。

**Acceptance Criteria**:
- **AC-27**: `README.md` は日本語で、その内容は AC-19 で README に残す内容の翻訳である。内容を加えも削りもしない（`/japrose` による文章の整理、AC-25 のリンクの付け替え、AC-31a の引用の言語の変更を除く）。`README.en.md` は `README.md` の全文を `/mktrans` で翻訳したものである。
- **AC-28**: `README.md` の先頭に `README.en.md` へのリンクが、`README.en.md` の先頭に `README.md` へのリンクがある。
- **AC-29**: `README.md` と `README.en.md` で、見出しのレベルの並びと数が一致しないとき、テストが失敗する。このテストは、README だけを変えた PR でも CI が走らせる。コードブロックの数が一致しない場合と、どちらかのファイルがない場合も失敗する（スキップしない）。見出しの文言の違いでは失敗しない。両版の構造が一致していれば通る。コードブロックの中の行は、見出しとして数えない。失敗のメッセージは、一致しなかった項目（見出しの並びかコードブロックの数か）と、最初に食い違った見出しの位置を示す。
- **AC-30**: AC-29 のテストは、`make test` に含まれる。
- **AC-31**: README を読むガードテストは、README に残る値を、README の両版で検査する。どちらかの版が値を欠くと失敗し、失敗のメッセージで、どちらの版かがわかる。対象は、本書の作成時に README を読んでいるガードテストのうち、README に残る値を読むすべてである（一覧は [design_handoff.md](design_handoff.md) H-11）。
- **AC-31a**: `README.md` が引用するポップアップの文言は日本語のメッセージカタログ（`extension/src/core/i18n/ja.ts`）の文言と、`README.en.md` が引用する文言は英語のカタログ（`extension/src/core/i18n/en.ts`）の文言と一致する。今の README の「the popup text quoted in this README ... is the English version」（`README.md:1124-1127`）に当たる説明は、各版で、その版が引用する言語を述べる。
- **AC-32**: `cli.md` または `development.md` に移った値を読むガードテストは、移った先のファイルを検査する。本書の作成時にガードテストが README で検査していた値と表の行は、どれも、本タスクの後に、その値を持つ文書で検査される。この対応は、実装計画に、ガードテストごとの前後の表として記録し、PR のレビューで確かめる（`manual`）。
- **AC-33**: 拡張のジョブを走らせるかどうかの CI の判定（`extension/scripts/has-extension-changes.sh:18`）が、`README.en.md` を変えた PR で拡張のジョブを走らせる。拡張のテストが読む文書のうち、開発者向けの文書（例：拒否の理由の完全な表を持つ `development.md`）を変えた PR でも、走らせる。`extension/test/ciChanges.test.ts` が、その判定を検査する。issue が二言語化の対象に挙げている `ciChanges.test.ts` の変更は、この判定の検査に対応する（このテストは README の内容を読まない）。
- **AC-34**: 用語集（`docs/translation_glossary.md`）に、次の 3 つのうち、本タスクの前に用語集になかったものがすべてある。(1) README のすべての見出しの訳、(2) 本書の §6 の用語、(3) README の中で 2 つ以上の節（`##` または `###` の見出しの下）の地の文に現れる、概念を表す語（バッククォートの中のコード・製品名などの固有名詞を除く）。`README.en.md` は、用語集にある訳語を使っている。

#### F-005: 英語の UI からの参照

英語で表示する UI が、英語の利用者の読める文書、または内容が移った先の文書を指すようにする。

**Acceptance Criteria**:
- **AC-35**: 拡張の英語のメッセージ（`extension/src/core/i18n/en.ts:77`）は、ソケットのディレクトリの権限の説明の参照先として `README.en.md` を指す。日本語のメッセージ（`extension/src/core/i18n/ja.ts:80`）は変わらない。
- **AC-35a**: サーバの英語の診断（`cmd/yt2column-server/run.go:145`）は、設定の説明の参照先として `README.en.md` を指す。
- **AC-36**: CLI のヘルプ（`yt2column -h`）が設定の説明の参照先として指す文書（`cmd/yt2column/run.go:259`、`run.go:225`）は `docs/dev/guides/cli.md` である。`cli.md` は、`README.md` の設定の節へのリンクを含む。

#### F-006: 同期の手順

`README.md` の変更が `README.en.md` に反映されるよう、コマンドに手順を加え、`/mktrans` を改訂する。

**Acceptance Criteria**:
- **AC-37**: `/runplan`・`/fixpr`・`/runpr` のコマンド定義に、その実行が `README.md` を変えた場合は、PR に push する前に `/mktrans` で `README.en.md` に反映する手順がある。`README.md` を変えていない実行では、この手順は何もしない。`/mktrans` が止まった場合は、コマンドは push せずに止まり、理由を報告する。これらのコマンドを使わない変更（人間が直接行う変更）は、AC-29 のテストと PR のレビューで保つ。
- **AC-38**: `/mktrans` に `README.md` を渡すと、日本語から英語への翻訳として、`README.en.md` を出力する。`README.en.md` がすでにあるときは差分翻訳になり、`README.en.md` に最後に反映された時点より後の `README.md` の変更を、すべて反映する。
- **AC-39**: `/mktrans` に、どの翻訳の組にも当たらないファイル（`README.en.md` を含む）を渡すと、翻訳せずに止まり、利用者に尋ねる。`.ja.md` の組を前提にした方向の規則と、developer_guide の翻訳の組への言及は、`/mktrans` のコマンド定義に残っていない。
- **AC-40**: CLAUDE.md の "Translation Guidelines" 節と `.claude/commands/_context.md` の「Translation language pair」の行が、AC-38・AC-39 の `/mktrans` の振る舞いと矛盾しない。
- **AC-41**: CLAUDE.md の Development Notes にある、文書を読むガードテストの説明（`CLAUDE.md:431-448`）が、本タスクの後のファイル（`README.md`・`README.en.md`・`cli.md`・`development.md`）と、それぞれを読むテストに合っている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書とテストの変更であり、yt2column の実行時の振る舞いは変わらない。

### 4.2. セキュリティ (Security)
-   N/A。本タスクは、外部コマンドの起動、API キーや Webhook の URL の扱い、キャッシュの読み書き、ネットワークへの送信、信頼できない文字列のプロンプトや記事への埋め込みのどれも変えない（条件付きの指針 `docs/dev/security.md` の適用条件に当たらない）。`fixprguard` の変更は、診断で示す手引きのパスの文字列だけである。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。実行時の振る舞いは変わらない。

### 4.4. 互換性 (Compatibility)
-   旧パス（`docs/dev/developer_guide/`）への外部からのリンク（ブックマーク、GitHub 上の古いコメントなど）は切れる。issue の決定により、スタブは置かない。
-   インストール済みの `fixprguard` は、入れ直すまで旧パスを示す。移動を含む PR の本文に、入れ直しが必要であることを書く（AC-18）。
-   README の見出しへの外部からのリンク（英語の見出しの `#...`）は、`README.md` では切れる。同じ見出しは `README.en.md` にあるか、開発者向けの文書に移っている。

### 4.5. 保守性 (Maintainability)
-   新しい文書を追加するとき、AC-01 の原則と AC-02 のディレクトリの規則から、言語が一つに決まる。
-   AC-29 が両版で比べるのは、見出しのレベルの並びと数、コードブロックの数だけである。文言、表の内容、コードブロックの内容が両版で対応していることと、同じ形のまま節を入れ替えた変更を両版に反映することは、AC-37 の手順と PR のレビューで保つ。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書を読むテストは、値・構造・実行できる例を確かめ、文の意味は確かめない（CLAUDE.md、Testing Strategy の "Document tests check values, structure, and executable examples, never prose"）。AC-29 の構造の一致のテストは、構造を確かめるテストに当たる。次の AC は、文の意味に関わるので、PR のレビューでの読み合わせ（`manual`）で確かめる。
    -   AC-01〜AC-04a・AC-07: CLAUDE.md の記述
    -   AC-03・AC-40・AC-41: ほかの記述と矛盾しないこと
    -   AC-10・AC-27: 翻訳の内容
    -   AC-11: 英語版の手引きの内容が日本語版の手引きに含まれること
    -   AC-19〜AC-22: 内容の振り分け
    -   AC-34: 訳語の使い方
    -   AC-37〜AC-39: コマンドの手順
-   旧パスの参照が残っていないこと（AC-13）や、過去のタスク文書の変更がパスだけであること（AC-14）は、変更そのものの性質である。そのため、文書のテストではなく、本タスクの `_checks/` の下のスクリプトで確かめる（CLAUDE.md、Testing Strategy の "A property of the change itself ... is a `diff` verification"、`requirements_process.md` §4）。
-   本タスクの README の作業（F-003・F-004 と README を読むガードテストの変更）は、`0044_article_job_api` の PR-7 と `0046_popup_redesign` の PR-4 のマージの後に始める（§1「進行中のタスクとの順序」）。
-   本タスクは、信頼できない入力を消費しない。要件定義の手順の「信頼できない入力の境界」は当てはまらない。
-   README の変更は、拡張の node のテストに影響する（CLAUDE.md、Development Notes）。README を変える PR では `make ext-test` を実行する。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書で使う用語のうち、用語集にないものを次に定める。

-   **利用者:** 拡張の利用者。issue の「エンドユーザー」に当たる。README の読み手である（`prompts/README.md` の「利用者」とは別の意味で使う）。
-   **正本:** 二言語で持つ文書のうち、先に変え、もう一方の版の翻訳の元になる版。README では `README.md`（日本語）である。
-   **英語版:** README の英語版、すなわち `README.en.md`。正本の全文の英語訳である。移動前の英語の手引き（`mergepr_guide.md`・`fixprguard_guide.md`）は「英語版の手引き」と呼んで区別する。
-   **開発者向けの文書:** `docs/dev/guides/cli.md` と `docs/dev/guides/development.md`。README から移した、CLI の使い方と開発の手順を書く。
-   **規約:** エージェントが従う決まりを書いた文書。`docs/dev/conventions/` に置く。
-   **手引き:** 人間がツールを使うための説明を書いた文書。`docs/dev/guides/` に置く。
-   **ガードテスト:** 文書に書かれた値や構造が、コードや設定の定義と一致することを確かめるテスト（CLAUDE.md、Testing Strategy）。
-   **構造の一致:** `README.md` と `README.en.md` で、見出しのレベルの並びと数、およびコードブロックの数が同じであること（AC-29）。
