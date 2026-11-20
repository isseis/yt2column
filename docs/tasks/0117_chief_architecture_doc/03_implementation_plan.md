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

-   [x] PR を作る前に `git fetch origin main` を行い、PR の base から `origin/main` までの次のパスの差分を読む。変更があれば取り込み、新しい文書（と ADR-0046 の `:74`）に反映する。確かめた範囲（2 つのコミットの ID）を PR の説明に書く。
    -   確かめた範囲: `c2df59f3daa72de99354a2992f92746517dc8ad1`..`c2df59f3daa72de99354a2992f92746517dc8ad1`（2026-11-20 の `git fetch origin main` の後）。4 つのパスに差分はなかった。
    -   `internal/chief`
    -   `cmd/chief`
    -   `.claude/commands/chief.md`
    -   `.claude/commands/_context.md` の「Command routing」
-   [x] 0115 の文書と PR を確かめる。0115 がまだマージされていなければ、PR の説明に、0115 の実装は新しい文書を直す必要があることを書き、`/runplan` の最後の報告でも人に示す。0115 の計画か PR への連絡は人が行い、エージェントは 0115 の PR や issue へは書き込まない（設計書の決定 5）。
    -   確かめた結果: `origin/main` に 0115 の要件定義書だけがあり、設計の PR #1051 は open で、実装はまだない。道具と定義と `_context.md` に `mergemain` の語はなく、ADR-0046 の `:74` は変えない。
-   [x] 設計書の 3.6 節の「別の issue に記録すること」の 2 つについて、issue を作ってよいかを人に確かめる。認められたら作り、作った issue の URL をこのチェックボックスの行に書いてコミットする（作り直さないため）。PR の説明にもリンクする。認められなければ、その旨をこの行に書き、PR の説明に 2 つを書く（1.3 節の決定 4）。
    -   人が認めたので、2 つを作った。https://github.com/isseis/yt2column/issues/1067 （定義の規則 2 が `message` を名指していないこと）と https://github.com/isseis/yt2column/issues/1068 （AC-03 が照合先に挙げる `security.md` §6 が chief を扱わないこと）。
-   [x] PR の説明に、マージの前の確かめ直しの手順を書く（1.3 節の決定 5）。`/runpr` のマージの問いに答える前に、人が `/runpr` のセッションに次を頼む。
    -   上の 4 つのパスの差分を、確かめた範囲の後から `origin/main` まで読み、0115 の文書と PR を確かめ、結果（範囲と 0115 の状態）を PR の説明に書く。
    -   変更があれば、マージせずに直す。ブランチに `origin/main` を merge で取り込み（rebase しない）、新しい文書と ADR-0046 の `:74` を新しいコミットで直す。ADR の直しは、ステップ 1-3 のとおり修正の行と修正の段落を書く。7 章の記録は `requirements_process.md` の「Rechecking a record」で記録し直す。

**完了の条件:** 確かめた範囲、0115 の扱い、範囲の外の 2 つの事柄の行き先が、PR の説明に書く材料としてそろっている。範囲の外の 2 つの事柄の扱いが、このステップのチェックボックスの行に書いてある。

#### ステップ 1-5: 差分を記録する（設計書の 7.3 節）

**変えるファイル:** 本計画（7 章の記録）、`docs/tasks/0117_chief_architecture_doc/_checks/`（AC-22 のスクリプト）

-   [x] ステップ 1-1〜1-4 のコミットの後、7 章の記録を、`docs/dev/conventions/requirements_process.md` の「`diff` verification」の決まったコマンドで書く。比べるパスは各記録に書いたものに限る。
-   [x] AC-22 のシェルのスクリプトを `docs/tasks/0117_chief_architecture_doc/_checks/` に置く。スクリプトは、マージベースとの差分で、`_test.go` のファイルのうち変わったのが `cmd/chief/docs_test.go` だけであることと、そのファイルで消えたテストの関数が `TestGuideValuesMatchTool` だけで、足したテストの関数が 4.1 節の 2 つだけであることを確かめる。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、PR のブランチで走らせ、出力と 2 つのコミットの ID を 7 章の AC-22 の行に書く。

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
- [x] ステップ 1-4: 並行するタスクの確かめと範囲の外の事柄
- [x] ステップ 1-5: 差分の記録

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
| AC-22 | `diff` | `docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh`。出力と 2 つのコミットの ID は下の「AC-22 の記録」にある |
| AC-23 | `manual` | PR のレビューで、新しい文書の冒頭を読む |
| AC-24 | `manual` | PR のレビューで、ADR-0046 の `:179` の決まりを読む |

### AC-14b の記録

Diff record (AC-14b):

- Compared paths: `.claude/commands/chief.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `7798f4b4e610482ef66ee355846a3ea080236676`

Path diff:

```text
diff --git a/.claude/commands/chief.md b/.claude/commands/chief.md
index bd8b3a2da77f501d1cfe94f265fbcd134c85d95e..5daab9f24909d3883bd8a6ff4fc84304638e5132 100644
--- a/.claude/commands/chief.md
+++ b/.claude/commands/chief.md
@@ -15,7 +15,9 @@ worktree and workspace are made and a finished one's removed. You
 never decide any of these yourself, you never type into another pane, and
 you never make or remove a worktree or a workspace yourself.
 How the tool works, what it guarantees, and how to install it are in the
-guide, [chief_guide.md](../../docs/dev/guides/chief_guide.md) (Japanese).
+guide, [chief_guide.md](../../docs/dev/guides/chief_guide.md) (Japanese). The
+tool's contract and records, to read beside this definition, are in
+[chief_architecture.md](../../docs/dev/chief_architecture.md) (Japanese).

 Talk to the human in Japanese (`CLAUDE.md`, "Language"). Show the tool's
 agent values as their names: `claude_code` as Claude Code, `opencode` as
@@ -82,7 +84,7 @@ Two calls differ:
 |---|---|---|
 | `0` | Done (a decision not to send is done too) | Read the JSON |
 | `1` | The tool failed (`outcome` `failed`, or `timeout` from `wait`) | Show `message`, and say that this call's result could not be read (for a round, that its table was not updated; for `wait`, step 5 says what to do). Do not judge from `message` which workspaces it concerns |
-| `2` | Usage error | A mistake in this definition or in your arguments: stop and report the call and its standard error |
+| `2` | Usage error, or the tool and this definition are from different commits | A mistake in this definition or in your arguments: stop and report the call and its standard error. A tool that does not know this call also exits `2`, so in every case also tell the human that the tool and this definition may be from different commits (the tool may be older), and that the fix is to stop `/chief`, update the chief worktree to `main`, reinstall the tool, and start `/chief` again, in the order the guide's chapter 2 gives |
 | `3` | Refused, nothing done (`outcome` `refused`, `reason`) | See below |

 The refusals (`reason`):
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-16 の記録

Diff record (AC-16):

- Compared paths: `docs/dev/adr/0046-chief-of-staff.md` `docs/dev/conventions/dev_tools.md` `docs/dev/adr/README.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `5e00e64e5971c037706044625dbec7d54159e634`

Path diff:

```text
diff --git a/docs/dev/adr/0046-chief-of-staff.md b/docs/dev/adr/0046-chief-of-staff.md
index fcba9199e8b47722a5d78bf8db132ef4a3b5c54d..49ce6e00bd675131489c414f61173526a3fa8c27 100644
--- a/docs/dev/adr/0046-chief-of-staff.md
+++ b/docs/dev/adr/0046-chief-of-staff.md
@@ -20,7 +20,7 @@ Herdr はリポジトリの外の道具で、chief はその CLI の出力に依

 chief は人の代わりに判断せず、人が選んだ答えと決まった範囲の入力だけを、Go の道具 `chief` が確かめてからほかのエージェントへ送る。マージの後の次のコマンドと、マージコンフリクトになった PR の解消のコマンドは、GitHub と git の状態を引き金に自動で送り、タスクの workspace は issue から作って、完了を確かめてから人の確認で消す。

-以下の決定の仕様は、[0088 要件定義書](../../tasks/0088_chief_of_staff/01_requirements.md) の機能要件が持つ。
+以下の決定の仕様は [chief_architecture.md](../chief_architecture.md) が持ち、決定の出典は [0088 要件定義書](../../tasks/0088_chief_of_staff/01_requirements.md) の機能要件である（以下、番号だけで書いた F-NNN は、その機能要件を指す）。

 ### 人の判断の中継

@@ -66,7 +66,7 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった
     -   理由：値を ASCII にして、似た文字による読み違いをなくすためである。形は `_context.md` の「PR marker conventions」が持つ（0088 設計書の 3.8）。
 -   送れたかが分からない送信は送り直さず、その workspace への送信を、人が解除するまで止める。
     -   理由：同じコマンドを 2 回走らせるより、人に示して止まるほうを選ぶためである（F-006）。
