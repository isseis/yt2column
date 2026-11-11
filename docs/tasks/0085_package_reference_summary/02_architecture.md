# 設計書：package_reference.md の行を責務の要約に絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクでは、パッケージの参照表（`docs/dev/conventions/package_reference.md`）を書き直す。あわせて、行から外した詳細の行き先と、参照の文を直す（[01_requirements.md](01_requirements.md) F-001〜F-008）。コードの振る舞いは変えない。Go のファイルで変えるのは doc コメントだけである。

変えるものは次のとおりである。

-   パッケージの参照表：冒頭の規則を書き換え、表を 6 つに分け、全行を絞る。
-   doc コメント：行から外した契約のうち、どの文書にもないものを書き足す。
-   ADR の参照の文：絞った行がもう持たない内容を指す 7 か所を、編集上の訂正として直す（3.3 節）。
-   ADR-0044 のずれの記述：実装の進み具合に合わせて、書き換えてから消す。
-   `CLAUDE.md:18` と `docs/dev/guides/pricing_update.md:39`：それぞれ 1 行を直す。

ADR-0044 とその索引の行は、本書と同じ 3 文書の PR で作る（[01_requirements.md](01_requirements.md) F-006）。

**設計原則:**

-   **行は、ADR が置き場所として参照する内容を落とさない。** ADR が行を参照する箇所のうち、新しい粒度に収まる内容（番兵の名前、責務）は行に残す。収まらない内容を指す参照だけを、ADR の側で直す（3.3 節）。
-   **詳細は写さずにリンクする。** ほかの文書にすでにある詳細は、行からその文書へのリンクにする。どの文書にもない契約だけを doc コメントへ移す（[01_requirements.md](01_requirements.md) F-004）。
-   **`main` の ADR の参照を古いまま残さない。** ADR の参照は、その参照先の行を絞る PR で直す（8 章）。

**設計の際に決めたこと（2026-11-11）:**

-   **パッケージをどの表に載せるかは、3.1 節の表のとおりとする。新しいパッケージを足すときに迷わないよう、冒頭に 6 つの種類それぞれの定義を 1 行で書く。** 設定の読み込み（`internal/config`）と、ディレクトリの排他（`internal/cachelock`）は、CLI とサーバの両方が使うので、共有部品に入れる。テスト用の補助（`testutil` と、テストからだけ使うパッケージ）は、親のパッケージの種類にかかわらずテスト専用の表に入れる。
    -   ADR：なし。表の中身の選択で、本タスクの中で閉じる。種類ごとに分ける方針そのものは ADR-0044 にある。
-   **表の中の行は、パッケージのパスの名前順に並べる。** 名前で探すときに目で追いやすく、新しい行を入れる位置にも迷わないためである。
    -   ADR：なし。本タスクの中で閉じる。
-   **ADR の参照の直し方は、3.3 節の表のとおりとする。** 直し先は、その ADR がすでに挙げている別の正本か、その内容を doc コメントに持つパッケージである（[01_requirements.md](01_requirements.md) F-005）。
    -   ADR：なし。各直しは既存の ADR の編集上の訂正で、決定を変えない。
-   **実装の PR を 2 本に分け、ADR の参照の訂正と `pricing_update.md` の直しは、その参照先の行を絞る PR で行う（8 章）。** 先に確かめた分け方では ADR の訂正をすべて 2 本目に置いていた。しかしその分け方では、1 本目のマージから 2 本目のマージまで、`main` の ADR が持たない内容を「行にある」と言い続ける。人に確かめて、分け方をこう改めた。
    -   ADR：なし。本タスクの中で閉じる。
-   **確かめ方は 7 章のとおりとする。** 状態を確かめるスクリプト 1 つ、差分を確かめるスクリプト 1 つ、記録した diff の読み取り、既存のテスト、レビューでの読み取りを組み合わせる。
    -   ADR：なし。本タスクの中で閉じる。

概念モデルの Mermaid の図は省く。変わるのは文書とコメントだけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 文書の構成

パッケージの参照表は、次の構成にする。文書は英語で書く（[01_requirements.md](01_requirements.md) 5 章）。

1.  冒頭：文書の目的と、運用の規則。
    -   更新の規則：今の規則（`docs/dev/conventions/package_reference.md:3-6`）を保つ（AC-03）。
    -   行に書くもの・書かないもの（AC-01）。
    -   詳細の置き場所（AC-02）。
    -   6 つの種類の定義。
    -   `project_overview.md` へのリンク：今の第 2 段落（`docs/dev/conventions/package_reference.md:8-10`）を保つ。
