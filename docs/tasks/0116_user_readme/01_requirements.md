# 要件定義書：利用者向けの README と INSTALL 文書

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、拡張を初めて使う利用者が読む文書（README とインストールの手順）の内容と構成についての要件を定める。

対応 issue: [#1042](https://github.com/isseis/yt2column/issues/1042)（関連: [#1039](https://github.com/isseis/yt2column/issues/1039) zip で配布するリリースの手順。このタスクが #1039 の前提になる。[#1044](https://github.com/isseis/yt2column/issues/1044) INSTALL.md の英語版）

**背景:**
#1039 で、拡張機能を zip にして初期の利用者へ配布する予定である。しかし今の README は、開発者がリポジトリからビルドすることを前提に書かれている。
- クイックスタートの最初の手順が `make ext-install` と `make ext-build` である（`README.md:53-55`）。「必要なもの」には Go と Node.js が並ぶ（`README.md:67-68`）。
- CLI の設定（`README.md:292-366`）、推奨構成（`README.md:368-385`）、料金の推定の細かな規則（`README.md:95-118`）、以前の版からの移行（`README.md:270-290`）など、利用者に関係の薄い内容が多い。
- API キーをどの契約で使えるか、どこで作るか、費用の目安、利用の上限の設け方が書かれていない。

**本書が決めることと、設計に委ねること:**
- **本書が決めること:** 利用者向けの文書の構成（README、INSTALL.md（後述の決定 2）、開発者向けの手引きの分担）、各文書が述べる内容、文書の言語と同期の扱い、README を読む検査が引き続き値を確かめること。
- **`02_architecture.md` に委ねること:** 節の見出しと順序の細部、README を読むテスト（Go と拡張）の読む先の変え方、移す内容の確かめ方（`diff` 検証のスクリプトなど）、ほかの文書から README の節へのリンクの付け替え方。設計に渡す論点は [design_handoff.md](design_handoff.md) にある。

### issue #1042 で決めたこと

- README を利用者が読める文書に作り直し、どのような内容にするかの検討もこのタスクに含める。配布を始める前に、初めて使う利用者が読める文書が要るためである。
- 利用者向けの内容は、issue の「1. 利用者向けの内容（案）」の項目を元にする。これは何か、使う前に必要なもの、インストール、初期設定、使い方、更新、アンインストール、困ったとき、プライバシーである。
- README の日本語版を正本とし、英語版は `/mktrans` で同期する。CLAUDE.md の「Document language」の principle (c) による。
- Chrome Web Store の掲載文とプライバシーポリシー、拡張の画面の文言の変更は対象外とする。前者は Web Store に申請する issue で扱う。
- README を読む既存のテスト（`metadataDocs.test.ts`、`manifest.test.ts`、Go のガードテスト）と構成を合わせる。

### 要件定義の際に決めたこと（2026-11-19）

1. プロセスは `full` とする。
   - **理由:** 変更は README の 2 つの版にとどまらない。Go のガードテスト（`cmd/yt2column/docs_test.go`、`extension_docs_test.go`、`readme_structure_test.go`）、拡張のテスト（`extension/test/metadataDocs.test.ts`、`manifest.test.ts`）、開発者向けの手引き、CLAUDE.md、ADR-0035 に及ぶ。決めることも多い。
   - **除外の判定:** 除外に当たらない。`docs/dev/security.md` の規則と前提、開発ツールの信頼境界、外部への依存のどれも変えないためである。
   - **ADR:** なし。タスクの進め方の選択で、このタスクの中で閉じるため。
2. インストールと利用の手順は、README とは別の `INSTALL.md`（リポジトリのルート）に、当面は日本語だけで置く。
   - **理由:** README を、概要と案内（これは何か、どう動くか、どこを読めばよいか）に絞れる。手順が長くなっても、README の概要が埋もれない。GitHub Releases の本文や zip からも、手順を直接指せる。英語版を同期する仕組み（`/mktrans`、同期の検査）の一般化は、英語版を足すときまで先送りする。
   - **CLAUDE.md の方針との関係:** 次の 2 つの例外になる。
     - **言語:** CLAUDE.md の「Document language」の principle (c)（利用者向けの文書は日本語を正本とし、英語版を同期する）の例外である。例外は期限付きとする（決定 3）。
     - **置き場所:** CLAUDE.md の「Documents」の「文書は `docs` の下に置く。例外は `README.md` と `README.en.md`」の例外である。理由は、利用者が README の隣で見つけられ、Releases や zip からもリポジトリのルートの文書として指せることである。
   - **採らなかった案:**
     - INSTALL.md と INSTALL.en.md を対で置き、`/mktrans` と同期の検査を対ごとに動くよう一般化する案。初期の利用者は日本語の話者に限られ、一般化の費用を今は払わないためである。
     - INSTALL を分けず、README の中で構成を整える案。画面の手順を含む長い説明で README の概要が埋もれ、Releases や zip から手順だけを指せないためである。
   - **ADR:** [ADR-0035](../../dev/adr/0035-language-policy.md)（文書の言語の方針）を更新する。principle (c) の例外を定め、後のタスクを縛るため。
3. INSTALL.md の英語版は、Chrome Web Store に申請する前に足す。この作業を [#1044](https://github.com/isseis/yt2column/issues/1044) として記録した。それまで、README の 2 つの版は、INSTALL.md が今は日本語だけであることを示して INSTALL.md へリンクする（AC-07）。
   - **理由:** Web Store に載せると、日本語を読まない利用者が来る。zip で配る初期の利用者は、日本語の話者に限られる。
   - **ADR:** ADR-0035 を更新する（決定 2 と同じ項目）。
4. README にある開発者向けの内容は `docs/dev/guides/` の手引きに移し、README からはリンクで案内する。「以前の版からの移行」の節は、移さずに削除する。
   - **移す内容:** CLI の設定（環境変数）、推奨構成、料金の推定の細かな規則、生成の後の工程と出力の書式の詳細、ソースからのビルドと読み込み・更新、ログを見る方法。
   - **理由:** README を利用者向けの内容だけにするためである。移した先の手引きは日本語だけの文書（principle (a)）なので、英語版で訳す量も減る。以前の版（localhost サーバと Native Messaging）を使っていたのは開発者 1 人だけで（[ADR-0048](../../dev/adr/0048-extension-only-generation.md)）、移行はもう済んでいる。
   - **ADR:** ADR-0035 を更新する。「README は利用者向けの内容だけを持ち、開発者向けの内容は `docs/dev/guides/` に置く」と決め、後のタスクを縛るため。あわせて、内容が移ることで決定の文が古くなる 2 つの ADR を更新する。[ADR-0029](../../dev/adr/0029-extension-ui-languages.md)（表示の言語を利用者に説明する場所が INSTALL.md に移る）と [ADR-0004](../../dev/adr/0004-production-profile.md)（推奨構成が手引きに移る。決定 7 も参照）である。[ADR-0030](../../dev/adr/0030-minimum-chrome-version.md) は変えない。必要なブラウザは、引き続き両方の README が書く（AC-01）。
5. #1039 との分担：このタスクは、zip の入手・展開・読み込み・更新・アンインストールの手順まで書く。#1039 は、来歴の検証（`gh attestation verify`）の手順を足し、未リリースの注記を外す。
   - **理由:** このタスクは #1039 より先にマージされるので、マージした時点では zip がまだない。一方、来歴の検証は、#1039 のワークフローが作る attestation がないと書けない。それ以外の手順は、zip の中のフォルダの構成によらずに書ける。AC-16 が、読み込むフォルダを「`manifest.json` のあるフォルダ」と示すからである。
   - **最初のリリースまでの扱い:** README と INSTALL.md は、まだリリースしていないことを示し、ソースからのビルドは開発者向けの手引きにあると案内する。
   - **ADR:** なし。タスクの間の作業の分け方で、このタスクと #1039 の中で閉じるため。
6. API キーを使える契約は、要件定義のときに Anthropic の公式の情報で確かめた（2026-11-19）。
   - **確かめた内容:**
     - Claude の有料の契約（claude.ai）と Claude API は別の製品で、API の利用には別に課金される（[Claude ヘルプセンター](https://support.claude.com/en/articles/9876003-i-have-a-paid-claude-subscription-pro-max-team-or-enterprise-plans-why-do-i-have-to-pay-separately-to-use-the-claude-api-and-console)）。
     - Max・Ultra・Team の契約には、毎月の Claude API のクレジットが付く。claude.ai の請求の設定から Claude Console の組織を結び付けると、その組織に届く。付いたクレジットは繰り越せず、購入したクレジットより先に使う。付いたクレジットを使い切ると、購入したクレジットがあればそれを使い、なければ API のリクエストは次のクレジットが届くまで止まる。契約の側には請求されない。新しく契約した人がクレジットを受け取れるのは契約から 7 日後で、1 つの契約に結び付けられる Console の組織は 1 つだけ（後から変えるにはサポートへの連絡が要る）、結び付けるには組織の Admin か Billing の役割が要る。額は Max が月 100 USD、Ultra が月 200 USD、Team が最大月 500 USD である。Free・Pro・Enterprise は対象外である（[API credits for Max, Ultra, and Team plans](https://platform.claude.com/docs/en/about-claude/api-credits-for-subscribers)）。
     - どの場合も、API キーは Claude Console で作る。ワークスペースごとに利用の上限（spend limit）を設けられる（同）。
   - **理由:** 書く内容がこの結果で変わるからである（issue の「要確認」）。
   - **ADR:** なし。文書に書く事実の確認で、決定ではないため。
7. 記事 1 本あたりの費用の目安として「約 0.4 USD」を示す。
   - **根拠:** [production_profile.md](../../dev/production_profile.md) の製品の制約「費用」（約 0.36 USD。0018 の評価記録 §4.8）。
   - **添える内容:** 目安は評価のときの値で、請求額ではない。実際の額は、ビューアが示す推定の料金と Claude Console で確かめる。
   - **値の更新:** production_profile.md の「本書を更新するタスク」が挙げる、本番の構成の値を繰り返している箇所に INSTALL.md の目安を加え、料金表の更新の手引き（`docs/dev/guides/pricing_update.md`）にも目安を見直す箇所として加える（AC-41）。
   - **理由:** 利用の上限を決めるときの手がかりになる。根拠の文書を名指ししておけば、値が古くなったときに直す元をたどれる。
   - **ADR:** [ADR-0004](../../dev/adr/0004-production-profile.md)（本番の構成）を更新する。ADR-0004 は、本番の構成の値を繰り返す箇所を README の推奨構成と定めており、決定 4 でそれが手引きに移り、INSTALL.md の目安も加わるため。
8. 手順に画面の画像は入れない。画面の名前（ボタンなど）は、今の README と同じく太字で引用する。
   - **理由:** UI は日本語と英語の 2 言語で、画面を変えるたびに画像を撮り直す負担が大きい。
   - **ADR:** なし。このタスクの中で閉じるため。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

- 拡張を初めて使う利用者が、Go や Node.js なしで、入手から最初の記事の保存まで進めること。
- 利用者が、API キーを使える契約、費用の目安、利用の上限の設け方を、使い始める前に知ること。
- 開発者向けの内容を利用者向けの文書から分け、それぞれの読み手が必要な文書だけを読めるようにすること。

### 2.2. スコープ (In Scope)

- `README.md` と `README.en.md` を、利用者向けの概要と案内に作り直す。
- ルートに `INSTALL.md`（日本語）を新しく置き、使う前に必要なものから困ったときまでの手順を書く。
- README にある開発者向けの内容を `docs/dev/guides/` の手引きに移し、「以前の版からの移行」の節を削除する。
- README の内容を確かめている既存のテストを、内容が移った先の文書に合わせる。
- 文書の言語の例外（INSTALL.md）を、ADR-0035 と CLAUDE.md に記録する。
- 移した節を指すほかの文書のリンクと名指しを、移った先に付け替える。
- 拡張のテストが読む文書の変更で、CI の拡張のジョブが走るようにする。
- 利用者向けの文書が新しく使う語を、用語集に足す。

### 2.3. スコープ外 (Out of Scope)

- **来歴の検証（`gh attestation verify`）の手順と、未リリースの注記を外すこと。** #1039 で扱う（決定 5）。
- **INSTALL.md の英語版と、`/mktrans` や同期の検査の一般化。** #1044 で扱う（決定 3）。
- **Chrome Web Store の掲載文とプライバシーポリシー。** Web Store に申請する issue で扱う（issue の「対象外」）。
- **拡張の画面の文言の変更。** issue の「対象外」による。
- **画面の画像。** 決定 8 による。
- **CLI の挙動の変更。** 開発者向けの内容は場所を移すだけで、述べる挙動は変えない。

## 3. 機能要件 (Functional Requirements)

この章で「利用者向けの文書」とは、`README.md`、`README.en.md`、`INSTALL.md` を指す。「開発者向けの手引き」とは、`docs/dev/guides/` の `cli.md` と `development.md`、またはそこに新しく置く手引きを指す。

### 3.1. 機能一覧

#### F-001: 文書の構成と分担

README は概要と案内、INSTALL.md は手順、開発者向けの手引きは開発者向けの内容を持つ。

**Acceptance Criteria**:
- **AC-01**: `README.md` は、次の内容を持つ。
  - これは何か（できること）
  - 仕組みの概要（ブラウザの中だけで動き、記事の生成のためのサーバもホストも要らないこと。API キーの送り先は `https://api.anthropic.com` だけであること）
  - プライバシー（F-009）
  - 必要なブラウザ（「Chrome 142 以降、または Chromium 142 以降の Brave」。AC-08 と同じ値）。両方の README が書くことは [ADR-0030](../../dev/adr/0030-minimum-chrome-version.md) の決定による
  - INSTALL.md への案内
  - 開発者向けの手引きへのリンク
- **AC-02**: `README.md` と `README.en.md` には、次のどれも含まれない。
  - 利用の前提として Go か Node.js を挙げる記述
  - `make` のターゲットを利用者の手順とする記述
  - CLI の環境変数の表と、その値の文法
  - 推奨構成の設定のブロック
  - DeepSeek と OpenAI の設定と料金の規則
  - 以前の版からの移行
  - 構成要素（`TranscriptSource`、`ArticleWriter`、`Publisher`）の図（`README.md:11-37`）
  - CLI と yt-dlp の説明と、CLI で使えるプロバイダの説明（`README.md:8-9`、`:39-44`、`:72-73`）

  ただし、開発者向けの CLI があることを 1 文で述べ、開発者向けの手引きへリンクしてよい。図と CLI の説明は、開発者向けの手引きにすでに同じ内容がなければ、そこへ移す（AC-04 が差分を限る規則に従う）。
- **AC-03**: ルートに `INSTALL.md` があり、日本語で書かれ、次の内容を順に持つ：使う前に必要なもの（F-002）、インストール（F-003）、初期設定（F-004）、使い方（F-005）、更新（F-006）、アンインストール（F-007）、困ったとき（F-008）。その後に、AC-24 の参考の節を置いてよい。
- **AC-04**: 今の README にある次の内容は、開発者向けの手引きにある。
  - CLI の設定（`README.md:292-366`）
  - 推奨構成（`README.md:368-385`）
  - 料金の推定の規則（`README.md:95-118`）
  - 生成の後の工程（`README.md:75-86`）と出力の書式（`README.md:88-93`）の詳細
  - ソースからのビルド、読み込み、更新、拡張の ID（`README.md:120-155`）
  - ログを見る方法（`README.md:266-268`）

  移した内容が述べる挙動は、今の README と変わらない。移した先の文と今の README の文の差分は、次の 4 つに限る。
  - 見出しの階層
  - リンクの先
  - 移した先の手引きにすでにある同じ内容を重ねて書かないための削除
  - 移した先の文書に適用される書式の規約（`docs/dev/conventions/document_formatting.md`）が求める、太字と項目の多い節の組み方の変更
- **AC-05**: 「以前の版からの移行」の節（`README.md:270-290`）は、どの文書にも移さずに削除する。クイックスタートからその節への案内（`README.md:61`）も削除する。
- **AC-06**: 利用者向けの文書と開発者向けの手引きにあるすべての相対リンクとアンカーは、存在するファイルと見出しを指す。ほかの文書が今の README の節を指す箇所は、リンクでも、リンクのない名指しでも、内容が移った先を指す。現時点で分かっている箇所は [design_handoff.md](design_handoff.md) の H-03 にある。
- **AC-07**: `README.md` と `README.en.md` は、インストールと使い方の手順が INSTALL.md にあること、INSTALL.md が今は日本語だけであることを示し、INSTALL.md へリンクする。2 つの版は同じ内容を述べる（`README.en.md` だけが述べる文は置かない）。

#### F-002: 使う前に必要なもの

INSTALL.md は、使い始める前にそろえるものと、それにかかる費用を示す。

**Acceptance Criteria**:
- **AC-08**: 必要なブラウザとして「Chrome 142 以降、または Chromium 142 以降の Brave」を示す。示す版は、`extension/static/manifest.json` の `minimum_chrome_version` と一致する。
- **AC-09**: API キーを使える契約について、次を示す。
  - Claude API は claude.ai の契約とは別の製品で、API の利用は API のクレジットで支払うこと。
  - Max・Ultra・Team の契約には、毎月の API のクレジットが付くこと。claude.ai の請求の設定から Claude Console の組織を結び付けると使えること。結び付けには決定 6 の条件（契約から 7 日後、組織は 1 つだけで後から変えにくい、組織の Admin か Billing の役割）があること。
  - 付いたクレジットを使い切ると、購入したクレジットがあればそれを使い、なければ次のクレジットが届くまで API が使えないこと。
  - Free・Pro・Enterprise の契約には、API の利用分が含まれないこと。
  - どの場合も、API キーは Claude Console で作ること。

  この記述には、決定 6 の公式の出典へのリンクと、確かめた日付を添える。
- **AC-10**: Claude Console でアカウントを作り、支払い方法を登録して（または、決定 6 の契約のクレジットを結び付けて）、API キーを作るまでの手順を示す。
- **AC-11**: 月ごとの利用の上限（spend limit）を設けた、この拡張のためだけのワークスペースを作り、そこで API キーを作ることを勧める。その設け方を示す。そのワークスペースで作った API キーなら、ワークスペース ID の入力が要らないことも示す。
- **AC-12**: 記事 1 本あたりの費用の目安として「約 0.4 USD」を示し、[production_profile.md](../../dev/production_profile.md) を根拠として名指しする。あわせて、次を示す。
  - 目安は評価に使った動画での値で、失敗とやり直しの分を含まないこと（`docs/dev/production_profile.md:67`）。額は字幕の長さで変わること。
  - 目安は請求額ではないこと。
  - 実際の額は、ビューアの推定の料金と Claude Console で確かめること。
- **AC-13**: ゼロデータ保持（ZDR）か Compliance API を有効にした組織の API キーでは、この拡張が使えないことと、その理由（ブラウザからの API の呼び出しを受け付けないこと）を示す。今の記述は `README.md:70` にある。

#### F-003: インストール

INSTALL.md は、配布された zip から拡張を読み込むまでの手順を示す。

**Acceptance Criteria**:
- **AC-14**: zip を GitHub Releases から入手することを示し、リポジトリの Releases のページへリンクする。最初のリリースまでは、まだリリースしていないことを示し、ソースからのビルドの手順が開発者向けの手引きにあることを案内する（決定 5）。
- **AC-15**: zip の中身は、この拡張のためだけに新しく作った空のフォルダに置くこと、そのフォルダは後で消したり動かしたりしない場所に作ることを示す。理由として、フォルダを消すか動かすと拡張が動かなくなること、更新のときにフォルダの中身をすべて消すこと（AC-28）を示す。
- **AC-16**: 次の順の手順を示す。
  1. `chrome://extensions/`（Brave: `brave://extensions/`）を開く。
  2. **デベロッパー モード**（**Developer mode**）をオンにする。
  3. **パッケージ化されていない拡張機能を読み込む**（**Load unpacked**）で、展開したフォルダのうち `manifest.json` のあるフォルダを選ぶ。
- **AC-17**: 拡張をツールバーにピン留めする手順を示す。
- **AC-18**: ストアの外から読み込んだ拡張について、ブラウザが警告を示すことがあること、それはこの読み込み方のために出る想定どおりのものであることを示す。警告の文言を引用するときは、実装のときに Chrome と Brave で確かめた文言だけを、確かめた日付とともに引用する。
- **AC-19**: 拡張はシークレットモードでは有効にできないことを示す（今の記述は `README.md:155`）。

#### F-004: 初期設定

INSTALL.md は、API キーを保存する手順と、保存したときの結果の読み方を示す。

**Acceptance Criteria**:
- **AC-20**: オプションページの開き方を 2 つ示す。ポップアップの **設定** から開く方法と、拡張の **詳細** の **拡張機能のオプション** から開く方法である。
- **AC-21**: API キーの欄とワークスペース ID の欄の意味と、ワークスペース ID を省ける場合と省けない場合を示す（今の記述は `README.md:166-167`）。
- **AC-22**: **保存する** を押したときの 6 つの結果と、それぞれへの対処を示す（今の記述は `README.md:169-176`）。**削除する** の働きも示す（`README.md:178`）。

#### F-005: 使い方

INSTALL.md は、字幕の選択から記事の保存とコピーまでの使い方を示す。

**Acceptance Criteria**:
- **AC-23**: 使い方の流れを、次の順に示す。
  1. 動画の概要欄（「もっと見る」）と文字起こしパネルを開き、字幕の行を選択する。
  2. ツールバーのアイコンからポップアップを開き、**記事を生成する** を押す。
  3. 背面に開くビューアのタブで記事を読む。
  4. **.md で保存する** か **Markdown をコピーする** を選ぶ。
- **AC-24**: 今の README の「ポップアップ」と「ビューア」の節（`README.md:182-264`）が述べる、利用者に見える挙動を、すべて INSTALL.md で述べる。言い回しは書き直してよい。挙動とは、画面の状態と操作、記録の残り方、選択範囲の扱いと上限である。初めて読む利用者の妨げにならないよう、細部（記録の件数と大きさの上限の値、ビューアの URL が持つ値、同じ動画の照会の限界）は、INSTALL.md の末尾の参考の節に置いてよい。ログを見る方法は、開発者向けの手引きに移す（AC-04）。
- **AC-25**: 次の 3 つを、拡張の日本語のメッセージのカタログと同じ文で述べる。
  - 概要欄が折りたたまれているときにポップアップが示すメッセージとその手順
  - 記事の生成の 3 つの工程の名前（「下書きの生成」「字数の調整」「検証・推敲」）
  - ポップアップとビューアの画面の名前（太字で引用する）
- **AC-26**: 生成の後の工程のうち、利用者に見える次の 2 点を示す。
  - 本文が長いときに字数を調整すること。
  - それでも 4,000 文字を超えると、記事はできずに失敗すること（今の記述は `README.md:79-80`）。
- **AC-27**: 画面の文言は、ブラウザの UI の言語が日本語のときは日本語、それ以外では英語になることと、INSTALL.md が引用する文言は日本語の版であることを示す。

#### F-006: 更新

INSTALL.md は、新しい版に更新する手順を示す。

**Acceptance Criteria**:
- **AC-28**: 次の順の手順を示す。
  1. 拡張を読み込んだフォルダ（AC-15 の、この拡張のためだけのフォルダ）の中身を、すべて消す。
  2. 新しい zip の中身を、そのフォルダに置く。`manifest.json` が、そのフォルダの直下に来るようにする。
  3. `chrome://extensions/`（Brave: `brave://extensions/`）で拡張の再読み込みのボタンを押す。
  4. 更新できたことを、`chrome://extensions/`（Brave: `brave://extensions/`）が示す拡張の版が、入手したリリースの版と同じであることで確かめる。

  あわせて、zip の展開ツールが別名のフォルダや入れ子のフォルダに中身を置くと、フォルダの直下に `manifest.json` がなく、再読み込みで読み込めなくなることと、その対処を示す。
- **AC-29**: 拡張の ID が固定されているので、zip の版から zip の版へ更新しても、保存した API キーが引き継がれることを示す。Chrome Web Store の版へ移るときは ID が変わりうる（[ADR-0026](../../dev/adr/0026-extension-platform.md)、[ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)）ので、その場合の記述は Web Store に申請する issue で直す。
- **AC-30**: 再読み込みすると、開いている拡張のページ（ビューアを含む）が閉じられ、生成中の記事も完成した記事の記録も失われることを示す。そのため、必要な記事は先に保存するよう勧める。

#### F-007: アンインストール

INSTALL.md は、拡張を削除する手順と、そのときに消えるものを示す。

**Acceptance Criteria**:
- **AC-31**: `chrome://extensions/`（Brave: `brave://extensions/`）で拡張を削除する手順を示す。
- **AC-32**: 拡張を削除すると、保存した API キーとワークスペース ID も消えることを示す。削除した後は、展開したフォルダを消してよいことも示す。

#### F-008: 困ったとき

INSTALL.md は、よくある失敗とその対処を示す。

**Acceptance Criteria**:
- **AC-33**: 少なくとも次の場合について、利用者が画面で見るメッセージと、その対処を示す。
  - Anthropic が API キーを拒んだ。
  - API のクレジットの残高が足りないか、利用の上限に達した。このとき拡張は、LLM の呼び出しの失敗として示す。401 以外の拒否は `llm_failed` になるためである（`internal/llm/claude/errors.go:66-73`、`internal/extjob/extjob.go:238-239`）。
  - API キーを保存していない。
  - 字幕を選択していないか、選択が字幕として受け付けられない。
  - 概要欄が折りたたまれている。
  - 記事の生成中に、ビューアのタブを閉じた。

  引用するメッセージは、拡張の日本語のメッセージのカタログ（`extension/src/core/i18n/ja.ts`）の文と一致する。場合を分ける見出し（「API キーが拒まれた」など）は、INSTALL.md 自身の呼び名でよい。
- **AC-34**: インストールのときと使い始めた後の、次の失敗についても、利用者が見ることと対処を示す。ブラウザが示す文言を引用するときは、AC-18 と同じく、確かめた文言だけを引用する。
  - 選んだフォルダに `manifest.json` がなく、読み込めない。
  - ブラウザの版が、AC-08 の版より古い。
  - 展開したフォルダを消したか動かしたため、拡張が動かない。
  - 別のブラウザのプロファイルで使う。拡張はプロファイルごとに読み込むので、そのプロファイルでも拡張を読み込み（AC-16）、API キーを保存し直す（F-004）必要がある。
- **AC-35**: 失敗した記事の生成で料金がかかりうること、やり直すたびにその費用がかかることを示す（今の README の「推定の料金」の節と [production_profile.md](../../dev/production_profile.md) による）。

#### F-009: プライバシー

README は、何をどこに送り、何を保存するかを示す。

**Acceptance Criteria**:
- **AC-36**: `README.md` と `README.en.md` は、次を示す。今の記述は `README.md:180` と `:256-260`、[ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)、`docs/dev/security.md` §8 にある。
  - **送るもの:** 選択した字幕、動画の URL、動画メタデータ（動画タイトル、チャンネル名、概要欄）を、記事の生成のために `https://api.anthropic.com` にだけ送ること。開発者には何も送らないこと（開発者は API を中継するサーバを提供しない：[ADR-0048](../../dev/adr/0048-extension-only-generation.md)）。
  - **API キーとワークスペース ID:** そのブラウザのプロファイルの `chrome.storage.local` にだけ保存し、ほかの端末へ同期しないこと。同じ OS のユーザーとして動くプロセスは読めること。
  - **記事の生成の記録:** ブラウザのメモリに置き、ブラウザを終了すると消えること。
  - **ブラウザの履歴:** ビューアの URL（動画の URL とタイトルを含む）が履歴に残ること。履歴はブラウザの同期でほかの端末に渡りうること、`tabs` や `history` の権限を持つほかの拡張が読めること（`docs/dev/security.md:131`）。
  - **細工したビューアの URL:** URL の値を細工したビューアは誤解させる見出しを表示しうるので、見出しは記事の内容を保証しないこと（`README.md:260`、`docs/dev/security.md:131`）。
  - **記事のファイル:** 利用者が保存を選んだときだけ書くこと。

#### F-010: 文書の言語と検査

利用者向けの文書の言語と同期の扱いを守り、README の値を確かめていた検査を、値が移った先で続ける。

**Acceptance Criteria**:
- **AC-37**: `README.en.md` は `README.md` に同期されている。`TestReadmeSyncMarker` と `TestReadmeVersionsShareStructure` が通る。
- **AC-38**: CLAUDE.md が次を示す。
  - 「Document language」：INSTALL.md が日本語だけであること、それが principle (c) の例外であること、その期限（Chrome Web Store に申請する前）と記録した issue（#1044）。
  - 「Documents」：INSTALL.md がリポジトリのルートに置く例外であることと、その理由。
- **AC-39**: 今のテストが README から読んで確かめている値は、変更の後、その値を述べる文書（README、INSTALL.md、開発者向けの手引きのどれか）から読んで、今と同じ正本と照合する。対象の値は次のとおり。
  - 最低の Chrome の版（`manifest.test.ts`）
  - 概要欄のメッセージ・工程の名前・画面の名前（`metadataDocs.test.ts`）
  - 拡張の ID（`extension_docs_test.go`）
  - 推奨構成の設定のブロック・LLM の既定・effort の値・OpenAI の出力の上限（`docs_test.go` の `TestProductionProfileDocsContract`、`TestLLMDefaultsDocsContract`、`TestEffortValuesDocsContract`、`TestOpenAIDocsContract`）
  - リンク（`TestReadmeLinks`）

  どの値も、述べる文書が変わったことで確かめられなくなってはならない。ただし、どの文書も述べなくなった値の検査は除く。除く検査は、値ごとに理由とともに `02_architecture.md` に挙げる。
- **AC-40**: CLAUDE.md の「Development Notes」で、README を読むテストについて述べた文は、テストが読む文書を正しく示す。
- **AC-41**: 本番の構成と料金の値を繰り返している箇所を挙げる文書が、新しい置き場所を挙げる。
  - `docs/dev/production_profile.md` の「本書を更新するタスク」（`docs/dev/production_profile.md:5`）は、推奨構成の移った先と、INSTALL.md の費用の目安を挙げる。
  - `docs/dev/guides/pricing_update.md`（「料金の変更がほかに触れるもの」、`:34-40`）は、料金の規則とモデルの名前の移った先と、料金が変わったときに見直す箇所として INSTALL.md の費用の目安を挙げる。
- **AC-42**: 拡張のテストが読む文書は、どれも、それを変える PR で CI の拡張のジョブを走らせる。今その対象は、`README.md`、`README.en.md`、`docs/dev/guides/development.md` である（`extension/scripts/has-extension-changes.sh` の冒頭の説明）。CLAUDE.md の「Development Notes」で `make ext-test` が要る文書として挙げる一覧も、同じ文書を挙げる。
- **AC-43**: 利用者向けの文書が新しく使う語（API のクレジット、利用の上限、ワークスペースなど）は、`docs/translation_glossary.md` にある。INSTALL.md の英語版を足すとき（#1044）に同じ訳語を使うためである。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

- N/A。文書だけの変更で、実行時の処理を変えないため。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` は、API キーの扱いの説明を README に頼っている。規則と前提は変えないが、次を守る。

- `docs/dev/security.md` §8 が「README に書く」とする 2 つの記述を、利用者向けの文書で保つ。
  - 専用のワークスペースで月ごとの利用の上限を設けて API キーを作ることを勧める記述（`docs/dev/security.md:142`）。AC-11 で INSTALL.md に置く。
  - 細工した URL のビューアが、誤解させる見出しを表示しうるという記述（`docs/dev/security.md:131`）。AC-36 で README のプライバシーの節に置く。
- 置き場所が INSTALL.md に変わる記述は、`docs/dev/security.md` の参照先も合わせる。
- 文書の例に、実際の API キーを含めない。
- 展開したフォルダは、同じ OS のユーザーとして動くプロセスが書き換えうる。これは、`docs/dev/security.md` §8 が `chrome.storage.local` について受け入れている危険と同じ種類のもので、新しい規則は設けない。
- **信頼できない入力:** N/A。文書とテストだけの変更で、新しく受け取る信頼できない入力はないため。

### 4.3. 信頼性・可用性 (Reliability/Availability)

- N/A。文書だけの変更で、実行時の処理を変えないため。

### 4.4. 互換性 (Compatibility)

- README の節を指すリンクのうち、リポジトリの中のものは AC-06 で付け替える。リポジトリの外からのリンク（GitHub の README の表示のアンカー）は保たない。利用者はまだいないためである（未リリース）。

### 4.5. 保守性 (Maintainability)

- 利用者向けの文書が画面の名前と最低の Chrome の版を述べる箇所は、画面の名前をカタログと、最低の Chrome の版を manifest と、テストで照合する（AC-39）。画面の文言や manifest を変えたときに、文書のずれがテストで分かるようにするためである。
- 費用の目安の値は、根拠の文書（production_profile.md）を名指しする（AC-12）。

## 5. 制約条件 (Constraints)

- `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
- 文書の言語は CLAUDE.md の「Document language」と [ADR-0035](../../dev/adr/0035-language-policy.md) に従う。INSTALL.md の例外は決定 2 と 3 による。
- 文書のテストは、値・構造・実行できる例だけを確かめ、文の意味は PR のレビューで読む（[ADR-0016](../../dev/adr/0016-document-test-policy.md)）。AC-24 と F-008、F-009 の記述の正しさは、PR のレビューで読んで確かめる。
- 拡張の ID は manifest の `key` で固定している（[ADR-0026](../../dev/adr/0026-extension-platform.md)）。AC-29 はこれに基づく。

## 6. 用語集 (Glossary)

用語は [docs/translation_glossary.md](../../translation_glossary.md) と統一する。

- **利用者向けの文書:** `README.md`、`README.en.md`、`INSTALL.md`。
- **開発者向けの手引き:** `docs/dev/guides/` の `cli.md`、`development.md`、またはそこに新しく置く手引き。
- **INSTALL.md:** リポジトリのルートに置く、インストールと利用の手順の文書。当面は日本語だけである。
- **API のクレジット:** Claude Console の組織の残高。Claude API の利用に使う。Max・Ultra・Team の契約に付く毎月の分と、購入した分がある。
