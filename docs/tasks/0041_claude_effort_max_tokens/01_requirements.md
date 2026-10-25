# 要件定義書：Claude の effort の既定を high にし、出力の上限 16,000 を緩和する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Claude アダプタの 2 つの既定を本番の構成に合わせて変えるための要件を定義する（[#171](https://github.com/isseis/yt2column/issues/171)）。1 つ目は effort の既定で、`YT2COLUMN_CLAUDE_EFFORT` を指定しなければ `high` を使う。2 つ目は出力の上限（`max_tokens`）の既定で、16,000 から引き上げる。

**背景:**

Claude アダプタは、`llm.GenerateRequest` の `MaxOutputTokens` が 0 のとき、`max_tokens` にアダプタの既定値 16,000 を送る（`internal/llm/claude/request.go:23-26`・`:70-75`）。生成、字数の調整、検証・推敲の 3 つの段階は、どれも `MaxOutputTokens` を 0 にして呼ぶ（`internal/writer/writer.go:463`・`internal/writer/shorten.go:28`・`internal/writer/review.go:256`）。そのため、3 つの段階とも上限は 16,000 である。16,000 という値は、タスク 0009 の設計で決めた。40 分前後の動画を effort `high`・`xhigh` で生成したときの出力の最大が 8,099 トークンだったので、その約 2 倍とした（`docs/tasks/0009_claude_llm_client/02_architecture.md` §1.4・§3.6）。この測定は、当時の既定のテンプレート（仮の文面）で行った。

テンプレート v5 以降では、この前提が成り立たない。effort `high` では、推論（考える部分）も出力の上限に数えられる。

| 評価 | モデル・effort・テンプレート | 観測 |
|---|---|---|
| 0008 評価記録 §5.7 | claude-sonnet-5-5・high・v5（0010 の生成後の段階なし） | roNZ-rLxhUE（32:29）は、本文を書き始める前に推論だけで 16,000 に達して打ち切られた。18tbwwPM91A（24:05）は本文の途中で打ち切られた |
| 0018 評価記録 §4.7.2（B3。本番の構成と同じ） | claude-opus-5-5・high・v5 | 10 件とも成功した。出力の最大は 18tbwwPM91A の 13,688 トークン（上限の約 86%）、所要時間は 2:15.45 だった |
| 0018 評価記録 §4.7.4（C04 の B4） | claude-opus-5-5・high・v7 | roNZ-rLxhUE の 3 回の試行のうち 1 回が 16,000 に届き、記事が出なかった（推定コスト 0.415428 USD）。成功した 1 回も 15,884 トークン（所要時間 2:30.56）だった |

打ち切られた実行にも料金がかかる。主経路（localhost サーバ）では、記事が投稿されず、失敗の通知だけが届く。

