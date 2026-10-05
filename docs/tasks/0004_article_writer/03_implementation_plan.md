# 実装計画書：ArticleWriter とプロンプトテンプレート

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-04 |
| Review date | - |
| Reviewer | - |
| Comments | 2026-10-05 PR 構成の変更（再承認が必要。2026-10-05 に isseis が承認した版からの変更）: 要件 3.2・AC-32 と architecture §1.1・§3.7・§3.8・§3.9・§7.1 の改訂と、本計画のフェーズ 4 の記述の改訂を、コードを含まない PR-3（#71）として先にマージすることにした。実装とその設計の変更を分けてレビューするためである。Markdown の判定の実装（フェーズ 4）は PR-4 に、文書の照合（フェーズ 5）は PR-5 に繰り下げた。実装を PR-4 に移したので、フェーズ 4 のステップのチェックボックスを未完了に戻した。2026-10-05 要件 3.2・AC-32 と architecture §3.7・§7.1 に、統一の後もなお U+FEFF で始まる本文の部分を拒否する規則が加わったので、ステップ 4-2・4-4・4-6 にその実装・テスト・壊して失敗することの確認を加えた（PR #71 のレビュー指摘）。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md`（以下「requirements」）で定義した `ArticleWriter` とプロンプトテンプレートを、`02_architecture.md`（以下「architecture」）の設計どおりに実装する。具体的には、typed nil の判定を新設の `internal/nilcheck` へ移し、`internal/transcript` に `NormalizedVideoURL` を加え、`writer.Article` に `ModelVersion` を加える。そのうえで、既定のテンプレートを埋め込む `prompts` パッケージと、`internal/writer` の `New`・`Write`（テンプレートの読み込みと検査、`Transcript` の検証、プロンプトの展開、生成テキストの検証、出典ブロックの付与）を実装する。

本計画は architecture §8 の実装優先順位（共有部品 → テンプレート → 記事の生成 → Markdown の判定 → 文書）をそのままフェーズ 1〜5 とする。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。特に、`ArticleWriter` は `New` を通してしか作れないこと（原則 1）、テンプレートの誤りを構築時に見つけること（原則 2）、構文を許可リストで絞り、どの分岐にも当たらない構文を拒否すること（原則 3）、LLM の出力を補正せずに拒否すること（原則 4）、Markdown の判定を拒否の側へ倒すこと（原則 5）を守る。
- 本番コードで新設・変更するファイルは architecture §3.12 の一覧に限る。テストのヘルパーとして `internal/writer/test_helpers.go` を加える（§4.4）。
- `internal/writer` のユニットテストは同じパッケージ（`package writer`）に置き、先頭に `//go:build test` を付ける（`internal/transcript/video_id_test.go:1`・`internal/pipeline/pipeline_test.go:1` と同じ）。`internal/nilcheck/nilcheck_test.go` も同様とする。
- Go のコメント・識別子・文字列リテラルは英語で書く。テンプレートの文面（`prompts/*.tmpl`）と `prompts/README.md` は日本語で書く（requirements 4.5）。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- **英語の規則の例外（2 種類）。** (1) 出典ブロックの見出し語 `出典: ` は architecture §3.10 が固定した記事の出力の内容であり、本番コードの文字列リテラルとして日本語で書く。この定数 1 つだけを例外とし、doc コメントに出力の内容である旨を英語で書く。(2) テストの入力（`# タイトル\n本文`・`<東京>`・全角空白など）は requirements の AC の例を写したもので、日本語の文字列リテラルとして書く。テストの識別子・コメント・失敗メッセージは英語で書く。どちらも CLAUDE.md の規則（Go の文字列リテラルは英語）と食い違うので、本計画のレビューで承認を受ける。
- 上限値・出典ブロックの見出し語・区切りなどは名前付き定数にする（`mnd`・`goconst`。値は architecture §3.10）。
- 各フェーズの完了条件は `make fmt` → `make test` → `make lint` が通ること。各テストは、対象の分岐を実際に壊して失敗することを確認し、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。

### 1.3. 既存コード調査結果

HEAD `4194276`（ブランチ `issei/0004-article-writer-02`）で確認した。追跡対象のファイルの未コミットの変更は、本計画の作成時に行った `02_architecture.md` への編集上の修正（本節の最後の項目に記す）だけである。ベースラインの `go test -tags test ./...` は全パッケージが `ok` または `[no test files]` で成功した（2026-10-04、HEAD `4194276`。`internal/writer` は `[no test files]`）。architecture §1.3 が HEAD `1d39443` で記した行番号は、以下で挙げるものについて HEAD `4194276` でも同じである。

- **`internal/writer`。** `writer.go:11-17` の `Article` は 4 フィールド、`:19-24` の `ArticleWriter` の doc コメントは `must return an error on failure` と `must not return an empty Article as a successful result` を含む。この 2 つの条項は `internal/pipeline/pipeline_test.go:449` の `TestInterfaceDocComments` が固定しているので、doc コメントを書き換えるときも残す。`internal/writer` にテストファイルはない。
- **`Article` を組み立てる既存コード。** `rg -n "Article\{" -g '*.go' internal` の結果（`Article{}` のゼロ値を除く）は、`internal/writer/testutil/mocks_test.go:16`、`internal/publisher/testutil/mocks_test.go:17`、`internal/pipeline/pipeline_test.go:68` の 3 か所で、いずれもフィールド名付きまたは型名だけの組み立てである（2026-10-04、HEAD `4194276`）。`ModelVersion` を加えても変更は要らない。
- **`isTypedNil`。** `rg -n "isTypedNil" -g '!docs/tasks/**' .` の結果は `internal/pipeline/pipeline.go:131`（`checkStage` の呼び出し）・`:137`（doc コメント）・`:138`（定義）の 3 件だけで、テストからの直接の参照はない（2026-10-04、HEAD `4194276`）。`pipeline.go` で `reflect` を使うのは `isTypedNil` だけなので、移動の後は import から外れる。typed nil の拒否は `pipeline_test.go:279` の `TestPipelineNewNilStage` が段階ごとのポインタの typed nil で確かめている。テストを削除しないので、網羅率の比較（CLAUDE.md「Deleting a test」）は要らない。
- **正規化した URL の組み立て。** テストを除く Go のコードで `"https://www.youtube.com/watch?v=" +` を含むのは `internal/transcript/video_id.go:58` の 1 か所だけである（`rg -n 'watch\?v=" \+' -g '*.go' -g '!*_test.go' internal`、2026-10-04、HEAD `4194276`）。`videoIDPattern` を直接使う `cache.go:405`・`test_helpers.go:282` は URL を組み立てないので変更しない（architecture §3.11）。`TestValidateVideoURL`（`video_id_test.go:10`）は変更せずに通す。
- **`internal/pipeline/pipeline_test.go` の guard。** `TestCommonTypesFieldSets`（`:375`。`Article` の期待値は `:403-408`）を更新する。`TestFakesCarryBuildTag`（`:512`）は `../*/testutil/*.go` を 8 件に固定している。本計画は `testutil/` にファイルを足さないので、この guard は変わらない。
- **fake。** `internal/llm/testutil/mocks.go:15-33` の `FakeLLMClient`（`package llmtestutil`）は `Ctx` と `Request` を記録し、設定した `Result` と `Err` を同時に返せる。AC-14 の「空でない `Text` とエラーを同時に返す」形もこれで作れる。変更しない。
- **root での実行の扱い。** `internal/transcript/test_helpers.go:304-312` の `requireNonRoot` は、root での実行を失敗にする非公開のヘルパーである。`internal/writer` からは使えない（非公開・別パッケージ）。`test_organization.md` の原則では、公開の API だけを使うヘルパーは `testutil/` に置く。また、`internal/transcript` と同じ関数を `internal/writer` に置くことは DRY にも反する。それでも本計画は `internal/writer/test_helpers.go` に同じ数行の関数を置く（I-01）。`testutil/` に置くと `TestFakesCarryBuildTag` の件数が変わり、`testutil/` は fake の置き場所という現在の使い方からも外れるためである。この判断も本計画のレビューで承認を受ける。
- **名前付きパイプ（FIFO）の作成。** 既存のテストに FIFO を作るものはない。`syscall.Mkfifo(path string, mode uint32) error` は darwin と linux にあり、windows にはない（`go doc syscall.Mkfifo` を `GOOS=linux`・既定（darwin）・`GOOS=windows` で実行して確認。go1.27.1）。requirements 4.4 の対象は macOS と Linux で、CI は `ubuntu-latest` だけである（`.github/workflows/ci.yml:14`・`:47`・`:68`）。`syscall.O_NONBLOCK`・`syscall.O_NOCTTY` は darwin・linux・windows のいずれでも定義されている（同じく `go doc` で確認）。
- **lint の前提。** `.golangci.yml` は `gosec`・`err113`・`goconst`・`mnd`・`gocyclo`（`min-complexity: 20`）を有効にし、`_test.go` に限って `gosec`・`err113`・`goconst`・`gocyclo`・`errcheck`・`dupl` を除外している。`test_helpers.go` は `_test.go` でないので除外が効かない。上書きファイルを変数のパスで開く `os.OpenFile` は `gosec` の G304 に当たりうる。`make lint` は `--build-tags test,integration` で解析する（`Makefile:10`）。
- **`deadcode`・`depguard`。** architecture §2.1 のとおり、`make deadcode` は `./cmd/yt2column` から読み込めるパッケージだけを解析し、現在の `main` は何も import しない。`prompts`・`internal/nilcheck` は `depguard` の `deps` ルール（`github.com/isseis/yt2column` の下を許可）で通る。`.golangci.yml`・`Makefile` は変更しない。
- **ドキュメントの更新対象。** `docs/dev/project_overview.md:31`（`Article` の項目の列挙）・`:73-84`（想定ディレクトリ構成）、`docs/dev/developer_guide/package_reference.md:3-4`（対象の説明）・表の `internal/transcript`・`internal/writer` の行。英語版の文書（`*.en.md`）は存在しない（`find docs -name '*.en.md'` の結果が空。2026-10-04）。`CLAUDE.md` の Architecture Overview の `internal/writer` の説明（provider-independent prompt assembly and output post-processing）は本タスクの後も正しいので変更しない。
- **`internal/pipeline` の `package_reference.md` の行。** 行の記述（`Pipeline`・`Stage`・`StageError`・`ErrNilStage`）は `isTypedNil` に触れていないので、移動の後も正しい。architecture §3.12 の記述は、編集上の修正で「`internal/pipeline` の行は変わらないことを確かめる」にした（ステップ 1-8）。
- **具体型の構築の経路。** `templateWriter`（仮称）は `internal/writer` の非公開の型で、本番コードで値を作るのは `New` だけとする。この型で分岐する処理はないので、型をリーフのパッケージへ移すことはしない。テストも具体型の値を直接作らず、`New` を通す。構文の検査は `text/template/parse` のノードの具体型で分岐するが、ノードは標準ライブラリの解析だけが作り、本計画のコードは作らない。未知の種類は `default` で拒否する（ステップ 2-4）。
- **architecture との整合。** 調査の範囲で、architecture の決定を変える必要のある不整合は見つかっていない。`package_reference.md` を更新する時期だけは、同ファイルの規則（パッケージを追加・変更するコミットで更新する）と architecture §8 の手順 5 が食い違っていたため、architecture §8 を編集上の修正として各手順に分けた。また、AC-01・AC-03・AC-04・AC-30 は `Write` が `LLMClient` に渡すプロンプトで確かめるため、それらのテストを architecture §8 の手順 2 から手順 3 へ移した（同じく編集上の修正）。いずれも同書の Comments に記録した。

