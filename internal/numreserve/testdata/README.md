# numreserve のテストデータ

`internal/numreserve` のテストが、外部のコマンドの出力を読む部分の入力を作るときのもとにする、実物から採ったサンプルである。タスク 0100 の PR-2（実装計画書のステップ 2-1〜2-3）で採った。リポジトリに置く実データの規則は [security.md](../../../docs/dev/security.md) §7 と [ADR-0032](../../../docs/dev/adr/0032-testdata-license.md) にある。

## 採った日と版

-   採った日: 2026-11-17（2 回。12:00 UTC ごろの 1 回目と、12:43 UTC ごろの 2 回目）
-   gh 2.96.0、git 2.50.1（Apple Git-155）、macOS の手元の worktree。どのコマンドもリポジトリの根で実行した

## 採り方

-   **`gh/`。** 記録の名前空間の外の試しの tag（`refs/tags/numreserve-probe/<日時>-<番号>`）について、`GH_PROMPT_DISABLED=1`・`NO_COLOR=1` の環境で `gh api --hostname github.com -i` を実行した標準出力である。各回の試しの tag は確かめの後に `DELETE` で消し、2 回目の後に `git ls-remote origin 'refs/tags/numreserve-probe/*'` が何も返さないことを確かめた。`exits.txt` に各実行の `gh` の終了コードがある。
    -   `post-201.txt`: 1 回目。`POST git/refs` で `main` の SHA を指す tag を作った。
    -   `post-422-same-sha.txt`: 1 回目。同じ名前・同じ SHA で、もう一度作ろうとした。
    -   `post-422-other-sha.txt`: 1 回目。同じ名前で、`main` の親の SHA を指して作ろうとした。
    -   `get-200.txt`: 2 回目。作った tag を `GET git/ref/tags/<名前>` で読んだ。応答の本文を含む。
    -   `delete-204.txt`: 1 回目。`DELETE git/refs/tags/<名前>` で消した。
    -   `get-404.txt`: 1 回目。消した後に `GET` で読んだ。
-   **`git/`。**
    -   `ls-remote.txt`: 2 回目。試しの tag があるときの `git ls-remote origin 'refs/heads/*' 'refs/tags/*'` の標準出力である。
    -   `ls-tree-adr.z`・`ls-tree-tasks.z`: `ls-remote.txt` の `main` の commit について、`git ls-tree --full-tree -z <SHA> -- docs/dev/adr/` と `-- docs/tasks/` を実行した標準出力である。

## 形について分かったこと

-   `gh api -i` の標準出力の状態行（`HTTP/2.0 201 Created` など）の行末は LF で、見出しの行と、見出しの終わりの空行の行末は CRLF である。本文はその空行の後に続き、末尾に改行がない。本文のない 204 は、空行で終わる。
-   見出しの名前は `X-Ratelimit-Remaining` の形で出る。
-   `GET` の本文は 1 つの JSON のオブジェクトで、`ref`・`node_id`・`url`・`object`（`sha`・`type`・`url`）を持つ。`ref` は問い合わせた名前と一致した。
-   `git ls-remote` の tag の行は、軽量の tag では「SHA、タブ、`refs/tags/<名前>`」の 1 行だけで、`^{}` の行はない。
-   `git ls-tree -z` の項目は「モード、空白、種類、空白、SHA、タブ、パス、NUL」である。`--full-tree` がなければ、パスの指定とパスの出力は今のディレクトリからの相対になる：`docs/` で `git ls-tree <SHA> docs/dev/adr/` を実行すると、標準出力は空で終了コードは 0 だった。指定したディレクトリのない commit でも、標準出力は空で終了コードは 0 である。
-   レート制限の見出しは、同じ `core` でも、`POST` の 3 回と `GET`・`DELETE` とで `X-Ratelimit-Used` と `X-Ratelimit-Reset` の値の組が別だった。それぞれの組の中では 1 ずつ増えている。

## テストで作る形

次の形は採っていない。テストは、上の形をもとに作る。

-   429、403 と rate limit の見出し（`X-Ratelimit-Remaining: 0`、`Retry-After`）、401、5xx の状態行
-   `HTTP/1.1`・`HTTP/2` の状態行。cloud のセッションでは `HTTP/1.1` の形を見たが、見出しは採っていない
-   `gh` が認証されていないときの出力（終了コード 4、状態行なし）
-   注釈付きの tag の `^{}` の行と、入れ子の名前の tag の行

## 置き換えたもの・除いたもの

| 元の値 | 扱い |
|---|---|
| `POST`（201・422）・`DELETE`・404 の応答の本文 | 除いた |
| `GET` の本文の `url` の値（API の URL） | `https://api.github.invalid/removed` に置き換えた |
| `Location` の見出し（試しの tag の API の URL） | 行ごと除いた |
| 道具が読まず、採った人の環境を表す見出し（`X-Github-Request-Id`・`X-Github-Edge-Region`・`X-Oauth-Client-Id`・`X-Oauth-Scopes`） | 行ごと除いた |
| ブランチ名の GitHub のログイン名の部分（`refs/heads/<ログイン名>/...`） | `owner` に置き換えた |

置き換えの後に、URL（置き換えた値を除く）、ログイン名、除いた見出しが残っていないことを確かめた。試しの tag の名前は、採った日時とプロセスの番号だけからなり、残している。