effort には既定がない。生成または検証・推敲のプロバイダが `claude` のとき、`YT2COLUMN_CLAUDE_EFFORT` を指定しないと、設定の読み込みがエラーになる（`internal/config/config.go:532-538`・`:586-601`）。[#490](https://github.com/isseis/yt2column/issues/490) は、既定の LLM を claude-opus-5-5 にする。本番の構成（claude-opus-5-5・effort `high`。0008 評価記録 §6.1）を既定にすると、何も指定しない利用者も、長い動画でこの上限に当たりうる。そこで本タスクで、effort の既定を `high` にし、出力の上限を引き上げる。issue は、どちらもリリースの前に行うとしている。

対応 issue: [#171](https://github.com/isseis/yt2column/issues/171)（関連: [#490](https://github.com/isseis/yt2column/issues/490)・[#488](https://github.com/isseis/yt2column/issues/488)）

**本書の記述範囲:** 本書が定めるのは次の 3 つである。effort の既定とそれを当てる条件、出力の上限の既定を引き上げることとその新しい値が満たす条件、文書に書くことである。出力の上限の新しい値は、本書では決めない。`02_architecture.md` が、本書の条件（F-002）に従って既存の測定から決める。設定の既定値をどこに置くか、テストの手段も設計で決める。要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #171 で決めたこと:**

-   **effort の既定は `high`:** effort の既定値 `high` をプログラムに持たせ、`YT2COLUMN_CLAUDE_EFFORT` で上書きできるようにする。何も指定しない利用者が、本番の構成の effort で記事を作れるようにするためである。
-   **出力の上限は緩和する:** 本番の構成で、40 分前後の動画を打ち切られずに生成できるよう、上限を引き上げる方向で見直す。v5 以降のテンプレートでは、16,000 の根拠（0009 の測定）が成り立たないためである。
-   **#491 の統合:** #491 は本 issue に統合された。

**要件定義の際に決めたこと（2026-10-25）:**

-   **上限の値は設計で決める:** 本書は、新しい値が満たす条件（F-002）だけを定め、値そのものは `02_architecture.md` で決める。値は、測定値と、タイムアウトに収まるかの両方から決まる。その確認は、0009 と同じく設計で行うのが適切なためである。
-   **値の根拠は既存の測定だけにする:** 実 API での追加の測定は行わない。新しい値は、0009 §1.4、0008 評価記録 §5.7、0018 評価記録 §4.7.2・§4.7.4 の測定から決める。追加の測定には料金がかかる。また、0018 の 2 つの基準の集合（本番の構成の 10 件と、テンプレートだけを変えた 10 件）で、上限に近づく動画が分かっている。そのため、レビュアーは既存の測定で足りると判断した。この結果、テンプレート v5 で 40 分前後の動画の出力は測っていない。0018 の測定でいちばん長い動画は roNZ-rLxhUE（32:29）である。出力のトークン数は動画の長さの順に並ばない（24 分の 18tbwwPM91A の出力が、32 分の roNZ-rLxhUE より多い。0018 評価記録 §4.7.2）。そのため、F-002 の余裕は動画の長さからではなく、観測した出力の最大から取る。
-   **非ストリーミングと 15 分のタイムアウトは変えない:** 新しい値は、LLM の 1 回の呼び出しのタイムアウト `provider.LLMTimeout`（15 分。`internal/llm/provider/provider.go:16-20`）に収まる範囲で決める。ストリーミングは、0009 要件定義書 §2.3 と同じくスコープ外のままにする。0018 の測定から計算した出力の速さは毎秒 100 トークン前後で（13,688 トークンで 2:15、15,884 トークンで 2:30）、16,000 の数倍の値でも 15 分に収まる見込みがある。ストリーミングを入れると応答の検証（0009 要件定義書 3.2）を作り直すことになり、規模が大きい。タイムアウトを変えないので、localhost サーバの停止で待つ時間の上限（約 45 分。`cmd/yt2column-server/run.go:39-42`・`:232-235`、README「Stopping the server」）も変わらない。
-   **上限は 3 つの段階で共通にする:** 生成、字数の調整、検証・推敲の 3 つの段階に同じ上限を当てる。字数の調整と検証・推敲の出力は小さく、打ち切りは起きていない（0010 評価記録 §3.7）。それでも、段階ごとに分ける理由はない。生成だけを変えるには、生成からプロバイダに依存しない `MaxOutputTokens` を渡す方法がある。しかし、その方法では DeepSeek にも `max_tokens` が送られるようになり（`internal/llm/deepseek/request.go:36-52`）、DeepSeek の振る舞いが変わる。
-   **新しい値を受け付けることを求めるモデルは Opus 5.5 と Sonnet 5.5:** 新しい値は、claude-opus-5-5 と claude-sonnet-5-5 が受け付ける値とする（F-002）。この 2 つは評価で使ったモデルで、0009 §1.4 では、どちらも `max_tokens` 64,000 を受け付けた。claude-haiku-5-5 など、ほかのモデルは要件の対象にしない。ただし、設計で公開文書の出力の上限を確かめる。新しい値がそのモデルの上限を超える場合は、README で案内する（[design_handoff.md](design_handoff.md) H-02）。
-   **effort の既定は、claude を使う段階すべてに当てる:** 生成または検証・推敲のプロバイダが `claude` で、今は `YT2COLUMN_CLAUDE_EFFORT` の未設定がエラーになる条件のとき、未設定なら `high` を使う。effort は今も 3 つの段階で 1 つの値を共有している（`internal/llm/provider/provider.go:67-78`）。段階ごとに分ける理由はないので、共有は変えない。空の値は、今までどおり拒否する（`docs/dev/project_overview.md`「設定（環境変数）」の、空の値を未設定とせずにエラーとする方針）。
-   **使った effort を出力に出さない:** 0009 は、「どの effort で生成したかを、設定から常に読み取れるようにする」ために、effort に既定値を持たせなかった（0009 要件定義書 F-006）。本タスクの既定はこの判断を変える。それでも、effort を出力（標準エラーの要約、`--out` のファイルのフロントマター、Slack の投稿）に加えることはしない。既定値を README と `project_overview.md` に書くので、未設定なら `high` だと文書から分かるためである。出力の形も変えずに済む。
-   **README の effort の案内は、既定を書き、`max` の注意を残す:** README「Configuration」の表に、未設定のときは `high` であることを書く。表の後の `max` の注意（`README.md:750-752`）は残す。その文が出力の上限について書く内容は、新しい値に合わせる（F-003）。
-   **測り直しの手順は開発者ガイドに書く:** テンプレートや effort を変えたときに出力の上限の根拠を測り直す手順は、`docs/dev/developer_guide/` に英語の開発者ガイドとして新設し、`CLAUDE.md` の Quick Links からリンクする（F-003）。単価の表の更新手順（`docs/dev/developer_guide/pricing_update.md`）と同じ形である。タスクの文書は経緯の記録なので、繰り返し行う作業の手順を置く場所に向かない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   プロバイダが `claude` のとき、`YT2COLUMN_CLAUDE_EFFORT` を指定しなくても、本番の構成の effort（`high`）で記事を作れるようにする。
-   本番の構成（claude-opus-5-5・effort `high`）で、既存の測定で上限に近づいた動画を、出力の上限で打ち切られずに生成できるようにする。
-   LLM の 1 回の呼び出しのタイムアウトと、非ストリーミングの通信を変えない。
-   DeepSeek の振る舞いを変えない。
-   出力の上限の根拠を、テンプレートや effort を変えたときに測り直せるようにする。

### 2.2. スコープ (In Scope)

-   `YT2COLUMN_CLAUDE_EFFORT` が未設定のときの effort の既定（`high`）（F-001）。
-   Claude アダプタの出力の上限の既定値の引き上げと、新しい値が満たす条件（F-002）。
-   文書: README「Configuration」の表と `max` の注意、`docs/dev/project_overview.md`「設定（環境変数）」の表、測り直しの手順の開発者ガイドの新設と `CLAUDE.md` からのリンク（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   既定のプロバイダとモデルの変更（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`）。#490 で扱う。
-   本番の構成と品質をまとめた共通の文書、README の推奨構成。#488（タスク 0040）で扱う。
-   ストリーミング応答の導入。理由は「要件定義の際に決めたこと」の非ストリーミングの項にある。
-   LLM のタイムアウト（`provider.LLMTimeout`）の値の変更。
-   段階ごとに異なる出力の上限、または段階ごとに異なる effort。
-   出力の上限を利用者が設定で変える手段（環境変数など）。issue は既定の見直しだけを求めている。
-   使った effort の出力への表示。
-   実 API での追加の測定。
-   claude-opus-5-5・claude-sonnet-5-5 以外のモデルが新しい値を受け付けることの保証。
-   DeepSeek アダプタの変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: effort の既定

設定の読み込みで、`YT2COLUMN_CLAUDE_EFFORT` が未設定なら effort を `high` とする。この既定を当てるのは、今は未設定がエラーになる条件だけである。その条件は、実行が必要とする段階（生成、検証・推敲）のうち、プロバイダが `claude` の段階があることで、`--after` による再開で実行しない段階は数えない。設定した値の受理と拒否の規則は変えない。

**Acceptance Criteria**:
- **AC-01**: `YT2COLUMN_LLM_PROVIDER=claude`、`YT2COLUMN_MODEL`、`ANTHROPIC_API_KEY` を設定し、`YT2COLUMN_CLAUDE_EFFORT` を設定しない環境から読み込んだ設定は、エラーにならず、effort が `high` である。その設定から構築した生成のアダプタと検証・推敲のアダプタが送るリクエストの `output_config.effort` は、どちらも `high` である。
- **AC-02**: 生成と検証・推敲の一方だけのプロバイダが `claude` である場合（`YT2COLUMN_LLM_PROVIDER=deepseek` と `YT2COLUMN_REVIEW_PROVIDER=claude` の組み合わせ、`YT2COLUMN_LLM_PROVIDER=claude` と `YT2COLUMN_REVIEW_PROVIDER=deepseek` の組み合わせ）、`YT2COLUMN_CLAUDE_EFFORT` を設定しなければ、`claude` の段階のアダプタが送るリクエストの `output_config.effort` は `high` である。
- **AC-03**: プロバイダが `claude` の段階があり、`YT2COLUMN_CLAUDE_EFFORT` が `low`・`medium`・`high`・`xhigh`・`max` のいずれかである場合、構築したアダプタが送るリクエストの `output_config.effort` はその値である。5 つの値のそれぞれについて確かめる。
- **AC-04**: プロバイダが `claude` の段階があり、`YT2COLUMN_CLAUDE_EFFORT` が空の値である場合、読み込みは `errors.Is(err, config.ErrMissing)` が真になるエラーになり、エラーから変数名を取り出せる。5 つの値以外（例: `High`、` high`、`high\n`、`none`、`min`）の場合は、`errors.Is(err, config.ErrInvalid)` が真になるエラーになる。どちらのエラーのメッセージにも、拒否した値（テストで埋め込んだ目印）は現れない。
- **AC-05**: 実行が必要とする段階のどれもプロバイダが `claude` でない場合、`YT2COLUMN_CLAUDE_EFFORT` が未設定・空・5 つの値以外のどれであっても、読み込みの結果（成功か、どの変数のエラーか）は、この変数が未設定のときと同じである。`--after` で再開し、プロバイダが `claude` の段階を実行しない場合も同じである。
- **AC-06**: プロバイダが `claude` の段階があり、`ANTHROPIC_API_KEY` が未設定の場合は、`YT2COLUMN_CLAUDE_EFFORT` が未設定でも、読み込みは今までどおり `errors.Is(err, config.ErrMissing)` が真になるエラーになる。エラーから `ANTHROPIC_API_KEY` を取り出せる。effort の既定は、ほかの変数を任意にしない。

#### F-002: 出力の上限の既定の引き上げ

`MaxOutputTokens` が 0 のときに Claude アダプタが送る `max_tokens` を、16,000 より大きい値 V に変える。V は `02_architecture.md` が次の条件に従って決め、根拠とともに記録する。3 つの段階（生成、字数の調整、検証・推敲）は、どれも V を使う。

-   **(a) 観測した出力の最大を上回る:** V は、本番の構成（claude-opus-5-5・effort `high`）で観測した出力のトークン数の最大を、余裕を持って上回る。最大は、0018 評価記録 §4.7.4 の 15,884 トークン（成功した実行）と、16,000 に届いて打ち切られた実行である。余裕の大きさとその理由は設計で記録する。
-   **(b) タイムアウトに収まる:** V トークンを出力するのにかかる時間の見積もりは、`provider.LLMTimeout`（15 分）に、余裕を持って収まる。見積もりには、既存の測定（0009 §1.4、0018 評価記録 §4.7.2・§4.7.4）で観測した出力の速さのうち遅いほうを使う。
-   **(c) モデルが受け付ける:** V は、claude-opus-5-5 と claude-sonnet-5-5 が `max_tokens` として受け付ける値である。公開文書、または既存の観測（0009 §1.4 では、どちらも 64,000 を受け付けた）で確かめる。

**Acceptance Criteria**:
- **AC-07**: Claude アダプタに `MaxOutputTokens` 0 の `Generate` を呼ぶと、送るリクエスト本文の `max_tokens` は V である。V は 16,000 より大きい。
- **AC-08**: プロバイダが `claude` のとき、1 回の実行で生成、字数の調整、検証・推敲の 3 つの段階のアダプタが送るリクエスト本文の `max_tokens` は、どれも V である。
- **AC-09**: Claude アダプタに正の `MaxOutputTokens` の `Generate` を呼ぶと、送るリクエスト本文の `max_tokens` は、今までどおりその値である。
- **AC-10**: Claude の応答が `stop_reason` `max_tokens` で打ち切られた場合、エラーは今までどおり `errors.Is(err, llm.ErrTruncated)` が真になり、メッセージが示す `max_tokens` は、そのリクエストが送った値（`MaxOutputTokens` が 0 なら V）である。`text` ブロックがないまま打ち切られた場合のメッセージは、今までどおり effort を下げるよう案内する（`internal/llm/claude/response.go:296-306`）。
- **AC-11**: プロバイダが `deepseek` のとき、3 つの段階のアダプタが送るリクエスト本文は本タスクの前と変わらない。`MaxOutputTokens` が 0 のリクエストは `max_tokens` を含まない。
- **AC-12**: LLM の 1 回の呼び出しのタイムアウトは 15 分のままである。CLI がタイムアウトで出すメッセージの時間と、localhost サーバの停止の通知が示す待ち時間の上限（約 45 分）は、本タスクの前と変わらない。
- **AC-13**: `02_architecture.md` は、V の値と、V が (a)・(b)・(c) を満たす根拠を記録している。根拠には、(a) の余裕の大きさ、(b) の見積もりの時間とそれに使った出力の速さ、(c) を確かめた出典を含む。

#### F-003: 文書

effort の既定と出力の上限の変更を、利用者向けの文書と開発者向けの文書に反映する。出力の上限の根拠を測り直す手順を、開発者ガイドとして新設する。

**Acceptance Criteria**:
- **AC-14**: README「Configuration」の表の `YT2COLUMN_CLAUDE_EFFORT` の行の「When unset」は、未設定なら `high` を使うことを書いている。生成または検証・推敲のプロバイダが `claude` でなければこの変数を読まないこと、空の値はエラーになることは、表とその後の説明から読み取れる。
- **AC-15**: README「Configuration」の `max` の注意（`README.md:750-752`）は残っている。その文が出力の上限について書く内容は、V と矛盾しない。V で `max` が記事を生成できるかは、0009 §1.4 の観測（64,000 でも推論だけで上限に達した）と設計の判断に従う。
- **AC-16**: `docs/dev/project_overview.md`「設定（環境変数）」の表の `YT2COLUMN_CLAUDE_EFFORT` の行の「未設定のとき」は、生成または検証・推敲のプロバイダが `claude` なら `high` を使うことを書いている。
- **AC-17**: `docs/dev/developer_guide/` に、英語の開発者ガイドが新設されている。ガイドは、Claude アダプタの出力の上限の根拠を測り直す手順として、次の 4 つを書いている。測り直す契機（本番の構成の生成のテンプレート、effort、モデルのいずれかを変えたとき）。集める値（段階ごとの出力のトークン数。README「Token usage」の `tokens:` の行の `output=`。ほかに、所要時間と、打ち切られた実行の有無）。判定の方法（F-002 の (a)・(b)・(c) と照らす）。条件を満たさないときに更新するもの（V、V の根拠の記録、README の `max` の注意）。
- **AC-18**: `CLAUDE.md` の Quick Links の「Development Guides」に、AC-17 のガイドへのリンクと 1 行の説明がある。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

本タスクは、信頼できない入力の境界を新設も変更もしない。環境変数 `YT2COLUMN_CLAUDE_EFFORT` の受理と拒否の規則は変えない（F-001 の AC-03・AC-04）。Claude の応答の検証（0009 要件定義書 3.2）も変えない。V に達した応答の打ち切りは、今までどおり `llm.ErrTruncated` になる（AC-10）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   LLM の 1 回の呼び出しは、今までどおり 15 分以内に終わるか、タイムアウトで失敗する（AC-12）。V の大きさは、この時間に収まるように決める（F-002 (b)）。
-   出力が V に近い応答では、非ストリーミングで応答を待つ時間（生成が終わるまで何も届かない時間）が、16,000 のときより延びる。0009 §1.4 では約 9 分の無通信でも接続は切れなかったが、利用者のネットワークによっては途中で切れうる。このリスクは 0009 の設計（`02_architecture.md` §3.6）と同じく受け入れる（[design_handoff.md](design_handoff.md) H-03）。

### 4.2. セキュリティ (Security)

本タスクは、LLM API へデータを送る経路の設定を変えるので、条件付きのガイド（[security.md](../../dev/security.md)）の対象である。ただし、次のとおり、ガイドが扱う点を変えない。

-   送る相手と送るデータ（security.md §4）は変えない。変えるのは、リクエスト本文の `max_tokens` と、既定のときの `output_config.effort` の値だけである。
-   秘密情報（security.md §2）の扱いは変えない。`YT2COLUMN_CLAUDE_EFFORT` の拒否のエラーは、今までどおり変数名と固定の理由だけを持ち、値を含めない（AC-04）。
-   応答の本文の読み取りの上限（security.md §3。`internal/llm/llmhttp/llmhttp.go:19` の 8 MiB）は変えない。V で返りうる応答の本文の大きさがこの上限に収まるかは、設計で確かめる（[design_handoff.md](design_handoff.md) H-04）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   本番の構成の生成が、既存の測定で観測した出力の量で打ち切られない（F-002 (a)）。
-   打ち切られた実行の料金は、上限が大きいほど高くなりうる。例えば effort `max` を選ぶと、上限まで推論に使って打ち切られうる（0009 §1.4）。V で打ち切られた 1 回の実行の推定コストを、設計で記録する（[design_handoff.md](design_handoff.md) H-05）。

### 4.4. 互換性 (Compatibility)

-   `YT2COLUMN_CLAUDE_EFFORT` を設定している利用者の振る舞いは変わらない（AC-03）。設定していない利用者は、今まではエラーで止まっていたが、本タスクの後は `high` で記事を作る（AC-01・AC-02）。
-   DeepSeek を使う利用者の振る舞いは変わらない（AC-05・AC-11）。
-   正の `MaxOutputTokens` を渡す呼び出し元（統合テスト）の振る舞いは変わらない（AC-09）。
-   claude-opus-5-5・claude-sonnet-5-5 以外のモデルで V が受け付けられない場合は、README で案内する（[design_handoff.md](design_handoff.md) H-02）。

### 4.5. 保守性 (Maintainability)

-   出力の上限の根拠を、テンプレートや effort を変えたときに開発者ガイドの手順で測り直せる（AC-17・AC-18）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   0009 の判断の変更: 0009 要件定義書 F-006 は、`YT2COLUMN_CLAUDE_EFFORT` について、「プロバイダが `claude` のとき必須とし、既定値を持たない。どの effort で生成したかを、設定から常に読み取れるようにするためである」と定めた。本タスクは、未設定なら `high` を使うことで、この判断を変える。理由は、issue #171 の決定（何も指定しない利用者が本番の構成の effort で記事を作れるようにする）である。既定値を README と `project_overview.md` に書くので、未設定のときに使う effort は文書から読み取れる。0009 の要件定義書の AC-24 のうち、`YT2COLUMN_CLAUDE_EFFORT` の未設定を `config.ErrMissing` とする部分は、本タスクの AC-01・AC-02 に置き換わる。空の値を `config.ErrMissing` とする部分は、AC-04 として残る。
-   0009 の判断の変更: 0009 の設計（`02_architecture.md` §3.6）は、「Anthropic は非ストリーミングの `max_tokens` の目安を約 16,000 としており、それを超えない」ことを 16,000 の根拠の 1 つにした。本タスクはこの目安を超える値を許す。理由は、v5 以降のテンプレートと effort `high` では 16,000 で打ち切られるためである（1. の背景）。この目安を超えることで生じるリスク（無通信の時間が延びる）は、4.1 で受け入れる。目安の現在の内容は、設計で確かめる（[design_handoff.md](design_handoff.md) H-03）。
-   `docs/dev/project_overview.md`「前提・制約」の「モデル名は頻繁に更新される。コードにハードコードせず設定で与える」は、モデル名についての方針である。effort の既定と出力の上限は、モデル名ではないので、この方針の例外にあたらない。
-   #490 との順序: 本タスクは #490 に依存しない。#490 の要件定義書（未作成）は、#490 と本タスクをどちらの順で入れるかを決める。#490 の既定が先に入ると、何も指定しない利用者が長い動画で打ち切りに当たりうる（#490 の issue 本文）。
-   タスク 0040（#488）の文書は、本タスクの既定の変更を前提に書かれ、本タスクの後に `main` に入る（0040 要件定義書 §5。PR #489 の時点）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **effort**: Claude の Messages API の `output_config.effort`。推論にかける量の指定で、`low`・`medium`・`high`・`xhigh`・`max` のいずれか。
-   **出力の上限**: Messages API の `max_tokens`。推論（`thinking` ブロック）を含む出力のトークン数の上限。上限に達した応答は打ち切られる（`stop_reason` が `max_tokens`）。
-   **V**: 本書で、`MaxOutputTokens` が 0 のときに Claude アダプタが送る新しい `max_tokens` の値を指す記号。値は `02_architecture.md` で決める。
-   **本番の構成**: 生成、字数の調整、検証・推敲の 3 つの段階を、すべて claude-opus-5-5（effort `high`）で行い、生成のテンプレートを v5 とする構成（0008 評価記録 §6.1）。
-   **打ち切り**: LLM の応答が出力の上限に達して終わること。アダプタは `llm.ErrTruncated` として報告し、記事は出ない。