2.  6 つの節：種類ごとに `## ` の見出しを 1 つ置き、その下に `| Package | Responsibility |` の表を 1 つ置く（AC-04）。

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

各行の Responsibility の欄は、次の順に書く（AC-01・AC-06）。

1.  責務（1〜3 文）。
2.  主な公開の型・関数・番兵の名前。
3.  詳しい仕様の置き場所へのリンク（あるとき）。

ADR が置き場所として参照していて、行に残す内容は次のとおりである（3.3 節の「残す」）。

| 行 | 残す内容 | 参照する ADR |
|---|---|---|
| `internal/llm` | どのプロバイダでも起こる失敗の番兵の一覧、`GenerateRequest.Validate`、使用量の型（`TokenCounts`・`Usage`） | ADR-0019、ADR-0021 |
| `internal/llm/claude`・`internal/llm/deepseek`・`internal/llm/openai` | 各アダプタの番兵の名前 | ADR-0019 |
| `internal/llm/llmhttp` | プロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告すること | ADR-0001 |
| `internal/llm/claudeparam`・`internal/llm/deepseekparam`・`internal/llm/openaiparam` | 設定の読み込みとアダプタの両方が使う、プロバイダ固有のパラメータの型を持つこと | ADR-0001 |
| `internal/strictjson` | `Shared by A, B, and C` の書式の利用者の一覧（AC-07） | ADR-0020 |
| `internal/secret` | 責務（秘密を包む型を持つこと） | ADR-0031 |

### 3.3. ADR の参照の直し

コミット `04dc4824` で、ADR がパッケージの参照表を参照する文は次のとおりである（AC-13）。「直す」の行は、編集上の訂正として、参照の文からパッケージの参照表の行を外す。直した後は、その ADR がすでに挙げている別の正本か、その内容を doc コメントに持つパッケージを指す（AC-14）。

| 参照（ADR の行） | 指している内容 | 扱い | 直した後の参照先 | PR |
|---|---|---|---|---|
| ADR-0001:20 | `llmhttp` の送信の手順 | 直す | `internal/llm/llmhttp` のパッケージ（doc コメント） | PR-1 |
| ADR-0001:21 | `llmhttp` は番兵を持たない規則 | 残す | — | — |
| ADR-0001:22 | パラメータの型の置き場所 | 残す | — | — |
| ADR-0005:18 | パッケージの表（構成要素の分け方） | 残す | — | — |
| ADR-0019:19 | パッケージの表（構成要素の分け方） | 残す | — | — |
| ADR-0019:23 | `internal/llm` の番兵の一覧 | 残す | — | — |
| ADR-0019:24 | 各アダプタの番兵 | 残す | — | — |
| ADR-0020:17 | strictjson が拒否する入力 | 直す | 0003 設計書「3.5. `internal/strictjson`（厳格な JSON の部品）」だけ | PR-1 |
| ADR-0020:19 | strictjson の各手段の契約 | 直す | `internal/strictjson` のパッケージ（doc コメント） | PR-1 |
| ADR-0020:20 | `Value.AsNonNegativeInt64` の受理する構文とエラー | 直す | `internal/strictjson` の `Value.AsNonNegativeInt64`（doc コメント。`internal/strictjson/strictjson.go:252-256`） | PR-1 |
| ADR-0020:22 | strictjson の利用者の一覧 | 残す | — | — |
| ADR-0020:37・38・47 | 利用者の一覧の更新と、その経緯 | 残す | — | — |
| ADR-0021:24 | 使用量の型 | 残す | — | — |
| ADR-0023:20 | 成果物の形式 | 直す | 0017 設計書 §3.2「成果物の形式と識別子（H-01）」だけ | PR-1 |
| ADR-0027:20 | ホストが受け付けるメッセージ | 直す | security.md §8.2 だけ | PR-2 |
| ADR-0027:27 | 記事のキャッシュが記録を捨てる規則 | 直す | security.md §8.2 だけ | PR-2 |
| ADR-0031:17 | `internal/secret` の役割 | 残す | — | — |

