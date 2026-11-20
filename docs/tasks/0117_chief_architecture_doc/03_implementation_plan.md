# 実装計画書：chief の今の仕様を持つ文書 chief_architecture.md と、プロトコル番号を上げる基準

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-20 |
| Review date | 2026-11-20 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件定義書）の項番を、「設計書の N 節」は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。設計への申し送り [design_handoff.md](design_handoff.md) の H-01〜H-05 は、設計書の 10 章が対応を記録している。本タスクに `implementation_handoff.md` はない。

用語は設計書に合わせる。`/chief` のコマンドの定義（`.claude/commands/chief.md`）を「定義」、Go の開発用の道具 `chief`（`cmd/chief`・`internal/chief`）を「道具」、新しく作る `docs/dev/chief_architecture.md` を「新しい文書」と呼ぶ。「新しい文書の N 章」は設計書の 2.3 節の章立ての章を指す。本計画が引くファイルの行番号は、特に断らない限り、コミット `4b464b45`（本計画を書いた時点の `main`）のものである。設計書が `f9d27314` で引いた行番号は、ADR-0046 を除いて、このコミットでも同じである（`git diff --stat f9d27314 4b464b45` で、対象のファイルのうち変わったのは `docs/dev/adr/0046-chief-of-staff.md` だけだった）。ADR-0046 の行番号のずれは 1.4 節に書く。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

chief の今の仕様を 1 か所にまとめた新しい文書を `docs/dev/chief_architecture.md` に置く。あわせて、ほかの文書の案内と表の置き場所を新しい文書へ向け、ガードテストの照合先を移す（設計書の 1 章〜3 章）。道具の振る舞いとプロトコル番号は変えない（要件定義書の 2.3）。

### 1.2. 実装の原則

