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

文書・会話・コード・GitHub 投稿の言語は、誰が読み誰が承認するかの原則で決め、日英の 2 ファイル対で機械同期する文書は README だけとし、PR のタイトルは日本語・squash の件名と本文は英語にする。

### 1. 文書・会話・コードの言語原則

- **文書の言語の 3 原則:** 置き場所ではなく、誰が読み誰が承認するかで決める。原則 (a) として、人間が判断・承認する文書と人間向けの手引きは日本語にする。原則 (b) として、エージェントの実行を制御する指示（`.claude/commands/` と `.agents/skills/` を含む。[ADR-0055](0055-harness-independent-commands.md)）とコードは英語にする。原則 (c) として、利用者向けの文書（README と `INSTALL.md`）は日本語を正本として英語版を同期する（`INSTALL.md` の期限付きの例外は下の項目）。
  - **理由:** 置き場所ごとの例外の列挙では原則から導けず、新しい文書を足すたびに判断が要るためである。
  - **規則:** [CLAUDE.md](../../../CLAUDE.md) の「Document language」に従う。
- **会話・リポジトリ・プロンプト・GitHub の言語:** 対話型エージェント（Claude Code・Antigravity CLI 等）のセッションの会話と GitHub に置くものは日本語、コード・コミットのメッセージ・プロンプトは英語とする。
  - **理由:** 元々の会話の言語を選んだ理由は記録されていない。特定のハーネス名への限定を外したのは、複数のハーネスで同じ対話運用を行うためである（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md) の F-008）。
  - **規則:** [CLAUDE.md](../../../CLAUDE.md) の「Language」に従う。
