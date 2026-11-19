# 要件定義書：chief は PR のマージコンフリクトを見つけて `/mergemain` で解消させ、解消できなければ人の判断を求める

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

**目的:** 本書は、次の要件を定義する。

-   chief の待ち受けの間に、開いている PR が GitHub でマージコンフリクトになったとき、chief がそれを見つけて状況の表に示す。
-   chief が、その workspace の Claude Code へ新しいコマンド `/mergemain <PR>` を自動で送る。`/mergemain` は、`origin/main` を merge で取り込んで衝突を解消し、検査を通して push する。
-   解消できなかったとき、`/mergemain` は作業の前の状態に戻して報告する。chief は、その行を人の判断を待つものとして示す。

**背景:**

今は、コンフリクトに気づいた人が、その workspace のエージェントに解消を自由な文で指示している。コンフリクト中の PR では GitHub Actions の CI が走らないので、気づくのが遅れると、レビューとマージも遅れる。今の chief と `/runpr` の振る舞いは次のとおりである。

1.  chief はコンフリクトを見ていない。ラウンドごとの `gh pr list` は `headRefName,number,title,url` だけを読み（`internal/chief/workspace.go:143`）、Go のコードのどこも `mergeable` を読まない。開いている PR があり、エージェントが動いても質問で止まってもいなければ、行は必ず「レビュー待ち」になる（`internal/chief/state.go:503`、`internal/chief/table.go:217`）。
2.  chief が送る入力は、型で閉じた範囲に限られる。送れるのは、コマンドの種類 5 つ（`internal/chief/paneinput/paneinput.go:92-107`）と、`/clear`・`/model`・`/new`、選択肢を選ぶ入力だけである。この範囲は ADR-0046 の「送る入力と送り先」と、[0088 要件定義書](../0088_chief_of_staff/01_requirements.md) の F-006（AC-31）が決めている。自由な文は送れない。
3.  送り先のエージェントとモデルは、`_context.md` の「Command routing」の表から読む。この表は 6 行がそれぞれ 1 回ずつあることを求め（`.claude/commands/_context.md:157-167`）、chief は行の欠けも、知らない行も拒む（`internal/chief/routing.go:93-120`）。
4.  chief が人に尋ねずに送る自動送信は、マージの後の次のコマンド（`/mkarch`・`/mkplan`・`/runplan`）だけである（0088 要件定義書の AC-41・AC-47、`internal/chief/autosend.go:97-237`）。その記録 `autosend-<PR>.json` は、PR 1 件につき 1 回だけ作られる（`internal/chief/records.go:1291-1325`）。
5.  chief は、Claude Code の pane へコマンドを送る前に、必ず `/clear` と `/model` を送る（`internal/chief/send.go:403-434`）。そのため、pane に残っていた前のコマンドの報告は消える。報告の控え（`report-<PR>-<head>.json`）は、人が PR の詳細を開いたときにしか作られない（`internal/chief/detail.go:520-563`）。
6.  chief が transcript から読む報告の形は、`/runpr` の NEXT の行と、PR を作ったコマンドの報告の 2 つだけである（`internal/chief/screen_report.go:12-63`、`internal/chief/screen_transcript_report.go:12-50`）。失敗を表す報告の形はない。
7.  `/runpr` には、コンフリクトを解消する明示の段がない。「Taking in `main`」は、`main` を取り込んだときの README と ADR の扱いだけを決めている（`.claude/commands/runpr.md:272-279`）。
8.  待ち受けの `chief wait` は、Herdr の状態の変化でだけ終わる（`internal/chief/wait.go:66-82`）。GitHub の状態の変化では終わらないので、PR がコンフリクトになっても、chief が気づくのは次のラウンドで、最長で `wait` の制限時間（既定 10 分、`internal/chief/wait.go:24-28`）の後になる。

