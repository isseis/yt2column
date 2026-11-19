# 設計への引き継ぎ：利用者向けの README と INSTALL 文書

`02_architecture.md` が扱う項目。どれも要件定義のときに出た、設計で決める論点である（[01_requirements.md](01_requirements.md)）。

## H-01: README を読むテストの読む先を、値が移った先の文書に変える

**懸念:** 今は、次のテストが README の 2 つの版から値を読んでいる。
- 拡張のテスト：`extension/test/metadataDocs.test.ts`（`## ブラウザ拡張` の節から、概要欄のメッセージ・工程の名前・画面の名前）と `manifest.test.ts`（最低の Chrome の版）。読む先は `extension/test/helpers/readmeVersions.ts` で決めている。
- Go のテスト：`cmd/yt2column/docs_test.go`（`TestOpenAIDocsContract`、`TestDeepSeekDocsContract`、`TestLLMDefaultsDocsContract`、`TestEffortValuesDocsContract`、`TestProductionProfileDocsContract`、`TestCostDocsContract`、`TestReadmeLinks`）と `extension_docs_test.go`（拡張の ID）。

**新しい構成での変化:**
- 手順と画面の名前は INSTALL.md（日本語だけ）に移る。英語のカタログに照らす README.en.md の検査は、照らす先の文書がなくなる。
- 設定や料金の値は、日本語だけの開発者向けの手引きに移る。

**なぜ大事か:** 読む先を変えないとテストが失敗する。また、検査を外すと、値がずれても分からなくなる（AC-38）。

**案:** 値ごとに、それを述べる文書を設計書の表に書き、テストの読む先をその文書にする。どの文書も述べなくなる値（例：README.en.md の英語の画面の名前）は、検査を外すテストの削除として扱う。CLAUDE.md の「Testing Strategy」の「Deleting a test is a claim that must be checked」に従い、削除だけのコミットでカバレッジを比べる。#1044 で英語版を足すときに戻す検査を、#1044 に書き添えるかも決める。

**関連:** AC-08、AC-25、AC-33、AC-38、AC-39

## H-02: 移す内容と削除する内容を `diff` 検証で確かめる

**懸念:** AC-04 は、移した内容の差分を、見出しの階層・リンクの先・重複の削除の 3 つに限る。AC-05 は、移行の節がどの文書にも残らないことを求める。

**なぜ大事か:** 移すときに文を落とすか書き換えると、述べる挙動が黙って変わる。

**案:** 実装の PR で、マージベースとの差分を読むスクリプトを `_checks/` に置くか、差分を読んだ記録を残す（`requirements_process.md` §4「`diff` verification」）。どちらにするかと、重複として削除してよい文の判定の仕方を設計で決める。

**関連:** AC-04、AC-05

## H-03: ほかの文書から README の節へのリンクを付け替える

**懸念:** README の節をアンカー付きで指すリンクが、ほかの文書にある。
- `docs/dev/guides/cli.md:4`、`:7`、`:16`、`:58`、`:189`、`:234`
- `docs/dev/guides/development.md:99`
- `docs/dev/conventions/package_reference.md:103`

`TestReadmeLinks` は README と 2 つの手引きのリンクを確かめる。`package_reference.md` のリンクは、`TestInstructionDocPaths` がアンカーを落として確かめる。

**なぜ大事か:** 節が移ると、アンカーが行き先を失う。

**案:** 付け替える先を設計書の表に書く。`TestReadmeLinks` が INSTALL.md と、新しく置く手引きも見るようにするかを決める。

**関連:** AC-06

## H-04: `docs/dev/security.md` の参照先を INSTALL.md に合わせる

**懸念:** `docs/dev/security.md:131` と `:142` は、記述が「README に書く」としている。そのうちワークスペースを勧める記述は、INSTALL.md に移る。

**なぜ大事か:** 規則は変えないが、security.md の説明が実際の文書とずれる。

**案:** 移る記述ごとに、security.md の参照先を直す。ビューアの URL の注意を README と INSTALL.md のどちらに置くかを決める（プライバシーの節は README にある：AC-35）。

**関連:** 01_requirements.md §4.2、AC-11、AC-35

## H-05: 契約の記述で、クレジットの額を写すかどうか

**懸念:** 公式の情報には、Max（月 100 USD）、Ultra（月 200 USD）、Team（最大 500 USD）のクレジットの額がある（01_requirements.md の決定 6）。額は Anthropic が変えうる。

**なぜ大事か:** 写した額が古くなると、利用者が誤解する。

**案:** INSTALL.md には額を写さず、契約の名前と仕組みだけを書く。額は公式のページへのリンクで示し、確かめた日付を添える（AC-09）。額を写すなら、更新の方法を決める。

**関連:** AC-09

## H-06: CLAUDE.md の記述を合わせる箇所

**懸念:** 次の箇所を、新しい文書の構成に合わせて変える必要がある。
- 「Document language」の principle (c) と例外（INSTALL.md）。
- 「Development Notes」の、README を読むテストの記述（`make ext-test` が要る文書の一覧と、Go のガードの一覧）。
- 「Commands」の `/mktrans` の説明。

**なぜ大事か:** CLAUDE.md は毎回のセッションで読まれる。ずれると、エージェントが誤った文書を同期するか、検査を走らせない。

**案:** 変える文の一覧を設計書に書き、実装計画のどの PR で変えるかを決める。ADR-0035 の「決定」が CLAUDE.md を正本として指す文との整合もあわせて確かめる。

**関連:** AC-37、AC-39

## H-07: 未リリースの注記と #1039 の境界

**懸念:** 最初のリリースまで、README と INSTALL.md は「まだリリースしていない」ことを示す（AC-14、決定 5）。今の README の「状態」の注記（`README.md:47`）との関係と、#1039 が外す箇所を、一目で分かるようにしておく必要がある。

**なぜ大事か:** #1039 が外す箇所を取り違えると、未リリースの注記が残るか、手順が欠ける。

**案:** 注記を 1 か所（または README と INSTALL.md に 1 つずつ）にまとめ、#1039 が直す箇所を設計書に書く。来歴の検証の手順をどの節に足すかも、INSTALL.md の構成とあわせて決める。

**関連:** AC-14
