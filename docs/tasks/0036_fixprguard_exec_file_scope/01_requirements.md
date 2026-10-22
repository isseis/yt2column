# 要件定義書：fixprguard の範囲の規則で、下の階層の指示ファイルとビルドの入力を範囲外にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ガードツール `fixprguard` の範囲の規則を改めるための要件を定義する（[#425](https://github.com/isseis/yt2column/issues/425)）。改めた規則では、下の階層に置かれた、エージェントが読む指示ファイルとビルドにリンクされる入力が範囲外になる。あわせて、規則 2・3 が範囲に入れるファイルを、決まった種類だけに限る。

**背景:**

`fixprguard inspect` は、`/fixpr` の修正のフェーズが変えたファイルを、処理したスレッドの `path` から決まる範囲と照らす。範囲外の変更があれば、終了コード `3` で利用者の確認に回す（`docs/dev/developer_guide/fixprguard_guide.md:156`）。範囲の規則は、タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）で決まった（0030 `02_architecture.md` 3.6）。次の 3 つのどれかに当たる変更が範囲に入る（`internal/fixprguard/scope.go:16-28`）。

1.  `path` そのもの。
2.  `path` と同じディレクトリのファイル（サブディレクトリの中は含まない）。
3.  `path` が `docs/tasks/NNNN_<name>/` の中にあれば、そのディレクトリの下のすべてのファイル。

ただし、コマンドの実行に関わるファイルは、`path` そのものでなければ範囲外とする。名指しされた想定外の著者のスレッドでは、`path` そのものでも範囲外とする（0030 `02_architecture.md` 3.6 の規則 4。`internal/fixprguard/scope.go:170-172`）。

コマンドの実行に関わるファイルの定義は、名前の拒否リストである（`internal/fixprguard/scope.go:39-54`）。規則 2・3 は、このリストに当たらないファイルをすべて通す（`internal/fixprguard/scope.go:181-189`）。ところが、リストには次の制限がある。

-   `CLAUDE.md`・`AGENTS.md` と、名前が `.` で始まるファイルは、リポジトリの最上位にあるときだけ当たる（`internal/fixprguard/scope.go:48`・`:119`）。
-   `.claude/` は、最上位のディレクトリだけが当たる（`internal/fixprguard/scope.go:45`・`:114-118`）。
-   Go のビルドに使われる `.go` 以外のファイル（`.syso`・`.s`・`.c` など）は、リストにない。

そのため、Claude Code が読む指示ファイルやビルドにリンクされる入力が下の階層にあると、範囲内と判定される。issue #425 は、`inScope` と `isExecutionRelated` を直接呼んで、次の判定を確かめた（一部を抜粋した）。

| 変更したパス | スレッドの `path` | 名指し | 今の判定 |
|---|---|---|---|
| `internal/writer/CLAUDE.md` | `internal/writer/writer.go` | なし | 範囲内 |
| `CLAUDE.local.md` | `README.md` | なし | 範囲内 |
| `docs/tasks/0030_x/.claude/commands/evil.md` | `docs/tasks/0030_x/01_requirements.md` | なし | 範囲内 |
| `docs/tasks/0030_x/CLAUDE.md` | `docs/tasks/0030_x/01_requirements.md` | あり | 範囲内 |
| `internal/writer/foo.syso` | `internal/writer/writer.go` | なし | 範囲内 |

issue #425 によれば、影響は次のとおりである。Claude Code はサブディレクトリの `CLAUDE.md` を読むので、そこに書かれた指示は commit・push されて残る。`.syso` はパッケージのビルドに無条件でリンクされる。`make test` は `-race` を付けて実行するので外部リンクになり（`Makefile:84`）、ELF のコンストラクタがテストバイナリの起動時に実行されうる。名指しのスレッドでは `.go` のファイルを範囲外にする規則（`internal/fixprguard/scope.go:182-184`）を、これで迂回できる。下の階層の `.gitignore` は、以後の変更を git から隠せる（0030 `02_architecture.md` 5.3「無視されるファイル」）。