--   次のコマンドを `/runplan` と決めるために読む計画書のステップは、受け入れる 3 つの書き方（チェックボックスの行、2 章の見出し、2 章の 1 行の太字の段落）からだけ読む。2 章の中のステップに似た行（書き方 A に似たチェックボックスの行を含む）や、チェックボックスのないステップがあれば、ステップを読めないものとし、推し量らずに要確認にして送らない。すべての PR がマージ済みの計画書は、ステップを読めるかどうかにかかわらず完了とする。受け入れる書き方と読めない条件は、[0113 要件定義書](../../tasks/0113_plan_step_reading/01_requirements.md) の 3 章が持つ。
+-   次のコマンドを `/runplan` と決めるために読む計画書のステップは、受け入れる 3 つの書き方（チェックボックスの行、2 章の見出し、2 章の 1 行の太字の段落）からだけ読む。2 章の中のステップに似た行（書き方 A に似たチェックボックスの行を含む）や、チェックボックスのないステップがあれば、ステップを読めないものとし、推し量らずに要確認にして送らない。すべての PR がマージ済みの計画書は、ステップを読めるかどうかにかかわらず完了とする。今の仕様は [chief_architecture.md](../chief_architecture.md) の 7 章（判定と副作用の条件）にあり、決定の出典は [0113 要件定義書](../../tasks/0113_plan_step_reading/01_requirements.md) の 3 章である。
     -   理由：規約はステップをチェックボックスで書くことだけを求め、`main` の計画書は見出しと太字の段落でもステップを書いている。それを読まないと、PR が残るタスクの次のコマンドが決まらない。似た行を読み替えないのは、残りのステップを見落として「残りなし」と誤って読まないためである（[0113 要件定義書](../../tasks/0113_plan_step_reading/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。

 ### マージコンフリクトの解消の自動送信
@@ -119,13 +119,13 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった

 ### 画面と transcript の読み取り

-報告を transcript から読む仕様は [0095 要件定義書](../../tasks/0095_chief_transcript_report/01_requirements.md) の F-001〜F-003 に、PR の記録の確かめの結果は [0095 設計書](../../tasks/0095_chief_transcript_report/02_architecture.md) の 1.2 にある。
+報告を transcript から読む今の仕様は [chief_architecture.md](../chief_architecture.md) の 6 章にあり、決定の出典は [0095 要件定義書](../../tasks/0095_chief_transcript_report/01_requirements.md) の F-001〜F-003 に、PR の記録の確かめの結果は [0095 設計書](../../tasks/0095_chief_transcript_report/02_architecture.md) の 1.2 にある。

 -   pane の画面とセッションの記録の文字列から判断することは、CLAUDE.md の「Declare, don't infer」の例外とする。対象は、エージェントが質問で止まっているかどうか、その質問が 1 つを選ぶ問いかどうか、権限の確認で止まっているかどうか、`/runpr` の報告の次のコマンド、モデル、入力欄に残っている文字、PR を作ったコマンドの報告の要約である。
     -   合わない画面とセッションの記録は広く解釈せず、「要確認」か「送らない」にする。transcript の行も、サンプルで確かめた形に合わなければ広く解釈しない。
     -   ただし、Claude Code の transcript の値や種類の名前だけが変わったときと、transcript のファイルがないときは、その pane に報告がないものとして読み、自動送信の前の NEXT の照合は通る（「結果」）。
     -   状況の表を組むときの、文字ごとの表示幅の数え方と、折り返しと切る位置も、表に入る外から来る値の中身から決めるので、同じく例外とする。その結果は表の配置にだけ使い、何を送るか、行の状態、急ぎの度合いを左右しない（[0102 要件定義書](../../tasks/0102_chief_text_table/01_requirements.md) の AC-28）。
-    -   理由：画面は Claude Code と OpenCode が描く外部の値で、セッションの記録は Claude Code が書く外部の値で、どちらも型で表せず、OpenCode の質問の画面は変えられないためである。範囲と境界は 0088 要件定義書の 5 章と、報告を transcript から読むように変えた [0095 要件定義書](../../tasks/0095_chief_transcript_report/01_requirements.md) の 5 章にあり、今の範囲と境界は CLAUDE.md の例外の一覧に書く。
+    -   理由：画面は Claude Code と OpenCode が描く外部の値で、セッションの記録は Claude Code が書く外部の値で、どちらも型で表せず、OpenCode の質問の画面は変えられないためである。範囲と境界の出典は 0088 要件定義書の 5 章と、報告を transcript から読むように変えた [0095 要件定義書](../../tasks/0095_chief_transcript_report/01_requirements.md) の 5 章にある。今の範囲と境界は CLAUDE.md の例外の一覧に書く。
 -   Claude Code の pane の最後の報告（`/runpr` の報告と、PR を作ったコマンドの報告）は、画面ではなく、そのセッションの transcript で最後に終わった応答の本文から読む。OpenCode の pane の報告は画面から読む。transcript を読めないときは画面に戻らず、送らないか、空にしないか、PR の説明から要約する。
     -   理由：Herdr は pane の高さの分の行しか返さない。そのため、画面より長い報告や、折り返しと描き直しで切り出せない報告があると、送るべきものが止まり、PR の詳細とリセットが働かなかった。transcript には報告の全文が折り返しなしで残る。画面に戻らないのは、壊れやすい経路を残さないためである。
 -   報告を使う確かめ（`/runpr` の報告の NEXT の照合、ブランチのない状態での自動送信と `/runpr` の pane のリセットでの NEXT と PR 番号、PR を作った pane のリセットの前の確かめ）は外さない。ただし、Claude Code の pane の NEXT の照合は、最後の報告だけを対象にする。
@@ -157,7 +157,7 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった
 ### 道具と実行の枠組み

 -   chief の判定と送信は、Go の開発用の道具 `chief`（`cmd/chief`・`internal/chief`）に置く。`/chief` のコマンドの定義は、道具の呼び出し、人への問い、報告の要約だけを持つ。道具は送る入力を、コマンドの種類と検証した引数から型で組み立て、文字列の入力を受け取らない。
-    -   理由：エージェントが手順を読み違えても、決まった範囲の外の入力が送られないようにし、判定をテストで確かめるためである（[ADR-0014](0014-deterministic-checks-in-go.md)）。構成は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.1「道具 `chief`」にある。
+    -   理由：エージェントが手順を読み違えても、決まった範囲の外の入力が送られないようにし、判定をテストで確かめるためである（[ADR-0014](0014-deterministic-checks-in-go.md)）。構成は [chief_architecture.md](../chief_architecture.md) の 2 章にある。
 -   自動の進行と通知は、chief のセッションが待ち受けている間だけ行い、セッションと別に常駐するプロセスを置かない。chief は同時に 1 つだけ動かす。chief の読み取り専用のネットワークの操作（PR・CI・レビューの状態の取得、`git fetch`、workspace を作る前の issue の取得）は、chief を起動したことで承認したものとする。
     -   理由：常駐のプロセスは、エージェントのいないところでほかのセッションへ入力を送ることになり、信頼の境界が広がるためである。1 つに限るのは、2 つの chief が同じコマンドを送らないためである。読み取りの承認は、人がいないときにも状態を読む必要があるためで、書き込みの操作は含めない。規則は CLAUDE.md の「Tool Execution Safety」に書く（F-010）。
 -   chief は Herdr の中でだけ動き、Herdr の出力を読めないときは状態を推し量らずに「要確認」とする。
@@ -172,14 +172,13 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった

     道具の削除のサブコマンドは、確認なしに許す道具の呼び出しから外し、呼ぶたびに Claude Code の許可の確認を出させる。
     -   理由：画面や PR の説明の指示に従ったセッションが、任意の入力を送ること、chief の確かめが頼る状態とセッションの値を偽ること、タスクの pane を閉じるなどして作業を失わせること、道具を通さずに worktree と workspace を作るか消すことを防ぐためである。削除のサブコマンドに許可の確認を出させるのは、消すかどうかの人の確認を、セッションの外でも求めるためである。読み取りは、ほかのセッションが使うことがあり、送ることも偽ることもできないので、拒まない。
-    -   拒む呼び出しの一覧は [0088 実装計画書](../../tasks/0088_chief_of_staff/03_implementation_plan.md) の「計画の際に決めたこと」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.8 にある。パスで書いた呼び出しなど、規則の外の書き方は残る危険とする（0088 設計書の 5.2）。
+    -   拒む呼び出しの一覧は [chief_guide.md](../guides/chief_guide.md) の 6 章にあり、決定の出典は [0088 実装計画書](../../tasks/0088_chief_of_staff/03_implementation_plan.md) の「計画の際に決めたこと」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.8 である。パスで書いた呼び出しなど、規則の外の書き方は残る危険とする（0088 設計書の 5.2）。

 ### 今の仕様の文書とプロトコル番号

--   chief の今の仕様は、`docs/dev/` の直下の開発者向けの文書 chief_architecture.md が持つ。持つのは、構成要素と信頼の境界、`/chief` の定義と道具の契約、状態のディレクトリ・ロック・記録、道具が書き出すもの（中身・書き出す先・残る期間・信頼できない文字列を含むか）、画面と transcript の読み取りの扱い、判定と副作用の条件の要約（判定ごとに、読むもの、副作用の条件にあたるか、細部の置き場所を示す）、プロトコル番号を上げる基準である。読み取りの範囲は CLAUDE.md の例外の一覧が持つ。この文書が述べる事柄を変える PR は、同じ PR でこの文書を直す。
+-   chief の今の仕様は、`docs/dev/` の直下の開発者向けの文書 [chief_architecture.md](../chief_architecture.md) が持つ。持つのは、構成要素と信頼の境界、`/chief` の定義と道具の契約、状態のディレクトリ・ロック・記録、道具が書き出すもの（中身・書き出す先・残る期間・信頼できない文字列を含むか）、画面と transcript の読み取りの扱い、判定と副作用の条件の要約（判定ごとに、読むもの、副作用の条件にあたるか、細部の置き場所を示す）、プロトコル番号を上げる基準、テストのしかたである。読み取りの範囲は CLAUDE.md の例外の一覧が持つ。この文書が述べる事柄を変える PR は、同じ PR でこの文書を直す。
     -   理由：今の仕様が 10 本のタスクの設計書に分かれ、後のタスクが上書きした部分を読む人が自分で差し引く必要があったためである。同じ PR で直さなければ、文書はまた古くなる（[0117 要件定義書](../../tasks/0117_chief_architecture_doc/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）。
-    -   食い違い：この文書はまだなく、[issue #1055](https://github.com/isseis/yt2column/issues/1055) の実装で作る。それまで、今の仕様はこの ADR の各節が指す要件定義書・設計書と chief_guide にあり、番号を上げる基準の一覧は 0117 要件定義書の F-003 にある。
--   chief のプロトコル番号は、道具と `/chief` の定義の間の約束を変える PR と、pane へ送る・自動で送る・workspace と worktree を作る・消すといった副作用の条件を変える PR で上げる。約束を変える追加でも、古い道具と新しい定義、新しい道具と古い定義のどちらの組み合わせも黙って誤らないことを示す 3 つの条件がすべて成り立てば、上げなくてよい。状況の表の分類など、副作用の条件でない判定だけを変える PR は上げない。定義だけを変える PR も、定義が読まなかった出力の項目を読み始めるか、示すだけだった項目で分岐し始めるときは上げる。約束と 3 つの条件の一覧は chief_architecture.md のプロトコル番号の節にあり、番号の決め方とマージの時点の確かめは CLAUDE.md の「Protocol numbers」にある。
+-   chief のプロトコル番号は、道具と `/chief` の定義の間の約束を変える PR と、pane へ送る・自動で送る・workspace と worktree を作る・消すといった副作用の条件を変える PR で上げる。約束を変える追加でも、古い道具と新しい定義、新しい道具と古い定義のどちらの組み合わせも黙って誤らないことを示す 3 つの条件がすべて成り立てば、上げなくてよい。状況の表の分類など、副作用の条件でない判定だけを変える PR は上げない。定義だけを変える PR も、定義が読まなかった出力の項目を読み始めるか、示すだけだった項目で分岐し始めるときは上げる。約束と 3 つの条件の一覧は [chief_architecture.md](../chief_architecture.md) のプロトコル番号の章にあり、番号の決め方とマージの時点の確かめは CLAUDE.md の「Protocol numbers」にある。
     -   理由：古い定義と新しい道具の組み合わせが黙って誤ることと、入れ直しを忘れた古い道具が取り消せない副作用を古い条件で起こし続けることを防ぎ、害のない追加では番号を上げないためである。上げるたびに道具の入れ直しが要り、並行するタスクとの番号の衝突も増える。`fixprguard` の基準の形にそろえ、判定の変更は副作用の条件に絞る（[0117 要件定義書](../../tasks/0117_chief_architecture_doc/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）。

 ### workspace の作成と削除
@@ -193,7 +192,7 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった
     -   エージェントは、人の代わりに許可や信頼を与える引数を付けずに起動し、フォルダの信頼を問う画面などが出たら送らずに人が答えるよう示す。
     -   パスと名前は issue 番号だけから決める。同じパスの worktree か workspace がすでにあれば作らない。
     -   途中で失敗したら、そこで止まって人に示し、作ったものは消さない。
-    -   理由：人が issue 番号を答えるだけでタスクを始められるようにし、タスクにならない番号（存在しない issue、閉じた issue、下書きの issue、PR）で、片付けられずに残る workspace を作らないためである。許可や信頼を与える引数を付けないのは、chief が人の代わりにリポジトリの hooks や設定を信頼しないためである。名前を issue 番号だけから決めるのは、issue の題名などの中身で振る舞いを選ばず（CLAUDE.md の「Declare, don't infer」）、chief が入力を送れなくなる文字をパスに入れないためである。detached にするのは、`/mkreq` がタスクのブランチを作った後に一時的なブランチを残さないためである。仕様は [0092 要件定義書](../../tasks/0092_chief_task_workspace/01_requirements.md) の F-001〜F-004 にある。
+    -   理由：人が issue 番号を答えるだけでタスクを始められるようにし、タスクにならない番号（存在しない issue、閉じた issue、下書きの issue、PR）で、片付けられずに残る workspace を作らないためである。許可や信頼を与える引数を付けないのは、chief が人の代わりにリポジトリの hooks や設定を信頼しないためである。名前を issue 番号だけから決めるのは、issue の題名などの中身で振る舞いを選ばず（CLAUDE.md の「Declare, don't infer」）、chief が入力を送れなくなる文字をパスに入れないためである。detached にするのは、`/mkreq` がタスクのブランチを作った後に一時的なブランチを残さないためである。今の仕様は [chief_architecture.md](../chief_architecture.md) の 7 章にあり、決定の出典は [0092 要件定義書](../../tasks/0092_chief_task_workspace/01_requirements.md) の F-001〜F-004 である。
 -   chief が削除を提案するのは、chief が作った workspace のうち、記録した issue を `対応 issue` とするタスクが `origin/main` でちょうど 1 つあり、完了したものだけにする。人が選び、消すかどうかの問いに答えたときだけ、消す直前に次を確かめ直し、通ったときだけ workspace を閉じて worktree を消す。
     -   追跡していないファイルを含めて変更がないこと。
     -   HEAD が `origin/main` から届くこと。
@@ -205,7 +204,7 @@ chief は人の代わりに判断せず、人が選んだ答えと決まった

 ### 状況の表

-仕様は [0093 要件定義書](../../tasks/0093_chief_status_sheet/01_requirements.md) の F-001〜F-006 と、表の形については [0102 要件定義書](../../tasks/0102_chief_text_table/01_requirements.md) の F-001〜F-005 にある。
+今の仕様は [chief_architecture.md](../chief_architecture.md) の 7 章にあり、決定の出典は [0093 要件定義書](../../tasks/0093_chief_status_sheet/01_requirements.md) の F-001〜F-006 と、表の形については [0102 要件定義書](../../tasks/0102_chief_text_table/01_requirements.md) の F-001〜F-005 である。

 -   状況の表の各行に、急ぎの度合いの印（すぐ 🛑・あとで 🔶・待つ 🟢。色だけでなく形でも区別でき、表の上の凡例で意味を示す）と、タスクの作業項目の表示（`origin/main` の文書から決まる）と状態の詳細を示し、行を急ぎの度合いの順に並べる。次に動くのが人か AI か（番）も道具が決めて出力に含めるが、表には示さない。これらは道具が決める。
     -   理由：人が表を一目見て、何から手を付ければよいかを分かるようにするためである。道具が決めるのは、セッションに判断を残さず、判定をテストで確かめるためである。番を表に示さないのは、表を pane の幅に収めるためで、番は状態から分かる。印の形を変えたのは、色だけに頼らないためと、「待」が「人の対応を待っている」とも読めたためである。
diff --git a/docs/dev/conventions/dev_tools.md b/docs/dev/conventions/dev_tools.md
index 1f15ba59c0a6e4c31a7cdda116bf91198ad5827c..150c399474941f707526a17b241b4e022387e37c 100644
--- a/docs/dev/conventions/dev_tools.md
+++ b/docs/dev/conventions/dev_tools.md
@@ -63,7 +63,7 @@ The contract is written in `CLAUDE.md` ("Build Commands", "Test Commands",
 [_context.md](../../../.claude/commands/_context.md), and in the guides of the
 installed tools: [fixprguard_guide.md](../guides/fixprguard_guide.md),
 [mergepr_guide.md](../guides/mergepr_guide.md),
-[chief_guide.md](../guides/chief_guide.md), and
+[chief_architecture.md](../chief_architecture.md), and
 [numreserve_guide.md](../guides/numreserve_guide.md).

 ## Rule 5: Install tools from main; agents never install or build them
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-17 の記録

Diff record (AC-17):

- Compared paths: `docs/dev/guides/chief_guide.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `7798f4b4e610482ef66ee355846a3ea080236676`

Path diff:

```text
diff --git a/docs/dev/guides/chief_guide.md b/docs/dev/guides/chief_guide.md
index 0c48af5cdc7083640de019e26f27426da522ae40..fa9ae74794ff881482eaccd5dc3389ec2dbaba55 100644
--- a/docs/dev/guides/chief_guide.md
+++ b/docs/dev/guides/chief_guide.md
@@ -1,6 +1,6 @@
 # chief 利用ガイド

-chief は、Herdr の workspace で並行して進むタスクを見渡し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る開発用の道具である。人が示した issue からタスク用の worktree と workspace を作り、終わったタスクの workspace を人の確認を得て消すこともできる。Go の道具 `chief`（`cmd/chief`・`internal/chief`）と、それを呼ぶコマンド `/chief`（`.claude/commands/chief.md`）からなる。判定と送信はすべて道具が行い、`/chief` のセッションは道具の出力を人に示し、人の選んだものを道具の引数で渡すだけである（[ADR-0046](../adr/0046-chief-of-staff.md)、[ADR-0014](../adr/0014-deterministic-checks-in-go.md)）。要件と設計は [0088 要件定義書](../../tasks/0088_chief_of_staff/01_requirements.md) と [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) に、作成と削除の要件と設計は [0092 要件定義書](../../tasks/0092_chief_task_workspace/01_requirements.md) と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) にある。
+chief は、Herdr の workspace で並行して進むタスクを見渡し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る開発用の道具である。人が示した issue からタスク用の worktree と workspace を作り、終わったタスクの workspace を人の確認を得て消すこともできる。Go の道具 `chief`（`cmd/chief`・`internal/chief`）と、それを呼ぶコマンド `/chief`（`.claude/commands/chief.md`）からなる。判定と送信はすべて道具が行い、`/chief` のセッションは道具の出力を人に示し、人の選んだものを道具の引数で渡すだけである（[ADR-0046](../adr/0046-chief-of-staff.md)、[ADR-0014](../adr/0014-deterministic-checks-in-go.md)）。chief の今の仕様は [chief_architecture.md](../chief_architecture.md) にあり、決定の理由と経緯は [ADR-0046](../adr/0046-chief-of-staff.md) にある。

 ## 1. 前提条件

@@ -29,7 +29,7 @@ make install-chief
 |---|---|
 | プロトコル番号 | `6` |

-番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。番号を上げる PR では、その PR の `/runpr` の前に、利用者がその PR のブランチで入れ直す。
+番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。引数の形が合わない呼び出しは、番号の照合より前に使い方の誤り（終了コード `2`）になる。番号を上げる PR では、その PR の `/runpr` の前に、利用者がその PR のブランチで入れ直す。組み合わせの誤りに当たったときの見分け方は [chief_architecture.md](../chief_architecture.md) の 8 章にある。

 **入れ直す順。** 動いている `/chief` を止めてから `make install-chief` を実行し、`/chief` を起動し直す。番号を上げた道具を `/chief` を止めずに入れると、動いている `/chief` は古い定義の番号のまま次のラウンドを走らせ、新しい道具に `protocol_mismatch` で拒まれる。定義はこのとき `make install-chief` での入れ直しを促して止まるが、道具はすでに新しいので、入れ直しても直らない。手当ては、`/chief` を起動し直して新しい定義を読ませることである。

@@ -39,9 +39,9 @@ make install-chief
 2.  chief の workspace の `.claude/commands/chief.md` を新しい版にし（`main` を更新する）、`make install-chief` を実行する。
 3.  `/chief` を起動し直す。

-**古い版に戻すとき。** 道具と `/chief` の定義を、同じコミットから入れる。新しい道具が書いた送信の記録は `kind`・`arg` の欄を持ち、古い道具は知らない欄を持つ記録を読まない。戻す前に、新しい道具の `chief start` を実行して、終わりの記録のない送信を送信の停止に変える（7 章の「片付け」）。停止した workspace があれば、その pane で送信が何をしたかを確かめてから停止を解く。そのうえで、最後の送信から 25 分たってから `chief start` をもう一度実行して終わった送信の記録を片付けさせるか、状態のディレクトリの workspace ごとの送信の記録（`send-*.json`）を消す。終わりの記録のない送信を停止に変える前に記録を消すと、送れたかが分からない送信が分からなくなる（5.1 の「送れたかが分からない送信は送り直さない」）。
+**古い版に戻すとき。** 道具と `/chief` の定義を、同じコミットから入れる。新しい道具が書いた送信の記録は `kind`・`arg` の欄を持ち、古い道具は知らない欄を持つ記録を読まない。戻す前に、新しい道具の `chief start` を実行して、終わりの記録のない送信を送信の停止に変える（[chief_architecture.md](../chief_architecture.md) の 4.5）。停止した workspace があれば、その pane で送信が何をしたかを確かめてから停止を解く。そのうえで、最後の送信から 25 分たってから `chief start` をもう一度実行して終わった送信の記録を片付けさせるか、状態のディレクトリの workspace ごとの送信の記録（`send-*.json`）を消す。終わりの記録のない送信を停止に変える前に記録を消すと、送れたかが分からない送信が分からなくなる（5.1 の「送れたかが分からない送信は送り直さない」）。

-番号を変えずに道具と定義が変わることもある（たとえばサブコマンドや出力の欄が増える）。道具を入れ直したら、動いている `/chief` のセッションを終えて、`/chief` を起動し直す。古いセッションは、読み込んだときの定義のまま動き続けるためである。
+番号を上げるかどうかは [chief_architecture.md](../chief_architecture.md) の 8 章の基準で決める。道具を入れ直したら、動いている `/chief` のセッションを終えて、`/chief` を起動し直す。古いセッションは、読み込んだときの定義のまま動き続けるためである。

 `.claude/settings.json` は `Bash(chief *)` を許すので、待ち受けの間に許可を尋ねない。道具は `chief` の語で始まり、ほかのコマンドと連ねない形でだけ呼ぶ（`/chief` の定義の「Calling the tool」）。ただし、削除の `chief remove` は `ask` の規則 `Bash(chief remove *)` に当たるので、呼ぶたびに Claude Code が許可を尋ねる（6 章）。

@@ -166,29 +166,7 @@ chief が作った workspace のうち、作ったときの issue を `対応 is

 ## 4. サブコマンドと終了コード

-どのサブコマンドも `--protocol N` を必須とし、標準出力に 1 つの JSON を書く。サブコマンドの細部は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.1.1 と、[0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.1 にある。
-
-| サブコマンド | 何をするか |
-|---|---|
-| `start` | ロックを取り、状態のディレクトリを確かめ、古い記録（ほかのセッションの履歴の記録を含む）を片付け、終わり方の分からない送信を送信の停止に変える |
-| `snapshot` | 状況を読み、状況の表、人の判断を待つもの、新しい通知を返す。送らない |
-| `tick` | `snapshot` に加えて、PR が開いているのを見た記録と、worktree ごとの履歴の記録（PR とブランチについて確かめたこと）を作り、自動送信の条件を満たすものを送る |
-| `wait` | 前のラウンドの基準から、タスクの workspace のエージェントの状態が変わるまで待つ（既定は 10 分で時間切れ） |
-| `send` | 人の選んだコマンドを送る。タスクのコマンド（`mkarch`・`mkplan`・`runplan`・`runplan-takeover`）は、人の選んだ項目の `id` を `--item` で受け取り、送る直前に作り直した行が同じ項目を提案しているときだけ送る。`/runpr` の後は、PR を作ったコマンドの pane を空にする |
-| `relay` | 人の選んだ答えを、質問で止まっているエージェントへ中継する |
-| `reset` | 人の選んだ、見送ったリセットの項目を、確かめ直してから送る（`/clear` か `/new`） |
-| `detail` | workspace の開いている PR の詳細を集め、控える |
-| `release` | 人の選択で、workspace の送信の停止を解く |
-| `targets` | issue 番号の形を確かめ、issue を読んで題名か拒否を返し、通れば `/mkreq` を提案しているタスクのない workspace を返す。何も作らない |
-| `create` | issue を確かめ、worktree と workspace を作ってエージェントを起動し、`/mkreq` を送る（3.3） |
-| `remove` | 削除の確かめを行い直し、通れば workspace を閉じて worktree を消す（3.4） |
-
-| 終了コード | 意味 |
-|---|---|
-| `0` | 処理を終えた。送らなかった判断、作らなかった判断（issue 番号の形の違いを含む）、消さなかった判断、作成と削除が途中で止まったことも含み、出力が理由を持つ |
-| `1` | 道具の失敗（読めない入力、記録を作れない、外部のコマンドの失敗、`wait` の時間切れ） |
-| `2` | 使い方の誤り |
-| `3` | 何もせずに拒んだ（Herdr の外、ほかの chief がロックを持つ、`start` していない、`--protocol` が合わない、状態のディレクトリの土台がない、`HERDR_PANE_ID` がほかの worktree の pane を指す） |
+どのサブコマンドも `--protocol` を必須とし、標準出力に 1 つの JSON を書く。サブコマンドとそのフラグ、出力の JSON の項目、終了コードは [chief_architecture.md](../chief_architecture.md) の 3 章にある。

 ## 5. 保証すること・しないこと

@@ -233,21 +211,10 @@ chief のセッションが道具を通さずに入力を送らず、タスク

 ## 7. 状態のディレクトリ

-記録とロックは、リポジトリの外の `$XDG_STATE_HOME/yt2column/chief/`（`XDG_STATE_HOME` が絶対パスでなければ `$HOME/.local/state/yt2column/chief/`）に置く。`start` の出力の `state_dir` が、解決したパスである。ファイルは利用者だけが読み書きでき、同じ名前の記録がすでにあれば作らない。
-
-| 記録 | 置き場所 | 何のためか |
-|---|---|---|
-| ロック | `lock.json` | chief を 1 つに限る |
-| 通知の記録 | `notice-*.json` | 同じものを 2 回通知しない。待っている時間による並べ方。セッションの中の項目の番号（人が番号で答えた項目を見分ける） |
-| ラウンドの記録・PR が開いているのを見た記録 | `round-*.json`・`observe-*.json` | 自動送信の条件（chief の待ち受けの中でマージを見たこと） |
-| 自動送信の記録 | `autosend-<PR 番号>.json` | マージした PR 1 件につき 1 回に限る |
-| 作成の排他 | 下のディレクトリ `create` | 作成と、`tick` の自動送信を重ねない |
-| workspace ごとの記録 | worktree の正規のパスの SHA-256 の 16 進を名前にした下のディレクトリ | 送信の記録（`send-*.json`）、PR の詳細の控え（`detail-*.json`）、報告の控え（`report-*.json`。PR の詳細の控えを作った後に書かれた、PR を作ったコマンドの報告を、人が PR の詳細を見直したときに控える）、送信の停止（`stop.json`）、見送りの記録（`skip-*.json`。報告が読めずに見送ったリセットを、後のラウンドで人に示すためのもの）、作ったことの記録（`created.json`。chief が作った worktree のパスと issue 番号。削除の候補を名指すことにだけ読む）、履歴の記録（`trace-*.json`。chief の pane のセッションごとに、`tick` が PR とブランチについて確かめたことを 1 件ずつ書く。`main` をチェックアウトした worktree のタスクの特定にだけ読む） |
+記録とロックの場所とその決め方、記録の種類ごとの中身・何のために読むか・書くとき・消えるときは、[chief_architecture.md](../chief_architecture.md) の 4 章にある。`start` の出力の `state_dir` が、解決したパスである。

 -   **送信の停止を解く。** `/chief` の問いで `停止を解く: <workspace の名前>` と答えるか、ほかの方法がないときだけ、その workspace のディレクトリの `stop.json` を人が消す。停止の記録を読めない workspace（表に、停止の記録を読めないと示される）には `停止を解く` は示されないので、`stop.json` の中身と pane を人が確かめてから消す。停止の理由と、そのときの画面は、状況の表に示される。消す前に、止まった送信がその pane で何をしたかを確かめる。
--   **見送りの記録。** ラウンドは見送りの記録を読むだけで、作りも消しもしない。記録が消えるのは、人が選んだリセットを送る直前、chief がその pane へコマンドかリセットを打てたとき、`start` の片付けのときである。決まった形に合わない記録は項目にならず、ラウンドの `problems` にパスと理由が示される。その記録は、人が手で消してよい。
--   **片付け。** `start` が、終わったセッションのラウンド・観測・通知の記録、始めと終わりの両方を書いた古い送信の記録、14 日より古い PR の詳細の控えと報告の控えと見送りの記録（読めない見送りの記録は、ファイルの更新が 14 日より古いもの）を消す。終わりの記録のない送信の記録は消さず、送信の停止に変える。自動送信の記録は消さない（消すと、同じマージで 2 回送りうる）。ほかのセッションの履歴の記録も消す（3.1 の「要確認から戻す」）。
--   **作るときと消すときの記録。** `create` は、作る前に、同じパスの workspace ごとの記録（前の使い方の送信の停止など）をすべて消してから、作ったことの記録を書く。`remove` は、worktree を消し終えた後に、その workspace ごとの記録をすべて消す。作ったことの記録は最後に消す。
+-   **見送りの記録。** ラウンドは見送りの記録を読むだけで、作りも消しもしない。決まった形に合わない記録は項目にならず、ラウンドの `problems` にパスと理由が示される。その記録は、人が手で消してよい。
 -   **すべてを消す。** chief が動いていないときに、ディレクトリごと消してよい。消した後は、chief を起動する前のマージを自動では送らず、提案として示す。作ったことの記録も消えるので、chief が作った workspace は削除の提案に出なくなる。

 ## 8. 既知の制限
@@ -272,7 +239,7 @@ chief のセッションが道具を通さずに入力を送らず、タスク
 -   **打った直後のラウンドは 1 つ前のコマンドを示しうる。** 人が Claude Code の pane でコマンドを打った直後で、Claude Code がまだその行を記録に 1 バイトも書いていないラウンドは、1 つ前のコマンドを示す。書きかけの行は 200 ms の間隔で 3 回まで読み直し、それでも終わらなければ何も示さない（`partial_line`）が、書き始める前は見分けられない。
 -   **`/clear` の直後は古いセッションのコマンドを示しうる。** ラウンドの初めに読んだ Herdr のセッションの識別が、`/clear` の直後でまだ古いと、古いセッションの記録のコマンドを示しうる。
 -   **人が引数なしで打ったコマンドは示されない。** 人が引数なしで打ったコマンド（`/runpr` など）は、記録の形が人の入力の読み取りの受け付ける形と違い、「そのほかの人の入力」と読まれる。その前にタスクのコマンドがあればそれに `＋指示` を添えて示し、なければ何も示さない（`not_task_command`）。たとえば `/runplan` の後に引数なしの `/runpr` を打つと、`/runpr` が走っている間も、内容の列には前の `/runplan` のコマンドが `＋指示` を添えて示される。
--   **起動し直すと長く走る OpenCode のコマンドは示されない。** `chief start` は、始めと終わりの両方を書いた 25 分より古い送信の記録を消す（7 章の「片付け」）。そのため、長く走る OpenCode のコマンドは、chief を起動し直した後に示されなくなる。
+-   **起動し直すと長く走る OpenCode のコマンドは示されない。** `chief start` は、始めと終わりの両方を書いた 25 分より古い送信の記録を消す（[chief_architecture.md](../chief_architecture.md) の 4.5）。そのため、長く走る OpenCode のコマンドは、chief を起動し直した後に示されなくなる。
 -   **エージェントのいない workspace は提案されない。** chief はエージェントの作業ディレクトリから workspace を見つけるので、エージェントがすべて終了した workspace は表に出ず、削除も提案されない。人が端末で片付ける。
 -   **作ったことの記録だけが残った worktree は表に出ない。** 作成が workspace を作る前に止まったときや、削除が workspace を閉じた後に止まったときは、worktree と記録が残っても表に出ない。結果に示された残ったものを、人が端末で片付ける。
 -   **作るか消すと、ほかの workspace の `make coverage-compare` が止まる。** 比較は、走っている間の `git worktree list` の変化を見つけると止まる（`internal/coveragecompare/userstate.go`）。比較の間は作成と削除を避けるか、止まったら走らせ直す。
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-18 の記録

Diff record (AC-18):

- Compared paths: `docs/dev/guides/chief_guide.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `7798f4b4e610482ef66ee355846a3ea080236676`

Path diff:

```text
diff --git a/docs/dev/guides/chief_guide.md b/docs/dev/guides/chief_guide.md
index 0c48af5cdc7083640de019e26f27426da522ae40..fa9ae74794ff881482eaccd5dc3389ec2dbaba55 100644
--- a/docs/dev/guides/chief_guide.md
+++ b/docs/dev/guides/chief_guide.md
@@ -1,6 +1,6 @@
 # chief 利用ガイド

-chief は、Herdr の workspace で並行して進むタスクを見渡し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る開発用の道具である。人が示した issue からタスク用の worktree と workspace を作り、終わったタスクの workspace を人の確認を得て消すこともできる。Go の道具 `chief`（`cmd/chief`・`internal/chief`）と、それを呼ぶコマンド `/chief`（`.claude/commands/chief.md`）からなる。判定と送信はすべて道具が行い、`/chief` のセッションは道具の出力を人に示し、人の選んだものを道具の引数で渡すだけである（[ADR-0046](../adr/0046-chief-of-staff.md)、[ADR-0014](../adr/0014-deterministic-checks-in-go.md)）。要件と設計は [0088 要件定義書](../../tasks/0088_chief_of_staff/01_requirements.md) と [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) に、作成と削除の要件と設計は [0092 要件定義書](../../tasks/0092_chief_task_workspace/01_requirements.md) と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) にある。
+chief は、Herdr の workspace で並行して進むタスクを見渡し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る開発用の道具である。人が示した issue からタスク用の worktree と workspace を作り、終わったタスクの workspace を人の確認を得て消すこともできる。Go の道具 `chief`（`cmd/chief`・`internal/chief`）と、それを呼ぶコマンド `/chief`（`.claude/commands/chief.md`）からなる。判定と送信はすべて道具が行い、`/chief` のセッションは道具の出力を人に示し、人の選んだものを道具の引数で渡すだけである（[ADR-0046](../adr/0046-chief-of-staff.md)、[ADR-0014](../adr/0014-deterministic-checks-in-go.md)）。chief の今の仕様は [chief_architecture.md](../chief_architecture.md) にあり、決定の理由と経緯は [ADR-0046](../adr/0046-chief-of-staff.md) にある。

 ## 1. 前提条件

@@ -29,7 +29,7 @@ make install-chief
 |---|---|
 | プロトコル番号 | `6` |

-番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。番号を上げる PR では、その PR の `/runpr` の前に、利用者がその PR のブランチで入れ直す。
+番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。引数の形が合わない呼び出しは、番号の照合より前に使い方の誤り（終了コード `2`）になる。番号を上げる PR では、その PR の `/runpr` の前に、利用者がその PR のブランチで入れ直す。組み合わせの誤りに当たったときの見分け方は [chief_architecture.md](../chief_architecture.md) の 8 章にある。

 **入れ直す順。** 動いている `/chief` を止めてから `make install-chief` を実行し、`/chief` を起動し直す。番号を上げた道具を `/chief` を止めずに入れると、動いている `/chief` は古い定義の番号のまま次のラウンドを走らせ、新しい道具に `protocol_mismatch` で拒まれる。定義はこのとき `make install-chief` での入れ直しを促して止まるが、道具はすでに新しいので、入れ直しても直らない。手当ては、`/chief` を起動し直して新しい定義を読ませることである。

@@ -39,9 +39,9 @@ make install-chief
 2.  chief の workspace の `.claude/commands/chief.md` を新しい版にし（`main` を更新する）、`make install-chief` を実行する。
 3.  `/chief` を起動し直す。

-**古い版に戻すとき。** 道具と `/chief` の定義を、同じコミットから入れる。新しい道具が書いた送信の記録は `kind`・`arg` の欄を持ち、古い道具は知らない欄を持つ記録を読まない。戻す前に、新しい道具の `chief start` を実行して、終わりの記録のない送信を送信の停止に変える（7 章の「片付け」）。停止した workspace があれば、その pane で送信が何をしたかを確かめてから停止を解く。そのうえで、最後の送信から 25 分たってから `chief start` をもう一度実行して終わった送信の記録を片付けさせるか、状態のディレクトリの workspace ごとの送信の記録（`send-*.json`）を消す。終わりの記録のない送信を停止に変える前に記録を消すと、送れたかが分からない送信が分からなくなる（5.1 の「送れたかが分からない送信は送り直さない」）。
+**古い版に戻すとき。** 道具と `/chief` の定義を、同じコミットから入れる。新しい道具が書いた送信の記録は `kind`・`arg` の欄を持ち、古い道具は知らない欄を持つ記録を読まない。戻す前に、新しい道具の `chief start` を実行して、終わりの記録のない送信を送信の停止に変える（[chief_architecture.md](../chief_architecture.md) の 4.5）。停止した workspace があれば、その pane で送信が何をしたかを確かめてから停止を解く。そのうえで、最後の送信から 25 分たってから `chief start` をもう一度実行して終わった送信の記録を片付けさせるか、状態のディレクトリの workspace ごとの送信の記録（`send-*.json`）を消す。終わりの記録のない送信を停止に変える前に記録を消すと、送れたかが分からない送信が分からなくなる（5.1 の「送れたかが分からない送信は送り直さない」）。

-番号を変えずに道具と定義が変わることもある（たとえばサブコマンドや出力の欄が増える）。道具を入れ直したら、動いている `/chief` のセッションを終えて、`/chief` を起動し直す。古いセッションは、読み込んだときの定義のまま動き続けるためである。
+番号を上げるかどうかは [chief_architecture.md](../chief_architecture.md) の 8 章の基準で決める。道具を入れ直したら、動いている `/chief` のセッションを終えて、`/chief` を起動し直す。古いセッションは、読み込んだときの定義のまま動き続けるためである。

 `.claude/settings.json` は `Bash(chief *)` を許すので、待ち受けの間に許可を尋ねない。道具は `chief` の語で始まり、ほかのコマンドと連ねない形でだけ呼ぶ（`/chief` の定義の「Calling the tool」）。ただし、削除の `chief remove` は `ask` の規則 `Bash(chief remove *)` に当たるので、呼ぶたびに Claude Code が許可を尋ねる（6 章）。

@@ -166,29 +166,7 @@ chief が作った workspace のうち、作ったときの issue を `対応 is

 ## 4. サブコマンドと終了コード

-どのサブコマンドも `--protocol N` を必須とし、標準出力に 1 つの JSON を書く。サブコマンドの細部は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.1.1 と、[0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.1 にある。
-
-| サブコマンド | 何をするか |
-|---|---|
-| `start` | ロックを取り、状態のディレクトリを確かめ、古い記録（ほかのセッションの履歴の記録を含む）を片付け、終わり方の分からない送信を送信の停止に変える |
-| `snapshot` | 状況を読み、状況の表、人の判断を待つもの、新しい通知を返す。送らない |
-| `tick` | `snapshot` に加えて、PR が開いているのを見た記録と、worktree ごとの履歴の記録（PR とブランチについて確かめたこと）を作り、自動送信の条件を満たすものを送る |
-| `wait` | 前のラウンドの基準から、タスクの workspace のエージェントの状態が変わるまで待つ（既定は 10 分で時間切れ） |
-| `send` | 人の選んだコマンドを送る。タスクのコマンド（`mkarch`・`mkplan`・`runplan`・`runplan-takeover`）は、人の選んだ項目の `id` を `--item` で受け取り、送る直前に作り直した行が同じ項目を提案しているときだけ送る。`/runpr` の後は、PR を作ったコマンドの pane を空にする |
-| `relay` | 人の選んだ答えを、質問で止まっているエージェントへ中継する |
-| `reset` | 人の選んだ、見送ったリセットの項目を、確かめ直してから送る（`/clear` か `/new`） |
-| `detail` | workspace の開いている PR の詳細を集め、控える |
-| `release` | 人の選択で、workspace の送信の停止を解く |
-| `targets` | issue 番号の形を確かめ、issue を読んで題名か拒否を返し、通れば `/mkreq` を提案しているタスクのない workspace を返す。何も作らない |
-| `create` | issue を確かめ、worktree と workspace を作ってエージェントを起動し、`/mkreq` を送る（3.3） |
-| `remove` | 削除の確かめを行い直し、通れば workspace を閉じて worktree を消す（3.4） |
-
-| 終了コード | 意味 |
-|---|---|
-| `0` | 処理を終えた。送らなかった判断、作らなかった判断（issue 番号の形の違いを含む）、消さなかった判断、作成と削除が途中で止まったことも含み、出力が理由を持つ |
-| `1` | 道具の失敗（読めない入力、記録を作れない、外部のコマンドの失敗、`wait` の時間切れ） |
-| `2` | 使い方の誤り |
-| `3` | 何もせずに拒んだ（Herdr の外、ほかの chief がロックを持つ、`start` していない、`--protocol` が合わない、状態のディレクトリの土台がない、`HERDR_PANE_ID` がほかの worktree の pane を指す） |
+どのサブコマンドも `--protocol` を必須とし、標準出力に 1 つの JSON を書く。サブコマンドとそのフラグ、出力の JSON の項目、終了コードは [chief_architecture.md](../chief_architecture.md) の 3 章にある。

 ## 5. 保証すること・しないこと

@@ -233,21 +211,10 @@ chief のセッションが道具を通さずに入力を送らず、タスク

 ## 7. 状態のディレクトリ

-記録とロックは、リポジトリの外の `$XDG_STATE_HOME/yt2column/chief/`（`XDG_STATE_HOME` が絶対パスでなければ `$HOME/.local/state/yt2column/chief/`）に置く。`start` の出力の `state_dir` が、解決したパスである。ファイルは利用者だけが読み書きでき、同じ名前の記録がすでにあれば作らない。
-
-| 記録 | 置き場所 | 何のためか |
-|---|---|---|
-| ロック | `lock.json` | chief を 1 つに限る |
-| 通知の記録 | `notice-*.json` | 同じものを 2 回通知しない。待っている時間による並べ方。セッションの中の項目の番号（人が番号で答えた項目を見分ける） |
-| ラウンドの記録・PR が開いているのを見た記録 | `round-*.json`・`observe-*.json` | 自動送信の条件（chief の待ち受けの中でマージを見たこと） |
-| 自動送信の記録 | `autosend-<PR 番号>.json` | マージした PR 1 件につき 1 回に限る |
-| 作成の排他 | 下のディレクトリ `create` | 作成と、`tick` の自動送信を重ねない |
-| workspace ごとの記録 | worktree の正規のパスの SHA-256 の 16 進を名前にした下のディレクトリ | 送信の記録（`send-*.json`）、PR の詳細の控え（`detail-*.json`）、報告の控え（`report-*.json`。PR の詳細の控えを作った後に書かれた、PR を作ったコマンドの報告を、人が PR の詳細を見直したときに控える）、送信の停止（`stop.json`）、見送りの記録（`skip-*.json`。報告が読めずに見送ったリセットを、後のラウンドで人に示すためのもの）、作ったことの記録（`created.json`。chief が作った worktree のパスと issue 番号。削除の候補を名指すことにだけ読む）、履歴の記録（`trace-*.json`。chief の pane のセッションごとに、`tick` が PR とブランチについて確かめたことを 1 件ずつ書く。`main` をチェックアウトした worktree のタスクの特定にだけ読む） |
+記録とロックの場所とその決め方、記録の種類ごとの中身・何のために読むか・書くとき・消えるときは、[chief_architecture.md](../chief_architecture.md) の 4 章にある。`start` の出力の `state_dir` が、解決したパスである。

 -   **送信の停止を解く。** `/chief` の問いで `停止を解く: <workspace の名前>` と答えるか、ほかの方法がないときだけ、その workspace のディレクトリの `stop.json` を人が消す。停止の記録を読めない workspace（表に、停止の記録を読めないと示される）には `停止を解く` は示されないので、`stop.json` の中身と pane を人が確かめてから消す。停止の理由と、そのときの画面は、状況の表に示される。消す前に、止まった送信がその pane で何をしたかを確かめる。
--   **見送りの記録。** ラウンドは見送りの記録を読むだけで、作りも消しもしない。記録が消えるのは、人が選んだリセットを送る直前、chief がその pane へコマンドかリセットを打てたとき、`start` の片付けのときである。決まった形に合わない記録は項目にならず、ラウンドの `problems` にパスと理由が示される。その記録は、人が手で消してよい。
--   **片付け。** `start` が、終わったセッションのラウンド・観測・通知の記録、始めと終わりの両方を書いた古い送信の記録、14 日より古い PR の詳細の控えと報告の控えと見送りの記録（読めない見送りの記録は、ファイルの更新が 14 日より古いもの）を消す。終わりの記録のない送信の記録は消さず、送信の停止に変える。自動送信の記録は消さない（消すと、同じマージで 2 回送りうる）。ほかのセッションの履歴の記録も消す（3.1 の「要確認から戻す」）。
--   **作るときと消すときの記録。** `create` は、作る前に、同じパスの workspace ごとの記録（前の使い方の送信の停止など）をすべて消してから、作ったことの記録を書く。`remove` は、worktree を消し終えた後に、その workspace ごとの記録をすべて消す。作ったことの記録は最後に消す。
+-   **見送りの記録。** ラウンドは見送りの記録を読むだけで、作りも消しもしない。決まった形に合わない記録は項目にならず、ラウンドの `problems` にパスと理由が示される。その記録は、人が手で消してよい。
 -   **すべてを消す。** chief が動いていないときに、ディレクトリごと消してよい。消した後は、chief を起動する前のマージを自動では送らず、提案として示す。作ったことの記録も消えるので、chief が作った workspace は削除の提案に出なくなる。

 ## 8. 既知の制限
@@ -272,7 +239,7 @@ chief のセッションが道具を通さずに入力を送らず、タスク
 -   **打った直後のラウンドは 1 つ前のコマンドを示しうる。** 人が Claude Code の pane でコマンドを打った直後で、Claude Code がまだその行を記録に 1 バイトも書いていないラウンドは、1 つ前のコマンドを示す。書きかけの行は 200 ms の間隔で 3 回まで読み直し、それでも終わらなければ何も示さない（`partial_line`）が、書き始める前は見分けられない。
 -   **`/clear` の直後は古いセッションのコマンドを示しうる。** ラウンドの初めに読んだ Herdr のセッションの識別が、`/clear` の直後でまだ古いと、古いセッションの記録のコマンドを示しうる。
 -   **人が引数なしで打ったコマンドは示されない。** 人が引数なしで打ったコマンド（`/runpr` など）は、記録の形が人の入力の読み取りの受け付ける形と違い、「そのほかの人の入力」と読まれる。その前にタスクのコマンドがあればそれに `＋指示` を添えて示し、なければ何も示さない（`not_task_command`）。たとえば `/runplan` の後に引数なしの `/runpr` を打つと、`/runpr` が走っている間も、内容の列には前の `/runplan` のコマンドが `＋指示` を添えて示される。
--   **起動し直すと長く走る OpenCode のコマンドは示されない。** `chief start` は、始めと終わりの両方を書いた 25 分より古い送信の記録を消す（7 章の「片付け」）。そのため、長く走る OpenCode のコマンドは、chief を起動し直した後に示されなくなる。
+-   **起動し直すと長く走る OpenCode のコマンドは示されない。** `chief start` は、始めと終わりの両方を書いた 25 分より古い送信の記録を消す（[chief_architecture.md](../chief_architecture.md) の 4.5）。そのため、長く走る OpenCode のコマンドは、chief を起動し直した後に示されなくなる。
 -   **エージェントのいない workspace は提案されない。** chief はエージェントの作業ディレクトリから workspace を見つけるので、エージェントがすべて終了した workspace は表に出ず、削除も提案されない。人が端末で片付ける。
 -   **作ったことの記録だけが残った worktree は表に出ない。** 作成が workspace を作る前に止まったときや、削除が workspace を閉じた後に止まったときは、worktree と記録が残っても表に出ない。結果に示された残ったものを、人が端末で片付ける。
 -   **作るか消すと、ほかの workspace の `make coverage-compare` が止まる。** 比較は、走っている間の `git worktree list` の変化を見つけると止まる（`internal/coveragecompare/userstate.go`）。比較の間は作成と削除を避けるか、止まったら走らせ直す。
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-19 の記録

Diff record (AC-19):

- Compared paths: `docs/dev/conventions/package_reference.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `7798f4b4e610482ef66ee355846a3ea080236676`

Path diff:

```text
diff --git a/docs/dev/conventions/package_reference.md b/docs/dev/conventions/package_reference.md
index bfc9484edd62b4926fe48312a69bbe214f8af3ca..898a656da8bfab609db9f3e3db942d47050f3802 100644
--- a/docs/dev/conventions/package_reference.md
+++ b/docs/dev/conventions/package_reference.md
@@ -113,14 +113,14 @@ add a kind here or to widen one of these definitions.

 | Package | Responsibility |
 |---|---|
-| `cmd/chief` | Entry point of the `chief` developer tool: `main` reads the environment once (`chief.ReadEnv`), parses `--protocol`, the subcommand (`start`, `snapshot`, `tick`, `wait`, `send`, `relay`, `reset`, `detail`, `release`, `targets`, `create`, `remove` today), and its flags (`targets` and `create` take `--issue` as text, which `internal/chief` checks), and prints one JSON value with the exit code of the 0088 design (3.1.1) and the 0092 design (3.1). |
+| `cmd/chief` | Entry point of the `chief` developer tool: `main` reads the environment once (`chief.ReadEnv`), parses `--protocol`, the subcommand (`start`, `snapshot`, `tick`, `wait`, `send`, `relay`, `reset`, `detail`, `release`, `targets`, `create`, `remove` today), and its flags (`targets` and `create` take `--issue` as text, which `internal/chief` checks), and prints one JSON value with the exit code listed in chapter 3 of [chief_architecture.md](../chief_architecture.md). |
 | `cmd/codexreview` | Entry point of the `codexreview` developer tool, which runs Codex CLI's local review when the CodeRabbit CLI fails: `main` parses `--protocol` and `--base`, runs `codexreview.Run`, and prints its `Result` as JSON. |
 | `cmd/coveragecompare` | Entry point of the coverage comparison tool behind `make coverage-compare` and `make ext-coverage-compare`: it parses `go --base <rev>` or `ext --base <rev>` and runs `coveragecompare.Run`. |
 | `cmd/fixprguard` | Entry point of the `fixprguard` guard tool behind `/fixpr` and `/runpr`: it parses the subcommands and orders the `internal/fixprguard` calls. Usage: [fixprguard guide](../guides/fixprguard_guide.md). |
 | `cmd/mergepr` | Entry point of the `/mergepr` developer tool: it prepares a PR for message drafting, builds and checks the block of editorial correction lines the squash body carries (`paragraph`, `check-body`), then merges and cleans up after the user approves the message. Usage: [mergepr guide](../guides/mergepr_guide.md). |
 | `cmd/numreserve` | Entry point of the `numreserve` developer tool: `main` takes one word (`task` or `adr`), runs `numreserve.Tool.Reserve`, and prints the number, or one stop line with its reason word, with the exit code of the design (3.1). The design: [0100 design](../../tasks/0100_number_reservation/02_architecture.md). |
-| `internal/chief` | The decisions and side effects of the `chief` developer tool: the screen readers, which read an agent's pane screen and Claude Code's transcript into typed results (`Screen`, `Question`, `InputLine`, `RunprNext`, `RunprReport`, `ModelSwitch`, `HumanInput`, `SessionReport`); the sanitizing and read caps for text shown to the human (`Sanitize`, `CapHead`); the one path that runs herdr, git, and gh (`Commands`); and the readers of Herdr (`Herdr`), of a task's documents and PR markers (`Tasks`, `ParsePlan`), and of the routing table (`ParseRouting`, `ReadRouting`); the state directory and the lock (`Store`, `OpenStore`, `Owner`), the git and GitHub reader of a worktree (`GitReader`), the classification of each task workspace and its pending items (`TakeSnapshot`, `Snapshot`, `Row`, `PendingItem`), with each row's urgency, turn, and position (`Urgency`, `Turn`, `Position`, `PositionDocument`, `Progress`) and the command its working agents run (`RunningCommand`, `RunningShown`, `RunningUnknown`); the place a task is made in and the workspaces at a path (`TaskSpace`, `NewTaskSpace`), the read of an issue from GitHub (`GitReader.ReadIssue`, `Issue`, `IssueStatus`), the registered worktrees and the ancestry of a commit (`GitReader.Worktrees`, `GitReader.IsAncestor`, `WorktreeEntry`), the task that answers an issue (`Tasks.IssueIndex`, `IssueIndex`, `Tasks.ReadDir`); the removal proposal of a workspace chief made whose task is done, with the checks before a removal (`RemovalCandidate`, `RemovalProposal`, `RemovalCheck`, `CheckResult`, `PendingRemoval`, `Row.RemovalRow`); the only writer of input to a pane (`PaneWriter`), the creation of a task's worktree and workspace with its `/mkreq` and the workspaces `/mkreq` can go to (`Create`, `CreateOutcome`, `CreateRefusal`, `CreateStage`, `CreateResult`, `Made`, `MadeKind`, `NotReady`, `Targets`, `TargetsOutcome`, `TargetWorkspace`), the only maker of a workspace (`WorkspaceMaker`, `NewWorkspaceMaker`), the removal of a finished task's workspace and worktree (`Remove`, `RemoveOutcome`, `RemoveRefusal`, `RemoveStage`, `RemoveResult`) by the only closer of a workspace and remover of a worktree (`WorkspaceCloser`), the exclusion between a creation and the automatic sends (`Store.LockCreation`, `CreationLock`, `ErrCreationBusy`), the send of a command with its checks and the reset after `/runpr` (`Send`, `ParseSendRequest`, `ActionResult`), the send of a reset skipped for want of a report and offered to the human (`SendOfferedReset`, `ParseOfferedResetRequest`, `ResetItem`), the send records, stops, and per-workspace exclusion (`WorkspaceRecords`, `HeldWorkspace`, `Release`), the record of a worktree chief made (`CreatedRecord`, `HeldWorkspace.KeepCreated`, `WorkspaceRecords.Created`, `HeldWorkspace.ClearRecords`), a PR's detail (`Detail`, `CollectDetail`, `PRDetail`), the wait for a change of the task agents (`Wait`, `WaitBaseline`), and the round of the wait with its observations and the automatic send after a merge (`Tick`, `Store.BeginRound`, `Observation`, `AutoSendResult`). The design: [0088 design](../../tasks/0088_chief_of_staff/02_architecture.md); the urgency, position, and running command: [0093 design](../../tasks/0093_chief_status_sheet/02_architecture.md). |
-| `internal/chief/paneinput` | The only types that describe an input `chief` may send to an agent's pane, each built only by a validating constructor: `Command` (`NewMkreq`, `NewMkarch`, `NewMkplan`, `NewRunplan`, `NewRunpr`; its kind and argument read back by `Command.Parts`), `CommandKind` (named by `CommandKind.Name` and read from a name by `ParseCommandKind`), `Prompt` (`ClearSession`, `NewSession`, `SwitchModel`, `TypeCommand`), `Route`, `RouteKey`, `TaskID`, `IssueNumber`. The design: [0088 design §3.1.3](../../tasks/0088_chief_of_staff/02_architecture.md). |
+| `internal/chief` | The decisions and side effects of the `chief` developer tool: the screen readers, which read an agent's pane screen and Claude Code's transcript into typed results (`Screen`, `Question`, `InputLine`, `RunprNext`, `RunprReport`, `ModelSwitch`, `HumanInput`, `SessionReport`); the sanitizing and read caps for text shown to the human (`Sanitize`, `CapHead`); the one path that runs herdr, git, and gh (`Commands`); and the readers of Herdr (`Herdr`), of a task's documents and PR markers (`Tasks`, `ParsePlan`), and of the routing table (`ParseRouting`, `ReadRouting`); the state directory and the lock (`Store`, `OpenStore`, `Owner`), the git and GitHub reader of a worktree (`GitReader`), the classification of each task workspace and its pending items (`TakeSnapshot`, `Snapshot`, `Row`, `PendingItem`), with each row's urgency, turn, and position (`Urgency`, `Turn`, `Position`, `PositionDocument`, `Progress`) and the command its working agents run (`RunningCommand`, `RunningShown`, `RunningUnknown`); the place a task is made in and the workspaces at a path (`TaskSpace`, `NewTaskSpace`), the read of an issue from GitHub (`GitReader.ReadIssue`, `Issue`, `IssueStatus`), the registered worktrees and the ancestry of a commit (`GitReader.Worktrees`, `GitReader.IsAncestor`, `WorktreeEntry`), the task that answers an issue (`Tasks.IssueIndex`, `IssueIndex`, `Tasks.ReadDir`); the removal proposal of a workspace chief made whose task is done, with the checks before a removal (`RemovalCandidate`, `RemovalProposal`, `RemovalCheck`, `CheckResult`, `PendingRemoval`, `Row.RemovalRow`); the only writer of input to a pane (`PaneWriter`), the creation of a task's worktree and workspace with its `/mkreq` and the workspaces `/mkreq` can go to (`Create`, `CreateOutcome`, `CreateRefusal`, `CreateStage`, `CreateResult`, `Made`, `MadeKind`, `NotReady`, `Targets`, `TargetsOutcome`, `TargetWorkspace`), the only maker of a workspace (`WorkspaceMaker`, `NewWorkspaceMaker`), the removal of a finished task's workspace and worktree (`Remove`, `RemoveOutcome`, `RemoveRefusal`, `RemoveStage`, `RemoveResult`) by the only closer of a workspace and remover of a worktree (`WorkspaceCloser`), the exclusion between a creation and the automatic sends (`Store.LockCreation`, `CreationLock`, `ErrCreationBusy`), the send of a command with its checks and the reset after `/runpr` (`Send`, `ParseSendRequest`, `ActionResult`), the send of a reset skipped for want of a report and offered to the human (`SendOfferedReset`, `ParseOfferedResetRequest`, `ResetItem`), the send records, stops, and per-workspace exclusion (`WorkspaceRecords`, `HeldWorkspace`, `Release`), the record of a worktree chief made (`CreatedRecord`, `HeldWorkspace.KeepCreated`, `WorkspaceRecords.Created`, `HeldWorkspace.ClearRecords`), a PR's detail (`Detail`, `CollectDetail`, `PRDetail`), the wait for a change of the task agents (`Wait`, `WaitBaseline`), and the round of the wait with its observations and the automatic send after a merge (`Tick`, `Store.BeginRound`, `Observation`, `AutoSendResult`). The design: chapter 2 of [chief_architecture.md](../chief_architecture.md); the urgency, position, and running command: chapter 7 of it. |
+| `internal/chief/paneinput` | The only types that describe an input `chief` may send to an agent's pane, each built only by a validating constructor: `Command` (`NewMkreq`, `NewMkarch`, `NewMkplan`, `NewRunplan`, `NewRunpr`; its kind and argument read back by `Command.Parts`), `CommandKind` (named by `CommandKind.Name` and read from a name by `ParseCommandKind`), `Prompt` (`ClearSession`, `NewSession`, `SwitchModel`, `TypeCommand`), `Route`, `RouteKey`, `TaskID`, `IssueNumber`. The design: chapter 2 of [chief_architecture.md](../chief_architecture.md). |
 | `internal/codexreview` | The checks behind the `codexreview` tool: `Run`, `Result`, `Kind`, `Protocol`. |
 | `internal/coveragecompare` | The coverage comparison tool's core: `Run`, `Write`, `ExitCode`, `StopError`, and the input parsers. |
 | `internal/fixprguard` | The deterministic checks for `/fixpr` and `/runpr`: `GitHub`, `NewGitHub`, `ParseReviewers`, `CheckState`, `Worktree.Baseline`. Usage: [fixprguard guide](../guides/fixprguard_guide.md). |
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-19a の記録

Diff record (AC-19a):

- Compared paths: `CLAUDE.md` `.claude/commands/chief.md`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `7798f4b4e610482ef66ee355846a3ea080236676`

Path diff:

```text
diff --git a/.claude/commands/chief.md b/.claude/commands/chief.md
index bd8b3a2da77f501d1cfe94f265fbcd134c85d95e..5daab9f24909d3883bd8a6ff4fc84304638e5132 100644
--- a/.claude/commands/chief.md
+++ b/.claude/commands/chief.md
@@ -15,7 +15,9 @@ worktree and workspace are made and a finished one's removed. You
 never decide any of these yourself, you never type into another pane, and
 you never make or remove a worktree or a workspace yourself.
 How the tool works, what it guarantees, and how to install it are in the
-guide, [chief_guide.md](../../docs/dev/guides/chief_guide.md) (Japanese).
+guide, [chief_guide.md](../../docs/dev/guides/chief_guide.md) (Japanese). The
+tool's contract and records, to read beside this definition, are in
+[chief_architecture.md](../../docs/dev/chief_architecture.md) (Japanese).

 Talk to the human in Japanese (`CLAUDE.md`, "Language"). Show the tool's
 agent values as their names: `claude_code` as Claude Code, `opencode` as
@@ -82,7 +84,7 @@ Two calls differ:
 |---|---|---|
 | `0` | Done (a decision not to send is done too) | Read the JSON |
 | `1` | The tool failed (`outcome` `failed`, or `timeout` from `wait`) | Show `message`, and say that this call's result could not be read (for a round, that its table was not updated; for `wait`, step 5 says what to do). Do not judge from `message` which workspaces it concerns |
-| `2` | Usage error | A mistake in this definition or in your arguments: stop and report the call and its standard error |
+| `2` | Usage error, or the tool and this definition are from different commits | A mistake in this definition or in your arguments: stop and report the call and its standard error. A tool that does not know this call also exits `2`, so in every case also tell the human that the tool and this definition may be from different commits (the tool may be older), and that the fix is to stop `/chief`, update the chief worktree to `main`, reinstall the tool, and start `/chief` again, in the order the guide's chapter 2 gives |
 | `3` | Refused, nothing done (`outcome` `refused`, `reason`) | See below |

 The refusals (`reason`):
diff --git a/CLAUDE.md b/CLAUDE.md
index abd61bb57d0214ba9d7d4f9a2fdd7d4dd362d4e8..e12e73f86b7f572d7205c229eeaee2dda73f7ab9 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -21,7 +21,7 @@ This file provides guidance to Claude Code (claude.ai/code) when working with co
 - [OpenAI Output Limit](docs/dev/guides/openai_output_limit.md) - When and how to re-measure the basis of the OpenAI adapter's default output limit (`max_output_tokens`)
 - [mergepr Guide](docs/dev/guides/mergepr_guide.md) - Setup, usage, guarantees, and troubleshooting for the `/mergepr` tool
 - [fixprguard Guide](docs/dev/guides/fixprguard_guide.md) - Setup, subcommands, guarantees, scope rules, and recovery for the `fixprguard` guard tool for `/fixpr` and `/runpr`
-- [chief Guide](docs/dev/guides/chief_guide.md) - Setup, usage, subcommands, guarantees, the settings' herdr denials, the state directory, and known limits of the `chief` tool that `/chief` drives
+- [chief Guide](docs/dev/guides/chief_guide.md) - Setup, usage, guarantees, the settings' herdr denials, and known limits of the `chief` tool that `/chief` drives. The subcommands, flags, output, state directory, and records are specified in [chief Architecture](docs/dev/chief_architecture.md)
 - [tools Guide](docs/dev/guides/tools_guide.md) - Purpose and usage of the developer scripts under `tools/`: the low-cost `/chief` launcher and the scripts that estimate `/chief`'s token cost
 - [numreserve Guide](docs/dev/guides/numreserve_guide.md) - Installation, invocation, stop reasons and what to do next, the record ruleset and removing a wrong record tag, and the cloud-session limits of the `numreserve` tool that decides task and ADR numbers
 - [Developer Tools](docs/dev/conventions/dev_tools.md) - Rules for adding or changing a developer tool: write it in Go with tests, install installed tools from `main` (the user installs them; agents never install or build them), and keep its arguments, output, and exit codes as a contract
```

File list:

```text
.claude/commands/chief.md
CLAUDE.md
cmd/chief/docs_test.go
docs/dev/adr/0046-chief-of-staff.md
docs/dev/chief_architecture.md
docs/dev/conventions/dev_tools.md
docs/dev/conventions/package_reference.md
docs/dev/guides/chief_guide.md
docs/tasks/0117_chief_architecture_doc/03_implementation_plan.md
docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh
```

### AC-22 の記録

`diff` verification by a script (ステップ 1-5):

- Script: `docs/tasks/0117_chief_architecture_doc/_checks/check-ac22.sh`
- Merge-base: `c2df59f3daa72de99354a2992f92746517dc8ad1`
- Compared commit: `5e00e64e5971c037706044625dbec7d54159e634`

壊して確かめたこと（各確かめが少なくとも 1 つの壊し方で失敗すること）:

- `cmd/chief/docs_test.go` のほかに変えたテストのファイルを 1 つ置くと、1 つ目の確かめが失敗した（`FAIL: a test file other than cmd/chief/docs_test.go changed:` と、そのファイルを挙げた）。
- `cmd/chief/docs_test.go` のほかにテストのファイルを 1 つ消すと、1 つ目の確かめが失敗した（同じ理由を挙げた）。
- 消したテストの関数に `TestSettingsAllowsTool` を足すと、2 つ目の確かめが失敗した（消えた関数として 2 つを挙げた）。
- 足したテストの関数に `TestOutputFieldsMatchTool` を足すと、3 つ目の確かめが失敗した（足した関数として 3 つを挙げた）。

PR のブランチで走らせた出力:

```text
changed test files:
cmd/chief/docs_test.go
removed test functions:
TestGuideValuesMatchTool
added test functions:
TestArchitectureValuesMatchTool
TestGuideProtocolMatchesTool
OK
```

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