-   **写すもの。** 今の振る舞いを写す。新しい文書の値と記述は、実装の時点の `origin/main` の道具のコードと定義から写す。設計書から写すのは今も効いている構成と判定の要約だけで、後のタスクが上書きした記述は今のコードと照らして書く（設計書の 1.1 節・2.1 節）。
-   **コミットの単位。** どのコミットでもテストが通る。各コミットで `make test`・`make lint` が通る。chief_guide の表を消す変更と、テストの照合先を新しい文書へ移す変更は、同じコミットに入れる（設計書の 8 章）。
-   **置き場所。** 1 つの事柄は 1 か所に書く。設計書の 1.1 節の「置き場所」と 2.4 節の振り分けに従う。新しい文書は、プロトコル番号の値と、具体的な `--protocol N` の例を持たない（設計書の決定 15）。
-   **リンク。** リンクの先は確かめられる形で書く。新しい文書と直す文書のリンクとコードの中の `docs/dev/` のパスは、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` がファイルの存在を確かめる。アンカーは確かめないので、PR のレビューで読む。
-   **定義に足す文。** 定義と CLAUDE.md に足す文は英語で書く。定義に足す文には、次の 2 種類のコードの部分を入れない。1 つは、`chief` の語（か、名前が `chief` で終わるパス）と空白を含むもの、もう 1 つは、`herdr` を走らせるものである。コードブロックの行も同じである。`doctestutil.CommandTexts`（`internal/doctestutil/commands.go:201-231`）が、空白を含むコードの部分とコードブロックの行をコマンドとして読み、`TestDocumentedInvocationsParse`（`cmd/chief/docs_test.go:269`、`Invocations` は `commands.go:32-50`）と `TestNoDirectHerdrInvocation`（同 `:302`）が照合するためである。たとえば `` `go install ./cmd/chief` `` は、引数のない呼び出しとして読まれる。

### 1.3. 計画の際に決めたこと（2026-11-20）

1.  実装は 1 つの PR にする。人に確かめた。
    -   理由: 今の仕様を書いた文書は、全体がそろってはじめて正しいかを照らせる。分けると、途中の `main` に章の欠けた文書が置かれ、ADR-0046 の食い違いの記述を 2 回直す必要が出る（設計書の決定 16）。代わりに、1 回のレビューの量は大きくなる。
    -   ADR: なし（PR の分け方は、このタスクの中で閉じる選択のため）。
2.  前後の関係を述べる AC のうち、文書の差分を見る AC-14b・AC-16・AC-17・AC-18・AC-19・AC-19a は、記録した差分を読んで確かめる。Go のテストの差分を見る AC-22 は、`_checks/` のシェルのスクリプトで確かめる。
    -   理由: 文書の差分は、どれも数行から数十行で、レビュアーはそのまま読める（設計書の 7.3 節の見込み）。AC-17 と AC-16 は、差分を設計書の 2.4 節と 3.6 節の表と突き合わせて読める。記録した差分を読む形は文書の変更のためのもの（`requirements_process.md` の「Recorded diff reading」）なので、Go のテストの AC-22 には使わない。
    -   ADR: なし。
3.  `TestGuideValuesMatchTool` を分けた後のテストの名前を、4.1 節の表のとおりにする。
    -   理由: 失敗したときに、どちらの文書の誤りかが名前で分かるようにする（設計書の決定 4）。
    -   ADR: なし。
4.  本タスクの範囲の外で見つかった 2 つの事柄（設計書の 3.6 節の「別の issue に記録すること」）は、ステップ 1-4 で人に issue を作ってよいかを確かめてから作る。作らないと決まったら、PR の説明に書く。
    -   理由: issue を作ることはネットワークへの書き込みで、`/runplan` の承認の範囲（`_lib/task-pr-handoff.md` の「Authorization」）に入らない。
    -   ADR: なし。
5.  マージの前の並行するタスクの確かめ直し（設計書の決定 5）は、`/runpr` のマージの問いに答える前に、人が `/runpr` のセッションに頼んで行う。計画書と PR の説明にその手順を書く（ステップ 1-4）。人に確かめた。
    -   理由: マージの時点では `/runplan` は終わっていて、`/runpr` は計画書のチェックボックスを読まない。人が頼めば、設計書の決定 5 の確かめ（4 つのパスの差分、0115 の文書と PR、結果を PR に書くこと）を、設計を変えずにマージの前に行える。
    -   ADR: なし。

### 1.4. 既存コード調査結果

**ガードテスト（`cmd/chief/docs_test.go`）。**

-   **あるもの:** `TestGuideValuesMatchTool`（`:503-542`）が、chief_guide のサブコマンドの表と終了コードの表の 1 列目、値の表の「プロトコル番号」、状態のディレクトリの名前を照合している。表は `doctestutil.FindTable(guide, "サブコマンド")` と `FindTable(guide, "終了コード")` で文書の全体から探す（`:510-511`）。番号は `guideValue`（`:545-556`）で読む。状態のディレクトリは `chief.OpenStore`（`internal/chief/records.go:131`）が開くディレクトリの相対パスを、文書の全体の `strings.Contains` で探す（`:530-540`）。
-   **照合先の値:** `subcommandFlags`（`cmd/chief/main.go:72`）、`exitOK`〜`exitRefused`（`cmd/chief/main.go:35-38`）、`chief.Protocol`（`internal/chief/records.go:29`、今の値は `6`）。
-   **変えること:** テストを 2 つに分け、新しい文書の 3 つの値は見出しの節の中で探す（4.1 節、設計書の 3.5 節）。節の切り出しには既存の `doctestutil.SectionOutsideBlocks`（`internal/doctestutil/markdown.go:44`）を使い、表は `doctestutil.FindTable`（`internal/doctestutil/tables.go:63`）で探す。新しいヘルパーは足さない。
-   **ほかの文書のテスト:** `TestDocumentedProtocolMatches`（`:205`）と `TestDocumentedInvocationsParse`（`:269`）は、`doctestutil.CommandDocs`（`internal/doctestutil/protocol.go:112`）が返す `.claude/commands` の文書だけを読む。新しい文書は読まれない。`TestSettingsDeniesPaneInput` などの設定のテスト（`:416-481`）は変えない。

**リンクのテスト。** `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`:140`）は、`CLAUDE.md`・`AGENTS.md`・`.claude/commands`・`docs/dev` の Markdown を読み、相対リンクの先とコードの中の `docs/dev/` のパスがあることを確かめる（`:104-131`）。新しい文書も `docs/dev` の下なので、このテストの対象になる。アンカーは確かめない。CI の `doc-guard` のジョブは `./cmd/yt2column` と `./cmd/chief` を走らせる。

