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

**目的:** このドキュメントは、拡張機能を初めて使う利用者が読む文書（README とインストールの手順）の内容と構成についての要件を定める。

対応 issue: [#1042](https://github.com/isseis/yt2column/issues/1042)（関連: [#1039](https://github.com/isseis/yt2column/issues/1039) zip で配布するリリースの手順。このタスクは #1039 をブロックする。[#1044](https://github.com/isseis/yt2column/issues/1044) INSTALL.md の英語版）

**背景:**
#1039 で、拡張機能を zip にして初期の利用者へ配布する予定である。しかし今の README は、開発者がリポジトリからビルドすることを前提に書かれている。
- クイックスタートの最初の手順が `make ext-install` と `make ext-build` である（`README.md:53-55`）。「必要なもの」には Go と Node.js が並ぶ（`README.md:67-68`）。
- CLI の設定（`README.md:292-366`）、推奨構成（`README.md:368-385`）、料金の推定の細かな規則（`README.md:95-118`）、以前の版からの移行（`README.md:270-290`）など、利用者に関係の薄い内容が多い。
- API キーをどの契約で使えるか、どこで作るか、費用の目安、利用の上限の設け方が書かれていない。

**本書が決めることと、設計に委ねること:**
- **本書が決めること:** 利用者向けの文書の構成（README、INSTALL.md、開発者向けの文書の分担）、各文書が述べる内容、文書の言語と同期の扱い、README を読む検査が引き続き値を確かめること。
- **`02_architecture.md` に委ねること:** 節の見出しと順序の細部、README を読むテスト（Go と拡張）の読む先の変え方、移す内容の確かめ方（`diff` 検証のスクリプトなど）、ほかの文書から README の節へのリンクの付け替え方。設計に渡す論点は [design_handoff.md](design_handoff.md) にある。

### issue #1042 で決めたこと

- **README を利用者が読める文書に作り直し、どのような内容にするかの検討もこのタスクに含める。** 配布を始める前に、初めて使う利用者が読める文書が要るためである。
- **利用者向けの内容は、issue の「1. 利用者向けの内容（案）」の項目を元にする。** これは何か、使う前に必要なもの、インストール、初期設定、使い方、更新、アンインストール、困ったとき、プライバシーである。
- **README の日本語版を正本とし、英語版は `/mktrans` で同期する。** CLAUDE.md の「Document language」の principle (c) による。
- **Chrome Web Store の掲載文とプライバシーポリシー、拡張の画面の文言の変更は対象外とする。** 前者は Web Store に申請する issue で扱う。
- **README を読む既存のテスト（`metadataDocs.test.ts`、`manifest.test.ts`、Go のガードテスト）と構成を合わせる。**

### 要件定義の際に決めたこと（2026-11-19）

1. **プロセスは `full` とする。**
   - **理由:** 変更は README の 2 つの版にとどまらない。Go のガードテスト（`cmd/yt2column/docs_test.go`、`extension_docs_test.go`、`readme_structure_test.go`）、拡張のテスト（`extension/test/metadataDocs.test.ts`、`manifest.test.ts`）、開発者向けの手引き、CLAUDE.md、ADR-0035 に及ぶ。決めることも多い。
   - **除外の判定:** 除外に当たらない。`docs/dev/security.md` の規則と前提、開発ツールの信頼境界、外部への依存のどれも変えないためである。
   - **ADR:** なし。タスクの進め方の選択で、このタスクの中で閉じるため。
2. **インストールと利用の手順は、README とは別の `INSTALL.md`（リポジトリのルート）に、当面は日本語だけで置く。**
   - **理由:** README を、概要と案内（これは何か、どう動くか、どこを読めばよいか）に絞れる。手順が長くなっても、README の概要が埋もれない。GitHub Releases の本文や zip からも、手順を直接指せる。英語版を同期する仕組み（`/mktrans`、同期の検査）の一般化は、英語版を足すときまで先送りする。
   - **CLAUDE.md の方針との関係:** CLAUDE.md の「Document language」の principle (c)（利用者向けの文書は日本語を正本とし、英語版を同期する）の例外になる。例外は期限付きとする（決定 3）。
   - **採らなかった案:**
     - INSTALL.md と INSTALL.en.md を対で置き、`/mktrans` と同期の検査を対ごとに動くよう一般化する案。
     - INSTALL を分けず、README の中で構成を整える案。
   - **ADR:** [ADR-0035](../../dev/adr/0035-language-policy.md)（文書の言語の方針）を更新する。principle (c) の例外を定め、後のタスクを縛るため。
3. **INSTALL.md の英語版は、Chrome Web Store に申請する前に足す。** この作業を [#1044](https://github.com/isseis/yt2column/issues/1044) として記録した。それまで、README.en.md は「インストールの手順は今は日本語版だけである」と書いて INSTALL.md へリンクする。
   - **理由:** Web Store に載せると、日本語を読まない利用者が来る。zip で配る初期の利用者は、日本語の話者に限られる。
   - **ADR:** ADR-0035 を更新する（決定 2 と同じ項目）。
4. **README にある開発者向けの内容は `docs/dev/guides/` の手引きに移し、README からはリンクで案内する。「以前の版からの移行」の節は、移さずに削除する。**
   - **移す内容:** CLI の設定（環境変数）、推奨構成、料金の推定の細かな規則、生成の後の工程と出力の書式の詳細、ソースからのビルドと読み込み・更新、ログを見る方法。
   - **理由:** README を利用者向けの内容だけにするためである。移した先の手引きは日本語だけの文書（principle (a)）なので、英語版で訳す量も減る。以前の版（localhost サーバと Native Messaging）を使っていたのは開発者 1 人だけで（[ADR-0048](../../dev/adr/0048-extension-only-generation.md)）、移行はもう済んでいる。
   - **ADR:** ADR-0035 を更新する。「README は利用者向けの内容だけを持ち、開発者向けの内容は `docs/dev/guides/` に置く」と決め、後のタスクを縛るため。
5. **#1039 との分担：このタスクは、zip の入手・展開・読み込み・更新・アンインストールの手順まで書く。#1039 は、来歴の検証（`gh attestation verify`）の手順を足し、未リリースの注記を外す。**
   - **理由:** このタスクは #1039 より先にマージされるので、マージした時点では zip がまだない。一方、来歴の検証は、#1039 のワークフローが作る attestation がないと書けない。それ以外の手順は、zip の作り方によらずに書ける。
   - **最初のリリースまでの扱い:** README と INSTALL.md は、まだリリースしていないことを示し、ソースからのビルドは開発者向けの手引きにあると案内する。
   - **ADR:** なし。タスクの間の作業の分け方で、このタスクと #1039 の中で閉じるため。
6. **API キーを使える契約は、要件定義のときに Anthropic の公式の情報で確かめた（2026-11-19）。**
   - **確かめた内容:**
     - Claude の有料の契約（claude.ai）と Claude API は別の製品で、API の利用は別に課金される（[Claude ヘルプセンター](https://support.claude.com/en/articles/9876003-i-have-a-paid-claude-subscription-pro-max-team-or-enterprise-plans-why-do-i-have-to-pay-separately-to-use-the-claude-api-and-console)）。
     - Max・Ultra・Team の契約には、毎月の Claude API のクレジットが付く。claude.ai の請求の設定から Claude Console の組織を結び付けると、その組織に届く。クレジットを使い切ると、API の要求は次のクレジットが届くまで止まり、契約の側には請求されない。Free・Pro・Enterprise は対象外である（[API credits for Max, Ultra, and Team plans](https://platform.claude.com/docs/en/about-claude/api-credits-for-subscribers)）。
     - どの場合も、API キーは Claude Console で作る。ワークスペースごとに利用の上限（spend limit）を設けられる（同）。
   - **理由:** 書く内容がこの結果で変わるからである（issue の「要確認」）。
   - **ADR:** なし。文書に書く事実の確認で、決定ではないため。
7. **記事 1 本あたりの費用の目安として「約 0.4 USD」を示す。**
   - **根拠:** [production_profile.md](../../dev/production_profile.md) の製品の制約「費用」（約 0.36 USD。0018 の評価記録 §4.8）。
   - **添える内容:** 目安は評価のときの値で、請求額ではない。実際の額は、ビューアが示す推定の料金と Claude Console で確かめる。
   - **値の更新:** 本番の構成か料金表を変えるタスクが直す。
   - **理由:** 利用の上限を決めるときの手がかりになる。根拠の文書を名指ししておけば、値が古くなったときに直す元をたどれる。
   - **ADR:** なし。文書に書く値で、このタスクの中で閉じるため。
8. **手順に画面の画像は入れない。** 画面の名前（ボタンなど）は、今の README と同じく太字で引用する。
   - **理由:** UI は日本語と英語の 2 言語で、画面を変えるたびに画像を撮り直す負担が大きい。
   - **ADR:** なし。このタスクの中で閉じるため。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

- 拡張機能を初めて使う利用者が、Go や Node.js なしで、入手から最初の記事の保存まで進めること。
- 利用者が、API キーを使える契約、費用の目安、利用の上限の設け方を、使い始める前に知ること。
- 開発者向けの内容を利用者向けの文書から分け、それぞれの読み手が必要な文書だけを読めるようにすること。

### 2.2. スコープ (In Scope)

- `README.md` と `README.en.md` を、利用者向けの概要と案内に作り直す。
- ルートに `INSTALL.md`（日本語）を新しく置き、使う前に必要なものから困ったときまでの手順を書く。
- README にある開発者向けの内容を `docs/dev/guides/` の手引きに移し、「以前の版からの移行」の節を削除する。
- README の内容を確かめている既存のテストを、内容が移った先の文書に合わせる。
- 文書の言語の例外（INSTALL.md）を、ADR-0035 と CLAUDE.md に記録する。
- 移した節を指すほかの文書のリンクを、移った先に付け替える。

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
  - INSTALL.md への案内
  - 開発者向けの手引きへのリンク
- **AC-02**: `README.md` と `README.en.md` には、次のどれも含まれない。
  - 利用の前提として Go か Node.js を挙げる記述
  - `make` のターゲットを利用者の手順とする記述
  - CLI の環境変数の表と、その値の文法
  - 推奨構成の設定のブロック
  - DeepSeek と OpenAI の設定と料金の規則
  - 以前の版からの移行
- **AC-03**: ルートに `INSTALL.md` があり、日本語で書かれ、次の内容を順に持つ：使う前に必要なもの（F-002）、インストール（F-003）、初期設定（F-004）、使い方（F-005）、更新（F-006）、アンインストール（F-007）、困ったとき（F-008）。
- **AC-04**: 今の README にある次の内容は、開発者向けの手引きにある。
  - CLI の設定（`README.md:292-366`）
  - 推奨構成（`README.md:368-385`）
  - 料金の推定の規則（`README.md:95-118`）
  - 生成の後の工程（`README.md:75-86`）と出力の書式（`README.md:88-93`）の詳細
  - ソースからのビルド、読み込み、更新、拡張の ID（`README.md:120-155`）
  - ログを見る方法（`README.md:266-268`）

  移した内容が述べる挙動は、今の README と変わらない。移した先の文と今の README の文の差分は、次の 3 つに限る。
  - 見出しの階層
  - リンクの先
  - 移した先の手引きにすでにある同じ内容を重ねて書かないための削除
- **AC-05**: 「以前の版からの移行」の節（`README.md:270-290`）は、どの文書にも移さずに削除する。クイックスタートからその節への案内（`README.md:61`）も削除する。
- **AC-06**: 利用者向けの文書と開発者向けの手引きにあるすべての相対リンクとアンカーは、存在するファイルと見出しを指す。今の README の節を指すほかの文書のリンク（`docs/dev/guides/cli.md`、`docs/dev/guides/development.md`、`docs/dev/conventions/package_reference.md` など）も、内容が移った先を指す。
- **AC-07**: `README.md` と `README.en.md` は、来歴の検証を除くインストールの手順が INSTALL.md にあることを示し、INSTALL.md へリンクする。`README.en.md` は、手順が今は日本語版だけであることを示す。

#### F-002: 使う前に必要なもの

INSTALL.md は、使い始める前にそろえるものと、それにかかる費用を示す。

**Acceptance Criteria**:
- **AC-08**: 必要なブラウザとして「Chrome 142 以降、または Chromium 142 以降の Brave」を示す。示す版は、`extension/static/manifest.json` の `minimum_chrome_version` と一致する。
- **AC-09**: API キーを使える契約について、次を示す。
  - Claude API の利用は、claude.ai の契約（Free、Pro、Max、Ultra、Team、Enterprise）とは別に課金されること。
  - Max・Ultra・Team の契約には毎月の API のクレジットが付き、claude.ai の請求の設定から Claude Console の組織を結び付けると使えること。
  - Free・Pro・Enterprise の契約には、API の利用分が含まれないこと。
  - どの場合も、API キーは Claude Console で作ること。

  この記述は、決定 6 の公式の出典へのリンクと、確かめた日付を伴う。
- **AC-10**: Claude Console でアカウントを作り、支払い方法を登録して（または、決定 6 の契約のクレジットを結び付けて）、API キーを作るまでの手順を示す。
- **AC-11**: 月ごとの利用の上限（spend limit）を設けた、この拡張のためだけのワークスペースを作り、そこで API キーを作ることを勧める。その設け方を示す。そのワークスペースで作った API キーなら、ワークスペース ID の入力が要らないことも示す。
- **AC-12**: 記事 1 本あたりの費用の目安として「約 0.4 USD」を示し、[production_profile.md](../../dev/production_profile.md) を根拠として名指しする。あわせて、目安は請求額ではないこと、実際の額はビューアの推定の料金と Claude Console で確かめることを示す。
- **AC-13**: ゼロデータ保持（ZDR）か Compliance API を有効にした組織の API キーでは使えないことと、その理由（ブラウザからの API の呼び出しを受け付けないこと）を示す。今の記述は `README.md:70` にある。

#### F-003: インストール

INSTALL.md は、配布された zip から拡張を読み込むまでの手順を示す。

**Acceptance Criteria**:
- **AC-14**: zip を GitHub Releases から入手することを示し、リポジトリの Releases のページへリンクする。最初のリリースまでは、まだリリースしていないことを示し、ソースからのビルドの手順が開発者向けの手引きにあることを案内する（決定 5）。
- **AC-15**: zip を展開する場所は、後で消したり動かしたりしない場所にすること、その理由（フォルダを消すか動かすと拡張が動かなくなること）を示す。
- **AC-16**: 次の順の手順を示す。
  1. `chrome://extensions/`（Brave: `brave://extensions/`）を開く。
  2. **デベロッパー モード**（**Developer mode**）をオンにする。
  3. **パッケージ化されていない拡張機能を読み込む**（**Load unpacked**）で、展開したフォルダを選ぶ。
- **AC-17**: 拡張をツールバーに固定（ピン留め）する手順を示す。
- **AC-18**: デベロッパー モードの拡張について、ブラウザが示しうる警告の意味を示す。警告は、ストアの外から読み込んだ拡張であるために出ること、それへの対処を示す。
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
- **AC-24**: 今の README の「ポップアップ」と「ビューア」の節（`README.md:182-264`）が述べる、利用者に見える挙動を、すべて INSTALL.md で述べる。言い回しは書き直してよい。挙動とは、画面の状態と操作、記録の残り方、選択範囲の扱いと上限である。ログを見る方法は、開発者向けの手引きに移す（AC-04）。
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
- **AC-28**: 新しい zip を同じフォルダに展開して中身を差し替え、`chrome://extensions/`（Brave: `brave://extensions/`）で拡張の再読み込みのボタンを押す手順を示す。
- **AC-29**: 拡張の ID が固定されているので、更新しても保存した API キーが引き継がれることを示す。
- **AC-30**: 再読み込みすると、開いている拡張のページ（ビューアを含む）が閉じられ、生成中の記事も完成した記事の記録も失われることを示す。そのため、必要な記事は先に保存するよう勧める。

#### F-007: アンインストール

INSTALL.md は、拡張を削除する手順と、そのときに消えるものを示す。

**Acceptance Criteria**:
- **AC-31**: `chrome://extensions/`（Brave: `brave://extensions/`）で拡張を削除する手順を示す。
- **AC-32**: 拡張を削除すると、保存した API キーとワークスペース ID も消えることを示す。削除した後は、展開したフォルダを消してよいことも示す。

#### F-008: 困ったとき

INSTALL.md は、よくある失敗とその対処を示す。

**Acceptance Criteria**:
- **AC-33**: 少なくとも次の場合について、利用者が画面で見る状態またはメッセージと、その対処を示す。
  - Anthropic が API キーを拒んだ（**API キーが拒まれた**）
  - API のクレジットの残高が足りない（**失敗** になり、LLM の呼び出しが失敗したと示す。Claude Console で残高と利用の上限を確かめる）
  - API キーを保存していない（**設定が要る**）
  - 字幕を選択していない、または選択が字幕として受け付けられない（**開始が拒否された** など）
  - 概要欄が折りたたまれている
  - 記事の生成中にビューアのタブを閉じた（**ページとともに止まった**）

  引用する画面の状態の名前は、拡張の日本語のメッセージのカタログの名前と一致する。
- **AC-34**: 失敗した記事の生成で料金がかかりうること、やり直すたびにその費用がかかることを示す（今の README の「推定の料金」の節と [production_profile.md](../../dev/production_profile.md) による）。

#### F-009: プライバシー

README は、何をどこに送り、何を保存するかを示す。

**Acceptance Criteria**:
- **AC-35**: `README.md` と `README.en.md` は、次を示す。今の記述は `README.md:180` と `:256-260`、[ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)、`docs/dev/security.md` §8 にある。
  - **送るもの:** 選択した字幕、動画の URL、動画メタデータ（動画タイトル、チャンネル名、概要欄）を、記事の生成のために `https://api.anthropic.com` にだけ送ること。開発者には何も送らないこと。
  - **API キーとワークスペース ID:** そのブラウザのプロファイルの `chrome.storage.local` にだけ保存し、ほかの端末へ同期しないこと。同じ OS のユーザーとして動くプロセスは読めること。
  - **記事の生成の記録:** ブラウザのメモリに置き、ブラウザを終了すると消えること。
  - **ブラウザの履歴:** ビューアの URL（動画の URL とタイトルを含む）が履歴に残ること。
  - **記事のファイル:** 利用者が保存を選んだときだけ書くこと。

#### F-010: 文書の言語と検査

利用者向けの文書の言語と同期の扱いを守り、README の値を確かめていた検査を、値が移った先で続ける。

**Acceptance Criteria**:
- **AC-36**: `README.en.md` は `/mktrans` で `README.md` に同期され、`TestReadmeSyncMarker` と `TestReadmeVersionsShareStructure` に通る。
- **AC-37**: INSTALL.md が日本語だけであること、それが principle (c) の例外であること、その期限（Chrome Web Store に申請する前）と記録した issue（#1044）を、CLAUDE.md の「Document language」が示す。
- **AC-38**: 今のテストが README から読んで確かめている値は、変更の後、その値を述べる文書（README、INSTALL.md、開発者向けの手引きのどれか）から読んで、今と同じ正本と照らして確かめる。対象の値は次のとおり。
  - 最低の Chrome の版（`manifest.test.ts`）
  - 概要欄のメッセージ・工程の名前・画面の名前（`metadataDocs.test.ts`）
  - 拡張の ID（`extension_docs_test.go`）
  - 推奨構成の設定のブロック・LLM の既定・effort の値・出力の上限・料金に関する値（`docs_test.go`）
  - リンク（`TestReadmeLinks`）

  どの値も、述べる文書が変わったことで確かめられなくなってはならない。ただし、どの文書も述べなくなった値の検査は除く。
- **AC-39**: CLAUDE.md の「Development Notes」で、README を読むテストについて述べた文は、テストが読む文書を正しく示す。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

- N/A。文書だけの変更で、実行時の処理を変えないため。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` は、API キーの扱いの説明を README に頼っている。規則と前提は変えないが、次を守る。

- `docs/dev/security.md` §8 が「README に書く」とする 2 つの記述を、利用者向けの文書で保つ。
  - 専用のワークスペースで月ごとの利用の上限を設けて API キーを作ることを勧める記述（`docs/dev/security.md:142`）。AC-11 で INSTALL.md に置く。
  - 細工した URL のビューアが、誤解させる見出しを表示しうるという記述（`docs/dev/security.md:131`）。
- 置き場所が INSTALL.md に変わる記述は、`docs/dev/security.md` の参照先も合わせる。
- 文書の例とスクリーンショットに、実際の API キーを含めない（画像は入れない：決定 8）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

- N/A。文書だけの変更で、実行時の処理を変えないため。

### 4.4. 互換性 (Compatibility)

- README の節を指すリンクのうち、リポジトリの中のものは AC-06 で付け替える。リポジトリの外からのリンク（GitHub の README の表示のアンカー）は保たない。利用者はまだいないためである（未リリース）。

### 4.5. 保守性 (Maintainability)

- 利用者向けの文書が画面の名前と最低の Chrome の版を述べる箇所は、テストでカタログと manifest に照らす（AC-38）。画面の文言や manifest を変えたときに、文書のずれがテストで分かるようにするためである。
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
