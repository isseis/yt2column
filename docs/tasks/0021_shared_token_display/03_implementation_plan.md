# 実装計画書：localhost サーバの起動時に共有トークンを表示する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-16 |
| Review date | 2026-10-16 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した、localhost サーバの起動のたびに共有トークンを表示する振る舞いを、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   `cmd/yt2column-server` の `run` の手順 2（トークンの読み込みと表示）で、生成した起動だけでなく起動のたびに表示し、表示に失敗しても起動を止めない（設計書 3.1）。
-   標準出力が閉じたパイプのときもプロセスが終了しないようにし（設計書 3.1）、表示に失敗しても待ち受けるテストと、閉じたパイプでも待ち受けるテストを加える（設計書 7）。
-   使われなくなる `created` を `internal/authtoken.LoadOrCreate` の戻り値から外し、表示に合わせて `Token.Value` のコメントを改める（設計書 2.2・3.2）。
-   README・`docs/dev/security.md` §8.2・`docs/dev/developer_guide/package_reference.md` を改め、README・`security.md` §8.2 の記載を `cmd/yt2column-server/docs_test.go` のガードのテストで固定する（設計書 3.4・7）。

本計画で使う略号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。

本計画は、設計書 8 章の実装優先順位 1〜3 の順序に従い、フェーズ 1〜3 とする。ただし、表示の条件を変えると同じコミットで失敗する既存のテスト（設計書 8 章の優先順位 2 が挙げる `TestTokenShownOnlyWhenCreated` と `TestStartupFailures` の表示失敗の行の更新）は、グリーンゲート（`make test && make lint` が通る状態。§3.2）を保つためフェーズ 1 に含める。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に守るのは、表示の判断から「生成したとき」を外すこと、表示を best-effort（失敗しても起動を止めない扱い）とすること、共有トークンの生成・保存・読み込み・検査の振る舞いを変えないこと（`internal/authtoken.LoadOrCreate` は戻り値から `created` を外すだけで、ファイルの扱いは変えない）、出力先の分離（標準出力はトークンの 1 行だけ）を維持することである。
-   変更は `cmd/yt2column-server` の起動の手順と、本計画が挙げる既存のテスト・文書に限る。本番のコードに新しいパッケージ・型・インタフェース・エラー型を加えない（設計書 1.1 原則 5、要件書 4.5）。テストの補助（`test_helpers.go` の同期するバッファなど）は、設計書 2.2 のとおり `cmd/yt2column-server` の中で加える。
-   `internal/authtoken` では、`LoadOrCreate` と内部の `create` の戻り値から `created` を外し（`(Token, bool, error)` → `(Token, error)`）、`Token.Value` のコメントを改める。ファイルの生成・保存・読み込み・検査の振る舞いは変えない。`internal/authtoken/authtoken_test.go` は `created` の検査だけを外し、値・ファイル・エラーの検査は残す（設計書 3.2。H-01）。
-   Go のソースのコメント・識別子・文字列リテラルは英語で書く（`_context.md` の Source-language rule）。表示（標準出力に書くトークンの 1 行）と案内（表示のあとに標準エラー出力に書く 1 行）の文言は、設計書 3.4 の「案内の文言（実装で確定する）」のとおり、実装で決める。
-   変更するテストの名前（`TestTokenShownOnEveryStart`）は計画上の名前である。実装で変える場合は、§1.3 の名前の台帳と §5 を同じコミットで更新する。
-   各テストは、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」、設計書 7）。壊す対象と、それを検出するテストは、各フェーズの最後のステップに挙げる。
-   各フェーズの完了条件は、`make fmt` → `make test` → `make lint` が通ることである。

### 1.3. 既存コード調査結果

HEAD `f40a831` で確認した。設計書は既存のファイルの行番号をコミット `3180ae3` で記している。`git diff --name-only 3180ae3 f40a831 -- cmd/yt2column-server internal/authtoken internal/server README.md docs/dev CLAUDE.md` の出力は空だった（2026-10-16 に実行）。したがって、設計書の `file:line` は HEAD `f40a831` でもそのまま有効である。以下の `file:line` も HEAD `f40a831` のものである。

**`cmd/yt2column-server`（変更するもの）**

-   `run`（`cmd/yt2column-server/run.go:133-224`）の手順 2 は `run.go:159-183` にある。トークンを `token, created, err := authtoken.LoadOrCreate(...)`（`:161`）で受け取り、`created` が真のときだけ値を標準出力へ書き出し（`:172-183`）、値の取り出しの失敗（`:173-177`）と標準出力への書き出しの失敗（`:178-181`）を、いずれも標準エラー出力に 1 行書いて `exitUsage` で終えている。本計画は、この分岐から表示の条件（`created`）を外し、書き出しの失敗で終えないようにする。値の取り出しの失敗は `exitUsage` のまま残す（設計書 3.1）。
-   表示の 1 行の案内（`run.go:182`）は「…(it is not shown again; read the file to see it later)」と述べる。本計画では、起動のたびに表示する振る舞いと食い違うこの記述を、設計書 3.1 の「常に同じ文言」の方針に合わせて改める。
-   表示の失敗の 1 行（`run.go:175` の「the created token cannot be shown」と `run.go:179` の「writing the created token to standard output failed」）は「created」を前提にする。起動のたびに表示する振る舞いに合わせて改める（設計書 3.1）。
-   `run` の doc コメント（`run.go:128-132`）の「Standard output only ever receives a newly created token」と、手順 2 のコメント（`run.go:159-160`）の「so a token created by a start whose listen then fails is still shown once」は、起動のたびに表示する形に改める（設計書 3.1）。
-   トークンは `redact.SecretStrings` の対象に含まれ（`run.go:170`）、伏せ字の経路（`redact.Writer`・`redact.Line`）を通る。この扱いは変えない（設計書 3.2・5）。
-   `main`（`cmd/yt2column-server/main.go:18-21`）は `signalrun.Run` に `run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionDeps())` を渡す。標準出力が閉じたパイプへの書き込みでプロセスが終了しないようにする処理は無い。本計画で加える（設計書 3.1）。