### 1.4. 名前の変更・移動の一覧

| 変更前 | 変更後 |
|---|---|
| `internal/pipeline` の非公開の `isTypedNil(v any) bool` と、`checkStage` の `stage == nil \|\| isTypedNil(stage)` | `internal/nilcheck` の `IsNil(v any) bool`（nil の interface 値も真）。`checkStage` は `nilcheck.IsNil(stage)` を使う |
| `internal/transcript/video_id.go:55-58` の、動画 ID の検査と正規化した URL の組み立て | `transcript.NormalizedVideoURL(videoID string) (string, bool)`。`validateVideoURL` はこれを使う |

### 1.5. implementation_handoff.md の項目への対応

| ID | 対応 |
|---|---|
| I-01 | 各方法を次のステップで扱う。**fake と一時ファイル**: `llmtestutil.FakeLLMClient` を使い、上書きファイルは `t.TempDir` の下に作る（ステップ 2-7・2-8・3-5〜3-7）。**目印の文字列**: 値ごとに互いに異なる目印を `test_helpers.go` の定数にし、AC-10・AC-26・AC-30 で使う。AC-26 では、各拒否ケースの入力の形が許す限りタイトル・本文の部分・`Model`・`ModelVersion` のすべてに目印を置き、共有のアサーションでエラーの文字列に現れないことを確かめる（ステップ 3-4・3-7）。**出力の形の指示**: 既定の system テンプレートに `# ` で始まる行があることを文字列で確かめる（ステップ 2-8）。**タイムスタンプを含めないこと**: 目印の数値が現れないことに加えて、`StartMs` だけが異なる 2 つの `Transcript` のプロンプトが同一であることを確かめる（ステップ 3-5）。**読めない上書きファイル**: 権限のない通常のファイルを含め、root での実行は `requireNonRoot` で失敗させる。`requireNonRoot` は `internal/writer/test_helpers.go` に同じ関数を置く（§1.3）。**名前付きパイプ**: 構築を別の goroutine で呼び、名前付き定数の時間内に戻らなければ失敗として報告し、書き込み側を開いて goroutine を解放する（ステップ 2-8）。**失敗時の呼び出し回数**: AC-14 のテストで `Generate` がちょうど 1 回であることを確かめる（ステップ 3-6）。 |
| I-02 | `TestCommonTypesFieldSets` の `Article` の期待値に `ModelVersion`（`string`）を加え、同じコミットで `project_overview.md:31` を更新する（ステップ 1-6・1-7）。 |

design_handoff.md の H-01〜H-11 は、すべて architecture §3.13 に対応が記録されている。

## 2. 実装ステップ (Implementation Steps)

ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名と AC の対応は §5 にまとめ、各ステップでは対象の AC だけを示す。テスト関数名は計画上の名前であり、実装で変える場合は §5 を同じコミットで更新する。

### フェーズ 1: 共有部品

**対象ファイル**
- 新設: `internal/nilcheck/nilcheck.go`・`internal/nilcheck/nilcheck_test.go`（`//go:build test`）
- 変更: `internal/pipeline/pipeline.go`・`internal/transcript/video_id.go`・`internal/transcript/video_id_test.go`・`internal/writer/writer.go`・`internal/pipeline/pipeline_test.go`・`docs/dev/project_overview.md`・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 1-1**: `nilcheck.go` に `IsNil`（architecture §3.11）を作る。判定は `v == nil` と、`isTypedNil` の `reflect` の判定（`Pointer`・`Func`・`Map`・`Slice`・`Chan`）を合わせたものとする。import は標準ライブラリだけとし、パッケージの doc コメントを書く。
- [x] **ステップ 1-2**: `nilcheck_test.go` に `TestIsNil` を作る。nil の interface 値、5 種類それぞれの nil と nil でない値、nil になりえない値（整数のゼロ値・構造体）を表のテストで確かめる。
- [x] **ステップ 1-3**: `pipeline.go` の `checkStage` を `nilcheck.IsNil(stage)` で判定するように変え、`isTypedNil` とその doc コメント、`reflect` の import を削除する。`ErrNilStage` と段階名のエラーは変えない。`TestPipelineNewNilStage` を変更せずに通す。
- [x] **ステップ 1-4**: `video_id.go` に `NormalizedVideoURL`（architecture §3.11）を追加し、`validateVideoURL` の `:55-58` をこの関数の呼び出しに置き換える。失敗時のエラーは変えない（`fmt.Errorf("%w: video ID must be 11 characters of [A-Za-z0-9_-]", ErrInvalidVideoURL)` のまま）。
- [x] **ステップ 1-5**: `video_id_test.go` に `TestNormalizedVideoURL` を追加する。11 文字の有効な ID の受理と返る URL、空・10 文字・12 文字・`/` や `..` を含む・許されない記号・非 ASCII・末尾の改行の拒否を確かめる。`TestValidateVideoURL` は変更しない。
- [x] **ステップ 1-6**: `writer.go` の `Article` に `ModelVersion string` を加え、doc コメントを architecture §3.1 のとおりにする。`pipeline_test.go` の `TestCommonTypesFieldSets` の `Article` の期待値に `"ModelVersion": "string"` を加える。
- [x] **ステップ 1-7**: `project_overview.md` を更新する。
  - `:31` の変更前: `` - `ArticleWriter`: プロバイダに依存しない。プロンプトテンプレートにタイムスタンプを除いた本文とメタ情報を埋め込み、`LLMClient` を呼び出して、結果を `Article`（タイトル・Markdown 本文・出典 URL・生成モデル名）に変換する。 ``
  - 変更後: `` - `ArticleWriter`: プロバイダに依存しない。プロンプトテンプレートにタイムスタンプを除いた本文とメタ情報を埋め込み、`LLMClient` を呼び出して、結果を `Article`（タイトル・Markdown 本文・出典 URL・生成モデル名・モデルの版の識別子）に変換する。 ``
  - 想定ディレクトリ構成の `internal/strictjson/` の行の次に、`internal/nilcheck/        # typed nil を含む nil の判定` を加える（`#` の位置は前後の行にそろえる）。
- [x] **ステップ 1-8**: `package_reference.md` に `internal/nilcheck` の行（nil の interface 値と typed nil の判定。`internal/pipeline` が使う）を加え、`internal/transcript` の行に `NormalizedVideoURL`（動画 ID を検証し、正規化した URL を組み立てる）を加える。`internal/writer` が使うことは、使い始めるステップ 2-9・3-8 で書き加える。`internal/pipeline` の行は変更しない（§1.3）。
- [x] **ステップ 1-9**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `IsNil` の `Pointer`・`Func`・`Map`・`Slice`・`Chan` の場合を 1 つずつ外す（`TestIsNil`）、`IsNil` の `v == nil` を外す（`TestIsNil` の nil の interface 値のケース）、`NormalizedVideoURL` の検査を外す（`TestNormalizedVideoURL` と `TestValidateVideoURL` の拒否のケース）、`checkStage` が `IsNil` を呼ばない（`TestPipelineNewNilStage`）。
- [x] **ステップ 1-10**: このフェーズの差分のテストファイルが `nilcheck_test.go`・`video_id_test.go`（追加だけ）・`pipeline_test.go`（`TestCommonTypesFieldSets` の 1 行だけ）であることを、コミット前に差分で確認する。`make fmt` → `make test` → `make lint` を通す。

### PR-1 作成ポイント: shared parts (nil check, normalized video URL, Article.ModelVersion)

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10

**推奨タイトル**: `refactor(0004): extract the shared nil check and normalized video URL, add Article.ModelVersion`

**レビュー観点**: `nilcheck.IsNil` が `isTypedNil` の振る舞い（nil の interface 値と 5 種類の typed nil）を保ち、`TestPipelineNewNilStage` が変更なしで通ること（ステップ 1-1〜1-3） / `NormalizedVideoURL` の切り出しで `validateVideoURL` の返す番兵と受理・拒否の境界が変わらず、`TestValidateVideoURL` が変更なしで通ること（ステップ 1-4・1-5） / `Article.ModelVersion` の追加に合わせて `TestCommonTypesFieldSets` と `project_overview.md` が同じコミットで更新されていること（ステップ 1-6・1-7） / `package_reference.md` の `internal/nilcheck`・`internal/transcript` の行が実装と一致し、`internal/pipeline` の行が変わっていないこと（ステップ 1-8）

**実装モデル要件**: standard

**判定理由**: 既存テストで振る舞いの保存を確かめられる純粋なリファクタリングと型・文書の追加に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・Conditional checks のいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: テンプレートと構築