対応 issue: [#425](https://github.com/isseis/yt2column/issues/425)（関連: [#395](https://github.com/isseis/yt2column/issues/395)、タスク `0030_fixpr_untrusted_threads`。[#424](https://github.com/isseis/yt2column/issues/424)、タスク `0034_fixprguard_ignored_files`）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。どの変更が範囲に入り、どの変更が範囲外になるか、利用者に何が示されるか、ガイドに何が書かれるかである。次のことは設計（`02_architecture.md`）で決める。

-   判定の順序と、定義を持つデータの形
-   ガイドへの定義の載せ方と、文書のガードがそれを確かめる手段
-   `protocol` の番号の扱い
-   テストの手段

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #425 で決めたこと:**

-   **完了条件:** issue の表（本書 3 章の AC-12 に全行を載せる）の変更したパスが、すべて範囲外になる。例外は、変更したパスがスレッドの `path` そのもので、かつそのスレッドが信頼する著者のスレッドである場合だけである。`make test`・`make lint`・`doc-guard` が通る。
    -   理由: 表の各行は、規則 4 が範囲外にしようとしたファイルの種類に当たる。
-   **指示ファイルと git の設定を、どの階層でも実行関連とする:** `claude.md`・`agents.md`・`claude.local.md`・`.claude/`（ディレクトリの名前として）・`.gitattributes`・`.gitignore` を、どの階層でもコマンドの実行に関わるファイルとする。
    -   理由: Claude Code はサブディレクトリの指示ファイルを読む。下の階層の `.gitignore` は、以後の変更を隠せる。
-   **定義を変えたら、一覧・ガイド・ガードを合わせる:** `ExecutionFilePatterns`（`internal/fixprguard/scope.go:60-75`）、`fixprguard_guide.md` 6 章の表、`cmd/fixprguard/docs_test.go` のガードを更新する。
    -   理由: ガイドは定義と同じ一覧を載せると約束している（`docs/dev/developer_guide/fixprguard_guide.md:219`）。
-   **表の各行をテストに入れる:** 壊して失敗を確かめる（`CLAUDE.md`「Testing Strategy」）。

**要件定義の際に決めたこと（2026-10-22）:**

-   **拒否リストを広げ、規則 2・3 には許可リストも加える:** issue は「拒否リストを拡張子の許可リストに反転する案を検討する」とした。本書は、拒否リストを広げたうえで（F-001）、規則 2・3 が範囲に入れるファイルを許可リストに限る（F-002）。
    -   理由: 拒否リストは、原理的に網羅できない（issue #425）。許可リストに当たらないファイルは、利用者の確認に回る。確認が増えることは、0030 で受け入れ済みである（0030 `01_requirements.md` 4.4）。
    -   拒否リストを残す理由: 規則 1（`path` そのもの）と名指しのスレッドの扱いは、拒否リストで決まる。許可リストは `.md` を通すので、`CLAUDE.md` のような `.md` の指示ファイルは拒否リストで止める必要もある。
-   **許可リストは issue の例だけにする:** 規則 2・3 が範囲に入れるのは、`.md`・`.txt` のファイル、`testdata` のディレクトリの下のファイル、既存の `.go` の規則（`internal/fixprguard/scope.go:182-184`）に当たる `.go` のファイルだけである。`.tmpl`・`.ts`・`.json`・`.sh`・`.py` などは、規則 2・3 では範囲に入らない。
    -   理由: 範囲を最小にし、ほかの種類は利用者の確認に回す。プロンプトのテンプレート（`.tmpl`）を確認なしで通す案は採らない。
-   **Go のビルドの入力は、`.go` の規則ではなく実行関連とする:** issue の「やること」は、`.syso` などを「`.go` と同じ規則」にするとした。本書は、これらをコマンドの実行に関わるファイルとして扱う。範囲に入るのは、信頼する著者のスレッドの `path` そのものであるときだけである。
    -   理由: `.go` の規則では、`foo.syso`（スレッドの `path` は `writer.go`、名指しなし）が範囲内のまま残り、issue の完了条件と食い違う。
    -   このリポジトリには、この種類のファイルは 1 つもない。そのため、正当な修正で確認が増えることはない。
-   **Go のビルドの入力の拡張子は、Go が扱うもの全部にする:** issue が挙げた 7 種類（`.syso`・`.s`・`.c`・`.h`・`.cc`・`.m`・`.S`）に、Go がパッケージのビルドで扱うほかの拡張子を加える（F-001 AC-03）。
    -   理由: issue の 7 種類だけでは、`.cpp`・`.hpp` などで同じ迂回ができる。拡張子の一覧は `go help filetype` と、`go/build` の `Package` の `FFiles`（Fortran）による。
-   **`.` で始まる要素を、どの階層でも実行関連とする:** issue が挙げた `.claude/`・`.gitattributes`・`.gitignore` に限らない。パスのどの要素（ファイルの名前またはディレクトリの名前）でも、`.` で始まれば実行関連とする。
    -   理由: ツールは、下の階層の `.` で始まるファイルも読む。たとえば direnv は、下のディレクトリの `.envrc` を読む。`.npmrc` も下の階層に置ける。最上位で `.` で始まるファイルを実行関連とする今の規則（`internal/fixprguard/scope.go:119`）を、どの階層にも広げる。
-   **`inspect` の出力の形は変えない:** 範囲外になった理由（拒否リストか、許可リストに当たらないか）は出力しない。`outOfScope` の要素は今の項目のままである（`docs/dev/developer_guide/fixprguard_guide.md:156`）。
    -   理由: 利用者は、一覧を見て続けるかを判断できる。出力の契約を変えずに済む。
-   **コマンドが読むよう指示する文書は、実行関連にしない:** `.claude/commands/` の手順書は、`docs/dev/developer_guide/` の文書などを読んで従うよう指示している。これらの文書は実行関連とせず、残余のリスクとして記録する（4.2）。
    -   理由: これらの文書は、正当な修正でもよく変わる。変更は PR の diff として人が確かめる。

以下の決定は、要件定義書の作成の中で安全側の既定として選んだ。PR のレビューで確かめてほしい。

-   **許可リストの照合は、大文字と小文字を区別する:** `README.MD`・`TestData/` は許可リストに当たらない（AC-09）。
    -   理由: 許可リストは範囲を広げる側の規則なので、狭いほうが安全である。リポジトリの既存のファイルは、すべて小文字の `.md`・`.txt`・`testdata` を使う。拒否リストは今までどおり ASCII の大文字小文字を同一視して照合する（`internal/fixprguard/scope.go:37-38`）。
-   **`.` で始まるディレクトリの中は、すべて実行関連とする:** `.` で始まる要素の規則は、ディレクトリの名前にも当たる（AC-02）。
    -   理由: `.claude/`・`.github/` のように、`.` で始まるディレクトリはツールの設定を置く場所として使われる。issue が挙げた `.claude/` もこれに含まれる。
-   **0030 の設計書と要件定義書に、規則が変わったことの注記を加える:** 0030 `02_architecture.md` 3.6 と 0030 `01_requirements.md` F-004 に、本タスクで規則が変わったことと、本書への参照を加える（AC-17）。
    -   理由: ガイドは、設計の背景として 0030 の設計書を挙げる（`docs/dev/developer_guide/fixprguard_guide.md:5`）。古い規則のまま残すと、読み手が今の規則を誤解する。注記は決定を変えないので、0030 の文書の状態は `approved` のままとする（要件定義のプロセスのガイド「Editing an approved document」の Editorial correction）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   どの階層に置かれた指示ファイル（`CLAUDE.md` など）・`.` で始まるファイル・Go のビルドの入力も、規則 2・3 では範囲に入らないようにする。
-   名指しされた想定外の著者のスレッドで、`.go` のファイルを範囲外にする規則を、Go のビルドの入力で迂回できないようにする。
-   規則 2・3 が範囲に入れるファイルを決まった種類に限り、拒否リストに載っていない新しい種類のファイルが、確認なしに commit されないようにする。

### 2.2. スコープ (In Scope)

-   コマンドの実行に関わるファイルの定義を広げる（F-001）。
-   規則 2・3 に許可リストを加える（F-002）。
-   issue の表の各行を範囲外にする（F-003）。
-   `inspect` の出力と終了コードを、今までの形のまま保つ（F-004）。
-   ガイド（日英）・文書のガード・0030 の文書を新しい規則に合わせる（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   **git が無視するファイルの検出:** 無視されるファイルへの書き込みは、変更の集合に入らない。これはタスク 0034（[#424](https://github.com/isseis/yt2column/issues/424)）が扱う。
-   **コマンドが読むよう指示する文書の保護:** `docs/dev/developer_guide/*.md` などは実行関連にしない（1 章の決定）。残余のリスクとして 4.2 に記録する。
-   **規則 1（`path` そのもの）の変更:** 実行関連でないファイルは、今までどおり、`path` そのものなら範囲に入る。種類は問わない。
-   **範囲の規則以外の、ガードツールの振る舞い:** 著者の判定、基準値、`finish`・`push` の確認は変えない。
-   **使っていないエージェントの指示ファイル:** `GEMINI.md` などは対象にしない。このリポジトリで使うのは Claude Code（`CLAUDE.md`）と、`AGENTS.md` を読む自動レビュアーである。名前が `.` で始まるもの（`.cursorrules` など）は、F-001 AC-02 に含まれる。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下で「判定」とは、変更したパス・変更の種類・スレッドの `path`・名指しの有無から、その変更が範囲に入るかを決めることをいう。判定の結果は、`fixprguard inspect` の `outOfScope` に現れる（範囲外ならその要素になる）。各 AC の例は、変更が通常のファイルの追加または変更である場合を示す。

#### F-001: コマンドの実行に関わるファイルの定義を広げる

コマンドの実行に関わるファイルの定義に、どの階層でも当たる 3 種類を加える。指示ファイルの名前、`.` で始まる要素、Go のビルドの入力の拡張子である。拒否リストの照合は、今までどおり ASCII の大文字小文字を同一視して行う。

**Acceptance Criteria**:
- **AC-01**: 名前が `claude.md`・`agents.md`・`claude.local.md` のどれか（ASCII の大文字小文字を同一視する）のファイルは、どの階層にあってもコマンドの実行に関わるファイルとして判定される。例: `internal/writer/CLAUDE.md`・`internal/writer/AGENTS.md`・`CLAUDE.local.md`・`docs/tasks/0030_x/claude.local.md`。
- **AC-02**: パスの要素（ディレクトリの名前とファイルの名前）のどれかが `.` で始まるファイルは、どの階層にあってもコマンドの実行に関わるファイルとして判定される。`.claude/`・`.gitattributes`・`.gitignore` はこれに含まれる。例: `docs/tasks/0030_x/.claude/commands/evil.md`・`internal/writer/.gitattributes`・`internal/writer/.gitignore`・`internal/writer/.envrc`・`internal/.config/foo.md`。
- **AC-03**: 名前が次の拡張子のどれかで終わる（ASCII の大文字小文字を同一視する）ファイルは、どの階層にあってもコマンドの実行に関わるファイルとして判定される。`.c`・`.h`・`.cc`・`.cpp`・`.cxx`・`.hh`・`.hpp`・`.hxx`・`.m`・`.s`・`.S`・`.sx`・`.f`・`.F`・`.for`・`.f90`・`.swig`・`.swigcxx`・`.syso`。例: `internal/writer/foo.syso`・`internal/writer/foo_amd64.s`・`internal/writer/foo.c`・`internal/writer/foo.HPP`。
- **AC-04**: 今の定義（`internal/fixprguard/scope.go:39-54`）に当たるファイルは、引き続きコマンドの実行に関わるファイルとして判定される。例: `Makefile`・`internal/x/go.mod`・`build/rules.mk`・`.github/workflows/ci.yml`・`extension/scripts/has-extension-changes.sh`・`extension/package.json`。
- **AC-05**: コマンドの実行に関わるファイルの扱い（規則 4）は変えない。範囲に入るのは、変更したパスがスレッドの `path` そのもので、そのスレッドが名指しされたものでないときだけである。例: 変更したパスとスレッドの `path` がどちらも `internal/writer/foo.syso` のとき、信頼する著者のスレッドなら範囲内、名指しのスレッドなら範囲外になる。

#### F-002: 規則 2・3 を許可リストに限る

規則 2（同じディレクトリ）と規則 3（タスクのディレクトリの下）で範囲に入るのは、コマンドの実行に関わるファイルでなく、決まった種類に当たるファイルだけとする。規則 1（`path` そのもの）には許可リストを適用しない。

**Acceptance Criteria**:
- **AC-06**: 規則 2 または規則 3 で範囲に入るのは、コマンドの実行に関わるファイルでなく、次のどれかに当たるファイルだけである。
    -   (a) 名前が `.md` または `.txt` で終わる。
    -   (b) パスのディレクトリの要素のどれかが `testdata` である。
    -   (c) 今の `.go` の規則に当たる。`.go` のファイルで、スレッドの `path` が同じディレクトリの `.go` のファイルで、そのスレッドが名指しされたものでない（`internal/fixprguard/scope.go:182-184`）。

    `.go` のファイルには、(b) にかかわらず (c) の規則だけを適用する。
- **AC-07**: 規則 2・3 の位置にあっても、AC-06 に当たらない変更は範囲外になる。`inspect` は終了コード `3` で終わり、その変更を `outOfScope` に入れる。例（どれも名指しなし）:
    -   スレッドの `path` が `extension/src/a.ts` のときの `extension/src/a.test.ts`
    -   `prompts/system.tmpl` のときの `prompts/user.tmpl`
    -   `docs/tasks/0032_japanese_pr_title/01_requirements.md` のときの `docs/tasks/0032_japanese_pr_title/check_rules.sh`
    -   `internal/x/x.go` のときの `internal/x/data.json`
- **AC-08**: AC-06 に当たる変更は、今までどおり範囲に入る。例:
    -   スレッドの `path` が `docs/tasks/0030_x/01_requirements.md` のときの `docs/tasks/0030_x/02_architecture.md` と `docs/tasks/0030_x/sub/notes.txt`（名指しの有無を問わない）
    -   `internal/x/testdata/a.json` のときの `internal/x/testdata/b.json`（名指しの有無を問わない）
    -   `internal/x/x.go` のときの `internal/x/x_test.go`（名指しなし）
- **AC-09**: AC-06 (a)・(b) の照合は、大文字と小文字を区別する。例: スレッドの `path` が `docs/a.md` のとき、`docs/B.MD` は範囲外になる。`internal/x/x.go` のとき、`internal/x/TestData/a.json` は範囲外になる。
- **AC-10**: 規則 1 には許可リストを適用しない。コマンドの実行に関わるファイルでない変更は、スレッドの `path` そのものなら、種類にかかわらず範囲に入る。名指しの有無は問わない。例: 変更したパスとスレッドの `path` がどちらも `extension/src/a.ts` のとき。
- **AC-11**: 範囲の規則のうち、本書が変えないものは今までどおりである。パスの検査、ASCII でない文字を含むパスはバイト単位で `path` と一致するときだけ範囲に入ること、名前の変更を変更前の削除と変更後の追加として別々に判定すること、通常のファイル以外の変更を範囲外とすることである（`docs/dev/developer_guide/fixprguard_guide.md:208-217`）。

#### F-003: issue の表の各行を範囲外にする

**Acceptance Criteria**:
- **AC-12**: issue #425 の表の各行は、範囲外と判定される。変更の種類は通常のファイルの追加とする。

    | 変更したパス | スレッドの `path` | 名指し |
    |---|---|---|
    | `internal/writer/CLAUDE.md` | `internal/writer/writer.go` | なし |
    | `internal/writer/AGENTS.md` | `internal/writer/writer.go` | なし |
    | `CLAUDE.local.md` | `README.md` | なし |
    | `docs/tasks/0030_x/.claude/commands/evil.md` | `docs/tasks/0030_x/01_requirements.md` | なし |
    | `docs/tasks/0030_x/CLAUDE.md` | `docs/tasks/0030_x/01_requirements.md` | あり |
    | `internal/writer/foo.syso` | `internal/writer/writer.go` | なし |
    | `internal/writer/foo_amd64.s` | `internal/writer/writer.go` | なし |
    | `internal/writer/foo.c` | `internal/writer/writer.go` | なし |
    | `internal/writer/.gitattributes` | `internal/writer/writer.go` | なし |
    | `internal/writer/.gitignore` | `internal/writer/writer.go` | なし |

- **AC-13**: AC-12 の各行の変更したパスは、スレッドの `path` そのものであるときには、信頼する著者のスレッドなら範囲内、名指しのスレッドなら範囲外と判定される（AC-05 の適用）。

#### F-004: 出力と終了コードを保つ

**Acceptance Criteria**:
- **AC-14**: `inspect` の出力の項目と終了コードの意味は変わらない（`docs/dev/developer_guide/fixprguard_guide.md:73-78`・`:156`）。新しい規則で範囲外になった変更も、今までの範囲外の変更と同じ形で `outOfScope` に入る。範囲外になった理由は出力しない。
- **AC-15**: `/fixpr` と `/runpr` での範囲外の変更の扱い（利用者に一覧を示して続けるかを確かめる、続けないと答えたら変更を作業ツリーに残して止まる）は変わらない（0030 `01_requirements.md` AC-23・AC-25・AC-31）。

#### F-005: 文書とガード

**Acceptance Criteria**:
- **AC-16**: `fixprguard_guide.ja.md` と `fixprguard_guide.md` の 6 章が、新しい規則を説明する。許可リストの規則と、その照合が大文字と小文字を区別することが書かれる。コマンドの実行に関わるファイルの表には、F-001 で加えた定義と今の定義が、すべて載る。コマンドの実行に関わるファイルの説明は、build checks と CI に加えて、開発に使うエージェント（Claude Code と自動レビュアー）が指示として読むファイルを含むことを述べる。
- **AC-17**: 0030 `02_architecture.md` 3.6 と 0030 `01_requirements.md` F-004 に、範囲の規則が本タスクで変わったことと、本書への参照が書かれる。0030 の文書の状態は変えず、`Comments` に、決定を変えない注記であることを記録する。
- **AC-18**: コマンドの実行に関わるファイルの定義、または許可リストの項目のどれかが、ガイド（日英のどちらか）に載っていないと、`doc-guard` が実行するテストが失敗する。
- **AC-19**: ガイドの 7.2（保証しないこと）に、4.2 の残余のリスクのうち、ガイドにまだ書かれていないものが書かれる。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。判定はパスの文字列だけを見る関数で、変更のあるファイルは 1000 個まで（`docs/dev/developer_guide/fixprguard_guide.md:272`）なので、実行時間は問題にならない。

### 4.2. セキュリティ (Security)

-   本タスクは、条件付きのセキュリティの手引き（`docs/dev/security.md`）の適用条件に当たらない。`yt-dlp` を起動せず、秘密の値・キャッシュ・ネットワークを扱わず、信頼できないテキストをプロンプトや記事に埋め込まない。N/A とする。
-   範囲の規則は、0030 の守りの層 (3)（範囲外の変更の検出。0030 `01_requirements.md` 4.2）の一部である。本タスクは、この層が範囲内と見なすファイルを狭める。
-   範囲の規則は、今までどおりパスの文字列だけで決まる。ファイルシステムの状態には依存しない（`internal/fixprguard/scope.go:11-14`）。
-   **残余のリスク:** 次のことは、本タスクの後も範囲内のまま残る。
    -   **指示として読まれる `.md` の文書:** コマンドの手順書は、`docs/dev/developer_guide/` の文書などを読んで従うよう指示する。これらの `.md` は、規則 2・3 で範囲に入る。そこに書かれた指示は commit・push され、同じブランチの後のセッションが読む。変更は PR の diff に現れ、マージの前に人が確かめる。
    -   **`testdata` の下のファイル:** テストは `testdata` の下のファイルを読む。種類を問わず、規則 2・3 で範囲に入る。テストがそれを実行しない限り、実行されるコードにはならない。
    -   **規則 1 で範囲に入るファイル:** 実行関連でないファイルは、スレッドの `path` そのものなら種類を問わず範囲に入る（AC-10）。たとえば、スレッドの `path` が `.sh` のファイルなら、それを書き換えられる。
    -   **拒否リストに載らない、実行されるファイル:** 規則 1 で範囲に入る実行関連でないファイルが、別のツールに設定や指示として読まれる場合は防げない。拒否リストは網羅できない（1 章の決定）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。止まり方と失敗の扱いは変えない。

### 4.4. 互換性 (Compatibility)

-   **確認が増える:** 今まで規則 2・3 で確認なしに通った変更のうち、許可リストに当たらないものは、利用者の確認に回る。例: 同じディレクトリの `.ts` のテスト、プロンプトのテンプレート（`.tmpl`）、タスクのディレクトリのスクリプト（`.sh`・`.py`）、`testdata` の外の `.json`（AC-07）。確認が増えることは、0030 で受け入れ済みである（0030 `01_requirements.md` 4.4）。
-   **以前の規則の例外:** 0030 は「範囲は、スレッドの `path` と同じディレクトリまでとする」と決め、規則 2・3 で同じディレクトリとタスクのディレクトリのファイルを範囲に入れた（0030 `01_requirements.md` 1 章「要件定義の際に決めたこと」と F-004）。本書は、規則 2・3 で範囲に入るファイルを、許可リストに当たるものに限る。理由は 1 章「要件定義の際に決めたこと」の 1 つめのとおりで、拒否リストでは網羅できないからである。範囲を狭める変更なので、0030 が守ろうとしたこと（範囲外の変更を確認なしに commit しない）は弱まらない。
-   **インストール済みのツール:** 規則を変えたツールは、マージの後に `make install-fixprguard` で入れ直すまで使われない（`docs/dev/developer_guide/fixprguard_guide.md:46`）。古いツールが新しい文書から呼ばれたときの振る舞い（`protocol` の番号を上げるか）は、設計で決める（H-01）。
-   **タスク 0034 との関係:** 0034 の要件定義書（PR #429、`draft`）は、git が無視するファイルのうち、コマンドの実行に関わるファイルの定義に当たるものを基準値の対象にする。本タスクで定義が広がると、0034 の対象も広がる。たとえば、`.` で始まる要素の規則（AC-02）は、無視される `extension/node_modules/` の下の `.bin/` なども対象にする。後から実装するほうのタスクが、両者を合わせる（H-04）。

### 4.5. 保守性 (Maintainability)

-   コマンドの実行に関わるファイルの定義と許可リストは、`internal/fixprguard/scope.go` の 1 か所で持つ。ガイドはその一覧を載せ、文書のガードが一致を確かめる（AC-18）。`fixpr.md`・`runpr.md` は一覧を書かない（0030 `01_requirements.md` 4.5）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Testing Strategy」に従う。各テストは、対象の規則を壊して失敗することを確かめ、そのことを commit メッセージに書く。
-   範囲の規則は、パスの文字列だけで決める（`internal/fixprguard/scope.go:11-14`）。
-   標準ライブラリ以外のモジュールは加えない（`CLAUDE.md`「Dependencies」）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書は、範囲外の変更・信頼する著者のスレッド・想定外の著者のスレッド・名指し・ガードツールを、その定義で使う。次の用語は本書で使う意味を定める。

-   **コマンドの実行に関わるファイル:** build checks と CI が設定として読む、またはコンパイル・実行するファイルと、開発に使うエージェントが指示として読むファイル。定義は F-001 と `internal/fixprguard/scope.go` にある。規則 2・3 では範囲に入らず、規則 1 では信頼する著者のスレッドの `path` そのものであるときだけ範囲に入る。本書では、この定義を「拒否リスト」とも呼ぶ。
-   **許可リスト:** 規則 2・3 で範囲に入りうるファイルの種類（AC-06）。
-   **Go のビルドの入力:** Go がパッケージのビルドで、`.go` のファイルのほかに扱うファイル（AC-03 の拡張子）。