**`internal/authtoken`（戻り値とコメントを変更するもの）**

-   `LoadOrCreate(path string, random io.Reader) (Token, bool, error)`（`internal/authtoken/authtoken.go:99`）の第 2 戻り値 `created` を読む本番のコードは `run.go:161` だけである（`rg -n "LoadOrCreate" --type go` の結果。2026-10-16、HEAD `f40a831`）。本計画で `run` が使わなくなると、読むのは `internal/authtoken` のテストだけになる。CLAUDE.md の「Before preserving an exported field as an extension point, count its real uses」に従い、`LoadOrCreate` の署名を `(Token, error)` に改める。doc コメント（`:94-98`）の「created reports whether this call generated the token」も外す（設計書 3.2）。
-   内部の `create(path string, random io.Reader) (Token, bool, error)`（`authtoken.go:201`）の `created` も、`LoadOrCreate` がそのまま返すためだけにある。同じく `(Token, error)` に改め、doc コメント（`:194-200`）の「created」「reports created=false」の記述を「既存のファイルの値を返す」形に改める。`link(2)` の `EEXIST` で既存のファイルを読む振る舞い（`:232-238`）は変えない。
-   `Token.Value`（`internal/authtoken/authtoken.go:135`）のコメント（`:132-134`）の「for the one-time display after creation and for the log redaction list」を、起動のたびの表示に合わせて改める（設計書 2.2）。

**既存のテスト（更新・追加するもの）**

-   `TestStartupFailures`（`cmd/yt2column-server/run_test.go:43-235`）は、起動を止める各失敗の行を持つ表である。「token not written to stdout」の行（`:114-121`）と、その行だけが使う `stdoutFails` のフィールド（`:62-63`）・分岐（`:194-197`・`:216-219`）を外す。他の行（「model and webhook missing」`:75-84`・「token file readable by others」`:85-93` など）は維持する。`tokenShown` の説明（`:54-56`）と、「port in use」の行のコメント（`:159-160` の「so a start whose listen fails still shows it once」）は起動のたびの表示に合わせて改める（設計書 7）。
-   `TestTokenShownOnlyWhenCreated`（`run_test.go:257-286`）は、2 回目の起動が標準出力へ何も書かないことを確かめる。`TestTokenShownOnEveryStart` に改称し、2 回目も標準出力に値の 1 行を書くこと、ファイルが変わらないこと、標準エラー出力の案内に「値が再び表示されない」とする記述が現れないことを確かめる形にする（設計書 7。AC-01〜AC-03）。
-   `TestOutputsOmitSecrets`（`run_test.go:302-338`）と `requireNoSecret`（`cmd/yt2column-server/test_helpers.go:669-678`）は、トークンが出力に現れないことの検査に使う（AC-06）。変更しない。
-   `TestServerEndToEnd`（`cmd/yt2column-server/server_test.go:27-87`）は、2 回目の起動が 1 回目のトークンで認証を受けることは確かめるが、標準出力に表示された 1 行を値として使ってはいない。表示された 1 行だけを取り出して `Authorization: Bearer` に使う検査を加える（AC-07。設計書 7）。`:26` のコメント「That only the first start shows the token is TestTokenShownOnlyWhenCreated's.」を改める（設計書 1.4）。
-   `failingWriter`（`run_test.go:238-241`）・`writeTokenFile`（`run_test.go:244-255`）・`startServer`（`test_helpers.go:238-263`）・`startServerChild`（`test_helpers.go:571-605`）・`requireNoSecret` は既存の補助で、本計画の新しいテストで再利用する。補助の追加・拡張は、§1.3「テストの補助で直す点」の同期するバッファと、ステップ 2-1・2-2 の `startServer`・`startServerChild` の拡張に限り、それ以外の補助は作らない。
-   `internal/authtoken/authtoken_test.go` は、`created` を受け取って確かめる箇所（`TestLoadOrCreateCreates` `:25-31`、`TestLoadOrCreateReadsAgain` `:74-93`、`TestLoadOrCreateRejectsInvalidFile` `:220-223`、`TestCreateDoesNotOverwriteExisting` `:278-284`、`TestLoadOrCreateRandomError` `:305-311`）と、`_` で捨てる箇所（`:254`・`:321`・`:380`）を新しい署名に合わせる。`created` の検査は外し、残りの検査（ファイルの作成と権限、2 回目に同じ値を読むこと、不正なファイルの拒否、既存のファイルを上書きしないこと、乱数の失敗）はそのまま残す（設計書 3.2・7）。生成したか読んだかは、ファイルの有無と値の一致で確かめられるので、`created` の検査を外しても確かめる振る舞いは減らない。

