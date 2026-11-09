# 実装計画書：README の表示の言語の段落を、ビューアにも触れる形に直す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語（ビューア・ポップアップ・表示の言語・ずれの記述）と、設計書の冒頭の用語（表示の言語の段落・実装の PR・内容のコミット・差分の確認・編集上の訂正・訂正の行・訂正の段落）を同じ意味で使う。

既存のファイルの行番号は、コミット `ccc55981fc6e8907e2b94e5bf7af046f2d622bc5`（`main`）で確かめた。設計書が行番号を確かめたコミット `960e3737` からこのコミットまでに、`README.md`・`README.en.md`・`docs/dev/adr/`・`extension/src/` を変えたコミットはない（`git log --oneline 960e3737..ccc55981 -- README.md README.en.md docs/dev/adr extension/src` の出力が空）。そのため、設計書の行番号はそのまま使える。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`README.md` の表示の言語の段落を、ポップアップとビューアの両方に触れる形に直し、`/mktrans` で `README.en.md` に写す（F-001・F-002）。同じ PR で、ADR-0029 のずれの記述を消す（F-003）。前後の関係を述べる AC は、`_checks/` の 1 回限りのプログラムで確かめる。

### 1.2. 実装の原則

-   **変える箇所は設計書 1.1 の 3 か所と、このタスクのディレクトリの下だけにする。** 拡張のコード、メッセージカタログ、Go のコードとテスト、ADR の索引は変えない（設計書 2.2）。
-   **`README.md` を先に直し、`/mktrans` は sub-step として実行する**（設計書 3.2）。単独の `/mktrans` は用語集を変えて別にコミットするので実行しない。
-   **段落の文面は実装で決める。** 本計画は、段落が述べる内容を設計書 3.1 の表に委ね、文面を書かない。
-   **新しい恒久的なテストは足さない**（設計書 1.2・7.1）。

### 1.3. 計画の際に決めたこと（2026-11-09）

人間のレビュアーに諮るべき未決の判断はなかった（設計書が PR の数、コミットの分け方、差分の確認の性質まで決めている）。次のことは、計画で決めたもので、理由とともに記す。PR のレビューで異論があれば改める。

-   **フェーズは 1 つにし、PR の作成ポイントは置かない。** 設計書 8 章は PR を 1 本と決めており、手順も一続きである。フェーズが 1 つの計画には PR の作成ポイントを置かず、`/runplan` がフェーズの完了の後に PR を作る（`.claude/commands/runplan.md:231`）。ADR：なし。本タスクの中で閉じる。
-   **差分の確認は Go のプログラムにする。** `docs/tasks/0069_readme_display_language/_checks/main.go` に置き、`go run` で実行する。段落の切り出しと比較を、プロジェクトの開発環境に必ずある Go で書けば、シェルと `awk` の方言（macOS の BSD 版と GNU 版）の違いに左右されない。先例はタスク 0067 の `_checks/main.go` である。ADR：なし。本タスクの中で閉じる。
-   **AC-10 と AC-11 の一部も、同じプログラムで確かめる。** 設計書は AC-10 を manual、AC-11 を「確かめて記録する」としている。プログラムは 2 つのコミットを読むので、ADR-0029 を変えるコミットのメッセージに訂正の行があること（AC-10 のうちコミットのメッセージの形）と、#727 のマージのコミットが PR の先頭の祖先であること（AC-11）を、追加の手間なく機械で確かめられる。PR の説明の訂正の段落は、PR の作成の後にしか読めないので、manual のままにする。設計書の確かめ方を置き換えるのではなく、加えるものである。ADR：なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コードの変更はない。変える文書と、変更に関わる既存の仕組みは次のとおりである（行番号は `ccc55981`）。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| `README.md:574-575` | 表示の言語の段落（3 文）。前の段落は `README.md:570-572`（「拡張で設定するものはない。」で始まる）、後の段落は `README.md:577`（「送るには、」で始まる） | 設計書 3.1 の内容に改める |
| `README.en.md:786-789` と `README.en.md:1` | 対応する段落と同期の目印。前の段落は「host is most often not installed」を含む段落、後の段落は `README.en.md:791` の「To send,」で始まる段落 | `/mktrans` の sub-step が更新する |
| `docs/dev/adr/0029-extension-ui-languages.md:24` | 「決定」の最後の項目（4 文。2〜4 文目がずれの記述） | 2〜4 文目を消す |
| `cmd/yt2column/readme_sync_test.go:79` `TestReadmeSyncMarker` | 同期の目印の検査。`make readme-sync-check` と pre-commit hook も実行する | 変えない。AC-06 の検証に使う |
| 2 つの README と ADR を読むほかの検査 | 設計書 3.4 の表のとおり。`TestReadmeVersionsShareStructure`（`cmd/yt2column/readme_structure_test.go:18`）、`TestReadmeLinks`（`cmd/yt2column/docs_test.go:672`）、`metadataDocs.test.ts`、`TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:437`）、`TestADRRecords`（`cmd/yt2column/adr_test.go:434`） | 変えない。AC-12 の `make test`・`make ext-test` で通ることを確かめる |
| ADR-0029 の文面を読むテスト | ない（`cmd`・`internal` の `*_test.go` と `extension/test` の `*.ts` を `719` と `0029-extension` で探して、該当なし） | なし |
| #727 のマージのコミット | `cf76c1f622df0883230e9b1de0c426f6aef9cb88` は `ccc55981` の祖先（`git merge-base --is-ancestor` が成功） | AC-11 の確認を差分の確認のプログラムに入れる |
| `_checks/` の先例 | `docs/tasks/0067_stage_terminology/_checks/main.go`（2 つのコミットを引数で受け取り、終了コード 0・1・2 を返す Go のプログラム） | 同じ形にならう。`go list ./...` は `_` で始まるディレクトリを無視するので、`make test`・`make lint` の対象にならない |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: README の段落の直しと ADR-0029 の訂正（全 AC）