**直す文書の箇所。** 設計書が引いた行番号は、次のとおり `4b464b45` でも同じである。

-   chief_guide（`docs/dev/guides/chief_guide.md`）: `:3`・`:32`・`:42`・`:44`・`:169`・`:171-191`・`:236`・`:238-245`・`:247-251`・`:275`
-   package_reference（`docs/dev/conventions/package_reference.md`）: `:116`・`:122`・`:123`
-   `docs/dev/conventions/dev_tools.md`: `:60-66`
-   `CLAUDE.md`: `:24`
-   定義（`.claude/commands/chief.md`）: `:17-18`・`:85`

**ADR-0046 の行番号。** 設計の PR（#1063）は、`:179`・`:182` の文を行の数を変えずに直し、`:398-399` に採らなかった案を書き足し、経緯の末尾にも 1 行を足した（`git diff f9d27314 4b464b45 -- docs/dev/adr/0046-chief-of-staff.md` の hunk は `@@ -176,10 +176,10 @@`・`@@ -395,6 +395,8 @@`・`@@ -531,3 +533,4 @@`）。ずれたのは `:397` より後の行だけで、設計書の 3.6 節の表が引く行（`:23`・`:69`・`:74`・`:122`・`:128`・`:160`・`:175`・`:179`・`:181`・`:182`・`:196`・`:208`）は、`4b464b45` でも同じである。

**変えない参照。** 次の参照は、chief_guide に残る中身を指すので変えない。

-   `.claude/commands/runpr.md:1749` は、道具が自分で決める次のコマンドの説明として chief_guide を指している。この説明は chief_guide の 3 章・5 章・8 章にあり、移さない。
-   `.claude/commands/chief.md:107` は、chief_guide の 8 章（既知の制限）を指している。
-   `docs/dev/guides/tools_guide.md:21` は、使い方として chief_guide を指している。
-   `Makefile:81` のコメントは、道具を変えた PR の後に入れ直すこととして chief_guide を指している。入れ直しは chief_guide の 2 章に残る。
-   `CLAUDE.md:222` の `/chief` の行は、起動の場所の案内として chief_guide を指している。この説明は chief_guide の 3 章に残る。
-   `CLAUDE.md:90` は、定義が chief_guide を手順として読む例外を述べている。定義の `:17-18` を直した後も chief_guide へのリンクは残るので、この例外の文は変えない（設計書の 3.6 節の「定義」）。

**並行するタスク。** 0115（マージコンフリクトの解消）は、`main` に要件定義書と申し送りの文書だけがある（`docs/tasks/0115_chief_conflict_resolution/`）。道具と定義と `_context.md` には `mergemain` の語がない（`git grep -l mergemain -- internal/chief cmd/chief .claude/commands/chief.md .claude/commands/_context.md` の出力が空だった）。ADR-0046 は 0115 の決定を `:43`・`:49`・`:74` と経緯に持つ。新しい文書は、実装の時点で 0115 の振る舞いが道具にあるかを確かめて書く（ステップ 1-4、設計書の決定 5）。

**型の構築の経路。** Go の型は足さない（設計書の 3.7 節）。具体の型で分岐する設計はないので、構築の経路の確かめは当てはまらない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 新しい文書を置き、案内を直す（設計書の 8 章の 1〜4）

#### ステップ 1-1: 新しい文書の 3 章と 4 章を書き、表とテストを移す（設計書の 8 章の 1）

**変えるファイル:** `docs/dev/chief_architecture.md`（新規）、`docs/dev/guides/chief_guide.md`、`cmd/chief/docs_test.go`

-   [x] 新しい文書を作り、2.3 節の章立ての見出しを置く。このステップでは 3 章と 4 章の中身を書く。3 章の 2 つの節（サブコマンド、終了コード）と 4 章の場所の節の見出しは、テストと同じコミットで決める（設計書の 2.3 節）。
-   [x] 3 章に、設計書の 3.2 節の 3 種類の表（サブコマンド、終了コード、出力の型ごと）と、引数の確かめの順・`--protocol` の照合・使い方の誤りの扱いを書く。
    -   値は `cmd/chief/main.go`（`subcommandFlags`、終了コードの定数、`result`）と `internal/chief/output.go` の型から写す。
    -   「定義の使い方」の列は、定義を読んで設計書の 3.2 節の「列の値の決め方」で決める。