**テストの補助で直す点**

-   `startServer`（`test_helpers.go:238-263`）は、待ち受けの前にポートを取られた試行を再試行し、すべての試行の出力を同じバッファへ書き足す（コメント `:232-236`）。起動のたびに表示すると、再試行が起きたときに標準出力が複数行になり、`TestTokenShownOnEveryStart` と AC-07 の検査が偶発的に失敗する。試行ごとに標準出力のバッファを新しくし、待ち受けた試行の標準出力だけを返す形に改める（設計書 7）。
-   `runningServer.stdout` は `run` の書き込みを `stop` の後にだけ読む取り決めになっている（`test_helpers.go:228` のコメント）。AC-07 はサーバの稼働中に標準出力に表示された行を読むので、標準出力を、書き込みと読み出しを同期するバッファ（`run.go` の `lockedWriter` と同じ考え方）にし、稼働中に表示された行を取り出せるようにする。標準出力の読み出しが `run` の書き込みと競合しないようにするためである（`make test` は `-race` で走る）。

**文書**

-   README の生成した起動だけを前提にした記述は `README.md:196`（「(on the first start only)」）、`:214`（「prints it once」）、`:217-218`（「On later starts the server reads the same token and does not print it」）にある。ステップ 3-1 で起動のたびの表示に改める。
-   `security.md` §8.2 の「表示と作り直し」は `docs/dev/security.md:119` にあり、「生成した起動だけ」を前提にする。ステップ 3-2 で改める。同じ節の「拡張への受け渡し」の行（`:120`）の「生成時に表示されたトークン」も、起動のたびの表示に合わせて改める（要件書 F-002 の「`security.md` §8.2 を起動のたびに表示する形に改める」に含まれる）。
-   `docs/dev/developer_guide/package_reference.md:16` の `cmd/yt2column-server` の行の 2 か所を、§1.3 の名前の台帳のとおりに改める。

**名前の台帳**

本計画で名前を変える・消すものは、この表を正とする。タスクの本文と §5 は、この表に合わせる。

| 種類 | 旧 | 新 | 場所 | ステップ |
|---|---|---|---|---|
| テスト関数 | `TestTokenShownOnlyWhenCreated` | `TestTokenShownOnEveryStart` | `cmd/yt2column-server/run_test.go` | 1-6 |
| テスト関数（追加） | - | `TestDisplayFailureKeepsServing` | `cmd/yt2column-server/run_test.go` | 2-3 |
| テスト関数（追加） | - | `TestClosedStdoutPipeKeepsServing` | `cmd/yt2column-server/run_test.go` | 2-4 |
| 関数の署名 | `LoadOrCreate(path string, random io.Reader) (Token, bool, error)` | `LoadOrCreate(path string, random io.Reader) (Token, error)` | `internal/authtoken/authtoken.go` | 1-1 |
| 関数の署名 | `create(path string, random io.Reader) (Token, bool, error)` | `create(path string, random io.Reader) (Token, error)` | `internal/authtoken/authtoken.go` | 1-1 |
| 表の行 | `TestStartupFailures` の「token not written to stdout」 | 削除（表示の失敗は起動を止めない） | 同上 | 1-6 |
| 表のフィールド | `TestStartupFailures.stdoutFails` | 削除（表示の失敗で終了しなくなる） | 同上 | 1-6 |
| 文書の語句 | 「他方が先に作ったファイルを読んで `created=false` を返し、上書きしない」 | 「他方が先に作ったファイルを読んでその値を返し、上書きしない」 | `docs/dev/security.md:116` | 3-2 |
| 文書の語句 | 「生成時に表示されたトークン」 | 「起動時に表示されたトークン」 | `docs/dev/security.md:120` | 3-2 |
| 文書の語句 | 「a created token is written to standard output once, before listening, and nowhere else」・「Standard error carries every line except the created token」 | 「the shared token is written to standard output on every start, before listening, and nowhere else」・「Standard error carries every line except the shared token」 | `docs/dev/developer_guide/package_reference.md:16` | 3-3 |

**改名の残余参照の確認（横断検索）**

-   `TestTokenShownOnlyWhenCreated` の出現は、`cmd/yt2column-server/run_test.go:257`・`:261`、`cmd/yt2column-server/server_test.go:26` である（`rg -n "TestTokenShownOnlyWhenCreated"` の結果。2026-10-16、HEAD `f40a831`）。run_test.go はステップ 1-6 で改め、server_test.go:26 は同じステップで改める。
-   `LoadOrCreate` の呼び出しは、`cmd/yt2column-server/run.go:161`、`internal/server/test_helpers.go:164`（`newTestToken`）、`internal/authtoken/authtoken_test.go` の各テストである。`create` の直接の呼び出しは `authtoken.go:107` と `authtoken_test.go:278` である（`rg -n "LoadOrCreate\(|[^a-zA-Z.]create\(" --type go` の結果。2026-10-16、HEAD `f40a831`）。いずれもステップ 1-1 で改める。`newTestToken`（`internal/server/test_helpers.go:157-171`）は `created` が偽であること（ファイルから読んだこと）を確かめている。新しい署名では、代わりに返った `Token` が `Matches(testTokenValue)` であることを確かめる（ファイルの値を読んだことを、より直接に確かめる）。文書のうち生きている文書で `created` に触れるのは `docs/dev/security.md:116` だけで、ステップ 3-2 で改める。
-   `docs/tasks/0011_localhost_server/03_implementation_plan.md:294` も旧名を挙げるが、`0011`（localhost サーバ本体を作ったタスク）の文書は当時の記録として残し、変更しない（要件書 §1・F-002）。設計書 1.4 も `0011` の文書を変更しないとしている。