-   直した後の参照先が doc コメントの行では、その内容が doc コメントにあることを、直す PR で確かめる。ないときは、その PR で doc コメントへ移す（AC-10）。
-   実装のときの `main` で ADR の行番号や参照が変わっていれば、そのときの `git grep -n package_reference -- docs/dev/adr/` の出力で、この表を読み替える。
-   ADR-0044 のずれの記述は、PR-1 が「行の一部がまだ規則に従っていない」と残るずれだけを述べる形に書き換え、PR-2 が消す（AC-16）。どちらも、`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類（ずれの記述の削除と書き換え）である。

### 3.4. 行から外した詳細の扱い

行から外す詳細は、行ごとに次の順で扱う（[01_requirements.md](01_requirements.md) F-004）。

1.  ほかの文書（`cli.md`・`development.md`・`security.md`・README・`production_profile.md`・タスクの設計書）にあれば、行からその文書へリンクする（AC-09）。
2.  どの文書にもなく、呼び出し側が頼る契約なら、パッケージか公開の識別子の doc コメントへ移す（AC-10）。doc コメントは英語で書く。
3.  それ以外は削る。

PR の説明には、PR で絞った行ごとに、外した詳細がどれに当たるかの一覧を載せる（AC-12）。

既存のテストで、パッケージの参照表の行の文を確かめるものは 2 つだけである（[01_requirements.md](01_requirements.md) 1 章）。doc コメントの文を確かめるテストはない。

## 4. エラーハンドリング設計

N/A。実行時の処理を変えない。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の規則と前提を変えず、コードの振る舞いも変えない（[01_requirements.md](01_requirements.md) 4.2）。行から外す詳細のうち、秘密の扱いと子プロセスの環境変数に関わるもの（`internal/codexreview`・`internal/redact`・`internal/secret` などの行）は、3.4 節の 1 か 2 で扱い、削るだけにしない。

## 6. 処理フローの詳細

N/A。処理のフローがない。

## 7. テスト戦略

恒久のテストは足さない。粒度の上限をテストで守らないことは、要件で決めた（[01_requirements.md](01_requirements.md) 1 章）。確かめ方は次のとおりである。

-   **既存のテスト（`test`）:** `TestPackageReferenceListsPackages`（AC-05 のパッケージの集合）、`TestPackageReferenceStrictjsonUsers`（AC-07）、`TestADRRecords`（AC-15 の索引の行と ADR の構造）、`TestInstructionDocPaths`（足したリンクの行き先）。どれも `make test` と CI の `doc-guard` で走る。
-   **状態を確かめるスクリプト（`static`）:** タスクの `_checks/` に置くシェルスクリプトである。パッケージの参照表を読み、次を確かめる。
    -   `## ` の見出しが 6 つあり、各見出しの下に表が 1 つずつある（AC-04）。
    -   どのパッケージ名も、ちょうど 1 行に載る（AC-05 の重複の部分）。
    -   文書のバイト数（AC-08）。15,000 バイトの上限を判定に使うのは、PR-2 の実行だけである。
-   **差分を確かめるスクリプト（`diff`）:** タスクの `_checks/` に置くシェルスクリプトである。マージベースと PR のコミットを比べ、次を確かめる。
    -   変えたファイルが AC-19 の範囲にある。
    -   Go のファイルの差分が、コメントの行に限る（AC-11）。
-   **記録した diff の読み取り（`diff`）:** 各 PR の ADR の差分（AC-14）と、`CLAUDE.md`（AC-17）、`pricing_update.md`（AC-18）の差分である。どれも数行なので、スクリプトにしない（`docs/dev/conventions/requirements_process.md`「`diff` verification」）。
-   **読んで確かめる（`manual`）:** 冒頭の規則の内容（AC-01〜AC-03）、各行の中身（AC-06）、詳細の行き先（AC-09・AC-10）、PR の説明の一覧（AC-12）、ADR の参照が行の内容と合うこと（AC-13）、ADR-0044 の内容（AC-15）、ずれの記述の扱い（AC-16）、`CLAUDE.md` と `pricing_update.md` の文の内容（AC-17・AC-18）。

ADR-0044 は本書の 3 文書の PR に入るので、AC-15 はその PR のレビューと CI で確かめる。

## 8. 実装の優先順位

実装の PR は 2 本である。

| PR | 内容 |
|---|---|
| PR-1 | 冒頭の規則、表を 6 つに分けること（全行を新しい節へ移す）、パイプラインの構成要素・LLM・共有部品の行を絞ること、3.3 節の PR-1 の ADR の訂正、ADR-0044 のずれの記述の書き換え、`pricing_update.md` の直し |
| PR-2 | サーバとホスト・開発ツール・テスト専用の行を絞ること、3.3 節の PR-2 の ADR の訂正、ADR-0044 のずれの記述の削除、`CLAUDE.md` の直し、文書の大きさの確認（AC-08） |

PR-1 では、PR-2 で絞る節の行は、新しい節へ内容を変えずに移す。

## 9. 将来の拡張性

新しい種類のパッケージ（たとえば WASM の入口）が増えたときは、種類を足すか、既存の種類の定義を広げる。どちらにするかは、そのパッケージを足すタスクが決める。