設計書 8 章の優先順位 1〜4 を、この順で行う。優先順位 5（PR の作成）は、フェーズの完了の後に `/runplan` が行う。

**変更するファイル:** `README.md`、`README.en.md`、`docs/dev/adr/0029-extension-ui-languages.md`、`docs/tasks/0069_readme_display_language/_checks/main.go`（新規）、`docs/tasks/0069_readme_display_language/03_implementation_plan.md`（進み具合と §7 の記録）

-   [ ] **ステップ 1-1（内容のコミット 1）**: `README.md:574-575` の段落を、設計書 3.1 の表の 3 文の内容に改める。1 文目の主語は画面の文言だけを指す語にし、記事の本文が選んだ言語で表示されるとは読めない形にする（AC-01）。段落は 1 つのまま、場所も変えない。段落の外は変えない（AC-04）。
-   [ ] **ステップ 1-2（内容のコミット 1）**: `make readme-sync-check` を実行し、失敗したら `.claude/commands/mktrans.md`「When a caller runs it」のとおり `/mktrans README.md` を sub-step として実行する。変更後の `README.en.md` の段落の 3 文目が「英語の版」を述べていること（設計書 3.2）と、段落の外の行が変わっていないことを読んで確かめる。そうでなければ、`README.en.md` を `/mktrans` の前の状態に戻してやり直す（設計書 4.2）。`/mktrans` が止まったら、`README.md` だけをコミットせず、止まって利用者に報告する。用語集にない語として報告された語を控える（PR の説明に載せる）。
-   [ ] **ステップ 1-3（内容のコミット 1）**: `make readme-sync-check` と `make ext-test` が通ることを確かめ、`README.md` と `README.en.md` を 1 つのコミットに入れる。
-   [ ] **ステップ 1-4（内容のコミット 2）**: `docs/dev/adr/0029-extension-ui-languages.md:24` の項目から、ずれの記述の 3 文を消す。変更の前後は次のとおりで、項目のほかの部分と ADR のほかの行は変えない（AC-08・AC-09）。
    -   変更前：`-   **表示の言語の決まり方を利用者に説明するのは [README.md](../../../README.md) の「ブラウザ拡張」節の段落である。** この段落は、ポップアップの言語だけを述べている。現在のコードでは、ビューアも同じ規則で選んだ言語で表示する。段落がビューアに触れていないことは、記録した issue [#719](https://github.com/isseis/yt2column/issues/719) で扱う。`
    -   変更後：`-   **表示の言語の決まり方を利用者に説明するのは [README.md](../../../README.md) の「ブラウザ拡張」節の段落である。**`

    このファイルだけをコミットする。コミットのメッセージには、設計書 3.3 の形の訂正の行を 1 行だけ書く（AC-10）。行は `Editorial correction: docs/dev/adr/0029-extension-ui-languages.md: ` で始まり、何を何に直したかを自分の言葉で述べ、`; no decision changed` で終わる。`@` と `<` を含めない。経緯の項目は足さない。