**対象ファイル**
- 新設: `prompts/prompts.go`・`prompts/system.tmpl`・`prompts/user.tmpl`・`prompts/README.md`
- 新設: `internal/writer/errors.go`・`internal/writer/template.go`・`internal/writer/prompt.go`（`templateData` と定数 `maxPromptBytes` の定義だけ）
- 新設: `internal/writer/test_helpers.go`（`//go:build test`）・`internal/writer/writer_test.go`・`internal/writer/template_test.go`（いずれも `//go:build test`）
- 変更: `internal/writer/writer.go`・`internal/pipeline/pipeline_test.go`（`TestPromptsREADMEMatchesContract` を加える）・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 2-1**: `prompts.go` に、`//go:embed` で 2 つのテンプレートを非公開の `string` 変数に埋め込み、`System`・`User` で返す実装を作る（architecture §3.2）。パッケージ変数を公開せず、検査もしない。
- [x] **ステップ 2-2**: `system.tmpl`・`user.tmpl` に、architecture §3.2「仮のテンプレートの内容」を満たす仮の文面を日本語で書く。system には `# ` で始まる行の例を含め、user は 4 つの値を見出し付きの区画に埋め込む。許可リスト（architecture §3.4）にない構文を使わない。
- [x] **ステップ 2-3**: `errors.go` に公開の番兵 3 つ（architecture §4.1）と、nil の `LLMClient` を表す非公開の静的エラー `errNilLLMClient` を置く。ステップ 2-4 の非公開の静的エラー（`errNotRegularFile`、`define`・`block` の拒否を表す `errTemplateDefinition`、許可リストにない構文の拒否を表す `errDisallowedSyntax`）もここに置く。後の 2 つは `ErrInvalidTemplate` に加えてつなぎ、テストがどの検査による拒否かを区別できるようにする（実装時に追加）。
- [x] **ステップ 2-4**: `template.go` に、上書きファイルの読み込み（architecture §3.3）、テンプレートの検査の手順 1〜6（同 §3.4）、定数 `maxTemplateBytes` を実装する。
  - 開くフラグは `os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY` とし、開いたファイルの `Stat` で通常のファイルかを確かめ、`io.LimitReader` で上限 + 1 バイトまで読む。開く・`Stat`・読み取りの失敗は `ErrInvalidTemplate` と `os` のエラーの両方を `%w` でつなぐ（architecture §4.1）。通常のファイルでないことによる拒否は、`ErrInvalidTemplate` に加えて非公開の静的エラー（`errNotRegularFile`）もつなぎ、テストが種類の確認による拒否を読み取りの失敗と区別できるようにする。
  - テンプレートデータ `templateData`（architecture §3.2）と定数 `maxPromptBytes`（同 §3.10）をこのフェーズで `prompt.go` に定義する。構文の検査で参照を許す名前は `templateData` のフィールドから得る。許可する組み込み関数は 1 つの非公開の値（集合）に定め、検査とテストの両方がそれを使う。
  - `gosec` が `os.OpenFile` を G304 として指摘した場合は、その 1 行に限った `//nolint:gosec` と、利用者が自分で指定した上書きファイルを読み取り用に開くだけである旨の英語のコメントを付ける。ファイル・パッケージ単位の抑制はしない。
  - 構文の検査は解析木のノードの種類で分岐する。許可する種類だけを `case` に挙げ、拒否する種類（`with`・`range`・`template`・`break`・`continue`・変数・`.` そのものなど）は個別の `case` に挙げずに `default` で拒否する（architecture §7.3 の fail-secure の分岐）。`gocyclo` の上限（20）を超えないよう、ノードの種類ごとの判定を関数に分ける。
  - 同じ名前の `{{define}}` も検出する方法を選ぶ（architecture §3.4 の手順 5）。採用した方法: テンプレートの名前で解析した結果のテンプレートの数が 1 であることに加え、別の名前（テンプレートの名前 + `probeNameSuffix`）で解析し直した結果の数も 1 であることを確かめる。テンプレートと同じ名前の `define` は 2 回目の解析で 2 つ目のテンプレートになり、2 回目の解析の名前の `define` は 1 回目の解析で 2 つ目のテンプレートになる。
  - 開いたファイルの `Stat` と読み取りは、`*os.File` の一部（`io.Reader` と `Stat`）を表す非公開の interface を受け取る関数 `readOpenedFile` に分ける。通常のファイルでは `Stat` と読み取りの失敗を起こせないため、テストはこの関数に fake のファイルを渡して各失敗を実行する（ステップ 2-8）。
  - エラーの文言の内容は architecture §4.2 のとおりとする。
  - 関数の引数の数とコマンドの形を検査する（architecture §3.4 の表。実装時のレビューを受けた設計の変更）。許可する関数の集合 `allowedFuncs` は関数ごとの引数の数の範囲を持つ。関数名はコマンドの先頭でだけ受理し、パイプラインで前から渡る値も引数として数える。関数名以外で始まるコマンドは、引数も前から渡る値もとらない。この検査はセキュリティ上の要件ではなく、誤りを構築時に報告するための利便性の検査である（architecture §3.4）。
- [x] **ステップ 2-5**: `writer.go` に `Options`・`New`・非公開の具体型（architecture §3.1・§6.1）を実装する。`New` は `nilcheck.IsNil` で `client` を拒否し、パスが空なら `prompts.System()`・`prompts.User()`、空でなければ上書きファイルの内容に、ステップ 2-4 の同じ検査を行う。具体型の `Write` は、フェーズ 3 のステップ 3-3 で置き換えるまでの暫定の実装とし、常にゼロ値の `Article` と非公開の静的エラーを返す（引数は `_` で受ける）。`unparam` などが指摘した場合は、その関数に限った `//nolint` と「ステップ 3-3 で置き換える暫定の実装」である旨の英語のコメントを付ける。
- [x] **ステップ 2-6**: `prompts/README.md`（日本語）に、上書きファイルを書く利用者向けの仕様を書く。参照できる 4 つの名前、使える構文と使えない構文の例、テンプレートとプロンプトの上限（バイト数）、空白文字だけのテンプレートを拒否すること、テンプレートに秘密情報を書かないこと（architecture §5.2）を含める。
- [x] **ステップ 2-7**: `test_helpers.go` を作り、ステップ 2-8 のテストが使うものだけを置く。上書きファイルを `t.TempDir` の下に書くヘルパー、`requireNonRoot`（§1.3）、system・user の片方だけを上書きする `Options` の一覧 `overrideTargets`（実装時に追加）である（検証を通る `Transcript` と目印の定数は、最初に使うステップ 3-4 で加える。`unused` が使われていない関数を報告するため）。`errcheck`・`goconst` などのテスト向けの除外が効かないので、エラーを無視しない。ファイルの書き込みを `gosec` が指摘した場合は、`internal/transcript/test_helpers.go:154` と同じく 1 行に限った `//nolint:gosec` と理由の英語のコメントを付ける。
- [x] **ステップ 2-8**: テストを作る（AC-02・AC-05・AC-06・AC-30 の後半）。構築の拒否のケースでは、いずれも返る `ArticleWriter` が nil であることも確かめる。
  - `writer_test.go`: `TestNewRejectsNilClient`（nil と typed nil の `*llmtestutil.FakeLLMClient`。エラーと nil の `ArticleWriter`）。
  - `template_test.go`: `TestNewRejectsInvalidOverrideFile`（AC-05 のファイルの各ケースと、権限のない通常のファイル。§4.1 の義務に従い、開く・`Stat`・読み取りの各失敗を独立に実行し、下にある OS のエラーが `errors.Is` で取り出せることを、存在しないファイルの `fs.ErrNotExist` に限らず確かめる。開く失敗は `New` で（存在しないファイルの `fs.ErrNotExist` と権限のないファイルの `fs.ErrPermission`）、`Stat` と読み取りの失敗は `readOpenedFile` に fake のファイルを渡して（`syscall.EIO`）確かめる。シンボリックリンクの先が通常のファイルなら受理し、ディレクトリなら拒否する（architecture §3.3）。ディレクトリとシンボリックリンクの先のディレクトリでは `errNotRegularFile` も確かめる）、`TestNewOverrideFileFIFO`（書き込み側を開かない FIFO。I-01 の時間切れの扱いと、`errNotRegularFile`。時間切れのときは書き込み側を `O_WRONLY|O_NONBLOCK` で開いて閉じ、構築の goroutine を解放する）、`TestNewOverrideFileSizeLimit`（ちょうど上限の受理と上限 + 1 バイトの拒否）、`TestTemplateSyntaxAllowlist`（architecture §3.4 の表の各行の許可と拒否の例、分岐・`else`・括弧・パイプの中の拒否、`{{"\xff"}}`、別の名前の `define`（本体は許可する構文だけ）と `{{block "x" .}}{{end}}`、各関数の引数の数の境界（少なすぎる・多すぎる・パイプラインで渡る値を数える形）、関数でない値への引数、パイプラインの 2 段目以降の関数でない値、引数の位置の関数名、同じ名前の `define` のうち、解析が成功する 2 つの形（本体が空で `define` の中身が空でない形と、本体が空でなく `define` の中身が空の形）と、2 回目の解析の名前（ステップ 2-4）の `define` の同じ 2 つの形。これらの `define` を含む入力が、`define` 以外の検査を通ることも確かめる。また、拒否の例はすべて、system と user のどちらの上書きでも拒否されることを確かめる）、`TestDefaultTemplatesPassChecks`（`New(client, Options{})` が成功し、`prompts.System()`・`prompts.User()` の実物がステップ 2-4 の検査を通る。AC-06 のとおり両方が同じ検査を受けるので、§4.1 の義務に従い system・user のそれぞれを独立に検査していることを、どちらか一方だけを壊しても失敗することで確かめる）、`TestDefaultSystemTemplateHeadingInstruction`（`prompts.System()` の実物が `# ` で始まる行を含む）、`TestReadOpenedFileBound`（上限を大きく超えるファイルでも上限 + 1 バイトまでしか読まない。実装時に追加）、`TestInvalidTemplateErrorNamesSource`（拒否のエラーが、どちらのテンプレートか、既定のものか上書きファイルか、上書きファイルのパスを含む。architecture §4.2。実装時に追加）。
  - `internal/pipeline/pipeline_test.go`: `TestPromptsREADMEMatchesContract` を加える（クロスパッケージの静的 guard。`prompts/README.md` が `internal/writer` の実際の契約と同期していることを確かめる。対象は 4 つのテンプレートデータの名前、許可する組み込み関数、2 つの上限値である。契約は `internal/writer` のソース（AST・ソースの解析など）から導き、値を写した一覧には依存しない。`internal/writer` の非公開の識別子は他パッケージから `reflect` では参照できないため、その方法は採らない。`internal/pipeline/pipeline_test.go` の `TestInterfaceDocComments` と同じ形の guard とし、`internal/writer` のユニットテストをテストの外のファイルに依存させない）。
