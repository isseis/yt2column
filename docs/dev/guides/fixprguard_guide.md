# fixprguard 利用ガイド

`fixprguard` は、`/fixpr` と `/runpr` のための開発者向けのガードツールである。レビュースレッドの著者の判定、修正の変更範囲の判定、git の状態の比較、commit と push、GitHub への返信と PR の説明の更新を、手順書を読むセッションやエージェントに任せず、決定的なコードで行う。本書では、前提となる環境、インストール、各サブコマンドの使い方、範囲の規則、ツールが保証すること・しないこと、止まったときの対処を説明する。

設計の経緯と根拠は、タスク 0030 の設計書（`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`）にある。

## 1. 位置付け

レビュースレッドの本文は、PR に書き込める誰でも書けるので、信頼できないテキストである。`fixprguard` は、次の 3 つの守りの層をコードで強制する。

1. **著者の判定。** すべてのコメントの著者が信頼する著者であるスレッドだけを処理する。信頼する著者は、自動レビュアー（`--reviewer` で渡した名前のどれかの Bot。`/fixpr`・`/runpr` は、`.claude/commands/_context.md` の `Automated reviewer list` のすべての名前を渡す）と、リポジトリへの書き込み権限（`role_name` が `write`・`maintain`・`admin`）を持つ人である。想定外の著者のスレッドの本文は取得しない。
2. **区切りのブロック。** エージェントのプロンプトに埋め込む信頼できないテキストを、実行ごとの乱数（nonce）を含む区切りの行で囲む。
3. **変更範囲と git の状態の確認。** 修正が、処理したスレッドの範囲の外のファイルや、git の hooks・config、git が無視するファイルのうち監視の一覧（7.1）に当たるもの（対象のファイル）を変えていないことを、commit と push の前に確かめる。config では、今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定は比べない。並行するセッションがほかの worktree でブランチを操作しても止まらないようにするためである。設定の値の一覧を変えない編集（コメント・空行・字下げだけの編集）も検出しない。ほかのブランチの設定の書き換えは、残余のリスクである（7.2）。

役割分担は次のとおり。

| 担当 | 内容 |
|---|---|
| `fixprguard` バイナリ（`cmd/fixprguard`、`internal/fixprguard`） | 判定、取得、比較、commit・push、GitHub への書き込み |
| `/fixpr`・`/runpr` コマンド（`.claude/commands/`） | サブコマンドを呼ぶ順序、利用者への確認、エージェントの起動 |
| 開発者 | 範囲外の変更や想定外の著者のスレッドについての判断、ツールが止まったときの対処 |

ツールは状態のファイルを持たない（`begin`・`inspect`・`finish`・`push` が git のディレクトリに書く記録のファイル（8.1）は利用者のためのもので、ツールは読み返さない）。サブコマンドが出力した基準値や判定した集合の識別子は、セッションが保持し、次のサブコマンドの引数で渡す。

## 2. 前提条件

| 項目 | 内容 |
|---|---|
| `git`、`gh`、`make` | `PATH` 上にあり、`gh auth login` 済みであること |
| git の版 | 2.12 以降。2.12 は `protocol.<name>.allow` を読む最初の版で、これより古い git は、ツールが付ける `-c protocol.ext.allow=never` を黙って無視する |
| 作業ディレクトリ | 作業ツリーの最上位で、HEAD がブランチにいる状態で実行すること。最上位でないと、`begin` は終了コード `2` で、`inspect`・`finish`・`push` は `git-state-changed`（`config`）で止まる。detached HEAD では `begin` が終了コード `2` で止まる |
| `origin` | GitHub 上の本リポジトリを指していること。`gh repo view` がプロンプトなしで本リポジトリを返すこと |
| `GIT_*` の環境変数 | ツールは、起動する git と `gh` から取り除く |
| `GH_REPO`・`GH_HOST` | 設定されていないこと。設定されていると、`gh` を呼ぶサブコマンドは `gh` を起動せずに終了コード `2` で止まる（`gh` が別のリポジトリを対象にするため） |

## 3. インストール

最新の `main` をチェックアウトした状態で、次を実行する。

```sh
make install-fixprguard
```

`$(go env GOPATH)/bin` に `PATH` が通っていることを確認する。

ツールを変更した PR がマージされたら、`main` を更新してから再インストールする。コマンドの文書は `--protocol` で版を照合するが、同じ `protocol` の番号のまま規則が変わる変更は検出できない。インストール済みのバイナリが作られたコミットは、次のコマンドの `vcs.revision` で確かめられる。

```sh
go version -m "$(command -v fixprguard)"
```

1 つのバイナリが、同じ利用者のすべての worktree で共有される。一方、`/fixpr`・`/runpr` は、その worktree の checkout にあるコマンドの文書を読み、そこに書かれた `--protocol` を渡す。そのため、`protocol` の番号を上げる変更（例: タスク 0035 で `1` から `2`）では、次の手順に従う。

- その PR の `/runpr` の前に、その PR のブランチで再インストールする。その PR のコマンドの文書が新しい番号を渡すためである。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は、終了コード `2` で止まる。
- マージの直後に、マージの後の `main` で再インストールする。再インストールの前に新しい番号の文書から実行すると、終了コード `2` で止まる。
- 作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。取り込む前は、そのブランチの文書が古い番号を渡すので、終了コード `2` で止まる。

どちらも、古い規則で判定することはない（fail-closed）。

## 4. 共通の約束

| 項目 | 内容 |
|---|---|
| `--protocol N` | すべてのサブコマンドに必須。`N` がバイナリの番号と一致しなければ、終了コード `2` で止まる（古いバイナリが新しい文書の想定で判定するのを防ぐ）。現在の番号は `4` |
| `--reviewer LOGIN` | `list`・`begin`・`inspect`・`reply` に 1 回以上必須で、ほかのサブコマンドには渡せない。信頼する自動レビュアーの GraphQL の `author.login`（例: `coderabbitai`）を、名前ごとに 1 回ずつ繰り返して渡す（例: `--reviewer <login-1> --reviewer <login-2>`）。`/fixpr`・`/runpr` が渡す名前は、`.claude/commands/_context.md` の `Automated reviewer list` のすべての名前である。渡す順序は判定に関係しない。1 つもないとき、同じ名前を 2 回以上渡したとき、名前の文法に合わないときは、使い方の誤り（終了コード `2`）になる。既定値はない |
| `[PR]` | 10 進数の PR 番号だけを受け付ける。URL やブランチ名は拒否する。省略すると、現在のブランチの PR を `gh pr view` で求める。GitHub を読まない `finish` は省略できない |
| フラグの位置 | フラグは位置引数（`PR`・`FILE`）の前に書く。同じフラグを 2 回渡すと拒否する（`--accept`・`--thread`・`--reviewer` は複数回渡せる。ただし、`--reviewer` に同じ名前を 2 回渡すと拒否する） |
| 標準出力 | 1 行の JSON（`fence` だけは区切りのブロックそのもの）。すべての JSON に `protocol` が入る |
| 標準エラー | 1 行の診断。拒否した規則の名前だけを示し、GitHub から来た値や外部コマンドの引数・標準エラーは含めない |

終了コードは次のとおり。呼び出し側は、これ以外の終了コード、信号による終了、JSON として読めない出力を、すべて「止める」として扱う。

