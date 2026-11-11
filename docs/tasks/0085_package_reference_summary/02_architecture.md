# 設計書：package_reference.md の行を責務の要約に絞る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

## 1. 設計の全体像

本タスクでは、パッケージの参照表（`docs/dev/conventions/package_reference.md`）を書き直す。あわせて、行から外した詳細の行き先を整え、その詳細を参照していた文を直す（[01_requirements.md](01_requirements.md) F-001〜F-009）。コードの振る舞いは変えない。Go のファイルで変えるのは、doc コメントと、strictjson の利用者の一覧を読むテスト（F-009）だけである。

ADR-0044 とその索引の行は、本書と同じ 3 文書の PR で作る（[01_requirements.md](01_requirements.md) F-006）。

**設計原則:**

-   **行は、ADR が置き場所として参照する内容を落とさない。** ADR が行を参照する箇所のうち、新しい粒度に収まる内容（番兵の名前、責務と、ADR が引く規則の 1 文）は行に残す。収まらない内容を指す参照だけを、ADR の側で直す（3.3 節）。
-   **詳細は写さずにリンクする。** ほかの今の文書にすでにある詳細は、行からその文書へのリンクにする。どの文書にもない呼び出し側の契約だけを doc コメントへ移す（[01_requirements.md](01_requirements.md) F-004）。
-   **`main` の文書の参照を古いまま残さない。** ADR の参照と、パッケージの参照表を説明する文（`CLAUDE.md`・`pricing_update.md`）は、それを古くする変更と同じ PR で直す（8 章）。

**設計の際に決めたこと（2026-11-11）:**

-   **パッケージをどの表に載せるかは、3.1 節の表のとおりとする。新しいパッケージを足すときに迷わないよう、冒頭に 6 つの種類それぞれの定義を 1 行ずつ書く。** 設定の読み込み（`internal/config`）とディレクトリの排他（`internal/cachelock`）は、CLI とサーバの両方が使うので、共有部品に入れる。テスト用の補助（`testutil` と、テストからだけ使うパッケージ）は、親のパッケージの種類にかかわらずテスト専用の表に入れる。テストを書く人が、補助をまとめて探せるようにするためである。
    -   ADR：なし。表の中身の選択で、本タスクの中で閉じる。種類ごとに分ける方針そのものは ADR-0044 にある。
-   **表の中の行は、パッケージのパスの名前順に並べる。** 名前で探すときに目で追いやすく、新しい行を入れる位置にも迷わないためである。
    -   ADR：なし。本タスクの中で閉じる。
-   **ADR の参照の直し方は、3.3 節の表のとおりとする。** 直し先は、その ADR がすでに挙げている別の正本のうち今の挙動を述べるものか、その内容を doc コメントに持つパッケージとする（[01_requirements.md](01_requirements.md) F-005）。古い設計書の節を直し先にすると、参照が指すものと合わなくなるためである。
    -   ADR：なし。各直しは既存の ADR の編集上の訂正で、決定を変えない。
-   **実装の PR を 2 本に分け、ADR の参照の訂正と、`CLAUDE.md`・`pricing_update.md` の直しは、それを古くする PR で行う（8 章）。** 1 本目のマージから 2 本目のマージまで、`main` の文書が持たない内容を指し続けないためである。
    -   ADR：なし。本タスクの中で閉じる。
-   **`TestPackageReferenceStrictjsonUsers` の変更は、表を分ける変更と同じ PR-1 で、その前に行う（3.5 節）。** 表を分けたコミットからテストが通るようにするためである。
    -   ADR：なし。本タスクの中で閉じる。
-   **確かめ方は 7 章のとおりとする。状態を確かめるスクリプト 1 つ、差分を確かめるスクリプト 1 つ、記録した diff の読み取り、既存のテスト、レビューでの読み取りを組み合わせる。** 文書の構造と変更の範囲は機械で確かめ、文の内容はレビューで読むためである（`CLAUDE.md`「Testing Strategy」）。差分が数行の ADR と 2 つの文書は、スクリプトを書くより diff を読むほうが手間が少ない。
    -   ADR：なし。本タスクの中で閉じる。

概念モデルの Mermaid の図は省く。変わるのは文書とコメントとテスト 1 つだけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