- [x] **ステップ 2-9**: `package_reference.md` を更新する。
  - `:3-4` の 2 行にわたる文の変更前: ``This document lists the packages under `cmd/` and `internal/` and the responsibility of each.``
  - 変更後: ``This document lists the packages under `cmd/` and `internal/`, plus the `prompts` package at the repository root, with the responsibility of each.``（改行の位置は前後の段落にそろえる）
  - `prompts` の行（既定のプロンプトテンプレートを埋め込み、`System`・`User` で返す）を加え、`internal/writer` の行に `Options`・`New`（`LLMClient` とテンプレートの検査）を加える。`internal/nilcheck` の行に `internal/writer` も使うことを加える。
- [x] **ステップ 2-10**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `Stat` による種類の確認を外す（`TestNewRejectsInvalidOverrideFile` のディレクトリと `TestNewOverrideFileFIFO` の `errNotRegularFile` の確認）、`O_NONBLOCK` を外す（`TestNewOverrideFileFIFO` が時間切れになる）、上限の比較を 1 ずらす（`TestNewOverrideFileSizeLimit`）、`%w` で `os` のエラーをつながない（`TestNewRejectsInvalidOverrideFile` の開く・`Stat`・読み取りの各失敗。存在しないファイルに限らない）、`default` を許可にする（`TestTemplateSyntaxAllowlist` の `with`・`range`・変数の行）、`if` の条件・`else` の中を検査しない（同、分岐の中の行）、同じ名前の `define` の検出を外す（同、同じ名前の 2 つの形）、1 回目の解析のテンプレートの数の確認を外す（同、2 回目の解析の名前の `define` の 2 つの形）、2 回目の解析を外す（同、同じ名前の `define` の 2 つの形。上の「同じ名前の `define` の検出を外す」と同じ変更）、`PipeNode` の変数の宣言を検査しない（同、`{{$x := .Title}}`）、フィールドの参照で 2 つ目以降の名前を検査しない（同、`.Title.Foo`）、`ChainNode`・`NilNode` を許可する（同、`(.Title).Foo`・`nil`）、文字列の定数の UTF-8 の検査を外す（同、`{{"\xff"}}`）、`printf` を許可する（同）、既定の system・user のテンプレートをそれぞれ 1 つずつ壊し（例: 許可リストにない構文を足す）、AC-06 の検査が両方を独立に実行していることを確かめる（`TestDefaultTemplatesPassChecks`）、`system.tmpl` から `# ` の行を消す（`TestDefaultSystemTemplateHeadingInstruction`）、README の記述を実際の契約からずらす（例: テンプレートデータの名前・許可する関数・上限のいずれかを変える）（`TestPromptsREADMEMatchesContract`）、引数の数の検査（下限・上限・可変長の扱い）を外す・パイプラインで渡る値を数えない・関数でない値への引数やパイプラインの 2 段目以降の値を許す・引数の位置の関数名を許す（`TestTemplateSyntaxAllowlist` の対応する行）、`New` の nil の確認を外す（`TestNewRejectsNilClient`）、エラーの文言からパスや既定のものの表示を外す（`TestInvalidTemplateErrorNamesSource`）。
- [x] **ステップ 2-11**: `make fmt` → `make test` → `make lint` を通す。

AC-01・AC-03・AC-04・AC-30（目印が `LLMClient` に渡ること）は `Write` が `LLMClient` に渡すプロンプトで確かめるので、そのテストはフェーズ 3 のステップ 3-6 で作る（architecture §8 の手順 2・3 もこの割り当てに合わせた。§1.3）。

**PR の区切りへの制約。** 常にエラーを返す暫定の `Write`（ステップ 2-5）を `main` ブランチに入れないため、フェーズ 2 とステップ 3-3 は同じ PR に含める（`0003_deepseek_llm_client/03_implementation_plan.md` のステップ 3-4 と同じ扱い）。ステップ 3-3 はステップ 3-1・3-2 の検証関数に依存し、そのテスト（ステップ 3-4〜3-7）は実装と同じ PR に置く（`mkplan2.md` の Buildability の原則「Never split a tightly coupled unit (interface + implementation + test) across PRs」）。そのためフェーズ 2 とフェーズ 3 を 1 つの PR-2 にまとめる（§3.2）。

### フェーズ 3: 記事の生成（Markdown の判定を除く）

**対象ファイル**
- 新設: `internal/writer/output.go`
- 新設: `internal/writer/prompt_test.go`・`internal/writer/output_test.go`（いずれも `//go:build test`）
- 変更: `internal/writer/prompt.go`・`internal/writer/writer.go`・`internal/writer/errors.go`・`internal/writer/test_helpers.go`・`internal/writer/writer_test.go`・`internal/pipeline/pipeline_test.go`（`TestWriterImports` を加える）・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 3-1**: `prompt.go` に、`Transcript` の検証（architecture §3.5）、`Transcript` から `templateData` を作る処理と字幕本文の連結（同 §3.2・§3.5）、上限付きの書き込み先と展開（同 §3.6）、定数 `maxPromptBytes` を実装する（フェーズ 2 で `maxPromptBytes` に付けた `//nolint:unused` を外す）。上限付きの書き込み先は、累計が上限を超える書き込みを受け取ったら、受け取ったバイト列を保持せずに非公開の静的エラー `errPromptTooLarge`（`errors.go` に置く）を返す。展開時のエラー・上限の超過・空白文字だけの展開結果は `ErrInvalidTemplate` とし、ラップの方法とエラーの文言は architecture §4.1・§4.2 のとおりとする。`ErrInvalidTranscript` のエラーに `Transcript` の値を含めない。
- [x] **ステップ 3-2**: `output.go` に、生成テキスト（`GenerateResponse.Text`。先頭行のタイトルと、その後の本文の部分からなる）の検証（architecture §3.7 の手順 1〜5・7・8）、タイトル・`Model`・`ModelVersion` が共有する必須と制御文字の判定、出典ブロックと `Article` の組み立て（同 §3.9）、定数 `maxTextBytes`・`maxModelBytes`・出典ブロックの見出し語・区切りを実装する。手順 6（Markdown の判定）はフェーズ 4 で手順 5 と手順 7 の間に加える。`ErrMalformedOutput` のエラーには値を含めない（architecture §4.2）。
- [x] **ステップ 3-3**: `writer.go` の暫定の `Write`（ステップ 2-5）を、architecture §6.2 の順（`ctx` の終了の確認 → `Transcript` の検証 → system・user の展開 → `Generate` を `MaxOutputTokens: 0` で 1 回 → 生成テキストの検証 → 出典ブロックの付与）の実装に置き換え、暫定の実装の静的エラーと `//nolint` を削除する。`ctx` の終了は `ctx.Err()` を、`LLMClient` のエラーは受け取ったものを、それぞれ `%w` で 1 回ラップし、`ArticleWriter` の番兵ではラップしない（architecture §4.1）。どの失敗でもゼロ値の `Article` を返す。
- [x] **ステップ 3-4**: `test_helpers.go` に、検証を通る `Transcript` を作るヘルパー、値ごとに互いに異なる目印の定数、`ErrMalformedOutput` の拒否を確かめる共有のアサーションを加える。共有のアサーションの内容は、`errors.Is(err, ErrMalformedOutput)` が真、返る `Article` がゼロ値、`err.Error()` にタイトル・本文の部分・`Model`・`ModelVersion` の目印のどれも現れないこと（AC-26）。拒否する値そのもの（例: 制御文字を含む `Model`）にも、形が許す限り目印を含める（I-01）。
- [x] **ステップ 3-5**: `prompt_test.go` を作る（AC-07・AC-08・AC-10〜AC-13・AC-28）。`TestWriteRejectsInvalidTranscript`（AC-07 の各ケース。`Generate` が 0 回、ゼロ値の `Article`、エラーに `Transcript` の目印が現れない。不正な `VideoID` のケースは `VideoURL` を `"https://www.youtube.com/watch?v=" + VideoID` にして、`VideoID` の検査だけが拒否できる入力にする）、`TestWriteAcceptsEmptyMetadata`、`TestWriteEmbedsValues`（目印で囲んで 4 つの値を埋め込むテスト用のテンプレートを system・user の両方に与え、前後の空白・改行を含む `Text` も含める）、`TestWriteOmitsStartMs`（I-01 の 2 つの確かめ方）、`TestWriteDoesNotInterpretValues`、`TestWriteExpansionFailure`（AC-13 の 2 種類。`Generate` が 0 回）、`TestWritePromptSizeLimit`（system・user のそれぞれで、展開結果がちょうど上限なら `Generate` が 1 回、上限 + 1 バイトなら `ErrInvalidTemplate` で 0 回）、`TestBoundedWriter`（上限付きの書き込み先の、ちょうど上限・上限を超える 1 回の書き込み・複数回の書き込みの累計）。`TestWritePromptSizeLimitStopsExpansion`（上限を超える書き込みで展開が止まり、後の記述が実行されないこと。展開の後で大きさを確かめる実装を検出する。実装時のレビューを受けて追加）。
- [x] **ステップ 3-6**: `writer_test.go` にテストを加える（AC-01・AC-03・AC-04・AC-09・AC-14・AC-15・AC-21・AC-30）。
  - `TestNewUsesDefaultTemplates`: `Options{}` で構築した `ArticleWriter` と、`prompts.System()`・`prompts.User()` の内容をそのまま上書きファイルにして構築した `ArticleWriter` が、同じ `Transcript` に対して同じプロンプトを `LLMClient` に渡す（既定の経路が埋め込みの内容を使うことを、展開をテストで書き直さずに確かめる）。
  - `TestNewOverridesEachTemplate`: system 用だけ・user 用だけを上書きした場合に、上書きした側のプロンプトは上書きファイルの展開結果と一致し、上書きしなかった側のプロンプトは `Options{}` で構築したときのプロンプトと一致する。
  - `TestNewReadsOverrideOnce`: 構築後に上書きファイルを書き換えた場合と削除した場合の両方で、`Write` のプロンプトが構築時の内容のまま。
  - `TestDefaultTemplatesEmbedAllValues`: 既定のテンプレートで、4 つの目印がそれぞれ system・user の少なくとも一方に現れる。
  - `TestWriteCallsGenerateOnce`: 成功時に `Generate` が 1 回、`Write` に渡した `ctx`（`context.WithValue` で区別できるようにしたもの）を受け取り、`MaxOutputTokens` が 0。
  - `TestWriteReportsLLMError`: AC-14 の各エラーと、空でない `Text` を伴うエラー。`errors.Is` が真、`ArticleWriter` の 3 つの番兵のどれにも当たらない、ゼロ値の `Article`、`Generate` が 1 回。
  - `TestWriteContextDone`: キャンセル済みと期限切れの `ctx`。§4.1 の判別的な拒否テストの義務に従い、終了した `ctx` を、それだけなら拒否される入力（不正な `Transcript` または展開に失敗するテンプレート）と組み合わせ、architecture §6.2 の順どおり `ctx` の終了の確認が `Transcript` の検証とテンプレートの展開より前に来ることを確かめる。`Generate` が 0 回、ゼロ値の `Article`。
  - `internal/pipeline/pipeline_test.go`: `TestWriterImports` を加える（クロスパッケージの静的 guard）。`internal/writer` と `prompts` の Go ファイル（テストファイルと `test_helpers.go` を含む）の import を調べ、`net`・`net/http` と `internal/llm/` の下のパッケージを import しないことを確かめる（AC-21、requirements 4.5）。`internal/llm/testutil` のような `//go:build test` 付きのパッケージは、テストのファイル（`_test.go` または `//go:build test` のファイル）からの import だけを許し、本番のファイルからの import は拒否する（許すと、通常のビルドが壊れることをテストが検出できなくなる）。あわせて、テストファイルが `os.Open`・`os.ReadFile` などでファイルを開く箇所が `t.TempDir` の下のファイルだけであることを、レビューで確かめる（§4.1）。