| 終了コード | 意味 | 標準出力 |
|---|---|---|
| `0` | 続けてよい | サブコマンドの結果 |
| `1` | 止める | `stop`（止まった種類）と、種類ごとの詳細（8 章） |
| `2` | 使い方の誤り（フラグ、値の文法、`--protocol` の不一致、`GH_REPO`・`GH_HOST`、空の位置引数）。ほかに、作業ツリーの最上位でない・detached HEAD・`.git` ファイルの形の誤り（`begin`）と、`--comment-id` がスレッドの最初のコメントでないこと（`reply`。GitHub を読んだ後に分かる）も `2` になる。`reply` の本文、`edit-pr` のタイトルか本文が `@codex` のメンションを含むことも `2` になる（GitHub に書き込む前に分かる。5 章）。診断が `flag given twice: --reviewer` を含むときは、インストール済みのツールが `--reviewer` の繰り返しに対応する前のものである。`make install-fixprguard` で再インストールすれば直る。インストール済みのツールが作られたコミットは、`go version -m "$(command -v fixprguard)"` の `vcs.revision` で確かめられる（3 章） | なし |
| `3` | 利用者に確かめる（`inspect` だけ。範囲外の変更がある） | `inspect` の結果 |

## 5. サブコマンド

各サブコマンドは単独でも実行できる。拒否の理由を調べるときは、コマンドの文書と同じ引数で、このとおりに実行する。

### `head [PR]`

PR の head がこのリポジトリのブランチであることを確かめる（`isCrossRepository` が真偽値の `false` で、`headRepository` があるときだけ）。fork の PR は `cross-repository` で止まる。`/runpr` は、`gh pr checkout` の前にこれを実行する。

```sh
fixprguard head --protocol 4 42
```

出力: `{"protocol":4,"sameRepository":true}`。副作用: GitHub の読み取りだけ。

### `list --reviewer LOGIN ... [PR]`

未解決のスレッドを全ページ取得し（最大 2000 件。超えると `thread-cap` で止まる）、スレッドごとの信頼の判定と理由、指紋を出力する。本文は取得も出力もしない。最初に `head` と同じ確認をする。

```sh
fixprguard list --protocol 4 --reviewer coderabbitai 42
```

出力: `threads`（各要素は `threadId`・`url`・`trusted`・`fingerprint`・`findings`）と `permissionLookups`（権限の照会の使用回数 `used` と残り `remaining`）。`findings` の `reason` は次のいずれかである。

| `reason` | 意味 | 付く項目 |
|---|---|---|
| `no-write-permission` | 書き込み権限のない利用者 | `account` |
| `other-bot` | `--reviewer` で渡したどの名前とも一致しない Bot | `account` |
| `no-author-name` | 著者がない、名前が空、または名前の文法に合わない | `commentUrl` |
| `permission-unavailable` | 権限を照会できなかった | `cause`（`not-found`・`forbidden`・`rate-limited`・`budget-exceeded`・`rejected-response`・`failed`） |
| `incomplete-comments` | コメントが 100 件を超え、全部を取得できない | なし |
| `unknown-author-kind` | 著者の種類が `User` でも `Bot` でもない | なし |

### `begin --reviewer LOGIN ... [--accept ID=FINGERPRINT ...] [PR]`

`/fixpr` のフェーズ 1。次の順に行う。(1) 作業ツリーに未コミットの変更（未追跡のファイルを含む）がないこと（あれば `dirty-work-tree` で止まり、GitHub に問い合わせない）。(2) `head` と同じ確認。(3) 基準値の採取（対象のファイルの状態を含む）と、`begin` の記録のファイルの書き込み。(4) 最初の 1 ページ（100 スレッド）の判定と、処理するスレッドの本文の取得。

処理するスレッドは、信頼する著者のスレッドと、`--accept` で名指しされて指紋が現在と一致するスレッドである。`--accept` には、利用者に示した報告の出力にあるスレッド ID と指紋を渡す。

```sh
fixprguard begin --protocol 4 --reviewer coderabbitai 42
```

出力の項目:

| 項目 | 内容 |
|---|---|
| `baseline` | 基準値。`inspect`・`finish`・`push` に `--baseline` で渡す。`.` で分けた 9 番目の欄は対象のファイルの状態をまとめた digest、10 番目の欄は `begin` の記録のファイルの SHA-256 である（8.1） |
| `number`・`capHit` | PR 番号。`capHit` が真なら、2 ページ目以降のスレッドは今回の対象外 |
| `processCount` | 処理するスレッドの数。`0` なら処理するものはない |
| `untrusted` | 処理しない想定外の著者のスレッド（`threadId`・`url`・`fingerprint`・`findings`）。本文はない |
| `changed` | コメントが変わったため処理しないスレッド（`stage` が `since-accept` なら名指しの後、`since-judgment` なら判定と本文の取得の間に変わった） |
| `acceptNotFound` | 名指しされたが、未解決のスレッドに見つからなかった ID |
| `permissionLookups` | 権限の照会の使用回数と残り |
| `nonce`・`threads` | `threads` は、処理するスレッドの配列（`threadId`・`databaseId`・`authorType`・`body`・`path`・`line`・`url`・`fingerprint`・`named`）を 1 行の JSON にし、`BEGIN-UNTRUSTED-<nonce>` と `END-UNTRUSTED-<nonce>` の行で囲んだ文字列。エージェントのプロンプトにはこの文字列をそのまま埋め込む |

副作用: GitHub と `git ls-remote` の読み取りと、作業ツリーごとの git のディレクトリ（main のチェックアウトでは `.git`、linked worktree では、その `.git` ファイルの `gitdir:` の行が示すディレクトリ（`<main のチェックアウト>/.git/worktrees/<name>`））への `begin` の記録のファイル `fixprguard-ignored-begin` の書き込み（置き換え）。前のラウンドの記録のファイル（`fixprguard-ignored-begin` と止まった時点の記録 `fixprguard-ignored-stop`）は、対象のファイルの状態を求め直す前に消す。記録のファイルは、対象のファイルごとに、そのファイルの状態の digest と、ASCII に引用したパスを 1 行に書く。基準値を求めた後に対象のファイルが変わっていれば、記録を書かずに `git-state-changed`（`ignored`）で止まる。作業ツリーと対象のファイルは変えない。

### `fence FILE`

ファイルの内容を区切りのブロックで囲んで、そのまま標準出力に書く（JSON ではない）。nonce は、ファイルに含まれない値を選び直す。通常のファイル（シンボリックリンク、FIFO、デバイスでない）で 16 MiB 以下のものだけを読む。`/fixpr` は、トリアージのエージェントの出力をファイルに書き、これで囲んでから修正のエージェントに渡す。

```sh
fixprguard fence --protocol 4 /path/to/triage.json
```

副作用: なし。

### `inspect --reviewer LOGIN ... --baseline TOKEN [--thread ID=FINGERPRINT ...] [PR]`

修正のフェーズの後に実行する。まず config・info・hooks と対象のファイルを基準値と比べ、違えば `gh` を起動せずに `git-state-changed` で止まる。比べる順序は次のとおりである。(1) git のディレクトリの場所、config のファイルの種類、info を、git を起動せずにファイルだけで比べる。(2) `git config --list` で git が読む設定の値の一覧を読み、今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定を除いて比べる。(3) hooks を比べる。(4) 対象のファイル（7.1）の状態を比べる。列挙の git が config を読むので、config と hooks が一致した後に比べる。違えば `parts` は `ignored` だけである。設定の値の一覧を変えない編集（コメント・空行・字下げだけの編集）は検出しない。ほかのブランチの設定の書き換えは、残余のリスクである（7.2）。次に、`--thread` のスレッドを GitHub から取り直して `path` を求め（指紋が違えば `thread-changed`）、作業ツリーの変更を数え上げて、範囲（6 章）と照らす。

```sh
fixprguard inspect --protocol 4 --reviewer coderabbitai --baseline 'v3....' --thread 'PRRT_...=<指紋>' 42
```

