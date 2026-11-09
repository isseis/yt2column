# 要件定義書：/fixpr Phase 4.5 の commit と push の前に、ガードツールが git の状態を基準値と比べる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/fixpr` の Phase 4.5（ローカルの CLI レビュー）で作る commit と push を、ガードツール `fixprguard` の検査に通すための要件を定義する（[#426](https://github.com/isseis/yt2column/issues/426)）。Phase 4.5 の最初、各 commit の前、push の前に、hooks・config・info・対象のファイルを基準値と比べ、違えば止める。push はガードツールが行う。

**背景:**

`fixprguard` は、`/fixpr` と `/runpr` が信頼できないテキスト（PR のレビューのスレッド）を扱う間の守りとして、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で作られた。`/fixpr` の Phase 4 では、`finish` が build checks と commit を行い、`push` がその commit を push する（`.claude/commands/fixpr.md:636-677`）。`push` は、直前に hooks・config・info・対象のファイルが基準値と同じであること、HEAD の親が基準値の commit だけであること、push 先のブランチの OID が基準値と同じであることを確かめ、明示した refspec で push する（`internal/fixprguard/worktree.go:571-589`）。

その後の Phase 4.5 では、セッション自身がローカルの CLI レビューを行う（`.claude/commands/fixpr.md:695-721`）。手順は `.claude/commands/_lib/local-cli-review.md` に従う。セッションは、`git status` を確かめ（同 70 行目）、CodeRabbit CLI を実行し（同 78 行目）、それが失敗すれば `codexreview` を経て Codex CLI を実行する（同 102 行目）。指摘を直したら build checks を実行して commit する（同 204 行目）。README の同期で `README.en.md` が変われば、それも commit する（`.claude/commands/fixpr.md:712-716`）。最後に素の `git push` を実行する（同 718 行目）。

この間、ガードツールは何も確かめない。次の 3 つの検査がない。

-   hooks・config・info・対象のファイルの、基準値との比較。Phase 4 の `push` が確かめた後に書き換えられた hooks は、`git commit`（pre-commit・commit-msg・post-commit）と `git push`（pre-push）で実行される。git が実行するコマンドを指す config（`core.fsmonitor` など）は、`git status`、レビューの CLI が起動する git、`make fmt` が起動する git（`Makefile:309`）でも実行されうる。
-   変更範囲の検査（`inspect`）。ローカルの CLI レビューの指摘は、修正のフェーズのエージェントが書いた diff から作られる。そのため、信頼できないテキストの影響を間接的に受ける。
-   判定した集合との一致の確認（stage するパスの限定、commit のツリーの確認）。

タスク 0030 の設計書は、「確認と実行の間のずれ」として、`push` の確認の後に別のプロセスが hooks や config を書き換えることを残余のリスクとした（0030 `02_architecture.md` 5.3）。ガイドも、`push` の後の書き換えを検出しないとしている（`docs/dev/guides/fixprguard_guide.md` 7.2.4・7.2.5）。しかし、同じラウンドの中で、守られていない commit と push がその後に続くことは書かれていない。切り離されたプロセスによる書き換えが成功すると、それがここで実行される。