- [x] **ステップ 3-7**: `output_test.go` を作る（AC-16〜AC-19・AC-22・AC-23・AC-25・AC-26・AC-29・AC-31）。`TestWriteArticle`（AC-16 の値と、`ModelVersion` が空の応答。見出し・リスト・リンクと前後の空白・改行を含む本文でも、`Body` が LLM が生成した本文・区切り・出典ブロックの連結と完全に一致する。`#  タイトル  \n本文` で `Title` が `タイトル` になる）、`TestWriteSourceBlock`（AC-19 の各ケースと、本文が `\r` で終わる場合。`SourceURL`・`Body` の末尾・直前の空行）、`TestWriteRejectsMalformedText`（AC-22・AC-23 の各例）、`TestWriteRejectsInvalidModel`（AC-25・AC-29 の各例）、`TestWriteOutputSizeLimits`（`Text`・`Model`・`ModelVersion` のそれぞれのちょうど上限と上限 + 1 バイト）。拒否のケースはすべてステップ 3-4 の共有のアサーションを使う。
- [x] **ステップ 3-8**: `package_reference.md` の `internal/writer` の行に、`Write` の責務（`Transcript` の検証、上限付きのプロンプトの展開、`LLMClient` の 1 回の呼び出し、生成テキストの検証、出典ブロックの付与）を加える。
- [x] **ステップ 3-9**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `VideoURL` の一致の検査を外す（`TestWriteRejectsInvalidTranscript` の `VideoURL` のケース）、`NormalizedVideoURL` を使わずに `VideoID` を連結して URL を作る（同、不正な `VideoID` のケース。ステップ 3-5 のとおり、これらのケースの `VideoURL` は連結した URL と一致するので、`VideoID` の検査がなければ受理される）、字幕本文を `StartMs` 付きで連結する（`TestWriteOmitsStartMs`）、展開に上限付きの書き込み先を使わない（`TestWritePromptSizeLimit` の上限 + 1）、上限付きの書き込み先の比較を 1 ずらす（`TestBoundedWriter`・`TestWritePromptSizeLimit`）、空白文字だけの展開結果の検査を外す（`TestWriteExpansionFailure`）、`ctx` の確認を外す（`TestWriteContextDone`）、`LLMClient` のエラーを `%v` でつなぐ（`TestWriteReportsLLMError`）、エラーとともに返った `Text` で記事を作る（同）、出典ブロックの URL を生成テキストから取る（`TestWriteSourceBlock` の別の URL を含むケース）、区切りを `\n` 1 つにする（`TestWriteSourceBlock` の改行で終わらないケース）、タイトルの前後の半角空白とタブを除かない（`TestWriteArticle` の `Title` が `タイトル` になるケース）、制御文字の検査を外す（`TestWriteRejectsMalformedText` の `\r` と `TestWriteRejectsInvalidModel`）、`ModelVersion` に制御文字の検査を適用しない（`TestWriteRejectsInvalidModel` の `ModelVersion` のケース）、`MaxOutputTokens` に 0 以外を渡す・`Generate` に別の `ctx` を渡す（`TestWriteCallsGenerateOnce`）、`maxModelBytes` の比較を 1 ずらす（`TestWriteOutputSizeLimits`）、`ErrMalformedOutput` のエラーに `Model` の値を含める（共有のアサーション）、`maxTextBytes` の比較を 1 ずらす（`TestWriteOutputSizeLimits`）、既定のテンプレートから `.Description` の参照をすべて消す（`TestDefaultTemplatesEmbedAllValues`）、上書きのパスを system と user で取り違える（`TestNewOverridesEachTemplate`）、`New` が上書きファイルを `Write` のたびに読む（`TestNewReadsOverrideOnce`）、`internal/writer` に `net/http` の import を足す（`TestWriterImports`）、本番の `internal/writer` のファイルに `internal/llm/testutil` の import を足す（`TestWriterImports`。テストのファイルからだけ許す規則が効いていることを確かめる）。
- [x] **ステップ 3-10**: `make fmt` → `make test` → `make lint` を通す。

### PR-2 作成ポイント: ArticleWriter templates, construction, and generation

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10 / 2-11 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10

**推奨タイトル**: `feat(0004): add the ArticleWriter templates, construction, and generation`

**レビュー観点**: `prompts` の埋め込みテンプレートと `New` の検査（許可リストの fail-secure な分岐、上限、UTF-8、開いたファイルの種類の確認、`O_NONBLOCK`）が architecture §3.3・§3.4 のとおりで、`gosec` の抑制が 1 行に限られていること（ステップ 2-1〜2-4） / 暫定の `Write`（ステップ 2-5）が同じ PR のステップ 3-3 で置き換えられ、PR の最終状態に残っていないこと（§3.2・§6 のリスク） / `Write` が architecture §6.2 の順（`ctx` → `Transcript` の検証 → system・user の展開 → `Generate` 1 回 → 生成テキストの検証 → 出典ブロック）で進み、Markdown の判定を除く検証と出典ブロックを実装していること（ステップ 3-1〜3-3・3-7） / 判別的な拒否テスト（§4.1）、値をエラーに含めないこと（AC-26）、プロバイダ非依存の import guard、`New` が上書きファイルを構築時に 1 回だけ読みテストがテストの外のファイルを使わないこと（AC-04・AC-21）が満たされていること（ステップ 3-4〜3-7）

**実装モデル要件**: frontier-recommended

**判定理由**: `os.OpenFile` に対する `gosec` の 1 行の抑制（Conditional check の「security-linter-flagged construct」）と、`//go:build test` の `internal/writer/test_helpers.go` という非 `_test.go` のビルドタグ下のソース（Conditional check の「build-tag compiled non-`_test.go` source」）の 2 つに該当し、加えて信頼できないテンプレートの検査と LLM 出力の検証というセキュリティの中核を含むため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### PR-3 作成ポイント: revised requirements, architecture, and plan for body normalization

**対象ステップ**: なし（文書の改訂だけで、ステップを含まない。改訂の対象は、requirements 3.2・AC-32、architecture §1.1・§3.7・§3.8・§3.9・§7.1、本計画のフェーズ 4 の記述・§3.2・§5・§6・§7・§9）

**推奨タイトル**: `docs(0004): normalize the generated body before the Markdown check`

**レビュー観点**: 本文の部分の表記の統一（改行と先頭の BOM）が要件・設計・計画で一致し、タイトルには適用しないこと / 統一した後の同じ文字列を判定と `Article.Body` の両方に使うことが architecture §3.7 に書かれていること / 線形性の確かめ方（上限の大きさの入力に余裕の大きい期限を設ける）の記述がステップ 4-3・§5・§6 で一致すること / コードの変更を含まないこと

**実装モデル要件**: standard

**判定理由**: 文書の改訂だけで実装のステップを含まず、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・Conditional checks のいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: Markdown の判定