- **ソースの文字列リテラルの英語原則と例外:** ソースのコメント・識別子・文字列リテラルは英語とし、日本語のリテラルを置いてよい場所は例外として範囲を限る（`.claude/commands/_lib/` の副作用のない 2 つの記録判定シェルスクリプトを含む）。現在、例外に当たらない日本語の文字列リテラルも製品のコードに 2 か所ある（記事本文に出力する出典ブロックの見出しと、YouTube の文字起こしパネルの文字列と照合する字幕パーサの単位・操作ラベル）。どちらもコメントで日本語にする理由を述べている。このずれは issue [#726](https://github.com/isseis/yt2column/issues/726) で扱う。拡張の UI を 2 言語にすることは [ADR-0029](0029-extension-ui-languages.md) が定める。
  - **理由:** エージェントの実行を制御する指示とコードは英語で書くという原則による。
  - **規則:** 例外の一覧と範囲・理由・境界は [CLAUDE.md](../../../CLAUDE.md) の「Go Idioms」に従う。

### 2. 利用者向け文書の分割と同期

- **README の 2 言語同期:** 日英の 2 ファイル対で機械同期する文書は README だけである（Chrome Web Store 掲載文の扱いは下の項目）。`README.md` を先に変え、[mktrans.md](../../../.claude/commands/mktrans.md) の `/mktrans` で `README.en.md` に反映し、両方を 1 つのコミットに入れる。`README.en.md` の 1 行目の同期の印は、翻訳の元にした `README.md` の版を指し、`/mktrans` はその版からの差分を翻訳する。
  - **理由:** 2 つの言語で持つ文書は、片方を変えたときにもう片方が古くなりやすく、同期をエージェントの判断に任せると、手作業の変更や手順を通らない変更で同期が抜けるためである。
  - **規則:** 印の形と翻訳の手順は [mktrans.md](../../../.claude/commands/mktrans.md) に、同期の規則は [CLAUDE.md](../../../CLAUDE.md) の「Translation Guidelines (Japanese to English)」に従う。
- **同期の印の決定的な検査:** 同期の印は、エージェントの判断ではなく Go のテスト `TestReadmeSyncMarker` で確かめる。テストは、印が今の `README.md` を指すことを確かめ、`make test`・CI の `doc-guard` のジョブ・pre-commit のフックで走る。同期の手順を持つコマンドは、この検査が失敗したときに `/mktrans` を実行する。
  - **理由:** プロンプトの判定は決定的でなく、手作業の変更は手順を通らないためである。
  - **規則:** [CLAUDE.md](../../../CLAUDE.md) の「Translation Guidelines (Japanese to English)」に従う。
- **利用者向け文書の分割:** README を概要と案内に、ルートの `INSTALL.md` をインストールと利用の手順に分け、開発者向けの内容は `docs/dev/guides/` の手引きに置く。
  - **理由:** README を、初めて使う利用者が読める文書にするためである。開発者向けの手引きは日本語だけの文書なので、英語版に訳す量も減る。
  - **規則:** [0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md) の F-001 にある。
- **`INSTALL.md` の配置と期限付きの例外:** `INSTALL.md` は、README と同じくリポジトリのルートに置き、Chrome Web Store に申請するまで日本語だけで持つ。英語版を足す作業と、`/mktrans` と同期の検査の一般化は [#1044](https://github.com/isseis/yt2column/issues/1044) で扱う。
  - **理由:** ルートに置くのは、利用者が README の隣で見つけられ、Releases や zip からも指せるためである。日本語だけで持つことは原則 (c) の期限付きの例外であり、zip で配る初期の利用者は日本語の話者に限られる一方、Web Store に載せると日本語を読めない利用者が来るためである。
  - **規則:** [0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md) の「要件定義の際に決めたこと（2026-11-19）」にある。
- **Chrome Web Store 掲載文の保管と同期対象外の扱い:** Chrome Web Store 詳細説明文（日本語・英語）は `docs/store/chrome_web_store.md` の 1 ファイルに保管し、`/mktrans` と `TestReadmeSyncMarker` による 2 言語同期の対象外とする。現在 `docs/store/chrome_web_store.md` はまだなく、`CLAUDE.md` の「Document language」にもこの位置づけが未反映であり、[#1148](https://github.com/isseis/yt2column/issues/1148) の実装 PR でファイルを作成し `CLAUDE.md` に追記する。
  - **理由:** ストア掲載文は日本語版・英語版の確定稿をストアへ貼り付ける用途であり、`README.md` のような継続的な差分翻訳の仕組みを広げる必要がないためである。
  - **規則:** [0125 要件定義書](../../tasks/0125_product_name/01_requirements.md) の「要件定義の際に決めたこと（2026-11-25）」と [0125 設計書](../../tasks/0125_product_name/02_architecture.md) の「1.2. 設計の際に決めたこと（2026-11-25）」にある。

### 3. GitHub 上のテキストと用語統一

- **PR のタイトルと squash コミットの言語分離:** PR のタイトルは日本語で書き、squash のコミットの件名と本文は英語で書く。PR のタイトルは conventional-commit の接頭辞を ASCII のまま残し、その後の要約を日本語で書く。squash の件名は、`/mergepr` が PR の材料から英語で書き、PR のタイトルからは接頭辞だけを読む。
  - **理由:** squash のコミットは `main` に残る唯一のコミットであり、リポジトリに残るものは英語で書くためである。
  - **規則:** [_context.md](../../../.claude/commands/_context.md) の「PR title and squash subject」の 1 か所に定める。
- **用語集による日本語用語の統一:** PR のタイトルと説明の日本語は、用語集 [docs/translation_glossary.md](../../translation_glossary.md) の語で書く。コマンドは書く前に用語集を読み、旧称と、用語集が使わないとする訳を使わない。用語集にない語が要るときは、勝手に作らず、最終報告で挙げて利用者に追加を委ねる。
  - **理由:** 用語集はそれまで文書の翻訳と `japrose` の基準としてだけ使い、PR の説明には英語の直訳（cover を「覆う」とするなど）が残っていたためである。
  - **規則:** [_context.md](../../../.claude/commands/_context.md) の「Translation glossary」の 1 か所に置き、PR の説明を書く手順（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B と [runpr.md](../../../.claude/commands/runpr.md) の単独の変更の PR）はそこを指す。

## 採らなかった案

以下は、文書の言語原則、2 言語同期の仕組み、PR タイトルと squash 件名、日本語リテラルの例外、および利用者向け文書・ストア掲載文の配置について比べて採らなかった案と理由である。

- **場所ごとの例外の列挙で文書の言語を決める案:** 0048 より前の書き方である。原則から導けないので、新しい文書を足すたびに言語の判断が要る（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md) §1 と「issue #537 で決めたこと」）。
- **手引きを英語版と日本語版の 2 つで持つ案:** 手引きは人間がツールを使うための文書であり、同じ内容を英語でも持つと二重管理に戻る。エージェントは日本語の文書も読める（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」の「エージェントが読む手引き」）。
- **それまでの英語の README を英語版としてそのまま残す案:** 最初から日本語の正本を訳した英語版にするため、既存の英語 README は残さなかった（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」の「英語版の作り方」）。
- **`/mktrans` の同期点を出力先ファイルの最終変更コミットにする案:** `README.md` に未反映の変更があるときに `README.en.md` だけを直すコミットができると、同期点が先へ進み、未反映の変更が差分から落ちる（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-10-28）」）。
- **同期が要るかをコマンド手順の判定で確かめる案:** `/runpr` の手順に印の検査を置いていた。プロンプトの判定は決定的でなく、手作業の変更は手順を通らない（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）。
- **PR のタイトルから conventional-commit の接頭辞を外す案:** PR の一覧で、変更の種類とタスクが分からなくなる。接頭辞は、`/mergepr` が英語の件名の接頭辞を決める手がかりにもなる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
- **squash の件名の接頭辞を常に PR の材料から決める案:** PR のタイトルの接頭辞は、PR を開いたコマンドかレビュアーが選んだものであり、捨てる理由がない（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
- **PR のタイトルも squash の件名の要約の情報源にする案:** 要約が、タイトルをほぼ訳しただけの文になりやすい（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
- **`推奨タイトル` を英語件名の下書きとして残すか日英両方の欄を持つ案:** プランは実装の前に書くので、件名の下書きは実際の内容とずれうる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md)「要件定義の際に決めたこと（2026-10-21）」）。
- **PR のタイトルと squash の件名の形をすべて `CLAUDE.md` に書く案:** `CLAUDE.md` は毎回のセッションで読み込まれるので、接頭辞の文法のような、コマンドだけが使う細部を置く場所に向かない（[0032 設計書](../../tasks/0032_japanese_pr_title/02_architecture.md)「1.2. 設計の際に決めたこと（2026-10-21）」）。
- **タスクの `_checks/` のスクリプトのリテラルの例外を 4 種類に限る案:** 例外を guard のテストと同じ 4 種類（見出し・表のセル・行の始めの項目の名前・太字の印）に限ると、確かめに要る語・文の断片・文を別ファイル `data.json` に逃がす仕組みが残る（[0080 要件定義書](../../tasks/0080_diff_check_proportion/01_requirements.md)「要件定義の際に決めたこと（2026-11-10）」）。
- **タスクの `_checks/` のスクリプトで見出しを構造で見つけることを既定にして例外を広げない案:** 構造で見つけられるのは節の位置だけで、語や文そのものを確かめる場合には足りない（同）。
- **上位モデルのレビュー判定スクリプトに例外を広げず、計画書の見出しと欄名をそれを引用する英語のコマンドの定義から引数で渡す案:** 写し誤りがあると値を見つけられず、値がないものとして上位モデルのレビューを行わない側に倒れる（[0081 実装計画書](../../tasks/0081_implementation_model_choice/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-11-11）」）。
- **`INSTALL.md` と `INSTALL.en.md` を対で置き `/mktrans` と同期検査を一般化する案:** 初期の利用者は日本語の話者に限られ、一般化の費用を今は払わないためである。英語版を足すときまで一般化を先送りした（[0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
- **インストールの手順を README と分けず README の中で構成を整える案:** 画面の手順を含む長い説明で README の概要が埋もれ、Releases や zip から手順だけを指せないためである（[0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
- **Chrome Web Store 詳細説明文をリポジトリに置かず issue コメントにのみ残す案:** 申請時に参照する確定稿が issue コメントに散逸し、版管理できないためである（[0125 要件定義書](../../tasks/0125_product_name/01_requirements.md)「要件定義の際に決めたこと（2026-11-25）」）。
- **Chrome Web Store 詳細説明文を `docs/store/description_ja.txt` と `description_en.txt` の 2 ファイルに分ける案:** コピーはしやすいが、表示名や商標条件の注記を別ファイルで管理する必要がある（[0125 設計書](../../tasks/0125_product_name/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-25）」）。

## 結果

本決定により、文書の言語と同期の責務が一貫する一方、次の運用条件と制約が生じる。

- **新規文書の言語決定:** 新しい文書の言語は、その文書の読み手と承認者から決まる。新しい種類の文書が原則の当てはめに挙がっていないときは、原則から言語を選ぶ。
- **README 同期の強制:** `README.md` を変更するときは、同じコミットで `README.en.md` も同期する。同期の印が `README.md` を指していない場合、pre-commit のフックはコミットを拒否し、`make test` は失敗する。pre-commit のフックは飛ばせるので、印の検査の確実な関門は CI である（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md) §2.4）。
- **同期検査の限界:** `TestReadmeSyncMarker` が確かめるのは、印と `README.md` の対応だけである。`README.en.md` の本文が翻訳されているかは確かめない。印だけを書き換えた変更は、2 つの版の構造の一致のテストと PR のレビューで見つける（[0048 設計書](../../tasks/0048_doc_language_policy/02_architecture.md) §2.4）。
- **改行コードとオブジェクト形式:** 作業ツリーのバイトがブロブのバイトと同じになるように、両方の README の改行を `.gitattributes` で LF に固定する。印の ID は SHA-1 の 40 文字だけを受け入れる。リポジトリのオブジェクトの形式が変わったら、テストを改める（[0048 実装計画書](../../tasks/0048_doc_language_policy/03_implementation_plan.md)「1.3. 計画の際に決めたこと（2026-10-28）」）。
- **開発者向け文書への参照:** 開発者向けの文書（`docs/dev/guides/cli.md` など）は日本語だけなので、CLI の英語のヘルプが日本語の文書を指す。CLI は補助の経路であり、読み手は日本語の話者の開発者である（[0048 要件定義書](../../tasks/0048_doc_language_policy/01_requirements.md)「要件定義の際に決めたこと（2026-10-28）」）。
- **squash 件名と旧プランの互換:** squash の件名は PR の材料から書くので、PR のタイトルと違う文になりうる。件名は `/mergepr` の承認の質問で利用者が確かめる（[0032 要件定義書](../../tasks/0032_japanese_pr_title/01_requirements.md) §4.4 と「要件定義の際に決めたこと（2026-10-21）」）。0032 より前のプランに残る英語の `推奨タイトル` は書き換えない。`/runplan` は、英語の `推奨タイトル` からも日本語の PR のタイトルを書く（同）。
- **ソース内の日本語リテラル例外:** ソースに日本語のリテラルが要るときは、「Go Idioms」の例外に当たるかを確かめる。当たらない場合の先例として、0061 では範囲を狭く限った例外を `CLAUDE.md` の「Go Idioms」に足した（[0061 設計書](../../tasks/0061_lightweight_process/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-04）」）。この例外は、0075 で軽量プロセスとともに除くことにした（[0075 要件定義書](../../tasks/0075_remove_lightweight_process/01_requirements.md)）。
- **README の利用者向け限定と `INSTALL.md` の英語版不在:** README に開発者向けの内容を足さない。CLI の設定や料金の推定の規則などは、`docs/dev/guides/` の手引きに書く（[0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。`INSTALL.md` に英語版がない間は、英語を読む利用者はインストールの手順を英語で読めない。`README.en.md` は、手順が日本語でしか書かれていないことを示して `INSTALL.md` へリンクする（同）。
- **Chrome Web Store 掲載文の手動整合:** `docs/store/chrome_web_store.md` の日英両テキストは `/mktrans` と `TestReadmeSyncMarker` で機械同期しない。内容を改めるときは日本語版と英語版をあわせて更新し、PR のレビューで確認する。

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
- 2026-11-17 guard のテストの日本語のリテラルの例外を、ローカルの CLI レビューが見たかと Fable の記録が有効かを決める判定のスクリプト `.claude/commands/_lib/review-records.sh` にも広げることにした。範囲は、実装計画書の PR の進み具合のチェックボックスの行と PR マーカーの見出しと一致しなければならないリテラルに限る。`CLAUDE.md` の例外の文は、スクリプトを入れる実装の PR で変える（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 22）
- 2026-11-18 chief の道具が、状況の表の決まった語（見出し・凡例・状態の文字列・文書の段など）を日本語で出力してよい例外を、CLAUDE.md の「Go Idioms」に足すことにした。これまでの例外は日本語の文書の文字列と一致しなければならないリテラルだったが、この例外は道具が人に示す語そのものである。範囲は表の決まった語に限り、道具の出力のほかの値、エラーのメッセージ、ほかのサブコマンドの出力は英語のままにする（[0102 要件定義書](../../tasks/0102_chief_text_table/01_requirements.md)「要件定義の際に決めたこと（2026-11-18）」）
- 2026-11-19 利用者向けの文書を、概要と案内の README と、インストールと利用の手順の `INSTALL.md` に分け、開発者向けの内容を `docs/dev/guides/` の手引きに移すことにした。`INSTALL.md` は Chrome Web Store に申請するまで日本語だけで持ち、原則 (c) の期限付きの例外とした。`CLAUDE.md` の「Document language」と「Documents」の文は、実装の PR で変える（[0116 要件定義書](../../tasks/0116_user_readme/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）
- 2026-11-21 エージェントの実行を制御する英語の指示（原則 (b)）の対象に `.agents/skills/` を加え、セッションの会話の言語規則（日本語）を Claude Code 限定からハーネス非依存の記述へ改めた。あわせて、シェルスクリプトの日本語リテラルの例外で対象とするスクリプト名も一般化後の名前に追随させることにした（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」、[ADR-0055](0055-harness-independent-commands.md)）
- 2026-11-25 Chrome Web Store 詳細説明文（日英）を `docs/store/chrome_web_store.md` に保管し、`/mktrans` による 2 言語同期の対象外と位置づけた。`CLAUDE.md` の「Document language」の記述は実装の PR で追記する（[0125 要件定義書](../../tasks/0125_product_name/01_requirements.md)「要件定義の際に決めたこと（2026-11-25）」、[0125 設計書](../../tasks/0125_product_name/02_architecture.md)「1.2. 設計の際に決めたこと（2026-11-25）」）
