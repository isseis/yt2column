# 要件定義書：実装モデル要件を Opus・Sonnet の選択に直結させ、Opus のレビューを一度に行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-10 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、計画書の PR マーカーに書く実装モデル要件を、`/runplan` を Opus と Sonnet のどちらで実行するかと、Opus のレビューを足すかどうかに直結する値にするための要件を定義する。あわせて、`/weakreview` を、一つのコマンドで Opus のレビューを二つ行う形に改める。

対応 issue: [#760](https://github.com/isseis/yt2column/issues/760)
関連 issue: [#790](https://github.com/isseis/yt2column/issues/790)（OpenCode と DeepSeek-V4.1-Flash で開発できるかを PoC で確かめる。本タスクの範囲外とした）

本書が決めるのは、値とその意味、値を書く場所と読む場所、Opus のレビューを実行する時点と直す主体、`/weakreview` の振る舞い、旧い値の扱いである。どのコマンドのどの手順に何を書くか、二つのレビューの指示をどこに置くか、指摘をどう統合するかは、`02_architecture.md` に委ねる（`design_handoff.md`）。

**背景:**

`/mkplan2` は、PR マーカーの `**実装モデル要件**` に `frontier-required`・`frontier-recommended`・`standard` のどれかを書く（`.claude/commands/mkplan2.md:41-44`、`:66`、`:84`）。`/mkplan` は step 8.5 でこれを実行する。この値には、次の問題がある。

-   値がモデル名ではない。`runplan.md` は「frontier モデルで実行する」「frontier モデルが望ましい」「安いモデルで実行し、`weakreview.md` で確かめる」としか書かない（`.claude/commands/runplan.md:15-20`）。`runpr.md` は値をそのまま見せるだけである（`.claude/commands/runpr.md:713-722`、`:1018-1019`）。Opus と Sonnet のどちらに当たるかは、どこにも書かれていない。
-   中間の `frontier-recommended` は「frontier モデルが望ましい」としか言わない。Sonnet で進めてよいのか、よいならどんな条件なのかが決まらない。既存の計画書に 89 件ある。
-   `**判定理由**` は、どの契機に当たったかを書く形である（`mkplan2.md:85`）。Sonnet に任せると何が起きやすいかは分からない。
-   `standard` の後で確かめる `weakreview.md` は、どのコマンドからも自動では呼ばれない。また、`code-review` が別に走ることを前提にし（`.claude/commands/weakreview.md:14-29`）、`--fix` がなければ報告だけで終わる（同 128-132 行目）。`/runplan` の中のサブエージェントのレビューは、セッションのモデルを引き継ぐ（`.claude/commands/_lib/review-subagent-pattern.md:89-99`）。そのため Sonnet のセッションでは、Opus のレビューは誰も実行しない。
-   PR マーカーのない単一フェーズの計画書には、値がない。`runpr.md` は「値なし」と表示する（`runpr.md:713-722`）。

**費用の目安（要件定義の際に手元で集計した値）:** Claude Code の記録にある過去の `/runplan` のセッション 123 件のトークン数を、Opus 5.5 の単価（入力 $4、出力 $20、キャッシュ読み $0.20。100 万トークンあたり）で換算した。1 セッションの中央値は約 $13（四分位は $7.5〜$19.2）で、費用の大半はキャッシュ読みだった。レビューのサブエージェントは 1 回で中央値約 $0.5、上位四分位で約 $1.2 だった。Sonnet 5.5 の単価は、入力・出力・キャッシュ読みのいずれも Opus 5.5 の半分である。Sonnet で同じ量のトークンを使うとすれば、Sonnet で実装して Opus のレビュー（約 $2〜4）を足しても、中央値の PR で約 3 割安くなる。ただし、Sonnet が手戻りなどで Opus の約 1.5 倍を超えるトークンを使えば、安くはならない。この倍率は測っていない。

**issue #760 で決めたこと:**

-   **実装モデル要件を、とるべき行動で表す 3 つの値にする。** Opus で実装する値、Sonnet で実装して Opus のレビューを足す値、Sonnet で実装する値の 3 つである。理由: 値から、セッションのモデルと、追加のレビューの要否が直接分かるようにするため。
-   **当てはめる契機は、今の区分の契機を引き継ぐ。** Opus で実装する値は今の `frontier-required` の契機、Sonnet＋Opus のレビューは `frontier-recommended` の契機、Sonnet は `standard` に対応する。理由: 契機は今の設計の判断の材料として使われており、値の意味だけを変えれば足りるため。
-   **`**判定理由**` の書き方を変える。** Opus と Sonnet＋Opus のレビューの値には「Sonnet に任せると何が起きやすいか」を、Sonnet の値には「当たる契機がない」ことを書く。理由: モデルを選ぶ人が、Sonnet にしたときの危険を読めるようにするため。
-   **Sonnet の値では、追加のレビューをしない。** `/runplan` の中のレビュー、ローカルの CLI レビュー、Codex の PR レビューで足りるとする。
-   **`/weakreview` が、二つのレビューを Opus で一度に行う。** 一つは一般的な正しさのレビュー（`/code-review` 相当）、もう一つは今の `/weakreview` 相当のレビュー（能力の低いモデルが `runplan.md` の字面だけを満たして判断を落とす失敗）である。二つの指摘を統合し、重複を除いて、トリアージする。実行するセッションのモデルに関係なく、レビューは Opus で走る。
-   **完了した計画書は書き換えない。**

**要件定義の際に決めたこと（2026-11-10）:**

-   **プロセス: 完全プロセスにする。** 変更は、`mkplan2.md`・`mkplan.md`・`runplan.md`・`runpr.md`・`weakreview.md`・`_context.md` の 6 つのコマンドと `CLAUDE.md` に及ぶ。未決の判断も多い。さらに、Opus のサブエージェントを二つ走らせて指摘を統合するという、新しい仕組みを作る。そのため、簡易プロセスは推奨しなかった。除外の判断: 除外に当たらない。`docs/dev/security.md` の規則と前提、開発用ツールの信頼境界（`fixprguard`・`codexreview`・`mergepr`、レビューのスレッドの扱い）、外部の依存は、どれも変えないからである（`docs/dev/conventions/requirements_process.md`、「Simple Process」の「Excluded changes」）。利用者は完全プロセスを選んだ。ADR: なし（プロセスの選択はタスクの中で閉じる）。
-   **3 段階を残す。** 段階の数を減らす案と比べたうえで、3 段階を採る。大きくて誤りの入りやすい PR（今の `frontier-recommended`）は、Sonnet にしたときの節約の額が最も大きい層である。誤りがレビューで見つけて直せる種類なら、Sonnet で実装して Opus のレビューを足す形が効く（上の「費用の目安」）。この比較は、Sonnet が使うトークンが Opus の約 1.5 倍を超えない、という測っていない前提に立つ。ADR: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を作る（下の ADR の項）。
    -   採らない案 1: `opus` と `sonnet` の 2 段階（今の `frontier-recommended` を `opus` に寄せる）。仕組みは最も単純だが、節約が最も大きい層で節約を手放す。
    -   採らない案 2: `opus` と `sonnet+opus-review` の 2 段階（Sonnet で実装するときは必ず Opus がレビューする）。小さな PR では、レビューの費用が実装の費用と同じくらいになる。
-   **値の名前は `opus`・`sonnet+opus-review`・`sonnet` にする。** モデルのファミリー名だけを使い、版を含めない。理由: モデルの版が上がっても、値と計画書を書き換えずに済むため。ADR: ADR-0042。
-   **DeepSeek-V4.1-Flash を実装モデルの候補にすることは、本タスクの範囲外にする。** OpenCode と DeepSeek-V4.1-Flash で開発できるかは、PoC から始める別の issue（#790）で扱う。理由: 開発の道具に外部のサービスを加えることになり、除外される変更に当たる。コードを DeepSeek のサービスへ送ることと、このリポジトリのコマンドを守れるかも、確かめていない。ADR: なし（本タスクは何も決めない）。
-   **進行中の計画書の旧い値は書き換えず、`_context.md` に読み替えを置く。** 未作成の PR マーカーが残る承認済みの計画書は、0073・0076・0077 の 3 つである（0029 は `withdrawn`）。読み替えは、`frontier-required` を `opus`、`frontier-recommended` を `sonnet+opus-review`、`standard` を `sonnet` とする。理由: 承認済みの文書に触れずに済み、読み替えが 1 か所に集まるため。値の行だけを書き換える案は、`frontier-recommended` の意味が「Opus が望ましい」から「Sonnet＋Opus のレビュー」に変わるので、編集上の訂正と呼べるかに議論の余地がある。決定の変更として `draft` に戻す案は、3 つの計画書の再承認を要する。ADR: ADR-0042。
-   **`sonnet+opus-review` の PR の Opus のレビューは、`/runplan` が PR を開く前に自分で実行する。** 理由: Codex の PR レビューは PR を開いたときの 1 回だけである（[ADR-0013](../../dev/adr/0013-review-setup.md)）。PR を開いた後に直したものは、Codex に見てもらえない。開く前に直せば、直した後のコードを Codex がレビューする。人が `/weakreview` を手で実行する手順も増えない。採らない案: PR を作った後、`/runpr` の前に人が `/weakreview` を実行する案（issue の例）と、`/runpr` が最初に実行する案。どちらも、直したものが Codex の PR レビューに届かない。ADR: ADR-0042。
-   **統合した指摘は、セッションが直し、確認のレビューを Opus で走らせる。** `/runplan` のセッションが Critical と Major の指摘を直す。直した後の確認のレビューは Opus で走らせる。理由: 指摘は場所が決まっているので、Sonnet でも直せる。直し方を誤っても、Opus の確認で見つかる。採らない案: 直す作業も Opus のサブエージェントに任せる案（費用が増える）と、報告だけにする案（PR が直す前の状態で開かれる）。ADR: ADR-0042。
-   **手で実行する `/weakreview` は、既定で直し、`--no-fix` を付けたときだけ報告だけにする。** 今の、`--fix` を付けたときだけ直す約束を逆にする。利用者の選択である。ADR: ADR-0042。
-   **セッションのモデルと実装モデル要件が合わなくても、`/runplan` は止めずに案内だけにする。ただし、`sonnet+opus-review` の PR を Opus のセッションで実行したときは、Opus のレビューを省く。** 理由: 実装もレビューも Opus になり、Opus のレビューを足すと同じ費用を二重に払うため。採らない案: Opus のセッションでもレビューを必ず走らせる案（仕組みは単純だが費用が二重になる）と、`opus` の PR を Opus 以外のセッションで実行したら止める案。ADR: ADR-0042。
-   **単一フェーズの計画書にも、実装モデル要件の値を持たせる。** `/mkplan` が、PR マーカーのある計画書と同じ規則で値と判定理由を決めて書く。`/runplan` と `/runpr` は、PR マーカーの値と同じように読む。理由: 単一フェーズの計画書でも、モデルを選び、Opus のレビューの要否を決める必要があるため。採らない案: 今のまま値を持たせない案。ADR: ADR-0042。
-   **ADR: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を作る。** 実装モデル要件の値とその意味、Opus のレビューを PR を開く前に置くこと、旧い値の読み替えは、後のすべての計画書と、複数のコマンドを縛る。これは `CLAUDE.md`、「Design decisions (ADR)」の基準で主要な決定であり、開発プロセスの方針に当たる。[ADR-0013](../../dev/adr/0013-review-setup.md) の決定（Codex の PR レビューを作成時の 1 回だけにする）は変えないので、ADR-0013 は更新しない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   実装モデル要件の値を見れば、`/runplan` を Opus と Sonnet のどちらで実行するかと、Opus のレビューが足されるかが分かるようにする。
-   `sonnet+opus-review` の PR では、人が別のコマンドを実行しなくても、PR を開く前に Opus のレビューが行われるようにする。
-   `/weakreview` 一つで、一般的な正しさのレビューと、能力の低いモデルの失敗を探すレビューを、Opus で行えるようにする。

### 2.2. スコープ (In Scope)

-   実装モデル要件の 3 つの値、その意味、当てはめる契機、判定理由の書き方（F-001）。
-   単一フェーズの計画書の値（F-002）。
-   旧い値の読み替え（F-003）。
-   `/runplan` と `/runpr` での値の案内（F-004）。
-   `/runplan` が PR を開く前に行う Opus のレビュー（F-005）。
-   `/weakreview` の改良（F-006）。
-   `CLAUDE.md` のコマンドの一覧の `/weakreview` の説明と、`_context.md` の PR marker conventions の記述（F-007）。
-   ADR-0042 の作成と、ADR の索引の更新。

### 2.3. スコープ外 (Out of Scope)

-   **DeepSeek-V4.1-Flash など、Claude 以外のモデルでの実装。** 理由は 1 節「要件定義の際に決めたこと」のとおりで、#790 で扱う。
-   **完了した計画書と、進行中の計画書（0073・0076・0077）の書き換え。** 旧い値は読み替えで扱う（F-003）。
-   **`/runplan` の中のレビュー（step 7・7.5）と、ローカルの CLI レビューのモデルの変更。** issue が求めていない。これらは今のまま、セッションのモデルを引き継ぐ。
-   **Codex の PR レビューの方針（ADR-0013）の変更。** PR レビューを作成時の 1 回だけにする方針は変えない。
-   **Sonnet と Opus の実際の費用の測定と、それを記録する仕組み。** 1 節「費用の目安」の前提（Sonnet が使うトークンの倍率）は、本タスクでは確かめない。
-   **`/code-review` と `/simplify` そのものの変更。** 組み込みのスキルであり、リポジトリの外にある。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 実装モデル要件の値と意味

`/mkplan2`（`/mkplan` の step 8.5 と、`/mkreq` の簡易プロセスから使われるものを含む）が PR マーカーに書く `**実装モデル要件**` を、次の 3 つの値のどれか一つにする。

| 値 | 意味 | 当てはめる契機 |
|---|---|---|
| `opus` | Opus で実装する。 | 今の `frontier-required` の契機（前例のない設計判断、`mkplan.md` step 8 の panel mode の契機） |
| `sonnet+opus-review` | Sonnet で実装し、PR を開く前に Opus のレビュー（F-005）を行う。 | 今の `frontier-recommended` の契機（競合するやり方があるステップ、Conditional checks に二つ以上当たるステップ、復旧処理・並行処理・状態機械などの複雑なステップ） |
| `sonnet` | Sonnet で実装し、追加のレビューはしない。 | それ以外 |

**Acceptance Criteria**:
- **AC-01**: `mkplan2.md` の PR マーカーの書式と説明が、`**実装モデル要件**` の値として `opus`・`sonnet+opus-review`・`sonnet` の 3 つだけを挙げる。`frontier-required`・`frontier-recommended`・`standard` を、新しく書く値として挙げない。
- **AC-02**: `mkplan2.md` が、上の表のとおり、各値の意味と当てはめる契機を述べる。PR は、その PR のステップが求める値のうち最も高いものをとる（`opus` が最も高く、`sonnet` が最も低い）と述べる。
- **AC-03**: `mkplan2.md` が、`**判定理由**` の書き方として、`opus` と `sonnet+opus-review` のときは当たった契機とあわせて「Sonnet に任せると何が起きやすいか」を、`sonnet` のときは当たる契機がないことを書くと述べる。
- **AC-04**: `mkplan2.md` の `### 3.2 PR 構成` の表の例と、step 9 の PR 境界のレビューの観点とチェックリストが、新しい値で書かれている。
- **AC-05**: 本タスクの後に `/mkplan2`（または `/mkplan`）が作った計画書の PR マーカーは、`**実装モデル要件**` に 3 つの値のどれか一つを持ち、`**判定理由**` を AC-03 の形で持つ。

#### F-002: 単一フェーズの計画書の値

PR マーカーのない単一フェーズの計画書にも、実装モデル要件の値と判定理由を持たせる。

**Acceptance Criteria**:
- **AC-06**: `mkplan.md` が、`### フェーズ` が一つの計画書にも、F-001 と同じ値・契機・判定理由の書き方で、実装モデル要件と判定理由を書くと述べる。書く場所は、計画書の中で一つに決まっている。
- **AC-07**: `/runplan` と `/runpr` は、単一フェーズの計画書の値を、PR マーカーの値と同じように読む（F-004・F-005）。
- **AC-08**: `mkreq.md` の簡易プロセスで書く単一フェーズの計画書にも、AC-06 の値と判定理由がある。

#### F-003: 旧い値の読み替え

本タスクの前に書かれた計画書の旧い値を、書き換えずに読み替える。

**Acceptance Criteria**:
- **AC-09**: `_context.md` の PR marker conventions が、読み替えを一つの表で持つ: `frontier-required` は `opus`、`frontier-recommended` は `sonnet+opus-review`、`standard` は `sonnet` として扱う。読み替えを持つ場所は、この表だけである。
- **AC-10**: `/runplan` と `/runpr` は、旧い値を持つ PR マーカーを読み替えの表で新しい値として扱う。旧い値を表示するときは、旧い値と読み替えた値の両方を示す。
- **AC-11**: 実装モデル要件の欄に、新しい 3 つの値と旧い 3 つの値のどれでもない文字列があるとき、`/runplan` は値を推し量らず、実装を始める前に止まる。止まった理由として、計画書の場所とその文字列を報告する。`/runpr` は、その文字列をそのまま示し、読めない値であることを添える。
- **AC-12**: 実装モデル要件の欄がない計画書（本タスクの前に書かれた単一フェーズの計画書や、欄のない PR マーカー）では、`/runplan` と `/runpr` は値がないものとして扱う。案内は値がないことを示し、`/runplan` は Opus のレビューを行わない。
- **AC-13**: 0073・0076・0077 の計画書は、本タスクの PR で変更されない。

#### F-004: 値の案内

**Acceptance Criteria**:
- **AC-14**: `runplan.md` の冒頭の案内が、3 つの値それぞれについて、セッションをどのモデルで始めるかと、Opus のレビューが足されるかを述べる。`opus` は Opus で、`sonnet+opus-review` と `sonnet` は Sonnet で始める。
- **AC-15**: `runplan.md` の冒頭の案内が、セッションのモデルと値が合わなくても `/runplan` は止まらないと述べる。`sonnet+opus-review` の PR を Opus のセッションで実行したときは、Opus のレビューを省くと述べる（F-005）。
- **AC-16**: `/runpr` が step 4 の質問と step 5 の `/clear` の案内で次の PR の実装モデル要件を示すとき、値に加えて、新しいセッションを始めるモデル（Opus か Sonnet）を示す。値がないときと読めないときは、今と同じくその旨と分かる範囲の理由を示す。

#### F-005: `/runplan` が PR を開く前に行う Opus のレビュー

実装モデル要件が `sonnet+opus-review` の PR で、`/runplan` のセッションが Opus 以外のモデルで動いているとき、`/runplan` は PR を開く前に、`/weakreview` と同じ二つのレビューを行う。

**Acceptance Criteria**:
- **AC-17**: 値が `sonnet+opus-review`（読み替えた値を含む）で、セッションが Opus 以外で動いているとき、`/runplan` は PR を開く前（`_lib/task-pr-handoff.md` の Part B より前）に、F-006 の二つのレビューを、その PR のすべての commit（`origin/main` との merge-base から HEAD まで）に対して行う。PR マーカーのある計画書でも、単一フェーズの計画書でも同じである。
- **AC-18**: 二つのレビューと、AC-19 の確認のレビューは、セッションのモデルに関係なく Opus で走る。Opus で走らせられないときは、セッションのモデルで代わりに走らせず、PR を開く前に止まり、その旨を報告する。何も push しない。
- **AC-19**: 二つのレビューの指摘は、重複を除いて一つにまとめ、トリアージする。`/runplan` のセッションは Critical と Major の指摘を直して commit し、直したことの確認のレビューを Opus で走らせる。確認のレビューの繰り返しの上限と、上限に達したときに止まって報告することは、`_lib/review-subagent-pattern.md` の規則に従う。
- **AC-20**: 値が `sonnet+opus-review` でもセッションが Opus で動いているときは、このレビューを行わない。値が `opus`・`sonnet` のとき、値がないとき（AC-12）も行わない。
- **AC-21**: PR の説明は、この PR で Opus のレビューを行ったか、省いたか、行わなかったかと、その理由を示す。行ったときは、指摘の件数を重大度ごとに示し、直さなかった Critical と Major があればその内容を示す。
- **AC-22**: このレビューの直しの commit は、PR を開く前にブランチに入っている。そのため、PR を開いたときの Codex の PR レビューの範囲に入る。

#### F-006: `/weakreview` の改良

**Acceptance Criteria**:
- **AC-23**: `/weakreview` は、二つのレビューを行う。一つは一般的な正しさのレビュー（`/code-review` に相当する範囲）、もう一つは今の「Weak-model failure checklist」によるレビューである。
- **AC-24**: 二つのレビューは、`/weakreview` を実行するセッションのモデルに関係なく Opus で走る。Opus で走らせられないときは、セッションのモデルで代わりに走らせず、止まってその旨を報告する。
- **AC-25**: `/weakreview` は、二つのレビューの指摘を重複を除いて一つにまとめ、重大度ごとに報告する。
- **AC-26**: 引数に `--no-fix` がないとき、`/weakreview` は Critical と Major の指摘を直し、直したことの確認のレビューを Opus で走らせる（AC-19 と同じ規則）。`--no-fix` があるときは、何も直さずに報告だけをする。
- **AC-27**: `/weakreview` が受け付けない引数（`--fix` を含む）を与えられたときは、推し量らずに止まり、受け付ける引数を示す。
- **AC-28**: `weakreview.md` の Purpose が、`code-review` が別に走ることを前提にしない形に書き換えられている。
- **AC-29**: `/weakreview` を手で実行するときの範囲の指定（commit の範囲、PR 番号、省略時の `origin/main...HEAD`）は、今と同じに受け付ける。

#### F-007: 関連する記述

**Acceptance Criteria**:
- **AC-30**: `_context.md` の PR marker conventions が、新しい 3 つの値と、それを書くコマンドと読むコマンドを述べる。`mkplan.md` が値を書く場面として単一フェーズの計画書（F-002）を含む。
- **AC-31**: `CLAUDE.md` のコマンドの一覧の `/weakreview` の説明が、F-006 の振る舞い（Opus の二つのレビュー、既定で直す）と合っている。
- **AC-32**: `.claude/commands/` の中に、実装モデル要件の値として旧い 3 つの値を新しく書く指示が残っていない。旧い値が現れるのは、`_context.md` の読み替えの表（AC-09）と、旧い値を読むときの扱いを述べる箇所だけである。
- **AC-33**: ADR-0042 が、本書の決定（3 つの値とその意味、Opus のレビューを PR を開く前に `/runplan` が行うこと、直す主体、Opus のセッションでは省くこと、単一フェーズの計画書の値、旧い値の読み替え、`/weakreview` の既定）を `## 決定` に持ち、ADR の索引に行がある。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   **費用:** `sonnet+opus-review` の PR では、Opus のレビュー（二つのレビューと確認のレビュー）の分だけ費用が増える。1 節「費用の目安」の集計では、レビューのサブエージェントは 1 回で中央値約 $0.5、上位四分位で約 $1.2 だった。この費用は、Sonnet で実装して節約する額より小さいことを前提にしている。この前提は確かめていない（2.3 節）。
-   **時間:** `sonnet+opus-review` の PR では、PR を開くまでの時間がレビューの分だけ延びる。時間の上限は設けない。

### 4.2. セキュリティ (Security)

-   N/A。条件付きのガイド（`docs/dev/security.md`）の契機に当たらない。変えるのは開発用のコマンドの手順で、外部コマンドの起動、API キーと Webhook の URL、キャッシュ、プロンプトへの信頼できないテキストの埋め込みには触れない。レビューのサブエージェントが Anthropic の API にコードを送ることは今と同じで、送り先のサービスも変わらない。開発用ツールの信頼境界（ADR-0015）も変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   Opus のレビューを Opus で走らせられないときは、より弱いモデルで黙って代えずに止まる（AC-18、AC-24）。理由: レビューが Opus で走ったと思い込んだまま PR が開かれると、`sonnet+opus-review` の意味が失われるため。
-   読めない値は推し量らずに止まる（AC-11）。`CLAUDE.md` の「Reject, don't normalize」に従う。

### 4.4. 互換性 (Compatibility)

-   本タスクの前に書かれた計画書は、書き換えずに読み替えの表で読める（F-003）。
-   `/weakreview` の `--fix` は受け付けなくなる（AC-27）。直すのが既定になるためである。旧い呼び方は、黙って受け入れずに止まる。

### 4.5. 保守性 (Maintainability)

-   値にはモデルのファミリー名だけを使い、版を含めない。Opus や Sonnet の版が上がっても、値と計画書は書き換えない。
-   二つのレビューの指示は、`/weakreview` と `/runplan` で一つの定義を共有する（`CLAUDE.md`、「DRY」）。共有の仕方は `02_architecture.md` で決める（`design_handoff.md`）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   Codex の PR レビューは PR を開いたときの 1 回だけとし、追加の PR レビューは頼まない（ADR-0013）。
-   コマンドの定義は英語で書く（`CLAUDE.md`、「Document language」の原則 (b)）。

## 6. 用語集 (Glossary)

-   **実装モデル要件:** 計画書の PR マーカー（単一フェーズの計画書では、F-002 が決める場所）にある `**実装モデル要件**` の値。`/runplan` を実行するモデルと、Opus のレビューを足すかを示す。
-   **Opus のレビュー:** 一般的な正しさのレビューと、能力の低いモデルの失敗を探すレビューの二つを Opus で行い、指摘を統合したもの。`/weakreview` と、`sonnet+opus-review` の PR の `/runplan` が行う。
-   **旧い値:** 本タスクの前の実装モデル要件の値、`frontier-required`・`frontier-recommended`・`standard`。
-   **読み替えの表:** 旧い値を新しい値として扱うための、`_context.md` の表（AC-09）。
-   ほかの用語は `docs/translation_glossary.md` と統一する。