出力: `judgedSet`（判定した集合の識別子。`finish` に渡す）、`changes`（変更のあったパスの数）、`outOfScope`（範囲外の変更。各要素は `position`・`path`・`kind`（`add`・`modify`・`delete`・`other`）・`added`・`deleted`・`binary`）。`outOfScope` が空でなければ終了コード `3` で、利用者に一覧を示して続けるかを確かめる。パスの検査（6 章）に通らないパスは `path` を出さず、`position` だけで示す。

副作用: GitHub と git（`git ls-remote` を含む）の読み取りだけ。作業ツリーは変えない。`ignored` で止まったときだけ、git のディレクトリに止まった時点の記録のファイル `fixprguard-ignored-stop` を書く（置き換える）。

### `finish --baseline TOKEN --judged-set ID PR`

`/fixpr` のフェーズ 4。1 回の呼び出しで次の順序を強制する。(1) 作業ツリーが判定した集合と一致し、git の状態（対象のファイルを含む）が基準値と同じ。(2) `make fmt`。(3) 整形後の集合を記録（変更のあるパスの集合が変わっていれば止まる）。(4) `make test`・`make lint`（`make test` の後にも git の状態を比べる）。(5) 整形後の集合と厳密に一致。(6) 判定したパスだけを、パスを文字どおりに解釈する指定で stage。(7) stage の内容を確認（違えば取り消す）。(8) commit。(9) commit のツリーが整形後の集合と一致することを確認。make のターゲットは `fmt`・`test`・`lint` の固定値で、引数からは受け取らない。commit の件名は `fix: address PR #<PR> review comments` である。

```sh
fixprguard finish --protocol 4 --baseline 'v3....' --judged-set '<judgedSet>' 42
```

出力: `committed`（判定した集合が空なら `false` で、何もしない）と `commit`（作った commit の SHA。`push` に渡す）。

副作用: `make fmt` の整形、build checks が行う書き込み、stage と commit。push・ブランチの切り替え・reset はしない（stage の取り消しを除く）。`ignored` で止まったときは、止まった時点の記録のファイルを書く。

### `push --baseline TOKEN --commit SHA`

`finish` が返した SHA を push する。直前に、hooks・config・info と対象のファイルが基準値と同じこと、HEAD がその SHA でその親が基準値の commit だけであること、push 先（現在のブランチについて git が解決するリモートの、同じ名前のブランチ）の OID が基準値と同じことを確かめる。明示した refspec で、タグとサブモジュールを伴わずに push する。config の比べ方は `inspect` と同じである。今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定は比べず、設定の値の一覧を変えない編集は検出しない。ほかのブランチの設定の書き換えは、残余のリスクである（7.2）。

```sh
fixprguard push --protocol 4 --baseline 'v3....' --commit '<SHA>'
```

出力: `{"protocol":4,"pushed":true,"commit":"<SHA>"}`。`ignored` で止まったときは、止まった時点の記録のファイルを書く。

### `reply --reviewer LOGIN ... --thread-id ID --comment-id N --fingerprint FP --body-file FILE [--accept ID=FINGERPRINT ...] [PR]`

スレッドを現在の状態で判定し直し、信頼する著者のスレッド（または、`--accept` で名指しされて指紋が一致するスレッド）で、`--fingerprint` が現在の指紋と一致し、`--comment-id` がそのスレッドの最初のコメントのときだけ、返信を投稿する。本文は JSON として標準入力で `gh api` に渡すので、シェルを通らない。返信の後、スレッドを取り直して返信のほかに変化がないことを確かめてから、スレッドを解決する。

本文のファイルは、通常のファイル（最後の要素がシンボリックリンクでない）で、64 KiB 以下、空白だけでなく、NUL を含まない有効な UTF-8 でなければならない。また、`@codex` のメンションを含んではならない。比べ方は ASCII の範囲で大文字と小文字を区別しない部分文字列の一致で、語の境界を求めない（`@codexbot` や `a@codex.example` も拒否する）。メンションの検査はファイルの検査の後、スレッドを読む前に行い、含むときは終了コード `2` で拒否する。利用者のアカウントの投稿が Codex を動かさないためである。

```sh
fixprguard reply --protocol 4 --reviewer coderabbitai --thread-id 'PRRT_...' --comment-id 123 --fingerprint '<指紋>' --body-file /path/to/reply.md 42
```

出力: `{"protocol":4,"written":true}`。`reply-not-resolved` で止まったときは、返信が投稿された可能性があるので、同じ返信を再び投稿しない。

### `edit-pr --title-file FILE --body-file FILE [PR]`

PR のタイトルと本文を更新する。タイトルは `--title=<値>` の形で、本文は検査した内容を標準入力から渡す（`gh` にファイルを読み直させない）。タイトルのファイルは、本文のファイルと同じ検査に加えて、制御文字と行区切りを含まない 1 行であること。タイトルと本文のどちらかが `@codex` のメンションを含むときは、`reply` の本文と同じ規則で、`gh pr edit` を起動する前に終了コード `2` で拒否する。

```sh
fixprguard edit-pr --protocol 4 --title-file /path/to/title.txt --body-file /path/to/body.md 42
```

出力: `{"protocol":4,"written":true}`。

## 6. 範囲の規則

`inspect` は、`--thread` で渡したスレッドの `path`（GitHub から取り直したもの）ごとに、範囲に入るファイルを次の規則で決める。

1. `path` そのもの。
2. `path` と同じディレクトリのファイル（サブディレクトリの中は含まない）。ただし、許可リスト（下の表）に当たるファイルだけ。
3. `path` が `docs/tasks/NNNN_<name>/` の中にあれば、そのディレクトリの下のファイル（サブディレクトリの中を含む）。ただし、許可リストに当たるファイルだけ。
4. ただし、コマンドの実行に関わるファイル（実行関連ファイル。下の表）は、`path` そのものでなければ範囲外とする。名指しされた想定外の著者のスレッドでは、`path` そのものでも範囲外とする。
5. 名前を変えたファイルは、変える前の削除と変えた後の追加を、それぞれ判定する。
6. 通常のファイル以外の変更（シンボリックリンク、サブモジュール、ファイルの種類の変更、FIFO やデバイス、8 MiB を超えるファイル）は、パスにかかわらず範囲外とする。

規則 1 には許可リストを当てない。実行関連ファイルでないファイルは、`path` そのものなら種類を問わず範囲に入る。

**パスの検査。** 有効な UTF-8 で制御文字を含まず、絶対パスでなく、`..` の要素を含まず、正規化で変わらないパスだけを判定する。それ以外は範囲外とし、出力に `path` を出さない。ASCII でない文字を含むパスは、`path` とバイト単位で一致するときだけ範囲に入る。

**実行関連ファイル。** 規則 4 で、`path` そのものでなければ範囲外とするファイル（拒否リスト）である。次の 3 つの部分からなる。

- build checks と CI が、設定として読む、またはコンパイル・実行するファイル。Go のビルドの入力（Go がパッケージのビルドで、`.go` のファイルのほかに扱うファイル）を含む。
- 指示ファイル。Claude Code や自動レビュアーが、置かれたディレクトリへの指示として自動で読むファイルである。最上位の `CLAUDE.md`・`AGENTS.md` も含む。
- `.` で始まる要素（ディレクトリの名前またはファイルの名前）を含むパス。`.claude/`・`.github/` の下のファイルや、`.gitignore`・`.gitattributes` は、どの階層にあってもこれに当たる。

照合は ASCII の大文字小文字を同一視して行う（大文字小文字を区別しないファイルシステムでは `makefile` と `Makefile` が同じファイルを指すため）。一覧は `internal/fixprguard/scope.go` の定義と同じである。

