# 要件定義書：旧 API（`/articles`）とホストの今の形の中継の削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-03 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張がジョブの API に移った後に、localhost サーバの旧 API（`POST /articles`）と、Native Messaging のホストの今の形のメッセージの中継を削除するための要件を定める。あわせて、新旧の API を両方扱うために複雑になったところを簡素化する（[#526](https://github.com/isseis/yt2column/issues/526)）。

**背景:**

タスク `0044_article_job_api`（[#502](https://github.com/isseis/yt2column/issues/502)）は、サーバとホストにジョブの API（生成の開始・状態の取得・投稿の操作）を加えた。その時点では、拡張がまだ `/articles` を使っていたので、サーバは両方の API を受け付けた（`0044` F-007）。ホストは、メッセージが 1 つの厳格な JSON オブジェクトで、最上位に `v` を持つかどうかで今の形と新しい形を区別した（`0044` F-008、`internal/nmhost/relay.go:457-470`）。この区別は、CLAUDE.md の「Declare, don't infer」の例外として記録されている。`0044` は、拡張が移った後に削除するものと簡素化するものの一覧を、設計書の §3.14（`docs/tasks/0044_article_job_api/02_architecture.md`）に残した。issue #526 の本文は、この一覧である。

タスク `0050_viewer_tab`（[#503](https://github.com/isseis/yt2column/issues/503)、完了）で、拡張はジョブの API に移り、今の形のメッセージを送らなくなった（`docs/tasks/0050_viewer_tab/01_requirements.md` 1 章「issue #503 で決めたこと」と 2.3）。サーバの `/articles` とホストの今の形の中継の削除は、#526 に残された（同 2.3）。#503 の後も、次のものが残っている。

-   サーバは `/articles` を受け付け、旧 API のジョブ（パイプラインで生成し、Webhook に投稿する）を走らせ、失敗を Webhook への失敗の通知で報告する（`internal/server/handler.go:12-14`・`:49-51`・`:96-`、`internal/server/job.go:44-114`・`:118-142`・`:190-222`）。Webhook の設定は、旧 API のための `Notifier` を持つ（`internal/server/server.go:64-68`）。
-   生成の枠（`2`）は、旧 API のリクエスト・旧 API のジョブ・失敗の通知と、ジョブの API の生成のジョブが合わせて数える（`internal/server/job.go:17-22`、`internal/server/server.go:84-89`）。
-   シャットダウンのログと停止の案内は、`/articles` のジョブと失敗の通知を待つ対象に挙げる（`internal/server/server.go:262-273`、`cmd/yt2column-server/run.go:237-243`）。停止の案内の「約 45 分」は、旧 API のジョブの LLM の呼び出し 3 回分である（`cmd/yt2column-server/run.go:40-43`）。
-   ホストは、`v` を持たないメッセージと、1 つの厳格な JSON オブジェクトとして読めないメッセージを、バイト列を変えずに `POST /articles` へ中継する（`internal/nmhost/relay.go:55`・`:466-470`・`:528-530`）。
-   文書は、`/articles`・失敗の通知・今の形の中継を説明している（README の「localhost サーバ」節、`docs/dev/guides/development.md`、`docs/dev/security.md` §8、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、CLAUDE.md）。

対応 issue: [#526](https://github.com/isseis/yt2column/issues/526)（親: [#501](https://github.com/isseis/yt2column/issues/501)、前提: [#503](https://github.com/isseis/yt2column/issues/503)、関連: [#502](https://github.com/isseis/yt2column/issues/502)）

**本書の記述範囲:** 本書は、削除と簡素化の後に観測できる振る舞い（サーバとホストが何を受け付け、何を拒否し、何を投稿し、停止のときに何を待つか）と、削除するものと書き換える文書の範囲を定める。コードの構造（型、関数、テストの組み替え）、PR の分け方、検証の手段は、設計（`02_architecture.md`）で決める。要件の作成中に挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #526 で決めたこと:**

-   **削除するもの:** 旧 API だけが使うもの（`0044` 設計書 §3.14 の削除の 8 項目）を削除する。サーバの `/articles` の受け口と旧 API のジョブ、失敗の通知（`internal/failure`、`SlackFailureNotifier` と、それだけが使うエラー、サーバの `Notifier`）、ホストの今の形の中継と、その形の判定、CLAUDE.md の例外の記述、文書の `/articles` の記述、`/articles` の curl の例のガード、である。#503 で拡張が旧 API を使わなくなったので、残す理由がない。
-   **`v` を持たないメッセージは `protocol_error`:** 削除の後、ホストは `v` を持たないメッセージを中継せず、`protocol_error` を返す。拡張は新しい形しか送らないので、今の形を受け付け続ける理由がない。
-   **簡素化するもの:** 新旧の両対応のために複雑になったもの（同 §3.14 の簡素化の 6 項目）を簡素化する。生成の枠を数える主体の区別、`webhook_not_configured` の `/articles` での使い方、`jobInput` の旧 API だけが使う値、ホストの `forward` の契約の今の形との対比、停止の案内の「約 45 分」である。
-   **CLI の `Publish` は残す:** `SlackWebhookPublisher.Publish` は CLI の `--slack` が使うので残す。サーバがそれを使わなくなることを確かめる。
-   **停止の案内は約 5 分:** 停止のときに待つのは投稿のジョブだけになるので、停止の案内の目安を最長約 5 分に改める。

**要件定義の際に決めたこと（2026-11-03）:**

-   **`/articles` は `not_found`:** 削除の後、`/articles` へのリクエストは、ほかの知らないパスと同じく `not_found`（404）で拒否する（F-001）。専用の拒否の理由は加えない。#503 の後、`/articles` を呼ぶのは手で送る `curl` くらいで、専用の理由を加えると、拒否の理由の表とそのガードを変えることになり、`/articles` のための分岐もまた残るためである。
-   **書き換える文書は、旧 API を書いている今の文書すべて:** issue の一覧が挙げる文書（README、`docs/dev/security.md`、`docs/dev/project_overview.md`、CLAUDE.md の例外）に加えて、タスク文書（`docs/tasks/`）を除き、旧 API・失敗の通知・今の形の中継を書いている文書をすべて書き換える（F-004）。issue の一覧の後、タスク 0048 の文書の分割で、HTTP API の説明は `docs/dev/guides/development.md` に移った。ほかにも `package_reference.md`・`production_profile.md`・用語集・CLAUDE.md の Architecture Overview に、旧 API の記述が残っている。一覧の文書だけを直すと、削除したものを説明する記述が残る。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   サーバが受け付ける API を、ジョブの API だけにする。
-   ホストが受け付けるメッセージを新しい形だけにし、メッセージの中身で形を選ぶ処理と、その CLAUDE.md の例外をなくす。
-   サーバが Webhook へ送るものを、利用者が求めた記事の投稿だけにする（失敗の通知をなくす）。
-   新旧の両対応のために複雑になった生成の枠の数え方・停止の待ち方・その説明を、ジョブの API だけを前提にした形に戻す。
-   文書から、削除したものの説明を取り除く。

### 2.2. スコープ (In Scope)

-   `internal/server` の `/articles` の受け口・旧 API のジョブ・失敗の通知の送信（F-001）。
-   `internal/failure` と `internal/failure/testutil`、`internal/publisher` の `SlackFailureNotifier` と、それだけが使う `ErrSlackInvalidNotice`・`ErrSlackNoticeRejected`、`cmd/yt2column-server` の失敗の通知の送り手の組み立て（F-001）。
-   `internal/nmhost` の今の形の中継と、形の判定（F-002）。
-   停止の案内とシャットダウンのログ（F-003）。
-   issue の簡素化の 6 項目（F-001〜F-003 と [design_handoff.md](design_handoff.md)）。
-   文書と、文書を読むガードの更新（F-004）。README の変更は `README.en.md` に `/mktrans` で写す（CLAUDE.md「Translation Guidelines」）。

### 2.3. スコープ外 (Out of Scope)

-   ジョブの API の振る舞いの変更。3 つの操作、拒否の理由、状態コード、応答の形、生成の枠と読み込みの枠の数、本文の上限、記事のキャッシュは変えない（F-001 AC-03）。
-   ホストの新しい形のメッセージの検査規則、送り先、ホストの応答の形、ホストのエラーの値の変更（F-002 AC-07）。
-   ブラウザ拡張の変更。拡張は #503 で今の形を送らなくなった（`docs/tasks/0050_viewer_tab/02_architecture.md` §9「#526 の今の形の削除」）。
-   CLI の変更。CLI はパイプラインと `SlackWebhookPublisher.Publish` を使い続ける（F-001 AC-05）。
-   タスク文書（`docs/tasks/`）の書き換え。過去のタスクの記録は、その時点の記述として残す。
-   ジョブの API の失敗を Webhook へ通知すること。ジョブの API の失敗はビューアが表示する（`0044` 設計書 §3.13「新しい API の失敗を Webhook へ通知しない」）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: サーバの旧 API と失敗の通知の削除

サーバは `/articles` を受け付けず、旧 API のジョブを走らせず、失敗の通知を Webhook へ送らない。ジョブの API の振る舞いは変えない。

**Acceptance Criteria**:
- **AC-01**: リクエストのターゲットが `/articles` のリクエストは、メソッドによらず、ほかの知らないパスと同じく `not_found`（404）で拒否される。サーバは本文を読まず、生成の枠も読み込みの枠も取らず、記事を生成せず、Webhook へ何も送らない。
- **AC-02**: サーバは、どの拒否・生成の失敗・投稿の失敗・パニックでも、失敗の通知を Webhook へ送らない。サーバが Webhook へ送るのは、投稿の操作が始めた投稿のジョブの記事だけである。
- **AC-03**: ジョブの API の 3 つの操作（生成の開始・状態の取得・投稿の操作）の受理と拒否の理由、状態コード、応答の本文の形、生成の枠の数（`2`）、読み込みの枠の数（`2`）、本文の上限（4 MiB）は変わらない。生成の枠を数えるのは、ジョブの API の生成のジョブだけになる。
- **AC-04**: `SLACK_WEBHOOK_URL` が未設定のとき、サーバは今と同じく起動し、投稿の操作を `webhook_not_configured` で拒否する。設定されていれば、サーバは記事の投稿のためだけに Webhook を使い、失敗の通知の送り手を組み立てない。サーバの起動が、失敗の通知の送り手の組み立ての失敗で終わることはない。
- **AC-05**: CLI の振る舞い（`--slack` での投稿を含む）は変わらない。
- **AC-06**: リポジトリの Go のコード（テストを含む）に、次のものが残らない: `ArticlesPath`、旧 API のジョブとその失敗の分類・失敗の種類への対応づけ、失敗の通知の送信、`internal/failure` と `internal/failure/testutil`、`SlackFailureNotifier`・`NewSlackFailureNotifier`・`ErrSlackInvalidNotice`・`ErrSlackNoticeRejected`、`cmd/yt2column-server` の失敗の通知の送り手の組み立て、サーバの `internal/pipeline` の使用。

#### F-002: ホストの今の形の中継の削除

ホストは新しい形のメッセージだけを中継する。それ以外のメッセージは、サーバへリクエストを送らずに `protocol_error` で拒否する。

**Acceptance Criteria**:
- **AC-07**: 新しい形のメッセージ（`0044` F-008 の受理する形: 1 つの厳格な JSON オブジェクトで、最上位のメンバは `v`・`op`・`input`・`key` だけ、重複なし、`v` は整数の `1`、`op` は `start`・`status`・`publish` のどれかで、それに合う `input` か `key` を持つ）の中継は変わらない。送り先（`POST /jobs`・`GET /jobs/{key}`・`POST /jobs/{key}/publish`）、送る本文、ホストの応答の形、32 MiB のメッセージの上限、512 KiB のサーバの応答の上限、20 秒の中継の時間の上限は、今と同じである。
- **AC-08**: 1 つの厳格な JSON オブジェクトとして読めるが、最上位に `v` を持たない（エスケープを解いた名前で比べる）メッセージには、ホストはサーバへリクエストを送らずに `protocol_error` を返す。`v` 以外のメンバ（`op` など）がそろっていても同じである。
- **AC-09**: 1 つの厳格な JSON オブジェクトとして読めないメッセージ（正しい UTF-8 でない、JSON として正しくない、最上位がオブジェクトでない、後続のデータがある、対になっていないサロゲートのエスケープを含む）には、ホストはサーバへリクエストを送らずに `protocol_error` を返す。中身を直して送ることはしない。
- **AC-10**: ホストの Go のコードに、`/articles` の送り先と、今の形を表す値が残らない。ホストがメッセージの中身で中継の形を選ぶ処理はなくなり、メッセージは新しい形として検査されるか、拒否されるかのどちらかになる。
- **AC-11**: AC-08・AC-09 の拒否のとき、ホストのログには、今と同じく固定の値（段階・規則の名前など）だけを書き、メッセージのメンバ名や値を書かない。

#### F-003: 停止

停止のときにサーバが待つものと、その案内を、ジョブの API だけを前提にした形にする。

**Acceptance Criteria**:
- **AC-12**: 最初のシグナルの後に書く停止の案内は、待つ対象をビューアから送った投稿（投稿のジョブ）とし、待つ時間の最長の目安を約 5 分とする。`/articles` のジョブ、失敗の通知、LLM の呼び出しの回数に基づく「約 45 分」は案内に出ない。実行中の生成を取り消し、その記事が失われることは、今と同じく案内に出る。
- **AC-13**: シャットダウンのログは、待つ対象として `/articles` のジョブと失敗の通知を挙げない。待っている数を書くときは、今も残っている種類のジョブ（投稿のジョブと、取り消した生成のジョブ）だけを数える。
- **AC-14**: 停止の振る舞いは変わらない。最初のシグナルで、接続の受け付けをやめ、ソケットを消し、実行中の生成を取り消し、投稿のジョブが終わるのを待つ。2 つ目のシグナルですぐに終わる。ジョブが終わるまで、新しいサーバは起動を断る。

#### F-004: 文書とガード

タスク文書を除く今の文書から、削除したものの説明を取り除き、残る振る舞いに合わせる。

**Acceptance Criteria**:
- **AC-15**: 次の文書は、`/articles`、旧 API のジョブ、失敗の通知、ホストの今の形の中継を、今あるものとして説明しない: `README.md` と `README.en.md`、`docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、CLAUDE.md。生成の枠の説明は、ジョブの API の生成のジョブだけが数えることを書く。停止の説明は、F-003 の振る舞い（待つのは投稿のジョブで、最長およそ 5 分）を書く。版がそろっていない場合の説明（`README.md:562-564`）は、今の形のメッセージを送る古い拡張が `protocol_error` を受け取ることに合わせる。
- **AC-16**: CLAUDE.md の「Declare, don't infer」から、Native Messaging のホストが今の形と新しい形をメッセージの中身で選ぶ例外の段落を取り除く。CLAUDE.md の Architecture Overview から、`internal/failure` の記述を取り除く。
- **AC-17**: `docs/dev/guides/development.md` の curl の例は、`/articles` へのリクエストを含まない。curl の例のガード（`cmd/yt2column-server/docs_test.go`）は、ジョブの API の 3 つの操作の例を求め、`/articles` の例を求めない。サーバが動いているかを確かめる例は、ジョブの API のパスを使う。
- **AC-18**: 文書を読む既存のガード（README と `development.md` の拒否の理由の表、`security.md` の枠の数と本文の上限など）は、変更の後も通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。削除と簡素化で、ジョブの API の処理の量は変わらない。

### 4.2. セキュリティ (Security)

本タスクは Webhook URL を扱うコードと、ネットワークへ送るコード（Webhook への投稿）を変えるので、`docs/dev/security.md` を適用する。

-   **Webhook へ送るものが減る:** サーバが Webhook へ送るのは、利用者が投稿の操作で求めた記事だけになる（AC-02）。失敗の通知（リクエスト ID・動画 ID・失敗の種類）は送られなくなる。
-   **秘密の扱いは変えない:** Webhook URL と API キーは今と同じく `secret.Secret` で持ち、ログと応答に出さない。ログを伏せ字にする仕組み（`internal/redact`）は変えない。
-   **二重の生成と投稿のリスクがなくなる:** `docs/dev/security.md` §8.2 が `/articles` の残るリスクとして挙げる、`202` が届かずに送り直したときの二重の生成と投稿（`docs/dev/security.md:155`）は、`/articles` とともになくなる。§8 の記述を合わせる（AC-15）。
-   **ホストの入力の境界:** ホストがブラウザから受け取るメッセージの受理する形は、新しい形だけになる（AC-07〜AC-09）。
    -   受理する形: `0044` F-008 の新しい形（AC-07）。消費するメンバは `v`・`op`・`input`・`key` で、拡張可能と宣言するメンバはない。宣言していないメンバ、重複するメンバ、`op` に合わない余分なメンバ、欠けたメンバは拒否する（今と同じ）。
    -   拒否の対応: 受理する形でないメッセージは、すべてホストのエラー `protocol_error` になり、サーバへリクエストを送らない（AC-08・AC-09）。新しいホストのエラーの値は加えない。
    -   上限: メッセージの長さの上限 32 MiB と、その超過の `message_too_large`、長さ 0 の `protocol_error` は変えない。
    -   標準ライブラリが黙って直したり捨てたりする入力（正しくない UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、重複するメンバ）は、今の形として中継されていたものも含めて、拒否する（AC-09）。
    -   `input` の中身は、今と同じくホストは検査せず、サーバの生成の開始の検査に任せる。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   ジョブの API の時間の上限（LLM の呼び出し、Webhook への 1 通の送信 30 秒、中継の 20 秒）は変えない。
-   停止のときに待つ最長の時間は、投稿のジョブ 1 つ分（Slack への投稿は最大 10 通、1 通 30 秒、`internal/publisher/slack_message.go:17`・`internal/publisher/slack.go:23`）になる（AC-12）。

### 4.4. 互換性 (Compatibility)

-   拡張・ホスト・サーバは同じ版を使う前提を保つ（`README.md:562-564`）。
-   #503 より前の拡張（今の形のメッセージを送る）を新しいホストと使うと、送信はホストの `protocol_error` になる（AC-08・AC-09）。拡張を #503 以降に更新すれば解消する。
-   新しいホストを `/articles` を持つ古いサーバと使う場合、ホストは新しい形だけを送るので、古いサーバのジョブの API が受け付ける。古いホスト（今の形を `/articles` へ中継する）を新しいサーバと使う場合、今の形のメッセージは `not_found` になる。
-   `curl` などで `/articles` を直接呼んでいたクライアントは、`not_found` を受け取る（AC-01）。代わりにジョブの API を使う。

### 4.5. 保守性 (Maintainability)

-   CLAUDE.md の「Declare, don't infer」の例外が 1 つ減る（AC-16）。ホストは、宣言された `op` で送り先を選び、メッセージの中身で形を選ばない。
-   削除で使われなくなるコードを残さない。`make deadcode` が何も報告しないこと（CLAUDE.md「Code Quality」）。
-   `internal/server` が `internal/pipeline` と失敗の通知に依存しなくなる。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。同書の `/articles` と失敗の通知の記述は、本タスクで書き換える（AC-15）。
-   README の変更は、`README.md` を先に変え、`/mktrans README.md` で `README.en.md` に写し、同じコミットに入れる（CLAUDE.md「Translation Guidelines」）。
-   テストを削除するときは、`go tool cover -func` の関数ごとの結果が変わらないことを確かめる（CLAUDE.md「Testing Strategy」）。削除するコードだけを対象にするテストは、そのコードとともに削除する。
-   文書のガードは値と構造を確かめ、文言を確かめない（CLAUDE.md「Document tests」）。文書の記述の変更（AC-15・AC-16）は、PR のレビューで読んで確かめる。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。

-   **旧 API**: localhost サーバの `POST /articles`。検査に通ると、記事の生成から Webhook への投稿までを非同期に行うジョブを走らせる（`0011_localhost_server`）。本タスクで削除する。
-   **ジョブの API**: 生成の開始（`POST /jobs`）・状態の取得（`GET /jobs/{key}`）・投稿の操作（`POST /jobs/{key}/publish`）の 3 つの操作（`0044`）。
-   **今の形のメッセージ**: 版のメンバ `v` を持たない、ホストが `POST /articles` へ中継していたメッセージ（`0044` F-008）。本タスクの後は、受理する形ではない。
-   **新しい形のメッセージ**: 版のメンバ `v` を持ち、操作 `op` を宣言するメッセージ（`0044` F-008）。
-   **失敗の通知**: 旧 API の失敗を Webhook へ知らせるメッセージ（`internal/failure`、`SlackFailureNotifier`）。本タスクで削除する。
-   **生成の枠**: 同時に走る記事の生成の数の上限（`2`）。本タスクの後は、ジョブの API の生成のジョブだけが数える。
-   **投稿のジョブ**: 投稿の操作が始める、記事を Webhook へ送るジョブ。生成の枠を取らない。