**適用しない項目**

-   本計画では、表示の判断を具象型で分岐しない（`created` を戻り値から外し、表示は無条件に行う）。そのため、設計書が特定の具象型の作り方を制約する場合に必要となる確認（計画の作成手順「mkplan」の step 5 の「具象型の作り方」）は、本計画には該当しない。

### 1.4. 申し送りの項目への対応

-   `implementation_handoff.md` はない。
-   [design_handoff.md](design_handoff.md) の H-01〜H-03 は、すべて設計書 3.3 に記録されている。本計画は設計書のその方針に従って実装する。
    -   H-01（表示の判断と `created` の扱い）: フェーズ 1。表示は `created` で分岐せず、案内は常に同じ文言にする。使われなくなる `created` は `LoadOrCreate` の戻り値から外す（設計書 3.2。ファイルの扱いは変えない）。
    -   H-02（露出の経路と README の起動例）: フェーズ 3。README の `2>>server.log` の起動例を維持し、標準出力をリダイレクトした場合の注意を加える。`security.md` §8.2 にも経路を列挙する。
    -   H-03（表示の検査の作り方）: フェーズ 1・2。`run` の標準出力・標準エラー出力の引数に `bytes.Buffer` と書き出しに失敗する `io.Writer` を渡す。トークンが標準出力の 1 行だけに載り、標準エラー出力のログと失敗の通知に出ないことは、既存の `requireNoSecret` と同じ手段で確かめる。標準出力を稼働中に読む検査（AC-07）のために、標準出力の書き込みと読み出しを同期する（§1.3）。

## 2. 実装ステップ (Implementation Steps)

各ステップの末尾に **対象:**（そのステップが触るファイル）と **完了:**（そのステップの検証）を記す。フェーズの「完了条件」は、そのフェーズのすべてのステップの後に確かめる。

### フェーズ 1: 起動時の表示

**対象ファイル**
-   変更: `cmd/yt2column-server/run.go`・`main.go`、`internal/authtoken/authtoken.go`・`authtoken_test.go`、`internal/server/test_helpers.go`、`cmd/yt2column-server/run_test.go`・`server_test.go`・`test_helpers.go`

**タスク**
-   [ ] **ステップ 1-1**: `internal/authtoken` の `LoadOrCreate` と `create` の戻り値から `created` を外し、`(Token, error)` にする（§1.3 の名前の台帳。設計書 3.2）。両者の doc コメントから `created` の記述を外す。続けて `run.go` の手順 2 の表示を、`created` で分岐せず起動のたびに行う（`token, err := authtoken.LoadOrCreate(...)`。設計書 3.1）。表示の内容（値 + 改行）と表示先（標準出力）は変えない。同じステップで、新しい署名に合わせて `internal/authtoken/authtoken_test.go` から `created` の検査を外し（§1.3）、`internal/server/test_helpers.go` の `newTestToken` の `created` の検査を `token.Matches(testTokenValue)` の検査に置き換える（§1.3）。
    -   **対象:** `internal/authtoken/authtoken.go`・`authtoken_test.go`、`internal/server/test_helpers.go`、`cmd/yt2column-server/run.go`。**完了:** `make fmt` と `go build ./...` が通り、`go test -tags test ./internal/authtoken ./internal/server` が通る。`TestTokenShownOnlyWhenCreated` は、このステップの後は失敗する。`TestStartupFailures` の表示失敗の行は、ステップ 1-2 の後に失敗する（いずれもステップ 1-6 で改める。§1.2・§3.2）。
-   [ ] **ステップ 1-2**: 表示の失敗を best-effort にする。標準出力への書き出しに失敗したら（閉じたパイプへの書き込みを含む）、値を含まない 1 行を `redact.Line` を通して標準エラー出力に書き、起動を続ける（`exitUsage` で終えない）。値の取り出し（`token.Value().Reveal()`）の失敗は `exitUsage` のまま残す（設計書 3.1・4）。値の取り出しの失敗の 1 行（`run.go:175`）と書き出しの失敗の 1 行（`run.go:179`）の「created」を前提にした文言を、起動のたびに表示する振る舞いに合わせて改める（§1.3）。
    -   **対象:** `cmd/yt2column-server/run.go`。**完了:** `make fmt` が通る。
-   [ ] **ステップ 1-3**: 案内（標準エラー出力に書く 1 行）を、起動の種類で分けず常に同じ文言にする。値を含めず、「標準出力に表示したこと」「貼り付け」「起動のたびに表示すること」「ファイルのパス」を述べ、「値が再び表示されない」とする記述を含めない（設計書 3.1・3.4）。`run` の doc コメントと手順 2 のコメントを、起動のたびに表示する形に改める（設計書 3.1）。
    -   **対象:** `cmd/yt2column-server/run.go`。**完了:** `make fmt` が通り、表示の 1 行のあとに値を含まない案内が出ることを目視で確かめる。
