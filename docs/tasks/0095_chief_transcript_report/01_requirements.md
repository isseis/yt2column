# 要件定義書：chief が Claude Code の pane の最後の報告を、画面ではなく transcript から読む

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-16 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、chief（[0088 要件定義書](../0088_chief_of_staff/01_requirements.md)）が Claude Code の pane の最後の報告を読む情報源を、pane の画面から Claude Code の transcript に替えるための要件を定義する。最後の報告とは、`/runpr` の報告と、PR を作ったコマンドの報告のことである。

**背景:**

chief は、Claude Code の pane の最後の報告を画面から読み、次の 4 つの確かめと表示に使っている。

| 記号 | 使い道 | 今の読み方 | 読んだ結果の使い方 |
|---|---|---|---|
| ① | 自動送信の前に、`/runpr` の報告の NEXT が、chief の決めた次のコマンドと同じかを照合する（0088 要件定義書の AC-42） | workspace のすべての pane の画面にある NEXT の行のすべて（`internal/chief/autosend.go:441-461`、`internal/chief/screen_report.go:338-349`） | 違うとき、または読めないときは送らない |
| ② | ブランチのない状態での自動送信の許可（0088 要件定義書の AC-41）と、自動送信の後に `/runpr` の pane を空にすること（同 AC-53） | `/runpr` の送り先の pane の画面の最後のメッセージにある NEXT と PR 番号（`internal/chief/autosend.go:464-487`、`internal/chief/autosend.go:769-771`、`internal/chief/screen_report.go:311-333`） | マージした PR と次のコマンドを名指していなければ、送らないか、空にしない |
| ③ | PR の詳細に、PR を作ったコマンドの報告を示し、その要約の材料にする（0088 要件定義書の AC-17・AC-19・AC-55） | Claude Code の pane の画面の最後のメッセージのうち、PR の URL を含むもの（`internal/chief/detail.go:444-465`、`internal/chief/screen_report.go:140-153`） | ちょうど 1 つの pane で読めればその報告を示し、そうでなければ PR の説明を示す |
| ④ | `/runpr` の後に、PR を作った pane を空にする前の確かめ（0088 要件定義書の AC-52） | ③と同じ報告がまだ画面にあるか（`internal/chief/reset.go:133-137`） | なければ空にしない |

④では、このほかに、報告を控えたときから pane の状態が変わっていないこと（`internal/chief/reset.go:128-130`）と、その Claude Code のセッションの transcript で人が最後に入力した行が PR を作ったコマンドであること（`internal/chief/reset.go:51-74`）も確かめている。

画面からの読み取りには、次の問題がある。