変更・追加するファイルは次のとおりである。

| ファイル | 変更 | PR |
|---|---|---|
| `docs/dev/conventions/package_reference.md` | 冒頭の規則、6 つの表、全行の書き直し（3.1・3.2 節） | PR-1・PR-2 |
| `cmd/`・`internal/` の下の既存の Go のファイル | doc コメントの追加と書き換え（3.4 節） | PR-1・PR-2 |
| `internal/pipeline/strictjson_users_test.go` | すべての表から行を探す（3.5 節） | PR-1 |
| `docs/dev/adr/0001-llm-adapter-stdlib.md`・`0020-strict-json.md`・`0023-artifact-resume.md` | 参照の編集上の訂正（3.3 節） | PR-1 |
| `docs/dev/adr/0027-article-job-api.md` | 参照の編集上の訂正（3.3 節） | PR-2 |
| `docs/dev/adr/0044-package-reference-row-granularity.md` | ずれの記述の書き換え（PR-1）と削除（PR-2） | PR-1・PR-2 |
| `CLAUDE.md` | Quick Links の説明文の 1 行 | PR-1 |
| `docs/dev/guides/pricing_update.md` | `internal/pricing` の行についての 1 文 | PR-1 |
| `docs/tasks/0085_package_reference_summary/_checks/check_state.sh`（新規） | 状態を確かめるスクリプト（7 章） | PR-1 |
| `docs/tasks/0085_package_reference_summary/_checks/check_diff.sh`（新規） | 差分を確かめるスクリプト（7 章） | PR-1 |

### 3.1. 文書の構成

パッケージの参照表は、次の構成にする。文書は英語で書く（[01_requirements.md](01_requirements.md) 5 章）。

1.  冒頭：文書の目的と、運用の規則。
    -   更新の規則：今の規則（`docs/dev/conventions/package_reference.md:3-6`）を保つ（AC-03）。
    -   行に書くもの・書かないもの（AC-01）。
    -   詳細の置き場所（AC-02）。
    -   6 つの種類の定義と、どれにも当たらないパッケージの扱い（AC-04）。定義は箇条書きにし、表にしない。表にすると、`TestPackageReferenceListsPackages` がその行をパッケージの行として数えるためである（`internal/pipeline/pipeline_test.go:672` は `|` で始まるすべての行を読む）。
    -   `project_overview.md` へのリンク：今の第 2 段落（`docs/dev/conventions/package_reference.md:8-10`）を保つ。
2.  6 つの節：種類ごとに `## ` の見出しを 1 つ置き、その下に `| Package | Responsibility |` の表を 1 つ置く（AC-05）。

パッケージの割り当ては次のとおりである（コミット `04dc4824` の 56 行）。

| 種類（節） | パッケージ |
|---|---|
| パイプラインの構成要素 | `cmd/yt2column`、`internal/artifact`、`internal/job`、`internal/pipeline`、`internal/pricing`、`internal/publisher`、`internal/transcript`、`internal/writer`、`prompts` |
| サーバとホスト | `cmd/yt2column-nmhost`、`cmd/yt2column-server`、`internal/articlecache`、`internal/nmhost`、`internal/privatefile`、`internal/server`、`internal/serversock`、`internal/slackwebhook` |
| LLM | `internal/llm`、`internal/llm/claude`、`internal/llm/claudeparam`、`internal/llm/deepseek`、`internal/llm/deepseekparam`、`internal/llm/llmhttp`、`internal/llm/openai`、`internal/llm/openaiparam`、`internal/llm/provider` |
| 共有部品 | `internal/cachelock`、`internal/cmdrunner`、`internal/config`、`internal/nilcheck`、`internal/redact`、`internal/secret`、`internal/signalrun`、`internal/strictjson` |
| 開発ツール | `cmd/codexreview`、`cmd/coveragecompare`、`cmd/fixprguard`、`cmd/mergepr`、`internal/codexreview`、`internal/coveragecompare`、`internal/fixprguard`、`internal/mergepr` |
| テスト専用 | `internal/cmdrunner/testutil`、`internal/codexreview/testutil`、`internal/doctestutil`、`internal/llm/claude/testutil`、`internal/llm/deepseek/testutil`、`internal/llm/llmhttp/llmhttptest`、`internal/llm/testutil`、`internal/loopbacktest`、`internal/maketestutil`、`internal/pricing/testutil`、`internal/publisher/testutil`、`internal/transcript/testutil`、`internal/udstest`、`internal/writer/testutil` |