-   [ ] **ステップ 1-4**: `main.go` で、`run` を呼ぶ前に `signal.Ignore(syscall.SIGPIPE)` を呼ぶ。SIGPIPE を無視すると、ファイル記述子 1・2 への閉じたパイプの書き込みはプロセスを終了させず、書き込みが `syscall.EPIPE` を返すので、ステップ 1-2 の失敗の扱いに届く（ランタイムの `sigpipe`（`$(go env GOROOT)/src/runtime/signal_unix.go:334-339`）は `signal_ignored(_SIGPIPE)` のとき終了しない。2026-10-16、Go 1.27.1 で確認）。`signal.Notify` で購読してチャネルを読まない形と比べ、読まれないチャネルを残さず 1 回の呼び出しで済む。無視は子プロセスに引き継がれるが、サーバは子プロセスを起動しないので影響しない。
    -   **対象:** `cmd/yt2column-server/main.go`。**完了:** `make fmt` と `go build ./...` が通り、`TestClosedStdoutPipeKeepsServing`（ステップ 2-4）で確かめる。
-   [ ] **ステップ 1-5**: `internal/authtoken/authtoken.go` の `Token.Value` のコメント（`:132-134`）を、起動のたびの表示に合わせて改める。ステップ 1-1 の署名の変更のほかは、`LoadOrCreate` の振る舞いと実装を変えない（設計書 3.2）。
    -   **対象:** `internal/authtoken/authtoken.go`。**完了:** `make fmt` が通り、`internal/authtoken` のテストが通る。
-   [ ] **ステップ 1-6**: 振る舞いの変更で更新が要る既存のテストを改める（設計書 1.4・7）。
    -   `TestTokenShownOnlyWhenCreated` を `TestTokenShownOnEveryStart` に改称し、2 回目の起動も標準出力に値の 1 行を書き、ファイルが変わらないことを確かめる形にする（§1.3 の名前の台帳。AC-01〜AC-03）。加えて、2 回目の起動の標準エラー出力の案内が、値を再び表示しないとする記述（旧文言の「not shown again」など）を含まず、トークンのファイルのパスを含むことを確かめる（AC-02）。実装する文言に合わせて、この検査の禁止する語句を決める。
    -   `TestStartupFailures` から「token not written to stdout」の行と、`stdoutFails` のフィールド・分岐を外す（§1.3 の名前の台帳）。`tokenShown` の説明と「port in use」の行のコメント（`run_test.go:159-160`）を起動のたびの表示に合わせて改める。他の行は維持する（AC-04・AC-05・AC-06）。
    -   `server_test.go:26` のコメントを改める。
    -   `startServer` を、試行ごとに標準出力のバッファを新しくし、待ち受けた試行の標準出力だけを返す形に改める。標準出力を、書き込みと読み出しを同期するバッファにする（§1.3）。
    -   **対象:** `cmd/yt2column-server/run_test.go`・`server_test.go`・`test_helpers.go`。**完了:** `make test` が通る。
-   [ ] **ステップ 1-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。2 回目の起動で表示を書かない形に壊す（例: 手順 2 の前にトークンのファイルがあったときは表示しない。`TestTokenShownOnEveryStart` の 2 回目の起動）。案内を「値が再び表示されない」とする旧文言に戻す（`TestTokenShownOnEveryStart` の案内の検査）。`newTestToken` の新しい検査は、`LoadOrCreate` に渡すファイルの値を `testTokenValue` と異なる値にして失敗することを確かめる。`internal/authtoken/authtoken_test.go` から `created` の検査を外したことについて、`go tool cover -func` の結果（`internal/authtoken`）が関数ごとに変わらないことを確かめ、コミットメッセージに記す（CLAUDE.md「Deleting a test is a claim that must be checked」）。続けて `make fmt` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make fmt` → `make test` → `make lint` が通り、`TestTokenShownOnEveryStart`・`TestStartupFailures` が通る。

### フェーズ 2: テスト

**対象ファイル**
-   変更: `cmd/yt2column-server/run_test.go`・`server_test.go`・`test_helpers.go`

ステップ 2-3・2-4 は、フェーズ 1 と同じファイルを触るが、フェーズ 1 の完了後（`make test` が通った状態）に、新しい検査を加えるステップである。

**タスク**
-   [ ] **ステップ 2-1**: `test_helpers.go` に、標準出力への書き出しに失敗する `io.Writer` を渡して起動できる補助を加える（`startServer` の再試行・後始末の処理を共有し、標準出力のバッファを受け取れる形にする。設計書 7）。`requireNoSecret`・`writeTokenFile`・`failingWriter` を再利用し、新しい依存を足さない。
    -   **対象:** `cmd/yt2column-server/test_helpers.go`。**完了:** `make fmt` と `go vet -tags test ./cmd/yt2column-server` が通る。
-   [ ] **ステップ 2-2**: `test_helpers.go` に、子プロセスの標準出力として、読み手を閉じたパイプの書き込み側を渡して起動できる形を `startServerChild` に加える（設計書 7）。読み手は子の起動（`cmd.Start`）より前に閉じる（子の最初の書き込みが必ず閉じたパイプへ届くようにする）。親が持つ書き込み側は `t.Cleanup` で閉じる。子の環境と待ち受けの判定は既存の処理をそのまま使う。
    -   **対象:** `cmd/yt2column-server/test_helpers.go`。**完了:** `make fmt` が通る。
-   [ ] **ステップ 2-3**: `TestDisplayFailureKeepsServing` を加える（要件書 4.3、設計書 7）。トークンのファイルがある状態とない状態（初回の起動）の 2 通りで、書き出しに失敗する標準出力を渡して起動し、標準エラー出力に失敗の 1 行（失敗の理由とファイルのパスを含み、`requireNoSecret` を通る）が出ること、サーバがリクエストを受理すること、停止したときの終了コードが `exitOK` であることを確かめる。ファイルがある状態ではファイルが書き換えられないことを、ない状態ではファイルが作られていること（削除する「token not written to stdout」の行が確かめていた点）を確かめる。
    -   **対象:** `cmd/yt2column-server/run_test.go`。**完了:** このテストが通り、表示に失敗しても待ち受ける。
-   [ ] **ステップ 2-4**: `TestClosedStdoutPipeKeepsServing` を加える（SIGPIPE。設計書 7）。SIGPIPE はプロセス全体に働くので、子プロセスのテスト（`childModeMain`。本番の `main` をそのまま動かす）で確かめる。子の標準出力に、読み手を閉じた `os.Pipe` の書き込み側を渡して起動し、子が待ち受けを開始すること、標準エラー出力に表示の失敗の 1 行（`requireNoSecret` を通る）が出ること、`SIGTERM` で停止したときの終了コードが `exitOK` であることを確かめる。
    -   **対象:** `cmd/yt2column-server/run_test.go`。**完了:** このテストが通り、子が閉じたパイプで終了しない。
-   [ ] **ステップ 2-5**: `server_test.go` の `TestServerEndToEnd` に、標準出力に表示された 1 行だけを取り出して `Authorization: Bearer` に使う検査を加える（AC-07。設計書 7）。稼働中のサーバの標準出力から、同期したバッファを通して表示された行を取り出し（§1.3）、その値で認証が通ることを確かめる。`e.readToken(t)`（トークンのファイルの内容）ではなく、表示された行を使う。
    -   **対象:** `cmd/yt2column-server/server_test.go`。**完了:** `TestServerEndToEnd` が通り、`-race` で競合を報告しない。
-   [ ] **ステップ 2-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。表示に失敗すると `exitUsage` で終える形に戻す（`TestDisplayFailureKeepsServing`）。閉じたパイプの扱い（`signal.Ignore`）を外す（`TestClosedStdoutPipeKeepsServing`。子が SIGPIPE で終了して失敗する）。表示された 1 行の代わりに別の値を使う（`TestServerEndToEnd` の表示からの認証）。続けて `make fmt` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make fmt` → `make test` → `make lint` が通り、ステップ 2-3〜2-5 のテストが通る。