-   [x] 4 章に、場所とその決め方、ロック、作成の排他、記録の表（設計書の 3.3 節の 7 列）、片付け、作るとき・消すときの記録を書く。値は `internal/chief` の記録のコード（`records.go` ほか）と chief_guide の 7 章から写し、今の道具と食い違う値は今の値にする（AC-18）。
-   [x] chief_guide を、設計書の 2.4 節の表のうち次の行のとおりに直す。
    -   4 章: `:169` の文と `:171-191` の 2 つの表を、新しい文書の 3 章へのリンクに置き換える。
    -   7 章: `:236`・`:238-245`・`:248` の消えるときの部分・`:249`・`:250` を、新しい文書の 4 章へのリンクに置き換える。`:247`・`:251` と、`:248` の手で消してよいことは残す。
    -   「7 章の『片付け』」を指す `:42` と `:275` を、新しい文書の 4 章を指すように直す。
-   [x] `TestGuideValuesMatchTool` を、4.1 節の表の 2 つのテストに置き換える（設計書の 3.5 節）。新しい文書のパスの定数を足し、見出しの文字列は日本語のリテラルにする（CLAUDE.md の「Go Idioms」のガードテストのリテラルの例外）。失敗の示し方は設計書の 4.1 節に従う。
-   [x] 次の壊し方を 1 つずつ行い、名指したテストが失敗することを確かめる（設計書の 3.5 節）。結果をコミットメッセージに書く。
    -   サブコマンドの表と終了コードの表のそれぞれで、行を 1 つ消す。道具にない行を 1 つ足す（`TestArchitectureValuesMatchTool`）。
    -   状態のディレクトリの名前を変える。名前を 4 章の場所の節から別の章へ移す（`TestArchitectureValuesMatchTool`）。
    -   3 つの見出しのそれぞれの文字を変える（`TestArchitectureValuesMatchTool`）。
    -   chief_guide の番号を変える（`TestGuideProtocolMatchesTool`）。
-   [x] 誤った行を持つ `サブコマンド` の列の表と `終了コード` の列の表を、サブコマンドの節より前（たとえば 2 章）に置き、`TestArchitectureValuesMatchTool` が通ることを確かめる。これは、テストが節の中の表を読んでいることの確かめである。結果をコミットメッセージに書く。

**完了の条件:** `make test`・`make lint` が通る。chief_guide の 4 章に表がなく、新しい文書の 3 章と 4 章に移した中身がある。

#### ステップ 1-2: 新しい文書の残りの章を書く（設計書の 8 章の 2）

**変えるファイル:** `docs/dev/chief_architecture.md`

-   [x] 冒頭と 1 章を、設計書の 2.3 節の表のとおりに書く。冒頭に保守の決まり（AC-23）を置き、ADR-0046 へリンクする。
-   [x] 2 章に、設計書の 2.5 節の下書きを今のコードと照らして仕上げた構成図と、囲み・信頼する入力・出力の点線の説明を書く。図は `docs/dev/conventions/mermaid_reference.md` に従う。本文で、pane へ送る入力が `internal/chief/paneinput` の型だけから組み立てられることと、`.claude/settings.json` の拒否・確認の規則の役割を述べる（AC-05）。保証の一覧は chief_guide の 5 章へ、拒む呼び出しの一覧は 6 章へリンクする。
-   [x] 5 章に、出力の先ごとの表（設計書の 3.3 節の「書き出すものの表」）を書く。状態のディレクトリの行は 4 章へリンクし、記録の項目を繰り返さない。
-   [x] 6 章に、画面と transcript の読み取りを書く。読み取りの範囲は CLAUDE.md の「Declare, don't infer」の chief の例外へリンクする。
-   [x] 7 章に、判定の領域ごとの要約の表（設計書の 2.3 節の 5 列）を書く。手順の細部は書かず、chief_guide の節・設計書の節・ADR-0046 の節へリンクする。
-   [x] 8 章を、設計書の 3.4 節の 8 つの節のとおりに書く。規則の文は要件定義書の「要件定義の際に決めたこと（2026-11-20）」と F-003 の文に合わせ、意味を変えない。番号の値と `--protocol N` の例は書かない（設計書の決定 15）。
-   [x] 9 章に、標本の足し方と採り直し方の要約、ガードテストが照合するもの、`make test` に要るもの（CLAUDE.md の「Development Notes」へリンク）を書く。手順の細部は `internal/chief/testdata/README.md` へリンクする。
-   [x] 書いた各章の記述を、実装の時点の `origin/main` の道具と定義と照らす。照らしたコミットを PR の説明に書く（AC-03）。