| 形 | 項目 |
|---|---|
| どの階層でも、この名前のファイル | `makefile`、`gnumakefile`、`go.mod`、`go.sum`、`go.work`、`go.work.sum` |
| どの階層でも、この名前の指示ファイル | `claude.md`、`agents.md`、`claude.local.md`、`agent.md`、`gemini.md`、`agents.override.md` |
| どの階層でも、この拡張子のファイル | `*.mk` |
| どの階層でも、この拡張子の Go のビルドの入力 | `*.c`、`*.h`、`*.cc`、`*.cpp`、`*.cxx`、`*.hh`、`*.hpp`、`*.hxx`、`*.m`、`*.s`、`*.sx`、`*.f`、`*.for`、`*.f90`、`*.swig`、`*.swigcxx`、`*.syso` |
| どの階層でも、`.` で始まる要素を含むパス | `.*`（例: `.claude/`・`.github/` の下のすべてのファイル、`.golangci.yml`・`.gitignore`・`.gitattributes`・`.tool-versions`・`.mise.toml`、`extension/.npmrc`・`extension/.node-version`・`extension/.prettierrc.json`・`extension/.prettierignore`） |
| この下のすべてのファイル | `extension/scripts/` |
| `extension/` の直下のファイル | `extension/package.json`、`extension/package-lock.json`、`extension/eslint.config.js`、`extension/tsconfig.json`、`extension/tsconfig.core.json`、`extension/tsconfig.build.json` |

**許可リスト。** 規則 2・3 で範囲に入りうるファイルの種類である。実行関連ファイルでなく、次のどれかに当たるファイルだけが、規則 2・3 で範囲に入る。(a)・(b) は大文字と小文字を区別して照合する（`README.MD` や `TestData/` の下のファイルは当たらない）。(c) は、実行関連ファイルと同じく大文字小文字を同一視する。

| 形 | 項目 |
|---|---|
| (a) 名前がこの拡張子で終わる（大文字小文字を区別する） | `*.md`、`*.txt` |
| (b) ディレクトリの要素のどれかがこの名前（大文字小文字を区別する） | `testdata/` |
| (c) `.go` の規則（下記。大文字小文字を同一視する） | `*.go` |

`.go` の規則: `.go` のファイルは、規則 2 では、`path` が `.go` のファイルで同じディレクトリにあるときだけ範囲に入り（同じパッケージのテストの修正）、規則 3 では範囲に入らない。名指しされた想定外の著者のスレッドでは、`path` そのもの以外の `.go` のファイルは範囲外である。`make test` が範囲内の `.go` のファイルをコンパイルして実行するためである。`.go` のファイルには、(b) にかかわらずこの規則だけを当てる（`testdata` の下の `.go` のファイルも、この規則で判定する）。

**判定の順序。** `inspect` は、範囲外になった理由を出力しない。一覧のパスを次の順序と上の 2 つの表に照らせば、理由が分かる。規則 6 に当たる変更と、パスの検査に通らないパスは、この順序の前に範囲外になる。それ以外の変更したパスを、1 つのスレッドの `path` について、次の順に調べる。どれかのスレッドで範囲内になれば、その変更は範囲内である。

1. 実行関連ファイルなら、名指しのスレッドでなく、変更したパスが `path` そのもののときだけ範囲内。それ以外は範囲外。
2. 変更したパスが `path` そのものなら範囲内（規則 1）。
3. 変更したパスが ASCII でない文字を含めば範囲外。
4. `.go` のファイルなら、`.go` の規則で決める。
5. 規則 2・3 のどちらの場所にもなければ範囲外。
6. 許可リストの (a) または (b) に当たれば範囲内。当たらなければ範囲外。

## 7. ツールが保証すること・しないこと

### 7.1 保証すること

| 保証 | 仕組み |
|---|---|
| 想定外の著者のスレッドの本文を取得しない | 判定は本文を含まない問い合わせで行い、本文は処理するスレッドについてだけ取得する。判定と本文の取得の間にコメントが変わったスレッドは、指紋の不一致で処理しない |
| 区切りのブロックを本文が終わらせられない | nonce が本文に含まれないことを確かめる。`begin` の `threads` は 1 行の JSON なので、本文の改行や引用符は区切りの行を作れない |
| 範囲外の変更を、利用者の確認なしに commit しない | `inspect` が範囲外を一覧にして終了コード `3` を返し、`finish` は判定した集合と一致する変更だけを commit する |
| 判定の後に現れた変更を commit しない | `finish` が、判定した集合との厳密な一致を build checks の前と後で確かめ、判定したパスだけを stage する |
| 書き換えられた hooks・config で git を起動しない | `inspect`・`finish`・`push` が、まず git のディレクトリの場所と config のファイルの種類を、git を起動せずにファイルだけで比べる。次に、コマンドを起動しない `git config --list` だけを起動し、今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定を除いた設定の値の一覧を基準値と比べる。config を比べる前に起動する git は `git config --list` だけで、違えば、ほかの git・`gh`・make を起動しない。hooks は、設定の値の一覧が一致した後に `git rev-parse --git-path hooks` で場所を求めて比べる。hook を実行しうるコマンドより前である。設定の値の一覧を変えない編集は検出しない。ほかのブランチの設定の書き換えは、残余のリスクである（7.2） |
| メンションを含む文面を GitHub に送らない | `reply` の本文と `edit-pr` のタイトル・本文が `@codex` を（ASCII の範囲で大文字と小文字を区別せず、語の一部でも）含めば、GitHub に書き込む前に終了コード `2` で拒否する。利用者のアカウントの投稿が Codex を動かすのを防ぐ。コマンドの文書が投稿の前に行う確認（タスク 0031 で加える）に重ねる裏の守りである |
| 返信と PR の説明がシェルのコマンドにならない | `reply`・`edit-pr` はシェルを介さず、本文を標準入力で渡す |
| fork の PR の head を扱わない | `head`・`list`・`begin` が最初に確かめる |
| git が無視するファイルのうち監視の一覧に当たるものの作成・書き換え・削除を、`inspect`・`finish`・`push` が基準値と比べて検出する | `begin` が、`git ls-files --others --ignored --exclude-standard` で列挙した無視されるファイル（git が 1 つの項目として示す入れ子のリポジトリは、ツールが中をたどる）のうち、下の監視の一覧に当たるもの（対象のファイル）と、必ず含める `.claude/settings.local.json` の状態を基準値に記録する。`inspect`・`finish`（build checks の後を含む）・`push` は、config と hooks の後に同じ手順で状態を求めて比べ、違えば `git-state-changed`（`ignored`）で止まる。状態は、有無・種類・内容・実行の許可・シンボリックリンクの文字列とたどる先である。パス・名前・内容は出力せず、git のディレクトリの記録のファイルにだけ書く |

**監視の一覧。** 対象のファイルは、git が無視するファイル（`.gitignore`・`info/exclude`・グローバルの除外のファイルのどれで無視されてもよい）のうち、作業ツリーの最上位からのパスが次の表に当たるものである。表の項目は `internal/fixprguard/ignored.go` の定義と同じで、`cmd/fixprguard/docs_test.go` が一致を確かめる。

| 形 | 項目 |
|---|---|
| 最上位のこのディレクトリの下のすべて（ディレクトリそのものを含む） | `.claude/` |
| 最上位のこの名前のファイル（先頭のスラッシュは最上位だけで当たる印） | `/.env`、`/.envrc`、`/claude.local.md` |
| どの階層でも、この名前のファイル | `claude.md`、`makefile`、`gnumakefile`、`go.mod`、`go.work` |
| どの階層でも、名前がこの文字列で終わるファイル（名前がその文字列だけのものを含む） | `*.go`、`*.mk`、`*.c`、`*.h`、`*.cc`、`*.cpp`、`*.cxx`、`*.hh`、`*.hpp`、`*.hxx`、`*.m`、`*.s`、`*.sx`、`*.f`、`*.for`、`*.f90`、`*.swig`、`*.swigcxx`、`*.syso` |