### フェーズ 3: 文書

**対象ファイル**
-   変更: `README.md`、`docs/dev/security.md`、`docs/dev/developer_guide/package_reference.md`、`cmd/yt2column-server/docs_test.go`

**タスク**
-   [ ] **ステップ 3-1**: README を設計書 3.4 の `README.md` の行のとおりに改める（F-002）。「Running the server」節の「(on the first start only)」を、起動のたびに標準出力へ表示される形に改める。「The shared token」節の「prints it once」「does not print it」を改め、値を再び確認する主たる経路を「起動し直す」とし、ファイルを読む手順を表示に失敗したときの補足として残す。標準出力をファイルにリダイレクトするとトークンがそのファイルに残る注意を加え、`2>>server.log` の起動例（標準エラー出力だけをファイルに残す形）を維持する。
    -   **対象:** `README.md`。**完了:** `make test`（README を読む Go のテスト）が通る。
-   [ ] **ステップ 3-2**: `security.md` §8.2 の「表示と作り直し」の記述を、生成した起動だけでなく起動のたびに表示する形に改める（設計書 3.4。F-002）。露出の経路（標準出力、端末のスクロールバック、標準出力をリダイレクトしたファイル、起動した呼び出し元による出力の捕捉）、ログへの非露出、標準出力と標準エラー出力の分離、表示に失敗しても起動を続けることを記す。同じ節の「拡張への受け渡し」の行（`security.md:120`）の「生成時に表示されたトークン」も、起動のたびの表示に合わせて「起動時に表示されたトークン」に改める。「生成」の行（`security.md:116`）の「`created=false` を返し」は、ステップ 1-1 で外した戻り値に触れているので「その値を返し」に改める（いずれも §1.3 の名前の台帳）。見出し「### 8.2. localhost サーバ（#109）」は変えない（`docs_test.go` の `sectionByHeading` が読むため）。
    -   **対象:** `docs/dev/security.md`。**完了:** `make test`（`security.md` を読む Go のテスト）が通る。
-   [ ] **ステップ 3-3**: `package_reference.md` の `cmd/yt2column-server` の行の 2 か所を、§1.3 の名前の台帳のとおりに改める（設計書 3.4）。この行の文面は `internal/pipeline` のガードが固定しない（`TestPackageReferenceListsPackages` はパッケージの行の有無だけを確かめる）ので、文面の正しさはレビューで確かめ、§5 の AC には数えない。
    -   **対象:** `docs/dev/developer_guide/package_reference.md`。**完了:** `make test`（`internal/pipeline` の `TestPackageReferenceListsPackages` ほか）が通る。
