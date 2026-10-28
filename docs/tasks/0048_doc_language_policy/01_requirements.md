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
-   **ガードテスト:** README を読むガードテストは、日本語版と英語版の両方を検査する。見出しや列名のリテラルは言語ごとに持つ。開発者向けの文書に移った値を読むガードは、移った先を読む。日本語版と英語版で、見出しの構造（見出しのレベルの並びと数）とコードブロックの数が一致することを検査するテストを追加する。見出しの文言は比較しない。CLAUDE.md の Go Idioms にある「日本語の見出しや表のセルに一致させるガード用のリテラル」の例外を Go のテストにも適用すると明記する。
-   **英語の UI からの参照:** 拡張の英語のメッセージ（`extension/src/core/i18n/en.ts:77`）は `README.en.md` を指す。CLI のヘルプ（`cmd/yt2column/run.go:259`）は、CLI の説明が移った先の開発者向けの文書を指す。
-   **同期の手順:** README.md を変えうるコマンド（`/runplan`・`/fixpr`・`/runpr`）に、README.md を変えたときは `/mktrans` で `README.en.md` に反映する手順を加える。README.md を変えていない PR では何もしない。同期漏れは、構造の一致を調べるテストが CI で検出する。コマンドの手順は、その前に気づくための保険とする。

**要件定義の際に決めたこと（2026-10-28）:**

-   **README と開発者向けの文書の境界:** 拡張の利用者が観測するものは README に、CLI を使ったときだけ観測するものは開発者向けの文書に置く。CLI と主経路の両方を扱う節（後処理の 2 段階・トークンの使用量・推定の料金・出力の書式・キャッシュ・Webhook への投稿）は、この規則で節の中を分ける（F-003）。節を丸ごと移すと、利用者に必要な説明（Slack への投稿の書式、脚注の料金の意味など）まで README から消えるためである。
-   **設定の表:** Configuration 節は README に残す。CLI だけが読む変数（`YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH`）と、CLI だけに関わる注記は、開発者向けの文書に移す。利用者が設定する変数だけが README に並ぶようにするためである。
-   **移行の手順:** 「Upgrading from the shared-token version」と「Migrating from the retired LLM variables」は、どちらも README に残す。以前の版を使っていた利用者が、更新するときに読む手順だからである。
-   **サーバの API:** curl でサーバを確かめる手順と Job API の値の表は、開発者向けの文書に移す。サーバの API は拡張の利用者の操作に影響しないためである。あわせて、「サーバの API は README に書く」とする CLAUDE.md の記述（`CLAUDE.md:212-213`）を、開発者向けの文書に書くと改める。ただし、拒否の理由（`reason`）と拡張での表示の対応は README にも残す。ポップアップは拒否の理由を表示する（`README.md:1158`）ので、利用者がその意味を README で調べられるようにするためである。ステータスコードを含む完全な表は、開発者向けの文書に置く。
-   **開発者向けの文書の構成:** 2 つの文書に分ける。CLI の使い方は `docs/dev/guides/cli.md` に、開発の手順（`make` のターゲット、開発用のサーバ、拡張の開発用のターゲット、統合テスト、サーバの API）は `docs/dev/guides/development.md` に置く。移す量は、CLI が約 500 行、開発の手順が約 300 行である。CLI のヘルプが CLI の説明だけを指せるようにするためである。
-   **`/mktrans` の改訂:** 今回のスコープに含める。今の `/mktrans` は、翻訳の方向をファイル名の `.ja.md` で決める（`.claude/commands/mktrans.md:27-35`）。そのため、`README.md` を渡すと英語から日本語への翻訳と判断し、`README.ja.md` を出力する。これでは issue の同期の手順が動かない。改訂では、`README.md`（日本語）から `README.en.md`（英語）への組を扱えるようにする。二言語で持つ文書は README だけになるので、`.ja.md` の組を前提にした方向の規則と、developer_guide の翻訳の組への言及（`.claude/commands/mktrans.md:20-22`）は削除する。
-   **用語集に加える範囲:** README の見出しの訳と、README の中で繰り返し使う用語のうち、用語集（`docs/translation_glossary.md`）にないものを加える。差分翻訳のたびに、同じ語の訳が揺れないようにするためである。
-   **`prompts/README.md` と `testdata/README.md`:** どちらも全体を日本語の文書とし、英語のまま残す部分はない。どちらも、見出しと URL などを除いて、すでに全文が日本語である（本書の作成時に確かめた）。人間がテンプレートを書くとき、または出典を確かめるときに読む文書であり、エージェントの実行を制御する指示ではない。
-   **英語版から日本語版へのリンク:** `README.en.md` の先頭にも、`README.md` へのリンクを置く。issue が定めるのは日本語版から英語版へのリンクだけだが、どちらの版からも、もう一方の版へ移れるようにするためである。
-   **`--slack` のヘルプの参照先:** issue が挙げる `cmd/yt2column/run.go:259` に加えて、`--slack` のフラグのヘルプ（`cmd/yt2column/run.go:225` の "see README"）も `docs/dev/guides/cli.md` を指す。CLI のヘルプが参照する先を 1 つにそろえるためである。`cli.md` から README の設定の節へリンクする。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   新しい文書を追加するとき、読み手と承認者から言語が一つに決まるようにする。
-   規約と手引きの置き場所を分け、手引きの二重管理をなくす。
-   README を拡張の利用者のための文書にし、日本語で読めるようにする。英語版は日本語版と同期して保つ。
-   二言語の README の同期漏れを、CI で検出できるようにする。

