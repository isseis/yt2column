# 要件定義書：外部の人が自動レビュアーを介して /fixpr のエージェントに指示を届ける経路を塞ぐ

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-02 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、書き込み権限のない第三者が、コメントから自動レビュアーを介して `/fixpr`・`/runpr` のエージェントに指示を届け、`make` の実行に至る経路を塞ぐための要件を定義する。経路の入口・信頼・増幅の 3 か所に手を入れ、塞がない部分を残余のリスクとして記録する。用語の意味は 6 章で定める。

**背景:**

`fixprguard` は、すべてのコメントの著者が信頼する著者であるスレッドを、信頼する著者のスレッドとする。信頼する著者は、自動レビュアーの一覧の名前の Bot と、書き込み権限を持つ人である（`internal/fixprguard/trust.go:62-103`）。自動レビュアーの一覧は、`coderabbitai` と `chatgpt-codex-connector` である（`.claude/commands/_context.md:185`）。信頼する著者のスレッドの本文は、トリアージと修正のエージェントに渡る。

本リポジトリは公開リポジトリである。そのため、次の経路がある（issue #629）。

1.  **入口:** 書き込み権限のない人が、issue や PR へのコメントで自動レビュアーに指示する。自動レビュアーは、攻撃者の文言を含むレビュースレッドを Bot 名義で作りうる。
2.  **信頼:** このスレッドの著者は Bot だけなので、信頼する著者のスレッドになる。本文はエージェントに届き、残る守りは区切りのブロックだけである。区切りのブロックは、エージェントがその中の指示に従わないことを保証しない（`docs/dev/guides/fixprguard_guide.md` 7.2.2）。
3.  **増幅:** 範囲の規則は、信頼する著者のスレッドの `path` が実行関連ファイルなら、その `path` そのものの変更を範囲内とする（`internal/fixprguard/scope.go:232-234`、`docs/dev/guides/fixprguard_guide.md` 6 章の規則 4）。修正のフェーズが `Makefile` を書き換えると、`finish` は利用者の確認なしに、書き換え後の `Makefile` で `make fmt`・`make test`・`make lint` を実行する。
4.  **被害:** `make` に渡す環境は、`GIT_*` を除いてセッションの環境変数を引き継ぐ（`cmd/fixprguard/main.go:164`、`internal/cmdrunner/git.go:36-45`）。

要件定義の際に、次のことを確かめた（2026-11-02）。