-   [ ] **ステップ 1-5（コミット 3）**: 差分の確認のプログラムを `docs/tasks/0069_readme_display_language/_checks/main.go` に書く。設計書 7.2 の表の 4 項目と、本計画 §1.3 で加えた 2 項目を確かめる。
    -   AC-04・AC-07・AC-09・変えたファイルの範囲：設計書 7.2 の表のとおり。
    -   AC-10（コミットのメッセージの形）：比べる範囲のコミットのうち `docs/dev/adr/` を変えるものがちょうど 1 つで、そのコミットが変えるのは ADR-0029 だけであり、そのメッセージに `Editorial correction: docs/dev/adr/0029-extension-ui-languages.md: ` で始まり `; no decision changed` で終わる行がちょうど 1 行ある。
    -   AC-11：`cf76c1f622df0883230e9b1de0c426f6aef9cb88` が PR の先頭の祖先である。

    設計書 7.2 の 3 つの性質（2 つのコミットを引数で受け取って作業ツリーを読まない、段落の位置を前後の変わらない段落で決める、違反でも読み取りの失敗でも 0 でない終了コードで終わる）を持たせる。引数は先例にならい、マージベースを取る上流と先頭の 2 つとする。出力には、比べた 2 つのコミットの完全な ID と、項目ごとの結果を含める。コメント・識別子・文字列は英語で書く。`make fmt` と `go vet ./docs/tasks/0069_readme_display_language/_checks/` を通す。
-   [ ] **ステップ 1-6（コミット 3）**: プログラムが違反を検出できることを確かめる。PR のブランチから `git worktree` で使い捨てのブランチを作り、次の壊し方を 1 件ずつ別の一時的なコミットとして作る。それぞれのコミットを先頭として PR のブランチの作業ツリーからプログラムを実行し、壊した項目を名指して 0 でない終了コードで終わることを確かめる。
    -   `README.md` の表示の言語の段落の外の 1 行を変える（AC-04）。
    -   `README.md` の段落の中に空行を入れ、2 つの段落にする（AC-04）。
    -   `README.en.md` の段落の外の 1 行を変える（AC-07）。
    -   ADR-0029 の、「決定」の最後の項目の外の 1 行を変える。また、項目の 1 文目を変える（AC-09）。
    -   ADR-0029 のほかの ADR を 1 つ変える（AC-09）。
    -   `README.md`・`README.en.md`・ADR-0029・このタスクのディレクトリの外のファイルを 1 つ変える（範囲）。
    -   ADR-0029 を変えるコミットのメッセージから訂正の行を除いたコミットにする（AC-10）。
    -   目印にする前の段落を、`README.md` にもう 1 つ写す（段落の位置が決まらず、読み取りの失敗として終わる）。

    AC-11 は、`cf76c1f6` を含まない先頭を引数にして、失敗することを確かめる（たとえば `cf76c1f6` の親のコミットを先頭にする）。使い捨てのブランチは push せず、確かめた後に worktree とともに消す。PR のブランチの履歴は変えない。壊し方ごとの結果を、プログラムを足すコミットのメッセージに書いて、プログラムをコミットする。
