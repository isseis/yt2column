# ADR-0014: 決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

開発の手順には、入力が同じなら結果が同じで、分岐が多く、安全側に倒すことが要る判定がある。`/fixpr` と `/runpr` のガードは、どのレビューのスレッドを処理するか、修正が及んでよい範囲はどこか、git の状態が始めたときと同じか、ビルドとテストの後に何を commit して push するかを決める。カバレッジの比較は、テストを削除する前後で、製品の関数の網羅が下がっていないかを関数ごとに比べる。

これらをエージェントの判断や、コマンドの定義に書いたシェルの手順に任せると、同じ入力でも結果が変わりうる。誤ると、レビューの信頼の境界が実行時に働かなくなったり、範囲外の変更が commit されたりする。誤りは、テストで壊して失敗を確かめられなければ見つけられない。

この問題に対しては、次の 4 つの問いに答える必要がある。

1.  **判定の置き場所:** 判定を、エージェントの判断・コマンドの定義に書いた手順・道具のコードのどこに置くか。
2.  **基準値と順序の守り方:** 判定の基準値と処理の順序を、エージェントの読み違いや、修正のフェーズが実行するコードから、どう守るか。
3.  **道具と文書の食い違いの防ぎ方:** 道具と、それを呼ぶコマンドの定義や文書が、違う約束のまま黙って動くことを、どう防ぐか。
4.  **道具を入れる人:** 道具を、誰がどのコミットから入れるか。

## 決定

4 つの問いに、次のように答えた。判定と副作用を持つ道具はテストのある Go で書き、基準値と順序は道具のコードが守り、道具と文書は契約とプロトコル番号で揃え、道具は利用者が `main` から入れてエージェントは入れもビルドもしない。規則の本文と手順は [dev_tools.md](../conventions/dev_tools.md) が持ち、規則 1〜6 の項目は規約の節をその見出しの名前で指す。各問いで比べたほかの選択肢と、採らなかった理由は、「採らなかった案」の同じ見出しの下にある。

### 1. 判定の置き場所

**選んだもの:** 判定と副作用を持つ道具の判定は、エージェントの判断にも、コマンドの定義に書いたシェルの手順にも任せず、テストのある Go の道具のコードに置く。判定の誤りを、テストで壊して失敗を確かめて見つけられるのは、テストのあるコードだけだからである。拡張の関数の範囲を求める TypeScript の補助と、副作用を持たない判定のシェルスクリプトも、判定をテストのある 1 か所に保つ。

-   **Go の道具とテスト:** 判定と副作用を持つ開発の道具は、Go で書き、テストを持つ。判定の誤りを、テストで壊して失敗を確かめることで見つけられるようにするためである。何を決めるかは [ADR-0015](0015-untrusted-review-threads.md)・[ADR-0013](0013-review-setup.md)・[ADR-0037](0037-test-deletion-coverage.md)・[ADR-0046](0046-chief-of-staff.md)・[ADR-0051](0051-number-reservation.md)・[ADR-0054](0054-ci-skip-doc-only-commit.md) と [mergepr_guide.md](../guides/mergepr_guide.md) が定める。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 1: Write a tool in Go, with tests」と「Kinds of tools」にある。CI 専用ツール（`ciskip`）は GitHub Actions ワークフローから `go run` で直接実行するため、インストール型ツールではなく、都度ビルド型ツールと同様に、後述の「4. 道具を入れる人」で述べる Rule 5（利用者が `main` からインストールし、エージェントはインストールもビルドもしない規則）とプロトコル番号の対象外とする（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2・§2.2）。
    -   **食い違い:** CI 専用ツール（`ciskip`）の区分と Rule 5 の適用除外は、まだ [dev_tools.md](../conventions/dev_tools.md) に反映されておらず、`cmd/ciskip` と `internal/ciskip` も未作成である。この食い違いは https://github.com/isseis/yt2column/issues/1073 の実装で解消する。