-   [ ] **ステップ 3-4**: `docs_test.go` の `TestServerDocsContract` に、README・`security.md` §8.2 の新しい記載を固定する検査を加える（AC-08・AC-09。設計書 3.4・7）。既存の検査（`tokenName`・`0600`・`O_NOFOLLOW`・`link(2)` など）は維持する。照合する語句は実装で決め、文書の書きぶりに合わせる。README は「起動のたびの表示」「貼り付け」「再確認（起動し直す）」「ファイルを読む補足」「リダイレクトの注意」を、`security.md` §8.2 は「起動のたびの表示」「露出の経路」「ログへの非露出」「標準出力と標準エラー出力の分離」を含むことを確かめる。
    -   **対象:** `cmd/yt2column-server/docs_test.go`。**完了:** `make test` が通る。
-   [ ] **ステップ 3-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。README の「on the first start only」を戻す。`security.md` §8.2 を「生成した起動だけ表示する」に戻す。`security.md:120` の「起動時に表示されたトークン」を「生成時に表示されたトークン」に戻す。ステップ 3-4 の確かめる語句を含む文を README・`security.md` から 1 つずつ消す（いずれも `TestServerDocsContract`）。`security.md:120` を戻すことが検出されない場合は、その語句の検査をステップ 3-4 に加える。続けて `make fmt` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make fmt` → `make test` → `make lint` が通り、README・`security.md` §8.2 の記載が `TestServerDocsContract` で固定されている。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1: 起動時の表示 | フェーズ 1 | `TestTokenShownOnEveryStart`・`TestStartupFailures` が通り、`make test && make lint` が通る |
| M2: テスト | フェーズ 2 | 表示失敗・閉じたパイプ・AC-07 のテストが通り、`make test && make lint` が通る |
| M3: 文書 | フェーズ 3 | `TestServerDocsContract` と `make test` が通る |

### 3.2. 実装順序の根拠

設計書 8 章の順序（起動時の表示 → テスト → 文書）に従う。フェーズ 1 では、振る舞いの変更で更新が要る既存のテスト（`TestTokenShownOnEveryStart`・`TestStartupFailures`）を同じフェーズに含める。設計書 8 章はこれらを「テスト」の段階に置くが、表示の条件を変えると同じコミットで既存のテストが失敗するので、グリーンゲートを保つためフェーズ 1 に含める。フェーズ 2 の残りのテストは、新しい検査（表示失敗・SIGPIPE・AC-07）だけを加える。文書（フェーズ 3）は、実装した振る舞いを記すので最後に行う。

各フェーズのグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make fmt` を先に加えた `make fmt` → `make test` → `make lint` とする。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   実際のブラウザを起動せず、LLM の API も Slack の Webhook も呼ばない（要件書 §5、`0011` の制約を維持する）。表示の検査は、`run` の標準出力・標準エラー出力の引数に `bytes.Buffer` または書き出しに失敗する `io.Writer` を渡して行う（要件書 4.5、設計書 7）。
-   トークンが標準出力の 1 行だけに載り、標準エラー出力のログと失敗の通知に出ないことは、既存の `requireNoSecret`（`test_helpers.go:669-678`）と同じ手段で確かめる（設計書 7）。
-   表示の判定の網羅は `cmd/yt2column-server` のテストで行う。`TestTokenShownOnEveryStart` は起動のたびの表示・ファイルの不変・案内を確かめ、表示失敗のテストと閉じたパイプのテストは起動を続けることを確かめ、`TestServerEndToEnd` は表示された値での認証を確かめる。トークンの生成・読み込みの検査は `internal/authtoken` のテストに残し、`cmd/yt2column-server` で再実装しない（設計書 7）。
-   閉じたパイプ（SIGPIPE）はプロセス全体に働くので、`run` を同じプロセスで呼ぶテストでは確かめられない。子プロセスのテスト（`childModeMain`）で確かめる（設計書 7）。

### 4.2. 統合テスト

-   `TestServerEndToEnd`（`server_test.go`）は、fake の LLM クライアントと、loopback の Webhook に投稿する実際の publisher・notifier を使う。本計画は、そのうち表示された値での認証の検査だけを加える（AC-07）。

### 4.3. 後方互換性