**対象ファイル**
- 新設: `internal/writer/markdown.go`・`internal/writer/markdown_test.go`（`//go:build test`）
- 変更: `internal/writer/output.go`・`internal/writer/output_test.go`・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 4-1**: `markdown.go` に、本文の部分の生の HTML と閉じていないコードフェンスの判定（architecture §3.8）を非公開の関数として実装する。行の区切り、開くフェンス・フェンスに見える行・閉じるフェンス、`<` の判定と例外（エスケープ・URI の自動リンク・コードスパンの 3 条件）は architecture §3.8 のとおりとする。判定は本文の部分の長さに比例する時間で行い、同じ長さのバッククォートの列を探すたびに行の残りを読み直す方法を採らない。`gocyclo` の上限を超えないよう関数を分ける。
- [ ] **ステップ 4-2**: `output.go` の生成テキストの検証の手順 5 と手順 7 の間で、ステップ 4-1 の判定を呼ぶ。拒否は `ErrMalformedOutput` とし、エラーには理由と本文の部分の中の行番号だけを含め、HTML と判定した文字列を含めない（architecture §4.2）。あわせて、手順 5 の前に本文の部分の表記の統一（architecture §3.7。先頭の U+FEFF を除き、`\r\n` と単独の `\r` を `\n` にする）を行い、手順 5・6 と `Article.Body` に統一した後の同じ文字列を使う。統一の後もなお U+FEFF で始まる本文の部分は、手順 5 で `ErrMalformedOutput` で拒否する。判定は統一した後の本文の部分を `\n` だけで行に分ける。
- [ ] **ステップ 4-3**: `markdown_test.go` に `TestCheckBodyMarkdown` を作る。architecture §3.8 の「受理する例」「拒否する例」「過剰な拒否の一覧」の各行と、§7.1 の「Markdown の判定」に挙げた境界（フェンスの中の HTML に見える文字列、開いたものより短い閉じるフェンス、バッククォートの対を取り違えさせる形、行をまたぐリンクのタイトルの形）を、非公開の関数に対する表のテストで確かめる。あわせて、バッククォートを含む URI の自動リンクだけを含み、コードスパンを含まない行（`` <https://e.example/`> ``、拒否）を表に加える。ステップ 4-6 で外す条件ごとに、対応する行を用意する。各行は、その条件以外の規則だけでは拒否されないことを、条件を 1 つずつ外して確かめてから表に置く（例: コードスパンの例外の 1 つ目・2 つ目の条件の行は、バッククォートの列が同じ行の中で対になり、3 つ目の条件を満たす形にする）。
  - `TestCheckBodyMarkdownLinearWork` を作る（§5 の非 AC の表）。判定が本文の部分の長さに対して線形であることを、合否を壁時計の時間に依存させずに確かめる。決定的な確かめ方として、長さの異なる入力で文字の走査・比較の回数を数え、入力の長さに対して線形にしか増えないことを確かめる形を優先する。実装の構造から操作を数えられない場合に限り、線形より遅い実装（例: `<` ごとに行の先頭から読み直してコードスパンの中かを決める実装）と比べる、合否を左右しない benchmark（`BenchmarkCheckBodyMarkdown`）に時間の測定を移し、`make test` の成否を時間に依存させない。architecture §3.8 の 2 乗の例（同じ長さの列を探すたびに残りを読み直す方法）は 3 つ目の条件で線形にもなりうるので、線形より遅い実装は実装時に確かめて決める。
  - **実装時の決定。** 操作の回数を数えるには本番コードにテストのためだけの計数を入れる必要があり、`.claude/commands/runplan.md` のステップ 5 の自己確認（テストのための振る舞いを本番コードに置かない）に反するので、数える形は採らなかった。代わりに `TestCheckBodyMarkdownLinearWork` は、`Write` が受理する上限（1 MiB）の入力 4 種類（1 行の `>` のない `<` の列、1 行のコードスパンの中の `<b>` の列、1 行のエスケープした `<` の列、コードスパンを 1 つずつ含む多数の行）を、テストの中で明示した期限（20 秒）付きで判定し、期限を超えたら失敗とする。これは時間に依存しない確かめ方ではなく、余裕の大きい時間の上限である。線形の実装は `-race` でも入力 1 つあたり 0.2 秒未満で終わり、線形より遅い実装（`<` ごとに次の `>` まで読む、`<` ごとに行の先頭からコードスパンを求め直す、`<` ごとに行の先頭からバックスラッシュを数える、行ごとにコードスパンの例外の条件を本文の部分の全体で判定し直す）は 1 分以上かかるので、負荷による揺れで合否が入れ替わる余地は小さい。入力の大きさを実際の上限に合わせたのは、守る対象が実際に入りうる最大の入力であり（CLAUDE.md の Performance）、メモリの使用量も抑えられるためである。時間の測定は `BenchmarkCheckBodyMarkdown`（`make test` では実行せず、合否を左右しない）に置く。同じ長さの列を探すたびに残りを読み直す方法は、閉じる列が見つからなければその時点で条件を満たさないと判定して止まり、見つかれば閉じる列の後から読み続けるので線形になる。そのため入力に含めない。
- [ ] **ステップ 4-4**: `output_test.go` に `TestWriteRejectsMarkdownHazards` を加える。AC-24・AC-27 の各例を `Write` で通し、拒否するものはステップ 3-4 の共有のアサーション（AC-26）で、受理するもの（コードスパンの中・閉じたフェンスの中の `<details>`、URI の自動リンク、閉じたフェンス）は記事が返ることで確かめる。また、`TestWriteNormalizesBody`（AC-32。`\r\n`・単独の `\r`・`\r` の後の `\r\n`・末尾の `\r`・先頭の U+FEFF の統一と、先頭以外の U+FEFF が残ることを、`Body` の完全一致で確かめる。`\r` だけの改行で閉じたフェンスは、統一の後でだけ受理される入力にする）と `TestWriteChecksNormalizedBody`（AC-32。先頭の U+FEFF の後のフェンスが閉じていないフェンスとして、U+FEFF だけの本文の部分が空として拒否されること。どちらも統一をしなければ受理される入力にする。あわせて、閉じていないフェンスの前に U+FEFF が 2 つ続く本文の部分が拒否されること）を加える。
- [ ] **ステップ 4-5**: `package_reference.md` の `internal/writer` の行に、本文の部分の生の HTML と閉じていないコードフェンスの拒否を加える。
- [ ] **ステップ 4-6**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: 表記の統一をしない・`\r\n` より先に `\r` を置き換える・先頭の U+FEFF を除かない・すべての U+FEFF を除く・統一の後も U+FEFF で始まる本文の部分を拒否しない・判定に統一する前の本文の部分を渡す（`TestWriteNormalizesBody`・`TestWriteChecksNormalizedBody` の対応するケース）、閉じるフェンスの個数の比較を外す（同、短い閉じるフェンス）、フェンスに見える行の拒否を外す（同、インデントしたフェンス）、エスケープの判定でバックスラッシュの数の偶奇を見ない（同、`\\<div>`）、コードスパンの例外の 1 つ目の条件を外す（同、`` [a](/u "`") <details>` ``）、2 つ目の条件を外す（同、`` <1`@a.bc> <details>` ``）、自動リンクの例外でバッククォートを許す（同、`` <https://e.example/`> ``）、ステップ 4-3 で決めた線形より遅い実装に替える（`TestCheckBodyMarkdownLinearWork`。benchmark に移した場合は、時間の測定が `make test` の合否には含まれないことを述べる）、`output.go` が判定を呼ばない（`TestWriteRejectsMarkdownHazards`）。
- [ ] **ステップ 4-7**: `make fmt` → `make test` → `make lint` を通す。

### PR-4 作成ポイント: Markdown hazard and code-fence judgment

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0004): reject raw HTML and unclosed code fences in ArticleWriter output`

**レビュー観点**: architecture §3.8 の判定（行の区切り、フェンスの開閉、`<` の例外 3 条件）が本文の部分の生の HTML と閉じていないコードフェンスを拒否し、受理する形（コードスパンの中・閉じたフェンスの中の HTML、URI の自動リンク、閉じたフェンス）を誤って拒否しないこと（ステップ 4-1・4-2） / `TestCheckBodyMarkdown` が §3.8 の受理・拒否・過剰な拒否の各行と §7.1 の境界を、条件を 1 つずつ外して判別的に確かめていること（ステップ 4-3） / 線形性の確認が、上限の大きさの入力に明示した余裕の大きい期限を設ける形であり、線形の実装と線形より遅い実装の差が期限に対して十分に大きく、時間の測定は合否を左右しない benchmark に限ること（ステップ 4-3） / 拒否が `ErrMalformedOutput` で、エラーに HTML と判定した文字列を含めないこと（AC-24・AC-26・AC-27、ステップ 4-2・4-4）

**実装モデル要件**: frontier-recommended

**判定理由**: architecture §3.8 の Markdown の判定（本文の長さに比例する時間の走査、`<` の例外の 3 条件、フェンスの開閉）は、リカバリや状態機械に類する独立した最も込み入ったステップであり、PR-4 に隔離してレビューするため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書

**対象ファイル**
- 変更（必要な場合だけ）: `docs/dev/developer_guide/package_reference.md`・`docs/dev/project_overview.md`

**タスク**
- [ ] **ステップ 5-1**: `package_reference.md` の `prompts`・`internal/nilcheck`・`internal/transcript`・`internal/writer` の行と、`project_overview.md` の `:31` と想定ディレクトリ構成を、実装（各パッケージの公開 API と `writer.Article` のフィールド）と照らし合わせ、食い違いがあれば直す。
- [ ] **ステップ 5-2**: §6.1 のクロス検索を行い、結果を本計画に記録する。
- [ ] **ステップ 5-3**: `make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: documentation verification and cross-search

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `docs(0004): verify the ArticleWriter documentation against the implementation`

**レビュー観点**: `package_reference.md` の `prompts`・`internal/nilcheck`・`internal/transcript`・`internal/writer` の行と、`project_overview.md` の `Article` の項目・想定ディレクトリ構成が、PR-1・PR-2・PR-4 で実装した公開 API と `writer.Article` のフィールドに一致すること（ステップ 5-1） / クロス検索 `rg -n "isTypedNil" -g '!docs/tasks/**' .` が 0 件で、実行した HEAD と結果が §6.1 に記録されていること（ステップ 5-2） / グリーンゲートが通ること（ステップ 5-3）

**実装モデル要件**: standard

**判定理由**: ドキュメントと実装の照合、クロス検索の記録、グリーンゲートの確認に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・Conditional checks のいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `internal/nilcheck`、`NormalizedVideoURL`、`Article.ModelVersion`、`TestCommonTypesFieldSets` と文書の更新 | `make test` / `make lint` が通り、変更したテスト以外の既存テストが無変更で通る |
| M2 | フェーズ 2 | `prompts` パッケージと README、`internal/writer` の `New` とテンプレートの検査（暫定の `Write` は PR-2 のブランチ内だけ） | 同上 |
| M3 | フェーズ 3 | `Write` の実装、`prompt_test.go`・`output_test.go`・`writer_test.go` の追加分 | 同上。暫定の `Write` が残っていない |
| M4 | フェーズ 4 | `markdown.go` と判定のテスト | 同上 |
| M5 | フェーズ 5 | 文書と実装の照合、クロス検索の記録 | 同上 |

### 3.2. PR 構成

PR はフェーズを単位とするが、フェーズ 2（テンプレートと構築）とフェーズ 3（記事の生成）は 1 つの PR-2 にまとめる。PR-3 はステップを持たない文書の改訂の PR とする。各 PR は主たる関心事（共有部品 / テンプレート・構築・生成 / 文書の改訂 / Markdown の判定 / 文書の照合）を持ち、単独でグリーンゲートを通せる単位とする。本タスクは `cmd/yt2column/main.go` を変更しないため（配線は #6）、`internal/` の変更が `cmd/` に先行する順序の問題は生じない。

PR-1 は `internal/nilcheck`・`NormalizedVideoURL`・`Article.ModelVersion` という共有部品を先に完成させる。PR-2 は `New` とテンプレートの検査（フェーズ 2）と `Write` の実装（フェーズ 3）をまとめる。フェーズ 2 の暫定の `Write`（ステップ 2-5）は常にエラーを返すため単独で `main` に入れられず（§1.3 の制約）、置き換えるステップ 3-3、ステップ 3-3 が依存するステップ 3-1・3-2、その検証と同じ PR に置くテスト（ステップ 3-4〜3-7）が同じ PR に要る。この制約が強制する最小の組はフェーズ 2 とステップ 3-1〜3-7 である。残るステップ 3-8〜3-10 は、`package_reference.md` を同コミットで更新する規則（ステップ 3-8）、実装と同じ PR に置く壊し確認（ステップ 3-9）、グリーンゲート（ステップ 3-10）であり、いずれもフェーズ 3 の実装と同じ PR に置く。そのためフェーズ 2 とフェーズ 3 を 1 つの PR-2 にまとめる。PR-3 は最も込み入った Markdown の判定（architecture §3.8）を隔離し、PR-4 は実装の確定後に文書と実装を照合する。`package_reference.md` は PR-1〜PR-3 で更新し、PR-4 では実装との一致を確認するだけである。