対応 issue: [#1019](https://github.com/isseis/yt2column/issues/1019)（関連: [#931](https://github.com/isseis/yt2column/issues/931)（workspace 間の衝突の調停。その中の「ほかの PR のマージの後に conflict が起き、CI が走らなくなる」を本タスクに切り出した）、[#836](https://github.com/isseis/yt2column/issues/836)（タスク 0088。chief の送る入力の範囲と自動送信））

本書は、コンフリクトの読み取りと表示、`/mergemain` を自動で送る条件と人が選んで送る条件、送り直しの規則、`/mergemain` が解消してよい衝突と止まる場合、止まったときの戻し方と報告の項目、送り先を決める。次のことは `02_architecture.md` が決める（[design_handoff.md](design_handoff.md)）。

-   GitHub から `mergeable` を読む呼び出しの形。
-   送った記録の形。
-   衝突の種類の判定を道具とエージェントのどちらが行うか。
-   戻し方の手順。
-   表の語と状態の詳細の文言。
-   道具のプロトコル番号。

### issue #1019 で決めたこと

-   chief は、開いている PR の GitHub の `mergeable` が `CONFLICTING` のとき、それをコンフリクトとして表に示す。`UNKNOWN`（GitHub がまだ計算していない）はコンフリクトとして扱わず、次のラウンドで読み直す（F-001）。 GitHub が計算を終えていない値から推し量って送らないためである。
    -   ADR：[ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（下の「要件定義の際に決めたこと」の ADR の項を参照）。
-   解消のコマンドは、PR のブランチに `origin/main` を merge で取り込み、rebase はしない。取り込みには既存の「Taking in `main`」の規則（README の同期、ADR の番号、プロトコル番号）を当て、`make test`・`make lint` を通してから push し、決まった項目で報告する（F-005）。 履歴を書き換えずに済み、CLAUDE.md の「Protocol numbers」が、プロトコル番号を上げたブランチに `main` を merge で取り込むよう求めているためである。
    -   ADR：ADR-0046 の同じ更新に含める。
-   自動送信の引き金は GitHub の状態（`CONFLICTING`）で、画面ではない。画面と transcript は、送ることを止める向きにだけ働く（F-002）。 ADR-0046 の原則（人に尋ねずに入力を送る判断を、信頼できない文字列に委ねない）と同じにするためである。
    -   ADR：ADR-0046 の同じ更新に含める。
-   解消できない場合（検査が通らない、決めた種類以外の衝突がある、前提が合わない）は、コマンドが作業の前の状態に戻し、理由の種類と衝突したファイルを報告して止まる。Go・TypeScript のロジックの衝突は止める（F-005）。 誤った解消を push しないためと、最初は狭く始めて実績を見て広げるためである。
    -   ADR：ADR-0046 の同じ更新に含める。
-   送ったことを記録し、同じ条件で 2 回送らない（F-002）。 送ったコマンドが失敗しても、同じものを繰り返し送らないためである。記録の単位は、要件定義で「PR の head ごと」に決めた（下）。
    -   ADR：ADR-0046 の同じ更新に含める。
-   解消のコミットは PR のブランチに push されるだけで、その後の `/runpr` のレビューと、マージの確認での人の判断を必ず通る。自動にするのは解消の作業だけで、マージの判断は人に残す（2.3）。 ADR-0046 の「chief は人の代わりに判断しない」を保つためである。
    -   ADR：ADR-0046 の同じ更新に含める。

### 要件定義の際に決めたこと（2026-11-19）

-   進め方は通常の進め方（`full`）とする。除外の判断：この変更は簡易な進め方をとれない変更（除外される変更）に当たる（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。`docs/dev/security.md` の規則と前提は変えず、外部依存も足さない。`fixprguard`・`codexreview`・`mergepr` も変えない。しかし、chief が人に尋ねずにほかのエージェントへ送る入力を増やすので、開発の道具の信頼の境界（ADR-0046 の「背景」、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）に触れる。issue 本文も通常の進め方を指定している。 人の答え：通常の進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   `/mergemain` が解消してよい衝突は、両側が内容を変えた衝突のうち、次の 2 つに限る。どちらでも、Go・TypeScript のロジックの衝突は止める（AC-22・AC-23）。
    1.  `.md` のファイルの衝突。対象は、`CLAUDE.md` と `.claude/commands/` の下を含む、すべての `.md` である。
    2.  プロトコル番号の宣言の行（CLAUDE.md の「Protocol numbers」の `const Protocol = <number>`）だけの衝突。

    ほかの文書の衝突は、エージェントが両側の変更を保てると判断したときだけ解消し、保てないと判断すれば止める。 文書の衝突は多くのタスクが同時に変える `CLAUDE.md` やコマンドの定義でも起き、そこまで止めると効果が小さいためである。解消の結果は `/runpr` と人の確認を通る。既存の規則があるファイルだけに限る案と、検査が通れば何でも解消する案は採らなかった。 人の答え：文書の衝突まで。`.md` すべて（指示書を含む）。
    -   ADR：ADR-0046 を更新する。chief が自動で送る入力が、エージェントに何を任せるかを決めるためである。
-   `.md` でも、片側が消して他方が変えた衝突（modify/delete）、名前を変えた衝突、内容以外の衝突は止める（AC-23）。 これらには「両側の変更を保つ」という判断が成り立たないためである。 人の答え：止める。
    -   ADR：ADR-0046 の同じ更新に含める。
-   解消できなかったことは、chief が GitHub の状態から判定する。送った後、送り終えた記録があり、workspace のエージェントがすべて入力を待っていて、PR が同じ head のまま `CONFLICTING` なら、「解消できず」として人の判断を待つ行にする。理由の種類と衝突したファイルは `/mergemain` の報告に書き、人が pane で読む。chief は報告を読まない（F-003）。 これなら transcript の読み取りの例外（CLAUDE.md の「Declare, don't infer」）を広げずに済み、引き金を GitHub の状態に置く原則とも合う。報告の形を足して chief が transcript から理由を読む案は、読み取りの例外が広がるので採らなかった。 人の答え：GitHub の状態で判定する。
    -   ADR：ADR-0046 の同じ更新に含める。
-   自動送信は、PR の head のコミットごとに 1 回までとする。失敗すると head は動かないので、人が手を入れて head が変わるまで、自動では送り直さない（AC-08・AC-14）。 issue 本文の案（head と `origin/main` の組ごとに 1 回）は、`main` が進むたびにやり直し、続けて失敗する回数の上限も別に決めることになるので採らなかった。 人の答え：PR の head ごとに 1 回。
    -   ADR：ADR-0046 の同じ更新に含める。
-   `/runpr` が実行中のときは、chief は `/mergemain` を送らない。`/runpr` の手順は変えない。`/runpr` が終わった後も `CONFLICTING` なら、chief が送る（AC-10、2.3）。 `/runpr` の中で `main` を取り込むときは、今の「Taking in `main`」の規則に従うので足りる。`/runpr` に解消の段を足す案は、範囲が広がるので採らなかった。 人の答え：送らず、`/runpr` は変えない。
    -   ADR：ADR-0046 の同じ更新に含める。
-   解消のコミットのレビューは、`/runpr` の今の規則（PR レビューの再依頼の判定を含む）に任せる。`/mergemain` はローカルの CLI レビューを走らせず、報告に、解消したファイルとその解消の仕方を書く（AC-27）。 CodeRabbit と Codex の allowance を使わずに済み、解消の結果は `/runpr` と人の確認を通るためである。`git diff --cc` の範囲をローカルの CLI レビューにかける案は採らなかった。merge コミットを再依頼の判定がどう数えるかは、設計で確かめる（design_handoff.md の H-05）。 人の答え：`/runpr` の今の規則に任せる。
    -   ADR：ADR-0046 の同じ更新に含める。
-   新しいコマンドの名前は `/mergemain` とする。 merge で取り込むことが名前から分かり、issue の仮の名前 `/syncmain` は rebase とも読めるためである。 人の答え：`/mergemain`。
    -   ADR：なし。名前は本タスクの中で閉じる。
-   自動送信は、chief の待ち受け（`tick`）のラウンドで `CONFLICTING` を読めば送る。状態が変わるところを見る必要はない。`once`（`snapshot`）では送らず、提案として示す（AC-05〜AC-07）。 マージと違い、コンフリクトは今の状態そのものが解消すべきものなので、chief が動いていなかった間に起きたコンフリクトでも送ってよいためである。前のラウンドで `CONFLICTING` でなかった PR だけに送る案は採らなかった。 人の答え：待ち受けで `CONFLICTING` を読めば送る。
    -   ADR：ADR-0046 の同じ更新に含める。
-   コンフリクトの行では、人が `/mergemain <PR>` を選んで送れるようにする。自動送信が失敗した後でも、`once` でも、送れる（F-004）。 人が直してからもう一度試させる手段を、pane へ移らずに使えるようにするためである。 人の答え：送れるようにする。
    -   ADR：ADR-0046 の同じ更新に含める。
-   コンフリクトの行と「解消できず」の行の急ぎの度合いは、レビュー待ちと同じ「あとで 🔶」とする（AC-02・AC-14）。 エージェントが入力を待って止まっているわけではないためである。「すぐ 🛑」にする案は採らなかった。 人の答え：あとで 🔶。
    -   ADR：なし。表の印の割り当ては本タスクの中で閉じる。
-   `/mergemain` の送り先は Claude Code・Opus とし、`_context.md` の送り先の表に行を 1 つ足す（F-006）。 文書の衝突の判断に Opus を使うためである。Sonnet にする案は、判断の質が下がりうるので採らなかった。 人の答え：Claude Code・Opus。
    -   ADR：ADR-0046 を更新する。コマンドごとの送り先は ADR-0046 の決定である。
-   手元で `origin/main` を取り込んで衝突しなかったときは、取り込んで検査を通し、push する（AC-21）。 GitHub の `mergeable` が古いままのときや、送るまでの間に `main` が変わったときにも、PR を最新の `main` に追いつかせ、GitHub に判定を計算し直させるためである。変えずに止まる案は採らなかった。 人の答え：取り込んで push する。
    -   ADR：なし。`/mergemain` の手順の中で閉じる。
-   送る前に、chief は報告の控えを作れれば作る。pane の最後の報告がこの PR の報告として読めなければ、控えなしで送る。その場合、PR の詳細は今の後戻りの経路と同じく PR の説明から要約する（AC-12）。 issue 本文の案（控えを作れなければ送らない）では、今の控えが人が PR の詳細を開いたときにしか作られないので、多くの場合に自動送信が止まるためである。 人の答え：作れれば作り、なくても送る。
    -   ADR：ADR-0046 の同じ更新に含める（記録の使い道は変えない）。
-   自動送信の条件に、Claude Code の pane で、人の最後のタスクのコマンドの後に人の入力がないことを入れる。表の `＋指示` と同じ考え方である。transcript を読めないときも送らない（AC-05 の (4)）。 送る前の `/clear` で、人が pane で会話した文脈を黙って消さないためである。 人の答え：それでよい。
    -   ADR：ADR-0046 の同じ更新に含める。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   コンフリクト中の PR を、人が気づく前に chief が見つけて表に示す。
-   決めた種類の衝突は、人が指示を書かなくても、chief が送る `/mergemain` で解消される。
-   解消できなかったときは、作業の前の状態に戻ったうえで、人が判断すべきものとして表に示される。

### 2.2. スコープ (In Scope)

-   chief：開いている PR の `mergeable` の読み取り、表の状態と急ぎの度合い、`/mergemain` の自動送信と人が選ぶ送信、送った記録、送る前の報告の控え。
-   新しいコマンドの定義 `.claude/commands/mergemain.md`。
-   `_context.md` の「Command routing」の表への行の追加と、表の行数の記述。
-   chief の文書（`chief.md`・`docs/dev/guides/chief_guide.md`）、CLAUDE.md のコマンドの一覧、ADR-0046 の更新。

### 2.3. スコープ外 (Out of Scope)

-   `/runpr` の手順は変えない。`/runpr` が実行中にコンフリクトになった場合は、今の「Taking in `main`」の規則で `/runpr` 自身が扱う。 要件定義の際に決めた（1 章）。
-   マージの判断と PR のレビューは自動にしない。`/mergemain` は `gh pr merge` を実行せず、PR レビューを依頼しない。 issue #1019 で決めた（1 章）。
-   Go・TypeScript のロジック、`.md` 以外のファイル（プロトコル番号の行を除く）、内容以外の衝突の解消。 最初は狭く始めるためである（1 章）。
-   chief が `main` へのマージを見た直後に、ほかの開いている PR の `mergeable` を読み直して早く気づく仕組み、および GitHub の状態の変化で `chief wait` を終わらせる仕組み。 検出は次のラウンドで足り、待ち受けの引き金を増やす仕組みは別の設計になるためである。必要かどうかは設計で確かめる（design_handoff.md の H-02）。
-   #931 のほかの衝突（ADR 番号やタスク番号の重なり、共有ファイルの同時の変更の調停）。 #931 で扱う。
-   失敗の理由の種類を chief が読んで表に示すこと。 要件定義の際に決めた（1 章）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **マージコンフリクト（コンフリクト）**：GitHub が、開いている PR の `mergeable` を `CONFLICTING` と返す状態。PR の状態を指す。
-   **衝突**：`/mergemain` が手元で `origin/main` を取り込んだときに、git が解消できずに残したファイルの中の箇所。ファイルの状態を指す。
-   **内容の衝突**：両側がファイルの内容を変えたか、両側が同じパスにファイルを足したことによる衝突。片側が消して他方が変えた衝突（modify/delete）、名前の変更による衝突、ファイルの種類の衝突は、内容の衝突ではない。
-   **PR の head**：GitHub が示す PR の head ブランチの先頭のコミット。
-   **コンフリクトの行**：状況の表で、開いている PR が `CONFLICTING` と読まれた workspace の行。

### 3.2. 機能一覧

#### F-001: コンフリクトの読み取りと表示

chief は、ラウンドごとに、各 workspace の開いている PR の `mergeable` を読み、コンフリクトを表に示す。

**Acceptance Criteria**:
-   **AC-01**: chief は、ラウンドごとに（`tick` でも `snapshot` でも）、行に示す開いている PR の GitHub の `mergeable` を読む。
-   **AC-02**: 開いている PR が `CONFLICTING` で、その workspace のどのエージェントも実行中でも質問で止まってもいなければ、行の状態の文字列は「レビュー待ち」ではなく、コンフリクトを示す文字列になり、急ぎの度合いは「あとで 🔶」になる。実行中と質問の行の扱いは今のまま変わらない。
-   **AC-03**: `mergeable` が `MERGEABLE` か `UNKNOWN` のとき、行は今と同じ規則で決まる（エージェントが止まっていれば「レビュー待ち」）。`UNKNOWN` の PR は、次のラウンドで読み直す。
-   **AC-04**: 読み取った `mergeable` は信頼できない入力として扱い、受け付ける形を次のとおりとする。
    -   受け付ける値は、JSON の文字列の `MERGEABLE`・`CONFLICTING`・`UNKNOWN` のいずれかで、バイト列として完全に一致するものだけとする。
    -   次の場合は、その行の PR の状態を読めないものとし、行を「要確認」にして、その workspace へ `/mergemain` を送らない。今の PR の一覧の読み取りの失敗と同じ扱いである。
        -   メンバーがない。
        -   `null` である。
        -   文字列でない。
        -   大文字と小文字だけが違う値や、前後に空白がある値を含め、3 つのどれとも違う文字列である。
        -   同じメンバーが 2 回ある。
    -   PR の一覧の応答のうち本書が消費するメンバーは、今の 4 つ（`headRefName`・`number`・`title`・`url`）と `mergeable` である。そのほかのメンバーの扱いは今の読み取りを変えない。

#### F-002: `/mergemain` の自動送信

chief は、待ち受けの間、コンフリクトの行の workspace の Claude Code へ、人に尋ねずに `/mergemain <PR>` を送る。

**Acceptance Criteria**:
-   **AC-05**: chief は、待ち受け（`tick`）のラウンドで、次のすべてを満たすとき、その workspace の Claude Code の pane へ `/mergemain <PR>` を送る。`<PR>` はその PR の番号である。
    1.  その PR が開いていて、このラウンドで `CONFLICTING` と読まれた。さらに、送る直前に読み直しても、開いていて `CONFLICTING` で、PR の head がこのラウンドで読んだものと同じである。
    2.  その workspace に Claude Code の pane がちょうど 1 つある。
    3.  その workspace のどのエージェントも入力を待っている（実行中でも質問で止まってもいない）。マージの後の自動送信と同じ条件である（`internal/chief/autosend.go:422-439`）。
    4.  Claude Code の pane の transcript を読め、人が最後に入力したタスクのコマンドの後に、人の入力がない。
    5.  その workspace への送信が止まっていない。
    6.  その PR とその PR の head について、`/mergemain` を送った記録がない（AC-08）。
    7.  送り先の pane の入力欄が空で、`/model` で送り先の表のモデルに切り替わったことを確かめられた。今の送信の確かめと同じである。
-   **AC-06**: `once`（`snapshot`）では、chief は `/mergemain` を自動で送らない。コンフリクトの行は、F-004 の選択肢とともに示す。
-   **AC-07**: chief を起動する前からコンフリクトだった PR も、待ち受けのラウンドで AC-05 の条件を満たせば送る。前のラウンドで `CONFLICTING` でなかったことは条件にしない。
-   **AC-08**: 自動送信は、PR とその head のコミットの組ごとに 1 回までとする。送った記録は、最初の入力を送る前に作り、chief を起動し直しても残る。記録を読めないときは、送ったものとして扱い、送らない。
-   **AC-09**: 送れたかが分からない送信は送り直さず、その workspace への送信を、人が解除するまで止める。今の送信の規則と同じである。
-   **AC-10**: `/runpr` を実行中のエージェントがある workspace と、`/runpr` のマージの質問などの質問で止まっているエージェントがある workspace には送らない。`/runpr` が終わり、エージェントがすべて入力を待つ状態になった後も PR が `CONFLICTING` なら、AC-05 の条件で送る。
-   **AC-11**: chief は、`mergeable` が `CONFLICTING` でない PR について、画面や transcript の中身にかかわらず、`/mergemain` を自動で送らない。画面と transcript は、AC-05 の (3)・(4)・(7) のとおり、送ることを止める向きにだけ使う。
-   **AC-12**: 送る前に、Claude Code の pane の最後の報告が、その PR について transcript の PR の記録が名指す報告として読めれば、chief はその報告の控えを作る。その後の PR の詳細は、控えから報告を示す。報告がこの PR の報告として読めなければ、控えを作らずに送る。この場合の PR の詳細は、今の後戻りの経路と同じく PR の説明から要約する。
-   **AC-13**: 送る入力は、ほかの Claude Code へのコマンドと同じく、`/clear`、`/model <送り先の表のモデル>`、`/mergemain <PR>` の順で、それだけである。

#### F-003: 解消できなかったときの表示

`/mergemain` が解消できなかったことを、chief は GitHub の状態から判定し、人の判断を待つものとして示す。

**Acceptance Criteria**:
-   **AC-14**: 次のすべてを満たすとき、chief はその行を「解消できず」として示し、急ぎの度合いを「あとで 🔶」にして、自動では送り直さない。状態の詳細で、まだ送っていないコンフリクトの行と区別できるようにする。
    -   その PR の今の head について `/mergemain` を送った記録があり、その送信は送り終えた記録を持つ。
    -   その workspace のどのエージェントも入力を待っている。
    -   PR がまだ開いていて `CONFLICTING` で、head が送ったときと同じである。
-   **AC-15**: PR の head が変わった後も `CONFLICTING` なら、その行は再び AC-05 の自動送信の対象になる。head が変わる例は、人の push と、`/mergemain` の push の後にまた `main` が進んだ場合である。
-   **AC-16**: PR が `MERGEABLE` か `UNKNOWN` になれば、行は AC-03 のとおりになる。

#### F-004: 人が選んで送る `/mergemain`

コンフリクトの行では、人が `/mergemain <PR>` を選んで送れる。

**Acceptance Criteria**:
-   **AC-17**: chief は、コンフリクトの行（まだ送っていない行と「解消できず」の行）に、`/mergemain <PR>` を送る選択肢を示す。この選択肢は `once` でも示す。
-   **AC-18**: 人がその選択肢を選んだときだけ、chief は送る直前に次をすべて確かめ直し、通れば AC-13 の入力を送る。送る対象は、人が選んだ項目と同じ PR・同じ head である（ADR-0046 の、人が選んだ項目の ID による送信の規則）。
    -   PR が開いていて `CONFLICTING` で、head が人に示したときと同じである。
    -   AC-05 の (2)・(3)・(5)・(7) を満たす。

    AC-05 の (4)（人の後の入力）と (6)（送った記録）は、人が選んだ送信の条件にしない。
-   **AC-19**: 人が選んで送ったときも、その PR とその head について送った記録を作る。その head について、自動ではもう送らない。

#### F-005: コマンド `/mergemain`

`/mergemain <PR>` は、PR のブランチに `origin/main` を merge で取り込み、決めた種類の衝突だけを解消して push する。解消できなければ、作業の前の状態に戻して報告する。

**Acceptance Criteria**:
-   **AC-20**: `/mergemain` は、引数として PR の番号（10 進の数字だけ）を 1 つ受け取る。引数がないか、数字でない文字を含むときは、何も変えずに止まり、前提が合わないことを報告する。
-   **AC-21**: 次のどれかが成り立たないとき、何も変えずに止まり、前提が合わないことと、合わなかった項目を報告する。
    -   PR が開いている。
    -   作業ツリーに、追跡していないファイルを含めて変更がない。
    -   今のブランチが PR の head ブランチである。
    -   `origin` から取得した後の手元の HEAD が、PR の head と同じコミットである。

    すべて成り立てば、`origin` から取得し、`origin/main` を merge で取り込む。rebase も、履歴の書き換えも、force push もしない。衝突が起きなかったときも、merge コミットを作って AC-24 の検査と AC-25 の push に進む。`origin/main` がすでに HEAD の祖先で取り込むものがないときは、何も変えずに止まり、取り込むものがないことを報告する。
-   **AC-22**: 衝突がすべて次のどちらかに当たるときだけ、解消に進む。
    1.  `.md` のファイルの内容の衝突。
    2.  `internal/<name>/` の直下の、`_test.go` でない Go のファイルの内容の衝突で、衝突した箇所がすべてプロトコル番号の宣言の行（`const Protocol = <number>`）だけのもの。

    解消は次の規則に従う。

    -   README（`README.md`・`README.en.md`）は、`mktrans.md` の「Taking in `main`」に従う。
    -   `docs/dev/adr/` が merge base の後に `main` で変わっていれば、ADR は `docs/dev/conventions/adr.md` の「Taking in `main`」に従う。
    -   プロトコル番号は、CLAUDE.md の「Protocol numbers」に従う。値を `origin/main` の値に 1 を足したものにし、文書の `--protocol` も合わせる。
    -   ほかの `.md` は、両側の変更をどちらも保つように解消する。エージェントが、両側の変更を保てない（互いに矛盾する）と判断したときは、止まる。
-   **AC-23**: 衝突のうち 1 つでも AC-22 に当たらないものがあれば、解消せずに止まる。例は次のとおりである。
    -   `.md` 以外のファイルの衝突（プロトコル番号の宣言の行だけの衝突を除く）。
    -   プロトコル番号の宣言の行のほかにも衝突した箇所がある Go のファイル。
    -   modify/delete の衝突。
    -   名前の変更による衝突。
    -   バイナリのファイルの衝突。
-   **AC-24**: 解消した後（衝突が起きなかったときは取り込んだ後）、`make test` と `make lint` を走らせる。解消したファイルに `README.md`・`README.en.md`・`docs/dev/guides/development.md` があれば、`make ext-test` も走らせる。どれかが通らなければ止まる。
-   **AC-25**: 検査が通れば、merge コミットを PR の head ブランチへ push する（force push ではない）。push が拒まれたら止まる。
-   **AC-26**: 止まるとき（AC-20・AC-21 で何も変えずに止まるときを除く）は、PR のブランチと作業ツリーを、コマンドを始める前の状態に戻す。戻した後は、次の状態である。
    -   HEAD が始める前の PR の head のコミットである。
    -   作業ツリーに変更がない。
    -   merge の途中の状態が残っていない。
    -   何も push していない。
-   **AC-27**: 最後の報告に、次を書く。報告は日本語で書く（CLAUDE.md の「Language」）。
    -   結果。「解消した」か「止まった」のどちらか。
    -   止まったとき：理由の種類と、衝突したファイルの一覧。理由の種類は、次のどれか 1 つである。
        -   前提が合わない
        -   取り込むものがない
        -   対象外の衝突
        -   保てない文書の衝突
        -   検査の失敗
        -   push の失敗
    -   解消したとき：次の 2 つ。
        -   衝突したファイルごとに、当てた規則（README・ADR・プロトコル番号・両側の変更を保つ）と、解消の仕方の要約。
        -   push した新しい head のコミット。
-   **AC-28**: `/mergemain` は、次のことをしない。
    -   `gh pr merge` の実行。
    -   PR レビューの依頼。
    -   PR とレビューのスレッドへの書き込み。
    -   ローカルの CLI レビュー。
    -   PR の head ブランチ以外のブランチへの push。
    -   rebase。
-   **AC-29**: `/mergemain` のコマンドの定義は、それを実行することで、その PR についてだけ、次のネットワークの操作を承認するものとする（CLAUDE.md の「Tool Execution Safety」）。
    -   `gh pr view`
    -   `git fetch origin`
    -   PR の head ブランチへの `git push`

#### F-006: 送り先と送る入力の範囲

**Acceptance Criteria**:
-   **AC-30**: `_context.md` の「Command routing」の表に、`/mergemain` の行（エージェント Claude Code、モデル `opus`）を足す。chief は、`/mergemain` の送り先とモデルを、ほかのコマンドと同じく `origin/main` の表から読む。表の行の数の記述（今は「6 行」）も合わせる。表を読めなければ、今の規則のとおり何も送らない。
-   **AC-31**: chief がタスクの workspace へ送る入力の範囲に、`/mergemain <PR>` だけを足す。`git` のコマンド、`gh pr merge`、自由な文、`/weakreview` は、今のとおり送らない。chief の workspace の pane には、今のとおり何も送らない。
-   **AC-32**: chief は、`/mergemain` を OpenCode の pane へ送らない。

#### F-007: 文書

**Acceptance Criteria**:
-   **AC-33**: `docs/dev/guides/chief_guide.md` に、次を書く。
    -   コンフリクトの行と「解消できず」の行の状態の文字列、急ぎの度合い、状態の詳細。
    -   自動送信の条件。
    -   送り直しの規則。
    -   人が選んで送る選択肢。
-   **AC-34**: CLAUDE.md の「Claude Commands」に `/mergemain` を足し、`.claude/commands/chief.md` に、`/mergemain` の選択肢の示し方と送り方を書く。
-   **AC-35**: ADR-0046 の決定（送る入力と送り先、コンフリクトの解消の自動送信）が、本書の決定と合っている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `mergeable` の読み取りで、ラウンドごとの GitHub の呼び出しの回数を、開いている PR の数に比例する以上に増やさない。呼び出しの形は設計で決める（design_handoff.md の H-01）。

### 4.2. セキュリティ (Security)

-   `docs/dev/security.md` を当てる条件（`yt-dlp` の起動、API キー、キャッシュ、LLM API への送信、信頼できない文字列のプロンプトや記事への埋め込み）には当たらないので、条件付きの指針は N/A とする。chief はこれらのどれも新しく行わない。`/mergemain` が Claude Code でリポジトリの内容を扱うのは、ほかのタスクのコマンドと同じである。
-   chief は信頼の境界である（ADR-0046 の「背景」、security.md §6）。本書は次のことを保つ。
    -   送る入力は、型と、GitHub から読んで検証した PR の番号だけから組み立てる。画面、transcript、PR の題名、PR の説明の文字列を、送る入力に入れない（AC-13・AC-31）。
    -   自動送信の引き金は GitHub の状態だけとする（AC-11）。
    -   GitHub の応答の `mergeable` は、AC-04 の形だけを受け付ける。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   同じ PR の同じ head へ、自動で 2 回送らない（AC-08）。送れたかが分からない送信は送り直さない（AC-09）。
-   `/mergemain` は、止まるときに作業の前の状態に戻す（AC-26）。

### 4.4. 互換性 (Compatibility)

-   `_context.md` の送り先の表に行を足すので、表を読む chief と表は同じ PR で変える。足す前の表は新しい chief に、足した後の表は古い chief に拒まれる（`internal/chief/routing.go:93-120`）。利用者は 1 人で、版の互換の仕組みは設けない。

### 4.5. 保守性 (Maintainability)

-   解消してよい衝突の種類は、`/mergemain` のコマンドの定義の 1 か所に置き、実績を見て広げるときにそこを直せば済むようにする。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   本書は、次の決定に例外を足す。
    -   **ADR-0046 の「送る入力と送り先」と、0088 要件定義書の F-006（AC-31）。** chief が送る入力は、`/clear`・`/model`・`/new`、`/mkreq`・`/mkarch`・`/mkplan`・`/runplan`・`/runpr`、選択肢を選ぶ入力だけと決まっている。本書は `/mergemain <PR>` を足す。 コンフリクトの解消を、自由な文ではなく決まった形のコマンドとして送るためである。
    -   **ADR-0046 の「マージの後の自動送信」と、0088 要件定義書の AC-41・AC-47。** 人に尋ねずに送るのは、マージの後の次のコマンドだけと決まっている。本書は、PR が `CONFLICTING` のときの `/mergemain` の自動送信を足す。引き金を GitHub の状態に置く原則と、1 回に限る原則は同じである。 人の判断の要らない解消の作業を自動にし、マージの判断は人に残すためである。
-   CLAUDE.md の「Declare, don't infer」の chief の例外（画面と transcript の読み取り）は広げない。`/mergemain` の報告を chief は読まない（1 章）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **マージコンフリクト（コンフリクト）**・**衝突**・**内容の衝突**・**PR の head**・**コンフリクトの行**：3.1 のとおり。用語集にはまだない。
-   **ラウンド**：用語集のとおり、`/chief` が状況を読み取って示す 1 回分（`snapshot` か `tick` の 1 回）。