### 2.2. スコープ (In Scope)

-   CLAUDE.md の "Documents" 節・"Language" 節・"Translation Guidelines" 節・Go Idioms・Development Notes・Quick Links、`.claude/commands/_context.md`、`AGENTS.md` の、言語の方針と文書のパスに関わる記述の改訂。
-   `docs/dev/developer_guide/` の `docs/dev/conventions/` と `docs/dev/guides/` への分割。手引きの日本語版への一本化と、`pricing_update.md`・`claude_output_limit.md` の日本語への翻訳。
-   旧パスを参照しているすべての箇所の置き換え。コード（`cmd/fixprguard/main.go:132`）、ガードテスト、コードのコメント、`Makefile`、`.claude/commands/` 配下、過去のタスク文書を含む。
-   README から開発者向けの内容を分け、`docs/dev/guides/cli.md` と `docs/dev/guides/development.md` を新設すること。
-   `README.md` の日本語化、`README.en.md` の新設、両版の相互リンク。
-   README を読むガードテストの二言語化と、移った値を読むガードの付け替え。構造の一致を調べるテストの追加。CI の判定（拡張のジョブを走らせるかどうか）の更新。
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
-   `.github/copilot-instructions.md` の `*-ja.md` への言及。issue が挙げておらず、エージェント向けの英語の指示として言語は原則に合っているので、変えない。
-   ガードテストを CI のどのジョブで走らせるかの変更。今の分担（`CLAUDE.md`、Development Notes）を、新しいファイルにも当てはめるだけとする（AC-30・AC-33・AC-41）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 言語の原則

文書の言語を、場所ごとの例外ではなく、読み手と承認者で決まる原則として CLAUDE.md に書く。今の文書の言語は、この原則に合っている。

**Acceptance Criteria**:
- **AC-01**: CLAUDE.md の "Documents" 節が、言語を次の 3 つの原則で定めている。(a) 人間が判断・承認する文書と人間向けの手引きは日本語、(b) エージェントの実行を制御する指示とリポジトリ内のコードは英語、(c) エンドユーザー（拡張の利用者）向けの文書である README は日本語を正本として英語版を同期して維持する。場所ごとの例外の列挙（`CLAUDE.md:25`）は残っていない。
- **AC-02**: CLAUDE.md の "Documents" 節が、ディレクトリごとの言語として、`docs/dev/conventions/` は英語、`docs/dev/guides/` は日本語と定めている。
- **AC-03**: CLAUDE.md の "Language" 節の記述が、AC-01 の原則と矛盾しない。
- **AC-04**: CLAUDE.md に、二言語で持つ文書は README（`README.md` と `README.en.md`）だけであること、`README.md` を先に変えて `/mktrans` で `README.en.md` に反映することが、言語の原則の例外として書かれている。
- **AC-05**: CLAUDE.md の Go Idioms にある「日本語の文書の見出しや表のセルに一致させるガードテストのリテラル」の例外が、拡張のテストだけでなく Go のテストにも適用されると明記されている。
- **AC-06**: 次の表の各文書が、表の言語で書かれている。

    | 文書 | 原則の分類 | 言語 |
    |---|---|---|
    | `CLAUDE.md`、`AGENTS.md`、`.claude/commands/` 配下 | エージェントの実行を制御する指示 | 英語 |
    | `docs/dev/conventions/` 配下 | エージェントが従う規約 | 英語 |
    | `docs/dev/guides/` 配下 | 人間向けの手引き | 日本語 |
    | `docs/dev/` の直下の文書、`docs/tasks/` 配下、`docs/translation_glossary.md` | 人間が判断・承認する文書 | 日本語 |
    | `prompts/README.md`、`testdata/README.md` | 人間向けの手引き（開発者向け） | 日本語（全体） |
    | `README.md` | エンドユーザー向けの文書の正本 | 日本語 |
    | `README.en.md` | エンドユーザー向けの文書の英語版 | 英語 |