照合は、すべての行で Unicode の単純な大文字小文字の変換で同一視する（KELVIN SIGN と `k`、LONG S と `s` を含む）。不正な UTF-8 のバイトは、同じバイトだけを同一視する。パスは正規化しない。範囲の規則（6 章）とは別の一覧で、共有するのは Go のビルドの入力の拡張子だけである。FIFO・ソケット・デバイスは、git が無視されるファイルとして示さないので対象にならない（`.claude/settings.local.json` は例外で、種類を記録する）。最上位の `.claude/` の下に名前が `.git` と同一視できるものがあれば、git がその下を列挙しないので `git-failed` で止まる（git が無視されるものとして示す入れ子のリポジトリのものを除く）。

### 7.2 保証しないこと

#### 7.2.1 著者の判定と GitHub への書き込み

- `--reviewer` で渡した名前の集合が、`.claude/commands/_context.md` の `Automated reviewer list` の名前と同じであること。ツールは `_context.md` を読まない。名前を渡し忘れても、その Bot のスレッドは `other-bot` で信頼されないだけである。
- 同じ秒の内の再編集。指紋の時刻は秒の単位である。
- 返信と PR の説明の本文の内容のうち、メンションのほかのもの。ツールが検査するのは、ファイルの種類・大きさ・文字コードと、`@codex` のメンションだけである。全角の `＠codex` や、`@` と `codex` の間に幅のない文字を挟んだ形は、GitHub のメンションにならないので検査しない（Codex が本文を独自に解釈するかは確かめていない）。また、セッションは Bash で `gh` を直接実行できるので、メンションの検査は抜け道のない境界ではない。

#### 7.2.2 区切りのブロックとコマンドの文書

- エージェントが区切りの内側の指示に従わないこと。区切りの偽造は防ぐが、LLM の振る舞いは保証できない。
- `inspect` から `finish` までを呼ぶ順序、利用者への確認、名指しを直前の報告にあるスレッドに限ること。これらはコマンドの文書（セッション）に依存する。

#### 7.2.3 変更範囲

**隠された変更。**

- 監視の一覧（7.1）に当たらない、git が無視するファイルの書き換え。7.2.5 の残余のリスクである。監視の一覧に当たるものは、`inspect`・`finish`・`push` が検出する（7.1）。
- 作業ツリーの中の `.gitignore` やグローバルの除外のファイルで隠したファイルのうち、対象のファイルでないもの。隠したファイルは無視されるファイルとして列挙されるので、監視の一覧に当たれば検出する。当たらなければ、変更の集合にも現れない。隠した変更は commit されないが、build checks が実行することは防げない。
- index の stat の偽造で隠した変更。
- 自分自身を無視させる `.gitignore`（2 つ前の項目の一部）。下の階層に、自分自身と別のファイル（`foo.syso`・`CLAUDE.md` など）を無視させる `.gitignore` を置くと、どちらも変更の集合に現れないので、範囲の規則（6 章）は判定しない。無視されたファイルは commit されないが、作業ツリーに残る。`foo.syso`・`CLAUDE.md` のように監視の一覧に当たるものは、対象のファイルとして検出する（7.1）。`AGENTS.md` など一覧に当たらない指示ファイルは、無視されると GitHub に届かないが、ローカルのセッションが読むことは防げない（7.2.5）。

**範囲内として commit・push されるもの。**

- 範囲内のテストのコードが build checks で実行されること。`finish` と `push` は、そのコードが範囲内のファイルや hooks を書き換えたことは検出する。
- 指示として読まれる `.md` の文書の書き換え。手順書が読んで従うよう指示する文書（`/fixpr` が読む本書や要件定義のプロセスのガイドなど）、`AGENTS.md` が自動レビュアーに示す文書、PR-Agent が読む `docs/dev/security.md` は、指示ファイルではないので、規則 2・3 で範囲に入る。そこに書かれた指示は commit・push され、同じブランチの後のセッションや自動レビュアーが読む。変更は PR の diff に現れるので、マージの前に人が確かめる。
- `testdata` の下のファイル。実行関連ファイルと `.go` のファイル（`.go` の規則で判定する）を除き、種類を問わず規則 2・3 で範囲に入る（許可リストの (b)）。テストが読むが、テストがそれを実行しない限り、実行されるコードにはならない。
- 規則 1 で範囲に入るファイル。実行関連ファイルでないファイルは、`path` そのものなら種類を問わず範囲に入る。たとえば、スレッドの `path` が `.sh` のファイルなら、その書き換えは範囲内である。
- 拒否リストに載らず読まれる・実行されるファイル。拒否リストは網羅できない。規則 1 で範囲に入る、実行関連ファイルでないファイルが、別のツールに設定や指示として読まれることは防げない。6 章の表にない名前の指示ファイルのうち名前が `.md` で終わるものと、`.txt` の名前で読まれるファイル（`vendor/modules.txt`・`CMakeLists.txt` など。リポジトリには今はない）も、許可リストの (a) に当たるので、規則 2・3 で範囲に入る。
- 同じパッケージの `.go` のファイル。信頼する著者のスレッドでは、`path` が `.go` のファイルなら、`.go` の規則で同じディレクトリの `.go` のファイルが範囲に入り、cgo の前文（`import "C"` の前の C のコードと `#cgo` の指示）も書ける。名指しのスレッドでも、`path` そのものの `.go` のファイルには、規則 1 で cgo の前文を書ける。
- ファイルシステムの Unicode の畳み込みによる別名。拒否リストの照合は `strings.ToLower` で小文字にするだけで、大文字小文字を区別しない macOS のボリュームの畳み込みとは一致しない。APFS では、`agentſ.md`（U+017F）が `AGENTS.md` として、`makeﬁle`（U+FB01）が `makefile` として開かれる。規則 2・3 では ASCII でない文字を含むパスは範囲外なので、影響は規則 1 の位置に限られる。そのパスがすでに PR の diff にあり、スレッドが付いている必要がある。

**利用者が範囲外の一覧で見落としうるもの。**

- 双方向の書式文字によるパスの見せかけ。パスの検査は制御文字を拒否するが、U+202E などの書式文字は拒否しない。`/fixpr` が示す範囲外の変更の一覧で、`.syso` のパスが `.go` のパスのように見えうる。範囲外になった理由は出力しないので、利用者は名前を目で見て判断する。
- 確認への慣れ。許可リストの外の正当な変更（`.ts` のテストなど）は、毎回範囲外として確認に回る。「続ける」と答えることに慣れると、長い一覧の中の危険なパス（指示ファイルや Go のビルドの入力）を見落としうる。一覧のパスは、6 章の判定の順序と 2 つの表に照らして、範囲外になった理由を確かめる。

#### 7.2.4 git の状態の比較