-   Herdr の `herdr agent read` は、pane の高さの分の行しか返さない。`/mkarch`・`/mkplan`・`/runplan` の最後の報告は、たいてい画面より長い。報告の始まりが画面の外に出ると、chief は報告として読まない。このため③はいつも PR の説明から要約され、④は空にされない。（[chief 利用ガイド](../../dev/guides/chief_guide.md) の「8. 既知の制限」の「長い報告は画面から読めない」、[#892](https://github.com/isseis/yt2column/issues/892)）
-   画面の折り返しや描き直しで切り出しに失敗すると、送るべきものが止まる（issue #905 の背景）。

Claude Code の transcript には、最後の応答の本文が、折り返しなしで全文残る。chief は、モデルの切り替えと人の最後の入力を、すでに transcript から読んでいる（`internal/chief/screen_transcript.go:46-63`、`internal/chief/screen_transcript.go:170-189`）。transcript の場所は、Herdr が報告するセッションと作業ディレクトリから決めている（`internal/chief/transcript.go:53-70`）。

対応 issue: [#905](https://github.com/isseis/yt2column/issues/905)（親: [#902](https://github.com/isseis/yt2column/issues/902)、関連: [#892](https://github.com/isseis/yt2column/issues/892)・[#904](https://github.com/isseis/yt2column/issues/904)・[#906](https://github.com/isseis/yt2column/issues/906)・[#908](https://github.com/isseis/yt2column/issues/908)）

**本書の記述範囲:** 本書は、Claude Code の pane の最後の報告をどこから読むか、どの確かめにどう使うか、読めないときにどう振る舞うかという、観測できる振る舞いを定める。次の事項は設計（`02_architecture.md`）で決める。

-   transcript のどの行と項目から「最後に終わった応答」を見分けるか
-   transcript を読む量の上限の値
-   画面の読み取りのうち、使われなくなる部分とそのサンプルの扱い
-   送らない理由と空にしない理由の値の綴りと文言
-   テストの手段と、採る transcript のサンプル

要件の段階で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #905 で決めたこと:**

-   **Claude Code の pane の最後の報告（`/runpr` の報告の NEXT と PR 番号、PR を作ったコマンドの報告）を、画面ではなく transcript の最後の応答から読む。** transcript には報告の全文が折り返しなしで残り、画面の折り返し・描き直し・高さに左右されないためである。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する。
-   **報告を確かめとして使い続けることが要るかを見直し、計画書・送り先の表・GitHub から決まる値と重なるなら、確かめの対象を減らす。** 見直しの結果は、下の「要件定義の際に決めたこと」にある。ADR: ADR-0046 を更新する（見直しの結果を決定に書くため）。
-   **ADR-0046 と CLAUDE.md の「Declare, don't infer」の例外のうち、この変更で変わる部分を更新する（F-004）。** ADR: ADR-0046 を更新する。
-   **Claude Code の質問の判定（[#904](https://github.com/isseis/yt2column/issues/904)）と、OpenCode の pane の報告（[#906](https://github.com/isseis/yt2column/issues/906)）は扱わない。** 親の issue #902 が、sub-issue ごとに分けて進めると決めているためである。ADR: なし（作業の分け方で、決定を変えないため）。

**要件定義の際に決めたこと（2026-11-16）:**

-   **進め方は通常の進め方にする。対象外の判断は「対象外」で、上書きはない。** この変更は、chief が人に尋ねずに送るのを止める条件と、pane を空にする条件について、情報源（信頼できない文字列）を替える。これは開発用の道具の信頼の境界に触れるので、簡易な進め方の対象外である（[requirements_process.md](../../dev/conventions/requirements_process.md) の「Simple Process」の「Excluded changes」）。ADR-0046 の決定を変え、道具の 4 か所の読み取り、CLAUDE.md、利用ガイドに及ぶことも理由である。ADR: なし（進め方の選択はこのタスクの中で閉じるため）。
-   **①〜④の確かめは、すべて残す。変えるのは、Claude Code の pane の報告を読む情報源だけである（F-002）。①は、pane の画面にある NEXT の行のすべてではなく、最後の報告だけを読む。** NEXT と PR 番号は計画書と GitHub から決まる値と重なる。それでも①は、chief 自身の判定の誤りや古い状態を拾える唯一の照合である。また④の確かめは、人の最後の入力を確かめるために読んでいる transcript で済み、残しても負担がほとんどない。確かめを外すと、その分だけ、送る条件と空にする条件が緩む。①を最後の報告だけにするのは、transcript には画面に見えていない古い報告まで残っており、そのすべてを照合すると、人が見ていない古い報告で送信が止まるためである。①を外す案と、①と④の確かめを外す案は採らない。ADR: ADR-0046 を更新する。
-   **transcript を読めないときは、画面の読み取りに戻らず、安全側に倒す（F-003）。①②は送らない。③は PR の説明から要約する。④は空にしない。** 画面に戻ると、壊れやすい経路が残る。これは、判定の情報源を画面から移すという親の issue #902 の方針に反する。画面に戻る案は採らない。ADR: ADR-0046 を更新する。
-   **[#892](https://github.com/isseis/yt2column/issues/892)（PR を作ったコマンドの報告を transcript から読む）は、このタスクで扱う。利用ガイドの既知の制限「長い報告は画面から読めない」も、このタスクで直す（AC-14）。#892 は、このタスクが完了したときに閉じる。** #892 の「やること」の 1・2 は、本書の F-001・F-002 と同じである。3（OpenCode の報告）は #906 が扱う。#892 の「決めること」は、次のように扱った。
    -   画面の読み取りの例外の変更は F-004 で扱う。
    -   報告と認める条件は、F-001 と、④で今の条件を残すこと（AC-08）で決めた。
    -   画面の読み取りを残すかどうかは、上の項で「戻らない」と決めた。

    ADR: なし（issue の扱いで、決定を変えないため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   画面の高さより長い報告も、Claude Code の pane から読めるようにする。PR の詳細に PR を作ったコマンドの報告を示し、`/runpr` の後に PR を作った pane を空にできるようにする。
-   画面の折り返しや描き直しのせいで、Claude Code の pane の報告を読み損ねて送信が止まることをなくす。
-   報告を使う確かめの強さを変えない。

### 2.2. スコープ (In Scope)

-   Claude Code の pane の最後の報告を、そのセッションの transcript から読む（F-001）
-   ①〜④の確かめで、Claude Code の pane の報告を、transcript から読んだ報告に替える（F-002）
-   transcript を読めないときの振る舞い（F-003）
-   ADR-0046、CLAUDE.md の例外、chief 利用ガイドの更新（F-004）

### 2.3. スコープ外 (Out of Scope)

-   Claude Code の質問の判定と、答えを中継する前の照合（[#904](https://github.com/isseis/yt2column/issues/904)）。理由: 親の issue #902 の分け方による。
-   OpenCode の pane の報告の読み取り（[#906](https://github.com/isseis/yt2column/issues/906)）。OpenCode の pane の報告は、今までどおり画面から読む（AC-12）。理由: 同上。
-   使われなくなる画面の読み取りを、まとめて消すこと（[#908](https://github.com/isseis/yt2column/issues/908)）。ただし、このタスクで使われなくなる Claude Code の報告の画面の読み取りの扱いは、設計で決める（design_handoff.md の H-05）。
-   入力欄に残った文字の判定。理由: 画面でしか読めず、#902 の方針でも画面に残すためである。
-   次のコマンドの決め方、自動送信の引き金、送る入力の範囲。理由: issue #902 の方針で変えないと決めているためである。
-   0088 要件定義書の書き換え。完了したタスクの文書は書き換えない。0088 の AC-19 は、報告が pane に残っていない例として「スクロールで消えた」を挙げている。この変更の後は、Claude Code の pane ではスクロールで報告が消えることはなく、`/clear` などでセッションが変わったときに報告がなくなる（AC-02）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: Claude Code の pane の最後の報告を transcript から読む

Claude Code の pane の最後の報告を、その pane のセッションの transcript に記録された、最後に終わった応答の本文から読む。

**Acceptance Criteria**:
-   **AC-01**: Claude Code の pane の最後の報告は、その pane のセッションの transcript に記録された応答のうち、最後に終わった応答（エージェントが人の入力を待つ状態に戻ったときの応答）の本文である。報告が画面の高さより長くても、本文の全体を読む（読む量の上限は 4.2 のとおり）。
-   **AC-02**: 次のときは、その Claude Code の pane に最後の報告はない。
    -   transcript に、終わった応答がない。
    -   最後に終わった応答の後に、人の入力か、新しい応答の始まりがある（実行中か、人が次の作業を始めた）。
    -   最後の応答が、終わらずに止まっている（人が中断したときなど）。
    -   最後に終わった応答に本文がない。
    -   `/clear` などでセッションが替わり、新しいセッションの transcript に終わった応答がない。
-   **AC-03**: サブエージェントの応答は、pane の最後の報告ではない。
-   **AC-04**: Claude Code の pane の最後の報告を、画面からは読まない。transcript に報告があり画面にないとき、報告は読める。画面に報告があり transcript にないとき、報告はない。

#### F-002: 報告を使う確かめを保ち、Claude Code の pane では transcript の報告を使う

①〜④の確かめと表示は、条件を変えずに残す。Claude Code の pane については、F-001 の最後の報告を使う。

**Acceptance Criteria**:
-   **AC-05**（①）: 自動送信の前に、workspace の Claude Code の pane の最後の報告に NEXT の行があり、その値が chief の決めた次のコマンドとコマンドかタスクのどちらかで違うとき、chief は送らない。報告が次のコマンドはないと示すときも送らない。送らないときは、違いとともに提案として示す（0088 要件定義書の AC-42 と同じ）。最後の報告がない pane と、最後の報告に NEXT の行がない pane は、違うものとして扱わない。最後の報告より前の報告の NEXT は照合しない。
-   **AC-06**（②）: `/runpr` の送り先の表のエージェントが Claude Code のとき、次の 2 つでは、その pane の最後の報告を F-001 の報告とする。ブランチのない状態で自動送信を許すかどうかの判定（0088 要件定義書の AC-41）と、自動送信の後に `/runpr` の pane を空にする前の確かめ（同 AC-53）である。報告がマージした PR の番号と次のコマンドを名指すことを求める条件は変えない。
-   **AC-07**（③）: PR の詳細では、Claude Code の pane の最後の報告のうち、その PR の URL（後に数字が続かないもの）を含むものを、PR を作ったコマンドの報告とする。ちょうど 1 つの pane の報告がそれに当たるときは、その報告を示す。そうでないときは PR の説明を示す。どちらから示したかが分かること（0088 要件定義書の AC-19）と、pane を空にした後も示せること（同 AC-55）は変えない。
-   **AC-08**（④）: `/runpr` の後に PR を作った pane を空にするのは、今の条件（控えた報告がその pane のものであること、控えたときから pane の状態が変わっていないこと、transcript で人が最後に入力した行が PR を作ったコマンドであること）をすべて満たし、かつ、その pane の最後の報告が、空にする直前に読み直しても、その PR の URL を含むときだけである。
-   **AC-09**: 報告から読んだ NEXT、PR 番号、本文は、照合と表示と要約にだけ使う。送るコマンドとタスク、送る先の pane を決めるためには使わない（0088 要件定義書の AC-42、[ADR-0046](../../dev/adr/0046-chief-of-staff.md) の決定と同じ）。

#### F-003: transcript を読めないときは安全側に倒す

Claude Code の pane の transcript を読めないときは、画面の読み取りに戻らず、送らないか、空にしないか、PR の説明を示す。

**Acceptance Criteria**:
-   **AC-10**: 次のいずれかのとき、その Claude Code の pane の transcript は読めない。
    -   Herdr がその pane のセッションを報告していない。
    -   作業ディレクトリが、transcript の場所を決められない文字を含む（`internal/chief/transcript.go:22` の `unsupported_path`）。
    -   transcript のファイルがないか、通常のファイルでない。
    -   transcript が 4.2 の受け付ける形に合わない。
    -   transcript が読む量の上限を超える。
-   **AC-11**: AC-10 のとき、chief は画面から報告を読まない。①②ではその workspace に送らず、送らない理由（transcript を読めないことと、その pane）を示す。③ではその pane に報告がないものとして扱う（ほかに報告がちょうど 1 つの pane がなければ PR の説明を示す）。④ではその pane を空にせず、空にしない理由を示す。作業ディレクトリの文字が理由のときは、人ができること（その文字を含まないパスの worktree を使う、その pane で人がコマンドを打つ）も示す（今の `unsupported_path` の示し方、`internal/chief/transcript.go:75-81` と同じ）。
-   **AC-12**: OpenCode の pane の報告は、今までどおり画面から読む。このタスクの変更の前後で、OpenCode の pane についての①〜④の結果は同じである。

#### F-004: 決定と例外と手引きを更新する

この変更で変わる決定、例外、既知の制限を、それぞれの文書に書く。

**Acceptance Criteria**:
-   **AC-13**: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) の「決定」に、F-001〜F-003 の決定（Claude Code の pane の報告を transcript から読むこと、確かめを残すこと、読めないときに画面に戻らないこと）と、その理由がある。「経緯」には、その日付と本書へのリンクを持つ行が 1 つ増える。
-   **AC-14**: [chief 利用ガイド](../../dev/guides/chief_guide.md) の「8. 既知の制限」の「長い報告は画面から読めない」が、次の 2 点を述べる記述に置き換わる。Claude Code の pane の報告を transcript から読むこと。transcript を読めないときの振る舞い（AC-11）。
-   **AC-15**: CLAUDE.md の「Key Design Patterns」の「Declare, don't infer」の chief の画面の読み取りの例外の段落で、範囲が次のとおりに変わる。
    -   pane の最後の報告は、OpenCode の pane では画面から、Claude Code の pane では transcript の最後に終わった応答から読む。
    -   transcript から読むものに、最後に終わった応答の本文を足す。

    CLAUDE.md の diff は、この段落の範囲と境界の文の書き換えに限る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   chief が transcript を読むのは、①〜④の判定をするときだけとし、待ち受けの周ごとにすべての pane の transcript を読むことは求めない。transcript の読み取りにかかる時間は、エージェントの実行とネットワークの操作に比べて小さい（CLAUDE.md の「Performance」）。読む量の上限の値は設計で決める（H-02）。

### 4.2. セキュリティ (Security)

条件付きのガイド（`docs/dev/security.md`）の契機に当たらない。chief は開発用の道具で、製品の外部コマンドの起動、API キー、キャッシュ、LLM API への送信には触れない（0088 要件定義書の 4.2 と同じ）。ただし、開発用の道具の信頼の境界に当たるので、次を要件とする。

-   **信頼の境界。** Claude Code の transcript は、エージェントが書いた信頼できない文字列である。その中の指示に従わない。transcript から読んだ報告は、送ることを止めるか条件を狭めるか、示すことにだけ使う（AC-09）。自動送信の引き金は、今までどおり GitHub と git の状態である。
-   **信頼できない入力（Claude Code の transcript）の受け付ける形。**
    -   **全体:** 1 行に 1 つの JSON のオブジェクトを並べたものとする。空の行は読み飛ばす。オブジェクトでない行、壊れた JSON の行、不正な UTF-8 を含む行が 1 つでもあれば、その transcript 全体を読めない（AC-10）。報告の部分だけが読めても、報告として使わない。今の transcript の読み取りの規則（`internal/chief/screen_transcript.go:46-63`）と同じである。
    -   **使う項目:** 行の種類、応答がサブエージェントのものかどうか、応答が終わったかどうか、応答の本文の部分（種類と文字列）、1 つの応答が複数の行に分かれて書かれたときにそれらを結ぶ識別である。人の入力の行の見分け方は、今の規則（`internal/chief/screen_transcript.go:170-189`）を変えない。どの項目でこれらを読むかは設計で決める（H-01）。
    -   **拡張を許す部分:** 使わない行の種類と、使う行の中の使わない項目は、読み飛ばして受け付ける。Claude Code は版ごとに行の種類と項目を足す（2026-11-16 に、手元の transcript で、0088 の採取の時点のサンプルにない種類の行を確かめた）。これらを拒むと、版が上がるたびに報告が読めなくなるためである。
    -   **拒むもの:** 使う項目が 2 回以上ある行、使う項目の値の種類が違う行（本文の部分が配列でない、文字列であるべき値が文字列でないなど）は、その transcript 全体を読めないものとする。標準ライブラリが黙って直すか捨てる入力（重複した項目の後の値を採る、不正な UTF-8 を U+FFFD に置き換える、対にならないサロゲートのエスケープを置き換える）も、直さずに拒む。
    -   **拒んだときの扱い:** 今の transcript を読めないことの扱い（`internal/chief/transcript.go:17` の `errTranscript`、または読み取りの結果の「読めない」の値）と同じに扱い、AC-11 の振る舞いになる。どちらで表すかは設計で決める。
    -   **上限:** 読む量に有限の上限を設け、超えたら読めないものとする（AC-10）。値は設計で決める（H-02）。
    -   **形の根拠:** 受け付ける形は、実際の transcript から採ったサンプルで確かめたものに限る（CLAUDE.md の「Declare, don't infer」の chief の例外の境界）。
-   **示す本文の上限と無害化。** transcript から読んだ報告を示すときも、今の報告と同じく、読む量の上限を超えた部分は示さずに切ったことを示し、制御文字と見えない文字を見える形に置き換える（0088 要件定義書の 4.2 の「読み取る量の上限」と「画面のとおりに示す」）。
-   **サンプルの取り扱い。** リポジトリに足す transcript のサンプルは、採った人を特定できる値（ホームディレクトリのパス、利用者名など）を含まない形にする（[security.md](../../dev/security.md) の §7）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   Claude Code が transcript を書いている途中の行（書きかけの行）は、読めない行になる（4.2）。そのときは AC-11 の振る舞いになり、次の周で読み直す。書きかけの行のために送ってしまうことはない。

### 4.4. 互換性 (Compatibility)

-   このタスクの前に画面から控えた PR の詳細の記録（出どころが報告のもの）は、そのまま読める。④の確かめ（AC-08）は、そうした記録にも同じ条件で行う。
-   Claude Code 2.1.325 の transcript の形を基準にする。版が上がって形が変わり、受け付ける形に合わなくなったときは、AC-11 の振る舞いになる。サンプルを採り直すまで、Claude Code の pane の報告は読めない。

### 4.5. 保守性 (Maintainability)

-   transcript から報告を読む部品は、CLAUDE.md の「Declare, don't infer」の chief の例外の範囲（`internal/chief` の、名前が `screen` で始まるファイル）に置く。ほかの部品は、その部品が返す型だけで振る舞いを選ぶ。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [ADR-0046](../../dev/adr/0046-chief-of-staff.md) の決定に従う。この変更は、その決定のうち、報告を読む情報源と、読めないときの扱いを変える。chief は人の代わりに判断しない。自動送信の引き金は GitHub と git の状態とする。送る入力の範囲も変えない。
-   **CLAUDE.md の「Declare, don't infer」の例外。** 方針（[CLAUDE.md](../../../CLAUDE.md)「Key Design Patterns」の「Declare, don't infer」）は、呼び出し側が渡した文字列の中身を調べて振る舞いを選ばず、型で宣言することを求める。chief は、transcript の最後に終わった応答の本文の中身を調べ、NEXT の値と PR 番号と PR の URL を読む。これは既存の chief の例外（同じ段落）の範囲で行うもので、例外の範囲のうち、Claude Code の pane の報告を読む情報源を画面から transcript に替える（AC-15）。理由は既存の例外と同じで、transcript は Claude Code とエージェントが書く外部の値であり、chief が型で表すことはできないためである。境界も既存の例外と同じで、合わない形は広く解釈せず、報告がないか読めないものとし、送らないか空にしない。
-   外部のモジュールは足さない（CLAUDE.md の「Dependencies」）。

## 6. 用語集 (Glossary)

用語は [docs/translation_glossary.md](../../translation_glossary.md) と 0088 要件定義書の 6 章に合わせる。本書で足すか、意味を絞る語は次のとおり。

-   **transcript**: Claude Code がセッションごとに書く、1 行に 1 つの JSON のオブジェクトを並べた記録（`<設定のディレクトリ>/projects/<作業ディレクトリから作った名前>/<セッション ID>.jsonl`、`internal/chief/transcript.go:53-70`）。
-   **最後の報告**: pane のエージェントが最後に終えた応答の本文。Claude Code の pane では transcript から（F-001）、OpenCode の pane では画面から読む。
-   **終わった応答**: エージェントがその応答を書き終え、人の入力を待つ状態に戻ったときの応答。途中で止められた応答と、ツールを呼ぶために中断した応答は含まない。