-   **拡張の関数の範囲を求める TypeScript の補助:** 拡張の関数の範囲だけは、TypeScript の補助に求める。`coveragecompare` は Go と拡張の両方の比較を受け持つが、拡張の関数の終わりの行は node の lcov が出さないので、`extension/scripts/function-ranges.ts` が入っている `typescript` の構文木から求める。新しい npm のパッケージは加えない。拡張の関数の終わりの行を、lcov からは求められないためである。規則は [0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) の 3.5「拡張の測り方」にある。
-   **副作用のない判定のシェルスクリプト:** 例外として、記録の行を読んで判定するだけで副作用を持たない判定は、コマンドの定義から独立したシェルスクリプトにしてよい。判定は記録の行と `git rev-list` の組み合わせで足り、Go の開発ツールにすると、新しい `cmd/` と `internal/` を足し、利用者にも入れ直しを求めることになるためである。対象は、上位モデルのレビューの判断のスクリプト（`_lib/high-review-decision.sh`。[ADR-0042](0042-implementation-model-tiers.md)）と、ローカルの CLI レビューが見たかとクロスチェックの部分レビューの記録が有効かを決める `_lib/review-records.sh`（[ADR-0013](0013-review-setup.md)、[ADR-0050](0050-fable-partial-review.md)）である。前者はこの例外より前に作られ、常設の Go テストが確かめるのは計画書の値の読み方だけで、判断の規則はタスク 0081 の一度きりの確認で確かめた。採らなかった案の「判定をコマンドの定義に書いたシェルの手順」では、エージェントが文を読んで判定する。この例外では、テストを持つ 1 つのファイルが判定する点が違う（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 15）。4 つの条件と、この例外より後に足すスクリプトの扱いは [dev_tools.md](../conventions/dev_tools.md) の「Rule 6: Exception: a side-effect-free record check may be a shell script」にある。

### 2. 基準値と順序の守り方

**選んだもの:** 判定の基準値はディスクのファイルに置かず、セッションが会話に持って次の呼び出しの引数で渡す。処理の順序は、コマンドの定義ではなく道具のコードで強制する。ファイルに置いた基準値は修正のフェーズが実行するコードに書き換えられ、定義に書いた順序はエージェントが読み違えうるためである。基準値の例外は chief である。

-   **基準値を状態のファイルから読み返さないこと:** 道具は、判定に使う基準値を状態のファイルから読み返さない。判定をディスクに置くと、修正のフェーズや build checks が実行するコードに書き換えられるためである。例外は次の項目にある。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 2: Do not read a decision's baseline back from a state file」にある。
-   **chief の例外:** chief の道具は、リポジトリの外の状態のディレクトリにある次の記録を、判定に読み返す。自動送信の記録、送信の記録、送信の停止、待ち受けの記録（待ち受けのラウンドごとの記録と、PR が開いているのを見た記録）、通知の記録、PR の詳細の控え、ロック、workspace を作ったときの記録である。記録は、送るコマンドとタスクを決めるためには使わない。送ることについては、送ることを止める条件（リセットの候補を狭める PR の詳細の控えを含む）として使うか、自動送信の候補の workspace を名指して Herdr・git・GitHub で確かめ直す条件として使うだけである。通知の記録は通知の重複を防ぐことと待っている時間による並べ方に、送信の記録は送った入力の履歴と「実行中」の行の表示に使い、どちらも送る中身を左右しない。送ることに関わる記録が読めなければ送らない。workspace を作ったときの記録は、削除の候補の workspace を名指すことと、その行に示すタスクを探す手がかりにだけ使い、名指した workspace は Herdr・git・`origin/main` の文書で確かめ直す。読めなければ削除を提案しない。作ったときの記録は worktree ごとにあり、同じパスに作り直す前と、消し終えた後に、そのパスの分をすべて消す。起動し直しても自動送信を 2 回しないこと、送信を止めた workspace を覚えていること、chief を 1 つに限ることを、セッションの記憶では守れないためである（[ADR-0046](0046-chief-of-staff.md)）。範囲と、記録が条件として働く範囲は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.5「状態のディレクトリの記録とロック」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.5「作ったことの記録」にある。
-   **順序の道具のコードでの強制:** 処理の順序は、コマンドの定義ではなく道具のコードで強制する。エージェントが手順を読み違えても、範囲外の変更が commit されたり、信頼の境界が迂回されたりしないためである。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 3: Enforce the order in the tool's code」にある。

### 3. 道具と文書の食い違いの防ぎ方

