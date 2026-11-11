# ADR-0035: 文書の言語は読み手と承認者で決め、README だけを日英の 2 言語で同期し、PR のタイトルは日本語、squash の件名は英語にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリでは、書き手と読み手が言語ごとに分かれる。開発者も拡張の利用者も日本語の話者である一方、エージェントの実行を制御する指示とコードは英語で書く（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md) §1）。そのため、どの文書とどのテキストを何語で書くかを決める必要がある。決め方を場所ごとの例外の列挙にすると、原則から導けず、新しい文書を足すたびに判断が要る（同 §1）。

2 つの言語で持つ文書は、片方を変えたときにもう片方が古くなりやすい。いつ同期するかをエージェントの判断に任せると、手作業の変更や手順を通らない変更で同期が抜ける（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）。

GitHub に置くものは日本語で書く。一方、squash のコミットは `main` に残る唯一のコミットであり、リポジトリに残るものは英語で書く。そのため、PR のタイトルと squash の件名の言語を分けて決める必要がある（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md) §1）。

## 決定

-   **文書の言語は、置き場所ではなく、誰が読み誰が承認するかで決める。** 人間が判断・承認する文書と人間向けの手引きは日本語、エージェントの実行を制御する指示とコードは英語、エンドユーザー向けの文書（README）は日本語を正本として英語版を同期する。原則、ディレクトリへの当てはめ、例外は、[CLAUDE.md](../../../CLAUDE.md) の「Document language」に従う。置き場所ごとの例外の列挙では原則から導けず、新しい文書を足すたびに判断が要るためである。
-   **会話・リポジトリ・AI へのプロンプト・GitHub のそれぞれの言語は、[CLAUDE.md](../../../CLAUDE.md) の「Language」に従う。** Claude Code のセッションの会話と GitHub に置くものは日本語、コード・コミットのメッセージ・プロンプトは英語である。理由は記録されていない。
-   **ソースのコメント・識別子・文字列リテラルは英語とし、日本語のリテラルを置いてよい場所は例外として範囲を限る。** エージェントの実行を制御する指示とコードは英語で書く、という原則によるものである。例外の一覧と、それぞれの範囲・理由・境界は、[CLAUDE.md](../../../CLAUDE.md) の「Go Idioms」に従う。現在のコードでは、例外に当たらない日本語の文字列リテラルも製品のコードにある。記事の本文として出力する出典のブロックの見出しと、YouTube の文字起こしパネルが出す文字列と照合する字幕のパーサの単位と操作のラベルである。どちらもコメントで日本語にする理由を述べている。このずれは、記録した issue [#726](https://github.com/isseis/yt2column/issues/726) で扱う。拡張の UI を 2 言語にすることは [ADR-0029](0029-extension-ui-languages.md) が定める。
-   **2 言語で持つ文書は README だけである。** `README.md` を先に変え、[mktrans.md](../../../.claude/commands/mktrans.md) の `/mktrans` で `README.en.md` に反映し、両方を 1 つのコミットに入れる。`README.en.md` の 1 行目の同期の印は、翻訳の元にした `README.md` の版を指し、`/mktrans` はその版からの差分を翻訳する。印の形と翻訳の手順は [mktrans.md](../../../.claude/commands/mktrans.md) に、同期の規則は [CLAUDE.md](../../../CLAUDE.md) の「Translation Guidelines (Japanese to English)」に従う。2 つの言語で持つ文書は、片方を変えたときにもう片方が古くなりやすく、同期をエージェントの判断に任せると、手作業の変更や手順を通らない変更で同期が抜けるためである。
-   **同期の印は、エージェントの判断ではなく Go のテスト `TestReadmeSyncMarker` で確かめる。** テストは、印が今の `README.md` を指すことを確かめ、`make test`・CI の `doc-guard` のジョブ・pre-commit のフックで走る。同期の手順を持つコマンドは、この検査が失敗したときに `/mktrans` を実行する。プロンプトの判定は決定的でなく、手作業の変更は手順を通らないためである。規則は [CLAUDE.md](../../../CLAUDE.md) の「Translation Guidelines (Japanese to English)」にある。
-   **PR のタイトルは日本語で書き、squash のコミットの件名と本文は英語で書く。** PR のタイトルは conventional-commit の接頭辞を ASCII のまま残し、その後の要約を日本語で書く。squash の件名は、`/mergepr` が PR の材料から英語で書き、PR のタイトルからは接頭辞だけを読む。squash のコミットは `main` に残る唯一のコミットであり、リポジトリに残るものは英語で書くためである。形は [_context.md](../../../.claude/commands/_context.md) の「PR title and squash subject」の 1 か所に定める。
-   **PR のタイトルと説明の日本語は、用語集 [docs/translation_glossary.md](../../translation_glossary.md) の語で書く。** コマンドは書く前に用語集を読み、旧称と、用語集が使わないとする訳を使わない。用語集にない語が要るときは、勝手に作らず、最終報告で挙げて利用者に追加を委ねる。規則は [_context.md](../../../.claude/commands/_context.md) の「Translation glossary」の 1 か所に置き、PR の説明を書く手順（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B と [runpr.md](../../../.claude/commands/runpr.md) の単独の変更の PR）はそこを指す。用語集はそれまで文書の翻訳と `japrose` の基準としてだけ使い、PR の説明には英語の直訳（cover を「覆う」とするなど）が残っていたためである。

## 採らなかった案

-   **文書の言語を、場所ごとの例外の列挙で決める。** 0048 より前の書き方である。原則から導けないので、新しい文書を足すたびに言語の判断が要る（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md) §1 と「issue #537 で決めたこと」）。
-   **手引きを英語版と日本語版の 2 つで持つ。** 手引きは人間がツールを使うための文書であり、同じ内容を英語でも持つと二重管理に戻る。エージェントは日本語の文書も読める（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」の「エージェントが読む手引き」）。
-   **それまでの英語の README を、英語版としてそのまま残す。** 最初から、英語版が正本の翻訳になるようにするためである（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」の「英語版の作り方」）。
-   **`/mktrans` の同期点を、出力先のファイルを最後に変えたコミットにする。** `README.md` に未反映の変更があるときに `README.en.md` だけを直すコミットができると、同期点が先へ進み、未反映の変更が差分から落ちる（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-10-28）」）。
-   **同期が要るかを、コマンドの手順の判定で確かめる。** `/runpr` の手順に印の検査を置いていた。プロンプトの判定は決定的でなく、手作業の変更は手順を通らない（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）。
-   **PR のタイトルから conventional-commit の接頭辞を外す。** PR の一覧で、変更の種類とタスクが分からなくなる。接頭辞は、`/mergepr` が英語の件名の接頭辞を決める手がかりにもなる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
-   **squash の件名の接頭辞を、常に PR の材料から決める。** PR のタイトルの接頭辞は、PR を開いたコマンドかレビュアーが選んだものであり、捨てる理由がない（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
-   **PR のタイトルも、squash の件名の要約の情報源にする。** 要約が、タイトルをほぼ訳しただけの文になりやすい（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
-   **`推奨タイトル` を英語の件名の下書きとして残す、または日本語のタイトルと英語の件名の両方の欄を持つ。** プランは実装の前に書くので、件名の下書きは実際の内容とずれうる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
-   **PR のタイトルと squash の件名の形を、すべて CLAUDE.md に書く。** CLAUDE.md は毎回のセッションで読み込まれるので、接頭辞の文法のような、コマンドだけが使う細部を置く場所に向かない（[0032 設計書](../../tasks/0032_japanese_pr_title/02_architecture.md)「1.2. 設計の際に決めたこと（2026-10-21）」）。

