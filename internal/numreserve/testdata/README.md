# numreserve のテストデータ

`internal/numreserve` のテストが、外部のコマンドの出力を読む部分の入力を作るときのもとにする、実物から採ったサンプルである。タスク 0100 の PR-2（実装計画書のステップ 2-1〜2-3）で採った。リポジトリに置く実データの規則は [security.md](../../../docs/dev/security.md) §7 と [ADR-0032](../../../docs/dev/adr/0032-testdata-license.md) にある。

## 採った日と版

-   採った日: 2026-11-17
-   gh 2.96.0、git 2.50.1（Apple Git-155）、macOS の手元の worktree

## 採り方

-   **`gh/`。** 記録の名前空間の外の試しの tag（`refs/tags/numreserve-probe/<日時>-<番号>`）について、`GH_PROMPT_DISABLED=1`・`NO_COLOR=1` の環境で `gh api --hostname github.com -i` を実行した標準出力のうち、状態行から見出しの終わりの空行までである。応答の本文は除いた。試しの tag は確かめの後に `DELETE` で消し、`GET` の 404 で消えたことを確かめた。`exits.txt` に各実行の `gh` の終了コードがある。
    -   `post-201.txt`: `POST git/refs` で `main` の SHA を指す tag を作った。
    -   `post-422-same-sha.txt`: 同じ名前・同じ SHA で、もう一度作ろうとした。
    -   `post-422-other-sha.txt`: 同じ名前で、`main` の親の SHA を指して作ろうとした。
    -   `get-200.txt`: `GET git/ref/tags/<名前>` で読んだ。
    -   `delete-204.txt`: `DELETE git/refs/tags/<名前>` で消した。
    -   `get-404.txt`: 消した後に `GET` で読んだ。
-   **`git/`。** `git/ls-remote-heads.txt` は `git ls-remote origin 'refs/heads/*' 'refs/tags/*'` の標準出力である（採った日には tag が 1 つもなかった）。`git/ls-tree-adr.txt` は、空のリポジトリに `git fetch --no-tags --no-write-fetch-head origin <main の SHA>` で取得した commit について、`git ls-tree <SHA> docs/dev/adr/` を実行した標準出力である。

## 形について分かったこと

-   `gh api -i` の標準出力の状態行（`HTTP/2.0 201 Created` など）の行末は LF で、見出しの行と、見出しの終わりの空行の行末は CRLF である。そのため、`gh/` の下のファイルは git の改行の変換と pre-commit の空白のフックから外している（`.gitattributes`・`.pre-commit-config.yaml`）。
-   見出しの名前は `X-Ratelimit-Remaining` の形で出る。
-   `git ls-tree` の行は「モード、空白、種類、空白、SHA、タブ、パス」で、パスは `docs/dev/adr/` からの全体である。`docs/dev/adr/` がない commit では、標準出力が空で終了コードは 0 である。

## 置き換えたもの・除いたもの

| 元の値 | 扱い |
|---|---|
| 応答の本文 | 除いた |
| `Location` の見出し（試しの tag の API の URL） | 行ごと除いた |
| ブランチ名の GitHub のログイン名の部分（`refs/heads/<ログイン名>/...`） | `owner` に置き換えた |

置き換えの後に、URL、ログイン名、試しの tag の名前が残っていないことを確かめた。
