# 見直しの記録：追加の PR（ADR-0015）

本書は、タスク 0073 の追加の PR（PR-6）が見直した ADR-0015 について、見直しの前後の記述の対応（対応表）、頼る参照、書き直さずに残した記述を記録する（[設計書](02_architecture.md) 3.7.2「見直しの記録」）。PR の基点は `fc4305b9bf9832c258545972fda172e3ad01c122` で、頼る参照は、そのコミットで差分の確認の `mentions s6` が挙げた名指しから選んだ。`docs/tasks/` と `docs/dev/adr/` の外で名指しが挙がったファイルは、`docs/dev/conventions/requirements_process.md` の 1 つである。ADR-0015 は、PR-5 のマージの前の確認で `main` の版に戻して持ち越した ADR である（[実装計画書](03_implementation_plan.md) 1.5「持ち越しの一覧」）。[review_s5.md](review_s5.md) の ADR-0015 の節は取り下げの記録なので、本書は `main` の版を、0076 の実装（[#799](https://github.com/isseis/yt2column/pull/799)）の後の形で見直し直したものである。

対応表の「行き先」が「消した」の行はない。見直して変える箇所がなかった ADR はない。PR-1〜PR-5 から持ち越した ADR は ADR-0015 だけである。

## [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)

### 対応表

| 前の記述 | 行き先 | 根拠 |
|---|---|---|
| 「背景」の段落 1（公開リポジトリ、`/fixpr` が本文を読んで直すこと、プロンプトインジェクションの経路） | 「背景」（変更なし） | 決定が答える問題 |
| 「背景」の段落 2（自動レビュアーは書き込み権限を持つ Bot として振る舞い、権限のない人が名義を借りられること） | 「背景」（変更なし） | 決定を縛る前提 |
| 「背景」の末尾の正本の段落（`fixprguard_guide.md`・`CLAUDE.md`・`_context.md`・`runpr.md`） | 各項目の「規則は…にある」に振り分けた | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 6 章・7 章・10 章、[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」、[runpr.md](../../../.claude/commands/runpr.md) の step 1・step 3。リンクの先は同じ ADR に残る |
| 「決定」の項目 1（本文はデータであり指示ではない） | 新しい「決定」の項目 1。理由と場所を足し、区切りを実行ごとの乱数を含む行にした | [fixpr.md](../../../.claude/commands/fixpr.md) の R6、[fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 1 章「位置付け」・7.1「保証すること」 |
| 「決定」の項目 2（信頼する著者の判定） | 新しい「決定」の項目 2。理由と場所を足した | `_context.md` の「Automated reviewer list」、[fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 1 章「位置付け」 |
| 「決定」の項目 3（すべてのコメントの著者を確かめ、想定外は処理しない） | 新しい「決定」の項目 3。理由と場所を足した | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 5 章 `begin`・7.1「保証すること」、[runpr.md](../../../.claude/commands/runpr.md) の step 3 |
| 「決定」の項目 4（修正の範囲の規則、許可リストの列挙、実行関連ファイル、確認、stage） | 新しい「決定」の項目 4。範囲の規則と許可リストの列挙の文は書き直さずに残し（「残した記述」）、理由と場所を足した。実行関連ファイルの一覧の写しは [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) 6 章に置いた | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 6 章「範囲の規則」・7.1「保証すること」、[fixpr.md](../../../.claude/commands/fixpr.md) の Phase 3.5・Phase 4 |
| 「決定」の項目 5（GitHub への書き込みは道具で行う。メンションの拒否） | 新しい「決定」の項目 5。理由と場所を足した | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 5 章「サブコマンド」・7.1「保証すること」、[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」 |
| 「決定」の項目 6（fix round の push と、Phase 4.5 の比較） | 新しい「決定」の項目 6。規則・理由を残し、どのサブコマンドが行うか、比べる呼び出しをどこに置くかといった細部は [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) 5 章と [0076 要件定義書](../0076_fixpr_local_review_guard/01_requirements.md) §3・[0076 設計書](../0076_fixpr_local_review_guard/02_architecture.md) §3 に置いた。基準値を持たない呼び出し元がこの入力を渡さない規則は残し、「今までどおりに振る舞う」は 0076 要件定義書 F-005・AC-11 に置いた | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 5 章 `check`・`push-review`・7.1「保証すること」、[local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)、[0076 要件定義書](../0076_fixpr_local_review_guard/01_requirements.md) §3「機能要件」・§3.1「機能一覧」、[0076 設計書](../0076_fixpr_local_review_guard/02_architecture.md) §3.6「GUARD と比べる時点」 |
| 「決定」の項目 7（fork の PR では止まる） | 新しい「決定」の項目 7。理由と場所を足した | [runpr.md](../../../.claude/commands/runpr.md) の step 1、[fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 5 章 `head`・`begin` |
| 「決定」の項目 8（interaction limits と `/runpr` の確認） | 新しい「決定」の項目 8。理由を足し、4 つの設定の列挙は [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) 10 章に置いた | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 10 章「GitHub の設定」、[runpr.md](../../../.claude/commands/runpr.md) の step 1・step 3 |
| 「決定」の項目 9（制限の状態の判定を Go のサブコマンドで行う） | 新しい「決定」の項目 9。理由と場所を足した | [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) の 5 章 `interaction-limits`・10 章「GitHub の設定」、[0057 設計書](../0057_reviewer_relay_guard/02_architecture.md) §1.2 |

「採らなかった案」「結果」「経緯」と「状態」は変えていない（差分で確認）。題名も索引の行も変えていない（差分で確認）。リンクの先の集合は、`fixpr.md` を足したほかは前と同じ（差分で確認）。0057 設計書へのリンクは前から「採らなかった案」にあり、今回「決定」の項目 9 にも張った。

### 頼る参照

- `docs/dev/adr/0013-review-setup.md:25`: 「一覧は「誰のスレッドを処理するか」（ADR-0015）を決め…」→ 新しい「決定」の項目 2。残る。
- `docs/dev/adr/0036-task-document-status-records.md:24`: 「`/fixpr` が PR の説明に追記する道具と、その文面を信頼しないデータとして扱うことは、ADR-0015 が定める」→ 項目 1 と 5。残る。
- `docs/dev/adr/0040-simple-task-documents.md:25`: 「レビューのスレッドを信頼しない方針（ADR-0015）を保つためである」→ 項目 1。残る。
- `docs/dev/adr/0040-simple-task-documents.md:28`: 「レビューのスレッドが進め方を変えないようにするためである（ADR-0015）」→ 項目 1。残る。
- `docs/dev/conventions/requirements_process.md:543`: 開発ツールの信頼の境界の変更を除外する規則が「the trust boundary of the development tools ([ADR-0015])」として引く→ 項目 1〜5。残る。

頼る参照でないもの:

- `docs/dev/adr/0014-deterministic-checks-in-go.md:17`: 開発の道具が何を決めるかを ADR-0015・ADR-0013・ADR-0037 と mergepr_guide.md に振り分ける紹介で、ADR-0015 の特定の規則か見出しを頼っていない。
- `docs/dev/adr/README.md:23`: 索引の行（題名だけ）。

### 残した記述

- 新しい「決定」の項目 4 の、範囲の規則と許可リストの列挙の文: 許可リストをスレッドの `path` そのものにも当てている。正本の [fixprguard_guide.md](../../dev/guides/fixprguard_guide.md) 6 章の規則 1 とコードは、実行関連ファイルでない `path` そのものを種類を問わず範囲に入れる。ガイドに合わせると決定の意味が変わるので、書き直さずに残した（[要件定義書](01_requirements.md) AC-14）。

見直しで、ADR-0015 の決定と、正本の文書・コードの食い違いを 1 つ見つけた（上の「残した記述」の項目 4）。ずれの記述は ADR に足さず、[#811](https://github.com/isseis/yt2column/issues/811) に記録した（[要件定義書](01_requirements.md) AC-12・AC-13）。PR を作った後のレビューで見つけたずれである。