-   **`_checks/` のスクリプトのリテラルの例外を、guard のテストと同じ 4 種類（見出し・表のセル・行の始めの項目の名前・太字の印）に限る。** 確かめに要る語・文の断片・文を置く `data.json` が残る（[0080 要件定義書](../../tasks/0080_diff_check_proportion/01_requirements.md)「要件定義の際に決めたこと（2026-11-10）」）。
-   **`_checks/` のスクリプトで見出しを構造で見つけることを既定にし、例外を広げない。** 構造で見つけられるのは節の位置だけで、語や文そのものを確かめる場合には足りない（同）。
-   **Opus のレビューの判断のスクリプトには例外を広げず、計画書の見出しと欄名を、それを引用する英語のコマンドの定義から引数で渡す。** 写し誤りがあると値を見つけられず、値がないものとして Opus のレビューを行わない側に倒れる（[0081 実装計画書](../../tasks/0081_implementation_model_choice/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-11-11）」）。

## 結果

-   新しい文書の言語は、その文書の読み手と承認者から決まる。新しい種類の文書が原則の当てはめに挙がっていないときは、原則から言語を選ぶ。
-   `README.md` を変えるコミットは、同じコミットで `README.en.md` を同期する。pre-commit のフックと `make test` は、印が `README.md` を指していないコミットを断る。pre-commit のフックは飛ばせるので、印の検査の確実な関門は CI である（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md) §2.4）。
-   `TestReadmeSyncMarker` が確かめるのは、印と `README.md` の対応だけである。`README.en.md` の本文が翻訳されているかは確かめない。印だけを書き換えた変更は、2 つの版の構造の一致のテストと PR のレビューで見つける（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md) §2.4）。
-   作業ツリーのバイトがブロブのバイトと同じになるように、両方の README の改行を `.gitattributes` で LF に固定する。印の ID は SHA-1 の 40 文字だけを受け入れる。リポジトリのオブジェクトの形式が変わったら、テストを改める（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）。
-   開発者向けの文書（`docs/dev/guides/cli.md` など）は日本語だけなので、CLI の英語のヘルプが日本語の文書を指す。CLI は補助の経路であり、読み手は日本語の話者の開発者である（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」）。
-   squash の件名は PR の材料から書くので、PR のタイトルと違う文になりうる。件名は `/mergepr` の承認の質問で利用者が確かめる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md) §4.4 と「要件定義の際に決めたこと（2026-10-21）」）。
-   0032 より前のプランに残る英語の `推奨タイトル` は書き換えない。`/runplan` は、英語の `推奨タイトル` からも日本語の PR のタイトルを書く（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
-   ソースに日本語のリテラルが要るときは、「Go Idioms」の例外に当たるかを確かめる。当たらない場合、0061 は、範囲を狭く限った例外を CLAUDE.md の「Go Idioms」に足した（[0061 設計書](../../tasks/0061_lightweight_process/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-04）」）。この例外は、0075 で軽量プロセスとともに除くことにした（[0075 要件定義書](../../tasks/0075_remove_lightweight_process/01_requirements.md)）。

## 経緯

- 2026-10-21 PR のタイトルを日本語にし、conventional-commit の接頭辞は英語のまま残した。squash のコミットの件名と本文は英語のままとし、`/mergepr` は PR のタイトルをそのまま件名に使わず、PR の材料から件名を書く（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「issue #408 で決めたこと」と「要件定義の際に決めたこと（2026-10-21）」）
- 2026-10-21 PR のタイトルと squash の件名の形を `_context.md` の 1 つの節に置き、CLAUDE.md の「Language」には言語の規則だけを書いた（[0032 設計書](../../tasks/0032_japanese_pr_title/02_architecture.md)「1.2. 設計の際に決めたこと（2026-10-21）」）
- 2026-10-26 本番の構成の文書を加えるときも、当時の文書の言語の規則（`docs/` は日本語、README は英語）を保った（[0040 要件定義書](../../tasks/0040_production_profile/01_requirements.md)「issue #488 で決めたこと」）
- 2026-10-27 拡張のソースの言語の規則を改め、利用者に表示する文言は言語ごとの表に置いて日本語も書くことにした。識別子とコメントは英語のままとした（[0043 要件定義書](../../tasks/0043_extension_i18n/01_requirements.md)「issue #506 で決めたこと」）
- 2026-10-28 文書の言語を、場所ごとの例外の列挙から、読み手と承認者で決める 3 つの原則に改めた。手引きを日本語に一本化し、コマンドの定義が日本語の手引きをエージェントに読ませることを原則 (b) の例外とした。README を日本語の正本と英語版の 2 言語にし、`README.md` を先に変えて `/mktrans` で反映することにした。guard のテストの日本語のリテラルの例外を、Go のテストにも当てはめた（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「issue #537 で決めたこと」と「要件定義の際に決めたこと（2026-10-28）」）
- 2026-10-28 `/mktrans` の同期点を、`README.en.md` の 1 行目に置く、翻訳の元の `README.md` のブロブの ID にした（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-10-28）」と §2.4）
- 2026-10-28 同期の印の検査を Go のテスト `TestReadmeSyncMarker` にし、`make test`・CI の `doc-guard`・pre-commit のフックで走らせることにした。`/runpr` の手順の印の検査はなくした。guard のテストのリテラルの例外は、Go の guard のテストと、そのロケータを持つ `internal/doctestutil` に及ぶ形で書いた（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）
- 2026-10-31 文書のテストを整理したとき、`TestReadmeSyncMarker` を README の同期の仕組みそのものとして残した（[0052 要件定義書](../../tasks/0052_doc_test_pruning/01_requirements.md)「issue #591 で決めたこと」）
- 2026-11-04 CLAUDE.md の「Document language」の原則 (a) に、ADR と索引を置く `docs/dev/adr/` を足した（[0060 設計書](../../tasks/0060_adr_records/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-04）」）
- 2026-11-04 軽量プロセスの確かめ直しのスクリプトとそのテストに、仕様の見出しの日本語のリテラルを許す範囲の狭い例外を、CLAUDE.md の「Go Idioms」に足した（[0061 設計書](../../tasks/0061_lightweight_process/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-04）」）
- 2026-11-09 軽量プロセスを削除することにし、その確かめ直しのスクリプトとテストのための日本語のリテラルの例外（lightweight-check literal exception）を、CLAUDE.md の「Go Idioms」から除くことにし、「決定」から [ADR-0012](0012-lightweight-check-in-bash.md) への参照も除いた（[0075 要件定義書](../../tasks/0075_remove_lightweight_process/01_requirements.md)「要件定義の際に決めたこと（2026-11-09）」）
- 2026-11-10 guard のテストの日本語のリテラルの例外（guard-test literal exception）を、言語にかかわらずタスクの `_checks/` のスクリプトに広げ、そこでは日本語の文書の文字列と一致しなければならないリテラルを種類を限らずに認めることにした。英語だけを求めるコマンドの定義の規則は、例外に当たるリテラルを指摘しない。日本語の文字列を `data.json` に逃がす仕組みを要らなくするためである（[0080 要件定義書](../../tasks/0080_diff_check_proportion/01_requirements.md)「要件定義の際に決めたこと（2026-11-10）」）
- 2026-11-11 guard のテストの日本語のリテラルの例外（guard-test literal exception）を、Opus のレビューの判断のスクリプト `.claude/commands/_lib/opus-review-decision.sh` に広げた。範囲は、実装計画書の見出し・欄名・値の行の書き出しと一致しなければならないリテラルに限る。計画書は日本語の文書であり、見出しを英語の文書から渡すと、写し誤りで Opus のレビューを行わない側に倒れるためである（[0081 実装計画書](../../tasks/0081_implementation_model_choice/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-11-11）」）
- 2026-11-11 PR のタイトルと説明の日本語を、用語集の語で書くことにした。規則は `_context.md` の「Translation glossary」に置き、PR の説明を書く手順からそこを指す（[PR #825](https://github.com/isseis/yt2column/pull/825)）