実装のときの `main` にパッケージの増減があれば（たとえばタスク 0083 が `internal/slackwebhook` を消す）、そのときのパッケージの集合で割り当てる。`TestPackageReferenceListsPackages` は、その集合と表の行が一致することを確かめる。

### 3.2. 行の中身

各行の Responsibility の欄は、次の順に書く（AC-01・AC-07）。

1.  責務（1〜3 文）。ADR が置き場所として参照する規則は、ここに 1 文で残してよい。
2.  主な公開の型・関数・番兵の名前。
3.  詳しい仕様の置き場所へのリンク（あるとき）。行き先の文書ごとに 1 つまでとし、詳細ごとには張らない。

`internal/strictjson` の欄だけは、リンクを利用者の一覧より前に置き、`Shared by A, B, and C` の一覧で終える（AC-08）。`TestPackageReferenceStrictjsonUsers` は、一覧の後ろに文があると拒否するためである（`internal/pipeline/strictjson_users_test.go:230` の `parseUserList`）。

ADR が置き場所として参照していて、行に残す内容は次のとおりである（3.3 節の「残す」）。

| 行 | 残す内容 | 参照する ADR |
|---|---|---|
| `internal/llm` | どのプロバイダでも起こる失敗の番兵の一覧、`GenerateRequest.Validate`、使用量の型（`TokenCounts`・`Usage`） | ADR-0019、ADR-0021 |
| `internal/llm/claude`・`internal/llm/deepseek`・`internal/llm/openai` | 各アダプタの番兵の名前 | ADR-0019 |
| `internal/llm/llmhttp` | プロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告すること | ADR-0001 |
| `internal/llm/claudeparam`・`internal/llm/deepseekparam`・`internal/llm/openaiparam` | 設定の読み込みとアダプタの両方が使うプロバイダ固有のパラメータの型を持ち、標準ライブラリだけに依存すること | ADR-0001 |
| `internal/strictjson` | `Shared by A, B, and C` の書式の利用者の一覧（AC-08） | ADR-0020 |
| `internal/secret` | 責務（秘密を包む型を持つこと） | ADR-0031 |

### 3.3. ADR の参照の直し

コミット `04dc4824` で、ADR がパッケージの参照表を参照する文は次のとおりである（AC-14）。「直す」の行は、編集上の訂正として、参照の文からパッケージの参照表の行を外す。直した後は、その ADR がすでに挙げている今の正本か、その内容を doc コメントに持つパッケージを指す（AC-15）。ADR-0005:18 と ADR-0019:19 の「パッケージの表」は、表が 6 つになっても文書の表全体を指す言い方として正しいので、残す。

| 参照（ADR の行） | 指している内容 | 扱い | 直した後の参照先 | PR |
|---|---|---|---|---|
| ADR-0001:20 | `llmhttp` の送信の手順 | 直す | `internal/llm/llmhttp` のパッケージの doc コメント | PR-1 |
| ADR-0001:21 | `llmhttp` は番兵を持たない規則 | 残す | — | — |
| ADR-0001:22 | パラメータの型の置き場所と依存 | 残す | — | — |
| ADR-0005:18 | パッケージの表（構成要素の分け方） | 残す | — | — |
| ADR-0019:19 | パッケージの表（構成要素の分け方） | 残す | — | — |
| ADR-0019:23 | `internal/llm` の番兵の一覧 | 残す | — | — |
| ADR-0019:24 | 各アダプタの番兵 | 残す | — | — |
| ADR-0020:17 | strictjson が拒否する入力 | 直す | 0003 設計書「3.5. `internal/strictjson`（厳格な JSON の部品）」と、`internal/strictjson` の doc コメント | PR-1 |
| ADR-0020:19 | strictjson の各手段の契約 | 直す | `internal/strictjson` の各メソッドの doc コメント | PR-1 |
| ADR-0020:20 | `Value.AsNonNegativeInt64` の受理する構文とエラー | 直す | `Value.AsNonNegativeInt64` の doc コメント（コミット `04dc4824` の `internal/strictjson/strictjson.go:262-266`） | PR-1 |
| ADR-0020:22 | strictjson の利用者の一覧 | 残す | — | — |
| ADR-0020:37・38・47 | 利用者の一覧の更新と、その経緯 | 残す | — | — |
| ADR-0021:24 | 使用量の型 | 残す | — | — |
| ADR-0023:20 | 成果物の形式 | 直す | `internal/artifact` のパッケージの doc コメントと、0077 設計書「3.4.2. 中間生成物と既存の再開の互換性」 | PR-1 |
| ADR-0027:20 | ホストが受け付けるメッセージ | 直す | security.md §8.2 だけ | PR-2 |
| ADR-0027:27 | 記事のキャッシュが記録を捨てる規則 | 直す | security.md §8.2 だけ | PR-2 |
| ADR-0031:17 | `internal/secret` の役割 | 残す | — | — |