**完了の条件:** 新しい文書が冒頭と 9 つの章を持つ。`make test`・`make lint` が通る。

#### ステップ 1-3: ほかの文書の案内を直す（設計書の 8 章の 3）

**変えるファイル:** `docs/dev/guides/chief_guide.md`、`docs/dev/adr/0046-chief-of-staff.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/dev_tools.md`、`CLAUDE.md`、`.claude/commands/chief.md`

-   [x] chief_guide の残りを、設計書の 2.4 節の表のとおりに直す（`:3`・`:32`・`:44`）。表にない行は変えない。
-   [x] ADR-0046 を、設計書の 3.6 節の表のとおりに直す。`:74` は、0115 が道具に入っていなければ変えない。入っているかはステップ 1-4 で確かめ、入っていればそこで直す。経緯の節は変えない。ADR-0046 の直しは、どれも `docs/dev/conventions/adr.md` の「Editorial corrections」の 2 つ目（食い違いの記述の削除）か 3 つ目（古くなった参照の直し）にあたるので、同じ節の「Recording」のとおり、直したコミットのメッセージに修正の行を書き、PR の説明に修正の段落を書く。
-   [x] ADR-0046 の中の要件定義書・設計書・計画書へのリンクを `git grep -n 'tasks/' docs/dev/adr/0046-chief-of-staff.md` で洗い出し、3.6 節の表にない文がないかを確かめる。見つかれば設計書の決定 6 の規則で分け、PR の説明に書く。
-   [x] package_reference の 3 つの行を、設計書の 3.6 節の表のとおりに直す。行のほかの文は変えない。
-   [x] `dev_tools.md` の `:60-66` の `chief_guide.md` のリンクを、chief の契約の置き場所として新しい文書へのリンクにする（設計書の決定 13）。
-   [x] `CLAUDE.md:24` を、設計書の 3.6 節の「CLAUDE.md」のとおりに、英語で直す。
-   [x] 定義の `:17-18` と `:85` を、設計書の 3.6 節の「定義」のとおりに、英語で直す。

**完了の条件:** `make test`・`make lint` が通る。`docs/dev/adr/README.md` の索引の行は変わっていない。

#### ステップ 1-4: 並行するタスクを確かめ、範囲の外の事柄を記録する（設計書の決定 5、3.6 節）

-   [ ] PR を作る前に `git fetch origin main` を行い、PR の base から `origin/main` までの次のパスの差分を読む。変更があれば取り込み、新しい文書（と ADR-0046 の `:74`）に反映する。確かめた範囲（2 つのコミットの ID）を PR の説明に書く。
    -   `internal/chief`
    -   `cmd/chief`
    -   `.claude/commands/chief.md`
    -   `.claude/commands/_context.md` の「Command routing」
-   [ ] 0115 の文書と PR を確かめる。0115 がまだマージされていなければ、PR の説明に、0115 の実装は新しい文書を直す必要があることを書き、`/runplan` の最後の報告でも人に示す。0115 の計画か PR への連絡は人が行い、エージェントは 0115 の PR や issue へは書き込まない（設計書の決定 5）。
-   [ ] 設計書の 3.6 節の「別の issue に記録すること」の 2 つについて、issue を作ってよいかを人に確かめる。認められたら作り、作った issue の URL をこのチェックボックスの行に書いてコミットする（作り直さないため）。PR の説明にもリンクする。認められなければ、その旨をこの行に書き、PR の説明に 2 つを書く（1.3 節の決定 4）。
-   [ ] PR の説明に、マージの前の確かめ直しの手順を書く（1.3 節の決定 5）。`/runpr` のマージの問いに答える前に、人が `/runpr` のセッションに次を頼む。
    -   上の 4 つのパスの差分を、確かめた範囲の後から `origin/main` まで読み、0115 の文書と PR を確かめ、結果（範囲と 0115 の状態）を PR の説明に書く。
    -   変更があれば、マージせずに直す。ブランチに `origin/main` を merge で取り込み（rebase しない）、新しい文書と ADR-0046 の `:74` を新しいコミットで直す。ADR の直しは、ステップ 1-3 のとおり修正の行と修正の段落を書く。7 章の記録は `requirements_process.md` の「Rechecking a record」で記録し直す。