**選んだもの:** 道具の引数・出力・終了コードを、文書が読む契約にする。道具は `main` から入れる。道具と文書の版を照合する道具はプロトコル番号を持ち、番号はマージの時点で `mergepr` が確かめる。人の手順や `/runpr` だけに確かめを任せると、確かめが黙って抜けうるためである。

-   **引数・出力・終了コードの契約:** 道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約にする。契約が食い違うと、レビューの信頼の境界が実行時に働かなくなるためである。規則と、契約が書かれている場所の一覧は [dev_tools.md](../conventions/dev_tools.md) の「Rule 4: Arguments, output, and exit codes are a contract」にある。
-   **`main` からのインストール:** 道具は `main` からインストールして使う。インストール済みの古い道具が、古い想定で黙って判定しないようにするためである。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」に、文書と道具を変える順序は同じ規約の「Rule 4: Arguments, output, and exit codes are a contract」にある。
-   **プロトコル番号:** `--protocol` の番号（プロトコル番号）を上げる PR は、番号を実装の時点の `origin/main` の値にちょうど 1 を足した値にし、マージの時点でも `origin/main` の値 + 1 であることを確かめる。食い違えばマージしない。タスク文書には上げた後の番号を書かず、「`main` の値 + 1」と書く。確かめは、`/runpr` からのマージにも `/mergepr` を単独で使うマージにも当てはめ、マージの直前にも確かめ直す。`mergepr` も番号を持ち、`/mergepr` の定義と版を照合する。並行する 2 つのブランチが同じ番号に上げると、git は定数の行の同じ変更を衝突なしにまとめ、文書のガードテストも通る。その結果、違う約束が 1 つの番号を共有したまま `main` に入り、古い文書と新しい道具が黙って違う規則で動きうるためである。対象は、道具とそれを呼ぶ文書の版を照合する番号で、今は `fixprguard`・`chief` と、確かめを入れる `mergepr` の 3 つである（`codexreview` は Codex CLI とともに撤去し、番号も道具ごと外す。[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-008）。`mergepr` に番号を持たせるのは、確かめを入れる前の `mergepr` や古い定義でマージして、確かめが黙って抜けることを防ぐためである。確かめは `mergepr` の `prepare` と `merge` が行い、`merge` は GitHub にマージを依頼する直前に、`origin/main` を取り直して確かめ直す。`/runpr` と単独の `/mergepr` のどちらのマージも、この 2 つのサブコマンドを必ず通るためである。番号を持つ道具は、`internal/<名前>/` の直下のファイルに、`const Protocol = <整数>` の 1 行で番号を置く。確かめがこの形の行だけを番号として読むためである。この形でない宣言は、確かめが読めない値として止まるか、確かめの対象の道具と文書のガードの一覧を突き合わせるテストが CI で止める。確かめの仕様は [mergepr_guide.md](../guides/mergepr_guide.md) にある。受け入れた危険は 4 つある。(1) マージの前は、同じ道具の番号を上げる並行するブランチが同じ番号を共有し、片方のブランチから入れた道具がもう片方の文書を受け付けうる。ブランチごとに一意の番号を配る仕組みは、issue #933 の範囲を超えるためである。(2) マージの直前の確かめ直しと GitHub のマージの間の数秒に、ほかの PR が同じ番号でマージされうる。(3) 番号を上げたブランチを、同じ番号の入った `main` の上へ rebase するか履歴をまとめ直すと、確かめが衝突を見落としうる。(4) 一度消した道具を同じ名前で足し直すと、番号が `1` に戻る。規則の正本は [CLAUDE.md](../../../CLAUDE.md) の「Development Notes」の「Protocol numbers」にあり、確かめの要件は [0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md) の F-003 にある。

### 4. 道具を入れる人

**選んだもの:** 道具は利用者が入れ、エージェントは入れることもビルドすることもしない。開発のための例外だけを残す。入れることだけを禁じても、エージェントは `go run` や `go build` でブランチの判定を走らせられるためである。

-   **エージェントがインストールもビルドもしないこと:** エージェントは、インストール型ツールをどれもインストールもビルドもしない。エージェントが PR のブランチから `fixprguard` や `mergepr` を入れるか走らせると、そのブランチが変えた判定で `/fixpr`・`/runpr`・`/mergepr` が動くことになり、`main` の判定を迂回できるためである。利用者が PR のブランチで入れ直したあと、そのツールがブランチの判定で動くことは受け入れる。利用者がその PR の変更を見たうえで入れ直すためである。Claude Code では `.claude/settings.json` に拒否の規則を置き（auto モードで確認なしに入れることを防ぐ層。[ADR-0046](0046-chief-of-staff.md) と同じ理由）、Antigravity では `AGENTS.md`・`CLAUDE.md`・ハーネスのアダプタ（[ADR-0055](0055-harness-independent-commands.md)）の指示とサンドボックス外での実行（`BypassSandbox: true`）時の利用者承認で守る。規則と開発のための例外は [dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」に、拒否の規則の形は同じ節の「The permission layer」にある。
    -   **食い違い:** Antigravity 用のハーネスのアダプタと `AGENTS.md` への禁止規則の明記は未実施である。この食い違いは https://github.com/isseis/yt2column/issues/1072 の実装で解消する。

## 採らなかった案

問いごとに、比べて採らなかった選択肢と、その理由を示す。

### 1. 判定の置き場所

-   **判定をコマンドの定義に書いたシェルの手順にする。** 判定が多く、安全側に倒すことが要るので、エージェントの判断では結果が変わらないようにできない。テストで壊して失敗を確かめることもできない（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1、§1.3）。
-   **Codex の最終メッセージの判定をエージェントに任せる。** 文法だけで決まる機械的な判定なので、テストのあるコードで行う（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **素の `codex review` の標準出力や、セッションの記録から完了を判定する。** 素の出力には完了を示す欄がない。セッションの記録にある構造化された結果は公開された仕様ではなく、実行ごとに 2 つできるファイルから選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **カバレッジの比較を node のスクリプトで書く、または Go と拡張で別の道具にする。** 差分による関数の対応、分類、比較、結果の出し方を 1 か所に保てない。判定の論理は Go に置き、TypeScript は拡張の関数の範囲だけを求める（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3）。
-   **`ext-coverage-compare` を node だけで完結させる。** 拡張の lcov は関数の終わりの行を出さないので、関数の範囲を求めるのに TypeScript の構文解析が要る。それは node の側でしか行えない（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3、§3.5）。

### 2. 基準値と順序の守り方

-   **ガードの道具が基準値をディスクのファイルに持つ。** そのファイルは、修正のフェーズのエージェントや build checks が実行するコードから書き換えられる（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1）。
-   **commit を `git add -A` で行う。** 修正のフェーズが作った未追跡のファイル（作業用に作った一時ファイルを含む）も commit に入り、範囲の判定と照合していない変更が commit されるため（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) AC-21、[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §3.6）。

### 3. 道具と文書の食い違いの防ぎ方

-   **プロトコル番号のマージの時点の確かめを、CLAUDE.md に人の手順として書くだけにする。** 確かめを忘れたときに、同じ番号の衝突を防げない（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号の確かめを `/runpr` だけに入れる。** `/mergepr` を直接使うと確かめが抜ける（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号の確かめを、`mergepr` の独立したサブコマンドにして、`/mergepr` と `/runpr` の定義から呼ぶ。** 定義が呼び忘れると、確かめが黙って抜ける（[0099 設計書](../../tasks/0099_protocol_number_rule/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号を、CI の必須チェックと「ブランチを最新にする」設定で確かめる。** すべての PR で、マージの前に更新が要る（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **マージの前も、並行するブランチが同じ番号を使わないようにする。** ブランチごとに一意の番号を配る仕組みが要り、issue #933 の範囲を超える（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号が食い違ったとき、`/runpr` が番号と文書の `--protocol` を書き換えて自動で直す。** `/runpr` を受け持つモデルに複数の文書を書き換えさせると誤りうる（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号の決め方を、新しい ADR にする。** 同じ主題の決定が 2 つの ADR に分かれる（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。

### 4. 道具を入れる人

-   **インストールだけを禁じる案:** `go run` や `go build` で作ったバイナリで、ブランチの判定を走らせられる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **開発のための場合も含めてソースからの実行をすべて禁じる案:** ツールを変えるタスクの実装の確かめが、利用者頼みになる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **開発のための例外に副作用なしの条件を付けない案:** ツールを変えるタスクで、ソースから走らせたツールが取り消せない記録の tag を作るなどしうる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **`.claude/settings.json` で `go build ./cmd/*` と `go run ./cmd/*` も拒む案:** 開発のための例外が使えなくなる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **対象を `codexreview`・`numreserve`・`chief` の 3 つに限る案:** `fixprguard` と `mergepr` について規則がないままになる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **指示の文だけで守る案（Claude Code）:** auto モードでは、確認なしにツールを入れられる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **Antigravity 向けに Hook 等で禁止コマンドを機械的に拒否する仕組みを作る案:** 機械的な拒否は増えるが、ハーネス固有の仕組みの保守対象が増える（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **規約だけに書き `CLAUDE.md` には写しを置かない案:** 規約を読まないエージェントには届かない（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **手順の主語を揃える作業を別の issue にする案:** 規則と手順がぶつかったまま `main` に入る（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。

## 結果

本決定により、開発ツールの判定と信頼の境界を実行時とテストで守れる一方、次の運用負担と残余の危険が生じる。

-   **テストによる誤りの検出:** 判定の誤りは、ツールのテストで見つかる。壊して失敗を確かめた結果は、そのテストを加えたコミットのメッセージに残る。
-   **実行時の境界強制:** エージェントが手順を読み違えても、範囲外の commit・push や、信頼の境界の迂回は、ツールが実行時に止める。
-   **利用者の再インストール手順:** ツールを変えたマージの後は、利用者が `make install-<tool>` を実行し直す必要がある（`mergepr`・`fixprguard`・`chief`・`numreserve`）。再インストールを忘れると、`fixprguard` は `--protocol` の食い違いで止まる。ただし、同じツールの番号を上げる並行するブランチの間では、番号が同じなので止まらない。確かめを入れた後は、`mergepr` を入れ直さないと、`/mergepr` が `--protocol` の食い違いで止まる。
-   **カバレッジ比較の依存ツール:** 拡張のカバレッジの比較には Go と node の両方が要る。`make ext-coverage-compare` も Go のツールを呼ぶ。
-   **プロトコル番号の運用:** プロトコル番号を上げるタスクの文書は、上げた後の番号を書かず「`main` の値 + 1」と書く。マージの時点で番号が食い違えば、マージは止まる。番号と文書を直し、ツールを入れ直してから、マージをやり直す。
-   **ツールと文書の同時更新:** ツールの引数・出力・終了コードは、コマンドの定義と文書のガードが前提にする契約になる。変えるときは、ツール・文書・ガードを同じ PR で変える。
-   **インストール・ビルド禁止の強制範囲と残る危険:** エージェントがインストール型ツールをインストールもビルドもしない規則は、指示の文と、Claude Code にだけ効く `make install-*` と `go install` の拒否の規則で守る。`go build` と `go run` は指示の文だけで守る。Antigravity には `.claude/settings.json` のようなリポジトリ設定による機械的な拒否（`deny`）がないため、指示の文と `BypassSandbox: true` 時の利用者承認で守る（[ADR-0055](0055-harness-independent-commands.md)）。エージェントが指示に従わない危険は受け入れる。`/runpr` などを走らせる OpenCode が `CLAUDE.md` を読むかは確かめておらず、読まない場合の危険も受け入れる。
-   **ビルド元コミットの確認限界:** `PATH` にあるツールのビルド元のコミットは `go version -m` の `vcs.revision` で分かるが、コミットしていない変更からの入れ直し（`vcs.modified` にだけ出る）、`go run` や別の場所のバイナリでの実行、`-buildvcs=false` のビルドは分からない。
-   **ブランチからの入れ直しの影響範囲:** プロトコル番号を上げる PR で利用者がブランチから入れ直したツールは、`/runpr` が途中で止まるか PR をやめたときも、利用者が `main` から入れ直すまで、すべての worktree で使われる。
-   **利用者への依頼による停止:** ツールを入れる手順に来たエージェントは止まるので、利用者が入れる手間が増える。

## 経緯

- 2026-10-21 `/fixpr` と `/runpr` の著者の信頼・修正の範囲・git の状態の判定と、build checks・commit・push・返信を、`fixprguard` に移した。判定の基準値を会話に持ち、副作用を信頼できない本文を読んだエージェントにさせない形にした（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md)）
- 2026-10-28 CodeRabbit CLI が失敗したときの Codex CLI の実行と、最終メッセージの形の判定を、`codexreview` に移した。時間と大きさの上限、環境変数、ツールの許可も、テストのあるコードで扱う（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
- 2026-11-02 テストを削除する前後のカバレッジの比較を、Go の `coveragecompare` で行うことにした。拡張の関数の範囲だけを TypeScript の補助に求め、`make coverage-compare` と `make ext-coverage-compare` から使う（[0055 要件定義書](../../tasks/0055_coverage_compare/01_requirements.md)）
- 2026-11-11 chief の判定と送信を Go の道具 `chief` に置いた。自動送信の記録・送信の停止・ロックなどを状態のディレクトリから判定に読み返すことは、例外として認めた。その記録は、送るコマンドとタスクを決めない。送ることについては、送ることを止める条件か、自動送信の候補の workspace を名指して確かめ直す条件としてだけ使い、通知の記録と送信の記録は、通知の重複の防止と並べ方、履歴と「実行中」の行の表示にだけ使う（[0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md)「設計の際に決めたこと（2026-11-11）」）
- 2026-11-16 chief が workspace を作ったときの記録を、判定に読み返す記録の例外に加えた。その記録は、削除の候補の workspace を名指して Herdr・git・`origin/main` の文書で確かめ直す条件としてだけ使う（[0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md)「設計の際に決めたこと（2026-11-16）」）
- 2026-11-17 プロトコル番号を上げる PR は、番号を `origin/main` の値 + 1 にし、マージの時点で確かめて食い違えばマージしないことにした。タスク文書には上げた後の番号を書かない。規則の正本を CLAUDE.md に置き、対象を `fixprguard`・`codexreview`・`chief` と、番号を持たせる `mergepr` の 4 つとした（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「issue #933 で決めたこと」「要件定義の際に決めたこと（2026-11-17）」）
- 2026-11-17 タスク番号と ADR 番号を決める道具 `numreserve` を、`main` から入れる Go の開発の道具に加えた（[0100 設計書](../../tasks/0100_number_reservation/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-17 プロトコル番号の確かめを `mergepr` の `prepare` と `merge` に置き、`merge` は GitHub にマージを依頼する直前に `origin/main` を取り直して確かめ直すことにした。番号は `internal/<名前>/` に `const Protocol = <整数>` の 1 行で置くこととし、規則の正本を CLAUDE.md の「Development Notes」の「Protocol numbers」に置いた（[0099 設計書](../../tasks/0099_protocol_number_rule/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-17 記録の行を読むだけの判定を、`origin/main` から取り出して走らせるテスト付きのシェルスクリプトにしてよいとする例外を加えた（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-19 エージェントが道具をインストールもビルドもしないことを、利用者が `main` から入れる道具のすべて（`fixprguard`・`mergepr` を含む）に当てはめ、ビルドの意味と開発のための例外を定め、プロトコル番号を上げる PR のブランチで入れ直すのも利用者とした。規則は指示の文と、`make install-*` と `go install` を拒む `.claude/settings.json` の規則で守る（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）
- 2026-11-20 Codex CLI をローカルの CLI レビューから外すのに合わせ、`codexreview` をプロトコル番号ごと撤去することにした。番号を持つ道具は `fixprguard`・`chief`・`mergepr` の 3 つになる（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）
- 2026-11-21 CI で重いジョブ（`test`・`lint`・`extension`）のスキップ可否を判定する `ciskip`（`cmd/ciskip` + `internal/ciskip`）を、テストのある Go 製の CI 専用ツールとして加えることにした。GitHub Actions から `go run` で実行するため、インストール型ツールではなく、都度ビルド型ツールと同様に Rule 5 とプロトコル番号の対象外とする（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2、[ADR-0054](0054-ci-skip-doc-only-commit.md)）
- 2026-11-21 Antigravity でもインストール型ツールのインストール・ビルド禁止を `AGENTS.md`・`CLAUDE.md`・ハーネスのアダプタの指示と `BypassSandbox: true` の利用者承認で守り、リポジトリ設定による機械的な拒否（`deny`）がない点は受け入れる危険とした（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」、[ADR-0055](0055-harness-independent-commands.md)）