**PR-2 の大きさとレビュー方法。** PR-2 はフェーズ 2 とフェーズ 3 を合わせた 21 ステップになり、レビューする差分が大きい。`/runplan` はフェーズの区切りでコミットを分けるので、レビューは (1) テンプレートの読み込み・検査と `New`（ステップ 2-1〜2-11）、(2) `Write` の実装と生成の検証（ステップ 3-1〜3-10）の 2 つのチェックポイントに分けて行う。ステップ 3-3 で暫定の `Write` が置き換わり、PR-2 の最終状態に暫定の実装は残らない。

**高リスクなステップの隔離（レビュー用チェックリストの記録）。** ステップ 2-4（テンプレートの構文の許可リストと fail-secure な分岐）とステップ 3-1・3-2（信頼できない LLM 出力の検証）はセキュリティの中核だが、暫定の `Write` の制約により独立した PR にできない。`New`（ステップ 2-5）がステップ 2-4 の検査を使い、`Write`（ステップ 3-3）がステップ 3-1・3-2 を使う依存関係から、これらを PR-2 の最後に置くこともできない。そのため PR-2 では隔離せず、上記の 2 つのチェックポイントでレビューする。

**PR-3（文書の改訂）。** Markdown の判定の実装のレビューで生じた設計の変更（本文の部分の表記の統一、線形性の確かめ方）を、コードより先に要件・設計・計画の改訂として承認し、マージする。実装の差分と、それを律する文書の差分を分けてレビューするためである。PR-3 はコードを変更しないので、グリーンゲートは `main` と同じ結果になる。

**PR-4 までの順序の制約。** PR-2 のマージ直後の `main` は、Markdown の判定（PR-4）が入るまで生の HTML と閉じていないコードフェンスを受理する。`main` から `ArticleWriter` を使う配線は #6 まで無いので利用者への影響はない（architecture §8）。PR-4 は、`ArticleWriter` を `main` の経路に載せる #6 の作業より前にマージする（§6・§9）。

**PR の区切りの不変条件。** `/runplan` は §2 を文書の順に走査し、`PR-N 作成ポイント` に達したときにだけ PR を作る。そのため §2 は次を満たす。(1) 文書の順で、すべてのステップは、自分の PR の 1 つ前の作成ポイント（先頭の PR では文書の先頭）と自分の PR の作成ポイントの間にあり、他の PR のステップがその間に入らない。(2) すべての PR が作成ポイントを持ち、§3.2 の表・§7 のチェックリスト・§9 の実行順に現れる。本計画はステップを並べ替えていないため、ステップの番号の順と文書の順は一致する。 PR-3 はステップを持たないので (1) は当てはまらず、作成ポイントを PR-2 の作成ポイントとフェーズ 4 の間に置く。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 | `internal/nilcheck` の新設、`transcript.NormalizedVideoURL`、`Article.ModelVersion`、`pipeline_test.go`・`project_overview.md`・`package_reference.md` の更新 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10 / 2-11 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 | `prompts` パッケージとテンプレート、`internal/writer` の `errors.go`・`template.go`・`prompt.go`・`output.go`・`New`・`Write`（Markdown の判定を除く）とそのテスト、import guard、`package_reference.md` の更新 | frontier-recommended |
| PR-3 | なし | requirements 3.2・AC-32、architecture §1.1・§3.7・§3.8・§3.9・§7.1、本計画のフェーズ 4 の記述などの改訂（コードの変更なし） | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | `internal/writer/markdown.go` の判定、`output.go` からの呼び出し、`markdown_test.go`・`output_test.go` の追加、`package_reference.md` の更新 | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 | 文書と実装の照合、クロス検索の記録、グリーンゲート | standard |

### 3.3. 実装順序の根拠

architecture §8 の順序に従う。共有部品（フェーズ 1）は `internal/writer` が使うので最初に置く。`New` とテンプレートの検査（フェーズ 2）は `Write` の前提になる。`ArticleWriter` の interface を満たすため、フェーズ 2 は暫定の `Write` を置き、フェーズ 3 で置き換える。暫定の `Write` が存在するのは PR-2 のブランチ内だけで、`main` には残らない（§3.2）。`main` からの配線はない（配線は issue #6 で行う）。Markdown の判定（フェーズ 4）は最も込み入っているので独立させ、PR-3 とする。PR-3 をマージするまでは `main` が生の HTML を受理するが、配線が #6 まで無いので利用者への影響はない（§3.2・§6）。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。テスト関数名と AC の対応は §5 に示す。

### 4.1. ユニットテスト

- `LLMClient` には `llmtestutil.FakeLLMClient` を使い、LLM の API・ネットワークを呼ばない。上書きファイルは `t.TempDir` の下に作る（AC-21）。`TestPromptsREADMEMatchesContract` と `TestWriterImports` はリポジトリの中の文書とソースを読む guard であり、`ArticleWriter` の振る舞いのテストではない。前者は、README の記述が `internal/writer` の実際の契約（テンプレートデータ・許可する関数・上限）と一致することを確かめ、契約は writer のソースから導く（コピーした一覧に依存しない）。`internal/writer` のユニットテストをテストの外のファイルに依存させないため、`TestInterfaceDocComments` と同じ場所である `internal/pipeline/pipeline_test.go` のクロスパッケージの guard に置く（AC-21 は `internal/writer` のユニットテストだけに掛かる）。
- 境界の値（テンプレート・プロンプト・`Text`・`Model`・`ModelVersion` の上限）は、ちょうど上限と上限 + 1 バイトの組で確かめる。
- 拒否のケースは、番兵、ゼロ値の `Article`、`LLMClient` の呼び出し回数（呼ぶ前の拒否では 0 回）を確かめる。
- **判別的な拒否テスト。** 拒否のテストの入力は、対象の規則だけに違反するようにする。ほかの規則や検査の順序を入れ替えた場合には別の番兵になるので、規則の飛ばし・並べ替え・2 つの経路の片方への適用漏れがテストを失敗させる。計画が検証すると述べるすべてのコード経路（埋め込みの既定テンプレート 2 つを含む）は、それぞれ独立に実行する。
- 網羅率の目標は、`internal/writer`・`internal/nilcheck`・`prompts` の本番コードのうち到達できるすべての文を、それぞれのパッケージのテストが実行すること（文の網羅率）とする。`go test -tags test -coverprofile` の結果を `go tool cover -func` と `-html` で確認し、通らない文がある場合は理由をフェーズのコミットメッセージに書く。`prompts` はテストファイルを持たないので、`internal/writer` のテストから `-coverpkg` で確かめる。
- **後方互換性:** `internal/pipeline`・`internal/transcript` の振る舞いは変えない。期待値を更新する既存のテストは `TestCommonTypesFieldSets` だけで、ほかの既存テストは変更せずに通す（ステップ 1-10）。

### 4.2. 統合テスト

本タスクに統合テストはない（architecture §7.2）。

### 4.3. セキュリティテスト

architecture §7.3 に従う。計画固有の事項は次のとおり。

- エラーに値を含めないこと（AC-26）は、`ErrMalformedOutput` のすべての拒否ケースで共有のアサーション（ステップ 3-4）を使って確かめる。`ErrInvalidTranscript` のエラーに `Transcript` の値が現れないことも `TestWriteRejectsInvalidTranscript` で確かめる。
- 許可リストの fail-secure の分岐は、拒否する構文を `default` で拒否する実装（ステップ 2-4）と、それらの構文のテスト（`TestTemplateSyntaxAllowlist`）で確かめる。

### 4.4. テストヘルパー