- 今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定の書き換え（残余のリスク）。並行するセッションのブランチの操作で止まらないように、比べない。修正のフェーズや build checks のコードがほかのブランチの設定を書き換えても検出せず、書き換えは実行の後にも残る。そのブランチの上で後に行う `git push`・`git pull`・`git merge` の宛先や振る舞いが変わりうる。そのブランチに切り替えて `/fixpr` を実行すれば、そのブランチが今のブランチになるので、`begin` の後の書き換えは検出される。ツールが実行の間に起動する git は、ほかのブランチの設定を宛先の決定に使わない。ただし、値のない `branch.<name>.remote`・`merge`・`pushRemote` や `[branch ""]` の節があると git が失敗し、`git-failed` または `response-rejected` で止まる（8 章）。
- 設定の値の一覧を変えない編集（コメント・空行・字下げだけの編集）。git の振る舞いを変えないので検出しない。
- include の先を FIFO に置き換えた場合、`git config --list` が時間切れ（2 分）まで待つので、ツールが `git-state-changed`（`config`）で止まるまで最大 2 分かかる。2 分は Claude Code の Bash ツールの既定のタイムアウトと同じなので、止まったことがセッションに遅れて届くことがある。
- `gh` が起動する git について確かめた範囲。`inspect` は、PR 番号を渡すと、`gh repo view`、スレッドの照会の `gh api graphql`、権限の照会の `gh api` を起動する。`gh` 2.96.0 で確かめたところ、`gh repo view` が起動する git は `git remote -v` と `remote.*` の読み取りだけで、後の 2 つは git を起動しなかった。`git remote -v` は、ほかのブランチの設定を読み込むが、宛先の決定には使わない。値のない設定や `[branch ""]` の節があると失敗する。PR 番号を省いたときに `gh pr view --json number` が起動する git は確かめていない（`/fixpr` は PR 番号を渡す）。`gh` の版が変われば変わりうる。並行するセッションの `gh pr checkout` は、通常の PR で `branch.<そのブランチ>.*` だけを書き込むことを確かめた。fork の PR は確かめていない。ほかの設定にも書き込むなら、`git-state-changed` で止まりすぎるだけである。
- 確認の時点の状態だけを保証する。確認から実行までの間に別のプロセスが hooks や config を書き換えること、修正のフェーズの間に変えて元に戻したことは検出しない。
- 基準値を持たない `head`・`list`・`reply`・`edit-pr` が `gh` を起動する前の比較。`/fixpr` はこれらを `finish`・`push` の後に呼ぶので、それまでの比較が書き換えを検出するが、`push` の後の書き換えは検出しない。
- hooks のディレクトリの外にあり、hook が実行するファイル（pre-commit のフレームワークのキャッシュなど）。

#### 7.2.5 git が無視するファイル

次のことは検出しない。どれも Claude Code の権限の確認（利用者の設定）に依存する。

- 監視の一覧に当たらない、git が無視するファイルを変えること。後で実行されるか、設定として読まれるものを含む。例: 一覧に当たらない名前のドットファイル（`.tool-versions`・`.mise.toml`・`.npmrc`、下の階層の `.envrc` など）、`go.sum`・`go.work.sum`、`extension/` の下の設定のファイルとスクリプト、`.github/` の下のファイル、`build/` の下のバイナリ（`yt2column-nmhost install` が登録した Native Messaging host は、ブラウザが次の送信で起動する）、`extension/node_modules/` の下の JavaScript（`make ext-check` が実行する）、`.vscode/` の下の設定、下の階層の `CLAUDE.local.md`、`AGENTS.md`・`GEMINI.md` など `CLAUDE.md` でない名前の指示ファイル。
- この作業ツリーの外のファイルを変えること。同じリポジトリのほかの worktree と main のチェックアウトを含む。
- `begin` の後に、監視の一覧に当たるパスへ、通常のファイルでないもの（FIFO・ソケット・デバイスなど）を新しく作ること。`.claude/settings.local.json` は例外で、検出する。すでにある対象のファイルを通常のファイルでないものに置き換えた場合も検出する。
- 完全な大文字小文字の変換だけで監視の一覧の項目と同じになる名前のファイルを置くこと。例: 合字 `ﬁ`（U+FB01）を含む `GNUmakeﬁle`。大文字小文字を区別しない macOS の APFS は、これを `GNUmakefile` として解決するので `make` が読むが、照合は 1 つの文字を複数の文字に展開する変換（`ﬁ` → `fi`、`ß` → `ss` など）を同一視しない。
- 対象のファイルを変えて、確認の前に元に戻すこと。確認の後に書き込むこと（修正のフェーズのエージェントや `make test` が残したプロセスによるものを含む）と、`push` の後に変えること。
- 検出の前に効く書き込み。`.claude/settings.local.json` への書き込みが、実行中のセッションに、ツールが止まる前から効く場合がある。
- まとめた値からの推測。ほかの対象のファイルの内容が分かっていて、`.env` の内容の候補が少なければ、基準値の 9 番目の欄から内容を推測できる。基準値を読めるエージェントは `.env` を直接読むこともできる。
- 記録のファイルの削除と書き換え。修正のフェーズのエージェントが git のディレクトリに書けるかどうかは確かめていない。`begin` の記録の書き換えは基準値との照合で、削除は記録がないことで分かるが（8.1）、そのときは変わったファイルを記録から特定できない。止まった時点の記録は基準値と結び付けていないので、書いた後の書き換えは分からない。
- 止まった後のやり直しで、利用者が中身を確かめずに `begin` を実行すること。正当な書き込みでも止まるので、止まることに慣れて確かめなくなるおそれがある。やり直した `begin` は、その時点の対象のファイルを新しい基準値として受け入れる。

次の場合は、正当な書き込みや置き方でも止まる。

- 利用者が修正のフェーズの間に Claude Code の許可の確認に「今後は確認しない」と答えた（`.claude/settings.local.json` が書き換わる）場合や、`.claude/` の下に Claude Code の実行時のファイル（`scheduled_tasks.lock` など）、エディタのスワップファイル、`.DS_Store` ができた場合。
- 対象のファイルがディレクトリを指すシンボリックリンクである場合と、`.claude/worktrees/` の下に worktree がある場合。`begin` も止まり続けるので、原因を取り除くまで `/fixpr` を使えない。worktree は `.claude/` の外に置く。

既存の clean の検査と変更の集合は、読めないディレクトリを git の警告だけで飛ばす（#446）。対象のファイルの列挙は、git が標準エラーに何か書けば止まる。

## 8. 止まったときの対処

終了コード `1` の出力の `stop` が、止まった種類を示す。止まった場合はツールを回避せず、原因を取り除いてから、ラウンドを `begin` からやり直す。

