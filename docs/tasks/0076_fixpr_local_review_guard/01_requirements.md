# 要件定義書：/fixpr Phase 4.5 の git の起動と push の前に、ガードツールが git の状態を基準値と比べる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/fixpr` の Phase 4.5（ローカルの CLI レビュー）で行う git の起動、commit、push を、ガードツール `fixprguard` の検査に通すための要件を定義する（[#426](https://github.com/isseis/yt2column/issues/426)）。手順が定める git の起動の前と push の前に、git の状態を基準値（3 章で定義）と比べ、違えば止める。push はガードツールが行う。

**背景:**

`fixprguard` は、`/fixpr` と `/runpr` が信頼できないテキスト（PR のレビューのスレッド）を扱う間の守りとして、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で作られた。`/fixpr` の Phase 4 では、`finish` が build checks と commit を行い、`push` がその commit を push する（`.claude/commands/fixpr.md:636-677`）。`push` は、直前に hooks・config・info・対象のファイル（git が無視するファイルのうち、ツールが監視するもの。6 章）が基準値と同じであること、HEAD の親が基準値の commit（3 章で定義）だけであること、push 先のブランチの OID が基準値と同じであることを確かめ、明示した refspec で push する（`internal/fixprguard/worktree.go:571-589`）。

その後の Phase 4.5 では、セッション自身がローカルの CLI レビューを行う（`.claude/commands/fixpr.md:695-721`）。手順は `.claude/commands/_lib/local-cli-review.md` に従う。

-   レビューの範囲の起点 BASE（基準値とは別のもの）を `git rev-parse` で求める（`.claude/commands/fixpr.md:703-704`）。
-   試行ごとに `git status` と `git rev-parse` を実行し（`local-cli-review.md:70-77`）、CodeRabbit CLI を実行する（同 78 行目）。それが失敗すれば、`codexreview` を経て Codex CLI を実行する（同 102 行目）。試行は 2 回までである（同、手順 6）。
-   指摘を直したら build checks を実行して commit する（同 204 行目）。build checks の `make fmt` は git を起動する（`Makefile:309`）。
-   README の同期の検査を行い（`.claude/commands/fixpr.md:712-716`）、`README.en.md` が変われば commit する。この検査は `git hash-object` や `git diff` を起動する（`.claude/commands/mktrans.md:77`・`160`）。
-   最後に素の `git push` を実行する（`.claude/commands/fixpr.md:718`）。

この間、ガードツールは何も確かめない。issue #426 は、次の 3 つの検査がないことを挙げている。

-   hooks・config・info の、基準値との比較。Phase 4 の `push` が確かめた後に書き換えられた hooks は、`git commit`（pre-commit・commit-msg・post-commit）と `git push`（pre-push）で実行される。git が実行するコマンドを指す config（`core.fsmonitor` や clean filter など）は、上に挙げた git の起動のどれでも実行されうる。
-   変更範囲の検査（`inspect`）。ローカルの CLI レビューの指摘は、修正のフェーズのエージェントが書いた diff から作られる。そのため、信頼できないテキストの影響を間接的に受ける。
-   判定した集合（commit してよいと判定したパスの集合）との一致の確認（stage するパスの限定、commit のツリーの確認）。

タスク 0030 の設計書は、「確認と実行の間のずれ」として、`push` の確認の後に別のプロセスが hooks や config を書き換えることを残余のリスクとした（0030 `02_architecture.md` 5.3）。`docs/dev/guides/fixprguard_guide.md` も、`push` の後の書き換えを検出しないとしている（7.2.4・7.2.5）。しかし、どちらの文書も、同じ fix round の中で、守られていない git の起動・commit・push がその後に続くことには触れていない。切り離されたプロセスが書き換えに成功していれば、書き換えた内容がここで実行される。