- `internal/writer/test_helpers.go`（`//go:build test`、`test_organization.md` 分類 B）に、上書きファイルを書くヘルパー、検証を通る `Transcript` を作るヘルパー、目印の定数、`requireNonRoot`、`ErrMalformedOutput` の共有のアサーションを置く。`package writer` のテストは `internal/writer/testutil`（`writer` を import する）を import できないので、`internal/writer` の複数のテストファイルが使うヘルパーは分類 B とする。
- `testutil/` は追加しない（新しい公開 interface の fake は要らない。`TestFakesCarryBuildTag` の件数を変えない）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行可能なテスト、`static` は guard テストを指す。テストのパスはすべて `internal/writer/` の下のファイルである（明記したものを除く）。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | 上書きなしの構築で既定のテンプレートのプロンプト | test | `writer_test.go::TestNewUsesDefaultTemplates` |
| AC-02 | nil・typed nil の `LLMClient` を拒否 | test | `writer_test.go::TestNewRejectsNilClient`・`internal/nilcheck/nilcheck_test.go::TestIsNil` |
| AC-03 | 片方だけの上書き | test | `writer_test.go::TestNewOverridesEachTemplate` |
| AC-04 | 構築後の書き換えの影響を受けない | test | `writer_test.go::TestNewReadsOverrideOnce` |
| AC-05 | 不正な上書きファイルを `ErrInvalidTemplate` で拒否 | test | `template_test.go::TestNewRejectsInvalidOverrideFile`・`TestNewOverrideFileFIFO`・`TestNewOverrideFileSizeLimit`・`TestTemplateSyntaxAllowlist` |
| AC-06 | 既定のテンプレートが同じ検査を通る | test | `template_test.go::TestDefaultTemplatesPassChecks` |
| AC-07 | 不正な `Transcript` を拒否し `LLMClient` を呼ばない | test | `prompt_test.go::TestWriteRejectsInvalidTranscript`・`internal/transcript/video_id_test.go::TestNormalizedVideoURL` |
| AC-08 | 空のメタ情報を拒否しない | test | `prompt_test.go::TestWriteAcceptsEmptyMetadata` |
| AC-09 | `Generate` 1 回、同じ `ctx`、`MaxOutputTokens` 0 | test | `writer_test.go::TestWriteCallsGenerateOnce` |
| AC-10 | 値の埋め込みと字幕本文の連結 | test | `prompt_test.go::TestWriteEmbedsValues` |
| AC-11 | `StartMs` を含めない | test | `prompt_test.go::TestWriteOmitsStartMs` |
| AC-12 | 値をテンプレートとして解釈しない | test | `prompt_test.go::TestWriteDoesNotInterpretValues` |
| AC-13 | 展開時の失敗は `ErrInvalidTemplate` | test | `prompt_test.go::TestWriteExpansionFailure` |
| AC-14 | `LLMClient` のエラーの判別 | test | `writer_test.go::TestWriteReportsLLMError` |
| AC-15 | 終了済みの `ctx` | test | `writer_test.go::TestWriteContextDone` |
| AC-16 | 成功時の `Article` の各フィールド | test | `output_test.go::TestWriteArticle` |
| AC-17 | `Body` は本文・区切り・出典ブロックの連結 | test | `output_test.go::TestWriteArticle`・`TestWriteSourceBlock` |
| AC-18 | `SourceURL` と出典ブロックで終わる `Body` | test | `output_test.go::TestWriteSourceBlock` |
| AC-19 | 本文の終わり方と偽の出典によらず出典ブロックと空行 | test | `output_test.go::TestWriteSourceBlock` |
| AC-21 | ユニットテストは API・ネットワーク・テストの外のファイルを使わない | test | `Write` を呼ぶすべての `internal/writer` のテスト（`LLMClient` は `llmtestutil.FakeLLMClient` で、`Calls` の記録を確かめる。上書きファイルは `t.TempDir` の下に作る） |
| AC-22 | 先頭行の形の拒否 | test | `output_test.go::TestWriteRejectsMalformedText` |
| AC-23 | タイトルと本文の部分の拒否、前後の空白の除去 | test | `output_test.go::TestWriteRejectsMalformedText` |
| AC-24 | 閉じていないコードフェンスの拒否 | test | `output_test.go::TestWriteRejectsMarkdownHazards`・`markdown_test.go::TestCheckBodyMarkdown` |
| AC-25 | 空の `Model` の拒否 | test | `output_test.go::TestWriteRejectsInvalidModel` |
| AC-26 | エラーに値が現れない | test | ステップ 3-4 の共有のアサーション（`TestWriteRejectsMalformedText`・`TestWriteRejectsInvalidModel`・`TestWriteOutputSizeLimits`・`TestWriteRejectsMarkdownHazards`・`TestWriteChecksNormalizedBody` の全拒否ケース） |
| AC-27 | 生の HTML の拒否と受理する形 | test | `output_test.go::TestWriteRejectsMarkdownHazards`・`markdown_test.go::TestCheckBodyMarkdown` |
| AC-28 | プロンプトの上限の境界と `printf` の構築時の拒否 | test | `prompt_test.go::TestWritePromptSizeLimit`・`TestBoundedWriter`・`template_test.go::TestTemplateSyntaxAllowlist`（`{{printf "%1000000000s" .Title}}`） |
| AC-29 | `Model`・`ModelVersion` の拒否 | test | `output_test.go::TestWriteRejectsInvalidModel` |
| AC-30 | 既定のテンプレートが 4 つの値と見出しの指示を含む | test | `writer_test.go::TestDefaultTemplatesEmbedAllValues`・`template_test.go::TestDefaultSystemTemplateHeadingInstruction` |
| AC-31 | `Text`・`Model`・`ModelVersion` の上限の境界 | test | `output_test.go::TestWriteOutputSizeLimits` |
| AC-32 | 本文の部分の表記の統一と、統一の後の判定 | test | `output_test.go::TestWriteNormalizesBody`・`TestWriteChecksNormalizedBody` |

AC に対応しない、architecture が求める検証は次のとおり。

| 対象 | 検証の実行場所 |
|---|---|
| `isTypedNil` の移動で `internal/pipeline` の振る舞いが変わらない（architecture §3.11） | `internal/pipeline/pipeline_test.go::TestPipelineNewNilStage`（無変更） |
| `NormalizedVideoURL` の切り出しで `validateVideoURL` の振る舞いが変わらない（architecture §3.11） | `internal/transcript/video_id_test.go::TestValidateVideoURL`（無変更） |
| `Article` のフィールドの組（architecture §3.1、I-02） | `internal/pipeline/pipeline_test.go::TestCommonTypesFieldSets` |
| `ArticleWriter` の doc コメントの条項 | `internal/pipeline/pipeline_test.go::TestInterfaceDocComments`（無変更） |
| `prompts/README.md` の記述が `internal/writer` の実際の契約（テンプレートデータ・許可する関数・上限）と一致する（architecture §3.2） | `internal/pipeline/pipeline_test.go::TestPromptsREADMEMatchesContract`（契約は writer のソースから導く） |
| `internal/writer`・`prompts` がプロバイダのパッケージを import しない（requirements 4.5、AC-21） | `internal/pipeline/pipeline_test.go::TestWriterImports` |
| シンボリックリンクを辿る（architecture §3.3） | `template_test.go::TestNewRejectsInvalidOverrideFile` |
| 上書きファイルを上限 + 1 バイトまでしか読まない（architecture §3.3・§5.1 の T5） | `template_test.go::TestReadOpenedFileBound`（実装時に追加） |
| 構築時のエラーが、どちらのテンプレートか、既定のものか上書きファイルか、上書きファイルのパスを含む（architecture §4.2） | `template_test.go::TestInvalidTemplateErrorNamesSource` |
| 上限付きの書き込み先（architecture §3.6） | `prompt_test.go::TestBoundedWriter`・`TestWritePromptSizeLimitStopsExpansion` |
| Markdown の判定が本文の部分の長さに対して線形である（architecture §3.8） | `markdown_test.go::TestCheckBodyMarkdownLinearWork`（上限の大きさの入力を、テストの中で明示した余裕の大きい期限付きで判定する。ステップ 4-3 の実装時の決定）・`BenchmarkCheckBodyMarkdown`（時間の測定。合否を左右しない） |

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `TestNewOverrideFileFIFO` で構築が戻らない | テスト全体が止まる | 構築を別の goroutine で呼び、時間切れを失敗として報告し、書き込み側を開いて goroutine を解放する（I-01） |
| Markdown の判定の線形性の確認が実行環境の負荷で不安定になる | CI が断続的に失敗する | 合否は期限による時間の上限だが、線形の実装（`-race` で入力 1 つあたり 0.2 秒未満）と期限（20 秒）と線形より遅い実装（1 分以上）の間に数十倍以上の差を取り、負荷で結果が変わりにくくする。時間の測定は合否を左右しない benchmark に限る（ステップ 4-3） |
| `syscall.Mkfifo` が windows にない | windows で `internal/writer` のテストがビルドできない | requirements 4.4 の対象は macOS と Linux で、CI も Linux だけである（§1.3）。windows は対象外とする |
| `test_helpers.go` はテスト向けの lint の除外が効かない | `make lint` が通らない | エラーを無視せず、固定の文字列を定数にする |
| `gosec` が上書きファイルの `os.OpenFile` を指摘する | `make lint` が通らない | 1 行に限った `//nolint:gosec` と理由のコメント（ステップ 2-4） |
| 暫定の `Write` がフェーズ 3 の後も残る | `Write` が常に失敗する | ステップ 3-3 で置き換え、M3 の完了条件で確かめる。暫定の実装は常にエラーを返し、検証していない記事を返さない |
| PR-4 をマージする前に #6 の配線が `main` に入る | 生の HTML と閉じていないコードフェンスを受理する `Write` が利用者に届く | PR-4 を、`ArticleWriter` を `main` の経路に載せる #6 の作業より前にマージする（§3.2・§9） |
| Go のバージョンの違い（CI の 1.26.5 と手元の 1.27.1）で `text/template/parse` のノードの種類が異なる | 許可リストの判定が CI と手元で変わる | 未知の種類は `default` で拒否する（fail-secure）。`TestTemplateSyntaxAllowlist` が CI で両方の判定を確かめる（architecture §1.3） |

### 6.1. クロス検索

`make lint` と `make test` では見つからない、削除した識別子の残りを確かめる。ステップ 5-2 で実行し、結果をここに記録する。

- [ ] `isTypedNil` が、`docs/tasks/` を除くリポジトリのどこにも残っていない（Go のコメントと文書を含む）。コマンドは `rg -n "isTypedNil" -g '!docs/tasks/**' .` とし、期待する結果は 0 件である。計画の作成時（HEAD `4194276`）の結果は §1.3 の 3 件である。実行した HEAD と結果をここに書く。

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10 / 2-11 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10）。暫定の `Write` が残っていない
- [ ] PR-3 マージ済み（対象ステップ: なし。文書の改訂）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
- [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る
- [ ] §5 のすべての AC の検証が通る
- [ ] `implementation_handoff.md` の I-01・I-02 が §1.5 のとおり反映されている

## 8. 成功基準 (Success Criteria)

- **機能:** AC-01〜AC-19・AC-21〜AC-32 のすべてが §5 の検証で確認されている。
- **品質:** `make test`・`make lint` が通る。§4.1 の網羅率の目標を満たす。各テストは対象を壊して失敗することを確認済みで、そのことがコミットメッセージに記録されている。
- **セキュリティ:** 出典リンクを検証済みの `VideoID` から組み立てること（AC-18・AC-19）、値をテンプレートとして解釈しないこと（AC-12）、エラーに値を含めないこと（AC-26）、生の HTML と閉じていないフェンスの拒否（AC-24・AC-27）、制御文字の拒否（AC-29）、展開と上書きファイルの読み込みでメモリを使い切らず待ち続けないこと（AC-05・AC-28）を確認済みである。
- **互換性:** `TestCommonTypesFieldSets` 以外の既存テストが無変更で通る。依存モジュールを追加していない（`.golangci.yml` を変更していない）。
- **ドキュメント:** `prompts/README.md`・`package_reference.md`・`project_overview.md` が実装と一致する（`internal/pipeline/pipeline_test.go::TestPromptsREADMEMatchesContract` とステップ 5-1）。

## 9. 次のステップ (Next Steps)

- 本計画は `approved`。PR の区切りは §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済みである。
- `/runplan 0004` で PR の順（PR-1 → PR-2 → PR-3 → PR-4 → PR-5）に実装する（各 PR は独立してグリーンゲートを通す。§3.2 の不変条件）。
- PR-4（Markdown の判定）は、`ArticleWriter` を `main` の経路に載せる #6 の作業より前にマージする（§3.2・§6）。
- 実装の完了後、architecture §9 の申し送りを #6（上書きファイルのパスの受け渡し、本文の部分の制御文字の端末への表示）、#7（出典ブロックの Slack での表示、タイトルのエスケープ、Slack の山括弧の記法）、#8（本番用のテンプレート）の作業で参照する。