-   **CodeRabbit:** 応答する相手を限る設定は `chat.allow_non_org_members` だけで、organization のリポジトリにしか効かない（[CodeRabbit configuration reference](https://docs.coderabbit.ai/reference/configuration)）。本リポジトリは個人のリポジトリなので、設定では限れない。また、`.coderabbit.yaml` は自動レビューを無効にしている（`.coderabbit.yaml:13-14`）。それでも CodeRabbit は、PR #617・#620 で、PR が開いた約 10 秒後にレビュースレッドを 3 件ずつ作った。
-   **Codex:** 公式の文書（[Codex code review in GitHub](https://learn.chatgpt.com/docs/third-party/github.md)）には、誰が `@codex` で Codex を起動できるかの制限が書かれていない。`review` 以外の言葉を添えたメンションは cloud chat を始め、権限があれば修正を push しうる、とある。
-   **秘密の置き場所:** 秘密は、作業ツリーの最上位の `.envrc` に置かれる（`CLAUDE.md`「Development Notes」）。`make` が実行するコードは、このファイルを読める。このセッションの環境変数を名前だけで確かめたところ、issue が挙げたパターン（`*_API_KEY`・`*_TOKEN`・`*_WEBHOOK_URL`・`*_SECRET`）に当たるのは `CLAUDE_CODE_MESSAGING_TOKEN` だけだった。push に使える `SSH_AUTH_SOCK` は、パターンに当たらない。

対応 issue: [#629](https://github.com/isseis/yt2column/issues/629)（統合した issue: [#427](https://github.com/isseis/yt2column/issues/427)、[#397](https://github.com/isseis/yt2column/issues/397)、[#428](https://github.com/isseis/yt2column/issues/428) の項目 2。関連: [#395](https://github.com/isseis/yt2column/issues/395)、タスク `0030_fixpr_untrusted_threads`。[#425](https://github.com/isseis/yt2column/issues/425)、タスク `0036_fixprguard_exec_file_scope`。[#405](https://github.com/isseis/yt2column/issues/405)）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。範囲の規則がどの変更を範囲外にするか、どの Bot のスレッドを信頼するか、利用者が行う GitHub の設定、`/runpr` が示す警告、ガイドに書かれることである。次のことは設計（`02_architecture.md`）で決める。

-   interaction limits の状態を読む処理を置く場所（`fixprguard` のサブコマンドか、コマンドの文書の `gh` の呼び出しか）と、応答の検査の手段
-   期限の比較に使う現在の時刻の取り方
-   ガイドへの記述の載せ方と、文書のガードの更新
-   テストの手段

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #629 で決めたこと:**

-   **経路を、入口・増幅・被害の 3 か所で塞ぐことを検討する:** 本書の F-001〜F-004 が入口と増幅を扱う。被害については、要件定義の際に、仕組みを足さないと決めた（下記）。
    -   理由: 1 か所だけを塞いでも、ほかの 2 か所が残れば経路は開いたままである。
-   **コマンドの側の守りは残す:** タスク 0030 の F-001・F-002（想定外の著者のスレッドを処理しない守り）は変えない。
    -   理由: GitHub の設定が外れたときの安全策である。
-   **#405（複数の自動レビュアー）で Bot を増やすときは、この経路を Bot ごとに評価する:** 本書はその手順を定めないが、ガイドの残余のリスクに書く（AC-21）。
    -   理由: 新しい Bot が外部の人の指示に応じるかは、Bot ごとに違う。

**要件定義の際に決めたこと（2026-11-02）:**

-   **Bot の挙動は、要件定義の際にベンダーの文書で確かめる:** 結果は「背景」に書いた。
    -   理由: 確かめていない前提の上に要件を立てないためである。
-   **CodeRabbit を自動レビュアーの一覧から外す（F-002）:** CodeRabbit のスレッドは、`other-bot` として想定外の著者のスレッドになる。利用者が名指ししない限り処理されない。
    -   理由: CodeRabbit は、個人のリポジトリでは応答する相手を設定で限れない。PR レビューは Codex が行い（`CLAUDE.md`「Claude Commands」）、CodeRabbit は手順の上で PR にスレッドを作る役を持たない。
    -   受け入れる負担: CodeRabbit が意図せず作ったスレッド（PR #617・#620 の例）は、`/fixpr` が自動では直さず、想定外の著者のスレッドとして利用者の判断に回る。
-   **Codex は、外部の人の指示に応じうるものとして扱う:** Codex は自動レビュアーの一覧に残す。入口の守りは interaction limits（F-003）に頼り、制限が外れている間を残余のリスクとする（4.2）。
    -   理由: Codex は PR レビューを行う唯一の自動レビュアーで、一覧から外せない。文書では応じないことを確かめられないので、安全側に倒す。
-   **interaction limits と、PR・issue の作成の制限を使う（F-003）:** interaction limits の `collaborators_only` を有効にする。あわせて、PR の作成と issue の作成を collaborators に限る恒久的な設定も使う。設定は利用者が行う。
    -   理由: 書き込み権限のない人のコメントを止められるのは interaction limits だけである。ただし最長 6 か月で解除されるので、期限のない作成の制限を重ねる。
    -   受け入れる負担: 外部の人は、本リポジトリに issue（脆弱性の報告を含む）・PR・コメントを作れなくなる。
-   **`/runpr` が interaction limits の状態を確かめて警告する（F-004）:** 制限がないとき、制限が弱いとき、期限まで 14 日以内のとき、状態を確かめられないときに警告し、処理は続ける。
    -   理由: 期限で自動的に解除されるので、気づく仕組みが要る。止めると、制限が外れている間の PR がすべて止まる。警告で利用者が更新できれば足りる。
-   **規則 4 の例外を、信頼する著者のスレッドについても外す（F-001）:** 実行関連ファイルは、`path` そのものでも、どの著者のスレッドでも範囲外とする。
    -   理由: 範囲外の変更は利用者の確認に回る（`docs/dev/guides/fixprguard_guide.md` 6 章）。build checks の前に利用者が見ることが目的なので、手間は小さい。
-   **同じパッケージの `.go` のファイルの規則は変えない:** 信頼する著者のスレッドの `path` が `.go` のファイルなら、同じディレクトリの `.go` のファイルは範囲に入り（`internal/fixprguard/scope.go:244-246`）、`make test` がそれを実行する。この経路は残余のリスクとして記録する（4.2）。
    -   理由: 狭めると、日常の修正（同じパッケージのテストの修正）のたびに確認が入る。入口は F-002・F-003 で塞ぐ。
-   **`make` に渡す環境から秘密を落とさない:** issue の案（#428 の項目 2）は採らない。`make` が実行するコードが秘密を使えることは、残余のリスクとして記録する（4.2）。
    -   理由: 秘密は `.envrc` からファイルとして読めるので、環境変数を落としても被害はほとんど減らない（「背景」）。前提が成り立たない仕組みは足さない（`CLAUDE.md`「Performance」の「Do not close a review finding by adding a mechanism until the finding's premise is verified」）。
-   **CodeRabbit が設定に反して自動レビューを行う件は、本書の範囲外とする:** 調査と対応は別の issue で扱う。
    -   理由: F-002 で CodeRabbit のスレッドは信頼されなくなるので、本タスクの経路には影響しない。原因の調査は本タスクの目的と別である。

以下の決定は、要件定義書を作成する際に、安全側の既定として選んだ。PR のレビューで確かめてほしい。

-   **`protocol` の番号を上げる（F-005）:** 本タスクの前のツールは、新しい文書から呼ばれると終了コード `2` で止まる。
    -   理由: 前のツールは、信頼する著者のスレッドの `path` そのものなら実行関連ファイルを範囲内とする。番号が同じなら、入れ直していない利用者は、古い規則で判定されていることに気づけない（`docs/dev/guides/fixprguard_guide.md` 3 章）。タスク 0036 と同じ扱いである。
-   **interaction limits の制限が `collaborators_only` より弱ければ警告する（AC-15）:** `existing_users`・`contributors_only` は、書き込み権限のない人のコメントを許すので、制限がないときと同じく警告する。
    -   理由: 守りたいのは、書き込み権限のない人のコメントを止めることである。
-   **警告は、`/fixpr` の単独の実行では行わない:** `/runpr` だけが確かめる。
    -   理由: PR は `/runpr` が駆動する（`CLAUDE.md`「Claude Commands」）。`/fixpr` を単独で実行する場合の扱いは、残余のリスクとして記録する（4.2）。
-   **0030 の設計書と 0036 の要件定義書に、規則が変わったことの注記を加える（AC-22）:** 決定を変えない注記なので、文書の状態は `approved` のままとする（要件定義のプロセスのガイド「Editing an approved document」の Editorial correction）。
    -   理由: ガイドは、設計の背景として 0030 の設計書を挙げる（`docs/dev/guides/fixprguard_guide.md:5`）。古い規則のまま残すと、読み手が今の規則を誤解する。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   書き込み権限のない人が、自動レビュアーを介して、信頼する著者のスレッドを作れないようにする。
-   信頼する著者のスレッドからの修正でも、実行関連ファイルの変更が、利用者の確認なしに build checks で使われないようにする。
-   GitHub の設定が外れたときに、利用者が気づけるようにする。
-   塞がない経路を、残余のリスクとして文書に残す。

### 2.2. スコープ (In Scope)

-   範囲の規則で、実行関連ファイルを常に範囲外にする（F-001）。
-   CodeRabbit を自動レビュアーの一覧から外す（F-002）。
-   interaction limits と、PR・issue の作成の制限を設定する手順を文書にし、設定する（F-003）。
-   `/runpr` が interaction limits の状態を確かめて警告する（F-004）。
-   `protocol` の番号を上げる（F-005）。
-   ガイド・0030 と 0036 の文書・用語集を合わせる（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **`make` に渡す環境から秘密を落とすこと:** 1 章の決定により採らない。残余のリスクとして 4.2 に記録する。
-   **CodeRabbit が設定に反して自動レビューを行う件:** 別の issue で扱う（1 章の決定）。
-   **同じパッケージの `.go` のファイルの規則:** 変えない（1 章の決定）。
-   **`/fixpr` の単独の実行での警告:** 行わない（1 章の決定）。
-   **#428 の残りの項目（基準値の健全性検査、`reply`・`edit-pr` の fork の検査、`inspect` の確認の明示）、#426、#446:** それぞれの issue で扱う。
-   **書き込み権限を持つ人のコメントの扱い:** 今までどおり信頼する。
-   **製品のセキュリティの文書（`docs/dev/security.md`）:** 対象は製品であり、本タスクは開発の手順の守りなので変えない（タスク 0030 と同じ扱い）。
-   **Codex の設定:** Codex の自動レビューの設定はリポジトリの外にあり（`CLAUDE.md`「Claude Commands」）、変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下で「判定」とは、`fixprguard inspect` が、変更したパス・変更の種類・スレッドの `path`・名指しの有無から、その変更が範囲に入るかを決めることをいう。判定の結果は、`inspect` の `outOfScope` に現れる（範囲外ならその要素になり、`inspect` は終了コード `3` を返す）。各 AC の例は、断りがなければ、変更が通常のファイルの変更である場合を示す。

#### F-001: 実行関連ファイルを常に範囲外にする

実行関連ファイルの変更は、スレッドの著者と `path` にかかわらず範囲外とする。実行関連ファイルの定義（`docs/dev/guides/fixprguard_guide.md` 6 章の表）は変えない。

**Acceptance Criteria**:
- **AC-01**: 信頼する著者のスレッドの `path` が実行関連ファイルで、変更したパスがその `path` そのものであるとき、変更は範囲外になる。少なくとも次の各行で範囲外になる。

    | 変更したパス（= スレッドの `path`） |
    |---|
    | `Makefile` |
    | `.golangci.yml` |
    | `.github/workflows/ci.yml` |
    | `internal/writer/CLAUDE.md` |
    | `extension/package.json` |
    | `go.mod` |

- **AC-02**: 名指しされた想定外の著者のスレッドでも、AC-01 の各行は範囲外である（今の振る舞いと同じ）。
- **AC-03**: 実行関連ファイルでないファイルの判定は変わらない。少なくとも次の各行の判定が、本タスクの前と同じである。

    | 変更したパス | スレッドの `path` | 名指し | 判定 |
    |---|---|---|---|
    | `internal/writer/writer.go` | `internal/writer/writer.go` | なし | 範囲内 |
    | `internal/writer/writer_test.go` | `internal/writer/writer.go` | なし | 範囲内 |
    | `internal/writer/writer_test.go` | `internal/writer/writer.go` | あり | 範囲外 |
    | `internal/writer/notes.md` | `internal/writer/writer.go` | なし | 範囲内 |
    | `scripts/check.sh` | `scripts/check.sh` | なし | 範囲内 |

- **AC-04**: AC-01 の変更で `inspect` が終了コード `3` を返した後、利用者が続けることを選べば、`finish` は今までどおりその変更を commit できる。

#### F-002: CodeRabbit を自動レビュアーの一覧から外す

自動レビュアーの一覧を `chatgpt-codex-connector` だけにする。

**Acceptance Criteria**:
- **AC-05**: `.claude/commands/_context.md` の「Automated reviewer list」の名前は `chatgpt-codex-connector` だけである。`/fixpr`・`/runpr` が `fixprguard` に渡す `--reviewer` は `chatgpt-codex-connector` だけである。
- **AC-06**: コメントの著者に `coderabbitai` の Bot を含むスレッドは、`/fixpr` の実行で想定外の著者のスレッドになり、理由 `other-bot`（アカウント `coderabbitai`）として利用者に示される。そのスレッドの本文は取得されず、名指しされない限り処理されない。
- **AC-07**: 利用者が AC-06 のスレッドを名指しすると、そのスレッドは名指しされた想定外の著者のスレッドとして処理される。範囲の規則は名指しのスレッドのもの（`path` そのもの以外の `.go` のファイルは範囲外、実行関連ファイルは範囲外）を使う。
- **AC-08**: ガイドの説明と例は、`coderabbitai` を信頼する自動レビュアーとして挙げない。`--reviewer` の例は、自動レビュアーの一覧にある名前を使う。

#### F-003: GitHub の設定で、書き込み権限のない人の操作を制限する

利用者は、本リポジトリに次の 3 つを設定する。エージェントは設定を変えない。

1.  interaction limits の `collaborators_only`（期限は 6 か月）
2.  PR の作成を collaborators に限る設定
3.  issue の作成を collaborators に限る設定

**Acceptance Criteria**:
- **AC-09**: `docs/dev/guides/` の下のガイドに、3 つの設定の目的、設定する手順、interaction limits を更新する手順（期限が来る前の更新を含む）、設定を変えるのは利用者でありエージェントは変えないことが書かれている。
- **AC-10**: 本タスクの最後の PR のマージの前に、利用者が 3 つの設定を行い、次の状態になっている。interaction limits の取得の結果が `limit` `collaborators_only` で期限が 14 日より先である。リポジトリの設定画面で、PR の作成と issue の作成が collaborators に限られている。
- **AC-11**: AC-10 の設定の後に開いた PR で、Codex の PR レビューが今までどおり投稿される。投稿されなければ、作業を止め、本書の決定を見直す（決定の変更）。

#### F-004: `/runpr` が interaction limits の状態を確かめて警告する

`/runpr` は、実行ごとに 1 回、Codex のレビューを待ち始める前に、本リポジトリの interaction limits の状態を読み、次のどれかに分類する。

| 状態 | 条件 | 振る舞い |
|---|---|---|
| 有効 | `limit` が `collaborators_only` で、期限が今から 14 日より先 | 警告しない |
| 期限が近い | `limit` が `collaborators_only` で、期限が今から 14 日以内（今を過ぎていない） | 警告する |
| 制限なし | 制限がない応答、または期限が今以前 | 警告する |
| 制限が弱い | `limit` が `existing_users` または `contributors_only` | 警告する |
| 確かめられない | 取得の失敗、または応答が 4.2 の受け入れる形でない | 警告する |

**Acceptance Criteria**:
- **AC-12**: `/runpr` は、実行ごとに 1 回、Codex のレビューを待ち始める前に、interaction limits の状態を読む。読む操作は `/runpr` の「Authorization」に挙げられている。
- **AC-13**: 状態が「有効」なら、`/runpr` は警告を出さず、最終の報告に期限の日付を 1 行で示す。
- **AC-14**: 状態が「期限が近い」「制限なし」「制限が弱い」なら、`/runpr` は、状態の名前、（あれば）期限の日付、F-003 の更新の手順があるガイドの場所を含む警告を、日本語で、確かめた時点と最終の報告の両方に示す。処理は止めない。
- **AC-15**: 状態が「制限が弱い」のときの警告は、`collaborators_only` に変える必要があることを示す。
- **AC-16**: 状態が「確かめられない」なら、`/runpr` は、確かめられなかったことと、利用者が設定画面で確かめる必要があることを、AC-14 と同じ 2 か所に示す。処理は止めない。警告に応答の本文を含めない。
- **AC-17**: `/runpr` は、interaction limits と PR・issue の作成の設定を変える操作（`PUT`・`DELETE` など）を行わない。
- **AC-18**: 期限の判定の境界: 期限が今からちょうど 14 日後のときは「期限が近い」、14 日と 1 秒後のときは「有効」になる。期限が今と同じ時刻のときは「制限なし」になる。

#### F-005: `protocol` の番号を上げる

**Acceptance Criteria**:
- **AC-19**: `fixprguard` の `protocol` の番号は `4` から `5` になる。`/fixpr`・`/runpr` の文書とガイドの例は `--protocol 5` を渡す。本タスクの前のツールに `--protocol 5` を渡すと、終了コード `2` で止まる。本タスクの後のツールに `--protocol 4` を渡すと、終了コード `2` で止まる。

#### F-006: 文書を合わせる

**Acceptance Criteria**:
- **AC-20**: `docs/dev/guides/fixprguard_guide.md` 6 章の規則 4 と「判定の順序」は、実行関連ファイルが、著者と `path` にかかわらず範囲外であることを示す。
- **AC-21**: ガイドの 7.2（保証しないこと）に、4.2 の残余のリスクがすべて書かれている。
- **AC-22**: タスク 0030 の `02_architecture.md` 5.1・5.3 と、タスク 0036 の `01_requirements.md` 1 章に、本タスクで規則と自動レビュアーの一覧が変わったことと、本書への参照が加わっている。両文書の状態は `approved` のままである。
- **AC-23**: `docs/translation_glossary.md` の「実行関連ファイル」の説明は F-001 の規則に合い、「interaction limits」の項目が加わっている。
- **AC-24**: `make test`・`make lint`・`doc-guard` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `/runpr` の 1 回の実行で、interaction limits を読む操作は 1 回である（AC-12）。

### 4.2. セキュリティ (Security)

条件付きのセキュリティガイド（`docs/dev/security.md`）のトリガーのうち、「ネットワークでデータを送る」に当たる。ただし同書は製品を対象にした文書なので、変えない（2.3）。本タスクの守りは、開発の手順の守りである。

**信頼できない入力: interaction limits の取得の応答。** GitHub の `GET /repos/{owner}/{repo}/interaction-limits`（[REST API endpoints for repository interactions](https://docs.github.com/en/rest/interactions/repos)）の応答を、次のとおり扱う。

-   受け入れる形は次の 2 つだけである。
    1.  制限がないことを示す応答。文書は「制限がなければ空の応答」とする。どのバイト列を空の応答として受け入れるかは、実際の応答を観測して設計で決める（design_handoff.md の H-01）。決めた形のほかは受け入れない。
    2.  HTTP の状態が `200` で、本文が 1 つの JSON のオブジェクトである応答。
-   2 の形で使う項目は `limit` と `expires_at` で、どちらも必須の文字列である。`limit` は `existing_users`・`contributors_only`・`collaborators_only` のどれかである。`expires_at` は、時差を含む RFC 3339 の日時である。ほかの項目（`origin` など）は使わず、拡張可能として無視する。GitHub が項目を加えうるためである。
-   次のものは、受け入れずに「確かめられない」とする。
    -   有効な UTF-8 でない本文、JSON の文字列の対になっていないサロゲートのエスケープ
    -   オブジェクトでない最上位の値、`null`、2 つ目の値や後続のデータ
    -   使う項目の欠落、重複、文字列でない値
    -   一覧にない `limit` の値
    -   解釈できない `expires_at`
    -   上限（設計で決める。数十 KiB を目安とする）を超える本文
    -   `200` でない HTTP の状態、取得の失敗
-   標準ライブラリが黙って修復・破棄・無視する入力（不正なバイトの置き換え、重複した項目の後勝ち、後続のデータの読み残し）は、修復せずに「確かめられない」とする。
-   拒否は「確かめられない」の状態に写す。Go のコードで実装する場合、拒否は 1 つの番兵のエラーに写す（名前は設計で決める）。拒否のときに部分的な結果を使わない。

**残余のリスク。** 次の経路は塞がない。ガイドの 7.2 に書く（AC-21）。

-   **制限が外れている間の Codex の経路。** interaction limits の期限が切れてから利用者が更新するまでの間、または `/fixpr` を単独で実行する場合に警告がないまま、書き込み権限のない人は Codex に指示しうる。Codex 名義のスレッドは、信頼する著者のスレッドになる。Codex が外部の人の指示に応じるかは、確かめていない。
-   **同じパッケージの `.go` のファイル。** 信頼する著者のスレッドの `path` が `.go` のファイルなら、同じディレクトリの `.go` のファイルは確認なしに範囲に入り、`make test` がそれを実行する（1 章の決定）。上の経路でスレッドが作られると、第三者の文言がこの実行に至りうる。
-   **`make` が実行するコードが使える秘密。** `make` が実行するコードは、作業ツリーの `.envrc` の秘密、引き継いだ環境変数（`SSH_AUTH_SOCK` を含む）、`gh` の資格情報を使える（1 章の決定）。
-   **名指しした CodeRabbit のスレッド。** 利用者が名指しすれば、その本文はエージェントに届く。名指しの前に、利用者が内容を確かめる。
-   **確認への慣れ。** 実行関連ファイルの変更は、正当な修正でも毎回確認に回る。確認に慣れると、危険な変更を見落としうる。
-   **新しい自動レビュアー。** 自動レビュアーの一覧に Bot を加えるときは、その Bot が外部の人の指示に応じるかを評価する（#405）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   interaction limits を読めないことで、`/runpr` は止まらない（AC-16）。

### 4.4. 互換性 (Compatibility)

-   古いツールと新しい文書、新しいツールと古い文書の組み合わせは、どちらも終了コード `2` で止まり、古い規則で判定しない（AC-19）。
-   すでに PR にある CodeRabbit のスレッドは、本タスクの後の `/fixpr` では想定外の著者のスレッドになる（AC-06）。

### 4.5. 保守性 (Maintainability)

-   自動レビュアーの一覧は、今までどおり `.claude/commands/_context.md` の 1 か所で定める。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Tool Execution Safety」に従う。interaction limits を読む操作はネットワークの操作なので、`/runpr` の「Authorization」に挙げる（AC-12）。設定を変える操作は、エージェントは行わない（AC-17）。
-   外部のモジュールを加えない（`CLAUDE.md`「Dependencies」）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書で新たに使う用語は次のとおり。

-   **interaction limits:** GitHub のリポジトリの設定で、指定した範囲の外のアカウントによる、コメント・issue・PR の作成を一定の期間制限するもの。`collaborators_only` は、書き込み権限を持つ人（collaborators）のほかを制限する。期限は最長 6 か月で、期限が来ると自動的に解除される。
-   **作成の制限:** PR の作成と issue の作成を collaborators に限る、期限のないリポジトリの設定。コメントは制限しない。

既存の用語（信頼する著者、信頼する著者のスレッド、想定外の著者のスレッド、自動レビュアー、自動レビュアーの一覧、実行関連ファイル、名指し）は、用語集の定義に従う。