`_context.md` の「Guard tool」の行は、`fixpr.md` と `runpr.md` が fix round の commit と push にガードツールを使うとしている（`.claude/commands/_context.md:194`）。[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の決定も、`push` を道具が行うとしている。Phase 4.5 の commit と push は、どちらにも合っていない。

対応 issue: [#426](https://github.com/isseis/yt2column/issues/426)（関連: [#395](https://github.com/isseis/yt2column/issues/395)、タスク `0030_fixpr_untrusted_threads`、タスク `0049_codex_local_review`）

**本書の記述範囲:** 本書が定めるのは、観測できる振る舞い、つまり Phase 4.5 のどの時点で何を確かめ、違えば何をせずに止まり、何を報告するかである。次のことは設計（`02_architecture.md`）で決める。

-   ガードツールの呼び出しの形。新しいサブコマンドを加えるか、既存のサブコマンドに引数を加えるか。
-   比べる時点ごとの呼び出しを、`fixpr.md` と `local-cli-review.md` のどちらに書くか。
-   `--protocol` の番号を上げるか。
-   テストの手段。

要件のレビューで挙がった設計事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #426 で決めたこと:**

-   **完了の条件:** Phase 4.5 の commit と push の前に、少なくとも hooks・config・info を基準値と比べ、違えば止める。文書のガードと `make test`・`make lint` が通る。
-   **ほかの呼び出し元を対象にしない:** `local-cli-review.md` を呼ぶ `/fixpr` 以外のコマンド（`/runpr` の step 1、`/runplan`、`/runissue`、`_lib/task-pr-handoff.md` の Part B）は対象にしない。どれも `begin` の基準値を持たないので、比べる相手がない。ただし、同じ手順を共有するので、手順を変える場合は、その影響を確かめる（F-005）。
-   **文書を合わせる:** `_context.md` の「Guard tool」の行と、タスク 0030 の設計書の 5.2・5.3 を、決めた対応に合わせる。設計書の直し方は、下の「要件定義の際に決めたこと」で決めた。

**要件定義の際に決めたこと（2026-11-09）:**

-   **状態を比べ、push をガードツールで行う。範囲の検査はしない:** issue は、比べるだけの案 1 と、`inspect` → `finish` → `push` に通す案 2 を挙げていた。本書は、案 1 に、ガードツールによる push を加えた形にする（F-001・F-003）。
    -   案 2 を採らない理由は 2 つある。1 つ目は、`inspect` の範囲の規則がスレッドの `path` を元にすることである。Phase 4.5 にはスレッドがないので、新しい範囲の規則を決める必要がある。2 つ目は、`finish` が commit の件名を固定し、本文に `Editorial correction:` の行だけを認めることである（`docs/dev/guides/fixprguard_guide.md` 5 章の `finish`）。このため、ローカルの CLI レビューの記録の行（`local-cli-review.md` の手順 7）や、README の同期の commit を通せない。
    -   push をガードツールで行うのは、ADR-0015 の「`push` も道具が行う」に合わせるためと、push 先の確認と明示した refspec を Phase 4 と同じにするためである。
    -   範囲の検査をしないことは、ADR-0015 の決定のうち「範囲外の変更が出たら、build checks・commit・push の前に、利用者に一覧を示して確かめる」の例外である。この決定は修正のフェーズの変更に当てはまり、Phase 4.5 の修正には当てはまらないものとして、ADR-0015 に書いた。
    -   ADR の判断: [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) を更新した。ガードツールの守りが及ぶ範囲を変える決定であり、`/fixpr` の手順とガードツールの 2 つの構成要素にまたがるためである。
-   **手順が定める git の起動のすべての前に比べる:** commit と push の前だけでなく、Phase 4.5 の手順が定める git の起動（BASE の解決、各試行の最初、`codexreview` の前、README の同期の検査、build checks、stage と commit）の前にも比べる（AC-01）。git が実行するコマンドを指す config が書き換えられていると、それより後の比較では間に合わないためである。2 回目の試行の前の build checks は、セッションが指摘に従って書いたテストのコードを実行する。そのため、build checks の前に比べても、その後に書き換えが起こりうる。比べる回数が増えるので、正当な書き込み（権限の確認で `.claude/settings.local.json` が書き換わるなど）で止まることも増える。その費用は受け入れる（4.3）。ADR の判断: 上の決定と同じく、ADR-0015 の更新に含めた。
-   **判定した集合との一致はスコープ外にする:** issue が挙げた 3 つ目の検査（stage するパスの限定と commit のツリーの確認）は行わない。Phase 4.5 には、どのパスを commit してよいかを、セッションとは別に判定する主体がない。集合はセッションの宣言になるので、それとの一致を確かめても守りは増えないためである。ADR の判断: 上の決定と同じく、ADR-0015 の更新に含めた。
-   **止まった後は報告だけにする:** 止まったとき、Final report に、返信も解決もしていないスレッド、push されていない commit、ローカルの CLI レビューを受けていない範囲などを示し、復旧は利用者に任せる（AC-05）。次の `/fixpr` の `begin` が、push されていない commit を見つけて止まるようにはしない。`begin` の振る舞いを変えると範囲が広がるためである。ADR の判断: ADR にしない。報告の内容は本タスクの中で閉じる決定だからである。
-   **issue を超えて足す 4 つの要件:** 次の 4 つを、issue の求めに加える。どれも安全側に倒す追加である。ADR の判断: ADR-0015 の更新に含めた。
    -   各比較で、今のブランチが基準値のブランチであることと、HEAD の系列を確かめる（AC-02）。基準値は Phase 4 の commit の前の状態を記録している。そのため、ブランチを切り替えられても、別の commit を混ぜられても、hooks などの比較だけでは気づけないためである。
    -   hooks・config・info に加えて、対象のファイル（`.claude/settings.local.json` など）も比べる（AC-02）。Phase 4 の `push` も比べており、比べ方を `push` と共有するためである。
    -   インストール済みのツールが古いときは、検査を飛ばさずに止まる（AC-10）。古いツールで黙って検査がなくなると、本タスクの守りが利用者に分からないまま失われるためである。
    -   Phase 4 の commit は、セッションが渡す値を信じず、ツール自身が git の状態から求めて確かめる（AC-02・AC-07）。セッションは信頼できないテキストの影響を受けうるので、セッションが渡す値で検査が緩んではならないためである。
-   **タスク 0030 の設計書には注記だけを足す:** 今の仕様は `docs/dev/guides/fixprguard_guide.md` と `.claude/commands/_context.md` に書く。タスク 0030 の設計書（承認済み）には、5.2 と 5.3 に本タスクへの注記を足すだけにし、本文は書き換えない（AC-13）。完了したタスクの記録を残したまま、今の仕様の置き場所を 1 か所に保つためである。タスク 0057 が 0030 の設計書の 5.3 に足した注記と同じ形にする。ADR の判断: ADR にしない。どの文書に書くかの選択で、本タスクの中で閉じる決定だからである。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Phase 4 の `push` の確認の後に hooks・config・info・対象のファイルが書き換えられた場合、Phase 4.5 の手順が git を起動する前に止める。
-   Phase 4.5 の push を、Phase 4 と同じ確認と明示した refspec で行う。
-   `_context.md`、`fixprguard_guide.md`、ADR-0015 の記述を、ガードツールが実際に守る範囲と一致させる。

### 2.2. スコープ (In Scope)

-   `/fixpr` の Phase 4.5 の手順が定める git の起動の前の比較（F-001）。
-   止まったときの振る舞いと報告（F-002）。
-   Phase 4.5 の push をガードツールで行うこと（F-003）。
-   インストール済みのツールが古いときの振る舞い（F-004）。
-   `local-cli-review.md` を共有するほかの呼び出し元の振る舞いを変えないこと（F-005）。
-   `fixpr.md`、`_context.md`、`fixprguard_guide.md`、ADR-0015、タスク 0030 の設計書の注記（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **Phase 4.5 の修正の範囲の検査と、判定した集合との一致の確認。** 1 章の決定のとおり行わない。Phase 4.5 の修正が範囲外のファイルを変えても、利用者への確認はない。残余のリスクとして 4.2 に記録する。
-   **`local-cli-review.md` を呼ぶ、`/fixpr` 以外のコマンド。** issue #426 のとおり対象にしない（1 章）。
-   **Phase 5・6 で基準値を持たないサブコマンド（`reply`・`edit-pr`・`append-pr` など）の起動前の比較。** Phase 4.5 の push の後の書き換えは、今までの `push` の後と同じく検出しない（`fixprguard_guide.md` 7.2.4）。issue #426 が求めていないためである。
-   **手順が定めない、セッション自身の git の起動。** セッションがコードを読むために起動する `git diff` や `git log` などは、守られない。守るのは、手順が定める git の起動だけである。
-   **`begin` で、push されていない commit を見つけて止まること。** 1 章の決定のとおり行わない。
-   **タスク 0030 の設計書の 5.2・5.3 の本文の書き換え。** 1 章の決定のとおり、注記だけを足す。

## 3. 機能要件 (Functional Requirements)

本章では、次の言葉を使う。

-   **基準値:** その回の `/fixpr` の `begin` が出力し、`inspect`・`finish`・`push` に `--baseline` で渡す値。そのときの git の状態（HEAD、ブランチ、push 先の OID、hooks、config、info、対象のファイル）を記録している。
-   **基準値の commit:** 基準値が記録する HEAD の commit。
-   **Phase 4 の commit:** HEAD から親をたどった列のうち、基準値の commit の直後にある commit。その回の Phase 4 で `finish` が作り、`push` が push した commit に当たる。
-   **1 本につながる:** Phase 4 の commit から HEAD までの各 commit が、親をちょうど 1 つ持ち、その親が列の 1 つ前の commit であること。Phase 4 の commit 自身の親は、基準値の commit だけである。
-   **比べる:** 次の 3 つを確かめること。
    -   (a) Phase 4 の `push` が直前に比べる git の状態のうち、HEAD と push 先の OID を除くすべてが基準値と同じであること。hooks・config・info・対象のファイルを含む。比べ方（何を同じとみなし、何を比べないか）も `push` と同じである（`docs/dev/guides/fixprguard_guide.md` 5 章の `inspect`・`push`、7.1）。
    -   (b) 今のブランチが基準値のブランチであること。
    -   (c) HEAD から基準値の commit までが 1 本につながること。

### 3.1. 機能一覧

#### F-001: Phase 4.5 の手順が定める git の起動の前の比較

Phase 4.5 は、手順が定める git の起動の前に比べる。Phase 4 の `push` の後に書き換えられた設定が、その git の起動で実行される前に止めるためである。Phase 4 が commit を作らなかったときは、今までどおり Phase 4.5 を飛ばし（`.claude/commands/fixpr.md:697`）、比べもしない。

**Acceptance Criteria**:

-   **AC-01**: Phase 4.5 は、次の各操作の直前に比べる。
    -   BASE を求める `git rev-parse`。
    -   各試行の最初の `git status`。続く CodeRabbit CLI も、この比較の後に実行する。2 回目の試行でも同じく比べる。
    -   CodeRabbit CLI が失敗したときに `codexreview` の前に起動する `git rev-parse`（`local-cli-review.md` の手順 3）。続く `codexreview` も、この比較の後に実行する。
    -   README の同期の検査。Phase 4.5 の終わりの検査と、修正が `README.md` を変えたときの検査（`local-cli-review.md` の手順 5）の両方である。
    -   修正の build checks。
    -   修正の stage と commit。README の同期の `README.en.md` の stage と commit を含む。stage から commit までの間に、ほかの git の起動を挟まない。
-   **AC-02**: AC-01 の比較で、(a)・(b)・(c) のどれかに合わなければ、その時点の git の起動をしない（F-002）。(c) で使う Phase 4 の commit は、ツールが git の状態から求める。呼び出し元が commit を渡す場合も、その値によって (c) の判定は緩まない。
-   **AC-03**: 比較に通ったとき、Phase 4.5 の commit のメッセージは今までと同じである。ローカルの CLI レビューの修正の commit は、手順が定める記録の行を持つ（`local-cli-review.md` の手順 7）。README の同期の commit は、今までどおり、その commit のためのメッセージを持つ。

#### F-002: 止まったときの振る舞いと報告

比較か push の確認に通らなければ、Phase 4.5 は止まる。止まった時点より後のことは何もせず、作業ツリーとローカルの commit を残す。

**Acceptance Criteria**:

-   **AC-04**: 止まったとき、Phase 4.5 は、止まった時点の git の起動と、それより後の build checks・stage・commit・push をしない。Phase 5 と Phase 6 も行わない。作業ツリーの修正と、それまでに Phase 4.5 が作った commit を捨てない。
-   **AC-05**: 止まったとき、Final report は次のことを示す。
    -   止まった時点（AC-01 のどれか、または push の前）。
    -   止まった種類（`stop`）と、違っていた部分（`parts`）。(b) か (c) に合わないとき、または push 先の OID が合わないときも、どれに合わなかったかが分かる。対象のファイルの違いで止まったときは、既存のサブコマンドと同じく、止まった時点の記録を残し、`fixprguard_guide.md` 8.1 の手順で変わったファイルを調べられる。
    -   Phase 4 の commit が push 済みであること。
    -   返信も解決もしていないスレッドの一覧。
    -   push されていない Phase 4.5 の commit の数。
    -   ローカルの CLI レビューを受けていない範囲。手順が定める `Local CLI review: not run` の行の形で示す（`local-cli-review.md` の手順 7）。
    -   Phase 6 で PR の説明に足していない編集上の訂正。
    -   復旧のために利用者が確かめること。作業ツリーに変更が残るとき、次の `/fixpr` は `dirty-work-tree` で止まる。push されていない commit が残るとき、次の `/fixpr` の Phase 4 は、その回に直す指摘がなければ失敗し（`.claude/commands/fixpr.md:682-688`）、直す指摘があれば、`push` がその commit も一緒に push する。止まった種類が対象のファイルなら、`fixprguard_guide.md` 8.1 の手順に従う。

#### F-003: Phase 4.5 の push をガードツールで行う

Phase 4.5 が commit を作ったとき、その push はガードツールが行う。セッションは `git push` を実行しない。

**Acceptance Criteria**:

-   **AC-06**: Phase 4.5 が commit を作ったとき、その push はガードツールが行う。`fixpr.md` の Phase 4.5 は、セッションに `git push` を実行させない。
-   **AC-07**: ガードツールは、push の直前に比べ、さらに push 先のブランチの OID が Phase 4 の commit であることを確かめる。どれかに合わなければ、push せずに止まる（F-002）。
-   **AC-08**: ガードツールは、Phase 4 の `push` と同じく、明示した refspec で HEAD を push 先のブランチに送り、タグとサブモジュールを伴わない。
-   **AC-09**: Phase 4.5 が commit を作らなかったときは、push もその確認もしない。今までどおり Phase 5 に進む。

#### F-004: 古いツールでは検査を飛ばさない

インストール済みの `fixprguard` が、本タスクの検査に対応していないことがある（`make install-fixprguard` を、本タスクのマージの後に実行していない場合）。そのとき、Phase 4.5 は検査なしに進まない。

**Acceptance Criteria**:

-   **AC-10**: インストール済みのツールが本タスクの検査に対応していないとき、`/fixpr` は、Phase 4.5 で比較なしに git を起動せず、commit も push もしない。止まって、最新の `main` から `make install-fixprguard` を実行するよう利用者に示す。止まる時点が Phase 4.5 より前でもよい。Phase 4.5 の中で止まったときは、AC-05 の報告も行う。

#### F-005: ほかの呼び出し元の振る舞いを変えない

`local-cli-review.md` を呼ぶ `/fixpr` 以外のコマンドは、本タスクの後も今までと同じに振る舞う。`/runpr` は、スレッドを処理するときに `fixpr.md` を実行する（`.claude/commands/runpr.md:639`）。その中の Phase 4.5 は本書に従う。

**Acceptance Criteria**:

-   **AC-11**: `/runpr` の step 1、`/runplan`、`/runissue`、`_lib/task-pr-handoff.md` の Part B が行うローカルの CLI レビューは、本タスクの後もガードツールを呼ばず、今までと同じ手順で commit と push をする。

#### F-006: 文書

ガードツールが守る範囲の記述を、本タスクの後の実際と一致させる。

**Acceptance Criteria**:

-   **AC-12**: 次の文書が、本タスクの後の振る舞いを述べる。
    -   `.claude/commands/fixpr.md` の Phase 4.5、その許可の段落（`git push` を Phase 4.5 が行うとする記述）、Final report の「Stops to act on」「Editorial corrections」「Local CLI review」の項目。
    -   `.claude/commands/_lib/local-cli-review.md`。設計が比較の呼び出しをここに書く場合だけである。
    -   `.claude/commands/_context.md` の「Guard tool」の行。Phase 4.5 の commit が比較だけを受け、範囲の検査を受けないことを含む。
    -   `docs/dev/guides/fixprguard_guide.md`。サブコマンドか引数を加えるなら 5 章に、守ることを 7.1 に、守らないこと（Phase 4.5 の修正の範囲の検査がないこと、比較が時点だけの確認であること、手順が定めない git の起動）を 7.2 に書く。
    -   ADR-0015 の決定。本書の PR が加えた「まだ反映されていない。#426 で扱う」という旨の記述を、実装の PR が取り除く。
-   **AC-13**: タスク 0030 の設計書の 5.2 と 5.3 に、本タスクへのリンクを持つ注記がある。両節の本文は変わらない。
-   **AC-14**: 文書のガード（`cmd/fixprguard/docs_test.go` など）を含む `make test` と `make lint` が通る。issue #426 の完了の条件である。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比較は Phase 4 の `push` と同じもので、1 回の Phase 4.5 で多くても十数回である。ローカルの CLI レビュー（数十秒から数分）に比べて小さい。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の適用: N/A。その適用の条件のうちネットワークへの送信は、製品が LLM の API と Webhook へ送るものを指す。本タスクの push は、fix round がすでに行っている git の操作であり、製品の送信ではない。`yt-dlp` の起動、API キーや Webhook の URL、キャッシュのディレクトリ、プロンプトへの信頼できないテキストの埋め込みにも関わらない。
-   信頼できない入力の境界: 新しい境界は加えない。ガードツールが読むのは、今までと同じ git の状態である。Phase 4 の commit は、呼び出し元が渡す値に頼らず、ツールが git の状態から求める（AC-02）。呼び出し元が引数を渡す設計にする場合、その値の受け付ける形と拒否の振る舞いは、`fixprguard_guide.md` 4 章の使い方の誤り（終了コード `2`）の規則に従う。ローカルの CLI レビューの指摘は、今までどおり `local-cli-review.md` の手順 4 の扱いを受ける。
-   **残余のリスク**（本タスクの後も残るもの）:
    -   **Phase 4.5 の修正は範囲の検査を受けない。** Phase 4.5 の修正は、信頼できないテキストの影響を間接的に受けた指摘に従って、セッションが行う。範囲外のファイル（実行関連ファイルを含む）を変えても、利用者への確認なしに build checks で実行され、commit・push される。比較が検出するのは、git の状態の書き換えだけである。1 章の決定で受け入れる。
    -   **比較は時点だけの確認である。** 比較の後、git の起動・commit・push が行われるまでの間に、別のプロセスが書き換えることは防げない。0030 の設計書 5.3 の「確認と実行の間のずれ」と同じである。本タスクは、このずれの長さを、Phase 4 の `push` から Phase 4.5 の終わりまでの間から、各時点の直前の短い間へと縮める。
    -   **手順が定めない git の起動。** 2.3 のとおり、セッションが自分の判断で起動する git は守られない。
    -   **比較を呼ぶことは文書に依存する。** Phase 4.5 で比較を呼ぶこと、`git push` を実行しないことは、コマンドの文書に従うセッションに依存する（`fixprguard_guide.md` 7.2.2 と同じ）。
    -   **Phase 4.5 の push の後の書き換え。** Phase 5・6 の `reply`・`edit-pr`・`append-pr` などは、今までどおり `gh` の起動の前に比べない（2.3）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   止まったとき、作業ツリーの修正と、push されていない commit を捨てない（AC-04）。利用者が中身を確かめてから、やり直せるようにする。
-   ガードツールの終了コードのうち、続けてよいもの以外（読めない出力や信号による終了を含む）は、すべて止まるものとして扱う（`fixprguard_guide.md` 4 章と同じ）。
-   比べる時点が増え、Phase 4.5 は数分かかるので、正当な書き込みでも止まりやすくなる。たとえば、権限の確認で「今後は聞かない」と答えると `.claude/settings.local.json` が書き換わり、Claude Code やエディタが `.claude/` の下にファイルを作ることもある（`fixprguard_guide.md` 7.2.5・8.1）。止まると、Phase 4 の commit は push 済みでも、返信と解決はされない（AC-05）。安全側の停止として受け入れる（1 章の決定）。利用者は、`fixprguard_guide.md` 8.1 に従って確かめてからやり直す。

### 4.4. 互換性 (Compatibility)

-   古いツールと新しい文書の組み合わせでは、検査を飛ばさずに止まる（AC-10）。
-   `/runpr` が実行する `/fixpr` の Phase 4.5 も本書に従う。ほかの呼び出し元は変わらない（AC-11）。

### 4.5. 保守性 (Maintainability)

-   比べ方の規則（何を同じとみなし、何を比べないか）は、Phase 4 の `push` と共有し、Phase 4.5 のために別の規則を作らない。
-   ガードツールの決定的な判定は Go のツールに置き、コマンドの文書には呼び出しだけを書く（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)（本タスクで更新）と [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) に従う。
-   ガードツールを変える PR のマージの後は、利用者が `make install-fixprguard` を実行する（`fixprguard_guide.md` 3 章）。

## 6. 用語集 (Glossary)

-   **基準値・基準値の commit・Phase 4 の commit・1 本につながる・比べる:** 3 章の冒頭で定義した。
-   **対象のファイル:** git が無視するファイルのうち、監視の一覧に当たるものと、必ず含める `.claude/settings.local.json`（`fixprguard_guide.md` 7.1）。
-   **ローカルの CLI レビュー:** `.claude/commands/_lib/local-cli-review.md` の手順。CodeRabbit CLI でレビューし、失敗すれば Codex CLI を `codexreview` を経て使う。
-   用語は `docs/translation_glossary.md` と統一する。