**完了の条件:** 確かめた範囲、0115 の扱い、範囲の外の 2 つの事柄の行き先が、PR の説明に書く材料としてそろっている。範囲の外の 2 つの事柄の扱いが、このステップのチェックボックスの行に書いてある。

#### ステップ 1-5: 差分を記録する（設計書の 7.3 節）

**変えるファイル:** 本計画（7 章の記録）、`docs/tasks/0117_chief_architecture_doc/_checks/`（AC-22 のスクリプト）

-   [ ] ステップ 1-1〜1-4 のコミットの後、7 章の記録を、`docs/dev/conventions/requirements_process.md` の「`diff` verification」の決まったコマンドで書く。比べるパスは各記録に書いたものに限る。
-   [ ] AC-22 のシェルのスクリプトを `docs/tasks/0117_chief_architecture_doc/_checks/` に置く。スクリプトは、マージベースとの差分で、`_test.go` のファイルのうち変わったのが `cmd/chief/docs_test.go` だけであることと、そのファイルで消えたテストの関数が `TestGuideValuesMatchTool` だけで、足したテストの関数が 4.1 節の 2 つだけであることを確かめる。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、PR のブランチで走らせ、出力と 2 つのコミットの ID を 7 章の AC-22 の行に書く。

**完了の条件:** 7 章のすべての記録に、マージベースと比べたコミットの ID と出力がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（ステップ 1-1） | 新しい文書の 3 章と 4 章、表を消した chief_guide、2 つに分けたガードテスト |
| M2（ステップ 1-2） | 新しい文書の冒頭と残りの章 |
| M3（ステップ 1-3〜1-5） | ほかの文書の案内、並行するタスクの確かめ、差分の記録 |

### 3.2 PR 構成

作業は 1 つの PR にする（1.3 節の決定 1）。フェーズ 1 を終えたら、`/runplan` が PR を作る。

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-1 は番号の判断が頼る「定義の使い方」の列を決め、ステップ 1-2 は 10 本の設計書と今のコードから信頼の境界・判定・番号の基準を要約する複雑なステップで、どちらも Sonnet は上書きされた古い記述を写すか、列を弱く書いて番号の判断を上げない側に倒しやすいため

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **単体テスト（ガードテスト）:** `TestGuideValuesMatchTool` を 4.1 節の 2 つのテストに分ける（ステップ 1-1）。照合する 4 つの値は減らさず、照合先を新しい文書と chief_guide に分ける。設計書の 3.5 節の「テストの削除ではない」のとおり、カバレッジの比較の対象にはしない。
-   **壊して確かめる:** ステップ 1-1 の最後の項目で行い、結果をコミットメッセージに書く。
-   **リンク:** 新しい文書と直した文書のリンクの先とパスは、既存の `TestInstructionDocPaths` が確かめる。新しいテストは足さない。
-   **出力の項目とフラグ:** 照合するテストは足さない（AC-22）。表の正しさは PR のレビューで読む。
-   **統合テスト:** なし。道具の振る舞いを変えない（設計書の 7.2 節）。
-   **後方互換:** 道具と定義の約束とプロトコル番号を変えないので、入れ直しは要らない（要件定義書の 4.4）。
-   **`diff` 検証:** 7 章の記録と AC-22 のスクリプトを、ステップ 1-5 で書いて走らせる（1.3 節の決定 2）。
-   **読んで確かめること（`manual`）:** 文の中身は PR のレビューで読む（設計書の 7.3 節）。

### 4.1 テストの名前の対応

