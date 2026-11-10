# ADR-0014: 決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

開発の手順には、入力が同じなら結果が同じで、分岐が多く、安全側に倒すことが要る判定がある。`/fixpr` と `/runpr` のガードは、どのレビューのスレッドを処理するか、修正が及んでよい範囲はどこか、git の状態が始めたときと同じか、ビルドとテストの後に何を commit して push するかを決める。ローカルの CLI レビューは、Codex CLI の実行と、その最終メッセージが指摘を述べているかの判定を行う。カバレッジの比較は、テストを削除する前後で、製品の関数の網羅が下がっていないかを関数ごとに比べる。

これらをエージェントの判断や、コマンドの定義に書いたシェルの手順に任せると、同じ入力でも結果が変わりうる。誤ると、レビューの信頼の境界が実行時に働かなくなったり、範囲外の変更が commit されたりする。誤りは、テストで壊して失敗を確かめられなければ見つけられない。

## 決定

-   **判定と副作用を持つ開発の道具は Go で書き、テストを持つ。** 判定の誤りを、テストで壊して失敗を確かめることで見つけられるようにするためである。今日あるのは `fixprguard`・`codexreview`・`coveragecompare`・`mergepr` の 4 つで、どれも `cmd/<名前>` と `internal/<名前>` の組であり、製品の 3 つのバイナリには入らない。何を決めるかは [ADR-0015](0015-untrusted-review-threads.md)・[ADR-0013](0013-review-setup.md)・[ADR-0037](0037-test-deletion-coverage.md) と [mergepr_guide.md](../guides/mergepr_guide.md) が定め、導入手順は [CLAUDE.md](../../../CLAUDE.md) の「Build Commands」にある。
-   **道具は、判定に使う基準値を状態のファイルから読み返さない。** 基準値と判定した集合は、サブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。`fixprguard` は利用者のために git のディレクトリへ記録を書くが、それを判定に読み返さない。判定をディスクに置くと、修正のフェーズや build checks が実行するコードに書き換えられるためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
-   **順序を、コマンドの定義ではなく道具の中で強制する。** 確認より前に build checks を動かさないこと、commit と push の間で hooks や config が変わっていたら push しないこと、は道具のコードが守る。コマンドの定義に残るのは、サブコマンドを呼ぶ順序と、利用者への確認だけである。エージェントが手順を読み違えても、範囲外の変更が commit されたり、信頼の境界が迂回されたりしないためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
-   **拡張の関数の範囲だけは、TypeScript の補助に求める。** `coveragecompare` は Go と拡張の両方の比較を受け持つが、拡張の関数の終わりの行は node の lcov が出さないので、`extension/scripts/function-ranges.ts` が入っている `typescript` の構文木から求める。新しい npm のパッケージは加えない。拡張の関数の終わりの行を、lcov からは求められないためである。規則は [0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) の 3.5「拡張の測り方」にある。
-   **道具は `main` からインストールして使う。** `mergepr`・`fixprguard`・`codexreview` は `make install-mergepr`・`make install-fixprguard`・`make install-codexreview` で入れ、道具を変えたマージの後は再インストールする。`--protocol` の番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで再インストールする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §3）。`coveragecompare` は `make coverage-compare` と `make ext-coverage-compare` が、そのつど一時ディレクトリにビルドして実行する。`fixprguard` は `--protocol` の番号で、呼び出し側の文書と道具の版が合っているかを照合する。あわせて、文書と道具を変える順序は、両方が動き続ける形にする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §9）。インストール済みの古い道具が、古い想定で黙って判定しないようにするためである。
-   **道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約にする。** 変えたときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える。契約が食い違うと、レビューの信頼の境界が実行時に働かなくなるためである。契約は、[CLAUDE.md](../../../CLAUDE.md) の「Build Commands」「Test Commands」「Development Notes」、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Guard tool」の行、[fixprguard_guide.md](../guides/fixprguard_guide.md)、[mergepr_guide.md](../guides/mergepr_guide.md) にある。

## 採らなかった案

-   **判定をコマンドの定義に書いたシェルの手順にする。** 判定が多く、安全側に倒すことが要るので、エージェントの判断では結果が変わらないようにできない。テストで壊して失敗を確かめることもできない（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1、§1.3）。
-   **commit を `git add -A` で行う。** 修正のフェーズが作った未追跡のファイル（作業用に作った一時ファイルを含む）も commit に入り、範囲の判定と照らしていない変更が commit されるため（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) AC-21、[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §3.6）。
-   **ガードの道具が基準値をディスクのファイルに持つ。** そのファイルは、修正のフェーズのエージェントや build checks が実行するコードから書き換えられる（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1）。
-   **Codex の最終メッセージの判定をエージェントに任せる。** 文法だけで決まる機械的な判定なので、テストのあるコードで行う（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **素の `codex review` の標準出力や、セッションの記録から完了を判定する。** 素の出力には完了を示す欄がない。セッションの記録にある構造化された結果は公開された仕様ではなく、実行ごとに 2 つできるファイルから選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **カバレッジの比較を node のスクリプトで書く、または Go と拡張で別の道具にする。** 差分による関数の対応、分類、比較、結果の出し方を 1 か所に保てない。判定の論理は Go に置き、TypeScript は拡張の関数の範囲だけを求める（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3）。
-   **`ext-coverage-compare` を node だけで完結させる。** 拡張の lcov は関数の終わりの行を出さないので、関数の範囲を求めるのに TypeScript の構文解析が要る。それは node の側でしか行えない（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3、§3.5）。

## 結果

-   判定の誤りは、道具のテストで見つかる。壊して失敗を確かめた結果は、そのテストを加えたコミットのメッセージに残る。
-   エージェントが手順を読み違えても、範囲外の commit・push や、信頼の境界の迂回は、道具が実行時に止める。
-   道具を変えたマージの後は、利用者が `make install-<tool>` を実行し直す必要がある（`mergepr`・`fixprguard`・`codexreview`）。再インストールを忘れると、`fixprguard` は `--protocol` の食い違いで止まる。
-   拡張のカバレッジの比較には Go と node の両方が要る。`make ext-coverage-compare` も Go の道具を呼ぶ。
-   道具の引数・出力・終了コードは、コマンドの定義と文書のガードが前提にする契約になる。変えるときは、道具・文書・ガードを同じ PR で変える。

## 経緯

- 2026-10-21 `/fixpr` と `/runpr` の著者の信頼・修正の範囲・git の状態の判定と、build checks・commit・push・返信を、`fixprguard` に移した。判定の基準値を会話に持ち、副作用を信頼できない本文を読んだエージェントにさせない形にした（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md)）
- 2026-10-28 CodeRabbit CLI が失敗したときの Codex CLI の実行と、最終メッセージの形の判定を、`codexreview` に移した。時間と大きさの上限、環境変数、ツールの許可も、テストのあるコードで扱う（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
- 2026-11-02 テストを削除する前後のカバレッジの比較を、Go の `coveragecompare` で行うことにした。拡張の関数の範囲だけを TypeScript の補助に求め、`make coverage-compare` と `make ext-coverage-compare` から使う（[0055 要件定義書](../../tasks/0055_coverage_compare/01_requirements.md)）