| `stop` | 原因 | 詳細の項目 | 対処 |
|---|---|---|---|
| `dirty-work-tree` | `begin` の時点で作業ツリーに未コミットの変更（未追跡のファイルを含む）がある | なし | `git status` で変更を調べ、捨てるか commit してからやり直す。範囲外の変更で前のラウンドを止めたときも、変更が残っているのでここで止まる |
| `cross-repository` | PR の head が fork など別のリポジトリにある | なし | ツールでは扱わない。PR の内容を自分で確かめる |
| `git-state-changed` | HEAD・ブランチ・push 先・hooks・config・info・対象のファイル（7.1 の監視の一覧に当たる、git が無視するファイル）のどれかが基準値と違う。config は、今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*` の設定を除いた設定の値の一覧、git のディレクトリの場所、config のファイルの種類を比べる。`begin` の後に設定の値の一覧が読めなくなった場合（構文の誤り、include の先の FIFO による時間切れなど）も含む。設定の値の一覧を変えない編集では止まらない。ほかのブランチの設定の書き換えは、残余のリスクである（7.2） | `parts`（`head`・`remote`・`hooks`・`config`・`info`・`ignored`）。`ignored` は対象のファイルで、config・info・hooks が一致したときだけ並ぶ。`inspect`・`finish`・`push` は HEAD と push 先をその後に比べるので、`ignored` で止まったときは、それらを比べていない | 違う部分を調べる。`ignored` なら 8.1 の手順に従う。どのファイルが変わったかは出力しないので、セッションに調べさせない。`config` なら `git config --list --show-origin` で設定を調べ、意図しない hooks や config の変更なら取り除く。リモートが正当に進んだだけなら、作業ツリーを整理してやり直す |
| `judged-set-changed` | 作業ツリーや commit が判定した集合と違う（判定の後に変更が現れた、build checks が範囲内のファイルを書き換えた、commit の hook が index を書き換えた、など） | `changes`（見つかった変更の一覧） | 一覧の変更を調べる。commit が作られた後に止まったときは、その commit を調べて取り消す |
| `thread-cap` | `list` が読むスレッドが 2000 件を超える | なし | GitHub 上でスレッドを解決して減らす |
| `change-cap` | 変更のあるファイルが 1000 個を超える | なし | 変更を調べて減らす |
| `thread-changed` | `inspect` や `reply` の対象のスレッドのコメントが、指紋を採った後に変わった、または解決された | なし | 次のラウンドで拾う。`begin` からやり直す |
| `thread-not-trusted` | `reply` の対象が、信頼する著者のスレッドでも、名指しされたスレッドでもない | なし | 返信しない。想定外の著者のスレッドの扱いは利用者が決める |
| `reply-not-resolved` | 返信が投稿された（か、投稿されたか分からない）が、スレッドを解決できなかった | なし | 同じ返信を再び投稿しない。GitHub でスレッドを確かめ、必要なら手で解決する |
| `response-rejected` | GitHub の呼び出しが失敗した、時間切れになった、または応答が厳格な検査や大きさの上限に通らなかった。`gh` の中の `git remote -v` が、ほかのブランチの値のない設定や `[branch ""]` の節（`git-failed` の行）で失敗した場合も含む | なし | `gh auth status` と GitHub の状態を確かめてやり直す。`git remote -v` が失敗するなら、`git-failed` の行の対処をする |
| `build-checks-failed` | `make fmt`・`make test`・`make lint` のどれかが失敗した（出力が 64 MiB を超えた場合を含む。そのときは `output` がない） | `stage`（`fmt`・`test`・`lint`）と `output`（出力の末尾 8 KiB を区切りのブロックにしたもの） | 出力を確かめて原因を直す。`output` はコードの実行結果なので、信頼できないテキストとして扱う |
| `git-failed` | git が失敗した、出力が想定の形でない、または git のメタデータを読めなかった（追跡済みのファイルを FIFO に置き換えた、git のリポジトリの外で実行した、など）。ほかのブランチの `branch.<name>.remote`・`merge`・`pushRemote` が値なしで書かれた、`[branch ""]` の節がある、設定の値の一覧が出力の上限（64 MiB）を超えた、の各場合も含む。対象のファイルの状態を求められない場合（列挙の上限、1 つのファイルの大きさの上限、読み取る内容の合計の上限、対象のファイルやディレクトリを読み取れない、など）も含み、標準エラーの規則の名前で分かる（8.2） | なし | `git status` が通るかを確かめる。`git config --list --show-origin` が動くかとその大きさを確かめ、値のない設定や `[branch ""]` の節があれば取り除く。規則の名前が 8.2 の表にあれば、8.2 の対処をする |
| `failed` | 上のどれにも当たらない失敗（区切りの nonce を選べなかった、など） | なし | 標準エラーの診断を確かめる |

範囲外の変更で利用者が「続けない」と答えたときは、変更は作業ツリーに残る。次の `/fixpr` は `dirty-work-tree` で止まるので、変更を調べて、捨てるか commit してからやり直す。残った変更は、git の表示で調べる（`git status --untracked-files=all`・`git diff`、未追跡のファイルの中身は `git diff --no-index /dev/null <path>`。自分自身を無視させる `.gitignore`（7.2）で隠したファイルは `git status --ignored --untracked-files=all` で探す）。調べ終えるまで、`make`・`go test` を実行したり、そのディレクトリで Claude Code のセッションを始めたりしない。今のセッションにも、そのディレクトリのファイルを読ませない。下の階層の指示ファイルや Go のビルドの入力は、置かれているだけで読まれる、またはリンクされるためである。

### 8.1 `ignored` で止まったとき

ツールは、どの対象のファイルが変わったかを出力しない。ファイルの名前は書いた者が選べるので、セッションに届けないためである。次の手順は、Claude Code のセッションの外の端末で、作業ツリーの最上位で実行する。セッションには対象のファイルを一覧にさせたり読ませたりしない。更新時刻は偽造できるので、確認するファイルを更新時刻で絞り込まない。

**(a) 記録の照合と (b) 変わったファイルの特定。** `begin` の記録のファイルは、`begin` の時点の対象のファイルごとに、状態の digest と ASCII に引用したパスを 1 行に書いたものである。止まった時点の記録のファイルは、止まった時点の同じ一覧である。`begin` が出力した `baseline`（セッションに残る `begin` の出力にある）を変数 `token` に入れてから（`token='v3....'`）、次を実行する。`begin` の記録の SHA-256 が基準値の 10 番目の欄と一致すれば、2 つの記録を `diff` で比べる。

```sh
gitdir=.git
if [ -f .git ]; then gitdir=$(sed -n 's/^gitdir: //p' .git); fi
want=$(printf '%s\n' "$token" | cut -d. -f10)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum < "$gitdir/fixprguard-ignored-begin" | cut -d' ' -f1)
else
  got=$(shasum -a 256 < "$gitdir/fixprguard-ignored-begin" | cut -d' ' -f1)
fi
if [ -n "$got" ] && [ "$got" = "$want" ]; then
  echo 'begin record: matches the baseline'
  diff "$gitdir/fixprguard-ignored-begin" "$gitdir/fixprguard-ignored-stop" | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
else
  echo 'begin record: does not match the baseline, do not trust it'
fi
```

Linux では `sha256sum` を、macOS では `shasum -a 256` を使う。`diff` の `<` の行は `begin` の時点にだけある行、`>` の行は止まった時点にだけある行である。同じパスが両方にあればそのファイルが変わり、`<` だけなら消え、`>` だけなら増えた。`diff` の出力は、`cat -vt` が制御文字とタブを `^` で始まる形で示す（行を折り返さない）。その前に `sed` が文字の `^` を `^^` にするので、`^A` は制御文字、`^^A` は `^` と `A` の 2 文字である。止まった時点の記録の各行のパスが、止まった時点の対象のファイルの一覧である。止まった時点の記録は基準値と結び付いていないので、内容を信頼しすぎない。止まった時点の記録だけがない場合は、書けなかったことがある。`inspect` をやり直せば、同じ比較で書き直される。

照合が一致しない、または `begin` の記録がない場合は、侵害を強く疑う。`.claude/settings.local.json`・`.envrc`・`.env` を、利用者が別に保管している信頼できる写しから戻す。記録に頼らず、次のコマンドで無視されるファイルと `.claude/` の下を一覧にし、監視の一覧（7.1）に当たるすべてのファイルを確かめる。出力のパスはエスケープされ、名前の中の改行は `^A`、文字の `^` は `^^` と示される。

```sh
git -c core.fsmonitor=false ls-files --others --ignored --exclude-standard -z | tr '\n\0' '\001\n' | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
find .claude -print0 | tr '\n\0' '\001\n' | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
```

`begin` が `ignored` で止まったときは、このラウンドの記録はない（`begin` は、状態を求め直す前に前のラウンドの記録を消す）。上のブロックは実行せず、このコマンドで一覧にしたファイルを (c) のとおり確かめてから、やり直す。

**(c) 内容を確かめるファイル。** 変わったファイルに加えて、一覧のすべてのファイルの内容を確かめる。特に `.claude/settings.local.json` の `hooks`・`permissions`、`.envrc`・`.env`、無視される場所の `.go`・Go のビルドの入力・`Makefile`・`*.mk` である。意図しない変更は取り除く。

**(d) 作業ツリーに残った変更と、作られた commit。** 修正のフェーズが作業ツリーに残した変更は、調べたうえで捨てて `/fixpr` をやり直すか、利用者が確かめて手で commit する。`finish` は commit の後にも比べるので、commit が作られてから、または stage したまま止まることがある。HEAD が基準値の commit でなく、その件名が `fix: address PR #<PR> review comments` なら、その commit を調べて取り消す。変更を残したまま `begin` を実行すると `dirty-work-tree` で止まる。