- **AC-07**: CLAUDE.md は、サーバの API を `docs/dev/guides/development.md` に書くとしている。README に書くとしていた記述（`CLAUDE.md:212-213`）は残っていない。

#### F-002: developer_guide の分割

`docs/dev/developer_guide/` を、英語の規約の `docs/dev/conventions/` と、日本語の手引きの `docs/dev/guides/` に分ける。旧パスへの参照をすべて新しいパスに置き換える。

**Acceptance Criteria**:
- **AC-08**: `docs/dev/conventions/` に `requirements_process.md`・`test_organization.md`・`mermaid_reference.md`・`package_reference.md`・`task_identification.md` がある。それぞれの内容は、移動前の同名のファイルから、リンクのパスの変更を除いて変わっていない。
- **AC-09**: `docs/dev/guides/` に `mergepr_guide.md`・`fixprguard_guide.md`・`pricing_update.md`・`claude_output_limit.md` があり、どれも日本語で書かれている。`mergepr_guide.md` と `fixprguard_guide.md` の内容は、移動前の `.ja.md` の版から、リンクのパスの変更を除いて変わっていない。`docs/dev/guides/` に `.ja.md` で終わるファイルはない。
- **AC-10**: `pricing_update.md` と `claude_output_limit.md` は、移動前の英語版の全文の翻訳である。節の構成と、手順・値・コマンドの例は、移動前と同じである。
- **AC-11**: 英語版の `mergepr_guide.md` と `fixprguard_guide.md` を削除する前に、日本語版が英語版の内容をすべて含んでいる。英語版にだけあった記述は、日本語版に加えてから削除する。
- **AC-12**: `docs/dev/developer_guide/` というディレクトリはない。旧パスに、リダイレクト用のスタブのファイルはない。
- **AC-13**: リポジトリの中で、移動した文書を旧パスで指している参照はない。コード、ガードテスト、コードのコメント、`Makefile`、`.claude/commands/` 配下、CLAUDE.md、AGENTS.md、README、`docs/` 配下（過去のタスク文書を含む）が対象である。例外は、本タスク（`docs/tasks/0048_doc_language_policy/`）の文書が移動を説明するために旧パスを述べる箇所だけである。英語版の手引き（`mergepr_guide.md`・`fixprguard_guide.md`）を指していた参照も、日本語版を指していた参照も、`docs/dev/guides/` の一本化した手引きを指す。
- **AC-14**: 過去のタスク文書で変わるのは、旧パスへの参照だけである。Document Status（`Status`・`Review date`・`Reviewer`・`Comments`）を含め、ほかの記述は変わらない。
- **AC-15**: 置き換えた後、移動した文書を指していた相対リンクは、どれも存在するファイルを指している。
- **AC-16**: `fixprguard` の利用法の診断（`cmd/fixprguard/main.go:268`）が示す手引きのパスは `docs/dev/guides/fixprguard_guide.md` であり、そのファイルは存在する。
- **AC-17**: 移動前に `docs/dev/developer_guide/` の文書を読んでいたガードテスト（`cmd/fixprguard/docs_test.go:28-29` の手引き、`internal/pipeline/pipeline_test.go:707` の `package_reference.md`）は、移動後の文書を検査し、`make test` で通る。`fixprguard` の手引きのガードが検査するのは、日本語に一本化した 1 つのファイルである。
- **AC-18**: 移動を含む PR の本文に、マージの後で `make install-fixprguard` による入れ直しが必要であることが書かれている。

#### F-003: README と開発者向けの文書の分離

README を拡張の利用者のための文書にする。CLI を使ったときだけ観測するものは `docs/dev/guides/cli.md` に、開発の手順は `docs/dev/guides/development.md` に移す。どちらも日本語の文書であり、英語版は持たない。