-   ADR-0023:20 の今の参照先のうち、0017 設計書 §3.2 は版 1 の形式を述べる（`docs/tasks/0017_intermediate_artifacts/02_architecture.md:296`）。今の形式（版 4 と版 3 の読み込み）を述べるのは 0077 設計書と `internal/artifact` の doc コメント（`internal/artifact/artifact.go:19-22`）なので、直し先をこの 2 つにする。
-   ADR-0020:17 の 0003 設計書 §3.5 は、拒否する入力（不正な UTF-8、対にならないサロゲートのエスケープ、後続のデータ）を今も正しく述べる。ただし、後のタスクが足した手段の契約は述べていない（たとえば `Has` の扱い）。このため、拒否する入力は 0003 設計書と doc コメントの両方を指し、手段の契約（ADR-0020:19）は doc コメントだけを指す。
-   ADR の本文には行番号を書かない（`docs/dev/conventions/adr.md`「Links」）。表の行番号は、この設計書が参照を探すためだけのものである。
-   直した後の参照先が doc コメントのものは、その内容が doc コメントにあることを、直す PR で確かめる。ないときは、その PR で doc コメントへ移す（AC-11）。
-   実装のときの `main` で ADR の行番号や参照が変わっていれば、そのときの `git grep -n package_reference -- docs/dev/adr/` の出力で、この表を読み替える。
-   ADR-0044 のずれの記述は、PR-1 が「後半の 3 つの表の行がまだ規則に従っていない」と、残るずれだけを述べる形に書き換え、PR-2 が消す（AC-17）。どちらも、`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類（ずれの記述の削除と書き換え）である。

### 3.4. 行から外した詳細の扱い

行から外す詳細は、行ごとに [01_requirements.md](01_requirements.md) 1 章の「行から外す詳細の扱い」の順で扱う（F-004）。

-   タスクの設計書の節にリンクするときは、その節が今の挙動を述べていることを、コードと照らして確かめる（AC-10）。
-   doc コメントは既存の Go のファイルに書き、英語で書く。新しい `doc.go` は作らない（AC-12）。
-   秘密の扱いと子プロセスの環境変数に関わる詳細（`internal/codexreview`・`internal/redact`・`internal/secret` などの行）は、要件定義書の「呼び出し側が頼る契約」の定義に当たるので、リンクか doc コメントで扱い、削るだけにはしない。

PR の説明には、PR で絞った行ごとに、外した文か節のすべてと、その行き先の一覧を載せる（AC-13）。

### 3.5. strictjson の利用者の一覧を読むテスト

`strictjsonRowCell`（`internal/pipeline/strictjson_users_test.go:196`）は、`doctestutil.FindTable` で `Package` の列を持つ最初の表だけを読む（`internal/doctestutil/tables.go:61-75`）。これを、`Package` の列を持つすべての表を読み、`internal/strictjson` の行を集める形に直す（AC-21）。

-   行がどの表にもないときは、今の `errNoStrictjsonRow` で失敗する。
-   行が 2 つ以上あるとき（別々の表にあるときを含む）は、今の `errDuplicateStrictjsonRow` で失敗する。
-   `Package` の列を持つ表が 1 つもないときは、今の `errNoPackageTable` で失敗する。

`FindTable` 自体と、ほかの guard テストは変えない。

## 4. エラーハンドリング設計

N/A。実行時の処理を変えない。テストの失敗の番兵は、3.5 節のとおり今のものを使う。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の規則と前提を変えず、コードの振る舞いも変えない（[01_requirements.md](01_requirements.md) 4.2）。秘密と子プロセスの環境変数に関わる詳細の扱いは、3.4 節のとおりである。

## 6. 処理フローの詳細

N/A。処理のフローがない。

## 7. テスト戦略

恒久のテストは足さない。粒度の上限をテストで守らないことは、要件定義書で決めた（[01_requirements.md](01_requirements.md) 1 章）。確かめ方は次のとおりである。

-   **既存のテスト（`test`）:** `TestPackageReferenceListsPackages`（AC-06 のパッケージの集合）、`TestPackageReferenceStrictjsonUsers`（AC-08、3.5 節の直しの後）、`TestADRRecords`（AC-16 の索引の行と ADR の構造）、`TestInstructionDocPaths`（足したリンクの行き先）。どれも `make test` と CI の `doc-guard` で走る。
-   **テストの直し（`test`）:** 3.5 節の直しは、`TestStrictjsonRowCell`（`internal/pipeline/strictjson_users_test.go:417`）の表に、行が 2 つ目の表にある場合と、別々の表に 1 つずつある場合を足して確かめる（AC-21）。
-   **状態を確かめるスクリプト（`static`）:** `_checks/check_state.sh`。パッケージの参照表を読み、次を確かめる。
    -   `## ` の見出しが 6 つあり、各見出しの下に表が 1 つずつある（AC-05 の構造）。
    -   どのパッケージ名も、ちょうど 1 行に載る（AC-06 の重複の部分）。
    -   文書のバイト数（AC-09）。15,000 バイトの上限を判定に使うのは、PR-2 の実行だけである。