**(e) 正当な原因の場合。** 利用者が許可の確認に答えたことや、Claude Code の実行時のファイル（`.claude/scheduled_tasks.lock`、`.claude/worktrees/` の下の worktree など）が原因でも、内容を確かめてからやり直す。worktree は `.claude/` の外に置く。やり直した `begin` は、その時点の対象のファイルを新しい基準値として受け入れる。

### 8.2 対象のファイルの状態を求められないとき

`git-failed` の標準エラーの診断 `fixprguard: git command failed: <規則の名前>` の規則の名前が、原因を示す。対処は、Claude Code のセッションの外の端末で行う。

| 規則の名前 | 原因 | 対処 |
|---|---|---|
| `list ignored files failed` | 無視されるファイルを列挙する git が失敗した、または標準エラーに何か書いた（読めないディレクトリの警告など） | 下のコマンドの 1 行目で git の警告を表示し、警告のディレクトリの読み取りの許可を戻すか、ディレクトリを取り除く |
| `list ignored files output over the cap` | 列挙の出力が 64 MiB を超えた | 無視されるファイルを減らす |
| `list ignored files output shape` | 列挙の出力が受理する形でない | git の版と、作業ツリーのファイル名を確かめる |
| `ignored target count over the cap` | 対象のファイルが 10,000 個を超えた | 監視の一覧に当たる無視されるファイルを減らす |
| `ignored directory entries over the cap` | 入れ子のリポジトリと `.claude/` の下でたどったエントリが 100,000 個を超えた | 無視される場所の入れ子のリポジトリと、`.claude/worktrees/` の下の worktree を取り除く |
| `ignored file over the size cap` | 1 つの対象のファイルの大きさが 16 MiB を超えた | 下のコマンドの 2 行目で探し、取り除く |
| `ignored read total over the cap` | 1 回で読む対象のファイルの内容の合計が 64 MiB を超えた（同じ内容を指す多数のリンクを含む。どのファイルも 16 MiB 以下である） | 下のコマンドの 3 行目でリンクをたどって大きなファイルを、4 行目でシンボリックリンクを探し、不要なものを取り除く |
| `read ignored files timed out` | 状態を求める処理が 2 分を超えた（応答しないファイルシステムを指すリンクなど） | 作業ツリーの外を指すシンボリックリンクを確かめる |
| `read ignored directory` | 入れ子のリポジトリか `.claude/` の下のディレクトリを読めない | 読み取りの許可を戻す |
| `read ignored file` | 対象のファイル（リンクのたどる先を含む）を読めない | 読み取りの許可を戻すか、ファイルを取り除く |
| `ignored link to a directory` | 対象のファイルが、ディレクトリを指すシンボリックリンクである | リンクを取り除く |
| `.git name below .claude` | 最上位の `.claude/` の下に `.git` という名前のものがある | 取り除く。`.claude/worktrees/` の下の worktree なら `.claude/` の外に移す |
| `write ignored begin record` | `begin` の記録のファイルを書けない、または前のラウンドの止まった時点の記録を消せない | git のディレクトリの `fixprguard-ignored-begin`・`fixprguard-ignored-stop` の種類と許可を確かめる |

次のコマンドの 1 行目は、無視されるファイルを列挙する git の警告だけを示す。2 行目は 16 MiB を超えるファイル、3 行目はリンクをたどって 1 MiB を超えるファイル、4 行目はシンボリックリンクを示す。出力のパスは `cat -vt` が制御文字を `^` で始まる形で示すので、端末を制御しない。

```sh
git -c core.fsmonitor=false ls-files --others --ignored --exclude-standard -z 2>&1 >/dev/null | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
find . -path ./.git -prune -o -type f -size +16M -print | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
find -L . -path ./.git -prune -o -type f -size +1M -print | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
find . -path ./.git -prune -o -type l -print | LC_ALL=C sed 's/\^/^^/g' | LC_ALL=C cat -vt
```

## 9. ツールを変更する場合

- 判定は `internal/fixprguard`、引数の解釈と呼び出しの順序は `cmd/fixprguard` にある。外部コマンドは `internal/cmdrunner` の `Runner`・`InputRunner` 越しに呼ぶ。テストでは、`gh` と `make` を `internal/cmdrunner/testutil` の fake に置き換え、git は一時ディレクトリの実際のリポジトリで試す。
- 7.1 節の保証を変更する場合は、対応する実装を壊してテストが失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md の「Testing Strategy」）。
- 出力や引数の契約、または範囲の規則（6 章）を変える場合は、`internal/fixprguard` の `Protocol` を上げ、コマンドの文書が渡す `--protocol` も合わせて更新する。番号は上げるだけで、一度使った番号を再び使ったり、下げたりしない。並行する 2 つのブランチが同じ番号を使うと、片方のブランチで作ったバイナリが、もう片方の文書の `--protocol` を受け付け、違う規則で黙って判定するためである（タスク 0036 が `3` を使ったので、タスク 0034 は `4` を使った）。ただし、次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい。`--reviewer` を繰り返し受け付ける変更（タスク 0033）はこれに当たる。
    1. 古いツールは、新しい文書の呼び出しを引数の解析で拒否し（終了コード `2`）、古い規則で判定しない。
    2. 新しいツールは、古い文書の呼び出しを今までと同じ規則で判定する。
    3. 出力の形は変えない。

  この場合は、古いツールが止まったときの診断と再インストールの案内を、4 章の終了コード `2` の説明に書く。

  また、本文とタイトルのファイルの受け付ける内容を狭めるだけの変更は、狭めた分がコマンドの文書の送らない内容に限って、上の 3 つの条件によらず `Protocol` を上げなくてよい。古いツールはその内容を受け付けるが、文書はそれを送らないので誤った投稿にはならず、新しいツールが古い文書の呼び出しで拒否するのは送ってはならない内容だけだからである。`@codex` のメンションの拒否（タスク 0031）はこれに当たる。
- 自動レビュアーを一覧に加えるとき、または一覧から外すときは、次のものを同じコミットで変え、そのコミットで文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`）を実行する。`fixpr.md`・`runpr.md` は名前も数も書かず、ツールは名前の数に上限を設けないので、どちらも変えなくてよい。
    1. `.claude/commands/_context.md` の `Automated reviewer list` の行。
    2. 内容のガード（`cmd/yt2column/fixpr_docs_test.go`）の、期待する名前（`wantReviewers`）。
    3. 同じファイルの、壊した写しの表（`TestContextReviewerRowsMalformed`）の行に書いた名前。

  期待する名前の確認を「解析に通って空でない」程度に緩めない。一覧だけが書き換えられたことを、内容のガードが検出できなくなるためである。
- 一覧から外した名前の Bot は、すぐには信頼されなくならない。各セッションは作業しているブランチの `_context.md` を読むので、`main` を取り込んでいない開いたブランチでは、その Bot のスレッドの信頼が続く。急いで信頼をやめるときは、外す変更を `main` に入れたうえで、開いている PR のブランチに `main` を取り込む。取り込むまでは、そのブランチで `/fixpr`・`/runpr` を使わない。
- `stop` の種類、実行関連ファイルの一覧、許可リストを変えたら、本書の 6 章と 8 章も更新する。監視の一覧を変えたら、7.1 の監視の一覧の表も更新する。`cmd/fixprguard/docs_test.go` が、本書（日英）にすべての種類と項目があることを確かめる。
- マージの後は、`make install-fixprguard` で再インストールする（3 章）。