-   [ ] **ステップ 1-7（コミット 4）**: `make test`・`make ext-test`・`make lint` を実行して通ることを確かめる（AC-12）。`git fetch origin main` の後、PR のブランチで `go run ./docs/tasks/0069_readme_display_language/_checks/ origin/main HEAD` を実行する。出力と、比べた 2 つのコミットの完全な ID を本計画 §7 の該当する行に書き、本計画の進み具合とともにコミットする。このコミットはこのタスクのディレクトリの下しか変えないので、記録した結果は変わらない（設計書 6.1）。記録したコミットの後に、`README.md`・`README.en.md`・`docs/dev/adr/` を変えるコミットや `main` を取り込むコミットが入ったら、設計書 7.2 の最後の段落のとおり実行し直して記録を改める。

**完了の条件:** 内容のコミット 1・2、プログラムのコミット 3、記録のコミット 4 がこの順にある。差分の確認のプログラムが終了コード 0 で終わり、その記録が §7 にある。`make test`・`make ext-test`・`make lint`・`make readme-sync-check` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: README の直し | `README.md` と `README.en.md` の内容のコミット 1 | 1-1〜1-3 |
| M2: ADR-0029 の訂正 | 訂正の行つきの内容のコミット 2 | 1-4 |
| M3: 差分の確認 | `_checks/main.go` と、壊して確かめた記録（コミット 3）、§7 の実行の記録（コミット 4） | 1-5〜1-7 |

各マイルストーンは前のマイルストーンを前提とする。M3 の後、`/runplan` が PR を作る。

### 3.2. PR 構成

PR は 1 本である（本計画 §1.3）。フェーズが 1 つなので PR の作成ポイントはなく、`/runplan` がフェーズの完了の後に PR を作り、§6 に PR の記録を足す。実装の PR は `.md` と `docs/` の下と `_checks/` の Go のファイルしか変えない。`_checks/` は `go list ./...` の外なので、CI の `check-changes` の判定は設計書 3.4 のとおりと見込むが、実際に動いたジョブは PR の上で確かめる。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

-   新しい恒久的なテストは足さない（設計書 7.1）。既存のテストが通ることを `make test` と `make ext-test` で確かめる（AC-12）。どの検査が影響を受けうるかは設計書 3.4 の表のとおりである。
-   同期の目印は `make readme-sync-check` で確かめる（AC-06）。pre-commit hook も同じテストを実行する。

### 4.2. 差分の確認

-   AC-04・AC-07・AC-09、変えたファイルの範囲、AC-10 のコミットのメッセージの形、AC-11 を、`_checks/main.go` で 1 回確かめる（ステップ 1-5・1-7）。
-   プログラムが違反を検出できることは、ステップ 1-6 で壊し方ごとに確かめる。

### 4.3. 読んで確かめる

-   AC-01〜AC-03・AC-05・AC-08、AC-10 の PR の説明の訂正の段落は、PR のレビューで読む（設計書 7.3）。

### 4.4. 結合テスト・後方互換のテスト

N/A。実行時の振る舞いを変えない（設計書 7.4）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `/mktrans` が段落の外の行を変える、または 3 文目を日本語版の直訳（「日本語の版」）にする | AC-05・AC-07 を満たさない | ステップ 1-2 で読んで確かめ、`README.en.md` を戻してやり直す。差分の確認のプログラムも段落の外の変更を検出する。やり直しても直らなければ止まって報告する（設計書 4.2） |
| `/mktrans` が止まる | `README.md` だけのコミットになる | `README.md` だけをコミットしない。pre-commit hook と CI の `doc-guard` も、目印のずれた README を拒む（設計書 4.2） |
| README を変える別のタスク（タスク 0066 の残りなど）が先にマージされ、`main` を取り込む | 同期の目印が衝突し、差分の確認の記録が古くなる | `/mktrans` をやり直し（`.claude/commands/mktrans.md`「Taking in `main`」）、AC-05 の段落を読み直し、差分の確認を実行し直して §7 の記録を改める（設計書 7.2） |
| 段落の 1 文目の主語が「ビューア」になり、記事の本文も英語になると読める | AC-01 を満たさない | ステップ 1-1 で主語を画面の文言に限る。PR のレビューで読んで確かめる |
| 目印にする前後の段落が、`main` の取り込みで変わるか、2 回以上現れる | 差分の確認が段落の位置を決められない | プログラムは読み取りの失敗として 0 でない終了コードで終わる。そのときは目印を選び直してプログラムを直し、ステップ 1-6 をやり直す |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1〜1-7）
- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）と `make ext-test` が通ることを確認した

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