-   **差分を確かめるスクリプト（`diff`）:** `_checks/check_diff.sh`。マージベースと PR のコミットを比べ、次を確かめる。
    -   変えたファイルが AC-20 の範囲にあり、Go のファイルの追加がない。
    -   Go のファイルの差分が、`internal/pipeline/strictjson_users_test.go` を除いて、コメントの行（`//` で始まる行と、それに挟まれた空行）に限る（AC-12）。
-   **記録した diff の読み取り（`diff`）:** 各 PR の ADR の差分（AC-15）と、`CLAUDE.md`（AC-18）、`pricing_update.md`（AC-19）の差分である。
-   **読んで確かめる（`manual`）:** 冒頭の規則の内容（AC-01〜AC-04）、見出しが 6 つの種類に当たり割り当てが 3.1 節に合うこと（AC-05）、各行の中身（AC-07）、詳細の行き先（AC-10・AC-11）、PR の説明の一覧（AC-13）、ADR の参照が行の内容と合うこと（AC-14）、ADR-0044 の内容と issue の本文（AC-16）、ずれの記述の扱い（AC-17）、`CLAUDE.md` と `pricing_update.md` の文の内容（AC-18・AC-19）。

ADR-0044 は本書の 3 文書の PR に入るので、AC-16 はその PR のレビューと CI で確かめる。

## 8. 実装の優先順位

実装の PR は 2 本である。

| PR | 内容 |
|---|---|
| PR-1 | `_checks/` の 2 つのスクリプト、strictjson のテストの直し（3.5 節）、冒頭の規則、表を 6 つに分けること（全行を新しい節へ移す）、パイプラインの構成要素・LLM・共有部品の行を絞ること、3.3 節の PR-1 の ADR の訂正、ADR-0044 のずれの記述の書き換え、`CLAUDE.md` と `pricing_update.md` の直し |
| PR-2 | サーバとホスト・開発ツール・テスト専用の行を絞ること、3.3 節の PR-2 の ADR の訂正、ADR-0044 のずれの記述の削除、文書の大きさの確認（AC-09） |

PR-1 では、PR-2 で絞る節の行は、新しい節へ内容を変えずに移す。

## 9. 将来の拡張性

どの種類にも当たらないパッケージ（たとえば WASM の入口）が増えたときは、そのパッケージを足すタスクが、種類を足すか既存の種類の定義を広げるかを決める（AC-04）。

## 変更履歴

- 2026-11-11 承認（isseis、#821）