-   トークンの生成・保存・読み込み・検査と、リクエストの認証の振る舞いを変えない。`internal/authtoken` のテストのうち、`created` 以外の検査（ファイルの作成と権限、同じ値を読むこと、不正なファイルの拒否、上書きしないこと）を変更なく通すことで確かめる。`LoadOrCreate` の戻り値から `created` を外すのは、リポジトリの外に利用者がいないため互換性の問題にならない（CLAUDE.md「Declare, don't infer」の「the project has no consumers outside the repository」）。
-   `TestStartupFailures` の「an argument」「model and webhook missing」「token file readable by others」「LLM client not built」ほか（表示失敗の行を除く）を変更なく通すことで、設定の読み込み・不正なファイル・部品の構築の各停止の振る舞いを保つことを確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベル: `test` は失敗しうる実行のテスト、`static` は文書を確かめるガードのテストである。場所は `<path>::<テスト名>` で示す。テストの名前は計画上の名前である（§1.2）。「実装」の列は、実装のステップとテストを作るステップを挙げる。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `cmd/yt2column-server/run_test.go::TestTokenShownOnEveryStart`（初回の起動）、`cmd/yt2column-server/server_test.go::TestServerEndToEnd` | ステップ 1-1・1-6 |
| AC-02 | test | `cmd/yt2column-server/run_test.go::TestTokenShownOnEveryStart`（2 回目の起動。標準出力 1 行・ファイル不変・案内がトークンのファイルのパスを含み、値を再び表示しないとする語句を含まない） | ステップ 1-1・1-3・1-6 |
| AC-03 | test | `cmd/yt2column-server/run_test.go::TestTokenShownOnEveryStart`、`cmd/yt2column-server/server_test.go::TestServerEndToEnd`（2 回目の起動の認証） | ステップ 1-1・1-6 |
| AC-04 | test | `cmd/yt2column-server/run_test.go::TestStartupFailures`（「token file readable by others」）、`internal/authtoken/authtoken_test.go::TestLoadOrCreateRejectsInvalidFile`・`::TestLoadOrCreateRejectsWrongOwner`（不正なファイルの各ケース） | 変更なし（既存） |
| AC-05 | test | `cmd/yt2column-server/run_test.go::TestStartupFailures`（「model and webhook missing」） | 変更なし（既存） |
| AC-06 | test | `cmd/yt2column-server/run_test.go::TestOutputsOmitSecrets`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（`requireNoSecret` の検査）、`cmd/yt2column-server/run_test.go::TestDisplayFailureKeepsServing`、`internal/server/response_test.go::TestResponseOmitsInputs`、`internal/server/job_test.go::TestNotifyNoticeFields` | ステップ 1-2・2-3 |
| AC-07 | test | `cmd/yt2column-server/server_test.go::TestServerEndToEnd`（稼働中に標準出力から同期して取り出した 1 行を認証に使う） | ステップ 2-5 |
| AC-08 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（README の検査を追加） | ステップ 3-1・3-4 |
| AC-09 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`security.md` §8.2 の検査を追加） | ステップ 3-2・3-4 |

-   `TestDisplayFailureKeepsServing`（ステップ 2-3）と `TestClosedStdoutPipeKeepsServing`（ステップ 2-4）は、要件書 4.3 に対応するが、単独の AC 番号を持たない。両テストは `TestStartupFailures` の表示失敗の行を外した後に加える（§2 のフェーズ 2）。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| Go のランタイムは、標準出力・標準エラー出力（ファイル記述子 1・2）への閉じたパイプの書き込みで、既定ではプロセスを SIGPIPE で終了させる（`go doc os/signal` の「SIGPIPE」節。2026-10-16、Go 1.27.1）。閉じたパイプでも起動を続けるには、この終了を抑える必要がある | 表示に失敗しても起動を続けるという設計書 1.1 原則 2 が、閉じたパイプの場合に成り立たない | ステップ 1-4 で、`run` の前に `signal.Ignore(syscall.SIGPIPE)` を呼ぶ。ステップ 2-4 の子プロセスのテストで確かめる |
| `startServer` が試行ごとに標準出力を同じバッファへ書き足すと、待ち受けの再試行のときに標準出力が複数行になり、検査が偶発的に失敗する | `TestTokenShownOnEveryStart` と AC-07 の検査が不安定になる | ステップ 1-6 で、試行ごとに標準出力のバッファを新しくし、待ち受けた試行の標準出力だけを返す形に改める（§1.3） |
| AC-07 は、サーバの稼働中に `run` の書き込む標準出力を読む。`runningServer.stdout` は `stop` の後に読む取り決めで（`test_helpers.go:228`）、同期しないと `-race` で競合になる | AC-07 の検査が競合として失敗する | ステップ 1-6 で、標準出力の書き込みと読み出しを同期するバッファにし、稼働中に取り出せるようにする（§1.3） |
| 案内の文言を実装で決めるため、AC-02 の検査は、実装が選んだ文言に依存する | 文言を変えたときに検査が追従しない | ステップ 1-6 で、案内がトークンのファイルのパスを含み、値を再び表示しないとする語句（実装した文言に合わせて決める）を含まないことを確かめる。ステップ 1-7 で、案内を旧文言に戻すとこの検査が失敗することを確かめる |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| 文書のガードのテスト（ステップ 3-4）の照合語句は、README・`security.md` の書きぶりに合わせる必要がある | 文書（ステップ 3-1・3-2）を先に改め、その文面に合わせてステップ 3-4 の検査を書く |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1 完了（ステップ 1-1〜1-7）
-   [ ] フェーズ 2 完了（ステップ 2-1〜2-6）
-   [ ] フェーズ 3 完了（ステップ 3-1〜3-5）
-   [ ] §5 のすべての AC の検証が通り、各テストを壊して失敗することをコミットメッセージに記録している

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の `test`・`static` の検証がすべて通る。起動のたびにトークンが標準出力に 1 行で表示され、表示に失敗しても待ち受ける。
-   **品質:** `make fmt`・`make test`・`make lint` が通る。各テストを壊して失敗することを確かめ、コミットメッセージに記録している。
-   **セキュリティ:** AC-06 の検証が通り、共有トークンが標準出力への表示とトークンのファイルを除き、ログ・エラーメッセージ・応答・失敗の Slack への投稿に現れない。`internal/authtoken` の生成・検査と、リクエストの認証を変えていない。
-   **文書:** ステップ 3-1〜3-3 の文書を改め、`TestServerDocsContract` と `make test` が通る。

## 9. 次のステップ (Next Steps)

-   本計画をレビューし、`approved` にする。
-   `/mkplan2 0021` で PR の境界を本計画に埋め込む。
-   その後、`/runplan 0021` でフェーズ 1 から実装する。