差分の確認のプログラムは `docs/tasks/0069_readme_display_language/_checks/main.go`（以下、`_checks`）である。実行の記録（日付、マージベースと PR の先頭の完全なコミット ID、出力、終了コード）は、ステップ 1-7 で各行に書く。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | manual | 変更後の `README.md` の段落を PR のレビューで読む。記事の本文まで UI の言語で表示されると読めないこと（設計書 3.1・7.3） | 1-1 |
| AC-02 | manual | 同上 | 1-1 |
| AC-03 | manual | 同上 | 1-1 |
| AC-04 | diff | `_checks`（`README.md` の表示の言語の段落の外が前後で同じで、変更後の段落が 1 つの段落のまま）。実行の記録: （ステップ 1-7 で書く） | 1-5〜1-7 |
| AC-05 | manual | 変更後の `README.en.md` の段落を PR のレビューで読む。3 文目が「英語の版」を述べること（設計書 3.2・7.3） | 1-2 |
| AC-06 | test | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`（`make readme-sync-check`、`make test`、CI の `doc-guard`） | 1-2・1-3・1-7 |
| AC-07 | diff | `_checks`（`README.en.md` の 1 行目と表示の言語の段落の外が前後で同じで、変更後の段落が 1 つの段落のまま）。実行の記録: （ステップ 1-7 で書く） | 1-5〜1-7 |
| AC-08 | manual | 変更後の ADR-0029 の「決定」の最後の項目を PR のレビューで読む（設計書 7.3） | 1-4 |
| AC-08 | diff | `_checks`（ADR-0029 の変更後の項目が、変更前の項目を 1 文目の末尾で切ったものに等しい。AC-09 の行と同じ確認） | 1-5〜1-7 |
| AC-09 | diff | `_checks`（`docs/dev/adr/` の下で変わったのは ADR-0029 だけで、追加・削除・名前の変更がなく、ADR-0029 の変更は項目の切り取りだけ）。実行の記録: （ステップ 1-7 で書く） | 1-5〜1-7 |
| AC-10 | static | `_checks`（`docs/dev/adr/` を変えるコミットがちょうど 1 つで、ADR-0029 だけを変え、そのメッセージに ADR-0029 の訂正の行がちょうど 1 行ある）。実行の記録: （ステップ 1-7 で書く） | 1-4〜1-7 |
| AC-10 | manual | 訂正の行の内容と、PR の説明の訂正の段落を PR のレビューで読む（設計書 7.3） | 1-4、PR の作成 |
| AC-11 | static | `_checks`（`cf76c1f622df0883230e9b1de0c426f6aef9cb88` が PR の先頭の祖先）。実行の記録: （ステップ 1-7 で書く） | 1-5〜1-7 |
| AC-12 | test | `make test`、`make ext-test` | 1-3・1-7 |

AC-04・AC-07・AC-09 のほか、設計書 1.1 の「変えたファイルの範囲」も `_checks` で確かめ、その実行の記録を AC-04 の行に合わせて書く。

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** `README.md` と `README.en.md` の段落がポップアップとビューアの両方に触れ、ADR-0029 のずれの記述がない。§7 のすべての AC の検証が済んでいる。
-   **品質:** `make test`・`make ext-test`・`make lint`・`make readme-sync-check` が通る。`_checks` が終了コード 0 で終わり、壊し方ごとに失敗することを確かめた記録がコミットのメッセージにある。
-   **セキュリティ:** N/A（設計書 5 章）。
-   **文書:** 変更は設計書 1.1 の 3 か所とこのタスクのディレクトリの下だけにある。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0069` でフェーズ 1 を行い、PR を作る。
-   実装の PR のマージの後、issue #719 を閉じる（`/runpr` が閉じる。`Closes #N` では自動で閉じない）。