境界の規則は、「拡張の利用者が観測するものは README、CLI を使ったときだけ観測するものは `cli.md`、開発者が開発や確認のために使うものは `development.md`」である。次の AC の表は、今の README の各節へのこの規則の当てはめである。

**Acceptance Criteria**:
- **AC-19**: README に、次の内容がある。

    | 今の README の節 | README に残す内容 |
    |---|---|
    | 冒頭（概要と図） | 全体 |
    | Quick start | 全体 |
    | Requirements | 主経路の要件（`README.md:80-86`）と、選んだプロバイダの API キーの説明（`README.md:94-96`） |
    | Post-generation steps | 字数の調整と検証・推敲の 2 段階が何をするか、それぞれが LLM を呼ぶので費用が増えること、検証・推敲のプロバイダとモデルを `YT2COLUMN_REVIEW_LLM` で選べること（`README.md:147-163`） |
    | Output format | 2 つの出力の配置の考え方（`README.md:269-272`）のうち Slack への投稿に関わる部分と、「The Slack post」の小節の全体 |
    | Estimated cost | 投稿の脚注に出る推定の料金の意味と形（`README.md:403-410` のうち Slack への投稿）、推定の料金が請求額と異なりうる理由（`README.md:414-434`）、サーバの投稿では料金が不明な理由を示せないこと（`README.md:436-439`） |
    | Webhook publishing | 投稿の分割、`silent`、メンションになりうる書式の検査と拒否、分割した投稿の途中で失敗した場合（`README.md:469-523`）。ただし、AC-20 で `cli.md` に移す CLI だけの記述を除く |
    | Localhost server | 「Running the server」「Stopping the server」「Installing the native messaging host」「Troubleshooting」「Upgrading from the shared-token version」の各小節と、節の冒頭の説明 |
    | Checking the server with curl | 拒否の理由（`reason`）と拡張での表示の対応 |
    | Configuration | CLI だけが読む変数と CLI だけに関わる注記を除いた全体。「Recommended configuration」「Migrating from the retired LLM variables」の各小節を含む |
    | Browser extension | 拡張の説明、Node.js の版の要件、`make ext-install` と `make ext-build` によるビルド、読み込みと更新の手順、拡張の ID、ホストの登録、ポップアップの表示言語、送り方、概要欄が折りたたまれているときの扱い、取得できない値の扱い、結果の読み方、`requestId` の使い方、選択範囲の扱い、ポップアップのコンソールの開き方 |

- **AC-20**: `docs/dev/guides/cli.md` に、次の内容がある。

    | 今の README の節 | `cli.md` に移す内容 |
    |---|---|
    | Requirements | 「For the CLI only」の要件 |
    | Usage、Exit codes | 全体 |
    | Post-generation steps | `--no-shorten`・`--no-review`・`--review-log`・`--artifacts`・`--after` の説明と、`--no-review` でも検証・推敲の設定を検査すること（`README.md:162-198` のうち、AC-19 で README に残す部分を除く） |
    | Token usage | 全体 |
    | Output format | 「The `--out` file」の小節の全体と、配置の考え方のうち `--out` のファイルに関わる部分 |
    | Estimated cost | 標準エラーに出る `cost:`・`cost-total:` の行の書式、その例、`--out` のファイルの `estimated_cost_usd` |
    | Cache and concurrency | 全体 |
    | Webhook publishing | `--slack` の説明、URL の拒否で `--out` の実行も止まること、`--out` のファイルが変わらないこと、生成された記事を `--out` で確かめる方法 |
    | Setting up yt-dlp | 全体 |
    | Configuration | `YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` の行と、CLI だけに関わる注記（`--slack` と `--out` での `SLACK_WEBHOOK_URL` の扱い、`--after review` などの再開した実行での設定の読み方） |

- **AC-21**: `docs/dev/guides/development.md` に、次の内容がある。

    | 今の README の節 | `development.md` に移す内容 |
    |---|---|
    | Development | `make` のターゲットの一覧と、開発の進め方への案内 |
    | Running a separate server for development | 全体 |
    | Browser extension | `make ext-check` など、開発のためのターゲット |
    | Checking the server with curl | 全体（ステータスコードを含む拒否の理由の完全な表を含む） |
    | Job API codes | 全体 |
    | Integration test、DeepSeek integration test、Claude integration test、CLI integration test、Webhook integration test、CLI webhook integration test | 全体 |