| 今の名前 | 新しい名前 | 照合する値 |
|---|---|---|
| `TestGuideValuesMatchTool` | `TestGuideProtocolMatchesTool` | chief_guide の値の表の「プロトコル番号」と `chief.Protocol` |
| `TestGuideValuesMatchTool` | `TestArchitectureValuesMatchTool` | 新しい文書の 3 章のサブコマンドの節の表、3 章の終了コードの節の表、4 章の場所の節の状態のディレクトリの名前 |

ほかの節がテストの名前を書くときは、この表に従う。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 新しい文書が、後のタスクが上書きした設計書の記述を今の仕様として写す | 値は道具のコードから写し、各章を実装の時点の `origin/main` と照らす（ステップ 1-2 の最後の項目）。PR のレビューで AC-03 を読む |
| 「定義の使い方」の列を弱く書き、番号の判断が上げない側に倒れる | 設計書の 3.2 節の「迷うとき」のとおり強いほうを書く。PR のレビューで読む |
| chief_guide から消した中身が新しい文書に入っていない | AC-17 の差分の記録を設計書の 2.4 節の表と突き合わせて読み、AC-18 を PR のレビューで読む |
| 並行するタスク（0115 など）が実装の間に道具を変え、新しい文書が古くなる | PR を作る前に、設計書の決定 5 のパスの差分を読む（ステップ 1-4）。マージの前に、人が `/runpr` のセッションに確かめ直しを頼む（1.3 節の決定 5） |
| 定義に足した文が、`TestDocumentedInvocationsParse` か `TestNoDirectHerdrInvocation` にコマンドとして読まれる | 1.2 節の決まりに従って書く。入れたときは `make test` が失敗する |
| PR が大きく、レビューで読み落とす | PR の説明で、章ごとに照らした道具のファイルを挙げ、レビュー観点を分ける |
| ADR-0046 の直しに修正の行と段落がなく、レビュアーが分類を確かめられない | ステップ 1-3 の ADR の項目のとおり、直したコミットに修正の行を、PR の説明に修正の段落を書く |
| 新しい文書を書く量が見込みより大きく、PR が遅れる | ステップ 1-1 の後に `make test` の通るコミットを置き、残りの章を章ごとのコミットにして、途中で止まっても続きから書けるようにする |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] ステップ 1-1: 3 章と 4 章、表とテストの移動
- [x] ステップ 1-2: 残りの章
- [x] ステップ 1-3: ほかの文書の案内
- [ ] ステップ 1-4: 並行するタスクの確かめと範囲の外の事柄
- [ ] ステップ 1-5: 差分の記録

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`make test` が Go のガードテストを走らせる。CI の `doc-guard` のジョブも `./cmd/chief` と `./cmd/yt2column` を走らせる。

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test`、`manual` | 文書があること: `cmd/chief/docs_test.go::TestArchitectureValuesMatchTool`（文書を読めないと失敗する）。ADR-0046 へのリンクの先: `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`。日本語であることと冒頭の中身: PR のレビューで読む |
| AC-02 | `manual` | PR のレビューで、1 章と ADR-0046 の節へのリンクを読む |
| AC-03 | `manual` | PR のレビューで、各章を実装の時点の道具と定義と、2 章と 5 章を設計書の 5 章の文書と照らして読む |
| AC-04 | `manual` | PR のレビューで、設計書へのリンクの書き方を読む |
| AC-05 | `manual` | PR のレビューで、構成図を設計書の 2.5 節と今のコードと照らし、`mermaid_reference.md` に従うことを読む。本文の paneinput と `.claude/settings.json` の規則の記述と、chief_guide の 6 章へのリンクも読む |
| AC-06 | `test`、`manual` | サブコマンドと終了コード: `cmd/chief/docs_test.go::TestArchitectureValuesMatchTool`。フラグ、出力の型の表、`--protocol` の照合、使い方の誤りの扱い: PR のレビューで `cmd/chief/main.go` と `internal/chief/output.go` と照らして読む |
| AC-07 | `test`、`manual` | 状態のディレクトリの名前: `cmd/chief/docs_test.go::TestArchitectureValuesMatchTool`。場所の決め方、ロック、作成の排他、記録の表: PR のレビューで読む |
| AC-08 | `manual` | PR のレビューで、5 章の表を読む |
| AC-09 | `manual` | PR のレビューで、6 章を読む |
| AC-10 | `manual` | PR のレビューで、9 章を読む |
| AC-11〜AC-13 | `manual` | PR のレビューで、8 章を要件定義書の「要件定義の際に決めたこと（2026-11-20）」と F-003 の文と照らして読む |
| AC-12a | `manual` | PR のレビューで、8 章の約束の節と chief_guide の 2 章へのリンクを読む |
| AC-14 | `manual` | PR のレビューで、8 章の「上げる PR が変える場所」を読む |
| AC-14a | `manual` | PR のレビューで、8 章の「組み合わせの誤りに当たったとき」を読む |
| AC-14b | `diff`、`manual` | 下の「AC-14b の記録」。案内の中身は PR のレビューで読む |
| AC-15 | `test`、`manual` | 新しい文書へのリンクの先: `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`。要約が AC-11・AC-12 と食い違わないこと: PR のレビューで読む |
| AC-16 | `diff`、`test`、`manual` | 下の「AC-16 の記録」を設計書の 3.6 節の表と突き合わせて読む。同じ記録で、`dev_tools.md` の差分が chief の 1 か所のリンクに限られること（設計書の決定 13）と、`docs/dev/adr/README.md` が変わらないことも読む。リンクの先: `TestInstructionDocPaths`。直した文が今の仕様の置き場所を指すこと: PR のレビューで読む |
| AC-17 | `diff`、`test` | 下の「AC-17 の記録」を設計書の 2.4 節の表と突き合わせて読む。番号の値の表が残ること: `cmd/chief/docs_test.go::TestGuideProtocolMatchesTool` |
| AC-18 | `diff`、`manual` | 下の「AC-18 の記録」の消した行を 1 つずつ、新しい文書の 3 章と 4 章と突き合わせて読む |
| AC-19 | `diff`、`test` | 下の「AC-19 の記録」。リンクの先: `TestInstructionDocPaths` |
| AC-19a | `diff`、`test` | 下の「AC-19a の記録」。リンクの先: `TestInstructionDocPaths` |
| AC-20 | `test` | `cmd/chief/docs_test.go::TestArchitectureValuesMatchTool` |
| AC-21 | `test` | `cmd/chief/docs_test.go::TestGuideProtocolMatchesTool` |
| AC-22 | `diff` | `docs/tasks/0117_chief_architecture_doc/_checks/` の AC-22 のスクリプト（ステップ 1-5）。出力と 2 つのコミットの ID をこの行に書く |
| AC-23 | `manual` | PR のレビューで、新しい文書の冒頭を読む |
| AC-24 | `manual` | PR のレビューで、ADR-0046 の `:179` の決まりを読む |

### AC-14b の記録

Diff record (AC-14b):

- Compared paths: `.claude/commands/chief.md`

### AC-16 の記録

Diff record (AC-16):

- Compared paths: `docs/dev/adr/0046-chief-of-staff.md` `docs/dev/conventions/dev_tools.md` `docs/dev/adr/README.md`

### AC-17 の記録

Diff record (AC-17):

- Compared paths: `docs/dev/guides/chief_guide.md`

### AC-18 の記録

Diff record (AC-18):

- Compared paths: `docs/dev/guides/chief_guide.md`

### AC-19 の記録

Diff record (AC-19):

- Compared paths: `docs/dev/conventions/package_reference.md`

### AC-19a の記録

Diff record (AC-19a):

- Compared paths: `CLAUDE.md` `.claude/commands/chief.md`

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-24 がすべて 7 章の検証で確かめられている。
-   `make test`・`make lint` が通る。
-   ガードテストを壊して確かめた記録が、ステップ 1-1 のコミットメッセージにある。
-   道具の振る舞い、プロトコル番号、`.claude/settings.json` の規則を変えていない（設計書の 5 章）。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、issue #1055 を閉じる（`/runpr` が案内する）。
-   0115 が後にマージされるときは、その PR が F-006 の決まりで新しい文書を直す（設計書の決定 5）。

## 変更履歴

- 2026-11-20 承認（isseis、#1065）