`_context.md` の「Guard tool」の行は、`fixpr.md` と `runpr.md` が fix round の commit と push にガードツールを使うとしている（`.claude/commands/_context.md:194`）。[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の決定も、`push` を道具が行うとしている。Phase 4.5 の commit と push は、どちらとも合っていない。

対応 issue: [#426](https://github.com/isseis/yt2column/issues/426)（関連: [#395](https://github.com/isseis/yt2column/issues/395)、タスク `0030_fixpr_untrusted_threads`、タスク `0049_codex_local_review`）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。Phase 4.5 のどの時点で何を確かめ、違えば何をせずに止まり、何を報告するかである。次のことは設計（`02_architecture.md`）で決める。

-   ガードツールの呼び出しの形。新しいサブコマンドを加えるか、既存のサブコマンドに引数を加えるか。
-   Phase 4 の commit をツールに渡す手段と、基準値の扱い。
-   `--protocol` の番号を上げるか。
-   比較の呼び出しを、`fixpr.md` と `local-cli-review.md` のどちらに書くか。
-   テストの手段。

要件のレビューで挙がった設計事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #426 で決めたこと:**

-   **完了の条件:** Phase 4.5 の commit と push の前に、少なくとも hooks・config・info を基準値と比べ、違えば止める。
-   **ほかの呼び出し元を対象にしない:** `local-cli-review.md` を呼ぶ `/fixpr` 以外のコマンド（`/runpr` の step 1、`/runplan`、`/runissue`、`_lib/task-pr-handoff.md` の Part B）は対象にしない。どれも `begin` の基準値を持たないので、比べる相手がない。ただし、同じ手順を共有するので、手順を変える場合は、その影響を確かめる（F-005）。
-   **文書を合わせる:** `_context.md` の「Guard tool」の行と、タスク 0030 の設計書の 5.2・5.3 を、決めた対応に合わせる。設計書の直し方は、下の「要件定義の際に決めたこと」で決めた。

**要件定義の際に決めたこと（2026-11-09）:**

-   **状態を比べ、push をガードツールで行う。範囲の検査はしない:** issue は、比べるだけの案 1 と、`inspect` → `finish` → `push` に通す案 2 を挙げていた。本書は、案 1 に、ガードツールによる push を加えた形にする（F-002・F-003）。案 2 を採らない理由は 2 つある。1 つ目は、`inspect` の範囲の規則がスレッドの `path` を元にすることである。Phase 4.5 にはスレッドがないので、新しい範囲の規則を決める必要がある。2 つ目は、`finish` が commit の件名を固定し、本文に `Editorial correction:` の行だけを認めることである（`docs/dev/guides/fixprguard_guide.md` 5 章の `finish`）。このため、ローカルの CLI レビューの記録の行（`local-cli-review.md` の手順 7）や、README の同期の commit を通せない。push をガードツールで行うのは、ADR-0015 の「`push` も道具が行う」に合わせるためと、push 先の確認と明示した refspec を Phase 4 と同じにするためである。ADR の判断: [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) を更新した。ガードツールの守りが及ぶ範囲を変える決定であり、`/fixpr` の手順とガードツールの 2 つの構成要素にまたがるためである。
-   **Phase 4.5 の最初にも比べる:** commit と push の前だけでなく、Phase 4.5 が最初に git を起動する前にも比べる（F-001）。Phase 4.5 は、commit より前に `git status`、CodeRabbit CLI、`codexreview` を実行し、どれも git を起動する。git が実行するコマンドを指す config が書き換えられていると、commit の前の比較より先に、それが実行されるためである。ADR の判断: 上の決定と同じく、ADR-0015 の更新に含めた。
-   **タスク 0030 の設計書には注記だけを足す:** 今の仕様は `docs/dev/guides/fixprguard_guide.md` と `.claude/commands/_context.md` に書く。タスク 0030 の設計書（承認済み）には、5.2 と 5.3 に本タスクへの注記を足すだけにし、本文は書き換えない（AC-13）。完了したタスクの記録を残したまま、今の仕様の置き場所を 1 か所に保つためである。タスク 0057 が 0030 の設計書の 5.3 に足した注記と同じ形にする。ADR の判断: ADR にしない。どの文書に書くかの選択で、本タスクの中で閉じる決定だからである。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Phase 4 の `push` の確認の後に hooks・config・info・対象のファイルが書き換えられた場合、Phase 4.5 がそれを実行する前に止める。
-   Phase 4.5 の push を、Phase 4 と同じ確認（push 先の OID と HEAD の系列）と明示した refspec で行う。
-   `_context.md`、ガイド、ADR-0015 の記述を、ガードツールが実際に守る範囲と一致させる。

### 2.2. スコープ (In Scope)

-   `/fixpr` の Phase 4.5 で、最初に git を起動する前の比較（F-001）。
-   Phase 4.5 で作る各 commit の前の比較（F-002）。対象は、ローカルの CLI レビューの修正の commit と、README の同期による `README.en.md` の commit である。
-   Phase 4.5 の push をガードツールで行うこと（F-002・F-003）。
-   インストール済みのツールが古いときの振る舞い（F-004）。
-   `local-cli-review.md` を共有するほかの呼び出し元の振る舞いを変えないこと（F-005）。
-   `_context.md`、`fixprguard_guide.md`、ADR-0015、タスク 0030 の設計書の注記（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **Phase 4.5 の修正の範囲の検査と、判定した集合との一致の確認。** 1 章の決定のとおり行わない。Phase 4.5 の修正が範囲外のファイルを変えても、利用者への確認はない。残余のリスクとして 4.2 に記録する。
-   **`local-cli-review.md` を呼ぶ、`/fixpr` 以外のコマンド。** issue #426 のとおり対象にしない（1 章）。
-   **Phase 5・6 で基準値を持たないサブコマンド（`reply`・`edit-pr` など）の起動前の比較。** Phase 4.5 の push の後の書き換えは、今までの `push` の後と同じく検出しない（`fixprguard_guide.md` 7.2.4）。issue #426 が求めていないためである。
-   **Phase 4.5 の中でセッションが自分で起動する git。** セッションがコードを読むために起動する `git diff` や `git log` などは、比較の時点の間にあれば守られない。比較は、Phase 4.5 の最初、build checks、stage、commit、push という、手順が定める時点の前に行う。
-   **タスク 0030 の設計書の 5.2・5.3 の本文の書き換え。** 1 章の決定のとおり、注記だけを足す。

## 3. 機能要件 (Functional Requirements)

本章の「比べる」は、hooks・config・info・対象のファイルを基準値と比べることをいう。比べ方（何を同じとみなし、何を比べないか）は、`push` と同じである（`docs/dev/guides/fixprguard_guide.md` 5 章の `inspect` と `push`、7.1）。「対象のファイル」は、git が無視するファイルのうち監視の一覧に当たるもの（同 7.1）をいう。「基準値」は、その回の `/fixpr` で `begin` が出力した基準値をいう。「Phase 4 の commit」は、その回の Phase 4 で `finish` が作り、`push` が push した commit をいう。

### 3.1. 機能一覧

#### F-001: Phase 4.5 の最初の比較

Phase 4.5 は、最初に git を起動する前に比べる。Phase 4 の `push` の後に書き換えられた設定が、レビューの CLI の起動する git で実行される前に止めるためである。

**Acceptance Criteria**:

-   **AC-01**: Phase 4 が commit を作って push したとき、Phase 4.5 は、`git status` も CodeRabbit CLI も `codexreview` も起動する前に比べる。
-   **AC-02**: AC-01 の比較で違いがあれば、Phase 4.5 は git も、どちらのレビューの CLI も起動せずに止まり、Phase 5 と Phase 6 を行わない。Final report は、止まった種類、違う部分（`parts`）、Phase 4 の commit が push 済みであること、返信と解決をしていないことを示す。Phase 4 が commit を作らなかったときは、今までどおり Phase 4.5 を飛ばし、比較もしない。

#### F-002: Phase 4.5 の commit の前の比較

Phase 4.5 は、commit を作るたびに、その前に比べる。対象の commit は、ローカルの CLI レビューの修正の commit と、README の同期による `README.en.md` の commit である。

**Acceptance Criteria**:

-   **AC-03**: ローカルの CLI レビューの修正を commit するとき、Phase 4.5 は、build checks を実行する前と、build checks の後で修正を stage する前の 2 回比べる。README の同期の commit では、`README.en.md` を stage する前に比べる。
-   **AC-04**: AC-03 の各比較では、今のブランチが基準値のブランチであることと、HEAD が Phase 4 の commit か、Phase 4 の commit から 1 本につながる commit の列の先端であることも確かめる。1 本につながるとは、Phase 4 の commit の後の各 commit の親が、ちょうど 1 つで、1 つ前の commit であることをいう。
-   **AC-05**: AC-03 の比較で違いがあるか、AC-04 の条件に合わなければ、Phase 4.5 は build checks・stage・commit をせずに止まる。push せず、Phase 5 と Phase 6 も行わない。修正は作業ツリーに残り、それまでに Phase 4.5 が作った commit は push されずにローカルに残る。Final report は、止まった種類、違う部分（`parts`）、push されていない commit があればその数を示す。また、次の `/fixpr` が、利用者が作業ツリーを確かめるまで `dirty-work-tree` で止まることを示す。
-   **AC-06**: 比較に通ったとき、Phase 4.5 の commit のメッセージは今までと同じである。ローカルの CLI レビューの修正の commit は、手順が定める記録の行を持つ（`local-cli-review.md` の手順 7）。README の同期の commit は、自分のメッセージを持つ。

#### F-003: Phase 4.5 の push をガードツールで行う

Phase 4.5 が commit を作ったとき、その push はガードツールが行う。セッションは `git push` を実行しない。

**Acceptance Criteria**:

-   **AC-07**: ガードツールは、push の直前に比べ、次のことも確かめる。今のブランチが基準値のブランチであること。HEAD から Phase 4 の commit までが、AC-04 の意味で 1 本につながっていること。push 先のブランチの OID が Phase 4 の commit であること。どれかに合わなければ、push せずに止まる。Phase 4.5 は Phase 5 と Phase 6 を行わず、Final report は、止まった種類、違う部分（`parts`）、push されていない commit の数を示す。
-   **AC-08**: ガードツールは、Phase 4 の `push` と同じく、明示した refspec で HEAD を push 先のブランチに送り、タグとサブモジュールを伴わない。
-   **AC-09**: Phase 4.5 が commit を作らなかったときは、push もその確認もしない。今までどおり Phase 5 に進む。

#### F-004: 古いツールでは検査を飛ばさない

インストール済みの `fixprguard` が、本タスクの検査に対応していないことがある（`make install-fixprguard` を、本タスクのマージの後に実行していない場合）。そのとき、Phase 4.5 は検査なしに進まない。

**Acceptance Criteria**:

-   **AC-10**: インストール済みのツールが本タスクの検査に対応していないとき、`/fixpr` は、Phase 4.5 で比較なしに git もレビューの CLI も起動せず、commit も push もしない。止まって、最新の `main` から `make install-fixprguard` を実行するよう利用者に示す。止まる時点が Phase 4.5 より前でもよい。

#### F-005: ほかの呼び出し元の振る舞いを変えない

`local-cli-review.md` を呼ぶ `/fixpr` 以外のコマンドは、本タスクの後も今までと同じに振る舞う。

**Acceptance Criteria**:

-   **AC-11**: `/runpr` の step 1、`/runplan`、`/runissue`、`_lib/task-pr-handoff.md` の Part B が行うローカルの CLI レビューは、本タスクの後もガードツールを呼ばず、今までと同じ手順で commit する。

#### F-006: 文書

ガードツールが守る範囲の記述を、本タスクの後の実際と一致させる。

**Acceptance Criteria**:

-   **AC-12**: 次の文書が、本タスクの後の振る舞いを述べる。
    -   `.claude/commands/_context.md` の「Guard tool」の行。Phase 4.5 の commit が比較だけを受け、範囲の検査を受けないことを含む。
    -   `docs/dev/guides/fixprguard_guide.md`。サブコマンドか引数を加えるなら 5 章に、守ることを 7.1 に、守らないこと（Phase 4.5 の修正の範囲の検査がないこと、比較が時点だけの確認であること）を 7.2 に書く。
    -   `.claude/commands/fixpr.md` の Phase 4.5 と、その許可の段落（`git push` を Phase 4.5 が行うとする記述）。
-   **AC-13**: タスク 0030 の設計書の 5.2 と 5.3 に、本タスクへのリンクを持つ注記がある。両節の本文は変わらない。
-   **AC-14**: 文書のガード（`cmd/fixprguard/docs_test.go` など）を含む `make test` と `make lint` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比較は Phase 4 の `push` と同じもので、Phase 4.5 の 1 回につき、最初、各 commit、push の時点で行う。ローカルの CLI レビュー（数十秒から数分）に比べて小さい。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の適用: N/A。本タスクは、`yt-dlp` の起動、API キーや Webhook の URL、キャッシュのディレクトリ、ネットワークへの送信、プロンプトへの信頼できないテキストの埋め込みのどれにも関わらない。
-   信頼できない入力の境界: 新しい境界は加えない。ガードツールが読むのは、今までと同じ git の状態である。ローカルの CLI レビューの指摘は、今までどおり `local-cli-review.md` の手順 4 の扱いを受ける。
-   **残余のリスク**（本タスクの後も残るもの）:
    -   **Phase 4.5 の修正は範囲の検査を受けない。** Phase 4.5 の修正は、信頼できないテキストの影響を間接的に受けた指摘に従って、セッションが行う。範囲外のファイル（実行関連ファイルを含む）を変えても、利用者への確認なしに build checks で実行され、commit・push される。比較が検出するのは、hooks・config・info・対象のファイルの書き換えだけである。1 章の決定で受け入れる。
    -   **比較は時点だけの確認である。** 比較から、git の起動、commit、push までの間に、別のプロセスが書き換えることは防げない。0030 の設計書 5.3 の「確認と実行の間のずれ」と同じである。本タスクは、そのずれの幅を Phase 4 の `push` から Phase 4.5 の終わりまでに広げずに、各時点の直前に縮める。
    -   **比較の時点の間にセッションが起動する git。** 2.3 のとおり、手順が定める時点の間にセッションが自分で起動する git は守られない。
    -   **比較を呼ぶことは文書に依存する。** Phase 4.5 で比較を呼ぶこと、`git push` を実行しないことは、コマンドの文書に従うセッションに依存する（`fixprguard_guide.md` 7.2.2 と同じ）。
    -   **Phase 4.5 の push の後の書き換え。** Phase 5・6 の `reply`・`edit-pr` などは、今までどおり `gh` の起動の前に比べない（2.3）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   比較が止まったとき、作業ツリーの修正と、push されていない commit を捨てない（AC-05・AC-07）。利用者が中身を確かめてから、やり直せるようにする。
-   ガードツールの終了コードのうち、続けてよいもの以外（読めない出力や信号による終了を含む）は、すべて止まるものとして扱う（`fixprguard_guide.md` 4 章と同じ）。

### 4.4. 互換性 (Compatibility)

-   古いツールと新しい文書の組み合わせでは、検査を飛ばさずに止まる（AC-10）。
-   `/runpr` は Phase 4.5 を `/fixpr` を通して実行するので、`/runpr` の振る舞いも本書に従う。ほかの呼び出し元は変わらない（AC-11）。

### 4.5. 保守性 (Maintainability)

-   比較の規則（何を同じとみなし、何を比べないか）は、Phase 4 の `push` と 1 か所で共有し、Phase 4.5 のために別の規則を作らない。
-   ガードツールの決定的な判定は Go のツールに置き、コマンドの文書には呼び出しだけを書く（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)（本タスクで更新）と [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) に従う。
-   ガードツールを変える PR のマージの後は、利用者が `make install-fixprguard` を実行する（`fixprguard_guide.md` 3 章）。

## 6. 用語集 (Glossary)

-   **基準値:** その回の `/fixpr` の `begin` が出力し、`inspect`・`finish`・`push` に `--baseline` で渡す値（`fixprguard_guide.md` 5 章の `begin`）。
-   **対象のファイル:** git が無視するファイルのうち、監視の一覧に当たるもの（`fixprguard_guide.md` 7.1）。
-   **Phase 4 の commit:** その回の Phase 4 で `finish` が作り、`push` が push した commit。
-   **ローカルの CLI レビュー:** `.claude/commands/_lib/local-cli-review.md` の手順。CodeRabbit CLI でレビューし、失敗すれば Codex CLI を `codexreview` を経て使う。
-   用語は `docs/translation_glossary.md` と統一する。