- **AC-22**: 今の README の段落・表・コードブロックは、どれも README・`cli.md`・`development.md` の少なくとも 1 つにある。2 つ以上の文書に同じ内容があるのは、本書が決めた箇所（拒否の理由と拡張での表示の対応）と、ほかの文書の節を参照するための短い案内だけである。
- **AC-23**: README に、開発者向けの文書として `cli.md` と `development.md` へのリンクがある。
- **AC-24**: `cli.md` と `development.md` は日本語で書かれている。英語版のファイルはない。
- **AC-25**: README・`README.en.md`・`cli.md`・`development.md` の相対リンクは、どれも存在するファイルを指している。見出しへのリンク（`#...`）は、どれも、リンク先の文書にある見出しを指している。README の節が移ったために、README の中のリンク（例：Requirements から「Setting up yt-dlp」へのリンク）の先が README にない場合は、移った先の文書の見出しを指す。
- **AC-26**: 過去のタスク文書以外の文書で README の節を名前で参照している箇所は、移った後の文書と見出しを指している。`docs/dev/project_overview.md:156`（Migrating）、`docs/dev/production_profile.md:5`（Recommended configuration）、`docs/translation_glossary.md:43`（推奨構成）、`pricing_update.md`（Estimated cost・Output format・Configuration）、`claude_output_limit.md`（Token usage・Configuration）、CLAUDE.md などが対象である。

#### F-004: README の日英二言語化

`README.md` を日本語の正本にし、英語版の `README.en.md` を新設する。両版の対応を、テストと手順で保つ。

**Acceptance Criteria**:
- **AC-27**: `README.md` は日本語で、`README.en.md` は `README.md` の全文の英語訳である。
- **AC-28**: `README.md` の先頭に `README.en.md` へのリンクが、`README.en.md` の先頭に `README.md` へのリンクがある。
- **AC-29**: `README.md` と `README.en.md` で、見出しのレベルの並びと数が一致しないと、`make test` が失敗する。コードブロックの数が一致しない場合も失敗する。見出しの文言の違いでは失敗しない。両版の構造が一致していれば通る。コードブロックの中にある `#` で始まる行（例：`README.md:292` の出力の例）は、見出しとして数えない。
- **AC-30**: AC-29 のテストは、`README.md` または `README.en.md` だけを変えた PR でも、CI で走る。
- **AC-31**: 次のガードテストは、README の 2 つの版の両方を検査する。どちらかの版が値を欠くと失敗し、失敗のメッセージで、どちらの版かがわかる。
    - `cmd/yt2column/docs_test.go` のうち、README に残る内容を読むもの（拡張の ID、設定の表の LLM の既定、推奨構成、移行の例、Quick start のリンク、README のリンク、推定の料金のうち README に残る部分など）
    - `cmd/yt2column/output_layout_docs_test.go` の、Slack への投稿の例
    - `cmd/yt2column/extension_docs_test.go` のうち、README を読むもの
    - `cmd/yt2column-server/docs_test.go` のうち、README に残る内容を読むもの（ソケットのパス、終了コードの表など）
    - `cmd/yt2column-nmhost/main_test.go` の `TestReadmeHostExitCodes`
    - `extension/test/serverReasons.test.ts`、`extension/test/metadataDocs.test.ts` のうち、README に残る内容を読むもの
- **AC-32**: `cli.md` または `development.md` に移った値を読むガードは、移った先のファイルを検査する。CLI のフラグの表、CLI の終了コード、トークンの使用量の行、推定の料金の行、`--out` のファイルの例、統合テストの節、curl の例、拒否の理由の完全な表、Job API の値の表が対象である。移動の前後で、各ガードが検査する値の集合は減らない。
- **AC-33**: 拡張のジョブを走らせるかどうかの CI の判定（`extension/scripts/has-extension-changes.sh:18`）が、`README.en.md` を変えた PR で拡張のジョブを走らせる。拡張のテストが読む文書のうち、開発者向けの文書に移った値を読むもの（例：拒否の理由の完全な表）の文書を変えた PR でも、走らせる。`extension/test/ciChanges.test.ts` が、その判定を検査する。
- **AC-34**: 用語集（`docs/translation_glossary.md`）に、README の見出しの訳と、README の中で繰り返し使う用語のうち、本タスクの前に用語集になかったものがある。`README.en.md` は、用語集にある訳語を使っている。

#### F-005: 英語の UI からの参照

英語で表示する UI が、英語の利用者の読める文書、または内容が移った先の文書を指すようにする。

**Acceptance Criteria**:
- **AC-35**: 拡張の英語のメッセージ（`extension/src/core/i18n/en.ts:77`）は、ソケットのディレクトリの権限の説明を `README.en.md` に求める。日本語のメッセージ（`extension/src/core/i18n/ja.ts:80`）は変わらない。
- **AC-36**: CLI のヘルプ（`yt2column -h`）が設定の説明を求める先（`cmd/yt2column/run.go:259`、`run.go:225`）は `docs/dev/guides/cli.md` である。`cli.md` は、README に残る設定（Configuration 節）へのリンクを含む。

#### F-006: 同期の手順

`README.md` の変更が `README.en.md` に反映されるよう、コマンドに手順を加え、`/mktrans` を改訂する。

**Acceptance Criteria**:
- **AC-37**: `/runplan`・`/fixpr`・`/runpr` のコマンド定義に、その実行が `README.md` を変えた場合は、`/mktrans` で `README.en.md` に反映してからコミットする手順がある。`README.md` を変えていない実行では、この手順は何もしない。
- **AC-38**: `/mktrans` に `README.md` を渡すと、日本語から英語への翻訳として、`README.en.md` を出力する。`README.en.md` がすでにあるときは、差分翻訳になる。
- **AC-39**: `/mktrans` に `README.md` 以外のファイル（`README.en.md` を含む）を渡すと、翻訳せずに止まり、理由を示す。`.ja.md` の組を前提にした方向の規則と、developer_guide の翻訳の組への言及は、`/mktrans` のコマンド定義に残っていない。
- **AC-40**: CLAUDE.md の "Translation Guidelines" 節と `.claude/commands/_context.md` の「Translation language pair」の行が、AC-38・AC-39 の `/mktrans` の振る舞いと矛盾しない。
- **AC-41**: CLAUDE.md の Development Notes にある、文書を読むガードテストの説明（`CLAUDE.md:431-447`）が、本タスクの後のファイル（`README.md`・`README.en.md`・`cli.md`・`development.md`）と、それぞれを読むテストに合っている。

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
-   `README.md` を変えて `README.en.md` への反映を忘れた場合、見出しの構造またはコードブロックの数が変わる変更であれば、AC-29 のテストが CI で検出する。文言だけの変更の同期漏れは、テストでは検出できず、AC-37 の手順と PR のレビューで防ぐ。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書を読むテストは、値・構造・実行できる例を確かめ、文の意味は確かめない（CLAUDE.md、Testing Strategy の "Document tests check values, structure, and executable examples, never prose"）。AC-29 の構造の一致のテストは、構造を確かめるテストに当たる。AC-27 の翻訳の内容、AC-10 の翻訳の内容、AC-11 の英語版の内容が日本語版に含まれることは、PR のレビューでの読み合わせ（`manual`）で確かめる。
-   旧パスの参照が残っていないこと（AC-13）や、過去のタスク文書の変更がパスだけであること（AC-14）は、変更そのものの性質である。そのため、文書のテストではなく、本タスクの `_checks/` の下のスクリプトによる、マージベースとの差分の確かめ（`requirements_process.md` §4）で確かめる。
-   README の変更は、拡張の node のテストに影響する（CLAUDE.md、Development Notes）。README を変える PR では `make ext-test` を実行する。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書で使う用語のうち、用語集にないものを次に定める。

-   **利用者:** 拡張の利用者。issue の「エンドユーザー」に当たる。README の読み手である。
-   **正本:** 二言語で持つ文書のうち、先に変え、もう一方の版の翻訳の元になる版。README では `README.md`（日本語）である。
-   **英語版:** `README.en.md`。正本の全文の英語訳である。
-   **開発者向けの文書:** `docs/dev/guides/cli.md` と `docs/dev/guides/development.md`。README から移した、CLI の使い方と開発の手順を書く。
-   **規約:** エージェントが従う決まりを書いた文書。`docs/dev/conventions/` に置く。
-   **手引き:** 人間がツールを使うための説明を書いた文書。`docs/dev/guides/` に置く。
-   **ガードテスト:** 文書に書かれた値や構造が、コードや設定の定義と一致することを確かめるテスト（CLAUDE.md、Testing Strategy）。
-   **構造の一致:** `README.md` と `README.en.md` で、見出しのレベルの並びと数、およびコードブロックの数が同じであること（AC-29）。
